// Package edit provides a programmatic editing API for PowerPoint
// presentations, mirroring the surface of pkg/word/edit.
package edit

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/smartart"
	preslegacy "office/pkg/presentation/convert/legacytoooxml"
	presir "office/pkg/presentation/ir"
	presparts "office/pkg/presentation/parsers/ooxml/parts"
	presresolve "office/pkg/presentation/parsers/ooxml/resolve"
)

// Presentation wraps a presentation IR document for editing.
type Presentation struct {
	IR            *presir.Presentation
	sourcePackage *presparts.Package
}

// NewPresentation creates a new blank presentation with default 16:9 slide size.
func NewPresentation() *Presentation {
	return &Presentation{
		IR: &presir.Presentation{
			SlideWidthEMU:  12192000, // 16:9 width in EMU
			SlideHeightEMU: 6858000,  // 16:9 height in EMU
			Slides:         []*presir.Slide{},
		},
	}
}

// OpenIR wraps an existing presentation IR for editing.
func OpenIR(ir *presir.Presentation) *Presentation {
	if ir == nil {
		ir = &presir.Presentation{}
	}
	return &Presentation{IR: ir}
}

// SlideCount returns the number of slides.
func (p *Presentation) SlideCount() int {
	if p == nil || p.IR == nil {
		return 0
	}
	return len(p.IR.Slides)
}

// Slides returns all slides in order.
func (p *Presentation) Slides() []*Slide {
	if p == nil || p.IR == nil {
		return nil
	}
	out := make([]*Slide, len(p.IR.Slides))
	for i, s := range p.IR.Slides {
		out[i] = &Slide{pres: p, ir: s, index: i}
	}
	return out
}

// Slide returns the slide at the given index, or nil if out of range.
func (p *Presentation) Slide(index int) *Slide {
	if p == nil || p.IR == nil || index < 0 || index >= len(p.IR.Slides) {
		return nil
	}
	return &Slide{pres: p, ir: p.IR.Slides[index], index: index}
}

// AddSlide appends a new slide and returns it.
func (p *Presentation) AddSlide(layoutType string) *Slide {
	return p.InsertSlide(len(p.IR.Slides), layoutType)
}

// AddBlankSlide appends a new blank slide.
func (p *Presentation) AddBlankSlide() *Slide {
	return p.AddSlide("blank")
}

// InsertSlide inserts a new slide at the given index.
func (p *Presentation) InsertSlide(index int, layoutType string) *Slide {
	if p == nil || p.IR == nil {
		return nil
	}
	if index < 0 {
		index = 0
	}
	if index > len(p.IR.Slides) {
		index = len(p.IR.Slides)
	}
	slide := &presir.Slide{
		Index:      index,
		LayoutType: layoutType,
		Shapes:     []*presir.Shape{},
	}
	p.IR.Slides = append(p.IR.Slides[:index], append([]*presir.Slide{slide}, p.IR.Slides[index:]...)...)
	p.reindexSlides()
	return &Slide{pres: p, ir: slide, index: index}
}

// DuplicateSlide creates a deep copy of the slide at the given index and
// inserts it immediately after the original. Returns the new slide, or nil if
// the index is out of range.
func (p *Presentation) DuplicateSlide(index int) *Slide {
	if p == nil || p.IR == nil || index < 0 || index >= len(p.IR.Slides) {
		return nil
	}
	clone := cloneSlide(p.IR.Slides[index])
	insertAt := index + 1
	p.IR.Slides = append(p.IR.Slides[:insertAt], append([]*presir.Slide{clone}, p.IR.Slides[insertAt:]...)...)
	p.reindexSlides()
	return &Slide{pres: p, ir: clone, index: clone.Index}
}

// RemoveSlide removes the slide at the given index.
func (p *Presentation) RemoveSlide(index int) {
	if p == nil || p.IR == nil || index < 0 || index >= len(p.IR.Slides) {
		return
	}
	p.IR.Slides = append(p.IR.Slides[:index], p.IR.Slides[index+1:]...)
	p.reindexSlides()
}

// MoveSlide moves a slide from one index to another.
func (p *Presentation) MoveSlide(from, to int) {
	if p == nil || p.IR == nil || from < 0 || from >= len(p.IR.Slides) {
		return
	}
	if to < 0 {
		to = 0
	}
	if to >= len(p.IR.Slides) {
		to = len(p.IR.Slides) - 1
	}
	if from == to {
		return
	}
	slide := p.IR.Slides[from]
	p.IR.Slides = append(p.IR.Slides[:from], p.IR.Slides[from+1:]...)
	p.IR.Slides = append(p.IR.Slides[:to], append([]*presir.Slide{slide}, p.IR.Slides[to:]...)...)
	p.reindexSlides()
}

// SetSlideSize sets the slide dimensions in EMU.
func (p *Presentation) SetSlideSize(widthEMU, heightEMU int64) {
	if p == nil || p.IR == nil {
		return
	}
	p.IR.SlideWidthEMU = widthEMU
	p.IR.SlideHeightEMU = heightEMU
}

// SlideSize returns the slide dimensions in EMU.
func (p *Presentation) SlideSize() (widthEMU, heightEMU int64) {
	if p == nil || p.IR == nil {
		return 0, 0
	}
	return p.IR.SlideWidthEMU, p.IR.SlideHeightEMU
}

// ToOOXML converts the presentation IR to an OOXML parts package.
func (p *Presentation) ToOOXML() (*presparts.Package, error) {
	if p == nil || p.IR == nil {
		return nil, fmt.Errorf("edit: nil presentation")
	}
	pkg, err := preslegacy.Convert(p.IR)
	if err != nil {
		return nil, err
	}
	if p.sourcePackage != nil {
		pkg.PreservedParts = append([]presparts.PreservedPart(nil), p.sourcePackage.PreservedParts...)
		if p.sourcePackage.MainPartName != "" {
			pkg.MainPartName = p.sourcePackage.MainPartName
		}
	}
	return pkg, nil
}

// SaveAsPPTX writes the presentation as a .pptx file to w.
func (p *Presentation) SaveAsPPTX(w io.Writer) error {
	pkg, err := p.ToOOXML()
	if err != nil {
		return err
	}
	return presparts.WritePackage(pkg, w)
}

// LoadFromPPTX reads a .pptx file and returns an editable Presentation.
func LoadFromPPTX(r io.Reader) (*Presentation, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("edit: read pptx: %w", err)
	}
	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		return nil, fmt.Errorf("edit: open pptx: %w", err)
	}
	partsPkg, err := presparts.LoadPackage(opcPkg)
	if err != nil {
		return nil, fmt.Errorf("edit: load parts: %w", err)
	}
	resolved, err := presresolve.ResolvePackage(partsPkg)
	if err != nil {
		return nil, fmt.Errorf("edit: resolve: %w", err)
	}
	attachEditableSmartArt(resolved, partsPkg)
	edited := OpenIR(resolved)
	edited.sourcePackage = partsPkg
	return edited, nil
}

// LoadFromPPTXBytes reads .pptx bytes and returns an editable Presentation.
func LoadFromPPTXBytes(data []byte) (*Presentation, error) {
	return LoadFromPPTX(bytes.NewReader(data))
}

func attachEditableSmartArt(resolved *presir.Presentation, partsPkg *presparts.Package) {
	if resolved == nil || partsPkg == nil {
		return
	}
	for i, entry := range partsPkg.Slides {
		if i >= len(resolved.Slides) || entry == nil {
			continue
		}
		frames := smartArtFramesByRel(entry)
		for _, part := range entry.SmartArtParts {
			if part == nil || len(part.DataModelXML) == 0 {
				continue
			}
			diagram, err := smartart.ParseDiagram(part.DataModelXML)
			if err != nil {
				continue
			}
			art := smartArtIRFromDiagram(diagram)
			if frame, ok := frames[part.RelID]; ok && frame != nil && frame.Xfrm != nil {
				art.X = frame.Xfrm.Off.X
				art.Y = frame.Xfrm.Off.Y
				art.W = frame.Xfrm.Ext.CX
				art.H = frame.Xfrm.Ext.CY
			}
			resolved.Slides[i].SmartArt = append(resolved.Slides[i].SmartArt, art)
		}
	}
}

func smartArtFramesByRel(entry *presparts.SlideEntry) map[string]*presparts.SmartArtFrame {
	out := map[string]*presparts.SmartArtFrame{}
	if entry == nil || entry.Slide == nil {
		return out
	}
	var visitFrames func([]presparts.SmartArtFrame)
	visitFrames = func(frames []presparts.SmartArtFrame) {
		for i := range frames {
			frame := &frames[i]
			if relID := frame.RelIDs["dm"]; relID != "" {
				out[relID] = frame
			}
		}
	}
	var visitGroups func([]presparts.GroupShape)
	visitGroups = func(groups []presparts.GroupShape) {
		for i := range groups {
			visitFrames(groups[i].SmartArtFrames)
			visitGroups(groups[i].GroupShapes)
		}
	}
	visitFrames(entry.Slide.CSld.SpTree.SmartArtFrames)
	visitGroups(entry.Slide.CSld.SpTree.GroupShapes)
	return out
}

func smartArtIRFromDiagram(diagram *smartart.Diagram) *presir.SmartArt {
	art := &presir.SmartArt{Type: "hierarchy"}
	if diagram == nil {
		return art
	}
	if diagram.Layout != nil && diagram.Layout.Type != "" {
		art.Type = diagram.Layout.Type
	}
	if diagram.Style != nil {
		if sf, ok := diagram.Style.Fill.(drawing.SolidFill); ok {
			art.FillColor = sf.Color
		} else if sf, ok := diagram.Style.Fill.(*drawing.SolidFill); ok && sf != nil {
			art.FillColor = sf.Color
		}
		if diagram.Style.Line != nil {
			art.LineColor = diagram.Style.Line.Color
		}
		art.TextColor = diagram.Style.TextColor
	}
	art.Nodes = make([]*presir.SmartArtNode, 0, len(diagram.Nodes))
	for _, node := range diagram.Nodes {
		if node == nil {
			continue
		}
		if art.LayoutID == "" {
			art.LayoutID = node.PropertySet.LayoutTypeID
		}
		if art.QuickStyleID == "" {
			art.QuickStyleID = node.PropertySet.QuickStyleTypeID
		}
		if art.ColorID == "" {
			art.ColorID = node.PropertySet.ColorTypeID
		}
		irNode := &presir.SmartArtNode{
			ID:        node.ID,
			Type:      node.Type,
			Text:      node.Text,
			TextColor: node.TextColor,
		}
		if sf, ok := node.Fill.(drawing.SolidFill); ok {
			irNode.FillColor = sf.Color
		} else if sf, ok := node.Fill.(*drawing.SolidFill); ok && sf != nil {
			irNode.FillColor = sf.Color
		}
		for _, child := range node.Children {
			if child != nil {
				irNode.Children = append(irNode.Children, child.ID)
			}
		}
		art.Nodes = append(art.Nodes, irNode)
	}
	for _, cxn := range diagram.Connections {
		art.Connections = append(art.Connections, presir.SmartArtConnection{SrcID: cxn.SrcID, DestID: cxn.DestID})
	}
	return art
}

// AddEmbeddedFont adds a regular embedded font face to the presentation.
// The data should be a raw font file (e.g. TTF/OTF/EOT). The content type
// defaults to application/x-fontdata when empty.
func (p *Presentation) AddEmbeddedFont(family string, data []byte) {
	p.AddEmbeddedFontFace(family, 400, "normal", "", data)
}

// AddEmbeddedFontFace adds an embedded font face with the given weight, style,
// and optional content type. Weight follows CSS conventions (400 = regular,
// 700 = bold). Style should be "normal" or "italic".
func (p *Presentation) AddEmbeddedFontFace(family string, weight int, style, contentType string, data []byte) {
	if p == nil || p.IR == nil {
		return
	}
	family = strings.TrimSpace(family)
	if family == "" || len(data) == 0 {
		return
	}
	var font *presir.EmbeddedFont
	for _, f := range p.IR.EmbeddedFonts {
		if f != nil && f.Family == family {
			font = f
			break
		}
	}
	if font == nil {
		font = &presir.EmbeddedFont{Family: family}
		p.IR.EmbeddedFonts = append(p.IR.EmbeddedFonts, font)
	}
	if contentType == "" {
		contentType = "application/x-fontdata"
	}
	font.Faces = append(font.Faces, &presir.EmbeddedFontFace{
		Family:      family,
		Weight:      weight,
		Style:       style,
		ContentType: contentType,
		Data:        append([]byte(nil), data...),
	})
}

// AddSlideMaster creates a new slide master and returns it for editing.
func (p *Presentation) AddSlideMaster() *SlideMaster {
	if p == nil || p.IR == nil {
		return nil
	}
	master := &presir.SlideMaster{
		Name:    "Slide Master",
		Shapes:  []*presir.Shape{},
		Layouts: []*presir.SlideLayout{},
	}
	p.IR.SlideMasters = append(p.IR.SlideMasters, master)
	return &SlideMaster{pres: p, ir: master}
}

// SlideMasters returns all slide masters in the presentation.
func (p *Presentation) SlideMasters() []*SlideMaster {
	if p == nil || p.IR == nil {
		return nil
	}
	out := make([]*SlideMaster, len(p.IR.SlideMasters))
	for i, m := range p.IR.SlideMasters {
		out[i] = &SlideMaster{pres: p, ir: m}
	}
	return out
}

// EmbeddedFontNames returns the families of all embedded fonts.
func (p *Presentation) EmbeddedFontNames() []string {
	if p == nil || p.IR == nil {
		return nil
	}
	names := make([]string, 0, len(p.IR.EmbeddedFonts))
	for _, f := range p.IR.EmbeddedFonts {
		if f == nil {
			continue
		}
		names = append(names, f.Family)
	}
	return names
}

// Theme returns the presentation-level theme for editing, or nil if none.
func (p *Presentation) Theme() *Theme {
	if p == nil || p.IR == nil || p.IR.Theme == nil {
		return nil
	}
	return &Theme{ir: p.IR.Theme}
}

// SetTheme sets the presentation-level theme.
func (p *Presentation) SetTheme(theme *Theme) {
	if p == nil || p.IR == nil {
		return
	}
	if theme == nil {
		p.IR.Theme = nil
		return
	}
	p.IR.Theme = theme.ir
}

// AddTheme creates a new empty theme and attaches it to the presentation.
func (p *Presentation) AddTheme() *Theme {
	if p == nil || p.IR == nil {
		return nil
	}
	p.IR.Theme = &presir.Theme{
		ColorScheme: make(map[string]string),
	}
	return &Theme{ir: p.IR.Theme}
}

// Theme wraps a presentation theme for editing.
type Theme struct {
	ir *presir.Theme
}

// Name returns the theme name.
func (t *Theme) Name() string {
	if t == nil || t.ir == nil {
		return ""
	}
	return t.ir.Name
}

// SetName sets the theme name.
func (t *Theme) SetName(name string) {
	if t == nil || t.ir == nil {
		return
	}
	t.ir.Name = name
}

// Color returns the sRGB hex color for the given scheme slot (e.g. "dk1",
// "lt1", "accent1", "hlink"), or "" if not set.
func (t *Theme) Color(slot string) string {
	if t == nil || t.ir == nil || t.ir.ColorScheme == nil {
		return ""
	}
	return t.ir.ColorScheme[slot]
}

// SetColor sets an sRGB hex color for the given scheme slot.
func (t *Theme) SetColor(slot, hex string) {
	if t == nil || t.ir == nil {
		return
	}
	if t.ir.ColorScheme == nil {
		t.ir.ColorScheme = make(map[string]string)
	}
	t.ir.ColorScheme[slot] = hex
}

// MajorFont returns the major (heading) Latin font typeface.
func (t *Theme) MajorFont() string {
	if t == nil || t.ir == nil {
		return ""
	}
	return t.ir.MajorFont
}

// SetMajorFont sets the major Latin font typeface.
func (t *Theme) SetMajorFont(family string) {
	if t == nil || t.ir == nil {
		return
	}
	t.ir.MajorFont = family
}

// MinorFont returns the minor (body) Latin font typeface.
func (t *Theme) MinorFont() string {
	if t == nil || t.ir == nil {
		return ""
	}
	return t.ir.MinorFont
}

// SetMinorFont sets the minor Latin font typeface.
func (t *Theme) SetMinorFont(family string) {
	if t == nil || t.ir == nil {
		return
	}
	t.ir.MinorFont = family
}

func (p *Presentation) reindexSlides() {
	for i, s := range p.IR.Slides {
		s.Index = i
	}
}
