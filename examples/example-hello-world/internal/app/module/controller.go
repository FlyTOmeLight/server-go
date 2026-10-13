package module

import (
	"github.com/FlyTOmeLight/server-go"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/idempotence"
	"github.com/FlyTOmeLight/server-go/middleware"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/creasty/defaults"

	sdk "github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

// Controller
// @title HelloWorldService Module OpenAPI
// @BasePath /helloworld/internal
type Controller struct{ service *Service }

func NewModuleController(database *db.DB) *Controller {
	return &Controller{service: NewModuleService(database)}
}

func (c *Controller) RegisterRoutes(engine *core.RouterGroup, cache *idempotence.Controller) {
	g := engine.Group("/v1")
	g.POST("/CreateModule", middleware.Idempotent(cache, c.createModule))
	g.POST("/ListModule", middleware.Idempotent(cache, c.listModule))
	g.POST("/GetModule", middleware.Idempotent(cache, c.getModule))
	g.POST("/UpdateModule", middleware.Idempotent(cache, c.updateModule))
	g.POST("/DeleteModule", middleware.Idempotent(cache, c.deleteModule))
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
func (c *Controller) createModule(ctx *core.Context) {
	var req sdk.CreateModuleRequest
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := c.service.CreateModule(ctx, &req)
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
func (c *Controller) listModule(ctx *core.Context) {
	var req sdk.ListModuleRequest
	if err := defaults.Set(&req); err != nil {
		_ = ctx.Error(err)
		return
	}
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := c.service.ListModule(ctx, &req)
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
func (c *Controller) getModule(ctx *core.Context) {
	var req sdk.GetModuleRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := c.service.GetModule(ctx, &req)
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
func (c *Controller) updateModule(ctx *core.Context) {
	var req sdk.UpdateModuleRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	if err := c.service.UpdateModule(ctx, &req); err != nil {
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
func (c *Controller) deleteModule(ctx *core.Context) {
	var req sdk.DeleteModuleRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	if err := c.service.DeleteModule(ctx, &req); err != nil {
		_ = ctx.Error(err)
		return
	}

	server.OK(ctx, nil)
}
