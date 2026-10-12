package redis

import (
	"context"
	"testing"
)

func TestModuleRejectsMissingConfigAndCancelledPing(t *testing.T) {
	if err := (&Module{}).Initialize(context.Background()); err == nil {
		t.Fatal("missing configuration accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	module := &Module{Config: &Config{Addrs: []string{"127.0.0.1:1"}}}
	if err := module.Initialize(ctx); err == nil {
		t.Fatal("cancelled ping succeeded")
	}
	if module.Redis != nil {
		t.Fatal("failed initialization published a client")
	}
	if err := module.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
