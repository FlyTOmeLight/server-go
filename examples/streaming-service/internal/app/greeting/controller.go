package greeting

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/constants"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/logger"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/FlyTOmeLight/server-go/stream"
)

type Controller struct{ service *Service }

func NewController(service *Service) *Controller { return &Controller{service: service} }

func (s *Controller) RegisterRoutes(router *core.RouterGroup) error {
	if s.service.generate == nil {
		return errors.New("greeting generator is required")
	}
	router.GET("/stream", s.greet)
	return nil
}

func (s *Controller) greet(c *core.Context) {
	var req struct {
		Name string `form:"name" binding:"max=64"`
	}
	if err := binder.Bind(c, &req).Query().Error(); err != nil {
		_ = c.Error(err)
		return
	}
	if req.Name == "" {
		req.Name = "world"
	}
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	events, err := stream.NewSSE(c.Writer, 5*time.Second)
	if err != nil {
		_ = c.Error(err)
		return
	}
	sendFailed := false
	err = s.service.Greet(c, req.Name, func(text string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := events.Send("message", gin.H{"text": text})
		sendFailed = err != nil
		return err
	})
	if err != nil {
		if sendFailed || c.Request.Context().Err() != nil {
			return
		}
		logger.FromContext(ctx).Errorf(ctx, "greeting stream: %v", err)
		detail := serrors.New(err)
		detail.RequestID = c.GetHeader(constants.HeaderRequestID)
		detail.SetStatusCode(c.GetInt("server.AppCode"))
		// After streaming starts, failure is an SSE event, not a JSON HTTP response.
		_ = events.Send("error", detail)
		return
	}
	_ = events.Send("done", gin.H{})
}
