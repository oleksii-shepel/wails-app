package main

import (
	"fmt"
	"math"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StartOperation begins tracking a new long-running operation
func (om *OperationManager) StartOperation(opType OperationType, title, source, dest string) *ActiveOperation {
	om.mu.Lock()
	defer om.mu.Unlock()

	id := fmt.Sprintf("op-%d-%d", time.Now().Unix(), om.nextID)
	om.nextID++

	now := time.Now()
	op := &ActiveOperation{
		ID:   id,
		Type: opType,
		Progress: OperationProgress{
			ID:              id,
			Type:            opType,
			Status:          StatusRunning,
			Title:           title,
			SourcePath:      source,
			DestinationPath: dest,
			StartTime:       now.Unix(),
			TotalItems:      0,
			ProcessedItems:  0,
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
	return op
}

// GetOperation retrieves an active operation
func (om *OperationManager) GetOperation(id string) *ActiveOperation {
	om.mu.RLock()
	defer om.mu.RUnlock()
	return om.operations[id]
}

// ListActiveOperations returns all non-completed operations
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

// CancelOperation requests cancellation of an operation
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
		op.emitUpdate()
		return true
	}
	return false
}

// RemoveOperation deletes a completed operation from tracking
func (om *OperationManager) RemoveOperation(id string) {
	om.mu.Lock()
	delete(om.operations, id)
	om.mu.Unlock()
}

// ActiveOperation methods

func (op *ActiveOperation) SetTotal(totalItems, totalBytes int64) {
	op.mu.Lock()
	defer op.mu.Unlock()
	op.Progress.TotalItems = totalItems
	op.Progress.TotalBytes = totalBytes
	op.emitUpdate()
}

func (op *ActiveOperation) UpdateProgress(processedItems, processedBytes int64, currentFile string, currentFileSize int64) {
	op.mu.Lock()
	defer op.mu.Unlock()

	now := time.Now()
	op.Progress.ProcessedItems = processedItems
	op.Progress.ProcessedBytes = processedBytes
	op.Progress.CurrentFile = currentFile
	op.Progress.CurrentFileSize = currentFileSize

	// Calculate percentage
	if op.Progress.TotalBytes > 0 {
		op.Progress.PercentComplete = math.Min(100, float64(processedBytes)/float64(op.Progress.TotalBytes)*100)
	} else if op.Progress.TotalItems > 0 {
		op.Progress.PercentComplete = math.Min(100, float64(processedItems)/float64(op.Progress.TotalItems)*100)
	}

	// Calculate speed (bytes/sec)
	duration := now.Sub(op.snapshot.lastUpdateTime).Seconds()
	if duration > 0.1 { // Update speed every 100ms minimum
		bytesDelta := processedBytes - op.snapshot.lastUpdateBytes
		instantSpeed := float64(bytesDelta) / duration

		// Keep last 10 samples for averaging
		op.snapshot.speedSamples = append(op.snapshot.speedSamples, instantSpeed)
		if len(op.snapshot.speedSamples) > 10 {
			op.snapshot.speedSamples = op.snapshot.speedSamples[1:]
		}

		// Calculate average
		var sum float64
		for _, s := range op.snapshot.speedSamples {
			sum += s
		}
		op.Progress.AvgSpeedBytesPerSec = sum / float64(len(op.snapshot.speedSamples))
		op.Progress.SpeedBytesPerSec = instantSpeed

		op.snapshot.lastUpdateTime = now
		op.snapshot.lastUpdateBytes = processedBytes
	}

	// Calculate ETA
	elapsed := now.Sub(op.snapshot.timeAtStart).Seconds()
	op.Progress.ElapsedSeconds = int64(elapsed)

	if op.Progress.AvgSpeedBytesPerSec > 0 && op.Progress.TotalBytes > 0 {
		remainingBytes := op.Progress.TotalBytes - processedBytes
		eta := float64(remainingBytes) / op.Progress.AvgSpeedBytesPerSec
		op.Progress.ETASeconds = int64(eta)
	}

	op.emitUpdate()
}

func (op *ActiveOperation) Complete() {
	op.mu.Lock()
	defer op.mu.Unlock()

	now := time.Now().Unix()
	op.Progress.Status = StatusCompleted
	op.Progress.EndTime = &now
	op.Progress.PercentComplete = 100
	op.Progress.ETASeconds = 0
	op.emitUpdate()
}

func (op *ActiveOperation) SetError(err error) {
	op.mu.Lock()
	defer op.mu.Unlock()

	now := time.Now().Unix()
	msg := err.Error()
	op.Progress.Status = StatusError
	op.Progress.EndTime = &now
	op.Progress.Error = &msg
	op.emitUpdate()
}

func (op *ActiveOperation) IsCancelled() bool {
	select {
	case <-op.cancel:
		return true
	default:
		return false
	}
}

func (op *ActiveOperation) emitUpdate() {
	// This will be called from App context
	if appInstance != nil && appInstance.ctx != nil {
		runtime.EventsEmit(appInstance.ctx, "operation:progress", op.Progress)
	}
}

// Global reference for event emission
var appInstance *App
