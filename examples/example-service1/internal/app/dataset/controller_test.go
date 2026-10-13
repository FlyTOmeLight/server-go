package dataset

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FlyTOmeLight/server-go/constants"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/gin-gonic/gin"
)

func TestInvalidRequestsCompleteWithoutErrorMiddleware(t *testing.T) {
	binder.RegisterValidator()
	controller := NewController(nil)
	for _, test := range []struct {
		method, path, target, body string
		handler                    core.HandlerFunc
	}{
		{method: http.MethodPost, path: "/v1/projects/:projectName/datasets", target: "/v1/projects/project1/datasets", body: `{}`, handler: controller.createDataset},
		{method: http.MethodGet, path: "/v1/projects/:projectName/datasets", target: "/v1/projects/project1/datasets?pageSize=-1", handler: controller.listDataset},
		{method: http.MethodGet, path: "/v1/projects/:projectName/datasets/:localName", target: "/v1/projects/project1/datasets/_", handler: controller.getDataset},
		{method: http.MethodPut, path: "/v1/projects/:projectName/datasets/:localName", target: "/v1/projects/project1/datasets/_", body: `{}`, handler: controller.updateDataset},
		{method: http.MethodDelete, path: "/v1/projects/:projectName/datasets/:localName", target: "/v1/projects/project1/datasets/_", handler: controller.deleteDataset},
	} {
		t.Run(test.method+test.target, func(t *testing.T) {
			router := gin.New()
			continued := false
			router.Handle(test.method, test.path, test.handler, func(*core.Context) { continued = true })
			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
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
