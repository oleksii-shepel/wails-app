// Package webdav provides a vfs.FileSystem backed by a WebDAV server.
// It uses only stdlib (net/http + encoding/xml) with no extra dependencies.
package webdav

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"wails-app/vfs"
)

// Config holds WebDAV connection parameters.
type Config struct {
	Host     string
	Port     int    // default 443 for https, 80 for http
	Schema   string // "http" or "https"; default "https"
	BasePath string // server-side path prefix, e.g. "/remote.php/dav/files/user"
	Username string
	Password string
	Insecure bool // skip TLS certificate verification
}

// FS implements vfs.FileSystem over a WebDAV connection.
type FS struct {
	client   *http.Client
	baseURL  string // e.g. "https://host:443/remote.php/dav/files/user"
	username string
	password string
}

var _ vfs.FileSystem = (*FS)(nil)

// New creates an FS and verifies the root can be reached via a PROPFIND.
func New(ctx context.Context, cfg Config) (*FS, error) {
	schema := cfg.Schema
	if schema == "" {
		schema = "https"
	}
	port := cfg.Port
	if port == 0 {
		if schema == "https" {
			port = 443
		} else {
			port = 80
		}
	}
	basePath := "/" + strings.Trim(cfg.BasePath, "/")

	baseURL := fmt.Sprintf("%s://%s:%d%s", schema, cfg.Host, port, basePath)

	transport := &http.Transport{}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	f := &FS{
		client:   client,
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: cfg.Username,
		password: cfg.Password,
	}

	// Verify connectivity with a depth-0 PROPFIND on the root.
	if _, err := f.propfind(ctx, "/", 0); err != nil {
		return nil, fmt.Errorf("webdav: connect %s: %w", baseURL, err)
	}
	return f, nil
}

// Close releases idle HTTP connections held by the transport.
func (f *FS) Close() error {
	if t, ok := f.client.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
	return nil
}

// serverURL converts a VFS-relative path to a full WebDAV URL.
func (f *FS) serverURL(p string) string {
	if p == "" || p == "." {
		return f.baseURL + "/"
	}
	return f.baseURL + "/" + strings.TrimLeft(url.PathEscape(path.Clean(p)), "/")
}

// ── FileSystem implementation ────────────────────────────────────────────────

func (f *FS) Open(ctx context.Context, p string) (vfs.File, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.serverURL(p), nil)
	if err != nil {
		return nil, err
	}
	f.addAuth(req)
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, vfs.ErrNotFound
	}
	if resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("webdav: GET %s: %s", p, resp.Status)
	}
	fi := &fileInfo{name: path.Base(p)}
	return &readFile{rc: resp.Body, info: fi}, nil
}

func (f *FS) Stat(ctx context.Context, p string) (vfs.FileInfo, error) {
	responses, err := f.propfind(ctx, p, 0)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, vfs.ErrNotFound
	}
	return responseToFileInfo(responses[0], path.Base(p)), nil
}

// ReadDir lists a WebDAV collection. Each FileInfo.Path() is the entry name only.
func (f *FS) ReadDir(ctx context.Context, p string) ([]vfs.FileInfo, error) {
	responses, err := f.propfind(ctx, p, 1)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, vfs.ErrNotFound
	}
	// responses[0] is the directory itself; skip it.
	result := make([]vfs.FileInfo, 0, len(responses)-1)
	for _, r := range responses[1:] {
		name := path.Base(strings.TrimRight(r.Href, "/"))
		if name == "" || name == "." {
			continue
		}
		result = append(result, responseToFileInfo(r, name))
	}
	return result, nil
}

func (f *FS) Exists(ctx context.Context, p string) bool {
	_, err := f.Stat(ctx, p)
	return err == nil
}

// Create returns a write-buffering File; the PUT upload occurs on Close.
func (f *FS) Create(_ context.Context, p string) (vfs.File, error) {
	return &writeFile{
		fs:   f,
		path: p,
		name: path.Base(p),
	}, nil
}

func (f *FS) Mkdir(ctx context.Context, p string, _ fs.FileMode) error {
	return f.mkcol(ctx, p)
}

func (f *FS) Remove(ctx context.Context, p string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", f.serverURL(p), nil)
	if err != nil {
		return err
	}
	f.addAuth(req)
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return vfs.ErrNotFound
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webdav: DELETE %s: %s", p, resp.Status)
	}
	return nil
}

func (f *FS) Rename(ctx context.Context, old, nw string) error {
	req, err := http.NewRequestWithContext(ctx, "MOVE", f.serverURL(old), nil)
	if err != nil {
		return err
	}
	f.addAuth(req)
	req.Header.Set("Destination", f.serverURL(nw))
	req.Header.Set("Overwrite", "T")
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return vfs.ErrNotFound
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webdav: MOVE %s → %s: %s", old, nw, resp.Status)
	}
	return nil
}

func (f *FS) Join(elem ...string) string        { return path.Join(elem...) }
func (f *FS) Split(p string) (string, string)   { return path.Split(p) }
func (f *FS) IsAbs(p string) bool               { return strings.HasPrefix(p, "/") }
func (f *FS) RealPath(_ string) (string, error) { return "", nil }

// ── internal HTTP helpers ────────────────────────────────────────────────────

func (f *FS) addAuth(req *http.Request) {
	if f.username != "" {
		req.SetBasicAuth(f.username, f.password)
	}
}

const propfindBody = `<?xml version="1.0" encoding="utf-8"?>` +
	`<D:propfind xmlns:D="DAV:"><D:prop>` +
	`<D:resourcetype/><D:getlastmodified/><D:getcontentlength/><D:displayname/>` +
	`</D:prop></D:propfind>`

func (f *FS) propfind(ctx context.Context, p string, depth int) ([]davResponse, error) {
	depthStr := "infinity"
	if depth == 0 {
		depthStr = "0"
	} else if depth == 1 {
		depthStr = "1"
	}

	req, err := http.NewRequestWithContext(ctx, "PROPFIND",
		f.serverURL(p), strings.NewReader(propfindBody))
	if err != nil {
		return nil, err
	}
	f.addAuth(req)
	req.Header.Set("Depth", depthStr)
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, vfs.ErrNotFound
	}
	if resp.StatusCode != 207 {
		return nil, fmt.Errorf("webdav: PROPFIND %s: %s", p, resp.Status)
	}

	var ms multiStatus
	if err := xml.NewDecoder(resp.Body).Decode(&ms); err != nil {
		return nil, fmt.Errorf("webdav: decode PROPFIND response: %w", err)
	}
	return ms.Responses, nil
}

func (f *FS) mkcol(ctx context.Context, p string) error {
	req, err := http.NewRequestWithContext(ctx, "MKCOL", f.serverURL(p), nil)
	if err != nil {
		return err
	}
	f.addAuth(req)
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusMethodNotAllowed {
		return vfs.ErrExists
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webdav: MKCOL %s: %s", p, resp.Status)
	}
	return nil
}

// ── WebDAV XML types ─────────────────────────────────────────────────────────

type multiStatus struct {
	XMLName   xml.Name      `xml:"multistatus"`
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href     string        `xml:"href"`
	PropStat []davPropStat `xml:"propstat"`
}

type davPropStat struct {
	Prop   davProp `xml:"prop"`
	Status string  `xml:"status"`
}

type davProp struct {
	ResourceType  resourceType `xml:"resourcetype"`
	LastModified  string       `xml:"getlastmodified"`
	ContentLength int64        `xml:"getcontentlength"`
	DisplayName   string       `xml:"displayname"`
}

type resourceType struct {
	InnerXML string `xml:",innerxml"`
}

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
func (fi *fileInfo) Path() string         { return fi.name }
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
	rc   io.ReadCloser
	info vfs.FileInfo
}

func (f *readFile) Read(p []byte) (int, error) { return f.rc.Read(p) }
func (f *readFile) Write(_ []byte) (int, error) {
	return 0, fmt.Errorf("webdav: file opened read-only")
}
func (f *readFile) Seek(_ int64, _ int) (int64, error) {
	return 0, fmt.Errorf("webdav: seek not supported")
}
func (f *readFile) Close() error                { return f.rc.Close() }
func (f *readFile) Sync() error                 { return nil }
func (f *readFile) Stat() (vfs.FileInfo, error) { return f.info, nil }

var _ vfs.File = (*readFile)(nil)

// ── writeFile ─────────────────────────────────────────────────────────────────

type writeFile struct {
	buf    bytes.Buffer
	fs     *FS
	path   string
	name   string
	closed bool
}

func (f *writeFile) Read(_ []byte) (int, error)  { return 0, io.EOF }
func (f *writeFile) Write(p []byte) (int, error) { return f.buf.Write(p) }
func (f *writeFile) Seek(_ int64, _ int) (int64, error) {
	return 0, fmt.Errorf("webdav: seek not supported for write")
}
func (f *writeFile) Sync() error                 { return nil }
func (f *writeFile) Stat() (vfs.FileInfo, error) { return &fileInfo{name: f.name}, nil }
func (f *writeFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut,
		f.fs.serverURL(f.path), &f.buf)
	if err != nil {
		return err
	}
	f.fs.addAuth(req)
	resp, err := f.fs.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webdav: PUT %s: %s", f.path, resp.Status)
	}
	return nil
}

var _ vfs.File = (*writeFile)(nil)

// ── helpers ───────────────────────────────────────────────────────────────────

func responseToFileInfo(r davResponse, name string) *fileInfo {
	fi := &fileInfo{name: name}
	for _, ps := range r.PropStat {
		if !strings.Contains(ps.Status, "200") {
			continue
		}
		fi.isDir = strings.Contains(ps.Prop.ResourceType.InnerXML, "collection")
		fi.size = ps.Prop.ContentLength
		if ps.Prop.LastModified != "" {
			for _, layout := range []string{
				time.RFC1123, time.RFC1123Z,
				"Mon, 2 Jan 2006 15:04:05 MST",
				"Mon, 2 Jan 2006 15:04:05 -0700",
			} {
				if t, err := time.Parse(layout, ps.Prop.LastModified); err == nil {
					fi.modTime = t
					break
				}
			}
		}
		if ps.Prop.DisplayName != "" && fi.name == "" {
			fi.name = ps.Prop.DisplayName
		}
	}
	return fi
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
