package main

import (
	"github.com/FlyTOmeLight/server-go/cli"

	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/command"
)

func main() {
	commandline := cli.New()

	commandline.Append(&command.ModuleCommandBuilder{})

	commandline.Execute()
}
