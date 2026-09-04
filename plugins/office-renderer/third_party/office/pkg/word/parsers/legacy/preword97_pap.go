package legacy

import (
	"encoding/binary"
	"sort"
	"strings"

	wordir "office/pkg/word/ir"
)

// preWord97PapxEntry is the first old-PAP structure slice for Word 6 / 95.
// It mirrors the paragraph-range information HWPF derives from OldPAPBinTable
// and PAPFormattedDiskPage, but keeps the result in native parser terms.
//
// This first slice intentionally assumes a single-byte body stream, which is
// sufficient for the current repo-local old DOC fixtures. Double-byte old DOC
// text-piece translation can be added when a real fixture requires it.
type preWord97PapxEntry struct {
	StartFC           int
	EndFC             int
	StartCP           int
	EndCP             int
	Format            wordir.ParagraphFormat
	Meta              papxMetadata
	LegacyInTableHint bool
	LegacyRowMarkHint bool
	// Sprms stores the raw PAPX grpprl bytes so that lossless IR can round-trip
	// or reproject formatting without re-reading the binary file.
	Sprms []byte
}

// loadPreWord97PapxEntries reads the old PAP bin table identified by the
// pre-Word-97 body-span map and expands its BTE/FKP pages into paragraph
// ranges. The underlying FKP layout matches the parser's existing Word 97+
// PAPX page decoder closely enough that we can reuse the same grpprl parsing
// logic while keeping the old-doc range translation separate.
func loadPreWord97PapxEntries(wordStream []byte, span PreWord97BodySpan) []preWord97PapxEntry {
	if span.PapTableOffset <= 0 || span.PapTableSize < 10 || span.PapTableOffset+span.PapTableSize > len(wordStream) {
		return nil
	}
	plc := wordStream[span.PapTableOffset : span.PapTableOffset+span.PapTableSize]
	n := (len(plc) - 4) / 6
	if n <= 0 {
		return nil
	}
	cpBytes := (n + 1) * 4
	if cpBytes+n*2 > len(plc) {
		return nil
	}
	var entries []preWord97PapxEntry
	for index := 0; index < n; index++ {
		pageNum := int(binary.LittleEndian.Uint16(plc[cpBytes+index*2 : cpBytes+index*2+2]))
		pageOff := pageNum * 512
		if pageOff < 0 || pageOff+512 > len(wordStream) {
			continue
		}
		entries = append(entries, parsePreWord97PapxFKPPage(wordStream[pageOff:pageOff+512], span)...)
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].StartFC != entries[j].StartFC {
			return entries[i].StartFC < entries[j].StartFC
		}
		return entries[i].EndFC < entries[j].EndFC
	})
	write := 0
	for _, entry := range entries {
		if write > 0 {
			prev := entries[write-1]
			if prev.StartFC == entry.StartFC && prev.EndFC == entry.EndFC {
				continue
			}
		}
		entries[write] = entry
		write++
	}
	return entries[:write]
}

func parsePreWord97PapxFKPPage(page []byte, span PreWord97BodySpan) []preWord97PapxEntry {
	if len(page) < 512 {
		return nil
	}
	cpara := int(page[511])
	if cpara == 0 {
		return nil
	}
	rgbxStart := (cpara + 1) * 4
	// Pre-Word-97 BxPap entries are 7 bytes each (antiword prop6.c: tIndex2*7).
	// Word97+ uses 13-byte BxPap entries.
	if rgbxStart+cpara*7 > 511 {
		return nil
	}
	result := make([]preWord97PapxEntry, 0, cpara)
	for index := 0; index < cpara; index++ {
		startFC := int(binary.LittleEndian.Uint32(page[index*4 : index*4+4]))
		endFC := int(binary.LittleEndian.Uint32(page[(index+1)*4 : (index+1)*4+4]))
		if endFC <= startFC || endFC <= span.Start || startFC >= span.BodyEnd {
			continue
		}
		clippedStart := startFC
		if clippedStart < span.Start {
			clippedStart = span.Start
		}
		clippedEnd := endFC
		if clippedEnd > span.BodyEnd {
			clippedEnd = span.BodyEnd
		}
		var format wordir.ParagraphFormat
		var meta papxMetadata
		legacyInTableHint := false
		legacyRowMarkHint := false
		var sprms []byte
		// First byte of each 7-byte BxPap entry is the bOffset half-word index.
		bOffset := int(page[rgbxStart+index*7])
		if bOffset != 0 {
			off := bOffset * 2
			// Pre-Word-97 PAPx header (antiword vGet6PapInfo):
			//   byte 0:      cb (total = 2*cb bytes)
			//   byte 1:      istd (1 byte, not 2)
			//   byte 2:      (padding / unused)
			//   bytes 3..:   grpprl (1-byte opcode format), length = 2*cb-3
			if off+1 < 512 {
				cb := int(page[off])
				if cb >= 1 { // cb=0 means no formatting; cb=1 means istd only (no grpprl)
					istd := int(page[off+1])
					format.BinaryStyleIndex = &istd
					grpLen := 2*cb - 3
					grpOff := off + 3
					if grpLen > 0 && grpOff+grpLen <= 512 {
						sprms = page[grpOff : grpOff+grpLen]
						grpprlFormat, grpprlMeta, inTableHint, rowMarkHint := parsePreWord97PapxGrpprlWithMetadata(sprms)
						grpprlFormat.BinaryStyleIndex = format.BinaryStyleIndex
						format = grpprlFormat
						meta = grpprlMeta
						legacyInTableHint = legacyInTableHint || inTableHint
						legacyRowMarkHint = legacyRowMarkHint || rowMarkHint
					}
				}
			}
		}
		result = append(result, preWord97PapxEntry{
			StartFC:           clippedStart,
			EndFC:             clippedEnd,
			StartCP:           clippedStart - span.Start,
			EndCP:             clippedEnd - span.Start,
			Format:            format,
			Meta:              meta,
			LegacyInTableHint: legacyInTableHint,
			LegacyRowMarkHint: legacyRowMarkHint,
			Sprms:             sprms,
		})
	}
	return result
}

// preWord97PapxOperandSize returns the operand byte length (bytes after the
// opcode) for an old Word 6/95 PAP grpprl opcode. grpprl[pos] is the first
// byte after the opcode byte. Based on antiword iGet6InfoLength (prop6.c).
func preWord97PapxOperandSize(opcode byte, grpprl []byte, pos int) int {
	switch {
	case opcode == 23: // ChgTabs — special variable size
		if pos >= len(grpprl) {
			return -1
		}
		iTmp := int(grpprl[pos])
		if iTmp == 255 {
			if pos+2 > len(grpprl) {
				return -1
			}
			iDel := int(grpprl[pos+1])
			if pos+2+iDel*4 >= len(grpprl) {
				return -1
			}
			iAdd := int(grpprl[pos+2+iDel*4])
			iTmp = 2 + iDel*4 + iAdd*3
		}
		return 1 + iTmp
	case opcode == 3 || opcode == 12 || opcode == 15 ||
		opcode == 81 || opcode == 103 || opcode == 108 ||
		opcode == 188 || opcode == 190 || opcode == 191:
		// variable: first operand byte is the data byte count
		if pos >= len(grpprl) {
			return -1
		}
		return 1 + int(grpprl[pos])
	case opcode == 2 || opcode == 16 || opcode == 17 || opcode == 18 || opcode == 19 ||
		opcode == 21 || opcode == 22 ||
		(opcode >= 26 && opcode <= 36) ||
		(opcode >= 38 && opcode <= 49) ||
		opcode == 69 || opcode == 72 || opcode == 80 || opcode == 93 ||
		opcode == 96 || opcode == 97 || opcode == 99 ||
		opcode == 101 || opcode == 105 || opcode == 106 || opcode == 107 ||
		opcode == 109 || opcode == 110 ||
		(opcode >= 121 && opcode <= 124) ||
		opcode == 140 || opcode == 141 || opcode == 144 || opcode == 145 ||
		opcode == 148 || opcode == 149 ||
		(opcode >= 154 && opcode <= 157) ||
		(opcode >= 160 && opcode <= 171) ||
		opcode == 182 || opcode == 183 || opcode == 184 || opcode == 189 ||
		opcode == 195 || opcode == 197 || opcode == 198:
		return 2
	case opcode == 20 || opcode == 70 || opcode == 74 ||
		opcode == 192 || opcode == 194 || opcode == 196 || opcode == 200:
		return 4
	case opcode == 68 || opcode == 193 || opcode == 199:
		return 5
	case opcode == 73 || opcode == 95 || opcode == 136 || opcode == 137:
		return 3
	case opcode == 120 || opcode == 187:
		return 12
	default:
		return 1
	}
}

// parsePreWord97PapxGrpprl decodes grpprl bytes using the old Word 6/95 1-byte
// opcode table. Based on antiword vGet6StyleInfo and eGet6RowInfo (prop6.c).
func parsePreWord97PapxGrpprl(grpprl []byte) (wordir.ParagraphFormat, papxMetadata, bool, bool) {
	var out wordir.ParagraphFormat
	var meta papxMetadata
	legacyInTableHint := false
	legacyRowMarkHint := false
	pos := 0
	for pos < len(grpprl) {
		opcode := grpprl[pos]
		pos++
		opSize := preWord97PapxOperandSize(opcode, grpprl, pos)
		if opSize < 0 || pos+opSize > len(grpprl) {
			break
		}
		op := grpprl[pos : pos+opSize]
		pos += opSize
		switch opcode {
		case 2: // istd (2-byte operand per size table)
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				out.BinaryStyleIndex = &v
			}
		case 23: // ChgTabs — delete/add paragraph tab stops
			if len(op) > 1 {
				deleteEntryWidth := 2
				payload := op[1:]
				if op[0] == 255 {
					deleteEntryWidth = 4
				}
				out.TabStops = applyLegacyTabChange(out.TabStops, payload, deleteEntryWidth)
			}
		case 5: // jc — justification
			if len(op) >= 1 {
				switch op[0] {
				case 1:
					out.Alignment = "center"
				case 2:
					out.Alignment = "right"
				case 3:
					out.Alignment = "both"
				}
			}
		case 12: // anld — list number descriptor (includes nfc and optional start value)
			// Opcode 12 is variable-length: op[0] is byte count, op[1] is nfcSeqNumb.
			if len(op) >= 2 {
				v := int(op[1])
				out.BinaryListFormat = &v
			}
			// antiword vGet6StyleInfo reads usStartAt from opcode+12, i.e. op[11:13]
			// when the payload length byte indicates at least 12 bytes of data.
			if len(op) >= 13 {
				startAt := int(binary.LittleEndian.Uint16(op[11:13]))
				if startAt > 0 {
					meta.PreWord97ListStart = &startAt
				}
			}
		case 13: // nLvlAnm — list level
			if len(op) >= 1 {
				v := int(op[0])
				out.BinaryListLevel = &v
			}
		case 16: // dxaRight — right indent (signed twips)
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				out.IndentRight = &v
			}
		case 17: // dxaLeft — left indent (signed twips)
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				out.IndentLeft = &v
			}
		case 18: // Nest dxaLeft — delta to existing left indent
			if len(op) >= 2 {
				delta := int(int16(binary.LittleEndian.Uint16(op)))
				if out.IndentLeft == nil {
					if delta > 0 {
						out.IndentLeft = &delta
					}
				} else {
					v := *out.IndentLeft + delta
					if v < 0 {
						v = 0
					}
					out.IndentLeft = &v
				}
			}
		case 19: // dxaLeft1 — first-line indent (positive) or hanging (negative)
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				if v < 0 {
					hang := -v
					out.IndentHanging = &hang
				} else if v > 0 {
					out.IndentFirstLine = &v
				}
			}
		case 21: // dyaBefore — space before paragraph (unsigned twips)
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				if v > 0 {
					out.SpacingBefore = &v
				}
			}
		case 22: // dyaAfter — space after paragraph (unsigned twips)
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				if v > 0 {
					out.SpacingAfter = &v
				}
			}
		// Cell-mark detection: opcodes 24 (fInTable) and 25 (fTtp) are checked from
		// the paragraph's direct PAPx formatting and returned as hints for run assembly.
		// This layering supports both direct cell marks and (in future phase-2 work)
		// style-inherited cell marks: direct PAPx bits are checked here unconditionally,
		// and style-inheritance fallback would add a second check if no direct bits set.
		// Currently relies on direct PAPx bits being present; corpus fixtures confirm this.
		case 24: // fInTable — paragraph is in a table cell
			legacyInTableHint = true
			if len(op) >= 1 {
				out.InTable = op[0] != 0
			}
		case 25: // fTtp — table row end mark (provenance only; cell-mark detection
			// at run assembly time sets IsTableTrailingParagraph)
			if len(op) >= 1 && op[0] != 0 {
				legacyRowMarkHint = true
			}
		}
	}
	return out, meta, legacyInTableHint, legacyRowMarkHint
}

func parsePreWord97PapxGrpprlWithMetadata(grpprl []byte) (wordir.ParagraphFormat, papxMetadata, bool, bool) {
	return parsePreWord97PapxGrpprl(grpprl)
}

// extractPreWord97PapParagraphRuns decodes the old PAP paragraph ranges into
// docRuns so later pre-97 ports can choose between raw-body fallback and
// structure-aware paragraph assembly.
func extractPreWord97PapEntryRuns(wordStream []byte, span PreWord97BodySpan, syntheticParagraphBreaks bool) []docRun {
	entries := loadPreWord97PapxEntries(wordStream, span)
	if len(entries) == 0 {
		return nil
	}
	preferredCharset := guessPreWord97Charset(wordStream, span)
	runs := make([]docRun, 0, len(entries))
	for _, entry := range entries {
		if entry.StartFC < 0 || entry.EndFC > len(wordStream) || entry.EndFC <= entry.StartFC {
			continue
		}
		text := decodePreWord97SingleByteTextWithCharset(wordStream[entry.StartFC:entry.EndFC], preferredCharset)
		run := docRun{Text: text, StartCP: entry.StartCP, ParaFormat: entry.Format, PapxSprms: entry.Sprms}
		run.LegacyInTableHint = entry.LegacyInTableHint
		run.LegacyRowMarkHint = entry.LegacyRowMarkHint

		// For pre-97 documents, detect cell and row mark paragraphs based on
		// old-PAP one-byte toggles (0x18 = fInTable/cell, 0x19 = fTtp/row-mark).
		// Both end with \a; the distinguishing factor is the toggle that's set.
		if entry.LegacyInTableHint && strings.HasSuffix(text, "\a") {
			run.ParaFormat.IsTableCellTrailingParagraph = true
		}
		if entry.LegacyRowMarkHint && strings.HasSuffix(text, "\a") {
			run.ParaFormat.IsTableTrailingParagraph = true
		}

		if strings.ContainsRune(text, '\a') {
			run.NativeCellMarks = true
		}
		if entry.Meta.TableRowProps != nil {
			run.TableProps = entry.Meta.TableRowProps.clone()
		}
		if run.Text == "" {
			if !run.ParaFormat.IsTableTrailingParagraph && !run.ParaFormat.IsTableCellTrailingParagraph {
				if !syntheticParagraphBreaks {
					continue
				}
				run.Kind = "blankParagraph"
				runs = append(runs, run)
				continue
			}
		} else if syntheticParagraphBreaks && !strings.HasSuffix(run.Text, "\r") && !strings.HasSuffix(run.Text, "\n") && !strings.HasSuffix(run.Text, "\u000B") && !strings.HasSuffix(run.Text, "\u000C") {
			run.Text += "\n"
		}
		runs = append(runs, run)
	}
	return runs
}

// extractPreWord97PapParagraphRuns keeps the entry-level old-PAP ranges visible
// for focused regression tests and diagnostics.
func extractPreWord97PapParagraphRuns(wordStream []byte, span PreWord97BodySpan) []docRun {
	return extractPreWord97PapEntryRuns(wordStream, span, true)
}

func collapsePreWord97PapRuns(wordStream []byte, span PreWord97BodySpan) []docRun {
	rawRuns := extractPreWord97PapEntryRuns(wordStream, span, false)
	if len(rawRuns) == 0 {
		return nil
	}
	collapsed := make([]docRun, 0, len(rawRuns))
	flush := func(current *docRun) {
		if current == nil {
			return
		}
		if current.Kind == "" && current.Text == "" {
			return
		}
		collapsed = append(collapsed, *current)
	}
	var current *docRun
	for _, run := range rawRuns {
		if run.Kind == "blankParagraph" {
			flush(current)
			current = nil
			collapsed = append(collapsed, run)
			continue
		}
		if run.Kind != "" {
			flush(current)
			current = nil
			collapsed = append(collapsed, run)
			continue
		}
		if current == nil {
			copyRun := run
			current = &copyRun
		} else {
			if legacyTableRowPropsConflict(current.TableProps, run.TableProps) {
				flush(current)
				copyRun := run
				current = &copyRun
				continue
			}
			current.Text += run.Text
			current.LegacyInTableHint = current.LegacyInTableHint || run.LegacyInTableHint
			current.LegacyRowMarkHint = current.LegacyRowMarkHint || run.LegacyRowMarkHint
			current.NativeCellMarks = current.NativeCellMarks || run.NativeCellMarks
			current.TableProps = mergeLegacyTableRowProps(current.TableProps, run.TableProps)
		}
		if strings.HasSuffix(current.Text, "\r") || strings.HasSuffix(current.Text, "\n") || strings.HasSuffix(current.Text, "\u000B") || strings.HasSuffix(current.Text, "\u000C") || current.ParaFormat.IsTableTrailingParagraph || current.ParaFormat.IsTableCellTrailingParagraph {
			flush(current)
			current = nil
		}
	}
	flush(current)
	return collapsed
}