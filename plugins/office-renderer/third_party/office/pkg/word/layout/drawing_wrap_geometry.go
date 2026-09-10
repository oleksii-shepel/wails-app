package layout

import "strings"

// polygonIntervalsAcrossBand is the compatibility projection of the canonical
// span engine. The old sampled implementation in layout.go can be removed once
// its remaining callers are cut over; this adapter already uses Line Layout
// 2.0's deterministic complete-band projection.
func polygonIntervalsAcrossBand(polygon [][2]float64, lineTop, lineBottom float64) [][2]float64 {
	spans := exactPolygonIntervalsAtBand(polygon, lineTop, lineBottom)
	intervals := make([][2]float64, 0, len(spans))
	for _, span := range spans {
		intervals = append(intervals, [2]float64{span.Left, span.Right})
	}
	return intervals
}

func polygonXRangeAcrossBand(polygon [][2]float64, lineTop, lineBottom float64) (minX, maxX float64, ok bool) {
	spans := exactPolygonIntervalsAtBand(polygon, lineTop, lineBottom)
	if len(spans) == 0 {
		return 0, 0, false
	}
	return spans[0].Left, spans[len(spans)-1].Right, true
}

func transformedDrawingBounds(fl FloatBox, rotationDeg float64) (left, top, right, bottom float64) {
	polygon := rotatedRectPolygon(fl.X, fl.Y, fl.Width, fl.Height, rotationDeg)
	if l, t, r, b, ok := polygonBounds(polygon); ok {
		return l, t, r, b
	}
	return fl.X, fl.Y, fl.X + fl.Width, fl.Y + fl.Height
}

func drawingWrapGeometry(fl FloatBox, wrapType string, wrapPolygon [][2]float64, rotationDeg, sourceWidthPt, sourceHeightPt float64) (left, top, right, bottom float64, contour [][2]float64) {
	left, top, right, bottom = transformedDrawingBounds(fl, rotationDeg)
	if !strings.EqualFold(wrapType, "tight") && !strings.EqualFold(wrapType, "through") {
		return left, top, right, bottom, nil
	}
	if len(wrapPolygon) > 0 {
		contour = resolvedWrapPolygon(fl, wrapPolygon, sourceWidthPt, sourceHeightPt, rotationDeg)
	} else {
		contour = rotatedRectPolygon(fl.X, fl.Y, fl.Width, fl.Height, rotationDeg)
	}
	if l, t, r, b, ok := polygonBounds(contour); ok {
		left, top, right, bottom = l, t, r, b
	}
	return left, top, right, bottom, contour
}

// drawingLineExclusion is the single Drawing Layout 2.0 boundary from
// transformed drawing geometry to Line Layout 2.0 exclusion geometry.
func drawingLineExclusion(
	fl FloatBox,
	wrapType, wrapSide string,
	wrapPolygon [][2]float64,
	rotationDeg float64,
	sourceWidthPt, sourceHeightPt float64,
	distTop, distRight, distBottom, distLeft float64,
	effectTop, effectRight, effectBottom, effectLeft float64,
) lineExclusion {
	padLeft := distLeft + effectLeft
	padRight := distRight + effectRight
	left, top, right, bottom, contour := drawingWrapGeometry(fl, wrapType, wrapPolygon, rotationDeg, sourceWidthPt, sourceHeightPt)
	return lineExclusion{
		top:       top - distTop - effectTop,
		bottom:    bottom + distBottom + effectBottom,
		left:      left - padLeft,
		right:     right + padRight,
		fullWidth: strings.EqualFold(wrapType, "topAndBottom"),
		wrapSide:  wrapSide,
		polygon:   contour,
		padLeft:   padLeft,
		padRight:  padRight,
	}
}
