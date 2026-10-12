package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/FlyTOmeLight/server-go/conf"
)

const defaultName = "MongoDB"

type Module struct {
	Name   string
	Config *Config
	Client *DB
}

func (m *Module) ConfigKey() string {
	if m.Name == "" {
		m.Name = defaultName
	}
	return m.Name
}

func (m *Module) Initialize() error {
	if m.Config == nil {
		err := conf.UnmarshalKey(m.ConfigKey(), &m.Config)
		if err != nil {
			return fmt.Errorf("UnmarshalMongoDBConfigError: %v", err)
		}
		if m.Config == nil {
			return fmt.Errorf("initialize mongo db module failed, config is empty")
		}
	}

	if err := m.Config.check(); err != nil {
		return err
	}

	var err error
	m.Client, err = NewClientWithConfig(m.Config)
	if err != nil {
		return fmt.Errorf("NewMongoDBError: %v", err)
	}

	SetDefault(m.Client)
	defaultMongoDB.SetDefaultCollection(defaultMongoDB.config.Collection)

	// 初始化mongo参数InternalQueryExecMaxBlockingSortBytes
	if m.Config.InternalQueryExecMaxBlockingSortBytes > 0 {
		setQueryCacheResult := m.Client.Database("admin").RunCommand(
			context.Background(),
			bson.D{{Key: "setParameter", Value: 1}, {Key: "internalQueryExecMaxBlockingSortBytes", Value: m.Config.InternalQueryExecMaxBlockingSortBytes}},
		)
		if setQueryCacheResult.Err() != nil {
			return setQueryCacheResult.Err()
		}
	}

	return err
}

func (m *Module) Close() {
	if m.Client != nil {
		_ = m.Client.Disconnect(context.TODO())
	}
}

func (m *Module) Reload() error {
	oldDB := m.Client
	m.Config = nil

	err := m.Initialize()
	if err != nil {
		return err
	}

	if oldDB != nil {
		if err = oldDB.Disconnect(context.TODO()); err != nil {
			return err
		}
	}

	return nil
}
