package filesystem

import (
	"container/list"
	"path/filepath"
	"strings"
	"sync"

	contracts "wails-app/internal/contracts"
)

const DefaultDiskUsageCacheCapacity = 500

type DiskUsageCacheEntry struct {
	path   string
	result *contracts.DiskUsageResult
	elem   *list.Element
}

type DiskUsageCache struct {
	mu      sync.Mutex
	entries map[string]*DiskUsageCacheEntry
	lru     *list.List
	limit   int
}

func NewDiskUsageCache(limit int) *DiskUsageCache {
	if limit < 1 {
		limit = DefaultDiskUsageCacheCapacity
	}
	return &DiskUsageCache{
		entries: make(map[string]*DiskUsageCacheEntry, limit),
		lru:     list.New(),
		limit:   limit,
	}
}

func (c *DiskUsageCache) Get(path string) (*contracts.DiskUsageResult, bool) {
	key := diskUsageCacheKey(path)

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	c.lru.MoveToFront(entry.elem)
	return entry.result, true
}

func (c *DiskUsageCache) Set(result *contracts.DiskUsageResult) {
	if result == nil {
		return
	}
	key := diskUsageCacheKey(result.Path)

	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.entries[key]; ok {
		entry.result = result
		c.lru.MoveToFront(entry.elem)
		return
	}

	if c.lru.Len() >= c.limit {
		c.evict(c.lru.Back().Value.(*DiskUsageCacheEntry))
	}

	entry := &DiskUsageCacheEntry{path: key, result: result}
	entry.elem = c.lru.PushFront(entry)
	c.entries[key] = entry
}

func (c *DiskUsageCache) InvalidateSubtree(path string) {
	root := diskUsageCacheKey(path)

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.entries {
		if key == root || strings.HasPrefix(key, root+"/") {
			c.evict(entry)
		}
	}
}

func (c *DiskUsageCache) evict(entry *DiskUsageCacheEntry) {
	delete(c.entries, entry.path)
	c.lru.Remove(entry.elem)
}

func diskUsageCacheKey(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}