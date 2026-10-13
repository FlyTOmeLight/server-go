package service

import (
	"context"
	"errors"
	"testing"

	"github.com/FlyTOmeLight/server-go/db"
)

func TestServiceConstructionRejectsMissingDatabase(t *testing.T) {
	for _, autoMigrate := range []bool{false, true} {
		service, err := NewExampleService(t.Context(), nil, autoMigrate)
		if err == nil || service != nil {
			t.Fatalf("autoMigrate=%v: service=%v error=%v", autoMigrate, service, err)
		}
	}
}

func TestMigrationFailurePreservesBorrowedDatabase(t *testing.T) {
	database := &db.Module{
		Config: &db.Config{Driver: db.DriverSQLite, DB: ":memory:", MaxOpenConns: 1, MaxIdleConns: 1},
	}
	if err := database.Initialize(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	service, err := NewExampleService(ctx, database.DB, true)
	if service != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled migration: service=%v error=%v", service, err)
	}
	pool, err := database.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.PingContext(t.Context()); err != nil {
		t.Fatalf("construction failure closed its borrowed database: %v", err)
	}
}
