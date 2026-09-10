package sax

import "testing"

func TestWordprocessingShapeURIFallsBackToShapeClassification(t *testing.T) {
	const uri = "http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
	if got := normalizeGraphicDataURI(uri); got != "shape" {
		t.Fatalf("normalizeGraphicDataURI(%q) = %q, want shape", uri, got)
	}
}
