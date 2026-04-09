// Package gdrive provides a read-only vfs.FileSystem backed by the Google Drive API.
package gdrive

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
	"sync"
	"time"

	"wails-app/vfs"
	"wails-app/vfs/cloudutil"
)

const apiBaseURL = "https://www.googleapis.com/drive/v3"

// Config holds Google Drive connection parameters.
type Config struct {
	AccessToken string
	TokenSource cloudutil.AccessTokenSource
}

// FS implements vfs.FileSystem over the Google Drive API.
type FS struct {
	client        *http.Client
	tokenSource   cloudutil.AccessTokenSource
	rootName      string
	childrenMu    sync.RWMutex
	childrenCache map[string][]fileMetadata
}

var _ vfs.FileSystem = (*FS)(nil)

type fileMetadata struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	MimeType        string          `json:"mimeType"`
	Size            string          `json:"size,omitempty"`
	ModifiedTime    string          `json:"modifiedTime,omitempty"`
	ShortcutDetails shortcutDetails `json:"shortcutDetails,omitempty"`
}

type shortcutDetails struct {
	TargetID       string `json:"targetId,omitempty"`
	TargetMimeType string `json:"targetMimeType,omitempty"`
}

type listResponse struct {
	Files         []fileMetadata `json:"files"`
	NextPageToken string         `json:"nextPageToken"`
}

func New(ctx context.Context, cfg Config) (*FS, error) {
	source := cfg.TokenSource
	if source == nil {
		var err error
		source, err = cloudutil.NewStaticAccessTokenSource(cfg.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("gdrive: %w", err)
		}
	}

	fs := &FS{
		client:        &http.Client{Timeout: 60 * time.Second},
		tokenSource:   source,
		childrenCache: make(map[string][]fileMetadata),
	}

	root, err := fs.fetchMetadataByID(ctx, "root")
	if err != nil {
		return nil, err
	}
	fs.rootName = root.Name
	return fs, nil
}

func (f *FS) Open(ctx context.Context, p string) (vfs.File, error) {
	cleaned := cloudutil.CleanPath(p)
	if cleaned == "" {
		return nil, vfs.ErrNotFile
	}

	meta, err := f.resolvePath(ctx, cleaned)
	if err != nil {
		return nil, err
	}
	info := toInfo(meta)
	if info.IsDir() {
		return nil, vfs.ErrNotFile
	}
	if isGoogleWorkspaceFile(meta) {
		return nil, fmt.Errorf("gdrive: native Google Workspace files are not downloadable yet")
	}

	id := resolvedID(meta)
	return cloudutil.NewReadOnlyFile(ctx, info, func(ctx context.Context) ([]byte, error) {
		return f.download(ctx, id)
	}), nil
}

func (f *FS) Stat(ctx context.Context, p string) (vfs.FileInfo, error) {
	cleaned := cloudutil.CleanPath(p)
	if cleaned == "" {
		name := f.rootName
		if name == "" {
			name = "Google Drive"
		}
		return &cloudutil.Info{NameValue: name, Dir: true}, nil
	}

	meta, err := f.resolvePath(ctx, cleaned)
	if err != nil {
		return nil, err
	}
	return toInfo(meta), nil
}

func (f *FS) ReadDir(ctx context.Context, p string) ([]vfs.FileInfo, error) {
	cleaned := cloudutil.CleanPath(p)
	parentID := "root"
	if cleaned != "" {
		meta, err := f.resolvePath(ctx, cleaned)
		if err != nil {
			return nil, err
		}
		if resolvedMimeType(meta) != folderMimeType {
			return nil, vfs.ErrNotDir
		}
		parentID = resolvedID(meta)
	}

	children, err := f.listChildren(ctx, parentID)
	if err != nil {
		return nil, err
	}

	result := make([]vfs.FileInfo, 0, len(children))
	for _, child := range children {
		result = append(result, toInfo(child))
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

func (f *FS) resolvePath(ctx context.Context, cleaned string) (fileMetadata, error) {
	currentID := "root"
	segments := strings.Split(cleaned, "/")
	for index, segment := range segments {
		children, err := f.listChildren(ctx, currentID)
		if err != nil {
			return fileMetadata{}, err
		}

		candidate, ok := chooseByName(children, segment, index < len(segments)-1)
		if !ok {
			return fileMetadata{}, vfs.ErrNotFound
		}

		if index < len(segments)-1 && resolvedMimeType(candidate) != folderMimeType {
			return fileMetadata{}, vfs.ErrNotFound
		}

		currentID = resolvedID(candidate)
		if index == len(segments)-1 {
			return candidate, nil
		}
	}
	return fileMetadata{}, vfs.ErrNotFound
}

func (f *FS) listChildren(ctx context.Context, parentID string) ([]fileMetadata, error) {
	f.childrenMu.RLock()
	if cached, ok := f.childrenCache[parentID]; ok {
		f.childrenMu.RUnlock()
		return append([]fileMetadata(nil), cached...), nil
	}
	f.childrenMu.RUnlock()

	query := url.Values{}
	query.Set("q", fmt.Sprintf("'%s' in parents and trashed = false", strings.ReplaceAll(parentID, "'", "\\'")))
	query.Set("fields", "nextPageToken,files(id,name,mimeType,size,modifiedTime,shortcutDetails(targetId,targetMimeType))")
	query.Set("pageSize", "1000")
	query.Set("supportsAllDrives", "true")
	query.Set("includeItemsFromAllDrives", "true")

	nextToken := ""
	files := make([]fileMetadata, 0)
	for {
		pageQuery := cloneValues(query)
		if nextToken != "" {
			pageQuery.Set("pageToken", nextToken)
		}
		var response listResponse
		if err := f.getJSON(ctx, apiBaseURL+"/files?"+pageQuery.Encode(), &response); err != nil {
			return nil, err
		}
		files = append(files, response.Files...)
		if response.NextPageToken == "" {
			break
		}
		nextToken = response.NextPageToken
	}

	f.childrenMu.Lock()
	f.childrenCache[parentID] = append([]fileMetadata(nil), files...)
	f.childrenMu.Unlock()
	return files, nil
}

func (f *FS) fetchMetadataByID(ctx context.Context, id string) (fileMetadata, error) {
	query := url.Values{}
	query.Set("fields", "id,name,mimeType,size,modifiedTime,shortcutDetails(targetId,targetMimeType)")
	query.Set("supportsAllDrives", "true")

	var meta fileMetadata
	if err := f.getJSON(ctx, apiBaseURL+"/files/"+url.PathEscape(id)+"?"+query.Encode(), &meta); err != nil {
		return fileMetadata{}, err
	}
	return meta, nil
}

func (f *FS) download(ctx context.Context, id string) ([]byte, error) {
	query := url.Values{}
	query.Set("alt", "media")
	query.Set("supportsAllDrives", "true")

	request, err := f.newRequest(ctx, http.MethodGet, apiBaseURL+"/files/"+url.PathEscape(id)+"?"+query.Encode())
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
		return nil, fmt.Errorf("gdrive: download failed: HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(response.Body)
}

func (f *FS) getJSON(ctx context.Context, endpoint string, target any) error {
	request, err := f.newRequest(ctx, http.MethodGet, endpoint)
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
		return fmt.Errorf("gdrive: request failed: HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(response.Body).Decode(target)
}

func (f *FS) newRequest(ctx context.Context, method string, endpoint string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
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

func chooseByName(children []fileMetadata, name string, wantDir bool) (fileMetadata, bool) {
	for _, child := range children {
		if child.Name == name && resolvedMimeType(child) == folderMimeType && wantDir {
			return child, true
		}
	}
	for _, child := range children {
		if child.Name == name {
			return child, true
		}
	}
	return fileMetadata{}, false
}

func toInfo(meta fileMetadata) *cloudutil.Info {
	return &cloudutil.Info{
		NameValue:    meta.Name,
		SizeValue:    parseSize(meta.Size),
		ModifiedTime: parseTime(meta.ModifiedTime),
		Dir:          resolvedMimeType(meta) == folderMimeType,
	}
}

func resolvedID(meta fileMetadata) string {
	if meta.ShortcutDetails.TargetID != "" {
		return meta.ShortcutDetails.TargetID
	}
	return meta.ID
}

func resolvedMimeType(meta fileMetadata) string {
	if meta.ShortcutDetails.TargetMimeType != "" {
		return meta.ShortcutDetails.TargetMimeType
	}
	return meta.MimeType
}

func parseSize(value string) int64 {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func parseTime(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339, value)
	return parsed
}

func cloneValues(source url.Values) url.Values {
	target := make(url.Values, len(source))
	for key, values := range source {
		target[key] = append([]string(nil), values...)
	}
	return target
}

const folderMimeType = "application/vnd.google-apps.folder"

func isGoogleWorkspaceFile(meta fileMetadata) bool {
	mimeType := resolvedMimeType(meta)
	return strings.HasPrefix(mimeType, "application/vnd.google-apps.") && mimeType != folderMimeType
}
