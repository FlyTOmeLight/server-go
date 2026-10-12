package server

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type lifecycleModule struct {
	name              string
	events            *[]string
	initErr, closeErr error
	onInit            func(context.Context) error
	onClose           func(context.Context) error
}

func (m *lifecycleModule) Initialize(ctx context.Context) error {
	*m.events = append(*m.events, "init:"+m.name)
	if m.onInit != nil {
		return m.onInit(ctx)
	}
	return m.initErr
}
func (m *lifecycleModule) Close(ctx context.Context) error {
	*m.events = append(*m.events, "close:"+m.name)
	if m.onClose != nil {
		return m.onClose(ctx)
	}
	return m.closeErr
}

func TestBootstrapRollbackAndClose(t *testing.T) {
	failure := errors.New("initialize failed")
	cleanupFailure := errors.New("close failed")
	for _, failing := range []bool{false, true} {
		var events []string
		a := &lifecycleModule{name: "a", events: &events, closeErr: cleanupFailure}
		b := &lifecycleModule{name: "b", events: &events}
		c := &lifecycleModule{name: "c", events: &events}
		if failing {
			c.initErr = failure
		}
		bootstrap := NewBootstrap(a, b, c)
		err := bootstrap.Initialize(context.Background())
		if failing {
			if !errors.Is(err, failure) || !errors.Is(err, cleanupFailure) {
				t.Fatalf("rollback errors lost: %v", err)
			}
			if repeat := bootstrap.Initialize(context.Background()); !errors.Is(repeat, failure) {
				t.Fatalf("repeated initialize lost failure: %v", repeat)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if failing {
			a.closeErr = nil
		}
		closeErr := bootstrap.Close(context.Background())
		if !failing && !errors.Is(closeErr, cleanupFailure) {
			t.Fatalf("close error lost: %v", closeErr)
		}
		a.closeErr = nil
		if err := bootstrap.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
		_ = bootstrap.Close(context.Background())
		want := []string{"init:a", "init:b", "init:c", "close:b", "close:a", "close:a"}
		if !failing {
			want = []string{"init:a", "init:b", "init:c", "close:c", "close:b", "close:a", "close:a"}
		}
		if !reflect.DeepEqual(events, want) {
			t.Fatalf("events=%v; want %v", events, want)
		}
	}
}

func TestCancelledInitializationUsesBoundedRollbackContext(t *testing.T) {
	var events []string
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	closed := false
	resource := &lifecycleModule{name: "resource", events: &events, onClose: func(cleanup context.Context) error {
		if err := cleanup.Err(); err != nil {
			return err
		}
		deadline, ok := cleanup.Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Fatal("rollback lacks bounded cleanup deadline")
		}
		closed = true
		return nil
	}}
	failure := &lifecycleModule{name: "failure", events: &events, onInit: func(context.Context) error { cancel(); return ctx.Err() }}
	bootstrap := NewBootstrap(resource, failure)
	if err := bootstrap.Initialize(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("initialization: %v", err)
	}
	if !closed {
		t.Fatal("cancelled initialization left resources open")
	}
	if err := bootstrap.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
