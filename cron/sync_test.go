package cron

import (
	"context"
	"errors"
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
		})
	}
}
