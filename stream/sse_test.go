package stream

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type eventWriter struct {
	*httptest.ResponseRecorder
	writeErr, flushErr error
}

func (w *eventWriter) SetWriteDeadline(time.Time) error { return nil }

func (w *eventWriter) Write(data []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.ResponseRecorder.Write(data)
}

func (w *eventWriter) FlushError() error { w.ResponseRecorder.Flush(); return w.flushErr }

func TestSSEEncodingAndFailures(t *testing.T) {
	w := &eventWriter{ResponseRecorder: httptest.NewRecorder()}
	events, err := NewSSE(w, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := events.Send("message", map[string]string{"text": "hello\nworld"}); err != nil {
		t.Fatal(err)
	}
	if w.Body.String() != "event:message\ndata:{\"text\":\"hello\\nworld\"}\n\n" || !w.Flushed {
		t.Fatalf("incorrect SSE: %s", w.Body)
	}
	before := w.Body.String()
	if err := events.Send(
		"message",
		map[string]any{"invalid": func() {}},
	); err == nil ||
		w.Body.String() != before {
		t.Fatal("encoding error wrote a partial event")
	}
	failed := errors.New("transport failed")
	w.writeErr = failed
	if err := events.Send("message", "text"); !errors.Is(err, failed) {
		t.Fatalf("write error lost: %v", err)
	}
	w.writeErr = nil
	w.flushErr = failed
	if err := events.Send("message", "text"); !errors.Is(err, failed) {
		t.Fatalf("flush error lost: %v", err)
	}
}

func TestSSERequiresDeadlineSupport(t *testing.T) {
	if _, err := NewSSE(httptest.NewRecorder(), time.Second); err == nil {
		t.Fatal("unsupported deadline accepted")
	}
	if _, err := NewSSE(&eventWriter{ResponseRecorder: httptest.NewRecorder()}, 0); err == nil {
		t.Fatal("unbounded write accepted")
	}
}

func TestSSEIdleProducerDoesNotConsumeWriteTimeout(t *testing.T) {
	for _, http2 := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP1", true: "HTTP2"}[http2], func(t *testing.T) {
			next := make(chan struct{})
			done := make(chan error, 1)
			server := httptest.NewUnstartedServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					events, err := NewSSE(w, 100*time.Millisecond)
					if err == nil {
						err = events.Send("first", "one")
					}
					if err != nil {
						done <- err
						return
					}
					select {
					case <-next:
					case <-r.Context().Done():
						done <- r.Context().Err()
						return
					}
					done <- events.Send("second", "two")
				}),
			)
			server.EnableHTTP2 = http2
			server.StartTLS()
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if http2 && response.ProtoMajor != 2 {
				t.Fatalf("expected HTTP2, got %s", response.Proto)
			}
			reader := bufio.NewScanner(response.Body)
			if !reader.Scan() || reader.Text() != "event:first" {
				t.Fatalf("first event: %q, %v", reader.Text(), reader.Err())
			}
			<-time.After(200 * time.Millisecond)
			close(next)
			var remaining strings.Builder
			for reader.Scan() {
				remaining.WriteString(reader.Text())
				remaining.WriteByte('\n')
			}
			if err := reader.Err(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(remaining.String(), "event:second") {
				t.Fatal("second event was lost")
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}

func TestSSEEventIDAndHeartbeat(t *testing.T) {
	w := &eventWriter{ResponseRecorder: httptest.NewRecorder()}
	events, err := NewSSE(w, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := events.SendWithID("run:3", "completed", "done"); err != nil {
		t.Fatal(err)
	}
	if err := events.Heartbeat(); err != nil {
		t.Fatal(err)
	}
	if got := w.Body.String(); got != "id:run:3\nevent:completed\ndata:done\n\n: heartbeat\n\n" || !w.Flushed {
		t.Fatalf("unexpected SSE: %q", got)
	}
}
