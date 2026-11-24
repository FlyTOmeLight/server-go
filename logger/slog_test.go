package logger

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type testLogValue struct{}

func (testLogValue) LogValue() slog.Value {
	return slog.StringValue("resolved")
}

func TestSlogAndChildLoggersShareOutputsAndLevel(t *testing.T) {
	for _, rotation := range []bool{false, true} {
		name := "plain"
		if rotation {
			name = "rotating"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "app.log")
			root, err := NewLogger(&Config{
				AddCaller: true,
				Outputs:   []Output{{Type: "file", Path: path, Rotation: rotation}},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(root.Close)
			root.With().Close()
			child := root.With("component", "worker", zap.Int("attempt", 2))
			if err := child.Sync(); err != nil {
				t.Fatal(err)
			}
			child.Close()

			span := trace.NewSpanContext(
				trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}},
			)
			ctx := trace.ContextWithSpanContext(
				context.WithValue(context.Background(), "RequestID", "request-1"),
				span,
			)
			standard := child.Slog().With("operation", "work").WithGroup("job").
				With("id", 7).WithGroup("result")
			standard.DebugContext(ctx, "filtered")
			if root.Level() != "info" {
				t.Fatalf("default level = %q", root.Level())
			}
			if err := child.SetLevel("debug"); err != nil {
				t.Fatal(err)
			}
			if root.Level() != "debug" {
				t.Fatalf("parent level = %q", root.Level())
			}
			standard.DebugContext(
				ctx,
				"standard log",
				slog.Group("size", "bytes", 9),
				"value",
				testLogValue{},
			)
			standard.InfoContext(ctx, "empty result")
			child.Infof(ctx, "child log")
			root.Infof(context.Background(), "parent log")
			root.Close()
			child.Close()

			entries := readLogEntries(t, path)
			if len(entries) != 4 {
				t.Fatalf("unexpected entries: %v", entries)
			}
			for _, entry := range entries[:3] {
				if entry["component"] != "worker" || entry["attempt"] != float64(2) {
					t.Fatalf("missing child fields: %v", entry)
				}
				if entry["requestId"] != "request-1" || entry["traceId"] != span.TraceID().String() ||
					entry["spanId"] != span.SpanID().String() {
					t.Fatalf("missing context fields: %v", entry)
				}
			}
			if caller, _ := entries[0]["caller"].(string); !strings.Contains(
				caller,
				"logger/slog_test.go:",
			) {
				t.Fatalf("unexpected slog caller %q", caller)
			}
			first := entries[0]
			job, _ := first["job"].(map[string]any)
			result, _ := job["result"].(map[string]any)
			size, _ := result["size"].(map[string]any)
			if first["level"] != "debug" || first["operation"] != "work" || job["id"] != float64(7) ||
				size["bytes"] != float64(9) || result["value"] != "resolved" {
				t.Fatalf("unexpected structured slog entry: %v", first)
			}
			secondJob, _ := entries[1]["job"].(map[string]any)
			if secondJob["id"] != float64(7) || secondJob["result"] != nil {
				t.Fatalf("empty trailing group leaked into log: %v", entries[1])
			}
			if entries[3]["component"] != nil || entries[3]["traceId"] != nil ||
				entries[3]["msg"] != "parent log" {
				t.Fatalf("child fields affected parent: %v", entries[3])
			}
		})
	}
}
