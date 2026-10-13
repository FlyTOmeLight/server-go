package host

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/FlyTOmeLight/server-go/conf"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/FlyTOmeLight/server-go/middleware"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/FlyTOmeLight/server-go/version"
)

const (
	DefaultReadTimeout    = 20000
	DefaultWriteTimeout   = 10000
	DefaultIdleTimeout    = 10000
	DefaultMaxHeaderBytes = 100000
)

var ErrRestartRequired = errors.New("configuration change requires process restart")

// Setup installs optional HTTP integrations before business routes are registered.
type Setup func(*gin.Engine, *conf.Config) error

type HTTPHost struct {
	manager     *conf.Manager
	log         *logger.Logger
	setup       Setup
	services    []Service
	mu          sync.Mutex
	config      *conf.Config
	server      *http.Server
	address     string
	running     bool
	closed      bool
	ready       atomic.Bool
	router      atomic.Pointer[gin.Engine]
	unsubscribe func()
	closeOnce   sync.Once
	requestMu   sync.Mutex
	requests    sync.WaitGroup
	draining    bool
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

func (h *HTTPHost) buildRouter(config *conf.Config) (*gin.Engine, error) {
	engine := gin.New()
	_ = engine.SetTrustedProxies(nil)
	engine.HandleMethodNotAllowed = true
	engine.ContextWithFallback = true
	engine.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(logger.WithContext(c.Request.Context(), h.log))
		c.Set("server.AppCode", config.AppCode)
		c.Set("server.Debug", config.RunMode == conf.RunModeDebug)
		c.Next()
	}, middleware.RequestID(), middleware.RequestLog(), middleware.Recovery(), serrors.ErrorHandler(config))
	engine.NoRoute(middleware.APINotFound())
	engine.NoMethod(func(c *gin.Context) {
		serrors.Abort(c, http.StatusMethodNotAllowed, "MethodNotAllowed", "method not allowed")
	})
	engine.GET("/ping", middleware.Ping())
	engine.GET("/healthz", middleware.Healthz(h.ready.Load))
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

// Handler exposes the current immutable router to HTTP servers and integration tests.
func (h *HTTPHost) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.requestMu.Lock()
		if h.draining {
			h.requestMu.Unlock()
			http.Error(w, "service is stopping", http.StatusServiceUnavailable)
			return
		}
		h.requests.Add(1)
		h.requestMu.Unlock()
		defer h.requests.Done()
		h.router.Load().ServeHTTP(w, r)
	})
}

func (h *HTTPHost) Address() string { h.mu.Lock(); defer h.mu.Unlock(); return h.address }

// prepareReload never restarts a process lifecycle or its background workers.
func (h *HTTPHost) prepareReload(previous, next *conf.Config, keys []string) (func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, errors.New("host is closed")
	}
	for _, key := range keys {
		switch key {
		case "httpserver", "feature", "fakeauth", "runmode":
		default:
			return nil, fmt.Errorf("%w: %s", ErrRestartRequired, key)
		}
	}
	old, new := previous.HTTPServer, next.HTTPServer
	if old.Listen != new.Listen || old.ReadTimeout != new.ReadTimeout || old.WriteTimeout != new.WriteTimeout ||
		old.IdleTimeout != new.IdleTimeout ||
		old.MaxHeaderBytes != new.MaxHeaderBytes ||
		old.UseH2C != new.UseH2C {
		return nil, fmt.Errorf("%w: listener, protocol, or transport timeout", ErrRestartRequired)
	}
	engine, err := h.buildRouter(next)
	if err != nil {
		return nil, err
	}
	return func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if !h.closed {
			h.config = next.Clone()
			h.router.Store(engine)
		}
	}, nil
}

// NewHTTPServer configures a server without binding or reserving a socket.
func NewHTTPServer(config *conf.Config) *http.Server {
	cfg := config.HTTPServer
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:0"
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = DefaultReadTimeout
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
	return &http.Server{
		Addr:              cfg.Listen,
		ReadTimeout:       time.Duration(cfg.ReadTimeout) * time.Millisecond,
		ReadHeaderTimeout: time.Duration(cfg.ReadTimeout) * time.Millisecond,
		WriteTimeout:      time.Duration(cfg.WriteTimeout) * time.Millisecond,
		IdleTimeout:       time.Duration(cfg.IdleTimeout) * time.Millisecond,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}
}

// Run owns one serve loop. The process entry point owns signal handling.
// Run waits for admitted handlers before releasing service resources. A handler
// that ignores request cancellation delays Run's return until it finishes.
func (h *HTTPHost) Run(ctx context.Context) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	if h.running || h.closed {
		h.mu.Unlock()
		return errors.New("host can only run once")
	}
	h.running = true
	server := NewHTTPServer(h.config)
	requestCtx, cancelRequests := context.WithCancel(context.WithoutCancel(ctx))
	server.BaseContext = func(net.Listener) context.Context { return requestCtx }
	server.Handler = h.Handler()
	if h.config.HTTPServer.UseH2C {
		http2Server := &http2.Server{}
		if err := http2.ConfigureServer(server, http2Server); err != nil {
			h.running = false
			h.mu.Unlock()
			cancelRequests()
			return err
		}
		server.Handler = h2c.NewHandler(server.Handler, http2Server)
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		h.running = false
		h.mu.Unlock()
		cancelRequests()
		return err
	}
	owned := newHTTPListener(listener)
	h.server = server
	h.address = listener.Addr().String()
	h.mu.Unlock()
	workerCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	started := make([]Service, 0, len(h.services))
	defer func() {
		h.ready.Store(false)
		h.requestMu.Lock()
		h.draining = true
		h.requestMu.Unlock()
		drainCtx, drainCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		// Drain HTTP before releasing resources still used by in-flight requests.
		drainErr := server.Shutdown(drainCtx)
		_ = owned.Close()
		drained := make(chan struct{})
		go func() {
			h.requests.Wait()
			owned.wait()
			close(drained)
		}()
		if drainErr == nil {
			select {
			case <-drained:
			case <-drainCtx.Done():
				drainErr = drainCtx.Err()
			}
		}
		if drainErr != nil {
			cancelRequests()
			result = errors.Join(result, drainErr, server.Close())
			owned.closeConnections()
		}
		// Closing sockets does not join handler goroutines. Wait even after the
		// drain deadline so resources cannot be closed underneath user code.
		<-drained
		drainCancel()
		cancelRequests()
		cancel()
		for i := len(started) - 1; i >= 0; i-- {
			stopCtx, stopCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			result = errors.Join(result, started[i].Stop(stopCtx))
			stopCancel()
		}
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		h.closeOnce.Do(h.unsubscribe)
	}()
	for _, service := range h.services {
		if err := service.Start(workerCtx); err != nil {
			return fmt.Errorf("start service %s: %w", reflect.TypeOf(service), err)
		}
		started = append(started, service)
	}
	h.ready.Store(true)
	served := make(chan error, 1)
	go func() { served <- server.Serve(owned) }()
	h.log.Infof(ctx, "Server started at: %s", h.Address())
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
