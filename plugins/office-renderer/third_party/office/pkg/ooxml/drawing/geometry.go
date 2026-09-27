package drawing

import (
	"math"
	"strings"
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

// wedgeRoundRectCalloutPath keeps the rounded host outline distinct from the
// rectangular wedge callout while retaining a filled triangular tail.
func (pg *PresetGeometry) wedgeRoundRectCalloutPath(width, height float64) Path {
	r := min(width, height) * 0.12
	tipX := width * (pg.adjOrDefault(0, 0.10)*2.0 - 0.50)
	tipY := height * (pg.adjOrDefault(1, 0.90)*1.5 - 0.25)
	attachY := height * pg.adjOrDefault(2, 0.62)
	tailHalf := height * 0.10
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: r, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - r, Y: 0}}},
		{Type: "Q", Points: []Point{{X: width, Y: 0}, {X: width, Y: r}}},
		{Type: "L", Points: []Point{{X: width, Y: height - r}}},
		{Type: "Q", Points: []Point{{X: width, Y: height}, {X: width - r, Y: height}}},
		{Type: "L", Points: []Point{{X: r, Y: height}}},
		{Type: "Q", Points: []Point{{X: 0, Y: height}, {X: 0, Y: height - r}}},
		{Type: "L", Points: []Point{{X: 0, Y: r}}},
		{Type: "Q", Points: []Point{{X: 0, Y: 0}, {X: r, Y: 0}}},
		{Type: "Z"},
		{Type: "M", Points: []Point{{X: 0, Y: max(0, attachY-tailHalf)}}},
		{Type: "L", Points: []Point{{X: tipX, Y: tipY}}},
		{Type: "L", Points: []Point{{X: 0, Y: min(height, attachY+tailHalf)}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// wedgeEllipseCalloutPath materializes an elliptical host and its wedge tail as
// compound geometry instead of silently falling back to a rectangle.
func (pg *PresetGeometry) wedgeEllipseCalloutPath(width, height float64) Path {
	p := pg.ellipsePath(width, height)
	tipX := width * (pg.adjOrDefault(0, 0.10)*2.0 - 0.50)
	tipY := height * (pg.adjOrDefault(1, 0.90)*1.5 - 0.25)
	attachX := width * 0.16
	attachY := height * pg.adjOrDefault(2, 0.68)
	tailHalf := height * 0.08
	p.Commands = append(p.Commands,
		PathCommand{Type: "M", Points: []Point{{X: attachX, Y: max(0, attachY-tailHalf)}}},
		PathCommand{Type: "L", Points: []Point{{X: tipX, Y: tipY}}},
		PathCommand{Type: "L", Points: []Point{{X: attachX, Y: min(height, attachY+tailHalf)}}},
		PathCommand{Type: "Z"},
	)
	return p
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
//
// Curved-arrow presets have three independent authored handles.  Keep those
// semantics in the preset geometry instead of reducing the shape to a fixed
// six-point polygon:
//
//	adj[0] - shaft/band thickness
//	adj[1] - arrow-head width
//	adj[2] - arrow-head length
//
// Legacy OfficeArt values have already been normalized by the presentation
// bridge before they reach this package.
func (pg *PresetGeometry) curvedArrowPath(width, height float64, angleDeg float64) Path {
	shaft := pg.adjOrDefault(0, 0.28)
	headWidth := pg.adjOrDefault(1, 0.58)
	headLength := pg.adjOrDefault(2, 0.28)

	// Keep the band and head well-formed even for extreme authored handles.
	shaft = math.Max(0.06, math.Min(0.72, shaft))
	headWidth = math.Max(shaft+0.08, math.Min(0.96, headWidth))
	headLength = math.Max(0.10, math.Min(0.48, headLength))

	cy := height * 0.58
	outerY := cy - height*shaft*0.5
	innerY := cy + height*shaft*0.5
	tipX := width
	neckX := width * (1 - headLength)
	headTop := cy - height*headWidth*0.5
	headBottom := cy + height*headWidth*0.5

	// The two cubic edges share the same curvature but remain separated by the
	// authored shaft thickness. This gives the preset a genuine curved band and
	// leaves a stable tangent where the arrow head begins.
	cmds := []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: innerY}}},
		{Type: "C", Points: []Point{{X: width * 0.18, Y: height * 0.06}, {X: neckX * 0.72, Y: height * 0.08}, {X: neckX, Y: outerY}}},
		{Type: "L", Points: []Point{{X: neckX, Y: headTop}}},
		{Type: "L", Points: []Point{{X: tipX, Y: cy}}},
		{Type: "L", Points: []Point{{X: neckX, Y: headBottom}}},
		{Type: "L", Points: []Point{{X: neckX, Y: innerY}}},
		{Type: "C", Points: []Point{{X: neckX * 0.70, Y: height * 0.24}, {X: width * 0.22, Y: height * 0.25}, {X: 0, Y: innerY}}},
		{Type: "Z"},
	}

	if angleDeg != 0 {
		for ci := range cmds {
			for pi := range cmds[ci].Points {
				p := cmds[ci].Points[pi]
				r := rotatePoints([][2]float64{{p.X, p.Y}}, width, height, angleDeg)[0]
				cmds[ci].Points[pi] = Point{X: r[0], Y: r[1]}
			}
		}
	}

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

// flowChartPunchedTapePath creates the punched-tape symbol: a closed band
// whose top and bottom edges undulate in phase. Unlike the document preset,
// neither edge is straight and the geometry remains symmetric on non-square bounds.
func (pg *PresetGeometry) flowChartPunchedTapePath(width, height float64) Path {
	wave := math.Min(height*0.16, width*0.08)
	midTop := wave
	midBottom := height - wave
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: midTop}}},
		{Type: "C", Points: []Point{{X: width * 0.25, Y: -wave}, {X: width * 0.25, Y: wave * 3}, {X: width * 0.5, Y: midTop}}},
		{Type: "C", Points: []Point{{X: width * 0.75, Y: -wave}, {X: width * 0.75, Y: wave * 3}, {X: width, Y: midTop}}},
		{Type: "L", Points: []Point{{X: width, Y: midBottom}}},
		{Type: "C", Points: []Point{{X: width * 0.75, Y: height - wave*3}, {X: width * 0.75, Y: height + wave}, {X: width * 0.5, Y: midBottom}}},
		{Type: "C", Points: []Point{{X: width * 0.25, Y: height - wave*3}, {X: width * 0.25, Y: height + wave}, {X: 0, Y: midBottom}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// flowChartMergePath creates the merge symbol. Its point faces downward;
// this is deliberately distinct from the ordinary upward-pointing triangle.
func (pg *PresetGeometry) flowChartMergePath(width, height float64) Path {
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width / 2, Y: height}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
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

// moonPath creates the DrawingML moon preset. The geometry is adjustment-
// sensitive: PowerPoint's default adj=50000 is materially wider than the
// old fixed cubic approximation, especially for the very tall/narrow source
// boxes produced when a moon is rotated into a shallow horizontal band.
// Formulas mirror ECMA-376 presetShapeDefinitions.xml.
func (pg *PresetGeometry) moonPath(width, height float64) Path {
	if width <= 0 || height <= 0 {
		return Path{FillRule: "nonzero"}
	}
	ss := math.Min(width, height)
	a := pg.adjOrDefault(0, 0.5)
	if a > 0.875 {
		a = 0.875
	}
	g0 := ss * a
	g0w := g0 * width / ss
	g1 := ss - g0
	if math.Abs(g1) < 1e-9 {
		g1 = 1e-9
	}
	g2 := g0 * g0 / g1
	g3 := ss * ss / g1
	g5 := 2*g3 - g2
	g6 := g5 - g0
	g6w := g6 * width / ss
	g8 := g5/2 - g0
	dy1 := g8 * (height / 2) / ss
	g17w := g6w - g0w
	g18w := g17w / 2

	// presetShapeDefinitions.xml authors the moon as two arcs that share the
	// same endpoints: (r,b) -> (r,t) for the outer rim, then (r,t) -> (r,b)
	// for the concave inner rim.  Earlier code tried to re-derive the second
	// endpoint from stAng1/swAng1 using ordinary atan2 semantics.  DrawingML's
	// guide `at2` convention differs, and that calculation could place the end
	// point outside the authored box for tall/narrow moons (the common case when
	// a moon is rotated into a shallow atmospheric band).  The endpoint is
	// invariant in the preset definition, so materialize it directly and retain
	// only the guide-derived radii.
	outerStart := Point{X: width, Y: height}
	outerEnd := Point{X: width, Y: 0}
	rx := math.Abs(g18w)
	ry := math.Abs(dy1)

	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{outerStart}},
		{Type: "A", Points: []Point{outerEnd}, Args: []float64{width, height / 2, 0, 0, 1}},
		{Type: "A", Points: []Point{outerStart}, Args: []float64{rx, ry, 0, 0, 0}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
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
	case "teardrop":
		return pg.teardropPath(width, height), nil
	case "swooshArrow":
		return pg.swooshArrowPath(width, height), nil
	case "leftCircularArrow":
		return pg.circularArrowVariantPath(width, height, false), nil
	case "leftRightCircularArrow":
		return pg.circularArrowVariantPath(width, height, true), nil
	case "corner":
		return pg.cornerPath(width, height), nil
	case "diagStripe":
		return pg.diagStripePath(width, height), nil
	case "pie":
		return pg.piePath(width, height), nil
	case "arc":
		return pg.arcPath(width, height), nil
	case "chord":
		return pg.chordPath(width, height), nil
	case "wedgeRectCallout":
		return pg.wedgeCalloutPath(width, height), nil
	case "wedgeRoundRectCallout":
		return pg.wedgeRoundRectCalloutPath(width, height), nil
	case "wedgeEllipseCallout":
		return pg.wedgeEllipseCalloutPath(width, height), nil
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
	case "flowChartMultidocument":
		return pg.flowChartMultidocumentPath(width, height), nil
	case "flowChartInputOutput":
		return pg.flowChartInputOutputPath(width, height), nil
	case "flowChartPredefinedProcess":
		return pg.flowChartPredefinedProcessPath(width, height), nil
	case "flowChartInternalStorage":
		return pg.flowChartInternalStoragePath(width, height), nil
	case "flowChartManualOperation":
		return pg.flowChartManualOperationPath(width, height), nil
	case "flowChartPunchedCard":
		return pg.flowChartPunchedCardPath(width, height), nil
	case "flowChartSort":
		return pg.flowChartSortPath(width, height), nil
	case "flowChartSummingJunction":
		return pg.flowChartJunctionPath(width, height, true), nil
	case "flowChartOr":
		return pg.flowChartJunctionPath(width, height, false), nil
	case "flowChartCollate":
		return pg.flowChartCollatePath(width, height), nil
	case "flowChartExtract":
		return pg.flowChartExtractPath(width, height), nil
	case "flowChartStoredData":
		return pg.flowChartStoredDataPath(width, height), nil
	case "flowChartOfflineStorage":
		return pg.flowChartOfflineStoragePath(width, height), nil
	case "flowChartOnlineStorage":
		return pg.flowChartOnlineStoragePath(width, height), nil
	case "flowChartDelay":
		return pg.flowChartDelayPath(width, height), nil
	case "flowChartMagneticTape":
		return pg.flowChartMagneticTapePath(width, height), nil
	case "flowChartMagneticDisk":
		return pg.flowChartMagneticDiskPath(width, height), nil
	case "flowChartMagneticDrum":
		return pg.flowChartMagneticDrumPath(width, height), nil
	case "flowChartDisplay":
		return pg.flowChartDisplayPath(width, height), nil
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
	case "notch":
		return pg.notchPath(width, height), nil
	case "notchedRightArrow":
		return pg.notchedRightArrowPath(width, height), nil
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
	case "leftBracket":
		return pg.bracketPath(width, height, false), nil
	case "rightBracket":
		return pg.bracketPath(width, height, true), nil
	case "bracePair":
		return pg.bracePairPath(width, height), nil
	case "leftBrace":
		return pg.bracePath(width, height, false), nil
	case "rightBrace":
		return pg.bracePath(width, height, true), nil
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
		return pg.flowChartPunchedTapePath(width, height), nil
	case "flowChartMerge":
		return pg.flowChartMergePath(width, height), nil
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
	case "upDownArrowCallout":
		return pg.upDownArrowCalloutPath(width, height), nil
	case "quadArrow":
		return pg.quadArrowPath(width, height), nil
	case "quadArrowCallout":
		return pg.quadArrowCalloutPath(width, height), nil
	case "leftUpArrow":
		return pg.leftUpArrowPath(width, height), nil
	case "bentUpArrow":
		return pg.bentUpArrowPath(width, height), nil
	case "bentArrow":
		return pg.bentArrowPath(width, height), nil
	case "uturnArrow":
		return pg.uturnArrowPath(width, height), nil
	case "stripedRightArrow":
		return pg.stripedRightArrowPath(width, height), nil
	case "circularArrow":
		return pg.circularArrowPath(width, height), nil
	case "wave":
		return pg.wavePath(width, height), nil
	case "callout1", "callout2", "callout3", "callout4",
		"accentCallout1", "accentCallout2", "accentCallout3", "accentCallout4",
		"borderCallout1", "borderCallout2", "borderCallout3",
		"accentBorderCallout1", "accentBorderCallout2", "accentBorderCallout3":
		return pg.lineCalloutPath(width, height, pg.Name), nil
	// Ribbons / banners
	case "ribbon":
		return pg.ribbonPath(width, height, false), nil
	case "ribbon2":
		return pg.ribbonPath(width, height, true), nil
	case "ellipseRibbon":
		return pg.ellipseRibbonPath(width, height, false), nil
	case "ellipseRibbon2":
		return pg.ellipseRibbonPath(width, height, true), nil
	case "horizontalScroll":
		return pg.scrollPath(width, height, false), nil
	case "verticalScroll":
		return pg.scrollPath(width, height, true), nil
	// Other decorative shapes
	case "star16":
		return pg.starPath(width, height, 16, pg.adjOrDefault(0, 0.6)), nil
	case "star24":
		return pg.starPath(width, height, 24, pg.adjOrDefault(0, 0.7)), nil
	case "star32":
		return pg.starPath(width, height, 32, pg.adjOrDefault(0, 0.75)), nil
	case "irregularSeal1":
		return pg.irregularSealPath(width, height, false), nil
	case "irregularSeal2":
		return pg.irregularSealPath(width, height, true), nil
	case "cross":
		return pg.crossPath(width, height), nil
	case "plus":
		return pg.crossPath(width, height), nil
	case "mathPlus":
		return pg.mathPlusPath(width, height), nil
	case "mathMinus":
		return pg.mathMinusPath(width, height), nil
	case "mathMultiply":
		return pg.mathMultiplyPath(width, height), nil
	case "mathDivide":
		return pg.mathDividePath(width, height), nil
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
	// DrawingML homePlate defines its tip depth from ss (the short side),
	// not from the shape width.  The preset default is adj=50000, i.e. half
	// the short side.  Using a percentage of width makes wide SmartArt banners
	// develop a long, shallow wedge instead of PowerPoint's steep shoulder.
	shortSide := min(width, height)
	tipW := shortSide * 0.5
	if len(pg.Adjustments) > 0 {
		tipW = pg.Adjustments[0] * shortSide
	}
	tipW = max(0, min(tipW, width))
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
//
// The canonical PresetGeometry contract represents angular adjustments as
// normalized turns: 0.0 == 0 degrees, 0.25 == 90 degrees, 0.5 == 180
// degrees, and so on.  The first two adjustments are the authored start and
// end angles; the third is radial thickness as a fraction of the outer
// radius.  Keeping start/end distinct is important: treating the first handle
// as a generic sweep loses the rotation of the authored block arc.
func (pg *PresetGeometry) blockArcPath(width, height float64) Path {
	startTurn := pg.adjOrDefault(0, 0.5) // 180 degrees
	endTurn := pg.adjOrDefault(1, 0.0)   //   0 degrees
	thickness := pg.adjOrDefault(2, 0.25)

	// Angles are cyclic, not clampable geometry percentages.  Normalize them
	// into one turn while preserving their relative authored orientation.
	startTurn = startTurn - math.Floor(startTurn)
	endTurn = endTurn - math.Floor(endTurn)
	sweepTurn := endTurn - startTurn
	if sweepTurn <= 0 {
		sweepTurn += 1
	}
	// A coincident pair of handles denotes a complete ring.  SVG cannot encode
	// a full ellipse with one A command, so keep it infinitesimally below one
	// turn here; the two-arc materialization below handles the normal case.
	if sweepTurn >= 1 {
		sweepTurn = 1 - 1e-9
	}

	// A block arc cannot be thicker than its radius.  Zero remains valid and
	// degenerates to the outer arc boundary without inverting the inner arc.
	thickness = math.Max(0, math.Min(1, thickness))
	innerRatio := 1 - thickness

	rx, ry := width/2, height/2
	cx, cy := width/2, height/2
	irx, iry := rx*innerRatio, ry*innerRatio

	startRad := startTurn * 2 * math.Pi
	endRad := startRad + sweepTurn*2*math.Pi

	ox1 := cx + rx*math.Cos(startRad)
	oy1 := cy + ry*math.Sin(startRad)
	ox2 := cx + rx*math.Cos(endRad)
	oy2 := cy + ry*math.Sin(endRad)
	ix1 := cx + irx*math.Cos(endRad)
	iy1 := cy + iry*math.Sin(endRad)
	ix2 := cx + irx*math.Cos(startRad)
	iy2 := cy + iry*math.Sin(startRad)

	largeArc := 0.0
	if sweepTurn > 0.5 {
		largeArc = 1
	}
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: ox1, Y: oy1}}},
		{Type: "A", Points: []Point{{X: ox2, Y: oy2}}, Args: []float64{rx, ry, 0, largeArc, 1}},
		{Type: "L", Points: []Point{{X: ix1, Y: iy1}}},
		{Type: "A", Points: []Point{{X: ix2, Y: iy2}}, Args: []float64{irx, iry, 0, largeArc, 0}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// bracketPath materializes a single filled bracket.
func (pg *PresetGeometry) bracketPath(width, height float64, right bool) Path {
	arm := width * math.Max(0.08, math.Min(0.45, pg.adjOrDefault(0, 0.18)))
	thick := math.Min(width, height) * 0.08
	if thick > arm {
		thick = arm
	}
	var pts []Point
	if !right {
		pts = []Point{{arm, 0}, {0, 0}, {0, height}, {arm, height}, {arm, height - thick}, {thick, height - thick}, {thick, thick}, {arm, thick}}
	} else {
		pts = []Point{{width - arm, 0}, {width, 0}, {width, height}, {width - arm, height}, {width - arm, height - thick}, {width - thick, height - thick}, {width - thick, thick}, {width - arm, thick}}
	}
	return polygonPath(pts)
}

func (pg *PresetGeometry) bracketPairPath(width, height float64) Path {
	left := pg.bracketPath(width, height, false)
	right := pg.bracketPath(width, height, true)
	return Path{Commands: append(left.Commands, right.Commands...), FillRule: "nonzero"}
}

// bracePath creates a closed, filled curly brace with distinct outer/inner contours.
func (pg *PresetGeometry) bracePath(width, height float64, right bool) Path {
	depth := width * math.Max(0.12, math.Min(0.48, pg.adjOrDefault(0, 0.28)))
	t := math.Min(width, height) * 0.055
	if t > depth*0.45 {
		t = depth * 0.45
	}
	mid, shoulder := height/2, height*0.18
	cmds := []PathCommand{
		{Type: "M", Points: []Point{{depth, 0}}}, {Type: "Q", Points: []Point{{depth * .35, 0}, {depth * .35, shoulder}}},
		{Type: "L", Points: []Point{{depth * .35, mid - shoulder}}}, {Type: "Q", Points: []Point{{depth * .35, mid - t}, {0, mid}}},
		{Type: "Q", Points: []Point{{depth * .35, mid + t}, {depth * .35, mid + shoulder}}}, {Type: "L", Points: []Point{{depth * .35, height - shoulder}}},
		{Type: "Q", Points: []Point{{depth * .35, height}, {depth, height}}}, {Type: "L", Points: []Point{{depth, height - t}}},
		{Type: "Q", Points: []Point{{depth*.35 + t, height - t}, {depth*.35 + t, height - shoulder}}}, {Type: "L", Points: []Point{{depth*.35 + t, mid + shoulder}}},
		{Type: "Q", Points: []Point{{depth*.35 + t, mid + t*1.6}, {t * 1.8, mid}}}, {Type: "Q", Points: []Point{{depth*.35 + t, mid - t*1.6}, {depth*.35 + t, mid - shoulder}}},
		{Type: "L", Points: []Point{{depth*.35 + t, shoulder}}}, {Type: "Q", Points: []Point{{depth*.35 + t, t}, {depth, t}}}, {Type: "Z"}}
	if right {
		for i := range cmds {
			for j := range cmds[i].Points {
				cmds[i].Points[j].X = width - cmds[i].Points[j].X
			}
		}
	}
	return Path{Commands: cmds, FillRule: "nonzero"}
}
func (pg *PresetGeometry) bracePairPath(width, height float64) Path {
	left := pg.bracePath(width, height, false)
	right := pg.bracePath(width, height, true)
	return Path{Commands: append(left.Commands, right.Commands...), FillRule: "nonzero"}
}

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

// bentUpArrowPath owns the upward-turning L-arrow preset.
func (pg *PresetGeometry) bentUpArrowPath(width, height float64) Path {
	shaft := height * math.Max(.08, math.Min(.45, pg.adjOrDefault(0, .22)))
	head := width * math.Max(.18, math.Min(.65, pg.adjOrDefault(1, .34)))
	bendX := width * math.Max(.15, math.Min(.72, pg.adjOrDefault(2, .38)))
	if head < shaft*1.6 {
		head = shaft * 1.6
	}
	return polygonPath([]Point{{0, height}, {bendX + shaft, height}, {bendX + shaft, head}, {bendX + head, head}, {bendX, 0}, {bendX - head, head}, {bendX, head}, {bendX, height - shaft}, {0, height - shaft}})
}
func (pg *PresetGeometry) bentArrowPath(width, height float64) Path {
	shaft := math.Min(width, height) * math.Max(.08, math.Min(.38, pg.adjOrDefault(0, .20)))
	headLen := width * math.Max(.16, math.Min(.45, pg.adjOrDefault(1, .28)))
	bendY := height * math.Max(.25, math.Min(.78, pg.adjOrDefault(2, .58)))
	headHalf := math.Min(height*.45, shaft*1.65)
	base := width - headLen
	return polygonPath([]Point{{0, height}, {shaft, height}, {shaft, bendY + shaft/2}, {base, bendY + shaft/2}, {base, bendY + headHalf}, {width, bendY}, {base, bendY - headHalf}, {base, bendY - shaft/2}, {0, bendY - shaft/2}})
}
func (pg *PresetGeometry) uturnArrowPath(width, height float64) Path {
	shaft := math.Min(width, height) * math.Max(.07, math.Min(.28, pg.adjOrDefault(0, .14)))
	headLen := height * math.Max(.14, math.Min(.38, pg.adjOrDefault(1, .24)))
	r := math.Min(width*.42, height*.36)
	cx := width / 2
	left, right := cx-r, cx+r
	top := headLen
	innerR := math.Max(r-shaft, r*.35)
	return Path{Commands: []PathCommand{{Type: "M", Points: []Point{{left, height}}}, {Type: "L", Points: []Point{{left, top + r}}}, {Type: "A", Points: []Point{{right, top + r}}, Args: []float64{r, r, 0, 0, 1}}, {Type: "L", Points: []Point{{right, top + headLen}}}, {Type: "L", Points: []Point{{right + headLen, top + headLen}}}, {Type: "L", Points: []Point{{right, 0}}}, {Type: "L", Points: []Point{{right - headLen, top + headLen}}}, {Type: "L", Points: []Point{{right - shaft, top + headLen}}}, {Type: "L", Points: []Point{{right - shaft, top + r}}}, {Type: "A", Points: []Point{{left + shaft, top + r}}, Args: []float64{innerR, innerR, 0, 0, 0}}, {Type: "L", Points: []Point{{left + shaft, height}}}, {Type: "Z"}}, FillRule: "nonzero"}
}
func (pg *PresetGeometry) notchedRightArrowPath(width, height float64) Path {
	head := width * math.Max(.18, math.Min(.55, pg.adjOrDefault(0, .32)))
	sh := height * math.Max(.08, math.Min(.42, pg.adjOrDefault(1, .22)))
	notch := math.Min(width-head, width*math.Max(.04, math.Min(.30, pg.adjOrDefault(2, .14))))
	cy := height / 2
	return polygonPath([]Point{{0, cy - sh}, {width - head, cy - sh}, {width - head, 0}, {width, cy}, {width - head, height}, {width - head, cy + sh}, {0, cy + sh}, {notch, cy}})
}
func (pg *PresetGeometry) stripedRightArrowPath(width, height float64) Path {
	gap := width * math.Max(.015, math.Min(.08, pg.adjOrDefault(2, .035)))
	stripe := width * math.Max(.025, math.Min(.10, pg.adjOrDefault(3, .05)))
	mainX := 2 * (stripe + gap)
	arrowPreset := &PresetGeometry{Name: "rightArrow", Adjustments: pg.Adjustments}
	base := arrowPreset.arrowPath(width-mainX, height, 0)
	for i := range base.Commands {
		for j := range base.Commands[i].Points {
			base.Commands[i].Points[j].X += mainX
		}
	}
	cy := height / 2
	sh := height * math.Max(.08, math.Min(.42, pg.adjOrDefault(1, .22)))
	cmds := rectSubpath(0, cy-sh, stripe, 2*sh)
	cmds = append(cmds, rectSubpath(stripe+gap, cy-sh, stripe, 2*sh)...)
	cmds = append(cmds, base.Commands...)
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// lineCalloutPath creates the rectangular DrawingML/OfficeArt callout family.
//
// Unlike wedge callouts, the numbered callouts keep the text box rectangular
// and draw a one-, two-, or three-segment leader outside/through the box. The
// accent variants add the characteristic accent bar at the leader attachment.
// Border variants share the same geometry: their border/no-border distinction
// is a paint semantic, not a different outline.
func (pg *PresetGeometry) lineCalloutPath(width, height float64, preset string) Path {
	segments := 1
	if strings.Contains(preset, "2") {
		segments = 2
	} else if strings.Contains(preset, "3") || strings.Contains(preset, "4") {
		segments = 3
	}
	accent := strings.HasPrefix(preset, "accent") || strings.HasPrefix(preset, "accentBorder")

	// The first two handles own the external leader tip. Additional handles own
	// the elbows. Defaults intentionally put the tip outside the lower-left of
	// the host so the callout remains recognizable on non-square bounds.
	tipX := width * (pg.adjOrDefault(0, 0.10)*2.0 - 0.50)
	tipY := height * (pg.adjOrDefault(1, 0.90)*1.5 - 0.25)
	attachX := width * pg.adjOrDefault(2, 0.22)
	attachY := height * pg.adjOrDefault(3, 0.72)

	cmds := []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "Z"},
		{Type: "M", Points: []Point{{X: tipX, Y: tipY}}},
	}

	switch segments {
	case 1:
		cmds = append(cmds, PathCommand{Type: "L", Points: []Point{{X: attachX, Y: attachY}}})
	case 2:
		elbowX := width * pg.adjOrDefault(4, 0.08)
		cmds = append(cmds,
			PathCommand{Type: "L", Points: []Point{{X: elbowX, Y: attachY}}},
			PathCommand{Type: "L", Points: []Point{{X: attachX, Y: attachY}}},
		)
	default:
		elbowX := width * pg.adjOrDefault(4, 0.08)
		elbowY := height * pg.adjOrDefault(5, 0.88)
		cmds = append(cmds,
			PathCommand{Type: "L", Points: []Point{{X: elbowX, Y: elbowY}}},
			PathCommand{Type: "L", Points: []Point{{X: elbowX, Y: attachY}}},
			PathCommand{Type: "L", Points: []Point{{X: attachX, Y: attachY}}},
		)
	}

	if accent {
		barHalf := height * 0.16
		cmds = append(cmds,
			PathCommand{Type: "M", Points: []Point{{X: attachX, Y: max(0, attachY-barHalf)}}},
			PathCommand{Type: "L", Points: []Point{{X: attachX, Y: min(height, attachY+barHalf)}}},
		)
	}

	return Path{Commands: cmds, FillRule: "nonzero"}
}

// arrowCalloutPath creates an arrow callout with a rectangular label body and a directional arrow.
func (pg *PresetGeometry) arrowCalloutPath(width, height, angleDeg float64) Path {
	// Legacy OfficeArt arrow callouts expose three useful handles.  Keep their
	// interpretation in preset geometry: body extent, shaft thickness and head
	// length.  Values are already normalized by the legacy bridge.
	bodyRatio := pg.adjOrDefault(0, 0.48)
	shaftRatio := pg.adjOrDefault(1, 0.22)
	headRatio := pg.adjOrDefault(2, 0.24)
	// A callout must retain both a label body and an arrow.  Constrain authored
	// extremes instead of allowing the polygon to cross itself.
	bodyRatio = math.Max(0.15, math.Min(0.75, bodyRatio))
	shaftRatio = math.Max(0.04, math.Min(0.75, shaftRatio))
	headRatio = math.Max(0.08, math.Min(0.45, headRatio))

	if angleDeg == 0 || angleDeg == 180 {
		bodyW := width * bodyRatio
		bodyH := height * math.Max(0.36, math.Min(0.9, bodyRatio+0.08))
		bodyY := (height - bodyH) / 2
		shaftH := height * shaftRatio
		shaftY := (height - shaftH) / 2
		headLen := width * headRatio
		if angleDeg == 180 {
			bodyX := width - bodyW
			headBaseX := headLen
			cmds := rectSubpath(bodyX, bodyY, bodyW, bodyH)
			cmds = append(cmds,
				PathCommand{Type: "M", Points: []Point{{X: bodyX, Y: shaftY}}},
				PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: shaftY}}},
				PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: 0}}},
				PathCommand{Type: "L", Points: []Point{{X: 0, Y: height / 2}}},
				PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: height}}},
				PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: shaftY + shaftH}}},
				PathCommand{Type: "L", Points: []Point{{X: bodyX, Y: shaftY + shaftH}}}, PathCommand{Type: "Z"})
			return Path{Commands: cmds, FillRule: "nonzero"}
		}
		bodyX := 0.0
		headBaseX := width - headLen
		cmds := rectSubpath(bodyX, bodyY, bodyW, bodyH)
		cmds = append(cmds,
			PathCommand{Type: "M", Points: []Point{{X: bodyW, Y: shaftY}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: shaftY}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: 0}}},
			PathCommand{Type: "L", Points: []Point{{X: width, Y: height / 2}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: height}}},
			PathCommand{Type: "L", Points: []Point{{X: headBaseX, Y: shaftY + shaftH}}},
			PathCommand{Type: "L", Points: []Point{{X: bodyW, Y: shaftY + shaftH}}}, PathCommand{Type: "Z"})
		return Path{Commands: cmds, FillRule: "nonzero"}
	}

	bodyH := height * bodyRatio
	bodyW := width * math.Max(0.36, math.Min(0.9, bodyRatio+0.08))
	bodyX := (width - bodyW) / 2
	shaftW := width * shaftRatio
	shaftX := (width - shaftW) / 2
	headLen := height * headRatio
	if angleDeg == 270 {
		bodyY := height - bodyH
		headBaseY := headLen
		cmds := rectSubpath(bodyX, bodyY, bodyW, bodyH)
		cmds = append(cmds,
			PathCommand{Type: "M", Points: []Point{{X: shaftX, Y: bodyY}}}, PathCommand{Type: "L", Points: []Point{{X: shaftX, Y: headBaseY}}},
			PathCommand{Type: "L", Points: []Point{{X: 0, Y: headBaseY}}}, PathCommand{Type: "L", Points: []Point{{X: width / 2, Y: 0}}},
			PathCommand{Type: "L", Points: []Point{{X: width, Y: headBaseY}}}, PathCommand{Type: "L", Points: []Point{{X: shaftX + shaftW, Y: headBaseY}}},
			PathCommand{Type: "L", Points: []Point{{X: shaftX + shaftW, Y: bodyY}}}, PathCommand{Type: "Z"})
		return Path{Commands: cmds, FillRule: "nonzero"}
	}
	bodyY := 0.0
	headBaseY := height - headLen
	cmds := rectSubpath(bodyX, bodyY, bodyW, bodyH)
	cmds = append(cmds,
		PathCommand{Type: "M", Points: []Point{{X: shaftX, Y: bodyH}}}, PathCommand{Type: "L", Points: []Point{{X: shaftX, Y: headBaseY}}},
		PathCommand{Type: "L", Points: []Point{{X: 0, Y: headBaseY}}}, PathCommand{Type: "L", Points: []Point{{X: width / 2, Y: height}}},
		PathCommand{Type: "L", Points: []Point{{X: width, Y: headBaseY}}}, PathCommand{Type: "L", Points: []Point{{X: shaftX + shaftW, Y: headBaseY}}},
		PathCommand{Type: "L", Points: []Point{{X: shaftX + shaftW, Y: bodyH}}}, PathCommand{Type: "Z"})
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// leftRightArrowCalloutPath creates a center callout box with arrows extending left and right.
func (pg *PresetGeometry) leftRightArrowCalloutPath(width, height float64) Path {
	// Multi-direction callouts use the same normalized ownership contract as
	// directional callouts: adj1 owns the central callout extent, adj2 owns the
	// arrow shaft thickness, and adj3 owns arrow-head length. Keep the central
	// box large enough to remain a real callout and keep opposing heads from
	// crossing it at extreme legacy handle positions.
	bodyRatio := pg.adjOrDefault(0, 0.34)
	bodyRatio = math.Max(0.12, math.Min(0.82, bodyRatio))
	shaftRatio := pg.adjOrDefault(1, 0.22)
	shaftRatio = math.Max(0.04, math.Min(bodyRatio, shaftRatio))
	headRatio := pg.adjOrDefault(2, 0.20)
	headRatio = math.Max(0.04, math.Min((1-bodyRatio)/2, headRatio))

	centerW := width * bodyRatio
	centerH := height * math.Max(0.36, math.Min(0.90, bodyRatio+0.22))
	centerX := (width - centerW) / 2
	centerY := (height - centerH) / 2
	barH := height * shaftRatio
	barY := (height - barH) / 2
	headW := width * headRatio

	cmds := rectSubpath(centerX, centerY, centerW, centerH)
	cmds = append(cmds,
		PathCommand{Type: "M", Points: []Point{{X: 0, Y: height / 2}}},
		PathCommand{Type: "L", Points: []Point{{X: headW, Y: barY}}},
		PathCommand{Type: "L", Points: []Point{{X: centerX, Y: barY}}},
		PathCommand{Type: "L", Points: []Point{{X: centerX, Y: centerY}}},
		PathCommand{Type: "L", Points: []Point{{X: centerX + centerW, Y: centerY}}},
		PathCommand{Type: "L", Points: []Point{{X: centerX + centerW, Y: barY}}},
		PathCommand{Type: "L", Points: []Point{{X: width - headW, Y: barY}}},
		PathCommand{Type: "L", Points: []Point{{X: width, Y: height / 2}}},
		PathCommand{Type: "L", Points: []Point{{X: width - headW, Y: barY + barH}}},
		PathCommand{Type: "L", Points: []Point{{X: centerX + centerW, Y: barY + barH}}},
		PathCommand{Type: "L", Points: []Point{{X: centerX + centerW, Y: centerY + centerH}}},
		PathCommand{Type: "L", Points: []Point{{X: centerX, Y: centerY + centerH}}},
		PathCommand{Type: "L", Points: []Point{{X: centerX, Y: barY + barH}}},
		PathCommand{Type: "L", Points: []Point{{X: headW, Y: barY + barH}}},
		PathCommand{Type: "Z"},
	)
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// quadArrowPath creates a four-direction arrow with a central body.
func (pg *PresetGeometry) quadArrowPath(width, height float64) Path {
	// adj1 owns head reach and adj2 owns the cross/shaft thickness.  The same
	// geometry is reused by quadArrowCallout so both presets respond to legacy
	// OfficeArt handles consistently.
	headRatio := pg.adjOrDefault(0, 0.22)
	headRatio = math.Max(0.04, math.Min(0.42, headRatio))
	shaftRatio := pg.adjOrDefault(1, 0.24)
	shaftRatio = math.Max(0.04, math.Min(0.58, shaftRatio))
	cx, cy := width/2, height/2
	headW := width * headRatio
	headH := height * headRatio
	bodyW := width * shaftRatio
	bodyH := height * shaftRatio
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

func (pg *PresetGeometry) quadArrowCalloutPath(width, height float64) Path {
	quad := pg.quadArrowPath(width, height)
	// The third handle owns the central callout box. Constrain it against the
	// shaft thickness so the box remains contained by the four-direction body.
	boxRatio := pg.adjOrDefault(2, 0.20)
	shaftRatio := pg.adjOrDefault(1, 0.24)
	boxRatio = math.Max(0.06, math.Min(math.Max(0.06, shaftRatio), boxRatio))
	centerW := width * boxRatio
	centerH := height * boxRatio
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

// mathPlusPath creates the DrawingML mathematical plus symbol.  Unlike the
// generic cross preset, its arm thickness is adjustment-owned and symmetric.
func (pg *PresetGeometry) mathPlusPath(width, height float64) Path {
	thickness := clamp01(pg.adjOrDefault(0, 0.235))
	if thickness < 0.04 {
		thickness = 0.04
	}
	if thickness > 0.48 {
		thickness = 0.48
	}
	armW := width * thickness
	armH := height * thickness
	x0 := (width - armW) / 2
	y0 := (height - armH) / 2
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: x0, Y: 0}}}, {Type: "L", Points: []Point{{X: x0 + armW, Y: 0}}},
		{Type: "L", Points: []Point{{X: x0 + armW, Y: y0}}}, {Type: "L", Points: []Point{{X: width, Y: y0}}},
		{Type: "L", Points: []Point{{X: width, Y: y0 + armH}}}, {Type: "L", Points: []Point{{X: x0 + armW, Y: y0 + armH}}},
		{Type: "L", Points: []Point{{X: x0 + armW, Y: height}}}, {Type: "L", Points: []Point{{X: x0, Y: height}}},
		{Type: "L", Points: []Point{{X: x0, Y: y0 + armH}}}, {Type: "L", Points: []Point{{X: 0, Y: y0 + armH}}},
		{Type: "L", Points: []Point{{X: 0, Y: y0}}}, {Type: "L", Points: []Point{{X: x0, Y: y0}}}, {Type: "Z"},
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

// mathDividePath creates a division sign as three independent filled regions.
// Keeping the dots separate is important for even-odd fill and SVG conversion.
func (pg *PresetGeometry) mathDividePath(width, height float64) Path {
	barH := height * 0.16
	barW := width * 0.72
	barX := (width - barW) / 2
	d := math.Min(width, height) * 0.14
	dx := (width - d) / 2
	cmds := rectSubpath(barX, (height-barH)/2, barW, barH)
	cmds = append(cmds, ellipseSubpath(dx, height*0.12, d, d)...)
	cmds = append(cmds, ellipseSubpath(dx, height*0.88-d, d, d)...)
	return Path{Commands: cmds, FillRule: "evenodd"}
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
func (pg *PresetGeometry) irregularSealPath(width, height float64, alternate bool) Path {
	// The two seal presets are authored as different irregular burst outlines.
	// Keep their topology deterministic rather than aliasing both to star12.
	cx, cy := width/2, height/2
	rx, ry := width/2, height/2
	outer := []float64{1.00, .88, .98, .84, 1.00, .86, .96, .90, 1.00, .85, .97, .89, 1.00, .87, .99, .83}
	inner := []float64{.58, .52, .61, .49, .57, .54, .60, .50, .59, .51, .62, .48, .56, .55, .60, .50}
	if alternate {
		outer = []float64{.91, 1.00, .85, .97, .89, 1.00, .84, .98, .92, 1.00, .86, .96, .90, .99, .83, .97}
		inner = []float64{.49, .60, .53, .57, .50, .62, .48, .58, .52, .59, .51, .61, .47, .56, .54, .60}
	}
	cmds := make([]PathCommand, 0, 33)
	for i := 0; i < 32; i++ {
		a := -math.Pi/2 + float64(i)*math.Pi/16
		r := outer[i/2]
		if i%2 == 1 {
			r = inner[i/2]
		}
		pt := Point{X: cx + rx*r*math.Cos(a), Y: cy + ry*r*math.Sin(a)}
		t := "L"
		if i == 0 {
			t = "M"
		}
		cmds = append(cmds, PathCommand{Type: t, Points: []Point{pt}})
	}
	cmds = append(cmds, PathCommand{Type: "Z"})
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// ribbonPath materializes the authored ribbon depth and tail inset.  The first
// adjustment owns the fold/depth and the second owns the tail inset.  ribbon2
// mirrors the fold to the opposite edge instead of sharing ribbon's polygon.
func (pg *PresetGeometry) ribbonPath(width, height float64, inverted bool) Path {
	depth := clamp01(pg.adjOrDefault(0, 0.30)) * height
	tail := clamp01(pg.adjOrDefault(1, 0.25)) * width * 0.5
	// Keep a visible centre banner even for extreme authored handles.
	if tail > width*0.45 {
		tail = width * 0.45
	}
	foldY := height - depth
	if inverted {
		foldY = depth
	}
	midY := height / 2
	cmds := []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - tail, Y: midY}}},
		{Type: "L", Points: []Point{{X: width, Y: height}}},
		{Type: "L", Points: []Point{{X: width * 0.5, Y: foldY}}},
		{Type: "L", Points: []Point{{X: 0, Y: height}}},
		{Type: "L", Points: []Point{{X: tail, Y: midY}}},
		{Type: "Z"},
	}
	return Path{Commands: cmds, FillRule: "nonzero"}
}

// ellipseRibbonPath keeps the ribbon family distinct from the polygonal banner:
// its first adjustment controls the bow depth and its second the end inset.
func (pg *PresetGeometry) ellipseRibbonPath(width, height float64, inverted bool) Path {
	depth := clamp01(pg.adjOrDefault(0, 0.25)) * height
	inset := clamp01(pg.adjOrDefault(1, 0.15)) * width * 0.5
	if inset > width*0.4 {
		inset = width * 0.4
	}
	top, bottom := depth, height-depth
	if top > bottom {
		top, bottom = height/2, height/2
	}
	if inverted {
		top, bottom = height-bottom, height-top
	}
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: height / 2}}},
		{Type: "C", Points: []Point{{X: inset, Y: top}, {X: width - inset, Y: top}, {X: width, Y: height / 2}}},
		{Type: "C", Points: []Point{{X: width - inset, Y: bottom}, {X: inset, Y: bottom}, {X: 0, Y: height / 2}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// scrollPath gives horizontal/vertical scrolls their own rolled-edge geometry.
// The first adjustment controls roll radius; verticalScroll is a rotation of the
// same canonical contract rather than an unrelated approximation.
func (pg *PresetGeometry) scrollPath(width, height float64, vertical bool) Path {
	if vertical {
		base := pg.scrollPath(height, width, false)
		return rotateSimplePath(base, height, width, 90)
	}
	r := clamp01(pg.adjOrDefault(0, 0.18)) * height
	if r > height/2 {
		r = height / 2
	}
	if r > width/4 {
		r = width / 4
	}
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: r, Y: 0}}},
		{Type: "L", Points: []Point{{X: width - r, Y: 0}}},
		{Type: "C", Points: []Point{{X: width, Y: 0}, {X: width, Y: r}, {X: width - r, Y: r}}},
		{Type: "L", Points: []Point{{X: width - r, Y: height}}},
		{Type: "L", Points: []Point{{X: r, Y: height}}},
		{Type: "C", Points: []Point{{X: 0, Y: height}, {X: 0, Y: height - r}, {X: r, Y: height - r}}},
		{Type: "L", Points: []Point{{X: r, Y: 0}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

func rotateSimplePath(path Path, width, height, angleDeg float64) Path {
	cx, cy := width/2, height/2
	rad := angleDeg * math.Pi / 180
	cosA, sinA := math.Cos(rad), math.Sin(rad)
	for ci := range path.Commands {
		for pi := range path.Commands[ci].Points {
			p := &path.Commands[ci].Points[pi]
			dx, dy := p.X-cx, p.Y-cy
			p.X = cx + dx*cosA - dy*sinA
			p.Y = cy + dx*sinA + dy*cosA
		}
	}
	return path
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
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
	// The first adjustment owns the horizontal position of the apex. Legacy
	// OfficeArt stores this in the 21600 shape coordinate space; the legacy
	// bridge normalizes it to [0,1] before it reaches PresetGeometry. Keeping
	// the adjustment here (rather than baking it into the parser) also makes
	// the same geometry contract usable by other front ends.
	apexRatio := 0.5
	if len(pg.Adjustments) > 0 {
		apexRatio = pg.Adjustments[0]
	}
	if apexRatio < 0 {
		apexRatio = 0
	}
	if apexRatio > 1 {
		apexRatio = 1
	}
	apexX := width * apexRatio

	return Path{
		Commands: []PathCommand{
			// Move to bottom-left
			{Type: "M", Points: []Point{{X: 0, Y: height}}},
			// Line to the authored apex
			{Type: "L", Points: []Point{{X: apexX, Y: 0}}},
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
	if indent < 0 {
		indent = 0
	}
	if indent > width/2 {
		indent = width / 2
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
	if inset < 0 {
		inset = 0
	}
	if inset > width/2 {
		inset = width / 2
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
	innerRatio = clamp01(innerRatio)
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
	// Build the DrawingML rightArrow geometry, then rotate it for the basic
	// directional variants. ECMA-376 defines adj1 as the shaft-height fraction
	// and adj2 as arrow-head length measured against the short side. Keeping
	// those semantics here is important because PowerPoint translates legacy
	// OfficeArt arrow handles into exactly these canonical values.
	shaftFrac := math.Max(0, math.Min(1, pg.adjOrDefault(0, 0.5)))
	ss := math.Min(math.Abs(width), math.Abs(height))
	if ss == 0 {
		ss = math.Max(math.Abs(width), math.Abs(height))
	}
	maxHeadAdj := 0.0
	if ss > 0 {
		maxHeadAdj = math.Abs(width) / ss
	}
	headAdj := math.Max(0, math.Min(maxHeadAdj, pg.adjOrDefault(1, 0.5)))
	headW := ss * headAdj
	headX := width - headW
	tailH := height * shaftFrac
	tailY0 := (height - tailH) / 2
	pts := [][2]float64{
		{0, tailY0},
		{headX, tailY0},
		{headX, 0},
		{width, height / 2},
		{headX, height},
		{headX, tailY0 + tailH},
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

// leftUpArrowPath creates the legacy two-direction arrow with heads at the
// left and top ends of one joined L-shaped shaft.  It deliberately owns one
// continuous polygon instead of falling back to the generic rectangle.
func (pg *PresetGeometry) leftUpArrowPath(width, height float64) Path {
	t := math.Max(0.08, math.Min(0.42, pg.adjOrDefault(0, 0.22)))
	h := math.Max(t, math.Min(0.48, pg.adjOrDefault(1, 0.30)))
	tw, th := width*t, height*t
	hw, hh := width*h, height*h
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: height - th/2}}},
		{Type: "L", Points: []Point{{X: hw, Y: height - hh}}},
		{Type: "L", Points: []Point{{X: hw, Y: height - th}}},
		{Type: "L", Points: []Point{{X: width - tw, Y: height - th}}},
		{Type: "L", Points: []Point{{X: width - tw, Y: hh}}},
		{Type: "L", Points: []Point{{X: width - hw, Y: hh}}},
		{Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: hh * 2}}},
		{Type: "L", Points: []Point{{X: width - tw, Y: hh}}},
		{Type: "L", Points: []Point{{X: width - tw, Y: height}}},
		{Type: "L", Points: []Point{{X: hw, Y: height}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// upDownArrowCalloutPath is the vertical counterpart of the horizontal
// double-arrow callout.  Build it in transposed coordinates so adjustment
// semantics remain identical for both orientations.
func (pg *PresetGeometry) upDownArrowCalloutPath(width, height float64) Path {
	p := pg.leftRightArrowCalloutPath(height, width)
	for ci := range p.Commands {
		for pi := range p.Commands[ci].Points {
			q := &p.Commands[ci].Points[pi]
			q.X, q.Y = q.Y, q.X
		}
	}
	return p
}

// wavePath creates the legacy wave AutoShape as a closed sinusoidal band.
func (pg *PresetGeometry) wavePath(width, height float64) Path {
	a := math.Max(0.04, math.Min(0.22, pg.adjOrDefault(0, 0.12))) * height
	midTop, midBottom := height*.25, height*.75
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: midTop}}},
		{Type: "C", Points: []Point{{X: width * .25, Y: midTop - a}, {X: width * .25, Y: midTop + a}, {X: width * .5, Y: midTop}}},
		{Type: "C", Points: []Point{{X: width * .75, Y: midTop - a}, {X: width * .75, Y: midTop + a}, {X: width, Y: midTop}}},
		{Type: "L", Points: []Point{{X: width, Y: midBottom}}},
		{Type: "C", Points: []Point{{X: width * .75, Y: midBottom + a}, {X: width * .75, Y: midBottom - a}, {X: width * .5, Y: midBottom}}},
		{Type: "C", Points: []Point{{X: width * .25, Y: midBottom + a}, {X: width * .25, Y: midBottom - a}, {X: 0, Y: midBottom}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// circularArrowPath creates a clockwise annular arrow.  The body is an
// elliptical ring segment and the final polygonal head is part of the same
// filled path, preserving the preset on non-square bounds.
func (pg *PresetGeometry) circularArrowPath(width, height float64) Path {
	cx, cy := width/2, height/2
	rx, ry := width*.43, height*.43
	thick := math.Max(.10, math.Min(.38, pg.adjOrDefault(0, .22)))
	irx, iry := rx*(1-thick), ry*(1-thick)
	start := math.Pi * .15
	end := math.Pi * 1.72
	pt := func(rx, ry, a float64) Point { return Point{X: cx + rx*math.Cos(a), Y: cy + ry*math.Sin(a)} }
	o0, o1 := pt(rx, ry, start), pt(rx, ry, end)
	i1, i0 := pt(irx, iry, end), pt(irx, iry, start)
	tip := pt(rx*1.22, ry*1.22, end+.16)
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{o0}},
		{Type: "A", Points: []Point{o1}, Args: []float64{rx, ry, 0, 1, 1}},
		{Type: "L", Points: []Point{tip}},
		{Type: "L", Points: []Point{i1}},
		{Type: "A", Points: []Point{i0}, Args: []float64{irx, iry, 0, 1, 0}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// leftRightArrowPath creates a double-headed horizontal arrow.
func (pg *PresetGeometry) leftRightArrowPath(width, height float64) Path {
	// The double-headed arrow uses the same normalized adjustment contract as
	// the single arrow: adj1 owns head length, adj2 owns shaft thickness.  Each
	// head is constrained to at most half the shape so the two heads cannot
	// cross when an old OfficeArt handle is dragged to an extreme.
	headFrac := pg.adjOrDefault(0, 0.3)
	if headFrac > 0.5 {
		headFrac = 0.5
	}
	shaftFrac := pg.adjOrDefault(1, 0.4)
	headW := width * headFrac
	tailH := height * shaftFrac
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
	headFrac := pg.adjOrDefault(0, 0.3)
	if headFrac > 0.5 {
		headFrac = 0.5
	}
	shaftFrac := pg.adjOrDefault(1, 0.4)
	headH := height * headFrac
	tailW := width * shaftFrac
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
	// Keep the notch and nose shoulder on their respective halves.  Raw legacy
	// handles are normalized before reaching drawing geometry, but a normalized
	// extreme must still not self-intersect the chevron polygon.
	notchFrac := pg.adjOrDefault(0, 0.25)
	if notchFrac > 0.5 {
		notchFrac = 0.5
	}
	notch := width * notchFrac
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
	endDeg := 90.0
	if len(pg.Adjustments) >= 2 {
		startDeg = pg.Adjustments[0] * 360
		endDeg = pg.Adjustments[1] * 360
	}
	sweepDeg := math.Mod(endDeg-startDeg, 360)
	if sweepDeg < 0 {
		sweepDeg += 360
	}
	cx, cy := width/2, height/2
	rx, ry := width/2, height/2
	start := startDeg * math.Pi / 180
	end := endDeg * math.Pi / 180
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

// arcPath creates the open DrawingML arc.  Arc and pie share angular handles,
// but arc must not introduce radial edges or a fill-closing segment.
func (pg *PresetGeometry) arcPath(width, height float64) Path {
	startDeg, endDeg := 0.0, 90.0
	if len(pg.Adjustments) >= 2 {
		startDeg = pg.Adjustments[0] * 360
		endDeg = pg.Adjustments[1] * 360
	}
	sweep := math.Mod(endDeg-startDeg, 360)
	if sweep < 0 {
		sweep += 360
	}
	cx, cy, rx, ry := width/2, height/2, width/2, height/2
	a0, a1 := startDeg*math.Pi/180, endDeg*math.Pi/180
	p0 := Point{X: cx + rx*math.Cos(a0), Y: cy + ry*math.Sin(a0)}
	p1 := Point{X: cx + rx*math.Cos(a1), Y: cy + ry*math.Sin(a1)}
	large := 0.0
	if sweep > 180 {
		large = 1
	}
	return Path{Commands: []PathCommand{{Type: "M", Points: []Point{p0}}, {Type: "A", Points: []Point{p1}, Args: []float64{rx, ry, 0, large, 1}}}, FillRule: "nonzero"}
}

// chordPath creates a chord (circular segment) shape.
func (pg *PresetGeometry) chordPath(width, height float64) Path {
	startDeg := 0.0
	endDeg := 90.0
	if len(pg.Adjustments) >= 2 {
		startDeg = pg.Adjustments[0] * 360
		endDeg = pg.Adjustments[1] * 360
	}
	sweepDeg := math.Mod(endDeg-startDeg, 360)
	if sweepDeg < 0 {
		sweepDeg += 360
	}
	cx, cy := width/2, height/2
	rx, ry := width/2, height/2
	start := startDeg * math.Pi / 180
	end := endDeg * math.Pi / 180
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

func polygonPath(points []Point) Path {
	if len(points) == 0 {
		return Path{}
	}
	cmds := make([]PathCommand, 0, len(points)+1)
	cmds = append(cmds, PathCommand{Type: "M", Points: []Point{points[0]}})
	for _, pt := range points[1:] {
		cmds = append(cmds, PathCommand{Type: "L", Points: []Point{pt}})
	}
	cmds = append(cmds, PathCommand{Type: "Z"})
	return Path{Commands: cmds, FillRule: "nonzero"}
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
	// PaintFlagsSet preserves per-path OfficeArt paint semantics for custom geometry.
	// DrawingML paths that do not author path-local paint flags leave this false and
	// inherit the shape-level fill/stroke as before.
	PaintFlagsSet bool
	FillEnabled   bool
	StrokeEnabled bool
}

// BoundingBox returns the bounding rectangle of a path.
func (p *Path) BoundingBox() Rect {
	if len(p.Commands) == 0 {
		return Rect{}
	}

	var points []Point
	var current Point
	haveCurrent := false
	for _, cmd := range p.Commands {
		points = append(points, cmd.Points...)
		if cmd.Type == "A" && haveCurrent && len(cmd.Points) > 0 && len(cmd.Args) >= 5 {
			points = append(points, svgArcEllipseBoundsPoints(current, cmd.Points[0], cmd.Args)...)
		}
		if len(cmd.Points) > 0 {
			current = cmd.Points[len(cmd.Points)-1]
			haveCurrent = true
		}
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

// svgArcEllipseBoundsPoints returns conservative extrema for an SVG endpoint
// arc command.  Path.BoundingBox historically looked only at command points,
// which collapses valid half-ellipses whose start/end share one coordinate
// (for example the DrawingML moon preset) to zero width.  The path bbox is used
// only as a geometric diagnostic today, so the whole supporting ellipse is a
// safe conservative bound and avoids special-casing individual presets.
func svgArcEllipseBoundsPoints(start, end Point, args []float64) []Point {
	if len(args) < 5 {
		return nil
	}
	rx, ry := math.Abs(args[0]), math.Abs(args[1])
	if rx <= 0 || ry <= 0 || (start.X == end.X && start.Y == end.Y) {
		return nil
	}
	phi := args[2] * math.Pi / 180
	largeArc := args[3] != 0
	sweep := args[4] != 0
	cosPhi, sinPhi := math.Cos(phi), math.Sin(phi)
	dx2 := (start.X - end.X) / 2
	dy2 := (start.Y - end.Y) / 2
	x1p := cosPhi*dx2 + sinPhi*dy2
	y1p := -sinPhi*dx2 + cosPhi*dy2

	lambda := x1p*x1p/(rx*rx) + y1p*y1p/(ry*ry)
	if lambda > 1 {
		s := math.Sqrt(lambda)
		rx *= s
		ry *= s
	}
	den := rx*rx*y1p*y1p + ry*ry*x1p*x1p
	if den == 0 {
		return nil
	}
	num := rx*rx*ry*ry - den
	if num < 0 {
		num = 0
	}
	coef := math.Sqrt(num / den)
	if largeArc == sweep {
		coef = -coef
	}
	cxp := coef * (rx * y1p / ry)
	cyp := coef * (-ry * x1p / rx)
	cx := cosPhi*cxp - sinPhi*cyp + (start.X+end.X)/2
	cy := sinPhi*cxp + cosPhi*cyp + (start.Y+end.Y)/2

	xExtent := math.Hypot(rx*cosPhi, ry*sinPhi)
	yExtent := math.Hypot(rx*sinPhi, ry*cosPhi)
	return []Point{{X: cx - xExtent, Y: cy - yExtent}, {X: cx + xExtent, Y: cy + yExtent}}
}

// Class 13: canonical flowchart preset families. These paths deliberately live
// in preset geometry rather than renderer CSS so PPT and PPTX share topology.

func (pg *PresetGeometry) flowChartMultidocumentPath(w, h float64) Path {
	dx, dy := w*.10, h*.10
	p := Path{FillRule: "nonzero"}
	// Three staggered document sheets; each sheet owns its wavy lower edge.
	for i := 2; i >= 0; i-- {
		x, y := float64(i)*dx/2, float64(i)*dy/2
		r, b := w-float64(2-i)*dx/2, h-float64(2-i)*dy/2
		p.Commands = append(p.Commands,
			PathCommand{Type: "M", Points: []Point{{x, y}}},
			PathCommand{Type: "L", Points: []Point{{r, y}, {r, b * .78}}},
			PathCommand{Type: "C", Points: []Point{{r * .72, b * .66}, {r * .42, b * .96}, {x, b * .82}}},
			PathCommand{Type: "Z"})
	}
	return p
}

func (pg *PresetGeometry) flowChartJunctionPath(w, h float64, diagonal bool) Path {
	p := pg.ellipsePath(w, h)
	if diagonal {
		p.Commands = append(p.Commands,
			PathCommand{Type: "M", Points: []Point{{w * .20, h * .20}}}, PathCommand{Type: "L", Points: []Point{{w * .80, h * .80}}},
			PathCommand{Type: "M", Points: []Point{{w * .80, h * .20}}}, PathCommand{Type: "L", Points: []Point{{w * .20, h * .80}}})
	} else {
		p.Commands = append(p.Commands,
			PathCommand{Type: "M", Points: []Point{{w * .20, h * .50}}}, PathCommand{Type: "L", Points: []Point{{w * .80, h * .50}}},
			PathCommand{Type: "M", Points: []Point{{w * .50, h * .20}}}, PathCommand{Type: "L", Points: []Point{{w * .50, h * .80}}})
	}
	return p
}

func (pg *PresetGeometry) flowChartCollatePath(w, h float64) Path {
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{0, 0}}}, {Type: "L", Points: []Point{{w, 0}, {0, h}, {w, h}}}, {Type: "Z"},
	}, FillRule: "nonzero"}
}

func (pg *PresetGeometry) flowChartOfflineStoragePath(w, h float64) Path {
	d := w * .14
	return polygonPath([]Point{{d, 0}, {w, 0}, {w - d, h}, {0, h}})
}

func (pg *PresetGeometry) flowChartOnlineStoragePath(w, h float64) Path {
	// Direct-access storage: concave left edge and convex right edge.
	d := w * .16
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{d, 0}}}, {Type: "L", Points: []Point{{w - d, 0}}},
		{Type: "C", Points: []Point{{w + d*.15, h * .18}, {w + d*.15, h * .82}, {w - d, h}}},
		{Type: "L", Points: []Point{{d, h}}},
		{Type: "C", Points: []Point{{d * 1.65, h * .72}, {d * 1.65, h * .28}, {d, 0}}}, {Type: "Z"},
	}, FillRule: "nonzero"}
}

func (pg *PresetGeometry) flowChartMagneticDrumPath(w, h float64) Path {
	rx := w * .16
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{rx, 0}}}, {Type: "C", Points: []Point{{0, 0}, {0, h}, {rx, h}}},
		{Type: "L", Points: []Point{{w - rx, h}}}, {Type: "C", Points: []Point{{w, h}, {w, 0}, {w - rx, 0}}}, {Type: "Z"},
		{Type: "M", Points: []Point{{rx, 0}}}, {Type: "C", Points: []Point{{rx * 2, 0}, {rx * 2, h}, {rx, h}}},
	}, FillRule: "nonzero"}
}

func (pg *PresetGeometry) flowChartInputOutputPath(w, h float64) Path {
	d := w * .18
	return polygonPath([]Point{{d, 0}, {w, 0}, {w - d, h}, {0, h}})
}
func (pg *PresetGeometry) flowChartPredefinedProcessPath(w, h float64) Path {
	d := w * .14
	return Path{Commands: []PathCommand{{Type: "M", Points: []Point{{0, 0}}}, {Type: "L", Points: []Point{{w, 0}, {w, h}, {0, h}}}, {Type: "Z"}, {Type: "M", Points: []Point{{d, 0}}}, {Type: "L", Points: []Point{{d, h}}}, {Type: "M", Points: []Point{{w - d, 0}}}, {Type: "L", Points: []Point{{w - d, h}}}}, FillRule: "nonzero"}
}
func (pg *PresetGeometry) flowChartInternalStoragePath(w, h float64) Path {
	dx, dy := w*.14, h*.18
	return Path{Commands: []PathCommand{{Type: "M", Points: []Point{{0, 0}}}, {Type: "L", Points: []Point{{w, 0}, {w, h}, {0, h}}}, {Type: "Z"}, {Type: "M", Points: []Point{{dx, 0}}}, {Type: "L", Points: []Point{{dx, h}}}, {Type: "M", Points: []Point{{0, dy}}}, {Type: "L", Points: []Point{{w, dy}}}}, FillRule: "nonzero"}
}
func (pg *PresetGeometry) flowChartManualOperationPath(w, h float64) Path {
	d := w * .18
	return polygonPath([]Point{{0, 0}, {w, 0}, {w - d, h}, {d, h}})
}
func (pg *PresetGeometry) flowChartPunchedCardPath(w, h float64) Path {
	d := min(w, h) * .22
	return polygonPath([]Point{{0, d}, {d, 0}, {w, 0}, {w, h}, {0, h}})
}
func (pg *PresetGeometry) flowChartSortPath(w, h float64) Path {
	return Path{Commands: []PathCommand{{Type: "M", Points: []Point{{w / 2, 0}}}, {Type: "L", Points: []Point{{w, h / 2}, {w / 2, h}, {0, h / 2}}}, {Type: "Z"}, {Type: "M", Points: []Point{{0, h / 2}}}, {Type: "L", Points: []Point{{w, h / 2}}}}, FillRule: "nonzero"}
}
func (pg *PresetGeometry) flowChartExtractPath(w, h float64) Path {
	return polygonPath([]Point{{w / 2, 0}, {w, h}, {0, h}})
}
func (pg *PresetGeometry) flowChartStoredDataPath(w, h float64) Path {
	d := w * .16
	return Path{Commands: []PathCommand{{Type: "M", Points: []Point{{d, 0}}}, {Type: "L", Points: []Point{{w, 0}}}, {Type: "C", Points: []Point{{w - d*.7, h * .22}, {w - d*.7, h * .78}, {w, h}}}, {Type: "L", Points: []Point{{d, h}}}, {Type: "C", Points: []Point{{-d * .45, h * .78}, {-d * .45, h * .22}, {d, 0}}}, {Type: "Z"}}, FillRule: "nonzero"}
}
func (pg *PresetGeometry) flowChartDelayPath(w, h float64) Path {
	r := w * .28
	return Path{Commands: []PathCommand{{Type: "M", Points: []Point{{0, 0}}}, {Type: "L", Points: []Point{{w - r, 0}}}, {Type: "C", Points: []Point{{w, h * .05}, {w, h * .95}, {w - r, h}}}, {Type: "L", Points: []Point{{0, h}}}, {Type: "Z"}}, FillRule: "nonzero"}
}
func (pg *PresetGeometry) flowChartMagneticTapePath(w, h float64) Path {
	return Path{Commands: []PathCommand{{Type: "M", Points: []Point{{w * .08, h * .58}}}, {Type: "C", Points: []Point{{w * .08, h * .12}, {w * .92, h * .12}, {w * .92, h * .58}}}, {Type: "C", Points: []Point{{w * .92, h * .9}, {w * .55, h * .98}, {w * .38, h * .78}}}, {Type: "L", Points: []Point{{w * .92, h * .78}}}, {Type: "C", Points: []Point{{w * .72, h}, {w * .08, h * .96}, {w * .08, h * .58}}}, {Type: "Z"}}, FillRule: "nonzero"}
}
func (pg *PresetGeometry) flowChartMagneticDiskPath(w, h float64) Path {
	ry := h * .16
	return Path{Commands: []PathCommand{{Type: "M", Points: []Point{{0, ry}}}, {Type: "C", Points: []Point{{0, 0}, {w, 0}, {w, ry}}}, {Type: "L", Points: []Point{{w, h - ry}}}, {Type: "C", Points: []Point{{w, h}, {0, h}, {0, h - ry}}}, {Type: "Z"}, {Type: "M", Points: []Point{{0, ry}}}, {Type: "C", Points: []Point{{0, ry * 2}, {w, ry * 2}, {w, ry}}}}, FillRule: "nonzero"}
}
func (pg *PresetGeometry) flowChartDisplayPath(w, h float64) Path {
	d := w * .18
	return Path{Commands: []PathCommand{{Type: "M", Points: []Point{{d, 0}}}, {Type: "L", Points: []Point{{w - d, 0}}}, {Type: "C", Points: []Point{{w, h * .18}, {w, h * .82}, {w - d, h}}}, {Type: "L", Points: []Point{{d, h}, {0, h / 2}}}, {Type: "Z"}}, FillRule: "nonzero"}
}

// teardropPath is an ellipse-like body pulled to a single pointed corner.
func (pg *PresetGeometry) teardropPath(width, height float64) Path {
	k := pg.adjOrDefault(0, 0.55)
	k = max(0.25, min(0.85, k))
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: width, Y: 0}}},
		{Type: "C", Points: []Point{{X: width * k, Y: 0}, {X: 0, Y: height * (1 - k)}, {X: 0, Y: height / 2}}},
		{Type: "C", Points: []Point{{X: 0, Y: height}, {X: width, Y: height}, {X: width, Y: 0}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// swooshArrowPath owns the swept-tail silhouette instead of degrading to rect.
func (pg *PresetGeometry) swooshArrowPath(width, height float64) Path {
	head := max(0.18, min(0.55, pg.adjOrDefault(0, 0.34)))
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: height * .78}}},
		{Type: "C", Points: []Point{{X: width * .28, Y: height * .62}, {X: width * .48, Y: height * .38}, {X: width * (1 - head), Y: height * .32}}},
		{Type: "L", Points: []Point{{X: width * (1 - head), Y: 0}}},
		{Type: "L", Points: []Point{{X: width, Y: height * .38}}},
		{Type: "L", Points: []Point{{X: width * (1 - head), Y: height * .72}}},
		{Type: "L", Points: []Point{{X: width * (1 - head), Y: height * .50}}},
		{Type: "C", Points: []Point{{X: width * .45, Y: height * .56}, {X: width * .22, Y: height * .82}, {X: 0, Y: height}}},
		{Type: "Z"},
	}, FillRule: "nonzero"}
}

// circularArrowVariantPath materializes the left and bidirectional circular-arrow presets.
func (pg *PresetGeometry) circularArrowVariantPath(width, height float64, both bool) Path {
	cx, cy := width/2, height/2
	rx, ry := width*.42, height*.42
	thick := max(.10, min(.32, pg.adjOrDefault(0, .20)))
	irx, iry := rx*(1-thick), ry*(1-thick)
	headX := width * .18
	cmd := []PathCommand{
		{Type: "M", Points: []Point{{X: headX, Y: cy}}},
		{Type: "L", Points: []Point{{X: width * .34, Y: height * .18}}},
		{Type: "L", Points: []Point{{X: width * .35, Y: height * .36}}},
		{Type: "A", Points: []Point{{X: cx, Y: cy - ry}}, Args: []float64{rx, ry, 0, 0, 1}},
		{Type: "A", Points: []Point{{X: width * .65, Y: height * .64}}, Args: []float64{rx, ry, 0, 0, 1}},
	}
	if both {
		cmd = append(cmd,
			PathCommand{Type: "L", Points: []Point{{X: width * .82, Y: cy}}},
			PathCommand{Type: "L", Points: []Point{{X: width * .66, Y: height * .82}}},
			PathCommand{Type: "L", Points: []Point{{X: width * .65, Y: height * .64}}},
		)
	}
	cmd = append(cmd,
		PathCommand{Type: "A", Points: []Point{{X: cx, Y: cy - iry}}, Args: []float64{irx, iry, 0, 0, 0}},
		PathCommand{Type: "A", Points: []Point{{X: width * .35, Y: height * .36}}, Args: []float64{irx, iry, 0, 0, 0}},
		PathCommand{Type: "L", Points: []Point{{X: width * .34, Y: height * .18}}},
		PathCommand{Type: "Z"},
	)
	return Path{Commands: cmd, FillRule: "nonzero"}
}

func (pg *PresetGeometry) cornerPath(width, height float64) Path {
	t := max(.08, min(.45, pg.adjOrDefault(0, .25)))
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}}, {Type: "L", Points: []Point{{X: width * t, Y: 0}}},
		{Type: "L", Points: []Point{{X: width * t, Y: height * (1 - t)}}}, {Type: "L", Points: []Point{{X: width, Y: height * (1 - t)}}},
		{Type: "L", Points: []Point{{X: width, Y: height}}}, {Type: "L", Points: []Point{{X: 0, Y: height}}}, {Type: "Z"},
	}, FillRule: "nonzero"}
}

func (pg *PresetGeometry) diagStripePath(width, height float64) Path {
	t := max(.08, min(.42, pg.adjOrDefault(0, .22)))
	return Path{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: width * (1 - t), Y: 0}}}, {Type: "L", Points: []Point{{X: width, Y: 0}}},
		{Type: "L", Points: []Point{{X: width * t, Y: height}}}, {Type: "L", Points: []Point{{X: 0, Y: height}}}, {Type: "Z"},
	}, FillRule: "nonzero"}
}
