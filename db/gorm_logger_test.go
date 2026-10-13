package db_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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
	engine := gin.New()
	engine.Use(func(c *core.Context) {
		c.Request = c.Request.WithContext(logger.WithContext(c.Request.Context(), log))
		c.Next()
	}, middleware.RequestID())
	for _, tc := range []struct {
		path   string
		sql    string
		level  gormlogger.LogLevel
		status int
	}{
		{path: "/first", sql: "SELECT 1", level: gormlogger.Info, status: http.StatusOK},
		{path: "/second", sql: "SELECT 2", level: gormlogger.Info, status: http.StatusOK},
		{path: "/failure", sql: "SELECT * FROM missing_table", level: gormlogger.Error, status: http.StatusInternalServerError},
		{path: "/quiet", sql: "SELECT 3", level: gormlogger.Silent, status: http.StatusOK},
		{path: "/warn", sql: "SELECT 4", level: gormlogger.Warn, status: http.StatusOK},
	} {
		engine.GET(tc.path, func(c *core.Context) {
			tx := database.DB.Session(&gorm.Session{Logger: database.DB.Logger.LogMode(tc.level)})
			err := tx.WithContext(c.Request.Context()).Exec(tc.sql).Error
			if (err != nil) != (tc.status == http.StatusInternalServerError) {
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
	} {
		entry := entries[tc.id]
		message, _ := entry["msg"].(string)
		if entry["level"] != tc.level || !strings.Contains(message, tc.sql) {
			t.Fatalf("%s SQL log = %v", tc.id, entry)
		}
	}
	if len(entries) != 3 {
		t.Fatalf("silent or warn mode emitted successful queries: %v", entries)
	}
}
