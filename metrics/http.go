package metrics

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

// NewRegistry returns an isolated registry without default collectors.
func NewRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

// HandlerFor serves metrics collected by the supplied gatherer.
func HandlerFor(gatherer prometheus.Gatherer) core.HandlerFunc {
	return gin.WrapH(promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
}

type HTTPMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

// NewHTTPMetrics registers request metrics with the supplied registerer.
func NewHTTPMetrics(reg prometheus.Registerer, namespace string) (*HTTPMetrics, error) {
	if reg == nil {
		return nil, errors.New("HTTP metrics registerer is required")
	}
	labels := []string{"method", "route", "status"}
	metrics := &HTTPMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total HTTP requests.",
		}, labels),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
		}, labels),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_requests_in_flight",
			Help:      "Current HTTP requests in flight.",
		}),
	}
	collectors := []prometheus.Collector{metrics.requests, metrics.duration, metrics.inFlight}
	for index, collector := range collectors {
		if err := reg.Register(collector); err != nil {
			for _, registered := range collectors[:index] {
				reg.Unregister(registered)
			}
			return nil, fmt.Errorf("register HTTP metrics: %w", err)
		}
	}
	return metrics, nil
}

// Middleware measures requests using the matched route template as a label.
func (m *HTTPMetrics) Middleware() core.HandlerFunc {
	return func(c *core.Context) {
		start := time.Now()
		m.inFlight.Inc()
		defer m.inFlight.Dec()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())
		m.requests.WithLabelValues(method, route, status).Inc()
		m.duration.WithLabelValues(method, route, status).Observe(time.Since(start).Seconds())
	}
}
