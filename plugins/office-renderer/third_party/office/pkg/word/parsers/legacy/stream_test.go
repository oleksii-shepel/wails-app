package legacy

import (
	"errors"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestValidateLegacyStreamRunsRejectsLossyConstructs(t *testing.T) {
	tests := []struct {
		name        string
		runs        []docRun
		textboxes   int
		wantFeature string
	}{
		{
			name:        "unresolved picture anchor",
			runs:        []docRun{{Text: "\x01", Spec: true, Data: true}},
			wantFeature: "picture/data anchor",
		},
		{
			name:        "ole anchor",
			runs:        []docRun{{Text: "\x01", Spec: true, Obj: true}},
			wantFeature: "OLE object anchor",
		},
		{
			name:        "textbox",
			runs:        []docRun{{Text: "body"}},
			textboxes:   1,
			wantFeature: "textbox",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLegacyStreamRuns(tc.runs, tc.textboxes)
			var unsupported StreamUnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("error = %v, want StreamUnsupportedError", err)
			}
			if unsupported.Feature != tc.wantFeature {
				t.Fatalf("feature = %q, want %q", unsupported.Feature, tc.wantFeature)
			}
		})
	}
}

func TestValidateLegacyStreamRunsAcceptsResolvedContent(t *testing.T) {
	tests := [][]docRun{
		{{Text: "hello", ParaFormat: wordir.ParagraphFormat{}}},
		{{Text: "cell\a", ParaFormat: wordir.ParagraphFormat{InTable: true, IsTableCellTrailingParagraph: true}, TableDepth: 1}},
		{{Text: "\x01", Spec: true, Data: true, Image: &wordir.Image{MIMEType: "image/png", Data: []byte{1}}}},
		{{Text: string(legacyDrawingAnchorChar), Spec: true, Shape: &wordir.Shape{Kind: "shape"}}},
	}
	for _, runs := range tests {
		if err := validateLegacyStreamRuns(runs, 0); err != nil {
			t.Fatalf("validateLegacyStreamRuns: %v", err)
		}
	}
}

func TestLegacyStreamTableDepthUsesBinaryHints(t *testing.T) {
	if got := legacyStreamTableDepth(docRun{TableDepth: 2}); got != 2 {
		t.Fatalf("TableDepth = %d, want 2", got)
	}
	if got := legacyStreamTableDepth(docRun{ParaFormat: wordir.ParagraphFormat{InTable: true}}); got != 1 {
		t.Fatalf("InTable depth = %d, want 1", got)
	}
	if got := legacyStreamTableDepth(docRun{Text: "cell\a"}); got != 1 {
		t.Fatalf("cell marker depth = %d, want 1", got)
	}
	if got := legacyStreamTableDepth(docRun{Text: "body"}); got != 0 {
		t.Fatalf("plain depth = %d, want 0", got)
	}
}
