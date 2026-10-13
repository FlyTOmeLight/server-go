package dataset

import (
	"github.com/FlyTOmeLight/server-go"
	"github.com/FlyTOmeLight/server-go/idempotence"
	"github.com/FlyTOmeLight/server-go/middleware"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"

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

// Create a dataset
// @Summary Create a dataset
// @Schemes
// @Param request body trainv1.CreateDatasetRequest true "Request"
// @Param projectName path string true "Project Local Name"
// @Description Create a dataset
// @Tags dataset
// @Accept json
// @Produce json
// @Success 200 object trainv1.Dataset "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/projects/{projectName}/datasets [post]
func (c *Controller) createDataset(ctx *core.Context) {
	var req sdk.CreateDatasetRequest
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := c.service.CreateDataset(ctx, &req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	server.OK(ctx, resp)
}

// List datasets
// @Summary List datasets
// @Schemes
// @Param request query trainv1.ListDatasetRequest true "Request"
// @Param projectName path string true "Project Local Name"
// @Description List datasets
// @Tags dataset
// @Produce json
// @Success 200 object trainv1.ListDatasetResponse "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/projects/{projectName}/datasets [get]
func (c *Controller) listDataset(ctx *core.Context) {
	var req sdk.ListDatasetRequest
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := c.service.ListDataset(ctx, &req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	server.OK(ctx, resp)
}

// Get a dataset
// @Summary Get a dataset with latest artifact
// @Schemes
// @Param request query trainv1.GetDatasetRequest true "Request"
// @Param projectName path string true "Project Local Name"
// @Param localName path string true "Dataset Local Name"
// @Description Get a dataset
// @Tags dataset
// @Produce json
// @Success 200 object trainv1.GetDatasetResponse "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/projects/{projectName}/datasets/{localName} [get]
func (c *Controller) getDataset(ctx *core.Context) {
	var req sdk.GetDatasetRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := c.service.GetDataset(ctx, &req)
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	server.OK(ctx, resp)
}

// Update a dataset
// @Summary Update a dataset
// @Schemes
// @Param request body trainv1.UpdateDatasetRequest true "Request"
// @Param projectName path string true "Project Local Name"
// @Param localName path string true "Dataset Local Name"
// @Description Update a dataset
// @Tags dataset
// @Accept json
// @Success 200 "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/projects/{projectName}/datasets/{localName} [put]
func (c *Controller) updateDataset(ctx *core.Context) {
	var req sdk.UpdateDatasetRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		ctx.Error(err)
		return
	}

	if err := c.service.UpdateDataset(ctx, &req); err != nil {
		ctx.Error(err)
		return
	}

	server.OK(ctx, nil)
}

// Delete a dataset
// @Summary Delete a dataset, and it's all artifact
// @Schemes
// @Param request query trainv1.DeleteDatasetRequest true "Request"
// @Param projectName path string true "Project Local Name"
// @Param localName path string true "Dataset Local Name"
// @Description Delete a dataset
// @Tags dataset
// @Accept json
// @Success 200 "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/projects/{projectName}/datasets/{localName} [delete]
func (c *Controller) deleteDataset(ctx *core.Context) {
	var req sdk.DeleteDatasetRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	if err := c.service.DeleteDataset(ctx, &req); err != nil {
		ctx.Error(err)
		return
	}

	server.OK(ctx, nil)
}
