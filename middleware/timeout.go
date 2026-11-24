package middleware

import (
	"context"
	"time"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

// RequestTimeout gives downstream handlers a request deadline and cancels
// their context when the handler chain returns. Nonpositive timeouts are ignored.
func RequestTimeout(timeout time.Duration) core.HandlerFunc {
	return func(c *core.Context) {
		if timeout <= 0 {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
