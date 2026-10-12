package helloworldv1

import (
	"github.com/FlyTOmeLight/server-go/internalsdk"
	core "github.com/FlyTOmeLight/server-go/pkg"

	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

type Client struct {
	*helloworldv1.Client
}

var _ helloworldv1.HelloWorldAPI = (*Client)(nil)

func Wrap(c *Client, ctx *core.Context) *Client {
	t := internalsdk.Wrap(c.Client.Client, ctx)
	return &Client{Client: &helloworldv1.Client{Client: t.Client}}
}

func NewClient(ak, sk, endpoint string) (*Client, error) {
	c, err := internalsdk.NewInternalClient(ak, sk, endpoint+"/helloworld/internal")
	if err != nil {
		return nil, err
	}

	return &Client{Client: &helloworldv1.Client{Client: c.Client}}, nil
}

func NewClientWithConfig(config *internalsdk.InternalClientConfig) *Client {
	config.Endpoint += "/helloworld/internal"
	c := internalsdk.NewInternalClientWithConfig(config)
	return &Client{Client: &helloworldv1.Client{Client: c.Client}}
}
