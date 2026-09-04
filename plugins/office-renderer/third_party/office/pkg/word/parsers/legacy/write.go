package legacy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
)

// WriteDocument writes a Word 97-compatible legacy .doc file from the given IR
// document. Body paragraphs, tables, common field structures, images, and
// floating preset OfficeArt shapes are preserved.
func WriteDocument(doc *wordir.Document, w io.Writer) error {
	if doc == nil {
		return fmt.Errorf("nil document")
	}

	styles := buildLegacyStyleWriteCatalog(doc)
	numbering := buildLegacyNumberingWriteCatalog(doc)
	plan := buildLegacyWritePlan(doc, styles, numbering)
	fonts := buildLegacyFontWriteCatalog(doc, plan, styles)
	imageData := buildLegacyImageDataStream(plan.Images)
	text := plan.Text
	if text == "" {
		text = "\r"
		plan.Text = text
		plan.MainTextLen = 1
		plan.Paragraphs = []legacyParagraphWriteRun{{StartCP: 0, EndCP: 1}}
	}
	mainTextRunes := []rune(text)
	footnoteRunes := []rune(plan.FootnoteText)
	headerRunes := []rune(plan.HeaderText)
	commentRunes := []rune(plan.CommentText)
	endnoteRunes := []rune(plan.EndnoteText)
	textBoxRunes := []rune(plan.TextBoxText)
	allRunes := append([]rune(nil), mainTextRunes...)
	allRunes = append(allRunes, footnoteRunes...)
	allRunes = append(allRunes, headerRunes...)
	allRunes = append(allRunes, commentRunes...)
	allRunes = append(allRunes, endnoteRunes...)
	allRunes = append(allRunes, textBoxRunes...)
	textLen := int32(len(allRunes))

	// FIB length is fixed for the minimal Word 97 structure written below.
	// Store the text immediately after it as UTF-16LE so the piece table can
	// point at a single uncompressed Unicode piece.
	fibSize := serializedFIBSize()
	textBytes := encodeLegacyUTF16(allRunes)
	textFC := uint32(fibSize)

	var tableStream bytes.Buffer
	pieceTableOffset := tableStream.Len()
	pcdData := buildPieceTableAt(textLen, textFC, false)
	tableStream.WriteByte(0x02)
	_ = binary.Write(&tableStream, binary.LittleEndian, int32(len(pcdData)))
	tableStream.Write(pcdData)

	styleOffset := tableStream.Len()
	styleData := buildLegacyStyleSheetData(styles, fonts)
	tableStream.Write(styleData)

	fontOffset := tableStream.Len()
	fontData := buildLegacyFontTableData(fonts)
	tableStream.Write(fontData)

	ftnRefOffset, ftnRefData := appendLegacyPLC(&tableStream, buildLegacyRefPLC(plan.FootnoteRefs, false))
	ftnTxtOffset, ftnTxtData := appendLegacyPLC(&tableStream, buildLegacyTextPLC(plan.FootnoteBounds))
	hddOffset, hddData := appendLegacyPLC(&tableStream, buildLegacyHddPLC(plan.HeaderBounds))
	atnRefOffset, atnRefData := appendLegacyPLC(&tableStream, buildLegacyAnnotationRefPLC(plan.CommentRefs, doc.Comments))
	atnTxtOffset, atnTxtData := appendLegacyPLC(&tableStream, buildLegacyTextPLC(plan.CommentBounds))
	ednRefOffset, ednRefData := appendLegacyPLC(&tableStream, buildLegacyRefPLC(plan.EndnoteRefs, false))
	ednTxtOffset, ednTxtData := appendLegacyPLC(&tableStream, buildLegacyTextPLC(plan.EndnoteBounds))

	plfLfoOffset := tableStream.Len()
	plfLfoData := buildLegacyPlfLfoData(numbering)
	tableStream.Write(plfLfoData)
	plfLstOffset := tableStream.Len()
	plfLstData := buildLegacyPlfLstData(numbering)
	tableStream.Write(plfLstData)

	plcfSpaOffset := tableStream.Len()
	plcfSpaData := buildLegacyPlcfSpaDataWithShapes(plan.FloatingImages, plan.FloatingShapes, len(mainTextRunes))
	tableStream.Write(plcfSpaData)

	// FKP pages live in WordDocument at 512-byte page boundaries. The table
	// stream carries PlcBte* indexes pointing at those pages.
	papxPageNumber := uint32((fibSize + len(textBytes) + 511) / 512)
	chpxPageNumber := papxPageNumber + 1
	papxPage := buildPapxFKPPage(plan.Paragraphs, textFC)
	chpxPage := buildChpxFKPPage(plan.Characters, textFC, fonts)
	papxBteOffset := tableStream.Len()
	papxBte := buildPlcfBte(textFC, textFC+uint32(len(textBytes)), papxPageNumber)
	tableStream.Write(papxBte)
	chpxBteOffset := tableStream.Len()
	chpxBte := buildPlcfBte(textFC, textFC+uint32(len(textBytes)), chpxPageNumber)
	tableStream.Write(chpxBte)
	dopOffset := tableStream.Len()
	dopData := buildMinimalDOP()
	tableStream.Write(dopData)

	fib := buildFIB(int32(len(mainTextRunes)), int32(pieceTableOffset), int32(len(pcdData)+5),
		int32(styleOffset), int32(len(styleData)),
		int32(plfLfoOffset), int32(len(plfLfoData)), int32(plfLstOffset), int32(len(plfLstData)),
		int32(plcfSpaOffset), int32(len(plcfSpaData)),
		int32(papxBteOffset), int32(len(papxBte)), int32(chpxBteOffset), int32(len(chpxBte)),
		int32(dopOffset), int32(len(dopData)))
	fib.fibRgLw.ccpFtn = int32(len(footnoteRunes))
	fib.fibRgLw.ccpHdd = int32(len(headerRunes))
	fib.fibRgLw.ccpAtn = int32(len(commentRunes))
	fib.fibRgLw.ccpEdn = int32(len(endnoteRunes))
	fib.fibRgLw.ccpTxbx = int32(len(textBoxRunes))
	setLegacyFibEntry(&fib, 2, ftnRefOffset, len(ftnRefData))
	setLegacyFibEntry(&fib, 3, ftnTxtOffset, len(ftnTxtData))
	setLegacyFibEntry(&fib, 11, hddOffset, len(hddData))
	setLegacyFibEntry(&fib, 4, atnRefOffset, len(atnRefData))
	setLegacyFibEntry(&fib, 5, atnTxtOffset, len(atnTxtData))
	setLegacyFibEntry(&fib, 46, ednRefOffset, len(ednRefData))
	setLegacyFibEntry(&fib, 47, ednTxtOffset, len(ednTxtData))
	fib.fibRgFcLcb.entries[15*2] = uint32(fontOffset)
	fib.fibRgFcLcb.entries[15*2+1] = uint32(len(fontData))

	var wordDoc bytes.Buffer
	writeFIB(&wordDoc, fib)
	if wordDoc.Len() != fibSize {
		return fmt.Errorf("legacy doc: FIB size changed: got %d, want %d", wordDoc.Len(), fibSize)
	}
	wordDoc.Write(textBytes)
	for wordDoc.Len() < int(papxPageNumber)*512 {
		wordDoc.WriteByte(0)
	}
	wordDoc.Write(papxPage)
	wordDoc.Write(chpxPage)
	wordDoc.Write(buildLegacyFloatingImageOfficeArt(plan.FloatingImages))
	wordDoc.Write(buildLegacyFloatingShapeOfficeArt(plan.FloatingShapes))

	cw := cfb.NewWriter()
	cw.AddStream("WordDocument", wordDoc.Bytes())
	cw.AddStream("1Table", tableStream.Bytes())
	if len(imageData) > 0 {
		cw.AddStream("Data", imageData)
	}
	if preservationData := buildLegacyPreservationMeta(doc); len(preservationData) > 0 {
		if err := cw.AddStream(legacyPreservationMetaStream, preservationData); err != nil {
			return fmt.Errorf("legacy doc: preservation metadata: %w", err)
		}
	}

	cfbData, err := cw.Bytes()
	if err != nil {
		return fmt.Errorf("cfb write: %w", err)
	}
	_, err = w.Write(cfbData)
	return err
}

type legacyCharacterWriteRun struct {
	StartCP    int
	EndCP      int
	Format     wordir.RunFormat
	Spec       bool
	StyleIndex *uint16
}

type legacyParagraphWriteRun struct {
	StartCP    int
	EndCP      int
	Format     wordir.ParagraphFormat
	StyleIndex uint16
	Table      *legacyTableWriteMeta
}

type legacyTableWriteMeta struct {
	TableFormat wordir.TableFormat
	RowFormat   wordir.TableRowFormat
	Cells       []wordir.TableCell
}

type legacyWritePlan struct {
	Text           string
	MainTextLen    int
	FootnoteText   string
	HeaderText     string
	CommentText    string
	EndnoteText    string
	TextBoxText    string
	Characters     []legacyCharacterWriteRun
	Paragraphs     []legacyParagraphWriteRun
	Images         []*wordir.Image
	FloatingImages []legacyFloatingImageWrite
	FloatingShapes []legacyFloatingShapeWrite
	FootnoteRefs   []int
	CommentRefs    []int
	EndnoteRefs    []int
	FootnoteBounds []int
	HeaderBounds   []int
	CommentBounds  []int
	EndnoteBounds  []int
}

type legacyFloatingImageWrite struct {
	CP    int
	Shape uint32
	Image *wordir.Image
}

type legacyFloatingShapeWrite struct {
	CP      int
	ShapeID uint32
	Shape   *wordir.Shape
}

func buildLegacyWritePlan(doc *wordir.Document, styles legacyStyleWriteCatalog, numbering legacyNumberingWriteCatalog) legacyWritePlan {
	var plan legacyWritePlan
	if doc == nil {
		return plan
	}
	var out strings.Builder
	cp := 0
	appendText := func(text string, format wordir.RunFormat, styleIndex *uint16, special ...bool) {
		if text == "" {
			return
		}
		spec := len(special) > 0 && special[0]
		start := cp
		out.WriteString(text)
		cp += len([]rune(text))
		if len(plan.Characters) > 0 &&
			legacyRunFormatsEqual(plan.Characters[len(plan.Characters)-1].Format, format) &&
			legacyUint16PtrsEqual(plan.Characters[len(plan.Characters)-1].StyleIndex, styleIndex) &&
			plan.Characters[len(plan.Characters)-1].Spec == spec &&
			plan.Characters[len(plan.Characters)-1].EndCP == start {
			plan.Characters[len(plan.Characters)-1].EndCP = cp
		} else {
			plan.Characters = append(plan.Characters, legacyCharacterWriteRun{
				StartCP: start, EndCP: cp, Format: format, Spec: spec, StyleIndex: cloneLegacyUint16(styleIndex),
			})
		}
	}
	appendParagraph := func(paragraph *wordir.Paragraph, inTable bool) {
		if paragraph == nil {
			return
		}
		start := cp
		for _, run := range paragraph.Runs {
			if run.Kind == "image" && run.Image != nil && len(run.Image.Data) > 0 {
				if run.Image.Floating && legacyOfficeArtImageRecordType(run.Image) != 0 {
					shape := uint32(0x400 + len(plan.FloatingImages) + 1)
					anchorCP := cp
					appendText(string(legacyDrawingAnchorChar), run.Format, styles.characterIndex(run.StyleID), true)
					plan.FloatingImages = append(plan.FloatingImages, legacyFloatingImageWrite{
						CP: anchorCP, Shape: shape, Image: cloneLegacyWriteImage(run.Image),
					})
				} else {
					appendText("\x01", run.Format, styles.characterIndex(run.StyleID), true)
					plan.Images = append(plan.Images, cloneLegacyWriteImage(run.Image))
				}
				continue
			}
			if run.Kind == "shape" && run.Shape != nil {
				shapeID := uint32(0x800 + len(plan.FloatingShapes) + 1)
				anchorCP := cp
				appendText(string(legacyDrawingAnchorChar), run.Format, styles.characterIndex(run.StyleID), true)
				shape := cloneLegacyWriteShape(run.Shape)
				plan.FloatingShapes = append(plan.FloatingShapes, legacyFloatingShapeWrite{
					CP: anchorCP, ShapeID: shapeID, Shape: shape,
				})
				appendLegacyShapeTextBoxStories(&plan, shape)
				continue
			}
			if run.Kind == "textbox" && run.TextBox != nil {
				shapeID := uint32(0x800 + len(plan.FloatingShapes) + 1)
				anchorCP := cp
				appendText(string(legacyDrawingAnchorChar), run.Format, styles.characterIndex(run.StyleID), true)
				shape := legacyShapeFromTextBox(run.TextBox)
				plan.FloatingShapes = append(plan.FloatingShapes, legacyFloatingShapeWrite{
					CP: anchorCP, ShapeID: shapeID, Shape: shape,
				})
				appendLegacyTextBoxStory(&plan, run.TextBox)
				continue
			}
			switch run.Kind {
			case "footnoteRef":
				plan.FootnoteRefs = append(plan.FootnoteRefs, cp)
				appendText("\x02", run.Format, styles.characterIndex(run.StyleID), true)
				continue
			case "endnoteRef":
				plan.EndnoteRefs = append(plan.EndnoteRefs, cp)
				appendText("\x02", run.Format, styles.characterIndex(run.StyleID), true)
				continue
			case "commentRef":
				plan.CommentRefs = append(plan.CommentRefs, cp)
				appendText("\x05", run.Format, styles.characterIndex(run.StyleID), true)
				continue
			case "olePlaceholder":
				appendText("\x01", run.Format, styles.characterIndex(run.StyleID), true)
				continue
			}
			var s strings.Builder
			writeLegacyRunText(&s, &run)
			appendText(s.String(), run.Format, styles.characterIndex(run.StyleID))
		}
		appendText("\r", wordir.RunFormat{}, nil)
		format := paragraph.Format
		if inTable {
			format.InTable = true
		}
		if paragraph.Numbering != nil {
			level := paragraph.Numbering.Level
			if level < 0 {
				level = 0
			}
			if level > 8 {
				level = 8
			}
			if ilfo := numbering.ilfoForNumID(paragraph.Numbering.NumID); ilfo > 0 {
				format.BinaryListLevel = &level
				format.BinaryListOverride = &ilfo
			}
		} else if paragraph.List != nil {
			level := paragraph.List.Level
			if level < 0 {
				level = 0
			}
			if level > 8 {
				level = 8
			}
			if ilfo := numbering.ilfoForParagraph(paragraph); ilfo > 0 {
				format.BinaryListLevel = &level
				format.BinaryListOverride = &ilfo
			}
		}
		plan.Paragraphs = append(plan.Paragraphs, legacyParagraphWriteRun{
			StartCP: start, EndCP: cp, Format: format,
			StyleIndex: styles.paragraphIndex(paragraph.StyleID),
		})
	}
	for _, block := range doc.Blocks {
		if block.Paragraph != nil && block.Kind != "table" {
			appendParagraph(block.Paragraph, false)
			continue
		}
		if block.Table != nil {
			for _, row := range block.Table.Rows {
				rowStart := cp
				for cellIndex := range row.Cells {
					cell := &row.Cells[cellIndex]
					for paragraphIndex := range cell.Paragraphs {
						paragraph := &cell.Paragraphs[paragraphIndex]
						for _, run := range paragraph.Runs {
							if run.Kind == "image" && run.Image != nil && len(run.Image.Data) > 0 {
								if run.Image.Floating && legacyOfficeArtImageRecordType(run.Image) != 0 {
									shape := uint32(0x400 + len(plan.FloatingImages) + 1)
									anchorCP := cp
									appendText(string(legacyDrawingAnchorChar), run.Format, styles.characterIndex(run.StyleID), true)
									plan.FloatingImages = append(plan.FloatingImages, legacyFloatingImageWrite{
										CP: anchorCP, Shape: shape, Image: cloneLegacyWriteImage(run.Image),
									})
								} else {
									appendText("\x01", run.Format, styles.characterIndex(run.StyleID), true)
									plan.Images = append(plan.Images, cloneLegacyWriteImage(run.Image))
								}
								continue
							}
							if run.Kind == "shape" && run.Shape != nil {
								shapeID := uint32(0x800 + len(plan.FloatingShapes) + 1)
								anchorCP := cp
								appendText(string(legacyDrawingAnchorChar), run.Format, styles.characterIndex(run.StyleID), true)
								plan.FloatingShapes = append(plan.FloatingShapes, legacyFloatingShapeWrite{
									CP: anchorCP, ShapeID: shapeID, Shape: cloneLegacyWriteShape(run.Shape),
								})
								appendLegacyShapeTextBoxStories(&plan, plan.FloatingShapes[len(plan.FloatingShapes)-1].Shape)
								continue
							}
							switch run.Kind {
							case "footnoteRef":
								plan.FootnoteRefs = append(plan.FootnoteRefs, cp)
								appendText("\x02", run.Format, styles.characterIndex(run.StyleID), true)
								continue
							case "endnoteRef":
								plan.EndnoteRefs = append(plan.EndnoteRefs, cp)
								appendText("\x02", run.Format, styles.characterIndex(run.StyleID), true)
								continue
							case "commentRef":
								plan.CommentRefs = append(plan.CommentRefs, cp)
								appendText("\x05", run.Format, styles.characterIndex(run.StyleID), true)
								continue
							case "olePlaceholder":
								appendText("\x01", run.Format, styles.characterIndex(run.StyleID), true)
								continue
							}
							var s strings.Builder
							writeLegacyRunText(&s, &run)
							appendText(s.String(), run.Format, styles.characterIndex(run.StyleID))
						}
						if paragraphIndex+1 < len(cell.Paragraphs) {
							appendText("\v", wordir.RunFormat{}, nil)
						}
					}
					span := cell.Format.GridSpan
					if span < 1 {
						span = 1
					}
					for marker := 0; marker < span; marker++ {
						appendText("\x07", wordir.RunFormat{}, nil)
					}
				}
				contentEnd := cp
				if contentEnd > rowStart {
					plan.Paragraphs = append(plan.Paragraphs, legacyParagraphWriteRun{
						StartCP: rowStart,
						EndCP:   contentEnd,
						Format:  wordir.ParagraphFormat{InTable: true},
					})
				}
				appendText("\r", wordir.RunFormat{}, nil)
				plan.Paragraphs = append(plan.Paragraphs, legacyParagraphWriteRun{
					StartCP: contentEnd,
					EndCP:   cp,
					Format: wordir.ParagraphFormat{
						InTable:                  true,
						IsTableTrailingParagraph: true,
					},
					Table: &legacyTableWriteMeta{
						TableFormat: block.Table.Format,
						RowFormat:   row.Format,
						Cells:       append([]wordir.TableCell(nil), row.Cells...),
					},
				})
			}
		}
	}
	plan.Text = out.String()
	plan.MainTextLen = len([]rune(plan.Text))
	plan.FootnoteText, plan.FootnoteBounds = buildLegacyNoteStoryText(doc.Footnotes, plan.FootnoteRefs)
	plan.HeaderText, plan.HeaderBounds = buildLegacyHeaderStoryText(doc)
	plan.CommentText, plan.CommentBounds = buildLegacyCommentStoryText(doc.Comments, plan.CommentRefs)
	plan.EndnoteText, plan.EndnoteBounds = buildLegacyNoteStoryText(doc.Endnotes, plan.EndnoteRefs)
	appendLegacySecondaryFormatting(&plan, plan.MainTextLen, doc, styles)
	return plan
}

func buildLegacyNoteStoryText(stories map[int]wordir.Story, refs []int) (string, []int) {
	if len(refs) == 0 || len(stories) == 0 {
		return "", nil
	}
	var out strings.Builder
	bounds := []int{0}
	for i := range refs {
		id := i + 1
		story, ok := stories[id]
		if !ok {
			// Preserve PLC cardinality even when the authored IDs are sparse.
			out.WriteRune('\x02')
			out.WriteRune('\r')
			bounds = append(bounds, len([]rune(out.String())))
			continue
		}
		out.WriteRune('\x02') // automatic note-number marker stripped by the reader
		writeLegacyStoryText(&out, story)
		ensureLegacyParagraphEnd(&out)
		bounds = append(bounds, len([]rune(out.String())))
	}
	return out.String(), bounds
}

func buildLegacyCommentStoryText(stories map[int]wordir.Story, refs []int) (string, []int) {
	if len(refs) == 0 || len(stories) == 0 {
		return "", nil
	}
	var out strings.Builder
	bounds := []int{0}
	for i := range refs {
		story := stories[i+1]
		writeLegacyStoryText(&out, story)
		ensureLegacyParagraphEnd(&out)
		bounds = append(bounds, len([]rune(out.String())))
	}
	return out.String(), bounds
}

func buildLegacyHeaderStoryText(doc *wordir.Document) (string, []int) {
	if doc == nil {
		return "", nil
	}
	// PlcfHdd starts with the six footnote/endnote separator stories, followed
	// by six header/footer slots per section: even/default headers,
	// even/default footers, first header/footer.
	stories := make([]wordir.Story, 0, 6+6*maxLegacyInt(1, len(doc.Sections)))
	stories = append(stories,
		doc.FootnoteSeparator, doc.FootnoteContinuationSep, doc.FootnoteContinuationNotice,
		doc.EndnoteSeparator, doc.EndnoteContinuationSep, doc.EndnoteContinuationNotice,
	)
	sections := doc.Sections
	if len(sections) == 0 {
		sections = []wordir.Section{{}}
	}
	for _, section := range sections {
		var slots [6]wordir.Story
		for _, ref := range section.Headers {
			story := legacyStoryByReference(doc.Headers, ref)
			switch ref.Type {
			case "even":
				slots[0] = story
			case "first":
				slots[4] = story
			default:
				slots[1] = story
			}
		}
		for _, ref := range section.Footers {
			story := legacyStoryByReference(doc.Footers, ref)
			switch ref.Type {
			case "even":
				slots[2] = story
			case "first":
				slots[5] = story
			default:
				slots[3] = story
			}
		}
		// Documents produced directly in IR often populate only the maps. Use
		// conventional keys as a practical fallback for a single section.
		if len(section.Headers) == 0 {
			slots[0] = firstLegacyStory(doc.Headers, "even")
			slots[1] = firstLegacyStory(doc.Headers, "default", "header")
			slots[4] = firstLegacyStory(doc.Headers, "first")
		}
		if len(section.Footers) == 0 {
			slots[2] = firstLegacyStory(doc.Footers, "even")
			slots[3] = firstLegacyStory(doc.Footers, "default", "footer")
			slots[5] = firstLegacyStory(doc.Footers, "first")
		}
		stories = append(stories, slots[:]...)
	}
	var out strings.Builder
	bounds := []int{0}
	for _, story := range stories {
		before := out.Len()
		writeLegacyStoryText(&out, story)
		if out.Len() > before {
			ensureLegacyParagraphEnd(&out)
		}
		bounds = append(bounds, len([]rune(out.String())))
	}
	return out.String(), bounds
}

func legacyStoryByReference(stories map[string]wordir.Story, ref wordir.Reference) wordir.Story {
	if len(stories) == 0 {
		return wordir.Story{}
	}
	if story, ok := stories[ref.PartName]; ok {
		return story
	}
	if story, ok := stories[ref.Type]; ok {
		return story
	}
	return wordir.Story{}
}

func firstLegacyStory(stories map[string]wordir.Story, needles ...string) wordir.Story {
	for key, story := range stories {
		lk := strings.ToLower(key)
		for _, needle := range needles {
			if strings.Contains(lk, strings.ToLower(needle)) {
				return story
			}
		}
	}
	if len(stories) == 1 {
		for _, story := range stories {
			return story
		}
	}
	return wordir.Story{}
}

func writeLegacyStoryText(out *strings.Builder, story wordir.Story) {
	if out == nil {
		return
	}
	if len(story.Blocks) > 0 {
		for _, block := range story.Blocks {
			if block.Paragraph != nil {
				writeLegacyParagraphText(out, block.Paragraph)
				ensureLegacyParagraphEnd(out)
			}
			if block.Table != nil {
				writeLegacyTableText(out, block.Table)
			}
		}
		return
	}
	if story.Text != "" {
		out.WriteString(story.Text)
	}
}

func appendLegacySecondaryFormatting(plan *legacyWritePlan, start int, doc *wordir.Document, styles legacyStyleWriteCatalog) {
	if plan == nil {
		return
	}
	// Secondary stories use default paragraph formatting here, but still get
	// CHPX/PAPX coverage so field control characters and story text are not left
	// outside the writer's formatting tables.
	parts := []string{plan.FootnoteText, plan.HeaderText, plan.CommentText, plan.EndnoteText, plan.TextBoxText}
	cp := start
	for _, text := range parts {
		if text == "" {
			continue
		}
		n := len([]rune(text))
		plan.Characters = append(plan.Characters, legacyCharacterWriteRun{StartCP: cp, EndCP: cp + n})
		plan.Paragraphs = append(plan.Paragraphs, legacyParagraphWriteRun{StartCP: cp, EndCP: cp + n})
		cp += n
	}
}

func maxLegacyInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func legacyRunFormatsEqual(a, b wordir.RunFormat) bool {
	// Formatting coalescing only needs the core subset emitted by this writer.
	return boolPtrEqual(a.Bold, b.Bold) && boolPtrEqual(a.Italic, b.Italic) &&
		boolPtrEqual(a.Strike, b.Strike) && a.Underline == b.Underline &&
		a.Color == b.Color && a.Highlight == b.Highlight &&
		intPtrEqual(a.FontSize, b.FontSize) && a.VertAlignVal == b.VertAlignVal
}
func boolPtrEqual(a, b *bool) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
func intPtrEqual(a, b *int) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }
func legacyUint16PtrsEqual(a, b *uint16) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
func cloneLegacyUint16(value *uint16) *uint16 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func buildLegacyWriteText(doc *wordir.Document) string {
	if doc == nil {
		return ""
	}
	var out strings.Builder
	for _, block := range doc.Blocks {
		switch block.Kind {
		case "paragraph":
			if block.Paragraph != nil {
				writeLegacyParagraphText(&out, block.Paragraph)
				ensureLegacyParagraphEnd(&out)
			}
		case "table":
			if block.Table != nil {
				writeLegacyTableText(&out, block.Table)
			}
		default:
			if block.Paragraph != nil {
				writeLegacyParagraphText(&out, block.Paragraph)
				ensureLegacyParagraphEnd(&out)
			} else if block.Table != nil {
				writeLegacyTableText(&out, block.Table)
			}
		}
	}
	return out.String()
}

func writeLegacyParagraphText(out *strings.Builder, paragraph *wordir.Paragraph) {
	if out == nil || paragraph == nil {
		return
	}
	for _, run := range paragraph.Runs {
		writeLegacyRunText(out, &run)
	}
}

func writeLegacyRunText(out *strings.Builder, run *wordir.Run) {
	if out == nil || run == nil {
		return
	}
	if run.FieldInstr != "" {
		out.WriteRune('\x13')
		out.WriteString(run.FieldInstr)
		out.WriteRune('\x14')
		writeLegacyRunPayload(out, run)
		out.WriteRune('\x15')
		return
	}
	writeLegacyRunPayload(out, run)
}

func writeLegacyRunPayload(out *strings.Builder, run *wordir.Run) {
	switch run.Kind {
	case "tab":
		out.WriteRune('\t')
	case "break":
		switch run.BreakType {
		case "page":
			out.WriteRune('\f')
		case "column":
			out.WriteRune('\x0e')
		default:
			out.WriteRune('\v')
		}
	case "image", "shape", "textbox", "olePlaceholder":
		// Secondary stories cannot use the main-story PlcfSpaMom. Keep a native
		// object-anchor character in the story and preserve the rich object in
		// OfficeMeta so our reader can restore it without dropping layout content.
		out.WriteRune('\x01')
	case "footnoteRef", "endnoteRef":
		out.WriteRune('\x02')
	case "commentRef":
		out.WriteRune('\x05')
	default:
		out.WriteString(run.Text)
	}
}

func writeLegacyTableText(out *strings.Builder, table *wordir.Table) {
	if out == nil || table == nil {
		return
	}
	for _, row := range table.Rows {
		for cellIndex := range row.Cells {
			cell := &row.Cells[cellIndex]
			for paragraphIndex := range cell.Paragraphs {
				writeLegacyParagraphText(out, &cell.Paragraphs[paragraphIndex])
				if paragraphIndex+1 < len(cell.Paragraphs) {
					out.WriteRune('\v')
				}
			}
			// 0x07 is the legacy table cell/row terminator used by Word's text
			// stream. The parser combines it with PAPX table properties when
			// available; preserving it is still preferable to silently dropping
			// table content in a minimal writer.
			out.WriteRune('\x07')
		}
		ensureLegacyParagraphEnd(out)
	}
}

func ensureLegacyParagraphEnd(out *strings.Builder) {
	if out == nil {
		return
	}
	s := out.String()
	if s == "" || s[len(s)-1] != '\r' {
		out.WriteRune('\r')
	}
}

func encodeLegacyUTF16(text []rune) []byte {
	var out bytes.Buffer
	for _, r := range text {
		if r <= 0xffff {
			_ = binary.Write(&out, binary.LittleEndian, uint16(r))
			continue
		}
		r -= 0x10000
		hi := uint16(0xd800 + (r >> 10))
		lo := uint16(0xdc00 + (r & 0x3ff))
		_ = binary.Write(&out, binary.LittleEndian, hi)
		_ = binary.Write(&out, binary.LittleEndian, lo)
	}
	return out.Bytes()
}

func serializedFIBSize() int {
	var b bytes.Buffer
	writeFIB(&b, buildFIB(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0))
	return b.Len()
}

// fib holds the minimal File Information Block for Word 97.
type fib struct {
	base        fibBase
	fibRgW      fibRgW
	fibRgLw     fibRgLw
	fibRgFcLcb  fibRgFcLcb
	fibRgCswNew fibRgCswNew
}

type fibBase struct {
	WIdent    uint16
	nFib      uint16
	nProduct  uint16
	lid       uint16
	pnNext    uint16
	flags     uint16
	nFibBack  uint16
	lKey      uint32
	envr      uint8
	flags2    uint8
	reserved1 uint16
	reserved2 uint16
	reserved3 uint32
	reserved4 uint32
}

// fibRgW contains the Word 97 FibRgW97 reserved word array.
type fibRgW struct {
	csw      uint16
	reserved [14]uint16
}

// fibRgLw holds the long values (cslw = 0x16 = 22 entries).
type fibRgLw struct {
	cslw        uint16
	reservedPre [3]int32 // slots 0..2 precede ccpText in FibRgLw97
	ccpText     int32    // slot 3
	ccpFtn      int32
	ccpHdd      int32
	ccpMcr      int32
	ccpAtn      int32
	ccpEdn      int32
	ccpTxbx     int32
	ccpHdrTxbx  int32
	reserved    [11]int32
}

// fibRgFcLcb holds the file offsets and sizes (cbRgFcLcb = 0x0088 = 136 entries for Word 97).
type fibRgFcLcb struct {
	cbRgFcLcb uint16
	// We only care about a few entries:
	// [12] = fcPlcfBteChpx / lcbPlcfBteChpx
	// [13] = fcPlcfBtePapx / lcbPlcfBtePapx
	// [33] = fcClx / lcbClx
	// [48] = fcDop / lcbDop
	entries [136]uint32
}

type fibRgCswNew struct {
	cswNew  uint16
	nFibNew uint16
}

func buildFIB(textLen, fcClx, lcbClx, fcStshf, lcbStshf, fcPlfLfo, lcbPlfLfo, fcPlfLst, lcbPlfLst, fcPlcfSpa, lcbPlcfSpa, fcPlcfBtePapx, lcbPlcfBtePapx, fcPlcfBteChpx, lcbPlcfBteChpx, fcDop, lcbDop int32) fib {
	f := fib{
		base: fibBase{
			WIdent:   0xA5EC,
			nFib:     0x00C1,
			nProduct: 0x0000,
			lid:      0x0409,
			pnNext:   0,
			flags:    1 << 9, // fWhichTblStm: use 1Table
			nFibBack: 0x0065,
			lKey:     0,
			envr:     0,
		},
		fibRgW: fibRgW{csw: 14},
		fibRgLw: fibRgLw{
			cslw:       0x0016,
			ccpText:    textLen,
			ccpFtn:     0,
			ccpHdd:     0,
			ccpMcr:     0,
			ccpAtn:     0,
			ccpEdn:     0,
			ccpTxbx:    0,
			ccpHdrTxbx: 0,
		},
		fibRgFcLcb:  fibRgFcLcb{cbRgFcLcb: 0x0088},
		fibRgCswNew: fibRgCswNew{cswNew: 0, nFibNew: 0x00C1},
	}

	// Set fc/lcb pairs in FibRgFcLcb97.
	// Entry 1: fcStshf / lcbStshf
	f.fibRgFcLcb.entries[1*2] = uint32(fcStshf)
	f.fibRgFcLcb.entries[1*2+1] = uint32(lcbStshf)
	// Entries 20/21: list overrides and list definitions.
	f.fibRgFcLcb.entries[legacyFibFcLcbPlfLfoIndex*2] = uint32(fcPlfLfo)
	f.fibRgFcLcb.entries[legacyFibFcLcbPlfLfoIndex*2+1] = uint32(lcbPlfLfo)
	f.fibRgFcLcb.entries[legacyFibFcLcbPlfLstIndex*2] = uint32(fcPlfLst)
	f.fibRgFcLcb.entries[legacyFibFcLcbPlfLstIndex*2+1] = uint32(lcbPlfLst)
	// Entry 40: PlcfSpaMom. This matches real Word 97+ binary fixtures. The
	// reader still accepts historical slot-30 files produced by older versions
	// of this package.
	f.fibRgFcLcb.entries[40*2] = uint32(fcPlcfSpa)
	f.fibRgFcLcb.entries[40*2+1] = uint32(lcbPlcfSpa)
	// Entry 12: fcPlcfBteChpx / lcbPlcfBteChpx
	f.fibRgFcLcb.entries[12*2] = uint32(fcPlcfBteChpx)
	f.fibRgFcLcb.entries[12*2+1] = uint32(lcbPlcfBteChpx)
	// Entry 13: fcPlcfBtePapx / lcbPlcfBtePapx
	f.fibRgFcLcb.entries[13*2] = uint32(fcPlcfBtePapx)
	f.fibRgFcLcb.entries[13*2+1] = uint32(lcbPlcfBtePapx)
	// Entry 33: fcClx / lcbClx
	f.fibRgFcLcb.entries[33*2] = uint32(fcClx)
	f.fibRgFcLcb.entries[33*2+1] = uint32(lcbClx)
	// Entry 48: fcDop / lcbDop
	f.fibRgFcLcb.entries[48*2] = uint32(fcDop)
	f.fibRgFcLcb.entries[48*2+1] = uint32(lcbDop)

	return f
}

func writeFIB(b *bytes.Buffer, f fib) {
	binary.Write(b, binary.LittleEndian, f.base)
	binary.Write(b, binary.LittleEndian, f.fibRgW.csw)
	for i := 0; i < int(f.fibRgW.csw) && i < len(f.fibRgW.reserved); i++ {
		binary.Write(b, binary.LittleEndian, f.fibRgW.reserved[i])
	}
	binary.Write(b, binary.LittleEndian, f.fibRgLw.cslw)
	for i := range f.fibRgLw.reservedPre {
		binary.Write(b, binary.LittleEndian, f.fibRgLw.reservedPre[i])
	}
	binary.Write(b, binary.LittleEndian, f.fibRgLw.ccpText)
	binary.Write(b, binary.LittleEndian, f.fibRgLw.ccpFtn)
	binary.Write(b, binary.LittleEndian, f.fibRgLw.ccpHdd)
	binary.Write(b, binary.LittleEndian, f.fibRgLw.ccpMcr)
	binary.Write(b, binary.LittleEndian, f.fibRgLw.ccpAtn)
	binary.Write(b, binary.LittleEndian, f.fibRgLw.ccpEdn)
	binary.Write(b, binary.LittleEndian, f.fibRgLw.ccpTxbx)
	binary.Write(b, binary.LittleEndian, f.fibRgLw.ccpHdrTxbx)
	for i := range f.fibRgLw.reserved {
		binary.Write(b, binary.LittleEndian, f.fibRgLw.reserved[i])
	}
	binary.Write(b, binary.LittleEndian, f.fibRgFcLcb.cbRgFcLcb)
	for i := 0; i < 136; i++ {
		binary.Write(b, binary.LittleEndian, f.fibRgFcLcb.entries[i])
	}
	binary.Write(b, binary.LittleEndian, f.fibRgCswNew.cswNew)
	if f.fibRgCswNew.cswNew > 0 {
		binary.Write(b, binary.LittleEndian, f.fibRgCswNew.nFibNew)
	}
}

// buildPieceTable creates a single uncompressed Unicode piece starting at the
// supplied WordDocument stream offset.
func buildPieceTable(textLen int32) []byte {
	return buildPieceTableAt(textLen, 0, false)
}

func buildPieceTableAt(textLen int32, fc uint32, compressed bool) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, int32(0))
	_ = binary.Write(&b, binary.LittleEndian, textLen)

	// PCD: 2-byte flags followed by fcCompressed and a 2-byte prm.
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))
	fcRaw := fc
	if compressed {
		fcRaw = fc*2 | 0x40000000
	}
	_ = binary.Write(&b, binary.LittleEndian, fcRaw)
	_ = binary.Write(&b, binary.LittleEndian, uint16(0))
	return b.Bytes()
}

func appendLegacyPLC(table *bytes.Buffer, data []byte) (int, []byte) {
	if len(data) == 0 {
		return 0, nil
	}
	offset := table.Len()
	table.Write(data)
	return offset, data
}

func setLegacyFibEntry(f *fib, index, offset, length int) {
	if f == nil || length == 0 || index < 0 || index >= len(f.fibRgFcLcb.entries)/2 {
		return
	}
	f.fibRgFcLcb.entries[index*2] = uint32(offset)
	f.fibRgFcLcb.entries[index*2+1] = uint32(length)
}

func buildLegacyRefPLC(cps []int, annotation bool) []byte {
	if len(cps) == 0 {
		return nil
	}
	var b bytes.Buffer
	for _, cp := range cps {
		_ = binary.Write(&b, binary.LittleEndian, uint32(cp))
	}
	_ = binary.Write(&b, binary.LittleEndian, uint32(cps[len(cps)-1]+1))
	if annotation {
		for range cps {
			b.Write(make([]byte, 30))
		}
	} else {
		for range cps {
			_ = binary.Write(&b, binary.LittleEndian, uint16(0))
		}
	}
	return b.Bytes()
}

func buildLegacyAnnotationRefPLC(cps []int, stories map[int]wordir.Story) []byte {
	if len(cps) == 0 {
		return nil
	}
	var b bytes.Buffer
	for _, cp := range cps {
		_ = binary.Write(&b, binary.LittleEndian, uint32(cp))
	}
	_ = binary.Write(&b, binary.LittleEndian, uint32(cps[len(cps)-1]+1))
	for i := range cps {
		story := stories[i+1]
		var rec [30]byte
		initials := []rune(story.Author)
		if len(initials) > 10 {
			initials = initials[:10]
		}
		binary.LittleEndian.PutUint16(rec[0:2], uint16(len(initials)))
		for j, r := range initials {
			if r > 0xffff {
				r = '?'
			}
			binary.LittleEndian.PutUint16(rec[2+j*2:4+j*2], uint16(r))
		}
		binary.LittleEndian.PutUint32(rec[26:30], encodeLegacyDTTMDate(story.Date))
		b.Write(rec[:])
	}
	return b.Bytes()
}

func encodeLegacyDTTMDate(value string) uint32 {
	var year, month, day int
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d-%d-%d", &year, &month, &day); err != nil {
		return 0
	}
	if year < 1900 || year > 2411 || month < 1 || month > 12 || day < 1 || day > 31 {
		return 0
	}
	return uint32((year-1900)&0x1ff)<<20 | uint32(month&0xf)<<16 | uint32(day&0x1f)<<11
}

func buildLegacyTextPLC(bounds []int) []byte {
	if len(bounds) < 2 {
		return nil
	}
	var b bytes.Buffer
	for _, cp := range bounds {
		_ = binary.Write(&b, binary.LittleEndian, uint32(cp))
	}
	return b.Bytes()
}

func buildLegacyHddPLC(bounds []int) []byte {
	if len(bounds) < 2 {
		return nil
	}
	var b bytes.Buffer
	for _, cp := range bounds {
		_ = binary.Write(&b, binary.LittleEndian, uint32(cp))
	}
	// PlcfHdd carries one extra terminal CP beyond the per-story boundaries.
	_ = binary.Write(&b, binary.LittleEndian, uint32(bounds[len(bounds)-1]))
	return b.Bytes()
}

func buildPlcfBte(startFC, endFC, pageNumber uint32) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, startFC)
	_ = binary.Write(&b, binary.LittleEndian, endFC)
	_ = binary.Write(&b, binary.LittleEndian, pageNumber)
	return b.Bytes()
}

type legacyFontWriteCatalog struct {
	Names []string
	Index map[string]uint16
	Info  map[string]wordir.FontFamilyInfo
}

func buildLegacyFontWriteCatalog(doc *wordir.Document, plan legacyWritePlan, styles legacyStyleWriteCatalog) legacyFontWriteCatalog {
	c := legacyFontWriteCatalog{Index: map[string]uint16{}, Info: map[string]wordir.FontFamilyInfo{}}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if _, ok := c.Index[key]; ok {
			return
		}
		c.Index[key] = uint16(len(c.Names))
		c.Names = append(c.Names, name)
		if doc != nil {
			if info, ok := doc.FontFamilies[key]; ok {
				c.Info[key] = info
			} else {
				for familyName, info := range doc.FontFamilies {
					if strings.EqualFold(strings.TrimSpace(familyName), name) {
						c.Info[key] = info
						break
					}
				}
			}
		}
	}
	addFormat := func(f wordir.RunFormat) { add(f.FontASCII); add(f.FontHAnsi); add(f.FontEastAsia); add(f.FontCS) }
	for _, run := range plan.Characters {
		addFormat(run.Format)
	}
	for _, st := range styles.Entries {
		addFormat(st.RunFormat)
	}
	if doc != nil {
		keys := make([]string, 0, len(doc.FontFamilies))
		for name := range doc.FontFamilies {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			add(name)
		}
	}
	return c
}

func (c legacyFontWriteCatalog) index(name string) *uint16 {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	v, ok := c.Index[strings.ToLower(name)]
	if !ok {
		return nil
	}
	return &v
}

func appendLegacyFontSPRM(out []byte, code uint16, index *uint16) []byte {
	if index == nil {
		return out
	}
	return appendSPRM16(out, code, int(*index))
}

func buildLegacyFontTableData(c legacyFontWriteCatalog) []byte {
	if len(c.Names) == 0 {
		return nil
	}
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, uint16(len(c.Names)))
	for _, name := range c.Names {
		b.Write(buildLegacyFFNRecord(name, c.Info[strings.ToLower(strings.TrimSpace(name))]))
	}
	return b.Bytes()
}

func buildLegacyFFNRecord(name string, info wordir.FontFamilyInfo) []byte {
	rs := utf16.Encode([]rune(strings.TrimSpace(name)))
	if len(rs) > 107 {
		rs = rs[:107]
	}
	rs = append(rs, 0)
	rec := make([]byte, 40+len(rs)*2)
	rec[0] = byte(len(rec) - 1)
	rec[1] = legacyFFNPitchBits(info.Pitch) | legacyFFNFamilyBits(info.Family)<<4
	rec[4] = legacyFFNCharsetByte(info.Charset)
	if panose, ok := legacyDecodeHexBytes(info.Panose1, 10); ok {
		copy(rec[6:16], panose)
	}
	for i, r := range rs {
		binary.LittleEndian.PutUint16(rec[40+i*2:42+i*2], r)
	}
	return rec
}

func legacyFFNPitchBits(v string) byte {
	switch strings.ToLower(v) {
	case "fixed":
		return 1
	case "variable":
		return 2
	}
	return 0
}
func legacyFFNFamilyBits(v string) byte {
	switch strings.ToLower(v) {
	case "roman":
		return 1
	case "swiss":
		return 2
	case "modern":
		return 3
	case "script":
		return 4
	case "decorative":
		return 5
	}
	return 0
}
func legacyFFNCharsetByte(v string) byte {
	v = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(v)), "0x")
	n, err := strconv.ParseUint(v, 16, 8)
	if err != nil {
		return 0
	}
	return byte(n)
}
func legacyDecodeHexBytes(v string, want int) ([]byte, bool) {
	v = strings.TrimSpace(v)
	if len(v) != want*2 {
		return nil, false
	}
	out := make([]byte, want)
	for i := range out {
		n, err := strconv.ParseUint(v[i*2:i*2+2], 16, 8)
		if err != nil {
			return nil, false
		}
		out[i] = byte(n)
	}
	return out, true
}

func buildChpxFKPPage(runs []legacyCharacterWriteRun, textFC uint32, fonts legacyFontWriteCatalog) []byte {
	page := make([]byte, 512)
	if len(runs) == 0 {
		runs = []legacyCharacterWriteRun{{StartCP: 0, EndCP: 1}}
	}
	if len(runs) > 30 {
		runs = runs[:30]
	}
	crun := len(runs)
	for i, run := range runs {
		binary.LittleEndian.PutUint32(page[i*4:i*4+4], textFC+uint32(run.StartCP*2))
	}
	binary.LittleEndian.PutUint32(page[crun*4:crun*4+4], textFC+uint32(runs[crun-1].EndCP*2))
	rgbx := (crun + 1) * 4
	dataOff := 510
	for i := crun - 1; i >= 0; i-- {
		grp := encodeChpxWriteRun(runs[i], fonts)
		if len(grp) == 0 {
			page[rgbx+i] = 0
			continue
		}
		need := len(grp) + 1
		if dataOff-need < rgbx+crun {
			continue
		}
		dataOff -= need
		if dataOff&1 != 0 {
			dataOff--
		}
		page[dataOff] = byte(len(grp))
		copy(page[dataOff+1:], grp)
		page[rgbx+i] = byte(dataOff / 2)
	}
	page[511] = byte(crun)
	return page
}

func buildPapxFKPPage(runs []legacyParagraphWriteRun, textFC uint32) []byte {
	page := make([]byte, 512)
	if len(runs) == 0 {
		runs = []legacyParagraphWriteRun{{StartCP: 0, EndCP: 1}}
	}
	if len(runs) > 20 {
		runs = runs[:20]
	}
	cpara := len(runs)
	for i, run := range runs {
		binary.LittleEndian.PutUint32(page[i*4:i*4+4], textFC+uint32(run.StartCP*2))
	}
	binary.LittleEndian.PutUint32(page[cpara*4:cpara*4+4], textFC+uint32(runs[cpara-1].EndCP*2))
	rgbx := (cpara + 1) * 4
	dataOff := 510
	for i := cpara - 1; i >= 0; i-- {
		grp := encodePapxWriteRun(runs[i])
		// standard PapxInFkp: cb halfwords incl istd, istd(2), grpprl padded even
		if len(grp)&1 != 0 {
			grp = append(grp, 0)
		}
		cb := 1 + len(grp)/2
		need := 3 + len(grp)
		if dataOff-need < rgbx+cpara*13 {
			continue
		}
		dataOff -= need
		if dataOff&1 != 0 {
			dataOff--
		}
		page[dataOff] = byte(cb)
		binary.LittleEndian.PutUint16(page[dataOff+1:dataOff+3], runs[i].StyleIndex)
		copy(page[dataOff+3:], grp)
		page[rgbx+i*13] = byte(dataOff / 2)
	}
	page[511] = byte(cpara)
	return page
}

func appendSPRM8(out []byte, code uint16, value byte) []byte {
	return append(out, byte(code), byte(code>>8), value)
}
func appendSPRM16(out []byte, code uint16, value int) []byte {
	out = append(out, byte(code), byte(code>>8))
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], uint16(int16(value)))
	return append(out, b[:]...)
}
func appendSPRM32(out []byte, code uint16, value uint32) []byte {
	out = append(out, byte(code), byte(code>>8))
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], value)
	return append(out, b[:]...)
}
func encodeChpxWriteRun(run legacyCharacterWriteRun, fonts legacyFontWriteCatalog) []byte {
	out := encodeChpxGrpprl(run.Format)
	out = appendLegacyFontSPRM(out, 0x4A4F, fonts.index(run.Format.FontASCII))
	out = appendLegacyFontSPRM(out, 0x4A50, fonts.index(run.Format.FontEastAsia))
	out = appendLegacyFontSPRM(out, 0x4A51, fonts.index(run.Format.FontCS))
	if run.StyleIndex != nil {
		out = appendSPRM16(out, 0x4A30, int(*run.StyleIndex)) // sprmCIstd
	}
	if run.Spec {
		out = appendSPRM8(out, 0x0855, 1) // sprmCFSpec
	}
	return out
}

func cloneLegacyWriteImage(in *wordir.Image) *wordir.Image {
	if in == nil {
		return nil
	}
	out := *in
	out.Data = append([]byte(nil), in.Data...)
	out.WrapPolygon = append([][2]float64(nil), in.WrapPolygon...)
	return &out
}

func appendLegacyShapeTextBoxStories(plan *legacyWritePlan, shape *wordir.Shape) {
	if plan == nil || shape == nil {
		return
	}
	if shape.TextBox != nil {
		appendLegacyTextBoxStory(plan, shape.TextBox)
	}
	for i := range shape.Children {
		appendLegacyShapeTextBoxStories(plan, &shape.Children[i])
	}
}

func appendLegacyTextBoxStory(plan *legacyWritePlan, tb *wordir.TextBox) {
	if plan == nil || tb == nil || len(tb.Blocks) == 0 {
		return
	}
	var story strings.Builder
	for _, block := range tb.Blocks {
		if block.Paragraph != nil {
			for i := range block.Paragraph.Runs {
				writeLegacyRunText(&story, &block.Paragraph.Runs[i])
			}
			story.WriteByte('\r')
			continue
		}
		if block.Table != nil {
			for ri := range block.Table.Rows {
				for ci := range block.Table.Rows[ri].Cells {
					cell := &block.Table.Rows[ri].Cells[ci]
					for _, p := range cell.Paragraphs {
						for i := range p.Runs {
							writeLegacyRunText(&story, &p.Runs[i])
						}
						story.WriteByte('\r')
					}
					if ci+1 < len(block.Table.Rows[ri].Cells) {
						story.WriteByte('\t')
					}
				}
			}
		}
	}
	text := story.String()
	if text == "" {
		return
	}
	if plan.TextBoxText != "" {
		plan.TextBoxText += "\f"
	}
	plan.TextBoxText += text
}

func legacyShapeFromTextBox(tb *wordir.TextBox) *wordir.Shape {
	if tb == nil {
		return nil
	}
	shape := &wordir.Shape{
		Kind: "rect", Floating: true,
		X: tb.X, Y: tb.Y, Width: tb.Width, Height: tb.Height,
		XRelative: tb.XRelative, YRelative: tb.YRelative,
		XAlign: tb.XAlign, YAlign: tb.YAlign, ZIndex: tb.ZIndex,
		BehindDoc: tb.BehindDoc, WrapType: tb.WrapType, WrapSide: tb.WrapSide,
		WrapPolygon: append([][2]float64(nil), tb.WrapPolygon...),
		DistTop:     tb.DistTop, DistBottom: tb.DistBottom, DistLeft: tb.DistLeft, DistRight: tb.DistRight,
		Locked: tb.Locked, LayoutInCell: tb.LayoutInCell, AllowOverlap: tb.AllowOverlap,
		FillColor: tb.FillColor, StrokeColor: tb.BorderColor,
		StrokeWidth: int64(tb.BorderWidth * 12700),
		RotationDeg: tb.RotationDeg, FlipH: tb.FlipH, FlipV: tb.FlipV,
		TextBox: tb,
	}
	return shape
}

func buildLegacyImageDataStream(images []*wordir.Image) []byte {
	if len(images) == 0 {
		return nil
	}
	var out bytes.Buffer
	for _, image := range images {
		if image == nil || len(image.Data) == 0 {
			continue
		}
		// The current legacy reader deliberately scans Data for self-delimiting
		// raster payloads. Preserve exact bytes and add small zero padding so
		// adjacent signatures cannot overlap during producer/consumer scanning.
		if detectLegacyImageMIME(image.Data) == "" {
			continue
		}
		for out.Len()%4 != 0 {
			out.WriteByte(0)
		}
		out.Write(image.Data)
	}
	return out.Bytes()
}

func encodeChpxGrpprl(f wordir.RunFormat) []byte {
	var out []byte
	appendToggle := func(code uint16, value *bool) {
		if value == nil {
			return
		}
		v := byte(0)
		if *value {
			v = 1
		}
		out = appendSPRM8(out, code, v)
	}
	appendToggle(0x0835, f.Bold)
	appendToggle(0x0836, f.Italic)
	appendToggle(0x0837, f.Strike)
	appendToggle(0x083A, f.SmallCaps)
	appendToggle(0x083B, f.Caps)
	appendToggle(0x083C, f.Vanish)
	appendToggle(0x0838, f.Outline)
	appendToggle(0x0839, f.Shadow)
	appendToggle(0x0854, f.Imprint)
	appendToggle(0x0858, f.Emboss)
	appendToggle(0x2A53, f.DoubleStrike)
	appendToggle(0x0875, f.NoProof)
	appendToggle(0x083F, f.RTL)
	appendToggle(0x0811, f.WebHidden)
	appendToggle(0x0882, f.ComplexScript)
	appendToggle(0x0868, f.UsePageSetup)
	appendToggle(0x085A, f.RTL)
	appendToggle(0x0818, f.SpecVanish)
	if f.TextAnimation != "" {
		if v := legacyTextAnimationSfx(f.TextAnimation); v >= 0 {
			out = appendSPRM8(out, 0x2859, byte(v))
		}
	}
	if f.FELayout != nil {
		flags := byte(0)
		if f.FELayout.CombineCharacters {
			flags |= 0x01
		}
		if f.FELayout.CombineBrackets {
			flags |= 0x02
		}
		if f.FELayout.Vertical {
			flags |= 0x04
		}
		if f.FELayout.VerticalCompressed {
			flags |= 0x08
		}
		out = appendSPRMVar(out, 0xCA78, []byte{flags, byte(f.FELayout.CombineCharCount)})
	}

	if f.FontSize != nil {
		out = appendSPRM16(out, 0x4A43, *f.FontSize)
	}
	if f.FontSizeCS != nil {
		out = appendSPRM16(out, 0x4A61, *f.FontSizeCS)
	}
	if f.BoldCS != nil {
		appendToggle(0x085C, f.BoldCS)
	}
	if f.ItalicCS != nil {
		appendToggle(0x085D, f.ItalicCS)
	}

	switch f.Underline {
	case "single":
		out = appendSPRM8(out, 0x2A3E, 1)
	case "words":
		out = appendSPRM8(out, 0x2A3E, 2)
	case "double":
		out = appendSPRM8(out, 0x2A3E, 3)
	case "dotted":
		out = appendSPRM8(out, 0x2A3E, 4)
	case "thick":
		out = appendSPRM8(out, 0x2A3E, 6)
	case "dash":
		out = appendSPRM8(out, 0x2A3E, 7)
	case "dotDash":
		out = appendSPRM8(out, 0x2A3E, 9)
	case "dotDotDash":
		out = appendSPRM8(out, 0x2A3E, 10)
	case "wave":
		out = appendSPRM8(out, 0x2A3E, 11)
	}
	if color, ok := legacyColorRefValue(f.Color); ok {
		out = appendSPRM32(out, 0x6870, color)
	}
	if color, ok := legacyColorRefValue(f.UnderlineColor); ok {
		out = appendSPRM32(out, 0x6877, color)
	}
	if ico := legacyHighlightICO(f.Highlight); ico != 0 {
		out = appendSPRM8(out, 0x2A0C, ico)
	}
	if f.Background != "" {
		if shd := encodeLegacySHD(f.Background); len(shd) == 10 {
			out = appendSPRMVar(out, 0xCA71, shd)
		}
	}
	switch f.VertAlignVal {
	case "superscript":
		out = appendSPRM8(out, 0x2A48, 1)
	case "subscript":
		out = appendSPRM8(out, 0x2A48, 2)
	}
	if f.Position != nil {
		out = appendSPRM16(out, 0x4845, *f.Position)
	}
	if f.CharacterSpacing != nil {
		out = appendSPRM16(out, 0x8840, *f.CharacterSpacing)
	}
	if f.Kerning != nil {
		out = appendSPRM16(out, 0x484B, *f.Kerning)
	}
	if f.CharScale != nil && *f.CharScale >= 1 && *f.CharScale <= 600 {
		out = appendSPRM16(out, 0x4852, *f.CharScale)
	}
	if f.FitTextWidth != nil && *f.FitTextWidth > 0 {
		out = appendSPRMVar(out, 0xCA76, []byte{byte(*f.FitTextWidth), byte(*f.FitTextWidth >> 8)})
	}
	if f.Borders != nil {
		if side := firstLegacyRunBorder(f.Borders); side != nil {
			var brc [4]byte
			writeLegacyBRC80(brc[:], side)
			out = append(out, 0x65, 0x68) // sprmCBrc80
			out = append(out, brc[:]...)
		}
	}
	if lid := legacyBCP47ToLID(f.Lang); lid != 0 {
		out = appendSPRM16(out, 0x4A41, int(lid))
	}
	return out
}

func legacyTextAnimationSfx(value string) int {
	switch value {
	case "none":
		return 0
	case "lights":
		return 1
	case "blinkBackground":
		return 2
	case "sparkle":
		return 3
	case "antsBlack":
		return 4
	case "antsRed":
		return 5
	case "shimmer":
		return 6
	default:
		return -1
	}
}

func firstLegacyRunBorder(b *wordir.RunBorders) *wordir.BorderSide {
	if b == nil {
		return nil
	}
	for _, side := range []*wordir.BorderSide{b.Top, b.Left, b.Bottom, b.Right} {
		if side != nil {
			return side
		}
	}
	return nil
}

func legacyColorRefValue(color string) (uint32, bool) {
	r, g, b, ok := parseLegacyHexRGB(color)
	if !ok {
		return 0, false
	}
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16, true
}

func legacyHighlightICO(name string) byte {
	switch name {
	case "black":
		return 1
	case "blue":
		return 2
	case "cyan":
		return 3
	case "green":
		return 4
	case "magenta":
		return 5
	case "red":
		return 6
	case "yellow":
		return 7
	case "white":
		return 8
	case "darkBlue":
		return 9
	case "darkCyan":
		return 10
	case "darkGreen":
		return 11
	case "darkMagenta":
		return 12
	case "darkRed":
		return 13
	case "darkYellow":
		return 14
	case "darkGray":
		return 15
	case "lightGray":
		return 16
	default:
		return 0
	}
}

func legacyBCP47ToLID(tag string) uint16 {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "zh-tw":
		return 0x0404
	case "de-de":
		return 0x0407
	case "en-us":
		return 0x0409
	case "es-es":
		return 0x040A
	case "fr-fr":
		return 0x040C
	case "it-it":
		return 0x0410
	case "ja-jp":
		return 0x0411
	case "ko-kr":
		return 0x0412
	case "nl-nl":
		return 0x0413
	case "pl-pl":
		return 0x0415
	case "pt-br":
		return 0x0416
	case "ru-ru":
		return 0x0419
	case "uk-ua":
		return 0x0422
	case "zh-cn":
		return 0x0804
	case "en-gb":
		return 0x0809
	case "es-mx":
		return 0x080A
	case "pt-pt":
		return 0x0816
	default:
		return 0
	}
}

func encodePapxWriteRun(run legacyParagraphWriteRun) []byte {
	out := encodePapxGrpprl(run.Format)
	if run.Table != nil {
		out = append(out, encodeLegacyTableGrpprl(*run.Table)...)
	}
	return out
}

func appendSPRMVar(out []byte, code uint16, payload []byte) []byte {
	out = append(out, byte(code), byte(code>>8))
	if code == 0xD608 {
		// TDefTable carries a two-byte cb where total operand size is cb+1.
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16(len(payload)+1))
		out = append(out, b[:]...)
		return append(out, payload...)
	}
	if len(payload) > 255 {
		payload = payload[:255]
	}
	out = append(out, byte(len(payload)))
	return append(out, payload...)
}

func encodeLegacyTableGrpprl(meta legacyTableWriteMeta) []byte {
	var out []byte
	tf := meta.TableFormat
	rf := meta.RowFormat

	switch tf.Alignment {
	case "center":
		out = appendSPRM16(out, 0x5400, 1)
	case "right":
		out = appendSPRM16(out, 0x5400, 2)
	case "left":
		out = appendSPRM16(out, 0x5400, 0)
	}
	if tf.Indent != 0 {
		out = appendSPRM16(out, 0x9601, tf.Indent)
	}
	if tf.CellSpacing != 0 {
		out = appendSPRM16(out, 0x9602, tf.CellSpacing/2)
	}
	if rf.Height != nil {
		height := *rf.Height
		if rf.HeightRule == "exact" && height > 0 {
			height = -height
		}
		out = appendSPRM16(out, 0x9407, height)
	}
	if rf.CantSplit {
		out = appendSPRM8(out, 0x3403, 1)
	}
	if rf.IsHeader {
		out = appendSPRM8(out, 0x3404, 1)
	}
	if tf.Width != nil || tf.WidthType != "" {
		fts := byte(2)
		switch tf.WidthType {
		case "nil":
			fts = 1
		case "auto", "":
			fts = 2
		case "dxa":
			fts = 3
		case "pct":
			fts = 4
		}
		width := 0
		if tf.Width != nil {
			width = *tf.Width
		}
		out = append(out, 0x14, 0xF6, fts, byte(width), byte(width>>8))
	}
	if tf.Layout != "" {
		v := byte(1)
		if tf.Layout == "fixed" {
			v = 0
		}
		out = appendSPRM8(out, 0x3615, v)
	}
	if tf.BidiVisual {
		out = appendSPRM16(out, 0x560B, 1)
	}
	if tf.Borders != nil {
		if payload := encodeLegacyTableBorders(tf.Borders); len(payload) == 24 {
			out = appendSPRMVar(out, 0xD605, payload)
		}
	}
	if len(meta.Cells) > 0 {
		if padding, ok := commonLegacyTableCellPadding(tf, meta.Cells); ok {
			mask := byte(0)
			if padding.Top > 0 {
				mask |= 0x01
			}
			if padding.Left > 0 {
				mask |= 0x02
			}
			if padding.Bottom > 0 {
				mask |= 0x04
			}
			if padding.Right > 0 {
				mask |= 0x08
			}
			// D634 represents one width for the selected sides. Emit one record per
			// distinct side width so asymmetric Word cell margins survive.
			for _, side := range []struct {
				mask  byte
				width int
			}{
				{0x01, padding.Top}, {0x02, padding.Left}, {0x04, padding.Bottom}, {0x08, padding.Right},
			} {
				if side.width <= 0 {
					continue
				}
				out = appendSPRMVar(out, 0xD634, []byte{0, byte(len(meta.Cells)), side.mask, 3, byte(side.width), byte(side.width >> 8)})
			}
			_ = mask
		}
	}
	if payload := encodeLegacyTDefTable(meta); len(payload) > 0 {
		out = appendSPRMVar(out, 0xD608, payload)
	}
	physicalIndex := 0
	for _, cell := range meta.Cells {
		span := cell.Format.GridSpan
		if span < 1 {
			span = 1
		}
		first := physicalIndex
		lim := physicalIndex + span
		if cell.Format.Shading != "" {
			if shd := encodeLegacySHD(cell.Format.Shading); len(shd) == 10 {
				payload := append([]byte{byte(first), byte(lim)}, shd...)
				out = appendSPRMVar(out, 0xD62D, payload)
			}
		}
		if flow := legacyTextFlowValue(cell.Format.TextDirection); flow != 0 {
			out = append(out, 0x29, 0x76, byte(first), byte(lim), byte(flow), byte(flow>>8))
		}
		physicalIndex = lim
	}
	return out
}

type legacyCellPaddingSet struct {
	Top, Left, Bottom, Right int
}

func commonLegacyTableCellPadding(table wordir.TableFormat, cells []wordir.TableCell) (legacyCellPaddingSet, bool) {
	padding := legacyCellPaddingSet{
		Top: table.CellPadTop, Left: table.CellPadLeft,
		Bottom: table.CellPadBottom, Right: table.CellPadRight,
	}
	if padding.Top != 0 || padding.Left != 0 || padding.Bottom != 0 || padding.Right != 0 {
		return padding, true
	}
	if len(cells) == 0 {
		return padding, false
	}
	first := cells[0].Format
	padding = legacyCellPaddingSet{Top: first.CellPadTop, Left: first.CellPadLeft, Bottom: first.CellPadBottom, Right: first.CellPadRight}
	for _, cell := range cells[1:] {
		if cell.Format.CellPadTop != padding.Top || cell.Format.CellPadLeft != padding.Left ||
			cell.Format.CellPadBottom != padding.Bottom || cell.Format.CellPadRight != padding.Right {
			return legacyCellPaddingSet{}, false
		}
	}
	return padding, padding.Top != 0 || padding.Left != 0 || padding.Bottom != 0 || padding.Right != 0
}

func encodeLegacyTableBorders(b *wordir.TableBorders) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, 24)
	writeLegacyBRC80(out[0:4], b.Top)
	writeLegacyBRC80(out[4:8], firstLegacyBorder(b.Left, b.Start))
	writeLegacyBRC80(out[8:12], b.Bottom)
	writeLegacyBRC80(out[12:16], firstLegacyBorder(b.Right, b.End))
	writeLegacyBRC80(out[16:20], b.InsideH)
	writeLegacyBRC80(out[20:24], b.InsideV)
	return out
}

func firstLegacyBorder(a, b *wordir.BorderSide) *wordir.BorderSide {
	if a != nil {
		return a
	}
	return b
}

func encodeLegacySHD(color string) []byte {
	r, g, b, ok := parseLegacyHexRGB(color)
	if !ok {
		return nil
	}
	out := make([]byte, 10)
	// Solid shading: cvFore supplies the visible color, ipat=1.
	out[0], out[1], out[2] = r, g, b
	out[4], out[5], out[6] = r, g, b
	binary.LittleEndian.PutUint16(out[8:10], 1)
	return out
}

func parseLegacyHexRGB(color string) (byte, byte, byte, bool) {
	s := strings.TrimSpace(strings.TrimPrefix(color, "#"))
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	var v uint64
	for _, ch := range s {
		v <<= 4
		switch {
		case ch >= '0' && ch <= '9':
			v += uint64(ch - '0')
		case ch >= 'a' && ch <= 'f':
			v += uint64(ch - 'a' + 10)
		case ch >= 'A' && ch <= 'F':
			v += uint64(ch - 'A' + 10)
		default:
			return 0, 0, 0, false
		}
	}
	return byte(v >> 16), byte(v >> 8), byte(v), true
}

func legacyTextFlowValue(direction string) uint16 {
	switch direction {
	case "tbRl":
		return 1
	case "btLr":
		return 3
	case "tbRlV":
		return 5
	default:
		return 0
	}
}

func encodeLegacyTDefTable(meta legacyTableWriteMeta) []byte {
	type physicalCell struct {
		Cell       wordir.TableCell
		Source     int
		MergeState byte // 0 none, 1 continuation, 2 restart
	}
	var physical []physicalCell
	for sourceIndex, cell := range meta.Cells {
		span := cell.Format.GridSpan
		if span < 1 {
			span = 1
		}
		for part := 0; part < span; part++ {
			state := byte(0)
			if span > 1 {
				if part == 0 {
					state = 2
				} else {
					state = 1
				}
			} else if cell.Format.HMerge {
				state = 2
			}
			physical = append(physical, physicalCell{Cell: cell, Source: sourceIndex, MergeState: state})
		}
	}
	if len(physical) == 0 || len(physical) > 63 {
		return nil
	}

	count := len(physical)
	payload := make([]byte, 1+(count+1)*2+count*20)
	payload[0] = byte(count)

	indent := meta.TableFormat.Indent
	cursor := indent
	binary.LittleEndian.PutUint16(payload[1:3], uint16(int16(cursor)))
	gridIndex := 0
	for i, item := range physical {
		width := legacyPhysicalTableCellWidth(meta.TableFormat, item.Cell, item.Source, gridIndex, count)
		cursor += width
		off := 1 + (i+1)*2
		binary.LittleEndian.PutUint16(payload[off:off+2], uint16(int16(cursor)))
		gridIndex++
	}

	tcOffset := 1 + (count+1)*2
	for i, item := range physical {
		cell := item.Cell
		off := tcOffset + i*20
		var flags uint16
		switch item.MergeState {
		case 1:
			flags |= 1
		case 2:
			flags |= 2
		}
		switch cell.Format.VMerge {
		case "continue":
			flags |= 1 << 5
		case "restart":
			flags |= 3 << 5
		}
		switch cell.Format.VertAlign {
		case "center":
			flags |= 1 << 7
		case "bottom":
			flags |= 2 << 7
		}
		switch cell.Format.WidthType {
		case "pct":
			flags |= 2 << 9
		case "dxa":
			flags |= 3 << 9
		}
		binary.LittleEndian.PutUint16(payload[off:off+2], flags)
		if cell.Format.Width != nil {
			width := *cell.Format.Width
			span := cell.Format.GridSpan
			if span > 1 {
				width /= span
			}
			binary.LittleEndian.PutUint16(payload[off+2:off+4], uint16(width))
		}
		writeLegacyBRC80(payload[off+4:off+8], borderSideAt(cell.Format.Borders, "top"))
		writeLegacyBRC80(payload[off+8:off+12], borderSideAt(cell.Format.Borders, "left"))
		writeLegacyBRC80(payload[off+12:off+16], borderSideAt(cell.Format.Borders, "bottom"))
		writeLegacyBRC80(payload[off+16:off+20], borderSideAt(cell.Format.Borders, "right"))
	}
	return payload
}

func legacyPhysicalTableCellWidth(table wordir.TableFormat, cell wordir.TableCell, sourceIndex, gridIndex, physicalCount int) int {
	if gridIndex < len(table.ColumnWidths) && table.ColumnWidths[gridIndex] > 0 {
		return table.ColumnWidths[gridIndex]
	}
	if cell.Format.Width != nil && *cell.Format.Width > 0 {
		span := cell.Format.GridSpan
		if span < 1 {
			span = 1
		}
		return *cell.Format.Width / span
	}
	if sourceIndex < len(table.ColumnWidths) && table.ColumnWidths[sourceIndex] > 0 {
		return table.ColumnWidths[sourceIndex]
	}
	if table.Width != nil && *table.Width > 0 && physicalCount > 0 {
		return *table.Width / physicalCount
	}
	return 1440
}

func borderSideAt(borders *wordir.CellBorders, side string) *wordir.BorderSide {
	if borders == nil {
		return nil
	}
	switch side {
	case "top":
		return borders.Top
	case "left":
		if borders.Left != nil {
			return borders.Left
		}
		return borders.Start
	case "bottom":
		return borders.Bottom
	case "right":
		if borders.Right != nil {
			return borders.Right
		}
		return borders.End
	}
	return nil
}

func writeLegacyBRC80(dst []byte, side *wordir.BorderSide) {
	if len(dst) < 4 {
		return
	}
	if side == nil || side.Val == "" || side.Val == "none" {
		for i := 0; i < 4; i++ {
			dst[i] = 0
		}
		return
	}
	width := int(side.SizePt * 8)
	if width < 1 {
		width = 1
	}
	if width > 255 {
		width = 255
	}
	dst[0] = byte(width)
	switch side.Val {
	case "double":
		dst[1] = 3
	case "dotted":
		dst[1] = 4
	case "dashed":
		dst[1] = 6
	case "thick":
		dst[1] = 2
	default:
		dst[1] = 1
	}
	dst[2] = legacyICOFromHex(side.Color)
	space := int(side.SpacePt)
	if space < 0 {
		space = 0
	}
	if space > 31 {
		space = 31
	}
	dst[3] = byte(space)
}

func legacyICOFromHex(color string) byte {
	switch strings.ToUpper(strings.TrimPrefix(color, "#")) {
	case "000000":
		return 1
	case "0000FF":
		return 2
	case "00FFFF":
		return 3
	case "00FF00":
		return 4
	case "FF00FF":
		return 5
	case "FF0000":
		return 6
	case "FFFF00":
		return 7
	case "FFFFFF":
		return 8
	case "000080":
		return 9
	case "008080":
		return 10
	case "008000":
		return 11
	case "800080":
		return 12
	case "800000":
		return 13
	case "808000":
		return 14
	case "808080":
		return 15
	case "C0C0C0":
		return 16
	default:
		return 0
	}
}

func encodePapxGrpprl(f wordir.ParagraphFormat) []byte {
	var out []byte
	if f.BinaryListLevel != nil && *f.BinaryListLevel >= 0 && *f.BinaryListLevel <= 8 {
		out = appendSPRM8(out, 0x260A, byte(*f.BinaryListLevel))
	}
	if f.BinaryListOverride != nil && *f.BinaryListOverride > 0 {
		out = appendSPRM16(out, 0x460B, *f.BinaryListOverride)
	}
	switch f.Alignment {
	case "center":
		out = appendSPRM8(out, 0x2403, 1)
	case "right":
		out = appendSPRM8(out, 0x2403, 2)
	case "both", "justify":
		out = appendSPRM8(out, 0x2403, 3)
	}
	if f.KeepLines {
		out = appendSPRM8(out, 0x2405, 1)
	}
	if f.KeepNext {
		out = appendSPRM8(out, 0x2406, 1)
	}
	if f.PageBreakBefore {
		out = appendSPRM8(out, 0x2407, 1)
	}
	if f.WidowControl {
		out = appendSPRM8(out, 0x2431, 1)
	}
	if f.ContextualSpacing {
		out = appendSPRM8(out, 0x246D, 1)
	}
	if f.NoLineNumber {
		out = appendSPRM8(out, 0x240C, 1)
	}
	if f.NoAutoHyphenation {
		out = appendSPRM8(out, 0x242A, 1)
	}
	if f.Kinsoku {
		out = appendSPRM8(out, 0x2433, 1)
	}
	if f.WordWrap != nil {
		v := byte(0)
		if *f.WordWrap {
			v = 1
		}
		out = appendSPRM8(out, 0x2434, v)
	}
	if f.OverflowPunct {
		out = appendSPRM8(out, 0x2435, 1)
	}
	if f.TopLinePunct {
		out = appendSPRM8(out, 0x2436, 1)
	}
	if f.AutoSpaceDE {
		out = appendSPRM8(out, 0x2437, 1)
	}
	if f.AutoSpaceDN {
		out = appendSPRM8(out, 0x2438, 1)
	}
	if f.Bidi {
		out = appendSPRM8(out, 0x2441, 1)
	}
	if f.AdjustRightInd {
		out = appendSPRM8(out, 0x2448, 1)
	}
	if f.UsePageSetup {
		out = appendSPRM8(out, 0x2447, 1)
	}
	if f.IsTableCellTrailingParagraph {
		out = appendSPRM8(out, 0x244B, 1)
	}
	switch f.FontAlign {
	case "auto":
		out = appendSPRM16(out, 0x4439, 0)
	case "top":
		out = appendSPRM16(out, 0x4439, 1)
	case "center":
		out = appendSPRM16(out, 0x4439, 2)
	case "baseline":
		out = appendSPRM16(out, 0x4439, 3)
	case "bottom":
		out = appendSPRM16(out, 0x4439, 4)
	}
	if f.MirrorIndents {
		out = appendSPRM8(out, 0x2470, 1)
	}
	if f.NoAllowOverlap {
		out = appendSPRM8(out, 0x2462, 1)
	}
	if f.IndentLeft != nil {
		out = appendSPRM16(out, 0x840E, *f.IndentLeft)
	}
	if f.IndentRight != nil {
		out = appendSPRM16(out, 0x840F, *f.IndentRight)
	}
	if f.IndentFirstLine != nil {
		out = appendSPRM16(out, 0x8411, *f.IndentFirstLine)
	} else if f.IndentHanging != nil {
		out = appendSPRM16(out, 0x8411, -*f.IndentHanging)
	}
	if f.SpacingBefore != nil {
		out = appendSPRM16(out, 0xA413, *f.SpacingBefore)
	}
	if f.SpacingAfter != nil {
		out = appendSPRM16(out, 0xA414, *f.SpacingAfter)
	}
	if len(f.TabStops) > 0 {
		if payload := encodeLegacyTabStops(f.TabStops); len(payload) > 0 {
			out = appendSPRMVar(out, 0xC615, payload)
		}
	}
	if f.SpacingBeforeAuto {
		out = appendSPRM8(out, 0x245B, 1)
	}
	if f.SpacingAfterAuto {
		out = appendSPRM8(out, 0x245C, 1)
	}
	if f.LineSpacing != nil && *f.LineSpacing > 0 {
		dy := *f.LineSpacing
		fMult := 0
		switch f.LineRule {
		case "auto":
			fMult = 1
		case "atLeast":
			dy = -dy
		}
		payload := []byte{byte(dy), byte(dy >> 8), byte(fMult), byte(fMult >> 8)}
		out = append(out, 0x12, 0x64) // sprmPLspd
		out = append(out, payload...)
	}
	if f.Borders != nil {
		for _, border := range []struct {
			code uint16
			side *wordir.BorderSide
		}{
			{0x6424, f.Borders.Top},
			{0x6425, f.Borders.Left},
			{0x6426, f.Borders.Bottom},
			{0x6427, f.Borders.Right},
		} {
			if border.side == nil {
				continue
			}
			var brc [4]byte
			writeLegacyBRC80(brc[:], border.side)
			out = append(out, byte(border.code), byte(border.code>>8))
			out = append(out, brc[:]...)
		}
	}
	if frame := f.BinaryFrame; frame != nil {
		if frame.X != 0 {
			out = appendSPRM16(out, 0x8418, frame.X)
		}
		if frame.Y != 0 {
			out = appendSPRM16(out, 0x8419, frame.Y)
		}
		if frame.W != 0 {
			out = appendSPRM16(out, 0x841A, frame.W)
		}
		if frame.H != 0 {
			out = appendSPRM16(out, 0x442B, frame.H)
		}
		if frame.HSpace != 0 {
			out = appendSPRM16(out, 0x842F, frame.HSpace)
		}
		if frame.VSpace != 0 {
			out = appendSPRM16(out, 0x842E, frame.VSpace)
		}
		if frame.AnchorLock {
			out = appendSPRM8(out, 0x2430, 1)
		}
		if frame.WrapType != "" {
			out = appendSPRM8(out, 0x2423, legacyFrameWrapCode(frame.WrapType))
		}
		if frame.XAlign != "" || frame.YAlign != "" {
			pc := legacyFrameXAlignCode(frame.XAlign) | legacyFrameYAlignCode(frame.YAlign)<<2
			out = appendSPRM8(out, 0x261B, pc)
		}
		if frame.DropCap != "" && frame.DropCap != "none" {
			kind := uint16(1)
			if frame.DropCap == "margin" {
				kind = 2
			}
			dcs := kind | uint16(maxLegacyInt(frame.Lines, 1)&0x1f)<<3
			out = appendSPRM16(out, 0x442C, int(dcs))
		}
		if flow := legacyTextFlowValue(frame.TextDirection); flow != 0 {
			out = appendSPRM16(out, 0x443A, int(flow))
		}
	}
	if f.OutlineLevel != nil && *f.OutlineLevel >= 0 && *f.OutlineLevel <= 9 {
		out = appendSPRM8(out, 0x2640, byte(*f.OutlineLevel))
	}
	if f.Shading != "" {
		if shd := encodeLegacySHD(f.Shading); len(shd) == 10 {
			out = appendSPRMVar(out, 0xC64D, shd)
		}
	}
	if f.InTable {
		out = appendSPRM8(out, 0x2416, 1)
	}
	if f.IsTableTrailingParagraph {
		out = appendSPRM8(out, 0x2417, 1)
	}
	return out
}

func encodeLegacyTabStops(stops []wordir.TabStop) []byte {
	if len(stops) == 0 {
		return nil
	}
	ordered := append([]wordir.TabStop(nil), stops...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Pos < ordered[j].Pos })
	if len(ordered) > 63 {
		ordered = ordered[:63]
	}
	payload := []byte{0, byte(len(ordered))} // no deletes; add all authored stops
	for _, tab := range ordered {
		payload = append(payload, byte(tab.Pos), byte(tab.Pos>>8))
	}
	for _, tab := range ordered {
		payload = append(payload, legacyTabDescriptor(tab))
	}
	return payload
}

func legacyTabDescriptor(tab wordir.TabStop) byte {
	var align byte
	switch tab.Algn {
	case "center":
		align = 1
	case "right":
		align = 2
	case "decimal":
		align = 3
	case "bar":
		align = 4
	}
	var leader byte
	switch tab.Leader {
	case "dot":
		leader = 1
	case "hyphen":
		leader = 2
	case "underscore":
		leader = 3
	case "middleDot":
		leader = 5
	}
	return align | leader<<3
}

func legacyFrameWrapCode(value string) byte {
	switch value {
	case "notBeside":
		return 1
	case "none":
		return 3
	case "tight":
		return 4
	case "through":
		return 5
	default:
		return 2
	}
}

func legacyFrameXAlignCode(value string) byte {
	switch value {
	case "center":
		return 1
	case "right":
		return 2
	default:
		return 0
	}
}

func legacyFrameYAlignCode(value string) byte {
	switch value {
	case "center":
		return 1
	case "bottom":
		return 2
	default:
		return 0
	}
}

// buildEmptyFKP creates a minimal formatted disk page (FKP) for PAPX or CHPX.
func buildEmptyFKP(isPara bool) []byte {
	// FKP = 512 bytes for v3 (default).
	// First 4 bytes = count of runs (crun).
	// Rest = rgfc (4 bytes each) + array of offsets + padding.
	b := make([]byte, 512)
	b[511] = 1 // crun = 1
	return b
}

// buildMinimalDOP creates a minimal Document Properties (DOP) structure.
func buildMinimalDOP() []byte {
	// DOP for Word 97 is 84 bytes (Dop97).
	b := make([]byte, 84)
	return b
}
