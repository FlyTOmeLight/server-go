package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestRecoveryKeepsStreamIntact(t *testing.T) {
	log, path := newHTTPTestLogger(t)
	engine := gin.New()
	engine.Use(Recovery())
	engine.GET("/panic", func(c *core.Context) { panic("secret panic value") })
	engine.GET("/stream", func(c *core.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Writer.WriteString("data: started\n\n")
		c.Writer.Flush()
		panic("secret panic value")
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(w, req.WithContext(logger.WithContext(req.Context(), log)))
	if w.Code != 500 || !strings.Contains(w.Body.String(), `"code":"InternalError"`) ||
		strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/stream", nil))
	if w.Body.String() != "data: started\n\n" {
		t.Fatalf("corrupted stream: %s", w.Body.String())
	}
	logs, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logs), "route=/panic panic=secret panic value stack=") {
		t.Errorf("missing recovered panic in log: %s", logs)
	}
}

func TestRequestLogRecordsRouteWithoutQuery(t *testing.T) {
	log, path := newHTTPTestLogger(t)
	engine := gin.New()
	engine.Use(RequestLog())
	engine.GET("/items/:id", func(c *core.Context) { c.Status(http.StatusNoContent) })
	for _, target := range []string{"/items/123?credential=secret", "/unmatched?credential=secret"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		engine.ServeHTTP(httptest.NewRecorder(), req.WithContext(logger.WithContext(req.Context(), log)))
	}
	logs, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	output := string(logs)
	if !strings.Contains(output, "route=/items/:id status=204") ||
		!strings.Contains(output, "route=/unmatched status=404") || strings.Contains(output, "credential") {
		t.Errorf("unexpected request log: %s", output)
	}
}

func newHTTPTestLogger(t *testing.T) (*logger.Logger, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "http.log")
	log, err := logger.NewLogger(&logger.Config{Outputs: []logger.Output{{Type: "file", Path: path}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(log.Close)
	return log, path
}
