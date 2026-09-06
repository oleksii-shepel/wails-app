package legacy

import docformat "office/pkg/word/parsers/doc/format"

func fromSequentialPAPXMetadata(meta docformat.PAPXMetadata) papxMetadata {
	return papxMetadata{
		TableDepth:         meta.TableDepth,
		TableRowProps:      legacyTableRowPropsFromCanonical(meta.TableRowProps),
		PreWord97ListStart: meta.PreWord97ListStart,
		PropertyRevision:   fromSequentialRevision(meta.PropertyRevision),
		Diagnostics:        meta.Diagnostics,
	}
}

func fromSequentialPAPX(entry docformat.PAPXEntry) papxEntry {
	return papxEntry{FC: entry.FC, Format: entry.Format, Meta: fromSequentialPAPXMetadata(entry.Meta), LegacyInTableHint: entry.LegacyInTableHint, LegacyRowMarkHint: entry.LegacyRowMarkHint, Sprms: append([]byte(nil), entry.Sprms...)}
}

func appendSequentialPAPX(dst []papxEntry, src []docformat.PAPXEntry) []papxEntry {
	for _, entry := range src { dst = append(dst, fromSequentialPAPX(entry)) }
	return dst
}
