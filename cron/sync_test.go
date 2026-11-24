package cron

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSyncHandlersReconcileCombinedTasks(t *testing.T) {
	ctx := context.Background()
	manager, err := NewTaskManager(time.Hour, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	a, b := &testTask{name: "a"}, &testTask{name: "b"}
	first := map[string]*PeriodTask{"a": NewPeriodTask(a, nil, AutoUpdate, true)}
	second := map[string]*PeriodTask{"b": NewPeriodTask(b, nil, AutoUpdate, true)}
	for _, tasks := range []map[string]*PeriodTask{first, second} {
		if err := manager.RegisterSyncHandler(
			func(context.Context) (map[string]*PeriodTask, error) { return tasks, nil },
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := manager.Exec(ctx); err != nil {
			t.Fatal(err)
		}
		for _, task := range []*testTask{a, b} {
			if manager.GetTask(task.name) != task || task.initialized.Load() != 1 ||
				task.stopped.Load() != 0 {
				t.Fatalf("source task %s was removed or recreated", task.name)
			}
		}
	}
	delete(first, "a")
	if err := manager.Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if manager.GetTask("a") != nil || a.stopped.Load() != 1 || manager.GetTask("b") != b || b.stopped.Load() != 0 {
		t.Fatal("combined sync did not remove only the deleted task")
	}
}

func TestSyncHandlerFailurePreservesExistingTasks(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(map[bool]string{false: "source failure", true: "duplicate name"}[duplicate], func(t *testing.T) {
			ctx := context.Background()
			manager, err := NewTaskManager(time.Hour, 1, true)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := manager.Stop(ctx); err != nil {
					t.Error(err)
				}
			})
			old, added := &testTask{name: "old"}, &testTask{name: "new"}
			started := make(chan struct{})
			old.start = func(context.Context) error { close(started); return nil }
			reported := make(chan error, 1)
			manager.SetErrorHandler(func(err error) { reported <- err })
			if err := manager.Add(ctx, NewPeriodTask(old, nil, AutoUpdate, true)); err != nil {
				t.Fatal(err)
			}
			tasks := map[string]*PeriodTask{"new": NewPeriodTask(added, nil, AutoUpdate, true)}
			if err := manager.RegisterSyncHandler(
				func(context.Context) (map[string]*PeriodTask, error) { return tasks, nil },
			); err != nil {
				t.Fatal(err)
			}
			failure := errors.New("source unavailable")
			if err := manager.RegisterSyncHandler(func(context.Context) (map[string]*PeriodTask, error) {
				if duplicate {
					return tasks, nil
				}
				return nil, failure
			}); err != nil {
				t.Fatal(err)
			}
			if err := manager.Start(ctx); err != nil {
				t.Fatal(err)
			}
			if err := manager.Exec(ctx); err == nil || (!duplicate && !errors.Is(err, failure)) {
				t.Fatalf("sync error=%v", err)
			}
			if manager.GetTask("old") != old || old.stopped.Load() != 0 || added.initialized.Load() != 0 {
				t.Fatal("failed source changed the previous task snapshot")
			}
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("failed source blocked the previous healthy task")
			}
			select {
			case err := <-reported:
				if (!duplicate && !errors.Is(err, failure)) ||
					(duplicate && !strings.Contains(err.Error(), "multiple sync handlers")) {
					t.Fatalf("handler received %v", err)
				}
			default:
				t.Fatal("source error was not reported")
			}
		})
	}
}

func TestSyncTaskFailuresDoNotBlockHealthyTasksOrDeletion(t *testing.T) {
	ctx := context.Background()
	manager := newStartedTaskManager(t, ctx, true)
	deleted := &testTask{name: "deleted"}
	if err := manager.Add(ctx, NewPeriodTask(deleted, nil, AutoUpdate, true)); err != nil {
		t.Fatal(err)
	}
	first, second := errors.New("cannot initialize first"), errors.New("cannot initialize second")
	started := make(chan struct{})
	good := &testTask{name: "good", start: func(context.Context) error { close(started); return nil }}
	synced := map[string]*PeriodTask{
		"first":   NewPeriodTask(&testTask{name: "first", initErr: first}, nil, AutoUpdate, true),
		"second":  NewPeriodTask(&testTask{name: "second", initErr: second}, nil, AutoUpdate, true),
		"good":    NewPeriodTask(good, nil, AutoUpdate, true),
		"invalid": nil,
	}
	if err := manager.RegisterSyncHandler(func(context.Context) (map[string]*PeriodTask, error) {
		return synced, nil
	}); err != nil {
		t.Fatal(err)
	}
	reported := make(chan error, 1)
	manager.SetErrorHandler(func(err error) { reported <- err })
	err := manager.Exec(ctx)
	if !errors.Is(err, first) || !errors.Is(err, second) || !strings.Contains(err.Error(), "invalid synced task") {
		t.Fatalf("sync errors were not combined: %v", err)
	}
	select {
	case got := <-reported:
		if got != err {
			t.Fatalf("handler received %v; want %v", got, err)
		}
	default:
		t.Fatal("sync task errors were not reported")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("sync errors prevented the healthy task from running")
	}
	if manager.GetTask(deleted.name) != nil || deleted.stopped.Load() != 1 {
		t.Fatal("sync errors prevented deleted task cleanup")
	}
}

func TestConcurrentSyncPreservesForbiddenUpdate(t *testing.T) {
	ctx := context.Background()
	manager := newStartedTaskManager(t, ctx, false)
	const count = 16
	started, release := make(chan struct{}), make(chan struct{})
	var initialized atomic.Int32
	task := &testTask{name: "existing", init: func(context.Context) error {
		if initialized.Add(1) == 1 {
			close(started)
			<-release
		}
		return nil
	}}
	synced := map[string]*PeriodTask{task.name: NewPeriodTask(task, nil, ForbiddenUpdate, true)}
	var workers sync.WaitGroup
	ready := make(chan struct{}, count)
	for range count {
		workers.Go(func() {
			ready <- struct{}{}
			if err := manager.SyncTasks(ctx, synced, nil); err != nil {
				t.Error(err)
			}
		})
	}
	<-started
	for range count {
		<-ready
	}
	close(release)
	workers.Wait()
	if manager.GetTask(task.name) != task || initialized.Load() != 1 {
		t.Fatal("concurrent sync recreated the task")
	}
}

func TestFailedInitCleanupEvictsTaskDuringExecution(t *testing.T) {
	ctx := context.Background()
	manager := newStartedTaskManager(t, ctx, false)
	cleanupFailure := errors.New("first cleanup failed")
	var attempts atomic.Int32
	task := &testTask{name: "retry", initErr: errors.New("init failed"), stop: func(context.Context) error {
		if attempts.Add(1) == 1 {
			return cleanupFailure
		}
		return nil
	}}
	period := NewPeriodTask(task, nil, AutoUpdate, true)
	if err := manager.Add(ctx, period); !errors.Is(err, cleanupFailure) || !errors.Is(err, task.initErr) {
		t.Fatalf("Add returned %v", err)
	}
	if manager.GetTask(task.name) != task {
		t.Fatal("failed init still owns resources after cleanup failure")
	}
	if err := manager.Exec(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	poll := time.NewTicker(time.Millisecond)
	defer poll.Stop()
	for manager.GetTask(task.name) != nil {
		select {
		case <-poll.C:
		case <-deadline.C:
			t.Fatal("successful cleanup left the task registered")
		}
	}
	if attempts.Load() != 2 || task.started.Load() != 0 {
		t.Fatalf("cleanup attempts=%d, starts=%d", attempts.Load(), task.started.Load())
	}
}

func TestSyncCanReinitializeSameTaskAfterFailedInitCleanup(t *testing.T) {
	ctx := context.Background()
	for _, policy := range []UpdatePolicy{AutoUpdate, ForbiddenUpdate, Recreate} {
		t.Run(string(policy), func(t *testing.T) {
			manager := newStartedTaskManager(t, ctx, false)
			initFailure := errors.New("first init failed")
			var inits, stops atomic.Int32
			task := &testTask{name: "retry", init: func(context.Context) error {
				if inits.Add(1) == 1 {
					return initFailure
				}
				return nil
			}, stop: func(context.Context) error {
				if stops.Add(1) == 1 {
					return errors.New("first cleanup failed")
				}
				return nil
			}}
			period := NewPeriodTask(task, nil, policy, true)
			synced := map[string]*PeriodTask{task.name: period}
			if err := manager.SyncTasks(ctx, synced, nil); !errors.Is(err, initFailure) {
				t.Fatalf("first sync returned %v", err)
			}
			if err := manager.SyncTasks(ctx, synced, nil); err != nil {
				t.Fatal(err)
			}
			if manager.GetPeriodTask(task.name) != period || inits.Load() != 2 || stops.Load() != 2 {
				t.Fatalf(
					"reinit did not take ownership: inits=%d, stops=%d",
					inits.Load(),
					stops.Load(),
				)
			}
			if err := period.Exec(ctx); err != nil {
				t.Fatal(err)
			}
			if period.Status() != TaskStatusRunning {
				t.Fatalf("reinitialized task not running: %s", period.Status())
			}
		})
	}
}
