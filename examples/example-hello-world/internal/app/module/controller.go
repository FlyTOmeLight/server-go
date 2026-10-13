package module

import (
	"github.com/FlyTOmeLight/server-go"
	"github.com/FlyTOmeLight/server-go/db"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/idempotence"
	"github.com/FlyTOmeLight/server-go/middleware"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/creasty/defaults"

	sdk "github.com/FlyTOmeLight/server-go/examples/example-hello-world/sdk/go/helloworldv1"
)

// Controller
// @title HelloWorldService OpenAPI
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

// CreateModule 创建示例模块。
// @Summary 创建示例模块
// @Description 创建模块并返回新建记录。JSON 请求体最大为 1 MiB。
// @ID CreateModule
// @Tags Module
// @Accept json
// @Produce json
// @Param request body helloworldv1.CreateModuleRequest true "JSON 请求"
// @Param clientToken query string false "可选幂等标识；有已验证身份时，同一路由及相同参数重放已成功响应"
// @Success 200 {object} helloworldv1.CreateModuleResponse "创建成功的模块"
// @Failure 400 {object} serrors.Error "JSON 或请求参数不符合契约"
// @Failure 408 {object} serrors.Error "等待幂等请求时取消或超时"
// @Failure 409 {object} serrors.Error "同一幂等标识的请求参数不同"
// @Failure 413 {object} serrors.Error "JSON 请求体超过 1 MiB"
// @Failure 500 {object} serrors.Error "内部错误"
// @Failure 503 {object} serrors.Error "幂等执行暂不可用"
// @Router /v1/CreateModule [post]
func (c *Controller) createModule(ctx *core.Context) {
	var req sdk.CreateModuleRequest
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	resp, err := c.service.CreateModule(ctx, &req)
	if err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	server.OK(ctx, resp)
}

// ListModule 获取示例模块列表。
// @Summary 获取示例模块列表
// @Description 按项目和关键词筛选模块并返回总数与当前页。默认 pageNo=1、pageSize=20，pageSize 最大为 200；query 分页参数覆盖请求体中的同名字段。
// @ID ListModule
// @Tags Module
// @Accept json
// @Produce json
// @Param request body helloworldv1.ListModuleRequest false "JSON 请求"
// @Param clientToken query string false "可选幂等标识；有已验证身份时，同一路由及相同参数重放已成功响应"
// @Param pageNo query int false "页码；0 使用默认页码" default(1) minimum(0)
// @Param pageSize query int false "每页条数；0 使用默认大小" default(20) minimum(0) maximum(200)
// @Param order query string false "排序方向" default(desc) Enums(asc,desc)
// @Param orderBy query string false "排序字段"
// @Success 200 {object} helloworldv1.ListModuleResponse "模块列表与总数"
// @Failure 400 {object} serrors.Error "JSON 或请求参数不符合契约"
// @Failure 408 {object} serrors.Error "等待幂等请求时取消或超时"
// @Failure 409 {object} serrors.Error "同一幂等标识的请求参数不同"
// @Failure 413 {object} serrors.Error "JSON 请求体超过 1 MiB"
// @Failure 500 {object} serrors.Error "内部错误"
// @Failure 503 {object} serrors.Error "幂等执行暂不可用"
// @Router /v1/ListModule [post]
func (c *Controller) listModule(ctx *core.Context) {
	var req sdk.ListModuleRequest
	if err := defaults.Set(&req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	resp, err := c.service.ListModule(ctx, &req)
	if err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	server.OK(ctx, resp)
}

// GetModule 获取示例模块详细信息。
// @Summary 获取示例模块详细信息
// @Description 按 ID 获取模块；不存在时返回 404。
// @ID GetModule
// @Tags Module
// @Accept json
// @Produce json
// @Param request body helloworldv1.GetModuleRequest true "JSON 请求"
// @Param clientToken query string false "可选幂等标识；有已验证身份时，同一路由及相同参数重放已成功响应"
// @Success 200 {object} helloworldv1.GetModuleResponse "模块详情"
// @Failure 400 {object} serrors.Error "JSON 或请求参数不符合契约"
// @Failure 404 {object} serrors.Error "模块不存在"
// @Failure 408 {object} serrors.Error "等待幂等请求时取消或超时"
// @Failure 409 {object} serrors.Error "同一幂等标识的请求参数不同"
// @Failure 413 {object} serrors.Error "JSON 请求体超过 1 MiB"
// @Failure 500 {object} serrors.Error "内部错误"
// @Failure 503 {object} serrors.Error "幂等执行暂不可用"
// @Router /v1/GetModule [post]
func (c *Controller) getModule(ctx *core.Context) {
	var req sdk.GetModuleRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	resp, err := c.service.GetModule(ctx, &req)
	if err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	server.OK(ctx, resp)
}

// UpdateModule 更新示例模块。
// @Summary 更新示例模块
// @Description 按 ID 更新模块描述。description 未传入或为 null 时保留原值，显式空字符串清空描述。成功返回 200，响应体为空。
// @ID UpdateModule
// @Tags Module
// @Accept json
// @Produce json
// @Param request body helloworldv1.UpdateModuleRequest true "JSON 请求"
// @Param clientToken query string false "可选幂等标识；有已验证身份时，同一路由及相同参数重放已成功响应"
// @Success 200 "操作完成，响应体为空"
// @Failure 400 {object} serrors.Error "JSON 或请求参数不符合契约"
// @Failure 404 {object} serrors.Error "模块不存在"
// @Failure 408 {object} serrors.Error "等待幂等请求时取消或超时"
// @Failure 409 {object} serrors.Error "同一幂等标识的请求参数不同"
// @Failure 413 {object} serrors.Error "JSON 请求体超过 1 MiB"
// @Failure 500 {object} serrors.Error "内部错误"
// @Failure 503 {object} serrors.Error "幂等执行暂不可用"
// @Router /v1/UpdateModule [post]
func (c *Controller) updateModule(ctx *core.Context) {
	var req sdk.UpdateModuleRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	if err := c.service.UpdateModule(ctx, &req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	server.OK(ctx, nil)
}

// DeleteModule 删除示例模块。
// @Summary 删除示例模块
// @Description 按 ID 删除模块；不存在时返回 404。成功返回 200，响应体为空。
// @ID DeleteModule
// @Tags Module
// @Accept json
// @Produce json
// @Param request body helloworldv1.DeleteModuleRequest true "JSON 请求"
// @Param clientToken query string false "可选幂等标识；有已验证身份时，同一路由及相同参数重放已成功响应"
// @Success 200 "操作完成，响应体为空"
// @Failure 400 {object} serrors.Error "JSON 或请求参数不符合契约"
// @Failure 404 {object} serrors.Error "模块不存在"
// @Failure 408 {object} serrors.Error "等待幂等请求时取消或超时"
// @Failure 409 {object} serrors.Error "同一幂等标识的请求参数不同"
// @Failure 413 {object} serrors.Error "JSON 请求体超过 1 MiB"
// @Failure 500 {object} serrors.Error "内部错误"
// @Failure 503 {object} serrors.Error "幂等执行暂不可用"
// @Router /v1/DeleteModule [post]
func (c *Controller) deleteModule(ctx *core.Context) {
	var req sdk.DeleteModuleRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	if err := c.service.DeleteModule(ctx, &req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	server.OK(ctx, nil)
}
