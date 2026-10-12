package command

import (
	"fmt"

	"github.com/FlyTOmeLight/server-go/cli"
	"github.com/spf13/cobra"

	sdk "github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

type ModuleCommandBuilder struct{}

var _ cli.CommandBuilder = &ModuleCommandBuilder{}

func (b *ModuleCommandBuilder) NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "helloworld",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			initClient()
			return nil
		},
		Run:     func(cmd *cobra.Command, args []string) {},
		Example: cli.DefaultName() + " module create --name xx",
	}

	cmd.AddCommand(createModuleCommand())
	cmd.AddCommand(listModuleCommand())
	cmd.AddCommand(getModuleCommand())
	cmd.AddCommand(updateModuleCommand())
	cmd.AddCommand(deleteModuleCommand())

	applyModule()

	return cmd
}

func createModuleCommand() *cobra.Command {
	var req sdk.CreateModuleRequest

	cmd := &cobra.Command{
		Use:   "create",
		Short: "create module",
		RunE: func(cmd *cobra.Command, args []string) error {
			return createModule(&req)
		},
		Example: cli.DefaultName() + " module create --local-name=xx",
	}

	cmd.Flags().StringVar(&req.Name, "name", "", " Module name (required)")
	cmd.Flags().StringVar(&req.Description, "description", "", " Module description")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func listModuleCommand() *cobra.Command {
	var req sdk.ListModuleRequest
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list module",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listModule(&req)
		},
		Example: cli.DefaultName() + " module list",
	}

	cmd.Flags().IntVar(&req.PageSize, "page-size", 10, " Module list page size")
	cmd.Flags().IntVar(&req.PageNo, "page-no", 1, " Module list page no ")
	cmd.Flags().StringVar(&req.Order, "order", "", " Module list order")
	cmd.Flags().StringVar(&req.OrderBy, "order-by", "", " Module list order by")
	cmd.Flags().StringVar(&req.Filter, "filter", "", " Module list filter")

	return cmd
}

func getModuleCommand() *cobra.Command {
	var req sdk.GetModuleRequest

	cmd := &cobra.Command{
		Use:   "get",
		Short: "get module",
		RunE: func(cmd *cobra.Command, args []string) error {
			return getModule(&req)
		},
		Example: cli.DefaultName() + " module get --id=xx",
	}

	cmd.Flags().StringVar(&req.ID, "id", "", " Module id (required)")
	_ = cmd.MarkFlagRequired("id")

	return cmd
}

func updateModuleCommand() *cobra.Command {
	var req sdk.UpdateModuleRequest
	var description string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update module",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Lookup("description").Changed {
				req.Description = &description
			}
			return updateModule(&req)
		},
		Example: cli.DefaultName() + " module update --id=xx --description=update",
	}

	cmd.Flags().StringVar(&req.ID, "id", "", " Module id (required)")
	cmd.Flags().StringVar(&description, "description", "", " Module description")

	_ = cmd.MarkFlagRequired("id")

	return cmd
}

func deleteModuleCommand() *cobra.Command {
	var req sdk.DeleteModuleRequest

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "delete module",
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteModule(&req)
		},
		Example: cli.DefaultName() + " module get --id=xx",
	}

	cmd.Flags().StringVar(&req.ID, "id", "", " Module id (required)")
	_ = cmd.MarkFlagRequired("id")

	return cmd
}

func createModule(req *sdk.CreateModuleRequest) error {
	resp, err := client.CreateModule(req)
	if err != nil {
		return err
	}
	cli.PrintJSON(resp)

	return nil
}

func listModule(req *sdk.ListModuleRequest) error {
	resp, err := client.ListModule(req)
	if err != nil {
		return err
	}
	cli.PrintJSON(resp)

	return nil
}

func getModule(req *sdk.GetModuleRequest) error {
	resp, err := client.GetModule(req)
	if err != nil {
		return err
	}
	cli.PrintJSON(resp)

	return nil
}

func updateModule(req *sdk.UpdateModuleRequest) error {
	err := client.UpdateModule(req)
	if err != nil {
		return err
	}

	fmt.Println()
	return nil
}

func deleteModule(req *sdk.DeleteModuleRequest) error {
	err := client.DeleteModule(req)
	if err != nil {
		return err
	}

	fmt.Println()
	return nil
}

func applyModule() {
	var req sdk.CreateModuleRequest
	cli.RegisterApplyHandler("Example", &req, func(args any) error {
		initClient()
		return createModule(&req)
	})
}
