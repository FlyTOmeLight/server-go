package core

import (
	"context"
	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/gookit/goutil/fsutil"
	"os"
	"path/filepath"
)

var (
	HTTPLogger *logger.Logger
	DBLogger   *logger.Logger
)

const (
	configDirPath        = "logger"
	configPathForService = "logger/service.toml"
)

func init() {
	var err error
	HTTPLogger, err = logger.NewLogger(loggerConfig("http"))
	if err != nil {
		panic(err)
	}
	DBLogger, err = logger.NewLogger(loggerConfig("db"))
	if err != nil {
		panic(err)
	}

	// set log config for service
	exists := fsutil.FileExists(filepath.Join("conf", configPathForService))
	if exists {
		err := logger.SetDefaultConfigWithFile(filepath.Join("conf", configPathForService))
		if err != nil {
			logger.Errorf(context.Background(), "LoggerSetConfigFileError path: %s, err: %v", configPathForService, err)
		}
	}
}

func loggerConfig(fileName string) *logger.Config {
	filePath := filepath.Join(configDirPath, fileName+".toml")
	exists := fsutil.FileExists(filepath.Join("conf", filePath))
	if exists {
		config, err := logger.ParseConfigFile(filepath.Join("conf", filePath))
		if err == nil {
			return config
		}
		logger.Errorf(context.Background(), "LoggerSetConfigFileError path: %s, err: %v", filePath, err)
	}
	consoleLevel := os.Getenv("CONSOLE_LOG_LEVEL")
	if consoleLevel == "" {
		consoleLevel = "Debug"
	}

	return &logger.Config{
		Outputs: []logger.Output{
			{
				Type:  "Console",
				Level: consoleLevel,
			},
			{
				Type:       "File",
				Level:      "Info",
				Path:       "./logs/trace/info.log",
				Rotation:   true,
				MaxAge:     7,
				MaxBackups: 5,
			},
			{
				Type:       "File",
				Level:      "Warn",
				Path:       "./logs/trace/error.log",
				Rotation:   true,
				MaxAge:     30,
				MaxBackups: 10,
			},
		},
	}
}
