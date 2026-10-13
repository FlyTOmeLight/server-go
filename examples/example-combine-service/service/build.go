package service

import (
	"context"

	"github.com/FlyTOmeLight/server-go/app"
	"github.com/FlyTOmeLight/server-go/db"
	hello "github.com/FlyTOmeLight/server-go/examples/example-hello-world/service"
	dataset "github.com/FlyTOmeLight/server-go/examples/example-service1/service"
	service2 "github.com/FlyTOmeLight/server-go/examples/example-service2/service"
)

type Config struct{ DB db.Config }

func Build(ctx context.Context, runtime *app.Runtime, cfg *Config) error {
	database := &db.Module{Config: &cfg.DB}
	if err := runtime.Initialize(ctx, database); err != nil {
		return err
	}
	datasets, err := dataset.NewExampleService(ctx, database.DB, cfg.DB.AutoMigrate)
	if err != nil {
		return err
	}
	if err = runtime.AddService(datasets); err != nil {
		return err
	}
	modules, err := hello.NewHelloWorldService(ctx, database.DB, cfg.DB.AutoMigrate)
	if err != nil {
		return err
	}
	if err = runtime.AddService(modules); err != nil {
		return err
	}
	return runtime.AddService(service2.NewExampleService())
}
