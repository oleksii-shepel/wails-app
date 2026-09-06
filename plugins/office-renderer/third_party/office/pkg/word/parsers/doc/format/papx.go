package docformat

import (
	"office/pkg/diag"
	docbinary "office/pkg/word/parsers/doc/binary"
	wordir "office/pkg/word/ir"
)

// PAPXMetadata carries paragraph/table semantics decoded from a PAPX/TAPX grpprl.
type PAPXMetadata struct {
	TableDepth         int
	TableRowProps      *TableRowProps
	PreWord97ListStart *int
	PropertyRevision   *PendingRevisionMark
	Diagnostics        diag.List
}

// PAPXEntry is one FC-indexed paragraph-formatting record from a PapxFkp page.
type PAPXEntry struct {
	FC                uint32
	Format            wordir.ParagraphFormat
	Meta              PAPXMetadata
	LegacyInTableHint bool
	LegacyRowMarkHint bool
	Sprms             []byte
}

// PAPXSemanticDecoder is retained only for compatibility tests and staged
// migration. Production parsing uses DecodePAPX when decode is nil.
type PAPXSemanticDecoder func(grpprl []byte) (wordir.ParagraphFormat, PAPXMetadata)

func ParsePAPXFKP(page []byte, decode PAPXSemanticDecoder) []PAPXEntry {
	raw, err := docbinary.ParsePAPXFKP(page)
	if err != nil || len(raw) == 0 {
		return nil
	}
	if decode == nil {
		decode = DecodePAPX
	}
	out := make([]PAPXEntry, 0, len(raw))
	for _, record := range raw {
		entry := PAPXEntry{FC: record.FC, Sprms: append([]byte(nil), record.Grpprl...)}
		if record.HasStyle {
			style := record.StyleIndex
			entry.Format.BinaryStyleIndex = &style
		}
		if len(record.Grpprl) > 0 {
			format, meta := decode(record.Grpprl)
			format.BinaryStyleIndex = entry.Format.BinaryStyleIndex
			meta.Diagnostics = scanUnmappedPAPX(record.Grpprl)
			entry.Format = format
			entry.Meta = meta
		}
		out = append(out, entry)
	}
	return out
}

// FindPAPXEntry returns the last entry whose FC is <= fc.
func FindPAPXEntry(entries []PAPXEntry, fc uint32) int {
	if len(entries) == 0 || entries[0].FC > fc { return -1 }
	lo, hi, best := 0, len(entries)-1, 0
	for lo <= hi {
		mid := (lo + hi) / 2
		if entries[mid].FC <= fc { best = mid; lo = mid + 1 } else { hi = mid - 1 }
	}
	return best
}
