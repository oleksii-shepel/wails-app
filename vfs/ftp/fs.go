// Package ftp provides a vfs.FileSystem backed by a persistent FTP connection.
// All operations serialize through a mutex because the FTP protocol is
// command/response and does not support concurrent in-flight commands on a
// single control connection.
package ftp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"path"
	"strings"
	"sync"
	"time"

	ftplib "github.com/jlaffaye/ftp"

	"wails-app/vfs"
)

const (
	ftpDialTimeout      = 30 * time.Second
	ftpOperationTimeout = 15 * time.Second
)

// Config holds FTP connection parameters.
type Config struct {
	Host     string
	Port     int // default 21
	Username string
	Password string
}

// FS implements vfs.FileSystem over a persistent FTP connection.
type FS struct {
	cfg  Config
	conn *ftplib.ServerConn
	mu   sync.Mutex
}

var _ vfs.FileSystem = (*FS)(nil)

// New dials and authenticates to the FTP server described by cfg.
func New(ctx context.Context, cfg Config) (*FS, error) {
	if cfg.Port == 0 {
		cfg.Port = 21
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	dialer := net.Dialer{Timeout: ftpDialTimeout}
	conn, err := ftplib.Dial(addr,
		ftplib.DialWithDialFunc(newDeadlineDialFunc(ctx, dialer, ftpOperationTimeout)),
		ftplib.DialWithShutTimeout(ftpOperationTimeout),
		ftplib.DialWithDisabledEPSV(true),
	)
	if err != nil {
		return nil, fmt.Errorf("ftp: dial %s: %w", addr, err)
	}
	if err := conn.Login(cfg.Username, cfg.Password); err != nil {
		_ = conn.Quit()
		return nil, fmt.Errorf("ftp: login: %w", err)
	}
	return &FS{cfg: cfg, conn: conn}, nil
}

// Close terminates the FTP session. The mount.Manager calls this automatically
// when the volume is unmounted.
func (f *FS) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.conn.Quit()
}

// clean converts a VFS-relative path to an absolute FTP server path.
func (f *FS) clean(p string) string {
	if p == "" || p == "." {
		return "/"
	}
	return path.Clean("/" + strings.TrimLeft(p, "/"))
}

// ── FileSystem implementation ────────────────────────────────────────────────

func (f *FS) Open(_ context.Context, p string) (vfs.File, error) {
	f.mu.Lock()
	r, err := f.conn.Retr(f.clean(p))
	if err != nil {
		f.mu.Unlock()
		return nil, mapErr(err)
	}
	return &readFile{
		rc:     r,
		name:   path.Base(p),
		unlock: f.mu.Unlock,
	}, nil
}

func (f *FS) Stat(_ context.Context, p string) (vfs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.statLocked(p)
}

func (f *FS) statLocked(p string) (*fileInfo, error) {
	clean := f.clean(p)
	if clean == "/" {
		return &fileInfo{name: "/", isDir: true}, nil
	}
	entries, err := f.conn.List(path.Dir(clean))
	if err != nil {
		return nil, mapErr(err)
	}
	name := path.Base(clean)
	for _, e := range entries {
		if e.Name == name {
			return entryToInfo(e), nil
		}
	}
	return nil, vfs.ErrNotFound
}

// ReadDir lists a remote directory. Each FileInfo.Path() is the entry *name*
// only (no directory prefix), which lets pathref.FromVFSEntry compose public
// paths correctly at any depth.
func (f *FS) ReadDir(_ context.Context, p string) ([]vfs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entries, err := f.conn.List(f.clean(p))
	if err != nil {
		return nil, mapErr(err)
	}
	result := make([]vfs.FileInfo, 0, len(entries))
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		result = append(result, entryToInfo(e))
	}
	return result, nil
}

func (f *FS) Exists(ctx context.Context, p string) bool {
	_, err := f.Stat(ctx, p)
	return err == nil
}

// Create returns a write-buffering File. The actual STOR upload occurs on Close.
func (f *FS) Create(_ context.Context, p string) (vfs.File, error) {
	return &writeFile{
		conn:       f.conn,
		mu:         &f.mu,
		remotePath: f.clean(p),
		name:       path.Base(p),
	}, nil
}

func (f *FS) Mkdir(_ context.Context, p string, _ fs.FileMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return mapErr(f.conn.MakeDir(f.clean(p)))
}

func (f *FS) Remove(_ context.Context, p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	clean := f.clean(p)
	if err := f.conn.RemoveDir(clean); err != nil {
		if err2 := f.conn.Delete(clean); err2 != nil {
			return mapErr(err2)
		}
	}
	return nil
}

func (f *FS) Rename(_ context.Context, old, nw string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return mapErr(f.conn.Rename(f.clean(old), f.clean(nw)))
}

func (f *FS) Join(elem ...string) string        { return path.Join(elem...) }
func (f *FS) Split(p string) (string, string)   { return path.Split(p) }
func (f *FS) IsAbs(p string) bool               { return strings.HasPrefix(p, "/") }
func (f *FS) RealPath(_ string) (string, error) { return "", nil }

// ── fileInfo ──────────────────────────────────────────────────────────────────

type fileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (fi *fileInfo) Name() string { return fi.name }
func (fi *fileInfo) Size() int64  { return fi.size }
func (fi *fileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0755
	}
	return 0644
}
func (fi *fileInfo) ModTime() time.Time   { return fi.modTime }
func (fi *fileInfo) IsDir() bool          { return fi.isDir }
func (fi *fileInfo) Sys() any             { return nil }
func (fi *fileInfo) Path() string         { return fi.name } // entry name only
func (fi *fileInfo) RealPath() string     { return "" }
func (fi *fileInfo) Volume() vfs.Volume   { return nil }
func (fi *fileInfo) IsArchive() bool      { return !fi.isDir && isArchiveExt(fi.name) }
func (fi *fileInfo) IsHidden() bool       { return strings.HasPrefix(fi.name, ".") }
func (fi *fileInfo) IsSymlink() bool      { return false }
func (fi *fileInfo) IsNested() bool       { return false }
func (fi *fileInfo) MountChain() []string { return nil }

var _ vfs.FileInfo = (*fileInfo)(nil)

// ── readFile ──────────────────────────────────────────────────────────────────

type readFile struct {
	rc        *ftplib.Response
	name      string
	unlock    func()
	closeOnce sync.Once
	closeErr  error
}

func (f *readFile) Read(p []byte) (int, error)  { return f.rc.Read(p) }
func (f *readFile) Write(_ []byte) (int, error) { return 0, fmt.Errorf("ftp: file opened read-only") }
func (f *readFile) Seek(_ int64, _ int) (int64, error) {
	return 0, fmt.Errorf("ftp: seek not supported")
}
func (f *readFile) Close() error {
	f.closeOnce.Do(func() {
		f.closeErr = f.rc.Close()
		if f.unlock != nil {
			f.unlock()
		}
	})
	return f.closeErr
}
func (f *readFile) Sync() error                 { return nil }
func (f *readFile) Stat() (vfs.FileInfo, error) { return &fileInfo{name: f.name}, nil }

var _ vfs.File = (*readFile)(nil)

// ── writeFile ─────────────────────────────────────────────────────────────────

// writeFile buffers writes in memory and uploads via STOR on Close.
type writeFile struct {
	buf        bytes.Buffer
	conn       *ftplib.ServerConn
	mu         *sync.Mutex
	remotePath string
	name       string
	closed     bool
}

func (f *writeFile) Read(_ []byte) (int, error)  { return 0, io.EOF }
func (f *writeFile) Write(p []byte) (int, error) { return f.buf.Write(p) }
func (f *writeFile) Seek(_ int64, _ int) (int64, error) {
	return 0, fmt.Errorf("ftp: seek not supported for write")
}
func (f *writeFile) Sync() error                 { return nil }
func (f *writeFile) Stat() (vfs.FileInfo, error) { return &fileInfo{name: f.name}, nil }
func (f *writeFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	f.mu.Lock()
	defer f.mu.Unlock()
	return mapErr(f.conn.Stor(f.remotePath, &f.buf))
}

var _ vfs.File = (*writeFile)(nil)

// ── helpers ───────────────────────────────────────────────────────────────────

func entryToInfo(e *ftplib.Entry) *fileInfo {
	return &fileInfo{
		name:    e.Name,
		size:    int64(e.Size),
		modTime: e.Time,
		isDir:   e.Type == ftplib.EntryTypeFolder,
	}
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

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "550") || strings.Contains(s, "no such file") || strings.Contains(s, "not found") {
		return vfs.ErrNotFound
	}
	return err
}

type deadlineConn struct {
	net.Conn
	timeout time.Duration
}

func (c *deadlineConn) Read(p []byte) (int, error) {
	if err := c.bumpReadDeadline(); err != nil {
		return 0, err
	}
	return c.Conn.Read(p)
}

func (c *deadlineConn) Write(p []byte) (int, error) {
	if err := c.bumpWriteDeadline(); err != nil {
		return 0, err
	}
	return c.Conn.Write(p)
}

func (c *deadlineConn) bumpReadDeadline() error {
	if c.timeout <= 0 {
		return nil
	}
	return c.Conn.SetReadDeadline(time.Now().Add(c.timeout))
}

func (c *deadlineConn) bumpWriteDeadline() error {
	if c.timeout <= 0 {
		return nil
	}
	return c.Conn.SetWriteDeadline(time.Now().Add(c.timeout))
}

func newDeadlineDialFunc(
	ctx context.Context,
	dialer net.Dialer,
	timeout time.Duration,
) func(network, address string) (net.Conn, error) {
	var firstDial sync.Once
	return func(network, address string) (net.Conn, error) {
		dialCtx := context.Background()
		firstDial.Do(func() {
			if ctx != nil {
				dialCtx = ctx
			}
		})

		conn, err := dialer.DialContext(dialCtx, network, address)
		if err != nil {
			return nil, err
		}
		return &deadlineConn{Conn: conn, timeout: timeout}, nil
	}
}
