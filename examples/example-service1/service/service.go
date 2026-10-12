package service

import (
	"github.com/FlyTOmeLight/server-go/examples/example-service1/internal/table"

	"github.com/FlyTOmeLight/server-go/examples/example-service1/internal/app/dataset"
	"github.com/FlyTOmeLight/server-go/host"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type ExampleService struct {
	engine *core.Engine
}

var _ host.Service = &ExampleService{}

func NewExampleService() *ExampleService {
	return &ExampleService{}
}

func (s *ExampleService) Initialize(e *core.Engine) {
	s.engine = e
	table.AutoMigrateDataset()
}

func (s *ExampleService) StartAsync() {
	dataset.RegisterRoute(s.engine)
}

func (s *ExampleService) StopAsync() {

}
