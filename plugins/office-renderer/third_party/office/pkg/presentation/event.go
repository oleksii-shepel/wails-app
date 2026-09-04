package presentation

import (
	"io"

	presax "office/pkg/presentation/parsers/ooxml/sax"
)

// EventType identifies a streamed PresentationML event.
type EventType = presax.EventType

const (
	EventText = presax.EventText
)

// Event is a single streamed PresentationML event.
type Event = presax.Event

// EventHandler consumes presentation events from the high-level package facade.
type EventHandler = presax.Handler

// EventHandlerFunc adapts a function to EventHandler.
type EventHandlerFunc = presax.HandlerFunc

// EventOptions controls which PPTX parts/events are emitted.
type EventOptions = presax.Options

// WalkPath streams PresentationML events from a PPTX-family file on disk.
func WalkPath(path string, handler EventHandler, opts EventOptions) error {
	return presax.WalkPath(path, handler, opts)
}

// WalkBytes streams PresentationML events from an in-memory PPTX-family payload.
func WalkBytes(data []byte, handler EventHandler, opts EventOptions) error {
	return presax.WalkBytes(data, handler, opts)
}

// WalkStream streams PresentationML events from a non-seekable PPTX-family stream.
func WalkStream(r io.Reader, handler EventHandler, opts EventOptions) error {
	return presax.WalkStream(r, handler, opts)
}

// WalkReaderAt streams PresentationML events from a seekable PPTX-family reader.
func WalkReaderAt(r io.ReaderAt, size int64, handler EventHandler, opts EventOptions) error {
	return presax.WalkReaderAt(r, size, handler, opts)
}
