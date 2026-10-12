package logger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileLogging(t *testing.T) {
	for _, rotation := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "rotating"}[rotation], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nested", "app.log")
			l, err := NewLogger(&Config{Level: "Info", Outputs: []Output{{Type: "File", Level: "Warn", Path: path, Rotation: rotation}}})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.WithValue(context.Background(), "RequestID", "request-123")
			l.Infof(ctx, "filtered")
			l.Warnf(ctx, "item %d", 7)
			l.Close()
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var entry map[string]any
			if err := json.Unmarshal(data, &entry); err != nil {
				t.Fatal(err)
			}
			if entry["msg"] != "item 7" || entry["requestId"] != "request-123" {
				t.Fatalf("unexpected log: %s", data)
			}
		})
	}
}

func TestInvalidConfig(t *testing.T) {
	for _, config := range []*Config{nil, {Level: "invalid"}, {Outputs: []Output{{Type: "File"}}}, {Outputs: []Output{{Type: "unknown"}}}} {
		if _, err := NewLogger(config); err == nil {
			t.Fatalf("accepted invalid config: %#v", config)
		}
	}
}

func TestParseConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logger.toml")
	if err := os.WriteFile(path, []byte("Level = 'Warn'\n[[Outputs]]\ntype = 'Console'\nlevel = 'Error'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := ParseConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Level != "Warn" || len(config.Outputs) != 1 || !strings.EqualFold(config.Outputs[0].Type, "Console") {
		t.Fatalf("unexpected config: %#v", config)
	}
}
