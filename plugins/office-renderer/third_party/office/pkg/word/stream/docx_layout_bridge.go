package stream

import (
	"fmt"
	"strconv"
	"strings"

	wordir "office/pkg/word/ir"
	"office/pkg/ooxml/mathml"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

type docxFieldFrame struct {
	instruction strings.Builder
	result      bool
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
	session       *LayoutSession
	fields        []docxFieldFrame
	hyperlinks    []docxHyperlinkFrame
	revisions     []docxRevisionFrame
	pendingSimple string
	notes         *docxNoteLoader
	comments      *docxCommentLoader
	media         *docxMediaLoader
	headers        *docxHeaderFooterLoader
	lastHeaders   []wordir.Reference
	lastFooters   []wordir.Reference
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
		h.hyperlinks = append(h.hyperlinks, docxHyperlinkFrame{
			target: raw.HyperlinkTarget,
			anchor: raw.HyperlinkAnchor,
			relID:  raw.RelationshipID,
		})
		return nil
	case EventEndHyperlink:
		if len(h.hyperlinks) > 0 {
			h.hyperlinks = h.hyperlinks[:len(h.hyperlinks)-1]
		}
		return nil
	case EventStartRevision:
		raw, _ := event.Raw.(wordsax.Event)
		h.revisions = append(h.revisions, docxRevisionFrame{
			kind:   normalizeDOCXRevisionKind(raw.RevisionKind),
			author: raw.Author,
			date:   raw.Date,
			id:     raw.RevisionID,
		})
		return nil
	case EventEndRevision:
		if len(h.revisions) > 0 {
			h.revisions = h.revisions[:len(h.revisions)-1]
		}
		return nil
	case EventFieldInstruction:
		if len(h.fields) > 0 {
			frame := &h.fields[len(h.fields)-1]
			frame.instruction.WriteString(event.FieldInstr)
		} else {
			h.pendingSimple += event.FieldInstr
		}
		return nil
	case EventFieldChar:
		switch strings.ToLower(strings.TrimSpace(event.FieldCharType)) {
		case "begin":
			h.fields = append(h.fields, docxFieldFrame{})
		case "separate":
			if len(h.fields) > 0 {
				h.fields[len(h.fields)-1].result = true
			}
		case "end":
			if len(h.fields) > 0 {
				h.fields = h.fields[:len(h.fields)-1]
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
		switch raw.DrawingGraphicKind {
		case "picture":
			return h.appendPicture(event)
		case "shape":
			return appendDOCXShapeRun(h.session, event)
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

	if event.Type == EventText || event.Type == EventTab || event.Type == EventBreak {
		h.annotateLastRunContext()
		instr := ""
		if len(h.fields) > 0 {
			frame := &h.fields[len(h.fields)-1]
			if frame.result {
				instr = strings.TrimSpace(frame.instruction.String())
			}
		} else if h.pendingSimple != "" {
			instr = strings.TrimSpace(h.pendingSimple)
			h.pendingSimple = ""
		}
		if instr != "" {
			h.annotateLastRun(instr)
		}
	}
	return nil
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
		doc.Headers[ref.PartName] = story
	}
	for _, ref := range explicitFooters {
		if ref.PartName == "" {
			continue
		}
		story, err := h.headers.Load(ref)
		if err != nil {
			return err
		}
		doc.Footers[ref.PartName] = story
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
			ruby.FontSize = float64(pr.PhoneticGuideTextFontSize.Val) / 2.0
		}
		if pr.BaseTextFontSize != nil {
			ruby.BaseFontSize = float64(pr.BaseTextFontSize.Val) / 2.0
		}
		if pr.PhoneticGuideRaise != nil {
			ruby.Raise = float64(pr.PhoneticGuideRaise.Val) / 2.0
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

	img := &wordir.Image{
		MIMEType: mimeType,
		Data:     data,
		Width:    raw.WidthEMU,
		Height:   raw.HeightEMU,
		Alt:      firstNonEmptyString(raw.DrawingDescription, raw.DrawingName),
		Title:    raw.DrawingTitle,
		Hidden:   raw.DrawingHidden,
	}
	if raw.DrawingKind == "anchor" {
		populateAnchoredImage(img, raw)
	}

	h.session.ensureParagraph()
	run := wordir.Run{Kind: "image", Image: img, StyleID: event.RunStyleID}
	if event.RunFormat != nil {
		run.Format = *event.RunFormat
	}
	h.session.paragraph.Runs = append(h.session.paragraph.Runs, run)
	h.annotateLastRunContext()
	return nil
}

func populateAnchoredImage(img *wordir.Image, raw wordsax.Event) {
	if img == nil || raw.DrawingModel == nil || raw.DrawingModel.Anchor == nil {
		return
	}
	anchor := raw.DrawingModel.Anchor
	img.Floating = true
	img.ZIndex = anchor.RelativeHeight
	img.BehindDoc = anchor.BehindDoc != 0
	if anchor.PositionH != nil {
		img.X = anchor.PositionH.PosOffset
		img.XRelative = anchor.PositionH.RelativeFrom
		img.XAlign = anchor.PositionH.Align
		if anchor.PositionH.Pct != 0 {
			img.X = anchor.PositionH.Pct
		}
	}
	if anchor.PositionV != nil {
		img.Y = anchor.PositionV.PosOffset
		img.YRelative = anchor.PositionV.RelativeFrom
		img.YAlign = anchor.PositionV.Align
		if anchor.PositionV.Pct != 0 {
			img.Y = anchor.PositionV.Pct
		}
	}
	if anchor.WrapNone != nil {
		img.WrapType = "none"
	} else if anchor.WrapSquare != nil {
		img.WrapType = "square"
		img.WrapSide = anchor.WrapSquare.WrapText
	} else if anchor.WrapTight != nil {
		img.WrapType = "tight"
		img.WrapSide = anchor.WrapTight.WrapText
	} else if anchor.WrapThrough != nil {
		img.WrapType = "through"
		img.WrapSide = anchor.WrapThrough.WrapText
	} else if anchor.WrapTopAndBottom != nil {
		img.WrapType = "topAndBottom"
	}
	img.DistTop = float64(anchor.DistT) / 12700.0
	img.DistBottom = float64(anchor.DistB) / 12700.0
	img.DistLeft = float64(anchor.DistL) / 12700.0
	img.DistRight = float64(anchor.DistR) / 12700.0
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
	runKind := kind + "Ref"
	run := wordir.Run{Kind: runKind, NoteID: id, StyleID: event.RunStyleID}
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
	if !ok {
		return fmt.Errorf("word stream: comment story %d not found", id)
	}
	if h.session.doc.Comments == nil {
		h.session.doc.Comments = make(map[int]wordir.Story)
	}
	h.session.doc.Comments[id] = story
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

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
