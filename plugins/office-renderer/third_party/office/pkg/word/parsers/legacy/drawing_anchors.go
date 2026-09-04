package legacy

import (
	"encoding/binary"
	officeartir "office/pkg/officeart/ir"

	officeartlegacy "office/pkg/officeart/legacy"
	dml "office/pkg/ooxml/drawing"
	"office/pkg/ooxml/units"
	wordir "office/pkg/word/ir"
)

const legacyDrawingAnchorChar = '\b'

const (
	// FibRgFcLcb97 field indices from [MS-DOC].
	// fcPlcSpaMom/lcbPlcSpaMom is the authoritative PlcfSpa for shapes
	// anchored in the Main Document; fcPlcSpaHdr is the Header equivalent.
	legacyFibPlcSpaMom = 40
	legacyFibPlcSpaHdr = 41
)

type legacyFloatingDrawingAnchor struct {
	CP     int
	Shape  uint32
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
	Flags  uint16

	// Decoded SPA flags. Keeping these on the parsed record avoids having
	// different image/shape/textbox paths reinterpret the same bit field.
	Header     bool
	BX         uint8
	BY         uint8
	Wrap       uint8
	WrapSide   uint8
	RcaSimple  bool
	BelowText  bool
	AnchorLock bool
}

func processLegacyDrawingAnchors(runs []docRun, wordStream, tableStream []byte, fibBase, cbRgFcLcb, mainTextLen int, images []*wordir.Image, additionalOfficeArtStreams ...[]byte) []docRun {
	if len(runs) == 0 || len(tableStream) == 0 {
		return runs
	}
	observed := legacyObservedDrawingAnchorCPs(runs)
	if len(observed) == 0 {
		return runs
	}
	anchors := loadLegacyFloatingDrawingAnchors(wordStream, tableStream, fibBase, cbRgFcLcb, mainTextLen, observed)
	if len(anchors) == 0 {
		return runs
	}
	officeArtStreams := make([][]byte, 0, 2+len(additionalOfficeArtStreams))
	officeArtStreams = append(officeArtStreams, wordStream, tableStream)
	officeArtStreams = append(officeArtStreams, additionalOfficeArtStreams...)
	shapeProps := extractLegacyDrawingShapePropsFromStreams(officeArtStreams...)
	shapeTrees := extractLegacyDrawingShapeTreesFromStreams(officeArtStreams...)
	textFlows := extractLegacyDrawingTextFlowsFromStreams(officeArtStreams...)
	out := make([]docRun, 0, len(runs))
	imageIndex := 0
	reuseSingle := len(images) == 1
	for _, r := range runs {
		if r.Kind != "" || !r.Spec || !containsLegacyDrawingAnchorRune(r.Text) {
			out = append(out, r)
			continue
		}
		runes := []rune(r.Text)
		segmentStart := 0
		segmentStartCP := r.StartCP
		cp := r.StartCP
		handled := false
		for i, ch := range runes {
			if ch != legacyDrawingAnchorChar {
				cp++
				continue
			}
			anchor, ok := anchors[cp]
			if !ok {
				cp++
				continue
			}
			handled = true
			if i > segmentStart {
				out = append(out, cloneDocRunWithText(r, string(runes[segmentStart:i]), segmentStartCP))
			}
			template := legacySelectDrawingImage(images, imageIndex, reuseSingle)
			if template != nil {
				out = append(out, docRun{
					Kind:       "image",
					Image:      legacyCloneDrawingImage(template, anchor, shapeProps),
					Format:     r.Format,
					ParaFormat: r.ParaFormat,
					StartCP:    cp,
					Revision:   r.Revision,
				})
				if !reuseSingle {
					imageIndex++
				}
			} else {
				if shape := legacyCloneDrawingShapeDetailed(anchor, shapeProps, shapeTrees, textFlows); shape != nil {
					if _, hasTextBox := textFlows[anchor.Shape]; hasTextBox && shape.TextBox == nil {
						shape.TextBox = &wordir.TextBox{LegacyShapeID: anchor.Shape}
					}
					out = append(out, docRun{
						Kind:       "shape",
						Shape:      shape,
						Format:     r.Format,
						ParaFormat: r.ParaFormat,
						StartCP:    cp,
						Revision:   r.Revision,
					})
				}
			}
			cp++
			segmentStart = i + 1
			segmentStartCP = cp
		}
		if !handled {
			out = append(out, r)
			continue
		}
		if segmentStart < len(runes) {
			out = append(out, cloneDocRunWithText(r, string(runes[segmentStart:]), segmentStartCP))
		}
	}
	return out
}

func containsLegacyDrawingAnchorRune(text string) bool {
	for _, ch := range text {
		if ch == legacyDrawingAnchorChar {
			return true
		}
	}
	return false
}

func legacyObservedDrawingAnchorCPs(runs []docRun) []int {
	var cps []int
	for _, r := range runs {
		if r.Kind != "" || !r.Spec || r.Text == "" {
			continue
		}
		cp := r.StartCP
		for _, ch := range r.Text {
			if ch == legacyDrawingAnchorChar {
				cps = append(cps, cp)
			}
			cp++
		}
	}
	return cps
}

func loadLegacyFloatingDrawingAnchors(wordStream, tableStream []byte, fibBase, cbRgFcLcb, mainTextLen int, observedCPs []int) map[int]legacyFloatingDrawingAnchor {
	// Use the field Word actually defines for this structure. The old code
	// scanned every FibRgFcLcb entry looking for a byte sequence that happened
	// to parse like PlcfSpa and then picked the highest CP-overlap score. Large
	// DOC files contain many PLCFs with similar layouts, so that heuristic can
	// select an unrelated table and give otherwise-correct shapes bogus bounds.
	if fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, legacyFibPlcSpaMom); ok &&
		lcb > 0 && fc >= 0 && fc+lcb <= len(tableStream) {
		if anchors, valid := parseLegacyPlcfSpa(tableStream[fc:fc+lcb], mainTextLen); valid && len(anchors) > 0 {
			return legacyAnchorMap(anchors)
		}
	}

	// Compatibility fallback for damaged/nonconforming files. Keep this narrow
	// and score candidates only when the authoritative FIB pair is unavailable
	// or malformed.
	return loadLegacyFloatingDrawingAnchorsHeuristic(wordStream, tableStream, fibBase, cbRgFcLcb, mainTextLen, observedCPs)
}

func legacyAnchorMap(anchors []legacyFloatingDrawingAnchor) map[int]legacyFloatingDrawingAnchor {
	if len(anchors) == 0 {
		return nil
	}
	out := make(map[int]legacyFloatingDrawingAnchor, len(anchors))
	for _, anchor := range anchors {
		out[anchor.CP] = anchor
	}
	return out
}

func loadLegacyFloatingDrawingAnchorsHeuristic(wordStream, tableStream []byte, fibBase, cbRgFcLcb, mainTextLen int, observedCPs []int) map[int]legacyFloatingDrawingAnchor {
	observed := make(map[int]struct{}, len(observedCPs))
	for _, cp := range observedCPs {
		observed[cp] = struct{}{}
	}
	bestScore := 0
	var best []legacyFloatingDrawingAnchor
	for index := 0; index < cbRgFcLcb; index++ {
		if index == legacyFibPlcSpaMom {
			continue
		}
		fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, index)
		if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
			continue
		}
		anchors, ok := parseLegacyPlcfSpa(tableStream[fc:fc+lcb], mainTextLen)
		if !ok || len(anchors) == 0 {
			continue
		}
		score := scoreLegacyPlcfSpaCandidate(anchors, observed)
		if score <= bestScore {
			continue
		}
		bestScore = score
		best = anchors
	}
	return legacyAnchorMap(best)
}

func parseLegacyPlcfSpa(blob []byte, mainTextLen int) ([]legacyFloatingDrawingAnchor, bool) {
	if len(blob) < 34 || (len(blob)-4)%30 != 0 {
		return nil, false
	}
	count := (len(blob) - 4) / 30
	if count <= 0 {
		return nil, false
	}
	cpBytes := (count + 1) * 4
	if cpBytes+count*26 != len(blob) {
		return nil, false
	}
	anchors := make([]legacyFloatingDrawingAnchor, 0, count)
	prevCP := -1
	for i := 0; i < count; i++ {
		cp := int(binary.LittleEndian.Uint32(blob[i*4 : i*4+4]))
		if cp < 0 || cp > mainTextLen || cp < prevCP {
			return nil, false
		}
		prevCP = cp
		off := cpBytes + i*26
		left := int32(binary.LittleEndian.Uint32(blob[off+4 : off+8]))
		top := int32(binary.LittleEndian.Uint32(blob[off+8 : off+12]))
		right := int32(binary.LittleEndian.Uint32(blob[off+12 : off+16]))
		bottom := int32(binary.LittleEndian.Uint32(blob[off+16 : off+20]))
		flags := binary.LittleEndian.Uint16(blob[off+20 : off+22])
		anchors = append(anchors, legacyFloatingDrawingAnchor{
			CP:         cp,
			Shape:      binary.LittleEndian.Uint32(blob[off : off+4]),
			Left:       left,
			Top:        top,
			Right:      right,
			Bottom:     bottom,
			Flags:      flags,
			Header:     flags&0x0001 != 0,
			BX:         uint8((flags >> 1) & 0x03),
			BY:         uint8((flags >> 3) & 0x03),
			Wrap:       uint8((flags >> 5) & 0x0F),
			WrapSide:   uint8((flags >> 9) & 0x0F),
			RcaSimple:  flags&0x2000 != 0,
			BelowText:  flags&0x4000 != 0,
			AnchorLock: flags&0x8000 != 0,
		})
	}
	lastCP := int(binary.LittleEndian.Uint32(blob[count*4 : count*4+4]))
	if lastCP < prevCP {
		return nil, false
	}
	return anchors, true
}

func scoreLegacyPlcfSpaCandidate(anchors []legacyFloatingDrawingAnchor, observed map[int]struct{}) int {
	if len(anchors) == 0 {
		return 0
	}
	score := 0
	for _, anchor := range anchors {
		if _, ok := observed[anchor.CP]; ok {
			score += 100
		}
		if anchor.Shape != 0 {
			score += 1
		}
		if anchor.Right > anchor.Left && anchor.Bottom > anchor.Top {
			score += 2
		}
		if anchor.Left >= 0 && anchor.Top >= 0 {
			score += 1
		}
	}
	return score
}

type legacyAnchorPlacement struct {
	X, Y          int64
	Width, Height int64
	XRelative     string
	YRelative     string
	WrapType      string
	WrapSide      string
	BehindDoc     bool
}

func legacyPlacementFromAnchor(anchor legacyFloatingDrawingAnchor) (legacyAnchorPlacement, bool) {
	// FSPA rectangle coordinates are signed twips. Normalize reversed producer
	// coordinates once before mapping them into Word IR.
	rect, ok := officeartlegacy.NormalizeRect(
		int64(anchor.Left),
		int64(anchor.Top),
		int64(anchor.Right),
		int64(anchor.Bottom),
	)
	if !ok {
		return legacyAnchorPlacement{}, false
	}

	xRelative := legacyAnchorRelative(anchor, false)
	yRelative := legacyAnchorRelative(anchor, true)
	wrapType := legacyWrapTypeFromSpaFlags(anchor.Flags)
	return legacyAnchorPlacement{
		X:         units.TwipsToEMU(rect.X),
		Y:         units.TwipsToEMU(rect.Y),
		Width:     units.TwipsToEMU(rect.W),
		Height:    units.TwipsToEMU(rect.H),
		XRelative: xRelative,
		YRelative: yRelative,
		WrapType:  wrapType,
		WrapSide:  legacyWrapSideFromSpaFlags(anchor.Flags),
		BehindDoc: wrapType == "none" && anchor.BelowText,
	}, true
}

// legacyAnchorRelative implements the FSPA origin rule, including fRcaSimple.
// MS-DOC defines fRcaSimple as temporarily overriding bx/by so all four
// rectangle coordinates are page-relative.
func legacyAnchorRelative(anchor legacyFloatingDrawingAnchor, vertical bool) string {
	if anchor.RcaSimple || anchor.Flags&0x2000 != 0 {
		return "page"
	}
	if vertical {
		switch anchor.BY {
		case 0:
			return "margin"
		case 1:
			return "page"
		case 2:
			return "paragraph"
		default:
			return "page"
		}
	}
	switch anchor.BX {
	case 0:
		return "margin"
	case 1:
		return "page"
	case 2:
		return "column"
	default:
		return "page"
	}
}

func legacySelectDrawingImage(images []*wordir.Image, index int, reuseSingle bool) *wordir.Image {
	if len(images) == 0 {
		return nil
	}
	if reuseSingle {
		return images[0]
	}
	if index < 0 || index >= len(images) {
		return nil
	}
	return images[index]
}

func legacyCloneDrawingImage(template *wordir.Image, anchor legacyFloatingDrawingAnchor, shapeProps map[uint32]legacyOfficeArtShapeProps) *wordir.Image {
	if template == nil {
		return nil
	}
	placement, ok := legacyPlacementFromAnchor(anchor)
	if !ok {
		return nil
	}
	cloned := *template
	cloned.Floating = true
	cloned.XRelative = placement.XRelative
	cloned.YRelative = placement.YRelative
	cloned.X = placement.X
	cloned.Y = placement.Y
	cloned.Width = placement.Width
	cloned.Height = placement.Height
	cloned.WrapType = placement.WrapType
	cloned.WrapSide = placement.WrapSide
	cloned.BehindDoc = placement.BehindDoc
	if props, ok := shapeProps[anchor.Shape]; ok {
		if props.HasRotation {
			cloned.RotationDeg = props.Rotation
		}
		cloned.FlipH = props.FlipH
		cloned.FlipV = props.FlipV
		cloned.CropTop = props.CropTop
		cloned.CropBottom = props.CropBottom
		cloned.CropLeft = props.CropLeft
		cloned.CropRight = props.CropRight
		cloned.Brightness = props.Brightness
		cloned.Contrast = props.Contrast
		cloned.Grayscale = props.Grayscale
		cloned.BiLevel = props.BiLevel
	}
	return &cloned
}

func legacyCloneDrawingShape(anchor legacyFloatingDrawingAnchor, shapeProps map[uint32]legacyOfficeArtShapeProps) *wordir.Shape {
	return legacyCloneDrawingShapeDetailed(anchor, shapeProps, nil, nil)
}

func legacyCloneDrawingShapeDetailed(anchor legacyFloatingDrawingAnchor, shapeProps map[uint32]legacyOfficeArtShapeProps, shapeTrees map[uint32]legacyOfficeArtShapeNode, textFlows map[uint32]string) *wordir.Shape {
	placement, ok := legacyPlacementFromAnchor(anchor)
	if !ok {
		return nil
	}
	shape := legacyCloneDrawingShapeByID(anchor.Shape, shapeProps, textFlows)
	if shape == nil {
		shape = &wordir.Shape{Kind: "rect"}
	}
	shape.LegacyShapeID = anchor.Shape
	shape.Floating = true
	shape.XRelative = placement.XRelative
	shape.YRelative = placement.YRelative
	shape.X = placement.X
	shape.Y = placement.Y
	shape.Width = placement.Width
	shape.Height = placement.Height
	shape.WrapType = placement.WrapType
	shape.WrapSide = placement.WrapSide
	shape.BehindDoc = placement.BehindDoc
	if tree, ok := shapeTrees[anchor.Shape]; ok && len(tree.Children) > 0 {
		shape.Kind = "group"
		shape.Geometry = nil
		shape.GeometryData = nil
		shape.Children = legacyCloneDrawingShapeChildren(tree.Children, shapeProps, textFlows)
		legacyNormalizeDrawingGroupChildren(shape)
	}
	return shape
}

// Legacy textbox invariant:
//
//	FTXBXS.lid identifies the FSP.spid owner.
//	Geometry/position/rotation stay on Shape; the story stays in Shape.TextBox.
//	An owned story must never be rendered as a standalone document paragraph.
func legacyCloneDrawingShapeByID(shapeID uint32, shapeProps map[uint32]legacyOfficeArtShapeProps, textFlows map[uint32]string) *wordir.Shape {
	shape := &wordir.Shape{Kind: "rect", LegacyShapeID: shapeID}
	props, hasProps := shapeProps[shapeID]

	if hasProps {
		if props.GeometryData != nil {
			shape.Kind = "custom"
			shape.GeometryData = props.GeometryData
		} else if preset := legacyOfficeArtShapeTypeToGeometry(props.ShapeType); preset != "" {
			shape.Kind = preset
			geometry := &dml.PresetGeometry{Name: preset}
			for _, adjust := range props.adjustments {
				if adjust == nil {
					continue
				}
				geometry.Adjustments = append(geometry.Adjustments, float64(*adjust))
			}
			shape.Geometry = geometry
		}
		if props.HasRotation {
			shape.RotationDeg = props.Rotation
		}
		shape.FlipH = props.FlipH
		shape.FlipV = props.FlipV
		shape.FillColor = props.FillColor
		shape.StrokeColor = props.StrokeColor
		shape.StrokeWidth = props.StrokeWidth
		if props.HasGeometryText && props.GeometryText.Text != "" {
			shape.TextPath = legacyGeometryTextPath(props.GeometryText)
		}
	}

	// A ClientTextbox record is sufficient to establish that this FSP owns a
	// textbox story. Previously we only allocated TextBox when txflTextFlow was
	// present, leaving ordinary horizontal textboxes empty and causing their
	// stories to fall through to appendLegacyDetachedDrawingTextBoxes().
	vert, hasFlow := textFlows[shapeID]
	if (hasProps && legacyShapeOwnsTextBox(props)) || hasFlow {
		tb := &wordir.TextBox{LegacyShapeID: shapeID}
		if hasProps {
			applyLegacyShapeTextBoxProperties(tb, props)
		}
		if hasFlow {
			tb.Vert = vert
			tb.HasVertSetting = true
		}
		shape.TextBox = tb
	}

	return shape
}

func legacyGeometryTextPath(text officeartlegacy.GeometryText) *wordir.TextPath {
	if text.Text == "" {
		return nil
	}
	path := &wordir.TextPath{
		On:       "t",
		String:   text.Text,
		Font:     text.Font,
		Bold:     text.Bold,
		Italic:   text.Italic,
		Vertical: text.Vertical,
	}
	if text.Size > 0 {
		path.SizePt = float64(text.Size) / 65536.0
	}
	return path
}

func legacyNormalizeDrawingGroupChildren(shape *wordir.Shape) {
	if shape == nil || len(shape.Children) == 0 || shape.Width <= 0 || shape.Height <= 0 {
		return
	}

	// Legacy child anchors are frequently expressed in the group's local
	// 21600-unit coordinate system. Detect that domain from the children and
	// map it into the actual group rectangle using the shared OfficeArt helper.
	const legacyGroupDomain = int64(21600)
	children := make([]*officeartir.Shape, 0, len(shape.Children))
	for i := range shape.Children {
		child := &shape.Children[i]
		children = append(children, &officeartir.Shape{
			Transform: officeartir.Transform{Rect: officeartir.Rect{
				X: child.X, Y: child.Y, W: child.Width, H: child.Height,
			}},
		})
	}

	frame := officeartlegacy.GroupFrame{
		Domain: officeartir.Rect{X: 0, Y: 0, W: legacyGroupDomain, H: legacyGroupDomain},
		Target: officeartir.Rect{X: 0, Y: 0, W: shape.Width, H: shape.Height},
		Valid:  true,
	}
	officeartlegacy.ComposeGroupChildren(children, frame)
	for i, child := range children {
		if child == nil {
			continue
		}
		shape.Children[i].X = child.Transform.Rect.X
		shape.Children[i].Y = child.Transform.Rect.Y
		shape.Children[i].Width = child.Transform.Rect.W
		shape.Children[i].Height = child.Transform.Rect.H
	}
}

func legacyShapeOwnsTextBox(props legacyOfficeArtShapeProps) bool {
	return props.HasTextBox ||
		props.HasTextLeft || props.HasTextTop || props.HasTextRight || props.HasTextBottom ||
		props.HasWrapText || props.HasTextAnchor || props.HasTextVert || props.HasTextAutoFit
}

func applyLegacyShapeTextBoxProperties(tb *wordir.TextBox, props legacyOfficeArtShapeProps) {
	if tb == nil {
		return
	}
	if props.HasTextLeft {
		tb.PaddingLeft = props.TextLeft
		tb.HasPaddingLeft = true
	}
	if props.HasTextTop {
		tb.PaddingTop = props.TextTop
		tb.HasPaddingTop = true
	}
	if props.HasTextRight {
		tb.PaddingRight = props.TextRight
		tb.HasPaddingRight = true
	}
	if props.HasTextBottom {
		tb.PaddingBottom = props.TextBottom
		tb.HasPaddingBottom = true
	}
	if props.HasWrapText {
		tb.NoWrap = props.NoWrap
		tb.HasWrapSetting = true
	}
	if props.HasTextAnchor {
		tb.Anchor = props.TextAnchor
		tb.AnchorCenter = props.TextAnchorCenter
		tb.HasAnchorSetting = true
	}
	if props.HasTextVert {
		tb.Vert = props.TextVert
		tb.HasVertSetting = true
	}
	if props.HasTextAutoFit {
		tb.Autofit = props.TextAutoFit
	}
}

func legacyCloneDrawingShapeChildren(nodes []legacyOfficeArtShapeNode, shapeProps map[uint32]legacyOfficeArtShapeProps, textFlows map[uint32]string) []wordir.Shape {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]wordir.Shape, 0, len(nodes))
	for _, node := range nodes {
		shape := legacyCloneDrawingShapeByID(node.ShapeID, shapeProps, textFlows)
		if shape == nil {
			continue
		}
		if node.Bounds != nil {
			shape.X = units.TwipsToEMU(int64(node.Bounds.Left))
			shape.Y = units.TwipsToEMU(int64(node.Bounds.Top))
			if width := int64(node.Bounds.Right) - int64(node.Bounds.Left); width > 0 {
				shape.Width = units.TwipsToEMU(width)
			}
			if height := int64(node.Bounds.Bottom) - int64(node.Bounds.Top); height > 0 {
				shape.Height = units.TwipsToEMU(height)
			}
		}
		if len(node.Children) > 0 {
			shape.Kind = "group"
			shape.Geometry = nil
			shape.GeometryData = nil
			shape.Children = legacyCloneDrawingShapeChildren(node.Children, shapeProps, textFlows)
		}
		out = append(out, *shape)
	}
	return out
}

// legacyWrapTypeFromSpaFlags decodes the FSPA wr field (MS-DOC §2.9.245),
// bits 5–8 of the flags word, into the OOXML wrap-type vocabulary used by
// wordir.Image.WrapType. The 6 wr values are listed in the spec; values 0
// and 2 both map to "square" (the rectangular wrap variants).
func legacyWrapTypeFromSpaFlags(flags uint16) string {
	switch (flags >> 5) & 0x0F {
	case 0:
		return "square"
	case 1:
		return "topAndBottom"
	case 2:
		return "square"
	case 3:
		return "none"
	case 4:
		return "tight"
	case 5:
		return "through"
	}
	return "square"
}

// legacyWrapSideFromSpaFlags decodes the FSPA wrk field (bits 9–12) into the
// OOXML wrap-side vocabulary used by wordir.Image.WrapSide.
func legacyWrapSideFromSpaFlags(flags uint16) string {
	switch (flags >> 9) & 0x0F {
	case 0:
		return "" // "bothSides" is the default; leave empty so consumers can pick the OOXML default
	case 1:
		return "left"
	case 2:
		return "right"
	case 3:
		return "largest"
	}
	return ""
}

// legacyAnchorRelativeFromSpaFlags maps the FSPA bx/by fields (bits 1–2 and
// 3–4) onto the OOXML positionH/positionV relativeFrom vocabulary used by
// wordir.Image.XRelative / YRelative.
func legacyAnchorRelativeFromSpaFlags(flags uint16, vertical bool) string {
	anchor := legacyFloatingDrawingAnchor{
		Flags:     flags,
		BX:        uint8((flags >> 1) & 0x03),
		BY:        uint8((flags >> 3) & 0x03),
		RcaSimple: flags&0x2000 != 0,
	}
	return legacyAnchorRelative(anchor, vertical)
}
