package main

import (
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/examples/example-service2/service"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/logger"
)

func main() {
	defer logger.Sync()
	config := conf.Default()
	h := host.NewHTTPHost(config)

	// add service
	svc := service.NewExampleService()
	h.AddService(svc)
	h.Run()
}
