package main

import (
	"github.com/FlyTOmeLight/server-go/cli"

	"github.com/FlyTOmeLight/server-go/examples/example-service2/command"
)

func main() {
	commandline := cli.New()

	commandline.Append(&command.JobCommandBuilder{})

	commandline.Execute()
}
