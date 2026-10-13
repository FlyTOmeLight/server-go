package middleware

import (
	"net/http"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

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
