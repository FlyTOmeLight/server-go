package db

import (
	"testing"

	"gorm.io/driver/postgres"
)

func TestPostgreSQLDSN(t *testing.T) {
	for _, dsn := range []string{
		"postgres://user:p%40ss@localhost:5432/test?sslmode=verify-full&search_path=tenant&TimeZone=UTC",
		"host=localhost port=5432 user=user password='p ass' dbname=test sslmode=require search_path=tenant TimeZone=UTC",
	} {
		config := &Config{Driver: DriverPostgreSQL, DSN: dsn}
		if err := config.check(); err != nil {
			t.Fatal(err)
		}
		database, err := newPostgreSQL(config)
		if err != nil {
			t.Fatal(err)
		}
		dialect := database.Dialector.(*AIBaseDialector).Dialector.(*postgres.Dialector)
		if dialect.DSN != dsn {
			t.Fatalf("DSN changed: %q", dialect.DSN)
		}
		if !dialect.PreferSimpleProtocol {
			t.Fatal("simple protocol disabled")
		}
		connection, err := database.DB()
		if err != nil {
			t.Fatal(err)
		}
		if err := connection.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
