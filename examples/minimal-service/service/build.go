package service

import (
	"context"

	"github.com/FlyTOmeLight/server-go/app"
)

type Config struct{}

func Build(_ context.Context, runtime *app.Runtime, _ *Config) error {
	return runtime.AddService(NewExampleService())
}
