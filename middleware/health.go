package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Healthz checks the owning host's readiness without process-wide configuration.
func Healthz(ready func() bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ready != nil && ready() {
			c.String(http.StatusOK, "healthz")
			return
		}
		c.AbortWithStatus(http.StatusServiceUnavailable)
	}
}
