package sdk_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/conf"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/FlyTOmeLight/server-go/sdk"
)

func TestPagingHTTPBinding(t *testing.T) {
	binder.RegisterValidator()
	for _, test := range []struct {
		name, query, body string
		want              sdk.PagingRequest
		status            int
	}{
		{name: "query", query: "?pageNo=2&pageSize=5&order=asc&orderBy=name", want: sdk.PagingRequest{PageNo: 2, PageSize: 5, Order: "asc", OrderBy: "name"}, status: 200},
		{name: "default", status: 200},
		{name: "negative query page", query: "?pageNo=-1", status: 400},
		{name: "negative query size", query: "?pageSize=-1", status: 400},
		{name: "negative JSON page", body: `{"pageNo":-1}`, status: 400},
		{name: "negative JSON size", body: `{"pageSize":-1}`, status: 400},
		{name: "oversized", query: "?pageSize=201", status: 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(serrors.ErrorHandler(&conf.Config{}))
			engine.Any("/", func(c *gin.Context) {
				var request sdk.PagingRequest
				if err := binder.Bind(c, &request).All().Error(); err != nil {
					_ = c.Error(err)
					return
				}
				if request.PageNo != test.want.PageNo || request.PageSize != test.want.PageSize ||
					request.Order != test.want.Order ||
					request.OrderBy != test.want.OrderBy {
					t.Fatalf("bound request = %#v, want %#v", request, test.want)
				}
				c.Status(200)
			})
			request := httptest.NewRequest(http.MethodGet, "/"+test.query, nil)
			if test.body != "" {
				request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
				request.Header.Set("Content-Type", "application/json")
			}
			writer := httptest.NewRecorder()
			engine.ServeHTTP(writer, request)
			if writer.Code != test.status {
				t.Fatalf("status = %d, want %d: %s", writer.Code, test.status, writer.Body)
			}
		})
	}
}
