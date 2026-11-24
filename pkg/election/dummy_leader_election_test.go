package election

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/synctest"
)

func TestDummyElectionCallbackOrderAndConcurrentRun(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		election := NewDummyAppElection("local")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		events := make(chan string, 6)
		release := make(chan struct{})
		election.WithStartedLeadingCallback(nil).WithStoppedLeadingCallback(nil).WithNewLeaderCallback(nil)
		election.RunOnLeader(nil)
		election.WithStartedLeadingCallback(func(context.Context) { events <- "started 1" }).
			WithStartedLeadingCallback(func(context.Context) { events <- "started 2" }).
			WithNewLeaderCallback(func(identity string) { events <- identity }).
			WithStoppedLeadingCallback(func() { events <- "stopped" })
		election.RunOnLeader(func(ctx context.Context) {
			events <- "task started"
			<-ctx.Done()
			<-release
			events <- "task stopped"
		})
		done := election.RunAsync(ctx)
		synctest.Wait()
		// Registrations during a run must not change its callback and task snapshot.
		election.WithStoppedLeadingCallback(func() { t.Error("late callback ran in the active term") })
		competingErr := election.Run(ctx)
		cancel()
		synctest.Wait()
		if len(events) != 4 {
			t.Errorf("received %d events before the task drained, want 4", len(events))
		}
		close(release)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if competingErr == nil {
			t.Fatal("concurrent Run succeeded")
		}
		close(events)
		var got []string
		for event := range events {
			got = append(got, event)
		}
		want := []string{"started 1", "started 2", "local", "task started", "task stopped", "stopped"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("events=%v; want %v", got, want)
		}
	})
}

func TestDummyElectionRejectsCanceledStartup(t *testing.T) {
	election := NewDummyAppElection("local")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	election.WithStartedLeadingCallback(func(context.Context) { t.Error("canceled election started") }).
		WithNewLeaderCallback(func(string) { t.Error("canceled election announced a leader") }).
		WithStoppedLeadingCallback(func() { t.Error("canceled election stopped a term it did not start") })
	election.RunOnLeader(func(context.Context) { t.Error("canceled election started a task") })
	if err := <-election.RunAsync(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled startup returned %v", err)
	}
}
