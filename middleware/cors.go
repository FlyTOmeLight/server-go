package middleware

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

// CORSConfig specifies the origins, methods and headers allowed across origins.
// MaxAge controls how long browsers may cache a preflight response.
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// CORS handles browser preflights and sends headers for allowed origins.
// Credentials require an explicitly configured origin.
func CORS(cfg CORSConfig) core.HandlerFunc {
	origins := slices.Clone(cfg.AllowOrigins)
	allowAnyOrigin := !cfg.AllowCredentials && slices.Contains(origins, "*")
	methods := strings.Join(cfg.AllowMethods, ", ")
	headers := strings.Join(cfg.AllowHeaders, ", ")
	exposed := strings.Join(cfg.ExposeHeaders, ", ")
	return func(c *core.Context) {
		c.Writer.Header().Add("Vary", "Origin")
		origin := c.GetHeader("Origin")
		options := origin != "" && c.Request.Method == http.MethodOptions
		if options {
			c.Writer.Header().Add("Vary", "Access-Control-Request-Method")
		}
		preflight := options &&
			c.GetHeader("Access-Control-Request-Method") != ""
		if origin != "" && origin != "*" && (allowAnyOrigin || slices.Contains(origins, origin)) {
			allowedOrigin := origin
			if allowAnyOrigin {
				allowedOrigin = "*"
			}
			c.Header("Access-Control-Allow-Origin", allowedOrigin)
			if cfg.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			if preflight {
				c.Header("Access-Control-Allow-Methods", methods)
				c.Header("Access-Control-Allow-Headers", headers)
				if cfg.MaxAge > 0 {
					c.Header(
						"Access-Control-Max-Age",
						strconv.FormatInt(int64(cfg.MaxAge/time.Second), 10),
					)
				}
			} else {
				c.Header("Access-Control-Expose-Headers", exposed)
			}
		}
		if preflight {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
