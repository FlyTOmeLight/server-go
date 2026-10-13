// Package lifecycle coordinates task completion before dependencies are released.
package lifecycle

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Stopper shares successful completion between owners that may retry cleanup.
// A failed attempt retains ownership and permits a later retry.
type Stopper struct {
	mu      sync.Mutex
	stop    func(context.Context) error
	stopped bool
}

func NewStopper(stop func(context.Context) error) *Stopper {
	return &Stopper{stop: stop}
}

func (s *Stopper) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return nil
	}
	if err := s.stop(ctx); err != nil {
		return err
	}
	s.stopped = true
	return nil
}

// Stop gives the first attempt its own budget. A deadline limits that attempt,
// not ownership: continue waiting without cancellation until the task confirms
// completion. The returned error retains a deadline even if the retry succeeds;
// stopped separately reports whether dependencies can now be released.
func Stop(ctx context.Context, timeout time.Duration, stop func(context.Context) error) (err error, stopped bool) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	remainingErr := stop(cleanupCtx)
	err = remainingErr
	if errors.Is(remainingErr, context.DeadlineExceeded) {
		remainingErr = stop(context.WithoutCancel(ctx))
		err = errors.Join(err, remainingErr)
	}
	return err, remainingErr == nil
}
