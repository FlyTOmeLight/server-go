package service

import (
	"context"

	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"

	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/internal/app/module"
	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/internal/client"
	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/internal/table"
)

type HelloWorldService struct {
	engine *core.Engine
	config *Config
}

type Config struct {
	// 该结构体放服务所需的配置结构
}

var _ host.Service = &HelloWorldService{}

func NewHelloWorldService(config *Config) *HelloWorldService {
	if config == nil {
		config = &Config{}
	}
	return &HelloWorldService{config: config}
}

func (s *HelloWorldService) Initialize(e *core.Engine) {
	s.engine = e
	// 初始化clients
	err := client.Initialize()
	if err != nil {
		logger.Fatalf(context.Background(), "InitialzeClientError: %v", err)
	}
	table.AutoMigrate()
}

func (s *HelloWorldService) StartAsync() {
	g := s.engine.Group("/helloworld/internal")
	module.RegisterRoute(g)
}

func (s *HelloWorldService) StopAsync() {

}
