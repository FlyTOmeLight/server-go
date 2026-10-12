package main

import (
	"context"
	"errors"
	"fmt"
	server "github.com/FlyTOmeLight/server-go"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/db/mongodb"
	service1 "github.com/FlyTOmeLight/server-go/examples/example-service1/service"
	service2 "github.com/FlyTOmeLight/server-go/examples/example-service2/service"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/httpintegration"
	"github.com/FlyTOmeLight/server-go/logger"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func run() (result error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	manager, err := conf.NewManager("")
	if err != nil {
		return err
	}
	log, err := logger.NewLogger(&logger.Config{Outputs: []logger.Output{{Type: "Console", Level: "Info"}}})
	if err != nil {
		return err
	}
	defer log.Close()
	var sqlConfig db.Config
	var mongoConfig mongodb.Config
	if err := manager.Snapshot().UnmarshalKey("DB", &sqlConfig); err != nil {
		return err
	}
	if err := manager.Snapshot().UnmarshalKey("MongoDB", &mongoConfig); err != nil {
		return err
	}
	database := &db.Module{Config: &sqlConfig}
	mongo := &mongodb.Module{Config: &mongoConfig}
	bootstrap := server.NewBootstrap(database, mongo)
	if err := bootstrap.Initialize(ctx); err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result = errors.Join(result, manager.Close(closeCtx), bootstrap.Close(closeCtx))
	}()
	svc1, err := service1.NewExampleService(ctx, database.DB)
	if err != nil {
		return err
	}
	h, err := host.NewHTTPHost(manager, log, httpintegration.Install, svc1, service2.NewExampleService())
	if err != nil {
		return err
	}
	defer h.Close()
	if err := manager.Watch(ctx, func(err error) { log.Errorf(ctx, "configuration reload: %v", err) }); err != nil {
		return err
	}
	return h.Run(ctx)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
