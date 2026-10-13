// Package app assembles configuration, resources and an HTTP host into one lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	server "github.com/FlyTOmeLight/server-go"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/logger"
)

type Options struct {
	ConfigPath   string
	LogConfig    *logger.Config
	Setup        host.Setup
	InitTimeout  time.Duration // Zero uses 15 seconds.
	CloseTimeout time.Duration // Zero uses 5 seconds per cleanup operation.
}

// Runtime is available during assembly. Run owns Config, Log, initialized modules
// and added services. Assembly must not start tasks using its initialization context.
type Runtime struct {
	Config   *conf.Manager
	Log      *logger.Logger
	services []host.Service
	cleanup  []func(context.Context) error
}

// Initialize transfers module ownership to Run, including failed rollback cleanup.
// Modules are initialized in order and closed in reverse order.
func (r *Runtime) Initialize(ctx context.Context, modules ...server.Module) error {
	b := server.NewBootstrap(modules...)
	r.cleanup = append(r.cleanup, b.Close)
	return b.Initialize(ctx)
}

// AddService transfers a constructed service to Run. Its Stop is called once,
// including when assembly, route registration or host startup fails.
func (r *Runtime) AddService(service host.Service) error {
	if service == nil {
		return errors.New("service is nil")
	}
	owned := &ownedService{Service: service}
	r.services = append(r.services, owned)
	r.cleanup = append(r.cleanup, owned.Stop)
	return nil
}

type ownedService struct {
	host.Service
	stop sync.Once
}

func (s *ownedService) Stop(ctx context.Context) (err error) {
	s.stop.Do(func() { err = s.Service.Stop(ctx) })
	return err
}

// Run constructs configuration and logging, invokes build under an initialization
// deadline with a typed configuration decoded once from the startup snapshot,
// watches configuration and runs one HTTP host. Cancellation stops the
// host before services and modules are released. The caller owns process signals.
// Build must release partial resources that it has not transferred to Runtime.
func Run[T any](ctx context.Context, options Options, build func(context.Context, *Runtime, *T) error) (result error) {
	if build == nil {
		return errors.New("application build function is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if options.InitTimeout == 0 {
		options.InitTimeout = 15 * time.Second
	}
	if options.CloseTimeout == 0 {
		options.CloseTimeout = 5 * time.Second
	}
	if options.LogConfig == nil {
		options.LogConfig = &logger.Config{Level: "info", Outputs: []logger.Output{{Type: "console"}}}
	}
	log, err := logger.NewLogger(options.LogConfig)
	if err != nil {
		return err
	}
	defer log.Close()
	r := &Runtime{Log: log}
	runCtx, cancelRun := context.WithCancel(ctx)
	defer func() {
		cancelRun()
		for _, v := range slices.Backward(r.cleanup) {
			closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), options.CloseTimeout)
			result = errors.Join(result, v(closeCtx))
			cancel()
		}
	}()
	r.Config, err = conf.NewManager(options.ConfigPath)
	if err != nil {
		return err
	}
	r.cleanup = append(r.cleanup, r.Config.Close)
	var config T
	if err := r.Config.Snapshot().Unmarshal(&config); err != nil {
		return fmt.Errorf("decode application config: %w", err)
	}
	initCtx, cancelInit := context.WithTimeout(runCtx, options.InitTimeout)
	err = build(logger.WithContext(initCtx, log), r, &config)
	if err == nil {
		err = initCtx.Err()
	}
	cancelInit()
	if err != nil {
		return err
	}
	if err := runCtx.Err(); err != nil {
		return err
	}
	h, err := host.NewHTTPHost(r.Config, log, options.Setup, r.services...)
	if err != nil {
		return err
	}
	r.cleanup = append(r.cleanup, func(context.Context) error { return h.Close() })
	if err := r.Config.Watch(
		runCtx,
		func(err error) { log.Errorf(runCtx, "reload configuration: %v", err) },
	); err != nil {
		return err
	}
	return h.Run(runCtx)
}
