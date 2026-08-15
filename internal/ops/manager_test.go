package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	contracts "wails-app/internal/contracts"
)

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// Helpers
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

type eventRecord struct {
	name string
	data interface{}
}

func newTestManager() (*Manager, *[]eventRecord) {
	var events []eventRecord
	m := NewManager()
	m.SetEmitter(func(name string, data interface{}) {
		events = append(events, eventRecord{name, data})
	})
	return m, &events
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// speedRingBuffer
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestSpeedRingBufferEmptyAvg(t *testing.T) {
	var r speedRingBuffer
	if got := r.avg(); got != 0 {
		t.Fatalf("expected avg 0 for empty buffer, got %f", got)
	}
}

func TestSpeedRingBufferAvg(t *testing.T) {
	var r speedRingBuffer
	r.push(10)
	r.push(20)
	r.push(30)
	if got := r.avg(); got != 20 {
		t.Fatalf("expected avg 20, got %f", got)
	}
}

func TestSpeedRingBufferWrapsAtMax(t *testing.T) {
	var r speedRingBuffer
	for i := 0; i < maxSpeedSamples+5; i++ {
		r.push(float64(i + 1))
	}
	if r.len != maxSpeedSamples {
		t.Fatalf("expected len %d after overflow, got %d", maxSpeedSamples, r.len)
	}
	// Last 10 values pushed were 6..15
	var expected float64
	for i := 6; i <= 15; i++ {
		expected += float64(i)
	}
	expected /= maxSpeedSamples
	if got := r.avg(); got != expected {
		t.Fatalf("expected avg %f, got %f", expected, got)
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// Manager lifecycle
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestStartOperationCreatesRunningOperation(t *testing.T) {
	m, _ := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "Copy files", "/src", "/dst")

	if op == nil {
		t.Fatal("expected non-nil operation")
	}
	if op.ID == "" {
		t.Fatal("expected operation ID to be set")
	}
	if op.Type != contracts.OpCopy {
		t.Fatalf("expected type COPY, got %s", op.Type)
	}
	if op.Progress.Status != contracts.StatusRunning {
		t.Fatalf("expected status RUNNING, got %s", op.Progress.Status)
	}
	if op.Progress.Title != "Copy files" {
		t.Fatalf("expected title 'Copy files', got %s", op.Progress.Title)
	}
	if op.Progress.SourcePath != "/src" || op.Progress.DestinationPath != "/dst" {
		t.Fatalf("unexpected paths: src=%s dst=%s", op.Progress.SourcePath, op.Progress.DestinationPath)
	}
	if op.Progress.StartTime == 0 {
		t.Fatal("expected StartTime to be set")
	}
}

func TestStartOperationEmitsStartEvent(t *testing.T) {
	m, events := newTestManager()
	m.StartOperation(contracts.OpMove, "Move items", "/a", "/b")

	if len(*events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(*events))
	}
	ev := (*events)[0]
	if ev.name != "operation:start" {
		t.Fatalf("expected event 'operation:start', got %s", ev.name)
	}
	se, ok := ev.data.(startEvent)
	if !ok {
		t.Fatalf("expected startEvent, got %T", ev.data)
	}
	if se.Kind != "move" {
		t.Fatalf("expected kind 'move', got %s", se.Kind)
	}
	if se.Title != "Move items" {
		t.Fatalf("expected title 'Move items', got %s", se.Title)
	}
}

func TestGetOperation(t *testing.T) {
	m, _ := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")

	if m.GetOperation(op.ID) == nil {
		t.Fatal("expected to find started operation")
	}
	if m.GetOperation("nonexistent") != nil {
		t.Fatal("expected nil for missing operation")
	}
}

func TestListActiveOperationsFiltersByStatus(t *testing.T) {
	m, _ := newTestManager()
	running := m.StartOperation(contracts.OpCopy, "Running", "", "")
	completed := m.StartOperation(contracts.OpCopy, "Completed", "", "")
	completed.Complete()

	active := m.ListActiveOperations()
	if len(active) != 1 {
		t.Fatalf("expected 1 active operation, got %d", len(active))
	}
	if active[0].ID != running.ID {
		t.Fatalf("expected active op to be the running one")
	}
}

func TestRemoveOperation(t *testing.T) {
	m, _ := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")
	m.RemoveOperation(op.ID)

	if m.GetOperation(op.ID) != nil {
		t.Fatal("expected operation to be removed")
	}
	if len(m.ListActiveOperations()) != 0 {
		t.Fatal("expected no active operations after removal")
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// Cancel
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestCancelRunningOperation(t *testing.T) {
	m, events := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")

	if !m.CancelOperation(op.ID) {
		t.Fatal("expected CancelOperation to return true")
	}
	if op.Progress.Status != contracts.StatusCancelled {
		t.Fatalf("expected status CANCELLED, got %s", op.Progress.Status)
	}
	if !op.IsCancelled() {
		t.Fatal("expected IsCancelled to be true")
	}
	// Start + Complete
	if len(*events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(*events))
	}
	if (*events)[1].name != "operation:complete" {
		t.Fatalf("expected complete event after cancel, got %s", (*events)[1].name)
	}
}

func TestCancelMissingOperation(t *testing.T) {
	m, _ := newTestManager()
	if m.CancelOperation("no-such-id") {
		t.Fatal("expected CancelOperation to return false for missing op")
	}
}

func TestCancelCompletedOperation(t *testing.T) {
	m, _ := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")
	op.Complete()

	if m.CancelOperation(op.ID) {
		t.Fatal("expected CancelOperation to return false for completed op")
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// Progress updates
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestSetTotalUpdatesProgress(t *testing.T) {
	m, events := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")
	op.SetTotal(10, 1024)

	if op.Progress.TotalItems != 10 || op.Progress.TotalBytes != 1024 {
		t.Fatalf("expected total 10 items / 1024 bytes, got %d / %d", op.Progress.TotalItems, op.Progress.TotalBytes)
	}
	// SetTotal re-emits start
	if len(*events) != 2 {
		t.Fatalf("expected 2 events (initial start + re-start), got %d", len(*events))
	}
	se := (*events)[1].data.(startEvent)
	if se.TotalItems != 10 || se.TotalBytes != 1024 {
		t.Fatalf("expected start event to carry updated totals")
	}
}

func TestUpdateProgressCalculatesPercentAndSpeed(t *testing.T) {
	m, events := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")
	op.SetTotal(10, 1000)

	// First update establishes baseline
	op.UpdateProgress(1, 100, "file1.txt", 50)
	if op.Progress.PercentComplete != 10 {
		t.Fatalf("expected 10%%, got %f", op.Progress.PercentComplete)
	}

	// Sleep so duration > 0.1s for speed calculation
	time.Sleep(150 * time.Millisecond)
	op.UpdateProgress(2, 300, "file2.txt", 60)

	if op.Progress.ProcessedItems != 2 || op.Progress.ProcessedBytes != 300 {
		t.Fatalf("unexpected progress values")
	}
	if op.Progress.CurrentFile != "file2.txt" {
		t.Fatalf("expected current file 'file2.txt', got %s", op.Progress.CurrentFile)
	}
	if op.Progress.SpeedBytesPerSec == 0 {
		t.Fatal("expected non-zero instant speed after 0.1s+ gap")
	}
	if op.Progress.AvgSpeedBytesPerSec == 0 {
		t.Fatal("expected non-zero avg speed")
	}

	// Events: start + start (SetTotal) + progress + progress
	if len(*events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(*events))
	}
}

func TestUpdateProgressFallsBackToItemPercent(t *testing.T) {
	m, _ := newTestManager()
	op := m.StartOperation(contracts.OpDelete, "", "", "")
	op.SetTotal(10, 0)
	op.UpdateProgress(5, 0, "", 0)

	if op.Progress.PercentComplete != 50 {
		t.Fatalf("expected 50%% from items, got %f", op.Progress.PercentComplete)
	}
}

func TestUpdateProgressCapsAt100(t *testing.T) {
	m, _ := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")
	op.SetTotal(10, 100)
	op.UpdateProgress(20, 500, "", 0)

	if op.Progress.PercentComplete != 100 {
		t.Fatalf("expected capped 100%%, got %f", op.Progress.PercentComplete)
	}
}

func TestCompleteSetsFinalState(t *testing.T) {
	m, events := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")
	op.Complete()

	if op.Progress.Status != contracts.StatusCompleted {
		t.Fatalf("expected status COMPLETED, got %s", op.Progress.Status)
	}
	if op.Progress.PercentComplete != 100 {
		t.Fatalf("expected 100%%, got %f", op.Progress.PercentComplete)
	}
	if op.Progress.EndTime == nil {
		t.Fatal("expected EndTime to be set")
	}
	if (*events)[1].name != "operation:complete" {
		t.Fatalf("expected complete event, got %s", (*events)[1].name)
	}
}

func TestSetErrorSetsFinalState(t *testing.T) {
	m, events := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")
	op.SetError(os.ErrPermission)

	if op.Progress.Status != contracts.StatusError {
		t.Fatalf("expected status ERROR, got %s", op.Progress.Status)
	}
	if op.Progress.Error == nil || *op.Progress.Error != os.ErrPermission.Error() {
		t.Fatalf("expected error string to be set")
	}
	if op.Progress.EndTime == nil {
		t.Fatal("expected EndTime to be set")
	}
	ce := (*events)[1].data.(completeEvent)
	if ce.Status != "error" {
		t.Fatalf("expected complete status 'error', got %s", ce.Status)
	}
	if ce.Error != os.ErrPermission.Error() {
		t.Fatalf("expected error message in complete event")
	}
}

func TestCancelSignal(t *testing.T) {
	m, _ := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")
	ch := op.CancelSignal()

	select {
	case <-ch:
		t.Fatal("unexpected signal on fresh operation")
	default:
	}

	m.CancelOperation(op.ID)

	select {
	case <-ch:
		// expected
	default:
		t.Fatal("expected signal after cancel")
	}
}

func TestAddTotalBytes(t *testing.T) {
	m, _ := newTestManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")
	op.AddTotalBytes(100)
	op.AddTotalBytes(50)
	if op.Progress.TotalBytes != 150 {
		t.Fatalf("expected TotalBytes 150, got %d", op.Progress.TotalBytes)
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// operationKind
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestOperationKindMapping(t *testing.T) {
	tests := []struct {
		input    contracts.OperationType
		expected string
	}{
		{contracts.OpCopy, "copy"},
		{contracts.OpMove, "move"},
		{contracts.OpDelete, "delete"},
		{contracts.OpArchive, "archive"},
		{contracts.OpExtract, "extract"},
		{contracts.OpSearch, "search"},
		{contracts.OpCalculate, "calculate"},
	}
	for _, tc := range tests {
		if got := operationKind(tc.input); got != tc.expected {
			t.Fatalf("operationKind(%s) = %s, want %s", tc.input, got, tc.expected)
		}
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// CalculateSize
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestCalculateSize(t *testing.T) {
	tmp := t.TempDir()
	// Create nested structure
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(tmp, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("world!!"), 0644); err != nil {
		t.Fatal(err)
	}

	items, bytes := CalculateSize(tmp)
	if items != 4 { // a.txt, sub, sub/b.txt, tmp root counted by walk
		t.Fatalf("expected 4 items, got %d", items)
	}
	if bytes != 12 { // "hello" (5) + "world!!" (7)
		t.Fatalf("expected 12 bytes, got %d", bytes)
	}
}

func TestCalculateSizeMissingPath(t *testing.T) {
	items, bytes := CalculateSize("/nonexistent/path/12345")
	if items != 0 || bytes != 0 {
		t.Fatalf("expected 0 items and 0 bytes for missing path, got %d / %d", items, bytes)
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// SetEmitter retroactive wiring
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestSetEmitterWiresExistingOperations(t *testing.T) {
	m := NewManager()
	op := m.StartOperation(contracts.OpCopy, "", "", "")

	var events []eventRecord
	m.SetEmitter(func(name string, data interface{}) {
		events = append(events, eventRecord{name, data})
	})

	// Trigger an event via the operation to verify wiring
	op.Complete()

	if len(events) != 1 {
		t.Fatalf("expected 1 event after SetEmitter wiring, got %d", len(events))
	}
}
