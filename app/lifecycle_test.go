package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

type lifecycleService struct {
	start func(context.Context) error
	stop  func(context.Context) error
}

func (s *lifecycleService) RegisterRoutes(*core.RouterGroup) error { return nil }

func (s *lifecycleService) Start(ctx context.Context) error { return s.start(ctx) }

func (s *lifecycleService) Stop(ctx context.Context) error { return s.stop(ctx) }

type lifecycleModule struct{ closed atomic.Bool }

func (m *lifecycleModule) Initialize(context.Context) error { return nil }

func (m *lifecycleModule) Close(context.Context) error { m.closed.Store(true); return nil }

func TestRunJoinsWorkerAfterStopDeadlineBeforeClosingDependency(t *testing.T) {
	options := testOptions(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	module := &lifecycleModule{}
	workerDone, release, retry := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var stops atomic.Int32
	service := &lifecycleService{
		start: func(ctx context.Context) error {
			go func() {
				<-ctx.Done()
				<-release
				close(workerDone)
			}()
			cancel()
			return nil
		},
		stop: func(ctx context.Context) error {
			if stops.Add(1) == 1 {
				return context.DeadlineExceeded
			}
			close(retry)
			select {
			case <-workerDone:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, options, func(ctx context.Context, runtime *Runtime, _ *struct{}) error {
			if err := runtime.Initialize(ctx, module); err != nil {
				return err
			}
			return runtime.AddService(service)
		})
	}()
	select {
	case <-retry:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("stop did not retry waiting for the worker")
	}
	premature := module.closed.Load()
	select {
	case err := <-done:
		close(release)
		t.Fatalf("Run returned before worker completion: %v", err)
	default:
	}
	close(release)
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("lost stop deadline: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not join the worker")
	}
	if premature || !module.closed.Load() || stops.Load() != 2 {
		t.Fatalf(
			"dependency release: premature=%v closed=%v stops=%d",
			premature,
			module.closed.Load(),
			stops.Load(),
		)
	}
}

func TestRunPreservesDependenciesWhenServiceStopFails(t *testing.T) {
	module := &lifecycleModule{}
	failure := errors.New("worker has not stopped")
	service := &lifecycleService{
		start: func(context.Context) error { return nil },
		stop:  func(context.Context) error { return failure },
	}
	err := Run(
		context.Background(),
		testOptions(t),
		func(ctx context.Context, runtime *Runtime, _ *struct{}) error {
			if err := runtime.Initialize(ctx, module); err != nil {
				return err
			}
			if err := runtime.AddService(service); err != nil {
				return err
			}
			return errors.New("assembly failed")
		},
	)
	if !errors.Is(err, failure) || module.closed.Load() {
		t.Fatalf("failed stop released dependency: err=%v closed=%v", err, module.closed.Load())
	}
}

func TestInitializationBudgetIncludesServiceStartup(t *testing.T) {
	options := testOptions(t)
	options.InitTimeout = 20 * time.Millisecond
	module := &lifecycleModule{}
	rescue := make(chan struct{})
	service := &lifecycleService{
		start: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-rescue:
				return errors.New("test released blocked startup")
			}
		},
		stop: func(context.Context) error { return nil },
	}
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), options, func(ctx context.Context, runtime *Runtime, _ *struct{}) error {
			if err := runtime.Initialize(ctx, module); err != nil {
				return err
			}
			return runtime.AddService(service)
		})
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) || !module.closed.Load() {
			t.Fatalf("startup deadline did not clean up: err=%v closed=%v", err, module.closed.Load())
		}
	case <-time.After(3 * time.Second):
		close(rescue)
		<-done
		t.Fatal("initialization budget did not cancel Start")
	}
}
