package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/examples/example-service2/service"
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
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result = errors.Join(result, manager.Close(closeCtx))
	}()
	h, err := host.NewHTTPHost(manager, log, httpintegration.Install, service.NewExampleService())
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
