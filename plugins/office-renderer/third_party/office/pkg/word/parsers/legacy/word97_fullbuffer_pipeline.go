package legacy

// materializeLegacyWord97ParseData is the canonicalized full-buffer adapter
// used by the remaining legacy ParseDoc path. Binary and revision-attribution
// ownership stay in doc/word97; this function only adapts canonical runs/context
// into the old legacyDocParseData shape consumed by the document builder.
func materializeLegacyWord97ParseData(wordStream []byte, core *legacyDocStreamCore) *legacyDocParseData {
	if core == nil {
		return nil
	}

	runs := materializeCanonicalLosslessRuns(
		wordStream,
		core.ClxData,
		0,
		core.CcpText,
		core.ChpxEntries,
		core.PapxEntries,
		core.RevisionAuthors,
	)
	if runs == nil {
		return nil
	}

	runs, enrichment := applyCanonicalWord97Enrichment(runs, core)
	runs = applyDocFieldsToRunsWithDataStream(runs, core.DataStream)

	out := runs[:0]
	for _, run := range runs {
		if run.Format.Vanish != nil && *run.Format.Vanish {
			continue
		}
		out = append(out, run)
	}
	if scoreLegacyText(joinDocRunsText(out)) == 0 {
		return nil
	}

	return &legacyDocParseData{
		Runs:           out,
		Footnotes:      enrichment.Footnotes,
		Endnotes:       enrichment.Endnotes,
		Comments:       enrichment.Comments,
		BookmarkStarts: enrichment.BookmarkStarts,
		BookmarkEnds:   legacyBookmarkEndCPByName(enrichment.BookmarkEnds),
		ListTables:     core.ListTables,
		StyleSheet:     core.StyleSheet,
		TextBoxes:      enrichment.TextBoxes,
		DocSettings:    core.DocSettings,
		FontFamilies:   core.FontFamilies,
		EmbeddedFonts:  core.EmbeddedFonts,
		RawDOP:         core.RawDOP,
		Version:        core.Version,
		Compatibility:  core.Compatibility,
		Diagnostics:    core.Diagnostics,
	}
}

func legacyBookmarkEndCPByName(ends map[int][]string) map[string]int {
	if len(ends) == 0 {
		return nil
	}
	out := make(map[string]int)
	for cp, names := range ends {
		for _, name := range names {
			if name == "" {
				continue
			}
			out[name] = cp
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
