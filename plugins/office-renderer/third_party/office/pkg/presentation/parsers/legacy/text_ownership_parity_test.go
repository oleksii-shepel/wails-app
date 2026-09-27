package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestOrdinaryBodyTextTypeDoesNotStealExplicitPlaceholderFragment(t *testing.T) {
	metas := []legacyPptTextShape{
		{shapeID: 10, outlineRef: -1, textType: pptTextTypeBody, bounds: presentationir.Rect{W: 100, H: 100}},
		{shapeID: 20, outlineRef: -1, textType: pptTextTypeBody, placeholder: presentationir.PlaceholderBody, bounds: presentationir.Rect{W: 100, H: 100}},
	}
	slideText := legacySlideText{Fragments: []legacySlideFragment{{
		ShapeID:  20,
		TextType: pptTextTypeBody,
		Text:     "owned by placeholder",
	}}}

	got := buildLegacyPptTextShapesFromMetas(metas, slideText)
	if len(got) != 1 {
		t.Fatalf("materialized text shapes = %d, want only explicitly owned placeholder", len(got))
	}
	if got[0].OriginalShapeID != 20 || got[0].Placeholder != presentationir.PlaceholderBody {
		t.Fatalf("owner = id %d placeholder %v, want id 20/body", got[0].OriginalShapeID, got[0].Placeholder)
	}
	if text := paragraphText(got[0].Paragraphs[0]); text != "owned by placeholder" {
		t.Fatalf("placeholder text = %q", text)
	}
}

func TestSlideLocalTextTypeDoesNotInventPlaceholderRole(t *testing.T) {
	for _, textType := range []int{pptTextTypeTitle, pptTextTypeBody, pptTextTypeCenterBody, pptTextTypeOther} {
		metas := []legacyPptTextShape{{
			shapeID:    10,
			textType:   textType,
			bounds:     presentationir.Rect{W: 100, H: 100},
			paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "ordinary"}}}},
		}}
		got := buildLegacyPptTextShapesFromMetas(metas, legacySlideText{})
		if len(got) != 1 {
			t.Fatalf("text type %d shapes = %d", textType, len(got))
		}
		if got[0].Placeholder != presentationir.PlaceholderNone {
			t.Fatalf("text type %d invented placeholder %v", textType, got[0].Placeholder)
		}
	}
}
