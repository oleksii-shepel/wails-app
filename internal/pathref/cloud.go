package pathref

import (
	"os"
	"path/filepath"
	"strings"
)

type LocalCloudProvider struct {
	Name     string
	Scheme   string
	RootPath string
}

type localCloudSpec struct {
	name       string
	scheme     string
	candidates func(home string) []string
}

var (
	userHomeDirFn = os.UserHomeDir
	pathExistsFn  = func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.IsDir()
	}
)

var localCloudSpecs = []localCloudSpec{
	{
		name:   "OneDrive",
		scheme: OneDriveScheme,
		candidates: func(home string) []string {
			return []string{
				filepath.Join(home, "OneDrive"),
				filepath.Join(home, "OneDrive - Personal"),
				filepath.Join(home, "Library", "CloudStorage", "OneDrive*"),
			}
		},
	},
	{
		name:   "Google Drive",
		scheme: GDriveScheme,
		candidates: func(home string) []string {
			return []string{
				filepath.Join(home, "Google Drive", "My Drive"),
				filepath.Join(home, "Google Drive"),
				filepath.Join(home, "My Drive"),
				filepath.Join(home, "Library", "CloudStorage", "GoogleDrive*"),
			}
		},
	},
	{
		name:   "Dropbox",
		scheme: DropboxScheme,
		candidates: func(home string) []string {
			return []string{
				filepath.Join(home, "Dropbox"),
				filepath.Join(home, "Library", "CloudStorage", "Dropbox*"),
			}
		},
	},
}

func AvailableLocalCloudProviders() []LocalCloudProvider {
	home, err := userHomeDirFn()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}

	providers := make([]LocalCloudProvider, 0, len(localCloudSpecs))
	for _, spec := range localCloudSpecs {
		root := resolveExistingCloudRoot(home, spec.candidates(home))
		if root == "" {
			continue
		}
		providers = append(providers, LocalCloudProvider{
			Name:     spec.name,
			Scheme:   spec.scheme,
			RootPath: root,
		})
	}

	return providers
}

func ResolveLocalCloudPath(path string) (string, bool) {
	normalized := Normalize(path)
	for _, provider := range AvailableLocalCloudProviders() {
		if !strings.HasPrefix(strings.ToLower(normalized), provider.Scheme) {
			continue
		}

		suffix := strings.TrimPrefix(normalized, provider.Scheme)
		if suffix == "" {
			return filepath.Clean(provider.RootPath), true
		}
		return filepath.Join(provider.RootPath, filepath.FromSlash(suffix)), true
	}
	return "", false
}

func LocalCloudAliasFromNativePath(path string) (string, bool) {
	native := filepath.ToSlash(filepath.Clean(path))
	for _, provider := range AvailableLocalCloudProviders() {
		root := filepath.ToSlash(filepath.Clean(provider.RootPath))
		if !pathHasPrefix(native, root) {
			continue
		}

		suffix := strings.TrimPrefix(native, root)
		suffix = strings.TrimPrefix(suffix, "/")
		if suffix == "" {
			return provider.Scheme, true
		}
		return provider.Scheme + suffix, true
	}
	return "", false
}

func IsLocalCloud(path string) bool {
	normalized := strings.ToLower(Normalize(path))
	return strings.HasPrefix(normalized, OneDriveScheme) ||
		strings.HasPrefix(normalized, GDriveScheme) ||
		strings.HasPrefix(normalized, DropboxScheme)
}

func resolveExistingCloudRoot(home string, candidates []string) string {
	for _, candidate := range candidates {
		for _, expanded := range expandCloudCandidate(candidate) {
			if !pathExistsFn(expanded) {
				continue
			}
			return filepath.Clean(expanded)
		}
	}
	return ""
}

func expandCloudCandidate(pattern string) []string {
	if strings.ContainsAny(pattern, "*?[") {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil
		}
		return matches
	}
	return []string{pattern}
}

func pathHasPrefix(path string, prefix string) bool {
	if filepath.Separator == '\\' {
		path = strings.ToLower(path)
		prefix = strings.ToLower(prefix)
	}

	if path == prefix {
		return true
	}

	return strings.HasPrefix(path, prefix+"/")
}
