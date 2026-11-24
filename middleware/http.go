package middleware

import (
	"io"
	"net/http"
	"runtime/debug"
	"slices"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/auth"
	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// RequestLog records request metadata without credentials or payloads.
func RequestLog() core.HandlerFunc {
	return requestLog(RequestLogOptions{}, true)
}

type RequestLogOptions struct {
	SkipPaths       []string
	IncludeIdentity bool
}

// RequestLogWithOptions records request metadata with optional identity fields.
func RequestLogWithOptions(opts RequestLogOptions) core.HandlerFunc {
	return requestLog(opts, false)
}

func requestLog(opts RequestLogOptions, legacy bool) core.HandlerFunc {
	skipPaths := slices.Clone(opts.SkipPaths)
	return func(c *core.Context) {
		path := c.Request.URL.Path
		if slices.Contains(skipPaths, path) {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		route := c.FullPath()
		if route == "" || (!legacy && status == http.StatusNotFound) {
			route = path
		}
		ctx := c.Request.Context()
		log := logger.FromContext(ctx)
		const format = "http request method=%s route=%s status=%d duration=%s"
		duration := time.Since(start)
		if legacy {
			log.Infof(ctx, format, c.Request.Method, route, status, duration)
			return
		}
		log = log.With("clientIP", c.ClientIP(), "bytes", max(c.Writer.Size(), 0))
		if opts.IncludeIdentity {
			identity := auth.IdentityFromGin(c)
			log = log.With("tenantId", identity.TenantID, "userId", identity.UserID)
		}
		switch {
		case status >= http.StatusInternalServerError:
			log.Errorf(ctx, format, c.Request.Method, route, status, duration)
		case status >= http.StatusBadRequest:
			log.Warnf(ctx, format, c.Request.Method, route, status, duration)
		default:
			log.Infof(ctx, format, c.Request.Method, route, status, duration)
		}
	}
}

// Recovery returns framework errors before a response starts. Once streaming,
// it stops handling without adding a JSON document to the response.
func Recovery() core.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *core.Context, recovered any) {
		logger.FromContext(c.Request.Context()).
			Errorf(c.Request.Context(), "http handler panic route=%s panic=%v stack=%s", c.FullPath(), recovered, debug.Stack())
		errors.AbortWithError(c, errors.ErrorInternalError)
	})
}
