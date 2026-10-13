package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
		created, err := client.CreateDataset(
			ctx,
			&api.CreateDatasetRequest{ProjectName: "paging", LocalName: fmt.Sprintf("item%02d", i)},
		)
		if err != nil {
			t.Fatal(err)
		}
		if created.FileFormat != api.FileFormatFile {
			t.Fatalf("SDK omission lost the Controller default: %#v", created)
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
	t.Run("controller defaults and tags", func(t *testing.T) {
		response, err := http.Get(server.URL + "/v1/projects/paging/datasets?orderBy=localName&order=asc")
		if err != nil {
			t.Fatal(err)
		}
		var page api.ListDatasetResponse
		err = json.NewDecoder(response.Body).Decode(&page)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || page.TotalCount != 25 || len(page.Results) != 20 {
			t.Fatalf("default page: %#v, %v", page, err)
		}
		response, err = http.Post(
			server.URL+"/v1/projects/paging/datasets",
			"application/json",
			strings.NewReader(`{"localName":"default-file","description":"keep"}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		var created api.Dataset
		err = json.NewDecoder(response.Body).Decode(&created)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || created.FileFormat != api.FileFormatFile {
			t.Fatalf("create default: %#v, %v", created, err)
		}
		if err := client.UpdateDataset(ctx, api.NewUpdateDatasetRequest(created.Name)); err != nil {
			t.Fatal(err)
		}
		got, err := client.GetDataset(ctx, api.NewDatasetName(created.Name))
		if err != nil || got.Description != "keep" {
			t.Fatalf("omitted update changed description: %#v, %v", got, err)
		}
		for _, test := range []struct{ path, body string }{
			{path: "/v1/projects/paging/datasets", body: `{}`},
			{path: "/v1/projects/paging/datasets", body: `{"localName":"bad name"}`},
			{path: "/v1/projects/paging/datasets", body: `{"localName":"new","fileFormat":"unsupported"}`},
			{path: "/v1/projects/paging/datasets", body: `{"localName":"new","fileFormat":""}`},
			{path: "/v1/projects/" + strings.Repeat("p", 65) + "/datasets", body: `{"localName":"new"}`},
		} {
			response, err := http.Post(
				server.URL+test.path,
				"application/json",
				strings.NewReader(test.body),
			)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 400 {
				t.Fatalf("invalid request %s %s = %d", test.path, test.body, response.StatusCode)
			}
		}
	})
}
