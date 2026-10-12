package logger

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestContextLoggersAreIsolatedAndPreserveRequestID(t *testing.T) {
	paths := []string{filepath.Join(t.TempDir(), "first.log"), filepath.Join(t.TempDir(), "second.log")}
	loggers := make([]*Logger, len(paths))
	for i, path := range paths {
		l, err := NewLogger(&Config{Outputs: []Output{{Type: "File", Path: path}}})
		if err != nil {
			t.Fatal(err)
		}
		loggers[i] = l
		defer l.Close()
	}
	engine := gin.New()
	engine.ContextWithFallback = true
	engine.GET("/log", func(c *gin.Context) {
		c.Set("RequestID", "request-123")
		FromContext(c).Warnf(c, "explicit %s", c.GetHeader("Instance"))
		Infof(c, "package %s", c.GetHeader("Instance"))
		c.Status(http.StatusNoContent)
	})
	for i, l := range loggers {
		name := []string{"first", "second"}[i]
		req := httptest.NewRequest(http.MethodGet, "/log", nil)
		req.Header.Set("Instance", name)
		req = req.WithContext(WithContext(req.Context(), l))
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatal(w.Code)
		}
		if err := l.Sync(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(paths[i])
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "explicit "+name) ||
			!strings.Contains(string(data), "package "+name) ||
			!strings.Contains(string(data), `"requestId":"request-123"`) {
			t.Fatalf("context logger or request ID was lost: %s", data)
		}
		other := []string{"second", "first"}[i]
		if strings.Contains(string(data), "explicit "+other) ||
			strings.Contains(string(data), "package "+other) {
			t.Fatalf("loggers shared outputs: %s", data)
		}
	}
}

func TestFromContextWithoutLoggerUsesConsole(t *testing.T) {
	if FromContext(nil) != FromContext(context.Background()) {
		t.Fatal("console fallback was not stable")
	}
	if FromContext(WithContext(context.Background(), nil)) != FromContext(nil) {
		t.Fatal("nil contextual logger did not use console")
	}
}
