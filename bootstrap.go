package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Module owns a resource initialized and closed by one Bootstrap.
// Initialize must release its own partial resources before returning an error.
type Module interface {
	Initialize(context.Context) error
	Close(context.Context) error
}

type Bootstrap struct {
	mu          sync.Mutex
	modules     []Module
	initialized []Module
	started     bool
	closed      bool
	initErr     error
	closeErr    error
}

func NewBootstrap(modules ...Module) *Bootstrap {
	return &Bootstrap{modules: append([]Module(nil), modules...)}
}

func (b *Bootstrap) Initialize(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("bootstrap is closed")
	}
	if b.started {
		return b.initErr
	}
	b.started = true
	for index, module := range b.modules {
		if err := ctx.Err(); err != nil {
			b.initErr = errors.Join(err, b.rollback(ctx))
			return b.initErr
		}
		if module == nil {
			b.initErr = errors.Join(fmt.Errorf("module %d is nil", index), b.rollback(ctx))
			return b.initErr
		}
		if err := module.Initialize(ctx); err != nil {
			b.initErr = errors.Join(fmt.Errorf("initialize module %d: %w", index, err), b.rollback(ctx))
			return b.initErr
		}
		b.initialized = append(b.initialized, module)
	}
	return nil
}

func (b *Bootstrap) Close(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed || len(b.initialized) > 0 {
		b.closed = true
		b.closeErr = b.closeInitialized(ctx)
	}
	return b.closeErr
}

func (b *Bootstrap) rollback(ctx context.Context) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return b.closeInitialized(cleanupCtx)
}

func (b *Bootstrap) closeInitialized(ctx context.Context) error {
	var errs []error
	failed := make([]Module, 0)
	for index := len(b.initialized) - 1; index >= 0; index-- {
		if err := b.initialized[index].Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("close module %d: %w", index, err))
			failed = append(failed, b.initialized[index])
		}
	}
	for left, right := 0, len(failed)-1; left < right; left, right = left+1, right-1 {
		failed[left], failed[right] = failed[right], failed[left]
	}
	b.initialized = failed
	return errors.Join(errs...)
}
