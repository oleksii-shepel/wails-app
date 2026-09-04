package sheet

import (
	"io"

	sheetsax "office/pkg/sheet/parsers/ooxml/sax"
)

// EventType identifies a streamed SpreadsheetML event.
type EventType = sheetsax.EventType

const (
	EventStartWorkbook = sheetsax.EventStartWorkbook
	EventEndWorkbook   = sheetsax.EventEndWorkbook
	EventStartSheet    = sheetsax.EventStartSheet
	EventEndSheet      = sheetsax.EventEndSheet
	EventStartRow      = sheetsax.EventStartRow
	EventEndRow        = sheetsax.EventEndRow
	EventCell          = sheetsax.EventCell
	EventComment       = sheetsax.EventComment
)

// Event is a single streamed SpreadsheetML event.
type Event = sheetsax.Event

// EventHandler consumes spreadsheet events from the high-level package facade.
type EventHandler = sheetsax.Handler

// EventHandlerFunc adapts a function to EventHandler.
type EventHandlerFunc = sheetsax.HandlerFunc

// EventOptions controls which XLSX parts/events are emitted.
type EventOptions = sheetsax.Options

// WalkPath streams SpreadsheetML events from an XLSX-family file on disk.
func WalkPath(path string, handler EventHandler, opts EventOptions) error {
	return sheetsax.WalkPath(path, handler, opts)
}

// WalkBytes streams SpreadsheetML events from an in-memory XLSX-family payload.
func WalkBytes(data []byte, handler EventHandler, opts EventOptions) error {
	return sheetsax.WalkBytes(data, handler, opts)
}

// WalkStream streams SpreadsheetML events from a non-seekable XLSX-family stream.
func WalkStream(r io.Reader, handler EventHandler, opts EventOptions) error {
	return sheetsax.WalkStream(r, handler, opts)
}

// WalkReaderAt streams SpreadsheetML events from a seekable XLSX-family reader.
func WalkReaderAt(r io.ReaderAt, size int64, handler EventHandler, opts EventOptions) error {
	return sheetsax.WalkReaderAt(r, size, handler, opts)
}
