package webresourcecache

import (
	"testing"
)

func TestStoreAndLoad(t *testing.T) {
	Clear("https://example.com")
	defer Clear("https://example.com")

	changed := Store("https://example.com", []Resource{
		{URL: "https://example.com/app.js", Size: 42, InitiatorType: "script", MimeType: "application/javascript"},
		{URL: "https://example.com/style.css", Size: 10, InitiatorType: "link", MimeType: "text/css"},
	})
	if !changed {
		t.Fatalf("expected Store to return true on first insert")
	}

	resources := Load("https://example.com")
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Store identical data should return false
	changed = Store("https://example.com", []Resource{
		{URL: "https://example.com/app.js", Size: 42, InitiatorType: "script", MimeType: "application/javascript"},
		{URL: "https://example.com/style.css", Size: 10, InitiatorType: "link", MimeType: "text/css"},
	})
	if changed {
		t.Fatalf("expected Store to return false when data is unchanged")
	}
}

func TestStoreIgnoresSamePageURL(t *testing.T) {
	Clear("https://example.com")
	defer Clear("https://example.com")

	Store("https://example.com", []Resource{
		{URL: "https://example.com/", Size: 100},
	})

	resources := Load("https://example.com")
	if len(resources) != 0 {
		t.Fatalf("expected same-page URL to be filtered out, got %d resources", len(resources))
	}
}

func TestMergeUpdatesResource(t *testing.T) {
	Clear("https://example.com")
	defer Clear("https://example.com")

	Store("https://example.com", []Resource{
		{URL: "https://example.com/app.js", Size: 42, InitiatorType: "script"},
	})

	changed := Merge("https://example.com", Resource{URL: "https://example.com/app.js", Size: 99, MimeType: "application/javascript"})
	if !changed {
		t.Fatalf("expected Merge to return true when data changes")
	}

	resources := Load("https://example.com")
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].Size != 99 {
		t.Fatalf("expected size 99, got %d", resources[0].Size)
	}
	if resources[0].MimeType != "application/javascript" {
		t.Fatalf("expected mimeType application/javascript, got %q", resources[0].MimeType)
	}
	if resources[0].InitiatorType != "script" {
		t.Fatalf("expected initiatorType script to be preserved, got %q", resources[0].InitiatorType)
	}
}

func TestMergeNoOp(t *testing.T) {
	Clear("https://example.com")
	defer Clear("https://example.com")

	Store("https://example.com", []Resource{
		{URL: "https://example.com/app.js", Size: 42},
	})

	changed := Merge("https://example.com", Resource{URL: "https://example.com/app.js", Size: 42})
	if changed {
		t.Fatalf("expected Merge to return false when nothing changes")
	}
}

func TestClear(t *testing.T) {
	Store("https://example.com", []Resource{{URL: "https://example.com/a.js", Size: 1}})
	Clear("https://example.com")

	resources := Load("https://example.com")
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources after clear, got %d", len(resources))
	}
}

func TestLoadSortsByURL(t *testing.T) {
	Clear("https://example.com")
	defer Clear("https://example.com")

	Store("https://example.com", []Resource{
		{URL: "https://example.com/z.js", Size: 1},
		{URL: "https://example.com/a.js", Size: 2},
		{URL: "https://example.com/m.js", Size: 3},
	})

	resources := Load("https://example.com")
	if resources[0].URL != "https://example.com/a.js" {
		t.Fatalf("expected first resource to be a.js, got %q", resources[0].URL)
	}
	if resources[1].URL != "https://example.com/m.js" {
		t.Fatalf("expected second resource to be m.js, got %q", resources[1].URL)
	}
	if resources[2].URL != "https://example.com/z.js" {
		t.Fatalf("expected third resource to be z.js, got %q", resources[2].URL)
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"https://Example.COM/path/", "https://example.com/path"},
		{"https://example.com/path#fragment", "https://example.com/path"},
		{"https://example.com/", "https://example.com"},
		{"  https://example.com  ", "https://example.com"},
		{"ftp://example.com", ""},
		{"", ""},
		{"not-a-url", ""},
	}

	for _, c := range cases {
		got := normalizeURL(c.input)
		if got != c.expected {
			t.Fatalf("normalizeURL(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestStoreEmptyPageURL(t *testing.T) {
	changed := Store("", []Resource{{URL: "https://example.com/a.js", Size: 1}})
	if changed {
		t.Fatalf("expected Store to return false for empty page URL")
	}
}

func TestMergeEmptyPageURL(t *testing.T) {
	changed := Merge("", Resource{URL: "https://example.com/a.js", Size: 1})
	if changed {
		t.Fatalf("expected Merge to return false for empty page URL")
	}
}

func TestMergeSameResourceAsPage(t *testing.T) {
	changed := Merge("https://example.com", Resource{URL: "https://example.com", Size: 1})
	if changed {
		t.Fatalf("expected Merge to return false when resource URL equals page URL")
	}
}
