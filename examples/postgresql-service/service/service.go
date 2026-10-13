package service

import (
	"context"
	"errors"

	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/examples/postgresql-service/internal/app/note"
	"github.com/FlyTOmeLight/server-go/examples/postgresql-service/internal/table"
	"github.com/FlyTOmeLight/server-go/host"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type Notes struct{ controller *note.Controller }

var _ host.Service = (*Notes)(nil)

// New borrows the database; its owner closes it after the HTTP host has drained.
func New(ctx context.Context, database *db.DB, autoMigrate bool) (*Notes, error) {
	if database == nil {
		return nil, errors.New("notes require a database")
	}
	store := table.New(database)
	if autoMigrate {
		if err := store.AutoMigrate(ctx); err != nil {
			return nil, err
		}
	}
	return &Notes{controller: note.NewController(note.New(store))}, nil
}

func (s *Notes) RegisterRoutes(router *core.RouterGroup) error {
	s.controller.RegisterRoutes(router)
	return nil
}

func (s *Notes) Start(context.Context) error { return nil }

func (s *Notes) Stop(context.Context) error { return nil }
