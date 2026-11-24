package cli

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/viper"

	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/FlyTOmeLight/server-go/sdk"
)

var ErrEndpointNotFound = errors.New(
	"CLI endpoint is empty, please run config set core.endpoint to set a valid endpoint",
)

var (
	ConfigName      string
	ConfigFilePath  string
	ConfigEnvPrefix string
	Config          CommandConfig
)

type CommandConfig struct {
	Client *sdk.Client
}

func SetConfig() error {
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	if viper.GetString("core.endpoint") == "" {
		return ErrEndpointNotFound
	}

	setLog(viper.GetViper())
	client, err := newClient(viper.GetViper())
	if err != nil {
		return err
	}
	if Config.Client != nil {
		if err := Config.Client.Close(); err != nil {
			return errors.Join(err, client.Close())
		}
	}
	Config.Client = client
	return nil
}

func logConfig(config *viper.Viper) *logger.Config {
	result := &logger.Config{}
	switch strings.ToLower(strings.TrimSpace(config.GetString("log.handler"))) {
	case "stdout", "stderr", "console":
		result.Outputs = []logger.Output{{Type: "Console", Level: "Debug"}}
	case "file":
		result.Outputs = []logger.Output{{
			Type:       "File",
			Level:      "Info",
			Path:       "./logs/service/info.log",
			Rotation:   true,
			MaxAge:     7,
			MaxBackups: 5,
		}}
	}
	return result
}

func setLog(config *viper.Viper) {
	if err := logger.SetDefaultConfig(logConfig(config)); err != nil {
		logger.Errorf(context.Background(), "CommandLogConfigError: %v", err)
	}
}

func newClient(config *viper.Viper) (*sdk.Client, error) {
	configuration := sdk.Config{
		Endpoint: config.GetString("core.endpoint"),
		Timeout:  30,
	}

	ak := config.GetString("core.access_key")
	sk := config.GetString("core.secret_key")
	if ak != "" || sk != "" {
		configuration.Credentials = &sdk.Credentials{
			AuthType:        sdk.AuthTypeAKSK,
			AccessKeyId:     ak,
			SecretAccessKey: sk,
		}
	}

	apiKey := config.GetString("core.api_key")
	if apiKey != "" {
		if ak != "" || sk != "" {
			return nil, errors.New("CLI config must select one authentication mode")
		}
		configuration.Credentials = &sdk.Credentials{
			AuthType: sdk.AuthTypeAPIKey,
			APIKey:   apiKey,
		}
	}

	if config.IsSet("core.timeout") {
		timeout := config.GetInt("core.timeout")
		configuration.Timeout = timeout
	}

	if config.IsSet("core.retry") {
		retry := config.GetInt("core.retry")
		configuration.Retry = retry
	}

	return sdk.NewClientWithConfig(&configuration)
}
