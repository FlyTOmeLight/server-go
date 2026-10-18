package middleware

import (
	"net/http"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

// GinContextPublisher copies verified identity from the request context onto a
// Gin context after HTTP authorization middleware succeeds.
type GinContextPublisher func(c *core.Context, r *http.Request)

// HTTPAuthorization adapts standard net/http authorization middleware to
// server-go. The middleware must write failures to the response writer and
// only call next with an enriched request context on success.
func HTTPAuthorization(
	authorize func(http.Handler) http.Handler,
	publish GinContextPublisher,
) core.HandlerFunc {
	if authorize == nil {
		panic("HTTPAuthorization requires authorization middleware")
	}
	return func(c *core.Context) {
		passed := false
		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			passed = true
			c.Request = r
			if publish != nil {
				publish(c, r)
			}
			c.Next()
		})
		authorize(next).ServeHTTP(c.Writer, c.Request)
		if !passed {
			c.Abort()
		}
	}
}
