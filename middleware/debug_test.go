package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestDebugBoundsCaptureWithoutTruncatingResponse(t *testing.T) {
	router := gin.New()
	router.Use(Debug(nil, nil))
	payload := strings.Repeat("x", maxLength*3)
	router.GET("/stream", func(c *core.Context) {
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

func TestDebugRedactsHeadersAndSkipsBinaryBodies(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "debug.log")
	log, err := logger.NewLogger(&logger.Config{Outputs: []logger.Output{{Type: "File", Path: logPath}}})
	require.NoError(t, err)
	defer log.Close()
	router := gin.New()
	router.ContextWithFallback = true
	router.Use(Debug(nil, []string{".png"}))
	router.POST("/upload", func(c *core.Context) {
		c.Header("Set-Cookie", "response-secret")
		c.Data(http.StatusOK, "IMAGE/PNG; charset=utf-8", []byte("binary-response"))
	})
	router.GET("/skip.png", func(c *core.Context) {
		c.String(http.StatusOK, "skipped-response")
	})
	for _, test := range []struct {
		method, path string
	}{
		{method: http.MethodPost, path: "/upload"},
		{method: http.MethodGet, path: "/skip.png?v=1"},
	} {
		request := httptest.NewRequest(test.method, test.path, strings.NewReader("binary-request"))
		request = request.WithContext(logger.WithContext(request.Context(), log))
		request.Header.Set("Content-Type", "application/octet-stream")
		for _, name := range []string{"Authorization", "Cookie", "X-Api-Key", "Proxy-Authorization"} {
			request.Header.Set(name, "request-secret")
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
	}
	logged, err := os.ReadFile(logPath)
	require.NoError(t, err)
	require.Equal(t, 5, strings.Count(string(logged), "[REDACTED]"))
	for _, hidden := range []string{"secret", "binary-response", "binary-request", "skipped-response", "skip.png"} {
		require.NotContains(t, string(logged), hidden)
	}
	for _, mime := range []string{"application/pdf", "application/zip", "video/mp4", "audio/wav"} {
		require.True(t, binaryMIME(mime))
	}
}

func TestDebugBoundsNonJSONPreviewAndKeepsRequestBody(t *testing.T) {
	router := gin.New()
	router.Use(Debug(nil, nil))
	payload := strings.Repeat("界", maxLength*3)
	source := &countedBody{Reader: strings.NewReader(payload)}
	router.POST("/upload", func(c *core.Context) {
		require.Zero(t, source.read)
		seen, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Equal(t, payload, string(seen))
		preview := c.Request.Body.(*bodyPreview)
		require.LessOrEqual(t, preview.buffer.Len(), maxLength*utf8.UTFMax)
		trimmed := body(preview.buffer.String())
		require.True(t, utf8.ValidString(trimmed))
		require.Equal(t, maxLength, utf8.RuneCountInString(trimmed))
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/upload", nil)
	request.Header.Set("Content-Type", "text/plain")
	request.Body = source
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)
}
