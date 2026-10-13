package legacy

import (
	"encoding/binary"
	"fmt"
	"math"

	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

// ShapeFromEscher creates the shared, format-neutral portion of a shape.
// Application adapters still provide anchors, text story ownership and images.
func ShapeFromEscher(node *escher.ShapeNode) *ir.Shape {
	return ShapeFromEscherWithColorResolver(node, nil)
}

// ShapeFromEscherWithColorResolver preserves application theme/scheme color references
// while decoding the shared OfficeArt visual model. Binary PowerPoint commonly stores
// gradient endpoints as scheme colors, so decoding with the generic RGB resolver loses
// the authored endpoint before the presentation adapter can recover it.
func ShapeFromEscherWithColorResolver(node *escher.ShapeNode, resolve ColorResolver) *ir.Shape {
	if node == nil {
		return nil
	}

	adjustments, adjustmentPresent := DecodeGeometryAdjustmentSlots(node.Properties)
	shape := &ir.Shape{
		ID:   node.ShapeID,
		Kind: ir.ShapeAuto,
		Geometry: ir.Geometry{
			Kind:              ir.GeometryPreset,
			Preset:            ShapeTypeToPreset(node.ShapeType),
			Adjustments:       adjustments,
			AdjustmentPresent: adjustmentPresent,
		},
		Transform: ir.Transform{
			FlipH: node.Flags&0x40 != 0,
			FlipV: node.Flags&0x80 != 0,
		},
		Source: ir.SourceRef{
			Record:  escher.RecordSpContainer,
			ShapeID: node.ShapeID,
		},
	}
	if OfficeArtMirrorsFlipV(node.ShapeType) {
		shape.Transform.FlipV = !shape.Transform.FlipV
	}
	// A picture frame remains a picture semantically even when the host has not
	// supplied the BStore payload yet. DOC commonly resolves the shared BLIP
	// separately from DggInfo; losing picture ownership here makes picture-frame
	// FSPs look like empty text/rect shapes and prevents the Word adapter from
	// attaching the recovered image.
	if _, ok := PictureRef(node.Properties); ok && !node.HasClientTextbox && len(node.Children) == 0 {
		shape.Kind = ir.ShapePicture
	}

	if node.ShapeType == 19 { // mso_sptArc has special sector-box geometry semantics.
		if arc := DecodeArcGeometry(node.Properties); arc.Kind == ir.GeometryCustom {
			shape.Geometry = arc
		}
	} else if custom := DecodeCustomGeometry(node.Properties); custom.Kind == ir.GeometryCustom {
		shape.Geometry = custom
	}
	if node.HasClientTextbox {
		shape.Kind = ir.ShapeTextBox
		shape.Text = &ir.TextBody{}
		// A ClientTextbox without an explicit AutoShape preset is still a
		// rectangular text frame.  PowerPoint's PPTX conversion materializes
		// these as prstGeom="rect"; keeping an empty preset in OfficeArt IR
		// creates a format-only geometry difference with no semantic meaning.
		if shape.Geometry.Kind == ir.GeometryPreset && shape.Geometry.Preset == "" {
			shape.Geometry.Preset = "rect"
		}
	}

	contextResolve := contextualOfficeArtColorResolver(node.Properties, resolve)
	visual := DecodeVisualProperties(node.Properties, contextResolve)
	if visual.HasRotation {
		shape.Transform.RotationDeg = visual.RotationDeg
	}
	if (visual.HasFill && !visual.FillEnabled) || (node.HasClientTextbox && !visual.HasFill) {
		// OfficeArt text boxes/placeholders are transparent unless fFilled is
		// explicitly authored. A cached/default fillColor alone is not paint.
		shape.Fill.Kind = ir.FillNone
	} else if visual.HasFillType && visual.FillType == 1 && (visual.HasFillColor || visual.HasFillBackColor) {
		// msofillPattern stores two authored colors: fillColor is the pattern
		// foreground and fillBackColor is the background. Treating fillType=1
		// as an ordinary solid fill collapses the pattern to its foreground
		// (commonly black), which is especially visible on legacy PPT pulse /
		// highlight rectangles whose authored background is yellow. Preserve the
		// two-color semantic in canonical OfficeArt IR; hosts that cannot recover
		// the original pattern bitmap can still paint the correct background.
		foreground, background := visual.FillColor, visual.FillBackColor
		if foreground == "" {
			foreground = background
		}
		if background == "" {
			background = foreground
		}
		shape.Fill.Kind = ir.FillPattern
		shape.Fill.Color = background
		shape.Fill.Alpha = 1
		shape.Fill.Pattern = &ir.PatternFill{Foreground: foreground, Background: background}
	} else if visual.HasFillType && visual.FillType >= 4 && visual.FillType <= 8 && visual.HasFillColor && visual.FillColor != "" {
		gradientType := legacyGradientType(visual.FillType)
		angle := visual.FillAngleDeg
		// OfficeArt angles are clockwise from straight down in the painted
		// frame; DrawingML/CSS angles are clockwise from the positive x-axis,
		// so a legacy angle maps to a DrawingML angle as 90 + angle.
		// PowerPoint's own OfficeArt->OOXML conversion confirms the sign on the
		// 41246-2 edge rules: fillAngle -90 becomes a:lin ang="0", and the
		// deck's PDF paints those rules left-to-right with the first stop at
		// the left.
		if gradientType == "linear" {
			angle = 90 + angle
		}
		shape.Fill.Kind = ir.FillGradient
		// Shade fills may omit the back color; PowerPoint resolves that stop
		// against the page background, which renders as white here. Requiring an
		// authored back color downgraded those shapes to a flat foreground fill.
		back := visual.FillBackColor
		if !visual.HasFillBackColor || back == "" {
			back = "FFFFFF"
		}
		stops := legacyGradientShadeStops(node.Properties, contextResolve)
		if len(stops) == 0 {
			stops = legacyGradientStops(visual.FillColor, back, visual)
		}
		gradient := &ir.GradientFill{Type: gradientType, Angle: angle, Stops: stops, ScaleAngleWithAspect: visual.FillType == 7, LegacyShadeType: legacyGradientShadeType(visual.FillType)}
		if visual.HasFillToLeft || visual.HasFillToTop || visual.HasFillToRight || visual.HasFillToBottom {
			gradient.HasFillTo = true
			gradient.FillToLeft = visual.FillToLeft
			gradient.FillToTop = visual.FillToTop
			gradient.FillToRight = visual.FillToRight
			gradient.FillToBottom = visual.FillToBottom
		}
		shape.Fill.Gradient = gradient
		// A gradient fill is opaque unless the authored opacity says otherwise.
		// Leaving alpha at zero made the shape look unpainted to the
		// empty-textbox heuristic, which then dropped grouped edge rules that
		// carry a text property (41246-2 lost one of the two bars on its left
		// edge that way).
		shape.Fill.Alpha = 1
		if visual.HasFillOpacity {
			shape.Fill.Alpha = visual.FillOpacity
		}
	} else if visual.HasFillType && (visual.FillType == 2 || visual.FillType == 3) && fillBlipRef(node.Properties) != 0 {
		// msofillTextured (2) / msofillPicture (3) paint the shape with the BLIP
		// named by fillBlip. A texture repeats the image at its natural size from
		// the shape's top-left corner; a picture fill stretches it. WordArt text
		// takes this fill for the glyphs themselves (PowerPoint's conversion of
		// 41246-2 slide 16 writes the deck's crumpled-paper BLIP as an a:tile
		// blipFill on the run). The decoder cannot reach a picture store, so the
		// index travels in OfficeArt IR and the host resolves the bytes.
		shape.Fill.Kind = ir.FillPicture
		shape.Fill.Picture = &ir.PictureFill{BlipRef: fillBlipRef(node.Properties)}
		if visual.FillType == 2 {
			shape.Fill.Picture.Tile = &ir.Tile{ScaleX: 1, ScaleY: 1}
		}
		shape.Fill.Alpha = 1
		if visual.HasFillOpacity {
			shape.Fill.Alpha = visual.FillOpacity
		}
	} else if visual.HasFillColor && visual.FillColor != "" {
		shape.Fill.Kind = ir.FillSolid
		shape.Fill.Color = visual.FillColor
		shape.Fill.Alpha = 1
		if visual.HasFillOpacity {
			shape.Fill.Alpha = visual.FillOpacity
		}
	} else if !node.HasClientTextbox && !officeArtLineGeometry(node.ShapeType) && (!visual.HasFill || visual.FillEnabled) {
		// OfficeArt AutoShapes are filled by default.  The default fill color is
		// white even when neither fillColor nor fFilled is repeated in the local
		// FOPT.  Preserve that default for area geometry; line/connector families
		// are normalized to no-fill below.  This is important for tiny authored
		// primitives (for example indicator lamps) whose only local properties are
		// geometry/position and which otherwise disappear against dark artwork.
		shape.Fill.Kind = ir.FillSolid
		shape.Fill.Color = "FFFFFF"
		shape.Fill.Alpha = 1
	}
	// Line/connector geometries have no area fill. Binary OfficeArt often omits
	// an explicit fillStyleBooleanProperties record for them, while PowerPoint's
	// PPTX conversion materializes the same semantic as a:noFill. Publish the
	// semantic value here so downstream presentation IR does not depend on source
	// syntax (nil fill in PPT versus explicit noFill in DrawingML).
	if shape.Fill.Kind == "" && officeArtLineGeometry(node.ShapeType) {
		shape.Fill.Kind = ir.FillNone
	}

	if visual.HasLine && !visual.LineEnabled {
		shape.Line.Visible = false
	} else {
		if visual.HasLineColor && visual.LineColor != "" {
			shape.Line.Visible = true
			shape.Line.Color = visual.LineColor
			shape.Line.Alpha = 1
		}
		if visual.HasLineWidth {
			shape.Line.Visible = true
			shape.Line.WidthPt = float64(visual.LineWidthEMU) / 12700.0
		}
		if visual.HasLineDash {
			shape.Line.Dash = visual.LineDash
		}
		if visual.HasLineJoin {
			shape.Line.Join = visual.LineJoin
		}
		if visual.HasLineCap {
			shape.Line.Cap = visual.LineCap
		}
		if visual.HasLineOpacity {
			shape.Line.Visible = true
			shape.Line.Alpha = visual.LineOpacity
		}
		if visual.HasStartArrow {
			shape.Line.Visible = true
			width, length := string(visual.StartArrowWidth), string(visual.StartArrowLength)
			if visual.StartArrow != ArrowNone {
				if width == "" {
					width = "med"
				}
				if length == "" {
					length = "med"
				}
			}
			shape.Line.StartArrow = ir.Arrow{Type: string(visual.StartArrow), Width: width, Length: length}
		}
		if visual.HasEndArrow {
			shape.Line.Visible = true
			width, length := string(visual.EndArrowWidth), string(visual.EndArrowLength)
			if visual.EndArrow != ArrowNone {
				if width == "" {
					width = "med"
				}
				if length == "" {
					length = "med"
				}
			}
			shape.Line.EndArrow = ir.Arrow{Type: string(visual.EndArrow), Width: width, Length: length}
		}
		// OfficeArt lineJoinStyle defaults to round. Preserve the format default
		// in shared IR instead of inheriting DrawingML's generic miter fallback.
		if shape.Line.Join == "" {
			shape.Line.Join = "round"
		}
		if shape.Line.Cap == "" {
			shape.Line.Cap = "flat"
		}

		// OfficeArt's lineWidth default is 9525 EMU (0.75 pt).  Preserve that
		// default in canonical OfficeArt IR whenever a stroke is visible but no
		// explicit lineWidth property was authored.  Leaving WidthPt at zero made
		// downstream renderers invent their own generic 1 pt default, which caused
		// PPT/DOC/XLS shapes to disagree with equivalent DrawingML shapes.
		if shape.Line.Visible && shape.Line.WidthPt <= 0 {
			shape.Line.WidthPt = 0.75
		}
		// OfficeArt line color/opacity defaults are black and fully opaque. A
		// width, dash or arrow property can make a line visible without carrying
		// either property explicitly; leaving Color empty or Alpha at Go's zero
		// value made canonical PPT IR disagree with the equivalent DrawingML that
		// PowerPoint writes on save, and could make shared renderers treat a real
		// stroke as transparent. Preserve an explicitly authored zero opacity.
		if shape.Line.Visible {
			if shape.Line.Color == "" {
				shape.Line.Color = "000000"
			}
			if !visual.HasLineOpacity {
				shape.Line.Alpha = 1
			}
		}
	}

	if visual.HasShadow && !visual.ShadowEnabled {
		// fShadow is authoritative when its use bit is present. This matters when
		// primary/secondary/tertiary OfficeArt properties are merged: a local
		// explicit false must suppress an inherited/default shadow rather than
		// merely declining to add another one.
		shape.Effects.Shadow = nil
	} else if visual.HasShadow && visual.ShadowEnabled {
		dx := float64(visual.ShadowOffsetXEMU) / 12700.0
		dy := float64(visual.ShadowOffsetYEMU) / 12700.0
		distance := math.Hypot(dx, dy)
		angle := 0.0
		if distance != 0 {
			angle = math.Atan2(-dy, dx) * 180 / math.Pi
		}
		alpha := visual.ShadowOpacity
		if !visual.HasShadowOpacity {
			alpha = 1
		}
		color := visual.ShadowColor
		if color == "" {
			color = "000000"
		}
		shape.Effects.Shadow = &ir.Shadow{Color: color, Alpha: alpha, BlurRadius: math.Abs(float64(visual.ShadowSoftnessEMU)) / 12700.0, Distance: distance, AngleDeg: angle}
	}

	if visual.HasCrop || visual.HasPictureBrightness || visual.HasPictureContrast || visual.HasPictureGrayscale || visual.HasPictureBiLevel {
		if shape.Image == nil {
			shape.Image = &ir.Image{}
		}
		if visual.HasCrop {
			shape.Image.Crop = visual.Crop
		}
		if visual.HasPictureBrightness {
			shape.Image.Brightness = visual.PictureBrightness
		}
		if visual.HasPictureContrast {
			shape.Image.Contrast = visual.PictureContrast
		}
		if visual.HasPictureGrayscale {
			shape.Image.Grayscale = visual.PictureGrayscale
		}
		if visual.HasPictureBiLevel {
			shape.Image.BiLevel = visual.PictureBiLevel
		}
	}

	text := visual.Text
	geometryText := DecodeGeometryText(node.Properties)
	if shape.Text != nil || geometryText.Present || text.HasLeft || text.HasTop || text.HasRight || text.HasBottom ||
		text.HasWrap || text.HasAnchor || text.HasVert || text.HasAutoFit {
		if shape.Text == nil {
			shape.Text = &ir.TextBody{}
		}
		if text.HasLeft {
			shape.Text.Insets.Left = text.Insets.Left
		}
		if text.HasTop {
			shape.Text.Insets.Top = text.Insets.Top
		}
		if text.HasRight {
			shape.Text.Insets.Right = text.Insets.Right
		}
		if text.HasBottom {
			shape.Text.Insets.Bottom = text.Insets.Bottom
		}
		if text.HasWrap {
			shape.Text.Wrap = text.Wrap
		}
		if text.HasAnchor {
			shape.Text.Anchor = text.Anchor
			shape.Text.AnchorCenter = text.AnchorCenter
		}
		if text.HasVert {
			shape.Text.Vert = text.Vert
		}
		if text.HasAutoFit {
			shape.Text.AutoFit = text.AutoFit
		}
	}

	if geometryText.Present {
		shape.GeometryText = &ir.GeometryText{
			Text:          geometryText.Text,
			FontFamily:    geometryText.Font,
			FontSizePt:    legacyGeometryTextSizePt(geometryText.Size),
			Spacing:       float64(geometryText.Spacing) / 65536.0,
			Align:         geometryText.Align,
			Warp:          WordArtWarpName(node.ShapeType),
			ReverseRows:   geometryText.ReverseRows,
			Vertical:      geometryText.Vertical,
			Kerning:       geometryText.Kerning,
			Tight:         geometryText.Tight,
			Stretch:       geometryText.Stretch,
			ShrinkFit:     geometryText.ShrinkFit,
			BestFit:       geometryText.BestFit,
			Normalize:     geometryText.Normalize,
			DxMeasure:     geometryText.DxMeasure,
			Bold:          geometryText.Bold,
			Italic:        geometryText.Italic,
			Underline:     geometryText.Underline,
			Shadow:        geometryText.Shadow,
			SmallCaps:     geometryText.SmallCaps,
			Strikethrough: geometryText.Strikethrough,
		}
	}
	if geometryText.Present && geometryText.Text != "" {
		if shape.Text == nil {
			shape.Text = &ir.TextBody{}
		}
		if geometryText.Vertical {
			shape.Text.Vert = ir.TextWordArtVertical
		}
		shape.Text.Paragraphs = append(shape.Text.Paragraphs, ir.Paragraph{
			Align: legacyGeometryTextAlign(geometryText.Align),
			Runs: []ir.TextRun{{
				Text:       geometryText.Text,
				FontFamily: geometryText.Font,
				FontSizePt: legacyGeometryTextSizePt(geometryText.Size),
				Bold:       geometryText.Bold,
				Italic:     geometryText.Italic,
			}},
		})
	}

	for _, child := range node.Children {
		if converted := ShapeFromEscher(child); converted != nil {
			shape.Children = append(shape.Children, converted)
		}
	}
	if len(shape.Children) > 0 {
		shape.Kind = ir.ShapeGroup
	}
	return shape
}

func legacyGeometryTextSizePt(raw int32) float64 {
	if raw <= 0 {
		return 0
	}
	return float64(raw) / 65536.0
}

func legacyGeometryTextAlign(raw uint32) string {
	switch raw {
	case 1:
		return "center"
	case 2:
		return "right"
	case 3:
		return "justify"
	default:
		return "left"
	}
}

// legacyGradientType preserves the semantic distinction in MSOFILLTYPE.
// In particular, msofillShadeScale (7) is still a directional shade: it is
// msofillShade with fillAngle scaled by the shape aspect ratio. Treating it as
// radial discards the authored direction and was the main reason old PPT
// gradients rendered as centered highlights. msofillShadeShape (6) is a path
// gradient because its outer boundary is the actual shape outline.
func legacyGradientShadeType(fillType uint32) string {
	switch fillType {
	case 4:
		return "shade"
	case 5:
		return "shadeCenter"
	case 6:
		return "shadeShape"
	case 7:
		return "shadeScale"
	case 8:
		return "shadeTitle"
	default:
		return ""
	}
}

func legacyGradientType(fillType uint32) string {
	switch fillType {
	case 5: // msofillShadeCenter: bounding rectangle -> focus point
		return "radial"
	case 6: // msofillShadeShape: shape outline -> focus point
		return "path"
	case 4, 7, 8: // shade, shadeScale, PowerPoint-hosted shadeTitle
		return "linear"
	default:
		return "linear"
	}
}

// contextualOfficeArtColorResolver resolves OfficeArt fSysIndex colors whose
// base color is another property of the same shape (for example 0xF0 = the
// shape fill color).  These references are common inside fillShadeColors and
// must be resolved before the gradient reaches canonical IR.
func contextualOfficeArtColorResolver(props []escher.Property, resolve ColorResolver) ColorResolver {
	if resolve == nil {
		resolve = ColorRefToHex
	}
	propertyRaw := func(id uint16) (uint32, bool) {
		for i := len(props) - 1; i >= 0; i-- {
			if props[i].ID == id && !props[i].IsComplex {
				return props[i].Value, true
			}
		}
		return 0, false
	}
	var resolveContext func(uint32, int) string
	resolveContext = func(raw uint32, depth int) string {
		if depth > 4 {
			return ""
		}
		flags := byte(raw >> 24)
		if flags&0x10 == 0 { // not fSysIndex
			return resolve(raw)
		}
		red, green, blue := byte(raw), byte(raw>>8), byte(raw>>16)
		var base string
		if red >= 0xF0 {
			var id uint16
			switch red {
			case 0xF0, 0xF7: // fill color; F7 falls back to line only when fill is absent
				id = escher.PropFillColor
			case 0xF1, 0xF2: // line-or-fill / line color
				id = escher.PropLineColor
			case 0xF3:
				id = escher.PropShadowColor
			case 0xF5:
				id = escher.PropFillBackColor
			}
			if id != 0 {
				if v, ok := propertyRaw(id); ok {
					base = resolveContext(v, depth+1)
				}
			}
			if base == "" && (red == 0xF1 || red == 0xF7) {
				fallback := escher.PropFillColor
				if red == 0xF7 {
					fallback = escher.PropLineColor
				}
				if v, ok := propertyRaw(fallback); ok {
					base = resolveContext(v, depth+1)
				}
			}
		} else if green == 0 && red < 25 {
			// Win32 system-color defaults used by OfficeArt when no host override is
			// available.  Keep this table here because fSysIndex is OfficeArt, not PPT.
			system := [...]string{"C0C0C0", "008080", "000080", "808080", "C0C0C0", "FFFFFF", "000000", "000000", "000000", "FFFFFF", "C0C0C0", "C0C0C0", "808080", "000080", "FFFFFF", "C0C0C0", "808080", "808080", "000000", "C0C0C0", "FFFFFF", "000000", "C0C0C0", "000000", "FFFFC0"}
			base = system[red]
		}
		if base == "" {
			return ""
		}
		return applyOfficeArtSystemColorTransform(base, green&0x0F, blue)
	}
	return func(raw uint32) string { return resolveContext(raw, 0) }
}

func applyOfficeArtSystemColorTransform(hex string, op, amount byte) string {
	if len(hex) != 6 || op == 0 {
		return hex
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(hex, "%02X%02X%02X", &r, &g, &b); err != nil {
		return hex
	}
	p := float64(amount) / 255.0
	transform := func(v uint8) uint8 {
		x := float64(v)
		switch op {
		case 1: // darken: 0 => black, FF => unchanged
			x *= p
		case 2: // lighten: 0 => white, FF => unchanged
			x = 255 - (255-x)*p
		default:
			return v
		}
		if x < 0 {
			x = 0
		}
		if x > 255 {
			x = 255
		}
		return uint8(math.Round(x))
	}
	return fmt.Sprintf("%02X%02X%02X", transform(r), transform(g), transform(b))
}

func legacyGradientShadeStops(props []escher.Property, resolve ColorResolver) []ir.GradientStop {
	if resolve == nil {
		resolve = ColorRefToHex
	}
	for _, prop := range props {
		if prop.ID != escher.PropFillShadeColors || !prop.IsComplex || len(prop.Complex) < 6 {
			continue
		}
		n := int(binary.LittleEndian.Uint16(prop.Complex[0:2]))
		cb := int(binary.LittleEndian.Uint16(prop.Complex[4:6]))
		if cb == 0xFFF0 {
			cb = 4
		}
		if n <= 0 || cb < 8 || 6+n*cb > len(prop.Complex) {
			return nil
		}
		stops := make([]ir.GradientStop, 0, n)
		for i := 0; i < n; i++ {
			off := 6 + i*cb
			raw := binary.LittleEndian.Uint32(prop.Complex[off : off+4])
			color := resolve(raw)
			if color == "" {
				return nil
			}
			pos := float64(int32(binary.LittleEndian.Uint32(prop.Complex[off+4:off+8]))) / 65536.0
			if pos < 0 {
				pos = 0
			}
			if pos > 1 {
				pos = 1
			}
			stops = append(stops, ir.GradientStop{Position: pos, Color: color, Alpha: 1})
		}
		return stops
	}
	return nil
}

// legacyGradientStops preserves OfficeArt fillFocus instead of flattening every
// shaded fill to a two-stop gradient. OfficeArt/VML focus is signed: positive
// values put the foreground color at both ends with the background color at the
// focus point; negative values do the inverse. +/-100 reverses the blend.
func legacyGradientStops(fore, back string, visual VisualProperties) []ir.GradientStop {
	focus := int32(0)
	if visual.HasFillFocus {
		focus = visual.FillFocus
	}
	if focus > 100 {
		focus = 100
	}
	if focus < -100 {
		focus = -100
	}
	if focus == 100 {
		return []ir.GradientStop{{Position: 0, Color: fore, Alpha: 1}, {Position: 1, Color: back, Alpha: 1}}
	}
	if focus == 0 {
		// An absent focus is OfficeArt's default, and it is not the same as
		// focus 100: the blend runs from the back color to the fill color.
		// Apache POI's HSLFFill builds the same order, and PowerPoint's own
		// OfficeArt -> OOXML conversion of the 41246-2 edge rules writes the
		// reversed stop list, matching the exported PDF.
		return []ir.GradientStop{{Position: 0, Color: back, Alpha: 1}, {Position: 1, Color: fore, Alpha: 1}}
	}
	if focus == -100 {
		return []ir.GradientStop{{Position: 0, Color: back, Alpha: 1}, {Position: 1, Color: fore, Alpha: 1}}
	}
	position := math.Abs(float64(focus)) / 100.0
	outer, inner := fore, back
	if focus < 0 {
		outer, inner = back, fore
	}
	// OfficeArt axial shades reverse the color roles when the authored angle is
	// negative.  This is not the same as reversing the final stop list: the
	// outer color remains mirrored at both ends.  PowerPoint uses this form for
	// reflected backgrounds (for example fillFocus=50 with a negative angle).
	if visual.HasFillAngle && visual.FillAngleDeg < 0 {
		outer, inner = inner, outer
	}
	return []ir.GradientStop{{Position: 0, Color: outer, Alpha: 1}, {Position: position, Color: inner, Alpha: 1}, {Position: 1, Color: outer, Alpha: 1}}
}

// officeArtLineGeometry reports shape types whose geometry is a line or
// connector. OfficeArt fills AutoShape areas white by default, but these
// families carry no area at all, so they must stay no-fill whether or not the
// record repeats a fillStyleBooleanProperties.
func officeArtLineGeometry(shapeType uint16) bool {
	return shapeType == 20 || (shapeType >= 32 && shapeType <= 40)
}

// OfficeArtMirrorsFlipV reports whether an OfficeArt preset is authored
// vertically mirrored relative to the IR preset that renders it, so callers
// have to exchange the fFlipV flag to keep the authored orientation.
//
// mso_sptTrapezoid is authored with its long edge on top, the mirror of the
// DrawingML trapezoid preset. The pyramid bands of the 41246-2 deck carry
// fFlipV and PowerPoint draws them long-edge-down, which is this preset
// unflipped.
func OfficeArtMirrorsFlipV(shapeType uint16) bool {
	return shapeType == 8
}
