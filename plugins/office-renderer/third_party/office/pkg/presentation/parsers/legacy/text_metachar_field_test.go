package legacy

import (
	"encoding/binary"
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func legacyMetaFieldTextBox(text string, fieldType uint16, position uint32) []byte {
	stream := buildPptAtomRecord(0, pptRTTextHeaderAtom, binary.LittleEndian.AppendUint32(nil, uint32(pptTextTypeOther)))
	stream = append(stream, buildPptAtomRecord(0, pptRTTextBytesAtom, []byte(text))...)
	stream = append(stream, buildPptAtomRecord(0, fieldType, binary.LittleEndian.AppendUint32(nil, position))...)
	return stream
}

func TestLegacyTextBoxDecodesSlideNumberMetacharacterIntoCanonicalFieldRun(t *testing.T) {
	frags, _ := parseLegacyPptTextBoxFragments(legacyMetaFieldTextBox("*", pptRTSlideNumberMCAtom, 0), nil, nil, nil)
	if len(frags) != 1 {
		t.Fatalf("fragments = %d, want 1", len(frags))
	}
	paras := fragmentParagraphs(frags[0])
	if len(paras) != 1 || len(paras[0].Runs) != 1 {
		t.Fatalf("paragraphs = %#v, want one field run", paras)
	}
	run := paras[0].Runs[0]
	if run.Text != "" || run.FieldKind != "slidenum" {
		t.Fatalf("run = %#v, want empty slidenum field", run)
	}
}

func TestLegacyTextBoxSplitsTextAroundMetacharacterWithoutLosingRunSemantics(t *testing.T) {
	frags, _ := parseLegacyPptTextBoxFragments(legacyMetaFieldTextBox("A*B", pptRTSlideNumberMCAtom, 1), nil, nil, nil)
	paras := fragmentParagraphs(frags[0])
	if len(paras) != 1 || len(paras[0].Runs) != 3 {
		t.Fatalf("paragraphs = %#v, want text/field/text runs", paras)
	}
	got := paras[0].Runs
	if got[0].Text != "A" || got[1].FieldKind != "slidenum" || got[1].Text != "" || got[2].Text != "B" {
		t.Fatalf("runs = %#v, want A + slidenum + B", got)
	}
}

func TestMasterArtworkOwnershipKeepsFieldOnlyText(t *testing.T) {
	shape := &presentationir.Shape{
		Kind:       presentationir.ShapeText,
		Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{FieldKind: "slidenum"}}}},
	}
	if !legacyPptShapePaintsMasterArtwork(shape) {
		t.Fatal("field-only master text must remain artwork")
	}
}
