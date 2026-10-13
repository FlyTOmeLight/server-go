package hello

import (
	"net/http"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

// Controller serves the composable HTTP example.
// @title Example Service 2 API
// @version 1.0
// @BasePath /
type Controller struct{}

type HelloResponse struct {
	Message string `json:"message" example:"hello from service2"`
}

func NewController() *Controller { return &Controller{} }

func (c *Controller) RegisterRoutes(router *core.RouterGroup) { router.GET("/service2/hello", c.hello) }

// hello returns the greeting.
// @Summary 获取服务问候语
// @Tags Hello
// @Produce json
// @Success 200 {object} HelloResponse
// @Router /service2/hello [get]
func (c *Controller) hello(ctx *core.Context) {
	ctx.JSON(http.StatusOK, HelloResponse{Message: "hello from service2"})
}
