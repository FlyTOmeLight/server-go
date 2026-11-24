// Package idempotence coordinates duplicate operations and replays HTTP results.
package idempotence

import (
	"container/list"
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

var (
	ErrConflict        = errors.New("idempotent request parameters differ")
	ErrClosed          = errors.New("idempotence controller is closed")
	ErrCapacity        = errors.New("idempotence controller is at capacity")
	ErrExecutionFailed = errors.New("idempotent operation did not complete")
)

const (
	defaultTTL          = 24 * time.Hour
	defaultMaxBodyBytes = 1 << 20
)

// Key scopes a token to a verified tenant, user, project and operation.
type Key struct {
	TenantID  string
	UserID    string
	ProjectID string
	Operation string
	Token     string
}

// Store coordinates operation execution and replay.
type Store interface {
	Do(ctx context.Context, key Key, fingerprint [32]byte, execute func() (Response, error)) (Response, error)
	Close() error
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
	key         Key
	fingerprint [32]byte
	done        chan struct{}
	response    Response
	err         error
	expires     time.Time
}

// Options configures a Controller. Nonpositive TTL and MaxEntries use defaults.
// A nonpositive MaxBodyBytes leaves the cached body size unlimited.
type Options struct {
	TTL          time.Duration
	MaxEntries   int
	MaxBodyBytes int
}

// Controller owns a bounded process-local cache. Completed results expire
// after TTL and may be evicted in completion order to make room for new claims.
// In-progress operations retain their entries until they finish.
type Controller struct {
	mu           sync.Mutex
	entries      map[Key]*entry
	completed    list.List
	ttl          time.Duration
	maxEntries   int
	maxBodyBytes int
	closed       chan struct{}
}

var _ Store = (*Controller)(nil)

// New constructs a process-local controller. Nonpositive ttl defaults to 24 hours.
func New(ttl time.Duration) *Controller {
	return NewWithOptions(Options{TTL: ttl, MaxEntries: 10000, MaxBodyBytes: defaultMaxBodyBytes})
}

// NewWithOptions constructs a process-local controller with the given limits.
func NewWithOptions(opts Options) *Controller {
	if opts.TTL <= 0 {
		opts.TTL = defaultTTL
	}
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = 10000
	}
	return &Controller{
		entries:      make(map[Key]*entry),
		ttl:          opts.TTL,
		maxEntries:   opts.MaxEntries,
		maxBodyBytes: opts.MaxBodyBytes,
		closed:       make(chan struct{}),
	}
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
	for oldest := c.completed.Front(); oldest != nil &&
		!now.Before(oldest.Value.(*entry).expires); oldest = c.completed.Front() {
		delete(c.entries, c.completed.Remove(oldest).(*entry).key)
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
	if len(c.entries) == c.maxEntries {
		oldest := c.completed.Front()
		if oldest == nil {
			c.mu.Unlock()
			return Response{}, ErrCapacity
		}
		delete(c.entries, c.completed.Remove(oldest).(*entry).key)
	}
	e := &entry{key: key, fingerprint: fingerprint, done: make(chan struct{})}
	c.entries[key] = e
	c.mu.Unlock()

	completed := false
	defer func() {
		if !completed {
			c.complete(e, Response{}, ErrExecutionFailed, true)
		}
	}()
	response, err := execute()
	cache := err == nil && response.Status < http.StatusBadRequest &&
		(c.maxBodyBytes <= 0 || len(response.Body) <= c.maxBodyBytes)
	c.complete(e, response, err, cache)
	completed = true
	return response, err
}

func (c *Controller) complete(e *entry, response Response, err error, cache bool) {
	e.response = response.clone()
	e.err = err
	c.mu.Lock()
	defer c.mu.Unlock()
	close(e.done)
	if c.entries[e.key] != e {
		return
	}
	if !cache {
		delete(c.entries, e.key)
		return
	}
	e.expires = time.Now().Add(c.ttl)
	c.completed.PushBack(e)
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
		c.completed.Init()
		return nil
	}
}
