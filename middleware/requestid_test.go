package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestRequestIDNormalizesAndPropagates(t *testing.T) {
	for _, test := range []struct {
		name    string
		headers []string
		want    string
	}{
		{name: "missing"},
		{name: "empty", headers: []string{"", "ignored"}},
		{name: "only whitespace", headers: []string{" \t "}},
		{name: "valid punctuation with surrounding whitespace", headers: []string{" \t/first-_.:ID ", "ignored"}, want: "/first-_.:ID"},
		{name: "one byte", headers: []string{"~"}, want: "~"},
		{name: "128 bytes", headers: []string{strings.Repeat("a", 128)}, want: strings.Repeat("a", 128)},
		{name: "129 bytes", headers: []string{strings.Repeat("a", 129)}},
		{name: "embedded space", headers: []string{"bad id"}},
		{name: "control character", headers: []string{"bad\x00id"}},
		{name: "delete character", headers: []string{"bad\x7fid"}},
		{name: "non-ASCII", headers: []string{"请求"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(RequestID())
			engine.GET("/", func(c *core.Context) {
				requestID := c.GetString(constants.KeyRequestID)
				if values := c.Request.Header.Values(constants.HeaderRequestID); len(values) != 1 ||
					values[0] != requestID {
					t.Errorf("request header=%q, context request ID=%q", values, requestID)
				}
				if got := c.Request.Context().Value(constants.KeyRequestID); got != requestID {
					t.Errorf("request context ID=%q, want %q", got, requestID)
				}
				c.String(http.StatusOK, requestID)
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for _, value := range test.headers {
				req.Header.Add(constants.HeaderRequestID, value)
			}
			writer := httptest.NewRecorder()
			engine.ServeHTTP(writer, req)
			id := writer.Header().Get(constants.HeaderRequestID)
			if writer.Code != http.StatusOK || writer.Body.String() != id {
				t.Fatalf("status=%d response header=%q body=%q", writer.Code, id, writer.Body.String())
			}
			if test.want == "" {
				if _, err := uuid.Parse(id); err != nil {
					t.Errorf("request ID %q is not a UUID: %v", id, err)
				}
			} else if id != test.want {
				t.Errorf("request ID=%q, want %q", id, test.want)
			}
		})
	}
}
