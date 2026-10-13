package greeting

import (
	"context"
	"time"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

type Generate func(ctx context.Context, name string, emit func(string) error) error

type Service struct{ generate Generate }

func New(generate Generate) *Service { return &Service{generate: generate} }

func (s *Service) Greet(ctx *core.Context, name string, emit func(string) error) error {
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 30*time.Second)
	defer cancel()
	return s.generate(requestCtx, name, emit)
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
