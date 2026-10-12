package helloworldv1

import (
	"errors"

	"resty.dev/v3"

	"github.com/FlyTOmeLight/server-go/internalsdk"
	core "github.com/FlyTOmeLight/server-go/pkg"

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

func NewClient(ak, sk, endpoint string) (*Client, error) {
	c, err := internalsdk.NewInternalClient(ak, sk, endpoint+"/helloworld/internal")
	if err != nil {
		return nil, err
	}
	return &Client{Client: &helloworldv1.Client{Client: c.Client}, internal: c}, nil
}

func NewClientWithConfig(config *internalsdk.InternalClientConfig) (*Client, error) {
	if config == nil || config.Config == nil {
		return nil, errors.New("internal SDK config is required")
	}
	copied := *config
	base := *config.Config
	base.Endpoint += "/helloworld/internal"
	copied.Config = &base
	c, err := internalsdk.NewInternalClientWithConfig(&copied)
	if err != nil {
		return nil, err
	}
	return &Client{Client: &helloworldv1.Client{Client: c.Client}, internal: c}, nil
}
