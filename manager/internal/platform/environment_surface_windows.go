//go:build windows

package platform

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"

	contracts "wails-app/internal/contracts"
)

const (
	userEnvironmentRegistryPath   = `Environment`
	systemEnvironmentRegistryPath = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
)

// ListEnvironmentSurface returns process, user and system environment variables
// without starting PowerShell or any other child process.
func ListEnvironmentSurface() ([]contracts.SystemSurfaceRecord, error) {
	records := make([]contracts.SystemSurfaceRecord, 0, len(os.Environ())+64)

	for _, pair := range os.Environ() {
		name, value, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		records = append(records, contracts.SystemSurfaceRecord{
			ID:   "process:" + name,
			Name: name,
			Fields: map[string]any{
				"scope":     "Process",
				"value":     value,
				"valueType": "Process",
				"readOnly":  true,
			},
		})
	}

	user, userErr := environmentRegistryRecords(
		registry.CURRENT_USER,
		userEnvironmentRegistryPath,
		"User",
		"user:",
	)
	records = append(records, user...)

	system, systemErr := environmentRegistryRecords(
		registry.LOCAL_MACHINE,
		systemEnvironmentRegistryPath,
		"System",
		"system:",
	)
	records = append(records, system...)

	// Registry access to one scope should not hide process variables or the
	// other scope. Return an error only when no data at all can be produced.
	if len(records) == 0 {
		if userErr != nil {
			return nil, userErr
		}
		if systemErr != nil {
			return nil, systemErr
		}
	}

	sort.SliceStable(records, func(i, j int) bool {
		leftScope := environmentScopeOrder(fieldString(records[i].Fields, "scope"))
		rightScope := environmentScopeOrder(fieldString(records[j].Fields, "scope"))
		if leftScope != rightScope {
			return leftScope < rightScope
		}
		return strings.ToLower(records[i].Name) < strings.ToLower(records[j].Name)
	})

	return records, nil
}

func environmentRegistryRecords(
	root registry.Key,
	path string,
	scope string,
	idPrefix string,
) ([]contracts.SystemSurfaceRecord, error) {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return []contracts.SystemSurfaceRecord{}, err
	}
	defer key.Close()

	names, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, err
	}

	records := make([]contracts.SystemSurfaceRecord, 0, len(names))
	for _, name := range names {
		value, valueType, err := key.GetStringValue(name)
		if err != nil {
			// Environment values are expected to be REG_SZ or REG_EXPAND_SZ.
			// Ignore unrelated value kinds rather than failing the whole scope.
			continue
		}

		records = append(records, contracts.SystemSurfaceRecord{
			ID:   idPrefix + name,
			Name: name,
			Fields: map[string]any{
				"scope":        scope,
				"value":        value,
				"valueType":    environmentRegistryValueType(valueType),
				"expandable":   valueType == registry.EXPAND_SZ,
				"registryType": valueType,
				"readOnly":     false,
			},
		})
	}
	return records, nil
}

func environmentRegistryValueType(valueType uint32) string {
	switch valueType {
	case registry.EXPAND_SZ:
		return "Expandable"
	case registry.SZ:
		return "String"
	default:
		return "String"
	}
}

func environmentScopeOrder(scope string) int {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "process":
		return 0
	case "user":
		return 1
	case "system":
		return 2
	default:
		return 3
	}
}

func fieldString(fields map[string]any, key string) string {
	if fields == nil {
		return ""
	}
	value, ok := fields[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
