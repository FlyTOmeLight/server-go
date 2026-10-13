package service

import (
	"context"
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
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := worker.Run(ctx); err != nil {
		t.Fatal(err)
	}
}
