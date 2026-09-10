package legacy

import (
	"strings"

	wordir "office/pkg/word/ir"
)

type binaryWordDocumentInput struct {
	Runs            []binaryWordRun
	BookmarkStarts  map[int][]string
	BookmarkEnds    map[string]int
	ListCatalog     *binaryWordListCatalog
	StyleCatalog    *binaryWordStyleCatalog
	StyleNameByIstd map[int]string
}

// buildBinaryWordDocument assembles already-decoded binary Word runs into the
// public IR. Word97 and genuine pre-97 construct its semantic inputs separately.
func buildBinaryWordDocument(filename string, input binaryWordDocumentInput) *wordir.Document {
	runs := input.Runs
	bookmarkStarts := input.BookmarkStarts
	bookmarkEnds := input.BookmarkEnds
	listCatalog := input.ListCatalog
	styleCatalog := input.StyleCatalog
	styleNameByIstd := input.StyleNameByIstd
	lineSlices := splitDocRunsIntoLines(runs)
	doc := &wordir.Document{Filename: filename}
	if styleCatalog != nil {
		doc.DefaultParagraphStyleID = styleCatalog.DefaultParagraphStyleID
		doc.Styles = make(map[string]wordir.Style, len(styleCatalog.ParagraphStyles))
		for id, style := range styleCatalog.ParagraphStyles {
			doc.Styles[id] = style
		}
		doc.CharacterStyles = make(map[string]wordir.CharacterStyle, len(styleCatalog.CharacterStyles))
		for id, style := range styleCatalog.CharacterStyles {
			doc.CharacterStyles[id] = style
		}
		if style, ok := styleCatalog.ParagraphStyles[styleCatalog.DefaultParagraphStyleID]; ok {
			doc.DefaultParagraphFormat = legacyCloneParagraphFormat(style.Format)
			doc.DefaultRunFormat = legacyCloneRunFormat(style.RunFormat)
		}
	} else {
		fontSize := 24
		doc.DefaultRunFormat = wordir.RunFormat{FontSize: &fontSize, FontASCII: "Times New Roman", FontHAnsi: "Times New Roman"}
		doc.DefaultParagraphFormat = wordir.ParagraphFormat{Alignment: "left"}
	}
	binaryListState := &legacyBinaryListState{counters: map[string]int{}, lastEndCPByIlfo: map[int]int{}, lastListLevelByIlfo: map[int]string{}, lastListFormatByIlfo: map[int]string{}, lastParagraphGeometryByIlfo: map[int]string{}, lastParagraphKeepByIlfo: map[int]string{}, lastParagraphLayoutByIlfo: map[int]string{}, lastRunContextByIlfo: map[int]string{}}
	activeBookmarkEnds := map[string]int{}

	for index := 0; index < len(lineSlices); {
		start := index
		if lineSliceIsLegacyListBoundary(lineSlices[index]) {
			binaryListState.prevWasBinary = false
			index++
			continue
		}
		if (lineSliceInTable(lineSlices[index]) && !lineSliceInTableFromLegacyHintOnly(lineSlices[index])) || lineSliceHasBinaryTableMetadata(lineSlices[index]) {
			var tableLineSlices [][]binaryWordRun
			for index < len(lineSlices) && ((lineSliceInTable(lineSlices[index]) && !lineSliceInTableFromLegacyHintOnly(lineSlices[index])) || lineSliceIsExplicitRowMark(lineSlices[index]) || lineSliceHasBinaryTableMetadata(lineSlices[index]) || lineSliceIsPreWord97RowMarkContent(lineSlices[index])) {
				tableLineSlices = append(tableLineSlices, lineSlices[index])
				index++
			}
			rowGroups := collectLegacyBinaryTableRowGroups(tableLineSlices)
			if !appendBinaryTableToDoc(doc, rowGroups) {
				index = start
			} else {
				binaryListState.prevWasBinary = false
				continue
			}
		}
		lineBookmarks := bookmarksForLineSlice(lineSlices[index], bookmarkStarts)
		lineStartCP, lineEndCP, hasLineCP := lineSliceCPRange(lineSlices[index])
		if hasLineCP {
			for name, endCP := range activeBookmarkEnds {
				if endCP <= lineStartCP {
					delete(activeBookmarkEnds, name)
				}
			}
		}
		for _, name := range lineBookmarks {
			if endCP, ok := bookmarkEnds[name]; ok {
				activeBookmarkEnds[name] = endCP
			}
		}
		lineBookmarks = mergeLineAndActiveBookmarks(lineBookmarks, lineStartCP, lineEndCP, hasLineCP, activeBookmarkEnds)
		line := strings.TrimSpace(joinDocRunsText(lineSlices[index]))
		if line == "" && len(lineBookmarks) == 0 && !lineSliceHasSpecialRuns(lineSlices[index]) && !lineSliceIsExplicitBlankParagraph(lineSlices[index]) {
			binaryListState.prevWasBinary = false
			index++
			continue
		}
		if lineSliceHasNativeCellMarks(lineSlices[index]) && !lineSliceHasLegacyListFormatting(lineSlices[index]) {
			tableLines := make([]string, 0, 4)
			tableLineSlices := make([][]binaryWordRun, 0, 4)
			for index < len(lineSlices) {
				if !lineSliceHasNativeCellMarks(lineSlices[index]) || lineSliceHasLegacyListFormatting(lineSlices[index]) {
					break
				}
				candidate := strings.TrimSpace(joinDocRunsText(lineSlices[index]))
				if candidate == "" {
					break
				}
				tableLines = append(tableLines, candidate)
				tableLineSlices = append(tableLineSlices, lineSlices[index])
				index++
			}
			if table, trailing := buildLegacyTable(len(doc.Tables), tableLines, len(doc.Paragraphs)); table != nil {
				applyLegacyHeuristicTableProps(table, tableLineSlices)
				absorbLegacyTableBanner(doc, table)
				styleLegacyRecoveredPriceTable(table)
				doc.Tables = append(doc.Tables, *table)
				doc.Blocks = append(doc.Blocks, wordir.Block{Index: len(doc.Blocks), Kind: "table", Table: &doc.Tables[len(doc.Tables)-1]})
				for _, trailingLine := range trailing {
					paragraph := buildLegacyParagraph(len(doc.Paragraphs), trailingLine)
					doc.Paragraphs = append(doc.Paragraphs, paragraph)
					doc.Blocks = append(doc.Blocks, wordir.Block{Index: len(doc.Blocks), Kind: "paragraph", Paragraph: &doc.Paragraphs[len(doc.Paragraphs)-1]})
				}
				binaryListState.prevWasBinary = false
				continue
			}
			index = start
			lineBookmarks = bookmarksForLineSlice(lineSlices[index], bookmarkStarts)
			lineStartCP, lineEndCP, hasLineCP = lineSliceCPRange(lineSlices[index])
			if hasLineCP {
				for name, endCP := range activeBookmarkEnds {
					if endCP <= lineStartCP {
						delete(activeBookmarkEnds, name)
					}
				}
			}
			for _, name := range lineBookmarks {
				if endCP, ok := bookmarkEnds[name]; ok {
					activeBookmarkEnds[name] = endCP
				}
			}
			lineBookmarks = mergeLineAndActiveBookmarks(lineBookmarks, lineStartCP, lineEndCP, hasLineCP, activeBookmarkEnds)
			line = strings.TrimSpace(joinDocRunsText(lineSlices[index]))
		}
		structuredPapx := styleCatalog != nil && styleCatalog.StructuredPapx
		allowHeuristicTable := !structuredPapx || lineSliceInTable(lineSlices[index])
		if allowHeuristicTable && !lineSliceHasLegacyListFormatting(lineSlices[index]) && isLegacyTableCollectionLine(line) {
			explicitDelimitedTable := isExplicitLegacyTableLine(line) || isLegacyTableSeparatorLine(line)
			tableLines := make([]string, 0, 4)
			tableLineSlices := make([][]binaryWordRun, 0, 4)
			for index < len(lineSlices) {
				if lineSliceHasLegacyListFormatting(lineSlices[index]) {
					break
				}
				candidate := strings.TrimSpace(joinDocRunsText(lineSlices[index]))
				if !isLegacyTableCollectionLine(candidate) {
					break
				}
				if !isLegacyTableSeparatorLine(candidate) {
					tableLines = append(tableLines, candidate)
					tableLineSlices = append(tableLineSlices, lineSlices[index])
				}
				index++
			}
			if !explicitDelimitedTable && len(tableLines) < 2 {
				index = start
			} else if table, trailing := buildLegacyTable(len(doc.Tables), tableLines, len(doc.Paragraphs)); table != nil {
				applyLegacyHeuristicTableProps(table, tableLineSlices)
				absorbLegacyTableBanner(doc, table)
				styleLegacyRecoveredPriceTable(table)
				doc.Tables = append(doc.Tables, *table)
				doc.Blocks = append(doc.Blocks, wordir.Block{Index: len(doc.Blocks), Kind: "table", Table: &doc.Tables[len(doc.Tables)-1]})
				for _, trailingLine := range trailing {
					paragraph := buildLegacyParagraph(len(doc.Paragraphs), trailingLine)
					doc.Paragraphs = append(doc.Paragraphs, paragraph)
					doc.Blocks = append(doc.Blocks, wordir.Block{Index: len(doc.Blocks), Kind: "paragraph", Paragraph: &doc.Paragraphs[len(doc.Paragraphs)-1]})
				}
				binaryListState.prevWasBinary = false
				continue
			} else {
				index = start
			}
		}
		paragraph, fromStyle := buildLegacyParagraphFromRuns(len(doc.Paragraphs), lineSlices[index], lineBookmarks, listCatalog, styleCatalog, styleNameByIstd)
		applyLegacyBinaryListContinuation(&paragraph, binaryListState, listCatalog, lineSlices[index], fromStyle)
		doc.Paragraphs = append(doc.Paragraphs, paragraph)
		doc.Blocks = append(doc.Blocks, wordir.Block{Index: len(doc.Blocks), Kind: "paragraph", Paragraph: &doc.Paragraphs[len(doc.Paragraphs)-1]})
		index++
	}
	rebindLegacyTopLevelBlocks(doc)
	return doc
}
