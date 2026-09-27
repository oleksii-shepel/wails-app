package legacy

import (
	"encoding/binary"
	"math"

	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

type officeArtPoint struct{ X, Y int32 }

const (
	officeArtPathLineTo       = uint16(0)
	officeArtPathCurveTo      = uint16(1)
	officeArtPathMoveTo       = uint16(2)
	officeArtPathClose        = uint16(3)
	officeArtPathEnd          = uint16(4)
	officeArtPathEscape       = uint16(5)
	officeArtPathClientEscape = uint16(6)

	officeArtEscapeNoFill         = uint16(0x0A)
	officeArtEscapeNoLine         = uint16(0x0B)
	officeArtEscapeAutoLine       = uint16(0x0C)
	officeArtEscapeAutoCurve      = uint16(0x0D)
	officeArtEscapeCornerLine     = uint16(0x0E)
	officeArtEscapeCornerCurve    = uint16(0x0F)
	officeArtEscapeSmoothLine     = uint16(0x10)
	officeArtEscapeSmoothCurve    = uint16(0x11)
	officeArtEscapeSymmetricLine  = uint16(0x12)
	officeArtEscapeSymmetricCurve = uint16(0x13)
	officeArtEscapeFreeform       = uint16(0x14)
)

func officeArtPathInfo(pathType uint16, segmentCount int) uint16 {
	return (pathType << 13) | uint16(segmentCount&0x1fff)
}

func officeArtEscapeInfo(escape uint16, segmentCount int) uint16 {
	return (officeArtPathEscape << 13) | ((escape & 0x1f) << 8) | uint16(segmentCount&0xff)
}

// DecodeArcGeometry materializes the special OfficeArt mso_sptArc preset into
// canonical custom paths. Legacy Arc uses a sector-sized shape coordinate box
// while pVertices[0:2] describe the full ellipse rectangle; its first path is
// the clockwise arc stroke and the second repeats the arc and closes to the
// ellipse center for fill semantics. Materializing this source geometry makes
// the canonical IR independent of whether a producer kept mso_sptArc or
// exported its equivalent custom DrawingML path.
func DecodeArcGeometry(props []escher.Property) ir.Geometry {
	const (
		defaultViewSize = 21600.0
		defaultEllipseL = -21600.0
		defaultEllipseT = 0.0
		defaultEllipseR = 21600.0
		defaultEllipseB = 43200.0
	)

	viewL, viewT, viewR, viewB := 0.0, 0.0, defaultViewSize, defaultViewSize
	startDeg, endDeg := -90.0, 0.0
	var vertexBlob []byte
	for _, prop := range props {
		if prop.IsComplex {
			if prop.ID == escher.PropVertices {
				vertexBlob = prop.Complex
			}
			continue
		}
		if prop.IsBlip {
			continue
		}
		switch prop.ID {
		case escher.PropGeoLeft:
			viewL = float64(int32(prop.Value))
		case escher.PropGeoTop:
			viewT = float64(int32(prop.Value))
		case escher.PropGeoRight:
			viewR = float64(int32(prop.Value))
		case escher.PropGeoBottom:
			viewB = float64(int32(prop.Value))
		case escher.PropAdjust1:
			startDeg = float64(int32(prop.Value)) / 65536.0
		case escher.PropAdjust2:
			endDeg = float64(int32(prop.Value)) / 65536.0
		}
	}
	if viewR == viewL || viewB == viewT {
		return ir.Geometry{}
	}

	ellipseL, ellipseT, ellipseR, ellipseB := defaultEllipseL, defaultEllipseT, defaultEllipseR, defaultEllipseB
	if vertices := parseOfficeArtVertexArrayMode(vertexBlob, true); len(vertices) >= 2 {
		ellipseL, ellipseT = float64(vertices[0].X), float64(vertices[0].Y)
		ellipseR, ellipseB = float64(vertices[1].X), float64(vertices[1].Y)
	}
	if ellipseR == ellipseL || ellipseB == ellipseT {
		return ir.Geometry{}
	}

	cx, cy := (ellipseL+ellipseR)/2, (ellipseT+ellipseB)/2
	rx, ry := (ellipseR-ellipseL)/2, (ellipseB-ellipseT)/2
	viewW, viewH := viewR-viewL, viewB-viewT
	norm := func(x, y float64) ir.PointF {
		return ir.PointF{X: (x - viewL) / viewW, Y: (y - viewT) / viewH}
	}

	// OfficeArt Arc is clockwise in its y-down coordinate system. Split the
	// sweep at quarter turns so each cubic remains a stable ellipse approximation.
	sweepDeg := math.Mod(endDeg-startDeg, 360.0)
	if sweepDeg < 0 {
		sweepDeg += 360.0
	}
	if sweepDeg == 0 && startDeg != endDeg {
		sweepDeg = 360.0
	}
	// Split at absolute quadrant boundaries. The legacy msoArc prototype keeps
	// those boundaries as distinct curve segments (for example 269.3°->270°
	// followed by 270°->360°), rather than dividing the total sweep evenly.
	endUnwrapped := startDeg + sweepDeg
	segmentEnds := make([]float64, 0, 5)
	nextQuadrant := (math.Floor(startDeg/90.0) + 1.0) * 90.0
	for nextQuadrant < endUnwrapped-1e-10 {
		segmentEnds = append(segmentEnds, nextQuadrant)
		nextQuadrant += 90.0
	}
	segmentEnds = append(segmentEnds, endUnwrapped)

	theta := startDeg * math.Pi / 180.0
	start := norm(cx+rx*math.Cos(theta), cy+ry*math.Sin(theta))
	arcCommands := []ir.PathCommand{{Op: "M", Points: []ir.PointF{start}}}
	for _, segmentEndDeg := range segmentEnds {
		next := segmentEndDeg * math.Pi / 180.0
		k := 4.0 / 3.0 * math.Tan((next-theta)/4.0)
		x0, y0 := cx+rx*math.Cos(theta), cy+ry*math.Sin(theta)
		x1, y1 := cx+rx*math.Cos(next), cy+ry*math.Sin(next)
		c1 := norm(x0-k*rx*math.Sin(theta), y0+k*ry*math.Cos(theta))
		c2 := norm(x1+k*rx*math.Sin(next), y1-k*ry*math.Cos(next))
		end := norm(x1, y1)
		arcCommands = append(arcCommands, ir.PathCommand{Op: "C", Points: []ir.PointF{c1, c2, end}})
		theta = next
	}

	stroke := ir.Path{Commands: append([]ir.PathCommand(nil), arcCommands...), FillRule: "nonzero", Fill: false, Stroke: true}
	fillCommands := append([]ir.PathCommand(nil), arcCommands...)
	fillCommands = append(fillCommands, ir.PathCommand{Op: "L", Points: []ir.PointF{norm(cx, cy)}}, ir.PathCommand{Op: "Z"})
	fill := ir.Path{Commands: fillCommands, FillRule: "nonzero", Fill: true, Stroke: false}
	return ir.Geometry{
		Kind:  ir.GeometryCustom,
		Paths: []ir.Path{stroke, fill},
		ViewBox: ir.Rect{
			X: int64(viewL), Y: int64(viewT), W: int64(viewW), H: int64(viewH),
		},
	}
}

// DecodeCustomGeometry projects OfficeArt vertices/segmentInfo into the shared
// format-neutral geometry model. Empty/incomplete geometry returns GeometryNone.
func DecodeCustomGeometry(props []escher.Property) ir.Geometry {
	var (
		geoLeft, geoTop, geoRight, geoBottom int32
		vertices                             []officeArtPoint
		segments                             []uint16
	)
	for _, prop := range props {
		if prop.IsComplex {
			switch prop.ID {
			case escher.PropVertices:
				vertices = parseOfficeArtVertexArray(prop.Complex)
			case escher.PropSegmentInfo:
				segments = parseOfficeArtSegmentArray(prop.Complex)
			}
			continue
		}
		if prop.IsBlip {
			continue
		}
		switch prop.ID {
		case escher.PropGeoLeft:
			geoLeft = int32(prop.Value)
		case escher.PropGeoTop:
			geoTop = int32(prop.Value)
		case escher.PropGeoRight:
			geoRight = int32(prop.Value)
		case escher.PropGeoBottom:
			geoBottom = int32(prop.Value)
		}
	}
	if len(vertices) == 0 || len(segments) == 0 {
		return ir.Geometry{}
	}
	left, top := float64(geoLeft), float64(geoTop)
	width := float64(geoRight - geoLeft)
	height := float64(geoBottom - geoTop)
	if width == 0 {
		width = 21600
	}
	if height == 0 {
		height = 21600
	}
	point := func(v officeArtPoint) ir.PointF {
		return ir.PointF{X: (float64(v.X) - left) / width, Y: (float64(v.Y) - top) / height}
	}

	var paths []ir.Path
	var commands []ir.PathCommand
	vertexIndex := 0
	pathFill, pathStroke := true, true
	flush := func() {
		if len(commands) == 0 {
			pathFill, pathStroke = true, true
			return
		}
		paths = append(paths, ir.Path{Commands: commands, FillRule: "nonzero", Fill: pathFill, Stroke: pathStroke})
		commands = nil
		pathFill, pathStroke = true, true
	}
	for _, info := range segments {
		// MSOPATHINFO stores the path type in the high three bits and the
		// segment count in the low 13 bits. Escape records reuse bits 8..12
		// for MSOPATHESCAPE and the low byte for their count.
		pathType := info >> 13
		segmentCount := int(info & 0x1fff)
		switch pathType {
		case officeArtPathMoveTo:
			// MoveTo consumes one POINT even though the encoded count is normally 0.
			if vertexIndex < len(vertices) {
				commands = append(commands, ir.PathCommand{Op: "M", Points: []ir.PointF{point(vertices[vertexIndex])}})
				vertexIndex++
			}
		case officeArtPathLineTo:
			for i := 0; i < segmentCount && vertexIndex < len(vertices); i++ {
				commands = append(commands, ir.PathCommand{Op: "L", Points: []ir.PointF{point(vertices[vertexIndex])}})
				vertexIndex++
			}
		case officeArtPathCurveTo:
			for i := 0; i < segmentCount && vertexIndex+2 < len(vertices); i++ {
				commands = append(commands, ir.PathCommand{Op: "C", Points: []ir.PointF{
					point(vertices[vertexIndex]), point(vertices[vertexIndex+1]), point(vertices[vertexIndex+2]),
				}})
				vertexIndex += 3
			}
		case officeArtPathClose:
			commands = append(commands, ir.PathCommand{Op: "Z"})
		case officeArtPathEnd:
			flush()
		case officeArtPathEscape:
			escape := (info >> 8) & 0x1f
			switch escape {
			case officeArtEscapeNoFill:
				pathFill = false
			case officeArtEscapeNoLine:
				pathStroke = false
			case officeArtEscapeAutoLine, officeArtEscapeAutoCurve,
				officeArtEscapeCornerLine, officeArtEscapeCornerCurve,
				officeArtEscapeSmoothLine, officeArtEscapeSmoothCurve,
				officeArtEscapeSymmetricLine, officeArtEscapeSymmetricCurve,
				officeArtEscapeFreeform:
				// Editing/tangent metadata. It does not consume POINT records.
			}
		case officeArtPathClientEscape:
			// Client-specific editing metadata does not define drawable segments.
		}
	}
	flush()
	if len(paths) == 0 {
		return ir.Geometry{}
	}
	out := ir.Geometry{
		Kind:    ir.GeometryCustom,
		Paths:   paths,
		ViewBox: ir.Rect{X: int64(geoLeft), Y: int64(geoTop), W: int64(geoRight - geoLeft), H: int64(geoBottom - geoTop)},
	}
	out.Adjustments = DecodeGeometryAdjustments(props)
	return out
}

// DecodeGeometryAdjustments preserves authored OfficeArt adjustValue1..8 slots
// for both preset and custom geometry. Missing intermediate slots remain zero so
// later shape-specific normalization cannot accidentally shift adjustValue2 into
// slot 1.
func DecodeGeometryAdjustments(props []escher.Property) []float64 {
	values, _ := DecodeGeometryAdjustmentSlots(props)
	return values
}

// DecodeGeometryAdjustmentSlots preserves both the positional value array and
// which OfficeArt adjustValue slots were actually authored. A zero adjustment
// is a valid value for several AutoShapes, so downstream shape-specific
// normalization must not confuse a missing slot with an authored zero.
func DecodeGeometryAdjustmentSlots(props []escher.Property) ([]float64, []bool) {
	var values [8]float64
	var present [8]bool
	last := -1
	for _, prop := range props {
		if prop.IsComplex || prop.IsBlip || prop.ID < escher.PropAdjust1 || prop.ID > escher.PropAdjust8 {
			continue
		}
		index := int(prop.ID - escher.PropAdjust1)
		values[index] = float64(int32(prop.Value))
		present[index] = true
		if index > last {
			last = index
		}
	}
	if last < 0 {
		return nil, nil
	}
	out := make([]float64, last+1)
	has := make([]bool, last+1)
	copy(out, values[:last+1])
	copy(has, present[:last+1])
	return out, has
}

func parseOfficeArtVertexArray(blob []byte) []officeArtPoint {
	return parseOfficeArtVertexArrayMode(blob, false)
}

func parseOfficeArtVertexArrayMode(blob []byte, unsignedCompact bool) []officeArtPoint {
	if len(blob) < 6 {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(blob[0:2]))
	cbElem := escher.OfficeArtArrayElementSize(binary.LittleEndian.Uint16(blob[4:6]))
	if count <= 0 || cbElem <= 0 || 6+count*cbElem > len(blob) {
		return nil
	}
	out := make([]officeArtPoint, 0, count)
	for i := 0; i < count; i++ {
		off := 6 + i*cbElem
		switch {
		case cbElem >= 8:
			out = append(out, officeArtPoint{X: int32(binary.LittleEndian.Uint32(blob[off : off+4])), Y: int32(binary.LittleEndian.Uint32(blob[off+4 : off+8]))})
		case cbElem >= 4 && unsignedCompact:
			out = append(out, officeArtPoint{X: int32(binary.LittleEndian.Uint16(blob[off : off+2])), Y: int32(binary.LittleEndian.Uint16(blob[off+2 : off+4]))})
		case cbElem >= 4:
			out = append(out, officeArtPoint{X: int32(int16(binary.LittleEndian.Uint16(blob[off : off+2]))), Y: int32(int16(binary.LittleEndian.Uint16(blob[off+2 : off+4])))})
		default:
			return nil
		}
	}
	return out
}

func parseOfficeArtSegmentArray(blob []byte) []uint16 {
	if len(blob) < 6 {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(blob[0:2]))
	cbElem := escher.OfficeArtArrayElementSize(binary.LittleEndian.Uint16(blob[4:6]))
	if count <= 0 || cbElem < 2 || 6+count*cbElem > len(blob) {
		return nil
	}
	out := make([]uint16, 0, count)
	for i := 0; i < count; i++ {
		off := 6 + i*cbElem
		out = append(out, binary.LittleEndian.Uint16(blob[off:off+2]))
	}
	return out
}
