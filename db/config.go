package db

import (
	"errors"
	"fmt"
)

const (
	DriverMySQL      = "MySQL"
	DriverSQLite     = "SQLite"
	DriverPostgreSQL = "PostgreSQL"
	DriverClickHouse = "ClickHouse"
	DriverKingbase   = "Kingbase"
	DriverGaussDB    = "GaussDB"
)

type Config struct {
	Driver          string
	Host            string
	Port            int
	User            string
	Password        string
	DSNParams       string
	ConnMaxIdleTime int
	ConnMaxLifeTime int
	MaxIdleConns    int
	MaxOpenConns    int
	DB              string
	AutoMigrate     bool
}

func (c *Config) check() error {
	switch c.Driver {
	case DriverMySQL:
		return c.checkWithMySQL()
	case DriverSQLite:
		return c.checkWithSQLite()
	case DriverPostgreSQL:
		return c.checkWithPostgreSQL()
	case DriverClickHouse:
		return c.checkWithClickHouse()
	case DriverKingbase:
		return c.checkWithKingbase()
	case DriverGaussDB:
		return c.checkWithGaussDB()
	case "":
		return c.checkWithMySQL()
	default:
		return fmt.Errorf("unsupported database driver: %s", c.Driver)
	}
}

func (c *Config) checkWithMySQL() error {
	if c.Host == "" {
		return errors.New("DBConfig.Host is empty")
	}
	if c.Port == 0 {
		return errors.New("DBConfig.Port is empty")
	}
	if c.User == "" {
		return errors.New("DBConfig.User is empty")
	}
	if c.Password == "" {
		return errors.New("DBConfig.Password is empty")
	}
	if c.DB == "" {
		return errors.New("DBConfig.DB is empty")
	}
	return nil
}

func (c *Config) checkWithSQLite() error {
	if c.DB == "" {
		return errors.New("DBConfig.DB is empty")
	}
	return nil
}

func (c *Config) checkWithPostgreSQL() error {
	if c.Host == "" {
		return errors.New("DBConfig.Host is empty")
	}
	if c.Port == 0 {
		return errors.New("DBConfig.Port is empty")
	}
	if c.User == "" {
		return errors.New("DBConfig.User is empty")
	}
	if c.Password == "" {
		return errors.New("DBConfig.Password is empty")
	}
	if c.DB == "" {
		return errors.New("DBConfig.DB is empty")
	}
	return nil
}

func (c *Config) checkWithClickHouse() error {
	if c.Host == "" {
		return errors.New("DBConfig.Host is empty")
	}
	if c.Port == 0 {
		return errors.New("DBConfig.Port is empty")
	}
	if c.User == "" {
		return errors.New("DBConfig.User is empty")
	}
	if c.Password == "" {
		return errors.New("DBConfig.Password is empty")
	}
	if c.DB == "" {
		return errors.New("DBConfig.DB is empty")
	}
	return nil
}

func (c *Config) checkWithKingbase() error {
	if c.Host == "" {
		return errors.New("DBConfig.Host is empty")
	}
	if c.Port == 0 {
		return errors.New("DBConfig.Port is empty")
	}
	if c.User == "" {
		return errors.New("DBConfig.User is empty")
	}
	if c.Password == "" {
		return errors.New("DBConfig.Password is empty")
	}
	if c.DB == "" {
		return errors.New("DBConfig.DB is empty")
	}
	return nil
}

func (c *Config) checkWithGaussDB() error {
	if c.Host == "" {
		return errors.New("DBConfig.Host is empty")
	}
	if c.Port == 0 {
		return errors.New("DBConfig.Port is empty")
	}
	if c.User == "" {
		return errors.New("DBConfig.User is empty")
	}
	if c.Password == "" {
		return errors.New("DBConfig.Password is empty")
	}
	if c.DB == "" {
		return errors.New("DBConfig.DB is empty")
	}
	return nil
}
