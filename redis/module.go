package redis

import (
	"context"
	"errors"
)

type Module struct {
	Config *Config
	Redis  Redis
}

func (m *Module) Initialize(ctx context.Context) error {
	if m.Redis != nil {
		return errors.New("redis module is already initialized")
	}
	client, err := NewRedis(ctx, m.Config)
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
	client := m.Redis
	m.Redis = nil
	return client.Close()
}
