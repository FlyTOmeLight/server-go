// Package httpintegration explicitly installs optional server-go HTTP features.
package httpintegration

import (
	"errors"
	"fmt"

	"github.com/gin-contrib/pprof"

	"github.com/FlyTOmeLight/server-go/auth"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/console"
	"github.com/FlyTOmeLight/server-go/metrics"
	"github.com/FlyTOmeLight/server-go/middleware"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/swagger"
)

// Install configures one engine before its business routes are registered.
// The host must install its error handling and recovery before calling Install.
// Dapr subscriptions and route-level idempotence are assembled separately.
func Install(engine *core.Engine, config *conf.Config) error {
	return InstallWithAuth(engine, config, nil)
}

// InstallWithAuth installs an explicit authentication handler before protected
// assets and business routes. AuthCenter requires a handler such as
// auth.TrustedIdentityHeaders with a gateway verifier; no headers are trusted by default.
func InstallWithAuth(engine *core.Engine, config *conf.Config, authenticate core.HandlerFunc) error {
	if engine == nil || config == nil {
		return errors.New("HTTP integrations require an engine and config")
	}
	switch config.HTTPServer.AuthType {
	case "", conf.AuthTypeFake, conf.AuthTypeAuthCenter:
	default:
		return fmt.Errorf("unsupported HTTP authentication mode: %q", config.HTTPServer.AuthType)
	}
	if config.HTTPServer.AuthType == conf.AuthTypeAuthCenter && authenticate == nil {
		return errors.New("AuthCenter requires an explicit verified authentication handler")
	}

	switch config.RunMode {
	case conf.RunModeDebug, conf.RunModeTest:
		engine.Use(
			middleware.Debug(config.HTTPServer.Trace.PrefixFilters, config.HTTPServer.Trace.SuffixFilters),
		)
	}
	if config.HTTPServer.PprofEnable {
		pprof.Register(engine, "/pprof")
	}
	if config.HTTPServer.MetricsEnable {
		engine.GET("/metrics", metrics.Metrics())
	}
	if config.HTTPServer.SwaggerEnable {
		engine.GET("/swagger/*any", swagger.Swagger())
	}
	engine.GET("/feature", middleware.Feature(config.Feature))
	middleware.Asset(config.HTTPServer.Assets, false, engine)
	if config.HTTPServer.ConsoleEnable {
		console.RegisterConsoleAPI(engine, config)
	}
	if authenticate != nil {
		engine.Use(authenticate)
	} else if err := auth.Use(engine, config); err != nil {
		return err
	}
	middleware.Asset(config.HTTPServer.Assets, true, engine)
	return nil
}
