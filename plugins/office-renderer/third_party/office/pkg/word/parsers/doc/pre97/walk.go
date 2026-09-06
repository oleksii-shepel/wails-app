package pre97

import (
	"strings"

	wordir "office/pkg/word/ir"
)

func numberingReference(run Run) *wordir.NumberingReference {
	if run.ParagraphFormat.BinaryListOverride == nil || run.ParagraphFormat.BinaryListLevel == nil {
		return nil
	}
	numID := *run.ParagraphFormat.BinaryListOverride
	level := *run.ParagraphFormat.BinaryListLevel
	if numID <= 0 || level < 0 {
		return nil
	}
	return &wordir.NumberingReference{NumID: numID, Level: level}
}

func tableDepth(run Run) int {
	if run.TableDepth > 0 {
		return run.TableDepth
	}
	if run.ParagraphFormat.InTable || run.LegacyInTableHint || strings.ContainsRune(run.Text, '\a') {
		return 1
	}
	return 0
}

func validateRun(run Run) error {
	if run.Spec && run.Data && run.Image == nil && run.Shape == nil {
		return UnsupportedError{Feature: "picture/data anchor"}
	}
	if run.Spec && run.Obj {
		return UnsupportedError{Feature: "OLE object anchor"}
	}
	if strings.ContainsAny(run.Text, "\x13\x14\x15") {
		return UnsupportedError{Feature: "field"}
	}
	return nil
}

// Walk owns the bounded pre-Word97 event state machine. The cursor may still be
// produced by a compatibility decoder, but paragraph/table/section/reference
// orchestration is canonical and independent of parsers/legacy.
func Walk(cursor Cursor, context *Context, handler EventHandler) error {
	if cursor == nil {
		return UnsupportedError{Feature: "nil run cursor"}
	}
	start := Event{Type: EventStartDocument}
	if context != nil {
		start.Headers = context.Headers
		start.Footers = context.Footers
		if context.FootnoteSeparator.Text != "" { s := context.FootnoteSeparator; start.FootnoteSeparator = &s }
		if context.FootnoteContinuationSep.Text != "" { s := context.FootnoteContinuationSep; start.FootnoteContinuationSep = &s }
		if context.FootnoteContinuationNotice.Text != "" { s := context.FootnoteContinuationNotice; start.FootnoteContinuationNotice = &s }
		if context.EndnoteSeparator.Text != "" { s := context.EndnoteSeparator; start.EndnoteSeparator = &s }
		if context.EndnoteContinuationSep.Text != "" { s := context.EndnoteContinuationSep; start.EndnoteContinuationSep = &s }
		if context.EndnoteContinuationNotice.Text != "" { s := context.EndnoteContinuationNotice; start.EndnoteContinuationNotice = &s }
	}
	if err := handler(start); err != nil { return err }

	paragraphOpen := false
	tableLevel := 0
	rowOpen := make([]bool, 0, 2)
	cellOpen := make([]bool, 0, 2)
	sectionIndex := 0

	openParagraph := func(run Run) error {
		if paragraphOpen { return nil }
		paragraphOpen = true
		format := run.ParagraphFormat
		return handler(Event{Type: EventStartParagraph, ParagraphFormat: &format, ParagraphNumbering: numberingReference(run)})
	}
	closeParagraph := func(run Run, force bool) error {
		if !paragraphOpen {
			if !force { return nil }
			if err := openParagraph(run); err != nil { return err }
		}
		paragraphOpen = false
		return handler(Event{Type: EventEndParagraph})
	}
	ensureTableLevel := func(depth int) error {
		for tableLevel < depth {
			if err := handler(Event{Type: EventStartTable}); err != nil { return err }
			tableLevel++
			rowOpen = append(rowOpen, false)
			cellOpen = append(cellOpen, false)
		}
		if depth > 0 {
			level := depth - 1
			if !rowOpen[level] { if err := handler(Event{Type: EventStartTableRow}); err != nil { return err }; rowOpen[level] = true }
			if !cellOpen[level] { if err := handler(Event{Type: EventStartTableCell}); err != nil { return err }; cellOpen[level] = true }
		}
		return nil
	}
	closeTableToDepth := func(depth int) error {
		if paragraphOpen { if err := closeParagraph(Run{}, false); err != nil { return err } }
		for tableLevel > depth {
			level := tableLevel - 1
			if cellOpen[level] { if err := handler(Event{Type: EventEndTableCell}); err != nil { return err }; cellOpen[level] = false }
			if rowOpen[level] { if err := handler(Event{Type: EventEndTableRow}); err != nil { return err }; rowOpen[level] = false }
			if err := handler(Event{Type: EventEndTable}); err != nil { return err }
			tableLevel--
			rowOpen = rowOpen[:tableLevel]
			cellOpen = cellOpen[:tableLevel]
		}
		return nil
	}
	emitSectionsThrough := func(cp int) error {
		if context == nil { return nil }
		for sectionIndex < len(context.Sections) {
			sec := context.Sections[sectionIndex]
			end := 0
			if sec.Source != nil { end = sec.Source.EndCP }
			if end > cp { return nil }
			if err := closeTableToDepth(0); err != nil { return err }
			copySec := sec
			if err := handler(Event{Type: EventSection, Section: &copySec}); err != nil { return err }
			sectionIndex++
		}
		return nil
	}
	emitText := func(run Run, text string) error {
		if text == "" { return nil }
		format := run.Format
		return handler(Event{Type: EventText, Text: text, RunFormat: &format, FieldKind: run.FieldKind, FieldInstr: run.FieldInstr, FieldArg: run.FieldArg, Target: run.Target, Anchor: run.Anchor})
	}

	for {
		run, ok, err := cursor.Next()
		if err != nil { return err }
		if !ok { break }
		if err := validateRun(run); err != nil { return err }
		if err := emitSectionsThrough(run.StartCP); err != nil { return err }

		if run.Kind == "bookmarkStart" || run.Kind == "bookmarkEnd" {
			if err := openParagraph(run); err != nil { return err }
			t := EventBookmarkStart
			if run.Kind == "bookmarkEnd" { t = EventBookmarkEnd }
			if err := handler(Event{Type: t, BookmarkName: run.Anchor}); err != nil { return err }
			continue
		}
		if run.Kind == "olePlaceholder" {
			if err := openParagraph(run); err != nil { return err }
			var embedded *wordir.Run
			if context != nil && context.OLEByCP != nil { if value := context.OLEByCP[run.StartCP]; value != nil { copyRun := *value; embedded = &copyRun } }
			if embedded == nil { embedded = &wordir.Run{Kind: "olePlaceholder", Text: "Embedded Object", OLEMIME: "application/octet-stream", OLEType: "Embed"} }
			format := run.Format
			embedded.Format = format
			if err := handler(Event{Type: EventEmbeddedObject, RunFormat: &format, EmbeddedRun: embedded}); err != nil { return err }
			continue
		}
		if run.Kind == "footnoteRef" || run.Kind == "endnoteRef" || run.Kind == "commentRef" {
			if err := openParagraph(run); err != nil { return err }
			format := run.Format
			ev := Event{RunFormat: &format}
			switch run.Kind {
			case "footnoteRef":
				ev.Type, ev.NoteID = EventFootnoteReference, run.NoteID
				if context != nil { if story, ok := context.Footnotes[run.NoteID]; ok { s := story; ev.Story = &s } }
			case "endnoteRef":
				ev.Type, ev.NoteID = EventEndnoteReference, run.NoteID
				if context != nil { if story, ok := context.Endnotes[run.NoteID]; ok { s := story; ev.Story = &s } }
			default:
				ev.Type, ev.CommentID = EventCommentReference, run.CommentID
				if context != nil { if story, ok := context.Comments[run.CommentID]; ok { s := story; ev.Story = &s } }
			}
			if err := handler(ev); err != nil { return err }
			continue
		}

		depth := tableDepth(run)
		if depth < tableLevel { if err := closeTableToDepth(depth); err != nil { return err } }
		if depth > 0 { if err := ensureTableLevel(depth); err != nil { return err } } else if tableLevel > 0 { if err := closeTableToDepth(0); err != nil { return err } }

		text := run.Text
		if run.Image != nil || run.Shape != nil {
			if err := openParagraph(run); err != nil { return err }
			format := run.Format
			if err := handler(Event{Type: EventDrawing, RunFormat: &format, Image: run.Image, Shape: run.Shape}); err != nil { return err }
			text = strings.ReplaceAll(text, "\x08", "")
			text = strings.ReplaceAll(text, "\x01", "")
		}
		for len(text) > 0 {
			i := strings.IndexAny(text, "\r\t\v\f\a")
			if i < 0 { if err := openParagraph(run); err != nil { return err }; if err := emitText(run, text); err != nil { return err }; break }
			if i > 0 { if err := openParagraph(run); err != nil { return err }; if err := emitText(run, text[:i]); err != nil { return err } }
			ch := text[i]
			text = text[i+1:]
			switch ch {
			case '\r':
				if err := closeParagraph(run, true); err != nil { return err }
			case '\a':
				if !run.ParagraphFormat.IsTableTrailingParagraph { if err := closeParagraph(run, paragraphOpen); err != nil { return err } }
				if tableLevel > 0 {
					level := tableLevel - 1
					if cellOpen[level] { if err := handler(Event{Type: EventEndTableCell}); err != nil { return err }; cellOpen[level] = false }
					if run.ParagraphFormat.IsTableTrailingParagraph && rowOpen[level] { if err := handler(Event{Type: EventEndTableRow}); err != nil { return err }; rowOpen[level] = false }
				}
			case '\t':
				if err := openParagraph(run); err != nil { return err }; format := run.Format; if err := handler(Event{Type: EventTab, RunFormat: &format}); err != nil { return err }
			case '\v':
				if err := openParagraph(run); err != nil { return err }; format := run.Format; if err := handler(Event{Type: EventBreak, BreakType: "line", RunFormat: &format}); err != nil { return err }
			case '\f':
				if err := openParagraph(run); err != nil { return err }; format := run.Format; if err := handler(Event{Type: EventBreak, BreakType: "page", RunFormat: &format}); err != nil { return err }
			}
		}
	}

	if paragraphOpen { if err := closeParagraph(Run{}, false); err != nil { return err } }
	if err := closeTableToDepth(0); err != nil { return err }
	if context != nil {
		for sectionIndex < len(context.Sections) {
			sec := context.Sections[sectionIndex]
			copySec := sec
			if err := handler(Event{Type: EventSection, Section: &copySec}); err != nil { return err }
			sectionIndex++
		}
	}
	return handler(Event{Type: EventEndDocument})
}
