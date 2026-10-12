package dataset

import (
	"github.com/jinzhu/copier"

	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/examples/example-service1/internal/table"
	sdk "github.com/FlyTOmeLight/server-go/examples/example-service1/sdk/go/trainv1"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

func CreateDataset(ctx *core.Context, req *sdk.CreateDatasetRequest) (*sdk.GetDatasetResponse, error) {
	if req.LocalName == "" {
		return nil, errors.NewInvalidParameter("localName is missing")
	}

	data := &table.DatasetData{
		ID:          core.NewID("ds"),
		Name:        req.GetName(),
		ProjectName: req.ProjectName,
		LocalName:   req.LocalName,
		Description: &req.Description,
	}

	err := table.CreateDataset(data)
	if err != nil {
		logger.Errorf(ctx, "CreateDatasetError: %v", err)
		return nil, err
	}

	var resp sdk.CreateDatasetResponse
	_ = copier.Copy(&resp, data)
	return &resp, nil
}

func ListDataset(ctx *core.Context, req *sdk.ListDatasetRequest) (*sdk.ListDatasetResponse, error) {
	filters := table.DatasetFilter{
		ProjectName: req.ProjectName,
	}

	resp := sdk.ListDatasetResponse{Results: make([]sdk.Dataset, 0)}
	var err error

	resp.TotalCount, err = table.CountDataset(filters)
	if err != nil {
		logger.Errorf(ctx, "ListDatasetErrorDBCount: req: %#v, err: %v", req, err)
		return nil, err
	}
	if resp.TotalCount == 0 {
		return &resp, nil
	}

	data, listErr := table.ListDataset(filters, req.PagingRequest)
	if listErr != nil {
		logger.Errorf(ctx, "ListDatasetErrorDBQuery: req: %#v, err: %v", req, err)
		return nil, err
	}

	_ = copier.Copy(&resp.Results, data)
	return &resp, nil
}

func GetDataset(ctx *core.Context, req *sdk.GetDatasetRequest) (*sdk.GetDatasetResponse, error) {
	data, err := table.GetDataset(req.GetName())
	if err != nil {
		logger.Errorf(ctx, "GetDatasetErrorDBQuery: req: %#v, err: %v", req, err)
		return nil, err
	}

	var resp sdk.GetDatasetResponse
	_ = copier.Copy(&resp, data)
	return &resp, nil
}

func UpdateDataset(ctx *core.Context, req *sdk.UpdateDatasetRequest) error {
	data := table.DatasetData{Name: req.GetName()}
	data.Description = req.Description

	err := table.UpdateDataset(&data)
	if err != nil {
		logger.Errorf(ctx, "UpdateDatasetErrorDBUpdate: req: %#v, data: %v, err: %v", req, data, err)
		return err
	}

	return nil
}

func DeleteDataset(ctx *core.Context, req *sdk.DeleteDatasetRequest) error {
	err := table.DeleteDataset(req.GetName())
	if err != nil {
		logger.Errorf(ctx, "DeleteDatasetErrorDBDelete: req: %#v, err: %v", req, err)
		return err
	}

	return nil
}
