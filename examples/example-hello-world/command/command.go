package command

import (
	"strings"

	"github.com/FlyTOmeLight/server-go/cli"

	sdk "github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

var client *sdk.Client

func initClient() {
	base := cli.Config.Client.Clone()
	base.SetBaseURL(strings.TrimRight(base.Config.Endpoint, "/") + "/helloworld/internal")
	client = &sdk.Client{Client: base}
}
