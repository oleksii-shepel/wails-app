// Package sftp provides a vfs.FileSystem backed by an SSH/SFTP connection.
package sftp

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	gosftp "github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"

	"wails-app/vfs"
)

// Config holds SFTP connection parameters.
type Config struct {
	Host          string
	Port          int // default 22
	Username      string
	Password      string
	PrivateKey    []byte // PEM-encoded private key; takes precedence over Password
	IgnoreHostKey bool   // skip host-key verification (insecure, useful for dev)
}

// FS implements vfs.FileSystem over an SFTP connection.
type FS struct {
	ssh    *gossh.Client
	client *gosftp.Client
}

var _ vfs.FileSystem = (*FS)(nil)

// New opens an SSH connection and then negotiates the SFTP subsystem.
func New(_ context.Context, cfg Config) (*FS, error) {
	if cfg.Port == 0 {
		cfg.Port = 22
	}

	var auths []gossh.AuthMethod
	if len(cfg.PrivateKey) > 0 {
		signer, err := gossh.ParsePrivateKey(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("sftp: parse private key: %w", err)
		}
		auths = append(auths, gossh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		auths = append(auths, gossh.Password(cfg.Password))
	}

	hostKeyCallback := gossh.InsecureIgnoreHostKey()
	if !cfg.IgnoreHostKey {
		// TODO: integrate with known_hosts; for now still permissive
		hostKeyCallback = gossh.InsecureIgnoreHostKey()
	}

	sshCfg := &gossh.ClientConfig{
		User:            cfg.Username,
		Auth:            auths,
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	sshConn, err := gossh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("sftp: ssh dial %s: %w", addr, err)
	}

	client, err := gosftp.NewClient(sshConn)
	if err != nil {
		_ = sshConn.Close()
		return nil, fmt.Errorf("sftp: open sftp subsystem: %w", err)
	}
	return &FS{ssh: sshConn, client: client}, nil
}

// Close terminates the SFTP subsystem and the underlying SSH connection.
func (f *FS) Close() error {
	err := f.client.Close()
	_ = f.ssh.Close()
	return err
}

// absPath converts a VFS-relative path to an absolute server path.
func (f *FS) absPath(p string) string {
	if p == "" || p == "." {
		return "/"
	}
	return path.Clean("/" + strings.TrimLeft(p, "/"))
}

// ── FileSystem implementation ────────────────────────────────────────────────

func (f *FS) Open(_ context.Context, p string) (vfs.File, error) {
	fh, err := f.client.Open(f.absPath(p))
	if err != nil {
		return nil, mapErr(err)
	}
	return &file{File: fh}, nil
}

func (f *FS) Stat(_ context.Context, p string) (vfs.FileInfo, error) {
	si, err := f.client.Lstat(f.absPath(p))
	if err != nil {
		return nil, mapErr(err)
	}
	return &fileInfo{FileInfo: si}, nil
}

// ReadDir lists a remote directory. Each FileInfo.Path() returns the entry
// name only, so pathref.FromVFSEntry works correctly at any depth.
func (f *FS) ReadDir(_ context.Context, p string) ([]vfs.FileInfo, error) {
	entries, err := f.client.ReadDir(f.absPath(p))
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
	fh, err := f.client.Create(f.absPath(p))
	if err != nil {
		return nil, mapErr(err)
	}
	return &file{File: fh}, nil
}

func (f *FS) Mkdir(_ context.Context, p string, _ fs.FileMode) error {
	return mapErr(f.client.MkdirAll(f.absPath(p)))
}

func (f *FS) Remove(_ context.Context, p string) error {
	return mapErr(f.client.RemoveAll(f.absPath(p)))
}

func (f *FS) Rename(_ context.Context, old, nw string) error {
	return mapErr(f.client.Rename(f.absPath(old), f.absPath(nw)))
}

func (f *FS) Join(elem ...string) string        { return path.Join(elem...) }
func (f *FS) Split(p string) (string, string)   { return path.Split(p) }
func (f *FS) IsAbs(p string) bool               { return strings.HasPrefix(p, "/") }
func (f *FS) RealPath(_ string) (string, error) { return "", nil }

// ── fileInfo ──────────────────────────────────────────────────────────────────

// fileInfo wraps an os.FileInfo from the SFTP client and satisfies vfs.FileInfo.
type fileInfo struct {
	fs.FileInfo // os.FileInfo is fs.FileInfo
}

// Path returns the entry name only (not a full path), consistent with
// how the unified VFS and FromVFSEntry expect remote FS entries.
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
	*gosftp.File
}

func (f *file) Stat() (vfs.FileInfo, error) {
	si, err := f.File.Stat()
	if err != nil {
		return nil, mapErr(err)
	}
	return &fileInfo{FileInfo: si}, nil
}

func (f *file) Sync() error { return nil }

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
	if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "no such file") {
		return vfs.ErrNotFound
	}
	return err
}
