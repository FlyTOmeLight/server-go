package main

import (
	"github.com/FlyTOmeLight/server-go/cli"
	hello "github.com/FlyTOmeLight/server-go/examples/example-hello-world/command"
	dataset "github.com/FlyTOmeLight/server-go/examples/example-service1/command"
)

func main() {
	commandline := cli.New()
	commandline.Append(&hello.ModuleCommandBuilder{}, &dataset.DatasetCommandBuilder{})
	commandline.Execute()
}
