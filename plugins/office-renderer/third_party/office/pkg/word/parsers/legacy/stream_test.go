package legacy

import (
	"errors"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestValidateLegacyStreamRunsRejectsLossyConstructs(t *testing.T) {
	tests := []struct {
		name        string
		runs        []binaryWordRun
		textboxes   int
		wantFeature string
	}{
		{
			name:        "ole anchor",
			runs:        []binaryWordRun{{Text: "\x01", Spec: true, Obj: true}},
			wantFeature: "OLE object anchor",
		},
		{
			name:        "textbox",
			runs:        []binaryWordRun{{Text: "body"}},
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
	tests := [][]binaryWordRun{
		{{Text: "hello", ParaFormat: wordir.ParagraphFormat{}}},
		{{Text: "cell\a", ParaFormat: wordir.ParagraphFormat{InTable: true, IsTableCellTrailingParagraph: true}, TableDepth: 1}},
		// fData is a generic data-bearing run flag, not picture identity. Word97
		// fixed pictures are resolved earlier from fSpec + sprmCPicLocation, so a
		// generic fData run must not be rejected by this compatibility validator.
		{{Text: "form-data", Spec: true, Data: true}},
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
	if got := legacyStreamTableDepth(binaryWordRun{TableDepth: 2}); got != 2 {
		t.Fatalf("TableDepth = %d, want 2", got)
	}
	if got := legacyStreamTableDepth(binaryWordRun{ParaFormat: wordir.ParagraphFormat{InTable: true}}); got != 1 {
		t.Fatalf("InTable depth = %d, want 1", got)
	}
	if got := legacyStreamTableDepth(binaryWordRun{Text: "cell\a"}); got != 1 {
		t.Fatalf("cell marker depth = %d, want 1", got)
	}
	if got := legacyStreamTableDepth(binaryWordRun{Text: "body"}); got != 0 {
		t.Fatalf("plain depth = %d, want 0", got)
	}
}
