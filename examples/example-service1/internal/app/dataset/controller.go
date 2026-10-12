package dataset

import (
	"github.com/FlyTOmeLight/server-go"
	_ "github.com/FlyTOmeLight/server-go/examples/example-service1/doc/openapi"
	sdk "github.com/FlyTOmeLight/server-go/examples/example-service1/sdk/go/trainv1"
	"github.com/FlyTOmeLight/server-go/idempotence"
	"github.com/FlyTOmeLight/server-go/middleware"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/gin-gonic/gin"
)

// RegisterRoute
// @title Example service dataset open api
// @BasePath /example-service1/v1
func (s *Service) RegisterRoute(engine *gin.RouterGroup, control *idempotence.Controller) {
	g := engine.Group("/v1/projects/:projectName/datasets")
	g.POST("", middleware.Idempotent(control, s.createDataset))
	g.GET("", middleware.Idempotent(control, s.listDataset))
	g.GET("/:localName", middleware.Idempotent(control, s.getDataset))
	g.PUT("/:localName", middleware.Idempotent(control, s.updateDataset))
	g.DELETE("/:localName", middleware.Idempotent(control, s.deleteDataset))
}

// Create a dataset
// @Summary Create a dataset
// @Schemes
// @Param request body trainv1.CreateDatasetRequest true "Request"
// @Param workspaceID path string true "Workspace id"
// @Param projectName path string true "Project Local Name"
// @Description Create a dataset
// @Tags dataset
// @Accept json
// @Produce json
// @Success 200 object trainv1.Dataset "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/workspaces/:workspaceID/projects/:projectName/datasets [post]
func (s *Service) createDataset(ctx *core.Context) {
	var req sdk.CreateDatasetRequest
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := s.CreateDataset(ctx.Request.Context(), &req)
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
// @Param workspaceID path string true "Workspace id"
// @Param projectName path string true "Project Local Name"
// @Description List datasets
// @Tags dataset
// @Produce json
// @Success 200 object sdk.ListDatasetResponse "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/workspaces/:workspaceID/projects/:projectName/datasets [get]
func (s *Service) listDataset(ctx *core.Context) {
	var req sdk.ListDatasetRequest
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := s.ListDataset(ctx.Request.Context(), &req)
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
// @Param workspaceID path string true "Workspace id"
// @Param projectName path string true "Project Local Name"
// @Param localName path string true "Dataset Local Name"
// @Description Get a dataset
// @Tags dataset
// @Produce json
// @Success 200 object trainv1.GetDatasetResponse "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/workspaces/:workspaceID/projects/:projectName/datasets/:localName [get]
func (s *Service) getDataset(ctx *core.Context) {
	var req sdk.GetDatasetRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	resp, err := s.GetDataset(ctx.Request.Context(), &req)
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
// @Param workspaceID path string true "Workspace id"
// @Param projectName path string true "Project Local Name"
// @Param localName path string true "Dataset Local Name"
// @Description Update a project
// @Tags project
// @Accept json
// @Success 200 "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/workspaces/:workspaceID/projects/:projectName/datasets/:localName [put]
func (s *Service) updateDataset(ctx *core.Context) {
	var req sdk.UpdateDatasetRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		ctx.Error(err)
		return
	}

	if err := s.UpdateDataset(ctx.Request.Context(), &req); err != nil {
		ctx.Error(err)
		return
	}

	server.OK(ctx, nil)
}

// Delete a dataset
// @Summary Delete a dataset, and it's all artifact
// @Schemes
// @Param request query trainv1.DeleteDatasetRequest true "Request"
// @Param workspaceID path string true "Workspace id"
// @Param projectName path string true "Project Local Name"
// @Param localName path string true "Dataset Local Name"
// @Description Delete a dataset
// @Tags dataset
// @Accept json
// @Success 200 "OK"
// @Failure 400 {string} string "Invalid Params"
// @Failure 500 {string} string "Internal Error"
// @Router /v1/workspaces/:workspaceID/projects/:projectName/datasets/:localName [delete]
func (s *Service) deleteDataset(ctx *core.Context) {
	var req sdk.DeleteDatasetRequest

	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		_ = ctx.Error(err)
		return
	}

	if err := s.DeleteDataset(ctx.Request.Context(), &req); err != nil {
		ctx.Error(err)
		return
	}

	server.OK(ctx, nil)
}
