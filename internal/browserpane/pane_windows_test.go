//go:build windows

package browserpane

import (
	"testing"
)

func TestExtractCDPBase64Data(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{`{"data":"iVBORw0KGgo="}`, "iVBORw0KGgo=", false},
		{`{"other":"x","data":"abc"}`, "abc", false},
		{`{}`, "", true},
		{`{"data":}`, "", true},
		{`not-json`, "", true},
	}

	for _, c := range cases {
		got, err := extractCDPBase64Data(c.input)
		if (err != nil) != c.wantErr {
			t.Fatalf("extractCDPBase64Data(%q) error = %v, wantErr %v", c.input, err, c.wantErr)
		}
		if got != c.expected {
			t.Fatalf("extractCDPBase64Data(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestExtractCDPStringValue(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{`{"result":{"type":"string","value":"hello"}}`, "hello", false},
		{`{"result":{"type":"string","value":""}}`, "", false},
		{`{"result":{"type":"string","value":"  spaced  "}}`, "spaced", false},
		{`{"result":{"type":"undefined"},"exceptionDetails":null}`, "", false},
		{`{"result":{"type":"string","value":"ok"},"exceptionDetails":{"lineNumber":1}}`, "", true},
		{`not-json`, "", true},
	}

	for _, c := range cases {
		got, err := extractCDPStringValue(c.input)
		if (err != nil) != c.wantErr {
			t.Fatalf("extractCDPStringValue(%q) error = %v, wantErr %v", c.input, err, c.wantErr)
		}
		if got != c.expected {
			t.Fatalf("extractCDPStringValue(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}
