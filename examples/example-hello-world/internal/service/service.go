package service

import (
	"context"
	"errors"
	"time"

	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/idempotence"
	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/internal/app/module"
	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/internal/table"
)

type HelloWorldService struct {
	cache            *idempotence.Controller
	moduleController *module.ModuleController
}

var _ host.Service = (*HelloWorldService)(nil)

func NewHelloWorldService(ctx context.Context, database *db.DB, autoMigrate bool) (*HelloWorldService, error) {
	if database == nil {
		return nil, errors.New("service requires a database")
	}
	if autoMigrate {
		if err := table.AutoMigrate(ctx, database); err != nil {
			return nil, err
		}
	}
	return &HelloWorldService{
		cache:            idempotence.New(time.Hour),
		moduleController: module.NewModuleController(database),
	}, nil
}

// RegisterRoutes can rebuild HTTP handlers without creating resources.
func (s *HelloWorldService) RegisterRoutes(root *gin.RouterGroup) error {
	group := root.Group("/helloworld/internal")
	s.moduleController.RegisterRoutes(group, s.cache)
	return nil
}

func (s *HelloWorldService) Start(context.Context) error { return nil }

func (s *HelloWorldService) Stop(context.Context) error { return s.cache.Close() }
