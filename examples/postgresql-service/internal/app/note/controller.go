package note

import (
	"net/http"

	"github.com/creasty/defaults"

	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/examples/postgresql-service/sdk/go/notev1"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
)

// Controller exposes the PostgreSQL note example.
// @title PostgreSQL Notes OpenAPI
// @version 1.0
// @description Note CRUD with PostgreSQL persistence and typed request validation.
// @BasePath /
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

// create creates a note.
// @Summary Create a note
// @Description Name is required and at most 64 characters; description is at most 1024 characters and priority is 0..10. Omitted priority defaults to 2 and enabled to true. Explicit null for either field is invalid; reviewed may be null.
// @Tags Notes
// @Accept json
// @Produce json
// @Param request body notev1.CreateNoteRequest true "Note payload; name is required"
// @Success 201 {object} notev1.Note "Created note"
// @Failure 400 {object} serrors.Error "Invalid payload"
// @Failure 413 {object} serrors.Error "JSON body exceeds 1 MiB"
// @Failure 500 {object} serrors.Error "Storage or initialization failure"
// @Router /v1/notes [post]
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

// list returns a page of notes.
// @Summary List notes
// @Tags Notes
// @Produce json
// @Param pageNo query int false "Page number" default(1) minimum(1) maximum(1000000)
// @Param pageSize query int false "Notes per page" default(20) minimum(1) maximum(100)
// @Success 200 {object} notev1.ListNotesResponse "Notes and total count"
// @Failure 400 {object} serrors.Error "Invalid pagination"
// @Failure 500 {object} serrors.Error "Storage or initialization failure"
// @Router /v1/notes [get]
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

// get returns a note by ID.
// @Summary Get a note
// @Tags Notes
// @Produce json
// @Param id path string true "Note UUID" format(uuid)
// @Success 200 {object} notev1.Note "Stored note"
// @Failure 400 {object} serrors.Error "Invalid note ID"
// @Failure 404 {object} serrors.Error "Note not found"
// @Failure 500 {object} serrors.Error "Storage failure"
// @Router /v1/notes/{id} [get]
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

// update replaces a note's mutable fields.
// @Summary Replace a note
// @Description PUT replaces name, description, priority, enabled and reviewed. Name is required and at most 64 characters; description is at most 1024 characters and priority is 0..10. Omitted priority defaults to 2, enabled to true, and reviewed to null; explicit null for priority or enabled is invalid.
// @Tags Notes
// @Accept json
// @Produce json
// @Param id path string true "Note UUID" format(uuid)
// @Param request body notev1.UpdateNoteRequest true "Complete replacement payload; ID comes from the path"
// @Success 200 {object} notev1.Note "Updated note"
// @Failure 400 {object} serrors.Error "Invalid ID or payload"
// @Failure 404 {object} serrors.Error "Note not found"
// @Failure 413 {object} serrors.Error "JSON body exceeds 1 MiB"
// @Failure 500 {object} serrors.Error "Storage or initialization failure"
// @Router /v1/notes/{id} [put]
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

// delete removes a note.
// @Summary Delete a note
// @Tags Notes
// @Produce json
// @Param id path string true "Note UUID" format(uuid)
// @Success 204 "Note deleted; response has no body"
// @Failure 400 {object} serrors.Error "Invalid note ID"
// @Failure 404 {object} serrors.Error "Note not found"
// @Failure 500 {object} serrors.Error "Storage failure"
// @Router /v1/notes/{id} [delete]
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
