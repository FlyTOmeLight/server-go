package service

import (
	"context"

	"github.com/FlyTOmeLight/server-go/examples/minimal-service/internal/app/hello"
	"github.com/FlyTOmeLight/server-go/host"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type ExampleService struct{ controller *hello.Controller }

var _ host.Service = (*ExampleService)(nil)

func NewExampleService() *ExampleService { return &ExampleService{controller: hello.NewController()} }

func (s *ExampleService) RegisterRoutes(router *core.RouterGroup) error {
	s.controller.RegisterRoutes(router)
	return nil
}

func (s *ExampleService) Start(context.Context) error { return nil }

func (s *ExampleService) Stop(context.Context) error { return nil }
