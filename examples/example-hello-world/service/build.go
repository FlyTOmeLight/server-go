package service

import (
	"context"

	"github.com/FlyTOmeLight/server-go/app"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/swagger"
)

type Config struct {
	DB db.Config
}

// Build assembles the standalone service. Combined applications construct
// NewHelloWorldService with their own shared database instead.
func Build(ctx context.Context, runtime *app.Runtime, cfg *Config) error {
	database := &db.Module{Config: &cfg.DB}
	if err := runtime.Initialize(ctx, database); err != nil {
		return err
	}
	svc, err := NewHelloWorldService(ctx, database.DB, cfg.DB.AutoMigrate)
	if err != nil {
		return err
	}
	swagger.DocPrefix = "HelloWorld"
	return runtime.AddService(svc)
}
