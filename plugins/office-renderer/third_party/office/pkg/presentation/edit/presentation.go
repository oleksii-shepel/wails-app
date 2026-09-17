// Package edit provides a programmatic editing API for PowerPoint
// presentations, mirroring the surface of pkg/word/edit.
package edit

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/media"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/smartart"
	preslegacy "office/pkg/presentation/convert/legacytoooxml"
	presir "office/pkg/presentation/ir"
	"office/pkg/presentation/layout/cascade"
	presparts "office/pkg/presentation/parsers/ooxml/parts"
	presproject "office/pkg/presentation/parsers/ooxml/project"
	presresolve "office/pkg/presentation/parsers/ooxml/resolve"
	pressax "office/pkg/presentation/parsers/ooxml/sax"
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

	// The forward-only SAX pipeline is now the first semantic reader in the
	// production PPTX load path.  During the ownership-transfer phase the
	// mature package composer still supplies features that have not moved to
	// SAX yet (charts, SmartArt, media, animations, notes, comments, etc.), but
	// every production load must first prove that the authored slide stream can
	// be consumed and materialized by the new parser.  This prevents the SAX
	// implementation from remaining a test-only parallel parser while keeping
	// unsupported feature families on the established resolver until their
	// migration is complete.
	saxPresentation, err := pressax.BuildContextualPresentationBytes(data, pressax.Options{
		IncludeHiddenSlides: true,
		IncludeNotes:        true,
		IncludeComments:     true,
	})
	if err != nil {
		return nil, fmt.Errorf("edit: sax parse pptx: %w", err)
	}

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		return nil, fmt.Errorf("edit: open pptx: %w", err)
	}
	partsPkg, err := presparts.LoadPackage(opcPkg)
	if err != nil {
		return nil, fmt.Errorf("edit: load parts: %w", err)
	}

	// BIG16: SAX is the production presentation parser for PPTX. The typed parts
	// package remains a secondary-part/preservation index only; it no longer
	// triggers a second visual composition of the presentation.
	packageProjection := presproject.NewPresentation(partsPkg)
	if packageProjection != nil {
		saxPresentation.EmbeddedFonts = packageProjection.EmbeddedFonts
		saxPresentation.Diagnostics = append(saxPresentation.Diagnostics, packageProjection.Diagnostics...)
	}
	presproject.ApplySections(partsPkg.Presentation, saxPresentation)
	applySAXPackageVisualSemantics(saxPresentation, partsPkg)
	applySAXSmartArtSemantics(saxPresentation, partsPkg)

	// Timing is a package/slide semantic, not visual composition. Project it
	// directly onto the SAX-authored tree before considering the remaining
	// compatibility island.
	for i, entry := range partsPkg.Slides {
		if entry != nil && i < len(saxPresentation.Slides) {
			presproject.ApplyAnimationMetadata(entry.Slide, saxPresentation.Slides[i])
		}
	}

	// BIG22: the compatibility audit is gone. The production PPTX path is now
	// structurally SAX + focused package projections only; unsupported semantics
	// must be represented by those projections rather than a fallback capability gate.
	attachEditableSmartArt(saxPresentation, partsPkg)

	edited := OpenIR(saxPresentation)
	edited.sourcePackage = partsPkg
	return edited, nil

}

// applySAXPackageVisualSemantics projects package-scoped visual facts that need
// theme/layout/media context onto the SAX-authored tree. It deliberately works
// by stable shape id: source ordering and shape ownership remain SAX concerns.
// applySAXSmartArtSemantics expands package SmartArt parts directly onto the
// SAX-authored slide tree. This deliberately invokes the focused SmartArt
// semantic resolver, not whole-package resolver: unrelated slide visuals never cross
// back through the compatibility presentation resolver.
func applySAXSmartArtSemantics(p *presir.Presentation, pkg *presparts.Package) {
	if p == nil || pkg == nil {
		return
	}
	rawTheme := firstPackageTheme(pkg)
	for i, entry := range pkg.Slides {
		if entry == nil || entry.Slide == nil || i >= len(p.Slides) || p.Slides[i] == nil || len(entry.SmartArtParts) == 0 {
			continue
		}
		master, _ := packageMasterForSlide(pkg, entry)
		ctx := cascade.ResolveThemeContext(rawTheme, master, entry.Layout, entry.Slide)
		frames := smartArtFramesByRel(entry)
		// SAX materializes the hosting graphicFrame as a placeholder. Once the
		// diagram has been expanded, that placeholder must not paint as a second
		// copy of the diagram.
		frameIDs := map[int]bool{}
		for _, frame := range frames {
			if frame != nil {
				frameIDs[frame.ID] = true
			}
		}
		p.Slides[i].Shapes = removeSmartArtHostFrames(p.Slides[i].Shapes, frameIDs)
		for _, part := range entry.SmartArtParts {
			if part == nil {
				continue
			}
			shapes, diags := presresolve.ResolveSmartArtPart(part, frames[part.RelID], ctx.SlideColors, ctx.RawTheme, ctx.Theme)
			p.Slides[i].Shapes = append(p.Slides[i].Shapes, shapes...)
			p.Slides[i].Diagnostics = append(p.Slides[i].Diagnostics, diags...)
		}
	}
}

func removeSmartArtHostFrames(shapes []*presir.Shape, frameIDs map[int]bool) []*presir.Shape {
	out := shapes[:0]
	for _, sh := range shapes {
		if sh == nil {
			continue
		}
		if len(sh.Children) > 0 {
			sh.Children = removeSmartArtHostFrames(sh.Children, frameIDs)
		}
		if sh.Kind == presir.ShapeSmartArt && frameIDs[sh.ID] {
			continue
		}
		out = append(out, sh)
	}
	return out
}

func applySAXPackageVisualSemantics(p *presir.Presentation, pkg *presparts.Package) {
	if p == nil || pkg == nil {
		return
	}
	rawTheme := firstPackageTheme(pkg)
	for i, entry := range pkg.Slides {
		if entry == nil || entry.Slide == nil || i >= len(p.Slides) || p.Slides[i] == nil {
			continue
		}
		master, masterEntry := packageMasterForSlide(pkg, entry)
		ctx := cascade.ResolveThemeContext(rawTheme, master, entry.Layout, entry.Slide)
		applyEffectiveBackground(p.Slides[i], entry, master, masterEntry, ctx)
		applySAXTableStyleSemantics(p.Slides[i].Shapes, &entry.Slide.CSld.SpTree, pkg.TableStyles, ctx.SlideColors, ctx.Theme, ctx.RawTheme)
		facts := map[int]packageVisualFact{}
		collectPackageVisualFacts(&entry.Slide.CSld.SpTree, ctx.SlideColors, facts)
		applyPackageVisualFacts(p.Slides[i].Shapes, facts)
	}
}

type packageVisualFact struct {
	effects      drawing.EffectList
	overlay      *presir.EffectOverlay
	imageEffects []drawing.Effect
}

func collectPackageVisualFacts(tree *presparts.ShapeTree, colors map[string]string, out map[int]packageVisualFact) {
	if tree == nil {
		return
	}
	for i := range tree.Shapes {
		sp := &tree.Shapes[i]
		el := sp.SpPr.DrawingEffectList(colors)
		out[sp.NvSpPr.CNvPr.ID] = packageVisualFact{effects: el, overlay: effectOverlayFromList(el)}
	}
	for i := range tree.Pictures {
		pic := &tree.Pictures[i]
		f := packageVisualFact{effects: pic.SpPr.DrawingEffectList(colors)}
		f.overlay = effectOverlayFromList(f.effects)
		if pic.BlipFill.Blip != nil {
			f.imageEffects = pic.BlipFill.Blip.DrawingEffects(colors)
		}
		out[pic.NvPicPr.CNvPr.ID] = f
	}
	for i := range tree.GroupShapes {
		collectPackageGroupVisualFacts(&tree.GroupShapes[i], colors, out)
	}
}

func collectPackageGroupVisualFacts(g *presparts.GroupShape, colors map[string]string, out map[int]packageVisualFact) {
	if g == nil {
		return
	}
	for i := range g.Shapes {
		sp := &g.Shapes[i]
		el := sp.SpPr.DrawingEffectList(colors)
		out[sp.NvSpPr.CNvPr.ID] = packageVisualFact{effects: el, overlay: effectOverlayFromList(el)}
	}
	for i := range g.Pictures {
		pic := &g.Pictures[i]
		f := packageVisualFact{effects: pic.SpPr.DrawingEffectList(colors)}
		f.overlay = effectOverlayFromList(f.effects)
		if pic.BlipFill.Blip != nil {
			f.imageEffects = pic.BlipFill.Blip.DrawingEffects(colors)
		}
		out[pic.NvPicPr.CNvPr.ID] = f
	}
	for i := range g.GroupShapes {
		collectPackageGroupVisualFacts(&g.GroupShapes[i], colors, out)
	}
}

func effectOverlayFromList(el drawing.EffectList) *presir.EffectOverlay {
	for _, e := range el.Effects {
		switch v := e.(type) {
		case drawing.FillOverlay:
			return &presir.EffectOverlay{Blend: v.BlendMode, Fill: v.Fill}
		case *drawing.FillOverlay:
			if v != nil {
				return &presir.EffectOverlay{Blend: v.BlendMode, Fill: v.Fill}
			}
		}
	}
	return nil
}

func applyPackageVisualFacts(shapes []*presir.Shape, facts map[int]packageVisualFact) {
	for _, sh := range shapes {
		if sh == nil {
			continue
		}
		if f, ok := facts[sh.ID]; ok {
			if len(f.effects.Effects) != 0 {
				sh.Effects = f.effects
			}
			if f.overlay != nil {
				sh.EffectOverlay = f.overlay
			}
			if len(f.imageEffects) != 0 {
				sh.ImageEffects = append([]drawing.Effect(nil), f.imageEffects...)
			}
		}
		applyPackageVisualFacts(sh.Children, facts)
	}
}

func firstPackageTheme(pkg *presparts.Package) *presparts.Theme {
	for _, t := range pkg.Themes {
		if t != nil {
			return t
		}
	}
	return nil
}

func packageMasterForSlide(pkg *presparts.Package, entry *presparts.SlideEntry) (*presparts.SlideMaster, *presparts.SlideMasterEntry) {
	if entry != nil && entry.MasterPartName != "" {
		for _, me := range pkg.SlideMasters {
			if me != nil && me.PartName == entry.MasterPartName {
				return me.Master, me
			}
		}
	}
	for _, me := range pkg.SlideMasters {
		if me != nil && me.Master != nil {
			return me.Master, me
		}
	}
	return nil, nil
}

func applyEffectiveBackground(dst *presir.Slide, entry *presparts.SlideEntry, master *presparts.SlideMaster, masterEntry *presparts.SlideMasterEntry, ctx cascade.ThemeContext) {
	if dst == nil || entry == nil {
		return
	}
	type candidate struct {
		bg     *presparts.SlideBg
		colors map[string]string
		media  map[string]media.Part
	}
	var masterMedia map[string]media.Part
	if masterEntry != nil {
		masterMedia = masterEntry.MediaParts
	}
	candidates := []candidate{{entry.Slide.CSld.Bg, ctx.SlideColors, entry.MediaParts}}
	if entry.Layout != nil {
		candidates = append(candidates, candidate{entry.Layout.CSld.Bg, ctx.LayoutColors, entry.LayoutMediaParts})
	}
	if master != nil {
		candidates = append(candidates, candidate{master.CSld.Bg, ctx.MasterColors, masterMedia})
	}
	for _, c := range candidates {
		if dst.BgColor == "" {
			dst.BgColor = cascade.BackgroundColor(c.bg, c.colors)
		}
		if dst.BgFill == nil {
			dst.BgFill = cascade.BackgroundFill(c.bg, c.colors, ctx.RawTheme, c.media)
			if dst.BgFill != nil {
				dst.BgPictureTile = packageBackgroundTile(c.bg, ctx.RawTheme)
			}
		}
	}
}

func packageBackgroundTile(bg *presparts.SlideBg, theme *presparts.Theme) *presir.PictureTile {
	if bg == nil {
		return nil
	}
	tile := bg.BgPrBlipFill
	if tile != nil && tile.Tile != nil {
		t := tile.Tile
		return &presir.PictureTile{Tx: int64(t.Tx), Ty: int64(t.Ty), Sx: int(t.Sx), Sy: int(t.Sy), Flip: t.Flip, Alignment: t.Algn}
	}
	if bg.BgRef != nil && theme != nil {
		idx := bg.BgRef.Idx - 1001
		if idx >= 0 && idx < len(theme.Elements.FmtScheme.BGFillStyleList) {
			bf := theme.Elements.FmtScheme.BGFillStyleList[idx].BlipFill
			if bf != nil && bf.Tile != nil {
				t := bf.Tile
				return &presir.PictureTile{Tx: int64(t.Tx), Ty: int64(t.Ty), Sx: int(t.Sx), Sy: int(t.Sy), Flip: t.Flip, Alignment: t.Algn}
			}
		}
	}
	return nil
}

// verifySAXSlideIdentity is intentionally a narrow production parity gate.
// It verifies facts that the SAX parser already owns authoritatively without
// pretending that feature families still owned by whole-package composer have moved.
// The gate grows as ownership transfers from the old resolver.
func verifySAXSlideIdentity(saxSlides []*presir.Slide, resolved *presir.Presentation) error {
	if resolved == nil {
		return fmt.Errorf("resolved presentation is nil")
	}
	if len(saxSlides) != len(resolved.Slides) {
		return fmt.Errorf("slide count differs: sax=%d compose=%d", len(saxSlides), len(resolved.Slides))
	}
	for i, slide := range saxSlides {
		if slide == nil {
			return fmt.Errorf("sax slide %d is nil", i)
		}
		if slide.Index != i {
			return fmt.Errorf("sax slide %d has index %d", i, slide.Index)
		}
		if resolved.Slides[i] == nil {
			return fmt.Errorf("composed slide %d is nil", i)
		}
	}
	return nil
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

// applySAXTableStyleSemantics resolves DrawingML table-style inheritance onto
// the SAX-authored table cells. Table identity/structure/direct tcPr paint stay
// SAX-owned; this projection supplies only the package-level style cascade.
func applySAXTableStyleSemantics(dst []*presir.Shape, tree *presparts.ShapeTree, styles *presparts.TableStyleList, colors map[string]string, theme *presir.Theme, rawTheme *presparts.Theme) {
	if tree == nil {
		return
	}
	frames := map[int]*presparts.TableFrame{}
	collectPackageTableFrames(tree, frames)
	applyTableStyleFrames(dst, frames, styles, colors, theme, rawTheme)
}

func collectPackageTableFrames(tree *presparts.ShapeTree, out map[int]*presparts.TableFrame) {
	for i := range tree.Tables {
		f := &tree.Tables[i]
		out[f.ID] = f
	}
	for i := range tree.GroupShapes {
		collectPackageGroupTableFrames(&tree.GroupShapes[i], out)
	}
}

func collectPackageGroupTableFrames(g *presparts.GroupShape, out map[int]*presparts.TableFrame) {
	if g == nil {
		return
	}
	for i := range g.Tables {
		f := &g.Tables[i]
		out[f.ID] = f
	}
	for i := range g.GroupShapes {
		collectPackageGroupTableFrames(&g.GroupShapes[i], out)
	}
}

func applyTableStyleFrames(shapes []*presir.Shape, frames map[int]*presparts.TableFrame, styles *presparts.TableStyleList, colors map[string]string, theme *presir.Theme, rawTheme *presparts.Theme) {
	for _, sh := range shapes {
		if sh == nil {
			continue
		}
		if sh.Kind == presir.ShapeTable && sh.Table != nil {
			if f := frames[sh.ID]; f != nil && f.Table != nil {
				applyTableStyleFrame(sh.Table, f.Table, styles, colors, theme, rawTheme)
			}
		}
		applyTableStyleFrames(sh.Children, frames, styles, colors, theme, rawTheme)
	}
}

func applyTableStyleFrame(dst *presir.Table, src *presparts.PresentationTable, styles *presparts.TableStyleList, colors map[string]string, theme *presir.Theme, rawTheme *presparts.Theme) {
	style := cascade.EffectiveTableStyle(src, styles)
	if style == nil {
		return
	}
	totalRows, totalCols := len(src.Rows), cascade.TableColumnCount(src)
	for ri := range src.Rows {
		if ri >= len(dst.Rows) || dst.Rows[ri] == nil {
			continue
		}
		col := 0
		for ci := range src.Rows[ri].Cells {
			cell := &src.Rows[ri].Cells[ci]
			span := cell.GridSpan
			if span <= 0 || cell.HMerge || cell.VMerge {
				span = 1
			}
			if ci < len(dst.Rows[ri].Cells) && dst.Rows[ri].Cells[ci] != nil {
				d := dst.Rows[ri].Cells[ci]
				st := cascade.ResolveTableCellStyle(style, src.Props, ri, col, span, totalRows, totalCols, colors, theme, rawTheme)
				// Style is the inherited base; direct tcPr remains more specific.
				d.Fill, d.FillColor, d.Effects = st.Fill, st.FillColor, st.Effects
				d.BorderLeft, d.BorderRight, d.BorderTop, d.BorderBottom = st.BorderLeft, st.BorderRight, st.BorderTop, st.BorderBottom
				if cell.Props != nil {
					if fill := cascade.TableCellFill(cell.Props.SolidFill, cell.Props.SchemeFill, cell.Props.GradFill, cell.Props.PattFill, cell.Props.BlipFill, cell.Props.NoFill, colors); fill != nil {
						d.Fill = fill
						d.FillColor = tableProjectionSolidColor(fill)
					}
					if cell.Props.LnL != nil {
						d.BorderLeft = cascade.TableLineStyle(cell.Props.LnL, colors)
					}
					if cell.Props.LnR != nil {
						d.BorderRight = cascade.TableLineStyle(cell.Props.LnR, colors)
					}
					if cell.Props.LnT != nil {
						d.BorderTop = cascade.TableLineStyle(cell.Props.LnT, colors)
					}
					if cell.Props.LnB != nil {
						d.BorderBottom = cascade.TableLineStyle(cell.Props.LnB, colors)
					}
				}
				applyTableTextStyleDefaults(d.Paragraphs, st.Text)
			}
			col += span
		}
	}
}

func tableProjectionSolidColor(fill drawing.Fill) string {
	switch v := fill.(type) {
	case drawing.SolidFill:
		return v.Color
	case *drawing.SolidFill:
		if v != nil {
			return v.Color
		}
	}
	return ""
}

func applyTableTextStyleDefaults(paragraphs []*presir.Paragraph, d cascade.TableTextDefaults) {
	for _, p := range paragraphs {
		if p == nil {
			continue
		}
		for _, r := range p.Runs {
			if r == nil {
				continue
			}
			if d.Bold != nil && *d.Bold {
				r.Bold = true
			}
			if d.Italic != nil && *d.Italic {
				r.Italic = true
			}
			if r.FontFamily == "" && d.FontFamily != "" {
				r.FontFamily = d.FontFamily
			}
			if r.Color == "" && d.Color != "" {
				r.Color = d.Color
			}
		}
	}
}
