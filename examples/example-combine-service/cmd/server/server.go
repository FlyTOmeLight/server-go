package main

import (
	"context"
	"github.com/FlyTOmeLight/server-go"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/db/mongodb"
	service1 "github.com/FlyTOmeLight/server-go/examples/example-service1/service"
	service2 "github.com/FlyTOmeLight/server-go/examples/example-service2/service"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/logger"
)

func main() {
	// sync logs before stops
	defer logger.Sync()

	// init bootstrap
	bootstrap := server.NewBootstrap(&db.Module{}, &mongodb.Module{})
	err := bootstrap.Initialize()
	if err != nil {
		logger.Fatalf(context.Background(), "init bootstrap error: %v", err)
	}
	defer bootstrap.Close()

	// host & services
	h := host.NewHTTPHost(conf.Default())
	svc1 := service1.NewExampleService()
	svc2 := service2.NewExampleService()
	h.AddService(svc1, svc2)
	h.Run()
}
