package mongodb

import "errors"

type Config struct {
	Host                                  string
	Port                                  int
	User                                  string
	Password                              string
	DB                                    string
	Collection                            string
	InternalQueryExecMaxBlockingSortBytes int
	ShardUser                             string
	ShardPassword                         string
	AutoMigrate                           bool
}

func (c *Config) check() error {
	if c.Host == "" {
		return errors.New("MongoDB Config.Host is empty")
	}
	if c.Port == 0 {
		return errors.New("MongoDB Config.Port is empty")
	}
	if c.User == "" {
		return errors.New("MongoDB Config.User is empty")
	}
	if c.Password == "" {
		return errors.New("MongoDB Config.Password is empty")
	}
	if c.DB == "" {
		return errors.New("MongoDB Config.DB is empty")
	}
	if c.Collection == "" {
		return errors.New("MongoDB Config.Collection is empty")
	}
	return nil
}
