package platform

import (
	"testing"
)

func TestToSlash(t *testing.T) {
	if got := ToSlash(`C:\Users\name`); got != "C:/Users/name" {
		t.Fatalf("ToSlash failed: %q", got)
	}
	if got := ToSlash("/home/user"); got != "/home/user" {
		t.Fatalf("ToSlash failed: %q", got)
	}
}

func TestCleanPath(t *testing.T) {
	if got := CleanPath("/a/b/../c"); got != "/a/c" {
		t.Fatalf("CleanPath failed: %q", got)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`C:\Users\name`, "C:/Users/name"},
		{"/home/user/", "home/user"},
		{"///a///b///", "a///b"},
		{"", ""},
		{"no-separators", "no-separators"},
	}
	for _, tc := range tests {
		got := Normalize(tc.input)
		if got != tc.want {
			t.Fatalf("Normalize(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestJoinNormalized(t *testing.T) {
	tests := []struct {
		base  string
		child string
		want  string
	}{
		{"/home", "docs", "home/docs"},
		{"", "docs", "docs"},
		{"/home/", "/docs/", "home/docs"},
	}
	for _, tc := range tests {
		got := JoinNormalized(tc.base, tc.child)
		if got != tc.want {
			t.Fatalf("JoinNormalized(%q, %q) = %q, want %q", tc.base, tc.child, got, tc.want)
		}
	}
}

func TestJoin(t *testing.T) {
	if got := Join("a", "b", "c"); got != "a/b/c" {
		t.Fatalf("Join failed: %q", got)
	}
	if got := Join(); got != "" {
		t.Fatalf("Join() with no args failed: %q", got)
	}
}

func TestParentPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user/docs", "/home/user"},
		{"/home/user", "/home"},
		{"/home", "/"},
		{"/", "/"},
		{"C:/Users", "C:/"},
	}
	for _, tc := range tests {
		got := ParentPath(tc.input)
		if got != tc.want {
			t.Fatalf("ParentPath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSplitPath(t *testing.T) {
	dir, base, name, ext := SplitPath("/home/user/file.txt")
	if dir != "/home/user" || base != "file.txt" || name != "file" || ext != ".txt" {
		t.Fatalf("SplitPath failed: dir=%q base=%q name=%q ext=%q", dir, base, name, ext)
	}

	dir, base, name, ext = SplitPath("/home/user/.gitignore")
	// filepath.Ext(".gitignore") returns ".gitignore"; name becomes ""
	if name != "" || ext != ".gitignore" {
		t.Fatalf("SplitPath dotfile failed: name=%q ext=%q", name, ext)
	}
}

func TestBreadcrumbs(t *testing.T) {
	crumbs := Breadcrumbs("/home/user/docs")
	if len(crumbs) != 4 {
		t.Fatalf("expected 4 crumbs, got %d", len(crumbs))
	}
	// Root label is platform-dependent ("/" on Unix, "\" on Windows)
	if crumbs[1]["label"] != "home" || crumbs[2]["label"] != "user" || crumbs[3]["label"] != "docs" {
		t.Fatalf("unexpected labels: %+v", crumbs)
	}
	if crumbs[3]["path"] != "/home/user/docs" {
		t.Fatalf("expected last path to be /home/user/docs, got %s", crumbs[3]["path"])
	}
}

func TestCloudSchemeType(t *testing.T) {
	tests := []struct {
		scheme string
		want   string
	}{
		{"onedrive://", "cloud-onedrive"},
		{"gdrive://", "cloud-google"},
		{"dropbox://", "cloud-dropbox"},
		{"unknown", "unknown"},
	}
	for _, tc := range tests {
		got := cloudSchemeType(tc.scheme)
		if string(got) != tc.want {
			t.Fatalf("cloudSchemeType(%q) = %q, want %q", tc.scheme, got, tc.want)
		}
	}
}
