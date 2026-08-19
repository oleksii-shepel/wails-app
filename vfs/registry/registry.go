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
// Rename supports sibling key and value renames while preserving value types.
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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
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

var (
	advapi32          = windows.NewLazySystemDLL("advapi32.dll")
	regRenameKeyProc  = advapi32.NewProc("RegRenameKey")
	regSetValueExProc = advapi32.NewProc("RegSetValueExW")
)

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

func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if os.IsPermission(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "eacces") ||
		strings.Contains(msg, "eperm")
}

func mapLookupError(err error) error {
	if err == nil {
		return nil
	}
	if isPermissionError(err) {
		return fs.ErrPermission
	}
	return vfs.ErrNotFound
}

func mapAccessError(err error) error {
	if err == nil {
		return nil
	}
	if isPermissionError(err) {
		return fs.ErrPermission
	}
	return err
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
	if isPermissionError(err) {
		return 0, "", false, fs.ErrPermission
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
		return 0, "", false, mapLookupError(err2)
	}
	return pk, valName, false, nil
}

func apiValueName(name string) string {
	if strings.EqualFold(strings.TrimSpace(name), "(default)") {
		return ""
	}
	return name
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

	value, err := readRegistryValue(k, valName)
	if err != nil {
		return nil, err
	}

	info := &valueInfo{
		path:      path,
		name:      valName,
		size:      int64(len(value.data)),
		modTime:   time.Time{},
		valueType: value.valueType,
		preview:   value.preview,
	}
	return &regFile{reader: bytes.NewReader(value.data), info: info}, nil
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
	if isPermissionError(err) {
		return nil, fs.ErrPermission
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
			return nil, mapLookupError(err2)
		}
		defer pk.Close()
		parentKey = pk
		valName = subkey[idx+1:]
	}

	value, err := readRegistryValue(parentKey, valName)
	if err != nil {
		return nil, mapLookupError(err)
	}
	return &valueInfo{
		path:      path,
		name:      valName,
		size:      int64(len(value.data)),
		valueType: value.valueType,
		preview:   value.preview,
	}, nil
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
			return nil, mapLookupError(err)
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
	} else if isPermissionError(err) {
		return nil, fs.ErrPermission
	}

	// Values → files.
	valueNames, err := k.ReadValueNames(-1)
	if err == nil {
		for _, name := range valueNames {
			displayName := name
			if name == "" {
				displayName = "(default)"
			}
			value, _ := readRegistryValue(k, name)
			childPath := path + "/" + displayName
			result = append(result, &valueInfo{
				path:      childPath,
				name:      displayName,
				size:      int64(len(value.data)),
				valueType: value.valueType,
				preview:   value.preview,
			})
		}
	} else if isPermissionError(err) {
		return nil, fs.ErrPermission
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
		return fmt.Errorf("registry mkdir %q: %w", path, mapAccessError(err))
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
			return mapLookupError(err2)
		}
		defer pk.Close()
		parentKey = pk
		valName = subkey[idx+1:]
	}
	if err2 := parentKey.DeleteValue(apiValueName(valName)); err2 != nil {
		return fmt.Errorf("registry remove value %q: %w", valName, mapAccessError(err2))
	}
	return nil
}

// Rename renames a registry key or value within its current parent.
func (f *FS) Rename(_ context.Context, oldPath, newPath string) error {
	oldHive, oldSubkey := splitPath(oldPath)
	newHive, newSubkey := splitPath(newPath)
	if oldHive == "" || newHive == "" {
		return vfs.ErrNotFound
	}
	if !strings.EqualFold(oldHive, newHive) {
		return fmt.Errorf("registry rename: source and destination hives differ")
	}
	if oldSubkey == "" || newSubkey == "" {
		return fmt.Errorf("registry rename: hive roots cannot be renamed")
	}

	oldParent, oldName := splitParentPath(oldSubkey)
	newParent, newName := splitParentPath(newSubkey)
	if !strings.EqualFold(oldParent, newParent) {
		return fmt.Errorf("registry rename: moving between parent keys is not supported")
	}
	if strings.TrimSpace(newName) == "" {
		return fmt.Errorf("registry rename: destination name is empty")
	}

	root, ok := rootKey(oldHive)
	if !ok {
		return vfs.ErrNotFound
	}

	if key, err := registry.OpenKey(root, strings.ReplaceAll(oldSubkey, "/", "\\"), registry.READ); err == nil {
		key.Close()
		return renameRegistryKey(root, oldParent, oldName, newName)
	} else if isPermissionError(err) {
		return fs.ErrPermission
	}

	return renameRegistryValue(root, oldParent, oldName, newName)
}

func splitParentPath(path string) (parent, name string) {
	path = strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "", path
	}
	return path[:i], path[i+1:]
}

func renameRegistryKey(root registry.Key, parentPath, oldName, newName string) error {
	parent := root
	if parentPath != "" {
		k, err := registry.OpenKey(root, strings.ReplaceAll(parentPath, "/", "\\"), registry.ALL_ACCESS)
		if err != nil {
			return mapAccessError(err)
		}
		defer k.Close()
		parent = k
	}

	oldPtr, err := windows.UTF16PtrFromString(oldName)
	if err != nil {
		return err
	}
	newPtr, err := windows.UTF16PtrFromString(newName)
	if err != nil {
		return err
	}
	result, _, _ := regRenameKeyProc.Call(
		uintptr(parent),
		uintptr(unsafe.Pointer(oldPtr)),
		uintptr(unsafe.Pointer(newPtr)),
	)
	if result != 0 {
		return mapAccessError(syscall.Errno(result))
	}
	return nil
}

func renameRegistryValue(root registry.Key, parentPath, oldName, newName string) error {
	parent := root
	if parentPath != "" {
		k, err := registry.OpenKey(
			root,
			strings.ReplaceAll(parentPath, "/", "\\"),
			registry.QUERY_VALUE|registry.SET_VALUE,
		)
		if err != nil {
			return mapAccessError(err)
		}
		defer k.Close()
		parent = k
	}

	oldAPIName := apiValueName(oldName)
	newAPIName := apiValueName(newName)
	if _, _, err := parent.GetValue(newAPIName, nil); err == nil {
		return fs.ErrExist
	} else if !errors.Is(err, registry.ErrNotExist) && !errors.Is(err, registry.ErrShortBuffer) {
		return mapAccessError(err)
	}

	data, valueType, err := rawRegistryValue(parent, oldAPIName)
	if err != nil {
		return mapLookupError(err)
	}
	if err := setRawRegistryValue(parent, newAPIName, valueType, data); err != nil {
		return mapAccessError(err)
	}
	if err := parent.DeleteValue(oldAPIName); err != nil {
		_ = parent.DeleteValue(newAPIName)
		return mapAccessError(err)
	}
	return nil
}

func rawRegistryValue(k registry.Key, name string) ([]byte, uint32, error) {
	size, valueType, err := k.GetValue(name, nil)
	if err != nil && !errors.Is(err, registry.ErrShortBuffer) {
		return nil, valueType, err
	}
	if size == 0 {
		return nil, valueType, nil
	}
	buf := make([]byte, size)
	n, valueType, err := k.GetValue(name, buf)
	if err != nil {
		return nil, valueType, err
	}
	return buf[:n], valueType, nil
}

func setRawRegistryValue(k registry.Key, name string, valueType uint32, data []byte) error {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	var dataPtr uintptr
	if len(data) > 0 {
		dataPtr = uintptr(unsafe.Pointer(&data[0]))
	}
	result, _, _ := regSetValueExProc.Call(
		uintptr(k),
		uintptr(unsafe.Pointer(namePtr)),
		0,
		uintptr(valueType),
		dataPtr,
		uintptr(len(data)),
	)
	if result != 0 {
		return syscall.Errno(result)
	}
	return nil
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

type registryValueData struct {
	data      []byte
	valueType string
	preview   string
}

// readRegistryValue reads a registry value and serialises it to UTF-8 bytes.
func readRegistryValue(k registry.Key, name string) (registryValueData, error) {
	name = apiValueName(name)
	// Try as string (REG_SZ, REG_EXPAND_SZ).
	if s, valueType, err := k.GetStringValue(name); err == nil {
		return registryValueData{
			data:      []byte(s),
			valueType: registryTypeName(valueType),
			preview:   previewRegistryValue(s),
		}, nil
	}

	// Try as integer (REG_DWORD, REG_QWORD). Preserve the Registry value's
	// declared capacity in the hexadecimal representation: DWORD is 32-bit and
	// QWORD is 64-bit, even when the numeric value itself has leading zeroes.
	if n, valueType, err := k.GetIntegerValue(name); err == nil {
		var serialized string
		switch valueType {
		case registry.DWORD, registry.DWORD_BIG_ENDIAN:
			serialized = fmt.Sprintf("%d (0x%08X)", uint32(n), uint32(n))
		case registry.QWORD:
			serialized = fmt.Sprintf("%d (0x%016X)", n, n)
		default:
			serialized = fmt.Sprintf("%d (0x%X)", n, n)
		}
		return registryValueData{
			data:      []byte(serialized),
			valueType: registryTypeName(valueType),
			preview:   serialized,
		}, nil
	}

	// Try as multi-string (REG_MULTI_SZ).
	if ss, valueType, err := k.GetStringsValue(name); err == nil {
		serialized := strings.Join(ss, "\n")
		return registryValueData{
			data:      []byte(serialized),
			valueType: registryTypeName(valueType),
			preview:   previewRegistryValue(serialized),
		}, nil
	}

	// Fall back to the raw Registry representation. This preserves REG_BINARY,
	// REG_NONE and less common value types instead of coercing them.
	b, valueType, err := rawRegistryValue(k, name)
	if err != nil {
		return registryValueData{}, fmt.Errorf("registry: read value %q: %w", name, err)
	}
	serialized := formatRegistryBytes(b)
	return registryValueData{
		data:      []byte(serialized),
		valueType: registryTypeName(valueType),
		preview:   previewRegistryValue(serialized),
	}, nil
}

func registryTypeName(valueType uint32) string {
	switch valueType {
	case registry.NONE:
		return "REG_NONE"
	case registry.SZ:
		return "REG_SZ"
	case registry.EXPAND_SZ:
		return "REG_EXPAND_SZ"
	case registry.BINARY:
		return "REG_BINARY"
	case registry.DWORD:
		return "REG_DWORD"
	case registry.DWORD_BIG_ENDIAN:
		return "REG_DWORD_BIG_ENDIAN"
	case registry.LINK:
		return "REG_LINK"
	case registry.MULTI_SZ:
		return "REG_MULTI_SZ"
	case registry.RESOURCE_LIST:
		return "REG_RESOURCE_LIST"
	case registry.FULL_RESOURCE_DESCRIPTOR:
		return "REG_FULL_RESOURCE_DESCRIPTOR"
	case registry.RESOURCE_REQUIREMENTS_LIST:
		return "REG_RESOURCE_REQUIREMENTS_LIST"
	case registry.QWORD:
		return "REG_QWORD"
	default:
		return fmt.Sprintf("REG_0x%X", valueType)
	}
}

func previewRegistryValue(value string) string {
	preview := strings.ReplaceAll(value, "\r\n", " | ")
	preview = strings.ReplaceAll(preview, "\n", " | ")
	preview = strings.ReplaceAll(preview, "\t", " ")
	preview = strings.TrimSpace(preview)
	if preview == "" {
		return "(empty)"
	}
	const limit = 120
	if len(preview) > limit {
		return preview[:limit-1] + "..."
	}
	return preview
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
func (k *keyInfo) RegistryKind() string { return "Key" }
func (k *keyInfo) RegistryValueType() string {
	return ""
}
func (k *keyInfo) RegistryPreview() string { return "" }

// ── valueInfo ─────────────────────────────────────────────────────────────────

type valueInfo struct {
	path      string
	name      string
	size      int64
	modTime   time.Time
	valueType string
	preview   string
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
func (v *valueInfo) RegistryKind() string { return "Value" }
func (v *valueInfo) RegistryValueType() string {
	return v.valueType
}
func (v *valueInfo) RegistryPreview() string { return v.preview }

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

// writeFile buffers a textual Registry representation. Existing values keep
// their Registry type; newly created values default to REG_SZ.
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
	return writeRegistryTextValue(w.key, apiValueName(w.valName), w.buf.String())
}

func writeRegistryTextValue(k registry.Key, name, text string) error {
	_, valueType, err := k.GetValue(name, nil)
	if errors.Is(err, registry.ErrNotExist) {
		return k.SetStringValue(name, text)
	}
	if err != nil && !errors.Is(err, registry.ErrShortBuffer) {
		return mapAccessError(err)
	}

	switch valueType {
	case registry.SZ:
		return k.SetStringValue(name, text)
	case registry.EXPAND_SZ:
		return k.SetExpandStringValue(name, text)
	case registry.MULTI_SZ:
		normalized := strings.ReplaceAll(text, "\r\n", "\n")
		if normalized == "" {
			return k.SetStringsValue(name, []string{})
		}
		return k.SetStringsValue(name, strings.Split(normalized, "\n"))
	case registry.DWORD:
		value, err := parseRegistryInteger(text, 32)
		if err != nil {
			return err
		}
		return k.SetDWordValue(name, uint32(value))
	case registry.DWORD_BIG_ENDIAN:
		value, err := parseRegistryInteger(text, 32)
		if err != nil {
			return err
		}
		data := make([]byte, 4)
		binary.BigEndian.PutUint32(data, uint32(value))
		return setRawRegistryValue(k, name, valueType, data)
	case registry.QWORD:
		value, err := parseRegistryInteger(text, 64)
		if err != nil {
			return err
		}
		return k.SetQWordValue(name, value)
	default:
		data, err := parseRegistryBytes(text)
		if err != nil {
			return err
		}
		return setRawRegistryValue(k, name, valueType, data)
	}
}

func parseRegistryInteger(text string, bits int) (uint64, error) {
	value := strings.TrimSpace(text)
	if i := strings.Index(value, "("); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	if value == "" {
		return 0, fmt.Errorf("registry integer value is empty")
	}
	n, err := strconv.ParseUint(value, 0, bits)
	if err != nil {
		return 0, fmt.Errorf("invalid REG_DWORD/REG_QWORD value %q: %w", text, err)
	}
	return n, nil
}

func parseRegistryBytes(text string) ([]byte, error) {
	value := strings.TrimSpace(text)
	if value == "" || strings.EqualFold(value, "(empty)") {
		return nil, nil
	}
	fields := strings.Fields(strings.NewReplacer(",", " ", ";", " ").Replace(value))
	out := make([]byte, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimPrefix(strings.TrimPrefix(field, "0x"), "0X")
		if len(field) == 0 || len(field) > 2 {
			return nil, fmt.Errorf("invalid registry byte %q", field)
		}
		n, err := strconv.ParseUint(field, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid registry byte %q: %w", field, err)
		}
		out = append(out, byte(n))
	}
	return out, nil
}

func formatRegistryBytes(data []byte) string {
	if len(data) == 0 {
		return "(empty)"
	}
	var sb strings.Builder
	for i, byt := range data {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "%02X", byt)
	}
	return sb.String()
}

// Compile-time interface check.
var _ vfs.FileSystem = (*FS)(nil)
