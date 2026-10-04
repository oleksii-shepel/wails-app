package stream

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	dml "office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/mathml"
	wordir "office/pkg/word/ir"
	wordparts "office/pkg/word/parsers/ooxml/parts"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

type docxFieldFrame struct {
	instruction                   string
	result                        bool
	storyStartBlock               int
	storyEndBlock                 int
	storyResultInCurrentParagraph bool
}

type docxStoryFieldResult struct {
	instruction string
	startBlock  int
	endBlock    int
}

type docxHyperlinkFrame struct {
	target string
	anchor string
	relID  string
}

type docxRevisionFrame struct {
	kind   string
	author string
	date   string
	id     string
}

type docxLayoutHandler struct {
	session            *LayoutSession
	fields             []docxFieldFrame
	hyperlinks         []docxHyperlinkFrame
	revisions          []docxRevisionFrame
	pendingSimple      string
	pendingStoryFields []docxStoryFieldResult
	notes              *docxNoteLoader
	comments           *docxCommentLoader
	media              *docxMediaLoader
	headers            *docxHeaderFooterLoader
	lastHeaders        []wordir.Reference
	lastFooters        []wordir.Reference
}

func docxHandlerForLayout(handler Handler) Handler {
	session, ok := handler.(*LayoutSession)
	if !ok || session == nil {
		return handler
	}
	return &docxLayoutHandler{session: session}
}

func (h *docxLayoutHandler) HandleEvent(event Event) error {
	if h == nil || h.session == nil {
		return nil
	}

	switch event.Type {
	case EventBookmarkStart:
		h.session.ensureParagraph()
		if event.BookmarkName != "" {
			h.session.paragraph.Bookmarks = appendUniqueString(h.session.paragraph.Bookmarks, event.BookmarkName)
		}
		return nil
	case EventBookmarkEnd:
		return nil
	case EventStartHyperlink:
		raw, _ := event.Raw.(wordsax.Event)
		h.hyperlinks = append(h.hyperlinks, docxHyperlinkFrame{target: raw.HyperlinkTarget, anchor: raw.HyperlinkAnchor, relID: raw.RelationshipID})
		return nil
	case EventEndHyperlink:
		if len(h.hyperlinks) > 0 {
			h.hyperlinks = h.hyperlinks[:len(h.hyperlinks)-1]
		}
		return nil
	case EventStartRevision:
		raw, _ := event.Raw.(wordsax.Event)
		h.revisions = append(h.revisions, docxRevisionFrame{kind: normalizeDOCXRevisionKind(raw.RevisionKind), author: raw.Author, date: raw.Date, id: raw.RevisionID})
		return nil
	case EventEndRevision:
		if len(h.revisions) > 0 {
			h.revisions = h.revisions[:len(h.revisions)-1]
		}
		return nil
	case EventFieldInstruction:
		if len(h.fields) > 0 {
			h.fields[len(h.fields)-1].instruction += event.FieldInstr
		} else {
			h.pendingSimple += event.FieldInstr
		}
		return nil
	case EventFieldChar:
		switch strings.ToLower(strings.TrimSpace(event.FieldCharType)) {
		case "begin":
			h.fields = append(h.fields, docxFieldFrame{storyStartBlock: -1, storyEndBlock: -1})
		case "separate":
			if len(h.fields) > 0 {
				h.fields[len(h.fields)-1].result = true
			}
		case "end":
			if len(h.fields) > 0 {
				frame := &h.fields[len(h.fields)-1]
				instr := strings.TrimSpace(frame.instruction)
				story := docxStoryFieldResult{instruction: instr, startBlock: frame.storyStartBlock, endBlock: frame.storyEndBlock}
				pendingCurrentParagraph := frame.storyResultInCurrentParagraph
				h.fields = h.fields[:len(h.fields)-1]
				if isBlockCachedFieldInstruction(instr) && story.startBlock >= 0 {
					if pendingCurrentParagraph {
						h.pendingStoryFields = append(h.pendingStoryFields, story)
					} else {
						h.finalizeStoryField(story)
					}
				}
			}
		}
		return nil
	case EventFootnoteReference:
		return h.appendNoteReference("footnote", event)
	case EventEndnoteReference:
		return h.appendNoteReference("endnote", event)
	case EventCommentReference:
		return h.appendCommentReference(event)
	case EventSymbol, EventSoftHyphen, EventNoBreakHyphen:
		return h.appendVisibleToken(event)
	case EventRuby:
		return h.appendRuby(event)
	case EventMath:
		return h.appendMath(event)
	case EventDrawing:
		raw, ok := event.Raw.(wordsax.Event)
		if !ok {
			return StreamUnsupportedEventError{Type: event.Type}
		}
		if docxDrawingCarriesShape(raw) {
			return appendDOCXShapeRun(h.session, event)
		}
		switch raw.DrawingGraphicKind {
		case "picture":
			return h.appendPicture(event)
		default:
			return StreamUnsupportedEventError{Type: event.Type}
		}
	case EventSection:
		if err := h.prepareSection(&event); err != nil {
			return err
		}
	}
	if err := h.session.HandleEvent(event); err != nil {
		return err
	}
	if event.Type == EventEndParagraph {
		h.finishStoryFieldParagraph()
	}
	if event.Type == EventText || event.Type == EventTab || event.Type == EventBreak {
		h.markStoryFieldResultVisible()
		h.annotateLastRunContext()
		instr := ""
		if len(h.fields) > 0 {
			frame := &h.fields[len(h.fields)-1]
			if frame.result {
				instr = strings.TrimSpace(frame.instruction)
			}
		} else if h.pendingSimple != "" {
			instr = strings.TrimSpace(h.pendingSimple)
			h.pendingSimple = ""
		}
		if instr != "" && !isBlockCachedFieldInstruction(instr) {
			h.annotateLastRun(instr)
		}
	}
	return nil
}

func (h *docxLayoutHandler) markStoryFieldResultVisible() {
	if h == nil || h.session == nil || len(h.session.tables) > 0 {
		return
	}
	for i := range h.fields {
		frame := &h.fields[i]
		if !frame.result || !isBlockCachedFieldInstruction(frame.instruction) {
			continue
		}
		if frame.storyStartBlock < 0 {
			// The current paragraph has not been appended to doc.Blocks yet. Its
			// body block will therefore occupy the current length.
			frame.storyStartBlock = len(h.session.doc.Blocks)
		}
		frame.storyResultInCurrentParagraph = true
	}
}

func (h *docxLayoutHandler) finishStoryFieldParagraph() {
	if h == nil || h.session == nil || len(h.session.tables) > 0 {
		return
	}
	endBlock := len(h.session.doc.Blocks) - 1
	if endBlock < 0 {
		return
	}
	for i := range h.fields {
		frame := &h.fields[i]
		if frame.storyResultInCurrentParagraph {
			frame.storyEndBlock = endBlock
			frame.storyResultInCurrentParagraph = false
		}
	}
	if len(h.pendingStoryFields) == 0 {
		return
	}
	pending := h.pendingStoryFields
	h.pendingStoryFields = nil
	for _, story := range pending {
		story.endBlock = endBlock
		h.finalizeStoryField(story)
	}
}

func (h *docxLayoutHandler) finalizeStoryField(story docxStoryFieldResult) {
	if h == nil || h.session == nil || story.startBlock < 0 || story.endBlock < story.startBlock {
		return
	}
	instr := strings.TrimSpace(story.instruction)
	kind := fieldKindFromInstruction(instr)
	if !isBlockCachedFieldInstruction(instr) || kind == "" {
		return
	}
	if story.endBlock >= len(h.session.doc.Blocks) {
		story.endBlock = len(h.session.doc.Blocks) - 1
	}
	if story.endBlock < story.startBlock {
		return
	}
	fieldIndex := len(h.session.doc.Fields)
	for i := story.startBlock; i <= story.endBlock; i++ {
		h.session.doc.Blocks[i].FieldKind = kind
		h.session.doc.Blocks[i].FieldIndex = fieldIndex
	}
	blocks := append([]wordir.Block(nil), h.session.doc.Blocks[story.startBlock:story.endBlock+1]...)
	h.session.doc.Fields = append(h.session.doc.Fields, wordir.StoryField{
		Kind:   kind,
		Instr:  instr,
		Arg:    instr,
		Blocks: blocks,
	})
}

func (h *docxLayoutHandler) prepareSection(event *Event) error {
	if event == nil || event.Section == nil || h.headers == nil {
		return nil
	}
	explicitHeaders, explicitFooters, err := h.headers.NextSection()
	if err != nil {
		return err
	}
	mergedHeaders := mergeSectionStoryRefs(h.lastHeaders, explicitHeaders)
	mergedFooters := mergeSectionStoryRefs(h.lastFooters, explicitFooters)
	doc := &h.session.doc
	doc.EvenAndOddHeaders = h.headers.evenAndOdd
	if len(mergedHeaders) > 0 && doc.Headers == nil {
		doc.Headers = make(map[string]wordir.Story)
	}
	if len(mergedFooters) > 0 && doc.Footers == nil {
		doc.Footers = make(map[string]wordir.Story)
	}
	for _, ref := range explicitHeaders {
		if ref.PartName == "" {
			continue
		}
		story, err := h.headers.Load(ref)
		if err != nil {
			return err
		}
		doc.Headers[ref.PartName] = h.session.resolveStoryFormatting(story)
	}
	for _, ref := range explicitFooters {
		if ref.PartName == "" {
			continue
		}
		story, err := h.headers.Load(ref)
		if err != nil {
			return err
		}
		doc.Footers[ref.PartName] = h.session.resolveStoryFormatting(story)
	}
	event.Section.Headers = append([]wordir.Reference(nil), mergedHeaders...)
	event.Section.Footers = append([]wordir.Reference(nil), mergedFooters...)
	h.lastHeaders = append([]wordir.Reference(nil), mergedHeaders...)
	h.lastFooters = append([]wordir.Reference(nil), mergedFooters...)
	return nil
}

func (h *docxLayoutHandler) appendVisibleToken(event Event) error {
	raw, _ := event.Raw.(wordsax.Event)
	text := raw.Text
	if text == "" {
		switch event.Type {
		case EventSoftHyphen:
			text = "\u00ad"
		case EventNoBreakHyphen:
			text = "\u2011"
		}
	}
	if text == "" {
		return nil
	}
	h.session.ensureParagraph()
	run := wordir.Run{Kind: "run", Text: text, StyleID: event.RunStyleID}
	if event.RunFormat != nil {
		run.Format = *event.RunFormat
	}
	h.session.paragraph.Text += text
	h.session.paragraph.Runs = append(h.session.paragraph.Runs, run)
	h.annotateLastRunContext()
	return nil
}

func (h *docxLayoutHandler) appendRuby(event Event) error {
	raw, ok := event.Raw.(wordsax.Event)
	if !ok || (raw.RubyBaseText == "" && raw.RubyText == "") {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	ruby := &wordir.RubyContent{Base: raw.RubyBaseText, Text: raw.RubyText}
	if raw.RubyModel != nil && raw.RubyModel.RubyPr != nil {
		pr := raw.RubyModel.RubyPr
		if pr.RubyAlign != nil {
			ruby.Align = pr.RubyAlign.Val
		}
		if pr.PhoneticGuideTextFontSize != nil {
			ruby.FontSize = float64(pr.PhoneticGuideTextFontSize.Val) / 2
		}
		if pr.BaseTextFontSize != nil {
			ruby.BaseFontSize = float64(pr.BaseTextFontSize.Val) / 2
		}
		if pr.PhoneticGuideRaise != nil {
			ruby.Raise = float64(pr.PhoneticGuideRaise.Val) / 2
		}
		if pr.LanguageID != nil {
			ruby.Lang = pr.LanguageID.Val
		}
	}
	h.session.ensureParagraph()
	run := wordir.Run{Kind: "ruby", Ruby: ruby, Text: raw.RubyBaseText, StyleID: event.RunStyleID}
	if event.RunFormat != nil {
		run.Format = *event.RunFormat
	}
	h.session.paragraph.Text += raw.RubyBaseText
	h.session.paragraph.Runs = append(h.session.paragraph.Runs, run)
	h.annotateLastRunContext()
	return nil
}

func (h *docxLayoutHandler) appendMath(event Event) error {
	raw, ok := event.Raw.(wordsax.Event)
	if !ok || raw.RawXML == "" {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	mml := mathml.Convert([]byte(raw.RawXML))
	if mml == "" {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	h.session.ensureParagraph()
	run := wordir.Run{Kind: "math", MathML: mml, Text: raw.MathText, StyleID: event.RunStyleID}
	if event.RunFormat != nil {
		run.Format = *event.RunFormat
	}
	h.session.paragraph.Text += raw.MathText
	h.session.paragraph.Runs = append(h.session.paragraph.Runs, run)
	h.annotateLastRunContext()
	return nil
}

func (h *docxLayoutHandler) appendPicture(event Event) error {
	raw, ok := event.Raw.(wordsax.Event)
	if !ok || raw.DrawingGraphicKind != "picture" || (raw.DrawingKind != "inline" && raw.DrawingKind != "anchor") {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	if h.media == nil || raw.RelationshipTarget == "" {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	data, mimeType, err := h.media.Load(raw.RelationshipTarget)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("word stream: picture payload %q is empty", raw.RelationshipTarget)
	}
	img := &wordir.Image{MIMEType: mimeType, Data: data, Width: raw.WidthEMU, Height: raw.HeightEMU, Alt: firstNonEmptyString(raw.DrawingDescription, raw.DrawingName), Title: raw.DrawingTitle, Hidden: raw.DrawingHidden}
	if raw.DrawingKind == "anchor" {
		populateAnchoredImage(img, raw)
	} else {
		// An inline drawing's effect extent is part of its in-flow box: Word
		// insets the painted picture by the left/top values and advances the pen
		// by the full l+extent+r span.
		img.EffectExtentL, img.EffectExtentT = raw.EffectExtentL, raw.EffectExtentT
		img.EffectExtentR, img.EffectExtentB = raw.EffectExtentR, raw.EffectExtentB
	}
	populateDOCXPictureProperties(img, raw.DrawingModel, h.themeColors())
	h.session.ensureParagraph()
	run := wordir.Run{Kind: "image", Image: img, StyleID: event.RunStyleID}
	if event.RunFormat != nil {
		run.Format = *event.RunFormat
	}
	h.session.paragraph.Runs = append(h.session.paragraph.Runs, run)
	h.annotateLastRunContext()
	return nil
}

// populateDOCXPictureProperties projects paint properties from the SAX drawing
// model into the shared IR. The HTML renderer owns their CSS realization.
func populateDOCXPictureProperties(img *wordir.Image, drawing *wordparts.Drawing, themeColors map[string]string) {
	if img == nil || drawing == nil {
		return
	}
	graphicData := drawingGraphicData(drawing)
	if graphicData == nil || graphicData.Picture == nil {
		return
	}
	picture := graphicData.Picture
	if picture.BlipFill != nil {
		if crop := picture.BlipFill.SrcRect; crop != nil {
			img.CropLeft = clampDOCXCropPct(crop.L)
			img.CropTop = clampDOCXCropPct(crop.T)
			img.CropRight = clampDOCXCropPct(crop.R)
			img.CropBottom = clampDOCXCropPct(crop.B)
		}
		img.Tile = picture.BlipFill.Tile != nil
		img.Stretch = picture.BlipFill.Stretch != nil || (!img.Tile && picture.BlipFill.Stretch == nil)
		if tile := picture.BlipFill.Tile; tile != nil {
			img.TileOffsetX = int64(tile.Tx)
			img.TileOffsetY = int64(tile.Ty)
			img.TileScaleX = tile.Sx
			img.TileScaleY = tile.Sy
			img.TileFlip = tile.Flip
			img.TileAlign = tile.Algn
		}
		if blip := picture.BlipFill.Blip; blip != nil {
			if blip.Lum != nil {
				// DrawingML a:lum stores signed percentage adjustments where 0 is
				// unchanged. Map them onto the picture corrections Word paints.
				img.Brightness, img.Contrast = wordir.PictureLumCorrection(blip.Lum.Bright, blip.Lum.Contrast)
			}
			if blip.AlphaMod != nil {
				img.HasAlpha = true
				img.Alpha = float64(blip.AlphaMod.Val) / 100000.0
			}
			img.Grayscale = blip.Grayscale != nil
			if blip.BiLevel != nil {
				img.BiLevel = true
				img.BiLevelThreshold = blip.BiLevel.Thresh
			}
		}
	}
	props := picture.ShapeProps
	if props == nil {
		return
	}
	if props.Xfrm != nil {
		img.RotationDeg = float64(props.Xfrm.Rot) / 60000.0
		img.FlipH = props.Xfrm.FlipH
		img.FlipV = props.Xfrm.FlipV
	}
	if props.PrstGeom != nil {
		img.FrameGeometry = props.PrstGeom.Prst
	}
	if line := drawingxml.ResolveLineStyle(props.Ln, themeColors); line != nil && line.Color != "" {
		img.BorderColor = line.Color
		img.BorderWidth = line.Width
	}
	if fill := resolveDOCXPictureFill(props, themeColors); fill != "" {
		img.FillColor = fill
	}
	img.Effects = props.DrawingEffectList(themeColors)
}

func drawingGraphicData(drawing *wordparts.Drawing) *wordparts.GraphicData {
	if drawing.Inline != nil && drawing.Inline.Graphic != nil {
		return drawing.Inline.Graphic.GraphicData
	}
	if drawing.Anchor != nil && drawing.Anchor.Graphic != nil {
		return drawing.Anchor.Graphic.GraphicData
	}
	return nil
}

func clampDOCXCropPct(value int) float64 {
	return math.Min(1, math.Max(0, float64(value)/100000.0))
}

func resolveDOCXPictureFill(props *wordparts.PictureShapeProps, themeColors map[string]string) string {
	if props == nil {
		return ""
	}
	var fill dml.Fill
	switch {
	case props.SolidFill != nil:
		fill = drawingxml.ResolveSolidFill(props.SolidFill, themeColors)
	case props.GradFill != nil:
		fill = drawingxml.ResolveGradientFill(props.GradFill, themeColors)
	case props.PattFill != nil:
		fill = drawingxml.ResolvePatternFill(props.PattFill, themeColors)
	case props.NoFill != nil:
		return ""
	}
	switch value := fill.(type) {
	case dml.SolidFill:
		return value.ApplyColorTransforms(value.Color)
	case *dml.SolidFill:
		return value.ApplyColorTransforms(value.Color)
	case dml.GradientFill:
		if len(value.Stops) > 0 {
			return value.Stops[0].Color
		}
	case *dml.GradientFill:
		if len(value.Stops) > 0 {
			return value.Stops[0].Color
		}
	}
	return ""
}

func (h *docxLayoutHandler) themeColors() map[string]string {
	if h == nil || h.session == nil || h.session.doc.Theme == nil {
		return nil
	}
	return h.session.doc.Theme.ColorScheme
}

func populateAnchoredImage(img *wordir.Image, raw wordsax.Event) {
	if img == nil || raw.DrawingModel == nil || raw.DrawingModel.Anchor == nil {
		return
	}
	anchor := raw.DrawingModel.Anchor
	placement := drawingPlacementFromAnchor(anchor)
	placement.Width, placement.Height = img.Width, img.Height
	if anchor.PositionH != nil {
		placement.X = anchor.PositionH.PosOffset
		placement.XRelative = anchor.PositionH.RelativeFrom
		placement.XAlign = anchor.PositionH.Align
		if anchor.PositionH.Pct != 0 {
			placement.X = anchor.PositionH.Pct
		}
	}
	if anchor.PositionV != nil {
		placement.Y = anchor.PositionV.PosOffset
		placement.YRelative = anchor.PositionV.RelativeFrom
		placement.YAlign = anchor.PositionV.Align
		if anchor.PositionV.Pct != 0 {
			placement.Y = anchor.PositionV.Pct
		}
	}
	placement.ApplyTo(img)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (h *docxLayoutHandler) appendNoteReference(kind string, event Event) error {
	id, err := strconv.Atoi(strings.TrimSpace(event.NoteID))
	if err != nil || id < 0 {
		return fmt.Errorf("word stream: invalid %s reference id %q", kind, event.NoteID)
	}
	if h.notes == nil {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	story, ok, err := h.notes.Load(kind, id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("word stream: %s story %d not found", kind, id)
	}
	story = h.session.resolveStoryFormatting(story)
	doc := &h.session.doc
	if kind == "footnote" {
		if doc.Footnotes == nil {
			doc.Footnotes = make(map[int]wordir.Story)
		}
		doc.Footnotes[id] = story
	} else {
		if doc.Endnotes == nil {
			doc.Endnotes = make(map[int]wordir.Story)
		}
		doc.Endnotes[id] = story
	}
	h.session.ensureParagraph()
	run := wordir.Run{Kind: kind + "Ref", NoteID: id, StyleID: event.RunStyleID}
	if event.RunFormat != nil {
		run.Format = *event.RunFormat
	}
	h.session.paragraph.Runs = append(h.session.paragraph.Runs, run)
	h.annotateLastRunContext()
	return nil
}

func (h *docxLayoutHandler) appendCommentReference(event Event) error {
	id, err := strconv.Atoi(strings.TrimSpace(event.CommentID))
	if err != nil || id < 0 {
		return fmt.Errorf("word stream: invalid comment reference id %q", event.CommentID)
	}
	if h.comments == nil {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	story, ok, err := h.comments.Load(id)
	if err != nil {
		return err
	}
	if ok {
		if h.session.doc.Comments == nil {
			h.session.doc.Comments = make(map[int]wordir.Story)
		}
		h.session.doc.Comments[id] = story
	}
	h.session.ensureParagraph()
	run := wordir.Run{Kind: "commentRef", CommentID: id, StyleID: event.RunStyleID}
	if event.RunFormat != nil {
		run.Format = *event.RunFormat
	}
	h.session.paragraph.Runs = append(h.session.paragraph.Runs, run)
	h.annotateLastRunContext()
	return nil
}

func (h *docxLayoutHandler) annotateLastRunContext() {
	if h.session.paragraph == nil || len(h.session.paragraph.Runs) == 0 {
		return
	}
	run := &h.session.paragraph.Runs[len(h.session.paragraph.Runs)-1]
	if len(h.hyperlinks) > 0 {
		link := h.hyperlinks[len(h.hyperlinks)-1]
		run.Target = link.target
		run.Anchor = link.anchor
		run.RelID = link.relID
	}
	if len(h.revisions) > 0 {
		revision := h.revisions[len(h.revisions)-1]
		run.Revision = revision.kind
		run.RevisionAuthor = revision.author
		run.RevisionDate = revision.date
		if revision.kind == "movedFrom" || revision.kind == "movedTo" {
			if id, err := strconv.Atoi(strings.TrimSpace(revision.id)); err == nil {
				run.MoveID = id
			}
		}
	}
}
func normalizeDOCXRevisionKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "ins":
		return "inserted"
	case "del":
		return "deleted"
	case "moveFrom":
		return "movedFrom"
	case "moveTo":
		return "movedTo"
	default:
		return kind
	}
}
func (h *docxLayoutHandler) annotateLastRun(instr string) {
	if h.session.paragraph == nil || len(h.session.paragraph.Runs) == 0 {
		return
	}
	run := &h.session.paragraph.Runs[len(h.session.paragraph.Runs)-1]
	run.FieldInstr = instr
	run.FieldKind = fieldKindFromInstruction(instr)
}

// isBlockCachedFieldInstruction identifies fields whose authored result is a
// block sequence rather than one inline value. A DOCX TOC/INDEX outer field can
// remain open across many result paragraphs, while nested PAGEREF/REF fields
// inside those paragraphs are ordinary inline fields. Stamping the outer field
// onto every visible result run causes the renderer to recalculate the whole
// TOC/INDEX once per cached entry. Keep those cached runs as ordinary content;
// nested inline fields still receive their own field semantics because they are
// the top frame while their result is emitted.
func isBlockCachedFieldInstruction(instr string) bool {
	switch fieldKindFromInstruction(instr) {
	case "TOC", "INDEX":
		return true
	default:
		return false
	}
}

func fieldKindFromInstruction(instr string) string {
	instr = strings.TrimSpace(instr)
	if instr == "" {
		return ""
	}
	fields := strings.Fields(instr)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}
