package ooxml

import (
	"bytes"
	"testing"
)

func TestParseReaderAtRejectsNilReader(t *testing.T) {
	if _, err := ParseReaderAt(nil, 0); err == nil {
		t.Fatal("ParseReaderAt(nil) succeeded, want error")
	}
}

func TestParseReaderAtRejectsNegativeSize(t *testing.T) {
	if _, err := ParseReaderAt(bytes.NewReader(nil), -1); err == nil {
		t.Fatal("ParseReaderAt with negative size succeeded, want error")
	}
}
