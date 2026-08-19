// Package onedrive provides a read-only vfs.FileSystem backed by the Microsoft Graph OneDrive API.
package onedrive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"wails-app/vfs"
	"wails-app/vfs/cloudutil"
)

const graphBaseURL = "https://graph.microsoft.com/v1.0"

// Config holds OneDrive connection parameters.
type Config struct {
	AccessToken string
	TokenSource cloudutil.AccessTokenSource
}

// FS implements vfs.FileSystem over the OneDrive API.
type FS struct {
	client      *http.Client
	tokenSource cloudutil.AccessTokenSource
}

var _ vfs.FileSystem = (*FS)(nil)
var _ vfs.CapabilityFileSystem = (*FS)(nil)
var _ vfs.PagedFileSystem = (*FS)(nil)

func (f *FS) ProviderCapabilities() vfs.ProviderCapabilities {
	return vfs.ProviderCapabilities{Pagination: true, RandomAccess: false, TotalCount: false, ServerSort: false, ServerFilter: false, RangeRead: false, Watch: false, Rename: false, AtomicMove: false, Trash: false}
}

type item struct {
	Name                 string          `json:"name"`
	Size                 int64           `json:"size"`
	LastModifiedDateTime string          `json:"lastModifiedDateTime"`
	File                 json.RawMessage `json:"file"`
	Folder               json.RawMessage `json:"folder"`
}

type childrenResponse struct {
	Value    []item `json:"value"`
	NextLink string `json:"@odata.nextLink"`
}

func New(ctx context.Context, cfg Config) (*FS, error) {
	source := cfg.TokenSource
	if source == nil {
		var err error
		source, err = cloudutil.NewStaticAccessTokenSource(cfg.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("onedrive: %w", err)
		}
	}

	fs := &FS{
		client:      &http.Client{Timeout: 60 * time.Second},
		tokenSource: source,
	}
	if _, err := fs.fetchItem(ctx, ""); err != nil {
		return nil, err
	}
	return fs, nil
}

func (f *FS) Open(ctx context.Context, p string) (vfs.File, error) {
	cleaned := cloudutil.CleanPath(p)
	if cleaned == "" {
		return nil, vfs.ErrNotFile
	}

	meta, err := f.fetchItem(ctx, cleaned)
	if err != nil {
		return nil, err
	}
	info := toInfo(meta)
	if info.IsDir() {
		return nil, vfs.ErrNotFile
	}

	return cloudutil.NewReadOnlyFile(ctx, info, func(ctx context.Context) ([]byte, error) {
		return f.download(ctx, cleaned)
	}), nil
}

func (f *FS) Stat(ctx context.Context, p string) (vfs.FileInfo, error) {
	cleaned := cloudutil.CleanPath(p)
	meta, err := f.fetchItem(ctx, cleaned)
	if err != nil {
		return nil, err
	}
	info := toInfo(meta)
	if cleaned == "" && info.Name() == "" {
		info.NameValue = "OneDrive"
	}
	return info, nil
}

func (f *FS) ReadDir(ctx context.Context, p string) ([]vfs.FileInfo, error) {
	cleaned := cloudutil.CleanPath(p)
	if cleaned != "" {
		meta, err := f.fetchItem(ctx, cleaned)
		if err != nil {
			return nil, err
		}
		if len(meta.Folder) == 0 {
			return nil, vfs.ErrNotDir
		}
	}

	nextURL := graphBaseURL + childrenEndpoint(cleaned) + "?$select=name,size,lastModifiedDateTime,file,folder"
	result := make([]vfs.FileInfo, 0)
	for nextURL != "" {
		var response childrenResponse
		if err := f.getJSON(ctx, nextURL, &response); err != nil {
			return nil, err
		}
		for _, entry := range response.Value {
			result = append(result, toInfo(entry))
		}
		nextURL = response.NextLink
	}
	return result, nil
}

func (f *FS) ReadDirPage(ctx context.Context, p, token string, limit int) (vfs.DirectoryPage, error) {
	cleaned := cloudutil.CleanPath(p)
	if cleaned != "" {
		meta, err := f.fetchItem(ctx, cleaned)
		if err != nil {
			return vfs.DirectoryPage{}, err
		}
		if len(meta.Folder) == 0 {
			return vfs.DirectoryPage{}, vfs.ErrNotDir
		}
	}
	if limit <= 0 || limit > 999 {
		limit = 999
	}
	nextURL := token
	if nextURL == "" {
		nextURL = graphBaseURL + childrenEndpoint(cleaned) + "?$select=name,size,lastModifiedDateTime,file,folder&$top=" + strconv.Itoa(limit)
	}
	var response childrenResponse
	if err := f.getJSON(ctx, nextURL, &response); err != nil {
		return vfs.DirectoryPage{}, err
	}
	entries := make([]vfs.FileInfo, 0, len(response.Value))
	for _, entry := range response.Value {
		entries = append(entries, toInfo(entry))
	}
	return vfs.DirectoryPage{Entries: entries, NextToken: response.NextLink}, nil
}

func (f *FS) Exists(ctx context.Context, p string) bool {
	_, err := f.Stat(ctx, p)
	return err == nil
}

func (f *FS) Create(_ context.Context, _ string) (vfs.File, error) { return nil, vfs.ErrReadOnly }
func (f *FS) Mkdir(_ context.Context, _ string, _ fs.FileMode) error {
	return vfs.ErrReadOnly
}
func (f *FS) Remove(_ context.Context, _ string) error { return vfs.ErrReadOnly }
func (f *FS) Rename(_ context.Context, _, _ string) error {
	return vfs.ErrReadOnly
}
func (f *FS) Join(elem ...string) string        { return path.Join(elem...) }
func (f *FS) Split(p string) (string, string)   { return path.Split(p) }
func (f *FS) IsAbs(p string) bool               { return strings.HasPrefix(p, "/") }
func (f *FS) RealPath(_ string) (string, error) { return "", nil }

func (f *FS) fetchItem(ctx context.Context, cleaned string) (item, error) {
	endpoint := graphBaseURL + itemEndpoint(cleaned)
	query := "?$select=name,size,lastModifiedDateTime,file,folder"
	var response item
	if err := f.getJSON(ctx, endpoint+query, &response); err != nil {
		return item{}, err
	}
	return response, nil
}

func (f *FS) download(ctx context.Context, cleaned string) ([]byte, error) {
	request, err := f.newRequest(ctx, http.MethodGet, graphBaseURL+contentEndpoint(cleaned), nil)
	if err != nil {
		return nil, err
	}
	response, err := f.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return nil, vfs.ErrNotFound
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024))
		return nil, fmt.Errorf("onedrive: download failed: HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(response.Body)
}

func (f *FS) getJSON(ctx context.Context, endpoint string, target any) error {
	request, err := f.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := f.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return vfs.ErrNotFound
	default:
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024))
		return fmt.Errorf("onedrive: request failed: HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func (f *FS) newRequest(ctx context.Context, method string, endpoint string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	token, err := f.tokenSource.AccessToken(ctx)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	return request, nil
}

func itemEndpoint(cleaned string) string {
	if cleaned == "" {
		return "/me/drive/root"
	}
	return "/me/drive/root:/" + escapePath(cleaned) + ":"
}

func childrenEndpoint(cleaned string) string {
	if cleaned == "" {
		return "/me/drive/root/children"
	}
	return itemEndpoint(cleaned) + "/children"
}

func contentEndpoint(cleaned string) string {
	return itemEndpoint(cleaned) + "/content"
}

func escapePath(cleaned string) string {
	parts := strings.Split(cleaned, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func toInfo(meta item) *cloudutil.Info {
	return &cloudutil.Info{
		NameValue:    meta.Name,
		SizeValue:    meta.Size,
		ModifiedTime: parseTime(meta.LastModifiedDateTime),
		Dir:          len(meta.Folder) > 0,
	}
}

func parseTime(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339, value)
	return parsed
}