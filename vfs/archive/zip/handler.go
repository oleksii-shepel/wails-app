// Package zip provides a VFS archive handler for .zip files.
// It self-registers via init(); blank-import it in main.go:
//
//	import _ "wails-app/vfs/archive/zip"
package zip

import (
	"archive/zip"
	"context"
	"encoding/binary"
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

// ── Handler ──────────────────────────────────────────────────────────────────

type Handler struct{}

func (h *Handler) Name() string         { return "zip" }
func (h *Handler) Extensions() []string { return []string{".zip"} }
func (h *Handler) Magic() []byte        { return []byte{0x50, 0x4B, 0x03, 0x04} }
func (h *Handler) CanRead() bool        { return true }
func (h *Handler) CanWrite() bool       { return true }
func (h *Handler) CanCreate() bool      { return true }
func (h *Handler) CanStream() bool      { return true }

func (h *Handler) Detect(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	sig := binary.LittleEndian.Uint32(data[:4])
	// PK\x03\x04 local-file header OR PK\x05\x06 empty-archive end-of-central-dir
	return sig == 0x504B0304 || sig == 0x504B0506
}

func (h *Handler) Open(ctx context.Context, path string, opts archive.OpenOptions) (archive.Archive, error) {
	if len(opts.NestedChain) > 0 {
		return openNested(ctx, path, opts)
	}
	return openDisk(path, opts.ReadOnly)
}

// openDisk opens a ZIP directly from an OS path.
func openDisk(osPath string, readOnly bool) (*Archive, error) {
	r, err := zip.OpenReader(osPath)
	if err != nil {
		return nil, fmt.Errorf("zip: open %q: %w", osPath, err)
	}
	return &Archive{
		reader:   r,
		osPath:   osPath,
		readOnly: readOnly,
	}, nil
}

// openNested resolves the nested chain and opens the innermost ZIP from memory.
func openNested(_ context.Context, osPath string, opts archive.OpenOptions) (*Archive, error) {
	data, err := extractNestedBytes(osPath, opts.NestedChain)
	if err != nil {
		return nil, err
	}
	r, err := zip.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("zip: open nested %v: %w", opts.NestedChain, err)
	}
	return &Archive{
		readerAt:  strings.NewReader(string(data)),
		memReader: r,
		osPath:    osPath,
		chain:     opts.NestedChain,
		readOnly:  true, // nested archives are always read-only
	}, nil
}

// extractNestedBytes resolves a chain of nested archives entirely in memory.
func extractNestedBytes(osPath string, chain []string) ([]byte, error) {
	// Step 1: extract first link from disk.
	current, err := extractEntryFromDisk(osPath, chain[0])
	if err != nil {
		return nil, fmt.Errorf("zip: resolve chain[0] %q from %q: %w", chain[0], osPath, err)
	}
	// Step 2: recurse through the rest in memory.
	for i := 1; i < len(chain); i++ {
		next, err := extractEntryFromBytes(current, chain[i])
		if err != nil {
			return nil, fmt.Errorf("zip: resolve chain[%d] %q: %w", i, chain[i], err)
		}
		current = next
	}
	return current, nil
}

func extractEntryFromDisk(osPath, entryPath string) ([]byte, error) {
	r, err := zip.OpenReader(osPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	want := normalizePath(entryPath)
	for _, f := range r.File {
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

func extractEntryFromBytes(data []byte, entryPath string) ([]byte, error) {
	r, err := zip.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return nil, err
	}
	want := normalizePath(entryPath)
	for _, f := range r.File {
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

// Archive implements archive.Archive and vfs.FileSystem for ZIP files.
type Archive struct {
	// Exactly one of reader (disk) or memReader (in-memory) is set.
	reader    *zip.ReadCloser
	readerAt  io.ReadSeeker // backing bytes for memReader
	memReader *zip.Reader

	osPath   string
	chain    []string // non-empty for nested archives
	readOnly bool
}

func (a *Archive) files() []*zip.File {
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
			return &zipFile{rc: rc, info: &zipFileInfo{f: f, chain: a.chain}}, nil
		}
	}
	return nil, vfs.ErrNotFound
}

func (a *Archive) Stat(_ context.Context, path string) (vfs.FileInfo, error) {
	path = normalizePath(path)
	if path == "" || path == "." {
		// Root of the archive — synthesize a directory entry.
		return &zipFileInfo{synthetic: true, name: ".", isDir: true, chain: a.chain}, nil
	}
	// Exact entry match.
	for _, f := range a.files() {
		if normalizePath(f.Name) == path {
			return &zipFileInfo{f: f, chain: a.chain}, nil
		}
	}
	// Implicit directory: check if any entry starts with path+"/".
	prefix := path + "/"
	for _, f := range a.files() {
		if strings.HasPrefix(normalizePath(f.Name), prefix) {
			return &zipFileInfo{synthetic: true, name: filepath.Base(path), isDir: true, chain: a.chain}, nil
		}
	}
	return nil, vfs.ErrNotFound
}

func (a *Archive) ReadDir(_ context.Context, path string) ([]vfs.FileInfo, error) {
	path = normalizePath(path)
	// Collect immediate children of path.
	seen := make(map[string]bool)
	var result []vfs.FileInfo

	for _, f := range a.files() {
		ep := normalizePath(f.Name)
		rel := childOf(ep, path)
		if rel == "" {
			continue
		}
		// rel is the immediate child name (no slashes) or "dir/..." (need dir node).
		parts := strings.SplitN(rel, "/", 2)
		childName := parts[0]
		if seen[childName] {
			continue
		}
		seen[childName] = true

		if len(parts) == 1 {
			// Leaf entry — use its actual metadata.
			result = append(result, &zipFileInfo{f: f, chain: a.chain})
		} else {
			// Implicit or explicit directory — synthesize.
			result = append(result, &zipFileInfo{
				synthetic: true,
				name:      childName,
				isDir:     true,
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

// Write operations — all return ErrReadOnly since ZIP rewrite happens at Close().

func (a *Archive) Create(_ context.Context, _ string) (vfs.File, error) {
	if a.readOnly {
		return nil, vfs.ErrReadOnly
	}
	return nil, fmt.Errorf("zip: in-archive Create not yet implemented; use Add after Close")
}

func (a *Archive) Mkdir(_ context.Context, _ string, _ fs.FileMode) error {
	return vfs.ErrReadOnly
}

func (a *Archive) Remove(_ context.Context, _ string) error {
	if a.readOnly {
		return vfs.ErrReadOnly
	}
	// TODO: mark for removal; rewrite on Close.
	return fmt.Errorf("zip: Remove not yet implemented")
}

func (a *Archive) Rename(_ context.Context, _, _ string) error {
	return vfs.ErrReadOnly
}

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
	// Nested archives have no direct OS path for individual entries.
	if len(a.chain) > 0 {
		return "", nil
	}
	return a.osPath, nil
}

// ── archive.Archive ───────────────────────────────────────────────────────────

func (a *Archive) Comment() string { return "" } // zip.ReadCloser exposes comment via r.Comment
func (a *Archive) IsSolid() bool   { return false }
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

	for _, f := range a.files() {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		ep := normalizePath(f.Name)
		if !matchesEntries(ep, entries) {
			continue
		}
		if f.FileInfo().IsDir() {
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

		// Conflict resolution.
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
	return nil // in-memory reader has no Close
}

// ── zipFileInfo ───────────────────────────────────────────────────────────────

type zipFileInfo struct {
	f         *zip.File
	synthetic bool   // true for implicit directories not in the ZIP
	name      string // used when synthetic=true
	isDir     bool   // used when synthetic=true
	chain     []string
}

func (z *zipFileInfo) Name() string {
	if z.synthetic {
		return z.name
	}
	return filepath.Base(normalizePath(z.f.Name))
}
func (z *zipFileInfo) Size() int64 {
	if z.synthetic {
		return 0
	}
	return int64(z.f.UncompressedSize64)
}
func (z *zipFileInfo) Mode() fs.FileMode {
	if z.isDir || (z.f != nil && z.f.FileInfo().IsDir()) {
		return fs.ModeDir | 0755
	}
	return 0644
}
func (z *zipFileInfo) ModTime() time.Time {
	if z.synthetic || z.f == nil {
		return time.Time{}
	}
	return z.f.Modified
}
func (z *zipFileInfo) IsDir() bool {
	if z.synthetic {
		return z.isDir
	}
	return z.f.FileInfo().IsDir()
}
func (z *zipFileInfo) Sys() any { return nil }

func (z *zipFileInfo) Path() string {
	if z.f == nil {
		return ""
	}
	return normalizePath(z.f.Name)
}
func (z *zipFileInfo) RealPath() string   { return "" }
func (z *zipFileInfo) Volume() vfs.Volume { return nil }
func (z *zipFileInfo) IsArchive() bool {
	return !z.IsDir() && isArchiveExt(z.Name())
}
func (z *zipFileInfo) IsHidden() bool  { return strings.HasPrefix(z.Name(), ".") }
func (z *zipFileInfo) IsSymlink() bool { return false }
func (z *zipFileInfo) IsNested() bool  { return len(z.chain) > 0 }
func (z *zipFileInfo) MountChain() []string {
	if z.chain == nil {
		return nil
	}
	cp := make([]string, len(z.chain))
	copy(cp, z.chain)
	return cp
}

// ── zipFile ───────────────────────────────────────────────────────────────────

type zipFile struct {
	rc   io.ReadCloser
	info *zipFileInfo
}

func (f *zipFile) Read(p []byte) (int, error)  { return f.rc.Read(p) }
func (f *zipFile) Write(p []byte) (int, error) { return 0, vfs.ErrReadOnly }
func (f *zipFile) Seek(offset int64, whence int) (int64, error) {
	if s, ok := f.rc.(io.Seeker); ok {
		return s.Seek(offset, whence)
	}
	return 0, fmt.Errorf("zip entry does not support seeking")
}
func (f *zipFile) Close() error                { return f.rc.Close() }
func (f *zipFile) Stat() (vfs.FileInfo, error) { return f.info, nil }
func (f *zipFile) Sync() error                 { return nil }

// ── Helpers ───────────────────────────────────────────────────────────────────

func normalizePath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.Trim(p, "/")
	return p
}

// childOf returns the path of ep relative to parent, or "" if ep is not under parent.
// Example: childOf("a/b/c.txt", "a/b") == "c.txt"
//
//	childOf("a/b/c.txt", "a")  == "b/c.txt"
//	childOf("a/b/c.txt", "")   == "a/b/c.txt"
func childOf(ep, parent string) string {
	parent = normalizePath(parent)
	ep = normalizePath(ep)
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

// matchesEntries reports whether ep matches any of the listed entry paths.
// An empty entries list means "everything".
func matchesEntries(ep string, entries []string) bool {
	if len(entries) == 0 {
		return true
	}
	for _, e := range entries {
		e = normalizePath(e)
		if ep == e || strings.HasPrefix(ep, e+"/") {
			return true
		}
	}
	return false
}

// rebasePath strips stripPrefix from ep.
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
	for _, ext := range []string{".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".rar", ".7z"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// Ensure interface compliance at compile time.
var _ archive.Handler = (*Handler)(nil)
var _ archive.Archive = (*Archive)(nil)
var _ vfs.FileSystem = (*Archive)(nil)
