package drawing

import "math"

// ConnectorWaypoints returns the ordered waypoints (in points) for a connector shape.
//
// boundsX, boundsY, boundsW, boundsH are the bounding-box coordinates in points.
// connType is the DrawingML connector preset name (e.g. "straightConnector1",
// "bentConnector3"). adjustments holds the preset adjustment values in DrawingML
// units (0–100000 scale). flipH and flipV mirror the geometry horizontally/vertically.
func ConnectorWaypoints(connType string, boundsX, boundsY, boundsW, boundsH float64, adjustments []int, flipH, flipV bool) []Point {
	adj := func(idx int) int {
		if idx < len(adjustments) {
			return adjustments[idx]
		}
		return 0
	}
	switch connType {
	case "straightConnector1":
		return straightConnectorEndpoints(boundsX, boundsY, boundsW, boundsH, flipH, flipV)
	case "bentConnector2":
		return bentConnector2Waypoints(boundsX, boundsY, boundsW, boundsH, adj(0), flipH, flipV)
	case "bentConnector3":
		return bentConnector3Waypoints(boundsX, boundsY, boundsW, boundsH, adj(0), flipH, flipV)
	case "bentConnector4":
		return bentConnector4Waypoints(boundsX, boundsY, boundsW, boundsH, adj(0), adj(1), flipH, flipV)
	case "bentConnector5":
		return bentConnector5Waypoints(boundsX, boundsY, boundsW, boundsH, adj(0), adj(1), flipH, flipV)
	case "curvedConnector2":
		return curvedConnector2Waypoints(boundsX, boundsY, boundsW, boundsH, adj(0), flipH, flipV)
	case "curvedConnector3":
		return curvedConnector3Waypoints(boundsX, boundsY, boundsW, boundsH, adj(0), flipH, flipV)
	case "curvedConnector4":
		return curvedConnector4Waypoints(boundsX, boundsY, boundsW, boundsH, adj(0), adj(1), flipH, flipV)
	case "curvedConnector5":
		return curvedConnector5Waypoints(boundsX, boundsY, boundsW, boundsH, adj(0), adj(1), flipH, flipV)
	default:
		// Fall back to straight connector for unknown types
		return straightConnectorEndpoints(boundsX, boundsY, boundsW, boundsH, flipH, flipV)
	}
}

// bentConnector2Waypoints produces a 2-segment bent connector (L-shaped).
// The adjustment controls the position of the elbow.
func bentConnector2Waypoints(bx, by, bw, bh float64, adj1 int, flipH, flipV bool) []Point {
	// For bentConnector2, adj1 controls where the elbow is placed
	// 0-100000 maps to 0-100% along the path
	mx := bx + math.Round(bw*12700.0*float64(adj1)/100000.0)/12700.0
	my := by + math.Round(bh*12700.0*float64(adj1)/100000.0)/12700.0

	// Simple L-shaped connector: start → elbow → end
	pts := []Point{
		{X: bx, Y: by},
		{X: mx, Y: my},
		{X: bx + bw, Y: by + bh},
	}
	if flipH {
		for i := range pts {
			pts[i].X = bx + bw - (pts[i].X - bx)
		}
	}
	if flipV {
		for i := range pts {
			pts[i].Y = by + bh - (pts[i].Y - by)
		}
	}
	return compactWaypoints(pts)
}

// bentConnector3Waypoints produces the 4-point (or compacted) path for a
// bentConnector3: start → elbow-column → elbow-row → end.
func bentConnector3Waypoints(bx, by, bw, bh float64, adj1 int, flipH, flipV bool) []Point {
	mx := bx + math.Round(bw*12700.0*float64(adj1)/100000.0)/12700.0
	pts := []Point{
		{X: bx, Y: by},
		{X: mx, Y: by},
		{X: mx, Y: by + bh},
		{X: bx + bw, Y: by + bh},
	}
	if flipH {
		for i := range pts {
			pts[i].X = bx + bw - (pts[i].X - bx)
		}
	}
	if flipV {
		for i := range pts {
			pts[i].Y = by + bh - (pts[i].Y - by)
		}
	}
	return compactWaypoints(pts)
}

// bentConnector4Waypoints produces a 4-segment bent connector.
// adj1 controls the first horizontal position, adj2 controls the second.
func bentConnector4Waypoints(bx, by, bw, bh float64, adj1, adj2 int, flipH, flipV bool) []Point {
	a1 := adj1
	if a1 == 0 {
		a1 = 50000
	}
	a2 := adj2
	if a2 == 0 {
		a2 = 50000
	}
	mx1 := bx + math.Round(bw*12700.0*float64(a1)/100000.0)/12700.0
	mx2 := bx + math.Round(bw*12700.0*float64(a2)/100000.0)/12700.0
	my := by + bh/2

	pts := []Point{
		{X: bx, Y: by},
		{X: mx1, Y: by},
		{X: mx1, Y: my},
		{X: mx2, Y: my},
		{X: mx2, Y: by + bh},
		{X: bx + bw, Y: by + bh},
	}
	if flipH {
		for i := range pts {
			pts[i].X = bx + bw - (pts[i].X - bx)
		}
	}
	if flipV {
		for i := range pts {
			pts[i].Y = by + bh - (pts[i].Y - by)
		}
	}
	return compactWaypoints(pts)
}

// bentConnector5Waypoints produces a 5-segment bent connector.
// adj1 controls the first vertical position, adj2 controls the second.
func bentConnector5Waypoints(bx, by, bw, bh float64, adj1, adj2 int, flipH, flipV bool) []Point {
	a1 := adj1
	if a1 == 0 {
		a1 = 25000
	}
	a2 := adj2
	if a2 == 0 {
		a2 = 75000
	}
	my1 := by + math.Round(bh*12700.0*float64(a1)/100000.0)/12700.0
	my2 := by + math.Round(bh*12700.0*float64(a2)/100000.0)/12700.0
	mx1 := bx + bw/2

	pts := []Point{
		{X: bx, Y: by},
		{X: mx1, Y: by},
		{X: mx1, Y: my1},
		{X: bx + bw, Y: my1},
		{X: bx + bw, Y: my2},
		{X: mx1, Y: my2},
		{X: mx1, Y: by + bh},
		{X: bx + bw, Y: by + bh},
	}
	if flipH {
		for i := range pts {
			pts[i].X = bx + bw - (pts[i].X - bx)
		}
	}
	if flipV {
		for i := range pts {
			pts[i].Y = by + bh - (pts[i].Y - by)
		}
	}
	return compactWaypoints(pts)
}

// curvedConnector2Waypoints produces waypoints for a curved connector with 2 control points.
// This returns the endpoints and control points for a cubic Bézier curve.
func curvedConnector2Waypoints(bx, by, bw, bh float64, adj1 int, flipH, flipV bool) []Point {
	// For a curved connector with 2 control points (cubic Bézier)
	// We'll place control points at 1/3 and 2/3 of the bounding box
	cp1x := bx + bw/3
	cp1y := by + bh/3
	cp2x := bx + 2*bw/3
	cp2y := by + 2*bh/3

	pts := []Point{
		{X: bx, Y: by},           // Start point
		{X: cp1x, Y: cp1y},       // First control point
		{X: cp2x, Y: cp2y},       // Second control point
		{X: bx + bw, Y: by + bh}, // End point
	}
	if flipH {
		for i := range pts {
			pts[i].X = bx + bw - (pts[i].X - bx)
		}
	}
	if flipV {
		for i := range pts {
			pts[i].Y = by + bh - (pts[i].Y - by)
		}
	}
	return pts
}

// curvedConnector3Waypoints produces waypoints for a curved connector with 3 control points.
// This returns the endpoints and control points for a more complex curve.
func curvedConnector3Waypoints(bx, by, bw, bh float64, adj1 int, flipH, flipV bool) []Point {
	// For a curved connector with 3 control points
	// We'll create a more complex curve with multiple segments
	cp1x := bx + bw/4
	cp1y := by + bh/4
	cp2x := bx + bw/2
	cp2y := by + bh/2
	cp3x := bx + 3*bw/4
	cp3y := by + 3*bh/4

	pts := []Point{
		{X: bx, Y: by},           // Start point
		{X: cp1x, Y: cp1y},       // First control point
		{X: cp2x, Y: cp2y},       // Second control point
		{X: cp3x, Y: cp3y},       // Third control point
		{X: bx + bw, Y: by + bh}, // End point
	}
	if flipH {
		for i := range pts {
			pts[i].X = bx + bw - (pts[i].X - bx)
		}
	}
	if flipV {
		for i := range pts {
			pts[i].Y = by + bh - (pts[i].Y - by)
		}
	}
	return pts
}

// curvedConnector4Waypoints produces waypoints for a curved connector with 4 control points.
// adj1 and adj2 control the curvature of the first and second halves.
func curvedConnector4Waypoints(bx, by, bw, bh float64, adj1, adj2 int, flipH, flipV bool) []Point {
	a1 := adj1
	if a1 == 0 {
		a1 = 50000
	}
	a2 := adj2
	if a2 == 0 {
		a2 = 50000
	}
	t1 := float64(a1) / 100000.0
	t2 := float64(a2) / 100000.0

	cp1x := bx + bw*t1*0.5
	cp1y := by + bh*t1*0.5
	cp2x := bx + bw*(0.5+t2*0.3)
	cp2y := by + bh*(0.5-t2*0.3)
	cp3x := bx + bw*(0.7+t1*0.2)
	cp3y := by + bh*(0.7-t1*0.2)

	pts := []Point{
		{X: bx, Y: by},           // Start point
		{X: cp1x, Y: cp1y},       // First control point
		{X: cp2x, Y: cp2y},       // Second control point
		{X: cp3x, Y: cp3y},       // Third control point
		{X: bx + bw, Y: by + bh}, // End point
	}
	if flipH {
		for i := range pts {
			pts[i].X = bx + bw - (pts[i].X - bx)
		}
	}
	if flipV {
		for i := range pts {
			pts[i].Y = by + bh - (pts[i].Y - by)
		}
	}
	return pts
}

// curvedConnector5Waypoints produces waypoints for a curved connector with 5 control points.
// adj1 and adj2 control the curvature of the first and second halves.
func curvedConnector5Waypoints(bx, by, bw, bh float64, adj1, adj2 int, flipH, flipV bool) []Point {
	a1 := adj1
	if a1 == 0 {
		a1 = 50000
	}
	a2 := adj2
	if a2 == 0 {
		a2 = 50000
	}
	t1 := float64(a1) / 100000.0
	t2 := float64(a2) / 100000.0

	cp1x := bx + bw*t1*0.4
	cp1y := by + bh*t1*0.4
	cp2x := bx + bw*(0.3+t2*0.2)
	cp2y := by + bh*(0.3-t2*0.2)
	cp3x := bx + bw*0.5
	cp3y := by + bh*0.5
	cp4x := bx + bw*(0.7+t1*0.2)
	cp4y := by + bh*(0.7-t1*0.2)

	pts := []Point{
		{X: bx, Y: by},           // Start point
		{X: cp1x, Y: cp1y},       // First control point
		{X: cp2x, Y: cp2y},       // Second control point
		{X: cp3x, Y: cp3y},       // Third control point
		{X: cp4x, Y: cp4y},       // Fourth control point
		{X: bx + bw, Y: by + bh}, // End point
	}
	if flipH {
		for i := range pts {
			pts[i].X = bx + bw - (pts[i].X - bx)
		}
	}
	if flipV {
		for i := range pts {
			pts[i].Y = by + bh - (pts[i].Y - by)
		}
	}
	return pts
}

// straightConnectorEndpoints returns the two endpoints of a straight connector.
func straightConnectorEndpoints(bx, by, bw, bh float64, flipH, flipV bool) []Point {
	x1, x2 := bx, bx+bw
	y1, y2 := by, by+bh
	if flipH {
		x1, x2 = x2, x1
	}
	if flipV {
		y1, y2 = y2, y1
	}
	return []Point{{X: x1, Y: y1}, {X: x2, Y: y2}}
}

// compactWaypoints removes consecutive duplicate points.
func compactWaypoints(pts []Point) []Point {
	if len(pts) == 0 {
		return nil
	}
	out := pts[:1]
	for _, p := range pts[1:] {
		prev := out[len(out)-1]
		if p.X == prev.X && p.Y == prev.Y {
			continue
		}
		out = append(out, p)
	}
	return out
}

// ShortenWaypointsForArrows returns a copy of points with the start moved
// toward its neighbour by headTrim, and the end moved toward its neighbour
// by tailTrim. Non-positive trim values are ignored.
func ShortenWaypointsForArrows(pts []Point, headTrim, tailTrim float64) []Point {
	if len(pts) < 2 {
		return pts
	}
	out := make([]Point, len(pts))
	copy(out, pts)
	if headTrim > 0 {
		out[0] = moveTowardPt(out[0], out[1], headTrim)
	}
	if tailTrim > 0 {
		last := len(out) - 1
		out[last] = moveTowardPt(out[last], out[last-1], tailTrim)
	}
	return out
}

func moveTowardPt(from, to Point, dist float64) Point {
	dx := to.X - from.X
	dy := to.Y - from.Y
	length := math.Hypot(dx, dy)
	if length == 0 {
		return from
	}
	if dist > length {
		dist = length
	}
	s := dist / length
	return Point{X: from.X + dx*s, Y: from.Y + dy*s}
}

// ArrowDimensions returns the (lengthPt, widthPt) for an arrowhead in points.
// arrowType is e.g. "triangle", "stealth", "arrow", "diamond", "oval".
// widthSize and lengthSize are "sm", "med" (or ""), or "lg".
func ArrowDimensions(arrowType, widthSize, lengthSize string) (lengthPt, widthPt float64) {
	return ArrowDimensionsForStroke(arrowType, widthSize, lengthSize, 1)
}

// ArrowDimensionsForStroke resolves Office arrow size enums against the
// effective line width. Endpoint sizes are relative to the owning stroke;
// treating sm/med/lg as absolute point sizes makes thin lines grow giant
// markers and thick lines grow undersized markers.
func ArrowDimensionsForStroke(arrowType, widthSize, lengthSize string, strokeWidthPt float64) (lengthPt, widthPt float64) {
	if strokeWidthPt <= 0 {
		strokeWidthPt = 1
	}
	lengthPt = mapArrowLengthPt(lengthSize) * strokeWidthPt
	widthPt = mapArrowWidthPt(widthSize) * strokeWidthPt
	switch arrowType {
	case "diamond":
		return lengthPt / 2, widthPt
	default:
		return lengthPt, widthPt
	}
}

func mapArrowLengthPt(size string) float64 {
	switch size {
	case "sm":
		return 6
	case "lg":
		return 12
	default:
		return 8
	}
}

func mapArrowWidthPt(size string) float64 {
	switch size {
	case "sm":
		return 4.5
	case "lg":
		return 8
	default:
		return 5.5
	}
}

// supportedArrowMarkerType reports whether an Office arrow type has canonical
// geometry in this package. Keep shaft trimming and marker materialization on
// the same contract: an unsupported marker must never create an invisible gap.
func supportedArrowMarkerType(arrowType string) bool {
	switch arrowType {
	case "triangle", "stealth", "arrow", "diamond", "oval":
		return true
	default:
		return false
	}
}

// ArrowTrimDistance returns the legacy 1pt-stroke trim for callers that do
// not yet carry an effective stroke width.
func ArrowTrimDistance(arrowType, lengthSize string) float64 {
	return ArrowTrimDistanceForStroke(arrowType, lengthSize, 1)
}

// ArrowTrimDistanceForStroke returns how far the connector shaft should be
// pulled back from the authored endpoint for the effective stroke width.
func ArrowTrimDistanceForStroke(arrowType, lengthSize string, strokeWidthPt float64) float64 {
	if !supportedArrowMarkerType(arrowType) {
		return 0
	}
	length, _ := ArrowDimensionsForStroke(arrowType, "", lengthSize, strokeWidthPt)
	switch arrowType {
	case "diamond":
		return length * 2
	case "oval":
		return length
	case "arrow":
		return 0
	default:
		return length
	}
}

// ArrowGeometry holds the precomputed geometry for one arrowhead at a connector endpoint.
// All coordinates are in points.
type ArrowGeometry struct {
	// Type is the arrowhead type string ("triangle", "stealth", "arrow", "diamond", "oval").
	Type string
	// Tip is the outermost point of the arrowhead.
	Tip Point
	// PolyPts holds the polygon vertices for filled arrowheads (triangle, stealth, diamond).
	// For "triangle": [tip, left, right]
	// For "stealth":  [tip, left, notch, right]
	// For "diamond":  [tip, left, back, right]
	PolyPts []Point
	// For "arrow" (open outline): Left and Right are the two wing tips.
	Left, Right Point
	// StrokeWidth is the open-arrow stroke width in points.
	StrokeWidth float64
	// For "oval": Center, RX, RY describe the ellipse.
	Center Point
	RX, RY float64
	// LengthPt and WidthPt are the computed dimensions.
	LengthPt, WidthPt float64
}

// ComputeArrowGeometry returns the ArrowGeometry for an arrowhead at tip,
// pointing away from neighbor. Returns nil when arrowType is empty or "none".
func ComputeArrowGeometry(arrowType, widthSize, lengthSize string, tip, neighbor Point) *ArrowGeometry {
	return ComputeArrowGeometryForStroke(arrowType, widthSize, lengthSize, 1, tip, neighbor)
}

// ComputeArrowGeometryForStroke materializes an endpoint marker using the
// effective connector stroke width.
func ComputeArrowGeometryForStroke(arrowType, widthSize, lengthSize string, strokeWidthPt float64, tip, neighbor Point) *ArrowGeometry {
	if !supportedArrowMarkerType(arrowType) {
		return nil
	}
	length, width := ArrowDimensionsForStroke(arrowType, widthSize, lengthSize, strokeWidthPt)

	dx := tip.X - neighbor.X
	dy := tip.Y - neighbor.Y
	mag := math.Hypot(dx, dy)
	if mag == 0 {
		return nil
	}
	ux, uy := dx/mag, dy/mag // unit vector tip←neighbor (direction the tip points)
	px, py := -uy, ux        // perpendicular

	baseCenter := Point{X: tip.X - ux*length, Y: tip.Y - uy*length}
	half := width / 2
	left := Point{X: baseCenter.X + px*half, Y: baseCenter.Y + py*half}
	right := Point{X: baseCenter.X - px*half, Y: baseCenter.Y - py*half}

	ag := &ArrowGeometry{
		Type:     arrowType,
		Tip:      tip,
		LengthPt: length,
		WidthPt:  width,
	}

	switch arrowType {
	case "triangle":
		ag.PolyPts = []Point{tip, left, right}
	case "stealth":
		notch := Point{X: tip.X - ux*(length*0.55), Y: tip.Y - uy*(length*0.55)}
		ag.PolyPts = []Point{tip, left, notch, right}
	case "arrow":
		ag.Left, ag.Right = left, right
		ag.StrokeWidth = math.Max(1.5*strokeWidthPt, 0.5)
	case "diamond":
		back := Point{X: tip.X - ux*(length*2), Y: tip.Y - uy*(length*2)}
		ag.PolyPts = []Point{tip, left, back, right}
	case "oval":
		ag.Center = Point{X: tip.X - ux*(length/2), Y: tip.Y - uy*(length/2)}
		ag.RX = length / 2
		ag.RY = width / 2
	}
	return ag
}
