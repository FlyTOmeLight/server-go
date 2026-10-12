package host

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/gin-gonic/gin"
)

type testService struct {
	starts, stops atomic.Int32
	registered    atomic.Int32
	handle        gin.HandlerFunc
	stop          func(context.Context) error
}

func (s *testService) RegisterRoutes(router *gin.RouterGroup) error {
	s.registered.Add(1)
	router.GET("/business", s.handle)
	return nil
}
func (s *testService) Start(context.Context) error { s.starts.Add(1); return nil }
func (s *testService) Stop(ctx context.Context) error {
	s.stops.Add(1)
	if s.stop != nil {
		return s.stop(ctx)
	}
	return nil
}

func newTestHost(t *testing.T, service Service) (*HTTPHost, *conf.Manager) {
	t.Helper()
	return newTestHostWithHTTPConfig(t, service, conf.HTTPServerConfig{})
}

func newTestHostWithHTTPConfig(t *testing.T, service Service, httpConfig conf.HTTPServerConfig) (*HTTPHost, *conf.Manager) {
	t.Helper()
	manager, err := conf.NewManagerFromConfig(&conf.Config{AppName: "test", AppCode: 100, RunMode: conf.RunModeRelease, HTTPServer: httpConfig})
	if err != nil {
		t.Fatal(err)
	}
	log, err := logger.NewLogger(&logger.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(log.Close)
	h, err := NewHTTPHost(manager, log, nil, service)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := h.Close(); err != nil {
			t.Error(err)
		}
	})
	return h, manager
}
func startTestHost(t *testing.T, h *HTTPHost) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.Run(ctx) }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if h.ready.Load() && h.Address() != "" {
			return cancel, done
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("host did not start")
		}
		time.Sleep(time.Millisecond)
	}
}
func waitStopped(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("host did not stop")
	}
}

func TestReloadDoesNotRestartLifecycle(t *testing.T) {
	svc := &testService{handle: func(c *gin.Context) { c.String(200, "business") }}
	h, manager := newTestHost(t, svc)
	cancel, done := startTestHost(t, h)
	defer func() { cancel(); waitStopped(t, done) }()
	next := manager.Snapshot()
	next.Feature = map[string]any{"enabled": true}
	if err := manager.Update(next); err != nil {
		t.Fatal(err)
	}
	if svc.starts.Load() != 1 || svc.registered.Load() != 2 {
		t.Fatalf("starts=%d registered=%d", svc.starts.Load(), svc.registered.Load())
	}
	response, err := http.Get("http://" + h.Address() + "/business")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.StatusCode)
	}
	next = manager.Snapshot()
	next.HTTPServer.ReadTimeout = 1234
	if err := manager.Update(next); !errors.Is(err, ErrRestartRequired) {
		t.Fatalf("static change: %v", err)
	}
	if manager.Snapshot().HTTPServer.ReadTimeout != 0 {
		t.Fatal("rejected config published")
	}
	if svc.starts.Load() != 1 {
		t.Fatal("worker restarted")
	}
}

func TestDrainBeforeStoppingServices(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var prematurelyStopped atomic.Bool
	svc := &testService{handle: func(c *gin.Context) { close(entered); <-release; c.String(200, "complete") }}
	svc.stop = func(context.Context) error {
		select {
		case <-release:
		default:
			prematurelyStopped.Store(true)
		}
		return nil
	}
	h, _ := newTestHost(t, svc)
	cancel, done := startTestHost(t, h)
	responseDone := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + h.Address() + "/business")
		if err == nil {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if string(body) != "complete" {
				err = errors.New("response not drained")
			}
		}
		responseDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request did not enter")
	}
	cancel()
	select {
	case <-done:
		t.Fatal("run returned before draining request")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-responseDone; err != nil {
		t.Fatal(err)
	}
	waitStopped(t, done)
	if prematurelyStopped.Load() || svc.stops.Load() != 1 {
		t.Fatal("service resources stopped before drain or stopped more than once")
	}
}

func TestHostInstancesAndConstructorSocket(t *testing.T) {
	config := &conf.Config{}
	server := NewHTTPServer(config)
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()
	svc := &testService{handle: func(c *gin.Context) { c.String(200, "ok") }}
	h, manager := newTestHost(t, svc)
	other, _ := newTestHost(t, &testService{handle: func(c *gin.Context) { c.String(200, "other") }})
	next := manager.Snapshot()
	next.HTTPServer.APIBasePath = "/new"
	if err := manager.Update(next); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	h.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/new/business", nil))
	if recorder.Code != 200 {
		t.Fatal(recorder.Code)
	}
	recorder = httptest.NewRecorder()
	other.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/business", nil))
	if recorder.Body.String() != "other" {
		t.Fatal("hosts share routing state")
	}
}

func TestConstructionDoesNotMissReplacement(t *testing.T) {
	manager, err := conf.NewManagerFromConfig(&conf.Config{AppName: "test", AppCode: 100, RunMode: conf.RunModeRelease})
	if err != nil {
		t.Fatal(err)
	}
	log, err := logger.NewLogger(&logger.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(log.Close)
	entered, release := make(chan struct{}), make(chan struct{})
	var builds atomic.Int32
	setup := func(*gin.Engine, *conf.Config) error {
		if builds.Add(1) == 1 {
			close(entered)
			<-release
		}
		return nil
	}
	type result struct {
		host *HTTPHost
		err  error
	}
	constructed := make(chan result, 1)
	go func() {
		h, err := NewHTTPHost(manager, log, setup, &testService{handle: func(c *gin.Context) { c.String(200, "ok") }})
		constructed <- result{h, err}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("construction did not enter setup")
	}
	next := manager.Snapshot()
	next.HTTPServer.APIBasePath = "/new"
	updated := make(chan error, 1)
	go func() { updated <- manager.Update(next) }()
	select {
	case err := <-updated:
		close(release)
		t.Fatalf("replacement bypassed unfinished construction: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	var h *HTTPHost
	select {
	case result := <-constructed:
		if result.err != nil {
			t.Fatal(result.err)
		}
		h = result.host
	case <-time.After(time.Second):
		t.Fatal("construction did not complete")
	}
	defer h.Close()
	select {
	case err := <-updated:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("replacement did not complete")
	}
	recorder := httptest.NewRecorder()
	h.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/new/business", nil))
	if recorder.Code != 200 {
		t.Fatal("router missed replacement", recorder.Code)
	}
}
