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
