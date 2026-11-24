package redis

import (
	"context"
	"errors"
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

func TestLazyModuleSkipsStartupPing(t *testing.T) {
	if err := (&Module{Lazy: true}).Initialize(t.Context()); err == nil {
		t.Fatal("missing configuration accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	module := &Module{Lazy: true, Config: &Config{Addrs: []string{"127.0.0.1:1"}}}
	if err := module.Initialize(ctx); err != nil || module.Redis == nil {
		t.Fatalf("lazy initialization: client=%v error=%v", module.Redis, err)
	}
	if err := module.Close(t.Context()); err != nil || module.Redis != nil {
		t.Fatalf("close: client=%v error=%v", module.Redis, err)
	}
}

type retryCloseClient struct {
	Redis
	err error
}

func (c *retryCloseClient) Close() error { return c.err }

func TestModuleRetainsClientForCloseRetry(t *testing.T) {
	closeErr := errors.New("redis close failed")
	client := &retryCloseClient{err: closeErr}
	module := &Module{Redis: client}
	if err := module.Close(t.Context()); !errors.Is(err, closeErr) || module.Redis != client {
		t.Fatalf("failed close: client=%v error=%v", module.Redis, err)
	}
	client.err = nil
	if err := module.Close(t.Context()); err != nil || module.Redis != nil {
		t.Fatalf("close retry: client=%v error=%v", module.Redis, err)
	}
}
