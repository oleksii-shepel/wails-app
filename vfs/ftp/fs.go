// Package ftp provides a vfs.FileSystem backed by operation-scoped FTP
// sessions. Each directory read / stat / file stream gets its own connection so
// long-lived previews do not block navigation on the same mount.
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
	cfg Config
}

var _ vfs.FileSystem = (*FS)(nil)

// New dials and authenticates to the FTP server described by cfg.
func New(ctx context.Context, cfg Config) (*FS, error) {
	if cfg.Port == 0 {
		cfg.Port = 21
	}
	fs := &FS{cfg: cfg}
	// Validate mount credentials eagerly so navigation fails up front.
	conn, err := fs.connect(ctx)
	if err != nil {
		return nil, err
	}
	_ = conn.Quit()
	return fs, nil
}

// Close is a no-op because operation-scoped sessions are closed per request.
func (f *FS) Close() error {
	return nil
}

// clean converts a VFS-relative path to an absolute FTP server path.
func (f *FS) clean(p string) string {
	if p == "" || p == "." {
		return "/"
	}
	return path.Clean("/" + strings.TrimLeft(p, "/"))
}

// ── FileSystem implementation ────────────────────────────────────────────────

func (f *FS) Open(ctx context.Context, p string) (vfs.File, error) {
	conn, err := f.connect(ctx)
	if err != nil {
		return nil, err
	}
	r, err := conn.Retr(f.clean(p))
	if err != nil {
		_ = conn.Quit()
		return nil, mapErr(err)
	}
	return &readFile{
		rc:   r,
		conn: conn,
		name: path.Base(p),
	}, nil
}

func (f *FS) Stat(ctx context.Context, p string) (vfs.FileInfo, error) {
	var info *fileInfo
	err := f.withConn(ctx, func(conn *ftplib.ServerConn) error {
		var err error
		info, err = f.statWithConn(conn, p)
		return err
	})
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (f *FS) statWithConn(conn *ftplib.ServerConn, p string) (*fileInfo, error) {
	clean := f.clean(p)
	if clean == "/" {
		return &fileInfo{name: "/", isDir: true}, nil
	}
	entries, err := conn.List(path.Dir(clean))
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
func (f *FS) ReadDir(ctx context.Context, p string) ([]vfs.FileInfo, error) {
	result := make([]vfs.FileInfo, 0)
	err := f.withConn(ctx, func(conn *ftplib.ServerConn) error {
		entries, err := conn.List(f.clean(p))
		if err != nil {
			return mapErr(err)
		}
		result = make([]vfs.FileInfo, 0, len(entries))
		for _, e := range entries {
			if e.Name == "." || e.Name == ".." {
				continue
			}
			result = append(result, entryToInfo(e))
		}
		return nil
	})
	if err != nil {
		return nil, err
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
		cfg:        f.cfg,
		remotePath: f.clean(p),
		name:       path.Base(p),
	}, nil
}

func (f *FS) Mkdir(ctx context.Context, p string, _ fs.FileMode) error {
	return f.withConn(ctx, func(conn *ftplib.ServerConn) error {
		return mapErr(conn.MakeDir(f.clean(p)))
	})
}

func (f *FS) Remove(ctx context.Context, p string) error {
	return f.withConn(ctx, func(conn *ftplib.ServerConn) error {
		clean := f.clean(p)
		if err := conn.RemoveDir(clean); err != nil {
			if err2 := conn.Delete(clean); err2 != nil {
				return mapErr(err2)
			}
		}
		return nil
	})
}

func (f *FS) Rename(ctx context.Context, old, nw string) error {
	return f.withConn(ctx, func(conn *ftplib.ServerConn) error {
		return mapErr(conn.Rename(f.clean(old), f.clean(nw)))
	})
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
	rc     *ftplib.Response
	conn   *ftplib.ServerConn
	name   string
	closed bool
}

func (f *readFile) Read(p []byte) (int, error)  { return f.rc.Read(p) }
func (f *readFile) Write(_ []byte) (int, error) { return 0, fmt.Errorf("ftp: file opened read-only") }
func (f *readFile) Seek(_ int64, _ int) (int64, error) {
	return 0, fmt.Errorf("ftp: seek not supported")
}
func (f *readFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	var closeErr error
	if f.rc != nil {
		closeErr = f.rc.Close()
	}
	if f.conn != nil {
		if quitErr := f.conn.Quit(); closeErr == nil {
			closeErr = quitErr
		}
	}
	return closeErr
}
func (f *readFile) Sync() error                 { return nil }
func (f *readFile) Stat() (vfs.FileInfo, error) { return &fileInfo{name: f.name}, nil }

var _ vfs.File = (*readFile)(nil)

// ── writeFile ─────────────────────────────────────────────────────────────────

// writeFile buffers writes in memory and uploads via STOR on Close.
type writeFile struct {
	buf        bytes.Buffer
	cfg        Config
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
	conn, err := connect(context.Background(), f.cfg)
	if err != nil {
		return err
	}
	defer conn.Quit()
	return mapErr(conn.Stor(f.remotePath, &f.buf))
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

func (f *FS) withConn(ctx context.Context, fn func(conn *ftplib.ServerConn) error) error {
	conn, err := f.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Quit()
	return fn(conn)
}

func (f *FS) connect(ctx context.Context) (*ftplib.ServerConn, error) {
	return connect(ctx, f.cfg)
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
	var firstDial bool
	return func(network, address string) (net.Conn, error) {
		dialCtx := context.Background()
		if !firstDial {
			firstDial = true
			if ctx != nil {
				dialCtx = ctx
			}
		}

		conn, err := dialer.DialContext(dialCtx, network, address)
		if err != nil {
			return nil, err
		}
		return &deadlineConn{Conn: conn, timeout: timeout}, nil
	}
}

func connect(ctx context.Context, cfg Config) (*ftplib.ServerConn, error) {
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
	return conn, nil
}
