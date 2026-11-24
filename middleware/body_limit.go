package middleware

import (
	"net/http"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

// MaxBodyBytes limits reads from the request body for every content type.
// Nonpositive limits leave the body unrestricted by this middleware.
func MaxBodyBytes(limit int64) core.HandlerFunc {
	return func(c *core.Context) {
		if limit > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}
