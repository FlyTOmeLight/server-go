package module

import (
	"github.com/FlyTOmeLight/server-go/constants"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/jinzhu/copier"

	"github.com/FlyTOmeLight/server-go/examples/example-hello-world/internal/table"
	sdk "github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

func CreateModule(ctx *core.Context, req *sdk.CreateModuleRequest) (*sdk.GetModuleResponse, error) {
	var data table.ModuleData
	_ = copier.Copy(&data, req)
	data.ID = core.NewID("ex")
	data.UserID = ctx.GetString(constants.KeyUserID)

	err := table.CreateModule(&data, nil)
	if err != nil {
		logger.Errorf(ctx, "CreateModuleErrorDB req: %#v, err: %v", req, err)
		return nil, err
	}

	var resp sdk.CreateModuleResponse
	_ = copier.Copy(&resp, data)
	return &resp, nil
}

func ListModule(ctx *core.Context, req *sdk.ListModuleRequest) (*sdk.ListModuleResponse, error) {
	var filter table.ModuleFilter
	_ = copier.Copy(&filter, req)

	count, err := table.CountModule(filter)
	if err != nil {
		logger.Errorf(ctx, "ListModuleErrorDBCount req: %#v, err: %v", req, err)
		return nil, err
	}
	if count == 0 {
		return &sdk.ListModuleResponse{}, nil
	}

	list, err := table.ListModule(filter, req.PagingRequest)
	if err != nil {
		logger.Errorf(ctx, "ListModuleErrorDBQuery req: %#v, err: %v", req, err)
		return nil, err
	}

	results := make([]sdk.Module, 0)
	_ = copier.Copy(&results, list)

	return &sdk.ListModuleResponse{TotalCount: count, Results: results}, nil
}

func GetModule(ctx *core.Context, req *sdk.GetModuleRequest) (*sdk.GetModuleResponse, error) {
	filter := table.ModuleFilter{ID: req.ID}
	data, err := table.GetModule(filter)
	if err != nil {
		logger.Errorf(ctx, "GetModuleErrorDBQuery req: %#v, err: %v", req, err)
		return nil, err
	}

	var resp sdk.GetModuleResponse
	_ = copier.Copy(&resp, data)
	return &resp, nil
}

func UpdateModule(ctx *core.Context, req *sdk.UpdateModuleRequest) error {
	filter := table.ModuleFilter{ID: req.ID}
	_, err := table.GetModule(filter)
	if err != nil {
		logger.Errorf(ctx, "UpdateModuleErrorDBQuery req: %#v, err: %v", req, err)
		return err
	}

	data := table.ModuleData{
		BaseData: db.BaseData{
			ID:          req.ID,
			Description: req.Description,
		},
	}

	err = table.UpdateModule(&data, nil)
	if err != nil {
		logger.Errorf(ctx, "UpdateModuleErrorDBUpdate req: %#v, data: %v, err: %v", req, data, err)
		return err
	}

	return nil
}

func DeleteModule(ctx *core.Context, req *sdk.DeleteModuleRequest) error {
	filter := table.ModuleFilter{ID: req.ID}
	_, err := table.GetModule(filter)
	if err != nil {
		logger.Errorf(ctx, "DeleteModuleErrorDBQuery req: %#v, err: %v", req, err)
		return err
	}

	err = table.DeleteModule(filter, nil)
	if err != nil {
		logger.Errorf(ctx, "DeleteModuleErrorDBDelete req: %#v, err: %v", req, err)
		return err
	}

	return nil
}
