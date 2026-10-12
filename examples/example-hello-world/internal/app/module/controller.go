package module

import (
	"github.com/FlyTOmeLight/server-go"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"

	sdk "github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

// @title HelloWorldService Module OpenAPI
// @BasePath /helloworld/internal
func RegisterRoute(engine *core.RouterGroup) {
	g := engine.Group("/v1")
	g.POST("/CreateModule", createModule)
	g.POST("/ListModule", listModule)
	g.POST("/GetModule", getModule)
	g.POST("/UpdateModule", updateModule)
	g.POST("/DeleteModule", deleteModule)
}

// Create Module
// @Summary 创建示例模块
// @Schemes
// @Param request body helloworldv1.CreateModuleRequest true "Request"
// @Description Create Module
// @Tags Module
// @Accept json
// @Produce json
// @Success 200  {object}  helloworldv1.CreateModuleResponse "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/CreateModule [post]
func createModule(ctx *core.Context) {
	var req sdk.CreateModuleRequest
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := CreateModule(ctx, &req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	server.OK(ctx, resp)
}

// List Module
// @Summary 获取示例模块列表
// @Schemes
// @Param request body helloworldv1.ListModuleRequest true "Request"
// @Description List Modules
// @Tags Module
// @Produce json
// @Success 200 object helloworldv1.ListModuleResponse "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/ListModule [post]
func listModule(ctx *core.Context) {
	var req sdk.ListModuleRequest
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := ListModule(ctx, &req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	server.OK(ctx, resp)
}

// Get Module
// @Summary 获取示例模块详细信息
// @Schemes
// @Param request body helloworldv1.GetModuleRequest true "Request"
// @Description Get Module
// @Tags Module
// @Produce json
// @Success 200 object helloworldv1.GetModuleResponse "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/GetModule [post]
func getModule(ctx *core.Context) {
	var req sdk.GetModuleRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := GetModule(ctx, &req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	server.OK(ctx, resp)
}

// Update Module
// @Summary 更新示例模块
// @Schemes
// @Param request body helloworldv1.UpdateModuleRequest true "Request"
// @Description Update Module
// @Tags Module
// @Accept json
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/UpdateModule [post]
func updateModule(ctx *core.Context) {
	var req sdk.UpdateModuleRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	if err := UpdateModule(ctx, &req); err != nil {
		_ = ctx.Error(err)
		return
	}

	server.OK(ctx, nil)
}

// Delete Module
// @Summary 删除示例模块
// @Schemes
// @Param request body helloworldv1.DeleteModuleRequest true "Request"
// @Description Delete Module
// @Tags Module
// @Success 200 "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/DeleteModule [post]
func deleteModule(ctx *core.Context) {
	var req sdk.DeleteModuleRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	if err := DeleteModule(ctx, &req); err != nil {
		_ = ctx.Error(err)
		return
	}

	server.OK(ctx, nil)
}
