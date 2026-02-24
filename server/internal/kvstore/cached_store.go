package kvstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/orca-zhang/ecache"
)

// CachedStore wraps a Store with an ecache read-through cache.
// Reads hit the cache first; writes go through to the underlying store
// and invalidate the cache entry so the next read refreshes.
type CachedStore struct {
	inner Store
	cache *ecache.Cache
}

// NewCachedStore wraps an existing Store with an LRU read cache.
// Config keys are few and rarely change, so a small cache with long TTL works well.
func NewCachedStore(inner Store) *CachedStore {
	return &CachedStore{
		inner: inner,
		// 1 bucket × 64 entries, 10 min TTL — more than enough for ~20 config keys
		cache: ecache.NewLRUCache(1, 64, 10*time.Minute),
	}
}

func (c *CachedStore) Get(ctx context.Context, key string) (interface{}, error) {
	if v, ok := c.cache.Get(key); ok {
		return v, nil
	}
	v, err := c.inner.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	c.cache.Put(key, v)
	return v, nil
}

func (c *CachedStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if err := c.inner.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	c.cache.Del(key)
	return nil
}

func (c *CachedStore) Delete(ctx context.Context, key string) error {
	if err := c.inner.Delete(ctx, key); err != nil {
		return err
	}
	c.cache.Del(key)
	return nil
}

func (c *CachedStore) Exists(ctx context.Context, key string) (bool, error) {
	if _, ok := c.cache.Get(key); ok {
		return true, nil
	}
	return c.inner.Exists(ctx, key)
}

func (c *CachedStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	return c.inner.Keys(ctx, pattern)
}

func (c *CachedStore) Clear(ctx context.Context) error {
	if err := c.inner.Clear(ctx); err != nil {
		return err
	}
	c.cache = ecache.NewLRUCache(1, 64, 10*time.Minute)
	return nil
}

func (c *CachedStore) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if err := c.inner.SetJSON(ctx, key, value, ttl); err != nil {
		return err
	}
	// Invalidate so next GetJSON re-reads from DB and caches the fresh value
	c.cache.Del(key)
	return nil
}

// GetJSON retrieves and unmarshals a JSON value, using the cache for the raw string.
func (c *CachedStore) GetJSON(ctx context.Context, key string, dest interface{}) error {
	// Try cache first — stores the raw JSON string
	if v, ok := c.cache.Get(key); ok {
		if s, isStr := v.(string); isStr {
			return json.Unmarshal([]byte(s), dest)
		}
	}

	// Cache miss — read from underlying store via Get (returns raw string)
	v, err := c.inner.Get(ctx, key)
	if err != nil {
		return err
	}

	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("value is not a string")
	}

	// Cache the raw JSON string
	c.cache.Put(key, str)

	return json.Unmarshal([]byte(str), dest)
}
