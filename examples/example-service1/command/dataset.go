package command

import (
	"context"
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

	cmd := &cobra.Command{
		Use:   "create",
		Short: "create dataset",
		RunE: func(cmd *cobra.Command, args []string) error {
			return createDataset(cmd.Context(), &req)
		},
		Example: cli.DefaultName() + " dataset create --project-name=yy --local-name=xx",
	}

	cmd.Flags().StringVar(&req.LocalName, "local-name", "", "Dataset localName (required)")
	cmd.Flags().StringVar(&req.ProjectName, "project-name", "", "Project name (required)")
	cmd.Flags().StringVar(&req.Description, "description", "", "Dataset description")

	cmd.Flags().StringVar(&req.FileFormat, "file-format", "", "Dataset file format")

	_ = cmd.MarkFlagRequired("local-name")
	_ = cmd.MarkFlagRequired("project-name")

	return cmd
}

func listDatasetCommand() *cobra.Command {
	var req trainv1.ListDatasetRequest
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list dataset",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listDataset(cmd.Context(), &req)
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
			return getDataset(cmd.Context(), &req)
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
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update dataset",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Lookup("description").Changed {
				req.Description = &description
			}
			return updateDataset(cmd.Context(), &req)
		},
		Example: cli.DefaultName() + " dataset update --project-name=yy --local-name=xx --description=update",
	}

	cmd.Flags().StringVar(&req.ProjectName, "project-name", "", "Project name (required)")
	cmd.Flags().StringVar(&req.LocalName, "local-name", "", "Dataset localName (required)")
	cmd.Flags().StringVar(&description, "description", "", "Dataset description")

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
			return deleteDataset(cmd.Context(), &req)
		},
		Example: cli.DefaultName() + " dataset get --project-name=yy --local-name=xx",
	}

	cmd.Flags().StringVar(&req.ProjectName, "project-name", "", "Project name (required)")
	cmd.Flags().StringVar(&req.LocalName, "local-name", "", "Dataset localName (required)")
	_ = cmd.MarkFlagRequired("local-name")
	_ = cmd.MarkFlagRequired("project-name")

	return cmd
}

func createDataset(ctx context.Context, req *trainv1.CreateDatasetRequest) error {
	resp, err := client.CreateDataset(ctx, req)
	if err != nil {
		return err
	}
	cli.PrintJSON(resp)

	return nil
}

func listDataset(ctx context.Context, req *trainv1.ListDatasetRequest) error {
	resp, err := client.ListDataset(ctx, req)
	if err != nil {
		return err
	}
	cli.PrintJSON(resp)

	return nil
}

func getDataset(ctx context.Context, req *trainv1.GetDatasetRequest) error {
	resp, err := client.GetDataset(ctx, req)
	if err != nil {
		return err
	}
	cli.PrintJSON(resp)

	return nil
}

func updateDataset(ctx context.Context, req *trainv1.UpdateDatasetRequest) error {
	err := client.UpdateDataset(ctx, req)
	if err != nil {
		return err
	}

	fmt.Println()
	return nil
}

func deleteDataset(ctx context.Context, req *trainv1.DeleteDatasetRequest) error {
	err := client.DeleteDataset(ctx, req)
	if err != nil {
		return err
	}

	fmt.Println()
	return nil
}

// Dataset metadata includes the project path separately from the HTTP body.
type datasetMetadata struct {
	trainv1.CreateDatasetRequest
	ProjectName string `json:"projectName"`
}

func applyDataset() {
	var input datasetMetadata
	cli.RegisterApplyHandler("Dataset", &input, func(args any) error {
		metadata := args.(*datasetMetadata)
		req := metadata.CreateDatasetRequest
		req.ProjectName = metadata.ProjectName
		initClient()
		return createDataset(context.Background(), &req)
	})
}
