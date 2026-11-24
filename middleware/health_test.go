package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestReadiness(t *testing.T) {
	for _, test := range []struct {
		name   string
		ready  bool
		checks map[string]HealthCheck
		code   int
		want   map[string]string
	}{
		{
			name: "starting", code: http.StatusServiceUnavailable, want: map[string]string{},
			checks: map[string]HealthCheck{"db": func(context.Context) error {
				t.Error("ran check while host was unready")
				return nil
			}},
		},
		{
			name: "healthy", ready: true, code: http.StatusOK, want: map[string]string{"db": "ok"},
			checks: map[string]HealthCheck{"db": func(context.Context) error { return nil }},
		},
		{
			name: "failed checks", ready: true, code: http.StatusServiceUnavailable,
			want: map[string]string{"db": "failed", "cache": "ok", "panic": "failed", "nil": "failed"},
			checks: map[string]HealthCheck{
				"db":    func(context.Context) error { return errors.New("private database error") },
				"cache": func(context.Context) error { return nil },
				"panic": func(context.Context) error { panic("private panic") },
				"nil":   nil,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.GET("/readyz", Readiness(func() bool { return test.ready }, test.checks))
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			wantStatus := "unavailable"
			if test.code == http.StatusOK {
				wantStatus = "ok"
			}
			assertReadiness(t, response, test.code, wantStatus, test.want)
		})
	}
}

func TestReadinessReturnsAtDeadlineWhileCheckIsBlocked(t *testing.T) {
	release, exited := make(chan struct{}), make(chan struct{})
	observed := make(chan context.Context, 1)
	engine := gin.New()
	engine.GET("/readyz", Readiness(func() bool { return true }, map[string]HealthCheck{
		"blocked": func(ctx context.Context) error {
			defer close(exited)
			observed <- ctx
			<-release
			return nil
		},
	}))
	defer close(release)
	response := httptest.NewRecorder()
	returned := make(chan struct{})
	go func() {
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("readiness waited beyond its timeout")
	}
	assertReadiness(
		t,
		response,
		http.StatusServiceUnavailable,
		"unavailable",
		map[string]string{"blocked": "timeout"},
	)
	select {
	case ctx := <-observed:
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("check context error = %v", ctx.Err())
		}
	default:
		t.Fatal("health check did not run")
	}
	select {
	case <-exited:
		t.Fatal("check returned before its release")
	default:
	}
}

func assertReadiness(
	t *testing.T,
	response *httptest.ResponseRecorder,
	code int,
	status string,
	checks map[string]string,
) {
	t.Helper()
	var result struct {
		Status string
		Checks map[string]string
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if response.Code != code || result.Status != status || !reflect.DeepEqual(result.Checks, checks) {
		t.Fatalf("readiness = %d %s, want %d %s %v", response.Code, response.Body, code, status, checks)
	}
}
