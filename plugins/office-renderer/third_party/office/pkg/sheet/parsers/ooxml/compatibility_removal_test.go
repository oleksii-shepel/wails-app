package ooxml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionHasNoCompatibilityOrFullParserBridge(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	forbidden := []string{
		"parts.LoadPackage(", "resolve.ResolvePackage(",
		"LoadCompatibilityPackage(", "ResolveCompatibilityPackage(",
	}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, symbol := range forbidden {
			if strings.Contains(string(data), symbol) {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s still uses obsolete OOXML parser bridge %q", filepath.ToSlash(rel), symbol)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
