//go:build windows

// Package registry exposes the Windows Registry as a vfs.FileSystem.
//
// # Mapping
//
// The registry tree maps naturally onto the VFS directory model:
//
//	Registry concept          VFS concept
//	────────────────────────  ─────────────────────────────────────
//	Hive (HKLM, HKCU, …)     top-level directory
//	Key                       directory
//	Named value               file
//	Default value ("")        file named "(default)"
//
// ReadDir on a key returns both its subkeys (as dirs) and its values (as files)
// in a single listing, matching how a real file manager would show a key.
//
// # Value serialisation
//
// Values are serialised to UTF-8 text so they can be read through the File
// interface.  The format mirrors .reg file conventions:
//
//	REG_SZ / REG_EXPAND_SZ   the string, no decoration
//	REG_MULTI_SZ              one string per line
//	REG_DWORD / REG_QWORD     decimal (hex in parentheses): 4294967295 (0xFFFFFFFF)
//	REG_BINARY                upper-case hex pairs: DE AD BE EF …
//	REG_NONE / unknown        "(empty)" or hex dump
//
// # Write support
//
// Create/Mkdir/Remove/Rename delegate to the appropriate registry API calls.
// Most write operations require elevation; the OS returns access-denied
// naturally and the error is propagated as-is — no special handling needed.
//
// Rename is not supported by the registry API and always returns ErrReadOnly.
//
// # Build constraints
//
// This file only compiles on Windows.  A companion registry_stub.go provides
// a no-op New() for other platforms so the rest of the codebase compiles
// everywhere without build tags at the call site.
package registry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"

	"wails-app/vfs"
)

// ── Hive table ────────────────────────────────────────────────────────────────

type hiveEntry struct {
	name string
	key  registry.Key
}

var hives = []hiveEntry{
	{"HKEY_LOCAL_MACHINE", registry.LOCAL_MACHINE},
	{"HKEY_CURRENT_USER", registry.CURRENT_USER},
	{"HKEY_CLASSES_ROOT", registry.CLASSES_ROOT},
	{"HKEY_USERS", registry.USERS},
	{"HKEY_CURRENT_CONFIG", registry.CURRENT_CONFIG},
	// Short aliases — resolved in openKey.
	{"HKLM", registry.LOCAL_MACHINE},
	{"HKCU", registry.CURRENT_USER},
	{"HKCR", registry.CLASSES_ROOT},
	{"HKU", registry.USERS},
	{"HKCC", registry.CURRENT_CONFIG},
}

// canonical returns the canonical long name for a hive abbreviation.
var canonical = map[string]string{
	"HKLM": "HKEY_LOCAL_MACHINE",
	"HKCU": "HKEY_CURRENT_USER",
	"HKCR": "HKEY_CLASSES_ROOT",
	"HKU":  "HKEY_USERS",
	"HKCC": "HKEY_CURRENT_CONFIG",
}

// ── FS ────────────────────────────────────────────────────────────────────────

// FS implements vfs.FileSystem over the Windows Registry.
type FS struct{}

// New returns an FS that exposes the Windows Registry.
// The returned error is always nil on Windows; the companion stub returns
// ErrNotSupported on other platforms.
func New() (*FS, error) {
	return &FS{}, nil
}

// ── path helpers ──────────────────────────────────────────────────────────────

// splitPath breaks a VFS path into (hiveName, subkeyPath, valueName).
//
// Rules:
//   - path ""  or "/"        → root listing (no hive)
//   - path "HKLM"            → hive root, no subkey, no value
//   - path "HKLM/SOFTWARE"   → hive=HKLM, subkey="SOFTWARE", no value
//
// Whether the last component is a subkey or a value name is determined by
// actually opening it at call time (Stat/Open), not here.
func splitPath(path string) (hive, subkey string) {
	path = strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	if path == "" {
		return "", ""
	}
	parts := strings.SplitN(path, "/", 2)
	hive = strings.ToUpper(parts[0])
	if c, ok := canonical[hive]; ok {
		hive = c
	}
	if len(parts) == 2 {
		subkey = parts[1]
	}
	return hive, subkey
}

// rootKey returns the registry.Key constant for the named hive.
func rootKey(hiveName string) (registry.Key, bool) {
	upper := strings.ToUpper(hiveName)
	if c, ok := canonical[upper]; ok {
		upper = c
	}
	for _, h := range hives {
		if h.name == upper {
			return h.key, true
		}
	}
	return 0, false
}

// openKey opens the registry key for path with the requested access mask.
// It returns the open key and the value name component (empty if path is a key).
// Caller must close the returned key when done (unless it is a hive root).
func openKey(path string, access uint32) (key registry.Key, valueName string, isHiveRoot bool, err error) {
	hiveName, subkey := splitPath(path)
	if hiveName == "" {
		return 0, "", false, vfs.ErrNotFound
	}

	root, ok := rootKey(hiveName)
	if !ok {
		return 0, "", false, vfs.ErrNotFound
	}

	if subkey == "" {
		// Path is the hive root itself — not a real openable key, but valid.
		return root, "", true, nil
	}

	// Try opening subkey directly first.
	k, err := registry.OpenKey(root, strings.ReplaceAll(subkey, "/", "\\"), access)
	if err == nil {
		return k, "", false, nil
	}

	// If that failed, maybe the last component is a value name.
	idx := strings.LastIndex(subkey, "/")
	if idx < 0 {
		// e.g. "HKLM/SomeName" where SomeName is a value of the hive root.
		// Try treating the whole subkey as a value of the hive root.
		return root, subkey, true, nil
	}

	parentSubkey := subkey[:idx]
	valName := subkey[idx+1:]
	pk, err2 := registry.OpenKey(root, strings.ReplaceAll(parentSubkey, "/", "\\"), access)
	if err2 != nil {
		return 0, "", false, vfs.ErrNotFound
	}
	return pk, valName, false, nil
}

// ── FileSystem interface ───────────────────────────────────────────────────────

func (f *FS) Open(_ context.Context, path string) (vfs.File, error) {
	k, valName, isHiveRoot, err := openKey(path, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	if !isHiveRoot {
		defer k.Close()
	}

	if valName == "" {
		// Path points to a key, not a value — not a readable file.
		return nil, vfs.ErrNotFile
	}

	data, err := readValue(k, valName)
	if err != nil {
		return nil, err
	}

	info := &valueInfo{
		path:    path,
		name:    valName,
		size:    int64(len(data)),
		modTime: time.Time{},
	}
	return &regFile{reader: bytes.NewReader(data), info: info}, nil
}

func (f *FS) Stat(_ context.Context, path string) (vfs.FileInfo, error) {
	path = strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")

	// Root.
	if path == "" {
		return &keyInfo{path: "", name: "registry", isRoot: true}, nil
	}

	hiveName, subkey := splitPath(path)
	if hiveName == "" {
		return nil, vfs.ErrNotFound
	}

	// Hive root.
	if subkey == "" {
		root, ok := rootKey(hiveName)
		if !ok {
			return nil, vfs.ErrNotFound
		}
		_ = root
		return &keyInfo{path: path, name: hiveName}, nil
	}

	root, ok := rootKey(hiveName)
	if !ok {
		return nil, vfs.ErrNotFound
	}

	// Try as a key.
	k, err := registry.OpenKey(root, strings.ReplaceAll(subkey, "/", "\\"), registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		return &keyInfo{path: path, name: lastName(path)}, nil
	}

	// Try as a value.
	idx := strings.LastIndex(subkey, "/")
	var parentKey registry.Key
	var valName string
	if idx < 0 {
		parentKey = root
		valName = subkey
	} else {
		pk, err2 := registry.OpenKey(root, strings.ReplaceAll(subkey[:idx], "/", "\\"), registry.QUERY_VALUE)
		if err2 != nil {
			return nil, vfs.ErrNotFound
		}
		defer pk.Close()
		parentKey = pk
		valName = subkey[idx+1:]
	}

	data, err := readValue(parentKey, valName)
	if err != nil {
		return nil, vfs.ErrNotFound
	}
	return &valueInfo{path: path, name: valName, size: int64(len(data))}, nil
}

func (f *FS) ReadDir(_ context.Context, path string) ([]vfs.FileInfo, error) {
	path = strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")

	// Root — list hives.
	if path == "" {
		result := make([]vfs.FileInfo, 0, 5)
		seen := map[string]bool{}
		for _, h := range hives {
			if seen[h.name] || len(h.name) <= 4 { // skip short aliases
				continue
			}
			seen[h.name] = true
			result = append(result, &keyInfo{path: h.name, name: h.name})
		}
		return result, nil
	}

	hiveName, subkey := splitPath(path)
	root, ok := rootKey(hiveName)
	if !ok {
		return nil, vfs.ErrNotFound
	}

	var k registry.Key
	if subkey == "" {
		k = root
	} else {
		var err error
		k, err = registry.OpenKey(root, strings.ReplaceAll(subkey, "/", "\\"),
			registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
		if err != nil {
			return nil, vfs.ErrNotFound
		}
		defer k.Close()
	}

	var result []vfs.FileInfo

	// Subkeys → directories.
	subkeyNames, err := k.ReadSubKeyNames(-1)
	if err == nil {
		for _, name := range subkeyNames {
			childPath := path + "/" + name
			result = append(result, &keyInfo{path: childPath, name: name})
		}
	}

	// Values → files.
	valueNames, err := k.ReadValueNames(-1)
	if err == nil {
		for _, name := range valueNames {
			displayName := name
			if name == "" {
				displayName = "(default)"
			}
			data, _ := readValue(k, name)
			childPath := path + "/" + displayName
			result = append(result, &valueInfo{
				path: childPath,
				name: displayName,
				size: int64(len(data)),
			})
		}
	}

	return result, nil
}

func (f *FS) Exists(ctx context.Context, path string) bool {
	info, err := f.Stat(ctx, path)
	return err == nil && info != nil
}

// ── Write operations ──────────────────────────────────────────────────────────

// Create creates a new REG_SZ value at path.  The caller writes the string
// value through the returned File; on Close the value is written.
func (f *FS) Create(_ context.Context, path string) (vfs.File, error) {
	k, valName, _, err := openKey(path, registry.SET_VALUE)
	if err != nil {
		return nil, err
	}
	if valName == "" {
		k.Close()
		return nil, vfs.ErrNotFile
	}
	return &writeFile{key: k, valName: valName}, nil
}

// Mkdir creates a new registry key at path.
func (f *FS) Mkdir(_ context.Context, path string, _ fs.FileMode) error {
	hiveName, subkey := splitPath(path)
	root, ok := rootKey(hiveName)
	if !ok {
		return vfs.ErrNotFound
	}
	if subkey == "" {
		return vfs.ErrExists
	}
	k, _, err := registry.CreateKey(root, strings.ReplaceAll(subkey, "/", "\\"), registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("registry mkdir %q: %w", path, err)
	}
	k.Close()
	return nil
}

// Remove deletes a registry key or value at path.
func (f *FS) Remove(_ context.Context, path string) error {
	hiveName, subkey := splitPath(path)
	root, ok := rootKey(hiveName)
	if !ok {
		return vfs.ErrNotFound
	}
	if subkey == "" {
		return fmt.Errorf("registry remove: cannot delete hive root")
	}

	// Try as a key first.
	err := registry.DeleteKey(root, strings.ReplaceAll(subkey, "/", "\\"))
	if err == nil {
		return nil
	}

	// Try as a value.
	idx := strings.LastIndex(subkey, "/")
	var parentKey registry.Key
	var valName string
	if idx < 0 {
		parentKey = root
		valName = subkey
	} else {
		pk, err2 := registry.OpenKey(root, strings.ReplaceAll(subkey[:idx], "/", "\\"), registry.SET_VALUE)
		if err2 != nil {
			return vfs.ErrNotFound
		}
		defer pk.Close()
		parentKey = pk
		valName = subkey[idx+1:]
	}
	if err2 := parentKey.DeleteValue(valName); err2 != nil {
		return fmt.Errorf("registry remove value %q: %w", valName, err2)
	}
	return nil
}

// Rename is not supported by the registry API.
func (f *FS) Rename(_ context.Context, _, _ string) error {
	return fmt.Errorf("registry: rename not supported")
}

// ── Path helpers ──────────────────────────────────────────────────────────────

func (f *FS) Join(elem ...string) string        { return strings.Join(elem, "/") }
func (f *FS) IsAbs(path string) bool            { return false } // registry paths are always relative to root
func (f *FS) RealPath(_ string) (string, error) { return "", nil }

func (f *FS) Split(path string) (string, string) {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "", path
	}
	return path[:i], path[i+1:]
}

// ── Value serialisation ───────────────────────────────────────────────────────

// readValue reads a registry value and serialises it to UTF-8 bytes.
func readValue(k registry.Key, name string) ([]byte, error) {
	// Try as string (REG_SZ, REG_EXPAND_SZ).
	if s, _, err := k.GetStringValue(name); err == nil {
		return []byte(s), nil
	}

	// Try as integer (REG_DWORD, REG_QWORD).
	if n, _, err := k.GetIntegerValue(name); err == nil {
		return []byte(fmt.Sprintf("%d (0x%X)", n, n)), nil
	}

	// Try as multi-string (REG_MULTI_SZ).
	if ss, _, err := k.GetStringsValue(name); err == nil {
		return []byte(strings.Join(ss, "\n")), nil
	}

	// Fall back to binary (REG_BINARY, REG_NONE, unknown).
	b, _, err := k.GetBinaryValue(name)
	if err != nil {
		return nil, fmt.Errorf("registry: read value %q: %w", name, err)
	}
	if len(b) == 0 {
		return []byte("(empty)"), nil
	}
	var sb strings.Builder
	for i, byt := range b {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "%02X", byt)
	}
	return []byte(sb.String()), nil
}

func lastName(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return path
	}
	return path[i+1:]
}

// ── keyInfo ───────────────────────────────────────────────────────────────────

type keyInfo struct {
	path   string
	name   string
	isRoot bool
}

func (k *keyInfo) Name() string       { return k.name }
func (k *keyInfo) Size() int64        { return 0 }
func (k *keyInfo) Mode() fs.FileMode  { return fs.ModeDir | 0555 }
func (k *keyInfo) ModTime() time.Time { return time.Time{} }
func (k *keyInfo) IsDir() bool        { return true }
func (k *keyInfo) Sys() any           { return nil }

func (k *keyInfo) Path() string         { return k.path }
func (k *keyInfo) RealPath() string     { return "" }
func (k *keyInfo) Volume() vfs.Volume   { return nil }
func (k *keyInfo) IsArchive() bool      { return false }
func (k *keyInfo) IsHidden() bool       { return false }
func (k *keyInfo) IsSymlink() bool      { return false }
func (k *keyInfo) IsNested() bool       { return false }
func (k *keyInfo) MountChain() []string { return nil }

// ── valueInfo ─────────────────────────────────────────────────────────────────

type valueInfo struct {
	path    string
	name    string
	size    int64
	modTime time.Time
}

func (v *valueInfo) Name() string       { return v.name }
func (v *valueInfo) Size() int64        { return v.size }
func (v *valueInfo) Mode() fs.FileMode  { return 0444 }
func (v *valueInfo) ModTime() time.Time { return v.modTime }
func (v *valueInfo) IsDir() bool        { return false }
func (v *valueInfo) Sys() any           { return nil }

func (v *valueInfo) Path() string         { return v.path }
func (v *valueInfo) RealPath() string     { return "" }
func (v *valueInfo) Volume() vfs.Volume   { return nil }
func (v *valueInfo) IsArchive() bool      { return false }
func (v *valueInfo) IsHidden() bool       { return false }
func (v *valueInfo) IsSymlink() bool      { return false }
func (v *valueInfo) IsNested() bool       { return false }
func (v *valueInfo) MountChain() []string { return nil }

// ── regFile (read) ────────────────────────────────────────────────────────────

type regFile struct {
	reader *bytes.Reader
	info   *valueInfo
}

func (f *regFile) Read(p []byte) (int, error)  { return f.reader.Read(p) }
func (f *regFile) Write(_ []byte) (int, error) { return 0, vfs.ErrReadOnly }
func (f *regFile) Seek(offset int64, whence int) (int64, error) {
	return f.reader.Seek(offset, whence)
}
func (f *regFile) Close() error                { return nil }
func (f *regFile) Stat() (vfs.FileInfo, error) { return f.info, nil }
func (f *regFile) Sync() error                 { return nil }

// ── writeFile (write) ─────────────────────────────────────────────────────────

// writeFile buffers written bytes and commits them as REG_SZ on Close.
type writeFile struct {
	key     registry.Key
	valName string
	buf     bytes.Buffer
}

func (w *writeFile) Read(_ []byte) (int, error) { return 0, io.EOF }
func (w *writeFile) Seek(_ int64, _ int) (int64, error) {
	return 0, fmt.Errorf("registry write file: seek not supported")
}
func (w *writeFile) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *writeFile) Sync() error                 { return nil }
func (w *writeFile) Stat() (vfs.FileInfo, error) {
	return &valueInfo{name: w.valName, size: int64(w.buf.Len())}, nil
}
func (w *writeFile) Close() error {
	defer w.key.Close()
	return w.key.SetStringValue(w.valName, w.buf.String())
}

// Compile-time interface check.
var _ vfs.FileSystem = (*FS)(nil)
