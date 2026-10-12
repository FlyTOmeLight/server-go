package internalsdk

import (
	"errors"

	"resty.dev/v3"

	"github.com/FlyTOmeLight/server-go/constants"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/sdk"
)

type InternalClient struct {
	*sdk.Client
	Config *InternalClientConfig
}

type InternalClientConfig struct {
	*sdk.Config
	EnableImpersonate bool
	EnableIdempotence bool
	EnableSTS         bool
	STSDuration       int
	STSACL            string
}

func NewInternalClient(ak, sk, endpoint string) (*InternalClient, error) {
	return NewInternalClientWithConfig(&InternalClientConfig{
		Config: &sdk.Config{
			Endpoint: endpoint,
			Timeout:  30,
			Credentials: &sdk.Credentials{
				AuthType:        sdk.AuthTypeAKSK,
				AccessKeyId:     ak,
				SecretAccessKey: sk,
			},
		},
	})
}

func NewInternalClientWithConfig(config *InternalClientConfig) (*InternalClient, error) {
	if config == nil {
		return nil, errors.New("internal SDK config is required")
	}
	if config.EnableSTS {
		return nil, errors.New("internal SDK STS authentication is not implemented")
	}
	if config.EnableIdempotence {
		return nil, errors.New("internal SDK idempotence is not implemented")
	}
	client, err := sdk.NewClientWithConfig(config.Config)
	if err != nil {
		return nil, err
	}
	copied := *config
	copied.Config = client.Config
	return &InternalClient{Client: client, Config: &copied}, nil
}

// Wrap creates a downstream request. It borrows the client and does not retain
// the inbound Gin context or modify any shared client settings.
func Wrap(client *InternalClient, ctx *core.Context) *resty.Request {
	return client.R(ctx)
}

// R creates a request tied to the inbound request's cancellation and deadline.
// ctx must contain an inbound HTTP request.
func (c *InternalClient) R(ctx *core.Context) *resty.Request {
	request := c.Client.R().SetContext(ctx.Request.Context())
	if c.Config.EnableImpersonate {
		request.SetHeader(constants.HeaderImpersonateOrgID, ctx.GetString(constants.KeyOrgID))
		request.SetHeader(constants.HeaderImpersonateUserID, ctx.GetString(constants.KeyUserID))
	}
	return request
}
