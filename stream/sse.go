// Package stream sends SSE events without retaining a complete response.
package stream

import (
	"bytes"
	"errors"
	"net/http"
	"time"

	"github.com/gin-contrib/sse"
)

// SSE belongs to one handler goroutine and must not be used after it returns.
type SSE struct {
	writer       http.ResponseWriter
	control      *http.ResponseController
	writeTimeout time.Duration
}

// NewSSE replaces the server's total write deadline with a deadline per event.
// Call it before writing the response. Keep the producer's context bounded.
// The handler owns the operation deadline and request cancellation. Middleware
// wrapping writer must support Unwrap for ResponseController. Use a positive
// writeTimeout to bound a blocked client. Do not cache the stream as an idempotent response.
func NewSSE(writer http.ResponseWriter, writeTimeout time.Duration) (*SSE, error) {
	if writeTimeout <= 0 {
		return nil, errors.New("SSE write timeout must be positive")
	}
	control := http.NewResponseController(writer)
	if err := control.SetWriteDeadline(time.Time{}); err != nil {
		return nil, err
	}
	writer.Header().Set("Content-Type", sse.ContentType)
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("X-Accel-Buffering", "no")
	return &SSE{writer: writer, control: control, writeTimeout: writeTimeout}, nil
}

// Send encodes and flushes one event. An encoding failure writes no partial event.
// Stop sending after a write or flush failure; the handler must cancel its producer.
func (s *SSE) Send(event string, data any) error {
	return s.SendWithID("", event, data)
}

// SendWithID includes the event cursor used by Last-Event-ID on reconnect.
func (s *SSE) SendWithID(id, event string, data any) error {
	var encoded bytes.Buffer
	if err := sse.Encode(&encoded, sse.Event{Id: id, Event: event, Data: data}); err != nil {
		return err
	}
	return s.write(encoded.Bytes())
}

// Heartbeat flushes a comment without dispatching an application event.
func (s *SSE) Heartbeat() error {
	return s.write([]byte(": heartbeat\n\n"))
}

func (s *SSE) write(data []byte) error {
	if err := s.control.SetWriteDeadline(time.Now().Add(s.writeTimeout)); err != nil {
		return err
	}
	if _, err := s.writer.Write(data); err != nil {
		return err
	}
	if err := s.control.Flush(); err != nil {
		return err
	}
	// Waiting for the next event is producer time, not client write time.
	// HTTP/2 otherwise resets an idle stream when this deadline expires.
	return s.control.SetWriteDeadline(time.Time{})
}
