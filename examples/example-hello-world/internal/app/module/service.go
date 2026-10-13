package module

import (
	"time"

	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/db"
	core "github.com/FlyTOmeLight/server-go/pkg"

	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/internal/table"
	sdk "github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

type Service struct{ database *db.DB }

func NewModuleService(database *db.DB) *Service { return &Service{database: database} }

func (s *Service) CreateModule(
	ctx *core.Context,
	req *sdk.CreateModuleRequest,
) (*sdk.CreateModuleResponse, error) {
	userID := ctx.GetString(constants.KeyUserID)
	now := db.UTCTime{Time: time.Now().UTC()}
	data := table.ModuleData{
		ID: core.NewID("ex"), Name: req.Name, Description: &req.Description,
		UserID: userID, ProjectID: req.ProjectID, CreatedAt: now, UpdatedAt: now,
	}
	if err := table.CreateModule(ctx.Request.Context(), s.database, &data); err != nil {
		return nil, err
	}
	result := moduleResponse(&data)
	return &result, nil
}

func (s *Service) ListModule(ctx *core.Context, req *sdk.ListModuleRequest) (*sdk.ListModuleResponse, error) {
	filter := table.ModuleFilter{ProjectID: req.ProjectID, Filter: req.Filter}
	count, err := table.CountModule(ctx.Request.Context(), s.database, filter)
	if err != nil {
		return nil, err
	}
	results := make([]sdk.Module, 0)
	if count > 0 {
		list, err := table.ListModule(ctx.Request.Context(), s.database, filter, req.PagingRequest)
		if err != nil {
			return nil, err
		}
		for _, item := range list {
			results = append(results, moduleResponse(item))
		}
	}
	return &sdk.ListModuleResponse{TotalCount: count, Results: results}, nil
}

func (s *Service) GetModule(ctx *core.Context, req *sdk.GetModuleRequest) (*sdk.GetModuleResponse, error) {
	data, err := table.GetModule(ctx.Request.Context(), s.database, table.ModuleFilter{ID: req.ID})
	if err != nil {
		return nil, err
	}
	result := moduleResponse(data)
	return &result, nil
}

func (s *Service) UpdateModule(ctx *core.Context, req *sdk.UpdateModuleRequest) error {
	if _, err := table.GetModule(ctx.Request.Context(), s.database, table.ModuleFilter{ID: req.ID}); err != nil {
		return err
	}
	data := table.ModuleData{
		ID:          req.ID,
		Description: req.Description,
		UpdatedAt:   db.UTCTime{Time: time.Now().UTC()},
	}
	return table.UpdateModule(ctx.Request.Context(), s.database, &data)
}

func (s *Service) DeleteModule(ctx *core.Context, req *sdk.DeleteModuleRequest) error {
	filter := table.ModuleFilter{ID: req.ID}
	if _, err := table.GetModule(ctx.Request.Context(), s.database, filter); err != nil {
		return err
	}
	return table.DeleteModule(ctx.Request.Context(), s.database, filter)
}

func moduleResponse(data *table.ModuleData) sdk.Module {
	description := ""
	if data.Description != nil {
		description = *data.Description
	}
	return sdk.Module{
		ID: data.ID, Name: data.Name, Description: description, UserID: data.UserID,
		ProjectID: data.ProjectID, CreatedAt: data.CreatedAt.Time, UpdatedAt: data.UpdatedAt.Time,
	}
}
