package sdk

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"

	"resty.dev/v3"

	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/errors"
)

// Complete finishes a buffered JSON request after Resty has decoded its result.
// Transport, cancellation and decoding errors keep their original identity.
func Complete(response *resty.Response, err error) error {
	if err != nil {
		return err
	}
	if response == nil {
		return stderrors.New("empty HTTP response")
	}
	if response.IsStatusFailure() {
		detail := DecodeError(response.StatusCode(), bytes.NewReader(response.Bytes()))
		if detail.RequestID == "" {
			detail.RequestID = response.Header().Get(constants.HeaderRequestID)
		}
		return detail
	}
	return nil
}

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
