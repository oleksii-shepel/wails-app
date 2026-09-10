package stream

import (
	wordir "office/pkg/word/ir"
	wordparts "office/pkg/word/parsers/ooxml/parts"
)

// drawingPlacementFromAnchor is the single DOCX-anchor normalization boundary.
func drawingPlacementFromAnchor(anchor *wordparts.DrawingAnchor) wordir.DrawingPlacement {
	if anchor == nil {
		return wordir.DrawingPlacement{}
	}
	placement := wordir.DrawingPlacement{
		Floating:     true,
		ZIndex:       anchor.RelativeHeight,
		BehindDoc:    anchor.BehindDoc != 0,
		Locked:       anchor.Locked,
		LayoutInCell: anchor.LayoutInCell,
		AllowOverlap: anchor.AllowOverlap,
	}
	if anchor.SimplePosAttr && anchor.SimplePos != nil {
		placement.SimplePosAttr = true
		placement.SimplePosX = anchor.SimplePos.X
		placement.SimplePosY = anchor.SimplePos.Y
	}
	if anchor.EffectExtent != nil {
		placement.EffectExtentL = anchor.EffectExtent.L
		placement.EffectExtentT = anchor.EffectExtent.T
		placement.EffectExtentR = anchor.EffectExtent.R
		placement.EffectExtentB = anchor.EffectExtent.B
	}
	if anchor.SizeRelH != nil {
		placement.SizeRelHPct = anchor.SizeRelH.Pct
		placement.SizeRelHFrom = anchor.SizeRelH.RelativeFrom
	}
	if anchor.SizeRelV != nil {
		placement.SizeRelVPct = anchor.SizeRelV.Pct
		placement.SizeRelVFrom = anchor.SizeRelV.RelativeFrom
	}
	placement.WrapType, placement.WrapSide = drawingAnchorWrapMode(anchor)
	placement.WrapPolygon = drawingAnchorWrapPolygon(anchor)
	distT, distB, distL, distR := drawingAnchorWrapDistances(anchor)
	placement.DistTop = drawingEMUToPoints(distT)
	placement.DistBottom = drawingEMUToPoints(distB)
	placement.DistLeft = drawingEMUToPoints(distL)
	placement.DistRight = drawingEMUToPoints(distR)
	return placement
}

func drawingAnchorWrapMode(anchor *wordparts.DrawingAnchor) (wrapType, wrapSide string) {
	if anchor == nil {
		return "", ""
	}
	switch {
	case anchor.WrapNone != nil:
		return "none", ""
	case anchor.WrapSquare != nil:
		return "square", anchor.WrapSquare.WrapText
	case anchor.WrapTight != nil:
		return "tight", anchor.WrapTight.WrapText
	case anchor.WrapThrough != nil:
		return "through", anchor.WrapThrough.WrapText
	case anchor.WrapTopAndBottom != nil:
		return "topAndBottom", ""
	default:
		return "", ""
	}
}

func drawingAnchorWrapDistances(anchor *wordparts.DrawingAnchor) (distT, distB, distL, distR int64) {
	if anchor == nil {
		return 0, 0, 0, 0
	}
	switch {
	case anchor.WrapSquare != nil:
		return firstNonZeroDrawingDistance(anchor.WrapSquare.DistT, anchor.DistT), firstNonZeroDrawingDistance(anchor.WrapSquare.DistB, anchor.DistB), firstNonZeroDrawingDistance(anchor.WrapSquare.DistL, anchor.DistL), firstNonZeroDrawingDistance(anchor.WrapSquare.DistR, anchor.DistR)
	case anchor.WrapTight != nil:
		return firstNonZeroDrawingDistance(anchor.WrapTight.DistT, anchor.DistT), firstNonZeroDrawingDistance(anchor.WrapTight.DistB, anchor.DistB), firstNonZeroDrawingDistance(anchor.WrapTight.DistL, anchor.DistL), firstNonZeroDrawingDistance(anchor.WrapTight.DistR, anchor.DistR)
	case anchor.WrapThrough != nil:
		return firstNonZeroDrawingDistance(anchor.WrapThrough.DistT, anchor.DistT), firstNonZeroDrawingDistance(anchor.WrapThrough.DistB, anchor.DistB), firstNonZeroDrawingDistance(anchor.WrapThrough.DistL, anchor.DistL), firstNonZeroDrawingDistance(anchor.WrapThrough.DistR, anchor.DistR)
	case anchor.WrapTopAndBottom != nil:
		return firstNonZeroDrawingDistance(anchor.WrapTopAndBottom.DistT, anchor.DistT), firstNonZeroDrawingDistance(anchor.WrapTopAndBottom.DistB, anchor.DistB), anchor.DistL, anchor.DistR
	default:
		return anchor.DistT, anchor.DistB, anchor.DistL, anchor.DistR
	}
}

func drawingAnchorWrapPolygon(anchor *wordparts.DrawingAnchor) [][2]float64 {
	if anchor == nil {
		return nil
	}
	var polygon *wordparts.WrapPolygon
	switch {
	case anchor.WrapTight != nil:
		polygon = anchor.WrapTight.Polygon
	case anchor.WrapThrough != nil:
		polygon = anchor.WrapThrough.Polygon
	}
	if polygon == nil {
		return nil
	}
	points := make([][2]float64, 0, 1+len(polygon.LineTo))
	points = append(points, [2]float64{drawingEMUToPoints(polygon.Start.X), drawingEMUToPoints(polygon.Start.Y)})
	for _, point := range polygon.LineTo {
		points = append(points, [2]float64{drawingEMUToPoints(point.X), drawingEMUToPoints(point.Y)})
	}
	return points
}

func firstNonZeroDrawingDistance(primary, fallback int64) int64 {
	if primary != 0 {
		return primary
	}
	return fallback
}

func drawingEMUToPoints(value int64) float64 { return float64(value) / 12700.0 }
