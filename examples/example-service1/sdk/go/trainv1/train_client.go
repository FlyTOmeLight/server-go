package trainv1

import (
	"github.com/FlyTOmeLight/server-go/sdk"
)

type Client struct {
	*sdk.Client
}

var _ TrainingAPI = (*Client)(nil)

// NewClient creates an unauthenticated client with the default request timeout.
// Use NewClientWithConfig to configure API key authentication or transport options.
func NewClient(endpoint string) (*Client, error) {
	return NewClientWithConfig(&sdk.Config{Endpoint: endpoint, Timeout: 30})
}

func NewClientWithConfig(config *sdk.Config) (*Client, error) {
	client, err := sdk.NewClientWithConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{Client: client}, nil
}
