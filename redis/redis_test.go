package redis

import (
	"testing"
)

func TestNewRedisLazyStartsWhileServerUnreachable(t *testing.T) {
	client, err := NewRedisLazy(&Config{Addrs: []string{"127.0.0.1:1"}})
	if err != nil {
		t.Fatalf("lazy construction failed: %v", err)
	}
	if client == nil {
		t.Fatal("lazy construction returned no client")
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewRedisLazyRejectsMissingConfig(t *testing.T) {
	if _, err := NewRedisLazy(nil); err == nil {
		t.Fatal("missing configuration accepted")
	}
}
