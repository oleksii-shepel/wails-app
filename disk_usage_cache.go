package main

import (
	"container/list"
	"path/filepath"
	"strings"
	"sync"
)

const diskUsageCacheCapacity = 500

type diskUsageCacheEntry struct {
	path   string
	result *DiskUsageResult
	elem   *list.Element
}

type diskUsageCache struct {
	mu      sync.Mutex
	entries map[string]*diskUsageCacheEntry
	lru     *list.List
	limit   int
}

func newDiskUsageCache(limit int) *diskUsageCache {
	if limit < 1 {
		limit = diskUsageCacheCapacity
	}
	return &diskUsageCache{
		entries: make(map[string]*diskUsageCacheEntry, limit),
		lru:     list.New(),
		limit:   limit,
	}
}

func (c *diskUsageCache) Get(path string) (*DiskUsageResult, bool) {
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

func (c *diskUsageCache) Set(result *DiskUsageResult) {
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
		c.evict(c.lru.Back().Value.(*diskUsageCacheEntry))
	}

	entry := &diskUsageCacheEntry{path: key, result: result}
	entry.elem = c.lru.PushFront(entry)
	c.entries[key] = entry
}

func (c *diskUsageCache) InvalidateSubtree(path string) {
	root := diskUsageCacheKey(path)

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.entries {
		if key == root || strings.HasPrefix(key, root+"/") {
			c.evict(entry)
		}
	}
}

func (c *diskUsageCache) evict(entry *diskUsageCacheEntry) {
	delete(c.entries, entry.path)
	c.lru.Remove(entry.elem)
}

func diskUsageCacheKey(path string) string {
	return toSlash(filepath.Clean(path))
}
