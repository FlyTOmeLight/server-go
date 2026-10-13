package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecoveryKeepsStreamIntact(t *testing.T) {
	engine := gin.New()
	engine.Use(Recovery())
	engine.GET("/panic", func(c *gin.Context) { panic("secret panic value") })
	engine.GET("/stream", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Writer.WriteString("data: started\n\n")
		c.Writer.Flush()
		panic("secret panic value")
	})
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/panic", nil))
	if w.Code != 500 || !strings.Contains(w.Body.String(), `"code":"InternalError"`) ||
		strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/stream", nil))
	if w.Body.String() != "data: started\n\n" {
		t.Fatalf("corrupted stream: %s", w.Body.String())
	}
}
