package service

import (
	"context"

	"github.com/FlyTOmeLight/server-go/app"
	"github.com/FlyTOmeLight/server-go/db"
)

type Config struct{ DB db.Config }

func Build(ctx context.Context, runtime *app.Runtime, cfg *Config) error {
	database := &db.Module{Config: &cfg.DB}
	if err := runtime.Initialize(ctx, database); err != nil {
		return err
	}
	notes, err := New(ctx, database.DB, cfg.DB.AutoMigrate)
	if err != nil {
		return err
	}
	return runtime.AddService(notes)
}
