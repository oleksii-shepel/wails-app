package legacy

import (
	officeartir "office/pkg/officeart/ir"
	"office/pkg/presentation/ir"
	"testing"
)

func TestBIG39OfficeArtCustomGeometryCrossesPresentationPhysicalBoundary(t *testing.T) {
	shared := &officeartir.Shape{Geometry: officeartir.Geometry{
		Kind:        officeartir.GeometryCustom,
		Adjustments: []float64{12500, 75000},
		Paths: []officeartir.Path{{FillRule: "evenodd", Fill: true, Stroke: true, Commands: []officeartir.PathCommand{
			{Op: "M", Points: []officeartir.PointF{{X: 0.1, Y: 0.2}}},
			{Op: "L", Points: []officeartir.PointF{{X: 0.9, Y: 0.2}, {X: 0.5, Y: 0.8}}},
			{Op: "Z"},
		}}},
	}}
	shape := &ir.Shape{Geometry: "rect"}
	applyLegacyPptOfficeArtGeometry(shape, shared)
	if shape.Geometry != "" || shape.GeometryData == nil {
		t.Fatalf("custom geometry projection = Geometry %q Data %#v", shape.Geometry, shape.GeometryData)
	}
	if len(shape.GeometryData.Paths) != 1 || shape.GeometryData.Paths[0].FillRule != "evenodd" {
		t.Fatalf("custom paths = %#v", shape.GeometryData.Paths)
	}
	commands := shape.GeometryData.Paths[0].Commands
	if len(commands) != 3 || commands[0].Type != "M" || commands[1].Type != "L" || commands[2].Type != "Z" {
		t.Fatalf("custom commands = %#v", commands)
	}
	if len(commands[1].Points) != 2 || commands[1].Points[1].X != .5 || commands[1].Points[1].Y != .8 {
		t.Fatalf("custom points = %#v", commands[1].Points)
	}
	if len(shape.GeometryAdjustments) != 2 || shape.GeometryAdjustments[0] != 12500 || shape.GeometryAdjustments[1] != 75000 {
		t.Fatalf("adjustments = %#v", shape.GeometryAdjustments)
	}
}

func TestBIG39OfficeArtPresetGeometryRemainsPreset(t *testing.T) {
	shape := &ir.Shape{}
	applyLegacyPptOfficeArtGeometry(shape, &officeartir.Shape{Geometry: officeartir.Geometry{Kind: officeartir.GeometryPreset, Preset: "roundRect"}})
	if shape.Geometry != "roundRect" || shape.GeometryData != nil {
		t.Fatalf("preset projection = Geometry %q Data %#v", shape.Geometry, shape.GeometryData)
	}
}

func TestLegacyPresetGeometryAdjustmentsNormalizeFromOfficeArtCoordinateSpace(t *testing.T) {
	shape := &ir.Shape{}
	applyLegacyPptOfficeArtGeometry(shape, &officeartir.Shape{Geometry: officeartir.Geometry{
		Kind: officeartir.GeometryPreset, Preset: "roundRect", Adjustments: []float64{10800, 5400, 21600},
	}})
	want := []float64{0.5, 0.25, 1}
	if len(shape.GeometryAdjustments) != len(want) {
		t.Fatalf("adjustments = %#v", shape.GeometryAdjustments)
	}
	for i := range want {
		if shape.GeometryAdjustments[i] != want[i] {
			t.Fatalf("adjustment[%d] = %v, want %v", i, shape.GeometryAdjustments[i], want[i])
		}
	}
}

func TestLegacyArcAngularAdjustmentsNormalizeFromFixedPointDegrees(t *testing.T) {
	shape := &ir.Shape{}
	applyLegacyPptOfficeArtGeometry(shape, &officeartir.Shape{Geometry: officeartir.Geometry{
		Kind: officeartir.GeometryPreset, Preset: "arc", Adjustments: []float64{90 * 65536, 270 * 65536},
	}})
	want := []float64{0.25, 0.75}
	for i := range want {
		if shape.GeometryAdjustments[i] != want[i] {
			t.Fatalf("adjustment[%d] = %v, want %v", i, shape.GeometryAdjustments[i], want[i])
		}
	}
}

func TestLegacyBlockArcSeparatesAngularAndLinearAdjustmentUnits(t *testing.T) {
	shape := &ir.Shape{}
	applyLegacyPptOfficeArtGeometry(shape, &officeartir.Shape{Geometry: officeartir.Geometry{
		Kind: officeartir.GeometryPreset, Preset: "blockArc", Adjustments: []float64{180 * 65536, 90 * 65536, 5400},
	}})
	want := []float64{0.5, 0.25, 0.25}
	for i := range want {
		if shape.GeometryAdjustments[i] != want[i] {
			t.Fatalf("adjustment[%d] = %v, want %v", i, shape.GeometryAdjustments[i], want[i])
		}
	}
}

func TestLegacyArcFamilyAngularAdjustmentsUseFixedPointDegrees(t *testing.T) {
	for _, preset := range []string{"arc", "pie", "chord"} {
		got := normalizeLegacyPptPresetAdjustments(preset, []float64{90 * 65536, 270 * 65536})
		if len(got) != 2 || got[0] != 0.25 || got[1] != 0.75 {
			t.Fatalf("%s adjustments = %#v, want [0.25 0.75]", preset, got)
		}
	}
}

func TestLegacyRightArrowAdjustmentsMapOfficeArtHandlesToDrawingMLSemantics(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 288000, H: 288000}}
	applyLegacyPptOfficeArtGeometry(shape, &officeartir.Shape{Geometry: officeartir.Geometry{
		Kind:              officeartir.GeometryPreset,
		Preset:            "rightArrow",
		Adjustments:       []float64{8775, 6225},
		AdjustmentPresent: []bool{true, true},
	}})
	want := []float64{0.42361111111111116, 0.59375}
	for i := range want {
		if diff := shape.GeometryAdjustments[i] - want[i]; diff < -1e-12 || diff > 1e-12 {
			t.Fatalf("adjustment[%d] = %.12f, want %.12f", i, shape.GeometryAdjustments[i], want[i])
		}
	}
}

func TestLegacyRightArrowUsesOfficeArtDefaultForMissingHeadStartSlot(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 685800, H: 457200}}
	applyLegacyPptOfficeArtGeometry(shape, &officeartir.Shape{Geometry: officeartir.Geometry{
		Kind:              officeartir.GeometryPreset,
		Preset:            "rightArrow",
		Adjustments:       []float64{0, 6675},
		AdjustmentPresent: []bool{false, true},
	}})
	want := []float64{0.3819444444444444, 0.375}
	for i := range want {
		if diff := shape.GeometryAdjustments[i] - want[i]; diff < -1e-12 || diff > 1e-12 {
			t.Fatalf("adjustment[%d] = %.12f, want %.12f", i, shape.GeometryAdjustments[i], want[i])
		}
	}
}

func TestLegacyRightArrowDistinguishesAuthoredZeroFromMissingAdjustment(t *testing.T) {
	bounds := ir.Rect{W: 685800, H: 457200}
	missing := normalizeLegacyPptRightArrowAdjustments([]float64{0, 5400}, []bool{false, true}, bounds)
	authoredZero := normalizeLegacyPptRightArrowAdjustments([]float64{0, 5400}, []bool{true, true}, bounds)
	if missing[1] != 0.375 {
		t.Fatalf("missing head-start slot = %v, want OfficeArt default mapped to 0.375", missing[1])
	}
	if authoredZero[1] != 1.5 {
		t.Fatalf("authored zero head-start slot = %v, want full-width head mapped to 1.5", authoredZero[1])
	}
}

func TestLegacyRightArrowMaterializesEffectiveDefaultsForAspectRatio(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 685800, H: 457200}}
	applyLegacyPptOfficeArtGeometry(shape, &officeartir.Shape{Geometry: officeartir.Geometry{
		Kind:   officeartir.GeometryPreset,
		Preset: "rightArrow",
	}})
	want := []float64{0.5, 0.375}
	if len(shape.GeometryAdjustments) != 2 || shape.GeometryAdjustments[0] != want[0] || shape.GeometryAdjustments[1] != want[1] {
		t.Fatalf("default right-arrow adjustments = %#v, want %#v", shape.GeometryAdjustments, want)
	}
}
