package mongodb

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
)

type Module struct {
	Config *Config
	Client *DB
}

func (m *Module) Initialize(ctx context.Context) error {
	if m.Client != nil {
		return errors.New("mongodb module is already initialized")
	}
	client, err := NewClientWithConfig(ctx, m.Config)
	if err != nil {
		return err
	}
	if m.Config.InternalQueryExecMaxBlockingSortBytes > 0 {
		err := client.Database("admin").RunCommand(ctx, bson.D{
			{Key: "setParameter", Value: 1},
			{
				Key:   "internalQueryExecMaxBlockingSortBytes",
				Value: m.Config.InternalQueryExecMaxBlockingSortBytes,
			},
		}).Err()
		if err != nil {
			return errors.Join(err, client.Disconnect(ctx))
		}
	}
	m.Client = client
	return nil
}

func (m *Module) Close(ctx context.Context) error {
	if m.Client == nil {
		return nil
	}
	client := m.Client
	m.Client = nil
	return client.Disconnect(ctx)
}
