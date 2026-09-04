package render

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"office/pkg/diag"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/htmlasset"
	"office/pkg/presentation/parsers/ooxml/resolve"
	drawingcss "office/pkg/render/drawingcss"
)

type HTMLImageMode string

type HTMLFontMode string

const (
	HTMLImageModeInline      HTMLImageMode = "inline"
	HTMLImageModeExternal    HTMLImageMode = "external"
	HTMLImageModePlaceholder HTMLImageMode = "placeholder"

	HTMLFontModeInline   HTMLFontMode = "inline"
	HTMLFontModeExternal HTMLFontMode = "external"
)

type HTMLAsset = htmlasset.Asset

type HTMLOptions struct {
	ImageMode        HTMLImageMode
	ExternalImageDir string
	FontMode         HTMLFontMode
	ExternalFontDir  string
}

type htmlRenderContext struct {
	embeddedFamilies map[string]struct{}
	options          HTMLOptions
	assets           []HTMLAsset
	nextImageIndex   int
	nextMediaIndex   int
	nextFontIndex    int
	nextEffectIndex  int
	animShapeIDs     map[int]struct{}
	slideNumber      int    // 1-based current slide number for field rendering
	defaultTextColor string // sRGB hex fallback when run.Color is empty (theme dk1)
}

// HTML renders a resolved Presentation as an HTML document.
// Each slide is rendered as a positioned canvas div.
func HTML(pres *resolve.Presentation) (string, error) {
	html, _, err := HTMLWithOptions(pres, HTMLOptions{})
	return html, err
}

// HTMLWithOptions renders a resolved Presentation as an HTML document and
// returns any sidecar assets generated for the selected image mode.
func HTMLWithOptions(pres *resolve.Presentation, opts HTMLOptions) (string, []HTMLAsset, error) {
	if pres == nil {
		return "", nil, fmt.Errorf("presentation render: presentation is nil")
	}
	if err := opts.validate(); err != nil {
		return "", nil, err
	}

	slideW := drawing.EMUToPoints(pres.SlideWidthEMU)
	slideH := drawing.EMUToPoints(pres.SlideHeightEMU)
	if slideW <= 0 {
		slideW = 720 // 10 inch default
	}
	if slideH <= 0 {
		slideH = 540 // 7.5 inch default
	}
	ctx := newHTMLRenderContext(pres, opts)
	playbackEnabled := presentationNeedsPlayback(pres)

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<style>\n")
	renderEmbeddedFontCSS(&b, pres, ctx)
	fmt.Fprintf(&b, slideCSS, slideW, slideH, slideW/slideH)
	if playbackEnabled {
		fmt.Fprintf(&b, playbackCSS, slideW/slideH)
	}
	if len(pres.Diagnostics) > 0 {
		b.WriteString(diagnosticsCSS)
	}
	b.WriteString("</style>\n")
	if playbackEnabled {
		b.WriteString("<script>\n")
		b.WriteString(playbackScript)
		b.WriteString("\n</script>\n")
	}
	b.WriteString("</head>\n<body>\n")
	renderDiagnostics(&b, pres.Diagnostics)

	for _, slide := range pres.Slides {
		ctx.slideNumber = slide.Index + 1
		renderSlide(&b, slide, slideW, slideH, ctx)
	}

	b.WriteString("</body>\n</html>")
	return b.String(), ctx.assetsCopy(), nil
}

func newHTMLRenderContext(pres *resolve.Presentation, opts HTMLOptions) *htmlRenderContext {
	ctx := &htmlRenderContext{
		embeddedFamilies: map[string]struct{}{},
		options:          opts.normalized(),
		defaultTextColor: "000000",
	}
	if pres == nil {
		return ctx
	}
	if pres.Theme != nil {
		if dk1, ok := pres.Theme.ColorScheme["dk1"]; ok && dk1 != "" {
			ctx.defaultTextColor = dk1
		}
	}
	for _, font := range pres.EmbeddedFonts {
		if font == nil {
			continue
		}
		key := normalizeFontFamily(font.Family)
		if key == "" {
			continue
		}
		ctx.embeddedFamilies[key] = struct{}{}
	}
	return ctx
}

func (opts HTMLOptions) normalized() HTMLOptions {
	imageMode := opts.ImageMode
	if imageMode == "" {
		imageMode = HTMLImageModeInline
	}
	fontMode := opts.FontMode
	if fontMode == "" {
		fontMode = HTMLFontModeInline
	}
	imageDir := htmlasset.NormalizeDir(opts.ExternalImageDir)
	if imageMode == HTMLImageModeExternal && imageDir == "" {
		imageDir = "images"
	}
	fontDir := htmlasset.NormalizeDir(opts.ExternalFontDir)
	if fontMode == HTMLFontModeExternal && fontDir == "" {
		fontDir = "fonts"
	}
	return HTMLOptions{
		ImageMode:        imageMode,
		ExternalImageDir: imageDir,
		FontMode:         fontMode,
		ExternalFontDir:  fontDir,
	}
}

func (opts HTMLOptions) validate() error {
	normalized := opts.normalized()
	switch normalized.ImageMode {
	case HTMLImageModeInline, HTMLImageModeExternal:
	default:
		return fmt.Errorf("presentation render: unsupported html image mode %q", normalized.ImageMode)
	}
	switch normalized.FontMode {
	case HTMLFontModeInline, HTMLFontModeExternal:
		return nil
	default:
		return fmt.Errorf("presentation render: unsupported html font mode %q", normalized.FontMode)
	}
}

func (ctx *htmlRenderContext) assetsCopy() []HTMLAsset {
	if ctx == nil || len(ctx.assets) == 0 {
		return nil
	}
	assets := make([]HTMLAsset, len(ctx.assets))
	copy(assets, ctx.assets)
	return assets
}

func renderSlide(b *strings.Builder, slide *resolve.Slide, slideW, slideH float64, ctx *htmlRenderContext) {
	if ctx != nil {
		ctx.animShapeIDs = animationTargetShapeIDs(slide)
	}
	fmt.Fprintf(b, "<div class=\"slide\" id=\"slide-%d\"", slide.Index+1)
	if slide.SectionName != "" {
		fmt.Fprintf(b, " data-section=%q", slide.SectionName)
	}
	if slide.Transition != nil {
		tr := slide.Transition
		if tr.Type != "" {
			fmt.Fprintf(b, " data-transition=%q", tr.Type)
		}
		if tr.Spd != "" {
			fmt.Fprintf(b, " data-transition-spd=%q", tr.Spd)
		}
		if !tr.AdvClick {
			b.WriteString(" data-transition-adv-click=\"0\"")
		}
		if tr.AdvTmMs > 0 {
			fmt.Fprintf(b, " data-transition-adv-tm=\"%d\"", tr.AdvTmMs)
		}
	}
	if maxStep := slideMaxAnimStep(slide); maxStep > 0 {
		fmt.Fprintf(b, " data-max-anim-step=%q", strconv.Itoa(maxStep))
	}
	if len(slide.AnimationTimeline) > 0 {
		b.WriteString(" data-has-anim-timeline=\"1\"")
	}
	b.WriteString(">\n")
	fmt.Fprintf(b, "<div class=\"slide-num\">Slide %d</div>\n", slide.Index+1)
	bgStyle := ""
	if slide.BgColor != "" {
		bgStyle = fmt.Sprintf(" style=\"background-color:#%s\"", slide.BgColor)
	}
	fmt.Fprintf(b, "<div class=\"slide-scroll\"><div class=\"canvas\"%s>\n", bgStyle)
	if slide.BgFill != nil && drawingcss.NeedsSVGDefinition(slide.BgFill) {
		fillDefID := fmt.Sprintf("slide-bg-fill-%d", slide.Index)
		bgWEMU := slideW * 12700.0
		bgHEMU := slideH * 12700.0
		fmt.Fprintf(b, "<svg class=\"slide-background\" style=\"position:absolute;inset:0;width:100%%;height:100%%;z-index:0\" viewBox=\"0 0 %.0f %.0f\" preserveAspectRatio=\"none\">", bgWEMU, bgHEMU)
		fmt.Fprintf(b, "<defs>%s</defs>", svgFillDefinition(slide.BgFill, slide.BgPictureTile, fillDefID, bgWEMU, bgHEMU, 1.0))
		fmt.Fprintf(b, "<rect width=\"100%%\" height=\"100%%\" fill=\"url(#%s)\"/></svg>\n", fillDefID)
	}

	for _, shape := range slide.Shapes {
		renderShape(b, shape, slideW, slideH, ctx)
	}

	b.WriteString("</div></div>\n")

	// Render headers/footers if present
	if slide.Header != nil || slide.Footer != nil || slide.DateTime != nil || slide.SlideNumber != nil {
		b.WriteString("<div class=\"headers-footers\">\n")
		if slide.Header != nil {
			b.WriteString("<div class=\"header\">\n")
			renderShape(b, slide.Header, slideW, slideH, ctx)
			b.WriteString("</div>\n")
		}
		if slide.Footer != nil {
			b.WriteString("<div class=\"footer\">\n")
			renderShape(b, slide.Footer, slideW, slideH, ctx)
			b.WriteString("</div>\n")
		}
		if slide.DateTime != nil {
			b.WriteString("<div class=\"datetime\">\n")
			renderShape(b, slide.DateTime, slideW, slideH, ctx)
			b.WriteString("</div>\n")
		}
		if slide.SlideNumber != nil {
			b.WriteString("<div class=\"slide-number\">\n")
			renderShape(b, slide.SlideNumber, slideW, slideH, ctx)
			b.WriteString("</div>\n")
		}
		b.WriteString("</div>\n")
	}

	// Render notes slide if present
	if slide.Notes != nil && len(slide.Notes.Shapes) > 0 {
		notesStyle := ""
		if slide.Notes.BgColor != "" {
			notesStyle = fmt.Sprintf(" style=\"background-color:#%s\"", slide.Notes.BgColor)
		}
		fmt.Fprintf(b, "<div class=\"notes\"%s>\n", notesStyle)
		for _, shape := range slide.Notes.Shapes {
			renderShape(b, shape, slideW, slideH, ctx)
		}
		b.WriteString("</div>\n")
	}

	// Render comments if present
	if len(slide.Comments) > 0 {
		renderSlideComments(b, slide.Comments)
	}
	if len(slide.AnimationTimeline) > 0 {
		if raw, err := json.Marshal(slide.AnimationTimeline); err == nil {
			payload := strings.ReplaceAll(string(raw), "<", "\\u003c")
			fmt.Fprintf(b, "<script type=\"application/json\" class=\"ppt-animation-timeline\">%s</script>\n", payload)
		}
	}

	b.WriteString("</div>\n")
}

func renderSlideComments(b *strings.Builder, comments []*resolve.Comment) {
	b.WriteString("<aside class=\"comments\">\n")
	for _, c := range comments {
		b.WriteString("<div class=\"comment\">\n")
		b.WriteString("<div class=\"comment-meta\">")
		if c.AuthorName != "" {
			fmt.Fprintf(b, "<span class=\"comment-author\">%s</span>", escapeHTML(c.AuthorName))
		}
		if c.DateTime != "" {
			fmt.Fprintf(b, "<span class=\"comment-dt\">%s</span>", escapeHTML(c.DateTime))
		}
		b.WriteString("</div>\n")
		fmt.Fprintf(b, "<p class=\"comment-text\">%s</p>\n", escapeHTML(c.Text))
		b.WriteString("</div>\n")
	}
	b.WriteString("</aside>\n")
}

func renderShape(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	if shape.HyperlinkURL != "" || shape.ActionLink != "" {
		var inner strings.Builder
		renderShapeContent(&inner, shape, slideW, slideH, ctx)
		if shape.HyperlinkURL != "" {
			fmt.Fprintf(b, "<a href=%q class=\"shape-link\">%s</a>\n", escapeAttr(shape.HyperlinkURL), inner.String())
		} else {
			fmt.Fprintf(b, "<a href=%q class=\"shape-link\" data-ppt-action=%q>%s</a>\n", "#"+actionLinkAnchor(shape.ActionLink), escapeAttr(shape.ActionLink), inner.String())
		}
		return
	}
	renderShapeContent(b, shape, slideW, slideH, ctx)
}

func actionLinkAnchor(action string) string {
	// "slide3" → "slide-3"; named actions become "#action-firstSlide" etc.
	if strings.HasPrefix(action, "slide") {
		n := action[len("slide"):]
		if n != "" {
			return "slide-" + n
		}
	}
	return "action-" + action
}

func renderShapeContent(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	switch shape.Kind {
	case resolve.ShapeText:
		renderTextShape(b, shape, slideW, slideH, ctx)
	case resolve.ShapeImage:
		renderImageShape(b, shape, slideW, slideH, ctx)
	case resolve.ShapeGroup:
		for _, child := range shape.Children {
			renderShape(b, child, slideW, slideH, ctx)
		}
	case resolve.ShapeTable:
		renderTableShape(b, shape, slideW, slideH, ctx)
	case resolve.ShapeConnector:
		renderConnector(b, shape, slideW, slideH, ctx)
	case resolve.ShapeSmartArt:
		renderSmartArtShape(b, shape, slideW, slideH, ctx)
	case resolve.ShapeSmartArtConnector:
		renderSmartArtConnector(b, shape, slideW, slideH, ctx)
	case resolve.ShapeChart:
		renderChartShape(b, shape, slideW, slideH, ctx)
	case resolve.ShapeMedia:
		renderMediaShape(b, shape, slideW, slideH, ctx)
	case resolve.ShapeOleObject:
		renderOleObjectShape(b, shape, slideW, slideH, ctx)
	}
}

func renderTableShape(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	if shape == nil || shape.Table == nil || len(shape.Table.Rows) == 0 {
		return
	}
	x, y, w, h := boundsPercent(shape.Bounds, slideW, slideH)
	styles := []string{
		fmt.Sprintf("left:%.4f%%", x),
		fmt.Sprintf("top:%.4f%%", y),
		fmt.Sprintf("width:%.4f%%", w),
	}
	if h > 0 {
		styles = append(styles, fmt.Sprintf("min-height:%.4f%%", h))
	}
	effectStyles := effectStyleProps(shape)
	if len(effectStyles) > 0 {
		styles = append(styles, "overflow:visible")
		transform, origin := mergeEffectTransform("", effectStyles)
		if transform != "" {
			styles = append(styles, "transform:"+transform)
		}
		if origin != "" {
			styles = append(styles, "transform-origin:"+origin)
		}
		styles = appendSortedCSSProps(styles, effectStyles)
	}
	fmt.Fprintf(b, "<div class=\"shape shape--table\" style=%q%s%s%s%s>\n", strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape))
	b.WriteString("<table class=\"ppt-table\">\n")
	if len(shape.Table.ColumnWidthsEMU) > 0 {
		total := int64(0)
		for _, colW := range shape.Table.ColumnWidthsEMU {
			total += colW
		}
		if total > 0 {
			b.WriteString("<colgroup>")
			for _, colW := range shape.Table.ColumnWidthsEMU {
				fmt.Fprintf(b, "<col style=%q>", fmt.Sprintf("width:%.4f%%", float64(colW)*100/float64(total)))
			}
			b.WriteString("</colgroup>\n")
		}
	}
	for _, row := range shape.Table.Rows {
		rowStyle := ""
		if row != nil && row.HeightEMU > 0 {
			rowStyle = fmt.Sprintf(" style=%q", fmt.Sprintf("height:%.2fpt", emuToPt(row.HeightEMU)))
		}
		fmt.Fprintf(b, "<tr%s>\n", rowStyle)
		for _, cell := range row.Cells {
			if cell == nil || cell.HMerge || cell.VMerge {
				continue
			}
			var attrs []string
			if cell.GridSpan > 1 {
				attrs = append(attrs, fmt.Sprintf("colspan=\"%d\"", cell.GridSpan))
			}
			if cell.RowSpan > 1 {
				attrs = append(attrs, fmt.Sprintf("rowspan=\"%d\"", cell.RowSpan))
			}
			var cellStyles []string
			cellStyles = append(cellStyles, cssBackgroundFillPropsForFill(cell.Fill, cell.FillColor)...)
			cellStyles = appendSortedCSSProps(cellStyles, effectStylePropsForList(cell.Effects))
			appendBorderStyle := func(slot string, line *drawing.LineStyle) {
				if line == nil {
					return
				}
				width := line.Width
				if width <= 0 {
					width = 0.75
				}
				color := line.Color
				if color == "" {
					color = "000000"
				}
				cellStyles = append(cellStyles, fmt.Sprintf("border-%s:%.2fpt solid #%s", slot, width, color))
			}
			appendBorderStyle("left", cell.BorderLeft)
			appendBorderStyle("right", cell.BorderRight)
			appendBorderStyle("top", cell.BorderTop)
			appendBorderStyle("bottom", cell.BorderBottom)
			if cell.MarginLeftEMU != 0 || cell.MarginRightEMU != 0 || cell.MarginTopEMU != 0 || cell.MarginBottomEMU != 0 {
				cellStyles = append(cellStyles, fmt.Sprintf("padding:%.2fpt %.2fpt %.2fpt %.2fpt", emuToPt(cell.MarginTopEMU), emuToPt(cell.MarginRightEMU), emuToPt(cell.MarginBottomEMU), emuToPt(cell.MarginLeftEMU)))
			}
			switch cell.TextAnchor {
			case "ctr":
				cellStyles = append(cellStyles, "vertical-align:middle")
			case "b":
				cellStyles = append(cellStyles, "vertical-align:bottom")
			default:
				cellStyles = append(cellStyles, "vertical-align:top")
			}
			if len(cellStyles) > 0 {
				attrs = append(attrs, fmt.Sprintf("style=%q", strings.Join(cellStyles, ";")))
			}
			fmt.Fprintf(b, "<td %s>", strings.Join(attrs, " "))
			autoCounters := map[string]int{}
			for _, p := range cell.Paragraphs {
				renderParagraph(b, p, autoCounters, 1.0, 1.0, ctx)
			}
			b.WriteString("</td>\n")
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</table>\n")
	writeHTMLFillOverlays(b, shape.Effects)
	b.WriteString("</div>\n")
}

func renderTextShape(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	hasContent := len(shape.Paragraphs) > 0 || shapeHasVisibleFill(shape) || shape.Geometry != "" || shape.GeometryData != nil || shape.LineColor != "" || shapeHasTimelineTarget(shape, ctx)
	if !hasContent {
		return
	}
	effectStyles := effectStyleProps(shape)

	x, y, w, h := boundsPercent(shape.Bounds, slideW, slideH)

	var classes []string
	classes = append(classes, "shape", "shape--text")
	if shape.Placeholder != resolve.PlaceholderNone {
		classes = append(classes, placeholderClass(shape.Placeholder))
	}

	var styles []string
	if shape.TextAutoGrow {
		styles = append(styles,
			fmt.Sprintf("left:%.4f%%", x),
			fmt.Sprintf("top:%.4f%%", y),
			fmt.Sprintf("width:%.4f%%", w),
			fmt.Sprintf("min-height:%.4f%%", h),
		)
	} else {
		styles = append(styles,
			fmt.Sprintf("left:%.4f%%", x),
			fmt.Sprintf("top:%.4f%%", y),
			fmt.Sprintf("width:%.4f%%", w),
			fmt.Sprintf("height:%.4f%%", h),
		)
	}
	if len(effectStyles) > 0 || shape.TextAllowOverflow || shape.TextAutoGrow {
		styles = append(styles, "overflow:visible")
	}
	// Apply rotation and flip transforms
	var transforms []string
	if shape.RotationDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", shape.RotationDeg))
	}
	if shape.FlipH {
		transforms = append(transforms, "scaleX(-1)")
	}
	if shape.FlipV {
		transforms = append(transforms, "scaleY(-1)")
	}
	transform, effectOrigin := mergeEffectTransform(strings.Join(transforms, " "), effectStyles)
	if transform != "" {
		styles = append(styles, "transform:"+transform)
		if effectOrigin != "" {
			styles = append(styles, "transform-origin:"+effectOrigin)
		} else {
			styles = append(styles, "transform-origin:50% 50%")
		}
	} else if effectOrigin != "" {
		styles = append(styles, "transform-origin:"+effectOrigin)
	}
	if len(effectStyles) > 0 {
		styles = appendSortedCSSProps(styles, effectStyles)
	}

	fmt.Fprintf(b, "<div class=%q style=%q%s%s%s%s>\n", strings.Join(classes, " "), strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape))
	renderedVector := renderTextShapeVector(b, shape, nil)
	if !renderedVector {
		if shape.FillColor != "" {
			fmt.Fprintf(b, "<div class=\"shape__background\" style=%q></div>\n", strings.Join(rectShapeStyles(shape), ";"))
		} else if shape.LineColor != "" && shape.LineWidthPt > 0 {
			fmt.Fprintf(b, "<div class=\"shape__background\" style=%q></div>\n", strings.Join(rectShapeStyles(shape), ";"))
		}
		writeHTMLFillOverlays(b, shape.Effects)
	}

	var contentStyles []string
	contentHeight := "height:100%"
	if shape.TextAutoGrow || shape.TextAllowOverflow {
		contentHeight = "height:auto"
	}
	contentStyles = append(contentStyles,
		"position:relative",
		"z-index:1",
		"display:flex",
		"flex-direction:column",
		"box-sizing:border-box",
		"width:100%",
		contentHeight,
	)
	if shape.TextAllowOverflow || shape.TextAutoGrow {
		contentStyles = append(contentStyles, "overflow:visible")
	}

	// Use drawingcss.TextBodyProps for text body styling
	cssProps := drawingcss.TextBodyProps(shape.TextBodyProps)
	contentStyles = appendSortedCSSProps(contentStyles, cssProps)

	// Fall back to individual fields if CSS() doesn't provide certain properties
	if _, hasJustifyContent := cssProps["justify-content"]; !hasJustifyContent && shape.TextAnchor != "" {
		contentStyles = append(contentStyles, "justify-content:"+textAnchorJustify(shape.TextAnchor))
	}

	// Use TextBodyProps for insets if available, otherwise fall back to individual fields
	topInset := drawing.EMUToPoints(shape.TextBodyProps.InsetT)
	rightInset := drawing.EMUToPoints(shape.TextBodyProps.InsetR)
	bottomInset := drawing.EMUToPoints(shape.TextBodyProps.InsetB)
	leftInset := drawing.EMUToPoints(shape.TextBodyProps.InsetL)

	// Fall back to individual fields if TextBodyProps insets are zero
	if topInset == 0 {
		topInset = drawing.EMUToPoints(shape.TextInsetTopEMU)
	}
	if rightInset == 0 {
		rightInset = drawing.EMUToPoints(shape.TextInsetRightEMU)
	}
	if bottomInset == 0 {
		bottomInset = drawing.EMUToPoints(shape.TextInsetBottomEMU)
	}
	if leftInset == 0 {
		leftInset = drawing.EMUToPoints(shape.TextInsetLeftEMU)
	}

	if shape.Geometry == "ellipse" {
		width := drawing.EMUToPoints(shape.Bounds.W)
		height := drawing.EMUToPoints(shape.Bounds.H)
		if width > 0 && height > 0 {
			topInset = maxFloat(topInset, height*0.16)
			rightInset = maxFloat(rightInset, width*0.18)
			bottomInset = maxFloat(bottomInset, height*0.16)
			leftInset = maxFloat(leftInset, width*0.18)
		}
	}
	if topInset > 0 || rightInset > 0 || bottomInset > 0 || leftInset > 0 {
		contentStyles = append(contentStyles, fmt.Sprintf("padding:%.2fpt %.2fpt %.2fpt %.2fpt", topInset, rightInset, bottomInset, leftInset))
	}
	if shape.Geometry == "ellipse" {
		contentStyles = append(contentStyles, "clip-path:ellipse(50% 50% at 50% 50%)")
	}
	fmt.Fprintf(b, "<div class=\"shape__content\" style=%q>\n", strings.Join(contentStyles, ";"))

	autoCounters := map[string]int{}
	fontScale := textAutofitFontScale(shape.TextBodyProps)
	lineScale := textAutofitLineScale(shape.TextBodyProps)
	for _, p := range shape.Paragraphs {
		renderParagraph(b, p, autoCounters, fontScale, lineScale, ctx)
	}

	b.WriteString("</div>\n")
	b.WriteString("</div>\n")
}

func appendSortedCSSProps(dst []string, props map[string]string) []string {
	if len(props) == 0 {
		return dst
	}
	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		dst = append(dst, key+":"+props[key])
	}
	return dst
}

func renderTextShapeVector(b *strings.Builder, shape *resolve.Shape, effectStyles map[string]string) bool {
	if shape == nil {
		return false
	}
	requiresVectorFill := drawingcss.NeedsSVGDefinition(shape.Fill)
	if shape.Geometry == "" && shape.GeometryData == nil && !requiresVectorFill {
		return false
	}
	width := drawing.EMUToPoints(shape.Bounds.W)
	height := drawing.EMUToPoints(shape.Bounds.H)
	if width <= 0 || height <= 0 {
		return false
	}
	presetName := shape.Geometry
	if presetName == "" {
		presetName = "rect"
	}
	primitives, err := drawing.BuildShapePrimitives(drawing.ShapeGeometrySpec{
		Geometry:    shape.GeometryData,
		Preset:      &drawing.PresetGeometry{Name: presetName, Adjustments: shape.GeometryAdjustments},
		Width:       width,
		Height:      height,
		Fill:        shape.Fill,
		FillColor:   shape.FillColor,
		Stroke:      shape.LineStyle,
		StrokeColor: shape.LineColor,
		StrokeWidth: shape.LineWidthPt,
	})
	if err != nil {
		return false
	}
	if len(primitives) == 0 {
		return false
	}

	fillDefID := ""
	fillDef := ""
	if drawingcss.NeedsSVGDefinition(shape.Fill) {
		fillDefID = "shape-fill"
		fillDef = svgFillDefinition(shape.Fill, shape.PictureTile, fillDefID, width, height, 1.0/12700.0)
	}
	lightOverlays := drawingcss.ThreeDLightOverlays(extractShapeThreeD(shape.Effects))
	overlayRenders := drawingcss.BuildSVGOverlays(lightOverlays, "shape-fill-overlay")
	fillOverlays := fillOverlayEffects(shape.Effects)
	fillOverlayDefs := make([]string, len(fillOverlays))
	fillOverlayDefIDs := make([]string, len(fillOverlays))
	for i, overlay := range fillOverlays {
		if overlay.Fill != nil && drawingcss.NeedsSVGDefinition(overlay.Fill) {
			id := fmt.Sprintf("shape-effect-fill-%d", i)
			fillOverlayDefIDs[i] = id
			fillOverlayDefs[i] = svgDrawingFillDefinition(overlay.Fill, id)
		}
	}
	strokeDefs := collectStrokeFillDefs(primitives, "shape")
	fmt.Fprintf(b, "<svg class=\"shape__vector\" viewBox=\"0 0 %.4f %.4f\" preserveAspectRatio=\"none\" aria-hidden=\"true\"", width, height)
	if len(effectStyles) > 0 {
		var styles []string
		styles = appendSortedCSSProps(styles, effectStyles)
		fmt.Fprintf(b, " style=%q", strings.Join(styles, ";"))
	}
	b.WriteString(" xmlns=\"http://www.w3.org/2000/svg\">")
	if fillDef != "" || len(strokeDefs) > 0 || len(overlayRenders) > 0 || len(fillOverlayDefs) > 0 {
		b.WriteString("<defs>")
		if fillDef != "" {
			b.WriteString(fillDef)
		}
		for _, def := range strokeDefs {
			b.WriteString(def)
		}
		for _, overlay := range overlayRenders {
			if overlay.Def != "" {
				b.WriteString(overlay.Def)
			}
		}
		for _, def := range fillOverlayDefs {
			if def != "" {
				b.WriteString(def)
			}
		}
		b.WriteString("</defs>")
	}
	writeThreeDExtrusionSVG(b, primitives, extractShapeThreeD(shape.Effects))
	for _, primitive := range primitives {
		writeSVGPrimitive(b, primitive, fillDefID)
	}
	for _, overlay := range overlayRenders {
		if overlay.Fill == nil {
			continue
		}
		if blendStyle := drawingcss.SVGOverlayBlendStyle(overlay.BlendMode); blendStyle != "" {
			fmt.Fprintf(b, "<g style=%q>", blendStyle)
		} else {
			b.WriteString("<g>")
		}
		for _, primitive := range primitives {
			writeSVGPrimitive(b, overlayShapePrimitiveFill(primitive, overlay.Fill), overlay.DefID)
		}
		b.WriteString("</g>")
	}
	for i, overlay := range fillOverlays {
		if overlay.Fill == nil {
			continue
		}
		style := ""
		if mode := cssBlendMode(overlay.BlendMode); mode != "" && mode != "normal" {
			style = "mix-blend-mode:" + mode
		}
		if style != "" {
			fmt.Fprintf(b, "<g class=\"shape__effect-overlay\" style=%q>", style)
		} else {
			b.WriteString("<g class=\"shape__effect-overlay\">")
		}
		for _, primitive := range primitives {
			writeSVGPrimitive(b, overlayShapePrimitiveFill(primitive, overlay.Fill), fillOverlayDefIDs[i])
		}
		b.WriteString("</g>")
	}
	b.WriteString("</svg>\n")
	return true
}

func svgFillDefinition(fill drawing.Fill, tile *resolve.PictureTile, id string, width, height, emuScale float64) string {
	def := svgDrawingFillDefinition(fill, id)
	if tile == nil || def == "" || width <= 0 || height <= 0 {
		return def
	}
	start := strings.Index(def, "<pattern")
	if start < 0 {
		return def
	}
	endRel := strings.Index(def[start:], ">")
	if endRel < 0 {
		return def
	}
	end := start + endRel

	sx, sy := tile.Sx, tile.Sy
	if sx == 0 {
		sx = 100000
	}
	if sy == 0 {
		sy = 100000
	}
	tileW := width * float64(sx) / 100000.0
	tileH := height * float64(sy) / 100000.0
	if tileW <= 0 || tileH <= 0 {
		return def
	}
	x, y := pictureTileAlignmentOffset(tile.Alignment, width, height, tileW, tileH)
	x += float64(tile.Tx) * emuScale
	y += float64(tile.Ty) * emuScale

	patternW, patternH := tileW, tileH
	flip := strings.ToLower(strings.TrimSpace(tile.Flip))
	switch flip {
	case "x":
		patternW *= 2
	case "y":
		patternH *= 2
	case "xy":
		patternW *= 2
		patternH *= 2
	default:
		flip = ""
	}

	open := fmt.Sprintf(`<pattern id=%q patternUnits="userSpaceOnUse" x="%.4f" y="%.4f" width="%.4f" height="%.4f"`, id, x, y, patternW, patternH)
	if flip != "" {
		open += fmt.Sprintf(` data-ppt-tile-flip=%q`, flip)
	}
	open += ">"

	if flip == "" {
		return def[:start] + open + def[end+1:]
	}
	closeRel := strings.Index(def[end+1:], "</pattern>")
	if closeRel < 0 {
		return def[:start] + open + def[end+1:]
	}
	close := end + 1 + closeRel
	content := def[end+1 : close]
	flipped := pictureTileFlipContent(content, flip, tileW, tileH)
	return def[:start] + open + flipped + def[close:]
}

func pictureTileFlipContent(content, flip string, tileW, tileH float64) string {
	if content == "" || tileW <= 0 || tileH <= 0 {
		return content
	}

	tile := func(x, y float64, transform string) string {
		inner := fmt.Sprintf(`<svg x="%.4f" y="%.4f" width="%.4f" height="%.4f" viewBox="0 0 1 1" preserveAspectRatio="none">%s</svg>`, x, y, tileW, tileH, content)
		if transform == "" {
			return inner
		}
		return fmt.Sprintf(`<g transform=%q>%s</g>`, transform, inner)
	}

	var b strings.Builder
	b.WriteString(tile(0, 0, ""))
	switch flip {
	case "x":
		b.WriteString(tile(0, 0, fmt.Sprintf("translate(%.4f 0) scale(-1 1)", 2*tileW)))
	case "y":
		b.WriteString(tile(0, 0, fmt.Sprintf("translate(0 %.4f) scale(1 -1)", 2*tileH)))
	case "xy":
		b.WriteString(tile(0, 0, fmt.Sprintf("translate(%.4f 0) scale(-1 1)", 2*tileW)))
		b.WriteString(tile(0, 0, fmt.Sprintf("translate(0 %.4f) scale(1 -1)", 2*tileH)))
		b.WriteString(tile(0, 0, fmt.Sprintf("translate(%.4f %.4f) scale(-1 -1)", 2*tileW, 2*tileH)))
	}
	return b.String()
}

func pictureTileAlignmentOffset(alignment string, width, height, tileW, tileH float64) (float64, float64) {
	var x, y float64
	switch alignment {
	case "t", "ctr", "b":
		x = (width - tileW) / 2
	case "tr", "r", "br":
		x = width - tileW
	}
	switch alignment {
	case "l", "ctr", "r":
		y = (height - tileH) / 2
	case "bl", "b", "br":
		y = height - tileH
	}
	return x, y
}

func extractShapeThreeD(effects drawing.EffectList) *drawing.ThreeD {
	for _, effect := range effects.Effects {
		if threeD, ok := effect.(drawing.ThreeD); ok {
			return &threeD
		}
		if threeD, ok := effect.(*drawing.ThreeD); ok {
			return threeD
		}
	}
	return nil
}

func writeThreeDExtrusionSVG(b *strings.Builder, primitives []drawing.VectorPrimitive, t *drawing.ThreeD) {
	if b == nil || t == nil || t.ExtrusionDepth <= 0 || len(primitives) == 0 {
		return
	}
	depth := t.ExtrusionDepth
	steps := int(math.Ceil(depth / 1.5))
	if steps < 2 {
		steps = 2
	}
	if steps > 14 {
		steps = 14
	}
	dirX, dirY := threeDDepthVector(*t)
	color := strings.TrimPrefix(strings.TrimSpace(t.ExtrusionColor), "#")
	if color == "" {
		color = "555555"
	}
	fill := drawing.SolidFill{Color: color}
	fmt.Fprintf(b, "<g class=\"shape__3d-extrusion\" data-depth=\"%.2f\" data-material=%q>", depth, escapeAttr(t.Material))
	for i := steps; i >= 1; i-- {
		f := float64(i) / float64(steps)
		x := dirX * depth * f
		y := dirY * depth * f
		opacity := 0.32 + (1-f)*0.28
		fmt.Fprintf(b, "<g transform=\"translate(%.4f %.4f)\" opacity=\"%.4f\">", x, y, opacity)
		for _, primitive := range primitives {
			writeSVGPrimitive(b, overlayShapePrimitiveFill(primitive, fill), "")
		}
		b.WriteString("</g>")
	}
	b.WriteString("</g>")
}

func threeDDepthVector(t drawing.ThreeD) (float64, float64) {
	// Project extrusion away from the camera. Camera rotations dominate; preset
	// names provide a deterministic fallback for files that omit explicit rot.
	x := math.Sin(t.RotationY * math.Pi / 180.0)
	y := math.Sin(t.RotationX * math.Pi / 180.0)
	if math.Abs(x)+math.Abs(y) < 0.05 {
		preset := strings.ToLower(t.CameraPreset)
		switch {
		case strings.Contains(preset, "topleft"):
			x, y = 0.65, 0.45
		case strings.Contains(preset, "topright"):
			x, y = -0.65, 0.45
		case strings.Contains(preset, "bottomleft"):
			x, y = 0.65, -0.45
		case strings.Contains(preset, "bottomright"):
			x, y = -0.65, -0.45
		default:
			x, y = 0.55, 0.38
		}
	}
	length := math.Hypot(x, y)
	if length == 0 {
		return 0.55, 0.38
	}
	return x / length, y / length
}

func overlayShapePrimitiveFill(primitive drawing.VectorPrimitive, fill drawing.Fill) drawing.VectorPrimitive {
	primitive.Style.Fill = fill
	primitive.Style.FillColor = ""
	primitive.Style.Stroke = nil
	primitive.Style.StrokeColor = ""
	primitive.Style.StrokeWidth = 0
	return primitive
}

func pictureFillEffects(fill drawing.Fill) []drawing.Effect {
	switch f := fill.(type) {
	case drawing.PictureFill:
		return f.BlipEffects
	case *drawing.PictureFill:
		if f != nil {
			return f.BlipEffects
		}
	}
	return nil
}

func svgDrawingFillDefinition(fill drawing.Fill, id string) string {
	def := drawingcss.SVGDefinition(fill, id)
	effects := pictureFillEffects(fill)
	if len(effects) == 0 || def == "" {
		return def
	}
	filterID := id + "-blip-filter"
	filterDef := imageEffectFilterDefinition(effects, filterID)
	if filterDef == "" {
		return def
	}
	// drawingcss picture definitions contain an <image> element. Apply the
	// generated DrawingML blip filter only to that image, not to shape text or
	// strokes that happen to use the picture fill.
	def = strings.Replace(def, "<image ", "<image style=\"filter:url(#"+filterID+")\" ", 1)
	return filterDef + def
}

func imageEffectNames(effects []drawing.Effect) string {
	if len(effects) == 0 {
		return ""
	}
	var names []string
	for _, effect := range effects {
		if effect != nil {
			names = append(names, string(effect.Type()))
		}
	}
	return strings.Join(names, " ")
}

func imageEffectFilterDefinition(effects []drawing.Effect, id string) string {
	if len(effects) == 0 || id == "" {
		return ""
	}
	var primitives []string
	input := "SourceGraphic"
	nextResult := func() string { return fmt.Sprintf("fx%d", len(primitives)+1) }
	appendPrimitive := func(fragment, result string) {
		primitives = append(primitives, fragment)
		input = result
	}

	for _, effect := range effects {
		if effect == nil {
			continue
		}
		switch e := effect.(type) {
		case drawing.Grayscale, *drawing.Grayscale, drawing.DeSat, *drawing.DeSat:
			result := nextResult()
			appendPrimitive(fmt.Sprintf(`<feColorMatrix in="%s" type="saturate" values="0" result="%s"/>`, input, result), result)
		case drawing.BiLevel:
			result := appendBiLevelFilter(&primitives, input, e.Threshold)
			input = result
		case *drawing.BiLevel:
			if e != nil {
				input = appendBiLevelFilter(&primitives, input, e.Threshold)
			}
		case drawing.Duotone:
			input = appendDuotoneFilter(&primitives, input, e.Color1, e.Color2)
		case *drawing.Duotone:
			if e != nil {
				input = appendDuotoneFilter(&primitives, input, e.Color1, e.Color2)
			}
		case drawing.Lum:
			input = appendLuminanceFilter(&primitives, input, e.Brightness, e.Contrast)
		case *drawing.Lum:
			if e != nil {
				input = appendLuminanceFilter(&primitives, input, e.Brightness, e.Contrast)
			}
		case drawing.HSLEffect:
			input = appendHSLFilter(&primitives, input, e)
		case *drawing.HSLEffect:
			if e != nil {
				input = appendHSLFilter(&primitives, input, *e)
			}
		case drawing.DeSatMod:
			result := nextResult()
			amount := clampFloat64(float64(e.Amount)/100000.0, 0, 1)
			appendPrimitive(fmt.Sprintf(`<feColorMatrix in="%s" type="saturate" values="%.6f" result="%s"/>`, input, amount, result), result)
		case *drawing.DeSatMod:
			if e != nil {
				result := nextResult()
				amount := clampFloat64(float64(e.Amount)/100000.0, 0, 1)
				appendPrimitive(fmt.Sprintf(`<feColorMatrix in="%s" type="saturate" values="%.6f" result="%s"/>`, input, amount, result), result)
			}
		case drawing.AlphaMod:
			input = appendAlphaLinearFilter(&primitives, input, float64(e.Amount)/100000.0, 0)
		case *drawing.AlphaMod:
			if e != nil {
				input = appendAlphaLinearFilter(&primitives, input, float64(e.Amount)/100000.0, 0)
			}
		case drawing.AlphaModFix:
			input = appendAlphaLinearFilter(&primitives, input, float64(e.Amount)/100000.0, 0)
		case *drawing.AlphaModFix:
			if e != nil {
				input = appendAlphaLinearFilter(&primitives, input, float64(e.Amount)/100000.0, 0)
			}
		case drawing.AlphaOff:
			input = appendAlphaLinearFilter(&primitives, input, 1, float64(e.Amount)/100000.0)
		case *drawing.AlphaOff:
			if e != nil {
				input = appendAlphaLinearFilter(&primitives, input, 1, float64(e.Amount)/100000.0)
			}
		case drawing.AlphaRepl:
			input = appendAlphaLinearFilter(&primitives, input, 0, float64(e.Alpha)/100000.0)
		case *drawing.AlphaRepl:
			if e != nil {
				input = appendAlphaLinearFilter(&primitives, input, 0, float64(e.Alpha)/100000.0)
			}
		case drawing.AlphaInv:
			input = appendAlphaInvertFilter(&primitives, input)
		case *drawing.AlphaInv:
			if e != nil {
				input = appendAlphaInvertFilter(&primitives, input)
			}
		case drawing.AlphaBiLevel:
			input = appendAlphaThresholdFilter(&primitives, input, float64(e.Threshold)/100000.0)
		case *drawing.AlphaBiLevel:
			if e != nil {
				input = appendAlphaThresholdFilter(&primitives, input, float64(e.Threshold)/100000.0)
			}
		case drawing.AlphaCeiling:
			input = appendAlphaCeilingFilter(&primitives, input)
		case *drawing.AlphaCeiling:
			if e != nil {
				input = appendAlphaCeilingFilter(&primitives, input)
			}
		case drawing.AlphaFloor:
			input = appendAlphaFloorFilter(&primitives, input)
		case *drawing.AlphaFloor:
			if e != nil {
				input = appendAlphaFloorFilter(&primitives, input)
			}
		case drawing.AlphaOutset:
			input = appendAlphaOutsetFilter(&primitives, input, e.Radius)
		case *drawing.AlphaOutset:
			if e != nil {
				input = appendAlphaOutsetFilter(&primitives, input, e.Radius)
			}
		case drawing.ClrRepl:
			input = appendColorReplacementFilter(&primitives, input, e.Color)
		case *drawing.ClrRepl:
			if e != nil {
				input = appendColorReplacementFilter(&primitives, input, e.Color)
			}
		case drawing.ClrChange:
			input = appendColorChangeFilter(&primitives, input, e)
		case *drawing.ClrChange:
			if e != nil {
				input = appendColorChangeFilter(&primitives, input, *e)
			}
		}
	}
	if len(primitives) == 0 {
		return ""
	}
	return fmt.Sprintf(`<filter id="%s" x="-25%%" y="-25%%" width="150%%" height="150%%" color-interpolation-filters="sRGB">%s</filter>`, escapeAttr(id), strings.Join(primitives, ""))
}

func appendBiLevelFilter(primitives *[]string, input string, threshold int) string {
	gray := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feColorMatrix in="%s" type="saturate" values="0" result="%s"/>`, input, gray))
	result := fmt.Sprintf("fx%d", len(*primitives)+1)
	t := clampFloat64(float64(threshold)/100000.0, 0, 1)
	// A steep linear transfer approximates DrawingML's threshold operation.
	slope := 100.0
	intercept := -slope * t
	*primitives = append(*primitives, fmt.Sprintf(`<feComponentTransfer in="%s" result="%s"><feFuncR type="linear" slope="%.6f" intercept="%.6f"/><feFuncG type="linear" slope="%.6f" intercept="%.6f"/><feFuncB type="linear" slope="%.6f" intercept="%.6f"/></feComponentTransfer>`, gray, result, slope, intercept, slope, intercept, slope, intercept))
	return result
}

func appendDuotoneFilter(primitives *[]string, input, color1, color2 string) string {
	r1, g1, b1, ok1 := hexRGB01(color1)
	r2, g2, b2, ok2 := hexRGB01(color2)
	if !ok1 || !ok2 {
		return input
	}
	gray := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feColorMatrix in="%s" type="matrix" values="0.2126 0.7152 0.0722 0 0 0.2126 0.7152 0.0722 0 0 0.2126 0.7152 0.0722 0 0 0 0 0 1 0" result="%s"/>`, input, gray))
	result := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feComponentTransfer in="%s" result="%s"><feFuncR type="linear" slope="%.6f" intercept="%.6f"/><feFuncG type="linear" slope="%.6f" intercept="%.6f"/><feFuncB type="linear" slope="%.6f" intercept="%.6f"/></feComponentTransfer>`, gray, result, r2-r1, r1, g2-g1, g1, b2-b1, b1))
	return result
}

func appendLuminanceFilter(primitives *[]string, input string, brightness, contrast int) string {
	result := fmt.Sprintf("fx%d", len(*primitives)+1)
	c := 1.0 + float64(contrast)/100000.0
	if c < 0 {
		c = 0
	}
	b := float64(brightness) / 100000.0
	intercept := 0.5 - 0.5*c + b
	*primitives = append(*primitives, fmt.Sprintf(`<feComponentTransfer in="%s" result="%s"><feFuncR type="linear" slope="%.6f" intercept="%.6f"/><feFuncG type="linear" slope="%.6f" intercept="%.6f"/><feFuncB type="linear" slope="%.6f" intercept="%.6f"/></feComponentTransfer>`, input, result, c, intercept, c, intercept, c, intercept))
	return result
}

func appendHSLFilter(primitives *[]string, input string, effect drawing.HSLEffect) string {
	if effect.Hue != 0 {
		result := fmt.Sprintf("fx%d", len(*primitives)+1)
		*primitives = append(*primitives, fmt.Sprintf(`<feColorMatrix in="%s" type="hueRotate" values="%.6f" result="%s"/>`, input, float64(effect.Hue)/60000.0, result))
		input = result
	}
	if effect.Sat != 0 {
		result := fmt.Sprintf("fx%d", len(*primitives)+1)
		*primitives = append(*primitives, fmt.Sprintf(`<feColorMatrix in="%s" type="saturate" values="%.6f" result="%s"/>`, input, clampFloat64(float64(effect.Sat)/100000.0, 0, 4), result))
		input = result
	}
	if effect.Lum != 0 {
		input = appendLuminanceFilter(primitives, input, effect.Lum-100000, 0)
	}
	return input
}

func appendAlphaLinearFilter(primitives *[]string, input string, slope, intercept float64) string {
	result := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feComponentTransfer in="%s" result="%s"><feFuncA type="linear" slope="%.6f" intercept="%.6f"/></feComponentTransfer>`, input, result, slope, intercept))
	return result
}

func appendAlphaInvertFilter(primitives *[]string, input string) string {
	result := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feComponentTransfer in="%s" result="%s"><feFuncA type="table" tableValues="1 0"/></feComponentTransfer>`, input, result))
	return result
}

func alphaDiscreteTable(threshold float64, highAbove bool) string {
	threshold = clampFloat64(threshold, 0, 1)
	const n = 256
	values := make([]string, n)
	for i := 0; i < n; i++ {
		v := float64(i) / float64(n-1)
		on := v >= threshold
		if !highAbove {
			on = v > 0
		}
		if on {
			values[i] = "1"
		} else {
			values[i] = "0"
		}
	}
	return strings.Join(values, " ")
}

func appendAlphaThresholdFilter(primitives *[]string, input string, threshold float64) string {
	result := fmt.Sprintf("fx%d", len(*primitives)+1)
	table := alphaDiscreteTable(threshold, true)
	*primitives = append(*primitives, fmt.Sprintf(`<feComponentTransfer in="%s" result="%s"><feFuncA type="discrete" tableValues="%s"/></feComponentTransfer>`, input, result, table))
	return result
}

func appendAlphaCeilingFilter(primitives *[]string, input string) string {
	result := fmt.Sprintf("fx%d", len(*primitives)+1)
	values := make([]string, 256)
	values[0] = "0"
	for i := 1; i < len(values); i++ {
		values[i] = "1"
	}
	*primitives = append(*primitives, fmt.Sprintf(`<feComponentTransfer in="%s" result="%s"><feFuncA type="discrete" tableValues="%s"/></feComponentTransfer>`, input, result, strings.Join(values, " ")))
	return result
}

func appendAlphaFloorFilter(primitives *[]string, input string) string {
	result := fmt.Sprintf("fx%d", len(*primitives)+1)
	values := make([]string, 256)
	for i := range values {
		values[i] = "0"
	}
	values[len(values)-1] = "1"
	*primitives = append(*primitives, fmt.Sprintf(`<feComponentTransfer in="%s" result="%s"><feFuncA type="discrete" tableValues="%s"/></feComponentTransfer>`, input, result, strings.Join(values, " ")))
	return result
}

func appendAlphaOutsetFilter(primitives *[]string, input string, radius float64) string {
	if radius == 0 {
		return input
	}
	ceil := appendAlphaCeilingFilter(primitives, input)
	result := fmt.Sprintf("fx%d", len(*primitives)+1)
	op := "dilate"
	r := radius
	if r < 0 {
		op, r = "erode", -r
	}
	*primitives = append(*primitives, fmt.Sprintf(`<feMorphology in="%s" operator="%s" radius="%.3f" result="%s"/>`, ceil, op, r, result))
	return result
}

func appendColorChangeFilter(primitives *[]string, input string, effect drawing.ClrChange) string {
	fr, fg, fb, okFrom := hexRGB01(effect.From)
	tr, tg, tb, okTo := hexRGB01(effect.To)
	if !okFrom || !okTo {
		return input
	}
	key := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feFlood flood-color="rgb(%d,%d,%d)" flood-opacity="1" result="%s"/>`, int(fr*255+.5), int(fg*255+.5), int(fb*255+.5), key))
	diff := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feBlend in="%s" in2="%s" mode="difference" result="%s"/>`, input, key, diff))
	metric := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feColorMatrix in="%s" type="matrix" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 .333333 .333333 .333333 0 0" result="%s"/>`, diff, metric))
	mask := fmt.Sprintf("fx%d", len(*primitives)+1)
	vals := make([]string, 256)
	vals[0] = "1"
	for i := 1; i < len(vals); i++ {
		vals[i] = "0"
	}
	*primitives = append(*primitives, fmt.Sprintf(`<feComponentTransfer in="%s" result="%s"><feFuncA type="discrete" tableValues="%s"/></feComponentTransfer>`, metric, mask, strings.Join(vals, " ")))
	target := fmt.Sprintf("fx%d", len(*primitives)+1)
	alpha := 1.0
	if effect.UseAlpha && effect.ToAlpha >= 0 && effect.ToAlpha <= 1 {
		alpha = effect.ToAlpha
	}
	*primitives = append(*primitives, fmt.Sprintf(`<feFlood flood-color="rgb(%d,%d,%d)" flood-opacity="%.6f" result="%s"/>`, int(tr*255+.5), int(tg*255+.5), int(tb*255+.5), alpha, target))
	repl := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feComposite in="%s" in2="%s" operator="in" result="%s"/>`, target, mask, repl))
	rest := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feComposite in="%s" in2="%s" operator="out" result="%s"/>`, input, mask, rest))
	result := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feMerge result="%s"><feMergeNode in="%s"/><feMergeNode in="%s"/></feMerge>`, result, rest, repl))
	return result
}

func appendColorReplacementFilter(primitives *[]string, input, color string) string {
	if _, _, _, ok := hexRGB01(color); !ok {
		return input
	}
	flood := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feFlood flood-color="#%s" result="%s"/>`, strings.TrimPrefix(color, "#"), flood))
	result := fmt.Sprintf("fx%d", len(*primitives)+1)
	*primitives = append(*primitives, fmt.Sprintf(`<feComposite in="%s" in2="%s" operator="in" result="%s"/>`, flood, input, result))
	return result
}

func hexRGB01(color string) (float64, float64, float64, bool) {
	color = strings.TrimPrefix(strings.TrimSpace(color), "#")
	if len(color) != 6 {
		return 0, 0, 0, false
	}
	value, err := strconv.ParseUint(color, 16, 24)
	if err != nil {
		return 0, 0, 0, false
	}
	return float64((value>>16)&0xff) / 255.0, float64((value>>8)&0xff) / 255.0, float64(value&0xff) / 255.0, true
}

func clampFloat64(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// collectStrokeFillDefs scans primitives for gradient/pattern stroke fills and
// assigns each a unique SVG def ID.  The returned defs are emitted before the
// primitives so the stroke can reference them.
func collectStrokeFillDefs(primitives []drawing.VectorPrimitive, prefix string) []string {
	var defs []string
	for i := range primitives {
		if primitives[i].Style.Stroke != nil && primitives[i].Style.Stroke.Fill != nil && drawingcss.NeedsSVGDefinition(primitives[i].Style.Stroke.Fill) {
			id := fmt.Sprintf("%s-stroke-%d", prefix, len(defs))
			primitives[i].Style.StrokeFillDefID = id
			defs = append(defs, svgDrawingFillDefinition(primitives[i].Style.Stroke.Fill, id))
		}
	}
	return defs
}

func rectShapeStyles(shape *resolve.Shape) []string {
	var styles []string
	styles = append(styles,
		"position:absolute",
		"inset:0",
	)
	if shape.FillColor != "" {
		styles = append(styles, cssBackgroundFillPropsForFill(shape.Fill, shape.FillColor)...)
	}
	if shape.LineColor != "" && shape.LineWidthPt > 0 {
		styles = append(styles, fmt.Sprintf("border:%.2fpt %s #%s", shape.LineWidthPt, cssBorderStyleForLine(shape.LineStyle), shape.LineColor))
	}
	return styles
}

// cssBorderStyleForLine maps a DrawingML line style to a CSS border style.
// Compound lines are approximated: "dbl" becomes "double"; other compound
// variants fall back to "double" as well because CSS does not distinguish
// thick-thin or triple lines.
func cssBorderStyleForLine(style *drawing.LineStyle) string {
	if style != nil && style.CompoundType != "" && style.CompoundType != "sng" {
		return "double"
	}
	if style != nil && style.Dash != drawing.DashPatternSolid {
		return "dashed"
	}
	return "solid"
}

func svgFillValue(shape *resolve.Shape) string {
	if shape == nil {
		return "none"
	}
	if shape.Fill != nil {
		if fill, ok := drawingcss.Fill(shape.Fill)["fill"]; ok && fill != "" {
			return fill
		}
	}
	if shape.FillColor != "" {
		return "#" + shape.FillColor
	}
	return "none"
}

func shapeHasVisibleFill(shape *resolve.Shape) bool {
	if shape == nil {
		return false
	}
	if shape.FillColor != "" {
		return true
	}
	switch shape.Fill.(type) {
	case nil, drawing.NoFill, *drawing.NoFill:
		return false
	default:
		return true
	}
}

func cssBackgroundFillValue(shape *resolve.Shape) string {
	if shape == nil {
		return ""
	}
	return strings.Join(cssBackgroundFillPropsForFill(shape.Fill, shape.FillColor), ";")
}

func effectStyleProps(shape *resolve.Shape) map[string]string {
	if shape == nil || len(shape.Effects.Effects) == 0 {
		return nil
	}
	return presentationEffectStyles(shape.Effects)
}

func presentationEffectStyles(effects drawing.EffectList) map[string]string {
	if len(effects.Effects) == 0 {
		return nil
	}
	props := drawingcss.EffectList(effects)
	if props == nil {
		props = make(map[string]string)
	}
	var transforms []string
	for _, effect := range effects.Effects {
		switch e := effect.(type) {
		case drawing.RelativeOffset:
			transforms = append(transforms, relativeOffsetTransform(e))
		case *drawing.RelativeOffset:
			if e != nil {
				transforms = append(transforms, relativeOffsetTransform(*e))
			}
		case drawing.TransformEffect:
			transforms = append(transforms, drawingTransformCSS(e))
		case *drawing.TransformEffect:
			if e != nil {
				transforms = append(transforms, drawingTransformCSS(*e))
			}
		case drawing.BlendMode:
			if mode := cssBlendMode(e.Mode); mode != "" {
				props["mix-blend-mode"] = mode
			}
		case *drawing.BlendMode:
			if e != nil {
				if mode := cssBlendMode(e.Mode); mode != "" {
					props["mix-blend-mode"] = mode
				}
			}
		case drawing.Reflection:
			applyReflectionCSS(props, e)
		case *drawing.Reflection:
			if e != nil {
				applyReflectionCSS(props, *e)
			}
		case drawing.ThreeD:
			applyThreeDCSS(props, e)
		case *drawing.ThreeD:
			if e != nil {
				applyThreeDCSS(props, *e)
			}
		}
	}
	if len(transforms) > 0 {
		base := strings.TrimSpace(props["transform"])
		parts := make([]string, 0, len(transforms)+1)
		if base != "" {
			parts = append(parts, base)
		}
		for _, t := range transforms {
			if strings.TrimSpace(t) != "" {
				parts = append(parts, t)
			}
		}
		props["transform"] = strings.Join(parts, " ")
		if strings.TrimSpace(props["transform-origin"]) == "" {
			props["transform-origin"] = "50% 50%"
		}
	}
	return props
}

func relativeOffsetTransform(e drawing.RelativeOffset) string {
	if e.X == 0 && e.Y == 0 {
		return ""
	}
	return fmt.Sprintf("translate(%.4f%%,%.4f%%)", e.X*100, e.Y*100)
}

func drawingTransformCSS(e drawing.TransformEffect) string {
	var parts []string
	if e.TranslateX != 0 || e.TranslateY != 0 {
		parts = append(parts, fmt.Sprintf("translate(%.4f%%,%.4f%%)", e.TranslateX*100, e.TranslateY*100))
	}
	sx, sy := e.ScaleX, e.ScaleY
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	if sx != 1 || sy != 1 {
		parts = append(parts, fmt.Sprintf("scale(%.4f,%.4f)", sx, sy))
	}
	if e.SkewX != 0 {
		parts = append(parts, fmt.Sprintf("skewX(%.4fdeg)", e.SkewX))
	}
	if e.SkewY != 0 {
		parts = append(parts, fmt.Sprintf("skewY(%.4fdeg)", e.SkewY))
	}
	return strings.Join(parts, " ")
}

func cssBlendMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "mult", "multiply":
		return "multiply"
	case "screen":
		return "screen"
	case "darken":
		return "darken"
	case "lighten":
		return "lighten"
	case "over", "normal", "":
		return "normal"
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func applyThreeDCSS(props map[string]string, t drawing.ThreeD) {
	if props == nil {
		return
	}
	perspective := 960.0
	if t.CameraFOV > 0 && t.CameraFOV < 179 {
		// Approximate the camera focal distance against a 720px reference viewport.
		perspective = 360.0 / math.Tan(t.CameraFOV*math.Pi/360.0)
		if perspective < 160 {
			perspective = 160
		}
		if perspective > 4000 {
			perspective = 4000
		}
	}
	zoom := t.CameraZoom
	if zoom <= 0 {
		zoom = 1
	}
	var transforms []string
	base := strings.TrimSpace(props["transform"])
	if base != "" {
		transforms = append(transforms, base)
	}
	if !strings.Contains(base, "perspective(") {
		transforms = append(transforms, fmt.Sprintf("perspective(%.0fpx)", perspective))
	}
	if t.RotationX != 0 {
		transforms = append(transforms, fmt.Sprintf("rotateX(%.4fdeg)", t.RotationX))
	}
	if t.RotationY != 0 {
		transforms = append(transforms, fmt.Sprintf("rotateY(%.4fdeg)", -t.RotationY))
	}
	if t.RotationZ != 0 {
		transforms = append(transforms, fmt.Sprintf("rotateZ(%.4fdeg)", t.RotationZ))
	}
	if zoom != 1 {
		transforms = append(transforms, fmt.Sprintf("scale(%.4f)", zoom))
	}
	if len(transforms) > 0 {
		props["transform"] = strings.Join(transforms, " ")
		props["transform-origin"] = "50% 50%"
		props["transform-style"] = "preserve-3d"
	}
	if t.Material != "" {
		props["--ppt-3d-material"] = t.Material
	}
	if t.CameraPreset != "" {
		props["--ppt-3d-camera"] = t.CameraPreset
	}
	if t.LightRig != "" {
		props["--ppt-3d-light-rig"] = t.LightRig
	}
	if t.LightDirection != "" {
		props["--ppt-3d-light-dir"] = t.LightDirection
	}
}

func applyReflectionCSS(props map[string]string, r drawing.Reflection) {
	if props == nil {
		return
	}
	direction := "below"
	a := math.Mod(r.Direction, 360)
	if a < 0 {
		a += 360
	}
	switch {
	case a >= 45 && a < 135:
		direction = "right"
	case a >= 135 && a < 225:
		direction = "above"
	case a >= 225 && a < 315:
		direction = "left"
	}
	startAlpha := r.Alpha
	if r.StartAlpha > 0 {
		startAlpha = float64(r.StartAlpha) / 100000.0
	}
	if startAlpha <= 0 {
		startAlpha = 0.3
	}
	endAlpha := 0.0
	if r.EndAlpha > 0 {
		endAlpha = float64(r.EndAlpha) / 100000.0
	}
	startPos := float64(r.StartPosition) / 1000.0
	endPos := float64(r.EndPosition) / 1000.0
	if endPos <= startPos {
		startPos, endPos = 0, 100
	}
	mask := fmt.Sprintf("linear-gradient(to bottom,rgba(0,0,0,%.4f) %.2f%%,rgba(0,0,0,%.4f) %.2f%%)", startAlpha, startPos, endAlpha, endPos)
	props["-webkit-box-reflect"] = fmt.Sprintf("%s %.2fpt %s", direction, r.Distance, mask)
}

func fillOverlayEffects(effects drawing.EffectList) []drawing.FillOverlay {
	var overlays []drawing.FillOverlay
	for _, effect := range effects.Effects {
		switch e := effect.(type) {
		case drawing.FillOverlay:
			overlays = append(overlays, e)
		case *drawing.FillOverlay:
			if e != nil {
				overlays = append(overlays, *e)
			}
		}
	}
	return overlays
}

func writeHTMLFillOverlays(b *strings.Builder, effects drawing.EffectList) {
	if b == nil {
		return
	}
	for _, overlay := range fillOverlayEffects(effects) {
		if overlay.Fill == nil {
			continue
		}
		styles := []string{"position:absolute", "inset:0", "pointer-events:none", "z-index:2"}
		styles = append(styles, cssBackgroundFillPropsForFill(overlay.Fill, "")...)
		if mode := cssBlendMode(overlay.BlendMode); mode != "" && mode != "normal" {
			styles = append(styles, "mix-blend-mode:"+mode)
		}
		fmt.Fprintf(b, "<div class=\"shape__effect-overlay\" style=%q aria-hidden=\"true\"></div>\n", strings.Join(styles, ";"))
	}
}

func cssBackgroundFillPropsForFill(fill drawing.Fill, fillColor string) []string {
	if fill != nil {
		if css := drawingcss.Fill(fill); css != nil {
			if img := strings.TrimSpace(css["background-image"]); img != "" {
				var props []string
				props = append(props, "background-image:"+img)
				if r := css["background-repeat"]; r != "" {
					props = append(props, "background-repeat:"+r)
				}
				if s := css["background-size"]; s != "" {
					props = append(props, "background-size:"+s)
				}
				if p := css["background-position"]; p != "" {
					props = append(props, "background-position:"+p)
				}
				if op := css["opacity"]; op != "" {
					props = append(props, "opacity:"+op)
				}
				if len(props) > 0 {
					return props
				}
			}
			if value := strings.TrimSpace(css["fill"]); value != "" && value != "none" {
				return []string{"background:" + value}
			}
		}
	}
	if fillColor != "" {
		return []string{"background:#" + fillColor}
	}
	return nil
}

func effectStylePropsForList(effects drawing.EffectList) map[string]string {
	return presentationEffectStyles(effects)
}

func mergeEffectTransform(base string, props map[string]string) (string, string) {
	if len(props) == 0 {
		return base, ""
	}
	effectTransform := strings.TrimSpace(props["transform"])
	delete(props, "transform")
	origin := strings.TrimSpace(props["transform-origin"])
	delete(props, "transform-origin")
	if effectTransform == "" {
		return base, origin
	}
	if base == "" {
		return effectTransform, origin
	}
	return strings.TrimSpace(base + " " + effectTransform), origin
}

func svgPathData(path drawing.Path) string {
	var parts []string
	for _, cmd := range path.Commands {
		switch cmd.Type {
		case "M", "L":
			if len(cmd.Points) == 0 {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s %.4f %.4f", cmd.Type, cmd.Points[0].X, cmd.Points[0].Y))
		case "C":
			if len(cmd.Points) < 3 {
				continue
			}
			parts = append(parts, fmt.Sprintf("C %.4f %.4f %.4f %.4f %.4f %.4f",
				cmd.Points[0].X, cmd.Points[0].Y,
				cmd.Points[1].X, cmd.Points[1].Y,
				cmd.Points[2].X, cmd.Points[2].Y,
			))
		case "Q":
			if len(cmd.Points) < 2 {
				continue
			}
			parts = append(parts, fmt.Sprintf("Q %.4f %.4f %.4f %.4f",
				cmd.Points[0].X, cmd.Points[0].Y,
				cmd.Points[1].X, cmd.Points[1].Y,
			))
		case "A":
			if len(cmd.Points) == 0 || len(cmd.Args) < 5 {
				continue
			}
			parts = append(parts, fmt.Sprintf("A %.4f %.4f %.4f %.0f %.0f %.4f %.4f",
				cmd.Args[0], cmd.Args[1], cmd.Args[2], cmd.Args[3], cmd.Args[4], cmd.Points[0].X, cmd.Points[0].Y,
			))
		case "Z":
			parts = append(parts, "Z")
		}
	}
	return strings.Join(parts, " ")
}

func renderImageShape(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	x, y, w, h := boundsPercent(shape.Bounds, slideW, slideH)
	effectStyles := effectStyleProps(shape)
	if len(shape.ImageData) > 0 {
		if !browserRenderablePresentationImage(imageContentType(shape)) {
			var styles []string
			styles = append(styles,
				"position:absolute",
				fmt.Sprintf("left:%.4f%%", x),
				fmt.Sprintf("top:%.4f%%", y),
				fmt.Sprintf("width:%.4f%%", w),
				fmt.Sprintf("height:%.4f%%", h),
				"display:flex",
				"align-items:center",
				"justify-content:center",
				"background:rgba(245, 239, 222, 0.92)",
				"border:1px solid #9a8855",
				"color:#6b5c34",
				"font-size:10px",
				"text-align:center",
				"padding:4px",
			)
			if len(effectStyles) > 0 {
				styles = append(styles, "overflow:visible")
				transform, origin := mergeEffectTransform("", effectStyles)
				if transform != "" {
					styles = append(styles, "transform:"+transform)
				}
				if origin != "" {
					styles = append(styles, "transform-origin:"+origin)
				}
				styles = appendSortedCSSProps(styles, effectStyles)
			}
			fmt.Fprintf(b,
				"<div class=\"shape shape--image shape--image-placeholder\" style=%q%s%s%s%s aria-label=%q>[%s]</div>\n",
				strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape), escapeAttr(shape.Name), escapeHTML(imageContentType(shape)))
			return
		}
		src := imageSource(ctx, shape)
		var styles []string
		styles = append(styles,
			"position:absolute",
			fmt.Sprintf("left:%.4f%%", x),
			fmt.Sprintf("top:%.4f%%", y),
			fmt.Sprintf("width:%.4f%%", w),
			fmt.Sprintf("height:%.4f%%", h),
		)
		if len(effectStyles) > 0 {
			styles = append(styles, "overflow:visible")
		}
		// Apply rotation and flip transforms
		var transforms []string
		if shape.RotationDeg != 0 {
			transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", shape.RotationDeg))
		}
		if shape.FlipH {
			transforms = append(transforms, "scaleX(-1)")
		}
		if shape.FlipV {
			transforms = append(transforms, "scaleY(-1)")
		}
		transform, effectOrigin := mergeEffectTransform(strings.Join(transforms, " "), effectStyles)
		if transform != "" {
			styles = append(styles, fmt.Sprintf("transform:%s", transform))
			if effectOrigin != "" {
				styles = append(styles, "transform-origin:"+effectOrigin)
			} else {
				styles = append(styles, "transform-origin:50% 50%")
			}
		} else if effectOrigin != "" {
			styles = append(styles, "transform-origin:"+effectOrigin)
		}
		if len(effectStyles) > 0 {
			styles = appendSortedCSSProps(styles, effectStyles)
		}
		frameStyles := imageFrameStyles(shape)
		imgStyles := imageElementStyles(shape)
		imageFilterID := fmt.Sprintf("picture-%d-blip-filter", shape.ID)
		if ctx != nil {
			ctx.nextEffectIndex++
			imageFilterID = fmt.Sprintf("picture-blip-filter-%d", ctx.nextEffectIndex)
		}
		imageFilterDef := imageEffectFilterDefinition(shape.ImageEffects, imageFilterID)
		if imageFilterDef != "" {
			imgStyles = append(imgStyles, "filter:url(#"+imageFilterID+")")
		}
		imageEffectsAttr := ""
		if names := imageEffectNames(shape.ImageEffects); names != "" {
			imageEffectsAttr = fmt.Sprintf(" data-ppt-image-effects=%q", names)
		}
		fmt.Fprintf(b, "<div class=\"shape shape--image\" style=%q%s%s%s%s>", strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape))
		if imageFilterDef != "" {
			fmt.Fprintf(b, "<svg width=\"0\" height=\"0\" aria-hidden=\"true\"><defs>%s</defs></svg>", imageFilterDef)
		}
		fmt.Fprintf(b, "<div class=\"shape__image-frame\" style=%q>", strings.Join(frameStyles, ";"))
		geometryAttr := fmt.Sprintf(" data-ppt-image-dpi=%q data-ppt-rot-with-shape=%q data-ppt-fill-rect=%q", strconv.Itoa(shape.ImageDPI), strconv.FormatBool(!shape.ImageRotWithShapeSet || shape.ImageRotWithShape), fmt.Sprintf("%.5f,%.5f,%.5f,%.5f", shape.ImageFillLeft, shape.ImageFillTop, shape.ImageFillRight, shape.ImageFillBottom))
		fmt.Fprintf(b,
			"<img class=\"shape__image\" style=%q src=%q alt=%q%s%s>",
			strings.Join(imgStyles, ";"), src, escapeAttr(shape.Name), imageEffectsAttr, geometryAttr)
		b.WriteString("</div>")
		writeHTMLFillOverlays(b, shape.Effects)
		b.WriteString("</div>\n")
	} else {
		var styles []string
		styles = append(styles,
			"position:absolute",
			fmt.Sprintf("left:%.4f%%", x),
			fmt.Sprintf("top:%.4f%%", y),
			fmt.Sprintf("width:%.4f%%", w),
			fmt.Sprintf("height:%.4f%%", h),
		)
		if len(effectStyles) > 0 {
			styles = append(styles, "overflow:visible")
			transform, origin := mergeEffectTransform("", effectStyles)
			if transform != "" {
				styles = append(styles, "transform:"+transform)
			}
			if origin != "" {
				styles = append(styles, "transform-origin:"+origin)
			}
			styles = appendSortedCSSProps(styles, effectStyles)
		}
		fmt.Fprintf(b,
			"<div class=\"shape shape--image\" style=%q%s%s%s%s aria-label=%q></div>\n",
			strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape), escapeAttr(shape.Name))
	}
}

func renderMediaShape(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	x, y, w, h := boundsPercent(shape.Bounds, slideW, slideH)
	effectStyles := effectStyleProps(shape)
	var styles []string
	styles = append(styles,
		"position:absolute",
		fmt.Sprintf("left:%.4f%%", x),
		fmt.Sprintf("top:%.4f%%", y),
		fmt.Sprintf("width:%.4f%%", w),
		fmt.Sprintf("height:%.4f%%", h),
		"display:flex",
		"align-items:center",
		"justify-content:center",
		"background:#111",
	)
	if len(effectStyles) > 0 {
		styles = append(styles, "overflow:visible")
	}
	var transforms []string
	if shape.RotationDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", shape.RotationDeg))
	}
	if shape.FlipH {
		transforms = append(transforms, "scaleX(-1)")
	}
	if shape.FlipV {
		transforms = append(transforms, "scaleY(-1)")
	}
	transform, effectOrigin := mergeEffectTransform(strings.Join(transforms, " "), effectStyles)
	if transform != "" {
		styles = append(styles, "transform:"+transform)
		if effectOrigin != "" {
			styles = append(styles, "transform-origin:"+effectOrigin)
		} else {
			styles = append(styles, "transform-origin:50% 50%")
		}
	} else if effectOrigin != "" {
		styles = append(styles, "transform-origin:"+effectOrigin)
	}
	if len(effectStyles) > 0 {
		styles = appendSortedCSSProps(styles, effectStyles)
	}

	label := shape.MediaKind
	if label == "" {
		label = "media"
	}
	if (len(shape.MediaData) == 0 && strings.TrimSpace(shape.MediaLinkURL) == "") || !browserRenderableMedia(shape.MediaMIME) {
		styles = append(styles,
			"border:1px solid #555",
			"color:#f2f2f2",
			"font-size:11px",
			"text-align:center",
			"padding:6px",
		)
		if len(shape.MediaPosterData) > 0 && browserRenderablePresentationImage(shape.MediaPosterMIME) {
			styles = append(styles, "background-image:url("+mediaPosterSource(ctx, shape)+")")
			styles = append(styles, "background-size:cover", "background-position:center")
		}
		fmt.Fprintf(b, "<div class=\"shape shape--media shape--media-placeholder\" style=%q%s%s%s%s aria-label=%q>[%s]</div>\n",
			strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape), escapeAttr(shape.Name), escapeHTML(mediaLabel(shape, label)))
		return
	}

	fmt.Fprintf(b, "<div class=\"shape shape--media\" style=%q%s%s%s%s>", strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape))
	src := mediaSource(ctx, shape)
	switch mediaTagName(shape) {
	case "audio":
		audioStyles := []string{"width:100%", "max-width:100%", "position:relative", "z-index:1"}
		fmt.Fprintf(b, "<audio controls preload=\"metadata\" style=%q src=%q aria-label=%q></audio>",
			strings.Join(audioStyles, ";"), src, escapeAttr(shape.Name))
	default:
		videoStyles := []string{"position:absolute", "inset:0", "width:100%", "height:100%", "object-fit:fill", "display:block"}
		fmt.Fprintf(b, "<video controls preload=\"metadata\" style=%q src=%q", strings.Join(videoStyles, ";"), src)
		if poster := mediaPosterSource(ctx, shape); poster != "" {
			fmt.Fprintf(b, " poster=%q", poster)
		}
		fmt.Fprintf(b, " aria-label=%q></video>", escapeAttr(shape.Name))
	}
	b.WriteString("</div>\n")
}

func renderOleObjectShape(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	x, y, w, h := boundsPercent(shape.Bounds, slideW, slideH)
	effectStyles := effectStyleProps(shape)

	baseStyles := []string{
		"position:absolute",
		fmt.Sprintf("left:%.4f%%", x),
		fmt.Sprintf("top:%.4f%%", y),
		fmt.Sprintf("width:%.4f%%", w),
		fmt.Sprintf("height:%.4f%%", h),
	}
	if len(shape.OLEIconData) > 0 && browserRenderablePresentationImage(shape.OLEIconMIME) {
		if len(effectStyles) > 0 {
			baseStyles = append(baseStyles, "overflow:visible")
		}
		var transforms []string
		if shape.RotationDeg != 0 {
			transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", shape.RotationDeg))
		}
		if shape.FlipH {
			transforms = append(transforms, "scaleX(-1)")
		}
		if shape.FlipV {
			transforms = append(transforms, "scaleY(-1)")
		}
		transform, effectOrigin := mergeEffectTransform(strings.Join(transforms, " "), effectStyles)
		if transform != "" {
			baseStyles = append(baseStyles, "transform:"+transform)
			if effectOrigin != "" {
				baseStyles = append(baseStyles, "transform-origin:"+effectOrigin)
			} else {
				baseStyles = append(baseStyles, "transform-origin:50% 50%")
			}
		} else if effectOrigin != "" {
			baseStyles = append(baseStyles, "transform-origin:"+effectOrigin)
		}
		if len(effectStyles) > 0 {
			baseStyles = appendSortedCSSProps(baseStyles, effectStyles)
		}
		src := oleObjectIconSource(ctx, shape)
		if shape.OLEObjectLinkURL != "" {
			fmt.Fprintf(b, "<a class=\"ole-object-link\" href=%q target=\"_blank\" rel=\"noopener noreferrer\">", escapeAttr(shape.OLEObjectLinkURL))
		}
		fmt.Fprintf(b, "<img class=\"shape shape--ole-object\" style=%q src=%q alt=%q%s%s%s%s>\n",
			strings.Join(baseStyles, ";"), src, escapeAttr(shape.Name), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape))
		if shape.OLEObjectLinkURL != "" {
			b.WriteString("</a>\n")
		}
		return
	}

	styles := append([]string{}, baseStyles...)
	styles = append(styles,
		"display:flex",
		"align-items:center",
		"justify-content:center",
		"background:rgba(245, 239, 222, 0.92)",
		"border:1px solid #9a8855",
		"color:#6b5c34",
		"font-size:10px",
		"text-align:center",
		"padding:4px",
	)
	if len(effectStyles) > 0 {
		styles = append(styles, "overflow:visible")
		transform, origin := mergeEffectTransform("", effectStyles)
		if transform != "" {
			styles = append(styles, "transform:"+transform)
		}
		if origin != "" {
			styles = append(styles, "transform-origin:"+origin)
		}
		styles = appendSortedCSSProps(styles, effectStyles)
	}
	label := shape.OLEObjectName
	if label == "" {
		label = shape.OLEObjectProgID
	}
	if label == "" {
		label = "Object"
	}
	if shape.OLEObjectLinkURL != "" {
		fmt.Fprintf(b, "<a class=\"ole-object-link\" href=%q target=\"_blank\" rel=\"noopener noreferrer\">", escapeAttr(shape.OLEObjectLinkURL))
	}
	fmt.Fprintf(b,
		"<div class=\"shape shape--ole-object\" style=%q%s%s%s%s aria-label=%q>[%s]</div>\n",
		strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape), escapeAttr(shape.Name), escapeHTML(label))
	if shape.OLEObjectLinkURL != "" {
		b.WriteString("</a>\n")
	}
}

func oleObjectIconSource(ctx *htmlRenderContext, shape *resolve.Shape) string {
	mime := shape.OLEIconMIME
	if mime == "" {
		mime = "image/jpeg"
	}
	if ctx != nil && ctx.options.ImageMode == HTMLImageModeExternal {
		ctx.nextImageIndex++
		assetPath := htmlasset.AssetPath(ctx.options.ExternalImageDir, "ole-icon", ctx.nextImageIndex, htmlasset.ImageFileExtension(mime))
		ctx.assets = append(ctx.assets, HTMLAsset{
			Path:        assetPath,
			ContentType: mime,
			Data:        append([]byte(nil), shape.OLEIconData...),
		})
		return assetPath
	}
	encoded := base64.StdEncoding.EncodeToString(shape.OLEIconData)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded)
}

func imageSource(ctx *htmlRenderContext, shape *resolve.Shape) string {
	mime := imageContentType(shape)
	if ctx != nil && ctx.options.ImageMode == HTMLImageModeExternal {
		return ctx.addImageAsset(shape, mime)
	}
	encoded := base64.StdEncoding.EncodeToString(shape.ImageData)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded)
}

func mediaSource(ctx *htmlRenderContext, shape *resolve.Shape) string {
	if shape != nil && strings.TrimSpace(shape.MediaLinkURL) != "" {
		return shape.MediaLinkURL
	}
	mime := mediaContentType(shape)
	if ctx != nil && ctx.options.ImageMode == HTMLImageModeExternal {
		return ctx.addMediaAsset(shape, mime)
	}
	encoded := base64.StdEncoding.EncodeToString(shape.MediaData)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded)
}

func mediaPosterSource(ctx *htmlRenderContext, shape *resolve.Shape) string {
	if shape == nil || len(shape.MediaPosterData) == 0 {
		return ""
	}
	if ctx != nil && ctx.options.ImageMode == HTMLImageModeExternal {
		return ctx.addMediaPosterAsset(shape, mediaPosterContentType(shape))
	}
	encoded := base64.StdEncoding.EncodeToString(shape.MediaPosterData)
	return fmt.Sprintf("data:%s;base64,%s", mediaPosterContentType(shape), encoded)
}

func imageContentType(shape *resolve.Shape) string {
	if shape != nil && shape.ImageMIME != "" {
		return shape.ImageMIME
	}
	return "image/jpeg"
}

func mediaContentType(shape *resolve.Shape) string {
	if shape != nil && shape.MediaMIME != "" {
		return shape.MediaMIME
	}
	return "application/octet-stream"
}

func mediaPosterContentType(shape *resolve.Shape) string {
	if shape != nil && shape.MediaPosterMIME != "" {
		return shape.MediaPosterMIME
	}
	return "image/jpeg"
}

func browserRenderablePresentationImage(contentType string) bool {
	baseType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch baseType {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/webp", "image/svg+xml", "image/bmp", "image/avif":
		return true
	default:
		return false
	}
}

func browserRenderableMedia(contentType string) bool {
	baseType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch baseType {
	case "video/mp4", "video/webm", "video/ogg", "audio/mpeg", "audio/mp3", "audio/mp4", "audio/ogg", "audio/wav", "audio/x-wav", "audio/webm":
		return true
	default:
		return false
	}
}

func (ctx *htmlRenderContext) addImageAsset(shape *resolve.Shape, mime string) string {
	ctx.nextImageIndex++
	assetPath := htmlasset.AssetPath(ctx.options.ExternalImageDir, "image", ctx.nextImageIndex, htmlasset.ImageFileExtension(mime))
	ctx.assets = append(ctx.assets, HTMLAsset{
		Path:        assetPath,
		ContentType: mime,
		Data:        append([]byte(nil), shape.ImageData...),
	})
	return assetPath
}

func (ctx *htmlRenderContext) addMediaAsset(shape *resolve.Shape, mime string) string {
	ctx.nextMediaIndex++
	assetPath := htmlasset.AssetPath(ctx.options.ExternalImageDir, "media", ctx.nextMediaIndex, htmlasset.MediaFileExtension(mime))
	ctx.assets = append(ctx.assets, HTMLAsset{
		Path:        assetPath,
		ContentType: mime,
		Data:        append([]byte(nil), shape.MediaData...),
	})
	return assetPath
}

func (ctx *htmlRenderContext) addMediaPosterAsset(shape *resolve.Shape, mime string) string {
	ctx.nextImageIndex++
	assetPath := htmlasset.AssetPath(ctx.options.ExternalImageDir, "image", ctx.nextImageIndex, htmlasset.ImageFileExtension(mime))
	ctx.assets = append(ctx.assets, HTMLAsset{
		Path:        assetPath,
		ContentType: mime,
		Data:        append([]byte(nil), shape.MediaPosterData...),
	})
	return assetPath
}

func (ctx *htmlRenderContext) addFontAsset(src fontdata.Source) string {
	ctx.nextFontIndex++
	assetPath := htmlasset.AssetPath(ctx.options.ExternalFontDir, "font", ctx.nextFontIndex, htmlasset.FontFileExtension(src.MIME, src.Format))
	ctx.assets = append(ctx.assets, HTMLAsset{
		Path:        assetPath,
		ContentType: src.MIME,
		Data:        append([]byte(nil), src.Data...),
	})
	return assetPath
}

func imageFrameStyles(shape *resolve.Shape) []string {
	styles := []string{"position:absolute", "overflow:hidden"}
	if shape == nil {
		return append(styles, "inset:0")
	}
	left, top := shape.ImageFillLeft, shape.ImageFillTop
	right, bottom := shape.ImageFillRight, shape.ImageFillBottom
	if left+right >= 1 || top+bottom >= 1 {
		return append(styles, "display:none")
	}
	styles = append(styles,
		fmt.Sprintf("left:%.4f%%", left*100),
		fmt.Sprintf("top:%.4f%%", top*100),
		fmt.Sprintf("right:%.4f%%", right*100),
		fmt.Sprintf("bottom:%.4f%%", bottom*100),
	)
	return styles
}

func imageElementStyles(shape *resolve.Shape) []string {
	styles := []string{
		"position:absolute",
		"display:block",
	}
	if shape == nil {
		return styles
	}
	visibleWidth := 1.0 - shape.ImageCropLeft - shape.ImageCropRight
	visibleHeight := 1.0 - shape.ImageCropTop - shape.ImageCropBottom
	if visibleWidth <= 0 || visibleHeight <= 0 {
		return append(styles, "visibility:hidden")
	}
	if visibleWidth > 1 {
		visibleWidth = 1
	}
	if visibleHeight > 1 {
		visibleHeight = 1
	}
	styles = append(styles,
		fmt.Sprintf("width:%.4f%%", 100.0/visibleWidth),
		fmt.Sprintf("height:%.4f%%", 100.0/visibleHeight),
		fmt.Sprintf("left:%.4f%%", -100.0*shape.ImageCropLeft/visibleWidth),
		fmt.Sprintf("top:%.4f%%", -100.0*shape.ImageCropTop/visibleHeight),
	)
	if shape.ImageRotWithShapeSet && !shape.ImageRotWithShape && shape.RotationDeg != 0 {
		styles = append(styles, fmt.Sprintf("transform:rotate(%.2fdeg)", -shape.RotationDeg), "transform-origin:50% 50%")
	}
	return styles
}

func mediaTagName(shape *resolve.Shape) string {
	if shape != nil && shape.MediaKind == "audio" {
		return "audio"
	}
	return "video"
}

func mediaLabel(shape *resolve.Shape, fallback string) string {
	if shape != nil && shape.MediaMIME != "" {
		return shape.MediaMIME
	}
	return fallback
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// renderSmartArtShape renders a SmartArt diagram shape as HTML.
func renderSmartArtShape(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	if shape == nil {
		return
	}
	x, y, w, h := boundsPercent(shape.Bounds, slideW, slideH)
	textAutoGrow := shape.TextAutoGrow || shape.TextBodyProps.AutofitType == "spAutoFit"
	textAllowOverflow := shape.TextAllowOverflow || shape.TextBodyProps.AutofitType == "noAutofit"

	// Build CSS styles for positioning
	var styles []string
	styles = append(styles,
		"position:absolute",
		fmt.Sprintf("left:%.4f%%", x),
		fmt.Sprintf("top:%.4f%%", y),
		fmt.Sprintf("width:%.4f%%", w),
	)
	if textAutoGrow {
		styles = append(styles, fmt.Sprintf("min-height:%.4f%%", h))
	} else {
		styles = append(styles, fmt.Sprintf("height:%.4f%%", h))
	}

	effectStyles := effectStyleProps(shape)
	if len(effectStyles) > 0 {
		styles = appendSortedCSSProps(styles, effectStyles)
	}

	useSVG := shape.GeometryData != nil || (shape.Geometry != "" && shape.Geometry != "rect")
	if useSVG {
		if textAutoGrow || textAllowOverflow {
			styles = append(styles, "overflow:visible")
		} else {
			styles = append(styles, "overflow:hidden")
		}
	} else {
		// Add fill using shared DrawingML types.
		styles = append(styles, cssBackgroundFillPropsForFill(shape.Fill, shape.FillColor)...)

		// Add border/line styling.
		if shape.LineStyle != nil && shape.LineStyle.Color != "" {
			styles = append(styles, fmt.Sprintf("border:%.2fpt %s #%s", shape.LineStyle.Width, cssBorderStyleForLine(shape.LineStyle), shape.LineStyle.Color))
		} else if shape.LineColor != "" && shape.LineWidthPt > 0 {
			styles = append(styles, fmt.Sprintf("border:%.2fpt %s #%s", shape.LineWidthPt, cssBorderStyleForLine(shape.LineStyle), shape.LineColor))
		}
		if textAutoGrow || textAllowOverflow {
			styles = append(styles, "overflow:visible")
		}
	}

	// Render as a div container
	fmt.Fprintf(b, "<div class=\"shape shape--smartart\" style=%q%s>\n", strings.Join(styles, ";"), animationAttrs(shape, ctx))

	// Render non-rectangular geometry as an inline SVG.
	if useSVG {
		renderSmartArtVectorShape(b, shape, nil)
	} else {
		writeHTMLFillOverlays(b, shape.Effects)
	}

	// Render text content if present
	if len(shape.Paragraphs) > 0 {
		var textStyles []string
		textStyles = append(textStyles,
			"position:absolute",
			"display:flex",
			"flex-direction:column",
			"box-sizing:border-box",
			"width:100%",
		)
		if textAutoGrow {
			textStyles = append(textStyles, "left:0", "top:0", "right:0")
		} else {
			textStyles = append(textStyles, "inset:0")
		}
		if textAutoGrow || textAllowOverflow {
			textStyles = append(textStyles, "height:auto", "overflow:visible")
		} else {
			textStyles = append(textStyles, "height:100%", "overflow:hidden")
		}
		cssProps := drawingcss.TextBodyProps(shape.TextBodyProps)
		textStyles = appendSortedCSSProps(textStyles, cssProps)
		if _, hasJustifyContent := cssProps["justify-content"]; !hasJustifyContent {
			textStyles = append(textStyles, "justify-content:center")
		}
		if _, hasTextAlign := cssProps["text-align"]; !hasTextAlign {
			textStyles = append(textStyles, "text-align:center")
		}
		fmt.Fprintf(b, "<div style=%q>\n", strings.Join(textStyles, ";"))

		autoCounters := map[string]int{}
		fontScale := textAutofitFontScale(shape.TextBodyProps)
		lineScale := textAutofitLineScale(shape.TextBodyProps)
		for _, p := range shape.Paragraphs {
			renderParagraph(b, p, autoCounters, fontScale, lineScale, ctx)
		}
		b.WriteString("</div>\n")
	}

	b.WriteString("</div>\n")
}

// renderSmartArtVectorShape renders a SmartArt node's geometry as an inline SVG.
// It uses the shared preset-geometry primitive builder so any preset shape known
// to pkg/ooxml/drawing can be rendered, including ones with gradient or picture
// fills that need SVG definitions.
func renderSmartArtVectorShape(b *strings.Builder, shape *resolve.Shape, effectStyles map[string]string) {
	if b == nil || shape == nil {
		return
	}
	width := drawing.EMUToPoints(shape.Bounds.W)
	height := drawing.EMUToPoints(shape.Bounds.H)
	if width <= 0 || height <= 0 {
		return
	}

	spec := drawing.ShapeGeometrySpec{
		Geometry:    shape.GeometryData,
		Preset:      &drawing.PresetGeometry{Name: shape.Geometry, Adjustments: shape.GeometryAdjustments},
		Width:       width,
		Height:      height,
		Fill:        shape.Fill,
		FillColor:   shape.FillColor,
		Stroke:      shape.LineStyle,
		StrokeColor: shape.LineColor,
		StrokeWidth: shape.LineWidthPt,
	}
	primitives, err := drawing.BuildShapePrimitives(spec)
	if err != nil || len(primitives) == 0 {
		return
	}

	fillDefID := ""
	fillDef := ""
	if drawingcss.NeedsSVGDefinition(shape.Fill) {
		fillDefID = "smartart-fill"
		fillDef = svgDrawingFillDefinition(shape.Fill, fillDefID)
	}
	lightOverlays := drawingcss.ThreeDLightOverlays(extractShapeThreeD(shape.Effects))
	overlayRenders := drawingcss.BuildSVGOverlays(lightOverlays, "smartart-fill-overlay")
	fillOverlays := fillOverlayEffects(shape.Effects)
	fillOverlayDefs := make([]string, len(fillOverlays))
	fillOverlayDefIDs := make([]string, len(fillOverlays))
	for i, overlay := range fillOverlays {
		if overlay.Fill != nil && drawingcss.NeedsSVGDefinition(overlay.Fill) {
			id := fmt.Sprintf("smartart-effect-fill-%d", i)
			fillOverlayDefIDs[i] = id
			fillOverlayDefs[i] = svgDrawingFillDefinition(overlay.Fill, id)
		}
	}
	strokeDefs := collectStrokeFillDefs(primitives, "smartart")

	fmt.Fprintf(b, "<svg class=\"shape__vector\" width=\"100%%\" height=\"100%%\" viewBox=\"0 0 %.4f %.4f\" preserveAspectRatio=\"none\"", width, height)
	if len(effectStyles) > 0 {
		var styles []string
		styles = appendSortedCSSProps(styles, effectStyles)
		fmt.Fprintf(b, " style=%q", strings.Join(styles, ";"))
	}
	b.WriteString(" xmlns=\"http://www.w3.org/2000/svg\">")
	if fillDef != "" || len(strokeDefs) > 0 || len(overlayRenders) > 0 || len(fillOverlayDefs) > 0 {
		b.WriteString("<defs>")
		if fillDef != "" {
			b.WriteString(fillDef)
		}
		for _, def := range strokeDefs {
			b.WriteString(def)
		}
		for _, overlay := range overlayRenders {
			if overlay.Def != "" {
				b.WriteString(overlay.Def)
			}
		}
		for _, def := range fillOverlayDefs {
			if def != "" {
				b.WriteString(def)
			}
		}
		b.WriteString("</defs>")
	}
	writeThreeDExtrusionSVG(b, primitives, extractShapeThreeD(shape.Effects))
	for _, primitive := range primitives {
		writeSVGPrimitive(b, primitive, fillDefID)
	}
	for _, overlay := range overlayRenders {
		if overlay.Fill == nil {
			continue
		}
		if blendStyle := drawingcss.SVGOverlayBlendStyle(overlay.BlendMode); blendStyle != "" {
			fmt.Fprintf(b, "<g style=%q>", blendStyle)
		} else {
			b.WriteString("<g>")
		}
		for _, primitive := range primitives {
			writeSVGPrimitive(b, overlayShapePrimitiveFill(primitive, overlay.Fill), overlay.DefID)
		}
		b.WriteString("</g>")
	}
	for i, overlay := range fillOverlays {
		if overlay.Fill == nil {
			continue
		}
		style := ""
		if mode := cssBlendMode(overlay.BlendMode); mode != "" && mode != "normal" {
			style = "mix-blend-mode:" + mode
		}
		if style != "" {
			fmt.Fprintf(b, "<g class=\"shape__effect-overlay\" style=%q>", style)
		} else {
			b.WriteString("<g class=\"shape__effect-overlay\">")
		}
		for _, primitive := range primitives {
			writeSVGPrimitive(b, overlayShapePrimitiveFill(primitive, overlay.Fill), fillOverlayDefIDs[i])
		}
		b.WriteString("</g>")
	}
	b.WriteString("</svg>\n")
}

// renderSmartArtConnector draws a connector line between two SmartArt nodes,
// honoring line style, dash patterns, and arrowheads.
func renderSmartArtConnector(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	if shape == nil {
		return
	}
	// When the resolver supplied a DrawingML connector preset and bounds, render
	// it using the shared connector primitive builder so curved connectors are
	// emitted as SVG paths.
	if shape.ConnectorType != "" && shape.Bounds.W > 0 && shape.Bounds.H >= 0 {
		renderSmartArtConnectorWithPreset(b, shape, slideW, slideH, ctx)
		return
	}
	if shape.ConnectorStart == nil || shape.ConnectorEnd == nil {
		return
	}

	color := shape.LineColor
	if color == "" {
		color = "999999"
	}
	width := shape.LineWidthPt
	if width == 0 {
		width = 1.0
	}

	// Build the connector path in points, remembering the original endpoints
	// for arrowhead placement before any trimming.
	pts := []drawing.Point{
		{X: emuToPt(shape.ConnectorStart.X), Y: emuToPt(shape.ConnectorStart.Y)},
	}
	if len(shape.ConnectorPath) > 2 {
		for _, p := range shape.ConnectorPath[1 : len(shape.ConnectorPath)-1] {
			pts = append(pts, drawing.Point{X: emuToPt(p.X), Y: emuToPt(p.Y)})
		}
	}
	pts = append(pts, drawing.Point{X: emuToPt(shape.ConnectorEnd.X), Y: emuToPt(shape.ConnectorEnd.Y)})

	originalStart := pts[0]
	originalEnd := pts[len(pts)-1]

	// Trim the line so arrowheads don't extend past the connection points.
	if shape.ArrowHead != "" && len(pts) >= 2 {
		trim := drawing.ArrowTrimDistance(shape.ArrowHead, shape.ArrowHeadLength)
		pts = trimSmartArtConnectorTip(pts, trim, false)
	}
	if shape.ArrowTail != "" && len(pts) >= 2 {
		trim := drawing.ArrowTrimDistance(shape.ArrowTail, shape.ArrowTailLength)
		pts = trimSmartArtConnectorTip(pts, trim, true)
	}

	fmt.Fprintf(b, "<svg class=\"shape\" style=\"position:absolute;left:0;top:0;width:100%%;height:100%%;overflow:visible;pointer-events:none\" viewBox=\"0 0 %.4f %.4f\" xmlns=\"http://www.w3.org/2000/svg\"%s>", slideW, slideH, animationAttrs(shape, ctx))
	var curveStartTangent, curveEndTangent *drawing.Point
	if (shape.ConnectorType == "curved" || shape.ConnectorType == "longCurve") && len(pts) == 2 {
		cp1, cp2 := smartArtCurveControlPoints(pts[0], pts[1], shape.ConnectorType == "longCurve")
		fmt.Fprintf(b, "<path d=\"M %.4f %.4f C %.4f %.4f %.4f %.4f %.4f %.4f\" fill=\"none\"", pts[0].X, pts[0].Y, cp1.X, cp1.Y, cp2.X, cp2.Y, pts[1].X, pts[1].Y)
		writeSVGStrokeAttrs(b, shape.LineStyle, "", "#"+color, width)
		b.WriteString("/>")
		curveStartTangent, curveEndTangent = &cp1, &cp2
	} else if len(pts) > 2 {
		fmt.Fprintf(b, "<polyline points=%q fill=\"none\"", smartArtConnectorPointsString(pts))
		writeSVGStrokeAttrs(b, shape.LineStyle, "", "#"+color, width)
		b.WriteString("/>")
	} else {
		fmt.Fprintf(b, "<line x1=\"%.4f\" y1=\"%.4f\" x2=\"%.4f\" y2=\"%.4f\"", pts[0].X, pts[0].Y, pts[1].X, pts[1].Y)
		writeSVGStrokeAttrs(b, shape.LineStyle, "", "#"+color, width)
		b.WriteString("/>")
	}

	// Render arrowheads at the original endpoints. For curved synthetic
	// connectors use the Bezier control points as tangent references.
	if shape.ArrowHead != "" && len(pts) >= 2 {
		ref := pts[len(pts)-2]
		if curveEndTangent != nil {
			ref = *curveEndTangent
		}
		ag := drawing.ComputeArrowGeometry(shape.ArrowHead, shape.ArrowHeadWidth, shape.ArrowHeadLength, originalEnd, ref)
		b.WriteString(smartArtArrowGeometrySVG(ag, color))
	}
	if shape.ArrowTail != "" && len(pts) >= 2 {
		ref := pts[1]
		if curveStartTangent != nil {
			ref = *curveStartTangent
		}
		ag := drawing.ComputeArrowGeometry(shape.ArrowTail, shape.ArrowTailWidth, shape.ArrowTailLength, originalStart, ref)
		b.WriteString(smartArtArrowGeometrySVG(ag, color))
	}
	b.WriteString("</svg>\n")
}

// renderSmartArtConnectorWithPreset renders a pre-rendered SmartArt connector
// using the shared DrawingML connector primitive builder. This handles curved
// connectors as SVG paths and applies line styles, dashes, and arrowheads.
func smartArtCurveControlPoints(start, end drawing.Point, long bool) (drawing.Point, drawing.Point) {
	dx := end.X - start.X
	dy := end.Y - start.Y
	f1, f2 := 0.35, 0.65
	if long {
		f1, f2 = 0.18, 0.82
	}
	if math.Abs(dx) >= math.Abs(dy) {
		return drawing.Point{X: start.X + dx*f1, Y: start.Y}, drawing.Point{X: start.X + dx*f2, Y: end.Y}
	}
	return drawing.Point{X: start.X, Y: start.Y + dy*f1}, drawing.Point{X: end.X, Y: start.Y + dy*f2}
}

func renderSmartArtConnectorWithPreset(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	bx := emuToPt(shape.Bounds.X)
	by := emuToPt(shape.Bounds.Y)
	bw := emuToPt(shape.Bounds.W)
	bh := emuToPt(shape.Bounds.H)

	fmt.Fprintf(b, "<svg class=\"shape\" style=\"position:absolute;left:0;top:0;width:100%%;height:100%%;overflow:visible;pointer-events:none\" viewBox=\"0 0 %.4f %.4f\" xmlns=\"http://www.w3.org/2000/svg\"%s>", slideW, slideH, animationAttrs(shape, ctx))

	hasRotation := shape.RotationDeg != 0
	if hasRotation {
		cx := bx + bw/2
		cy := by + bh/2
		fmt.Fprintf(b, "<g transform=\"rotate(%.4f %.4f %.4f)\">", shape.RotationDeg, cx, cy)
	}

	primitives := drawing.BuildConnectorPrimitives(drawing.ConnectorSpec{
		Type:            shape.ConnectorType,
		BoundsX:         bx,
		BoundsY:         by,
		BoundsW:         bw,
		BoundsH:         bh,
		Adjustments:     shape.ConnectorAdjustments,
		FlipH:           shape.ConnectorFlipH,
		FlipV:           shape.ConnectorFlipV,
		LineStyle:       shape.LineStyle,
		LineColor:       shape.LineColor,
		LineWidthPt:     shape.LineWidthPt,
		ArrowHead:       shape.ArrowHead,
		ArrowHeadWidth:  shape.ArrowHeadWidth,
		ArrowHeadLength: shape.ArrowHeadLength,
		ArrowTail:       shape.ArrowTail,
		ArrowTailWidth:  shape.ArrowTailWidth,
		ArrowTailLength: shape.ArrowTailLength,
	})
	for _, primitive := range primitives {
		writeSVGPrimitive(b, primitive, "")
	}

	if hasRotation {
		b.WriteString("</g>")
	}
	b.WriteString("</svg>\n")
}

func smartArtConnectorPointsString(pts []drawing.Point) string {
	var b strings.Builder
	for i, p := range pts {
		if i > 0 {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "%.4f,%.4f", p.X, p.Y)
	}
	return b.String()
}

func trimSmartArtConnectorTip(pts []drawing.Point, trim float64, tail bool) []drawing.Point {
	if len(pts) < 2 || trim <= 0 {
		return pts
	}
	var a, b drawing.Point
	if tail {
		a, b = pts[0], pts[1]
	} else {
		a, b = pts[len(pts)-1], pts[len(pts)-2]
	}
	dx, dy := b.X-a.X, b.Y-a.Y
	mag := math.Hypot(dx, dy)
	if mag == 0 {
		return pts
	}
	d := math.Min(trim, mag)
	np := drawing.Point{X: a.X + dx*d/mag, Y: a.Y + dy*d/mag}
	if tail {
		pts[0] = np
	} else {
		pts[len(pts)-1] = np
	}
	return pts
}

func smartArtArrowGeometrySVG(ag *drawing.ArrowGeometry, color string) string {
	if ag == nil {
		return ""
	}
	switch ag.Type {
	case "triangle", "stealth", "diamond":
		return fmt.Sprintf(`<polygon points=%q fill="#%s"/>`, smartArtConnectorPointsString(ag.PolyPts), color)
	case "arrow":
		pts := []drawing.Point{ag.Left, ag.Tip, ag.Right}
		return fmt.Sprintf(`<polyline points=%q fill="none" stroke="#%s" stroke-width="%.2f" stroke-linecap="round" stroke-linejoin="round"/>`, smartArtConnectorPointsString(pts), color, ag.StrokeWidth)
	case "oval":
		return fmt.Sprintf(`<ellipse cx="%.4f" cy="%.4f" rx="%.4f" ry="%.4f" fill="#%s"/>`, ag.Center.X, ag.Center.Y, ag.RX, ag.RY, color)
	}
	return ""
}

// renderChartShape renders a chart shape as an SVG image in HTML.
func renderChartShape(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	if shape == nil || len(shape.ImageData) == 0 {
		return
	}
	x, y, w, h := boundsPercent(shape.Bounds, slideW, slideH)

	// Build CSS styles for positioning
	var styles []string
	styles = append(styles,
		"position:absolute",
		fmt.Sprintf("left:%.4f%%", x),
		fmt.Sprintf("top:%.4f%%", y),
		fmt.Sprintf("width:%.4f%%", w),
		fmt.Sprintf("height:%.4f%%", h),
	)

	// Create base64 encoded SVG data URI
	svgBase64 := base64.StdEncoding.EncodeToString(shape.ImageData)
	dataURI := fmt.Sprintf("data:%s;base64,%s", shape.ImageMIME, svgBase64)

	// Render as an image container
	fmt.Fprintf(b, "<div class=\"shape shape--chart\" style=%q%s>\n", strings.Join(styles, ";"), animationAttrs(shape, ctx))
	fmt.Fprintf(b, "<img src=%q alt=%q style=\"width:100%%;height:100%%;object-fit:contain;\">\n", dataURI, escapeAttr(shape.Name))
	b.WriteString("</div>\n")
}

func renderConnector(b *strings.Builder, shape *resolve.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	bx := emuToPt(shape.Bounds.X)
	by := emuToPt(shape.Bounds.Y)
	bw := emuToPt(shape.Bounds.W)
	bh := emuToPt(shape.Bounds.H)

	fmt.Fprintf(b, "<svg class=\"shape\" style=\"position:absolute;left:0;top:0;width:100%%;height:100%%;overflow:visible;pointer-events:none\" viewBox=\"0 0 %.4f %.4f\" xmlns=\"http://www.w3.org/2000/svg\"%s>", slideW, slideH, animationAttrs(shape, ctx))

	// Open a group with a rotation transform when the connector has non-zero rotation.
	// The pivot is the center of the bounding box (same convention as CSS transform-origin:50% 50%).
	hasRotation := shape.RotationDeg != 0
	if hasRotation {
		cx := bx + bw/2
		cy := by + bh/2
		fmt.Fprintf(b, "<g transform=\"rotate(%.4f %.4f %.4f)\">", shape.RotationDeg, cx, cy)
	}

	primitives := drawing.BuildConnectorPrimitives(drawing.ConnectorSpec{
		Type:            shape.ConnectorType,
		BoundsX:         bx,
		BoundsY:         by,
		BoundsW:         bw,
		BoundsH:         bh,
		Adjustments:     shape.ConnectorAdjustments,
		FlipH:           shape.ConnectorFlipH,
		FlipV:           shape.ConnectorFlipV,
		LineStyle:       shape.LineStyle,
		LineColor:       shape.LineColor,
		LineWidthPt:     shape.LineWidthPt,
		ArrowHead:       shape.ArrowHead,
		ArrowHeadWidth:  shape.ArrowHeadWidth,
		ArrowHeadLength: shape.ArrowHeadLength,
		ArrowTail:       shape.ArrowTail,
		ArrowTailWidth:  shape.ArrowTailWidth,
		ArrowTailLength: shape.ArrowTailLength,
	})
	for _, primitive := range primitives {
		writeSVGPrimitive(b, primitive, "")
	}

	if hasRotation {
		b.WriteString("</g>")
	}
	b.WriteString("</svg>\n")
}

func writeSVGPrimitive(b *strings.Builder, primitive drawing.VectorPrimitive, fillDefID string) {
	if b == nil {
		return
	}
	switch primitive.Kind {
	case drawing.PrimitivePolyline:
		if len(primitive.Points) == 0 {
			return
		}
		if primitive.Closed {
			fmt.Fprintf(b, "<polygon points=%q fill=%q", drawingPointsString(primitive.Points), svgPrimitiveFillValue(primitive.Style, fillDefID))
			if primitive.Style.StrokeColor != "" && primitive.Style.StrokeWidth > 0 {
				writeSVGStrokeAttrs(b, primitive.Style.Stroke, primitive.Style.StrokeFillDefID, "#"+primitive.Style.StrokeColor, primitive.Style.StrokeWidth)
			}
			b.WriteString("/>")
			return
		}
		if primitive.Style.StrokeColor != "" {
			if len(primitive.Points) == 2 {
				fmt.Fprintf(b, "<line x1=\"%.4f\" y1=\"%.4f\" x2=\"%.4f\" y2=\"%.4f\"",
					primitive.Points[0].X, primitive.Points[0].Y, primitive.Points[1].X, primitive.Points[1].Y)
				writeSVGStrokeAttrs(b, primitive.Style.Stroke, primitive.Style.StrokeFillDefID, "#"+primitive.Style.StrokeColor, primitive.Style.StrokeWidth)
				b.WriteString("/>")
				return
			}
			fmt.Fprintf(b, "<polyline points=%q fill=\"none\"", drawingPointsString(primitive.Points))
			writeSVGStrokeAttrs(b, primitive.Style.Stroke, primitive.Style.StrokeFillDefID, "#"+primitive.Style.StrokeColor, primitive.Style.StrokeWidth)
			b.WriteString("/>")
		}
	case drawing.PrimitiveEllipse:
		if primitive.Ellipse == nil {
			return
		}
		fill := svgPrimitiveFillValue(primitive.Style, fillDefID)
		fmt.Fprintf(b, "<ellipse cx=\"%.4f\" cy=\"%.4f\" rx=\"%.4f\" ry=\"%.4f\" fill=%q",
			primitive.Ellipse.Center.X, primitive.Ellipse.Center.Y, primitive.Ellipse.RX, primitive.Ellipse.RY, fill)
		if primitive.Style.StrokeColor != "" && primitive.Style.StrokeWidth > 0 {
			writeSVGStrokeAttrs(b, primitive.Style.Stroke, primitive.Style.StrokeFillDefID, "#"+primitive.Style.StrokeColor, primitive.Style.StrokeWidth)
		}
		b.WriteString("/>")
	case drawing.PrimitivePath:
		if primitive.Path == nil {
			return
		}
		fmt.Fprintf(b, "<path d=%q fill=%q", svgPathData(*primitive.Path), svgPrimitiveFillValue(primitive.Style, fillDefID))
		if primitive.Style.StrokeColor != "" && primitive.Style.StrokeWidth > 0 {
			writeSVGStrokeAttrs(b, primitive.Style.Stroke, primitive.Style.StrokeFillDefID, "#"+primitive.Style.StrokeColor, primitive.Style.StrokeWidth)
		} else if primitive.Style.Stroke != nil {
			writeSVGStrokeAttrs(b, primitive.Style.Stroke, primitive.Style.StrokeFillDefID, "none", 0)
		}
		if primitive.Path.FillRule != "" {
			fmt.Fprintf(b, " fill-rule=%q", primitive.Path.FillRule)
		}
		b.WriteString("/>")
	}
}

func svgPrimitiveFillValue(style drawing.VectorStyle, fillDefID string) string {
	if style.Fill != nil {
		if fillDefID != "" && drawingcss.NeedsSVGDefinition(style.Fill) {
			return "url(#" + fillDefID + ")"
		}
		if fill, ok := drawingcss.Fill(style.Fill)["fill"]; ok && fill != "" {
			return fill
		}
	}
	if style.FillColor != "" {
		return "#" + style.FillColor
	}
	return "none"
}

func writeSVGStrokeAttrs(b *strings.Builder, style *drawing.LineStyle, strokeDefID string, fallbackColor string, fallbackWidth float64) {
	if b == nil {
		return
	}
	color := fallbackColor
	if strokeDefID != "" {
		color = "url(#" + strokeDefID + ")"
	}
	width := fallbackWidth
	cap := ""
	join := ""
	miterLimit := 0.0
	dashArray := ""
	if style != nil {
		if style.Color != "" {
			color = "#" + style.Color
		}
		if style.Width > 0 {
			width = style.Width
		}
		switch style.Cap {
		case drawing.LineCapRound:
			cap = "round"
		case drawing.LineCapSquare:
			cap = "square"
		default:
			cap = "butt"
		}
		switch style.Join {
		case drawing.LineJoinRound:
			join = "round"
		case drawing.LineJoinBevel:
			join = "bevel"
		default:
			join = "miter"
		}
		miterLimit = style.MiterLimit
		dashArray = svgDashArray(style.Dash)
	}
	if color == "" || (color == "none" && strokeDefID == "") || width <= 0 {
		return
	}
	fmt.Fprintf(b, " stroke=%q stroke-width=\"%.2f\"", color, width)
	if cap != "" {
		fmt.Fprintf(b, " stroke-linecap=%q", cap)
	}
	if join != "" {
		fmt.Fprintf(b, " stroke-linejoin=%q", join)
	}
	if join == "miter" && miterLimit > 0 {
		fmt.Fprintf(b, " stroke-miterlimit=\"%.2f\"", miterLimit)
	}
	if dashArray != "" {
		fmt.Fprintf(b, " stroke-dasharray=%q", dashArray)
	}
}

func svgDashArray(dash drawing.DashPattern) string {
	switch dash {
	case drawing.DashPatternDash, drawing.DashPatternSystemDash:
		return "3, 3"
	case drawing.DashPatternDot, drawing.DashPatternSystemDot:
		return "1, 3"
	case drawing.DashPatternDashDot, drawing.DashPatternSystemDashDot:
		return "3, 3, 1, 3"
	case drawing.DashPatternLongDash:
		return "5, 3"
	case drawing.DashPatternLongDashDot:
		return "5, 3, 1, 3"
	case drawing.DashPatternLongDashDotDot, drawing.DashPatternSystemDashDotDot:
		return "5, 3, 1, 3, 1, 3"
	default:
		return ""
	}
}

func drawingPointsString(points []drawing.Point) string {
	var b strings.Builder
	for i, pt := range points {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%.4f,%.4f", pt.X, pt.Y)
	}
	return b.String()
}

func renderParagraph(b *strings.Builder, p *resolve.Paragraph, autoCounters map[string]int, fontScale, lineScale float64, ctx *htmlRenderContext) {
	var pStyles []string
	if p.Align != "" {
		pStyles = append(pStyles, "text-align:"+mapAlign(p.Align))
	}
	if p.TextDirection == "rtl" || p.TextDirection == "ltr" {
		pStyles = append(pStyles, "direction:"+p.TextDirection)
	}
	if p.HasMarginLeft && p.MarginLeftEMU != 0 {
		pStyles = append(pStyles, "margin-left:"+emuIntCSSPt(p.MarginLeftEMU))
	}
	if p.HasMarginRight && p.MarginRightEMU != 0 {
		pStyles = append(pStyles, "margin-right:"+emuIntCSSPt(p.MarginRightEMU))
	}
	if p.HasTextIndent && p.TextIndentEMU != 0 {
		pStyles = append(pStyles, "text-indent:"+emuIntCSSPt(p.TextIndentEMU))
	}
	if p.FontAlign != "" {
		switch p.FontAlign {
		case "t":
			pStyles = append(pStyles, "vertical-align:top")
		case "ctr":
			pStyles = append(pStyles, "vertical-align:middle")
		case "base":
			pStyles = append(pStyles, "vertical-align:baseline")
		case "b":
			pStyles = append(pStyles, "vertical-align:bottom")
		}
	}
	if p.HasSpaceBefore {
		pStyles = append(pStyles, "margin-top:"+points100CSSPt(p.SpaceBefore))
	} else if p.SpaceBeforePct > 0 {
		pStyles = append(pStyles, fmt.Sprintf("margin-top:%.4fem", float64(p.SpaceBeforePct)/100000.0))
	}
	if p.HasSpaceAfter {
		pStyles = append(pStyles, "margin-bottom:"+points100CSSPt(p.SpaceAfter))
	} else if p.SpaceAfterPct > 0 {
		pStyles = append(pStyles, fmt.Sprintf("margin-bottom:%.4fem", float64(p.SpaceAfterPct)/100000.0))
	}
	if p.HasLineSpacing && p.LineSpacingPct > 0 {
		effectiveLineHeight := (float64(p.LineSpacingPct) / 100000.0) * lineScale
		pStyles = append(pStyles, fmt.Sprintf("line-height:%.4f", effectiveLineHeight))
	}
	indent := p.Level
	if p.BulletChar != "" || p.AutoNum {
		indent++
	}
	if !p.HasMarginLeft && !p.HasTextIndent && indent > 0 {
		pStyles = append(pStyles, fmt.Sprintf("padding-left:%dem", indent*2))
	}

	// Determine base font size for bullet percentage calculations.
	baseFontSize := 0
	for _, run := range p.Runs {
		if run.FontSize > 0 {
			baseFontSize = run.FontSize
			break
		}
	}

	// Tab handling: if the paragraph has tab stops or default tab size and
	// contains tab characters, emit tab-size CSS.
	hasTabs := false
	for _, run := range p.Runs {
		if strings.Contains(run.Text, "\t") {
			hasTabs = true
			break
		}
	}
	if hasTabs {
		if p.DefaultTabSizeEMU > 0 {
			pStyles = append(pStyles, fmt.Sprintf("tab-size:%.1fpt", emuToPt(int64(p.DefaultTabSizeEMU))))
		} else if len(p.TabStops) > 0 {
			pStyles = append(pStyles, fmt.Sprintf("tab-size:%.1fpt", emuToPt(p.TabStops[0].Pos)))
		}
	}

	styleAttr := ""
	if len(pStyles) > 0 {
		styleAttr = fmt.Sprintf(" style=%q", strings.Join(pStyles, ";"))
	}

	dataAttr := ""
	if len(p.TabStops) > 0 {
		dataAttr = fmt.Sprintf(" data-tab-stops=%q", encodeTabStops(p.TabStops))
	}

	if len(p.Runs) == 0 {
		clear(autoCounters)
		fmt.Fprintf(b, "<p%s%s><br></p>\n", styleAttr, dataAttr)
		return
	}

	fmt.Fprintf(b, "<p%s%s>", styleAttr, dataAttr)

	switch {
	case p.BulletChar != "":
		clear(autoCounters)
		renderMarker(b, p, escapeHTML(p.BulletChar), fontScale, baseFontSize, ctx)
	case p.AutoNum:
		key := fmt.Sprintf("%s-%d", p.AutoNumType, p.Level)
		if autoCounters[key] == 0 {
			start := p.AutoNumStart
			if start <= 0 {
				start = 1
			}
			autoCounters[key] = start
		}
		marker := formatAutoNum(p.AutoNumType, autoCounters[key])
		autoCounters[key]++
		renderMarker(b, p, marker, fontScale, baseFontSize, ctx)
	default:
		clear(autoCounters)
	}

	for _, run := range p.Runs {
		renderRun(b, run, fontScale, ctx)
	}
	b.WriteString("</p>\n")
}

func renderMarker(b *strings.Builder, p *resolve.Paragraph, marker string, fontScale float64, baseFontSize int, ctx *htmlRenderContext) {
	var styles []string
	if p.BulletSize > 0 {
		styles = append(styles, fmt.Sprintf("font-size:%.1fpt", (float64(p.BulletSize)/100)*fontScale))
	} else if p.BulletSizePct > 0 && baseFontSize > 0 {
		size := (float64(baseFontSize) / 100.0) * (float64(p.BulletSizePct) / 100000.0) * fontScale
		styles = append(styles, fmt.Sprintf("font-size:%.1fpt", size))
	}
	if p.BulletFontFamily != "" {
		styles = append(styles, "font-family:"+cssFontFamilyValue(p.BulletFontFamily, !ctx.isEmbeddedFamily(p.BulletFontFamily)))
	}
	if p.BulletColor != "" {
		styles = append(styles, "color:#"+p.BulletColor)
	}
	if len(styles) == 0 {
		fmt.Fprintf(b, "<span class=\"bullet-marker\">%s</span> ", marker)
		return
	}
	fmt.Fprintf(b, "<span class=\"bullet-marker\" style=%q>%s</span> ", strings.Join(styles, ";"), marker)
}

func textAutofitFontScale(props drawing.TextBodyProperties) float64 {
	if props.AutofitType != "normAutofit" || props.AutofitFontScale <= 0 {
		return 1
	}
	scale := float64(props.AutofitFontScale) / 100000.0
	if scale <= 0 {
		return 1
	}
	return scale
}

func textAutofitLineScale(props drawing.TextBodyProperties) float64 {
	if props.AutofitType != "normAutofit" || props.AutofitLineSpaceReduction <= 0 {
		return 1
	}
	scale := 1 - (float64(props.AutofitLineSpaceReduction) / 100000.0)
	if scale <= 0 {
		return 1
	}
	return scale
}

func formatAutoNum(numType string, n int) string {
	switch numType {
	case "arabicPeriod":
		return fmt.Sprintf("%d.", n)
	case "arabicParenR":
		return fmt.Sprintf("%d)", n)
	case "alphaLcPeriod":
		return fmt.Sprintf("%c.", 'a'+n-1)
	case "alphaUcPeriod":
		return fmt.Sprintf("%c.", 'A'+n-1)
	case "romanLcPeriod":
		return fmt.Sprintf("%s.", romanLower(n))
	case "romanUcPeriod":
		return fmt.Sprintf("%s.", romanUpper(n))
	default:
		return fmt.Sprintf("%d.", n)
	}
}

func romanLower(n int) string { return strings.ToLower(romanUpper(n)) }
func romanUpper(n int) string {
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var out strings.Builder
	for i, v := range vals {
		for n >= v {
			out.WriteString(syms[i])
			n -= v
		}
	}
	return out.String()
}

func renderRun(b *strings.Builder, run *resolve.Run, fontScale float64, ctx *htmlRenderContext) {
	// Math runs carry pre-converted MathML — emit verbatim, no styling.
	if run.MathML != "" {
		b.WriteString(run.MathML)
		return
	}
	var content strings.Builder
	var styles []string
	// Resolve field value into display text.
	fieldText := ""
	if run.FieldKind != "" {
		switch run.FieldKind {
		case "slidenum":
			fieldText = fmt.Sprintf("%d", ctx.slideNumber)
		default:
			// Unknown fields fall through to display run.Text as-is.
		}
	}
	if run.Bold {
		styles = append(styles, "font-weight:bold")
	}
	if run.Italic {
		styles = append(styles, "font-style:italic")
	}
	if run.SmallCaps {
		styles = append(styles, "font-variant:small-caps")
	}
	var decorations []string
	if run.Underline {
		decorations = append(decorations, "underline")
	}
	if run.Strike {
		decorations = append(decorations, "line-through")
	}
	if len(decorations) > 0 {
		styles = append(styles, "text-decoration:"+strings.Join(decorations, " "))
	}
	if run.FontSize > 0 {
		styles = append(styles, fmt.Sprintf("font-size:%.1fpt", (float64(run.FontSize)/100)*fontScale))
	}
	if run.FontFamily != "" {
		styles = append(styles, "font-family:"+cssFontFamilyValue(run.FontFamily, !ctx.isEmbeddedFamily(run.FontFamily)))
	}
	if run.Color != "" {
		styles = append(styles, "color:#"+run.Color)
	} else if len(styles) > 0 {
		// Always include an explicit color in styled spans so the output is
		// self-contained and does not rely on CSS color inheritance.
		styles = append(styles, "color:#"+ctx.defaultTextColor)
	}
	if run.Highlight != "" {
		styles = append(styles, "background-color:#"+run.Highlight)
	}
	if len(run.Effects.Effects) > 0 {
		styles = appendSortedCSSProps(styles, drawingcss.EffectList(run.Effects))
	}
	rawText := run.Text
	if fieldText != "" {
		rawText = fieldText
	}
	if run.AllCaps {
		rawText = strings.ToUpper(rawText)
	}
	text := escapeHTML(rawText)
	// DrawingML line breaks (a:br) are resolved as \n runs; emit HTML <br> tags.
	text = strings.ReplaceAll(text, "\n", "<br>")
	wrapTag := ""
	if run.Superscript {
		wrapTag = "sup"
	} else if run.Subscript {
		wrapTag = "sub"
	}
	if len(styles) == 0 {
		if wrapTag != "" {
			fmt.Fprintf(&content, "<%s>%s</%s>", wrapTag, text, wrapTag)
		} else {
			content.WriteString(text)
		}
	} else if wrapTag != "" {
		fmt.Fprintf(&content, "<%s><span style=%q>%s</span></%s>", wrapTag, strings.Join(styles, ";"), text, wrapTag)
	} else {
		fmt.Fprintf(&content, "<span style=%q>%s</span>", strings.Join(styles, ";"), text)
	}
	if strings.TrimSpace(run.HyperlinkURL) != "" {
		fmt.Fprintf(b, "<a href=%q>%s</a>", escapeAttr(run.HyperlinkURL), content.String())
		return
	}
	b.WriteString(content.String())
}

func (ctx *htmlRenderContext) isEmbeddedFamily(family string) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.embeddedFamilies[normalizeFontFamily(family)]
	return ok
}

func renderEmbeddedFontCSS(b *strings.Builder, pres *resolve.Presentation, ctx *htmlRenderContext) {
	if pres == nil {
		return
	}
	for _, font := range pres.EmbeddedFonts {
		if font == nil || strings.TrimSpace(font.Family) == "" {
			continue
		}
		for _, face := range font.Faces {
			if face == nil || len(face.Data) == 0 {
				continue
			}
			sources := fontdata.EmbeddedSources(face.Data, face.ContentType, face.Family)
			if len(sources) == 0 {
				continue
			}
			fmt.Fprintf(b, "@font-face{font-family:%s;font-style:%s;font-weight:%d;font-display:swap;src:",
				quoteCSSFontFamilyName(face.Family),
				fontStyleValue(face.Style),
				fontWeightValue(face.Weight),
			)
			for i, src := range sources {
				if i > 0 {
					b.WriteByte(',')
				}
				if ctx != nil && ctx.options.FontMode == HTMLFontModeExternal {
					fmt.Fprintf(b, "url(%q) format(%q)", ctx.addFontAsset(src), src.Format)
					continue
				}
				encoded := base64.StdEncoding.EncodeToString(src.Data)
				fmt.Fprintf(b, "url(data:%s;base64,%s) format(%q)", src.MIME, encoded, src.Format)
			}
			b.WriteString(";}\n")
		}
	}
}

func fontStyleValue(style string) string {
	if style == "italic" {
		return "italic"
	}
	return "normal"
}

func fontWeightValue(weight int) int {
	if weight >= 700 {
		return 700
	}
	return 400
}

func cssFontFamilyValue(family string, includeFallbacks bool) string {
	if family == "" {
		return family
	}
	families := []string{family}
	if includeFallbacks {
		families = append(families, bestEffortFontFallbacks(family)...)
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(families))
	for _, candidate := range families {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := normalizeFontFamily(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if isGenericFontFamily(candidate) {
			out = append(out, candidate)
			continue
		}
		out = append(out, quoteCSSFontFamilyName(candidate))
	}
	return strings.Join(out, ",")
}

func bestEffortFontFallbacks(family string) []string {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "source code pro":
		return []string{"Consolas", "Courier New", "monospace"}
	case "oswald":
		return []string{"Arial Narrow", "Arial", "sans-serif"}
	}
	lower := strings.ToLower(family)
	if strings.Contains(lower, "code") || strings.Contains(lower, "mono") {
		return []string{"Consolas", "Courier New", "monospace"}
	}
	if strings.Contains(lower, "display") || strings.Contains(lower, "condensed") {
		return []string{"Arial Narrow", "Arial", "sans-serif"}
	}
	return []string{"Arial", "sans-serif"}
}

func normalizeFontFamily(family string) string {
	return strings.ToLower(strings.TrimSpace(family))
}

func isGenericFontFamily(family string) bool {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "serif", "sans-serif", "monospace", "cursive", "fantasy", "system-ui", "math", "emoji", "fangsong":
		return true
	default:
		return false
	}
}

func quoteCSSFontFamilyName(family string) string {
	if family == "" {
		return family
	}
	if strings.ContainsAny(family, " ,\"'") {
		// Use single quotes so the value is safe inside a double-quoted HTML attribute.
		return "'" + strings.ReplaceAll(family, "'", "\\'") + "'"
	}
	return family
}

func placeholderClass(kind resolve.PlaceholderKind) string {
	switch kind {
	case resolve.PlaceholderTitle:
		return "ph-title"
	case resolve.PlaceholderBody:
		return "ph-body"
	case resolve.PlaceholderSubtitle:
		return "ph-subtitle"
	default:
		return "ph-other"
	}
}

func mapAlign(a string) string {
	switch a {
	case "ctr":
		return "center"
	case "r":
		return "right"
	case "just":
		return "justify"
	default:
		return "left"
	}
}

// boundsPercent converts EMU rect to percentage coordinates within the slide canvas.
func boundsPercent(r resolve.Rect, slideWPt, slideHPt float64) (x, y, w, h float64) {
	xPt := drawing.EMUToPoints(r.X)
	yPt := drawing.EMUToPoints(r.Y)
	wPt := drawing.EMUToPoints(r.W)
	hPt := drawing.EMUToPoints(r.H)
	x = xPt / slideWPt * 100
	y = yPt / slideHPt * 100
	w = wPt / slideWPt * 100
	h = hPt / slideHPt * 100
	return
}

func emuCSSPt(emu int64) string {
	return fmt.Sprintf("%.2fpt", drawing.EMUToPoints(emu))
}

func emuIntCSSPt(emu int) string {
	return fmt.Sprintf("%.2fpt", drawing.EMUToPoints(int64(emu)))
}

func points100CSSPt(value int) string {
	return fmt.Sprintf("%.2fpt", float64(value)/100.0)
}

func textAnchorJustify(anchor string) string {
	switch anchor {
	case "ctr":
		return "center"
	case "b":
		return "flex-end"
	default:
		return "flex-start"
	}
}

func encodeTabStops(stops []resolve.TabStop) string {
	var parts []string
	for _, s := range stops {
		parts = append(parts, fmt.Sprintf("%d:%s", s.Pos, s.Algn))
	}
	return strings.Join(parts, ",")
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func escapeAttr(s string) string {
	s = escapeHTML(s)
	s = strings.ReplaceAll(s, "\"", "&#34;")
	return s
}

// animationAttrs returns the playback metadata for a rendered shape.
func animationAttrs(shape *resolve.Shape, ctx *htmlRenderContext) string {
	if shape == nil {
		return ""
	}
	var attrs strings.Builder
	if shapeHasTimelineTarget(shape, ctx) {
		fmt.Fprintf(&attrs, " data-shape-id=\"%d\"", shape.ID)
	}
	if shape.AnimStep <= 0 {
		return attrs.String()
	}
	fmt.Fprintf(&attrs, " data-anim-step=\"%d\"", shape.AnimStep)
	return attrs.String()
}

func shapeHasTimelineTarget(shape *resolve.Shape, ctx *htmlRenderContext) bool {
	if shape == nil || shape.ID <= 0 || ctx == nil || len(ctx.animShapeIDs) == 0 {
		return false
	}
	_, ok := ctx.animShapeIDs[shape.ID]
	return ok
}

func animationTargetShapeIDs(slide *resolve.Slide) map[int]struct{} {
	if slide == nil || len(slide.AnimationTimeline) == 0 {
		return nil
	}
	ids := make(map[int]struct{}, len(slide.AnimationTimeline))
	for _, event := range slide.AnimationTimeline {
		if event.ShapeID > 0 {
			ids[event.ShapeID] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func emphasisEffectsAttr(shape *resolve.Shape) string {
	if shape == nil || len(shape.EmphasisEffects) == 0 {
		return ""
	}
	return fmt.Sprintf(" data-anim-emphasis=\"%s\"", strings.Join(shape.EmphasisEffects, " "))
}

func motionPathAttr(shape *resolve.Shape) string {
	if shape == nil || shape.MotionPath == nil || shape.MotionPath.Path == "" {
		return ""
	}
	mp := shape.MotionPath
	var parts []string
	parts = append(parts, fmt.Sprintf("path=\"%s\"", escapeAttr(mp.Path)))
	if mp.Dur > 0 {
		parts = append(parts, fmt.Sprintf("dur=\"%d\"", mp.Dur))
	}
	if mp.Origin != "" {
		parts = append(parts, fmt.Sprintf("origin=\"%s\"", mp.Origin))
	}
	return fmt.Sprintf(" data-anim-motion=\"%s\"", strings.Join(parts, " "))
}

func triggerAttr(shape *resolve.Shape) string {
	if shape == nil || len(shape.AnimationTriggers) == 0 {
		return ""
	}
	return fmt.Sprintf(" data-anim-trigger=\"%s\"", strings.Join(shape.AnimationTriggers, " "))
}

func presentationNeedsPlayback(pres *resolve.Presentation) bool {
	if pres == nil {
		return false
	}
	for _, slide := range pres.Slides {
		if slide == nil {
			continue
		}
		if slide.Transition != nil || slideMaxAnimStep(slide) > 0 {
			return true
		}
		if len(slide.AnimationTimeline) > 0 {
			return true
		}
		if slideHasInteractiveAnimations(slide) {
			return true
		}
	}
	return false
}

func slideHasInteractiveAnimations(slide *resolve.Slide) bool {
	if slide == nil {
		return false
	}
	return shapesHaveInteractiveAnimations(slide.Shapes)
}

func shapesHaveInteractiveAnimations(shapes []*resolve.Shape) bool {
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		if len(shape.EmphasisEffects) > 0 || shape.MotionPath != nil || len(shape.AnimationTriggers) > 0 || shape.ActionLink != "" {
			return true
		}
		if shapesHaveInteractiveAnimations(shape.Children) {
			return true
		}
	}
	return false
}

func slideMaxAnimStep(slide *resolve.Slide) int {
	if slide == nil {
		return 0
	}
	return maxAnimStep(slide.Shapes)
}

func maxAnimStep(shapes []*resolve.Shape) int {
	maxStep := 0
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		if shape.AnimStep > maxStep {
			maxStep = shape.AnimStep
		}
		if childMax := maxAnimStep(shape.Children); childMax > maxStep {
			maxStep = childMax
		}
	}
	return maxStep
}

func renderDiagnostics(b *strings.Builder, diagnostics []diag.Diagnostic) {
	if len(diagnostics) == 0 {
		return
	}
	b.WriteString("<details class=\"diagnostics\">\n")
	b.WriteString("<summary>Unsupported Features</summary>\n")
	b.WriteString("<ul>\n")
	for _, diagnostic := range diagnostics {
		severityClass := "diagnostic-warning"
		switch diagnostic.Severity {
		case diag.SeverityError:
			severityClass = "diagnostic-error"
		case diag.SeverityInfo:
			severityClass = "diagnostic-info"
		}
		message := escapeHTML(diagnostic.Message)
		if diagnostic.Code != "" {
			message = fmt.Sprintf("[%s] %s", escapeHTML(diagnostic.Code), message)
		}
		if diagnostic.Location.Part != "" {
			message = fmt.Sprintf("%s (%s)", message, escapeHTML(diagnostic.Location.Part))
		}
		fmt.Fprintf(b, "<li class=%q>%s</li>\n", severityClass, message)
	}
	b.WriteString("</ul>\n")
	b.WriteString("</details>\n")
}

// slideCSS is a format string receiving (slideWidthPt, slideHeightPt, aspectRatio).
const slideCSS = `
html { box-sizing: border-box; }
*, *::before, *::after { box-sizing: inherit; }
body { font-family: sans-serif; margin: 0; padding: 1rem; background: #888; }
.slide { margin: 1rem auto; max-width: 960px; }
.slide-num { font-size: 11px; color: #aaa; margin-bottom: 4px; }
.slide-scroll { overflow-x: auto; }
.canvas {
  position: relative;
  width: %.2fpt;
  height: %.2fpt;
  background: #fff;
  overflow: hidden;
  box-shadow: 0 2px 8px rgba(0,0,0,.4);
  aspect-ratio: %.4f;
}
.shape {
  position: absolute;
  overflow: hidden;
  box-sizing: border-box;
}
.shape--text { display: block; }
.shape--text p { margin: 0; }
.shape__effect-overlay { box-sizing: border-box; }
.shape__background,
.shape__vector {
  position: absolute;
  inset: 0;
  width: 100%%;
  height: 100%%;
}
.bullet-marker { margin-right: 0.35em; }
.shape--image { background: #e8e8e8; border: 1px dashed #bbb; }
.notes {
  margin-top: 1rem;
  padding: 1rem;
  background: #f8f8f8;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 12px;
}
.notes .shape {
  position: relative;
  margin-bottom: 0.5rem;
}
.notes .canvas {
  width: 100%%;
  height: auto;
  background: #fff;
  border: 1px solid #ccc;
}
.headers-footers {
  margin-top: 1rem;
  padding: 0.5rem;
  background: #f0f0f0;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 11px;
}
.headers-footers .header,
.headers-footers .footer,
.headers-footers .datetime,
.headers-footers .slide-number {
  margin-bottom: 0.25rem;
}
.headers-footers .shape {
  position: relative;
  display: inline-block;
  margin: 0;
}
.comments {
  margin-top: 1rem;
  padding: 0.75rem 1rem;
  background: #fffbe6;
  border: 1px solid #e6d96a;
  border-radius: 4px;
  font-size: 12px;
}
.comment {
  margin-bottom: 0.5rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid #e6d96a;
}
.comment:last-child { border-bottom: none; margin-bottom: 0; }
.comment-meta { font-size: 11px; color: #777; margin-bottom: 0.25rem; }
.comment-author { font-weight: 700; margin-right: 0.5em; }
.comment-dt { font-style: italic; }
.comment-text { margin: 0; }
.shape--table { position: absolute; overflow: hidden; }
.ppt-table { width: 100%%; border-collapse: collapse; table-layout: fixed; background: #fff; }
.ppt-table td { font-size: 12px; overflow-wrap: anywhere; }
.shape-link { display: contents; text-decoration: none; color: inherit; }
`

const playbackCSS = `
body.ppt-player-ready {
  padding: 0;
  background: #111;
  overflow: hidden;
}
body.ppt-player-ready .diagnostics {
  position: fixed;
  top: 0.75rem;
  left: 0.75rem;
  z-index: 20;
  max-width: min(28rem, calc(100vw - 1.5rem));
}
body.ppt-player-ready .slide {
  display: none;
  margin: 0;
  max-width: none;
  width: 100vw;
  min-height: 100vh;
  position: relative;
}
body.ppt-player-ready .slide.is-active,
body.ppt-player-ready .slide.is-entering,
body.ppt-player-ready .slide.is-leaving {
  display: block;
}
body.ppt-player-ready .slide-num,
body.ppt-player-ready .headers-footers,
body.ppt-player-ready .notes,
body.ppt-player-ready .comments {
  display: none;
}
body.ppt-player-ready .slide-scroll {
  overflow: hidden;
  width: 100vw;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.25rem;
}
body.ppt-player-ready .canvas {
  width: min(calc(100vw - 2.5rem), calc((100vh - 2.5rem) * %.4f));
  height: auto;
  max-height: calc(100vh - 2.5rem);
  margin: 0;
  box-shadow: 0 10px 30px rgba(0,0,0,.45);
  transform-origin: center center;
}
body.ppt-player-ready [data-anim-step][data-anim-visible="0"] {
  visibility: hidden;
}
body.ppt-player-ended .slide {
  display: none !important;
}
body.ppt-player-ready .slide.is-entering .canvas,
body.ppt-player-ready .slide.is-leaving .canvas {
  animation-duration: var(--ppt-tr-duration, 280ms);
  animation-fill-mode: both;
  animation-timing-function: ease;
}
body.ppt-player-ready .slide.is-entering .canvas { animation-name: ppt-fade-in; }
body.ppt-player-ready .slide.is-leaving .canvas { animation-name: ppt-fade-out; }
body.ppt-player-ready .slide[data-transition="push"].is-entering .canvas { animation-name: ppt-push-in; }
body.ppt-player-ready .slide[data-transition="push"].is-leaving .canvas { animation-name: ppt-push-out; }
body.ppt-player-ready .slide[data-transition="wipe"].is-entering .canvas { animation-name: ppt-wipe-in; }
body.ppt-player-ready .slide[data-transition="wipe"].is-leaving .canvas { animation-name: ppt-fade-out; }
body.ppt-player-ready .slide[data-transition="cut"].is-entering .canvas,
body.ppt-player-ready .slide[data-transition="cut"].is-leaving .canvas {
  animation-duration: 1ms;
}
body.ppt-player-ready .slide.is-backward[data-transition="push"].is-entering .canvas { animation-name: ppt-push-in-back; }
body.ppt-player-ready .slide.is-backward[data-transition="push"].is-leaving .canvas { animation-name: ppt-push-out-back; }
@keyframes ppt-fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
@keyframes ppt-fade-out {
  from { opacity: 1; }
  to { opacity: 0; }
}
@keyframes ppt-push-in {
  from { opacity: 0.6; transform: translateX(8%%); }
  to { opacity: 1; transform: translateX(0); }
}
@keyframes ppt-push-out {
  from { opacity: 1; transform: translateX(0); }
  to { opacity: 0.4; transform: translateX(-8%%); }
}
@keyframes ppt-push-in-back {
  from { opacity: 0.6; transform: translateX(-8%%); }
  to { opacity: 1; transform: translateX(0); }
}
@keyframes ppt-push-out-back {
  from { opacity: 1; transform: translateX(0); }
  to { opacity: 0.4; transform: translateX(8%%); }
}
@keyframes ppt-wipe-in {
  from { opacity: 0.4; clip-path: inset(0 100%% 0 0); }
  to { opacity: 1; clip-path: inset(0 0 0 0); }
}
/* Emphasis animations — auto-play when no trigger is present */
body.ppt-player-ready .shape[data-anim-emphasis~="spin"]:not([data-anim-trigger]) { animation: ppt-spin 0.6s ease both; }
body.ppt-player-ready .shape[data-anim-emphasis~="growShrink"]:not([data-anim-trigger]) { animation: ppt-grow 0.4s ease both; }
body.ppt-player-ready .shape[data-anim-emphasis~="motion"]:not([data-anim-trigger]) { animation: ppt-motion 0.6s ease both; }
/* Trigger-activated emphasis animations */
body.ppt-player-ready .shape.ppt-trigger-active[data-anim-emphasis~="spin"] { animation: ppt-spin 0.6s ease both; }
body.ppt-player-ready .shape.ppt-trigger-active[data-anim-emphasis~="growShrink"] { animation: ppt-grow 0.4s ease both; }
body.ppt-player-ready .shape.ppt-trigger-active[data-anim-emphasis~="motion"] { animation: ppt-motion 0.6s ease both; }
@keyframes ppt-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
@keyframes ppt-grow {
  from { transform: scale(1); }
  50%% { transform: scale(1.2); }
  to { transform: scale(1); }
}
@keyframes ppt-motion {
  from { transform: translate(0, 0); }
  25%% { transform: translate(10%%, 0); }
  75%% { transform: translate(-10%%, 0); }
  to { transform: translate(0, 0); }
}
/* Motion path animations */
@keyframes ppt-motion-path {
  from { offset-distance: 0%%; }
  to { offset-distance: 100%%; }
}
body.ppt-player-ready .shape[data-anim-motion]:not([data-anim-trigger]) { animation-name: ppt-motion-path; animation-timing-function: linear; animation-fill-mode: both; }
body.ppt-player-ready .shape.ppt-trigger-active[data-anim-motion] { animation-name: ppt-motion-path; animation-timing-function: linear; animation-fill-mode: both; }
`

const playbackScript = `
document.addEventListener("DOMContentLoaded", function () {
  var slides = Array.prototype.slice.call(document.querySelectorAll(".slide"));
  if (!slides.length) {
    return;
  }
  var interactive = slides.some(function (slide) {
    return slide.hasAttribute("data-transition") ||
      slide.hasAttribute("data-transition-adv-tm") ||
      slide.hasAttribute("data-max-anim-step") ||
      slide.hasAttribute("data-has-anim-timeline") ||
      slide.querySelector("[data-anim-motion],[data-anim-trigger],[data-ppt-action]") !== null;
  });
  if (!interactive) {
    return;
  }

  document.body.classList.add("ppt-player-ready");

  var currentIndex = 0;
  var currentStep = 0;
  var autoTimer = 0;

  function clearTimer() {
    if (autoTimer) {
      window.clearTimeout(autoTimer);
      autoTimer = 0;
    }
  }

  function parseIntAttr(node, name) {
    var raw = node.getAttribute(name);
    if (!raw) {
      return 0;
    }
    var value = parseInt(raw, 10);
    return isNaN(value) ? 0 : value;
  }

  function transitionDuration(slide) {
    switch ((slide.getAttribute("data-transition-spd") || "fast").toLowerCase()) {
      case "slow":
        return 700;
      case "med":
        return 450;
      default:
        return 280;
    }
  }

  function maxStep(slide) {
    return parseIntAttr(slide, "data-max-anim-step");
  }

  function applyStep(slide, step) {
    var nodes = slide.querySelectorAll("[data-anim-step]");
    nodes.forEach(function (node) {
      var required = parseIntAttr(node, "data-anim-step");
      node.setAttribute("data-anim-visible", required <= step ? "1" : "0");
    });
  }

  function parseMotionAttr(node) {
    var raw = node.getAttribute("data-anim-motion");
    if (!raw) return null;
    var result = { path: "", dur: 0, origin: "" };
    var m;
    m = raw.match(/path="([^"]*)"/);
    if (m) result.path = m[1];
    m = raw.match(/dur="(\d+)"/);
    if (m) result.dur = parseInt(m[1], 10);
    m = raw.match(/origin="([^"]*)"/);
    if (m) result.origin = m[1];
    return result;
  }

  function applyMotionPaths(slide) {
    var nodes = slide.querySelectorAll("[data-anim-motion]");
    nodes.forEach(function (node) {
      var motion = parseMotionAttr(node);
      if (motion && motion.path) {
        node.style.offsetPath = 'path("' + motion.path + '")';
        if (motion.dur > 0) {
          node.style.animationDuration = motion.dur + "ms";
        }
      }
    });
  }

  function applyTriggers(slide) {
    var nodes = slide.querySelectorAll("[data-anim-trigger]");
    nodes.forEach(function (node) {
      var triggers = (node.getAttribute("data-anim-trigger") || "").split(" ");
      if (triggers.indexOf("onClick") >= 0) {
        node.addEventListener("click", function (e) {
          e.stopPropagation();
          node.classList.add("ppt-trigger-active");
          var dur = parseFloat(node.style.animationDuration || "1000");
          window.setTimeout(function () {
            node.classList.remove("ppt-trigger-active");
          }, dur);
        });
      }
    });
  }

  function timelineEvents(slide) {
    var script = slide.querySelector("script.ppt-animation-timeline");
    if (!script) return [];
    try { return JSON.parse(script.textContent || "[]"); } catch (_) { return []; }
  }

  function timelineTarget(slide, event) {
    return slide.querySelector('[data-shape-id="' + event.ShapeID + '"]');
  }

  function playTimelineEvent(slide, event) {
    var node = timelineTarget(slide, event);
    if (!node) return;
    var delay = Math.max(0, event.DelayMs || 0);
    var duration = Math.max(1, event.DurationMs || 400);
    window.setTimeout(function () {
      var keyframes = [{ opacity: 1 }, { opacity: 1 }];
      switch (event.Behavior) {
        case "animScale": keyframes = [{ transform: "scale(1)" }, { transform: "scale(1.15)" }, { transform: "scale(1)" }]; break;
        case "animRot": keyframes = [{ transform: "rotate(0deg)" }, { transform: "rotate(360deg)" }]; break;
        case "animEffect":
          if ((event.Effect || "").toLowerCase().indexOf("fade") >= 0) keyframes = [{ opacity: 0 }, { opacity: 1 }];
          break;
        case "animMotion":
          if (event.MotionPath) {
            node.style.offsetPath = 'path("' + event.MotionPath.replace(/"/g, '\\"') + '")';
            keyframes = [{ offsetDistance: "0%" }, { offsetDistance: "100%" }];
          }
          break;
      }
      var iterations = event.RepeatCount && event.RepeatCount > 0 ? event.RepeatCount : 1;
      if (event.RepeatCount < 0 && event.RepeatDurMs > 0) iterations = Math.max(1, Math.floor(event.RepeatDurMs / duration));
      if (node.animate) node.animate(keyframes, { duration: duration, iterations: iterations, direction: event.AutoReverse ? "alternate" : "normal", fill: event.Fill === "hold" ? "forwards" : "none" });
    }, delay);
  }

  function runTimeline(slide, trigger, step) {
    timelineEvents(slide).forEach(function (event) {
      var eventTrigger = event.Trigger || "";
      if (trigger === "activate" && (eventTrigger === "withPrevious" || eventTrigger === "afterPrevious" || eventTrigger === "onBegin")) {
        playTimelineEvent(slide, event);
      } else if (trigger === "step" && event.Step === step && (eventTrigger === "onClick" || event.NodeType === "clickEffect" || eventTrigger === "withPrevious" || eventTrigger === "afterPrevious")) {
        playTimelineEvent(slide, event);
      }
    });
  }

  function syncAutoAdvance() {
    clearTimer();
    if (currentIndex >= slides.length - 1) {
      return;
    }
    var slide = slides[currentIndex];
    var ms = parseIntAttr(slide, "data-transition-adv-tm");
    if (ms > 0) {
      autoTimer = window.setTimeout(function () {
        advance(true);
      }, ms);
    }
  }

  function resetSlideClasses(slide) {
    slide.classList.remove("is-active", "is-entering", "is-leaving", "is-backward");
    slide.style.removeProperty("--ppt-tr-duration");
  }

  function activate(index, options) {
    if (index < 0 || index >= slides.length) {
      return;
    }
    clearTimer();

    options = options || {};
    var previous = slides[currentIndex];
    var next = slides[index];
    var backwards = !!options.backwards;
    var animate = !!options.animate && previous && previous !== next;
    var duration = transitionDuration(next);

    slides.forEach(function (slide) {
      if (slide !== previous && slide !== next) {
        resetSlideClasses(slide);
      }
    });

    currentIndex = index;
    currentStep = options.restoreFinalStep ? maxStep(next) : 0;
    applyStep(next, currentStep);
    applyMotionPaths(next);
    applyTriggers(next);
    runTimeline(next, "activate", 0);

    if (!animate) {
      if (previous && previous !== next) {
        resetSlideClasses(previous);
      }
      next.classList.add("is-active");
      syncAutoAdvance();
      return;
    }

    next.style.setProperty("--ppt-tr-duration", duration + "ms");
    previous.style.setProperty("--ppt-tr-duration", duration + "ms");
    next.classList.add("is-active", "is-entering");
    previous.classList.remove("is-active");
    previous.classList.add("is-leaving");
    if (backwards) {
      next.classList.add("is-backward");
      previous.classList.add("is-backward");
    }

    window.setTimeout(function () {
      resetSlideClasses(previous);
      next.classList.remove("is-entering", "is-backward");
      next.classList.add("is-active");
    }, duration + 40);

    syncAutoAdvance();
  }

  function advance(fromTimer) {
    var slide = slides[currentIndex];
    if (currentStep < maxStep(slide)) {
      currentStep += 1;
      applyStep(slide, currentStep);
      runTimeline(slide, "step", currentStep);
      syncAutoAdvance();
      return;
    }
    if (currentIndex >= slides.length - 1) {
      clearTimer();
      return;
    }
    if (fromTimer || slide.getAttribute("data-transition-adv-click") !== "0") {
      activate(currentIndex + 1, { animate: true });
    }
  }

  function retreat() {
    var slide = slides[currentIndex];
    if (currentStep > 0) {
      currentStep -= 1;
      applyStep(slide, currentStep);
      syncAutoAdvance();
      return;
    }
    if (currentIndex > 0) {
      activate(currentIndex - 1, { animate: true, backwards: true, restoreFinalStep: true });
    }
  }

  function activateAction(action) {
    if (!action) {
      return false;
    }

    var targetIndex = -1;
    switch (action) {
      case "firstSlide":
        targetIndex = 0;
        break;
      case "lastSlide":
        targetIndex = slides.length - 1;
        break;
      case "nextSlide":
        targetIndex = currentIndex + 1;
        break;
      case "prevSlide":
      case "previousSlide":
        targetIndex = currentIndex - 1;
        break;
      case "endShow":
        clearTimer();
        slides.forEach(resetSlideClasses);
        document.body.classList.add("ppt-player-ended");
        return true;
      default:
        var match = action.match(/^slide(\d+)$/);
        if (match) {
          targetIndex = parseInt(match[1], 10) - 1;
        }
        break;
    }

    if (targetIndex < 0 || targetIndex >= slides.length) {
      return false;
    }

    document.body.classList.remove("ppt-player-ended");
    activate(targetIndex, {
      animate: targetIndex !== currentIndex,
      backwards: targetIndex < currentIndex
    });
    return true;
  }

  document.addEventListener("keydown", function (event) {
    switch (event.key) {
      case "ArrowRight":
      case "PageDown":
      case " ":
      case "Enter":
        event.preventDefault();
        advance(false);
        break;
      case "ArrowLeft":
      case "PageUp":
      case "Backspace":
        event.preventDefault();
        retreat();
        break;
    }
  });

  document.addEventListener("click", function (event) {
    var actionNode = event.target.closest("[data-ppt-action]");
    if (actionNode) {
      var action = actionNode.getAttribute("data-ppt-action") || "";
      if (activateAction(action)) {
        event.preventDefault();
      }
      return;
    }
    if (event.target.closest("a,button,input,select,textarea,summary")) {
      return;
    }
    advance(false);
  });

  var requested = window.location.hash ? document.getElementById(window.location.hash.slice(1)) : null;
  if (requested) {
    var requestedIndex = slides.indexOf(requested);
    if (requestedIndex >= 0) {
      currentIndex = requestedIndex;
    }
  }
  activate(currentIndex, { animate: false });
});
`

const diagnosticsCSS = `
.diagnostics { max-width: 960px; margin: 0 auto 1rem; padding: 0.75rem 1rem; background: #fffbe6; border: 1px solid #d9c87c; border-radius: 6px; }
.diagnostics summary { cursor: pointer; font-weight: 700; }
.diagnostics ul { margin: 0.5rem 0 0 1.25rem; padding: 0; }
.diagnostics li { margin: 0.25rem 0; }
.diagnostic-error { color: #a94442; }
.diagnostic-warning { color: #8a6d3b; }
.diagnostic-info { color: #31708f; }
`

func emuToPt(emu int64) float64 {
	return float64(emu) / 12700 // 914400 EMU/inch ÷ 72 pt/inch
}
