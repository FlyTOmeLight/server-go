// Package greeting provides a streaming HTTP client. The caller owns a returned stream.
package greeting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	serrors "github.com/FlyTOmeLight/server-go/errors"
)

type Client struct {
	endpoint string
	http     *http.Client
}

func NewClient(endpoint string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("endpoint must be an absolute HTTP or HTTPS URL")
	}
	return &Client{endpoint: strings.TrimRight(endpoint, "/"), http: &http.Client{Timeout: 35 * time.Second}}, nil
}

// Open returns a live SSE body. Close it when done or when stopping early.
// HTTP failures close their own body; stream errors arrive as error events.
func (c *Client) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.endpoint+"/stream?"+url.Values{"name": {name}}.Encode(),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		var detail serrors.Error
		if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil || detail.Code == "" {
			return nil, serrors.NewError(
				resp.StatusCode,
				"UnexpectedResponse",
				http.StatusText(resp.StatusCode),
			)
		}
		detail.Status = resp.StatusCode
		return nil, &detail
	}
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType != "text/event-stream" {
		resp.Body.Close()
		return nil, fmt.Errorf("expected text/event-stream, received %q", resp.Header.Get("Content-Type"))
	}
	return resp.Body, nil
}
