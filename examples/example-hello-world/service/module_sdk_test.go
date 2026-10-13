package service

import (
	"context"
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

	api "github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

func TestModuleSDKRoundTrip(t *testing.T) {
	ctx := context.Background()
	database := &db.Module{
		Config: &db.Config{Driver: db.DriverSQLite, DB: ":memory:", MaxIdleConns: 1, MaxOpenConns: 1},
	}
	if err := database.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	svc, err := NewHelloWorldService(ctx, database.DB, true)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Stop(ctx)
	router := gin.New()
	binder.RegisterValidator()
	router.Use(serrors.ErrorHandler(&conf.Config{}), auth.FakeAuth())
	if err := svc.RegisterRoutes(router.Group("")); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	client, err := api.NewClientWithConfig(&sdk.Config{Endpoint: server.URL + "/helloworld/internal", Timeout: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for _, body := range []string{`{"projectID":"project1","pageSize":-1}`, `{"projectID":"project1","pageNo":-1}`} {
		response, err := http.Post(
			server.URL+"/helloworld/internal/v1/ListModule",
			"application/json",
			strings.NewReader(body),
		)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("negative pagination accepted: %d", response.StatusCode)
		}
	}
	created, err := client.CreateModule(ctx, &api.CreateModuleRequest{ProjectID: "project1", Name: "example"})
	if err != nil {
		t.Fatal(err)
	}
	listed, err := client.ListModule(
		ctx,
		&api.ListModuleRequest{
			ProjectID:     "project1",
			PagingRequest: sdk.PagingRequest{PageNo: 1, PageSize: 10},
		},
	)
	if err != nil || listed.TotalCount != 1 || len(listed.Results) != 1 || listed.Results[0].ID != created.ID {
		t.Fatalf("list: %#v, %v", listed, err)
	}
	description := "updated"
	if err := client.UpdateModule(
		ctx,
		&api.UpdateModuleRequest{ModuleID: api.ModuleID{ID: created.ID}, Description: &description},
	); err != nil {
		t.Fatal(err)
	}
	got, err := client.GetModule(ctx, &api.GetModuleRequest{ID: created.ID})
	if err != nil || got.Description != description {
		t.Fatalf("get: %#v, %v", got, err)
	}
	if err := client.DeleteModule(ctx, &api.DeleteModuleRequest{ID: created.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetModule(ctx, &api.GetModuleRequest{ID: created.ID}); err == nil {
		t.Fatal("deleted record was returned")
	}
}
