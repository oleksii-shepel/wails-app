package app

import (
	"context"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"strings"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	internalruntime "wails-app/internal/runtime"
)

func (s *State) handlesBatchVFSPath(path string) bool {
	if s.vfs == nil {
		return false
	}
	normalized := pathref.Normalize(path)
	if pathref.IsAlias(normalized) {
		return s.vfs.IsMounted(pathref.ToVFS(normalized))
	}
	return s.vfs.IsMounted(normalized)
}

func (s *State) copyBatchPath(
	source string,
	destination string,
	recursive bool,
	streamToFile func(io.Reader, string, int64, string) (int64, error),
) error {
	if s.vfs == nil {
		return fmt.Errorf("virtual filesystem is not available")
	}

	source = pathref.Normalize(source)
	destination = pathref.Normalize(destination)
	if !s.handlesBatchVFSPath(source) {
		return fmt.Errorf("source is not a mounted virtual path")
	}

	info, err := internalruntime.VFSGetFileInfo(s.vfs, source)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if info.Type == contracts.FileTypeDirectory {
		if !recursive {
			return fmt.Errorf("source is a directory; use recursive=true")
		}
		return s.copyBatchVFSDirectory(ctx, source, destination, streamToFile)
	}

	return s.copyBatchVFSFile(ctx, source, destination, info, streamToFile)
}

func (s *State) copyBatchVFSDirectory(
	ctx context.Context,
	source string,
	destination string,
	streamToFile func(io.Reader, string, int64, string) (int64, error),
) error {
	if err := s.ensureBatchDestinationDir(ctx, destination); err != nil {
		return err
	}

	listing, err := internalruntime.VFSListDir(s.vfs, source)
	if err != nil {
		return err
	}

	for _, entry := range listing.Entries {
		childDestination := joinBatchPath(destination, entry.Name)
		if entry.Type == contracts.FileTypeDirectory {
			if err := s.copyBatchVFSDirectory(ctx, entry.Path, childDestination, streamToFile); err != nil {
				return err
			}
			continue
		}

		entryCopy := entry
		if err := s.copyBatchVFSFile(ctx, entry.Path, childDestination, &entryCopy, streamToFile); err != nil {
			return err
		}
	}

	return nil
}

func (s *State) copyBatchVFSFile(
	ctx context.Context,
	source string,
	destination string,
	info *contracts.FileInfo,
	streamToFile func(io.Reader, string, int64, string) (int64, error),
) error {
	reader, err := s.vfs.Open(ctx, pathref.ToVFS(source))
	if err != nil {
		return err
	}
	defer reader.Close()

	if s.handlesBatchVFSPath(destination) {
		return s.writeBatchVFSFile(ctx, destination, reader)
	}

	if streamToFile == nil {
		return fmt.Errorf("stream copy callback is not available")
	}

	_, err = streamToFile(reader, pathref.ToNativePath(destination), info.Size, info.Name)
	return err
}

func (s *State) writeBatchVFSFile(ctx context.Context, destination string, reader io.Reader) error {
	internalPath := pathref.ToVFS(destination)
	if err := s.ensureVFSDir(ctx, pathpkg.Dir(internalPath)); err != nil {
		return err
	}

	file, err := s.vfs.Create(ctx, internalPath)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return err
	}
	return file.Sync()
}

func (s *State) ensureBatchDestinationDir(ctx context.Context, destination string) error {
	if s.handlesBatchVFSPath(destination) {
		return s.ensureVFSDir(ctx, pathref.ToVFS(destination))
	}
	return os.MkdirAll(pathref.ToNativePath(destination), 0o755)
}

func (s *State) ensureVFSDir(ctx context.Context, internalPath string) error {
	clean := cleanBatchVFSPath(internalPath)
	if clean == "/" || clean == "." {
		return nil
	}
	if s.vfs.Exists(ctx, clean) {
		return nil
	}

	parent := pathpkg.Dir(clean)
	if parent != clean {
		if err := s.ensureVFSDir(ctx, parent); err != nil {
			return err
		}
	}

	if err := s.vfs.Mkdir(ctx, clean, 0o755); err != nil && !s.vfs.Exists(ctx, clean) {
		return err
	}
	return nil
}

func cleanBatchVFSPath(path string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	if normalized == "" {
		return "/"
	}
	return pathpkg.Clean("/" + strings.TrimPrefix(normalized, "/"))
}

func joinBatchPath(base string, child string) string {
	trimmedBase := strings.TrimRight(pathref.Normalize(base), "/")
	trimmedChild := strings.TrimLeft(strings.ReplaceAll(child, "\\", "/"), "/")
	if trimmedChild == "" {
		return trimmedBase
	}
	return pathref.Normalize(trimmedBase + "/" + trimmedChild)
}
