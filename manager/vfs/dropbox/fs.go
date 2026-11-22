// Package dropbox provides a read-only vfs.FileSystem backed by the Dropbox API.
package dropbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"wails-app/vfs"
	"wails-app/vfs/cloudutil"
)

const (
	apiBaseURL     = "https://api.dropboxapi.com/2"
	contentBaseURL = "https://content.dropboxapi.com/2"
)

// Config holds Dropbox connection parameters.
type Config struct {
	AccessToken string
	TokenSource cloudutil.AccessTokenSource
}

// FS implements vfs.FileSystem over the Dropbox file API.
type FS struct {
	client      *http.Client
	tokenSource cloudutil.AccessTokenSource
}

var _ vfs.FileSystem = (*FS)(nil)

type metadata struct {
	Tag            string `json:".tag"`
	Name           string `json:"name"`
	Size           int64  `json:"size,omitempty"`
	ClientModified string `json:"client_modified,omitempty"`
	ServerModified string `json:"server_modified,omitempty"`
}

type listFolderResponse struct {
	Entries []metadata `json:"entries"`
	Cursor  string     `json:"cursor"`
	HasMore bool       `json:"has_more"`
}

func New(ctx context.Context, cfg Config) (*FS, error) {
	source := cfg.TokenSource
	if source == nil {
		var err error
		source, err = cloudutil.NewStaticAccessTokenSource(cfg.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("dropbox: %w", err)
		}
	}

	fs := &FS{
		client:      &http.Client{Timeout: 60 * time.Second},
		tokenSource: source,
	}
	if _, err := fs.listFolder(ctx, ""); err != nil {
		return nil, err
	}
	return fs, nil
}

func (f *FS) Open(ctx context.Context, p string) (vfs.File, error) {
	cleaned := cloudutil.CleanPath(p)
	if cleaned == "" {
		return nil, vfs.ErrNotFile
	}

	info, err := f.statMetadata(ctx, cleaned)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, vfs.ErrNotFile
	}

	return cloudutil.NewReadOnlyFile(ctx, info, func(ctx context.Context, rangeHeader string) (*http.Response, error) {
		return f.download(ctx, cleaned, rangeHeader)
	}), nil
}

func (f *FS) Stat(ctx context.Context, p string) (vfs.FileInfo, error) {
	cleaned := cloudutil.CleanPath(p)
	if cleaned == "" {
		return &cloudutil.Info{NameValue: "Dropbox", Dir: true}, nil
	}
	return f.statMetadata(ctx, cleaned)
}

func (f *FS) ReadDir(ctx context.Context, p string) ([]vfs.FileInfo, error) {
	cleaned := cloudutil.CleanPath(p)
	if cleaned != "" {
		info, err := f.statMetadata(ctx, cleaned)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, vfs.ErrNotDir
		}
	}

	entries, err := f.listFolder(ctx, cleaned)
	if err != nil {
		return nil, err
	}

	result := make([]vfs.FileInfo, 0, len(entries))
	for _, entry := range entries {
		result = append(result, toInfo(entry))
	}
	return result, nil
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

func (f *FS) statMetadata(ctx context.Context, cleaned string) (vfs.FileInfo, error) {
	var meta metadata
	if err := f.postJSON(ctx, apiBaseURL+"/files/get_metadata", map[string]string{
		"path": dropboxPath(cleaned),
	}, &meta, nil); err != nil {
		return nil, err
	}
	return toInfo(meta), nil
}

func (f *FS) listFolder(ctx context.Context, cleaned string) ([]metadata, error) {
	var response listFolderResponse
	if err := f.postJSON(ctx, apiBaseURL+"/files/list_folder", map[string]any{
		"path": dropboxPath(cleaned),
	}, &response, nil); err != nil {
		return nil, err
	}

	entries := append([]metadata{}, response.Entries...)
	cursor := response.Cursor
	for response.HasMore {
		response = listFolderResponse{}
		if err := f.postJSON(ctx, apiBaseURL+"/files/list_folder/continue", map[string]string{
			"cursor": cursor,
		}, &response, nil); err != nil {
			return nil, err
		}
		entries = append(entries, response.Entries...)
		cursor = response.Cursor
	}

	return entries, nil
}

func (f *FS) download(ctx context.Context, cleaned string, rangeHeader string) (*http.Response, error) {
	headers := map[string]string{
		"Dropbox-API-Arg": mustJSON(map[string]string{"path": dropboxPath(cleaned)}),
	}
	request, err := f.newRequest(ctx, http.MethodPost, contentBaseURL+"/files/download", nil, headers)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Range", rangeHeader)
	request.Header.Set("Accept-Encoding", "identity")
	response, err := f.client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		return nil, vfs.ErrNotFound
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		defer response.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024))
		return nil, fmt.Errorf("dropbox: download failed: HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return response, nil
}

func (f *FS) postJSON(ctx context.Context, endpoint string, body any, target any, headers map[string]string) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := f.newRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(payload), headers)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

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
		return fmt.Errorf("dropbox: request failed: HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	return json.NewDecoder(response.Body).Decode(target)
}

func (f *FS) newRequest(ctx context.Context, method string, endpoint string, body io.Reader, headers map[string]string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	token, err := f.tokenSource.AccessToken(ctx)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return request, nil
}

func dropboxPath(cleaned string) string {
	if cleaned == "" {
		return ""
	}
	return "/" + cleaned
}

func toInfo(meta metadata) *cloudutil.Info {
	modified := parseTime(meta.ServerModified)
	if modified.IsZero() {
		modified = parseTime(meta.ClientModified)
	}
	return &cloudutil.Info{
		NameValue:    meta.Name,
		SizeValue:    meta.Size,
		ModifiedTime: modified,
		Dir:          meta.Tag == "folder",
	}
}

func parseTime(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339, value)
	return parsed
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
