package legacy

import (
	wordir "office/pkg/word/ir"
)

// ProjectDocument recomputes the resolved semantic fields of a Document from
// its lossless Source fields. This is the projection layer for the legacy .doc
// IR: the parser builds source fields (raw SPRMs, style indices, etc.) and this
// function projects them into the effective formatting that renderers consume.
//
// It re-parses PAPX, CHPX, and SEPX grpprls, re-applies style inheritance, and
// refreshes paragraph/run/section semantic fields. Structural aspects such as
// tables, lists, fields, and bookmarks are NOT rebuilt; only formatting is
// reprojected.
func ProjectDocument(doc *wordir.Document) *wordir.Document {
	if doc == nil {
		return nil
	}

	// Build a style lookup by index for paragraph styles.
	styleByIndex := make(map[int]wordir.Style)
	for _, s := range doc.Styles {
		if s.Source != nil {
			// We don't have the original istd in the resolved Style,
			// but we can try to recover it from RawFormat.BinaryStyleIndex
			// or from the style ID if it follows the legacy-istd-N pattern.
			if istd := legacyParseStyleIstd(s.ID); istd >= 0 {
				styleByIndex[istd] = s
			}
		}
	}

	// Reproject paragraph and run formatting.
	for i := range doc.Paragraphs {
		p := &doc.Paragraphs[i]
		projectParagraph(p, doc, styleByIndex)
	}

	// Reproject section formatting.
	for i := range doc.Sections {
		s := &doc.Sections[i]
		projectSection(s)
	}

	return doc
}

// projectParagraph recomputes paragraph and run formatting from source fields.
func projectParagraph(p *wordir.Paragraph, doc *wordir.Document, styleByIndex map[int]wordir.Style) {
	if p.Source == nil {
		return
	}

	// Parse the raw PAPX grpprl to get direct paragraph formatting.
	var directPara wordir.ParagraphFormat
	if len(p.Source.PapxSprms) > 0 {
		directPara = parsePapxGrpprl(p.Source.PapxSprms)
	}

	// Resolve style inheritance.
	var basePara wordir.ParagraphFormat
	var baseRun wordir.RunFormat
	if p.Source.StyleIndex != nil {
		if style, ok := styleByIndex[*p.Source.StyleIndex]; ok {
			p.StyleID = style.ID
			p.StyleName = style.Name
			basePara = style.Format
			baseRun = style.RunFormat
		} else {
			p.StyleID = legacyBinaryParagraphStyleID(*p.Source.StyleIndex)
		}
	} else if doc != nil {
		basePara = doc.DefaultParagraphFormat
		baseRun = doc.DefaultRunFormat
	}

	p.Format = legacyMergeParagraphFormat(basePara, directPara)

	// Reproject run formatting within the paragraph.
	for j := range p.Runs {
		r := &p.Runs[j]
		projectRun(r, baseRun)
	}
}

// projectRun recomputes run formatting from source fields.
func projectRun(r *wordir.Run, baseRun wordir.RunFormat) {
	if r.Source == nil {
		r.Format = legacyMergeRunFormat(baseRun, r.Format)
		return
	}

	var directRun wordir.RunFormat
	if len(r.Source.ChpxSprms) > 0 {
		directRun = parseChpxGrpprl(r.Source.ChpxSprms)
	}
	r.Format = legacyMergeRunFormat(baseRun, directRun)

	// Preserve revision metadata from source.
	r.Revision = r.Source.Revision
	if r.Source.Spec {
		// Spec runs may need special handling; for now just preserve the flag.
	}
}

// projectSection recomputes section formatting from source fields.
func projectSection(s *wordir.Section) {
	if s.Source == nil || len(s.Source.SepxSprms) == 0 {
		return
	}
	*s = parseSepxGrpprl(s.Source.SepxSprms)
}

// legacyParseStyleIstd extracts the numeric istd from a legacy style ID such
// as "legacy-istd-42". Returns -1 if the ID does not match the pattern.
func legacyParseStyleIstd(id string) int {
	const prefix = "legacy-istd-"
	if len(id) <= len(prefix) || id[:len(prefix)] != prefix {
		return -1
	}
	var istd int
	for i := len(prefix); i < len(id); i++ {
		c := id[i]
		if c < '0' || c > '9' {
			return -1
		}
		istd = istd*10 + int(c-'0')
	}
	return istd
}