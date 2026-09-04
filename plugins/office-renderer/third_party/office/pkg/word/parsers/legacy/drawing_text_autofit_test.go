package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"

	officeartir "office/pkg/officeart/ir"
	officeartescher "office/pkg/officeart/legacy/escher"
)

func TestLegacyTextAutoFitNameMatchesWordRendererVocabulary(t *testing.T) {
	cases := []struct {
		in   officeartir.AutoFit
		want string
	}{
		{officeartir.AutoFitNone, "none"},
		{officeartir.AutoFitShape, "shape"},
		{officeartir.AutoFitNormal, "norm"},
	}
	for _, tc := range cases {
		if got := legacyTextAutoFitName(tc.in); got != tc.want {
			t.Fatalf("legacyTextAutoFitName(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestLegacyApplyOfficeArtPropertiesPreservesExplicitHorizontalTextFlow(t *testing.T) {
	// txflTextFlow=0 is an authored horizontal setting, not "property absent".
	props := []officeartescher.Property{{
		ID:    officeartescher.PropTextFlow,
		Value: 0,
	}}
	var out legacyOfficeArtShapeProps
	legacyApplyOfficeArtProperties(props, &out)

	if !out.HasTextVert {
		t.Fatal("explicit horizontal text flow lost")
	}
	if out.TextVert != "" {
		t.Fatalf("TextVert=%q want horizontal empty value", out.TextVert)
	}
}

func TestLegacyApplyOfficeArtPropertiesTransfersAutoFit(t *testing.T) {
	// OfficeArt text boolean: value bit0=1, use bit16=1 => fit shape to text.
	props := []officeartescher.Property{{
		ID:    officeartescher.PropTextBooleans,
		Value: (1 << 16) | 1,
	}}
	var out legacyOfficeArtShapeProps
	legacyApplyOfficeArtProperties(props, &out)

	if !out.HasTextAutoFit {
		t.Fatal("autofit presence lost")
	}
	if out.TextAutoFit != "shape" {
		t.Fatalf("TextAutoFit=%q want shape", out.TextAutoFit)
	}
}

func TestApplyLegacyShapeTextBoxPropertiesTransfersAutoFit(t *testing.T) {
	props := legacyOfficeArtShapeProps{
		HasTextBox:     true,
		HasTextAutoFit: true,
		TextAutoFit:    "shape",
	}
	tb := &wordir.TextBox{}
	applyLegacyShapeTextBoxProperties(tb, props)
	if tb.Autofit != "shape" {
		t.Fatalf("Autofit=%q want shape", tb.Autofit)
	}
}
