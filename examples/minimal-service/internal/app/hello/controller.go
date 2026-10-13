package hello

import (
	"net/http"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

// Controller serves the minimal HTTP example.
// @title Minimal Service API
// @version 1.0
// @BasePath /
type Controller struct{}

type HelloResponse struct {
	Message string `json:"message" example:"hello"`
}

func NewController() *Controller { return &Controller{} }

func (c *Controller) RegisterRoutes(router *core.RouterGroup) { router.GET("/hello", c.hello) }

// hello returns the greeting.
// @Summary 获取问候语
// @Tags Hello
// @Produce json
// @Success 200 {object} HelloResponse
// @Router /hello [get]
func (c *Controller) hello(ctx *core.Context) {
	ctx.JSON(http.StatusOK, HelloResponse{Message: "hello"})
}
