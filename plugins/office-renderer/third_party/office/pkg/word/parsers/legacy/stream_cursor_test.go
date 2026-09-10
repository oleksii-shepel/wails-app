package legacy

import (
	"errors"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestLegacyRunSliceCursorPreservesOrder(t *testing.T) {
	cursor := newLegacyRunSliceCursor([]binaryWordRun{
		{Text: "one"},
		{Text: "two", ParaFormat: wordir.ParagraphFormat{InTable: true}},
	})
	first, ok, err := cursor.Next()
	if err != nil || !ok || first.Text != "one" {
		t.Fatalf("first = %#v, ok=%v err=%v", first, ok, err)
	}
	second, ok, err := cursor.Next()
	if err != nil || !ok || second.Text != "two" || !second.ParaFormat.InTable {
		t.Fatalf("second = %#v, ok=%v err=%v", second, ok, err)
	}
	_, ok, err = cursor.Next()
	if err != nil || ok {
		t.Fatalf("eof ok=%v err=%v", ok, err)
	}
}

func TestLegacySemanticRunCursorAppliesInheritedStyleAndListHints(t *testing.T) {
	istd := 7
	ilfo := 3
	level := 1
	style := wordir.Style{Format: wordir.ParagraphFormat{InTable: true, BinaryListOverride: &ilfo, BinaryListLevel: &level}}
	cursor := newLegacySemanticRunCursor(
		newLegacyRunSliceCursor([]binaryWordRun{{Text: "cell\a", ParaFormat: wordir.ParagraphFormat{BinaryStyleIndex: &istd}}}),
		&binaryWordStyleCatalog{paragraphByIndex: map[int]wordir.Style{istd: style}},
	)
	run, ok, err := cursor.Next()
	if err != nil || !ok {
		t.Fatalf("Next: run=%#v ok=%v err=%v", run, ok, err)
	}
	if !run.ParaFormat.InTable || !run.LegacyInTableHint || !run.ParaFormat.IsTableCellTrailingParagraph {
		t.Fatalf("table inheritance not applied: %#v", run)
	}
	ref := legacyStreamNumberingReference(run)
	if ref == nil || ref.NumID != ilfo || ref.Level != level {
		t.Fatalf("numbering = %#v, want numId=%d level=%d", ref, ilfo, level)
	}
}

func TestValidateLegacyStreamRunRejectsOnlyUnresolvedOLEAndRawFields(t *testing.T) {
	resolved := binaryWordRun{
		Text:  "\x01",
		Spec:  true,
		Data:  true,
		Image: &wordir.Image{MIMEType: "image/png"},
	}
	if err := validateLegacyStreamRun(resolved); err != nil {
		t.Fatalf("resolved image: %v", err)
	}
	// fData is not itself a picture discriminator. Word97 pictures are resolved
	// from fSpec + sprmCPicLocation before this compatibility validator.
	if err := validateLegacyStreamRun(binaryWordRun{Text: "\x01", Spec: true, Data: true}); err != nil {
		t.Fatalf("generic fData run: %v", err)
	}

	for _, tc := range []struct {
		name string
		run  binaryWordRun
		want string
	}{
		{name: "ole", run: binaryWordRun{Spec: true, Obj: true}, want: "OLE object anchor"},
		{name: "field", run: binaryWordRun{Text: "before\x13 PAGE \x14 1\x15after"}, want: "field"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLegacyStreamRun(tc.run)
			var unsupported StreamUnsupportedError
			if !errors.As(err, &unsupported) || unsupported.Feature != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
