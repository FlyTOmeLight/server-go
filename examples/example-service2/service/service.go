package service

import (
	"github.com/FlyTOmeLight/server-go/host"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type ExampleService struct {
	host.Service
	engine *core.Engine
}

var _ host.Service = &ExampleService{}

func NewExampleService() *ExampleService {
	return &ExampleService{}
}

func (s *ExampleService) Initialize(e *core.Engine) {
	s.engine = e
	// 初始化所有必备启动资源
}

func (s *ExampleService) StartAsync() {

}

func (s *ExampleService) StopAsync() {

}
