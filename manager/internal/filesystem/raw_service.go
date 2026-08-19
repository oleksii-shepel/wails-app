package filesystem

import (
	"encoding/base64"
	"io"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"time"

	"wails-app/internal/contentprobe"
	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
)

type MetadataCallbacks struct {
	ToSlash                      func(string) string
	IsHiddenFile                 func(name, path string) bool
	IsArchive                    func(name string) bool
	SymlinkPointsToDir           func(path string) bool
	IsUntrackedDirectoryJunction func(path string) bool
	GetCreatedTime               func(os.FileInfo) int64
	GetAccessedTime              func(os.FileInfo) int64
	FormatPermissions            func(os.FileInfo) string
}

type RawCallbacks struct {
	Metadata    MetadataCallbacks
	MoveToTrash func(path string) error
}

type RawService struct {
	callbacks RawCallbacks
}

func NewRawService(callbacks RawCallbacks) *RawService {
	return &RawService{callbacks: callbacks}
}

func (s *RawService) ListDirectory(path string, showHidden bool) (*contracts.DirectoryListing, error) {
	path = resolveRawPath(path)
	entries, err := os.ReadDir(path)
	if err != nil {
		if resolved, readLinkErr := os.Readlink(path); readLinkErr == nil {
			if resolvedEntries, retryErr := os.ReadDir(resolved); retryErr == nil {
				entries = resolvedEntries
				path = s.toSlash(resolved)
				err = nil
			}
		}
		if err != nil {
			return nil, err
		}
	}

	files := make([]contracts.FileInfo, 0, len(entries))
	var totalSize int64
	for _, entry := range entries {
		name := entry.Name()
		fullPath := s.toSlash(filepath.Join(path, name))
		hidden := s.isHiddenFile(name, fullPath)
		if !showHidden && hidden {
			continue
		}

		info, statErr := os.Lstat(fullPath)
		if statErr != nil {
			if fallback, ok := BuildFallbackLinkInfo(s.callbacks.Metadata, name, fullPath, entry, hidden); ok {
				files = append(files, fallback)
				totalSize += fallback.Size
			}
			continue
		}

		fi := BuildFileInfoWithHidden(s.callbacks.Metadata, name, fullPath, info, hidden)
		files = append(files, fi)
		totalSize += fi.Size
	}

	return &contracts.DirectoryListing{
		Path:      s.toSlash(path),
		Entries:   files,
		TotalSize: totalSize,
	}, nil
}

func (s *RawService) ReadFile(path string, encoding string) (*contracts.FileContent, error) {
	path = resolveRawPath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	probe := contentprobe.ProbeBytes(data)
	if encoding == "" {
		encoding = "utf8"
		if probe.IsText && probe.Encoding != "" {
			encoding = probe.Encoding
		}
	}

	content, err := contentprobe.Decode(data, encoding)
	if err != nil {
		return nil, err
	}

	return &contracts.FileContent{
		Path:     path,
		Content:  content,
		Encoding: encoding,
		Size:     info.Size(),
	}, nil
}

func (s *RawService) ReadFileRange(path string, offset int64, length int) (*contracts.FileContent, error) {
	path = resolveRawPath(path)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	if offset < 0 {
		offset = 0
	}
	if offset > fileSize {
		offset = fileSize
	}
	available := fileSize - offset
	if int64(length) > available {
		length = int(available)
	}
	if length <= 0 {
		return &contracts.FileContent{
			Path:     path,
			Content:  "",
			Encoding: "base64",
			Size:     fileSize,
		}, nil
	}

	buf := make([]byte, length)
	n, err := file.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return &contracts.FileContent{
		Path:     path,
		Content:  base64.StdEncoding.EncodeToString(buf[:n]),
		Encoding: "base64",
		Size:     fileSize,
	}, nil
}

func (s *RawService) ProbeFile(path string) (*contracts.FileProbe, error) {
	path = resolveRawPath(path)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, probe, err := contentprobe.ProbeReader(file)
	if err != nil {
		return nil, err
	}

	return &contracts.FileProbe{
		MimeType:  probe.MimeType,
		Encoding:  probe.Encoding,
		IsText:    probe.IsText,
		IsArchive: probe.IsArchive,
	}, nil
}

func (s *RawService) WriteFile(path string, content string, encoding string) (*contracts.OperationResult, error) {
	path = resolveRawPath(path)
	var data []byte
	var err error
	if encoding == "base64" {
		data, err = base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, err
		}
	} else {
		data = []byte(content)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, err
	}

	return &contracts.OperationResult{
		Success:       true,
		Message:       "File written successfully",
		AffectedPaths: []string{path},
	}, nil
}

func (s *RawService) DeleteFile(path string, recursive bool, useTrash *bool) (*contracts.OperationResult, error) {
	path = resolveRawPath(path)
	trash := useTrash == nil || *useTrash
	if trash {
		if err := s.moveToTrash(path); err != nil {
			return nil, err
		}
		return &contracts.OperationResult{
			Success:       true,
			Message:       "Moved to trash successfully",
			AffectedPaths: []string{path},
		}, nil
	}

	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		if recursive {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
	} else {
		err = os.Remove(path)
	}
	if err != nil {
		return nil, err
	}

	return &contracts.OperationResult{
		Success:       true,
		Message:       "Deleted successfully",
		AffectedPaths: []string{path},
	}, nil
}

func (s *RawService) CreateDirectory(path string, recursive bool) (*contracts.OperationResult, error) {
	path = resolveRawPath(path)
	var err error
	if recursive {
		err = os.MkdirAll(path, 0755)
	} else {
		err = os.Mkdir(path, 0755)
	}
	if err != nil {
		return nil, err
	}

	return &contracts.OperationResult{
		Success:       true,
		Message:       "Directory created",
		AffectedPaths: []string{path},
	}, nil
}

func (s *RawService) MoveFile(source, destination string) (*contracts.OperationResult, error) {
	source = resolveRawPath(source)
	destination = resolveRawPath(destination)
	if err := os.Rename(source, destination); err != nil {
		if copyErr := s.CopyPath(source, destination); copyErr != nil {
			return nil, copyErr
		}
		if removeErr := os.RemoveAll(source); removeErr != nil {
			return nil, removeErr
		}
	}

	return &contracts.OperationResult{
		Success:       true,
		Message:       "Moved successfully",
		AffectedPaths: []string{source, destination},
	}, nil
}

func (s *RawService) CopyFile(source, destination string, recursive bool) (*contracts.OperationResult, error) {
	source = resolveRawPath(source)
	destination = resolveRawPath(destination)
	info, err := os.Stat(source)
	if err != nil {
		return nil, err
	}

	if info.IsDir() && !recursive {
		return &contracts.OperationResult{
			Success: false,
			Message: "Source is a directory; set recursive=true to copy directories",
		}, nil
	}

	if err := s.CopyPath(source, destination); err != nil {
		return nil, err
	}

	return &contracts.OperationResult{
		Success:       true,
		Message:       "Copied successfully",
		AffectedPaths: []string{source, destination},
	}, nil
}

func (s *RawService) CopyPath(src, dst string) error {
	src = resolveRawPath(src)
	dst = resolveRawPath(dst)
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return s.copyDir(src, dst)
	}
	return s.copyFile(src, dst)
}

func (s *RawService) copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func (s *RawService) copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return s.copyFile(path, target)
	})
}

func BuildFileInfo(callbacks MetadataCallbacks, name, fullPath string, info os.FileInfo) contracts.FileInfo {
	return BuildFileInfoWithHidden(callbacks, name, fullPath, info, callHidden(callbacks, name, fullPath))
}

func BuildFileInfoWithHidden(callbacks MetadataCallbacks, name, fullPath string, info os.FileInfo, hidden bool) contracts.FileInfo {
	fileType := contracts.FileTypeFile
	isSymlink := false
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		isSymlink = true
		if callbacks.SymlinkPointsToDir != nil && callbacks.SymlinkPointsToDir(fullPath) {
			fileType = contracts.FileTypeDirectory
		} else {
			fileType = contracts.FileTypeSymlink
		}
	} else if info.IsDir() {
		fileType = contracts.FileTypeDirectory
	} else if callbacks.IsArchive != nil && callbacks.IsArchive(name) {
		fileType = contracts.FileTypeArchive
	}

	if !info.IsDir() && mode&os.ModeSymlink == 0 && callbacks.IsUntrackedDirectoryJunction != nil && callbacks.IsUntrackedDirectoryJunction(fullPath) {
		fileType = contracts.FileTypeDirectory
		isSymlink = true
	}

	return contracts.FileInfo{
		Name:        name,
		Path:        fullPath,
		Type:        fileType,
		MimeType:    inferMimeTypeFromName(name),
		Size:        info.Size(),
		Created:     callCreated(callbacks, info),
		Modified:    info.ModTime().Unix(),
		Accessed:    callAccessed(callbacks, info),
		Permissions: callPermissions(callbacks, info),
		IsHidden:    hidden,
		IsSymlink:   isSymlink,
	}
}

func BuildFallbackLinkInfo(callbacks MetadataCallbacks, name, fullPath string, entry fs.DirEntry, hidden bool) (contracts.FileInfo, bool) {
	mode := entry.Type()
	isSymlink := mode&os.ModeSymlink != 0
	isJunction := callbacks.IsUntrackedDirectoryJunction != nil && callbacks.IsUntrackedDirectoryJunction(fullPath)
	if !isSymlink && !isJunction {
		return contracts.FileInfo{}, false
	}

	fileType := contracts.FileTypeSymlink
	if entry.IsDir() || isJunction || (isSymlink && callbacks.SymlinkPointsToDir != nil && callbacks.SymlinkPointsToDir(fullPath)) {
		fileType = contracts.FileTypeDirectory
	}

	now := time.Now().Unix()
	return contracts.FileInfo{
		Name:        name,
		Path:        fullPath,
		Type:        fileType,
		MimeType:    inferMimeTypeFromName(name),
		Size:        0,
		Created:     now,
		Modified:    now,
		Accessed:    now,
		Permissions: "",
		IsHidden:    hidden,
		IsSymlink:   true,
	}, true
}

func (s *RawService) toSlash(path string) string {
	if s.callbacks.Metadata.ToSlash != nil {
		return s.callbacks.Metadata.ToSlash(path)
	}
	return filepath.ToSlash(path)
}

func (s *RawService) isHiddenFile(name, path string) bool {
	return callHidden(s.callbacks.Metadata, name, path)
}

func (s *RawService) moveToTrash(path string) error {
	path = resolveRawPath(path)
	if s.callbacks.MoveToTrash == nil {
		return os.RemoveAll(path)
	}
	return s.callbacks.MoveToTrash(path)
}

func resolveRawPath(path string) string {
	return pathref.ToNativePath(pathref.Normalize(path))
}

func callHidden(callbacks MetadataCallbacks, name, path string) bool {
	if callbacks.IsHiddenFile == nil {
		return false
	}
	return callbacks.IsHiddenFile(name, path)
}

func callCreated(callbacks MetadataCallbacks, info os.FileInfo) int64 {
	if callbacks.GetCreatedTime == nil {
		return info.ModTime().Unix()
	}
	return callbacks.GetCreatedTime(info)
}

func callAccessed(callbacks MetadataCallbacks, info os.FileInfo) int64 {
	if callbacks.GetAccessedTime == nil {
		return info.ModTime().Unix()
	}
	return callbacks.GetAccessedTime(info)
}

func callPermissions(callbacks MetadataCallbacks, info os.FileInfo) string {
	if callbacks.FormatPermissions == nil {
		return info.Mode().Perm().String()
	}
	return callbacks.FormatPermissions(info)
}

func inferMimeTypeFromName(name string) string {
	return mime.TypeByExtension(filepath.Ext(name))
}