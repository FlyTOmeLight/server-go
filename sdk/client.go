package sdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"resty.dev/v3"

	"github.com/FlyTOmeLight/server-go/constants"
)

const (
	AuthTypeAPIKey = "api_key"
	AuthTypeAKSK   = "ak_sk"
)

var ErrUnsupportedAuth = errors.New("SDK authentication mode is not implemented")

type Credentials struct {
	// 通用字段
	AuthType string

	// API Key 方式
	APIKey string

	// AK/SK 方式
	AccessKeyId string

	SecretAccessKey string
}

type Config struct {
	// Endpoint Client访问地址，可以带URLPath，例如：http://0.0.0.0:80/v1
	Endpoint string

	ProxyURL string
	Retry    int
	Timeout  int

	Credentials *Credentials
}

type Client struct {
	*resty.Client
	Config    *Config
	borrowed  bool
	closeOnce sync.Once
	closeErr  error
}

// Clone preserves the Resty settings and borrows the original client's resources.
// Call it only during setup, before either client is used concurrently. Close the
// original client after all clones have finished using it.
func (c *Client) Clone() *Client {
	return &Client{Client: c.Client.Clone(context.Background()), Config: copyConfig(c.Config), borrowed: true}
}

// Close releases resources owned by this client. Borrowed clones do not own them.
func (c *Client) Close() error {
	if c == nil || c.borrowed {
		return nil
	}
	c.closeOnce.Do(func() { c.closeErr = c.Client.Close() })
	return c.closeErr
}

func NewClientWithConfig(config *Config) (*Client, error) {
	if config == nil {
		return nil, errors.New("SDK config is required")
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, errors.New("SDK endpoint must be an absolute HTTP or HTTPS URL")
	}
	if config.ProxyURL != "" {
		proxy, err := url.Parse(config.ProxyURL)
		if err != nil || proxy.Hostname() == "" ||
			(proxy.Scheme != "http" && proxy.Scheme != "https" && proxy.Scheme != "socks5") {
			return nil, errors.New("SDK proxy must be an absolute HTTP, HTTPS or SOCKS5 URL")
		}
	}
	if config.Timeout < 0 || config.Retry < 0 {
		return nil, errors.New("SDK timeout and retry count must not be negative")
	}
	if config.Credentials != nil {
		if config.Credentials.AuthType != AuthTypeAPIKey {
			return nil, fmt.Errorf("%w: %q", ErrUnsupportedAuth, config.Credentials.AuthType)
		}
		if config.Credentials.APIKey == "" {
			return nil, errors.New("SDK API key must not be empty")
		}
	}
	config = copyConfig(config)
	client := resty.New().SetBaseURL(config.Endpoint)
	client.AddRequestMiddleware(func(_ *resty.Client, request *resty.Request) error {
		ctx := request.Context()
		if id, ok := ctx.Value(constants.KeyRequestID).(string); ok && id != "" {
			if _, explicit := requestHeader(request.Header, constants.HeaderRequestID); !explicit {
				request.SetHeader(constants.HeaderRequestID, id)
			}
		}
		propagator := otel.GetTextMapPropagator()
		if len(propagator.Fields()) == 0 {
			propagator = propagation.TraceContext{}
		}
		propagator.Inject(ctx, propagation.HeaderCarrier(request.Header))
		return nil
	})

	if config.ProxyURL != "" {
		client = client.SetProxy(config.ProxyURL)
	}
	if config.Timeout > 0 {
		client = client.SetTimeout(time.Duration(config.Timeout) * time.Second)
	}

	if config.Retry > 0 {
		client.SetRetryCount(config.Retry).
			SetRetryWaitTime(2*time.Second).
			SetRetryMaxWaitTime(10*time.Second).
			SetRetryAllowNonIdempotent(true).
			AddRetryConditions(resty.RetryConditionStatusTooManyRequests, resty.RetryConditionStatus5XX)
		client.AddRequestMiddleware(func(_ *resty.Client, request *resty.Request) error {
			// Resty's retry conditions are ORed with the built-in transport checks.
			if !canRetry(request) {
				request.SetRetryCount(0)
			}
			return nil
		})
	}
	if config.Credentials != nil {
		client = client.SetAuthToken(config.Credentials.APIKey)
	}

	return &Client{Client: client, Config: config}, nil
}

func canRetry(request *resty.Request) bool {
	switch request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	}
	if values, _ := requestHeader(
		request.Header,
		"Idempotency-Key",
	); len(values) > 0 &&
		strings.TrimSpace(values[0]) != "" {
		return true
	}
	// Resty appends QueryParams after the query parameters already in the URL.
	if parsed, err := url.Parse(request.URL); err == nil {
		query := parsed.Query()
		if query.Has(constants.QueryClientToken) {
			return strings.TrimSpace(query.Get(constants.QueryClientToken)) != ""
		}
	}
	return strings.TrimSpace(request.QueryParams.Get(constants.QueryClientToken)) != ""
}

func requestHeader(headers http.Header, key string) ([]string, bool) {
	if values, ok := headers[http.CanonicalHeaderKey(key)]; ok {
		return values, true
	}
	// Resty also allows callers to set header names verbatim.
	for name, values := range headers {
		if strings.EqualFold(name, key) {
			return values, true
		}
	}
	return nil, false
}

func copyConfig(config *Config) *Config {
	copied := *config
	if config.Credentials != nil {
		credentials := *config.Credentials
		copied.Credentials = &credentials
	}
	return &copied
}
