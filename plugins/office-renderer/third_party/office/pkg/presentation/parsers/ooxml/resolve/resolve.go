package resolve

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"sort"
	"strconv"
	"strings"

	"office/pkg/diag"
	officeartir "office/pkg/officeart/ir"
	officeartdml "office/pkg/officeart/ooxml/drawingml"
	"office/pkg/ooxml/chart"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/mathml"
	"office/pkg/ooxml/media"
	"office/pkg/ooxml/smartart"
	smartartrender "office/pkg/ooxml/smartart/render"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// ResolvePackage converts a loaded parts.Package into a resolved Presentation IR.
func ResolvePackage(pkg *parts.Package) (*Presentation, error) {
	if pkg == nil {
		return nil, nil
	}

	pres := &Presentation{Diagnostics: pkg.Diagnostics}

	if pkg.Presentation != nil && pkg.Presentation.SlideSize != nil {
		pres.SlideWidthEMU = pkg.Presentation.SlideSize.CX
		pres.SlideHeightEMU = pkg.Presentation.SlideSize.CY
	}

	rawTheme := firstThemePart(pkg)
	pres.Theme = resolveTheme(rawTheme)
	pres.EmbeddedFonts = resolveEmbeddedFonts(pkg.EmbeddedFonts)

	masterIDByPart := buildMasterIDByPart(pkg)

	for i, entry := range pkg.Slides {
		master, masterEntry := slideMasterForEntry(pkg, entry)
		layoutRawTheme := applyThemeOverride(rawTheme, themeOverrideForLayout(entry.Layout))
		slideRawTheme := applyThemeOverride(layoutRawTheme, themeOverrideForSlide(entry.Slide))
		slideTheme := resolveTheme(slideRawTheme)
		baseThemeColors := map[string]string{}
		if slideTheme != nil {
			baseThemeColors = buildThemeColorMap(slideTheme.ColorScheme, nil)
		}
		masterThemeColors := baseThemeColors
		if master != nil {
			masterThemeColors = buildThemeColorMap(baseThemeColors, master.ClrMap)
		}
		layoutThemeColors := applyColorMapOverride(baseThemeColors, masterThemeColors, colorMapOverrideForLayout(entry.Layout))
		slideThemeColors := applyColorMapOverride(baseThemeColors, layoutThemeColors, colorMapOverrideForSlide(entry.Slide))

		slide := resolveSlide(i, entry, master, masterEntry, pkg.NotesMaster, slideThemeColors, layoutThemeColors, masterThemeColors, slideTheme, slideRawTheme, pkg.TableStyles)
		slide.Comments = resolveSlideComments(entry.Comments, pkg.CommentAuthors)
		if id, ok := masterIDByPart[entry.MasterPartName]; ok {
			slide.MasterID = id
		}
		if entry.Layout != nil {
			slide.LayoutType = entry.Layout.Type
		}
		pres.Slides = append(pres.Slides, slide)
	}

	// Resolve slide sections when present.
	if pkg.Presentation != nil {
		resolveSections(pkg.Presentation, pres)
	}

	return pres, nil
}

// resolveSections maps section metadata onto the resolved presentation and slides.
func resolveSections(presParts *parts.Presentation, pres *Presentation) {
	if presParts == nil || len(presParts.Sections) == 0 {
		return
	}

	// Build numeric slide id → slide index map from presentation-level sldIdLst.
	slideIDToIndex := make(map[int]int, len(presParts.SlideIDs))
	for i, idRef := range presParts.SlideIDs {
		if idRef.ID != 0 {
			slideIDToIndex[idRef.ID] = i
		}
	}

	for _, sec := range presParts.Sections {
		if sec.Name == "" {
			continue
		}
		pres.Sections = append(pres.Sections, sec.Name)
		for _, sldID := range sec.SlideIDs {
			if idx, ok := slideIDToIndex[sldID.ID]; ok && idx >= 0 && idx < len(pres.Slides) {
				pres.Slides[idx].SectionName = sec.Name
			}
		}
	}
}

func resolveEmbeddedFonts(fonts []*parts.EmbeddedFontData) []*EmbeddedFont {
	out := make([]*EmbeddedFont, 0, len(fonts))
	for _, font := range fonts {
		if font == nil || strings.TrimSpace(font.Typeface) == "" {
			continue
		}
		resolved := &EmbeddedFont{Family: strings.TrimSpace(font.Typeface)}
		appendFace := func(part *parts.FontPart, weight int, style string) {
			if part == nil || len(part.Data) == 0 {
				return
			}
			resolved.Faces = append(resolved.Faces, &EmbeddedFontFace{
				Family:      resolved.Family,
				Weight:      weight,
				Style:       style,
				ContentType: part.ContentType,
				Name:        part.Name,
				Data:        part.Data,
			})
		}
		appendFace(font.Regular, 400, "normal")
		appendFace(font.Bold, 700, "normal")
		appendFace(font.Italic, 400, "italic")
		appendFace(font.BoldItalic, 700, "italic")
		if len(resolved.Faces) == 0 {
			continue
		}
		out = append(out, resolved)
	}
	return out
}

func firstSlideMasterEntry(pkg *parts.Package) *parts.SlideMasterEntry {
	for _, me := range pkg.SlideMasters {
		if me.Master != nil {
			return me
		}
	}
	return nil
}

// buildMasterIDByPart maps each slide-master OPC part name to its persist ID
// declared in presentation.xml > sldMasterIdLst. Empty for packages that do
// not list their masters (uncommon).
func buildMasterIDByPart(pkg *parts.Package) map[string]int {
	if pkg == nil || pkg.Presentation == nil {
		return nil
	}
	partByRelID := make(map[string]string, len(pkg.SlideMasters))
	for _, me := range pkg.SlideMasters {
		if me == nil || me.RelID == "" {
			continue
		}
		partByRelID[me.RelID] = me.PartName
	}
	out := make(map[string]int, len(pkg.Presentation.SlideMasterIDs))
	for _, ref := range pkg.Presentation.SlideMasterIDs {
		if ref.ID == 0 {
			continue
		}
		if part := partByRelID[ref.RelID]; part != "" {
			out[part] = ref.ID
		}
	}
	return out
}

// slideMasterForEntry returns the slide master (and its entry) that should be
// used for the given slide, following POI XSLF parity: each slide uses the
// master linked by its layout, not a presentation-wide default.
func slideMasterForEntry(pkg *parts.Package, entry *parts.SlideEntry) (*parts.SlideMaster, *parts.SlideMasterEntry) {
	// Prefer the master explicitly resolved from the layout's relationships.
	if entry.MasterPartName != "" {
		for _, me := range pkg.SlideMasters {
			if me.PartName == entry.MasterPartName && me.Master != nil {
				return me.Master, me
			}
		}
	}
	// Fallback: locate the master that owns this slide's layout.
	if entry.Layout != nil && entry.Layout.PartName != "" {
		for _, me := range pkg.SlideMasters {
			for _, layout := range me.Layouts {
				if layout.PartName == entry.Layout.PartName {
					return me.Master, me
				}
			}
		}
	}
	// Final fallback: first available master (preserves old behavior).
	me := firstSlideMasterEntry(pkg)
	if me != nil {
		return me.Master, me
	}
	return nil, nil
}

func firstThemePart(pkg *parts.Package) *parts.Theme {
	for _, t := range pkg.Themes {
		return t
	}
	return nil
}

func resolveTheme(t *parts.Theme) *Theme {
	if t == nil {
		return nil
	}
	out := &Theme{
		Name:        t.Name,
		ColorScheme: make(map[string]string),
		MajorFont:   t.Elements.FontScheme.MajorFont.Latin.Typeface,
		MinorFont:   t.Elements.FontScheme.MinorFont.Latin.Typeface,
		MajorEAFont: t.Elements.FontScheme.MajorFont.EA.Typeface,
		MinorEAFont: t.Elements.FontScheme.MinorFont.EA.Typeface,
		MajorCSFont: t.Elements.FontScheme.MajorFont.CS.Typeface,
		MinorCSFont: t.Elements.FontScheme.MinorFont.CS.Typeface,
	}
	cs := t.Elements.ClrScheme
	resolveThemeColor(out.ColorScheme, "dk1", cs.DK1)
	resolveThemeColor(out.ColorScheme, "lt1", cs.LT1)
	resolveThemeColor(out.ColorScheme, "dk2", cs.DK2)
	resolveThemeColor(out.ColorScheme, "lt2", cs.LT2)
	resolveThemeColor(out.ColorScheme, "accent1", cs.Accent1)
	resolveThemeColor(out.ColorScheme, "accent2", cs.Accent2)
	resolveThemeColor(out.ColorScheme, "accent3", cs.Accent3)
	resolveThemeColor(out.ColorScheme, "accent4", cs.Accent4)
	resolveThemeColor(out.ColorScheme, "accent5", cs.Accent5)
	resolveThemeColor(out.ColorScheme, "accent6", cs.Accent6)
	resolveThemeColor(out.ColorScheme, "hlink", cs.HLink)
	resolveThemeColor(out.ColorScheme, "folHlink", cs.FolHLink)
	return out
}

func themeOverrideForLayout(layout *parts.SlideLayout) *parts.ThemeOverride {
	if layout == nil {
		return nil
	}
	return layout.ThemeOverride
}

func themeOverrideForSlide(slide *parts.Slide) *parts.ThemeOverride {
	if slide == nil {
		return nil
	}
	return slide.ThemeOverride
}

func applyThemeOverride(base *parts.Theme, override *parts.ThemeOverride) *parts.Theme {
	if base == nil || override == nil {
		return base
	}
	merged := *base
	merged.Elements = base.Elements
	if override.ClrScheme != nil {
		merged.Elements.ClrScheme = *override.ClrScheme
	}
	if override.FontScheme != nil {
		merged.Elements.FontScheme = *override.FontScheme
	}
	if override.FmtScheme != nil {
		merged.Elements.FmtScheme = *override.FmtScheme
	}
	return &merged
}

func resolveThemeColor(m map[string]string, slot string, tc *parts.ThemeColor) {
	if tc == nil {
		return
	}
	if tc.SrgbClr != nil {
		m[slot] = tc.SrgbClr.Val
		return
	}
	if tc.SysClr != nil && tc.SysClr.LastClr != "" {
		m[slot] = tc.SysClr.LastClr
	}
}

func colorMapOverrideForLayout(layout *parts.SlideLayout) *parts.ColorMapOverride {
	if layout == nil {
		return nil
	}
	return layout.ClrMapOvr
}

func colorMapOverrideForSlide(slide *parts.Slide) *parts.ColorMapOverride {
	if slide == nil {
		return nil
	}
	return slide.ClrMapOvr
}

func buildThemeColorMap(base map[string]string, colorMap *parts.ColorMap) map[string]string {
	out := make(map[string]string, len(base)+16)
	for key, value := range base {
		setThemeColor(out, key, value)
	}
	aliases := presentationColorMapAliases()
	if colorMap != nil {
		override := colorMapAliases(colorMap)
		for key, value := range override {
			aliases[key] = value
		}
	}
	for alias, target := range aliases {
		setThemeColor(out, alias, themeColor(base, target))
	}
	return out
}

func applyColorMapOverride(base, inherited map[string]string, override *parts.ColorMapOverride) map[string]string {
	if override == nil {
		return inherited
	}
	if override.MasterClrMapping != nil || override.OverrideClrMapping == nil {
		return inherited
	}
	return buildThemeColorMap(base, &override.OverrideClrMapping.ColorMap)
}

func presentationColorMapAliases() map[string]string {
	return map[string]string{
		"bg1":           "lt1",
		"tx1":           "dk1",
		"bg2":           "lt2",
		"tx2":           "dk2",
		"accent1":       "accent1",
		"accent2":       "accent2",
		"accent3":       "accent3",
		"accent4":       "accent4",
		"accent5":       "accent5",
		"accent6":       "accent6",
		"hlink":         "hlink",
		"followedHlink": "folHlink",
		"folHlink":      "folHlink",
	}
}

func colorMapAliases(colorMap *parts.ColorMap) map[string]string {
	if colorMap == nil {
		return nil
	}
	return map[string]string{
		"bg1":           strings.TrimSpace(colorMap.Bg1),
		"tx1":           strings.TrimSpace(colorMap.Tx1),
		"bg2":           strings.TrimSpace(colorMap.Bg2),
		"tx2":           strings.TrimSpace(colorMap.Tx2),
		"accent1":       strings.TrimSpace(colorMap.Accent1),
		"accent2":       strings.TrimSpace(colorMap.Accent2),
		"accent3":       strings.TrimSpace(colorMap.Accent3),
		"accent4":       strings.TrimSpace(colorMap.Accent4),
		"accent5":       strings.TrimSpace(colorMap.Accent5),
		"accent6":       strings.TrimSpace(colorMap.Accent6),
		"hlink":         strings.TrimSpace(colorMap.Hlink),
		"followedHlink": strings.TrimSpace(colorMap.FollowedHlink),
		"folHlink":      strings.TrimSpace(colorMap.FollowedHlink),
	}
}

func themeColor(themeColors map[string]string, key string) string {
	if themeColors == nil {
		return ""
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if value := strings.TrimSpace(themeColors[key]); value != "" {
		return value
	}
	return strings.TrimSpace(themeColors[strings.ToLower(key)])
}

func setThemeColor(themeColors map[string]string, key, value string) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	themeColors[key] = value
	themeColors[strings.ToLower(key)] = value
}

func pictureTileInfo(tile *drawingxml.Tile) *PictureTile {
	if tile == nil {
		return nil
	}
	return &PictureTile{
		Tx:        int64(tile.Tx),
		Ty:        int64(tile.Ty),
		Sx:        int(tile.Sx),
		Sy:        int(tile.Sy),
		Flip:      tile.Flip,
		Alignment: tile.Algn,
	}
}

func backgroundPictureTile(bg *parts.SlideBg, rawTheme *parts.Theme) *PictureTile {
	if bg == nil {
		return nil
	}
	if bg.BgPrBlipFill != nil {
		return pictureTileInfo(bg.BgPrBlipFill.Tile)
	}
	if bg.BgRef != nil && rawTheme != nil {
		idx := bg.BgRef.Idx - 1001
		if idx >= 0 && idx < len(rawTheme.Elements.FmtScheme.BGFillStyleList) {
			style := &rawTheme.Elements.FmtScheme.BGFillStyleList[idx]
			if style.BlipFill != nil {
				return pictureTileInfo(style.BlipFill.Tile)
			}
		}
	}
	return nil
}

func effectiveShapePictureTile(sp, layoutShape, masterShape *parts.Shape, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string) *PictureTile {
	if sp != nil {
		if fill := sp.SpPr.DrawingFill(slideThemeColors); fill != nil {
			if sp.SpPr.BlipFill != nil {
				return pictureTileInfo(sp.SpPr.BlipFill.Tile)
			}
			return nil
		}
	}
	if layoutShape != nil {
		if fill := layoutShape.SpPr.DrawingFill(layoutThemeColors); fill != nil {
			if layoutShape.SpPr.BlipFill != nil {
				return pictureTileInfo(layoutShape.SpPr.BlipFill.Tile)
			}
			return nil
		}
	}
	if masterShape != nil {
		if fill := masterShape.SpPr.DrawingFill(masterThemeColors); fill != nil {
			if masterShape.SpPr.BlipFill != nil {
				return pictureTileInfo(masterShape.SpPr.BlipFill.Tile)
			}
		}
	}
	return nil
}

func resolveSlide(index int, entry *parts.SlideEntry, master *parts.SlideMaster, masterEntry *parts.SlideMasterEntry, notesMaster *parts.NotesMaster, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string, theme *Theme, rawTheme *parts.Theme, tableStyles *parts.TableStyleList) *Slide {
	slide := &Slide{Index: index}
	if entry.Slide == nil {
		return slide
	}
	slide.BgColor = resolveBgColor(entry.Slide.CSld.Bg, slideThemeColors)
	if slide.BgColor == "" && entry.Layout != nil {
		slide.BgColor = resolveBgColor(entry.Layout.CSld.Bg, layoutThemeColors)
	}
	if slide.BgColor == "" && master != nil {
		slide.BgColor = resolveBgColor(master.CSld.Bg, masterThemeColors)
	}
	slide.BgFill = resolveBackgroundFill(entry.Slide.CSld.Bg, slideThemeColors, rawTheme, entry.MediaParts)
	if slide.BgFill != nil {
		slide.BgPictureTile = backgroundPictureTile(entry.Slide.CSld.Bg, rawTheme)
	}
	if slide.BgFill == nil && entry.Layout != nil {
		slide.BgFill = resolveBackgroundFill(entry.Layout.CSld.Bg, layoutThemeColors, rawTheme, entry.LayoutMediaParts)
		if slide.BgFill != nil {
			slide.BgPictureTile = backgroundPictureTile(entry.Layout.CSld.Bg, rawTheme)
		}
	}
	if slide.BgFill == nil && master != nil {
		var masterMediaParts map[string]media.Part
		if masterEntry != nil {
			masterMediaParts = masterEntry.MediaParts
		}
		slide.BgFill = resolveBackgroundFill(master.CSld.Bg, masterThemeColors, rawTheme, masterMediaParts)
		if slide.BgFill != nil {
			slide.BgPictureTile = backgroundPictureTile(master.CSld.Bg, rawTheme)
		}
	}
	// Prepend master decorative elements (non-placeholder shapes and pictures, rendered first/bottom).
	if masterEntry != nil && master != nil {
		for _, item := range master.CSld.SpTree.Items {
			switch item.Kind {
			case parts.ShapeTreeItemShape:
				sp := master.CSld.SpTree.Shapes[item.Index]
				if sp.NvSpPr.NvPr.Ph != nil {
					continue // skip placeholders — overridden by slide content
				}
				shape := resolveShape(&sp, nil, nil, master, masterThemeColors, masterThemeColors, masterThemeColors, theme, rawTheme, masterEntry.MediaParts, nil, nil, nil)
				if shape != nil {
					slide.Shapes = append(slide.Shapes, shape)
				}
			case parts.ShapeTreeItemPicture:
				pic := master.CSld.SpTree.Pictures[item.Index]
				shape := resolvePicture(&pic, masterEntry.MediaParts, masterThemeColors, nil)
				if shape != nil {
					slide.Shapes = append(slide.Shapes, shape)
				}
			}
		}
	}
	// Prepend layout decorative elements (non-placeholder shapes and pictures, above master).
	if entry.Layout != nil {
		for _, sp := range entry.Layout.CSld.SpTree.Shapes {
			if sp.NvSpPr.NvPr.Ph != nil {
				continue // skip placeholders — overridden by slide content
			}
			shape := resolveShape(&sp, nil, nil, master, layoutThemeColors, layoutThemeColors, masterThemeColors, theme, rawTheme, entry.LayoutMediaParts, nil, nil, nil)
			if shape != nil {
				slide.Shapes = append(slide.Shapes, shape)
			}
		}
		for _, pic := range entry.Layout.CSld.SpTree.Pictures {
			shape := resolvePicture(&pic, entry.LayoutMediaParts, layoutThemeColors, nil)
			if shape != nil {
				slide.Shapes = append(slide.Shapes, shape)
			}
		}
	}
	resolvedSlideShapes := make([]*Shape, len(entry.Slide.CSld.SpTree.Shapes))
	for i, sp := range entry.Slide.CSld.SpTree.Shapes {
		layoutShape := findLayoutPlaceholderShape(entry.Layout, sp.NvSpPr.NvPr.Ph)
		masterShape := findMasterPlaceholderShape(master, sp.NvSpPr.NvPr.Ph)
		var masterMediaParts map[string]media.Part
		if masterEntry != nil {
			masterMediaParts = masterEntry.MediaParts
		}
		resolvedSlideShapes[i] = resolveShape(&sp, layoutShape, masterShape, master, slideThemeColors, layoutThemeColors, masterThemeColors, theme, rawTheme, entry.MediaParts, entry.LayoutMediaParts, masterMediaParts, entry.HlinkURLs)
	}
	resolvedSlidePictures := make([]*Shape, len(entry.Slide.CSld.SpTree.Pictures))
	for i, pic := range entry.Slide.CSld.SpTree.Pictures {
		resolvedSlidePictures[i] = resolvePicture(&pic, entry.MediaParts, slideThemeColors, entry.HlinkURLs)
	}
	resolvedSlideGroups := make([]*Shape, len(entry.Slide.CSld.SpTree.GroupShapes))
	for i, grp := range entry.Slide.CSld.SpTree.GroupShapes {
		resolvedSlideGroups[i] = resolveGroupShape(&grp, master, slideThemeColors, theme, rawTheme, entry.MediaParts, entry.OleObjectParts, entry.HlinkURLs, tableStyles)
	}
	resolvedSlideTables := make([]*Shape, len(entry.Slide.CSld.SpTree.Tables))
	for i, tbl := range entry.Slide.CSld.SpTree.Tables {
		resolvedSlideTables[i] = resolveTableFrame(&tbl, slideThemeColors, theme, rawTheme, tableStyles)
	}
	resolvedOleObjects := make([]*Shape, len(entry.Slide.CSld.SpTree.OleObjectFrames))
	for i, ole := range entry.Slide.CSld.SpTree.OleObjectFrames {
		resolvedOleObjects[i] = resolveOleObjectFrame(&ole, entry)
	}
	shapeIndex := map[int]*Shape{}
	for _, shape := range slide.Shapes {
		indexShape(shapeIndex, shape)
	}
	for _, shape := range resolvedSlideShapes {
		indexShape(shapeIndex, shape)
	}
	for _, shape := range resolvedSlidePictures {
		indexShape(shapeIndex, shape)
	}
	for _, shape := range resolvedSlideGroups {
		indexShape(shapeIndex, shape)
	}
	for _, shape := range resolvedSlideTables {
		indexShape(shapeIndex, shape)
	}
	for _, shape := range resolvedOleObjects {
		indexShape(shapeIndex, shape)
	}
	for _, item := range orderedShapeTreeItems(entry.Slide.CSld.SpTree) {
		switch item.Kind {
		case parts.ShapeTreeItemShape:
			if shape := resolvedSlideShapes[item.Index]; shape != nil {
				// Check if this is a header/footer placeholder
				switch shape.Placeholder {
				case PlaceholderFooter:
					if headerFooterPlaceholderEnabled(master, PlaceholderFooter) {
						slide.Footer = shape
					}
				case PlaceholderDateTime:
					if headerFooterPlaceholderEnabled(master, PlaceholderDateTime) {
						slide.DateTime = shape
					}
				case PlaceholderSlideNumber:
					if headerFooterPlaceholderEnabled(master, PlaceholderSlideNumber) {
						slide.SlideNumber = shape
					}
				case PlaceholderOther:
					// Check if it's a header (type "hdr")
					if shape.Name == "hdr" || (shape.Name == "" && shape.Placeholder == PlaceholderOther) {
						slide.Header = shape
					} else {
						slide.Shapes = append(slide.Shapes, shape)
					}
				default:
					slide.Shapes = append(slide.Shapes, shape)
				}
			}
		case parts.ShapeTreeItemPicture:
			if shape := resolvedSlidePictures[item.Index]; shape != nil {
				slide.Shapes = append(slide.Shapes, shape)
			}
		case parts.ShapeTreeItemGroup:
			if shape := resolvedSlideGroups[item.Index]; shape != nil {
				slide.Shapes = append(slide.Shapes, shape)
			}
		case parts.ShapeTreeItemTable:
			if shape := resolvedSlideTables[item.Index]; shape != nil {
				slide.Shapes = append(slide.Shapes, shape)
			}
		case parts.ShapeTreeItemConnector:
			cx := entry.Slide.CSld.SpTree.Connectors[item.Index]
			shape := resolveConnector(&cx, slideThemeColors, shapeIndex)
			if shape != nil {
				slide.Shapes = append(slide.Shapes, shape)
			}
		case parts.ShapeTreeItemOleObject:
			if shape := resolvedOleObjects[item.Index]; shape != nil {
				slide.Shapes = append(slide.Shapes, shape)
			}
		}
	}
	if slide.Footer == nil {
		slide.Footer = resolveInheritedHeaderFooterPlaceholder(entry, master, masterEntry, PlaceholderFooter, slideThemeColors, layoutThemeColors, masterThemeColors, theme, rawTheme)
	}
	if slide.DateTime == nil {
		slide.DateTime = resolveInheritedHeaderFooterPlaceholder(entry, master, masterEntry, PlaceholderDateTime, slideThemeColors, layoutThemeColors, masterThemeColors, theme, rawTheme)
	}
	if slide.SlideNumber == nil {
		slide.SlideNumber = resolveInheritedHeaderFooterPlaceholder(entry, master, masterEntry, PlaceholderSlideNumber, slideThemeColors, layoutThemeColors, masterThemeColors, theme, rawTheme)
	}
	// Resolve SmartArt parts and add as shapes
	frameByDataRel := buildSmartArtFrameMap(entry)
	for _, smartArtPart := range entry.SmartArtParts {
		frame := frameByDataRel[smartArtPart.RelID]
		smartArtShapes, smartArtDiags := resolveSmartArt(smartArtPart, frame, slideThemeColors, rawTheme, theme)
		slide.Shapes = append(slide.Shapes, smartArtShapes...)
		slide.Diagnostics = append(slide.Diagnostics, smartArtDiags...)
	}
	// Resolve Chart parts and add as shapes
	for relID, chartXML := range entry.ChartParts {
		chartShape := resolveChart(relID, chartXML, entry.ChartFrames[relID], slideThemeColors)
		if chartShape != nil {
			slide.Shapes = append(slide.Shapes, chartShape)
		}
	}
	// Resolve notes slide if present
	if entry.Notes != nil {
		slide.Notes = resolveNotesSlide(entry.Notes, notesMaster, slideThemeColors, theme, rawTheme, tableStyles)
	}
	// Copy transition metadata.
	if entry.Slide.Transition != nil {
		pt := entry.Slide.Transition
		slide.Transition = &SlideTransition{
			Type:     pt.Type,
			Spd:      pt.Spd,
			AdvClick: pt.AdvClick,
			AdvTmMs:  pt.AdvTmMs,
		}
	}
	if len(entry.Slide.AnimationEvents) > 0 {
		slide.AnimationTimeline = make([]AnimationEvent, 0, len(entry.Slide.AnimationEvents))
		for _, ev := range entry.Slide.AnimationEvents {
			slide.AnimationTimeline = append(slide.AnimationTimeline, AnimationEvent{
				ShapeID: ev.ShapeID, Step: entry.Slide.AnimSteps[ev.ShapeID], NodeType: ev.NodeType, Behavior: ev.Behavior, Effect: ev.Effect, Trigger: ev.Trigger,
				DelayMs: ev.DelayMs, DurationMs: ev.DurationMs, RepeatCount: ev.RepeatCount, RepeatDurMs: ev.RepeatDurMs,
				AutoReverse: ev.AutoReverse, Fill: ev.Fill, MotionPath: ev.MotionPath, MotionOrigin: ev.MotionOrigin,
			})
		}
	}
	// Apply per-shape animation entrance step from p:timing.
	if len(entry.Slide.AnimSteps) > 0 {
		applyAnimSteps(slide.Shapes, entry.Slide.AnimSteps)
	}
	// Apply per-shape emphasis animation effects from p:timing.
	if len(entry.Slide.EmphasisEffects) > 0 {
		applyEmphasisEffects(slide.Shapes, entry.Slide.EmphasisEffects)
	}
	// Apply per-shape motion path animations from p:timing.
	if len(entry.Slide.MotionPaths) > 0 {
		applyMotionPaths(slide.Shapes, entry.Slide.MotionPaths)
	}
	// Apply per-shape animation triggers from p:timing.
	if len(entry.Slide.AnimationTriggers) > 0 {
		applyAnimationTriggers(slide.Shapes, entry.Slide.AnimationTriggers)
	}
	for _, shape := range slide.Shapes {
		syncPresentationOfficeArtShape(shape)
	}

	return slide
}

// applyAnimSteps walks all shapes (including children of groups) and sets
// AnimStep from the shape ID → click-step map extracted from p:timing.
func applyAnimSteps(shapes []*Shape, steps map[int]int) {
	for _, shape := range shapes {
		if step, ok := steps[shape.ID]; ok {
			shape.AnimStep = step
		}
		if len(shape.Children) > 0 {
			applyAnimSteps(shape.Children, steps)
		}
	}
}

// applyEmphasisEffects walks all shapes and attaches emphasis animation
// effect names from the slide-level EmphasisEffects map.
func applyEmphasisEffects(shapes []*Shape, effects map[int][]string) {
	for _, shape := range shapes {
		if fx, ok := effects[shape.ID]; ok {
			shape.EmphasisEffects = fx
		}
		if len(shape.Children) > 0 {
			applyEmphasisEffects(shape.Children, effects)
		}
	}
}

// applyMotionPaths walks all shapes and attaches motion path data from the
// slide-level MotionPaths map.
func applyMotionPaths(shapes []*Shape, paths map[int]parts.AnimMotionPath) {
	for _, shape := range shapes {
		if mp, ok := paths[shape.ID]; ok {
			shape.MotionPath = &AnimMotionPath{
				Path:   mp.Path,
				Dur:    mp.Dur,
				Origin: mp.Origin,
			}
		}
		if len(shape.Children) > 0 {
			applyMotionPaths(shape.Children, paths)
		}
	}
}

// applyAnimationTriggers walks all shapes and attaches trigger event names
// from the slide-level AnimationTriggers map.
func applyAnimationTriggers(shapes []*Shape, triggers map[int][]string) {
	for _, shape := range shapes {
		if evts, ok := triggers[shape.ID]; ok {
			shape.AnimationTriggers = evts
		}
		if len(shape.Children) > 0 {
			applyAnimationTriggers(shape.Children, triggers)
		}
	}
}

// resolveChart converts chart XML to a renderable shape with SVG data. When
// xfrm is provided, the chart is rendered at the hosting p:graphicFrame's
// position and size; otherwise a default 4x3 inch top-left placement is used.
func resolveChart(relID string, chartXML []byte, xfrm *parts.Transform2D, themeColors map[string]string) *Shape {
	if len(chartXML) == 0 {
		return nil
	}

	c, err := chart.ParseChart(chartXML)
	if err != nil || c == nil {
		return nil
	}
	c.ThemeColors = themeColors

	bounds := Rect{X: 0, Y: 0, W: 5760000, H: 3240000} // Default 4x3 inches in EMU
	widthPt, heightPt := 400, 300
	if xfrm != nil {
		bounds.X = xfrm.Off.X
		bounds.Y = xfrm.Off.Y
		bounds.W = xfrm.Ext.CX
		bounds.H = xfrm.Ext.CY
		widthPt = int(bounds.W / 12700)
		heightPt = int(bounds.H / 12700)
		if widthPt <= 0 {
			widthPt = 400
		}
		if heightPt <= 0 {
			heightPt = 300
		}
	}

	svgStr := c.SVG(widthPt, heightPt)
	if svgStr == "" {
		return nil
	}

	return &Shape{
		ID:        0, // Charts don't have shape IDs
		Name:      "Chart",
		Kind:      ShapeChart,
		Bounds:    bounds,
		ImageData: []byte(svgStr),
		ImageMIME: "image/svg+xml",
	}
}

// resolveOleObjectFrame converts a parsed OLE object frame into a placeholder shape.
func resolveOleObjectFrame(frame *parts.OleObjectFrame, entry *parts.SlideEntry) *Shape {
	if frame == nil {
		return nil
	}
	shape := &Shape{
		ID:              frame.ID,
		OriginalShapeID: uint32(frame.ID),
		Name:            frame.Name,
		Kind:            ShapeOleObject,
		Bounds:          resolveTransform(frame.Xfrm),
		RotationDeg:     shapeRotationDegrees(frame.Xfrm),
		FlipH:           frame.Xfrm != nil && frame.Xfrm.FlipH,
		FlipV:           frame.Xfrm != nil && frame.Xfrm.FlipV,
		OLEObjectName:   frame.ObjectName,
		OLEObjectProgID: frame.ProgID,
	}
	if entry != nil && entry.OleObjectParts != nil {
		if part, ok := entry.OleObjectParts[normalizeRelID(frame.RelID)]; ok {
			shape.OLEObjectData = part.Data
			shape.OLEObjectMIME = part.ContentType
		}
	}
	if !frame.Embed && entry != nil && entry.OleObjectLinks != nil {
		shape.OLEObjectLinkURL = entry.OleObjectLinks[normalizeRelID(frame.RelID)]
	}
	if frame.IconRelID != "" && entry != nil && entry.MediaParts != nil {
		if part, ok := entry.MediaParts[normalizeRelID(frame.IconRelID)]; ok {
			shape.OLEIconData = part.Data
			shape.OLEIconMIME = part.ContentType
		}
	}
	return shape
}

// buildSmartArtFrameMap indexes the hosting p:graphicFrame for each SmartArt
// data-model relationship on the slide, including frames nested in groups.
func buildSmartArtFrameMap(entry *parts.SlideEntry) map[string]*parts.SmartArtFrame {
	m := make(map[string]*parts.SmartArtFrame)
	if entry == nil || entry.Slide == nil {
		return m
	}
	for i := range entry.Slide.CSld.SpTree.SmartArtFrames {
		frame := &entry.Slide.CSld.SpTree.SmartArtFrames[i]
		if dm := frame.RelIDs["dm"]; dm != "" {
			m[dm] = frame
		}
	}
	var walk func(*parts.GroupShape)
	walk = func(g *parts.GroupShape) {
		if g == nil {
			return
		}
		for i := range g.SmartArtFrames {
			frame := &g.SmartArtFrames[i]
			if dm := frame.RelIDs["dm"]; dm != "" {
				m[dm] = frame
			}
		}
		for i := range g.GroupShapes {
			walk(&g.GroupShapes[i])
		}
	}
	for i := range entry.Slide.CSld.SpTree.GroupShapes {
		walk(&entry.Slide.CSld.SpTree.GroupShapes[i])
	}
	return m
}

// resolveSmartArt converts a SmartArt part to renderable shapes.
func resolveSmartArt(part *parts.SmartArtPart, frame *parts.SmartArtFrame, themeColors map[string]string, rawTheme *parts.Theme, theme *Theme) ([]*Shape, diag.List) {
	var diags diag.List
	if part == nil || len(part.DataModelXML) == 0 {
		if part != nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     "smartart.empty-data-model",
				Message:  "SmartArt part has no data model; skipping diagram",
				Location: diag.Location{Part: part.RelID},
			})
		}
		return nil, diags
	}

	if len(part.LayoutXML) == 0 {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityInfo,
			Code:     "smartart.missing-layout",
			Message:  "SmartArt layout definition not present; using fallback layout",
			Location: diag.Location{Part: part.RelID},
		})
	}
	if len(part.StyleXML) == 0 {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityInfo,
			Code:     "smartart.missing-style",
			Message:  "SmartArt quick-style part not present; using default style",
			Location: diag.Location{Part: part.RelID},
		})
	}
	if len(part.ColorsXML) == 0 {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityInfo,
			Code:     "smartart.missing-colors",
			Message:  "SmartArt color part not present; using default palette",
			Location: diag.Location{Part: part.RelID},
		})
	}

	var frameBounds *smartartrender.FrameBounds
	if frame != nil && frame.Xfrm != nil {
		frameBounds = &smartartrender.FrameBounds{
			X: float64(frame.Xfrm.Off.X),
			Y: float64(frame.Xfrm.Off.Y),
			W: float64(frame.Xfrm.Ext.CX),
			H: float64(frame.Xfrm.Ext.CY),
		}
	}

	fmtScheme := buildSmartArtFormatScheme(rawTheme, themeColors, theme)

	// Use the smartart render package to process the diagram. Pass the layout,
	// style, color, and optional pre-rendered drawing parts so exact geometry
	// can be used when available.  Theme colors are supplied so schemeClr values
	// in the color style XML can be resolved. A resolved format scheme lets the
	// quick-style matrix references produce correct fills and lines.
	smartArtShapes, err := smartartrender.ProcessDiagramWithOptionsAndFrameFormat(
		part.DataModelXML,
		part.LayoutXML,
		part.StyleXML,
		part.ColorsXML,
		part.DrawingXML,
		frameBounds,
		themeColors,
		fmtScheme,
	)
	if err != nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityWarning,
			Code:     "smartart.process-failed",
			Message:  "SmartArt diagram processing failed; skipping diagram",
			Location: diag.Location{Part: part.RelID},
			Cause:    err,
		})
		return nil, diags
	}

	// Parse the diagram again to access layout metadata for connector routing.
	diagram, parseErr := smartart.ParseDiagramWithOptionsAndThemeFormat(
		part.DataModelXML,
		part.LayoutXML,
		part.StyleXML,
		part.ColorsXML,
		themeColors,
		fmtScheme,
	)
	if parseErr != nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityInfo,
			Code:     "smartart.layout-parse-failed",
			Message:  "Could not parse SmartArt layout metadata; using fallback connector routing",
			Location: diag.Location{Part: part.RelID},
			Cause:    parseErr,
		})
	}
	if diagram == nil {
		diagram = &smartart.Diagram{Layout: &smartart.Layout{Type: "hierarchy"}}
	}
	if len(diagram.Diagnostics) > 0 {
		for _, d := range diagram.Diagnostics {
			diags.Add(d)
		}
	}

	// A pre-rendered drawing part can override the style-defined connector line.
	// ProcessDiagram applies the override to its internal diagram, but we re-parse
	// the diagram above, so re-apply it here.
	if len(part.DrawingXML) > 0 {
		if connStyle := smartartrender.ExtractConnectorStyle(part.DrawingXML, themeColors); connStyle != nil {
			if diagram.Style == nil {
				diagram.Style = &smartart.Style{}
			}
			diagram.Style.ConnectorLine = connStyle
		}
	}

	// Resolve embedded image relationships for picture fills before converting.
	mediaParts := part.MediaParts
	for _, sa := range smartArtShapes {
		if pf, ok := sa.Fill.(*drawing.PictureFill); ok && pf != nil && pf.EmbedID != "" {
			if m, ok := mediaParts[pf.EmbedID]; ok {
				pf.ImageData = m.Data
				pf.ContentType = m.ContentType
			}
		}
	}

	// Convert SmartArt shapes to resolve.Shape
	var shapes []*Shape
	nodeMap := make(map[string]*Shape)
	for _, sa := range smartArtShapes {
		shape := &Shape{
			ID:                  0, // SmartArt shapes don't have IDs
			Name:                "SmartArt",
			Kind:                ShapeSmartArt,
			Bounds:              Rect{X: int64(sa.X * 12700), Y: int64(sa.Y * 12700), W: int64(sa.Width * 12700), H: int64(sa.Height * 12700)},
			Fill:                sa.Fill,
			LineStyle:           sa.Line,
			Effects:             sa.Effect,
			Geometry:            sa.Type,
			GeometryAdjustments: sa.GeometryAdjustments,
			GeometryData:        sa.CustomGeometry,
			DrawingOrder:        sa.DrawingOrder,
			RotationDeg:         sa.RotationDeg,
			FlipH:               sa.FlipH,
			FlipV:               sa.FlipV,
			SmartArtNodeID:      sa.ID,
			TextBodyProps:       sa.TextBodyProps,
		}
		for _, c := range sa.Connections {
			shape.SmartArtConnections = append(shape.SmartArtConnections, c.ToID)
		}
		// Derive legacy string fields from shared types for renderers
		// that have not yet migrated to the drawing types.
		if sf, ok := sa.Fill.(drawing.SolidFill); ok {
			shape.FillColor = sf.Color
		}
		if sa.Line != nil {
			shape.LineColor = sa.Line.Color
			shape.LineWidthPt = sa.Line.Width
		}
		// Add text as styled paragraphs when the drawing part supplied them,
		// otherwise fall back to a single plain-text paragraph.
		if len(sa.Paragraphs) > 0 {
			shape.Paragraphs = make([]*Paragraph, 0, len(sa.Paragraphs))
			for _, p := range sa.Paragraphs {
				para := &Paragraph{
					Align:             p.Align,
					Level:             p.Level,
					BulletChar:        p.BulletChar,
					AutoNum:           p.AutoNum,
					AutoNumType:       p.AutoNumType,
					AutoNumStart:      p.AutoNumStart,
					BulletColor:       p.BulletColor,
					BulletFontFamily:  p.BulletFontFamily,
					BulletSize:        p.BulletSize,
					BulletSizePct:     p.BulletSizePct,
					SpaceBefore:       p.SpaceBefore,
					SpaceAfter:        p.SpaceAfter,
					SpaceBeforePct:    p.SpaceBeforePct,
					SpaceAfterPct:     p.SpaceAfterPct,
					LineSpacingPct:    p.LineSpacingPct,
					HasLineSpacing:    p.LineSpacingPct != 0 || p.LineSpacingPts != 0,
					DefaultTabSizeEMU: p.DefaultTabSizeEMU,
				}
				if len(p.TabStops) > 0 {
					para.TabStops = make([]TabStop, 0, len(p.TabStops))
					for _, stop := range p.TabStops {
						para.TabStops = append(para.TabStops, TabStop{Pos: stop.Pos, Algn: stop.Algn})
					}
				}
				for _, r := range p.Runs {
					color := r.Color
					if color == "" {
						color = sa.TextColor
					}
					font := r.FontFamily
					if font == "" {
						font = r.FontFamilyEA
					}
					if font == "" {
						font = r.FontFamilyCS
					}
					para.Runs = append(para.Runs, &Run{
						Text:       r.Text,
						Bold:       r.Bold,
						Italic:     r.Italic,
						Underline:  r.Underline,
						FontSize:   r.FontSize,
						FontFamily: font,
						Color:      color,
						Effects:    r.Effects,
					})
				}
				shape.Paragraphs = append(shape.Paragraphs, para)
			}
		} else {
			text := sa.Text
			if text == "" {
				text = sa.PlaceholderText
			}
			if text != "" {
				shape.Paragraphs = []*Paragraph{
					{
						Runs: []*Run{
							{Text: text, Color: sa.TextColor},
						},
					},
				}
			}
		}
		shapes = append(shapes, shape)
		nodeMap[sa.ID] = shape
	}

	// Generate connector shapes between connected SmartArt nodes.
	connLine := &drawing.LineStyle{Color: "999999", Width: 1.0}
	if diagram != nil && diagram.Style != nil && diagram.Style.ConnectorLine != nil {
		connLine = diagram.Style.ConnectorLine
	} else if len(shapes) > 0 && shapes[0].LineStyle != nil {
		connLine = shapes[0].LineStyle
	}
	layoutType := diagram.Layout.Type
	layoutDir := diagram.Layout.Direction
	connectorLayout := diagram.Layout

	// When a pre-rendered drawing part is present, try to match its connector
	// geometries to the diagram's logical edges in declaration order.
	preRenderedConns := smartartrender.ExtractConnectors(part.DrawingXML, themeColors)
	connScale, connOffsetX, connOffsetY := smartArtConnectorTransform(part.DrawingXML, frameBounds, themeColors)
	sort.Slice(preRenderedConns, func(i, j int) bool {
		return preRenderedConns[i].DrawingOrder < preRenderedConns[j].DrawingOrder
	})

	// Build the ordered list of logical edges.
	type smartArtEdge struct {
		srcID     string
		dstID     string
		assistant bool
	}
	assistantByID := make(map[string]bool)
	if diagram != nil {
		for _, n := range diagram.Nodes {
			if n != nil && n.Assistant {
				assistantByID[n.ID] = true
			}
		}
	}
	var edges []smartArtEdge
	if diagram != nil && len(diagram.Connections) > 0 {
		for _, c := range diagram.Connections {
			edges = append(edges, smartArtEdge{srcID: c.SrcID, dstID: c.DestID, assistant: assistantByID[c.DestID]})
		}
	} else {
		for _, src := range shapes {
			for _, dstID := range src.SmartArtConnections {
				edges = append(edges, smartArtEdge{srcID: src.SmartArtNodeID, dstID: dstID, assistant: assistantByID[dstID]})
			}
		}
	}

	for i, edge := range edges {
		if !smartArtConnectorEdgeAllowed(edge.srcID, connectorLayout.ConnectorSourceIDs) ||
			!smartArtConnectorEdgeAllowed(edge.dstID, connectorLayout.ConnectorDestIDs) {
			continue
		}
		src, ok1 := nodeMap[edge.srcID]
		dst, ok2 := nodeMap[edge.dstID]
		if !ok1 || !ok2 {
			continue
		}

		var conn *Shape
		if i < len(preRenderedConns) {
			conn = buildSmartArtConnectorFromDrawing(preRenderedConns[i], connLine, connScale, connOffsetX, connOffsetY)
		}
		if conn == nil {
			conn = buildSyntheticSmartArtConnector(src, dst, connLine, layoutType, layoutDir, edge.assistant, connectorLayout)
		}
		shapes = append(shapes, conn)
	}

	// Preserve the source drawing order for SmartArt shapes when a pre-rendered
	// drawing part supplied it. Connectors and fallback-laid-out shapes keep
	// their relative positions.
	sort.SliceStable(shapes, func(i, j int) bool {
		if shapes[i].DrawingOrder == 0 || shapes[j].DrawingOrder == 0 {
			return false
		}
		return shapes[i].DrawingOrder < shapes[j].DrawingOrder
	})

	return shapes, diags
}

// smartArtConnectorTransform computes the uniform scale and offset needed to map
// pre-rendered drawing connector coordinates into the hosting graphic frame.
// Values are returned in points.
func smartArtConnectorTransform(drawingXML []byte, frame *smartartrender.FrameBounds, themeColors map[string]string) (scale, offsetX, offsetY float64) {
	if len(drawingXML) == 0 || frame == nil {
		return 1, 0, 0
	}
	shapes := smartartrender.ExtractShapeInfos(drawingXML, themeColors)
	if len(shapes) == 0 {
		return 1, 0, 0
	}
	minX, minY, maxX, maxY, ok := smartartrender.DrawingBounds(shapes)
	if !ok {
		return 1, 0, 0
	}
	drawingW := maxX - minX
	drawingH := maxY - minY
	if drawingW == 0 || drawingH == 0 {
		return 1, 0, 0
	}
	frameX := frame.X / emuPerPt
	frameY := frame.Y / emuPerPt
	frameW := frame.W / emuPerPt
	frameH := frame.H / emuPerPt
	if frameW <= 0 || frameH <= 0 {
		return 1, 0, 0
	}
	scale = frameW / drawingW
	if frameH/drawingH < scale {
		scale = frameH / drawingH
	}
	offsetX = frameX + (frameW-drawingW*scale)/2
	offsetY = frameY + (frameH-drawingH*scale)/2
	return scale, offsetX, offsetY
}

const emuPerPt = 12700.0

// buildSmartArtConnectorFromDrawing creates a resolve.Shape from a pre-rendered
// dsp:cxnSp connector. The connector bounds are scaled uniformly to fit inside
// the hosting graphic frame, matching the transform applied to shape positions.
func buildSmartArtConnectorFromDrawing(info *smartartrender.DrawingConnectorInfo, connLine *drawing.LineStyle, scale, offsetX, offsetY float64) *Shape {
	if info == nil {
		return nil
	}
	line := mergeSmartArtConnectorLine(connLine, info.Line)
	conn := &Shape{
		Kind:                 ShapeSmartArtConnector,
		Name:                 "SmartArt Connector",
		LineStyle:            line,
		LineColor:            line.Color,
		LineWidthPt:          line.Width,
		ConnectorType:        info.Geometry,
		ConnectorFlipH:       info.FlipH,
		ConnectorFlipV:       info.FlipV,
		ConnectorAdjustments: smartArtConnectorAdjustments(info.Adjustments),
		DrawingOrder:         info.DrawingOrder,
		RotationDeg:          info.RotationDeg,
	}
	if line.HeadEnd != nil {
		conn.ArrowHead = string(line.HeadEnd.Type)
		conn.ArrowHeadWidth = string(line.HeadEnd.Size)
		conn.ArrowHeadLength = string(line.HeadEnd.Length)
	}
	if line.TailEnd != nil {
		conn.ArrowTail = string(line.TailEnd.Type)
		conn.ArrowTailWidth = string(line.TailEnd.Size)
		conn.ArrowTailLength = string(line.TailEnd.Length)
	}

	bx := info.X/emuPerPt*scale + offsetX
	by := info.Y/emuPerPt*scale + offsetY
	bw := info.W / emuPerPt * scale
	bh := info.H / emuPerPt * scale
	conn.Bounds = Rect{
		X: int64(bx * emuPerPt),
		Y: int64(by * emuPerPt),
		W: int64(bw * emuPerPt),
		H: int64(bh * emuPerPt),
	}
	return conn
}

// smartArtConnectorAdjustments converts 0-1 adjustment values from the drawing
// part into the 0-100000 integer scale used by drawing.ConnectorWaypoints.
func smartArtConnectorAdjustments(adjs []float64) []int {
	out := make([]int, len(adjs))
	for i, v := range adjs {
		out[i] = int(math.Round(v * 100000))
	}
	return out
}

// mergeSmartArtConnectorLine combines the diagram-wide connector line style
// with any overrides found on the pre-rendered connector.
func mergeSmartArtConnectorLine(base, override *drawing.LineStyle) *drawing.LineStyle {
	out := &drawing.LineStyle{Color: "999999", Width: 1.0}
	if base != nil {
		*out = *base
	}
	if override == nil {
		return out
	}
	if override.Color != "" {
		out.Color = override.Color
	}
	if override.Width > 0 {
		out.Width = override.Width
	}
	if override.Fill != nil {
		out.Fill = override.Fill
	}
	if override.Alpha != 0 {
		out.Alpha = override.Alpha
	}
	if override.Dash != "" {
		out.Dash = override.Dash
	}
	if override.Cap != "" {
		out.Cap = override.Cap
	}
	if override.Compound {
		out.Compound = true
		out.CompoundType = override.CompoundType
	}
	if override.HeadEnd != nil {
		out.HeadEnd = override.HeadEnd
	}
	if override.TailEnd != nil {
		out.TailEnd = override.TailEnd
	}
	return out
}

// buildSyntheticSmartArtConnector creates a straight or elbow connector between
// two SmartArt nodes when no pre-rendered connector geometry is available.
func buildSyntheticSmartArtConnector(src, dst *Shape, connLine *drawing.LineStyle, layoutType, layoutDir string, assistant bool, connectorLayouts ...*smartart.Layout) *Shape {
	var connectorLayout *smartart.Layout
	if len(connectorLayouts) > 0 {
		connectorLayout = connectorLayouts[0]
	}
	conn := &Shape{
		Kind:        ShapeSmartArtConnector,
		Name:        "SmartArt Connector",
		LineStyle:   connLine,
		LineColor:   connLine.Color,
		LineWidthPt: connLine.Width,
	}
	if connLine.HeadEnd != nil {
		conn.ArrowHead = string(connLine.HeadEnd.Type)
		conn.ArrowHeadWidth = string(connLine.HeadEnd.Size)
		conn.ArrowHeadLength = string(connLine.HeadEnd.Length)
	}
	if connLine.TailEnd != nil {
		conn.ArrowTail = string(connLine.TailEnd.Type)
		conn.ArrowTailWidth = string(connLine.TailEnd.Size)
		conn.ArrowTailLength = string(connLine.TailEnd.Length)
	}

	routing := ""
	bendPoint := ""
	begPts := ""
	endPts := ""
	dimension := ""
	if connectorLayout != nil {
		routing = strings.ToLower(strings.TrimSpace(connectorLayout.ConnectorRouting))
		bendPoint = strings.ToLower(strings.TrimSpace(connectorLayout.ConnectorBendPoint))
		begPts = connectorLayout.ConnectorBeginPts
		endPts = connectorLayout.ConnectorEndPts
		dimension = strings.ToLower(strings.TrimSpace(connectorLayout.ConnectorDimension))
		applySmartArtConnectorArrowStyle(conn, connectorLayout.ConnectorBeginStyle, true)
		applySmartArtConnectorArrowStyle(conn, connectorLayout.ConnectorEndStyle, false)
	}
	if dimension == "1d" {
		routing = "stra"
	}

	// Determine the authored or layout-derived endpoints first. Explicit
	// begPts/endPts take precedence over hierarchy-direction heuristics.
	srcCenter := Point{X: src.Bounds.X + src.Bounds.W/2, Y: src.Bounds.Y + src.Bounds.H/2}
	dstCenter := Point{X: dst.Bounds.X + dst.Bounds.W/2, Y: dst.Bounds.Y + dst.Bounds.H/2}
	start, startExplicit := smartArtConnectorAnchor(src.Bounds, begPts, dstCenter)
	end, endExplicit := smartArtConnectorAnchor(dst.Bounds, endPts, srcCenter)

	if !startExplicit || !endExplicit {
		if layoutType == "hierarchy" {
			if assistant {
				if dstCenter.X >= srcCenter.X {
					if !startExplicit {
						start = Point{X: src.Bounds.X + src.Bounds.W, Y: srcCenter.Y}
					}
					if !endExplicit {
						end = Point{X: dst.Bounds.X, Y: dstCenter.Y}
					}
				} else {
					if !startExplicit {
						start = Point{X: src.Bounds.X, Y: srcCenter.Y}
					}
					if !endExplicit {
						end = Point{X: dst.Bounds.X + dst.Bounds.W, Y: dstCenter.Y}
					}
				}
			} else {
				switch layoutDir {
				case "leftRight":
					if !startExplicit {
						start = Point{X: src.Bounds.X + src.Bounds.W, Y: srcCenter.Y}
					}
					if !endExplicit {
						end = Point{X: dst.Bounds.X, Y: dstCenter.Y}
					}
				case "rightLeft":
					if !startExplicit {
						start = Point{X: src.Bounds.X, Y: srcCenter.Y}
					}
					if !endExplicit {
						end = Point{X: dst.Bounds.X + dst.Bounds.W, Y: dstCenter.Y}
					}
				case "bottomUp":
					if !startExplicit {
						start = Point{X: srcCenter.X, Y: src.Bounds.Y}
					}
					if !endExplicit {
						end = Point{X: dstCenter.X, Y: dst.Bounds.Y + dst.Bounds.H}
					}
				default:
					if !startExplicit {
						start = Point{X: srcCenter.X, Y: src.Bounds.Y + src.Bounds.H}
					}
					if !endExplicit {
						end = Point{X: dstCenter.X, Y: dst.Bounds.Y}
					}
				}
			}
		} else {
			if !startExplicit {
				start = smartArtNearestPerimeterPoint(src.Bounds, dstCenter)
			}
			if !endExplicit {
				end = smartArtNearestPerimeterPoint(dst.Bounds, srcCenter)
			}
		}
	}

	conn.ConnectorStart = &start
	conn.ConnectorEnd = &end

	if routing == "" && layoutType == "hierarchy" {
		routing = "bend"
	}
	switch routing {
	case "bend":
		conn.ConnectorType = "elbow"
		conn.ConnectorPath = smartArtElbowPath(start, end, bendPoint)
	case "curve":
		conn.ConnectorType = "curved"
	case "longcurve":
		conn.ConnectorType = "longCurve"
	case "stra", "":
		conn.ConnectorType = "straight"
	default:
		conn.ConnectorType = "straight"
	}
	return conn
}

func smartArtConnectorEdgeAllowed(id string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if candidate == id {
			return true
		}
	}
	return false
}

func applySmartArtConnectorArrowStyle(conn *Shape, style string, beginning bool) {
	style = strings.ToLower(strings.TrimSpace(style))
	if style == "" || style == "auto" {
		return
	}
	if beginning {
		if style == "noarr" {
			conn.ArrowTail = ""
			return
		}
		if style == "arr" {
			conn.ArrowTail = "triangle"
			if conn.ArrowTailWidth == "" {
				conn.ArrowTailWidth = "med"
			}
			if conn.ArrowTailLength == "" {
				conn.ArrowTailLength = "med"
			}
		}
		return
	}
	if style == "noarr" {
		conn.ArrowHead = ""
		return
	}
	if style == "arr" {
		conn.ArrowHead = "triangle"
		if conn.ArrowHeadWidth == "" {
			conn.ArrowHeadWidth = "med"
		}
		if conn.ArrowHeadLength == "" {
			conn.ArrowHeadLength = "med"
		}
	}
}

func smartArtConnectorAnchor(bounds Rect, token string, toward Point) (Point, bool) {
	parts := strings.FieldsFunc(token, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' })
	if len(parts) == 0 {
		return Point{}, false
	}
	token = strings.ToLower(strings.TrimSpace(parts[0]))
	cx := bounds.X + bounds.W/2
	cy := bounds.Y + bounds.H/2
	switch token {
	case "bctr":
		return Point{X: cx, Y: bounds.Y + bounds.H}, true
	case "ctr":
		return Point{X: cx, Y: cy}, true
	case "midl":
		return Point{X: bounds.X, Y: cy}, true
	case "midr":
		return Point{X: bounds.X + bounds.W, Y: cy}, true
	case "tctr":
		return Point{X: cx, Y: bounds.Y}, true
	case "bl":
		return Point{X: bounds.X, Y: bounds.Y + bounds.H}, true
	case "br":
		return Point{X: bounds.X + bounds.W, Y: bounds.Y + bounds.H}, true
	case "tl":
		return Point{X: bounds.X, Y: bounds.Y}, true
	case "tr":
		return Point{X: bounds.X + bounds.W, Y: bounds.Y}, true
	case "radial":
		return smartArtNearestPerimeterPoint(bounds, toward), true
	case "auto", "":
		return Point{}, false
	default:
		return Point{}, false
	}
}

func smartArtNearestPerimeterPoint(bounds Rect, toward Point) Point {
	cx := bounds.X + bounds.W/2
	cy := bounds.Y + bounds.H/2
	dx := float64(toward.X - cx)
	dy := float64(toward.Y - cy)
	if dx == 0 && dy == 0 {
		return Point{X: cx, Y: cy}
	}
	hw := float64(bounds.W) / 2
	hh := float64(bounds.H) / 2
	sx, sy := math.Inf(1), math.Inf(1)
	if dx != 0 {
		sx = hw / math.Abs(dx)
	}
	if dy != 0 {
		sy = hh / math.Abs(dy)
	}
	t := math.Min(sx, sy)
	return Point{X: cx + int64(math.Round(dx*t)), Y: cy + int64(math.Round(dy*t))}
}

func smartArtElbowPath(start, end Point, bendPoint string) []Point {
	dx := end.X - start.X
	dy := end.Y - start.Y
	fraction := 0.5
	switch bendPoint {
	case "beg":
		fraction = 0.25
	case "end":
		fraction = 0.75
	}
	if absInt64(dx) >= absInt64(dy) {
		midX := start.X + int64(math.Round(float64(dx)*fraction))
		return []Point{start, {X: midX, Y: start.Y}, {X: midX, Y: end.Y}, end}
	}
	midY := start.Y + int64(math.Round(float64(dy)*fraction))
	return []Point{start, {X: start.X, Y: midY}, {X: end.X, Y: midY}, end}
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func resolveConnector(cx *parts.Connector, themeColors map[string]string, shapeIndex map[int]*Shape) *Shape {
	shape := &Shape{
		ID:                   cx.NvCxnSpPr.CNvPr.ID,
		OriginalShapeID:      uint32(cx.NvCxnSpPr.CNvPr.ID),
		Name:                 cx.NvCxnSpPr.CNvPr.Name,
		Kind:                 ShapeConnector,
		Bounds:               resolveTransform(cx.SpPr.Xfrm),
		RotationDeg:          shapeRotationDegrees(cx.SpPr.Xfrm),
		ConnectorType:        connectorType(cx.SpPr.PrstGeom),
		ConnectorFlipH:       cx.SpPr.Xfrm != nil && cx.SpPr.Xfrm.FlipH,
		ConnectorFlipV:       cx.SpPr.Xfrm != nil && cx.SpPr.Xfrm.FlipV,
		ConnectorAdjustments: connectorAdjValues(cx.SpPr.PrstGeom),
	}
	if lineStyle := cx.SpPr.Line.DrawingLineStyle(themeColors); lineStyle != nil {
		shape.LineStyle = lineStyle
		shape.LineColor = lineStyle.Color
		shape.LineWidthPt = lineStyle.Width
	}
	if cx.SpPr.Line != nil {
		if cx.SpPr.Line.HeadEnd != nil && cx.SpPr.Line.HeadEnd.Type != "none" {
			shape.ArrowHead = cx.SpPr.Line.HeadEnd.Type
			shape.ArrowHeadWidth = cx.SpPr.Line.HeadEnd.W
			shape.ArrowHeadLength = cx.SpPr.Line.HeadEnd.Len
		}
		if cx.SpPr.Line.TailEnd != nil && cx.SpPr.Line.TailEnd.Type != "none" {
			shape.ArrowTail = cx.SpPr.Line.TailEnd.Type
			shape.ArrowTailWidth = cx.SpPr.Line.TailEnd.W
			shape.ArrowTailLength = cx.SpPr.Line.TailEnd.Len
		}
	}
	if cx.NvCxnSpPr.CNvCxnSpPr != nil {
		shape.ConnectorStart = resolveConnectionPoint(cx.NvCxnSpPr.CNvCxnSpPr.StCxn, shapeIndex)
		shape.ConnectorEnd = resolveConnectionPoint(cx.NvCxnSpPr.CNvCxnSpPr.EndCxn, shapeIndex)
	}
	if shape.LineWidthPt == 0 {
		shape.LineWidthPt = 1.0
	}
	return shape
}

func indexShape(index map[int]*Shape, shape *Shape) {
	if shape == nil {
		return
	}
	if shape.ID != 0 {
		index[shape.ID] = shape
	}
	for _, child := range shape.Children {
		indexShape(index, child)
	}
}

func connectorType(geom *parts.PresetGeometry) string {
	if pg := geom.DrawingPresetGeometry(); pg != nil {
		return pg.Name
	}
	if geom == nil {
		return ""
	}
	return geom.Prst
}

func connectorAdjValues(geom *parts.PresetGeometry) []int {
	if geom == nil {
		return nil
	}
	var vals []int
	for _, guide := range geom.AvLst {
		fields := strings.Fields(guide.Fmla)
		if len(fields) != 2 || fields[0] != "val" {
			continue
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		vals = append(vals, n)
	}
	return vals
}

func solidFillColor(fill drawing.Fill) string {
	switch f := fill.(type) {
	case drawing.SolidFill:
		return f.Color
	case *drawing.SolidFill:
		return f.Color
	case drawing.GradientFill:
		if len(f.Stops) > 0 {
			return f.Stops[0].Color
		}
		return ""
	case *drawing.GradientFill:
		if len(f.Stops) > 0 {
			return f.Stops[0].Color
		}
		return ""
	case drawing.PatternFill:
		if f.Foreground != "" {
			return f.Foreground
		}
		return f.Background
	case *drawing.PatternFill:
		if f.Foreground != "" {
			return f.Foreground
		}
		return f.Background
	case drawing.PictureFill:
		// For picture fills, return empty string (no solid color fallback)
		return ""
	case *drawing.PictureFill:
		// For picture fills, return empty string (no solid color fallback)
		return ""
	default:
		return ""
	}
}

func materializeShapeFill(fill drawing.Fill, mediaParts map[string]media.Part) drawing.Fill {
	if fill == nil || len(mediaParts) == 0 {
		return fill
	}
	switch f := fill.(type) {
	case drawing.PictureFill:
		if f.EmbedID == "" {
			return f
		}
		if part, ok := mediaParts[f.EmbedID]; ok {
			f.ImageData = part.Data
			f.ContentType = part.ContentType
		}
		return f
	case *drawing.PictureFill:
		if f == nil || f.EmbedID == "" {
			return fill
		}
		if part, ok := mediaParts[f.EmbedID]; ok {
			clone := *f
			clone.ImageData = part.Data
			clone.ContentType = part.ContentType
			return &clone
		}
	}
	return fill
}

func resolveConnectionPoint(ref *parts.ConnectionSiteRef, shapeIndex map[int]*Shape) *Point {
	if ref == nil || shapeIndex == nil {
		return nil
	}
	shape := shapeIndex[ref.ID]
	if shape == nil {
		return nil
	}
	pt := shapeConnectionPoint(shape.Bounds, ref.Idx)
	return &pt
}

func shapeConnectionPoint(bounds Rect, idx int) Point {
	switch idx {
	case 0:
		return Point{X: bounds.X + bounds.W/2, Y: bounds.Y}
	case 1:
		return Point{X: bounds.X + bounds.W, Y: bounds.Y + bounds.H/2}
	case 2:
		return Point{X: bounds.X + bounds.W/2, Y: bounds.Y + bounds.H}
	case 3:
		return Point{X: bounds.X, Y: bounds.Y + bounds.H/2}
	default:
		return Point{X: bounds.X + bounds.W/2, Y: bounds.Y + bounds.H/2}
	}
}

func resolveBgColor(bg *parts.SlideBg, themeColors map[string]string) string {
	if bg == nil {
		return ""
	}
	if bg.BgPrSrgb != nil {
		return bg.BgPrSrgb.Val
	}
	if bg.BgPrScheme != nil {
		return themeColor(themeColors, bg.BgPrScheme.Val)
	}
	if bg.BgRef != nil {
		if bg.BgRef.Srgb != nil {
			return bg.BgRef.Srgb.Val
		}
		if bg.BgRef.Scheme != nil {
			return themeColor(themeColors, bg.BgRef.Scheme.Val)
		}
	}
	return ""
}

func resolveBackgroundFill(bg *parts.SlideBg, themeColors map[string]string, rawTheme *parts.Theme, mediaParts map[string]media.Part) drawing.Fill {
	if bg == nil {
		return nil
	}
	if bg.BgPrNoFill != nil {
		return drawing.NoFill{}
	}
	if bg.BgPrSrgb != nil {
		return drawing.SolidFill{Color: bg.BgPrSrgb.Val, Alpha: 1.0}
	}
	if bg.BgPrScheme != nil {
		color := themeColor(themeColors, bg.BgPrScheme.Val)
		return drawing.SolidFill{Color: color, ThemeColor: bg.BgPrScheme.Val, Alpha: 1.0}
	}
	if bg.BgPrGradFill != nil {
		if fill := (&parts.ShapeProperties{GradFill: bg.BgPrGradFill}).DrawingFill(themeColors); fill != nil {
			return fill
		}
	}
	if bg.BgPrPattFill != nil {
		if fill := (&parts.ShapeProperties{PattFill: bg.BgPrPattFill}).DrawingFill(themeColors); fill != nil {
			return fill
		}
	}
	if bg.BgPrBlipFill != nil {
		if fill := (&parts.ShapeProperties{BlipFill: bg.BgPrBlipFill}).DrawingFill(themeColors); fill != nil {
			return materializeShapeFill(fill, mediaParts)
		}
	}
	if bg.BgRef != nil && rawTheme != nil {
		idx := bg.BgRef.Idx - 1001
		if idx >= 0 && idx < len(rawTheme.Elements.FmtScheme.BGFillStyleList) {
			style := &rawTheme.Elements.FmtScheme.BGFillStyleList[idx]
			fill := resolveThemeFormatFillStyle(style, themeColors)
			if pf, ok := fill.(drawing.PictureFill); ok {
				return materializeShapeFill(pf, mediaParts)
			}
			return fill
		}
	}
	return nil
}

func resolveThemeFormatFillStyle(style *parts.ThemeFormatFillStyle, themeColors map[string]string) drawing.Fill {
	if style == nil {
		return nil
	}
	if style.NoFill != nil {
		return drawing.NoFill{}
	}
	if style.SolidFill != nil && style.SolidFill.Val != "" {
		return drawing.SolidFill{Color: style.SolidFill.Val, Alpha: 1.0}
	}
	if style.FillScheme != nil {
		color := themeColor(themeColors, style.FillScheme.Val)
		return drawing.SolidFill{Color: color, ThemeColor: style.FillScheme.Val, Alpha: 1.0}
	}
	if style.GradFill != nil {
		if fill := (&parts.ShapeProperties{GradFill: style.GradFill}).DrawingFill(themeColors); fill != nil {
			return fill
		}
	}
	if style.PattFill != nil {
		if fill := (&parts.ShapeProperties{PattFill: style.PattFill}).DrawingFill(themeColors); fill != nil {
			return fill
		}
	}
	if style.BlipFill != nil {
		if fill := (&parts.ShapeProperties{BlipFill: style.BlipFill}).DrawingFill(themeColors); fill != nil {
			return fill
		}
	}
	return nil
}

const (
	defaultTextInsetLeftRightEMU = 91440
	defaultTextInsetTopBottomEMU = 45720
)

type textBodyStyle struct {
	Anchor                    string
	AnchorCtr                 bool
	Rot                       int
	Wrap                      string
	LIns                      int64
	RIns                      int64
	TIns                      int64
	BIns                      int64
	Autofit                   string
	AutofitFontScale          int
	AutofitLineSpaceReduction int
	Columns                   int
	ColumnSpacingEMU          int64
}

func (style textBodyStyle) drawingProps() drawing.TextBodyProperties {
	return drawing.TextBodyProperties{
		Anchor:                    style.Anchor,
		AnchorCtr:                 style.AnchorCtr,
		Rot:                       style.Rot,
		Wrap:                      style.Wrap,
		AutofitType:               style.Autofit,
		AutofitFontScale:          style.AutofitFontScale,
		AutofitLineSpaceReduction: style.AutofitLineSpaceReduction,
		InsetL:                    style.LIns,
		InsetR:                    style.RIns,
		InsetT:                    style.TIns,
		InsetB:                    style.BIns,
		Columns:                   style.Columns,
		ColumnSpacingEMU:          style.ColumnSpacingEMU,
	}
}

func resolveTextBodyStyle(shapeBody, layoutBody, masterBody *parts.TextBody) textBodyStyle {
	style := textBodyStyle{
		Anchor: "t",
		LIns:   defaultTextInsetLeftRightEMU,
		RIns:   defaultTextInsetLeftRightEMU,
		TIns:   defaultTextInsetTopBottomEMU,
		BIns:   defaultTextInsetTopBottomEMU,
	}
	applyTextBodyStyle(&style, masterBody)
	applyTextBodyStyle(&style, layoutBody)
	applyTextBodyStyle(&style, shapeBody)
	return style
}

func applyTextBodyStyle(style *textBodyStyle, body *parts.TextBody) {
	if style == nil || body == nil || body.BodyPr == nil {
		return
	}
	pr := body.BodyPr
	if pr.Anchor != "" {
		style.Anchor = pr.Anchor
	}
	if pr.AnchorCtr != nil {
		style.AnchorCtr = *pr.AnchorCtr
	}
	if pr.Rot != 0 {
		style.Rot = pr.Rot
	}
	if pr.Wrap != "" {
		style.Wrap = pr.Wrap
	}
	if pr.LIns != nil {
		style.LIns = *pr.LIns
	}
	if pr.RIns != nil {
		style.RIns = *pr.RIns
	}
	if pr.TIns != nil {
		style.TIns = *pr.TIns
	}
	if pr.BIns != nil {
		style.BIns = *pr.BIns
	}
	switch {
	case pr.NoAutofit != nil:
		style.Autofit = "noAutofit"
		style.AutofitFontScale = 0
		style.AutofitLineSpaceReduction = 0
	case pr.SpAutoFit != nil:
		style.Autofit = "spAutoFit"
		style.AutofitFontScale = 0
		style.AutofitLineSpaceReduction = 0
	case pr.NormAutofit != nil:
		style.Autofit = "normAutofit"
		if pr.NormAutofit.FontScale != nil {
			style.AutofitFontScale = *pr.NormAutofit.FontScale
		} else {
			style.AutofitFontScale = 0
		}
		if pr.NormAutofit.LnSpcReduction != nil {
			style.AutofitLineSpaceReduction = *pr.NormAutofit.LnSpcReduction
		} else {
			style.AutofitLineSpaceReduction = 0
		}
	}
	if pr.NumCol != nil && *pr.NumCol > 1 {
		style.Columns = *pr.NumCol
		if pr.SpcCol != nil {
			style.ColumnSpacingEMU = *pr.SpcCol
		}
	}
}

func effectiveShapeXfrm(sp *parts.Shape, layoutShape, masterShape *parts.Shape) *parts.Transform2D {
	if sp.SpPr.Xfrm != nil {
		return sp.SpPr.Xfrm
	}
	if layoutShape != nil && layoutShape.SpPr.Xfrm != nil {
		return layoutShape.SpPr.Xfrm
	}
	if masterShape != nil && masterShape.SpPr.Xfrm != nil {
		return masterShape.SpPr.Xfrm
	}
	return nil
}

func effectiveShapeFill(sp *parts.Shape, layoutShape, masterShape *parts.Shape, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string, mediaParts, layoutMediaParts, masterMediaParts map[string]media.Part) drawing.Fill {
	if fill := sp.SpPr.DrawingFill(slideThemeColors); fill != nil {
		return materializeShapeFill(fill, mediaParts)
	}
	if layoutShape != nil {
		if fill := layoutShape.SpPr.DrawingFill(layoutThemeColors); fill != nil {
			return materializeShapeFill(fill, layoutMediaParts)
		}
	}
	if masterShape != nil {
		if fill := masterShape.SpPr.DrawingFill(masterThemeColors); fill != nil {
			return materializeShapeFill(fill, masterMediaParts)
		}
	}
	return nil
}

func effectiveShapeLine(sp *parts.Shape, layoutShape, masterShape *parts.Shape, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string) *drawing.LineStyle {
	// If the shape has an explicit line element, use it (including NoFill).
	if sp.SpPr.Line != nil {
		return sp.SpPr.Line.DrawingLineStyle(slideThemeColors)
	}
	// Shape has no line element at all — inherit from layout/master.
	if layoutShape != nil && layoutShape.SpPr.Line != nil {
		return layoutShape.SpPr.Line.DrawingLineStyle(layoutThemeColors)
	}
	if masterShape != nil && masterShape.SpPr.Line != nil {
		return masterShape.SpPr.Line.DrawingLineStyle(masterThemeColors)
	}
	return nil
}

func effectiveShapeEffectList(sp *parts.Shape, layoutShape, masterShape *parts.Shape, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string) drawing.EffectList {
	effects := sp.SpPr.DrawingEffectList(slideThemeColors)
	if len(effects.Effects) > 0 {
		return effects
	}
	if layoutShape != nil {
		effects = layoutShape.SpPr.DrawingEffectList(layoutThemeColors)
		if len(effects.Effects) > 0 {
			return effects
		}
	}
	if masterShape != nil {
		effects = masterShape.SpPr.DrawingEffectList(masterThemeColors)
		if len(effects.Effects) > 0 {
			return effects
		}
	}
	return drawing.EffectList{}
}

func resolveShape(sp *parts.Shape, layoutShape, masterShape *parts.Shape, master *parts.SlideMaster, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string, theme *Theme, rawTheme *parts.Theme, mediaParts, layoutMediaParts, masterMediaParts map[string]media.Part, hlinkURLs map[string]string) *Shape {
	ph := resolvePlaceholderKind(sp.NvSpPr.NvPr.Ph)
	xfrm := effectiveShapeXfrm(sp, layoutShape, masterShape)
	shape := &Shape{
		ID:              sp.NvSpPr.CNvPr.ID,
		OriginalShapeID: uint32(sp.NvSpPr.CNvPr.ID),
		Name:            sp.NvSpPr.CNvPr.Name,
		Kind:            ShapeText,
		Placeholder:     ph,
		Bounds:          resolveTransform(xfrm),
		RotationDeg:     shapeRotationDegrees(xfrm),
		FlipH:           xfrm != nil && xfrm.FlipH,
		FlipV:           xfrm != nil && xfrm.FlipV,
	}
	if phInfo := sp.NvSpPr.NvPr.Ph; phInfo != nil {
		shape.PlaceholderIdx = phInfo.Idx
		shape.PlaceholderOrient = phInfo.Orient
		shape.PlaceholderSize = phInfo.Size
		if phInfo.HasCustomPrompt != nil {
			shape.PlaceholderHasCustomPrompt = *phInfo.HasCustomPrompt
		}
	}
	if geom := sp.SpPr.DrawingGeometry(); geom != nil {
		if geom.Type == drawing.GeometryTypeCustom && len(geom.Paths) > 0 {
			shape.GeometryData = geom
		} else if pg := sp.SpPr.PrstGeom.DrawingPresetGeometry(); pg != nil && pg.Name != "" && pg.Name != "rect" {
			shape.Geometry = pg.Name
		}
	}
	if fill := effectiveShapeFill(sp, layoutShape, masterShape, slideThemeColors, layoutThemeColors, masterThemeColors, mediaParts, layoutMediaParts, masterMediaParts); fill != nil {
		shape.Fill = fill
		shape.FillColor = solidFillColor(fill)
		shape.PictureTile = effectiveShapePictureTile(sp, layoutShape, masterShape, slideThemeColors, layoutThemeColors, masterThemeColors)
	} else if rawTheme != nil {
		if styleFill := resolveShapeStyleFill(sp, layoutShape, masterShape, rawTheme, slideThemeColors, layoutThemeColors, masterThemeColors); styleFill != nil {
			shape.Fill = styleFill
			shape.FillColor = solidFillColor(styleFill)
		}
	}
	shape.Effects = effectiveShapeEffectList(sp, layoutShape, masterShape, slideThemeColors, layoutThemeColors, masterThemeColors)
	if len(shape.Effects.Effects) == 0 && rawTheme != nil {
		shape.Effects = resolveShapeStyleEffect(sp, layoutShape, masterShape, rawTheme, slideThemeColors, layoutThemeColors, masterThemeColors)
	}
	if lineStyle := effectiveShapeLine(sp, layoutShape, masterShape, slideThemeColors, layoutThemeColors, masterThemeColors); lineStyle != nil {
		shape.LineStyle = lineStyle
		shape.LineColor = lineStyle.Color
		shape.LineWidthPt = lineStyle.Width
	} else if rawTheme != nil {
		if styleLine := resolveShapeStyleLine(sp, layoutShape, masterShape, rawTheme, slideThemeColors, layoutThemeColors, masterThemeColors); styleLine != nil {
			shape.LineStyle = styleLine
			shape.LineColor = styleLine.Color
			shape.LineWidthPt = styleLine.Width
		}
	}
	if sp.TxBody != nil {
		bodyStyle := resolveTextBodyStyle(sp.TxBody, layoutTextBody(layoutShape), layoutTextBody(masterShape))
		shape.TextAnchor = bodyStyle.Anchor
		shape.TextInsetLeftEMU = bodyStyle.LIns
		shape.TextInsetRightEMU = bodyStyle.RIns
		shape.TextInsetTopEMU = bodyStyle.TIns
		shape.TextInsetBottomEMU = bodyStyle.BIns
		shape.TextAllowOverflow = bodyStyle.Autofit == "noAutofit"
		shape.TextAutoGrow = bodyStyle.Autofit == "spAutoFit"
		shape.SentinelPlaceholder = placeholderIdxIsSentinel(sp.NvSpPr.NvPr.Ph)
		shape.TextBodyProps = bodyStyle.drawingProps()

		for _, ap := range sp.TxBody.Paragraphs {
			level := 0
			if ap.Properties != nil {
				level = ap.Properties.Lvl
			}
			defaults := resolveTextDefaults(sp.TxBody, layoutTextBody(layoutShape), layoutTextBody(masterShape), master, ph, level, slideThemeColors, layoutThemeColors, masterThemeColors, theme)
			if defaults.FontFamily == "" && theme != nil {
				if family := resolveShapeStyleFontRef(sp, layoutShape, masterShape, theme); family != "" {
					defaults.FontFamily = family
				}
			}
			shape.Paragraphs = append(shape.Paragraphs, resolveParagraph(&ap, defaults, slideThemeColors, theme, hlinkURLs))
		}
	}
	ensureAutoSlideNumberField(shape, sp, layoutShape, masterShape, master, slideThemeColors, layoutThemeColors, masterThemeColors, theme)
	if sp.NvSpPr.CNvPr.HlinkClick != nil {
		shape.HyperlinkURL, shape.ActionLink = resolveHlinkClick(sp.NvSpPr.CNvPr.HlinkClick, hlinkURLs)
	}
	return shape
}

func syncPresentationOfficeArtShape(shape *Shape) *officeartir.Shape {
	if shape == nil {
		return nil
	}
	spec := officeartdml.ShapeSpec{
		ID:          uint32(shape.ID),
		Name:        shape.Name,
		Kind:        presentationOfficeArtKind(shape.Kind),
		Bounds:      officeartir.Rect{X: shape.Bounds.X, Y: shape.Bounds.Y, W: shape.Bounds.W, H: shape.Bounds.H},
		RotationDeg: shape.RotationDeg,
		FlipH:       shape.FlipH,
		FlipV:       shape.FlipV,
		Geometry:    shape.GeometryData,
		Fill:        shape.Fill,
		Line:        shape.LineStyle,
		Effects:     shape.Effects,
		Source:      officeartir.SourceRef{Format: "pptx", ShapeID: shape.OriginalShapeID},
	}
	if shape.Geometry != "" {
		spec.Preset = &drawing.PresetGeometry{
			Name:        shape.Geometry,
			Adjustments: append([]float64(nil), shape.GeometryAdjustments...),
		}
	}
	if shape.Kind == ShapeImage && len(shape.ImageData) > 0 {
		spec.Image = &drawing.PictureFill{
			EmbedID:     shape.ImageRelID,
			ImageData:   append([]byte(nil), shape.ImageData...),
			ContentType: shape.ImageMIME,
			DPI:         shape.ImageDPI,
			Crop: &drawing.Rect{
				X:      shape.ImageCropLeft,
				Y:      shape.ImageCropTop,
				Width:  1 - shape.ImageCropLeft - shape.ImageCropRight,
				Height: 1 - shape.ImageCropTop - shape.ImageCropBottom,
			},
		}
	}
	if shape.Kind == ShapeConnector {
		spec.Connector = &officeartdml.ConnectorSpec{Type: shape.Geometry}
	}
	shape.OfficeArt = officeartdml.Shape(spec)

	if len(shape.Children) > 0 {
		shape.OfficeArt.Children = shape.OfficeArt.Children[:0]
		for _, child := range shape.Children {
			if composed := syncPresentationOfficeArtShape(child); composed != nil {
				shape.OfficeArt.Children = append(shape.OfficeArt.Children, composed)
			}
		}
		if len(shape.OfficeArt.Children) > 0 {
			shape.OfficeArt.Kind = officeartir.ShapeGroup
		}
	}
	return shape.OfficeArt
}

func presentationOfficeArtKind(kind ShapeKind) officeartir.ShapeKind {
	switch kind {
	case ShapeImage:
		return officeartir.ShapePicture
	case ShapeGroup:
		return officeartir.ShapeGroup
	case ShapeConnector, ShapeSmartArtConnector:
		return officeartir.ShapeConnector
	case ShapeText:
		return officeartir.ShapeTextBox
	default:
		return officeartir.ShapeAuto
	}
}

func layoutTextBody(shape *parts.Shape) *parts.TextBody {
	if shape == nil {
		return nil
	}
	return shape.TxBody
}

func resolvePicture(pic *parts.Picture, mediaParts map[string]media.Part, themeColors map[string]string, hlinkURLs map[string]string) *Shape {
	shape := &Shape{
		ID:              pic.NvPicPr.CNvPr.ID,
		OriginalShapeID: uint32(pic.NvPicPr.CNvPr.ID),
		Name:            pic.NvPicPr.CNvPr.Name,
		Bounds:          resolveTransform(pic.SpPr.Xfrm),
		RotationDeg:     shapeRotationDegrees(pic.SpPr.Xfrm),
		FlipH:           pic.SpPr.Xfrm != nil && pic.SpPr.Xfrm.FlipH,
		FlipV:           pic.SpPr.Xfrm != nil && pic.SpPr.Xfrm.FlipV,
	}
	shape.ImageDPI = pic.BlipFill.Dpi
	if pic.BlipFill.RotWithShape != nil {
		shape.ImageRotWithShape = *pic.BlipFill.RotWithShape
		shape.ImageRotWithShapeSet = true
	}
	if pic.BlipFill.SrcRect != nil {
		shape.ImageCropLeft = clampCropPct(pic.BlipFill.SrcRect.L)
		shape.ImageCropTop = clampCropPct(pic.BlipFill.SrcRect.T)
		shape.ImageCropRight = clampCropPct(pic.BlipFill.SrcRect.R)
		shape.ImageCropBottom = clampCropPct(pic.BlipFill.SrcRect.B)
	}
	if pic.BlipFill.Stretch != nil && pic.BlipFill.Stretch.FillRect != nil {
		shape.ImageFillLeft = clampCropPct(pic.BlipFill.Stretch.FillRect.L)
		shape.ImageFillTop = clampCropPct(pic.BlipFill.Stretch.FillRect.T)
		shape.ImageFillRight = clampCropPct(pic.BlipFill.Stretch.FillRect.R)
		shape.ImageFillBottom = clampCropPct(pic.BlipFill.Stretch.FillRect.B)
	}
	if effects := pic.SpPr.DrawingEffectList(themeColors); len(effects.Effects) > 0 {
		shape.Effects = effects
	}
	if mediaRef := pic.NvPicPr.NvPr.PrimaryMedia(); mediaRef != nil {
		shape.Kind = ShapeMedia
		shape.MediaRelID = mediaRef.RelID()
		shape.MediaKind = pic.NvPicPr.NvPr.MediaKind()
		if m, ok := mediaParts[shape.MediaRelID]; ok {
			shape.MediaData = m.Data
			shape.MediaMIME = m.ContentType
			if shape.MediaKind == "" {
				shape.MediaKind = inferMediaKind(m.ContentType)
			}
		}
		if pic.BlipFill.Blip != nil {
			relID := pic.BlipFill.Blip.Embed
			shape.ImageRelID = relID
			if m, ok := mediaParts[relID]; ok {
				shape.MediaPosterData = m.Data
				shape.MediaPosterMIME = m.ContentType
			}
		}
	} else {
		shape.Kind = ShapeImage
	}
	if pic.BlipFill.Blip != nil {
		relID := pic.BlipFill.Blip.Embed
		if shape.Kind == ShapeImage {
			shape.ImageRelID = relID
			shape.ImageEffects = pic.BlipFill.Blip.DrawingEffects(themeColors)
			if m, ok := mediaParts[relID]; ok {
				shape.ImageData = m.Data
				shape.ImageMIME = m.ContentType
				applyIntrinsicPictureSize(shape)
			}
		}
	}
	if pic.NvPicPr.CNvPr.HlinkClick != nil {
		shape.HyperlinkURL, shape.ActionLink = resolveHlinkClick(pic.NvPicPr.CNvPr.HlinkClick, hlinkURLs)
	}
	return shape
}

func applyIntrinsicPictureSize(shape *Shape) {
	if shape == nil || len(shape.ImageData) == 0 || (shape.Bounds.W > 0 && shape.Bounds.H > 0) {
		return
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(shape.ImageData))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return
	}
	dpi := shape.ImageDPI
	if dpi <= 0 {
		dpi = 96
	}
	w := int64(math.Round(float64(cfg.Width) * 914400.0 / float64(dpi)))
	h := int64(math.Round(float64(cfg.Height) * 914400.0 / float64(dpi)))
	if shape.Bounds.W <= 0 && shape.Bounds.H <= 0 {
		shape.Bounds.W, shape.Bounds.H = w, h
		return
	}
	if shape.Bounds.W <= 0 && shape.Bounds.H > 0 {
		shape.Bounds.W = int64(math.Round(float64(shape.Bounds.H) * float64(cfg.Width) / float64(cfg.Height)))
	}
	if shape.Bounds.H <= 0 && shape.Bounds.W > 0 {
		shape.Bounds.H = int64(math.Round(float64(shape.Bounds.W) * float64(cfg.Height) / float64(cfg.Width)))
	}
}

func inferMediaKind(contentType string) string {
	baseType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch {
	case strings.HasPrefix(baseType, "video/"):
		return "video"
	case strings.HasPrefix(baseType, "audio/"):
		return "audio"
	default:
		return ""
	}
}

func clampCropPct(value int) float64 {
	if value <= 0 {
		return 0
	}
	if value >= 100000 {
		return 1
	}
	return float64(value) / 100000.0
}

// resolveHlinkClick resolves a hlinkClick reference to either an external URL
// or an action link string. It returns (hyperlinkURL, actionLink) — at most one
// of the two will be non-empty.
//
// ppaction:// URIs are parsed for well-known navigation targets:
//   - ppaction://hlinksldjump?slidenum=N  → actionLink "slideN" (1-based)
//   - ppaction://hlinkPresentationJump?jump=firstSlide → actionLink "firstSlide"
//   - ppaction://hlinkPresentationJump?jump=lastSlide  → actionLink "lastSlide"
//   - ppaction://hlinkPresentationJump?jump=nextSlide  → actionLink "nextSlide"
//   - ppaction://hlinkPresentationJump?jump=prevSlide  → actionLink "prevSlide"
//   - ppaction://hlinkshowjump?jump=endShow           → actionLink "endShow"
//
// External URLs are looked up in hlinkURLs by RelID.
func resolveHlinkClick(h *parts.HlinkClick, hlinkURLs map[string]string) (hyperlinkURL string, actionLink string) {
	if h == nil {
		return "", ""
	}
	if strings.HasPrefix(h.Action, "ppaction://") {
		action := h.Action[len("ppaction://"):]
		// Strip query and parse
		base, query, _ := strings.Cut(action, "?")
		switch base {
		case "hlinksldjump":
			params := parseSimpleQuery(query)
			if n := params["slidenum"]; n != "" {
				return "", "slide" + n
			}
		case "hlinkPresentationJump":
			params := parseSimpleQuery(query)
			if j := params["jump"]; j != "" {
				return "", j // "firstSlide","lastSlide","nextSlide","prevSlide"
			}
		case "hlinkshowjump":
			params := parseSimpleQuery(query)
			if j := params["jump"]; j != "" {
				return "", j // "endShow"
			}
		}
		return "", ""
	}
	if h.RelID != "" && hlinkURLs != nil {
		if url, ok := hlinkURLs[h.RelID]; ok {
			return url, ""
		}
	}
	return "", ""
}

// parseSimpleQuery parses a simple key=value&key=value query string.
func parseSimpleQuery(query string) map[string]string {
	m := map[string]string{}
	for _, pair := range strings.Split(query, "&") {
		k, v, ok := strings.Cut(pair, "=")
		if ok {
			m[k] = v
		}
	}
	return m
}

func resolveGroupShape(grp *parts.GroupShape, master *parts.SlideMaster, themeColors map[string]string, theme *Theme, rawTheme *parts.Theme, mediaParts, oleObjectParts map[string]media.Part, hlinkURLs map[string]string, tableStyles *parts.TableStyleList) *Shape {
	shape := &Shape{Kind: ShapeGroup}
	if grp.NvGrpSpPr != nil {
		shape.ID = grp.NvGrpSpPr.CNvPr.ID
		shape.OriginalShapeID = uint32(grp.NvGrpSpPr.CNvPr.ID)
		shape.Name = grp.NvGrpSpPr.CNvPr.Name
	}

	// Get group transform if present
	var groupXfrm *parts.Transform2D
	if grp.GrpSpPr != nil {
		groupXfrm = grp.GrpSpPr.Xfrm
	}

	for _, sp := range grp.Shapes {
		child := resolveShape(&sp, nil, nil, master, themeColors, themeColors, themeColors, theme, rawTheme, mediaParts, nil, nil, hlinkURLs)
		if child != nil {
			applyGroupTransformToShape(child, groupXfrm)
			shape.Children = append(shape.Children, child)
		}
	}
	for _, pic := range grp.Pictures {
		child := resolvePicture(&pic, mediaParts, themeColors, hlinkURLs)
		if child != nil {
			applyGroupTransformToShape(child, groupXfrm)
			shape.Children = append(shape.Children, child)
		}
	}
	for _, g := range grp.GroupShapes {
		child := resolveGroupShape(&g, master, themeColors, theme, rawTheme, mediaParts, oleObjectParts, hlinkURLs, tableStyles)
		if child != nil {
			applyGroupTransformToShape(child, groupXfrm)
			shape.Children = append(shape.Children, child)
		}
	}
	for _, tbl := range grp.Tables {
		child := resolveTableFrame(&tbl, themeColors, theme, rawTheme, tableStyles)
		if child != nil {
			applyGroupTransformToShape(child, groupXfrm)
			shape.Children = append(shape.Children, child)
		}
	}
	for i := range grp.Connectors {
		child := resolveConnector(&grp.Connectors[i], themeColors, nil)
		if child != nil {
			applyGroupTransformToShape(child, groupXfrm)
			shape.Children = append(shape.Children, child)
		}
	}
	for i := range grp.OleObjectFrames {
		child := resolveOleObjectFrame(&grp.OleObjectFrames[i], &parts.SlideEntry{OleObjectParts: oleObjectParts, MediaParts: mediaParts})
		if child != nil {
			applyGroupTransformToShape(child, groupXfrm)
			shape.Children = append(shape.Children, child)
		}
	}
	return shape
}

func resolveTableFrame(frame *parts.TableFrame, themeColors map[string]string, theme *Theme, rawTheme *parts.Theme, tableStyles *parts.TableStyleList) *Shape {
	if frame == nil || frame.Table == nil {
		return nil
	}
	style := effectiveTableStyle(frame.Table, tableStyles)
	totalRows := len(frame.Table.Rows)
	totalCols := tableColumnCount(frame.Table)
	shape := &Shape{
		ID:              frame.ID,
		OriginalShapeID: uint32(frame.ID),
		Name:            frame.Name,
		Kind:            ShapeTable,
		Bounds:          resolveTransform(frame.Xfrm),
		Table:           &Table{},
	}
	for _, col := range frame.Table.Grid.Columns {
		shape.Table.ColumnWidthsEMU = append(shape.Table.ColumnWidthsEMU, col.W)
	}
	for _, row := range frame.Table.Rows {
		resolvedRow := &TableRow{HeightEMU: row.H}
		colIndex := 0
		for _, cell := range row.Cells {
			startCol := colIndex
			span := cell.GridSpan
			if span <= 0 {
				span = 1
			}
			if cell.HMerge || cell.VMerge {
				span = 1
			}
			colIndex += span

			styleDefaults := resolveTableCellStyleDefaults(style, frame.Table.Props, len(shape.Table.Rows), startCol, span, totalRows, totalCols, themeColors, theme, rawTheme)
			resolvedCell := &TableCell{
				GridSpan:     cell.GridSpan,
				RowSpan:      cell.RowSpan,
				HMerge:       cell.HMerge,
				VMerge:       cell.VMerge,
				Fill:         styleDefaults.Fill,
				FillColor:    styleDefaults.FillColor,
				Effects:      styleDefaults.Effects,
				BorderLeft:   styleDefaults.BorderLeft,
				BorderRight:  styleDefaults.BorderRight,
				BorderTop:    styleDefaults.BorderTop,
				BorderBottom: styleDefaults.BorderBottom,
			}
			if resolvedCell.GridSpan <= 0 {
				resolvedCell.GridSpan = 1
			}
			if resolvedCell.RowSpan <= 0 {
				resolvedCell.RowSpan = 1
			}
			if cell.Props != nil {
				resolvedCell.TextAnchor = cell.Props.Anchor
				resolvedCell.MarginLeftEMU = cell.Props.MarL
				resolvedCell.MarginRightEMU = cell.Props.MarR
				resolvedCell.MarginTopEMU = cell.Props.MarT
				resolvedCell.MarginBottomEMU = cell.Props.MarB
				if fill := resolveTableCellFill(cell.Props.SolidFill, cell.Props.SchemeFill, cell.Props.GradFill, cell.Props.PattFill, cell.Props.BlipFill, cell.Props.NoFill, themeColors); fill != nil {
					resolvedCell.Fill = fill
					resolvedCell.FillColor = solidFillColor(fill)
				}
				if cell.Props.LnL != nil {
					resolvedCell.BorderLeft = resolveTableLineStyle(cell.Props.LnL, themeColors)
				}
				if cell.Props.LnR != nil {
					resolvedCell.BorderRight = resolveTableLineStyle(cell.Props.LnR, themeColors)
				}
				if cell.Props.LnT != nil {
					resolvedCell.BorderTop = resolveTableLineStyle(cell.Props.LnT, themeColors)
				}
				if cell.Props.LnB != nil {
					resolvedCell.BorderBottom = resolveTableLineStyle(cell.Props.LnB, themeColors)
				}
			}
			if cell.TextBody != nil {
				for _, ap := range cell.TextBody.Paragraphs {
					level := 0
					if ap.Properties != nil {
						level = ap.Properties.Lvl
					}
					directDefaults := resolveTextDefaults(cell.TextBody, nil, nil, nil, PlaceholderNone, level, themeColors, nil, nil, theme)
					defaults := mergeTextDefaults(directDefaults, styleDefaults.Text)
					resolvedCell.Paragraphs = append(resolvedCell.Paragraphs, resolveParagraph(&ap, defaults, themeColors, theme, nil))
				}
			}
			resolvedRow.Cells = append(resolvedRow.Cells, resolvedCell)
		}
		shape.Table.Rows = append(shape.Table.Rows, resolvedRow)
	}
	return shape
}

type tableCellResolvedStyle struct {
	Fill         drawing.Fill
	FillColor    string
	Effects      drawing.EffectList
	BorderLeft   *drawing.LineStyle
	BorderRight  *drawing.LineStyle
	BorderTop    *drawing.LineStyle
	BorderBottom *drawing.LineStyle
	InsideH      *drawing.LineStyle
	InsideV      *drawing.LineStyle
	Text         textDefaults
}

func effectiveTableStyle(table *parts.PresentationTable, styles *parts.TableStyleList) *parts.DrawingTableStyle {
	if table == nil || table.Props == nil {
		return nil
	}
	if table.Props.TableStyle != nil {
		return table.Props.TableStyle
	}
	if styles == nil {
		return nil
	}
	styleID := strings.TrimSpace(table.Props.TableStyleID)
	if styleID == "" {
		styleID = strings.TrimSpace(styles.Default)
	}
	if styleID == "" {
		return nil
	}
	for i := range styles.Styles {
		if strings.EqualFold(strings.TrimSpace(styles.Styles[i].StyleID), styleID) {
			return &styles.Styles[i]
		}
	}
	return nil
}

func tableColumnCount(table *parts.PresentationTable) int {
	if table == nil {
		return 0
	}
	if n := len(table.Grid.Columns); n > 0 {
		return n
	}
	maxCols := 0
	for _, row := range table.Rows {
		colCount := 0
		for _, cell := range row.Cells {
			span := cell.GridSpan
			if span <= 0 || cell.HMerge || cell.VMerge {
				span = 1
			}
			colCount += span
		}
		if colCount > maxCols {
			maxCols = colCount
		}
	}
	return maxCols
}

func resolveTableCellStyleDefaults(style *parts.DrawingTableStyle, props *parts.PresentationTableProps, rowIdx, colIdx, span, totalRows, totalCols int, themeColors map[string]string, theme *Theme, rawTheme *parts.Theme) tableCellResolvedStyle {
	var resolved tableCellResolvedStyle
	apply := func(region *parts.TableStyleRegion) {
		resolved = mergeResolvedTableCellStyle(resolved, region, rowIdx, colIdx, span, totalRows, totalCols, themeColors, theme, rawTheme)
	}
	if style == nil {
		return resolved
	}
	apply(style.WholeTbl)
	if props != nil && props.BandRow {
		if index, ok := bandedRegionIndex(rowIdx, totalRows, props.FirstRow, props.LastRow); ok {
			if index%2 == 0 {
				apply(style.Band1H)
			} else {
				apply(style.Band2H)
			}
		}
	}
	if props != nil && props.BandCol {
		if index, ok := bandedRegionIndex(colIdx, totalCols, props.FirstCol, props.LastCol); ok {
			if index%2 == 0 {
				apply(style.Band1V)
			} else {
				apply(style.Band2V)
			}
		}
	}
	lastColIdx := colIdx + span - 1
	if props != nil && props.FirstRow && rowIdx == 0 {
		apply(style.FirstRow)
	}
	if props != nil && props.LastRow && rowIdx == totalRows-1 {
		apply(style.LastRow)
	}
	if props != nil && props.FirstCol && colIdx == 0 {
		apply(style.FirstCol)
	}
	if props != nil && props.LastCol && lastColIdx == totalCols-1 {
		apply(style.LastCol)
	}
	if props != nil && props.FirstRow && props.FirstCol && rowIdx == 0 && colIdx == 0 {
		apply(style.NWCell)
	}
	if props != nil && props.FirstRow && props.LastCol && rowIdx == 0 && lastColIdx == totalCols-1 {
		apply(style.NECell)
	}
	if props != nil && props.LastRow && props.FirstCol && rowIdx == totalRows-1 && colIdx == 0 {
		apply(style.SWCell)
	}
	if props != nil && props.LastRow && props.LastCol && rowIdx == totalRows-1 && lastColIdx == totalCols-1 {
		apply(style.SECell)
	}
	applyInsideBorders(&resolved, rowIdx, colIdx, span, totalRows, totalCols)
	return resolved
}

func mergeResolvedTableCellStyle(dst tableCellResolvedStyle, region *parts.TableStyleRegion, rowIdx, colIdx, span, totalRows, totalCols int, themeColors map[string]string, theme *Theme, rawTheme *parts.Theme) tableCellResolvedStyle {
	if region == nil {
		return dst
	}
	if region.CellStyle != nil {
		if fill := resolveTableCellFill(region.CellStyle.FillColor, region.CellStyle.FillScheme, region.CellStyle.GradFill, region.CellStyle.PattFill, region.CellStyle.BlipFill, region.CellStyle.NoFill, themeColors); fill != nil {
			dst.Fill = fill
			dst.FillColor = solidFillColor(fill)
		} else if fill := resolveTableFillRef(region.CellStyle.FillRef, rawTheme, themeColors); fill != nil {
			dst.Fill = fill
			dst.FillColor = solidFillColor(fill)
		}
		if effects := resolveTableEffectRef(region.CellStyle.EffectRef, rawTheme, themeColors); len(effects.Effects) > 0 {
			dst.Effects = effects
		}
		if region.CellStyle.Borders != nil {
			mergeLine := func(current **drawing.LineStyle, border *parts.DrawingTableBorder) {
				if border == nil {
					return
				}
				if style := resolveTableBorderStyle(border, rawTheme, themeColors); style != nil {
					*current = style
				}
			}
			mergeLine(&dst.BorderLeft, region.CellStyle.Borders.Left)
			mergeLine(&dst.BorderRight, region.CellStyle.Borders.Right)
			mergeLine(&dst.BorderTop, region.CellStyle.Borders.Top)
			mergeLine(&dst.BorderBottom, region.CellStyle.Borders.Bottom)
			mergeLine(&dst.InsideH, region.CellStyle.Borders.InsideH)
			mergeLine(&dst.InsideV, region.CellStyle.Borders.InsideV)
		}
	}
	if region.TextStyle != nil {
		dst.Text = mergeTextDefaults(resolveTableTextDefaults(region.TextStyle, themeColors, theme), dst.Text)
	}
	return dst
}

func applyInsideBorders(style *tableCellResolvedStyle, rowIdx, colIdx, span, totalRows, totalCols int) {
	if style == nil {
		return
	}
	lastColIdx := colIdx + span - 1
	if style.BorderTop == nil && style.InsideH != nil && rowIdx > 0 {
		style.BorderTop = style.InsideH
	}
	if style.BorderBottom == nil && style.InsideH != nil && rowIdx < totalRows-1 {
		style.BorderBottom = style.InsideH
	}
	if style.BorderLeft == nil && style.InsideV != nil && colIdx > 0 {
		style.BorderLeft = style.InsideV
	}
	if style.BorderRight == nil && style.InsideV != nil && lastColIdx < totalCols-1 {
		style.BorderRight = style.InsideV
	}
}

func bandedRegionIndex(index, total int, firstOn, lastOn bool) (int, bool) {
	start := 0
	end := total - 1
	if firstOn {
		start++
	}
	if lastOn {
		end--
	}
	if index < start || index > end {
		return 0, false
	}
	return index - start, true
}

func resolveTableTextDefaults(style *parts.DrawingTableCellTextStyle, themeColors map[string]string, theme *Theme) textDefaults {
	if style == nil {
		return textDefaults{}
	}
	defaults := textDefaults{}
	if value, ok := parseOnOffStyle(style.Bold); ok {
		defaults.Bold = &value
	}
	if value, ok := parseOnOffStyle(style.Italic); ok {
		defaults.Italic = &value
	}
	if style.Color != nil && style.Color.Val != "" {
		defaults.Color = style.Color.Val
	} else if color := resolveTableSchemeColor(style.SchemeClr, themeColors); color != "" {
		defaults.Color = color
	} else if style.FontRef != nil {
		if color := resolveTableColor(style.FontRef.Color, style.FontRef.SchemeClr, themeColors); color != "" {
			defaults.Color = color
		}
	}
	if style.FontRef != nil {
		switch strings.TrimSpace(strings.ToLower(style.FontRef.Idx)) {
		case "major":
			defaults.FontFamily = fallbackThemeFont(theme, PlaceholderTitle)
		case "minor":
			defaults.FontFamily = fallbackThemeFont(theme, PlaceholderBody)
		}
	}
	return defaults
}

func parseOnOffStyle(value string) (bool, bool) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "1", "true", "on":
		return true, true
	case "0", "false", "off":
		return false, true
	default:
		return false, false
	}
}

func resolveTableColor(srgb *parts.ColorRef, scheme *drawingxml.SchemeClr, themeColors map[string]string) string {
	if srgb != nil && srgb.Val != "" {
		return srgb.Val
	}
	return resolveTableSchemeColor(scheme, themeColors)
}

func resolveTableLineStyle(line *parts.ConnectorLine, themeColors map[string]string) *drawing.LineStyle {
	if line == nil {
		return nil
	}
	style := line.DrawingLineStyle(themeColors)
	if style == nil || line.SolidFill == nil || line.SolidFill.SchemeClr == nil {
		return style
	}
	if color := resolveTableSchemeColor(line.SolidFill.SchemeClr, themeColors); color != "" {
		style.Color = color
	}
	return style
}

func resolveTableCellFill(fillColor *parts.ColorRef, fillScheme *drawingxml.SchemeClr, gradFill *parts.GradFill, pattFill *parts.PattFill, blipFill *parts.BlipFill, noFill *struct{}, themeColors map[string]string) drawing.Fill {
	if noFill != nil {
		return drawing.NoFill{}
	}
	if fillColor != nil && fillColor.Val != "" {
		return drawing.SolidFill{Color: fillColor.Val, Alpha: 1.0}
	}
	if fillScheme != nil {
		color := resolveTableSchemeColor(fillScheme, themeColors)
		if color != "" || strings.TrimSpace(fillScheme.Val) != "" {
			return drawing.SolidFill{Color: color, ThemeColor: fillScheme.Val, Alpha: 1.0}
		}
	}
	if gradFill != nil {
		return (&parts.ShapeProperties{GradFill: gradFill}).DrawingFill(themeColors)
	}
	if pattFill != nil {
		return (&parts.ShapeProperties{PattFill: pattFill}).DrawingFill(themeColors)
	}
	if blipFill != nil {
		return (&parts.ShapeProperties{BlipFill: blipFill}).DrawingFill(themeColors)
	}
	return nil
}

func resolveTableBorderStyle(border *parts.DrawingTableBorder, rawTheme *parts.Theme, themeColors map[string]string) *drawing.LineStyle {
	if border == nil {
		return nil
	}
	if border.Line != nil {
		return resolveTableLineStyle(border.Line, themeColors)
	}
	return resolveTableLineRefStyle(border.LineRef, rawTheme, themeColors)
}

func resolveTableLineRefStyle(lineRef *parts.DrawingTableLineRef, rawTheme *parts.Theme, themeColors map[string]string) *drawing.LineStyle {
	if lineRef == nil || rawTheme == nil || lineRef.Idx < 1 || lineRef.Idx > len(rawTheme.Elements.FmtScheme.LineStyleList) {
		return nil
	}
	line := rawTheme.Elements.FmtScheme.LineStyleList[lineRef.Idx-1]
	style := resolveTableLineStyle(&line, themeColors)
	if style == nil || line.SolidFill == nil || line.SolidFill.SchemeClr == nil {
		return style
	}
	if strings.EqualFold(strings.TrimSpace(line.SolidFill.SchemeClr.Val), "phClr") {
		base := resolveTableColor(lineRef.Color, lineRef.SchemeClr, themeColors)
		if base == "" {
			return style
		}
		style.Color = applyTableSchemeColorTransform(base, line.SolidFill.SchemeClr)
	}
	return style
}

func resolveTableFillRef(fillRef *parts.DrawingTableFillRef, rawTheme *parts.Theme, themeColors map[string]string) drawing.Fill {
	if fillRef == nil || rawTheme == nil {
		return nil
	}
	var fill *parts.ThemeFormatFillStyle
	switch {
	case fillRef.Idx >= 1001:
		index := fillRef.Idx - 1001
		if index < 0 || index >= len(rawTheme.Elements.FmtScheme.BGFillStyleList) {
			return nil
		}
		fill = &rawTheme.Elements.FmtScheme.BGFillStyleList[index]
	case fillRef.Idx >= 1:
		index := fillRef.Idx - 1
		if index < 0 || index >= len(rawTheme.Elements.FmtScheme.FillStyleList) {
			return nil
		}
		fill = &rawTheme.Elements.FmtScheme.FillStyleList[index]
	default:
		return nil
	}
	if fill == nil {
		return nil
	}
	return resolveTableCellFill(fill.SolidFill, fill.FillScheme, fill.GradFill, fill.PattFill, fill.BlipFill, fill.NoFill, themeColorsWithPlaceholder(themeColors, resolveTableColor(fillRef.Color, fillRef.SchemeClr, themeColors)))
}

func resolveTableEffectRef(effectRef *parts.DrawingTableEffectRef, rawTheme *parts.Theme, themeColors map[string]string) drawing.EffectList {
	if effectRef == nil || rawTheme == nil || effectRef.Idx < 1 || effectRef.Idx > len(rawTheme.Elements.FmtScheme.EffectStyleList) {
		return drawing.EffectList{}
	}
	style := rawTheme.Elements.FmtScheme.EffectStyleList[effectRef.Idx-1]
	colors := themeColorsWithPlaceholder(themeColors, resolveTableColor(effectRef.Color, effectRef.SchemeClr, themeColors))
	return (&parts.ShapeProperties{
		OuterShadow:  style.OuterShadow,
		PresetShadow: style.PresetShadow,
		FillOverlay:  style.FillOverlay,
		InnerShadow:  style.InnerShadow,
		Glow:         style.Glow,
		SoftEdge:     style.SoftEdge,
		Reflection:   style.Reflection,
		Blur:         style.Blur,
		EffectDag:    style.EffectDag,
	}).DrawingEffectList(colors)
}

func themeColorsWithPlaceholder(themeColors map[string]string, placeholder string) map[string]string {
	if placeholder == "" {
		return themeColors
	}
	out := make(map[string]string, len(themeColors)+1)
	for key, value := range themeColors {
		out[key] = value
	}
	out["phclr"] = placeholder
	out["phClr"] = placeholder
	return out
}

func resolveShapeFillRef(fillRef *parts.ShapeStyleRef, rawTheme *parts.Theme, themeColors map[string]string) drawing.Fill {
	if fillRef == nil || rawTheme == nil {
		return nil
	}
	var fill *parts.ThemeFormatFillStyle
	switch {
	case fillRef.Idx >= 1001:
		index := fillRef.Idx - 1001
		if index < 0 || index >= len(rawTheme.Elements.FmtScheme.BGFillStyleList) {
			return nil
		}
		fill = &rawTheme.Elements.FmtScheme.BGFillStyleList[index]
	case fillRef.Idx >= 1:
		index := fillRef.Idx - 1
		if index < 0 || index >= len(rawTheme.Elements.FmtScheme.FillStyleList) {
			return nil
		}
		fill = &rawTheme.Elements.FmtScheme.FillStyleList[index]
	default:
		return nil
	}
	if fill == nil {
		return nil
	}
	placeholder := resolveTableColor(fillRef.Color, fillRef.SchemeClr, themeColors)
	colors := themeColorsWithPlaceholder(themeColors, placeholder)
	return resolveTableCellFill(fill.SolidFill, fill.FillScheme, fill.GradFill, fill.PattFill, fill.BlipFill, fill.NoFill, colors)
}

func resolveShapeLineRef(lineRef *parts.ShapeStyleRef, rawTheme *parts.Theme, themeColors map[string]string) *drawing.LineStyle {
	if lineRef == nil || rawTheme == nil || lineRef.Idx < 1 || lineRef.Idx > len(rawTheme.Elements.FmtScheme.LineStyleList) {
		return nil
	}
	line := rawTheme.Elements.FmtScheme.LineStyleList[lineRef.Idx-1]
	style := resolveTableLineStyle(&line, themeColors)
	if style == nil || line.SolidFill == nil || line.SolidFill.SchemeClr == nil {
		return style
	}
	if strings.EqualFold(strings.TrimSpace(line.SolidFill.SchemeClr.Val), "phClr") {
		base := resolveTableColor(lineRef.Color, lineRef.SchemeClr, themeColors)
		if base == "" {
			return style
		}
		style.Color = applyTableSchemeColorTransform(base, line.SolidFill.SchemeClr)
	}
	return style
}

func resolveShapeEffectRef(effectRef *parts.ShapeStyleRef, rawTheme *parts.Theme, themeColors map[string]string) drawing.EffectList {
	if effectRef == nil || rawTheme == nil || effectRef.Idx < 1 || effectRef.Idx > len(rawTheme.Elements.FmtScheme.EffectStyleList) {
		return drawing.EffectList{}
	}
	style := rawTheme.Elements.FmtScheme.EffectStyleList[effectRef.Idx-1]
	colors := themeColorsWithPlaceholder(themeColors, resolveTableColor(effectRef.Color, effectRef.SchemeClr, themeColors))
	return (&parts.ShapeProperties{
		OuterShadow:  style.OuterShadow,
		PresetShadow: style.PresetShadow,
		FillOverlay:  style.FillOverlay,
		InnerShadow:  style.InnerShadow,
		Glow:         style.Glow,
		SoftEdge:     style.SoftEdge,
		Reflection:   style.Reflection,
		Blur:         style.Blur,
		EffectDag:    style.EffectDag,
	}).DrawingEffectList(colors)
}

func resolveShapeFontRef(fontRef *parts.ShapeFontRef, theme *Theme) string {
	if fontRef == nil || theme == nil {
		return ""
	}
	idx := strings.ToLower(strings.TrimSpace(fontRef.Idx))
	var typeface string
	switch idx {
	case "major":
		typeface = theme.MajorFont
	case "minor":
		typeface = theme.MinorFont
	}
	if typeface == "" {
		return ""
	}
	return resolveThemeTypeface(theme, typeface)
}

func resolveShapeStyleFill(sp, layoutShape, masterShape *parts.Shape, rawTheme *parts.Theme, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string) drawing.Fill {
	if sp != nil && sp.Style != nil && sp.Style.FillRef != nil {
		if fill := resolveShapeFillRef(sp.Style.FillRef, rawTheme, slideThemeColors); fill != nil {
			return fill
		}
	}
	if layoutShape != nil && layoutShape.Style != nil && layoutShape.Style.FillRef != nil {
		if fill := resolveShapeFillRef(layoutShape.Style.FillRef, rawTheme, layoutThemeColors); fill != nil {
			return fill
		}
	}
	if masterShape != nil && masterShape.Style != nil && masterShape.Style.FillRef != nil {
		if fill := resolveShapeFillRef(masterShape.Style.FillRef, rawTheme, masterThemeColors); fill != nil {
			return fill
		}
	}
	return nil
}

func resolveShapeStyleLine(sp, layoutShape, masterShape *parts.Shape, rawTheme *parts.Theme, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string) *drawing.LineStyle {
	if sp != nil && sp.Style != nil && sp.Style.LnRef != nil {
		if line := resolveShapeLineRef(sp.Style.LnRef, rawTheme, slideThemeColors); line != nil {
			return line
		}
	}
	if layoutShape != nil && layoutShape.Style != nil && layoutShape.Style.LnRef != nil {
		if line := resolveShapeLineRef(layoutShape.Style.LnRef, rawTheme, layoutThemeColors); line != nil {
			return line
		}
	}
	if masterShape != nil && masterShape.Style != nil && masterShape.Style.LnRef != nil {
		if line := resolveShapeLineRef(masterShape.Style.LnRef, rawTheme, masterThemeColors); line != nil {
			return line
		}
	}
	return nil
}

func resolveShapeStyleEffect(sp, layoutShape, masterShape *parts.Shape, rawTheme *parts.Theme, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string) drawing.EffectList {
	if sp != nil && sp.Style != nil && sp.Style.EffectRef != nil {
		if effects := resolveShapeEffectRef(sp.Style.EffectRef, rawTheme, slideThemeColors); len(effects.Effects) > 0 {
			return effects
		}
	}
	if layoutShape != nil && layoutShape.Style != nil && layoutShape.Style.EffectRef != nil {
		if effects := resolveShapeEffectRef(layoutShape.Style.EffectRef, rawTheme, layoutThemeColors); len(effects.Effects) > 0 {
			return effects
		}
	}
	if masterShape != nil && masterShape.Style != nil && masterShape.Style.EffectRef != nil {
		if effects := resolveShapeEffectRef(masterShape.Style.EffectRef, rawTheme, masterThemeColors); len(effects.Effects) > 0 {
			return effects
		}
	}
	return drawing.EffectList{}
}

func resolveShapeStyleFontRef(sp, layoutShape, masterShape *parts.Shape, theme *Theme) string {
	if sp != nil && sp.Style != nil && sp.Style.FontRef != nil {
		if family := resolveShapeFontRef(sp.Style.FontRef, theme); family != "" {
			return family
		}
	}
	if layoutShape != nil && layoutShape.Style != nil && layoutShape.Style.FontRef != nil {
		if family := resolveShapeFontRef(layoutShape.Style.FontRef, theme); family != "" {
			return family
		}
	}
	if masterShape != nil && masterShape.Style != nil && masterShape.Style.FontRef != nil {
		if family := resolveShapeFontRef(masterShape.Style.FontRef, theme); family != "" {
			return family
		}
	}
	return ""
}

// buildSmartArtFormatScheme converts the active theme's a:fmtScheme into the
// format-agnostic ThemeFormatScheme used by the shared SmartArt parser. Each
// matrix entry is pre-resolved through the existing shape-ref helpers, and a
// placeholder-color flag is recorded so the SmartArt parser can apply the
// a:fillRef/a:lnRef child color when one is supplied.
func buildSmartArtFormatScheme(rawTheme *parts.Theme, themeColors map[string]string, theme *Theme) *smartart.ThemeFormatScheme {
	if rawTheme == nil {
		return nil
	}
	fs := rawTheme.Elements.FmtScheme
	scheme := &smartart.ThemeFormatScheme{}

	for i := range fs.FillStyleList {
		ref := &parts.ShapeStyleRef{Idx: i + 1}
		fill := resolveShapeFillRef(ref, rawTheme, themeColors)
		entry := smartart.ThemeFillStyle{Fill: fill}
		if usesPlaceholderFill(&fs.FillStyleList[i]) {
			entry.UsesPlaceholder = true
		}
		scheme.FillStyles = append(scheme.FillStyles, entry)
	}

	for i := range fs.BGFillStyleList {
		ref := &parts.ShapeStyleRef{Idx: i + 1001}
		fill := resolveShapeFillRef(ref, rawTheme, themeColors)
		entry := smartart.ThemeFillStyle{Fill: fill}
		if usesPlaceholderFill(&fs.BGFillStyleList[i]) {
			entry.UsesPlaceholder = true
		}
		scheme.BgFillStyles = append(scheme.BgFillStyles, entry)
	}

	for i := range fs.LineStyleList {
		ref := &parts.ShapeStyleRef{Idx: i + 1}
		line := resolveShapeLineRef(ref, rawTheme, themeColors)
		entry := smartart.ThemeLineStyle{Line: line}
		ln := &fs.LineStyleList[i]
		if ln.SolidFill != nil && ln.SolidFill.SchemeClr != nil &&
			strings.EqualFold(strings.TrimSpace(ln.SolidFill.SchemeClr.Val), "phClr") {
			entry.UsesPlaceholder = true
		}
		scheme.LineStyles = append(scheme.LineStyles, entry)
	}

	for i := range fs.EffectStyleList {
		ref := &parts.ShapeStyleRef{Idx: i + 1}
		scheme.EffectStyles = append(scheme.EffectStyles, resolveShapeEffectRef(ref, rawTheme, themeColors))
	}

	if theme != nil {
		scheme.MajorFont = theme.MajorFont
		scheme.MinorFont = theme.MinorFont
	}
	return scheme
}

func usesPlaceholderFill(e *parts.ThemeFormatFillStyle) bool {
	if e == nil {
		return false
	}
	if e.FillScheme != nil && strings.EqualFold(strings.TrimSpace(e.FillScheme.Val), "phClr") {
		return true
	}
	return false
}

func resolveTableSchemeColor(scheme *drawingxml.SchemeClr, themeColors map[string]string) string {
	if scheme == nil || themeColors == nil {
		return ""
	}
	resolved := drawingxml.ResolveSolidFill(&drawingxml.SolidFill{SchemeClr: scheme}, themeColors)
	if resolved == nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(resolved.Color)), "#")
}

func applyTableSchemeColorTransform(hexRGB string, scheme *drawingxml.SchemeClr) string {
	if scheme == nil {
		return strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(hexRGB)), "#")
	}
	colors := themeColorsWithPlaceholder(nil, hexRGB)
	return resolveTableSchemeColor(scheme, colors)
}

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

type textDefaults struct {
	Align             string
	MarginLeftEMU     *int
	MarginRightEMU    *int
	TextIndentEMU     *int
	DefaultTabSizeEMU *int
	FontAlign         string
	BulletColor       string
	BulletFontFamily  string
	BulletSize        int
	BulletSizePct     int
	Bold              *bool
	Italic            *bool
	Underline         string
	Strike            string
	Caps              string
	Baseline          *int
	FontSize          int
	FontFamily        string
	Color             string
	Highlight         string
	SpaceBefore       *int
	SpaceAfter        *int
	SpaceBeforePct    *int
	SpaceAfterPct     *int
	LineSpacingPct    *int
	TabStops          []TabStop
}

func resolveTextDefaults(shapeBody, layoutBody, masterBody *parts.TextBody, master *parts.SlideMaster, ph PlaceholderKind, level int, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string, theme *Theme) textDefaults {
	defaults := textDefaults{}
	if shapeBody != nil {
		defaults = mergeTextDefaults(defaults, defaultsFromListStyle(shapeBody.ListStyle, level, slideThemeColors, theme))
	}
	if layoutBody != nil {
		defaults = mergeTextDefaults(defaults, defaultsFromListStyle(layoutBody.ListStyle, level, layoutThemeColors, theme))
	}
	if masterBody != nil {
		defaults = mergeTextDefaults(defaults, defaultsFromListStyle(masterBody.ListStyle, level, masterThemeColors, theme))
	}
	defaults = mergeTextDefaults(defaults, defaultsFromMaster(master, ph, level, masterThemeColors, theme))
	if defaults.FontFamily == "" {
		defaults.FontFamily = fallbackThemeFont(theme, ph)
	}
	return defaults
}

func mergeTextDefaults(dst, src textDefaults) textDefaults {
	if dst.Align == "" {
		dst.Align = src.Align
	}
	if dst.MarginLeftEMU == nil {
		dst.MarginLeftEMU = src.MarginLeftEMU
	}
	if dst.MarginRightEMU == nil {
		dst.MarginRightEMU = src.MarginRightEMU
	}
	if dst.TextIndentEMU == nil {
		dst.TextIndentEMU = src.TextIndentEMU
	}
	if dst.DefaultTabSizeEMU == nil {
		dst.DefaultTabSizeEMU = src.DefaultTabSizeEMU
	}
	if dst.FontAlign == "" {
		dst.FontAlign = src.FontAlign
	}
	if dst.BulletColor == "" {
		dst.BulletColor = src.BulletColor
	}
	if dst.BulletFontFamily == "" {
		dst.BulletFontFamily = src.BulletFontFamily
	}
	if dst.BulletSize == 0 {
		dst.BulletSize = src.BulletSize
	}
	if dst.BulletSizePct == 0 {
		dst.BulletSizePct = src.BulletSizePct
	}
	if dst.Bold == nil {
		dst.Bold = src.Bold
	}
	if dst.Italic == nil {
		dst.Italic = src.Italic
	}
	if dst.Underline == "" {
		dst.Underline = src.Underline
	}
	if dst.Strike == "" {
		dst.Strike = src.Strike
	}
	if dst.Caps == "" {
		dst.Caps = src.Caps
	}
	if dst.Baseline == nil {
		dst.Baseline = src.Baseline
	}
	if dst.FontSize == 0 {
		dst.FontSize = src.FontSize
	}
	if dst.FontFamily == "" {
		dst.FontFamily = src.FontFamily
	}
	if dst.Color == "" {
		dst.Color = src.Color
	}
	if dst.SpaceBefore == nil {
		dst.SpaceBefore = src.SpaceBefore
	}
	if dst.SpaceAfter == nil {
		dst.SpaceAfter = src.SpaceAfter
	}
	if dst.SpaceBeforePct == nil {
		dst.SpaceBeforePct = src.SpaceBeforePct
	}
	if dst.SpaceAfterPct == nil {
		dst.SpaceAfterPct = src.SpaceAfterPct
	}
	if dst.LineSpacingPct == nil {
		dst.LineSpacingPct = src.LineSpacingPct
	}
	if len(dst.TabStops) == 0 {
		dst.TabStops = src.TabStops
	}
	return dst
}

func defaultsFromMaster(master *parts.SlideMaster, ph PlaceholderKind, level int, themeColors map[string]string, theme *Theme) textDefaults {
	if master == nil || master.TxStyles == nil {
		return textDefaults{}
	}
	var style *parts.DMLTextListStyle
	switch ph {
	case PlaceholderTitle:
		style = master.TxStyles.TitleStyle
	case PlaceholderBody, PlaceholderSubtitle:
		style = master.TxStyles.BodyStyle
	default:
		style = master.TxStyles.OtherStyle
		if style == nil {
			style = master.TxStyles.BodyStyle
		}
	}
	return defaultsFromListStyle(style, level, themeColors, theme)
}

func defaultsFromListStyle(style *parts.DMLTextListStyle, level int, themeColors map[string]string, theme *Theme) textDefaults {
	levelStyle := style.Level(level)
	if levelStyle == nil {
		return textDefaults{}
	}
	defaults := textDefaults{
		Align: levelStyle.Algn,
	}
	defaults.MarginLeftEMU = levelStyle.MarL
	defaults.MarginRightEMU = levelStyle.MarR
	defaults.TextIndentEMU = levelStyle.Indent
	defaults.DefaultTabSizeEMU = levelStyle.DefTabSz
	defaults.FontAlign = levelStyle.FontAlgn
	defaults.SpaceBefore = resolveSpacingPts(levelStyle.SpcBef)
	defaults.SpaceAfter = resolveSpacingPts(levelStyle.SpcAft)
	defaults.SpaceBeforePct = resolveSpacingPctValue(levelStyle.SpcBef)
	defaults.SpaceAfterPct = resolveSpacingPctValue(levelStyle.SpcAft)
	defaults.LineSpacingPct = resolveSpacingPct(levelStyle.LnSpc)
	defaults.BulletColor = resolveBulletColor(levelStyle.BuClr, levelStyle.BuClrScheme, themeColors)
	defaults.BulletFontFamily = resolveBulletFontFamily(levelStyle.BuFont)
	defaults.BulletSize = resolveBulletSize(levelStyle.BuSzPts)
	defaults.BulletSizePct = resolveBulletSizePct(levelStyle.BuSzPct)
	defaults.TabStops = resolveTabStops(levelStyle.TabStops)
	if levelStyle.DefRPr != nil {
		defaults.Bold = levelStyle.DefRPr.Bold
		defaults.Italic = levelStyle.DefRPr.Italic
		defaults.Underline = levelStyle.DefRPr.Underline
		defaults.Strike = levelStyle.DefRPr.Strike
		defaults.Caps = levelStyle.DefRPr.Cap
		defaults.Baseline = levelStyle.DefRPr.Baseline
		defaults.FontSize = levelStyle.DefRPr.FontSize
		defaults.Color = resolveRunColor(levelStyle.DefRPr, themeColors)
		defaults.FontFamily = resolveRunFontFamily(levelStyle.DefRPr, theme)
	}
	return defaults
}

func resolveBulletColor(srgb *parts.ColorRef, scheme *drawingxml.SchemeClr, themeColors map[string]string) string {
	if srgb != nil {
		return srgb.Val
	}
	if scheme != nil {
		return themeColor(themeColors, scheme.Val)
	}
	return ""
}

func resolveBulletFontFamily(font *parts.BuFont) string {
	if font == nil {
		return ""
	}
	return font.Typeface
}

func resolveBulletSize(size *parts.BuSzPts) int {
	if size == nil {
		return 0
	}
	return size.Val
}

func resolveBulletSizePct(size *parts.BuSzPct) int {
	if size == nil {
		return 0
	}
	return size.Val
}

func resolveTabStops(stops []parts.TabStop) []TabStop {
	if len(stops) == 0 {
		return nil
	}
	result := make([]TabStop, len(stops))
	for i, s := range stops {
		result[i] = TabStop{Pos: int64(s.Pos), Algn: s.Algn}
	}
	return result
}

func resolveSpacingPts(spacing *parts.DMLSpacing) *int {
	if spacing == nil || spacing.SpcPts == nil {
		return nil
	}
	value := spacing.SpcPts.Val
	return &value
}

func resolveSpacingPct(spacing *parts.DMLSpacing) *int {
	if spacing == nil || spacing.SpcPct == nil {
		return nil
	}
	value := spacing.SpcPct.Val
	return &value
}

// resolveSpacingPctValue extracts the SpcPct percentage value from a spacing
// descriptor. Used for spcBef/spcAft where SpcPts is absent but SpcPct is set.
func resolveSpacingPctValue(spacing *parts.DMLSpacing) *int {
	if spacing == nil || spacing.SpcPts != nil || spacing.SpcPct == nil {
		return nil
	}
	value := spacing.SpcPct.Val
	return &value
}

func fallbackThemeFont(theme *Theme, ph PlaceholderKind) string {
	if theme == nil {
		return ""
	}
	switch ph {
	case PlaceholderTitle:
		if theme.MajorFont != "" {
			return theme.MajorFont
		}
	case PlaceholderBody, PlaceholderSubtitle:
		if theme.MinorFont != "" {
			return theme.MinorFont
		}
	}
	if theme.MinorFont != "" {
		return theme.MinorFont
	}
	return theme.MajorFont
}

func findLayoutPlaceholderShape(layout *parts.SlideLayout, ph *parts.PlaceholderInfo) *parts.Shape {
	if layout == nil || ph == nil {
		return nil
	}
	return findPlaceholderShape(layout.CSld.SpTree.Shapes, ph)
}

func findMasterPlaceholderShape(master *parts.SlideMaster, ph *parts.PlaceholderInfo) *parts.Shape {
	if master == nil || ph == nil {
		return nil
	}
	return findPlaceholderShape(master.CSld.SpTree.Shapes, ph)
}

func findLayoutPlaceholderShapeByKind(layout *parts.SlideLayout, kind PlaceholderKind) *parts.Shape {
	if layout == nil {
		return nil
	}
	return findPlaceholderShapeByKind(layout.CSld.SpTree.Shapes, kind)
}

func findMasterPlaceholderShapeByKind(master *parts.SlideMaster, kind PlaceholderKind) *parts.Shape {
	if master == nil {
		return nil
	}
	return findPlaceholderShapeByKind(master.CSld.SpTree.Shapes, kind)
}

func findPlaceholderShape(shapes []parts.Shape, ph *parts.PlaceholderInfo) *parts.Shape {
	if ph == nil {
		return nil
	}
	targetKind := resolvePlaceholderKind(ph)
	if placeholderIdxIsSentinel(ph) {
		return nil
	}
	for i := range shapes {
		candidate := &shapes[i]
		cph := candidate.NvSpPr.NvPr.Ph
		if cph == nil {
			continue
		}
		if ph.Type != "" && cph.Type == ph.Type && ph.Idx == cph.Idx {
			return candidate
		}
		if resolvePlaceholderKind(cph) == targetKind {
			if ph.Idx == 0 || cph.Idx == ph.Idx {
				return candidate
			}
		}
	}
	return nil
}

func findPlaceholderShapeByKind(shapes []parts.Shape, kind PlaceholderKind) *parts.Shape {
	for i := range shapes {
		candidate := &shapes[i]
		if resolvePlaceholderKind(candidate.NvSpPr.NvPr.Ph) == kind {
			return candidate
		}
	}
	return nil
}

func resolveInheritedHeaderFooterPlaceholder(entry *parts.SlideEntry, master *parts.SlideMaster, masterEntry *parts.SlideMasterEntry, kind PlaceholderKind, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string, theme *Theme, rawTheme *parts.Theme) *Shape {
	if entry == nil || !headerFooterPlaceholderEnabled(master, kind) {
		return nil
	}
	layoutShape := findLayoutPlaceholderShapeByKind(entry.Layout, kind)
	masterShape := findMasterPlaceholderShapeByKind(master, kind)
	if layoutShape != nil {
		var masterMediaParts map[string]media.Part
		if masterEntry != nil {
			masterMediaParts = masterEntry.MediaParts
		}
		return resolveShape(layoutShape, nil, masterShape, master, layoutThemeColors, layoutThemeColors, masterThemeColors, theme, rawTheme, entry.LayoutMediaParts, nil, masterMediaParts, nil)
	}
	if masterShape != nil {
		var mediaParts map[string]media.Part
		if masterEntry != nil {
			mediaParts = masterEntry.MediaParts
		}
		return resolveShape(masterShape, nil, nil, master, masterThemeColors, masterThemeColors, masterThemeColors, theme, rawTheme, mediaParts, nil, nil, nil)
	}
	return nil
}

func headerFooterPlaceholderEnabled(master *parts.SlideMaster, kind PlaceholderKind) bool {
	if master == nil || master.HF == nil {
		return true
	}
	switch kind {
	case PlaceholderDateTime:
		return master.HF.DateTime == nil || *master.HF.DateTime
	case PlaceholderSlideNumber:
		return master.HF.SlideNumber == nil || *master.HF.SlideNumber
	case PlaceholderFooter:
		return master.HF.Footer == nil || *master.HF.Footer
	default:
		return true
	}
}

func placeholderIdxIsSentinel(ph *parts.PlaceholderInfo) bool {
	if ph == nil || ph.Idx < 0 {
		return false
	}
	return int64(ph.Idx) == int64(^uint32(0))
}

func resolvePlaceholderKind(ph *parts.PlaceholderInfo) PlaceholderKind {
	if ph == nil {
		return PlaceholderNone
	}
	switch ph.Type {
	case "title", "ctrTitle":
		return PlaceholderTitle
	case "body":
		return PlaceholderBody
	case "subTitle":
		return PlaceholderSubtitle
	case "dt":
		return PlaceholderDateTime
	case "sldNum":
		return PlaceholderSlideNumber
	case "ftr":
		return PlaceholderFooter
	case "hdr":
		return PlaceholderHeader
	case "obj":
		return PlaceholderContent
	case "chart":
		return PlaceholderChart
	case "tbl":
		return PlaceholderTable
	case "pic":
		return PlaceholderPicture
	case "media":
		return PlaceholderMedia
	case "clipArt":
		return PlaceholderClipArt
	case "dgm":
		return PlaceholderDiagram
	case "sldImg":
		return PlaceholderSlideImage
	case "":
		if ph.Idx == 0 {
			return PlaceholderTitle
		}
		return PlaceholderBody
	default:
		return PlaceholderOther
	}
}

func resolveTransform(xfrm *parts.Transform2D) Rect {
	if xfrm == nil {
		return Rect{}
	}
	return Rect{
		X: xfrm.Off.X,
		Y: xfrm.Off.Y,
		W: xfrm.Ext.CX,
		H: xfrm.Ext.CY,
	}
}

func shapeRotationDegrees(xfrm *parts.Transform2D) float64 {
	if xfrm == nil || xfrm.Rot == 0 {
		return 0
	}
	return float64(xfrm.Rot) / 60000.0
}

// applyGroupTransformToChildBounds transforms child shape bounds from group-local
// coordinates to slide coordinates using the group's transform.
// It handles scaling via chOff/chExt, translation via off, and returns bounds
// prior to rotation/flip (those are composed separately on the shape).
func applyGroupTransformToChildBounds(childBounds Rect, groupXfrm *parts.Transform2D) Rect {
	if groupXfrm == nil {
		return childBounds
	}

	// Compute scale factors from child coordinate space to slide space.
	// If the child bounding box dimension is zero, scaling is skipped in that
	// dimension per OOXML spec (L.4.7.4).
	scaleX := 1.0
	scaleY := 1.0
	if groupXfrm.ChExt.CX != 0 {
		scaleX = float64(groupXfrm.Ext.CX) / float64(groupXfrm.ChExt.CX)
	}
	if groupXfrm.ChExt.CY != 0 {
		scaleY = float64(groupXfrm.Ext.CY) / float64(groupXfrm.ChExt.CY)
	}

	return Rect{
		X: int64(float64(groupXfrm.Off.X) + (float64(childBounds.X)-float64(groupXfrm.ChOff.X))*scaleX),
		Y: int64(float64(groupXfrm.Off.Y) + (float64(childBounds.Y)-float64(groupXfrm.ChOff.Y))*scaleY),
		W: int64(float64(childBounds.W) * scaleX),
		H: int64(float64(childBounds.H) * scaleY),
	}
}

// applyGroupTransformToShape applies a group transform to a resolved shape and
// all of its descendants. It updates bounds, rotation, and flip flags.
func applyGroupTransformToShape(shape *Shape, groupXfrm *parts.Transform2D) {
	if shape == nil || groupXfrm == nil {
		return
	}
	shape.Bounds = applyGroupTransformToChildBounds(shape.Bounds, groupXfrm)
	shape.RotationDeg += shapeRotationDegrees(groupXfrm)
	shape.FlipH = shape.FlipH != groupXfrm.FlipH
	shape.FlipV = shape.FlipV != groupXfrm.FlipV
	// Connectors keep their flip state in separate fields.
	shape.ConnectorFlipH = shape.ConnectorFlipH != groupXfrm.FlipH
	shape.ConnectorFlipV = shape.ConnectorFlipV != groupXfrm.FlipV
	if shape.ConnectorStart != nil {
		shape.ConnectorStart = applyGroupTransformToPoint(shape.ConnectorStart, groupXfrm)
	}
	if shape.ConnectorEnd != nil {
		shape.ConnectorEnd = applyGroupTransformToPoint(shape.ConnectorEnd, groupXfrm)
	}
	for _, child := range shape.Children {
		applyGroupTransformToShape(child, groupXfrm)
	}
}

func applyGroupTransformToPoint(pt *Point, groupXfrm *parts.Transform2D) *Point {
	if pt == nil || groupXfrm == nil {
		return pt
	}
	scaleX := 1.0
	scaleY := 1.0
	if groupXfrm.ChExt.CX != 0 {
		scaleX = float64(groupXfrm.Ext.CX) / float64(groupXfrm.ChExt.CX)
	}
	if groupXfrm.ChExt.CY != 0 {
		scaleY = float64(groupXfrm.Ext.CY) / float64(groupXfrm.ChExt.CY)
	}
	return &Point{
		X: int64(float64(groupXfrm.Off.X) + (float64(pt.X)-float64(groupXfrm.ChOff.X))*scaleX),
		Y: int64(float64(groupXfrm.Off.Y) + (float64(pt.Y)-float64(groupXfrm.ChOff.Y))*scaleY),
	}
}

func mergeEndParaRPrDefaults(dst textDefaults, rpr *parts.DMLRunPr, themeColors map[string]string, theme *Theme) textDefaults {
	if rpr == nil {
		return dst
	}
	if rpr.Bold != nil {
		dst.Bold = rpr.Bold
	}
	if rpr.Italic != nil {
		dst.Italic = rpr.Italic
	}
	if rpr.Underline != "" {
		dst.Underline = rpr.Underline
	}
	if rpr.Strike != "" {
		dst.Strike = rpr.Strike
	}
	if rpr.Cap != "" {
		dst.Caps = rpr.Cap
	}
	if rpr.Baseline != nil {
		dst.Baseline = rpr.Baseline
	}
	if rpr.FontSize > 0 {
		dst.FontSize = rpr.FontSize
	}
	if family := resolveRunFontFamily(rpr, theme); family != "" {
		dst.FontFamily = family
	}
	if color := resolveRunColor(rpr, themeColors); color != "" {
		dst.Color = color
	}
	if hl := resolveRunHighlight(rpr, themeColors); hl != "" {
		dst.Highlight = hl
	}
	return dst
}

func resolveParagraph(ap *parts.DMLParagraph, defaults textDefaults, themeColors map[string]string, theme *Theme, hlinkURLs map[string]string) *Paragraph {
	// Apply endParaRPr as paragraph-level character defaults.
	defaults = mergeEndParaRPrDefaults(defaults, ap.EndParaRPr, themeColors, theme)

	p := &Paragraph{
		BulletColor:      defaults.BulletColor,
		BulletFontFamily: defaults.BulletFontFamily,
		BulletSize:       defaults.BulletSize,
		BulletSizePct:    defaults.BulletSizePct,
		TabStops:         defaults.TabStops,
	}
	if defaults.MarginLeftEMU != nil {
		p.MarginLeftEMU = *defaults.MarginLeftEMU
		p.HasMarginLeft = true
	}
	if defaults.MarginRightEMU != nil {
		p.MarginRightEMU = *defaults.MarginRightEMU
		p.HasMarginRight = true
	}
	if defaults.TextIndentEMU != nil {
		p.TextIndentEMU = *defaults.TextIndentEMU
		p.HasTextIndent = true
	}
	if defaults.DefaultTabSizeEMU != nil {
		p.DefaultTabSizeEMU = *defaults.DefaultTabSizeEMU
	}
	if defaults.FontAlign != "" {
		p.FontAlign = defaults.FontAlign
	}
	if defaults.SpaceBefore != nil {
		p.SpaceBefore = *defaults.SpaceBefore
		p.HasSpaceBefore = true
	} else if defaults.SpaceBeforePct != nil {
		p.SpaceBeforePct = *defaults.SpaceBeforePct
	}
	if defaults.SpaceAfter != nil {
		p.SpaceAfter = *defaults.SpaceAfter
		p.HasSpaceAfter = true
	} else if defaults.SpaceAfterPct != nil {
		p.SpaceAfterPct = *defaults.SpaceAfterPct
	}
	if defaults.LineSpacingPct != nil {
		p.LineSpacingPct = *defaults.LineSpacingPct
		p.HasLineSpacing = true
	}
	if ap.Properties != nil {
		p.Level = ap.Properties.Lvl
		p.Align = ap.Properties.Algn
		if ap.Properties.MarL != nil {
			p.MarginLeftEMU = *ap.Properties.MarL
			p.HasMarginLeft = true
		}
		if ap.Properties.MarR != nil {
			p.MarginRightEMU = *ap.Properties.MarR
			p.HasMarginRight = true
		}
		if ap.Properties.Indent != nil {
			p.TextIndentEMU = *ap.Properties.Indent
			p.HasTextIndent = true
		}
		if ap.Properties.DefTabSz != nil {
			p.DefaultTabSizeEMU = *ap.Properties.DefTabSz
		}
		if ap.Properties.FontAlgn != "" {
			p.FontAlign = ap.Properties.FontAlgn
		}
		if value := resolveSpacingPts(ap.Properties.SpcBef); value != nil {
			p.SpaceBefore = *value
			p.HasSpaceBefore = true
			p.SpaceBeforePct = 0
		} else if value := resolveSpacingPctValue(ap.Properties.SpcBef); value != nil {
			p.SpaceBeforePct = *value
			p.SpaceBefore = 0
			p.HasSpaceBefore = false
		}
		if value := resolveSpacingPts(ap.Properties.SpcAft); value != nil {
			p.SpaceAfter = *value
			p.HasSpaceAfter = true
			p.SpaceAfterPct = 0
		} else if value := resolveSpacingPctValue(ap.Properties.SpcAft); value != nil {
			p.SpaceAfterPct = *value
			p.SpaceAfter = 0
			p.HasSpaceAfter = false
		}
		if value := resolveSpacingPct(ap.Properties.LnSpc); value != nil {
			p.LineSpacingPct = *value
			p.HasLineSpacing = true
		}
		if ap.Properties.BuNone == nil {
			if ap.Properties.BuChar != nil && ap.Properties.BuChar.Char != "" {
				p.BulletChar = ap.Properties.BuChar.Char
			} else if ap.Properties.BuAutoNum != nil {
				p.AutoNum = true
				p.AutoNumType = ap.Properties.BuAutoNum.Type
				p.AutoNumStart = ap.Properties.BuAutoNum.StartAt
			}
		}
		if color := resolveBulletColor(ap.Properties.BuClr, ap.Properties.BuClrScheme, themeColors); color != "" {
			p.BulletColor = color
		}
		if family := resolveBulletFontFamily(ap.Properties.BuFont); family != "" {
			p.BulletFontFamily = family
		}
		if size := resolveBulletSize(ap.Properties.BuSzPts); size > 0 {
			p.BulletSize = size
			p.BulletSizePct = 0
		}
		if pct := resolveBulletSizePct(ap.Properties.BuSzPct); pct > 0 {
			p.BulletSizePct = pct
			p.BulletSize = 0
		}
		if len(ap.Properties.TabStops) > 0 {
			p.TabStops = resolveTabStops(ap.Properties.TabStops)
		}
	}
	if p.Align == "" {
		p.Align = defaults.Align
	}
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
	defaults := resolveTextDefaults(sourceBody, layoutTextBody(layoutShape), layoutTextBody(masterShape), master, PlaceholderSlideNumber, level, slideThemeColors, layoutThemeColors, masterThemeColors, theme)
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
	run := &Run{
		Text:        r.Text,
		Bold:        defaults.Bold != nil && *defaults.Bold,
		Italic:      defaults.Italic != nil && *defaults.Italic,
		Underline:   defaults.Underline != "" && defaults.Underline != "none",
		Strike:      defaults.Strike != "" && defaults.Strike != "noStrike",
		AllCaps:     defaults.Caps == "all",
		SmallCaps:   defaults.Caps == "small",
		Superscript: defaults.Baseline != nil && *defaults.Baseline > 0,
		Subscript:   defaults.Baseline != nil && *defaults.Baseline < 0,
		FontSize:    defaults.FontSize,
		FontFamily:  defaults.FontFamily,
		Color:       defaults.Color,
		Highlight:   defaults.Highlight,
	}
	if r.Properties != nil {
		applyRunProperties(run, r.Properties, defaults, themeColors, theme, hlinkURLs)
	}
	return run
}

// applyRunProperties applies DML run properties onto a resolved Run.
func applyRunProperties(run *Run, props *parts.DMLRunPr, defaults textDefaults, themeColors map[string]string, theme *Theme, hlinkURLs map[string]string) {
	if props.Bold != nil {
		run.Bold = *props.Bold
	}
	if props.Italic != nil {
		run.Italic = *props.Italic
	}
	if props.Underline != "" {
		run.Underline = props.Underline != "none"
	}
	if props.Strike != "" {
		run.Strike = props.Strike != "noStrike"
	}
	if props.Cap != "" {
		run.AllCaps = props.Cap == "all"
		run.SmallCaps = props.Cap == "small"
	}
	if props.Baseline != nil {
		run.Superscript = *props.Baseline > 0
		run.Subscript = *props.Baseline < 0
	}
	if props.FontSize > 0 {
		run.FontSize = props.FontSize
	}
	if color := resolveRunColor(props, themeColors); color != "" {
		run.Color = color
	}
	if family := resolveRunFontFamily(props, theme); family != "" {
		run.FontFamily = family
	}
	if hl := resolveRunHighlight(props, themeColors); hl != "" {
		run.Highlight = hl
	}
	if props.HlinkClick != nil {
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
		run = &Run{
			Bold:        defaults.Bold != nil && *defaults.Bold,
			Italic:      defaults.Italic != nil && *defaults.Italic,
			Underline:   defaults.Underline != "" && defaults.Underline != "none",
			Strike:      defaults.Strike != "" && defaults.Strike != "noStrike",
			AllCaps:     defaults.Caps == "all",
			SmallCaps:   defaults.Caps == "small",
			Superscript: defaults.Baseline != nil && *defaults.Baseline > 0,
			Subscript:   defaults.Baseline != nil && *defaults.Baseline < 0,
			FontSize:    defaults.FontSize,
			FontFamily:  defaults.FontFamily,
			Color:       defaults.Color,
			Highlight:   defaults.Highlight,
		}
		if f.Properties != nil {
			if f.Properties.Bold != nil {
				run.Bold = *f.Properties.Bold
			}
			if f.Properties.Italic != nil {
				run.Italic = *f.Properties.Italic
			}
			if f.Properties.FontSize > 0 {
				run.FontSize = f.Properties.FontSize
			}
			if color := resolveRunColor(f.Properties, themeColors); color != "" {
				run.Color = color
			}
			if hl := resolveRunHighlight(f.Properties, themeColors); hl != "" {
				run.Highlight = hl
			}
			if family := resolveRunFontFamily(f.Properties, theme); family != "" {
				run.FontFamily = family
			}
		}
	}
	run.FieldKind = kind
	return run
}

func resolveRunColor(pr *parts.DMLRunPr, themeColors map[string]string) string {
	if pr == nil {
		return ""
	}
	if pr.SolidFill != nil {
		return pr.SolidFill.Val
	}
	if pr.SchemeColor != nil {
		return themeColor(themeColors, pr.SchemeColor.Val)
	}
	return ""
}

func resolveRunHighlight(pr *parts.DMLRunPr, themeColors map[string]string) string {
	if pr == nil {
		return ""
	}
	if pr.Highlight != nil {
		return pr.Highlight.Val
	}
	if pr.HighlightScheme != nil {
		return themeColor(themeColors, pr.HighlightScheme.Val)
	}
	return ""
}

func resolveRunFontFamily(pr *parts.DMLRunPr, theme *Theme) string {
	if pr == nil {
		return ""
	}
	for _, ref := range []*parts.FontRef{pr.Latin, pr.EA, pr.CS} {
		if ref == nil || ref.Typeface == "" {
			continue
		}
		if family := resolveThemeTypeface(theme, ref.Typeface); family != "" {
			return family
		}
	}
	return ""
}

func resolveThemeTypeface(theme *Theme, typeface string) string {
	typeface = strings.TrimSpace(typeface)
	if typeface == "" {
		return ""
	}
	token := strings.ToLower(typeface)
	switch token {
	case "+mj", "+mj-lt", "+mj-latin":
		if theme != nil && theme.MajorFont != "" {
			return theme.MajorFont
		}
		return ""
	case "+mn", "+mn-lt", "+mn-latin":
		if theme != nil && theme.MinorFont != "" {
			return theme.MinorFont
		}
		return ""
	case "+mj-ea":
		if theme != nil {
			if theme.MajorEAFont != "" {
				return theme.MajorEAFont
			}
			return theme.MajorFont
		}
		return ""
	case "+mn-ea":
		if theme != nil {
			if theme.MinorEAFont != "" {
				return theme.MinorEAFont
			}
			return theme.MinorFont
		}
		return ""
	case "+mj-cs":
		if theme != nil {
			if theme.MajorCSFont != "" {
				return theme.MajorCSFont
			}
			return theme.MajorFont
		}
		return ""
	case "+mn-cs":
		if theme != nil {
			if theme.MinorCSFont != "" {
				return theme.MinorCSFont
			}
			return theme.MinorFont
		}
		return ""
	default:
		if strings.HasPrefix(token, "+mj") || strings.HasPrefix(token, "+mn") {
			return ""
		}
	}
	return typeface
}

// resolveNotesSlide converts a notes slide part to a resolved NotesSlide.
func resolveNotesSlide(notes *parts.NotesSlide, notesMaster *parts.NotesMaster, themeColors map[string]string, theme *Theme, rawTheme *parts.Theme, tableStyles *parts.TableStyleList) *NotesSlide {
	if notes == nil {
		return nil
	}
	if notes.ThemeOverride != nil {
		rawTheme = applyThemeOverride(rawTheme, notes.ThemeOverride)
		theme = resolveTheme(rawTheme)
		themeColors = map[string]string{}
		if theme != nil {
			themeColors = buildThemeColorMap(theme.ColorScheme, nil)
		}
	}
	resolved := &NotesSlide{}
	master := notesMasterAsSlideMaster(notesMaster)
	resolved.BgColor = resolveBgColor(notes.CSld.Bg, themeColors)
	if resolved.BgColor == "" && master != nil {
		resolved.BgColor = resolveBgColor(master.CSld.Bg, themeColors)
	}
	// Resolve shapes in the notes slide
	for _, sp := range notes.CSld.SpTree.Shapes {
		masterShape := findMasterPlaceholderShape(master, sp.NvSpPr.NvPr.Ph)
		shape := resolveShape(&sp, nil, masterShape, master, themeColors, themeColors, themeColors, theme, rawTheme, nil, nil, nil, nil)
		if shape != nil {
			resolved.Shapes = append(resolved.Shapes, shape)
		}
	}
	for _, pic := range notes.CSld.SpTree.Pictures {
		shape := resolvePicture(&pic, nil, themeColors, nil)
		if shape != nil {
			resolved.Shapes = append(resolved.Shapes, shape)
		}
	}
	for _, grp := range notes.CSld.SpTree.GroupShapes {
		shape := resolveGroupShape(&grp, nil, themeColors, nil, rawTheme, nil, nil, nil, tableStyles)
		if shape != nil {
			resolved.Shapes = append(resolved.Shapes, shape)
		}
	}
	for _, cx := range notes.CSld.SpTree.Connectors {
		shape := resolveConnector(&cx, themeColors, nil)
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

// resolveSlideComments converts raw PptxComment parts into resolved Comment IR.
func resolveSlideComments(raw []*parts.PptxComment, authors map[int]parts.CommentAuthor) []*Comment {
	if len(raw) == 0 {
		return nil
	}
	out := make([]*Comment, 0, len(raw))
	for _, c := range raw {
		if c == nil {
			continue
		}
		comment := &Comment{
			DateTime: c.DateTime,
			Text:     c.Text,
			X:        c.X,
			Y:        c.Y,
		}
		if a, ok := authors[c.AuthorID]; ok {
			comment.AuthorName = a.Name
			comment.Initials = a.Initials
		}
		out = append(out, comment)
	}
	return out
}

func normalizeRelID(id string) string {
	return id
}
