package core

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"
)

type BufferedWriter struct {
	gin.ResponseWriter
	Buf *bytes.Buffer

	// Limit bounds captured bytes when positive. Zero preserves full capture.
	Limit int
}

func (w BufferedWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	captured := b[:n]
	if w.Limit > 0 {
		captured = captured[:min(len(captured), max(0, w.Limit-w.Buf.Len()))]
	}
	w.Buf.Write(captured)
	return n, err
}

func (w BufferedWriter) WriteString(data string) (int, error) { return w.Write([]byte(data)) }

func (w BufferedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
