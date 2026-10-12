package sdk

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"resty.dev/v3"
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
	if c.borrowed {
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

	if config.ProxyURL != "" {
		client = client.SetProxy(config.ProxyURL)
	}
	if config.Timeout > 0 {
		client = client.SetTimeout(time.Duration(config.Timeout) * time.Second)
	}

	if config.Retry > 0 {
		client = client.SetRetryCount(config.Retry).
			SetRetryWaitTime(2 * time.Second).
			SetRetryMaxWaitTime(10 * time.Second)
	}
	if config.Credentials != nil {
		client = client.SetAuthToken(config.Credentials.APIKey)
	}

	return &Client{Client: client, Config: config}, nil
}

func copyConfig(config *Config) *Config {
	copied := *config
	if config.Credentials != nil {
		credentials := *config.Credentials
		copied.Credentials = &credentials
	}
	return &copied
}
