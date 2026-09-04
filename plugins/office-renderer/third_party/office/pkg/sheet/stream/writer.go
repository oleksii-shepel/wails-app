// Package stream provides bounded-memory XLSX writing for large worksheets.
//
// Unlike pkg/sheet/edit, stream does not build a complete workbook IR in
// memory. Rows are retained only inside a configurable sliding window and are
// flushed to temporary worksheet backing files as the window advances.
package stream

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	sheetir "office/pkg/sheet/ir"
)

const (
	spreadsheetNS          = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	relationshipsNS        = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	packageRelationshipsNS = "http://schemas.openxmlformats.org/package/2006/relationships"
)

var (
	// ErrClosed is returned after the streaming workbook has been closed.
	ErrClosed = errors.New("sheet stream: workbook is closed")
	// ErrRowFlushed is returned when callers attempt to mutate a row that has
	// already left the configured row window.
	ErrRowFlushed = errors.New("sheet stream: row has already been flushed")
)

// Options controls bounded-memory XLSX output.
type Options struct {
	// TempDir controls where worksheet backing files are written. Empty uses
	// the operating system default temporary directory.
	TempDir string
	// RowWindowSize is the maximum number of mutable rows retained per sheet.
	// Values <= 0 use 100. A value of 1 provides strict forward-only writing.
	RowWindowSize int
	// Date1904 selects the workbook's 1904 date system.
	Date1904 bool
}

// Workbook is a bounded-memory XLSX writer.
type Workbook struct {
	out         io.Writer
	opts        Options
	tempDir     string
	ownsTempDir bool
	sheets      []*Sheet
	styles      *styleRegistry
	closed      bool
}

// Sheet is a streaming worksheet.
type Sheet struct {
	wb    *Workbook
	index int
	name  string

	rowFile          *os.File
	hyperlinkFile    *os.File
	hyperlinkRelFile *os.File
	commentFile      *os.File
	commentVMLFile   *os.File

	pending        []*Row
	lastCreatedRow int
	lastFlushedRow int
	hyperlinkCount int
	commentCount   int
	authors        []string
	authorIDs      map[string]int

	minRow, maxRow         int
	minCol, maxCol         int
	columnWidths           map[int]float64
	freezeRows, freezeCols int
	autoFilter             string
	merges                 []string
	closed                 bool
}

// Row is mutable until it leaves its sheet's sliding row window.
type Row struct {
	sheet   *Sheet
	Index   int
	Height  float64
	Hidden  bool
	cells   map[int]Cell
	flushed bool
}

// Cell describes a streamed worksheet cell.
type Cell struct {
	Value         any
	Formula       string
	Cached        any
	StyleID       int
	Hyperlink     string
	Comment       string
	CommentAuthor string
}

// New creates a bounded-memory XLSX writer.
func New(out io.Writer, options ...Options) (*Workbook, error) {
	if out == nil {
		return nil, fmt.Errorf("sheet stream: nil output writer")
	}
	opts := Options{RowWindowSize: 100}
	if len(options) > 0 {
		opts = options[0]
		if opts.RowWindowSize <= 0 {
			opts.RowWindowSize = 100
		}
	}
	wb := &Workbook{out: out, opts: opts, styles: newStyleRegistry()}
	if opts.TempDir == "" {
		dir, err := os.MkdirTemp("", "office-sheet-stream-*")
		if err != nil {
			return nil, fmt.Errorf("sheet stream: create temp dir: %w", err)
		}
		wb.tempDir = dir
		wb.ownsTempDir = true
	} else {
		if err := os.MkdirAll(opts.TempDir, 0o755); err != nil {
			return nil, fmt.Errorf("sheet stream: create temp dir: %w", err)
		}
		wb.tempDir = opts.TempDir
	}
	return wb, nil
}

// AddSheet appends a worksheet. Sheet names must be unique case-insensitively.
func (w *Workbook) AddSheet(name string) (*Sheet, error) {
	if err := w.ensureOpen(); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("Sheet%d", len(w.sheets)+1)
	}
	if err := validateSheetName(name); err != nil {
		return nil, err
	}
	for _, s := range w.sheets {
		if strings.EqualFold(s.name, name) {
			return nil, fmt.Errorf("sheet stream: duplicate sheet name %q", name)
		}
	}
	idx := len(w.sheets) + 1
	mk := func(suffix string) (*os.File, error) {
		return os.CreateTemp(w.tempDir, fmt.Sprintf("sheet-%d-*%s", idx, suffix))
	}
	rowFile, err := mk(".rows")
	if err != nil {
		return nil, err
	}
	hf, err := mk(".links")
	if err != nil {
		rowFile.Close()
		return nil, err
	}
	hrf, err := mk(".linkrels")
	if err != nil {
		rowFile.Close()
		hf.Close()
		return nil, err
	}
	cf, err := mk(".comments")
	if err != nil {
		rowFile.Close()
		hf.Close()
		hrf.Close()
		return nil, err
	}
	vf, err := mk(".vml")
	if err != nil {
		rowFile.Close()
		hf.Close()
		hrf.Close()
		cf.Close()
		return nil, err
	}
	s := &Sheet{
		wb: w, index: idx, name: name,
		rowFile: rowFile, hyperlinkFile: hf, hyperlinkRelFile: hrf, commentFile: cf, commentVMLFile: vf,
		authorIDs: map[string]int{}, columnWidths: map[int]float64{},
	}
	w.sheets = append(w.sheets, s)
	return s, nil
}

// AddStyle adds or reuses a workbook cell style and returns its zero-based style ID.
func (w *Workbook) AddStyle(style sheetir.CellStyle) int {
	if w == nil || w.styles == nil {
		return 0
	}
	return w.styles.add(style)
}

// Name returns the worksheet name.
func (s *Sheet) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// CreateRow creates a mutable row. Rows must be created in strictly increasing order.
func (s *Sheet) CreateRow(index int) (*Row, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	if index < 1 || index > 1048576 {
		return nil, fmt.Errorf("sheet stream: row index %d out of range", index)
	}
	if index <= s.lastCreatedRow {
		return nil, fmt.Errorf("sheet stream: rows must be created in increasing order: %d after %d", index, s.lastCreatedRow)
	}
	r := &Row{sheet: s, Index: index, cells: map[int]Cell{}}
	s.pending = append(s.pending, r)
	s.lastCreatedRow = index
	if len(s.pending) > s.wb.opts.RowWindowSize {
		if err := s.flushOldest(len(s.pending) - s.wb.opts.RowWindowSize); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// AppendRow appends a row after the last created row, starting at row 1.
func (s *Sheet) AppendRow(values ...any) (*Row, error) {
	idx := s.lastCreatedRow + 1
	r, err := s.CreateRow(idx)
	if err != nil {
		return nil, err
	}
	for i, v := range values {
		if err := r.SetCell(i+1, Cell{Value: v}); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// SetCell sets a 1-based column on a mutable row.
func (r *Row) SetCell(col int, cell Cell) error {
	if r == nil || r.sheet == nil {
		return fmt.Errorf("sheet stream: nil row")
	}
	if r.flushed {
		return ErrRowFlushed
	}
	if col < 1 || col > 16384 {
		return fmt.Errorf("sheet stream: column index %d out of range", col)
	}
	if cell.StyleID < 0 || cell.StyleID >= r.sheet.wb.styles.count() {
		return fmt.Errorf("sheet stream: invalid style id %d", cell.StyleID)
	}
	r.cells[col] = cell
	return nil
}

// SetHeight sets an explicit row height in points.
func (r *Row) SetHeight(points float64) error {
	if r.flushed {
		return ErrRowFlushed
	}
	if points < 0 {
		return fmt.Errorf("sheet stream: negative row height")
	}
	r.Height = points
	return nil
}

// SetHidden controls row visibility.
func (r *Row) SetHidden(hidden bool) error {
	if r.flushed {
		return ErrRowFlushed
	}
	r.Hidden = hidden
	return nil
}

// SetColumnWidth stores an Excel column width. It must be called before Close;
// unlike row cells, column metadata remains small and in memory.
func (s *Sheet) SetColumnWidth(col int, width float64) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if col < 1 || col > 16384 {
		return fmt.Errorf("sheet stream: column index %d out of range", col)
	}
	if width <= 0 {
		return fmt.Errorf("sheet stream: column width must be > 0")
	}
	s.columnWidths[col] = width
	return nil
}

// FreezePanes freezes the leading rows and columns.
func (s *Sheet) FreezePanes(rows, cols int) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if rows < 0 || cols < 0 {
		return fmt.Errorf("sheet stream: freeze counts must be >= 0")
	}
	s.freezeRows, s.freezeCols = rows, cols
	return nil
}

// SetAutoFilter sets an A1 filter range written after sheetData.
func (s *Sheet) SetAutoFilter(ref string) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	s.autoFilter = strings.TrimSpace(ref)
	return nil
}

// MergeCells records an A1 merge range.
func (s *Sheet) MergeCells(ref string) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("sheet stream: empty merge range")
	}
	s.merges = append(s.merges, ref)
	return nil
}

// FlushRows flushes the oldest mutable rows while retaining keep rows in memory.
func (s *Sheet) FlushRows(keep int) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if keep < 0 {
		return fmt.Errorf("sheet stream: keep must be >= 0")
	}
	if keep >= len(s.pending) {
		return nil
	}
	return s.flushOldest(len(s.pending) - keep)
}

func (s *Sheet) flushOldest(n int) error {
	if n <= 0 {
		return nil
	}
	if n > len(s.pending) {
		n = len(s.pending)
	}
	for i := 0; i < n; i++ {
		if err := s.writeRow(s.pending[i]); err != nil {
			return err
		}
		s.pending[i].flushed = true
		s.lastFlushedRow = s.pending[i].Index
	}
	copy(s.pending, s.pending[n:])
	s.pending = s.pending[:len(s.pending)-n]
	return nil
}

func (s *Sheet) writeRow(r *Row) error {
	var b strings.Builder
	b.WriteString(`<row r="`)
	b.WriteString(strconv.Itoa(r.Index))
	b.WriteByte('"')
	if r.Height > 0 {
		b.WriteString(` ht="`)
		b.WriteString(formatFloat(r.Height))
		b.WriteString(`" customHeight="1"`)
	}
	if r.Hidden {
		b.WriteString(` hidden="1"`)
	}
	b.WriteByte('>')
	cols := make([]int, 0, len(r.cells))
	for c := range r.cells {
		cols = append(cols, c)
	}
	sort.Ints(cols)
	for _, col := range cols {
		cell := r.cells[col]
		if err := s.writeCellXML(&b, r.Index, col, cell); err != nil {
			return err
		}
		if cell.Hyperlink != "" {
			if err := s.writeHyperlink(r.Index, col, cell.Hyperlink); err != nil {
				return err
			}
		}
		if cell.Comment != "" {
			if err := s.writeComment(r.Index, col, cell.Comment, cell.CommentAuthor); err != nil {
				return err
			}
		}
		s.observe(r.Index, col)
	}
	b.WriteString(`</row>`)
	_, err := io.WriteString(s.rowFile, b.String())
	return err
}

func (s *Sheet) writeCellXML(b *strings.Builder, row, col int, c Cell) error {
	ref := cellRef(row, col)
	b.WriteString(`<c r="`)
	b.WriteString(ref)
	b.WriteByte('"')
	if c.StyleID > 0 {
		b.WriteString(` s="`)
		b.WriteString(strconv.Itoa(c.StyleID))
		b.WriteByte('"')
	}
	value := c.Value
	if c.Formula != "" {
		value = c.Cached
	}
	kind, text, err := encodeValue(value, s.wb.opts.Date1904)
	if err != nil {
		return fmt.Errorf("sheet stream: %s: %w", ref, err)
	}
	if kind != "" {
		b.WriteString(` t="`)
		b.WriteString(kind)
		b.WriteByte('"')
	}
	b.WriteByte('>')
	if c.Formula != "" {
		b.WriteString(`<f>`)
		xmlEscapeText(b, c.Formula)
		b.WriteString(`</f>`)
	}
	switch kind {
	case "inlineStr":
		b.WriteString(`<is><t`)
		if needsPreserve(text) {
			b.WriteString(` xml:space="preserve"`)
		}
		b.WriteByte('>')
		xmlEscapeText(b, text)
		b.WriteString(`</t></is>`)
	default:
		if c.Formula != "" || text != "" {
			b.WriteString(`<v>`)
			xmlEscapeText(b, text)
			b.WriteString(`</v>`)
		}
	}
	b.WriteString(`</c>`)
	return nil
}

func (s *Sheet) writeHyperlink(row, col int, target string) error {
	s.hyperlinkCount++
	rid := fmt.Sprintf("rIdHyperlink%d", s.hyperlinkCount)
	ref := cellRef(row, col)
	var b strings.Builder
	b.WriteString(`<hyperlink ref="`)
	b.WriteString(ref)
	b.WriteString(`" r:id="`)
	b.WriteString(rid)
	b.WriteString(`"/>`)
	if _, err := io.WriteString(s.hyperlinkFile, b.String()); err != nil {
		return err
	}
	var r strings.Builder
	r.WriteString(`<Relationship Id="`)
	r.WriteString(rid)
	r.WriteString(`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="`)
	xmlEscapeAttr(&r, target)
	r.WriteString(`" TargetMode="External"/>`)
	_, err := io.WriteString(s.hyperlinkRelFile, r.String())
	return err
}

func (s *Sheet) writeComment(row, col int, text, author string) error {
	if author == "" {
		author = "Author"
	}
	id, ok := s.authorIDs[author]
	if !ok {
		id = len(s.authors)
		s.authorIDs[author] = id
		s.authors = append(s.authors, author)
	}
	s.commentCount++
	ref := cellRef(row, col)
	var b strings.Builder
	b.WriteString(`<comment ref="`)
	b.WriteString(ref)
	b.WriteString(`" authorId="`)
	b.WriteString(strconv.Itoa(id))
	b.WriteString(`"><text><t`)
	if needsPreserve(text) {
		b.WriteString(` xml:space="preserve"`)
	}
	b.WriteByte('>')
	xmlEscapeText(&b, text)
	b.WriteString(`</t></text></comment>`)
	if _, err := io.WriteString(s.commentFile, b.String()); err != nil {
		return err
	}
	// Minimal Excel-compatible legacy note shape. Row/Column are zero based.
	shapeID := 1024 + s.commentCount
	vml := fmt.Sprintf(`<v:shape id="_x0000_s%d" type="#_x0000_t202" style="position:absolute;visibility:hidden;width:108pt;height:59.25pt" fillcolor="#ffffe1" o:insetmode="auto"><v:fill color2="#ffffe1"/><v:shadow on="t" color="black" obscured="t"/><v:path o:connecttype="none"/><v:textbox style="mso-direction-alt:auto"><div style="text-align:left"/></v:textbox><x:ClientData ObjectType="Note"><x:MoveWithCells/><x:SizeWithCells/><x:Anchor>%d, 15, %d, 10, %d, 31, %d, 9</x:Anchor><x:AutoFill>False</x:AutoFill><x:Row>%d</x:Row><x:Column>%d</x:Column></x:ClientData></v:shape>`, shapeID, col-1, row-1, col+1, row+3, row-1, col-1)
	_, err := io.WriteString(s.commentVMLFile, vml)
	return err
}

func (s *Sheet) observe(row, col int) {
	if s.minRow == 0 || row < s.minRow {
		s.minRow = row
	}
	if row > s.maxRow {
		s.maxRow = row
	}
	if s.minCol == 0 || col < s.minCol {
		s.minCol = col
	}
	if col > s.maxCol {
		s.maxCol = col
	}
}

// Close flushes all rows and writes the final XLSX archive. It is idempotent.
func (w *Workbook) Close() error {
	if w == nil {
		return nil
	}
	if w.closed {
		return nil
	}
	w.closed = true
	defer w.cleanup()
	if len(w.sheets) == 0 {
		return fmt.Errorf("sheet stream: workbook has no sheets")
	}
	for _, s := range w.sheets {
		if err := s.finish(); err != nil {
			return err
		}
	}
	return w.writeArchive()
}

func (s *Sheet) finish() error {
	if s.closed {
		return nil
	}
	if err := s.flushOldest(len(s.pending)); err != nil {
		return err
	}
	s.closed = true
	for _, f := range []*os.File{s.rowFile, s.hyperlinkFile, s.hyperlinkRelFile, s.commentFile, s.commentVMLFile} {
		if err := f.Sync(); err != nil {
			return err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}
	return nil
}

func (w *Workbook) writeArchive() error {
	zw := zip.NewWriter(w.out)
	write := func(name string, data []byte) error {
		fw, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = fw.Write(data)
		return err
	}
	if err := write("[Content_Types].xml", []byte(w.contentTypesXML())); err != nil {
		return err
	}
	if err := write("_rels/.rels", []byte(rootRelsXML())); err != nil {
		return err
	}
	if err := write("xl/workbook.xml", []byte(w.workbookXML())); err != nil {
		return err
	}
	if err := write("xl/_rels/workbook.xml.rels", []byte(w.workbookRelsXML())); err != nil {
		return err
	}
	if err := write("xl/styles.xml", []byte(w.styles.xml())); err != nil {
		return err
	}
	for _, s := range w.sheets {
		if err := s.writeSheetPart(zw); err != nil {
			return err
		}
		if err := s.writeSheetRels(zw); err != nil {
			return err
		}
		if s.commentCount > 0 {
			if err := s.writeCommentsPart(zw); err != nil {
				return err
			}
			if err := s.writeVMLPart(zw); err != nil {
				return err
			}
		}
	}
	return zw.Close()
}

func (s *Sheet) writeSheetPart(zw *zip.Writer) error {
	fw, err := zw.Create(fmt.Sprintf("xl/worksheets/sheet%d.xml", s.index))
	if err != nil {
		return err
	}
	io.WriteString(fw, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="`+spreadsheetNS+`" xmlns:r="`+relationshipsNS+`">`)
	if s.minRow > 0 {
		io.WriteString(fw, `<dimension ref="`+cellRef(s.minRow, s.minCol)+`:`+cellRef(s.maxRow, s.maxCol)+`"/>`)
	}
	if s.freezeRows > 0 || s.freezeCols > 0 {
		top := cellRef(max(1, s.freezeRows+1), max(1, s.freezeCols+1))
		io.WriteString(fw, `<sheetViews><sheetView workbookViewId="0"><pane`)
		if s.freezeCols > 0 {
			io.WriteString(fw, ` xSplit="`+strconv.Itoa(s.freezeCols)+`"`)
		}
		if s.freezeRows > 0 {
			io.WriteString(fw, ` ySplit="`+strconv.Itoa(s.freezeRows)+`"`)
		}
		io.WriteString(fw, ` topLeftCell="`+top+`" state="frozen"/></sheetView></sheetViews>`)
	}
	if len(s.columnWidths) > 0 {
		cols := make([]int, 0, len(s.columnWidths))
		for c := range s.columnWidths {
			cols = append(cols, c)
		}
		sort.Ints(cols)
		io.WriteString(fw, `<cols>`)
		for _, c := range cols {
			io.WriteString(fw, `<col min="`+strconv.Itoa(c)+`" max="`+strconv.Itoa(c)+`" width="`+formatFloat(s.columnWidths[c])+`" customWidth="1"/>`)
		}
		io.WriteString(fw, `</cols>`)
	}
	io.WriteString(fw, `<sheetData>`)
	if _, err := io.Copy(fw, s.rowFile); err != nil {
		return err
	}
	io.WriteString(fw, `</sheetData>`)
	if s.autoFilter != "" {
		var b strings.Builder
		b.WriteString(`<autoFilter ref="`)
		xmlEscapeAttr(&b, s.autoFilter)
		b.WriteString(`"/>`)
		io.WriteString(fw, b.String())
	}
	if len(s.merges) > 0 {
		io.WriteString(fw, `<mergeCells count="`+strconv.Itoa(len(s.merges))+`">`)
		for _, m := range s.merges {
			var b strings.Builder
			b.WriteString(`<mergeCell ref="`)
			xmlEscapeAttr(&b, m)
			b.WriteString(`"/>`)
			io.WriteString(fw, b.String())
		}
		io.WriteString(fw, `</mergeCells>`)
	}
	if s.hyperlinkCount > 0 {
		io.WriteString(fw, `<hyperlinks>`)
		if _, err := io.Copy(fw, s.hyperlinkFile); err != nil {
			return err
		}
		io.WriteString(fw, `</hyperlinks>`)
	}
	if s.commentCount > 0 {
		io.WriteString(fw, `<legacyDrawing r:id="rIdVML"/>`)
	}
	io.WriteString(fw, `</worksheet>`)
	return nil
}

func (s *Sheet) writeSheetRels(zw *zip.Writer) error {
	if s.hyperlinkCount == 0 && s.commentCount == 0 {
		return nil
	}
	fw, err := zw.Create(fmt.Sprintf("xl/worksheets/_rels/sheet%d.xml.rels", s.index))
	if err != nil {
		return err
	}
	io.WriteString(fw, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="`+packageRelationshipsNS+`">`)
	if s.hyperlinkCount > 0 {
		if _, err := io.Copy(fw, s.hyperlinkRelFile); err != nil {
			return err
		}
	}
	if s.commentCount > 0 {
		io.WriteString(fw, `<Relationship Id="rIdComments" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="../comments`+strconv.Itoa(s.index)+`.xml"/><Relationship Id="rIdVML" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/vmlDrawing" Target="../drawings/vmlDrawing`+strconv.Itoa(s.index)+`.vml"/>`)
	}
	io.WriteString(fw, `</Relationships>`)
	return nil
}

func (s *Sheet) writeCommentsPart(zw *zip.Writer) error {
	fw, err := zw.Create(fmt.Sprintf("xl/comments%d.xml", s.index))
	if err != nil {
		return err
	}
	io.WriteString(fw, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><comments xmlns="`+spreadsheetNS+`"><authors>`)
	for _, a := range s.authors {
		io.WriteString(fw, `<author>`)
		var b strings.Builder
		xmlEscapeText(&b, a)
		io.WriteString(fw, b.String())
		io.WriteString(fw, `</author>`)
	}
	io.WriteString(fw, `</authors><commentList>`)
	if _, err := io.Copy(fw, s.commentFile); err != nil {
		return err
	}
	io.WriteString(fw, `</commentList></comments>`)
	return nil
}

func (s *Sheet) writeVMLPart(zw *zip.Writer) error {
	fw, err := zw.Create(fmt.Sprintf("xl/drawings/vmlDrawing%d.vml", s.index))
	if err != nil {
		return err
	}
	io.WriteString(fw, `<?xml version="1.0" encoding="UTF-8"?><xml xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:x="urn:schemas-microsoft-com:office:excel"><o:shapelayout v:ext="edit"><o:idmap v:ext="edit" data="1"/></o:shapelayout><v:shapetype id="_x0000_t202" coordsize="21600,21600" o:spt="202" path="m,l,21600r21600,l21600,xe"><v:stroke joinstyle="miter"/><v:path gradientshapeok="t" o:connecttype="rect"/></v:shapetype>`)
	if _, err := io.Copy(fw, s.commentVMLFile); err != nil {
		return err
	}
	io.WriteString(fw, `</xml>`)
	return nil
}

func (w *Workbook) contentTypesXML() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/>`)
	if hasComments(w.sheets) {
		b.WriteString(`<Default Extension="vml" ContentType="application/vnd.openxmlformats-officedocument.vmlDrawing"/>`)
	}
	b.WriteString(`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	for _, s := range w.sheets {
		b.WriteString(`<Override PartName="/xl/worksheets/sheet` + strconv.Itoa(s.index) + `.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`)
		if s.commentCount > 0 {
			b.WriteString(`<Override PartName="/xl/comments` + strconv.Itoa(s.index) + `.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.comments+xml"/>`)
		}
	}
	b.WriteString(`</Types>`)
	return b.String()
}
func rootRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="` + packageRelationshipsNS + `"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`
}
func (w *Workbook) workbookXML() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="` + spreadsheetNS + `" xmlns:r="` + relationshipsNS + `">`)
	if w.opts.Date1904 {
		b.WriteString(`<workbookPr date1904="1"/>`)
	}
	b.WriteString(`<sheets>`)
	for _, s := range w.sheets {
		b.WriteString(`<sheet name="`)
		xmlEscapeAttr(&b, s.name)
		b.WriteString(`" sheetId="` + strconv.Itoa(s.index) + `" r:id="rIdSheet` + strconv.Itoa(s.index) + `"/>`)
	}
	b.WriteString(`</sheets></workbook>`)
	return b.String()
}
func (w *Workbook) workbookRelsXML() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="` + packageRelationshipsNS + `">`)
	for _, s := range w.sheets {
		b.WriteString(`<Relationship Id="rIdSheet` + strconv.Itoa(s.index) + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet` + strconv.Itoa(s.index) + `.xml"/>`)
	}
	b.WriteString(`<Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`)
	return b.String()
}

func (w *Workbook) cleanup() {
	for _, s := range w.sheets {
		for _, f := range []*os.File{s.rowFile, s.hyperlinkFile, s.hyperlinkRelFile, s.commentFile, s.commentVMLFile} {
			if f != nil {
				name := f.Name()
				f.Close()
				os.Remove(name)
			}
		}
	}
	if w.ownsTempDir {
		os.RemoveAll(w.tempDir)
	}
}
func (w *Workbook) ensureOpen() error {
	if w == nil || w.closed {
		return ErrClosed
	}
	return nil
}
func (s *Sheet) ensureOpen() error {
	if s == nil || s.wb == nil || s.wb.closed || s.closed {
		return ErrClosed
	}
	return nil
}

func encodeValue(v any, date1904 bool) (kind, text string, err error) {
	switch x := v.(type) {
	case nil:
		return "", "", nil
	case string:
		return "inlineStr", x, nil
	case []byte:
		return "inlineStr", string(x), nil
	case bool:
		if x {
			return "b", "1", nil
		}
		return "b", "0", nil
	case int:
		return "", strconv.Itoa(x), nil
	case int8:
		return "", strconv.FormatInt(int64(x), 10), nil
	case int16:
		return "", strconv.FormatInt(int64(x), 10), nil
	case int32:
		return "", strconv.FormatInt(int64(x), 10), nil
	case int64:
		return "", strconv.FormatInt(x, 10), nil
	case uint:
		return "", strconv.FormatUint(uint64(x), 10), nil
	case uint8:
		return "", strconv.FormatUint(uint64(x), 10), nil
	case uint16:
		return "", strconv.FormatUint(uint64(x), 10), nil
	case uint32:
		return "", strconv.FormatUint(uint64(x), 10), nil
	case uint64:
		return "", strconv.FormatUint(x, 10), nil
	case float32:
		return "", strconv.FormatFloat(float64(x), 'g', -1, 32), nil
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return "", "", fmt.Errorf("non-finite number")
		}
		return "", strconv.FormatFloat(x, 'g', -1, 64), nil
	case time.Time:
		return "", formatFloat(excelSerial(x, date1904)), nil
	default:
		return "", "", fmt.Errorf("unsupported value type %T", v)
	}
}
func excelSerial(t time.Time, date1904 bool) float64 {
	t = t.UTC()
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	if date1904 {
		epoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return t.Sub(epoch).Hours() / 24
}
func formatFloat(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
func needsPreserve(s string) bool {
	return strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") || strings.Contains(s, "\n") || strings.Contains(s, "\t")
}
func xmlEscapeText(b *strings.Builder, s string) {
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(s))
	b.Write(buf.Bytes())
}
func xmlEscapeAttr(b *strings.Builder, s string) { xmlEscapeText(b, s) }
func cellRef(row, col int) string {
	var letters [3]byte
	i := len(letters)
	n := col
	for n > 0 {
		i--
		n--
		letters[i] = byte('A' + n%26)
		n /= 26
	}
	return string(letters[i:]) + strconv.Itoa(row)
}
func validateSheetName(name string) error {
	if len([]rune(name)) > 31 {
		return fmt.Errorf("sheet stream: sheet name exceeds 31 characters")
	}
	if strings.ContainsAny(name, "[]:*?/\\") {
		return fmt.Errorf("sheet stream: invalid sheet name %q", name)
	}
	if name == "" {
		return fmt.Errorf("sheet stream: empty sheet name")
	}
	return nil
}
func hasComments(sheets []*Sheet) bool {
	for _, s := range sheets {
		if s.commentCount > 0 {
			return true
		}
	}
	return false
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// styleRegistry stores compact, deduplicated style components in memory.
type styleRegistry struct {
	styles []sheetir.CellStyle
	index  map[string]int
}

func newStyleRegistry() *styleRegistry {
	return &styleRegistry{styles: []sheetir.CellStyle{{}}, index: map[string]int{styleKey(sheetir.CellStyle{}): 0}}
}
func (r *styleRegistry) count() int { return len(r.styles) }
func (r *styleRegistry) add(s sheetir.CellStyle) int {
	k := styleKey(s)
	if id, ok := r.index[k]; ok {
		return id
	}
	id := len(r.styles)
	r.styles = append(r.styles, s)
	r.index[k] = id
	return id
}
func styleKey(s sheetir.CellStyle) string {
	return fmt.Sprintf("%t|%t|%t|%t|%s|%s|%s|%t|%d|%g|%s|%s|%s:%s|%s:%s|%s:%s|%s:%s|%s|%t|%t", s.Bold, s.Italic, s.Underline, s.Strike, s.FontFamily, s.HAlign, s.VAlign, s.WrapText, s.TextRotation, s.FontSizePt, s.FontColor, s.BgColor, s.BorderTop.Style, s.BorderTop.Color, s.BorderRight.Style, s.BorderRight.Color, s.BorderBottom.Style, s.BorderBottom.Color, s.BorderLeft.Style, s.BorderLeft.Color, s.NumFmtCode, s.Locked, s.FormulaHidden)
}

func (r *styleRegistry) xml() string {
	type fontKey struct {
		b, i, u, st   bool
		family, color string
		size          float64
	}
	type fillKey struct{ color string }
	type borderKey struct{ t, tc, rr, rc, b, bc, l, lc string }
	fonts := []fontKey{{}}
	fills := []fillKey{{}, {}}
	borders := []borderKey{{}}
	fontIDs := map[fontKey]int{{}: 0}
	fillIDs := map[fillKey]int{{}: 0}
	borderIDs := map[borderKey]int{{}: 0}
	numFmtIDs := map[string]int{}
	nextFmt := 164
	// Excel requires gray125 as the second fill.
	fills[1] = fillKey{color: "__gray125"}
	fillIDs[fills[1]] = 1
	type xf struct {
		font, fill, border, numfmt int
		alignH, alignV             string
		wrap                       bool
		rot                        int
		locked, hidden             bool
	}
	xfs := make([]xf, len(r.styles))
	for idx, s := range r.styles {
		fk := fontKey{s.Bold, s.Italic, s.Underline, s.Strike, s.FontFamily, s.FontColor, s.FontSizePt}
		fid, ok := fontIDs[fk]
		if !ok {
			fid = len(fonts)
			fonts = append(fonts, fk)
			fontIDs[fk] = fid
		}
		fl := fillKey{s.BgColor}
		flid, ok := fillIDs[fl]
		if !ok {
			flid = len(fills)
			fills = append(fills, fl)
			fillIDs[fl] = flid
		}
		bk := borderKey{s.BorderTop.Style, s.BorderTop.Color, s.BorderRight.Style, s.BorderRight.Color, s.BorderBottom.Style, s.BorderBottom.Color, s.BorderLeft.Style, s.BorderLeft.Color}
		bid, ok := borderIDs[bk]
		if !ok {
			bid = len(borders)
			borders = append(borders, bk)
			borderIDs[bk] = bid
		}
		nfid := 0
		if s.NumFmtCode != "" {
			var ok bool
			nfid, ok = numFmtIDs[s.NumFmtCode]
			if !ok {
				nfid = nextFmt
				nextFmt++
				numFmtIDs[s.NumFmtCode] = nfid
			}
		}
		va := s.VAlign
		if va == "middle" {
			va = "center"
		}
		xfs[idx] = xf{fid, flid, bid, nfid, s.HAlign, va, s.WrapText, s.TextRotation, s.Locked, s.FormulaHidden}
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="` + spreadsheetNS + `">`)
	if len(numFmtIDs) > 0 {
		codes := make([]string, 0, len(numFmtIDs))
		for code := range numFmtIDs {
			codes = append(codes, code)
		}
		sort.Slice(codes, func(i, j int) bool { return numFmtIDs[codes[i]] < numFmtIDs[codes[j]] })
		b.WriteString(`<numFmts count="` + strconv.Itoa(len(codes)) + `">`)
		for _, code := range codes {
			b.WriteString(`<numFmt numFmtId="` + strconv.Itoa(numFmtIDs[code]) + `" formatCode="`)
			xmlEscapeAttr(&b, code)
			b.WriteString(`"/>`)
		}
		b.WriteString(`</numFmts>`)
	}
	b.WriteString(`<fonts count="` + strconv.Itoa(len(fonts)) + `">`)
	for _, f := range fonts {
		b.WriteString(`<font>`)
		if f.b {
			b.WriteString(`<b/>`)
		}
		if f.i {
			b.WriteString(`<i/>`)
		}
		if f.u {
			b.WriteString(`<u/>`)
		}
		if f.st {
			b.WriteString(`<strike/>`)
		}
		if f.size > 0 {
			b.WriteString(`<sz val="` + formatFloat(f.size) + `"/>`)
		}
		if f.color != "" {
			b.WriteString(`<color rgb="` + normalizeColor(f.color) + `"/>`)
		}
		if f.family != "" {
			b.WriteString(`<name val="`)
			xmlEscapeAttr(&b, f.family)
			b.WriteString(`"/>`)
		}
		b.WriteString(`</font>`)
	}
	b.WriteString(`</fonts>`)
	b.WriteString(`<fills count="` + strconv.Itoa(len(fills)) + `">`)
	for _, f := range fills {
		switch f.color {
		case "":
			b.WriteString(`<fill><patternFill patternType="none"/></fill>`)
		case "__gray125":
			b.WriteString(`<fill><patternFill patternType="gray125"/></fill>`)
		default:
			b.WriteString(`<fill><patternFill patternType="solid"><fgColor rgb="` + normalizeColor(f.color) + `"/><bgColor indexed="64"/></patternFill></fill>`)
		}
	}
	b.WriteString(`</fills>`)
	b.WriteString(`<borders count="` + strconv.Itoa(len(borders)) + `">`)
	for _, br := range borders {
		b.WriteString(`<border>`)
		writeBorderSide(&b, "left", br.l, br.lc)
		writeBorderSide(&b, "right", br.rr, br.rc)
		writeBorderSide(&b, "top", br.t, br.tc)
		writeBorderSide(&b, "bottom", br.b, br.bc)
		b.WriteString(`<diagonal/></border>`)
	}
	b.WriteString(`</borders>`)
	b.WriteString(`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>`)
	b.WriteString(`<cellXfs count="` + strconv.Itoa(len(xfs)) + `">`)
	for _, x := range xfs {
		b.WriteString(`<xf numFmtId="` + strconv.Itoa(x.numfmt) + `" fontId="` + strconv.Itoa(x.font) + `" fillId="` + strconv.Itoa(x.fill) + `" borderId="` + strconv.Itoa(x.border) + `" xfId="0"`)
		if x.numfmt != 0 {
			b.WriteString(` applyNumberFormat="1"`)
		}
		if x.font != 0 {
			b.WriteString(` applyFont="1"`)
		}
		if x.fill != 0 {
			b.WriteString(` applyFill="1"`)
		}
		if x.border != 0 {
			b.WriteString(` applyBorder="1"`)
		}
		if x.alignH != "" || x.alignV != "" || x.wrap || x.rot != 0 {
			b.WriteString(` applyAlignment="1"`)
		}
		if x.locked || x.hidden {
			b.WriteString(` applyProtection="1"`)
		}
		b.WriteByte('>')
		if x.alignH != "" || x.alignV != "" || x.wrap || x.rot != 0 {
			b.WriteString(`<alignment`)
			if x.alignH != "" {
				b.WriteString(` horizontal="` + x.alignH + `"`)
			}
			if x.alignV != "" {
				b.WriteString(` vertical="` + x.alignV + `"`)
			}
			if x.wrap {
				b.WriteString(` wrapText="1"`)
			}
			if x.rot != 0 {
				b.WriteString(` textRotation="` + strconv.Itoa(x.rot) + `"`)
			}
			b.WriteString(`/>`)
		}
		if x.locked || x.hidden {
			b.WriteString(`<protection`)
			if x.locked {
				b.WriteString(` locked="1"`)
			}
			if x.hidden {
				b.WriteString(` hidden="1"`)
			}
			b.WriteString(`/>`)
		}
		b.WriteString(`</xf>`)
	}
	b.WriteString(`</cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`)
	return b.String()
}
func writeBorderSide(b *strings.Builder, name, style, color string) {
	b.WriteByte('<')
	b.WriteString(name)
	if style != "" {
		b.WriteString(` style="`)
		xmlEscapeAttr(b, style)
		b.WriteByte('"')
	}
	if color == "" {
		b.WriteString(`/>`)
		return
	}
	b.WriteByte('>')
	b.WriteString(`<color rgb="` + normalizeColor(color) + `"/>`)
	b.WriteString(`</` + name + `>`)
}
func normalizeColor(s string) string {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	s = strings.ToUpper(s)
	if len(s) == 6 {
		return "FF" + s
	}
	return s
}

// TempFiles returns the current worksheet backing files. It is intended for
// diagnostics and tests; callers must not modify them.
func (w *Workbook) TempFiles() []string {
	if w == nil {
		return nil
	}
	out := []string{}
	for _, s := range w.sheets {
		for _, f := range []*os.File{s.rowFile, s.hyperlinkFile, s.hyperlinkRelFile, s.commentFile, s.commentVMLFile} {
			if f != nil {
				out = append(out, filepath.Clean(f.Name()))
			}
		}
	}
	return out
}
