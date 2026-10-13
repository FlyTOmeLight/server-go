package sdk

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/FlyTOmeLight/server-go/errors"
)

// DecodeError preserves the transport status and framework error metadata.
// Non-JSON HTTP failures still return a checkable error with their actual status.
func DecodeError(status int, body io.Reader) *errors.Error {
	var detail errors.Error
	if err := json.NewDecoder(io.LimitReader(body, 64<<10)).Decode(&detail); err != nil || detail.Code == "" {
		detail = errors.Error{Code: "HTTPError", Message: http.StatusText(status), StatusCode: status}
	}
	detail.Status = status
	if detail.StatusCode == 0 {
		detail.StatusCode = status
	}
	return &detail
}
