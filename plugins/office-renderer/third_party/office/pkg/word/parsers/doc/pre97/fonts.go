package pre97

import "golang.org/x/text/encoding/charmap"

// FontEntry is one old Word 6/95 FFN entry.
type FontEntry struct {
	Charset byte
	Name    string
	AltName string
}

// LoadFontTable reads the Word 6/95 font table embedded in the WordDocument
// stream. Names are decoded as Windows-1252; Charset retains the original chs
// byte so callers can choose the document text code page independently.
func LoadFontTable(word []byte, offset, size int) []FontEntry {
	if offset <= 0 || size < 6 || offset+size > len(word) {
		return nil
	}
	buf := word[offset : offset+size]
	pos := 2 // redundant table-length header
	var out []FontEntry
	for pos < len(buf) {
		if pos+6 > len(buf) {
			break
		}
		recLen := int(buf[pos])
		end := pos + 1 + recLen
		if recLen == 0 || end > len(buf) {
			break
		}
		entry := FontEntry{Charset: buf[pos+4]}
		nameStart := pos + 6
		if nameStart < end {
			nameEnd := nameStart
			for nameEnd < end && buf[nameEnd] != 0 {
				nameEnd++
			}
			entry.Name = decodeWin1252(buf[nameStart:nameEnd])
			altStart := nameEnd + 1
			if altStart < end {
				altEnd := altStart
				for altEnd < end && buf[altEnd] != 0 {
					altEnd++
				}
				if altEnd > altStart {
					entry.AltName = decodeWin1252(buf[altStart:altEnd])
				}
			}
		}
		out = append(out, entry)
		pos = end
	}
	return out
}

// FontNames projects canonical FFN entries into the index-ordered font names
// consumed by old CHPX semantics.
func FontNames(fonts []FontEntry) []string {
	if len(fonts) == 0 {
		return nil
	}
	out := make([]string, len(fonts))
	for i := range fonts {
		out[i] = fonts[i].Name
	}
	return out
}

// GuessCharset mirrors the Word 6/95 code-page selection used by the old
// parser: choose the first meaningful FFN charset that maps to a supported
// Windows single-byte code page.
func GuessCharset(fonts []FontEntry) *charmap.Charmap {
	for _, font := range fonts {
		switch font.Charset {
		case 0x00, 0x01, 0x02:
			continue
		}
		if enc := fontCharsetCharmap(font.Charset); enc != nil {
			return enc
		}
	}
	return nil
}

func fontCharsetCharmap(chs byte) *charmap.Charmap {
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

func decodeWin1252(b []byte) string {
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

var win1252High = [32]rune{
	0x20AC, 0x0081, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x008D, 0x017D, 0x008F,
	0x0090, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x009D, 0x017E, 0x0178,
}
