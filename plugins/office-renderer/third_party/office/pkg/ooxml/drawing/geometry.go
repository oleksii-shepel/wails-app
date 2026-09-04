package drawing

import (
	"math"
)

// Point represents a 2D point with X,Y coordinates.
type Point struct {
	X, Y float64
}

// Rect represents a rectangle with position and size.
type Rect struct {
	X, Y, Width, Height float64
}

// Transform represents a 2D affine transformation matrix.
// The matrix is stored in row-major order:
// [a c e]
// [b d f]
// [0 0 1]
type Transform struct {
	A, B, C, D, E, F float64
}

// Identity returns the identity transformation.
func Identity() Transform {
	return Transform{A: 1, D: 1}
}

// Translate creates a translation transformation.
func Translate(tx, ty float64) Transform {
	return Transform{A: 1, D: 1, E: tx, F: ty}
}

// Scale creates a scaling transformation.
func Scale(sx, sy float64) Transform {
	return Transform{A: sx, D: sy}
}

// Rotate creates a rotation transformation (angle in degrees).
func Rotate(angle float64) Transform {
	rad := angle * math.Pi / 180
	cos := math.Cos(rad)
	sin := math.Sin(rad)
	return Transform{A: cos, B: sin, C: -sin, D: cos}
}

// Multiply returns the product t1 Г— t2 (apply t2 then t1).
func (t1 Transform) Multiply(t2 Transform) Transform {
	return Transform{
		A: t1.A*t2.A + t1.C*t2.B,
		B: t1.B*t2.A + t1.D*t2.B,
		C: t1.A*t2.C + t1.C*t2.D,
		D: t1.B*t2.C + t1.D*t2.D,
		E: t1.A*t2.E + t1.C*t2.F + t1.E,
		F: t1.B*t2.E + t1.D*t2.F + t1.F,
	}
}

// Apply applies the transformation to a point.
func (t Transform) Apply(p Point) Point {
	return Point{
		X: t.A*p.X + t.C*p.Y + t.E,
		Y: t.B*p.X + t.D*p.Y + t.F,
	}
}

// ApplyRect applies the transformation to a rectangle, returning the bounding box
// of the transformed rectangle (which may be rotated).
func (t Transform) ApplyRect(r Rect) Rect {
	// Transform all four corners
	corners := []Point{
		{X: r.X, Y: r.Y},
		{X: r.X + r.Width, Y: r.Y},
		{X: r.X, Y: r.Y + r.Height},
		{X: r.X + r.Width, Y: r.Y + r.Height},
	}

	transformed := make([]Point, 4)
	for i, p := range corners {
		transformed[i] = t.Apply(p)
	}

	// Find bounding box
	minX, minY := transformed[0].X, transformed[0].Y
	maxX, maxY := minX, minY

	for _, p := range transformed[1:] {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}

	return Rect{
		X:      minX,
		Y:      minY,
		Width:  maxX - minX,
		Height: maxY - minY,
	}
}

// wedgeCalloutPath creates a wedge-shaped callout (speech bubble with wedge).
func (pg *PresetGeometry) wedgeCalloutPath(width, height float64) Path {
	// Simple rectangle with a wedge on the left side
	wedgeSize := min(width, height) * 0.2
	return Path{
		Commands: []PathCommand{
			{Type: "M", Points: []Point{{X: wedgeSize, Y: 0}}},
			{Type: "L", Points: []Point{{X: width, Y: 0}}},
			{Type: "L", Points: []Point{{X: width, Y: height}}},
			{Type: "L", Points: []Point{{X: wedgeSize, Y: height}}},
			{Type: "L", Points: []Point{{X: 0, Y: height / 2}}},
			{Type: "Z"},
		},
		FillRule: "nonzero",
	}
}

// cloudCalloutPath creates a cloud-shaped callout (cloud perimeter + speech-bubble tail).
func (pg *PresetGeometry) cloudCalloutPath(width, height float64) Path {
	// Build the same cloud perimeter as cloudPath, then append a triangular tail
	// pointing from the lower-left area of the cloud toward a callout anchor.
	rx, ry := width/2, height/2
	cx, cy := width/2, height/2
	smallRx := rx * 0.4
	smallRy := ry * 0.4

	// Cloud perimeter (same arcs as cloudPath).
	cmds := []PathCommand{
		{Type: "M", Points: []Point{{X: cx - rx*0.7, Y: cy}}},
		{Type: "A", Points: []Point{{X: cx - smallRx, Y: cy - ry*0.7}}, Args: []float64{smallRx, smallRy, 0, 0, 1}},
		{Type: "A", Points: []Point{{X: cx, Y: cy - ry}}, Args: []float64{rx * 0.6, ry * 0.6, 0, 0, 1}},
		{Type: "A", Points: []Point{{X: cx + smallRx, Y: cy - ry*0.7}}, Args: []float64{smallRx, smallRy, 0, 0, 1}},
		{Type: "A", Points: []Point{{X: cx + rx*0.7, Y: cy}}, Args: []float64{rx * 0.6, ry * 0.6, 0, 0, 1}},
		{Type: "A", Points: []Point{{X: cx + smallRx, Y: cy + ry*0.7}}, Args: []float64{smallRx, smallRy, 0, 0, 1}},
		{Type: "A", Points: []Point{{X: cx, Y: cy + ry}}, Args: []float64{rx * 0.6, ry * 0.6, 0, 0, 1}},
		{Type: "A", Points: []Point{{X: cx - smallRx, Y: cy + ry*0.7}}, Args: []float64{smallRx, smallRy, 0, 0, 1}},
		{Type: "A", Points: []Point{{X: cx - rx*0.7, Y: cy}}, Args: []float64{rx * 0.6, ry * 0.6, 0, 0, 1}},
		{Type: "Z"},
		// Callout tail: a narrow triangle pointing to the lower-left outside the cloud.
		{Type: "M", Points: []Point{{X: cx - rx*0.55, Y: cy + ry*0.55}}},
		{Type: "L", Points: []Point{{X: cx - rx*0.35, Y: cy + ry*0.75}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "Z"},
	}
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// cloudPath creates a standalone cloud shape (no callout tail).
func (pg *PresetGeometry) cloudPath(width, height float64) Path {
	rx, ry := width/2, height/2
	cx, cy := width/2, height/2
	smallRx := rx * 0.4
	smallRy := ry * 0.4

	return Path{
		Commands: []PathCommand{
			{Type: "M", Points: []Point{{X: cx - rx*0.7, Y: cy}}},
			{Type: "A", Points: []Point{{X: cx - smallRx, Y: cy - ry*0.7}}, Args: []float64{smallRx, smallRy, 0, 0, 1}},
			{Type: "A", Points: []Point{{X: cx, Y: cy - ry}}, Args: []float64{rx * 0.6, ry * 0.6, 0, 0, 1}},
			{Type: "A", Points: []Point{{X: cx + smallRx, Y: cy - ry*0.7}}, Args: []float64{smallRx, smallRy, 0, 0, 1}},
			{Type: "A", Points: []Point{{X: cx + rx*0.7, Y: cy}}, Args: []float64{rx * 0.6, ry * 0.6, 0, 0, 1}},
			{Type: "A", Points: []Point{{X: cx + smallRx, Y: cy + ry*0.7}}, Args: []float64{smallRx, smallRy, 0, 0, 1}},
			{Type: "A", Points: []Point{{X: cx, Y: cy + ry}}, Args: []float64{rx * 0.6, ry * 0.6, 0, 0, 1}},
			{Type: "A", Points: []Point{{X: cx - smallRx, Y: cy + ry*0.7}}, Args: []float64{smallRx, smallRy, 0, 0, 1}},
			{Type: "A", Points: []Point{{X: cx - rx*0.7, Y: cy}}, Args: []float64{rx * 0.6, ry * 0.6, 0, 0, 1}},
			{Type: "Z"},
		},
		FillRule: "nonzero",
	}
}

// curvedArrowPath creates a curved arrow pointing in the given direction.
func (pg *PresetGeometry) curvedArrowPath(width, height float64, angleDeg float64) Path {
	// Simple curved arrow shape
	pts := [][2]float64{
		{0, height * 0.5},
		{width * 0.5, height * 0.5},
		{width * 0.8, height * 0.2},
		{width, height * 0.5},
		{width * 0.8, height * 0.8},
		{width * 0.5, height * 0.5},
	}

	pts = rotatePoints(pts, width, height, angleDeg)

	cmds := make([]PathCommand, 0, len(pts)+1)
	for i, p := range pts {
		t := "L"
		if i == 0 {
			t = "M"
		}
		cmds = append(cmds, PathCommand{Type: t, Points: []Point{{X: p[0], Y: p[1]}}})
	}
	cmds = append(cmds, PathCommand{Type: "Z"})

	return Path{Commands: cmds, FillRule: "nonzero"}
}

// flowChartProcessPath creates a flowchart process shape (rectangle).
func (pg *PresetGeometry) flowChartProcessPath(width, height float64) Path {
	// Flowchart process is just a rectangle
	return pg.rectanglePath(width, height)
}

// flowChartDecisionPath creates a flowchart decision shape (diamond).
func (pg *PresetGeometry) flowChartDecisionPath(width, height float64) Path {
	// Flowchart decision is a diamond
	return pg.diamondPath(width, height)
}

// flowChartDocumentPath creates a flowchart document shape (rectangle with wavy bottom).
func (pg *PresetGeometry) flowChartDocumentPath(width, height float64) Path {
	// Simplified document shape with wavy bottom
	waveHeight := height * 0.1
	waveCount := 3
	waveWidth := width / float64(waveCount*2)

	cmds := []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height - waveHeight}}},
	}

	// Add wavy bottom
	for i := 0; i < waveCount; i++ {
		x1 := width - float64(i)*waveWidth*2
		x2 := x1 - waveWidth
		y1 := height - waveHeight
		y2 := height

		if i == 0 {
			cmds = append(cmds, PathCommand{
				Type:   "Q",
				Points: []Point{{X: x1 - waveWidth, Y: y1}, {X: x2, Y: (y1 + y2) / 2}},
			})
		} else {
			cmds = append(cmds, PathCommand{
				Type:   "Q",
				Points: []Point{{X: x1, Y: y2}, {X: x2, Y: (y1 + y2) / 2}},
			})
		}
	}

	cmds = append(cmds, PathCommand{Type: "L", Points: []Point{{X: 0, Y: height - waveHeight}}})
	cmds = append(cmds, PathCommand{Type: "Z"})

	return Path{Commands: cmds, FillRule: "nonzero"}
}

// cubePath creates a simple 3D cube shape.
func (pg *PresetGeometry) cubePath(width, height float64) Path {
	// Simplified cube with perspective
	depth := min(width, height) * 0.3

	cmds := []PathCommand{
		// Front face
		{Type: "M", Points: []Point{{X: depth, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height - depth}}},
		{Type: "L", Points: []Point{{X: depth, Y: height}}},
		{Type: "Z"},
		// Top face
		{Type: "M", Points: []Point{{X: depth, Y: 0}}},
		{Type: "L", Points: []Point{{X: 0, Y: depth}}},
		{Type: "L", Points: []Point{{X: width - depth, Y: depth}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "Z"},
		// Side face
		{Type: "M", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height - depth}}},
		{Type: "L", Points: []Point{{X: width - depth, Y: height}}},
		{Type: "L", Points: []Point{{X: width - depth, Y: depth}}},
		{Type: "Z"},
	}

	return Path{Commands: cmds, FillRule: "nonzero"}
}

// canPath creates a cylindrical can shape.
// Unlike the generic cylinder, the can has a taller top ellipse (the lid) whose
// height is driven by adj1 (DrawingML default 25000 в‰€ 25 % of total height).
// The body sides run straight down, and the bottom half-ellipse closes the base.
func (pg *PresetGeometry) canPath(width, height float64) Path {
	// adj1 controls the top-ellipse height as a fraction of total height.
	// Default in ECMA-376: 25000 / 100000 = 0.25.
	lidRatio := 0.25
	if len(pg.Adjustments) > 0 {
		lidRatio = pg.Adjustments[0]
	}
	if lidRatio < 0.05 {
		lidRatio = 0.05
	}
	if lidRatio > 0.5 {
		lidRatio = 0.5
	}

	rx := width / 2
	lidRY := height * lidRatio // half-height of the top ellipse (lid)

	cmds := []PathCommand{
		// Left side of body, starting at the left of the top ellipse
		{Type: "M", Points: []Point{{X: 0, Y: lidRY}}},
		// Top full ellipse: left в†’ right (large arc, sweep=1)
		{Type: "A", Points: []Point{{X: width, Y: lidRY}}, Args: []float64{rx, lidRY, 0, 1, 1}},
		// Top full ellipse: right в†’ left (completes the lid circle)
		{Type: "A", Points: []Point{{X: 0, Y: lidRY}}, Args: []float64{rx, lidRY, 0, 1, 1}},
		// Right side of body going down
		{Type: "M", Points: []Point{{X: width, Y: lidRY}}},
		{Type: "L", Points: []Point{{X: width, Y: height - lidRY/2}}},
		// Bottom half-ellipse: right в†’ left (the base)
		{Type: "A", Points: []Point{{X: 0, Y: height - lidRY/2}}, Args: []float64{rx, lidRY / 2, 0, 1, 1}},
		// Left side going back up
		{Type: "L", Points: []Point{{X: 0, Y: lidRY}}},
		{Type: "Z"},
	}

	return Path{Commands: cmds, FillRule: "nonzero"}
}

// cylinderPath creates a simple cylinder shape.
func (pg *PresetGeometry) cylinderPath(width, height float64) Path {
	// Simplified cylinder with elliptical top and bottom
	rx := width / 2
	ry := height * 0.15 // Ellipse height for top/bottom
	bodyHeight := height - 2*ry

	cmds := []PathCommand{
		// Top ellipse
		{Type: "M", Points: []Point{{X: 0, Y: ry}}},
		{Type: "A", Points: []Point{{X: width, Y: ry}}, Args: []float64{rx, ry, 0, 1, 1}},
		// Right side
		{Type: "L", Points: []Point{{X: width, Y: ry + bodyHeight}}},
		// Bottom ellipse
		{Type: "A", Points: []Point{{X: 0, Y: ry + bodyHeight}}, Args: []float64{rx, ry, 0, 1, 1}},
		// Left side
		{Type: "L", Points: []Point{{X: 0, Y: ry}}},
		{Type: "Z"},
	}

	return Path{Commands: cmds, FillRule: "nonzero"}
}

// heartPath creates a heart shape.
func (pg *PresetGeometry) heartPath(width, height float64) Path {
	// Heart shape using cubic BГ©zier curves
	// Based on a typical heart shape formula
	cx := width / 2
	cy := height / 2
	rx := width / 2 * 0.9
	ry := height / 2 * 0.9

	// Control points for the heart shape
	// Left lobe
	cmds := []PathCommand{
		// Start at bottom center
		{Type: "M", Points: []Point{{X: cx, Y: height * 0.9}}},
		// Curve to left top of heart
		{Type: "C", Points: []Point{
			{X: cx - rx*0.5, Y: height * 0.7},
			{X: cx - rx, Y: cy},
			{X: cx, Y: cy - ry*0.3},
		}},
		// Curve to right top of heart
		{Type: "C", Points: []Point{
			{X: cx + rx, Y: cy},
			{X: cx + rx*0.5, Y: height * 0.7},
			{X: cx, Y: height * 0.9},
		}},
		{Type: "Z"},
	}

	return Path{Commands: cmds, FillRule: "nonzero"}
}

// lightningBoltPath creates a lightning bolt shape.
func (pg *PresetGeometry) lightningBoltPath(width, height float64) Path {
	// Lightning bolt shape - zigzag pattern
	// Typical lightning bolt has a sharp zigzag pattern
	quarterWidth := width / 4

	cmds := []PathCommand{
		// Start at top-left
		{Type: "M", Points: []Point{{X: quarterWidth, Y: 0}}},
		// Down-right
		{Type: "L", Points: []Point{{X: width * 0.75, Y: height * 0.4}}},
		// Up-right
		{Type: "L", Points: []Point{{X: width * 0.6, Y: height * 0.4}}},
		// Down-right to bottom
		{Type: "L", Points: []Point{{X: width * 0.9, Y: height}}},
		// Up-left from bottom
		{Type: "L", Points: []Point{{X: width * 0.25, Y: height * 0.6}}},
		// Down-left
		{Type: "L", Points: []Point{{X: width * 0.4, Y: height * 0.6}}},
		// Up-left back to near start
		{Type: "L", Points: []Point{{X: quarterWidth, Y: 0}}},
		{Type: "Z"},
	}

	return Path{Commands: cmds, FillRule: "nonzero"}
}

// smileyFacePath creates a smiley face with cut-out eyes and mouth.
func (pg *PresetGeometry) smileyFacePath(width, height float64) Path {
	cmds := ellipseSubpath(0, 0, width, height)
	cmds = append(cmds, ellipseSubpath(width*0.24, height*0.24, width*0.12, height*0.12)...)
	cmds = append(cmds, ellipseSubpath(width*0.64, height*0.24, width*0.12, height*0.12)...)
	cmds = append(cmds,
		PathCommand{Type: "M", Points: []Point{{X: width * 0.28, Y: height * 0.58}}},
		PathCommand{Type: "Q", Points: []Point{
			{X: width * 0.50, Y: height * 0.82},
			{X: width * 0.72, Y: height * 0.58},
		}},
		PathCommand{Type: "Q", Points: []Point{
			{X: width * 0.50, Y: height * 0.68},
			{X: width * 0.28, Y: height * 0.58},
		}},
		PathCommand{Type: "Z"},
	)
	return Path{Commands: cmds, FillRule: "evenodd"}
}

// sunPath creates a simple sunburst.
func (pg *PresetGeometry) sunPath(width, height float64) Path {
	return pg.starPath(width, height, 16, 0.7)
}

// moonPath creates a crescent moon shape.
func (pg *PresetGeometry) moonPath(width, height float64) Path {
	cmds := []PathCommand{
		{Type: "M", Points: []Point{{X: width * 0.62, Y: height * 0.04}}},
		{Type: "C", Points: []Point{
			{X: width * 0.18, Y: height * 0.08},
			{X: width * 0.06, Y: height * 0.92},
			{X: width * 0.62, Y: height * 0.96},
		}},
		{Type: "C", Points: []Point{
			{X: width * 0.38, Y: height * 0.82},
			{X: width * 0.36, Y: height * 0.18},
			{X: width * 0.62, Y: height * 0.04},
		}},
		{Type: "Z"},
	}
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// donutPath creates a ring shape with a transparent centre.
func (pg *PresetGeometry) donutPath(width, height float64) Path {
	innerScale := 0.55
	if len(pg.Adjustments) > 0 {
		innerScale = pg.Adjustments[0]
	}
	if innerScale < 0.1 {
		innerScale = 0.1
	}
	if innerScale > 0.9 {
		innerScale = 0.9
	}

	innerW := width * innerScale
	innerH := height * innerScale
	cmds := ellipseSubpath(0, 0, width, height)
	cmds = append(cmds, ellipseSubpath((width-innerW)/2, (height-innerH)/2, innerW, innerH)...)
	return Path{Commands: cmds, FillRule: "evenodd"}
}

// framePath creates a rectangular frame with a transparent centre.
func (pg *PresetGeometry) framePath(width, height float64) Path {
	inset := min(width, height) * 0.15
	if len(pg.Adjustments) > 0 {
		inset = pg.Adjustments[0] * min(width, height)
	}
	maxInset := min(width, height) * 0.45
	if inset < 1 {
		inset = min(1, maxInset)
	}
	if inset > maxInset {
		inset = maxInset
	}

	cmds := rectSubpath(0, 0, width, height)
	cmds = append(cmds, rectSubpath(inset, inset, width-2*inset, height-2*inset)...)
	return Path{Commands: cmds, FillRule: "evenodd"}
}

// plaquePath creates a softly contoured plaque.
func (pg *PresetGeometry) plaquePath(width, height float64) Path {
	insetX := width * 0.08
	insetY := height * 0.10
	cmds := []PathCommand{
		{Type: "M", Points: []Point{{X: insetX, Y: 0}}},
		{Type: "Q", Points: []Point{
			{X: width * 0.50, Y: insetY * 0.55},
			{X: width - insetX, Y: 0},
		}},
		{Type: "Q", Points: []Point{
			{X: width, Y: insetY},
			{X: width, Y: height * 0.24},
		}},
		{Type: "L", Points: []Point{{X: width, Y: height * 0.76}}},
		{Type: "Q", Points: []Point{
			{X: width, Y: height - insetY},
			{X: width - insetX, Y: height},
		}},
		{Type: "Q", Points: []Point{
			{X: width * 0.50, Y: height - insetY*0.55},
			{X: insetX, Y: height},
		}},
		{Type: "Q", Points: []Point{
			{X: 0, Y: height - insetY},
			{X: 0, Y: height * 0.76},
		}},
		{Type: "L", Points: []Point{{X: 0, Y: height * 0.24}}},
		{Type: "Q", Points: []Point{
			{X: 0, Y: insetY},
			{X: insetX, Y: 0},
		}},
		{Type: "Z"},
	}
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// EMUToPoints converts English Metric Units (EMU) to points.
// 914400 EMU = 1 inch = 72 points.
func EMUToPoints(emu int64) float64 {
	return float64(emu) / 914400.0 * 72.0
}

// PointsToEMU converts points to English Metric Units (EMU).
func PointsToEMU(points float64) int64 {
	return int64(points / 72.0 * 914400.0)
}

// TwipsToPoints converts twips (1/20th of a point) to points.
func TwipsToPoints(twips int64) float64 {
	return float64(twips) / 20.0
}

// PointsToTwips converts points to twips.
func PointsToTwips(points float64) int64 {
	return int64(points * 20.0)
}

// PresetGeometry represents a DrawingML preset shape geometry.
type PresetGeometry struct {
	Name string // e.g., "rect", "roundRect", "ellipse", "triangle"
	// Adjustment values modify the base shape (0.0 to 1.0)
	Adjustments []float64
	Handles     []AdjustHandle
}

// BoundingBox returns the bounding rectangle for a preset geometry.
func (pg *PresetGeometry) BoundingBox(width, height float64) Rect {
	return Rect{X: 0, Y: 0, Width: width, Height: height}
}

// adjOrDefault returns Adjustments[idx] clamped to [0,1] if present, else def.
func (pg *PresetGeometry) adjOrDefault(idx int, def float64) float64 {
	if idx < len(pg.Adjustments) {
		v := pg.Adjustments[idx]
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		return v
	}
	return def
}

// GetPath returns the SVG path for the preset geometry.
func (pg *PresetGeometry) GetPath(width, height float64) (Path, error) {
	switch pg.Name {
	case "rect", "":
		return pg.rectanglePath(width, height), nil
	case "roundRect":
		return pg.roundRectPath(width, height), nil
	case "ellipse":
		return pg.ellipsePath(width, height), nil
	case "triangle", "triangle2":
		return pg.trianglePath(width, height), nil
	case "rtTriangle":
		return pg.rightTrianglePath(width, height), nil
	case "diamond":
		return pg.diamondPath(width, height), nil
	case "parallelogram":
		return pg.parallelogramPath(width, height), nil
	case "trapezoid":
		return pg.trapezoidPath(width, height), nil
	case "pentagon":
		return pg.regularPolygonPath(width, height, 5), nil
	case "hexagon":
		return pg.regularPolygonPath(width, height, 6), nil
	case "heptagon":
		return pg.regularPolygonPath(width, height, 7), nil
	case "octagon":
		return pg.regularPolygonPath(width, height, 8), nil
	case "decagon":
		return pg.regularPolygonPath(width, height, 10), nil
	case "dodecagon":
		return pg.regularPolygonPath(width, height, 12), nil
	case "star4":
		return pg.starPath(width, height, 4, pg.adjOrDefault(0, 0.5)), nil
	case "star5":
		return pg.starPath(width, height, 5, pg.adjOrDefault(0, 0.382)), nil
	case "star6":
		return pg.starPath(width, height, 6, pg.adjOrDefault(0, 0.5)), nil
	case "star7":
		return pg.starPath(width, height, 7, pg.adjOrDefault(0, 0.45)), nil
	case "star8":
		return pg.starPath(width, height, 8, pg.adjOrDefault(0, 0.4)), nil
	case "star10":
		return pg.starPath(width, height, 10, pg.adjOrDefault(0, 0.35)), nil
	case "star12":
		return pg.starPath(width, height, 12, pg.adjOrDefault(0, 0.3)), nil
	case "rightArrow":
		return pg.arrowPath(width, height, 0), nil
	case "leftArrow":
		return pg.arrowPath(width, height, 180), nil
	case "upArrow":
		return pg.arrowPath(width, height, 270), nil
	case "downArrow":
		return pg.arrowPath(width, height, 90), nil
	case "leftRightArrow":
		return pg.leftRightArrowPath(width, height), nil
	case "upDownArrow":
		return pg.upDownArrowPath(width, height), nil
	case "chevron":
		return pg.chevronPath(width, height), nil
	case "pie", "arc":
		return pg.piePath(width, height), nil
	case "chord":
		return pg.chordPath(width, height), nil
	case "wedgeRectCallout", "wedgeRoundRectCallout":
		return pg.wedgeCalloutPath(width, height), nil
	case "cloudCallout":
		return pg.cloudCalloutPath(width, height), nil
	case "smileyFace":
		return pg.smileyFacePath(width, height), nil
	case "sun":
		return pg.sunPath(width, height), nil
	case "moon":
		return pg.moonPath(width, height), nil
	case "cloud":
		return pg.cloudPath(width, height), nil
	case "curvedRightArrow":
		return pg.curvedArrowPath(width, height, 0), nil
	case "curvedLeftArrow":
		return pg.curvedArrowPath(width, height, 180), nil
	case "curvedUpArrow":
		return pg.curvedArrowPath(width, height, 270), nil
	case "curvedDownArrow":
		return pg.curvedArrowPath(width, height, 90), nil
	case "flowChartProcess":
		return pg.flowChartProcessPath(width, height), nil
	case "flowChartDecision":
		return pg.flowChartDecisionPath(width, height), nil
	case "flowChartDocument":
		return pg.flowChartDocumentPath(width, height), nil
	case "cube":
		return pg.cubePath(width, height), nil
	case "can":
		return pg.canPath(width, height), nil
	case "cylinder":
		return pg.cylinderPath(width, height), nil
	case "heart":
		return pg.heartPath(width, height), nil
	case "lightningBolt":
		return pg.lightningBoltPath(width, height), nil
	case "donut":
		return pg.donutPath(width, height), nil
	case "frame":
		return pg.framePath(width, height), nil
	case "plaque":
		return pg.plaquePath(width, height), nil
	// Snipped / beveled corner rectangles
	case "snip1Rect":
		return pg.snip1RectPath(width, height), nil
	case "snip2SameRect":
		return pg.snip2SameRectPath(width, height), nil
	case "snip2DiagRect":
		return pg.snip2DiagRectPath(width, height), nil
	case "snipRoundRect":
		return pg.snipRoundRectPath(width, height), nil
	case "round1Rect":
		return pg.round1RectPath(width, height), nil
	case "round2SameRect":
		return pg.round2SameRectPath(width, height), nil
	case "round2DiagRect":
		return pg.round2DiagRectPath(width, height), nil
	// Notched / home-plate / folded-corner / bevel shapes
	case "notch", "notchedRightArrow":
		return pg.notchPath(width, height), nil
	case "homePlate":
		return pg.homePlatePath(width, height), nil
	case "foldedCorner":
		return pg.foldedCornerPath(width, height), nil
	case "bevel":
		return pg.bevelPath(width, height), nil
	// No-smoking / block-arc / bracket shapes
	case "noSmoking":
		return pg.noSmokingPath(width, height), nil
	case "blockArc":
		return pg.blockArcPath(width, height), nil
	case "bracketPair":
		return pg.bracketPairPath(width, height), nil
	case "bracePair":
		return pg.bracePairPath(width, height), nil
	// Additional flowchart shapes
	case "flowChartTerminator":
		return pg.flowChartTerminatorPath(width, height), nil
	case "flowChartPreparation":
		return pg.flowChartPreparationPath(width, height), nil
	case "flowChartManualInput":
		return pg.flowChartManualInputPath(width, height), nil
	case "flowChartConnector":
		return pg.ellipsePath(width, height), nil
	case "flowChartOffPageConnector":
		return pg.flowChartOffPageConnectorPath(width, height), nil
	case "flowChartPunchedTape":
		return pg.flowChartDocumentPath(width, height), nil // approximate
	case "flowChartMerge":
		return pg.trianglePath(width, height), nil // inverted but approximate
	// Additional arrow variants
	case "rightArrowCallout":
		return pg.arrowCalloutPath(width, height, 0), nil
	case "downArrowCallout":
		return pg.arrowCalloutPath(width, height, 90), nil
	case "leftArrowCallout":
		return pg.arrowCalloutPath(width, height, 180), nil
	case "upArrowCallout":
		return pg.arrowCalloutPath(width, height, 270), nil
	case "leftRightArrowCallout":
		return pg.leftRightArrowCalloutPath(width, height), nil
	case "quadArrow":
		return pg.quadArrowPath(width, height), nil
	case "quadArrowCallout":
		return pg.quadArrowCalloutPath(width, height), nil
	case "bentArrow", "uturnArrow":
		return pg.bentArrowPath(width, height), nil
	case "stripedRightArrow":
		return pg.arrowPath(width, height, 0), nil // simplified
	case "accentCallout1", "accentCallout2", "accentCallout3", "accentCallout4",
		"callout1", "callout2", "callout3",
		"borderCallout1", "borderCallout2", "borderCallout3",
		"accentBorderCallout1", "accentBorderCallout2", "accentBorderCallout3":
		return pg.wedgeCalloutPath(width, height), nil // simplified
	// Ribbons / banners
	case "ribbon", "ribbon2", "ellipseRibbon", "ellipseRibbon2",
		"horizontalScroll", "verticalScroll":
		return pg.ribbonPath(width, height), nil
	// Other decorative shapes
	case "star16":
		return pg.starPath(width, height, 16, pg.adjOrDefault(0, 0.6)), nil
	case "star24":
		return pg.starPath(width, height, 24, pg.adjOrDefault(0, 0.7)), nil
	case "star32":
		return pg.starPath(width, height, 32, pg.adjOrDefault(0, 0.75)), nil
	case "irregularSeal1", "irregularSeal2":
		return pg.irregularSealPath(width, height), nil
	case "cross":
		return pg.crossPath(width, height), nil
	case "plus":
		return pg.crossPath(width, height), nil
	case "mathPlus":
		return pg.crossPath(width, height), nil
	case "mathMinus":
		return pg.mathMinusPath(width, height), nil
	case "mathMultiply":
		return pg.mathMultiplyPath(width, height), nil
	case "mathDivide":
		return pg.crossPath(width, height), nil // approximate
	case "mathEqual":
		return pg.mathEqualPath(width, height), nil
	case "mathNotEqual":
		return pg.mathNotEqualPath(width, height), nil
	case "gear6":
		return pg.gearPath(width, height, 6), nil
	case "gear9":
		return pg.gearPath(width, height, 9), nil
	default:
		return pg.rectanglePath(width, height), nil
	}
}

// в”Ђв”Ђ Snipped / rounded corner rectangles в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

// snip1RectPath creates a rectangle with one snipped (45В°-cut) corner at top-right.
func (pg *PresetGeometry) snip1RectPath(width, height float64) Path {
	cut := min(width, height) * 0.2
	if len(pg.Adjustments) > 0 {
		cut = pg.Adjustments[0] * min(width, height)
	}
	cut = min(cut, min(width, height)*0.45)
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - cut, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: cut}}},
		{Type: "L", Points: []Point{{X: width, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// snip2SameRectPath creates a rectangle with two snipped corners on the same side (top).
func (pg *PresetGeometry) snip2SameRectPath(width, height float64) Path {
	cut := min(width, height) * 0.2
	if len(pg.Adjustments) > 0 {
		cut = pg.Adjustments[0] * min(width, height)
	}
	cut = min(cut, width*0.45)
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: cut, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - cut, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: cut}}},
		{Type: "L", Points: []Point{{X: width, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: cut}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// snip2DiagRectPath creates a rectangle with two snipped corners on opposite diagonals.
func (pg *PresetGeometry) snip2DiagRectPath(width, height float64) Path {
	cut := min(width, height) * 0.2
	if len(pg.Adjustments) > 0 {
		cut = pg.Adjustments[0] * min(width, height)
	}
	cut = min(cut, min(width, height)*0.45)
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - cut, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: cut}}},
		{Type: "L", Points: []Point{{X: width, Y: height}}},
		{Type: "L", Points: []Point{{X: cut, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height - cut}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// snipRoundRectPath creates a rectangle with one snipped and one rounded corner.
func (pg *PresetGeometry) snipRoundRectPath(width, height float64) Path {
	cut := min(width, height) * 0.2
	radius := min(width, height) * 0.15
	if len(pg.Adjustments) > 1 {
		cut = pg.Adjustments[0] * min(width, height)
		radius = pg.Adjustments[1] * min(width, height)
	}
	cut = min(cut, min(width, height)*0.45)
	radius = min(radius, min(width, height)*0.45)
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: radius, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - cut, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: cut}}},
		{Type: "L", Points: []Point{{X: width, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: radius}}},
		{Type: "A", Points: []Point{{X: radius, Y: 0}}, Args: []float64{radius, radius, 0, 0, 1}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// round1RectPath creates a rectangle with one rounded corner at top-left.
func (pg *PresetGeometry) round1RectPath(width, height float64) Path {
	r := min(width, height) * 0.2
	if len(pg.Adjustments) > 0 {
		r = pg.Adjustments[0] * min(width, height)
	}
	r = min(r, min(width, height)*0.45)
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: r, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: r}}},
		{Type: "A", Points: []Point{{X: r, Y: 0}}, Args: []float64{r, r, 0, 0, 1}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// round2SameRectPath creates a rectangle with two rounded corners on the same side.
func (pg *PresetGeometry) round2SameRectPath(width, height float64) Path {
	r := min(width, height) * 0.15
	if len(pg.Adjustments) > 0 {
		r = pg.Adjustments[0] * min(width, height)
	}
	r = min(r, min(width, height)*0.45)
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: r, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - r, Y: 0}}},
		{Type: "A", Points: []Point{{X: width, Y: r}}, Args: []float64{r, r, 0, 0, 1}},
		{Type: "L", Points: []Point{{X: width, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: r}}},
		{Type: "A", Points: []Point{{X: r, Y: 0}}, Args: []float64{r, r, 0, 0, 1}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// round2DiagRectPath creates a rectangle with two rounded corners on opposite diagonals.
func (pg *PresetGeometry) round2DiagRectPath(width, height float64) Path {
	r := min(width, height) * 0.15
	if len(pg.Adjustments) > 0 {
		r = pg.Adjustments[0] * min(width, height)
	}
	r = min(r, min(width, height)*0.45)
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: r, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height - r}}},
		{Type: "A", Points: []Point{{X: width - r, Y: height}}, Args: []float64{r, r, 0, 0, 1}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: r}}},
		{Type: "A", Points: []Point{{X: r, Y: 0}}, Args: []float64{r, r, 0, 0, 1}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// в”Ђв”Ђ Notched / home-plate / folded-corner / bevel в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

// notchPath creates a notched right-arrow shape (pentagon with right-pointing tip,
// left side notched inward at center).
func (pg *PresetGeometry) notchPath(width, height float64) Path {
	notch := width * 0.2
	if len(pg.Adjustments) > 0 {
		notch = pg.Adjustments[0] * width
	}
	notch = min(notch, width*0.45)
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - notch, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height / 2}}},
		{Type: "L", Points: []Point{{X: width - notch, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "L", Points: []Point{{X: notch, Y: height / 2}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// homePlatePath creates a home-plate (pentagon with flat left, pointed right).
func (pg *PresetGeometry) homePlatePath(width, height float64) Path {
	tipW := width * 0.3
	if len(pg.Adjustments) > 0 {
		tipW = pg.Adjustments[0] * width
	}
	tipW = min(tipW, width*0.45)
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - tipW, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height / 2}}},
		{Type: "L", Points: []Point{{X: width - tipW, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// foldedCornerPath creates a rectangle with a folded-page corner at bottom-right.
func (pg *PresetGeometry) foldedCornerPath(width, height float64) Path {
	fold := min(width, height) * 0.2
	if len(pg.Adjustments) > 0 {
		fold = pg.Adjustments[0] * min(width, height)
	}
	fold = min(fold, min(width, height)*0.45)
	return Path{Commands: []PathCommand{
		// Main body (minus the corner triangle)
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height - fold}}},
		{Type: "L", Points: []Point{{X: width - fold, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "Z"},
		// Folded triangle tab
		{Type: "M", Points: []Point{{X: width - fold, Y: height - fold}}},
		{Type: "L", Points: []Point{{X: width, Y: height - fold}}},
		{Type: "L", Points: []Point{{X: width - fold, Y: height}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// bevelPath creates a beveled rectangle (outer rect minus four corner triangles).
func (pg *PresetGeometry) bevelPath(width, height float64) Path {
	bev := min(width, height) * 0.2
	if len(pg.Adjustments) > 0 {
		bev = pg.Adjustments[0] * min(width, height)
	}
	bev = min(bev, min(width, height)*0.45)
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: bev, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - bev, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: bev}}},
		{Type: "L", Points: []Point{{X: width, Y: height - bev}}},
		{Type: "L", Points: []Point{{X: width - bev, Y: height}}},
		{Type: "L", Points: []Point{{X: bev, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height - bev}}},
		{Type: "L", Points: []Point{{X: 0, Y: bev}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// в”Ђв”Ђ Special / symbol shapes в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

// noSmokingPath creates a "no-smoking" style circle with a diagonal slash.
func (pg *PresetGeometry) noSmokingPath(width, height float64) Path {
	// Outer ring + diagonal slash using evenodd fill.
	thickness := min(width, height) * 0.15
	cmds := ellipseSubpath(0, 0, width, height)
	cmds = append(cmds, ellipseSubpath(thickness, thickness, width-2*thickness, height-2*thickness)...)
	// Diagonal bar
	bw := thickness * 0.7
	cmds = append(cmds,
		PathCommand{Type: "M", Points: []Point{{X: width*0.5 - bw, Y: 0}}},
		PathCommand{Type: "L", Points: []Point{{X: width*0.5 + bw, Y: 0}}},
		PathCommand{Type: "L", Points: []Point{{X: width*0.5 + bw, Y: height}}},
		PathCommand{Type: "L", Points: []Point{{X: width*0.5 - bw, Y: height}}},
		PathCommand{Type: "Z"},
	)
	return Path{Commands: cmds, FillRule: "evenodd"}
}

// blockArcPath creates a block-arc (thick arc segment).
func (pg *PresetGeometry) blockArcPath(width, height float64) Path {
	// Use adj1 for sweep angle (default 270В°), adj2 for inner radius ratio.
	sweepDeg := 270.0
	innerRatio := 0.5
	if len(pg.Adjustments) > 0 {
		sweepDeg = pg.Adjustments[0] * 360
	}
	if len(pg.Adjustments) > 1 {
		innerRatio = pg.Adjustments[1]
	}
	innerRatio = math.Max(0.1, math.Min(0.9, innerRatio))

	rx, ry := width/2, height/2
	cx, cy := width/2, height/2
	irx, iry := rx*innerRatio, ry*innerRatio

	startRad := -math.Pi / 2
	endRad := startRad + sweepDeg*math.Pi/180

	// Outer arc start/end
	ox1 := cx + rx*math.Cos(startRad)
	oy1 := cy + ry*math.Sin(startRad)
	ox2 := cx + rx*math.Cos(endRad)
	oy2 := cy + ry*math.Sin(endRad)
	// Inner arc start/end (reversed)
	ix1 := cx + irx*math.Cos(endRad)
	iy1 := cy + iry*math.Sin(endRad)
	ix2 := cx + irx*math.Cos(startRad)
	iy2 := cy + iry*math.Sin(startRad)

	largeArc := 0.0
	if sweepDeg > 180 {
		largeArc = 1.0
	}
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: ox1, Y: oy1}}},
		{Type: "A", Points: []Point{{X: ox2, Y: oy2}}, Args: []float64{rx, ry, 0, largeArc, 1}},
		{Type: "L", Points: []Point{{X: ix1, Y: iy1}}},
		{Type: "A", Points: []Point{{X: ix2, Y: iy2}}, Args: []float64{irx, iry, 0, largeArc, 0}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// bracketPairPath creates a pair of square brackets [ ].
func (pg *PresetGeometry) bracketPairPath(width, height float64) Path {
	armW := width * 0.15
	return Path{Commands: []PathCommand{
		// Left bracket
		{Type: "M", Points: []Point{{X: armW, Y: 0}}},
		{Type: "L", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "L", Points: []Point{{X: armW, Y: height}}},
		// Right bracket
		{Type: "M", Points: []Point{{X: width - armW, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height}}},
		{Type: "L", Points: []Point{{X: width - armW, Y: height}}},
	}, FillRule: "nonzero"}
}

// bracePairPath creates a pair of curly braces { }.
func (pg *PresetGeometry) bracePairPath(width, height float64) Path {
	// Simplified curly brace pair using quadratic curves.
	mW := width * 0.08  // main width of brace arm
	cH := height * 0.15 // height of the curved segment at top/bottom/middle
	return Path{Commands: []PathCommand{
		// Left brace
		{Type: "M", Points: []Point{{X: mW * 2, Y: 0}}},
		{Type: "Q", Points: []Point{{X: mW, Y: 0}, {X: mW, Y: cH}}},
		{Type: "L", Points: []Point{{X: mW, Y: height/2 - cH}}},
		{Type: "Q", Points: []Point{{X: mW, Y: height / 2}, {X: 0, Y: height / 2}}},
		{Type: "Q", Points: []Point{{X: mW, Y: height / 2}, {X: mW, Y: height/2 + cH}}},
		{Type: "L", Points: []Point{{X: mW, Y: height - cH}}},
		{Type: "Q", Points: []Point{{X: mW, Y: height}, {X: mW * 2, Y: height}}},
		// Right brace
		{Type: "M", Points: []Point{{X: width - mW*2, Y: 0}}},
		{Type: "Q", Points: []Point{{X: width - mW, Y: 0}, {X: width - mW, Y: cH}}},
		{Type: "L", Points: []Point{{X: width - mW, Y: height/2 - cH}}},
		{Type: "Q", Points: []Point{{X: width - mW, Y: height / 2}, {X: width, Y: height / 2}}},
		{Type: "Q", Points: []Point{{X: width - mW, Y: height / 2}, {X: width - mW, Y: height/2 + cH}}},
		{Type: "L", Points: []Point{{X: width - mW, Y: height - cH}}},
		{Type: "Q", Points: []Point{{X: width - mW, Y: height}, {X: width - mW*2, Y: height}}},
	}, FillRule: "nonzero"}
}

// в”Ђв”Ђ Flowchart shapes в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

// flowChartTerminatorPath creates a flowchart terminator (stadium / pill shape).
func (pg *PresetGeometry) flowChartTerminatorPath(width, height float64) Path {
	r := height / 2
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: r, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - r, Y: 0}}},
		{Type: "A", Points: []Point{{X: width - r, Y: height}}, Args: []float64{r, r, 0, 0, 1}},
		{Type: "L", Points: []Point{{X: r, Y: height}}},
		{Type: "A", Points: []Point{{X: r, Y: 0}}, Args: []float64{r, r, 0, 0, 1}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// flowChartPreparationPath creates a hexagonal flowchart preparation shape.
func (pg *PresetGeometry) flowChartPreparationPath(width, height float64) Path {
	indent := width * 0.2
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: indent, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - indent, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height / 2}}},
		{Type: "L", Points: []Point{{X: width - indent, Y: height}}},
		{Type: "L", Points: []Point{{X: indent, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height / 2}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// flowChartManualInputPath creates a slanted-top rectangle for manual input.
func (pg *PresetGeometry) flowChartManualInputPath(width, height float64) Path {
	slant := height * 0.2
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: slant}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// flowChartOffPageConnectorPath creates a pentagon that looks like a page tab.
func (pg *PresetGeometry) flowChartOffPageConnectorPath(width, height float64) Path {
	tipH := height * 0.3
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height - tipH}}},
		{Type: "L", Points: []Point{{X: width / 2, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height - tipH}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// в”Ђв”Ђ Arrow variants в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

// bentArrowPath creates a simple L-shaped arrow (bent 90В°).
func (pg *PresetGeometry) bentArrowPath(width, height float64) Path {
	armW := height * 0.35
	headW := height * 0.6
	headH := width * 0.3
	return Path{Commands: []PathCommand{
		// Horizontal arm
		{Type: "M", Points: []Point{{X: 0, Y: height/2 - armW/2}}},
		{Type: "L", Points: []Point{{X: width - headH, Y: height/2 - armW/2}}},
		// Arrow head going up
		{Type: "L", Points: []Point{{X: width - headH, Y: height/2 - headW/2}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: 0, Y: 0}}},
		// Close
		{Type: "L", Points: []Point{{X: 0, Y: height/2 - armW/2}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// arrowCalloutPath creates an arrow callout with a rectangular label body and a directional arrow.
func (pg *PresetGeometry) arrowCalloutPath(width, height, angleDeg float64) Path {
	switch angleDeg {
	case 180:
		bodyW := width * 0.48
		bodyH := height * 0.56
		bodyX := width - bodyW
		bodyY := (height - bodyH) / 2
		shaftH := height * 0.22
		shaftY := (height - shaftH) / 2
		headBaseX := width * 0.24
		cmds := rectSubpath(bodyX, bodyY, bodyW, bodyH)
		cmds = append(cmds,
			PathCommand{Type: "M", Points: []Point{{X: bodyX, Y: shaftY}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: shaftY}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: 0}}},
			PathCommand{Type: "L", Points: []Point{{X: 0, Y: height / 2}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: height}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: shaftY + shaftH}}},
			PathCommand{Type: "L", Points: []Point{{X: bodyX, Y: shaftY + shaftH}}},
			PathCommand{Type: "Z"},
		)
		return Path{Commands: cmds, FillRule: "nonzero"}
	case 270:
		bodyW := width * 0.56
		bodyH := height * 0.42
		bodyX := (width - bodyW) / 2
		bodyY := height - bodyH
		shaftW := width * 0.22
		shaftX := (width - shaftW) / 2
		headBaseY := height * 0.24
		headW := width * 0.5
		cmds := rectSubpath(bodyX, bodyY, bodyW, bodyH)
		cmds = append(cmds,
			PathCommand{Type: "M", Points: []Point{{X: shaftX, Y: bodyY}}},
			PathCommand{Type: "L", Points: []Point{{X: shaftX, Y: headBaseY}}},
			PathCommand{Type: "L", Points: []Point{{X: width/2 - headW/2, Y: headBaseY}}},
			PathCommand{Type: "L", Points: []Point{{X: width / 2, Y: 0}}},
			PathCommand{Type: "L", Points: []Point{{X: width/2 + headW/2, Y: headBaseY}}},
			PathCommand{Type: "L", Points: []Point{{X: shaftX + shaftW, Y: headBaseY}}},
			PathCommand{Type: "L", Points: []Point{{X: shaftX + shaftW, Y: bodyY}}},
			PathCommand{Type: "Z"},
		)
		return Path{Commands: cmds, FillRule: "nonzero"}
	case 90:
		bodyW := width * 0.56
		bodyH := height * 0.42
		bodyX := (width - bodyW) / 2
		bodyY := 0.0
		shaftW := width * 0.22
		shaftX := (width - shaftW) / 2
		headBaseY := height * 0.76
		headW := width * 0.5
		cmds := rectSubpath(bodyX, bodyY, bodyW, bodyH)
		cmds = append(cmds,
			PathCommand{Type: "M", Points: []Point{{X: shaftX, Y: bodyH}}},
			PathCommand{Type: "L", Points: []Point{{X: shaftX, Y: headBaseY}}},
			PathCommand{Type: "L", Points: []Point{{X: width/2 - headW/2, Y: headBaseY}}},
			PathCommand{Type: "L", Points: []Point{{X: width / 2, Y: height}}},
			PathCommand{Type: "L", Points: []Point{{X: width/2 + headW/2, Y: headBaseY}}},
			PathCommand{Type: "L", Points: []Point{{X: shaftX + shaftW, Y: headBaseY}}},
			PathCommand{Type: "L", Points: []Point{{X: shaftX + shaftW, Y: bodyH}}},
			PathCommand{Type: "Z"},
		)
		return Path{Commands: cmds, FillRule: "nonzero"}
	default:
		bodyW := width * 0.48
		bodyH := height * 0.56
		bodyY := (height - bodyH) / 2
		shaftH := height * 0.22
		shaftX := bodyW
		shaftY := (height - shaftH) / 2
		headBaseX := width - width*0.24
		cmds := rectSubpath(0, bodyY, bodyW, bodyH)
		cmds = append(cmds,
			PathCommand{Type: "M", Points: []Point{{X: shaftX, Y: shaftY}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: shaftY}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: 0}}},
			PathCommand{Type: "L", Points: []Point{{X: width, Y: height / 2}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: height}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: shaftY + shaftH}}},
			PathCommand{Type: "L", Points: []Point{{X: shaftX, Y: shaftY + shaftH}}},
			PathCommand{Type: "Z"},
		)
		return Path{Commands: cmds, FillRule: "nonzero"}
	}
}

// leftRightArrowCalloutPath creates a center callout box with arrows extending left and right.
func (pg *PresetGeometry) leftRightArrowCalloutPath(width, height float64) Path {
	centerW := width * 0.34
	centerH := height * 0.56
	centerX := (width - centerW) / 2
	centerY := (height - centerH) / 2
	barH := height * 0.22
	barY := (height - barH) / 2
	headW := width * 0.2

	cmds := rectSubpath(centerX, centerY, centerW, centerH)
	arrow := []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: height / 2}}},
		{Type: "L", Points: []Point{{X: headW, Y: barY}}},
		{Type: "L", Points: []Point{{X: centerX, Y: barY}}},
		{Type: "L", Points: []Point{{X: centerX, Y: centerY}}},
		{Type: "L", Points: []Point{{X: centerX + centerW, Y: centerY}}},
		{Type: "L", Points: []Point{{X: centerX + centerW, Y: barY}}},
		{Type: "L", Points: []Point{{X: width - headW, Y: barY}}},
		{Type: "L", Points: []Point{{X: width, Y: height / 2}}},
		{Type: "L", Points: []Point{{X: width - headW, Y: barY + barH}}},
		{Type: "L", Points: []Point{{X: centerX + centerW, Y: barY + barH}}},
		{Type: "L", Points: []Point{{X: centerX + centerW, Y: centerY + centerH}}},
		{Type: "L", Points: []Point{{X: centerX, Y: centerY + centerH}}},
		{Type: "L", Points: []Point{{X: centerX, Y: barY + barH}}},
		{Type: "L", Points: []Point{{X: headW, Y: barY + barH}}},
		{Type: "Z"},
	}
	cmds = append(cmds, arrow...)
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// quadArrowPath creates a four-direction arrow with a central body.
func (pg *PresetGeometry) quadArrowPath(width, height float64) Path {
	cx, cy := width/2, height/2
	headW := width * 0.22
	headH := height * 0.22
	bodyW := width * 0.24
	bodyH := height * 0.24
	x0 := cx - bodyW/2
	x1 := cx + bodyW/2
	y0 := cy - bodyH/2
	y1 := cy + bodyH/2

	cmds := []PathCommand{
		{Type: "M", Points: []Point{{X: cx, Y: 0}}},
		{Type: "L", Points: []Point{{X: cx + headW/2, Y: headH}}},
		{Type: "L", Points: []Point{{X: x1, Y: headH}}},
		{Type: "L", Points: []Point{{X: x1, Y: y0}}},
		{Type: "L", Points: []Point{{X: width - headW, Y: y0}}},
		{Type: "L", Points: []Point{{X: width - headW, Y: cy - headH/2}}},
		{Type: "L", Points: []Point{{X: width, Y: cy}}},
		{Type: "L", Points: []Point{{X: width - headW, Y: cy + headH/2}}},
		{Type: "L", Points: []Point{{X: width - headW, Y: y1}}},
		{Type: "L", Points: []Point{{X: x1, Y: y1}}},
		{Type: "L", Points: []Point{{X: x1, Y: height - headH}}},
		{Type: "L", Points: []Point{{X: cx + headW/2, Y: height - headH}}},
		{Type: "L", Points: []Point{{X: cx, Y: height}}},
		{Type: "L", Points: []Point{{X: cx - headW/2, Y: height - headH}}},
		{Type: "L", Points: []Point{{X: x0, Y: height - headH}}},
		{Type: "L", Points: []Point{{X: x0, Y: y1}}},
		{Type: "L", Points: []Point{{X: headW, Y: y1}}},
		{Type: "L", Points: []Point{{X: headW, Y: cy + headH/2}}},
		{Type: "L", Points: []Point{{X: 0, Y: cy}}},
		{Type: "L", Points: []Point{{X: headW, Y: cy - headH/2}}},
		{Type: "L", Points: []Point{{X: headW, Y: y0}}},
		{Type: "L", Points: []Point{{X: x0, Y: y0}}},
		{Type: "L", Points: []Point{{X: x0, Y: headH}}},
		{Type: "L", Points: []Point{{X: cx - headW/2, Y: headH}}},
		{Type: "Z"},
	}
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// quadArrowCalloutPath adds a central callout box to the four-direction arrow shape.
func (pg *PresetGeometry) quadArrowCalloutPath(width, height float64) Path {
	quad := pg.quadArrowPath(width, height)
	centerW := width * 0.2
	centerH := height * 0.2
	centerX := (width - centerW) / 2
	centerY := (height - centerH) / 2
	quad.Commands = append(rectSubpath(centerX, centerY, centerW, centerH), quad.Commands...)
	return quad
}

// в”Ђв”Ђ Math / symbol shapes в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

// crossPath creates a plus/cross shape.
func (pg *PresetGeometry) crossPath(width, height float64) Path {
	armW := width * 0.35
	armH := height * 0.35
	x0 := (width - armW) / 2
	y0 := (height - armH) / 2
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: x0, Y: 0}}},
		{Type: "L", Points: []Point{{X: x0 + armW, Y: 0}}},
		{Type: "L", Points: []Point{{X: x0 + armW, Y: y0}}},
		{Type: "L", Points: []Point{{X: width, Y: y0}}},
		{Type: "L", Points: []Point{{X: width, Y: y0 + armH}}},
		{Type: "L", Points: []Point{{X: x0 + armW, Y: y0 + armH}}},
		{Type: "L", Points: []Point{{X: x0 + armW, Y: height}}},
		{Type: "L", Points: []Point{{X: x0, Y: height}}},
		{Type: "L", Points: []Point{{X: x0, Y: y0 + armH}}},
		{Type: "L", Points: []Point{{X: 0, Y: y0 + armH}}},
		{Type: "L", Points: []Point{{X: 0, Y: y0}}},
		{Type: "L", Points: []Point{{X: x0, Y: y0}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// mathMinusPath creates a minus-sign rectangle bar.
func (pg *PresetGeometry) mathMinusPath(width, height float64) Path {
	barH := height * 0.3
	y0 := (height - barH) / 2
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: y0}}},
		{Type: "L", Points: []Point{{X: width, Y: y0}}},
		{Type: "L", Points: []Point{{X: width, Y: y0 + barH}}},
		{Type: "L", Points: []Point{{X: 0, Y: y0 + barH}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// mathMultiplyPath creates an X (multiply) shape.
func (pg *PresetGeometry) mathMultiplyPath(width, height float64) Path {
	return pg.starPath(width, height, 4, 0.35)
}

// mathEqualPath creates a double bar equal sign.
func (pg *PresetGeometry) mathEqualPath(width, height float64) Path {
	barH := height * 0.2
	gap := height * 0.15
	y1 := height/2 - gap/2 - barH
	y2 := height/2 + gap/2
	cmds := rectSubpath(0, y1, width, barH)
	cmds = append(cmds, rectSubpath(0, y2, width, barH)...)
	return Path{Commands: cmds, FillRule: "evenodd"}
}

// mathNotEqualPath creates a not-equal sign (two bars with a diagonal slash).
func (pg *PresetGeometry) mathNotEqualPath(width, height float64) Path {
	barH := height * 0.2
	gap := height * 0.15
	y1 := height/2 - gap/2 - barH
	y2 := height/2 + gap/2
	slashW := width * 0.1
	// Two horizontal bars
	cmds := rectSubpath(0, y1, width, barH)
	cmds = append(cmds, rectSubpath(0, y2, width, barH)...)
	// Diagonal slash
	cmds = append(cmds,
		PathCommand{Type: "M", Points: []Point{{X: width/2 - slashW, Y: 0}}},
		PathCommand{Type: "L", Points: []Point{{X: width/2 + slashW, Y: 0}}},
		PathCommand{Type: "L", Points: []Point{{X: width/2 + slashW, Y: height}}},
		PathCommand{Type: "L", Points: []Point{{X: width/2 - slashW, Y: height}}},
		PathCommand{Type: "Z"},
	)
	return Path{Commands: cmds, FillRule: "evenodd"}
}

// в”Ђв”Ђ Gear / irregular shapes в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

// gearPath creates a simple gear shape with the given number of teeth.
func (pg *PresetGeometry) gearPath(width, height float64, teeth int) Path {
	cx, cy := width/2, height/2
	outerR := min(width, height) / 2
	innerR := outerR * 0.65
	toothDepth := outerR - innerR
	_ = toothDepth

	total := teeth * 2 // alternating inner/outer points
	cmds := make([]PathCommand, 0, total+2)
	for i := 0; i < total; i++ {
		angle := float64(i) * math.Pi / float64(teeth)
		var r float64
		if i%2 == 0 {
			r = outerR
		} else {
			r = innerR
		}
		x := cx + r*math.Cos(angle)
		y := cy + r*math.Sin(angle)
		t := "L"
		if i == 0 {
			t = "M"
		}
		cmds = append(cmds, PathCommand{Type: t, Points: []Point{{X: x, Y: y}}})
	}
	cmds = append(cmds, PathCommand{Type: "Z"})
	// Inner hub hole (evenodd)
	hubR := outerR * 0.3
	cmds = append(cmds, ellipseSubpath(cx-hubR, cy-hubR, hubR*2, hubR*2)...)
	return Path{Commands: cmds, FillRule: "evenodd"}
}

// irregularSealPath creates an irregular star-burst / seal shape.
func (pg *PresetGeometry) irregularSealPath(width, height float64) Path {
	// Use a high-point-count star with slight inner-ratio variation for irregularity.
	return pg.starPath(width, height, 12, 0.6)
}

// ribbonPath creates a simple ribbon / banner shape.
func (pg *PresetGeometry) ribbonPath(width, height float64) Path {
	fold := height * 0.3
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height - fold}}},
		{Type: "L", Points: []Point{{X: width * 0.75, Y: height}}},
		{Type: "L", Points: []Point{{X: width * 0.5, Y: height - fold*0.5}}},
		{Type: "L", Points: []Point{{X: width * 0.25, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height - fold}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// rectanglePath creates a rectangle path.
func (pg *PresetGeometry) rectanglePath(width, height float64) Path {
	return Path{
		Commands: []PathCommand{
			{Type: "M", Points: []Point{{X: 0, Y: 0}}},
			{Type: "L", Points: []Point{{X: width, Y: 0}}},
			{Type: "L", Points: []Point{{X: width, Y: height}}},
			{Type: "L", Points: []Point{{X: 0, Y: height}}},
			{Type: "Z", Points: []Point{}},
		},
		FillRule: "nonzero",
	}
}

// roundRectPath creates a rounded rectangle path.
func (pg *PresetGeometry) roundRectPath(width, height float64) Path {
	// Use first adjustment for corner radius, default to 10% of smaller dimension
	radius := 0.1 * min(width, height)
	if len(pg.Adjustments) > 0 {
		radius = pg.Adjustments[0] * min(width, height)
	}

	// Clamp radius to reasonable values
	maxRadius := min(width, height) / 2
	if radius > maxRadius {
		radius = maxRadius
	}

	return Path{
		Commands: []PathCommand{
			// Move to top-left corner (after horizontal line)
			{Type: "M", Points: []Point{{X: radius, Y: 0}}},
			// Top line
			{Type: "L", Points: []Point{{X: width - radius, Y: 0}}},
			// Top-right arc
			{Type: "A", Points: []Point{{X: width, Y: radius}}, Args: []float64{radius, radius, 0, 0, 1}},
			// Right line
			{Type: "L", Points: []Point{{X: width, Y: height - radius}}},
			// Bottom-right arc
			{Type: "A", Points: []Point{{X: width - radius, Y: height}}, Args: []float64{radius, radius, 0, 0, 1}},
			// Bottom line
			{Type: "L", Points: []Point{{X: radius, Y: height}}},
			// Bottom-left arc
			{Type: "A", Points: []Point{{X: 0, Y: height - radius}}, Args: []float64{radius, radius, 0, 0, 1}},
			// Left line
			{Type: "L", Points: []Point{{X: 0, Y: radius}}},
			// Top-left arc
			{Type: "A", Points: []Point{{X: radius, Y: 0}}, Args: []float64{radius, radius, 0, 0, 1}},
			// Close path
			{Type: "Z", Points: []Point{}},
		},
		FillRule: "nonzero",
	}
}

// ellipsePath creates an ellipse path.
func (pg *PresetGeometry) ellipsePath(width, height float64) Path {
	rx := width / 2
	ry := height / 2
	cx := width / 2
	cy := height / 2

	return Path{
		Commands: []PathCommand{
			// Move to rightmost point
			{Type: "M", Points: []Point{{X: cx + rx, Y: cy}}},
			// Arc to top
			{Type: "A", Points: []Point{{X: cx, Y: cy - ry}}, Args: []float64{rx, ry, 0, 0, 1}},
			// Arc to left
			{Type: "A", Points: []Point{{X: cx - rx, Y: cy}}, Args: []float64{rx, ry, 0, 0, 1}},
			// Arc to bottom
			{Type: "A", Points: []Point{{X: cx, Y: cy + ry}}, Args: []float64{rx, ry, 0, 0, 1}},
			// Arc back to right
			{Type: "A", Points: []Point{{X: cx + rx, Y: cy}}, Args: []float64{rx, ry, 0, 0, 1}},
			// Close path
			{Type: "Z", Points: []Point{}},
		},
		FillRule: "nonzero",
	}
}

// trianglePath creates a triangle path.
func (pg *PresetGeometry) trianglePath(width, height float64) Path {
	// Default to upward-pointing triangle
	// Adjustments could change orientation, but we'll keep it simple for now
	return Path{
		Commands: []PathCommand{
			// Move to bottom-left
			{Type: "M", Points: []Point{{X: 0, Y: height}}},
			// Line to top-center
			{Type: "L", Points: []Point{{X: width / 2, Y: 0}}},
			// Line to bottom-right
			{Type: "L", Points: []Point{{X: width, Y: height}}},
			// Close path
			{Type: "Z", Points: []Point{}},
		},
		FillRule: "nonzero",
	}
}

// rightTrianglePath creates a right-angle triangle (right angle at bottom-left).
func (pg *PresetGeometry) rightTrianglePath(width, height float64) Path {
	return Path{
		Commands: []PathCommand{
			{Type: "M", Points: []Point{{X: 0, Y: height}}},
			{Type: "L", Points: []Point{{X: width, Y: height}}},
			{Type: "L", Points: []Point{{X: 0, Y: 0}}},
			{Type: "Z"},
		},
		FillRule: "nonzero",
	}
}

// diamondPath creates a diamond (rhombus) path.
func (pg *PresetGeometry) diamondPath(width, height float64) Path {
	cx, cy := width/2, height/2
	return Path{
		Commands: []PathCommand{
			{Type: "M", Points: []Point{{X: cx, Y: 0}}},
			{Type: "L", Points: []Point{{X: width, Y: cy}}},
			{Type: "L", Points: []Point{{X: cx, Y: height}}},
			{Type: "L", Points: []Point{{X: 0, Y: cy}}},
			{Type: "Z"},
		},
		FillRule: "nonzero",
	}
}

// parallelogramPath creates a parallelogram slanted to the right.
func (pg *PresetGeometry) parallelogramPath(width, height float64) Path {
	indent := width * 0.25
	if len(pg.Adjustments) > 0 {
		indent = pg.Adjustments[0] * width
	}
	return Path{
		Commands: []PathCommand{
			{Type: "M", Points: []Point{{X: indent, Y: 0}}},
			{Type: "L", Points: []Point{{X: width, Y: 0}}},
			{Type: "L", Points: []Point{{X: width - indent, Y: height}}},
			{Type: "L", Points: []Point{{X: 0, Y: height}}},
			{Type: "Z"},
		},
		FillRule: "nonzero",
	}
}

// trapezoidPath creates a trapezoid (wider at the bottom).
func (pg *PresetGeometry) trapezoidPath(width, height float64) Path {
	inset := width * 0.25
	if len(pg.Adjustments) > 0 {
		inset = pg.Adjustments[0] * width
	}
	return Path{
		Commands: []PathCommand{
			{Type: "M", Points: []Point{{X: 0, Y: height}}},
			{Type: "L", Points: []Point{{X: width, Y: height}}},
			{Type: "L", Points: []Point{{X: width - inset, Y: 0}}},
			{Type: "L", Points: []Point{{X: inset, Y: 0}}},
			{Type: "Z"},
		},
		FillRule: "nonzero",
	}
}

// regularPolygonPath creates a regular n-gon inscribed in the shape bounds.
func (pg *PresetGeometry) regularPolygonPath(width, height float64, sides int) Path {
	cx, cy := width/2, height/2
	rx, ry := width/2, height/2
	// Start at the top (angle = -ПЂ/2) for upright orientation.
	startAngle := -math.Pi / 2
	cmds := make([]PathCommand, 0, sides+1)
	for i := 0; i <= sides; i++ {
		angle := startAngle + float64(i)*2*math.Pi/float64(sides)
		x := cx + rx*math.Cos(angle)
		y := cy + ry*math.Sin(angle)
		t := "L"
		if i == 0 {
			t = "M"
		}
		cmds = append(cmds, PathCommand{Type: t, Points: []Point{{X: x, Y: y}}})
	}
	cmds = append(cmds, PathCommand{Type: "Z"})
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// starPath creates a star with the given number of points and inner-radius ratio.
func (pg *PresetGeometry) starPath(width, height float64, points int, innerRatio float64) Path {
	cx, cy := width/2, height/2
	outerRX, outerRY := width/2, height/2
	innerRX, innerRY := outerRX*innerRatio, outerRY*innerRatio
	startAngle := -math.Pi / 2
	cmds := make([]PathCommand, 0, points*2+2)
	for i := 0; i < points*2; i++ {
		angle := startAngle + float64(i)*math.Pi/float64(points)
		var x, y float64
		if i%2 == 0 {
			x = cx + outerRX*math.Cos(angle)
			y = cy + outerRY*math.Sin(angle)
		} else {
			x = cx + innerRX*math.Cos(angle)
			y = cy + innerRY*math.Sin(angle)
		}
		t := "L"
		if i == 0 {
			t = "M"
		}
		cmds = append(cmds, PathCommand{Type: t, Points: []Point{{X: x, Y: y}}})
	}
	cmds = append(cmds, PathCommand{Type: "Z"})
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// arrowPath creates a single-headed arrow pointing right, then rotated by angleDeg.
// angleDeg: 0=right, 90=down, 180=left, 270=up.
// adj1 (Adjustments[0]) = body fraction of total length (0→1, default 0.6): where arrowhead starts.
// adj2 (Adjustments[1]) = shaft height fraction (0→1, default 0.4).
func (pg *PresetGeometry) arrowPath(width, height float64, angleDeg float64) Path {
	// Build a right-pointing arrow, then rotate.
	// adj1 = fraction of width where arrowhead starts (shaft ends).
	bodyFrac := pg.adjOrDefault(0, 0.6)
	shaftFrac := pg.adjOrDefault(1, 0.4)
	headW := width * (1 - bodyFrac) // arrowhead occupies (1-adj1) of width from right
	tailH := height * shaftFrac
	tailY0 := (height - tailH) / 2
	pts := [][2]float64{
		{0, tailY0},
		{width - headW, tailY0},
		{width - headW, 0},
		{width, height / 2},
		{width - headW, height},
		{width - headW, tailY0 + tailH},
		{0, tailY0 + tailH},
	}
	pts = rotatePoints(pts, width, height, angleDeg)
	cmds := make([]PathCommand, 0, len(pts)+1)
	for i, p := range pts {
		t := "L"
		if i == 0 {
			t = "M"
		}
		cmds = append(cmds, PathCommand{Type: t, Points: []Point{{X: p[0], Y: p[1]}}})
	}
	cmds = append(cmds, PathCommand{Type: "Z"})
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// leftRightArrowPath creates a double-headed horizontal arrow.
func (pg *PresetGeometry) leftRightArrowPath(width, height float64) Path {
	headW := width * 0.3
	tailH := height * 0.4
	tailY0 := (height - tailH) / 2
	cmds := []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: height / 2}}},
		{Type: "L", Points: []Point{{X: headW, Y: 0}}},
		{Type: "L", Points: []Point{{X: headW, Y: tailY0}}},
		{Type: "L", Points: []Point{{X: width - headW, Y: tailY0}}},
		{Type: "L", Points: []Point{{X: width - headW, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height / 2}}},
		{Type: "L", Points: []Point{{X: width - headW, Y: height}}},
		{Type: "L", Points: []Point{{X: width - headW, Y: tailY0 + tailH}}},
		{Type: "L", Points: []Point{{X: headW, Y: tailY0 + tailH}}},
		{Type: "L", Points: []Point{{X: headW, Y: height}}},
		{Type: "Z"},
	}
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// upDownArrowPath creates a double-headed vertical arrow.
func (pg *PresetGeometry) upDownArrowPath(width, height float64) Path {
	headH := height * 0.3
	tailW := width * 0.4
	tailX0 := (width - tailW) / 2
	cmds := []PathCommand{
		{Type: "M", Points: []Point{{X: width / 2, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: headH}}},
		{Type: "L", Points: []Point{{X: tailX0 + tailW, Y: headH}}},
		{Type: "L", Points: []Point{{X: tailX0 + tailW, Y: height - headH}}},
		{Type: "L", Points: []Point{{X: width, Y: height - headH}}},
		{Type: "L", Points: []Point{{X: width / 2, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height - headH}}},
		{Type: "L", Points: []Point{{X: tailX0, Y: height - headH}}},
		{Type: "L", Points: []Point{{X: tailX0, Y: headH}}},
		{Type: "L", Points: []Point{{X: 0, Y: headH}}},
		{Type: "Z"},
	}
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// chevronPath creates a right-pointing chevron.
func (pg *PresetGeometry) chevronPath(width, height float64) Path {
	notch := width * 0.25
	if len(pg.Adjustments) > 0 {
		notch = pg.Adjustments[0] * width
	}
	cmds := []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - notch, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height / 2}}},
		{Type: "L", Points: []Point{{X: width - notch, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "L", Points: []Point{{X: notch, Y: height / 2}}},
		{Type: "Z"},
	}
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// piePath creates a pie-slice / sector shape.
func (pg *PresetGeometry) piePath(width, height float64) Path {
	// Default: quarter circle (90В°).
	startDeg := 0.0
	sweepDeg := 90.0
	if len(pg.Adjustments) >= 2 {
		startDeg = pg.Adjustments[0] * 360
		sweepDeg = pg.Adjustments[1] * 360
	}
	cx, cy := width/2, height/2
	rx, ry := width/2, height/2
	start := startDeg * math.Pi / 180
	end := (startDeg + sweepDeg) * math.Pi / 180
	x0 := cx + rx*math.Cos(start)
	y0 := cy + ry*math.Sin(start)
	x1 := cx + rx*math.Cos(end)
	y1 := cy + ry*math.Sin(end)
	largeArc := 0.0
	if sweepDeg > 180 {
		largeArc = 1
	}
	return Path{
		Commands: []PathCommand{
			{Type: "M", Points: []Point{{X: cx, Y: cy}}},
			{Type: "L", Points: []Point{{X: x0, Y: y0}}},
			{Type: "A", Points: []Point{{X: x1, Y: y1}}, Args: []float64{rx, ry, 0, largeArc, 1}},
			{Type: "Z"},
		},
		FillRule: "nonzero",
	}
}

// chordPath creates a chord (circular segment) shape.
func (pg *PresetGeometry) chordPath(width, height float64) Path {
	startDeg := 0.0
	sweepDeg := 90.0
	if len(pg.Adjustments) >= 2 {
		startDeg = pg.Adjustments[0] * 360
		sweepDeg = pg.Adjustments[1] * 360
	}
	cx, cy := width/2, height/2
	rx, ry := width/2, height/2
	start := startDeg * math.Pi / 180
	end := (startDeg + sweepDeg) * math.Pi / 180
	x0 := cx + rx*math.Cos(start)
	y0 := cy + ry*math.Sin(start)
	x1 := cx + rx*math.Cos(end)
	y1 := cy + ry*math.Sin(end)
	largeArc := 0.0
	if sweepDeg > 180 {
		largeArc = 1
	}
	return Path{
		Commands: []PathCommand{
			{Type: "M", Points: []Point{{X: x0, Y: y0}}},
			{Type: "A", Points: []Point{{X: x1, Y: y1}}, Args: []float64{rx, ry, 0, largeArc, 1}},
			{Type: "Z"},
		},
		FillRule: "nonzero",
	}
}

// rotatePoints rotates a set of (x,y) pairs around the centre of (width,height) by angleDeg.
func rotatePoints(pts [][2]float64, width, height, angleDeg float64) [][2]float64 {
	if angleDeg == 0 {
		return pts
	}
	cx, cy := width/2, height/2
	rad := angleDeg * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	out := make([][2]float64, len(pts))
	for i, p := range pts {
		dx, dy := p[0]-cx, p[1]-cy
		out[i] = [2]float64{
			cx + dx*cos - dy*sin,
			cy + dx*sin + dy*cos,
		}
	}
	return out
}

func rectSubpath(x, y, width, height float64) []PathCommand {
	return []PathCommand{
		{Type: "M", Points: []Point{{X: x, Y: y}}},
		{Type: "L", Points: []Point{{X: x + width, Y: y}}},
		{Type: "L", Points: []Point{{X: x + width, Y: y + height}}},
		{Type: "L", Points: []Point{{X: x, Y: y + height}}},
		{Type: "Z"},
	}
}

func ellipseSubpath(x, y, width, height float64) []PathCommand {
	rx := width / 2
	ry := height / 2
	cx := x + rx
	cy := y + ry
	return []PathCommand{
		{Type: "M", Points: []Point{{X: cx + rx, Y: cy}}},
		{Type: "A", Points: []Point{{X: cx, Y: cy - ry}}, Args: []float64{rx, ry, 0, 0, 1}},
		{Type: "A", Points: []Point{{X: cx - rx, Y: cy}}, Args: []float64{rx, ry, 0, 0, 1}},
		{Type: "A", Points: []Point{{X: cx, Y: cy + ry}}, Args: []float64{rx, ry, 0, 0, 1}},
		{Type: "A", Points: []Point{{X: cx + rx, Y: cy}}, Args: []float64{rx, ry, 0, 0, 1}},
		{Type: "Z"},
	}
}

// min returns the minimum of two float64 values.
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// PathCommand represents a single command in a vector path.
type PathCommand struct {
	Type   string    // "M", "L", "C", "Q", "A", "Z"
	Points []Point   // Coordinates for the command
	Args   []float64 // Additional arguments (e.g., for arc commands)
}

// Path represents a vector path composed of commands.
type Path struct {
	Commands []PathCommand
	FillRule string // "nonzero" or "evenodd"
}

// BoundingBox returns the bounding rectangle of a path.
func (p *Path) BoundingBox() Rect {
	if len(p.Commands) == 0 {
		return Rect{}
	}

	var points []Point
	for _, cmd := range p.Commands {
		points = append(points, cmd.Points...)
	}

	if len(points) == 0 {
		return Rect{}
	}

	minX, minY := points[0].X, points[0].Y
	maxX, maxY := minX, minY

	for _, pt := range points[1:] {
		if pt.X < minX {
			minX = pt.X
		}
		if pt.X > maxX {
			maxX = pt.X
		}
		if pt.Y < minY {
			minY = pt.Y
		}
		if pt.Y > maxY {
			maxY = pt.Y
		}
	}

	return Rect{
		X:      minX,
		Y:      minY,
		Width:  maxX - minX,
		Height: maxY - minY,
	}
}
