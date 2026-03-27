package runtime

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"path/filepath"

	internalarchive "wails-app/internal/archive"
	"wails-app/internal/contentprobe"
	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	vfslib "wails-app/vfs"
	"wails-app/vfs/memory"
	"wails-app/vfs/mount"
	"wails-app/vfs/osfs"
	"wails-app/vfs/registry"

	_ "wails-app/vfs/archive/7z"
	_ "wails-app/vfs/archive/rar"
	_ "wails-app/vfs/archive/tar"
	_ "wails-app/vfs/archive/zip"
)

func InitVFS(memoryFS *memory.FS) (*vfslib.Unified, error) {
	manager := mount.NewManager(osfs.New("/"))
	vfs := vfslib.NewUnified(manager)

	if memoryFS != nil {
		if _, err := vfs.Mount("/memory", memoryFS, false, "Memory", vfslib.VolumeMemory); err != nil {
			return nil, err
		}
	}

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

func VFSReadFileRange(vfs *vfslib.Unified, path string, offset int64, length int) (*contracts.FileContent, error) {
	publicPath := pathref.Normalize(path)
	internalPath := pathref.ToVFS(path)
	file, err := vfs.Open(context.Background(), internalPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	if offset < 0 {
		offset = 0
	}
	if offset > fileSize {
		offset = fileSize
	}
	available := fileSize - offset
	if int64(length) > available {
		length = int(available)
	}
	if length <= 0 {
		return &contracts.FileContent{
			Path:     publicPath,
			Content:  "",
			Encoding: "base64",
			Size:     fileSize,
		}, nil
	}

	if _, seekErr := file.Seek(offset, io.SeekStart); seekErr != nil {
		return nil, seekErr
	}

	buf := make([]byte, length)
	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}

	return &contracts.FileContent{
		Path:     publicPath,
		Content:  base64.StdEncoding.EncodeToString(buf[:n]),
		Encoding: "base64",
		Size:     fileSize,
	}, nil
}

func VFSReadFile(vfs *vfslib.Unified, path string, encoding string) (*contracts.FileContent, error) {
	publicPath := pathref.Normalize(path)
	internalPath := pathref.ToVFS(path)
	file, err := vfs.Open(context.Background(), internalPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	probe := contentprobe.ProbeBytes(data)
	if encoding == "" {
		encoding = "utf8"
		if probe.IsText && probe.Encoding != "" {
			encoding = probe.Encoding
		}
	}

	content, err := contentprobe.Decode(data, encoding)
	if err != nil {
		return nil, err
	}

	return &contracts.FileContent{
		Path:     publicPath,
		Content:  content,
		Encoding: encoding,
		Size:     int64(len(data)),
	}, nil
}

func VFSProbeFile(vfs *vfslib.Unified, path string) (*contracts.FileProbe, error) {
	internalPath := pathref.ToVFS(path)
	file, err := vfs.Open(context.Background(), internalPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, probe, err := contentprobe.ProbeReader(file)
	if err != nil {
		return nil, err
	}
	return &contracts.FileProbe{
		MimeType:  probe.MimeType,
		Encoding:  probe.Encoding,
		IsText:    probe.IsText,
		IsArchive: probe.IsArchive,
	}, nil
}

func VFSWriteFile(vfs *vfslib.Unified, path string, content string, encoding string) (*contracts.OperationResult, error) {
	publicPath := pathref.Normalize(path)
	internalPath := pathref.ToVFS(path)

	var data []byte
	var err error
	if encoding == "base64" {
		data, err = base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, err
		}
	} else {
		data = []byte(content)
	}

	file, err := vfs.Create(context.Background(), internalPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}

	return &contracts.OperationResult{
		Success:       true,
		Message:       "File written successfully",
		AffectedPaths: []string{publicPath},
	}, nil
}

func VFSMkdir(vfs *vfslib.Unified, path string) (*contracts.OperationResult, error) {
	publicPath := pathref.Normalize(path)
	internalPath := pathref.ToVFS(path)
	if err := vfs.Mkdir(context.Background(), internalPath, 0755); err != nil {
		return nil, err
	}

	return &contracts.OperationResult{
		Success:       true,
		Message:       "Directory created successfully",
		AffectedPaths: []string{publicPath},
	}, nil
}

func VFSRemove(vfs *vfslib.Unified, path string) (*contracts.OperationResult, error) {
	publicPath := pathref.Normalize(path)
	internalPath := pathref.ToVFS(path)
	if err := vfs.Remove(context.Background(), internalPath); err != nil {
		return nil, err
	}

	return &contracts.OperationResult{
		Success:       true,
		Message:       "Deleted successfully",
		AffectedPaths: []string{publicPath},
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

	file := contracts.FileInfo{
		Name:        entry.Name(),
		Path:        entry.Path(),
		Type:        fileType,
		MimeType:    mime.TypeByExtension(filepath.Ext(entry.Name())),
		Size:        entry.Size(),
		Modified:    entry.ModTime().Unix(),
		IsHidden:    entry.IsHidden(),
		IsSymlink:   entry.IsSymlink(),
		IsNested:    fileType == contracts.FileTypeArchive,
		NestedChain: nestedChain,
	}

	registry.ApplyFileInfoMetadata(&file, entry)

	return file
}
