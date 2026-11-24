package middleware

import (
	"bufio"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/auth"
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
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			passed = true
			original := c.Writer
			var wrapped *authorizationResponseWriter
			if w != original {
				wrapped = &authorizationResponseWriter{
					ResponseWriter: original,
					wrapped:        w,
					status:         original.Status(),
					size:           original.Size(),
				}
				c.Writer = wrapped
				defer func() { c.Writer = original }()
			}
			c.Request = r
			if publish != nil {
				publish(c, r)
				auth.SyncIdentityToRequestContext(c)
			}
			c.Next()
			if wrapped != nil && wrapped.statusSet {
				wrapped.WriteHeaderNow()
			}
		})
		authorize(next).ServeHTTP(c.Writer, c.Request)
		if !passed {
			c.Abort()
		}
	}
}

// authorizationResponseWriter keeps Gin's lazy status and body accounting while
// sending the response through the writer supplied by net/http middleware.
type authorizationResponseWriter struct {
	gin.ResponseWriter
	wrapped   http.ResponseWriter
	status    int
	size      int
	statusSet bool
}

func (w *authorizationResponseWriter) Unwrap() http.ResponseWriter { return w.wrapped }

func (w *authorizationResponseWriter) Header() http.Header { return w.wrapped.Header() }

func (w *authorizationResponseWriter) Status() int { return w.status }

func (w *authorizationResponseWriter) Size() int { return w.size }

func (w *authorizationResponseWriter) Written() bool { return w.size >= 0 }

func (w *authorizationResponseWriter) WriteHeader(code int) {
	if code > 0 && !w.Written() {
		w.status = code
		w.statusSet = true
	}
}

func (w *authorizationResponseWriter) WriteHeaderNow() {
	if !w.Written() {
		w.size = 0
		w.wrapped.WriteHeader(w.status)
	}
}

func (w *authorizationResponseWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	n, err := w.wrapped.Write(data)
	w.size += n
	return n, err
}

func (w *authorizationResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *authorizationResponseWriter) Flush() {
	_ = w.FlushError()
}

func (w *authorizationResponseWriter) FlushError() error {
	w.WriteHeaderNow()
	return http.NewResponseController(w.wrapped).Flush()
}

func (w *authorizationResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, buffer, err := http.NewResponseController(w.wrapped).Hijack()
	if err == nil && !w.Written() {
		w.size = 0
	}
	return conn, buffer, err
}
