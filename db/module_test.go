package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"gorm.io/gorm"
)

type closeErrorConnector struct {
	driver.Connector
	err error
}

func (c closeErrorConnector) Close() error { return c.err }

func TestModuleRetainsDBOnCloseError(t *testing.T) {
	closeErr := errors.New("driver close failed")
	pool := sql.OpenDB(closeErrorConnector{err: closeErr})
	t.Cleanup(func() { _ = pool.Close() })
	database := &DB{Config: &gorm.Config{ConnPool: pool}}
	module := &Module{DB: database}

	if err := module.Close(t.Context()); !errors.Is(err, closeErr) {
		t.Fatalf("Close returned %v, want %v", err, closeErr)
	}
	if module.DB != database {
		t.Fatal("failed close discarded the database")
	}
	// database/sql treats the second Close on the same pool as a no-op.
	if err := module.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if module.DB != nil {
		t.Fatal("successful retry retained the database")
	}
}

func TestModuleOwnsIndependentDatabases(t *testing.T) {
	ctx := context.Background()
	first := &Module{Config: &Config{Driver: DriverSQLite, DB: ":memory:"}}
	second := &Module{Config: &Config{Driver: DriverSQLite, DB: ":memory:"}}
	if err := first.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if err := second.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close(ctx); _ = second.Close(ctx) })
	if err := first.DB.Exec("CREATE TABLE example (id INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	if second.DB.Migrator().HasTable("example") {
		t.Fatal("database state leaked to another module")
	}
	connection, err := first.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := connection.PingContext(ctx); err == nil {
		t.Fatal("SQL connection remains open")
	}
	if err := first.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if first.DB != nil {
		t.Fatal("closed module exposes a database")
	}
}

func TestModuleRejectsMissingConfigAndCancelledInitialization(t *testing.T) {
	if err := (&Module{}).Initialize(context.Background()); err == nil {
		t.Fatal("missing configuration accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	module := &Module{Config: &Config{Driver: DriverSQLite, DB: ":memory:"}}
	if err := module.Initialize(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled initialization: %v", err)
	}
	if module.DB != nil {
		t.Fatal("failed initialization published a database")
	}
	if err := module.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
