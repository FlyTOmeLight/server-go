package dataset

import (
	"github.com/FlyTOmeLight/server-go"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/idempotence"
	"github.com/FlyTOmeLight/server-go/middleware"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/creasty/defaults"

	_ "github.com/FlyTOmeLight/server-go/examples/example-service1/doc/openapi"
	sdk "github.com/FlyTOmeLight/server-go/examples/example-service1/sdk/go/trainv1"
)

type Controller struct{ service *Service }

func NewController(service *Service) *Controller { return &Controller{service: service} }

// RegisterRoute
// @title Example service dataset open api
// @BasePath /
func (c *Controller) RegisterRoute(engine *core.RouterGroup, control *idempotence.Controller) {
	g := engine.Group("/v1/projects/:projectName/datasets")
	g.POST("", middleware.Idempotent(control, c.createDataset))
	g.GET("", middleware.Idempotent(control, c.listDataset))
	g.GET("/:localName", middleware.Idempotent(control, c.getDataset))
	g.PUT("/:localName", middleware.Idempotent(control, c.updateDataset))
	g.DELETE("/:localName", middleware.Idempotent(control, c.deleteDataset))
}

// createDataset 创建数据集。
// @Summary 创建数据集
// @Description 在指定项目下创建数据集。省略 fileFormat 时使用 File；成功返回数据集对象。clientToken 仅在已有鉴权身份时启用进程内幂等。
// @Tags dataset
// @Accept json
// @Produce json
// @Param projectName path string true "项目名称" maxlength(64)
// @Param clientToken query string false "可选幂等令牌，同一身份和操作下重复提交须保持请求一致"
// @Param request body trainv1.CreateDatasetRequest true "创建参数，projectName 来自路径"
// @Success 200 {object} trainv1.CreateDatasetResponse "数据集"
// @Failure 400 {object} serrors.Error "参数无效或请求体读取失败"
// @Failure 408 {object} serrors.Error "等待幂等请求时取消或超时"
// @Failure 409 {object} serrors.Error "数据库唯一约束冲突或幂等令牌对应的请求不同"
// @Failure 413 {object} serrors.Error "JSON 正文超过 1 MiB"
// @Failure 500 {object} serrors.Error "内部错误"
// @Failure 503 {object} serrors.Error "幂等处理不可用"
// @Router /v1/projects/{projectName}/datasets [post]
func (c *Controller) createDataset(ctx *core.Context) {
	var req sdk.CreateDatasetRequest
	if err := defaults.Set(&req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	resp, err := c.service.CreateDataset(ctx, &req)
	if err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	server.OK(ctx, resp)
}

// listDataset 分页查询数据集。
// @Summary 查询数据集列表
// @Description 按项目分页查询；filter 对数据集本地名称和描述进行包含匹配。0 值页码按 1 处理，0 值页大小按 20 处理。
// @Tags dataset
// @Produce json
// @Param projectName path string true "项目名称" maxlength(64)
// @Param pageNo query int false "页码" minimum(0) default(1)
// @Param pageSize query int false "每页条数" minimum(0) maximum(200) default(20)
// @Param order query string false "排序方向" Enums(asc,desc) default(desc)
// @Param orderBy query string false "排序字段；省略时使用 created_at，字段名不能包含空格"
// @Param filter query string false "本地名称或描述的包含匹配" maxlength(100)
// @Success 200 {object} trainv1.ListDatasetResponse "总数和当前页结果"
// @Failure 400 {object} serrors.Error "参数无效"
// @Failure 500 {object} serrors.Error "内部错误"
// @Router /v1/projects/{projectName}/datasets [get]
func (c *Controller) listDataset(ctx *core.Context) {
	var req sdk.ListDatasetRequest
	if err := defaults.Set(&req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	resp, err := c.service.ListDataset(ctx, &req)
	if err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	server.OK(ctx, resp)
}

// getDataset 查询单个数据集。
// @Summary 查询数据集
// @Description 按项目名称和数据集本地名称查询数据集。
// @Tags dataset
// @Produce json
// @Param projectName path string true "项目名称" maxlength(64)
// @Param localName path string true "数据集本地名称：首尾为ASCII字母或数字，中间可含下划线和连字符" minlength(1) maxlength(36)
// @Success 200 {object} trainv1.GetDatasetResponse "数据集"
// @Failure 400 {object} serrors.Error "参数无效"
// @Failure 404 {object} serrors.Error "数据集不存在"
// @Failure 500 {object} serrors.Error "内部错误"
// @Router /v1/projects/{projectName}/datasets/{localName} [get]
func (c *Controller) getDataset(ctx *core.Context) {
	var req sdk.GetDatasetRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	resp, err := c.service.GetDataset(ctx, &req)
	if err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	server.OK(ctx, resp)
}

// updateDataset 修改数据集描述。
// @Summary 修改数据集描述
// @Description 仅修改 description。省略该字段或传入 null 时保留原描述；成功返回空正文。未匹配到记录时也返回成功。
// @Tags dataset
// @Accept json
// @Produce json
// @Param projectName path string true "项目名称" maxlength(64)
// @Param localName path string true "数据集本地名称：首尾为ASCII字母或数字，中间可含下划线和连字符" minlength(1) maxlength(36)
// @Param request body trainv1.UpdateDatasetRequest false "更新参数，项目和本地名称来自路径；无正文时保留原描述"
// @Success 200 "成功，空正文"
// @Failure 400 {object} serrors.Error "参数无效或请求体读取失败"
// @Failure 413 {object} serrors.Error "JSON 正文超过 1 MiB"
// @Failure 500 {object} serrors.Error "内部错误"
// @Router /v1/projects/{projectName}/datasets/{localName} [put]
func (c *Controller) updateDataset(ctx *core.Context) {
	var req sdk.UpdateDatasetRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	if err := c.service.UpdateDataset(ctx, &req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	server.OK(ctx, nil)
}

// deleteDataset 删除数据集。
// @Summary 删除数据集
// @Description 按项目和本地名称软删除数据集，成功返回空正文。未匹配到记录时也返回成功。
// @Tags dataset
// @Produce json
// @Param projectName path string true "项目名称" maxlength(64)
// @Param localName path string true "数据集本地名称：首尾为ASCII字母或数字，中间可含下划线和连字符" minlength(1) maxlength(36)
// @Success 200 "成功，空正文"
// @Failure 400 {object} serrors.Error "参数无效"
// @Failure 500 {object} serrors.Error "内部错误"
// @Router /v1/projects/{projectName}/datasets/{localName} [delete]
func (c *Controller) deleteDataset(ctx *core.Context) {
	var req sdk.DeleteDatasetRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	if err := c.service.DeleteDataset(ctx, &req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}

	server.OK(ctx, nil)
}
