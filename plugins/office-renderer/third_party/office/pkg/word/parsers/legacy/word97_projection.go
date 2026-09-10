package legacy

import (
	"bytes"
	"fmt"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

// loadCanonicalWord97Document is the single semantic projection from the
// canonical Word97 snapshot into the public Word IR. Word97 binary state never
// enters legacyDocParseData; that compatibility carrier is reserved for pre-97.
func loadCanonicalWord97Document(filename string, wordStream []byte, f *cfb.File) (*wordir.Document, error) {
	wordReader := bytes.NewReader(wordStream)
	snapshot, err := word97.OpenSnapshotReaderAt(wordReader, int64(len(wordStream)), f)
	if err != nil {
		return nil, fmt.Errorf("legacy doc: canonical Word97 snapshot: %w", err)
	}
	if snapshot == nil || snapshot.Core == nil {
		return nil, fmt.Errorf("legacy doc: canonical Word97 snapshot is incomplete")
	}
	opened := snapshot.Core
	canonicalRuns := word97.MaterializeRunRange(
		wordReader,
		int64(len(wordStream)),
		opened,
		word97.StoryRange{StartCP: 0, EndCP: opened.Metadata.StoryCounts.Text},
	)
	canonicalRuns, err = word97.EnrichMaterializedRuns(canonicalRuns, opened.Metadata, snapshot.Context, f)
	if err != nil {
		return nil, err
	}
	runs := adaptCanonicalWord97Runs(canonicalRuns)

	visible := runs[:0]
	for _, run := range runs {
		if run.Format.Vanish != nil && *run.Format.Vanish {
			continue
		}
		visible = append(visible, run)
	}

	input := word97BinaryDocumentInput(snapshot, visible)
	doc := buildBinaryWordDocument(filename, input)
	applyLegacyDocProperties(doc, loadLegacyDocProperties(f))

	word97.ApplySnapshotDocumentState(doc, snapshot)

	// Complete document-level Word97 semantics after the shared IR builder has
	// assembled paragraphs/tables. Binary and compatibility ownership remains
	// canonical; legacy hosts only the final public-IR assembly boundary.
	word97.ApplyDocumentPostProjection(doc, opened.Metadata.BookmarkStarts)
	word97.RecoverDetachedDrawingImages(doc, wordStream)
	word97.RecoverOLECompoundPayloads(f, doc)
	word97.RecoverOLEPreviews(f, doc)

	word97.ApplySnapshotDiagnostics(doc, snapshot)
	return doc, nil
}
