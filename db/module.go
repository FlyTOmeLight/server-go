package db

import (
	"context"
	"errors"
)

type Module struct {
	Config *Config
	DB     *DB
}

func (m *Module) Initialize(ctx context.Context) error {
	if m.DB != nil {
		return errors.New("database module is already initialized")
	}
	client, err := NewDB(ctx, m.Config)
	if err != nil {
		return err
	}
	m.DB = client
	return nil
}

func (m *Module) Close(ctx context.Context) error {
	if m.DB == nil {
		return nil
	}
	connection, err := m.DB.DB()
	if err != nil {
		return err
	}
	// database/sql.Close drains its owned connections and has no context variant.
	if err := connection.Close(); err != nil {
		return err
	}
	m.DB = nil
	return nil
}
