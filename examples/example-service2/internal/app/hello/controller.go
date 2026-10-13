package hello

import (
	"net/http"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

type Controller struct{}

func NewController() *Controller { return &Controller{} }

func (c *Controller) RegisterRoutes(router *core.RouterGroup) { router.GET("/service2/hello", c.hello) }

func (c *Controller) hello(ctx *core.Context) {
	ctx.JSON(http.StatusOK, struct {
		Message string `json:"message"`
	}{Message: "hello from service2"})
}
