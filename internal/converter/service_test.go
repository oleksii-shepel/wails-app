package converter

import "testing"

func TestClassifyDocumentFormat(t *testing.T) {
	tests := []struct {
		ext  string
		want documentFormatKind
	}{
		{ext: ".docx", want: documentFormatOffice},
		{ext: "epub", want: documentFormatEpub},
		{ext: "djvu", want: documentFormatDjvu},
		{ext: "txt", want: documentFormatUnknown},
	}

	for _, tt := range tests {
		if got := classifyDocumentFormat(tt.ext); got != tt.want {
			t.Fatalf("classifyDocumentFormat(%q) = %q, want %q", tt.ext, got, tt.want)
		}
	}
}

func TestNormalizeOutputExtension(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{format: "jpeg", want: "jpg"},
		{format: "tiff", want: "tif"},
		{format: "PNG", want: "png"},
		{format: "webp", want: "webp"},
	}

	for _, tt := range tests {
		if got := normalizeOutputExtension(tt.format); got != tt.want {
			t.Fatalf("normalizeOutputExtension(%q) = %q, want %q", tt.format, got, tt.want)
		}
	}
}
