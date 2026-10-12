package cache

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCacheStringKey(t *testing.T) {
	c := NewCache(time.Second, time.Minute, mockLoadStringKey)

	_, err := c.Lookup(&String{"ok"})
	assert.Nil(t, err)
	_, err = c.Lookup(&String{"ok"}) // cache hit
	assert.Nil(t, err)
	c.Delete("ok")

	_, err = c.Lookup(&String{"fail"})
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
		return nil, fmt.Errorf("type error")
	}
	if item.key == "fail" {
		return nil, errors.New("failed")
	}
	if item.extraValue != "test" {
		return nil, errors.New("failed")
	}
	return "ok", nil
}
