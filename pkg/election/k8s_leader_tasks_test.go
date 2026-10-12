package election

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestK8sLeaderCallbacksCancelAndJoinTasks(t *testing.T) {
	election := NewK8sLeaderElection("application")
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	election.RunOnLeader(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release
	})
	callbacks := election.leaderCallbacks(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startedCallbackDone := make(chan struct{})
	go func() { callbacks.OnStartedLeading(ctx); close(startedCallbackDone) }()
	t.Cleanup(func() {
		cancel()
		releaseOnce.Do(func() { close(release) })
		select {
		case <-startedCallbackDone:
		case <-time.After(time.Second):
			t.Error("leader callback leaked its tasks")
		}
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("leader task did not start")
	}
	stoppedCallbackDone := make(chan struct{})
	go func() { callbacks.OnStoppedLeading(); close(stoppedCallbackDone) }()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("leadership loss did not cancel its task")
	}
	select {
	case <-stoppedCallbackDone:
		t.Fatal("leadership term ended before task cleanup")
	default:
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-stoppedCallbackDone:
	case <-time.After(time.Second):
		t.Fatal("leadership loss did not wait for its task")
	}
}

func TestK8sLeaderTaskInheritsParentCancellationAndCannotStartLate(t *testing.T) {
	election := NewK8sLeaderElection("application")
	var executions atomic.Int32
	started := make(chan struct{})
	election.RunOnLeader(func(ctx context.Context) {
		executions.Add(1)
		close(started)
		<-ctx.Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	callbacks := election.leaderCallbacks(ctx)
	done := make(chan struct{})
	go func() { callbacks.OnStartedLeading(ctx); close(done) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("leader task did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("leader task did not inherit parent cancellation")
	}
	callbacks.OnStoppedLeading()
	// The elector may dispatch this callback after its term already stopped.
	callbacks.OnStartedLeading(context.Background())
	if executions.Load() != 1 {
		t.Fatal("a stopped term started leader work")
	}
}
