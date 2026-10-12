package redis

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

type Redis = redis.UniversalClient

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
