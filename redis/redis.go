package redis

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

type Redis = redis.UniversalClient

// XReadArgs describes a stream read: the streams and IDs to follow, the entry
// count, and how long to block waiting for new entries.
type XReadArgs = redis.XReadArgs

// Nil is the driver's no-result sentinel. Get returns it when the key does not
// exist, and a blocking XRead returns it when the wait times out without new
// entries. Compare with errors.Is.
var Nil = redis.Nil

func NewRedis(ctx context.Context, config *Config) (Redis, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}

	client := redis.NewUniversalClient(config)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.Join(err, client.Close())
	}
	return client, nil
}

// NewRedisLazy returns a client without verifying the connection. The client
// connects on first use and reconnects automatically, so a service can start
// while Redis is unreachable and recover once it is back. Use NewRedis to fail
// fast when the server must be reachable at startup.
func NewRedisLazy(config *Config) (Redis, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}
	return redis.NewUniversalClient(config), nil
}
