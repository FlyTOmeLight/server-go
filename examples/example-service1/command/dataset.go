package command

import (
	"fmt"

	"github.com/FlyTOmeLight/server-go/cli"
	"github.com/spf13/cobra"

	"github.com/FlyTOmeLight/server-go/examples/example-service1/sdk/go/trainv1"
)

type DatasetCommandBuilder struct{}

var _ cli.CommandBuilder = &DatasetCommandBuilder{}

func (b *DatasetCommandBuilder) NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "dataset",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			initClient()
			return nil
		},
		Run:     func(cmd *cobra.Command, args []string) {},
		Example: cli.DefaultName() + " dataset create --project-name yy --local-name xx",
	}

	cmd.AddCommand(createDatasetCommand())
	cmd.AddCommand(listDatasetCommand())
	cmd.AddCommand(getDatasetCommand())
	cmd.AddCommand(updateDatasetCommand())
	cmd.AddCommand(deleteDatasetCommand())

	applyDataset()

	return cmd
}

func createDatasetCommand() *cobra.Command {
	var req trainv1.CreateDatasetRequest
	var uri string
	var tags map[string]string
	var alias []string
	var artifactDesc string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "create dataset",
		RunE: func(cmd *cobra.Command, args []string) error {
			return createDataset(&req)
		},
		Example: cli.DefaultName() + " dataset create --project-name=yy --local-name=xx --artifact-uri=zz",
	}

	cmd.Flags().StringVar(&req.LocalName, "local-name", "", "Dataset localName (required)")
	cmd.Flags().StringVar(&req.ProjectName, "project-name", "", "Project name (required)")
	cmd.Flags().StringVar(&req.Description, "description", "", "Dataset description")

	cmd.Flags().StringVar(&req.FileFormat, "file-format", "", "Dataset file format")

	cmd.Flags().StringVar(&uri, "artifact-uri", "", "Dataset artifactUri")
	cmd.Flags().StringVar(&artifactDesc, "--artifact-description", "", "Dataset artifact Description")
	cmd.Flags().StringArrayVar(&alias, "alias", []string{}, "Dataset alias")
	cmd.Flags().StringToStringVar(&tags, "tags", map[string]string{}, "Dataset tags, --tags=tag1")

	_ = cmd.MarkFlagRequired("local-name")
	_ = cmd.MarkFlagRequired("project-name")
	_ = cmd.MarkFlagRequired("artifact-uri")
	_ = cmd.MarkFlagRequired("category")

	return cmd
}

func listDatasetCommand() *cobra.Command {
	var req trainv1.ListDatasetRequest
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list dataset",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listDataset(&req)
		},
		Example: cli.DefaultName() + " dataset list --project-name=yy",
	}

	cmd.Flags().StringVar(&req.ProjectName, "project-name", "", "Project name (required)")
	cmd.Flags().IntVar(&req.PageSize, "page-size", 10, "Dataset list page size")
	cmd.Flags().IntVar(&req.PageNo, "page-no", 1, "Dataset list page no ")
	cmd.Flags().StringVar(&req.Order, "order", "", "Dataset list order")
	cmd.Flags().StringVar(&req.OrderBy, "order-by", "", "Dataset list order by")
	cmd.Flags().StringVar(&req.Filter, "filter", "", "Dataset list filter")

	_ = cmd.MarkFlagRequired("project-name")

	return cmd
}

func getDatasetCommand() *cobra.Command {
	var req trainv1.GetDatasetRequest

	cmd := &cobra.Command{
		Use:   "get",
		Short: "get dataset",
		RunE: func(cmd *cobra.Command, args []string) error {
			return getDataset(&req)
		},
		Example: cli.DefaultName() + " dataset get --project-name=yy --local-name=xx",
	}

	cmd.Flags().StringVar(&req.ProjectName, "project-name", "", "Project name (required)")
	cmd.Flags().StringVar(&req.LocalName, "local-name", "", "Dataset localName (required)")
	_ = cmd.MarkFlagRequired("local-name")
	_ = cmd.MarkFlagRequired("project-name")

	return cmd
}

func updateDatasetCommand() *cobra.Command {
	var req trainv1.UpdateDatasetRequest
	var description string
	var category string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update dataset",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Lookup("description").Changed {
				req.Description = &description
			}
			return updateDataset(&req)
		},
		Example: cli.DefaultName() + " dataset update --project-name=yy --local-name=xx --description=update",
	}

	cmd.Flags().StringVar(&req.ProjectName, "project-name", "", "Project name (required)")
	cmd.Flags().StringVar(&req.LocalName, "local-name", "", "Dataset localName (required)")
	cmd.Flags().StringVar(&description, "description", "", "Dataset description")
	cmd.Flags().StringVar(&category, "category", "", "Dataset category")

	_ = cmd.MarkFlagRequired("local-name")
	_ = cmd.MarkFlagRequired("project-name")

	return cmd
}

func deleteDatasetCommand() *cobra.Command {
	var req trainv1.DeleteDatasetRequest

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "delete dataset",
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteDataset(&req)
		},
		Example: cli.DefaultName() + " dataset get --project-name=yy --local-name=xx",
	}

	cmd.Flags().StringVar(&req.ProjectName, "project-name", "", "Project name (required)")
	cmd.Flags().StringVar(&req.LocalName, "local-name", "", "Dataset localName (required)")
	_ = cmd.MarkFlagRequired("local-name")
	_ = cmd.MarkFlagRequired("project-name")

	return cmd
}

func createDataset(req *trainv1.CreateDatasetRequest) error {
	resp, err := client.CreateDataset(req)
	if err != nil {
		return err
	}
	cli.PrintJSON(resp)

	return nil
}

func listDataset(req *trainv1.ListDatasetRequest) error {
	resp, err := client.ListDataset(req)
	if err != nil {
		return err
	}
	cli.PrintJSON(resp)

	return nil
}

func getDataset(req *trainv1.GetDatasetRequest) error {
	resp, err := client.GetDataset(req)
	if err != nil {
		return err
	}
	cli.PrintJSON(resp)

	return nil
}

func updateDataset(req *trainv1.UpdateDatasetRequest) error {
	err := client.UpdateDataset(req)
	if err != nil {
		return err
	}

	fmt.Println()
	return nil
}

func deleteDataset(req *trainv1.DeleteDatasetRequest) error {
	err := client.DeleteDataset(req)
	if err != nil {
		return err
	}

	fmt.Println()
	return nil
}

func applyDataset() {
	var req trainv1.CreateDatasetRequest
	cli.RegisterApplyHandler("Dataset", &req, func(args any) error {
		initClient()
		return createDataset(&req)
	})
}
