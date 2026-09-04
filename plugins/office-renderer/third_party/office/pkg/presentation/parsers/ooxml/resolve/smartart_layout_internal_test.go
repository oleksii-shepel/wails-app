package resolve

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/smartart"
)

func TestSyntheticSmartArtAssistantConnectorUsesSideAnchors(t *testing.T) {
	src := &Shape{Bounds: Rect{X: 100, Y: 100, W: 200, H: 100}}
	dst := &Shape{Bounds: Rect{X: 400, Y: 110, W: 120, H: 80}}
	line := &drawing.LineStyle{Color: "999999", Width: 1}

	conn := buildSyntheticSmartArtConnector(src, dst, line, "hierarchy", "topDown", true)
	if conn == nil || conn.ConnectorStart == nil || conn.ConnectorEnd == nil {
		t.Fatal("assistant connector endpoints missing")
	}
	if conn.ConnectorStart.X != src.Bounds.X+src.Bounds.W {
		t.Fatalf("start X = %d, want parent right edge %d", conn.ConnectorStart.X, src.Bounds.X+src.Bounds.W)
	}
	if conn.ConnectorEnd.X != dst.Bounds.X {
		t.Fatalf("end X = %d, want assistant left edge %d", conn.ConnectorEnd.X, dst.Bounds.X)
	}
	if len(conn.ConnectorPath) != 4 {
		t.Fatalf("connector path points = %d, want 4", len(conn.ConnectorPath))
	}
}

func TestSyntheticSmartArtConnectorHonorsAuthoredRouting(t *testing.T) {
	line := &drawing.LineStyle{Color: "112233", Width: 2}
	src := &Shape{Bounds: Rect{X: 0, Y: 0, W: 1000, H: 600}}
	dst := &Shape{Bounds: Rect{X: 4000, Y: 2000, W: 1000, H: 600}}
	layout := &smartart.Layout{
		ConnectorRouting:    "bend",
		ConnectorBendPoint:  "beg",
		ConnectorBeginPts:   "midR",
		ConnectorEndPts:     "midL",
		ConnectorDimension:  "2D",
		ConnectorBeginStyle: "arr",
		ConnectorEndStyle:   "noArr",
	}
	conn := buildSyntheticSmartArtConnector(src, dst, line, "process", "", false, layout)
	if conn.ConnectorType != "elbow" || conn.ConnectorStart == nil || conn.ConnectorEnd == nil {
		t.Fatalf("expected routed elbow connector, got %+v", conn)
	}
	if conn.ConnectorStart.X != 1000 || conn.ConnectorStart.Y != 300 || conn.ConnectorEnd.X != 4000 || conn.ConnectorEnd.Y != 2300 {
		t.Fatalf("unexpected authored anchors: start=%+v end=%+v", conn.ConnectorStart, conn.ConnectorEnd)
	}
	if len(conn.ConnectorPath) != 4 || conn.ConnectorPath[1].X != 1750 {
		t.Fatalf("expected begin-weighted elbow path, got %+v", conn.ConnectorPath)
	}
	if conn.ArrowTail != "triangle" || conn.ArrowHead != "" {
		t.Fatalf("expected begSty/endSty override, tail=%q head=%q", conn.ArrowTail, conn.ArrowHead)
	}
}

func TestSyntheticSmartArtConnectorHonorsCurveAndOneDimension(t *testing.T) {
	line := &drawing.LineStyle{Color: "112233", Width: 1}
	src := &Shape{Bounds: Rect{X: 0, Y: 0, W: 1000, H: 600}}
	dst := &Shape{Bounds: Rect{X: 4000, Y: 0, W: 1000, H: 600}}
	curve := buildSyntheticSmartArtConnector(src, dst, line, "process", "", false, &smartart.Layout{ConnectorRouting: "longCurve"})
	if curve.ConnectorType != "longCurve" {
		t.Fatalf("expected longCurve, got %q", curve.ConnectorType)
	}
	oneD := buildSyntheticSmartArtConnector(src, dst, line, "process", "", false, &smartart.Layout{ConnectorRouting: "bend", ConnectorDimension: "1D"})
	if oneD.ConnectorType != "straight" {
		t.Fatalf("1D connector should force straight routing, got %q", oneD.ConnectorType)
	}
}
