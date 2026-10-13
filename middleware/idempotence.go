package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	stderr "errors"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/idempotence"
	"github.com/FlyTOmeLight/server-go/internal/httpbody"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

// Idempotence caches JSON POST responses with a nonempty clientToken. Install
// after all authentication and authorization middleware, before business handlers.
// Only successful responses are retained; outer error rendering remains in charge.
// Identities must come from verified Gin context keys; requests without a verified
// identity do not use the cache.
func Idempotence(controller *idempotence.Controller) core.HandlerFunc {
	return idempotenceHandler(controller, func(c *core.Context) { c.Next() })
}

func idempotenceHandler(controller *idempotence.Controller, next core.HandlerFunc) core.HandlerFunc {
	return func(c *core.Context) {
		clientToken := c.Query(constants.QueryClientToken)
		mediaType, _, _ := mime.ParseMediaType(c.GetHeader(constants.HeaderContentType))
		orgID := c.GetString(constants.KeyOrgID)
		userID := c.GetString(constants.KeyUserID)
		if clientToken == "" || c.Request.Method != http.MethodPost || mediaType != constants.ContentTypeJSON ||
			orgID == "" ||
			userID == "" {
			next(c)
			return
		}
		if controller == nil {
			errors.AbortWithError(
				c,
				errors.NewError(
					http.StatusServiceUnavailable,
					"IdempotenceUnavailable",
					"idempotence controller is not configured",
				),
			)
			return
		}
		body, err := httpbody.Read(c)
		if err != nil {
			errors.AbortWithError(c, err)
			return
		}
		// Query parameters (including the token) and content type are part of
		// the request fingerprint, while the operation key uses the actual path.
		fingerprint := sha256.Sum256(
			append([]byte(c.Request.URL.RequestURI()+"\x00"+mediaType+"\x00"), body...),
		)
		key := idempotence.Key{
			OrgID:     orgID,
			UserID:    userID,
			Operation: c.Request.Method + " " + c.Request.URL.Path,
			Token:     clientToken,
		}
		executed := false
		response, err := controller.Do(
			c.Request.Context(),
			key,
			fingerprint,
			func() (idempotence.Response, error) {
				executed = true
				writer := &idempotenceWriter{ResponseWriter: c.Writer}
				original := c.Writer
				c.Writer = writer
				defer func() { c.Writer = original }()
				next(c)
				if lastErr := c.Errors.Last(); lastErr != nil {
					return idempotence.Response{}, lastErr.Err
				}
				writer.WriteHeaderNow()
				return idempotence.Response{
					Status: writer.Status(),
					Header: writer.header,
					Body:   writer.body.Bytes(),
				}, nil
			},
		)
		if err != nil {
			if executed {
				// The original request keeps its error for the outer renderer.
				return
			}
			switch {
			case stderr.Is(err, idempotence.ErrConflict):
				errors.AbortWithError(
					c,
					errors.NewError(
						http.StatusConflict,
						"IdempotentParameterMismatch",
						err.Error(),
					),
				)
			case stderr.Is(err, context.Canceled), stderr.Is(err, context.DeadlineExceeded):
				errors.AbortWithError(
					c,
					errors.NewError(
						http.StatusRequestTimeout,
						"IdempotenceWaitCanceled",
						err.Error(),
					),
				)
			case stderr.Is(err, idempotence.ErrClosed), stderr.Is(err, idempotence.ErrExecutionFailed):
				errors.AbortWithError(
					c,
					errors.NewError(
						http.StatusServiceUnavailable,
						"IdempotenceUnavailable",
						err.Error(),
					),
				)
			default:
				errors.AbortWithError(c, err)
			}
			return
		}
		if executed {
			return
		}
		// Replace headers as well as status and body. Requests still pass the
		// current authorization checks before reaching this replay branch.
		clear(c.Writer.Header())
		for key, values := range response.Header {
			c.Writer.Header()[key] = append([]string(nil), values...)
		}
		c.Status(response.Status)
		_, _ = c.Writer.Write(response.Body)
		c.Abort()
	}
}

type idempotenceWriter struct {
	gin.ResponseWriter
	body   bytes.Buffer
	header http.Header
}

func (w *idempotenceWriter) WriteHeaderNow() {
	if w.header == nil {
		w.header = w.Header().Clone()
	}
	w.ResponseWriter.WriteHeaderNow()
}

func (w *idempotenceWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	n, err := w.ResponseWriter.Write(data)
	_, _ = w.body.Write(data[:n])
	return n, err
}

func (w *idempotenceWriter) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}

// Idempotent wraps the final endpoint handler so preceding route-level
// authentication and authorization always execute, including on cache hits.
func Idempotent(controller *idempotence.Controller, handler core.HandlerFunc) core.HandlerFunc {
	return idempotenceHandler(controller, handler)
}
