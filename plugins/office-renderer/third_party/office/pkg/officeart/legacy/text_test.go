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
	if got.Wrap != ir.TextWrapSquare {
		t.Fatalf("wrap = %q, want square", got.Wrap)
	}
	if got.Anchor != ir.TextAnchorCenter || !got.AnchorCenter {
		t.Fatalf("anchor = %q center=%v", got.Anchor, got.AnchorCenter)
	}
	if got.Vert != ir.TextVerticalRL {
		t.Fatalf("vert = %q", got.Vert)
	}
}

func TestDecodeTextPropertiesNoWrapIsExplicitValueTwo(t *testing.T) {
	for _, tc := range []struct {
		value uint32
		want  ir.TextWrap
	}{
		{value: 0, want: ir.TextWrapSquare},
		{value: 1, want: ir.TextWrapSquare},
		{value: 2, want: ir.TextWrapNone},
		{value: 3, want: ir.TextWrapSquare},
		{value: 4, want: ir.TextWrapSquare},
	} {
		got := DecodeTextProperties([]escher.Property{{ID: escher.PropWrapText, Value: tc.value}})
		if !got.HasWrap || got.Wrap != tc.want {
			t.Fatalf("wrap value %d = %q present=%v, want %q", tc.value, got.Wrap, got.HasWrap, tc.want)
		}
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

func TestTextVerticalFromFlowCoversMSOTXFLEnumeration(t *testing.T) {
	cases := map[uint32]ir.TextVertical{
		0: ir.TextHorizontal,
		1: ir.TextVerticalRL,
		2: ir.TextVertical270,
		3: ir.TextVerticalRL,
		4: ir.TextHorizontal,
		5: ir.TextVerticalRL,
	}
	for raw, want := range cases {
		if got := TextVerticalFromFlow(raw); got != want {
			t.Fatalf("TextVerticalFromFlow(%d) = %q, want %q", raw, got, want)
		}
	}
}
