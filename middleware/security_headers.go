package middleware

import core "github.com/FlyTOmeLight/server-go/pkg"

// SecurityHeadersConfig overrides the default headers and enables optional policies.
type SecurityHeadersConfig struct {
	FrameOptions            string
	ReferrerPolicy          string
	StrictTransportSecurity string
	ContentSecurityPolicy   string
	CacheControl            string
}

// SecurityHeaders applies baseline browser security headers.
func SecurityHeaders(config ...SecurityHeadersConfig) core.HandlerFunc {
	var cfg SecurityHeadersConfig
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.FrameOptions == "" {
		cfg.FrameOptions = "DENY"
	}
	if cfg.ReferrerPolicy == "" {
		cfg.ReferrerPolicy = "strict-origin-when-cross-origin"
	}
	return func(c *core.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", cfg.FrameOptions)
		c.Header("Referrer-Policy", cfg.ReferrerPolicy)
		if cfg.StrictTransportSecurity != "" {
			c.Header("Strict-Transport-Security", cfg.StrictTransportSecurity)
		}
		if cfg.ContentSecurityPolicy != "" {
			c.Header("Content-Security-Policy", cfg.ContentSecurityPolicy)
		}
		if cfg.CacheControl != "" {
			c.Header("Cache-Control", cfg.CacheControl)
		}
		c.Next()
	}
}
