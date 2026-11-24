package logger

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileLogging(t *testing.T) {
	for _, rotation := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "rotating"}[rotation], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nested", "app.log")
			l, err := NewLogger(
				&Config{
					Level: "Info",
					Outputs: []Output{
						{Type: "File", Level: "Warn", Path: path, Rotation: rotation},
					},
				},
			)
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
	for _, config := range []*Config{
		nil,
		{Level: "invalid"},
		{AddStacktrace: "invalid"},
		{Outputs: []Output{{Type: "File"}}},
		{Outputs: []Output{{Type: "unknown"}}},
		{Outputs: []Output{{Type: "console", Format: "invalid"}}},
	} {
		if _, err := NewLogger(config); err == nil {
			t.Fatalf("accepted invalid config: %#v", config)
		}
	}
}

func TestParseConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logger.toml")
	if err := os.WriteFile(
		path,
		[]byte("Level = 'Warn'\n[[Outputs]]\ntype = 'Console'\nlevel = 'Error'\n"),
		0o600,
	); err != nil {
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

type logStringer func() string

func (f logStringer) String() string { return f() }

func TestDisabledLevelDoesNotFormatArguments(t *testing.T) {
	l, err := NewLogger(&Config{
		Level:   "warn",
		Outputs: []Output{{Type: "file", Path: filepath.Join(t.TempDir(), "app.log")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(l.Close)
	calls := 0
	argument := logStringer(func() string {
		calls++
		return "formatted"
	})
	l.Infof(context.Background(), "%s", argument)
	if calls != 0 {
		t.Fatal("disabled level formatted the argument")
	}
	l.Warnf(context.Background(), "%s", argument)
	if calls != 1 {
		t.Fatalf("enabled level formatted the argument %d times", calls)
	}
}

type countingCloser struct{ closes int }

func (c *countingCloser) Close() error { c.closes++; return nil }

func TestCloseIsIdempotent(t *testing.T) {
	file := &countingCloser{}
	l := &Logger{files: []io.Closer{file}}
	l.Close()
	l.Close()
	if file.closes != 1 {
		t.Fatalf("closed log file %d times", file.closes)
	}
}

func TestNilAndZeroValueLogger(t *testing.T) {
	for _, l := range []*Logger{nil, {}} {
		if err := l.Sync(); err != nil {
			t.Fatal(err)
		}
		l.Close()
		l.Infof(context.Background(), "discarded")
	}
}

func TestRuntimeLevelPreservesOutputThresholds(t *testing.T) {
	directory := t.TempDir()
	inheritedPath := filepath.Join(directory, "inherited.log")
	explicitPath := filepath.Join(directory, "explicit.log")
	log, err := NewLogger(&Config{
		Level: "info",
		Outputs: []Output{
			{Type: "file", Path: inheritedPath},
			{Type: "file", Path: explicitPath, Level: "warn"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(log.Close)
	ctx := context.Background()
	log.Debugf(ctx, "initial debug")
	if err := log.SetLevel("Debug"); err != nil {
		t.Fatal(err)
	}
	if err := log.SetLevel("invalid"); err == nil || log.Level() != "debug" {
		t.Fatalf("invalid level changed logger to %q", log.Level())
	}
	log.Debugf(ctx, "enabled debug")
	log.Warnf(ctx, "enabled warning")
	if err := log.SetLevel("error"); err != nil {
		t.Fatal(err)
	}
	log.Warnf(ctx, "disabled warning")
	log.Errorf(ctx, "enabled error")
	log.Close()
	for _, test := range []struct {
		path string
		want []string
	}{
		{path: inheritedPath, want: []string{"enabled debug", "enabled warning", "enabled error"}},
		{path: explicitPath, want: []string{"enabled warning", "enabled error"}},
	} {
		entries := readLogEntries(t, test.path)
		if len(entries) != len(test.want) {
			t.Fatalf("unexpected entries in %s: %v", test.path, entries)
		}
		for index, message := range test.want {
			if entries[index]["msg"] != message {
				t.Fatalf("entry %d in %s = %v, want %q", index, test.path, entries[index], message)
			}
		}
	}
}

func TestLogEncodersAndDiagnostics(t *testing.T) {
	stdout, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdout.Close() })
	stderr, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stderr.Close() })
	originalStdout, originalStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdout, stderr
	log, err := NewLogger(&Config{
		AddCaller:     true,
		AddStacktrace: "error",
		Outputs: []Output{
			{Type: "stdout", Format: "json"},
			{Type: "stderr", Format: "console"},
		},
	})
	os.Stdout, os.Stderr = originalStdout, originalStderr
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(log.Close)
	ctx := WithContext(context.Background(), log)
	Infof(ctx, "plain message")
	log.Errorf(ctx, "error with stack")
	log.Close()

	entries := readLogEntries(t, stdout.Name())
	if len(entries) != 2 || entries[0]["msg"] != "plain message" || entries[1]["msg"] != "error with stack" {
		t.Fatalf("unexpected stdout entries: %v", entries)
	}
	for _, entry := range entries {
		caller, _ := entry["caller"].(string)
		if !strings.Contains(caller, "logger/logger_test.go:") {
			t.Fatalf("caller did not resolve to the logging site: %v", entry)
		}
	}
	stack, _ := entries[1]["stacktrace"].(string)
	if entries[0]["stacktrace"] != nil || !strings.Contains(stack, "TestLogEncodersAndDiagnostics") {
		t.Fatalf("unexpected stack traces: %v", entries)
	}
	text, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "\tinfo\t") || !strings.Contains(string(text), "plain message") {
		t.Fatalf("unexpected console output: %s", text)
	}
}

func readLogEntries(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	for decoder.More() {
		var entry map[string]any
		if err := decoder.Decode(&entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	return entries
}
