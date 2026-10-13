package helloworldv1

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

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

	var credentials *sdk.Credentials
	if ak != "" || sk != "" {
		credentials = &sdk.Credentials{AuthType: sdk.AuthTypeAKSK, AccessKeyId: ak, SecretAccessKey: sk}
	}
	return NewClientWithConfig(&sdk.Config{
		Credentials: credentials,
		Endpoint:    strings.TrimRight(endpoint, "/") + "/helloworld/internal",
		Timeout:     30,
	})
}

// responseError preserves HTTP failure even when the upstream returns HTML or invalid JSON.
func responseError(status int, body []byte) error {
	if status < http.StatusBadRequest {
		return nil
	}
	return sdk.DecodeError(status, bytes.NewReader(body))
}

func NewClientWithConfig(config *sdk.Config) (*Client, error) {
	client, err := sdk.NewClientWithConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{Client: client}, nil
}
