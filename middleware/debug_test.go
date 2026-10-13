package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestDebugBoundsCaptureWithoutTruncatingResponse(t *testing.T) {
	router := gin.New()
	router.Use(Debug(nil, nil))
	payload := strings.Repeat("x", maxLength*3)
	router.GET("/stream", func(c *gin.Context) {
		_, _ = c.Writer.Write([]byte(payload[:maxLength]))
		_, _ = c.Writer.WriteString(payload[maxLength:])
		writer := c.Writer.(core.BufferedWriter)
		if writer.Buf.Len() != maxLength {
			t.Fatalf("captured %d bytes", writer.Buf.Len())
		}
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/stream", nil))
	if response.Body.String() != payload {
		t.Fatal("response was truncated")
	}
}
