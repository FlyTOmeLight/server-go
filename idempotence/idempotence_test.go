package idempotence

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoConcurrentReplayAndConflict(t *testing.T) {
	controller := New(time.Minute)
	defer controller.Close()
	key := Key{TenantID: "tenant", UserID: "alice", Operation: "POST /create", Token: "token"}
	fingerprint := sha256.Sum256([]byte("request"))
	entered := make(chan struct{})
	release := make(chan struct{})
	var executions atomic.Int32
	execute := func() (Response, error) {
		executions.Add(1)
		close(entered)
		<-release
		return Response{
			Status: http.StatusCreated,
			Header: http.Header{"Location": {"/item/1"}},
			Body:   []byte("created"),
		}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := controller.Do(ctx, key, fingerprint, execute)
			if err != nil || response.Status != http.StatusCreated ||
				response.Header.Get("Location") != "/item/1" ||
				string(response.Body) != "created" {
				t.Errorf("unexpected replay: %+v, %v", response, err)
			}
		}()
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("operation did not start")
	}
	_, err := controller.Do(ctx, key, sha256.Sum256([]byte("different")), execute)
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected conflict, got %v", err)
	}
	close(release)
	wg.Wait()
	if executions.Load() != 1 {
		t.Fatalf("operation executed %d times", executions.Load())
	}
	// Public results are copies; mutation cannot corrupt future replays.
	response, err := controller.Do(ctx, key, fingerprint, execute)
	if err != nil {
		t.Fatal(err)
	}
	response.Header.Set("Location", "modified")
	response.Body[0] = 'X'
	replayed, err := controller.Do(ctx, key, fingerprint, execute)
	if err != nil || replayed.Header.Get("Location") != "/item/1" || string(replayed.Body) != "created" {
		t.Fatalf("mutable replay corrupted cache: %+v, %v", replayed, err)
	}
}

func TestWaitCancellationAndClose(t *testing.T) {
	for _, action := range []string{"cancel", "close"} {
		t.Run(action, func(t *testing.T) {
			controller := New(time.Minute)
			defer controller.Close()
			key := Key{Token: "token"}
			fingerprint := sha256.Sum256(nil)
			entered := make(chan struct{})
			release := make(chan struct{})
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				_, _ = controller.Do(context.Background(), key, fingerprint, func() (Response, error) {
					close(entered)
					<-release
					return Response{Status: http.StatusOK}, nil
				})
			}()
			defer func() { close(release); <-finished }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("operation did not start")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, err := controller.Do(ctx, key, fingerprint, func() (Response, error) {
					t.Error("duplicate executed")
					return Response{}, nil
				})
				result <- err
			}()
			want := context.Canceled
			if action == "cancel" {
				cancel()
			} else {
				_ = controller.Close()
				want = ErrClosed
			}
			select {
			case err := <-result:
				if !errors.Is(err, want) {
					t.Fatalf("expected %v, got %v", want, err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("waiter did not return")
			}
		})
	}
}

func TestExpirationAndInstanceIsolation(t *testing.T) {
	first := New(5 * time.Millisecond)
	defer first.Close()
	second := New(time.Minute)
	defer second.Close()
	key := Key{Token: "token"}
	fingerprint := sha256.Sum256(nil)
	executions := 0
	execute := func() (Response, error) {
		executions++
		return Response{Status: http.StatusOK}, nil
	}
	if _, err := first.Do(context.Background(), key, fingerprint, execute); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Do(context.Background(), key, fingerprint, execute); err != nil {
		t.Fatal(err)
	}
	if executions != 2 {
		t.Fatal("controllers shared their caches")
	}
	deadline := time.Now().Add(time.Second)
	for executions == 2 && time.Now().Before(deadline) {
		_, err := first.Do(context.Background(), key, fingerprint, execute)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	if executions != 3 {
		t.Fatal("completed response did not expire")
	}
}

func TestPanickingOperationDoesNotStrandOrRepeat(t *testing.T) {
	controller := New(time.Minute)
	defer controller.Close()
	key := Key{Token: "token"}
	fingerprint := sha256.Sum256(nil)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("expected execute panic to propagate")
			}
		}()
		_, _ = controller.Do(
			context.Background(),
			key,
			fingerprint,
			func() (Response, error) { panic("unknown effect") },
		)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := controller.Do(ctx, key, fingerprint, func() (Response, error) {
		t.Error("panicking operation executed again")
		return Response{}, nil
	})
	if !errors.Is(err, ErrExecutionFailed) {
		t.Fatalf("expected execution failure, got %v", err)
	}
}

func TestControllerEvictsOldestCompletion(t *testing.T) {
	controller := NewWithOptions(Options{MaxEntries: 2})
	defer controller.Close()
	executions := 0
	execute := func() (Response, error) {
		executions++
		return Response{Status: http.StatusCreated}, nil
	}
	for _, tc := range []struct {
		token string
		want  int
	}{
		{token: "first", want: 1},
		{token: "second", want: 2},
		{token: "first", want: 2},
		{token: "third", want: 3},
		{token: "second", want: 3},
		{token: "first", want: 4},
	} {
		_, err := controller.Do(t.Context(), Key{Token: tc.token}, [32]byte{}, execute)
		if err != nil || executions != tc.want {
			t.Fatalf("token %q: executions=%d, want %d, error=%v", tc.token, executions, tc.want, err)
		}
	}
}

func TestControllerSharesOversizedResponseWithoutCaching(t *testing.T) {
	controller := NewWithOptions(Options{MaxEntries: 1, MaxBodyBytes: 3})
	defer controller.Close()
	key := Key{Token: "token"}
	entered := make(chan struct{})
	release := make(chan struct{})
	owner := make(chan string, 1)
	waiter := make(chan string, 1)
	waiting := make(chan struct{})
	execute := func() (Response, error) {
		close(entered)
		<-release
		return Response{Status: http.StatusOK, Body: []byte("large")}, nil
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	go func() {
		response, err := controller.Do(ctx, key, [32]byte{}, execute)
		if err != nil {
			t.Error(err)
		}
		owner <- string(response.Body)
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("operation did not start")
	}
	go func() {
		response, err := controller.Do(
			observedWaitContext{Context: ctx, waiting: waiting},
			key,
			[32]byte{},
			func() (Response, error) {
				t.Error("duplicate executed")
				return Response{}, nil
			},
		)
		if err != nil {
			t.Error(err)
		}
		waiter <- string(response.Body)
	}()
	select {
	case <-waiting:
	case <-ctx.Done():
		t.Fatal("duplicate did not wait")
	}
	_, err := controller.Do(ctx, Key{Token: "other"}, [32]byte{}, func() (Response, error) {
		t.Error("operation ran at capacity")
		return Response{}, nil
	})
	if !errors.Is(err, ErrCapacity) {
		t.Fatalf("expected capacity error, got %v", err)
	}
	close(release)
	for _, result := range []chan string{owner, waiter} {
		select {
		case body := <-result:
			if body != "large" {
				t.Fatalf("unexpected body %q", body)
			}
		case <-ctx.Done():
			t.Fatal("operation did not return")
		}
	}
	response, err := controller.Do(ctx, key, [32]byte{}, func() (Response, error) {
		return Response{Status: http.StatusCreated, Body: []byte("new")}, nil
	})
	if err != nil || response.Status != http.StatusCreated {
		t.Fatalf("oversized response blocked retry: %+v, %v", response, err)
	}
}

type observedWaitContext struct {
	context.Context
	waiting chan struct{}
}

func (c observedWaitContext) Done() <-chan struct{} {
	close(c.waiting)
	return c.Context.Done()
}
