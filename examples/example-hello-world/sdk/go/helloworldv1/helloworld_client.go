package helloworldv1

import (
	"errors"

	"github.com/FlyTOmeLight/server-go/sdk"
)

type Client struct {
	*sdk.Client
}

var _ HelloWorldAPI = (*Client)(nil)

func NewClient(ak, sk, endpoint string) (*Client, error) {
	if endpoint == "" {
		return nil, errors.New("endpoint should not be empty")
	}

	return NewClientWithConfig(&sdk.Config{
		Endpoint: endpoint + "/helloworld/openapi",
	}), nil
}

func NewClientWithConfig(config *sdk.Config) *Client {
	client := &Client{Client: sdk.NewClientWithConfig(config)}
	return client
}
