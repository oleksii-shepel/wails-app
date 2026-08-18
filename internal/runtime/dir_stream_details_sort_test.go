package runtime

import (
	"testing"

	contracts "wails-app/internal/contracts"
)

func TestPreparedDirectoryStreamSupportsDetailsColumns(t *testing.T) {
	left := contracts.FileInfo{
		Name:              "b.txt",
		Path:              "/b.txt",
		Type:              contracts.FileTypeFile,
		Size:              20,
		Modified:          20,
		SearchParentPath:  "/z",
		RegistryKind:      "value",
		RegistryValueType: "REG_SZ",
		RegistryPreview:   "z",
	}
	right := contracts.FileInfo{
		Name:              "a.txt",
		Path:              "/a.txt",
		Type:              contracts.FileTypeFile,
		Size:              10,
		Modified:          10,
		SearchParentPath:  "/a",
		RegistryKind:      "key",
		RegistryValueType: "REG_BINARY",
		RegistryPreview:   "a",
	}

	for _, field := range []string{
		"size",
		"modified",
		"type",
		"searchParentPath",
		"registryKind",
		"registryValueType",
		"registryPreview",
	} {
		options := DirStreamOptions{
			SortField: field,
			SortDirection: "asc",
		}
		if comparePreparedEntries(left, right, options) == 0 {
			t.Fatalf("field %q did not participate in comparison", field)
		}
	}
}
