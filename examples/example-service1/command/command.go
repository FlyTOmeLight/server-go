package command

import (
	"github.com/FlyTOmeLight/server-go/cli"

	sdk "github.com/FlyTOmeLight/server-go/examples/example-service1/sdk/go/trainv1"
)

var client *sdk.Client

func initClient() {
	client = &sdk.Client{Client: cli.Config.Client}
}
