package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/httpintegration"
	"github.com/FlyTOmeLight/server-go/logger"
)

type helloService struct{}

func (s *helloService) RegisterRoutes(router *gin.RouterGroup) error {
	router.GET("/hello", func(c *gin.Context) { c.JSON(200, gin.H{"message": "hello"}) })
	return nil
}

func (s *helloService) Start(context.Context) error { return nil }

func (s *helloService) Stop(context.Context) error { return nil }

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
	h, err := host.NewHTTPHost(manager, log, httpintegration.Install, &helloService{})
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
