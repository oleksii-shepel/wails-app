package word

import (
	"io"

	wordsax "office/pkg/word/parsers/ooxml/sax"
)

// EventType identifies a streamed WordprocessingML event.
type EventType = wordsax.EventType

const (
	EventText = wordsax.EventText
)

// StoryKind identifies which Word story a streamed event belongs to.
type StoryKind = wordsax.StoryKind

// RelatedPart describes a relationship-backed payload referenced by an event.
type RelatedPart = wordsax.RelatedPart

// MathNode is the normalized OMML tree attached to math events.
type MathNode = wordsax.MathNode

// Event is a single streamed WordprocessingML event.
type Event = wordsax.Event

// EventHandler consumes Word events from the high-level package facade.
type EventHandler = wordsax.Handler

// EventHandlerFunc adapts a function to EventHandler.
type EventHandlerFunc = wordsax.HandlerFunc

// EventOptions controls which OOXML Word stories/events are emitted.
type EventOptions = wordsax.Options

// WalkPath streams OOXML WordprocessingML events from a DOCX-family file on disk.
func WalkPath(path string, handler EventHandler, opts EventOptions) error {
	return wordsax.WalkPath(path, handler, opts)
}

// WalkBytes streams OOXML WordprocessingML events from an in-memory DOCX-family payload.
func WalkBytes(data []byte, handler EventHandler, opts EventOptions) error {
	return wordsax.WalkBytes(data, handler, opts)
}

// WalkStream streams OOXML WordprocessingML events from a non-seekable DOCX-family stream.
func WalkStream(r io.Reader, handler EventHandler, opts EventOptions) error {
	return wordsax.WalkStream(r, handler, opts)
}

// WalkReaderAt streams OOXML WordprocessingML events from a seekable DOCX-family reader.
func WalkReaderAt(r io.ReaderAt, size int64, handler EventHandler, opts EventOptions) error {
	return wordsax.WalkReaderAt(r, size, handler, opts)
}
