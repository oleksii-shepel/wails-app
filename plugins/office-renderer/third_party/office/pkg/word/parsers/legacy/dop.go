package legacy

import (
	"encoding/binary"

	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

type legacyDocSettings struct {
	DefaultTabStop        int
	FootnoteNumberFormat  string
	FootnoteNumberStart   int
	FootnoteRestart       string
	EndnotePosition       string
	EndnoteNumberFormat   string
	EndnoteNumberStart    int
	EndnoteRestart        string
	ZoomPercent           int
	ViewMode              string
	ProtectionEnabled     bool
	DocumentProtection    *wordir.DocumentProtection
	CreatedAt             string
	ModifiedAt            string
	LastPrintedAt         string
	WordCount             int
	CharCount             int
	PageCount             int
	ParagraphCount        int
	MirrorMargins         bool
	TrackChangesEnabled   bool
}

// loadLegacyDocSettings is retained only as a compatibility surface for old
// tests/helpers. DOP semantic decoding belongs to doc/word97.
func loadLegacyDocSettings(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) legacyDocSettings {
	dop, ok := loadLegacyDopRecord(wordStream, tableStream, fibBase, cbRgFcLcb)
	if !ok {
		return legacyDocSettings{}
	}
	return legacyDocSettingsFromSequential(word97.DecodeDocumentSettings(dop))
}

// loadLegacyDopRecord retains the historical synthetic-FIB adapter. Normal
// Word97 parsing consumes OpenCore.RawDOP and never calls this function.
func loadLegacyDopRecord(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) ([]byte, bool) {
	const dopFibIndex = 31
	if dopFibIndex >= cbRgFcLcb || fibBase+(dopFibIndex+1)*8 > len(wordStream) {
		return nil, false
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[fibBase+dopFibIndex*8:]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[fibBase+dopFibIndex*8+4:]))
	if lcb < 12 || fc < 0 || fc+lcb > len(tableStream) {
		return nil, false
	}
	return tableStream[fc : fc+lcb], true
}

// loadLegacyDocCompatibility is a compatibility alias for tests and old
// projection helpers. The canonical Word97 DOP decoder owns the bit layout.
func loadLegacyDocCompatibility(dop []byte) *wordir.DocCompatibility {
	return word97.DecodeDocumentSettings(dop).Compatibility
}

// legacyDopViewMode remains for historical test helpers; production decoding
// uses word97.DecodeDocumentSettings.
func legacyDopViewMode(v byte) string {
	switch v {
	case 1:
		return "print"
	case 2:
		return "outline"
	case 3:
		return "masterDoc"
	case 4:
		return "normal"
	case 5:
		return "online"
	case 6:
		return "web"
	default:
		return ""
	}
}
