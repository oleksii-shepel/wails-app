package legacy

import (
	"testing"

	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

func TestDecodeVisualProperties(t *testing.T) {
	props := []escher.Property{
		{ID: escher.PropRotation, Value: 45 * 65536},
		{ID: escher.PropFillColor, Value: 0x00332211},
		{ID: escher.PropLineWidth, Value: 25400},
		{ID: escher.PropCropLeft, Value: 16384},
		{ID: escher.PropTextFlow, Value: 1},
	}
	got := DecodeVisualProperties(props, nil)
	if !got.HasRotation || got.RotationDeg != 45 {
		t.Fatalf("rotation = %+v", got)
	}
	if got.FillColor != "112233" {
		t.Fatalf("fill = %q", got.FillColor)
	}
	if got.LineWidthEMU != 25400 {
		t.Fatalf("line width = %d", got.LineWidthEMU)
	}
	if got.Crop.Left != 0.25 {
		t.Fatalf("crop left = %g", got.Crop.Left)
	}
	if got.Text.Vert != ir.TextVerticalRL {
		t.Fatalf("text vert = %q", got.Text.Vert)
	}
}

func TestMergeVisualPropertiesPreservesExplicitZeroInsets(t *testing.T) {
	base := VisualProperties{Text: TextProperties{
		HasLeft: true,
		Insets:  ir.Insets{Left: 12700},
	}}
	incoming := VisualProperties{Text: TextProperties{
		HasLeft: true,
		Insets:  ir.Insets{Left: 0},
	}}
	got := MergeVisualProperties(base, incoming)
	if !got.Text.HasLeft || got.Text.Insets.Left != 0 {
		t.Fatalf("merged text = %+v", got.Text)
	}
}

func TestDecodeVisualPropertiesTracksZeroValuedFillAndCropPresence(t *testing.T) {
	got := DecodeVisualProperties([]escher.Property{
		{ID: escher.PropFillType, Value: 0},
		{ID: escher.PropCropTop, Value: 0},
		{ID: escher.PropCropRight, Value: 0},
	}, nil)

	if !got.HasFillType || got.FillType != 0 {
		t.Fatalf("fill type presence/value = (%v,%d), want (true,0)", got.HasFillType, got.FillType)
	}
	if !got.HasCrop || !got.HasCropTop || !got.HasCropRight {
		t.Fatalf("crop presence = %+v", got)
	}
	if got.HasCropBottom || got.HasCropLeft {
		t.Fatalf("unexpected crop edge presence = %+v", got)
	}
}

func TestMergeVisualPropertiesAllowsExplicitZeroFillTypeOverride(t *testing.T) {
	base := VisualProperties{HasFillType: true, FillType: 3}
	incoming := VisualProperties{HasFillType: true, FillType: 0}

	got := MergeVisualProperties(base, incoming)
	if !got.HasFillType || got.FillType != 0 {
		t.Fatalf("merged fill type = (%v,%d), want (true,0)", got.HasFillType, got.FillType)
	}
}

func TestMergeVisualPropertiesAllowsExplicitZeroCropOverride(t *testing.T) {
	base := VisualProperties{
		HasCrop:     true,
		HasCropTop:  true,
		HasCropLeft: true,
		Crop:        ir.Crop{Top: 0.25, Left: 0.50},
	}
	incoming := VisualProperties{
		HasCrop:    true,
		HasCropTop: true,
		Crop:       ir.Crop{Top: 0},
	}

	got := MergeVisualProperties(base, incoming)
	if !got.HasCropTop || got.Crop.Top != 0 {
		t.Fatalf("top crop = (%v,%g), want (true,0)", got.HasCropTop, got.Crop.Top)
	}
	if !got.HasCropLeft || got.Crop.Left != 0.50 {
		t.Fatalf("left crop = (%v,%g), want preserved 0.50", got.HasCropLeft, got.Crop.Left)
	}
}

func TestDecodeVisualPropertiesTracksExplicitNoneArrow(t *testing.T) {
	got := DecodeVisualProperties([]escher.Property{
		{ID: escher.PropLineStartArrow, Value: 0},
		{ID: escher.PropLineEndArrow, Value: 1},
	}, nil)

	if !got.HasStartArrow || got.StartArrow != ArrowNone {
		t.Fatalf("start arrow = presence:%v kind:%q, want explicit none", got.HasStartArrow, got.StartArrow)
	}
	if !got.HasEndArrow || got.EndArrow != ArrowTriangle {
		t.Fatalf("end arrow = presence:%v kind:%q, want triangle", got.HasEndArrow, got.EndArrow)
	}
}

func TestMergeVisualPropertiesAllowsExplicitArrowRemoval(t *testing.T) {
	base := VisualProperties{HasStartArrow: true, StartArrow: ArrowTriangle}
	incoming := VisualProperties{HasStartArrow: true, StartArrow: ArrowNone}

	got := MergeVisualProperties(base, incoming)
	if !got.HasStartArrow || got.StartArrow != ArrowNone {
		t.Fatalf("merged start arrow = presence:%v kind:%q, want explicit none", got.HasStartArrow, got.StartArrow)
	}
}

func TestDecodeVisualPropertiesPreservesExplicitZeroLineOpacity(t *testing.T) {
	props := []escher.Property{{ID: escher.PropLineColor, Value: 0x000000FF}, {ID: escher.PropLineOpacity, Value: 0}}
	got := DecodeVisualProperties(props, nil)
	if !got.HasLineOpacity || got.LineOpacity != 0 {
		t.Fatalf("line opacity = present:%v value:%v", got.HasLineOpacity, got.LineOpacity)
	}
}

func TestMergeVisualPropertiesPreservesZeroLineOpacity(t *testing.T) {
	base := VisualProperties{HasLineOpacity: true, LineOpacity: 1}
	base = MergeVisualProperties(base, VisualProperties{HasLineOpacity: true, LineOpacity: 0})
	if !base.HasLineOpacity || base.LineOpacity != 0 {
		t.Fatalf("merged line opacity = present:%v value:%v", base.HasLineOpacity, base.LineOpacity)
	}
}

func TestDecodeVisualPropertiesUnrelatedFillBooleanDoesNotCancelEarlierFilled(t *testing.T) {
	props := []escher.Property{
		// Explicit fFilled=true: value bit 4 + use bit 4.
		{ID: escher.PropFillBooleans, Value: (1 << 20) | (1 << 4)},
		// Later fill boolean record authors unrelated use bits only.
		{ID: escher.PropFillBooleans, Value: 0x00600000},
	}
	got := DecodeVisualProperties(props, nil)
	if !got.HasFill || !got.FillEnabled {
		t.Fatalf("fill = present:%v enabled:%v, want preserved explicit fFilled=true", got.HasFill, got.FillEnabled)
	}
}

func TestDecodeVisualPropertiesTracksFillOpacity(t *testing.T) {
	got := DecodeVisualProperties([]escher.Property{{ID: escher.PropFillOpacity, Value: 0x00004CCD}}, nil)
	if !got.HasFillOpacity || got.FillOpacity < 0.299 || got.FillOpacity > 0.301 {
		t.Fatalf("fill opacity = present:%v value:%v, want about 0.30", got.HasFillOpacity, got.FillOpacity)
	}
}
