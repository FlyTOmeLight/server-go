package command

import (
	"context"
	"strings"

	"github.com/FlyTOmeLight/server-go/cli"

	sdk "github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

func initClient(ctx context.Context) (*sdk.Client, error) {
	base, err := cli.Client(ctx)
	if err != nil {
		return nil, err
	}
	base = base.Clone()
	base.SetBaseURL(strings.TrimRight(base.Config.Endpoint, "/") + "/helloworld/internal")
	return &sdk.Client{Client: base}, nil
}
