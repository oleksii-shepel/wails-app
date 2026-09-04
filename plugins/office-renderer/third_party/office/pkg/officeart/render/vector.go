package render

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"

	"office/pkg/officeart/ir"
)

func renderVectorShape(b *strings.Builder, shape *ir.Shape) {
	if shape == nil {
		return
	}
	if shape.Kind == ir.ShapeConnector {
		renderConnectorSVG(b, shape)
		return
	}
	if len(shape.Geometry.Paths) > 0 {
		renderGeometrySVG(b, shape)
	}
}

func renderConnectorSVG(b *strings.Builder, shape *ir.Shape) {
	if !shape.Line.Visible {
		return
	}

	points := connectorPoints(shape)
	if len(points) < 2 {
		return
	}

	w := pt(shape.Transform.Rect.W)
	h := pt(shape.Transform.Rect.H)
	viewW := math.Max(math.Abs(w), 1)
	viewH := math.Max(math.Abs(h), 1)

	fmt.Fprintf(b, `<svg class="officeart-vector officeart-connector" viewBox="0 0 %.6f %.6f" preserveAspectRatio="none" style="position:absolute;left:0;top:0;width:100%%;height:100%%;overflow:visible">`,
		viewW, viewH)

	startArrow := shape.Line.StartArrow
	endArrow := shape.Line.EndArrow
	if shape.Connector != nil {
		if shape.Connector.StartArrow.Type != "" {
			startArrow = shape.Connector.StartArrow
		}
		if shape.Connector.EndArrow.Type != "" {
			endArrow = shape.Connector.EndArrow
		}
	}

	startID := ""
	endID := ""
	if startArrow.Type != "" {
		startID = markerID(shape.ID, "start")
		renderArrowMarker(b, startID, startArrow, shape.Line)
	}
	if endArrow.Type != "" {
		endID = markerID(shape.ID, "end")
		renderArrowMarker(b, endID, endArrow, shape.Line)
	}

	var attrs strings.Builder
	writeSVGLineStyle(&attrs, shape.Line)
	if startID != "" {
		fmt.Fprintf(&attrs, ` marker-start="url(#%s)"`, html.EscapeString(startID))
	}
	if endID != "" {
		fmt.Fprintf(&attrs, ` marker-end="url(#%s)"`, html.EscapeString(endID))
	}

	if len(points) == 2 {
		fmt.Fprintf(b, `<line x1="%.6f" y1="%.6f" x2="%.6f" y2="%.6f"%s/>`,
			points[0].X, points[0].Y, points[1].X, points[1].Y, attrs.String())
	} else {
		var coords strings.Builder
		for i, p := range points {
			if i > 0 {
				coords.WriteByte(' ')
			}
			fmt.Fprintf(&coords, "%.6f,%.6f", p.X, p.Y)
		}
		fmt.Fprintf(b, `<polyline points="%s" fill="none"%s/>`, coords.String(), attrs.String())
	}
	b.WriteString(`</svg>`)
}

type localPoint struct {
	X float64
	Y float64
}

func connectorPoints(shape *ir.Shape) []localPoint {
	rect := shape.Transform.Rect
	toLocal := func(p ir.Point) localPoint {
		return localPoint{
			X: pt(p.X - rect.X),
			Y: pt(p.Y - rect.Y),
		}
	}

	if shape.Connector != nil {
		if len(shape.Connector.Path) >= 2 {
			out := make([]localPoint, 0, len(shape.Connector.Path))
			for _, p := range shape.Connector.Path {
				out = append(out, toLocal(p))
			}
			return out
		}
		if shape.Connector.Start != nil && shape.Connector.End != nil {
			return []localPoint{toLocal(*shape.Connector.Start), toLocal(*shape.Connector.End)}
		}
	}

	return []localPoint{
		{X: 0, Y: 0},
		{X: pt(rect.W), Y: pt(rect.H)},
	}
}

func renderGeometrySVG(b *strings.Builder, shape *ir.Shape) {
	paths := shape.Geometry.Paths
	if len(paths) == 0 {
		return
	}
	vb := geometryViewBox(shape.Geometry)
	fmt.Fprintf(b, `<svg class="officeart-vector officeart-geometry" viewBox="%s %s %s %s" preserveAspectRatio="none" style="position:absolute;left:0;top:0;width:100%%;height:100%%;overflow:visible">`,
		num(float64(vb.X)), num(float64(vb.Y)), num(float64(vb.W)), num(float64(vb.H)))

	for _, path := range paths {
		d := svgPathData(path)
		if d == "" {
			continue
		}
		fill := "none"
		if path.Fill {
			fill = svgFill(shape.Fill)
		}
		stroke := "none"
		strokeWidth := 0.0
		dash := ""
		if path.Stroke && shape.Line.Visible {
			stroke = svgStroke(shape.Line)
			strokeWidth = shape.Line.WidthPt
			if strokeWidth <= 0 {
				strokeWidth = 1
			}
			dash = svgDashArray(shape.Line.Dash, strokeWidth)
		}
		fmt.Fprintf(b, `<path d="%s" fill="%s" stroke="%s"`,
			html.EscapeString(d), html.EscapeString(fill), html.EscapeString(stroke))
		if stroke != "none" {
			fmt.Fprintf(b, ` stroke-width="%g"`, strokeWidth)
			if dash != "" {
				fmt.Fprintf(b, ` stroke-dasharray="%s"`, html.EscapeString(dash))
			}
		}
		if path.FillRule != "" {
			fmt.Fprintf(b, ` fill-rule="%s"`, html.EscapeString(path.FillRule))
		}
		b.WriteString(`/>`)
	}
	b.WriteString(`</svg>`)
}

func geometryViewBox(g ir.Geometry) ir.Rect {
	if g.ViewBox.W > 0 && g.ViewBox.H > 0 {
		return g.ViewBox
	}

	first := true
	var minX, minY, maxX, maxY float64
	for _, path := range g.Paths {
		for _, cmd := range path.Commands {
			for _, p := range cmd.Points {
				if first {
					minX, minY, maxX, maxY = p.X, p.Y, p.X, p.Y
					first = false
					continue
				}
				minX = math.Min(minX, p.X)
				minY = math.Min(minY, p.Y)
				maxX = math.Max(maxX, p.X)
				maxY = math.Max(maxY, p.Y)
			}
		}
	}
	if first {
		return ir.Rect{W: 1, H: 1}
	}
	w := maxX - minX
	h := maxY - minY
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	return ir.Rect{
		X: int64(math.Floor(minX)),
		Y: int64(math.Floor(minY)),
		W: int64(math.Ceil(w)),
		H: int64(math.Ceil(h)),
	}
}

func svgPathData(path ir.Path) string {
	var b strings.Builder
	for _, cmd := range path.Commands {
		op := strings.ToUpper(strings.TrimSpace(cmd.Op))
		switch op {
		case "M", "L":
			if len(cmd.Points) < 1 {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%s %s %s", op, num(cmd.Points[0].X), num(cmd.Points[0].Y))
		case "C":
			if len(cmd.Points) < 3 {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "C %s %s %s %s %s %s",
				num(cmd.Points[0].X), num(cmd.Points[0].Y),
				num(cmd.Points[1].X), num(cmd.Points[1].Y),
				num(cmd.Points[2].X), num(cmd.Points[2].Y))
		case "Q":
			if len(cmd.Points) < 2 {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "Q %s %s %s %s",
				num(cmd.Points[0].X), num(cmd.Points[0].Y),
				num(cmd.Points[1].X), num(cmd.Points[1].Y))
		case "Z":
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteByte('Z')
		}
	}
	return b.String()
}

func writeSVGLineStyle(b *strings.Builder, line ir.Line) {
	color := svgStroke(line)
	width := line.WidthPt
	if width <= 0 {
		width = 1
	}
	fmt.Fprintf(b, ` stroke="%s" stroke-width="%g" fill="none"`, html.EscapeString(color), width)
	if dash := svgDashArray(line.Dash, width); dash != "" {
		fmt.Fprintf(b, ` stroke-dasharray="%s"`, dash)
	}
	if line.Cap != "" {
		fmt.Fprintf(b, ` stroke-linecap="%s"`, html.EscapeString(line.Cap))
	}
	if line.Join != "" {
		fmt.Fprintf(b, ` stroke-linejoin="%s"`, html.EscapeString(line.Join))
	}
}

func svgStroke(line ir.Line) string {
	if line.Color == "" {
		return "#000000"
	}
	return cssColor(line.Color, line.Alpha)
}

func svgFill(fill ir.Fill) string {
	switch fill.Kind {
	case ir.FillNone:
		return "none"
	case ir.FillSolid:
		if fill.Color != "" {
			return cssColor(fill.Color, fill.Alpha)
		}
	}
	return "none"
}

func svgDashArray(dash string, width float64) string {
	if width <= 0 {
		width = 1
	}
	switch strings.ToLower(strings.TrimSpace(dash)) {
	case "", "solid":
		return ""
	case "dot", "sysdot":
		return fmt.Sprintf("%g %g", width, width*2)
	case "dash", "sysdash":
		return fmt.Sprintf("%g %g", width*4, width*2)
	case "dashdot", "sysdashdot":
		return fmt.Sprintf("%g %g %g %g", width*4, width*2, width, width*2)
	case "longdash", "lgdash":
		return fmt.Sprintf("%g %g", width*8, width*3)
	case "longdashdot", "lgdashdot":
		return fmt.Sprintf("%g %g %g %g", width*8, width*3, width, width*3)
	default:
		return fmt.Sprintf("%g %g", width*4, width*2)
	}
}

func markerID(id uint32, side string) string {
	return fmt.Sprintf("officeart-arrow-%d-%s", id, side)
}

func renderArrowMarker(b *strings.Builder, id string, arrow ir.Arrow, line ir.Line) {
	mw := arrowSize(arrow.Length)
	mh := arrowSize(arrow.Width)
	color := svgStroke(line)

	fmt.Fprintf(b, `<defs><marker id="%s" viewBox="0 0 10 10" refX="10" refY="5" markerUnits="strokeWidth" markerWidth="%g" markerHeight="%g" orient="auto-start-reverse">`,
		html.EscapeString(id), mw, mh)

	switch strings.ToLower(strings.TrimSpace(arrow.Type)) {
	case "triangle":
		fmt.Fprintf(b, `<path d="M 0 0 L 10 5 L 0 10 Z" fill="%s"/>`, html.EscapeString(color))
	case "stealth":
		fmt.Fprintf(b, `<path d="M 0 0 L 10 5 L 0 10 L 3 5 Z" fill="%s"/>`, html.EscapeString(color))
	case "diamond":
		fmt.Fprintf(b, `<path d="M 0 5 L 5 0 L 10 5 L 5 10 Z" fill="%s"/>`, html.EscapeString(color))
	case "oval":
		fmt.Fprintf(b, `<ellipse cx="5" cy="5" rx="4.5" ry="4" fill="%s"/>`, html.EscapeString(color))
	case "arrow", "open":
		fmt.Fprintf(b, `<path d="M 0 0 L 10 5 L 0 10" fill="none" stroke="%s" stroke-width="1.5"/>`, html.EscapeString(color))
	default:
		// Unknown endpoint decorations stay non-destructive: no visible marker.
	}
	b.WriteString(`</marker></defs>`)
}

func arrowSize(size string) float64 {
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "sm":
		return 3.5
	case "lg":
		return 7
	default:
		return 5
	}
}

func num(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
