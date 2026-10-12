package redis

import (
	"fmt"

	"github.com/FlyTOmeLight/server-go/conf"
)

const defaultName = "Redis"

type Module struct {
	Name   string
	Config *Config
	Redis  Redis
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
			return fmt.Errorf("UnmarshalConfigError: %v", err)
		}
		if m.Config == nil {
			return fmt.Errorf("initialize redis module failed, config is empty")
		}
	}

	var err error
	m.Redis, err = NewRedis(m.Config)
	if err != nil {
		return fmt.Errorf("NewRedisError: %v", err)
	}

	SetDefaultRedis(m.Redis)
	return err
}

func (m *Module) Close() {

}

func (m *Module) Reload() error {
	return nil
}
