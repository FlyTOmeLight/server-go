package service

import (
	"context"

	"github.com/FlyTOmeLight/server-go/app"
	"github.com/FlyTOmeLight/server-go/examples/streaming-service/internal/app/greeting"
)

type Config struct{}

func Build(_ context.Context, runtime *app.Runtime, _ *Config) error {
	return runtime.AddService(New(greeting.GenerateGreeting))
}
