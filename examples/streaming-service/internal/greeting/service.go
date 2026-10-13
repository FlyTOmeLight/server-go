package greeting

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/constants"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/FlyTOmeLight/server-go/pkg/binder"
	"github.com/FlyTOmeLight/server-go/stream"
)

// Generate runs synchronously, observes ctx and returns when emit fails.
type Generate func(ctx context.Context, name string, emit func(string) error) error

type Service struct{ generate Generate }

var _ host.Service = (*Service)(nil)

func New(generate Generate) *Service { return &Service{generate: generate} }

func (s *Service) RegisterRoutes(router *gin.RouterGroup) error {
	if s.generate == nil {
		return errors.New("greeting generator is required")
	}
	router.GET("/stream", s.greet)
	return nil
}

func (s *Service) Start(context.Context) error { return nil }

func (s *Service) Stop(context.Context) error { return nil }

func (s *Service) greet(c *gin.Context) {
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	events, err := stream.NewSSE(c.Writer, 5*time.Second)
	if err != nil {
		_ = c.Error(err)
		return
	}
	sendFailed := false
	err = s.generate(ctx, req.Name, func(text string) error {
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

// GenerateGreeting is a local data source; replace it with a context-aware model call.
func GenerateGreeting(ctx context.Context, name string, emit func(string) error) error {
	for _, text := range []string{"hello", " ", name} {
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		if err := emit(text); err != nil {
			return err
		}
	}
	return nil
}
