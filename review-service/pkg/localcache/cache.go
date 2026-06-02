package localcache

import (
	"strings"
	"sync"
	"time"
)

type item struct {
	value     []byte
	expiresAt time.Time
}

// Cache 进程内 KV 缓存：map + 读写锁 + 过期时间。
type Cache struct {
	mu    sync.RWMutex
	items map[string]item
}

// New 创建本地缓存实例。
func New() *Cache {
	return &Cache{items: make(map[string]item)}
}

// Get 读取缓存；不存在或已过期返回 false。
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(it.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}
	// 返回副本，避免调用方修改底层 slice。
	out := make([]byte, len(it.value))
	copy(out, it.value)
	return out, true
}

// Set 写入缓存，ttl <= 0 时使用默认 1 分钟。
func (c *Cache) Set(key string, value []byte, ttl time.Duration) {
	if ttl <= 0 {
		ttl = time.Minute
	}
	val := make([]byte, len(value))
	copy(val, value)
	c.mu.Lock()
	c.items[key] = item{
		value:     val,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// Delete 删除单个 key。
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// DeletePrefix 删除所有以 prefix 开头的 key（用于按店铺批量失效）。
func (c *Cache) DeletePrefix(prefix string) {
	c.mu.Lock()
	for k := range c.items {
		if strings.HasPrefix(k, prefix) {
			delete(c.items, k)
		}
	}
	c.mu.Unlock()
}
