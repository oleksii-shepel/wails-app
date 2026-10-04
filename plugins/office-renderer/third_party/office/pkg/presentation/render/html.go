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
	"office/pkg/fontmetrics"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/htmlasset"
	ir "office/pkg/presentation/ir"
	playout "office/pkg/presentation/layout"
	drawingcss "office/pkg/render/drawingcss"
	"office/pkg/render/metafile"
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
	// IncludeNotes exposes speaker notes as an inspection panel after each slide.
	// Normal presentation rendering intentionally leaves auxiliary stories out.
	IncludeNotes bool
	// IncludeComments exposes comments as an inspection panel after each slide.
	IncludeComments bool
	// EnablePlayback opts into slideshow-style playback. Normal HTML is a document
	// view and renders every slide even when the source contains transitions or animations.
	EnablePlayback bool
}

type htmlRenderContext struct {
	embeddedFamilies map[string]struct{}
	options          HTMLOptions
	assets           []HTMLAsset
	nextImageIndex   int
	nextMediaIndex   int
	nextFontIndex    int
	nextEffectIndex  int
	nextWordArtIndex int
	animShapeIDs     map[int]struct{}
	slideNumber      int    // 1-based current slide number for field rendering
	defaultTextColor string // sRGB hex fallback when run.Color is empty (theme dk1)
}

// HTML renders a resolved Presentation as an HTML document.
// Each slide is rendered as a positioned canvas div.
func HTML(pres *ir.Presentation) (string, error) {
	html, _, err := HTMLWithOptions(pres, HTMLOptions{})
	return html, err
}

// HTMLWithOptions renders a resolved Presentation as an HTML document and
// returns any sidecar assets generated for the selected image mode.
func HTMLWithOptions(pres *ir.Presentation, opts HTMLOptions) (string, []HTMLAsset, error) {
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
	physicalSlides := make([]playout.PhysicalSlidePlan, 0, len(pres.Slides))
	materializeOpts := playout.PhysicalMaterializeOptions{
		MetafileFonts: presentationMetafileFontFaces(pres),
		TextFonts:     presentationFontMetricsCatalog(pres),
		SlideWidthPt:  slideW,
		SlideHeightPt: slideH,
	}
	for _, slide := range pres.Slides {
		physicalSlides = append(physicalSlides, playout.MaterializePhysicalSlideWithOptions(slide, materializeOpts))
	}
	playbackEnabled := opts.EnablePlayback && physicalPresentationNeedsPlayback(physicalSlides)
	behaviorRuntimeEnabled := playbackEnabled || physicalPresentationNeedsBehaviorRuntime(physicalSlides)

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<style>\n")
	renderEmbeddedFontCSS(&b, pres, ctx)
	fmt.Fprintf(&b, slideCSS, slideW, slideH, slideW/slideH, slideW, slideH)
	b.WriteString(oleMathCSS)
	if behaviorRuntimeEnabled {
		fmt.Fprintf(&b, playbackCSS, slideW/slideH)
	}
	if len(pres.Diagnostics) > 0 {
		b.WriteString(diagnosticsCSS)
	}
	b.WriteString("</style>\n")
	b.WriteString("<script>\n")
	b.WriteString(oleMathScript)
	b.WriteString("\n</script>\n")
	if behaviorRuntimeEnabled {
		b.WriteString("<script>\n")
		b.WriteString(playbackScript)
		b.WriteString("\n</script>\n")
	}
	b.WriteString("</head>\n<body>\n")
	renderDiagnostics(&b, pres.Diagnostics)

	for i := range physicalSlides {
		physicalSlide := &physicalSlides[i]
		ctx.slideNumber = physicalSlide.Index + 1
		renderSlide(&b, physicalSlide, slideW, slideH, ctx)
	}

	b.WriteString("</body>\n</html>")
	return b.String(), ctx.assetsCopy(), nil
}

func presentationMetafileFontFaces(pres *ir.Presentation) []metafile.FontFace {
	if pres == nil {
		return nil
	}
	var out []metafile.FontFace
	for _, font := range pres.EmbeddedFonts {
		if font == nil {
			continue
		}
		for _, face := range font.Faces {
			if face == nil || len(face.Data) == 0 {
				continue
			}
			for _, source := range fontdata.EmbeddedSources(face.Data, face.ContentType, face.Family) {
				if source.Format != "truetype" && source.Format != "opentype" {
					continue
				}
				out = append(out, metafile.FontFace{
					Family: face.Family,
					Bold:   face.Weight >= 600,
					Italic: strings.EqualFold(strings.TrimSpace(face.Style), "italic"),
					Data:   append([]byte(nil), source.Data...),
				})
				break
			}
		}
	}
	return out
}

func presentationFontMetricsCatalog(pres *ir.Presentation) *fontmetrics.Catalog {
	if pres == nil {
		return nil
	}
	cat := &fontmetrics.Catalog{}
	for _, font := range pres.EmbeddedFonts {
		if font == nil {
			continue
		}
		for _, face := range font.Faces {
			if face == nil || len(face.Data) == 0 {
				continue
			}
			for _, source := range fontdata.EmbeddedSources(face.Data, face.ContentType, face.Family) {
				if source.Format != "truetype" && source.Format != "opentype" {
					continue
				}
				parsed, ok := fontmetrics.ParseSFNT(source.Data)
				if !ok {
					continue
				}
				cat.Faces = append(cat.Faces, fontmetrics.NamedFace{
					Family: face.Family,
					Weight: face.Weight,
					Italic: strings.EqualFold(strings.TrimSpace(face.Style), "italic"),
					Face:   parsed,
				})
				break
			}
		}
	}
	if cat.Empty() {
		return nil
	}
	return cat
}

func newHTMLRenderContext(pres *ir.Presentation, opts HTMLOptions) *htmlRenderContext {
	ctx := &htmlRenderContext{
		embeddedFamilies: map[string]struct{}{},
		options:          opts.normalized(),
		defaultTextColor: "000000",
	}
	if pres == nil {
		return ctx
	}
	presentationVisual := playout.ResolvePresentationVisualPlan(pres)
	ctx.defaultTextColor = presentationVisual.DefaultTextColor
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
		IncludeNotes:     opts.IncludeNotes,
		IncludeComments:  opts.IncludeComments,
		EnablePlayback:   opts.EnablePlayback,
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

func renderSlide(b *strings.Builder, physicalSlide *playout.PhysicalSlidePlan, slideW, slideH float64, ctx *htmlRenderContext) {
	if physicalSlide == nil {
		return
	}
	if ctx != nil {
		ctx.animShapeIDs = physicalSlide.Playback.AnimationTargetShapeIDs
	}
	fmt.Fprintf(b, "<div class=\"slide\" id=\"slide-%d\"", physicalSlide.Index+1)
	if physicalSlide.SectionName != "" {
		fmt.Fprintf(b, " data-section=%q", physicalSlide.SectionName)
	}
	playback := physicalSlide.Playback
	if playback.TransitionType != "" {
		fmt.Fprintf(b, " data-transition=%q", playback.TransitionType)
	}
	if playback.TransitionSpeed != "" {
		fmt.Fprintf(b, " data-transition-spd=%q", playback.TransitionSpeed)
	}
	if !playback.AdvanceClick {
		b.WriteString(" data-transition-adv-click=\"0\"")
	}
	if playback.AdvanceTimeMs > 0 {
		fmt.Fprintf(b, " data-transition-adv-tm=\"%d\"", playback.AdvanceTimeMs)
	}
	if playback.MaxAnimationStep > 0 {
		fmt.Fprintf(b, " data-max-anim-step=%q", strconv.Itoa(playback.MaxAnimationStep))
	}
	if playback.HasAnimationTimeline() {
		b.WriteString(" data-has-anim-timeline=\"1\"")
	}
	b.WriteString(">\n")
	fmt.Fprintf(b, "<div class=\"slide-num\">Slide %d</div>\n", physicalSlide.Index+1)
	bgStyle := ""
	if physicalSlide.Background.Color != "" {
		bgStyle = fmt.Sprintf(" style=\"background-color:#%s\"", physicalSlide.Background.Color)
	}
	fmt.Fprintf(b, "<div class=\"slide-scroll\"><div class=\"canvas\"%s>\n", bgStyle)
	if physicalSlide.Background.Fill != nil && drawingcss.NeedsSVGDefinition(physicalSlide.Background.Fill) {
		fillDefID := fmt.Sprintf("slide-bg-fill-%d", physicalSlide.Index)
		bgWEMU := slideW * 12700.0
		bgHEMU := slideH * 12700.0
		fmt.Fprintf(b, "<svg class=\"slide-background\" style=\"position:absolute;inset:0;width:100%%;height:100%%;z-index:%d\" viewBox=\"0 0 %.0f %.0f\" preserveAspectRatio=\"none\">", physicalSlide.Background.ZIndex, bgWEMU, bgHEMU)
		fmt.Fprintf(b, "<defs>%s</defs>", svgFillDefinition(physicalSlide.Background.Fill, physicalSlide.Background.PictureTile, fillDefID, bgWEMU, bgHEMU, 1.0))
		fmt.Fprintf(b, "<rect width=\"100%%\" height=\"100%%\" fill=\"url(#%s)\"/></svg>\n", fillDefID)
	}

	// Layout owns the complete slide paint sequence. The slide-number wrapper is
	// inserted inline and therefore does not regroup or reorder physical paint.
	renderPhysicalSlidePaintOrder(b, physicalSlide, slideW, slideH, ctx)

	b.WriteString("</div></div>\n")

	// Auxiliary slide surfaces are physically materialized by layout and remain opt-in.
	if ctx != nil && ctx.options.IncludeNotes && physicalSlide.Auxiliary.Notes != nil && len(physicalSlide.Auxiliary.Notes.Content) > 0 {
		notesStyle := ""
		if physicalSlide.Auxiliary.Notes.BackgroundColor != "" {
			notesStyle = fmt.Sprintf(" style=\"background-color:#%s\"", physicalSlide.Auxiliary.Notes.BackgroundColor)
		}
		fmt.Fprintf(b, "<div class=\"notes\"%s>\n", notesStyle)
		renderPhysicalVisuals(b, physicalSlide.Auxiliary.Notes.Content, slideW, slideH, ctx)
		b.WriteString("</div>\n")
	}

	if ctx != nil && ctx.options.IncludeComments && len(physicalSlide.Auxiliary.Comments) > 0 {
		renderPhysicalSlideComments(b, physicalSlide.Auxiliary.Comments)
	}
	if playback.HasAnimationTimeline() {
		if raw, err := json.Marshal(playback.AnimationTimeline); err == nil {
			payload := strings.ReplaceAll(string(raw), "<", "\\u003c")
			fmt.Fprintf(b, "<script type=\"application/json\" class=\"ppt-animation-timeline\">%s</script>\n", payload)
		}
	}

	b.WriteString("</div>\n")
}

func renderPhysicalSlideComments(b *strings.Builder, comments []playout.PhysicalCommentPlan) {
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

func renderVisualShapes(b *strings.Builder, shapes []*ir.Shape, slideW, slideH float64, ctx *htmlRenderContext) {
	physical := playout.MaterializePhysicalVisualTree(shapes)
	renderPhysicalVisuals(b, physical, slideW, slideH, ctx)
}

func renderPhysicalVisuals(b *strings.Builder, physical []playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	for i := range physical {
		renderPhysicalVisual(b, &physical[i], slideW, slideH, ctx)
	}
}

func renderPhysicalSlidePaintOrder(b *strings.Builder, slide *playout.PhysicalSlidePlan, slideW, slideH float64, ctx *htmlRenderContext) {
	if slide == nil {
		return
	}
	for i := range slide.PaintOrder {
		item := &slide.PaintOrder[i]
		// The wrapper is semantic/CSS compatibility only. It is emitted exactly
		// where the slide-number visual occurs in the physical paint sequence,
		// so it cannot move that visual above later authored paint.
		if slide.SlideNumber != nil && item.SourceShape == slide.SlideNumber.SourceShape {
			b.WriteString("<div class=\"slide-number\">\n")
			renderPhysicalVisual(b, item, slideW, slideH, ctx)
			b.WriteString("</div>\n")
			continue
		}
		renderPhysicalVisual(b, item, slideW, slideH, ctx)
	}
}

// renderPhysicalVisual serializes explicit ancestor clips already resolved by
// layout. Each clip is a full-slide wrapper, so nested wrappers naturally
// intersect without asking HTML to reconstruct group hierarchy. The wrapper
// owns the paint-layer z-index; the leaf remains positioned in slide space.
func renderPhysicalVisual(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	if item == nil || item.SourceShape == nil || item.Drawing == nil {
		return
	}
	// Layout owns clip visibility. A fully clipped visual remains in the physical
	// scene for identity/order semantics but produces no HTML paint.
	if item.Drawing.Paint.PaintClipped && item.Drawing.Paint.ClippedBounds.Empty() {
		return
	}
	opened := 0
	for _, clip := range item.Drawing.Paint.AncestorClips {
		path := physicalAncestorClipCSSPath(clip, slideW, slideH)
		if path == "" {
			continue
		}
		styles := []string{"position:absolute", "inset:0", "pointer-events:none", "clip-path:" + path}
		if opened == 0 && item.Drawing.Paint.ZIndex > 0 {
			styles = append(styles, fmt.Sprintf("z-index:%d", item.Drawing.Paint.ZIndex))
		}
		fmt.Fprintf(b, "<div class=\"shape-ancestor-clip\" style=%q>", strings.Join(styles, ";"))
		opened++
	}
	renderShape(b, item, slideW, slideH, ctx)
	for opened > 0 {
		b.WriteString("</div>")
		opened--
	}
	if len(item.Drawing.Paint.AncestorClips) > 0 {
		b.WriteByte('\n')
	}
}

func physicalAncestorClipCSSPath(clip playout.PhysicalAncestorClipPlan, slideW, slideH float64) string {
	if clip.Bounds.Empty() || slideW <= 0 || slideH <= 0 {
		return ""
	}
	if len(clip.Polygon) >= 3 {
		points := make([]string, 0, len(clip.Polygon))
		for _, point := range clip.Polygon {
			points = append(points, fmt.Sprintf("%.4f%% %.4f%%", point.XPt*100/slideW, point.YPt*100/slideH))
		}
		return "polygon(" + strings.Join(points, ",") + ")"
	}
	left := clip.Bounds.LeftPt * 100 / slideW
	top := clip.Bounds.TopPt * 100 / slideH
	width := clip.Bounds.WidthPt * 100 / slideW
	height := clip.Bounds.HeightPt * 100 / slideH
	right := 100 - left - width
	bottom := 100 - top - height
	switch strings.ToLower(clip.Kind) {
	case "ellipse":
		return fmt.Sprintf("ellipse(%.4f%% %.4f%% at %.4f%% %.4f%%)", width/2, height/2, left+width/2, top+height/2)
	case "roundrect":
		radiusPt := math.Min(clip.Bounds.WidthPt, clip.Bounds.HeightPt) * 0.08
		return fmt.Sprintf("inset(%.4f%% %.4f%% %.4f%% %.4f%% round %.4fpt)", top, right, bottom, left, radiusPt)
	default:
		return fmt.Sprintf("inset(%.4f%% %.4f%% %.4f%% %.4f%%)", top, right, bottom, left)
	}
}

func appendPhysicalPaintStyles(styles []string, paint playout.PhysicalPaintPlan) []string {
	if paint.ZIndex > 0 {
		styles = append(styles, fmt.Sprintf("z-index:%d", paint.ZIndex))
	}
	if paint.OverflowVisible {
		styles = append(styles, "overflow:visible")
	}
	return styles
}

func physicalPaintSVGStyle(paint playout.PhysicalPaintPlan) string {
	if paint.ZIndex <= 0 {
		return ""
	}
	return fmt.Sprintf(";z-index:%d", paint.ZIndex)
}

func renderShape(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	if item == nil || item.SourceShape == nil {
		return
	}
	shape := item.SourceShape
	if shape.AnimationOnly && (ctx == nil || !ctx.options.EnablePlayback) {
		return
	}
	if shape.HyperlinkURL != "" || shape.ActionLink != "" {
		var inner strings.Builder
		renderShapeContent(&inner, item, slideW, slideH, ctx)
		if shape.HyperlinkURL != "" {
			fmt.Fprintf(b, "<a href=%q class=\"shape-link\">%s</a>\n", escapeAttr(shape.HyperlinkURL), inner.String())
		} else {
			fmt.Fprintf(b, "<a href=%q class=\"shape-link\" data-ppt-action=%q>%s</a>\n", "#"+actionLinkAnchor(shape.ActionLink), escapeAttr(shape.ActionLink), inner.String())
		}
		return
	}
	renderShapeContent(b, item, slideW, slideH, ctx)
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

func renderShapeContent(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	switch item.Kind {
	case ir.ShapeText:
		renderTextShape(b, item, slideW, slideH, ctx)
	case ir.ShapeImage:
		renderImageShape(b, item, slideW, slideH, ctx)
	case ir.ShapeGroup:
		// Groups are canonical composition nodes, not renderable objects. Layout
		// must flatten them before HTML emission.
		return
	case ir.ShapeTable:
		renderTableShape(b, item, slideW, slideH, ctx)
	case ir.ShapeConnector:
		renderConnector(b, item, slideW, slideH, ctx)
	case ir.ShapeSmartArt:
		renderSmartArtShape(b, item, slideW, slideH, ctx)
	case ir.ShapeSmartArtConnector:
		renderSmartArtConnector(b, item, slideW, slideH, ctx)
	case ir.ShapeChart:
		renderChartShape(b, item, slideW, slideH, ctx)
	case ir.ShapeMedia:
		renderMediaShape(b, item, slideW, slideH, ctx)
	case ir.ShapeOleObject:
		renderOleObjectShape(b, item, slideW, slideH, ctx)
	}
}

func renderTableShape(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	shape := item.SourceShape // generic animation metadata only
	table := item.Table
	if table == nil || len(table.Rows) == 0 {
		return
	}
	x, y, w, h := physicalBoundsPercent(table.FrameBounds, slideW, slideH)
	styles := []string{fmt.Sprintf("left:%.4f%%", x), fmt.Sprintf("top:%.4f%%", y), fmt.Sprintf("width:%.4f%%", w)}
	if h > 0 {
		// The physical table frame is authoritative; browser content must not grow it.
		styles = append(styles, fmt.Sprintf("height:%.4f%%", h))
	}
	var transforms []string
	if table.Transform.RotationDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", table.Transform.RotationDeg))
	}
	if table.Transform.FlipH {
		transforms = append(transforms, "scaleX(-1)")
	}
	if table.Transform.FlipV {
		transforms = append(transforms, "scaleY(-1)")
	}
	effectStyles := presentationEffectPlanStyles(table.Effects)
	transform, origin := mergeEffectTransform(strings.Join(transforms, " "), effectStyles)
	if transform != "" {
		styles = append(styles, "transform:"+transform)
	}
	if origin != "" {
		styles = append(styles, "transform-origin:"+origin)
	}
	styles = appendSortedCSSProps(styles, effectStyles)
	if table.Paint.ZIndex > 0 {
		styles = append(styles, fmt.Sprintf("z-index:%d", table.Paint.ZIndex))
	}
	if table.Paint.OverflowVisible {
		styles = append(styles, "overflow:visible")
	}
	anim, emphasis, motion, trigger := "", "", "", ""
	if shape != nil {
		anim = animationAttrs(shape, ctx)
		emphasis = emphasisEffectsAttr(shape)
		motion = motionPathAttr(shape)
		trigger = triggerAttr(shape)
	}
	fmt.Fprintf(b, "<div class=\"shape shape--table\" style=%q%s%s%s%s>\n", strings.Join(styles, ";"), anim, emphasis, motion, trigger)
	b.WriteString("<table class=\"ppt-table\">\n")
	if len(table.ColumnWidthsPercent) > 0 {
		b.WriteString("<colgroup>")
		for _, width := range table.ColumnWidthsPercent {
			fmt.Fprintf(b, "<col style=%q>", fmt.Sprintf("width:%.4f%%", width))
		}
		b.WriteString("</colgroup>\n")
	}
	for _, row := range table.Rows {
		rowStyle := ""
		if row.HeightPt > 0 {
			rowStyle = fmt.Sprintf(" style=%q", fmt.Sprintf("height:%.2fpt", row.HeightPt))
		}
		fmt.Fprintf(b, "<tr%s>\n", rowStyle)
		for _, cell := range row.Cells {
			if cell.Skip {
				continue
			}
			var attrs, cellStyles []string
			if cell.ColumnSpan > 1 {
				attrs = append(attrs, fmt.Sprintf("colspan=\"%d\"", cell.ColumnSpan))
			}
			if cell.RowSpan > 1 {
				attrs = append(attrs, fmt.Sprintf("rowspan=\"%d\"", cell.RowSpan))
			}
			cellStyles = append(cellStyles, cssBackgroundFillPropsForFill(cell.Fill.Fill, cell.Fill.FallbackColor)...)
			cellStyles = appendSortedCSSProps(cellStyles, presentationEffectPlanStyles(cell.Effects))
			appendBorder := func(slot string, stroke playout.StrokeVisualPlan) {
				if stroke.WidthPt > 0 && stroke.Color != "" {
					style := stroke.CSSStyle
					if style == "" {
						style = "solid"
					}
					cellStyles = append(cellStyles, fmt.Sprintf("border-%s:%.2fpt %s %s", slot, stroke.WidthPt, style, physicalEffectColor(stroke.Color, stroke.Alpha)))
				}
			}
			appendBorder("left", cell.BorderLeft)
			appendBorder("right", cell.BorderRight)
			appendBorder("top", cell.BorderTop)
			appendBorder("bottom", cell.BorderBottom)
			if cell.PaddingLeftPt != 0 || cell.PaddingRightPt != 0 || cell.PaddingTopPt != 0 || cell.PaddingBottomPt != 0 {
				cellStyles = append(cellStyles, fmt.Sprintf("padding:%.2fpt %.2fpt %.2fpt %.2fpt", cell.PaddingTopPt, cell.PaddingRightPt, cell.PaddingBottomPt, cell.PaddingLeftPt))
			}
			// Cell vertical placement is materialized by the physical content box below.
			// Keep the table cell itself top-aligned so browser table layout cannot
			// reinterpret PowerPoint's anchor semantics.
			cellStyles = append(cellStyles, "vertical-align:top")
			if len(cellStyles) > 0 {
				attrs = append(attrs, fmt.Sprintf("style=%q", strings.Join(cellStyles, ";")))
			}
			fmt.Fprintf(b, "<td %s>", strings.Join(attrs, " "))
			contentStyles := []string{"display:flex", "flex-direction:column", "justify-content:" + cell.JustifyContent}
			if cell.ContentHeightPt > 0 {
				contentStyles = append(contentStyles, fmt.Sprintf("height:%.4fpt", cell.ContentHeightPt))
			}
			if cell.ClipContent {
				contentStyles = append(contentStyles, "overflow:hidden")
			}
			fmt.Fprintf(b, "<div class=\"ppt-table__cell-content\" style=%q>", strings.Join(contentStyles, ";"))
			for i, p := range cell.Paragraphs {
				if i >= len(cell.ParagraphPlans) {
					break
				}
				renderParagraphWithPlan(b, p, cell.ParagraphPlans[i], 1.0, ctx)
			}
			b.WriteString("</div></td>\n")
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</table>\n")
	if len(table.Edges) > 0 {
		b.WriteString("<div class=\"ppt-table__edge-layer\" style=\"position:absolute;inset:0;pointer-events:none\">")
		for _, edge := range table.Edges {
			stroke := edge.Stroke
			if stroke.WidthPt <= 0 || stroke.Color == "" {
				continue
			}
			style := stroke.CSSStyle
			if style == "" {
				style = "solid"
			}
			color := physicalEffectColor(stroke.Color, stroke.Alpha)
			if edge.Y1Pct == edge.Y2Pct {
				fmt.Fprintf(b, "<span class=\"ppt-table__edge ppt-table__edge--h\" style=%q></span>", fmt.Sprintf("position:absolute;left:%.4f%%;top:%.4f%%;width:%.4f%%;height:0;border-top:%.2fpt %s %s", edge.X1Pct, edge.Y1Pct, edge.X2Pct-edge.X1Pct, stroke.WidthPt, style, color))
			} else {
				fmt.Fprintf(b, "<span class=\"ppt-table__edge ppt-table__edge--v\" style=%q></span>", fmt.Sprintf("position:absolute;left:%.4f%%;top:%.4f%%;height:%.4f%%;width:0;border-left:%.2fpt %s %s", edge.X1Pct, edge.Y1Pct, edge.Y2Pct-edge.Y1Pct, stroke.WidthPt, style, color))
			}
		}
		b.WriteString("</div>\n")
	}
	writeHTMLFillOverlays(b, item.Drawing.Visual.Effects)
	b.WriteString("</div>\n")
}

func renderTextShape(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	shape := item.SourceShape
	visual := item.Drawing.Visual
	hasContent := len(shape.Paragraphs) > 0 || visual.HasVisibleFill || visual.HasGeometry || visual.HasVisibleStroke || shapeHasTimelineTarget(shape, ctx)
	if !hasContent {
		return
	}
	textPlan := *item.Text
	effectStyles := presentationTextShapeEffectPlanStyles(textPlan.Effects)

	x, y, w, h := physicalBoundsPercent(textPlan.FrameBounds, slideW, slideH)

	var classes []string
	classes = append(classes, "shape", "shape--text")
	if item.PlaceholderClass != "" {
		classes = append(classes, item.PlaceholderClass)
	}

	textFrame := textPlan.Frame
	textFlow := textPlan.Flow
	frameLayout := textPlan.Layout
	textGeometry := textPlan.Geometry
	textBox := textPlan.Box

	var styles []string
	if textBox.MinOuterWidthPt > 0 {
		styles = append(styles,
			fmt.Sprintf("left:%.4f%%", x),
			fmt.Sprintf("top:%.4f%%", y),
			fmt.Sprintf("min-width:%.4fpt", textBox.MinOuterWidthPt),
			fmt.Sprintf("height:%.4f%%", h),
		)
	} else if textBox.MinOuterHeightPt > 0 {
		styles = append(styles,
			fmt.Sprintf("left:%.4f%%", x),
			fmt.Sprintf("top:%.4f%%", y),
			fmt.Sprintf("width:%.4f%%", w),
			fmt.Sprintf("min-height:%.4fpt", textBox.MinOuterHeightPt),
		)
	} else {
		styles = append(styles,
			fmt.Sprintf("left:%.4f%%", x),
			fmt.Sprintf("top:%.4f%%", y),
			fmt.Sprintf("width:%.4f%%", w),
			fmt.Sprintf("height:%.4f%%", h),
		)
	}
	if len(effectStyles) > 0 || textBox.OverflowVisible {
		styles = append(styles, "overflow:visible")
	}
	// Outer text-shape transforms are already materialized by layout.
	var transforms []string
	if textPlan.Transform.RotationDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", textPlan.Transform.RotationDeg))
	}
	if textPlan.Transform.FlipH {
		transforms = append(transforms, "scaleX(-1)")
	}
	if textPlan.Transform.FlipV {
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

	if textPlan.Paint.ZIndex > 0 {
		styles = append(styles, fmt.Sprintf("z-index:%d", textPlan.Paint.ZIndex))
	}
	if textPlan.Paint.OverflowVisible && !textBox.OverflowVisible && len(effectStyles) == 0 {
		styles = append(styles, "overflow:visible")
	}
	geometryAttrs := ""
	if textGeometry.Enabled {
		geometryAttrs = ` data-text-geometry="wordart"`
		if textGeometry.Warp != "" {
			geometryAttrs += fmt.Sprintf(" data-text-warp=%q", escapeAttr(textGeometry.Warp))
		}
	}
	fmt.Fprintf(b, "<div class=%q style=%q%s%s%s%s%s>\n", strings.Join(classes, " "), strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape), geometryAttrs)
	renderedWordArt := renderPhysicalWordArt(b, textGeometry, visual, ctx)
	renderedVector := renderedWordArt
	if !renderedWordArt {
		renderedVector = renderTextShapeVector(b, shape, visual, nil, item.Drawing.Paint.SVG.OverflowVisible, ctx)
	}
	if !renderedVector {
		if visual.HasVisibleFill || visual.HasVisibleStroke {
			fmt.Fprintf(b, "<div class=\"shape__background\" style=%q></div>\n", strings.Join(rectShapeStyles(visual), ";"))
		}
		writeHTMLFillOverlays(b, visual.Effects)
	}

	if renderedWordArt {
		b.WriteString("</div>\n")
		return
	}

	// Many legacy OfficeArt drawing primitives are represented by the parser as
	// text-capable shapes even though they have no text body at all. Emitting an
	// empty flex text layer for those shapes adds a second clipping/overflow
	// surface that can only interfere with the vector paint. The vector is the
	// complete visible content when there are no authored paragraphs, so stop
	// after serializing it.
	if len(shape.Paragraphs) == 0 {
		b.WriteString("</div>\n")
		return
	}

	var contentStyles []string
	contentWidth := "width:100%"
	contentHeight := "height:100%"
	if textBox.ContentAutoWidth {
		contentWidth = "width:auto"
	}
	if textBox.ContentAutoHeight {
		contentHeight = "height:auto"
	}
	contentStyles = append(contentStyles,
		"position:relative",
		"z-index:1",
		"display:flex",
		"flex-direction:column",
		"box-sizing:border-box",
		contentWidth,
		contentHeight,
	)
	if textBox.OverflowVisible || frameLayout.Measured {
		contentStyles = append(contentStyles, "overflow:visible")
	} else if textBox.ClipContent {
		contentStyles = append(contentStyles, "overflow:hidden")
	}

	// Text-body composition is fully layout-owned. HTML serializes the physical
	// frame/flow plan and never reinterprets DrawingML bodyPr.
	if textFlow.RotationDeg != 0 {
		contentStyles = append(contentStyles, fmt.Sprintf("transform:rotate(%.2fdeg)", textFlow.RotationDeg))
	}
	if textFlow.TransformOrigin != "" {
		contentStyles = append(contentStyles, "transform-origin:"+textFlow.TransformOrigin)
	}
	if textFlow.NoWrap {
		contentStyles = append(contentStyles, "white-space:nowrap")
	}
	if textFlow.WritingMode != "" {
		contentStyles = append(contentStyles, "writing-mode:"+textFlow.WritingMode)
	}
	if textFlow.Columns > 1 {
		contentStyles = append(contentStyles, fmt.Sprintf("column-count:%d", textFlow.Columns))
		if textFlow.ColumnGapPt > 0 {
			contentStyles = append(contentStyles, fmt.Sprintf("column-gap:%.2fpt", textFlow.ColumnGapPt))
		}
	}

	// OfficeArt geometry text / WordArt is projected by layout. Keep these
	// glyph-level semantics on the text content rather than reconstructing the
	// legacy FOPT payload in the serializer.
	if textGeometry.Enabled {
		if textGeometry.FontFamily != "" {
			contentStyles = append(contentStyles, "font-family:"+textGeometry.FontFamily)
		}
		if textGeometry.FontSizePt > 0 {
			contentStyles = append(contentStyles, fmt.Sprintf("font-size:%.2fpt", textGeometry.FontSizePt))
		}
		if textGeometry.LetterSpaceEm != 0 {
			contentStyles = append(contentStyles, fmt.Sprintf("letter-spacing:%.4fem", textGeometry.LetterSpaceEm))
		}
		if textGeometry.Align != "" {
			contentStyles = append(contentStyles, "text-align:"+textGeometry.Align)
		}
		if textGeometry.Vertical {
			contentStyles = append(contentStyles, "writing-mode:vertical-rl")
		}
		if textGeometry.Bold {
			contentStyles = append(contentStyles, "font-weight:700")
		}
		if textGeometry.Italic {
			contentStyles = append(contentStyles, "font-style:italic")
		}
		if textGeometry.SmallCaps {
			contentStyles = append(contentStyles, "font-variant:small-caps")
		}
		var decorations []string
		if textGeometry.Underline {
			decorations = append(decorations, "underline")
		}
		if textGeometry.Strike {
			decorations = append(decorations, "line-through")
		}
		if len(decorations) > 0 {
			contentStyles = append(contentStyles, "text-decoration:"+strings.Join(decorations, " "))
		}
		if textGeometry.Shadow {
			contentStyles = append(contentStyles, "text-shadow:1pt 1pt 1pt rgba(0,0,0,.35)")
		}
		if textGeometry.Tight {
			contentStyles = append(contentStyles, "font-kerning:normal")
		}
	}

	contentStyles = appendTextBoxPlacementStyles(contentStyles, textFrame, frameLayout, textBox)

	// Shape/effect overflow and text-frame clipping are independent physical
	// contracts. Shadows, glows, reflections and other paint may need the outer
	// shape to overflow while a fixed text frame still clips its text. Keep the
	// physical line container overflow-visible so glyph ink and flex items are
	// not clipped individually; when clipping is authored, a dedicated frame
	// wrapper owns it instead. This also keeps body rotation and vertical-anchor
	// offsets inside the same authored clipping frame.
	textClipOpen := false
	if textBox.ClipContent {
		clipStyles := []string{"position:absolute", "inset:0", "overflow:hidden", "z-index:1"}
		// Geometry clipping belongs to the fixed text frame, not to the physical
		// line container. A noAutofit body has ClipContent=false and therefore
		// intentionally escapes both the frame and ellipse/freeform geometry.
		if item.Drawing.Paint.Clip.ClipContent && item.Drawing.Paint.Clip.CSSPath != "" {
			clipStyles = append(clipStyles, "clip-path:"+item.Drawing.Paint.Clip.CSSPath)
		}
		fmt.Fprintf(b, "<div class=\"shape__text-clip\" style=%q>\n", strings.Join(clipStyles, ";"))
		textClipOpen = true
	}
	fmt.Fprintf(b, "<div class=\"shape__content\" style=%q>\n", strings.Join(contentStyles, ";"))

	fontScale := textFrame.FontScale
	for i, p := range shape.Paragraphs {
		if i >= len(textPlan.Paragraphs) {
			break
		}
		renderParagraphWithPlan(b, p, textPlan.Paragraphs[i], fontScale, ctx)
	}

	b.WriteString("</div>\n")
	if textClipOpen {
		b.WriteString("</div>\n")
	}
	b.WriteString("</div>\n")
}

// removeLayoutOwnedTextBodyCSS removes properties whose effective values are
// already resolved by presentation/layout. Keeping them here would create a
// second CSS-side text layout model and, in particular, duplicate padding.
func removeLayoutOwnedTextBodyCSS(props map[string]string) {
	for _, key := range []string{
		"transform", "text-align", "white-space", "writing-mode",
		"column-count", "column-gap", "justify-content",
		"padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
	} {
		delete(props, key)
	}
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

func renderPhysicalWordArt(b *strings.Builder, geometry playout.TextGeometryVisualPlan, visual playout.ShapeVisualPlan, ctx *htmlRenderContext) bool {
	if b == nil || !geometry.Enabled || !geometry.Vector || geometry.PathData == "" || strings.TrimSpace(geometry.Text) == "" {
		return false
	}
	pathID := fmt.Sprintf("wordart-path-%d", ctx.nextWordArtIndex)
	ctx.nextWordArtIndex++
	fill := visual.Fill.FallbackColor
	if fill == "" {
		fill = "000000"
	}
	if !strings.HasPrefix(fill, "#") {
		fill = "#" + fill
	}
	styles := []string{"dominant-baseline:middle", "paint-order:stroke fill"}
	if geometry.FontFamily != "" {
		styles = append(styles, "font-family:'"+strings.ReplaceAll(geometry.FontFamily, "'", "\\\\'")+"'")
	}
	if geometry.FontSizePt > 0 {
		styles = append(styles, fmt.Sprintf("font-size:%.2fpt", geometry.FontSizePt))
	}
	if geometry.LetterSpaceEm != 0 {
		styles = append(styles, fmt.Sprintf("letter-spacing:%.4fem", geometry.LetterSpaceEm))
	}
	if geometry.Bold {
		styles = append(styles, "font-weight:700")
	}
	if geometry.Italic {
		styles = append(styles, "font-style:italic")
	}
	if geometry.SmallCaps {
		styles = append(styles, "font-variant:small-caps")
	}
	var decorations []string
	if geometry.Underline {
		decorations = append(decorations, "underline")
	}
	if geometry.Strike {
		decorations = append(decorations, "line-through")
	}
	if len(decorations) > 0 {
		styles = append(styles, "text-decoration:"+strings.Join(decorations, " "))
	}
	if geometry.Kerning || geometry.Tight {
		styles = append(styles, "font-kerning:normal")
	}
	if geometry.Shadow {
		styles = append(styles, "filter:drop-shadow(1px 1px 1px rgba(0,0,0,.45))")
	}

	fmt.Fprintf(b, `<svg class="shape__wordart" viewBox="0 0 100 100" width="100%%" height="100%%" preserveAspectRatio="none" role="img" aria-label=%q xmlns="http://www.w3.org/2000/svg">`, geometry.Text)
	fmt.Fprintf(b, `<defs><path id=%q d=%q pathLength="100"/></defs>`, pathID, geometry.PathData)
	fmt.Fprintf(b, `<text fill=%q text-anchor=%q style=%q`, fill, geometry.TextAnchor, strings.Join(styles, ";"))
	if visual.Stroke.Color != "" && visual.Stroke.WidthPt > 0 {
		stroke := visual.Stroke.Color
		if !strings.HasPrefix(stroke, "#") {
			stroke = "#" + stroke
		}
		fmt.Fprintf(b, ` stroke=%q stroke-width="%.4f"`, stroke, visual.Stroke.WidthPt)
	}
	b.WriteString(`>`)
	fmt.Fprintf(b, `<textPath href="#%s" startOffset=%q`, pathID, geometry.StartOffset)
	if geometry.Stretch || geometry.BestFit || geometry.ShrinkFit {
		b.WriteString(` textLength="90" lengthAdjust="spacingAndGlyphs"`)
	}
	b.WriteString(`>`)
	b.WriteString(escapeHTML(geometry.Text))
	b.WriteString(`</textPath></text></svg>`)
	return true
}

func renderTextShapeVector(b *strings.Builder, shape *ir.Shape, visual playout.ShapeVisualPlan, effectStyles map[string]string, svgOverflowVisible bool, ctx *htmlRenderContext) bool {
	if !visual.HasGeometry && !visual.UseVector {
		return false
	}
	prefix := "shape"
	if shape != nil && shape.ID != 0 {
		prefix = fmt.Sprintf("shape-%d", shape.ID)
	}
	return renderCanonicalShapeVector(b, visual, effectStyles, shapeVectorRenderOptions{
		DefPrefix:          prefix,
		UsePictureTile:     true,
		PictureTile:        visual.Fill.PictureTile,
		WidthHeightAttr:    false,
		SVGOverflowVisible: svgOverflowVisible,
	})
}

type shapeVectorRenderOptions struct {
	DefPrefix          string
	UsePictureTile     bool
	PictureTile        *ir.PictureTile
	WidthHeightAttr    bool
	SVGOverflowVisible bool
}

// renderCanonicalShapeVector is the single SVG materialization path for
// canonical presentation shape visuals. Ordinary shapes and SmartArt resolve
// their semantics before entering this function; only surface-specific SVG
// attributes and definition identifiers remain configurable here.
func renderCanonicalShapeVector(b *strings.Builder, visual playout.ShapeVisualPlan, effectStyles map[string]string, opts shapeVectorRenderOptions) bool {
	if b == nil || visual.WidthPt <= 0 || visual.HeightPt <= 0 {
		return false
	}
	primitives := visual.Primitives
	if len(primitives) == 0 {
		return false
	}
	prefix := opts.DefPrefix
	if prefix == "" {
		prefix = "shape"
	}

	fillDefID := ""
	fillDef := ""
	if visual.FillNeedsDefinition {
		fillDefID = prefix + "-fill"
		if visual.Fill.Gradient != nil {
			fillDef = svgPhysicalGradientDefinition(visual.Fill.Gradient, fillDefID)
		} else if opts.UsePictureTile {
			fillDef = svgFillDefinition(visual.Fill.Fill, opts.PictureTile, fillDefID, visual.WidthPt, visual.HeightPt, 1.0/12700.0)
		} else {
			fillDef = svgDrawingFillDefinition(visual.Fill.Fill, fillDefID)
		}
	}

	lightOverlays := physicalEffectOverlays(visual.Effects.Composition.ThreeD)
	overlayRenders := drawingcss.BuildSVGOverlays(lightOverlays, prefix+"-fill-overlay")
	fillOverlays := visual.Effects.Overlays
	fillOverlayDefs := make([]string, len(fillOverlays))
	fillOverlayDefIDs := make([]string, len(fillOverlays))
	for i, overlay := range fillOverlays {
		if overlay.Fill != nil && overlay.NeedsDefinition {
			id := fmt.Sprintf("%s-effect-fill-%d", prefix, i)
			fillOverlayDefIDs[i] = id
			fillOverlayDefs[i] = svgDrawingFillDefinition(overlay.Fill, id)
		}
	}
	strokeDefs := collectStrokeFillDefsPlans(primitives, prefix)

	fmt.Fprintf(b, "<svg class=\"shape__vector\"")
	if opts.WidthHeightAttr {
		b.WriteString(" width=\"100%\" height=\"100%\"")
	}
	fmt.Fprintf(b, " viewBox=\"0 0 %.4f %.4f\" preserveAspectRatio=\"none\"", visual.WidthPt, visual.HeightPt)
	var svgStyles []string
	if len(effectStyles) > 0 {
		svgStyles = appendSortedCSSProps(svgStyles, effectStyles)
	}
	if opts.SVGOverflowVisible {
		svgStyles = append(svgStyles, "overflow:visible")
	}
	if len(svgStyles) > 0 {
		fmt.Fprintf(b, " style=%q", strings.Join(svgStyles, ";"))
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

	writeThreeDExtrusionSVG(b, primitives, visual.Effects.Composition.ThreeD)
	for _, primitive := range primitives {
		writeSVGPrimitivePlan(b, primitive, fillDefID)
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
			writeSVGPrimitivePlan(b, playout.VectorPrimitivePlan{Primitive: overlayShapePrimitiveFill(primitive.Primitive, overlay.Fill), Stroke: primitive.Stroke}, overlay.DefID)
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
			writeSVGPrimitivePlan(b, playout.VectorPrimitivePlan{Primitive: overlayShapePrimitiveFill(primitive.Primitive, overlay.Fill), Stroke: primitive.Stroke}, fillOverlayDefIDs[i])
		}
		b.WriteString("</g>")
	}
	b.WriteString("</svg>\n")
	return true
}

func svgFillDefinition(fill drawing.Fill, tile *ir.PictureTile, id string, width, height, emuScale float64) string {
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

func physicalEffectOverlays(t *playout.ThreeDVisualPlan) []drawing.FillOverlay {
	if t == nil || len(t.LightingOverlays) == 0 {
		return nil
	}
	overlays := make([]drawing.FillOverlay, 0, len(t.LightingOverlays))
	for _, overlay := range t.LightingOverlays {
		overlays = append(overlays, drawing.FillOverlay{Fill: overlay.Fill, BlendMode: overlay.BlendMode})
	}
	return overlays
}

func writeThreeDExtrusionSVG(b *strings.Builder, primitives []playout.VectorPrimitivePlan, t *playout.ThreeDVisualPlan) {
	if b == nil || t == nil || t.ExtrusionDepthPt <= 0 || len(primitives) == 0 {
		return
	}
	depth := t.ExtrusionDepthPt
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
			writeSVGPrimitivePlan(b, playout.VectorPrimitivePlan{Primitive: overlayShapePrimitiveFill(primitive.Primitive, fill), Stroke: primitive.Stroke}, "")
		}
		b.WriteString("</g>")
	}
	b.WriteString("</g>")
}

func threeDDepthVector(t playout.ThreeDVisualPlan) (float64, float64) {
	// Project extrusion away from the camera. Camera rotations dominate; preset
	// names provide a deterministic fallback for files that omit explicit rot.
	x := math.Sin(t.RotationYDeg * math.Pi / 180.0)
	y := math.Sin(t.RotationXDeg * math.Pi / 180.0)
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

func svgPhysicalGradientDefinition(gradient *playout.GradientVisualPlan, id string) string {
	if gradient == nil || id == "" || len(gradient.Stops) == 0 {
		return ""
	}
	var b strings.Builder
	if gradient.Kind == drawing.GradientTypePath && strings.EqualFold(gradient.PathBoundary, "rect") {
		if def := svgRectangularPathGradientDefinition(gradient, id); def != "" {
			return def
		}
	}
	if gradient.Kind == drawing.GradientTypePath && gradient.PathBoundaryPath != nil {
		if def := svgShapePathGradientDefinition(gradient, id); def != "" {
			return def
		}
	}
	if gradient.Kind == drawing.GradientTypeRadial || gradient.Kind == drawing.GradientTypePath {
		cx, cy := 50.0, 50.0
		if gradient.HasCenter {
			cx, cy = gradient.CenterX*100, gradient.CenterY*100
		}
		// OfficeArt fillTo* defines the center rectangle of a concentric shade.
		// SVG has no rectangular focal plateau, but a zero-sized center rectangle
		// (the form used by the legacy PPT corpus) maps exactly to a radial focus
		// point. Keep non-zero extents in the physical plan rather than discarding
		// them so a future path-gradient backend can materialize them losslessly.
		radius := 70.7107
		// OfficeArt ShadeShape starts at the actual shape boundary and converges
		// on the interior focus. For an ellipse, SVG objectBoundingBox radial
		// geometry is an exact representation: r=50% is the ellipse boundary.
		// Keep the conservative corner-reaching approximation for arbitrary paths
		// until the path-gradient backend can materialize their true boundary.
		if gradient.Kind == drawing.GradientTypePath && strings.EqualFold(gradient.PathBoundary, "ellipse") {
			radius = 50
		}
		fmt.Fprintf(&b, `<radialGradient id="%s" cx="%.4f%%" cy="%.4f%%" r="%.4f%%">`, escapeAttr(id), cx, cy, radius)
	} else {
		// Physical layout owns linear gradient orientation. Compatibility for
		// direct renderer-unit plans remains intentionally narrow; production
		// plans always carry HasLinearVector.
		x1, y1, x2, y2 := gradient.LinearX1*100, gradient.LinearY1*100, gradient.LinearX2*100, gradient.LinearY2*100
		if !gradient.HasLinearVector {
			a := gradient.AngleDeg * math.Pi / 180
			dx, dy := math.Cos(a), math.Sin(a)
			x1, y1 = 50-50*dx, 50-50*dy
			x2, y2 = 50+50*dx, 50+50*dy
			if gradient.FlipH {
				x1, x2 = 100-x1, 100-x2
			}
			if gradient.FlipV {
				y1, y2 = 100-y1, 100-y2
			}
		}
		fmt.Fprintf(&b, `<linearGradient id="%s" x1="%.4f%%" y1="%.4f%%" x2="%.4f%%" y2="%.4f%%">`, escapeAttr(id), x1, y1, x2, y2)
	}
	for _, stop := range gradient.Stops {
		// Alpha has already been canonicalized by physical layout. In
		// particular, zero means fully transparent and must not be rewritten as
		// opaque by the renderer.
		alpha := math.Max(0, math.Min(1, stop.Alpha))
		fmt.Fprintf(&b, `<stop offset="%.4f%%" stop-color="%s"`, stop.Position*100, escapeAttr(svgPaintColor(stop.Color)))
		if alpha < 1 {
			fmt.Fprintf(&b, ` stop-opacity="%.4f"`, alpha)
		}
		b.WriteString(`/>`)
	}
	if gradient.Kind == drawing.GradientTypeRadial || gradient.Kind == drawing.GradientTypePath {
		b.WriteString(`</radialGradient>`)
	} else {
		b.WriteString(`</linearGradient>`)
	}
	return b.String()
}

func svgShapePathGradientDefinition(gradient *playout.GradientVisualPlan, id string) string {
	if gradient == nil || gradient.PathBoundaryPath == nil || id == "" || len(gradient.Stops) == 0 || gradient.PathBoundaryWidthPt <= 0 || gradient.PathBoundaryHeightPt <= 0 {
		return ""
	}
	// SVG has no native path-gradient primitive. Materialize OfficeArt ShadeShape
	// as nested copies of the actual shape boundary, contracting toward the
	// authored focus point. This preserves arbitrary preset/freeform outlines
	// instead of replacing them with an unrelated radial ellipse.
	const layers = 128
	cx, cy := .5, .5
	if gradient.HasCenter {
		cx, cy = gradient.CenterX, gradient.CenterY
	}
	w, h := gradient.PathBoundaryWidthPt, gradient.PathBoundaryHeightPt
	// fillTo* is an inner rectangle in normalized shape coordinates.  For an
	// arbitrary ShadeShape boundary, materialize each contour with the affine
	// map that takes the original shape box to an interpolated box between the
	// outer boundary and that authored inner rectangle.  The point-focus case
	// is simply a zero-sized inner box.  This keeps the actual freeform/preset
	// outline at every contour instead of falling back to radial geometry.
	cw, ch := gradient.CenterWidth, gradient.CenterHeight
	left, top := cx-cw/2, cy-ch/2
	pathData := svgPathData(*gradient.PathBoundaryPath)
	if strings.TrimSpace(pathData) == "" {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<pattern id="%s" patternUnits="objectBoundingBox" patternContentUnits="userSpaceOnUse" width="1" height="1" viewBox="0 0 %.6f %.6f" preserveAspectRatio="none">`, escapeAttr(id), w, h)
	for i := 0; i < layers; i++ {
		t := float64(i) / float64(layers-1) // outline -> inner fillTo box
		sx := math.Max(0, 1-t+t*cw)
		sy := math.Max(0, 1-t+t*ch)
		tx := t * left * w
		ty := t * top * h
		color, alpha, ok := samplePhysicalGradient(gradient.Stops, 1-t)
		if !ok {
			return ""
		}
		fmt.Fprintf(&b, `<path d=%q transform="matrix(%.8f 0 0 %.8f %.6f %.6f)" fill="%s"`, pathData, sx, sy, tx, ty, escapeAttr(color))
		if alpha < 1 {
			fmt.Fprintf(&b, ` fill-opacity="%.4f"`, alpha)
		}
		b.WriteString(`/>`)
	}
	b.WriteString(`</pattern>`)
	return b.String()
}

func svgRectangularPathGradientDefinition(gradient *playout.GradientVisualPlan, id string) string {
	if gradient == nil || id == "" || len(gradient.Stops) == 0 {
		return ""
	}
	// OfficeArt ShadeShape is a path gradient: the last gradient position lies
	// on the shape outline and the first lies at the interior focus/center
	// rectangle. SVG 1.1 has no path-gradient primitive. For rectangular
	// boundaries, however, the authored geometry can be materialized exactly as
	// nested rectangular contours converging on the OfficeArt fillTo rectangle.
	// The layers are deliberately emitted in object-bounding-box coordinates so
	// the definition remains independent of the physical shape size.
	const layers = 128
	cx, cy := .5, .5
	cw, ch := 0.0, 0.0
	if gradient.HasCenter {
		cx, cy = gradient.CenterX, gradient.CenterY
		cw, ch = gradient.CenterWidth, gradient.CenterHeight
	}
	left, top := cx-cw/2, cy-ch/2
	right, bottom := cx+cw/2, cy+ch/2
	left = math.Max(0, math.Min(1, left))
	top = math.Max(0, math.Min(1, top))
	right = math.Max(left, math.Min(1, right))
	bottom = math.Max(top, math.Min(1, bottom))

	var b strings.Builder
	fmt.Fprintf(&b, `<pattern id="%s" patternUnits="objectBoundingBox" patternContentUnits="objectBoundingBox" width="1" height="1">`, escapeAttr(id))
	for i := 0; i < layers; i++ {
		t := float64(i) / float64(layers-1) // outline -> center
		x := t * left
		y := t * top
		x2 := 1 - t*(1-right)
		y2 := 1 - t*(1-bottom)
		w, h := math.Max(0, x2-x), math.Max(0, y2-y)
		color, alpha, ok := samplePhysicalGradient(gradient.Stops, 1-t)
		if !ok {
			return ""
		}
		fmt.Fprintf(&b, `<rect x="%.6f" y="%.6f" width="%.6f" height="%.6f" fill="%s"`, x, y, w, h, escapeAttr(color))
		if alpha < 1 {
			fmt.Fprintf(&b, ` fill-opacity="%.4f"`, alpha)
		}
		b.WriteString(`/>`)
	}
	b.WriteString(`</pattern>`)
	return b.String()
}

func samplePhysicalGradient(stops []playout.GradientStopVisualPlan, position float64) (string, float64, bool) {
	if len(stops) == 0 {
		return "", 1, false
	}
	position = math.Max(0, math.Min(1, position))
	lo, hi := stops[0], stops[len(stops)-1]
	if position <= lo.Position {
		return normalizedPhysicalGradientColor(lo.Color, lo.Alpha)
	}
	if position >= hi.Position {
		return normalizedPhysicalGradientColor(hi.Color, hi.Alpha)
	}
	for i := 1; i < len(stops); i++ {
		if position > stops[i].Position {
			continue
		}
		lo, hi = stops[i-1], stops[i]
		span := hi.Position - lo.Position
		t := 0.0
		if span > 0 {
			t = (position - lo.Position) / span
		}
		lr, lg, lb, lok := parsePhysicalGradientRGB(lo.Color)
		hr, hg, hb, hok := parsePhysicalGradientRGB(hi.Color)
		if !lok || !hok {
			return "", 1, false
		}
		r := math.Round(float64(lr) + (float64(hr)-float64(lr))*t)
		g := math.Round(float64(lg) + (float64(hg)-float64(lg))*t)
		bl := math.Round(float64(lb) + (float64(hb)-float64(lb))*t)
		la, ha := normalizedPhysicalGradientAlpha(lo.Alpha), normalizedPhysicalGradientAlpha(hi.Alpha)
		return fmt.Sprintf("#%02X%02X%02X", int(r), int(g), int(bl)), la + (ha-la)*t, true
	}
	return normalizedPhysicalGradientColor(hi.Color, hi.Alpha)
}

func normalizedPhysicalGradientColor(color string, alpha float64) (string, float64, bool) {
	r, g, b, ok := parsePhysicalGradientRGB(color)
	if !ok {
		return "", 1, false
	}
	return fmt.Sprintf("#%02X%02X%02X", r, g, b), normalizedPhysicalGradientAlpha(alpha), true
}

func normalizedPhysicalGradientAlpha(alpha float64) float64 {
	if alpha <= 0 || alpha > 1 {
		return 1
	}
	return alpha
}

func parsePhysicalGradientRGB(color string) (int, int, int, bool) {
	color = strings.TrimPrefix(strings.TrimSpace(color), "#")
	if len(color) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(color, 16, 24)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v >> 16), int((v >> 8) & 0xff), int(v & 0xff), true
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

func imageEffectPlanNames(effects []playout.ImageEffectVisualPlan) string {
	if len(effects) == 0 {
		return ""
	}
	names := make([]string, 0, len(effects))
	for _, effect := range effects {
		if effect.Type != "" {
			names = append(names, effect.Type)
		}
	}
	return strings.Join(names, " ")
}

func imageEffectPlanFilterDefinition(visual playout.ImageVisualPlan, id string) string {
	effects := visual.Effects
	if len(effects) == 0 || id == "" {
		return ""
	}
	var primitives []string
	input := "SourceGraphic"
	for _, e := range effects {
		switch e.Type {
		case string(drawing.EffectTypeGrayscale), string(drawing.EffectTypeDeSat):
			result := fmt.Sprintf("fx%d", len(primitives)+1)
			primitives = append(primitives, fmt.Sprintf(`<feColorMatrix in="%s" type="saturate" values="0" result="%s"/>`, input, result))
			input = result
		case string(drawing.EffectTypeBiLevel):
			input = appendBiLevelFilter(&primitives, input, e.Int1)
		case string(drawing.EffectTypeDuotone):
			input = appendDuotoneFilter(&primitives, input, e.Color1, e.Color2)
		case string(drawing.EffectTypeLum):
			input = appendLuminanceFilter(&primitives, input, e.Int1, e.Int2)
		case string(drawing.EffectTypeHSLEffect):
			input = appendHSLFilterValues(&primitives, input, e.Int1, e.Int2, e.Int3)
		case string(drawing.EffectTypeDeSatMod):
			result := fmt.Sprintf("fx%d", len(primitives)+1)
			amount := clampFloat64(float64(e.Int1)/100000.0, 0, 1)
			primitives = append(primitives, fmt.Sprintf(`<feColorMatrix in="%s" type="saturate" values="%.6f" result="%s"/>`, input, amount, result))
			input = result
		case string(drawing.EffectTypeAlphaMod), string(drawing.EffectTypeAlphaModFix):
			input = appendAlphaLinearFilter(&primitives, input, float64(e.Int1)/100000.0, 0)
		case string(drawing.EffectTypeAlphaOff):
			input = appendAlphaLinearFilter(&primitives, input, 1, float64(e.Int1)/100000.0)
		case string(drawing.EffectTypeAlphaRepl):
			input = appendAlphaLinearFilter(&primitives, input, 0, float64(e.Int1)/100000.0)
		case string(drawing.EffectTypeAlphaInv):
			input = appendAlphaInvertFilter(&primitives, input)
		case string(drawing.EffectTypeAlphaBiLevel):
			input = appendAlphaThresholdFilter(&primitives, input, float64(e.Int1)/100000.0)
		case string(drawing.EffectTypeAlphaCeiling):
			input = appendAlphaCeilingFilter(&primitives, input)
		case string(drawing.EffectTypeAlphaFloor):
			input = appendAlphaFloorFilter(&primitives, input)
		case string(drawing.EffectTypeAlphaOutset):
			input = appendAlphaOutsetFilter(&primitives, input, e.Float1)
		case string(drawing.EffectTypeClrRepl):
			input = appendColorReplacementFilter(&primitives, input, e.Color1)
		case string(drawing.EffectTypeClrChange):
			input = appendColorChangeFilterValues(&primitives, input, e.Color1, e.Color2, e.Float1, e.Float2, e.Bool1)
		}
	}
	if len(primitives) == 0 {
		return ""
	}
	x, y, w, h := visual.FilterXPct, visual.FilterYPct, visual.FilterWidthPct, visual.FilterHeightPct
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 100
	}
	return fmt.Sprintf(`<filter id="%s" x="%.4f%%" y="%.4f%%" width="%.4f%%" height="%.4f%%" color-interpolation-filters="sRGB">%s</filter>`, escapeAttr(id), x, y, w, h, strings.Join(primitives, ""))
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
	// Raw-effect fallback has no physical ImageVisualPlan. Keep its conservative
	// viewport here; the normal picture path uses imageEffectPlanFilterDefinition,
	// whose viewport is owned by layout and materialized on ImageVisualPlan.
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
	return appendHSLFilterValues(primitives, input, effect.Hue, effect.Sat, effect.Lum)
}

func appendHSLFilterValues(primitives *[]string, input string, hue, sat, lum int) string {
	effect := drawing.HSLEffect{Hue: hue, Sat: sat, Lum: lum}
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
	return appendColorChangeFilterValues(primitives, input, effect.From, effect.To, effect.FromAlpha, effect.ToAlpha, effect.UseAlpha)
}

func appendColorChangeFilterValues(primitives *[]string, input, from, to string, fromAlpha, toAlpha float64, useAlpha bool) string {
	effect := drawing.ClrChange{From: from, To: to, FromAlpha: fromAlpha, ToAlpha: toAlpha, UseAlpha: useAlpha}
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
func collectStrokeFillDefsPlans(primitives []playout.VectorPrimitivePlan, prefix string) []string {
	var defs []string
	for i := range primitives {
		primitive := &primitives[i].Primitive
		if primitive.Style.Stroke != nil && primitive.Style.Stroke.Fill != nil && drawingcss.NeedsSVGDefinition(primitive.Style.Stroke.Fill) {
			id := fmt.Sprintf("%s-stroke-%d", prefix, len(defs))
			primitive.Style.StrokeFillDefID = id
			defs = append(defs, svgDrawingFillDefinition(primitive.Style.Stroke.Fill, id))
		}
	}
	return defs
}

func rectShapeStyles(visual playout.ShapeVisualPlan) []string {
	var styles []string
	styles = append(styles,
		"position:absolute",
		"inset:0",
	)
	if visual.HasVisibleFill {
		styles = append(styles, cssBackgroundFillPropsForFill(visual.Fill.Fill, visual.Fill.FallbackColor)...)
	}
	stroke := visual.Stroke
	if stroke.Color != "" && stroke.WidthPt > 0 {
		styles = append(styles, fmt.Sprintf("border:%.2fpt %s #%s", stroke.WidthPt, stroke.CSSStyle, stroke.Color))
	}
	return styles
}

func presentationEffectPlanStyles(effects playout.EffectVisualPlan) map[string]string {
	if !effects.HasEffects {
		return nil
	}
	props := make(map[string]string)
	composition := effects.Composition
	var transforms []string
	for _, step := range composition.Transforms {
		if value := effectTransformStepCSS(step); value != "" {
			transforms = append(transforms, value)
		}
	}
	if mode := cssBlendMode(composition.BlendMode); mode != "" && composition.BlendMode != "" {
		props["mix-blend-mode"] = mode
	}
	if composition.Reflection != nil {
		applyReflectionVisualCSS(props, *composition.Reflection)
	}
	if composition.ThreeD != nil {
		applyThreeDVisualCSS(props, *composition.ThreeD)
	}

	// Presentation effects belong to the painted shape, never to the rectangular
	// DOM positioning frame. CSS box-shadow follows that frame's border box and
	// therefore turns freeforms, connectors and other non-rectangular DrawingML
	// geometry into visible rectangles. drop-shadow instead follows the alpha of
	// the rendered descendants (SVG geometry, picture or text paint).
	var filters []string
	for _, shadow := range effects.Shadows {
		if shadow.Inner || shadow.Color == "" {
			continue
		}
		filters = append(filters, fmt.Sprintf("drop-shadow(%.3fpt %.3fpt %.3fpt %s)", shadow.OffsetXPt, shadow.OffsetYPt, shadow.BlurPt, physicalEffectColor(shadow.Color, shadow.Alpha)))
	}
	for _, glow := range effects.Glows {
		if glow.Color != "" && glow.RadiusPt > 0 {
			filters = append(filters, fmt.Sprintf("drop-shadow(0 0 %.3fpt %s)", glow.RadiusPt, physicalEffectColor(glow.Color, glow.Alpha)))
		}
	}
	for _, blur := range effects.Blurs {
		if blur.RadiusPt > 0 {
			filters = append(filters, fmt.Sprintf("blur(%.3fpt)", blur.RadiusPt))
		}
	}
	if len(filters) > 0 {
		props["filter"] = strings.Join(filters, " ")
	}
	if len(transforms) > 0 {
		base := strings.TrimSpace(props["transform"])
		parts := make([]string, 0, len(transforms)+1)
		if base != "" {
			parts = append(parts, base)
		}
		parts = append(parts, transforms...)
		props["transform"] = strings.Join(parts, " ")
		if strings.TrimSpace(props["transform-origin"]) == "" {
			props["transform-origin"] = "50% 50%"
		}
	}
	return props
}

func presentationPictureEffectPlanStyles(effects playout.EffectVisualPlan) map[string]string {
	return presentationEffectPlanStyles(effects)
}

// presentationTextShapeEffectPlanStyles uses the same geometry-aware shape
// effect contract as every other presentation visual. Text shapes are not a
// special case: the positioning wrapper never owns presentation paint.
func presentationTextShapeEffectPlanStyles(effects playout.EffectVisualPlan) map[string]string {
	return presentationEffectPlanStyles(effects)
}

func presentationTextEffectPlanStyles(effects playout.EffectVisualPlan) map[string]string {
	if !effects.HasEffects {
		return nil
	}
	props := presentationEffectPlanStyles(effects)
	delete(props, "box-shadow")
	var filters []string
	for _, shadow := range effects.Shadows {
		if shadow.Inner || shadow.Color == "" {
			continue
		}
		filters = append(filters, fmt.Sprintf("drop-shadow(%.3fpt %.3fpt %.3fpt %s)", shadow.OffsetXPt, shadow.OffsetYPt, shadow.BlurPt, physicalEffectColor(shadow.Color, shadow.Alpha)))
	}
	for _, glow := range effects.Glows {
		if glow.Color != "" && glow.RadiusPt > 0 {
			filters = append(filters, fmt.Sprintf("drop-shadow(0 0 %.3fpt %s)", glow.RadiusPt, physicalEffectColor(glow.Color, glow.Alpha)))
		}
	}
	for _, blur := range effects.Blurs {
		if blur.RadiusPt > 0 {
			filters = append(filters, fmt.Sprintf("blur(%.3fpt)", blur.RadiusPt))
		}
	}
	if len(filters) > 0 {
		props["filter"] = strings.Join(filters, " ")
	}
	return props
}

func svgPaintColor(color string) string {
	color = strings.TrimSpace(color)
	if color == "" || color == "none" || strings.HasPrefix(color, "#") || strings.HasPrefix(color, "rgb(") || strings.HasPrefix(color, "rgba(") || strings.HasPrefix(color, "url(") || strings.HasPrefix(color, "var(") {
		return color
	}
	if len(color) == 3 || len(color) == 6 || len(color) == 8 {
		for _, r := range color {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return color
			}
		}
		return "#" + color
	}
	return color
}

func physicalEffectColor(hex string, alpha float64) string {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if alpha <= 0 || alpha > 1 {
		alpha = 1
	}
	if len(hex) == 6 {
		if r, err := strconv.ParseUint(hex[0:2], 16, 8); err == nil {
			if g, err := strconv.ParseUint(hex[2:4], 16, 8); err == nil {
				if b, err := strconv.ParseUint(hex[4:6], 16, 8); err == nil {
					return fmt.Sprintf("rgba(%d,%d,%d,%.4f)", r, g, b, alpha)
				}
			}
		}
	}
	return "#" + hex
}

func effectTransformStepCSS(step playout.EffectTransformStep) string {
	switch step.Kind {
	case "offset":
		if step.X == 0 && step.Y == 0 {
			return ""
		}
		return fmt.Sprintf("translate(%.4f%%,%.4f%%)", step.X*100, step.Y*100)
	case "transform":
		var parts []string
		if step.X != 0 || step.Y != 0 {
			parts = append(parts, fmt.Sprintf("translate(%.4f%%,%.4f%%)", step.X*100, step.Y*100))
		}
		if step.ScaleX != 1 || step.ScaleY != 1 {
			parts = append(parts, fmt.Sprintf("scale(%.4f,%.4f)", step.ScaleX, step.ScaleY))
		}
		if step.SkewXDeg != 0 {
			parts = append(parts, fmt.Sprintf("skewX(%.4fdeg)", step.SkewXDeg))
		}
		if step.SkewYDeg != 0 {
			parts = append(parts, fmt.Sprintf("skewY(%.4fdeg)", step.SkewYDeg))
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
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

func applyThreeDVisualCSS(props map[string]string, t playout.ThreeDVisualPlan) {
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
	if t.RotationXDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotateX(%.4fdeg)", t.RotationXDeg))
	}
	if t.RotationYDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotateY(%.4fdeg)", -t.RotationYDeg))
	}
	if t.RotationZDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotateZ(%.4fdeg)", t.RotationZDeg))
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

func applyReflectionVisualCSS(props map[string]string, r playout.ReflectionVisualPlan) {
	if props == nil {
		return
	}
	direction := "below"
	a := math.Mod(r.DirectionDeg, 360)
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
		startAlpha = r.StartAlpha
	}
	if startAlpha <= 0 {
		startAlpha = 0.3
	}
	endAlpha := 0.0
	if r.EndAlpha > 0 {
		endAlpha = r.EndAlpha
	}
	startPos := r.StartPosition
	endPos := r.EndPosition
	if endPos <= startPos {
		startPos, endPos = 0, 100
	}
	mask := fmt.Sprintf("linear-gradient(to bottom,rgba(0,0,0,%.4f) %.2f%%,rgba(0,0,0,%.4f) %.2f%%)", startAlpha, startPos, endAlpha, endPos)
	props["-webkit-box-reflect"] = fmt.Sprintf("%s %.2fpt %s", direction, r.DistancePt, mask)
}

func writeHTMLFillOverlays(b *strings.Builder, effects playout.EffectVisualPlan) {
	if b == nil {
		return
	}
	for _, overlay := range effects.Overlays {
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
	return presentationEffectPlanStyles(playout.ResolveEffectVisualPlan(effects))
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
			// Drawing paths may compact several consecutive vertices into one
			// command. SVG accepts the same representation: additional points on
			// M are implicit line-tos and additional points on L are further
			// line-tos. Serializing only Points[0] silently collapsed presets such
			// as flowChartCollate and multi-vertex legacy custom geometry.
			var coords strings.Builder
			coords.WriteString(cmd.Type)
			for _, point := range cmd.Points {
				fmt.Fprintf(&coords, " %.4f %.4f", point.X, point.Y)
			}
			parts = append(parts, coords.String())
		case "C":
			// A canonical command may carry several consecutive cubic segments.
			// SVG permits all control/end-point triples after one C opcode. Dropping
			// everything after the first triple changes freeform OfficeArt curves.
			for i := 0; i+2 < len(cmd.Points); i += 3 {
				parts = append(parts, fmt.Sprintf("C %.4f %.4f %.4f %.4f %.4f %.4f",
					cmd.Points[i].X, cmd.Points[i].Y,
					cmd.Points[i+1].X, cmd.Points[i+1].Y,
					cmd.Points[i+2].X, cmd.Points[i+2].Y))
			}
		case "Q":
			for i := 0; i+1 < len(cmd.Points); i += 2 {
				parts = append(parts, fmt.Sprintf("Q %.4f %.4f %.4f %.4f",
					cmd.Points[i].X, cmd.Points[i].Y,
					cmd.Points[i+1].X, cmd.Points[i+1].Y))
			}
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

func appendImagePhysicalPaintStyles(styles []string, image playout.ImageVisualPlan) []string {
	if image.Composition.Paint.ZIndex > 0 {
		styles = append(styles, fmt.Sprintf("z-index:%d", image.Composition.Paint.ZIndex))
	}
	if image.Composition.Paint.OverflowVisible {
		styles = append(styles, "overflow:visible")
	}
	return styles
}

func appendMediaPhysicalPaintStyles(styles []string, media *playout.PhysicalMediaPlan) []string {
	if media == nil {
		return styles
	}
	if media.Paint.ZIndex > 0 {
		styles = append(styles, fmt.Sprintf("z-index:%d", media.Paint.ZIndex))
	}
	if media.Paint.OverflowVisible {
		styles = append(styles, "overflow:visible")
	}
	return styles
}

func appendOLEPhysicalPaintStyles(styles []string, ole *playout.PhysicalOLEPlan) []string {
	if ole == nil {
		return styles
	}
	if ole.Paint.ZIndex > 0 {
		styles = append(styles, fmt.Sprintf("z-index:%d", ole.Paint.ZIndex))
	}
	if ole.Paint.OverflowVisible {
		styles = append(styles, "overflow:visible")
	}
	return styles
}

func renderImageShape(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	shape := item.SourceShape
	imageVisual := *item.Image
	x, y, w, h := physicalBoundsPercent(imageVisual.Composition.FrameBounds, slideW, slideH)
	effectStyles := presentationPictureEffectPlanStyles(imageVisual.Composition.Effects)
	if len(imageVisual.Data) > 0 {
		if !browserRenderablePresentationImage(imageVisual.MIME) {
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
			styles = appendImagePhysicalPaintStyles(styles, imageVisual)
			fmt.Fprintf(b,
				"<div class=\"shape shape--image shape--image-placeholder\" style=%q%s%s%s%s aria-label=%q>[%s]</div>\n",
				strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape), escapeAttr(imageVisual.Alt), escapeHTML(imageVisual.MIME))
			return
		}
		src := imageVisualSource(ctx, imageVisual)
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
		// Picture outer effects are alpha-aware and run after geometry masking.
		var transforms []string
		if imageVisual.Composition.Transform.RotationDeg != 0 {
			transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", imageVisual.Composition.Transform.RotationDeg))
		}
		if imageVisual.Composition.Transform.FlipH {
			transforms = append(transforms, "scaleX(-1)")
		}
		if imageVisual.Composition.Transform.FlipV {
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
		clipStyles := imageClipStyles(imageVisual)
		frameStyles := imageFrameStyles(imageVisual)
		imgStyles := imageElementStyles(imageVisual)
		imageFilterID := fmt.Sprintf("picture-%d-blip-filter", shape.ID)
		if ctx != nil {
			ctx.nextEffectIndex++
			imageFilterID = fmt.Sprintf("picture-blip-filter-%d", ctx.nextEffectIndex)
		}
		imageFilterDef := imageEffectPlanFilterDefinition(imageVisual, imageFilterID)
		if imageFilterDef != "" {
			imgStyles = append(imgStyles, "filter:url(#"+imageFilterID+")")
		}
		imageEffectsAttr := ""
		if names := imageEffectPlanNames(imageVisual.Effects); names != "" {
			imageEffectsAttr = fmt.Sprintf(" data-ppt-image-effects=%q", names)
		}
		styles = appendImagePhysicalPaintStyles(styles, imageVisual)
		fmt.Fprintf(b, "<div class=\"shape shape--image\" style=%q%s%s%s%s>", strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape))
		if imageFilterDef != "" {
			fmt.Fprintf(b, "<svg width=\"0\" height=\"0\" aria-hidden=\"true\"><defs>%s</defs></svg>", imageFilterDef)
		}
		fmt.Fprintf(b, "<div class=\"shape__image-clip\" style=%q>", strings.Join(clipStyles, ";"))
		fmt.Fprintf(b, "<div class=\"shape__image-frame\" style=%q>", strings.Join(frameStyles, ";"))
		geometryAttr := fmt.Sprintf(" data-ppt-image-dpi=%q data-ppt-rot-with-shape=%q data-ppt-fill-rect=%q", strconv.Itoa(imageVisual.DPI), strconv.FormatBool(imageVisual.RotWithShape), fmt.Sprintf("%.5f,%.5f,%.5f,%.5f", imageVisual.FrameLeft/100, imageVisual.FrameTop/100, imageVisual.FrameRight/100, imageVisual.FrameBottom/100))
		fmt.Fprintf(b,
			"<img class=\"shape__image\" style=%q src=%q alt=%q%s%s>",
			strings.Join(imgStyles, ";"), src, escapeAttr(imageVisual.Alt), imageEffectsAttr, geometryAttr)
		b.WriteString("</div>")
		// Shape-local overlays stay inside the authored picture geometry mask.
		writeHTMLFillOverlays(b, imageVisual.Composition.Effects)
		b.WriteString("</div></div>\n")
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
		styles = appendImagePhysicalPaintStyles(styles, imageVisual)
		fmt.Fprintf(b,
			"<div class=\"shape shape--image\" style=%q%s%s%s%s aria-label=%q></div>\n",
			strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape), escapeAttr(shape.Name))
	}
}

func renderMediaShape(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	shape := item.SourceShape
	media := item.Media
	if media == nil {
		return
	}
	x, y, w, h := physicalBoundsPercent(media.FrameBounds, slideW, slideH)
	effectStyles := presentationEffectPlanStyles(media.Effects)
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
	if media.Transform.RotationDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", media.Transform.RotationDeg))
	}
	if media.Transform.FlipH {
		transforms = append(transforms, "scaleX(-1)")
	}
	if media.Transform.FlipV {
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
		styles = appendMediaPhysicalPaintStyles(styles, media)
		fmt.Fprintf(b, "<div class=\"shape shape--media shape--media-placeholder\" style=%q%s%s%s%s aria-label=%q>[%s]</div>\n",
			strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape), escapeAttr(shape.Name), escapeHTML(mediaLabel(shape, label)))
		return
	}

	styles = appendMediaPhysicalPaintStyles(styles, media)
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

func renderOleObjectShape(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	shape := item.SourceShape
	ole := item.OLE
	if ole == nil {
		return
	}
	x, y, w, h := physicalBoundsPercent(ole.FrameBounds, slideW, slideH)
	effectStyles := presentationEffectPlanStyles(ole.Effects)

	baseStyles := []string{
		"position:absolute",
		fmt.Sprintf("left:%.4f%%", x),
		fmt.Sprintf("top:%.4f%%", y),
		fmt.Sprintf("width:%.4f%%", w),
		fmt.Sprintf("height:%.4f%%", h),
	}
	if ole.MathML != "" {
		baseStyles = append(baseStyles, "overflow:visible")
		baseStyles = appendOLEPhysicalPaintStyles(baseStyles, ole)
		if color := oleMathForegroundColor(ole); color != "" {
			baseStyles = append(baseStyles, "color:"+color)
		}
		if ole.LinkURL != "" {
			fmt.Fprintf(b, "<a class=\"ole-object-link\" href=%q target=\"_blank\" rel=\"noopener noreferrer\">", escapeAttr(ole.LinkURL))
		}
		metricAttr := ""
		if dx, baselineY, centerY, ok := oleMathPreviewPlacement(ole); ok {
			metricAttr += fmt.Sprintf(` data-math-offset-x="%.6f" data-math-baseline-y="%.6f" data-math-center-y="%.6f"`, dx, baselineY, centerY)
		}
		if oleMathPreviewIsMultiline(ole) {
			metricAttr += ` data-math-multiline="1"`
		}
		preview := ""
		if strings.EqualFold(strings.TrimSpace(ole.IconMIME), "image/svg+xml") && len(ole.IconData) > 0 {
			preview = `<span class="ole-math-authored-preview" aria-hidden="true">` + string(ole.IconData) + `</span>`
		}
		fmt.Fprintf(b, "<div class=\"shape shape--ole-object shape--math shape--ole-math\" style=%q%s%s%s%s%s>%s<span class=\"ole-math-inner\">%s<span class=\"ole-math-baseline-marker\" aria-hidden=\"true\"></span></span></div>\n", strings.Join(baseStyles, ";"), metricAttr, animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape), preview, ole.MathML)
		if ole.LinkURL != "" {
			b.WriteString("</a>\n")
		}
		return
	}

	if len(ole.IconData) > 0 && browserRenderablePresentationImage(ole.IconMIME) {
		if len(effectStyles) > 0 {
			baseStyles = append(baseStyles, "overflow:visible")
		}
		var transforms []string
		if ole.Transform.RotationDeg != 0 {
			transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", ole.Transform.RotationDeg))
		}
		if ole.Transform.FlipH {
			transforms = append(transforms, "scaleX(-1)")
		}
		if ole.Transform.FlipV {
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
		baseStyles = appendOLEPhysicalPaintStyles(baseStyles, ole)
		if ole.LinkURL != "" {
			fmt.Fprintf(b, "<a class=\"ole-object-link\" href=%q target=\"_blank\" rel=\"noopener noreferrer\">", escapeAttr(ole.LinkURL))
		}
		if strings.EqualFold(strings.TrimSpace(ole.IconMIME), "image/svg+xml") {
			fmt.Fprintf(b, "<div class=\"shape shape--ole-object shape--ole-object-preview\" style=%q%s%s%s%s aria-label=%q><div class=\"shape--ole-object-preview-svg\">%s</div></div>\n",
				strings.Join(baseStyles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape), escapeAttr(shape.Name), oleInlineSVGPreviewMarkup(ole.IconData))
		} else {
			src := oleObjectIconSource(ctx, ole)
			fmt.Fprintf(b, "<img class=\"shape shape--ole-object\" style=%q src=%q alt=%q%s%s%s%s>\n",
				strings.Join(baseStyles, ";"), src, escapeAttr(shape.Name), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape))
		}
		if ole.LinkURL != "" {
			b.WriteString("</a>\n")
		}
		return
	}
	// Equation Editor objects have no meaningful generic icon. When neither the
	// native MTEF payload nor a cached presentation image is available, the OLE
	// record is metadata only; drawing a fallback badge creates a false overlay.
	if isEquationOLE(ole, shape) {
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
	var transforms []string
	if ole.Transform.RotationDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", ole.Transform.RotationDeg))
	}
	if ole.Transform.FlipH {
		transforms = append(transforms, "scaleX(-1)")
	}
	if ole.Transform.FlipV {
		transforms = append(transforms, "scaleY(-1)")
	}
	if len(effectStyles) > 0 {
		styles = append(styles, "overflow:visible")
	}
	transform, origin := mergeEffectTransform(strings.Join(transforms, " "), effectStyles)
	if transform != "" {
		styles = append(styles, "transform:"+transform)
	}
	if origin != "" {
		styles = append(styles, "transform-origin:"+origin)
	} else if transform != "" {
		styles = append(styles, "transform-origin:50% 50%")
	}
	if len(effectStyles) > 0 {
		styles = appendSortedCSSProps(styles, effectStyles)
	}
	label := shape.OLEObjectName
	if label == "" {
		label = shape.OLEObjectProgID
	}
	if label == "" {
		label = "Object"
	}
	if ole.LinkURL != "" {
		fmt.Fprintf(b, "<a class=\"ole-object-link\" href=%q target=\"_blank\" rel=\"noopener noreferrer\">", escapeAttr(ole.LinkURL))
	}
	styles = appendOLEPhysicalPaintStyles(styles, ole)
	fmt.Fprintf(b,
		"<div class=\"shape shape--ole-object\" style=%q%s%s%s%s aria-label=%q>[%s]</div>\n",
		strings.Join(styles, ";"), animationAttrs(shape, ctx), emphasisEffectsAttr(shape), motionPathAttr(shape), triggerAttr(shape), escapeAttr(shape.Name), escapeHTML(label))
	if ole.LinkURL != "" {
		b.WriteString("</a>\n")
	}
}

func isEquationOLE(ole *playout.PhysicalOLEPlan, shape *ir.Shape) bool {
	if ole != nil && strings.Contains(strings.ToLower(ole.ProgID), "equation") {
		return true
	}
	if shape == nil {
		return false
	}
	return strings.Contains(strings.ToLower(shape.OLEObjectProgID), "equation") ||
		strings.Contains(strings.ToLower(shape.OLEObjectName), "equation")
}

func oleMathPreviewPlacement(ole *playout.PhysicalOLEPlan) (float64, float64, float64, bool) {
	if ole == nil || !strings.EqualFold(strings.TrimSpace(ole.IconMIME), "image/svg+xml") || len(ole.IconData) == 0 {
		return 0, 0, 0, false
	}
	svg := string(ole.IconData)
	start := strings.Index(svg, "<svg")
	if start < 0 {
		return 0, 0, 0, false
	}
	end := strings.IndexByte(svg[start:], '>')
	if end < 0 {
		return 0, 0, 0, false
	}
	vb := strings.Fields(svgAttributeValue(svg[start:start+end], "viewBox"))
	if len(vb) != 4 {
		return 0, 0, 0, false
	}
	vx, e0 := strconv.ParseFloat(vb[0], 64)
	vy, e1 := strconv.ParseFloat(vb[1], 64)
	vw, e2 := strconv.ParseFloat(vb[2], 64)
	vh, e3 := strconv.ParseFloat(vb[3], 64)
	if e0 != nil || e1 != nil || e2 != nil || e3 != nil || vw <= 0 || vh <= 0 {
		return 0, 0, 0, false
	}

	minX, maxX := 0.0, 0.0
	minY, maxY := 0.0, 0.0
	have := false
	maxFS := 0.0
	var baselineSum float64
	var baselineCount int
	// The metafile-to-SVG path writes GDI text y as its baseline.  Preserve the
	// dominant authored baseline rather than the preview ink-box centre. Scripts
	// have a smaller font size and therefore do not pull the equation vertically.
	for from := 0; ; {
		i := strings.Index(svg[from:], "<text")
		if i < 0 {
			break
		}
		i += from
		j := strings.IndexByte(svg[i:], '>')
		if j < 0 {
			break
		}
		tagEnd := i + j
		tag := svg[i:tagEnd]
		x, ex := strconv.ParseFloat(svgAttributeValue(tag, "x"), 64)
		y, ey := strconv.ParseFloat(svgAttributeValue(tag, "y"), 64)
		fs, ef := strconv.ParseFloat(svgAttributeValue(tag, "font-size"), 64)
		if ex == nil && ey == nil && ef == nil && fs > 0 {
			w := fs * 0.55
			if close := strings.Index(svg[tagEnd+1:], "</text>"); close >= 0 {
				if n := len([]rune(strings.TrimSpace(svg[tagEnd+1 : tagEnd+1+close]))); n > 0 {
					w *= float64(n)
				}
			}
			if !have {
				minX, maxX, have = x, x+w, true
				minY, maxY = y-fs*.8, y+fs*.2
			} else {
				if x < minX {
					minX = x
				}
				if x+w > maxX {
					maxX = x + w
				}
				minY = math.Min(minY, y-fs*.8)
				maxY = math.Max(maxY, y+fs*.2)
			}
			if fs > maxFS+0.01 {
				maxFS, baselineSum, baselineCount = fs, y, 1
			} else if math.Abs(fs-maxFS) <= 0.01 {
				baselineSum += y
				baselineCount++
			}
		}
		from = tagEnd + 1
	}
	if !have || baselineCount == 0 {
		return 0, 0, 0, false
	}
	baseline := baselineSum / float64(baselineCount)
	// The formula follows text in an authored OLE gap, so its horizontal
	// placement is determined by the preview's left ink origin—not its centre.
	// Centre-derived offsets pull narrow formulas back over the preceding text.
	return (minX - vx) / vw, (baseline - vy) / vh, ((minY+maxY)/2 - vy) / vh, true
}

func oleMathPreviewIsMultiline(ole *playout.PhysicalOLEPlan) bool {
	if ole == nil || !strings.EqualFold(strings.TrimSpace(ole.IconMIME), "image/svg+xml") || len(ole.IconData) == 0 {
		return false
	}
	svg := string(ole.IconData)
	maxFS := 0.0
	var ys []float64
	for from := 0; ; {
		i := strings.Index(svg[from:], "<text")
		if i < 0 {
			break
		}
		i += from
		j := strings.IndexByte(svg[i:], '>')
		if j < 0 {
			break
		}
		tagEnd := i + j
		tag := svg[i:tagEnd]
		y, ey := strconv.ParseFloat(svgAttributeValue(tag, "y"), 64)
		fs, ef := strconv.ParseFloat(svgAttributeValue(tag, "font-size"), 64)
		if ey == nil && ef == nil && fs > 0 {
			if fs > maxFS+0.01 {
				maxFS, ys = fs, []float64{y}
			} else if math.Abs(fs-maxFS) <= 0.01 {
				ys = append(ys, y)
			}
		}
		from = tagEnd + 1
	}
	if len(ys) < 2 {
		return false
	}
	minY, maxY := ys[0], ys[0]
	for _, y := range ys[1:] {
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	return maxY-minY > maxFS*0.5
}

func oleMathForegroundColor(ole *playout.PhysicalOLEPlan) string {
	if ole == nil {
		return ""
	}
	// Legacy PPT can recolor an OLE preview at the OfficeArt picture-shape
	// level. The cached WMF/EMF remains black, so inspecting converted SVG
	// paint alone loses the authored equation color. Explicit recolor wins.
	if c := cssOLEMathColor(ole.MathColor); c != "" {
		return c
	}
	if !strings.EqualFold(strings.TrimSpace(ole.IconMIME), "image/svg+xml") {
		return ""
	}
	svg := string(ole.IconData)
	// Text is the strongest signal when the metafile converter retained text.
	// Many MathType previews are outlined to paths, however, so fall back to
	// the first authored path/line paint rather than silently rendering black.
	// Retained SVG text is authoritative because it represents actual glyphs.
	if c := firstOLEMathSVGPaint(svg, "text"); c != "" {
		return c
	}
	// Outlined previews may contain bookkeeping/clipping paths before the glyph
	// outlines.  Choosing the first path therefore makes equation colour depend
	// on converter serialization order.  Use the dominant authored glyph paint
	// across drawable primitives instead.
	return dominantOLEMathSVGPaint(svg, []string{"path", "polyline", "polygon", "line"})
}

func cssOLEMathColor(c string) string {
	c = strings.TrimSpace(c)
	if len(c) == 6 {
		for _, r := range c {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return c
			}
		}
		return "#" + c
	}
	return c
}

func dominantOLEMathSVGPaint(svg string, names []string) string {
	counts := map[string]int{}
	order := []string{}
	for _, name := range names {
		needle := "<" + name
		for from := 0; ; {
			i := strings.Index(svg[from:], needle)
			if i < 0 {
				break
			}
			i += from
			j := strings.IndexByte(svg[i:], '>')
			if j < 0 {
				break
			}
			tag := svg[i : i+j]
			c := ""
			for _, attr := range []string{"fill", "stroke"} {
				if v := svgAttributeValue(tag, attr); validOLEMathColor(v) {
					c = strings.TrimSpace(v)
					break
				}
			}
			if c == "" {
				if style := svgAttributeValue(tag, "style"); style != "" {
					for _, decl := range strings.Split(style, ";") {
						kv := strings.SplitN(decl, ":", 2)
						if len(kv) == 2 && (strings.TrimSpace(kv[0]) == "fill" || strings.TrimSpace(kv[0]) == "stroke") && validOLEMathColor(kv[1]) {
							c = strings.TrimSpace(kv[1])
							break
						}
					}
				}
			}
			if c != "" {
				key := strings.ToLower(c)
				if counts[key] == 0 {
					order = append(order, key)
				}
				counts[key]++
			}
			from = i + j + 1
		}
	}
	best, bestN := "", 0
	for _, c := range order {
		if counts[c] > bestN {
			best, bestN = c, counts[c]
		}
	}
	return best
}

func firstOLEMathSVGPaint(svg, name string) string {
	needle := "<" + name
	for from := 0; ; {
		i := strings.Index(svg[from:], needle)
		if i < 0 {
			return ""
		}
		i += from
		j := strings.IndexByte(svg[i:], '>')
		if j < 0 {
			return ""
		}
		tag := svg[i : i+j]
		for _, attr := range []string{"fill", "stroke"} {
			if c := svgAttributeValue(tag, attr); validOLEMathColor(c) {
				return c
			}
		}
		if style := svgAttributeValue(tag, "style"); style != "" {
			for _, decl := range strings.Split(style, ";") {
				kv := strings.SplitN(decl, ":", 2)
				if len(kv) == 2 && (strings.TrimSpace(kv[0]) == "fill" || strings.TrimSpace(kv[0]) == "stroke") {
					if c := strings.TrimSpace(kv[1]); validOLEMathColor(c) {
						return c
					}
				}
			}
		}
		from = i + j + 1
	}
}
func svgAttributeValue(tag, name string) string {
	// SVG emitted by the metafile converters is not required to use one quote
	// style.  Accept both forms so MathType preview paint is not lost merely
	// because a converter serialized fill='white' instead of fill="white".
	for _, quote := range []byte{'"', '\''} {
		needle := name + "=" + string(quote)
		i := strings.Index(tag, needle)
		if i < 0 {
			continue
		}
		start := i + len(needle)
		end := strings.IndexByte(tag[start:], quote)
		if end >= 0 {
			return tag[start : start+end]
		}
	}
	return ""
}
func validOLEMathColor(c string) bool {
	c = strings.TrimSpace(strings.ToLower(c))
	if c == "" || c == "none" || c == "transparent" || c == "currentcolor" || strings.HasPrefix(c, "url(") {
		return false
	}
	if strings.HasPrefix(c, "#") || strings.HasPrefix(c, "rgb(") || strings.HasPrefix(c, "rgba(") ||
		strings.HasPrefix(c, "hsl(") || strings.HasPrefix(c, "hsla(") {
		return true
	}
	// CSS/SVG named colors (white, yellow, black, etc.) are common in
	// converter output.  They are safe to preserve verbatim as a CSS color.
	for _, r := range c {
		if (r < 'a' || r > 'z') && r != '-' {
			return false
		}
	}
	return true
}

func oleObjectIconSource(ctx *htmlRenderContext, ole *playout.PhysicalOLEPlan) string {
	if ole == nil || len(ole.IconData) == 0 {
		return ""
	}
	mime := ole.IconMIME
	if mime == "" {
		mime = "image/jpeg"
	}
	if ctx != nil && ctx.options.ImageMode == HTMLImageModeExternal {
		ctx.nextImageIndex++
		assetPath := htmlasset.AssetPath(ctx.options.ExternalImageDir, "ole-icon", ctx.nextImageIndex, htmlasset.ImageFileExtension(mime))
		ctx.assets = append(ctx.assets, HTMLAsset{
			Path:        assetPath,
			ContentType: mime,
			Data:        append([]byte(nil), ole.IconData...),
		})
		return assetPath
	}
	encoded := base64.StdEncoding.EncodeToString(ole.IconData)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded)
}

func oleInlineSVGPreviewMarkup(data []byte) string {
	markup := string(data)
	start := strings.Index(markup, "<svg")
	if start < 0 {
		return markup
	}
	end := strings.Index(markup[start:], ">")
	if end < 0 {
		return markup
	}
	end += start
	if strings.Contains(markup[start:end], "preserveAspectRatio=") {
		return markup
	}
	return markup[:end] + ` preserveAspectRatio="xMidYMid meet"` + markup[end:]
}

func imageVisualSource(ctx *htmlRenderContext, image playout.ImageVisualPlan) string {
	mime := image.MIME
	if mime == "" {
		mime = "image/jpeg"
	}
	if ctx != nil && ctx.options.ImageMode == HTMLImageModeExternal {
		return ctx.addImageArtifact(image.Data, mime)
	}
	encoded := base64.StdEncoding.EncodeToString(image.Data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded)
}

func imageSource(ctx *htmlRenderContext, shape *ir.Shape) string {
	mime := imageContentType(shape)
	if ctx != nil && ctx.options.ImageMode == HTMLImageModeExternal {
		return ctx.addImageAsset(shape, mime)
	}
	encoded := base64.StdEncoding.EncodeToString(shape.ImageData)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded)
}

func mediaSource(ctx *htmlRenderContext, shape *ir.Shape) string {
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

func mediaPosterSource(ctx *htmlRenderContext, shape *ir.Shape) string {
	if shape == nil || len(shape.MediaPosterData) == 0 {
		return ""
	}
	if ctx != nil && ctx.options.ImageMode == HTMLImageModeExternal {
		return ctx.addMediaPosterAsset(shape, mediaPosterContentType(shape))
	}
	encoded := base64.StdEncoding.EncodeToString(shape.MediaPosterData)
	return fmt.Sprintf("data:%s;base64,%s", mediaPosterContentType(shape), encoded)
}

func imageContentType(shape *ir.Shape) string {
	if shape != nil && shape.ImageMIME != "" {
		return shape.ImageMIME
	}
	return "image/jpeg"
}

func mediaContentType(shape *ir.Shape) string {
	if shape != nil && shape.MediaMIME != "" {
		return shape.MediaMIME
	}
	return "application/octet-stream"
}

func mediaPosterContentType(shape *ir.Shape) string {
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

func (ctx *htmlRenderContext) addImageArtifact(data []byte, mime string) string {
	ctx.nextImageIndex++
	assetPath := htmlasset.AssetPath(ctx.options.ExternalImageDir, "image", ctx.nextImageIndex, htmlasset.ImageFileExtension(mime))
	ctx.assets = append(ctx.assets, HTMLAsset{
		Path:        assetPath,
		ContentType: mime,
		Data:        append([]byte(nil), data...),
	})
	return assetPath
}

func (ctx *htmlRenderContext) addImageAsset(shape *ir.Shape, mime string) string {
	ctx.nextImageIndex++
	assetPath := htmlasset.AssetPath(ctx.options.ExternalImageDir, "image", ctx.nextImageIndex, htmlasset.ImageFileExtension(mime))
	ctx.assets = append(ctx.assets, HTMLAsset{
		Path:        assetPath,
		ContentType: mime,
		Data:        append([]byte(nil), shape.ImageData...),
	})
	return assetPath
}

func (ctx *htmlRenderContext) addMediaAsset(shape *ir.Shape, mime string) string {
	ctx.nextMediaIndex++
	assetPath := htmlasset.AssetPath(ctx.options.ExternalImageDir, "media", ctx.nextMediaIndex, htmlasset.MediaFileExtension(mime))
	ctx.assets = append(ctx.assets, HTMLAsset{
		Path:        assetPath,
		ContentType: mime,
		Data:        append([]byte(nil), shape.MediaData...),
	})
	return assetPath
}

func (ctx *htmlRenderContext) addMediaPosterAsset(shape *ir.Shape, mime string) string {
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

func imageClipStyles(visual playout.ImageVisualPlan) []string {
	styles := []string{"position:absolute", "inset:0", "overflow:hidden"}
	if visual.ClipPath != "" {
		styles = append(styles, "clip-path:"+visual.ClipPath)
	}
	var inner []string
	for _, shadow := range visual.Composition.Effects.Shadows {
		if !shadow.Inner || shadow.Color == "" {
			continue
		}
		inner = append(inner, fmt.Sprintf("inset %.3fpt %.3fpt %.3fpt %s", shadow.OffsetXPt, shadow.OffsetYPt, shadow.BlurPt, physicalEffectColor(shadow.Color, shadow.Alpha)))
	}
	if len(inner) > 0 {
		styles = append(styles, "box-shadow:"+strings.Join(inner, ","))
	}
	return styles
}

func imageFrameStyles(visual playout.ImageVisualPlan) []string {
	styles := []string{"position:absolute", "overflow:hidden"}
	if visual.FrameHidden {
		return append(styles, "display:none")
	}
	return append(styles,
		fmt.Sprintf("left:%.4f%%", visual.FrameLeft),
		fmt.Sprintf("top:%.4f%%", visual.FrameTop),
		fmt.Sprintf("right:%.4f%%", visual.FrameRight),
		fmt.Sprintf("bottom:%.4f%%", visual.FrameBottom),
	)
}

func imageElementStyles(visual playout.ImageVisualPlan) []string {
	styles := []string{"position:absolute", "display:block"}
	if visual.ImageHidden {
		return append(styles, "visibility:hidden")
	}
	styles = append(styles,
		fmt.Sprintf("width:%.4f%%", visual.ImageWidth),
		fmt.Sprintf("height:%.4f%%", visual.ImageHeight),
		fmt.Sprintf("left:%.4f%%", visual.ImageLeft),
		fmt.Sprintf("top:%.4f%%", visual.ImageTop),
	)
	if visual.CounterRotationDeg != 0 {
		styles = append(styles,
			fmt.Sprintf("transform:rotate(%.2fdeg)", visual.CounterRotationDeg),
			fmt.Sprintf("transform-origin:%.4f%% %.4f%%", visual.CounterRotationOriginX, visual.CounterRotationOriginY),
		)
	}
	return styles
}

func mediaTagName(shape *ir.Shape) string {
	if shape != nil && shape.MediaKind == "audio" {
		return "audio"
	}
	return "video"
}

func mediaLabel(shape *ir.Shape, fallback string) string {
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

// Empty SmartArt picture placeholders are rendered from their authoritative
// drawing geometry only. PowerPoint does not synthesize a generic image glyph
// into the presentation scene; such editor-only placeholder chrome must not be
// emitted into exported HTML.

// renderSmartArtShape renders a SmartArt diagram shape as HTML.
func renderSmartArtShape(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	shape := item.SourceShape
	if shape == nil {
		return
	}
	smartArt := item.SmartArt
	if smartArt == nil {
		return
	}
	x, y, w, h := physicalBoundsPercent(smartArt.FrameBounds, slideW, slideH)
	textPlan := *item.Text
	textFrame := textPlan.Frame
	frameLayout := textPlan.Layout
	textFlow := textPlan.Flow
	textBox := textPlan.Box
	textAutoGrow := textBox.ContentAutoHeight
	textAllowOverflow := textBox.OverflowVisible

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

	var transforms []string
	if smartArt.Transform.RotationDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", smartArt.Transform.RotationDeg))
	}
	if smartArt.Transform.FlipH {
		transforms = append(transforms, "scaleX(-1)")
	}
	if smartArt.Transform.FlipV {
		transforms = append(transforms, "scaleY(-1)")
	}
	effectStyles := presentationEffectPlanStyles(smartArt.Effects)
	transform, origin := mergeEffectTransform(strings.Join(transforms, " "), effectStyles)
	if transform != "" {
		styles = append(styles, "transform:"+transform)
	}
	if origin != "" {
		styles = append(styles, "transform-origin:"+origin)
	}
	styles = appendSortedCSSProps(styles, effectStyles)
	if smartArt.Paint.ZIndex > 0 {
		styles = append(styles, fmt.Sprintf("z-index:%d", smartArt.Paint.ZIndex))
	}

	visual := item.Drawing.Visual
	useSVG := visual.UseVector
	if useSVG {
		if textAutoGrow || textAllowOverflow {
			styles = append(styles, "overflow:visible")
		} else {
			styles = append(styles, "overflow:hidden")
		}
	} else {
		// Add fill using shared DrawingML types.
		styles = append(styles, cssBackgroundFillPropsForFill(visual.Fill.Fill, visual.Fill.FallbackColor)...)

		// Add border/line styling.
		stroke := visual.Stroke
		if stroke.Color != "" && stroke.WidthPt > 0 {
			styles = append(styles, fmt.Sprintf("border:%.2fpt %s #%s", stroke.WidthPt, stroke.CSSStyle, stroke.Color))
		}
		if textAutoGrow || textAllowOverflow {
			styles = append(styles, "overflow:visible")
		}
	}

	// Render as a div container
	if smartArt.Paint.OverflowVisible {
		styles = append(styles, "overflow:visible")
	}
	fmt.Fprintf(b, "<div class=\"shape shape--smartart\" style=%q%s>\n", strings.Join(styles, ";"), animationAttrs(shape, ctx))

	// Render non-rectangular geometry as an inline SVG.
	if useSVG {
		renderSmartArtVectorShape(b, item, nil, ctx)
	} else {
		writeHTMLFillOverlays(b, visual.Effects)
	}

	// SmartArt text uses the same physical TextVisualPlan as ordinary shapes.
	// The SmartArt plan remains the node-provenance gate, but paragraph wrapping,
	// frame placement and overflow are never recomputed in the serializer.
	if len(textPlan.Paragraphs) > 0 {
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
		// The shared TextBoxVisualPlan owns growth and overflow independently.
		// noAutofit may allow glyph overflow, but it never turns the authored
		// SmartArt text frame into height:auto. Doing so changes vertical
		// centering and makes text drift outside pre-rendered diagram nodes.
		if textBox.ContentAutoHeight {
			textStyles = append(textStyles, "height:auto")
		} else {
			textStyles = append(textStyles, "height:100%")
		}
		if textBox.OverflowVisible || frameLayout.Measured {
			textStyles = append(textStyles, "overflow:visible")
		} else if textBox.ClipContent {
			textStyles = append(textStyles, "overflow:hidden")
		}
		// SmartArt drawing shapes commonly rotate their geometry (for example,
		// a 180-degree homePlate to make a left-pointing banner) while the
		// generated text body remains upright in slide coordinates. Because the
		// HTML text is nested inside the transformed shape, cancel the node
		// rotation before applying any authored text-flow rotation.
		textRotation := textFlow.RotationDeg - smartArt.Transform.RotationDeg
		if textRotation != 0 {
			textStyles = append(textStyles, fmt.Sprintf("transform:rotate(%.2fdeg)", textRotation))
		}
		if textFlow.TransformOrigin != "" {
			textStyles = append(textStyles, "transform-origin:"+textFlow.TransformOrigin)
		}
		if textFlow.NoWrap {
			textStyles = append(textStyles, "white-space:nowrap")
		}
		if textFlow.WritingMode != "" {
			textStyles = append(textStyles, "writing-mode:"+textFlow.WritingMode)
		}
		if textFlow.Columns > 1 {
			textStyles = append(textStyles, fmt.Sprintf("column-count:%d", textFlow.Columns))
			if textFlow.ColumnGapPt > 0 {
				textStyles = append(textStyles, fmt.Sprintf("column-gap:%.2fpt", textFlow.ColumnGapPt))
			}
		}
		// SmartArt and ordinary text consume the same physical placement plan.
		// In particular, measured vertical anchoring is materialized exactly once
		// as a physical-axis offset; the serializer must not invent a SmartArt-only
		// center fallback or drop that offset.
		textStyles = appendTextBoxPlacementStyles(textStyles, textFrame, frameLayout, textBox)
		fmt.Fprintf(b, "<div style=%q>\n", strings.Join(textStyles, ";"))

		for i, p := range smartArt.Paragraphs {
			if i >= len(textPlan.Paragraphs) {
				break
			}
			renderParagraphWithPlan(b, p, textPlan.Paragraphs[i], textFrame.FontScale, ctx)
		}
		b.WriteString("</div>\n")
	}

	b.WriteString("</div>\n")
}

// appendTextBoxPlacementStyles serializes the layout-owned placement of a text
// body. It is deliberately shared by ordinary shapes and SmartArt so anchoring,
// effective geometry insets and physical-axis offsets cannot drift by renderer
// branch.
func appendTextBoxPlacementStyles(styles []string, frame playout.TextFrameMetrics, frameLayout playout.TextFrameLayout, box playout.TextBoxVisualPlan) []string {
	justify := frame.VerticalJustify
	if justify == "" {
		justify = "flex-start"
	}
	if frameLayout.Measured && justify != "space-between" && justify != "space-around" {
		justify = "flex-start"
	}
	styles = append(styles, "justify-content:"+justify)

	top := box.InsetTopPt + math.Max(0, box.ContentTopPt)
	right := box.InsetRightPt + math.Max(0, box.ContentRightPt)
	bottom := box.InsetBottomPt + math.Max(0, box.ContentBottomPt)
	left := box.InsetLeftPt + math.Max(0, box.ContentLeftPt)
	if top > 0 || right > 0 || bottom > 0 || left > 0 {
		styles = append(styles, fmt.Sprintf("padding:%.2fpt %.2fpt %.2fpt %.2fpt", top, right, bottom, left))
	}
	// Padding cannot represent a negative physical anchor. Overflowing centered
	// and end-anchored text therefore carries its signed progression offset as a
	// relative translation, while clipping remains owned by the dedicated
	// text-frame wrapper.
	if box.ContentTopPt < 0 {
		styles = append(styles, fmt.Sprintf("position:relative;top:%.2fpt", box.ContentTopPt))
	}
	if box.ContentLeftPt < 0 {
		styles = append(styles, fmt.Sprintf("position:relative;left:%.2fpt", box.ContentLeftPt))
	}
	if box.ContentRightPt < 0 {
		styles = append(styles, fmt.Sprintf("position:relative;right:%.2fpt", box.ContentRightPt))
	}
	return styles
}

// renderSmartArtVectorShape renders a SmartArt node's geometry as an inline SVG.
// It uses the shared preset-geometry primitive builder so any preset shape known
// to pkg/ooxml/drawing can be rendered, including ones with gradient or picture
// fills that need SVG definitions.
func renderSmartArtVectorShape(b *strings.Builder, item *playout.PhysicalVisual, effectStyles map[string]string, ctx *htmlRenderContext) {
	if b == nil || item == nil || item.SourceShape == nil {
		return
	}
	visual := item.Drawing.Visual
	renderCanonicalShapeVector(b, visual, effectStyles, shapeVectorRenderOptions{
		DefPrefix:          "smartart",
		WidthHeightAttr:    true,
		SVGOverflowVisible: item.Drawing.Paint.SVG.OverflowVisible,
	})
}

// renderSmartArtConnector draws a connector line between two SmartArt nodes,
// honoring line style, dash patterns, and arrowheads.
func renderSmartArtConnector(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	shape := item.SourceShape
	if shape == nil {
		return
	}
	if item == nil {
		return
	}
	if item.Connector != nil {
		renderPhysicalConnector(b, shape, slideW, slideH, ctx, *item.Connector)
		return
	}
	if item.FreeConnector != nil {
		renderPhysicalFreeConnector(b, shape, slideW, slideH, ctx, *item.FreeConnector)
	}
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

func smartArtArrowGeometrySVG(ag *drawing.ArrowGeometry, color string, alpha float64, alphaSet bool) string {
	if ag == nil {
		return ""
	}
	fillOpacity := ""
	strokeOpacity := ""
	if alphaSet && alpha < 1 {
		if alpha < 0 {
			alpha = 0
		}
		fillOpacity = fmt.Sprintf(` fill-opacity="%.4f"`, alpha)
		strokeOpacity = fmt.Sprintf(` stroke-opacity="%.4f"`, alpha)
	}
	switch ag.Type {
	case "triangle", "stealth", "diamond":
		return fmt.Sprintf(`<polygon points=%q fill="#%s"%s/>`, smartArtConnectorPointsString(ag.PolyPts), color, fillOpacity)
	case "arrow":
		pts := []drawing.Point{ag.Left, ag.Tip, ag.Right}
		return fmt.Sprintf(`<polyline points=%q fill="none" stroke="#%s" stroke-width="%.2f" stroke-linecap="round" stroke-linejoin="round"%s/>`, smartArtConnectorPointsString(pts), color, ag.StrokeWidth, strokeOpacity)
	case "oval":
		return fmt.Sprintf(`<ellipse cx="%.4f" cy="%.4f" rx="%.4f" ry="%.4f" fill="#%s"%s/>`, ag.Center.X, ag.Center.Y, ag.RX, ag.RY, color, fillOpacity)
	}
	return ""
}

// renderChartShape renders a chart shape as an SVG image in HTML.
func renderChartShape(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	shape := item.SourceShape // generic animation metadata only
	chart := item.Chart
	if chart == nil || len(chart.Data) == 0 {
		return
	}
	x, y, w, h := physicalBoundsPercent(chart.FrameBounds, slideW, slideH)
	styles := []string{"position:absolute", fmt.Sprintf("left:%.4f%%", x), fmt.Sprintf("top:%.4f%%", y), fmt.Sprintf("width:%.4f%%", w), fmt.Sprintf("height:%.4f%%", h)}
	if chart.ClipContent {
		styles = append(styles, "overflow:hidden")
	}
	var transforms []string
	if chart.Transform.RotationDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", chart.Transform.RotationDeg))
	}
	if chart.Transform.FlipH {
		transforms = append(transforms, "scaleX(-1)")
	}
	if chart.Transform.FlipV {
		transforms = append(transforms, "scaleY(-1)")
	}
	effectStyles := presentationEffectPlanStyles(chart.Effects)
	if len(effectStyles) > 0 {
		styles = append(styles, "overflow:visible")
	}
	transform, origin := mergeEffectTransform(strings.Join(transforms, " "), effectStyles)
	if transform != "" {
		styles = append(styles, "transform:"+transform)
	}
	if origin != "" {
		styles = append(styles, "transform-origin:"+origin)
	}
	styles = appendSortedCSSProps(styles, effectStyles)
	dataURI := fmt.Sprintf("data:%s;base64,%s", chart.MIME, base64.StdEncoding.EncodeToString(chart.Data))
	if chart.Paint.ZIndex > 0 {
		styles = append(styles, fmt.Sprintf("z-index:%d", chart.Paint.ZIndex))
	}
	if chart.Paint.OverflowVisible {
		styles = append(styles, "overflow:visible")
	}
	anim := ""
	if shape != nil {
		anim = animationAttrs(shape, ctx)
	}
	fmt.Fprintf(b, "<div class=\"shape shape--chart\" style=%q%s>\n", strings.Join(styles, ";"), anim)
	fit := chart.ObjectFit
	if fit == "" {
		fit = "fill"
	}
	fmt.Fprintf(b, "<img class=\"ppt-chart__artifact\" src=%q alt=%q style=%q>\n", dataURI, escapeAttr(chart.Alt), fmt.Sprintf("display:block;width:100%%;height:100%%;object-fit:%s", fit))
	b.WriteString("</div>\n")
}

func renderConnector(b *strings.Builder, item *playout.PhysicalVisual, slideW, slideH float64, ctx *htmlRenderContext) {
	shape := item.SourceShape
	if shape == nil {
		return
	}
	// Layout deliberately chooses one of two physical connector contracts.
	// Legacy PPT connectors normally carry resolved slide-space endpoints, so
	// they materialize as FreeConnectorVisualPlan even when they also retain a
	// preset connector type. Serializing them through ConnectorVisualPlan loses
	// that plan and used to emit an empty full-slide SVG.
	free := func() playout.FreeConnectorVisualPlan {
		if item.FreeConnector != nil {
			return *item.FreeConnector
		}
		return playout.FreeConnectorVisualPlan{}
	}()
	if len(free.Points) >= 2 {
		renderPhysicalFreeConnector(b, shape, slideW, slideH, ctx, free)
		return
	}
	connector := func() playout.ConnectorVisualPlan {
		if item.Connector != nil {
			return *item.Connector
		}
		return playout.ConnectorVisualPlan{}
	}()
	if len(connector.Primitives) > 0 {
		renderPhysicalConnector(b, shape, slideW, slideH, ctx, connector)
	}
}

func renderPhysicalFreeConnector(b *strings.Builder, shape *ir.Shape, slideW, slideH float64, ctx *htmlRenderContext, visual playout.FreeConnectorVisualPlan) {
	if len(visual.Points) < 2 {
		return
	}
	color := visual.Stroke.Color
	fmt.Fprintf(b, "<svg class=\"shape\" style=\"position:absolute;left:0;top:0;width:100%%;height:100%%;overflow:visible;pointer-events:none%s\" viewBox=\"0 0 %.4f %.4f\" xmlns=\"http://www.w3.org/2000/svg\"%s>", physicalPaintSVGStyle(visual.Composition.Paint), slideW, slideH, animationAttrs(shape, ctx))
	if visual.Curved {
		if len(visual.CurveSegments) > 0 {
			first := visual.CurveSegments[0]
			fmt.Fprintf(b, "<path d=\"M %.4f %.4f", first.Start.X, first.Start.Y)
			for _, seg := range visual.CurveSegments {
				fmt.Fprintf(b, " C %.4f %.4f %.4f %.4f %.4f %.4f", seg.Control1.X, seg.Control1.Y, seg.Control2.X, seg.Control2.Y, seg.End.X, seg.End.Y)
			}
			b.WriteString("\" fill=\"none\"")
		} else {
			p0, p1 := visual.Points[0], visual.Points[len(visual.Points)-1]
			fmt.Fprintf(b, "<path d=\"M %.4f %.4f C %.4f %.4f %.4f %.4f %.4f %.4f\" fill=\"none\"", p0.X, p0.Y, visual.Control1.X, visual.Control1.Y, visual.Control2.X, visual.Control2.Y, p1.X, p1.Y)
		}
		writeSVGStrokePlan(b, visual.Stroke, "", "#"+color)
		b.WriteString("/>")
	} else if len(visual.Points) > 2 {
		fmt.Fprintf(b, "<polyline points=%q fill=\"none\"", smartArtConnectorPointsString(visual.Points))
		writeSVGStrokePlan(b, visual.Stroke, "", "#"+color)
		b.WriteString("/>")
	} else {
		fmt.Fprintf(b, "<line x1=\"%.4f\" y1=\"%.4f\" x2=\"%.4f\" y2=\"%.4f\"", visual.Points[0].X, visual.Points[0].Y, visual.Points[1].X, visual.Points[1].Y)
		writeSVGStrokePlan(b, visual.Stroke, "", "#"+color)
		b.WriteString("/>")
	}
	b.WriteString(smartArtArrowGeometrySVG(visual.EndArrow, color, visual.Stroke.Alpha, visual.Stroke.AlphaSet))
	b.WriteString(smartArtArrowGeometrySVG(visual.StartArrow, color, visual.Stroke.Alpha, visual.Stroke.AlphaSet))
	b.WriteString("</svg>\n")
}

// renderPhysicalConnector serializes connector geometry already projected into
// point-space by presentation/layout.
func renderPhysicalConnector(b *strings.Builder, shape *ir.Shape, slideW, slideH float64, ctx *htmlRenderContext, visual playout.ConnectorVisualPlan) {
	if shape == nil {
		return
	}
	fmt.Fprintf(b, "<svg class=\"shape\" style=\"position:absolute;left:0;top:0;width:100%%;height:100%%;overflow:visible;pointer-events:none%s\" viewBox=\"0 0 %.4f %.4f\" xmlns=\"http://www.w3.org/2000/svg\"%s>", physicalPaintSVGStyle(visual.Composition.Paint), slideW, slideH, animationAttrs(shape, ctx))

	if visual.Composition.Transform.RotationDeg != 0 {
		cx := visual.BoundsXPt + visual.BoundsWPt/2
		cy := visual.BoundsYPt + visual.BoundsHPt/2
		fmt.Fprintf(b, "<g transform=\"rotate(%.4f %.4f %.4f)\">", visual.Composition.Transform.RotationDeg, cx, cy)
	}

	for _, primitive := range visual.Primitives {
		writeSVGPrimitivePlan(b, primitive, "")
	}

	if visual.Composition.Transform.RotationDeg != 0 {
		b.WriteString("</g>")
	}
	b.WriteString("</svg>\n")
}

func writeSVGPrimitivePlan(b *strings.Builder, plan playout.VectorPrimitivePlan, fillDefID string) {
	primitive := plan.Primitive
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
			writeSVGPrimitiveFillOpacity(b, primitive.Style)
			if primitive.Style.StrokeColor != "" && primitive.Style.StrokeWidth > 0 {
				writeSVGStrokePlan(b, plan.Stroke, primitive.Style.StrokeFillDefID, "#"+primitive.Style.StrokeColor)
			}
			b.WriteString("/>")
			return
		}
		if primitive.Style.StrokeColor != "" {
			if len(primitive.Points) == 2 {
				fmt.Fprintf(b, "<line x1=\"%.4f\" y1=\"%.4f\" x2=\"%.4f\" y2=\"%.4f\"",
					primitive.Points[0].X, primitive.Points[0].Y, primitive.Points[1].X, primitive.Points[1].Y)
				writeSVGStrokePlan(b, plan.Stroke, primitive.Style.StrokeFillDefID, "#"+primitive.Style.StrokeColor)
				b.WriteString("/>")
				return
			}
			fmt.Fprintf(b, "<polyline points=%q fill=\"none\"", drawingPointsString(primitive.Points))
			writeSVGStrokePlan(b, plan.Stroke, primitive.Style.StrokeFillDefID, "#"+primitive.Style.StrokeColor)
			b.WriteString("/>")
		}
	case drawing.PrimitiveEllipse:
		if primitive.Ellipse == nil {
			return
		}
		fill := svgPrimitiveFillValue(primitive.Style, fillDefID)
		fmt.Fprintf(b, "<ellipse cx=\"%.4f\" cy=\"%.4f\" rx=\"%.4f\" ry=\"%.4f\" fill=%q",
			primitive.Ellipse.Center.X, primitive.Ellipse.Center.Y, primitive.Ellipse.RX, primitive.Ellipse.RY, fill)
		writeSVGPrimitiveFillOpacity(b, primitive.Style)
		if primitive.Style.StrokeColor != "" && primitive.Style.StrokeWidth > 0 {
			writeSVGStrokePlan(b, plan.Stroke, primitive.Style.StrokeFillDefID, "#"+primitive.Style.StrokeColor)
		}
		b.WriteString("/>")
	case drawing.PrimitivePath:
		if primitive.Path == nil {
			return
		}
		fmt.Fprintf(b, "<path d=%q fill=%q", svgPathData(*primitive.Path), svgPrimitiveFillValue(primitive.Style, fillDefID))
		writeSVGPrimitiveFillOpacity(b, primitive.Style)
		if primitive.Style.StrokeColor != "" && primitive.Style.StrokeWidth > 0 {
			writeSVGStrokePlan(b, plan.Stroke, primitive.Style.StrokeFillDefID, "#"+primitive.Style.StrokeColor)
		} else if primitive.Style.Stroke != nil {
			writeSVGStrokePlan(b, plan.Stroke, primitive.Style.StrokeFillDefID, "none")
		}
		if primitive.Path.FillRule != "" {
			fmt.Fprintf(b, " fill-rule=%q", primitive.Path.FillRule)
		}
		b.WriteString("/>")
	}
}

func writeSVGPrimitiveFillOpacity(b *strings.Builder, style drawing.VectorStyle) {
	if b == nil || !style.FillAlphaSet || style.FillAlpha >= 1 {
		return
	}
	alpha := style.FillAlpha
	if alpha < 0 {
		alpha = 0
	}
	fmt.Fprintf(b, " fill-opacity=\"%.4f\"", alpha)
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

func writeSVGStrokePlan(b *strings.Builder, plan playout.StrokeVisualPlan, strokeDefID string, fallbackColor string) {
	if b == nil {
		return
	}
	color := fallbackColor
	if plan.Color != "" {
		color = "#" + plan.Color
	}
	if strokeDefID != "" {
		color = "url(#" + strokeDefID + ")"
	}
	if color == "" || (color == "none" && strokeDefID == "") || plan.WidthPt <= 0 {
		return
	}
	fmt.Fprintf(b, " stroke=%q stroke-width=\"%.2f\"", color, plan.WidthPt)
	if plan.SVGLineCap != "" {
		fmt.Fprintf(b, " stroke-linecap=%q", plan.SVGLineCap)
	}
	if plan.SVGLineJoin != "" {
		fmt.Fprintf(b, " stroke-linejoin=%q", plan.SVGLineJoin)
	}
	if plan.SVGLineJoin == "miter" && plan.MiterLimit > 0 {
		fmt.Fprintf(b, " stroke-miterlimit=\"%.2f\"", plan.MiterLimit)
	}
	if plan.DashArray != "" {
		fmt.Fprintf(b, " stroke-dasharray=%q", plan.DashArray)
	}
	if plan.AlphaSet && plan.Alpha >= 0 && plan.Alpha < 1 {
		fmt.Fprintf(b, " stroke-opacity=\"%.4f\"", plan.Alpha)
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

func renderParagraphWithPlan(b *strings.Builder, p *ir.Paragraph, plan playout.ParagraphVisualPlan, fontScale float64, ctx *htmlRenderContext) {
	paragraphMetrics := plan.Metrics
	var pStyles []string
	if plan.Align != "" {
		pStyles = append(pStyles, "text-align:"+mapAlign(plan.Align))
	}
	if plan.TextDirection == "rtl" || plan.TextDirection == "ltr" {
		pStyles = append(pStyles, "direction:"+plan.TextDirection)
	}
	if paragraphMetrics.HasMarginLeft && paragraphMetrics.MarginLeftPt != 0 {
		pStyles = append(pStyles, fmt.Sprintf("margin-left:%.4fpt", paragraphMetrics.MarginLeftPt))
	}
	if paragraphMetrics.HasMarginRight && paragraphMetrics.MarginRightPt != 0 {
		pStyles = append(pStyles, fmt.Sprintf("margin-right:%.4fpt", paragraphMetrics.MarginRightPt))
	}
	if paragraphMetrics.HasTextIndent && paragraphMetrics.TextIndentPt != 0 {
		pStyles = append(pStyles, fmt.Sprintf("text-indent:%.4fpt", paragraphMetrics.TextIndentPt))
	}
	switch plan.FontAlign {
	case "t":
		pStyles = append(pStyles, "vertical-align:top")
	case "ctr":
		pStyles = append(pStyles, "vertical-align:middle")
	case "base":
		pStyles = append(pStyles, "vertical-align:baseline")
	case "b":
		pStyles = append(pStyles, "vertical-align:bottom")
	}
	if paragraphMetrics.HasSpaceBefore {
		pStyles = append(pStyles, fmt.Sprintf("margin-top:%.4fpt", paragraphMetrics.SpaceBeforePt))
	}
	if paragraphMetrics.HasSpaceAfter {
		pStyles = append(pStyles, fmt.Sprintf("margin-bottom:%.4fpt", paragraphMetrics.SpaceAfterPt))
	}
	if plan.LinesMeasured {
		// Physical lines own the complete line box. A paragraph-level line-height
		// would create a browser strut in addition to the measured child line and
		// can silently make the visual stack taller than TextFrameLayout. Zero the
		// strut so mixed-font/absolute-spacing lines serialize at their exact
		// physical heights.
		pStyles = append(pStyles, "line-height:0")
	} else if paragraphMetrics.LineHeightScale > 0 && plan.HasLineSpacing {
		pStyles = append(pStyles, fmt.Sprintf("line-height:%.4f", paragraphMetrics.LineHeightScale))
	}
	// Fallback list indentation is already materialized into Metrics.MarginLeftPt.
	// Do not add an em-based serializer-only padding that would narrow the line a
	// second time and disagree with physical wrapping.
	if plan.HasTabs && plan.TabSizePt > 0 {
		pStyles = append(pStyles, fmt.Sprintf("tab-size:%.1fpt", plan.TabSizePt))
	}
	styleAttr := ""
	if len(pStyles) > 0 {
		styleAttr = fmt.Sprintf(" style=%q", strings.Join(pStyles, ";"))
	}
	dataAttr := ""
	if len(plan.TabStops) > 0 {
		dataAttr = fmt.Sprintf(" data-tab-stops=%q", encodeTabStops(plan.TabStops))
	}
	if len(p.Runs) == 0 && !plan.LinesMeasured {
		fmt.Fprintf(b, "<p%s%s><br></p>\n", styleAttr, dataAttr)
		return
	}
	fmt.Fprintf(b, "<p%s%s>", styleAttr, dataAttr)
	marker := plan.Marker
	markerText := ""
	switch marker.Kind {
	case "bullet":
		markerText = escapeHTML(marker.Text)
	case "image":
		markerText = "image" // presence sentinel; renderPhysicalMarker paints the relationship-backed image
	case "auto":
		// Layout owns numbering state and materializes the exact marker text.
		markerText = marker.Text
	}
	if !plan.LinesMeasured && markerText != "" {
		renderPhysicalMarker(b, marker, markerText, ctx)
	}
	if plan.LinesMeasured {
		for lineIndex, line := range plan.Lines {
			// Physical layout owns baseline advance. Browser `normal` line-height is
			// font/engine dependent and would make serialized placement disagree
			// with the exact line boxes measured by TextFrameLayout.
			// A measured line is a physical block, not another browser inline. Using
			// inline-block here made paragraph justification treat each measured line
			// as an inline item around forced <br>s, producing enormous inter-word
			// gaps and allowing browser line construction to fight layout. A block
			// preserves the measured line boundary while still inheriting left/right/
			// center paragraph alignment. Physical wrapping is layout-owned; the browser must not add a second set of line breaks.
			lineIndentPt := 0.0
			if lineIndex == 0 && paragraphMetrics.HasTextIndent && markerText == "" {
				// Marker paragraphs materialize their authored first-line origin on
				// Marker.OffsetPt. Keeping text-indent as well would make marker
				// placement depend on browser inheritance and can apply the hanging
				// indent twice. Unmarked paragraphs still use the authored indent.
				lineIndentPt = paragraphMetrics.TextIndentPt
			}
			linePositionStyle := ""
			// A measured line also owns its horizontal origin. Do not ask the browser
			// to re-measure centred/right-aligned inline children: small font-engine
			// differences otherwise shift text relative to independently authored
			// highlight rectangles and chart annotations.
			if markerText == "" && !paragraphMetrics.HasTextIndent && line.WidthPt > 0 {
				available := paragraphMetrics.AvailableWidthPt
				switch plan.Align {
				case "ctr", "center":
					offset := math.Max(0, (available-line.WidthPt)/2)
					linePositionStyle = fmt.Sprintf("width:%.4fpt;margin-left:%.4fpt;text-align:left;", line.WidthPt, offset)
				case "r", "right":
					offset := math.Max(0, available-line.WidthPt)
					linePositionStyle = fmt.Sprintf("width:%.4fpt;margin-left:%.4fpt;text-align:left;", line.WidthPt, offset)
				}
			}
			if line.HeightPt > 0 {
				// `line-height` alone is only a minimum contribution to a block's
				// used height: a browser may grow it when the fallback face has taller
				// font metrics. That changes the following line's authored origin and
				// makes dense legacy-PPT paragraphs drift into one another. Lock the
				// physical advance with height, while keeping overflow visible so ink
				// such as descenders, shadows, and a substituted font is never clipped.
				fmt.Fprintf(b, "<span class=\"ppt-physical-line\" style=\"display:block;%swhite-space:pre;text-indent:%.4fpt;line-height:%.4fpt;height:%.4fpt;min-height:%.4fpt;overflow:visible;vertical-align:baseline\">", linePositionStyle, lineIndentPt, line.HeightPt, line.HeightPt, line.HeightPt)
			} else {
				fmt.Fprintf(b, "<span class=\"ppt-physical-line\" style=\"display:block;%swhite-space:pre;text-indent:%.4fpt\">", linePositionStyle, lineIndentPt)
			}

			// The marker is part of the first physical line box. Keeping it inside
			// that box prevents the browser from creating a second independent
			// baseline/height contribution beside layout's measured first line.
			if lineIndex == 0 && markerText != "" && physicalLineHasVisibleText(line) {
				renderPhysicalMarker(b, marker, markerText, ctx)
			}
			for _, fragment := range line.Fragments {
				if fragment.Run == nil || fragment.Text == "" {
					continue
				}
				if fragment.Text == "\t" {
					fmt.Fprintf(b, "<span class=\"ppt-tab\" style=\"display:inline-block;width:%.4fpt\"></span>", fragment.WidthPt)
					continue
				}
				run := *fragment.Run
				run.Text = fragment.Text
				if fragment.AscentPt > 0 || fragment.DescentPt > 0 {
					// Layout owns the baseline box. Place each mixed-font fragment from
					// the physical line top instead of asking the browser to synthesize
					// a common baseline from font-specific inline metrics.
					fmt.Fprintf(b, "<span class=\"ppt-physical-fragment\" style=\"display:inline-block;vertical-align:top;position:relative;top:%.4fpt;height:%.4fpt;line-height:%.4fpt\">", fragment.TopOffsetPt, fragment.AscentPt+fragment.DescentPt, fragment.AscentPt+fragment.DescentPt)
					renderRunWithPlan(b, &run, fragment.Visual, ctx)
					b.WriteString("</span>")
				} else {
					renderRunWithPlan(b, &run, fragment.Visual, ctx)
				}
			}
			b.WriteString("</span>")
		}
	} else {
		for _, run := range p.Runs {
			if run != nil {
				renderRun(b, run, fontScale, ctx)
			}
		}
	}
	b.WriteString("</p>\n")
}

func physicalLineHasVisibleText(line playout.PhysicalTextLine) bool {
	for _, fragment := range line.Fragments {
		if strings.TrimSpace(fragment.Text) != "" {
			return true
		}
	}
	return false
}

func renderPhysicalMarker(b *strings.Builder, marker playout.MarkerVisualPlan, text string, ctx *htmlRenderContext) {
	if marker.Kind == "image" && len(marker.ImageData) != 0 {
		mime := marker.ImageMIME
		if mime == "" {
			mime = "image/png"
		}
		size := marker.FontSizePt
		if size <= 0 {
			size = 12
		}
		styles := []string{fmt.Sprintf("width:%.4fpt", size), fmt.Sprintf("height:%.4fpt", size), "object-fit:contain", "display:inline-block", "vertical-align:top", "position:relative", fmt.Sprintf("top:%.4fpt", marker.TopOffsetPt)}
		if marker.OffsetPt != 0 {
			styles = append(styles, fmt.Sprintf("margin-left:%.4fpt", marker.OffsetPt))
		}
		if marker.GapPt > 0 {
			styles = append(styles, fmt.Sprintf("margin-right:%.4fpt", marker.GapPt))
		}
		encoded := base64.StdEncoding.EncodeToString(marker.ImageData)
		fmt.Fprintf(b, "<img class=\"bullet-marker bullet-marker--image\" style=%q src=\"data:%s;base64,%s\" alt=\"\">", strings.Join(styles, ";"), mime, encoded)
		return
	}
	var styles []string
	if marker.FontSizePt > 0 {
		styles = append(styles, fmt.Sprintf("font-size:%.1fpt", marker.FontSizePt))
	}
	if marker.FontFamily != "" {
		styles = append(styles, "font-family:"+cssFontFamilyValue(marker.FontFamily, !ctx.isEmbeddedFamily(marker.FontFamily)))
	}
	if marker.Color != "" {
		styles = append(styles, "color:#"+marker.Color)
	}
	if marker.AscentPt > 0 || marker.DescentPt > 0 {
		styles = append(styles, "display:inline-block", "vertical-align:top", "position:relative")
		styles = append(styles, fmt.Sprintf("top:%.4fpt", marker.TopOffsetPt))
		styles = append(styles, fmt.Sprintf("height:%.4fpt", marker.AscentPt+marker.DescentPt))
		styles = append(styles, fmt.Sprintf("line-height:%.4fpt", marker.AscentPt+marker.DescentPt))
	}
	if marker.OffsetPt != 0 {
		styles = append(styles, fmt.Sprintf("margin-left:%.4fpt", marker.OffsetPt))
	}
	if marker.GapPt > 0 {
		styles = append(styles, fmt.Sprintf("margin-right:%.4fpt", marker.GapPt))
	}
	if len(styles) == 0 {
		fmt.Fprintf(b, "<span class=\"bullet-marker\">%s</span>", text)
		return
	}
	fmt.Fprintf(b, "<span class=\"bullet-marker\" style=%q>%s</span>", strings.Join(styles, ";"), text)
}

func renderRun(b *strings.Builder, run *ir.Run, fontScale float64, ctx *htmlRenderContext) {
	renderRunWithPlan(b, run, playout.ResolveRunVisualPlan(run, fontScale), ctx)
}

// renderRunWithPlan serializes a layout-owned physical run plan. Composed text
// lines use this path so the renderer cannot independently resolve run paint
// after line measurement has already used that run's font identity and scale.
func renderRunWithPlan(b *strings.Builder, run *ir.Run, plan playout.RunVisualPlan, ctx *htmlRenderContext) {
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
	if plan.Bold {
		styles = append(styles, "font-weight:bold")
	}
	if plan.Italic {
		styles = append(styles, "font-style:italic")
	}
	if plan.SmallCaps {
		styles = append(styles, "font-variant:small-caps")
	}
	var decorations []string
	if plan.Underline {
		decorations = append(decorations, "underline")
	}
	if plan.Strike {
		decorations = append(decorations, "line-through")
	}
	if len(decorations) > 0 {
		styles = append(styles, "text-decoration:"+strings.Join(decorations, " "))
	}
	if plan.FontSizePt > 0 {
		styles = append(styles, fmt.Sprintf("font-size:%.1fpt", plan.FontSizePt))
	}
	if plan.CharSpacingPt != 0 {
		styles = append(styles, fmt.Sprintf("letter-spacing:%.2fpt", plan.CharSpacingPt))
	}
	if plan.KerningSpecified {
		if plan.KerningEnabled {
			styles = append(styles, "font-kerning:normal")
		} else {
			styles = append(styles, "font-kerning:none")
		}
	}
	if plan.FontFamily != "" {
		styles = append(styles, "font-family:"+cssFontFamilyValue(plan.FontFamily, !ctx.isEmbeddedFamily(plan.FontFamily)))
	}
	if plan.Color != "" {
		styles = append(styles, "color:#"+plan.Color)
	} else if len(styles) > 0 {
		// Always include an explicit color in styled spans so the output is
		// self-contained and does not rely on CSS color inheritance.
		styles = append(styles, "color:#"+ctx.defaultTextColor)
	}
	if plan.Highlight != "" {
		styles = append(styles, "background-color:#"+plan.Highlight)
	}
	if plan.Effects.HasEffects {
		styles = appendSortedCSSProps(styles, presentationTextEffectPlanStyles(plan.Effects))
	}
	rawText := run.Text
	if fieldText != "" {
		rawText = fieldText
	}
	if plan.AllCaps {
		rawText = strings.ToUpper(rawText)
	}
	if family, portableText := metafile.PortableMathText(plan.FontFamily, rawText); family != plan.FontFamily {
		rawText = portableText
		// Override the legacy Symbol family emitted from the run plan. Its
		// private-use positions have been translated to Unicode and must use a
		// Unicode math-capable fallback instead of browser-dependent Symbol.
		styles = append(styles, "font-family:"+cssFontFamilyValue(family, true))
	}
	text := escapeHTML(rawText)
	// DrawingML line breaks (a:br) are resolved as \n runs; emit HTML <br> tags.
	text = strings.ReplaceAll(text, "\n", "<br>")
	if plan.BaselineShiftPct != 0 {
		// Preserve the authored baseline displacement without invoking the browser's
		// <sup>/<sub> font-size heuristics. Layout already owns the run's font size
		// and physical line box; relative positioning changes paint only.
		styles = append(styles, fmt.Sprintf("position:relative;bottom:%.4fem", plan.BaselineShiftPct/100.0))
	}
	wrapTag := ""
	if plan.BaselineShiftPct == 0 && plan.Superscript {
		wrapTag = "sup"
	} else if plan.BaselineShiftPct == 0 && plan.Subscript {
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

func renderEmbeddedFontCSS(b *strings.Builder, pres *ir.Presentation, ctx *htmlRenderContext) {
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
	switch playout.ResolveFontFallbackClass(family) {
	case playout.FontFallbackMono:
		return []string{"Consolas", "Courier New", "monospace"}
	case playout.FontFallbackSerif:
		return []string{"Times New Roman", "serif"}
	case playout.FontFallbackCondensed:
		return []string{"Arial Narrow", "Arial", "sans-serif"}
	default:
		return []string{"Arial", "sans-serif"}
	}
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

// physicalBoundsPercent serializes layout-owned point bounds into slide percentages.
// It deliberately accepts no canonical IR geometry.
func physicalBoundsPercent(r playout.PhysicalPaintBoundsPlan, slideWPt, slideHPt float64) (x, y, w, h float64) {
	x = r.LeftPt / slideWPt * 100
	y = r.TopPt / slideHPt * 100
	w = r.WidthPt / slideWPt * 100
	h = r.HeightPt / slideHPt * 100
	return
}

// boundsPercent converts EMU rect to percentage coordinates within the slide canvas.
func boundsPercent(r ir.Rect, slideWPt, slideHPt float64) (x, y, w, h float64) {
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

func encodeTabStops(stops []ir.TabStop) string {
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
func animationAttrs(shape *ir.Shape, ctx *htmlRenderContext) string {
	if shape == nil {
		return ""
	}
	var attrs strings.Builder
	// AnimationOnly shapes are persisted paint-state overlays, not part of the
	// static slide. In playback mode they still need to be serialized so the
	// timeline can reveal them, but they must start hidden immediately rather
	// than waiting for the player bootstrap to add body.ppt-player-ready. This
	// prevents legacy emphasis/selection overlays from flashing in exported HTML
	// or screenshot pipelines before JavaScript applies step 0.
	if shape.AnimationOnly {
		attrs.WriteString(` data-animation-only="1" data-anim-visible="0"`)
	}
	if shapeHasTimelineTarget(shape, ctx) {
		fmt.Fprintf(&attrs, " data-shape-id=\"%d\"", shape.ID)
	}
	if shape.AnimStep <= 0 {
		return attrs.String()
	}
	fmt.Fprintf(&attrs, " data-anim-step=\"%d\"", shape.AnimStep)
	return attrs.String()
}

func shapeHasTimelineTarget(shape *ir.Shape, ctx *htmlRenderContext) bool {
	if shape == nil || shape.ID <= 0 || ctx == nil || len(ctx.animShapeIDs) == 0 {
		return false
	}
	_, ok := ctx.animShapeIDs[shape.ID]
	return ok
}

func emphasisEffectsAttr(shape *ir.Shape) string {
	if shape == nil || len(shape.EmphasisEffects) == 0 {
		return ""
	}
	return fmt.Sprintf(" data-anim-emphasis=\"%s\"", strings.Join(shape.EmphasisEffects, " "))
}

func motionPathAttr(shape *ir.Shape) string {
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

func triggerAttr(shape *ir.Shape) string {
	if shape == nil || len(shape.AnimationTriggers) == 0 {
		return ""
	}
	return fmt.Sprintf(" data-anim-trigger=\"%s\"", strings.Join(shape.AnimationTriggers, " "))
}

// presentationNeedsBehaviorRuntime reports whether static HTML still needs the
// presentation behavior runtime. Transitions alone do not opt a document into
// slideshow playback, but animation steps/timelines, triggers, motion paths and
// action links need the runtime in order for their authored behavior to work.
func physicalPresentationNeedsBehaviorRuntime(slides []playout.PhysicalSlidePlan) bool {
	for i := range slides {
		playback := slides[i].Playback
		if playback.MaxAnimationStep > 0 || playback.HasAnimationTimeline() || playback.HasInteractiveBehavior {
			return true
		}
	}
	return false
}

func physicalPresentationNeedsPlayback(slides []playout.PhysicalSlidePlan) bool {
	for i := range slides {
		playback := slides[i].Playback
		if playback.TransitionType != "" || playback.MaxAnimationStep > 0 || playback.HasAnimationTimeline() || playback.HasInteractiveBehavior {
			return true
		}
	}
	return false
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

func cssBorderStyleForLine(style *drawing.LineStyle) string {
	if style == nil {
		return "solid"
	}
	if style.Compound || (style.CompoundType != "" && style.CompoundType != "sng") {
		return "double"
	}
	if style.Dash != "" && style.Dash != drawing.DashPatternSolid {
		return "dashed"
	}
	return "solid"
}

// slideCSS is a format string receiving (slideWidthPt, slideHeightPt, aspectRatio, printWidthPt, printHeightPt).
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
.shape--text p { margin: 0; flex-shrink: 0; }
.shape__effect-overlay { box-sizing: border-box; }
.shape__background,
.shape__vector {
  position: absolute;
  inset: 0;
  width: 100%%;
  height: 100%%;
}
.bullet-marker { margin-right: 0; }
.shape--image-placeholder { background: #e8e8e8; border: 1px dashed #bbb; }
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
.ppt-table { width: 100%%; border-collapse: separate; border-spacing: 0; table-layout: fixed; }
.ppt-table td { overflow-wrap: normal; }
.shape-link { display: contents; text-decoration: none; color: inherit; }
.shape-ancestor-clip .shape, .shape-ancestor-clip .shape-link { pointer-events: auto; }
.slide-number { display: contents; }

@page { size: %.2fpt %.2fpt; margin: 0; }
@media print {
  html, body { margin: 0; padding: 0; background: #fff; }
  body { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
  .slide { margin: 0; max-width: none; break-after: page; page-break-after: always; }
  .slide:last-of-type { break-after: auto; page-break-after: auto; }
  .slide-num,
  .notes,
  .headers-footers,
  .comments,
  .diagnostics { display: none !important; }
  .slide-scroll { overflow: visible; }
  .canvas { box-shadow: none; }
}
`

const oleMathCSS = `
.shape--ole-math { overflow: visible; }
.shape--ole-math .ole-math-inner { position:absolute; left:0; top:0; display:inline-block; width:max-content; height:max-content; white-space:nowrap; line-height:normal; font-size:12pt; transform-origin:0 0; }
.shape--ole-math math { display:inline-block; margin:0; padding:0; font-size:1em; line-height:normal; color:inherit; }
.shape--ole-math .ole-math-baseline-marker { display:inline-block; width:0; height:0; margin:0; padding:0; vertical-align:baseline; }
.shape--ole-math .ole-math-authored-preview { position:absolute; inset:0; visibility:hidden; pointer-events:none; }
.shape--ole-math .ole-math-authored-preview > svg { width:100%; height:100%; display:block; }
.shape--ole-object-preview { overflow: hidden; }
.shape--ole-object-preview .shape--ole-object-preview-svg,
.shape--ole-object-preview .shape--ole-object-preview-svg > svg { width: 100%; height: 100%; display: block; }
`

const oleMathScript = `
(function(){
 function fitOne(host){
  var inner=host.querySelector(".ole-math-inner"); if(!inner)return;
  var box=host.getBoundingClientRect(); if(!(box.width>0&&box.height>0))return;
  var offsetX=parseFloat(host.getAttribute("data-math-offset-x")||"");
  var baselineY=parseFloat(host.getAttribute("data-math-baseline-y")||"");
  var centerY=parseFloat(host.getAttribute("data-math-center-y")||"");
  // Use one scale from MathML's natural metrics to the OLE frame.  Preview SVGs
  // are recorded in different coordinate systems by legacy PPT (WMF/EMF) and
  // PPTX, so applying their font-size/viewBox ratio before this fit compounds
  // that representation difference into the visible semantic equation.
  inner.style.fontSize="16px";
  inner.style.left="0px"; inner.style.top="0px"; inner.style.transform="none";
  void inner.offsetHeight;
  var content=inner.getBoundingClientRect(); if(!(content.width>0&&content.height>0))return;
  var scale=Math.min(box.width/content.width,box.height/content.height);
  if(!isFinite(scale)||scale<=0)return; scale=Math.max(0.05,scale);
  // The OLE frame owns absolute placement.  The preview contributes only the
  // authored content centre and baseline *inside* that frame.  This preserves
  // PowerPoint's independent-shape model while matching the WMF/EMF placement.
  // The OLE frame is already the authored absolute rectangle.  Do not centre
  // browser MathML inside it: MathML is often intrinsically narrower than the
  // authored equation viewport, and centring moves the visible formula away
  // from the gap PowerPoint reserved for the OLE object.  The preview offset is
  // an origin correction inside that rectangle, not a centring hint.
  var originShift=(isFinite(offsetX)?offsetX:0)*box.width;
  inner.style.left=originShift+"px";
  // Align semantic ink to the authored preview's ink centre. Scripted MathML
  // and a cached MTEF preview do not share a browser baseline box, whereas their
  // visual centre remains stable across fractions, limits, and fences.
  var marker=inner.querySelector(".ole-math-baseline-marker");
  var topShift=0;
  if(isFinite(centerY)){
   topShift=box.height*centerY-content.height*scale/2;
  } else if(marker&&isFinite(baselineY)){
   var markerBox=marker.getBoundingClientRect();
   var baselineOffset=markerBox.top-content.top;
   topShift=box.height*baselineY-baselineOffset*scale;
   if(!isFinite(topShift))topShift=0;
  }
  inner.style.top=topShift+"px";
  inner.style.transform="scale("+scale+")";
 }
 function fitAll(){document.querySelectorAll(".shape--ole-math").forEach(fitOne)}
 document.addEventListener("DOMContentLoaded",fitAll); window.addEventListener("resize",fitAll);
 if(document.fonts&&document.fonts.ready)document.fonts.ready.then(fitAll);
})();
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
[data-animation-only="1"][data-anim-visible="0"] {
  visibility: hidden;
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

  // Generated HTML is a document by default: all slides remain visible and
  // scrollable even when the presentation contains transitions/animations.
  // Playback is opt-in so one animated slide cannot silently turn a multi-slide
  // export into a single-slide player. Use ?play=1 (or ?mode=player) to enter
  // presentation playback mode.
  var params = new URLSearchParams(window.location.search);
  var playbackRequested = params.get("play") === "1" || params.get("mode") === "player";
  if (!playbackRequested) {
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
