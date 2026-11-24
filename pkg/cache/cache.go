package cache

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

const (
	DefaultExpiration time.Duration = 0
	NoExpiration      time.Duration = -1
)

// Item is a cached value with an expiration in Unix nanoseconds, or zero for no expiration.
type Item struct {
	Object     any
	Expiration int64
}

func (item Item) expired(now int64) bool {
	return item.Expiration > 0 && now > item.Expiration
}

type String struct {
	Key string
}

func (s *String) String() string {
	return s.Key
}

// LoadFn Caller should guarantee the uniqueness of the key
type LoadFn func(key fmt.Stringer) (any, error)

type Cache struct {
	mu                sync.RWMutex
	items             map[string]Item
	defaultExpiration time.Duration
	cleanupInterval   time.Duration
	lastCleanup       time.Time
	load              LoadFn
}

func NewCache(defaultExpiration, cleanupInterval time.Duration, loadFn LoadFn) *Cache {
	return &Cache{
		items:             make(map[string]Item),
		defaultExpiration: defaultExpiration,
		cleanupInterval:   cleanupInterval,
		lastCleanup:       time.Now(),
		load:              loadFn,
	}
}

func (c *Cache) Lookup(key fmt.Stringer) (any, error) {
	valueOfKey := reflect.ValueOf(key)
	if key == nil || (valueOfKey.Kind() == reflect.Pointer && valueOfKey.IsNil()) {
		return nil, errors.New("cache key is nil")
	}
	internalKey := key.String()
	if value, exist := c.Get(internalKey); exist {
		return value, nil
	}
	if c.load == nil {
		return nil, errors.New("cache load function is nil")
	}

	value, err := c.load(key)
	if err != nil {
		return nil, err
	}
	c.Set(internalKey, value, DefaultExpiration)
	return value, nil
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	now := time.Now()
	item, found := c.items[key]
	cleanupDue := c.cleanupDue(now)
	c.mu.RUnlock()
	if !cleanupDue && !item.expired(now.UnixNano()) {
		return item.Object, found
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	now = time.Now()
	c.maybeDeleteExpired(now)
	item, found = c.items[key]
	if item.expired(now.UnixNano()) {
		delete(c.items, key)
		return nil, false
	}
	return item.Object, found
}

func (c *Cache) Set(key string, value any, expiration time.Duration) {
	if expiration == DefaultExpiration {
		expiration = c.defaultExpiration
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.maybeDeleteExpired(now)
	item := Item{Object: value}
	if expiration > 0 {
		item.Expiration = now.Add(expiration).UnixNano()
	}
	if c.items == nil {
		c.items = make(map[string]Item)
	}
	c.items[key] = item
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *Cache) DeleteByPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.items {
		if strings.HasPrefix(key, prefix) {
			delete(c.items, key)
		}
	}
}

func (c *Cache) DeleteExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteExpired(time.Now())
}

func (c *Cache) cleanupDue(now time.Time) bool {
	return c.cleanupInterval > 0 && now.Sub(c.lastCleanup) >= c.cleanupInterval
}

func (c *Cache) maybeDeleteExpired(now time.Time) {
	if c.cleanupDue(now) {
		c.deleteExpired(now)
	}
}

func (c *Cache) deleteExpired(now time.Time) {
	timestamp := now.UnixNano()
	for key, item := range c.items {
		if item.expired(timestamp) {
			delete(c.items, key)
		}
	}
	c.lastCleanup = now
}

func (c *Cache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = nil
}

func (c *Cache) Items() map[string]Item {
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := time.Now().UnixNano()
	items := make(map[string]Item, len(c.items))
	for key, item := range c.items {
		if !item.expired(now) {
			items[key] = item
		}
	}
	return items
}

// ItemCount includes expired items that have yet to be removed.
func (c *Cache) ItemCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Close returns nil because the cache owns no background resources.
func (c *Cache) Close() error { return nil }
