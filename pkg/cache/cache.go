package cache

import (
	"fmt"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
)

type String struct {
	Key string
}

func (s *String) String() string {
	return s.Key
}

// LoadFn Caller should guarantee the uniqueness of the key
type LoadFn func(key fmt.Stringer) (any, error)

type Cache struct {
	*cache.Cache
	load LoadFn
}

func NewCache(defaultExpiration, cleanupInterval time.Duration, loadFn LoadFn) *Cache {
	return &Cache{Cache: cache.New(defaultExpiration, cleanupInterval), load: loadFn}
}

func (c *Cache) Lookup(key fmt.Stringer) (any, error) {
	internalKey := key.String()
	if value, exist := c.Get(internalKey); exist {
		return value, nil
	}

	value, err := c.load(key)
	if err != nil {
		return nil, err
	}
	c.Set(internalKey, value, cache.DefaultExpiration)
	return value, nil
}

func (c *Cache) DeleteByPrefix(prefix string) {
	items := c.Cache.Items()
	for k := range items {
		if strings.HasPrefix(k, prefix) {
			c.Cache.Delete(k)
		}
	}
}
