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
	"time"

	"golang.org/x/net/html"

	"wails-app/internal/contentprobe"
	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
)

const (
	webAliasTimeout   = 30 * time.Second
	webAliasReadLimit = 4 * 1024 * 1024
)

type webAliasService struct {
	client *http.Client
}

func newWebAliasService() *webAliasService {
	return &webAliasService{
		client: &http.Client{Timeout: webAliasTimeout},
	}
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

	headResp, headErr := s.request(http.MethodHead, targetURL, nil)
	if headResp != nil {
		defer headResp.Body.Close()
	}

	var (
		contentType string
		size        int64
		modifiedAt  int64
	)

	if headErr == nil && headResp != nil && headResp.StatusCode >= http.StatusOK && headResp.StatusCode < http.StatusBadRequest {
		contentType = normalizedMediaType(headResp.Header.Get("Content-Type"))
		size = responseSize(headResp, 0)
		modifiedAt = parseHTTPModified(headResp.Header.Get("Last-Modified"))
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
	} else if isLikelyWebPage(parsed) {
		fileType = contracts.FileTypeDirectory
	}

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

	return &contracts.FileContent{
		Path:     requestedPath,
		Content:  content,
		Encoding: encoding,
		Size:     responseSize(resp, int64(len(data))),
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

	return &contracts.FileContent{
		Path:     requestedPath,
		Content:  base64.StdEncoding.EncodeToString(data),
		Encoding: "base64",
		Size:     responseSize(resp, offset+int64(len(data))),
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

	resolvedBase := findHTMLBaseURL(doc, baseURL)
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
			case "form":
				addWebEntry(entries, resolvedBase, currentURL, attrValue(node, "action"), true)
			case "img":
				addWebEntry(entries, resolvedBase, currentURL, attrValue(node, "src"), false)
				for _, candidate := range parseSrcset(attrValue(node, "srcset")) {
					addWebEntry(entries, resolvedBase, currentURL, candidate, false)
				}
			case "script":
				addWebEntry(entries, resolvedBase, currentURL, attrValue(node, "src"), false)
			case "link":
				addWebEntry(entries, resolvedBase, currentURL, attrValue(node, "href"), false)
			case "source":
				addWebEntry(entries, resolvedBase, currentURL, attrValue(node, "src"), false)
				for _, candidate := range parseSrcset(attrValue(node, "srcset")) {
					addWebEntry(entries, resolvedBase, currentURL, candidate, false)
				}
			case "video", "audio":
				addWebEntry(entries, resolvedBase, currentURL, attrValue(node, "src"), false)
				addWebEntry(entries, resolvedBase, currentURL, attrValue(node, "poster"), false)
			case "iframe", "embed":
				addWebEntry(entries, resolvedBase, currentURL, attrValue(node, "src"), true)
			case "object":
				addWebEntry(entries, resolvedBase, currentURL, attrValue(node, "data"), false)
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

func addWebEntry(entries map[string]contracts.FileInfo, baseURL *url.URL, currentURL string, rawRef string, navigable bool) {
	resolved, ok := resolveWebReference(baseURL, rawRef)
	if !ok {
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
