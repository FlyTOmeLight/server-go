package command

import (
	"github.com/FlyTOmeLight/server-go/cli"
	sdk "github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

var client *sdk.Client

func initClient() {
	client = &sdk.Client{Client: cli.Config.Client}
}
