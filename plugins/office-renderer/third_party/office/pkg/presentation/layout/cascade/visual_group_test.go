package cascade

import (
	"testing"

	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestGroupChildRectMapsChildCoordinateSpace(t *testing.T) {
	group := &parts.Transform2D{}
	group.Off.X, group.Off.Y = 1000, 2000
	group.Ext.CX, group.Ext.CY = 4000, 6000
	group.ChOff.X, group.ChOff.Y = 100, 200
	group.ChExt.CX, group.ChExt.CY = 2000, 3000

	got := GroupChildRect(ir.Rect{X: 600, Y: 700, W: 500, H: 250}, group)
	want := ir.Rect{X: 2000, Y: 3000, W: 1000, H: 500}
	if got != want {
		t.Fatalf("group child rect = %+v, want %+v", got, want)
	}
}

func TestApplyGroupTransformComposesRotationFlipsAndConnectorPoints(t *testing.T) {
	group := &parts.Transform2D{Rot: 90 * 60000, FlipH: true}
	group.Off.X, group.Off.Y = 100, 200
	group.Ext.CX, group.Ext.CY = 1000, 1000
	group.ChExt.CX, group.ChExt.CY = 1000, 1000

	shape := &ir.Shape{
		Bounds:         ir.Rect{X: 10, Y: 20, W: 30, H: 40},
		RotationDeg:    15,
		FlipH:          true,
		ConnectorStart: &ir.Point{X: 10, Y: 20},
		Children:       []*ir.Shape{{Bounds: ir.Rect{X: 1, Y: 2, W: 3, H: 4}}},
	}
	ApplyGroupTransform(shape, group)

	if shape.Bounds.X != 1045 || shape.Bounds.Y != 1155 || shape.RotationDeg != 75 || shape.FlipH {
		t.Fatalf("unexpected composed shape: %+v", shape)
	}
	if shape.ConnectorStart == nil || shape.ConnectorStart.X != 1080 || shape.ConnectorStart.Y != 1190 {
		t.Fatalf("connector start = %+v", shape.ConnectorStart)
	}
	if got := shape.Children[0].Bounds; got.X != 1095 || got.Y != 1196 {
		t.Fatalf("descendant bounds = %+v", got)
	}
}

func TestApplyGroupTransformReversesChildRotationUnderSingleAxisReflection(t *testing.T) {
	group := &parts.Transform2D{Rot: 90 * 60000, FlipH: true}
	group.Ext.CX, group.Ext.CY = 1000, 1000
	group.ChExt.CX, group.ChExt.CY = 1000, 1000
	shape := &ir.Shape{Bounds: ir.Rect{X: 100, Y: 100, W: 200, H: 100}, RotationDeg: 15}

	ApplyGroupTransform(shape, group)
	if shape.RotationDeg != 75 {
		t.Fatalf("rotation under reflected group = %v, want 75", shape.RotationDeg)
	}
}

func TestApplyGroupTransformKeepsChildRotationHandednessUnderDoubleReflection(t *testing.T) {
	group := &parts.Transform2D{Rot: 90 * 60000, FlipH: true, FlipV: true}
	group.Ext.CX, group.Ext.CY = 1000, 1000
	group.ChExt.CX, group.ChExt.CY = 1000, 1000
	shape := &ir.Shape{Bounds: ir.Rect{X: 100, Y: 100, W: 200, H: 100}, RotationDeg: 15}

	ApplyGroupTransform(shape, group)
	if shape.RotationDeg != 105 {
		t.Fatalf("rotation under double-reflected group = %v, want 105", shape.RotationDeg)
	}
}

func TestApplyGroupTransformMapsEntireConnectorPath(t *testing.T) {
	start := ir.Point{X: 10, Y: 20}
	end := ir.Point{X: 30, Y: 40}
	shape := &ir.Shape{
		Kind:           ir.ShapeConnector,
		Bounds:         ir.Rect{X: 10, Y: 20, W: 20, H: 20},
		ConnectorStart: &start,
		ConnectorEnd:   &end,
		ConnectorPath: []ir.Point{
			start,
			{X: 20, Y: 25},
			{X: 20, Y: 35},
			end,
		},
	}
	group := &parts.Transform2D{
		Off:   parts.Offset{X: 100, Y: 200},
		Ext:   parts.Extents{CX: 200, CY: 400},
		ChOff: parts.Offset{X: 10, Y: 20},
		ChExt: parts.Extents{CX: 40, CY: 40},
	}

	ApplyGroupTransform(shape, group)

	want := []ir.Point{
		{X: 100, Y: 200},
		{X: 150, Y: 250},
		{X: 150, Y: 350},
		{X: 200, Y: 400},
	}
	if len(shape.ConnectorPath) != len(want) {
		t.Fatalf("ConnectorPath len = %d, want %d", len(shape.ConnectorPath), len(want))
	}
	for index := range want {
		if shape.ConnectorPath[index] != want[index] {
			t.Fatalf("ConnectorPath[%d] = %+v, want %+v", index, shape.ConnectorPath[index], want[index])
		}
	}
	if shape.ConnectorStart == nil || *shape.ConnectorStart != want[0] {
		t.Fatalf("ConnectorStart = %#v, want %+v", shape.ConnectorStart, want[0])
	}
	if shape.ConnectorEnd == nil || *shape.ConnectorEnd != want[len(want)-1] {
		t.Fatalf("ConnectorEnd = %#v, want %+v", shape.ConnectorEnd, want[len(want)-1])
	}
}

func TestApplyGroupTransformMapsConnectorPathThroughRotationAndFlip(t *testing.T) {
	shape := &ir.Shape{
		Kind: ir.ShapeConnector,
		ConnectorPath: []ir.Point{
			{X: 100, Y: 100},
			{X: 200, Y: 100},
			{X: 200, Y: 200},
		},
	}
	group := &parts.Transform2D{
		Off:   parts.Offset{X: 0, Y: 0},
		Ext:   parts.Extents{CX: 1000, CY: 1000},
		ChOff: parts.Offset{X: 0, Y: 0},
		ChExt: parts.Extents{CX: 1000, CY: 1000},
		Rot:   5400000, // 90 degrees
		FlipH: true,
	}

	ApplyGroupTransform(shape, group)

	want := []ir.Point{
		{X: 900, Y: 900},
		{X: 900, Y: 800},
		{X: 800, Y: 800},
	}
	for index := range want {
		if shape.ConnectorPath[index] != want[index] {
			t.Fatalf("ConnectorPath[%d] = %+v, want %+v", index, shape.ConnectorPath[index], want[index])
		}
	}
}
