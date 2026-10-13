package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FlyTOmeLight/server-go/auth"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/db"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/FlyTOmeLight/server-go/sdk"
	"github.com/gin-gonic/gin"

	api "github.com/FlyTOmeLight/server-go/examples/example-service1/sdk/go/trainv1"
)

func TestDatasetSDKPagination(t *testing.T) {
	ctx := t.Context()
	database, err := db.NewDB(
		ctx,
		&db.Config{Driver: db.DriverSQLite, DB: ":memory:", MaxIdleConns: 1, MaxOpenConns: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	svc, err := NewExampleService(ctx, database, true)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Stop(ctx)
	engine := gin.New()
	binder.RegisterValidator()
	engine.Use(serrors.ErrorHandler(&conf.Config{}), auth.FakeAuth())
	if err := svc.RegisterRoutes(engine.Group("")); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(engine)
	defer server.Close()
	client, err := api.NewClientWithConfig(&sdk.Config{Endpoint: server.URL, Timeout: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for i := range 25 {
		if _, err := client.CreateDataset(
			ctx,
			&api.CreateDatasetRequest{ProjectName: "paging", LocalName: fmt.Sprintf("item%02d", i)},
		); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		order string
		first int
		step  int
	}{
		{order: sdk.OrderASC, first: 5, step: 1},
		{order: sdk.OrderDESC, first: 19, step: -1},
	} {
		t.Run(test.order, func(t *testing.T) {
			result, err := client.ListDataset(
				ctx,
				&api.ListDatasetRequest{
					ProjectName: "paging",
					PagingRequest: sdk.PagingRequest{
						PageNo:   2,
						PageSize: 5,
						Order:    test.order,
						OrderBy:  "localName",
					},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.TotalCount != 25 || len(result.Results) != 5 {
				t.Fatalf("pagination count: %#v", result)
			}
			for i, item := range result.Results {
				if want := fmt.Sprintf("item%02d", test.first+i*test.step); item.LocalName != want {
					t.Fatalf("result %d = %s, want %s", i, item.LocalName, want)
				}
			}
		})
	}
	for _, query := range []string{"pageSize=-1", "pageNo=-1"} {
		response, err := http.Get(server.URL + "/v1/projects/paging/datasets?" + query)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("negative pagination %s = %d", query, response.StatusCode)
		}
	}
}
