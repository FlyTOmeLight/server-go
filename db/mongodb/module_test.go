package mongodb

import (
	"context"
	"errors"
	"testing"
)

func TestModuleRejectsMissingConfigAndCancelledInitialization(t *testing.T) {
	if err := (&Module{}).Initialize(context.Background()); err == nil {
		t.Fatal("missing configuration accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	module := &Module{Config: &Config{Host: "127.0.0.1", Port: 1, User: "user", Password: "password", DB: "db", Collection: "items"}}
	if err := module.Initialize(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled initialization: %v", err)
	}
	if module.Client != nil {
		t.Fatal("failed initialization published a client")
	}
	if err := module.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
