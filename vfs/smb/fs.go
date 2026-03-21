// Package smb provides a vfs.FileSystem backed by an SMB2/3 share connection.
package smb

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"path"
	"strings"
	"time"

	smb2 "github.com/hirochachacha/go-smb2"

	"wails-app/vfs"
)

// Config holds SMB connection parameters.
type Config struct {
	Host     string
	Port     int    // default 445
	Domain   string // NTLM domain; may be empty
	Username string
	Password string
	Share    string // share name, e.g. "Documents"
}

// FS implements vfs.FileSystem backed by one SMB share.
type FS struct {
	conn  net.Conn
	sess  *smb2.Session
	share *smb2.Share
}

var _ vfs.FileSystem = (*FS)(nil)

// New connects to the SMB server and mounts the named share.
func New(ctx context.Context, cfg Config) (*FS, error) {
	if cfg.Port == 0 {
		cfg.Port = 445
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("smb: dial %s: %w", addr, err)
	}

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     cfg.Username,
			Password: cfg.Password,
			Domain:   cfg.Domain,
		},
	}
	sess, err := d.DialContext(ctx, conn)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("smb: authenticate: %w", err)
	}

	shareName := `\\` + cfg.Host + `\` + strings.TrimLeft(cfg.Share, `\/`)
	share, err := sess.Mount(shareName)
	if err != nil {
		_ = sess.Logoff()
		_ = conn.Close()
		return nil, fmt.Errorf("smb: mount %s: %w", shareName, err)
	}
	return &FS{conn: conn, sess: sess, share: share}, nil
}

// Close unmounts the share, ends the session, and closes the TCP connection.
func (f *FS) Close() error {
	_ = f.share.Umount()
	_ = f.sess.Logoff()
	return f.conn.Close()
}

// clean converts a VFS-relative path to the format expected by go-smb2 (backslash-free).
func (f *FS) clean(p string) string {
	if p == "" || p == "." {
		return "."
	}
	return path.Clean(p)
}

// ── FileSystem implementation ────────────────────────────────────────────────

func (f *FS) Open(_ context.Context, p string) (vfs.File, error) {
	fh, err := f.share.Open(f.clean(p))
	if err != nil {
		return nil, mapErr(err)
	}
	return &file{File: fh}, nil
}

func (f *FS) Stat(_ context.Context, p string) (vfs.FileInfo, error) {
	si, err := f.share.Lstat(f.clean(p))
	if err != nil {
		return nil, mapErr(err)
	}
	return &fileInfo{FileInfo: si}, nil
}

// ReadDir lists a share directory. Each FileInfo.Path() is the entry name only.
func (f *FS) ReadDir(_ context.Context, p string) ([]vfs.FileInfo, error) {
	entries, err := f.share.ReadDir(f.clean(p))
	if err != nil {
		return nil, mapErr(err)
	}
	result := make([]vfs.FileInfo, 0, len(entries))
	for _, e := range entries {
		result = append(result, &fileInfo{FileInfo: e})
	}
	return result, nil
}

func (f *FS) Exists(ctx context.Context, p string) bool {
	_, err := f.Stat(ctx, p)
	return err == nil
}

func (f *FS) Create(_ context.Context, p string) (vfs.File, error) {
	fh, err := f.share.Create(f.clean(p))
	if err != nil {
		return nil, mapErr(err)
	}
	return &file{File: fh}, nil
}

func (f *FS) Mkdir(_ context.Context, p string, perm fs.FileMode) error {
	return mapErr(f.share.MkdirAll(f.clean(p), perm))
}

func (f *FS) Remove(_ context.Context, p string) error {
	return mapErr(f.share.RemoveAll(f.clean(p)))
}

func (f *FS) Rename(_ context.Context, old, nw string) error {
	return mapErr(f.share.Rename(f.clean(old), f.clean(nw)))
}

func (f *FS) Join(elem ...string) string        { return path.Join(elem...) }
func (f *FS) Split(p string) (string, string)   { return path.Split(p) }
func (f *FS) IsAbs(p string) bool               { return strings.HasPrefix(p, "/") }
func (f *FS) RealPath(_ string) (string, error) { return "", nil }

// ── fileInfo ──────────────────────────────────────────────────────────────────

type fileInfo struct {
	fs.FileInfo
}

func (fi *fileInfo) Path() string         { return fi.FileInfo.Name() }
func (fi *fileInfo) RealPath() string     { return "" }
func (fi *fileInfo) Volume() vfs.Volume   { return nil }
func (fi *fileInfo) IsArchive() bool      { return !fi.IsDir() && isArchiveExt(fi.Name()) }
func (fi *fileInfo) IsHidden() bool       { return strings.HasPrefix(fi.Name(), ".") }
func (fi *fileInfo) IsSymlink() bool      { return fi.Mode()&fs.ModeSymlink != 0 }
func (fi *fileInfo) IsNested() bool       { return false }
func (fi *fileInfo) MountChain() []string { return nil }

var _ vfs.FileInfo = (*fileInfo)(nil)

// ── file ──────────────────────────────────────────────────────────────────────

type file struct {
	*smb2.File
}

func (f *file) Stat() (vfs.FileInfo, error) {
	si, err := f.File.Stat()
	if err != nil {
		return nil, mapErr(err)
	}
	return &fileInfo{FileInfo: si}, nil
}

func (f *file) Sync() error { return f.File.Sync() }

var _ vfs.File = (*file)(nil)

// ── helpers ───────────────────────────────────────────────────────────────────

func isArchiveExt(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".rar", ".7z"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "object name not found") ||
		strings.Contains(s, "no such file") ||
		strings.Contains(s, "not found") {
		return vfs.ErrNotFound
	}
	if strings.Contains(s, "access is denied") || strings.Contains(s, "permission denied") {
		return vfs.ErrReadOnly
	}
	return err
}
