package legacy

import (
	"encoding/binary"

	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

type officeArtPoint struct{ X, Y int32 }

const (
	officeArtPathLineTo  = uint16(0)
	officeArtPathCurveTo = uint16(1)
	officeArtPathMoveTo  = uint16(2)
	officeArtPathClose   = uint16(3)
	officeArtPathEnd     = uint16(4)
)

// DecodeCustomGeometry projects OfficeArt vertices/segmentInfo into the shared
// format-neutral geometry model. Empty/incomplete geometry returns GeometryNone.
func DecodeCustomGeometry(props []escher.Property) ir.Geometry {
	var (
		geoLeft, geoTop, geoRight, geoBottom int32
		vertices                              []officeArtPoint
		segments                              []uint16
		adjustments                           [8]*int32
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
		case escher.PropAdjust1, escher.PropAdjust2, escher.PropAdjust3, escher.PropAdjust4,
			escher.PropAdjust5, escher.PropAdjust6, escher.PropAdjust7, escher.PropAdjust8:
			index := int(prop.ID - escher.PropAdjust1)
			value := int32(prop.Value)
			adjustments[index] = &value
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
	flush := func() {
		if len(commands) == 0 {
			return
		}
		paths = append(paths, ir.Path{Commands: commands, FillRule: "nonzero", Fill: true, Stroke: true})
		commands = nil
	}
	for _, info := range segments {
		pathType := info & 0x7
		segmentCount := int(info >> 3)
		switch pathType {
		case officeArtPathMoveTo:
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
	for _, adjustment := range adjustments {
		if adjustment != nil {
			out.Adjustments = append(out.Adjustments, float64(*adjustment))
		}
	}
	return out
}

func parseOfficeArtVertexArray(blob []byte) []officeArtPoint {
	if len(blob) < 6 {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(blob[0:2]))
	cbElem := int(binary.LittleEndian.Uint16(blob[4:6]))
	if count <= 0 || cbElem <= 0 || 6+count*cbElem > len(blob) {
		return nil
	}
	out := make([]officeArtPoint, 0, count)
	for i := 0; i < count; i++ {
		off := 6 + i*cbElem
		switch {
		case cbElem >= 8:
			out = append(out, officeArtPoint{X: int32(binary.LittleEndian.Uint32(blob[off : off+4])), Y: int32(binary.LittleEndian.Uint32(blob[off+4 : off+8]))})
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
	cbElem := int(binary.LittleEndian.Uint16(blob[4:6]))
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
