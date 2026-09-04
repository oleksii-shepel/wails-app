package legacy

import (
	"testing"

	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

func TestDecodeTextProperties(t *testing.T) {
	props := []escher.Property{
		{ID: escher.PropTextLeft, Value: 12700},
		{ID: escher.PropWrapText, Value: 0},
		{ID: escher.PropAnchorText, Value: 4},
		{ID: escher.PropTextFlow, Value: 1},
	}
	got := DecodeTextProperties(props)
	if !got.HasLeft || got.Insets.Left != 12700 {
		t.Fatalf("left = %+v", got)
	}
	if got.Wrap != ir.TextWrapNone {
		t.Fatalf("wrap = %q", got.Wrap)
	}
	if got.Anchor != ir.TextAnchorCenter || !got.AnchorCenter {
		t.Fatalf("anchor = %q center=%v", got.Anchor, got.AnchorCenter)
	}
	if got.Vert != ir.TextVerticalRL {
		t.Fatalf("vert = %q", got.Vert)
	}
}

func TestDecodeTextPropertiesKeepsExplicitHorizontalFlow(t *testing.T) {
	props := []escher.Property{{ID: escher.PropTextFlow, Value: 0}}
	got := DecodeTextProperties(props)
	if !got.HasVert {
		t.Fatal("explicit horizontal flow must retain presence")
	}
	if got.Vert != "" {
		t.Fatalf("Vert=%q want horizontal", got.Vert)
	}
}

func TestDecodeTextPropertiesAutoFitShape(t *testing.T) {
	props := []escher.Property{{ID: escher.PropTextBooleans, Value: (1 << 16) | 1}}
	got := DecodeTextProperties(props)
	if !got.HasAutoFit || got.AutoFit != ir.AutoFitShape {
		t.Fatalf("autofit=%q present=%v", got.AutoFit, got.HasAutoFit)
	}
}
