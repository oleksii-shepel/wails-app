package main

import (
	"context"
	"fmt"
	"path/filepath"

	internalarchive "wails-app/internal/archive"
	vfslib "wails-app/vfs"
	"wails-app/vfs/mount"
	"wails-app/vfs/osfs"
	"wails-app/vfs/registry"

	_ "wails-app/vfs/archive/7z"  // register 7z handler
	_ "wails-app/vfs/archive/rar" // register rar handler
	_ "wails-app/vfs/archive/tar" // register tar/tar.gz/tar.bz2 handler
	_ "wails-app/vfs/archive/zip" // register zip handler
)

// initVFS sets up the unified VFS. Call at the end of App.startup().
func (a *App) initVFS() {
	mgr := mount.NewManager(osfs.New("/"))
	a.vfs = vfslib.NewUnified(mgr)

	if regFS, err := registry.New(); err == nil {
		a.vfs.Mount("/registry", regFS, true, "Registry", vfslib.VolumeRegistry)
	}

	a.filesystem.SetVFS(a.vfs)
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
	arch, err := internalarchive.ResolveChain(ctx, osPath, nested, "")
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
