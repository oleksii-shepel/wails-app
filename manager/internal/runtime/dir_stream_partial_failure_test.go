package runtime

import (
	"context"
	"errors"
	"testing"

	contracts "wails-app/internal/contracts"
)

func TestVFSPartialFailureKeepsPublishedPages(t *testing.T) {
	service, _ := newTestDirStreamService()
	call := 0
	service.SetVFSPaged(func(
		ctx context.Context,
		path, token string,
		limit int,
	) (*contracts.DirectoryPage, error) {
		call++
		if call == 1 {
			return &contracts.DirectoryPage{
				Path: path,
				Entries: []contracts.FileInfo{
					{Name: "a.txt", Path: path + "/a.txt"},
					{Name: "b.txt", Path: path + "/b.txt"},
				},
				NextToken: "next",
			}, nil
		}
		return nil, errors.New("provider disconnected")
	})

	var batches int
	err := service.scanVFSStream(
		context.Background(),
		"s3://bucket",
		true,
		512,
		func(batch []contracts.FileInfo, _ int64) {
			batches += len(batch)
		},
	)
	if err == nil {
		t.Fatal("expected provider error")
	}
	if batches != 2 {
		t.Fatalf("published entries = %d, want 2", batches)
	}
}