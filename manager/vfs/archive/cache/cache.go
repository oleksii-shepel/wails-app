// Package cache provides a bounded LRU cache of open archive.Archive objects.
//
// ┌─────────────────────────────────────────────────────────────────────────┐
// │  TC-style VFS path:  /home/user/data.zip::subdir/inner.tar.gz::deep.rar │
// │                      └──── fsPath ─────┘  └──── nestedChain ──────────┘ │
// └─────────────────────────────────────────────────────────────────────────┘
//
// Design
// ──────
//   - One cache entry per (fsPath + nestedChain) tuple — the "archive identity".
//   - The entry holds the already-opened archive.Archive, alive until evicted.
//   - Lazy: nothing is opened until first access; navigation inside an already-
//     open archive is instant (ReadDir on the cached object, no extraction).
//   - No repeated extraction: the opener is called ONCE per identity, its
//     result is kept alive. Subsequent ReadDir calls for any innerPath reuse it.
//   - Eviction: LRU with a capacity cap + mtime invalidation. When the on-disk
//     file is newer than the cached open time, the entry is silently dropped and
//     re-opened. This keeps nested bytes fresh after archive rewrites.
//   - Thread-safe: one mutex guards the map. Archive.ReadDir is itself safe to
//     call concurrently — handler implementations use immutable entry slices.
package cache

import (
	"container/list"
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"wails-app/vfs/archive"
)

// Capacity is the maximum number of concurrently-open archives.
const Capacity = 12

// Opener is the function the cache calls on a miss to open an archive.
// It maps directly onto resolveChain in app_vfs.go.
type Opener func(ctx context.Context, fsPath string, nestedChain []string, password string) (archive.Archive, error)

// key uniquely identifies one opened archive level.
type key struct {
	fsPath string
	chain  string // nestedChain joined with "::" — cheap comparable string
}

func makeKey(fsPath string, nestedChain []string) key {
	return key{fsPath: fsPath, chain: strings.Join(nestedChain, "::")}
}

// entry is one slot in the LRU.
type entry struct {
	k        key
	arch     archive.Archive
	openedAt time.Time     // wall time when opener was called
	elem     *list.Element // position in the LRU list
}

// Cache is a bounded LRU cache of open archive.Archive objects.
type Cache struct {
	mu      sync.Mutex
	entries map[key]*entry
	lru     *list.List // front = most-recently used
	open    Opener
}

// New returns a Cache that uses opener to open archives on cache misses.
func New(opener Opener) *Cache {
	return &Cache{
		entries: make(map[key]*entry, Capacity),
		lru:     list.New(),
		open:    opener,
	}
}

// Get returns a live archive.Archive for (fsPath, nestedChain), calling the
// opener on first access or after the underlying file has changed on disk.
func (c *Cache) Get(ctx context.Context, fsPath string, nestedChain []string, password string) (archive.Archive, error) {
	k := makeKey(fsPath, nestedChain)

	c.mu.Lock()
	defer c.mu.Unlock()

	// ── Cache hit ─────────────────────────────────────────────────────────────
	if e, ok := c.entries[k]; ok {
		// Invalidate if the outer file has been modified since we opened it.
		if fi, err := os.Stat(fsPath); err == nil && fi.ModTime().After(e.openedAt) {
			c.evict(e) // stale — fall through to re-open
		} else {
			c.lru.MoveToFront(e.elem)
			return e.arch, nil
		}
	}

	// ── Cache miss ────────────────────────────────────────────────────────────
	arch, err := c.open(ctx, fsPath, nestedChain, password)
	if err != nil {
		return nil, err
	}

	if c.lru.Len() >= Capacity {
		c.evict(c.lru.Back().Value.(*entry))
	}

	e := &entry{k: k, arch: arch, openedAt: time.Now()}
	e.elem = c.lru.PushFront(e)
	c.entries[k] = e
	return arch, nil
}

// CloseAll closes every cached archive. Call from App.shutdown().
func (c *Cache) CloseAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.entries {
		_ = e.arch.Close()
	}
	c.entries = make(map[key]*entry)
	c.lru.Init()
}

// evict removes e from the cache and closes the underlying archive.
// Caller must hold c.mu.
func (c *Cache) evict(e *entry) {
	delete(c.entries, e.k)
	c.lru.Remove(e.elem)
	_ = e.arch.Close() // best-effort; tempArchive also removes its temp file
}