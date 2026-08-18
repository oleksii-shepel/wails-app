package work

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSchedulerDeduplicatesActiveKeys(t *testing.T) {
	scheduler := NewScheduler(1)
	defer scheduler.Close()

	var mu sync.Mutex
	calls := 0

	task := Task{
		Key:      "hash:/a",
		Priority: PriorityBackground,
		Kind:     "hashing",
		Run: func(ctx context.Context, reporter Reporter) error {
			mu.Lock()
			calls++
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			return nil
		},
	}

	first := scheduler.Submit(context.Background(), task)
	second := scheduler.Submit(context.Background(), task)

	if first.task != second.task {
		t.Fatal("expected active task deduplication")
	}
	if err := first.Err(); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestSchedulerReportsProgress(t *testing.T) {
	scheduler := NewScheduler(1)
	defer scheduler.Close()

	handle := scheduler.Submit(context.Background(), Task{
		Priority: PriorityInteractive,
		Kind:     "search",
		Run: func(ctx context.Context, reporter Reporter) error {
			reporter.Report(Progress{Completed: 2, Total: 10, Units: "files"})
			return nil
		},
	})
	if err := handle.Err(); err != nil {
		t.Fatal(err)
	}
	snapshot := handle.Snapshot()
	if snapshot.Progress.Completed != 2 || snapshot.Progress.Total != 10 {
		t.Fatalf("unexpected progress: %+v", snapshot.Progress)
	}
}

func TestSchedulerCancellationUsesContext(t *testing.T) {
	scheduler := NewScheduler(1)
	defer scheduler.Close()

	ctx, cancel := context.WithCancel(context.Background())
	handle := scheduler.Submit(ctx, Task{
		Priority: PriorityBackground,
		Kind:     "directory",
		Run: func(ctx context.Context, reporter Reporter) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})

	cancel()
	if err := handle.Err(); err == nil {
		t.Fatal("expected cancellation error")
	}
	if handle.Snapshot().State != "cancelled" {
		t.Fatalf("state = %q, want cancelled", handle.Snapshot().State)
	}
}
