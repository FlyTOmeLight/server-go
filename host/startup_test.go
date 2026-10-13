package host

import (
	"context"
	"errors"
	"testing"
	"time"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

func TestStartupRespondsToCancellationAndDeadline(t *testing.T) {
	for _, tc := range []struct {
		name    string
		timeout time.Duration
		cancel  bool
		want    error
	}{
		{name: "process cancellation", timeout: time.Second, cancel: true, want: context.Canceled},
		{name: "startup deadline", timeout: 20 * time.Millisecond, want: context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entered, rescue := make(chan struct{}), make(chan struct{})
			service := &testService{
				handle: func(c *core.Context) { c.String(200, "ok") },
				start: func(ctx context.Context) error {
					close(entered)
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-rescue:
						return errors.New("test released blocked startup")
					}
				},
			}
			h, _ := newTestHost(t, service)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- h.RunWithStartupTimeout(ctx, tc.timeout) }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				close(rescue)
				t.Fatal("Start was not called")
			}
			if tc.cancel {
				cancel()
			}
			select {
			case err := <-done:
				if !errors.Is(err, tc.want) {
					t.Fatalf("startup error = %v, want %v", err, tc.want)
				}
			case <-time.After(3 * time.Second):
				close(rescue)
				<-done
				t.Fatal("Start did not receive cancellation")
			}
			if h.ready.Load() || service.stops.Load() != 1 {
				t.Fatal("failed startup was ready or was not cleaned up")
			}
		})
	}
}

func TestStartupDeadlineDoesNotCancelRunningWorkers(t *testing.T) {
	const startupTimeout = 100 * time.Millisecond
	workerContext := make(chan context.Context, 1)
	service := &testService{
		handle: func(c *core.Context) { c.String(200, "ok") },
		start:  func(ctx context.Context) error { workerContext <- ctx; return nil },
	}
	h, _ := newTestHost(t, service)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.RunWithStartupTimeout(ctx, startupTimeout) }()
	var workerCtx context.Context
	select {
	case workerCtx = <-workerContext:
	case <-time.After(3 * time.Second):
		t.Fatal("worker was not started")
	}
	// Cross the actual startup deadline to verify it no longer owns this worker.
	select {
	case err := <-done:
		t.Fatalf("startup deadline ended the running host: %v", err)
	case <-time.After(2 * startupTimeout):
	}
	if workerCtx.Err() != nil {
		t.Fatalf("running worker canceled by startup budget: %v", workerCtx.Err())
	}
	cancel()
	waitStopped(t, done)
	if workerCtx.Err() == nil {
		t.Fatal("worker was not canceled on shutdown")
	}
}
