package helloworldv1

import (
	"strings"

	"github.com/FlyTOmeLight/server-go/sdk"
)

type Client struct {
	*sdk.Client
}

var _ HelloWorldAPI = (*Client)(nil)

// NewClient creates an unauthenticated client with the default request timeout.
// Use NewClientWithConfig to configure API key authentication or transport options.
func NewClient(endpoint string) (*Client, error) {
	return NewClientWithConfig(
		&sdk.Config{Endpoint: strings.TrimRight(endpoint, "/") + "/helloworld/internal", Timeout: 30},
	)
}

func NewClientWithConfig(config *sdk.Config) (*Client, error) {
	client, err := sdk.NewClientWithConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{Client: client}, nil
}
