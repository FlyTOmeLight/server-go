package binder

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/errors"
)

func TestBoundedStrictJSON(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		code       string
	}{
		{name: "valid", body: `{"name":"ok"}`, status: 200, code: ""},
		{name: "unknown", body: `{"name":"ok","extra":true}`, status: 400, code: "InvalidParameter"},
		{name: "trailing", body: `{"name":"ok"} {}`, status: 400, code: "InvalidParameter"},
		{name: "malformed", body: `{"name":`, status: 400, code: "InvalidParameter"},
		{name: "validation", body: `{"name":""}`, status: 400, code: "InvalidParameter"},
		{name: "oversized", body: `{"name":"` + strings.Repeat("a", MaxJSONBytes) + `"}`, status: 413, code: "PayloadTooLarge"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			engine.POST("/", func(c *gin.Context) {
				var req struct {
					Name string `json:"name" binding:"required"`
				}
				err := Bind(c, &req).JSON().Error()
				if err != nil {
					errors.AbortWithError(c, err)
					return
				}
				c.Status(200)
			})
			w := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			request.ContentLength = -1 // Exercise chunked bodies as well as fixed-length input.
			engine.ServeHTTP(w, request)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tc.code != "" && !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
				t.Fatalf("body=%s", w.Body.String())
			}
		})
	}
}

func TestQueryLimit(t *testing.T) {
	for _, tc := range []struct {
		query         string
		value, status int
	}{
		{query: "", value: 50, status: 200},
		{query: "?limit=1", value: 1, status: 200},
		{query: "?limit=100", value: 100, status: 200},
		{query: "?limit=0", value: 0, status: 400},
		{query: "?limit=101", value: 0, status: 400},
		{query: "?limit=bad", value: 0, status: 400},
	} {
		engine := gin.New()
		engine.GET("/", func(c *gin.Context) {
			value, ok := QueryLimit(c, 50, 100)
			if ok && value != tc.value {
				t.Errorf("value=%d", value)
			}
		})
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/"+tc.query, nil))
		if w.Code != tc.status {
			t.Fatalf("query=%s status=%d", tc.query, w.Code)
		}
	}
}
