package chart

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// rect3D describes a screen-space rectangle used as the projection target.
type rect3D struct {
	x, y, w, h int
}

// view3DDefaults returns an effective View3D with Excel defaults when nil.
func view3DDefaults(v *View3D) *View3D {
	if v != nil {
		return v
	}
	return &View3D{
		RotX:           15,
		RotY:           20,
		Perspective:    30,
		DepthPercent:   100,
		RightAngleAxes: false,
	}
}

// projector3D maps normalized world coordinates (wx,wy,wz ∈ [0,1]) to screen
// pixels inside plotRect, applying rotation and optional perspective.
type projector3D struct {
	view     *View3D
	plotRect rect3D
	minX     float64
	maxX     float64
	minY     float64
	maxY     float64
}

// newProjector creates a projector and pre-computes the bounds of the unit
// cube after rotation/perspective so that every projection fits plotRect.
func newProjector(view *View3D, plotRect rect3D) *projector3D {
	p := &projector3D{
		view:     view3DDefaults(view),
		plotRect: plotRect,
		minX:     math.Inf(1),
		maxX:     math.Inf(-1),
		minY:     math.Inf(1),
		maxY:     math.Inf(-1),
	}
	// Pre-compute bounds by projecting all 8 corners of the unit cube.
	for ix := 0; ix < 2; ix++ {
		for iy := 0; iy < 2; iy++ {
			for iz := 0; iz < 2; iz++ {
				sx, sy, _ := p.rawProject(float64(ix), float64(iy), float64(iz))
				p.minX = math.Min(p.minX, sx)
				p.maxX = math.Max(p.maxX, sx)
				p.minY = math.Min(p.minY, sy)
				p.maxY = math.Max(p.maxY, sy)
			}
		}
	}
	// Add a small padding so edges don’t touch the frame.
	dx := p.maxX - p.minX
	dy := p.maxY - p.minY
	if dx > 0 {
		pad := dx * 0.05
		p.minX -= pad
		p.maxX += pad
	}
	if dy > 0 {
		pad := dy * 0.05
		p.minY -= pad
		p.maxY += pad
	}
	return p
}

// rawProject applies rotation and perspective without final screen mapping.
// Returned values are raw projected coordinates plus the rotated z (depth).
func (p *projector3D) rawProject(wx, wy, wz float64) (x, y, z float64) {
	// Center coordinates around origin.
	x = wx - 0.5
	y = wy - 0.5
	z = wz - 0.5

	// Rotation around X axis (elevation).
	rx := float64(p.view.RotX) * math.Pi / 180.0
	cy, cz := math.Cos(rx), math.Sin(rx)
	y, z = y*cy-z*cz, y*cz+z*cy

	// Rotation around Y axis (azimuth).
	ry := float64(p.view.RotY) * math.Pi / 180.0
	cx, sx := math.Cos(ry), math.Sin(ry)
	x, z = x*cx+z*sx, -x*sx+z*cx

	// Perspective divide (if enabled).
	if !p.view.RightAngleAxes && p.view.Perspective > 0 {
		// Distance heuristic: larger perspective → smaller d → more distortion.
		d := 2.0 / math.Tan(float64(p.view.Perspective)*math.Pi/360.0+0.001)
		if d < 0.1 {
			d = 0.1
		}
		// Camera is at +d looking toward origin, so closer points (larger z)
		// have smaller (d - z) denominator and appear magnified.
		scale := d / (d - z)
		x *= scale
		y *= scale
	}
	return x, y, z
}

// project maps normalized world coordinates to screen pixels.
func (p *projector3D) project(wx, wy, wz float64) (sx, sy float64) {
	rx, ry, _ := p.rawProject(wx, wy, wz)

	dx := p.maxX - p.minX
	dy := p.maxY - p.minY
	if dx <= 0 {
		dx = 1
	}
	if dy <= 0 {
		dy = 1
	}

	nx := (rx - p.minX) / dx
	ny := (ry - p.minY) / dy

	sx = float64(p.plotRect.x) + nx*float64(p.plotRect.w)
	sy = float64(p.plotRect.y+p.plotRect.h) - ny*float64(p.plotRect.h)
	return sx, sy
}

// projectInt is a convenience wrapper returning integer screen coordinates.
func (p *projector3D) projectInt(wx, wy, wz float64) (sx, sy int) {
	fx, fy := p.project(wx, wy, wz)
	return int(math.Round(fx)), int(math.Round(fy))
}

// face3D is a projected polygon ready for SVG emission.
type face3D struct {
	pts   [][2]float64 // screen-space vertices
	color string       // fill color (hex, no #)
	depth float64      // average rotated Z used for sorting
}

// newBoxFaces creates the visible faces of an axis-aligned box in world space.
// The box spans [x, x+w] × [y, y+h] × [z, z+d]. Only faces pointing toward
// the viewer are returned. Front face uses base color, top is lighter, side is darker.
func newBoxFaces(x, y, z, w, h, d float64, baseColor string, p *projector3D) []face3D {
	// Box corners in world space.
	corners := [8][3]float64{
		{x, y + h, z + d},     // 0 front-top-left
		{x + w, y + h, z + d}, // 1 front-top-right
		{x + w, y, z + d},     // 2 front-bottom-right
		{x, y, z + d},         // 3 front-bottom-left
		{x, y + h, z},         // 4 back-top-left
		{x + w, y + h, z},     // 5 back-top-right
		{x + w, y, z},         // 6 back-bottom-right
		{x, y, z},             // 7 back-bottom-left
	}

	// Project all corners to screen space.
	proj := make([][3]float64, 8) // sx, sy, depth(z')
	for i, c := range corners {
		sx, sy, sz := p.rawProject(c[0], c[1], c[2])
		proj[i] = [3]float64{sx, sy, sz}
	}

	type boxFaceDef struct {
		idx     [4]int
		shade   string // "front", "top", "side"
		reorder bool   // whether this face is on the back of the box
	}
	// Face definitions with corner indices and shading hint.
	faces := []boxFaceDef{
		{idx: [4]int{0, 1, 2, 3}, shade: "front", reorder: false}, // front
		{idx: [4]int{4, 5, 1, 0}, shade: "top", reorder: false},    // top
		{idx: [4]int{1, 5, 6, 2}, shade: "side", reorder: false},   // right
		{idx: [4]int{4, 0, 3, 7}, shade: "side", reorder: true},    // left
		{idx: [4]int{3, 2, 6, 7}, shade: "front", reorder: true},   // bottom
		{idx: [4]int{5, 4, 7, 6}, shade: "front", reorder: true},   // back
	}

	var result []face3D
	for _, f := range faces {
		// Build 2D polygon vertices in screen space.
		pts := make([][2]float64, 4)
		var depth float64
		for i, ci := range f.idx {
			pts[i] = [2]float64{proj[ci][0], proj[ci][1]}
			depth += proj[ci][2]
		}
		depth /= 4

		// Back-face culling: compute signed area of the 2D polygon.
		// In our coordinate system, positive area = CCW = facing viewer
		// (because Y increases upward in rawProject, but screen Y is flipped).
		// We'll use the cross product of two edges in 3D screen-space.
		ax := pts[1][0] - pts[0][0]
		ay := pts[1][1] - pts[0][1]
		bx := pts[2][0] - pts[0][0]
		by := pts[2][1] - pts[0][1]
		cross := ax*by - ay*bx
		// After screen mapping Y is flipped, so negative cross means front-facing.
		if cross >= 0 {
			continue // back-facing
		}

		color := baseColor
		switch f.shade {
		case "top":
			color = lightenColor(baseColor, 15)
		case "side":
			color = darkenColor(baseColor, 15)
		}

		result = append(result, face3D{pts: pts, color: color, depth: depth})
	}
	return result
}

// sortFacesByDepth sorts faces back-to-front (farthest first).
func sortFacesByDepth(faces []face3D) {
	sort.Slice(faces, func(i, j int) bool {
		return faces[i].depth < faces[j].depth
	})
}

// emitFace writes a single face as an SVG <polygon>.
func emitFace(b *strings.Builder, f face3D, p *projector3D) {
	dx := p.maxX - p.minX
	dy := p.maxY - p.minY
	if dx <= 0 {
		dx = 1
	}
	if dy <= 0 {
		dy = 1
	}

	var ptsStr strings.Builder
	for i, pt := range f.pts {
		if i > 0 {
			ptsStr.WriteByte(' ')
		}
		nx := (pt[0] - p.minX) / dx
		ny := (pt[1] - p.minY) / dy
		sx := float64(p.plotRect.x) + nx*float64(p.plotRect.w)
		sy := float64(p.plotRect.y+p.plotRect.h) - ny*float64(p.plotRect.h)
		fmt.Fprintf(&ptsStr, "%.1f,%.1f", sx, sy)
	}
	fmt.Fprintf(b, `<polygon points="%s" fill="#%s" stroke="white" stroke-width="0.5"/>`,
		ptsStr.String(), f.color)
}

// --- color helpers ---

// parseHexColor converts "RRGGBB", "RGB", or "#RRGGBB" into (r,g,b).
func parseHexColor(hex string) (r, g, b int) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return 128, 128, 128
	}
	var rr, gg, bb byte
	if _, err := fmt.Sscanf(hex, "%02x%02x%02x", &rr, &gg, &bb); err != nil {
		return 128, 128, 128
	}
	return int(rr), int(gg), int(bb)
}

// hexColor formats (r,g,b) back into "RRGGBB".
func hexColor(r, g, b int) string {
	if r < 0 {
		r = 0
	}
	if r > 255 {
		r = 255
	}
	if g < 0 {
		g = 0
	}
	if g > 255 {
		g = 255
	}
	if b < 0 {
		b = 0
	}
	if b > 255 {
		b = 255
	}
	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}

// lightenColor increases each channel toward white by pct percent.
func lightenColor(hex string, pct int) string {
	r, g, b := parseHexColor(hex)
	f := float64(pct) / 100.0
	return hexColor(
		int(float64(r)+(255-float64(r))*f),
		int(float64(g)+(255-float64(g))*f),
		int(float64(b)+(255-float64(b))*f),
	)
}

// darkenColor decreases each channel toward black by pct percent.
func darkenColor(hex string, pct int) string {
	r, g, b := parseHexColor(hex)
	f := float64(pct) / 100.0
	return hexColor(
		int(float64(r)*(1-f)),
		int(float64(g)*(1-f)),
		int(float64(b)*(1-f)),
	)
}
// --- 3D bar / column chart renderer ---

// renderBar3DChart renders horizontal (bar) or vertical (column) 3D charts.
func (c *Chart) renderBar3DChart(b *strings.Builder, x, y, w, h int, horizontal bool) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	// Draw 2D axes as visual reference beneath the 3D bars.
	c.renderAxes(b, x, y, w, h, horizontal)

	categories := len(c.Series[0].Values)
	seriesCount := len(c.Series)
	plotX := x + 50
	plotY := y
	plotW := w - 60
	plotH := h - 40

	var maxVal float64
	if c.PercentStacked {
		maxVal = 100
	} else {
		maxVal = c.axisMax()
	}
	if maxVal == 0 {
		maxVal = 1
	}

	// Per-category totals for percent-stacked.
	catTotals := make([]float64, categories)
	if c.PercentStacked {
		for catIdx := 0; catIdx < categories; catIdx++ {
			total := 0.0
			for _, series := range c.Series {
				if catIdx < len(series.Values) {
					total += series.Values[catIdx]
				}
			}
			catTotals[catIdx] = total
		}
	}

	// Pre-compute stacked cumulative values.
	stackedOffsets := make([][]float64, categories)
	if c.Stacked || c.PercentStacked {
		for catIdx := 0; catIdx < categories; catIdx++ {
			stackedOffsets[catIdx] = make([]float64, len(c.Series))
			cum := 0.0
			for seriesIdx, series := range c.Series {
				stackedOffsets[catIdx][seriesIdx] = cum
				if catIdx < len(series.Values) {
					v := series.Values[catIdx]
					if c.PercentStacked && catTotals[catIdx] > 0 {
						v = (v / catTotals[catIdx]) * 100
					}
					cum += v
				}
			}
		}
	}

	view := view3DDefaults(c.View3D)
	proj := newProjector(view, rect3D{x: plotX, y: plotY, w: plotW, h: plotH})

	depthPct := float64(view.DepthPercent) / 100.0
	if depthPct <= 0 {
		depthPct = 1.0
	}

	// Bar sizing in world coordinates [0,1].
	catStep := 1.0 / float64(categories)

	var barPrimarySize float64
	if c.GapWidth > 0 {
		gapRatio := float64(c.GapWidth) / 100.0
		overlapRatio := float64(c.Overlap) / 100.0
		barUnit := catStep / (float64(seriesCount) + gapRatio)
		if c.Stacked || c.PercentStacked {
			barPrimarySize = barUnit * float64(seriesCount)
		} else {
			barPrimarySize = barUnit * (1 + overlapRatio)
		}
	} else {
		if c.Stacked || c.PercentStacked {
			barPrimarySize = catStep * 0.8
		} else {
			barPrimarySize = (catStep * 0.8) / float64(seriesCount)
		}
	}

	barDepth := 0.3 * depthPct
	if barDepth > catStep*0.8 {
		barDepth = catStep * 0.8
	}
	if barDepth < 0.02 {
		barDepth = 0.02
	}

	var allFaces []face3D

	for catIdx := 0; catIdx < categories; catIdx++ {
		for seriesIdx, series := range c.Series {
			if catIdx >= len(series.Values) {
				continue
			}

			val := series.Values[catIdx]
			color := c.seriesFillColorFor(seriesIdx, catIdx)
			displayVal := val
			if c.PercentStacked && catTotals[catIdx] > 0 {
				displayVal = (val / catTotals[catIdx]) * 100
				val = displayVal
			}
			if val < 0 {
				val = 0
			}

			var bx0, bx1, by0, by1, bz0, bz1 float64

			if horizontal {
				if c.Stacked || c.PercentStacked {
					by0 = float64(catIdx)*catStep + (catStep-barPrimarySize)/2
					by1 = by0 + barPrimarySize
					bx0 = stackedOffsets[catIdx][seriesIdx] / maxVal
					bx1 = bx0 + val/maxVal
				} else {
					by0 = float64(catIdx)*catStep + (catStep-barPrimarySize*float64(seriesCount))/2 + float64(seriesIdx)*barPrimarySize
					by1 = by0 + barPrimarySize
					bx0 = 0
					bx1 = val / maxVal
				}
			} else {
				if c.Stacked || c.PercentStacked {
					bx0 = float64(catIdx)*catStep + (catStep-barPrimarySize)/2
					bx1 = bx0 + barPrimarySize
					by0 = stackedOffsets[catIdx][seriesIdx] / maxVal
					by1 = by0 + val/maxVal
				} else {
					bx0 = float64(catIdx)*catStep + (catStep-barPrimarySize*float64(seriesCount))/2 + float64(seriesIdx)*barPrimarySize
					bx1 = bx0 + barPrimarySize
					by0 = 0
					by1 = val / maxVal
				}
			}
			bz0 = 0.5 - barDepth/2
			bz1 = bz0 + barDepth

			faces := newBoxFaces(bx0, by0, bz0, bx1-bx0, by1-by0, bz1-bz0, color, proj)
			allFaces = append(allFaces, faces...)
		}
	}

	sortFacesByDepth(allFaces)
	for _, f := range allFaces {
		emitFace(b, f, proj)
	}

	// Category labels (same positioning as 2D).
	for catIdx := 0; catIdx < categories; catIdx++ {
		if catIdx < len(c.Categories) {
			if horizontal {
				labelY := plotY + int(float64(catIdx)*catStep*float64(plotH)) + int(catStep*float64(plotH)/2)
				fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="end" font-size="10" fill="#666">%s</text>`,
					x+45, labelY, escapeSVG(c.Categories[catIdx]))
			} else {
				labelX := plotX + int(float64(catIdx)*catStep*float64(plotW)) + int(catStep*float64(plotW)/2)
				labelY := plotY + plotH + 20
				fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="10" fill="#666">%s</text>`,
					labelX, labelY, escapeSVG(c.Categories[catIdx]))
			}
		}
	}

	// Data labels.
	for catIdx := 0; catIdx < categories; catIdx++ {
		for seriesIdx, series := range c.Series {
			if catIdx >= len(series.Values) {
				continue
			}
			val := series.Values[catIdx]
			displayVal := val
			if c.PercentStacked && catTotals[catIdx] > 0 {
				displayVal = (val / catTotals[catIdx]) * 100
				val = displayVal
			}
			if val < 0 {
				val = 0
			}

			if series.ShowDataLabels || series.ShowPercent || series.ShowCatName || series.ShowSerName || series.ShowBubbleSize {
				var wx, wy, wz float64
				if horizontal {
					if c.Stacked || c.PercentStacked {
						wy = float64(catIdx)*catStep + catStep/2
						wx = (stackedOffsets[catIdx][seriesIdx] + val/2) / maxVal
					} else {
						wy = float64(catIdx)*catStep + (catStep-barPrimarySize*float64(seriesCount))/2 + float64(seriesIdx)*barPrimarySize + barPrimarySize/2
						wx = val / maxVal / 2
						if series.ShowPercent || series.ShowDataLabels {
							wx = val / maxVal * 0.75
						}
					}
				} else {
					if c.Stacked || c.PercentStacked {
						wx = float64(catIdx)*catStep + catStep/2
						wy = (stackedOffsets[catIdx][seriesIdx] + val/2) / maxVal
					} else {
						wx = float64(catIdx)*catStep + (catStep-barPrimarySize*float64(seriesCount))/2 + float64(seriesIdx)*barPrimarySize + barPrimarySize/2
						wy = val / maxVal / 2
						if series.ShowPercent || series.ShowDataLabels {
							wy = val / maxVal * 0.75
						}
					}
				}
				wz = 0.5 + barDepth/2 // front face center
				sx, sy := proj.projectInt(wx, wy, wz)
				labelText := buildDataLabel(&series, catIdx, displayVal, maxVal, c.Categories)
				style := dataLabelStyleAttrs(effectiveLabelStyle(&series, catIdx), 10, "#333")
				fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" %s>%s</text>`,
					sx, sy, style, escapeSVG(labelText))
			}
		}
	}
}

// --- 3D pie chart renderer ---

// renderPie3DChart renders an extruded 3D pie or doughnut chart.
func (c *Chart) renderPie3DChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	series := c.Series[0]
	total := 0.0
	for _, v := range series.Values {
		total += v
	}
	if total == 0 {
		total = 1
	}

	cx := x + w/2
	cy := y + h/2
	r := min(w, h) / 3

	view := view3DDefaults(c.View3D)
	depthPct := float64(view.DepthPercent) / 100.0
	if depthPct <= 0 {
		depthPct = 1.0
	}
	extrude := float64(r) * depthPct / 2.0
	if extrude < 2 {
		extrude = 2
	}
	if extrude > float64(r) {
		extrude = float64(r)
	}

	innerR := 0
	if c.ChartType == ChartDoughnut {
		holePct := c.HoleSize
		if holePct <= 0 {
			holePct = 50 // Word default
		}
		innerR = r * holePct / 100
	}

	type sliceInfo struct {
		idx        int
		val        float64
		angle      float64
		startAngle float64
		midAngle   float64
		midY       int // screen y of outer arc midpoint for sorting
	}
	var slices []sliceInfo
	startAngle := -90.0
	for i, val := range series.Values {
		angle := (val / total) * 360.0
		midAngle := startAngle + angle/2.0
		midRad := midAngle * math.Pi / 180.0

		// Explosion affects the midpoint position.
		_, ey := 0, 0
		explosion := c.seriesExplosionFor(0, i)
		if explosion > 0 {
			offset := explosion * r / 100
			ey = int(float64(offset) * math.Sin(midRad))
		}

		midY := cy + ey + int(float64(r)*math.Sin(midRad))

		slices = append(slices, sliceInfo{
			idx:        i,
			val:        val,
			angle:      angle,
			startAngle: startAngle,
			midAngle:   midAngle,
			midY:       midY,
		})
		startAngle += angle
	}

	// Sort back-to-front by screen-space y of outer midpoint.
	// Smaller y (higher on screen) = farther back = drawn first.
	sort.Slice(slices, func(i, j int) bool {
		return slices[i].midY < slices[j].midY
	})

	for _, sl := range slices {
		i := sl.idx
		angle := sl.angle
		sa := sl.startAngle * math.Pi / 180.0
		ea := (sl.startAngle + angle) * math.Pi / 180.0

		ex, ey := 0, 0
		explosion := c.seriesExplosionFor(0, i)
		if explosion > 0 {
			midRad := sl.midAngle * math.Pi / 180.0
			offset := explosion * r / 100
			ex = int(float64(offset) * math.Cos(midRad))
			ey = int(float64(offset) * math.Sin(midRad))
		}

		color := c.seriesFillColor(i)

		// Top-face arc endpoints.
		x1t := cx + ex + int(float64(r)*math.Cos(sa))
		y1t := cy + ey + int(float64(r)*math.Sin(sa))
		x2t := cx + ex + int(float64(r)*math.Cos(ea))
		y2t := cy + ey + int(float64(r)*math.Sin(ea))

		// Bottom-face arc endpoints (extruded downward).
		x1b := x1t
		y1b := y1t + int(extrude)
		x2b := x2t
		y2b := y2t + int(extrude)

		largeArc := 0
		if angle > 180 {
			largeArc = 1
		}

		wallColor := darkenColor(color, 20)

		if innerR > 0 {
			// Doughnut: outer wall, inner wall, top face.
			ix1t := cx + ex + int(float64(innerR)*math.Cos(sa))
			iy1t := cy + ey + int(float64(innerR)*math.Sin(sa))
			ix2t := cx + ex + int(float64(innerR)*math.Cos(ea))
			iy2t := cy + ey + int(float64(innerR)*math.Sin(ea))
			ix1b := ix1t
			iy1b := iy1t + int(extrude)
			ix2b := ix2t
			iy2b := iy2t + int(extrude)

			// Outer side wall.
			fmt.Fprintf(b, `<path d="M%d,%d A%d,%d 0 %d 1 %d,%d L%d,%d A%d,%d 0 %d 0 %d,%d Z" fill="#%s" stroke="white" stroke-width="0.5"/>`,
				x1t, y1t, r, r, largeArc, x2t, y2t, x2b, y2b, r, r, largeArc, x1b, y1b, wallColor)
			// Inner side wall.
			fmt.Fprintf(b, `<path d="M%d,%d A%d,%d 0 %d 0 %d,%d L%d,%d A%d,%d 0 %d 1 %d,%d Z" fill="#%s" stroke="white" stroke-width="0.5"/>`,
				ix1t, iy1t, innerR, innerR, largeArc, ix2t, iy2t, ix2b, iy2b, innerR, innerR, largeArc, ix1b, iy1b, wallColor)
			// Top face.
			fmt.Fprintf(b, `<path d="M%d,%d A%d,%d 0 %d 1 %d,%d L%d,%d A%d,%d 0 %d 0 %d,%d Z" fill="#%s" stroke="white" stroke-width="1"/>`,
				x1t, y1t, r, r, largeArc, x2t, y2t, ix2t, iy2t, innerR, innerR, largeArc, ix1t, iy1t, color)
		} else {
			// Solid pie: outer wall + top face.
			fmt.Fprintf(b, `<path d="M%d,%d A%d,%d 0 %d 1 %d,%d L%d,%d A%d,%d 0 %d 0 %d,%d Z" fill="#%s" stroke="white" stroke-width="0.5"/>`,
				x1t, y1t, r, r, largeArc, x2t, y2t, x2b, y2b, r, r, largeArc, x1b, y1b, wallColor)
			fmt.Fprintf(b, `<path d="M%d,%d L%d,%d A%d,%d 0 %d 1 %d,%d Z" fill="#%s" stroke="white" stroke-width="1"/>`,
				cx+ex, cy+ey, x1t, y1t, r, r, largeArc, x2t, y2t, color)
		}
	}

	// Labels (drawn on top of everything, at top-face positions).
	for _, sl := range slices {
		i := sl.idx
		val := sl.val
		labelRad := sl.midAngle * math.Pi / 180.0

		ex, ey := 0, 0
		explosion := c.seriesExplosionFor(0, i)
		if explosion > 0 {
			offset := explosion * r / 100
			ex = int(float64(offset) * math.Cos(labelRad))
			ey = int(float64(offset) * math.Sin(labelRad))
		}

		// Category label.
		if i < len(c.Categories) && c.Categories[i] != "" {
			labelR := r * 3 / 4
			if series.LabelPosition == 1 { // outEnd
				labelR = r + 25
			}
			lx := cx + ex + int(float64(labelR)*math.Cos(labelRad))
			ly := cy + ey + int(float64(labelR)*math.Sin(labelRad))
			anchor := "middle"
			if series.LabelPosition == 1 {
				if math.Cos(labelRad) >= 0 {
					anchor = "start"
				} else {
					anchor = "end"
				}
			}
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="%s" font-size="10" fill="#333">%s</text>`,
				lx, ly, anchor, escapeSVG(c.Categories[i]))
		}

		// Value / percent label.
		if series.ShowDataLabels || series.ShowPercent || series.ShowCatName || series.ShowSerName || series.ShowBubbleSize {
			valR := r / 2
			if innerR > 0 {
				valR = (r + innerR) / 2
			}
			if series.LabelPosition == 1 { // outEnd
				valR = r + 25
			}
			vx := cx + ex + int(float64(valR)*math.Cos(labelRad))
			vy := cy + ey + int(float64(valR)*math.Sin(labelRad))
			labelText := buildDataLabel(&series, i, val, total, c.Categories)
			style := dataLabelStyleAttrs(effectiveLabelStyle(&series, i), 9, "#fff")
			anchor := "middle"
			if series.LabelPosition == 1 {
				if math.Cos(labelRad) >= 0 {
					anchor = "start"
				} else {
					anchor = "end"
				}
			}
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="%s" %s>%s</text>`,
				vx, vy, anchor, style, escapeSVG(labelText))
		}

		// Leader line / callout.
		if series.ShowLeaderLines && (series.ShowDataLabels || series.ShowPercent || series.ShowCatName || series.ShowSerName || series.ShowBubbleSize) {
			if series.LabelPosition == 1 { // outEnd: L-shaped elbow callout
				stubR := r + 5
				fromX := cx + ex + int(float64(stubR)*math.Cos(labelRad))
				fromY := cy + ey + int(float64(stubR)*math.Sin(labelRad))
				toX := cx + ex + int(float64(r+25)*math.Cos(labelRad))
				toY := cy + ey + int(float64(r+25)*math.Sin(labelRad))
				horizontalFirst := math.Abs(math.Cos(labelRad)) >= math.Abs(math.Sin(labelRad))
				renderDataLabelElbowLine(b, fromX, fromY, toX, toY, horizontalFirst)
			} else {
				edgeR := r + 5
				x1 := cx + ex + int(float64(edgeR)*math.Cos(labelRad))
				y1 := cy + ey + int(float64(edgeR)*math.Sin(labelRad))
				x2 := cx + ex + int(float64(edgeR+15)*math.Cos(labelRad))
				y2 := cy + ey + int(float64(edgeR+15)*math.Sin(labelRad))
				fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#999" stroke-width="1"/>`,
					x1, y1, x2, y2)
			}
		}
	}
}

// --- 3D line chart renderer ---

// renderLine3DChart renders a 3D line chart with depth per series.
func (c *Chart) renderLine3DChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	c.renderAxes(b, x, y, w, h, false)

	maxVal := c.axisMax()
	if maxVal == 0 {
		maxVal = 1
	}

	points := len(c.Series[0].Values)
	seriesCount := len(c.Series)

	plotX := x + 50
	plotY := y
	plotW := w - 60
	plotH := h - 40

	view := view3DDefaults(c.View3D)
	proj := newProjector(view, rect3D{x: plotX, y: plotY, w: plotW, h: plotH})

	depthPct := float64(view.DepthPercent) / 100.0
	if depthPct <= 0 {
		depthPct = 1.0
	}

	// Each series gets a slice of the Z axis.
	var zStep float64
	if seriesCount > 1 {
		zStep = (0.8 * depthPct) / float64(seriesCount-1)
	} else {
		zStep = 0
	}

	for seriesIdx, series := range c.Series {
		color := c.seriesLineColor(seriesIdx)
		strokeWidth := 2
		if series.LineWidth > 0 {
			strokeWidth = series.LineWidth / 12700
			if strokeWidth < 1 {
				strokeWidth = 1
			}
		}

		z := 0.5
		if seriesCount > 1 {
			z = 0.5 - (0.4*depthPct) + float64(seriesIdx)*zStep
		}

		// Build projected points.
		var path strings.Builder
		path.WriteString("M")
		coords := make([][2]int, len(series.Values))
		for i, val := range series.Values {
			wx := 0.0
			if points > 1 {
				wx = float64(i) / float64(points-1)
			}
			wy := val / maxVal
			sx, sy := proj.projectInt(wx, wy, z)
			coords[i] = [2]int{sx, sy}
			if i > 0 {
				path.WriteString(" L")
			}
			fmt.Fprintf(&path, "%d,%d", sx, sy)
		}

		fmt.Fprintf(b, `<path d="%s" fill="none" stroke="#%s" stroke-width="%d" stroke-linecap="round" stroke-linejoin="round"%s/>`,
			path.String(), color, strokeWidth, strokeDashAttr(series.LinePattern))

		// Markers.
		markerColor := c.seriesMarkerColor(seriesIdx)
		markerSize := c.seriesMarkerSize(seriesIdx)
		for i := range series.Values {
			c.renderMarker(b, coords[i][0], coords[i][1], series.MarkerShape, markerColor, markerSize)
		}

		// Data labels.
		for i, val := range series.Values {
			if series.ShowDataLabels || series.ShowPercent || series.ShowCatName || series.ShowSerName || series.ShowBubbleSize {
				labelText := buildDataLabel(&series, i, val, 0, c.Categories)
				style := dataLabelStyleAttrs(effectiveLabelStyle(&series, i), 9, "#333")
				fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" %s>%s</text>`,
					coords[i][0], coords[i][1]-8, style, escapeSVG(labelText))
			}
		}
	}

	c.renderCategoryLabels(b, x, y, w, h, points)
}

// --- 3D area chart renderer ---

// renderArea3DChart renders a 3D area chart with depth per series.
func (c *Chart) renderArea3DChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	c.renderAxes(b, x, y, w, h, false)

	maxVal := c.axisMax()
	if maxVal == 0 {
		maxVal = 1
	}

	points := len(c.Series[0].Values)
	seriesCount := len(c.Series)

	plotX := x + 50
	plotY := y
	plotW := w - 60
	plotH := h - 40

	view := view3DDefaults(c.View3D)
	proj := newProjector(view, rect3D{x: plotX, y: plotY, w: plotW, h: plotH})

	depthPct := float64(view.DepthPercent) / 100.0
	if depthPct <= 0 {
		depthPct = 1.0
	}

	// Pre-compute stacked baseline if needed.
	stackedBase := make([][]float64, points)
	if c.Stacked || c.PercentStacked {
		for i := 0; i < points; i++ {
			stackedBase[i] = make([]float64, seriesCount)
			cum := 0.0
			for sIdx := 0; sIdx < seriesCount; sIdx++ {
				stackedBase[i][sIdx] = cum
				if i < len(c.Series[sIdx].Values) {
					cum += c.Series[sIdx].Values[i]
				}
			}
		}
	}

	var zStep float64
	if seriesCount > 1 {
		zStep = (0.8 * depthPct) / float64(seriesCount-1)
	} else {
		zStep = 0
	}

	// Draw fill polygons back-to-front so nearer series overlay farther ones.
	type areaSeries struct {
		idx   int
		z     float64
		depth float64 // for sorting
	}
	var order []areaSeries
	for sIdx := 0; sIdx < seriesCount; sIdx++ {
		z := 0.5
		if seriesCount > 1 {
			z = 0.5 - (0.4*depthPct) + float64(sIdx)*zStep
		}
		_, _, depth := proj.rawProject(0.5, 0.5, z)
		order = append(order, areaSeries{idx: sIdx, z: z, depth: depth})
	}
	sort.Slice(order, func(i, j int) bool {
		return order[i].depth < order[j].depth
	})

	for _, o := range order {
		seriesIdx := o.idx
		series := c.Series[seriesIdx]
		z := o.z
		color := c.seriesFillColor(seriesIdx)

		// Build polygon: data points left-to-right, then baseline right-to-left.
		var poly strings.Builder
		for i, val := range series.Values {
			wx := 0.0
			if points > 1 {
				wx = float64(i) / float64(points-1)
			}
			wy := val / maxVal
			if c.Stacked || c.PercentStacked {
				wy = (stackedBase[i][seriesIdx] + val) / maxVal
			}
			sx, sy := proj.projectInt(wx, wy, z)
			if i == 0 {
				fmt.Fprintf(&poly, "M%d,%d", sx, sy)
			} else {
				fmt.Fprintf(&poly, " L%d,%d", sx, sy)
			}
		}
		for i := len(series.Values) - 1; i >= 0; i-- {
			wx := 0.0
			if points > 1 {
				wx = float64(i) / float64(points-1)
			}
			wy := 0.0
			if c.Stacked || c.PercentStacked {
				wy = stackedBase[i][seriesIdx] / maxVal
			}
			sx, sy := proj.projectInt(wx, wy, z)
			fmt.Fprintf(&poly, " L%d,%d", sx, sy)
		}
		poly.WriteString(" Z")

		// Reduced opacity so overlapping areas remain visible.
		fmt.Fprintf(b, `<path d="%s" fill="#%s" fill-opacity="0.6" stroke="none"/>`,
			poly.String(), color)

		// Draw top edge as a stroke.
		var edge strings.Builder
		edge.WriteString("M")
		for i, val := range series.Values {
			wx := 0.0
			if points > 1 {
				wx = float64(i) / float64(points-1)
			}
			wy := val / maxVal
			if c.Stacked || c.PercentStacked {
				wy = (stackedBase[i][seriesIdx] + val) / maxVal
			}
			sx, sy := proj.projectInt(wx, wy, z)
			if i > 0 {
				edge.WriteString(" L")
			}
			fmt.Fprintf(&edge, "%d,%d", sx, sy)
		}
		strokeWidth := 2
		if series.LineWidth > 0 {
			strokeWidth = series.LineWidth / 12700
			if strokeWidth < 1 {
				strokeWidth = 1
			}
		}
		fmt.Fprintf(b, `<path d="%s" fill="none" stroke="#%s" stroke-width="%d" stroke-linecap="round" stroke-linejoin="round"%s/>`,
			edge.String(), color, strokeWidth, strokeDashAttr(series.LinePattern))
	}

	c.renderCategoryLabels(b, x, y, w, h, points)
}

// --- 3D surface chart renderer ---

// renderSurface3DChart renders a 3D surface chart with perspective.
func (c *Chart) renderSurface3DChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	plotX := x + 50
	plotY := y
	plotW := w - 60
	plotH := h - 40

	seriesCount := len(c.Series)
	points := len(c.Series[0].Values)
	if points == 0 {
		return
	}

	maxVal := c.axisMax()
	if maxVal == 0 {
		maxVal = 1
	}

	view := view3DDefaults(c.View3D)
	proj := newProjector(view, rect3D{x: plotX, y: plotY, w: plotW, h: plotH})

	depthPct := float64(view.DepthPercent) / 100.0
	if depthPct <= 0 {
		depthPct = 1.0
	}

	// Each series gets a slice of the Z axis.
	var zStep float64
	if seriesCount > 1 {
		zStep = (0.8 * depthPct) / float64(seriesCount-1)
	} else {
		zStep = 0
	}

	// Build projected grid cells.
	type cell3D struct {
		pts   [4][2]float64
		color string
		depth float64
	}
	var cells []cell3D

	for sIdx, series := range c.Series {
		z := 0.5
		if seriesCount > 1 {
			z = 0.5 - (0.4*depthPct) + float64(sIdx)*zStep
		}
		for i := 0; i < len(series.Values)-1 && i < points-1; i++ {
			wx0 := float64(i) / float64(points-1)
			wx1 := float64(i+1) / float64(points-1)

			v00 := 0.0
			if i < len(series.Values) {
				v00 = series.Values[i]
			}
			v01 := 0.0
			if i+1 < len(series.Values) {
				v01 = series.Values[i+1]
			}

			wy00 := v00 / maxVal
			wy01 := v01 / maxVal

			// For surface, we also interpolate to the next series if available.
			var v10, v11 float64
			if sIdx+1 < len(c.Series) {
				if i < len(c.Series[sIdx+1].Values) {
					v10 = c.Series[sIdx+1].Values[i]
				}
				if i+1 < len(c.Series[sIdx+1].Values) {
					v11 = c.Series[sIdx+1].Values[i+1]
				}
			} else {
				v10 = v00
				v11 = v01
			}
			wy10 := v10 / maxVal
			wy11 := v11 / maxVal

			// Average value for cell color.
			avgVal := (v00 + v01 + v10 + v11) / 4.0
			intensity := avgVal / maxVal
			r := int(255 * intensity)
			bb := int(255 * (1 - intensity))
			color := fmt.Sprintf("%02X00%02X", r, bb)

			// Next series Z.
			zNext := z
			if seriesCount > 1 && sIdx+1 < seriesCount {
				zNext = 0.5 - (0.4*depthPct) + float64(sIdx+1)*zStep
			}

			// Project 4 corners.
			var depth float64
			pts := [4][2]float64{}
			corners := [4][3]float64{{wx0, wy00, z}, {wx1, wy01, z}, {wx1, wy11, zNext}, {wx0, wy10, zNext}}
			for j, corner := range corners {
				sx, sy, sz := proj.rawProject(corner[0], corner[1], corner[2])
				pts[j] = [2]float64{sx, sy}
				depth += sz
			}
			depth /= 4

			cells = append(cells, cell3D{pts: pts, color: color, depth: depth})
		}
	}

	// Sort back-to-front.
	sort.Slice(cells, func(i, j int) bool {
		return cells[i].depth < cells[j].depth
	})

	// Emit cells.
	dx := proj.maxX - proj.minX
	dy := proj.maxY - proj.minY
	if dx <= 0 {
		dx = 1
	}
	if dy <= 0 {
		dy = 1
	}

	for _, cell := range cells {
		var ptsStr strings.Builder
		for j, pt := range cell.pts {
			if j > 0 {
				ptsStr.WriteByte(' ')
			}
			nx := (pt[0] - proj.minX) / dx
			ny := (pt[1] - proj.minY) / dy
			sx := float64(proj.plotRect.x) + nx*float64(proj.plotRect.w)
			sy := float64(proj.plotRect.y+proj.plotRect.h) - ny*float64(proj.plotRect.h)
			fmt.Fprintf(&ptsStr, "%.1f,%.1f", sx, sy)
		}
		fmt.Fprintf(b, `<polygon points="%s" fill="#%s" stroke="white" stroke-width="0.3"/>`,
			ptsStr.String(), cell.color)
	}

	c.renderCategoryLabels(b, x, y, w, h, points)
}
