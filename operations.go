package main

import (
	"fmt"
	"math"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// Event payloads — shapes must match operation.types.ts exactly
// ============================================================================

type opStartEvent struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Sources     []string `json:"sources"`
	Destination string   `json:"destination,omitempty"`
	TotalBytes  int64    `json:"totalBytes"`
	TotalItems  int64    `json:"totalItems"`
}

type opProgressEvent struct {
	ID              string  `json:"id"`
	ProcessedBytes  int64   `json:"processedBytes"`
	ProcessedItems  int64   `json:"processedItems"`
	CurrentFile     string  `json:"currentFile,omitempty"`
	CurrentFileSize int64   `json:"currentFileSize,omitempty"`
	SpeedBps        float64 `json:"speedBps"`
	ElapsedMs       int64   `json:"elapsedMs"`
	RemainingMs     int64   `json:"remainingMs"`
}

type opCompleteEvent struct {
	ID           string         `json:"id"`
	Status       string         `json:"status"`
	Error        string         `json:"error,omitempty"`
	FinalMetrics opFinalMetrics `json:"finalMetrics"`
	Timing       opFinalTiming  `json:"timing"`
}

type opFinalMetrics struct {
	TotalBytes     int64 `json:"totalBytes"`
	ProcessedBytes int64 `json:"processedBytes"`
	TotalItems     int64 `json:"totalItems"`
	ProcessedItems int64 `json:"processedItems"`
}

type opFinalTiming struct {
	StartedAt            int64   `json:"startedAt"`
	ElapsedMs            int64   `json:"elapsedMs"`
	EstimatedRemainingMs int64   `json:"estimatedRemainingMs"`
	SpeedBytesPerSecond  float64 `json:"speedBytesPerSecond"`
}

// ============================================================================
// operationKind maps internal OperationType to frontend OperationKind string
// ============================================================================

func operationKind(t OperationType) string {
	switch t {
	case OpCopy:
		return "copy"
	case OpMove:
		return "move"
	case OpDelete:
		return "delete"
	case OpArchive:
		return "archive"
	case OpExtract:
		return "extract"
	case OpSearch:
		return "search"
	default:
		return "copy"
	}
}

// ============================================================================
// OperationManager methods
// ============================================================================

// StartOperation begins tracking a new long-running operation and emits
// the "operation:start" event the frontend expects.
func (om *OperationManager) StartOperation(opType OperationType, title, source, dest string) *ActiveOperation {
	om.mu.Lock()
	defer om.mu.Unlock()

	id := fmt.Sprintf("op-%d-%d", time.Now().Unix(), om.nextID)
	om.nextID++

	now := time.Now()
	op := &ActiveOperation{
		ID:   id,
		Type: opType,
		app:  om.app,
		Progress: OperationProgress{
			ID:              id,
			Type:            opType,
			Status:          StatusRunning,
			Title:           title,
			SourcePath:      source,
			DestinationPath: dest,
			StartTime:       now.UnixMilli(),
		},
		snapshot: progressSnapshot{
			timeAtStart:     now,
			lastUpdateTime:  now,
			lastUpdateBytes: 0,
			speedSamples:    make([]float64, 0, 10),
		},
		cancel: make(chan struct{}),
	}

	om.operations[id] = op

	// Emit start immediately. TotalBytes/TotalItems will be 0 until SetTotal
	// is called — the frontend shows indeterminate progress until then.
	op.emitStart(source, dest)

	return op
}

// GetOperation retrieves an active operation.
func (om *OperationManager) GetOperation(id string) *ActiveOperation {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.operations[id]
}

// ListActiveOperations returns all non-completed operations.
func (om *OperationManager) ListActiveOperations() []OperationProgress {
	om.mu.RLock()
	defer om.mu.RUnlock()

	result := make([]OperationProgress, 0)
	for _, op := range om.operations {
		op.mu.RLock()
		prog := op.Progress
		op.mu.RUnlock()

		if prog.Status == StatusRunning || prog.Status == StatusPaused {
			result = append(result, prog)
		}
	}
	return result
}

// CancelOperation requests cancellation of an operation.
func (om *OperationManager) CancelOperation(id string) bool {
	om.mu.Lock()
	op, exists := om.operations[id]
	om.mu.Unlock()

	if !exists {
		return false
	}

	op.mu.Lock()
	defer op.mu.Unlock()

	if op.Progress.Status == StatusRunning {
		close(op.cancel)
		op.Progress.Status = StatusCancelled
		op.emitComplete("cancelled", "")
		return true
	}
	return false
}

// RemoveOperation deletes a completed operation from tracking.
func (om *OperationManager) RemoveOperation(id string) {
	om.mu.Lock()
	delete(om.operations, id)
	om.mu.Unlock()
}

// ============================================================================
// ActiveOperation methods
// ============================================================================

// SetTotal updates the totals once they are known and re-emits the start event
// so the frontend can switch from indeterminate to determinate progress.
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

	// Percentage
	if op.Progress.TotalBytes > 0 {
		op.Progress.PercentComplete = math.Min(100, float64(processedBytes)/float64(op.Progress.TotalBytes)*100)
	} else if op.Progress.TotalItems > 0 {
		op.Progress.PercentComplete = math.Min(100, float64(processedItems)/float64(op.Progress.TotalItems)*100)
	}

	// Speed — only recalculate every 100 ms to avoid noise
	duration := now.Sub(op.snapshot.lastUpdateTime).Seconds()
	if duration > 0.1 {
		bytesDelta := processedBytes - op.snapshot.lastUpdateBytes
		instantSpeed := float64(bytesDelta) / duration

		op.snapshot.speedSamples = append(op.snapshot.speedSamples, instantSpeed)
		if len(op.snapshot.speedSamples) > 10 {
			op.snapshot.speedSamples = op.snapshot.speedSamples[1:]
		}

		var sum float64
		for _, s := range op.snapshot.speedSamples {
			sum += s
		}
		op.Progress.AvgSpeedBytesPerSec = sum / float64(len(op.snapshot.speedSamples))
		op.Progress.SpeedBytesPerSec = instantSpeed

		op.snapshot.lastUpdateTime = now
		op.snapshot.lastUpdateBytes = processedBytes
	}

	// Elapsed / ETA
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
	op.Progress.Status = StatusCompleted
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
	op.Progress.Status = StatusError
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

// ============================================================================
// Internal event emitters — use op.app instead of a global variable
// ============================================================================

func (op *ActiveOperation) emitStart(source, dest string) {
	if op.app == nil || op.app.ctx == nil {
		return
	}
	runtime.EventsEmit(op.app.ctx, "operation:start", opStartEvent{
		ID:          op.ID,
		Kind:        operationKind(op.Type),
		Title:       op.Progress.Title,
		Sources:     []string{source},
		Destination: dest,
		TotalBytes:  op.Progress.TotalBytes,
		TotalItems:  op.Progress.TotalItems,
	})
}

func (op *ActiveOperation) emitProgress(elapsedMs, remainingMs int64) {
	if op.app == nil || op.app.ctx == nil {
		return
	}
	runtime.EventsEmit(op.app.ctx, "operation:progress", opProgressEvent{
		ID:              op.ID,
		ProcessedBytes:  op.Progress.ProcessedBytes,
		ProcessedItems:  op.Progress.ProcessedItems,
		CurrentFile:     op.Progress.CurrentFile,
		CurrentFileSize: op.Progress.CurrentFileSize,
		SpeedBps:        op.Progress.AvgSpeedBytesPerSec,
		ElapsedMs:       elapsedMs,
		RemainingMs:     remainingMs,
	})
}

func (op *ActiveOperation) emitComplete(status, errMsg string) {
	if op.app == nil || op.app.ctx == nil {
		return
	}
	elapsed := time.Since(op.snapshot.timeAtStart)
	runtime.EventsEmit(op.app.ctx, "operation:complete", opCompleteEvent{
		ID:     op.ID,
		Status: status,
		Error:  errMsg,
		FinalMetrics: opFinalMetrics{
			TotalBytes:     op.Progress.TotalBytes,
			ProcessedBytes: op.Progress.ProcessedBytes,
			TotalItems:     op.Progress.TotalItems,
			ProcessedItems: op.Progress.ProcessedItems,
		},
		Timing: opFinalTiming{
			StartedAt:            op.Progress.StartTime,
			ElapsedMs:            elapsed.Milliseconds(),
			EstimatedRemainingMs: 0,
			SpeedBytesPerSecond:  op.Progress.AvgSpeedBytesPerSec,
		},
	})
}
