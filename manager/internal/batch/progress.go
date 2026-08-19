package batch

import (
	"sync"
	"time"

	internalops "wails-app/internal/ops"
)

// ProgressTracker batches frequent progress updates before forwarding them to
// the shared operations runtime.
type ProgressTracker struct {
	op       *internalops.ActiveOperation
	throttle time.Duration
	lastEmit time.Time

	mu             sync.Mutex
	processedItems int64
	processedBytes int64
	currentFile    string
	currentSize    int64
}

func NewProgressTracker(op *internalops.ActiveOperation) *ProgressTracker {
	return &ProgressTracker{
		op:       op,
		throttle: 100 * time.Millisecond,
		lastEmit: time.Now(),
	}
}

func (p *ProgressTracker) Set(items, bytes int64, file string, fileSize int64, force bool) {
	p.mu.Lock()
	p.processedItems = items
	p.processedBytes = bytes
	p.currentFile = file
	p.currentSize = fileSize
	now := time.Now()
	emit := force || now.Sub(p.lastEmit) >= p.throttle
	if emit {
		p.lastEmit = now
	}
	i, b, f, s := p.processedItems, p.processedBytes, p.currentFile, p.currentSize
	p.mu.Unlock()
	if emit {
		p.op.UpdateProgress(i, b, f, s)
	}
}

func (p *ProgressTracker) Flush() {
	p.mu.Lock()
	i, b, f, s := p.processedItems, p.processedBytes, p.currentFile, p.currentSize
	p.mu.Unlock()
	p.op.UpdateProgress(i, b, f, s)
}

func (p *ProgressTracker) Snapshot() (int64, int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.processedItems, p.processedBytes
}

func (p *ProgressTracker) Advance(itemDelta, byteDelta int64, file string, fileSize int64, force bool) (int64, int64) {
	p.mu.Lock()
	p.processedItems += itemDelta
	p.processedBytes += byteDelta
	i, b := p.processedItems, p.processedBytes
	p.mu.Unlock()
	p.Set(i, b, file, fileSize, force)
	return i, b
}