package stream

import (
	"strconv"
	"strings"

	wordir "office/pkg/word/ir"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

type docxSemanticWrapper struct {
	inner           *docxLayoutHandler
	contentControls []*wordir.ContentControl
	customXML       []*wordir.CustomXmlInfo
	smartTags       []*wordir.SmartTagInfo
	directions      []string
}

func wrapDOCXLayoutSemantics(handler Handler) Handler {
	inner, ok := handler.(*docxLayoutHandler)
	if !ok || inner == nil {
		return handler
	}
	return &docxSemanticWrapper{inner: inner}
}

func (h *docxSemanticWrapper) HandleEvent(event Event) error {
	if h == nil || h.inner == nil || h.inner.session == nil {
		return nil
	}

	switch event.Type {
	case EventStartContentControl:
		raw, _ := event.Raw.(wordsax.Event)
		id, _ := strconv.Atoi(strings.TrimSpace(raw.ControlID))
		h.contentControls = append(h.contentControls, &wordir.ContentControl{
			ID:    id,
			Title: raw.ControlAlias,
			Tag:   raw.ControlTag,
			Type:  raw.ControlType,
		})
		return nil
	case EventEndContentControl:
		if len(h.contentControls) > 0 {
			h.contentControls = h.contentControls[:len(h.contentControls)-1]
		}
		return nil
	case EventStartCustomXML:
		raw, _ := event.Raw.(wordsax.Event)
		h.customXML = append(h.customXML, &wordir.CustomXmlInfo{
			URI:     raw.MarkupURI,
			Element: raw.MarkupElement,
		})
		return nil
	case EventEndCustomXML:
		if len(h.customXML) > 0 {
			h.customXML = h.customXML[:len(h.customXML)-1]
		}
		return nil
	case EventStartSmartTag:
		raw, _ := event.Raw.(wordsax.Event)
		h.smartTags = append(h.smartTags, &wordir.SmartTagInfo{
			URI:     raw.MarkupURI,
			Element: raw.MarkupElement,
		})
		return nil
	case EventEndSmartTag:
		if len(h.smartTags) > 0 {
			h.smartTags = h.smartTags[:len(h.smartTags)-1]
		}
		return nil
	case EventStartDirection:
		raw, _ := event.Raw.(wordsax.Event)
		h.directions = append(h.directions, strings.TrimSpace(raw.Direction))
		return nil
	case EventEndDirection:
		if len(h.directions) > 0 {
			h.directions = h.directions[:len(h.directions)-1]
		}
		return nil
	case EventCommentRangeStart:
		return h.appendCommentBoundary(event, true)
	case EventCommentRangeEnd:
		return h.appendCommentBoundary(event, false)
	case EventEmbeddedObject:
		return h.appendEmbeddedObject(event)
	case EventContentPart:
		// The mature semantic resolver intentionally omits contentPart references
		// from static layout when no cached text exists. Preserve that behaviour
		// here rather than turning a non-visible relationship into a layout error.
		return nil
	case EventAnnotationReference:
		return h.appendAnnotationReference(event)
	case EventAutoText:
		// day/month/year/pgNum tokens carry no cached display text. The mature
		// resolver omits them from static layout, so the streaming path does too.
		return nil
	case EventAltChunk:
		return h.appendAltChunk(event)
	}

	session := h.inner.session
	beforeRuns := 0
	if session.paragraph != nil {
		beforeRuns = len(session.paragraph.Runs)
	}
	beforeBlocks := len(session.doc.Blocks)

	if err := h.inner.HandleEvent(event); err != nil {
		return err
	}

	if session.paragraph != nil && len(session.paragraph.Runs) > beforeRuns {
		for i := beforeRuns; i < len(session.paragraph.Runs); i++ {
			h.decorateRun(&session.paragraph.Runs[i])
		}
	}
	if len(session.doc.Blocks) > beforeBlocks {
		for i := beforeBlocks; i < len(session.doc.Blocks); i++ {
			h.decorateBlock(&session.doc.Blocks[i])
		}
	}
	return nil
}

func (h *docxSemanticWrapper) appendCommentBoundary(event Event, start bool) error {
	raw, _ := event.Raw.(wordsax.Event)
	idText := event.CommentID
	if idText == "" {
		idText = raw.CommentID
	}
	id, err := strconv.Atoi(strings.TrimSpace(idText))
	if err != nil || id < 0 {
		return nil
	}
	h.inner.session.ensureParagraph()
	if start {
		h.inner.session.paragraph.CommentRangeStarts = append(h.inner.session.paragraph.CommentRangeStarts, id)
	} else {
		h.inner.session.paragraph.CommentRangeEnds = append(h.inner.session.paragraph.CommentRangeEnds, id)
	}
	return nil
}

func (h *docxSemanticWrapper) appendEmbeddedObject(event Event) error {
	raw, ok := event.Raw.(wordsax.Event)
	if !ok {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	label := strings.TrimSpace(raw.ProgID)
	if label == "" {
		label = "Object"
	}
	run := wordir.Run{
		Kind:      "olePlaceholder",
		Text:      label,
		OLERelID:  raw.RelationshipID,
		OLEProgID: raw.ProgID,
		OLEData:   append([]byte(nil), raw.PayloadData...),
		OLEMIME:   raw.PayloadContentType,
	}
	if raw.TargetExternal && len(raw.RelatedTargets) > 0 {
		run.OLELinkURL = raw.RelatedTargets[0]
	}
	h.inner.session.ensureParagraph()
	h.inner.session.paragraph.Text += label
	h.inner.session.paragraph.Runs = append(h.inner.session.paragraph.Runs, run)
	h.decorateRun(&h.inner.session.paragraph.Runs[len(h.inner.session.paragraph.Runs)-1])
	return nil
}

func (h *docxSemanticWrapper) appendAnnotationReference(event Event) error {
	h.inner.session.ensureParagraph()
	run := wordir.Run{Kind: "annotationRef", StyleID: event.RunStyleID}
	if event.RunFormat != nil {
		run.Format = *event.RunFormat
	}
	h.inner.session.paragraph.Runs = append(h.inner.session.paragraph.Runs, run)
	h.decorateRun(&h.inner.session.paragraph.Runs[len(h.inner.session.paragraph.Runs)-1])
	return nil
}

func (h *docxSemanticWrapper) appendAltChunk(event Event) error {
	raw, ok := event.Raw.(wordsax.Event)
	if !ok {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	session := h.inner.session
	if len(session.tables) > 0 {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	if session.paragraph != nil {
		if err := session.finishParagraph(); err != nil {
			return err
		}
	}
	block := wordir.Block{
		Kind: "altChunk",
		AltChunk: &wordir.AltChunk{
			Data:        append([]byte(nil), raw.PayloadData...),
			ContentType: raw.PayloadContentType,
		},
	}
	h.decorateBlock(&block)
	session.doc.Blocks = append(session.doc.Blocks, block)
	return session.flush(false)
}

func (h *docxSemanticWrapper) decorateRun(run *wordir.Run) {
	if run == nil {
		return
	}
	if len(h.contentControls) > 0 {
		cc := *h.contentControls[len(h.contentControls)-1]
		run.ContentControl = &cc
	}
	if len(h.customXML) > 0 {
		cx := *h.customXML[len(h.customXML)-1]
		run.CustomXml = &cx
	}
	if len(h.smartTags) > 0 {
		st := *h.smartTags[len(h.smartTags)-1]
		run.SmartTag = &st
	}
	if len(h.directions) > 0 {
		if direction := h.directions[len(h.directions)-1]; direction != "" {
			run.Format.Dir = direction
		}
	}
}

func (h *docxSemanticWrapper) decorateBlock(block *wordir.Block) {
	if block == nil {
		return
	}
	if len(h.contentControls) > 0 {
		cc := *h.contentControls[len(h.contentControls)-1]
		block.ContentControl = &cc
	}
	if len(h.customXML) > 0 {
		cx := *h.customXML[len(h.customXML)-1]
		block.CustomXml = &cx
	}
}
