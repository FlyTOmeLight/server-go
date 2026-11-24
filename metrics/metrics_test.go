package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestHTTPMetricsScrapeUsesRouteTemplates(t *testing.T) {
	registry := NewRegistry()
	instrumentation, err := NewHTTPMetrics(registry, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewHTTPMetrics(registry, "test"); err == nil {
		t.Fatal("expected duplicate registration error")
	}

	engine := gin.New()
	engine.GET("/metrics", HandlerFor(registry))
	engine.GET("/isolated", HandlerFor(NewRegistry()))
	api := engine.Group("/", instrumentation.Middleware())
	entered := make(chan struct{})
	release := make(chan struct{})
	finishRequest := sync.OnceFunc(func() { close(release) })
	done := make(chan struct{})
	api.GET("/items/:id", func(c *core.Context) {
		close(entered)
		<-release
		c.Status(http.StatusNoContent)
	})
	engine.NoRoute(instrumentation.Middleware())

	go func() {
		defer close(done)
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/private-id", nil))
	}()
	t.Cleanup(func() { <-done })
	defer finishRequest()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach handler")
	}
	scrape := func(path string) string {
		t.Helper()
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("scrape status = %d: %s", recorder.Code, recorder.Body)
		}
		return recorder.Body.String()
	}
	if body := scrape("/metrics"); !strings.Contains(body, "test_http_requests_in_flight 1") {
		t.Fatalf("missing active request: %s", body)
	}
	finishRequest()
	<-done
	for _, path := range []string{"/scan-one", "/scan-two"} {
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	if body := scrape("/isolated"); body != "" {
		t.Fatalf("isolated registry unexpectedly contains metrics: %s", body)
	}
	body := scrape("/metrics")
	for _, metric := range []string{
		"test_http_requests_in_flight 0",
		`test_http_requests_total{method="GET",route="/items/:id",status="204"} 1`,
		`test_http_requests_total{method="GET",route="unmatched",status="404"} 2`,
		`test_http_request_duration_seconds_count{method="GET",route="/items/:id",status="204"} 1`,
		`test_http_request_duration_seconds_count{method="GET",route="unmatched",status="404"} 2`,
	} {
		if !strings.Contains(body, metric) {
			t.Errorf("missing metric %s in %s", metric, body)
		}
	}
	if strings.Contains(body, "private-id") || strings.Contains(body, "/scan-") {
		t.Fatalf("raw paths leaked into metric labels: %s", body)
	}
}

func TestHTTPMetricsRollsBackPartialRegistration(t *testing.T) {
	registry := NewRegistry()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Current HTTP requests in flight.",
	})
	registry.MustRegister(gauge)
	var duplicate prometheus.AlreadyRegisteredError
	if _, err := NewHTTPMetrics(registry, ""); !errors.As(err, &duplicate) {
		t.Fatalf("expected gauge registration conflict, got %v", err)
	}
	registry.Unregister(gauge)
	if _, err := NewHTTPMetrics(registry, ""); err != nil {
		t.Fatalf("retry after removing the conflict: %v", err)
	}
}
