package pathref

import (
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	RegistryScheme  = "registry://"
	SearchScheme    = "search://"
	MemoryScheme    = "memory://"
	SMBScheme       = "smb://"
	FTPScheme       = "ftp://"
	SFTPScheme      = "sftp://"
	WebDAVScheme    = "webdav://"
	registryVFSRoot = "/registry"
)

type aliasSpec struct {
	scheme  string
	vfsRoot string
}

type SearchQuery struct {
	BasePath  string `json:"basePath"`
	Pattern   string `json:"pattern"`
	Recursive bool   `json:"recursive"`
}

var aliasSpecs = []aliasSpec{
	{scheme: RegistryScheme, vfsRoot: registryVFSRoot},
	{scheme: SearchScheme},
	{scheme: MemoryScheme},
	{scheme: SMBScheme},
	{scheme: FTPScheme},
	{scheme: SFTPScheme},
	{scheme: WebDAVScheme},
}

var registryHivePrefixes = []string{
	"HKEY_LOCAL_MACHINE",
	"HKEY_CURRENT_USER",
	"HKEY_CLASSES_ROOT",
	"HKEY_USERS",
	"HKEY_CURRENT_CONFIG",
	"HKLM",
	"HKCU",
	"HKCR",
	"HKU",
	"HKCC",
}

func aliasSpecFor(path string) (aliasSpec, bool) {
	normalized := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	for _, spec := range aliasSpecs {
		if len(normalized) >= len(spec.scheme) && strings.EqualFold(normalized[:len(spec.scheme)], spec.scheme) {
			return spec, true
		}
	}
	return aliasSpec{}, false
}

func normalizeAliasPath(path string, spec aliasSpec) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	rest := trimmed[len(spec.scheme):]
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return spec.scheme
	}
	return spec.scheme + rest
}

func aliasRoot(path string) (string, bool) {
	if spec, ok := aliasSpecFor(path); ok {
		return spec.scheme, true
	}
	return "", false
}

func Normalize(path string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	if trimmed == "" {
		return ""
	}
	if spec, ok := aliasSpecFor(trimmed); ok {
		return normalizeAliasPath(trimmed, spec)
	}
	if !IsRegistry(trimmed) {
		return trimmed
	}
	return normalizeAliasPath(trimmed, aliasSpec{scheme: RegistryScheme, vfsRoot: registryVFSRoot})
}

func IsRegistry(path string) bool {
	spec, ok := aliasSpecFor(path)
	return ok && spec.scheme == RegistryScheme
}

func IsAlias(path string) bool {
	_, ok := aliasSpecFor(path)
	return ok
}

func IsSearch(path string) bool {
	spec, ok := aliasSpecFor(path)
	return ok && spec.scheme == SearchScheme
}

func IsMemory(path string) bool {
	spec, ok := aliasSpecFor(path)
	return ok && spec.scheme == MemoryScheme
}

func BuildSearch(basePath, pattern string, recursive bool) (string, error) {
	basePath = Normalize(basePath)
	if basePath == "" || pattern == "" {
		return "", nil
	}
	parts := []string{
		"path=" + readableSearchValue(basePath),
		"pattern=" + readableSearchValue(pattern),
		"recursive=" + strconv.FormatBool(recursive),
	}
	return SearchScheme + "?" + strings.Join(parts, "&"), nil
}

func readableSearchValue(value string) string {
	escaped := url.QueryEscape(value)
	replacer := strings.NewReplacer(
		"%2F", "/",
		"%3A", ":",
		"%2A", "*",
	)
	return replacer.Replace(escaped)
}

func ParseSearch(path string) (SearchQuery, bool) {
	normalized := Normalize(path)
	if !IsSearch(normalized) || normalized == SearchScheme {
		return SearchQuery{}, false
	}
	raw := strings.TrimPrefix(normalized, SearchScheme)
	if !strings.HasPrefix(raw, "?") {
		return SearchQuery{}, false
	}
	values, err := url.ParseQuery(strings.TrimPrefix(raw, "?"))
	if err != nil {
		return SearchQuery{}, false
	}
	query := SearchQuery{
		BasePath:  Normalize(values.Get("path")),
		Pattern:   values.Get("pattern"),
		Recursive: strings.EqualFold(values.Get("recursive"), "true") || values.Get("recursive") == "1",
	}
	query.BasePath = Normalize(query.BasePath)
	if query.BasePath == "" || query.Pattern == "" {
		return SearchQuery{}, false
	}
	return query, true
}

func NormalizeSearchPattern(pattern string) string {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return ""
	}
	if strings.ContainsAny(trimmed, "*?[") {
		return trimmed
	}
	return "*" + trimmed + "*"
}

func MatchSearchPattern(pattern, name string) (bool, error) {
	normalizedPattern := NormalizeSearchPattern(pattern)
	if normalizedPattern == "" {
		return false, nil
	}
	return filepath.Match(strings.ToLower(normalizedPattern), strings.ToLower(name))
}

func ToVFS(path string) string {
	normalized := Normalize(path)
	spec, ok := aliasSpecFor(normalized)
	if !ok || spec.vfsRoot == "" {
		return normalized
	}
	rest := strings.TrimPrefix(normalized, spec.scheme)
	if rest == "" {
		return spec.vfsRoot
	}
	return spec.vfsRoot + "/" + rest
}

func FromVFS(path string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	for _, spec := range aliasSpecs {
		if spec.vfsRoot == "" {
			continue
		}
		if normalized == spec.vfsRoot {
			return spec.scheme
		}
		if strings.HasPrefix(normalized, spec.vfsRoot+"/") {
			return spec.scheme + strings.TrimPrefix(normalized, spec.vfsRoot+"/")
		}
	}
	return normalized
}

func FromVFSEntry(basePublicPath, entryPath string) string {
	basePublicPath = Normalize(basePublicPath)
	entryPath = strings.TrimSpace(strings.ReplaceAll(entryPath, "\\", "/"))
	if !IsAlias(basePublicPath) {
		return FromVFS(entryPath)
	}
	if IsAlias(entryPath) || strings.HasPrefix(entryPath, registryVFSRoot) {
		return FromVFS(entryPath)
	}
	if !IsRegistry(basePublicPath) {
		if root, ok := aliasRoot(basePublicPath); ok {
			if basePublicPath == root {
				return Normalize(root + strings.TrimPrefix(entryPath, "/"))
			}
			trimmedBase := strings.TrimSuffix(basePublicPath, "/")
			return Normalize(trimmedBase + "/" + strings.TrimPrefix(entryPath, "/"))
		}
		return FromVFS(entryPath)
	}
	for _, prefix := range registryHivePrefixes {
		if strings.EqualFold(entryPath, prefix) || strings.HasPrefix(strings.ToUpper(entryPath), prefix+"/") {
			return Normalize(RegistryScheme + entryPath)
		}
	}
	if basePublicPath == RegistryScheme {
		return Normalize(RegistryScheme + entryPath)
	}
	trimmedBase := strings.TrimSuffix(basePublicPath, "/")
	return Normalize(trimmedBase + "/" + strings.TrimPrefix(entryPath, "/"))
}

func Parent(path string) (string, bool) {
	normalized := Normalize(path)
	root, ok := aliasRoot(normalized)
	if !ok {
		return "", false
	}
	rest := strings.TrimPrefix(normalized, root)
	if rest == "" {
		return "", false
	}
	idx := strings.LastIndex(rest, "/")
	if idx < 0 {
		return root, true
	}
	return root + rest[:idx], true
}
