package command

import (
	"context"

	"github.com/FlyTOmeLight/server-go/cli"

	sdk "github.com/FlyTOmeLight/server-go/examples/example-service1/sdk/go/trainv1"
)

func initClient(ctx context.Context) (*sdk.Client, error) {
	base, err := cli.Client(ctx)
	if err != nil {
		return nil, err
	}
	return &sdk.Client{Client: base}, nil
}
