package middleware

import (
	"io"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// RequestLog records request metadata without credentials or payloads.
func RequestLog() core.HandlerFunc {
	return func(c *core.Context) {
		start := time.Now()
		c.Next()
		logger.FromContext(c.Request.Context()).
			Infof(c.Request.Context(), "http request method=%s route=%s status=%d duration=%s", c.Request.Method, c.FullPath(), c.Writer.Status(), time.Since(start))
	}
}

// Recovery returns framework errors before a response starts. Once streaming,
// it stops handling without adding a JSON document to the response.
func Recovery() core.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *core.Context, recovered any) {
		logger.FromContext(c.Request.Context()).
			Errorf(c.Request.Context(), "http handler panic route=%s stack=%s", c.FullPath(), debug.Stack())
		errors.AbortWithError(c, errors.ErrorInternalError)
	})
}
