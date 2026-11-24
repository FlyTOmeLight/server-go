package middleware

import (
	"container/list"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// RateLimitConfig configures a token bucket per key. KeyFunc defaults to the
// Gin client IP. Nonpositive Burst, MaxKeys and IdleTTL default to 1, 10000 and
// five minutes, respectively.
type RateLimitConfig struct {
	RequestsPerSecond float64
	Burst             int
	KeyFunc           func(*core.Context) string
	MaxKeys           int
	IdleTTL           time.Duration
}

type rateLimitEntry struct {
	key     string
	seen    time.Time
	limiter *rate.Limiter
}

// RateLimit rejects requests that exceed their key's rate or require a new
// bucket while all slots are active. A nonpositive rate disables the limiter.
func RateLimit(cfg RateLimitConfig) core.HandlerFunc {
	if !(cfg.RequestsPerSecond > 0) {
		return func(c *core.Context) { c.Next() }
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 1
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(c *core.Context) string { return c.ClientIP() }
	}
	if cfg.MaxKeys <= 0 {
		cfg.MaxKeys = 10000
	}
	if cfg.IdleTTL <= 0 {
		cfg.IdleTTL = 5 * time.Minute
	}

	var mu sync.Mutex
	var recent list.List
	entries := make(map[string]*list.Element)
	allow := func(key string) bool {
		mu.Lock()
		defer mu.Unlock()
		now := time.Now()
		cutoff := now.Add(-cfg.IdleTTL)
		for oldest := recent.Front(); oldest != nil &&
			!oldest.Value.(*rateLimitEntry).seen.After(cutoff); oldest = recent.Front() {
			delete(entries, recent.Remove(oldest).(*rateLimitEntry).key)
		}
		element := entries[key]
		if element == nil {
			if len(entries) == cfg.MaxKeys {
				return false
			}
			element = recent.PushBack(&rateLimitEntry{
				key: key, limiter: rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), cfg.Burst),
			})
			entries[key] = element
		}
		entry := element.Value.(*rateLimitEntry)
		entry.seen = now
		recent.MoveToBack(element)
		return entry.limiter.AllowN(now, 1)
	}

	return func(c *core.Context) {
		if !allow(cfg.KeyFunc(c)) {
			c.Header("Retry-After", "1")
			errors.Abort(c, http.StatusTooManyRequests, "TooManyRequests", "request rate limit exceeded")
			return
		}
		c.Next()
	}
}

// MaxInFlight caps concurrent requests. A nonpositive limit disables the cap.
func MaxInFlight(limit int) core.HandlerFunc {
	if limit <= 0 {
		return func(c *core.Context) { c.Next() }
	}
	active := make(chan struct{}, limit)
	return func(c *core.Context) {
		select {
		case active <- struct{}{}:
			defer func() { <-active }()
			c.Next()
		default:
			c.Header("Retry-After", "1")
			errors.Abort(
				c,
				http.StatusServiceUnavailable,
				"ServiceUnavailable",
				"concurrent request limit reached",
			)
		}
	}
}
