package cascade

import (
	"office/pkg/presentation/parsers/ooxml/parts"
	"testing"
)

func TestEffectiveTextBodyStyleComposesMasterLayoutShape(t *testing.T) {
	masterAnchor := "ctr"
	layoutLeft := int64(200)
	shapeRight := int64(300)
	cols := 2
	spacing := int64(400)
	master := &parts.TextBody{BodyPr: &parts.DMLBodyPr{Anchor: masterAnchor}}
	layout := &parts.TextBody{BodyPr: &parts.DMLBodyPr{LIns: &layoutLeft}}
	shape := &parts.TextBody{BodyPr: &parts.DMLBodyPr{RIns: &shapeRight, NumCol: &cols, SpcCol: &spacing}}

	got := EffectiveTextBodyStyle(shape, layout, master)
	if got.Anchor != "ctr" || got.LIns != 200 || got.RIns != 300 {
		t.Fatalf("effective text body style = %+v", got)
	}
	if got.TIns != DefaultTextInsetTopBottomEMU || got.BIns != DefaultTextInsetTopBottomEMU {
		t.Fatalf("default vertical insets lost: %+v", got)
	}
	if got.Columns != 2 || got.ColumnSpacingEMU != 400 {
		t.Fatalf("column composition = %+v", got)
	}
}

func TestEffectiveTextBodyStyleShapeOverridesInheritedValues(t *testing.T) {
	masterLeft := int64(100)
	shapeLeft := int64(500)
	master := &parts.TextBody{BodyPr: &parts.DMLBodyPr{LIns: &masterLeft}}
	shape := &parts.TextBody{BodyPr: &parts.DMLBodyPr{LIns: &shapeLeft, SpAutoFit: &struct{}{}}}
	got := EffectiveTextBodyStyle(shape, nil, master)
	if got.LIns != 500 {
		t.Fatalf("left inset = %d, want 500", got.LIns)
	}
	if got.Autofit != "spAutoFit" {
		t.Fatalf("autofit = %q", got.Autofit)
	}
	if got.DrawingProps().AutofitType != "spAutoFit" {
		t.Fatalf("drawing props autofit = %q", got.DrawingProps().AutofitType)
	}
}
