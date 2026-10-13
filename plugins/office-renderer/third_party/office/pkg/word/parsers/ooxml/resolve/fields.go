package resolve

import (
	"strings"

	"office/pkg/word/parsers/ooxml/parts"
)

func normalizeAutoTextName(name string) string {
	return strings.ToUpper(strings.TrimSpace(name))
}

// autoTextEntry holds resolved content for an AUTOTEXT building block.
type autoTextEntry struct {
	Name  string // authored glossary entry name (preserved for field metadata)
	Story Story
	Text  string // flattened text fallback
}

func buildAutoTextEntryIndex(ctx *resolveCtx) map[string]autoTextEntry {
	if ctx == nil || ctx.pkg == nil || ctx.pkg.Glossary == nil {
		return nil
	}
	index := make(map[string]autoTextEntry)
	temp := *ctx
	temp.currentPart = ctx.pkg.GlossaryPartName
	temp.autoTextEntries = nil
	temp.nextParaIndex = 0
	for _, entry := range ctx.pkg.Glossary.DocParts {
		if entry == nil || entry.Name == nil || entry.Body == nil {
			continue
		}
		name := normalizeAutoTextName(entry.Name.Val)
		if name == "" {
			continue
		}
		// Gallery filtering: only include entries from the AutoText gallery,
		// or entries with no gallery information (backward compatibility).
		if !isAutoTextGallery(entry) {
			continue
		}
		story := normalizeStory(&temp, entry.Body.Blocks, entry.Body.Paragraphs, entry.Body.Tables)
		if strings.TrimSpace(story.Text) == "" {
			continue
		}
		index[name] = autoTextEntry{Name: entry.Name.Val, Story: story, Text: story.Text}
	}
	if len(index) == 0 {
		return nil
	}
	return index
}

func standaloneAutoTextName(paragraph *parts.Paragraph, resolved Paragraph) string {
	if paragraph == nil || strings.TrimSpace(paragraphText(paragraph)) != "" {
		// A cached/display result is authored in the document. Preserve it rather
		// than replacing the host paragraph with the current glossary definition.
		return ""
	}
	name := ""
	hasFieldRun := false
	for _, run := range resolved.Runs {
		if run.Kind == "break" && run.Text == "\n" {
			continue
		}
		if strings.TrimSpace(run.Text) == "" && run.FieldKind == "" {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(run.FieldKind), "AUTOTEXT") {
			return ""
		}
		current := normalizeAutoTextName(run.FieldArg)
		if current == "" {
			return ""
		}
		if name == "" {
			name = current
		} else if current != name {
			return ""
		}
		hasFieldRun = true
	}
	if !hasFieldRun {
		return ""
	}
	return name
}

// cloneAutoTextBlocks creates independent block instances for one insertion and
// assigns fresh document paragraph/table indices. Glossary stories are reusable
// templates; their original indices must not leak into the consuming document.
func cloneAutoTextBlocks(ctx *resolveCtx, story Story, fieldName string) []Block {
	if ctx == nil || len(story.Blocks) == 0 {
		return nil
	}
	out := make([]Block, 0, len(story.Blocks))
	for _, source := range story.Blocks {
		block := cloneAutoTextBlock(ctx, source, fieldName)
		block.Index = len(out)
		out = append(out, block)
	}
	return out
}

func cloneAutoTextBlock(ctx *resolveCtx, source Block, fieldName string) Block {
	block := source
	block.Paragraph = nil
	block.Table = nil
	block.Section = nil

	switch source.Kind {
	case "paragraph":
		if source.Paragraph != nil {
			p := *source.Paragraph
			p.Runs = cloneAutoTextRuns(source.Paragraph.Runs, fieldName)
			p.Index = ctx.nextParaIndex
			ctx.nextParaIndex++
			block.Paragraph = &p
		}
	case "table":
		if source.Table != nil {
			tbl := cloneAutoTextTable(ctx, *source.Table, fieldName)
			block.Table = &tbl
		}
	case "section":
		if source.Section != nil {
			section := *source.Section
			block.Section = &section
		}
	}
	return block
}

func cloneAutoTextRuns(source []Run, fieldName string) []Run {
	if len(source) == 0 {
		return nil
	}
	out := make([]Run, len(source))
	copy(out, source)
	for i := range out {
		if out[i].FieldKind == "" {
			out[i].FieldKind = "AUTOTEXT"
			out[i].FieldArg = fieldName
		}
	}
	return out
}

func cloneAutoTextTable(ctx *resolveCtx, source Table, fieldName string) Table {
	out := source
	out.Index = 0
	out.Rows = make([]TableRow, len(source.Rows))
	for rowIndex := range source.Rows {
		out.Rows[rowIndex] = source.Rows[rowIndex]
		out.Rows[rowIndex].Cells = make([]TableCell, len(source.Rows[rowIndex].Cells))
		for cellIndex := range source.Rows[rowIndex].Cells {
			cell := source.Rows[rowIndex].Cells[cellIndex]
			cloned := cell
			if len(cell.Blocks) > 0 {
				cloned.Blocks = make([]Block, len(cell.Blocks))
				cloned.Paragraphs = nil
				cloned.Tables = nil
				for blockIndex := range cell.Blocks {
					cloned.Blocks[blockIndex] = cloneAutoTextBlock(ctx, cell.Blocks[blockIndex], fieldName)
					cloned.Blocks[blockIndex].Index = blockIndex
					switch cloned.Blocks[blockIndex].Kind {
					case "paragraph":
						if cloned.Blocks[blockIndex].Paragraph != nil {
							cloned.Paragraphs = append(cloned.Paragraphs, *cloned.Blocks[blockIndex].Paragraph)
						}
					case "table":
						if cloned.Blocks[blockIndex].Table != nil {
							cloned.Tables = append(cloned.Tables, *cloned.Blocks[blockIndex].Table)
						}
					}
				}
			} else {
				cloned.Paragraphs = make([]Paragraph, len(cell.Paragraphs))
				for paragraphIndex := range cell.Paragraphs {
					cloned.Paragraphs[paragraphIndex] = cell.Paragraphs[paragraphIndex]
					cloned.Paragraphs[paragraphIndex].Runs = cloneAutoTextRuns(cell.Paragraphs[paragraphIndex].Runs, fieldName)
					cloned.Paragraphs[paragraphIndex].Index = ctx.nextParaIndex
					ctx.nextParaIndex++
				}
				cloned.Tables = make([]Table, len(cell.Tables))
				for tableIndex := range cell.Tables {
					cloned.Tables[tableIndex] = cloneAutoTextTable(ctx, cell.Tables[tableIndex], fieldName)
				}
			}
			out.Rows[rowIndex].Cells[cellIndex] = cloned
		}
	}
	return out
}

func flattenStoryToInlineRuns(story Story) []Run {
	var runs []Run
	appendBreak := func() {
		if len(runs) == 0 || runs[len(runs)-1].Kind == "break" {
			return
		}
		runs = append(runs, Run{Kind: "break", Text: "\n"})
	}
	for _, block := range story.Blocks {
		switch block.Kind {
		case "paragraph":
			if block.Paragraph == nil {
				continue
			}
			if len(runs) > 0 {
				appendBreak()
			}
			if len(block.Paragraph.Runs) > 0 {
				runs = append(runs, block.Paragraph.Runs...)
				continue
			}
			runs = appendPlainTextRuns(runs, block.Paragraph.Text)
		case "table":
			if block.Table == nil {
				continue
			}
			if len(runs) > 0 {
				appendBreak()
			}
			runs = appendPlainTextRuns(runs, block.Table.Text)
		}
	}
	return runs
}

func appendPlainTextRuns(dst []Run, text string) []Run {
	if text == "" {
		return dst
	}
	start := 0
	for i, r := range text {
		switch r {
		case '\n':
			if start < i {
				dst = append(dst, Run{Kind: "run", Text: text[start:i]})
			}
			dst = append(dst, Run{Kind: "break", Text: "\n"})
			start = i + 1
		case '\t':
			if start < i {
				dst = append(dst, Run{Kind: "run", Text: text[start:i]})
			}
			dst = append(dst, Run{Kind: "tab", Text: "\t"})
			start = i + 1
		}
	}
	if start < len(text) {
		dst = append(dst, Run{Kind: "run", Text: text[start:]})
	}
	return dst
}

// isAutoTextGallery reports whether a glossary entry belongs to the AutoText
// gallery (or has no gallery information, in which case it is included for
// backward compatibility with older fixtures).
func isAutoTextGallery(entry *parts.GlossaryEntry) bool {
	if entry.DocPartGallery != nil {
		return normalizeAutoTextName(entry.DocPartGallery.Val) == "AUTOTEXT"
	}
	if entry.Gallery != nil {
		return normalizeAutoTextName(entry.Gallery.Val) == "AUTOTEXT"
	}
	return true
}

// runFieldContent inspects a run's Contents for field-related items and returns the field type:
//   - "begin", "separate", "end" — fldChar types
//   - "instr" — instrText (also returns the collected text)
//   - "" — none (normal run)
func runFieldContent(run *parts.Run) (fldType, instrText string) {
	if run == nil {
		return "", ""
	}
	for _, c := range run.Contents {
		switch c.Kind {
		case parts.RunContentFldChar:
			if c.FldChar != nil {
				return c.FldChar.Type, ""
			}
		case parts.RunContentInstrText:
			instrText += c.InstrText
			fldType = "instr"
		}
	}
	return fldType, instrText
}

// extractFormFieldData returns FormFieldData from a fldChar-begin run, if any.
func extractFormFieldData(run *parts.Run) *parts.FormFieldData {
	if run == nil {
		return nil
	}
	for _, c := range run.Contents {
		if c.Kind == parts.RunContentFldChar && c.FldChar != nil && c.FldChar.FormField != nil {
			return c.FldChar.FormField
		}
	}
	return nil
}

// emitField resolves a field instruction and appends runs to out.
// For HYPERLINK and REF instructions it emits hyperlink-styled runs; otherwise it
// emits the display runs as plain text. ECMA-376 Part 1 §17.16.5.
func emitField(ctx *resolveCtx, out *Paragraph, instr string, displayRuns []*parts.Run, paraRunFmt RunFormat, formField *parts.FormFieldData) {
	// Convert raw parts.Runs to resolved Runs up-front.
	runs := make([]Run, 0, len(displayRuns))
	for _, run := range displayRuns {
		runs = append(runs, normalizeRun(ctx, run, "", "", paraRunFmt)...)
	}

	instr = strings.TrimSpace(instr)
	upper := strings.ToUpper(instr)

	if strings.HasPrefix(upper, "HYPERLINK ") {
		url, anchor := parseFieldHyperlink(instr)
		target := url
		if anchor != "" {
			if target != "" {
				target += "#" + anchor
			} else {
				target = "#" + anchor
			}
		}
		if target != "" {
			for i := range runs {
				runs[i].FieldKind = "HYPERLINK"
				runs[i].FieldInstr = instr
				runs[i].FieldArg = url
				runs[i].Target = target
				runs[i].Anchor = anchor
				runs[i].Kind = "hyperlink"
			}
			appendSemanticRuns(out, runs)
			return
		}
	} else if strings.HasPrefix(upper, "REF ") {
		anchor := parseFieldRef(instr)
		if anchor != "" {
			for i := range runs {
				runs[i].FieldKind = "REF"
				runs[i].FieldInstr = instr
				runs[i].FieldArg = anchor
				runs[i].Target = "#" + anchor
				runs[i].Anchor = anchor
			}
			appendSemanticRuns(out, runs)
			return
		}
	} else if strings.HasPrefix(upper, "PAGEREF ") {
		anchor := parseFieldTarget(strings.TrimSpace(instr[len("PAGEREF"):]))
		if anchor != "" {
			for i := range runs {
				runs[i].FieldKind = "PAGEREF"
				runs[i].FieldInstr = instr
				runs[i].FieldArg = anchor
				runs[i].Target = "#" + anchor
				runs[i].Anchor = anchor
			}
			appendSemanticRuns(out, runs)
			return
		}
	} else {
		// Detect semantic field types that map to a CSS class.
		fieldKind := fieldInstrKind(upper)
		if fieldKind != "" {
			fieldArg := ""
			if fieldKind == "DOCPROPERTY" || fieldKind == "DOCVARIABLE" {
				fieldArg = parseDocPropertyName(instr)
			} else if fieldKind == "STYLEREF" {
				fieldArg = parseFieldStyleRef(instr)
			} else if fieldKind == "IF" || fieldKind == "COMPARE" || fieldKind == "QUOTE" || fieldKind == "EMBED" || fieldKind == "LINK" {
				fieldArg = strings.TrimSpace(instr)
			} else if fieldKind == "FILLIN" {
				fieldArg = parseFieldFillinDefault(instr)
			} else if fieldKind == "SEQ" || fieldKind == "ASK" || fieldKind == "MERGEFIELD" || fieldKind == "AUTOTEXT" || fieldKind == "SET" || fieldKind == "SYMBOL" || fieldKind == "EQ" || fieldKind == "LISTNUM" || fieldKind == "INCLUDETEXT" || fieldKind == "INCLUDEPICTURE" || fieldKind == "TA" || fieldKind == "TOA" || fieldKind == "BIDIOUTLINE" || fieldKind == "GOTOBUTTON" || fieldKind == "BARCODE" {
				fieldArg = parseFieldLeadingArg(instr, fieldKind)
			} else if fieldKind == "TOC" || fieldKind == "INDEX" || fieldKind == "XE" || fieldKind == "TC" {
				fieldArg = strings.TrimSpace(instr)
				base := Run{Kind: "run", Format: paraRunFmt, FieldKind: fieldKind, FieldArg: fieldArg, FieldInstr: instr}
				if len(runs) > 0 {
					base.Format = runs[0].Format
					base.Text = aggregateFieldDisplayText(runs)
				}
				appendSemanticRuns(out, []Run{base})
				return
			}
			if fieldKind == "AUTOTEXT" && len(runs) == 0 && ctx != nil {
				entry := ctx.autoTextEntries[normalizeAutoTextName(fieldArg)]
				expanded := flattenStoryToInlineRuns(entry.Story)
				if len(expanded) > 0 {
					expanded = append([]Run(nil), expanded...)
					for i := range expanded {
						if expanded[i].FieldKind == "" {
							expanded[i].FieldKind = fieldKind
							expanded[i].FieldArg = fieldArg
							expanded[i].FieldInstr = instr
						}
					}
					appendSemanticRuns(out, expanded)
					return
				}
				// Plain-text fallback.
				if strings.TrimSpace(entry.Text) != "" {
					runs = []Run{{
						Kind:       "run",
						Format:     paraRunFmt,
						Text:       entry.Text,
						FieldKind:  fieldKind,
						FieldArg:   fieldArg,
						FieldInstr: instr,
					}}
					appendSemanticRuns(out, runs)
					return
				}
			}
			if len(runs) == 0 {
				appendSemanticRuns(out, []Run{{
					Kind:       "run",
					Format:     paraRunFmt,
					FieldKind:  fieldKind,
					FieldArg:   fieldArg,
					FieldInstr: instr,
				}})
				return
			}
			for i := range runs {
				runs[i].FieldKind = fieldKind
				runs[i].FieldArg = fieldArg
				runs[i].FieldInstr = instr
			}
			appendSemanticRuns(out, runs)
			return
		}
	}

	// Default: emit display runs as plain text, or form field element if this is a
	// FORMTEXT / FORMCHECKBOX / FORMDROPDOWN field.
	keyword := strings.Fields(upper)
	if len(keyword) > 0 {
		switch keyword[0] {
		case "FORMTEXT", "FORMCHECKBOX", "FORMDROPDOWN":
			// Emit a form field run instead of plain text.
			ff := resolveFormField(keyword[0], formField, runs, paraRunFmt)
			out.Runs = append(out.Runs, ff)
			return
		}
	}
	appendSemanticRuns(out, runs)
}

// resolveFormField builds a Run{Kind:"formField"} for a legacy FORMTEXT/FORMCHECKBOX/FORMDROPDOWN field.
func resolveFormField(keyword string, ffd *parts.FormFieldData, displayRuns []Run, paraRunFmt RunFormat) Run {
	r := Run{Kind: "formField", Format: paraRunFmt}
	ff := &FormField{}

	// Use w:ffData when available; fall back to inferring kind from keyword.
	if ffd != nil {
		ff.Name = ffd.Name
		ff.Enabled = ffd.Enabled
		ff.CalcOnExit = ffd.CalcOnExit
		switch ffd.Kind {
		case "text":
			ff.Type = "text"
			ff.DefaultText = ffd.DefaultTxt
			ff.MaxLength = ffd.MaxLength
			ff.TextType = ffd.TextType
			ff.TextFormat = ffd.TextFormat
		case "checkbox":
			ff.Type = "checkbox"
			ff.Checked = ffd.Checked
			ff.CheckboxSize = ffd.CheckboxSize
			ff.CheckboxSizeAuto = ffd.CheckboxSizeAuto
		case "dropdown":
			ff.Type = "dropdown"
			ff.Items = append([]string(nil), ffd.DropItems...)
			ff.DefaultIdx = ffd.DropDefIdx
		}
	}
	// Fill kind from keyword when w:ffData was absent or did not set it.
	if ff.Type == "" {
		switch keyword {
		case "FORMTEXT":
			ff.Type = "text"
		case "FORMCHECKBOX":
			ff.Type = "checkbox"
		case "FORMDROPDOWN":
			ff.Type = "dropdown"
		}
	}
	// For text fields without a default value, aggregate the display run text
	// (between separate/end) as the current value.
	if ff.Type == "text" && ff.DefaultText == "" && len(displayRuns) > 0 {
		var sb strings.Builder
		for _, dr := range displayRuns {
			sb.WriteString(dr.Text)
		}
		ff.DefaultText = sb.String()
	}
	r.FormField = ff
	return r
}

func aggregateFieldDisplayText(runs []Run) string {
	if len(runs) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, run := range runs {
		sb.WriteString(run.Text)
	}
	return sb.String()
}

// fieldInstrKind maps a field instruction (already uppercased) to a semantic
// field kind string. Returns "" for unrecognized or complex field types.
func fieldInstrKind(upper string) string {
	// Strip any switches and extra whitespace; just look at the keyword.
	upper = strings.TrimSpace(upper)
	if strings.HasPrefix(upper, "=") {
		return "FORMULA"
	}
	fields := strings.Fields(upper)
	if len(fields) == 0 {
		return ""
	}
	keyword := fields[0]
	switch keyword {
	case "PAGE":
		return "PAGE"
	case "NUMPAGES":
		return "NUMPAGES"
	case "DATE":
		return "DATE"
	case "TIME":
		return "TIME"
	case "CREATEDATE":
		return "CREATEDATE"
	case "SAVEDATE":
		return "SAVEDATE"
	case "PRINTDATE":
		return "PRINTDATE"
	case "TITLE":
		return "TITLE"
	case "AUTHOR":
		return "AUTHOR"
	case "SUBJECT":
		return "SUBJECT"
	case "FILENAME":
		return "FILENAME"
	case "HYPERLINK":
		return "HYPERLINK"
	case "NUMCHARS":
		return "NUMCHARS"
	case "NUMWORDS":
		return "NUMWORDS"
	case "PAGEREF":
		return "PAGEREF"
	case "REF":
		return "REF"
	case "SEQ":
		return "SEQ"
	case "ASK":
		return "ASK"
	case "QUOTE":
		return "QUOTE"
	case "MERGEFIELD":
		return "MERGEFIELD"
	case "AUTOTEXT":
		return "AUTOTEXT"
	case "AUTONUM":
		return "AUTONUM"
	case "AUTONUMLGL":
		return "AUTONUMLGL"
	case "AUTONUMOUT":
		return "AUTONUMOUT"
	case "EMBED":
		return "EMBED"
	case "LINK":
		return "LINK"
	case "IF":
		return "IF"
	case "COMPARE":
		return "COMPARE"
	case "TOC":
		return "TOC"
	case "INDEX":
		return "INDEX"
	case "XE":
		return "XE"
	case "TC":
		return "TC"
	case "STYLEREF":
		return "STYLEREF"
	case "DOCPROPERTY":
		return "DOCPROPERTY"
	case "DOCVARIABLE":
		return "DOCVARIABLE"
	case "FILLIN":
		return "FILLIN"
	case "NOTEREF":
		return "NOTEREF"
	case "SECTION":
		return "SECTION"
	case "SECTIONPAGES":
		return "SECTIONPAGES"
	case "EDITTIME":
		return "EDITTIME"
	case "USERADDRESS":
		return "USERADDRESS"
	case "USERINITIALS":
		return "USERINITIALS"
	case "USERNAME":
		return "USERNAME"
	case "MACROBUTTON":
		return "MACROBUTTON"
	case "PRIVATE":
		return "PRIVATE"
	case "ADVANCE":
		return "ADVANCE"
	case "SET":
		return "SET"
	case "MERGEREC":
		return "MERGEREC"
	case "MERGESEQ":
		return "MERGESEQ"
	case "NEXT":
		return "NEXT"
	case "NEXTIF":
		return "NEXTIF"
	case "SKIPIF":
		return "SKIPIF"
	case "DATABASE":
		return "DATABASE"
	case "LISTNUM":
		return "LISTNUM"
	case "INCLUDETEXT":
		return "INCLUDETEXT"
	case "INCLUDEPICTURE":
		return "INCLUDEPICTURE"
	case "EQ":
		return "EQ"
	case "SYMBOL":
		return "SYMBOL"
	case "TA":
		return "TA"
	case "TOA":
		return "TOA"
	case "BIDIOUTLINE":
		return "BIDIOUTLINE"
	case "CITATION":
		return "CITATION"
	case "BIBLIOGRAPHY":
		return "BIBLIOGRAPHY"
	case "REVNUM":
		return "REVNUM"
	case "TEMPLATE":
		return "TEMPLATE"
	case "GOTOBUTTON":
		return "GOTOBUTTON"
	case "GREETINGLINE":
		return "GREETINGLINE"
	case "ADDRESSBLOCK":
		return "ADDRESSBLOCK"
	case "BARCODE":
		return "BARCODE"
	case "RD":
		return "RD"
	case "IMPORT":
		return "IMPORT"
	case "SAVEPICTURE":
		return "SAVEPICTURE"
	case "INFO":
		return "INFO"
	}
	return ""
}

// parseFieldHyperlink extracts the URL and optional \l anchor from a HYPERLINK instruction.
// The instruction format is: HYPERLINK "url" [\l "anchor"]
func parseFieldHyperlink(instr string) (url, anchor string) {
	// Strip the keyword.
	rest := strings.TrimSpace(instr[len("HYPERLINK"):])
	url, rest = extractFieldString(rest)
	// Look for \l switch.
	for strings.HasPrefix(rest, `\`) {
		rest = strings.TrimSpace(rest[1:])
		if strings.HasPrefix(strings.ToUpper(rest), "L ") {
			rest = strings.TrimSpace(rest[2:])
			anchor, _ = extractFieldString(rest)
			break
		}
		// skip other switches
		_, rest = extractFieldString(rest)
	}
	return url, anchor
}

// parseFieldRef extracts the bookmark name from a REF instruction.
// The instruction format is: REF bookmarkName [\switches]
func parseFieldRef(instr string) string {
	rest := strings.TrimSpace(instr[len("REF"):])
	return parseFieldTarget(rest)
}

func parseFieldTarget(rest string) string {
	// The bookmark name is the first token (no quotes).
	fields := strings.Fields(rest)
	if len(fields) > 0 && !strings.HasPrefix(fields[0], `\`) {
		return fields[0]
	}
	return ""
}

func parseDocPropertyName(instr string) string {
	rest := strings.TrimSpace(instr[len("DOCPROPERTY"):])
	if rest == "" {
		return ""
	}
	if strings.HasPrefix(rest, `"`) {
		name, _ := extractFieldString(rest)
		return strings.TrimSpace(name)
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	if strings.HasPrefix(fields[0], `\`) {
		return ""
	}
	return fields[0]
}

// extractFieldString reads a quoted string "..." from s.
// Returns the unquoted value and the remaining string after the closing quote.
func extractFieldString(s string) (value, rest string) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, `"`) {
		return "", s
	}
	s = s[1:] // skip opening quote
	i := strings.Index(s, `"`)
	if i < 0 {
		return s, "" // unterminated — treat rest as value
	}
	return s[:i], strings.TrimSpace(s[i+1:])
}

// parseFieldStyleRef extracts the style name from a STYLEREF instruction.
// Format: STYLEREF "StyleName" [\switches] or STYLEREF StyleName [\switches]
func parseFieldStyleRef(instr string) string {
	rest := strings.TrimSpace(instr[len("STYLEREF"):])
	if strings.HasPrefix(rest, `"`) {
		name, _ := extractFieldString(rest)
		return strings.TrimSpace(name)
	}
	fields := strings.Fields(rest)
	if len(fields) > 0 && !strings.HasPrefix(fields[0], `\`) {
		return fields[0]
	}
	return ""
}

func parseFieldLeadingArg(instr, keyword string) string {
	rest := strings.TrimSpace(instr[len(keyword):])
	if rest == "" {
		return ""
	}
	if strings.HasPrefix(rest, `"`) {
		value, _ := extractFieldString(rest)
		return strings.TrimSpace(value)
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 || strings.HasPrefix(fields[0], `\`) {
		return ""
	}
	return fields[0]
}

func parseFieldFillinDefault(instr string) string {
	rest := strings.TrimSpace(instr[len("FILLIN"):])
	if rest == "" {
		return ""
	}
	if strings.HasPrefix(rest, `"`) {
		_, rest = extractFieldString(rest)
	} else {
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return ""
		}
		rest = strings.TrimSpace(rest[len(fields[0]):])
	}
	if rest == "" {
		return ""
	}
	if strings.HasPrefix(rest, `"`) {
		value, _ := extractFieldString(rest)
		return value
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 || strings.HasPrefix(fields[0], `\`) {
		return ""
	}
	return fields[0]
}
