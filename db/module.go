package db

import (
	"fmt"

	"github.com/FlyTOmeLight/server-go/conf"
)

const defaultName = "DB"

type Module struct {
	Name   string
	Config *Config
	DB     *DB
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
			return fmt.Errorf("initialize db module failed, config is empty")
		}
	}

	if err := m.Config.check(); err != nil {
		return err
	}

	var err error
	m.DB, err = NewDB(m.Config)
	if err != nil {
		return fmt.Errorf("NewDBError: %v", err)
	}

	SetDefaultDB(m.DB)
	SetDefaultConfig(m.Config)
	return err
}

func (m *Module) Close() {
	sql, _ := m.DB.DB()
	sql.Close()
}

func (m *Module) Reload() error {
	oldDB := m.DB
	m.Config = nil

	err := m.Initialize()
	if err != nil {
		return err
	}

	sql, _ := oldDB.DB()
	return sql.Close()
}
