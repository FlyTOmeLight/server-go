package command

import (
	"github.com/FlyTOmeLight/server-go/cli"
	"github.com/spf13/cobra"
)

var _ cli.CommandBuilder = &JobCommandBuilder{}

type JobCommandBuilder struct{}

func (j JobCommandBuilder) NewCommand() *cobra.Command {
	// 1. 整个命令是一棵树，先把二级父命令job添加到commandline的根下面
	jobCommand := &cobra.Command{
		Use: "job",
	}
	// 2. 创建二级父命令下面的子命令create
	jobCommand.AddCommand(createJobCommand())

	// 3. 创建二级父命令下面的子命令delete
	jobCommand.AddCommand(deleteJobCommand())
	return jobCommand
}

func createJobCommand() *cobra.Command {
	var jobName string

	cmd := &cobra.Command{
		Use: "create",
		RunE: func(cmd *cobra.Command, args []string) error {
			return createJob()
		},
	}

	cmd.Flags().StringVarP(&jobName, "jobName", "n", "", "Name of job")
	return cmd
}

func deleteJobCommand() *cobra.Command {
	var jobName string

	cmd := &cobra.Command{
		Use: "delete",
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteJob()
		},
	}

	cmd.Flags().StringVarP(&jobName, "jobName", "n", "", "Name of job")
	return cmd
}

func createJob() error {
	return nil
}

func deleteJob() error {
	return nil
}
