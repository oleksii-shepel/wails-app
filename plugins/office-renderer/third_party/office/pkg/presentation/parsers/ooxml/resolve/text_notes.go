package resolve

import (
	"strings"

	"office/pkg/ooxml/mathml"
	"office/pkg/ooxml/media"
	"office/pkg/presentation/layout/cascade"
	"office/pkg/presentation/parsers/ooxml/parts"
	"office/pkg/presentation/parsers/ooxml/project"
)

func orderedShapeTreeItems(tree parts.ShapeTree) []parts.ShapeTreeItem {
	if len(tree.Items) > 0 {
		return tree.Items
	}
	var items []parts.ShapeTreeItem
	for i := range tree.Shapes {
		items = append(items, parts.ShapeTreeItem{Kind: parts.ShapeTreeItemShape, Index: i})
	}
	for i := range tree.Pictures {
		items = append(items, parts.ShapeTreeItem{Kind: parts.ShapeTreeItemPicture, Index: i})
	}
	for i := range tree.GroupShapes {
		items = append(items, parts.ShapeTreeItem{Kind: parts.ShapeTreeItemGroup, Index: i})
	}
	for i := range tree.Tables {
		items = append(items, parts.ShapeTreeItem{Kind: parts.ShapeTreeItemTable, Index: i})
	}
	for i := range tree.Connectors {
		items = append(items, parts.ShapeTreeItem{Kind: parts.ShapeTreeItemConnector, Index: i})
	}
	for i := range tree.OleObjectFrames {
		items = append(items, parts.ShapeTreeItem{Kind: parts.ShapeTreeItemOleObject, Index: i})
	}
	return items
}

type textDefaults = cascade.TextDefaults

func resolveInheritedHeaderFooterPlaceholder(entry *parts.SlideEntry, master *parts.SlideMaster, masterEntry *parts.SlideMasterEntry, kind PlaceholderKind, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string, theme *Theme, rawTheme *parts.Theme) *Shape {
	if entry == nil || !cascade.HeaderFooterPlaceholderEnabled(master, kind) {
		return nil
	}
	var layoutShape, masterShape *parts.Shape
	if entry.Layout != nil {
		layoutShape = cascade.FindPlaceholderByKind(entry.Layout.CSld.SpTree.Shapes, kind)
	}
	if master != nil {
		masterShape = cascade.FindPlaceholderByKind(master.CSld.SpTree.Shapes, kind)
	}
	if layoutShape != nil {
		var masterMediaParts map[string]media.Part
		if masterEntry != nil {
			masterMediaParts = masterEntry.MediaParts
		}
		return resolveShape(layoutShape, nil, masterShape, master, layoutThemeColors, layoutThemeColors, masterThemeColors, theme, rawTheme, entry.LayoutMediaParts, nil, masterMediaParts, nil, nil)
	}
	if masterShape != nil {
		var mediaParts map[string]media.Part
		if masterEntry != nil {
			mediaParts = masterEntry.MediaParts
		}
		return resolveShape(masterShape, nil, nil, master, masterThemeColors, masterThemeColors, masterThemeColors, theme, rawTheme, mediaParts, nil, nil, nil, nil)
	}
	return nil
}

func placeholderIdxIsSentinel(ph *parts.PlaceholderInfo) bool {
	return cascade.PlaceholderIndexIsSentinel(ph)
}

func resolvePlaceholderKind(ph *parts.PlaceholderInfo) PlaceholderKind {
	return cascade.PlaceholderKind(ph)
}

func resolveParagraph(ap *parts.DMLParagraph, defaults textDefaults, themeColors map[string]string, theme *Theme, hlinkURLs map[string]string) *Paragraph {
	p, effectiveDefaults := cascade.ParagraphFromLocal(ap, defaults, themeColors, theme)
	if p == nil {
		return nil
	}
	defaults = effectiveDefaults
	for _, r := range ap.Runs {
		run := resolveRun(&r, defaults, themeColors, theme, hlinkURLs)
		if run != nil {
			p.Runs = append(p.Runs, run)
		}
	}
	for _, f := range ap.Fields {
		run := resolveField(&f, defaults, themeColors, theme)
		if run != nil {
			p.Runs = append(p.Runs, run)
		}
	}
	// If the paragraph was parsed via UnmarshalXML, use the ordered Inlines slice
	// (which already populates Runs/Fields for backward compat above, but we need
	// to replace the appended runs to preserve correct document order and add math).
	if len(ap.Inlines) > 0 {
		p.Runs = p.Runs[:0]
		for _, inline := range ap.Inlines {
			switch inline.Kind {
			case parts.DMLParaInlineRun:
				if run := resolveRun(inline.Run, defaults, themeColors, theme, hlinkURLs); run != nil {
					p.Runs = append(p.Runs, run)
				}
			case parts.DMLParaInlineField:
				if run := resolveField(inline.Field, defaults, themeColors, theme); run != nil {
					p.Runs = append(p.Runs, run)
				}
			case parts.DMLParaInlineMath:
				if inline.Math != nil {
					if mml := mathml.Convert(inline.Math.RawXML); mml != "" {
						p.Runs = append(p.Runs, &Run{MathML: mml})
					}
				}
			case parts.DMLParaInlineBreak:
				brRun := &Run{Text: "\n"}
				if inline.Break != nil && inline.Break.Properties != nil {
					applyRunProperties(brRun, inline.Break.Properties, defaults, themeColors, theme, hlinkURLs)
				}
				p.Runs = append(p.Runs, brRun)
			}
		}
	}
	return p
}

func ensureAutoSlideNumberField(shape *Shape, sp *parts.Shape, layoutShape, masterShape *parts.Shape, master *parts.SlideMaster, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string, theme *Theme) {
	if shape == nil || shape.Placeholder != PlaceholderSlideNumber || !shapeTextEmpty(shape) || shapeHasFieldKind(shape, "slidenum") {
		return
	}
	var sourcePara parts.DMLParagraph
	var sourceBody *parts.TextBody
	switch {
	case sp != nil && sp.TxBody != nil:
		sourceBody = sp.TxBody
	case layoutShape != nil && layoutShape.TxBody != nil:
		sourceBody = layoutShape.TxBody
	case masterShape != nil && masterShape.TxBody != nil:
		sourceBody = masterShape.TxBody
	}
	level := 0
	if sourceBody != nil && len(sourceBody.Paragraphs) > 0 {
		sourcePara = sourceBody.Paragraphs[0]
		if sourcePara.Properties != nil {
			level = sourcePara.Properties.Lvl
		}
	}
	defaults := cascade.EffectiveTextDefaults(sourceBody, layoutTextBody(layoutShape), layoutTextBody(masterShape), master, nil, PlaceholderSlideNumber, level, slideThemeColors, layoutThemeColors, masterThemeColors, theme)
	shape.Paragraphs = []*Paragraph{resolveParagraph(&parts.DMLParagraph{
		Properties: sourcePara.Properties,
		Fields: []parts.DMLField{{
			Type: "slidenum",
		}},
	}, defaults, slideThemeColors, theme, nil)}
}

func shapeTextEmpty(shape *Shape) bool {
	if shape == nil {
		return true
	}
	for _, para := range shape.Paragraphs {
		if para == nil {
			continue
		}
		for _, run := range para.Runs {
			if run == nil {
				continue
			}
			if run.FieldKind != "" || strings.TrimSpace(run.Text) != "" || run.MathML != "" {
				return false
			}
		}
	}
	return true
}

func shapeHasFieldKind(shape *Shape, kind string) bool {
	if shape == nil {
		return false
	}
	for _, para := range shape.Paragraphs {
		if para == nil {
			continue
		}
		for _, run := range para.Runs {
			if run != nil && strings.EqualFold(run.FieldKind, kind) {
				return true
			}
		}
	}
	return false
}

func resolveRun(r *parts.DMLRun, defaults textDefaults, themeColors map[string]string, theme *Theme, hlinkURLs map[string]string) *Run {
	if r.Text == "" {
		return nil
	}
	run := cascade.RunFromLocal(r.Text, r.Properties, defaults, themeColors, theme)
	if r.Properties != nil && r.Properties.HlinkClick != nil {
		run.HyperlinkURL, _ = resolveHlinkClick(r.Properties.HlinkClick, hlinkURLs)
	}
	return run
}

// applyRunProperties adapts the layout-owned local run cascade for synthetic
// runs such as line breaks while keeping hyperlink relationship lookup here.
func applyRunProperties(run *Run, props *parts.DMLRunPr, defaults textDefaults, themeColors map[string]string, theme *Theme, hlinkURLs map[string]string) {
	resolved := cascade.RunFromLocal(run.Text, props, defaults, themeColors, theme)
	*run = *resolved
	if props != nil && props.HlinkClick != nil {
		run.HyperlinkURL, _ = resolveHlinkClick(props.HlinkClick, hlinkURLs)
	}
}

// resolveField converts a DMLField (a:fld) to a Run with FieldKind set.
// The FieldKind is the normalized lowercase field type (e.g. "slidenum", "datetime").
func resolveField(f *parts.DMLField, defaults textDefaults, themeColors map[string]string, theme *Theme) *Run {
	if f.Type == "" {
		return nil
	}
	kind := strings.ToLower(f.Type)
	// Use a synthetic DMLRun to reuse run formatting resolution.
	synth := &parts.DMLRun{
		Text:       f.Text,
		Properties: f.Properties,
	}
	// Allow empty Text for fields — the renderer will substitute the value.
	var run *Run
	if f.Text != "" {
		run = resolveRun(synth, defaults, themeColors, theme, nil)
	}
	if run == nil {
		run = cascade.RunFromLocal("", f.Properties, defaults, themeColors, theme)
	}

	run.FieldKind = kind
	return run
}

// resolveNotesSlide converts a notes slide part to a resolved NotesSlide.
func resolveNotesSlide(notes *parts.NotesSlide, notesMaster *parts.NotesMaster, themeColors map[string]string, theme *Theme, rawTheme *parts.Theme, tableStyles *parts.TableStyleList) *NotesSlide {
	if notes == nil {
		return nil
	}
	if notes.ThemeOverride != nil {
		rawTheme = cascade.ApplyThemeOverride(rawTheme, notes.ThemeOverride)
		theme = cascade.ResolveTheme(rawTheme)
		themeColors = map[string]string{}
		if theme != nil {
			themeColors = cascade.BuildThemeColorMap(theme.ColorScheme, nil)
		}
	}
	resolved := &NotesSlide{}
	master := notesMasterAsSlideMaster(notesMaster)
	resolved.BgColor = cascade.BackgroundColor(notes.CSld.Bg, themeColors)
	if resolved.BgColor == "" && master != nil {
		resolved.BgColor = cascade.BackgroundColor(master.CSld.Bg, themeColors)
	}
	// Resolve shapes in the notes slide
	for _, sp := range notes.CSld.SpTree.Shapes {
		var masterShape *parts.Shape
		if master != nil {
			masterShape = cascade.FindPlaceholder(master.CSld.SpTree.Shapes, sp.NvSpPr.NvPr.Ph)
		}
		shape := resolveShape(&sp, nil, masterShape, master, themeColors, themeColors, themeColors, theme, rawTheme, nil, nil, nil, nil, nil)
		if shape != nil {
			resolved.Shapes = append(resolved.Shapes, shape)
		}
	}
	for _, pic := range notes.CSld.SpTree.Pictures {
		shape := resolvePicture(&pic, project.SlideResources{}, themeColors)
		if shape != nil {
			resolved.Shapes = append(resolved.Shapes, shape)
		}
	}
	for _, grp := range notes.CSld.SpTree.GroupShapes {
		shape := resolveGroupShape(&grp, nil, themeColors, nil, rawTheme, nil, project.SlideResources{}, tableStyles, nil)
		if shape != nil {
			resolved.Shapes = append(resolved.Shapes, shape)
		}
	}
	for _, cx := range notes.CSld.SpTree.Connectors {
		shape := resolveConnector(&cx, themeColors, rawTheme, nil)
		if shape != nil {
			resolved.Shapes = append(resolved.Shapes, shape)
		}
	}
	return resolved
}

func notesMasterAsSlideMaster(notesMaster *parts.NotesMaster) *parts.SlideMaster {
	if notesMaster == nil {
		return nil
	}
	return &parts.SlideMaster{
		CSld:     notesMaster.CSld,
		TxStyles: notesMaster.TxStyles,
	}
}

func normalizeRelID(id string) string {
	return id
}
