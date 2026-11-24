package election

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestK8sLeaderCallbacksCancelAndJoinTasks(t *testing.T) {
	election := newTestK8sLeaderElection(t)
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	election.RunOnLeader(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release
	})
	callbacks := election.leaderCallbacks(context.Background(), nil)
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

func TestElectionBackoffGrowsCapsAndResetsAfterLeading(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		want := []time.Duration{
			0,
			500 * time.Millisecond,
			time.Second,
			2 * time.Second,
			4 * time.Second,
			5 * time.Second,
			5 * time.Second,
			500 * time.Millisecond,
			time.Second,
		}
		var attempts int
		previous := time.Now()
		err := runElection(ctx, func(context.Context) (bool, error) {
			now := time.Now()
			if got := now.Sub(previous); got != want[attempts] {
				t.Fatalf("attempt %d: wait=%s; want %s", attempts, got, want[attempts])
			}
			previous = now
			attempts++
			if attempts == len(want) {
				cancel()
			}
			return attempts == 7, nil
		})
		if err != nil || attempts != len(want) {
			t.Fatalf("attempts=%d, error=%v", attempts, err)
		}
	})
}

func TestElectionBackoffCanBeCanceled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		var attempts int
		go func() {
			done <- runElection(ctx, func(context.Context) (bool, error) {
				attempts++
				return false, nil
			})
		}()
		synctest.Wait()
		start := time.Now()
		cancel()
		if err := <-done; err != nil || attempts != 1 || time.Since(start) != 0 {
			t.Fatalf("cancellation took %s with %d attempts and error %v", time.Since(start), attempts, err)
		}
	})
}

func TestKubernetesLeaderHooksConcurrentRegistration(t *testing.T) {
	election := newTestK8sLeaderElection(t)
	ctx := context.Background()
	election.WithStartedLeadingCallback(nil).WithStoppedLeadingCallback(nil).WithNewLeaderCallback(nil)
	election.RunOnLeader(nil)
	var started, stopped, changed, tasks atomic.Int32
	var register sync.WaitGroup
	for range 16 {
		register.Go(func() {
			election.WithStartedLeadingCallback(func(context.Context) { started.Add(1) })
			election.WithStoppedLeadingCallback(func() { stopped.Add(1) })
			election.WithNewLeaderCallback(func(string) { changed.Add(1) })
			election.RunOnLeader(func(context.Context) { tasks.Add(1) })
			election.leaderCallbacks(ctx, nil)
		})
	}
	register.Wait()
	var led bool
	callbacks := election.leaderCallbacks(ctx, func() { led = true })
	callbacks.OnNewLeader(election.id)
	callbacks.OnStartedLeading(ctx)
	callbacks.OnStoppedLeading()
	if !led || started.Load() != 16 || stopped.Load() != 16 || changed.Load() != 16 || tasks.Load() != 16 {
		t.Fatalf("led=%v; started=%d, stopped=%d, changed=%d, tasks=%d",
			led, started.Load(), stopped.Load(), changed.Load(), tasks.Load())
	}
}

func TestK8sLeaderTaskInheritsParentCancellationAndCannotStartLate(t *testing.T) {
	election := newTestK8sLeaderElection(t)
	var executions atomic.Int32
	started := make(chan struct{})
	election.RunOnLeader(func(ctx context.Context) {
		executions.Add(1)
		close(started)
		<-ctx.Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	callbacks := election.leaderCallbacks(ctx, nil)
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
