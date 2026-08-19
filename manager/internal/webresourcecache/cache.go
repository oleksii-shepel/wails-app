package webresourcecache

import (
	"net/url"
	"sort"
	"strings"
	"sync"
)

type Resource struct {
	URL           string
	Size          int64
	InitiatorType string
	MimeType      string
}

var (
	mu            sync.RWMutex
	pageResources = make(map[string]map[string]Resource)
)

func Store(pageURL string, resources []Resource) bool {
	normalizedPage := normalizeURL(pageURL)
	if normalizedPage == "" {
		return false
	}

	mu.RLock()
	existing := cloneResources(pageResources[normalizedPage])
	mu.RUnlock()

	next := make(map[string]Resource)
	for _, resource := range resources {
		normalizedResource := normalizeURL(resource.URL)
		if normalizedResource == "" || normalizedResource == normalizedPage {
			continue
		}
		current := existing[normalizedResource]
		size := resource.Size
		if size <= 0 {
			size = current.Size
		}
		mimeType := strings.TrimSpace(resource.MimeType)
		if mimeType == "" {
			mimeType = current.MimeType
		}
		next[normalizedResource] = Resource{
			URL:           normalizedResource,
			Size:          size,
			InitiatorType: strings.TrimSpace(resource.InitiatorType),
			MimeType:      mimeType,
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if resourcesEqual(pageResources[normalizedPage], next) {
		return false
	}
	pageResources[normalizedPage] = next
	return true
}

func Merge(pageURL string, resource Resource) bool {
	normalizedPage := normalizeURL(pageURL)
	normalizedResource := normalizeURL(resource.URL)
	if normalizedPage == "" || normalizedResource == "" || normalizedPage == normalizedResource {
		return false
	}

	mu.Lock()
	defer mu.Unlock()
	if pageResources[normalizedPage] == nil {
		pageResources[normalizedPage] = make(map[string]Resource)
	}
	current := pageResources[normalizedPage][normalizedResource]
	next := current
	next.URL = normalizedResource
	if resource.Size > 0 {
		next.Size = resource.Size
	}
	if strings.TrimSpace(resource.InitiatorType) != "" {
		next.InitiatorType = strings.TrimSpace(resource.InitiatorType)
	}
	if strings.TrimSpace(resource.MimeType) != "" {
		next.MimeType = strings.TrimSpace(resource.MimeType)
	}
	if current == next {
		return false
	}
	pageResources[normalizedPage][normalizedResource] = next
	return true
}

func Clear(pageURL string) {
	normalizedPage := normalizeURL(pageURL)
	if normalizedPage == "" {
		return
	}
	mu.Lock()
	delete(pageResources, normalizedPage)
	mu.Unlock()
}

func Load(pageURL string) []Resource {
	normalizedPage := normalizeURL(pageURL)
	if normalizedPage == "" {
		return nil
	}

	mu.RLock()
	stored := pageResources[normalizedPage]
	mu.RUnlock()
	if len(stored) == 0 {
		return nil
	}

	keys := make([]string, 0, len(stored))
	for key := range stored {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]Resource, 0, len(keys))
	for _, key := range keys {
		result = append(result, stored[key])
	}
	return result
}

func cloneResources(input map[string]Resource) map[string]Resource {
	if len(input) == 0 {
		return map[string]Resource{}
	}
	result := make(map[string]Resource, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func normalizeURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	parsed.Scheme = scheme
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	parsed.Path = strings.ReplaceAll(parsed.Path, "\\", "/")
	if parsed.Path == "/" {
		parsed.Path = ""
	} else {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}
	parsed.RawPath = ""
	return parsed.String()
}

func resourcesEqual(left, right map[string]Resource) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		rightValue, ok := right[key]
		if !ok {
			return false
		}
		if leftValue.Size != rightValue.Size || leftValue.InitiatorType != rightValue.InitiatorType || leftValue.MimeType != rightValue.MimeType {
			return false
		}
	}
	return true
}