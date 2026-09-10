package render_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

)

func fixturePassword(t *testing.T, formatDir, name string) string {
	t.Helper()
	data, err := os.ReadFile(repoPath(t, "testdata", "passwords.json"))
	if err != nil {
		t.Fatalf("read encrypted fixture credentials: %v", err)
	}
	var passwords map[string]string
	if err := json.Unmarshal(data, &passwords); err != nil {
		t.Fatalf("parse encrypted fixture credentials: %v", err)
	}
	key := filepath.ToSlash(filepath.Join(formatDir, name))
	return passwords[key]
}

func docxFixturePassword(t *testing.T, name string) string {
	t.Helper()
	return fixturePassword(t, "docx", name)
}
