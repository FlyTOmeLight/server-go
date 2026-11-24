package cache

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheStringKey(t *testing.T) {
	c := NewCache(time.Second, time.Minute, mockLoadStringKey)

	_, err := c.Lookup(&String{Key: "ok"})
	assert.Nil(t, err)
	_, err = c.Lookup(&String{Key: "ok"}) // cache hit
	assert.Nil(t, err)
	c.Delete("ok")

	_, err = c.Lookup(&String{Key: "fail"})
	assert.NotNil(t, err)
}

func mockLoadStringKey(key fmt.Stringer) (any, error) {
	if key.String() == "fail" {
		return nil, errors.New("failed")
	}
	return "ok", nil
}

type cacheItem struct {
	key        string
	extraValue string
}

func (c *cacheItem) String() string {
	return c.key
}

func TestCacheStructKey(t *testing.T) {
	c := NewCache(time.Second, time.Minute, mockLoadStructKey)

	_, err := c.Lookup(&cacheItem{key: "ok", extraValue: "test"})
	assert.Nil(t, err)
	_, err = c.Lookup(&cacheItem{key: "ok", extraValue: "test"}) // cache hit
	assert.Nil(t, err)
	c.Delete("ok")

	_, err = c.Lookup(&cacheItem{key: "fail", extraValue: ""})
	assert.NotNil(t, err)
}

func mockLoadStructKey(key fmt.Stringer) (any, error) {
	item, ok := key.(*cacheItem)
	if !ok {
		return nil, errors.New("type error")
	}
	if item.key == "fail" {
		return nil, errors.New("failed")
	}
	if item.extraValue != "test" {
		return nil, errors.New("failed")
	}
	return "ok", nil
}

func TestCacheExpiration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewCache(time.Second, 0, nil)
		c.Set("default", "default", DefaultExpiration)
		c.Set("short", "short", time.Millisecond)
		c.Set("permanent", "permanent", NoExpiration)
		got, found := c.Get("default")
		require.True(t, found)
		require.Equal(t, "default", got)

		time.Sleep(time.Second + time.Nanosecond)
		require.Equal(t, map[string]Item{"permanent": {Object: "permanent"}}, c.Items())
		_, found = c.Get("short")
		require.False(t, found)
		require.Equal(t, 2, c.ItemCount())
		c.DeleteExpired()
		require.Equal(t, 1, c.ItemCount())
		got, found = c.Get("permanent")
		require.True(t, found)
		require.Equal(t, "permanent", got)
	})
}

func TestCacheOpportunisticCleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewCache(time.Millisecond, time.Second, nil)
		c.Set("expired", 1, DefaultExpiration)
		c.Set("live", 2, NoExpiration)
		time.Sleep(time.Second)
		_, found := c.Get("live")
		require.True(t, found)
		require.Equal(t, 1, c.ItemCount())
	})
}

func TestCacheDeleteByPrefixAndFlush(t *testing.T) {
	c := NewCache(NoExpiration, 0, nil)
	c.Set("a:first", 1, DefaultExpiration)
	c.Set("a:second", 2, DefaultExpiration)
	c.Set("b:first", 3, DefaultExpiration)
	delete(c.Items(), "b:first")
	require.Equal(t, 3, c.ItemCount())
	c.DeleteByPrefix("a:")
	require.Equal(t, map[string]Item{"b:first": {Object: 3}}, c.Items())

	c.Flush()
	require.Empty(t, c.Items())
	require.Zero(t, c.ItemCount())
	c.Set("after-flush", 4, DefaultExpiration)
	c.Delete("after-flush")
	require.Zero(t, c.ItemCount())
	require.NoError(t, c.Close())
	require.NoError(t, c.Close())
}

func TestCacheLookupWithoutKeyOrLoader(t *testing.T) {
	c := NewCache(NoExpiration, 0, nil)
	_, err := c.Lookup(nil)
	require.ErrorContains(t, err, "cache key is nil")
	var missing *String
	_, err = c.Lookup(missing)
	require.ErrorContains(t, err, "cache key is nil")
	key := &String{Key: "manual"}
	_, err = c.Lookup(key)
	require.ErrorContains(t, err, "cache load function is nil")
	c.Set(key.Key, 1, DefaultExpiration)
	got, err := c.Lookup(key)
	require.NoError(t, err)
	require.Equal(t, 1, got)
}

func TestCacheConcurrentAccess(t *testing.T) {
	c := NewCache(time.Second, time.Nanosecond, mockLoadStringKey)
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			key := &String{Key: "shared"}
			for range 32 {
				c.Set(key.Key, "ok", DefaultExpiration)
				got, err := c.Lookup(key)
				if err != nil || got != "ok" {
					t.Errorf("Lookup = %v, %v, want ok", got, err)
					return
				}
				c.Items()
				c.ItemCount()
				c.Delete(key.Key)
				c.DeleteByPrefix("shared")
				c.DeleteExpired()
				c.Flush()
			}
		})
	}
	workers.Wait()
}
