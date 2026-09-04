package layout

import (
	"strings"

	"office/pkg/presentation/parsers/ooxml/parts"
	"office/pkg/presentation/parsers/ooxml/resolve"
)

// ComposePackage transforms a raw parts.Package into a fully composed Scene.
func ComposePackage(pkg *parts.Package, opts ComposeOptions) (*Scene, error) {
	if pkg == nil {
		return nil, nil
	}

	scene := &Scene{Diagnostics: pkg.Diagnostics}

	if pkg.Presentation != nil && pkg.Presentation.SlideSize != nil {
		scene.SlideWidthEMU = pkg.Presentation.SlideSize.CX
		scene.SlideHeightEMU = pkg.Presentation.SlideSize.CY
	}

	// Resolve theme and embedded fonts (same as existing resolve logic)
	scene.Theme = resolveFirstTheme(pkg)
	scene.EmbeddedFonts = resolveEmbeddedFonts(pkg.EmbeddedFonts)

	// Build a flat color map from the first theme for scheme color resolution.
	themeColors := map[string]string{}
	if scene.Theme != nil {
		themeColors = scene.Theme.ColorScheme
	}

	// Find the first slide master
	masterEntry := firstSlideMasterEntry(pkg)
	var master *parts.SlideMaster
	if masterEntry != nil {
		master = masterEntry.Master
	}

	// Compose each slide
	for i, entry := range pkg.Slides {
		slideScene := composeSlide(i, entry, master, masterEntry, themeColors, scene.Theme, opts)
		scene.Slides = append(scene.Slides, slideScene)
	}

	return scene, nil
}

// composeSlide creates a fully composed slide scene with proper inheritance.
func composeSlide(index int, entry *parts.SlideEntry, master *parts.SlideMaster, masterEntry *parts.SlideMasterEntry, themeColors map[string]string, theme *resolve.Theme, opts ComposeOptions) *SlideScene {
	slide := &SlideScene{Index: index}

	// Resolve background color with inheritance (master → layout → slide)
	slide.BgColor = resolveBgColorWithInheritance(entry, master, themeColors)

	// Build inheritance trace if enabled
	var trace *InheritanceTrace
	if opts.EnableTracing {
		trace = &InheritanceTrace{
			MasterProperties:     make(map[string]interface{}),
			LayoutProperties:     make(map[string]interface{}),
			SlideProperties:      make(map[string]interface{}),
			OverriddenProperties: make(map[string][]string),
		}
		slide.InheritanceTrace = trace
	}

	// Collect shapes in exact tree order with proper z-ordering
	slide.Shapes = collectShapesInTreeOrder(entry, master, masterEntry, themeColors, theme, trace, opts)

	return slide
}

// resolveBgColorWithInheritance resolves background color with proper inheritance chain.
func resolveBgColorWithInheritance(entry *parts.SlideEntry, master *parts.SlideMaster, themeColors map[string]string) string {
	// Check slide first
	if entry.Slide != nil && entry.Slide.CSld.Bg != nil {
		if color := resolveBgColor(entry.Slide.CSld.Bg, themeColors); color != "" {
			return color
		}
	}

	// Check layout
	if entry.Layout != nil && entry.Layout.CSld.Bg != nil {
		if color := resolveBgColor(entry.Layout.CSld.Bg, themeColors); color != "" {
			return color
		}
	}

	// Check master
	if master != nil && master.CSld.Bg != nil {
		if color := resolveBgColor(master.CSld.Bg, themeColors); color != "" {
			return color
		}
	}

	return ""
}

// resolveBgColor is a helper to resolve background color from SlideBg.
func resolveBgColor(bg *parts.SlideBg, themeColors map[string]string) string {
	if bg == nil {
		return ""
	}
	if bg.BgPrSrgb != nil {
		return bg.BgPrSrgb.Val
	}
	if bg.BgPrScheme != nil {
		return themeColors[bg.BgPrScheme.Val]
	}
	if bg.BgRef != nil {
		if bg.BgRef.Srgb != nil {
			return bg.BgRef.Srgb.Val
		}
		if bg.BgRef.Scheme != nil {
			return themeColors[bg.BgRef.Scheme.Val]
		}
	}
	return ""
}

// collectShapesInTreeOrder collects all shapes in the correct tree order for rendering.
func collectShapesInTreeOrder(entry *parts.SlideEntry, master *parts.SlideMaster, masterEntry *parts.SlideMasterEntry, themeColors map[string]string, theme *resolve.Theme, trace *InheritanceTrace, opts ComposeOptions) []*ComposedShape {
	var shapes []*ComposedShape

	// 1. Master background shapes (bottom layer)
	if masterEntry != nil && master != nil {
		for _, pic := range master.CSld.SpTree.Pictures {
			shape := resolvePicture(&pic, masterEntry.MediaParts, themeColors)
			if shape != nil {
				shapes = append(shapes, &ComposedShape{
					Source: SourceMaster,
					Shape:  shape,
				})
			}
		}
	}

	// 2. Layout decorative shapes (non-placeholder shapes, above master)
	if entry.Layout != nil {
		// Collect non-placeholder shapes from layout
		for _, sp := range entry.Layout.CSld.SpTree.Shapes {
			if sp.NvSpPr.NvPr.Ph != nil {
				continue // skip placeholders — overridden by slide content
			}
			shape := resolveShape(&sp, nil, nil, master, themeColors, theme)
			if shape != nil {
				shapes = append(shapes, &ComposedShape{
					Source: SourceLayout,
					Shape:  shape,
				})
			}
		}

		// Collect pictures from layout
		for _, pic := range entry.Layout.CSld.SpTree.Pictures {
			shape := resolvePicture(&pic, entry.LayoutMediaParts, themeColors)
			if shape != nil {
				shapes = append(shapes, &ComposedShape{
					Source: SourceLayout,
					Shape:  shape,
				})
			}
		}
	}

	// 3. Slide shapes (including placeholders that override layout/master)
	if entry.Slide != nil {
		// Process slide shapes in tree order
		for _, item := range orderedShapeTreeItems(entry.Slide.CSld.SpTree) {
			switch item.Kind {
			case parts.ShapeTreeItemShape:
				sp := entry.Slide.CSld.SpTree.Shapes[item.Index]
				// Find corresponding layout and master shapes for inheritance
				layoutShape := findLayoutPlaceholderShape(entry.Layout, sp.NvSpPr.NvPr.Ph)
				masterShape := findMasterPlaceholderShape(master, sp.NvSpPr.NvPr.Ph)

				shape := resolveShape(&sp, layoutShape, masterShape, master, themeColors, theme)
				if shape != nil {
					shapes = append(shapes, &ComposedShape{
						Source: SourceSlide,
						Shape:  shape,
					})
				}

			case parts.ShapeTreeItemPicture:
				pic := entry.Slide.CSld.SpTree.Pictures[item.Index]
				shape := resolvePicture(&pic, entry.MediaParts, themeColors)
				if shape != nil {
					shapes = append(shapes, &ComposedShape{
						Source: SourceSlide,
						Shape:  shape,
					})
				}

			case parts.ShapeTreeItemGroup:
				grp := entry.Slide.CSld.SpTree.GroupShapes[item.Index]
				shape := resolveGroupShape(&grp, master, themeColors, theme, entry.MediaParts)
				if shape != nil {
					shapes = append(shapes, &ComposedShape{
						Source: SourceSlide,
						Shape:  shape,
					})
				}

			case parts.ShapeTreeItemConnector:
				cx := entry.Slide.CSld.SpTree.Connectors[item.Index]
				// Build shape index for connector endpoints
				shapeIndex := map[int]*resolve.Shape{}
				for _, cs := range shapes {
					indexShape(shapeIndex, cs.Shape)
				}
				shape := resolveConnector(&cx, themeColors, shapeIndex)
				if shape != nil {
					shapes = append(shapes, &ComposedShape{
						Source: SourceSlide,
						Shape:  shape,
					})
				}
			}
		}
	}

	return shapes
}

// Helper functions from resolve package (copied/adapted)

func resolveFirstTheme(pkg *parts.Package) *resolve.Theme {
	for _, t := range pkg.Themes {
		return resolveTheme(t)
	}
	return nil
}

func resolveTheme(t *parts.Theme) *resolve.Theme {
	out := &resolve.Theme{
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

func resolveEmbeddedFonts(fonts []*parts.EmbeddedFontData) []*resolve.EmbeddedFont {
	out := make([]*resolve.EmbeddedFont, 0, len(fonts))
	for _, font := range fonts {
		if font == nil || strings.TrimSpace(font.Typeface) == "" {
			continue
		}
		resolved := &resolve.EmbeddedFont{Family: strings.TrimSpace(font.Typeface)}
		appendFace := func(part *parts.FontPart, weight int, style string) {
			if part == nil || len(part.Data) == 0 {
				return
			}
			resolved.Faces = append(resolved.Faces, &resolve.EmbeddedFontFace{
				Family:      resolved.Family,
				Weight:      weight,
				Style:       style,
				ContentType: part.ContentType,
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

// The following functions are implemented in resolve_adapters.go