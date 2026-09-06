package stream

import (
	"fmt"
	"strings"

	"office/pkg/ooxml/chart"
	wordir "office/pkg/word/ir"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

type docxDrawingSemanticHandler struct {
	inner  Handler
	layout *docxLayoutHandler
}

func wrapDOCXDrawingSemantics(inner Handler, layout *docxLayoutHandler) Handler {
	if inner == nil || layout == nil || layout.session == nil {
		return inner
	}
	return &docxDrawingSemanticHandler{inner: inner, layout: layout}
}

func (h *docxDrawingSemanticHandler) HandleEvent(event Event) error {
	if event.Type == EventAltChunk && len(h.layout.session.tables) > 0 {
		return h.appendTableAltChunk(event)
	}
	if event.Type != EventDrawing {
		return h.inner.HandleEvent(event)
	}
	raw, ok := event.Raw.(wordsax.Event)
	if !ok {
		return h.inner.HandleEvent(event)
	}
	switch raw.DrawingGraphicKind {
	case "chart":
		return h.appendChart(event, raw)
	case "diagram":
		// The materialized Word resolver currently preserves SmartArt package
		// parts for round-trip but does not project a dedicated SmartArt node into
		// word/ir. Match that render-time behavior rather than forcing a fallback.
		return nil
	default:
		return h.inner.HandleEvent(event)
	}
}

func (h *docxDrawingSemanticHandler) appendChart(event Event, raw wordsax.Event) error {
	if h.layout.media == nil {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	target := ""
	for _, part := range raw.RelatedParts {
		if strings.HasSuffix(strings.ToLower(part.RelationshipType), "/chart") {
			target = part.Target
			break
		}
	}
	if target == "" && strings.Contains(strings.ToLower(raw.RelationshipType), "/chart") {
		target = raw.RelationshipTarget
	}
	if target == "" {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	data, _, err := h.layout.media.Load(target)
	if err != nil {
		return err
	}
	parsed, err := chart.ParseChart(data)
	if err != nil || parsed == nil {
		if err != nil { return fmt.Errorf("word stream: parse chart %q: %w", target, err) }
		return fmt.Errorf("word stream: chart %q produced no model", target)
	}
	if theme := h.layout.session.doc.Theme; theme != nil && len(theme.ColorScheme) > 0 {
		parsed.ThemeColors = make(map[string]string, len(theme.ColorScheme))
		for slot, color := range theme.ColorScheme {
			parsed.ThemeColors[slot] = color
		}
	}
	h.layout.session.ensureParagraph()
	run := wordir.Run{Kind: "chart", Chart: parsed, StyleID: event.RunStyleID}
	if event.RunFormat != nil { run.Format = *event.RunFormat }
	h.layout.session.paragraph.Runs = append(h.layout.session.paragraph.Runs, run)
	h.layout.annotateLastRunContext()
	return nil
}

func (h *docxDrawingSemanticHandler) appendTableAltChunk(event Event) error {
	raw, ok := event.Raw.(wordsax.Event)
	if !ok {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	session := h.layout.session
	if session.paragraph != nil {
		if err := session.finishParagraph(); err != nil { return err }
	}
	frame := session.topTable()
	if frame == nil || frame.cell == nil {
		return fmt.Errorf("word stream: altChunk encountered inside table but outside cell")
	}
	block := wordir.Block{
		Kind: "altChunk",
		AltChunk: &wordir.AltChunk{
			Data: append([]byte(nil), raw.PayloadData...),
			ContentType: raw.PayloadContentType,
		},
	}
	if semantic, ok := h.inner.(*docxSemanticWrapper); ok {
		semantic.decorateBlock(&block)
	}
	frame.cell.Blocks = append(frame.cell.Blocks, block)
	return nil
}
