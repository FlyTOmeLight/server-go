package cli

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/gookit/goutil/fsutil"
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

	setLog()
	client, err := newClient()
	if err != nil {
		return err
	}
	Config.Client = client
	return nil
}

func setLog() {
	defaultConfig := &logger.Config{
		Outputs: []logger.Output{
			{
				Type:  "Console",
				Level: "Debug",
			},
			{
				Type:       "File",
				Level:      "Info",
				Path:       "./logs/service/info.log",
				Rotation:   true,
				MaxAge:     7,
				MaxBackups: 5,
			},
			{
				Type:       "File",
				Level:      "Warn",
				Path:       "./logs/service/error.log",
				Rotation:   true,
				MaxAge:     30,
				MaxBackups: 10,
			},
		},
	}
	if viper.IsSet("log.handler") {
		str := viper.GetString("log.handler")
		outputs := []logger.Output{}
		switch str {
		case "NONE":
			outputs = []logger.Output{}
		case "STDOUT":
			outputs = []logger.Output{
				{
					Type:  "Console",
					Level: "Debug",
				},
			}
		case "FILE":
			outputs = []logger.Output{
				{
					Type:       "File",
					Level:      "Info",
					Path:       "./logs/service/info.log",
					Rotation:   true,
					MaxAge:     7,
					MaxBackups: 5,
				},
			}
		}
		defaultConfig.Outputs = outputs
	}

	if err := logger.SetDefaultConfig(defaultConfig); err != nil {
		logger.Errorf(context.Background(), "CommandLogConfigError: %v", err)
	}
}

func newClient() (*sdk.Client, error) {
	configuration := sdk.Config{
		Endpoint: viper.GetString("core.endpoint"),
		Timeout:  30,
	}

	ak := viper.GetString("core.access_key")
	sk := viper.GetString("core.secret_key")
	if ak != "" || sk != "" {
		configuration.Credentials = &sdk.Credentials{
			AuthType:        sdk.AuthTypeAKSK,
			AccessKeyId:     ak,
			SecretAccessKey: sk,
		}
	}

	apiKey := viper.GetString("core.api_key")
	if apiKey != "" {
		if ak != "" || sk != "" {
			return nil, errors.New("CLI config must select one authentication mode")
		}
		configuration.Credentials = &sdk.Credentials{
			AuthType: sdk.AuthTypeAPIKey,
			APIKey:   apiKey,
		}
	}

	if viper.IsSet("core.timeout") {
		timeout := viper.GetInt("core.timeout")
		configuration.Timeout = timeout
	}

	if viper.IsSet("core.retry") {
		retry := viper.GetInt("core.retry")
		configuration.Retry = retry
	}

	return sdk.NewClientWithConfig(&configuration)
}

func initConfig(name string) {
	if ConfigName == "" {
		ConfigName = name
	}
	if ConfigFilePath == "" {
		ConfigFilePath = os.Getenv("HOME") + "/." + strings.ToLower(ConfigName) + "/config.yaml"
	}
	if ConfigEnvPrefix == "" {
		ConfigEnvPrefix = strings.ToUpper(ConfigName)
	}

	if !fsutil.FileExists(ConfigFilePath) {
		_, err := fsutil.CreateFile(ConfigFilePath, os.ModePerm, os.ModePerm)
		if err != nil {
			logger.Errorf(context.Background(), "CommandConfigInitError create file: %v", err)
			return
		}
	}

	viper.SetEnvPrefix(ConfigEnvPrefix)
	viper.SetConfigFile(ConfigFilePath)
	viper.SetConfigType("yaml")

	err := viper.ReadInConfig()
	if err != nil {
		logger.Errorf(context.Background(), "CommandConfigInitError read in: %v", err)
		return
	}

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		logger.Infof(context.Background(), "CommandConfigChanged: %s", e.Name)
	})
}
