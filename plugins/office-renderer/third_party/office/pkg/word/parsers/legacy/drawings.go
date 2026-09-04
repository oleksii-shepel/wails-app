package legacy

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"math"

	officeartir "office/pkg/officeart/ir"
	officeartlegacy "office/pkg/officeart/legacy"
	officeartescher "office/pkg/officeart/legacy/escher"
	dml "office/pkg/ooxml/drawing"
	wordir "office/pkg/word/ir"
)

const (
	legacyOfficeArtSpgrContainer       = 0xF003
	legacyOfficeArtSpContainer         = 0xF004
	legacyOfficeArtFSPGRRecord         = 0xF009
	legacyOfficeArtFSPRecord           = 0xF00A
	legacyOfficeArtFOPTRecord          = 0xF00B
	legacyOfficeArtSecondaryFOPT       = 0xF121
	legacyOfficeArtTertiaryFOPT        = 0xF122
	legacyOfficeArtTextLeftProperty    = 0x0081
	legacyOfficeArtTextTopProperty     = 0x0082
	legacyOfficeArtTextRightProperty   = 0x0083
	legacyOfficeArtTextBottomProperty  = 0x0084
	legacyOfficeArtWrapTextProperty    = 0x0085
	legacyOfficeArtAnchorTextProperty  = 0x0087
	legacyOfficeArtTextFlowProperty    = 0x0088
	legacyOfficeArtChildAnchorRecord   = 0xF00F
	legacyOfficeArtClientTextboxRecord = 0xF00D

	// MS-ODRAW property IDs used to enrich legacy drawing IR.
	legacyOfficeArtRotationProperty          = 0x0004
	legacyOfficeArtFillColorProperty         = 0x0181
	legacyOfficeArtFillBoolsProperty         = 0x01BF
	legacyOfficeArtLineColorProperty         = 0x01C0
	legacyOfficeArtLineWidthProperty         = 0x01CB
	legacyOfficeArtLineBoolsProperty         = 0x01FF
	legacyOfficeArtCropFromTopProperty       = 0x0100
	legacyOfficeArtCropFromBottomProperty    = 0x0101
	legacyOfficeArtCropFromLeftProperty      = 0x0102
	legacyOfficeArtCropFromRightProperty     = 0x0103
	legacyOfficeArtPictureContrastProperty   = 0x0108
	legacyOfficeArtPictureBrightnessProperty = 0x0109
	legacyOfficeArtPictureGrayProperty       = 0x013D
	legacyOfficeArtPictureBiLevelProperty    = 0x013E
	legacyOfficeArtPictureBoolsProperty      = 0x013F
	legacyOfficeArtGeoLeftProperty           = 0x0140
	legacyOfficeArtGeoTopProperty            = 0x0141
	legacyOfficeArtGeoRightProperty          = 0x0142
	legacyOfficeArtGeoBottomProperty         = 0x0143
	legacyOfficeArtShapePathProperty         = 0x0144
	legacyOfficeArtVerticesProperty          = 0x0145
	legacyOfficeArtSegmentInfoProperty       = 0x0146
	legacyOfficeArtAdjust1Property           = 0x0147
	legacyOfficeArtAdjust2Property           = 0x0148
	legacyOfficeArtAdjust3Property           = 0x0149
	legacyOfficeArtAdjust4Property           = 0x014A
	legacyOfficeArtAdjust5Property           = 0x014B
	legacyOfficeArtAdjust6Property           = 0x014C
	legacyOfficeArtAdjust7Property           = 0x014D
	legacyOfficeArtAdjust8Property           = 0x014E

	legacyOfficeArtBlipEMF  = 0xF01A
	legacyOfficeArtBlipWMF  = 0xF01B
	legacyOfficeArtBlipPICT = 0xF01C
	legacyOfficeArtBlipJPEG = 0xF01D
	legacyOfficeArtBlipPNG  = 0xF01E
	legacyOfficeArtBlipDIB  = 0xF01F
)

// legacyOfficeArtShapeProps captures the per-shape transform/crop properties
// needed to populate wordir.Image and wordir.Shape fields beyond the FSPA
// rectangle. Empty fields are zero-valued so callers can apply only the bits
// that were actually present in FOPT/FSP records.
type legacyOfficeArtShapeProps struct {
	HasTextBox                           bool
	HasGeometryText                      bool
	GeometryText                         officeartlegacy.GeometryText
	HasTextLeft, HasTextTop              bool
	HasTextRight, HasTextBottom          bool
	TextLeft, TextTop                    float64 // points
	TextRight, TextBottom                float64 // points
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
	Rotation                             float64 // degrees, clockwise
	FlipH                                bool
	FlipV                                bool
	CropTop                              float64 // [0, 1] fraction
	CropBottom                           float64
	CropLeft                             float64
	CropRight                            float64
	Brightness                           float64 // CSS multiplier; 1 = unchanged, 0 = unset
	Contrast                             float64 // CSS multiplier; 1 = unchanged, 0 = unset
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

func extractLegacyDrawingImages(wordStream []byte) []*wordir.Image {
	if len(wordStream) < 8 {
		return nil
	}
	var images []*wordir.Image
	for off := 0; off+8 <= len(wordStream); off++ {
		recType := binary.LittleEndian.Uint16(wordStream[off+2 : off+4])
		if !legacyOfficeArtBlipRecordType(recType) {
			continue
		}
		recLen := int(binary.LittleEndian.Uint32(wordStream[off+4 : off+8]))
		if recLen <= 0 || off+8+recLen > len(wordStream) {
			continue
		}
		if img := legacyImageFromOfficeArtBlip(recType, wordStream[off+8:off+8+recLen]); img != nil {
			images = append(images, img)
		}
		off += 7 + recLen
	}
	if len(images) == 0 {
		return nil
	}
	return images
}

// extractLegacyDrawingShapePropsFromStreams merges OfficeArt shape metadata
// from all streams that may carry it in Word 97+ files. In real Word files
// picture blips are commonly stored in Data while FSP/FOPT transform metadata
// (notably rotation) is stored in the table stream. Older code inspected only
// WordDocument and therefore silently lost those transforms. Later streams
// overlay earlier ones field-by-field so table-stream FOPT values augment,
// rather than erase, data-stream picture properties.
func extractLegacyDrawingShapePropsFromStreams(streams ...[]byte) map[uint32]legacyOfficeArtShapeProps {
	var merged map[uint32]legacyOfficeArtShapeProps
	for _, stream := range streams {
		if len(stream) == 0 {
			continue
		}
		current := extractLegacyDrawingShapeProps(stream)
		if len(current) == 0 {
			continue
		}
		if merged == nil {
			merged = make(map[uint32]legacyOfficeArtShapeProps, len(current))
		}
		for id, incoming := range current {
			base := merged[id]
			merged[id] = mergeLegacyOfficeArtShapeProps(base, incoming)
		}
	}
	return merged
}

func mergeLegacyOfficeArtShapeProps(base, incoming legacyOfficeArtShapeProps) legacyOfficeArtShapeProps {
	base.HasTextBox = base.HasTextBox || incoming.HasTextBox
	if incoming.HasGeometryText {
		base.HasGeometryText = true
		base.GeometryText = incoming.GeometryText
	}
	if incoming.HasTextLeft {
		base.HasTextLeft = true
		base.TextLeft = incoming.TextLeft
	}
	if incoming.HasTextTop {
		base.HasTextTop = true
		base.TextTop = incoming.TextTop
	}
	if incoming.HasTextRight {
		base.HasTextRight = true
		base.TextRight = incoming.TextRight
	}
	if incoming.HasTextBottom {
		base.HasTextBottom = true
		base.TextBottom = incoming.TextBottom
	}
	if incoming.HasWrapText {
		base.HasWrapText = true
		base.NoWrap = incoming.NoWrap
	}
	if incoming.HasTextAnchor {
		base.HasTextAnchor = true
		base.TextAnchor = incoming.TextAnchor
		base.TextAnchorCenter = incoming.TextAnchorCenter
	}
	if incoming.HasTextVert {
		base.HasTextVert = true
		base.TextVert = incoming.TextVert
	}
	if incoming.HasTextAutoFit {
		base.HasTextAutoFit = true
		base.TextAutoFit = incoming.TextAutoFit
	}
	if incoming.ShapeType != 0 {
		base.ShapeType = incoming.ShapeType
	}
	if incoming.HasRotation {
		base.HasRotation = true
		base.Rotation = incoming.Rotation
	}
	base.FlipH = base.FlipH || incoming.FlipH
	base.FlipV = base.FlipV || incoming.FlipV
	if incoming.CropTop != 0 {
		base.CropTop = incoming.CropTop
	}
	if incoming.CropBottom != 0 {
		base.CropBottom = incoming.CropBottom
	}
	if incoming.CropLeft != 0 {
		base.CropLeft = incoming.CropLeft
	}
	if incoming.CropRight != 0 {
		base.CropRight = incoming.CropRight
	}
	if incoming.Brightness != 0 {
		base.Brightness = incoming.Brightness
	}
	if incoming.Contrast != 0 {
		base.Contrast = incoming.Contrast
	}
	base.Grayscale = base.Grayscale || incoming.Grayscale
	base.BiLevel = base.BiLevel || incoming.BiLevel
	if incoming.FillColor != "" {
		base.FillColor = incoming.FillColor
	}
	if incoming.StrokeColor != "" {
		base.StrokeColor = incoming.StrokeColor
	}
	if incoming.StrokeWidth != 0 {
		base.StrokeWidth = incoming.StrokeWidth
	}
	if incoming.GeometryData != nil {
		base.GeometryData = incoming.GeometryData
	}
	if incoming.geoLeft != 0 {
		base.geoLeft = incoming.geoLeft
	}
	if incoming.geoTop != 0 {
		base.geoTop = incoming.geoTop
	}
	if incoming.geoRight != 0 {
		base.geoRight = incoming.geoRight
	}
	if incoming.geoBottom != 0 {
		base.geoBottom = incoming.geoBottom
	}
	if len(incoming.vertices) > 0 {
		base.vertices = incoming.vertices
	}
	if len(incoming.segments) > 0 {
		base.segments = incoming.segments
	}
	for i, adjustment := range incoming.adjustments {
		if adjustment != nil {
			base.adjustments[i] = adjustment
		}
	}
	return base
}

func extractLegacyDrawingMetafileImages(stream []byte) []*wordir.Image {
	if len(stream) < 8 {
		return nil
	}
	var images []*wordir.Image
	for off := 0; off+8 <= len(stream); off++ {
		recType := binary.LittleEndian.Uint16(stream[off+2 : off+4])
		if recType != legacyOfficeArtBlipEMF && recType != legacyOfficeArtBlipWMF {
			continue
		}
		recLen := int(binary.LittleEndian.Uint32(stream[off+4 : off+8]))
		if recLen <= 0 || off+8+recLen > len(stream) {
			continue
		}
		if img := legacyImageFromOfficeArtBlip(recType, stream[off+8:off+8+recLen]); img != nil {
			images = append(images, img)
		}
		off += 7 + recLen
	}
	return images
}

func legacyOfficeArtBlipRecordType(recType uint16) bool {
	return officeartlegacy.IsBlipRecordType(recType)
}

func legacyImageFromOfficeArtBlip(recType uint16, payload []byte) *wordir.Image {
	if recType == legacyOfficeArtBlipEMF || recType == legacyOfficeArtBlipWMF {
		return legacyOfficeArtMetafileImage(recType, payload)
	}
	blob := legacyExtractRasterFromOfficeArtBlip(recType, payload)
	if len(blob) == 0 {
		return nil
	}
	mime := detectLegacyImageMIME(blob)
	if mime == "" {
		return nil
	}
	widthPx, heightPx := detectLegacyImageDimensions(blob, mime)
	copied := make([]byte, len(blob))
	copy(copied, blob)
	img := &wordir.Image{MIMEType: mime, Data: copied}
	if widthPx > 0 && heightPx > 0 {
		img.Width = legacyPixelsToEMU(widthPx)
		img.Height = legacyPixelsToEMU(heightPx)
	}
	return img
}

func legacyExtractRasterFromOfficeArtBlip(recType uint16, payload []byte) []byte {
	media := officeartlegacy.ExtractBLIP(recType, payload)
	if media.MIME == "image/png" || media.MIME == "image/jpeg" || media.MIME == "image/gif" || media.MIME == "image/bmp" {
		return media.Data
	}
	return nil
}

// extractLegacyDrawingShapeProps walks every OfficeArt SpContainer in the
// stream, building a per-spid map of shape-level properties (rotation, flip,
// crop). Returns nil when no SpContainer carries any property the caller can
// use; callers should treat a missing entry as "use IR defaults."
func extractLegacyDrawingShapeProps(wordStream []byte) map[uint32]legacyOfficeArtShapeProps {
	if len(wordStream) < officeartescher.HeaderSize {
		return nil
	}
	out := map[uint32]legacyOfficeArtShapeProps{}
	records := officeartescher.Scan(wordStream, func(h officeartescher.Header) bool {
		return h.Type == officeartescher.RecordSpContainer && h.IsContainer()
	})
	var nodes []*officeartescher.ShapeNode
	for _, record := range records {
		node, err := officeartescher.ShapeNodeFromRecord(record)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}
	index := officeartlegacy.IndexShapes(nodes)
	for _, node := range index {
		legacyCollectOfficeArtShapeNodeProps(node, out)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// legacyCollectOfficeArtShapeProps is kept as a compatibility bridge for
// inline-picture scanning and other Word-specific callers that already hold a
// complete SpContainer byte slice. Structural parsing is still delegated to
// the shared Escher frontend.
func legacyCollectOfficeArtShapeProps(record []byte, out map[uint32]legacyOfficeArtShapeProps) {
	if len(record) < officeartescher.HeaderSize || out == nil {
		return
	}
	records, err := officeartescher.ParseRecords(record)
	if err != nil {
		return
	}
	nodes, err := officeartescher.ShapeTree(records)
	if err != nil {
		return
	}
	for _, node := range nodes {
		legacyCollectOfficeArtShapeNodeProps(node, out)
	}
}

func legacyCollectOfficeArtShapeNodeProps(node *officeartescher.ShapeNode, out map[uint32]legacyOfficeArtShapeProps) {
	if node == nil {
		return
	}
	entry := legacyOfficeArtShapeProps{
		HasTextBox: node.HasClientTextbox,
		ShapeType:  node.ShapeType,
		FlipV:      node.Flags&0x40 != 0,
		FlipH:      node.Flags&0x80 != 0,
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
		if visual.HasLineColor && visual.LineColor != "" {
			entry.StrokeColor = visual.LineColor
		}
		if visual.HasLineWidth {
			entry.StrokeWidth = visual.LineWidthEMU
		}
	}
	if visual.HasCrop {
		entry.CropTop = visual.Crop.Top
		entry.CropBottom = visual.Crop.Bottom
		entry.CropLeft = visual.Crop.Left
		entry.CropRight = visual.Crop.Right
	}

	for _, prop := range props {
		baseID := prop.ID
		raw := prop.Value
		if prop.IsComplex {
			switch baseID {
			case legacyOfficeArtVerticesProperty:
				entry.vertices = legacyParseOfficeArtVertexArray(prop.Complex)
			case legacyOfficeArtSegmentInfoProperty:
				entry.segments = legacyParseOfficeArtSegmentArray(prop.Complex)
			}
			continue
		}
		if prop.IsBlip {
			continue
		}
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
			if useBits&0x0004 != 0 {
				entry.Grayscale = valueBits&0x0004 != 0
			}
			if useBits&0x0002 != 0 {
				entry.BiLevel = valueBits&0x0002 != 0
			}
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

// legacyApplyOfficeArtPropertyTable walks an FOPT property table (the bytes
// following the record header) and merges any rotation/crop properties into
// the in-progress shape properties.
func legacyApplyOfficeArtPropertyTable(payload []byte, propertyCount int, entry *legacyOfficeArtShapeProps) {
	if propertyCount <= 0 || entry == nil {
		return
	}
	props, err := officeartescher.ParseFOPT(payload, propertyCount)
	if err != nil {
		return
	}
	legacyApplyOfficeArtProperties(props, entry)
}

func legacyParseOfficeArtVertexArray(blob []byte) []legacyOfficeArtPoint {
	if len(blob) < 6 {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(blob[0:2]))
	cbElem := int(binary.LittleEndian.Uint16(blob[4:6]))
	if count <= 0 || cbElem <= 0 || 6+count*cbElem > len(blob) {
		return nil
	}
	out := make([]legacyOfficeArtPoint, 0, count)
	for i := 0; i < count; i++ {
		off := 6 + i*cbElem
		switch {
		case cbElem >= 8:
			out = append(out, legacyOfficeArtPoint{X: int32(binary.LittleEndian.Uint32(blob[off : off+4])), Y: int32(binary.LittleEndian.Uint32(blob[off+4 : off+8]))})
		case cbElem >= 4:
			out = append(out, legacyOfficeArtPoint{X: int32(int16(binary.LittleEndian.Uint16(blob[off : off+2]))), Y: int32(int16(binary.LittleEndian.Uint16(blob[off+2 : off+4])))})
		default:
			return nil
		}
	}
	return out
}

func legacyParseOfficeArtSegmentArray(blob []byte) []uint16 {
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

func legacyOfficeArtGeometryFromProps(entry *legacyOfficeArtShapeProps) *dml.Geometry {
	if entry == nil || len(entry.vertices) == 0 || len(entry.segments) == 0 {
		return nil
	}
	left, top := float64(entry.geoLeft), float64(entry.geoTop)
	width := float64(entry.geoRight - entry.geoLeft)
	height := float64(entry.geoBottom - entry.geoTop)
	if width == 0 {
		width = 21600
	}
	if height == 0 {
		height = 21600
	}
	point := func(v legacyOfficeArtPoint) dml.Point {
		return dml.Point{X: (float64(v.X) - left) / width, Y: (float64(v.Y) - top) / height}
	}
	var paths []dml.Path
	var commands []dml.PathCommand
	vertexIndex := 0
	flushPath := func() {
		if len(commands) == 0 {
			return
		}
		paths = append(paths, dml.Path{Commands: commands})
		commands = nil
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
	if len(paths) == 0 {
		return nil
	}
	return &dml.Geometry{Type: dml.GeometryTypeCustom, Paths: paths}
}

func legacyOfficeArtColorRefToHex(raw uint32) string {
	r := byte(raw)
	g := byte(raw >> 8)
	b := byte(raw >> 16)
	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}

// legacyFopt32ToFraction interprets an OfficeArt 32-bit fixed-point crop
// value as a [0, 1] fraction. The MS-ODRAW format stores crop values in
// FixedPoint (16.16) where 65536 == 100% of the source dimension.
func legacyFopt32ToFraction(raw uint32) float64 {
	v := float64(int32(raw)) / 65536.0
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return v
}

type legacyOfficeArtShapeNode struct {
	ShapeID  uint32
	Bounds   *legacyOfficeArtRect
	Children []legacyOfficeArtShapeNode
}

func extractLegacyDrawingShapeTrees(wordStream []byte) map[uint32]legacyOfficeArtShapeNode {
	if len(wordStream) < 8 {
		return nil
	}
	out := map[uint32]legacyOfficeArtShapeNode{}
	for off := 0; off+8 <= len(wordStream); off++ {
		recVer, _, recType, recLen, ok := legacyOfficeArtRecordHeader(wordStream[off:])
		if !ok || recVer != 0xF || recType != legacyOfficeArtSpgrContainer || recLen <= 0 || off+8+recLen > len(wordStream) {
			continue
		}
		if node, ok := legacyParseOfficeArtShapeNode(wordStream[off : off+8+recLen]); ok && node.ShapeID != 0 {
			out[node.ShapeID] = node
		}
		off += 7 + recLen
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func extractLegacyDrawingShapeTreesFromStreams(streams ...[]byte) map[uint32]legacyOfficeArtShapeNode {
	var merged map[uint32]legacyOfficeArtShapeNode
	for _, stream := range streams {
		for shapeID, tree := range extractLegacyDrawingShapeTrees(stream) {
			if merged == nil {
				merged = make(map[uint32]legacyOfficeArtShapeNode)
			}
			merged[shapeID] = tree
		}
	}
	return merged
}

func legacyParseOfficeArtShapeNode(record []byte) (legacyOfficeArtShapeNode, bool) {
	recVer, _, recType, recLen, ok := legacyOfficeArtRecordHeader(record)
	if !ok || recLen < 0 || 8+recLen > len(record) {
		return legacyOfficeArtShapeNode{}, false
	}
	payload := record[8 : 8+recLen]
	switch recType {
	case legacyOfficeArtSpContainer:
		var node legacyOfficeArtShapeNode
		for off := 0; off+8 <= len(payload); {
			ver, _, typ, length, valid := legacyOfficeArtRecordHeader(payload[off:])
			if !valid || length < 0 || off+8+length > len(payload) {
				break
			}
			body := payload[off+8 : off+8+length]
			switch typ {
			case legacyOfficeArtFSPRecord:
				if ver == 0x2 && len(body) >= 4 {
					node.ShapeID = binary.LittleEndian.Uint32(body[:4])
				}
			case legacyOfficeArtChildAnchorRecord:
				if len(body) >= 16 {
					node.Bounds = &legacyOfficeArtRect{
						Left: int32(binary.LittleEndian.Uint32(body[0:4])), Top: int32(binary.LittleEndian.Uint32(body[4:8])),
						Right: int32(binary.LittleEndian.Uint32(body[8:12])), Bottom: int32(binary.LittleEndian.Uint32(body[12:16])),
					}
				}
			}
			off += 8 + length
		}
		return node, node.ShapeID != 0
	case legacyOfficeArtSpgrContainer:
		if recVer != 0xF {
			return legacyOfficeArtShapeNode{}, false
		}
		var root legacyOfficeArtShapeNode
		for off := 0; off+8 <= len(payload); {
			_, _, typ, length, valid := legacyOfficeArtRecordHeader(payload[off:])
			if !valid || length < 0 || off+8+length > len(payload) {
				break
			}
			if typ == legacyOfficeArtSpContainer || typ == legacyOfficeArtSpgrContainer {
				child, childOK := legacyParseOfficeArtShapeNode(payload[off : off+8+length])
				if childOK {
					if root.ShapeID == 0 {
						root = child
					} else {
						root.Children = append(root.Children, child)
					}
				}
			}
			off += 8 + length
		}
		return root, root.ShapeID != 0
	}
	return legacyOfficeArtShapeNode{}, false
}

func extractLegacyDrawingTextFlows(wordStream []byte) map[uint32]string {
	if len(wordStream) < 8 {
		return nil
	}
	textFlows := map[uint32]string{}
	for off := 0; off+8 <= len(wordStream); off++ {
		recVer, _, recType, recLen, ok := legacyOfficeArtRecordHeader(wordStream[off:])
		if !ok || recType != legacyOfficeArtSpContainer || recVer != 0xF {
			continue
		}
		if recLen <= 0 || off+8+recLen > len(wordStream) {
			continue
		}
		legacyCollectOfficeArtTextFlows(wordStream[off:off+8+recLen], textFlows)
		off += 7 + recLen
	}
	if len(textFlows) == 0 {
		return nil
	}
	return textFlows
}

func extractLegacyDrawingTextFlowsFromStreams(streams ...[]byte) map[uint32]string {
	var merged map[uint32]string
	for _, stream := range streams {
		for shapeID, textFlow := range extractLegacyDrawingTextFlows(stream) {
			if merged == nil {
				merged = make(map[uint32]string)
			}
			merged[shapeID] = textFlow
		}
	}
	return merged
}

func legacyCollectOfficeArtTextFlows(record []byte, textFlows map[uint32]string) {
	if len(record) < 8 {
		return
	}
	payload := record[8:]
	var spid uint32
	var vert string
	for off := 0; off+8 <= len(payload); {
		recVer, recInstance, recType, recLen, ok := legacyOfficeArtRecordHeader(payload[off:])
		if !ok || recLen < 0 || off+8+recLen > len(payload) {
			break
		}
		recordBytes := payload[off : off+8+recLen]
		recordPayload := recordBytes[8:]
		switch recType {
		case legacyOfficeArtSpContainer:
			if recVer == 0xF {
				legacyCollectOfficeArtTextFlows(recordBytes, textFlows)
			}
		case legacyOfficeArtFSPRecord:
			if recVer == 0x2 && len(recordPayload) >= 4 {
				spid = binary.LittleEndian.Uint32(recordPayload[:4])
			}
		case legacyOfficeArtFOPTRecord, legacyOfficeArtSecondaryFOPT, legacyOfficeArtTertiaryFOPT:
			if recVer == 0x3 {
				if candidate := legacyTextBoxVertFromOfficeArtPropertyTable(recordPayload, int(recInstance)); candidate != "" {
					vert = candidate
				}
			}
		}
		off += 8 + recLen
	}
	if spid != 0 && vert != "" {
		textFlows[spid] = vert
	}
}

func legacyTextBoxVertFromOfficeArtPropertyTable(payload []byte, propertyCount int) string {
	if propertyCount <= 0 || propertyCount*6 > len(payload) {
		return ""
	}
	vert := ""
	for i := 0; i < propertyCount; i++ {
		off := i * 6
		opid := binary.LittleEndian.Uint16(payload[off : off+2])
		if opid&0x3FFF != legacyOfficeArtTextFlowProperty || opid&0x8000 != 0 {
			continue
		}
		if candidate := legacyTextBoxVertFromOfficeArtTextFlow(binary.LittleEndian.Uint32(payload[off+2 : off+6])); candidate != "" {
			vert = candidate
		}
	}
	return vert
}

func legacyTextBoxVertFromOfficeArtTextFlow(value uint32) string {
	return string(officeartlegacy.TextVerticalFromFlow(value))
}

func legacyOfficeArtRecordHeader(data []byte) (recVer uint16, recInstance uint16, recType uint16, recLen int, ok bool) {
	if len(data) < 8 {
		return 0, 0, 0, 0, false
	}
	verInstance := binary.LittleEndian.Uint16(data[:2])
	recType = binary.LittleEndian.Uint16(data[2:4])
	recLen64 := binary.LittleEndian.Uint32(data[4:8])
	if recLen64 > uint32(len(data)-8) {
		return 0, 0, 0, 0, false
	}
	return verInstance & 0x000F, verInstance >> 4, recType, int(recLen64), true
}

func legacyRasterEnd(data []byte, mime string) (int, bool) {
	switch mime {
	case "image/png":
		return scanPNGEnd(data, 0)
	case "image/jpeg":
		return scanJPEGEnd(data, 0)
	case "image/gif":
		return scanGIFEnd(data, 0)
	case "image/bmp":
		if len(data) < 6 {
			return 0, false
		}
		size := int(binary.LittleEndian.Uint32(data[2:6]))
		if size <= 14 || size > len(data) {
			return 0, false
		}
		return size, true
	default:
		return 0, false
	}
}

func legacyPixelsToEMU(px int) int64 {
	return int64(float64(px) / legacyImageDefaultDPI * 914400.0)
}

func appendLegacyDetachedDrawingImages(doc *wordir.Document, images []*wordir.Image) {
	if doc == nil || len(images) == 0 {
		return
	}
	seen := existingLegacyDocImageDigests(doc)
	for _, img := range images {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		digest := sha1.Sum(img.Data)
		if _, ok := seen[digest]; ok {
			continue
		}
		seen[digest] = struct{}{}
		paragraph := wordir.Paragraph{
			Index: len(doc.Paragraphs),
			Runs:  []wordir.Run{{Kind: "image", Image: img}},
		}
		doc.Paragraphs = append(doc.Paragraphs, paragraph)
		doc.Blocks = append(doc.Blocks, wordir.Block{
			Kind:      "paragraph",
			Paragraph: &doc.Paragraphs[len(doc.Paragraphs)-1],
		})
	}
}

func existingLegacyDocImageDigests(doc *wordir.Document) map[[20]byte]struct{} {
	seen := map[[20]byte]struct{}{}
	if doc == nil {
		return seen
	}
	for _, paragraph := range doc.Paragraphs {
		for _, run := range paragraph.Runs {
			if run.Image == nil || len(run.Image.Data) == 0 {
				continue
			}
			seen[sha1.Sum(run.Image.Data)] = struct{}{}
		}
	}
	return seen
}
