package db

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestNewDBRejectsNegativePoolSettings(t *testing.T) {
	for _, tc := range []struct {
		field string
		set   func(*Config)
	}{
		{field: "MaxIdleConns", set: func(c *Config) { c.MaxIdleConns = -1 }},
		{field: "MaxOpenConns", set: func(c *Config) { c.MaxOpenConns = -1 }},
		{field: "ConnMaxIdleTime", set: func(c *Config) { c.ConnMaxIdleTime = -1 }},
		{field: "ConnMaxLifeTime", set: func(c *Config) { c.ConnMaxLifeTime = -1 }},
	} {
		t.Run(tc.field, func(t *testing.T) {
			config := &Config{Driver: DriverSQLite, DB: ":memory:"}
			tc.set(config)
			database, err := NewDB(t.Context(), config)
			want := "DBConfig." + tc.field + " must be non-negative"
			if database != nil || err == nil || err.Error() != want {
				t.Fatalf("NewDB returned database=%v, error=%v, want %q", database, err, want)
			}
		})
	}
}

func TestNewDBRetainsDefaultIdleConnections(t *testing.T) {
	database, err := NewDB(t.Context(), &Config{Driver: DriverSQLite, DB: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := connection.PingContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if idle := connection.Stats().Idle; idle == 0 {
		t.Fatal("default pool discarded its idle connection after ping")
	}
}

func TestMySQLDSN(t *testing.T) {
	for _, tc := range []struct {
		name    string
		params  string
		charset string
	}{
		{name: "defaults", charset: "utf8mb4"},
		{name: "custom params", params: "charset=utf8&custom=sample%2Fvalue%25", charset: "utf8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &Config{
				Host: "localhost", Port: 3306, User: "test",
				Password: `p%@/'\ ass`, DB: "db/name?% @", DSNParams: tc.params,
			}
			dsn := mysqlDSN(config)
			parsed, err := mysql.ParseDSN(dsn)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.User != config.User || parsed.Passwd != config.Password ||
				parsed.DBName != config.DB ||
				parsed.Net != "tcp" ||
				parsed.Addr != "localhost:3306" {
				t.Fatalf("MySQL connection fields did not round-trip: %q", dsn)
			}
			if !parsed.ParseTime || !parsed.InterpolateParams || parsed.Loc != time.Local ||
				!parsed.AllowNativePasswords || !parsed.CheckConnLiveness {
				t.Fatalf("unexpected MySQL driver defaults: %q", dsn)
			}
			_, query, ok := strings.Cut(dsn, "?")
			if !ok {
				t.Fatal("DSN has no query")
			}
			values, err := url.ParseQuery(query)
			if err != nil {
				t.Fatal(err)
			}
			if charsets := values["charset"]; len(charsets) != 1 || charsets[0] != tc.charset {
				t.Fatalf("charset = %v, want [%s]", charsets, tc.charset)
			}
			if tc.params != "" && (!strings.HasSuffix(dsn, "&"+tc.params) ||
				parsed.Params["custom"] != "sample/value%") {
				t.Fatalf("custom parameters did not round-trip: %q", dsn)
			}
		})
	}
}

func TestLibpqDSNs(t *testing.T) {
	const params = "application_name='custom%2F value'"
	for _, tc := range []struct {
		name        string
		build       func(*Config) string
		defaultApp  string
		defaultTime bool
	}{
		{name: "PostgreSQL", build: postgresDSN, defaultTime: true},
		{name: "GaussDB", build: gaussdbDSN, defaultApp: "server-go", defaultTime: true},
		{name: "Kingbase", build: kingbaseDSN, defaultApp: "server-go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, custom := range []bool{false, true} {
				config := &Config{
					Host: `test host'\`, Port: 5432, User: `test user'\`, DB: `test db'\`,
					Password: `p%@/'\ ass' sslmode=require`,
				}
				wantApp := tc.defaultApp
				if custom {
					config.DSNParams = params
					wantApp = "custom%2F value"
				}
				dsn := tc.build(config)
				parsed, err := pgconn.ParseConfig(dsn)
				if err != nil {
					t.Fatal(err)
				}
				if parsed.Host != config.Host || parsed.Port != uint16(config.Port) ||
					parsed.User != config.User ||
					parsed.Password != config.Password ||
					parsed.Database != config.DB {
					t.Fatalf("libpq connection fields did not round-trip: %q", dsn)
				}
				if parsed.TLSConfig != nil || len(parsed.Fallbacks) != 0 {
					t.Fatalf("password changed sslmode: %q", dsn)
				}
				if (custom || wantApp != "") && parsed.RuntimeParams["application_name"] != wantApp {
					t.Fatalf(
						"application_name = %q, want %q",
						parsed.RuntimeParams["application_name"],
						wantApp,
					)
				}
				wantTimeZones := 0
				if !custom || tc.defaultTime {
					wantTimeZones = 1
				}
				if got := strings.Count(dsn, "TimeZone=Asia/Shanghai"); got != wantTimeZones {
					t.Fatalf("DSN has %d default time zones, want %d: %q", got, wantTimeZones, dsn)
				}
			}
		})
	}
}

func TestClickHouseDSN(t *testing.T) {
	config := &Config{
		Host: "localhost", Port: 9000, User: "test user+%",
		Password: `p%@/'\ ass&secure=true#`, DB: "db/name?&database=other",
	}
	parsed, err := url.Parse(clickhouseDSN(config))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "tcp" || parsed.Host != "localhost:9000" || parsed.Fragment != "" {
		t.Fatalf("unexpected ClickHouse URL: %q", parsed)
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		t.Fatal(err)
	}
	if len(query) != 3 || query.Get("database") != config.DB || query.Get("username") != config.User ||
		query.Get("password") != config.Password {
		t.Fatalf("ClickHouse parameters did not round-trip: %v", query)
	}
}
