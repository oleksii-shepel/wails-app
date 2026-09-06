package docformat

import docbinary "office/pkg/word/parsers/doc/binary"

// CHPXSemanticDecoder is retained as a narrow compatibility hook for callers
// that need to override Word97+ CHP interpretation. Production DOC parsing uses
// DecodeCHPX directly when decode is nil.
type CHPXSemanticDecoder func(grpprl []byte, fontNames []string) CHPXEntry

// ParseCHPXFKP decodes the FKP container through doc/binary and assembles
// canonical CHPX entries. Word97+ semantics are owned here; a non-nil decoder
// may override them only for compatibility/testing.
func ParseCHPXFKP(page []byte, fontNames []string, decode CHPXSemanticDecoder) []CHPXEntry {
	raw, err := docbinary.ParseCHPXFKP(page)
	if err != nil || len(raw) == 0 {
		return nil
	}
	if decode == nil {
		decode = DecodeCHPX
	}
	out := make([]CHPXEntry, 0, len(raw))
	for _, record := range raw {
		entry := CHPXEntry{FC: record.FC, Sprms: append([]byte(nil), record.Grpprl...)}
		if len(record.Grpprl) > 0 {
			decoded := decode(record.Grpprl, fontNames)
			decoded.FC = record.FC
			decoded.Sprms = append([]byte(nil), record.Grpprl...)
			entry = decoded
		}
		out = append(out, entry)
	}
	return out
}

// FindCHPXEntry returns the last entry whose FC is <= fc.
func FindCHPXEntry(entries []CHPXEntry, fc uint32) int {
	if len(entries) == 0 || entries[0].FC > fc { return -1 }
	lo, hi, best := 0, len(entries)-1, 0
	for lo <= hi {
		mid := (lo + hi) / 2
		if entries[mid].FC <= fc { best = mid; lo = mid + 1 } else { hi = mid - 1 }
	}
	return best
}
