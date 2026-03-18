package pathref

import "strings"

const (
	RegistryScheme  = "registry://"
	registryVFSRoot = "/registry"
)

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

func FromVFSEntry(basePublicPath, entryPath string) string {
	basePublicPath = Normalize(basePublicPath)
	entryPath = strings.TrimSpace(strings.ReplaceAll(entryPath, "\\", "/"))
	if !IsRegistry(basePublicPath) {
		return FromVFS(entryPath)
	}
	if IsRegistry(entryPath) || strings.HasPrefix(entryPath, registryVFSRoot) {
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
