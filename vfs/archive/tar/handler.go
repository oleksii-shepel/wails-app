// Package tar provides a VFS archive handler for .tar, .tar.gz/.tgz,
// .tar.bz2/.tbz2, and .tar.xz/.txz files using only the Go standard library.
//
// TAR is read-only; write/create/delete are not supported.
//
// Self-registers via init(); blank-import in app_vfs.go:
//
//	import _ "wails-app/vfs/archive/tar"
package tar

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wails-app/vfs"
	"wails-app/vfs/archive"
)

func init() {
	archive.Register(&Handler{})
}

// ── Handler ───────────────────────────────────────────────────────────────────

type Handler struct{}

func (h *Handler) Name() string { return "tar" }
func (h *Handler) Extensions() []string {
	return []string{".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tbz2", ".tar.xz", ".txz"}
}

// Magic covers uncompressed TAR (ustar, GNU, POSIX).
// Compressed variants are detected by extension in Detect.
func (h *Handler) Magic() []byte   { return []byte{0x75, 0x73, 0x74, 0x61, 0x72} } // "ustar" at offset 257 – handled via Detect
func (h *Handler) CanRead() bool   { return true }
func (h *Handler) CanWrite() bool  { return false }
func (h *Handler) CanCreate() bool { return false }
func (h *Handler) CanStream() bool { return true }

func (h *Handler) Detect(data []byte) bool {
	// Uncompressed TAR: "ustar" magic at byte offset 257.
	if len(data) >= 262 {
		magic := data[257:262]
		if bytes.HasPrefix(magic, []byte("ustar")) {
			return true
		}
	}
	// gzip magic (covers .tar.gz / .tgz).
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		return true
	}
	// bzip2 magic (covers .tar.bz2 / .tbz2).
	if len(data) >= 3 && data[0] == 0x42 && data[1] == 0x5a && data[2] == 0x68 {
		return true
	}
	// xz magic (covers .tar.xz / .txz).
	if len(data) >= 6 &&
		data[0] == 0xfd && data[1] == 0x37 && data[2] == 0x7a &&
		data[3] == 0x58 && data[4] == 0x5a && data[5] == 0x00 {
		return true
	}
	return false
}

func (h *Handler) Open(ctx context.Context, path string, opts archive.OpenOptions) (archive.Archive, error) {
	if len(opts.NestedChain) > 0 {
		return openNested(ctx, path, opts)
	}
	return openDisk(path)
}

// ── open helpers ──────────────────────────────────────────────────────────────

func openDisk(osPath string) (*Archive, error) {
	entries, err := scanDisk(osPath)
	if err != nil {
		return nil, fmt.Errorf("tar: open %q: %w", osPath, err)
	}
	return &Archive{
		osPath:  osPath,
		entries: entries,
	}, nil
}

func openNested(_ context.Context, osPath string, opts archive.OpenOptions) (*Archive, error) {
	data, err := extractNestedBytes(osPath, opts.NestedChain, opts.Password)
	if err != nil {
		return nil, err
	}
	entries, err := scanReader(bytes.NewReader(data), osPath)
	if err != nil {
		return nil, fmt.Errorf("tar: open nested %v: %w", opts.NestedChain, err)
	}
	return &Archive{
		osPath:  osPath,
		entries: entries,
		memData: data,
		chain:   opts.NestedChain,
	}, nil
}

// ── compression detection ─────────────────────────────────────────────────────

// decompressReader wraps r with the appropriate decompressor based on the
// file extension of osPath, returning an io.ReadCloser.
func decompressReader(r io.Reader, osPath string) (io.ReadCloser, error) {
	lower := strings.ToLower(osPath)
	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		gr, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("tar: gzip open: %w", err)
		}
		return gr, nil
	case strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz2"):
		// compress/bzip2 is read-only and returns a plain io.Reader.
		return io.NopCloser(bzip2.NewReader(r)), nil
	case strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz"):
		// stdlib has no xz support; use a pure-Go shim if available,
		// otherwise return a clear error so callers can fall back.
		return nil, fmt.Errorf("tar: .xz decompression requires an external dependency (e.g. github.com/ulikunitz/xz)")
	default:
		// Plain .tar — no decompression needed.
		return io.NopCloser(r), nil
	}
}

// ── entry scanning ────────────────────────────────────────────────────────────

type tarEntry struct {
	header  *tar.Header
	path    string // normalised forward-slash path
	isDir   bool
	size    int64
	modTime time.Time
}

func scanDisk(osPath string) ([]tarEntry, error) {
	f, err := os.Open(osPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return scanReader(f, osPath)
}

func scanReader(r io.Reader, osPath string) ([]tarEntry, error) {
	dc, err := decompressReader(r, osPath)
	if err != nil {
		return nil, err
	}
	defer dc.Close()
	return collectEntries(tar.NewReader(dc))
}

func collectEntries(tr *tar.Reader) ([]tarEntry, error) {
	var entries []tarEntry
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		p := normalizePath(hdr.Name)
		entries = append(entries, tarEntry{
			header:  hdr,
			path:    p,
			isDir:   hdr.Typeflag == tar.TypeDir,
			size:    hdr.Size,
			modTime: hdr.ModTime,
		})
	}
	return entries, nil
}

// ── nested resolution ─────────────────────────────────────────────────────────

func extractNestedBytes(osPath string, chain []string, _ string) ([]byte, error) {
	current, err := extractEntryFromDisk(osPath, chain[0])
	if err != nil {
		return nil, fmt.Errorf("tar: resolve chain[0] %q from %q: %w", chain[0], osPath, err)
	}
	for i := 1; i < len(chain); i++ {
		next, err := extractEntryFromBytes(current, chain[i], osPath)
		if err != nil {
			return nil, fmt.Errorf("tar: resolve chain[%d] %q: %w", i, chain[i], err)
		}
		current = next
	}
	return current, nil
}

func extractEntryFromDisk(osPath, entryPath string) ([]byte, error) {
	f, err := os.Open(osPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dc, err := decompressReader(f, osPath)
	if err != nil {
		return nil, err
	}
	defer dc.Close()
	return findEntry(tar.NewReader(dc), entryPath)
}

func extractEntryFromBytes(data []byte, entryPath, osPath string) ([]byte, error) {
	dc, err := decompressReader(bytes.NewReader(data), osPath)
	if err != nil {
		return nil, err
	}
	defer dc.Close()
	return findEntry(tar.NewReader(dc), entryPath)
}

func findEntry(tr *tar.Reader, entryPath string) ([]byte, error) {
	want := normalizePath(entryPath)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if normalizePath(hdr.Name) == want {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("entry %q not found", entryPath)
}

// ── Archive ───────────────────────────────────────────────────────────────────

// Archive implements archive.Archive for TAR files.
// archive/tar is forward-only, so we scan all headers on Open and cache them.
// Reading file content requires a second pass through the archive.
type Archive struct {
	osPath  string
	entries []tarEntry
	memData []byte   // non-nil for nested (in-memory) archives
	chain   []string // non-empty for nested archives
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
			return &tarFile{
				reader: bytes.NewReader(data),
				info:   &tarFileInfo{entry: e, chain: a.chain},
			}, nil
		}
	}
	return nil, vfs.ErrNotFound
}

func (a *Archive) Stat(_ context.Context, path string) (vfs.FileInfo, error) {
	path = normalizePath(path)
	if path == "" || path == "." {
		return &tarFileInfo{entry: tarEntry{path: ".", isDir: true}, chain: a.chain}, nil
	}
	for _, e := range a.entries {
		if e.path == path {
			return &tarFileInfo{entry: e, chain: a.chain}, nil
		}
	}
	// Implicit directory (not explicitly stored in the archive).
	prefix := path + "/"
	for _, e := range a.entries {
		if strings.HasPrefix(e.path, prefix) {
			return &tarFileInfo{
				entry: tarEntry{path: path, isDir: true},
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
			result = append(result, &tarFileInfo{entry: e, chain: a.chain})
		} else {
			var childPath string
			if path == "" {
				childPath = childName
			} else {
				childPath = path + "/" + childName
			}
			result = append(result, &tarFileInfo{
				entry: tarEntry{path: childPath, isDir: true},
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
func (a *Archive) IsSolid() bool   { return true } // TAR is inherently sequential/solid
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
// archive/tar does not support seeking, so every read is O(n).
func (a *Archive) readEntry(path string) ([]byte, error) {
	if a.memData != nil {
		dc, err := decompressReader(bytes.NewReader(a.memData), a.osPath)
		if err != nil {
			return nil, err
		}
		defer dc.Close()
		return findEntry(tar.NewReader(dc), path)
	}
	f, err := os.Open(a.osPath)
	if err != nil {
		return nil, fmt.Errorf("tar: re-open for read %q: %w", a.osPath, err)
	}
	defer f.Close()
	dc, err := decompressReader(f, a.osPath)
	if err != nil {
		return nil, err
	}
	defer dc.Close()
	return findEntry(tar.NewReader(dc), path)
}

// ── tarFileInfo ───────────────────────────────────────────────────────────────

type tarFileInfo struct {
	entry tarEntry
	chain []string
}

func (t *tarFileInfo) Name() string { return filepath.Base(t.entry.path) }
func (t *tarFileInfo) Size() int64  { return t.entry.size }
func (t *tarFileInfo) Mode() fs.FileMode {
	if t.entry.header != nil {
		return t.entry.header.FileInfo().Mode()
	}
	if t.entry.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}
func (t *tarFileInfo) ModTime() time.Time { return t.entry.modTime }
func (t *tarFileInfo) IsDir() bool        { return t.entry.isDir }
func (t *tarFileInfo) Sys() any           { return t.entry.header }

func (t *tarFileInfo) Path() string       { return t.entry.path }
func (t *tarFileInfo) RealPath() string   { return "" }
func (t *tarFileInfo) Volume() vfs.Volume { return nil }
func (t *tarFileInfo) IsArchive() bool    { return !t.entry.isDir && isArchiveExt(t.Name()) }
func (t *tarFileInfo) IsHidden() bool     { return strings.HasPrefix(t.Name(), ".") }
func (t *tarFileInfo) IsSymlink() bool {
	return t.entry.header != nil && t.entry.header.Typeflag == tar.TypeSymlink
}
func (t *tarFileInfo) IsNested() bool { return len(t.chain) > 0 }
func (t *tarFileInfo) MountChain() []string {
	if t.chain == nil {
		return nil
	}
	cp := make([]string, len(t.chain))
	copy(cp, t.chain)
	return cp
}

// ── tarFile ───────────────────────────────────────────────────────────────────

type tarFile struct {
	reader *bytes.Reader
	info   *tarFileInfo
}

func (f *tarFile) Read(p []byte) (int, error)  { return f.reader.Read(p) }
func (f *tarFile) Write(_ []byte) (int, error) { return 0, vfs.ErrReadOnly }
func (f *tarFile) Seek(offset int64, whence int) (int64, error) {
	return f.reader.Seek(offset, whence)
}
func (f *tarFile) Close() error                { return nil }
func (f *tarFile) Stat() (vfs.FileInfo, error) { return f.info, nil }
func (f *tarFile) Sync() error                 { return nil }

// ── helpers ───────────────────────────────────────────────────────────────────

func normalizePath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.Trim(p, "/")
	return p
}

func childOf(ep, parent string) string {
	ep = normalizePath(ep)
	parent = normalizePath(parent)
	if parent == "" {
		return ep
	}
	if ep == parent {
		return ""
	}
	prefix := parent + "/"
	if strings.HasPrefix(ep, prefix) {
		return ep[len(prefix):]
	}
	return ""
}

func matchesEntries(ep string, entries []string) bool {
	if len(entries) == 0 {
		return true
	}
	for _, e := range entries {
		if ep == e || strings.HasPrefix(ep, e+"/") {
			return true
		}
	}
	return false
}

func rebasePath(ep, stripPrefix string) string {
	if stripPrefix == "" {
		return ep
	}
	prefix := normalizePath(stripPrefix)
	if ep == prefix {
		return ""
	}
	if strings.HasPrefix(ep, prefix+"/") {
		return ep[len(prefix)+1:]
	}
	return ep
}

func isArchiveExt(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tbz2", ".tar.xz", ".txz", ".rar", ".7z"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// Compile-time interface checks.
var _ archive.Handler = (*Handler)(nil)
var _ archive.Archive = (*Archive)(nil)
var _ vfs.FileSystem = (*Archive)(nil)
