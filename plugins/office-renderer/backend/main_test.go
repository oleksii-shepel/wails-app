package main

import "testing"

func TestNativePath(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"filesystem://c/Users/oleks/report.docx", `c:\Users\oleks\report.docx`},
		{"filesystem://C", `C:\`},
		{`C:\Users\oleks\report.docx`, `C:\Users\oleks\report.docx`},
	} {
		got, err := nativePath(test.input)
		if err != nil || got != test.want {
			t.Fatalf("nativePath(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
	}
}

func TestDocumentKindSupportsLegacyAndOOXMLVariants(t *testing.T) {
	for extension, want := range map[string]string{
		".doc": "doc", ".docx": "docx", ".docm": "docx", ".dotx": "docx", ".dotm": "docx",
		".xls": "xls", ".xlsx": "xlsx", ".xlsm": "xlsx", ".xltx": "xlsx", ".xltm": "xlsx", ".xlam": "xlsx",
		".ppt": "ppt", ".pptx": "pptx", ".pptm": "pptx", ".ppsx": "pptx", ".ppsm": "pptx", ".potx": "pptx", ".potm": "pptx",
	} {
		if got := documentKind(extension); got != want {
			t.Errorf("documentKind(%q) = %q, want %q", extension, got, want)
		}
	}
	if got := documentKind(".pdf"); got != "" {
		t.Errorf("documentKind(.pdf) = %q, want unsupported", got)
	}
}
