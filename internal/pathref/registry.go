package pathref

import "strings"

const (
	RegistryScheme  = "registry://"
	registryVFSRoot = "/registry"
)

func Normalize(path string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	if trimmed == "" {
		return ""
	}
	if !IsRegistry(trimmed) {
		return trimmed
	}
	rest := strings.TrimPrefix(trimmed, RegistryScheme)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return RegistryScheme
	}
	return RegistryScheme + rest
}

func IsRegistry(path string) bool {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	if len(path) < len(RegistryScheme) {
		return false
	}
	return strings.EqualFold(path[:len(RegistryScheme)], RegistryScheme)
}

func ToVFS(path string) string {
	normalized := Normalize(path)
	if !IsRegistry(normalized) {
		return normalized
	}
	rest := strings.TrimPrefix(normalized, RegistryScheme)
	if rest == "" {
		return registryVFSRoot
	}
	return registryVFSRoot + "/" + rest
}

func FromVFS(path string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	if normalized == registryVFSRoot {
		return RegistryScheme
	}
	if strings.HasPrefix(normalized, registryVFSRoot+"/") {
		return RegistryScheme + strings.TrimPrefix(normalized, registryVFSRoot+"/")
	}
	return normalized
}

func Parent(path string) (string, bool) {
	normalized := Normalize(path)
	if !IsRegistry(normalized) {
		return "", false
	}
	rest := strings.TrimPrefix(normalized, RegistryScheme)
	if rest == "" {
		return "", false
	}
	idx := strings.LastIndex(rest, "/")
	if idx < 0 {
		return RegistryScheme, true
	}
	return RegistryScheme + rest[:idx], true
}
