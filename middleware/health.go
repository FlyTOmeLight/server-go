package middleware

import (
	"context"
	"maps"
	"net/http"
	"time"

	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// HealthCheck reports whether a dependency is ready and honors context cancellation.
type HealthCheck func(ctx context.Context) error

// Healthz checks the owning host's readiness without process-wide configuration.
func Healthz(ready func() bool) core.HandlerFunc {
	return func(c *core.Context) {
		if ready != nil && ready() {
			c.String(http.StatusOK, "healthz")
			return
		}
		c.AbortWithStatus(http.StatusServiceUnavailable)
	}
}

func Liveness() core.HandlerFunc {
	return func(c *core.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

func Readiness(ready func() bool, checks map[string]HealthCheck) core.HandlerFunc {
	snapshot := maps.Clone(checks)
	return ReadinessWithChecks(ready, func() map[string]HealthCheck { return snapshot })
}

// ReadinessWithChecks reads an immutable snapshot of the checks for each request.
func ReadinessWithChecks(ready func() bool, checks func() map[string]HealthCheck) core.HandlerFunc {
	type response struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	return func(c *core.Context) {
		statuses := make(map[string]string)
		unavailable := func() {
			c.JSON(http.StatusServiceUnavailable, response{Status: "unavailable", Checks: statuses})
		}
		if ready == nil || !ready() {
			unavailable()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		snapshot := checks()
		type result struct{ name, status string }
		results := make(chan result, len(snapshot))
		for name, check := range snapshot {
			statuses[name] = "timeout"
			go func() {
				results <- result{name: name, status: healthCheckStatus(ctx, name, check)}
			}()
		}
		passed := true
		for range snapshot {
			select {
			case result := <-results:
				statuses[result.name] = result.status
				passed = passed && result.status == "ok"
			case <-ctx.Done():
				unavailable()
				return
			}
		}
		if !passed || ctx.Err() != nil || !ready() {
			unavailable()
			return
		}
		c.JSON(http.StatusOK, response{Status: "ok", Checks: statuses})
	}
}

func healthCheckStatus(ctx context.Context, name string, check HealthCheck) (status string) {
	defer func() {
		if event := recover(); event != nil {
			logger.Errorf(ctx, "health check %q panicked: %v", name, event)
			status = "failed"
		}
	}()
	if ctx.Err() != nil {
		return "timeout"
	}
	if check == nil {
		return "failed"
	}
	err := check(ctx)
	if ctx.Err() != nil {
		return "timeout"
	}
	if err != nil {
		return "failed"
	}
	return "ok"
}
