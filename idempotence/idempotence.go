// Package idempotence coordinates duplicate operations within one process.
package idempotence

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

var (
	ErrConflict        = errors.New("idempotent request parameters differ")
	ErrClosed          = errors.New("idempotence controller is closed")
	ErrExecutionFailed = errors.New("idempotent operation did not complete")
)

// Key scopes a token to a verified tenant, user and operation.
type Key struct {
	TenantID  string
	UserID    string
	Operation string
	Token     string
}

// Response is the complete HTTP result of an operation.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

func (r Response) clone() Response {
	return Response{Status: r.Status, Header: r.Header.Clone(), Body: append([]byte(nil), r.Body...)}
}

type entry struct {
	fingerprint [32]byte
	done        chan struct{}
	response    Response
	err         error
	expires     time.Time
}

// Controller owns an independent cache. Completed results expire after ttl;
// in-progress operations never expire and therefore cannot execute twice.
// Expired entries are reclaimed on the next request, without background workers.
type Controller struct {
	mu      sync.Mutex
	entries map[Key]*entry
	ttl     time.Duration
	closed  chan struct{}
}

// New constructs a process-local controller. Nonpositive ttl defaults to 24 hours.
func New(ttl time.Duration) *Controller {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Controller{entries: make(map[Key]*entry), ttl: ttl, closed: make(chan struct{})}
}

// Do atomically claims an operation. Matching duplicates wait for its result;
// conflicting duplicates fail without running execute. Failed results (errors or
// HTTP statuses >=400) are shared with current waiters but allow later retries.
// A canceled waiter does not cancel the original operation. execute runs on the
// caller's goroutine.
func (c *Controller) Do(
	ctx context.Context,
	key Key,
	fingerprint [32]byte,
	execute func() (Response, error),
) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	c.mu.Lock()
	select {
	case <-c.closed:
		c.mu.Unlock()
		return Response{}, ErrClosed
	default:
	}
	now := time.Now()
	for k, e := range c.entries {
		if !e.expires.IsZero() && !now.Before(e.expires) {
			delete(c.entries, k)
		}
	}
	if e, exists := c.entries[key]; exists {
		c.mu.Unlock()
		if e.fingerprint != fingerprint {
			return Response{}, ErrConflict
		}
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		case <-c.closed:
			return Response{}, ErrClosed
		case <-e.done:
			return e.response.clone(), e.err
		}
	}
	e := &entry{fingerprint: fingerprint, done: make(chan struct{})}
	c.entries[key] = e
	c.mu.Unlock()

	completed := false
	defer func() {
		if !completed {
			// Keep a failed claim until ttl when execute panics. Retrying an
			// operation with unknown effects must not execute it a second time.
			c.complete(key, e, Response{}, ErrExecutionFailed, true)
		}
	}()
	response, err := execute()
	c.complete(key, e, response, err, err == nil && response.Status < http.StatusBadRequest)
	completed = true
	return response, err
}

func (c *Controller) complete(key Key, e *entry, response Response, err error, cache bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e.response = response.clone()
	e.err = err
	e.expires = time.Now().Add(c.ttl)
	close(e.done)
	if !cache {
		delete(c.entries, key)
	}
}

// Close releases cached results and wakes waiting duplicates. The owner must
// drain executing operations before closing their other dependencies.
func (c *Controller) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.closed:
		return nil
	default:
		close(c.closed)
		clear(c.entries)
		return nil
	}
}
