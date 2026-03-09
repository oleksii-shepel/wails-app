package main

import (
	"context"
	"vfs"
	"vfs/archive"
	_ "vfs/archive/sevenz"
	_ "vfs/archive/tar"
	_ "vfs/archive/zip" // Register handlers
)

// VFSApp extends App with VFS capabilities
type VFSApp struct {
	vfs      *vfs.UnifiedVFS
	archives *archive.Registry
}

func (a *App) initVFS() {
	// Create local filesystem as root
	localFS := local.NewFileSystem("/")

	// Create unified VFS
	a.vfs = vfs.New(localFS)

	// Archive handlers are auto-registered via init()
}

// Wails-exposed methods

func (a *App) VFSListDir(path string) (*DirectoryListing, error) {
	ctx := context.Background()

	entries, err := a.vfs.ReadDir(ctx, path)
	if err != nil {
		return nil, err
	}

	// Convert to existing FileInfo format
	files := make([]FileInfo, len(entries))
	for i, e := range entries {
		files[i] = convertVFSInfo(e)
	}

	return &DirectoryListing{
		Path:    path,
		Entries: files,
	}, nil
}

func (a *App) VFSOpenArchive(path string, nested []string) (ArchiveListing, error) {
	ctx := context.Background()

	arch, err := a.vfs.OpenArchive(ctx, path, nested)
	if err != nil {
		return ArchiveListing{}, err
	}

	// Return mount point info
	return ArchiveListing{
		ArchivePath: path,
		InnerPath:   "/",
		NestedChain: nested,
		MountPoint:  arch.MountPoint(),
	}, nil
}

func (a *App) VFSMountArchive(path string) (string, error) {
	// Auto-mount archive and return virtual path
	arch, err := a.vfs.OpenArchive(context.Background(), path, nil)
	if err != nil {
		return "", err
	}
	return arch.MountPoint(), nil
}

func (a *App) VFSUnmount(mountPoint string) error {
	return a.vfs.Unmount(mountPoint)
}
