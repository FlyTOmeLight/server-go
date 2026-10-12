package dataset

import (
	"context"
	"github.com/jinzhu/copier"

	"github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/examples/example-service1/internal/table"
	sdk "github.com/FlyTOmeLight/server-go/examples/example-service1/sdk/go/trainv1"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type Service struct{ store *table.Store }

func New(store *table.Store) *Service { return &Service{store: store} }

func (s *Service) CreateDataset(ctx context.Context, req *sdk.CreateDatasetRequest) (*sdk.GetDatasetResponse, error) {
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

	err := s.store.CreateDataset(ctx, data)
	if err != nil {
		logger.Errorf(ctx, "CreateDatasetError: %v", err)
		return nil, err
	}

	var resp sdk.CreateDatasetResponse
	_ = copier.Copy(&resp, data)
	return &resp, nil
}

func (s *Service) ListDataset(ctx context.Context, req *sdk.ListDatasetRequest) (*sdk.ListDatasetResponse, error) {
	filters := table.DatasetFilter{
		ProjectName: req.ProjectName,
	}

	resp := sdk.ListDatasetResponse{Results: make([]sdk.Dataset, 0)}
	var err error

	resp.TotalCount, err = s.store.CountDataset(ctx, filters)
	if err != nil {
		logger.Errorf(ctx, "ListDatasetErrorDBCount: req: %#v, err: %v", req, err)
		return nil, err
	}
	if resp.TotalCount == 0 {
		return &resp, nil
	}

	data, listErr := s.store.ListDataset(ctx, filters, req.PagingRequest)
	if listErr != nil {
		logger.Errorf(ctx, "ListDatasetErrorDBQuery: req: %#v, err: %v", req, err)
		return nil, err
	}

	_ = copier.Copy(&resp.Results, data)
	return &resp, nil
}

func (s *Service) GetDataset(ctx context.Context, req *sdk.GetDatasetRequest) (*sdk.GetDatasetResponse, error) {
	data, err := s.store.GetDataset(ctx, req.GetName())
	if err != nil {
		logger.Errorf(ctx, "GetDatasetErrorDBQuery: req: %#v, err: %v", req, err)
		return nil, err
	}

	var resp sdk.GetDatasetResponse
	_ = copier.Copy(&resp, data)
	return &resp, nil
}

func (s *Service) UpdateDataset(ctx context.Context, req *sdk.UpdateDatasetRequest) error {
	data := table.DatasetData{Name: req.GetName()}
	data.Description = req.Description

	err := s.store.UpdateDataset(ctx, &data)
	if err != nil {
		logger.Errorf(ctx, "UpdateDatasetErrorDBUpdate: req: %#v, data: %v, err: %v", req, data, err)
		return err
	}

	return nil
}

func (s *Service) DeleteDataset(ctx context.Context, req *sdk.DeleteDatasetRequest) error {
	err := s.store.DeleteDataset(ctx, req.GetName())
	if err != nil {
		logger.Errorf(ctx, "DeleteDatasetErrorDBDelete: req: %#v, err: %v", req, err)
		return err
	}

	return nil
}
