package legacy

import (
	"regexp"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// InspectPreWord97TextSpanPreview decodes and normalizes a bounded Word 6/95
// text-like span using the same charset heuristic as the live parser. It is
// intended for diagnostics around observed header-owned spans such as the
// secondary 0x58/0x5c block.
func InspectPreWord97TextSpanPreview(wordStream []byte, start, size int) string {
	if start <= 0 || size <= 0 || start >= len(wordStream) || start+size > len(wordStream) {
		return ""
	}
	var preferred *charmap.Charmap
	if span, ok := InspectPreWord97BodySpan(wordStream); ok {
		preferred = guessPreWord97Charset(wordStream, span)
	}
	text := normalizeLegacyText(decodePreWord97SingleByteTextWithCharset(wordStream[start:start+size], preferred))
	const previewLimit = 160
	runes := []rune(text)
	if len(runes) > previewLimit {
		return string(runes[:previewLimit]) + "..."
	}
	return text
}

var preWord97SecondaryAsciiPhrase = regexp.MustCompile(`[A-Za-z][A-Za-z ]{1,48}`)
var preWord97SecondaryCanonicalNames = []string{
	"Normal",
	"Default Paragraph Font",
	"Heading",
	"Text body",
	"List",
	"Caption",
	"Index",
}

func canonicalizePreWord97SecondaryStyleName(candidate string) string {
	lower := strings.ToLower(candidate)
	for _, canonical := range preWord97SecondaryCanonicalNames {
		key := strings.ToLower(canonical)
		if strings.Contains(lower, key) {
			return canonical
		}
	}
	return candidate
}

func isKnownPreWord97SecondaryStyleName(name string) bool {
	for _, canonical := range preWord97SecondaryCanonicalNames {
		if strings.EqualFold(name, canonical) {
			return true
		}
	}
	return false
}

// InspectPreWord97SecondaryStyleNames extracts style-name-like phrases from
// the observed secondary pre-97 text span (header slots 0x58/0x5c).
//
// This is an evidence helper, not a full old STSH parser.
func InspectPreWord97SecondaryStyleNames(wordStream []byte) []string {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok || span.SecondaryStart <= 0 || span.SecondarySize <= 0 || span.SecondaryStart+span.SecondarySize > len(wordStream) {
		return nil
	}
	var preferred *charmap.Charmap
	preferred = guessPreWord97Charset(wordStream, span)
	decoded := normalizeLegacyText(decodePreWord97SingleByteTextWithCharset(wordStream[span.SecondaryStart:span.SecondaryStart+span.SecondarySize], preferred))
	if decoded == "" {
		return nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, 8)
	for _, match := range preWord97SecondaryAsciiPhrase.FindAllString(decoded, -1) {
		candidate := strings.TrimSpace(strings.Join(strings.Fields(match), " "))
		if len(candidate) < 3 {
			continue
		}
		candidate = canonicalizePreWord97SecondaryStyleName(candidate)
		key := strings.ToLower(candidate)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, candidate)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func preWord97SecondaryStyleNameByIndex(wordStream []byte) map[int]string {
	filtered := preWord97SecondaryKnownStyleNames(wordStream)
	if len(filtered) == 0 {
		return nil
	}
	indexToName := make(map[int]string, len(filtered))
	for index, name := range filtered {
		indexToName[index] = name
	}
	return indexToName
}

func preWord97SecondaryKnownStyleNames(wordStream []byte) []string {
	names := InspectPreWord97SecondaryStyleNames(wordStream)
	if len(names) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if !isKnownPreWord97SecondaryStyleName(name) {
			continue
		}
		filtered = append(filtered, name)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

// LoadPreWord97FontTable reads the Word 6 / Word 95 font (FFN) table from the
// WordDocument stream at the FontTableOffset given by InspectPreWord97BodySpan.
// Layout follows HWPF's OldFontTable + OldFfn:
//
//	header (2 bytes) : redundant table length
//	per entry:
//	  1 byte  : record length (excluding the leading byte)
//	  3 bytes : reserved/unknown
//	  1 byte  : chs (character set hint)
//	  1 byte  : reserved/unknown
//	  N bytes : primary font name, NUL-terminated, single-byte (Windows codepage)
//	  M bytes : optional alternate name, NUL-terminated
//
// Returns nil when the offset/size is invalid or the table can't be decoded.
// Names are interpreted as Windows-1252; per HWPF's note, full per-entry
// codepage selection from the chs byte is left as a future refinement.
func LoadPreWord97FontTable(wordStream []byte, offset, size int) []PreWord97FontEntry {
	if offset <= 0 || size < 6 || offset+size > len(wordStream) {
		return nil
	}
	buf := wordStream[offset : offset+size]
	pos := 2 // skip the 2-byte redundant length header
	var out []PreWord97FontEntry
	for pos < len(buf) {
		if pos+6 > len(buf) {
			break
		}
		recLen := int(buf[pos])
		if recLen == 0 || pos+1+recLen > len(buf) {
			break
		}
		// Layout per HWPF OldFfn:
		//   pos+0 : recLen (excluded from body)
		//   pos+1..3 : reserved
		//   pos+4 : chs
		//   pos+5 : reserved
		//   pos+6 : start of primary font name
		entry := PreWord97FontEntry{Charset: buf[pos+4]}
		nameStart := pos + 6
		if nameStart >= pos+1+recLen {
			pos += 1 + recLen
			continue
		}
		nameEnd := nameStart
		for nameEnd < pos+1+recLen && buf[nameEnd] != 0 {
			nameEnd++
		}
		entry.Name = decodeLegacyWin1252(buf[nameStart:nameEnd])
		altStart := nameEnd + 1 // skip NUL
		if altStart < pos+1+recLen {
			altEnd := altStart
			for altEnd < pos+1+recLen && buf[altEnd] != 0 {
				altEnd++
			}
			if altEnd > altStart {
				entry.AltName = decodeLegacyWin1252(buf[altStart:altEnd])
			}
		}
		out = append(out, entry)
		pos += 1 + recLen
	}
	return out
}

// decodeLegacyWin1252 maps a single-byte buffer to a Go string assuming
// Windows-1252. Bytes 0x00–0x7F are ASCII; 0x80–0x9F use the Windows-1252
// code-page mapping; 0xA0–0xFF are Latin-1 supplement.
//
// This is a deliberate stand-in for full per-font charset decoding. HWPF
// chooses a charset per OldFfn entry from the chs byte but its rendering only
// uses the result for non-ASCII glyphs; for the repo-local Word 95 fixtures
// the ASCII subset is enough to exercise the round-trip.
func decodeLegacyWin1252(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	out := make([]rune, 0, len(b))
	for _, c := range b {
		switch {
		case c < 0x80:
			out = append(out, rune(c))
		case c >= 0xA0:
			out = append(out, rune(c))
		default:
			out = append(out, win1252High[c-0x80])
		}
	}
	return string(out)
}

// win1252High covers Windows-1252 code points 0x80–0x9F. Reserved positions
// (0x81, 0x8D, 0x8F, 0x90, 0x9D) round-trip as the byte itself.
var win1252High = [32]rune{
	0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, // 80-87
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F, // 88-8F
	0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, // 90-97
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178, // 98-9F
}

// guessPreWord97Charset mirrors HWPFOldDocument.guessCodePage() narrowly for
// the charsets we currently see in repo-local fixtures: pick the first old
// font-table charset that is not ANSI/default/symbol and map it to a Windows
// code page when the mapping is known locally.
func guessPreWord97Charset(wordStream []byte, span PreWord97BodySpan) *charmap.Charmap {
	fonts := LoadPreWord97FontTable(wordStream, span.FontTableOffset, span.FontTableSize)
	for _, font := range fonts {
		switch font.Charset {
		case 0x00, 0x01, 0x02:
			continue
		}
		if enc := preWord97FontCharsetCharmap(font.Charset); enc != nil {
			return enc
		}
	}
	return nil
}

func preWord97FontCharsetCharmap(chs byte) *charmap.Charmap {
	switch chs {
	case 0xA1:
		return charmap.Windows1253
	case 0xB1:
		return charmap.Windows1255
	case 0xB2:
		return charmap.Windows1256
	case 0xBA:
		return charmap.Windows1257
	case 0xC1:
		return charmap.Windows1254
	case 0xCC:
		return charmap.Windows1251
	case 0xEE:
		return charmap.Windows1250
	default:
		return nil
	}
}

func decodePreWord97SingleByteTextWithCharset(data []byte, preferred *charmap.Charmap) string {
	if preferred != nil {
		decoded := decodeSingleByteCharmap(data, preferred)
		if legacyDecodedTextQuality(decoded) > 0 {
			return decoded
		}
	}
	return decodePreWord97SingleByteText(data)
}