package filesystem

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	contracts "wails-app/internal/contracts"
	elevation "wails-app/internal/elevation"
	"wails-app/internal/pathref"
	internalruntime "wails-app/internal/runtime"
	vfslib "wails-app/vfs"
)

type elevationClientProvider interface {
	Client() elevation.Client
}

type memoryPathResolver interface {
	ResolvePublicPath(path string) (string, bool, error)
	ToPublicPath(path string) string
}

type ServiceConfig struct {
	Elevation            elevationClientProvider
	EmitEvent            func(string, interface{})
	StartElevatedSession func() (*elevation.SessionStatus, error)

	Raw      *RawService
	Metadata MetadataCallbacks

	IsProcessElevated              func() bool
	ExecuteElevatedRequest         func(elevation.Request) elevation.Response
	RunElevatedRequest             func(elevation.Request) (elevation.Response, error)
	IsPermissionDenied             func(error) bool
	ShouldRetryDeleteWithElevation func(error) bool
	MemoryResolver                 memoryPathResolver
}

type Service struct {
	elevation            elevationClientProvider
	emitEvent            func(string, interface{})
	startElevatedSession func() (*elevation.SessionStatus, error)
	raw                  *RawService
	metadata             MetadataCallbacks
	vfs                  *vfslib.Unified
	duStreams            *duStreamRegistry
	duCache              *DiskUsageCache

	isProcessElevated              func() bool
	executeElevatedRequest         func(elevation.Request) elevation.Response
	runElevatedRequest             func(elevation.Request) (elevation.Response, error)
	isPermissionDenied             func(error) bool
	shouldRetryDeleteWithElevation func(error) bool
	memoryResolver                 memoryPathResolver
}

func NewService(config ServiceConfig) *Service {
	return &Service{
		elevation:                      config.Elevation,
		emitEvent:                      config.EmitEvent,
		startElevatedSession:           config.StartElevatedSession,
		raw:                            config.Raw,
		metadata:                       config.Metadata,
		duStreams:                      newDuStreamRegistry(),
		duCache:                        NewDiskUsageCache(DefaultDiskUsageCacheCapacity),
		isProcessElevated:              config.IsProcessElevated,
		executeElevatedRequest:         config.ExecuteElevatedRequest,
		runElevatedRequest:             config.RunElevatedRequest,
		isPermissionDenied:             config.IsPermissionDenied,
		shouldRetryDeleteWithElevation: config.ShouldRetryDeleteWithElevation,
		memoryResolver:                 config.MemoryResolver,
	}
}

func (s *Service) resolvePublicPath(path string) (string, error) {
	normalized := pathref.Normalize(path)
	nativePath := pathref.ToNativePath(normalized)
	if s.memoryResolver == nil || !pathref.IsMemory(normalized) {
		return nativePath, nil
	}
	resolved, _, err := s.memoryResolver.ResolvePublicPath(nativePath)
	return resolved, err
}

func (s *Service) toPublicPath(path string) string {
	if s.memoryResolver == nil {
		return pathref.FromNativePath(filepath.ToSlash(path))
	}
	return pathref.FromNativePath(s.memoryResolver.ToPublicPath(path))
}

func (s *Service) mapFileInfo(info contracts.FileInfo) contracts.FileInfo {
	info.Path = s.toPublicPath(info.Path)
	if info.SearchBasePath != "" {
		info.SearchBasePath = s.toPublicPath(info.SearchBasePath)
	}
	if info.SearchParentPath != "" {
		info.SearchParentPath = s.toPublicPath(info.SearchParentPath)
	}
	return info
}

func (s *Service) mapListing(listing *contracts.DirectoryListing, requestedPath string) *contracts.DirectoryListing {
	if listing == nil {
		return nil
	}
	if requestedPath != "" {
		listing.Path = pathref.Normalize(requestedPath)
	} else {
		listing.Path = s.toPublicPath(listing.Path)
	}
	for index := range listing.Entries {
		listing.Entries[index] = s.mapFileInfo(listing.Entries[index])
	}
	return listing
}

func (s *Service) mapFileContent(file *contracts.FileContent, requestedPath string) *contracts.FileContent {
	if file == nil {
		return nil
	}
	if requestedPath != "" {
		file.Path = pathref.Normalize(requestedPath)
	} else {
		file.Path = s.toPublicPath(file.Path)
	}
	return file
}

func (s *Service) mapOperation(op *contracts.OperationResult) *contracts.OperationResult {
	if op == nil {
		return nil
	}
	for index := range op.AffectedPaths {
		op.AffectedPaths[index] = s.toPublicPath(op.AffectedPaths[index])
	}
	return op
}

func (s *Service) SetVFS(v *vfslib.Unified) {
	s.vfs = v
}

func (s *Service) handlesVFSPath(path string) bool {
	if s.vfs == nil {
		return false
	}
	normalized := pathref.Normalize(path)
	if pathref.IsAlias(normalized) {
		return s.vfs.IsMounted(pathref.ToVFS(normalized))
	}
	return s.vfs.IsMounted(normalized)
}

func (s *Service) emit(name string, data interface{}) {
	if s.emitEvent != nil {
		s.emitEvent(name, data)
	}
}

func (s *Service) elevationTargetSingle(path string) string {
	return path
}

func (s *Service) elevationTargetPair(source, destination string) string {
	return fmt.Sprintf("%s -> %s", source, destination)
}

func (s *Service) ListDirectory(path string, showHidden bool) (*contracts.DirectoryListing, error) {
	path = pathref.Normalize(path)
	if pathref.IsSearch(path) {
		return s.listSearchAlias(path)
	}
	if s.handlesVFSPath(path) {
		listing, err := internalruntime.VFSListDir(s.vfs, path)
		if err == nil {
			return listing, nil
		}
		if !pathref.IsRegistry(path) || !s.isPermissionDenied(err) {
			return nil, err
		}

		resp, elevErr := s.runElevatedWithEvents("list", s.elevationTargetSingle(path), elevation.Request{
			Op:         "list",
			Path:       path,
			ShowHidden: showHidden,
		})
		if elevErr != nil {
			return nil, err
		}
		return resp.DirListing, nil
	}
	if s.vfs != nil && s.vfs.IsMounted(path) {
		return internalruntime.VFSListDir(s.vfs, path)
	}

	resolvedPath, err := s.resolvePublicPath(path)
	if err != nil {
		return nil, err
	}
	listing, err := s.raw.ListDirectory(resolvedPath, showHidden)
	if err == nil {
		return s.mapListing(listing, path), nil
	}
	if !s.shouldRetryDeleteWithElevation(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("list", s.elevationTargetSingle(path), elevation.Request{
		Op:         "list",
		Path:       resolvedPath,
		ShowHidden: showHidden,
	})
	if elevErr != nil {
		return nil, err
	}
	return s.mapListing(resp.DirListing, path), nil
}

func (s *Service) listSearchAlias(path string) (*contracts.DirectoryListing, error) {
	if path == pathref.SearchScheme {
		return &contracts.DirectoryListing{
			Path:      pathref.SearchScheme,
			Entries:   []contracts.FileInfo{},
			TotalSize: 0,
		}, nil
	}

	query, ok := pathref.ParseSearch(path)
	if !ok {
		return nil, fmt.Errorf("invalid search alias path")
	}

	result, err := s.SearchFiles(query.BasePath, query.Pattern, query.Recursive)
	if err != nil {
		return nil, err
	}

	entries := append([]contracts.FileInfo{}, result.Matches...)
	var total int64
	for _, entry := range entries {
		total += entry.Size
	}

	return &contracts.DirectoryListing{
		Path:      path,
		Entries:   entries,
		TotalSize: total,
	}, nil
}

func (s *Service) ElevatedList(path string, showHidden bool) ([]contracts.FileInfo, error) {
	resp, err := s.runElevatedWithEvents("list", path, elevation.Request{
		Op:         "list",
		Path:       path,
		ShowHidden: showHidden,
	})
	if err != nil || resp.DirListing == nil {
		return nil, err
	}
	return resp.DirListing.Entries, nil
}

func (s *Service) DirSizeRecursive(path string) (int64, error) {
	resolvedPath, err := s.resolvePublicPath(path)
	if err != nil {
		return 0, err
	}
	var total int64
	queue := []string{resolvedPath}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		entries, err := os.ReadDir(current)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			full := filepath.Join(current, entry.Name())
			if entry.IsDir() {
				queue = append(queue, full)
			} else {
				info, err := entry.Info()
				if err == nil {
					total += info.Size()
				}
			}
		}
	}
	return total, nil
}

func (s *Service) GetFileInfo(path string) (*contracts.FileInfo, error) {
	resolvedPath, err := s.resolvePublicPath(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(resolvedPath)
	fi := BuildFileInfo(s.metadata, name, filepath.ToSlash(resolvedPath), info)
	fi.Path = path
	return &fi, nil
}

func (s *Service) ReadFile(path string, encoding string) (*contracts.FileContent, error) {
	path = pathref.Normalize(path)
	if s.handlesVFSPath(path) {
		file, err := internalruntime.VFSReadFile(s.vfs, path, encoding)
		if err == nil {
			return file, nil
		}
		if !pathref.IsRegistry(path) || !s.isPermissionDenied(err) {
			return nil, err
		}

		resp, elevErr := s.runElevatedWithEvents("read", s.elevationTargetSingle(path), elevation.Request{
			Op:       "read",
			Path:     path,
			Encoding: encoding,
		})
		if elevErr != nil {
			return nil, err
		}
		return resp.File, nil
	}

	resolvedPath, err := s.resolvePublicPath(path)
	if err != nil {
		return nil, err
	}
	file, err := s.raw.ReadFile(resolvedPath, encoding)
	if err == nil {
		return s.mapFileContent(file, path), nil
	}
	if !s.isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("read", s.elevationTargetSingle(path), elevation.Request{
		Op:       "read",
		Path:     resolvedPath,
		Encoding: encoding,
	})
	if elevErr != nil {
		return nil, err
	}
	return s.mapFileContent(resp.File, path), nil
}

func (s *Service) WriteFile(path string, content string, encoding string) (*contracts.OperationResult, error) {
	path = pathref.Normalize(path)
	if s.handlesVFSPath(path) {
		op, err := internalruntime.VFSWriteFile(s.vfs, path, content, encoding)
		if err == nil {
			return op, nil
		}
		if !pathref.IsRegistry(path) || !s.isPermissionDenied(err) {
			return nil, err
		}

		resp, elevErr := s.runElevatedWithEvents("write", s.elevationTargetSingle(path), elevation.Request{
			Op:       "write",
			Path:     path,
			Content:  content,
			Encoding: encoding,
		})
		if elevErr != nil {
			return nil, err
		}
		return resp.Operation, nil
	}

	resolvedPath, err := s.resolvePublicPath(path)
	if err != nil {
		return nil, err
	}
	op, err := s.raw.WriteFile(resolvedPath, content, encoding)
	if err == nil {
		return s.mapOperation(op), nil
	}
	if !s.isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("write", s.elevationTargetSingle(path), elevation.Request{
		Op:       "write",
		Path:     resolvedPath,
		Content:  content,
		Encoding: encoding,
	})
	if elevErr != nil {
		return nil, err
	}
	return s.mapOperation(resp.Operation), nil
}

func (s *Service) DeleteFile(path string, recursive bool, useTrash *bool) (*contracts.OperationResult, error) {
	path = pathref.Normalize(path)
	if s.handlesVFSPath(path) {
		op, err := internalruntime.VFSRemove(s.vfs, path)
		if err == nil {
			return op, nil
		}
		if !pathref.IsRegistry(path) || !s.isPermissionDenied(err) {
			return nil, err
		}

		resp, elevErr := s.runElevatedWithEvents("delete", s.elevationTargetSingle(path), elevation.Request{
			Op:        "delete",
			Path:      path,
			Recursive: recursive,
			UseTrash:  useTrash,
		})
		if elevErr != nil {
			return nil, err
		}
		return resp.Operation, nil
	}

	resolvedPath, err := s.resolvePublicPath(path)
	if err != nil {
		return nil, err
	}
	op, err := s.raw.DeleteFile(resolvedPath, recursive, useTrash)
	if err == nil {
		return s.mapOperation(op), nil
	}
	if !s.isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("delete", s.elevationTargetSingle(path), elevation.Request{
		Op:        "delete",
		Path:      resolvedPath,
		Recursive: recursive,
		UseTrash:  useTrash,
	})
	if elevErr != nil {
		return nil, err
	}
	return s.mapOperation(resp.Operation), nil
}

func (s *Service) CreateDirectory(path string, recursive bool) (*contracts.OperationResult, error) {
	path = pathref.Normalize(path)
	if s.handlesVFSPath(path) {
		op, err := internalruntime.VFSMkdir(s.vfs, path)
		if err == nil {
			return op, nil
		}
		if !pathref.IsRegistry(path) || !s.isPermissionDenied(err) {
			return nil, err
		}

		resp, elevErr := s.runElevatedWithEvents("mkdir", s.elevationTargetSingle(path), elevation.Request{
			Op:   "mkdir",
			Path: path,
		})
		if elevErr != nil {
			return nil, err
		}
		return resp.Operation, nil
	}

	resolvedPath, err := s.resolvePublicPath(path)
	if err != nil {
		return nil, err
	}
	op, err := s.raw.CreateDirectory(resolvedPath, recursive)
	if err == nil {
		return s.mapOperation(op), nil
	}
	if !s.isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("mkdir", s.elevationTargetSingle(path), elevation.Request{
		Op:        "mkdir",
		Path:      resolvedPath,
		Recursive: recursive,
	})
	if elevErr != nil {
		return nil, err
	}
	return s.mapOperation(resp.Operation), nil
}

func (s *Service) MoveFile(source, destination string) (*contracts.OperationResult, error) {
	resolvedSource, err := s.resolvePublicPath(source)
	if err != nil {
		return nil, err
	}
	resolvedDestination, err := s.resolvePublicPath(destination)
	if err != nil {
		return nil, err
	}
	op, err := s.raw.MoveFile(resolvedSource, resolvedDestination)
	if err == nil {
		return s.mapOperation(op), nil
	}
	if !s.isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("move", s.elevationTargetPair(source, destination), elevation.Request{
		Op:          "move",
		Source:      resolvedSource,
		Destination: resolvedDestination,
	})
	if elevErr != nil {
		return nil, err
	}
	return s.mapOperation(resp.Operation), nil
}

func (s *Service) CopyFile(source, destination string, recursive bool) (*contracts.OperationResult, error) {
	resolvedSource, err := s.resolvePublicPath(source)
	if err != nil {
		return nil, err
	}
	resolvedDestination, err := s.resolvePublicPath(destination)
	if err != nil {
		return nil, err
	}
	op, err := s.raw.CopyFile(resolvedSource, resolvedDestination, recursive)
	if err == nil {
		return s.mapOperation(op), nil
	}
	if !s.isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("copy", s.elevationTargetPair(source, destination), elevation.Request{
		Op:          "copy",
		Source:      resolvedSource,
		Destination: resolvedDestination,
		Recursive:   recursive,
	})
	if elevErr != nil {
		return nil, err
	}
	return s.mapOperation(resp.Operation), nil
}

func (s *Service) RunElevatedCopy(src, dst string, recursive bool) (bool, error) {
	resp, err := s.runElevatedWithEvents("copy", s.elevationTargetPair(src, dst), elevation.Request{
		Op:          "copy",
		Source:      src,
		Destination: dst,
		Recursive:   recursive,
	})
	return err == nil && resp.Success, err
}

func (s *Service) RunElevatedMove(src, dst string) (bool, error) {
	resp, err := s.runElevatedWithEvents("move", s.elevationTargetPair(src, dst), elevation.Request{
		Op:          "move",
		Source:      src,
		Destination: dst,
	})
	return err == nil && resp.Success, err
}

func (s *Service) RunElevatedDelete(path string, recursive bool, useTrash *bool) (bool, error) {
	resp, err := s.runElevatedWithEvents("delete", s.elevationTargetSingle(path), elevation.Request{
		Op:        "delete",
		Path:      path,
		Recursive: recursive,
		UseTrash:  useTrash,
	})
	return err == nil && resp.Success, err
}

func (s *Service) SearchFiles(path, pattern string, recursive bool) (*contracts.SearchResult, error) {
	var matches []contracts.FileInfo
	basePath := filepath.ToSlash(pathref.Normalize(path))
	resolvedPath, err := s.resolvePublicPath(path)
	if err != nil {
		return nil, err
	}

	walkFn := func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p == resolvedPath {
			return nil
		}
		if !recursive && d.IsDir() && p != resolvedPath {
			return filepath.SkipDir
		}

		name := d.Name()
		matched, err := pathref.MatchSearchPattern(pattern, name)
		if err != nil {
			return err
		}
		if !matched {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		entry := BuildFileInfo(s.metadata, name, filepath.ToSlash(p), info)
		entry.Path = s.toPublicPath(entry.Path)
		entry.SearchBasePath = basePath
		entry.SearchPattern = pattern
		entry.SearchParentPath = s.toPublicPath(filepath.ToSlash(filepath.Dir(p)))
		matches = append(matches, entry)
		return nil
	}

	if err := filepath.WalkDir(resolvedPath, walkFn); err != nil {
		return nil, err
	}

	if matches == nil {
		matches = []contracts.FileInfo{}
	}

	return &contracts.SearchResult{
		Path:         basePath,
		Matches:      matches,
		TotalMatches: len(matches),
	}, nil
}

type elevationEvent struct {
	RequestID string `json:"requestId,omitempty"`
	Phase     string `json:"phase"`
	Operation string `json:"operation"`
	Target    string `json:"target"`
	Message   string `json:"message,omitempty"`
	Success   bool   `json:"success,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (s *Service) emitElevationRequested(requestID, op, target string) {
	s.emit("elevation:requested", elevationEvent{
		RequestID: requestID,
		Phase:     "requested",
		Operation: op,
		Target:    target,
		Message:   "Administrative privileges required. Confirm to show OS elevation prompt.",
	})
}

func (s *Service) emitElevationResult(requestID, op, target string, success bool, err error) {
	evt := elevationEvent{
		RequestID: requestID,
		Phase:     "result",
		Operation: op,
		Target:    target,
		Success:   success,
	}
	if err != nil {
		evt.Error = err.Error()
		evt.Message = "Elevation failed or was cancelled."
	} else {
		evt.Message = "Elevated operation completed."
	}
	s.emit("elevation:result", evt)
}

func (s *Service) runElevatedWithEvents(op, target string, req elevation.Request) (elevation.Response, error) {
	if s.isProcessElevated != nil && s.isProcessElevated() {
		return s.executeElevatedRequest(req), nil
	}

	var svc elevation.Client
	if s.elevation != nil {
		svc = s.elevation.Client()
	}

	if (svc == nil || !svc.Alive()) && s.startElevatedSession != nil {
		s.emit("elevation:session-starting", map[string]string{"op": op, "target": target})
		if status, startErr := s.startElevatedSession(); startErr == nil {
			s.emit("elevation:session-started", status)
			if s.elevation != nil {
				svc = s.elevation.Client()
			}
		}
	}

	if svc != nil && svc.Alive() {
		resp, err := svc.Dispatch(req)
		if err != nil {
			s.emit("elevation:auto-failed", map[string]interface{}{"op": op, "target": target, "error": err.Error()})
			return elevation.Response{}, err
		}
		s.emit("elevation:auto-started", map[string]string{"op": op, "target": target})
		return resp, nil
	}

	s.emit("elevation:uac-requested", map[string]string{
		"phase":     "requested",
		"operation": op,
		"target":    target,
	})
	resp, err := s.runElevatedRequest(req)
	if err != nil {
		s.emit("elevation:auto-failed", map[string]interface{}{"op": op, "target": target, "error": err.Error()})
		return elevation.Response{}, err
	}
	s.emit("elevation:auto-started", map[string]string{"op": op, "target": target})
	return resp, nil
}
