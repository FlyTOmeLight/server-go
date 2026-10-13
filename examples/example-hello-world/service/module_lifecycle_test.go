package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/db"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/gin-gonic/gin"
)

func TestModuleRebuiltRoutesReuseBusinessResources(t *testing.T) {
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
	var first []byte
	for index := 0; index < 2; index++ {
		engine := gin.New()
		engine.Use(func(c *core.Context) {
			c.Set(constants.KeyOrgID, "tenant")
			c.Set(constants.KeyUserID, "user")
			c.Next()
		})
		if err := svc.RegisterRoutes(engine.Group("")); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(
			http.MethodPost,
			"/helloworld/internal/v1/CreateModule?clientToken=stable-token",
			bytes.NewBufferString(`{"projectID":"project1","name":"example"}`),
		)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("create returned %d: %s", response.Code, response.Body)
		}
		if index == 0 {
			first = append([]byte(nil), response.Body.Bytes()...)
		} else if !bytes.Equal(first, response.Body.Bytes()) {
			t.Fatalf("route rebuild changed cached operation result: %s vs %s", first, response.Body)
		}
	}
	var count int64
	if err := database.DB.Table("module").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("rebuilt router duplicated operation: %d rows", count)
	}
}

func TestModuleServiceConstructionReportsMigrationFailure(t *testing.T) {
	ctx := context.Background()
	database := &db.Module{
		Config: &db.Config{Driver: db.DriverSQLite, DB: ":memory:", MaxIdleConns: 1, MaxOpenConns: 1},
	}
	if err := database.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := NewHelloWorldService(cancelled, database.DB, true); err == nil {
		t.Fatal("cancelled migration succeeded")
	}
	if _, err := NewHelloWorldService(ctx, nil, false); err == nil {
		t.Fatal("missing database accepted")
	}
}
