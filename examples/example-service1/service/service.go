package service

import (
	"context"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/examples/example-service1/internal/app/dataset"
	"github.com/FlyTOmeLight/server-go/examples/example-service1/internal/table"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/idempotence"
	"github.com/gin-gonic/gin"
	"time"
)

type ExampleService struct {
	dataset *dataset.Service
	control *idempotence.Controller
}

var _ host.Service = (*ExampleService)(nil)

func NewExampleService(ctx context.Context, database *db.DB) (*ExampleService, error) {
	store := table.New(database)
	if err := store.AutoMigrate(ctx); err != nil {
		return nil, err
	}
	return &ExampleService{dataset: dataset.New(store), control: idempotence.New(time.Hour)}, nil
}
func (s *ExampleService) RegisterRoutes(router *gin.RouterGroup) error {
	s.dataset.RegisterRoute(router, s.control)
	return nil
}
func (s *ExampleService) Start(context.Context) error { return nil }
func (s *ExampleService) Stop(context.Context) error  { return s.control.Close() }
