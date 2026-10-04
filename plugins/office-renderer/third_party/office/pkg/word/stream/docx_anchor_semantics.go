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
	var width, height int64
	if anchor != nil && anchor.Extent != nil {
		width, height = anchor.Extent.CX, anchor.Extent.CY
	}
	return drawingAnchorWrapPolygonForExtent(anchor, width, height)
}

// drawingAnchorWrapPolygonForExtent accepts the resolved visual extent because
// some DrawingML payloads provide their size outside wp:extent. Word commonly
// encodes wrap polygons on a normalized 21600-by-21600 grid.
func drawingAnchorWrapPolygonForExtent(anchor *wordparts.DrawingAnchor, width, height int64) [][2]float64 {
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
	rawPoints := make([]wordparts.WrapPolygonPt, 0, 1+len(polygon.LineTo))
	rawPoints = append(rawPoints, polygon.Start)
	rawPoints = append(rawPoints, polygon.LineTo...)
	normalized := width > 0 && height > 0
	if normalized {
		for _, point := range rawPoints {
			if point.X < 0 || point.Y < 0 || point.X > 21600 || point.Y > 21600 {
				normalized = false
				break
			}
		}
	}
	points := make([][2]float64, 0, len(rawPoints))
	for _, point := range rawPoints {
		x, y := point.X, point.Y
		if normalized {
			x = x * width / 21600
			y = y * height / 21600
		}
		points = append(points, [2]float64{drawingEMUToPoints(x), drawingEMUToPoints(y)})
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
