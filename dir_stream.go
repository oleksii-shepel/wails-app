package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const dirStreamBatchSize = 256

type dirStreamMetaEvent struct {
	StreamID   string `json:"streamId"`
	PaneID     string `json:"paneId"`
	Path       string `json:"path"`
	ShowHidden bool   `json:"showHidden"`
}

type dirStreamBatchEvent struct {
	StreamID string     `json:"streamId"`
	PaneID   string     `json:"paneId"`
	Path     string     `json:"path"`
	Entries  []FileInfo `json:"entries"`
}

type dirStreamDoneEvent struct {
	StreamID    string `json:"streamId"`
	PaneID      string `json:"paneId"`
	Path        string `json:"path"`
	EntryCount  int    `json:"entryCount"`
	TotalSize   int64  `json:"totalSize"`
	DurationMS  int64  `json:"durationMs"`
	FromElevate bool   `json:"fromElevate"`
}

type dirStreamErrorEvent struct {
	StreamID string `json:"streamId"`
	PaneID   string `json:"paneId"`
	Path     string `json:"path"`
	Error    string `json:"error"`
}

func (a *App) startListDirectoryStream(paneID, path string, showHidden bool) (string, error) {
	return a.dirStream.Start(paneID, path, showHidden)
}

func (s *DirStreamService) Start(paneID, path string, showHidden bool) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	streamID := fmt.Sprintf("%s-%d", paneID, time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	s.Register(streamID, cancel)
	go s.run(ctx, streamID, paneID, toSlash(path), showHidden)
	return streamID, nil
}

func (s *DirStreamService) run(ctx context.Context, streamID, paneID, path string, showHidden bool) {
	defer s.Unregister(streamID)

	started := time.Now()
	var entryCount int
	var totalSize int64

	s.Emit("fs:listStream:meta", dirStreamMetaEvent{
		StreamID:   streamID,
		PaneID:     paneID,
		Path:       path,
		ShowHidden: showHidden,
	})

	err := scanDirectoryStream(ctx, path, showHidden, dirStreamBatchSize, func(batch []FileInfo, batchSize int64) {
		entryCount += len(batch)
		totalSize += batchSize
		s.Emit("fs:listStream:batch", dirStreamBatchEvent{
			StreamID: streamID,
			PaneID:   paneID,
			Path:     path,
			Entries:  batch,
		})
	})
	if err == nil {
		s.Emit("fs:listStream:done", dirStreamDoneEvent{
			StreamID:   streamID,
			PaneID:     paneID,
			Path:       path,
			EntryCount: entryCount,
			TotalSize:  totalSize,
			DurationMS: time.Since(started).Milliseconds(),
		})
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}

	// Permission denied fallback: stream one elevated listing batch so the UI
	// still completes through the same streaming channel.
	if shouldRetryDeleteWithElevation(err) {
		resp, elevErr := s.runElevatedWithEvents("list", opTargetSingle(path), elevatedRequest{
			Op:         "list",
			Path:       path,
			ShowHidden: showHidden,
		})
		if elevErr == nil && resp.DirListing != nil {
			entries := resp.DirListing.Entries
			if entries == nil {
				entries = []FileInfo{}
			}
			size := int64(0)
			for _, fi := range entries {
				size += fi.Size
			}
			s.Emit("fs:listStream:batch", dirStreamBatchEvent{
				StreamID: streamID,
				PaneID:   paneID,
				Path:     path,
				Entries:  entries,
			})
			s.Emit("fs:listStream:done", dirStreamDoneEvent{
				StreamID:    streamID,
				PaneID:      paneID,
				Path:        path,
				EntryCount:  len(entries),
				TotalSize:   size,
				DurationMS:  time.Since(started).Milliseconds(),
				FromElevate: true,
			})
			return
		}
	}

	s.Emit("fs:listStream:error", dirStreamErrorEvent{
		StreamID: streamID,
		PaneID:   paneID,
		Path:     path,
		Error:    err.Error(),
	})
}

func scanDirectoryStream(
	ctx context.Context,
	path string,
	showHidden bool,
	chunkSize int,
	onBatch func(batch []FileInfo, batchSize int64),
) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		infos, readErr := dir.Readdir(chunkSize)
		if len(infos) > 0 {
			batch := make([]FileInfo, 0, len(infos))
			batchSize := int64(0)
			for _, info := range infos {
				name := info.Name()
				fullPath := toSlash(filepath.Join(path, name))
				hidden := isHiddenFile(name, fullPath)
				if !showHidden && hidden {
					continue
				}
				fi := buildFileInfoWithHidden(name, fullPath, info, hidden)
				batch = append(batch, fi)
				batchSize += fi.Size
			}
			if len(batch) > 0 {
				onBatch(batch, batchSize)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}
