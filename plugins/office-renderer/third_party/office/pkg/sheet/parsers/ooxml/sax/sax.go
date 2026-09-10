// Package sax provides a streaming event API for XLSX files.
package sax

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

const (
	relTypeOfficeDocument = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	relTypeWorksheet      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet"
	relTypeSharedStrings  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings"
	relTypeComments       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments"
	relTypeHyperlink      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"

	strictRelPrefix       = "http://purl.oclc.org/ooxml/officeDocument/relationships/"
	transitionalRelPrefix = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
)

// EventType identifies a streaming SpreadsheetML event.
type EventType string

const (
	EventStartWorkbook   EventType = "startWorkbook"
	EventEndWorkbook     EventType = "endWorkbook"
	EventStartSheet      EventType = "startSheet"
	EventEndSheet        EventType = "endSheet"
	EventStartRow        EventType = "startRow"
	EventEndRow          EventType = "endRow"
	EventCell            EventType = "cell"
	EventComment         EventType = "comment"
	EventSheetFormat     EventType = "sheetFormat"
	EventColumn          EventType = "column"
	EventSheetView       EventType = "sheetView"
	EventPaneSelection   EventType = "paneSelection"
	EventMergedCell      EventType = "mergedCell"
	EventPrintOptions    EventType = "printOptions"
	EventPageMargins     EventType = "pageMargins"
	EventPageSetup       EventType = "pageSetup"
	EventRowBreaks       EventType = "rowBreaks"
	EventColBreaks       EventType = "colBreaks"
	EventSheetProtection EventType = "sheetProtection"
	EventDataValidation  EventType = "dataValidation"
	EventConditionalFmt  EventType = "conditionalFormatting"
	EventAutoFilter      EventType = "autoFilter"
)

// Event is a single streamed SpreadsheetML event.
type Event struct {
	Type               EventType
	PartName           string
	SheetIndex         int
	SheetID            int
	SheetName          string
	Hidden             bool
	RowIndex           int
	ColIndex           int
	CellRef            string
	CellType           string
	StyleIndex         int
	Formula            string
	FormulaType        string
	FormulaRef         string
	FormulaSharedIndex *int
	RawValue           string
	Text               string
	HyperlinkTarget    string
	HyperlinkLocation  string
	HyperlinkDisplay   string
	HyperlinkTooltip   string
	CommentAuthor      string
	CommentText        string
	RowHeight          float64
	RowHidden          bool
	RowStyleIndex      int
	RowOutlineLevel    int
	RowCollapsed       bool

	DefaultColWidth    float64
	DefaultRowHeight   float64
	DefaultRowCustom   bool
	DefaultRowZero     bool
	RowOutlineLevelMax int
	ColOutlineLevelMax int

	ColumnMin    int
	ColumnMax    int
	ColumnWidth  float64
	ColumnHidden bool

	ZoomScale             int
	ShowGridLines         *bool
	PaneState             string
	SplitX                float64
	SplitY                float64
	TopLeftCell           string
	ActivePane            string
	SelectionPane         string
	SelectionActiveCell   string
	SelectionActiveCellID int
	SelectionSqRef        string

	MergeRef string

	PrintOptions    *sheetparts.PrintOptions
	PageMargins     *sheetparts.PageMargins
	PageSetup       *sheetparts.PageSetup
	PageBreaks      *sheetparts.PageBreaks
	SheetProtection *sheetparts.SheetProtection
	DataValidation  *sheetparts.DataValidation
	ConditionalFmt  *sheetparts.ConditionalFmt
	AutoFilter      *sheetparts.AutoFilter
	CommentRichText *sheetparts.RichText
}

// Handler consumes spreadsheet events.
type Handler interface {
	HandleEvent(Event) error
}

// HandlerFunc adapts a function to the Handler interface.
type HandlerFunc func(Event) error

// HandleEvent implements Handler.
func (f HandlerFunc) HandleEvent(event Event) error {
	return f(event)
}

// Options control which spreadsheet parts are emitted.
type Options struct {
	IncludeComments     bool
	IncludeHiddenSheets bool
}

// WalkPath streams SpreadsheetML events from an XLSX-family file on disk.
func WalkPath(path string, handler Handler, opts Options) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	return WalkReaderAt(file, info.Size(), handler, opts)
}

// WalkBytes streams SpreadsheetML events from an in-memory XLSX-family payload.
func WalkBytes(data []byte, handler Handler, opts Options) error {
	return WalkReaderAt(bytes.NewReader(data), int64(len(data)), handler, opts)
}

// WalkStream streams SpreadsheetML events from a non-seekable XLSX-family stream.
func WalkStream(r io.Reader, handler Handler, opts Options) error {
	file, err := os.CreateTemp("", "office-xlsx-sax-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer func() {
		_ = os.Remove(name)
	}()
	defer func() {
		_ = file.Close()
	}()

	size, err := io.Copy(file, r)
	if err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return WalkReaderAt(file, size, handler, opts)
}

// WalkReaderAt streams SpreadsheetML events from a seekable XLSX-family reader.
func WalkReaderAt(r io.ReaderAt, size int64, handler Handler, opts Options) error {
	if handler == nil {
		return fmt.Errorf("sheet sax: handler cannot be nil")
	}

	zr, err := zip.NewReader(r, size)
	if err == nil {
		files := make(map[string]*zip.File, len(zr.File))
		for _, file := range zr.File {
			files[normalizePartName(file.Name)] = file
		}
		return walkSource(zipSource{files: files}, handler, opts)
	}

	pkg, openErr := opc.OpenReaderAt(r, size)
	if openErr != nil {
		return openErr
	}
	return walkSource(opcSource{pkg: pkg}, handler, opts)
}

type packageSource interface {
	openPart(name string) (io.ReadCloser, error)
	relationships(sourcePart string) (*rels.Document, bool, error)
}

type zipSource struct {
	files map[string]*zip.File
}

func (s zipSource) openPart(name string) (io.ReadCloser, error) {
	file := s.files[normalizePartName(name)]
	if file == nil {
		return nil, fmt.Errorf("sheet sax: part %q not found", name)
	}
	return file.Open()
}

func (s zipSource) relationships(sourcePart string) (*rels.Document, bool, error) {
	partName := rels.RelationshipsPartName(sourcePart)
	file := s.files[normalizePartName(partName)]
	if file == nil {
		return nil, false, nil
	}
	rc, err := file.Open()
	if err != nil {
		return nil, false, err
	}
	defer func() {
		_ = rc.Close()
	}()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, false, err
	}
	doc, err := rels.Parse(data)
	if err != nil {
		return nil, false, err
	}
	return doc, true, nil
}

type opcSource struct {
	pkg *opc.Package
}

func (s opcSource) openPart(name string) (io.ReadCloser, error) {
	part, ok := s.pkg.Part(name)
	if !ok {
		return nil, fmt.Errorf("sheet sax: part %q not found", name)
	}
	return io.NopCloser(bytes.NewReader(part.Data)), nil
}

func (s opcSource) relationships(sourcePart string) (*rels.Document, bool, error) {
	return s.pkg.Relationships(sourcePart)
}

type sheetRef struct {
	name     string
	partName string
	sheetID  int
	hidden   bool
}

type rowMeta struct {
	index        int
	height       float64
	hidden       bool
	styleIndex   int
	outlineLevel int
	collapsed    bool
}

type cellMeta struct {
	ref                string
	styleIndex         int
	cellType           string
	formula            string
	formulaType        string
	formulaRef         string
	formulaSharedIndex *int
	rawValue           string
	inlineText         string
}

type hyperlinkInfo struct {
	ref      string
	target   string
	location string
	display  string
	tooltip  string
}

type commentInfo struct {
	ref    string
	author string
	text   string
	rich   sheetparts.RichText
}

func walkSource(src packageSource, handler Handler, opts Options) error {
	mainPart, err := findMainPart(src)
	if err != nil {
		return err
	}
	workbook, err := loadWorkbook(src, mainPart)
	if err != nil {
		return err
	}
	sheets, err := resolveSheetRefs(src, mainPart, workbook)
	if err != nil {
		return err
	}
	sharedStrings, err := loadSharedStrings(src, mainPart)
	if err != nil {
		return err
	}

	if err := handler.HandleEvent(Event{Type: EventStartWorkbook, PartName: mainPart}); err != nil {
		return err
	}
	for sheetIndex, sheet := range sheets {
		if sheet.hidden && !opts.IncludeHiddenSheets {
			continue
		}
		if err := handler.HandleEvent(Event{
			Type:       EventStartSheet,
			PartName:   sheet.partName,
			SheetIndex: sheetIndex,
			SheetID:    sheet.sheetID,
			SheetName:  sheet.name,
			Hidden:     sheet.hidden,
		}); err != nil {
			return err
		}
		if err := walkWorksheet(src, handler, opts, sheetIndex, sheet, sharedStrings); err != nil {
			return err
		}
		if err := handler.HandleEvent(Event{
			Type:       EventEndSheet,
			PartName:   sheet.partName,
			SheetIndex: sheetIndex,
			SheetID:    sheet.sheetID,
			SheetName:  sheet.name,
			Hidden:     sheet.hidden,
		}); err != nil {
			return err
		}
	}
	return handler.HandleEvent(Event{Type: EventEndWorkbook, PartName: mainPart})
}

type sheetFormatMeta struct {
	defaultColWidth, defaultRowHeight      float64
	customHeight, zeroHeight               bool
	rowOutlineLevelMax, colOutlineLevelMax int
}
type columnMeta struct {
	min, max int
	width    float64
	hidden   bool
}
type sheetViewMeta struct {
	zoomScale     int
	showGridLines *bool
}
type paneMeta struct {
	state                   string
	xSplit, ySplit          float64
	topLeftCell, activePane string
}
type selectionMeta struct {
	pane, activeCell, sqRef string
	activeCellID            int
}

func attrInt(start xml.StartElement, name string) int {
	v, _ := strconv.Atoi(attrValue(start, name))
	return v
}
func attrFloat(start xml.StartElement, name string) float64 {
	v, _ := strconv.ParseFloat(attrValue(start, name), 64)
	return v
}
func attrBoolPtr(start xml.StartElement, name string) *bool {
	raw := attrValue(start, name)
	if raw == "" {
		return nil
	}
	v := raw == "1" || strings.EqualFold(raw, "true")
	return &v
}
func parseSheetFormatMeta(start xml.StartElement) sheetFormatMeta {
	return sheetFormatMeta{
		defaultColWidth: attrFloat(start, "defaultColWidth"), defaultRowHeight: attrFloat(start, "defaultRowHeight"),
		customHeight:       attrValue(start, "customHeight") == "1" || strings.EqualFold(attrValue(start, "customHeight"), "true"),
		zeroHeight:         attrValue(start, "zeroHeight") == "1" || strings.EqualFold(attrValue(start, "zeroHeight"), "true"),
		rowOutlineLevelMax: attrInt(start, "outlineLevelRow"), colOutlineLevelMax: attrInt(start, "outlineLevelCol"),
	}
}
func parseColumnMeta(start xml.StartElement) columnMeta {
	return columnMeta{min: attrInt(start, "min"), max: attrInt(start, "max"), width: attrFloat(start, "width"),
		hidden: attrValue(start, "hidden") == "1" || strings.EqualFold(attrValue(start, "hidden"), "true")}
}
func parseSheetViewMeta(start xml.StartElement) sheetViewMeta {
	return sheetViewMeta{zoomScale: attrInt(start, "zoomScale"), showGridLines: attrBoolPtr(start, "showGridLines")}
}
func parsePaneMeta(start xml.StartElement) paneMeta {
	return paneMeta{state: attrValue(start, "state"), xSplit: attrFloat(start, "xSplit"), ySplit: attrFloat(start, "ySplit"),
		topLeftCell: attrValue(start, "topLeftCell"), activePane: attrValue(start, "activePane")}
}
func parseSelectionMeta(start xml.StartElement) selectionMeta {
	return selectionMeta{pane: attrValue(start, "pane"), activeCell: attrValue(start, "activeCell"),
		activeCellID: attrInt(start, "activeCellId"), sqRef: attrValue(start, "sqref")}
}

func walkWorksheet(src packageSource, handler Handler, opts Options, sheetIndex int, sheet sheetRef, sharedStrings []string) error {
	hyperlinks, err := loadWorksheetHyperlinks(src, sheet.partName)
	if err != nil {
		return err
	}
	comments, err := loadWorksheetComments(src, sheet.partName, opts.IncludeComments)
	if err != nil {
		return err
	}
	commentByRef := make(map[string]commentInfo, len(comments))
	for _, comment := range comments {
		commentByRef[strings.ToUpper(comment.ref)] = comment
	}
	emittedComments := make(map[string]bool, len(comments))

	rc, err := src.openPart(sheet.partName)
	if err != nil {
		return err
	}
	defer func() {
		_ = rc.Close()
	}()

	dec := xml.NewDecoder(rc)
	var currentRow rowMeta
	var rowOpen bool
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "sheetFormatPr":
				format := parseSheetFormatMeta(t)
				if err := handler.HandleEvent(Event{
					Type: EventSheetFormat, PartName: sheet.partName, SheetIndex: sheetIndex,
					SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden,
					DefaultColWidth: format.defaultColWidth, DefaultRowHeight: format.defaultRowHeight,
					DefaultRowCustom: format.customHeight, DefaultRowZero: format.zeroHeight,
					RowOutlineLevelMax: format.rowOutlineLevelMax, ColOutlineLevelMax: format.colOutlineLevelMax,
				}); err != nil {
					return err
				}
			case "col":
				col := parseColumnMeta(t)
				if err := handler.HandleEvent(Event{
					Type: EventColumn, PartName: sheet.partName, SheetIndex: sheetIndex,
					SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden,
					ColumnMin: col.min, ColumnMax: col.max, ColumnWidth: col.width, ColumnHidden: col.hidden,
				}); err != nil {
					return err
				}
			case "sheetView":
				view := parseSheetViewMeta(t)
				if err := handler.HandleEvent(Event{
					Type: EventSheetView, PartName: sheet.partName, SheetIndex: sheetIndex,
					SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden,
					ZoomScale: view.zoomScale, ShowGridLines: view.showGridLines,
				}); err != nil {
					return err
				}
			case "pane":
				pane := parsePaneMeta(t)
				if err := handler.HandleEvent(Event{
					Type: EventSheetView, PartName: sheet.partName, SheetIndex: sheetIndex,
					SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden,
					PaneState: pane.state, SplitX: pane.xSplit, SplitY: pane.ySplit,
					TopLeftCell: pane.topLeftCell, ActivePane: pane.activePane,
				}); err != nil {
					return err
				}
			case "selection":
				sel := parseSelectionMeta(t)
				if err := handler.HandleEvent(Event{
					Type: EventPaneSelection, PartName: sheet.partName, SheetIndex: sheetIndex,
					SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden,
					SelectionPane: sel.pane, SelectionActiveCell: sel.activeCell,
					SelectionActiveCellID: sel.activeCellID, SelectionSqRef: sel.sqRef,
				}); err != nil {
					return err
				}
			case "mergeCell":
				if ref := attrValue(t, "ref"); ref != "" {
					if err := handler.HandleEvent(Event{
						Type: EventMergedCell, PartName: sheet.partName, SheetIndex: sheetIndex,
						SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden, MergeRef: ref,
					}); err != nil {
						return err
					}
				}
			case "printOptions":
				var value sheetparts.PrintOptions
				if err := dec.DecodeElement(&value, &t); err != nil {
					return err
				}
				if err := handler.HandleEvent(Event{Type: EventPrintOptions, PartName: sheet.partName, SheetIndex: sheetIndex, SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden, PrintOptions: &value}); err != nil {
					return err
				}
			case "pageMargins":
				var value sheetparts.PageMargins
				if err := dec.DecodeElement(&value, &t); err != nil {
					return err
				}
				if err := handler.HandleEvent(Event{Type: EventPageMargins, PartName: sheet.partName, SheetIndex: sheetIndex, SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden, PageMargins: &value}); err != nil {
					return err
				}
			case "pageSetup":
				var value sheetparts.PageSetup
				if err := dec.DecodeElement(&value, &t); err != nil {
					return err
				}
				if err := handler.HandleEvent(Event{Type: EventPageSetup, PartName: sheet.partName, SheetIndex: sheetIndex, SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden, PageSetup: &value}); err != nil {
					return err
				}
			case "rowBreaks", "colBreaks":
				var value sheetparts.PageBreaks
				if err := dec.DecodeElement(&value, &t); err != nil {
					return err
				}
				typ := EventRowBreaks
				if t.Name.Local == "colBreaks" {
					typ = EventColBreaks
				}
				if err := handler.HandleEvent(Event{Type: typ, PartName: sheet.partName, SheetIndex: sheetIndex, SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden, PageBreaks: &value}); err != nil {
					return err
				}
			case "sheetProtection":
				var value sheetparts.SheetProtection
				if err := dec.DecodeElement(&value, &t); err != nil {
					return err
				}
				if err := handler.HandleEvent(Event{Type: EventSheetProtection, PartName: sheet.partName, SheetIndex: sheetIndex, SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden, SheetProtection: &value}); err != nil {
					return err
				}
			case "dataValidation":
				var value sheetparts.DataValidation
				if err := dec.DecodeElement(&value, &t); err != nil {
					return err
				}
				if err := handler.HandleEvent(Event{Type: EventDataValidation, PartName: sheet.partName, SheetIndex: sheetIndex, SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden, DataValidation: &value}); err != nil {
					return err
				}
			case "conditionalFormatting":
				var value sheetparts.ConditionalFmt
				if err := dec.DecodeElement(&value, &t); err != nil {
					return err
				}
				if err := handler.HandleEvent(Event{Type: EventConditionalFmt, PartName: sheet.partName, SheetIndex: sheetIndex, SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden, ConditionalFmt: &value}); err != nil {
					return err
				}
			case "autoFilter":
				var value sheetparts.AutoFilter
				if err := dec.DecodeElement(&value, &t); err != nil {
					return err
				}
				if err := handler.HandleEvent(Event{Type: EventAutoFilter, PartName: sheet.partName, SheetIndex: sheetIndex, SheetID: sheet.sheetID, SheetName: sheet.name, Hidden: sheet.hidden, AutoFilter: &value}); err != nil {
					return err
				}
			case "row":
				currentRow = parseRowMeta(t)
				rowOpen = true
				if err := handler.HandleEvent(Event{
					Type:            EventStartRow,
					PartName:        sheet.partName,
					SheetIndex:      sheetIndex,
					SheetID:         sheet.sheetID,
					SheetName:       sheet.name,
					Hidden:          sheet.hidden,
					RowIndex:        currentRow.index,
					RowHeight:       currentRow.height,
					RowHidden:       currentRow.hidden,
					RowStyleIndex:   currentRow.styleIndex,
					RowOutlineLevel: currentRow.outlineLevel,
					RowCollapsed:    currentRow.collapsed,
				}); err != nil {
					return err
				}
			case "c":
				cell, err := parseCellMeta(dec, t)
				if err != nil {
					return err
				}
				colIndex := colIndexFromRef(cell.ref)
				text := resolveCellText(cell, sharedStrings)
				hyperlink, _ := resolveCellHyperlink(cell.ref, hyperlinks)
				comment, hasComment := commentByRef[strings.ToUpper(cell.ref)]
				if hasComment {
					emittedComments[strings.ToUpper(cell.ref)] = true
				}
				if err := handler.HandleEvent(Event{
					Type:               EventCell,
					PartName:           sheet.partName,
					SheetIndex:         sheetIndex,
					SheetID:            sheet.sheetID,
					SheetName:          sheet.name,
					Hidden:             sheet.hidden,
					RowIndex:           currentRow.index,
					ColIndex:           colIndex,
					CellRef:            cell.ref,
					CellType:           cell.cellType,
					StyleIndex:         cell.styleIndex,
					Formula:            cell.formula,
					FormulaType:        cell.formulaType,
					FormulaRef:         cell.formulaRef,
					FormulaSharedIndex: cell.formulaSharedIndex,
					RawValue:           cell.rawValue,
					Text:               text,
					HyperlinkTarget:    hyperlink.target,
					HyperlinkLocation:  hyperlink.location,
					HyperlinkDisplay:   hyperlink.display,
					HyperlinkTooltip:   hyperlink.tooltip,
					CommentAuthor:      comment.author,
					CommentText:        comment.text,
					CommentRichText:    &comment.rich,
					RowHeight:          currentRow.height,
					RowHidden:          currentRow.hidden,
					RowStyleIndex:      currentRow.styleIndex,
					RowOutlineLevel:    currentRow.outlineLevel,
					RowCollapsed:       currentRow.collapsed,
				}); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name.Local == "row" && rowOpen {
				if err := handler.HandleEvent(Event{
					Type:            EventEndRow,
					PartName:        sheet.partName,
					SheetIndex:      sheetIndex,
					SheetID:         sheet.sheetID,
					SheetName:       sheet.name,
					Hidden:          sheet.hidden,
					RowIndex:        currentRow.index,
					RowHeight:       currentRow.height,
					RowHidden:       currentRow.hidden,
					RowStyleIndex:   currentRow.styleIndex,
					RowOutlineLevel: currentRow.outlineLevel,
					RowCollapsed:    currentRow.collapsed,
				}); err != nil {
					return err
				}
				rowOpen = false
			}
		}
	}

	if opts.IncludeComments {
		for _, comment := range comments {
			key := strings.ToUpper(comment.ref)
			if emittedComments[key] {
				continue
			}
			colIndex := colIndexFromRef(comment.ref)
			rowIndex := rowIndexFromRef(comment.ref)
			if err := handler.HandleEvent(Event{
				Type:            EventComment,
				PartName:        sheet.partName,
				SheetIndex:      sheetIndex,
				SheetID:         sheet.sheetID,
				SheetName:       sheet.name,
				Hidden:          sheet.hidden,
				RowIndex:        rowIndex,
				ColIndex:        colIndex,
				CellRef:         comment.ref,
				CommentAuthor:   comment.author,
				CommentText:     comment.text,
				CommentRichText: &comment.rich,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func findMainPart(src packageSource) (string, error) {
	doc, ok, err := src.relationships("")
	if err != nil {
		return "", err
	}
	if !ok || doc == nil {
		return "", fmt.Errorf("sheet sax: root relationships not found")
	}
	for _, relationship := range doc.Relationships {
		if canonicalRelationshipType(relationship.Type) == relTypeOfficeDocument && !relationship.IsExternal() {
			return rels.ResolveTarget("", relationship.Target), nil
		}
	}
	return "", fmt.Errorf("sheet sax: officeDocument relationship not found")
}

func loadWorkbook(src packageSource, mainPart string) (*sheetparts.Workbook, error) {
	rc, err := src.openPart(mainPart)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rc.Close()
	}()

	var workbook sheetparts.Workbook
	if err := xml.NewDecoder(rc).Decode(&workbook); err != nil {
		return nil, err
	}
	return &workbook, nil
}

func resolveSheetRefs(src packageSource, mainPart string, workbook *sheetparts.Workbook) ([]sheetRef, error) {
	doc, ok, err := src.relationships(mainPart)
	if err != nil {
		return nil, err
	}
	if !ok || doc == nil {
		return nil, fmt.Errorf("sheet sax: workbook relationships not found")
	}

	targets := make(map[string]string)
	for _, relationship := range doc.Relationships {
		if canonicalRelationshipType(relationship.Type) == relTypeWorksheet && !relationship.IsExternal() {
			targets[strings.TrimSpace(relationship.ID)] = rels.ResolveTarget(mainPart, relationship.Target)
		}
	}

	out := make([]sheetRef, 0, len(workbook.Sheets))
	for _, sheet := range workbook.Sheets {
		partName := targets[strings.TrimSpace(sheet.RelID)]
		if partName == "" {
			continue
		}
		out = append(out, sheetRef{
			name:     sheet.Name,
			partName: partName,
			sheetID:  sheet.SheetID,
			hidden:   strings.EqualFold(sheet.State, "hidden") || strings.EqualFold(sheet.State, "veryHidden"),
		})
	}
	return out, nil
}

func loadSharedStrings(src packageSource, mainPart string) ([]string, error) {
	doc, ok, err := src.relationships(mainPart)
	if err != nil || !ok || doc == nil {
		return nil, err
	}
	for _, relationship := range doc.Relationships {
		if canonicalRelationshipType(relationship.Type) != relTypeSharedStrings || relationship.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPart, relationship.Target)
		rc, err := src.openPart(partName)
		if err != nil {
			return nil, err
		}
		var shared sheetparts.SharedStrings
		decodeErr := xml.NewDecoder(rc).Decode(&shared)
		_ = rc.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}
		out := make([]string, len(shared.Items))
		for i := range shared.Items {
			out[i] = shared.Items[i].PlainValue()
		}
		return out, nil
	}
	return nil, nil
}

func loadWorksheetHyperlinks(src packageSource, sheetPart string) ([]hyperlinkInfo, error) {
	doc, ok, err := src.relationships(sheetPart)
	if err != nil {
		return nil, err
	}
	externalTargets := make(map[string]string)
	if ok && doc != nil {
		for _, relationship := range doc.Relationships {
			if canonicalRelationshipType(relationship.Type) == relTypeHyperlink && relationship.IsExternal() {
				externalTargets[strings.TrimSpace(relationship.ID)] = strings.TrimSpace(relationship.Target)
			}
		}
	}

	rc, err := src.openPart(sheetPart)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rc.Close()
	}()

	dec := xml.NewDecoder(rc)
	var out []hyperlinkInfo
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "hyperlink" {
			continue
		}
		info := hyperlinkInfo{
			ref:      strings.TrimSpace(attrValue(start, "ref")),
			location: strings.TrimSpace(attrValue(start, "location")),
			display:  strings.TrimSpace(attrValue(start, "display")),
			tooltip:  strings.TrimSpace(attrValue(start, "tooltip")),
		}
		relID := strings.TrimSpace(attrValue(start, "id"))
		if relID != "" {
			info.target = externalTargets[relID]
		}
		out = append(out, info)
	}
	return out, nil
}

func loadWorksheetComments(src packageSource, sheetPart string, include bool) ([]commentInfo, error) {
	if !include {
		return nil, nil
	}
	doc, ok, err := src.relationships(sheetPart)
	if err != nil || !ok || doc == nil {
		return nil, err
	}
	for _, relationship := range doc.Relationships {
		if canonicalRelationshipType(relationship.Type) != relTypeComments || relationship.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(sheetPart, relationship.Target)
		rc, err := src.openPart(partName)
		if err != nil {
			return nil, err
		}
		var comments sheetparts.Comments
		decodeErr := xml.NewDecoder(rc).Decode(&comments)
		_ = rc.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}
		out := make([]commentInfo, 0, len(comments.CommentList))
		for _, comment := range comments.CommentList {
			author := ""
			if comment.AuthorID >= 0 && comment.AuthorID < len(comments.Authors) {
				author = comments.Authors[comment.AuthorID]
			}
			out = append(out, commentInfo{
				ref:    strings.TrimSpace(comment.Ref),
				author: author,
				text:   comment.Text.PlainValue(),
				rich:   comment.Text,
			})
		}
		return out, nil
	}
	return nil, nil
}

func parseRowMeta(start xml.StartElement) rowMeta {
	return rowMeta{
		index:        parseIntAttr(attrValue(start, "r")),
		height:       parseFloatAttr(attrValue(start, "ht")),
		hidden:       parseBoolAttr(attrValue(start, "hidden")),
		styleIndex:   parseIntAttr(attrValue(start, "s")),
		outlineLevel: parseIntAttr(attrValue(start, "outlineLevel")),
		collapsed:    parseBoolAttr(attrValue(start, "collapsed")),
	}
}

func parseCellMeta(dec *xml.Decoder, start xml.StartElement) (cellMeta, error) {
	cell := cellMeta{
		ref:        strings.TrimSpace(attrValue(start, "r")),
		styleIndex: parseIntAttr(attrValue(start, "s")),
		cellType:   strings.TrimSpace(attrValue(start, "t")),
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return cell, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "f":
				var formula sheetparts.Formula
				if err := dec.DecodeElement(&formula, &t); err != nil {
					return cell, err
				}
				cell.formula = strings.TrimSpace(formula.Text)
				cell.formulaType = strings.TrimSpace(formula.Type)
				cell.formulaRef = strings.TrimSpace(formula.Ref)
				cell.formulaSharedIndex = formula.SharedIndex
			case "v":
				var value string
				if err := dec.DecodeElement(&value, &t); err != nil {
					return cell, err
				}
				cell.rawValue = value
			case "is":
				var rich sheetparts.RichText
				if err := dec.DecodeElement(&rich, &t); err != nil {
					return cell, err
				}
				cell.inlineText = rich.PlainValue()
			default:
				if err := dec.Skip(); err != nil {
					return cell, err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return cell, nil
			}
		}
	}
}

func resolveCellText(cell cellMeta, sharedStrings []string) string {
	switch strings.TrimSpace(cell.cellType) {
	case "s":
		idx := parseIntAttr(strings.TrimSpace(cell.rawValue))
		if idx >= 0 && idx < len(sharedStrings) {
			return sharedStrings[idx]
		}
		return ""
	case "inlineStr":
		return cell.inlineText
	case "b":
		switch strings.TrimSpace(cell.rawValue) {
		case "1":
			return "TRUE"
		case "0":
			return "FALSE"
		default:
			return strings.TrimSpace(cell.rawValue)
		}
	case "str", "e":
		return strings.TrimSpace(cell.rawValue)
	default:
		if cell.inlineText != "" {
			return cell.inlineText
		}
		return strings.TrimSpace(cell.rawValue)
	}
}

func resolveCellHyperlink(ref string, hyperlinks []hyperlinkInfo) (hyperlinkInfo, bool) {
	ref = strings.ToUpper(strings.TrimSpace(ref))
	if ref == "" {
		return hyperlinkInfo{}, false
	}
	for _, hyperlink := range hyperlinks {
		if cellInRef(ref, hyperlink.ref) {
			return hyperlink, true
		}
	}
	return hyperlinkInfo{}, false
}

func cellInRef(cellRef, ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	if bang := strings.LastIndex(ref, "!"); bang >= 0 {
		ref = ref[bang+1:]
	}
	ref = strings.ReplaceAll(strings.ToUpper(ref), "$", "")
	cellRef = strings.ReplaceAll(strings.ToUpper(cellRef), "$", "")
	if ref == cellRef {
		return true
	}
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return false
	}
	cellRow := rowIndexFromRef(cellRef)
	cellCol := colIndexFromRef(cellRef)
	startRow := rowIndexFromRef(parts[0])
	startCol := colIndexFromRef(parts[0])
	endRow := rowIndexFromRef(parts[1])
	endCol := colIndexFromRef(parts[1])
	if startRow == 0 || startCol == 0 || endRow == 0 || endCol == 0 || cellRow == 0 || cellCol == 0 {
		return false
	}
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	if startCol > endCol {
		startCol, endCol = endCol, startCol
	}
	return cellRow >= startRow && cellRow <= endRow && cellCol >= startCol && cellCol <= endCol
}

func rowIndexFromRef(ref string) int {
	ref = strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(ref)), "$", "")
	start := 0
	for start < len(ref) && ref[start] >= 'A' && ref[start] <= 'Z' {
		start++
	}
	if start == len(ref) {
		return 0
	}
	value, err := strconv.Atoi(ref[start:])
	if err != nil {
		return 0
	}
	return value
}

func colIndexFromRef(ref string) int {
	ref = strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(ref)), "$", "")
	if bang := strings.LastIndex(ref, "!"); bang >= 0 {
		ref = ref[bang+1:]
	}
	col := 0
	for i := 0; i < len(ref); i++ {
		ch := ref[i]
		if ch < 'A' || ch > 'Z' {
			break
		}
		col = col*26 + int(ch-'A'+1)
	}
	return col
}

func canonicalRelationshipType(value string) string {
	if strings.HasPrefix(value, strictRelPrefix) {
		return transitionalRelPrefix + strings.TrimPrefix(value, strictRelPrefix)
	}
	return value
}

func attrValue(start xml.StartElement, local string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func parseIntAttr(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return v
}

func parseFloatAttr(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseBoolAttr(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "1", "true", "t", "yes", "on":
		return true
	default:
		return false
	}
}

func normalizePartName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSpace(name)
	return strings.ToLower(name)
}
