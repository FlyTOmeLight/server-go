package module

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FlyTOmeLight/server-go/constants"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/gin-gonic/gin"
)

func TestInvalidRequestsCompleteWithoutErrorMiddleware(t *testing.T) {
	controller := NewModuleController(nil)
	for _, test := range []struct {
		path    string
		handler core.HandlerFunc
	}{
		{path: "/v1/CreateModule", handler: controller.createModule},
		{path: "/v1/ListModule", handler: controller.listModule},
		{path: "/v1/GetModule", handler: controller.getModule},
		{path: "/v1/UpdateModule", handler: controller.updateModule},
		{path: "/v1/DeleteModule", handler: controller.deleteModule},
	} {
		t.Run(test.path, func(t *testing.T) {
			router := gin.New()
			continued := false
			router.POST(test.path, test.handler, func(*core.Context) { continued = true })
			request := httptest.NewRequest(
				http.MethodPost,
				test.path,
				strings.NewReader(`{"unknown":true}`),
			)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(constants.HeaderRequestID, "request-123")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			var detail serrors.Error
			if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusBadRequest || detail.Code != "InvalidParameter" ||
				detail.RequestID != "request-123" ||
				continued {
				t.Fatalf("status=%d continued=%v body=%s", response.Code, continued, response.Body)
			}
		})
	}
}
