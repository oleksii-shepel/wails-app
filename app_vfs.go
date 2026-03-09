package main

import (
	"context"
	"fmt"
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
		files = append(files, convertVFSInfo(e))
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

// mountArchive opens the archive at osPath (resolving nested if non-empty),
// mounts it in the unified VFS, and returns the virtual mount point path.
func (a *App) mountArchive(ctx context.Context, osPath string, nested []string) (string, error) {
	// The innermost name in the chain determines the handler.
	handlerPath := osPath
	if len(nested) > 0 {
		handlerPath = nested[len(nested)-1]
	}

	handler, ok := archive.ForPath(handlerPath)
	if !ok {
		return "", fmt.Errorf("vfs: no archive handler for %q", handlerPath)
	}

	arch, err := handler.Open(ctx, osPath, archive.OpenOptions{
		ReadOnly:    true,
		NestedChain: nested,
	})
	if err != nil {
		return "", fmt.Errorf("vfs: open archive %q: %w", osPath, err)
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
func convertVFSInfo(e vfslib.FileInfo) FileInfo {
	ft := FileTypeFile
	switch {
	case e.IsDir():
		ft = FileTypeDirectory
	case e.IsArchive():
		ft = FileTypeArchive
	case e.IsSymlink():
		ft = FileTypeSymlink
	}

	return FileInfo{
		Name:        e.Name(),
		Path:        e.Path(),
		Type:        ft,
		Size:        e.Size(),
		Modified:    e.ModTime().Unix(),
		IsHidden:    e.IsHidden(),
		IsSymlink:   e.IsSymlink(),
		IsNested:    e.IsNested(),
		NestedChain: e.MountChain(),
	}
}
