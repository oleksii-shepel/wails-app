package presentation

import "testing"

func TestDetectFormat(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want Format
	}{
		{"pptx", []byte{'P', 'K', 0x03, 0x04, 0}, FormatPPTX},
		{"ppt", []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, FormatPPT},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectFormat(tc.data)
			if err != nil || got != tc.want {
				t.Fatalf("DetectFormat() = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := DetectFormat([]byte("not office")); err == nil {
		t.Fatal("expected unknown format error")
	}
}

func TestIsPowerPointExtension(t *testing.T) {
	for _, name := range []string{"deck.ppt", "deck.PPTX", "show.pptm", "template.potx"} {
		if !IsPowerPointExtension(name) {
			t.Errorf("expected %q supported", name)
		}
	}
	if IsPowerPointExtension("notes.docx") {
		t.Fatal("docx must not be treated as PowerPoint")
	}
}
