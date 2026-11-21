package filesystem

import (
	"testing"
	"time"

	contracts "wails-app/internal/contracts"
)

func newDiskUsageTestService() *Service {
	return &Service{
		emitEvent: func(string, interface{}) {},
		duStreams: newDuStreamRegistry(),
		duCache:   NewDiskUsageCache(4),
	}
}

func TestFinishDiskUsageStreamSkipsCacheWhenCancelled(t *testing.T) {
	s := newDiskUsageTestService()
	done := make(chan struct{})
	close(done)

	if completed := s.finishDiskUsageStream("du-cancelled", done, t.TempDir(), "/partial", nil, time.Now()); completed {
		t.Fatal("cancelled stream must not report completion")
	}
	if _, ok := s.duCache.Get("/partial"); ok {
		t.Fatal("cancelled scan must not poison the disk usage cache")
	}
}

func TestFinishDiskUsageStreamCachesCompletedScan(t *testing.T) {
	s := newDiskUsageTestService()
	done := make(chan struct{})
	entries := []contracts.DiskUsageEntry{{Name: "a.txt", Size: 10}}

	if completed := s.finishDiskUsageStream("du-done", done, "C:/root", "/public", entries, time.Now()); !completed {
		t.Fatal("completed stream must report completion")
	}
	if _, ok := s.duCache.Get("/public"); !ok {
		t.Fatal("completed scan should be cached")
	}
}
