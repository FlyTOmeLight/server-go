package helloworldv1

import (
	"errors"
	"strings"

	"github.com/FlyTOmeLight/server-go/internalsdk"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/sdk"
	"resty.dev/v3"

	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

type Client struct {
	*helloworldv1.Client
	internal *internalsdk.InternalClient
}

var _ helloworldv1.HelloWorldAPI = (*Client)(nil)

func Wrap(c *Client, ctx *core.Context) *resty.Request {
	return internalsdk.Wrap(c.internal, ctx)
}

// NewClient creates an unauthenticated internal client with the default timeout.
// Use NewClientWithConfig for API keys and request impersonation.
func NewClient(endpoint string) (*Client, error) {
	return NewClientWithConfig(&internalsdk.InternalClientConfig{
		Config: &sdk.Config{Endpoint: endpoint, Timeout: 30},
	})
}

func NewClientWithConfig(config *internalsdk.InternalClientConfig) (*Client, error) {
	if config == nil || config.Config == nil {
		return nil, errors.New("internal SDK config is required")
	}
	copied := *config
	base := *config.Config
	base.Endpoint = strings.TrimRight(base.Endpoint, "/") + "/helloworld/internal"
	copied.Config = &base
	c, err := internalsdk.NewInternalClientWithConfig(&copied)
	if err != nil {
		return nil, err
	}
	return &Client{Client: &helloworldv1.Client{Client: c.Client}, internal: c}, nil
}
