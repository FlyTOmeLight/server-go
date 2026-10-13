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

// Controller exposes streamed greetings.
// @title Streaming Greeting OpenAPI
// @version 1.0
// @description Greetings delivered as server-sent events.
// @BasePath /
type Controller struct{ service *Service }

func NewController(service *Service) *Controller { return &Controller{service: service} }

func (s *Controller) RegisterRoutes(router *core.RouterGroup) error {
	if s.service.generate == nil {
		return errors.New("greeting generator is required")
	}
	router.GET("/stream", s.greet)
	return nil
}

// greet streams a greeting as SSE events.
// @Summary Stream a greeting
// @Description Success uses text/event-stream: message events contain JSON {"text":"fragment"}; done contains {}. Generator failures use an error event carrying the framework error object after streaming begins; disconnects end the stream. Before streaming begins, invalid query parameters or stream initialization failures return JSON errors with HTTP 400 or 500.
// @Tags Greetings
// @Produce text/event-stream,json
// @Param name query string false "Greeting recipient; an omitted or empty value uses world" default(world) maxLength(64)
// @Success 200 {string} string "SSE stream with message, done or error events"
// @Failure 400 {object} serrors.Error "Invalid name before streaming begins"
// @Failure 500 {object} serrors.Error "Stream initialization failed before streaming begins"
// @Router /stream [get]
func (s *Controller) greet(c *core.Context) {
	var req struct {
		Name string `form:"name" binding:"max=64"`
	}
	if err := binder.Bind(c, &req).Query().Error(); err != nil {
		serrors.AbortWithError(c, err)
		return
	}
	if req.Name == "" {
		req.Name = "world"
	}
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	events, err := stream.NewSSE(c.Writer, 5*time.Second)
	if err != nil {
		serrors.AbortWithError(c, err)
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
