// Package rar provides a VFS archive handler for .rar files (RAR3 and RAR5).
// It uses github.com/nwaples/rardecode which is already a transitive dependency
// via mholt/archiver — no new go.mod entry needed.
//
// RAR is read-only; write/create/delete are not supported (proprietary format).
//
// Self-registers via init(); blank-import in app_vfs.go:
//
//	import _ "wails-app/vfs/archive/rar"
package rar

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

	"github.com/nwaples/rardecode"

	"wails-app/vfs"
	"wails-app/vfs/archive"
	"wails-app/vfs/archive/pathutil"
)

func init() {
	archive.Register(&Handler{})
}

// ── Handler ───────────────────────────────────────────────────────────────────

type Handler struct{}

func (h *Handler) Name() string         { return "rar" }
func (h *Handler) Extensions() []string { return []string{".rar"} }
func (h *Handler) Magic() []byte        { return []byte{0x52, 0x61, 0x72, 0x21} } // "Rar!"
func (h *Handler) CanRead() bool        { return true }
func (h *Handler) CanWrite() bool       { return false }
func (h *Handler) CanCreate() bool      { return false }
func (h *Handler) CanStream() bool      { return true } // rardecode is stream-based

func (h *Handler) Detect(data []byte) bool {
	// RAR4: "Rar!\x1A\x07\x00"
	// RAR5: "Rar!\x1A\x07\x01\x00"
	return len(data) >= 4 && bytes.HasPrefix(data, []byte("Rar!"))
}

func (h *Handler) Open(ctx context.Context, path string, opts archive.OpenOptions) (archive.Archive, error) {
	if len(opts.NestedChain) > 0 {
		return openNested(ctx, path, opts)
	}
	return openDisk(path, opts.Password)
}

// ── open helpers ──────────────────────────────────────────────────────────────

func openDisk(osPath, password string) (*Archive, error) {
	entries, err := scanDisk(osPath, password)
	if err != nil {
		return nil, fmt.Errorf("rar: open %q: %w", osPath, err)
	}
	return &Archive{
		osPath:   osPath,
		password: password,
		entries:  entries,
	}, nil
}

func openNested(_ context.Context, osPath string, opts archive.OpenOptions) (*Archive, error) {
	data, err := extractNestedBytes(osPath, opts.NestedChain, opts.Password)
	if err != nil {
		return nil, err
	}
	entries, err := scanReader(bytes.NewReader(data), opts.Password)
	if err != nil {
		return nil, fmt.Errorf("rar: open nested %v: %w", opts.NestedChain, err)
	}
	return &Archive{
		osPath:   osPath,
		password: opts.Password,
		entries:  entries,
		memData:  data,
		chain:    opts.NestedChain,
	}, nil
}

// ── entry scanning ────────────────────────────────────────────────────────────

// rarEntry is the metadata we cache on open (rardecode is single-pass).
type rarEntry struct {
	header  *rardecode.FileHeader
	path    string // normalised forward-slash path
	isDir   bool
	size    int64
	modTime time.Time
}

func scanDisk(osPath, password string) ([]rarEntry, error) {
	var r *rardecode.ReadCloser
	var err error
	if password != "" {
		r, err = rardecode.OpenReader(osPath, password)
	} else {
		r, err = rardecode.OpenReader(osPath, "")
	}
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return collectEntries(&r.Reader)
}

func scanReader(rd io.Reader, password string) ([]rarEntry, error) {
	r, err := rardecode.NewReader(rd, password)
	if err != nil {
		return nil, err
	}
	return collectEntries(r)
}

func collectEntries(r *rardecode.Reader) ([]rarEntry, error) {
	var entries []rarEntry
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		// Drain the entry so Next() advances cleanly.
		if _, err := io.Copy(io.Discard, r); err != nil {
			return nil, err
		}
		p := normalizePath(hdr.Name)
		entries = append(entries, rarEntry{
			header:  hdr,
			path:    p,
			isDir:   hdr.IsDir,
			size:    hdr.UnPackedSize,
			modTime: hdr.ModificationTime,
		})
	}
	return entries, nil
}

// ── nested resolution ─────────────────────────────────────────────────────────

func extractNestedBytes(osPath string, chain []string, password string) ([]byte, error) {
	current, err := extractEntryFromDisk(osPath, chain[0], password)
	if err != nil {
		return nil, fmt.Errorf("rar: resolve chain[0] %q from %q: %w", chain[0], osPath, err)
	}
	for i := 1; i < len(chain); i++ {
		next, err := extractEntryFromBytes(current, chain[i], password)
		if err != nil {
			return nil, fmt.Errorf("rar: resolve chain[%d] %q: %w", i, chain[i], err)
		}
		current = next
	}
	return current, nil
}

func extractEntryFromDisk(osPath, entryPath, password string) ([]byte, error) {
	r, err := rardecode.OpenReader(osPath, password)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return findEntry(&r.Reader, entryPath)
}

func extractEntryFromBytes(data []byte, entryPath, password string) ([]byte, error) {
	r, err := rardecode.NewReader(bytes.NewReader(data), password)
	if err != nil {
		return nil, err
	}
	return findEntry(r, entryPath)
}

func findEntry(r *rardecode.Reader, entryPath string) ([]byte, error) {
	want := normalizePath(entryPath)
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if normalizePath(hdr.Name) == want {
			return io.ReadAll(r)
		}
		if _, err := io.Copy(io.Discard, r); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("entry %q not found", entryPath)
}

// ── Archive ───────────────────────────────────────────────────────────────────

// Archive implements archive.Archive for RAR files.
// rardecode is forward-only, so we scan all headers on Open and cache them.
// Reading file content requires a second pass through the archive.
type Archive struct {
	osPath   string
	password string
	entries  []rarEntry
	memData  []byte   // non-nil for nested (in-memory) archives
	chain    []string // non-empty for nested archives
}

// ── vfs.FileSystem ────────────────────────────────────────────────────────────

func (a *Archive) Open(_ context.Context, path string) (vfs.File, error) {
	path = normalizePath(path)
	for _, e := range a.entries {
		if e.path == path && !e.isDir {
			data, err := a.readEntry(path)
			if err != nil {
				return nil, err
			}
			return &rarFile{
				reader: bytes.NewReader(data),
				info:   &rarFileInfo{entry: e, chain: a.chain},
			}, nil
		}
	}
	return nil, vfs.ErrNotFound
}

func (a *Archive) Stat(_ context.Context, path string) (vfs.FileInfo, error) {
	path = normalizePath(path)
	if path == "" || path == "." {
		return &rarFileInfo{entry: rarEntry{path: ".", isDir: true}, chain: a.chain}, nil
	}
	for _, e := range a.entries {
		if e.path == path {
			return &rarFileInfo{entry: e, chain: a.chain}, nil
		}
	}
	// Implicit directory.
	prefix := path + "/"
	for _, e := range a.entries {
		if strings.HasPrefix(e.path, prefix) {
			return &rarFileInfo{
				entry: rarEntry{path: path, isDir: true},
				chain: a.chain,
			}, nil
		}
	}
	return nil, vfs.ErrNotFound
}

func (a *Archive) ReadDir(_ context.Context, path string) ([]vfs.FileInfo, error) {
	path = normalizePath(path)
	seen := make(map[string]bool)
	var result []vfs.FileInfo

	for _, e := range a.entries {
		rel := childOf(e.path, path)
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
			result = append(result, &rarFileInfo{entry: e, chain: a.chain})
		} else {
			var childPath string
			if path == "" {
				childPath = childName
			} else {
				childPath = path + "/" + childName
			}
			result = append(result, &rarFileInfo{
				entry: rarEntry{path: childPath, isDir: true},
				chain: a.chain,
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
func (a *Archive) IsSolid() bool   { return false } // rardecode doesn't expose this
func (a *Archive) EntryCount() int { return len(a.entries) }

func (a *Archive) Extract(ctx context.Context, srcEntryPath, dstOSPath string) error {
	data, err := a.readEntry(normalizePath(srcEntryPath))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dstOSPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(dstOSPath, data, 0644)
}

func (a *Archive) ExtractSelected(ctx context.Context, entries []string, dstOSPath string, opts archive.ExtractOptions) (*archive.ExtractResult, error) {
	result := &archive.ExtractResult{}

	// Normalise the wanted set once.
	var want []string
	for _, e := range entries {
		want = append(want, normalizePath(e))
	}

	for _, e := range a.entries {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if e.isDir {
			continue
		}
		if !matchesEntries(e.path, want) {
			continue
		}

		relPath := rebasePath(e.path, opts.StripPrefix)
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
			resolved, skip, err := opts.OnConflict(e.path, dstPath)
			if err != nil {
				result.Errors = append(result.Errors, archive.EntryError{Entry: e.path, Err: err})
				result.Failed++
				continue
			}
			if skip {
				continue
			}
			dstPath = resolved
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			result.Errors = append(result.Errors, archive.EntryError{Entry: e.path, Err: err})
			result.Failed++
			continue
		}

		data, err := a.readEntry(e.path)
		if err != nil {
			result.Errors = append(result.Errors, archive.EntryError{Entry: e.path, Err: err})
			result.Failed++
			continue
		}

		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			result.Errors = append(result.Errors, archive.EntryError{Entry: e.path, Err: err})
			result.Failed++
			continue
		}

		n := int64(len(data))
		if opts.Progress != nil {
			opts.Progress(result.Bytes+n, 0, e.path)
		}
		result.Extracted++
		result.Bytes += n
	}
	return result, nil
}

func (a *Archive) Close() error { return nil }

// ── internal read ─────────────────────────────────────────────────────────────

// readEntry does a fresh single-pass scan to read one entry's content.
// rardecode does not support seeking, so every read is O(n) in the archive.
func (a *Archive) readEntry(path string) ([]byte, error) {
	if a.memData != nil {
		r, err := rardecode.NewReader(bytes.NewReader(a.memData), a.password)
		if err != nil {
			return nil, fmt.Errorf("rar: re-open mem for read %q: %w", path, err)
		}
		return findEntry(r, path)
	}
	r, err := rardecode.OpenReader(a.osPath, a.password)
	if err != nil {
		return nil, fmt.Errorf("rar: re-open for read %q: %w", a.osPath, err)
	}
	defer r.Close()
	return findEntry(&r.Reader, path)
}

// ── rarFileInfo ───────────────────────────────────────────────────────────────

type rarFileInfo struct {
	entry rarEntry
	chain []string
}

func (r *rarFileInfo) Name() string {
	return filepath.Base(r.entry.path)
}
func (r *rarFileInfo) Size() int64 { return r.entry.size }
func (r *rarFileInfo) Mode() fs.FileMode {
	if r.entry.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}
func (r *rarFileInfo) ModTime() time.Time { return r.entry.modTime }
func (r *rarFileInfo) IsDir() bool        { return r.entry.isDir }
func (r *rarFileInfo) Sys() any           { return r.entry.header }

func (r *rarFileInfo) Path() string       { return r.entry.path }
func (r *rarFileInfo) RealPath() string   { return "" }
func (r *rarFileInfo) Volume() vfs.Volume { return nil }
func (r *rarFileInfo) IsArchive() bool    { return !r.entry.isDir && archive.IsArchiveExt(r.Name()) }
func (r *rarFileInfo) IsHidden() bool     { return strings.HasPrefix(r.Name(), ".") }
func (r *rarFileInfo) IsSymlink() bool    { return false }
func (r *rarFileInfo) IsNested() bool     { return len(r.chain) > 0 }
func (r *rarFileInfo) MountChain() []string {
	if r.chain == nil {
		return nil
	}
	cp := make([]string, len(r.chain))
	copy(cp, r.chain)
	return cp
}

// ── rarFile ───────────────────────────────────────────────────────────────────

type rarFile struct {
	reader *bytes.Reader
	info   *rarFileInfo
}

func (f *rarFile) Read(p []byte) (int, error)  { return f.reader.Read(p) }
func (f *rarFile) Write(_ []byte) (int, error) { return 0, vfs.ErrReadOnly }
func (f *rarFile) Seek(offset int64, whence int) (int64, error) {
	return f.reader.Seek(offset, whence)
}
func (f *rarFile) Close() error                { return nil }
func (f *rarFile) Stat() (vfs.FileInfo, error) { return f.info, nil }
func (f *rarFile) Sync() error                 { return nil }

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
