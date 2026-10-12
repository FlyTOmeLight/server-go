package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const defaultRootName = "ctl"

type CommandLine struct {
	Root *cobra.Command
}

type CommandBuilder interface {
	NewCommand() *cobra.Command
}

func DefaultName() string {
	exe, err := os.Executable()
	if err != nil {
		exe = defaultRootName
	}
	base := filepath.Base(exe)
	return base
}

func New() *CommandLine {
	name := DefaultName()
	return NewCommandLineWithName(name)
}

func NewCommandLine(cmd *cobra.Command) *CommandLine {
	return &CommandLine{Root: cmd}
}

func NewCommandLineWithName(name string) *CommandLine {
	initConfig(name)
	cli := &CommandLine{
		Root: &cobra.Command{
			Use: name,
		},
	}
	cli.Root.AddCommand(NewConfigCommand())
	cli.Root.AddCommand(NewApplyCommand())
	cli.Root.AddCommand(NewVersionCommand())
	cli.Root.AddCommand(NewEncryptCommand())
	return cli
}

func (cli *CommandLine) Append(builders ...CommandBuilder) {
	for _, builder := range builders {
		cmd := builder.NewCommand()

		persistentPreRunE := cmd.PersistentPreRunE
		cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			err := SetConfig()
			if err != nil {
				return err
			}
			if persistentPreRunE == nil {
				return nil
			}
			return persistentPreRunE(cmd, args)
		}

		cli.Root.AddCommand(cmd)
	}
}

func (cli *CommandLine) Execute() {
	if err := cli.Root.Execute(); err != nil {
		os.Exit(1)
	}
}
