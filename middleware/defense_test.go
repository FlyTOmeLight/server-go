package middleware

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gin-gonic/gin"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestCORS(t *testing.T) {
	for _, tc := range []struct {
		name, method, origin, requested, wantOrigin string
		origins                                     []string
		credentials, wantCredentials                bool
		status                                      int
	}{
		{name: "without origin", method: http.MethodGet, status: 200},
		{
			name: "allowed actual", method: http.MethodGet, origin: "https://example.com", wantOrigin: "https://example.com",
			origins: []string{"https://example.com"}, credentials: true, wantCredentials: true, status: 200,
		},
		{
			name: "denied", method: http.MethodGet, origin: "https://example.com.other.test",
			origins: []string{"https://example.com"}, status: 200,
		},
		{
			name: "public wildcard", method: http.MethodGet, origin: "https://example.com", wantOrigin: "*",
			origins: []string{"*"}, status: 200,
		},
		{
			name: "wildcard with credentials", method: http.MethodGet, origin: "https://example.com",
			origins: []string{"*"}, credentials: true, status: 200,
		},
		{
			name: "explicit origin with wildcard and credentials", method: http.MethodGet, origin: "https://example.com",
			wantOrigin: "https://example.com", origins: []string{"*", "https://example.com"},
			credentials: true, wantCredentials: true, status: 200,
		},
		{
			name: "preflight", method: http.MethodOptions, requested: http.MethodPost, origin: "https://example.com",
			wantOrigin: "https://example.com", origins: []string{"https://example.com"}, status: 204,
		},
		{
			name: "denied preflight", method: http.MethodOptions, requested: http.MethodPost, origin: "https://other.test",
			origins: []string{"https://example.com"}, status: 204,
		},
		{
			name: "ordinary options", method: http.MethodOptions, origin: "https://example.com",
			wantOrigin: "https://example.com", origins: []string{"https://example.com"}, status: 200,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(func(c *core.Context) { c.Writer.Header().Add("Vary", "Accept-Encoding"); c.Next() })
			engine.Use(CORS(CORSConfig{
				AllowOrigins: tc.origins, AllowCredentials: tc.credentials,
				AllowMethods:  []string{http.MethodGet, http.MethodPost},
				AllowHeaders:  []string{"Authorization", "Content-Type"},
				ExposeHeaders: []string{"Location"}, MaxAge: 2 * time.Minute,
			}))
			engine.Any("/", func(c *core.Context) { c.String(http.StatusOK, "handler") })
			request := httptest.NewRequest(tc.method, "/", nil)
			request.Header.Set("Origin", tc.origin)
			request.Header.Set("Access-Control-Request-Method", tc.requested)
			request.Header.Set("Access-Control-Request-Headers", "authorization")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			header := recorder.Result().Header
			if recorder.Code != tc.status || header.Get("Access-Control-Allow-Origin") != tc.wantOrigin ||
				(header.Get("Access-Control-Allow-Credentials") == "true") != tc.wantCredentials ||
				!slices.Contains(header.Values("Vary"), "Origin") ||
				!slices.Contains(header.Values("Vary"), "Accept-Encoding") {
				t.Fatalf("unexpected status or headers: %d %v", recorder.Code, header)
			}
			if tc.status == http.StatusNoContent && recorder.Body.Len() != 0 {
				t.Fatalf("preflight reached the handler: %s", recorder.Body)
			}
			switch {
			case tc.requested != "" && tc.wantOrigin != "":
				if header.Get("Access-Control-Allow-Methods") != "GET, POST" ||
					header.Get("Access-Control-Allow-Headers") != "Authorization, Content-Type" ||
					header.Get("Access-Control-Max-Age") != "120" {
					t.Fatalf("unexpected preflight headers: %v", header)
				}
			case tc.wantOrigin != "":
				if header.Get("Access-Control-Expose-Headers") != "Location" {
					t.Fatalf("unexpected actual headers: %v", header)
				}
			default:
				if header.Get("Access-Control-Allow-Methods") != "" ||
					header.Get("Access-Control-Allow-Headers") != "" ||
					header.Get("Access-Control-Expose-Headers") != "" {
					t.Fatalf("disallowed origin received CORS headers: %v", header)
				}
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler core.HandlerFunc
		headers map[string]string
	}{
		{
			name: "defaults", handler: SecurityHeaders(),
			headers: map[string]string{
				"X-Frame-Options": "DENY", "Referrer-Policy": "strict-origin-when-cross-origin",
				"Strict-Transport-Security": "", "Content-Security-Policy": "", "Cache-Control": "",
			},
		},
		{
			name: "configured",
			handler: SecurityHeaders(SecurityHeadersConfig{
				FrameOptions: "SAMEORIGIN", ReferrerPolicy: "no-referrer",
				StrictTransportSecurity: "max-age=3600", ContentSecurityPolicy: "default-src 'none'",
				CacheControl: "no-store",
			}),
			headers: map[string]string{
				"X-Frame-Options": "SAMEORIGIN", "Referrer-Policy": "no-referrer",
				"Strict-Transport-Security": "max-age=3600", "Content-Security-Policy": "default-src 'none'",
				"Cache-Control": "no-store",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(tc.handler)
			engine.GET("/", func(c *core.Context) { c.AbortWithStatus(http.StatusForbidden) })
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			header := recorder.Result().Header
			if recorder.Code != http.StatusForbidden || header.Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("unexpected response: %d %v", recorder.Code, header)
			}
			for name, want := range tc.headers {
				if got := header.Get(name); got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestRateLimitRefillsAndEvictsIdleKeys(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		engine := gin.New()
		engine.Use(RateLimit(RateLimitConfig{
			RequestsPerSecond: 1, Burst: 2, MaxKeys: 2, IdleTTL: 5 * time.Second,
			KeyFunc: func(c *core.Context) string { return c.GetHeader("Test-Key") },
		}))
		engine.GET("/", func(c *core.Context) { c.Status(http.StatusNoContent) })
		check := func(key string, want int) {
			t.Helper()
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Test-Key", key)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			if recorder.Code != want || want == 429 &&
				(recorder.Header().Get("Retry-After") != "1" || !strings.Contains(recorder.Body.String(), `"code":"TooManyRequests"`)) {
				t.Fatalf(
					"key %q: got %d %v %s, want %d",
					key,
					recorder.Code,
					recorder.Header(),
					recorder.Body,
					want,
				)
			}
		}
		check("first", 204)
		check("first", 204)
		check("first", 429)
		check("second", 204)
		check("third", 429)
		time.Sleep(time.Second)
		check("first", 204)
		check("first", 429)
		time.Sleep(4 * time.Second)
		check("third", 204)
		check("second", 429)
	})
}

func TestRateLimitConcurrentBurst(t *testing.T) {
	const burst = 3
	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	engine.Use(RateLimit(RateLimitConfig{RequestsPerSecond: 0.001, Burst: burst}))
	engine.GET("/", func(c *core.Context) { c.Status(http.StatusNoContent) })
	var admitted atomic.Int32
	var group sync.WaitGroup
	for range 20 {
		group.Go(func() {
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			switch recorder.Code {
			case http.StatusNoContent:
				admitted.Add(1)
			case http.StatusTooManyRequests:
			default:
				t.Errorf("unexpected status: %d", recorder.Code)
			}
		})
	}
	group.Wait()
	if admitted.Load() != burst {
		t.Fatalf("admitted %d requests, want %d", admitted.Load(), burst)
	}
}

func TestMaxInFlightRejectsAndReleasesSlots(t *testing.T) {
	for _, panicFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("panic=%t", panicFirst), func(t *testing.T) {
			engine := gin.New()
			engine.Use(gin.RecoveryWithWriter(io.Discard), MaxInFlight(1))
			entered := make(chan struct{})
			release := make(chan struct{})
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			engine.GET("/", func(c *core.Context) {
				if c.GetHeader("Block") != "" {
					close(entered)
					select {
					case <-release:
					case <-ctx.Done():
						return
					}
					if panicFirst {
						panic("handler interrupted")
					}
				}
				c.Status(http.StatusNoContent)
			})
			first := httptest.NewRecorder()
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				request := httptest.NewRequest(http.MethodGet, "/", nil)
				request.Header.Set("Block", "true")
				engine.ServeHTTP(first, request)
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("handler did not start")
			}
			rejected := httptest.NewRecorder()
			engine.ServeHTTP(rejected, httptest.NewRequest(http.MethodGet, "/", nil))
			close(release)
			select {
			case <-finished:
			case <-ctx.Done():
				t.Fatal("handler did not return")
			}
			if rejected.Code != http.StatusServiceUnavailable ||
				rejected.Header().Get("Retry-After") != "1" ||
				!strings.Contains(rejected.Body.String(), `"code":"ServiceUnavailable"`) {
				t.Fatalf(
					"unexpected rejection: %d %v %s",
					rejected.Code,
					rejected.Header(),
					rejected.Body,
				)
			}
			want := http.StatusNoContent
			if panicFirst {
				want = http.StatusInternalServerError
			}
			next := httptest.NewRecorder()
			engine.ServeHTTP(next, httptest.NewRequest(http.MethodGet, "/", nil))
			if first.Code != want || next.Code != http.StatusNoContent {
				t.Fatalf("unexpected codes: first=%d, next=%d", first.Code, next.Code)
			}
		})
	}
}

func TestRequestTimeout(t *testing.T) {
	for _, tc := range []struct {
		name         string
		timeout      time.Duration
		parent       time.Duration
		wantDeadline time.Duration
		wait         bool
	}{
		{name: "middleware deadline", timeout: time.Second, wantDeadline: time.Second, wait: true},
		{name: "earlier parent deadline", timeout: time.Hour, parent: time.Second, wantDeadline: time.Second, wait: true},
		{name: "cancel after return", timeout: time.Second, wantDeadline: time.Second},
		{name: "disabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				engine := gin.New()
				engine.Use(RequestTimeout(tc.timeout))
				parent := t.Context()
				if tc.parent > 0 {
					var cancel context.CancelFunc
					parent, cancel = context.WithTimeout(parent, tc.parent)
					defer cancel()
				}
				now := time.Now()
				var observed context.Context
				engine.GET("/", func(c *core.Context) {
					observed = c.Request.Context()
					deadline, ok := observed.Deadline()
					if ok != (tc.wantDeadline > 0) || ok && deadline.Sub(now) != tc.wantDeadline {
						t.Errorf("unexpected deadline: %v %t", deadline, ok)
					}
					if tc.wait {
						<-observed.Done()
						if !errors.Is(observed.Err(), context.DeadlineExceeded) {
							t.Errorf("unexpected deadline error: %v", observed.Err())
						}
					}
					c.Status(http.StatusAccepted)
				})
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(
					recorder,
					httptest.NewRequestWithContext(parent, http.MethodGet, "/", nil),
				)
				if recorder.Code != http.StatusAccepted {
					t.Fatalf("unexpected status: %d", recorder.Code)
				}
				if !tc.wait && tc.timeout > 0 && observed.Err() != context.Canceled ||
					tc.timeout == 0 && observed != parent {
					t.Fatalf("unexpected request context after handler: %v", observed)
				}
			})
		})
	}
}

func TestMaxBodyBytes(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body string
		limit                   int64
		nilBody                 bool
		want                    int
	}{
		{name: "exact size", contentType: "text/plain", body: "four", limit: 4, want: 200},
		{name: "octet stream", contentType: "application/octet-stream", body: "larger", limit: 4, want: 413},
		{name: "JSON", contentType: "application/json", body: `{"large":true}`, limit: 4, want: 413},
		{name: "no type", body: "larger", limit: 4, want: 413},
		{name: "no body", limit: 4, nilBody: true, want: 200},
		{name: "disabled", body: "larger", limit: 0, want: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(MaxBodyBytes(tc.limit))
			engine.POST("/", func(c *core.Context) {
				if c.Request.Body == nil {
					if !tc.nilBody {
						t.Error("body unexpectedly missing")
					}
					c.Status(http.StatusOK)
					return
				}
				body, err := io.ReadAll(c.Request.Body)
				if err != nil {
					var tooLarge *http.MaxBytesError
					if !errors.As(err, &tooLarge) || tooLarge.Limit != tc.limit ||
						int64(len(body)) != tc.limit {
						t.Errorf("unexpected read error after %d bytes: %v", len(body), err)
					}
					c.Status(http.StatusRequestEntityTooLarge)
					return
				}
				c.String(http.StatusOK, "%s", body)
			})
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", tc.contentType)
			request.ContentLength = -1
			if tc.nilBody {
				request.Body = nil
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			if recorder.Code != tc.want || tc.want == 200 && recorder.Body.String() != tc.body {
				t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body)
			}
		})
	}
}
