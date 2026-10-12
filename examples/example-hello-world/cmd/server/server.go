package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	server "github.com/FlyTOmeLight/server-go"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/httpintegration"
	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/FlyTOmeLight/server-go/swagger"

	_ "github.com/FlyTOmeLight/server-go/examples/example-hello-world/doc/openapi"
	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (result error) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	log, err := logger.NewLogger(&logger.Config{Level: "info", Outputs: []logger.Output{{Type: "console"}}})
	if err != nil {
		return err
	}
	defer log.Close()
	manager, err := conf.NewManager("")
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result = errors.Join(result, manager.Close(closeCtx))
	}()
	var databaseConfig db.Config
	if err := manager.Snapshot().UnmarshalKey("DB", &databaseConfig); err != nil {
		return fmt.Errorf("load database config: %w", err)
	}
	database := &db.Module{Config: &databaseConfig}
	bootstrap := server.NewBootstrap(database)
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result = errors.Join(result, bootstrap.Close(closeCtx))
	}()
	initCtx, initCancel := context.WithTimeout(ctx, 15*time.Second)
	defer initCancel()
	if err := bootstrap.Initialize(initCtx); err != nil {
		return err
	}
	svc, err := service.NewHelloWorldService(initCtx, database.DB, databaseConfig.AutoMigrate)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, svc.Stop(context.Background())) }()
	swagger.DocPrefix = "HelloWorld"
	h, err := host.NewHTTPHost(manager, log, httpintegration.Install, svc)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, h.Close()) }()
	if err := manager.Watch(ctx, func(err error) { log.Errorf(ctx, "reload configuration: %v", err) }); err != nil {
		return err
	}
	return h.Run(ctx)
}
