package idempotence

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const redisComplete = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
	return redis.call('PSETEX', KEYS[1], ARGV[2], ARGV[3])
end
return 0`

const redisRelease = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
	return redis.call('DEL', KEYS[1])
end
return 0`

// RedisClient is the subset of go-redis used by RedisStore.
type RedisClient interface {
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
}

// RedisStoreOptions configures shared idempotence records. LockTTL must cover
// the maximum execution duration. After that lease expires, callers can retry.
// A nonpositive MaxBodyBytes defaults to 1 MiB.
type RedisStoreOptions struct {
	Prefix       string
	TTL          time.Duration
	LockTTL      time.Duration
	PollInterval time.Duration
	MaxBodyBytes int
}

// RedisStore coordinates execution across replicas using a shared Redis client.
type RedisStore struct {
	client RedisClient
	opts   RedisStoreOptions
	closed chan struct{}
	once   sync.Once
}

var (
	_ Store       = (*RedisStore)(nil)
	_ RedisClient = (*redis.Client)(nil)
	_ RedisClient = redis.UniversalClient(nil)
)

type redisRecord struct {
	Owner       string    `json:"owner,omitempty"`
	Fingerprint [32]byte  `json:"fingerprint"`
	Response    *Response `json:"response,omitempty"`
}

// NewRedisStore constructs a store. The caller owns the Redis client's lifecycle.
func NewRedisStore(client RedisClient, opts RedisStoreOptions) *RedisStore {
	if opts.Prefix == "" {
		opts.Prefix = "idempotence"
	}
	if opts.TTL <= 0 {
		opts.TTL = defaultTTL
	}
	if opts.LockTTL <= 0 {
		opts.LockTTL = time.Minute
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = 50 * time.Millisecond
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = defaultMaxBodyBytes
	}
	return &RedisStore{client: client, opts: opts, closed: make(chan struct{})}
}

// Do claims the operation or waits for its Redis record to change.
func (s *RedisStore) Do(
	ctx context.Context,
	key Key,
	fingerprint [32]byte,
	execute func() (Response, error),
) (Response, error) {
	redisKey := s.redisKey(key)
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return Response{}, err
		}
		select {
		case <-s.closed:
			return Response{}, ErrClosed
		default:
		}

		value, err := s.client.Get(ctx, redisKey).Result()
		if errors.Is(err, redis.Nil) {
			owner, err := uuid.NewRandom()
			if err != nil {
				return Response{}, err
			}
			claim, err := json.Marshal(redisRecord{Owner: owner.String(), Fingerprint: fingerprint})
			if err != nil {
				return Response{}, err
			}
			claimed, err := s.client.SetNX(ctx, redisKey, string(claim), s.opts.LockTTL).Result()
			if err != nil {
				return Response{}, err
			}
			if claimed {
				return s.execute(ctx, redisKey, string(claim), fingerprint, execute)
			}
			continue
		}
		if err != nil {
			return Response{}, err
		}

		var record redisRecord
		if err := json.Unmarshal([]byte(value), &record); err != nil {
			return Response{}, err
		}
		if record.Fingerprint != fingerprint {
			return Response{}, ErrConflict
		}
		if record.Response != nil {
			return *record.Response, nil
		}

		if timer == nil {
			timer = time.NewTimer(s.opts.PollInterval)
		} else {
			timer.Reset(s.opts.PollInterval)
		}
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		case <-s.closed:
			return Response{}, ErrClosed
		case <-timer.C:
		}
	}
}

func (s *RedisStore) execute(
	ctx context.Context,
	key, claim string,
	fingerprint [32]byte,
	execute func() (Response, error),
) (response Response, err error) {
	var completed []byte
	defer func() {
		finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		var finalizeErr error
		if completed != nil {
			ttlMillis := max(int64(1), s.opts.TTL.Milliseconds())
			finalizeErr = s.client.Eval(finalizeCtx, redisComplete, []string{key}, claim, ttlMillis, string(completed)).
				Err()
		} else {
			finalizeErr = s.client.Eval(finalizeCtx, redisRelease, []string{key}, claim).Err()
		}
		if finalizeErr != nil {
			err = errors.Join(err, finalizeErr)
		}
	}()

	response, err = execute()
	if err == nil && response.Status < http.StatusBadRequest && len(response.Body) <= s.opts.MaxBodyBytes {
		completed, err = json.Marshal(redisRecord{Fingerprint: fingerprint, Response: &response})
	}
	return response, err
}

func (s *RedisStore) redisKey(key Key) string {
	var encoded []byte
	for _, part := range [...]string{key.TenantID, key.UserID, key.ProjectID, key.Operation, key.Token} {
		encoded = binary.AppendUvarint(encoded, uint64(len(part)))
		encoded = append(encoded, part...)
	}
	sum := sha256.Sum256(encoded)
	return s.opts.Prefix + ":" + hex.EncodeToString(sum[:])
}

// Close wakes waiting callers and leaves the shared Redis client open.
func (s *RedisStore) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}
