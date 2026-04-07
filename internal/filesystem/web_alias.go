package filesystem

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	vpath "path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"wails-app/internal/contentprobe"
	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	"wails-app/internal/webresourcecache"
)

const (
	webAliasTimeout   = 30 * time.Second
	webAliasReadLimit = 4 * 1024 * 1024
)

type webAliasService struct {
	client   *http.Client
	metaMu   sync.RWMutex
	metadata map[string]webAliasMetadata
}

type webAliasMetadata struct {
	size       int64
	mimeType   string
	modifiedAt int64
	fileType   contracts.FileType
}

func newWebAliasService() *webAliasService {
	return &webAliasService{
		client:   &http.Client{Timeout: webAliasTimeout},
		metadata: make(map[string]webAliasMetadata),
	}
}

func (s *webAliasService) lookupMetadata(path string) (webAliasMetadata, bool) {
	s.metaMu.RLock()
	defer s.metaMu.RUnlock()
	meta, ok := s.metadata[path]
	return meta, ok
}

func (s *webAliasService) rememberMetadata(path string, meta webAliasMetadata) {
	if path == "" {
		return
	}

	s.metaMu.Lock()
	defer s.metaMu.Unlock()

	current := s.metadata[path]
	if meta.size > 0 {
		current.size = meta.size
	}
	if meta.mimeType != "" {
		current.mimeType = meta.mimeType
	}
	if meta.modifiedAt > 0 {
		current.modifiedAt = meta.modifiedAt
	}
	if meta.fileType != "" {
		current.fileType = meta.fileType
	}
	s.metadata[path] = current
}

func (s *webAliasService) applyCachedMetadata(entries []contracts.FileInfo) []contracts.FileInfo {
	result := make([]contracts.FileInfo, len(entries))
	copy(result, entries)

	for i, entry := range result {
		meta, ok := s.lookupMetadata(entry.Path)
		if !ok {
			continue
		}
		if meta.size > 0 {
			result[i].Size = meta.size
		}
		if meta.mimeType != "" {
			result[i].MimeType = meta.mimeType
		}
		if meta.modifiedAt > 0 {
			result[i].Created = meta.modifiedAt
			result[i].Modified = meta.modifiedAt
			result[i].Accessed = meta.modifiedAt
		}
		if meta.fileType != "" {
			result[i].Type = meta.fileType
		}
	}

	return result
}

func (s *webAliasService) handles(path string) bool {
	return pathref.IsWeb(path)
}

func (s *webAliasService) listDirectory(path string) (*contracts.DirectoryListing, error) {
	requestedPath := pathref.Normalize(path)
	targetURL, rootOnly, err := normalizeWebAliasPath(requestedPath)
	if err != nil {
		return nil, err
	}

	if rootOnly {
		return &contracts.DirectoryListing{
			Path:      requestedPath,
			Entries:   []contracts.FileInfo{},
			TotalSize: 0,
		}, nil
	}

	resp, err := s.request(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("request failed: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, webAliasReadLimit))
	if err != nil {
		return nil, err
	}

	entries := []contracts.FileInfo{}
	if isHTMLResponse(resp.Header.Get("Content-Type"), body) {
		entries = discoverWebEntries(targetURL, body)
		if len(entries) == 0 {
			entries = cachedWebResources(targetURL)
		}
		entries = s.applyCachedMetadata(entries)
	}

	return &contracts.DirectoryListing{
		Path:      requestedPath,
		Entries:   entries,
		TotalSize: sumFileSizes(entries),
	}, nil
}

func (s *webAliasService) getFileInfo(path string) (*contracts.FileInfo, error) {
	requestedPath := pathref.Normalize(path)
	targetURL, rootOnly, err := normalizeWebAliasPath(requestedPath)
	if err != nil {
		return nil, err
	}

	if rootOnly {
		return &contracts.FileInfo{
			Name:        strings.TrimSuffix(requestedPath, "://"),
			Path:        requestedPath,
			Type:        contracts.FileTypeDirectory,
			Permissions: "r--",
			IsHidden:    false,
		}, nil
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	var (
		contentType string
		size        int64
		modifiedAt  int64
	)

	if cached, ok := s.lookupMetadata(requestedPath); ok {
		contentType = cached.mimeType
		size = cached.size
		modifiedAt = cached.modifiedAt
	}

	if contentType == "" || size == 0 {
		sample, headers, sampleSize, sampleErr := s.fetchSample(targetURL)
		if sampleErr == nil {
			if contentType == "" {
				contentType = normalizedMediaType(headers.Get("Content-Type"))
				if contentType == "" {
					contentType = contentprobe.ProbeBytes(sample).MimeType
				}
			}
			if size == 0 {
				size = sampleSize
			}
			if modifiedAt == 0 {
				modifiedAt = parseHTTPModified(headers.Get("Last-Modified"))
			}
		}
	}

	fileType := contracts.FileTypeFile
	if contentType != "" && isHTMLMediaType(contentType) {
		fileType = contracts.FileTypeDirectory
	} else if parsed.RawQuery == "" && isLikelyWebPage(parsed) {
		fileType = contracts.FileTypeDirectory
	}

	s.rememberMetadata(requestedPath, webAliasMetadata{
		size:       size,
		mimeType:   contentType,
		modifiedAt: modifiedAt,
		fileType:   fileType,
	})

	return &contracts.FileInfo{
		Name:        webEntryDisplayName(parsed),
		Path:        requestedPath,
		Type:        fileType,
		MimeType:    contentType,
		Size:        size,
		Created:     modifiedAt,
		Modified:    modifiedAt,
		Accessed:    modifiedAt,
		Permissions: "r--",
		IsHidden:    false,
	}, nil
}

func (s *webAliasService) probeFile(path string) (*contracts.FileProbe, error) {
	targetURL, rootOnly, err := normalizeWebAliasPath(path)
	if err != nil {
		return nil, err
	}
	if rootOnly {
		return nil, fmt.Errorf("cannot probe scheme root")
	}

	sample, headers, _, err := s.fetchSample(targetURL)
	if err != nil {
		return nil, err
	}

	probe := contentprobe.ProbeBytes(sample)
	if headerType := normalizedMediaType(headers.Get("Content-Type")); headerType != "" {
		probe.MimeType = headerType
		probe.IsText = probe.IsText || isTextMediaType(headerType)
	}

	fileType := contracts.FileTypeFile
	if probe.MimeType != "" && isHTMLMediaType(probe.MimeType) {
		fileType = contracts.FileTypeDirectory
	}
	s.rememberMetadata(pathref.Normalize(path), webAliasMetadata{
		size:       responseSize(&http.Response{Header: headers}, 0),
		mimeType:   probe.MimeType,
		modifiedAt: parseHTTPModified(headers.Get("Last-Modified")),
		fileType:   fileType,
	})

	return &contracts.FileProbe{
		MimeType:  probe.MimeType,
		Encoding:  probe.Encoding,
		IsText:    probe.IsText,
		IsArchive: probe.IsArchive,
	}, nil
}

func (s *webAliasService) readFile(path string, encoding string) (*contracts.FileContent, error) {
	requestedPath := pathref.Normalize(path)
	targetURL, rootOnly, err := normalizeWebAliasPath(requestedPath)
	if err != nil {
		return nil, err
	}
	if rootOnly {
		return nil, fmt.Errorf("cannot read scheme root")
	}

	resp, err := s.request(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("request failed: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
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

	totalSize := responseSize(resp, int64(len(data)))
	fileType := contracts.FileTypeFile
	if probe.MimeType != "" && isHTMLMediaType(probe.MimeType) {
		fileType = contracts.FileTypeDirectory
	}
	s.rememberMetadata(requestedPath, webAliasMetadata{
		size:       totalSize,
		mimeType:   probe.MimeType,
		modifiedAt: parseHTTPModified(resp.Header.Get("Last-Modified")),
		fileType:   fileType,
	})

	return &contracts.FileContent{
		Path:     requestedPath,
		Content:  content,
		Encoding: encoding,
		Size:     totalSize,
	}, nil
}

func (s *webAliasService) readFileRange(path string, offset int64, length int) (*contracts.FileContent, error) {
	requestedPath := pathref.Normalize(path)
	targetURL, rootOnly, err := normalizeWebAliasPath(requestedPath)
	if err != nil {
		return nil, err
	}
	if rootOnly {
		return nil, fmt.Errorf("cannot read scheme root")
	}
	if length <= 0 {
		return &contracts.FileContent{
			Path:     requestedPath,
			Content:  "",
			Encoding: "base64",
			Size:     0,
		}, nil
	}

	headers := map[string]string{
		"Range": fmt.Sprintf("bytes=%d-%d", offset, offset+int64(length)-1),
	}
	resp, err := s.request(http.MethodGet, targetURL, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return &contracts.FileContent{
			Path:     requestedPath,
			Content:  "",
			Encoding: "base64",
			Size:     0,
		}, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("request failed: HTTP %d", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusOK && offset > 0 {
		if _, err := io.CopyN(io.Discard, resp.Body, offset); err != nil && err != io.EOF {
			return nil, err
		}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(length)))
	if err != nil {
		return nil, err
	}

	totalSize := responseSize(resp, offset+int64(len(data)))
	s.rememberMetadata(requestedPath, webAliasMetadata{
		size:       totalSize,
		mimeType:   normalizedMediaType(resp.Header.Get("Content-Type")),
		modifiedAt: parseHTTPModified(resp.Header.Get("Last-Modified")),
		fileType:   contracts.FileTypeFile,
	})

	return &contracts.FileContent{
		Path:     requestedPath,
		Content:  base64.StdEncoding.EncodeToString(data),
		Encoding: "base64",
		Size:     totalSize,
	}, nil
}

func (s *webAliasService) fetchSample(targetURL string) ([]byte, http.Header, int64, error) {
	resp, err := s.request(http.MethodGet, targetURL, map[string]string{
		"Range": fmt.Sprintf("bytes=0-%d", contentprobe.SampleLimit-1),
	})
	if err != nil {
		return nil, nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return nil, nil, 0, fmt.Errorf("request failed: HTTP %d", resp.StatusCode)
	}

	sample, err := io.ReadAll(io.LimitReader(resp.Body, contentprobe.SampleLimit))
	if err != nil {
		return nil, nil, 0, err
	}

	return sample, resp.Header.Clone(), responseSize(resp, int64(len(sample))), nil
}

func (s *webAliasService) request(method string, targetURL string, extraHeaders map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "WailsFileManager/1.0")
	for key, value := range extraHeaders {
		if strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	return s.client.Do(req)
}

func normalizeWebAliasPath(path string) (string, bool, error) {
	normalized := pathref.Normalize(path)
	switch normalized {
	case pathref.HTTPAliasScheme, pathref.HTTPSAliasScheme:
		return normalized, true, nil
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", false, err
	}
	if parsed.Host == "" {
		return "", false, fmt.Errorf("invalid web alias path %q", path)
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return "", false, fmt.Errorf("unsupported web alias scheme %q", parsed.Scheme)
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	parsed.Path = strings.ReplaceAll(parsed.Path, "\\", "/")
	if parsed.Path == "/" {
		parsed.Path = ""
	} else {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}
	parsed.RawPath = ""

	return parsed.String(), false, nil
}

func discoverWebEntries(pageURL string, body []byte) []contracts.FileInfo {
	baseURL, err := url.Parse(pageURL)
	if err != nil {
		return []contracts.FileInfo{}
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return []contracts.FileInfo{}
	}
	if !looksLikeDirectoryListing(doc) {
		return []contracts.FileInfo{}
	}

	resolvedBase := findHTMLBaseURL(doc, directoryListingBaseURL(baseURL))
	currentURL := canonicalWebURL(baseURL)
	entries := make(map[string]contracts.FileInfo)

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}

		if node.Type == html.ElementNode {
			switch strings.ToLower(node.Data) {
			case "a":
				addWebEntry(entries, resolvedBase, currentURL, attrValue(node, "href"), true)
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(doc)

	if len(entries) == 0 {
		return []contracts.FileInfo{}
	}

	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	files := make([]contracts.FileInfo, 0, len(keys))
	for _, key := range keys {
		files = append(files, entries[key])
	}
	return files
}

func directoryListingBaseURL(baseURL *url.URL) *url.URL {
	if baseURL == nil {
		return nil
	}

	resolved := *baseURL
	if resolved.Path == "" {
		resolved.Path = "/"
	} else if !strings.HasSuffix(resolved.Path, "/") {
		resolved.Path += "/"
	}
	return &resolved
}

func addWebEntry(entries map[string]contracts.FileInfo, baseURL *url.URL, currentURL string, rawRef string, navigable bool) {
	resolved, ok := resolveWebReference(baseURL, rawRef)
	if !ok {
		return
	}
	if strings.TrimSpace(resolved.RawQuery) != "" {
		return
	}

	target := canonicalWebURL(resolved)
	if target == "" || target == currentURL {
		return
	}
	if _, exists := entries[target]; exists {
		return
	}

	fileType := contracts.FileTypeFile
	if navigable && isLikelyWebPage(resolved) {
		fileType = contracts.FileTypeDirectory
	}

	entries[target] = contracts.FileInfo{
		Name:        webEntryDisplayName(resolved),
		Path:        target,
		Type:        fileType,
		MimeType:    mime.TypeByExtension(strings.ToLower(vpath.Ext(resolved.Path))),
		Permissions: "r--",
		IsHidden:    false,
	}
}

func cachedWebResources(pageURL string) []contracts.FileInfo {
	resources := webresourcecache.Load(pageURL)
	if len(resources) == 0 {
		return []contracts.FileInfo{}
	}

	entries := make([]contracts.FileInfo, 0, len(resources))
	for _, resource := range resources {
		parsed, err := url.Parse(resource.URL)
		if err != nil {
			continue
		}
		if !shouldIncludeCachedWebResource(parsed, resource.InitiatorType, resource.MimeType, resource.Size) {
			continue
		}
		entries = append(entries, contracts.FileInfo{
			Name:        webEntryDisplayName(parsed),
			Path:        canonicalWebURL(parsed),
			Type:        contracts.FileTypeFile,
			MimeType:    chooseCachedResourceMimeType(parsed, resource.MimeType),
			Size:        resource.Size,
			Permissions: "r--",
			IsHidden:    false,
		})
	}
	if len(entries) == 0 {
		return []contracts.FileInfo{}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries
}

func shouldIncludeCachedWebResource(parsed *url.URL, initiatorType string, mimeType string, size int64) bool {
	if parsed == nil || size <= 0 {
		return false
	}
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	ext := strings.ToLower(vpath.Ext(parsed.Path))
	if ext != "" {
		if isLikelyWebPage(parsed) {
			return false
		}
		return true
	}
	if strings.HasPrefix(mimeType, "application/javascript") ||
		strings.HasPrefix(mimeType, "text/javascript") ||
		strings.HasPrefix(mimeType, "text/css") ||
		strings.HasPrefix(mimeType, "image/") ||
		strings.HasPrefix(mimeType, "audio/") ||
		strings.HasPrefix(mimeType, "video/") ||
		strings.HasPrefix(mimeType, "font/") ||
		strings.Contains(mimeType, "font") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(initiatorType)) {
	case "script", "css", "img", "image", "audio", "video", "track", "font", "link", "beacon":
		return true
	default:
		return false
	}
}

func chooseCachedResourceMimeType(parsed *url.URL, cached string) string {
	if trimmed := strings.TrimSpace(cached); trimmed != "" {
		return trimmed
	}
	return mime.TypeByExtension(strings.ToLower(vpath.Ext(parsed.Path)))
}

func resolveWebReference(baseURL *url.URL, rawRef string) (*url.URL, bool) {
	ref := strings.TrimSpace(rawRef)
	if ref == "" {
		return nil, false
	}

	lower := strings.ToLower(ref)
	if strings.HasPrefix(lower, "#") ||
		strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "data:") {
		return nil, false
	}

	parsed, err := url.Parse(ref)
	if err != nil {
		return nil, false
	}
	resolved := baseURL.ResolveReference(parsed)
	switch strings.ToLower(resolved.Scheme) {
	case "http", "https":
		resolved.Fragment = ""
		return resolved, true
	default:
		return nil, false
	}
}

func findHTMLBaseURL(doc *html.Node, fallback *url.URL) *url.URL {
	var walk func(*html.Node) *url.URL
	walk = func(node *html.Node) *url.URL {
		if node == nil {
			return nil
		}

		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "base") {
			href := strings.TrimSpace(attrValue(node, "href"))
			if href != "" {
				if parsed, ok := resolveWebReference(fallback, href); ok {
					return parsed
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if resolved := walk(child); resolved != nil {
				return resolved
			}
		}
		return nil
	}

	if resolved := walk(doc); resolved != nil {
		return resolved
	}
	return fallback
}

func looksLikeDirectoryListing(doc *html.Node) bool {
	for _, tag := range []string{"title", "h1"} {
		if label := strings.ToLower(strings.TrimSpace(firstElementText(doc, tag))); isDirectoryListingLabel(label) {
			return true
		}
	}
	return false
}

func isDirectoryListingLabel(label string) bool {
	return strings.HasPrefix(label, "index of") || strings.HasPrefix(label, "directory listing for")
}

func firstElementText(root *html.Node, tag string) string {
	var walk func(*html.Node) string
	walk = func(node *html.Node) string {
		if node == nil {
			return ""
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, tag) {
			return nodeText(node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if text := walk(child); text != "" {
				return text
			}
		}
		return ""
	}
	return walk(root)
}

func nodeText(node *html.Node) string {
	var parts []string
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current == nil {
			return
		}
		if current.Type == html.TextNode {
			if text := strings.TrimSpace(current.Data); text != "" {
				parts = append(parts, text)
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(parts, " ")
}

func attrValue(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return attr.Val
		}
	}
	return ""
}

func parseSrcset(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 {
			continue
		}
		values = append(values, fields[0])
	}
	return values
}

func webEntryDisplayName(parsed *url.URL) string {
	trimmedPath := strings.Trim(strings.TrimRight(parsed.Path, "/"), "/")
	if trimmedPath == "" {
		if parsed.RawQuery != "" {
			return parsed.Host + "?" + parsed.RawQuery
		}
		return parsed.Host
	}

	name := vpath.Base(trimmedPath)
	if name == "." || name == "/" || name == "" {
		name = parsed.Host
	}
	if parsed.RawQuery != "" {
		return name + "?" + parsed.RawQuery
	}
	return name
}

func canonicalWebURL(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}

	clone := *parsed
	clone.Scheme = strings.ToLower(clone.Scheme)
	clone.Host = strings.ToLower(clone.Host)
	clone.Fragment = ""
	clone.Path = strings.ReplaceAll(clone.Path, "\\", "/")
	if clone.Path == "/" {
		clone.Path = ""
	} else {
		clone.Path = strings.TrimRight(clone.Path, "/")
	}
	clone.RawPath = ""
	return clone.String()
}

func isLikelyWebPage(parsed *url.URL) bool {
	if parsed == nil {
		return false
	}

	if strings.HasSuffix(parsed.Path, "/") {
		return true
	}

	switch strings.ToLower(vpath.Ext(parsed.Path)) {
	case "", ".html", ".htm", ".xhtml", ".shtml", ".php", ".asp", ".aspx", ".jsp", ".jspx", ".cfm":
		return true
	default:
		return false
	}
}

func isHTMLResponse(contentType string, body []byte) bool {
	if isHTMLMediaType(normalizedMediaType(contentType)) {
		return true
	}
	return isHTMLMediaType(contentprobe.ProbeBytes(body).MimeType)
}

func isHTMLMediaType(contentType string) bool {
	switch normalizedMediaType(contentType) {
	case "text/html", "application/xhtml+xml":
		return true
	default:
		return false
	}
}

func isTextMediaType(contentType string) bool {
	contentType = normalizedMediaType(contentType)
	if contentType == "" {
		return false
	}
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	return strings.HasSuffix(contentType, "+json") || strings.HasSuffix(contentType, "+xml") ||
		contentType == "application/json" || contentType == "application/xml" ||
		contentType == "application/javascript" || contentType == "application/ecmascript"
}

func normalizedMediaType(contentType string) string {
	if contentType == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}

func parseHTTPModified(raw string) int64 {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	timestamp, err := time.Parse(http.TimeFormat, raw)
	if err != nil {
		return 0
	}
	return timestamp.Unix()
}

func responseSize(resp *http.Response, fallback int64) int64 {
	if resp == nil {
		return fallback
	}

	if contentRange := strings.TrimSpace(resp.Header.Get("Content-Range")); contentRange != "" {
		if slash := strings.LastIndex(contentRange, "/"); slash >= 0 && slash < len(contentRange)-1 {
			if size, err := strconv.ParseInt(contentRange[slash+1:], 10, 64); err == nil {
				return size
			}
		}
	}

	if resp.ContentLength > 0 {
		return resp.ContentLength
	}
	if headerLength := strings.TrimSpace(resp.Header.Get("Content-Length")); headerLength != "" {
		if size, err := strconv.ParseInt(headerLength, 10, 64); err == nil {
			return size
		}
	}
	return fallback
}

func sumFileSizes(entries []contracts.FileInfo) int64 {
	var total int64
	for _, entry := range entries {
		total += entry.Size
	}
	return total
}
