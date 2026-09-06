package legacy

import docformat "office/pkg/word/parsers/doc/format"

func toSequentialCHPXEntries(src []chpxEntry) []docformat.CHPXEntry {
	if len(src) == 0 {
		return nil
	}
	out := make([]docformat.CHPXEntry, 0, len(src))
	for _, e := range src {
		out = append(out, docformat.CHPXEntry{
			FC: e.FC,
			Format: e.Format,
			Revision: e.Revision,
			Spec: e.Spec,
			Data: e.Data,
			Obj: e.Obj,
			IbstRMark: e.IbstRMark,
			HasIbstRMark: e.HasIbstRMark,
			DttmRMark: e.DttmRMark,
			HasDttmRMark: e.HasDttmRMark,
			PropertyRevision: toSequentialRevision(e.PropertyRevision),
			DisplayFieldRevision: toSequentialRevision(e.DisplayFieldRevision),
			Diagnostics: e.Diagnostics,
			Sprms: append([]byte(nil), e.Sprms...),
		})
	}
	return out
}

func toSequentialPAPXEntries(src []papxEntry) []docformat.PAPXEntry {
	if len(src) == 0 {
		return nil
	}
	out := make([]docformat.PAPXEntry, 0, len(src))
	for _, e := range src {
		out = append(out, docformat.PAPXEntry{
			FC: e.FC,
			Format: e.Format,
			Meta: docformat.PAPXMetadata{
				TableDepth: e.Meta.TableDepth,
				TableRowProps: e.Meta.TableRowProps.canonical(),
				PreWord97ListStart: e.Meta.PreWord97ListStart,
				PropertyRevision: toSequentialRevision(e.Meta.PropertyRevision),
				Diagnostics: e.Meta.Diagnostics,
			},
			LegacyInTableHint: e.LegacyInTableHint,
			LegacyRowMarkHint: e.LegacyRowMarkHint,
			Sprms: append([]byte(nil), e.Sprms...),
		})
	}
	return out
}
