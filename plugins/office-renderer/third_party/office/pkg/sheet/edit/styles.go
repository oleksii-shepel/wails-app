package edit

import (
	"fmt"
	"sort"
	"strings"

	sheetir "office/pkg/sheet/ir"
)

// CellStyle is a reusable named style handle for spreadsheet editing.
type CellStyle struct {
	wb        *Workbook
	name      string
	builtinID *int
	style     sheetir.CellStyle
}

// DataFormat is a reusable number-format handle.
type DataFormat struct {
	code string
}

// CreateCellStyle creates a named reusable style.
func (w *Workbook) CreateCellStyle(name string) (*CellStyle, error) {
	return w.CreateOrReuseCellStyle(name, sheetir.CellStyle{})
}

// CreateOrReuseCellStyle returns an existing equal style or creates a named style.
func (w *Workbook) CreateOrReuseCellStyle(name string, style sheetir.CellStyle) (*CellStyle, error) {
	if w == nil {
		return nil, fmt.Errorf("sheet edit: nil workbook")
	}
	w.ensureRegistries()
	name = strings.TrimSpace(name)
	if name == "" {
		name = w.nextStyleName()
	}
	key := registryKey(name)
	if existing, ok := w.styles[key]; ok {
		if existing.style != style {
			return nil, fmt.Errorf("sheet edit: cell style %q already exists", name)
		}
		return existing, nil
	}
	if existing, ok := w.CellStyleByValue(style); ok {
		return existing, nil
	}
	cs := &CellStyle{wb: w, name: name, style: style}
	w.styles[key] = cs
	if style.NumFmtCode != "" {
		if _, err := w.RegisterDataFormat(style.NumFmtCode); err != nil {
			return nil, err
		}
	}
	return cs, nil
}

// CellStyle returns a reusable style by name.
func (w *Workbook) CellStyle(name string) (*CellStyle, bool) {
	if w == nil {
		return nil, false
	}
	w.ensureRegistries()
	style, ok := w.styles[registryKey(name)]
	return style, ok
}

// CellStyleByValue returns a reusable style with equal IR values.
func (w *Workbook) CellStyleByValue(style sheetir.CellStyle) (*CellStyle, bool) {
	if w == nil {
		return nil, false
	}
	w.ensureRegistries()
	for _, existing := range w.styles {
		if existing.style == style {
			return existing, true
		}
	}
	return nil, false
}

// CellStyles returns registered reusable styles sorted by name.
func (w *Workbook) CellStyles() []*CellStyle {
	if w == nil {
		return nil
	}
	w.ensureRegistries()
	out := make([]*CellStyle, 0, len(w.styles))
	for _, style := range w.styles {
		out = append(out, style)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].name) < strings.ToLower(out[j].name)
	})
	return out
}

// CloneCellStyle copies a registered style under a new name.
func (w *Workbook) CloneCellStyle(name, newName string) (*CellStyle, error) {
	style, ok := w.CellStyle(name)
	if !ok {
		return nil, fmt.Errorf("sheet edit: cell style %q not found", name)
	}
	return w.CreateOrReuseCellStyle(newName, style.style)
}

// RemoveCellStyle removes a named style from the edit-layer registry.
func (w *Workbook) RemoveCellStyle(name string) bool {
	if w == nil {
		return false
	}
	w.ensureRegistries()
	key := registryKey(name)
	if _, ok := w.styles[key]; !ok {
		return false
	}
	delete(w.styles, key)
	return true
}

// RegisterDataFormat stores a reusable number format code.
func (w *Workbook) RegisterDataFormat(code string) (*DataFormat, error) {
	if w == nil {
		return nil, fmt.Errorf("sheet edit: nil workbook")
	}
	w.ensureRegistries()
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("sheet edit: number format code is empty")
	}
	key := registryKey(code)
	if existing, ok := w.formats[key]; ok {
		return existing, nil
	}
	format := &DataFormat{code: code}
	w.formats[key] = format
	return format, nil
}

// DataFormat returns a registered number format by code.
func (w *Workbook) DataFormat(code string) (*DataFormat, bool) {
	if w == nil {
		return nil, false
	}
	w.ensureRegistries()
	format, ok := w.formats[registryKey(code)]
	return format, ok
}

// DataFormats returns registered number formats sorted by code.
func (w *Workbook) DataFormats() []*DataFormat {
	if w == nil {
		return nil
	}
	w.ensureRegistries()
	out := make([]*DataFormat, 0, len(w.formats))
	for _, format := range w.formats {
		out = append(out, format)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].code) < strings.ToLower(out[j].code)
	})
	return out
}

// Code returns the number format code.
func (f *DataFormat) Code() string {
	if f == nil {
		return ""
	}
	return f.code
}

// Name returns the style registry name.
func (s *CellStyle) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// IR returns a copy of the underlying cell style value.
func (s *CellStyle) IR() sheetir.CellStyle {
	if s == nil {
		return sheetir.CellStyle{}
	}
	return s.style
}

// SetFont updates font properties.
func (s *CellStyle) SetFont(family string, sizePt float64, color string, bold, italic, underline, strike bool) *CellStyle {
	if s == nil {
		return s
	}
	s.style.FontFamily = family
	s.style.FontSizePt = sizePt
	s.style.FontColor = color
	s.style.Bold = bold
	s.style.Italic = italic
	s.style.Underline = underline
	s.style.Strike = strike
	return s
}

// SetFillColor updates the solid background fill color.
func (s *CellStyle) SetFillColor(color string) *CellStyle {
	if s == nil {
		return s
	}
	s.style.BgColor = color
	return s
}

// SetBorders updates all border sides.
func (s *CellStyle) SetBorders(top, right, bottom, left sheetir.BorderStyle) *CellStyle {
	if s == nil {
		return s
	}
	s.style.BorderTop = top
	s.style.BorderRight = right
	s.style.BorderBottom = bottom
	s.style.BorderLeft = left
	return s
}

// SetAlignment updates horizontal/vertical alignment and wrapping.
func (s *CellStyle) SetAlignment(horizontal, vertical string, wrapText bool, textRotation int) *CellStyle {
	if s == nil {
		return s
	}
	s.style.HAlign = horizontal
	s.style.VAlign = vertical
	s.style.WrapText = wrapText
	s.style.TextRotation = textRotation
	return s
}

// SetNumberFormat updates the style number format and registers it on the workbook.
func (s *CellStyle) SetNumberFormat(format *DataFormat) *CellStyle {
	if s == nil || format == nil {
		return s
	}
	s.style.NumFmtCode = format.code
	if s.wb != nil {
		_, _ = s.wb.RegisterDataFormat(format.code)
	}
	return s
}

// SetNumberFormatCode updates the style number format from a raw format code.
func (s *CellStyle) SetNumberFormatCode(code string) (*CellStyle, error) {
	if s == nil {
		return s, nil
	}
	if s.wb == nil {
		s.style.NumFmtCode = code
		return s, nil
	}
	format, err := s.wb.RegisterDataFormat(code)
	if err != nil {
		return s, err
	}
	s.SetNumberFormat(format)
	return s, nil
}

// SetProtection updates IR-level protection flags.
func (s *CellStyle) SetProtection(locked, formulaHidden bool) *CellStyle {
	if s == nil {
		return s
	}
	s.style.Locked = locked
	s.style.FormulaHidden = formulaHidden
	return s
}

// ApplyToCell copies the style value to a cell.
func (s *CellStyle) ApplyToCell(cell *Cell) error {
	if s == nil {
		return fmt.Errorf("sheet edit: nil cell style")
	}
	if cell == nil || cell.cell == nil {
		return fmt.Errorf("sheet edit: nil cell")
	}
	cell.cell.Style = s.style
	return nil
}

// ApplyToRow copies the style value to a row.
func (s *CellStyle) ApplyToRow(row *Row) error {
	if s == nil {
		return fmt.Errorf("sheet edit: nil cell style")
	}
	if row == nil || row.row == nil {
		return fmt.Errorf("sheet edit: nil row")
	}
	style := s.style
	row.row.Style = &style
	return nil
}

// ApplyStyle applies a reusable style to the cell.
func (c *Cell) ApplyStyle(style *CellStyle) error {
	return style.ApplyToCell(c)
}

// ApplyStyle applies a reusable style to the row.
func (r *Row) ApplyStyle(style *CellStyle) error {
	return style.ApplyToRow(r)
}

func (w *Workbook) ensureRegistries() {
	if w.styles == nil {
		w.styles = make(map[string]*CellStyle)
	}
	if w.formats == nil {
		w.formats = make(map[string]*DataFormat)
	}
	if w.seeded {
		return
	}
	w.seeded = true

	nextImported := 1
	seenStyles := make(map[sheetir.CellStyle]struct{})
	for _, named := range w.ir.NamedCellStyles {
		if named == nil {
			continue
		}
		name := strings.TrimSpace(named.Name)
		if name == "" {
			continue
		}
		style := named.Style
		if style.NumFmtCode != "" {
			w.formats[registryKey(style.NumFmtCode)] = &DataFormat{code: style.NumFmtCode}
		}
		w.styles[registryKey(name)] = &CellStyle{
			wb:        w,
			name:      name,
			builtinID: cloneIntPtr(named.BuiltinID),
			style:     style,
		}
		seenStyles[style] = struct{}{}
	}
	seedStyle := func(style sheetir.CellStyle) {
		if isZeroCellStyle(style) {
			return
		}
		if _, ok := seenStyles[style]; ok {
			return
		}
		seenStyles[style] = struct{}{}
		if style.NumFmtCode != "" {
			w.formats[registryKey(style.NumFmtCode)] = &DataFormat{code: style.NumFmtCode}
		}
		name := fmt.Sprintf("ImportedStyle%d", nextImported)
		nextImported++
		w.styles[registryKey(name)] = &CellStyle{
			wb:    w,
			name:  name,
			style: style,
		}
	}

	if w.ir == nil {
		return
	}
	for _, sheet := range w.ir.Sheets {
		if sheet == nil {
			continue
		}
		for _, row := range sheet.Rows {
			if row == nil {
				continue
			}
			if row.Style != nil {
				seedStyle(*row.Style)
			}
			for _, cell := range row.Cells {
				if cell == nil {
					continue
				}
				seedStyle(cell.Style)
			}
		}
	}
}

func isZeroCellStyle(style sheetir.CellStyle) bool {
	return style == (sheetir.CellStyle{})
}

func (w *Workbook) syncNamedCellStyles() {
	if w == nil || w.ir == nil {
		return
	}
	w.ensureRegistries()
	styles := w.CellStyles()
	normalBuiltinID := 0
	normalStyle := sheetir.CellStyle{}
	if normal, ok := w.styles[registryKey("Normal")]; ok {
		normalStyle = normal.style
	}

	namedXFs := []sheetir.CellStyle{normalStyle}
	indexByStyle := map[sheetir.CellStyle]int{normalStyle: 0}
	named := []*sheetir.NamedCellStyle{{
		Name:         "Normal",
		StyleXFIndex: 0,
		BuiltinID:    &normalBuiltinID,
		Style:        normalStyle,
	}}

	for _, style := range styles {
		if strings.EqualFold(style.name, "Normal") {
			continue
		}
		xfIndex, ok := indexByStyle[style.style]
		if !ok {
			xfIndex = len(namedXFs)
			namedXFs = append(namedXFs, style.style)
			indexByStyle[style.style] = xfIndex
		}
		named = append(named, &sheetir.NamedCellStyle{
			Name:         style.name,
			StyleXFIndex: xfIndex,
			BuiltinID:    cloneIntPtr(style.builtinID),
			Style:        style.style,
		})
	}

	w.ir.NamedCellStyleXFs = namedXFs
	w.ir.NamedCellStyles = named
}

func (w *Workbook) nextStyleName() string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("Style%d", i)
		if _, ok := w.CellStyle(name); !ok {
			return name
		}
	}
}

func registryKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
