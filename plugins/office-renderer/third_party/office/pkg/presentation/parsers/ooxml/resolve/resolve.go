package resolve

import (
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/media"
	"office/pkg/presentation/layout/cascade"
	"office/pkg/presentation/parsers/ooxml/parts"
	"office/pkg/presentation/parsers/ooxml/project"
)

func resolveFixturePackage(pkg *parts.Package) (*Presentation, error) {
	if pkg == nil {
		return nil, nil
	}

	pres := project.NewPresentation(pkg)

	rawTheme := firstThemePart(pkg)
	pres.Theme = cascade.ResolveTheme(rawTheme)

	masterIDByPart := project.MasterIDsByPart(pkg)

	var presentationTextStyle *parts.DMLTextListStyle
	if pkg.Presentation != nil {
		presentationTextStyle = pkg.Presentation.DefaultTextStyle
	}

	for i, entry := range pkg.Slides {
		master, masterEntry := slideMasterForEntry(pkg, entry)
		themeContext := cascade.ResolveThemeContext(rawTheme, master, entry.Layout, entry.Slide)

		slide := resolveSlide(i, entry, project.Resources(entry), master, masterEntry, pkg.NotesMaster, themeContext.SlideColors, themeContext.LayoutColors, themeContext.MasterColors, themeContext.Theme, themeContext.RawTheme, pkg.TableStyles, presentationTextStyle)
		slide.Comments = project.Comments(entry.Comments, pkg.CommentAuthors)
		if id, ok := masterIDByPart[entry.MasterPartName]; ok {
			slide.MasterID = id
		}
		if entry.Layout != nil {
			slide.LayoutType = entry.Layout.Type
		}
		pres.Slides = append(pres.Slides, slide)
	}

	// Resolve slide sections when present.
	project.ApplySections(pkg.Presentation, pres)

	return pres, nil
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
	// No structural relationship means no master ownership.  Do not borrow
	// the first master: package order is not an ownership relationship.
	return nil, nil
}

func firstThemePart(pkg *parts.Package) *parts.Theme {
	for _, t := range pkg.Themes {
		return t
	}
	return nil
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

func resolveSlide(index int, entry *parts.SlideEntry, resources project.SlideResources, master *parts.SlideMaster, masterEntry *parts.SlideMasterEntry, notesMaster *parts.NotesMaster, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string, theme *Theme, rawTheme *parts.Theme, tableStyles *parts.TableStyleList, presentationTextStyle *parts.DMLTextListStyle) *Slide {
	slide := &Slide{Index: index}
	if entry.Slide == nil {
		return slide
	}
	slide.BgColor = cascade.BackgroundColor(entry.Slide.CSld.Bg, slideThemeColors)
	if slide.BgColor == "" && entry.Layout != nil {
		slide.BgColor = cascade.BackgroundColor(entry.Layout.CSld.Bg, layoutThemeColors)
	}
	if slide.BgColor == "" && master != nil {
		slide.BgColor = cascade.BackgroundColor(master.CSld.Bg, masterThemeColors)
	}
	slide.BgFill = cascade.BackgroundFill(entry.Slide.CSld.Bg, slideThemeColors, rawTheme, entry.MediaParts)
	if slide.BgFill != nil {
		slide.BgPictureTile = backgroundPictureTile(entry.Slide.CSld.Bg, rawTheme)
	}
	if slide.BgFill == nil && entry.Layout != nil {
		slide.BgFill = cascade.BackgroundFill(entry.Layout.CSld.Bg, layoutThemeColors, rawTheme, entry.LayoutMediaParts)
		if slide.BgFill != nil {
			slide.BgPictureTile = backgroundPictureTile(entry.Layout.CSld.Bg, rawTheme)
		}
	}
	if slide.BgFill == nil && master != nil {
		var masterMediaParts map[string]media.Part
		if masterEntry != nil {
			masterMediaParts = masterEntry.MediaParts
		}
		slide.BgFill = cascade.BackgroundFill(master.CSld.Bg, masterThemeColors, rawTheme, masterMediaParts)
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
				shape := resolveShape(&sp, nil, nil, master, masterThemeColors, masterThemeColors, masterThemeColors, theme, rawTheme, masterEntry.MediaParts, nil, nil, nil, presentationTextStyle)
				if shape != nil {
					slide.Shapes = append(slide.Shapes, shape)
				}
			case parts.ShapeTreeItemPicture:
				pic := master.CSld.SpTree.Pictures[item.Index]
				shape := resolvePicture(&pic, project.WithMedia(masterEntry.MediaParts), masterThemeColors)
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
			shape := resolveShape(&sp, nil, nil, master, layoutThemeColors, layoutThemeColors, masterThemeColors, theme, rawTheme, entry.LayoutMediaParts, nil, nil, nil, presentationTextStyle)
			if shape != nil {
				slide.Shapes = append(slide.Shapes, shape)
			}
		}
		for _, pic := range entry.Layout.CSld.SpTree.Pictures {
			shape := resolvePicture(&pic, project.WithMedia(entry.LayoutMediaParts), layoutThemeColors)
			if shape != nil {
				slide.Shapes = append(slide.Shapes, shape)
			}
		}
	}
	resolvedSlideShapes := make([]*Shape, len(entry.Slide.CSld.SpTree.Shapes))
	for i, sp := range entry.Slide.CSld.SpTree.Shapes {
		placeholderSources := cascade.ResolvePlaceholderSources(entry.Layout, master, sp.NvSpPr.NvPr.Ph)
		layoutShape := placeholderSources.Layout
		masterShape := placeholderSources.Master
		var masterMediaParts map[string]media.Part
		if masterEntry != nil {
			masterMediaParts = masterEntry.MediaParts
		}
		resolvedSlideShapes[i] = resolveShape(&sp, layoutShape, masterShape, master, slideThemeColors, layoutThemeColors, masterThemeColors, theme, rawTheme, entry.MediaParts, entry.LayoutMediaParts, masterMediaParts, entry.HlinkURLs, presentationTextStyle)
	}
	resolvedSlidePictures := make([]*Shape, len(entry.Slide.CSld.SpTree.Pictures))
	for i, pic := range entry.Slide.CSld.SpTree.Pictures {
		resolvedSlidePictures[i] = resolvePicture(&pic, resources, slideThemeColors)
	}
	resolvedSlideGroups := make([]*Shape, len(entry.Slide.CSld.SpTree.GroupShapes))
	for i, grp := range entry.Slide.CSld.SpTree.GroupShapes {
		resolvedSlideGroups[i] = resolveGroupShape(&grp, master, slideThemeColors, theme, rawTheme, entry.MediaParts, resources, tableStyles, presentationTextStyle)
	}
	resolvedSlideTables := make([]*Shape, len(entry.Slide.CSld.SpTree.Tables))
	for i, tbl := range entry.Slide.CSld.SpTree.Tables {
		resolvedSlideTables[i] = resolveTableFrame(&tbl, slideThemeColors, theme, rawTheme, tableStyles, presentationTextStyle)
	}
	resolvedOleObjects := make([]*Shape, len(entry.Slide.CSld.SpTree.OleObjectFrames))
	for i, ole := range entry.Slide.CSld.SpTree.OleObjectFrames {
		resolvedOleObjects[i] = resolveOleObjectFrame(&ole, resources)
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
					if cascade.HeaderFooterPlaceholderEnabled(master, PlaceholderFooter) {
						slide.Footer = shape
					}
				case PlaceholderDateTime:
					if cascade.HeaderFooterPlaceholderEnabled(master, PlaceholderDateTime) {
						slide.DateTime = shape
					}
				case PlaceholderSlideNumber:
					if cascade.HeaderFooterPlaceholderEnabled(master, PlaceholderSlideNumber) {
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
			shape := resolveConnector(&cx, slideThemeColors, rawTheme, shapeIndex)
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
	// Project slide-level non-visual semantics at the OOXML→IR boundary.
	project.ApplySlideMetadata(entry.Slide, slide)
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
