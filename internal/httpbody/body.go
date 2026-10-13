// Package httpbody shares JSON read limits and captured bodies across HTTP middleware.
package httpbody

import (
	"bytes"
	stderrors "errors"
	"io"
	"net/http"
	"strings"

	"github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

const MaxJSONBytes = 1 << 20

const captureKey = "server.httpbody.capture"

type capture struct {
	body []byte
	err  error
}

func IsJSON(c *core.Context) bool {
	contentType := c.GetHeader("Content-Type")
	mediaType, _, _ := strings.Cut(contentType, ";")
	// Framework JSON binding also accepts requests without a content type.
	// Invalid MIME parameters must not bypass the read budget when the main
	// media type is JSON: the binder still accepts that request body.
	return strings.EqualFold(strings.TrimSpace(mediaType), "application/json") || contentType == ""
}

// BoundJSON limits reads before decoding, including requests with unknown length.
func BoundJSON(c *core.Context) {
	if c.Request.Body == nil {
		c.Request.Body = http.NoBody
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxJSONBytes)
}

// Read captures a body once and restores it for binding. JSON captures use the
// same budget as decoding; other formats retain their existing read policy.
func Read(c *core.Context) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}
	if value, exists := c.Get(captureKey); exists {
		result := value.(*capture)
		c.Request.Body = io.NopCloser(bytes.NewReader(result.body))
		return result.body, result.err
	}
	if IsJSON(c) {
		BoundJSON(c)
	}
	body, err := io.ReadAll(c.Request.Body)
	err = ReadError(err)
	if err != nil && !errors.IsStatusCode(err, http.StatusRequestEntityTooLarge) {
		err = errors.NewError(http.StatusBadRequest, "InvalidRequestBody", "cannot read request body")
	}
	c.Set(captureKey, &capture{body: body, err: err})
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, err
}

// ReadError gives every JSON consumer the same oversized-body error contract.
func ReadError(err error) error {
	var oversized *http.MaxBytesError
	if stderrors.As(err, &oversized) {
		return errors.NewError(
			http.StatusRequestEntityTooLarge,
			"PayloadTooLarge",
			"JSON body exceeds size limit",
		)
	}
	return err
}
