package legacytoooxml

import (
	"fmt"
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/media"
	"office/pkg/ooxml/smartart"
	presentationir "office/pkg/presentation/ir"
	presparts "office/pkg/presentation/parsers/ooxml/parts"
)

// ── Theme ────────────────────────────────────────────────────────────────────

func (c *converter) mapTheme(theme *presentationir.Theme) {
	if theme == nil {
		return
	}
	t := &presparts.Theme{
		Name: theme.Name,
	}
	if theme.ColorScheme != nil {
		t.Elements.ClrScheme = presparts.ColorScheme{}
		for slot, color := range theme.ColorScheme {
			if color == "" {
				continue
			}
			tc := &presparts.ThemeColor{
				SrgbClr: &presparts.SrgbColor{Val: color},
			}
			switch slot {
			case "dk1":
				t.Elements.ClrScheme.DK1 = tc
			case "lt1":
				t.Elements.ClrScheme.LT1 = tc
			case "dk2":
				t.Elements.ClrScheme.DK2 = tc
			case "lt2":
				t.Elements.ClrScheme.LT2 = tc
			case "accent1":
				t.Elements.ClrScheme.Accent1 = tc
			case "accent2":
				t.Elements.ClrScheme.Accent2 = tc
			case "accent3":
				t.Elements.ClrScheme.Accent3 = tc
			case "accent4":
				t.Elements.ClrScheme.Accent4 = tc
			case "accent5":
				t.Elements.ClrScheme.Accent5 = tc
			case "accent6":
				t.Elements.ClrScheme.Accent6 = tc
			case "hlink":
				t.Elements.ClrScheme.HLink = tc
			case "folHlink":
				t.Elements.ClrScheme.FolHLink = tc
			}
		}
	}
	if theme.MajorFont != "" || theme.MinorFont != "" ||
		theme.MajorEAFont != "" || theme.MinorEAFont != "" ||
		theme.MajorCSFont != "" || theme.MinorCSFont != "" {
		t.Elements.FontScheme = presparts.FontScheme{
			MajorFont: presparts.FontGroup{
				Latin: presparts.FontRef{Typeface: theme.MajorFont},
				EA:    presparts.FontRef{Typeface: theme.MajorEAFont},
				CS:    presparts.FontRef{Typeface: theme.MajorCSFont},
			},
			MinorFont: presparts.FontGroup{
				Latin: presparts.FontRef{Typeface: theme.MinorFont},
				EA:    presparts.FontRef{Typeface: theme.MinorEAFont},
				CS:    presparts.FontRef{Typeface: theme.MinorCSFont},
			},
		}
	}

	c.pkg.Themes["ppt/theme/theme1.xml"] = t
}

// ── Slides ───────────────────────────────────────────────────────────────────

func (c *converter) convertSlide(slide *presentationir.Slide, num int) error {
	relID := c.nextRID()
	partName := fmt.Sprintf("ppt/slides/slide%d.xml", num)

	ps := &presparts.Slide{
		CSld: presparts.CommonSlideData{
			SpTree: presparts.ShapeTree{},
		},
	}
	if slide.Transition != nil {
		ps.Transition = &presparts.SlideTransition{
			Type:     slide.Transition.Type,
			Spd:      slide.Transition.Spd,
			AdvClick: slide.Transition.AdvClick,
			AdvTmMs:  slide.Transition.AdvTmMs,
		}
	}

	// Background.
	if slide.BgColor != "" {
		ps.CSld.Bg = &presparts.SlideBg{
			BgPrSrgb: &drawingxml.SRGBClr{Val: slide.BgColor},
		}
	}

	entry := &presparts.SlideEntry{
		RelID:      relID,
		PartName:   partName,
		Slide:      ps,
		ChartParts: make(map[string][]byte),
	}

	// Shapes.
	c.shapeIDMap = make(map[*presentationir.Shape]int)
	c.currentEntry = entry
	for _, shape := range slide.Shapes {
		c.addShapeToTree(&ps.CSld.SpTree, shape)
	}
	for _, art := range slide.SmartArt {
		c.addSmartArtToTree(&ps.CSld.SpTree, art)
	}
	c.currentEntry = nil

	// Build animation maps from shape IDs assigned during conversion.
	// This recurses into group children so nested shape animations survive
	// write-back.
	c.collectAnimations(slide, ps)

	// Notes.
	if slide.Notes != nil {
		entry.Notes = c.convertNotesSlide(slide.Notes)
	}

	// Comments.
	if len(slide.Comments) > 0 {
		entry.Comments = c.convertSlideComments(slide.Comments)
	}

	entry.Layout = c.layoutForSlide(slide)

	c.pkg.Slides = append(c.pkg.Slides, entry)
	c.pkg.Presentation.SlideIDs = append(c.pkg.Presentation.SlideIDs, presparts.IDRef{
		ID:    num,
		RelID: relID,
	})

	return nil
}

func (c *converter) addShapeToTree(tree *presparts.ShapeTree, shape *presentationir.Shape) {
	switch shape.Kind {
	case presentationir.ShapeText:
		s := c.convertTextShape(shape)
		tree.Shapes = append(tree.Shapes, s)
		tree.Items = append(tree.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemShape, Index: len(tree.Shapes) - 1})
	case presentationir.ShapeImage:
		pic := c.convertImageShape(shape)
		tree.Pictures = append(tree.Pictures, pic)
		tree.Items = append(tree.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemPicture, Index: len(tree.Pictures) - 1})
	case presentationir.ShapeGroup:
		gs := c.convertGroupShape(shape)
		tree.GroupShapes = append(tree.GroupShapes, gs)
		tree.Items = append(tree.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemGroup, Index: len(tree.GroupShapes) - 1})
	case presentationir.ShapeChart:
		gf := c.convertChartShape(shape)
		tree.GraphicFrames = append(tree.GraphicFrames, gf)
		tree.Items = append(tree.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemGraphicFrame, Index: len(tree.GraphicFrames) - 1})
	case presentationir.ShapeConnector:
		cxn := c.convertConnector(shape)
		tree.Connectors = append(tree.Connectors, cxn)
		tree.Items = append(tree.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemConnector, Index: len(tree.Connectors) - 1})
	case presentationir.ShapeTable:
		tbl := c.convertTableShape(shape)
		tree.Tables = append(tree.Tables, tbl)
		tree.Items = append(tree.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemTable, Index: len(tree.Tables) - 1})
	case presentationir.ShapeMedia:
		pic := c.convertMediaShape(shape)
		tree.Pictures = append(tree.Pictures, pic)
		tree.Items = append(tree.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemPicture, Index: len(tree.Pictures) - 1})
	case presentationir.ShapeOleObject:
		ole := c.convertOLEObjectShape(shape)
		tree.OleObjectFrames = append(tree.OleObjectFrames, ole)
		tree.Items = append(tree.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemOleObject, Index: len(tree.OleObjectFrames) - 1})
	}
}

// collectAnimations walks all shapes (including group children) and copies
// entrance steps, emphasis effects, motion paths, and triggers into the slide
// part's flat animation maps.
func (c *converter) collectAnimations(slide *presentationir.Slide, ps *presparts.Slide) {
	var walk func(shape *presentationir.Shape)
	walk = func(shape *presentationir.Shape) {
		if shape == nil {
			return
		}
		if id, ok := c.shapeIDMap[shape]; ok {
			if shape.AnimStep > 0 {
				if ps.AnimSteps == nil {
					ps.AnimSteps = make(map[int]int)
				}
				ps.AnimSteps[id] = shape.AnimStep
			}
			if len(shape.EmphasisEffects) > 0 {
				if ps.EmphasisEffects == nil {
					ps.EmphasisEffects = make(map[int][]string)
				}
				ps.EmphasisEffects[id] = append([]string(nil), shape.EmphasisEffects...)
			}
			if shape.MotionPath != nil {
				if ps.MotionPaths == nil {
					ps.MotionPaths = make(map[int]presparts.AnimMotionPath)
				}
				ps.MotionPaths[id] = presparts.AnimMotionPath{
					Path:   shape.MotionPath.Path,
					Dur:    shape.MotionPath.Dur,
					Origin: shape.MotionPath.Origin,
				}
			}
			if len(shape.AnimationTriggers) > 0 {
				if ps.AnimationTriggers == nil {
					ps.AnimationTriggers = make(map[int][]string)
				}
				ps.AnimationTriggers[id] = append([]string(nil), shape.AnimationTriggers...)
			}
		}
		for _, child := range shape.Children {
			walk(child)
		}
	}
	for _, shape := range slide.Shapes {
		walk(shape)
	}
}

// ── Text Shapes ──────────────────────────────────────────────────────────────

func placeholderTypeString(pk presentationir.PlaceholderKind) string {
	switch pk {
	case presentationir.PlaceholderTitle:
		return "title"
	case presentationir.PlaceholderBody:
		return "body"
	case presentationir.PlaceholderSubtitle:
		return "subTitle"
	case presentationir.PlaceholderDateTime:
		return "dt"
	case presentationir.PlaceholderSlideNumber:
		return "sldNum"
	case presentationir.PlaceholderFooter:
		return "ftr"
	case presentationir.PlaceholderHeader:
		return "hdr"
	case presentationir.PlaceholderContent:
		return "obj"
	case presentationir.PlaceholderChart:
		return "chart"
	case presentationir.PlaceholderTable:
		return "tbl"
	case presentationir.PlaceholderPicture:
		return "pic"
	case presentationir.PlaceholderMedia:
		return "media"
	case presentationir.PlaceholderClipArt:
		return "clipArt"
	case presentationir.PlaceholderDiagram:
		return "dgm"
	case presentationir.PlaceholderSlideImage:
		return "sldImg"
	default:
		return ""
	}
}

func (c *converter) convertTextShape(shape *presentationir.Shape) presparts.Shape {
	id := c.nextShapeID()
	c.recordShapeID(shape, id)
	s := presparts.Shape{
		NvSpPr: presparts.NonVisualShapeProps{
			CNvPr: presparts.CnvPr{
				ID:         id,
				Name:       shape.Name,
				HlinkClick: c.convertHlinkClick(shape.HyperlinkURL, shape.ActionLink),
			},
			NvPr: presparts.PlaceholderPr{},
		},
		SpPr: presparts.ShapeProperties{
			Xfrm: &presparts.Transform2D{
				Off:   presparts.Offset{X: shape.Bounds.X, Y: shape.Bounds.Y},
				Ext:   presparts.Extents{CX: shape.Bounds.W, CY: shape.Bounds.H},
				Rot:   int(shape.RotationDeg * 60000),
				FlipH: shape.FlipH,
				FlipV: shape.FlipV,
			},
		},
	}

	if shape.Placeholder != presentationir.PlaceholderNone {
		s.NvSpPr.NvPr.Ph = &presparts.PlaceholderInfo{
			Type:            placeholderTypeString(shape.Placeholder),
			Idx:             shape.PlaceholderIdx,
			Orient:          shape.PlaceholderOrient,
			Size:            shape.PlaceholderSize,
			HasCustomPrompt: boolPtrIf(shape.PlaceholderHasCustomPrompt),
		}
	}

	if shape.Geometry != "" {
		s.SpPr.PrstGeom = &presparts.PresetGeometry{Prst: shape.Geometry}
	}

	if shape.FillColor != "" {
		s.SpPr.SolidFill = &presparts.ColorRef{Val: shape.FillColor}
	}

	if shape.LineColor != "" || shape.LineWidthPt > 0 {
		s.SpPr.Line = &presparts.ConnectorLine{
			W: int64(shape.LineWidthPt * 12700),
			SolidFill: &drawingxml.SolidFill{
				SRGBClr: &drawingxml.SRGBClr{Val: shape.LineColor},
			},
		}
	}

	if len(shape.Paragraphs) > 0 {
		s.TxBody = c.convertTextBody(shape)
	}

	return s
}

func (c *converter) convertTextBody(shape *presentationir.Shape) *presparts.TextBody {
	tb := c.convertParagraphsToTextBody(shape.Paragraphs)
	tb.BodyPr.Anchor = shape.TextAnchor
	tb.BodyPr.LIns = ptrInt64(shape.TextInsetLeftEMU)
	tb.BodyPr.RIns = ptrInt64(shape.TextInsetRightEMU)
	tb.BodyPr.TIns = ptrInt64(shape.TextInsetTopEMU)
	tb.BodyPr.BIns = ptrInt64(shape.TextInsetBottomEMU)
	if shape.TextAllowOverflow {
		tb.BodyPr.NoAutofit = &struct{}{}
	}
	if shape.TextAutoGrow {
		tb.BodyPr.SpAutoFit = &struct{}{}
	}
	return tb
}

func (c *converter) convertParagraphsToTextBody(paragraphs []*presentationir.Paragraph) *presparts.TextBody {
	tb := &presparts.TextBody{
		BodyPr: &presparts.DMLBodyPr{},
	}
	for _, para := range paragraphs {
		tb.Paragraphs = append(tb.Paragraphs, c.convertDMLParagraph(para))
	}
	return tb
}

func (c *converter) convertDMLParagraph(para *presentationir.Paragraph) presparts.DMLParagraph {
	dp := presparts.DMLParagraph{}
	if para.Align != "" || para.Level > 0 || para.MarginLeftEMU > 0 || para.TextIndentEMU > 0 {
		dp.Properties = &presparts.DMLParaProps{
			Algn: para.Align,
			Lvl:  para.Level,
		}
		if para.MarginLeftEMU > 0 {
			dp.Properties.MarL = ptrInt(int(para.MarginLeftEMU))
		}
		if para.TextIndentEMU > 0 {
			dp.Properties.Indent = ptrInt(int(para.TextIndentEMU))
		}
		if para.SpaceBefore > 0 {
			dp.Properties.SpcBef = &presparts.DMLSpacing{SpcPts: &presparts.SpacingPts{Val: para.SpaceBefore}}
		}
		if para.SpaceAfter > 0 {
			dp.Properties.SpcAft = &presparts.DMLSpacing{SpcPts: &presparts.SpacingPts{Val: para.SpaceAfter}}
		}
		if para.LineSpacingPct > 0 {
			dp.Properties.LnSpc = &presparts.DMLSpacing{SpcPct: &presparts.SpacingPct{Val: para.LineSpacingPct}}
		}
		if para.BulletChar != "" {
			dp.Properties.BuChar = &presparts.BuChar{Char: para.BulletChar}
		}
		if para.AutoNum {
			dp.Properties.BuAutoNum = &presparts.BuAutoNum{Type: para.AutoNumType, StartAt: para.AutoNumStart}
		}
	}
	for _, run := range para.Runs {
		dr := c.convertDMLRun(run)
		dp.Runs = append(dp.Runs, dr)
		dp.Inlines = append(dp.Inlines, presparts.DMLParaInline{
			Kind: presparts.DMLParaInlineRun,
			Run:  &dp.Runs[len(dp.Runs)-1],
		})
	}
	return dp
}

func (c *converter) convertDMLRun(run *presentationir.Run) presparts.DMLRun {
	dr := presparts.DMLRun{Text: run.Text}
	if run.Bold || run.Italic || run.Underline || run.Strike || run.FontSize > 0 ||
		run.FontFamily != "" || run.Color != "" || run.Highlight != "" || run.HyperlinkURL != "" {
		dr.Properties = &presparts.DMLRunPr{}
		if run.Bold {
			dr.Properties.Bold = &run.Bold
		}
		if run.Italic {
			dr.Properties.Italic = &run.Italic
		}
		if run.Underline {
			dr.Properties.Underline = "sng"
		}
		if run.Strike {
			dr.Properties.Strike = "sngStrike"
		}
		if run.FontSize > 0 {
			dr.Properties.FontSize = run.FontSize
		}
		if run.FontFamily != "" {
			dr.Properties.Latin = &presparts.FontRef{Typeface: run.FontFamily}
		}
		if run.Color != "" {
			dr.Properties.SolidFill = &presparts.ColorRef{Val: run.Color}
		}
		if run.HyperlinkURL != "" {
			dr.Properties.HlinkClick = c.convertHlinkClick(run.HyperlinkURL, "")
		}
	}
	return dr
}

// convertHlinkClick turns an IR hyperlink URL or action link into a parts
// HlinkClick. External URLs become slide-level relationships; action links are
// encoded as ppaction:// URIs and do not need a relationship.
func (c *converter) convertHlinkClick(url, action string) *presparts.HlinkClick {
	url = strings.TrimSpace(url)
	action = strings.TrimSpace(action)
	if url == "" && action == "" {
		return nil
	}
	if url != "" {
		rid := c.nextRID()
		if c.currentEntry != nil {
			if c.currentEntry.HlinkURLs == nil {
				c.currentEntry.HlinkURLs = make(map[string]string)
			}
			c.currentEntry.HlinkURLs[rid] = url
		}
		return &presparts.HlinkClick{RelID: rid}
	}
	return &presparts.HlinkClick{Action: buildActionURI(action)}
}

// buildActionURI converts a POI-style action keyword into the OOXML
// ppaction:// URI used by PowerPoint.
func buildActionURI(action string) string {
	switch action {
	case "firstSlide":
		return "ppaction://hlinkPresentationJump?jump=firstSlide"
	case "lastSlide":
		return "ppaction://hlinkPresentationJump?jump=lastSlide"
	case "nextSlide":
		return "ppaction://hlinkPresentationJump?jump=nextSlide"
	case "prevSlide", "previousSlide":
		return "ppaction://hlinkPresentationJump?jump=prevSlide"
	case "endShow":
		return "ppaction://hlinkshowjump?jump=endShow"
	}
	if strings.HasPrefix(action, "slide") {
		return "ppaction://hlinksldjump?slidenum=" + strings.TrimPrefix(action, "slide")
	}
	if strings.HasPrefix(action, "ppaction://") {
		return action
	}
	return ""
}

func imageExtensionForMIME(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])) {
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/bmp":
		return ".bmp"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".png"
	}
}

// ── Image Shapes ─────────────────────────────────────────────────────────────

func (c *converter) convertImageShape(shape *presentationir.Shape) presparts.Picture {
	id := c.nextShapeID()
	c.recordShapeID(shape, id)
	relID := c.nextRID()
	if shape.ImageData != nil {
		part := media.Part{
			Name:        fmt.Sprintf("ppt/media/image%d%s", c.nextMediaNum(), imageExtensionForMIME(shape.ImageMIME)),
			Data:        append([]byte(nil), shape.ImageData...),
			ContentType: shape.ImageMIME,
		}
		c.pkg.MediaParts[relID] = part
		if c.currentEntry != nil {
			if c.currentEntry.MediaParts == nil {
				c.currentEntry.MediaParts = make(map[string]media.Part)
			}
			c.currentEntry.MediaParts[relID] = part
		}
	}

	return presparts.Picture{
		NvPicPr: presparts.NonVisualPicProps{
			CNvPr: presparts.CnvPr{
				ID:         id,
				Name:       shape.Name,
				HlinkClick: c.convertHlinkClick(shape.HyperlinkURL, shape.ActionLink),
			},
		},
		BlipFill: presparts.BlipFill{
			Blip: &presparts.Blip{Embed: relID},
		},
		SpPr: presparts.ShapeProperties{
			Xfrm: &presparts.Transform2D{
				Off: presparts.Offset{X: shape.Bounds.X, Y: shape.Bounds.Y},
				Ext: presparts.Extents{CX: shape.Bounds.W, CY: shape.Bounds.H},
			},
		},
	}
}

// ── Group Shapes ─────────────────────────────────────────────────────────────

func (c *converter) convertGroupShape(shape *presentationir.Shape) presparts.GroupShape {
	id := c.nextShapeID()
	c.recordShapeID(shape, id)
	gs := presparts.GroupShape{
		NvGrpSpPr: &presparts.NonVisualGroupShapeProps{
			CNvPr: presparts.CnvPr{
				ID:   id,
				Name: shape.Name,
			},
		},
		GrpSpPr: &presparts.GroupShapeProperties{
			Xfrm: &presparts.Transform2D{
				Off: presparts.Offset{X: shape.Bounds.X, Y: shape.Bounds.Y},
				Ext: presparts.Extents{CX: shape.Bounds.W, CY: shape.Bounds.H},
			},
		},
	}
	for _, child := range shape.Children {
		c.addShapeToTreeFromGroup(&gs, child)
	}
	return gs
}

func (c *converter) addShapeToTreeFromGroup(gs *presparts.GroupShape, shape *presentationir.Shape) {
	switch shape.Kind {
	case presentationir.ShapeText:
		s := c.convertTextShape(shape)
		gs.Shapes = append(gs.Shapes, s)
		gs.Items = append(gs.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemShape, Index: len(gs.Shapes) - 1})
	case presentationir.ShapeImage:
		pic := c.convertImageShape(shape)
		gs.Pictures = append(gs.Pictures, pic)
		gs.Items = append(gs.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemPicture, Index: len(gs.Pictures) - 1})
	case presentationir.ShapeGroup:
		g := c.convertGroupShape(shape)
		gs.GroupShapes = append(gs.GroupShapes, g)
		gs.Items = append(gs.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemGroup, Index: len(gs.GroupShapes) - 1})
	case presentationir.ShapeTable:
		tbl := c.convertTableShape(shape)
		gs.Tables = append(gs.Tables, tbl)
		gs.Items = append(gs.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemTable, Index: len(gs.Tables) - 1})
	case presentationir.ShapeConnector:
		cxn := c.convertConnector(shape)
		gs.Connectors = append(gs.Connectors, cxn)
		gs.Items = append(gs.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemConnector, Index: len(gs.Connectors) - 1})
	case presentationir.ShapeMedia:
		pic := c.convertMediaShape(shape)
		gs.Pictures = append(gs.Pictures, pic)
		gs.Items = append(gs.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemPicture, Index: len(gs.Pictures) - 1})
	case presentationir.ShapeOleObject:
		ole := c.convertOLEObjectShape(shape)
		gs.OleObjectFrames = append(gs.OleObjectFrames, ole)
		gs.Items = append(gs.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemOleObject, Index: len(gs.OleObjectFrames) - 1})
	}
}

func (c *converter) convertMediaShape(shape *presentationir.Shape) presparts.Picture {
	id := c.nextShapeID()
	c.recordShapeID(shape, id)
	relID := c.nextRID()
	mediaPartName := fmt.Sprintf("ppt/media/media%d.%s", c.nextMediaNum(), mediaExtension(shape.MediaMIME))
	if c.currentEntry != nil {
		if c.currentEntry.MediaParts == nil {
			c.currentEntry.MediaParts = make(map[string]media.Part)
		}
		c.currentEntry.MediaParts[relID] = media.Part{
			Name:        mediaPartName,
			ContentType: shape.MediaMIME,
			Data:        shape.MediaData,
		}
	}

	nvPr := presparts.ApplicationNonVisualDrawingProps{}
	mediaRef := &presparts.MediaReference{Embed: relID}
	if shape.MediaKind == "audio" {
		nvPr.AudioFile = mediaRef
	} else {
		nvPr.VideoFile = mediaRef
	}

	pic := presparts.Picture{
		NvPicPr: presparts.NonVisualPicProps{
			CNvPr: presparts.CnvPr{ID: id, Name: shape.Name},
			NvPr:  nvPr,
		},
		BlipFill: presparts.BlipFill{},
		SpPr: presparts.ShapeProperties{
			Xfrm: &presparts.Transform2D{
				Off: presparts.Offset{X: shape.Bounds.X, Y: shape.Bounds.Y},
				Ext: presparts.Extents{CX: shape.Bounds.W, CY: shape.Bounds.H},
			},
		},
	}

	if len(shape.MediaPosterData) > 0 {
		posterRelID := c.nextRID()
		posterPartName := fmt.Sprintf("ppt/media/poster%d.%s", c.nextMediaNum(), mediaExtension(shape.MediaPosterMIME))
		if c.currentEntry != nil {
			c.currentEntry.MediaParts[posterRelID] = media.Part{
				Name:        posterPartName,
				ContentType: shape.MediaPosterMIME,
				Data:        shape.MediaPosterData,
			}
		}
		pic.BlipFill.Blip = &presparts.Blip{Embed: posterRelID}
	}

	return pic
}

func mediaExtension(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "mp4"):
		return "mp4"
	case strings.Contains(mimeType, "mpeg"):
		return "mp3"
	case strings.Contains(mimeType, "wav"):
		return "wav"
	case strings.Contains(mimeType, "png"):
		return "png"
	case strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg"):
		return "jpg"
	default:
		return "bin"
	}
}

// ── Chart Shapes ─────────────────────────────────────────────────────────────

func oleObjectExtension(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "wordprocessingml.document"):
		return "docx"
	case strings.Contains(mimeType, "spreadsheetml.sheet"):
		return "xlsx"
	case strings.Contains(mimeType, "presentationml.presentation"):
		return "pptx"
	case strings.Contains(mimeType, "pdf"):
		return "pdf"
	default:
		return "bin"
	}
}

func (c *converter) convertOLEObjectShape(shape *presentationir.Shape) presparts.OleObjectFrame {
	id := c.nextShapeID()
	c.recordShapeID(shape, id)

	relID := c.nextRID()
	isLinked := strings.TrimSpace(shape.OLEObjectLinkURL) != ""
	if c.currentEntry != nil {
		if isLinked {
			if c.currentEntry.OleObjectLinks == nil {
				c.currentEntry.OleObjectLinks = make(map[string]string)
			}
			c.currentEntry.OleObjectLinks[relID] = shape.OLEObjectLinkURL
		} else {
			partName := fmt.Sprintf("ppt/embeddings/oleObject%d.%s", c.nextEmbedNum(), oleObjectExtension(shape.OLEObjectMIME))
			contentType := shape.OLEObjectMIME
			if contentType == "" {
				contentType = "application/vnd.openxmlformats-officedocument.oleObject"
			}
			if c.currentEntry.OleObjectParts == nil {
				c.currentEntry.OleObjectParts = make(map[string]media.Part)
			}
			c.currentEntry.OleObjectParts[relID] = media.Part{Name: partName, ContentType: contentType, Data: shape.OLEObjectData}
		}
	}

	iconRelID := ""
	if len(shape.OLEIconData) > 0 {
		iconRelID = c.nextRID()
		iconPartName := fmt.Sprintf("ppt/media/oleIcon%d.%s", c.nextMediaNum(), mediaExtension(shape.OLEIconMIME))
		if c.currentEntry != nil {
			if c.currentEntry.MediaParts == nil {
				c.currentEntry.MediaParts = make(map[string]media.Part)
			}
			c.currentEntry.MediaParts[iconRelID] = media.Part{
				Name:        iconPartName,
				ContentType: shape.OLEIconMIME,
				Data:        shape.OLEIconData,
			}
		}
	}

	objectName := shape.OLEObjectName
	if objectName == "" {
		objectName = "Object"
	}
	return presparts.OleObjectFrame{
		ID:         id,
		Name:       shape.Name,
		Xfrm:       &presparts.Transform2D{Off: presparts.Offset{X: shape.Bounds.X, Y: shape.Bounds.Y}, Ext: presparts.Extents{CX: shape.Bounds.W, CY: shape.Bounds.H}},
		RelID:      relID,
		Embed:      !isLinked,
		ShowAsIcon: len(shape.OLEIconData) > 0,
		ProgID:     shape.OLEObjectProgID,
		ObjectName: objectName,
		ImgW:       shape.Bounds.W,
		ImgH:       shape.Bounds.H,
		IconRelID:  iconRelID,
	}
}

func (c *converter) convertChartShape(shape *presentationir.Shape) presparts.GraphicFrame {
	id := c.nextShapeID()
	c.recordShapeID(shape, id)
	relID := c.nextRID()
	if c.currentEntry != nil && len(shape.ChartData) > 0 {
		c.currentEntry.ChartParts[relID] = shape.ChartData
	}
	return presparts.GraphicFrame{
		CNvPr: presparts.CnvPr{
			ID:   id,
			Name: shape.Name,
		},
		Graphic: presparts.Graphic{
			GraphicData: presparts.GraphicData{
				URI:   "http://schemas.openxmlformats.org/drawingml/2006/chart",
				Chart: &presparts.ChartRef{RelID: relID},
			},
		},
		Xfrm: &presparts.Transform2D{
			Off: presparts.Offset{X: shape.Bounds.X, Y: shape.Bounds.Y},
			Ext: presparts.Extents{CX: shape.Bounds.W, CY: shape.Bounds.H},
		},
	}
}

// ── Connectors ───────────────────────────────────────────────────────────────

func (c *converter) convertConnector(shape *presentationir.Shape) presparts.Connector {
	id := c.nextShapeID()
	c.recordShapeID(shape, id)
	line := &presparts.ConnectorLine{
		W: int64(shape.LineWidthPt * 12700),
		SolidFill: &drawingxml.SolidFill{
			SRGBClr: &drawingxml.SRGBClr{Val: shape.LineColor},
		},
	}
	if shape.ArrowHead != "" {
		line.HeadEnd = &presparts.LineEnd{Type: shape.ArrowHead}
	}
	if shape.ArrowTail != "" {
		line.TailEnd = &presparts.LineEnd{Type: shape.ArrowTail}
	}
	spPr := presparts.ConnectorProperties{
		Xfrm: &presparts.Transform2D{
			Off: presparts.Offset{X: shape.Bounds.X, Y: shape.Bounds.Y},
			Ext: presparts.Extents{CX: shape.Bounds.W, CY: shape.Bounds.H},
		},
		Line: line,
	}
	if shape.ConnectorType != "" {
		spPr.PrstGeom = &presparts.PresetGeometry{Prst: shape.ConnectorType}
	}
	return presparts.Connector{
		NvCxnSpPr: presparts.NonVisualCxnSpProps{
			CNvPr: presparts.CnvPr{
				ID:   id,
				Name: shape.Name,
			},
		},
		SpPr: spPr,
	}
}

// ── Tables ───────────────────────────────────────────────────────────────────

func (c *converter) convertTableShape(shape *presentationir.Shape) presparts.TableFrame {
	id := c.nextShapeID()
	c.recordShapeID(shape, id)
	table := &presparts.PresentationTable{}
	for _, w := range shape.Table.ColumnWidthsEMU {
		table.Grid.Columns = append(table.Grid.Columns, presparts.PresentationTableColumn{W: w})
	}
	for _, row := range shape.Table.Rows {
		tr := presparts.PresentationTableRow{H: row.HeightEMU}
		for _, cell := range row.Cells {
			tr.Cells = append(tr.Cells, c.convertTableCell(cell))
		}
		table.Rows = append(table.Rows, tr)
	}
	return presparts.TableFrame{
		ID:   id,
		Name: shape.Name,
		Xfrm: &presparts.Transform2D{
			Off: presparts.Offset{X: shape.Bounds.X, Y: shape.Bounds.Y},
			Ext: presparts.Extents{CX: shape.Bounds.W, CY: shape.Bounds.H},
		},
		Table: table,
	}
}

func (c *converter) convertTableCell(cell *presentationir.TableCell) presparts.PresentationTableCell {
	ptc := presparts.PresentationTableCell{
		GridSpan: cell.GridSpan,
		RowSpan:  cell.RowSpan,
		HMerge:   cell.HMerge,
		VMerge:   cell.VMerge,
		TextBody: c.convertParagraphsToTextBody(cell.Paragraphs),
	}

	if cell.FillColor != "" || cell.TextAnchor != "" ||
		cell.MarginLeftEMU != 0 || cell.MarginRightEMU != 0 ||
		cell.MarginTopEMU != 0 || cell.MarginBottomEMU != 0 ||
		cell.BorderLeft != nil || cell.BorderRight != nil ||
		cell.BorderTop != nil || cell.BorderBottom != nil {
		props := &presparts.PresentationTableCellProps{
			Anchor: cell.TextAnchor,
			MarL:   cell.MarginLeftEMU,
			MarR:   cell.MarginRightEMU,
			MarT:   cell.MarginTopEMU,
			MarB:   cell.MarginBottomEMU,
		}
		if cell.FillColor != "" {
			props.SolidFill = &presparts.ColorRef{Val: cell.FillColor}
		}
		if cell.BorderLeft != nil {
			props.LnL = c.convertTableCellBorder(cell.BorderLeft)
		}
		if cell.BorderRight != nil {
			props.LnR = c.convertTableCellBorder(cell.BorderRight)
		}
		if cell.BorderTop != nil {
			props.LnT = c.convertTableCellBorder(cell.BorderTop)
		}
		if cell.BorderBottom != nil {
			props.LnB = c.convertTableCellBorder(cell.BorderBottom)
		}
		ptc.Props = props
	}

	return ptc
}

func (c *converter) convertTableCellBorder(ls *drawing.LineStyle) *presparts.ConnectorLine {
	if ls == nil {
		return nil
	}
	line := &presparts.ConnectorLine{
		W: int64(ls.Width * 12700),
	}
	if ls.Color != "" {
		line.SolidFill = &drawingxml.SolidFill{
			SRGBClr: &drawingxml.SRGBClr{Val: ls.Color},
		}
	}
	return line
}

// ── Notes ────────────────────────────────────────────────────────────────────

func (c *converter) convertNotesSlide(notes *presentationir.NotesSlide) *presparts.NotesSlide {
	ns := &presparts.NotesSlide{
		CSld: presparts.CommonSlideData{
			SpTree: presparts.ShapeTree{},
		},
	}
	if notes.BgColor != "" {
		ns.CSld.Bg = &presparts.SlideBg{
			BgPrSrgb: &drawingxml.SRGBClr{Val: notes.BgColor},
		}
	}
	for _, shape := range notes.Shapes {
		c.addShapeToTree(&ns.CSld.SpTree, shape)
	}
	return ns
}

// ── Comments ─────────────────────────────────────────────────────────────────

func (c *converter) convertSlideComments(comments []*presentationir.Comment) []*presparts.PptxComment {
	var result []*presparts.PptxComment
	for _, comment := range comments {
		authorID := c.ensureCommentAuthor(comment.AuthorName, comment.Initials)
		result = append(result, &presparts.PptxComment{
			AuthorID: authorID,
			Index:    len(result) + 1,
			DateTime: comment.DateTime,
			Text:     comment.Text,
			X:        comment.X,
			Y:        comment.Y,
		})
	}
	return result
}

func (c *converter) ensureCommentAuthor(name, initials string) int {
	for id, author := range c.pkg.CommentAuthors {
		if author.Name == name {
			return id
		}
	}
	id := len(c.pkg.CommentAuthors) + 1
	if c.pkg.CommentAuthors == nil {
		c.pkg.CommentAuthors = make(map[int]presparts.CommentAuthor)
	}
	c.pkg.CommentAuthors[id] = presparts.CommentAuthor{
		ID:       id,
		Name:     name,
		Initials: initials,
	}
	return id
}

func (c *converter) convertEmbeddedFonts(fonts []*presentationir.EmbeddedFont) {
	for _, font := range fonts {
		if font == nil || strings.TrimSpace(font.Family) == "" {
			continue
		}
		data := &presparts.EmbeddedFontData{Typeface: strings.TrimSpace(font.Family)}
		for _, face := range font.Faces {
			if face == nil || len(face.Data) == 0 {
				continue
			}
			part := &presparts.FontPart{
				Name:        face.Name,
				ContentType: face.ContentType,
				Data:        append([]byte(nil), face.Data...),
			}
			bold := face.Weight >= 700
			italic := strings.EqualFold(face.Style, "italic")
			switch {
			case bold && italic:
				data.BoldItalic = part
			case bold:
				data.Bold = part
			case italic:
				data.Italic = part
			default:
				data.Regular = part
			}
		}
		if data.Regular != nil || data.Bold != nil || data.Italic != nil || data.BoldItalic != nil {
			c.pkg.EmbeddedFonts = append(c.pkg.EmbeddedFonts, data)
		}
	}
}

// ensureDefaultMasters creates a default slide master with a blank layout when
// the IR does not already contain one.
func (c *converter) ensureDefaultMasters(pres *presentationir.Presentation) {
	if pres == nil {
		return
	}
	if len(pres.SlideMasters) > 0 {
		return
	}
	master := &presentationir.SlideMaster{
		Name:    "Office Theme Slide Master",
		Shapes:  []*presentationir.Shape{},
		Layouts: []*presentationir.SlideLayout{},
	}
	master.Layouts = append(master.Layouts, &presentationir.SlideLayout{
		Type:   "blank",
		Name:   "Blank Layout",
		Master: master,
	})
	pres.SlideMasters = []*presentationir.SlideMaster{master}
}

// convertSlideMasters turns IR slide masters and layouts into parts entries.
func (c *converter) convertSlideMasters(masters []*presentationir.SlideMaster) {
	for mi, master := range masters {
		if master == nil {
			continue
		}
		masterPartName := fmt.Sprintf("ppt/slideMasters/slideMaster%d.xml", mi+1)
		entry := &presparts.SlideMasterEntry{
			PartName:   masterPartName,
			Master:     &presparts.SlideMaster{},
			MediaParts: make(map[string]media.Part),
		}
		if master.Name != "" {
			entry.Master.CSld.Name = master.Name
		}
		for _, shape := range master.Shapes {
			c.addShapeToTree(&entry.Master.CSld.SpTree, shape)
		}

		for li, layout := range master.Layouts {
			if layout == nil {
				continue
			}
			layoutPartName := fmt.Sprintf("ppt/slideLayouts/slideLayout%d.xml", li+1)
			partsLayout := &presparts.SlideLayout{
				PartName: layoutPartName,
				Type:     layout.Type,
			}
			if layout.Name != "" {
				partsLayout.MatchingName = layout.Name
			}
			for _, shape := range layout.Shapes {
				c.addShapeToTree(&partsLayout.CSld.SpTree, shape)
			}
			entry.Layouts = append(entry.Layouts, partsLayout)
		}

		c.pkg.SlideMasters = append(c.pkg.SlideMasters, entry)
	}
}

// layoutForSlide returns the parts layout that should be used for a slide.
// It prefers the slide's explicit Layout, then a layout whose Type matches
// LayoutType, and finally the first layout of the first master.
func (c *converter) layoutForSlide(slide *presentationir.Slide) *presparts.SlideLayout {
	if slide == nil || len(c.pkg.SlideMasters) == 0 {
		return nil
	}
	if slide.Layout != nil {
		for _, entry := range c.pkg.SlideMasters {
			for _, layout := range entry.Layouts {
				if layout != nil && layout.Type == slide.Layout.Type {
					return layout
				}
			}
		}
	}
	wantType := strings.ToLower(strings.TrimSpace(slide.LayoutType))
	if wantType == "" {
		wantType = "blank"
	}
	for _, entry := range c.pkg.SlideMasters {
		for _, layout := range entry.Layouts {
			if layout != nil && strings.ToLower(strings.TrimSpace(layout.Type)) == wantType {
				return layout
			}
		}
	}
	// Final fallback: first layout of first master.
	if len(c.pkg.SlideMasters[0].Layouts) > 0 {
		return c.pkg.SlideMasters[0].Layouts[0]
	}
	return nil
}

// addSmartArtToTree converts an IR SmartArt diagram into a SmartArt part and
// hosting p:graphicFrame and appends them to the slide tree.
func (c *converter) addSmartArtToTree(tree *presparts.ShapeTree, art *presentationir.SmartArt) {
	if art == nil {
		return
	}
	diagram := &smartart.Diagram{
		Type:  art.Type,
		Nodes: make([]*smartart.Node, len(art.Nodes)),
		Layout: &smartart.Layout{
			Type:      art.Type,
			Direction: "topDown",
			HSpacing:  50,
			VSpacing:  80,
		},
		Style: &smartart.Style{
			Fill:      drawing.SolidFill{Color: "4472C4", Alpha: 1.0},
			Line:      &drawing.LineStyle{Color: "2F5597", Width: 1.5, Alpha: 1.0},
			TextColor: "FFFFFF",
			ShapeType: "rectangle",
		},
	}
	if art.LayoutID != "" {
		diagram.Layout.Type = art.LayoutID
	}
	if art.FillColor != "" {
		diagram.Style.Fill = drawing.SolidFill{Color: strings.TrimPrefix(art.FillColor, "#"), Alpha: 1.0}
	}
	if art.LineColor != "" {
		diagram.Style.Line = &drawing.LineStyle{Color: strings.TrimPrefix(art.LineColor, "#"), Width: 1.5, Alpha: 1.0}
	}
	if art.TextColor != "" {
		diagram.Style.TextColor = strings.TrimPrefix(art.TextColor, "#")
	}
	nodeMap := make(map[string]*smartart.Node, len(art.Nodes))
	for i, n := range art.Nodes {
		node := &smartart.Node{
			ID:   n.ID,
			Type: n.Type,
			Text: n.Text,
		}
		node.PropertySet.LayoutTypeID = art.LayoutID
		node.PropertySet.QuickStyleTypeID = art.QuickStyleID
		node.PropertySet.ColorTypeID = art.ColorID
		if n.FillColor != "" {
			node.Fill = drawing.SolidFill{Color: strings.TrimPrefix(n.FillColor, "#"), Alpha: 1.0}
		}
		if n.TextColor != "" {
			node.TextColor = strings.TrimPrefix(n.TextColor, "#")
		}
		if node.Type == "" {
			if i == 0 {
				node.Type = "root"
			} else {
				node.Type = "node"
			}
		}
		diagram.Nodes[i] = node
		nodeMap[node.ID] = node
	}
	for _, cxn := range art.Connections {
		diagram.Connections = append(diagram.Connections, smartart.Connection{
			SrcID:  cxn.SrcID,
			DestID: cxn.DestID,
		})
		if parent, ok := nodeMap[cxn.SrcID]; ok {
			if child, ok := nodeMap[cxn.DestID]; ok {
				child.Parent = parent
				parent.Children = append(parent.Children, child)
			}
		}
	}

	c.smartArtNum++
	relID := c.nextRID()
	partName := fmt.Sprintf("ppt/diagrams/data%d.xml", c.smartArtNum)
	part := &presparts.SmartArtPart{
		RelID:        relID,
		PartName:     partName,
		DataModelXML: smartart.MarshalDataModel(diagram),
	}
	c.currentEntry.SmartArtParts = append(c.currentEntry.SmartArtParts, part)

	id := c.nextShapeID()
	name := fmt.Sprintf("Diagram %d", c.smartArtNum)
	frame := presparts.SmartArtFrame{
		ID:   id,
		Name: name,
		Xfrm: &presparts.Transform2D{
			Off: presparts.Offset{X: art.X, Y: art.Y},
			Ext: presparts.Extents{CX: art.W, CY: art.H},
		},
		RelIDs: map[string]string{"dm": relID},
		RawXML: buildSmartArtFrameRawXML(id, name, art.X, art.Y, art.W, art.H, relID),
	}
	idx := len(tree.SmartArtFrames)
	tree.SmartArtFrames = append(tree.SmartArtFrames, frame)
	tree.Items = append(tree.Items, presparts.ShapeTreeItem{Kind: presparts.ShapeTreeItemSmartArt, Index: idx})
}

func buildSmartArtFrameRawXML(id int, name string, x, y, w, h int64, relID string) []byte {
	return []byte(fmt.Sprintf(`<p:graphicFrame xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram">
  <p:nvGraphicFramePr>
    <p:cNvPr id="%d" name="%s"/>
    <p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr>
    <p:nvPr/>
  </p:nvGraphicFramePr>
  <p:xfrm>
    <a:off x="%d" y="%d"/>
    <a:ext cx="%d" cy="%d"/>
  </p:xfrm>
  <a:graphic>
    <a:graphicData uri="http://schemas.openxmlformats.org/drawing/2006/diagram">
      <dgm:relIds r:dm="%s"/>
    </a:graphicData>
  </a:graphic>
</p:graphicFrame>`, id, name, x, y, w, h, relID))
}
