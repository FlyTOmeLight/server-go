package main

import (
	"github.com/FlyTOmeLight/server-go/cli"
	cli1 "github.com/FlyTOmeLight/server-go/examples/example-service1/command"
	cli2 "github.com/FlyTOmeLight/server-go/examples/example-service2/command"
)

func main() {
	commandline := cli.New()

	// add subcommands
	commandline.Append(
		&cli1.DatasetCommandBuilder{},
		&cli2.JobCommandBuilder{})

	// run
	commandline.Execute()
}
