package note

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/FlyTOmeLight/server-go/conf"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/examples/postgresql-service/internal/table"
)

func TestInvalidRequestsStopBeforeStorage(t *testing.T) {
	router := gin.New()
	router.Use(serrors.ErrorHandler(&conf.Config{}))
	NewController(New(table.New(nil))).RegisterRoutes(router.Group(""))
	for _, test := range []struct{ name, method, path, body string }{
		{name: "missing name", method: http.MethodPost, path: "/v1/notes", body: `{}`},
		{name: "long name", method: http.MethodPost, path: "/v1/notes", body: `{"name":"` + strings.Repeat("x", 65) + `"}`},
		{name: "long description", method: http.MethodPost, path: "/v1/notes", body: `{"name":"a","description":"` + strings.Repeat("x", 1025) + `"}`},
		{name: "priority range", method: http.MethodPost, path: "/v1/notes", body: `{"name":"a","priority":11}`},
		{name: "null priority", method: http.MethodPost, path: "/v1/notes", body: `{"name":"a","priority":null}`},
		{name: "null enabled", method: http.MethodPost, path: "/v1/notes", body: `{"name":"a","enabled":null}`},
		{name: "unknown field", method: http.MethodPost, path: "/v1/notes", body: `{"name":"a","secret":true}`},
		{name: "invalid UUID", method: http.MethodGet, path: "/v1/notes/invalid", body: ""},
		{name: "invalid update UUID", method: http.MethodPut, path: "/v1/notes/invalid", body: `{"name":"a"}`},
		{name: "missing replacement name", method: http.MethodPut, path: "/v1/notes/" + uuid.NewString(), body: `{}`},
		{name: "invalid delete UUID", method: http.MethodDelete, path: "/v1/notes/invalid", body: ""},
		{name: "page number", method: http.MethodGet, path: "/v1/notes?pageNo=0", body: ""},
		{name: "page size", method: http.MethodGet, path: "/v1/notes?pageSize=0", body: ""},
		{name: "page size maximum", method: http.MethodGet, path: "/v1/notes?pageSize=101", body: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
