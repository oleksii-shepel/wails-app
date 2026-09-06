package legacy

import (
	"encoding/binary"
	"math"

	officeartir "office/pkg/officeart/ir"
	officeartlegacy "office/pkg/officeart/legacy"
	officeartescher "office/pkg/officeart/legacy/escher"
	dml "office/pkg/ooxml/drawing"
)

// legacyOfficeArtShapeProps is retained only as a writer-test oracle while the
// writer assertions migrate to the shared OfficeArt IR. Production DOC parsing
// no longer consumes this parallel property model.
type legacyOfficeArtShapeProps struct {
	HasTextBox                           bool
	HasGeometryText                      bool
	GeometryText                         officeartlegacy.GeometryText
	HasTextLeft, HasTextTop              bool
	HasTextRight, HasTextBottom          bool
	TextLeft, TextTop                    float64
	TextRight, TextBottom                float64
	HasWrapText                          bool
	NoWrap                               bool
	HasTextAnchor                        bool
	TextAnchor                           string
	TextAnchorCenter                     bool
	HasTextVert                          bool
	TextVert                             string
	HasTextAutoFit                       bool
	TextAutoFit                          string
	ShapeType                            uint16
	HasRotation                          bool
	Rotation                             float64
	FlipH                                bool
	FlipV                                bool
	CropTop                              float64
	CropBottom                           float64
	CropLeft                             float64
	CropRight                            float64
	Brightness                           float64
	Contrast                             float64
	Grayscale                            bool
	BiLevel                              bool
	FillColor                            string
	StrokeColor                          string
	StrokeWidth                          int64
	GeometryData                         *dml.Geometry
	geoLeft, geoTop, geoRight, geoBottom int32
	vertices                             []legacyOfficeArtPoint
	segments                             []uint16
	adjustments                          [8]*int32
}

func extractLegacyDrawingShapeProps(stream []byte) map[uint32]legacyOfficeArtShapeProps {
	if len(stream) < officeartescher.HeaderSize {
		return nil
	}
	out := map[uint32]legacyOfficeArtShapeProps{}
	records := officeartescher.Scan(stream, func(h officeartescher.Header) bool {
		return h.Type == officeartescher.RecordSpContainer && h.IsContainer()
	})
	var nodes []*officeartescher.ShapeNode
	for _, record := range records {
		node, err := officeartescher.ShapeNodeFromRecord(record)
		if err == nil && node != nil {
			nodes = append(nodes, node)
		}
	}
	for _, node := range officeartlegacy.IndexShapes(nodes) {
		legacyCollectOfficeArtShapeNodeProps(node, out)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeLegacyOfficeArtShapeProps(base, incoming legacyOfficeArtShapeProps) legacyOfficeArtShapeProps {
	base.HasTextBox = base.HasTextBox || incoming.HasTextBox
	if incoming.HasGeometryText {
		base.HasGeometryText = true
		base.GeometryText = incoming.GeometryText
	}
	if incoming.HasTextLeft {
		base.HasTextLeft, base.TextLeft = true, incoming.TextLeft
	}
	if incoming.HasTextTop {
		base.HasTextTop, base.TextTop = true, incoming.TextTop
	}
	if incoming.HasTextRight {
		base.HasTextRight, base.TextRight = true, incoming.TextRight
	}
	if incoming.HasTextBottom {
		base.HasTextBottom, base.TextBottom = true, incoming.TextBottom
	}
	if incoming.HasWrapText {
		base.HasWrapText, base.NoWrap = true, incoming.NoWrap
	}
	if incoming.HasTextAnchor {
		base.HasTextAnchor = true
		base.TextAnchor = incoming.TextAnchor
		base.TextAnchorCenter = incoming.TextAnchorCenter
	}
	if incoming.HasTextVert {
		base.HasTextVert, base.TextVert = true, incoming.TextVert
	}
	if incoming.HasTextAutoFit {
		base.HasTextAutoFit, base.TextAutoFit = true, incoming.TextAutoFit
	}
	if incoming.ShapeType != 0 {
		base.ShapeType = incoming.ShapeType
	}
	if incoming.HasRotation {
		base.HasRotation, base.Rotation = true, incoming.Rotation
	}
	base.FlipH = base.FlipH || incoming.FlipH
	base.FlipV = base.FlipV || incoming.FlipV
	if incoming.CropTop != 0 { base.CropTop = incoming.CropTop }
	if incoming.CropBottom != 0 { base.CropBottom = incoming.CropBottom }
	if incoming.CropLeft != 0 { base.CropLeft = incoming.CropLeft }
	if incoming.CropRight != 0 { base.CropRight = incoming.CropRight }
	if incoming.Brightness != 0 { base.Brightness = incoming.Brightness }
	if incoming.Contrast != 0 { base.Contrast = incoming.Contrast }
	base.Grayscale = base.Grayscale || incoming.Grayscale
	base.BiLevel = base.BiLevel || incoming.BiLevel
	if incoming.FillColor != "" { base.FillColor = incoming.FillColor }
	if incoming.StrokeColor != "" { base.StrokeColor = incoming.StrokeColor }
	if incoming.StrokeWidth != 0 { base.StrokeWidth = incoming.StrokeWidth }
	if incoming.GeometryData != nil { base.GeometryData = incoming.GeometryData }
	if incoming.geoLeft != 0 { base.geoLeft = incoming.geoLeft }
	if incoming.geoTop != 0 { base.geoTop = incoming.geoTop }
	if incoming.geoRight != 0 { base.geoRight = incoming.geoRight }
	if incoming.geoBottom != 0 { base.geoBottom = incoming.geoBottom }
	if len(incoming.vertices) > 0 { base.vertices = incoming.vertices }
	if len(incoming.segments) > 0 { base.segments = incoming.segments }
	for i, adjustment := range incoming.adjustments {
		if adjustment != nil { base.adjustments[i] = adjustment }
	}
	return base
}

func legacyCollectOfficeArtShapeNodeProps(node *officeartescher.ShapeNode, out map[uint32]legacyOfficeArtShapeProps) {
	if node == nil {
		return
	}
	entry := legacyOfficeArtShapeProps{
		HasTextBox: node.HasClientTextbox,
		ShapeType: node.ShapeType,
		FlipV: node.Flags&0x40 != 0,
		FlipH: node.Flags&0x80 != 0,
	}
	legacyApplyOfficeArtProperties(node.Properties, &entry)
	entry.GeometryData = legacyOfficeArtGeometryFromProps(&entry)
	if node.ShapeID != 0 {
		out[node.ShapeID] = mergeLegacyOfficeArtShapeProps(out[node.ShapeID], entry)
	}
	for _, child := range node.Children {
		legacyCollectOfficeArtShapeNodeProps(child, out)
	}
}

func legacyTextAutoFitName(value officeartir.AutoFit) string {
	switch value {
	case officeartir.AutoFitNone:
		return "none"
	case officeartir.AutoFitShape:
		return "shape"
	case officeartir.AutoFitNormal:
		return "norm"
	default:
		return ""
	}
}

func legacyApplyOfficeArtProperties(props []officeartescher.Property, entry *legacyOfficeArtShapeProps) {
	if entry == nil {
		return
	}
	visual := officeartlegacy.DecodeVisualProperties(props, nil)
	if geometryText := officeartlegacy.DecodeGeometryText(props); geometryText.Present {
		entry.HasGeometryText = true
		entry.GeometryText = geometryText
	}
	if visual.Text.HasLeft {
		entry.HasTextLeft = true
		entry.TextLeft = officeartlegacy.PointsFromEMU(visual.Text.Insets.Left)
	}
	if visual.Text.HasTop {
		entry.HasTextTop = true
		entry.TextTop = officeartlegacy.PointsFromEMU(visual.Text.Insets.Top)
	}
	if visual.Text.HasRight {
		entry.HasTextRight = true
		entry.TextRight = officeartlegacy.PointsFromEMU(visual.Text.Insets.Right)
	}
	if visual.Text.HasBottom {
		entry.HasTextBottom = true
		entry.TextBottom = officeartlegacy.PointsFromEMU(visual.Text.Insets.Bottom)
	}
	if visual.Text.HasWrap {
		entry.HasWrapText = true
		entry.NoWrap = visual.Text.Wrap == officeartir.TextWrapNone
	}
	if visual.Text.HasAnchor {
		entry.HasTextAnchor = true
		entry.TextAnchor = string(visual.Text.Anchor)
		entry.TextAnchorCenter = visual.Text.AnchorCenter
	}
	if visual.Text.HasVert {
		entry.HasTextVert = true
		entry.TextVert = string(visual.Text.Vert)
	}
	if visual.Text.HasAutoFit {
		entry.HasTextAutoFit = true
		entry.TextAutoFit = legacyTextAutoFitName(visual.Text.AutoFit)
	}
	if visual.HasRotation {
		entry.Rotation = visual.RotationDeg
		entry.HasRotation = true
	}
	if visual.HasFill && !visual.FillEnabled {
		entry.FillColor = ""
	} else if visual.HasFillColor && visual.FillColor != "" {
		entry.FillColor = visual.FillColor
	}
	if visual.HasLine && !visual.LineEnabled {
		entry.StrokeColor = ""
		entry.StrokeWidth = 0
	} else {
		if visual.HasLineColor && visual.LineColor != "" { entry.StrokeColor = visual.LineColor }
		if visual.HasLineWidth { entry.StrokeWidth = visual.LineWidthEMU }
	}
	if visual.HasCrop {
		entry.CropTop = visual.Crop.Top
		entry.CropBottom = visual.Crop.Bottom
		entry.CropLeft = visual.Crop.Left
		entry.CropRight = visual.Crop.Right
	}

	for _, prop := range props {
		baseID, raw := prop.ID, prop.Value
		if prop.IsComplex {
			switch baseID {
			case legacyOfficeArtVerticesProperty:
				entry.vertices = legacyParseOfficeArtVertexArray(prop.Complex)
			case legacyOfficeArtSegmentInfoProperty:
				entry.segments = legacyParseOfficeArtSegmentArray(prop.Complex)
			}
			continue
		}
		if prop.IsBlip { continue }
		switch baseID {
		case legacyOfficeArtPictureBrightnessProperty:
			entry.Brightness = math.Max(0, math.Min(2, 1+float64(int32(raw))/32768.0))
		case legacyOfficeArtPictureContrastProperty:
			entry.Contrast = math.Max(0, float64(int32(raw))/65536.0)
		case legacyOfficeArtPictureGrayProperty:
			entry.Grayscale = raw != 0
		case legacyOfficeArtPictureBiLevelProperty:
			entry.BiLevel = raw != 0
		case legacyOfficeArtPictureBoolsProperty:
			valueBits := raw & 0xFFFF
			useBits := (raw >> 16) & 0xFFFF
			if useBits&0x0004 != 0 { entry.Grayscale = valueBits&0x0004 != 0 }
			if useBits&0x0002 != 0 { entry.BiLevel = valueBits&0x0002 != 0 }
		case legacyOfficeArtAdjust1Property, legacyOfficeArtAdjust2Property, legacyOfficeArtAdjust3Property, legacyOfficeArtAdjust4Property,
			legacyOfficeArtAdjust5Property, legacyOfficeArtAdjust6Property, legacyOfficeArtAdjust7Property, legacyOfficeArtAdjust8Property:
			index := int(baseID - legacyOfficeArtAdjust1Property)
			v := int32(raw)
			entry.adjustments[index] = &v
		case legacyOfficeArtGeoLeftProperty:
			entry.geoLeft = int32(raw)
		case legacyOfficeArtGeoTopProperty:
			entry.geoTop = int32(raw)
		case legacyOfficeArtGeoRightProperty:
			entry.geoRight = int32(raw)
		case legacyOfficeArtGeoBottomProperty:
			entry.geoBottom = int32(raw)
		}
	}
}

func legacyApplyOfficeArtPropertyTable(payload []byte, propertyCount int, entry *legacyOfficeArtShapeProps) {
	if propertyCount <= 0 || entry == nil {
		return
	}
	props, err := officeartescher.ParseFOPT(payload, propertyCount)
	if err == nil {
		legacyApplyOfficeArtProperties(props, entry)
	}
}

func legacyParseOfficeArtVertexArray(blob []byte) []legacyOfficeArtPoint {
	if len(blob) < 6 { return nil }
	count := int(binary.LittleEndian.Uint16(blob[0:2]))
	cbElem := int(binary.LittleEndian.Uint16(blob[4:6]))
	if count <= 0 || cbElem <= 0 || 6+count*cbElem > len(blob) { return nil }
	out := make([]legacyOfficeArtPoint, 0, count)
	for i := 0; i < count; i++ {
		off := 6 + i*cbElem
		switch {
		case cbElem >= 8:
			out = append(out, legacyOfficeArtPoint{X: int32(binary.LittleEndian.Uint32(blob[off:off+4])), Y: int32(binary.LittleEndian.Uint32(blob[off+4:off+8]))})
		case cbElem >= 4:
			out = append(out, legacyOfficeArtPoint{X: int32(int16(binary.LittleEndian.Uint16(blob[off:off+2]))), Y: int32(int16(binary.LittleEndian.Uint16(blob[off+2:off+4])))})
		default:
			return nil
		}
	}
	return out
}

func legacyParseOfficeArtSegmentArray(blob []byte) []uint16 {
	if len(blob) < 6 { return nil }
	count := int(binary.LittleEndian.Uint16(blob[0:2]))
	cbElem := int(binary.LittleEndian.Uint16(blob[4:6]))
	if count <= 0 || cbElem < 2 || 6+count*cbElem > len(blob) { return nil }
	out := make([]uint16, 0, count)
	for i := 0; i < count; i++ {
		off := 6 + i*cbElem
		out = append(out, binary.LittleEndian.Uint16(blob[off:off+2]))
	}
	return out
}

func legacyOfficeArtGeometryFromProps(entry *legacyOfficeArtShapeProps) *dml.Geometry {
	if entry == nil || len(entry.vertices) == 0 || len(entry.segments) == 0 { return nil }
	left, top := float64(entry.geoLeft), float64(entry.geoTop)
	width := float64(entry.geoRight-entry.geoLeft)
	height := float64(entry.geoBottom-entry.geoTop)
	if width == 0 { width = 21600 }
	if height == 0 { height = 21600 }
	point := func(v legacyOfficeArtPoint) dml.Point {
		return dml.Point{X: (float64(v.X)-left)/width, Y: (float64(v.Y)-top)/height}
	}
	var paths []dml.Path
	var commands []dml.PathCommand
	vertexIndex := 0
	flushPath := func() {
		if len(commands) != 0 {
			paths = append(paths, dml.Path{Commands: commands})
			commands = nil
		}
	}
	for _, info := range entry.segments {
		pathType := info & 0x7
		segmentCount := int(info >> 3)
		switch pathType {
		case legacyOfficeArtPathMoveTo:
			if vertexIndex < len(entry.vertices) {
				commands = append(commands, dml.PathCommand{Type: "M", Points: []dml.Point{point(entry.vertices[vertexIndex])}})
				vertexIndex++
			}
		case legacyOfficeArtPathLineTo:
			for i := 0; i < segmentCount && vertexIndex < len(entry.vertices); i++ {
				commands = append(commands, dml.PathCommand{Type: "L", Points: []dml.Point{point(entry.vertices[vertexIndex])}})
				vertexIndex++
			}
		case legacyOfficeArtPathCurveTo:
			for i := 0; i < segmentCount && vertexIndex+2 < len(entry.vertices); i++ {
				commands = append(commands, dml.PathCommand{Type: "C", Points: []dml.Point{point(entry.vertices[vertexIndex]), point(entry.vertices[vertexIndex+1]), point(entry.vertices[vertexIndex+2])}})
				vertexIndex += 3
			}
		case legacyOfficeArtPathClose:
			commands = append(commands, dml.PathCommand{Type: "Z"})
		case legacyOfficeArtPathEnd:
			flushPath()
		}
	}
	flushPath()
	if len(paths) == 0 { return nil }
	return &dml.Geometry{Type: dml.GeometryTypeCustom, Paths: paths}
}
