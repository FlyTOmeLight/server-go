package service

import (
	"context"
	"errors"
	"time"

	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/idempotence"
	core "github.com/FlyTOmeLight/server-go/pkg"

	"github.com/FlyTOmeLight/server-go/examples/example-service1/internal/app/dataset"
	"github.com/FlyTOmeLight/server-go/examples/example-service1/internal/table"
)

type ExampleService struct {
	dataset *dataset.Controller
	control *idempotence.Controller
}

var _ host.Service = (*ExampleService)(nil)

// NewExampleService borrows the database; the application owns its cleanup.
func NewExampleService(ctx context.Context, database *db.DB, autoMigrate bool) (*ExampleService, error) {
	if database == nil {
		return nil, errors.New("dataset service requires a database")
	}
	store := table.New(database)
	if autoMigrate {
		if err := store.AutoMigrate(ctx); err != nil {
			return nil, err
		}
	}
	return &ExampleService{
		dataset: dataset.NewController(dataset.New(store)),
		control: idempotence.New(time.Hour),
	}, nil
}

func (s *ExampleService) RegisterRoutes(router *core.RouterGroup) error {
	s.dataset.RegisterRoute(router, s.control)
	return nil
}

func (s *ExampleService) Start(context.Context) error { return nil }

func (s *ExampleService) Stop(context.Context) error { return s.control.Close() }
