package cron

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testTask struct {
	name                          string
	initErr, startErr, stopErr    error
	init                          func(context.Context) error
	start                         func(context.Context) error
	stop                          func(context.Context) error
	initialized, started, stopped atomic.Int32
}

func (t *testTask) Name() string { return t.name }

func (t *testTask) Init(ctx context.Context) error {
	t.initialized.Add(1)
	if t.init != nil {
		return t.init(ctx)
	}
	return t.initErr
}

func (t *testTask) Start(ctx context.Context) error {
	t.started.Add(1)
	if t.start != nil {
		return t.start(ctx)
	}
	return t.startErr
}

func (t *testTask) Stop(ctx context.Context) error {
	t.stopped.Add(1)
	if t.stop != nil {
		return t.stop(ctx)
	}
	return t.stopErr
}

func TestTaskManagerInterval(t *testing.T) {
	for _, interval := range []time.Duration{time.Second, 5 * time.Second, 90 * time.Second} {
		manager, err := NewTaskManager(interval, 1, false)
		if err != nil {
			t.Fatal(err)
		}
		target := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
		next := manager.scheduler.Entries()[0].Schedule.Next(target)
		if next.Sub(target) != interval {
			t.Fatalf("interval=%s: next after %s", interval, next.Sub(target))
		}
		if err := manager.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	for _, interval := range []time.Duration{0, -time.Second, 100 * time.Millisecond, 1500 * time.Millisecond} {
		if _, err := NewTaskManager(interval, 1, false); err == nil {
			t.Fatalf("unsupported interval %s accepted", interval)
		}
	}
}

func TestStopCancelsAndWaitsBeforeClosingTask(t *testing.T) {
	manager, err := NewTaskManager(time.Hour, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	completed := make(chan struct{})
	task := &testTask{name: "blocked", start: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(completed)
		return ctx.Err()
	}}
	ctx := context.Background()
	if err := manager.Add(ctx, NewPeriodTask(task, nil, AutoUpdate, true)); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := manager.Exec(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("task did not start")
	}
	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := manager.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-completed:
	default:
		t.Fatal("task still executing after stop")
	}
	if task.stopped.Load() != 1 {
		t.Fatalf("resource closed %d times", task.stopped.Load())
	}
	if err := manager.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	if task.stopped.Load() != 1 {
		t.Fatal("repeated stop closed resources twice")
	}
	if err := manager.Add(ctx, NewPeriodTask(&testTask{name: "late"}, nil, AutoUpdate, true)); err == nil {
		t.Fatal("add after shutdown succeeded")
	}
}

func TestStopDeadlineKeepsCleanupOwned(t *testing.T) {
	manager, err := NewTaskManager(time.Hour, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	task := &testTask{name: "slow", start: func(context.Context) error { close(started); <-release; return nil }}
	ctx := context.Background()
	if err := manager.Add(ctx, NewPeriodTask(task, nil, AutoUpdate, true)); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := manager.Exec(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("task did not start")
	}
	stopCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	err = manager.Stop(stopCtx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stop deadline: %v", err)
	}
	if task.stopped.Load() != 0 {
		t.Fatal("resource closed before active work finished")
	}
	close(release)
	waitCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := manager.Stop(waitCtx); err != nil {
		t.Fatal(err)
	}
	if task.stopped.Load() != 1 {
		t.Fatal("eventual cleanup did not close resource")
	}
}

func TestPeriodTaskFailureStatusAndUpdate(t *testing.T) {
	failure := errors.New("cannot start")
	task := &testTask{name: "failing", startErr: failure}
	period := NewPeriodTask(task, nil, AutoUpdate, true)
	if err := period.Exec(context.Background()); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if period.Status() != TaskStatusFailed {
		t.Fatalf("status=%s", period.Status())
	}
	manager, err := NewTaskManager(time.Hour, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := manager.Add(ctx, period); err != nil {
		t.Fatal(err)
	}
	replacement := &testTask{name: "failing"}
	if err := manager.Add(ctx, NewPeriodTask(replacement, nil, AutoUpdate, false)); err != nil {
		t.Fatal(err)
	}
	if manager.GetTask("failing") != task || replacement.initialized.Load() != 0 {
		t.Fatal("auto update replaced an owned task")
	}
	if err := manager.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if task.stopped.Load() != 1 {
		t.Fatal("failed task resources were not closed")
	}
}

func TestTaskManagerConcurrentAccess(t *testing.T) {
	manager, err := NewTaskManager(time.Hour, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = manager.Add(ctx, NewPeriodTask(&testTask{name: "shared"}, nil, AutoUpdate, true))
				_ = manager.GetTask("shared")
				if task := manager.GetPeriodTask("shared"); task != nil {
					task.SetEnabled(j%2 == 0)
					_ = task.Status()
				}
				_ = manager.Exec(ctx)
				_ = manager.Remove(ctx, "shared")
			}
		}()
	}
	wg.Wait()
	if err := manager.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestRunPeriodDoesNotMutateInput(t *testing.T) {
	start := time.Date(2000, 1, 1, 22, 0, 0, 0, time.UTC)
	end := time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC)
	period := RunPeriod{Start: &start, End: &end}
	if !period.IsValid(time.Date(2026, 10, 12, 23, 0, 0, 0, time.UTC)) {
		t.Fatal("overnight period rejected")
	}
	if start.Year() != 2000 || end.Year() != 2000 {
		t.Fatal("time validation mutated shared input")
	}
	if (&RunPeriod{}).IsValid() {
		t.Fatal("missing times accepted")
	}
}

func TestBackgroundTaskContextLivesUntilManagerStop(t *testing.T) {
	manager, err := NewTaskManager(time.Hour, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	started := make(chan context.Context, 1)
	done := make(chan struct{})
	task := &testTask{name: "background", start: func(ctx context.Context) error {
		go func() { <-ctx.Done(); close(done) }()
		started <- ctx
		return nil
	}, stop: func(ctx context.Context) error {
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	period := NewPeriodTask(task, nil, AutoUpdate, true)
	if err := manager.Add(ctx, period); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	invocation, cancelInvocation := context.WithCancel(ctx)
	if err := manager.Exec(invocation); err != nil {
		t.Fatal(err)
	}
	var workCtx context.Context
	select {
	case workCtx = <-started:
	case <-time.After(time.Second):
		t.Fatal("background task did not start")
	}
	cancelInvocation()
	// Status waits for Start to return; completion of a scheduling invocation
	// must not cancel the background work that Start created.
	if period.Status() != TaskStatusRunning {
		t.Fatal("task is not running")
	}
	if err := workCtx.Err(); err != nil {
		t.Fatalf("background task context ended with scheduling invocation: %v", err)
	}
	if err := manager.Exec(ctx); err != nil {
		t.Fatal(err)
	}
	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := manager.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	default:
		t.Fatal("background task survived manager shutdown")
	}
	if task.started.Load() != 1 {
		t.Fatal("running background task started again")
	}
}

func TestStopCallerDeadlineDoesNotExpireResourceCleanup(t *testing.T) {
	manager, err := NewTaskManager(time.Hour, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	var cleaned atomic.Bool
	task := &testTask{
		name: "slow-cleanup", start: func(context.Context) error { close(started); <-release; return nil },
		stop: func(ctx context.Context) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if _, ok := ctx.Deadline(); !ok {
				return errors.New("cleanup is unbounded")
			}
			cleaned.Store(true)
			return nil
		},
	}
	ctx := context.Background()
	if err := manager.Add(ctx, NewPeriodTask(task, nil, AutoUpdate, true)); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := manager.Exec(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("task did not start")
	}
	stopCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	if err := manager.Stop(stopCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("caller deadline: %v", err)
	}
	cancel()
	close(release)
	waitCtx, cancelWait := context.WithTimeout(ctx, time.Second)
	defer cancelWait()
	if err := manager.Stop(waitCtx); err != nil {
		t.Fatal(err)
	}
	if !cleaned.Load() {
		t.Fatal("resource cleanup was prevented by expired caller context")
	}
}

func TestFailedCleanupRemainsOwnedAndCanBeRetried(t *testing.T) {
	manager, err := NewTaskManager(time.Hour, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("resource close failed")
	var attempts atomic.Int32
	task := &testTask{name: "retry", stop: func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if attempts.Add(1) == 1 {
			return failure
		}
		return nil
	}}
	ctx := context.Background()
	period := NewPeriodTask(task, nil, AutoUpdate, true)
	if err := manager.Add(ctx, period); err != nil {
		t.Fatal(err)
	}
	if err := manager.Stop(ctx); !errors.Is(err, failure) {
		t.Fatalf("close failure: %v", err)
	}
	if manager.GetTask("retry") != task || period.Status() != TaskStatusFailed {
		t.Fatal("failed resource was discarded")
	}
	if err := manager.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if manager.GetTask("retry") != nil || attempts.Load() != 2 {
		t.Fatal("failed resource was not retried and released")
	}
	if err := manager.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatal("successful resource cleanup repeated")
	}
}

func TestDisabledTaskCancelsItsBackgroundWork(t *testing.T) {
	ctx := context.Background()
	done := make(chan struct{})
	task := &testTask{name: "window", start: func(ctx context.Context) error {
		go func() { <-ctx.Done(); close(done) }()
		return nil
	}, stop: func(ctx context.Context) error {
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	period := NewPeriodTask(task, nil, AutoUpdate, true)
	if err := period.Exec(ctx); err != nil {
		t.Fatal(err)
	}
	period.SetEnabled(false)
	stopCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := period.Exec(stopCtx); err != nil {
		t.Fatal(err)
	}
	if period.Status() != TaskStatusStopped {
		t.Fatal("disabled task still running")
	}
}

func TestCancelledInitializationCleanupKeepsFailedResourcesOwned(t *testing.T) {
	for _, failCleanup := range []bool{false, true} {
		t.Run(fmt.Sprintf("cleanup-failure=%v", failCleanup), func(t *testing.T) {
			manager, err := NewTaskManager(time.Hour, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failure := errors.New("resource close failed")
			var attempts atomic.Int32
			task := &testTask{
				name: "initializing", init: func(context.Context) error { cancel(); return ctx.Err() },
				stop: func(cleanup context.Context) error {
					if err := cleanup.Err(); err != nil {
						return err
					}
					if _, ok := cleanup.Deadline(); !ok {
						return errors.New("unbounded cleanup")
					}
					if attempts.Add(1) == 1 && failCleanup {
						return failure
					}
					return nil
				},
			}
			period := NewPeriodTask(task, nil, AutoUpdate, true)
			err = manager.Add(ctx, period)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled initialization: %v", err)
			}
			if failCleanup {
				if !errors.Is(err, failure) {
					t.Fatalf("cleanup error lost: %v", err)
				}
				if manager.GetTask(task.name) != task || period.Status() != TaskStatusFailed {
					t.Fatal("failed initialization resource was discarded")
				}
			} else if manager.GetTask(task.name) != nil {
				t.Fatal("successfully cleaned failed initialization remained owned")
			}
			stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
			defer stopCancel()
			if err := manager.Stop(stopCtx); err != nil {
				t.Fatal(err)
			}
			if manager.GetTask(task.name) != nil {
				t.Fatal("cleanup retry left resources owned")
			}
			wantAttempts := int32(1)
			if failCleanup {
				wantAttempts = 2
			}
			if attempts.Load() != wantAttempts {
				t.Fatalf("cleanup attempts=%d; want %d", attempts.Load(), wantAttempts)
			}
		})
	}
}
