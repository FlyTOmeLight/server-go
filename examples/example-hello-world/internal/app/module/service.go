package module

import (
	"context"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/internal/table"
	sdk "github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"time"
)

type ModuleService struct{ database *db.DB }

func NewModuleService(database *db.DB) *ModuleService { return &ModuleService{database: database} }

func (s *ModuleService) CreateModule(ctx context.Context, userID string, req *sdk.CreateModuleRequest) (*sdk.CreateModuleResponse, error) {
	now := db.UTCTime{Time: time.Now().UTC()}
	data := table.ModuleData{BaseData: db.BaseData{
		ID: core.NewID("ex"), Name: req.Name, Description: &req.Description,
		UserID: userID, ProjectID: req.ProjectID, CreatedAt: now, UpdatedAt: now,
	}}
	if err := table.CreateModule(ctx, s.database, &data); err != nil {
		return nil, err
	}
	result := moduleResponse(&data)
	return &result, nil
}

func (s *ModuleService) ListModule(ctx context.Context, req *sdk.ListModuleRequest) (*sdk.ListModuleResponse, error) {
	filter := table.ModuleFilter{ProjectID: req.ProjectID, Filter: req.Filter}
	count, err := table.CountModule(ctx, s.database, filter)
	if err != nil {
		return nil, err
	}
	results := make([]sdk.Module, 0)
	if count > 0 {
		list, err := table.ListModule(ctx, s.database, filter, req.PagingRequest)
		if err != nil {
			return nil, err
		}
		for _, item := range list {
			results = append(results, moduleResponse(item))
		}
	}
	return &sdk.ListModuleResponse{TotalCount: count, Results: results}, nil
}

func (s *ModuleService) GetModule(ctx context.Context, req *sdk.GetModuleRequest) (*sdk.GetModuleResponse, error) {
	data, err := table.GetModule(ctx, s.database, table.ModuleFilter{ID: req.ID})
	if err != nil {
		return nil, err
	}
	result := moduleResponse(data)
	return &result, nil
}

func (s *ModuleService) UpdateModule(ctx context.Context, req *sdk.UpdateModuleRequest) error {
	if _, err := table.GetModule(ctx, s.database, table.ModuleFilter{ID: req.ID}); err != nil {
		return err
	}
	data := table.ModuleData{BaseData: db.BaseData{ID: req.ID, Description: req.Description, UpdatedAt: db.UTCTime{Time: time.Now().UTC()}}}
	return table.UpdateModule(ctx, s.database, &data)
}

func (s *ModuleService) DeleteModule(ctx context.Context, req *sdk.DeleteModuleRequest) error {
	filter := table.ModuleFilter{ID: req.ID}
	if _, err := table.GetModule(ctx, s.database, filter); err != nil {
		return err
	}
	return table.DeleteModule(ctx, s.database, filter)
}

func moduleResponse(data *table.ModuleData) sdk.Module {
	description := ""
	if data.Description != nil {
		description = *data.Description
	}
	return sdk.Module{ID: data.ID, Name: data.Name, Description: description, UserID: data.UserID,
		ProjectID: data.ProjectID, CreatedAt: data.CreatedAt.Time, UpdatedAt: data.UpdatedAt.Time}
}
