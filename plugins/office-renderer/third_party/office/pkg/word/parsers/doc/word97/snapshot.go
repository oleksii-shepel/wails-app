package word97

import (
	"io"

	"office/pkg/ooxml/cfb"
)

// Snapshot is the complete canonical Word97 document-wide decode state used by
// higher parser layers. Core owns bounded binary discovery; Context owns
// materialized side stories and drawing/media projection.
type Snapshot struct {
	Core    *OpenCore
	Context *Context
}

// OpenSnapshotReaderAt performs the complete canonical Word97 document open
// without applying public-pipeline validation or cursor decorators. Compatibility
// adapters can consume this snapshot without orchestrating Word97 loaders.
func OpenSnapshotReaderAt(word io.ReaderAt, wordSize int64, f *cfb.File) (*Snapshot, error) {
	core, err := OpenCoreReaderAt(word, wordSize, f)
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		Core:    core,
		Context: LoadDocumentContextReaderAt(word, wordSize, f, core),
	}, nil
}
