package trainv1

import (
	"errors"

	"github.com/FlyTOmeLight/server-go/sdk"
)

type Client struct {
	*sdk.Client
}

var _ TrainingAPI = (*Client)(nil)

func NewClient(ak, sk, endpoint string) (*Client, error) {
	if endpoint == "" {
		return nil, errors.New("endpoint should not be empty")
	}
	if ak == "" {
		return nil, errors.New("ak should not be empty")
	}
	if sk == "" {
		return nil, errors.New("sk should not be empty")
	}

	return NewClientWithConfig(&sdk.Config{
		Endpoint: endpoint,
	}), nil
}

func NewClientWithConfig(config *sdk.Config) *Client {
	client := &Client{Client: sdk.NewClientWithConfig(config)}
	return client
}
