package browserpane

import (
	"testing"
)

func TestIsSupportedBrowserPaneURL(t *testing.T) {
	cases := []struct {
		url      string
		expected bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"https://example.com/path", true},
		{"https://example.com:8080/path", true},
		{"", false},
		{"  ", false},
		{"ftp://example.com", false},
		{"file:///etc/passwd", false},
		{"javascript:alert(1)", false},
		{"example.com", false},
		{"/local/path", false},
		{"https://", false},
	}

	for _, c := range cases {
		got := isSupportedBrowserPaneURL(c.url)
		if got != c.expected {
			t.Fatalf("isSupportedBrowserPaneURL(%q) = %v, want %v", c.url, got, c.expected)
		}
	}
}

func TestNormalizeBrowserPaneMessageURL(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"https://Example.COM/path/", "https://example.com/path"},
		{"https://example.com/path#fragment", "https://example.com/path"},
		{"https://example.com/", "https://example.com"},
		{"https://example.com", "https://example.com"},
		{"https://example.com/path?a=1", "https://example.com/path?a=1"},
		{"", ""},
		{"  ", ""},
		{"ftp://example.com", ""},
		{"not-a-url", ""},
	}

	for _, c := range cases {
		got := normalizeBrowserPaneMessageURL(c.input)
		if got != c.expected {
			t.Fatalf("normalizeBrowserPaneMessageURL(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestNormalizeBrowserPaneTheme(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"light", "light"},
		{"LIGHT", "light"},
		{" Light ", "light"},
		{"dark", "dark"},
		{"DARK", "dark"},
		{"", "dark"},
		{"blue", "dark"},
	}

	for _, c := range cases {
		got := normalizeBrowserPaneTheme(c.input)
		if got != c.expected {
			t.Fatalf("normalizeBrowserPaneTheme(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestParseBrowserPaneColor(t *testing.T) {
	cases := []struct {
		input       string
		wantR       uint8
		wantG       uint8
		wantB       uint8
		wantOk      bool
	}{
		{"#ff0000", 255, 0, 0, true},
		{"#FF0000", 255, 0, 0, true},
		{"#f00", 255, 0, 0, true},
		{"#123456", 18, 52, 86, true},
		{"#abc", 170, 187, 204, true},
		{"", 0, 0, 0, false},
		{"red", 0, 0, 0, false},
		{"#gg0000", 0, 0, 0, false},
		{"#12345", 0, 0, 0, false},
	}

	for _, c := range cases {
		r, g, b, ok := parseBrowserPaneColor(c.input)
		if r != c.wantR || g != c.wantG || b != c.wantB || ok != c.wantOk {
			t.Fatalf("parseBrowserPaneColor(%q) = (%d,%d,%d,%v), want (%d,%d,%d,%v)",
				c.input, r, g, b, ok, c.wantR, c.wantG, c.wantB, c.wantOk)
		}
	}
}