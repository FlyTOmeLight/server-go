package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FlyTOmeLight/server-go/auth"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/db"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	hello "github.com/FlyTOmeLight/server-go/examples/example-hello-world/service"
	service1 "github.com/FlyTOmeLight/server-go/examples/example-service1/service"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/gin-gonic/gin"
)

func TestPublicServicesShareDatabaseAndRouter(t *testing.T) {
	ctx := context.Background()
	database := &db.Module{
		Config: &db.Config{Driver: db.DriverSQLite, DB: ":memory:", MaxIdleConns: 1, MaxOpenConns: 1},
	}
	if err := database.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	svc1, err := service1.NewExampleService(ctx, database.DB, true)
	if err != nil {
		t.Fatal(err)
	}
	defer svc1.Stop(ctx)
	svcHello, err := hello.NewHelloWorldService(ctx, database.DB, true)
	if err != nil {
		t.Fatal(err)
	}
	defer svcHello.Stop(ctx)
	engine := gin.New()
	binder.RegisterValidator()
	engine.Use(serrors.ErrorHandler(&conf.Config{}), auth.FakeAuth())
	for _, svc := range []host.Service{svc1, svcHello} {
		if err := svc.RegisterRoutes(engine.Group("")); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		path string
		body string
	}{
		{path: "/helloworld/internal/v1/CreateModule", body: `{"projectID":"project1","name":"hello"}`},
		{path: "/v1/projects/project1/datasets", body: `{"localName":"dataset1"}`},
	} {
		request := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", tc.path, response.Code, response.Body)
		}
	}
	for _, table := range []string{"module", "dataset"} {
		var count int64
		if err := database.DB.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s has %d rows, want 1", table, count)
		}
	}
}
