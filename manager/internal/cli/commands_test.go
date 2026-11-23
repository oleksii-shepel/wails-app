package cli

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wails-app/internal/contracts"
)

func TestFilesystemCommands(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", "")
	for path, data := range map[string]string{
		"sample file.txt": "alpha\nneedle\n",
		"copy.txt":        "alpha\nneedle\n",
		"other.txt":       "alpha\nchanged\n",
		"left/a.txt":      "one",
		"right/a.txt":     "two",
	} {
		target := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cli := Services{}
	run := func(command string, code int, want string) Result {
		t.Helper()
		result := cli.Execute(command, dir, false)
		if result.ExitCode != code || result.OK != (code == 0) || !strings.Contains(result.Stdout, want) {
			t.Fatalf("%s: %+v", command, result)
		}
		return result
	}
	pwd := run("cd left", 0, "")
	if !pwd.ChangedCwd || pwd.Cwd != filepath.Join(dir, "left") {
		t.Fatalf("cd: %+v", pwd)
	}
	if run("cd missing", 1, "").ChangedCwd {
		t.Fatal("changed directory on failure")
	}
	run("pwd", 0, dir)
	run("du --json --top 2", 0, "totalBytes")
	run("find -p '*.txt' -g needle", 0, "sample file.txt:L2 (1x) needle")
	run("dup --json", 0, "copy.txt")
	run("diff --context 0 'sample file.txt' other.txt", 1, "+changed")
	run("compare left right --hash", 1, "a.txt (content)")
	manifest := run("hash --manifest 'sample file.txt'", 0, "  sample file.txt").Stdout
	if err := os.WriteFile(filepath.Join(dir, "checksums"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	run("verify checksums", 0, "1 passed")
	run("rename other.txt --pattern '{name}_{nnn}'", 0, "Dry run")
	run("rename other.txt --pattern '{name}_{nnn}' --apply", 0, "other_001.txt")
	if _, err := os.Stat(filepath.Join(dir, "other_001.txt")); err != nil {
		t.Fatal(err)
	}
	run("rename copy.txt --pattern 'sample file' --apply", 1, "")
	run("trash empty", 2, "")
	run("ls --unknown", 2, "")
	run("ls '", 2, "")
}

func TestSplitJoinAndArchiveCommands(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", "")
	sample := bytes.Repeat([]byte{0, 1, 2, 3}, 700)
	if err := os.WriteFile(filepath.Join(dir, "sample.bin"), sample, 0600); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(dir, "sample.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("nested/sample.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(sample); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	cli := Services{}
	for _, command := range []string{
		"split sample.bin --size 1K --out-dir parts",
		"join parts/sample.bin.part001 --output joined.bin",
		"archive list sample.zip --inner nested",
		"archive extract sample.zip extracted",
	} {
		if result := cli.Execute(command, dir, false); !result.OK {
			t.Fatalf("%s: %+v", command, result)
		}
	}
	for _, path := range []string{"joined.bin", filepath.Join("extracted", "nested", "sample.bin")} {
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil || !bytes.Equal(sample, data) {
			t.Fatalf("invalid output %s: %v", path, err)
		}
	}
	for _, command := range []string{
		"split sample.bin --size 1K --out-dir parts",
		"join parts/sample.bin.part001 --output joined.bin",
		"archive extract sample.zip extracted",
	} {
		if result := cli.Execute(command, dir, false); result.OK {
			t.Fatalf("%s replaced existing output", command)
		}
	}
}

func TestWorkspacePathsUseHostWorkspace(t *testing.T) {
	state := contracts.WorkspaceList{ActiveWorkspaceID: "work", Workspaces: []contracts.WorkspaceConfig{{
		ID: "work", Name: "My Workspace", LeftPane: contracts.PaneState{ActiveTabID: "left", Tabs: []contracts.TabInfo{{ID: "left", Path: "filesystem://c/Users"}}},
	}}}
	cli := Services{Workspaces: func() contracts.WorkspaceList { return state }}
	result := cli.Execute("ws paths --json", t.TempDir(), false)
	if !result.OK || !strings.Contains(result.Stdout, "filesystem://c/Users") {
		t.Fatalf("ws paths: %+v", result)
	}
}
