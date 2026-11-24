package host

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"maps"
	"net"
	"net/http"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/conf"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/internal/lifecycle"
	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/FlyTOmeLight/server-go/middleware"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/FlyTOmeLight/server-go/version"
)

const (
	DefaultReadTimeout       = 20000
	DefaultReadHeaderTimeout = 5000
	DefaultWriteTimeout      = 10000
	DefaultIdleTimeout       = 10000
	DefaultMaxHeaderBytes    = 100000
)

var ErrRestartRequired = errors.New("configuration change requires process restart")

// Setup installs optional HTTP integrations before business routes are registered.
type Setup func(*core.Engine, *conf.Config) error

type HTTPHost struct {
	manager      *conf.Manager
	log          *logger.Logger
	setup        Setup
	services     []Service
	mu           sync.Mutex
	config       *conf.Config
	server       *http.Server
	address      string
	adminAddress string
	running      bool
	closed       bool
	ready        atomic.Bool
	router       atomic.Pointer[core.Engine]
	adminRouter  atomic.Pointer[core.Engine]
	unsubscribe  func()
	closeOnce    sync.Once
	healthMu     sync.RWMutex
	healthChecks map[string]middleware.HealthCheck
	requestMu    sync.RWMutex
	requests     sync.WaitGroup
	draining     bool
}

// NewHTTPHost borrows an application manager and logger. The caller owns their cleanup.
func NewHTTPHost(manager *conf.Manager, log *logger.Logger, setup Setup, services ...Service) (*HTTPHost, error) {
	if manager == nil || log == nil {
		return nil, errors.New("host requires configuration and logger instances")
	}
	h := &HTTPHost{manager: manager, log: log, setup: setup, services: append([]Service(nil), services...)}
	unsubscribe, err := manager.Bind(h.prepareReload)
	if err != nil {
		return nil, err
	}
	h.unsubscribe = unsubscribe
	return h, nil
}

func (h *HTTPHost) buildRouter(config *conf.Config) (*core.Engine, error) {
	engine, err := h.newRouter(config, config.HTTPServer.TrustedProxies)
	if err != nil {
		return nil, err
	}
	engine.GET("/version", version.Version())
	binder.RegisterValidator()
	if h.setup != nil {
		if err := h.setup(engine, config.Clone()); err != nil {
			return nil, err
		}
	}
	group := engine.Group(config.HTTPServer.APIBasePath)
	for _, service := range h.services {
		if service == nil {
			return nil, errors.New("service is nil")
		}
		if err := service.RegisterRoutes(group); err != nil {
			return nil, fmt.Errorf("register routes: %w", err)
		}
	}
	return engine, nil
}

func (h *HTTPHost) newRouter(config *conf.Config, trustedProxies []string) (*core.Engine, error) {
	engine := gin.New()
	if len(trustedProxies) == 0 {
		trustedProxies = nil
	}
	if err := engine.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("configure HTTP trusted proxies: %w", err)
	}
	engine.HandleMethodNotAllowed = true
	engine.ContextWithFallback = true
	engine.Use(func(c *core.Context) {
		c.Request = c.Request.WithContext(logger.WithContext(c.Request.Context(), h.log))
		c.Set("server.AppCode", config.AppCode)
		c.Set("server.Debug", config.RunMode == conf.RunModeDebug)
		c.Next()
	}, middleware.RequestID(), middleware.RequestLog(), middleware.Recovery(), serrors.ErrorHandler(config))
	engine.NoRoute(middleware.APINotFound())
	engine.NoMethod(func(c *core.Context) {
		serrors.Abort(c, http.StatusMethodNotAllowed, "MethodNotAllowed", "method not allowed")
	})
	engine.GET("/ping", middleware.Ping())
	engine.GET("/healthz", middleware.Healthz(h.ready.Load))
	engine.GET("/livez", middleware.Liveness())
	engine.GET("/readyz", middleware.ReadinessWithChecks(h.ready.Load, h.healthCheckSnapshot))
	return engine, nil
}

// RegisterHealthCheck adds or replaces a readiness check by name.
func (h *HTTPHost) RegisterHealthCheck(name string, check middleware.HealthCheck) {
	if strings.TrimSpace(name) == "" || check == nil {
		return
	}
	h.healthMu.Lock()
	defer h.healthMu.Unlock()
	if h.healthChecks == nil {
		h.healthChecks = make(map[string]middleware.HealthCheck)
	}
	h.healthChecks[name] = func(ctx context.Context) error {
		if !h.admitRequest() {
			return http.ErrServerClosed
		}
		defer h.requests.Done()
		return check(ctx)
	}
}

func (h *HTTPHost) healthCheckSnapshot() map[string]middleware.HealthCheck {
	h.healthMu.RLock()
	defer h.healthMu.RUnlock()
	return maps.Clone(h.healthChecks)
}

// Handler exposes the current immutable router to HTTP servers and integration tests.
func (h *HTTPHost) Handler() http.Handler {
	return h.serveRouter(&h.router)
}

func (h *HTTPHost) serveRouter(router *atomic.Pointer[core.Engine]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.admitRequest() {
			http.Error(w, "service is stopping", http.StatusServiceUnavailable)
			return
		}
		defer h.requests.Done()
		router.Load().ServeHTTP(w, r)
	})
}

func (h *HTTPHost) admitRequest() bool {
	h.requestMu.RLock()
	defer h.requestMu.RUnlock()
	if h.draining {
		return false
	}
	h.requests.Add(1)
	return true
}

func (h *HTTPHost) Address() string { h.mu.Lock(); defer h.mu.Unlock(); return h.address }

// prepareReload never restarts a process lifecycle or its background workers.
func (h *HTTPHost) prepareReload(previous, next *conf.Config, keys []string) (func(), error) {
	h.mu.Lock()
	closed := h.closed
	h.mu.Unlock()
	if closed {
		return nil, errors.New("host is closed")
	}
	// Bind uses an empty key list for the initial router build.
	rebuild := len(keys) == 0
	for _, key := range keys {
		switch key {
		case "httpserver", "feature", "fakeauth", "runmode":
			rebuild = true
		case "appname", "appcode", "deploymode", "services":
			return nil, fmt.Errorf("%w: %s", ErrRestartRequired, key)
		}
	}
	if !rebuild {
		return nil, nil
	}
	old, candidate := previous.HTTPServer, next.HTTPServer
	if old.Listen != candidate.Listen || old.AdminListen != candidate.AdminListen ||
		old.ReadTimeout != candidate.ReadTimeout || old.ReadHeaderTimeout != candidate.ReadHeaderTimeout ||
		old.WriteTimeout != candidate.WriteTimeout || old.IdleTimeout != candidate.IdleTimeout ||
		old.PreStopDelay != candidate.PreStopDelay || old.MaxHeaderBytes != candidate.MaxHeaderBytes ||
		old.UseH2C != candidate.UseH2C || old.TLS != candidate.TLS ||
		!slices.Equal(old.TrustedProxies, candidate.TrustedProxies) {
		return nil, fmt.Errorf("%w: HTTP transport settings", ErrRestartRequired)
	}
	engine, err := h.buildRouter(next)
	if err != nil {
		return nil, err
	}
	var admin *core.Engine
	if candidate.AdminListen != "" {
		admin, err = h.buildAdminRouter(next)
		if err != nil {
			return nil, err
		}
	}
	return func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if !h.closed {
			h.config = next.Clone()
			h.router.Store(engine)
			h.adminRouter.Store(admin)
		}
	}, nil
}

// NewHTTPServer configures a server without binding or reserving a socket.
func NewHTTPServer(config *conf.Config) *http.Server {
	return newHTTPServer(config.HTTPServer)
}

func newHTTPServer(cfg conf.HTTPServerConfig) *http.Server {
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:0"
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = DefaultReadTimeout
	}
	if cfg.ReadHeaderTimeout == 0 {
		cfg.ReadHeaderTimeout = DefaultReadHeaderTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = DefaultWriteTimeout
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}
	if cfg.MaxHeaderBytes == 0 {
		cfg.MaxHeaderBytes = DefaultMaxHeaderBytes
	}
	server := &http.Server{
		Addr:              cfg.Listen,
		ReadTimeout:       time.Duration(cfg.ReadTimeout) * time.Millisecond,
		ReadHeaderTimeout: time.Duration(cfg.ReadHeaderTimeout) * time.Millisecond,
		WriteTimeout:      time.Duration(cfg.WriteTimeout) * time.Millisecond,
		IdleTimeout:       time.Duration(cfg.IdleTimeout) * time.Millisecond,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}
	if cfg.TLS.Enable {
		server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else if cfg.UseH2C {
		server.Protocols = new(http.Protocols)
		server.Protocols.SetHTTP1(true)
		server.Protocols.SetUnencryptedHTTP2(true)
	}
	return server
}

func loadTLS(server *http.Server, config conf.HTTPServerTLSConfig) error {
	certificate, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return fmt.Errorf("load HTTP TLS certificate: %w", err)
	}
	server.TLSConfig.Certificates = []tls.Certificate{certificate}
	if config.ClientCAFile == "" {
		return nil
	}
	pem, err := os.ReadFile(config.ClientCAFile)
	if err != nil {
		return fmt.Errorf("read HTTP TLS client CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return errors.New("HTTP TLS client CA file contains no valid certificates")
	}
	server.TLSConfig.ClientCAs = pool
	server.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
	return nil
}

type httpErrorLog struct {
	ctx context.Context
	log *logger.Logger
}

func (l httpErrorLog) Write(message []byte) (int, error) {
	l.log.Errorf(l.ctx, "%s", strings.TrimSuffix(string(message), "\n"))
	return len(message), nil
}

type httpEndpoint struct {
	server   *http.Server
	listener *httpListener
}

func listenHTTP(server *http.Server) (httpEndpoint, error) {
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return httpEndpoint{}, err
	}
	return httpEndpoint{server: server, listener: newHTTPListener(listener)}, nil
}

func (e httpEndpoint) serve() error {
	if e.server.TLSConfig != nil {
		return e.server.ServeTLS(e.listener, "", "")
	}
	return e.server.Serve(e.listener)
}

// Run owns one serve loop. The process entry point owns signal handling.
// Run waits for admitted handlers before releasing service resources. A handler
// that ignores request cancellation delays Run's return until it finishes.
func (h *HTTPHost) Run(ctx context.Context) (result error) {
	return h.RunWithStartupTimeout(ctx, 15*time.Second)
}

// RunWithStartupTimeout limits all service starts to one shared budget. Startup
// responds to ctx cancellation; once ready, workers remain alive until HTTP drains.
func (h *HTTPHost) RunWithStartupTimeout(ctx context.Context, startupTimeout time.Duration) error {
	return h.RunWithShutdownHook(ctx, startupTimeout, nil)
}

// RunWithShutdownHook runs the host and calls beforeShutdown before draining HTTP
// and stopping services, including after a failed service start. The caller
// remains responsible for cleanup if the host exits before binding a listener.
func (h *HTTPHost) RunWithShutdownHook(
	ctx context.Context,
	startupTimeout time.Duration,
	beforeShutdown func() error,
) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if startupTimeout <= 0 {
		return errors.New("service startup timeout must be positive")
	}
	startupCtx, cancelStartup := context.WithTimeout(ctx, startupTimeout)
	defer cancelStartup()
	h.mu.Lock()
	if h.running || h.closed {
		h.mu.Unlock()
		return errors.New("host can only run once")
	}
	h.running = true
	cfg := h.config.HTTPServer
	server := newHTTPServer(cfg)
	if cfg.TLS.Enable {
		if err := loadTLS(server, cfg.TLS); err != nil {
			h.running = false
			h.mu.Unlock()
			return err
		}
	}
	requestCtx, cancelRequests := context.WithCancel(context.WithoutCancel(ctx))
	baseContext := func(net.Listener) context.Context { return requestCtx }
	errorLog := log.New(httpErrorLog{ctx: requestCtx, log: h.log}, "", 0)
	server.BaseContext = baseContext
	server.ErrorLog = errorLog
	server.Handler = h.Handler()
	main, err := listenHTTP(server)
	if err != nil {
		h.running = false
		h.mu.Unlock()
		cancelRequests()
		return err
	}
	endpoints := []httpEndpoint{main}
	if cfg.AdminListen != "" {
		adminServer := h.newAdminServer(cfg)
		adminServer.BaseContext = baseContext
		adminServer.ErrorLog = errorLog
		admin, err := listenHTTP(adminServer)
		if err != nil {
			_ = main.listener.Close()
			h.running = false
			h.mu.Unlock()
			cancelRequests()
			return fmt.Errorf("listen on admin address: %w", err)
		}
		h.adminAddress = admin.listener.Addr().String()
		endpoints = append(endpoints, admin)
	}
	h.server = server
	h.address = main.listener.Addr().String()
	h.mu.Unlock()
	workerCtx, cancel := context.WithCancel(logger.WithContext(context.WithoutCancel(ctx), h.log))
	stopStartupCancellation := context.AfterFunc(startupCtx, cancel)
	defer stopStartupCancellation()
	started := make([]Service, 0, len(h.services))
	var serving sync.WaitGroup
	defer func() {
		var preStop <-chan time.Time
		if h.ready.Swap(false) && cfg.PreStopDelay > 0 {
			timer := time.NewTimer(time.Duration(cfg.PreStopDelay) * time.Millisecond)
			defer timer.Stop()
			preStop = timer.C
		}
		if beforeShutdown != nil {
			result = errors.Join(result, beforeShutdown())
		}
		if preStop != nil {
			<-preStop
		}
		result = errors.Join(result, h.shutdownHTTP(ctx, endpoints, cancelRequests))
		serving.Wait()
		cancelRequests()
		cancel()
		for _, s := range slices.Backward(started) {
			stopErr, stopped := lifecycle.Stop(ctx, 5*time.Second, s.Stop)
			result = errors.Join(result, stopErr)
			if !stopped {
				break
			}
		}
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		h.closeOnce.Do(h.unsubscribe)
	}()
	for _, service := range h.services {
		if err := startupCtx.Err(); err != nil {
			return err
		}
		started = append(started, service)
		if err := service.Start(workerCtx); err != nil {
			return errors.Join(
				fmt.Errorf("start service %s: %w", reflect.TypeOf(service), err),
				startupCtx.Err(),
			)
		}
	}
	stopStartupCancellation()
	if err := startupCtx.Err(); err != nil {
		return err
	}
	h.ready.Store(true)
	served := make(chan error, len(endpoints))
	for _, endpoint := range endpoints {
		serving.Go(func() { served <- endpoint.serve() })
	}
	h.log.Infof(ctx, "Server started at: %s", h.Address())
	if address := h.AdminAddress(); address != "" {
		h.log.Infof(ctx, "Admin server started at: %s", address)
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-served:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (h *HTTPHost) shutdownHTTP(
	ctx context.Context,
	endpoints []httpEndpoint,
	cancelRequests context.CancelFunc,
) error {
	h.requestMu.Lock()
	h.draining = true
	h.requestMu.Unlock()
	drainCtx, drainCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer drainCancel()
	shutdown := make(chan error, len(endpoints))
	for _, endpoint := range endpoints {
		go func() {
			err := endpoint.server.Shutdown(drainCtx)
			_ = endpoint.listener.Close()
			shutdown <- err
		}()
	}
	var drainErr error
	for range endpoints {
		drainErr = errors.Join(drainErr, <-shutdown)
	}
	drained := make(chan struct{})
	go func() {
		h.requests.Wait()
		for _, endpoint := range endpoints {
			endpoint.listener.wait()
		}
		close(drained)
	}()
	if drainErr == nil {
		select {
		case <-drained:
			return nil
		case <-drainCtx.Done():
			drainErr = drainCtx.Err()
		}
	}
	cancelRequests()
	for _, endpoint := range endpoints {
		drainErr = errors.Join(drainErr, endpoint.server.Close())
		endpoint.listener.closeConnections()
	}
	<-drained
	return drainErr
}

// Close releases a host that was constructed but never run, or has already stopped.
// A running host is stopped through the context supplied to Run.
func (h *HTTPHost) Close() error {
	h.mu.Lock()
	if h.running && !h.closed {
		h.mu.Unlock()
		return errors.New("cancel the Run context before closing the host")
	}
	h.closed = true
	h.mu.Unlock()
	h.requestMu.Lock()
	h.draining = true
	h.requestMu.Unlock()
	h.requests.Wait()
	h.closeOnce.Do(h.unsubscribe)
	return nil
}
