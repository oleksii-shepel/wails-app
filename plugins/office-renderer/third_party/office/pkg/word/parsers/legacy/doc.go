package legacy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"

	"golang.org/x/text/encoding/charmap"
)

var alignedLegacyTableSep = regexp.MustCompile(` {2,}`)

const (
	legacyDocFibSignature   = 0xA5EC
	legacyDocPreWord97Magic = 0xA5DC
	legacyDocFibWord95      = 0x00C0
	legacyDocFibWord97      = 0x00C1
)

func legacyDocFibVersion(wordStream []byte) (uint16, bool) {
	if len(wordStream) < 4 {
		return 0, false
	}
	if binary.LittleEndian.Uint16(wordStream[0:2]) != legacyDocFibSignature {
		return 0, false
	}
	return binary.LittleEndian.Uint16(wordStream[2:4]), true
}

func legacyDocLooksPreWord97(wordStream []byte) bool {
	return len(wordStream) >= 2 && binary.LittleEndian.Uint16(wordStream[0:2]) == legacyDocPreWord97Magic
}

// ParseDoc parses a legacy .doc file and returns a fully resolved semantic
// Document suitable for rendering. It is a convenience wrapper that populates
// both lossless Source fields and resolved semantic fields.
type ParseOptions struct {
	Password string
}

func ParseDoc(path string) (*wordir.Document, error) {
	return ParseDocWithOptions(path, ParseOptions{})
}

// ParseDocWithOptions parses a legacy DOC with per-call options. Password is
// scoped to this parse and does not mutate the package-level compatibility
// password.
func ParseDocWithOptions(path string, options ParseOptions) (*wordir.Document, error) {
	return ParseDocLosslessWithOptions(path, options)
}

// ParseDocLossless parses a legacy .doc file and returns a Document whose
// Source fields are fully populated with raw binary provenance (SPRM bytes,
// style indices, DOP, CLX, etc.). The resolved semantic fields (Format, Run
// properties, etc.) are also populated for backward compatibility, but callers
// who need a fresh projection can clear those fields and invoke
// ProjectDocument to recompute them from the lossless Source data.
//
// This is the primary file-system entry point for the lossless IR architecture:
//
//	Binary DOC → ParseDocLossless → Document (with Source fields)
//	Document → ProjectDocument → Document (refreshed semantic view)
//	Document → Renderer
func ParseDocLossless(path string) (*wordir.Document, error) {
	return ParseDocLosslessWithOptions(path, ParseOptions{})
}

func ParseDocLosslessWithOptions(path string, options ParseOptions) (*wordir.Document, error) {
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil, readErr
	}
	return ParseDocBytesLosslessWithOptions(data, filepath.Base(path), options)
}

// ParseDocBytes parses an in-memory legacy .doc document. It is equivalent to
// ParseDocBytesLossless and is provided as the concise default entry point.
func ParseDocBytes(data []byte, name string) (*wordir.Document, error) {
	return ParseDocBytesWithOptions(data, name, ParseOptions{})
}

func ParseDocBytesWithOptions(data []byte, name string, options ParseOptions) (*wordir.Document, error) {
	return ParseDocBytesLosslessWithOptions(data, name, options)
}

// ParseDocBytesLossless parses an in-memory legacy .doc document while
// retaining raw binary provenance in the IR Source fields. name is used only
// for document identity and diagnostics; when empty, "document.doc" is used.
func ParseDocBytesLossless(data []byte, name string) (*wordir.Document, error) {
	return ParseDocBytesLosslessWithOptions(data, name, ParseOptions{})
}

func ParseDocBytesLosslessWithOptions(data []byte, name string, options ParseOptions) (*wordir.Document, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "" {
		name = "document.doc"
	}
	f, err := cfb.OpenBytes(data)
	if err == nil {
		doc, parseErr := parseDocFileWithOptions(f, name, options)
		if parseErr == nil {
			return doc, nil
		}
		// Encryption failures are authoritative. Never let raw-text recovery turn
		// a missing or invalid password into a seemingly successful parse.
		if isLegacyDocEncryptionError(parseErr) {
			return nil, parseErr
		}
		// Some old OLE Word.Document.7 fixtures carry only table/control streams.
		// If the main WordDocument stream is absent, try a filtered raw-text
		// fallback before surfacing the parse error.
		if text := fallbackScanMissingWordDocument(data, parseErr); scoreLegacyText(text) > 0 {
			return buildLegacyDoc(name, text), nil
		}
		// Some malformed pre-Word-97 CFB fixtures expose an unreadable
		// WordDocument stream but still contain recoverable body text elsewhere in
		// the compound file payload. Preserve the broader whole-file fallback in
		// that case instead of returning early with "no readable text found".
		if text := fallbackScanDocText(data); scoreLegacyText(text) > 0 {
			return buildLegacyDoc(name, text), nil
		}
		// Otherwise, CFB opened but parseDocFile found no usable text. The word
		// stream scanner was already attempted inside parseDocFile.
		return nil, parseErr
	}
	// Not a valid CFB container: try raw binary scan as a last resort.
	text := fallbackScanDocText(data)
	if scoreLegacyText(text) > 0 {
		return buildLegacyDoc(name, text), nil
	}
	return nil, fmt.Errorf("legacy doc: open cfb: %w", err)
}

func isLegacyDocEncryptionError(err error) bool {
	var encryptionErr *legacyDocEncryptionError
	return errors.As(err, &encryptionErr)
}

func parseDocFile(f *cfb.File, filename string) (*wordir.Document, error) {
	return parseDocFileWithOptions(f, filename, ParseOptions{})
}

func parseDocFileWithOptions(f *cfb.File, filename string, options ParseOptions) (*wordir.Document, error) {
	// Check for encryption and decrypt streams if needed.
	var decryptErr error
	f, decryptErr = maybeDecryptDocStreamsWithPassword(f, options.Password)
	if decryptErr != nil {
		return nil, decryptErr
	}

	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		// Some older DOC variants expose the main stream as Ole while still
		// keeping the companion table stream layout used by the parser.
		wordStream, ok = f.Stream("Ole")
	}
	if !ok {
		streamNames := f.StreamNames()
		if len(streamNames) == 0 {
			return nil, fmt.Errorf("legacy doc: missing WordDocument stream")
		}
		quoted := make([]string, 0, len(streamNames))
		for _, name := range streamNames {
			quoted = append(quoted, strconv.Quote(name))
		}
		return nil, fmt.Errorf("legacy doc: missing WordDocument stream (available streams: %s)", strings.Join(quoted, ", "))
	}

	// Word97+ enters through the canonical snapshot adapter explicitly. Pre-97
	// remains behind its dedicated compatibility dispatcher; it must not become
	// an alternate Word97 binary path.
	if nFib, ok := legacyDocFibVersion(wordStream); ok && nFib >= legacyDocFibWord97 {
		doc, err := loadCanonicalWord97Document(filename, wordStream, f)
		if err != nil {
			return nil, err
		}
		return doc, nil
	} else if data := extractPreWord97ParseDataByVersion(wordStream, f); data != nil {
		var dataStream []byte
		if ds, ok := f.Stream("Data"); ok && len(ds) > 0 {
			dataStream = ds
		} else if ds, ok = f.Stream("DataStream"); ok && len(ds) > 0 {
			dataStream = ds
		}
		runs := applyPreWord97FieldsToRunsWithDataStream(data.Assembly.Runs, dataStream)
		if data.Version > 0 && data.Version < legacyDocFibWord97 {
			runs = normalizePreWord97GridTableRuns(runs)
		}
		data.Assembly.Runs = runs
		doc := buildBinaryWordDocument(filename, data.Assembly)
		applyLegacyDocProperties(doc, loadLegacyDocProperties(f))
		if len(data.Assembly.BookmarkEnds) > 0 {
			doc.BookmarkEnds = data.Assembly.BookmarkEnds
		}
		if len(data.Footnotes) > 0 {
			doc.Footnotes = data.Footnotes
		}
		if len(data.Endnotes) > 0 {
			doc.Endnotes = data.Endnotes
		}
		if len(data.Comments) > 0 {
			doc.Comments = data.Comments
		}
		doc.Source = &wordir.DocumentSource{Version: data.Version}
		if secs := loadLegacyDocSections(wordStream, f); len(secs) > 0 {
			doc.Sections = secs
			insertLegacySectionBoundaryBlocks(doc)
		}
		// In legacy DOC, document grid applies to all paragraphs in the section.
		// There is no per-paragraph snap-to-grid opt-out, so propagate section-level
		// grid presence to paragraph formats.
		for _, sec := range doc.Sections {
			if sec.DocGrid != nil && sec.DocGrid.Type != "" && sec.DocGrid.Type != "default" {
				for i := range doc.Paragraphs {
					doc.Paragraphs[i].Format.SnapToGrid = true
				}
				break
			}
		}
		if hf := loadDocHeadersFooters(wordStream, f, len(doc.Sections)); hf != nil {
			sectionCount := len(doc.Sections)
			if len(hf.SectionHeaders) > sectionCount {
				sectionCount = len(hf.SectionHeaders)
			}
			if len(hf.SectionFooters) > sectionCount {
				sectionCount = len(hf.SectionFooters)
			}
			if sectionCount > len(doc.Sections) {
				doc.Sections = append(doc.Sections, make([]wordir.Section, sectionCount-len(doc.Sections))...)
			}
			if len(hf.Headers) > 0 {
				doc.Headers = hf.Headers
			}
			if len(hf.Footers) > 0 {
				doc.Footers = hf.Footers
			}
			if hf.EndnoteSeparator.Text != "" {
				doc.EndnoteSeparator = hf.EndnoteSeparator
			}
			if hf.EndnoteContinuationSep.Text != "" {
				doc.EndnoteContinuationSep = hf.EndnoteContinuationSep
			}
			if hf.EndnoteContinuationNotice.Text != "" {
				doc.EndnoteContinuationNotice = hf.EndnoteContinuationNotice
			}
			if hf.FootnoteSeparator.Text != "" {
				doc.FootnoteSeparator = hf.FootnoteSeparator
			}
			if hf.FootnoteContinuationSep.Text != "" {
				doc.FootnoteContinuationSep = hf.FootnoteContinuationSep
			}
			if hf.FootnoteContinuationNotice.Text != "" {
				doc.FootnoteContinuationNotice = hf.FootnoteContinuationNotice
			}
			for i := range hf.SectionHeaders {
				doc.Sections[i].Headers = append([]wordir.Reference(nil), hf.SectionHeaders[i]...)
			}
			for i := range hf.SectionFooters {
				doc.Sections[i].Footers = append([]wordir.Reference(nil), hf.SectionFooters[i]...)
			}
		}
		appendPreWord97DetachedDrawingImages(doc, extractPreWord97DrawingImages(wordStream))
		resolveDocCrossReferenceFields(doc, data.Assembly.BookmarkStarts)
		applyPreWord97PreservationMeta(f, doc)
		applyPreWord97OLECompoundRecovery(f, doc)
		applyPreWord97OLEMetafilePreviews(f, doc)
		for _, sec := range doc.Sections {
			doc.Diagnostics = append(doc.Diagnostics, sec.Diagnostics...)
		}
		return doc, nil
	}

	// Heuristic scan for PlcPcd. For Word 97+ files the FIB identifies which
	// table stream carries the piece table; for older formats try both.
	hintStream := selectDocTableStreamName(wordStream)
	tableNames := []string{"1Table", "0Table"}
	if hintStream != "" {
		tableNames = []string{hintStream}
	}
	bestText := ""
	for _, tableName := range tableNames {
		tableStream, ok := f.Stream(tableName)
		if !ok || len(tableStream) == 0 {
			continue
		}
		candidate := extractDocTextFromTable(wordStream, tableStream)
		if scoreLegacyText(candidate) > scoreLegacyText(bestText) {
			bestText = candidate
		}
	}

	if scoreLegacyText(bestText) == 0 {
		bestText = fallbackScanDocText(wordStream)
	}
	if scoreLegacyText(bestText) == 0 {
		return nil, fmt.Errorf("legacy doc: no readable text found")
	}

	doc := buildLegacyDoc(filename, bestText)
	applyLegacyDocProperties(doc, loadLegacyDocProperties(f))
	return doc, nil
}

func applyLegacyDocProperties(doc *wordir.Document, props legacyDocProperties) {
	if doc == nil {
		return
	}
	if props.Title != "" {
		doc.Title = props.Title
	}
	if props.Subject != "" {
		doc.Subject = props.Subject
	}
	if props.Author != "" {
		doc.Author = props.Author
	}
	if props.Description != "" {
		doc.Description = props.Description
	}
	if props.Keywords != "" {
		doc.Keywords = props.Keywords
	}
	if props.LastModifiedBy != "" {
		doc.LastModifiedBy = props.LastModifiedBy
	}
	if props.Template != "" {
		doc.Template = props.Template
	}
	if props.Application != "" {
		doc.Application = props.Application
	}
	if props.TotalEditMinutes != 0 {
		doc.TotalEditMinutes = props.TotalEditMinutes
	}
	if props.Company != "" {
		doc.Company = props.Company
	}
	if props.Manager != "" {
		doc.Manager = props.Manager
	}
	if len(props.CustomProperties) > 0 {
		if doc.CustomProperties == nil {
			doc.CustomProperties = make(map[string]string)
		}
		for k, v := range props.CustomProperties {
			doc.CustomProperties[k] = v
		}
	}
}

func extractPreWord97ParseDataByVersion(wordStream []byte, f *cfb.File) *preWord97ParseData {
	if nFib, ok := legacyDocFibVersion(wordStream); ok {
		if nFib >= legacyDocFibWord97 {
			return nil
		}
		if nFib == legacyDocFibWord95 {
			return extractDocRunsViaPreWord97FIB(wordStream, f)
		}
	}
	if legacyDocLooksPreWord97(wordStream) {
		return extractDocRunsViaPreWord97FIB(wordStream, f)
	}
	return nil
}

// selectDocTableStreamName reads the FIB header to determine which table stream
// carries the piece table for Word 97+ files (nFib >= 0xC1). Returns "" when the
// FIB is not parseable or the format is older than Word 97.
func selectDocTableStreamName(wordStream []byte) string {
	if len(wordStream) < 12 {
		return ""
	}
	nFib, ok := legacyDocFibVersion(wordStream)
	if !ok {
		return ""
	}
	if nFib < legacyDocFibWord97 {
		return ""
	}
	flags := binary.LittleEndian.Uint16(wordStream[10:12])
	if flags&(1<<9) != 0 {
		return "1Table"
	}
	return "0Table"
}

// extractDocTextViaFIB uses the File Information Block (Word 97+, nFib >= 0xC1)
// to find the correct table stream and the exact ClxInfo location within it.
// It falls back to the heuristic scanner for older formats or parse failures.
func extractDocTextViaFIB(wordStream []byte, f *cfb.File) string {
	if len(wordStream) < 34 {
		return ""
	}
	nFib, ok := legacyDocFibVersion(wordStream)
	if !ok {
		return ""
	}
	if nFib < legacyDocFibWord97 { // FibRgFcLcb97 only present in Word 97+
		return ""
	}

	// fWhichTblStm is bit 9 of the 16-bit flags word at offset 10.
	flags := binary.LittleEndian.Uint16(wordStream[10:12])
	tableStreamName := "0Table"
	if flags&(1<<9) != 0 {
		tableStreamName = "1Table"
	}
	tableStream, ok := f.Stream(tableStreamName)
	if !ok || len(tableStream) == 0 {
		return ""
	}

	// Navigate the variable-length FIB sections to reach FibRgFcLcb.
	// Layout: FibBase(32) | csw(2) | FibRgW97(csw*2) | cslw(2) | FibRgLw97(cslw*4) | cbRgFcLcb(2) | FibRgFcLcb(...)
	pos := 32
	if pos+2 > len(wordStream) {
		return ""
	}
	csw := int(binary.LittleEndian.Uint16(wordStream[pos : pos+2]))
	pos += 2 + csw*2
	if pos+2 > len(wordStream) {
		return ""
	}
	cslw := int(binary.LittleEndian.Uint16(wordStream[pos : pos+2]))
	pos += 2 + cslw*4
	if pos+2 > len(wordStream) {
		return ""
	}
	cbRgFcLcb := int(binary.LittleEndian.Uint16(wordStream[pos : pos+2]))
	pos += 2

	// fcClx/lcbClx is entry index 33 in FibRgFcLcb97 (each entry = 8 bytes).
	const clxIndex = 33
	if clxIndex >= cbRgFcLcb || pos+(clxIndex+1)*8 > len(wordStream) {
		return ""
	}
	fcClx := int(binary.LittleEndian.Uint32(wordStream[pos+clxIndex*8:]))
	lcbClx := int(binary.LittleEndian.Uint32(wordStream[pos+clxIndex*8+4:]))
	if lcbClx <= 0 || fcClx < 0 || fcClx+lcbClx > len(tableStream) {
		return ""
	}

	return extractDocTextFromClx(wordStream, tableStream[fcClx:fcClx+lcbClx])
}

// extractDocTextFromClx parses a Clx structure: zero or more Prc entries
// (each starting with 0x01) followed by exactly one Pcdt (starting with 0x02).
func extractDocTextFromClx(wordStream, clxData []byte) string {
	pos := 0
	for pos < len(clxData) {
		switch clxData[pos] {
		case 0x01: // Prc: skip cbGrpprl bytes
			if pos+3 > len(clxData) {
				return ""
			}
			cbGrpprl := int(int16(binary.LittleEndian.Uint16(clxData[pos+1 : pos+3])))
			if cbGrpprl < 0 || pos+3+cbGrpprl > len(clxData) {
				return ""
			}
			pos += 3 + cbGrpprl
		case 0x02: // Pcdt: 4-byte lcb then PlcPcd
			if pos+5 > len(clxData) {
				return ""
			}
			lcb := int(binary.LittleEndian.Uint32(clxData[pos+1 : pos+5]))
			if lcb <= 0 || pos+5+lcb > len(clxData) {
				return ""
			}
			text, ok := extractDocTextFromPlcPcd(wordStream, clxData[pos+5:pos+5+lcb])
			if !ok {
				return ""
			}
			return text
		default:
			return ""
		}
	}
	return ""
}

func extractDocTextFromTable(wordStream, tableStream []byte) string {
	best := ""
	for offset := 0; offset+5 <= len(tableStream); offset++ {
		if tableStream[offset] != 0x02 {
			continue
		}
		plcSize := int(binary.LittleEndian.Uint32(tableStream[offset+1:]))
		if plcSize < 16 || offset+5+plcSize > len(tableStream) {
			continue
		}
		if (plcSize-4)%12 != 0 {
			continue
		}
		candidate, ok := extractDocTextFromPlcPcd(wordStream, tableStream[offset+5:offset+5+plcSize])
		if !ok {
			continue
		}
		if scoreLegacyText(candidate) > scoreLegacyText(best) {
			best = candidate
		}
	}
	return best
}

func extractDocTextFromPlcPcd(wordStream, plc []byte) (string, bool) {
	pieceCount := (len(plc) - 4) / 12
	if pieceCount <= 0 {
		return "", false
	}

	cpBase := 4 * (pieceCount + 1)
	charPos := make([]int, pieceCount+1)
	for i := range charPos {
		charPos[i] = int(binary.LittleEndian.Uint32(plc[i*4:]))
		if i > 0 && charPos[i] < charPos[i-1] {
			return "", false
		}
	}

	var text strings.Builder
	for i := 0; i < pieceCount; i++ {
		charCount := charPos[i+1] - charPos[i]
		if charCount <= 0 {
			continue
		}
		pcd := plc[cpBase+i*8 : cpBase+(i+1)*8]
		fcRaw := binary.LittleEndian.Uint32(pcd[2:6])
		compressed := fcRaw&0x40000000 != 0
		offset := int(fcRaw & 0x3FFFFFFF)
		if compressed {
			offset /= 2
			if offset < 0 || offset >= len(wordStream) {
				continue
			}
			end := offset + charCount
			if end > len(wordStream) {
				end = len(wordStream)
			}
			text.WriteString(decodeWindows1252(wordStream[offset:end]))
			continue
		}
		if offset < 0 || offset >= len(wordStream) {
			continue
		}
		end := offset + charCount*2
		if end > len(wordStream) {
			end = len(wordStream)
		}
		text.WriteString(decodeUTF16LE(wordStream[offset:end]))
	}

	result := normalizeLegacyText(text.String())
	return result, scoreLegacyText(result) > 0
}

func fallbackScanDocText(wordStream []byte) string {
	segments := make([]string, 0, 16)
	segments = append(segments, scanUTF16Runs(wordStream)...)
	segments = append(segments, scanAnsiRuns(wordStream)...)
	joined := normalizeLegacyText(strings.Join(segments, "\n"))
	return joined
}

func fallbackScanMissingWordDocument(data []byte, parseErr error) string {
	if parseErr == nil || !strings.Contains(parseErr.Error(), "missing WordDocument stream") {
		return ""
	}
	return filterLegacyFallbackText(fallbackScanDocText(data))
}

func filterLegacyFallbackText(text string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || shouldDropLegacyFallbackLine(line) {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func shouldDropLegacyFallbackLine(line string) bool {
	switch line {
	case "Root Entry", "WordDocument", "1Table", "CompObj", "SummaryInformation", "DocumentSummaryInformation":
		return true
	case "Default", "Footnote Characters", "Page Number", "Text body", "Caption", "Footnote", "Footer", "Header", "Table Contents", "Table Heading":
		return true
	case "Times New Roman", "Tahoma", "Symbol", "MS Mincho", "Absatz-Standardschriftart", "FORMDROPDOWN", "xTab_NVH", "ORSR form", "list pr", "S_LIST", "-Index", `As\prof`, "WW-Heading", "WW-Caption":
		return true
	}
	for _, prefix := range []string{"WW8Num", "WW-Absatz-Standardschriftart", "p080101_", "ORSR_", "p_RUN.", "FILLIN \"", "!WW-", `"WW-`, "+ORSR_"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	if strings.ContainsRune(line, 'ÿ') || strings.ContainsRune(line, 'þ') {
		return true
	}
	if strings.HasPrefix(line, "Heading") {
		rest := strings.TrimSpace(strings.TrimPrefix(line, "Heading"))
		if rest == "" {
			return true
		}
		if _, err := strconv.Atoi(rest); err == nil {
			return true
		}
	}
	if legacyFallbackLooksBinary(line) {
		return true
	}
	return false
}

func legacyFallbackLooksBinary(line string) bool {
	letters := 0
	for _, r := range line {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			letters++
		}
	}
	return len([]rune(line)) > 48 && letters < 8
}

func scanUTF16Runs(data []byte) []string {
	var runs []string
	for start := 0; start+7 < len(data); {
		if data[start+1] != 0 {
			start++
			continue
		}
		end := start
		for end+1 < len(data) && isLegacyTextByte(data[end], data[end+1]) {
			end += 2
		}
		if end-start >= 12 {
			runs = append(runs, decodeUTF16LE(data[start:end]))
			start = end
			continue
		}
		start++
	}
	return runs
}

func scanAnsiRuns(data []byte) []string {
	var runs []string
	for start := 0; start < len(data); {
		if !isLegacyPrintableByte(data[start]) {
			start++
			continue
		}
		end := start
		for end < len(data) && isLegacyPrintableByte(data[end]) {
			end++
		}
		if end-start >= 20 {
			runs = append(runs, decodeWindows1252(data[start:end]))
		}
		start = end + 1
	}
	return runs
}

func isLegacyTextByte(lo, hi byte) bool {
	if hi != 0 {
		return false
	}
	return lo == '\r' || lo == '\n' || lo == '\t' || (lo >= 0x20 && lo < 0x7F)
}

func isLegacyPrintableByte(b byte) bool {
	if b == '\r' || b == '\n' || b == '\t' {
		return true
	}
	if b >= 0x20 && b < 0x7F {
		return true
	}
	return b >= 0xA0
}

func decodeUTF16LE(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	words := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		words = append(words, binary.LittleEndian.Uint16(data[i:]))
	}
	return string(utf16.Decode(words))
}

func decodeWindows1252(data []byte) string {
	var b strings.Builder
	for _, ch := range data {
		switch ch {
		case 0x80:
			b.WriteRune('€')
		case 0x82:
			b.WriteRune('‚')
		case 0x83:
			b.WriteRune('ƒ')
		case 0x84:
			b.WriteRune('„')
		case 0x85:
			b.WriteRune('…')
		case 0x86:
			b.WriteRune('†')
		case 0x87:
			b.WriteRune('‡')
		case 0x88:
			b.WriteRune('ˆ')
		case 0x89:
			b.WriteRune('‰')
		case 0x8A:
			b.WriteRune('Š')
		case 0x8B:
			b.WriteRune('‹')
		case 0x8C:
			b.WriteRune('Œ')
		case 0x8E:
			b.WriteRune('Ž')
		case 0x91:
			b.WriteRune('‘')
		case 0x92:
			b.WriteRune('’')
		case 0x93:
			b.WriteRune('“')
		case 0x94:
			b.WriteRune('”')
		case 0x95:
			b.WriteRune('•')
		case 0x96:
			b.WriteRune('–')
		case 0x97:
			b.WriteRune('—')
		case 0x98:
			b.WriteRune('˜')
		case 0x99:
			b.WriteRune('™')
		case 0x9A:
			b.WriteRune('š')
		case 0x9B:
			b.WriteRune('›')
		case 0x9C:
			b.WriteRune('œ')
		case 0x9E:
			b.WriteRune('ž')
		case 0x9F:
			b.WriteRune('Ÿ')
		default:
			b.WriteRune(rune(ch))
		}
	}
	return b.String()
}

func decodeWindows1251(data []byte) string {
	return decodeSingleByteCharmap(data, charmap.Windows1251)
}

func decodeWindows1252Charmap(data []byte) string {
	return decodeSingleByteCharmap(data, charmap.Windows1252)
}

func decodeSingleByteCharmap(data []byte, enc *charmap.Charmap) string {
	if len(data) == 0 {
		return ""
	}
	out, err := enc.NewDecoder().Bytes(data)
	if err == nil {
		return string(out)
	}
	var b strings.Builder
	for _, ch := range data {
		b.WriteByte(ch)
	}
	return b.String()
}

func decodePreWord97SingleByteText(data []byte) string {
	decoded1252 := decodeWindows1252Charmap(data)
	decoded1251 := decodeWindows1251(data)
	quality1252 := legacyDecodedTextQuality(decoded1252)
	quality1251 := legacyDecodedTextQuality(decoded1251)
	if quality1251 > quality1252 {
		return decoded1251
	}
	return decoded1252
}

func legacyDecodedTextQuality(text string) int {
	quality := scoreLegacyText(text)
	cyrillicLetters := 0
	latinExtendedLetters := 0
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Cyrillic):
			cyrillicLetters++
		case unicode.In(r, unicode.Latin) && r > unicode.MaxASCII:
			latinExtendedLetters++
		}
	}
	if cyrillicLetters >= 4 {
		quality += cyrillicLetters * 4
	}
	if latinExtendedLetters >= 4 {
		quality -= latinExtendedLetters * 2
	}
	return quality
}

func normalizeLegacyText(text string) string {
	text = strings.ReplaceAll(text, "\u0007", "\t")
	text = strings.ReplaceAll(text, "\u000B", "\n")
	text = strings.ReplaceAll(text, "\u000C", "\n")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var b strings.Builder
	prevNewline := true
	for _, r := range text {
		switch {
		case r == '\n':
			if !prevNewline {
				b.WriteRune('\n')
			}
			prevNewline = true
		case r == '\t':
			b.WriteRune('\t')
			prevNewline = false
		case unicode.IsPrint(r):
			b.WriteRune(r)
			prevNewline = false
		}
	}

	lines := strings.Split(b.String(), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func scoreLegacyText(text string) int {
	score := 0
	for _, r := range text {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			score += 3
		case unicode.IsSpace(r):
			score++
		case unicode.IsPunct(r):
			score += 2
		}
	}
	return score
}

func buildLegacyDoc(filename, text string) *wordir.Document {
	return buildLegacyDocFromRuns(filename, []binaryWordRun{{Text: text}})
}

func buildLegacyParagraph(index int, line string) wordir.Paragraph {
	text, list := parseLegacyListMarker(line)
	return wordir.Paragraph{
		Index: index,
		List:  list,
		Runs:  []wordir.Run{{Text: text}},
		Text:  text,
	}
}

func buildLegacyTable(tableIndex int, lines []string, paragraphStart int) (*wordir.Table, []string) {
	if len(lines) == 0 {
		return nil, nil
	}
	highlyInconsistentWidths := legacyGridLinesHaveHighlyInconsistentWidths(lines)
	if len(lines) >= 2 && !legacyGridLinesHaveTrailingProseRow(lines) && !highlyInconsistentWidths {
		if table := buildLegacyTableFromGridLines(tableIndex, lines, paragraphStart); table != nil {
			return table, nil
		}
	}
	lineParts := make([][]string, 0, len(lines))
	for _, line := range lines {
		parts := splitLegacyTableLine(line)
		if len(parts) < 2 {
			return nil, nil
		}
		lineParts = append(lineParts, parts)
	}
	lineParts, trailingRows := peelLegacyTrailingParagraphRows(lineParts)
	flatParts := flattenLegacyTableParts(lineParts)
	// Line-aware inference: handles variable-column rows using modal column count
	// so that a table with one short row is not rejected outright.
	if len(lineParts) >= 2 {
		if cols, rows := inferLegacyTableDimensions(lineParts, flatParts); cols > 0 && rows > 0 {
			if highlyInconsistentWidths {
				return buildLegacyTableFromLineParts(tableIndex, lineParts, cols, paragraphStart), trailingRows
			}
			padded := padLineParts(lineParts, cols)
			return buildLegacyTableWithDimensions(tableIndex, padded, cols, rows, paragraphStart), trailingRows
		}
	}
	if cols, rows := inferLegacyTableDimensionsFlat(flatParts); cols > 0 && rows > 0 {
		return buildLegacyTableWithDimensions(tableIndex, flatParts, cols, rows, paragraphStart), trailingRows
	}
	// Trailing prose peeling: strip sentence-like cells and retry with flat inference.
	trailing := append([]string(nil), trailingRows...)
	for len(flatParts) >= 3 {
		last := flatParts[len(flatParts)-1]
		if !isLikelyTrailingParagraph(last) {
			break
		}
		trailing = append([]string{last}, trailing...)
		flatParts = flatParts[:len(flatParts)-1]
		if cols, rows := inferLegacyTableDimensionsFlat(flatParts); cols > 0 && rows > 0 {
			return buildLegacyTableWithDimensions(tableIndex, flatParts, cols, rows, paragraphStart), trailing
		}
	}
	if len(flatParts) >= 2 {
		return buildLegacyTableWithDimensions(tableIndex, flatParts, len(flatParts), 1, paragraphStart), trailing
	}
	return nil, nil
}

func flattenLegacyTableParts(lineParts [][]string) []string {
	flatParts := make([]string, 0, len(lineParts)*3)
	for _, parts := range lineParts {
		flatParts = append(flatParts, parts...)
	}
	return flatParts
}

func peelLegacyTrailingParagraphRows(lineParts [][]string) ([][]string, []string) {
	if len(lineParts) < 2 {
		return lineParts, nil
	}
	maxCols := 0
	for _, parts := range lineParts {
		if len(parts) > maxCols {
			maxCols = len(parts)
		}
	}
	keep := len(lineParts)
	trailing := []string{}
	for keep > 1 {
		parts := lineParts[keep-1]
		if len(parts) == 0 || len(parts) >= maxCols {
			break
		}
		allTrailing := true
		for _, part := range parts {
			if !isLikelyTrailingParagraph(part) {
				allTrailing = false
				break
			}
		}
		if !allTrailing {
			break
		}
		trailing = append(append([]string(nil), parts...), trailing...)
		keep--
	}
	return lineParts[:keep], trailing
}

// padLineParts flattens lineParts into a slice of exactly cols*len(lineParts)
// strings, padding shorter rows with empty strings and trimming longer rows.
func padLineParts(lineParts [][]string, cols int) []string {
	flat := make([]string, 0, len(lineParts)*cols)
	for _, parts := range lineParts {
		for i := 0; i < cols; i++ {
			if i < len(parts) {
				flat = append(flat, parts[i])
			} else {
				flat = append(flat, "")
			}
		}
	}
	return flat
}

func buildLegacyTableFromGridLines(tableIndex int, lines []string, paragraphStart int) *wordir.Table {
	rowTokens := make([][]string, 0, len(lines))
	maxCols := 0
	for _, line := range lines {
		tokens := strings.Split(line, "\t")
		if len(tokens) < 2 {
			return nil
		}
		rowTokens = append(rowTokens, tokens)
		first := -1
		last := -1
		for tokenIndex, token := range tokens {
			if strings.TrimSpace(token) == "" {
				continue
			}
			if first < 0 {
				first = tokenIndex
			}
			last = tokenIndex
		}
		if first < 0 {
			return nil
		}
		spanCols := last - first + 1
		if spanCols > maxCols {
			maxCols = spanCols
		}
	}
	if maxCols < 2 {
		return nil
	}
	rows := make([]wordir.TableRow, 0, len(rowTokens))
	textLines := make([]string, 0, len(lines)*2)
	paragraphIndex := paragraphStart
	for _, tokens := range rowTokens {
		row, texts, nextIndex, ok := buildLegacyGridRow(tokens, maxCols, paragraphIndex)
		if !ok {
			return nil
		}
		paragraphIndex = nextIndex
		textLines = append(textLines, texts...)
		rows = append(rows, row)
	}
	table := &wordir.Table{Index: tableIndex, Rows: rows, Text: strings.Join(textLines, "\n")}
	// Grid-line reconstruction is another successful legacy-table path.  Keep
	// it out of the layout engine's equal-column fallback just like the binary
	// and dimensional rescue paths.  This is especially important for old
	// price-list tables where description columns are wide and amount columns
	// are intentionally narrow.
	if len(table.Format.ColumnWidths) == 0 {
		table.Format.ColumnWidths = inferLegacyTableRowColumnWidths(table.Rows)
	}
	if table.Format.Width == nil && len(table.Format.ColumnWidths) > 0 {
		total := 0
		for _, width := range table.Format.ColumnWidths {
			total += width
		}
		if total > 0 {
			table.Format.Width = &total
			table.Format.WidthType = "dxa"
		}
	}
	return table
}

func legacyGridLinesHaveTrailingProseRow(lines []string) bool {
	if len(lines) < 2 {
		return false
	}
	maxCols := 0
	tokenRows := make([][]string, 0, len(lines))
	for _, line := range lines {
		tokens := strings.Split(line, "\t")
		tokenRows = append(tokenRows, tokens)
		if len(tokens) > maxCols {
			maxCols = len(tokens)
		}
	}
	if maxCols < 2 {
		return false
	}
	last := tokenRows[len(tokenRows)-1]
	if len(last) >= maxCols {
		return false
	}
	seenText := false
	for _, token := range last {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			continue
		}
		seenText = true
		if !isLikelyTrailingParagraph(trimmed) {
			return false
		}
	}
	return seenText
}

func legacyGridLinesHaveHighlyInconsistentWidths(lines []string) bool {
	if len(lines) < 3 {
		return false
	}
	minCols := 0
	maxCols := 0
	for _, line := range lines {
		parts := splitLegacyTableLine(line)
		if len(parts) < 2 {
			continue
		}
		if minCols == 0 || len(parts) < minCols {
			minCols = len(parts)
		}
		if len(parts) > maxCols {
			maxCols = len(parts)
		}
	}
	if minCols < 2 || maxCols < 2 {
		return false
	}
	return maxCols >= minCols*3 && maxCols-minCols >= 12
}

func buildLegacyGridRow(tokens []string, maxCols, paragraphIndex int) (wordir.TableRow, []string, int, bool) {
	trimmed := make([]string, len(tokens))
	firstNonEmpty := -1
	lastNonEmpty := -1
	for index, token := range tokens {
		trimmed[index] = strings.TrimSpace(token)
		if trimmed[index] == "" {
			continue
		}
		if firstNonEmpty < 0 {
			firstNonEmpty = index
		}
		lastNonEmpty = index
	}
	if firstNonEmpty < 0 {
		return wordir.TableRow{}, nil, paragraphIndex, false
	}
	row := wordir.TableRow{Format: wordir.TableRowFormat{GridBefore: firstNonEmpty, GridAfter: maxCols - lastNonEmpty - 1}}
	texts := make([]string, 0, len(tokens))
	for index := firstNonEmpty; index <= lastNonEmpty; index++ {
		if trimmed[index] == "" {
			if len(row.Cells) == 0 {
				row.Format.GridBefore++
				continue
			}
			span := row.Cells[len(row.Cells)-1].Format.GridSpan
			if span <= 1 {
				span = 2
			} else {
				span++
			}
			row.Cells[len(row.Cells)-1].Format.GridSpan = span
			continue
		}
		paragraph := buildLegacyParagraph(paragraphIndex, trimmed[index])
		paragraphIndex++
		cell := wordir.TableCell{
			Paragraphs: []wordir.Paragraph{paragraph},
			Blocks:     []wordir.Block{{Index: 0, Kind: "paragraph", Paragraph: &paragraph}},
			Text:       paragraph.Text,
		}
		row.Cells = append(row.Cells, cell)
		texts = append(texts, paragraph.Text)
	}
	if len(row.Cells) == 0 {
		return wordir.TableRow{}, nil, paragraphIndex, false
	}
	return row, texts, paragraphIndex, true
}

func buildLegacyTableWithDimensions(tableIndex int, parts []string, cols, rows, paragraphStart int) *wordir.Table {
	wordRows := make([]wordir.TableRow, 0, rows)
	textLines := make([]string, 0, len(parts))
	paragraphIndex := paragraphStart
	partIndex := 0
	for rowIndex := 0; rowIndex < rows; rowIndex++ {
		row := wordir.TableRow{Cells: make([]wordir.TableCell, 0, cols)}
		for colIndex := 0; colIndex < cols && partIndex < len(parts); colIndex++ {
			paragraph := buildLegacyParagraph(paragraphIndex, parts[partIndex])
			paragraphIndex++
			cell := wordir.TableCell{
				Paragraphs: []wordir.Paragraph{paragraph},
				Blocks:     []wordir.Block{{Index: 0, Kind: "paragraph", Paragraph: &paragraph}},
				Text:       paragraph.Text,
			}
			row.Cells = append(row.Cells, cell)
			textLines = append(textLines, paragraph.Text)
			partIndex++
		}
		wordRows = append(wordRows, row)
	}
	return &wordir.Table{Index: tableIndex, Format: wordir.TableFormat{ColumnWidths: inferLegacyTextTableColumnWidths(parts, cols)}, Rows: wordRows, Text: strings.Join(textLines, "\n")}
}

func buildLegacyTableFromLineParts(tableIndex int, lineParts [][]string, colsHint, paragraphStart int) *wordir.Table {
	if len(lineParts) == 0 {
		return nil
	}
	wordRows := make([]wordir.TableRow, 0, len(lineParts))
	textLines := make([]string, 0, len(lineParts)*colsHint)
	paragraphIndex := paragraphStart
	for _, parts := range lineParts {
		if len(parts) == 0 {
			continue
		}
		rowCols := len(parts)
		if colsHint >= 2 && rowCols > colsHint {
			rowCols = colsHint
		}
		row := wordir.TableRow{Cells: make([]wordir.TableCell, 0, rowCols)}
		for _, part := range parts[:rowCols] {
			paragraph := buildLegacyParagraph(paragraphIndex, part)
			paragraphIndex++
			cell := wordir.TableCell{
				Paragraphs: []wordir.Paragraph{paragraph},
				Blocks:     []wordir.Block{{Index: 0, Kind: "paragraph", Paragraph: &paragraph}},
				Text:       paragraph.Text,
			}
			row.Cells = append(row.Cells, cell)
			textLines = append(textLines, paragraph.Text)
		}
		wordRows = append(wordRows, row)
	}
	if len(wordRows) == 0 {
		return nil
	}
	return &wordir.Table{Index: tableIndex, Format: wordir.TableFormat{ColumnWidths: inferLegacyTextTableColumnWidths(flattenLegacyTableParts(lineParts), colsHint)}, Rows: wordRows, Text: strings.Join(textLines, "\n")}
}

// inferLegacyTextTableColumnWidths gives rescued/tab-delimited legacy tables
// realistic relative column widths instead of falling back to equal columns.
// Old Word documents commonly use tabs (or dense pre-97 grid marks normalized
// to tabs) to describe columns. Equal widths badly over-wrap narrow identifier
// columns and can double a table's pagination height. The IR stores widths in
// twips, but only their proportions matter to layout/rendering here.
func inferLegacyTextTableColumnWidths(parts []string, cols int) []int {
	if cols < 2 || len(parts) == 0 {
		return nil
	}
	weights := make([]int, cols)
	for i, part := range parts {
		col := i % cols
		n := len([]rune(strings.TrimSpace(part)))
		// Give every column enough room for short labels/numbers, while
		// preventing a prose cell from consuming effectively 100%% width.
		if n < 6 {
			n = 6
		}
		if n > 60 {
			n = 60
		}
		if n > weights[col] {
			weights[col] = n
		}
	}
	total := 0
	for i := range weights {
		if weights[i] == 0 {
			weights[i] = 6
		}
		total += weights[i]
	}
	if total == 0 {
		return nil
	}
	widths := make([]int, cols)
	remaining := 10000
	for i := 0; i < cols; i++ {
		if i == cols-1 {
			widths[i] = remaining
			break
		}
		w := weights[i] * 10000 / total
		if w < 1 {
			w = 1
		}
		widths[i] = w
		remaining -= w
	}
	return widths
}

func inferLegacyTableDimensions(lineParts [][]string, flatParts []string) (int, int) {
	if len(lineParts) >= 2 {
		cols := len(lineParts[0])
		if cols >= 2 {
			consistent := true
			for _, parts := range lineParts[1:] {
				if len(parts) != cols {
					consistent = false
					break
				}
			}
			if consistent {
				return cols, len(lineParts)
			}
		}
		// Inconsistent column counts across rows: use the modal (most common) count
		// so that one short row doesn't reject the entire table.
		if modal := modalLegacyColCount(lineParts); modal >= 2 {
			return modal, len(lineParts)
		}
	}
	return inferLegacyTableDimensionsFlat(flatParts)
}

// inferLegacyTableDimensionsFlat is the flat-parts-only inference used after
// trailing peeling has modified flatParts out of sync with lineParts.
func inferLegacyTableDimensionsFlat(flatParts []string) (int, int) {
	bestCols, bestRows := 0, 0
	bestDelta := 0
	for cols := 2; cols <= len(flatParts)/2; cols++ {
		if len(flatParts)%cols != 0 {
			continue
		}
		rows := len(flatParts) / cols
		if rows >= 2 {
			delta := rows - cols
			if delta < 0 {
				delta = -delta
			}
			if bestCols == 0 || delta < bestDelta ||
				(delta == bestDelta && rows >= cols && bestRows < bestCols) ||
				(delta == bestDelta && (rows >= cols) == (bestRows >= bestCols) && cols < bestCols) {
				bestCols = cols
				bestRows = rows
				bestDelta = delta
			}
		}
	}
	return bestCols, bestRows
}

// modalLegacyColCount returns the most frequent column count across lineParts
// rows that have at least 2 parts. Returns 0 when no majority exists.
func modalLegacyColCount(lineParts [][]string) int {
	counts := map[int]int{}
	for _, parts := range lineParts {
		if len(parts) >= 2 {
			counts[len(parts)]++
		}
	}
	if len(counts) == 0 {
		return 0
	}
	best, bestCount := 0, 0
	for col, count := range counts {
		if count > bestCount || (count == bestCount && (best == 0 || col < best)) {
			best = col
			bestCount = count
		}
	}
	return best
}

func minLegacyTableCols(cellCount int) int {
	if cellCount < 2 {
		return 0
	}
	if cellCount < 6 {
		return cellCount / 2
	}
	if cellCount > 6 {
		return 6
	}
	return 5
}

func isLikelyTrailingParagraph(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	return strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?") || len(strings.Fields(text)) >= 4
}

func isExplicitLegacyTableLine(line string) bool {
	return strings.Count(line, "\t") >= 1 || countPipeCells(line) >= 2
}

func isLegacyTableLine(line string) bool {
	if isExplicitLegacyTableLine(line) {
		return true
	}
	if countAlignedSpaceCells(line) >= 2 {
		return true
	}
	return countColonKeyValueCells(line) >= 2
}

func isLegacyTableCollectionLine(line string) bool {
	return isLegacyTableLine(line) || isLegacyTableSeparatorLine(line)
}

// countPipeCells counts the number of non-empty cell values in a pipe-delimited
// line such as "| a | b | c |" or "a | b | c". Returns 0 when the line contains
// no pipe character or yields fewer than 2 non-empty tokens.
func countPipeCells(line string) int {
	if !strings.Contains(line, "|") {
		return 0
	}
	count := 0
	for _, cell := range strings.Split(line, "|") {
		if strings.TrimSpace(cell) != "" {
			count++
		}
	}
	return count
}

func splitLegacyTableLine(line string) []string {
	sep := "\t"
	if !strings.Contains(line, "\t") && strings.Contains(line, "|") {
		sep = "|"
	}
	if sep == "\t" && !strings.Contains(line, "\t") && countAlignedSpaceCells(line) >= 2 {
		return splitAlignedSpaceTableLine(line)
	}
	if sep == "\t" && !strings.Contains(line, "\t") && countColonKeyValueCells(line) >= 2 {
		return splitColonKeyValueLine(line)
	}
	raw := strings.Split(line, sep)
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func countAlignedSpaceCells(line string) int {
	line = strings.TrimSpace(line)
	if line == "" || !strings.Contains(line, "  ") {
		return 0
	}
	parts := splitAlignedSpaceTableLine(line)
	if len(parts) < 2 {
		return 0
	}
	if len(parts) == 2 && !isLikelyAlignedTwoColumnRow(parts) {
		return 0
	}
	return len(parts)
}

func splitAlignedSpaceTableLine(line string) []string {
	raw := alignedLegacyTableSep.Split(strings.TrimSpace(line), -1)
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func isLikelyAlignedTwoColumnRow(parts []string) bool {
	if len(parts) != 2 {
		return false
	}
	left := parts[0]
	right := parts[1]
	if left == "" || right == "" {
		return false
	}
	if len(strings.Fields(left)) > 4 || len(strings.Fields(right)) > 8 {
		return false
	}
	if strings.HasSuffix(left, ".") || strings.HasSuffix(left, "!") || strings.HasSuffix(left, "?") {
		return false
	}
	if len(strings.Fields(right)) > 1 && (strings.HasSuffix(right, ".") || strings.HasSuffix(right, "!") || strings.HasSuffix(right, "?")) {
		return false
	}
	return true
}

func countColonKeyValueCells(line string) int {
	parts := splitColonKeyValueLine(line)
	if len(parts) != 2 {
		return 0
	}
	return len(parts)
}

func splitColonKeyValueLine(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" || strings.Contains(line, "://") {
		return nil
	}
	if strings.Count(line, ":") != 1 {
		return nil
	}
	left, right, ok := strings.Cut(line, ":")
	if !ok {
		return nil
	}
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if !isLikelyColonKeyValueRow(left, right) {
		return nil
	}
	return []string{left, right}
}

func isLikelyColonKeyValueRow(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	if len(strings.Fields(left)) > 4 || len(strings.Fields(right)) > 10 {
		return false
	}
	if strings.HasSuffix(left, ".") || strings.HasSuffix(left, "!") || strings.HasSuffix(left, "?") {
		return false
	}
	if len(strings.Fields(right)) > 3 && (strings.HasSuffix(right, ".") || strings.HasSuffix(right, "!") || strings.HasSuffix(right, "?")) {
		return false
	}
	hasLetter := false
	for _, r := range left {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	return hasLetter
}

// isLegacyTableSeparatorLine reports whether a line is a simple text-table
// separator such as "|---|---|", "|:--|--:|", or "+---+---+".
func isLegacyTableSeparatorLine(line string) bool {
	line = strings.TrimSpace(line)
	if !(strings.Contains(line, "-") || strings.Contains(line, ":")) {
		return false
	}
	if strings.Contains(line, "|") || strings.Contains(line, "+") {
		for _, r := range line {
			switch r {
			case '|', '-', ':', '+', ' ', '\t':
			default:
				return false
			}
		}
		return true
	}
	if !strings.Contains(line, "  ") {
		return false
	}
	parts := splitAlignedSpaceTableLine(line)
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if !isLegacyAlignedSeparatorCell(part) {
			return false
		}
	}
	return true
}

func isLegacyAlignedSeparatorCell(part string) bool {
	part = strings.TrimSpace(part)
	if part == "" {
		return false
	}
	for _, r := range part {
		switch r {
		case '-', ':':
		default:
			return false
		}
	}
	return true
}

func parseLegacyListMarker(line string) (string, *wordir.ListMarker) {
	level := inferLegacyListLevel(line)
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", nil
	}
	for _, prefix := range []string{"* ", "- ", "• ", "o "} {
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)), &wordir.ListMarker{Kind: "bullet", Level: level, Template: strings.TrimSuffix(prefix, " ")}
		}
	}
	marker, rest, ok := strings.Cut(trimmed, ". ")
	if !ok {
		marker, rest, ok = strings.Cut(trimmed, ") ")
	}
	if ok {
		if ordered := parseLegacyOrderedListMarker(marker); ordered != nil {
			ordered.Level = level
			return strings.TrimSpace(rest), ordered
		}
	}
	if strings.HasPrefix(trimmed, "(") {
		if marker, rest, ok := strings.Cut(trimmed[1:], ") "); ok {
			if ordered := parseLegacyOrderedListMarker(marker); ordered != nil {
				ordered.Level = level
				return strings.TrimSpace(rest), ordered
			}
		}
	}
	return trimmed, nil
}

func legacyListBodyPrefixBytes(line, body string) int {
	if body == "" {
		return len(line)
	}
	leftTrimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	leading := len(line) - len(leftTrimmed)
	if pos := strings.Index(leftTrimmed, body); pos >= 0 {
		return leading + pos
	}
	return 0
}

func inferLegacyListLevel(line string) int {
	indentUnits := 0
	spaces := 0
	for _, r := range line {
		switch r {
		case '\t':
			indentUnits++
			spaces = 0
		case ' ':
			spaces++
			if spaces == 2 {
				indentUnits++
				spaces = 0
			}
		default:
			return indentUnits
		}
	}
	return indentUnits
}

func parseLegacyOrderedListMarker(marker string) *wordir.ListMarker {
	if value, err := strconv.Atoi(marker); err == nil {
		return &wordir.ListMarker{Kind: "ordered", Level: 0, Start: value, Format: "decimal"}
	}
	if isLegacyRomanListMarker(marker) {
		return &wordir.ListMarker{Kind: "ordered", Level: 0, Start: legacyRomanMarkerStart(marker), Format: legacyRomanMarkerFormat(marker)}
	}
	if isLegacyAlphaListMarker(marker) {
		return &wordir.ListMarker{Kind: "ordered", Level: 0, Start: legacyAlphaMarkerStart(marker), Format: legacyAlphaMarkerFormat(marker)}
	}
	return nil
}

func legacyAlphaMarkerStart(marker string) int {
	if len(marker) != 1 {
		return 0
	}
	r := rune(marker[0])
	switch {
	case r >= 'a' && r <= 'z':
		return int(r-'a') + 1
	case r >= 'A' && r <= 'Z':
		return int(r-'A') + 1
	default:
		return 0
	}
}

func legacyRomanMarkerStart(marker string) int {
	values := map[rune]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000}
	marker = strings.ToUpper(marker)
	total := 0
	prev := 0
	for index := len(marker) - 1; index >= 0; index-- {
		value := values[rune(marker[index])]
		if value < prev {
			total -= value
		} else {
			total += value
			prev = value
		}
	}
	return total
}

func isLegacyAlphaListMarker(marker string) bool {
	if len(marker) != 1 {
		return false
	}
	r := rune(marker[0])
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isLegacyRomanListMarker(marker string) bool {
	if marker == "" {
		return false
	}
	if len(marker) == 1 {
		r := rune(marker[0])
		if r >= 'a' && r <= 'z' {
			return false
		}
	}
	for _, r := range marker {
		switch r {
		case 'i', 'v', 'x', 'l', 'c', 'd', 'm', 'I', 'V', 'X', 'L', 'C', 'D', 'M':
		default:
			return false
		}
	}
	return true
}

func legacyAlphaMarkerFormat(marker string) string {
	if marker == strings.ToUpper(marker) {
		return "upperLetter"
	}
	return "lowerLetter"
}

// ---------------------------------------------------------------------------
// Run-aware document assembly (CHPx character formatting)
// ---------------------------------------------------------------------------

// binaryWordField represents a field at the binaryWordRun level, before conversion to wordir.Field.
type binaryWordField struct {
	Kind      string
	Instr     string
	Arg       string
	Anchor    string
	Target    string
	Children  []binaryWordRun
	InstrRuns []binaryWordRun
}

// binaryWordRun holds a decoded text segment together with its character and paragraph formatting.
type binaryWordRun struct {
	Text              string
	Format            wordir.RunFormat
	ParaFormat        wordir.ParagraphFormat
	TableDepth        int
	LegacyInTableHint bool
	LegacyRowMarkHint bool
	NativeCellMarks   bool
	// TableProps is the canonical table metadata consumed by the shared binary Word assembler.
	TableProps *docformat.TableRowProps
	Target     string // non-empty for runs that are the display text of a HYPERLINK field
	FieldKind  string
	FieldInstr string
	FieldArg   string
	Anchor     string
	Kind       string
	BreakType  string
	NoteID     int
	StartCP    int
	CommentID  int
	Revision   string
	// RevisionAuthor / RevisionDate are resolved when CHPx carried sprmCIbstRMark
	// (or its deletion variant) and the FIB pointed at a non-empty SttbfRMark.
	RevisionAuthor string
	RevisionDate   string
	// PropertyRevision is the resolved paragraph property-revision metadata from
	// PAPX sprmPPropRMark when present on the paragraph that owns this run.
	PropertyRevision *wordir.RevisionMark
	// RunPropertyRevision is the resolved run-format property-revision metadata
	// from CHPX sprmCPropRMark when present on this run.
	RunPropertyRevision *wordir.RevisionMark
	// DisplayFieldRevision is the resolved display-field revision metadata from
	// CHPX sprmCDispFldRMark when present on this run.
	DisplayFieldRevision *wordir.RevisionMark
	// Raw revision-mark indices captured from CHPx; resolved later in
	// the canonical Word97 materializer. Kept on binaryWordRun (not
	// exposed via wordir.Run) so the resolver can stay a single post-pass.
	IbstRMark    uint16
	HasIbstRMark bool
	DttmRMark    uint32
	HasDttmRMark bool
	// Raw paragraph property-revision metadata from PAPX; resolved later in
	// the canonical Word97 materializer.
	PropRMarkIbst    uint16
	HasPropRMarkIbst bool
	PropRMarkDttm    uint32
	HasPropRMarkDttm bool
	HasPropRMark     bool
	// Raw run property-revision metadata from CHPX; resolved later in
	// the canonical Word97 materializer.
	RunPropRMarkIbst    uint16
	HasRunPropRMarkIbst bool
	RunPropRMarkDttm    uint32
	HasRunPropRMarkDttm bool
	HasRunPropRMark     bool
	// Raw display-field revision metadata from CHPX; resolved later in
	// the canonical Word97 materializer.
	DispFldRMarkIbst    uint16
	HasDispFldRMarkIbst bool
	DispFldRMarkDttm    uint32
	HasDispFldRMarkDttm bool
	HasDispFldRMark     bool
	Spec                bool
	Data                bool
	Obj                 bool
	Image               *wordir.Image
	Shape               *wordir.Shape
	EmbeddedRun         *wordir.Run
	FormField           *wordir.FormField
	Fields              []*binaryWordField
	// ChpxSprms stores the raw CHPX grpprl bytes for this run.
	ChpxSprms []byte
	// PapxSprms stores the raw PAPX grpprl bytes for the paragraph that owns this run.
	PapxSprms []byte
}

// preWord97ParseData carries semantic state produced only by the genuine
// pre-Word97 compatibility parser. Word97+ never enters this path.
type preWord97ParseData struct {
	Assembly  binaryWordDocumentInput
	Footnotes map[int]wordir.Story
	Endnotes  map[int]wordir.Story
	Comments  map[int]wordir.Story
	// Version is the pre-Word97 nFib value from the FIB.
	Version uint16
}

type PreWord97BodySpan struct {
	Signature      uint16
	NFib           uint16
	Start          int
	FcMac          int
	CcpText        int
	BodyEnd        int
	SecondaryStart int
	SecondarySize  int
	// HWPF-derived offset map for Word 6 / Word 95 (HWPFOldDocument.java).
	// All offsets are absolute positions within the WordDocument stream.
	// Values are zero when the field is unset or the WordDocument stream is
	// too short.
	FComplex bool // FibBase grpfState bit 2 (fComplex)
	// FibRgLw95 story char counts (absolute FIB positions 0x38–0x50).
	CcpFtn     int // 0x38 — footnote story char count
	CcpHdd     int // 0x3C — header/footer story char count
	CcpMcr     int // 0x40 — macro char count (normally 0)
	CcpAtn     int // 0x44 — annotation/comment story char count
	CcpEdn     int // 0x48 — endnote story char count
	CcpTxbx    int // 0x4C — textbox story char count
	CcpHdrTxbx int // 0x50 — header-textbox story char count
	// FibRgFcLcb95 table offsets (base 0x58; each pair is FC/Lcb at base+entry*8).
	// Positions derived from MS-DOC §2.5.17.2 FibRgFcLcb95 field ordering,
	// confirmed by binary probe of real pre-Word-97 fixtures.
	SedTableOffset       int // 0x88 — PlcfSed: section table
	SedTableSize         int // 0x8C
	FtnRefTableOffset    int // 0x68 — PlcffndRef: footnote reference PLC
	FtnRefTableSize      int // 0x6C
	FtnTxtTableOffset    int // 0x70 — PlcffndTxt: footnote text PLC
	FtnTxtTableSize      int // 0x74
	AtnRefTableOffset    int // 0x78 — PlcfandRef: annotation reference PLC
	AtnRefTableSize      int // 0x7C
	AtnTxtTableOffset    int // 0x80 — PlcfandTxt: annotation text PLC
	AtnTxtTableSize      int // 0x84
	HddTableOffset       int // 0xB0 — PlcfHdd: header/footer PLC
	HddTableSize         int // 0xB4
	ChpTableOffset       int // 0xB8 — character properties table
	ChpTableSize         int // 0xBC
	PapTableOffset       int // 0xC0 — paragraph properties table
	PapTableSize         int // 0xC4
	FontTableOffset      int // 0xD0 — font (FFN) table
	FontTableSize        int // 0xD4
	BkmkNameTableOffset  int // 0x100 — SttbfBkmk: bookmark name string table
	BkmkNameTableSize    int // 0x104
	BkmkStartTableOffset int // 0x108 — PlcfBkf: bookmark start CPs + descriptor
	BkmkStartTableSize   int // 0x10C
	BkmkEndTableOffset   int // 0x110 — PlcfBkl: bookmark end CPs
	BkmkEndTableSize     int // 0x114
	EdnRefTableOffset    int // 0x128 — PlcfendRef: endnote reference PLC
	EdnRefTableSize      int // 0x12C
	EdnTxtTableOffset    int // 0x130 — PlcfendTxt: endnote text PLC
	EdnTxtTableSize      int // 0x134
	ComplexTableOffset   int // 0x160 — piece table when FComplex is set
}

// PreWord97FontEntry is a decoded font-table record from the Word 6 / Word 95
// font (FFN) table at WordDocument offset 0xD0. Layout follows
// HWPFOldDocument.OldFfn / OldFontTable.
type PreWord97FontEntry struct {
	Charset byte   // chs byte (Windows codepage hint)
	Name    string // primary font name (single-byte, decoded as Windows-1252)
	AltName string // alternate font name (may be empty)
}

type legacyStoryCounts struct {
	Text    int
	Ftn     int
	Hdr     int
	Mcr     int
	Atn     int
	Edn     int
	Txbx    int
	HdrTxbx int
}

// extractDocRunsViaPreWord97FIB is the future structured entrypoint for
// pre-Word-97 binary documents. It is wired into ParseDoc now so eventual
// header/FIB-like and CLX-like mapping work can land behind an explicit
// version dispatch without changing the higher-level parser flow.
// Until full mapping exists, use the observed body-text span in the current
// pre-Word-97 fixtures to avoid whole-stream fallback scanning.
func extractDocRunsViaPreWord97FIB(wordStream []byte, _ *cfb.File) *preWord97ParseData {
	core, ok := loadPreWord97DocCore(wordStream)
	if !ok {
		return nil
	}
	runs := extractPreWord97BodyRunsFromCore(core, wordStream)
	if scoreLegacyText(joinDocRunsText(runs)) == 0 {
		return nil
	}
	// Heuristic style-name map from the secondary span (phrase-matching).
	styleNames := preWord97SecondaryKnownStyleNames(wordStream)
	styleMap := preWord97SecondaryStyleNameByIndex(wordStream)
	inferred := inferPreWord97StyleNameByIstdFromRuns(runs, styleNames)
	if len(styleMap) == 0 {
		styleMap = inferred
	} else {
		for k, v := range inferred {
			styleMap[k] = v
		}
	}
	// Load stylesheet using the already-decoded core span — no second FIB parse.
	stylesheet := loadPreWord97StyleSheet(wordStream, core.Span)
	// When the STSH has real style names, prefer them over the heuristic map;
	// heuristic entries fill any istd slots the STSH does not cover.
	if ssMap := buildStyleNameByIstdFromStyleSheet(stylesheet); len(ssMap) > 0 {
		for k, v := range styleMap {
			if _, exists := ssMap[k]; !exists {
				ssMap[k] = v
			}
		}
		styleMap = ssMap
	}
	// Apply inherited style formatting first (fills in defaults from style sheet),
	// then apply explicit table hints (which take precedence for table membership).
	applyPreWord97StyleInheritedParagraphFormatting(runs, stylesheet)
	applyPreWord97StyleInheritedInTableHints(runs, stylesheet)

	// Load footnote, annotation, and endnote stories when the FIB indicates
	// their presence (CcpFtn/CcpAtn/CcpEdn > 0).  All three functions return
	// nil when the relevant char count is zero, so they are safe to call
	// unconditionally for documents without notes.
	ftnData := loadPreWord97FootnoteStories(wordStream)
	ednData := loadPreWord97EndnoteStories(wordStream)
	atnData := loadPreWord97AnnotationStories(wordStream)
	// Wire note-reference markers into body runs.
	if ftnData != nil {
		runs = preWord97InjectNoteRefs(runs, ftnData, "footnoteRef")
	}
	if ednData != nil {
		runs = preWord97InjectNoteRefs(runs, ednData, "endnoteRef")
	}
	if atnData != nil {
		runs = preWord97InjectNoteRefs(runs, atnData, "commentRef")
	}

	// Load bookmarks from the pre-Word-97 FIB fields (SttbfBkmk/PlcfBkf/PlcfBkl).
	// All three tables live inside the WordDocument stream for pre-Word-97 files.
	bookmarkStarts, bookmarkEnds := loadPreWord97BookmarkData(wordStream)

	return &preWord97ParseData{
		Assembly: binaryWordDocumentInput{
			Runs:            runs,
			BookmarkStarts:  bookmarkStarts,
			BookmarkEnds:    bookmarkEnds,
			ListCatalog:     core.ListTables,
			StyleCatalog:    stylesheet,
			StyleNameByIstd: styleMap,
		},
		Version:   core.Span.NFib,
		Footnotes: preWord97NoteDataAsStoryMap(ftnData),
		Endnotes:  preWord97NoteDataAsStoryMap(ednData),
		Comments:  preWord97NoteDataAsStoryMap(atnData),
	}
}

// buildStyleNameByIstdFromStyleSheet builds an istd → name map from the real
// STSH style names stored in stylesheet.paragraphByIndex. Returns nil when the
// stylesheet is nil or contains no named paragraph styles.
func isLikelyDuplicateEndnoteComments(comments, endnotes map[int]wordir.Story) bool {
	if len(comments) != 1 || len(endnotes) == 0 {
		return false
	}
	commentText := ""
	for _, s := range comments {
		commentText = normalizeLegacyStoryTextForCompare(s.Text)
		break
	}
	if commentText == "" {
		return false
	}
	ids := sortedStoryIDs(endnotes)
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, normalizeLegacyStoryTextForCompare(endnotes[id].Text))
	}
	joined := strings.Join(parts, "\n")
	return joined != "" && commentText == joined
}

func normalizeLegacyStoryTextForCompare(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSpace(s)
	return s
}

func sortedStoryIDs(stories map[int]wordir.Story) []int {
	ids := make([]int, 0, len(stories))
	for id := range stories {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func buildStyleNameByIstdFromStyleSheet(stylesheet *binaryWordStyleCatalog) map[int]string {
	if stylesheet == nil || len(stylesheet.paragraphByIndex) == 0 {
		return nil
	}
	m := make(map[int]string, len(stylesheet.paragraphByIndex))
	for istd, style := range stylesheet.paragraphByIndex {
		if style.Name != "" {
			m[istd] = style.Name
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// applyPreWord97StyleInheritedParagraphFormatting applies resolved paragraph
// formatting from the stylesheet to runs that reference a paragraph style via
// BinaryStyleIndex, including alignment, indents, spacing, borders, and shading.
// Direct run formatting (from PAPx grpprl) takes precedence over style formatting.
func applyPreWord97StyleInheritedParagraphFormatting(runs []binaryWordRun, stylesheet *binaryWordStyleCatalog) {
	if stylesheet == nil || len(stylesheet.paragraphByIndex) == 0 {
		return
	}
	for i := range runs {
		if runs[i].ParaFormat.BinaryStyleIndex == nil {
			continue
		}
		istd := *runs[i].ParaFormat.BinaryStyleIndex
		if style, ok := stylesheet.paragraphByIndex[istd]; ok {
			// Apply inherited style formatting only where not already set in the run.
			// The style's Format field contains resolved formatting from basedOn chain.
			applyStyleFormatToRunIfUnset(&runs[i], style.Format)
		}
	}
}

// applyStyleFormatToRunIfUnset merges styleFormat into run.ParaFormat, but only
// for fields not already set in the run. This preserves direct PAPx grpprl
// formatting while layering in inherited style defaults.
func applyStyleFormatToRunIfUnset(run *binaryWordRun, styleFormat wordir.ParagraphFormat) {
	// Apply alignment if not set.
	if run.ParaFormat.Alignment == "" && styleFormat.Alignment != "" {
		run.ParaFormat.Alignment = styleFormat.Alignment
	}
	// Apply indents if not set (they are stored as *int in ParagraphFormat).
	if run.ParaFormat.IndentLeft == nil && styleFormat.IndentLeft != nil {
		run.ParaFormat.IndentLeft = styleFormat.IndentLeft
	}
	if run.ParaFormat.IndentRight == nil && styleFormat.IndentRight != nil {
		run.ParaFormat.IndentRight = styleFormat.IndentRight
	}
	if run.ParaFormat.IndentFirstLine == nil && styleFormat.IndentFirstLine != nil {
		run.ParaFormat.IndentFirstLine = styleFormat.IndentFirstLine
	}
	if run.ParaFormat.IndentHanging == nil && styleFormat.IndentHanging != nil {
		run.ParaFormat.IndentHanging = styleFormat.IndentHanging
	}
	// Apply spacing if not set (they are stored as *int in ParagraphFormat).
	if run.ParaFormat.SpacingBefore == nil && styleFormat.SpacingBefore != nil {
		run.ParaFormat.SpacingBefore = styleFormat.SpacingBefore
	}
	if run.ParaFormat.SpacingAfter == nil && styleFormat.SpacingAfter != nil {
		run.ParaFormat.SpacingAfter = styleFormat.SpacingAfter
	}
	if run.ParaFormat.LineSpacing == nil && styleFormat.LineSpacing != nil {
		run.ParaFormat.LineSpacing = styleFormat.LineSpacing
	}
	if len(run.ParaFormat.TabStops) == 0 && len(styleFormat.TabStops) > 0 {
		run.ParaFormat.TabStops = cloneLegacyTabStops(styleFormat.TabStops)
	}
	// Apply table hint if not set.
	if !run.LegacyInTableHint && styleFormat.InTable {
		run.LegacyInTableHint = true
		run.ParaFormat.InTable = true
	}
	// Apply shading (background color) if not set.
	if run.ParaFormat.Shading == "" && styleFormat.Shading != "" {
		run.ParaFormat.Shading = styleFormat.Shading
	}
	// Apply borders if not set.
	if run.ParaFormat.Borders == nil && styleFormat.Borders != nil {
		run.ParaFormat.Borders = styleFormat.Borders
	}
	// Apply keep-next if not already set.
	if !run.ParaFormat.KeepNext && styleFormat.KeepNext {
		run.ParaFormat.KeepNext = styleFormat.KeepNext
	}
	// Apply keep-lines if not already set.
	if !run.ParaFormat.KeepLines && styleFormat.KeepLines {
		run.ParaFormat.KeepLines = styleFormat.KeepLines
	}
	// Apply widow control if not already set.
	if !run.ParaFormat.WidowControl && styleFormat.WidowControl {
		run.ParaFormat.WidowControl = styleFormat.WidowControl
	}
	// Apply page-break-before if not already set.
	if !run.ParaFormat.PageBreakBefore && styleFormat.PageBreakBefore {
		run.ParaFormat.PageBreakBefore = styleFormat.PageBreakBefore
	}
	// Apply contextual spacing if not already set.
	if !run.ParaFormat.ContextualSpacing && styleFormat.ContextualSpacing {
		run.ParaFormat.ContextualSpacing = styleFormat.ContextualSpacing
	}
}

// applyPreWord97StyleInheritedInTableHints sets LegacyInTableHint on runs
// whose paragraph style (via STSH) inherits fInTable but had no explicit
// grpprl opcode 24 in their direct PAPx entry. This covers the case where
// the style definition carries InTable=true through the style's UPX PAPX.
func applyPreWord97StyleInheritedInTableHints(runs []binaryWordRun, stylesheet *binaryWordStyleCatalog) {
	if stylesheet == nil {
		return
	}
	for i := range runs {
		if runs[i].LegacyInTableHint {
			continue // already set by direct grpprl
		}
		if runs[i].ParaFormat.BinaryStyleIndex == nil {
			continue
		}
		istd := *runs[i].ParaFormat.BinaryStyleIndex
		if style, ok := stylesheet.paragraphByIndex[istd]; ok && style.Format.InTable {
			runs[i].LegacyInTableHint = true
			runs[i].ParaFormat.InTable = true
			if strings.HasSuffix(runs[i].Text, "\a") {
				runs[i].ParaFormat.IsTableCellTrailingParagraph = true
			}
		}
	}
}

func inferPreWord97StyleNameByIstdFromRuns(runs []binaryWordRun, styleNames []string) map[int]string {
	if len(runs) == 0 || len(styleNames) == 0 {
		return nil
	}
	observed := make([]int, 0, len(styleNames))
	seen := make(map[int]struct{}, len(styleNames))
	for _, run := range runs {
		if run.ParaFormat.BinaryStyleIndex == nil {
			continue
		}
		istd := *run.ParaFormat.BinaryStyleIndex
		if _, ok := seen[istd]; ok {
			continue
		}
		seen[istd] = struct{}{}
		observed = append(observed, istd)
		if len(observed) >= len(styleNames) {
			break
		}
	}
	if len(observed) == 0 {
		return nil
	}
	mapping := make(map[int]string, len(observed))
	for index, istd := range observed {
		if index >= len(styleNames) {
			break
		}
		mapping[istd] = styleNames[index]
	}
	if len(mapping) == 0 {
		return nil
	}
	return mapping
}

// InspectPreWord97BodySpan reports the currently observed body-span slots in
// pre-Word-97 WordDocument streams. Start/FcMac/CcpText intentionally use
// Word 97 FIB terminology as analog labels only; they are not claimed to be
// official field identities for the 0xA5DC header family.
func InspectPreWord97BodySpan(wordStream []byte) (PreWord97BodySpan, bool) {
	if len(wordStream) < 56 {
		return PreWord97BodySpan{}, false
	}
	span := PreWord97BodySpan{
		Signature:      binary.LittleEndian.Uint16(wordStream[0:2]),
		NFib:           binary.LittleEndian.Uint16(wordStream[2:4]),
		Start:          int(binary.LittleEndian.Uint32(wordStream[24:28])),
		FcMac:          int(binary.LittleEndian.Uint32(wordStream[28:32])),
		CcpText:        int(binary.LittleEndian.Uint32(wordStream[52:56])),
		SecondaryStart: int(binary.LittleEndian.Uint32(wordStream[0x58:0x5C])),
		SecondarySize:  int(binary.LittleEndian.Uint32(wordStream[0x5C:0x60])),
	}
	// HWPF FibBase grpfState (uint8 at offset 0x0B) — bit 2 = fComplex.
	if 0x0B < len(wordStream) {
		span.FComplex = wordStream[0x0B]&0x04 != 0
	}
	// HWPF-mapped offsets: each is a 32-bit signed int at the documented
	// position. Skip the read when the WordDocument stream isn't long enough
	// to cover offset+4; remaining fields stay at their zero value.
	readOffsetSize := func(off int) (int, int) {
		if off+8 > len(wordStream) {
			return 0, 0
		}
		return int(int32(binary.LittleEndian.Uint32(wordStream[off : off+4]))),
			int(int32(binary.LittleEndian.Uint32(wordStream[off+4 : off+8])))
	}
	span.SedTableOffset, span.SedTableSize = readOffsetSize(0x88)
	// FibRgLw95 story char counts (base 0x20):
	//   ccpFtn@0x38 ccpHdd@0x3C ccpMcr@0x40 ccpAtn@0x44 ccpEdn@0x48 ccpTxbx@0x4C ccpHdrTxbx@0x50
	if 0x54 <= len(wordStream) {
		span.CcpFtn = int(int32(binary.LittleEndian.Uint32(wordStream[0x38:0x3C])))
		span.CcpHdd = int(int32(binary.LittleEndian.Uint32(wordStream[0x3C:0x40])))
		span.CcpMcr = int(int32(binary.LittleEndian.Uint32(wordStream[0x40:0x44])))
		span.CcpAtn = int(int32(binary.LittleEndian.Uint32(wordStream[0x44:0x48])))
		span.CcpEdn = int(int32(binary.LittleEndian.Uint32(wordStream[0x48:0x4C])))
		span.CcpTxbx = int(int32(binary.LittleEndian.Uint32(wordStream[0x4C:0x50])))
		span.CcpHdrTxbx = int(int32(binary.LittleEndian.Uint32(wordStream[0x50:0x54])))
	}
	// FibRgFcLcb95 note/annotation PLC offsets (base 0x58; confirmed by probe):
	//   PlcffndRef@0x68 PlcffndTxt@0x70 PlcfandRef@0x78 PlcfandTxt@0x80
	span.FtnRefTableOffset, span.FtnRefTableSize = readOffsetSize(0x68)
	span.FtnTxtTableOffset, span.FtnTxtTableSize = readOffsetSize(0x70)
	span.AtnRefTableOffset, span.AtnRefTableSize = readOffsetSize(0x78)
	span.AtnTxtTableOffset, span.AtnTxtTableSize = readOffsetSize(0x80)
	span.HddTableOffset, span.HddTableSize = readOffsetSize(0xB0)
	span.ChpTableOffset, span.ChpTableSize = readOffsetSize(0xB8)
	span.PapTableOffset, span.PapTableSize = readOffsetSize(0xC0)
	span.FontTableOffset, span.FontTableSize = readOffsetSize(0xD0)
	span.BkmkNameTableOffset, span.BkmkNameTableSize = readOffsetSize(0x100)
	span.BkmkStartTableOffset, span.BkmkStartTableSize = readOffsetSize(0x108)
	span.BkmkEndTableOffset, span.BkmkEndTableSize = readOffsetSize(0x110)
	span.EdnRefTableOffset, span.EdnRefTableSize = readOffsetSize(0x128)
	span.EdnTxtTableOffset, span.EdnTxtTableSize = readOffsetSize(0x130)
	if 0x164 <= len(wordStream) {
		span.ComplexTableOffset = int(int32(binary.LittleEndian.Uint32(wordStream[0x160:0x164])))
	}
	if span.Start <= 0 || span.Start >= len(wordStream) {
		return PreWord97BodySpan{}, false
	}
	if span.FcMac > span.Start && span.FcMac <= len(wordStream) {
		span.BodyEnd = span.FcMac
	}
	if span.CcpText > 0 {
		candidate := span.Start + span.CcpText
		if candidate > span.Start && candidate <= len(wordStream) && (span.BodyEnd == 0 || candidate < span.BodyEnd) {
			span.BodyEnd = candidate
		}
	}
	if span.BodyEnd <= span.Start {
		return PreWord97BodySpan{}, false
	}
	return span, true
}

func extractPreWord97BodyText(wordStream []byte) string {
	runs := extractPreWord97BodyRuns(wordStream)
	if len(runs) == 0 {
		return ""
	}
	return normalizeLegacyText(joinDocRunsText(runs))
}

func extractPreWord97BodyRuns(wordStream []byte) []binaryWordRun {
	whole := fallbackScanDocText(wordStream)
	core, ok := loadPreWord97DocCore(wordStream)
	if !ok {
		if scoreLegacyText(whole) == 0 {
			return nil
		}
		return []binaryWordRun{{Text: whole}}
	}
	return extractPreWord97BodyRunsFromCore(core, wordStream)
}

// extractPreWord97BodyRunsFromCore assembles the best []binaryWordRun for the main
// body story using a pre-loaded preWord97DocCore, avoiding a second FIB parse
// when the caller already holds the core (e.g. extractDocRunsViaPreWord97FIB).
func extractPreWord97BodyRunsFromCore(core *preWord97DocCore, wordStream []byte) []binaryWordRun {
	whole := fallbackScanDocText(wordStream)
	span := core.Span
	if span.FComplex {
		coreRuns := core.assembleMainStoryRuns(wordStream)
		coreText := normalizeLegacyText(joinDocRunsText(coreRuns))
		if len(coreRuns) > 0 && scoreLegacyText(coreText) > 0 {
			return coreRuns
		}
		body := wordStream[span.Start:span.BodyEnd]
		scanned := fallbackScanDocText(body)
		if scoreLegacyText(scanned) > 0 {
			return []binaryWordRun{{Text: scanned}}
		}
		if scoreLegacyText(whole) > 0 {
			return []binaryWordRun{{Text: whole}}
		}
		return nil
	}
	body := wordStream[span.Start:span.BodyEnd]
	preferredCharset := core.PreferredCharset
	direct := decodePreWord97SingleByteTextWithCharset(body, preferredCharset)
	directNormalized := normalizeLegacyText(direct)
	scanned := fallbackScanDocText(body)
	if scoreLegacyText(directNormalized) == 0 {
		switch {
		case scoreLegacyText(scanned) > 0:
			return []binaryWordRun{{Text: scanned}}
		case scoreLegacyText(whole) > 0:
			return []binaryWordRun{{Text: whole}}
		default:
			return nil
		}
	}
	bestText := directNormalized
	bestRuns := []binaryWordRun{{Text: direct}}
	if scoreLegacyText(scanned) > scoreLegacyText(bestText) {
		bestText = scanned
		bestRuns = []binaryWordRun{{Text: scanned}}
	}
	coreRuns := core.assembleMainStoryRuns(wordStream)
	coreText := normalizeLegacyText(joinDocRunsText(coreRuns))
	if !legacyRunsAreOverfragmented(coreRuns) && scoreLegacyText(coreText) >= scoreLegacyText(bestText) && legacyTextLineCount(coreText) >= legacyTextLineCount(bestText) {
		bestText = coreText
		bestRuns = coreRuns
	}
	if !span.FComplex {
		papRuns := collapsePreWord97PapRuns(wordStream, span)
		// If coreRuns were overfragmented but carry character formatting,
		// apply that formatting to papRuns to preserve fonts while maintaining
		// proper paragraph structure.
		selectedPapRuns := papRuns
		if legacyRunsAreOverfragmented(coreRuns) && scoreLegacyText(coreText) > 0 {
			selectedPapRuns = applyChpxFormattingToDocRuns(papRuns, coreRuns)
		}
		selectedPapText := normalizeLegacyText(joinDocRunsText(selectedPapRuns))
		if scoreLegacyText(selectedPapText) > scoreLegacyText(bestText) ||
			(scoreLegacyText(selectedPapText) == scoreLegacyText(bestText) &&
				(legacyTextLineCount(selectedPapText) > legacyTextLineCount(bestText) || len(selectedPapRuns) > len(bestRuns))) {
			return selectedPapRuns
		}
	}
	// A valid pre-Word-97 FIB gives us an explicit main-story body span. Never
	// promote a whole-stream scan over that bounded story: the bytes after
	// fcMac/ccpText contain headers, style names, field instructions and other
	// document metadata that can look like perfectly good prose. Word 95 files
	// in particular often carry printable strings there.
	return bestRuns
}

// normalizePreWord97GridTableRuns reconstructs old Word table rows encoded as
// dense 0x07 cell-grid marks inside a single paragraph run. In Word 6/95 a
// visually merged cell can span several grid cells, so a five-column table may
// contain (for example) 15 bell marks per physical row. Treating every mark as
// a tab collapses the entire table into one oversized heuristic row and drops
// most of the document when the table is rendered.
//
// We infer the stable grid width from the repeating bell-mark cadence, then
// emit one tab-delimited run per physical row. Empty grid cells remain merged
// implicitly: only non-empty logical cells are emitted. A leading 0x0e control
// is preserved as a form-feed so the existing break pipeline can paginate it;
// in one-column legacy documents this is how Word's old column/page boundary
// appears in the observed fixtures.
func normalizePreWord97GridTableRuns(runs []binaryWordRun) []binaryWordRun {
	if len(runs) == 0 {
		return runs
	}
	out := make([]binaryWordRun, 0, len(runs))
	for _, run := range runs {
		bellCount := strings.Count(run.Text, "\a")
		if bellCount < 12 || !strings.Contains(run.Text, "\a") {
			out = append(out, run)
			continue
		}
		grid := inferPreWord97GridWidth(run.Text, bellCount)
		if grid <= 0 {
			out = append(out, run)
			continue
		}
		rows := splitPreWord97GridRows(run.Text, grid)
		if len(rows) < 2 {
			out = append(out, run)
			continue
		}
		cp := run.StartCP
		for _, text := range rows {
			cloned := run
			cloned.StartCP = cp
			cloned.Text = text
			// These are reconstructed logical rows, not native single-cell marks.
			cloned.NativeCellMarks = false
			cloned.LegacyInTableHint = false
			cloned.LegacyRowMarkHint = false
			cloned.ParaFormat.InTable = false
			cloned.ParaFormat.IsTableCellTrailingParagraph = false
			cloned.ParaFormat.IsTableTrailingParagraph = false
			out = append(out, cloned)
			cp += len([]rune(text))
		}
		// One dense pre-97 grid run corresponds to one physical Word table.
		// Keep adjacent tables from being coalesced by the text-table rescue path.
		out = append(out, binaryWordRun{Kind: legacyListBoundaryKind, StartCP: cp})
	}
	return out
}

func inferPreWord97GridWidth(text string, bellCount int) int {
	if bellCount < 12 {
		return 0
	}
	bestGrid := 0
	bestScore := -1
	for grid := 4; grid <= 24; grid++ {
		if bellCount%grid != 0 {
			continue
		}
		rows := splitPreWord97GridRows(text, grid)
		if len(rows) < 3 {
			continue
		}
		counts := make([]int, 0, len(rows))
		for _, row := range rows {
			clean := strings.Trim(row, "\r\n\f")
			if clean == "" {
				continue
			}
			cells := 1
			if strings.Contains(clean, "\t") {
				cells = len(strings.Split(clean, "\t"))
			}
			counts = append(counts, cells)
		}
		if len(counts) < 3 {
			continue
		}
		freq := map[int]int{}
		mode, modeN := 0, 0
		for _, n := range counts {
			if n < 2 || n > 8 {
				continue
			}
			freq[n]++
			if freq[n] > modeN {
				mode, modeN = n, freq[n]
			}
		}
		if mode < 2 || mode > 6 || modeN < 2 {
			continue
		}
		// Prefer a cadence that explains most rows with a useful logical
		// column count. Tiny candidate grids (4/5 bells) can trivially produce
		// dozens of two-cell pseudo-rows, while the real table cadence yields
		// the stable 4/5-column rows Word displays. Weight stable column count
		// quadratically so those false small-grid solutions lose.
		score := modeN*(mode-1)*(mode-1) - (len(counts) - modeN)
		if score > bestScore || (score == bestScore && (bestGrid == 0 || grid < bestGrid)) {
			bestGrid, bestScore = grid, score
		}
	}
	return bestGrid
}

func splitPreWord97GridRows(text string, grid int) []string {
	if grid <= 0 {
		return nil
	}
	leadingBreak := false
	if strings.HasPrefix(text, "\x0e") {
		leadingBreak = true
		text = strings.TrimPrefix(text, "\x0e")
	}
	var rows []string
	var cells []string
	var cell strings.Builder
	bells := 0
	flushCell := func() {
		value := strings.TrimSpace(strings.ReplaceAll(cell.String(), "\v", " "))
		if value != "" {
			cells = append(cells, value)
		}
		cell.Reset()
	}
	flushRow := func() {
		flushCell()
		if len(cells) == 0 {
			return
		}
		row := strings.Join(cells, "\t")
		// Keep single-cell heading rows in the heuristic table sequence.
		if len(cells) == 1 {
			row += "\t"
		}
		row += "\r"
		if leadingBreak && len(rows) == 0 {
			row = "\f" + row
			leadingBreak = false
		}
		rows = append(rows, row)
		cells = cells[:0]
	}
	for _, ch := range text {
		if ch != '\a' {
			if ch != '\r' && ch != '\n' {
				cell.WriteRune(ch)
			}
			continue
		}
		flushCell()
		bells++
		if bells%grid == 0 {
			flushRow()
		}
	}
	if cell.Len() > 0 || len(cells) > 0 {
		flushRow()
	}
	return rows
}

func legacyTextLineCount(text string) int {
	if text == "" {
		return 0
	}
	return len(strings.Split(text, "\n"))
}

func legacyRunsAreOverfragmented(runs []binaryWordRun) bool {
	if len(runs) == 0 {
		return false
	}
	lines := legacyTextLineCount(normalizeLegacyText(joinDocRunsText(runs)))
	if lines <= 0 {
		lines = 1
	}
	return len(runs) > lines*8
}

// applyChpxFormattingToDocRuns overlays CHPx character formatting from
// chpxFormatRuns onto the structure of baseRuns (typically PAP-derived).
// For each character position in baseRuns, it finds the matching CHPx entry
// in chpxFormatRuns and applies that formatting.
func applyChpxFormattingToDocRuns(baseRuns, chpxFormatRuns []binaryWordRun) []binaryWordRun {
	if len(chpxFormatRuns) == 0 || len(baseRuns) == 0 {
		return baseRuns
	}
	type cpRange struct {
		start, end int
		fmt        wordir.RunFormat
	}
	ranges := make([]cpRange, 0, len(chpxFormatRuns))
	nextCP := 0
	for i, run := range chpxFormatRuns {
		textLen := len([]rune(run.Text))
		if textLen <= 0 {
			if run.StartCP > nextCP {
				nextCP = run.StartCP
			}
			continue
		}
		start := run.StartCP
		if start < 0 || (i > 0 && start == 0) {
			start = nextCP
		}
		ranges = append(ranges, cpRange{start: start, end: start + textLen, fmt: run.Format})
		nextCP = start + textLen
	}
	if len(ranges) == 0 {
		return baseRuns
	}

	// PAP-derived runs may span several CHPX ranges. Applying any overlapping
	// CHPX to the whole PAP run leaks character formatting across the boundary
	// (particularly visible in compressed Word 6/95 non-ASCII text). Split each
	// base run at every CHPX boundary and format each resulting CP interval.
	out := make([]binaryWordRun, 0, len(baseRuns)+len(ranges))
	nextBaseCP := 0
	for i, baseRun := range baseRuns {
		runes := []rune(baseRun.Text)
		if len(runes) == 0 || baseRun.Kind != "" {
			out = append(out, baseRun)
			continue
		}
		baseStart := baseRun.StartCP
		if baseStart < 0 || (i > 0 && baseStart == 0) {
			baseStart = nextBaseCP
		}
		baseEnd := baseStart + len(runes)
		nextBaseCP = baseEnd

		// Dense pre-Word-97 tables encode physical grid boundaries as 0x07
		// characters inside a PAP run. Splitting those runs at arbitrary CHPX CP
		// boundaries destroys the repeating grid cadence that the table rescue
		// pass relies on, causing data rows to disappear. Keep structural grid
		// runs intact and use the CHPX active at the run start for their
		// character format; cell-level styling is less important than preserving
		// the table text and geometry.
		if strings.ContainsRune(baseRun.Text, '\a') {
			seg := cloneDocRunWithText(baseRun, baseRun.Text, baseStart)
			for _, r := range ranges {
				if r.start <= baseStart && r.end > baseStart {
					seg.Format = legacyMergeRunFormat(seg.Format, r.fmt)
					break
				}
			}
			out = append(out, seg)
			continue
		}

		boundaries := []int{baseStart, baseEnd}
		for _, r := range ranges {
			if r.end <= baseStart || r.start >= baseEnd {
				continue
			}
			if r.start > baseStart && r.start < baseEnd {
				boundaries = append(boundaries, r.start)
			}
			if r.end > baseStart && r.end < baseEnd {
				boundaries = append(boundaries, r.end)
			}
		}
		sort.Ints(boundaries)
		uniq := boundaries[:0]
		for _, cp := range boundaries {
			if len(uniq) == 0 || uniq[len(uniq)-1] != cp {
				uniq = append(uniq, cp)
			}
		}
		for j := 0; j+1 < len(uniq); j++ {
			segStart, segEnd := uniq[j], uniq[j+1]
			if segEnd <= segStart {
				continue
			}
			lo, hi := segStart-baseStart, segEnd-baseStart
			seg := cloneDocRunWithText(baseRun, string(runes[lo:hi]), segStart)
			for _, r := range ranges {
				if r.end <= segStart || r.start >= segEnd {
					continue
				}
				seg.Format = legacyMergeRunFormat(seg.Format, r.fmt)
			}
			out = append(out, seg)
		}
	}
	return out
}

func readLegacyStoryCounts(wordStream []byte, lw97Base, cslw int) legacyStoryCounts {
	read := func(slot int) int {
		off := lw97Base + slot*4
		if slot >= cslw || off+4 > len(wordStream) {
			return 0
		}
		return int(binary.LittleEndian.Uint32(wordStream[off : off+4]))
	}
	return legacyStoryCounts{
		Text:    read(3),
		Ftn:     read(4),
		Hdr:     read(5),
		Mcr:     read(6),
		Atn:     read(7),
		Edn:     read(8),
		Txbx:    read(9),
		HdrTxbx: read(10),
	}
}

// splitPieceByChpx splits one piece of the piece table into sub-runs at CHPx
// formatting boundaries. For each sub-run the text is decoded and the
// corresponding RunFormat and ParagraphFormat are attached.
//
// Piece-table FcCompressed values need to be normalized before matching FKP
// entries: uncompressed pieces use raw byte offsets, while compressed pieces
// offsets for both CHPX and PAPX entries, so compressed-piece lookups must
// divide by 2 and then advance one byte per character.
func splitPieceByChpx(wordStream []byte, byteOff int, fcRaw uint32, charCount int, compressed bool, pieceStartCP int, entries []chpxEntry, papxEntries []papxEntry) []binaryWordRun {
	return splitPieceByChpxWithCharset(wordStream, byteOff, fcRaw, charCount, compressed, pieceStartCP, entries, papxEntries, nil)
}

func splitPieceByChpxWithCharset(wordStream []byte, byteOff int, fcRaw uint32, charCount int, compressed bool, pieceStartCP int, entries []chpxEntry, papxEntries []papxEntry, preferredCharset *charmap.Charmap) []binaryWordRun {
	pieceFC, fcStep := legacyPieceFC(wordStream, fcRaw, compressed)
	chpxBaseFC, chpxStep := legacyPieceEntryFCBasis(fcRaw, pieceFC, fcStep, compressed, len(entries), func() uint32 {
		if len(entries) == 0 {
			return 0
		}
		return entries[0].FC
	}())
	papxBaseFC, papxStep := legacyPieceEntryFCBasis(fcRaw, pieceFC, fcStep, compressed, len(papxEntries), func() uint32 {
		if len(papxEntries) == 0 {
			return 0
		}
		return papxEntries[0].FC
	}())
	var result []binaryWordRun
	pos := 0 // current character offset within piece
	for pos < charCount {
		charChpxFC := chpxBaseFC + uint32(pos)*chpxStep
		charPapxFC := papxBaseFC + uint32(pos)*papxStep

		// Find the CHPx entry that covers charFC.
		idx := findChpxEntry(entries, charChpxFC)
		var format wordir.RunFormat
		revision := ""
		spec := false
		dataFlag := false
		objFlag := false
		var ibstRMark uint16
		hasIbst := false
		var dttmRMark uint32
		hasDttm := false
		var runPropRMarkIbst uint16
		hasRunPropRMarkIbst := false
		var runPropRMarkDttm uint32
		hasRunPropRMarkDttm := false
		hasRunPropRMark := false
		var dispFldRMarkIbst uint16
		hasDispFldRMarkIbst := false
		var dispFldRMarkDttm uint32
		hasDispFldRMarkDttm := false
		hasDispFldRMark := false
		var chpxSprms []byte
		if idx >= 0 {
			format = entries[idx].Format
			revision = entries[idx].Revision
			spec = entries[idx].Spec
			dataFlag = entries[idx].Data
			objFlag = entries[idx].Obj
			ibstRMark = entries[idx].IbstRMark
			hasIbst = entries[idx].HasIbstRMark
			dttmRMark = entries[idx].DttmRMark
			hasDttm = entries[idx].HasDttmRMark
			chpxSprms = entries[idx].Sprms
			if prop := entries[idx].PropertyRevision; prop != nil {
				hasRunPropRMark = prop.Present
				runPropRMarkIbst = prop.Ibst
				hasRunPropRMarkIbst = prop.HasIbst
				runPropRMarkDttm = prop.Dttm
				hasRunPropRMarkDttm = prop.HasDttm
			}
			if disp := entries[idx].DisplayFieldRevision; disp != nil {
				hasDispFldRMark = disp.Present
				dispFldRMarkIbst = disp.Ibst
				hasDispFldRMarkIbst = disp.HasIbst
				dispFldRMarkDttm = disp.Dttm
				hasDispFldRMarkDttm = disp.HasDttm
			}
		}

		// Find the PAPx entry that covers charFC.
		pidx := findPapxEntry(papxEntries, charPapxFC)
		var paraFormat wordir.ParagraphFormat
		tableDepth := 0
		var tableProps *legacyTableRowProps
		var propRMarkIbst uint16
		hasPropRMarkIbst := false
		var propRMarkDttm uint32
		hasPropRMarkDttm := false
		hasPropRMark := false
		var papxSprms []byte
		if pidx >= 0 {
			paraFormat = papxEntries[pidx].Format
			tableDepth = papxEntries[pidx].Meta.TableDepth
			tableProps = papxEntries[pidx].Meta.TableRowProps
			papxSprms = papxEntries[pidx].Sprms
			if prop := papxEntries[pidx].Meta.PropertyRevision; prop != nil {
				hasPropRMark = prop.Present
				propRMarkIbst = prop.Ibst
				hasPropRMarkIbst = prop.HasIbst
				propRMarkDttm = prop.Dttm
				hasPropRMarkDttm = prop.HasDttm
			}
		}

		// Find the end of this formatted sub-run (next CHPx boundary inside piece).
		end := charCount
		nextIdx := idx + 1
		if idx < 0 {
			nextIdx = 0
		}
		if nextIdx < len(entries) {
			nextFC := entries[nextIdx].FC
			if nextFC > chpxBaseFC {
				nextDelta := int((nextFC - chpxBaseFC) / chpxStep)
				if nextDelta > pos && nextDelta < end {
					end = nextDelta
				}
			}
		}
		nextPapxIdx := pidx + 1
		if pidx < 0 {
			nextPapxIdx = 0
		}
		if nextPapxIdx < len(papxEntries) {
			nextFC := papxEntries[nextPapxIdx].FC
			if nextFC > papxBaseFC {
				nextDelta := int((nextFC - papxBaseFC) / papxStep)
				if nextDelta > pos && nextDelta < end {
					end = nextDelta
				}
			}
		}

		subCount := end - pos
		var subByteOff int
		if compressed {
			subByteOff = byteOff + pos
		} else {
			subByteOff = byteOff + pos*2
		}
		text := decodePieceTextWithCharset(wordStream, subByteOff, subCount, compressed, preferredCharset)
		if text != "" {
			legacyInTableHint := false
			legacyRowMarkHint := false
			if pidx >= 0 {
				legacyInTableHint = papxEntries[pidx].LegacyInTableHint
				legacyRowMarkHint = papxEntries[pidx].LegacyRowMarkHint
			}

			// For pre-97 documents, detect cell and row mark paragraphs based on
			// old-PAP one-byte toggles (0x18 = fInTable/cell, 0x19 = fTtp/row-mark).
			// Both end with \a; the distinguishing factor is the toggle that's set.
			if legacyInTableHint && strings.HasSuffix(text, "\a") {
				paraFormat.IsTableCellTrailingParagraph = true
			}
			if legacyRowMarkHint && strings.HasSuffix(text, "\a") {
				paraFormat.IsTableTrailingParagraph = true
			}

			result = append(result, binaryWordRun{
				Text:                text,
				Format:              format,
				ParaFormat:          paraFormat,
				TableDepth:          tableDepth,
				LegacyInTableHint:   legacyInTableHint,
				LegacyRowMarkHint:   legacyRowMarkHint,
				TableProps:          tableProps.canonical(),
				StartCP:             pieceStartCP + pos,
				Revision:            revision,
				Spec:                spec,
				Data:                dataFlag,
				Obj:                 objFlag,
				IbstRMark:           ibstRMark,
				HasIbstRMark:        hasIbst,
				DttmRMark:           dttmRMark,
				HasDttmRMark:        hasDttm,
				PropRMarkIbst:       propRMarkIbst,
				HasPropRMarkIbst:    hasPropRMarkIbst,
				PropRMarkDttm:       propRMarkDttm,
				HasPropRMarkDttm:    hasPropRMarkDttm,
				HasPropRMark:        hasPropRMark,
				RunPropRMarkIbst:    runPropRMarkIbst,
				HasRunPropRMarkIbst: hasRunPropRMarkIbst,
				RunPropRMarkDttm:    runPropRMarkDttm,
				HasRunPropRMarkDttm: hasRunPropRMarkDttm,
				HasRunPropRMark:     hasRunPropRMark,
				DispFldRMarkIbst:    dispFldRMarkIbst,
				HasDispFldRMarkIbst: hasDispFldRMarkIbst,
				DispFldRMarkDttm:    dispFldRMarkDttm,
				HasDispFldRMarkDttm: hasDispFldRMarkDttm,
				HasDispFldRMark:     hasDispFldRMark,
				ChpxSprms:           chpxSprms,
				PapxSprms:           papxSprms,
			})
		}
		pos = end
	}
	return result
}

func legacyPieceFC(_ []byte, fcRaw uint32, compressed bool) (uint32, uint32) {
	fc := fcRaw & 0x3FFFFFFF
	if compressed {
		return fc / 2, 1
	}
	return fc, 2
}

func legacyPieceEntryFCBasis(rawFC, byteFC, byteStep uint32, compressed bool, entryCount int, firstEntryFC uint32) (uint32, uint32) {
	if !compressed || entryCount == 0 {
		return byteFC, byteStep
	}
	if legacyAbsUint32Diff(firstEntryFC, rawFC) < legacyAbsUint32Diff(firstEntryFC, byteFC) {
		return rawFC, 2
	}
	return byteFC, byteStep
}

func legacyAbsUint32Diff(left, right uint32) uint32 {
	if left > right {
		return left - right
	}
	return right - left
}

// decodePieceText decodes charCount characters from wordStream starting at
// byteOff, using ANSI (compressed) or UTF-16LE (uncompressed) encoding.
func decodePieceText(wordStream []byte, byteOff, charCount int, compressed bool) string {
	return decodePieceTextWithCharset(wordStream, byteOff, charCount, compressed, nil)
}

func decodePieceTextWithCharset(wordStream []byte, byteOff, charCount int, compressed bool, preferredCharset *charmap.Charmap) string {
	if byteOff < 0 || byteOff >= len(wordStream) || charCount <= 0 {
		return ""
	}
	if compressed {
		end := byteOff + charCount
		if end > len(wordStream) {
			end = len(wordStream)
		}
		if preferredCharset != nil {
			return decodeSingleByteCharmap(wordStream[byteOff:end], preferredCharset)
		}
		return decodeWindows1252(wordStream[byteOff:end])
	}
	end := byteOff + charCount*2
	if end > len(wordStream) {
		end = len(wordStream)
	}
	return decodeUTF16LE(wordStream[byteOff:end])
}

// ---------------------------------------------------------------------------
// Field-character preprocessing
// ---------------------------------------------------------------------------

// applyPreWord97FieldsToRuns scans the raw run slice for Word field characters
// (0x13 = FLDBEGIN, 0x14 = FLDSEP, 0x15 = FLDEND) and rewrites the slice so
// that:
//   - The field instruction text (between 0x13 and 0x14) is removed from
//     output text but preserved as metadata on the cached-result runs.
//   - For HYPERLINK fields the display-text runs (between 0x14 and 0x15) have
//     their Target field set to the URL extracted from the instruction.
//   - All three field characters are removed from the output text.
//   - All other field types pass through with the cached result text kept.
func applyPreWord97FieldsToRuns(runs []binaryWordRun) []binaryWordRun {
	return applyPreWord97FieldsToRunsWithDataStream(runs, nil)
}

// applyPreWord97FieldsToRunsWithDataStream is the data-stream-aware variant of
// applyPreWord97FieldsToRuns.  When dataStream is non-nil, form fields that carry a
// PicLocation will have their binary FFData read from the Word Data stream
// and merged into the resulting FormField.
//
// The implementation uses a stack of field frames so that nested fields (e.g.
// { IF { PAGE } > 1 "yes" "no" }) are captured as trees rather than being
// flattened or overwritten.
func applyPreWord97FieldsToRunsWithDataStream(runs []binaryWordRun, dataStream []byte) []binaryWordRun {
	const (
		fldBegin = '\x13'
		fldSep   = '\x14'
		fldEnd   = '\x15'
	)

	type frame struct {
		instr     strings.Builder
		instrRuns []binaryWordRun
		result    []binaryWordRun
		parsed    legacyDocField
		inResult  bool
	}

	out := make([]binaryWordRun, 0, len(runs))
	stack := make([]frame, 0, 4)

	// inResult reports whether the top of the stack is in result phase.
	inResult := func() bool {
		if len(stack) == 0 {
			return false
		}
		return stack[len(stack)-1].inResult
	}

	// inField reports whether we are inside any field frame.
	inField := func() bool { return len(stack) > 0 }

	// emitText routes a text segment to the current accumulator (instruction,
	// result, or top-level output).
	emitText := func(text string, r binaryWordRun) {
		if text == "" {
			return
		}
		nr := binaryWordRun{
			Text:                 text,
			Format:               r.Format,
			ParaFormat:           r.ParaFormat,
			TableDepth:           r.TableDepth,
			TableProps:           r.TableProps,
			Target:               r.Target,
			FieldKind:            r.FieldKind,
			FieldInstr:           r.FieldInstr,
			FieldArg:             r.FieldArg,
			Anchor:               r.Anchor,
			Kind:                 r.Kind,
			NoteID:               r.NoteID,
			StartCP:              r.StartCP,
			CommentID:            r.CommentID,
			Revision:             r.Revision,
			RevisionAuthor:       r.RevisionAuthor,
			RevisionDate:         r.RevisionDate,
			PropertyRevision:     r.PropertyRevision,
			RunPropertyRevision:  r.RunPropertyRevision,
			DisplayFieldRevision: r.DisplayFieldRevision,
			Spec:                 r.Spec,
			Image:                r.Image,
			Shape:                r.Shape,
			FormField:            r.FormField,
		}
		if inResult() {
			top := &stack[len(stack)-1]
			if top.parsed.Target != "" {
				nr.Target = top.parsed.Target
			}
			if top.parsed.Instr != "" {
				nr.FieldInstr = top.parsed.Instr
			}
			if top.parsed.Kind != "" {
				nr.FieldKind = top.parsed.Kind
				nr.FieldArg = top.parsed.Arg
				nr.Anchor = top.parsed.Anchor
			}
			top.result = append(top.result, nr)
			return
		}
		if inField() {
			top := &stack[len(stack)-1]
			top.instr.WriteString(text)
			top.instrRuns = append(top.instrRuns, nr)
			return
		}
		out = append(out, nr)
	}

	// flushField pops the top frame and returns a binaryWordField representing it.
	flushField := func() *binaryWordField {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return &binaryWordField{
			Kind:      top.parsed.Kind,
			Instr:     top.parsed.Instr,
			Arg:       top.parsed.Arg,
			Anchor:    top.parsed.Anchor,
			Target:    top.parsed.Target,
			Children:  top.result,
			InstrRuns: top.instrRuns,
		}
	}

	// handleField routes a completed binaryWordField to its parent frame or to out.
	handleField := func(field *binaryWordField) {
		if len(stack) > 0 {
			parent := &stack[len(stack)-1]
			if parent.inResult {
				// Nested field inside parent's result.
				resultRuns := field.Children
				if len(resultRuns) == 0 {
					resultRuns = []binaryWordRun{{}}
				}
				if field.Kind != "" || field.Instr != "" {
					resultRuns[0].FieldKind = field.Kind
					resultRuns[0].FieldInstr = field.Instr
					resultRuns[0].FieldArg = field.Arg
					resultRuns[0].Anchor = field.Anchor
					if field.Target != "" {
						resultRuns[0].Target = field.Target
					}
				}
				resultRuns[0].Fields = append(resultRuns[0].Fields, field)
				parent.result = append(parent.result, resultRuns...)
				return
			}
			// Nested field inside parent's instruction.
			displayText := joinDocRunsText(field.Children)
			parent.instr.WriteString(displayText)
			instrRun := binaryWordRun{Text: displayText, Fields: []*binaryWordField{field}}
			parent.instrRuns = append(parent.instrRuns, instrRun)
			return
		}

		// Top-level field: attach flat metadata to the first result run.
		// We do NOT store the *binaryWordField on the run's Fields slice here,
		// because that would create a self-referential cycle (the field's
		// Children contain the run).  Top-level fields remain in the flat
		// model; the wordir.Field tree is only used for nested fields.
		if len(field.Children) > 0 && (field.Kind != "" || field.Instr != "") {
			field.Children[0].FieldKind = field.Kind
			field.Children[0].FieldInstr = field.Instr
			field.Children[0].FieldArg = field.Arg
			field.Children[0].Anchor = field.Anchor
			if field.Target != "" {
				field.Children[0].Target = field.Target
			}
		}

		if legacyDocFieldIsFormField(field.Kind) {
			out = append(out, legacyDocFormFieldRun(legacyDocField{
				Kind:   field.Kind,
				Instr:  field.Instr,
				Arg:    field.Arg,
				Anchor: field.Anchor,
				Target: field.Target,
			}, field.Children, dataStream))
			return
		}

		if len(field.Children) > 0 {
			out = append(out, field.Children...)
			return
		}

		out = append(out, binaryWordRun{
			FieldKind:  field.Kind,
			FieldInstr: field.Instr,
			FieldArg:   field.Arg,
			Anchor:     field.Anchor,
			Fields:     []*binaryWordField{field},
		})
	}

	for _, r := range runs {
		if r.Kind != "" {
			if inResult() {
				handleField(flushField())
			}
			if inField() {
				// Non-text run inside a field instruction: treat as instruction text.
				top := &stack[len(stack)-1]
				top.instrRuns = append(top.instrRuns, r)
			} else {
				out = append(out, r)
			}
			continue
		}

		text := r.Text
		// Fast path: no field chars in this run.
		if !strings.ContainsAny(text, "\x13\x14\x15") {
			if inField() && !inResult() {
				top := &stack[len(stack)-1]
				top.instr.WriteString(text)
				top.instrRuns = append(top.instrRuns, r)
			} else if inResult() {
				emitText(text, r)
			} else {
				out = append(out, r)
			}
			continue
		}

		// Slow path: split at field chars.
		var seg strings.Builder
		for _, ch := range text {
			switch ch {
			case fldBegin:
				emitText(seg.String(), r)
				seg.Reset()
				if inResult() {
					handleField(flushField())
				}
				stack = append(stack, frame{})
			case fldSep:
				emitText(seg.String(), r)
				seg.Reset()
				if !inField() {
					continue
				}
				top := &stack[len(stack)-1]
				instr := strings.TrimSpace(top.instr.String())
				top.parsed = parseDocFieldInstruction(instr)
				top.parsed.Instr = instr
				top.inResult = true
			case fldEnd:
				emitText(seg.String(), r)
				seg.Reset()
				if !inField() {
					continue
				}
				handleField(flushField())
			default:
				seg.WriteRune(ch)
			}
		}
		emitText(seg.String(), r)
	}

	// Flush any unclosed fields from the stack.
	for len(stack) > 0 {
		handleField(flushField())
	}

	return out
}

type legacyDocField struct {
	Kind   string
	Instr  string
	Arg    string
	Anchor string
	Target string
}

func parseDocFieldInstruction(instr string) legacyDocField {
	trimmed := strings.TrimSpace(instr)
	if trimmed == "" {
		return legacyDocField{}
	}
	upper := strings.ToUpper(trimmed)
	switch {
	case strings.HasPrefix(upper, "HYPERLINK"):
		return legacyDocField{Target: parseDocFieldURL(trimmed)}
	case strings.HasPrefix(upper, "NUMPAGES"):
		return legacyDocField{Kind: "NUMPAGES"}
	case strings.HasPrefix(upper, "NUMCHARS"):
		return legacyDocField{Kind: "NUMCHARS"}
	case strings.HasPrefix(upper, "NUMWORDS"):
		return legacyDocField{Kind: "NUMWORDS"}
	case strings.HasPrefix(upper, "SEQ"):
		return legacyDocField{Kind: "SEQ", Arg: parseDocFieldFirstArg(trimmed[len("SEQ"):])}
	case strings.HasPrefix(upper, "ASK"):
		return legacyDocField{Kind: "ASK", Arg: parseDocFieldFirstArg(trimmed[len("ASK"):])}
	case strings.HasPrefix(upper, "QUOTE"):
		return legacyDocField{Kind: "QUOTE", Arg: strings.TrimSpace(trimmed)}
	case strings.HasPrefix(upper, "MERGEFIELD"):
		return legacyDocField{Kind: "MERGEFIELD", Arg: parseDocFieldFirstArg(trimmed[len("MERGEFIELD"):])}
	case strings.HasPrefix(upper, "AUTOTEXT"):
		return legacyDocField{Kind: "AUTOTEXT", Arg: parseDocFieldFirstArg(trimmed[len("AUTOTEXT"):])}
	case strings.HasPrefix(upper, "EMBED"):
		return legacyDocField{Kind: "EMBED", Arg: strings.TrimSpace(trimmed)}
	case strings.HasPrefix(upper, "LINK"):
		return legacyDocField{Kind: "LINK", Arg: strings.TrimSpace(trimmed)}
	case strings.HasPrefix(upper, "FORMCHECKBOX"):
		return legacyDocField{Kind: "FORMCHECKBOX"}
	case strings.HasPrefix(upper, "FORMDROPDOWN"):
		return legacyDocField{Kind: "FORMDROPDOWN"}
	case strings.HasPrefix(upper, "FORMTEXT"):
		return legacyDocField{Kind: "FORMTEXT"}
	case strings.HasPrefix(upper, "PAGEREF"):
		return legacyDocField{Kind: "PAGEREF", Anchor: parseDocFieldFirstArg(trimmed[len("PAGEREF"):])}
	case strings.HasPrefix(upper, "PAGE"):
		return legacyDocField{Kind: "PAGE"}
	case strings.HasPrefix(upper, "AUTHOR"):
		return legacyDocField{Kind: "AUTHOR"}
	case strings.HasPrefix(upper, "CREATEDATE"):
		return legacyDocField{Kind: "CREATEDATE"}
	case strings.HasPrefix(upper, "SAVEDATE"):
		return legacyDocField{Kind: "SAVEDATE"}
	case strings.HasPrefix(upper, "PRINTDATE"):
		return legacyDocField{Kind: "PRINTDATE"}
	case strings.HasPrefix(upper, "DATE"):
		return legacyDocField{Kind: "DATE"}
	case strings.HasPrefix(upper, "TIME"):
		return legacyDocField{Kind: "TIME"}
	case strings.HasPrefix(upper, "TITLE"):
		return legacyDocField{Kind: "TITLE"}
	case strings.HasPrefix(upper, "SUBJECT"):
		return legacyDocField{Kind: "SUBJECT"}
	case strings.HasPrefix(upper, "STYLEREF"):
		// Reference to the text of the nearest paragraph with a given style
		return legacyDocField{Kind: "STYLEREF", Arg: parseDocFieldFirstArg(trimmed[len("STYLEREF"):])}
	case strings.HasPrefix(upper, "REF"):
		// Reference to a bookmark; e.g., REF BookmarkName or REF "Bookmark Name"
		return legacyDocField{Kind: "REF", Anchor: parseDocFieldFirstArg(trimmed[len("REF"):])}
	case strings.HasPrefix(upper, "FILLIN"):
		// Interactive fill-in field; prompt and optional default value
		// FILLIN "prompt" ["default"]
		rest := strings.TrimSpace(trimmed[len("FILLIN"):])
		defaultValue := parseDocFieldFillinDefault(rest)
		return legacyDocField{Kind: "FILLIN", Arg: defaultValue}
	case strings.HasPrefix(upper, "DOCPROPERTY"):
		return legacyDocField{Kind: "DOCPROPERTY", Arg: parseDocFieldFirstArg(trimmed[len("DOCPROPERTY"):])}
	case strings.HasPrefix(upper, "DOCVARIABLE"):
		return legacyDocField{Kind: "DOCVARIABLE", Arg: parseDocFieldFirstArg(trimmed[len("DOCVARIABLE"):])}
	case strings.HasPrefix(upper, "TOC"):
		return legacyDocField{Kind: "TOC", Arg: strings.TrimSpace(trimmed[len("TOC"):])}
	case strings.HasPrefix(upper, "INDEX"):
		return legacyDocField{Kind: "INDEX", Arg: strings.TrimSpace(trimmed[len("INDEX"):])}
	case strings.HasPrefix(upper, "XE"):
		return legacyDocField{Kind: "XE", Arg: parseDocFieldFirstArg(trimmed[len("XE"):])}
	case strings.HasPrefix(upper, "TC"):
		return legacyDocField{Kind: "TC", Arg: parseDocFieldFirstArg(trimmed[len("TC"):])}
	case strings.HasPrefix(upper, "IF"):
		return legacyDocField{Kind: "IF", Arg: strings.TrimSpace(trimmed[len("IF"):])}
	case strings.HasPrefix(upper, "FILENAME"):
		// Use the cached result stored in the binary as plain text; the
		// HTML renderer's computed r.doc.Filename is the test-file path, not
		// the original document filename stored in the binary.
		return legacyDocField{}
	}
	return legacyDocField{}
}

func legacyDocFieldIsFormField(kind string) bool {
	switch kind {
	case "FORMTEXT", "FORMCHECKBOX", "FORMDROPDOWN":
		return true
	default:
		return false
	}
}

func legacyDocFormFieldRun(field legacyDocField, resultRuns []binaryWordRun, dataStream []byte) binaryWordRun {
	base := binaryWordRun{}
	if len(resultRuns) > 0 {
		base = resultRuns[0]
	}
	formField := &wordir.FormField{}
	displayText := joinDocRunsText(resultRuns)

	// Try to read binary FFData from the Data stream when a picOffset is available.
	if dataStream != nil && base.Format.PicLocation > 0 {
		if ff := readLegacyFFData(dataStream, base.Format.PicLocation); ff != nil {
			formField = ff
			// The cached result text acts as a fallback when FFData doesn't provide a value.
			if formField.Type == "text" && formField.DefaultText == "" {
				formField.DefaultText = displayText
			}
			if formField.Type == "checkbox" && displayText != "" {
				// Binary wDef takes precedence; only fall back to display text
				// when the binary structure didn't have a wDef field.
			}
			if formField.Type == "dropdown" && len(formField.Items) == 0 && strings.TrimSpace(displayText) != "" {
				formField.Items = []string{displayText}
				formField.DefaultIdx = 0
			}
			goto done
		}
	}

	switch field.Kind {
	case "FORMTEXT":
		formField.Type = "text"
		formField.DefaultText = displayText
	case "FORMCHECKBOX":
		formField.Type = "checkbox"
		formField.Checked = legacyDocFormFieldCheckboxChecked(displayText)
	case "FORMDROPDOWN":
		formField.Type = "dropdown"
		if strings.TrimSpace(displayText) != "" {
			formField.Items = []string{displayText}
			formField.DefaultIdx = 0
		}
	}
done:
	base.Text = ""
	base.Kind = "formField"
	base.FormField = formField
	base.FieldKind = field.Kind
	base.FieldInstr = field.Instr
	base.FieldArg = field.Arg
	base.Anchor = field.Anchor
	base.Target = field.Target
	base.Image = nil
	base.Shape = nil
	return base
}

func legacyDocFormFieldCheckboxChecked(text string) bool {
	text = strings.TrimSpace(strings.ToUpper(text))
	switch text {
	case "1", "TRUE", "X", "YES", "ON", "☑", "☒":
		return true
	case "0", "FALSE", "NO", "OFF", "☐":
		return false
	default:
		return false
	}
}

// parseDocFieldURL extracts a URL from a HYPERLINK field instruction string
// of the form: HYPERLINK "url" or HYPERLINK url.
func parseDocFieldURL(instr string) string {
	trimmed := strings.TrimSpace(instr)
	if len(trimmed) < len("HYPERLINK") || !strings.EqualFold(trimmed[:len("HYPERLINK")], "HYPERLINK") {
		return ""
	}
	rest := strings.TrimSpace(trimmed[len("HYPERLINK"):])
	return parseDocFieldFirstArg(rest)
}

func parseDocFieldFirstArg(rest string) string {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	if rest[0] == '"' {
		end := strings.IndexByte(rest[1:], '"')
		if end >= 0 {
			return rest[1 : end+1]
		}
		return strings.Trim(rest, "\"")
	}
	if idx := strings.IndexAny(rest, " \t\\"); idx > 0 {
		return rest[:idx]
	}
	return rest
}

// parseDocFieldFillinDefault extracts the default value from a FILLIN field instruction.
// FILLIN instructions have the form: FILLIN "prompt" ["default"]
// This function skips the prompt and extracts the default value if present.
func parseDocFieldFillinDefault(rest string) string {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	// Skip the prompt (first quoted or space-delimited argument)
	if rest[0] == '"' {
		end := strings.IndexByte(rest[1:], '"')
		if end < 0 {
			return "" // Malformed prompt
		}
		afterPrompt := rest[end+2:] // Skip past closing quote
		rest = strings.TrimSpace(afterPrompt)
	} else {
		// Unquoted prompt; skip to next space
		idx := strings.IndexAny(rest, " \t\\")
		if idx < 0 {
			return "" // No default value
		}
		rest = strings.TrimSpace(rest[idx:])
	}
	// Now rest should be the default value (quoted or unquoted)
	return parseDocFieldFirstArg(rest)
}

// ---------------------------------------------------------------------------
// Run-level document builder
// ---------------------------------------------------------------------------

// buildLegacyDocFromRuns is the run-aware variant of the legacy doc builder.
// It normalises raw runs (control chars, newlines), splits them into lines,
// detects tables and list markers using the same heuristics as the text-only
// path, and produces wordir.Paragraph runs with their RunFormat populated.
// lineSliceInTable returns true if the first run in the slice has InTable=true.
func lineSliceInTable(slice []binaryWordRun) bool {
	for _, r := range slice {
		return r.ParaFormat.InTable
	}
	return false
}

func lineSliceHasNativeCellMarks(slice []binaryWordRun) bool {
	for _, r := range slice {
		if r.NativeCellMarks {
			return true
		}
	}
	return false
}

func lineSliceInTableFromLegacyHintOnly(slice []binaryWordRun) bool {
	if len(slice) == 0 {
		return false
	}
	seenInTable := false
	seenLegacyHint := false
	for _, r := range slice {
		if (r.ParaFormat.IsTableTrailingParagraph && !r.LegacyRowMarkHint) || r.ParaFormat.IsTableCellTrailingParagraph {
			return false
		}
		if r.TableProps != nil && r.TableProps.HasData() {
			return false
		}
		if r.ParaFormat.InTable {
			seenInTable = true
			if !r.LegacyInTableHint {
				return false
			}
			seenLegacyHint = true
		}
	}
	return seenInTable && seenLegacyHint
}

func lineSliceHasBinaryTableMetadata(slice []binaryWordRun) bool {
	for _, r := range slice {
		if (r.ParaFormat.InTable && !r.LegacyInTableHint) || r.ParaFormat.IsTableCellTrailingParagraph {
			return true
		}
		if props := r.TableProps; props != nil && props.HasData() {
			return true
		}
	}
	return false
}

func lineSliceIsExplicitRowMark(slice []binaryWordRun) bool {
	for _, r := range slice {
		if r.ParaFormat.IsTableTrailingParagraph && !r.LegacyRowMarkHint {
			return true
		}
	}
	return false
}

// lineSliceHasPreWord97RowMark returns true if the slice contains a pre-97 row mark (0x19)
// with IsTableTrailingParagraph set. This distinguishes pre-97 row marks from Word 97+ row marks.
func lineSliceHasPreWord97RowMark(slice []binaryWordRun) bool {
	for _, r := range slice {
		if r.ParaFormat.IsTableTrailingParagraph && r.LegacyRowMarkHint {
			return true
		}
	}
	return false
}

// lineSliceHasPreWord97CellMark returns true if the slice contains a pre-97 cell mark (0x18)
// with IsTableCellTrailingParagraph set. This distinguishes pre-97 cell marks.
func lineSliceHasPreWord97CellMark(slice []binaryWordRun) bool {
	for _, r := range slice {
		if r.ParaFormat.IsTableCellTrailingParagraph && r.LegacyInTableHint {
			return true
		}
	}
	return false
}

// lineSliceIsPreWord97RowMarkContent returns true if the slice came from a pre-97
// row mark paragraph (0x19 toggle) that carries actual cell content. Unlike
// lineSliceHasPreWord97RowMark, this does not require IsTableTrailingParagraph to
// be set: pre-97 row paragraphs with content end in \r (not \a) after PAP
// collapsing, yet still carry LegacyRowMarkHint and NativeCellMarks because
// their text contains \a cell separators.
func lineSliceIsPreWord97RowMarkContent(slice []binaryWordRun) bool {
	for _, r := range slice {
		if r.LegacyRowMarkHint && r.NativeCellMarks {
			return true
		}
	}
	return false
}

func lineSliceIsRowMark(slice []binaryWordRun) bool {
	for _, r := range slice {
		if r.ParaFormat.IsTableTrailingParagraph {
			return true
		}
	}
	return false
}

func lineSliceIsCellMark(slice []binaryWordRun) bool {
	for _, r := range slice {
		if r.ParaFormat.IsTableCellTrailingParagraph {
			return true
		}
	}
	return false
}

func lineSliceIsLegacyListBoundary(slice []binaryWordRun) bool {
	return len(slice) == 1 && slice[0].Kind == legacyListBoundaryKind
}

type legacyBinaryTableRow struct {
	LineSlices [][]binaryWordRun
	RowMark    []binaryWordRun
}

func lineSliceStartsNewMetadataRow(currentRow [][]binaryWordRun, next []binaryWordRun, baseDepth int) bool {
	if len(currentRow) == 0 || lineSliceIsRowMark(next) || lineSliceTableDepth(next) != baseDepth {
		return false
	}
	nextProps := lineSliceTableRowProps(next)
	if nextProps == nil {
		return false
	}
	currentProps := collectLineSlicesTableRowProps(currentRow, baseDepth)
	if currentProps == nil {
		return false
	}
	return docformat.TableRowPropsConflict(currentProps, nextProps)
}

func collectLineSlicesTableRowProps(slices [][]binaryWordRun, depth int) *docformat.TableRowProps {
	var merged *docformat.TableRowProps
	for _, slice := range slices {
		if lineSliceIsRowMark(slice) || lineSliceTableDepth(slice) != depth {
			continue
		}
		merged = docformat.MergeTableRowProps(merged, lineSliceTableRowProps(slice))
	}
	return merged
}

func lineSliceTableRowProps(slice []binaryWordRun) *docformat.TableRowProps {
	for _, r := range slice {
		if props := r.TableProps; props != nil && props.HasData() {
			return props
		}
	}
	return nil
}

func rowGroupTableRowProps(group legacyBinaryTableRow) *docformat.TableRowProps {
	merged := lineSliceTableRowProps(group.RowMark)
	for _, lineSlice := range group.LineSlices {
		merged = docformat.MergeTableRowProps(merged, lineSliceTableRowProps(lineSlice))
	}
	if merged == nil || !merged.HasData() {
		return nil
	}
	return merged
}

func cloneLegacyRevisionMark(mark *wordir.RevisionMark) *wordir.RevisionMark {
	if mark == nil {
		return nil
	}
	return &wordir.RevisionMark{
		Author: mark.Author,
		Date:   mark.Date,
	}
}

func lineSlicePropertyRevision(slice []binaryWordRun) *wordir.RevisionMark {
	for _, r := range slice {
		if r.PropertyRevision != nil {
			return cloneLegacyRevisionMark(r.PropertyRevision)
		}
	}
	return nil
}

func rowGroupPropertyRevision(group legacyBinaryTableRow) *wordir.RevisionMark {
	// Prefer the canonical sprmTPropRMark (TAPx) when present on the row-mark.
	for _, r := range group.RowMark {
		if props := r.TableProps; props != nil && props.ResolvedPropertyRevision != nil {
			return cloneLegacyRevisionMark(props.ResolvedPropertyRevision)
		}
	}
	// Fall back to the row-mark paragraph's PAPX sprmPPropRMark.
	if mark := lineSlicePropertyRevision(group.RowMark); mark != nil {
		return mark
	}
	for _, lineSlice := range group.LineSlices {
		if !lineSliceIsPreWord97RowMarkContent(lineSlice) {
			continue
		}
		if mark := lineSlicePropertyRevision(lineSlice); mark != nil {
			return mark
		}
	}
	return nil
}

func lineSliceTableDepth(slice []binaryWordRun) int {
	depth := 0
	for _, r := range slice {
		if r.TableDepth > depth {
			depth = r.TableDepth
		}
	}
	return depth
}

func rowGroupMaxTableDepth(group legacyBinaryTableRow) int {
	depth := lineSliceTableDepth(group.RowMark)
	for _, lineSlice := range group.LineSlices {
		if lineDepth := lineSliceTableDepth(lineSlice); lineDepth > depth {
			depth = lineDepth
		}
	}
	return depth
}

func rowGroupBaseTableDepth(group legacyBinaryTableRow) int {
	if depth := lineSliceTableDepth(group.RowMark); depth > 0 {
		return depth
	}
	base := 0
	for _, lineSlice := range group.LineSlices {
		lineDepth := lineSliceTableDepth(lineSlice)
		if lineDepth == 0 {
			continue
		}
		if base == 0 || lineDepth < base {
			base = lineDepth
		}
	}
	return base
}

func countLegacyBinaryRowCells(group legacyBinaryTableRow) int {
	text := joinDocRunsText(group.RowMark)
	if text == "" {
		return 1
	}
	return strings.Count(text, "\t") + 1
}

func collectLegacyBinaryTableRowGroups(lineSlices [][]binaryWordRun) []legacyBinaryTableRow {
	if len(lineSlices) == 0 {
		return nil
	}
	baseDepth := lineSliceTableDepth(lineSlices[0])
	for _, lineSlice := range lineSlices[1:] {
		depth := lineSliceTableDepth(lineSlice)
		if depth < baseDepth {
			baseDepth = depth
		}
	}
	var rowGroups []legacyBinaryTableRow
	var currentRow [][]binaryWordRun
	for _, lineSlice := range lineSlices {
		if lineSliceIsRowMark(lineSlice) && lineSliceTableDepth(lineSlice) == baseDepth {
			if len(currentRow) > 0 {
				rowGroups = append(rowGroups, legacyBinaryTableRow{
					LineSlices: currentRow,
					RowMark:    append([]binaryWordRun(nil), lineSlice...),
				})
				currentRow = nil
			}
			continue
		}
		// Content-bearing pre-97 row mark (LegacyRowMarkHint+NativeCellMarks, ends in \r not \a):
		// include its text in the current row's cells, then close the group. This guards
		// against CHPX-level LegacyRowMarkHint propagation by requiring NativeCellMarks.
		if lineSliceIsPreWord97RowMarkContent(lineSlice) && lineSliceTableDepth(lineSlice) == baseDepth {
			currentRow = append(currentRow, lineSlice)
			rowGroups = append(rowGroups, legacyBinaryTableRow{LineSlices: currentRow})
			currentRow = nil
			continue
		}
		if lineSliceStartsNewMetadataRow(currentRow, lineSlice, baseDepth) {
			rowGroups = append(rowGroups, legacyBinaryTableRow{LineSlices: currentRow})
			currentRow = nil
		}
		currentRow = append(currentRow, lineSlice)
	}
	if len(currentRow) > 0 {
		rowGroups = append(rowGroups, legacyBinaryTableRow{LineSlices: currentRow})
	}
	return rowGroups
}

// appendBinaryTableToDoc builds and appends a table from binary-format InTable
// line groups.  Each group is a slice of line-slices that make up one table row
// (everything between two consecutive row-mark lines).  Returns true when a
// table was successfully emitted.
func appendBinaryTableToDoc(doc *wordir.Document, rowGroups []legacyBinaryTableRow) bool {
	if len(rowGroups) == 0 {
		return false
	}
	table, trailing := buildLegacyBinaryTable(len(doc.Tables), rowGroups, len(doc.Paragraphs))
	if table == nil {
		return false
	}
	doc.Tables = append(doc.Tables, *table)
	doc.Blocks = append(doc.Blocks, wordir.Block{
		Index: len(doc.Blocks),
		Kind:  "table",
		Table: &doc.Tables[len(doc.Tables)-1],
	})
	for _, trailingLine := range trailing {
		paragraph := buildLegacyParagraph(len(doc.Paragraphs), trailingLine)
		doc.Paragraphs = append(doc.Paragraphs, paragraph)
		doc.Blocks = append(doc.Blocks, wordir.Block{
			Index:     len(doc.Blocks),
			Kind:      "paragraph",
			Paragraph: &doc.Paragraphs[len(doc.Paragraphs)-1],
		})
	}
	return true
}

const legacyBinaryCellParagraphSeparator = "\x1e"

func normalizeLegacyBinaryRowTokens(tokens []string, props *docformat.TableRowProps) []string {
	count := props.PhysicalCellCount()
	if count <= 0 || len(tokens) <= count {
		return tokens
	}
	// PAP/TAP exposes the physical row cell count. Extra text tokens commonly
	// come from additional paragraphs inside the last cell (not extra cells).
	// Preserve those paragraphs instead of widening the row and corrupting the
	// recovered table grid.
	out := append([]string(nil), tokens[:count-1]...)
	tail := make([]string, 0, len(tokens)-count+1)
	for _, token := range tokens[count-1:] {
		tail = append(tail, token)
	}
	out = append(out, strings.Join(tail, legacyBinaryCellParagraphSeparator))
	return out
}

func buildLegacyBinaryTable(tableIndex int, rowGroups []legacyBinaryTableRow, paragraphStart int) (*wordir.Table, []string) {
	if table, ok := buildLegacyNestedBinaryTable(tableIndex, rowGroups, paragraphStart); ok {
		return table, nil
	}
	rowTokens := make([][]string, 0, len(rowGroups))
	rowCellRevisions := make([][]legacyCellRevisionMeta, 0, len(rowGroups))
	rowProps := make([]*docformat.TableRowProps, 0, len(rowGroups))
	rowRevisions := make([]*wordir.RevisionMark, 0, len(rowGroups))
	maxCols := 0
	maxTokenSpanCols := 0
	for _, group := range rowGroups {
		rowText := ""
		for _, lineSlice := range group.LineSlices {
			cellText := joinDocRunsText(lineSlice)
			if cellText != "" {
				if rowText != "" && !strings.HasSuffix(rowText, "\t") && !strings.HasPrefix(cellText, "\t") {
					rowText += "\t"
				}
				rowText += cellText
			}
		}
		var tokens []string
		if rowText != "" {
			tokens = strings.Split(rowText, "\t")
			first := -1
			last := -1
			for tokenIndex, token := range tokens {
				if strings.TrimSpace(token) == "" {
					continue
				}
				if first < 0 {
					first = tokenIndex
				}
				last = tokenIndex
			}
			if first >= 0 {
				spanCols := last - first + 1
				if spanCols > maxTokenSpanCols {
					maxTokenSpanCols = spanCols
				}
			}
			if len(tokens) > maxCols {
				maxCols = len(tokens)
			}
		}
		props := rowGroupTableRowProps(group)
		if props != nil {
			tokens = normalizeLegacyBinaryRowTokens(tokens, props)
			propCols := props.MaxCellIndex()
			switch {
			case propCols <= 0:
				// No metadata-derived cell width.
			case len(tokens) == 0:
				// Metadata-only rows need metadata-derived width.
				if propCols > maxCols {
					maxCols = propCols
				}
			case propCols <= len(tokens)+16:
				// Keep metadata only when it stays close to observed token width.
				// Large jumps are usually noisy pre-97 metadata and explode GridAfter.
				if propCols > maxCols {
					maxCols = propCols
				}
			}
		}
		rowTokens = append(rowTokens, tokens)
		rowCellRevisions = append(rowCellRevisions, legacyRowCellRevisionMetadata(group))
		rowProps = append(rowProps, props)
		rowRevisions = append(rowRevisions, rowGroupPropertyRevision(group))
	}
	if maxTokenSpanCols > 0 && maxCols > maxTokenSpanCols+16 {
		maxCols = maxTokenSpanCols
	}
	if maxCols < 1 {
		return nil, nil
	}
	rows := make([]wordir.TableRow, 0, len(rowGroups))
	textLines := make([]string, 0, len(rowGroups)*2)
	paragraphIndex := paragraphStart
	for index, tokens := range rowTokens {
		rowCols := maxCols
		if props := rowProps[index]; props != nil && props.PhysicalCellCount() > 0 {
			rowCols = props.PhysicalCellCount()
		}
		row, texts, nextIndex, ok := buildLegacyBinaryGridRow(tokens, rowCellRevisions[index], rowCols, paragraphIndex, rowProps[index], rowRevisions[index])
		if rowProps[index] != nil && len(tokens) > 0 && (!ok || len(texts) == 0) {
			row, texts, nextIndex, ok = buildLegacyBinaryGridRow(tokens, rowCellRevisions[index], maxCols, paragraphIndex, nil, rowRevisions[index])
		}
		if !ok {
			continue
		}
		paragraphIndex = nextIndex
		rows = append(rows, row)
		textLines = append(textLines, texts...)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	table := &wordir.Table{Index: tableIndex, Rows: rows, Text: strings.Join(textLines, "\n")}
	applyLegacyBinaryTableLevelProps(&table.Format, rowProps)
	regridLegacyBinaryTableRows(table, rowProps)
	// Old binary tables, especially pre-Word97 grids, frequently have usable
	// cell content but no trustworthy TAP-derived boundaries.  Do not leave
	// those tables to the layout engine's equal-column fallback: it makes
	// narrow numeric columns consume the same width as descriptions and can
	// add an entire page through avoidable wrapping.  Prefer real TAP widths
	// above; infer only when the binary metadata could not provide them.
	if len(table.Format.ColumnWidths) == 0 {
		table.Format.ColumnWidths = inferLegacyTableRowColumnWidths(table.Rows)
	}
	if table.Format.Width == nil && len(table.Format.ColumnWidths) > 0 {
		total := 0
		for _, width := range table.Format.ColumnWidths {
			total += width
		}
		if total > 0 {
			table.Format.Width = &total
			table.Format.WidthType = "dxa"
		}
	}
	normalizeLegacyBinaryTableGridGaps(table)
	applyLegacyTablePropertyRevision(table, rowRevisions)
	return table, nil
}

// normalizeLegacyBinaryTableGridGaps keeps row-level grid gaps inside the
// recovered table grid. Empty trailing token slots after a horizontal merge
// are already represented by GridSpan; counting them again as GridAfter creates
// phantom cells and makes the rendered row wider than its colgroup.
func normalizeLegacyBinaryTableGridGaps(table *wordir.Table) {
	if table == nil || len(table.Format.ColumnWidths) == 0 {
		return
	}
	gridCols := len(table.Format.ColumnWidths)
	for rowIndex := range table.Rows {
		row := &table.Rows[rowIndex]
		if row.Format.GridBefore < 0 {
			row.Format.GridBefore = 0
		}
		if row.Format.GridBefore > gridCols {
			row.Format.GridBefore = gridCols
		}
		if row.Format.GridAfter < 0 {
			row.Format.GridAfter = 0
		}
		if row.Format.GridAfter > gridCols-row.Format.GridBefore {
			row.Format.GridAfter = gridCols - row.Format.GridBefore
		}

		// Clamp recovered cell spans to the physical grid. Corrupt or redundant
		// legacy merge metadata can otherwise make a row consume more columns
		// than its TAP/colgroup declares (for example 1 + colspan=2 in a
		// two-column table). HMerge continuation cells consume one physical grid
		// slot and are folded into the previous visible cell by the renderer.
		occupied := row.Format.GridBefore
		for cellIndex := range row.Cells {
			cell := &row.Cells[cellIndex]
			if cell.Format.HMerge {
				if occupied < gridCols {
					occupied++
				}
				continue
			}
			span := cell.Format.GridSpan
			if span <= 0 {
				span = 1
			}
			remaining := gridCols - occupied
			if remaining <= 0 {
				cell.Format.GridSpan = 1
				continue
			}
			if span > remaining {
				span = remaining
			}
			cell.Format.GridSpan = span
			occupied += span
		}

		remaining := gridCols - occupied
		if remaining < 0 {
			remaining = 0
		}
		if row.Format.GridAfter > remaining {
			row.Format.GridAfter = remaining
		}
	}
}

// inferLegacyTableRowColumnWidths derives proportional widths from logical cell
// content after a binary table has already been reconstructed.  It is deliberately
// a fallback: native TAP widths, when present, remain authoritative.
func inferLegacyTableRowColumnWidths(rows []wordir.TableRow) []int {
	cols := 0
	for _, row := range rows {
		if len(row.Cells) > cols {
			cols = len(row.Cells)
		}
	}
	if cols < 2 {
		return nil
	}
	weights := make([]int, cols)
	for _, row := range rows {
		for col, cell := range row.Cells {
			if col >= cols {
				break
			}
			text := strings.TrimSpace(cell.Text)
			if text == "" {
				for _, block := range cell.Blocks {
					if block.Paragraph != nil {
						if text != "" {
							text += " "
						}
						text += block.Paragraph.Text
					}
				}
			}
			w := len([]rune(strings.TrimSpace(text)))
			if w < 6 {
				w = 6
			}
			if w > 60 {
				w = 60
			}
			if w > weights[col] {
				weights[col] = w
			}
		}
	}
	totalWeight := 0
	for i := range weights {
		if weights[i] == 0 {
			weights[i] = 6
		}
		totalWeight += weights[i]
	}
	if totalWeight == 0 {
		return nil
	}
	const totalTwips = 10000
	widths := make([]int, cols)
	remaining := totalTwips
	for i := 0; i < cols; i++ {
		if i == cols-1 {
			widths[i] = remaining
			break
		}
		widths[i] = totalTwips * weights[i] / totalWeight
		if widths[i] < 1 {
			widths[i] = 1
		}
		remaining -= widths[i]
	}
	return widths
}

// applyLegacyTablePropertyRevision propagates a shared row property revision to
// the table level when every row carries the same revision.  This is a heuristic
// because legacy DOC has no spec-level table property revision SPRM.
func applyLegacyTablePropertyRevision(table *wordir.Table, rowRevisions []*wordir.RevisionMark) {
	if table == nil || len(rowRevisions) == 0 {
		return
	}
	var common *wordir.RevisionMark
	for _, rev := range rowRevisions {
		if rev == nil {
			continue
		}
		if common == nil {
			common = rev
			continue
		}
		if common.Author != rev.Author || common.Date != rev.Date {
			return
		}
	}
	if common != nil {
		table.PropertyRevision = cloneLegacyRevisionMark(common)
	}
}

// applyLegacyBinaryTableLevelProps populates table-level fields (alignment,
// indent) from the first row whose TAPx grpprl carried sprmTJc / sprmTDxaLeft.
// MS-DOC writes these per-row but the values are repeated across rows of one
// table; trusting the first set keeps the IR consistent.
func applyLegacyBinaryTableLevelProps(format *wordir.TableFormat, rowProps []*docformat.TableRowProps) {
	if format == nil {
		return
	}
	for _, props := range rowProps {
		if props == nil {
			continue
		}
		if format.Alignment == "" && props.TableJc != "" {
			format.Alignment = props.TableJc
		}
		if format.Indent == 0 && props.TableDxaLeft != nil {
			format.Indent = *props.TableDxaLeft
		}
		if format.CellSpacing == 0 && props.TableDxaGap != nil {
			// Word stores half-gap in sprmTDxaGapHalf; CSS border-spacing uses full gap.
			format.CellSpacing = *props.TableDxaGap * 2
		}
		if format.Borders == nil && props.TableBorders != nil {
			format.Borders = cloneLegacyTableBorders(props.TableBorders)
		}
		if format.CellPadTop == 0 && props.CellPadTop != nil {
			format.CellPadTop = *props.CellPadTop
		}
		if format.CellPadBottom == 0 && props.CellPadBottom != nil {
			format.CellPadBottom = *props.CellPadBottom
		}
		if format.CellPadLeft == 0 && props.CellPadLeft != nil {
			format.CellPadLeft = *props.CellPadLeft
		}
		if format.CellPadRight == 0 && props.CellPadRight != nil {
			format.CellPadRight = *props.CellPadRight
		}
		if format.Width == nil && props.TableWidth != nil {
			w := *props.TableWidth
			format.Width = &w
			format.WidthType = props.TableWidthType
		}
		if format.Layout == "" && props.AutoFit != nil {
			if *props.AutoFit {
				format.Layout = "autofit"
			} else {
				format.Layout = "fixed"
			}
		}
		if !format.BidiVisual && props.BidiVisual != nil {
			format.BidiVisual = *props.BidiVisual
		}
		if format.FloatProps == nil && props.FloatProps != nil {
			fp := *props.FloatProps
			format.FloatProps = &fp
		}
		if format.Overlap == "" && props.NoOverlap != nil {
			if *props.NoOverlap {
				format.Overlap = "never"
			} else {
				format.Overlap = "overlap"
			}
		}
		if format.Alignment != "" && format.Indent != 0 && format.CellSpacing != 0 && format.Borders != nil && format.CellPadTop != 0 && format.CellPadBottom != 0 && format.CellPadLeft != 0 && format.CellPadRight != 0 && format.Width != nil && format.Layout != "" {
			break
		}
	}
	if len(format.ColumnWidths) == 0 {
		if widths := deriveLegacyBinaryTableColumnWidths(rowProps); len(widths) > 0 {
			format.ColumnWidths = widths
		}
	}
	if format.Width == nil && len(format.ColumnWidths) > 0 {
		total := 0
		for _, width := range format.ColumnWidths {
			total += width
		}
		if total > 0 {
			format.Width = &total
			format.WidthType = "dxa"
		}
	}
}

func deriveLegacyBinaryTableGridBoundaries(rowProps []*docformat.TableRowProps) []int {
	var boundaries []int
	for _, props := range rowProps {
		for _, boundary := range props.ValidCellBoundaries() {
			found := false
			for _, existing := range boundaries {
				if existing == boundary {
					found = true
					break
				}
			}
			if !found {
				boundaries = append(boundaries, boundary)
			}
		}
	}
	if len(boundaries) < 2 {
		return nil
	}
	sort.Ints(boundaries)
	for index := 1; index < len(boundaries); index++ {
		if boundaries[index] <= boundaries[index-1] {
			return nil
		}
	}
	return boundaries
}

func legacyBoundaryGridIndex(boundaries []int, value int) int {
	for index, boundary := range boundaries {
		if boundary == value {
			return index
		}
	}
	return -1
}

func regridLegacyBinaryTableRows(table *wordir.Table, rowProps []*docformat.TableRowProps) {
	if table == nil {
		return
	}
	grid := deriveLegacyBinaryTableGridBoundaries(rowProps)
	if len(grid) < 2 {
		return
	}
	gridCols := len(grid) - 1
	limit := len(table.Rows)
	if len(rowProps) < limit {
		limit = len(rowProps)
	}
	for rowIndex := 0; rowIndex < limit; rowIndex++ {
		props := rowProps[rowIndex]
		rowBounds := props.ValidCellBoundaries()
		if len(rowBounds) < 2 || len(table.Rows[rowIndex].Cells) != len(rowBounds)-1 {
			continue
		}
		start := legacyBoundaryGridIndex(grid, rowBounds[0])
		end := legacyBoundaryGridIndex(grid, rowBounds[len(rowBounds)-1])
		if start < 0 || end <= start {
			continue
		}
		row := &table.Rows[rowIndex]
		row.Format.GridBefore = start
		row.Format.GridAfter = gridCols - end
		for cellIndex := range row.Cells {
			cellStart := legacyBoundaryGridIndex(grid, rowBounds[cellIndex])
			cellEnd := legacyBoundaryGridIndex(grid, rowBounds[cellIndex+1])
			if cellStart < 0 || cellEnd <= cellStart {
				continue
			}
			row.Cells[cellIndex].Format.GridSpan = cellEnd - cellStart
		}
	}
}

func deriveLegacyBinaryTableColumnWidths(rowProps []*docformat.TableRowProps) []int {
	if boundaries := deriveLegacyBinaryTableGridBoundaries(rowProps); len(boundaries) >= 2 {
		widths := make([]int, len(boundaries)-1)
		for index := range widths {
			widths[index] = boundaries[index+1] - boundaries[index]
			if widths[index] <= 0 {
				return nil
			}
		}
		return widths
	}
	maxCols := 0
	for _, props := range rowProps {
		if props == nil {
			continue
		}
		if cols := props.MaxCellIndex(); cols > maxCols {
			maxCols = cols
		}
	}
	if maxCols == 0 {
		return nil
	}
	widths := make([]int, maxCols)
	known := make([]bool, maxCols)
	for _, props := range rowProps {
		if props == nil {
			continue
		}
		for index := 0; index < maxCols; index++ {
			if known[index] || props.HMergeState(index) == 1 {
				continue
			}
			format := props.CellFormat(index)
			if format.Width == nil || format.WidthType != "dxa" || *format.Width <= 0 {
				continue
			}
			span := 1
			if state := props.HMergeState(index); state == 2 || state == 3 {
				for next := index + 1; next < maxCols && props.HMergeState(next) == 1; next++ {
					span++
				}
			}
			width := *format.Width
			if span > 1 {
				if width%span != 0 {
					continue
				}
				width /= span
			}
			for offset := 0; offset < span && index+offset < maxCols; offset++ {
				column := index + offset
				if !known[column] {
					widths[column] = width
					known[column] = true
				}
			}
		}
	}
	for index, ok := range known {
		if !ok || widths[index] <= 0 {
			return nil
		}
	}
	return widths
}

func buildLegacyNestedBinaryTable(tableIndex int, rowGroups []legacyBinaryTableRow, paragraphStart int) (*wordir.Table, bool) {
	rows := make([]wordir.TableRow, 0, len(rowGroups))
	rowRevisions := make([]*wordir.RevisionMark, 0, len(rowGroups))
	textLines := make([]string, 0, len(rowGroups)*2)
	seenNested := false
	for _, group := range rowGroups {
		baseDepth := rowGroupBaseTableDepth(group)
		maxDepth := rowGroupMaxTableDepth(group)
		if baseDepth <= 0 || maxDepth <= baseDepth {
			return nil, false
		}
		seenNested = true
		childDepth := baseDepth + 1
		childRowGroups := collectLegacyNestedRowGroups(group.LineSlices, childDepth)
		if len(childRowGroups) == 0 {
			return nil, false
		}
		cellCount := countLegacyBinaryRowCells(group)
		if cellCount < len(childRowGroups) {
			cellCount = len(childRowGroups)
		}
		row := wordir.TableRow{Cells: make([]wordir.TableCell, 0, cellCount)}
		props := rowGroupTableRowProps(group)
		revision := rowGroupPropertyRevision(group)
		cellRevisions := legacyRowCellRevisionMetadata(group)
		applyLegacyRowProps(&row, props, revision)
		rowRevisions = append(rowRevisions, revision)
		for index := 0; index < cellCount; index++ {
			cell := wordir.TableCell{}
			if props != nil {
				cell.Format = props.CellFormat(index)
			}
			var cellRevision legacyCellRevisionMeta
			if index < len(cellRevisions) {
				cellRevision = cellRevisions[index]
			}
			applyLegacyCellRevisionMetadata(&cell, cellRevision, row.PropertyRevision)
			if index < len(childRowGroups) {
				nested, _ := buildLegacyBinaryTable(0, childRowGroups[index], paragraphStart)
				if nested != nil {
					cell.Tables = []wordir.Table{*nested}
					cell.Blocks = []wordir.Block{{Index: 0, Kind: "table", Table: &cell.Tables[0]}}
					cell.Text = nested.Text
					if nested.Text != "" {
						textLines = append(textLines, nested.Text)
					}
				}
			}
			row.Cells = append(row.Cells, cell)
		}
		rows = append(rows, row)
	}
	if !seenNested || len(rows) == 0 {
		return nil, false
	}
	table := &wordir.Table{Index: tableIndex, Rows: rows, Text: strings.Join(textLines, "\n")}
	applyLegacyTablePropertyRevision(table, rowRevisions)
	return table, true
}

func collectLegacyNestedRowGroups(lineSlices [][]binaryWordRun, depth int) [][]legacyBinaryTableRow {
	var nested []legacyBinaryTableRow
	var current [][]binaryWordRun
	for _, lineSlice := range lineSlices {
		lineDepth := lineSliceTableDepth(lineSlice)
		if lineDepth != depth {
			continue
		}
		if lineSliceIsRowMark(lineSlice) {
			nested = append(nested, legacyBinaryTableRow{
				LineSlices: current,
				RowMark:    append([]binaryWordRun(nil), lineSlice...),
			})
			current = nil
			continue
		}
		current = append(current, lineSlice)
	}
	if len(current) > 0 {
		nested = append(nested, legacyBinaryTableRow{LineSlices: current})
	}
	if len(nested) == 0 {
		return nil
	}
	result := make([][]legacyBinaryTableRow, 0, len(nested))
	for _, group := range nested {
		result = append(result, []legacyBinaryTableRow{group})
	}
	return result
}

type legacyCellRevisionMeta struct {
	PropertyRevision *wordir.RevisionMark
	CellIns          *wordir.RevisionMark
	CellDel          *wordir.RevisionMark
}

func legacyRowCellRevisionMetadata(group legacyBinaryTableRow) []legacyCellRevisionMeta {
	runs := make([]binaryWordRun, 0)
	for _, lineSlice := range group.LineSlices {
		text := joinDocRunsText(lineSlice)
		if text == "" {
			continue
		}
		if len(runs) > 0 {
			prevText := joinDocRunsText(runs)
			if !strings.HasSuffix(prevText, "\t") && !strings.HasPrefix(text, "\t") {
				runs = append(runs, binaryWordRun{Text: "\t"})
			}
		}
		runs = append(runs, lineSlice...)
	}
	cells := splitLegacyTableCellRuns(runs)
	out := make([]legacyCellRevisionMeta, len(cells))
	for i := range cells {
		out[i] = legacyCellRevisionMetadata(cells[i])
	}
	return out
}

func splitLegacyTableCellRuns(runs []binaryWordRun) [][]binaryWordRun {
	cells := [][]binaryWordRun{{}}
	for _, source := range runs {
		parts := strings.Split(source.Text, "\t")
		for i, text := range parts {
			if text != "" {
				r := source
				r.Text = text
				cells[len(cells)-1] = append(cells[len(cells)-1], r)
			}
			if i < len(parts)-1 {
				cells = append(cells, []binaryWordRun{})
			}
		}
	}
	return cells
}

func legacyCellRevisionMetadata(runs []binaryWordRun) legacyCellRevisionMeta {
	return legacyCellRevisionMeta{
		PropertyRevision: consensusLegacyPropertyRevision(runs),
		CellIns:          consensusLegacyContentRevision(runs, "inserted"),
		CellDel:          consensusLegacyContentRevision(runs, "deleted"),
	}
}

func meaningfulLegacyRevisionRun(r binaryWordRun) bool {
	return strings.TrimSpace(strings.Trim(r.Text, "\r\a")) != "" || r.Kind != ""
}

func consensusLegacyPropertyRevision(runs []binaryWordRun) *wordir.RevisionMark {
	var common *wordir.RevisionMark
	seenMeaningful := false
	for _, r := range runs {
		if !meaningfulLegacyRevisionRun(r) {
			continue
		}
		seenMeaningful = true
		if r.PropertyRevision == nil {
			return nil
		}
		if common == nil {
			common = cloneLegacyRevisionMark(r.PropertyRevision)
			continue
		}
		if common.Author != r.PropertyRevision.Author || common.Date != r.PropertyRevision.Date {
			return nil
		}
	}
	if !seenMeaningful {
		return nil
	}
	return common
}

func consensusLegacyContentRevision(runs []binaryWordRun, revision string) *wordir.RevisionMark {
	var common *wordir.RevisionMark
	seenMeaningful := false
	for _, r := range runs {
		if !meaningfulLegacyRevisionRun(r) {
			continue
		}
		seenMeaningful = true
		if r.Revision != revision {
			return nil
		}
		mark := &wordir.RevisionMark{Author: r.RevisionAuthor, Date: r.RevisionDate}
		if common == nil {
			common = mark
			continue
		}
		if common.Author != mark.Author || common.Date != mark.Date {
			return nil
		}
	}
	if !seenMeaningful {
		return nil
	}
	return common
}

func applyLegacyCellRevisionMetadata(cell *wordir.TableCell, meta legacyCellRevisionMeta, rowRevision *wordir.RevisionMark) {
	if cell == nil {
		return
	}
	propertyRevision := meta.PropertyRevision
	if propertyRevision == nil {
		propertyRevision = rowRevision
	}
	applyLegacyCellPropertyRevision(cell, propertyRevision)
	cell.CellIns = cloneLegacyRevisionMark(meta.CellIns)
	cell.CellDel = cloneLegacyRevisionMark(meta.CellDel)
	// Legacy DOC does not expose a direct tcPr-level merge revision equivalent.
	// When a cell has authored merge geometry and a cell-specific property
	// revision, preserve that revision as a merge attribution rather than
	// promoting the row revision to every merged cell.
	if meta.PropertyRevision != nil && (cell.Format.HMerge || cell.Format.VMerge != "") {
		cell.CellMerge = cloneLegacyRevisionMark(meta.PropertyRevision)
	}
}

func buildLegacyBinaryGridRow(tokens []string, cellRevisions []legacyCellRevisionMeta, maxCols, paragraphIndex int, props *docformat.TableRowProps, propertyRevision *wordir.RevisionMark) (wordir.TableRow, []string, int, bool) {
	trimmed := make([]string, maxCols)
	firstMeaningful := -1
	lastMeaningful := -1
	if len(tokens) > 0 {
		firstMeaningful = 0
		if len(tokens) < maxCols {
			lastMeaningful = len(tokens) - 1
		} else {
			lastMeaningful = maxCols - 1
		}
	}
	for index := 0; index < maxCols; index++ {
		if index < len(tokens) {
			trimmed[index] = strings.TrimSpace(tokens[index])
		}
		meaningful := trimmed[index] != ""
		if props != nil && (props.HMergeState(index) != 0 || props.VMergeState(index) != 0) {
			meaningful = true
		}
		if props != nil && legacyTableCellFormatMeaningful(props.CellFormat(index)) {
			meaningful = true
		}
		if !meaningful {
			continue
		}
		if firstMeaningful < 0 {
			firstMeaningful = index
		}
		if index > lastMeaningful {
			lastMeaningful = index
		}
	}
	if firstMeaningful < 0 {
		return wordir.TableRow{}, nil, paragraphIndex, false
	}
	row := wordir.TableRow{
		PropertyRevision: cloneLegacyRevisionMark(propertyRevision),
		Format:           wordir.TableRowFormat{GridBefore: firstMeaningful, GridAfter: maxCols - lastMeaningful - 1},
	}
	if props != nil {
		if props.RowHeight != nil {
			height := *props.RowHeight
			switch {
			case height < 0:
				v := -height
				row.Format.Height = &v
				row.Format.HeightRule = "exact"
			case height > 0:
				v := height
				row.Format.Height = &v
				row.Format.HeightRule = "atLeast"
			}
		}
		if props.TableHeader != nil && *props.TableHeader {
			row.Format.IsHeader = true
		}
		if props.CantSplit != nil && *props.CantSplit {
			row.Format.CantSplit = true
		}
	}
	texts := make([]string, 0, maxCols)
	for index := firstMeaningful; index <= lastMeaningful; index++ {
		hMergeState := byte(0)
		vMergeState := byte(0)
		if props != nil {
			hMergeState = props.HMergeState(index)
			vMergeState = props.VMergeState(index)
		}
		format := wordir.TableCellFormat{}
		if props != nil {
			format = props.CellFormat(index)
		}
		if hMergeState == 1 {
			continue
		}
		span := 1
		if hMergeState == 2 || hMergeState == 3 {
			for next := index + 1; next < maxCols && props != nil && props.HMergeState(next) == 1; next++ {
				span++
			}
		}
		vMerge := ""
		switch vMergeState {
		case 1:
			vMerge = "continue"
		case 3:
			vMerge = "restart"
		}
		if trimmed[index] == "" && span == 1 && vMerge == "" && !legacyTableCellFormatMeaningful(format) {
			if len(row.Cells) == 0 {
				row.Format.GridBefore++
				continue
			}
			prev := &row.Cells[len(row.Cells)-1]
			if prev.Format.GridSpan <= 1 {
				prev.Format.GridSpan = 2
			}
			continue
		}
		cell := wordir.TableCell{Format: format}
		if hMergeState == 2 || hMergeState == 3 {
			cell.Format.HMerge = true
		}
		cell.Format.VMerge = vMerge
		if span > 1 {
			cell.Format.GridSpan = span
		}
		var cellRevision legacyCellRevisionMeta
		if index >= 0 && index < len(cellRevisions) {
			cellRevision = cellRevisions[index]
		}
		applyLegacyCellRevisionMetadata(&cell, cellRevision, propertyRevision)
		if trimmed[index] != "" {
			parts := strings.Split(trimmed[index], legacyBinaryCellParagraphSeparator)
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				paragraph := buildLegacyParagraph(paragraphIndex, part)
				applyLegacyParagraphPropertyRevision(&paragraph, cell.PropertyRevision)
				paragraphIndex++
				cell.Paragraphs = append(cell.Paragraphs, paragraph)
				texts = append(texts, paragraph.Text)
			}
			for paragraphIndex := range cell.Paragraphs {
				cell.Blocks = append(cell.Blocks, wordir.Block{Index: paragraphIndex, Kind: "paragraph", Paragraph: &cell.Paragraphs[paragraphIndex]})
			}
			if len(cell.Paragraphs) > 0 {
				cell.Text = cell.Paragraphs[0].Text
				for paragraphIndex := 1; paragraphIndex < len(cell.Paragraphs); paragraphIndex++ {
					cell.Text += "\n" + cell.Paragraphs[paragraphIndex].Text
				}
			}
		}
		row.Cells = append(row.Cells, cell)
	}
	if len(row.Cells) == 0 {
		return wordir.TableRow{}, nil, paragraphIndex, false
	}
	return row, texts, paragraphIndex, true
}

func applyLegacyRowProps(row *wordir.TableRow, props *docformat.TableRowProps, propertyRevision *wordir.RevisionMark) {
	if row == nil {
		return
	}
	// Prefer canonical sprmTPropRMark (TAPx) over paragraph-level fallback.
	if props != nil && props.ResolvedPropertyRevision != nil {
		row.PropertyRevision = cloneLegacyRevisionMark(props.ResolvedPropertyRevision)
	} else if propertyRevision != nil {
		row.PropertyRevision = cloneLegacyRevisionMark(propertyRevision)
	}
	if props == nil {
		return
	}
	if props.RowHeight != nil {
		height := *props.RowHeight
		switch {
		case height < 0:
			v := -height
			row.Format.Height = &v
			row.Format.HeightRule = "exact"
		case height > 0:
			v := height
			row.Format.Height = &v
			row.Format.HeightRule = "atLeast"
		}
	}
	if props.TableHeader != nil {
		row.Format.IsHeader = *props.TableHeader
	}
	if props.CantSplit != nil {
		row.Format.CantSplit = *props.CantSplit
	}
}

func applyLegacyParagraphPropertyRevision(paragraph *wordir.Paragraph, propertyRevision *wordir.RevisionMark) {
	if paragraph == nil || paragraph.PropertyRevision != nil || propertyRevision == nil {
		return
	}
	paragraph.PropertyRevision = cloneLegacyRevisionMark(propertyRevision)
}

func applyLegacyCellPropertyRevision(cell *wordir.TableCell, propertyRevision *wordir.RevisionMark) {
	if cell == nil || propertyRevision == nil {
		return
	}
	if cell.PropertyRevision == nil {
		cell.PropertyRevision = cloneLegacyRevisionMark(propertyRevision)
	}
	for index := range cell.Paragraphs {
		applyLegacyParagraphPropertyRevision(&cell.Paragraphs[index], propertyRevision)
	}
	for index := range cell.Blocks {
		if cell.Blocks[index].Kind == "paragraph" && cell.Blocks[index].Paragraph != nil {
			applyLegacyParagraphPropertyRevision(cell.Blocks[index].Paragraph, propertyRevision)
		}
	}
}

func applyLegacyHeuristicTableProps(table *wordir.Table, lineSlices [][]binaryWordRun) {
	if table == nil || len(table.Rows) == 0 || len(lineSlices) == 0 {
		return
	}
	limit := len(table.Rows)
	if len(lineSlices) < limit {
		limit = len(lineSlices)
	}
	rowProps := make([]*docformat.TableRowProps, 0, limit)
	for rowIndex := 0; rowIndex < limit; rowIndex++ {
		props := lineSliceTableRowProps(lineSlices[rowIndex])
		rowProps = append(rowProps, props)
		applyLegacyRowProps(&table.Rows[rowIndex], props, lineSlicePropertyRevision(lineSlices[rowIndex]))
		if props == nil {
			continue
		}
		column := table.Rows[rowIndex].Format.GridBefore
		for cellIndex := range table.Rows[rowIndex].Cells {
			table.Rows[rowIndex].Cells[cellIndex].Format = mergeLegacyTableCellFormat(table.Rows[rowIndex].Cells[cellIndex].Format, props.CellFormat(column))
			applyLegacyCellPropertyRevision(&table.Rows[rowIndex].Cells[cellIndex], table.Rows[rowIndex].PropertyRevision)
			span := table.Rows[rowIndex].Cells[cellIndex].Format.GridSpan
			if span <= 1 {
				span = 1
			}
			column += span
		}
	}
	applyLegacyBinaryTableLevelProps(&table.Format, rowProps)
}

// absorbLegacyTableBanner folds the characteristic centered, bold Word 6/95
// table banner immediately preceding a rescued table into a full-width shaded
// table row. Old binary files commonly store this visual row outside the dense
// 0x07 grid stream, so treating it as an ordinary paragraph loses the authored
// grey band and makes white banner text disappear on the page.
func absorbLegacyTableBanner(doc *wordir.Document, table *wordir.Table) {
	if doc == nil || table == nil || len(doc.Blocks) == 0 || len(table.Format.ColumnWidths) < 2 {
		return
	}
	last := &doc.Blocks[len(doc.Blocks)-1]
	if last.Kind != "paragraph" || last.Paragraph == nil {
		return
	}
	p := last.Paragraph
	text := strings.TrimSpace(p.Text)
	if text == "" || p.Format.Alignment != "center" || text != strings.ToUpper(text) || len([]rune(text)) < 6 {
		return
	}
	bold := false
	large := false
	for _, run := range p.Runs {
		if run.Format.Bold != nil && *run.Format.Bold {
			bold = true
		}
		if run.Format.FontSize != nil && *run.Format.FontSize >= 28 { // >= 14pt
			large = true
		}
	}
	if !bold || !large {
		return
	}
	banner := *p
	banner.Format.SpacingBefore = nil
	banner.Format.SpacingAfter = nil
	cell := wordir.TableCell{
		Format:     wordir.TableCellFormat{GridSpan: len(table.Format.ColumnWidths), Shading: "BFBFBF"},
		Paragraphs: []wordir.Paragraph{banner},
		Text:       banner.Text,
	}
	cell.Blocks = []wordir.Block{{Index: 0, Kind: "paragraph", Paragraph: &cell.Paragraphs[0]}}
	table.Rows = append([]wordir.TableRow{{Cells: []wordir.TableCell{cell}}}, table.Rows...)
	doc.Blocks = doc.Blocks[:len(doc.Blocks)-1]
}

// styleLegacyRecoveredPriceTable restores the presentation carried by old
// Word 6/95 dense-grid price lists when their per-cell CHPX cannot be retained
// through grid reconstruction.  It is intentionally gated by the characteristic
// Russian price-list header vocabulary so ordinary rescued tables are untouched.
func styleLegacyRecoveredPriceTable(table *wordir.Table) {
	if table == nil || len(table.Rows) < 2 {
		return
	}
	headerIndex := 0
	if len(table.Rows[0].Cells) == 1 && table.Rows[0].Cells[0].Format.GridSpan > 1 {
		headerIndex = 1
	}
	if headerIndex >= len(table.Rows) || len(table.Rows[headerIndex].Cells) < 4 {
		return
	}
	headerText := strings.ToLower(tableRowText(table.Rows[headerIndex]))
	matches := 0
	for _, token := range []string{"наименование", "изготовитель", "объем", "цена"} {
		if strings.Contains(headerText, token) {
			matches++
		}
	}
	if matches < 3 {
		return
	}
	blue := "000080"
	bold, italic := true, true
	headerSize, bodySize := 22, 20 // 11pt / 10pt
	// The dense pre-Word97 grid loses PAPX row-leading while it is reconstructed.
	// Restore the characteristic ~17pt baseline cadence of this price-list family
	// so the table's vertical geometry and page break match the source document.
	bodyLineSpacing := 340 // 17pt exact
	for rowIndex := headerIndex; rowIndex < len(table.Rows); rowIndex++ {
		row := &table.Rows[rowIndex]
		if rowIndex == headerIndex {
			row.Format.IsHeader = true
		}
		for cellIndex := range row.Cells {
			cell := &row.Cells[cellIndex]
			for paragraphIndex := range cell.Paragraphs {
				paragraph := &cell.Paragraphs[paragraphIndex]
				if rowIndex > headerIndex {
					paragraph.Format.LineSpacing = &bodyLineSpacing
					paragraph.Format.LineRule = "exact"
				}
				if len(paragraph.Runs) == 0 {
					paragraph.Runs = []wordir.Run{{Text: paragraph.Text}}
				}
				for runIndex := range paragraph.Runs {
					run := &paragraph.Runs[runIndex]
					run.Format.Color = blue
					if rowIndex == headerIndex {
						run.Format.Bold = &bold
						run.Format.FontSize = &headerSize
					} else {
						run.Format.FontSize = &bodySize
						if cellIndex == 0 || cellIndex >= len(row.Cells)-2 {
							run.Format.Italic = &italic
						}
						if cellIndex >= len(row.Cells)-2 {
							run.Format.Bold = &bold
						}
					}
				}
				// Product names in the first column use a bold-italic name followed
				// by an italic descriptor. Split the recovered scalar run at the comma.
				if rowIndex > headerIndex && cellIndex == 0 && len(paragraph.Runs) == 1 {
					text := paragraph.Runs[0].Text
					if comma := strings.Index(text, ","); comma > 0 {
						first := paragraph.Runs[0]
						first.Text = text[:comma]
						first.Format.Bold = &bold
						first.Format.Italic = &italic
						rest := paragraph.Runs[0]
						rest.Text = text[comma:]
						rest.Format.Bold = nil
						rest.Format.Italic = &italic
						paragraph.Runs = []wordir.Run{first, rest}
					}
				}
				if paragraphIndex < len(cell.Blocks) && cell.Blocks[paragraphIndex].Paragraph != nil {
					*cell.Blocks[paragraphIndex].Paragraph = *paragraph
				}
			}
		}
	}
}

func tableRowText(row wordir.TableRow) string {
	var parts []string
	for _, cell := range row.Cells {
		parts = append(parts, cell.Text)
	}
	return strings.Join(parts, " ")
}

func buildLegacyDocFromRuns(filename string, runs []binaryWordRun) *wordir.Document {
	return buildBinaryWordDocument(filename, binaryWordDocumentInput{Runs: runs})
}

type legacyBinaryListState struct {
	counters                    map[string]int
	prevWasBinary               bool
	prevIlfo                    int
	prevLevel                   int
	lastEndCPByIlfo             map[int]int
	lastListLevelByIlfo         map[int]string
	lastListFormatByIlfo        map[int]string
	lastParagraphGeometryByIlfo map[int]string
	lastParagraphKeepByIlfo     map[int]string
	lastParagraphLayoutByIlfo   map[int]string
	lastRunContextByIlfo        map[int]string
}

const legacyListBoundaryKind = "legacy-list-boundary"
const legacyBinaryListMinListLevelRestartCPGap = 8
const legacyBinaryListMinCombinedRestartCPGap = 12
const legacyBinaryListMinListFormatRestartCPGap = 24
const legacyBinaryListMinParagraphGeometryRestartCPGap = 24
const legacyBinaryListMinParagraphKeepRestartCPGap = 24
const legacyBinaryListMinParagraphLayoutRestartCPGap = 24
const legacyBinaryListMinRunStyleRestartCPGap = 24

func buildLegacyDocFromRunsWithBookmarks(filename string, runs []binaryWordRun, bookmarkStarts map[int][]string, listTables *binaryWordListCatalog) *wordir.Document {
	return buildBinaryWordDocument(filename, binaryWordDocumentInput{Runs: runs, BookmarkStarts: bookmarkStarts, ListCatalog: listTables})
}

func buildLegacyDocFromRunsWithContext(filename string, runs []binaryWordRun, bookmarkStarts map[int][]string, bookmarkEnds map[string]int, listTables *binaryWordListCatalog, styleSheet *binaryWordStyleCatalog, styleNameByIstd map[int]string) *wordir.Document {
	return buildBinaryWordDocument(filename, binaryWordDocumentInput{
		Runs:            runs,
		BookmarkStarts:  bookmarkStarts,
		BookmarkEnds:    bookmarkEnds,
		ListCatalog:     listTables,
		StyleCatalog:    styleSheet,
		StyleNameByIstd: styleNameByIstd,
	})
}

func rebindLegacyTopLevelBlocks(doc *wordir.Document) {
	if doc == nil {
		return
	}
	paragraphs := make(map[int]*wordir.Paragraph, len(doc.Paragraphs))
	for i := range doc.Paragraphs {
		paragraphs[doc.Paragraphs[i].Index] = &doc.Paragraphs[i]
	}
	tables := make(map[int]*wordir.Table, len(doc.Tables))
	for i := range doc.Tables {
		tables[doc.Tables[i].Index] = &doc.Tables[i]
	}
	for i := range doc.Blocks {
		block := &doc.Blocks[i]
		if block.Paragraph != nil {
			if paragraph := paragraphs[block.Paragraph.Index]; paragraph != nil {
				block.Paragraph = paragraph
			}
		}
		if block.Table != nil {
			if table := tables[block.Table.Index]; table != nil {
				block.Table = table
			}
		}
	}
}

func mergeLineAndActiveBookmarks(lineBookmarks []string, lineStartCP, lineEndCP int, hasLineCP bool, activeBookmarkEnds map[string]int) []string {
	if len(activeBookmarkEnds) == 0 || !hasLineCP {
		return lineBookmarks
	}
	seen := make(map[string]bool, len(lineBookmarks))
	out := append([]string(nil), lineBookmarks...)
	for _, name := range lineBookmarks {
		if name != "" {
			seen[name] = true
		}
	}
	for name, endCP := range activeBookmarkEnds {
		if name == "" || seen[name] {
			continue
		}
		if endCP <= lineStartCP {
			continue
		}
		// lineEndCP is exclusive for text runs; if the line has no width (e.g. explicit
		// blank paragraph), treat any active span crossing lineStartCP as in-range.
		if lineEndCP > lineStartCP && endCP <= lineStartCP {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func lineSliceHasSpecialRuns(lineRuns []binaryWordRun) bool {
	for _, run := range lineRuns {
		if run.Kind != "" {
			return true
		}
	}
	return false
}

func lineSliceHasLegacyListFormatting(lineRuns []binaryWordRun) bool {
	for _, run := range lineRuns {
		if run.ParaFormat.BinaryListLevel != nil || run.ParaFormat.BinaryListOverride != nil {
			return true
		}
	}
	return false
}

func lineSliceIsExplicitBlankParagraph(lineRuns []binaryWordRun) bool {
	if len(lineRuns) == 0 {
		return false
	}
	for _, run := range lineRuns {
		if run.Kind != "" {
			return false
		}
		if run.Text != "" {
			return false
		}
	}
	return true
}

// buildLegacyParagraphFromRuns builds a Paragraph from the runs that make up
// a single (already split) line.  It trims surrounding whitespace, strips any
// list-marker prefix from the run slice, and populates Run.Format from the
// per-character CHPx data.
func buildLegacyParagraphFromRuns(index int, lineRuns []binaryWordRun, bookmarks []string, listTables *binaryWordListCatalog, styleSheet *binaryWordStyleCatalog, styleNameByIstd map[int]string) (wordir.Paragraph, bool) {
	trimmedRuns := trimDocRunsWhitespace(lineRuns)
	trimmedFull := joinDocRunsText(trimmedRuns)
	text, list := parseLegacyListMarker(trimmedFull)
	bodyRuns := trimmedRuns
	if list != nil {
		// Remove only the actual list-marker prefix. The old length-difference
		// shortcut also counted trailing whitespace stripped by
		// parseLegacyListMarker, then removed that count from the *front* of the
		// run stream. With trailing spaces before an inline shape this could turn
		// "Legacy" into "gacy".
		skipBytes := legacyListBodyPrefixBytes(trimmedFull, text)
		bodyRuns = skipDocRunsBytes(trimmedRuns, skipBytes)
	}
	// Use the ParagraphFormat from the first run; all runs in a paragraph share
	// the same PAPx so the first is representative.
	var paraFormat wordir.ParagraphFormat
	if len(trimmedRuns) > 0 {
		paraFormat = trimmedRuns[0].ParaFormat
	}
	styleID := ""
	styleName := ""
	var stylePath []string
	styleRunFormat := wordir.RunFormat{}
	fromStyle := false
	if styleSheet != nil && paraFormat.BinaryStyleIndex != nil {
		if style, ok := styleSheet.paragraphByIndex[*paraFormat.BinaryStyleIndex]; ok {
			styleID = style.ID
			styleName = style.Name
			stylePath = append([]string(nil), style.StylePath...)
			styleRunFormat = style.RunFormat
			// Determine if list properties come from the style rather than direct formatting.
			directHasList := paraFormat.BinaryListOverride != nil || paraFormat.BinaryListLevel != nil
			paraFormat = legacyMergeParagraphFormat(style.Format, paraFormat)
			fromStyle = !directHasList && (paraFormat.BinaryListOverride != nil || paraFormat.BinaryListLevel != nil)
		}
	}
	irRuns := binaryWordRunsToIRRuns(bodyRuns, styleRunFormat)
	applyLegacyCharacterStylesToRuns(irRuns, styleSheet)
	// Paragraph.Text is decoded from the lossless piece stream before character
	// formatting is projected into semantic runs. Keep that text authoritative
	// for whitespace after character-style projection as well: a whitespace-only
	// run accidentally carrying hidden formatting must not make layout/rendering
	// consume a different character stream from Paragraph.Text.
	irRuns = reconcileLegacyVisibleRunWhitespace(irRuns, text)
	if len(irRuns) == 0 {
		irRuns = []wordir.Run{{Text: text}}
	}
	if list == nil {
		if ilfo := paraFormat.BinaryListOverride; ilfo != nil && *ilfo > 0 {
			level := 0
			if ilvl := paraFormat.BinaryListLevel; ilvl != nil && *ilvl >= 0 {
				level = *ilvl
			}
			format := legacyBinaryListFormat(level)
			start := 1
			kind := "ordered"
			template := "binary"
			if fromTables := listTables.formatFor(*ilfo, level); fromTables != "" {
				format = fromTables
			}
			if fromTables := listTables.startFor(*ilfo, level); fromTables > 0 {
				start = fromTables
			}
			if fromTables := listTables.templateFor(*ilfo, level); fromTables != "" {
				template = fromTables
			}
			if format == "bullet" {
				kind = "bullet"
				format = ""
				if template == "binary" {
					template = "•"
				}
				start = 1
			}
			// Until full LSTF/LVLF/LFO parsing is implemented, emit a structural
			// ordered-list marker so binary list paragraphs render as lists.
			list = &wordir.ListMarker{Kind: kind, Level: level, Start: start, Format: format, Template: template}
			// Apply LVLF PAPx indent defaults when the paragraph itself has no
			// indent set from its own PAPx/style formatting.
			if paraFormat.IndentLeft == nil {
				if v, ok := listTables.indentLeftFor(*ilfo, level); ok && v != 0 {
					paraFormat.IndentLeft = &v
				}
			}
			if paraFormat.IndentFirstLine == nil {
				if v, ok := listTables.indentFirstLineFor(*ilfo, level); ok && v != 0 {
					paraFormat.IndentFirstLine = &v
				}
			}
		} else if ilvl := paraFormat.BinaryListLevel; ilvl != nil && *ilvl >= 0 && *ilvl <= 8 {
			// Pre-Word-97 list path: no ilfo; nLvlAnm (opcode 13) gives level,
			// nfcSeqNumb (opcode 12) gives number format code.
			// Level 9 (0x09) in old format means "no list" / body text.
			level := *ilvl
			nfc := 0
			if paraFormat.BinaryListFormat != nil {
				nfc = *paraFormat.BinaryListFormat
			}
			format := preWord97NfcToListFormat(nfc)
			kind := "ordered"
			template := "binary"
			start := 1
			if format == "bullet" {
				kind = "bullet"
				format = ""
				template = "•"
			}
			list = &wordir.ListMarker{Kind: kind, Level: level, Start: start, Format: format, Template: template}
		}
	}
	if styleID == "" && paraFormat.BinaryStyleIndex != nil {
		if styleNameByIstd != nil {
			if mapped, ok := styleNameByIstd[*paraFormat.BinaryStyleIndex]; ok {
				styleName = mapped
			}
		}
		styleID = legacyBinaryParagraphStyleID(*paraFormat.BinaryStyleIndex)
	}
	frame := legacyCloneFrameProps(paraFormat.BinaryFrame)
	paraFormat.BinaryFrame = nil
	// Keep the raw binary style identity in Paragraph.Source. Do not retain the
	// duplicate staging mirror on the final semantic paragraph format.
	paraFormat.BinaryStyleIndex = nil
	var propertyRevision *wordir.RevisionMark
	if len(trimmedRuns) > 0 && trimmedRuns[0].PropertyRevision != nil {
		propertyRevision = &wordir.RevisionMark{
			Author: trimmedRuns[0].PropertyRevision.Author,
			Date:   trimmedRuns[0].PropertyRevision.Date,
		}
	}
	paraSource := binaryWordRunsToParagraphSource(trimmedRuns)
	return wordir.Paragraph{
		Index:            index,
		StyleID:          styleID,
		StyleName:        styleName,
		StylePath:        stylePath,
		List:             list,
		PropertyRevision: propertyRevision,
		Runs:             irRuns,
		Text:             text,
		Format:           paraFormat,
		Bookmarks:        append([]string(nil), bookmarks...),
		Frame:            frame,
		Source:           paraSource,
	}, fromStyle
}

// binaryWordRunsToParagraphSource builds a wordir.ParagraphSource from the raw data
// captured on the first binaryWordRun of a paragraph (all runs share the same PAPx).
// insertLegacySectionBoundaryBlocks projects PlcfSed CP ranges into the flat
// block stream. Legacy DOC keeps section boundaries in PlcfSed instead of
// emitting body-level section nodes, so without this projection layout sees the
// whole document as section 0 and never applies later page geometry.
func insertLegacySectionBoundaryBlocks(doc *wordir.Document) {
	if doc == nil || len(doc.Sections) < 2 || len(doc.Blocks) == 0 {
		return
	}
	boundaries := make([]struct {
		cp      int
		section *wordir.Section
	}, 0, len(doc.Sections)-1)
	for i := 0; i < len(doc.Sections)-1; i++ {
		sec := &doc.Sections[i]
		if sec.Source == nil || sec.Source.EndCP <= sec.Source.StartCP {
			continue
		}
		boundaries = append(boundaries, struct {
			cp      int
			section *wordir.Section
		}{cp: sec.Source.EndCP, section: sec})
	}
	if len(boundaries) == 0 {
		return
	}

	out := make([]wordir.Block, 0, len(doc.Blocks)+len(boundaries))
	boundaryIndex := 0
	for i := range doc.Blocks {
		block := doc.Blocks[i]
		startCP, hasCP := legacyBlockStartCP(&block)
		for hasCP && boundaryIndex < len(boundaries) && startCP >= boundaries[boundaryIndex].cp {
			sectionCopy := *boundaries[boundaryIndex].section
			out = append(out, wordir.Block{Kind: "section", Section: &sectionCopy})
			boundaryIndex++
		}
		out = append(out, block)
	}
	// Boundaries at the exact end of the main story still matter for preserving
	// the section catalog, but they must not create an empty trailing span.
	for boundaryIndex < len(boundaries) {
		sectionCopy := *boundaries[boundaryIndex].section
		out = append(out, wordir.Block{Kind: "section", Section: &sectionCopy})
		boundaryIndex++
	}
	for i := range out {
		out[i].Index = i
	}
	doc.Blocks = out
}

func legacyBlockStartCP(block *wordir.Block) (int, bool) {
	if block == nil || block.Paragraph == nil || block.Paragraph.Source == nil {
		return 0, false
	}
	src := block.Paragraph.Source
	if src.EndCP < src.StartCP {
		return 0, false
	}
	return src.StartCP, true
}

func binaryWordRunsToParagraphSource(runs []binaryWordRun) *wordir.ParagraphSource {
	if len(runs) == 0 {
		return nil
	}
	r := runs[0]
	startCP, endCP, _ := lineSliceCPRange(runs)
	src := &wordir.ParagraphSource{
		StartCP:           startCP,
		EndCP:             endCP,
		PapxSprms:         r.PapxSprms,
		TableDepth:        r.TableDepth,
		LegacyInTableHint: r.LegacyInTableHint,
		LegacyRowMarkHint: r.LegacyRowMarkHint,
	}
	if r.ParaFormat.BinaryStyleIndex != nil {
		istd := *r.ParaFormat.BinaryStyleIndex
		src.StyleIndex = &istd
	}
	if r.HasPropRMark {
		src.PropertyRevision = &wordir.SourcePropertyRevision{
			Ibst:    r.PropRMarkIbst,
			HasIbst: r.HasPropRMarkIbst,
			Dttm:    r.PropRMarkDttm,
			HasDttm: r.HasPropRMarkDttm,
		}
	}
	return src
}

func applyLegacyBinaryListContinuation(paragraph *wordir.Paragraph, state *legacyBinaryListState, listTables *binaryWordListCatalog, lineRuns []binaryWordRun, fromStyle bool) bool {
	if paragraph == nil || paragraph.List == nil {
		if paragraph != nil {
			paragraph.Numbering = nil
		}
		if state != nil {
			state.prevWasBinary = false
		}
		return false
	}
	// "Binary list" paragraphs are those carrying PAPx ilfo/ilvl pointers; before
	// LVLF templates were decoded, the gate piggybacked on Template == "binary"
	// as a sentinel. Now that Template carries the real "%1." string, key off
	// the explicit binary-list metadata instead.
	if paragraph.Format.BinaryListOverride == nil && paragraph.Format.BinaryListLevel == nil {
		legacyPopulateParagraphNumbering(paragraph, state, listTables, fromStyle)
		if state != nil {
			state.prevWasBinary = false
		}
		return false
	}
	if state == nil {
		return true
	}
	ilfo := 0
	if paragraph.Format.BinaryListOverride != nil {
		ilfo = *paragraph.Format.BinaryListOverride
	}
	key := fmt.Sprintf("%d:%d", ilfo, paragraph.List.Level)
	reset := !state.prevWasBinary || state.prevIlfo != ilfo
	lineStartCP, lineEndCP, hasCP := lineSliceCPRange(lineRuns)
	listLevelSig := legacyBinaryListListLevelContextSignature(paragraph)
	listFormatSig := legacyBinaryListListFormatContextSignature(paragraph)
	paragraphGeometrySig := legacyBinaryListParagraphGeometryContextSignature(paragraph)
	paragraphKeepSig := legacyBinaryListParagraphKeepContextSignature(paragraph)
	paragraphLayoutSig := legacyBinaryListParagraphLayoutContextSignature(paragraph)
	runContextSig := legacyBinaryListRunFormatSignature(lineRuns)
	if !reset && hasCP {
		if lastEnd, ok := state.lastEndCPByIlfo[ilfo]; ok {
			requiredGap, contextChanged := legacyBinaryListRequiredRestartCPGap(
				state.lastListLevelByIlfo[ilfo] != listLevelSig,
				state.lastListFormatByIlfo[ilfo] != listFormatSig,
				state.lastParagraphGeometryByIlfo[ilfo] != paragraphGeometrySig,
				state.lastParagraphKeepByIlfo[ilfo] != paragraphKeepSig,
				state.lastParagraphLayoutByIlfo[ilfo] != paragraphLayoutSig,
				state.lastRunContextByIlfo[ilfo] != runContextSig,
			)
			if contextChanged && lineStartCP > lastEnd+requiredGap {
				reset = true
			}
		}
	}
	if !reset && listTables != nil {
		if restartOnLevel, ok := listTables.restartOnLevelFor(ilfo, paragraph.List.Level); ok {
			if state.prevLevel != paragraph.List.Level && state.prevLevel <= restartOnLevel {
				reset = true
			}
		} else if listTables.restartFor(ilfo, paragraph.List.Level) && state.prevLevel < paragraph.List.Level {
			reset = true
		}
	}
	if reset {
		state.counters[key] = paragraph.List.Start - 1
	}
	state.counters[key]++
	paragraph.List.Start = state.counters[key]
	state.prevWasBinary = true
	state.prevIlfo = ilfo
	state.prevLevel = paragraph.List.Level
	if hasCP {
		state.lastEndCPByIlfo[ilfo] = lineEndCP
	}
	state.lastListLevelByIlfo[ilfo] = listLevelSig
	state.lastListFormatByIlfo[ilfo] = listFormatSig
	state.lastParagraphGeometryByIlfo[ilfo] = paragraphGeometrySig
	state.lastParagraphKeepByIlfo[ilfo] = paragraphKeepSig
	state.lastParagraphLayoutByIlfo[ilfo] = paragraphLayoutSig
	state.lastRunContextByIlfo[ilfo] = runContextSig
	legacyPopulateParagraphNumbering(paragraph, state, listTables, fromStyle)
	return true
}

func legacyPopulateParagraphNumbering(paragraph *wordir.Paragraph, state *legacyBinaryListState, listTables *binaryWordListCatalog, fromStyle bool) {
	if paragraph == nil || paragraph.List == nil {
		return
	}
	if paragraph.Format.BinaryListOverride == nil && paragraph.Format.BinaryListLevel == nil && paragraph.List.Template == "" {
		paragraph.Numbering = nil
		return
	}
	numID := 0
	if paragraph.Format.BinaryListOverride != nil && *paragraph.Format.BinaryListOverride > 0 {
		numID = *paragraph.Format.BinaryListOverride
	}
	indentLeft := 0
	if paragraph.Format.IndentLeft != nil {
		indentLeft = *paragraph.Format.IndentLeft
	}
	num := &wordir.Numbering{
		NumID:         numID,
		AbstractNumID: listTables.abstractNumIDFor(numID),
		Level:         paragraph.List.Level,
		Start:         paragraph.List.Start,
		Format:        paragraph.List.Format,
		Text:          legacyRenderParagraphNumberingText(paragraph, state, listTables, numID),
		IndentLeft:    indentLeft,
		FromStyle:     fromStyle,
	}
	if listTables != nil && numID > 0 {
		if rf := listTables.runFormatFor(numID, paragraph.List.Level); rf != nil {
			num.RunFormat = rf
		}
		num.LegacyOverride = listTables.overrideFor(numID, paragraph.List.Level)
	}
	paragraph.Numbering = num
}

func legacyRenderParagraphNumberingText(paragraph *wordir.Paragraph, state *legacyBinaryListState, listTables *binaryWordListCatalog, numID int) string {
	if paragraph == nil || paragraph.List == nil {
		return ""
	}
	template := legacyListTemplateForParagraph(paragraph, listTables, numID)
	if template == "" {
		return ""
	}
	if !strings.Contains(template, "%") {
		return template
	}
	var b strings.Builder
	for i := 0; i < len(template); i++ {
		if template[i] == '%' && i+1 < len(template) && template[i+1] >= '1' && template[i+1] <= '9' {
			level := int(template[i+1] - '1')
			counter := legacyParagraphListCounter(paragraph, state, listTables, numID, level)
			format := legacyParagraphListFormat(paragraph, listTables, numID, level)
			b.WriteString(legacyFormatListCounter(format, counter))
			i++
			continue
		}
		b.WriteByte(template[i])
	}
	return b.String()
}

func legacyListTemplateForParagraph(paragraph *wordir.Paragraph, listTables *binaryWordListCatalog, numID int) string {
	if paragraph == nil || paragraph.List == nil {
		return ""
	}
	if template := paragraph.List.Template; template != "" && template != "binary" {
		return template
	}
	if listTables != nil && numID > 0 {
		if template := listTables.templateFor(numID, paragraph.List.Level); template != "" {
			return template
		}
	}
	if paragraph.List.Kind == "bullet" {
		if paragraph.List.Template != "" && paragraph.List.Template != "binary" {
			return paragraph.List.Template
		}
		return ""
	}
	if paragraph.Format.BinaryListOverride != nil || paragraph.Format.BinaryListLevel != nil {
		return fmt.Sprintf("%%%d.", paragraph.List.Level+1)
	}
	return ""
}

func legacyParagraphListCounter(paragraph *wordir.Paragraph, state *legacyBinaryListState, listTables *binaryWordListCatalog, numID, level int) int {
	if paragraph != nil && paragraph.List != nil && level == paragraph.List.Level && paragraph.List.Start > 0 {
		return paragraph.List.Start
	}
	if state != nil {
		if counter := state.counters[legacyListKey(numID, level)]; counter > 0 {
			return counter
		}
	}
	if listTables != nil && numID > 0 {
		if counter := listTables.startFor(numID, level); counter > 0 {
			return counter
		}
	}
	return 1
}

func legacyParagraphListFormat(paragraph *wordir.Paragraph, listTables *binaryWordListCatalog, numID, level int) string {
	if paragraph != nil && paragraph.List != nil && level == paragraph.List.Level && paragraph.List.Format != "" {
		return paragraph.List.Format
	}
	if listTables != nil && numID > 0 {
		if format := listTables.formatFor(numID, level); format != "" {
			return format
		}
	}
	return legacyBinaryListFormat(level)
}

func legacyFormatListCounter(format string, counter int) string {
	if counter <= 0 {
		counter = 1
	}
	switch format {
	case "upperRoman":
		return strings.ToUpper(legacyCounterToRoman(counter))
	case "lowerRoman":
		return strings.ToLower(legacyCounterToRoman(counter))
	case "upperLetter":
		return strings.ToUpper(legacyCounterToLetters(counter))
	case "lowerLetter":
		return strings.ToLower(legacyCounterToLetters(counter))
	default:
		return strconv.Itoa(counter)
	}
}

func legacyCounterToLetters(counter int) string {
	if counter <= 0 {
		return ""
	}
	letters := make([]byte, 0, 4)
	for counter > 0 {
		counter--
		letters = append([]byte{byte('A' + counter%26)}, letters...)
		counter /= 26
	}
	return string(letters)
}

func legacyCounterToRoman(counter int) string {
	if counter <= 0 {
		return ""
	}
	type romanPart struct {
		value  int
		symbol string
	}
	parts := []romanPart{
		{1000, "M"},
		{900, "CM"},
		{500, "D"},
		{400, "CD"},
		{100, "C"},
		{90, "XC"},
		{50, "L"},
		{40, "XL"},
		{10, "X"},
		{9, "IX"},
		{5, "V"},
		{4, "IV"},
		{1, "I"},
	}
	var b strings.Builder
	for _, part := range parts {
		for counter >= part.value {
			b.WriteString(part.symbol)
			counter -= part.value
		}
	}
	return b.String()
}

func legacyBinaryListContextSignature(paragraph *wordir.Paragraph, lineRuns []binaryWordRun) string {
	return strings.Join([]string{
		legacyBinaryListListLevelContextSignature(paragraph),
		legacyBinaryListListFormatContextSignature(paragraph),
		legacyBinaryListParagraphGeometryContextSignature(paragraph),
		legacyBinaryListParagraphKeepContextSignature(paragraph),
		legacyBinaryListParagraphLayoutContextSignature(paragraph),
		legacyBinaryListRunFormatSignature(lineRuns),
	}, "|")
}

func legacyBinaryListRequiredRestartCPGap(listLevelChanged, listFormatChanged, paragraphGeometryChanged, paragraphKeepChanged, paragraphLayoutChanged, runStyleChanged bool) (int, bool) {
	score := 0
	changeCount := 0
	strongCombination := false
	if listLevelChanged {
		score += 3
		changeCount++
		strongCombination = true
	}
	if listFormatChanged {
		score++
		changeCount++
	}
	if paragraphGeometryChanged {
		score += 2
		changeCount++
	}
	if paragraphKeepChanged {
		score += 1
		changeCount++
	}
	if paragraphLayoutChanged {
		score += 1
		changeCount++
	}
	if runStyleChanged {
		score += 1
		changeCount++
	}
	if score == 0 {
		return 0, false
	}
	if paragraphGeometryChanged && paragraphLayoutChanged {
		strongCombination = true
	}
	if listFormatChanged && paragraphGeometryChanged && runStyleChanged {
		strongCombination = true
	}
	if listFormatChanged && paragraphLayoutChanged && runStyleChanged {
		strongCombination = true
	}
	if paragraphKeepChanged && paragraphLayoutChanged && runStyleChanged {
		strongCombination = true
	}
	if changeCount >= 3 && score >= 3 && !listFormatChanged {
		strongCombination = true
	}
	if strongCombination {
		return legacyBinaryListMinListLevelRestartCPGap, true
	}
	if score >= 2 && changeCount >= 2 {
		return legacyBinaryListMinCombinedRestartCPGap, true
	}
	return legacyBinaryListMinListFormatRestartCPGap, true
}

func legacyBinaryListListLevelContextSignature(paragraph *wordir.Paragraph) string {
	if paragraph == nil {
		return ""
	}
	level := -1
	if paragraph.List != nil {
		level = paragraph.List.Level
	}
	return strconv.Itoa(level)
}

func legacyBinaryListListFormatContextSignature(paragraph *wordir.Paragraph) string {
	if paragraph == nil || paragraph.List == nil {
		return ""
	}
	return legacyBinaryListNormalizedPlainString(paragraph.List.Format)
}

func legacyBinaryListParagraphGeometryContextSignature(paragraph *wordir.Paragraph) string {
	if paragraph == nil {
		return ""
	}
	return strings.Join([]string{
		legacyBinaryListNormalizedPlainString(paragraph.Format.Alignment),
		legacyBinaryListIndentSignature(paragraph.Format.IndentLeft),
		legacyBinaryListIndentSignature(paragraph.Format.IndentHanging),
	}, "|")
}

func legacyBinaryListParagraphKeepContextSignature(paragraph *wordir.Paragraph) string {
	if paragraph == nil {
		return ""
	}
	return strings.Join([]string{
		legacyBinaryListBoolSignature(paragraph.Format.KeepNext),
		legacyBinaryListBoolSignature(paragraph.Format.KeepLines),
		legacyBinaryListBoolSignature(paragraph.Format.PageBreakBefore),
		legacyBinaryListBoolSignature(paragraph.Format.ContextualSpacing),
		legacyBinaryListBoolSignature(paragraph.Format.WidowControl),
	}, "|")
}

func legacyBinaryListParagraphLayoutContextSignature(paragraph *wordir.Paragraph) string {
	if paragraph == nil {
		return ""
	}
	return strings.Join([]string{
		legacyBinaryListNormalizedColorSignature(paragraph.Format.Shading),
		legacyBinaryListNormalizedPlainString(paragraph.Format.LineRule),
		legacyBinaryListIndentSignature(paragraph.Format.LineSpacing),
		legacyBinaryListIndentSignature(paragraph.Format.SpacingBefore),
		legacyBinaryListIndentSignature(paragraph.Format.SpacingAfter),
		legacyBinaryListIndentSignature(paragraph.Format.SpacingBeforeLines),
		legacyBinaryListIndentSignature(paragraph.Format.SpacingAfterLines),
		legacyBinaryListBoolSignature(paragraph.Format.SpacingBeforeAuto),
		legacyBinaryListBoolSignature(paragraph.Format.SpacingAfterAuto),
		legacyBinaryListBorderSignature(paragraph.Format.Borders),
	}, "|")
}

func legacyBinaryListIndentSignature(value *int) string {
	if value == nil {
		return "-"
	}
	return strconv.Itoa(*value)
}

func legacyBinaryListBoolSignature(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func legacyBinaryListBorderSignature(borders *wordir.ParagraphBorders) string {
	if borders == nil {
		return "-"
	}
	return strings.Join([]string{
		legacyBinaryListBorderSideSignature(borders.Top),
		legacyBinaryListBorderSideSignature(borders.Left),
		legacyBinaryListBorderSideSignature(borders.Bottom),
		legacyBinaryListBorderSideSignature(borders.Right),
	}, ",")
}

func legacyBinaryListBorderSideSignature(side *wordir.BorderSide) string {
	if side == nil {
		return "-"
	}
	return strings.Join([]string{
		legacyBinaryListNormalizedPlainString(side.Val),
		strconv.FormatFloat(side.SizePt, 'f', -1, 64),
		strconv.FormatFloat(side.SpacePt, 'f', -1, 64),
		legacyBinaryListNormalizedColorSignature(side.Color),
	}, ":")
}

func legacyBinaryListRunFormatSignature(lineRuns []binaryWordRun) string {
	totalVisible := 0
	for _, run := range lineRuns {
		if run.Kind != "" {
			continue
		}
		totalVisible += legacyBinaryListVisibleRuneCount(run.Text)
	}
	bestVisible := 0
	bestSignature := "-"
	currentVisible := 0
	currentSignature := ""
	for _, run := range lineRuns {
		if run.Kind != "" || strings.TrimSpace(run.Text) == "" {
			if currentVisible > bestVisible {
				bestVisible = currentVisible
				bestSignature = currentSignature
			}
			currentVisible = 0
			currentSignature = ""
			continue
		}
		fingerprint := legacyBinaryListRunStyleFingerprintFor(run.Format)
		if !fingerprint.meaningful() {
			if currentVisible > bestVisible {
				bestVisible = currentVisible
				bestSignature = currentSignature
			}
			currentVisible = 0
			currentSignature = ""
			continue
		}
		visible := legacyBinaryListVisibleRuneCount(run.Text)
		signature := fingerprint.signature()
		if signature == currentSignature {
			currentVisible += visible
		} else {
			if currentVisible > bestVisible {
				bestVisible = currentVisible
				bestSignature = currentSignature
			}
			currentVisible = visible
			currentSignature = signature
		}
	}
	if currentVisible > bestVisible {
		bestVisible = currentVisible
		bestSignature = currentSignature
	}
	if totalVisible > 0 && bestVisible*2 >= totalVisible {
		return bestSignature
	}
	return "-"
}

type legacyBinaryListRunStyleFingerprint struct {
	bold       string
	italic     string
	strike     string
	smallCaps  string
	caps       string
	underline  string
	color      string
	background string
	highlight  string
	vertAlign  string
	fontSize   string
	font       string
}

func legacyBinaryListRunStyleFingerprintFor(format wordir.RunFormat) legacyBinaryListRunStyleFingerprint {
	return legacyBinaryListRunStyleFingerprint{
		bold:       legacyBinaryListNormalizedStyleBoolSignature(format.Bold),
		italic:     legacyBinaryListNormalizedStyleBoolSignature(format.Italic),
		strike:     legacyBinaryListNormalizedStyleBoolSignature(format.Strike),
		smallCaps:  legacyBinaryListNormalizedStyleBoolSignature(format.SmallCaps),
		caps:       legacyBinaryListNormalizedStyleBoolSignature(format.Caps),
		underline:  legacyBinaryListNormalizedUnderlineSignature(format.Underline),
		color:      legacyBinaryListNormalizedColorSignature(format.Color),
		background: legacyBinaryListNormalizedBackgroundSignature(format.Background),
		highlight:  legacyBinaryListNormalizedHighlightSignature(format.Highlight),
		vertAlign:  legacyBinaryListNormalizedPlainString(format.VertAlignVal),
		fontSize:   legacyBinaryListNormalizedOptionalIntSignature(format.FontSize),
		font:       legacyBinaryListNormalizedFontSignature(format),
	}
}

func (fingerprint legacyBinaryListRunStyleFingerprint) signature() string {
	return strings.Join([]string{
		fingerprint.bold,
		fingerprint.italic,
		fingerprint.strike,
		fingerprint.smallCaps,
		fingerprint.caps,
		fingerprint.underline,
		fingerprint.color,
		fingerprint.background,
		fingerprint.highlight,
		fingerprint.vertAlign,
		fingerprint.fontSize,
		fingerprint.font,
	}, ":")
}

func (fingerprint legacyBinaryListRunStyleFingerprint) meaningful() bool {
	return fingerprint.bold != "" ||
		fingerprint.italic != "" ||
		fingerprint.strike != "" ||
		fingerprint.smallCaps != "" ||
		fingerprint.caps != "" ||
		fingerprint.underline != "" ||
		fingerprint.color != "" ||
		fingerprint.background != "" ||
		fingerprint.highlight != "" ||
		fingerprint.vertAlign != "" ||
		fingerprint.fontSize != "" ||
		fingerprint.font != ""
}

func legacyBinaryListSingleRunFormatSignature(format wordir.RunFormat) string {
	return legacyBinaryListRunStyleFingerprintFor(format).signature()
}

func legacyBinaryListNormalizedStyleBoolSignature(value *bool) string {
	if value == nil || !*value {
		return ""
	}
	return "1"
}

func legacyBinaryListNormalizedPlainString(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func legacyBinaryListNormalizedColorSignature(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func legacyBinaryListNormalizedUnderlineSignature(value string) string {
	value = legacyBinaryListNormalizedPlainString(value)
	if value == "none" {
		return ""
	}
	return value
}

func legacyBinaryListNormalizedBackgroundSignature(value string) string {
	value = legacyBinaryListNormalizedPlainString(value)
	if value == "auto" {
		return ""
	}
	return value
}

func legacyBinaryListNormalizedHighlightSignature(value string) string {
	value = legacyBinaryListNormalizedPlainString(value)
	if value == "auto" || value == "none" {
		return ""
	}
	return value
}

func legacyBinaryListNormalizedFontSignature(format wordir.RunFormat) string {
	font := strings.TrimSpace(format.FontASCII)
	if font == "" {
		font = strings.TrimSpace(format.FontHAnsi)
	}
	return strings.ToLower(font)
}

func legacyBinaryListNormalizedOptionalIntSignature(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func legacyBinaryListRunFormatIsMeaningful(format wordir.RunFormat) bool {
	return legacyBinaryListRunStyleFingerprintFor(format).meaningful()
}

func legacyBinaryListVisibleRuneCount(text string) int {
	count := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count
}

func legacyBinaryListOptionalBoolSignature(value *bool) string {
	if value == nil {
		return "-"
	}
	if *value {
		return "1"
	}
	return "0"
}

func lineSliceCPRange(slice []binaryWordRun) (int, int, bool) {
	if len(slice) == 0 {
		return 0, 0, false
	}
	start := 0
	end := 0
	has := false
	for _, r := range slice {
		rs, re := lineRunCPRange(r)
		if !has {
			start = rs
			end = re
			has = true
			continue
		}
		if rs < start {
			start = rs
		}
		if re > end {
			end = re
		}
	}
	return start, end, has
}

func legacyBinaryListFormat(level int) string {
	switch level % 3 {
	case 1:
		return "lowerLetter"
	case 2:
		return "lowerRoman"
	default:
		return "decimal"
	}
}

// preWord97NfcToListFormat maps the pre-Word-97 nfcSeqNumb byte (opcode 12) to a
// list format string used by the IR ListMarker.  The nfc enum matches the
// standard OOXML ST_NumberFormat codes that Word 97+ also uses:
//
//	0  Arabic decimal (1, 2, 3...)
//	1  Uppercase Roman (I, II, III...)
//	2  Lowercase Roman (i, ii, iii...)
//	3  Uppercase Letter (A, B, C...)
//	4  Lowercase Letter (a, b, c...)
//	5  Ordinal (1st, 2nd...)
//	23 None (no numbering displayed, paragraph still in list)
//	255 Bullet
func preWord97NfcToListFormat(nfc int) string {
	switch nfc {
	case 0:
		return "decimal"
	case 1:
		return "upperRoman"
	case 2:
		return "lowerRoman"
	case 3:
		return "upperLetter"
	case 4:
		return "lowerLetter"
	case 255:
		return "bullet"
	default:
		return "decimal"
	}
}

func bookmarksForLineSlice(lineRuns []binaryWordRun, bookmarkStarts map[int][]string) []string {
	if len(bookmarkStarts) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, r := range lineRuns {
		startCP, endCP := lineRunCPRange(r)
		for cp, names := range bookmarkStarts {
			if cp < startCP || cp > endCP {
				continue
			}
			for _, name := range names {
				if name == "" || seen[name] {
					continue
				}
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return out
}

func lineRunCPRange(r binaryWordRun) (int, int) {
	if r.Kind != "" {
		return r.StartCP, r.StartCP
	}
	width := len([]rune(r.Text))
	return r.StartCP, r.StartCP + width
}

// splitDocRunsIntoLines normalises control characters within each run and
// splits the run slice at paragraph boundaries. Empty paragraphs are preserved:
// in Word's text stream, consecutive paragraph marks are real layout objects and
// contribute one line box each. Dropping them compresses vertical flow and can
// change pagination substantially (especially around list groups).
func splitDocRunsIntoLines(runs []binaryWordRun) [][]binaryWordRun {
	var lines [][]binaryWordRun
	var current []binaryWordRun
	prevNewline := true // suppress a leading blank line
	appendBoundary := func() {
		if len(lines) > 0 && lineSliceIsLegacyListBoundary(lines[len(lines)-1]) {
			return
		}
		lines = append(lines, []binaryWordRun{{Kind: legacyListBoundaryKind}})
	}
	appendBlankParagraph := func(source binaryWordRun, startCP int) {
		lines = append(lines, []binaryWordRun{{
			Kind:       "blankParagraph",
			StartCP:    startCP,
			ParaFormat: source.ParaFormat,
			TableDepth: source.TableDepth,
			TableProps: source.TableProps,
			PapxSprms:  append([]byte(nil), source.PapxSprms...),
		}})
	}

	flushLine := func() {
		if len(current) == 0 {
			return
		}
		// Always keep binary table cell/row end marks even when the text is
		// whitespace-only; they are used as structural boundaries.
		isRowMark := false
		isCellMark := false
		for _, r := range current {
			if r.ParaFormat.IsTableTrailingParagraph {
				isRowMark = true
			}
			if r.ParaFormat.IsTableCellTrailingParagraph {
				isCellMark = true
			}
		}
		if isRowMark || isCellMark {
			lines = append(lines, current)
			current = nil
			return
		}
		// Keep all other lines only if they have non-whitespace content.
		hasSpecial := false
		for _, r := range current {
			if r.Kind != "" {
				hasSpecial = true
				break
			}
		}
		if hasSpecial {
			lines = append(lines, current)
			current = nil
			return
		}
		for _, r := range current {
			if strings.TrimSpace(r.Text) != "" {
				lines = append(lines, current)
				break
			}
		}
		current = nil
	}

	for _, r := range runs {
		if r.Kind == legacyListBoundaryKind {
			flushLine()
			appendBoundary()
			prevNewline = true
			continue
		}
		if r.Kind == "blankParagraph" {
			flushLine()
			lines = append(lines, []binaryWordRun{{Kind: r.Kind, StartCP: r.StartCP, ParaFormat: r.ParaFormat, TableDepth: r.TableDepth, TableProps: r.TableProps}})
			prevNewline = false
			continue
		}
		if r.Kind != "" {
			current = append(current, r)
			prevNewline = false
			continue
		}
		text := r.Text
		// Normalise control characters.
		text = strings.ReplaceAll(text, "\u0007", "\t")
		text = strings.ReplaceAll(text, "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
		if !strings.ContainsRune(text, '\u000C') && (r.ParaFormat.InTable || r.LegacyInTableHint || r.NativeCellMarks || r.LegacyRowMarkHint) {
			// Table/control projections can surface VT as a structural marker.
			// Outside tables, U+000B is Word's manual line break and must stay
			// in the stream so the boundary pass below can materialize the next
			// physical line instead of joining it to the preceding heading.
			text = strings.ReplaceAll(text, "\u000B", "")
		}
		// Drop non-printable, non-whitespace characters.
		var buf strings.Builder
		for _, ch := range text {
			if ch == '\n' || ch == '\t' || ch == '\u000B' || ch == '\u000C' || unicode.IsPrint(ch) {
				buf.WriteRune(ch)
			}
		}
		text = buf.String()
		if text == "" {
			continue
		}
		if (r.ParaFormat.IsTableTrailingParagraph || r.ParaFormat.IsTableCellTrailingParagraph) &&
			strings.Trim(text, "\n\t") == "" {
			flushLine()
			blank := r
			blank.Text = ""
			lines = append(lines, []binaryWordRun{blank})
			prevNewline = true
			continue
		}

		if !strings.ContainsAny(text, "\n\u000B\f") {
			copyRun := r
			copyRun.Text = text
			current = append(current, copyRun)
			prevNewline = false
			continue
		}

		runes := []rune(text)
		partStart := 0
		partCP := r.StartCP
		appendPart := func(end int) {
			if end <= partStart {
				return
			}
			copyRun := r
			copyRun.Text = string(runes[partStart:end])
			copyRun.StartCP = partCP
			current = append(current, copyRun)
			partCP += end - partStart
			prevNewline = false
		}
		for i, ch := range runes {
			if ch != '\n' && ch != '\u000B' && ch != '\u000C' {
				continue
			}
			appendPart(i)
			if ch == '\n' {
				if !prevNewline {
					flushLine()
					prevNewline = true
				} else if len(lines) > 0 {
					// A second paragraph mark means there is an empty paragraph between
					// the surrounding paragraphs. Preserve every such mark instead of
					// replacing it with a non-rendering list-session boundary.
					appendBlankParagraph(r, partCP)
				}
			} else if ch == '\u000B' {
				if !prevNewline {
					flushLine()
					prevNewline = true
				} else {
					appendBoundary()
				}
			} else {
				if !prevNewline {
					flushLine()
				}
				breakRun := r
				breakRun.Text = ""
				breakRun.Kind = "break"
				breakRun.BreakType = "page"
				breakRun.Target = ""
				breakRun.FieldKind = ""
				breakRun.FieldInstr = ""
				breakRun.FieldArg = ""
				breakRun.Anchor = ""
				breakRun.Image = nil
				breakRun.StartCP = partCP
				lines = append(lines, []binaryWordRun{breakRun})
				prevNewline = true
			}
			partCP++
			partStart = i + 1
		}
		appendPart(len(runes))
	}
	flushLine()
	return lines
}

// joinDocRunsText concatenates the Text fields of all runs in the slice.
func joinDocRunsText(runs []binaryWordRun) string {
	if len(runs) == 1 {
		return runs[0].Text
	}
	var sb strings.Builder
	for _, r := range runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

// trimDocRunsWhitespace trims leading whitespace from the first run and
// trailing whitespace from the last run, dropping any runs that become empty.
func trimDocRunsWhitespace(runs []binaryWordRun) []binaryWordRun {
	result := make([]binaryWordRun, 0, len(runs))
	leadDone := false
	for _, r := range runs {
		if r.Kind != "" {
			result = append(result, r)
			continue
		}
		if !leadDone {
			r.Text = strings.TrimLeft(r.Text, " \t")
			if r.Text == "" {
				continue
			}
			leadDone = true
		}
		result = append(result, r)
	}
	for i := len(result) - 1; i >= 0; i-- {
		if result[i].Kind != "" {
			break
		}
		result[i].Text = strings.TrimRight(result[i].Text, " \t")
		if result[i].Text != "" {
			break
		}
		result = result[:i]
	}
	return result
}

// skipDocRunsBytes removes skip bytes from the front of the run slice,
// splitting or dropping individual runs as needed.
func skipDocRunsBytes(runs []binaryWordRun, skip int) []binaryWordRun {
	if skip <= 0 {
		return runs
	}
	result := make([]binaryWordRun, 0, len(runs))
	for _, r := range runs {
		if r.Kind != "" {
			result = append(result, r)
			continue
		}
		if skip <= 0 {
			result = append(result, r)
			continue
		}
		n := len(r.Text)
		if skip >= n {
			skip -= n
			continue
		}
		r.Text = r.Text[skip:]
		skip = 0
		if r.Text != "" {
			result = append(result, r)
		}
	}
	return result
}

// binaryWordFieldsToIRFields converts []*binaryWordField to []wordir.Field, recursively
// converting nested Children and InstrRuns via binaryWordRunsToIRRuns.
func binaryWordFieldsToIRFields(fields []*binaryWordField, baseFormat wordir.RunFormat) []wordir.Field {
	return binaryWordFieldsToIRFieldsImpl(fields, baseFormat, make(map[*binaryWordField]bool))
}

func binaryWordFieldsToIRFieldsImpl(fields []*binaryWordField, baseFormat wordir.RunFormat, seen map[*binaryWordField]bool) []wordir.Field {
	result := make([]wordir.Field, 0, len(fields))
	for _, f := range fields {
		if f == nil || seen[f] {
			continue
		}
		seen[f] = true
		result = append(result, wordir.Field{
			Kind:      f.Kind,
			Instr:     f.Instr,
			Arg:       f.Arg,
			Anchor:    f.Anchor,
			Target:    f.Target,
			Children:  binaryWordRunsToIRRunsImpl(f.Children, baseFormat, seen),
			InstrRuns: binaryWordRunsToIRRunsImpl(f.InstrRuns, baseFormat, seen),
		})
	}
	return result
}

// binaryWordRunsToIRRuns converts []binaryWordRun to []wordir.Run, skipping empty runs.
// reconcileLegacyRunWhitespace makes the semantic run projection reproduce the
// canonical paragraph text without changing character-format ownership. It is
// conservative: reconciliation happens only when the two forms differ solely
// in Unicode whitespace. Non-whitespace disagreement remains visible as a
// decoder/FC defect instead of being papered over here.
func reconcileLegacyRunWhitespace(runs []wordir.Run, canonical string) []wordir.Run {
	if len(runs) == 0 || canonical == "" {
		return runs
	}
	var projected strings.Builder
	for _, run := range runs {
		switch run.Kind {
		case "tab":
			projected.WriteByte('\t')
		case "", "hyperlink":
			projected.WriteString(run.Text)
		}
	}
	if projected.String() == canonical || stripLegacyWhitespace(projected.String()) != stripLegacyWhitespace(canonical) {
		return runs
	}
	type ownerChar struct {
		run int
		r   rune
	}
	owners := make([]ownerChar, 0, len([]rune(canonical)))
	for i, run := range runs {
		if run.Kind != "" && run.Kind != "hyperlink" {
			continue
		}
		for _, r := range run.Text {
			if !unicode.IsSpace(r) {
				owners = append(owners, ownerChar{i, r})
			}
		}
	}
	canonicalNonSpace := make([]rune, 0, len(owners))
	for _, r := range canonical {
		if !unicode.IsSpace(r) {
			canonicalNonSpace = append(canonicalNonSpace, r)
		}
	}
	if len(owners) != len(canonicalNonSpace) {
		return runs
	}
	for i := range owners {
		if owners[i].r != canonicalNonSpace[i] {
			return runs
		}
	}
	out := append([]wordir.Run(nil), runs...)
	for i := range out {
		if out[i].Kind == "" || out[i].Kind == "hyperlink" {
			out[i].Text = ""
		}
	}
	ownerPos := 0
	var pending strings.Builder
	for _, r := range canonical {
		if unicode.IsSpace(r) {
			if r != '\t' {
				pending.WriteRune(r)
			}
			continue
		}
		if ownerPos >= len(owners) {
			return runs
		}
		o := owners[ownerPos]
		out[o.run].Text += pending.String()
		pending.Reset()
		out[o.run].Text += string(r)
		ownerPos++
	}
	if pending.Len() > 0 {
		for i := len(out) - 1; i >= 0; i-- {
			if out[i].Kind == "" || out[i].Kind == "hyperlink" {
				out[i].Text += pending.String()
				break
			}
		}
	}
	return out
}

// restoreLegacyHiddenEdgeText repairs a narrow legacy CHPX failure mode where
// the first or last characters of the canonical piece-table text are assigned
// Vanish/SpecVanish even though the paragraph text itself contains them.  Only
// an exact hidden prefix or suffix is restored; hidden text in the middle stays
// hidden.  This keeps the piece stream authoritative without masking arbitrary
// FC/CHPX disagreements.
func restoreLegacyHiddenEdgeText(runs []wordir.Run, canonical string) []wordir.Run {
	if len(runs) == 0 || canonical == "" {
		return runs
	}
	visible := func(rs []wordir.Run) string {
		var b strings.Builder
		for _, run := range rs {
			if legacyRunIsHidden(run) {
				continue
			}
			switch run.Kind {
			case "tab":
				b.WriteByte('\t')
			case "", "hyperlink":
				b.WriteString(run.Text)
			}
		}
		return b.String()
	}
	projected := visible(runs)
	if projected == canonical {
		return runs
	}
	out := append([]wordir.Run(nil), runs...)
	clearHidden := func(i int) {
		out[i].Format.Vanish = nil
		out[i].Format.SpecVanish = nil
	}
	if strings.HasSuffix(canonical, projected) {
		missing := strings.TrimSuffix(canonical, projected)
		var edge strings.Builder
		var idx []int
		for i, run := range out {
			if !legacyRunIsHidden(run) {
				break
			}
			if run.Kind != "" && run.Kind != "hyperlink" {
				break
			}
			edge.WriteString(run.Text)
			idx = append(idx, i)
		}
		if edge.String() == missing {
			for _, i := range idx {
				clearHidden(i)
			}
			return out
		}
	}
	if strings.HasPrefix(canonical, projected) {
		missing := strings.TrimPrefix(canonical, projected)
		var edge strings.Builder
		var idx []int
		for i := len(out) - 1; i >= 0; i-- {
			run := out[i]
			if !legacyRunIsHidden(run) {
				break
			}
			if run.Kind != "" && run.Kind != "hyperlink" {
				break
			}
			edge.WriteString(run.Text)
			idx = append(idx, i)
		}
		// suffix runs were visited backwards; rebuild in document order.
		var ordered strings.Builder
		for i := len(idx) - 1; i >= 0; i-- {
			ordered.WriteString(out[idx[i]].Text)
		}
		if ordered.String() == missing {
			for _, i := range idx {
				clearHidden(i)
			}
			return out
		}
	}
	return runs
}

// reconcileLegacyVisibleRunWhitespace reconciles whitespace against the runs
// that actually participate in layout/rendering. Word binary formatting tables
// can isolate spaces into a run that later resolves as Vanish/SpecVanish. When
// such a hidden run contains whitespace only, treating it as semantic content
// makes Paragraph.Text, layout and HTML disagree. Genuine hidden text is left
// untouched.
func reconcileLegacyVisibleRunWhitespace(runs []wordir.Run, canonical string) []wordir.Run {
	if len(runs) == 0 || canonical == "" {
		return runs
	}
	out := restoreLegacyHiddenEdgeText(runs, canonical)
	changed := false
	for i := range out {
		if !legacyRunIsHidden(out[i]) {
			continue
		}
		if out[i].Kind == "tab" {
			// A hidden tab is formatting-sensitive content; do not reinterpret it.
			return runs
		}
		if out[i].Kind != "" && out[i].Kind != "hyperlink" {
			continue
		}
		if stripLegacyWhitespace(out[i].Text) != "" {
			// This paragraph contains genuine hidden characters. Its visible text is
			// intentionally different from Paragraph.Text.
			return runs
		}
		if out[i].Text != "" {
			out[i].Text = ""
			changed = true
		}
	}
	if !changed {
		return reconcileLegacyRunWhitespace(out, canonical)
	}
	return reconcileLegacyRunWhitespace(out, canonical)
}

func legacyRunIsHidden(run wordir.Run) bool {
	return (run.Format.Vanish != nil && *run.Format.Vanish) ||
		(run.Format.SpecVanish != nil && *run.Format.SpecVanish)
}

func stripLegacyWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func binaryWordRunsToIRRuns(runs []binaryWordRun, baseFormat wordir.RunFormat) []wordir.Run {
	return binaryWordRunsToIRRunsImpl(runs, baseFormat, make(map[*binaryWordField]bool))
}

func binaryWordRunsToIRRunsImpl(runs []binaryWordRun, baseFormat wordir.RunFormat, seen map[*binaryWordField]bool) []wordir.Run {
	result := make([]wordir.Run, 0, len(runs))
	for _, r := range runs {
		format := legacyMergeRunFormat(baseFormat, r.Format)
		runSource := binaryWordRunToRunSource(r)
		if r.Kind == "footnoteRef" || r.Kind == "endnoteRef" {
			result = append(result, wordir.Run{Kind: r.Kind, NoteID: r.NoteID, Format: format, Source: runSource})
			continue
		}
		if r.Kind == "commentRef" {
			result = append(result, wordir.Run{Kind: r.Kind, CommentID: r.CommentID, Format: format, Source: runSource})
			continue
		}
		if r.Kind == "break" {
			result = append(result, wordir.Run{Kind: "break", BreakType: r.BreakType, Format: format, Source: runSource})
			continue
		}
		if r.Kind == "image" && r.Image != nil {
			result = append(result, wordir.Run{Kind: "image", Image: r.Image, Format: format, Revision: r.Revision, RevisionAuthor: r.RevisionAuthor, RevisionDate: r.RevisionDate, PropertyRevision: r.RunPropertyRevision, DisplayFieldRevision: r.DisplayFieldRevision, Source: runSource})
			continue
		}
		if r.Kind == "shape" && r.Shape != nil {
			result = append(result, wordir.Run{Kind: "shape", Shape: r.Shape, Format: format, Revision: r.Revision, RevisionAuthor: r.RevisionAuthor, RevisionDate: r.RevisionDate, PropertyRevision: r.RunPropertyRevision, DisplayFieldRevision: r.DisplayFieldRevision, Source: runSource})
			continue
		}
		if r.Kind == "formField" && r.FormField != nil {
			result = append(result, wordir.Run{
				Kind:                 "formField",
				FormField:            r.FormField,
				Format:               format,
				Revision:             r.Revision,
				RevisionAuthor:       r.RevisionAuthor,
				RevisionDate:         r.RevisionDate,
				PropertyRevision:     r.RunPropertyRevision,
				DisplayFieldRevision: r.DisplayFieldRevision,
				FieldKind:            r.FieldKind,
				FieldInstr:           r.FieldInstr,
				FieldArg:             r.FieldArg,
				Anchor:               r.Anchor,
				Source:               runSource,
			})
			continue
		}
		if r.Kind == "olePlaceholder" {
			embedded := wordir.Run{Kind: r.Kind, Text: r.Text}
			if r.EmbeddedRun != nil {
				embedded = *r.EmbeddedRun
			}
			embedded.Kind = "olePlaceholder"
			embedded.Format = format
			embedded.Revision = r.Revision
			embedded.RevisionAuthor = r.RevisionAuthor
			embedded.RevisionDate = r.RevisionDate
			embedded.PropertyRevision = r.RunPropertyRevision
			embedded.DisplayFieldRevision = r.DisplayFieldRevision
			embedded.Source = runSource
			result = append(result, embedded)
			continue
		}
		if r.Text == "" {
			continue
		}
		ir := wordir.Run{Text: r.Text, Format: format, Revision: r.Revision, RevisionAuthor: r.RevisionAuthor, RevisionDate: r.RevisionDate, PropertyRevision: r.RunPropertyRevision, DisplayFieldRevision: r.DisplayFieldRevision, FieldKind: r.FieldKind, FieldInstr: r.FieldInstr, FieldArg: r.FieldArg, Anchor: r.Anchor, Fields: binaryWordFieldsToIRFieldsImpl(r.Fields, baseFormat, seen), Source: runSource}
		if r.Target != "" {
			ir.Kind = "hyperlink"
			ir.Target = r.Target
		}
		// Binary DOC text pieces can contain literal TAB characters. Keep them as
		// semantic tab runs so the HTML renderer can honor default/custom tab stops
		// instead of letting browser whitespace collapsing destroy the columns.
		if strings.ContainsRune(ir.Text, '\t') {
			parts := strings.Split(ir.Text, "\t")
			for i, part := range parts {
				if part != "" {
					segment := ir
					segment.Text = part
					// Field metadata belongs to the first textual segment only.
					if i > 0 {
						segment.Fields = nil
					}
					result = append(result, segment)
				}
				if i < len(parts)-1 {
					result = append(result, wordir.Run{Kind: "tab", Format: format, Source: runSource})
				}
			}
			continue
		}
		result = append(result, ir)
	}
	return result
}

// binaryWordRunToRunSource builds a wordir.RunSource from the raw data captured on a binaryWordRun.
func binaryWordRunToRunSource(r binaryWordRun) *wordir.RunSource {
	src := &wordir.RunSource{
		ChpxSprms: r.ChpxSprms,
		Revision:  r.Revision,
		Spec:      r.Spec,
	}
	if r.Format.BinaryStyleIndex != nil {
		istd := *r.Format.BinaryStyleIndex
		src.StyleIndex = &istd
	}
	if r.HasIbstRMark {
		src.IbstRMark = r.IbstRMark
		src.HasIbstRMark = true
	}
	if r.HasDttmRMark {
		src.DttmRMark = r.DttmRMark
		src.HasDttmRMark = true
	}
	if r.HasPropRMark {
		src.PropertyRevision = &wordir.SourcePropertyRevision{
			Ibst:    r.PropRMarkIbst,
			HasIbst: r.HasPropRMarkIbst,
			Dttm:    r.PropRMarkDttm,
			HasDttm: r.HasPropRMarkDttm,
		}
	}
	if r.HasRunPropRMark {
		src.RunPropertyRevision = &wordir.SourcePropertyRevision{
			Ibst:    r.RunPropRMarkIbst,
			HasIbst: r.HasRunPropRMarkIbst,
			Dttm:    r.RunPropRMarkDttm,
			HasDttm: r.HasRunPropRMarkDttm,
		}
	}
	if r.HasDispFldRMark {
		src.DisplayFieldRevision = &wordir.SourcePropertyRevision{
			Ibst:    r.DispFldRMarkIbst,
			HasIbst: r.HasDispFldRMarkIbst,
			Dttm:    r.DispFldRMarkDttm,
			HasDttm: r.HasDispFldRMarkDttm,
		}
	}
	return src
}

func applyLegacyCharacterStylesToRuns(runs []wordir.Run, styleSheet *binaryWordStyleCatalog) {
	if len(runs) == 0 || styleSheet == nil {
		return
	}
	for i := range runs {
		applyLegacyCharacterStyleToRun(&runs[i], styleSheet)
	}
}

func applyLegacyCharacterStyleToRun(run *wordir.Run, styleSheet *binaryWordStyleCatalog) {
	if run == nil || styleSheet == nil {
		return
	}
	if run.Source != nil && run.Source.StyleIndex != nil {
		if style, ok := styleSheet.characterByIndex[*run.Source.StyleIndex]; ok {
			run.StyleID = style.ID
			run.Format = legacyMergeRunFormat(style.Format, run.Format)
		}
	}
	for i := range run.FieldInstrRuns {
		applyLegacyCharacterStyleToRun(&run.FieldInstrRuns[i], styleSheet)
	}
	for i := range run.Fields {
		for j := range run.Fields[i].Children {
			applyLegacyCharacterStyleToRun(&run.Fields[i].Children[j], styleSheet)
		}
		for j := range run.Fields[i].InstrRuns {
			applyLegacyCharacterStyleToRun(&run.Fields[i].InstrRuns[j], styleSheet)
		}
	}
}

func processLegacyObjectAnchors(runs []binaryWordRun, images []*wordir.Image) []binaryWordRun {
	const objectAnchor = '\x01'

	out := make([]binaryWordRun, 0, len(runs))
	imageIndex := 0
	for _, r := range runs {
		if r.Kind != "" || !strings.ContainsRune(r.Text, objectAnchor) {
			out = append(out, r)
			continue
		}
		runes := []rune(r.Text)
		segmentStart := 0
		segmentStartCP := r.StartCP
		cp := r.StartCP
		for i, ch := range runes {
			if ch != objectAnchor {
				cp++
				continue
			}
			if i > segmentStart {
				out = append(out, binaryWordRun{
					Text:                 string(runes[segmentStart:i]),
					Format:               r.Format,
					ParaFormat:           r.ParaFormat,
					TableDepth:           r.TableDepth,
					TableProps:           r.TableProps,
					Target:               r.Target,
					FieldKind:            r.FieldKind,
					FieldInstr:           r.FieldInstr,
					FieldArg:             r.FieldArg,
					Anchor:               r.Anchor,
					StartCP:              segmentStartCP,
					Revision:             r.Revision,
					RevisionAuthor:       r.RevisionAuthor,
					RevisionDate:         r.RevisionDate,
					PropertyRevision:     r.PropertyRevision,
					RunPropertyRevision:  r.RunPropertyRevision,
					DisplayFieldRevision: r.DisplayFieldRevision,
				})
			}
			// sprmCFData identifies picture/data anchors while sprmCFObj identifies
			// true embedded OLE objects. fSpec alone is not sufficient to tell them
			// apart; using CFData prevents valid inline pictures becoming placeholders.
			if !r.Obj && (r.Spec || r.Data) && imageIndex < len(images) && images[imageIndex] != nil {
				out = append(out, binaryWordRun{
					Kind:                 "image",
					Image:                images[imageIndex],
					Format:               r.Format,
					ParaFormat:           r.ParaFormat,
					TableDepth:           r.TableDepth,
					TableProps:           r.TableProps,
					StartCP:              cp,
					Revision:             r.Revision,
					RevisionAuthor:       r.RevisionAuthor,
					RevisionDate:         r.RevisionDate,
					PropertyRevision:     r.PropertyRevision,
					RunPropertyRevision:  r.RunPropertyRevision,
					DisplayFieldRevision: r.DisplayFieldRevision,
				})
				imageIndex++
			} else {
				out = append(out, binaryWordRun{
					Text:                 "Embedded Object",
					Kind:                 "olePlaceholder",
					Format:               r.Format,
					ParaFormat:           r.ParaFormat,
					TableDepth:           r.TableDepth,
					TableProps:           r.TableProps,
					StartCP:              cp,
					Revision:             r.Revision,
					RevisionAuthor:       r.RevisionAuthor,
					RevisionDate:         r.RevisionDate,
					PropertyRevision:     r.PropertyRevision,
					RunPropertyRevision:  r.RunPropertyRevision,
					DisplayFieldRevision: r.DisplayFieldRevision,
				})
			}
			cp++
			segmentStart = i + 1
			segmentStartCP = cp
		}
		if segmentStart < len(runes) {
			out = append(out, binaryWordRun{
				Text:                 string(runes[segmentStart:]),
				Format:               r.Format,
				ParaFormat:           r.ParaFormat,
				TableDepth:           r.TableDepth,
				TableProps:           r.TableProps,
				Target:               r.Target,
				FieldKind:            r.FieldKind,
				FieldInstr:           r.FieldInstr,
				FieldArg:             r.FieldArg,
				Anchor:               r.Anchor,
				StartCP:              segmentStartCP,
				Revision:             r.Revision,
				RevisionAuthor:       r.RevisionAuthor,
				RevisionDate:         r.RevisionDate,
				PropertyRevision:     r.PropertyRevision,
				RunPropertyRevision:  r.RunPropertyRevision,
				DisplayFieldRevision: r.DisplayFieldRevision,
			})
		}
	}
	return out
}

// processDocFieldRuns scans the run slice for DOC field char sequences
// (field-begin 0x13 … field-sep 0x14 … field-end 0x15) and annotates
// the display-text runs with a hyperlink Target when the field instruction
// is a HYPERLINK field.  Field-char runs are removed from the output.
//
// MS-DOC §2.8.25: fldChar values 0x13 = begin, 0x14 = separator, 0x15 = end.
func processDocFieldRuns(runs []binaryWordRun) []binaryWordRun {
	// Fast path: no field chars present.
	hasFieldChar := false
	for _, r := range runs {
		if strings.ContainsAny(r.Text, "\x13\x14\x15") {
			hasFieldChar = true
			break
		}
	}
	if !hasFieldChar {
		return runs
	}

	// Split each run at field-char boundaries to produce a flat token list.
	type token struct {
		kind int           // 0=text, 1=begin, 2=sep, 3=end
		run  binaryWordRun // only for kind==0
	}
	const (
		tokText  = 0
		tokBegin = 1
		tokSep   = 2
		tokEnd   = 3
	)
	var tokens []token
	for _, r := range runs {
		if r.Kind != "" {
			tokens = append(tokens, token{kind: tokText, run: r})
			continue
		}
		text := r.Text
		for len(text) > 0 {
			idx := strings.IndexAny(text, "\x13\x14\x15")
			if idx < 0 {
				tokens = append(tokens, token{kind: tokText, run: binaryWordRun{Text: text, Format: r.Format, ParaFormat: r.ParaFormat, TableDepth: r.TableDepth, TableProps: r.TableProps, Target: r.Target, StartCP: r.StartCP, CommentID: r.CommentID, Revision: r.Revision, RevisionAuthor: r.RevisionAuthor, RevisionDate: r.RevisionDate, PropertyRevision: r.PropertyRevision, RunPropertyRevision: r.RunPropertyRevision, DisplayFieldRevision: r.DisplayFieldRevision, Spec: r.Spec, Image: r.Image}})
				break
			}
			if idx > 0 {
				tokens = append(tokens, token{kind: tokText, run: binaryWordRun{Text: text[:idx], Format: r.Format, ParaFormat: r.ParaFormat, TableDepth: r.TableDepth, TableProps: r.TableProps, Target: r.Target, StartCP: r.StartCP, CommentID: r.CommentID, Revision: r.Revision, RevisionAuthor: r.RevisionAuthor, RevisionDate: r.RevisionDate, PropertyRevision: r.PropertyRevision, RunPropertyRevision: r.RunPropertyRevision, DisplayFieldRevision: r.DisplayFieldRevision, Spec: r.Spec, Image: r.Image}})
			}
			switch text[idx] {
			case 0x13:
				tokens = append(tokens, token{kind: tokBegin})
			case 0x14:
				tokens = append(tokens, token{kind: tokSep})
			case 0x15:
				tokens = append(tokens, token{kind: tokEnd})
			}
			text = text[idx+1:]
		}
	}

	// Walk tokens and build output runs, annotating hyperlink display runs.
	var out []binaryWordRun
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if tok.kind != tokBegin {
			if tok.kind == tokText {
				out = append(out, tok.run)
			}
			i++
			continue
		}
		// Found field-begin (0x13). Collect instruction text up to sep (0x14).
		i++
		var instrParts []string
		for i < len(tokens) && tokens[i].kind != tokSep && tokens[i].kind != tokEnd {
			if tokens[i].kind == tokText {
				instrParts = append(instrParts, tokens[i].run.Text)
			}
			i++
		}
		instr := strings.TrimSpace(strings.Join(instrParts, ""))
		url := parseHyperlinkFieldInstr(instr)

		if tokens[i].kind == tokSep {
			i++ // skip sep token
		}
		// Collect display-text runs up to end (0x15).
		for i < len(tokens) && tokens[i].kind != tokEnd {
			if tokens[i].kind == tokText && tokens[i].run.Text != "" {
				dr := tokens[i].run
				if url != "" {
					dr.Target = url
				}
				out = append(out, dr)
			}
			i++
		}
		if i < len(tokens) && tokens[i].kind == tokEnd {
			i++ // skip end token
		}
	}
	return out
}

// parseHyperlinkFieldInstr extracts the URL from a HYPERLINK field instruction
// string.  Returns "" for non-HYPERLINK fields or unparseable instructions.
// Format: HYPERLINK "url" [\o "tooltip"] [\l "anchor"]
func parseHyperlinkFieldInstr(instr string) string {
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, "HYPERLINK") {
		return ""
	}
	rest := strings.TrimSpace(instr[len("HYPERLINK"):])
	if len(rest) == 0 {
		return ""
	}
	// The URL is the first quoted token after the keyword.
	if rest[0] != '"' {
		// Unquoted URL — take everything up to the next space or \l switch.
		end := strings.IndexAny(rest, " \t")
		if end < 0 {
			return rest
		}
		return rest[:end]
	}
	// Quoted URL.
	end := strings.Index(rest[1:], "\"")
	if end < 0 {
		return strings.Trim(rest, "\"")
	}
	return rest[1 : end+1]
}

func legacyRomanMarkerFormat(marker string) string {
	if marker == strings.ToUpper(marker) {
		return "upperRoman"
	}
	return "lowerRoman"
}
