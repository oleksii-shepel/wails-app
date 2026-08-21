//go:build !windows

package platform

import (
	"os"
	"sort"
	"strings"

	contracts "wails-app/internal/contracts"
)

func ListEnvironmentSurface() ([]contracts.SystemSurfaceRecord, error) {
	records := make([]contracts.SystemSurfaceRecord, 0, len(os.Environ()))
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
	sort.SliceStable(records, func(i, j int) bool {
		return strings.ToLower(records[i].Name) < strings.ToLower(records[j].Name)
	})
	return records, nil
}
