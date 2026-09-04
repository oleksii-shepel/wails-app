package legacytoooxml

import (
	"fmt"
	"strings"

	"office/pkg/ooxml/chart"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	wordir "office/pkg/word/ir"
	wordparts "office/pkg/word/parsers/ooxml/parts"
)

// ── Core Properties ──────────────────────────────────────────────────────────

func (c *converter) mapCoreProperties(doc *wordir.Document) {
	if doc.Title != "" || doc.Subject != "" || doc.Author != "" ||
		doc.Description != "" || doc.Keywords != "" || doc.LastModifiedBy != "" {
		c.pkg.CoreProperties = &wordparts.CoreProperties{
			Title:          doc.Title,
			Subject:        doc.Subject,
			Creator:        doc.Author,
			Description:    doc.Description,
			LastModifiedBy: doc.LastModifiedBy,
		}
		if doc.Keywords != "" {
			c.pkg.CoreProperties.Keywords = &wordparts.CoreKeywords{Text: doc.Keywords}
		}
	}
	if doc.Company != "" || doc.Manager != "" || doc.AppVersion != "" || doc.Template != "" ||
		doc.Application != "" || doc.WordCount != 0 || doc.CharCount != 0 || doc.PageCount != 0 ||
		doc.ParagraphCount != 0 {
		c.pkg.ExtendedProperties = &wordparts.ExtendedProperties{
			Company:     doc.Company,
			Manager:     doc.Manager,
			AppVersion:  doc.AppVersion,
			Template:    doc.Template,
			Application: doc.Application,
			Words:       doc.WordCount,
			Characters:  doc.CharCount,
			Pages:       doc.PageCount,
			Paragraphs:  doc.ParagraphCount,
		}
	}
	if len(doc.CustomProperties) > 0 {
		c.pkg.CustomProperties = &wordparts.CustomProperties{}
		for name, value := range doc.CustomProperties {
			c.pkg.CustomProperties.Properties = append(c.pkg.CustomProperties.Properties, wordparts.CustomProperty{
				Name:  name,
				Value: value,
			})
		}
	}
}

// ── Paragraphs ───────────────────────────────────────────────────────────────

func (c *converter) convertParagraph(p *wordir.Paragraph) *wordparts.Paragraph {
	pp := &wordparts.Paragraph{
		Properties: c.convertParagraphProperties(p),
	}

	// Determine if we need inline-based marshal for SDT or revision marks.
	needsInlines := false
	for _, run := range p.Runs {
		if run.ContentControl != nil || run.Revision != "" {
			needsInlines = true
			break
		}
	}

	if needsInlines {
		c.buildInlines(pp, p.Runs)
	} else {
		// Group consecutive hyperlink runs into Hyperlink elements.
		var hyperlinkRuns []*wordparts.Run
		var hyperlinkTarget string
		var hyperlinkRID string
		flushHyperlink := func() {
			if len(hyperlinkRuns) == 0 || hyperlinkTarget == "" {
				hyperlinkRuns = nil
				hyperlinkTarget = ""
				hyperlinkRID = ""
				return
			}
			rid := hyperlinkRID
			if rid == "" {
				rid = c.hyperlinkRelID(hyperlinkTarget)
			} else {
				c.reserveRelID(rid)
				c.pkg.SetHyperlinkTarget(c.pkg.MainPartName, rid, hyperlinkTarget)
			}
			hl := &wordparts.Hyperlink{
				ID:   rid,
				Runs: append([]*wordparts.Run(nil), hyperlinkRuns...),
			}
			pp.Hyperlinks = append(pp.Hyperlinks, hl)
			hyperlinkRuns = nil
			hyperlinkTarget = ""
			hyperlinkRID = ""
		}
		for _, run := range p.Runs {
			if run.Kind == "hyperlink" || run.Target != "" {
				if hyperlinkTarget != "" && hyperlinkTarget != run.Target {
					flushHyperlink()
				}
				hyperlinkTarget = run.Target
				if run.RelID != "" {
					hyperlinkRID = run.RelID
				}
				hyperlinkRuns = append(hyperlinkRuns, c.convertRun(&run))
			} else {
				flushHyperlink()
				pp.Runs = append(pp.Runs, c.convertRun(&run))
			}
		}
		flushHyperlink()
	}

	for _, bm := range p.Bookmarks {
		pp.Bookmarks = append(pp.Bookmarks, bm)
		id := c.bookmarkID()
		pp.BookmarkStartElems = append(pp.BookmarkStartElems, wordparts.BookmarkStart{ID: id, Name: bm})
		pp.BookmarkEndElems = append(pp.BookmarkEndElems, wordparts.BookmarkEnd{ID: id})
	}
	return pp
}

func (c *converter) buildInlines(pp *wordparts.Paragraph, runs []wordir.Run) {
	var hyperlinkRuns []*wordparts.Run
	var hyperlinkTarget string
	var hyperlinkRID string
	flushHyperlink := func() {
		if len(hyperlinkRuns) == 0 || hyperlinkTarget == "" {
			hyperlinkRuns = nil
			hyperlinkTarget = ""
			hyperlinkRID = ""
			return
		}
		rid := hyperlinkRID
		if rid == "" {
			rid = c.hyperlinkRelID(hyperlinkTarget)
		} else {
			c.reserveRelID(rid)
			c.pkg.SetHyperlinkTarget(c.pkg.MainPartName, rid, hyperlinkTarget)
		}
		hl := &wordparts.Hyperlink{
			ID:   rid,
			Runs: append([]*wordparts.Run(nil), hyperlinkRuns...),
		}
		pp.Hyperlinks = append(pp.Hyperlinks, hl)
		pp.Inlines = append(pp.Inlines, wordparts.ParagraphInline{
			Kind:      wordparts.ParagraphInlineHyperlink,
			Hyperlink: hl,
		})
		hyperlinkRuns = nil
		hyperlinkTarget = ""
		hyperlinkRID = ""
	}
	for i := range runs {
		run := &runs[i]
		pr := c.convertRun(run)

		if run.Kind == "hyperlink" || run.Target != "" {
			if hyperlinkTarget != "" && hyperlinkTarget != run.Target {
				flushHyperlink()
			}
			hyperlinkTarget = run.Target
			if run.RelID != "" {
				hyperlinkRID = run.RelID
			}
			hyperlinkRuns = append(hyperlinkRuns, pr)
			continue
		}
		flushHyperlink()

		if run.ContentControl != nil {
			pp.Inlines = append(pp.Inlines, wordparts.ParagraphInline{
				Kind:     wordparts.ParagraphInlineSDT,
				SdtProps: c.convertSdtProperties(run.ContentControl),
				SdtInlines: []wordparts.ParagraphInline{
					{Kind: wordparts.ParagraphInlineRun, Run: pr},
				},
			})
			continue
		}

		if run.Revision != "" {
			switch run.Revision {
			case "inserted":
				pp.Inlines = append(pp.Inlines, wordparts.ParagraphInline{
					Kind:           wordparts.ParagraphInlineIns,
					Run:            pr,
					RevisionAuthor: run.RevisionAuthor,
					RevisionDate:   run.RevisionDate,
				})
			case "deleted":
				pp.Inlines = append(pp.Inlines, wordparts.ParagraphInline{
					Kind:           wordparts.ParagraphInlineDel,
					Run:            pr,
					RevisionAuthor: run.RevisionAuthor,
					RevisionDate:   run.RevisionDate,
				})
			case "movedFrom":
				pp.Inlines = append(pp.Inlines, wordparts.ParagraphInline{
					Kind:           wordparts.ParagraphInlineMoveFrom,
					Run:            pr,
					RevisionAuthor: run.RevisionAuthor,
					RevisionDate:   run.RevisionDate,
					MoveID:         run.MoveID,
				})
			case "movedTo":
				pp.Inlines = append(pp.Inlines, wordparts.ParagraphInline{
					Kind:           wordparts.ParagraphInlineMoveTo,
					Run:            pr,
					RevisionAuthor: run.RevisionAuthor,
					RevisionDate:   run.RevisionDate,
					MoveID:         run.MoveID,
				})
			default:
				pp.Runs = append(pp.Runs, pr)
				pp.Inlines = append(pp.Inlines, wordparts.ParagraphInline{
					Kind: wordparts.ParagraphInlineRun,
					Run:  pr,
				})
			}
			continue
		}

		pp.Runs = append(pp.Runs, pr)
		pp.Inlines = append(pp.Inlines, wordparts.ParagraphInline{
			Kind: wordparts.ParagraphInlineRun,
			Run:  pr,
		})
	}
	flushHyperlink()
}

func (c *converter) convertSdtProperties(cc *wordir.ContentControl) *wordparts.SdtProperties {
	props := &wordparts.SdtProperties{
		ID:    cc.ID,
		Alias: cc.Title,
		Tag:   cc.Tag,
		Type:  cc.Type,
	}
	if cc.Checked {
		props.Checked = true
	}
	if cc.DateFormat != "" {
		props.DateFormat = cc.DateFormat
	}
	for _, item := range cc.ListItems {
		props.ListItems = append(props.ListItems, wordparts.SdtListItem{DisplayText: item.DisplayText, Value: item.Value})
	}
	if cc.ShowingPlaceholder {
		props.ShowingPlcHdr = true
	}
	if cc.DataXPath != "" {
		props.DataXPath = cc.DataXPath
	}
	if cc.DataStoreID != "" {
		props.DataStoreID = cc.DataStoreID
	}
	return props
}

func (c *converter) convertParagraphProperties(p *wordir.Paragraph) *wordparts.ParagraphProperties {
	pf := &p.Format
	pp := &wordparts.ParagraphProperties{}

	if p.StyleID != "" {
		pp.Style = stringVal(p.StyleID)
	}
	if pf.Alignment != "" {
		pp.Justification = stringVal(pf.Alignment)
	}
	if pf.SpacingBefore != nil || pf.SpacingAfter != nil || pf.LineSpacing != nil {
		pp.Spacing = &wordparts.Spacing{}
		if pf.SpacingBefore != nil {
			pp.Spacing.Before = pf.SpacingBefore
		}
		if pf.SpacingAfter != nil {
			pp.Spacing.After = pf.SpacingAfter
		}
		if pf.LineSpacing != nil {
			pp.Spacing.Line = pf.LineSpacing
			pp.Spacing.LineRule = pf.LineRule
			if pp.Spacing.LineRule == "" {
				pp.Spacing.LineRule = "auto"
			}
		}
	}
	if pf.IndentLeft != nil || pf.IndentRight != nil || pf.IndentFirstLine != nil || pf.IndentHanging != nil {
		pp.Indentation = &wordparts.Indentation{}
		if pf.IndentLeft != nil {
			pp.Indentation.Left = pf.IndentLeft
		}
		if pf.IndentRight != nil {
			pp.Indentation.Right = pf.IndentRight
		}
		if pf.IndentFirstLine != nil {
			pp.Indentation.FirstLine = pf.IndentFirstLine
		}
		if pf.IndentHanging != nil {
			pp.Indentation.Hanging = pf.IndentHanging
		}
	}
	if pf.Shading != "" {
		pp.Shading = &wordparts.Shading{
			Val:  "clear",
			Fill: pf.Shading,
		}
	}
	if pf.KeepNext {
		pp.KeepNext = onOffBool(true)
	}
	if pf.KeepLines {
		pp.KeepLines = onOffBool(true)
	}
	if pf.WidowControl {
		pp.WidowControl = onOffBool(true)
	}
	if pf.PageBreakBefore {
		pp.PageBreakBefore = onOffBool(true)
	}
	if pf.ContextualSpacing {
		pp.ContextualSpacing = onOffBool(true)
	}
	if pf.SnapToGrid {
		pp.SnapToGrid = onOffBool(true)
	}
	if pf.Bidi {
		pp.Bidi = onOffBool(true)
	}
	if pf.Borders != nil {
		pp.Borders = c.convertParagraphBorders(pf.Borders)
	}
	if p.Numbering != nil {
		pp.Numbering = &wordparts.NumberingReference{
			Level: intVal(p.Numbering.Level),
			NumID: intVal(p.Numbering.NumID),
		}
	}
	if p.Frame != nil {
		pp.Frame = c.convertFramePr(p.Frame)
	}
	if len(pf.TabStops) > 0 {
		for _, ts := range pf.TabStops {
			pp.Tabs = append(pp.Tabs, wordparts.TabStop{
				Val: ts.Algn,
				Pos: ts.Pos,
			})
		}
	}
	if pf.OutlineLevel != nil {
		pp.OutlineLvl = intVal(*pf.OutlineLevel)
	}
	if pf.NoLineNumber {
		pp.SuppressLineNumbers = onOffBool(true)
	}
	if pf.Kinsoku {
		pp.Kinsoku = onOffBool(true)
	}
	if pf.WordWrap != nil && *pf.WordWrap {
		pp.WordWrap = onOffBool(true)
	}
	if pf.OverflowPunct {
		pp.OverflowPunct = onOffBool(true)
	}
	if pf.TopLinePunct {
		pp.TopLinePunct = onOffBool(true)
	}
	if pf.AutoSpaceDE {
		pp.AutoSpaceDE = onOffBool(true)
	}
	if pf.AutoSpaceDN {
		pp.AutoSpaceDN = onOffBool(true)
	}
	if pf.MirrorIndents {
		pp.MirrorIndents = onOffBool(true)
	}
	if pf.NoAllowOverlap {
		pp.NoAllowOverlap = onOffBool(true)
	}
	if pf.AdjustRightInd {
		pp.AdjustRightInd = onOffBool(true)
	}
	if pf.FontAlign != "" {
		pp.TextAlignment = stringVal(pf.FontAlign)
	}
	pp.PropertyChange = convertPropertyRevision(p.PropertyRevision)

	return pp
}

func (c *converter) convertParagraphBorders(b *wordir.ParagraphBorders) *wordparts.ParagraphBorders {
	pb := &wordparts.ParagraphBorders{}
	if b.Top != nil {
		pb.Top = c.convertBorder(b.Top)
	}
	if b.Left != nil {
		pb.Left = c.convertBorder(b.Left)
	}
	if b.Bottom != nil {
		pb.Bottom = c.convertBorder(b.Bottom)
	}
	if b.Right != nil {
		pb.Right = c.convertBorder(b.Right)
	}
	if b.Between != nil {
		pb.Between = c.convertBorder(b.Between)
	}
	if b.Bar != nil {
		pb.Bar = c.convertBorder(b.Bar)
	}
	return pb
}

func (c *converter) convertBorder(b *wordir.BorderSide) *wordparts.Border {
	if b == nil {
		return nil
	}
	return &wordparts.Border{
		Val:   b.Val,
		Size:  int(b.SizePt * 8),
		Space: int(b.SpacePt),
		Color: b.Color,
	}
}

func (c *converter) convertPageBorders(pb *wordir.PageBorders) *wordparts.PageBorders {
	return &wordparts.PageBorders{
		Top:        c.convertBorder(pb.Top),
		Left:       c.convertBorder(pb.Left),
		Bottom:     c.convertBorder(pb.Bottom),
		Right:      c.convertBorder(pb.Right),
		Display:    pb.Display,
		OffsetFrom: pb.OffsetFrom,
		ZOrder:     pb.ZOrder,
	}
}

func (c *converter) convertFramePr(f *wordir.FrameProps) *wordparts.FramePr {
	fp := &wordparts.FramePr{
		XAlign:  f.XAlign,
		YAlign:  f.YAlign,
		HAnchor: f.HAnchor,
		VAnchor: f.VAnchor,
		Wrap:    f.WrapType,
		DropCap: f.DropCap,
	}
	if f.W > 0 {
		fp.W = ptrInt(f.W)
	}
	if f.H > 0 {
		fp.H = ptrInt(f.H)
	}
	if f.X > 0 {
		fp.X = ptrInt(f.X)
	}
	if f.Y > 0 {
		fp.Y = ptrInt(f.Y)
	}
	if f.HSpace > 0 {
		fp.HSpace = ptrInt(f.HSpace)
	}
	if f.VSpace > 0 {
		fp.VSpace = ptrInt(f.VSpace)
	}
	if f.Lines > 0 {
		fp.Lines = ptrInt(f.Lines)
	}
	return fp
}

// ── Runs ─────────────────────────────────────────────────────────────────────

func (c *converter) convertRun(r *wordir.Run) *wordparts.Run {
	pr := &wordparts.Run{
		Properties: c.convertRunProperties(r),
	}

	switch r.Kind {
	case "break":
		pr.Contents = append(pr.Contents, wordparts.RunContent{
			Kind:  wordparts.RunContentBreak,
			Break: &wordparts.Break{Type: r.BreakType, Clear: r.BreakClear},
		})
	case "tab":
		pr.Contents = append(pr.Contents, wordparts.RunContent{
			Kind: wordparts.RunContentTab,
		})
	case "footnoteRef":
		pr.Contents = append(pr.Contents, wordparts.RunContent{
			Kind:    wordparts.RunContentFootnoteRef,
			NoteRef: &wordparts.NoteReference{ID: r.NoteID},
		})
	case "endnoteRef":
		pr.Contents = append(pr.Contents, wordparts.RunContent{
			Kind:    wordparts.RunContentEndnoteRef,
			NoteRef: &wordparts.NoteReference{ID: r.NoteID},
		})
	case "commentRef":
		pr.Contents = append(pr.Contents, wordparts.RunContent{
			Kind:       wordparts.RunContentCommentRef,
			CommentRef: &wordparts.CommentReference{ID: r.CommentID},
		})
	case "image":
		if r.Image != nil {
			pr.Contents = append(pr.Contents, c.convertImage(r.Image))
		}
	case "textBox":
		if r.TextBox != nil {
			pr.Contents = append(pr.Contents, c.convertTextBox(r.TextBox))
		}
	case "formField":
		if r.FormField != nil {
			pr.Contents = append(pr.Contents, c.convertFormField(r.FormField, r.Text)...)
		}
	case "ruby":
		if r.Ruby != nil {
			pr.Contents = append(pr.Contents, wordparts.RunContent{
				Kind: wordparts.RunContentRuby,
				Ruby: &wordparts.RunRuby{
					Base: r.Ruby.Base,
					Text: r.Ruby.Text,
				},
			})
		}
	case "math":
		if r.MathML != "" {
			pr.Contents = append(pr.Contents, wordparts.RunContent{
				Kind:   wordparts.RunContentMath,
				MathML: r.MathML,
			})
		}
	case "shape":
		if r.Shape != nil {
			pr.Contents = append(pr.Contents, c.convertShape(r.Shape))
		}
	case "chart":
		if r.Chart != nil {
			pr.Contents = append(pr.Contents, c.convertChart(r.Chart))
		}
	case "olePlaceholder":
		pr.Contents = append(pr.Contents, c.convertOLEObject(r))
	default:
		if r.Text != "" {
			pr.Text = append(pr.Text, &wordparts.Text{
				Value: r.Text,
				Space: "preserve",
			})
			pr.Contents = append(pr.Contents, wordparts.RunContent{
				Kind: wordparts.RunContentText,
				Text: &wordparts.Text{Value: r.Text, Space: "preserve"},
			})
		}
	}

	if r.FieldInstr != "" {
		// Wrap as a complex field: begin → instrText → separate → result → end.
		var fieldContents []wordparts.RunContent
		fieldContents = append(fieldContents, wordparts.RunContent{
			Kind:    wordparts.RunContentFldChar,
			FldChar: &wordparts.FldChar{Type: "begin"},
		})
		fieldContents = append(fieldContents, wordparts.RunContent{
			Kind:      wordparts.RunContentInstrText,
			InstrText: r.FieldInstr,
		})
		fieldContents = append(fieldContents, wordparts.RunContent{
			Kind:    wordparts.RunContentFldChar,
			FldChar: &wordparts.FldChar{Type: "separate"},
		})
		fieldContents = append(fieldContents, pr.Contents...)
		fieldContents = append(fieldContents, wordparts.RunContent{
			Kind:    wordparts.RunContentFldChar,
			FldChar: &wordparts.FldChar{Type: "end"},
		})
		pr.Contents = fieldContents
	}

	return pr
}

func (c *converter) convertRunProperties(r *wordir.Run) *wordparts.RunProperties {
	f := &r.Format
	rp := &wordparts.RunProperties{}
	if r.StyleID != "" {
		rp.RunStyle = stringVal(r.StyleID)
	}

	if f.Bold != nil && *f.Bold {
		rp.Bold = onOffPtr(f.Bold)
	}
	if f.Italic != nil && *f.Italic {
		rp.Italic = onOffPtr(f.Italic)
	}
	if f.Caps != nil && *f.Caps {
		rp.Caps = onOffPtr(f.Caps)
	}
	if f.SmallCaps != nil && *f.SmallCaps {
		rp.SmallCaps = onOffPtr(f.SmallCaps)
	}
	if f.Strike != nil && *f.Strike {
		rp.Strike = onOffPtr(f.Strike)
	}
	if f.DoubleStrike != nil && *f.DoubleStrike {
		rp.DoubleStrike = onOffPtr(f.DoubleStrike)
	}
	if f.Underline != "" {
		rp.Underline = &wordparts.Underline{Val: f.Underline}
		if f.UnderlineColor != "" {
			rp.Underline.Color = f.UnderlineColor
		}
	}
	if f.Color != "" {
		rp.Color = &wordparts.RunColor{Val: f.Color}
	}
	if f.Background != "" {
		rp.Shading = &wordparts.Shading{Val: "clear", Fill: f.Background}
	}
	if f.FontSize != nil && *f.FontSize > 0 {
		rp.FontSize = intVal(*f.FontSize)
	}
	if f.FontASCII != "" || f.FontHAnsi != "" || f.FontEastAsia != "" || f.FontCS != "" {
		rp.Fonts = &wordparts.RunFonts{
			ASCII:    f.FontASCII,
			HAnsi:    f.FontHAnsi,
			EastAsia: f.FontEastAsia,
			CS:       f.FontCS,
		}
	}
	if f.Highlight != "" {
		rp.Highlight = stringVal(f.Highlight)
	}
	if f.VertAlignVal != "" {
		rp.VertAlign = stringVal(f.VertAlignVal)
	}
	if f.Vanish != nil && *f.Vanish {
		rp.Vanish = onOffPtr(f.Vanish)
	}
	if f.NoProof != nil && *f.NoProof {
		rp.NoProof = onOffPtr(f.NoProof)
	}
	if f.Lang != "" {
		rp.Lang = &wordparts.RunLang{Val: f.Lang}
	}
	if f.CharacterSpacing != nil && *f.CharacterSpacing != 0 {
		rp.Spacing = intVal(*f.CharacterSpacing)
	}
	if f.RTL != nil && *f.RTL {
		rp.RTL = onOffPtr(f.RTL)
	}
	if f.WebHidden != nil && *f.WebHidden {
		rp.WebHidden = onOffPtr(f.WebHidden)
	}
	if f.Kerning != nil && *f.Kerning > 0 {
		rp.Kerning = intVal(*f.Kerning)
	}
	if f.CharScale != nil && *f.CharScale != 0 {
		rp.CharScale = intVal(*f.CharScale)
	}
	if f.FitTextWidth != nil && *f.FitTextWidth > 0 {
		rp.FitText = intVal(*f.FitTextWidth)
	}
	if f.EmphasisMark != "" {
		rp.EmphasisMark = stringVal(f.EmphasisMark)
	}
	if f.Outline != nil && *f.Outline {
		rp.OutlineToggle = onOffPtr(f.Outline)
	}
	if f.Shadow != nil && *f.Shadow {
		rp.ShadowToggle = onOffPtr(f.Shadow)
	}
	if f.Emboss != nil && *f.Emboss {
		rp.Emboss = onOffPtr(f.Emboss)
	}
	if f.Imprint != nil && *f.Imprint {
		rp.Imprint = onOffPtr(f.Imprint)
	}
	if f.ComplexScript != nil && *f.ComplexScript {
		rp.CS = onOffPtr(f.ComplexScript)
	}
	if f.Borders != nil {
		rp.Borders = c.convertRunBorders(f.Borders)
	}
	if f.BoldCS != nil && *f.BoldCS {
		rp.BoldCS = onOffPtr(f.BoldCS)
	}
	if f.ItalicCS != nil && *f.ItalicCS {
		rp.ItalicCS = onOffPtr(f.ItalicCS)
	}
	if f.FontSizeCS != nil {
		rp.FontSizeCS = intVal(*f.FontSizeCS)
	}
	rp.PropertyChange = convertPropertyRevision(r.PropertyRevision)

	return rp
}

func (c *converter) convertRunBorders(b *wordir.RunBorders) *wordparts.Border {
	// OOXML w:bdr is a single border for all sides; legacy SPRM stores four sides.
	// Pick the first non-nil side as the uniform run border.
	for _, side := range []*wordir.BorderSide{b.Top, b.Left, b.Bottom, b.Right} {
		if side != nil {
			return c.convertBorder(side)
		}
	}
	return nil
}

func (c *converter) convertImage(img *wordir.Image) wordparts.RunContent {
	relID := c.nextRID()
	if img.Data != nil {
		c.pkg.SetMediaPart(c.pkg.MainPartName, relID, wordparts.MediaPart{
			ContentType: img.MIMEType,
			Data:        img.Data,
		}, "")
	}

	extent := &wordparts.DrawingExtent{CX: img.Width, CY: img.Height}
	docPr := &wordparts.DrawingDocPr{
		ID:     1,
		Name:   firstNonEmpty(img.Alt, "Image"),
		Title:  img.Title,
		Hidden: img.Hidden,
	}
	picture := &wordparts.Picture{
		BlipFill: &wordparts.BlipFill{
			Blip: &wordparts.Blip{Embed: relID},
		},
	}
	if img.Tile {
		picture.BlipFill.Tile = &drawingxml.Tile{}
	}
	if img.Stretch {
		picture.BlipFill.Stretch = &drawingxml.Stretch{}
	}
	if img.CropLeft != 0 || img.CropTop != 0 || img.CropRight != 0 || img.CropBottom != 0 {
		picture.BlipFill.SrcRect = &wordparts.SrcRect{
			L: int(img.CropLeft * 100000),
			T: int(img.CropTop * 100000),
			R: int(img.CropRight * 100000),
			B: int(img.CropBottom * 100000),
		}
	}
	if img.RotationDeg != 0 || img.FlipH || img.FlipV {
		picture.ShapeProps = &wordparts.PictureShapeProps{
			Xfrm: &wordparts.DrawingTransform2D{
				Rot:   int(img.RotationDeg * 60000),
				FlipH: img.FlipH,
				FlipV: img.FlipV,
				ExtCX: img.Width,
				ExtCY: img.Height,
			},
		}
	}
	graphic := &wordparts.Graphic{
		GraphicData: &wordparts.GraphicData{
			URI:     "http://schemas.openxmlformats.org/drawingml/2006/picture",
			Picture: picture,
		},
	}

	if !img.Floating {
		return wordparts.RunContent{
			Kind: wordparts.RunContentDrawing,
			Drawing: &wordparts.Drawing{
				Inline: &wordparts.DrawingInline{
					Extent:  extent,
					DocPr:   docPr,
					Graphic: graphic,
				},
			},
		}
	}

	anchor := &wordparts.DrawingAnchor{
		RelativeHeight: img.ZIndex,
		Locked:         img.Locked,
		Extent:         extent,
		DocPr:          docPr,
		Graphic:        graphic,
	}
	if img.BehindDoc {
		anchor.BehindDoc = 1
	}
	if img.DistTop != 0 {
		anchor.DistT = int64(img.DistTop * 12700)
	}
	if img.DistBottom != 0 {
		anchor.DistB = int64(img.DistBottom * 12700)
	}
	if img.DistLeft != 0 {
		anchor.DistL = int64(img.DistLeft * 12700)
	}
	if img.DistRight != 0 {
		anchor.DistR = int64(img.DistRight * 12700)
	}
	if img.SimplePosX != 0 || img.SimplePosY != 0 {
		anchor.SimplePosAttr = true
		anchor.SimplePos = &wordparts.SimplePos{X: img.SimplePosX, Y: img.SimplePosY}
	}

	if img.XAlign != "" {
		anchor.PositionH = &wordparts.DrawingPosition{
			RelativeFrom: img.XRelative,
			Align:        img.XAlign,
		}
	} else if img.X != 0 || img.XRelative != "" {
		anchor.PositionH = &wordparts.DrawingPosition{
			RelativeFrom: img.XRelative,
			PosOffset:    img.X,
		}
	}
	if img.YAlign != "" {
		anchor.PositionV = &wordparts.DrawingPosition{
			RelativeFrom: img.YRelative,
			Align:        img.YAlign,
		}
	} else if img.Y != 0 || img.YRelative != "" {
		anchor.PositionV = &wordparts.DrawingPosition{
			RelativeFrom: img.YRelative,
			PosOffset:    img.Y,
		}
	}

	if img.EffectExtentL != 0 || img.EffectExtentT != 0 || img.EffectExtentR != 0 || img.EffectExtentB != 0 {
		anchor.EffectExtent = &wordparts.EffectExtent{
			L: img.EffectExtentL,
			T: img.EffectExtentT,
			R: img.EffectExtentR,
			B: img.EffectExtentB,
		}
	}

	switch img.WrapType {
	case "none":
		anchor.WrapNone = &wordparts.DrawingWrapEmpty{
			DrawingWrapDistances: wordparts.DrawingWrapDistances{
				DistT: anchor.DistT,
				DistB: anchor.DistB,
			},
		}
	case "square":
		anchor.WrapSquare = &wordparts.DrawingWrapSided{
			WrapText: img.WrapSide,
			DrawingWrapDistances: wordparts.DrawingWrapDistances{
				DistT: anchor.DistT,
				DistB: anchor.DistB,
				DistL: anchor.DistL,
				DistR: anchor.DistR,
			},
		}
	case "tight":
		anchor.WrapTight = c.buildWrapTightThrough(img, anchor)
	case "through":
		anchor.WrapThrough = c.buildWrapTightThrough(img, anchor)
	case "topAndBottom":
		anchor.WrapTopAndBottom = &wordparts.DrawingWrapEmpty{
			DrawingWrapDistances: wordparts.DrawingWrapDistances{
				DistT: anchor.DistT,
				DistB: anchor.DistB,
			},
		}
	}

	return wordparts.RunContent{
		Kind: wordparts.RunContentDrawing,
		Drawing: &wordparts.Drawing{
			Anchor: anchor,
		},
	}
}

func (c *converter) buildWrapTightThrough(img *wordir.Image, anchor *wordparts.DrawingAnchor) *wordparts.DrawingWrapSided {
	wrap := &wordparts.DrawingWrapSided{
		WrapText: img.WrapSide,
		DrawingWrapDistances: wordparts.DrawingWrapDistances{
			DistT: anchor.DistT,
			DistB: anchor.DistB,
			DistL: anchor.DistL,
			DistR: anchor.DistR,
		},
	}
	if len(img.WrapPolygon) > 0 {
		wrap.Polygon = &wordparts.WrapPolygon{
			Start: wordparts.WrapPolygonPt{
				X: int64(img.WrapPolygon[0][0] * 12700),
				Y: int64(img.WrapPolygon[0][1] * 12700),
			},
		}
		for i := 1; i < len(img.WrapPolygon); i++ {
			wrap.Polygon.LineTo = append(wrap.Polygon.LineTo, wordparts.WrapPolygonPt{
				X: int64(img.WrapPolygon[i][0] * 12700),
				Y: int64(img.WrapPolygon[i][1] * 12700),
			})
		}
	}
	return wrap
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (c *converter) convertOLEObject(r *wordir.Run) wordparts.RunContent {
	if r == nil {
		return wordparts.RunContent{}
	}
	relID := r.OLERelID
	if relID == "" {
		relID = c.nextRID()
	} else {
		c.reserveRelID(relID)
	}
	if r.OLELinkURL != "" {
		c.pkg.SetOLELink(c.pkg.MainPartName, relID, r.OLELinkURL)
	} else if len(r.OLEData) > 0 {
		c.pkg.SetOLEPart(c.pkg.MainPartName, relID, wordparts.OLEPart{ContentType: r.OLEMIME, Data: r.OLEData}, "")
	}
	previewRelID := ""
	if r.OLEPreview != nil && len(r.OLEPreview.Data) > 0 {
		previewRelID = c.nextRID()
		c.pkg.SetMediaPart(c.pkg.MainPartName, previewRelID, wordparts.MediaPart{ContentType: r.OLEPreview.MIMEType, Data: append([]byte(nil), r.OLEPreview.Data...)}, "")
	}
	typ := r.OLEType
	if typ == "" {
		if r.OLELinkURL != "" {
			typ = "Link"
		} else {
			typ = "Embed"
		}
	}
	return wordparts.RunContent{Kind: wordparts.RunContentOLE, OLE: &wordparts.OLEObjectRun{ProgID: r.OLEProgID, RelID: relID, PreviewRelID: previewRelID, Type: typ, DrawAspect: r.OLEDrawAspect, ObjectID: r.OLEObjectID, UpdateMode: r.OLEUpdateMode}}
}

func (c *converter) convertChart(ch *chart.Chart) wordparts.RunContent {
	chartXML, err := chart.MarshalChart(ch)
	if err != nil {
		// Best-effort: return empty drawing on marshal failure.
		return wordparts.RunContent{}
	}
	relID := c.nextRID()
	c.pkg.SetChartPart(c.pkg.MainPartName, relID, chartXML, "")

	// Default size: 5 inches × 3 inches (in EMU).
	widthEMU := int64(5 * 914400)
	heightEMU := int64(3 * 914400)

	return wordparts.RunContent{
		Kind:       wordparts.RunContentDrawing,
		DrawingXML: makeInlineChartDrawingXML(relID, widthEMU, heightEMU),
	}
}

func makeInlineChartDrawingXML(relID string, widthEMU, heightEMU int64) []byte {
	aNS := "http://schemas.openxmlformats.org/drawingml/2006/main"
	cNS := "http://schemas.openxmlformats.org/drawingml/2006/chart"
	rNS := "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	return []byte(fmt.Sprintf(
		`<wp:inline distT="0" distB="0" distL="0" distR="0">`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:docPr id="1" name="Chart 1"/>`+
			`<wp:cNvGraphicFramePr>`+
			`<a:graphicFrameLocks noChangeAspect="1" xmlns:a="%s"/>`+
			`</wp:cNvGraphicFramePr>`+
			`<a:graphic xmlns:a="%s">`+
			`<a:graphicData uri="%s">`+
			`<c:chart xmlns:c="%s" r:id="%s" xmlns:r="%s"/>`+
			`</a:graphicData>`+
			`</a:graphic>`+
			`</wp:inline>`,
		widthEMU, heightEMU, aNS, aNS, cNS, cNS, relID, rNS,
	))
}

func (c *converter) convertShape(shape *wordir.Shape) wordparts.RunContent {
	var b strings.Builder
	graphic := c.buildShapeGraphicXML(shape)

	if shape.Floating {
		b.WriteString(`<wp:anchor xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" xmlns:wpg="http://schemas.microsoft.com/office/word/2010/wordprocessingGroup" xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"`)
		if shape.BehindDoc {
			b.WriteString(` behindDoc="1"`)
		}
		if shape.Locked {
			b.WriteString(` locked="1"`)
		}
		if shape.LayoutInCell {
			b.WriteString(` layoutInCell="1"`)
		}
		if shape.AllowOverlap {
			b.WriteString(` allowOverlap="1"`)
		}
		b.WriteString(`>`)

		if shape.SimplePosX != 0 || shape.SimplePosY != 0 {
			b.WriteString(fmt.Sprintf(`<wp:simplePos x="%d" y="%d"/>`, shape.SimplePosX, shape.SimplePosY))
		}

		if shape.XAlign != "" {
			b.WriteString(fmt.Sprintf(`<wp:positionH relativeFrom="%s"><wp:align>%s</wp:align></wp:positionH>`, firstNonEmpty(shape.XRelative, "page"), shape.XAlign))
		} else if shape.X != 0 || shape.XRelative != "" {
			b.WriteString(fmt.Sprintf(`<wp:positionH relativeFrom="%s"><wp:posOffset>%d</wp:posOffset></wp:positionH>`, firstNonEmpty(shape.XRelative, "page"), shape.X))
		}
		if shape.YAlign != "" {
			b.WriteString(fmt.Sprintf(`<wp:positionV relativeFrom="%s"><wp:align>%s</wp:align></wp:positionV>`, firstNonEmpty(shape.YRelative, "page"), shape.YAlign))
		} else if shape.Y != 0 || shape.YRelative != "" {
			b.WriteString(fmt.Sprintf(`<wp:positionV relativeFrom="%s"><wp:posOffset>%d</wp:posOffset></wp:positionV>`, firstNonEmpty(shape.YRelative, "page"), shape.Y))
		}

		b.WriteString(fmt.Sprintf(`<wp:extent cx="%d" cy="%d"/>`, shape.Width, shape.Height))

		if shape.EffectExtentL != 0 || shape.EffectExtentT != 0 || shape.EffectExtentR != 0 || shape.EffectExtentB != 0 {
			b.WriteString(fmt.Sprintf(`<wp:effectExtent l="%d" t="%d" r="%d" b="%d"/>`, shape.EffectExtentL, shape.EffectExtentT, shape.EffectExtentR, shape.EffectExtentB))
		}

		if shape.ZIndex != 0 {
			b.WriteString(fmt.Sprintf(`<wp:wrapNone/>`))
		}

		switch shape.WrapType {
		case "none":
			b.WriteString(`<wp:wrapNone/>`)
		case "square":
			b.WriteString(fmt.Sprintf(`<wp:wrapSquare wrapText="%s" distT="%d" distB="%d" distL="%d" distR="%d"/>`, firstNonEmpty(shape.WrapSide, "bothSides"), int64(shape.DistTop*12700), int64(shape.DistBottom*12700), int64(shape.DistLeft*12700), int64(shape.DistRight*12700)))
		case "tight", "through":
			b.WriteString(fmt.Sprintf(`<wp:wrapTight wrapText="%s" distT="%d" distB="%d" distL="%d" distR="%d"/>`, firstNonEmpty(shape.WrapSide, "bothSides"), int64(shape.DistTop*12700), int64(shape.DistBottom*12700), int64(shape.DistLeft*12700), int64(shape.DistRight*12700)))
		case "topAndBottom":
			b.WriteString(fmt.Sprintf(`<wp:wrapTopAndBottom distT="%d" distB="%d"/>`, int64(shape.DistTop*12700), int64(shape.DistBottom*12700)))
		default:
			b.WriteString(`<wp:wrapSquare wrapText="bothSides"/>`)
		}

		b.WriteString(graphic)
		b.WriteString(`</wp:anchor>`)
	} else {
		b.WriteString(`<wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" distT="0" distB="0" distL="0" distR="0">`)
		b.WriteString(fmt.Sprintf(`<wp:extent cx="%d" cy="%d"/>`, shape.Width, shape.Height))
		b.WriteString(`<wp:docPr id="1" name="Shape"/>`)
		b.WriteString(`<wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr>`)
		b.WriteString(graphic)
		b.WriteString(`</wp:inline>`)
	}

	return wordparts.RunContent{
		Kind:       wordparts.RunContentDrawing,
		DrawingXML: []byte(b.String()),
	}
}

func (c *converter) buildShapeGraphicXML(shape *wordir.Shape) string {
	var b strings.Builder
	b.WriteString(`<a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">`)
	b.WriteString(`<wps:wsp xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"><wps:cNvCnPr/><wps:cNvPr id="0" name=""/><wps:spPr>`)

	// Transform
	b.WriteString(`<a:xfrm`)
	if shape.RotationDeg != 0 {
		b.WriteString(fmt.Sprintf(` rot="%d"`, int(shape.RotationDeg*60000)))
	}
	if shape.FlipH {
		b.WriteString(` flipH="1"`)
	}
	if shape.FlipV {
		b.WriteString(` flipV="1"`)
	}
	b.WriteString(fmt.Sprintf(`><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, shape.Width, shape.Height))

	// Geometry
	geom := "rect"
	if shape.Geometry != nil && shape.Geometry.Name != "" {
		geom = shape.Geometry.Name
	}
	b.WriteString(fmt.Sprintf(`<a:prstGeom prst="%s"><a:avLst/></a:prstGeom>`, geom))

	// Fill — prefer dml.Fill when available, fall back to legacy FillColor.
	if shape.Fill != nil {
		b.WriteString(emitFillXML(shape.Fill))
	} else if shape.FillColor != "" {
		b.WriteString(fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, shape.FillColor))
	}

	// Line — prefer dml.LineStyle when available, fall back to legacy StrokeColor/StrokeWidth.
	if shape.LineStyle != nil {
		b.WriteString(emitLineStyleXML(shape.LineStyle))
	} else if shape.StrokeColor != "" || shape.StrokeWidth > 0 {
		b.WriteString(`<a:ln`)
		if shape.StrokeWidth > 0 {
			b.WriteString(fmt.Sprintf(` w="%d"`, shape.StrokeWidth))
		}
		b.WriteString(`>`)
		if shape.StrokeColor != "" {
			b.WriteString(fmt.Sprintf(`<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, shape.StrokeColor))
		}
		b.WriteString(`</a:ln>`)
	}

	// Effects
	b.WriteString(emitEffectListXML(shape.Effects))

	// 3D / bevel
	for _, eff := range shape.Effects.Effects {
		if td, ok := eff.(drawing.ThreeD); ok {
			b.WriteString(emitScene3DXML(&td))
			break
		}
		if td, ok := eff.(*drawing.ThreeD); ok && td != nil {
			b.WriteString(emitScene3DXML(td))
			break
		}
	}

	b.WriteString(`</wps:spPr>`)
	b.WriteString(emitBodyPrXML(shape.TextBox))
	b.WriteString(`</wps:wsp>`)
	b.WriteString(`</a:graphicData></a:graphic>`)
	return b.String()
}

func emitBodyPrXML(tb *wordir.TextBox) string {
	if tb == nil {
		return `<wps:bodyPr/>`
	}
	var b strings.Builder
	b.WriteString(`<wps:bodyPr`)
	ptToEmu := func(pt float64) int64 {
		return int64(pt * 12700)
	}
	const defL = 91440 // 7.2 pt
	const defT = 45720 // 3.6 pt
	const defR = 91440 // 7.2 pt
	const defB = 45720 // 3.6 pt
	if tb.PaddingLeft != 0 {
		b.WriteString(fmt.Sprintf(` lIns="%d"`, ptToEmu(tb.PaddingLeft)))
	}
	if tb.PaddingTop != 0 {
		b.WriteString(fmt.Sprintf(` tIns="%d"`, ptToEmu(tb.PaddingTop)))
	}
	if tb.PaddingRight != 0 {
		b.WriteString(fmt.Sprintf(` rIns="%d"`, ptToEmu(tb.PaddingRight)))
	}
	if tb.PaddingBottom != 0 {
		b.WriteString(fmt.Sprintf(` bIns="%d"`, ptToEmu(tb.PaddingBottom)))
	}
	if tb.Vert != "" {
		b.WriteString(fmt.Sprintf(` vert="%s"`, tb.Vert))
	}
	if tb.Anchor != "" {
		switch tb.Anchor {
		case "top":
			b.WriteString(` anchor="t"`)
		case "center":
			b.WriteString(` anchor="ctr"`)
		case "bottom":
			b.WriteString(` anchor="b"`)
		}
	}
	if tb.AnchorCenter {
		b.WriteString(` anchorCtr="1"`)
	}
	if tb.TextRotation != 0 {
		b.WriteString(fmt.Sprintf(` rot="%d"`, int(tb.TextRotation*60000)))
	}
	if tb.Upright {
		b.WriteString(` upright="1"`)
	}
	if tb.NumCol > 0 {
		b.WriteString(fmt.Sprintf(` numCol="%d"`, tb.NumCol))
	}
	if tb.SpcCol > 0 {
		b.WriteString(fmt.Sprintf(` spcCol="%d"`, ptToEmu(tb.SpcCol)))
	}
	if tb.RtlCol {
		b.WriteString(` rtlCol="1"`)
	}
	b.WriteString(`>`)
	switch tb.Autofit {
	case "none":
		b.WriteString(`<a:noAutofit/>`)
	case "shape":
		b.WriteString(`<a:spAutoFit/>`)
	case "norm":
		b.WriteString(`<a:normAutofit`)
		if tb.FontScale != 0 {
			b.WriteString(fmt.Sprintf(` fontScale="%d"`, tb.FontScale))
		}
		if tb.LnSpcReduction != 0 {
			b.WriteString(fmt.Sprintf(` lnSpcReduction="%d"`, tb.LnSpcReduction))
		}
		b.WriteString(`/>`)
	}
	if tb.PrstTxWarp != "" {
		b.WriteString(fmt.Sprintf(`<a:prstTxWarp prst="%s"/>`, tb.PrstTxWarp))
	}
	if tb.FlatTx {
		b.WriteString(`<a:flatTx/>`)
	}
	b.WriteString(`</wps:bodyPr>`)
	return b.String()
}

func (c *converter) convertTextBox(tb *wordir.TextBox) wordparts.RunContent {
	var paragraphs []*wordparts.Paragraph
	for _, block := range tb.Blocks {
		if block.Paragraph != nil {
			paragraphs = append(paragraphs, c.convertParagraph(block.Paragraph))
		}
	}
	if len(paragraphs) == 0 {
		paragraphs = append(paragraphs, &wordparts.Paragraph{})
	}
	return wordparts.RunContent{
		Kind: wordparts.RunContentVMLTextBox,
		VMLTextBox: &wordparts.TextBoxContent{
			Paragraphs: paragraphs,
		},
	}
}

func (c *converter) convertFormField(ff *wordir.FormField, text string) []wordparts.RunContent {
	ffd := &wordparts.FormFieldData{
		Name:             ff.Name,
		Kind:             ff.Type,
		Enabled:          ff.Enabled,
		CalcOnExit:       ff.CalcOnExit,
		DefaultTxt:       ff.DefaultText,
		MaxLength:        ff.MaxLength,
		TextType:         ff.TextType,
		TextFormat:       ff.TextFormat,
		Checked:          ff.Checked,
		CheckboxSize:     ff.CheckboxSize,
		CheckboxSizeAuto: ff.CheckboxSizeAuto,
		DropItems:        append([]string(nil), ff.Items...),
		DropDefIdx:       ff.DefaultIdx,
	}
	var contents []wordparts.RunContent
	contents = append(contents, wordparts.RunContent{
		Kind:    wordparts.RunContentFldChar,
		FldChar: &wordparts.FldChar{Type: "begin", FormField: ffd},
	})
	contents = append(contents, wordparts.RunContent{
		Kind:    wordparts.RunContentFldChar,
		FldChar: &wordparts.FldChar{Type: "separate"},
	})
	if text != "" {
		contents = append(contents, wordparts.RunContent{
			Kind: wordparts.RunContentText,
			Text: &wordparts.Text{Value: text, Space: "preserve"},
		})
	}
	contents = append(contents, wordparts.RunContent{
		Kind:    wordparts.RunContentFldChar,
		FldChar: &wordparts.FldChar{Type: "end"},
	})
	return contents
}

// ── Tables ───────────────────────────────────────────────────────────────────

func (c *converter) convertTable(t *wordir.Table) *wordparts.Table {
	pt := &wordparts.Table{
		Properties: c.convertTableProperties(&t.Format),
	}
	pt.Properties.PropertyChange = convertPropertyRevision(t.PropertyRevision)
	if len(t.Format.ColumnWidths) > 0 {
		pt.Grid = &wordparts.TableGrid{Columns: t.Format.ColumnWidths}
	}
	for _, row := range t.Rows {
		pt.Rows = append(pt.Rows, c.convertTableRow(&row))
	}
	return pt
}

func (c *converter) convertTableProperties(tf *wordir.TableFormat) *wordparts.TableProperties {
	tp := &wordparts.TableProperties{}
	if tf.Width != nil {
		tp.Width = &wordparts.TableMeasure{
			Width: *tf.Width,
			Type:  tf.WidthType,
		}
	}
	if tf.Alignment != "" {
		tp.Alignment = stringVal(tf.Alignment)
	}
	if tf.Layout != "" {
		tp.Layout = &wordparts.TableLayout{Type: tf.Layout}
	}
	if tf.Shading != "" {
		tp.Shading = &wordparts.Shading{Val: "clear", Fill: tf.Shading}
	}
	if tf.Borders != nil {
		tp.Borders = c.convertTableBorders(tf.Borders)
	}
	if tf.Indent != 0 {
		tp.Indent = &wordparts.TableMeasure{Width: tf.Indent, Type: "dxa"}
	}
	if tf.CellSpacing != 0 {
		tp.CellSpacing = &wordparts.TableMeasure{Width: tf.CellSpacing, Type: "dxa"}
	}
	if tf.CellPadTop != 0 || tf.CellPadBottom != 0 || tf.CellPadLeft != 0 || tf.CellPadRight != 0 {
		tp.CellMargin = &wordparts.TableCellMargin{}
		if tf.CellPadTop != 0 {
			tp.CellMargin.Top = &wordparts.TableMeasure{Width: tf.CellPadTop, Type: "dxa"}
		}
		if tf.CellPadBottom != 0 {
			tp.CellMargin.Bottom = &wordparts.TableMeasure{Width: tf.CellPadBottom, Type: "dxa"}
		}
		if tf.CellPadLeft != 0 {
			tp.CellMargin.Left = &wordparts.TableMeasure{Width: tf.CellPadLeft, Type: "dxa"}
		}
		if tf.CellPadRight != 0 {
			tp.CellMargin.Right = &wordparts.TableMeasure{Width: tf.CellPadRight, Type: "dxa"}
		}
	}
	if tf.FloatProps != nil {
		tp.FloatProps = c.convertTableFloatProps(tf.FloatProps)
	}
	if tf.Overlap != "" {
		tp.Overlap = stringVal(tf.Overlap)
	}
	if tf.Caption != "" {
		tp.Caption = tf.Caption
	}
	if tf.Description != "" {
		tp.Description = tf.Description
	}
	if tf.BidiVisual {
		tp.BidiVisual = onOffBool(true)
	}
	return tp
}

func (c *converter) convertTableBorders(b *wordir.TableBorders) *wordparts.TableBorders {
	return &wordparts.TableBorders{
		Top:     c.convertBorder(b.Top),
		Left:    c.convertBorder(b.Left),
		Start:   c.convertBorder(b.Start),
		Bottom:  c.convertBorder(b.Bottom),
		Right:   c.convertBorder(b.Right),
		End:     c.convertBorder(b.End),
		InsideH: c.convertBorder(b.InsideH),
		InsideV: c.convertBorder(b.InsideV),
	}
}

func (c *converter) convertTableFloatProps(f *wordir.TableFloatProps) *wordparts.TableFloatProps {
	return &wordparts.TableFloatProps{
		HorzAnchor:     f.HorzAnchor,
		VertAnchor:     f.VertAnchor,
		X:              f.X,
		Y:              f.Y,
		XAlign:         f.XAlign,
		YAlign:         f.YAlign,
		LeftFromText:   f.LeftFromText,
		RightFromText:  f.RightFromText,
		TopFromText:    f.TopFromText,
		BottomFromText: f.BottomFromText,
	}
}

func (c *converter) convertTableRow(row *wordir.TableRow) *wordparts.TableRow {
	pr := &wordparts.TableRow{
		Properties: c.convertTableRowProperties(&row.Format),
	}
	pr.Properties.PropertyChange = convertPropertyRevision(row.PropertyRevision)
	for _, cell := range row.Cells {
		pr.Cells = append(pr.Cells, c.convertTableCell(&cell))
	}
	return pr
}

func (c *converter) convertTableRowProperties(rf *wordir.TableRowFormat) *wordparts.TableRowProperties {
	trp := &wordparts.TableRowProperties{}
	if rf.Height != nil {
		trp.Height = &wordparts.TableRowHeight{
			Val:  *rf.Height,
			Rule: rf.HeightRule,
		}
	}
	if rf.IsHeader {
		trp.IsHeader = onOffBool(true)
	}
	if rf.CantSplit {
		trp.CantSplit = onOffBool(true)
	}
	if rf.Hidden {
		trp.Hidden = onOffBool(true)
	}
	if rf.GridBefore > 0 {
		trp.GridBefore = intVal(rf.GridBefore)
	}
	if rf.GridAfter > 0 {
		trp.GridAfter = intVal(rf.GridAfter)
	}
	if rf.WidthBefore != nil {
		trp.WidthBefore = &wordparts.TableMeasure{Width: *rf.WidthBefore, Type: rf.WidthBeforeType}
	}
	if rf.WidthAfter != nil {
		trp.WidthAfter = &wordparts.TableMeasure{Width: *rf.WidthAfter, Type: rf.WidthAfterType}
	}
	if rf.Justification != "" {
		trp.Justification = stringVal(rf.Justification)
	}
	if rf.Conditional != "" {
		trp.Conditional = stringVal(rf.Conditional)
	}
	return trp
}

func (c *converter) convertTableCell(cell *wordir.TableCell) *wordparts.TableCell {
	pc := &wordparts.TableCell{
		Properties: c.convertTableCellProperties(&cell.Format),
	}
	pc.Properties.PropertyChange = convertPropertyRevision(cell.PropertyRevision)
	pc.Properties.CellIns = convertTrackedCellChange(cell.CellIns)
	pc.Properties.CellDel = convertTrackedCellChange(cell.CellDel)
	pc.Properties.CellMerge = convertTrackedCellChange(cell.CellMerge)
	for _, para := range cell.Paragraphs {
		pc.Paragraphs = append(pc.Paragraphs, c.convertParagraph(&para))
	}
	for _, tbl := range cell.Tables {
		pc.Tables = append(pc.Tables, c.convertTable(&tbl))
	}
	return pc
}

func (c *converter) convertTableCellProperties(cf *wordir.TableCellFormat) *wordparts.TableCellProperties {
	cp := &wordparts.TableCellProperties{}
	if cf.Width != nil {
		cp.Width = &wordparts.TableMeasure{Width: *cf.Width, Type: cf.WidthType}
	}
	if cf.GridSpan > 1 {
		cp.GridSpan = intVal(cf.GridSpan)
	}
	if cf.VMerge != "" {
		cp.VMerge = &wordparts.VMerge{Val: cf.VMerge}
	}
	if cf.HMerge {
		cp.HMerge = &wordparts.VMerge{Val: "restart"}
	}
	if cf.VertAlign != "" {
		cp.VertAlign = stringVal(cf.VertAlign)
	}
	if cf.TextDirection != "" {
		cp.TextDirection = stringVal(cf.TextDirection)
	}
	if cf.Shading != "" {
		cp.Shading = &wordparts.Shading{Val: "clear", Fill: cf.Shading}
	}
	if cf.Borders != nil {
		cp.Borders = c.convertCellBorders(cf.Borders)
	}
	return cp
}

func (c *converter) convertCellBorders(b *wordir.CellBorders) *wordparts.TableCellBorders {
	return &wordparts.TableCellBorders{
		Top:    c.convertBorder(b.Top),
		Left:   c.convertBorder(b.Left),
		Start:  c.convertBorder(b.Start),
		Bottom: c.convertBorder(b.Bottom),
		Right:  c.convertBorder(b.Right),
		End:    c.convertBorder(b.End),
		TL2BR:  c.convertBorder(b.TL2BR),
		TR2BL:  c.convertBorder(b.TR2BL),
	}
}

func convertTrackedCellChange(mark *wordir.RevisionMark) *wordparts.TrackedCellChange {
	if mark == nil || (mark.Author == "" && mark.Date == "") {
		return nil
	}
	return &wordparts.TrackedCellChange{
		Author: mark.Author,
		Date:   mark.Date,
	}
}

func convertPropertyRevision(mark *wordir.RevisionMark) *wordparts.PropertyChangeInfo {
	if mark == nil || (mark.Author == "" && mark.Date == "") {
		return nil
	}
	return &wordparts.PropertyChangeInfo{
		Author: mark.Author,
		Date:   mark.Date,
	}
}

// ── Body ─────────────────────────────────────────────────────────────────────

func (c *converter) convertBody(doc *wordir.Document) {
	if len(doc.Blocks) > 0 {
		// Use Blocks for correct paragraph/table interleaving.
		// Group consecutive blocks sharing the same ContentControl into BodyBlockSDT.
		for i := 0; i < len(doc.Blocks); i++ {
			block := &doc.Blocks[i]
			if block.FieldKind == "AUTOTEXT" && block.FieldIndex >= 0 && block.FieldIndex < len(doc.Fields) {
				end := i
				for end+1 < len(doc.Blocks) && doc.Blocks[end+1].FieldKind == "AUTOTEXT" && doc.Blocks[end+1].FieldIndex == block.FieldIndex {
					end++
				}
				if c.emitAutoTextFieldBlocks(doc, i, end, &doc.Fields[block.FieldIndex]) {
					i = end
					continue
				}
			}
			if block.ContentControl != nil {
				// Collect all consecutive blocks with the same ContentControl.
				var sdtBlocks []wordparts.BodyBlock
				cc := block.ContentControl
				for ; i < len(doc.Blocks) && doc.Blocks[i].ContentControl != nil && contentControlsEqual(doc.Blocks[i].ContentControl, cc); i++ {
					inner := &doc.Blocks[i]
					if inner.Kind == "section" && inner.Section != nil {
						sp := c.convertSection(doc, inner.Section)
						sdtBlocks = append(sdtBlocks, wordparts.BodyBlock{Kind: wordparts.BodyBlockSection, Section: sp})
					} else {
						sdtBlocks = append(sdtBlocks, c.convertBodyBlock(doc, inner))
					}
				}
				i-- // undo last increment
				c.pkg.Document.Body.Blocks = append(c.pkg.Document.Body.Blocks, wordparts.BodyBlock{
					Kind:      wordparts.BodyBlockSDT,
					SdtProps:  c.convertSdtProperties(cc),
					SdtBlocks: sdtBlocks,
				})
				continue
			}
			c.emitBodyBlock(doc, block, i)
		}
	} else {
		// Fallback for IR built without Blocks (tests, hand-constructed docs).
		for i := range doc.Paragraphs {
			p := c.convertParagraph(&doc.Paragraphs[i])
			c.pkg.Document.Body.Paragraphs = append(c.pkg.Document.Body.Paragraphs, p)
			c.pkg.Document.Body.Blocks = append(c.pkg.Document.Body.Blocks, wordparts.BodyBlock{
				Kind:      wordparts.BodyBlockParagraph,
				Paragraph: p,
			})
		}
		for i := range doc.Tables {
			t := c.convertTable(&doc.Tables[i])
			c.pkg.Document.Body.Tables = append(c.pkg.Document.Body.Tables, t)
			c.pkg.Document.Body.Blocks = append(c.pkg.Document.Body.Blocks, wordparts.BodyBlock{
				Kind:  wordparts.BodyBlockTable,
				Table: t,
			})
		}
	}

	if len(doc.Sections) > 0 {
		c.pkg.Document.Body.Section = c.convertSection(doc, &doc.Sections[len(doc.Sections)-1])
	}
}

func (c *converter) emitAutoTextFieldBlocks(doc *wordir.Document, start, end int, field *wordir.StoryField) bool {
	if doc == nil || field == nil || !strings.EqualFold(strings.TrimSpace(field.Kind), "AUTOTEXT") || start < 0 || end < start || end >= len(doc.Blocks) {
		return false
	}
	// A complex Word field can span block results, but it still needs run-level
	// begin/separate/end markers. Reconstruct those markers only when the result
	// has paragraph boundaries we can anchor them to safely.
	if doc.Blocks[start].Kind != "paragraph" || doc.Blocks[start].Paragraph == nil || doc.Blocks[end].Kind != "paragraph" || doc.Blocks[end].Paragraph == nil {
		return false
	}
	converted := make([]wordparts.BodyBlock, 0, end-start+1)
	for i := start; i <= end; i++ {
		converted = append(converted, c.convertBodyBlock(doc, &doc.Blocks[i]))
	}
	if len(converted) == 0 || converted[0].Paragraph == nil || converted[len(converted)-1].Paragraph == nil {
		return false
	}
	instr := strings.TrimSpace(field.Instr)
	if instr == "" {
		name := strings.TrimSpace(field.Arg)
		if name == "" {
			name = "AutoText"
		}
		instr = `AUTOTEXT "` + name + `"`
	}
	prefix := &wordparts.Run{Contents: []wordparts.RunContent{
		{Kind: wordparts.RunContentFldChar, FldChar: &wordparts.FldChar{Type: "begin"}},
		{Kind: wordparts.RunContentInstrText, InstrText: instr},
		{Kind: wordparts.RunContentFldChar, FldChar: &wordparts.FldChar{Type: "separate"}},
	}}
	suffix := &wordparts.Run{Contents: []wordparts.RunContent{
		{Kind: wordparts.RunContentFldChar, FldChar: &wordparts.FldChar{Type: "end"}},
	}}
	prependParagraphRun(converted[0].Paragraph, prefix)
	appendParagraphRun(converted[len(converted)-1].Paragraph, suffix)
	c.pkg.Document.Body.Blocks = append(c.pkg.Document.Body.Blocks, converted...)
	return true
}

func prependParagraphRun(p *wordparts.Paragraph, run *wordparts.Run) {
	if p == nil || run == nil {
		return
	}
	p.Runs = append([]*wordparts.Run{run}, p.Runs...)
	if len(p.Inlines) > 0 {
		p.Inlines = append([]wordparts.ParagraphInline{{Kind: wordparts.ParagraphInlineRun, Run: run}}, p.Inlines...)
	}
}

func appendParagraphRun(p *wordparts.Paragraph, run *wordparts.Run) {
	if p == nil || run == nil {
		return
	}
	p.Runs = append(p.Runs, run)
	if len(p.Inlines) > 0 {
		p.Inlines = append(p.Inlines, wordparts.ParagraphInline{Kind: wordparts.ParagraphInlineRun, Run: run})
	}
}

func contentControlsEqual(a, b *wordir.ContentControl) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ID == b.ID && a.Tag == b.Tag && a.Title == b.Title && a.Type == b.Type
}

func (c *converter) convertBodyBlock(doc *wordir.Document, block *wordir.Block) wordparts.BodyBlock {
	switch block.Kind {
	case "paragraph":
		if block.Paragraph != nil {
			p := c.convertParagraph(block.Paragraph)
			c.pkg.Document.Body.Paragraphs = append(c.pkg.Document.Body.Paragraphs, p)
			return wordparts.BodyBlock{Kind: wordparts.BodyBlockParagraph, Paragraph: p}
		}
	case "table":
		if block.Table != nil {
			t := c.convertTable(block.Table)
			c.pkg.Document.Body.Tables = append(c.pkg.Document.Body.Tables, t)
			return wordparts.BodyBlock{Kind: wordparts.BodyBlockTable, Table: t}
		}
	}
	return wordparts.BodyBlock{}
}

func (c *converter) emitBodyBlock(doc *wordir.Document, block *wordir.Block, idx int) {
	switch block.Kind {
	case "paragraph":
		if block.Paragraph != nil {
			p := c.convertParagraph(block.Paragraph)
			c.pkg.Document.Body.Paragraphs = append(c.pkg.Document.Body.Paragraphs, p)
			c.pkg.Document.Body.Blocks = append(c.pkg.Document.Body.Blocks, wordparts.BodyBlock{
				Kind:      wordparts.BodyBlockParagraph,
				Paragraph: p,
			})
		}
	case "table":
		if block.Table != nil {
			t := c.convertTable(block.Table)
			c.pkg.Document.Body.Tables = append(c.pkg.Document.Body.Tables, t)
			c.pkg.Document.Body.Blocks = append(c.pkg.Document.Body.Blocks, wordparts.BodyBlock{
				Kind:  wordparts.BodyBlockTable,
				Table: t,
			})
		}
	case "section":
		if block.Section != nil {
			sp := c.convertSection(doc, block.Section)
			isLast := idx == len(doc.Blocks)-1
			if isLast {
				// Trailing section break: emit as body-level sectPr.
				c.pkg.Document.Body.Blocks = append(c.pkg.Document.Body.Blocks, wordparts.BodyBlock{
					Kind:    wordparts.BodyBlockSection,
					Section: sp,
				})
			} else {
				// Intermediate section break: attach to preceding paragraph.
				if len(c.pkg.Document.Body.Paragraphs) > 0 {
					lastP := c.pkg.Document.Body.Paragraphs[len(c.pkg.Document.Body.Paragraphs)-1]
					if lastP.Properties == nil {
						lastP.Properties = &wordparts.ParagraphProperties{}
					}
					lastP.Properties.Section = sp
				}
			}
		}
	}
}

// ── Sections ─────────────────────────────────────────────────────────────────

func (c *converter) convertSection(doc *wordir.Document, s *wordir.Section) *wordparts.SectionProperties {
	sp := &wordparts.SectionProperties{}

	if s.BreakKind != "" {
		sp.Type = stringVal(s.BreakKind)
	} else if s.Type != "" {
		sp.Type = stringVal(s.Type)
	}
	if s.PageSize != nil && s.PageSize.Width > 0 && s.PageSize.Height > 0 {
		sp.PageSize = &wordparts.PageSize{
			Width:  s.PageSize.Width,
			Height: s.PageSize.Height,
		}
		if s.PageSize.Orient != "" {
			sp.PageSize.Orient = s.PageSize.Orient
		}
	}
	if s.PageMargin != nil {
		sp.PageMargin = &wordparts.PageMargin{
			Top:    s.PageMargin.Top,
			Right:  s.PageMargin.Right,
			Bottom: s.PageMargin.Bottom,
			Left:   s.PageMargin.Left,
			Header: s.PageMargin.Header,
			Footer: s.PageMargin.Footer,
			Gutter: s.PageMargin.Gutter,
		}
	}
	if s.Columns != nil && s.Columns.Num > 0 {
		sp.Columns = &wordparts.Columns{
			Num:   s.Columns.Num,
			Space: s.Columns.Space,
			Sep:   s.Columns.Sep,
		}
	}
	if s.PageNumberFormat != "" || s.PageNumberStart != nil {
		sp.PageNumType = &wordparts.PageNumberType{
			Format: s.PageNumberFormat,
		}
		if s.PageNumberStart != nil {
			sp.PageNumType.Start = *s.PageNumberStart
		}
	}
	if s.TextDirection != "" {
		sp.TextDirection = stringVal(s.TextDirection)
	}
	if s.TitlePage {
		sp.TitlePage = onOffBool(true)
	}
	if s.Bidi {
		sp.Bidi = onOffBool(true)
	}
	if s.VerticalAlign != "" {
		sp.VAlign = stringVal(s.VerticalAlign)
	}
	if s.PageBorders != nil {
		sp.PageBorders = c.convertPageBorders(s.PageBorders)
	}
	if s.LineNumbering != nil {
		sp.LnNumType = &wordparts.LineNumberType{
			CountBy:  s.LineNumbering.CountBy,
			Start:    s.LineNumbering.Start,
			Distance: s.LineNumbering.Distance,
			Restart:  s.LineNumbering.Restart,
		}
	}
	if s.DocGrid != nil {
		sp.DocGrid = &wordparts.DocGrid{
			LinePitch: s.DocGrid.LinePitch,
			CharSpace: s.DocGrid.CharSpace,
			Type:      s.DocGrid.Type,
		}
	}
	if s.PaperSource != nil {
		sp.PaperSrc = &wordparts.PaperSource{
			First: s.PaperSource.First,
			Other: s.PaperSource.Other,
		}
	}
	if s.FootnotePos != "" || s.FootnoteNumFmt != "" || s.FootnoteNumStart != 0 || s.FootnoteRestart != "" {
		sp.FootnoteProps = &wordparts.NoteProperties{}
		if s.FootnotePos != "" {
			sp.FootnoteProps.Position = stringVal(s.FootnotePos)
		}
		if s.FootnoteNumFmt != "" {
			sp.FootnoteProps.Format = stringVal(s.FootnoteNumFmt)
		}
		if s.FootnoteNumStart != 0 {
			sp.FootnoteProps.Start = intVal(s.FootnoteNumStart)
		}
		if s.FootnoteRestart != "" {
			sp.FootnoteProps.Restart = stringVal(s.FootnoteRestart)
		}
	}
	if s.EndnoteNumFmt != "" || s.EndnoteNumStart != 0 || s.EndnoteRestart != "" {
		sp.EndnoteProps = &wordparts.NoteProperties{}
		if s.EndnoteNumFmt != "" {
			sp.EndnoteProps.Format = stringVal(s.EndnoteNumFmt)
		}
		if s.EndnoteNumStart != 0 {
			sp.EndnoteProps.Start = intVal(s.EndnoteNumStart)
		}
		if s.EndnoteRestart != "" {
			sp.EndnoteProps.Restart = stringVal(s.EndnoteRestart)
		}
	}
	if s.EndnoteAtSectionEnd {
		sp.NoEndnote = onOffBool(false)
	}
	if s.FormProt {
		sp.FormProt = onOffBool(true)
	}

	c.convertSectionHeadersFooters(doc, s, sp)

	return sp
}

func (c *converter) convertSectionHeadersFooters(doc *wordir.Document, s *wordir.Section, sp *wordparts.SectionProperties) {
	for _, ref := range s.Headers {
		sp.HeaderRefs = append(sp.HeaderRefs, wordparts.HeaderFooterReference{
			Type: ref.Type,
			ID:   c.ensureHeaderPart(doc, ref.PartName),
		})
	}
	for _, ref := range s.Footers {
		sp.FooterRefs = append(sp.FooterRefs, wordparts.HeaderFooterReference{
			Type: ref.Type,
			ID:   c.ensureFooterPart(doc, ref.PartName),
		})
	}
}

func (c *converter) ensureHeaderPart(doc *wordir.Document, partName string) string {
	if partName == "" {
		return ""
	}
	rels := c.pkg.EnsurePartRelationships(c.pkg.MainPartName)
	for _, rel := range rels.Relationships {
		if rel.Target == partName {
			return rel.ID
		}
	}
	// Convert the story content if it exists.
	if story, ok := doc.Headers[partName]; ok {
		if _, exists := c.pkg.Headers[partName]; !exists {
			c.pkg.Headers[partName] = c.convertStoryToHeader(story)
		}
	}
	rid := c.nextRID()
	c.pkg.UpsertRelationship(c.pkg.MainPartName, &wordparts.Relationship{
		ID:     rid,
		Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header",
		Target: partName,
	})
	return rid
}

func (c *converter) ensureFooterPart(doc *wordir.Document, partName string) string {
	if partName == "" {
		return ""
	}
	rels := c.pkg.EnsurePartRelationships(c.pkg.MainPartName)
	for _, rel := range rels.Relationships {
		if rel.Target == partName {
			return rel.ID
		}
	}
	if story, ok := doc.Footers[partName]; ok {
		if _, exists := c.pkg.Footers[partName]; !exists {
			c.pkg.Footers[partName] = c.convertStoryToFooter(story)
		}
	}
	rid := c.nextRID()
	c.pkg.UpsertRelationship(c.pkg.MainPartName, &wordparts.Relationship{
		ID:     rid,
		Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer",
		Target: partName,
	})
	return rid
}

func (c *converter) convertStoryToHeader(story wordir.Story) *wordparts.Header {
	h := &wordparts.Header{}
	for i := range story.Paragraphs {
		h.Paragraphs = append(h.Paragraphs, c.convertParagraph(&story.Paragraphs[i]))
	}
	for i := range story.Tables {
		h.Tables = append(h.Tables, c.convertTable(&story.Tables[i]))
	}
	return h
}

func (c *converter) convertStoryToFooter(story wordir.Story) *wordparts.Footer {
	f := &wordparts.Footer{}
	for i := range story.Paragraphs {
		f.Paragraphs = append(f.Paragraphs, c.convertParagraph(&story.Paragraphs[i]))
	}
	for i := range story.Tables {
		f.Tables = append(f.Tables, c.convertTable(&story.Tables[i]))
	}
	return f
}

// ── Styles ───────────────────────────────────────────────────────────────────

func (c *converter) mapStyles(doc *wordir.Document) {
	if len(doc.Styles) == 0 && len(doc.CharacterStyles) == 0 && doc.LatentStyles == nil {
		return
	}
	c.pkg.Styles = &wordparts.Styles{}

	for id, style := range doc.Styles {
		styleType := "paragraph"
		if style.TableFormat != nil || len(style.TableStyles) > 0 {
			styleType = "table"
		}
		ps := &wordparts.Style{
			Type:    styleType,
			StyleID: id,
			Name:    stringVal(style.Name),
		}
		if style.Default {
			bv := wordparts.BoolVal(true)
			ps.Default = &bv
		}
		if style.BasedOn != "" {
			ps.BasedOn = stringVal(style.BasedOn)
		}
		if style.LinkedCharID != "" {
			ps.Link = stringVal(style.LinkedCharID)
		}
		if style.NextStyleID != "" {
			ps.Next = stringVal(style.NextStyleID)
		}
		if style.UIPriority != 0 {
			ps.UIPriority = intVal(style.UIPriority)
		}
		ps.RSID = style.RSID
		ps.Unhide = onOffBool(style.UnhideWhenUsed)
		ps.QuickFormat = onOffBool(style.QFormat)
		ps.SemiHidden = onOffBool(style.SemiHidden)
		ps.AutoRedef = onOffBool(style.AutoRedefine)
		ps.ParaProps = c.convertStyleParagraphProperties(&style.Format)
		ps.RunProps = c.convertStyleRunProperties(&style.RunFormat)
		if style.Numbering != nil {
			ps.ParaProps = ensureParaProps(ps.ParaProps)
			ps.ParaProps.Numbering = &wordparts.NumberingReference{
				Level: intVal(style.Numbering.Level),
				NumID: intVal(style.Numbering.NumID),
			}
		}
		if style.TableFormat != nil {
			ps.TableProps = c.convertTableProperties(style.TableFormat)
		}
		for _, cts := range style.TableStyles {
			ps.TableStyles = append(ps.TableStyles, c.convertConditionalTableStyle(&cts))
		}
		c.pkg.Styles.Styles = append(c.pkg.Styles.Styles, ps)
	}

	for id, style := range doc.CharacterStyles {
		cs := &wordparts.Style{
			Type:    "character",
			StyleID: id,
			Name:    stringVal(style.Name),
		}
		if style.BasedOn != "" {
			cs.BasedOn = stringVal(style.BasedOn)
		}
		if style.NextStyleID != "" {
			cs.Next = stringVal(style.NextStyleID)
		}
		if style.UIPriority != 0 {
			cs.UIPriority = intVal(style.UIPriority)
		}
		cs.RSID = style.RSID
		cs.Unhide = onOffBool(style.UnhideWhenUsed)
		cs.QuickFormat = onOffBool(style.QFormat)
		cs.SemiHidden = onOffBool(style.SemiHidden)
		cs.AutoRedef = onOffBool(style.AutoRedefine)
		if id == doc.DefaultCharacterStyleID {
			bv := wordparts.BoolVal(true)
			cs.Default = &bv
		}
		cs.RunProps = c.convertStyleRunProperties(&style.Format)
		c.pkg.Styles.Styles = append(c.pkg.Styles.Styles, cs)
	}

	c.pkg.Styles.DocDefaults = &wordparts.DocDefaults{}
	c.pkg.Styles.DocDefaults.RunDefaults = &wordparts.RunDefaults{
		Properties: c.convertStyleRunProperties(&doc.DefaultRunFormat),
	}
	c.pkg.Styles.DocDefaults.ParaDefaults = &wordparts.ParagraphDefaults{
		Properties: c.convertStyleParagraphProperties(&doc.DefaultParagraphFormat),
	}

	if doc.LatentStyles != nil {
		c.pkg.Styles.LatentStyles = &wordparts.LatentStyles{}
		for _, exc := range doc.LatentStyles.Exceptions {
			c.pkg.Styles.LatentStyles.Exceptions = append(c.pkg.Styles.LatentStyles.Exceptions, wordparts.LsdException{
				Name:           exc.Name,
				Locked:         exc.Locked,
				UIPriority:     exc.UIPriority,
				SemiHidden:     exc.SemiHidden,
				UnhideWhenUsed: exc.UnhideWhenUsed,
				QFormat:        exc.QFormat,
			})
		}
	}
}

func (c *converter) convertStyleParagraphProperties(pf *wordir.ParagraphFormat) *wordparts.ParagraphProperties {
	dummy := &wordir.Paragraph{Format: *pf}
	return c.convertParagraphProperties(dummy)
}

func (c *converter) convertStyleRunProperties(rf *wordir.RunFormat) *wordparts.RunProperties {
	dummy := &wordir.Run{Format: *rf}
	return c.convertRunProperties(dummy)
}

func ensureParaProps(pp *wordparts.ParagraphProperties) *wordparts.ParagraphProperties {
	if pp == nil {
		return &wordparts.ParagraphProperties{}
	}
	return pp
}

func (c *converter) convertConditionalTableStyle(cts *wordir.ConditionalTableStyle) *wordparts.TableStyleProps {
	tsp := &wordparts.TableStyleProps{Type: cts.Type}
	if cts.CellFormat.Width != nil || cts.CellFormat.GridSpan > 1 || cts.CellFormat.VMerge != "" || cts.CellFormat.HMerge ||
		cts.CellFormat.VertAlign != "" || cts.CellFormat.TextDirection != "" || cts.CellFormat.Shading != "" || cts.CellFormat.Borders != nil {
		tsp.CellProps = c.convertTableCellProperties(&cts.CellFormat)
	}
	if !isEmptyRunFormat(&cts.RunFormat) {
		tsp.RunProps = c.convertStyleRunProperties(&cts.RunFormat)
	}
	if !isEmptyParagraphFormat(&cts.ParaFormat) {
		tsp.ParaProps = c.convertStyleParagraphProperties(&cts.ParaFormat)
	}
	return tsp
}

func isEmptyRunFormat(rf *wordir.RunFormat) bool {
	if rf == nil {
		return true
	}
	return rf.Bold == nil && rf.Italic == nil && rf.Strike == nil && rf.DoubleStrike == nil &&
		rf.Caps == nil && rf.SmallCaps == nil && rf.Underline == "" && rf.Color == "" && rf.Background == "" &&
		rf.FontASCII == "" && rf.FontHAnsi == "" && rf.FontEastAsia == "" && rf.FontCS == "" &&
		rf.FontSize == nil && (rf.Kerning == nil || *rf.Kerning == 0) && (rf.CharScale == nil || *rf.CharScale == 0) &&
		rf.WebHidden == nil && (rf.FitTextWidth == nil || *rf.FitTextWidth == 0) && rf.ComplexScript == nil &&
		rf.UsePageSetup == nil && rf.EmphasisMark == "" && rf.Lang == "" && rf.Highlight == "" &&
		rf.VertAlignVal == "" && rf.Vanish == nil && rf.NoProof == nil && rf.RTL == nil &&
		rf.Outline == nil && rf.Shadow == nil && rf.Emboss == nil && rf.Imprint == nil &&
		rf.Ligatures == "" && rf.NumberSpacing == "" && len(rf.StylisticSets) == 0 &&
		rf.Borders == nil && rf.FELayout == nil && rf.TextFill == nil && rf.TextOutline == nil &&
		rf.TextGlow == nil && rf.TextShadow == nil && rf.CharacterSpacing == nil
}

func isEmptyParagraphFormat(pf *wordir.ParagraphFormat) bool {
	if pf == nil {
		return true
	}
	return pf.Alignment == "" && pf.Shading == "" && pf.SpacingBefore == nil && pf.SpacingAfter == nil &&
		pf.SpacingBeforeLines == nil && pf.SpacingAfterLines == nil && pf.LineSpacing == nil &&
		pf.IndentLeft == nil && pf.IndentRight == nil && pf.IndentFirstLine == nil && pf.IndentHanging == nil &&
		!pf.ContextualSpacing && !pf.KeepNext && !pf.KeepLines && !pf.WidowControl && !pf.PageBreakBefore &&
		!pf.SpacingBeforeAuto && !pf.SpacingAfterAuto && pf.Borders == nil && !pf.SnapToGrid && !pf.Bidi &&
		!pf.Kinsoku && pf.WordWrap == nil && !pf.OverflowPunct && !pf.TopLinePunct && !pf.AutoSpaceDE &&
		!pf.AutoSpaceDN && !pf.UsePageSetup && pf.FontAlign == "" && len(pf.TabStops) == 0 &&
		(pf.OutlineLevel == nil || *pf.OutlineLevel == 0) && !pf.NoLineNumber && !pf.NoAutoHyphenation &&
		!pf.MirrorIndents && !pf.NoAllowOverlap && !pf.AdjustRightInd && pf.BinaryListLevel == nil &&
		pf.BinaryListOverride == nil && pf.BinaryListFormat == nil && pf.BinaryStyleIndex == nil &&
		pf.BinaryFrame == nil
}

// ── Numbering ────────────────────────────────────────────────────────────────

func (c *converter) mapNumbering(doc *wordir.Document) {
	numMap := make(map[int]*wordir.Numbering)
	for _, para := range doc.Paragraphs {
		if para.Numbering != nil {
			numMap[para.Numbering.NumID] = para.Numbering
		}
	}
	if len(numMap) == 0 && len(doc.NumberingDefinitions) == 0 {
		return
	}

	c.pkg.Numbering = &wordparts.Numbering{}
	abstractNumMap := make(map[int]int)

	// Emit editor-created numbering definitions first.
	for _, def := range doc.NumberingDefinitions {
		if _, ok := abstractNumMap[def.AbstractNumID]; !ok {
			abstractNumMap[def.AbstractNumID] = len(c.pkg.Numbering.AbstractNums)
			c.pkg.Numbering.AbstractNums = append(c.pkg.Numbering.AbstractNums,
				c.convertAbstractNumFromDef(&def))
		}
		c.pkg.Numbering.Nums = append(c.pkg.Numbering.Nums, &wordparts.Num{
			ID:            def.NumID,
			AbstractNumID: intVal(def.AbstractNumID),
		})
		// Mark this numID as handled.
		numMap[def.NumID] = nil
	}

	// Fall back to paragraph-level numbering for any remaining NumIDs.
	for _, num := range numMap {
		if num == nil {
			continue
		}
		if _, ok := abstractNumMap[num.AbstractNumID]; !ok {
			abstractNumMap[num.AbstractNumID] = len(c.pkg.Numbering.AbstractNums)
			c.pkg.Numbering.AbstractNums = append(c.pkg.Numbering.AbstractNums,
				c.convertAbstractNum(num))
		}
		c.pkg.Numbering.Nums = append(c.pkg.Numbering.Nums, &wordparts.Num{
			ID:            num.NumID,
			AbstractNumID: intVal(num.AbstractNumID),
		})
	}
}

func (c *converter) convertAbstractNum(num *wordir.Numbering) *wordparts.AbstractNum {
	return &wordparts.AbstractNum{
		ID: num.AbstractNumID,
		Levels: []*wordparts.Lvl{
			{
				Level:  num.Level,
				Start:  intVal(num.Start),
				Format: stringVal(num.Format),
			},
		},
	}
}

func (c *converter) convertAbstractNumFromDef(def *wordir.NumberingDefinition) *wordparts.AbstractNum {
	an := &wordparts.AbstractNum{
		ID:     def.AbstractNumID,
		Levels: make([]*wordparts.Lvl, 0, len(def.Levels)),
	}
	for _, lvl := range def.Levels {
		wpLvl := &wordparts.Lvl{
			Level:  lvl.Level,
			Start:  intVal(lvl.Start),
			Format: stringVal(lvl.Format),
		}
		if lvl.Text != "" {
			wpLvl.Text = &wordparts.StringVal{Val: lvl.Text}
		}
		if lvl.StyleID != "" {
			wpLvl.PStyle = &wordparts.StringVal{Val: lvl.StyleID}
		}
		if lvl.IndentLeft != 0 {
			wpLvl.ParaProps = &wordparts.LvlPPr{
				Ind: &wordparts.LvlInd{Left: lvl.IndentLeft},
			}
		}
		if lvl.RunFormat != nil {
			wpLvl.RunProps = c.convertStyleRunProperties(lvl.RunFormat)
		}
		an.Levels = append(an.Levels, wpLvl)
	}
	return an
}

// ── Footnotes / Endnotes ─────────────────────────────────────────────────────

func (c *converter) mapFootnotes(doc *wordir.Document) {
	if len(doc.Footnotes) == 0 && !storyHasBlocks(doc.FootnoteSeparator) && !storyHasBlocks(doc.FootnoteContinuationSep) && !storyHasBlocks(doc.FootnoteContinuationNotice) {
		return
	}
	c.pkg.Footnotes = &wordparts.FootnotesPart{}
	if storyHasBlocks(doc.FootnoteSeparator) {
		c.pkg.Footnotes.Notes = append(c.pkg.Footnotes.Notes, c.convertNote(-1, "separator", doc.FootnoteSeparator))
	}
	if storyHasBlocks(doc.FootnoteContinuationSep) {
		c.pkg.Footnotes.Notes = append(c.pkg.Footnotes.Notes, c.convertNote(0, "continuationSeparator", doc.FootnoteContinuationSep))
	}
	if storyHasBlocks(doc.FootnoteContinuationNotice) {
		c.pkg.Footnotes.Notes = append(c.pkg.Footnotes.Notes, c.convertNote(1, "continuationNotice", doc.FootnoteContinuationNotice))
	}
	for id, story := range doc.Footnotes {
		c.pkg.Footnotes.Notes = append(c.pkg.Footnotes.Notes, c.convertNote(id, "normal", story))
	}
}

func (c *converter) mapEndnotes(doc *wordir.Document) {
	if len(doc.Endnotes) == 0 && !storyHasBlocks(doc.EndnoteSeparator) && !storyHasBlocks(doc.EndnoteContinuationSep) && !storyHasBlocks(doc.EndnoteContinuationNotice) {
		return
	}
	c.pkg.Endnotes = &wordparts.EndnotesPart{}
	if storyHasBlocks(doc.EndnoteSeparator) {
		c.pkg.Endnotes.Notes = append(c.pkg.Endnotes.Notes, c.convertNote(-1, "separator", doc.EndnoteSeparator))
	}
	if storyHasBlocks(doc.EndnoteContinuationSep) {
		c.pkg.Endnotes.Notes = append(c.pkg.Endnotes.Notes, c.convertNote(0, "continuationSeparator", doc.EndnoteContinuationSep))
	}
	if storyHasBlocks(doc.EndnoteContinuationNotice) {
		c.pkg.Endnotes.Notes = append(c.pkg.Endnotes.Notes, c.convertNote(1, "continuationNotice", doc.EndnoteContinuationNotice))
	}
	for id, story := range doc.Endnotes {
		c.pkg.Endnotes.Notes = append(c.pkg.Endnotes.Notes, c.convertNote(id, "normal", story))
	}
}

func (c *converter) convertNote(id int, noteType string, story wordir.Story) *wordparts.Note {
	note := &wordparts.Note{
		ID:   id,
		Type: noteType,
	}
	for _, para := range story.Paragraphs {
		note.Paragraphs = append(note.Paragraphs, c.convertParagraph(&para))
	}
	for _, tbl := range story.Tables {
		note.Tables = append(note.Tables, c.convertTable(&tbl))
	}
	return note
}

// ── Comments ─────────────────────────────────────────────────────────────────

func (c *converter) mapComments(doc *wordir.Document) {
	if len(doc.Comments) == 0 {
		return
	}
	c.pkg.Comments = &wordparts.CommentsPart{}
	for id, story := range doc.Comments {
		c.pkg.Comments.Comments = append(c.pkg.Comments.Comments, &wordparts.Comment{
			ID:         id,
			Author:     story.Author,
			Date:       story.Date,
			Paragraphs: c.convertStoryParagraphs(story),
		})
	}
}

func (c *converter) convertStoryParagraphs(story wordir.Story) []*wordparts.Paragraph {
	var paragraphs []*wordparts.Paragraph
	for _, para := range story.Paragraphs {
		paragraphs = append(paragraphs, c.convertParagraph(&para))
	}
	return paragraphs
}

func storyHasBlocks(story wordir.Story) bool {
	return len(story.Blocks) > 0 || len(story.Paragraphs) > 0 || len(story.Tables) > 0
}

// ── Settings ─────────────────────────────────────────────────────────────────

func (c *converter) mapSettings(doc *wordir.Document) {
	c.pkg.Settings = &wordparts.Settings{}
	if doc.FootnoteNumberFormat != "" || doc.FootnoteNumberStart != 0 || doc.FootnoteRestart != "" {
		c.pkg.Settings.FootnoteProps = &wordparts.NoteProperties{}
		if doc.FootnoteNumberFormat != "" {
			c.pkg.Settings.FootnoteProps.Format = stringVal(doc.FootnoteNumberFormat)
		}
		if doc.FootnoteNumberStart != 0 {
			c.pkg.Settings.FootnoteProps.Start = intVal(doc.FootnoteNumberStart)
		}
		if doc.FootnoteRestart != "" {
			c.pkg.Settings.FootnoteProps.Restart = stringVal(doc.FootnoteRestart)
		}
	}
	if doc.EndnotePosition != "" || doc.EndnoteNumberFormat != "" || doc.EndnoteNumberStart != 0 || doc.EndnoteRestart != "" {
		c.pkg.Settings.EndnoteProps = &wordparts.NoteProperties{}
		if doc.EndnotePosition != "" {
			c.pkg.Settings.EndnoteProps.Position = stringVal(doc.EndnotePosition)
		}
		if doc.EndnoteNumberFormat != "" {
			c.pkg.Settings.EndnoteProps.Format = stringVal(doc.EndnoteNumberFormat)
		}
		if doc.EndnoteNumberStart != 0 {
			c.pkg.Settings.EndnoteProps.Start = intVal(doc.EndnoteNumberStart)
		}
		if doc.EndnoteRestart != "" {
			c.pkg.Settings.EndnoteProps.Restart = stringVal(doc.EndnoteRestart)
		}
	}
	if doc.DefaultTabStop != 0 {
		c.pkg.Settings.DefaultTabStop = intVal(doc.DefaultTabStop)
	}
	if doc.ZoomPercent != 0 {
		c.pkg.Settings.Zoom = &wordparts.ZoomSetting{Percent: doc.ZoomPercent}
	}
	if doc.ViewMode != "" {
		c.pkg.Settings.View = stringVal(doc.ViewMode)
	}
	if doc.TrackChangesEnabled {
		c.pkg.Settings.TrackRevisions = onOffBool(true)
	}
	if doc.ProtectionEnabled && doc.DocumentProtection != nil {
		c.pkg.Settings.DocumentProtection = &wordparts.DocumentProtection{
			Edit:        doc.DocumentProtection.Edit,
			Enforcement: doc.DocumentProtection.Enforcement,
		}
	}
	if doc.CharacterSpacingControl != "" {
		c.pkg.Settings.CharacterSpacingControl = stringVal(doc.CharacterSpacingControl)
	}
	if doc.AutoHyphenation {
		c.pkg.Settings.AutoHyphenation = onOffBool(true)
	}
	if doc.EvenAndOddHeaders {
		c.pkg.Settings.EvenAndOddHeaders = onOffBool(true)
	}
	if doc.UpdateFields {
		c.pkg.Settings.UpdateFields = onOffBool(true)
	}
	if doc.ConsecutiveHyphenLimit != 0 {
		c.pkg.Settings.ConsecutiveHyphenLimit = intVal(doc.ConsecutiveHyphenLimit)
	}
	if doc.HyphenationZone != 0 {
		c.pkg.Settings.HyphenationZone = intVal(doc.HyphenationZone)
	}
	if doc.DoNotHyphenateCaps {
		c.pkg.Settings.DoNotHyphenateCaps = onOffBool(true)
	}
	if doc.ThemeFontLang != nil {
		c.pkg.Settings.ThemeFontLang = &wordparts.ThemeFontLang{
			Val:      doc.ThemeFontLang.Val,
			EastAsia: doc.ThemeFontLang.EastAsia,
			Bidi:     doc.ThemeFontLang.Bidi,
		}
	}
	if doc.ProofState != nil {
		c.pkg.Settings.ProofState = &wordparts.ProofState{
			Spelling: doc.ProofState.Spelling,
			Grammar:  doc.ProofState.Grammar,
		}
	}
	if len(doc.DocumentVariables) > 0 {
		c.pkg.Settings.DocVars = make([]wordparts.DocVar, 0, len(doc.DocumentVariables))
		for name, value := range doc.DocumentVariables {
			c.pkg.Settings.DocVars = append(c.pkg.Settings.DocVars, wordparts.DocVar{
				Name:  name,
				Value: value,
			})
		}
	}
	if doc.MailMerge != nil {
		c.pkg.Settings.MailMerge = &wordparts.MailMergeSettings{
			MainDocumentType:        doc.MailMerge.MainDocumentType,
			LinkToQuery:             doc.MailMerge.LinkToQuery,
			DataType:                doc.MailMerge.DataType,
			ConnectString:           doc.MailMerge.ConnectString,
			Query:                   doc.MailMerge.Query,
			DataSource:              doc.MailMerge.DataSource,
			HeaderSource:            doc.MailMerge.HeaderSource,
			Destination:             doc.MailMerge.Destination,
			ActiveRecord:            stringVal(doc.MailMerge.ActiveRecord),
			AddressFieldName:        stringVal(doc.MailMerge.AddressFieldName),
			CheckErrors:             stringVal(doc.MailMerge.CheckErrors),
			DoNotSuppressBlankLines: onOffBool(doc.MailMerge.DoNotSuppressBlankLines),
			DynamicAddress:          onOffBool(doc.MailMerge.DynamicAddress),
			EntryMacro:              stringVal(doc.MailMerge.EntryMacro),
			ExitMacro:               stringVal(doc.MailMerge.ExitMacro),
			FHdr:                    onOffBool(doc.MailMerge.DataSourceHasHeaders),
			LinkedToFile:            onOffBool(doc.MailMerge.LinkedToFile),
			MailAsAttachment:        onOffBool(doc.MailMerge.MailAsAttachment),
			MailSubject:             stringVal(doc.MailMerge.MailSubject),
			MappedName:              stringVal(doc.MailMerge.MappedName),
			SourceFileName:          stringVal(doc.MailMerge.SourceFileName),
			StoreMappedDataAs:       stringVal(doc.MailMerge.StoreMappedDataAs),
			SubDoc:                  stringVal(doc.MailMerge.SubDoc),
			UDL:                     stringVal(doc.MailMerge.UDL),
			ViewMergedData:          onOffBool(doc.MailMerge.ViewMergedData),
			Personal:                onOffBool(doc.MailMerge.Personal),
			PersonalCompose:         onOffBool(doc.MailMerge.PersonalCompose),
			PersonalReply:           onOffBool(doc.MailMerge.PersonalReply),
		}
		if len(doc.MailMerge.Types) > 0 {
			c.pkg.Settings.MailMerge.Types = &wordparts.MMTypes{
				Types: make([]wordparts.MMType, 0, len(doc.MailMerge.Types)),
			}
			for _, mailMergeType := range doc.MailMerge.Types {
				if strings.TrimSpace(mailMergeType) == "" {
					continue
				}
				c.pkg.Settings.MailMerge.Types.Types = append(c.pkg.Settings.MailMerge.Types.Types, wordparts.MMType{
					Val: mailMergeType,
				})
			}
			if len(c.pkg.Settings.MailMerge.Types.Types) == 0 {
				c.pkg.Settings.MailMerge.Types = nil
			}
		}
		if len(doc.MailMerge.Recipients) > 0 {
			c.pkg.Settings.MailMerge.Recipients = &wordparts.Recipients{
				RecipientData: make([]wordparts.RecipientData, 0, len(doc.MailMerge.Recipients)),
			}
			for _, recipient := range doc.MailMerge.Recipients {
				c.pkg.Settings.MailMerge.Recipients.RecipientData = append(c.pkg.Settings.MailMerge.Recipients.RecipientData, wordparts.RecipientData{
					Active: onOffBool(recipient.Active),
				})
			}
		}
		if doc.MailMerge.ODSO != nil {
			odso := &wordparts.ODSO{
				ColDelim:       intVal(doc.MailMerge.ODSO.ColDelim),
				DynamicAddress: onOffBool(doc.MailMerge.ODSO.DynamicAddress),
				FHdr:           onOffBool(doc.MailMerge.ODSO.FHdr),
				SourceFileName: stringVal(doc.MailMerge.ODSO.SourceFileName),
				UDL:            stringVal(doc.MailMerge.ODSO.UDL),
			}
			for _, fieldMap := range doc.MailMerge.ODSO.FieldMapData {
				odso.FieldMapData = append(odso.FieldMapData, wordparts.FieldMapData{
					MappedName: fieldMap.MappedName,
					Name:       fieldMap.Name,
					Lang:       fieldMap.Lang,
					Val:        fieldMap.Val,
					Column:     fieldMap.Column,
				})
			}
			for _, recipient := range doc.MailMerge.ODSO.RecipientData {
				odso.RecipientData = append(odso.RecipientData, wordparts.RecipientData{
					Active: onOffBool(recipient.Active),
				})
			}
			c.pkg.Settings.MailMerge.ODSO = odso
		}
	}
	if doc.DocumentType != "" {
		c.pkg.Settings.DocumentType = stringVal(doc.DocumentType)
	}
	c.mapCompatSettings(doc)
}

func (c *converter) mapCompatSettings(doc *wordir.Document) {
	if doc.Compatibility == nil {
		return
	}
	compat := doc.Compatibility
	var settings []wordparts.CompatSetting
	add := func(name string, val bool) {
		if val {
			settings = append(settings, wordparts.CompatSetting{
				Name:  name,
				URI:   "http://schemas.microsoft.com/office/word",
				Value: "1",
			})
		}
	}
	add("doNotUseHTMLParagraphAutoSpacing", compat.DoNotUseHTMLParagraphAutoSpacing)
	add("alignBordersAndEdges", compat.AlignBordersAndEdges)
	add("lineWrapLikeWord6", compat.LineWrapLikeWord6)
	add("shapeLayoutLikeWW8", compat.ShapeLayoutLikeWW8)
	add("footnoteLayoutLikeWW8", compat.FootnoteLayoutLikeWW8)
	add("tableWrapLikeWW8", compat.TableWrapLikeWW8)
	add("noTabHangInd", compat.NoTabHangInd)
	add("noLeading", compat.NoLeading)
	add("spaceLikeWW8", compat.SpaceLikeWW8)
	add("noColumnBalance", compat.NoColumnBalance)
	add("doNotBreakWrappedTables", compat.DoNotBreakWrappedTables)
	add("doNotSnapToGridInCell", compat.DoNotSnapToGridInCell)
	add("doNotAllowFieldEndSelect", compat.DoNotAllowFieldEndSelect)
	add("applyBreakingRules", compat.ApplyBreakingRules)
	add("doNotBreakConstrainedForcedTables", compat.DoNotBreakConstrainedForcedTables)
	add("doNotVertAlignCellWithSp", compat.DoNotVertAlignCellWithSp)
	add("useWord2002TableRules", compat.UseWord2002TableRules)
	add("growAutofit", compat.GrowAutofit)
	add("useNormalStyleForListNumber", compat.UseNormalStyleForListNumber)
	add("doNotUseIndentAsNumberingTabStop", compat.DoNotUseIndentAsNumberingTabStop)
	add("useAltKinsokuLineBreakRules", compat.UseAltKinsokuLineBreakRules)
	add("allowSpaceOfSameStyleInTable", compat.AllowSpaceOfSameStyleInTable)
	add("doNotSuppressIndentation", compat.DoNotSuppressIndentation)
	add("doNotAutofitConstrainedTables", compat.DoNotAutofitConstrainedTables)
	add("autofitToFirstFixedWidthCell", compat.AutofitToFirstFixedWidthCell)
	add("underlineTabInNumberingList", compat.UnderlineTabInNumberingList)
	add("displayHangulFixedWidth", compat.DisplayHangulFixedWidth)
	add("splitPgBreakAndParaMark", compat.SplitPgBreakAndParaMark)
	add("doNotVertAlignInTxbx", compat.DoNotVertAlignInTxbx)
	add("useAnsiKerningPairs", compat.UseAnsiKerningPairs)
	add("cachedFieldBalance", compat.CachedFieldBalance)
	add("uiCompat97To2003", compat.UICompat97To2003)
	add("adjustLineHeightInTable", compat.AdjustLineHeightInTable)
	add("alignTablesRowByRow", compat.AlignTablesRowByRow)
	add("allowPNG", compat.AllowPNG)
	add("autoSpaceLikeWord95", compat.AutoSpaceLikeWord95)
	add("convMailMergeEsc", compat.ConvMailMergeEsc)
	add("doNotSuppressParagraphBorders", compat.DoNotSuppressParagraphBorders)
	add("doNotUseEastAsianBreakRules", compat.DoNotUseEastAsianBreakRules)
	add("doNotWrapTextWithPunct", compat.DoNotWrapTextWithPunct)
	add("forgetLastTabAlignment", compat.ForgetLastTabAlignment)
	add("layoutRawTableWidth", compat.LayoutRawTableWidth)
	add("layoutTableRowsApart", compat.LayoutTableRowsApart)
	add("mwSmallCaps", compat.MwSmallCaps)
	add("noExtraLineSpacing", compat.NoExtraLineSpacing)
	add("noSpaceRaiseLower", compat.NoSpaceRaiseLower)
	add("printBodyTextBeforeHeader", compat.PrintBodyTextBeforeHeader)
	add("printColBlack", compat.PrintColBlack)
	add("selectFldWithFirstOrLastChar", compat.SelectFldWithFirstOrLastChar)
	add("showBreaksInFrames", compat.ShowBreaksInFrames)
	add("spacingInWholePoints", compat.SpacingInWholePoints)
	add("subFontBySize", compat.SubFontBySize)
	add("suppressBottomSpacing", compat.SuppressBottomSpacing)
	add("suppressSpBfAfterPgBrk", compat.SuppressSpBfAfterPgBrk)
	add("suppressSpacingAtTopOfPage", compat.SuppressSpacingAtTopOfPage)
	add("suppressTopSpacing", compat.SuppressTopSpacing)
	add("suppressTopSpacingWP", compat.SuppressTopSpacingWP)
	add("swapBordersFacingPages", compat.SwapBordersFacingPages)
	add("truncateFontHeightsLikeWP6", compat.TruncateFontHeightsLikeWP6)
	add("useFELayout", compat.UseFELayout)
	add("usePrinterMetrics", compat.UsePrinterMetrics)
	add("useSingleBorderforContiguousCells", compat.UseSingleBorderforContiguousCells)
	add("useWord97LineBreakRules", compat.UseWord97LineBreakRules)
	add("wpJustification", compat.WpJustification)
	add("wpSpaceWidth", compat.WpSpaceWidth)
	add("wrapTrailSpaces", compat.WrapTrailSpaces)
	if len(settings) > 0 {
		c.pkg.Settings.Compat = &wordparts.CompatSettings{Settings: settings}
	}
}

// ── Font Table ───────────────────────────────────────────────────────────────

func (c *converter) mapFontTable(doc *wordir.Document) {
	if len(doc.FontFamilies) == 0 && len(doc.EmbeddedFonts) == 0 {
		return
	}
	c.pkg.FontTable = &wordparts.FontTable{}
	// Collect all font family names from both FontFamilies and EmbeddedFonts.
	names := make(map[string]bool)
	for name := range doc.FontFamilies {
		names[name] = true
	}
	for _, ef := range doc.EmbeddedFonts {
		names[ef.Family] = true
	}
	for name := range names {
		info, ok := doc.FontFamilies[name]
		font := &wordparts.Font{Name: name}
		if ok {
			if info.Family != "" {
				font.Family = stringVal(info.Family)
			}
			if info.Pitch != "" {
				font.Pitch = stringVal(info.Pitch)
			}
			if info.Charset != "" {
				font.Charset = stringVal(info.Charset)
			}
			if info.Panose1 != "" {
				font.Panose1 = stringVal(info.Panose1)
			}
		}
		c.pkg.FontTable.Fonts = append(c.pkg.FontTable.Fonts, font)
	}
	if len(doc.EmbeddedFonts) > 0 {
		c.mapEmbeddedFonts(doc)
	}
}

func (c *converter) mapEmbeddedFonts(doc *wordir.Document) {
	fontPartIndex := 0
	for _, ef := range doc.EmbeddedFonts {
		data := &wordparts.EmbeddedFontData{Typeface: ef.Family}
		for _, face := range ef.Faces {
			if len(face.Data) == 0 {
				continue
			}
			fontPartIndex++
			ext := fontFileExt(face.ContentType)
			name := fmt.Sprintf("word/fonts/font%d%s", fontPartIndex, ext)
			fp := &wordparts.FontPart{
				Name:        name,
				ContentType: face.ContentType,
				Data:        face.Data,
			}
			switch {
			case face.Weight >= 700 && face.Style == "italic":
				data.BoldItalic = fp
			case face.Weight >= 700:
				data.Bold = fp
			case face.Style == "italic":
				data.Italic = fp
			default:
				data.Regular = fp
			}
		}
		if data.Regular != nil || data.Bold != nil || data.Italic != nil || data.BoldItalic != nil {
			c.pkg.EmbeddedFonts = append(c.pkg.EmbeddedFonts, data)
		}
	}
	// Add embed refs to font table entries.
	fontPartMap := make(map[string]map[string]*wordparts.FontPart)
	for _, ef := range c.pkg.EmbeddedFonts {
		fontPartMap[ef.Typeface] = map[string]*wordparts.FontPart{
			"regular":    ef.Regular,
			"bold":       ef.Bold,
			"italic":     ef.Italic,
			"boldItalic": ef.BoldItalic,
		}
	}
	for _, font := range c.pkg.FontTable.Fonts {
		parts, ok := fontPartMap[font.Name]
		if !ok {
			continue
		}
		if parts["regular"] != nil {
			font.EmbedRegular = &wordparts.FontEmbedRef{RelID: c.nextRID()}
		}
		if parts["bold"] != nil {
			font.EmbedBold = &wordparts.FontEmbedRef{RelID: c.nextRID()}
		}
		if parts["italic"] != nil {
			font.EmbedItalic = &wordparts.FontEmbedRef{RelID: c.nextRID()}
		}
		if parts["boldItalic"] != nil {
			font.EmbedBoldItalic = &wordparts.FontEmbedRef{RelID: c.nextRID()}
		}
	}
}

func fontFileExt(contentType string) string {
	switch contentType {
	case "application/vnd.ms-fontobject":
		return ".eot"
	case "application/font-woff":
		return ".woff"
	case "application/x-font-ttf":
		return ".ttf"
	default:
		return ".odttf"
	}
}

// ── Theme ────────────────────────────────────────────────────────────────────

func (c *converter) mapTheme(doc *wordir.Document) {
	if doc.Theme != nil {
		c.pkg.Theme = themeFromIR(doc.Theme)
	} else {
		c.pkg.Theme = &wordparts.Theme{
			Elements: &wordparts.ThemeElements{
				ColorScheme: defaultColorScheme(),
				FontScheme:  defaultFontScheme(),
			},
		}
	}
}

func themeFromIR(t *wordir.Theme) *wordparts.Theme {
	pt := &wordparts.Theme{
		Name: t.Name,
		Elements: &wordparts.ThemeElements{
			ColorScheme: colorSchemeFromIR(t.ColorScheme),
			FontScheme:  fontSchemeFromIR(t.FontScheme),
		},
	}
	return pt
}

func colorSchemeFromIR(colors map[string]string) *wordparts.ColorScheme {
	cs := &wordparts.ColorScheme{}
	for slot, hex := range colors {
		entry := &wordparts.ThemeColorEntry{SRGB: &wordparts.ThemeSRGBColor{Val: hex}}
		switch slot {
		case "dk1":
			cs.Dk1 = entry
		case "lt1":
			cs.Lt1 = entry
		case "dk2":
			cs.Dk2 = entry
		case "lt2":
			cs.Lt2 = entry
		case "accent1":
			cs.Accent1 = entry
		case "accent2":
			cs.Accent2 = entry
		case "accent3":
			cs.Accent3 = entry
		case "accent4":
			cs.Accent4 = entry
		case "accent5":
			cs.Accent5 = entry
		case "accent6":
			cs.Accent6 = entry
		case "hlink":
			cs.Hlink = entry
		case "folHlink":
			cs.FolHlink = entry
		}
	}
	return cs
}

func fontSchemeFromIR(fs wordir.ThemeFontScheme) *wordparts.FontScheme {
	return &wordparts.FontScheme{
		MajorFont: fontSetFromIR(fs.MajorFont),
		MinorFont: fontSetFromIR(fs.MinorFont),
	}
}

func fontSetFromIR(fs wordir.ThemeFontSet) *wordparts.ThemeFontSet {
	tfs := &wordparts.ThemeFontSet{
		Latin: wordparts.ThemeLatin{Typeface: fs.Latin},
	}
	if fs.EastAsia != "" {
		tfs.EastAsia = append(tfs.EastAsia, &wordparts.ThemeTypeface{Typeface: fs.EastAsia})
	}
	if fs.ComplexScript != "" {
		tfs.ComplexScript = append(tfs.ComplexScript, &wordparts.ThemeTypeface{Typeface: fs.ComplexScript})
	}
	return tfs
}

func defaultColorScheme() *wordparts.ColorScheme {
	return &wordparts.ColorScheme{
		Dk1:      &wordparts.ThemeColorEntry{SysClr: &wordparts.ThemeSysColor{LastClr: "000000"}},
		Lt1:      &wordparts.ThemeColorEntry{SysClr: &wordparts.ThemeSysColor{LastClr: "FFFFFF"}},
		Dk2:      &wordparts.ThemeColorEntry{SRGB: &wordparts.ThemeSRGBColor{Val: "44546A"}},
		Lt2:      &wordparts.ThemeColorEntry{SRGB: &wordparts.ThemeSRGBColor{Val: "E7E6E6"}},
		Accent1:  &wordparts.ThemeColorEntry{SRGB: &wordparts.ThemeSRGBColor{Val: "4472C4"}},
		Accent2:  &wordparts.ThemeColorEntry{SRGB: &wordparts.ThemeSRGBColor{Val: "ED7D31"}},
		Accent3:  &wordparts.ThemeColorEntry{SRGB: &wordparts.ThemeSRGBColor{Val: "A5A5A5"}},
		Accent4:  &wordparts.ThemeColorEntry{SRGB: &wordparts.ThemeSRGBColor{Val: "FFC000"}},
		Accent5:  &wordparts.ThemeColorEntry{SRGB: &wordparts.ThemeSRGBColor{Val: "5B9BD5"}},
		Accent6:  &wordparts.ThemeColorEntry{SRGB: &wordparts.ThemeSRGBColor{Val: "70AD47"}},
		Hlink:    &wordparts.ThemeColorEntry{SRGB: &wordparts.ThemeSRGBColor{Val: "0563C1"}},
		FolHlink: &wordparts.ThemeColorEntry{SRGB: &wordparts.ThemeSRGBColor{Val: "954F72"}},
	}
}

func defaultFontScheme() *wordparts.FontScheme {
	return &wordparts.FontScheme{
		MajorFont: &wordparts.ThemeFontSet{
			Latin: wordparts.ThemeLatin{Typeface: "Calibri Light"},
		},
		MinorFont: &wordparts.ThemeFontSet{
			Latin: wordparts.ThemeLatin{Typeface: "Calibri"},
		},
	}
}
