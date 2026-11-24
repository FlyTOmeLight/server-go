package db_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/FlyTOmeLight/server-go/middleware"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestSQLLogsUseRequestContextAndRespectLogMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sql.log")
	log, err := logger.NewLogger(&logger.Config{Outputs: []logger.Output{{Type: "file", Path: path}}})
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	database := &db.Module{Config: &db.Config{Driver: db.DriverSQLite, DB: ":memory:", MaxOpenConns: 1}}
	if err := database.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer database.Close(context.Background())
	if err := database.DB.Exec("CREATE TABLE empty_records (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	engine.Use(func(c *core.Context) {
		c.Request = c.Request.WithContext(logger.WithContext(c.Request.Context(), log))
		c.Next()
	}, middleware.RequestID())
	for _, tc := range []struct {
		path     string
		sql      string
		level    gormlogger.LogLevel
		status   int
		notFound bool
	}{
		{path: "/first", sql: "SELECT 1", level: gormlogger.Info, status: http.StatusOK},
		{path: "/second", sql: "SELECT 2", level: gormlogger.Info, status: http.StatusOK},
		{path: "/failure", sql: "SELECT * FROM missing_table", level: gormlogger.Error, status: http.StatusInternalServerError},
		{path: "/not-found", level: gormlogger.Info, status: http.StatusNotFound, notFound: true},
		{path: "/not-found-error-mode", level: gormlogger.Error, status: http.StatusNotFound, notFound: true},
		{path: "/quiet", sql: "SELECT 3", level: gormlogger.Silent, status: http.StatusOK},
		{path: "/warn", sql: "SELECT 4", level: gormlogger.Warn, status: http.StatusOK},
	} {
		engine.GET(tc.path, func(c *core.Context) {
			tx := database.DB.WithContext(c.Request.Context()).Session(&gorm.Session{
				Logger: database.DB.Logger.LogMode(tc.level),
			})
			if tc.notFound {
				var record struct{ ID int }
				if err := tx.Table("empty_records").
					First(&record).
					Error; !errors.Is(
					err,
					gorm.ErrRecordNotFound,
				) {
					t.Errorf("query %s: got %v, want record not found", tc.path, err)
				}
			} else if err := tx.Exec(tc.sql).Error; (err != nil) != (tc.status == http.StatusInternalServerError) {
				t.Errorf("query %s: %v", tc.sql, err)
			}
			c.Status(tc.status)
		})
		request := httptest.NewRequest(http.MethodGet, tc.path, nil)
		request.Header.Set(constants.HeaderRequestID, tc.path)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("%s returned %d", tc.path, response.Code)
		}
	}
	if err := log.Sync(); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	entries := make(map[string]map[string]any)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		id, ok := entry["requestId"].(string)
		if !ok || id == "" {
			t.Fatalf("SQL log lost request ID: %v", entry)
		}
		entries[id] = entry
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id    string
		sql   string
		level string
	}{
		{id: "/first", sql: "SELECT 1", level: "info"},
		{id: "/second", sql: "SELECT 2", level: "info"},
		{id: "/failure", sql: "missing_table", level: "error"},
		{id: "/not-found", sql: "empty_records", level: "info"},
	} {
		entry := entries[tc.id]
		message, _ := entry["msg"].(string)
		if entry["level"] != tc.level || !strings.Contains(message, tc.sql) {
			t.Fatalf("%s SQL log = %v", tc.id, entry)
		}
	}
	if len(entries) != 4 {
		t.Fatalf("unexpected SQL log entries: %v", entries)
	}
}

func TestSQLLogsOmitBoundValues(t *testing.T) {
	const secret = "super-secret-value"
	path := filepath.Join(t.TempDir(), "sql.log")
	log, err := logger.NewLogger(&logger.Config{Outputs: []logger.Output{{Type: "file", Path: path}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(log.Close)
	database := &db.Module{Config: &db.Config{Driver: db.DriverSQLite, DB: ":memory:", MaxOpenConns: 1}}
	if err := database.Initialize(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	for _, sql := range []string{
		"CREATE TABLE records (secret TEXT)",
		"CREATE TRIGGER reject_secret BEFORE INSERT ON records BEGIN SELECT RAISE(ABORT, NEW.secret); END",
	} {
		if err := database.DB.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}

	tx := database.DB.WithContext(logger.WithContext(t.Context(), log)).Debug()
	var count int64
	if err := tx.Table("records").Where("secret = ?", secret).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(
		"INSERT INTO records (secret) VALUES (?)",
		secret,
	).Error; err == nil ||
		err.Error() != secret {
		t.Fatalf("expected driver error to retain its original value, got %v", err)
	}
	wrapped, err := gorm.Open(db.NewAIBaseDialector("postgres", postgres.Open("host=localhost")), &gorm.Config{
		Logger: database.DB.Logger, DryRun: true, DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := wrapped.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := wrapped.WithContext(logger.WithContext(t.Context(), log)).
		Table("records").Where("secret = ?", secret).Count(&count).Error; err != nil {
		t.Fatal(err)
	}

	if err := log.Sync(); err != nil {
		t.Fatal(err)
	}
	logged, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	message := string(logged)
	if !strings.Contains(message, "secret = ?") || !strings.Contains(message, "secret = $1") ||
		!strings.Contains(message, "VALUES (?)") ||
		!strings.Contains(message, `"level":"error"`) || strings.Contains(message, secret) {
		t.Fatalf("unexpected SQL logs: %s", message)
	}
}
