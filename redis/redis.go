package redis

import (
	"errors"

	"github.com/redis/go-redis/v9"
)

type Redis = redis.UniversalClient

var defaultRedis Redis

func DefaultRedis() Redis {
	return defaultRedis
}

func SetDefaultRedis(redisClient Redis) {
	defaultRedis = redisClient
}

func SetDefaultRedisWithConfig(config *Config) error {
	var err error
	defaultRedis, err = NewRedis(config)
	return err
}

func NewRedis(config *Config) (Redis, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}

	return redis.NewUniversalClient(config), nil
}
