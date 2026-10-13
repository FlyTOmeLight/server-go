package service

import (
	"context"

	"github.com/FlyTOmeLight/server-go/examples/streaming-service/internal/app/greeting"
	"github.com/FlyTOmeLight/server-go/host"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type Generate func(context.Context, string, func(string) error) error

type Service struct{ controller *greeting.Controller }

var _ host.Service = (*Service)(nil)

func New(generate Generate) *Service {
	return &Service{controller: greeting.NewController(greeting.New(greeting.Generate(generate)))}
}

func (s *Service) RegisterRoutes(router *core.RouterGroup) error {
	return s.controller.RegisterRoutes(router)
}

func (s *Service) Start(context.Context) error { return nil }

func (s *Service) Stop(context.Context) error { return nil }
