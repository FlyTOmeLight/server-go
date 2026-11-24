package host

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/httpintegration"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type testService struct {
	starts, stops atomic.Int32
	registered    atomic.Int32
	handle        core.HandlerFunc
	start         func(context.Context) error
	stop          func(context.Context) error
}

func (s *testService) RegisterRoutes(router *core.RouterGroup) error {
	s.registered.Add(1)
	router.GET("/business", s.handle)
	return nil
}

func (s *testService) Start(ctx context.Context) error {
	s.starts.Add(1)
	if s.start != nil {
		return s.start(ctx)
	}
	return nil
}

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

func newTestHostWithHTTPConfig(
	t *testing.T,
	service Service,
	httpConfig conf.HTTPServerConfig,
) (*HTTPHost, *conf.Manager) {
	t.Helper()
	return newTestHostWithSetup(t, service, httpConfig, nil)
}

func newTestHostWithSetup(
	t *testing.T,
	service Service,
	httpConfig conf.HTTPServerConfig,
	setup Setup,
) (*HTTPHost, *conf.Manager) {
	t.Helper()
	manager, err := conf.NewManagerFromConfig(
		&conf.Config{AppName: "test", AppCode: 100, RunMode: conf.RunModeRelease, HTTPServer: httpConfig},
	)
	if err != nil {
		t.Fatal(err)
	}
	log, err := logger.NewLogger(&logger.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(log.Close)
	h, err := NewHTTPHost(manager, log, setup, service)
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
	svc := &testService{handle: func(c *core.Context) { c.String(200, "business") }}
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

func TestReloadBusinessSectionKeepsRouter(t *testing.T) {
	svc := &testService{handle: func(c *core.Context) { c.String(http.StatusOK, "business") }}
	_, manager := newTestHost(t, svc)
	var committed bool
	manager.Subscribe(func(_, next *conf.Config, _ []string) (func(), error) {
		var business struct{ Enabled bool }
		if err := next.UnmarshalKey("Business", &business); err != nil {
			return nil, err
		}
		return func() { committed = business.Enabled }, nil
	})
	next, err := conf.FromSettings(map[string]any{
		"appname": "test", "appcode": 100, "runmode": conf.RunModeRelease,
		"business": map[string]any{"enabled": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Update(next); err != nil {
		t.Fatal(err)
	}
	if !committed || svc.registered.Load() != 1 {
		t.Fatalf("business reload: committed=%v route registrations=%d", committed, svc.registered.Load())
	}
}

func TestReloadSetupCanReadHostAddress(t *testing.T) {
	manager, err := conf.NewManagerFromConfig(
		&conf.Config{AppName: "test", AppCode: 100, RunMode: conf.RunModeRelease},
	)
	if err != nil {
		t.Fatal(err)
	}
	log, err := logger.NewLogger(&logger.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(log.Close)
	var h *HTTPHost
	h, err = NewHTTPHost(manager, log, func(*core.Engine, *conf.Config) error {
		if h != nil {
			_ = h.Address()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	next := manager.Snapshot()
	next.Feature = map[string]any{"enabled": true}
	done := make(chan error, 1)
	go func() { done <- manager.Update(next) }()
	select {
	case err := <-done:
		defer h.Close()
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reload deadlocked when setup read the host address")
	}
}

func TestServiceStartReceivesHostLogger(t *testing.T) {
	observed := make(chan *logger.Logger, 1)
	svc := &testService{
		handle: func(c *core.Context) { c.Status(http.StatusOK) },
		start: func(ctx context.Context) error {
			observed <- logger.FromContext(ctx)
			return nil
		},
	}
	h, _ := newTestHost(t, svc)
	cancel, done := startTestHost(t, h)
	defer func() { cancel(); waitStopped(t, done) }()
	if got := <-observed; got != h.log {
		t.Fatal("Start did not receive the host logger")
	}
}

func TestDrainBeforeStoppingServices(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var prematurelyStopped atomic.Bool
	svc := &testService{handle: func(c *core.Context) { close(entered); <-release; c.String(200, "complete") }}
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
	svc := &testService{handle: func(c *core.Context) { c.String(200, "ok") }}
	h, manager := newTestHost(t, svc)
	other, _ := newTestHost(t, &testService{handle: func(c *core.Context) { c.String(200, "other") }})
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

func TestHTTPServerReadHeaderTimeout(t *testing.T) {
	for _, test := range []struct {
		name                 string
		config               conf.HTTPServerConfig
		wantRead, wantHeader time.Duration
	}{
		{name: "defaults", wantRead: 20 * time.Second, wantHeader: 5 * time.Second},
		{
			name: "read override", config: conf.HTTPServerConfig{ReadTimeout: 12000},
			wantRead: 12 * time.Second, wantHeader: 5 * time.Second,
		},
		{
			name: "header override", config: conf.HTTPServerConfig{ReadHeaderTimeout: 1200},
			wantRead: 20 * time.Second, wantHeader: 1200 * time.Millisecond,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := NewHTTPServer(&conf.Config{HTTPServer: test.config})
			if server.ReadTimeout != test.wantRead || server.ReadHeaderTimeout != test.wantHeader {
				t.Fatalf(
					"read timeout = %v, header timeout = %v",
					server.ReadTimeout,
					server.ReadHeaderTimeout,
				)
			}
		})
	}
}

func TestHTTPTransportChangesRequireRestart(t *testing.T) {
	_, manager := newTestHostWithHTTPConfig(t, &testService{handle: func(*core.Context) {}}, conf.HTTPServerConfig{
		TrustedProxies: []string{"192.0.2.0/24"},
	})
	for _, test := range []struct {
		name   string
		change func(*conf.HTTPServerConfig)
	}{
		{name: "header timeout", change: func(cfg *conf.HTTPServerConfig) { cfg.ReadHeaderTimeout = 1000 }},
		{name: "prestop delay", change: func(cfg *conf.HTTPServerConfig) { cfg.PreStopDelay = 1000 }},
		{name: "trusted proxies", change: func(cfg *conf.HTTPServerConfig) { cfg.TrustedProxies[0] = "198.51.100.0/24" }},
		{name: "admin listener", change: func(cfg *conf.HTTPServerConfig) { cfg.AdminListen = "127.0.0.1:0" }},
		{name: "TLS", change: func(cfg *conf.HTTPServerConfig) {
			cfg.TLS = conf.HTTPServerTLSConfig{Enable: true, CertFile: "cert.pem", KeyFile: "key.pem"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			next := manager.Snapshot()
			test.change(&next.HTTPServer)
			if err := manager.Update(next); !errors.Is(err, ErrRestartRequired) {
				t.Fatalf("expected restart required, got %v", err)
			}
		})
	}
	if got := manager.Snapshot().HTTPServer.TrustedProxies[0]; got != "192.0.2.0/24" {
		t.Fatalf("rejected config changed proxy to %q", got)
	}
}

func TestHTTPHostTrustedProxies(t *testing.T) {
	for _, test := range []struct {
		name    string
		proxies []string
		want    string
	}{
		{name: "disabled by default", want: "192.0.2.1"},
		{name: "trusted remote address", proxies: []string{"192.0.2.0/24"}, want: "203.0.113.1"},
		{name: "untrusted remote address", proxies: []string{"198.51.100.0/24"}, want: "192.0.2.1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, _ := newTestHostWithHTTPConfig(t, &testService{handle: func(c *core.Context) {
				c.String(http.StatusOK, c.ClientIP())
			}}, conf.HTTPServerConfig{TrustedProxies: test.proxies})
			request := httptest.NewRequest(http.MethodGet, "/business", nil)
			request.RemoteAddr = "192.0.2.1:1234"
			request.Header.Set("X-Forwarded-For", "203.0.113.1")
			recorder := httptest.NewRecorder()
			h.Handler().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK || recorder.Body.String() != test.want {
				t.Fatalf(
					"client IP response = %d %q, want %s",
					recorder.Code,
					recorder.Body.String(),
					test.want,
				)
			}
		})
	}
	manager, err := conf.NewManagerFromConfig(&conf.Config{
		AppName: "test", AppCode: 100, RunMode: conf.RunModeRelease,
		HTTPServer: conf.HTTPServerConfig{TrustedProxies: []string{"invalid"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	log, err := logger.NewLogger(&logger.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	if h, err := NewHTTPHost(manager, log, nil); err == nil {
		h.Close()
		t.Fatal("host accepted invalid trusted proxies")
	}
}

func TestHTTPHostReadinessChecks(t *testing.T) {
	h, _ := newTestHost(t, &testService{handle: func(*core.Context) {}})
	h.RegisterHealthCheck("db", func(context.Context) error { return nil })
	h.RegisterHealthCheck("db", nil)
	h.RegisterHealthCheck(" ", func(context.Context) error { return errors.New("invalid name") })
	client := &http.Client{Timeout: 3 * time.Second}
	recorder := httptest.NewRecorder()
	h.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable ||
		recorder.Body.String() != `{"status":"unavailable","checks":{}}` {
		t.Fatalf("initial readiness = %d %s", recorder.Code, recorder.Body)
	}
	cancel, done := startTestHost(t, h)
	defer func() { cancel(); waitStopped(t, done) }()
	base := "http://" + h.Address()
	assertHTTPResponse(t, client, base+"/livez", http.StatusOK, `{"status":"ok"}`)
	assertHTTPResponse(t, client, base+"/readyz", http.StatusOK, `{"status":"ok","checks":{"db":"ok"}}`)

	var registered sync.WaitGroup
	registered.Go(func() {
		for range 100 {
			h.RegisterHealthCheck("db", func(context.Context) error { return nil })
		}
	})
	defer registered.Wait()
	assertHTTPResponse(t, client, base+"/readyz", http.StatusOK, `{"status":"ok","checks":{"db":"ok"}}`)
	registered.Wait()
	h.RegisterHealthCheck("cache", func(context.Context) error { return errors.New("offline") })
	assertHTTPResponse(
		t,
		client,
		base+"/readyz",
		http.StatusServiceUnavailable,
		`{"status":"unavailable","checks":{"cache":"failed","db":"ok"}}`,
	)
	assertHTTPResponse(t, client, base+"/livez", http.StatusOK, `{"status":"ok"}`)
}

func TestHTTPHostAdminListener(t *testing.T) {
	h, manager := newTestHostWithSetup(t, &testService{handle: func(c *core.Context) {
		c.String(http.StatusOK, "main")
	}}, conf.HTTPServerConfig{
		Listen: "127.0.0.1:0", AdminListen: "127.0.0.1:0",
		PprofEnable: true, MetricsEnable: true, SwaggerEnable: true,
	}, func(engine *core.Engine, cfg *conf.Config) error {
		return httpintegration.InstallWithAuth(engine, cfg, func(c *core.Context) { c.Next() })
	})
	if h.AdminAddress() != "" {
		t.Fatal("admin address exposed before binding")
	}
	cancel, done := startTestHost(t, h)
	t.Cleanup(func() {
		cancel()
		if done != nil {
			waitStopped(t, done)
		}
	})
	if h.AdminAddress() == "" || h.AdminAddress() == h.Address() {
		t.Fatalf("main address %q, admin address %q", h.Address(), h.AdminAddress())
	}
	mainURL, adminURL := "http://"+h.Address(), "http://"+h.AdminAddress()
	client := &http.Client{Timeout: 3 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	for _, path := range []string{"/ping", "/healthz", "/livez", "/readyz"} {
		assertHTTPResponse(t, client, adminURL+path, http.StatusOK, "")
		assertHTTPResponse(t, client, mainURL+path, http.StatusOK, "")
	}
	for _, path := range []string{"/pprof", "/metrics", "/swagger/index.html"} {
		assertHTTPResponse(t, client, adminURL+path, http.StatusOK, "")
		assertHTTPResponse(t, client, mainURL+path, http.StatusNotFound, "")
	}
	assertHTTPResponse(t, client, mainURL+"/business", http.StatusOK, "main")
	assertHTTPResponse(t, client, adminURL+"/business", http.StatusNotFound, "")
	next := manager.Snapshot()
	next.HTTPServer.MetricsEnable = false
	if err := manager.Update(next); err != nil {
		t.Fatal(err)
	}
	assertHTTPResponse(t, client, adminURL+"/metrics", http.StatusNotFound, "")

	cancel()
	waitStopped(t, done)
	done = nil
	for _, address := range []string{h.Address(), h.AdminAddress()} {
		connection, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			connection.Close()
			t.Errorf("shutdown left listener open at %s", address)
		}
	}
}

func TestHTTPHostAdminBindingFailureReleasesMainListener(t *testing.T) {
	admin, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	main, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mainAddress := main.Addr().String()
	main.Close()
	service := &testService{handle: func(*core.Context) {}}
	h, _ := newTestHostWithHTTPConfig(t, service, conf.HTTPServerConfig{
		Listen: mainAddress, AdminListen: admin.Addr().String(),
	})
	if err := h.Run(t.Context()); err == nil || !strings.Contains(err.Error(), "listen on admin address") {
		t.Fatalf("expected admin bind error, got %v", err)
	}
	if h.Address() != "" || h.AdminAddress() != "" || service.starts.Load() != 0 {
		t.Fatal("host published a partial startup")
	}
	main, err = net.Listen("tcp", mainAddress)
	if err != nil {
		t.Fatal("main listener leaked:", err)
	}
	main.Close()
}

func TestHTTPHostTLS(t *testing.T) {
	certFile, keyFile, root, signer := makeTestTLSRoot(t)
	clientCertificate, _ := issueTestTLSCertificate(t, &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "verified client"},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}, signer.Leaf, signer.PrivateKey.(ed25519.PrivateKey))
	for _, mutualTLS := range []bool{false, true} {
		name := "TLS"
		if mutualTLS {
			name = "mTLS"
		}
		t.Run(name, func(t *testing.T) {
			config := conf.HTTPServerTLSConfig{Enable: true, CertFile: certFile, KeyFile: keyFile}
			if mutualTLS {
				config.ClientCAFile = certFile
			}
			h, _ := newTestHostWithHTTPConfig(
				t,
				&testService{handle: func(*core.Context) {}},
				conf.HTTPServerConfig{
					TLS: config, AdminListen: "127.0.0.1:0",
				},
			)
			logFile := filepath.Join(t.TempDir(), "host.log")
			log, err := logger.NewLogger(
				&logger.Config{Outputs: []logger.Output{{Type: "file", Path: logFile}}},
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(log.Close)
			h.log = log
			cancel, done := startTestHost(t, h)
			defer func() {
				cancel()
				if done != nil {
					waitStopped(t, done)
				}
			}()

			configureClient := func(config *tls.Config) *http.Client {
				transport := &http.Transport{TLSClientConfig: config, ForceAttemptHTTP2: true}
				t.Cleanup(transport.CloseIdleConnections)
				return &http.Client{Transport: transport, Timeout: 3 * time.Second}
			}
			url := "https://" + h.Address() + "/livez"
			tlsConfig := &tls.Config{RootCAs: root}
			if mutualTLS {
				assertTLSRejected(t, configureClient(tlsConfig), url)
				untrusted, _ := issueTestTLSCertificate(t, &x509.Certificate{
					SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "untrusted client"},
					ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
				}, nil, nil)
				rogue := tlsConfig.Clone()
				rogue.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
					return &untrusted, nil
				}
				assertTLSRejected(t, configureClient(rogue), url)
				tlsConfig = tlsConfig.Clone()
				tlsConfig.Certificates = []tls.Certificate{clientCertificate}
			}
			assertHTTPResponse(t, configureClient(tlsConfig), url, http.StatusOK, `{"status":"ok"}`)
			tls11 := tlsConfig.Clone()
			tls11.MinVersion, tls11.MaxVersion = tls.VersionTLS11, tls.VersionTLS11
			assertTLSRejected(t, configureClient(tls11), url)
			assertHTTPResponse(
				t,
				&http.Client{Timeout: time.Second},
				"http://"+h.AdminAddress()+"/livez",
				http.StatusOK,
				`{"status":"ok"}`,
			)
			cancel()
			waitStopped(t, done)
			done = nil
			data, err := os.ReadFile(logFile)
			if err != nil || !strings.Contains(string(data), `"level":"error"`) ||
				!strings.Contains(string(data), "TLS handshake error") {
				t.Fatalf("net/http error log was not captured: %v %s", err, data)
			}
		})
	}
}

func TestHTTPHostRejectsInvalidTLSFilesBeforeStarting(t *testing.T) {
	certFile, keyFile, _, _ := makeTestTLSRoot(t)
	for _, test := range []struct {
		name, key, clientCA, wantError string
	}{
		{name: "missing key", key: keyFile + ".missing", wantError: "load HTTP TLS certificate"},
		{name: "missing client CA", key: keyFile, clientCA: keyFile + ".missing", wantError: "read HTTP TLS client CA"},
		{name: "invalid client CA", key: keyFile, clientCA: keyFile, wantError: "no valid certificates"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &testService{handle: func(*core.Context) {}}
			h, _ := newTestHostWithHTTPConfig(t, service, conf.HTTPServerConfig{
				TLS: conf.HTTPServerTLSConfig{
					Enable:       true,
					CertFile:     certFile,
					KeyFile:      test.key,
					ClientCAFile: test.clientCA,
				},
			})
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if err := h.Run(ctx); err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("expected %q, got %v", test.wantError, err)
			}
			if h.Address() != "" || service.starts.Load() != 0 {
				t.Fatal("invalid TLS configuration started the host")
			}
		})
	}
}

func makeTestTLSRoot(t *testing.T) (string, string, *x509.CertPool, tls.Certificate) {
	t.Helper()
	cert, keyPEM := issueTestTLSCertificate(t, &x509.Certificate{
		SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true,
		Subject:     pkix.Name{CommonName: "local server and test CA"},
		KeyUsage:    x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}, nil, nil)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	root := x509.NewCertPool()
	root.AddCert(cert.Leaf)
	return certFile, keyFile, root, cert
}

func issueTestTLSCertificate(
	t *testing.T,
	template, issuer *x509.Certificate,
	issuerKey ed25519.PrivateKey,
) (tls.Certificate, []byte) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template.NotBefore = time.Now().Add(-time.Minute)
	template.NotAfter = time.Now().Add(time.Hour)
	template.KeyUsage |= x509.KeyUsageDigitalSignature
	if issuer == nil {
		issuer, issuerKey = template, privateKey
	}
	der, err := x509.CreateCertificate(rand.Reader, template, issuer, publicKey, issuerKey)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: privateKey, Leaf: leaf},
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
}

func assertTLSRejected(t *testing.T, client *http.Client, url string) {
	t.Helper()
	response, err := client.Get(url)
	if err == nil {
		response.Body.Close()
		t.Fatalf("TLS connection succeeded with status %d", response.StatusCode)
	}
}

func assertHTTPResponse(t *testing.T, client *http.Client, url string, status int, body string) {
	t.Helper()
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status || body != "" && string(payload) != body {
		t.Fatalf("GET %s = %d %q, want %d %q", url, response.StatusCode, payload, status, body)
	}
}

func TestConstructionDoesNotMissReplacement(t *testing.T) {
	manager, err := conf.NewManagerFromConfig(
		&conf.Config{AppName: "test", AppCode: 100, RunMode: conf.RunModeRelease},
	)
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
	setup := func(*core.Engine, *conf.Config) error {
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
		h, err := NewHTTPHost(
			manager,
			log,
			setup,
			&testService{handle: func(c *core.Context) { c.String(200, "ok") }},
		)
		constructed <- result{host: h, err: err}
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
