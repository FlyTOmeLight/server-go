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
	database *db.DB
	cache    *idempotence.Controller
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
	return &HelloWorldService{database: database, cache: idempotence.New(time.Hour)}, nil
}

// RegisterRoutes can rebuild HTTP handlers without creating resources.
func (s *HelloWorldService) RegisterRoutes(root *gin.RouterGroup) error {
	group := root.Group("/helloworld/internal")
	module.RegisterRoute(group, s.database, s.cache)
	return nil
}

func (s *HelloWorldService) Start(context.Context) error { return nil }

func (s *HelloWorldService) Stop(context.Context) error { return s.cache.Close() }
