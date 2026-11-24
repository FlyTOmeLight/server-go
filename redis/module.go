package redis

import (
	"context"
	"errors"
)

type Module struct {
	Config *Config
	Redis  Redis

	// Lazy skips the startup connection check when true.
	Lazy bool
}

func (m *Module) Initialize(ctx context.Context) error {
	if m.Redis != nil {
		return errors.New("redis module is already initialized")
	}
	var client Redis
	var err error
	if m.Lazy {
		client, err = NewRedisLazy(m.Config)
	} else {
		client, err = NewRedis(ctx, m.Config)
	}
	if err != nil {
		return err
	}
	m.Redis = client
	return nil
}

func (m *Module) Close(ctx context.Context) error {
	if m.Redis == nil {
		return nil
	}
	if err := m.Redis.Close(); err != nil {
		return err
	}
	m.Redis = nil
	return nil
}
