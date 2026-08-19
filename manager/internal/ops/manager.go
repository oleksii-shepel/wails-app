package ops

import (
	"fmt"
	"io/fs"
	"math"
	"path/filepath"
	"sync"
	"time"

	contracts "wails-app/internal/contracts"
)

type startEvent struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Sources     []string `json:"sources"`
	Destination string   `json:"destination,omitempty"`
	TotalBytes  int64    `json:"totalBytes"`
	TotalItems  int64    `json:"totalItems"`
}

type progressEvent struct {
	ID              string  `json:"id"`
	ProcessedBytes  int64   `json:"processedBytes"`
	ProcessedItems  int64   `json:"processedItems"`
	CurrentFile     string  `json:"currentFile,omitempty"`
	CurrentFileSize int64   `json:"currentFileSize,omitempty"`
	SpeedBps        float64 `json:"speedBps"`
	ElapsedMs       int64   `json:"elapsedMs"`
	RemainingMs     int64   `json:"remainingMs"`
}

type completeEvent struct {
	ID           string       `json:"id"`
	Status       string       `json:"status"`
	Error        string       `json:"error,omitempty"`
	FinalMetrics finalMetrics `json:"finalMetrics"`
	Timing       finalTiming  `json:"timing"`
}

type finalMetrics struct {
	TotalBytes     int64 `json:"totalBytes"`
	ProcessedBytes int64 `json:"processedBytes"`
	TotalItems     int64 `json:"totalItems"`
	ProcessedItems int64 `json:"processedItems"`
}

type finalTiming struct {
	StartedAt            int64   `json:"startedAt"`
	ElapsedMs            int64   `json:"elapsedMs"`
	EstimatedRemainingMs int64   `json:"estimatedRemainingMs"`
	SpeedBytesPerSecond  float64 `json:"speedBytesPerSecond"`
}

const maxSpeedSamples = 10

type speedRingBuffer struct {
	buf  [maxSpeedSamples]float64
	head int
	len  int
}

func (r *speedRingBuffer) push(v float64) {
	r.buf[r.head] = v
	r.head = (r.head + 1) % maxSpeedSamples
	if r.len < maxSpeedSamples {
		r.len++
	}
}

func (r *speedRingBuffer) avg() float64 {
	if r.len == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < r.len; i++ {
		idx := (r.head - r.len + i) % maxSpeedSamples
		if idx < 0 {
			idx += maxSpeedSamples
		}
		sum += r.buf[idx]
	}
	return sum / float64(r.len)
}

type progressSnapshot struct {
	timeAtStart     time.Time
	lastUpdateTime  time.Time
	lastUpdateBytes int64
	speedSamples    speedRingBuffer
}

type ActiveOperation struct {
	ID        string
	Type      contracts.OperationType
	Progress  contracts.OperationProgress
	snapshot  progressSnapshot
	cancel    chan struct{}
	mu        sync.RWMutex
	emitEvent func(string, interface{})
}

type Manager struct {
	operations map[string]*ActiveOperation
	mu         sync.RWMutex
	nextID     int
	emitEvent  func(string, interface{})
}

func NewManager() *Manager {
	return &Manager{operations: make(map[string]*ActiveOperation), nextID: 1}
}

func (om *Manager) SetEmitter(emitEvent func(string, interface{})) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.emitEvent = emitEvent
	for _, op := range om.operations {
		op.emitEvent = emitEvent
	}
}

func (om *Manager) StartOperation(opType contracts.OperationType, title, source, dest string) *ActiveOperation {
	om.mu.Lock()
	defer om.mu.Unlock()

	id := fmt.Sprintf("op-%d-%d", time.Now().Unix(), om.nextID)
	om.nextID++
	now := time.Now()
	op := &ActiveOperation{
		ID:        id,
		Type:      opType,
		emitEvent: om.emitEvent,
		Progress: contracts.OperationProgress{
			ID:              id,
			Type:            opType,
			Status:          contracts.StatusRunning,
			Title:           title,
			SourcePath:      source,
			DestinationPath: dest,
			StartTime:       now.UnixMilli(),
		},
		snapshot: progressSnapshot{timeAtStart: now, lastUpdateTime: now},
		cancel:   make(chan struct{}),
	}
	om.operations[id] = op
	op.emitStart(source, dest)
	return op
}

func (om *Manager) GetOperation(id string) *ActiveOperation {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.operations[id]
}

func (om *Manager) ListActiveOperations() []contracts.OperationProgress {
	om.mu.RLock()
	defer om.mu.RUnlock()
	result := make([]contracts.OperationProgress, 0)
	for _, op := range om.operations {
		op.mu.RLock()
		prog := op.Progress
		op.mu.RUnlock()
		if prog.Status == contracts.StatusRunning || prog.Status == contracts.StatusPaused {
			result = append(result, prog)
		}
	}
	return result
}

func (om *Manager) CancelOperation(id string) bool {
	om.mu.Lock()
	op, exists := om.operations[id]
	if !exists {
		om.mu.Unlock()
		return false
	}
	op.mu.Lock()
	om.mu.Unlock()
	defer op.mu.Unlock()
	if op.Progress.Status == contracts.StatusRunning {
		close(op.cancel)
		op.Progress.Status = contracts.StatusCancelled
		op.emitComplete("cancelled", "")
		return true
	}
	return false
}

func (om *Manager) RemoveOperation(id string) {
	om.mu.Lock()
	delete(om.operations, id)
	om.mu.Unlock()
}

func (op *ActiveOperation) SetTotal(totalItems, totalBytes int64) {
	op.mu.Lock()
	defer op.mu.Unlock()
	op.Progress.TotalItems = totalItems
	op.Progress.TotalBytes = totalBytes
	op.emitStart(op.Progress.SourcePath, op.Progress.DestinationPath)
}

func (op *ActiveOperation) UpdateProgress(processedItems, processedBytes int64, currentFile string, currentFileSize int64) {
	op.mu.Lock()
	defer op.mu.Unlock()
	now := time.Now()
	op.Progress.ProcessedItems = processedItems
	op.Progress.ProcessedBytes = processedBytes
	op.Progress.CurrentFile = currentFile
	op.Progress.CurrentFileSize = currentFileSize
	if op.Progress.TotalBytes > 0 {
		op.Progress.PercentComplete = math.Min(100, float64(processedBytes)/float64(op.Progress.TotalBytes)*100)
	} else if op.Progress.TotalItems > 0 {
		op.Progress.PercentComplete = math.Min(100, float64(processedItems)/float64(op.Progress.TotalItems)*100)
	}
	duration := now.Sub(op.snapshot.lastUpdateTime).Seconds()
	if duration > 0.1 {
		bytesDelta := processedBytes - op.snapshot.lastUpdateBytes
		instantSpeed := float64(bytesDelta) / duration
		op.snapshot.speedSamples.push(instantSpeed)
		op.Progress.AvgSpeedBytesPerSec = op.snapshot.speedSamples.avg()
		op.Progress.SpeedBytesPerSec = instantSpeed
		op.snapshot.lastUpdateTime = now
		op.snapshot.lastUpdateBytes = processedBytes
	}
	elapsed := now.Sub(op.snapshot.timeAtStart)
	op.Progress.ElapsedSeconds = int64(elapsed.Seconds())
	var etaMs int64
	if op.Progress.AvgSpeedBytesPerSec > 0 && op.Progress.TotalBytes > 0 {
		remainingBytes := op.Progress.TotalBytes - processedBytes
		etaMs = int64(float64(remainingBytes) / op.Progress.AvgSpeedBytesPerSec * 1000)
		op.Progress.ETASeconds = etaMs / 1000
	}
	op.emitProgress(elapsed.Milliseconds(), etaMs)
}

func (op *ActiveOperation) Complete() {
	op.mu.Lock()
	defer op.mu.Unlock()
	now := time.Now().UnixMilli()
	op.Progress.Status = contracts.StatusCompleted
	op.Progress.EndTime = &now
	op.Progress.PercentComplete = 100
	op.Progress.ETASeconds = 0
	op.emitComplete("completed", "")
}

func (op *ActiveOperation) SetError(err error) {
	op.mu.Lock()
	defer op.mu.Unlock()
	now := time.Now().UnixMilli()
	msg := err.Error()
	op.Progress.Status = contracts.StatusError
	op.Progress.EndTime = &now
	op.Progress.Error = &msg
	op.emitComplete("error", msg)
}

func (op *ActiveOperation) IsCancelled() bool {
	select {
	case <-op.cancel:
		return true
	default:
		return false
	}
}

func (op *ActiveOperation) CancelSignal() <-chan struct{} {
	return op.cancel
}

func (op *ActiveOperation) AddTotalBytes(delta int64) {
	op.mu.Lock()
	defer op.mu.Unlock()
	op.Progress.TotalBytes += delta
}

func (op *ActiveOperation) emitStart(source, dest string) {
	if op.emitEvent == nil {
		return
	}
	op.emitEvent("operation:start", startEvent{ID: op.ID, Kind: operationKind(op.Type), Title: op.Progress.Title, Sources: []string{source}, Destination: dest, TotalBytes: op.Progress.TotalBytes, TotalItems: op.Progress.TotalItems})
}

func (op *ActiveOperation) emitProgress(elapsedMs, remainingMs int64) {
	if op.emitEvent == nil {
		return
	}
	op.emitEvent("operation:progress", progressEvent{ID: op.ID, ProcessedBytes: op.Progress.ProcessedBytes, ProcessedItems: op.Progress.ProcessedItems, CurrentFile: op.Progress.CurrentFile, CurrentFileSize: op.Progress.CurrentFileSize, SpeedBps: op.Progress.AvgSpeedBytesPerSec, ElapsedMs: elapsedMs, RemainingMs: remainingMs})
}

func (op *ActiveOperation) emitComplete(status, errMsg string) {
	if op.emitEvent == nil {
		return
	}
	elapsed := time.Since(op.snapshot.timeAtStart)
	op.emitEvent("operation:complete", completeEvent{ID: op.ID, Status: status, Error: errMsg, FinalMetrics: finalMetrics{TotalBytes: op.Progress.TotalBytes, ProcessedBytes: op.Progress.ProcessedBytes, TotalItems: op.Progress.TotalItems, ProcessedItems: op.Progress.ProcessedItems}, Timing: finalTiming{StartedAt: op.Progress.StartTime, ElapsedMs: elapsed.Milliseconds(), EstimatedRemainingMs: 0, SpeedBytesPerSecond: op.Progress.AvgSpeedBytesPerSec}})
}

func CalculateSize(path string) (items, bytes int64) {
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		items++
		if !d.IsDir() {
			if info, infoErr := d.Info(); infoErr == nil {
				bytes += info.Size()
			}
		}
		return nil
	})
	return
}

func operationKind(t contracts.OperationType) string {
	switch t {
	case contracts.OpCopy:
		return "copy"
	case contracts.OpMove:
		return "move"
	case contracts.OpDelete:
		return "delete"
	case contracts.OpArchive:
		return "archive"
	case contracts.OpExtract:
		return "extract"
	case contracts.OpSearch:
		return "search"
	case contracts.OpCalculate:
		return "calculate"
	default:
		return "copy"
	}
}