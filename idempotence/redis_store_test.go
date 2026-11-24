package idempotence

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type fakeRedisValue struct {
	value   string
	expires time.Time
}

type fakeRedis struct {
	mu      sync.Mutex
	now     time.Time
	entries map[string]fakeRedisValue
	read    chan struct{}
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		now: time.Now(), entries: make(map[string]fakeRedisValue), read: make(chan struct{}, 1),
	}
}

func (f *fakeRedis) advance(duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(duration)
}

func (f *fakeRedis) lookup(key string) (string, bool) {
	entry, exists := f.entries[key]
	if !exists || !f.now.Before(entry.expires) {
		delete(f.entries, key)
		return "", false
	}
	return entry.value, true
}

func (f *fakeRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return redis.NewStringResult("", err)
	}
	value, exists := f.lookup(key)
	if !exists {
		return redis.NewStringResult("", redis.Nil)
	}
	select {
	case f.read <- struct{}{}:
	default:
	}
	return redis.NewStringResult(value, nil)
}

func (f *fakeRedis) SetNX(ctx context.Context, key string, value any, ttl time.Duration) *redis.BoolCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return redis.NewBoolResult(false, err)
	}
	if _, exists := f.lookup(key); exists {
		return redis.NewBoolResult(false, nil)
	}
	f.entries[key] = fakeRedisValue{value: value.(string), expires: f.now.Add(ttl)}
	return redis.NewBoolResult(true, nil)
}

func (f *fakeRedis) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return redis.NewCmdResult(nil, err)
	}
	key := keys[0]
	value, exists := f.lookup(key)
	if !exists || value != args[0].(string) {
		return redis.NewCmdResult(int64(0), nil)
	}
	switch script {
	case redisComplete:
		ttl := time.Duration(args[1].(int64)) * time.Millisecond
		f.entries[key] = fakeRedisValue{value: args[2].(string), expires: f.now.Add(ttl)}
		return redis.NewCmdResult("OK", nil)
	case redisRelease:
		delete(f.entries, key)
		return redis.NewCmdResult(int64(1), nil)
	default:
		return redis.NewCmdResult(nil, fmt.Errorf("unexpected script %q", script))
	}
}

func TestRedisStoreDeduplicatesAcrossReplicas(t *testing.T) {
	backend := newFakeRedis()
	options := RedisStoreOptions{Prefix: "test", TTL: time.Minute, PollInterval: time.Millisecond}
	first := NewRedisStore(backend, options)
	second := NewRedisStore(backend, options)
	defer first.Close()
	defer second.Close()
	key := Key{TenantID: "tenant", UserID: "user", ProjectID: "project", Operation: "POST /create", Token: "token"}
	fingerprint := [32]byte{1}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	entered := make(chan struct{})
	release := make(chan struct{})
	var executions atomic.Int32
	var group sync.WaitGroup
	defer group.Wait()
	defer closeIfOpen(release)
	for _, store := range []*RedisStore{first, second} {
		for range 10 {
			group.Go(func() {
				response, err := store.Do(ctx, key, fingerprint, func() (Response, error) {
					if executions.Add(1) == 1 {
						close(entered)
					}
					<-release
					return Response{
						Status: http.StatusCreated,
						Header: http.Header{"Location": {"/item/1"}},
						Body:   []byte("created"),
					}, nil
				})
				if err != nil || response.Status != http.StatusCreated ||
					response.Header.Get(
						"Location",
					) != "/item/1" || string(response.Body) != "created" {
					t.Errorf("unexpected response: %+v, %v", response, err)
				}
			})
		}
	}
	awaitRedisSignal(t, ctx, entered)
	awaitRedisSignal(t, ctx, backend.read)
	never := func() (Response, error) {
		t.Error("conflicting duplicate executed")
		return Response{}, nil
	}
	_, err := second.Do(ctx, key, [32]byte{2}, never)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("in-progress conflict: %v", err)
	}
	close(release)
	group.Wait()
	if executions.Load() != 1 {
		t.Fatalf("executions = %d", executions.Load())
	}
	_, err = second.Do(ctx, key, [32]byte{2}, never)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("completed conflict: %v", err)
	}
	backend.advance(time.Minute)
	response, err := second.Do(ctx, key, [32]byte{2}, func() (Response, error) {
		return Response{Status: http.StatusAccepted}, nil
	})
	if err != nil || response.Status != http.StatusAccepted {
		t.Fatalf("retry after TTL: %+v, %v", response, err)
	}
}

func TestRedisStoreReleasesUncachedResults(t *testing.T) {
	failure := errors.New("handler failed")
	for _, tc := range []struct {
		name   string
		status int
		body   string
		err    error
		panics bool
	}{
		{name: "error", err: failure},
		{name: "HTTP failure", status: http.StatusBadRequest},
		{name: "oversized body", status: http.StatusOK, body: "oversized"},
		{name: "panic", panics: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := NewRedisStore(newFakeRedis(), RedisStoreOptions{MaxBodyBytes: 3})
			defer store.Close()
			key := Key{Token: "token"}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			func() {
				if tc.panics {
					defer func() {
						if recover() != "execution panic" {
							t.Error("execution panic did not propagate")
						}
					}()
				}
				response, err := store.Do(ctx, key, [32]byte{1}, func() (Response, error) {
					cancel()
					if tc.panics {
						panic("execution panic")
					}
					return Response{Status: tc.status, Body: []byte(tc.body)}, tc.err
				})
				if err != tc.err || response.Status != tc.status || string(response.Body) != tc.body {
					t.Fatalf("unexpected result: %+v, %v", response, err)
				}
			}()
			response, err := store.Do(t.Context(), key, [32]byte{2}, func() (Response, error) {
				return Response{Status: http.StatusCreated, Body: []byte("new")}, nil
			})
			if err != nil || response.Status != http.StatusCreated || string(response.Body) != "new" {
				t.Fatalf("retry failed: %+v, %v", response, err)
			}
		})
	}
}

func TestRedisStoreWaitCancellationAndClose(t *testing.T) {
	for _, action := range []string{"cancel", "close"} {
		t.Run(action, func(t *testing.T) {
			backend := newFakeRedis()
			owner := NewRedisStore(backend, RedisStoreOptions{})
			defer owner.Close()
			waiter := NewRedisStore(backend, RedisStoreOptions{PollInterval: time.Hour})
			defer waiter.Close()
			key := Key{Token: "token"}
			ctx, stop := context.WithTimeout(t.Context(), 3*time.Second)
			defer stop()
			waitCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			release := make(chan struct{})
			defer closeIfOpen(release)
			entered := make(chan struct{})
			ownerErr := make(chan error, 1)
			go func() {
				_, err := owner.Do(ctx, key, [32]byte{}, func() (Response, error) {
					close(entered)
					<-release
					return Response{Status: http.StatusOK}, nil
				})
				ownerErr <- err
			}()
			awaitRedisSignal(t, ctx, entered)
			waitErr := make(chan error, 1)
			go func() {
				_, err := waiter.Do(waitCtx, key, [32]byte{}, func() (Response, error) {
					t.Error("waiting duplicate executed")
					return Response{}, nil
				})
				waitErr <- err
			}()
			awaitRedisSignal(t, ctx, backend.read)
			want := context.Canceled
			if action == "cancel" {
				cancel()
			} else {
				_ = waiter.Close()
				want = ErrClosed
			}
			select {
			case err := <-waitErr:
				if !errors.Is(err, want) {
					t.Fatalf("wait returned %v, want %v", err, want)
				}
			case <-ctx.Done():
				t.Fatal("waiting duplicate did not return")
			}
			close(release)
			if err := <-ownerErr; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRedisStoreExpiredOwnerCannotChangeNewClaim(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadRequest} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			backend := newFakeRedis()
			store := NewRedisStore(backend, RedisStoreOptions{LockTTL: time.Second})
			defer store.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			key := Key{Token: "token"}
			entered := make(chan struct{})
			release := make(chan struct{})
			defer closeIfOpen(release)
			oldErr := make(chan error, 1)
			go func() {
				_, err := store.Do(ctx, key, [32]byte{1}, func() (Response, error) {
					close(entered)
					<-release
					return Response{Status: status}, nil
				})
				oldErr <- err
			}()
			awaitRedisSignal(t, ctx, entered)
			backend.advance(time.Second)
			response, err := store.Do(ctx, key, [32]byte{2}, func() (Response, error) {
				close(release)
				if err := <-oldErr; err != nil {
					t.Error(err)
				}
				_, err := store.Do(ctx, key, [32]byte{1}, func() (Response, error) {
					t.Error("old owner removed newer claim")
					return Response{}, nil
				})
				if !errors.Is(err, ErrConflict) {
					t.Errorf("new claim lost: %v", err)
				}
				return Response{Status: http.StatusCreated}, nil
			})
			if err != nil || response.Status != http.StatusCreated {
				t.Fatalf("new claim failed: %+v, %v", response, err)
			}
			replay, err := store.Do(ctx, key, [32]byte{2}, func() (Response, error) {
				t.Error("new owner could not cache its result")
				return Response{}, nil
			})
			if err != nil || replay.Status != http.StatusCreated {
				t.Fatalf("new record lost: %+v, %v", replay, err)
			}
		})
	}
}

func awaitRedisSignal(t *testing.T, ctx context.Context, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-ctx.Done():
		t.Fatal("timed out waiting for Redis store")
	}
}

func closeIfOpen(signal chan struct{}) {
	select {
	case <-signal:
	default:
		close(signal)
	}
}
