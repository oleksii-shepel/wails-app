package word97

import (
	"io"

	"office/pkg/ooxml/cfb"
)

// LoadDocumentContextReaderAt materializes the canonical document-wide context
// and drawing/media projection without applying OpenReaderAt validation or
// cursor decorators. This keeps compatibility adapters from needing to know
// which document-wide decoders must be composed while preserving their historic
// tolerance of partially-resolved side stories.
func LoadDocumentContextReaderAt(word io.ReaderAt, wordSize int64, f *cfb.File, opened *OpenCore) *Context {
	context := LoadContextReaderAt(word, wordSize, f, opened)
	if context == nil {
		context = &Context{}
	}
	if word == nil || opened == nil {
		return context
	}

	applyCanonicalCommentMetadata(context, opened)
	drawings := ResolveDrawingProjection(word, wordSize, f, opened)
	context.Drawings = drawings
	context.OLEByCP = drawings.OLEByCP
	return context
}
