package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestKubernetesConfigurationErrorAndExplicitLocalMode(t *testing.T) {
	configuration := Config{AppName: "worker", Kubeconfig: filepath.Join(t.TempDir(), "missing")}
	if worker, err := New(configuration); err == nil || worker != nil {
		t.Fatalf("invalid kubeconfig returned worker=%v error=%v", worker, err)
	}
	configuration.Local = true
	worker, err := New(configuration)
	if err != nil {
		t.Fatal(err)
	}
	canceledCtx, cancelImmediately := context.WithCancel(context.Background())
	cancelImmediately()
	if err := worker.Run(canceledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Run returned %v; want context.Canceled", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	worker.election.WithStartedLeadingCallback(func(context.Context) { close(started) })
	cancelAfterStart := make(chan error, 1)
	go func() {
		cancelAfterStart <- worker.Run(ctx)
	}()
	<-started
	cancel()
	if err := <-cancelAfterStart; err != nil {
		t.Fatalf("stopped Run returned %v", err)
	}
}
