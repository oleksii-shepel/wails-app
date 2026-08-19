// Package sevenz provides a VFS archive handler for .7z files.
// It uses github.com/bodgit/sevenzip — pure Go, no CGo, no DLL required.
//
// Add to go.mod:
//
//	go get github.com/bodgit/sevenzip
//
// Self-registers via init(); blank-import in app_vfs.go:
//
//	import _ "wails-app/vfs/archive/sevenz"
package sevenz

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	sevenzip "github.com/bodgit/sevenzip"

	"wails-app/vfs"
	"wails-app/vfs/archive"
	"wails-app/vfs/archive/pathutil"
)

func init() {
	archive.Register(&Handler{})
}

// ── Handler ───────────────────────────────────────────────────────────────────

type Handler struct{}

func (h *Handler) Name() string         { return "7z" }
func (h *Handler) Extensions() []string { return []string{".7z"} }
func (h *Handler) Magic() []byte        { return []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C} } // "7z\xBC\xAF'\x1C"
func (h *Handler) CanRead() bool        { return true }
func (h *Handler) CanWrite() bool       { return false }
func (h *Handler) CanCreate() bool      { return false }
func (h *Handler) CanStream() bool      { return false } // 7z requires random access

func (h *Handler) Detect(data []byte) bool {
	return len(data) >= 6 &&
		data[0] == 0x37 && data[1] == 0x7A &&
		data[2] == 0xBC && data[3] == 0xAF &&
		data[4] == 0x27 && data[5] == 0x1C
}

func (h *Handler) Open(ctx context.Context, path string, opts archive.OpenOptions) (archive.Archive, error) {
	if len(opts.NestedChain) > 0 {
		return openNested(ctx, path, opts)
	}
	return openDisk(path, opts.Password)
}

// ── open helpers ──────────────────────────────────────────────────────────────

func openDisk(osPath, password string) (*Archive, error) {
	var r *sevenzip.ReadCloser
	var err error
	if password != "" {
		r, err = sevenzip.OpenReaderWithPassword(osPath, password)
	} else {
		r, err = sevenzip.OpenReader(osPath)
	}
	if err != nil {
		return nil, fmt.Errorf("7z: open %q: %w", osPath, err)
	}
	return &Archive{
		reader:   r,
		osPath:   osPath,
		password: password,
	}, nil
}

func openNested(_ context.Context, osPath string, opts archive.OpenOptions) (*Archive, error) {
	data, err := extractNestedBytes(osPath, opts.NestedChain, opts.Password)
	if err != nil {
		return nil, err
	}
	sr := io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data)))
	var r *sevenzip.Reader
	if opts.Password != "" {
		r, err = sevenzip.NewReaderWithPassword(sr, int64(len(data)), opts.Password)
	} else {
		r, err = sevenzip.NewReader(sr, int64(len(data)))
	}
	if err != nil {
		return nil, fmt.Errorf("7z: open nested %v: %w", opts.NestedChain, err)
	}
	return &Archive{
		memReader: r,
		osPath:    osPath,
		password:  opts.Password,
		chain:     opts.NestedChain,
	}, nil
}

// ── nested resolution ─────────────────────────────────────────────────────────

func extractNestedBytes(osPath string, chain []string, password string) ([]byte, error) {
	current, err := extractEntryFromDisk(osPath, chain[0], password)
	if err != nil {
		return nil, fmt.Errorf("7z: resolve chain[0] %q from %q: %w", chain[0], osPath, err)
	}
	for i := 1; i < len(chain); i++ {
		next, err := extractEntryFromBytes(current, chain[i], password)
		if err != nil {
			return nil, fmt.Errorf("7z: resolve chain[%d] %q: %w", i, chain[i], err)
		}
		current = next
	}
	return current, nil
}

func extractEntryFromDisk(osPath, entryPath, password string) ([]byte, error) {
	var r *sevenzip.ReadCloser
	var err error
	if password != "" {
		r, err = sevenzip.OpenReaderWithPassword(osPath, password)
	} else {
		r, err = sevenzip.OpenReader(osPath)
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return findEntry(r.File, entryPath)
}

func extractEntryFromBytes(data []byte, entryPath, password string) ([]byte, error) {
	sr := io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data)))
	var r *sevenzip.Reader
	var err error
	if password != "" {
		r, err = sevenzip.NewReaderWithPassword(sr, int64(len(data)), password)
	} else {
		r, err = sevenzip.NewReader(sr, int64(len(data)))
	}
	if err != nil {
		return nil, err
	}
	return findEntry(r.File, entryPath)
}

func findEntry(files []*sevenzip.File, entryPath string) ([]byte, error) {
	want := normalizePath(entryPath)
	for _, f := range files {
		if normalizePath(f.Name) == want {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("entry %q not found", entryPath)
}

// ── Archive ───────────────────────────────────────────────────────────────────

// Archive implements archive.Archive for 7z files.
// bodgit/sevenzip supports random access so Open/Stat/ReadDir operate directly
// on the in-memory file list without re-scanning.
type Archive struct {
	reader    *sevenzip.ReadCloser // non-nil for disk archives
	memReader *sevenzip.Reader     // non-nil for nested/in-memory archives

	osPath   string
	password string
	chain    []string // non-empty for nested archives
}

func (a *Archive) files() []*sevenzip.File {
	if a.reader != nil {
		return a.reader.File
	}
	return a.memReader.File
}

// ── vfs.FileSystem ────────────────────────────────────────────────────────────

func (a *Archive) Open(_ context.Context, path string) (vfs.File, error) {
	path = normalizePath(path)
	for _, f := range a.files() {
		if normalizePath(f.Name) == path && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			return &szFile{
				rc:   rc,
				info: &szFileInfo{f: f, chain: a.chain},
			}, nil
		}
	}
	return nil, vfs.ErrNotFound
}

func (a *Archive) Stat(_ context.Context, path string) (vfs.FileInfo, error) {
	path = normalizePath(path)
	if path == "" || path == "." {
		return &szFileInfo{synthetic: true, name: ".", isDir: true, chain: a.chain}, nil
	}
	for _, f := range a.files() {
		if normalizePath(f.Name) == path {
			return &szFileInfo{f: f, chain: a.chain}, nil
		}
	}
	// Implicit directory.
	prefix := path + "/"
	for _, f := range a.files() {
		if strings.HasPrefix(normalizePath(f.Name), prefix) {
			return &szFileInfo{
				synthetic: true,
				name:      filepath.Base(path),
				isDir:     true,
				chain:     a.chain,
			}, nil
		}
	}
	return nil, vfs.ErrNotFound
}

func (a *Archive) ReadDir(_ context.Context, path string) ([]vfs.FileInfo, error) {
	path = normalizePath(path)
	seen := make(map[string]bool)
	var result []vfs.FileInfo

	for _, f := range a.files() {
		ep := normalizePath(f.Name)
		rel := childOf(ep, path)
		if rel == "" {
			continue
		}
		parts := strings.SplitN(rel, "/", 2)
		childName := parts[0]
		if seen[childName] {
			continue
		}
		seen[childName] = true

		if len(parts) == 1 {
			result = append(result, &szFileInfo{f: f, chain: a.chain})
		} else {
			var childPath string
			if path == "" {
				childPath = childName
			} else {
				childPath = path + "/" + childName
			}
			result = append(result, &szFileInfo{
				synthetic: true,
				name:      childName,
				isDir:     true,
				path:      childPath,
				chain:     a.chain,
			})
		}
	}
	return result, nil
}

func (a *Archive) Exists(ctx context.Context, path string) bool {
	info, err := a.Stat(ctx, path)
	return err == nil && info != nil
}

func (a *Archive) Create(_ context.Context, _ string) (vfs.File, error)   { return nil, vfs.ErrReadOnly }
func (a *Archive) Mkdir(_ context.Context, _ string, _ fs.FileMode) error { return vfs.ErrReadOnly }
func (a *Archive) Remove(_ context.Context, _ string) error               { return vfs.ErrReadOnly }
func (a *Archive) Rename(_ context.Context, _, _ string) error            { return vfs.ErrReadOnly }

func (a *Archive) Join(elem ...string) string { return strings.Join(elem, "/") }
func (a *Archive) Split(path string) (string, string) {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "", path
	}
	return path[:i], path[i+1:]
}
func (a *Archive) IsAbs(path string) bool { return strings.HasPrefix(path, "/") }
func (a *Archive) RealPath(_ string) (string, error) {
	if len(a.chain) > 0 {
		return "", nil
	}
	return a.osPath, nil
}

// ── archive.Archive ───────────────────────────────────────────────────────────

func (a *Archive) Comment() string { return "" }
func (a *Archive) IsSolid() bool   { return true } // 7z is typically solid
func (a *Archive) EntryCount() int { return len(a.files()) }

func (a *Archive) Extract(ctx context.Context, srcEntryPath, dstOSPath string) error {
	rc, err := a.Open(ctx, srcEntryPath)
	if err != nil {
		return err
	}
	defer rc.Close()
	if err := os.MkdirAll(filepath.Dir(dstOSPath), 0755); err != nil {
		return err
	}
	out, err := os.Create(dstOSPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

func (a *Archive) ExtractSelected(ctx context.Context, entries []string, dstOSPath string, opts archive.ExtractOptions) (*archive.ExtractResult, error) {
	result := &archive.ExtractResult{}

	var want []string
	for _, e := range entries {
		want = append(want, normalizePath(e))
	}

	for _, f := range a.files() {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		ep := normalizePath(f.Name)
		if f.FileInfo().IsDir() {
			continue
		}
		if !matchesEntries(ep, want) {
			continue
		}

		relPath := rebasePath(ep, opts.StripPrefix)
		if relPath == "" {
			continue
		}

		var dstPath string
		if opts.FlattenPaths {
			dstPath = filepath.Join(dstOSPath, filepath.Base(filepath.FromSlash(relPath)))
		} else {
			dstPath = filepath.Join(dstOSPath, filepath.FromSlash(relPath))
		}

		if _, err := os.Lstat(dstPath); err == nil && opts.OnConflict != nil {
			resolved, skip, err := opts.OnConflict(ep, dstPath)
			if err != nil {
				result.Errors = append(result.Errors, archive.EntryError{Entry: ep, Err: err})
				result.Failed++
				continue
			}
			if skip {
				continue
			}
			dstPath = resolved
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			result.Errors = append(result.Errors, archive.EntryError{Entry: ep, Err: err})
			result.Failed++
			continue
		}

		rc, err := f.Open()
		if err != nil {
			result.Errors = append(result.Errors, archive.EntryError{Entry: ep, Err: err})
			result.Failed++
			continue
		}

		out, err := os.Create(dstPath)
		if err != nil {
			rc.Close()
			result.Errors = append(result.Errors, archive.EntryError{Entry: ep, Err: err})
			result.Failed++
			continue
		}

		n, err := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			result.Errors = append(result.Errors, archive.EntryError{Entry: ep, Err: err})
			result.Failed++
			continue
		}

		if opts.Progress != nil {
			opts.Progress(result.Bytes+n, 0, ep)
		}
		result.Extracted++
		result.Bytes += n
	}
	return result, nil
}

func (a *Archive) Close() error {
	if a.reader != nil {
		return a.reader.Close()
	}
	return nil
}

// ── szFileInfo ────────────────────────────────────────────────────────────────

type szFileInfo struct {
	f         *sevenzip.File
	synthetic bool
	name      string // used when synthetic=true
	isDir     bool   // used when synthetic=true
	path      string // used when synthetic=true
	chain     []string
}

func (s *szFileInfo) Name() string {
	if s.synthetic {
		return s.name
	}
	return filepath.Base(normalizePath(s.f.Name))
}
func (s *szFileInfo) Size() int64 {
	if s.synthetic || s.f == nil {
		return 0
	}
	return s.f.FileInfo().Size()
}
func (s *szFileInfo) Mode() fs.FileMode {
	if s.isDir || (s.f != nil && s.f.FileInfo().IsDir()) {
		return fs.ModeDir | 0755
	}
	return 0644
}
func (s *szFileInfo) ModTime() time.Time {
	if s.synthetic || s.f == nil {
		return time.Time{}
	}
	return s.f.FileInfo().ModTime()
}
func (s *szFileInfo) IsDir() bool {
	if s.synthetic {
		return s.isDir
	}
	return s.f.FileInfo().IsDir()
}
func (s *szFileInfo) Sys() any { return nil }

func (s *szFileInfo) Path() string {
	if s.synthetic {
		return s.path
	}
	if s.f == nil {
		return ""
	}
	return normalizePath(s.f.Name)
}
func (s *szFileInfo) RealPath() string   { return "" }
func (s *szFileInfo) Volume() vfs.Volume { return nil }
func (s *szFileInfo) IsArchive() bool    { return !s.IsDir() && archive.IsArchiveExt(s.Name()) }
func (s *szFileInfo) IsHidden() bool     { return strings.HasPrefix(s.Name(), ".") }
func (s *szFileInfo) IsSymlink() bool    { return false }
func (s *szFileInfo) IsNested() bool     { return len(s.chain) > 0 }
func (s *szFileInfo) MountChain() []string {
	if s.chain == nil {
		return nil
	}
	cp := make([]string, len(s.chain))
	copy(cp, s.chain)
	return cp
}

// ── szFile ────────────────────────────────────────────────────────────────────

type szFile struct {
	rc   io.ReadCloser
	info *szFileInfo
}

func (f *szFile) Read(p []byte) (int, error)  { return f.rc.Read(p) }
func (f *szFile) Write(_ []byte) (int, error) { return 0, vfs.ErrReadOnly }
func (f *szFile) Seek(offset int64, whence int) (int64, error) {
	if s, ok := f.rc.(io.ReadSeekCloser); ok {
		return s.Seek(offset, whence)
	}
	return 0, fmt.Errorf("7z entry does not support seeking")
}
func (f *szFile) Close() error                { return f.rc.Close() }
func (f *szFile) Stat() (vfs.FileInfo, error) { return f.info, nil }
func (f *szFile) Sync() error                 { return nil }

// ── helpers ───────────────────────────────────────────────────────────────────

func normalizePath(p string) string {
	return pathutil.NormalizePath(p)
}

func childOf(ep, parent string) string {
	return pathutil.ChildOf(ep, parent)
}

func matchesEntries(ep string, entries []string) bool {
	return pathutil.MatchesEntries(ep, entries)
}

func rebasePath(ep, stripPrefix string) string {
	return pathutil.RebasePath(ep, stripPrefix)
}

// Compile-time interface checks.
var _ archive.Handler = (*Handler)(nil)
var _ archive.Archive = (*Archive)(nil)
var _ vfs.FileSystem = (*Archive)(nil)