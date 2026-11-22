package batch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	contracts "wails-app/internal/contracts"
	internalops "wails-app/internal/ops"
)

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
type OperationResult = contracts.OperationResult

func newTestOp() *internalops.ActiveOperation {
	m := internalops.NewManager()
	return m.StartOperation(contracts.OpCopy, "test", "/src", "/dst")
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestConflictManagerNextID(t *testing.T) {
	m := NewConflictManager()
	id1 := m.NextID("op1")
	id2 := m.NextID("op1")
	if id1 == id2 {
		t.Fatal("expected unique IDs")
	}
	if id1 != "op1-conflict-1" {
		t.Fatalf("unexpected id format: %s", id1)
	}
}

func TestConflictManagerRegisterAndResolve(t *testing.T) {
	m := NewConflictManager()
	ch := m.Register("c1")

	go func() {
		m.Resolve("c1", contracts.ConflictResolutionRequest{Resolution: contracts.ConflictOverwrite})
	}()

	select {
	case res := <-ch:
		if res.Resolution != contracts.ConflictOverwrite {
			t.Fatalf("unexpected resolution: %s", res.Resolution)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for resolution")
	}
}

func TestConflictManagerResolveMissing(t *testing.T) {
	m := NewConflictManager()
	if m.Resolve("missing", contracts.ConflictResolutionRequest{}) {
		t.Fatal("expected false for missing conflict")
	}
}

func TestConflictManagerCancelAll(t *testing.T) {
	m := NewConflictManager()
	ch1 := m.Register("c1")
	ch2 := m.Register("c2")

	m.CancelAll()

	select {
	case res := <-ch1:
		if res.Resolution != contracts.ConflictCancel {
			t.Fatalf("expected cancel, got %s", res.Resolution)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for c1 cancel")
	}
	select {
	case res := <-ch2:
		if res.Resolution != contracts.ConflictCancel {
			t.Fatalf("expected cancel, got %s", res.Resolution)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for c2 cancel")
	}
}

func TestProgressTrackerWithoutReporter(t *testing.T) {
	tracker := NewProgressTracker(nil)
	tracker.Set(1, 10, "first", 10, true)
	tracker.Advance(2, 20, "second", 20, true)
	tracker.Flush()
	if items, bytes := tracker.Snapshot(); items != 3 || bytes != 30 {
		t.Fatalf("snapshot = (%d, %d), want (3, 30)", items, bytes)
	}
}

func TestProgressTrackerSnapshot(t *testing.T) {
	op := newTestOp()
	pt := NewProgressTracker(op)
	pt.Advance(5, 100, "file.txt", 20, true)
	items, bytes := pt.Snapshot()
	if items != 5 || bytes != 100 {
		t.Fatalf("unexpected snapshot: %d items, %d bytes", items, bytes)
	}
}

func TestProgressTrackerFlush(t *testing.T) {
	op := newTestOp()
	pt := NewProgressTracker(op)
	pt.Advance(1, 50, "a.txt", 50, false)
	pt.Flush()
	if op.Progress.ProcessedItems != 1 || op.Progress.ProcessedBytes != 50 {
		t.Fatalf("unexpected progress after flush")
	}
}

func TestProgressTrackerThrottle(t *testing.T) {
	op := newTestOp()
	pt := NewProgressTracker(op)
	pt.throttle = time.Hour           // no natural expiration during test
	pt.Set(1, 10, "a.txt", 10, true)  // force emit to establish baseline
	pt.Set(2, 20, "b.txt", 20, false) // should be throttled
	if op.Progress.ProcessedItems != 1 {
		t.Fatalf("expected throttle to suppress second update, got %d", op.Progress.ProcessedItems)
	}
}

func TestProgressTrackerForce(t *testing.T) {
	op := newTestOp()
	pt := NewProgressTracker(op)
	pt.throttle = time.Hour
	pt.Set(1, 10, "a.txt", 10, false)
	pt.Set(2, 20, "b.txt", 20, true)
	if op.Progress.ProcessedItems != 2 {
		t.Fatalf("expected force to bypass throttle")
	}
}

func TestSameDir(t *testing.T) {
	if !SamePath("/a/b", "/a/b/") {
		t.Fatal("expected same dir")
	}
	if SamePath("/a/b", "/a/c") {
		t.Fatal("expected different dir")
	}
}

func TestSamePath(t *testing.T) {
	if !SamePath("/a/b.txt", "/a/b.txt") {
		t.Fatal("expected same path")
	}
	if SamePath("/a/b.txt", "/a/c.txt") {
		t.Fatal("expected different path")
	}
}

func TestApplyResolution(t *testing.T) {
	tests := []struct {
		res      contracts.ConflictResolution
		wantSkip bool
		wantErr  bool
	}{
		{contracts.ConflictSkip, true, false},
		{contracts.ConflictOverwrite, false, false},
		{contracts.ConflictRename, false, false},
		{contracts.ConflictCancel, true, true},
		{contracts.ConflictResolution("unknown"), true, false},
	}
	for _, tc := range tests {
		dst, skip, err := ApplyResolution(tc.res, "/dst")
		if skip != tc.wantSkip {
			t.Fatalf("ApplyResolution(%s) skip=%v, want %v", tc.res, skip, tc.wantSkip)
		}
		if tc.wantErr && err == nil {
			t.Fatalf("ApplyResolution(%s) expected error", tc.res)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("ApplyResolution(%s) unexpected error: %v", tc.res, err)
		}
		if tc.res == contracts.ConflictCancel && dst != "" {
			t.Fatalf("ApplyResolution(Cancel) expected empty dst")
		}
	}
}

func TestUniqueDestination(t *testing.T) {
	tmp := t.TempDir()
	existing := filepath.Join(tmp, "file.txt")
	if err := os.WriteFile(existing, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := uniqueDestination(existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(got) != "file_copy.txt" {
		t.Fatalf("unexpected unique name: %s", got)
	}

	// Create the _copy variant too
	if err := os.WriteFile(got, []byte("y"), 0644); err != nil {
		t.Fatal(err)
	}
	got2, err := uniqueDestination(existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(got2) != "file_copy2.txt" {
		t.Fatalf("unexpected unique name: %s", got2)
	}
}

func TestPathAvailable(t *testing.T) {
	tmp := t.TempDir()
	existing := filepath.Join(tmp, "exists.txt")
	if err := os.WriteFile(existing, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if pathAvailable(existing) {
		t.Fatal("expected false for existing path")
	}
	if !pathAvailable(filepath.Join(tmp, "new.txt")) {
		t.Fatal("expected true for missing path")
	}
}
