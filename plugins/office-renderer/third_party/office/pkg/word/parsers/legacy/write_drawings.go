package legacy

import (
	"bytes"
	"encoding/binary"
	"math"
	"sort"
	"strconv"
	"strings"

	dml "office/pkg/ooxml/drawing"
	wordir "office/pkg/word/ir"
)

func buildLegacyPlcfSpaData(images []legacyFloatingImageWrite, mainTextLen int) []byte {
	return buildLegacyPlcfSpaDataWithShapes(images, nil, mainTextLen)
}

type legacyFloatingAnchorWrite struct {
	CP, Left, Top, Width, Height int
	ShapeID                      uint32
	Flags                        uint16
}

func buildLegacyPlcfSpaDataWithShapes(images []legacyFloatingImageWrite, shapes []legacyFloatingShapeWrite, mainTextLen int) []byte {
	if len(images) == 0 && len(shapes) == 0 {
		return nil
	}
	ordered := make([]legacyFloatingAnchorWrite, 0, len(images)+len(shapes))
	for _, item := range images {
		img := item.Image
		if img == nil {
			img = &wordir.Image{}
		}
		ordered = append(ordered, legacyFloatingAnchorWrite{
			CP: item.CP, ShapeID: item.Shape,
			Left: int(legacyEMUToTwips(img.X)), Top: int(legacyEMUToTwips(img.Y)),
			Width: int(legacyEMUToTwips(img.Width)), Height: int(legacyEMUToTwips(img.Height)),
			Flags: legacyFSPAFlags(img.XRelative, img.YRelative, img.WrapType, img.WrapSide, img.BehindDoc),
		})
	}
	for _, item := range shapes {
		shape := item.Shape
		if shape == nil {
			shape = &wordir.Shape{}
		}
		ordered = append(ordered, legacyFloatingAnchorWrite{
			CP: item.CP, ShapeID: item.ShapeID,
			Left: int(legacyEMUToTwips(shape.X)), Top: int(legacyEMUToTwips(shape.Y)),
			Width: int(legacyEMUToTwips(shape.Width)), Height: int(legacyEMUToTwips(shape.Height)),
			Flags: legacyFSPAFlags(shape.XRelative, shape.YRelative, shape.WrapType, shape.WrapSide, shape.BehindDoc),
		})
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].CP < ordered[j].CP })

	var out bytes.Buffer
	for _, item := range ordered {
		_ = binary.Write(&out, binary.LittleEndian, uint32(maxLegacyInt(item.CP, 0)))
	}
	_ = binary.Write(&out, binary.LittleEndian, uint32(maxLegacyInt(mainTextLen, 0)))

	for _, item := range ordered {
		left, top, width, height := item.Left, item.Top, item.Width, item.Height
		if width <= 0 {
			width = 1
		}
		if height <= 0 {
			height = 1
		}
		_ = binary.Write(&out, binary.LittleEndian, item.ShapeID)
		_ = binary.Write(&out, binary.LittleEndian, int32(left))
		_ = binary.Write(&out, binary.LittleEndian, int32(top))
		_ = binary.Write(&out, binary.LittleEndian, int32(left+width))
		_ = binary.Write(&out, binary.LittleEndian, int32(top+height))
		_ = binary.Write(&out, binary.LittleEndian, item.Flags)
		// FSPA's final 4 bytes contain cTxbx and reserved data. They remain zero
		// for shapes without a linked textbox story.
		_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	}
	return out.Bytes()
}

func legacyFSPAFlagsForImage(img *wordir.Image) uint16 {
	if img == nil {
		return 0
	}
	return legacyFSPAFlags(img.XRelative, img.YRelative, img.WrapType, img.WrapSide, img.BehindDoc)
}

func legacyFSPAFlags(xRelative, yRelative, wrapType, wrapSide string, behindDoc bool) uint16 {
	var flags uint16
	flags |= legacyAnchorRelativeBits(xRelative, false) << 1
	flags |= legacyAnchorRelativeBits(yRelative, true) << 3

	var wrap uint16
	switch wrapType {
	case "topAndBottom":
		wrap = 1
	case "none":
		wrap = 3
	case "tight":
		wrap = 4
	case "through":
		wrap = 5
	default:
		wrap = 0 // square
	}
	flags |= wrap << 5

	var side uint16
	switch wrapSide {
	case "left":
		side = 1
	case "right":
		side = 2
	case "largest":
		side = 3
	}
	flags |= side << 9
	if behindDoc && wrapType == "none" {
		flags |= 0x4000
	}
	return flags
}

func buildLegacyFloatingShapeOfficeArt(shapes []legacyFloatingShapeWrite) []byte {
	var out bytes.Buffer
	nextChildID := uint32(0x10000)
	for _, item := range shapes {
		if item.Shape == nil {
			continue
		}
		out.Write(buildLegacyShapeOfficeArt(item.ShapeID, item.Shape, nil, &nextChildID))
	}
	return out.Bytes()
}

type legacyOfficeArtRect struct {
	Left, Top, Right, Bottom int32
}

func buildLegacyShapeOfficeArt(spid uint32, shape *wordir.Shape, childAnchor *legacyOfficeArtRect, nextChildID *uint32) []byte {
	if shape == nil {
		return nil
	}
	if len(shape.Children) == 0 {
		return buildLegacyShapeSpContainer(spid, shape, childAnchor, false)
	}

	groupSp := buildLegacyGroupShapeSpContainer(spid, shape, childAnchor)
	payload := append([]byte(nil), groupSp...)
	for i := range shape.Children {
		child := &shape.Children[i]
		childID := *nextChildID
		*nextChildID++
		anchor := legacyOfficeArtRect{
			Left:   int32(legacyEMUToTwips(child.X)),
			Top:    int32(legacyEMUToTwips(child.Y)),
			Right:  int32(legacyEMUToTwips(child.X + maxLegacyInt64(child.Width, 635))),
			Bottom: int32(legacyEMUToTwips(child.Y + maxLegacyInt64(child.Height, 635))),
		}
		payload = append(payload, buildLegacyShapeOfficeArt(childID, child, &anchor, nextChildID)...)
	}
	return buildLegacyOfficeArtRecord(0x000F, legacyOfficeArtSpgrContainer, payload)
}

func buildLegacyGroupShapeSpContainer(spid uint32, shape *wordir.Shape, childAnchor *legacyOfficeArtRect) []byte {
	width := int32(legacyEMUToTwips(maxLegacyInt64(shape.Width, 635)))
	height := int32(legacyEMUToTwips(maxLegacyInt64(shape.Height, 635)))
	fspgrPayload := make([]byte, 16)
	binary.LittleEndian.PutUint32(fspgrPayload[8:12], uint32(width))
	binary.LittleEndian.PutUint32(fspgrPayload[12:16], uint32(height))
	fspgr := buildLegacyOfficeArtRecord(0x0001, legacyOfficeArtFSPGRRecord, fspgrPayload)
	sp := buildLegacyShapeSpContainerPayload(spid, shape, childAnchor, true)
	return buildLegacyOfficeArtRecord(0x000F, legacyOfficeArtSpContainer, append(fspgr, sp...))
}

func buildLegacyShapeSpContainer(spid uint32, shape *wordir.Shape, extras ...any) []byte {
	var childAnchor *legacyOfficeArtRect
	var group bool
	if len(extras) > 0 {
		if value, ok := extras[0].(*legacyOfficeArtRect); ok {
			childAnchor = value
		}
	}
	if len(extras) > 1 {
		if value, ok := extras[1].(bool); ok {
			group = value
		}
	}
	return buildLegacyOfficeArtRecord(0x000F, legacyOfficeArtSpContainer, buildLegacyShapeSpContainerPayload(spid, shape, childAnchor, group))
}

func buildLegacyShapeSpContainerPayload(spid uint32, shape *wordir.Shape, childAnchor *legacyOfficeArtRect, group bool) []byte {
	if shape == nil {
		return nil
	}
	shapeType := legacyGeometryToOfficeArtShapeType(shape)
	if shape.GeometryData != nil && shape.GeometryData.Type == dml.GeometryTypeCustom {
		shapeType = 0
	}
	var flags uint32 = 0x0200 // fHaveAnchor
	if group {
		flags |= 0x0001 // fGroup
		if childAnchor == nil {
			flags |= 0x0004 // fPatriarch for the top-level group shape
		}
	}
	if childAnchor != nil {
		flags |= 0x0002 // fChild
	}
	if shape.FlipV {
		flags |= 0x40
	}
	if shape.FlipH {
		flags |= 0x80
	}
	fspPayload := make([]byte, 8)
	binary.LittleEndian.PutUint32(fspPayload[0:4], spid)
	binary.LittleEndian.PutUint32(fspPayload[4:8], flags)
	fsp := buildLegacyOfficeArtRecord((shapeType<<4)|0x2, legacyOfficeArtFSPRecord, fspPayload)

	props := legacyShapeOfficeArtProperties(shape)
	fopt := buildLegacyOfficeArtFOPT(props)
	payload := append(fsp, fopt...)
	if childAnchor != nil {
		child := make([]byte, 16)
		binary.LittleEndian.PutUint32(child[0:4], uint32(childAnchor.Left))
		binary.LittleEndian.PutUint32(child[4:8], uint32(childAnchor.Top))
		binary.LittleEndian.PutUint32(child[8:12], uint32(childAnchor.Right))
		binary.LittleEndian.PutUint32(child[12:16], uint32(childAnchor.Bottom))
		payload = append(payload, buildLegacyOfficeArtRecord(0x0000, legacyOfficeArtChildAnchorRecord, child)...)
	}
	return payload
}

func legacyShapeOfficeArtProperties(shape *wordir.Shape) []legacyOfficeArtWriteProperty {
	props := make([]legacyOfficeArtWriteProperty, 0, 16)
	if shape == nil {
		return props
	}
	if shape.RotationDeg != 0 {
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtRotationProperty, Value: uint32(int32(math.Round(shape.RotationDeg * 65536)))})
	}
	if shape.TextBox != nil {
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtTextFlowProperty, Value: legacyOfficeArtTextFlowFromTextBox(shape.TextBox)})
	}
	if value, ok := legacyOfficeArtHexColor(shape.FillColor); ok {
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtFillColorProperty, Value: value})
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtFillBoolsProperty, Value: 1 | 1<<16})
	} else {
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtFillBoolsProperty, Value: 1 << 16})
	}
	if value, ok := legacyOfficeArtHexColor(shape.StrokeColor); ok {
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtLineColorProperty, Value: value})
		if shape.StrokeWidth > 0 {
			props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtLineWidthProperty, Value: uint32(shape.StrokeWidth)})
		}
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtLineBoolsProperty, Value: 1<<3 | 1<<19})
	} else {
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtLineBoolsProperty, Value: 1 << 19})
	}
	if shape.Geometry != nil {
		for i, value := range shape.Geometry.Adjustments {
			if i >= 8 {
				break
			}
			props = append(props, legacyOfficeArtWriteProperty{ID: uint16(legacyOfficeArtAdjust1Property + i), Value: uint32(int32(math.Round(value)))})
		}
	}
	if shape.GeometryData != nil && shape.GeometryData.Type == dml.GeometryTypeCustom {
		props = append(props, legacyCustomGeometryOfficeArtProperties(shape.GeometryData)...)
	}
	return props
}

func legacyCustomGeometryOfficeArtProperties(geometry *dml.Geometry) []legacyOfficeArtWriteProperty {
	if geometry == nil || len(geometry.Paths) == 0 {
		return nil
	}
	vertices, segments := legacyOfficeArtGeometryArrays(geometry)
	if len(vertices) == 0 || len(segments) == 0 {
		return nil
	}
	return []legacyOfficeArtWriteProperty{
		{ID: legacyOfficeArtGeoLeftProperty, Value: 0},
		{ID: legacyOfficeArtGeoTopProperty, Value: 0},
		{ID: legacyOfficeArtGeoRightProperty, Value: 21600},
		{ID: legacyOfficeArtGeoBottomProperty, Value: 21600},
		{ID: legacyOfficeArtShapePathProperty, Value: 4},
		{ID: legacyOfficeArtVerticesProperty, Complex: legacyBuildOfficeArtVertexArray(vertices)},
		{ID: legacyOfficeArtSegmentInfoProperty, Complex: legacyBuildOfficeArtSegmentArray(segments)},
	}
}

type legacyOfficeArtPoint struct{ X, Y int32 }

const (
	legacyOfficeArtPathLineTo  = uint16(0)
	legacyOfficeArtPathCurveTo = uint16(1)
	legacyOfficeArtPathMoveTo  = uint16(2)
	legacyOfficeArtPathClose   = uint16(3)
	legacyOfficeArtPathEnd     = uint16(4)
)

func legacyOfficeArtPathInfo(pathType, segments uint16) uint16 {
	return (segments << 3) | (pathType & 0x7)
}

func legacyOfficeArtGeometryArrays(geometry *dml.Geometry) ([]legacyOfficeArtPoint, []uint16) {
	var vertices []legacyOfficeArtPoint
	var segments []uint16
	toPoint := func(p dml.Point) legacyOfficeArtPoint {
		return legacyOfficeArtPoint{X: int32(math.Round(p.X * 21600)), Y: int32(math.Round(p.Y * 21600))}
	}
	for _, path := range geometry.Paths {
		var current dml.Point
		haveCurrent := false
		for _, command := range path.Commands {
			switch strings.ToUpper(command.Type) {
			case "M":
				if len(command.Points) >= 1 {
					current = command.Points[0]
					haveCurrent = true
					vertices = append(vertices, toPoint(current))
					segments = append(segments, legacyOfficeArtPathInfo(legacyOfficeArtPathMoveTo, 0))
				}
			case "L":
				for _, point := range command.Points {
					vertices = append(vertices, toPoint(point))
					segments = append(segments, legacyOfficeArtPathInfo(legacyOfficeArtPathLineTo, 1))
					current, haveCurrent = point, true
				}
			case "C":
				if len(command.Points) >= 3 {
					vertices = append(vertices, toPoint(command.Points[0]), toPoint(command.Points[1]), toPoint(command.Points[2]))
					segments = append(segments, legacyOfficeArtPathInfo(legacyOfficeArtPathCurveTo, 1))
					current, haveCurrent = command.Points[2], true
				}
			case "Q":
				if haveCurrent && len(command.Points) >= 2 {
					control, end := command.Points[0], command.Points[1]
					c1 := dml.Point{X: current.X + (control.X-current.X)*2/3, Y: current.Y + (control.Y-current.Y)*2/3}
					c2 := dml.Point{X: end.X + (control.X-end.X)*2/3, Y: end.Y + (control.Y-end.Y)*2/3}
					vertices = append(vertices, toPoint(c1), toPoint(c2), toPoint(end))
					segments = append(segments, legacyOfficeArtPathInfo(legacyOfficeArtPathCurveTo, 1))
					current = end
				}
			case "A":
				if len(command.Points) >= 1 {
					// OfficeArt custom geometry can represent arcs through escape segments,
					// but cubic approximation is substantially more interoperable across
					// Word 97-2003 versions. Preserve the endpoint and tangent continuity.
					end := command.Points[len(command.Points)-1]
					if haveCurrent {
						c1 := dml.Point{X: current.X + (end.X-current.X)/3, Y: current.Y + (end.Y-current.Y)/3}
						c2 := dml.Point{X: current.X + 2*(end.X-current.X)/3, Y: current.Y + 2*(end.Y-current.Y)/3}
						vertices = append(vertices, toPoint(c1), toPoint(c2), toPoint(end))
						segments = append(segments, legacyOfficeArtPathInfo(legacyOfficeArtPathCurveTo, 1))
					} else {
						vertices = append(vertices, toPoint(end))
						segments = append(segments, legacyOfficeArtPathInfo(legacyOfficeArtPathMoveTo, 0))
					}
					current, haveCurrent = end, true
				}
			case "Z":
				segments = append(segments, legacyOfficeArtPathInfo(legacyOfficeArtPathClose, 1))
			}
		}
		segments = append(segments, legacyOfficeArtPathInfo(legacyOfficeArtPathEnd, 0))
	}
	return vertices, segments
}

func legacyBuildOfficeArtVertexArray(vertices []legacyOfficeArtPoint) []byte {
	out := make([]byte, 6+len(vertices)*8)
	binary.LittleEndian.PutUint16(out[0:2], uint16(len(vertices)))
	binary.LittleEndian.PutUint16(out[2:4], uint16(len(vertices)))
	binary.LittleEndian.PutUint16(out[4:6], 8)
	for i, point := range vertices {
		off := 6 + i*8
		binary.LittleEndian.PutUint32(out[off:off+4], uint32(point.X))
		binary.LittleEndian.PutUint32(out[off+4:off+8], uint32(point.Y))
	}
	return out
}

func legacyBuildOfficeArtSegmentArray(segments []uint16) []byte {
	out := make([]byte, 6+len(segments)*2)
	binary.LittleEndian.PutUint16(out[0:2], uint16(len(segments)))
	binary.LittleEndian.PutUint16(out[2:4], uint16(len(segments)))
	binary.LittleEndian.PutUint16(out[4:6], 2)
	for i, segment := range segments {
		binary.LittleEndian.PutUint16(out[6+i*2:8+i*2], segment)
	}
	return out
}

func buildLegacyOfficeArtFOPT(props []legacyOfficeArtWriteProperty) []byte {
	if len(props) == 0 {
		return nil
	}
	sort.SliceStable(props, func(i, j int) bool { return props[i].ID < props[j].ID })
	payload := make([]byte, len(props)*6)
	var complex bytes.Buffer
	for i, property := range props {
		off := i * 6
		opid := property.ID
		value := property.Value
		if len(property.Complex) > 0 {
			opid |= 0x8000
			value = uint32(len(property.Complex))
			complex.Write(property.Complex)
		}
		binary.LittleEndian.PutUint16(payload[off:off+2], opid)
		binary.LittleEndian.PutUint32(payload[off+2:off+6], value)
	}
	payload = append(payload, complex.Bytes()...)
	return buildLegacyOfficeArtRecord((uint16(len(props))<<4)|0x3, legacyOfficeArtFOPTRecord, payload)
}

func legacyGeometryToOfficeArtShapeType(shape *wordir.Shape) uint16 {
	name := strings.TrimSpace(shape.Kind)
	if shape.Geometry != nil && strings.TrimSpace(shape.Geometry.Name) != "" {
		name = strings.TrimSpace(shape.Geometry.Name)
	}
	for candidate := uint16(1); candidate < 256; candidate++ {
		if legacyOfficeArtShapeTypeToGeometry(candidate) == name {
			return candidate
		}
	}
	return 1
}

func legacyOfficeArtHexColor(value string) (uint32, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return 0, false
	}
	n, err := strconv.ParseUint(value, 16, 24)
	if err != nil {
		return 0, false
	}
	return uint32((n>>16)&0xff) | uint32(n&0xff00) | uint32((n&0xff)<<16), true
}

func cloneLegacyWriteShape(in *wordir.Shape) *wordir.Shape {
	if in == nil {
		return nil
	}
	out := *in
	if in.Geometry != nil {
		geometry := *in.Geometry
		out.Geometry = &geometry
	}
	if in.WrapPolygon != nil {
		out.WrapPolygon = append([][2]float64(nil), in.WrapPolygon...)
	}
	if in.Children != nil {
		out.Children = append([]wordir.Shape(nil), in.Children...)
	}
	if in.TextBox != nil {
		tb := *in.TextBox
		tb.Blocks = append([]wordir.Block(nil), in.TextBox.Blocks...)
		out.TextBox = &tb
	}
	return &out
}

func legacyOfficeArtTextFlowFromTextBox(tb *wordir.TextBox) uint32 {
	if tb == nil {
		return 0
	}
	switch strings.ToLower(strings.TrimSpace(tb.Vert)) {
	case "vert270", "wordartvert", "eavert":
		return 2
	case "vert", "mongolianvert", "wordartvertrtl":
		return 5
	default:
		return 0
	}
}

func legacyAnchorRelativeBits(value string, vertical bool) uint16 {
	// MS-DOC Spa.bx/by use the same numeric values but give value 2 a
	// direction-specific meaning: column horizontally, paragraph vertically.
	switch value {
	case "margin", "leftMargin", "topMargin", "insideMargin":
		return 0
	case "page":
		return 1
	case "paragraph", "line":
		if vertical {
			return 2
		}
	case "column":
		if !vertical {
			return 2
		}
	}
	return 1 // page is the safest portable fallback for authored absolute coords
}

func buildLegacyFloatingImageOfficeArt(images []legacyFloatingImageWrite) []byte {
	if len(images) == 0 {
		return nil
	}
	var out bytes.Buffer
	for _, item := range images {
		if item.Image == nil || len(item.Image.Data) == 0 {
			continue
		}
		recType := legacyOfficeArtImageRecordType(item.Image)
		if recType == 0 {
			continue
		}
		out.Write(buildLegacyOfficeArtRecord(0, recType, item.Image.Data))
		out.Write(buildLegacyPictureSpContainer(item.Shape, item.Image))
	}
	return out.Bytes()
}

func buildLegacyPictureSpContainer(spid uint32, img *wordir.Image) []byte {
	if img == nil {
		return nil
	}
	var flags uint32
	if img.FlipV {
		flags |= 0x40
	}
	if img.FlipH {
		flags |= 0x80
	}
	fspPayload := make([]byte, 8)
	binary.LittleEndian.PutUint32(fspPayload[0:4], spid)
	binary.LittleEndian.PutUint32(fspPayload[4:8], flags)
	// 75 is the OfficeArt picture-frame shape type.
	fsp := buildLegacyOfficeArtRecord((75<<4)|0x2, legacyOfficeArtFSPRecord, fspPayload)

	props := make([]legacyOfficeArtWriteProperty, 0, 5)
	if img.RotationDeg != 0 {
		props = append(props, legacyOfficeArtWriteProperty{
			ID:    legacyOfficeArtRotationProperty,
			Value: uint32(int32(math.Round(img.RotationDeg * 65536))),
		})
	}
	if img.CropTop != 0 {
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtCropFromTopProperty, Value: legacyFractionToFOPT(img.CropTop)})
	}
	if img.CropBottom != 0 {
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtCropFromBottomProperty, Value: legacyFractionToFOPT(img.CropBottom)})
	}
	if img.CropLeft != 0 {
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtCropFromLeftProperty, Value: legacyFractionToFOPT(img.CropLeft)})
	}
	if img.CropRight != 0 {
		props = append(props, legacyOfficeArtWriteProperty{ID: legacyOfficeArtCropFromRightProperty, Value: legacyFractionToFOPT(img.CropRight)})
	}
	var fopt []byte
	if len(props) > 0 {
		payload := make([]byte, len(props)*6)
		for i, property := range props {
			off := i * 6
			binary.LittleEndian.PutUint16(payload[off:off+2], property.ID)
			binary.LittleEndian.PutUint32(payload[off+2:off+6], property.Value)
		}
		fopt = buildLegacyOfficeArtRecord((uint16(len(props))<<4)|0x3, legacyOfficeArtFOPTRecord, payload)
	}
	payload := append(fsp, fopt...)
	return buildLegacyOfficeArtRecord(0x000F, legacyOfficeArtSpContainer, payload)
}

type legacyOfficeArtWriteProperty struct {
	ID      uint16
	Value   uint32
	Complex []byte
}

func buildLegacyOfficeArtRecord(verInstance, recType uint16, payload []byte) []byte {
	record := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint16(record[0:2], verInstance)
	binary.LittleEndian.PutUint16(record[2:4], recType)
	binary.LittleEndian.PutUint32(record[4:8], uint32(len(payload)))
	copy(record[8:], payload)
	return record
}

func legacyOfficeArtImageRecordType(img *wordir.Image) uint16 {
	if img == nil {
		return 0
	}
	mime := strings.ToLower(strings.TrimSpace(img.MIMEType))
	if mime == "" {
		mime = detectLegacyImageMIME(img.Data)
	}
	switch mime {
	case "image/png":
		return legacyOfficeArtBlipPNG
	case "image/jpeg", "image/jpg":
		return legacyOfficeArtBlipJPEG
	case "image/bmp":
		return legacyOfficeArtBlipDIB
	default:
		return 0
	}
}

func legacyFractionToFOPT(value float64) uint32 {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return uint32(int32(math.Round(value * 65536)))
}

func legacyEMUToTwips(value int64) int64 {
	// 1 twip = 635 EMU.
	if value >= 0 {
		return (value + 317) / 635
	}
	return -((-value + 317) / 635)
}

func maxLegacyInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
