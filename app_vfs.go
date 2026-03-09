package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	vfslib "wails-app/vfs"
	"wails-app/vfs/archive"
	"wails-app/vfs/mount"
	"wails-app/vfs/osfs"

	_ "wails-app/vfs/archive/7z"  // register 7z handler
	_ "wails-app/vfs/archive/rar" // register rar handler
	_ "wails-app/vfs/archive/tar" // register tar/tar.gz/tar.bz2 handler
	_ "wails-app/vfs/archive/zip" // register zip handler
)

// initVFS sets up the unified VFS. Call at the end of App.startup().
func (a *App) initVFS() {
	mgr := mount.NewManager(osfs.New("/"))
	a.vfs = vfslib.NewUnified(mgr)
}

// ── Wails-exposed methods ─────────────────────────────────────────────────────

// VFSListDir lists a directory through the VFS — works for real directories
// and paths inside mounted archives.
func (a *App) VFSListDir(path string) (*DirectoryListing, error) {
	entries, err := a.vfs.ReadDir(context.Background(), path)
	if err != nil {
		return nil, err
	}

	files := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		files = append(files, convertVFSInfo(e))
	}

	var total int64
	for _, f := range files {
		total += f.Size
	}

	return &DirectoryListing{
		Path:      path,
		Entries:   files,
		TotalSize: total,
	}, nil
}

// VFSOpenArchive mounts an archive (and optional nested chain) into the VFS
// and returns an ArchiveListing whose Entries show the top-level contents.
//
// path   — real OS path of the outermost archive file.
// nested — ordered inner archive entry paths, e.g. ["inner.tar.gz"].
func (a *App) VFSOpenArchive(path string, nested []string) (ArchiveListing, error) {
	ctx := context.Background()

	mountPoint, err := a.mountArchive(ctx, path, nested)
	if err != nil {
		return ArchiveListing{}, err
	}

	// List top-level entries inside the freshly mounted archive.
	entries, err := a.vfs.ReadDir(ctx, mountPoint)
	if err != nil {
		return ArchiveListing{}, err
	}

	files := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		files = append(files, convertVFSInfoWithChain(e, nested))
	}

	return ArchiveListing{
		ArchivePath: path,
		InnerPath:   mountPoint,
		NestedChain: nested,
		Entries:     files,
	}, nil
}

// VFSMountArchive mounts a top-level archive and returns its VFS mount point.
// The frontend uses the returned path as the root for VFSListDir calls.
func (a *App) VFSMountArchive(path string) (string, error) {
	return a.mountArchive(context.Background(), path, nil)
}

// VFSUnmount unmounts a previously mounted archive and frees its resources.
func (a *App) VFSUnmount(mountPoint string) error {
	return a.vfs.Unmount(mountPoint)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// mountArchive resolves the full nested chain and mounts the innermost archive.
func (a *App) mountArchive(ctx context.Context, osPath string, nested []string) (string, error) {
	arch, err := resolveChain(ctx, osPath, nested, "")
	if err != nil {
		return "", fmt.Errorf("vfs: open archive %q (chain %v): %w", osPath, nested, err)
	}

	mountPoint := archiveMountPoint(osPath, nested)

	if _, err := a.vfs.Mount(mountPoint, arch, true, filepath.Base(osPath), vfslib.VolumeArchive); err != nil {
		_ = arch.Close()
		return "", fmt.Errorf("vfs: mount %q → %q: %w", osPath, mountPoint, err)
	}

	return mountPoint, nil
}

// resolveChain opens a (possibly cross-format) nested archive chain and returns
// the innermost archive ready to browse.
//
// It works by peeling one level at a time, using the correct handler for each
// level's format.  No handler ever has to understand another format.
//
// Example — outer.zip contains subdir/inner.tar.gz which contains deep.rar:
//
//	osPath = "/data/outer.zip"
//	chain  = ["subdir/inner.tar.gz", "deep.rar"]
//
//	Step 1: open outer.zip with the zip handler (disk open)
//	Step 2: extract bytes of "subdir/inner.tar.gz" from the zip archive
//	Step 3: open those bytes as tar with the tar handler (openFromMemory)
//	Step 4: extract bytes of "deep.rar" from the tar archive
//	Step 5: open those bytes as rar with the rar handler (openFromMemory) → returned
func resolveChain(ctx context.Context, osPath string, chain []string, password string) (archive.Archive, error) {
	// ── Level 0: open top-level archive from disk ────────────────────────────
	h0, ok := archive.ForPath(osPath)
	if !ok {
		return nil, fmt.Errorf("no archive handler for %q", osPath)
	}

	if len(chain) == 0 {
		// No nesting — plain disk open.
		return h0.Open(ctx, osPath, archive.OpenOptions{ReadOnly: true, Password: password})
	}

	outer, err := h0.Open(ctx, osPath, archive.OpenOptions{ReadOnly: true, Password: password})
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", osPath, err)
	}

	// Extract the first nested archive's bytes from the outermost archive.
	data, err := readEntryBytes(ctx, outer, chain[0])
	outer.Close()
	if err != nil {
		return nil, fmt.Errorf("extract %q from %q: %w", chain[0], osPath, err)
	}

	// ── Levels 1..n: peel in memory ──────────────────────────────────────────
	for i := 1; i < len(chain); i++ {
		// chain[i-1] is the name (and thus format) of the archive we hold as bytes.
		mid, err := openFromMemory(ctx, chain[i-1], data, password)
		if err != nil {
			return nil, fmt.Errorf("open level %d %q from bytes: %w", i, chain[i-1], err)
		}
		next, err := readEntryBytes(ctx, mid, chain[i])
		mid.Close()
		if err != nil {
			return nil, fmt.Errorf("extract %q from level %d (%q): %w", chain[i], i, chain[i-1], err)
		}
		data = next
	}

	// Open the innermost archive from the final extracted bytes.
	return openFromMemory(ctx, chain[len(chain)-1], data, password)
}

// openFromMemory opens an archive whose bytes are already in memory.
// name is used only to select the correct handler by extension.
//
// Writes to a temp file so every handler can use its normal disk-open path.
// The temp file is deleted automatically when the returned archive is closed.
// This only happens for nested archives (rare), so the temp-file round-trip
// is acceptable.
func openFromMemory(ctx context.Context, name string, data []byte, password string) (archive.Archive, error) {
	h, ok := archive.ForPath(name)
	if !ok {
		return nil, fmt.Errorf("no archive handler for %q", name)
	}

	// Write to a temp file so the handler can use its normal disk-open path.
	// This is O(size-of-entry) disk I/O and is only needed for nested archives,
	// which are rare.  The temp file is deleted when the returned archive closes.
	f, err := os.CreateTemp("", "vfs-nested-*"+filepath.Ext(name))
	if err != nil {
		return nil, fmt.Errorf("temp file for %q: %w", name, err)
	}
	tmpPath := f.Name()

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return nil, fmt.Errorf("write temp %q: %w", name, err)
	}
	f.Close()

	arch, err := h.Open(ctx, tmpPath, archive.OpenOptions{ReadOnly: true, Password: password})
	if err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("open %q from temp: %w", name, err)
	}
	return &tempArchive{Archive: arch, tmp: tmpPath}, nil
}

// readEntryBytes reads all bytes for a named entry from an open archive.
// entryPath may contain directory components ("subdir/inner.tar.gz").
func readEntryBytes(ctx context.Context, arch archive.Archive, entryPath string) ([]byte, error) {
	f, err := arch.Open(ctx, normalize(entryPath))
	if err != nil {
		return nil, fmt.Errorf("open entry %q: %w", entryPath, err)
	}
	defer f.Close()
	return io.ReadAll(f)
}

// tempArchive wraps an Archive and deletes the backing temp file on Close.
type tempArchive struct {
	archive.Archive
	tmp string
}

func (t *tempArchive) Close() error {
	err := t.Archive.Close()
	os.Remove(t.tmp)
	return err
}

// archiveMountPoint builds the deterministic VFS path for an archive mount.
//
//	"/home/user/data.zip"                       → "/archives/home/user/data.zip"
//	"/home/user/outer.zip" + ["inner.tar.gz"]   → "/archives/home/user/outer.zip/inner.tar.gz"
func archiveMountPoint(osPath string, nested []string) string {
	base := filepath.ToSlash(filepath.Clean("/archives/" + osPath))
	for _, n := range nested {
		base += "/" + filepath.ToSlash(n)
	}
	return base
}

// convertVFSInfo maps a vfs.FileInfo to the app's wire FileInfo type.
// Use convertVFSInfoWithChain when listing inside an archive so that nested
// archive entries carry the correct parentChain for the frontend to open them.
func convertVFSInfo(e vfslib.FileInfo) FileInfo {
	return convertVFSInfoWithChain(e, nil)
}

// convertVFSInfoWithChain maps a vfs.FileInfo to the app's wire FileInfo type,
// building the NestedChain for archive entries by appending the entry's own
// path to parentChain so the frontend can open them via VFSOpenArchive.
func convertVFSInfoWithChain(e vfslib.FileInfo, parentChain []string) FileInfo {
	ft := FileTypeFile
	switch {
	case e.IsDir():
		ft = FileTypeDirectory
	case e.IsArchive():
		ft = FileTypeArchive
	case e.IsSymlink():
		ft = FileTypeSymlink
	}

	var nestedChain []string
	if ft == FileTypeArchive {
		nestedChain = append(append([]string{}, parentChain...), e.Path())
	}

	return FileInfo{
		Name:        e.Name(),
		Path:        e.Path(),
		Type:        ft,
		Size:        e.Size(),
		Modified:    e.ModTime().UnixMilli(),
		IsHidden:    e.IsHidden(),
		IsSymlink:   e.IsSymlink(),
		IsNested:    ft == FileTypeArchive,
		NestedChain: nestedChain,
	}
}
