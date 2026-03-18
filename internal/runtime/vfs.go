package runtime

import (
	"context"
	"fmt"
	"path/filepath"

	internalarchive "wails-app/internal/archive"
	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	vfslib "wails-app/vfs"
	"wails-app/vfs/mount"
	"wails-app/vfs/osfs"
	"wails-app/vfs/registry"

	_ "wails-app/vfs/archive/7z"
	_ "wails-app/vfs/archive/rar"
	_ "wails-app/vfs/archive/tar"
	_ "wails-app/vfs/archive/zip"
)

func InitVFS() (*vfslib.Unified, error) {
	manager := mount.NewManager(osfs.New("/"))
	vfs := vfslib.NewUnified(manager)

	if regFS, err := registry.New(); err == nil {
		if _, err := vfs.Mount("/registry", regFS, true, "Registry", vfslib.VolumeRegistry); err != nil {
			return nil, err
		}
	}

	return vfs, nil
}

func VFSListDir(vfs *vfslib.Unified, path string) (*contracts.DirectoryListing, error) {
	publicPath := pathref.Normalize(path)
	internalPath := pathref.ToVFS(path)
	entries, err := vfs.ReadDir(context.Background(), internalPath)
	if err != nil {
		return nil, err
	}

	files := make([]contracts.FileInfo, 0, len(entries))
	var total int64
	for _, entry := range entries {
		file := convertVFSInfo(entry)
		file.Path = pathref.FromVFSEntry(publicPath, file.Path)
		if len(file.NestedChain) > 0 {
			for index, nested := range file.NestedChain {
				file.NestedChain[index] = pathref.FromVFSEntry(publicPath, nested)
			}
		}
		files = append(files, file)
		total += file.Size
	}

	return &contracts.DirectoryListing{
		Path:      pathref.FromVFS(internalPath),
		Entries:   files,
		TotalSize: total,
	}, nil
}

func VFSOpenArchive(vfs *vfslib.Unified, path string, nested []string) (contracts.ArchiveListing, error) {
	ctx := context.Background()
	mountPoint, err := mountArchive(ctx, vfs, path, nested)
	if err != nil {
		return contracts.ArchiveListing{}, err
	}

	entries, err := vfs.ReadDir(ctx, mountPoint)
	if err != nil {
		return contracts.ArchiveListing{}, err
	}

	files := make([]contracts.FileInfo, 0, len(entries))
	for _, entry := range entries {
		files = append(files, convertVFSInfoWithChain(entry, nested))
	}

	return contracts.ArchiveListing{
		ArchivePath: path,
		InnerPath:   mountPoint,
		NestedChain: nested,
		Entries:     files,
	}, nil
}

func VFSMountArchive(vfs *vfslib.Unified, path string) (string, error) {
	return mountArchive(context.Background(), vfs, path, nil)
}

func VFSUnmount(vfs *vfslib.Unified, mountPoint string) error {
	return vfs.Unmount(mountPoint)
}

func mountArchive(ctx context.Context, vfs *vfslib.Unified, osPath string, nested []string) (string, error) {
	arch, err := internalarchive.ResolveChain(ctx, osPath, nested, "")
	if err != nil {
		return "", fmt.Errorf("vfs: open archive %q (chain %v): %w", osPath, nested, err)
	}

	mountPoint := archiveMountPoint(osPath, nested)
	if _, err := vfs.Mount(mountPoint, arch, true, filepath.Base(osPath), vfslib.VolumeArchive); err != nil {
		_ = arch.Close()
		return "", fmt.Errorf("vfs: mount %q -> %q: %w", osPath, mountPoint, err)
	}
	return mountPoint, nil
}

func archiveMountPoint(osPath string, nested []string) string {
	base := filepath.ToSlash(filepath.Clean("/archives/" + osPath))
	for _, nestedPath := range nested {
		base += "/" + filepath.ToSlash(nestedPath)
	}
	return base
}

func convertVFSInfo(entry vfslib.FileInfo) contracts.FileInfo {
	return convertVFSInfoWithChain(entry, nil)
}

func convertVFSInfoWithChain(entry vfslib.FileInfo, parentChain []string) contracts.FileInfo {
	fileType := contracts.FileTypeFile
	switch {
	case entry.IsDir():
		fileType = contracts.FileTypeDirectory
	case entry.IsArchive():
		fileType = contracts.FileTypeArchive
	case entry.IsSymlink():
		fileType = contracts.FileTypeSymlink
	}

	var nestedChain []string
	if fileType == contracts.FileTypeArchive {
		nestedChain = append(append([]string{}, parentChain...), entry.Path())
	}

	return contracts.FileInfo{
		Name:        entry.Name(),
		Path:        entry.Path(),
		Type:        fileType,
		Size:        entry.Size(),
		Modified:    entry.ModTime().Unix(),
		IsHidden:    entry.IsHidden(),
		IsSymlink:   entry.IsSymlink(),
		IsNested:    fileType == contracts.FileTypeArchive,
		NestedChain: nestedChain,
	}
}
