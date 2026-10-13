package note

import (
	"net/http"

	"github.com/creasty/defaults"

	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/examples/postgresql-service/sdk/go/notev1"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
)

type Controller struct{ service *Service }

func NewController(service *Service) *Controller { return &Controller{service: service} }

func (c *Controller) RegisterRoutes(router *core.RouterGroup) {
	notes := router.Group("/v1/notes")
	notes.POST("", c.create)
	notes.GET("", c.list)
	notes.GET("/:id", c.get)
	notes.PUT("/:id", c.update)
	notes.DELETE("/:id", c.delete)
}

func (c *Controller) create(ctx *core.Context) {
	var req notev1.CreateNoteRequest
	if err := defaults.Set(&req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	result, err := c.service.Create(ctx, &req)
	if err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, result)
}

func (c *Controller) list(ctx *core.Context) {
	var req notev1.ListNotesRequest
	if err := defaults.Set(&req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	result, err := c.service.List(ctx, &req)
	if err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) get(ctx *core.Context) {
	var req notev1.GetNoteRequest
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	result, err := c.service.Get(ctx, &req)
	if err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) update(ctx *core.Context) {
	var req notev1.UpdateNoteRequest
	if err := defaults.Set(&req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	result, err := c.service.Update(ctx, &req)
	if err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *Controller) delete(ctx *core.Context) {
	var req notev1.DeleteNoteRequest
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	if err := c.service.Delete(ctx, &req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
