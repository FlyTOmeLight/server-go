package greeting

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/constants"
	serrors "github.com/FlyTOmeLight/server-go/errors"
)

func TestFailuresBeforeStreamingCompleteHTTPError(t *testing.T) {
	for _, test := range []struct {
		name, query, code string
		status            int
	}{
		{name: "invalid query", query: "?name=" + strings.Repeat("x", 65), code: "InvalidParameter", status: 400},
		{name: "writer lacks deadline support", code: "InternalError", status: 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			generated := false
			controller := NewController(New(func(context.Context, string, func(string) error) error {
				generated = true
				return nil
			}))
			router := gin.New()
			if err := controller.RegisterRoutes(router.Group("")); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/stream"+test.query, nil)
			request.Header.Set(constants.HeaderRequestID, "request-123")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			var detail serrors.Error
			if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			if response.Code != test.status || detail.Code != test.code ||
				detail.RequestID != "request-123" ||
				generated {
				t.Fatalf("status=%d generated=%v body=%s", response.Code, generated, response.Body)
			}
		})
	}
}
