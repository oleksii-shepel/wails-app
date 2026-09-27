package legacy

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	officeartir "office/pkg/officeart/ir"
	officeartlegacy "office/pkg/officeart/legacy"
	officeartescher "office/pkg/officeart/legacy/escher"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"office/pkg/ooxml/cfb"
	"office/pkg/ooxml/drawing"
	presentationir "office/pkg/presentation/ir"
)

func TestDiagnostic53446PptStructure(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "53446.ppt")
	f, err := cfb.Open(path)
	if err != nil {
		t.Fatalf("open cfb: %v", err)
	}

	for _, name := range []string{"Current User", "PowerPoint Document", "Pictures", "PP40", "Text_Content"} {
		stream, ok := f.Stream(name)
		if !ok {
			t.Logf("stream %q: missing", name)
			continue
		}
		head := stream
		if len(head) > 32 {
			head = head[:32]
		}
		t.Logf("stream %q: len=%d head=% x", name, len(stream), head)
	}

	currentUser, cuOK := f.Stream("Current User")
	powerPointDocument, docOK := f.Stream("PowerPoint Document")
	if !docOK {
		t.Fatal("PowerPoint Document stream missing")
	}

	if record, payload, next, ok := readPptRecord(powerPointDocument, 0); ok {
		t.Logf("raw record@0: version=%d instance=%d type=0x%04X len=%d next=%d payload=%d",
			record.Version, record.Instance, record.Type, record.Length, next, len(payload))
		logLegacyPptDocumentChildren(t, payload, 0)
	} else {
		t.Logf("raw record@0: INVALID")
	}

	if cuOK {
		if record, payload, next, ok := readPptRecord(currentUser, 0); ok {
			t.Logf("Current User record@0: version=%d instance=%d type=0x%04X len=%d next=%d payload=%d",
				record.Version, record.Instance, record.Type, record.Length, next, len(payload))
		} else {
			t.Logf("Current User record@0: INVALID")
		}
		if off, ok := parseCurrentUserOffset(currentUser); ok {
			t.Logf("Current User edit offset=%d", off)
			if record, payload, next, ok := readPptRecord(powerPointDocument, off); ok {
				t.Logf("UserEdit@%d: version=%d instance=%d type=0x%04X len=%d next=%d payload=%d",
					off, record.Version, record.Instance, record.Type, record.Length, next, len(payload))
			} else {
				t.Logf("UserEdit@%d: INVALID", off)
			}
		} else {
			t.Logf("Current User edit offset: unavailable")
		}
	}

	if ctx, ok := extractPptPersistContext(f, powerPointDocument); ok {
		t.Logf("persist context: documentOffset=%d persistEntries=%d slidePersistIDs=%v notesInstance=%d",
			ctx.documentOffset, len(ctx.persistDirectory), ctx.slidePersistIDs, ctx.notesListInstance)
		if record, payload, next, ok := readPptRecord(powerPointDocument, ctx.documentOffset); ok {
			t.Logf("persist Document@%d: version=%d instance=%d type=0x%04X len=%d next=%d payload=%d",
				ctx.documentOffset, record.Version, record.Instance, record.Type, record.Length, next, len(payload))
			logLegacyPptDocumentChildren(t, payload, ctx.documentOffset)
		} else {
			t.Logf("persist Document@%d: INVALID", ctx.documentOffset)
		}
	} else {
		t.Logf("persist context: FAILED")
	}

	fontTable := parseLegacyPptFontTable(powerPointDocument)
	colorScheme := parseLegacyPptColorScheme(powerPointDocument)
	rootSlides := extractLegacyPptSlidesFromRootDocument(powerPointDocument, fontTable, colorScheme, nil)
	t.Logf("root structural slides=%d", len(rootSlides))
	for i, slide := range rootSlides {
		t.Logf("root slide[%d]: persistID=%d fragments=%d text=%q", i, slide.PersistID, len(slide.Fragments), diagnosticLegacySlideText(slide))
	}
}

func logLegacyPptDocumentChildren(t *testing.T, payload []byte, base int) {
	t.Helper()
	counts := map[uint16]int{}
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			t.Logf("Document child parse stopped: relative=%d absolute=%d", pos, base+8+pos)
			break
		}
		counts[record.Type]++
		if record.Type == pptRTSlideListWithText {
			slides := parseSlideListWithText(childPayload, nil, nil, nil)
			t.Logf("SlideListWithText: relative=%d absolute=%d instance=%d len=%d parsedSlides=%d",
				pos, base+8+pos, record.Instance, record.Length, len(slides))
		}
		pos = next
	}
	t.Logf("Document child record counts=%v", counts)
}

func diagnosticLegacySlideText(slide legacySlideText) string {
	var parts []string
	for _, fragment := range slide.Fragments {
		if s := strings.TrimSpace(fragment.Text); s != "" {
			parts = append(parts, s)
		}
	}
	text := strings.Join(parts, " | ")
	if len(text) > 160 {
		text = text[:160]
	}
	return text
}

func TestParsePptExtractsPictureShapesFromSlideContainers(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-pictures.ppt")
	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if len(pres.Slides) == 0 {
		t.Fatal("expected slides")
	}
	allowedMIME := map[string]bool{
		"image/jpeg":    true,
		"image/png":     true,
		"image/svg+xml": true,
		"image/x-emf":   true,
		"image/x-wmf":   true,
		"image/x-pict":  true,
	}
	wantMIME := map[string]bool{
		"image/jpeg":    false,
		"image/png":     false,
		"image/svg+xml": false,
		"image/x-pict":  false,
	}
	for _, slide := range pres.Slides {
		var imageShapes []*presentationir.Shape
		for _, shape := range slide.Shapes {
			if shape != nil && shape.Kind == presentationir.ShapeImage {
				imageShapes = append(imageShapes, shape)
			}
		}
		if len(imageShapes) != 1 {
			t.Fatalf("slide image shapes = %d (effective scene shapes=%d), want 1 authored image", len(imageShapes), len(slide.Shapes))
		}
		for _, shape := range imageShapes {
			if len(shape.ImageData) == 0 {
				t.Fatal("image shape missing payload")
			}
			if shape.ImageMIME == "" {
				t.Fatal("image shape missing mime type")
			}
			if shape.Bounds.W <= 0 || shape.Bounds.H <= 0 {
				t.Fatalf("image shape bounds = %+v, want non-empty", shape.Bounds)
			}
			if !allowedMIME[shape.ImageMIME] {
				t.Fatalf("unexpected image mime %q", shape.ImageMIME)
			}
			if _, ok := wantMIME[shape.ImageMIME]; ok {
				wantMIME[shape.ImageMIME] = true
			}
		}
	}
	for mime, seen := range wantMIME {
		if !seen {
			t.Fatalf("missing image mime %q in apache-pictures.ppt output", mime)
		}
	}
}

func TestParsePptUsesAnchoredTextShapesFromSlideContainers(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-text_shapes.ppt")
	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if len(pres.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(pres.Slides))
	}
	slide := pres.Slides[0]
	texts := make([]string, 0, len(slide.Shapes))
	for _, shape := range slide.Shapes {
		if shape == nil || shape.Kind != presentationir.ShapeText {
			continue
		}
		if len(shape.Paragraphs) == 0 {
			t.Fatalf("text shape missing paragraphs: %#v", shape)
		}
		texts = append(texts, paragraphText(shape.Paragraphs[0]))
	}
	if len(texts) != 5 {
		t.Fatalf("slide text shapes = %d (effective scene shapes=%d), want 5 authored text shapes: %q", len(texts), len(slide.Shapes), texts)
	}
	wantTexts := []string{"Title Placeholder", "Text in a TextBox", "Rectangle", "Ellipse", "Octagon"}
	for index, want := range wantTexts {
		if texts[index] != want {
			t.Fatalf("shape[%d] text = %q, want %q", index, texts[index], want)
		}
	}
	var titleShape *presentationir.Shape
	for _, shape := range slide.Shapes {
		if shape != nil && shape.Kind == presentationir.ShapeText && len(shape.Paragraphs) > 0 && paragraphText(shape.Paragraphs[0]) == "Title Placeholder" {
			titleShape = shape
			break
		}
	}
	if titleShape == nil {
		t.Fatal("authored title text shape not found in effective scene")
	}
	if titleShape.Placeholder != presentationir.PlaceholderTitle {
		t.Fatalf("title placeholder = %v, want title", titleShape.Placeholder)
	}
	titleBounds, ok := parseLegacyPptClientAnchor([]byte{173, 0, 32, 1, 96, 21, 125, 3})
	if !ok {
		t.Fatal("parseLegacyPptClientAnchor title bounds failed")
	}
	if titleShape.Bounds != titleBounds {
		t.Fatalf("title bounds = %+v, want %+v", titleShape.Bounds, titleBounds)
	}
	textboxBounds, ok := parseLegacyPptClientAnchor([]byte{23, 6, 150, 5, 112, 10, 4, 7})
	if !ok {
		t.Fatal("parseLegacyPptClientAnchor textbox bounds failed")
	}
	var textboxShape *presentationir.Shape
	for _, shape := range slide.Shapes {
		if shape != nil && shape.Kind == presentationir.ShapeText && len(shape.Paragraphs) > 0 && paragraphText(shape.Paragraphs[0]) == "Text in a TextBox" {
			textboxShape = shape
			break
		}
	}
	if textboxShape == nil {
		t.Fatal("authored textbox not found in effective scene")
	}
	if textboxShape.Bounds != textboxBounds {
		t.Fatalf("textbox bounds = %+v, want %+v", textboxShape.Bounds, textboxBounds)
	}
}

func TestParsePptExtractsHyperlinksFromRealFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "WithLinks.ppt")
	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	found := 0
	var urls []string
	var walkShapes func([]*presentationir.Shape)
	walkShapes = func(shapes []*presentationir.Shape) {
		for _, shape := range shapes {
			if shape == nil {
				continue
			}
			if shape.HyperlinkURL != "" {
				found++
				urls = append(urls, shape.HyperlinkURL)
			}
			if shape.ActionLink != "" {
				found++
				urls = append(urls, shape.ActionLink)
			}
			for _, para := range shape.Paragraphs {
				for _, run := range para.Runs {
					if run.HyperlinkURL != "" {
						found++
						urls = append(urls, run.HyperlinkURL)
					}
				}
			}
			if len(shape.Children) > 0 {
				walkShapes(shape.Children)
			}
		}
	}
	for _, slide := range pres.Slides {
		walkShapes(slide.Shapes)
	}
	if found == 0 {
		t.Fatal("expected at least one hyperlink or action link in WithLinks.ppt")
	}
}

func TestParsePptSynthesizesLegacyEmbeddedAudioFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "sound.ppt")
	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if len(pres.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(pres.Slides))
	}
	var mediaShapes []*presentationir.Shape
	for _, candidate := range pres.Slides[0].Shapes {
		if candidate != nil && candidate.Kind == presentationir.ShapeMedia {
			mediaShapes = append(mediaShapes, candidate)
		}
	}
	if len(mediaShapes) != 1 {
		t.Fatalf("slide media shapes = %d (effective scene shapes=%d), want 1 authored media shape", len(mediaShapes), len(pres.Slides[0].Shapes))
	}
	shape := mediaShapes[0]
	if shape.MediaKind != "audio" {
		t.Fatalf("MediaKind = %q, want audio", shape.MediaKind)
	}
	if len(shape.MediaData) == 0 || shape.MediaMIME == "" {
		t.Fatalf("media payload = %d bytes mime=%q, want embedded audio", len(shape.MediaData), shape.MediaMIME)
	}
	if len(shape.MediaPosterData) == 0 || shape.MediaPosterMIME == "" {
		t.Fatalf("poster payload = %d bytes mime=%q, want icon poster", len(shape.MediaPosterData), shape.MediaPosterMIME)
	}
}

func TestParsePptSynthesizesLegacyOLEFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "embedded_excel_ole_chart.ppt")
	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	found := 0
	for _, shape := range pres.Slides[0].Shapes {
		if shape == nil || shape.Kind != presentationir.ShapeOleObject {
			continue
		}
		found++
		if len(shape.OLEObjectData) == 0 {
			t.Fatal("expected embedded OLE storage bytes")
		}
		if shape.OLEObjectProgID == "" {
			t.Fatal("expected OLE ProgID")
		}
		if len(shape.OLEIconData) == 0 || shape.OLEIconMIME == "" {
			t.Fatal("expected OLE icon payload")
		}
	}
	if found == 0 {
		t.Fatal("expected at least one OLE placeholder shape")
	}
}

func TestParseLegacyPptTableShapesFromSlideBuildsGroupedTable(t *testing.T) {
	buildRect32 := func(left, top, right, bottom int32) []byte {
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint32(buf[0:4], uint32(left))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(top))
		binary.LittleEndian.PutUint32(buf[8:12], uint32(right))
		binary.LittleEndian.PutUint32(buf[12:16], uint32(bottom))
		return buf
	}
	buildClientAnchor := func(x1, y1, x2, y2 uint16) []byte {
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint16(buf[0:2], x1)
		binary.LittleEndian.PutUint16(buf[2:4], y1)
		binary.LittleEndian.PutUint16(buf[4:6], x2)
		binary.LittleEndian.PutUint16(buf[6:8], y2)
		return buf
	}
	buildCell := func(shapeID uint32, left, top, right, bottom int32, text string) []byte {
		textbox := append(
			buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeOther)),
			buildPptAtomRecord(0, pptRTTextBytesAtom, []byte(text))...,
		)
		return buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F,
			append(
				append(
					buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(shapeID, legacyOfficeArtSpFlagChild)),
					buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, buildRect32(left, top, right, bottom))...,
				),
				buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000, textbox)...,
			),
		)
	}

	fopt := make([]byte, 6)
	binary.LittleEndian.PutUint16(fopt[0:2], legacyOfficeArtPropTableProperties)
	binary.LittleEndian.PutUint32(fopt[2:6], 1)

	groupRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F,
		append(
			append(
				append(
					buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(1, 0x01)),
					buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, buildRect32(0, 0, 1000, 800))...,
				),
				buildOfficeArtRecord(1, legacyOfficeArtFOPT, 0x0000, fopt)...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, buildClientAnchor(0, 0, 2000, 1600))...,
		),
	)
	group := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, append(
		append(
			append(
				append(groupRoot, buildCell(2, 0, 0, 500, 400, "A1")...),
				buildCell(3, 500, 0, 1000, 400, "A2")...,
			),
			buildCell(4, 0, 400, 500, 800, "B1")...,
		),
		buildCell(5, 500, 400, 1000, 800, "B2")...,
	))
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x000F, group)

	scene := collectLegacyPptVisualScene(payload)
	var tables []*presentationir.Shape
	collectLegacyPptTableShapesFromSceneForSlideWithOwnership(scene.dispatch.containers, legacyPptGroupFrame{}, legacySlideText{}, nil, nil, nil, &tables, nil)
	if len(tables) != 1 {
		t.Fatalf("len(tables) = %d, want 1", len(tables))
	}
	tableShape := tables[0]
	if tableShape.Kind != presentationir.ShapeTable {
		t.Fatalf("shape kind = %v, want table", tableShape.Kind)
	}
	if tableShape.Table == nil || len(tableShape.Table.Rows) != 2 {
		t.Fatalf("table rows = %#v, want 2 rows", tableShape.Table)
	}
	if got := len(tableShape.Table.Rows[0].Cells); got != 2 {
		t.Fatalf("row 0 cells = %d, want 2", got)
	}
	if got := paragraphText(tableShape.Table.Rows[0].Cells[0].Paragraphs[0]); got != "A1" {
		t.Fatalf("cell (0,0) = %q, want A1", got)
	}
	if got := paragraphText(tableShape.Table.Rows[1].Cells[1].Paragraphs[0]); got != "B2" {
		t.Fatalf("cell (1,1) = %q, want B2", got)
	}

	// Raw text extraction remains independent from table projection. The table
	// collector owns the grouped table semantic shape, while textMetas still
	// exposes the four authored cell text containers for other ownership passes.
	textShapes := buildLegacyPptTextShapesFromMetas(collectLegacyPptVisualScene(payload).textMetas(nil, nil, nil), legacySlideText{})
	if len(textShapes) != 4 {
		t.Fatalf("standalone raw text shapes = %d, want 4 authored cell text containers", len(textShapes))
	}
}

func TestParseLegacyPptFontTableExtractsFixtureFonts(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-Single_Coloured_Page_With_Fonts_and_Alignments.ppt")
	f, err := cfb.Open(path)
	if err != nil {
		t.Fatalf("cfb.Open: %v", err)
	}
	stream, ok := f.Stream("PowerPoint Document")
	if !ok {
		t.Fatal("missing PowerPoint Document stream")
	}
	fontTable := parseLegacyPptFontTable(stream)
	if fontTable[0] != "Arial" {
		t.Fatalf("fontTable[0] = %q, want %q", fontTable[0], "Arial")
	}
	if fontTable[1] != "Times New Roman" {
		t.Fatalf("fontTable[1] = %q, want %q", fontTable[1], "Times New Roman")
	}
	if len(fontTable) != 2 {
		t.Fatalf("len(fontTable) = %d, want 2", len(fontTable))
	}
}

func TestParseLegacyPptFontTableFallsBackToEncounterOrder(t *testing.T) {
	stream := append(
		buildPptAtomRecord(0, pptRTFontEntityAtom, encodeLegacyPptFontEntityPayload("Calibri")),
		buildPptAtomRecord(0, pptRTFontEntityAtom, encodeLegacyPptFontEntityPayload("Arial"))...,
	)
	fontTable := parseLegacyPptFontTable(stream)
	if fontTable[0] != "Calibri" {
		t.Fatalf("fontTable[0] = %q, want %q", fontTable[0], "Calibri")
	}
	if fontTable[1] != "Arial" {
		t.Fatalf("fontTable[1] = %q, want %q", fontTable[1], "Arial")
	}
}

func TestParseLegacyPptColorSchemeExtractsFixturePalette(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-SampleShow.ppt")
	f, err := cfb.Open(path)
	if err != nil {
		t.Fatalf("cfb.Open: %v", err)
	}
	stream, ok := f.Stream("PowerPoint Document")
	if !ok {
		t.Fatal("missing PowerPoint Document stream")
	}
	palette := parseLegacyPptColorScheme(stream)
	if palette[0] != "FFFFFF" {
		t.Fatalf("palette[0] = %q, want %q", palette[0], "FFFFFF")
	}
	if palette[1] != "000000" {
		t.Fatalf("palette[1] = %q, want %q", palette[1], "000000")
	}
	if palette[5] != "C0504D" {
		t.Fatalf("palette[5] = %q, want %q", palette[5], "C0504D")
	}
	if palette[7] != "800080" {
		t.Fatalf("palette[7] = %q, want %q", palette[7], "800080")
	}
}

func TestParseLegacyPptColorSchemePrefersMostFrequentPalette(t *testing.T) {
	primary := []byte{
		0xFF, 0xFF, 0xFF, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0xEE, 0xEC, 0xE1, 0x00,
		0x1F, 0x49, 0x7D, 0x00,
		0x4F, 0x81, 0xBD, 0x00,
		0xC0, 0x50, 0x4D, 0x00,
		0x00, 0x00, 0xFF, 0x00,
		0x80, 0x00, 0x80, 0x00,
	}
	secondary := []byte{
		0x11, 0x11, 0x11, 0x00,
		0x22, 0x22, 0x22, 0x00,
		0x33, 0x33, 0x33, 0x00,
		0x44, 0x44, 0x44, 0x00,
		0x55, 0x55, 0x55, 0x00,
		0x66, 0x66, 0x66, 0x00,
		0x77, 0x77, 0x77, 0x00,
		0x88, 0x88, 0x88, 0x00,
	}
	stream := append(buildPptAtomRecord(0, pptRTColorSchemeAtom, primary), buildPptAtomRecord(0, pptRTColorSchemeAtom, secondary)...)
	stream = append(stream, buildPptAtomRecord(0, pptRTColorSchemeAtom, primary)...)
	palette := parseLegacyPptColorScheme(stream)
	if palette[5] != "C0504D" {
		t.Fatalf("palette[5] = %q, want %q", palette[5], "C0504D")
	}
	if palette[7] != "800080" {
		t.Fatalf("palette[7] = %q, want %q", palette[7], "800080")
	}
}

func TestParsePptPowerPoint40UsesStructuredDocumentRecovery(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "53446.ppt")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skip("53446.ppt corpus fixture is not installed")
		}
		t.Fatal(err)
	}

	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if pres == nil || len(pres.Slides) == 0 {
		t.Fatal("expected structurally recovered slides")
	}
	if !legacyPresentationContainsText(pres, "Risk Assessment & Control Group") {
		t.Fatal("presentation text missing Risk Assessment & Control Group")
	}
	if !legacyPresentationContainsText(pres, "Enron Corp.") {
		t.Fatal("presentation text missing Enron Corp.")
	}
}

func TestParsePpt95UsesTextContentFallback(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "PPT95.ppt")
	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if len(pres.Slides) == 0 {
		t.Fatal("expected slides from PPT95 Text_Content fallback")
	}
	if !legacyPresentationContainsText(pres, "XML, HTML and All That") {
		t.Fatal("presentation text missing PPT95 title from Text_Content stream")
	}
	if !legacyPresentationContainsText(pres, "The birth of HTML") {
		t.Fatal("presentation text missing PPT95 body content from Text_Content stream")
	}
}

func TestParsePptPP40UsesInlineTextRecords(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "pp40only.ppt")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skip("pp40only.ppt corpus fixture is not installed")
		}
		t.Fatal(err)
	}

	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if len(pres.Slides) == 0 {
		t.Fatal("expected slides from PP40 inline text records")
	}
	if !legacyPresentationContainsText(pres, "COMMUNITY ORGANISATIONS") {
		t.Fatal("presentation text missing PP40 title record")
	}
	if !legacyPresentationContainsText(pres, "Question1") {
		t.Fatal("presentation text missing PP40 question record")
	}
	if !legacyPresentationContainsText(pres, "Answer 1") {
		t.Fatal("presentation text missing PP40 answer record")
	}
}

func TestParsePptRepresentativeLegacyCorpusFixtures(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		wantTexts []string
	}{
		{name: "ppt95", file: "PPT95.ppt", wantTexts: []string{"XML, HTML and All That"}},
		{name: "ppt97_basic", file: "basic_test_ppt_file.ppt"},
		{name: "ppt2000_sample_show", file: "SampleShow.ppt"},
		{name: "ppt2003_headers_footers", file: "bug58144-headers-footers-2003.ppt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", tt.file)
			pres, err := ParsePpt(path)
			if err != nil {
				t.Fatalf("ParsePpt(%s): %v", tt.file, err)
			}
			if pres == nil {
				t.Fatalf("ParsePpt(%s) returned nil presentation", tt.file)
			}
			if len(pres.Slides) == 0 {
				t.Fatalf("ParsePpt(%s) produced 0 slides", tt.file)
			}
			for _, want := range tt.wantTexts {
				if !legacyPresentationContainsText(pres, want) {
					t.Fatalf("%s missing text %q", tt.file, want)
				}
			}
		})
	}
}

func TestParseLegacyPptTableShapesFromSlideUsesInlineFallbackForOutlineRefCells(t *testing.T) {
	buildRect32 := func(left, top, right, bottom int32) []byte {
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint32(buf[0:4], uint32(left))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(top))
		binary.LittleEndian.PutUint32(buf[8:12], uint32(right))
		binary.LittleEndian.PutUint32(buf[12:16], uint32(bottom))
		return buf
	}
	buildClientAnchor := func(x1, y1, x2, y2 uint16) []byte {
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint16(buf[0:2], x1)
		binary.LittleEndian.PutUint16(buf[2:4], y1)
		binary.LittleEndian.PutUint16(buf[4:6], x2)
		binary.LittleEndian.PutUint16(buf[6:8], y2)
		return buf
	}
	cellTextbox := func(text string, outlineRef uint32) []byte {
		return append(
			append(
				buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeBody)),
				buildPptAtomRecord(0, pptRTOutlineTextRefAtom, marshalUint32LE(outlineRef))...,
			),
			buildPptAtomRecord(0, pptRTTextBytesAtom, []byte(text))...,
		)
	}
	cell := func(shapeID uint32, x, y int32, text string, outlineRef uint32) []byte {
		return buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(shapeID, 0x02)),
				buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, buildRect32(x, y, x+900, y+500))...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000, cellTextbox(text, outlineRef))...,
		))
	}
	line := func(shapeID uint32, x1, y1, x2, y2 int32) []byte {
		return buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
			append(
				buildOfficeArtRecord(20, legacyOfficeArtSp, 0x0002, officeArtSpPayload(shapeID, 0x02)),
				buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, buildRect32(x1, y1, x2, y2))...,
			),
			buildOfficeArtRecord(7, legacyOfficeArtFOPT, 0x0003, make([]byte, 42))...,
		))
	}

	groupChildren := append(
		append(
			append(cell(11, 0, 0, "A1", 10), cell(12, 1000, 0, "B1", 11)...),
			cell(13, 0, 600, "A2", 12)...,
		),
		append(
			append(cell(14, 1000, 600, "B2", 13), line(21, 950, 0, 950, 1100)...),
			line(22, 0, 550, 1900, 550)...,
		)...,
	)
	groupRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(1, 0x01)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, buildRect32(0, 0, 1900, 1100))...,
		),
		buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, buildClientAnchor(0, 0, 3800, 2200))...,
	))
	group := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, append(groupRoot, groupChildren...))
	ppDrawing := buildPptContainerRecord(0, pptRTPPDrawing, 0x000F, group)

	scene := collectLegacyPptVisualScene(ppDrawing)
	var shapes []*presentationir.Shape
	collectLegacyPptTableShapesFromSceneForSlideWithOwnership(scene.dispatch.containers, legacyPptGroupFrame{}, legacySlideText{}, nil, nil, nil, &shapes, nil)
	// OutlineTextRef-backed cells are not sufficient evidence to synthesize an
	// implicit table in the current parser. Keep them in the ordinary ownership
	// path rather than fabricating table semantics from inline fallback text.
	if len(shapes) != 0 {
		t.Fatalf("len(shapes) = %d, want 0 implicit tables for OutlineTextRef-only cells", len(shapes))
	}
}

func TestParsePptExtractsLegacyTableFixtures(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		wantTexts   []string
		requireText bool
	}{
		{name: "table_test", file: "table_test.ppt"},
		{name: "apache_table_test", file: "apache-table_test.ppt"},
		{name: "bug57820", file: "bug57820-initTableNullRefrenceException.ppt", wantTexts: []string{"test"}, requireText: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", tt.file)
			pres, err := ParsePpt(path)
			if err != nil {
				t.Fatalf("ParsePpt: %v", err)
			}
			if len(pres.Slides) == 0 {
				t.Fatal("expected slides")
			}
			tableShape := legacyFirstTableShape(pres)
			if tableShape == nil || tableShape.Table == nil {
				t.Fatal("expected at least one table shape")
			}
			if len(tableShape.Table.Rows) == 0 {
				t.Fatal("expected table rows")
			}
			if tt.requireText && !legacyTableHasAnyText(tableShape.Table) {
				t.Fatal("expected table text content")
			}
			for _, want := range tt.wantTexts {
				if !legacyTableContainsText(tableShape.Table, want) {
					t.Fatalf("table missing %q", want)
				}
			}
		})
	}
}

func TestParseLegacyPptTextShapesUseOutlineRefToPickSpecificFragment(t *testing.T) {
	slideText := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "First body", TextType: pptTextTypeBody, OutlineRef: 0},
			{Text: "Second body", TextType: pptTextTypeBody, OutlineRef: 1},
		},
	}
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x000F,
		buildOfficeArtRecord(legacyOfficeArtSpContainer>>4, legacyOfficeArtSpContainer, 0x000F,
			append(
				append(
					buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x401, 0x0000000A)),
					buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, []byte{0, 0, 0, 0, 10, 0, 10, 0})...,
				),
				buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000, buildPptAtomRecord(0, pptRTOutlineTextRefAtom, marshalUint32LE(1)))...,
			),
		),
	)
	shapes := buildLegacyPptTextShapesFromMetas(collectLegacyPptVisualScene(payload).textMetas(nil, nil, nil), slideText)
	if len(shapes) != 2 {
		t.Fatalf("len(shapes) = %d, want 2", len(shapes))
	}
	if got := paragraphText(shapes[0].Paragraphs[0]); got != "Second body" {
		t.Fatalf("shape text = %q, want %q", got, "Second body")
	}
	if got := paragraphText(shapes[1].Paragraphs[0]); got != "First body" {
		t.Fatalf("leftover shape text = %q, want %q", got, "First body")
	}
	if len(slideText.Fragments) != 2 {
		t.Fatalf("slide fragments len mutated = %d, want 2", len(slideText.Fragments))
	}
}

func TestParseLegacyPptTextBoxFragmentsExposeOutlineRefWithoutInlineText(t *testing.T) {
	frags, outlineRef := parseLegacyPptTextBoxFragments(buildPptAtomRecord(0, pptRTOutlineTextRefAtom, marshalUint32LE(7)), nil, nil, nil)
	if len(frags) != 0 {
		t.Fatalf("len(frags) = %d, want 0", len(frags))
	}
	if outlineRef != 7 {
		t.Fatalf("outlineRef = %d, want 7", outlineRef)
	}
}

func TestParseLegacyPptTextShapesPreferShapeIDMatchOverFallback(t *testing.T) {
	slideText := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "First body", TextType: pptTextTypeBody, ShapeID: 0x401},
			{Text: "Second body", TextType: pptTextTypeBody, ShapeID: 0x402},
		},
	}
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x000F,
		buildOfficeArtRecord(legacyOfficeArtSpContainer>>4, legacyOfficeArtSpContainer, 0x000F,
			append(
				append(
					buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x402, 0x0000000A)),
					buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, []byte{0, 0, 0, 0, 10, 0, 10, 0})...,
				),
				buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000, nil)...,
			),
		),
	)
	shapes := buildLegacyPptTextShapesFromMetas(collectLegacyPptVisualScene(payload).textMetas(nil, nil, nil), slideText)
	if len(shapes) != 2 {
		t.Fatalf("len(shapes) = %d, want 2", len(shapes))
	}
	if got := paragraphText(shapes[0].Paragraphs[0]); got != "Second body" {
		t.Fatalf("shape text = %q, want %q", got, "Second body")
	}
	if got := paragraphText(shapes[1].Paragraphs[0]); got != "First body" {
		t.Fatalf("leftover shape text = %q, want %q", got, "First body")
	}
}

func TestParseLegacyPptTextShapesPreferOutlineRefWhenShapeIDMapsToPrompt(t *testing.T) {
	slideText := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "Click to edit Master title style", TextType: pptTextTypeBody, ShapeID: 0x401},
			{Text: "Actual body text", TextType: pptTextTypeBody, OutlineRef: 7},
		},
	}
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x000F,
		buildOfficeArtRecord(legacyOfficeArtSpContainer>>4, legacyOfficeArtSpContainer, 0x000F,
			append(
				append(
					buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x401, 0x0000000A)),
					buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, []byte{0, 0, 0, 0, 10, 0, 10, 0})...,
				),
				buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000, buildPptAtomRecord(0, pptRTOutlineTextRefAtom, marshalUint32LE(7)))...,
			),
		),
	)
	shapes := buildLegacyPptTextShapesFromMetas(collectLegacyPptVisualScene(payload).textMetas(nil, nil, nil), slideText)
	if len(shapes) != 1 {
		t.Fatalf("len(shapes) = %d, want 1", len(shapes))
	}
	if got := paragraphText(shapes[0].Paragraphs[0]); got != "Actual body text" {
		t.Fatalf("shape text = %q, want %q", got, "Actual body text")
	}
}

func TestParsePptExposesLegacyMasterThemePalette(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-WithMaster.ppt")
	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if pres.Theme == nil {
		t.Fatal("Theme = nil, want legacy master theme")
	}
	if got := pres.Theme.ColorScheme["dk1"]; got != "000000" {
		t.Fatalf("Theme.ColorScheme[dk1] = %q, want %q", got, "000000")
	}
	if got := pres.Theme.ColorScheme["accent2"]; got != "C0504D" {
		t.Fatalf("Theme.ColorScheme[accent2] = %q, want %q", got, "C0504D")
	}
	if len(pres.Slides) == 0 {
		t.Fatal("slides = 0, want at least one slide")
	}
	if got := pres.Slides[0].BgColor; got != "FFFFFF" {
		t.Fatalf("Slides[0].BgColor = %q, want %q", got, "FFFFFF")
	}
}

func TestParsePptAppliesLegacyMasterTextDefaults(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-WithMaster.ppt")
	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if len(pres.Slides) != 2 {
		t.Fatalf("slides = %d, want 2", len(pres.Slides))
	}
	assertStyled := func(text string) {
		t.Helper()
		for _, slide := range pres.Slides {
			for _, shape := range slide.Shapes {
				if shape == nil || len(shape.Paragraphs) == 0 || len(shape.Paragraphs[0].Runs) == 0 {
					continue
				}
				if paragraphText(shape.Paragraphs[0]) != text {
					continue
				}
				run := shape.Paragraphs[0].Runs[0]
				if run.FontSize == 0 && run.FontFamily == "" && run.Color == "" && !run.Bold && !run.Italic {
					t.Fatalf("run for %q remained unstyled: %+v", text, run)
				}
				return
			}
		}
		t.Fatalf("did not find shape text %q", text)
	}
	assertStyled("First page title")
	assertStyled("First page subtitle")
	assertStyled("2nd page subtitle")
	assertStyled("Footer from the master slide")
}

func TestParsePptKeepsLegacyMasterTitleTextWithoutInventingPlaceholder(t *testing.T) {
	pres, err := ParsePpt(filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-WithMaster.ppt"))
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	for _, slide := range pres.Slides {
		for _, shape := range slide.Shapes {
			if shape == nil || len(shape.Paragraphs) == 0 {
				continue
			}
			if paragraphText(shape.Paragraphs[0]) != "First page title" {
				continue
			}
			if shape.Placeholder != presentationir.PlaceholderNone {
				t.Fatalf("placeholder for master-derived title text = %v, want none without explicit placeholder evidence", shape.Placeholder)
			}
			return
		}
	}
	t.Fatal("did not find shape text \"First page title\"")
}

func TestParsePptPromotesLegacyHeadersAndFootersFixture(t *testing.T) {
	pres, err := ParsePpt(filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-headers_footers.ppt"))
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if len(pres.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(pres.Slides))
	}
	slide := pres.Slides[0]
	assertNoPrompt := func(text string) {
		t.Helper()
		if legacyPptLooksLikePlaceholderSampleText(text) {
			t.Fatalf("placeholder prompt leaked into parsed slide: %q", text)
		}
	}
	if slide.Footer == nil {
		t.Fatal("Footer = nil, want promoted footer placeholder")
	}
	if got := paragraphText(slide.Footer.Paragraphs[0]); got != "per-slide footer" {
		t.Fatalf("footer text = %q, want %q", got, "per-slide footer")
	}
	if slide.DateTime == nil {
		t.Fatal("DateTime = nil, want promoted date placeholder")
	}
	if got := paragraphText(slide.DateTime.Paragraphs[0]); got != "custom date format" {
		t.Fatalf("date text = %q, want %q", got, "custom date format")
	}
	if slide.SlideNumber == nil {
		t.Fatal("SlideNumber = nil, want synthesized slide-number placeholder")
	}
	if len(slide.SlideNumber.Paragraphs) != 1 || len(slide.SlideNumber.Paragraphs[0].Runs) != 1 {
		t.Fatalf("slide number paragraphs = %+v, want one synthesized field run", slide.SlideNumber.Paragraphs)
	}
	if got := slide.SlideNumber.Paragraphs[0].Runs[0].FieldKind; got != "slidenum" {
		t.Fatalf("slide number field kind = %q, want %q", got, "slidenum")
	}
	for _, shape := range slide.Shapes {
		if shape == nil {
			continue
		}
		if len(shape.Paragraphs) > 0 {
			assertNoPrompt(paragraphText(shape.Paragraphs[0]))
		}
	}
	if slide.Footer != nil && len(slide.Footer.Paragraphs) > 0 {
		assertNoPrompt(paragraphText(slide.Footer.Paragraphs[0]))
	}
	if slide.DateTime != nil && len(slide.DateTime.Paragraphs) > 0 {
		assertNoPrompt(paragraphText(slide.DateTime.Paragraphs[0]))
	}
	if slide.SlideNumber != nil && len(slide.SlideNumber.Paragraphs) > 0 {
		assertNoPrompt(paragraphText(slide.SlideNumber.Paragraphs[0]))
	}
}

func TestParsePptPromotesLegacyNotesHeadersAndFootersFixture(t *testing.T) {
	pres, err := ParsePpt(filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-headers_footers.ppt"))
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if len(pres.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(pres.Slides))
	}
	if pres.Slides[0].Notes == nil {
		t.Fatal("Notes = nil, want promoted notes header/footer shapes")
	}
	var sawHeader, sawFooter, sawSlideNumber bool
	for _, shape := range pres.Slides[0].Notes.Shapes {
		if shape == nil {
			continue
		}
		if shape.Placeholder == presentationir.PlaceholderSlideNumber {
			if len(shape.Paragraphs) != 1 || len(shape.Paragraphs[0].Runs) != 1 || shape.Paragraphs[0].Runs[0].FieldKind != "slidenum" {
				t.Fatalf("notes slide-number shape = %#v, want one slidenum field run", shape)
			}
			sawSlideNumber = true
		}
		if len(shape.Paragraphs) == 0 {
			continue
		}
		switch paragraphText(shape.Paragraphs[0]) {
		case "Notes Header":
			sawHeader = true
		case "Notes Footer":
			sawFooter = true
		}
	}
	if !sawHeader || !sawFooter || !sawSlideNumber {
		t.Fatalf("notes shapes = %#v, want Notes Header, Notes Footer, and note-side slide number", pres.Slides[0].Notes.Shapes)
	}
}

func TestParseLegacyPptTextMasterStyleParsesBodyLevels(t *testing.T) {
	payload := append(marshalLegacyUint16LE(2), buildLegacyPptMasterTextLevelPayload(stpaPFEntry{Align: "ctr"}, stpaCFRun{FontSize: 2400, Color: "112233"})...)
	payload = append(payload, buildLegacyPptMasterTextLevelPayload(stpaPFEntry{BulletChar: '•', HasMarginLeft: true, MarginLeftEMU: 127000}, stpaCFRun{FontFamily: "Arial"})...)
	textType, levels, ok := parseLegacyPptTextMasterStyle(pptRecord{Instance: pptTextTypeBody, Type: pptRTTextMasterStyle}, payload, map[uint16]string{0: "Arial"}, nil)
	if !ok {
		t.Fatal("parseLegacyPptTextMasterStyle returned ok=false")
	}
	if textType != pptTextTypeBody {
		t.Fatalf("textType = %d, want %d", textType, pptTextTypeBody)
	}
	if got := levels[0].paragraph.Align; got != "ctr" {
		t.Fatalf("levels[0].paragraph.Align = %q, want %q", got, "ctr")
	}
	if got := levels[0].run.FontSize; got != 2400 {
		t.Fatalf("levels[0].run.FontSize = %d, want 2400", got)
	}
	if got := levels[1].paragraph.BulletChar; got != '•' {
		t.Fatalf("levels[1].paragraph.BulletChar = %q, want %q", got, '•')
	}
	if !levels[1].paragraph.HasMarginLeft || levels[1].paragraph.MarginLeftEMU != 127000 {
		t.Fatalf("levels[1].paragraph margin = %+v, want HasMarginLeft with 127000 EMU", levels[1].paragraph)
	}
	if got := levels[1].run.FontFamily; got != "Arial" {
		t.Fatalf("levels[1].run.FontFamily = %q, want %q", got, "Arial")
	}
}

func TestParseLegacyPptTextMasterStyle10ParsesCenterTitleLevels(t *testing.T) {
	payload := append(marshalLegacyUint16LE(1), marshalLegacyUint16LE(0)...)
	payload = append(payload, encodeLegacyPptCFException10(0)...)
	textType, levels, ok := parseLegacyPptTextMasterStyle10(pptRecord{Instance: pptTextTypeCenterTitle, Type: pptRTTextMasterStyle10}, payload, map[uint16]string{0: "Arial"})
	if !ok {
		t.Fatal("parseLegacyPptTextMasterStyle10 returned ok=false")
	}
	if textType != pptTextTypeCenterTitle {
		t.Fatalf("textType = %d, want %d", textType, pptTextTypeCenterTitle)
	}
	if got := levels[0].run.FontFamily; got != "Arial" {
		t.Fatalf("levels[0].run.FontFamily = %q, want %q", got, "Arial")
	}
}

func TestParseLegacyPptMasterTextStylesUsesDocumentFallbackAndProgTags(t *testing.T) {
	docBase := buildPptAtomRecord(pptTextTypeBody, pptRTTextMasterStyle, append(
		marshalLegacyUint16LE(1),
		buildLegacyPptMasterTextLevelPayload(stpaPFEntry{Align: "ctr"}, stpaCFRun{Color: "112233"})...,
	))
	mainBase := buildPptAtomRecord(pptTextTypeBody, pptRTTextMasterStyle, append(
		marshalLegacyUint16LE(1),
		buildLegacyPptMasterTextLevelPayload(stpaPFEntry{}, stpaCFRun{FontSize: 2400})...,
	))
	docDefaults9Payload := append(encodeLegacyPptCFException9(), encodeLegacyPptPFException9AutoNum(0x0003)...)
	defaults9Level, ok := parseLegacyPptTextDefaults9(docDefaults9Payload)
	if !ok || !defaults9Level.paragraph.AutoNum || defaults9Level.paragraph.AutoNumType != "arabicPeriod" {
		t.Fatalf("parseLegacyPptTextDefaults9 = %+v, ok=%v, want autonum arabicPeriod", defaults9Level, ok)
	}
	docDefaults9 := buildPptAtomRecord(0, pptRTTextDefaults9, docDefaults9Payload)
	pp9Tag := buildPptContainerRecord(0, pptRTProgBinaryTag, 0x0F, append(
		buildPptAtomRecord(0, pptRTCString, encodeLegacyUTF16("___PPT9")),
		buildPptAtomRecord(0, pptRTBinaryTagDataBlob, docDefaults9)...,
	))
	pp9ProgTags := buildPptContainerRecord(0, pptRTProgTags, 0x0F, pp9Tag)
	fontEntity := buildPptAtomRecord(0, pptRTFontEntityAtom, buildLegacyFontEntityPayload("Malgun Gothic"))
	fontCollection10 := buildPptContainerRecord(0, pptRTFontCollection10, 0x0F, fontEntity)
	textMaster10 := buildPptAtomRecord(pptTextTypeBody, pptRTTextMasterStyle10, append(marshalLegacyUint16LE(1), encodeLegacyPptCFException10(0)...))
	pp10Tag := buildPptContainerRecord(0, pptRTProgBinaryTag, 0x0F, append(
		buildPptAtomRecord(0, pptRTCString, encodeLegacyUTF16("___PPT10")),
		buildPptAtomRecord(0, pptRTBinaryTagDataBlob, append(fontCollection10, textMaster10...))...,
	))
	pp10ProgTags := buildPptContainerRecord(0, pptRTProgTags, 0x0F, pp10Tag)
	documentPayload := append(buildPptContainerRecord(0, pptRTDocumentTextInfo, 0x0F, docBase), buildPptContainerRecord(0, 0x1000, 0x0F, pp9ProgTags)...)
	documentPayload = append(documentPayload, buildPptContainerRecord(0, pptRTMainMaster, 0x0F, append(mainBase, pp10ProgTags...))...)
	document := buildPptContainerRecord(0, pptRTDocument, 0x0F, documentPayload)
	styles := parseLegacyPptMasterTextStyles(document, 0, map[uint16]string{0: "Arial"}, nil)
	levels := styles[pptTextTypeBody]
	if len(levels) == 0 {
		t.Fatal("body master levels missing")
	}
	level := levels[0]
	if got := level.paragraph.Align; got != "ctr" {
		t.Fatalf("level.paragraph.Align = %q, want ctr", got)
	}
	if !level.paragraph.AutoNum {
		t.Fatal("level.paragraph.AutoNum = false, want true")
	}
	if got := level.paragraph.AutoNumType; got != "arabicPeriod" {
		t.Fatalf("level.paragraph.AutoNumType = %q, want arabicPeriod", got)
	}
	if got := level.run.FontSize; got != 2400 {
		t.Fatalf("level.run.FontSize = %d, want 2400", got)
	}
	if got := level.run.Color; got != "112233" {
		t.Fatalf("level.run.Color = %q, want 112233", got)
	}
	if got := level.run.FontFamily; got != "Malgun Gothic" {
		t.Fatalf("level.run.FontFamily = %q, want Malgun Gothic", got)
	}
}

func TestComposeLegacyPptMasterTextStylesFallsBackToLevel0ForPP10Ext(t *testing.T) {
	docBundle := legacyPptMasterTextBundle{
		ext10: legacyPptMasterTextStyles{
			pptTextTypeBody: {
				0: {run: stpaCFRun{FontFamily: "Malgun Gothic"}},
			},
		},
	}
	mainBundle := legacyPptMasterTextBundle{
		base: legacyPptMasterTextStyles{
			pptTextTypeBody: {
				3: {run: stpaCFRun{Color: "112233"}},
			},
		},
	}
	styles := composeLegacyPptMasterTextStyles(docBundle, mainBundle)
	if len(styles[pptTextTypeBody]) == 0 {
		t.Fatalf("body styles missing: %+v", styles)
	}
	level := styles[pptTextTypeBody][3]
	if got := level.run.FontFamily; got != "Malgun Gothic" {
		t.Fatalf("level.run.FontFamily = %q, want %q", got, "Malgun Gothic")
	}
	if got := level.run.Color; got != "112233" {
		t.Fatalf("level.run.Color = %q, want %q", got, "112233")
	}
}

func TestComposeLegacyPptMasterTextStylesAppliesMainBundleDefaults9(t *testing.T) {
	// mainBundle.defaults9 must act as a fallback at lower priority than mainBundle.base,
	// but higher priority than docBundle.defaults9, per the master > document precedence rule.
	mainBundle := legacyPptMasterTextBundle{
		base: legacyPptMasterTextStyles{
			pptTextTypeBody: {0: {run: stpaCFRun{FontSize: 2400}}},
		},
	}
	mainDefaults9 := legacyPptMasterTextLevel{run: stpaCFRun{FontFamily: "MasterDefault"}}
	mainBundle.defaults9 = &mainDefaults9
	docDefaults9 := legacyPptMasterTextLevel{
		run:       stpaCFRun{FontFamily: "DocDefault", Color: "AABBCC"},
		paragraph: stpaPFEntry{Align: "r"},
	}
	docBundle := legacyPptMasterTextBundle{defaults9: &docDefaults9}

	styles := composeLegacyPptMasterTextStyles(docBundle, mainBundle)
	level := styles[pptTextTypeBody][0]
	// FontFamily: mainBundle.defaults9 ("MasterDefault") must override docBundle.defaults9 ("DocDefault")
	if got := level.run.FontFamily; got != "MasterDefault" {
		t.Fatalf("run.FontFamily = %q, want MasterDefault (mainBundle.defaults9 priority)", got)
	}
	// Color: only in docBundle.defaults9, so it should be inherited
	if got := level.run.Color; got != "AABBCC" {
		t.Fatalf("run.Color = %q, want AABBCC (docBundle.defaults9 fallback)", got)
	}
	// Align: only in docBundle.defaults9
	if got := level.paragraph.Align; got != "r" {
		t.Fatalf("paragraph.Align = %q, want r (docBundle.defaults9 fallback)", got)
	}
	// FontSize: from mainBundle.base, should not be overridden by defaults
	if got := level.run.FontSize; got != 2400 {
		t.Fatalf("run.FontSize = %d, want 2400 (mainBundle.base)", got)
	}
}

func TestComposeLegacyPptMasterTextStylesAppliesMainBundleDefaults10(t *testing.T) {
	// Same as the Defaults9 test but for PP10 defaults.
	mainBundle := legacyPptMasterTextBundle{
		base: legacyPptMasterTextStyles{
			pptTextTypeBody: {0: {run: stpaCFRun{FontSize: 2800}}},
		},
	}
	mainDefaults10 := legacyPptMasterTextLevel{run: stpaCFRun{FontFamily: "Master10Default"}}
	mainBundle.defaults10 = &mainDefaults10
	docDefaults10 := legacyPptMasterTextLevel{run: stpaCFRun{FontFamily: "Doc10Default", Color: "001122"}}
	docBundle := legacyPptMasterTextBundle{defaults10: &docDefaults10}

	styles := composeLegacyPptMasterTextStyles(docBundle, mainBundle)
	level := styles[pptTextTypeBody][0]
	if got := level.run.FontFamily; got != "Master10Default" {
		t.Fatalf("run.FontFamily = %q, want Master10Default (mainBundle.defaults10 priority)", got)
	}
	if got := level.run.Color; got != "001122" {
		t.Fatalf("run.Color = %q, want 001122 (docBundle.defaults10 fallback)", got)
	}
}

func TestParseTextRulerAtomParsesMarginsAndIndents(t *testing.T) {
	mask := uint16(0x0002 | 0x0010 | 0x0200)
	payload := append(marshalLegacyUint16LE(mask), marshalLegacyUint16LE(0)...)
	payload = append(payload, marshalLegacyUint16LE(2)...)
	payload = append(payload, marshalLegacyUint16LE(uint16(int16(100)))...)
	payload = append(payload, marshalLegacyUint16LE(uint16(int16(80)))...)

	ruler, ok := parseTextRulerAtom(payload)
	if !ok {
		t.Fatal("parseTextRulerAtom returned ok=false")
	}
	if !ruler.hasLeft[1] || ruler.leftMargins[1] != 100 {
		t.Fatalf("leftMargins[1] = %d (has=%v), want 100 true", ruler.leftMargins[1], ruler.hasLeft[1])
	}
	if !ruler.hasIndent[1] || ruler.indents[1] != 80 {
		t.Fatalf("indents[1] = %d (has=%v), want 80 true", ruler.indents[1], ruler.hasIndent[1])
	}
}

func TestApplyRulerLevelsToParasAssignsLevelAndOffsets(t *testing.T) {
	paras := []*presentationir.Paragraph{{
		MarginLeftEMU: legacyPptMasterUnitsToEMU(100),
		HasMarginLeft: true,
	}}
	ruler := legacyPptTextRuler{}
	ruler.hasLeft[0] = true
	ruler.leftMargins[0] = 50
	ruler.hasLeft[1] = true
	ruler.leftMargins[1] = 100
	ruler.hasIndent[1] = true
	ruler.indents[1] = 20

	applyRulerLevelsToParas(paras, ruler)

	if got := paras[0].Level; got != 1 {
		t.Fatalf("paragraph level = %d, want 1", got)
	}
	if !paras[0].HasTextIndent || paras[0].TextIndentEMU != legacyPptMasterUnitsToEMU(20) {
		t.Fatalf("paragraph indent = %d (has=%v), want %d true", paras[0].TextIndentEMU, paras[0].HasTextIndent, legacyPptMasterUnitsToEMU(20))
	}
}

func TestParseLegacyPptMasterTextStylesSupplementsMissingTypesFromDirectMaster(t *testing.T) {
	docBase := buildPptAtomRecord(pptTextTypeBody, pptRTTextMasterStyle, append(
		marshalLegacyUint16LE(1),
		buildLegacyPptMasterTextLevelPayload(stpaPFEntry{Align: "ctr"}, stpaCFRun{Color: "112233"})...,
	))
	mainBase := buildPptAtomRecord(pptTextTypeBody, pptRTTextMasterStyle, append(
		marshalLegacyUint16LE(1),
		buildLegacyPptMasterTextLevelPayload(stpaPFEntry{}, stpaCFRun{FontSize: 2400})...,
	))
	rawTitle := "Master Title"
	titleCount := uint32(len([]rune(rawTitle)) + 1)
	titleSTPA := append(marshalUint32LE(titleCount), marshalLegacyUint16LE(0)...)
	titleSTPA = append(titleSTPA, encodeLegacyPptPFExceptionSpec(stpaPFEntry{Align: "ctr"})...)
	titleSTPA = append(titleSTPA, marshalUint32LE(titleCount)...)
	titleSTPA = append(titleSTPA, encodeLegacyPptCFExceptionSpec(stpaCFRun{FontSize: 3200, Bold: boolPtr(true), Color: "445566"})...)
	directTitle := append(buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(uint32(pptTextTypeTitle))), buildPptAtomRecord(0, pptRTTextCharsAtom, encodeLegacyUTF16(rawTitle))...)
	directTitle = append(directTitle, buildPptAtomRecord(0, pptRTStyleTextPropAtom, titleSTPA)...)
	documentPayload := append(buildPptContainerRecord(0, pptRTDocumentTextInfo, 0x0F, docBase), buildPptContainerRecord(0, pptRTMainMaster, 0x0F, append(mainBase, directTitle...))...)
	document := buildPptContainerRecord(0, pptRTDocument, 0x0F, documentPayload)

	styles := parseLegacyPptMasterTextStyles(document, 0, nil, nil)
	bodyLevels := styles[pptTextTypeBody]
	if len(bodyLevels) == 0 {
		t.Fatal("body master levels missing")
	}
	titleLevels := styles[pptTextTypeTitle]
	if len(titleLevels) == 0 {
		t.Fatal("title master levels missing")
	}
	titleLevel := titleLevels[0]
	if got := titleLevel.paragraph.Align; got != "ctr" {
		t.Fatalf("titleLevel.paragraph.Align = %q, want ctr", got)
	}
	if titleLevel.run.Bold == nil || !*titleLevel.run.Bold {
		t.Fatalf("titleLevel.run.Bold = %v, want true", titleLevel.run.Bold)
	}
	if got := titleLevel.run.FontSize; got != 3200 {
		t.Fatalf("titleLevel.run.FontSize = %d, want 3200", got)
	}
	if got := titleLevel.run.Color; got != "445566" {
		t.Fatalf("titleLevel.run.Color = %q, want 445566", got)
	}
}

func TestParseLegacyPptMasterTextStylesAppliesDefaultsToPlainDirectTitle(t *testing.T) {
	docDefaults10 := buildPptAtomRecord(0, pptRTTextDefaults10, encodeLegacyPptCFException10(0))
	fontEntity := buildPptAtomRecord(0, pptRTFontEntityAtom, buildLegacyFontEntityPayload("Malgun Gothic"))
	fontCollection10 := buildPptContainerRecord(0, pptRTFontCollection10, 0x0F, fontEntity)
	pp10Tag := buildPptContainerRecord(0, pptRTProgBinaryTag, 0x0F, append(
		buildPptAtomRecord(0, pptRTCString, encodeLegacyUTF16("___PPT10")),
		buildPptAtomRecord(0, pptRTBinaryTagDataBlob, append(fontCollection10, docDefaults10...))...,
	))
	pp10ProgTags := buildPptContainerRecord(0, pptRTProgTags, 0x0F, pp10Tag)
	directTitle := append(
		buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(uint32(pptTextTypeTitle))),
		buildPptAtomRecord(0, pptRTTextCharsAtom, encodeLegacyUTF16("Plain Master Title"))...,
	)
	documentPayload := append(pp10ProgTags, buildPptContainerRecord(0, pptRTMainMaster, 0x0F, directTitle)...)
	document := buildPptContainerRecord(0, pptRTDocument, 0x0F, documentPayload)

	styles := parseLegacyPptMasterTextStyles(document, 0, nil, nil)
	titleLevels := styles[pptTextTypeTitle]
	if len(titleLevels) == 0 {
		t.Fatal("title master levels missing")
	}
	if got := titleLevels[0].run.FontFamily; got != "Malgun Gothic" {
		t.Fatalf("titleLevels[0].run.FontFamily = %q, want %q", got, "Malgun Gothic")
	}
}

func TestParseLegacyPptPP10TextBundleResolvesFontsRegardlessOfRecordOrder(t *testing.T) {
	textMaster10 := buildPptAtomRecord(pptTextTypeCenterTitle, pptRTTextMasterStyle10, append(append(marshalLegacyUint16LE(1), marshalLegacyUint16LE(0)...), encodeLegacyPptCFException10(0)...))
	fontEntity := buildPptAtomRecord(0, pptRTFontEntityAtom, buildLegacyFontEntityPayload("Malgun Gothic"))
	fontCollection10 := buildPptContainerRecord(0, pptRTFontCollection10, 0x0F, fontEntity)
	bundle := parseLegacyPptPP10TextBundle(append(textMaster10, fontCollection10...))
	if got := bundle.ext10[pptTextTypeCenterTitle][0].run.FontFamily; got != "Malgun Gothic" {
		t.Fatalf("bundle.ext10[centerTitle][0].run.FontFamily = %q, want %q", got, "Malgun Gothic")
	}
}

func TestParseLegacyPptDirectMasterTextStylesFallsBackToPlainParagraphs(t *testing.T) {
	directTitle := append(
		buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(uint32(pptTextTypeTitle))),
		buildPptAtomRecord(0, pptRTTextCharsAtom, encodeLegacyUTF16("Plain Master Title"))...,
	)
	styles := parseLegacyPptDirectMasterTextStyles(directTitle, nil, nil)
	titleLevels := styles[pptTextTypeTitle]
	if len(titleLevels) == 0 {
		t.Fatalf("title master levels missing from direct styles: %+v", styles)
	}
}

func TestParseLegacyPptMasterTextStylesExtractsFixtureTitleLevels(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-WithMaster.ppt")
	f, err := cfb.Open(path)
	if err != nil {
		t.Fatalf("cfb.Open: %v", err)
	}
	stream, ok := f.Stream("PowerPoint Document")
	if !ok {
		t.Fatal("missing PowerPoint Document stream")
	}
	ctx, ok := extractPptPersistContext(f, stream)
	if !ok {
		t.Fatal("extractPptPersistContext failed")
	}
	fontTable := parseLegacyPptFontTable(stream)
	colorScheme := parseLegacyPptColorScheme(stream)
	styles := parseLegacyPptMasterTextStyles(stream, ctx.documentOffset, fontTable, colorScheme)
	if len(styles[pptTextTypeTitle]) == 0 && len(styles[pptTextTypeCenterTitle]) == 0 {
		t.Fatalf("title-related master levels missing: title=%+v centerTitle=%+v body=%+v", styles[pptTextTypeTitle], styles[pptTextTypeCenterTitle], styles[pptTextTypeBody])
	}
}

func TestParseLegacyPptMasterTextStylesSelectsStyledFixtureTitleLevels(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-WithMaster.ppt")
	f, err := cfb.Open(path)
	if err != nil {
		t.Fatalf("cfb.Open: %v", err)
	}
	stream, ok := f.Stream("PowerPoint Document")
	if !ok {
		t.Fatal("missing PowerPoint Document stream")
	}
	ctx, ok := extractPptPersistContext(f, stream)
	if !ok {
		t.Fatal("extractPptPersistContext failed")
	}
	fontTable := parseLegacyPptFontTable(stream)
	colorScheme := parseLegacyPptColorScheme(stream)
	styles := parseLegacyPptMasterTextStyles(stream, ctx.documentOffset, fontTable, colorScheme)
	levels := legacyPptMasterTextLevelsForType(styles, pptTextTypeTitle)
	if !legacyPptMasterTextLevelsHaveRunStyling(levels) {
		t.Fatalf("selected title levels have no run styling: title=%+v centerTitle=%+v body=%+v", styles[pptTextTypeTitle], styles[pptTextTypeCenterTitle], styles[pptTextTypeBody])
	}
}

func TestLegacyPptMasterTextLevelsForTypeSkipsEmptyTitleFallback(t *testing.T) {
	styles := legacyPptMasterTextStyles{
		pptTextTypeTitle:       {0: {}},
		pptTextTypeCenterTitle: {0: {run: stpaCFRun{FontFamily: "Arial"}}},
	}
	levels := legacyPptMasterTextLevelsForType(styles, pptTextTypeTitle)
	if got := levels[0].run.FontFamily; got != "Arial" {
		t.Fatalf("levels[0].run.FontFamily = %q, want %q", got, "Arial")
	}
}

func TestLegacyPptMasterTextLevelsForTypeSkipsParagraphOnlyTitleFallback(t *testing.T) {
	styles := legacyPptMasterTextStyles{
		pptTextTypeTitle:       {0: {paragraph: stpaPFEntry{Align: "ctr"}}},
		pptTextTypeCenterTitle: {0: {run: stpaCFRun{FontFamily: "Arial"}}},
	}
	levels := legacyPptMasterTextLevelsForType(styles, pptTextTypeTitle)
	if got := levels[0].run.FontFamily; got != "Arial" {
		t.Fatalf("levels[0].run.FontFamily = %q, want %q", got, "Arial")
	}
}

func TestBuildPresentationAppliesLegacyMasterTextDefaultsToUnstyledBody(t *testing.T) {
	styles := legacyPptMasterTextStyles{
		pptTextTypeBody: {
			0: {paragraph: stpaPFEntry{Align: "ctr", HasBullet: true, BulletChar: '•'}, run: stpaCFRun{FontSize: 3200, FontFamily: "Arial", Color: "112233", Bold: boolPtr(true)}},
		},
	}
	pres := buildPresentationWithMasterComposition([]legacySlideText{{Fragments: []legacySlideFragment{{Text: "Agenda item", TextType: pptTextTypeBody}}}}, nil, nil, legacyPptHeaderFooterInfo{}, "sample.ppt", nil, nil, styles, "", nil)
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 1 {
		t.Fatalf("presentation shapes = %#v", pres.Slides)
	}
	para := pres.Slides[0].Shapes[0].Paragraphs[0]
	if para.Align != "ctr" {
		t.Fatalf("Align = %q, want ctr", para.Align)
	}
	if para.BulletChar != "•" {
		t.Fatalf("BulletChar = %q, want •", para.BulletChar)
	}
	run := para.Runs[0]
	if !run.Bold {
		t.Fatal("Bold = false, want true")
	}
	if run.FontSize != 3200 {
		t.Fatalf("FontSize = %d, want 3200", run.FontSize)
	}
	if run.FontFamily != "Arial" {
		t.Fatalf("FontFamily = %q, want Arial", run.FontFamily)
	}
	if run.Color != "112233" {
		t.Fatalf("Color = %q, want 112233", run.Color)
	}
}

func TestBuildPresentationAppliesLegacyMasterTextDefaultsOnlyToUnsetFields(t *testing.T) {
	styles := legacyPptMasterTextStyles{
		pptTextTypeBody: {
			0: {paragraph: stpaPFEntry{Align: "ctr", BulletColor: "FF0000", BulletSize: 1800}, run: stpaCFRun{Bold: boolPtr(true), FontSize: 2800, FontFamily: "Arial", Color: "112233"}},
		},
	}
	slide := legacySlideText{Fragments: []legacySlideFragment{{
		Text:     "Agenda item",
		TextType: pptTextTypeBody,
		Paragraphs: []*presentationir.Paragraph{{
			Align:      "r",
			BulletChar: "-",
			Runs:       []*presentationir.Run{{Text: "Agenda item", FontSize: 3600}},
		}},
	}}}
	pres := buildPresentationWithMasterComposition([]legacySlideText{slide}, nil, nil, legacyPptHeaderFooterInfo{}, "sample.ppt", nil, nil, styles, "", nil)
	para := pres.Slides[0].Shapes[0].Paragraphs[0]
	if para.Align != "r" {
		t.Fatalf("Align = %q, want existing right alignment", para.Align)
	}
	if para.BulletChar != "-" {
		t.Fatalf("BulletChar = %q, want existing -", para.BulletChar)
	}
	if para.BulletColor != "112233" {
		t.Fatalf("BulletColor = %q, want effective inherited text color 112233", para.BulletColor)
	}
	run := para.Runs[0]
	if !run.Bold {
		t.Fatal("Bold not inherited onto partially styled paragraph; want master Bold=true to flow in when BoldSet=false")
	}
	if run.FontSize != 3600 {
		t.Fatalf("FontSize = %d, want existing 3600", run.FontSize)
	}
	if run.FontFamily != "Arial" {
		t.Fatalf("FontFamily = %q, want inherited Arial", run.FontFamily)
	}
	if run.Color != "112233" {
		t.Fatalf("Color = %q, want inherited 112233", run.Color)
	}
}

// TestMergeLegacyPptMasterRunInheritsBoldWhenNotExplicitlySet verifies the
// conflict-resolution order: a run that has FontSize from STPA (FontSize>0,
// BoldSet=false) should receive Bold from the master, because BoldSet==false
// means the STPA never mentioned Bold for this run.
func TestMergeLegacyPptMasterRunInheritsBoldWhenNotExplicitlySet(t *testing.T) {
	boldTrue := true
	master := stpaCFRun{Bold: &boldTrue, FontSize: 2400}
	run := &presentationir.Run{Text: "Hello", FontSize: 3600}
	// BoldSet is false (zero value) — Bold was never in the STPA mask.

	merged := mergeLegacyPptMasterRun(run, master, false)
	if merged == nil {
		t.Fatal("mergeLegacyPptMasterRun returned nil")
	}
	if !merged.Bold {
		t.Fatal("Bold = false; want true (inherited from master because BoldSet==false)")
	}
	if merged.FontSize != 3600 {
		t.Fatalf("FontSize = %d; want 3600 (explicit local value preserved)", merged.FontSize)
	}
}

// TestMergeLegacyPptMasterRunRespectsExplicitBoldFalse verifies that when the
// STPA explicitly set Bold=false (BoldSet=true), the master Bold=true does NOT
// override it.
func TestMergeLegacyPptMasterRunRespectsExplicitBoldFalse(t *testing.T) {
	boldTrue := true
	master := stpaCFRun{Bold: &boldTrue}
	run := &presentationir.Run{Text: "World", Bold: false, BoldSet: true}

	merged := mergeLegacyPptMasterRun(run, master, false)
	if merged == nil {
		t.Fatal("mergeLegacyPptMasterRun returned nil")
	}
	if merged.Bold {
		t.Fatal("Bold = true; want false (explicit STPA Bold=false must block master inheritance)")
	}
}

func TestLegacyPptMasterTextLevelsForTypeUnknownFallsBackToBody(t *testing.T) {
	// POI HSLFSlideMaster.getPropCollection maps a null txtype to BODY fallback.
	// Shapes without a TextHeaderAtom (textType = pptTextTypeUnknown = -1) should
	// therefore receive body master defaults rather than being left unstyled.
	styles := legacyPptMasterTextStyles{
		pptTextTypeBody: {
			0: {run: stpaCFRun{FontSize: 2400, Color: "AABBCC"}},
		},
	}
	levels := legacyPptMasterTextLevelsForType(styles, pptTextTypeUnknown)
	if len(levels) == 0 {
		t.Fatal("legacyPptMasterTextLevelsForType(Unknown) returned empty levels; want body fallback")
	}
	if got := levels[0].run.FontSize; got != 2400 {
		t.Fatalf("levels[0].run.FontSize = %d, want 2400 (from body fallback)", got)
	}
	if got := levels[0].run.Color; got != "AABBCC" {
		t.Fatalf("levels[0].run.Color = %q, want AABBCC (from body fallback)", got)
	}
}

func TestApplyLegacyPptMasterTextStylesClampsToHighestAvailableLevel(t *testing.T) {
	// POI HSLFSlideMaster.getPropHelper clamps to styles.size()-1 when the requested
	// level exceeds the highest defined level, rather than always falling back to level 0.
	// Here master has levels {0, 2} only; a paragraph at level 3 must inherit level 2
	// (highest ≤ 3), not level 0.
	styles := legacyPptMasterTextStyles{
		pptTextTypeBody: {
			0: {run: stpaCFRun{FontFamily: "LevelZero"}},
			2: {run: stpaCFRun{FontFamily: "LevelTwo"}},
		},
	}
	shape := &presentationir.Shape{
		Kind: presentationir.ShapeText,
		Paragraphs: []*presentationir.Paragraph{
			// Level 1: highest ≤ 1 is level 0 → LevelZero
			{Level: 1, Runs: []*presentationir.Run{{Text: "L1"}}},
			// Level 3: highest ≤ 3 is level 2 → LevelTwo (not LevelZero)
			{Level: 3, Runs: []*presentationir.Run{{Text: "L3"}}},
		},
	}
	applyLegacyPptMasterTextStylesToShape(shape, styles, pptTextTypeBody)
	if len(shape.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(shape.Paragraphs))
	}
	// Level 1 paragraph → clamped to level 0
	if got := shape.Paragraphs[0].Runs[0].FontFamily; got != "LevelZero" {
		t.Fatalf("level-1 paragraph FontFamily = %q, want LevelZero (from level 0)", got)
	}
	// Level 3 paragraph → clamped to level 2
	if got := shape.Paragraphs[1].Runs[0].FontFamily; got != "LevelTwo" {
		t.Fatalf("level-3 paragraph FontFamily = %q, want LevelTwo (from level 2, not level 0)", got)
	}
}

func TestParseLegacyPptFOPTFillColorExtractsSolidRGB(t *testing.T) {
	// Build a 2-property FOPT body: fillType=0 (solid) and fillColor=0x00CC3366 (R=0x66,G=0x33,B=0xCC).
	foptBody := make([]byte, 12)
	binary.LittleEndian.PutUint16(foptBody[0:2], legacyOfficeArtPropFillType)
	binary.LittleEndian.PutUint32(foptBody[2:6], 0) // fillType = solid
	binary.LittleEndian.PutUint16(foptBody[6:8], legacyOfficeArtPropFillColor)
	binary.LittleEndian.PutUint32(foptBody[8:12], 0x00CC3366) // R=0x66,G=0x33,B=0xCC,extra=0x00
	color, ok := parseLegacyPptFOPTFillColor(foptBody, 2, nil)
	if !ok {
		t.Fatal("parseLegacyPptFOPTFillColor returned ok=false")
	}
	if color != "6633CC" {
		t.Fatalf("color = %q, want %q", color, "6633CC")
	}
}

func TestParseLegacyPptFOPTFillColorRejectsNoFill(t *testing.T) {
	// fillType=0 (solid), fillColor present, but fillStyleBooleanProperties has fFilled=0 + use-bit set.
	foptBody := make([]byte, 18)
	binary.LittleEndian.PutUint16(foptBody[0:2], legacyOfficeArtPropFillType)
	binary.LittleEndian.PutUint32(foptBody[2:6], 0)
	binary.LittleEndian.PutUint16(foptBody[6:8], legacyOfficeArtPropFillColor)
	binary.LittleEndian.PutUint32(foptBody[8:12], 0x00FF0000) // R=0x00,G=0x00,B=0xFF
	binary.LittleEndian.PutUint16(foptBody[12:14], legacyOfficeArtPropFillBools)
	// fFilled (bit 4) = 0, fFilled use (bit 20) = 1 → no fill
	binary.LittleEndian.PutUint32(foptBody[14:18], 1<<20)
	_, ok := parseLegacyPptFOPTFillColor(foptBody, 3, nil)
	if ok {
		t.Fatal("parseLegacyPptFOPTFillColor returned ok=true, want false for no-fill shape")
	}
}

func TestParseLegacyPptFOPTFillColorResolvesSchemeColor(t *testing.T) {
	colorScheme := map[uint8]string{3: "AABBCC"}
	foptBody := make([]byte, 12)
	binary.LittleEndian.PutUint16(foptBody[0:2], legacyOfficeArtPropFillType)
	binary.LittleEndian.PutUint32(foptBody[2:6], 0)
	binary.LittleEndian.PutUint16(foptBody[6:8], legacyOfficeArtPropFillColor)
	// R=3 (scheme index), extra=0xFE (scheme color flag)
	binary.LittleEndian.PutUint32(foptBody[8:12], 0xFE000003)
	color, ok := parseLegacyPptFOPTFillColor(foptBody, 2, colorScheme)
	if !ok {
		t.Fatal("parseLegacyPptFOPTFillColor returned ok=false")
	}
	if color != "AABBCC" {
		t.Fatalf("color = %q, want %q", color, "AABBCC")
	}
}

func TestParseLegacyPptMasterBgFillOverridesThemeLt1(t *testing.T) {
	// Build: Document → MainMasterContainer → BackgroundContainer →
	//   BackgroundAtom (clientFillSpecified=1) + PPDrawing → OfficeArt → FOPT fill=0x00FF0000 (red).
	fillR, fillG, fillB := byte(0xFF), byte(0x00), byte(0x00)
	foptBody := make([]byte, 12)
	binary.LittleEndian.PutUint16(foptBody[0:2], legacyOfficeArtPropFillType)
	binary.LittleEndian.PutUint32(foptBody[2:6], 0)
	binary.LittleEndian.PutUint16(foptBody[6:8], legacyOfficeArtPropFillColor)
	binary.LittleEndian.PutUint32(foptBody[8:12], uint32(fillR)|uint32(fillG)<<8|uint32(fillB)<<16)
	// FOPT record: version=0 inst=2 (propCount), type=legacyOfficeArtFOPT
	foptRec := buildOfficeArtRecord(2, legacyOfficeArtFOPT, 0, foptBody)
	// SpContainer wrapping the FOPT.
	spContainer := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x0F, foptRec)
	// PPDrawing container wrapping the SpContainer.
	ppDrawing := buildPptContainerRecord(0, pptRTPPDrawing, 0x0F, spContainer)
	// BackgroundAtom: clientFillSpecified = bit 0 = 1.
	bgAtomPayload := make([]byte, 4)
	binary.LittleEndian.PutUint32(bgAtomPayload, 1)
	bgAtom := buildPptAtomRecord(0, pptRTBackgroundAtom, bgAtomPayload)
	// BackgroundContainer.
	bgContainer := buildPptContainerRecord(0, pptRTBackground, 0x0F, append(bgAtom, ppDrawing...))
	// MainMasterContainer holding the BackgroundContainer.
	masterContainer := buildPptContainerRecord(0, pptRTMainMaster, 0x0F, bgContainer)
	// Document container.
	document := buildPptContainerRecord(0, pptRTDocument, 0x0F, masterContainer)

	colorScheme := map[uint8]string{0: "FFFFFF"} // lt1 = white (would be the theme fallback)
	_, documentPayload, _, ok := readPptRecord(document, 0)
	if !ok {
		t.Fatal("synthetic Document record did not decode")
	}
	fill := ""
	for pos := 0; pos+8 <= len(documentPayload); {
		child, childPayload, next, ok := readPptRecord(documentPayload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTMainMaster {
			fill = parseLegacyPptBgFillFromContainer(childPayload, colorScheme)
			break
		}
		pos = next
	}
	if fill != "FF0000" {
		t.Fatalf("parseLegacyPptMasterBgFill = %q, want %q", fill, "FF0000")
	}
}

func TestBuildPresentationUsesExplicitMasterBgFillOverThemeLt1(t *testing.T) {
	theme := &presentationir.Theme{ColorScheme: map[string]string{"lt1": "FFFFFF"}}
	slides := []legacySlideText{{Fragments: []legacySlideFragment{{Text: "Slide text", TextType: pptTextTypeBody}}}}
	pres := buildPresentationWithMasterComposition(slides, nil, nil, legacyPptHeaderFooterInfo{}, "test.ppt", theme, nil, nil, "FF0000", nil)
	if len(pres.Slides) == 0 {
		t.Fatal("no slides")
	}
	if got := pres.Slides[0].BgColor; got != "FF0000" {
		t.Fatalf("BgColor = %q, want FF0000 (explicit override)", got)
	}
}

func TestBuildPresentationFallsBackToThemeLt1WhenNoExplicitBgFill(t *testing.T) {
	theme := &presentationir.Theme{ColorScheme: map[string]string{"lt1": "EEEEEE"}}
	slides := []legacySlideText{{Fragments: []legacySlideFragment{{Text: "Slide text", TextType: pptTextTypeBody}}}}
	pres := buildPresentationWithMasterComposition(slides, nil, nil, legacyPptHeaderFooterInfo{}, "test.ppt", theme, nil, nil, "", nil)
	if len(pres.Slides) == 0 {
		t.Fatal("no slides")
	}
	if got := pres.Slides[0].BgColor; got != "EEEEEE" {
		t.Fatalf("BgColor = %q, want EEEEEE (theme lt1 fallback)", got)
	}
}

func mustLegacyPptContainerPayload(t *testing.T, data []byte, recordType uint16) []byte {
	t.Helper()
	record, payload, _, ok := readPptRecord(data, 0)
	if !ok || record.Type != recordType {
		t.Fatalf("synthetic PPT container type=%#x did not decode", recordType)
	}
	return payload
}

func TestParseLegacyPptExHyperlinkURLsExtractsURLByID(t *testing.T) {
	exHyperlink := buildPptContainerRecord(0, pptRTExHyperlink, 0x0F, append(
		buildPptAtomRecord(0, pptRTExHyperlinkAtom, marshalUint32LE(7)),
		buildPptAtomRecord(0, pptRTCString, encodeLegacyUTF16("https://example.com/page"))...,
	))
	exObjList := buildPptContainerRecord(0, pptRTExObjList, 0x0F, exHyperlink)
	document := buildPptContainerRecord(0, pptRTDocument, 0x0F, exObjList)

	urls := parseLegacyPptExHyperlinkURLsFromPayload(mustLegacyPptContainerPayload(t, document, pptRTDocument))
	if len(urls) != 1 {
		t.Fatalf("len(urls) = %d, want 1", len(urls))
	}
	if got := urls[7]; got != "https://example.com/page" {
		t.Fatalf("urls[7] = %q, want %q", got, "https://example.com/page")
	}
}

func TestParseLegacyPptExHyperlinkURLsSkipsSchemaURLs(t *testing.T) {
	exHyperlink := buildPptContainerRecord(0, pptRTExHyperlink, 0x0F, append(
		buildPptAtomRecord(0, pptRTExHyperlinkAtom, marshalUint32LE(5)),
		buildPptAtomRecord(0, pptRTCString, encodeLegacyUTF16("http://schemas.openxmlformats.org/drawingml/2006/main"))...,
	))
	exObjList := buildPptContainerRecord(0, pptRTExObjList, 0x0F, exHyperlink)
	document := buildPptContainerRecord(0, pptRTDocument, 0x0F, exObjList)

	urls := parseLegacyPptExHyperlinkURLsFromPayload(mustLegacyPptContainerPayload(t, document, pptRTDocument))
	if len(urls) != 0 {
		t.Fatalf("urls = %#v, want empty", urls)
	}
}

func TestParseLegacyPptExHyperlinkURLsRequiresAtomAndURL(t *testing.T) {
	missingAtom := buildPptContainerRecord(0, pptRTExHyperlink, 0x0F,
		buildPptAtomRecord(0, pptRTCString, encodeLegacyUTF16("https://example.com/noatom")),
	)
	missingURL := buildPptContainerRecord(0, pptRTExHyperlink, 0x0F,
		buildPptAtomRecord(0, pptRTExHyperlinkAtom, marshalUint32LE(9)),
	)
	exObjList := buildPptContainerRecord(0, pptRTExObjList, 0x0F, append(missingAtom, missingURL...))
	document := buildPptContainerRecord(0, pptRTDocument, 0x0F, exObjList)

	urls := parseLegacyPptExHyperlinkURLsFromPayload(mustLegacyPptContainerPayload(t, document, pptRTDocument))
	if len(urls) != 0 {
		t.Fatalf("urls = %#v, want empty", urls)
	}
}

// buildInteractiveInfoAtomPayload constructs the 9-byte body of an InteractiveInfoAtom:
//
//	bytes 0-3: soundRef (uint32 LE, 0)
//	bytes 4-7: hyperlinkID (uint32 LE)
//	byte  8:   action (4 = ACTION_HYPERLINK)
func buildInteractiveInfoAtomPayload(hyperlinkID uint32) []byte {
	p := make([]byte, 9)
	binary.LittleEndian.PutUint32(p[4:8], hyperlinkID)
	p[8] = 4 // ACTION_HYPERLINK
	return p
}

func TestParseSlideListWithTextAppliesInteractiveInfoHyperlinkToRuns(t *testing.T) {
	slideList := append(
		append(
			append(
				buildPptAtomRecord(0, pptRTSlidePersistAtom, marshalUint32LE(1)),
				buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(uint32(pptTextTypeBody)))...,
			),
			buildPptAtomRecord(0, pptRTTextCharsAtom, encodeLegacyUTF16("Click here"))...,
		),
		buildPptAtomRecord(0, pptRTInteractiveInfoAtom, buildInteractiveInfoAtomPayload(7))...,
	)

	slides := parseSlideListWithText(slideList, nil, nil, map[uint32]string{7: "https://example.com/run"})
	if len(slides) != 1 {
		t.Fatalf("len(slides) = %d, want 1", len(slides))
	}
	if len(slides[0].Fragments) != 1 {
		t.Fatalf("len(slides[0].Fragments) = %d, want 1", len(slides[0].Fragments))
	}
	paras := slides[0].Fragments[0].Paragraphs
	if len(paras) != 1 || len(paras[0].Runs) != 1 {
		t.Fatalf("paragraphs/runs = %d/%d, want 1/1", len(paras), len(paras[0].Runs))
	}
	if got := paras[0].Runs[0].HyperlinkURL; got != "https://example.com/run" {
		t.Fatalf("run HyperlinkURL = %q, want %q", got, "https://example.com/run")
	}
}

func TestParseLegacyPptTextBoxFragmentsAppliesInteractiveInfoHyperlinkToRuns(t *testing.T) {
	payload := append(
		append(
			buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(uint32(pptTextTypeBody))),
			buildPptAtomRecord(0, pptRTTextCharsAtom, encodeLegacyUTF16("Visit"))...,
		),
		buildPptContainerRecord(0, pptRTInteractiveInfo, 0x0F,
			buildPptAtomRecord(0, pptRTInteractiveInfoAtom, buildInteractiveInfoAtomPayload(3)),
		)...,
	)

	frags, _ := parseLegacyPptTextBoxFragments(payload, nil, nil, map[uint32]string{3: "https://example.com/box"})
	if len(frags) != 1 {
		t.Fatalf("len(frags) = %d, want 1", len(frags))
	}
	paras := frags[0].Paragraphs
	if len(paras) != 1 || len(paras[0].Runs) != 1 {
		t.Fatalf("paragraphs/runs = %d/%d, want 1/1", len(paras), len(paras[0].Runs))
	}
	if got := paras[0].Runs[0].HyperlinkURL; got != "https://example.com/box" {
		t.Fatalf("run HyperlinkURL = %q, want %q", got, "https://example.com/box")
	}
}

func TestParseSlideListWithTextCapturesLegacySlideIdentity(t *testing.T) {
	slideAtom := make([]byte, 24)
	binary.LittleEndian.PutUint32(slideAtom[0:4], 0x0010)       // blank
	binary.LittleEndian.PutUint32(slideAtom[12:16], 0x80000000) // MainMaster identifier

	slideList := append(
		buildPptAtomRecord(0, pptRTSlidePersistAtom, marshalUint32LE(42)),
		buildPptAtomRecord(0, pptRTSlideAtom, slideAtom)...,
	)

	slides := parseSlideListWithText(slideList, nil, nil, nil)
	if len(slides) != 1 {
		t.Fatalf("len(slides) = %d, want 1", len(slides))
	}
	if slides[0].MasterID != -2147483648 {
		t.Fatalf("MasterID = %d, want -2147483648", slides[0].MasterID)
	}
	if slides[0].LayoutType != "blank" {
		t.Fatalf("LayoutType = %q, want blank", slides[0].LayoutType)
	}
}

func TestParseLegacyPptInteractiveInfoURLSkipsUnknownID(t *testing.T) {
	payload := buildInteractiveInfoAtomPayload(99)
	if got := parseLegacyPptInteractiveInfoURL(payload, map[uint32]string{7: "https://example.com"}); got != "" {
		t.Fatalf("parseLegacyPptInteractiveInfoURL = %q, want empty", got)
	}
}

func TestParseLegacyPptInteractiveInfoURLReadsHyperlinkIDAtOffset4(t *testing.T) {
	// soundRef (bytes 0-3) = 5 — which also exists in the exHyperlinks map but must NOT be matched.
	// hyperlinkID (bytes 4-7) = 2 — the correct entry to look up.
	p := make([]byte, 9)
	binary.LittleEndian.PutUint32(p[0:4], 5) // soundRef = 5
	binary.LittleEndian.PutUint32(p[4:8], 2) // hyperlinkID = 2
	p[8] = 4                                 // ACTION_HYPERLINK
	exHyperlinks := map[uint32]string{
		5: "https://example.com/wrong",
		2: "https://example.com/correct",
	}
	if got := parseLegacyPptInteractiveInfoURL(p, exHyperlinks); got != "https://example.com/correct" {
		t.Fatalf("got %q, want https://example.com/correct", got)
	}
}

func TestParseLegacyPptInteractiveInfoURLIgnoresSlideJumpAction(t *testing.T) {
	// action = 3 (ACTION_JUMP) — targets a slide, not an external URL.
	p := make([]byte, 9)
	binary.LittleEndian.PutUint32(p[4:8], 7) // hyperlinkID present in map
	p[8] = 3                                 // ACTION_JUMP
	if got := parseLegacyPptInteractiveInfoURL(p, map[uint32]string{7: "https://example.com"}); got != "" {
		t.Fatalf("got %q for slide-jump action, want empty", got)
	}
}

func TestParseLegacyPptTextShapeContainerExtractsAnimationInfo(t *testing.T) {
	anim := make([]byte, 28)
	binary.LittleEndian.PutUint16(anim[4:6], 0x0004) // fAutomatic = 1
	binary.LittleEndian.PutUint32(anim[12:16], 150)  // delay
	binary.LittleEndian.PutUint16(anim[16:18], 2)    // orderID
	anim[20] = 1                                     // one build
	anim[21] = 6                                     // fade

	container := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x700, 0)),
				buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, []byte{0, 0, 0, 0, 10, 0, 10, 0})...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientData, 0x0000,
				buildPptContainerRecord(0, pptRTAnimationInfo, 0x0F, buildPptAtomRecord(0, pptRTAnimationInfoAtom, anim)))...,
		),
		buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000,
			append(
				buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeBody)),
				buildPptAtomRecord(0, pptRTTextBytesAtom, []byte("Animated"))...,
			),
		)...,
	))
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x0F, container)

	shapes := buildLegacyPptTextShapesFromMetas(collectLegacyPptVisualScene(payload).textMetas(nil, nil, nil), legacySlideText{})
	if len(shapes) != 1 {
		t.Fatalf("len(shapes) = %d, want 1", len(shapes))
	}
	shape := shapes[0]
	if shape.AnimStep != 2 {
		t.Fatalf("AnimStep = %d, want 2", shape.AnimStep)
	}
	if len(shape.AnimationTriggers) != 1 || shape.AnimationTriggers[0] != "afterPrevious" {
		t.Fatalf("AnimationTriggers = %v, want [afterPrevious]", shape.AnimationTriggers)
	}
	if len(shape.EmphasisEffects) != 1 || shape.EmphasisEffects[0] != "fade" {
		t.Fatalf("EmphasisEffects = %v, want [fade]", shape.EmphasisEffects)
	}

	timelineShapes := cloneLegacyExtraShapes(0, 1, shapes)
	timeline := buildLegacySlideAnimationTimeline(nil, timelineShapes, nil, nil, nil)
	if len(timeline) != 1 {
		t.Fatalf("len(timeline) = %d, want 1", len(timeline))
	}
	if timeline[0].ShapeID != 1 || timeline[0].Step != 2 || timeline[0].Trigger != "afterPrevious" || timeline[0].Effect != "fade" {
		t.Fatalf("timeline[0] = %+v, want shapeID=1 step=2 trigger=afterPrevious effect=fade", timeline[0])
	}
}

func TestParseLegacyPptTextShapesFromSlideMapsGroupedChildAnchorBounds(t *testing.T) {
	spgr := make([]byte, 16)
	binary.LittleEndian.PutUint32(spgr[0:4], uint32(int32(0)))
	binary.LittleEndian.PutUint32(spgr[4:8], uint32(int32(0)))
	binary.LittleEndian.PutUint32(spgr[8:12], uint32(int32(1000)))
	binary.LittleEndian.PutUint32(spgr[12:16], uint32(int32(1000)))

	childAnchor := make([]byte, 16)
	binary.LittleEndian.PutUint32(childAnchor[0:4], uint32(int32(100)))
	binary.LittleEndian.PutUint32(childAnchor[4:8], uint32(int32(200)))
	binary.LittleEndian.PutUint32(childAnchor[8:12], uint32(int32(600)))
	binary.LittleEndian.PutUint32(childAnchor[12:16], uint32(int32(700)))

	groupContainer := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F,
		append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x500, 0x00000001)),
				buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, spgr)...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, []byte{0, 0, 0, 0, 128, 22, 224, 16})...,
		),
	)
	textContainer := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F,
		append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x501, 0x0000000A)),
				buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, childAnchor)...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000,
				append(
					buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeBody)),
					buildPptAtomRecord(0, pptRTTextBytesAtom, []byte("Grouped"))...,
				),
			)...,
		),
	)
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x000F,
		buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, append(groupContainer, textContainer...)),
	)

	shapes := buildLegacyPptTextShapesFromMetas(collectLegacyPptVisualScene(payload).textMetas(nil, nil, nil), legacySlideText{})
	if len(shapes) != 1 {
		t.Fatalf("len(shapes) = %d, want 1", len(shapes))
	}
	if got := paragraphText(shapes[0].Paragraphs[0]); got != "Grouped" {
		t.Fatalf("paragraph text = %q, want %q", got, "Grouped")
	}
	wantBounds := presentationir.Rect{X: 914400, Y: 1371600, W: 4572000, H: 3429000}
	if shapes[0].Bounds != wantBounds {
		t.Fatalf("bounds = %+v, want %+v", shapes[0].Bounds, wantBounds)
	}
}

func TestParseLegacyPptImageShapesFromSlideMapsGroupedChildAnchorBounds(t *testing.T) {
	spgr := make([]byte, 16)
	binary.LittleEndian.PutUint32(spgr[0:4], uint32(int32(0)))
	binary.LittleEndian.PutUint32(spgr[4:8], uint32(int32(0)))
	binary.LittleEndian.PutUint32(spgr[8:12], uint32(int32(1000)))
	binary.LittleEndian.PutUint32(spgr[12:16], uint32(int32(1000)))

	childAnchor := make([]byte, 16)
	binary.LittleEndian.PutUint32(childAnchor[0:4], uint32(int32(100)))
	binary.LittleEndian.PutUint32(childAnchor[4:8], uint32(int32(200)))
	binary.LittleEndian.PutUint32(childAnchor[8:12], uint32(int32(600)))
	binary.LittleEndian.PutUint32(childAnchor[12:16], uint32(int32(700)))

	fopt := make([]byte, 6)
	binary.LittleEndian.PutUint16(fopt[0:2], legacyOfficeArtPropPib)
	binary.LittleEndian.PutUint32(fopt[2:6], 1)

	groupContainer := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F,
		append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x510, 0x00000001)),
				buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, spgr)...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, []byte{0, 0, 0, 0, 128, 22, 224, 16})...,
		),
	)
	imageContainer := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F,
		append(
			append(
				buildOfficeArtRecord(legacyOfficeArtShapePicture, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x511, 0x00000002)),
				buildOfficeArtRecord(1, legacyOfficeArtFOPT, 0x0000, fopt)...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, childAnchor)...,
		),
	)
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x000F,
		buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, append(groupContainer, imageContainer...)),
	)

	scene := collectLegacyPptVisualScene(payload)
	var shapes []*presentationir.Shape
	collectLegacyPptImageShapesFromEvents(scene.dispatch.containers.officeArtEvents(), legacyPptGroupFrame{}, nil, map[uint32]*legacyPptPicture{1: {Data: []byte{1, 2, 3}, MIME: "image/png"}}, nil, &shapes)
	if len(shapes) != 1 {
		t.Fatalf("len(shapes) = %d, want 1", len(shapes))
	}
	wantBounds := presentationir.Rect{X: 914400, Y: 1371600, W: 4572000, H: 3429000}
	if shapes[0].Bounds != wantBounds {
		t.Fatalf("bounds = %+v, want %+v", shapes[0].Bounds, wantBounds)
	}
}

func TestParseLegacyPptConnectorShapesFromSlideMapsGroupedChildAnchorAndArrowStyle(t *testing.T) {
	appendFoptProp := func(dst []byte, opid uint16, value uint32) []byte {
		prop := make([]byte, 6)
		binary.LittleEndian.PutUint16(prop[0:2], opid)
		binary.LittleEndian.PutUint32(prop[2:6], value)
		return append(dst, prop...)
	}

	spgr := make([]byte, 16)
	binary.LittleEndian.PutUint32(spgr[8:12], 1000)
	binary.LittleEndian.PutUint32(spgr[12:16], 800)

	childAnchor := make([]byte, 16)
	binary.LittleEndian.PutUint32(childAnchor[0:4], 100)
	binary.LittleEndian.PutUint32(childAnchor[4:8], 200)
	binary.LittleEndian.PutUint32(childAnchor[8:12], 700)
	binary.LittleEndian.PutUint32(childAnchor[12:16], 600)

	groupContainerBody := append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x520, 0x00000001)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, spgr)...,
		),
		buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, []byte{0, 0, 0, 0, 128, 22, 224, 16})...,
	)
	groupContainer := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, groupContainerBody)
	frame, ok := parseLegacyPptGroupFrameFromSpContainer(groupContainerBody, legacyPptGroupFrame{})
	if !ok {
		t.Fatal("parseLegacyPptGroupFrameFromSpContainer returned false")
	}
	expectedBounds, ok := mapLegacyPptRectThroughGroupFrame(presentationir.Rect{X: 100, Y: 200, W: 600, H: 400}, frame)
	if !ok {
		t.Fatal("mapLegacyPptRectThroughGroupFrame returned false")
	}

	var fopt []byte
	fopt = appendFoptProp(fopt, legacyOfficeArtPropLineColor, 0x00332211)
	fopt = appendFoptProp(fopt, legacyOfficeArtPropLineWidth, 25400)
	fopt = appendFoptProp(fopt, legacyOfficeArtPropLineDashing, 7)
	fopt = appendFoptProp(fopt, legacyOfficeArtPropLineStartArrowhead, 2)
	fopt = appendFoptProp(fopt, legacyOfficeArtPropLineStartArrowWidth, 3)
	fopt = appendFoptProp(fopt, legacyOfficeArtPropLineStartArrowLength, 1)
	fopt = appendFoptProp(fopt, legacyOfficeArtPropLineEndArrowhead, 4)
	fopt = appendFoptProp(fopt, legacyOfficeArtPropLineEndArrowWidth, 1)
	fopt = appendFoptProp(fopt, legacyOfficeArtPropLineEndArrowLength, 3)

	connectorContainer := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F,
		append(
			append(
				buildOfficeArtRecord(20, legacyOfficeArtSp, 0x0002,
					officeArtSpPayload(0x521, legacyOfficeArtSpFlagChild|legacyOfficeArtSpFlagFlipH|legacyOfficeArtSpFlagFlipV)),
				buildOfficeArtRecord(9, legacyOfficeArtFOPT, 0x0003, fopt)...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, childAnchor)...,
		),
	)
	if direct, ok := parseLegacyPptConnectorShapeContainer(connectorContainer[8:], frame, nil); !ok {
		t.Fatal("parseLegacyPptConnectorShapeContainer returned false")
	} else if direct.bounds != expectedBounds {
		t.Fatalf("direct bounds = %+v, want %+v", direct.bounds, expectedBounds)
	}
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x000F,
		buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, append(groupContainer, connectorContainer...)),
	)

	scene := collectLegacyPptVisualScene(payload)
	var shapes []*presentationir.Shape
	collectLegacyPptConnectorShapesFromEvents(scene.dispatch.containers.officeArtEvents(), legacyPptGroupFrame{}, nil, scene.dispatch.rules, nil, &shapes)
	if len(shapes) != 1 {
		t.Fatalf("len(shapes) = %d, want 1", len(shapes))
	}
	shape := shapes[0]
	if shape.Kind != presentationir.ShapeConnector {
		t.Fatalf("kind = %v, want connector", shape.Kind)
	}
	if shape.OriginalShapeID != 0x521 {
		t.Fatalf("OriginalShapeID = %#x, want %#x", shape.OriginalShapeID, 0x521)
	}
	if shape.Bounds != expectedBounds {
		t.Fatalf("bounds = %+v, want %+v", shape.Bounds, expectedBounds)
	}
	if shape.ConnectorStart == nil || shape.ConnectorEnd == nil {
		t.Fatalf("connector endpoints = (%+v,%+v), want non-nil", shape.ConnectorStart, shape.ConnectorEnd)
	}
	if *shape.ConnectorStart != (presentationir.Point{X: expectedBounds.X + expectedBounds.W, Y: expectedBounds.Y + expectedBounds.H}) {
		t.Fatalf("ConnectorStart = %+v, want bottom-right after flip", *shape.ConnectorStart)
	}
	if *shape.ConnectorEnd != (presentationir.Point{X: expectedBounds.X, Y: expectedBounds.Y}) {
		t.Fatalf("ConnectorEnd = %+v, want top-left after flip", *shape.ConnectorEnd)
	}
	if shape.ConnectorType != "straightConnector1" {
		t.Fatalf("ConnectorType = %q, want straightConnector1", shape.ConnectorType)
	}
	if !shape.ConnectorFlipH || !shape.ConnectorFlipV {
		t.Fatalf("connector flips = (%t,%t), want both true", shape.ConnectorFlipH, shape.ConnectorFlipV)
	}
	if shape.LineColor != "112233" {
		t.Fatalf("LineColor = %q, want 112233", shape.LineColor)
	}
	if shape.LineWidthPt != 2 {
		t.Fatalf("LineWidthPt = %v, want 2", shape.LineWidthPt)
	}
	if shape.LineStyle == nil || shape.LineStyle.Dash != drawing.DashPatternLongDash {
		t.Fatalf("LineStyle = %#v, want long dash", shape.LineStyle)
	}
	if shape.ArrowHead != "stealth" || shape.ArrowHeadWidth != "med" || shape.ArrowHeadLength != "med" {
		t.Fatalf("head/start arrow = (%q,%q,%q), want (stealth,med,med)", shape.ArrowHead, shape.ArrowHeadWidth, shape.ArrowHeadLength)
	}
	if shape.ArrowTail != "oval" || shape.ArrowTailWidth != "med" || shape.ArrowTailLength != "med" {
		t.Fatalf("tail/end arrow = (%q,%q,%q), want (oval,med,med)", shape.ArrowTail, shape.ArrowTailWidth, shape.ArrowTailLength)
	}
}

func TestParseLegacyPptGroupShapesFromSlideBuildsNestedHierarchy(t *testing.T) {
	buildRect32 := func(left, top, right, bottom int32) []byte {
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint32(buf[0:4], uint32(left))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(top))
		binary.LittleEndian.PutUint32(buf[8:12], uint32(right))
		binary.LittleEndian.PutUint32(buf[12:16], uint32(bottom))
		return buf
	}
	buildClientAnchor := func(x1, y1, x2, y2 uint16) []byte {
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint16(buf[0:2], x1)
		binary.LittleEndian.PutUint16(buf[2:4], y1)
		binary.LittleEndian.PutUint16(buf[4:6], x2)
		binary.LittleEndian.PutUint16(buf[6:8], y2)
		return buf
	}
	buildTextShape := func(shapeID uint32, left, top, right, bottom int32, text string) []byte {
		textbox := append(
			buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeBody)),
			buildPptAtomRecord(0, pptRTTextBytesAtom, []byte(text))...,
		)
		return buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(shapeID, legacyOfficeArtSpFlagChild)),
				buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, buildRect32(left, top, right, bottom))...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000, textbox)...,
		))
	}

	innerGroupRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x810, 0x03)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, buildRect32(0, 0, 1000, 1000))...,
		),
		buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, buildRect32(400, 200, 900, 900))...,
	))
	innerGroup := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, append(
		innerGroupRoot,
		buildTextShape(0x811, 100, 100, 800, 600, "Inner child")...,
	))

	outerGroupRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x800, 0x01)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, buildRect32(0, 0, 1000, 1000))...,
		),
		buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, buildClientAnchor(0, 0, 2000, 1600))...,
	))
	outerGroup := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, append(
		append(
			outerGroupRoot,
			buildTextShape(0x801, 100, 100, 500, 400, "Outer child")...,
		),
		innerGroup...,
	))
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x000F, outerGroup)

	flat := buildLegacyPptTextShapesFromMetas(collectLegacyPptVisualScene(payload).textMetas(nil, nil, nil), legacySlideText{})
	if len(flat) != 2 {
		t.Fatalf("len(flat) = %d, want 2 flat child text shapes", len(flat))
	}

	scene := collectLegacyPptVisualScene(payload)
	groups := buildLegacyPptGroupShapesFromSceneWithOwnership(scene.dispatch.containers, legacyPptDiscoveredShapeSource(legacyPptShapeIndexByOriginalID(flat)), nil, nil, nil, nil)
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	group := groups[0]
	if group.Kind != presentationir.ShapeGroup {
		t.Fatalf("group kind = %v, want group", group.Kind)
	}
	if len(group.Children) != 2 {
		t.Fatalf("len(group.Children) = %d, want 2", len(group.Children))
	}
	if got := paragraphText(group.Children[0].Paragraphs[0]); got != "Outer child" {
		t.Fatalf("group.Children[0] text = %q, want Outer child", got)
	}
	if group.Children[1].Kind != presentationir.ShapeGroup {
		t.Fatalf("group.Children[1].Kind = %v, want group", group.Children[1].Kind)
	}
	if len(group.Children[1].Children) != 1 {
		t.Fatalf("len(group.Children[1].Children) = %d, want 1", len(group.Children[1].Children))
	}
	if got := paragraphText(group.Children[1].Children[0].Paragraphs[0]); got != "Inner child" {
		t.Fatalf("nested child text = %q, want Inner child", got)
	}

	filtered := filterLegacyGroupedShapeDuplicates(map[int][]*presentationir.Shape{3: flat}, map[int][]*presentationir.Shape{3: groups})
	if got := len(filtered[3]); got != 0 {
		t.Fatalf("len(filtered[3]) = %d, want 0 grouped duplicates remaining", got)
	}
}

func TestParseLegacyPptTableShapeFromGroupContainerSetsOriginalShapeID(t *testing.T) {
	buildRect32 := func(left, top, right, bottom int32) []byte {
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint32(buf[0:4], uint32(left))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(top))
		binary.LittleEndian.PutUint32(buf[8:12], uint32(right))
		binary.LittleEndian.PutUint32(buf[12:16], uint32(bottom))
		return buf
	}
	buildClientAnchor := func(x1, y1, x2, y2 uint16) []byte {
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint16(buf[0:2], x1)
		binary.LittleEndian.PutUint16(buf[2:4], y1)
		binary.LittleEndian.PutUint16(buf[4:6], x2)
		binary.LittleEndian.PutUint16(buf[6:8], y2)
		return buf
	}
	buildCell := func(shapeID uint32, left, top, right, bottom int32, text string) []byte {
		textbox := append(
			buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeOther)),
			buildPptAtomRecord(0, pptRTTextBytesAtom, []byte(text))...,
		)
		return buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(shapeID, legacyOfficeArtSpFlagChild)),
				buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, buildRect32(left, top, right, bottom))...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000, textbox)...,
		))
	}

	fopt := make([]byte, 6)
	binary.LittleEndian.PutUint16(fopt[0:2], legacyOfficeArtPropTableProperties)
	binary.LittleEndian.PutUint32(fopt[2:6], 1)

	groupRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x901, 0x01)),
				buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, buildRect32(0, 0, 1000, 800))...,
			),
			buildOfficeArtRecord(1, legacyOfficeArtFOPT, 0x0000, fopt)...,
		),
		buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, buildClientAnchor(0, 0, 2000, 1600))...,
	))
	group := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, append(
		append(
			append(
				append(groupRoot, buildCell(0x902, 0, 0, 500, 400, "A1")...),
				buildCell(0x903, 500, 0, 1000, 400, "A2")...,
			),
			buildCell(0x904, 0, 400, 500, 800, "B1")...,
		),
		buildCell(0x905, 500, 400, 1000, 800, "B2")...,
	))

	tableShape, ok := recoverLegacyPptTableShapeFromGroupContainer(group[8:], legacyPptGroupFrame{}, nil, nil, nil, 1)
	if !ok {
		t.Fatal("recoverLegacyPptTableShapeFromGroupContainer returned false")
	}
	if tableShape.OriginalShapeID != 0x901 {
		t.Fatalf("OriginalShapeID = %#x, want %#x", tableShape.OriginalShapeID, 0x901)
	}
}

func buildPptAtomRecord(instance, recordType uint16, payload []byte) []byte {
	record := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint16(record[0:2], instance<<4)
	binary.LittleEndian.PutUint16(record[2:4], recordType)
	binary.LittleEndian.PutUint32(record[4:8], uint32(len(payload)))
	copy(record[8:], payload)
	return record
}

func buildPptContainerRecord(instance, recordType, version uint16, payload []byte) []byte {
	record := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint16(record[0:2], instance<<4|version)
	binary.LittleEndian.PutUint16(record[2:4], recordType)
	binary.LittleEndian.PutUint32(record[4:8], uint32(len(payload)))
	copy(record[8:], payload)
	return record
}

func buildOfficeArtRecord(instance, recordType, version uint16, payload []byte) []byte {
	record := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint16(record[0:2], instance<<4|version)
	binary.LittleEndian.PutUint16(record[2:4], recordType)
	binary.LittleEndian.PutUint32(record[4:8], uint32(len(payload)))
	copy(record[8:], payload)
	return record
}

func officeArtSpPayload(shapeID uint32, flags uint32) []byte {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint32(payload[0:4], shapeID)
	binary.LittleEndian.PutUint32(payload[4:8], flags)
	return payload
}

func marshalLegacyUint16LE(value uint16) []byte {
	payload := make([]byte, 2)
	binary.LittleEndian.PutUint16(payload, value)
	return payload
}

func buildLegacyPptMasterTextLevelPayload(paragraph stpaPFEntry, run stpaCFRun) []byte {
	return append(encodeLegacyPptPFExceptionSpec(paragraph), encodeLegacyPptCFExceptionSpec(run)...)
}

func encodeLegacyPptPFExceptionSpec(entry stpaPFEntry) []byte {
	mask := uint32(0)
	payload := make([]byte, 4)
	appendWord := func(value uint16) {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, value)
		payload = append(payload, buf...)
	}
	if entry.BulletChar != 0 {
		mask |= 1 << 7
	}
	if entry.HasMarginLeft {
		mask |= 1 << 8
	}
	if entry.HasTextIndent {
		mask |= 1 << 10
	}
	if entry.Align != "" {
		mask |= 1 << 11
	}
	if entry.HasLineSpacing {
		mask |= 1 << 12
	}
	if entry.HasSpaceBefore || entry.SpaceBeforePct > 0 {
		mask |= 1 << 13
	}
	if entry.HasSpaceAfter || entry.SpaceAfterPct > 0 {
		mask |= 1 << 14
	}
	if entry.TextDirection != "" {
		mask |= 1 << 21
	}
	binary.LittleEndian.PutUint32(payload[:4], mask)
	if entry.BulletChar != 0 {
		appendWord(uint16(entry.BulletChar))
	}
	if entry.HasMarginLeft {
		appendWord(uint16(int16(legacyPptEMUToNearestMasterUnits(entry.MarginLeftEMU))))
	}
	if entry.HasTextIndent {
		appendWord(uint16(int16(legacyPptEMUToNearestMasterUnits(entry.TextIndentEMU))))
	}
	if entry.Align != "" {
		appendWord(legacyPptAlignValue(entry.Align))
	}
	if entry.HasLineSpacing {
		if entry.LineSpacingPt > 0 {
			appendWord(uint16(int16(-legacyPptHundredthPtToNearestMasterUnits(entry.LineSpacingPt))))
		} else {
			appendWord(uint16(int16(entry.LineSpacingPct / 1000)))
		}
	}
	if entry.HasSpaceBefore {
		appendWord(uint16(int16(-legacyPptHundredthPtToNearestMasterUnits(entry.SpaceBefore))))
	} else if entry.SpaceBeforePct > 0 {
		appendWord(uint16(int16(entry.SpaceBeforePct / 1000)))
	}
	if entry.HasSpaceAfter {
		appendWord(uint16(int16(-legacyPptHundredthPtToNearestMasterUnits(entry.SpaceAfter))))
	} else if entry.SpaceAfterPct > 0 {
		appendWord(uint16(int16(entry.SpaceAfterPct / 1000)))
	}
	if value := legacyPptParagraphDirectionValue(entry.TextDirection); value != 0 {
		appendWord(value)
	}
	return payload
}

func encodeLegacyPptCFExceptionSpec(run stpaCFRun) []byte {
	mask := uint32(0)
	payload := make([]byte, 4)
	appendWord := func(value uint16) {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, value)
		payload = append(payload, buf...)
	}
	appendColor := func(color string) {
		payload = append(payload, mustDecodeLegacyColor(color)...)
	}
	hasStyle := run.Bold != nil || run.Italic != nil || run.Underline != nil
	styleBits := uint16(0)
	if run.Bold != nil {
		mask |= 1 << 0
		if *run.Bold {
			styleBits |= 1 << 0
		}
	}
	if run.Italic != nil {
		mask |= 1 << 1
		if *run.Italic {
			styleBits |= 1 << 1
		}
	}
	if run.Underline != nil {
		mask |= 1 << 2
		if *run.Underline {
			styleBits |= 1 << 2
		}
	}
	if hasStyle {
		mask |= 1 << 10
	}
	if run.FontFamily != "" {
		mask |= 1 << 16
	}
	if run.FontSize > 0 {
		mask |= 1 << 17
	}
	if run.Color != "" {
		mask |= 1 << 18
	}
	binary.LittleEndian.PutUint32(payload[:4], mask)
	if hasStyle {
		appendWord(styleBits)
	}
	if run.FontFamily != "" {
		appendWord(0)
	}
	if run.FontSize > 0 {
		appendWord(uint16(run.FontSize / 100))
	}
	if run.Color != "" {
		appendColor(run.Color)
	}
	return payload
}

func encodeLegacyPptPFException(entry stpaPFEntry) []byte {
	mask := uint32(0)
	payload := make([]byte, 4)
	appendWord := func(value uint16) {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, value)
		payload = append(payload, buf...)
	}
	appendDWord := func(value uint32) {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, value)
		payload = append(payload, buf...)
	}
	if entry.BulletChar != 0 {
		mask |= 1 << 7
	}
	if entry.HasMarginLeft {
		mask |= 1 << 8
	}
	if entry.HasTextIndent {
		mask |= 1 << 9
	}
	if entry.Align != "" {
		mask |= 1 << 16
	}
	if entry.HasLineSpacing {
		mask |= 1 << 17
	}
	if entry.HasSpaceBefore || entry.SpaceBeforePct > 0 {
		mask |= 1 << 18
	}
	if entry.HasSpaceAfter || entry.SpaceAfterPct > 0 {
		mask |= 1 << 19
	}
	if entry.TextDirection != "" {
		mask |= 1 << 21
	}
	binary.LittleEndian.PutUint32(payload[:4], mask)
	if entry.BulletChar != 0 {
		appendWord(uint16(entry.BulletChar))
	}
	if entry.HasMarginLeft {
		appendDWord(uint32(int32(legacyPptEMUToNearestMasterUnits(entry.MarginLeftEMU))))
	}
	if entry.HasTextIndent {
		appendDWord(uint32(int32(legacyPptEMUToNearestMasterUnits(entry.TextIndentEMU))))
	}
	if entry.Align != "" {
		appendWord(legacyPptAlignValue(entry.Align))
	}
	if entry.HasLineSpacing {
		if entry.LineSpacingPt > 0 {
			appendWord(uint16(int16(-legacyPptHundredthPtToNearestMasterUnits(entry.LineSpacingPt))))
		} else {
			appendWord(uint16(int16(entry.LineSpacingPct / 1000)))
		}
	}
	if entry.HasSpaceBefore {
		appendWord(uint16(int16(-legacyPptHundredthPtToNearestMasterUnits(entry.SpaceBefore))))
	} else if entry.SpaceBeforePct > 0 {
		appendWord(uint16(int16(entry.SpaceBeforePct / 1000)))
	}
	if entry.HasSpaceAfter {
		appendWord(uint16(int16(-legacyPptHundredthPtToNearestMasterUnits(entry.SpaceAfter))))
	} else if entry.SpaceAfterPct > 0 {
		appendWord(uint16(int16(entry.SpaceAfterPct / 1000)))
	}
	if value := legacyPptParagraphDirectionValue(entry.TextDirection); value != 0 {
		appendWord(value)
	}
	return payload
}

func legacyPptParagraphDirectionValue(direction string) uint16 {
	switch direction {
	case "ltr":
		return 1
	case "rtl":
		return 2
	default:
		return 0
	}
}

func TestMergeLegacyPptMasterParagraphInheritsTextDirection(t *testing.T) {
	paragraph := &presentationir.Paragraph{Runs: []*presentationir.Run{{Text: "Hello"}}}
	master := legacyPptMasterTextLevel{paragraph: stpaPFEntry{TextDirection: "rtl"}}
	merged := mergeLegacyPptMasterParagraph(paragraph, master, false)
	if merged == nil {
		t.Fatal("expected merged paragraph")
	}
	if merged.TextDirection != "rtl" {
		t.Fatalf("TextDirection = %q, want %q", merged.TextDirection, "rtl")
	}
}

func encodeLegacyPptCFException(run stpaCFRun) []byte {
	mask := uint32(0)
	payload := make([]byte, 4)
	appendWord := func(value uint16) {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, value)
		payload = append(payload, buf...)
	}
	appendColor := func(color string) {
		payload = append(payload, mustDecodeLegacyColor(color)...)
	}
	hasStyle := run.Bold != nil || run.Italic != nil || run.Underline != nil || run.Strike != nil
	styleBits := uint16(0)
	if run.Bold != nil {
		mask |= 1 << 0
		if *run.Bold {
			styleBits |= 1 << 0
		}
	}
	if run.Italic != nil {
		mask |= 1 << 1
		if *run.Italic {
			styleBits |= 1 << 1
		}
	}
	if run.Underline != nil {
		mask |= 1 << 2
		if *run.Underline {
			styleBits |= 1 << 2
		}
	}
	if run.Strike != nil {
		mask |= 1 << 7
		if *run.Strike {
			styleBits |= 1 << 7
		}
	}
	if run.FontFamily != "" {
		mask |= 1 << 9
	}
	if run.FontSize > 0 {
		mask |= 1 << 10
	}
	if run.Color != "" {
		mask |= 1 << 11
	}
	binary.LittleEndian.PutUint32(payload[:4], mask)
	if hasStyle {
		appendWord(styleBits)
	}
	if run.FontFamily != "" {
		appendWord(0)
	}
	if run.FontSize > 0 {
		appendWord(uint16(run.FontSize))
	}
	if run.Color != "" {
		appendColor(run.Color)
	}
	return payload
}

func encodeLegacyPptCFException9() []byte {
	return []byte{0x00, 0x00, 0x00, 0x00}
}

func encodeLegacyPptPFException9AutoNum(scheme uint16) []byte {
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint32(payload[:4], 1<<21|1<<22)
	flag := make([]byte, 2)
	binary.LittleEndian.PutUint16(flag, 1)
	payload = append(payload, flag...)
	binaryScheme := make([]byte, 2)
	binary.LittleEndian.PutUint16(binaryScheme, scheme)
	payload = append(payload, binaryScheme...)
	return payload
}

func encodeLegacyPptCFException10(fontRef uint16) []byte {
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint32(payload[:4], 1<<16)
	ref := make([]byte, 2)
	binary.LittleEndian.PutUint16(ref, fontRef)
	return append(payload, ref...)
}

func encodeLegacyUTF16(value string) []byte {
	payload := make([]byte, 0, len(value)*2)
	for _, r := range value {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(r))
		payload = append(payload, buf...)
	}
	return payload
}

func buildLegacyFontEntityPayload(name string) []byte {
	payload := make([]byte, 0x44)
	encoded := encodeLegacyUTF16(name)
	if len(encoded) > 64 {
		encoded = encoded[:64]
	}
	copy(payload, encoded)
	return payload
}

func mustDecodeLegacyColor(color string) []byte {
	if len(color) != 6 {
		panic("legacy test color must be 6 hex digits")
	}
	r := mustDecodeLegacyHexByte(color[0], color[1])
	g := mustDecodeLegacyHexByte(color[2], color[3])
	b := mustDecodeLegacyHexByte(color[4], color[5])
	return []byte{r, g, b, 0xFE}
}

func mustDecodeLegacyHexByte(high, low byte) byte {
	return mustDecodeLegacyHexNibble(high)<<4 | mustDecodeLegacyHexNibble(low)
}

func mustDecodeLegacyHexNibble(value byte) byte {
	switch {
	case value >= '0' && value <= '9':
		return value - '0'
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10
	default:
		panic("invalid hex digit")
	}
}

func legacyPptAlignValue(align string) uint16 {
	switch align {
	case "l":
		return 0
	case "ctr":
		return 1
	case "r":
		return 2
	case "just":
		return 3
	default:
		return 0
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func encodeLegacyPptFontEntityPayload(name string) []byte {
	runes := []rune(name)
	payload := make([]byte, (len(runes)+1)*2)
	for index, r := range runes {
		binary.LittleEndian.PutUint16(payload[index*2:index*2+2], uint16(r))
	}
	return payload
}

func TestParseLegacyPptPlaceholderAtomObservedValues(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		want        presentationir.PlaceholderKind
		wantIdx     int
		wantMatched bool
	}{
		// Bytes 0-3: placementId (shape index). Byte 4: placeholderId (MS-PPT enum).
		{name: "title PT_Title=0x0D", payload: []byte{0, 0, 0, 0, 0x0D, 0, 0, 0}, want: presentationir.PlaceholderTitle, wantMatched: true},
		{name: "body PT_Body=0x0E", payload: []byte{1, 0, 0, 0, 0x0E, 0, 0, 0}, want: presentationir.PlaceholderBody, wantMatched: true},
		{name: "center title PT_CenterTitle=0x0F", payload: []byte{0, 0, 0, 0, 0x0F, 0, 0, 0}, want: presentationir.PlaceholderTitle, wantMatched: true},
		{name: "subtitle PT_SubTitle=0x10", payload: []byte{2, 0, 0, 0, 0x10, 0, 0, 0}, want: presentationir.PlaceholderSubtitle, wantMatched: true},
		{name: "vertical title PT_VerticalTitle=0x11", payload: []byte{0, 0, 0, 0, 0x11, 0, 0, 0}, want: presentationir.PlaceholderTitle, wantMatched: true},
		{name: "vertical body PT_VerticalBody=0x12", payload: []byte{1, 0, 0, 0, 0x12, 0, 0, 0}, want: presentationir.PlaceholderBody, wantMatched: true},
		{name: "date PT_MasterDate=0x07", payload: []byte{3, 0, 0, 0, 0x07, 0, 0, 0}, want: presentationir.PlaceholderDateTime, wantMatched: true},
		{name: "slide number PT_MasterSlideNumber=0x08", payload: []byte{4, 0, 0, 0, 0x08, 0, 0, 0}, want: presentationir.PlaceholderSlideNumber, wantMatched: true},
		{name: "footer PT_MasterFooter=0x09", payload: []byte{5, 0, 0, 0, 0x09, 0, 0, 0}, want: presentationir.PlaceholderFooter, wantMatched: true},
		{name: "notes body PT_NotesBody=0x0C", payload: []byte{1, 0, 0, 0, 0x0C, 0, 0, 0}, want: presentationir.PlaceholderBody, wantMatched: true},
		{name: "none sentinel placementId=0xFFFFFFFF", payload: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0}, want: presentationir.PlaceholderNone, wantMatched: false},
		{name: "unsupported PT_Object=0x13", payload: []byte{2, 0, 0, 0, 0x13, 0, 0, 0}, want: presentationir.PlaceholderNone, wantMatched: false},
		{name: "too short no placeholderId byte", payload: []byte{0, 0, 0, 0}, want: presentationir.PlaceholderNone, wantMatched: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, idx, matched := parseLegacyPptPlaceholderAtom(test.payload)
			wantIdx := test.wantIdx
			if test.wantMatched && len(test.payload) >= 4 {
				wantIdx = int(binary.LittleEndian.Uint32(test.payload[:4]))
			}
			if got != test.want || idx != wantIdx || matched != test.wantMatched {
				t.Fatalf("parseLegacyPptPlaceholderAtom(%v) = (%v, idx=%d, %v), want (%v, idx=%d, %v)", test.payload, got, idx, matched, test.want, wantIdx, test.wantMatched)
			}
		})
	}
}

func TestParsePptDoesNotInventPlaceholderFromTextTypeWhenPlaceholderAtomIsUnknown(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "apache-SampleShow.ppt")
	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if len(pres.Slides) < 1 {
		t.Fatalf("slides = %d, want at least 1", len(pres.Slides))
	}
	slide := pres.Slides[0]
	if len(slide.Shapes) < 2 {
		t.Fatalf("slide shapes = %d, want at least 2", len(slide.Shapes))
	}
	var titleShape, subtitleShape *presentationir.Shape
	for _, shape := range slide.Shapes {
		if shape == nil || shape.Kind != presentationir.ShapeText || len(shape.Paragraphs) == 0 {
			continue
		}
		text := strings.TrimSpace(paragraphText(shape.Paragraphs[0]))
		normalized := strings.Join(strings.Fields(text), " ")
		switch normalized {
		case "Title of the first slide":
			titleShape = shape
		case "Subtitle of the first slide":
			subtitleShape = shape
		}
	}
	if titleShape == nil {
		t.Fatal("authored title text shape not found")
	}
	if titleShape.Placeholder != presentationir.PlaceholderNone {
		t.Fatalf("authored title placeholder = %v, want none without recognized placeholder atom", titleShape.Placeholder)
	}
	if subtitleShape == nil {
		t.Fatal("authored subtitle text shape not found")
	}
	if subtitleShape.Placeholder != presentationir.PlaceholderNone {
		t.Fatalf("authored subtitle placeholder = %v, want none without recognized placeholder atom", subtitleShape.Placeholder)
	}
	if got := paragraphText(subtitleShape.Paragraphs[0]); got != "Subtitle of the first slide" {
		t.Fatalf("subtitle text = %q, want %q", got, "Subtitle of the first slide")
	}
}

func TestPartitionLegacySlideParagraphsSeparatesFooterPlaceholder(t *testing.T) {
	paragraphs := buildTextParagraphs("Quarterly Results\n- Revenue up\n- Costs down\nSlide 7")
	title, subtitle, body, footer := partitionLegacySlideParagraphs(paragraphs, "sample.ppt")
	if len(title) != 1 || paragraphText(title[0]) != "Quarterly Results" {
		t.Fatalf("title = %#v", title)
	}
	if len(subtitle) != 0 {
		t.Fatalf("subtitle = %#v, want none", subtitle)
	}
	if len(body) != 2 {
		t.Fatalf("body len = %d, want 2", len(body))
	}
	if len(footer) != 1 || paragraphText(footer[0]) != "Slide 7" {
		t.Fatalf("footer = %#v", footer)
	}
}

func TestPartitionLegacySlideParagraphsKeepsWeakTwoLineTailOutOfFooter(t *testing.T) {
	paragraphs := buildTextParagraphs("Agenda\nOverview")
	title, subtitle, body, footer := partitionLegacySlideParagraphs(paragraphs, "sample.ppt")
	if len(title) != 1 || paragraphText(title[0]) != "Agenda" {
		t.Fatalf("title = %#v", title)
	}
	if len(subtitle) != 1 || paragraphText(subtitle[0]) != "Overview" {
		t.Fatalf("subtitle = %#v", subtitle)
	}
	if len(body) != 0 {
		t.Fatalf("body = %#v, want none", body)
	}
	if len(footer) != 0 {
		t.Fatalf("footer = %#v, want none", footer)
	}
}

func TestBuildLegacySlideShapesCreatesSlideNumberFooterShape(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "Quarterly Results\n- Revenue up\n- Costs down\nSlide 7", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 3 {
		t.Fatalf("shapes len = %d, want 3", len(shapes))
	}
	footer := shapes[2]
	if footer.Placeholder != presentationir.PlaceholderSlideNumber {
		t.Fatalf("footer placeholder = %v, want slide number", footer.Placeholder)
	}
	if len(footer.Paragraphs) != 1 || paragraphText(footer.Paragraphs[0]) != "Slide 7" {
		t.Fatalf("footer text = %#v", footer.Paragraphs)
	}
}

func TestBuildLegacySlideShapesCreatesSeparateFooterPlaceholderShapes(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "Quarterly Results\n- Revenue up\n04/23/2026\nSlide 7", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 4 {
		t.Fatalf("shapes len = %d, want 4", len(shapes))
	}
	if shapes[2].Placeholder != presentationir.PlaceholderDateTime {
		t.Fatalf("shape[2] placeholder = %v, want datetime", shapes[2].Placeholder)
	}
	if shapes[2].Bounds != legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime) {
		t.Fatalf("shape[2] bounds = %+v, want %+v", shapes[2].Bounds, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime))
	}
	if shapes[3].Placeholder != presentationir.PlaceholderSlideNumber {
		t.Fatalf("shape[3] placeholder = %v, want slide number", shapes[3].Placeholder)
	}
	if shapes[3].Bounds != legacyFooterBoundsForPlaceholder(presentationir.PlaceholderSlideNumber) {
		t.Fatalf("shape[3] bounds = %+v, want %+v", shapes[3].Bounds, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderSlideNumber))
	}
}

func TestBuildLegacySlideShapesTreatsHyphenatedDateAsDateTimeFooter(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "Quarterly Results\n2026-04-24", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 2 {
		t.Fatalf("shapes len = %d, want 2", len(shapes))
	}
	if shapes[1].Placeholder != presentationir.PlaceholderDateTime {
		t.Fatalf("shape[1] placeholder = %v, want datetime", shapes[1].Placeholder)
	}
	if shapes[1].Bounds != legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime) {
		t.Fatalf("shape[1] bounds = %+v, want %+v", shapes[1].Bounds, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime))
	}
}

func TestBuildLegacySlideShapesTreatsMonthNameDateAsDateTimeFooter(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "Quarterly Results\nApril 24, 2026", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 2 {
		t.Fatalf("shapes len = %d, want 2", len(shapes))
	}
	if shapes[1].Placeholder != presentationir.PlaceholderDateTime {
		t.Fatalf("shape[1] placeholder = %v, want datetime", shapes[1].Placeholder)
	}
	if shapes[1].Bounds != legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime) {
		t.Fatalf("shape[1] bounds = %+v, want %+v", shapes[1].Bounds, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime))
	}
}

func TestBuildLegacySlideShapesTreatsMeridiemTimeAsDateTimeFooter(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "Quarterly Results\n9 AM", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 2 {
		t.Fatalf("shapes len = %d, want 2", len(shapes))
	}
	if shapes[1].Placeholder != presentationir.PlaceholderDateTime {
		t.Fatalf("shape[1] placeholder = %v, want datetime", shapes[1].Placeholder)
	}
	if shapes[1].Bounds != legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime) {
		t.Fatalf("shape[1] bounds = %+v, want %+v", shapes[1].Bounds, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime))
	}
}

func TestBuildLegacySlideShapesDoesNotTreatWordContainingAMAsTimeFooter(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "Quarterly Results\nTeam 7", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 2 {
		t.Fatalf("shapes len = %d, want 2", len(shapes))
	}
	if shapes[1].Placeholder != presentationir.PlaceholderSubtitle {
		t.Fatalf("shape[1] placeholder = %v, want subtitle", shapes[1].Placeholder)
	}
}

func TestBuildPresentationPromotesLegacyFooterPlaceholders(t *testing.T) {
	pres := buildPresentation([]legacySlideText{
		{Fragments: []legacySlideFragment{{Text: "Quarterly Results\n- Revenue up\n04/23/2026", TextType: pptTextTypeUnknown}}},
		{Fragments: []legacySlideFragment{{Text: "Roadmap\n- Next steps\nSlide 7", TextType: pptTextTypeUnknown}}},
	}, "sample.ppt")
	if len(pres.Slides) != 2 {
		t.Fatalf("slides len = %d, want 2", len(pres.Slides))
	}
	if pres.Slides[0].DateTime == nil || paragraphText(pres.Slides[0].DateTime.Paragraphs[0]) != "04/23/2026" {
		t.Fatalf("slide 1 datetime = %#v", pres.Slides[0].DateTime)
	}
	if pres.Slides[1].SlideNumber == nil || paragraphText(pres.Slides[1].SlideNumber.Paragraphs[0]) != "Slide 7" {
		t.Fatalf("slide 2 slide number = %#v", pres.Slides[1].SlideNumber)
	}
	if got := pres.Slides[1].SlideNumber.Paragraphs[0].Runs[0].FieldKind; got != "slidenum" {
		t.Fatalf("slide 2 slide number FieldKind = %q, want slidenum", got)
	}
	for index, slide := range pres.Slides {
		for _, shape := range slide.Shapes {
			if shape.Placeholder == presentationir.PlaceholderDateTime || shape.Placeholder == presentationir.PlaceholderSlideNumber || shape.Placeholder == presentationir.PlaceholderFooter {
				t.Fatalf("slide %d still contains footer placeholder in content shapes: %+v", index+1, shape)
			}
		}
	}
}

func TestBuildPresentationPromotesMultipleLegacyFooterPlaceholders(t *testing.T) {
	pres := buildPresentation([]legacySlideText{
		{Fragments: []legacySlideFragment{{Text: "Quarterly Results\n- Revenue up\n04/23/2026\nSlide 7", TextType: pptTextTypeUnknown}}},
	}, "sample.ppt")
	if len(pres.Slides) != 1 {
		t.Fatalf("slides len = %d, want 1", len(pres.Slides))
	}
	if pres.Slides[0].DateTime == nil || paragraphText(pres.Slides[0].DateTime.Paragraphs[0]) != "04/23/2026" {
		t.Fatalf("datetime = %#v", pres.Slides[0].DateTime)
	}
	if pres.Slides[0].SlideNumber == nil || paragraphText(pres.Slides[0].SlideNumber.Paragraphs[0]) != "Slide 7" {
		t.Fatalf("slide number = %#v", pres.Slides[0].SlideNumber)
	}
}

func TestBuildPresentationPromotesLegacyHeaderPlaceholder(t *testing.T) {
	pres := buildPresentationWithMasterComposition([]legacySlideText{
		{
			PersistID: 1,
			Fragments: []legacySlideFragment{{Text: "Quarterly Results", TextType: pptTextTypeTitle}},
		},
	}, nil, map[int]legacyPptHeaderFooterInfo{
		1: {
			flags:      legacyPptHFFlagHeader,
			headerText: "Agenda",
		},
	}, legacyPptHeaderFooterInfo{}, "sample.ppt", nil, nil, nil, "", nil)
	if len(pres.Slides) != 1 {
		t.Fatalf("slides len = %d, want 1", len(pres.Slides))
	}
	if pres.Slides[0].Header == nil {
		t.Fatal("header = nil, want promoted header shape")
	}
	if got := paragraphText(pres.Slides[0].Header.Paragraphs[0]); got != "Agenda" {
		t.Fatalf("header text = %q, want Agenda", got)
	}
	if pres.Slides[0].Header.Name != "hdr" {
		t.Fatalf("header name = %q, want hdr", pres.Slides[0].Header.Name)
	}
	for _, shape := range pres.Slides[0].Shapes {
		if shape.Name == "hdr" {
			t.Fatalf("header shape leaked into content shapes: %+v", shape)
		}
	}
}

func TestBuildPresentationSlideHeaderFooterOverrideCanDisableDocumentDefaults(t *testing.T) {
	pres := buildPresentationWithMasterComposition([]legacySlideText{
		{
			PersistID: 1,
			Fragments: []legacySlideFragment{{Text: "Quarterly Results", TextType: pptTextTypeTitle}},
		},
	}, nil, map[int]legacyPptHeaderFooterInfo{
		1: {
			hasAtom: true,
			flags:   0,
		},
	}, legacyPptHeaderFooterInfo{
		hasAtom:    true,
		flags:      legacyPptHFFlagFooter,
		footerText: "Default footer",
	}, "sample.ppt", nil, nil, nil, "", nil)
	if len(pres.Slides) != 1 {
		t.Fatalf("slides len = %d, want 1", len(pres.Slides))
	}
	if pres.Slides[0].Footer != nil {
		t.Fatalf("footer = %#v, want nil (slide override disabled default footer)", pres.Slides[0].Footer)
	}
}

func TestParseLegacyPptHeaderFooterInfoTracksHeadersFootersAtomPresence(t *testing.T) {
	payload := buildPptAtomRecord(0, pptRTHeadersFootersAtom, append(marshalLegacyUint16LE(0), marshalLegacyUint16LE(0)...))
	info := parseLegacyPptHeaderFooterInfo(payload)
	if !info.hasAtom {
		t.Fatal("hasAtom = false, want true when HeadersFootersAtom is present")
	}
	if info.flags != 0 {
		t.Fatalf("flags = 0x%04x, want 0x0000", info.flags)
	}
	if info.hasRenderableContent() {
		t.Fatalf("hasRenderableContent = true, want false for explicit zero-flag atom without text")
	}
}

func TestDateEnabledGatesOnlyOnFHasDateBit(t *testing.T) {
	// fHasTodayDate set but fHasDate not → date must be disabled per POI isDateTimeVisible()
	info := legacyPptHeaderFooterInfo{flags: legacyPptHFFlagTodayDate}
	if info.dateEnabled() {
		t.Error("dateEnabled() = true, want false: fHasTodayDate alone is not the visibility gate")
	}
	// fHasUserDate set but fHasDate not → date must be disabled
	info = legacyPptHeaderFooterInfo{flags: legacyPptHFFlagUserDate}
	if info.dateEnabled() {
		t.Error("dateEnabled() = true, want false: fHasUserDate alone is not the visibility gate")
	}
	// fHasDate set → date enabled regardless of sub-type bits
	info = legacyPptHeaderFooterInfo{flags: legacyPptHFFlagDateVisible}
	if !info.dateEnabled() {
		t.Error("dateEnabled() = false, want true when fHasDate (fHasDate) is set")
	}
}

func TestMergedWithClearsDateTextWhenFHasDateOff(t *testing.T) {
	// Defaults have date enabled with a user-provided date string.
	defaults := legacyPptHeaderFooterInfo{
		hasAtom:  true,
		flags:    legacyPptHFFlagDateVisible | legacyPptHFFlagUserDate,
		dateText: "April 2026",
	}
	// Override has fHasUserDate but not fHasDate → date slot should be cleared.
	override := legacyPptHeaderFooterInfo{
		hasAtom: true,
		flags:   legacyPptHFFlagUserDate,
	}
	merged := defaults.mergedWith(override)
	if merged.dateText != "" {
		t.Errorf("dateText = %q, want empty: fHasDate is off in override so date text must be cleared", merged.dateText)
	}
	if merged.dateEnabled() {
		t.Error("dateEnabled() = true, want false: fHasDate is off in the merged result")
	}
}

func TestSynthesizeLegacyHeaderFooterCreatesTodayDateFieldShape(t *testing.T) {
	// fHasDate + fHasTodayDate set with no dateText → DateTime shape with FieldKind "date"
	pres := buildPresentationWithMasterComposition([]legacySlideText{
		{
			PersistID: 1,
			Fragments: []legacySlideFragment{{Text: "Slide", TextType: pptTextTypeTitle}},
		},
	}, nil, nil, legacyPptHeaderFooterInfo{
		hasAtom: true,
		flags:   legacyPptHFFlagDateVisible | legacyPptHFFlagTodayDate,
	}, "sample.ppt", nil, nil, nil, "", nil)
	if len(pres.Slides) != 1 {
		t.Fatalf("slides len = %d, want 1", len(pres.Slides))
	}
	dt := pres.Slides[0].DateTime
	if dt == nil {
		t.Fatal("DateTime = nil, want a shape when fHasTodayDate is set")
	}
	if len(dt.Paragraphs) == 0 || len(dt.Paragraphs[0].Runs) == 0 {
		t.Fatal("DateTime shape has no runs")
	}
	if got := dt.Paragraphs[0].Runs[0].FieldKind; got != "date" {
		t.Errorf("FieldKind = %q, want %q", got, "date")
	}
}

func TestBuildLegacySlideShapesUsesTitlePlaceholderForSingleLineSlide(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "Quarterly Results", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 1 {
		t.Fatalf("shapes len = %d, want 1", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderTitle {
		t.Fatalf("placeholder = %v, want title", shapes[0].Placeholder)
	}
	if paragraphText(shapes[0].Paragraphs[0]) != "Quarterly Results" {
		t.Fatalf("title text = %#v", shapes[0].Paragraphs)
	}
}

func TestBuildLegacySlideShapesDoesNotTreatSentenceAsTitle(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "Please review the updated forecast by Friday.", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 1 {
		t.Fatalf("shapes len = %d, want 1", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderBody {
		t.Fatalf("placeholder = %v, want body", shapes[0].Placeholder)
	}
	if shapes[0].Bounds != legacyBodyBounds(false) {
		t.Fatalf("bounds = %+v, want %+v", shapes[0].Bounds, legacyBodyBounds(false))
	}
}

func TestBuildLegacySlideShapesTreatsAlphaBulletAsBodyNotSubtitle(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "Agenda\na. First topic", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 2 {
		t.Fatalf("shapes len = %d, want 2", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderTitle {
		t.Fatalf("first placeholder = %v, want title", shapes[0].Placeholder)
	}
	if shapes[1].Placeholder != presentationir.PlaceholderBody {
		t.Fatalf("second placeholder = %v, want body", shapes[1].Placeholder)
	}
	if paragraphText(shapes[1].Paragraphs[0]) != "a. First topic" {
		t.Fatalf("body text = %#v", shapes[1].Paragraphs)
	}
}

func TestBuildLegacySlideShapesDoesNotTreatSentenceAsSubtitle(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "Agenda\nPlease review the updated forecast by Friday.", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 2 {
		t.Fatalf("shapes len = %d, want 2", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderTitle {
		t.Fatalf("first placeholder = %v, want title", shapes[0].Placeholder)
	}
	if shapes[1].Placeholder != presentationir.PlaceholderBody {
		t.Fatalf("second placeholder = %v, want body", shapes[1].Placeholder)
	}
	if shapes[1].Bounds != legacyBodyBounds(true) {
		t.Fatalf("bounds = %+v, want %+v", shapes[1].Bounds, legacyBodyBounds(true))
	}
}

func TestBuildLegacySlideShapesDoesNotSplitSentenceLeadLineIntoTitle(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "Please review the updated forecast by Friday.\n- Revenue up", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 1 {
		t.Fatalf("shapes len = %d, want 1", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderBody {
		t.Fatalf("placeholder = %v, want body", shapes[0].Placeholder)
	}
	if len(shapes[0].Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(shapes[0].Paragraphs))
	}
}

func TestBuildLegacySlideShapesUsesBodyBoundsForBodyOnlyBulletSlide(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "- Revenue up\n- Costs down", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 1 {
		t.Fatalf("shapes len = %d, want 1", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderBody {
		t.Fatalf("placeholder = %v, want body", shapes[0].Placeholder)
	}
	if shapes[0].Bounds != legacyBodyBounds(false) {
		t.Fatalf("bounds = %+v, want %+v", shapes[0].Bounds, legacyBodyBounds(false))
	}
}

func TestBuildLegacySlideShapesUsesBodyBoundsForSingleBulletLine(t *testing.T) {
	shapes := buildLegacySlideShapes(0, legacySlideText{Fragments: []legacySlideFragment{{Text: "- Revenue up", TextType: pptTextTypeUnknown}}}, "sample.ppt")
	if len(shapes) != 1 {
		t.Fatalf("shapes len = %d, want 1", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderBody {
		t.Fatalf("placeholder = %v, want body", shapes[0].Placeholder)
	}
	if shapes[0].Bounds != legacyBodyBounds(false) {
		t.Fatalf("bounds = %+v, want %+v", shapes[0].Bounds, legacyBodyBounds(false))
	}
}

func TestLooksLikeLegacyBulletRecognizesExtendedGlyphs(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"▪ item one", true},
		{"‣ triangle bullet", true},
		{"› single angle", true},
		{"plain text no bullet", false},
		{"Normal sentence.", false},
	}
	for _, test := range tests {
		if got := looksLikeLegacyBullet(test.text); got != test.want {
			t.Fatalf("looksLikeLegacyBullet(%q) = %v, want %v", test.text, got, test.want)
		}
	}
}

func TestAttachNotesByIndexPopulatesNoteFragments(t *testing.T) {
	slides := []legacySlideText{
		{PersistID: 1, Fragments: []legacySlideFragment{{Text: "Title One", TextType: pptTextTypeTitle}}},
		{PersistID: 2, Fragments: []legacySlideFragment{{Text: "Title Two", TextType: pptTextTypeTitle}}},
	}
	notes := []legacySlideText{
		{Fragments: []legacySlideFragment{{Text: "Speaker note for slide 1", TextType: pptTextTypeNotes}}},
		{Fragments: []legacySlideFragment{{Text: "Speaker note for slide 2", TextType: pptTextTypeNotes}}},
	}
	result := attachNotesByIndex(slides, notes)
	if len(result) != 2 {
		t.Fatalf("slides len = %d, want 2", len(result))
	}
	notesFrags0 := noteFragments(result[0])
	if len(notesFrags0) != 1 || notesFrags0[0].Text != "Speaker note for slide 1" {
		t.Fatalf("slide[0] notes = %#v", notesFrags0)
	}
	notesFrags1 := noteFragments(result[1])
	if len(notesFrags1) != 1 || notesFrags1[0].Text != "Speaker note for slide 2" {
		t.Fatalf("slide[1] notes = %#v", notesFrags1)
	}
}

func TestAttachNotesByIndexPromotesUnknownTypeToNotes(t *testing.T) {
	slides := []legacySlideText{
		{Fragments: []legacySlideFragment{{Text: "Slide Title", TextType: pptTextTypeTitle}}},
	}
	notes := []legacySlideText{
		{Fragments: []legacySlideFragment{{Text: "Note text", TextType: pptTextTypeUnknown}}},
	}
	result := attachNotesByIndex(slides, notes)
	frags := noteFragments(result[0])
	if len(frags) != 1 || frags[0].Text != "Note text" {
		t.Fatalf("notes fragments = %#v", frags)
	}
}

func TestAttachNotesByIndexDedupsDuplicateNoteText(t *testing.T) {
	slides := []legacySlideText{
		{Fragments: []legacySlideFragment{{Text: "Slide Title", TextType: pptTextTypeTitle}, {Text: "Existing note", TextType: pptTextTypeNotes}}},
	}
	notes := []legacySlideText{
		{Fragments: []legacySlideFragment{{Text: "Existing note", TextType: pptTextTypeNotes}}},
	}
	result := attachNotesByIndex(slides, notes)
	frags := noteFragments(result[0])
	if len(frags) != 1 {
		t.Fatalf("expected 1 notes fragment after dedup, got %d: %#v", len(frags), frags)
	}
}

func TestAttachNotesByIndexIgnoresExcessNotes(t *testing.T) {
	slides := []legacySlideText{
		{Fragments: []legacySlideFragment{{Text: "Slide Title", TextType: pptTextTypeTitle}}},
	}
	notes := []legacySlideText{
		{Fragments: []legacySlideFragment{{Text: "Note 1", TextType: pptTextTypeNotes}}},
		{Fragments: []legacySlideFragment{{Text: "Note 2 — no matching slide", TextType: pptTextTypeNotes}}},
	}
	result := attachNotesByIndex(slides, notes)
	if len(result) != 1 {
		t.Fatalf("slides len = %d, want 1", len(result))
	}
	frags := noteFragments(result[0])
	if len(frags) != 1 || frags[0].Text != "Note 1" {
		t.Fatalf("notes fragments = %#v", frags)
	}
}

func TestSelectLegacyPptNotesSlideListInstancePrefersActualNotesPersist(t *testing.T) {
	masterList := buildPptContainerRecord(1, pptRTSlideListWithText, 0x0F,
		buildPptAtomRecord(0, pptRTSlidePersistAtom, marshalUint32LE(3)),
	)
	notesList := buildPptContainerRecord(2, pptRTSlideListWithText, 0x0F,
		buildPptAtomRecord(0, pptRTSlidePersistAtom, marshalUint32LE(17)),
	)
	document := buildPptContainerRecord(0, pptRTDocument, 0x0F, append(masterList, notesList...))
	masterPersist := buildPptContainerRecord(0, 0x03F8, 0x0F, nil)
	notesPersist := buildPptContainerRecord(0, pptRTNotes, 0x0F, nil)
	stream := append(append(document, masterPersist...), notesPersist...)
	persistDirectory := map[int]int{
		3:  len(document),
		17: len(document) + len(masterPersist),
	}
	if got := selectLegacyPptNotesSlideListInstance(stream, 0, persistDirectory); got != 2 {
		t.Fatalf("selectLegacyPptNotesSlideListInstance() = %d, want 2", got)
	}
}

func TestExtractSlideTextsFromDocumentMergesNotesSlideListsByInstance(t *testing.T) {
	slidesList := buildPptContainerRecord(0, pptRTSlideListWithText, 0x0F,
		append(
			buildPptAtomRecord(0, pptRTSlidePersistAtom, marshalUint32LE(1)),
			buildPptAtomRecord(0, pptRTSlidePersistAtom, marshalUint32LE(2))...,
		),
	)

	notesListPart1 := buildPptContainerRecord(2, pptRTSlideListWithText, 0x0F,
		append(
			buildPptAtomRecord(0, pptRTSlidePersistAtom, marshalUint32LE(101)),
			append(
				buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(uint32(pptTextTypeNotes))),
				buildPptAtomRecord(0, pptRTTextBytesAtom, []byte("Note one"))...,
			)...,
		),
	)
	notesListPart2 := buildPptContainerRecord(2, pptRTSlideListWithText, 0x0F,
		append(
			buildPptAtomRecord(0, pptRTSlidePersistAtom, marshalUint32LE(102)),
			append(
				buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(uint32(pptTextTypeNotes))),
				buildPptAtomRecord(0, pptRTTextBytesAtom, []byte("Note two"))...,
			)...,
		),
	)

	document := buildPptContainerRecord(0, pptRTDocument, 0x0F,
		append(append(slidesList, notesListPart1...), notesListPart2...),
	)

	slides := extractSlideTextsFromDocument(document, 0, 2, nil, nil, nil, nil)
	if len(slides) != 2 {
		t.Fatalf("slides len = %d, want 2", len(slides))
	}

	noteFrags0 := noteFragments(slides[0])
	if len(noteFrags0) != 1 || noteFrags0[0].Text != "Note one" {
		t.Fatalf("slides[0] notes = %#v, want one note fragment 'Note one'", noteFrags0)
	}
	noteFrags1 := noteFragments(slides[1])
	if len(noteFrags1) != 1 || noteFrags1[0].Text != "Note two" {
		t.Fatalf("slides[1] notes = %#v, want one note fragment 'Note two'", noteFrags1)
	}
}

func TestExtractLegacyNotesParagraphsReturnsNotesFragmentParagraphs(t *testing.T) {
	slide := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "Body text", TextType: pptTextTypeBody},
			{Text: "This is the speaker note", TextType: pptTextTypeNotes},
		},
	}
	paras := extractLegacyNotesParagraphs(slide)
	if len(paras) == 0 {
		t.Fatal("expected paragraphs from notes fragment, got none")
	}
	text := paragraphText(paras[0])
	if text != "This is the speaker note" {
		t.Fatalf("notes para text = %q", text)
	}
}

func TestBuildPresentationEmitsNotesSlide(t *testing.T) {
	slides := []legacySlideText{
		{
			PersistID: 1,
			Fragments: []legacySlideFragment{
				{Text: "Slide Title", TextType: pptTextTypeTitle},
				{Text: "These are the speaker notes", TextType: pptTextTypeNotes},
			},
		},
	}
	pres := buildPresentation(slides, "sample.ppt")
	if len(pres.Slides) != 1 {
		t.Fatalf("slides len = %d, want 1", len(pres.Slides))
	}
	slide := pres.Slides[0]
	if slide.Notes == nil {
		t.Fatal("notes = nil, want NotesSlide")
	}
	if len(slide.Notes.Shapes) == 0 {
		t.Fatal("notes shapes is empty")
	}
	notesShape := slide.Notes.Shapes[0]
	if len(notesShape.Paragraphs) == 0 {
		t.Fatal("notes shape has no paragraphs")
	}
	noteText := paragraphText(notesShape.Paragraphs[0])
	if noteText != "These are the speaker notes" {
		t.Fatalf("notes text = %q", noteText)
	}
}

func TestBuildPresentationEmitsNotesSlideForHeaderFooterMetadata(t *testing.T) {
	pres := buildPresentation([]legacySlideText{{
		PersistID: 1,
		NotesHeaderFooter: legacyPptHeaderFooterInfo{
			flags:      legacyPptHFFlagHeader | legacyPptHFFlagFooter,
			headerText: "Notes Header",
			footerText: "Notes Footer",
		},
	}}, "sample.ppt")
	if len(pres.Slides) != 1 {
		t.Fatalf("slides len = %d, want 1", len(pres.Slides))
	}
	if pres.Slides[0].Notes == nil {
		t.Fatal("notes = nil, want NotesSlide")
	}
	if len(pres.Slides[0].Notes.Shapes) != 2 {
		t.Fatalf("notes shapes len = %d, want 2", len(pres.Slides[0].Notes.Shapes))
	}
	if got := paragraphText(pres.Slides[0].Notes.Shapes[0].Paragraphs[0]); got != "Notes Header" {
		t.Fatalf("notes header text = %q, want Notes Header", got)
	}
	if got := paragraphText(pres.Slides[0].Notes.Shapes[1].Paragraphs[0]); got != "Notes Footer" {
		t.Fatalf("notes footer text = %q, want Notes Footer", got)
	}
}

func legacyPresentationContainsText(pres *presentationir.Presentation, want string) bool {
	if pres == nil {
		return false
	}
	for _, slide := range pres.Slides {
		if slide == nil {
			continue
		}
		for _, shape := range slide.Shapes {
			if legacyShapeContainsText(shape, want) {
				return true
			}
		}
		for _, shape := range []*presentationir.Shape{slide.Header, slide.Footer, slide.DateTime, slide.SlideNumber} {
			if legacyShapeContainsText(shape, want) {
				return true
			}
		}
		if slide.Notes != nil {
			for _, shape := range slide.Notes.Shapes {
				if legacyShapeContainsText(shape, want) {
					return true
				}
			}
		}
	}
	return false
}

func legacyShapeContainsText(shape *presentationir.Shape, want string) bool {
	if shape == nil {
		return false
	}
	for _, paragraph := range shape.Paragraphs {
		if strings.Contains(paragraphText(paragraph), want) {
			return true
		}
	}
	return false
}

func legacyFirstTableShape(pres *presentationir.Presentation) *presentationir.Shape {
	if pres == nil {
		return nil
	}
	for _, slide := range pres.Slides {
		for _, shape := range slide.Shapes {
			if shape != nil && shape.Kind == presentationir.ShapeTable && shape.Table != nil {
				return shape
			}
		}
	}
	return nil
}

func legacyTableHasAnyText(table *presentationir.Table) bool {
	return legacyTableContainsText(table, "")
}

func legacyTableContainsText(table *presentationir.Table, want string) bool {
	if table == nil {
		return false
	}
	for _, row := range table.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil {
				continue
			}
			for _, paragraph := range cell.Paragraphs {
				text := paragraphText(paragraph)
				if want == "" && strings.TrimSpace(text) != "" {
					return true
				}
				if want != "" && strings.Contains(text, want) {
					return true
				}
			}
		}
	}
	return false
}

func TestBuildPresentationEmitsNotesSlideForDateAndSlideNumberMetadata(t *testing.T) {
	pres := buildPresentation([]legacySlideText{{
		PersistID: 1,
		NotesHeaderFooter: legacyPptHeaderFooterInfo{
			flags:    legacyPptHFFlagDateVisible | legacyPptHFFlagSlideNumber,
			dateText: "April 28, 2026",
		},
	}}, "sample.ppt")
	if len(pres.Slides) != 1 {
		t.Fatalf("slides len = %d, want 1", len(pres.Slides))
	}
	if pres.Slides[0].Notes == nil {
		t.Fatal("notes = nil, want NotesSlide")
	}
	if len(pres.Slides[0].Notes.Shapes) != 2 {
		t.Fatalf("notes shapes len = %d, want 2", len(pres.Slides[0].Notes.Shapes))
	}
	dateShape := pres.Slides[0].Notes.Shapes[0]
	if got := dateShape.Placeholder; got != presentationir.PlaceholderDateTime {
		t.Fatalf("notes date placeholder = %v, want %v", got, presentationir.PlaceholderDateTime)
	}
	if got := paragraphText(dateShape.Paragraphs[0]); got != "April 28, 2026" {
		t.Fatalf("notes date text = %q, want April 28, 2026", got)
	}
	slideNumberShape := pres.Slides[0].Notes.Shapes[1]
	if got := slideNumberShape.Placeholder; got != presentationir.PlaceholderSlideNumber {
		t.Fatalf("notes slide number placeholder = %v, want %v", got, presentationir.PlaceholderSlideNumber)
	}
	if len(slideNumberShape.Paragraphs) != 1 || len(slideNumberShape.Paragraphs[0].Runs) != 1 || slideNumberShape.Paragraphs[0].Runs[0].FieldKind != "slidenum" {
		t.Fatalf("notes slide number shape = %#v, want slidenum field run", slideNumberShape)
	}
}

func TestBuildPresentationEmitsNotesSlideForTodayDateMetadata(t *testing.T) {
	pres := buildPresentation([]legacySlideText{{
		PersistID: 1,
		NotesHeaderFooter: legacyPptHeaderFooterInfo{
			flags: legacyPptHFFlagDateVisible | legacyPptHFFlagTodayDate,
		},
	}}, "sample.ppt")
	if len(pres.Slides) != 1 {
		t.Fatalf("slides len = %d, want 1", len(pres.Slides))
	}
	if pres.Slides[0].Notes == nil {
		t.Fatal("notes = nil, want NotesSlide")
	}
	if len(pres.Slides[0].Notes.Shapes) != 1 {
		t.Fatalf("notes shapes len = %d, want 1", len(pres.Slides[0].Notes.Shapes))
	}
	dateShape := pres.Slides[0].Notes.Shapes[0]
	if got := dateShape.Placeholder; got != presentationir.PlaceholderDateTime {
		t.Fatalf("notes date placeholder = %v, want %v", got, presentationir.PlaceholderDateTime)
	}
	if len(dateShape.Paragraphs) != 1 || len(dateShape.Paragraphs[0].Runs) != 1 || dateShape.Paragraphs[0].Runs[0].FieldKind != "date" {
		t.Fatalf("notes today-date shape = %#v, want one date field run", dateShape)
	}
}

func TestAttachNotesHeaderFooterInfoUsesNotesPersistID(t *testing.T) {
	slides := []legacySlideText{{PersistID: 1}, {PersistID: 2}}
	notes := []legacySlideText{{PersistID: 101}, {PersistID: 202}}
	slides = attachNotesByIndex(slides, notes)
	slides = []legacySlideText{slides[1], slides[0]}
	infos := mapLegacyPptPersistHeaderFooterInfos([]int{101, 202}, []legacyPptHeaderFooterInfo{{footerText: "Notes One"}, {footerText: "Notes Two"}})

	slides = attachNotesHeaderFooterInfoByPersistID(slides, infos, legacyPptHeaderFooterInfo{})

	if got := slides[0].NotesPersistID; got != 202 {
		t.Fatalf("slides[0] notes persist id = %d, want 202", got)
	}
	if got := slides[1].NotesPersistID; got != 101 {
		t.Fatalf("slides[1] notes persist id = %d, want 101", got)
	}
	if got := slides[0].NotesHeaderFooter.footerText; got != "Notes Two" {
		t.Fatalf("slides[0] notes footer = %q, want Notes Two", got)
	}
	if got := slides[1].NotesHeaderFooter.footerText; got != "Notes One" {
		t.Fatalf("slides[1] notes footer = %q, want Notes One", got)
	}
}

func TestAttachNotesHeaderFooterInfoSlideOverrideCanDisableDefaults(t *testing.T) {
	slides := []legacySlideText{{PersistID: 1, NotesPersistID: 101}}
	infos := map[int]legacyPptHeaderFooterInfo{101: {hasAtom: true, flags: 0}}
	defaults := legacyPptHeaderFooterInfo{hasAtom: true, flags: legacyPptHFFlagFooter, footerText: "Default Notes Footer"}

	slides = attachNotesHeaderFooterInfoByPersistID(slides, infos, defaults)
	if got := slides[0].NotesHeaderFooter.footerText; got != "" {
		t.Fatalf("notes footer text = %q, want empty (override clears defaults)", got)
	}
	if slides[0].NotesHeaderFooter.footerEnabled() {
		t.Fatalf("notes footer enabled = true, want false after explicit zero-flag override")
	}
}

func TestMergeLegacyPptNotesFragmentsReplacesPlaceholderPrompt(t *testing.T) {
	existing := []legacySlideFragment{
		{Text: "Click to add notes", TextType: pptTextTypeNotes},
		{Text: "non-notes metadata", TextType: pptTextTypeBody},
	}
	hydrated := []legacySlideFragment{{Text: "Actual speaker notes", TextType: pptTextTypeNotes}}

	got := mergeLegacyPptNotesFragments(existing, hydrated)

	if len(got) != 2 {
		t.Fatalf("fragments len = %d, want 2", len(got))
	}
	if got[0].Text != "non-notes metadata" || got[0].TextType != pptTextTypeBody {
		t.Fatalf("fragments[0] = %#v, want preserved non-notes fragment", got[0])
	}
	if got[1].Text != "Actual speaker notes" || got[1].TextType != pptTextTypeNotes {
		t.Fatalf("fragments[1] = %#v, want hydrated notes fragment", got[1])
	}
}

func TestMergeLegacyPptNotesFragmentsKeepsRealNotesAndDedupsHydrated(t *testing.T) {
	existing := []legacySlideFragment{{Text: "Real notes", TextType: pptTextTypeNotes}}
	hydrated := []legacySlideFragment{
		{Text: "Real notes", TextType: pptTextTypeNotes},
		{Text: "Second paragraph", TextType: pptTextTypeNotes},
	}

	got := mergeLegacyPptNotesFragments(existing, hydrated)

	if len(got) != 2 {
		t.Fatalf("fragments len = %d, want 2", len(got))
	}
	if got[0].Text != "Real notes" {
		t.Fatalf("fragments[0].Text = %q, want Real notes", got[0].Text)
	}
	if got[1].Text != "Second paragraph" {
		t.Fatalf("fragments[1].Text = %q, want Second paragraph", got[1].Text)
	}
}

// noteFragments is a test helper that returns all pptTextTypeNotes fragments from a slide.
func noteFragments(slide legacySlideText) []legacySlideFragment {
	var frags []legacySlideFragment
	for _, frag := range slide.Fragments {
		if frag.TextType == pptTextTypeNotes {
			frags = append(frags, frag)
		}
	}
	return frags
}
func TestBuildLegacySlideShapesTypedCenterBodyBecomesSubtitle(t *testing.T) {
	// A slide with CenterTitle + CenterBody should produce title + subtitle shapes.
	slide := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "Welcome", TextType: pptTextTypeCenterTitle},
			{Text: "An introduction", TextType: pptTextTypeCenterBody},
		},
	}
	shapes := buildLegacySlideShapesTyped(0, slide, "sample.ppt")
	if len(shapes) != 2 {
		t.Fatalf("shapes = %d, want 2", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderTitle {
		t.Fatalf("shape[0] placeholder = %v, want Title", shapes[0].Placeholder)
	}
	if shapes[1].Placeholder != presentationir.PlaceholderSubtitle {
		t.Fatalf("shape[1] placeholder = %v, want Subtitle", shapes[1].Placeholder)
	}
	if len(shapes[1].Paragraphs) == 0 || paragraphText(shapes[1].Paragraphs[0]) != "An introduction" {
		t.Fatalf("subtitle text = %#v", shapes[1].Paragraphs)
	}
	// Subtitle bounds must match legacySubtitleBounds().
	wantBounds := legacySubtitleBounds()
	if shapes[1].Bounds != wantBounds {
		t.Fatalf("subtitle bounds = %+v, want %+v", shapes[1].Bounds, wantBounds)
	}
}

func TestBuildLegacySlideShapesTypedCenterBodyWithoutTitleBecomesBody(t *testing.T) {
	// CenterBody without a title fragment should use body bounds, not full-slide.
	slide := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "Standalone text", TextType: pptTextTypeCenterBody},
		},
	}
	shapes := buildLegacySlideShapesTyped(0, slide, "sample.ppt")
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderBody {
		t.Fatalf("shape placeholder = %v, want Body", shapes[0].Placeholder)
	}
	if shapes[0].Bounds != legacyBodyBounds(false) {
		t.Fatalf("shape bounds = %+v, want %+v", shapes[0].Bounds, legacyBodyBounds(false))
	}
}

func TestBuildLegacySlideShapesTypedBodyWithoutTitleUsesBodyBounds(t *testing.T) {
	slide := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "Please review the updated forecast by Friday.", TextType: pptTextTypeBody},
		},
	}
	shapes := buildLegacySlideShapesTyped(0, slide, "sample.ppt")
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderBody {
		t.Fatalf("shape placeholder = %v, want Body", shapes[0].Placeholder)
	}
	if shapes[0].Bounds != legacyBodyBounds(false) {
		t.Fatalf("shape bounds = %+v, want %+v", shapes[0].Bounds, legacyBodyBounds(false))
	}
}

func TestBuildLegacySlideShapesTypedBodyOnlyBulletsUseBodyBounds(t *testing.T) {
	slide := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "- Revenue up\n- Costs down", TextType: pptTextTypeBody},
		},
	}
	shapes := buildLegacySlideShapesTyped(0, slide, "sample.ppt")
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderBody {
		t.Fatalf("shape placeholder = %v, want Body", shapes[0].Placeholder)
	}
	if shapes[0].Bounds != legacyBodyBounds(false) {
		t.Fatalf("shape bounds = %+v, want %+v", shapes[0].Bounds, legacyBodyBounds(false))
	}
}

func TestBuildLegacySlideShapesTypedSingleBulletUsesBodyBounds(t *testing.T) {
	slide := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "- Revenue up", TextType: pptTextTypeBody},
		},
	}
	shapes := buildLegacySlideShapesTyped(0, slide, "sample.ppt")
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderBody {
		t.Fatalf("shape placeholder = %v, want Body", shapes[0].Placeholder)
	}
	if shapes[0].Bounds != legacyBodyBounds(false) {
		t.Fatalf("shape bounds = %+v, want %+v", shapes[0].Bounds, legacyBodyBounds(false))
	}
}

func TestBuildLegacySlideShapesTypedPromotesFooterOnlyTypedFragment(t *testing.T) {
	slide := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "Quarterly Results", TextType: pptTextTypeTitle},
			{Text: "Slide 7", TextType: pptTextTypeOther},
		},
	}
	shapes := buildLegacySlideShapesTyped(0, slide, "sample.ppt")
	if len(shapes) != 2 {
		t.Fatalf("shapes = %d, want 2", len(shapes))
	}
	if shapes[0].Placeholder != presentationir.PlaceholderTitle {
		t.Fatalf("shape[0] placeholder = %v, want Title", shapes[0].Placeholder)
	}
	if shapes[1].Placeholder != presentationir.PlaceholderSlideNumber {
		t.Fatalf("shape[1] placeholder = %v, want SlideNumber", shapes[1].Placeholder)
	}
	if got := shapes[1].Paragraphs[0].Runs[0].FieldKind; got != "slidenum" {
		t.Fatalf("shape[1] FieldKind = %q, want slidenum", got)
	}
	if shapes[1].Bounds != legacyFooterBoundsForPlaceholder(presentationir.PlaceholderSlideNumber) {
		t.Fatalf("shape[1] bounds = %+v, want %+v", shapes[1].Bounds, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderSlideNumber))
	}
}

func TestLooksLikeLegacyBulletRecognizesParenthesizedMarkersWithoutSpace(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"(a) alpha item", true},
		{"(iv)roman no space", true},
		{"(3)numeric no space", true},
		{"(a ) not a marker", false},
		{"plain text", false},
	}
	for _, test := range tests {
		if got := looksLikeLegacyBullet(test.text); got != test.want {
			t.Fatalf("looksLikeLegacyBullet(%q) = %v, want %v", test.text, got, test.want)
		}
	}
}

// Slice 8: notes prompt filter

func TestExtractLegacyNotesParagraphsFiltersClickToAddNotes(t *testing.T) {
	st := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "Click to add notes", TextType: pptTextTypeNotes},
		},
	}
	paras := extractLegacyNotesParagraphs(st)
	if len(paras) != 0 {
		t.Fatalf("expected 0 paragraphs after filtering notes prompt, got %d: %#v", len(paras), paras)
	}
}

func TestExtractLegacyNotesParagraphsFiltersClickToAddTextVariant(t *testing.T) {
	st := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "Click to add text", TextType: pptTextTypeNotes},
		},
	}
	paras := extractLegacyNotesParagraphs(st)
	if len(paras) != 0 {
		t.Fatalf("expected 0 paragraphs for click-to-add-text prompt, got %d", len(paras))
	}
}

func TestExtractLegacyNotesParagraphsKeepsRealNotesText(t *testing.T) {
	st := legacySlideText{
		Fragments: []legacySlideFragment{
			{Text: "These are the actual speaker notes.", TextType: pptTextTypeNotes},
		},
	}
	paras := extractLegacyNotesParagraphs(st)
	if len(paras) == 0 {
		t.Fatal("expected non-empty paragraphs for real notes text")
	}
	if got := paragraphText(paras[0]); got != "These are the actual speaker notes." {
		t.Fatalf("notes text = %q", got)
	}
}

func TestBuildPresentationSuppressesNotesSlideForClickToAddNotes(t *testing.T) {
	slides := []legacySlideText{
		{
			PersistID: 1,
			Fragments: []legacySlideFragment{
				{Text: "Main Slide Title", TextType: pptTextTypeTitle},
				{Text: "Click to add notes", TextType: pptTextTypeNotes},
			},
		},
	}
	pres := buildPresentation(slides, "sample.ppt")
	if len(pres.Slides) == 0 {
		t.Fatal("expected at least one slide")
	}
	if pres.Slides[0].Notes != nil {
		t.Fatalf("expected nil notes for prompt-only notes fragment, got %#v", pres.Slides[0].Notes)
	}
}

// Slice 9: TextRulerAtom deferred application

func TestTextRulerAtomDeferredApplicationPopulatesIndent(t *testing.T) {
	// Synthetic stream: TextCharsAtom, then TextRulerAtom (before STPA), then StyleTextPropAtom.
	// This verifies pending-ruler behavior when paragraph records arrive later in stream order.
	rawText := "Hello"
	textChars := buildPptAtomRecord(0, pptRTTextCharsAtom, encodeLegacyUTF16(rawText))

	// TextRulerAtom: mask bit 3 (leftMargin[0]) present with value 500 master units.
	rulerPayload := marshalLegacyUint16LE(0x0008)
	rulerPayload = append(rulerPayload, marshalLegacyUint16LE(0x0000)...)
	rulerPayload = append(rulerPayload, marshalLegacyUint16LE(500)...)
	textRuler := buildPptAtomRecord(0, pptRTTextRulerAtom, rulerPayload)

	// Minimal legacy STPA payload for one paragraph/run covering all text.
	pf := buildPFRecord(uint16(len(rawText)+1), 0, nil)
	cf := buildCFRecord(uint16(len(rawText)+1), 0, nil)
	stpa := buildPptAtomRecord(0, pptRTStyleTextPropAtom, append(pf, cf...))

	stream := append(append(textChars, textRuler...), stpa...)
	frags, _ := parseLegacyPptTextBoxFragments(stream, nil, nil, nil)
	if len(frags) != 1 {
		t.Fatalf("expected 1 fragment, got %d", len(frags))
	}
	if len(frags[0].Paragraphs) == 0 {
		t.Fatal("expected paragraphs from STPA, got none")
	}
	para := frags[0].Paragraphs[0]
	if !para.HasMarginLeft {
		t.Fatal("expected HasMarginLeft=true from deferred ruler")
	}
	if para.MarginLeftEMU != legacyPptMasterUnitsToEMU(500) {
		t.Fatalf("MarginLeftEMU = %d, want %d", para.MarginLeftEMU, legacyPptMasterUnitsToEMU(500))
	}
}

// TestExtractLegacyPptOfficeArtBlipPICTDecompressesPayload verifies that a
// synthetic PICT BLIP payload (single UID + 34-byte Metafile.Header + zlib data)
// is correctly decompressed and returned with a 512-byte QuickDraw Mac header
// prepended, matching the HSLF PICT.getData() contract.
func TestExtractLegacyPptOfficeArtBlipPICTDecompressesPayload(t *testing.T) {
	// Build the zlib-compressed PICT payload.
	const pict = "PICT payload sentinel bytes"
	var compBuf bytes.Buffer
	zw := zlib.NewWriter(&compBuf)
	if _, err := zw.Write([]byte(pict)); err != nil {
		t.Fatalf("zlib write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zlib close: %v", err)
	}

	// Build the full BLIP payload: 16 UID bytes + 34 Metafile.Header bytes +
	// zlib data. UID and header contents are irrelevant for extraction.
	const uidSize = 16
	const metafileHeaderSize = 34
	payload := make([]byte, uidSize+metafileHeaderSize+compBuf.Len())
	// Write wmfSize field in Metafile.Header at offset uidSize so the 34-byte
	// header slot is populated (content doesn't affect extraction).
	binary.LittleEndian.PutUint32(payload[uidSize:], uint32(len(pict)))
	copy(payload[uidSize+metafileHeaderSize:], compBuf.Bytes())

	got := extractLegacyPptOfficeArtBlipPICT(payload)
	if len(got) == 0 {
		t.Fatal("extractLegacyPptOfficeArtBlipPICT returned empty")
	}
	const macHeaderSize = 512
	if len(got) < macHeaderSize {
		t.Fatalf("result len = %d, want >= %d", len(got), macHeaderSize)
	}
	// First 512 bytes must be the zero Mac header.
	for i, b := range got[:macHeaderSize] {
		if b != 0 {
			t.Fatalf("Mac header byte[%d] = 0x%02X, want 0", i, b)
		}
	}
	// Remaining bytes must be the original PICT content.
	if got := string(got[macHeaderSize:]); got != pict {
		t.Fatalf("PICT body = %q, want %q", got, pict)
	}
}

// TestExtractLegacyPptOfficeArtBlipImagePICTReturnsCorrectMIME verifies that
// extractLegacyPptOfficeArtBlipImage routes legacyOfficeArtBlipPICT through the
// PICT extractor and returns MIME type "image/x-pict".
func TestExtractLegacyPptOfficeArtBlipImagePICTReturnsCorrectMIME(t *testing.T) {
	const pict = "PICT body"
	var compBuf bytes.Buffer
	zw := zlib.NewWriter(&compBuf)
	if _, err := zw.Write([]byte(pict)); err != nil {
		t.Fatalf("zlib write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zlib close: %v", err)
	}
	payload := make([]byte, 16+34+compBuf.Len())
	copy(payload[16+34:], compBuf.Bytes())

	blob, mime := extractLegacyPptOfficeArtBlipImage(legacyOfficeArtBlipPICT, payload)
	if mime != "image/x-pict" {
		t.Fatalf("MIME = %q, want \"image/x-pict\"", mime)
	}
	if len(blob) == 0 {
		t.Fatal("blob is empty")
	}
}

// TestParseLegacyPptTextShapeContainerFlagChildSelectsChildAnchor verifies that
// when FLAG_CHILD (0x02) is set in EscherSpRecord.flags and both a ClientAnchor
// and a ChildAnchor are present, the ChildAnchor wins (matching POI
// HSLFShape.getAnchor() FLAG_CHILD logic, MS-ODRAW §2.2.38).
func TestParseLegacyPptTextShapeContainerFlagChildSelectsChildAnchor(t *testing.T) {
	// Group frame: domain 0..1000 × 0..1000 → target 0..10000 × 0..10000 EMU.
	const masterW, masterH = 1000, 1000
	clientAnchorBytes := []byte{0, 0, 0, 0, // left=0, top=0
		byte(masterW & 0xFF), byte(masterW >> 8), // right=masterW (uint16 LE)
		byte(masterH & 0xFF), byte(masterH >> 8)} // bottom=masterH

	// ChildAnchor in group-domain coords: (100,200)–(600,700) → maps to a
	// distinct region so we can tell the two apart.
	child := make([]byte, 16)
	binary.LittleEndian.PutUint32(child[0:4], 100)
	binary.LittleEndian.PutUint32(child[4:8], 200)
	binary.LittleEndian.PutUint32(child[8:12], 600)
	binary.LittleEndian.PutUint32(child[12:16], 700)

	// ClientAnchor covers the full slide — if selected it would produce
	// non-zero bounds that differ from the child-mapped bounds.
	wantClientBounds, _ := parseLegacyPptClientAnchor(clientAnchorBytes)

	// Build a group frame so ChildAnchor can be mapped.
	spgr := make([]byte, 16)
	binary.LittleEndian.PutUint32(spgr[0:4], 0)
	binary.LittleEndian.PutUint32(spgr[4:8], 0)
	binary.LittleEndian.PutUint32(spgr[8:12], masterW)
	binary.LittleEndian.PutUint32(spgr[12:16], masterH)
	frame, ok := parseLegacyPptGroupFrameFromSpContainer(
		append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(1, 0x01)),
				buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, spgr)...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, clientAnchorBytes)...,
		),
		legacyPptGroupFrame{},
	)
	if !ok {
		t.Fatal("failed to build group frame")
	}

	// Shape with FLAG_CHILD set (0x02) — ChildAnchor should win.
	container := append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002,
				officeArtSpPayload(2, legacyOfficeArtSpFlagChild)),
			buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, clientAnchorBytes)...,
		),
		append(
			buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, child),
			buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000,
				append(
					buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeBody)),
					buildPptAtomRecord(0, pptRTTextBytesAtom, []byte("hi"))...,
				),
			)...,
		)...,
	)
	shape, ok := parseLegacyPptTextShapeContainer(container, frame, nil, nil, nil)
	if !ok {
		t.Fatal("parseLegacyPptTextShapeContainer returned false")
	}
	if shape.bounds == wantClientBounds {
		t.Fatalf("bounds = ClientAnchor bounds %+v; expected ChildAnchor-mapped bounds", shape.bounds)
	}
	if shape.bounds.W == 0 || shape.bounds.H == 0 {
		t.Fatalf("bounds = %+v, want non-empty ChildAnchor-mapped bounds", shape.bounds)
	}
}

// TestParseLegacyPptTextShapeContainerNoFlagChildSelectsClientAnchor verifies
// that when FLAG_CHILD is clear and both anchor types are present, the
// ClientAnchor is selected (matching POI HSLFShape.getAnchor()).
func TestParseLegacyPptTextShapeContainerNoFlagChildSelectsClientAnchor(t *testing.T) {
	clientAnchorBytes := []byte{0, 0, 0, 0, 0xC0, 0x12, 0x90, 0x01} // some non-zero client anchor

	child := make([]byte, 16)
	binary.LittleEndian.PutUint32(child[0:4], 100)
	binary.LittleEndian.PutUint32(child[4:8], 200)
	binary.LittleEndian.PutUint32(child[8:12], 600)
	binary.LittleEndian.PutUint32(child[12:16], 700)

	wantBounds, ok := parseLegacyPptClientAnchor(clientAnchorBytes)
	if !ok {
		t.Fatal("failed to parse reference client anchor")
	}

	// Build a group frame so ChildAnchor could be mapped if (wrongly) selected.
	spgr := make([]byte, 16)
	binary.LittleEndian.PutUint32(spgr[8:12], 1000)
	binary.LittleEndian.PutUint32(spgr[12:16], 1000)
	frame, _ := parseLegacyPptGroupFrameFromSpContainer(
		append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(1, 0x01)),
				buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, spgr)...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, clientAnchorBytes)...,
		),
		legacyPptGroupFrame{},
	)

	// Shape with FLAG_CHILD clear (flags=0) — ClientAnchor should win.
	container := append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(2, 0x00)),
			buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, child)...,
		),
		append(
			buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, clientAnchorBytes),
			buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000,
				append(
					buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeBody)),
					buildPptAtomRecord(0, pptRTTextBytesAtom, []byte("hi"))...,
				),
			)...,
		)...,
	)
	shape, ok := parseLegacyPptTextShapeContainer(container, frame, nil, nil, nil)
	if !ok {
		t.Fatal("parseLegacyPptTextShapeContainer returned false")
	}
	if shape.bounds != wantBounds {
		t.Fatalf("bounds = %+v, want ClientAnchor bounds %+v", shape.bounds, wantBounds)
	}
}

// TestParseLegacyPptGroupFrameSpContainerPrefersClientAnchorOverChildAnchor
// verifies that when a group SpContainer has both ClientAnchor and ChildAnchor
// the ClientAnchor is used as the group's target placement, matching POI
// HSLFGroupShape.getAnchor() which tries ClientAnchor first.
func TestParseLegacyPptGroupFrameSpContainerPrefersClientAnchorOverChildAnchor(t *testing.T) {
	clientAnchorBytes := []byte{0, 0, 0, 0, 0xC0, 0x12, 0x90, 0x01}
	wantAnchor, ok := parseLegacyPptClientAnchor(clientAnchorBytes)
	if !ok {
		t.Fatal("failed to parse reference client anchor")
	}

	spgr := make([]byte, 16)
	binary.LittleEndian.PutUint32(spgr[8:12], 1000)
	binary.LittleEndian.PutUint32(spgr[12:16], 1000)

	child := make([]byte, 16)
	binary.LittleEndian.PutUint32(child[0:4], 50)
	binary.LittleEndian.PutUint32(child[4:8], 50)
	binary.LittleEndian.PutUint32(child[8:12], 500)
	binary.LittleEndian.PutUint32(child[12:16], 500)

	// Outer frame large enough that the ChildAnchor could be mapped.
	outerSpgr := make([]byte, 16)
	binary.LittleEndian.PutUint32(outerSpgr[8:12], 10000)
	binary.LittleEndian.PutUint32(outerSpgr[12:16], 10000)
	outerClientAnchor := []byte{0, 0, 0, 0, 0x00, 0x5A, 0x00, 0x5A}
	outerFrame, _ := parseLegacyPptGroupFrameFromSpContainer(
		append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(1, 0x01)),
				buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, outerSpgr)...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, outerClientAnchor)...,
		),
		legacyPptGroupFrame{},
	)

	// Group SpContainer with both ClientAnchor and ChildAnchor present.
	container := append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(2, 0x01)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, spgr)...,
		),
		append(
			buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, child),
			buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, clientAnchorBytes)...,
		)...,
	)
	frame, ok := parseLegacyPptGroupFrameFromSpContainer(container, outerFrame)
	if !ok {
		t.Fatal("parseLegacyPptGroupFrameFromSpContainer returned false")
	}
	if frame.target != wantAnchor {
		t.Fatalf("target = %+v, want ClientAnchor-derived %+v", frame.target, wantAnchor)
	}
}

// TestParseLegacyPptTextBoxFragmentsOutlineRefDiscardsInlineText verifies that
// when an OutlineTextRefAtom is present in a ClientTextbox, any inline text
// atoms (TextBytesAtom / TextCharsAtom) in the same container are discarded and
// only the outline-ref index is returned. This matches POI
// HSLFTextParagraph.findTextParagraphs, which ignores ClientTextbox inline
// content and resolves text solely from SlideListWithText when an
// OutlineTextRefAtom is found.
func TestParseLegacyPptTextBoxFragmentsOutlineRefDiscardsInlineText(t *testing.T) {
	// Build a ClientTextbox record body: TextHeaderAtom → OutlineTextRefAtom
	// (index 3) → TextBytesAtom ("inline"). Per POI, the inline text must be
	// discarded and the returned frags must be nil.
	outlineRefPayload := make([]byte, 4)
	binary.LittleEndian.PutUint32(outlineRefPayload, 3)

	body := append(
		append(
			buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeBody)),
			buildPptAtomRecord(0, pptRTOutlineTextRefAtom, outlineRefPayload)...,
		),
		buildPptAtomRecord(0, pptRTTextBytesAtom, []byte("inline text that must be dropped"))...,
	)

	frags, outlineRef := parseLegacyPptTextBoxFragments(body, nil, nil, nil)
	if outlineRef != 3 {
		t.Fatalf("outlineRef = %d, want 3", outlineRef)
	}
	if len(frags) != 0 {
		texts := make([]string, len(frags))
		for i, f := range frags {
			texts[i] = f.Text
		}
		t.Fatalf("frags = %v, want nil (inline text must be discarded when OutlineTextRefAtom present)", texts)
	}
}

// TestExtractSlideTextsFromDocumentExcludesMasterSlides verifies Document container
// ownership semantics per POI parity: master slides (SlideListWithText instance=1) are
// never included in the regular slide list. This test creates a synthetic Document with
// instance 0 (2 regular slides), instance 1 (1 master slide), and no instance 2 (notes).
// Only the 2 instance-0 slides should be extracted; the master slide must be excluded.
func TestExtractSlideTextsFromDocumentExcludesMasterSlides(t *testing.T) {
	// Build instance 0 (regular slides) with slides having PersistID 100, 101
	instance0Payload := append(
		append(buildPptAtomRecord(0, pptRTSlidePersistAtom, binary.LittleEndian.AppendUint32(nil, 100)),
			buildPptAtomRecord(0, pptRTTextHeaderAtom, binary.LittleEndian.AppendUint32(nil, pptTextTypeTitle))...),
		buildPptAtomRecord(0, pptRTSlidePersistAtom, binary.LittleEndian.AppendUint32(nil, 101))...)

	instance0 := buildPptContainerRecord(0, pptRTSlideListWithText, 0x0F, instance0Payload)

	// Build instance 1 (master slides) with slide having PersistID 200
	instance1Payload := append(
		buildPptAtomRecord(0, pptRTSlidePersistAtom, binary.LittleEndian.AppendUint32(nil, 200)),
		buildPptAtomRecord(0, pptRTTextHeaderAtom, binary.LittleEndian.AppendUint32(nil, pptTextTypeTitle))...)

	instance1 := buildPptContainerRecord(1, pptRTSlideListWithText, 0x0F, instance1Payload)

	// Build Document container with both instance 0 and 1
	docPayload := append(
		buildPptAtomRecord(0, pptRTDocumentAtom, make([]byte, 28)), // Document atom
		append(
			buildPptAtomRecord(0, 0x0F9D, make([]byte, 4)), // Environment-like record
			append(instance0, instance1...)...,
		)...,
	)

	docPayload = append(
		docPayload,
		buildPptAtomRecord(0, 0x03EA, nil)..., // EndDocument-like record
	)

	docContainer := buildPptContainerRecord(0, pptRTDocument, 0x0F, docPayload)

	// Create minimal stream with CurrentUserAtom pointing to Document
	stream := make([]byte, 2048)
	copy(stream, buildPptAtomRecord(0, pptRTCurrentUserAtom, binary.LittleEndian.AppendUint32(nil, 1024)))

	// Place Document at offset 1024
	copy(stream[1024:], docContainer)

	// Call extractSlideTextsFromDocument with notesListInstance=2 (should match nothing)
	slides := extractSlideTextsFromDocument(stream, 1024, 2, map[int]int{}, map[uint16]string{}, map[uint8]string{}, map[uint32]string{})

	// Verify only 2 slides extracted (not including master slide with ID 200)
	if len(slides) != 2 {
		t.Fatalf("len(slides) = %d, want 2 (master slides instance=1 must be excluded)", len(slides))
	}

	// Verify we got regular slides 100 and 101, not master slide 200
	if slides[0].PersistID != 100 {
		t.Fatalf("slides[0].PersistID = %d, want 100 (first regular slide)", slides[0].PersistID)
	}
	if slides[1].PersistID != 101 {
		t.Fatalf("slides[1].PersistID = %d, want 101 (second regular slide)", slides[1].PersistID)
	}
}

// TestParseLegacyPptTextShapeContainerOutlineRefExcludesInlineText verifies
// end-to-end: a shape container with both an OutlineTextRefAtom and inline
// text produces a shape whose text comes from the SlideListWithText outline
// path (paragraphs nil from the container parse), not from the inline bytes.
func TestParseLegacyPptTextShapeContainerOutlineRefExcludesInlineText(t *testing.T) {
	outlineRefPayload := make([]byte, 4)
	binary.LittleEndian.PutUint32(outlineRefPayload, 2)

	textboxBody := append(
		append(
			buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeBody)),
			buildPptAtomRecord(0, pptRTOutlineTextRefAtom, outlineRefPayload)...,
		),
		buildPptAtomRecord(0, pptRTTextBytesAtom, []byte("stale inline text"))...,
	)
	container := append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x42, 0x00)),
			buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, []byte{0, 0, 0, 0, 128, 22, 224, 16})...,
		),
		buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000, textboxBody)...,
	)
	shape, ok := parseLegacyPptTextShapeContainer(container, legacyPptGroupFrame{}, nil, nil, nil)
	if !ok {
		t.Fatal("parseLegacyPptTextShapeContainer returned false")
	}
	if shape.outlineRef != 2 {
		t.Fatalf("outlineRef = %d, want 2", shape.outlineRef)
	}
	// Paragraphs must be nil so the caller resolves text from outline-ref only.
	if len(shape.paragraphs) != 0 {
		t.Fatalf("paragraphs = %v, want nil (inline text discarded for outline-ref shapes)", shape.paragraphs)
	}
	if got := legacyPptParagraphsText(shape.inlineFallback); got != "stale inline text" {
		t.Fatalf("inlineFallback = %q, want %q", got, "stale inline text")
	}
}

// TestCollectUserEditsStopsAtBrokenLink verifies that a broken OffsetLastEdit
// (non-zero but not pointing to a valid UserEditAtom) does not discard edits
// already collected — the chain walk stops and the partial result is returned.
func TestCollectUserEditsStopsAtBrokenLink(t *testing.T) {
	makeUEA := func(offsetLastEdit, offsetPersistDir int) []byte {
		payload := make([]byte, 28)
		binary.LittleEndian.PutUint32(payload[8:12], uint32(offsetLastEdit))
		binary.LittleEndian.PutUint32(payload[12:16], uint32(offsetPersistDir))
		binary.LittleEndian.PutUint32(payload[16:20], 1) // DocPersistIDRef=1
		return buildPptAtomRecord(0, pptRTUserEditAtom, payload)
	}
	makePPH := func(persistID, offset uint32) []byte {
		p := make([]byte, 8)
		binary.LittleEndian.PutUint32(p[0:4], persistID|(1<<20))
		binary.LittleEndian.PutUint32(p[4:8], offset)
		return buildPptAtomRecord(0, pptRTPersistDirectory, p)
	}

	// Layout (all bytes after offset 0 are structured; offset 0 is left as zeros):
	//  72:  UserEditAtom B, OffsetLastEdit=256 (→ beyond stream, broken)
	// 108:  UserEditAtom C (newest), OffsetLastEdit=72 (→ B)
	// 144:  PersistPtrHolder for B (persistID=1 → 400)
	// 160:  PersistPtrHolder for C (persistID=1 → 500)
	atomB := makeUEA(256, 144) // OffsetLastEdit=256 → out-of-bounds, broken
	atomC := makeUEA(72, 160)
	ptrB := makePPH(1, 400)
	ptrC := makePPH(1, 500)

	stream := make([]byte, 176)
	copy(stream[72:], atomB)
	copy(stream[108:], atomC)
	copy(stream[144:], ptrB)
	copy(stream[160:], ptrC)

	// Walking starts at C (108). C→B (72). B.OffsetLastEdit=256 → out-of-bounds → break.
	// Should return [C, B], ok=true (2 edits, not nil).
	edits, ok := collectUserEdits(stream, 108)
	if !ok {
		t.Fatal("collectUserEdits returned ok=false; should stop at broken link and return partial chain")
	}
	if len(edits) != 2 {
		t.Fatalf("len(edits) = %d, want 2 (C and B collected before broken link)", len(edits))
	}

	dir := buildPersistDirectory(stream, edits)
	// Oldest-first application: B sets persistID=1→400, C overrides to 500.
	if dir[1] != 500 {
		t.Fatalf("dir[1] = %d, want 500 (newest edit wins)", dir[1])
	}
}

// TestApplyPersistDirectoryEntriesDuplicateIDLastWins verifies parity with POI
// PersistPtrHolder read-path behavior: when two groups within the same
// PersistPtrHolder payload cover the same persist ID, the last occurrence wins
// (HashMap.put silently overwrites; our map write does the same).
func TestApplyPersistDirectoryEntriesDuplicateIDLastWins(t *testing.T) {
	// Group 1: persistID=5, count=2 → offsets [100, 200]  (ID=5→100, ID=6→200)
	// Group 2: persistID=6, count=1 → offset  [999]       (ID=6→999, overwrites 200)
	payload := make([]byte, 20)
	binary.LittleEndian.PutUint32(payload[0:4], 5|(2<<20)) // persistID=5, count=2
	binary.LittleEndian.PutUint32(payload[4:8], 100)
	binary.LittleEndian.PutUint32(payload[8:12], 200)
	binary.LittleEndian.PutUint32(payload[12:16], 6|(1<<20)) // persistID=6, count=1
	binary.LittleEndian.PutUint32(payload[16:20], 999)

	dir := map[int]int{}
	applyPersistDirectoryEntries(dir, payload)

	if dir[5] != 100 {
		t.Fatalf("dir[5] = %d, want 100", dir[5])
	}
	if dir[6] != 999 {
		t.Fatalf("dir[6] = %d, want 999 (second occurrence wins)", dir[6])
	}
}

// TestCollectUserEditsCycleRepairFindsMissingOlderEdit verifies the HSLFSlideShowImpl
// initRecordOffsets heuristic: when OffsetLastEdit forms a 2-node cycle (B→C→B),
// collectUserEdits repairs by appending the oldest UserEditAtom found at
// (minKnownUserEditAtomOffset - 36).
func TestCollectUserEditsCycleRepairFindsMissingOlderEdit(t *testing.T) {
	makeUEA := func(offsetLastEdit, offsetPersistDir, docPersistRef int) []byte {
		payload := make([]byte, 28) // len=0x1C
		binary.LittleEndian.PutUint32(payload[8:12], uint32(offsetLastEdit))
		binary.LittleEndian.PutUint32(payload[12:16], uint32(offsetPersistDir))
		binary.LittleEndian.PutUint32(payload[16:20], uint32(docPersistRef))
		return buildPptAtomRecord(0, pptRTUserEditAtom, payload)
	}
	makePPH := func(payload []byte) []byte {
		return buildPptAtomRecord(0, pptRTPersistDirectory, payload)
	}

	// Stream layout:
	//   Offset   0: UserEditAtom A (36 bytes) – genuine oldest, OffsetLastEdit=0
	//   Offset  36: UserEditAtom B (36 bytes) – CORRUPT: OffsetLastEdit=72 (→C, not →A)
	//   Offset  72: UserEditAtom C (36 bytes) – OffsetLastEdit=36 (→B, forming cycle)
	//   Offset 108: PersistPtrHolder A (16 bytes) – persistID=10 → 500
	//   Offset 124: PersistPtrHolder B (20 bytes) – persistID=10 → 501, persistID=11 → 502
	//   Offset 144: PersistPtrHolder C (16 bytes) – persistID=10 → 503

	ptrPayloadA := make([]byte, 8)
	binary.LittleEndian.PutUint32(ptrPayloadA[0:4], uint32(10)|(1<<20)) // persistID=10, count=1
	binary.LittleEndian.PutUint32(ptrPayloadA[4:8], 500)

	ptrPayloadB := make([]byte, 12)
	binary.LittleEndian.PutUint32(ptrPayloadB[0:4], uint32(10)|(2<<20)) // persistID=10, count=2
	binary.LittleEndian.PutUint32(ptrPayloadB[4:8], 501)
	binary.LittleEndian.PutUint32(ptrPayloadB[8:12], 502)

	ptrPayloadC := make([]byte, 8)
	binary.LittleEndian.PutUint32(ptrPayloadC[0:4], uint32(10)|(1<<20)) // persistID=10, count=1
	binary.LittleEndian.PutUint32(ptrPayloadC[4:8], 503)

	atomA := makeUEA(0, 108, 1)  // A: terminus (OffsetLastEdit=0)
	atomB := makeUEA(72, 124, 1) // B: CORRUPT – OffsetLastEdit points to C at 72
	atomC := makeUEA(36, 144, 1) // C: OffsetLastEdit points to B at 36 (cycle)
	ptrA := makePPH(ptrPayloadA)
	ptrB := makePPH(ptrPayloadB)
	ptrC := makePPH(ptrPayloadC)

	stream := make([]byte, 160)
	copy(stream[0:], atomA)
	copy(stream[36:], atomB)
	copy(stream[72:], atomC)
	copy(stream[108:], ptrA)
	copy(stream[124:], ptrB)
	copy(stream[144:], ptrC)

	// Walking starts at C (offset 72).  Cycle forms at the second visit to 72
	// after walking C→B→C.  Repair: minOffset=36, candidate=36-36=0 → finds A.
	edits, ok := collectUserEdits(stream, 72)
	if !ok {
		t.Fatal("collectUserEdits returned ok=false")
	}
	if len(edits) != 3 {
		t.Fatalf("len(edits) = %d, want 3 (repair should append oldest A)", len(edits))
	}

	dir := buildPersistDirectory(stream, edits)
	// After applying A→B→C (oldest first): persistID=10 ends up at 503 (C wins).
	if dir[10] != 503 {
		t.Fatalf("dir[10] = %d, want 503", dir[10])
	}
	// persistID=11 is only in B's directory (502); C does not override it.
	if dir[11] != 502 {
		t.Fatalf("dir[11] = %d, want 502", dir[11])
	}
}

// TestCollectUserEditsCycleRepairStillReturnsCollectedEditsWhenCandidateInvalid verifies
// graceful degradation: when cycle is detected but the repair candidate at
// (minOffset-36) is not a valid UserEditAtom, collectUserEdits returns the edits
// collected so far without crashing.
func TestCollectUserEditsCycleRepairStillReturnsCollectedEditsWhenCandidateInvalid(t *testing.T) {
	makeUEA := func(offsetLastEdit, offsetPersistDir, docPersistRef int) []byte {
		payload := make([]byte, 28) // len=0x1C
		binary.LittleEndian.PutUint32(payload[8:12], uint32(offsetLastEdit))
		binary.LittleEndian.PutUint32(payload[12:16], uint32(offsetPersistDir))
		binary.LittleEndian.PutUint32(payload[16:20], uint32(docPersistRef))
		return buildPptAtomRecord(0, pptRTUserEditAtom, payload)
	}

	// Self-cycle: B at offset 36, OffsetLastEdit=36.
	// minOffset=36, candidate=0; stream[0] is 0xFF (garbage) → repair fails → break.
	// Expected: edits=[B], len=1.
	atomB := makeUEA(36, 100, 1)

	ptrPayload := make([]byte, 8)
	binary.LittleEndian.PutUint32(ptrPayload[0:4], uint32(1)|(1<<20))
	binary.LittleEndian.PutUint32(ptrPayload[4:8], 200)
	ptrRecord := buildPptAtomRecord(0, pptRTPersistDirectory, ptrPayload)

	stream := make([]byte, 200)
	stream[0] = 0xFF // garbage at candidate offset 0 – not a valid UserEditAtom header
	copy(stream[36:], atomB)
	copy(stream[100:], ptrRecord)

	edits, ok := collectUserEdits(stream, 36)
	if !ok {
		t.Fatal("collectUserEdits returned ok=false; should degrade gracefully")
	}
	if len(edits) != 1 {
		t.Fatalf("len(edits) = %d, want 1 (no repair when candidate invalid)", len(edits))
	}
}

// TestCollectMasterEntriesFromDocumentExtractsSlideIdentifiers verifies that
// collectMasterEntriesFromDocument reads refID (payload[0:4]) and
// slideIdentifier (payload[12:16]) from each SlidePersistAtom in the master
// SlideListWithText (instance 1) and ignores instance 0 (regular slides).
func TestCollectMasterEntriesFromDocumentExtractsSlideIdentifiers(t *testing.T) {
	// Build SlidePersistAtom payload: 28 bytes (refID @ 0, flags @ 4,
	// numPlaceholders @ 8, slideIdentifier @ 12, reserved @ 16).
	makeSPA := func(refID, slideIdentifier uint32) []byte {
		p := make([]byte, 28)
		binary.LittleEndian.PutUint32(p[0:4], refID)
		binary.LittleEndian.PutUint32(p[12:16], slideIdentifier)
		return p
	}
	spa1 := buildPptAtomRecord(0, pptRTSlidePersistAtom, makeSPA(3, 0x80000000))
	spa2 := buildPptAtomRecord(0, pptRTSlidePersistAtom, makeSPA(4, 0x8000000C))
	slwtPayload := append(spa1, spa2...)
	// instance 1 → master SLWT
	masterSLWT := buildPptContainerRecord(1, pptRTSlideListWithText, 0x0F, slwtPayload)
	// instance 0 → regular slide SLWT (should be ignored)
	spa3 := buildPptAtomRecord(0, pptRTSlidePersistAtom, makeSPA(5, 256))
	regularSLWT := buildPptContainerRecord(0, pptRTSlideListWithText, 0x0F, spa3)
	docPayload := append(masterSLWT, regularSLWT...)
	document := buildPptContainerRecord(0, pptRTDocument, 0x0F, docPayload)
	stream := document

	entries := collectMasterEntriesFromDocument(stream, 0)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	wantID0 := -2147483648 // 0x80000000 as signed int
	wantID1 := -2147483636 // 0x8000000C as signed int
	if entries[0].RefID != 3 || entries[0].SlideIdentifier != wantID0 {
		t.Errorf("entries[0] = %+v, want {RefID:3, SlideIdentifier:%d}", entries[0], wantID0)
	}
	if entries[1].RefID != 4 || entries[1].SlideIdentifier != wantID1 {
		t.Errorf("entries[1] = %+v, want {RefID:4, SlideIdentifier:%d}", entries[1], wantID1)
	}
}

// TestBuildPresentationUsesPerSlideMasterStyles verifies that when two masters
// with distinct slideIdentifiers carry different text styles, each slide gets
// the styles from the master its MasterID references rather than the first
// master found in the stream.
func TestBuildPresentationUsesPerSlideMasterStyles(t *testing.T) {
	boldTrue := true
	masterA := legacyPptMasterTextStyles{
		pptTextTypeBody: {
			0: legacyPptMasterTextLevel{
				run: stpaCFRun{Bold: &boldTrue},
			},
		},
	}
	masterB := legacyPptMasterTextStyles{
		pptTextTypeBody: {
			0: legacyPptMasterTextLevel{
				run: stpaCFRun{FontSize: 2400},
			},
		},
	}
	masterIDA := -2147483648 // 0x80000000 as signed int
	masterIDB := -2147483636 // 0x8000000C as signed int
	masterStylesByID := map[int]legacyPptMasterTextStyles{
		masterIDA: masterA,
		masterIDB: masterB,
	}
	slides := []legacySlideText{
		{PersistID: 1, MasterID: masterIDA, Fragments: []legacySlideFragment{{Text: "slide A", TextType: pptTextTypeBody}}},
		{PersistID: 2, MasterID: masterIDB, Fragments: []legacySlideFragment{{Text: "slide B", TextType: pptTextTypeBody}}},
	}
	pres := buildPresentationWithMasterComposition(slides, nil, nil, legacyPptHeaderFooterInfo{}, "test.ppt", nil, masterStylesByID, nil, "", nil)
	if len(pres.Slides) != 2 {
		t.Fatalf("slides = %d, want 2", len(pres.Slides))
	}
	// Slide 0 → masterA (Bold=true, no FontSize).
	slide0 := pres.Slides[0]
	if len(slide0.Shapes) == 0 || len(slide0.Shapes[0].Paragraphs) == 0 || len(slide0.Shapes[0].Paragraphs[0].Runs) == 0 {
		t.Fatal("slide 0 has no runs")
	}
	run0 := slide0.Shapes[0].Paragraphs[0].Runs[0]
	if !run0.Bold {
		t.Errorf("slide 0 run Bold = %v, want true (from masterA)", run0.Bold)
	}
	if run0.FontSize != 0 {
		t.Errorf("slide 0 run FontSize = %d, want 0 (masterA has no FontSize)", run0.FontSize)
	}
	// Slide 1 → masterB (FontSize=2400, no Bold).
	slide1 := pres.Slides[1]
	if len(slide1.Shapes) == 0 || len(slide1.Shapes[0].Paragraphs) == 0 || len(slide1.Shapes[0].Paragraphs[0].Runs) == 0 {
		t.Fatal("slide 1 has no runs")
	}
	run1 := slide1.Shapes[0].Paragraphs[0].Runs[0]
	if run1.FontSize != 2400 {
		t.Errorf("slide 1 run FontSize = %d, want 2400 (from masterB)", run1.FontSize)
	}
	if run1.Bold {
		t.Errorf("slide 1 run Bold = %v, want false (masterB has no Bold)", run1.Bold)
	}
}

func TestBuildPresentationPreservesLegacySlideIdentity(t *testing.T) {
	pres := buildPresentationWithMasterComposition([]legacySlideText{{
		PersistID:  7,
		MasterID:   -2147483648,
		LayoutType: "titleBody",
		Fragments:  []legacySlideFragment{{Text: "Agenda", TextType: pptTextTypeBody}},
	}}, nil, nil, legacyPptHeaderFooterInfo{}, "test.ppt", nil, nil, nil, "", nil)
	if len(pres.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(pres.Slides))
	}
	if pres.Slides[0].MasterID != -2147483648 {
		t.Fatalf("MasterID = %d, want -2147483648", pres.Slides[0].MasterID)
	}
	if pres.Slides[0].LayoutType != "titleBody" {
		t.Fatalf("LayoutType = %q, want titleBody", pres.Slides[0].LayoutType)
	}
}

func TestParseLegacyPptTextShapesFromSlidePreservesOriginalShapeID(t *testing.T) {
	clientAnchor := make([]byte, 8)
	binary.LittleEndian.PutUint16(clientAnchor[0:2], 720)
	binary.LittleEndian.PutUint16(clientAnchor[2:4], 720)
	binary.LittleEndian.PutUint16(clientAnchor[4:6], 2160)
	binary.LittleEndian.PutUint16(clientAnchor[6:8], 1800)
	container := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F,
		append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x401, 0x00000001)),
				buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, clientAnchor)...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000,
				append(
					buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeBody)),
					buildPptAtomRecord(0, pptRTTextBytesAtom, []byte("Hello"))...,
				),
			)...,
		),
	)
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x0F, container)

	shapes := buildLegacyPptTextShapesFromMetas(collectLegacyPptVisualScene(payload).textMetas(nil, nil, nil), legacySlideText{})
	if len(shapes) != 1 {
		t.Fatalf("len(shapes) = %d, want 1", len(shapes))
	}
	if shapes[0].OriginalShapeID != 0x401 {
		t.Fatalf("OriginalShapeID = %d, want 1025", shapes[0].OriginalShapeID)
	}
}

// --- Text decode edge-case regressions (mixed-encoding parity) ---

func TestDecodePptUTF16HandlesSurrogatePairs(t *testing.T) {
	// U+1F600 = 😀 encoded as D83D DE00 in UTF-16LE.
	data := []byte{0x3D, 0xD8, 0x00, 0xDE}
	got := decodePptUTF16(data)
	want := "\U0001F600"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDecodePptUTF16SkipsEmbeddedNulls(t *testing.T) {
	// Embedded null code units are dropped (common in PPT text atoms).
	data := []byte{'H', 0, 0, 0, 'i', 0}
	got := decodePptUTF16(data)
	want := "Hi"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDecodePptUTF16OddLengthTruncates(t *testing.T) {
	// Odd-length payload drops the trailing byte to stay aligned.
	data := []byte{'A', 0, 'B'}
	got := decodePptUTF16(data)
	want := "A"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDecodePptBytesExtendedCharacters(t *testing.T) {
	// decodePptBytes maps each byte directly to a rune (Latin-1 compatible).
	// This is the baseline behavior for TextBytesAtom until a code-page
	// fixture proves otherwise.
	data := []byte{0x80, 0xA9, 0xE9}
	got := decodePptBytes(data)
	want := "\u0080\u00A9\u00E9"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNormalizePptTextCollapsesDuplicateNewlines(t *testing.T) {
	got := normalizePptText("A\r\r\rB")
	want := "A\nB"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNormalizePptTextHandlesMixedLineEndings(t *testing.T) {
	got := normalizePptText("A\r\nB\rC\u000BD\u000CE")
	want := "A\nB\nC\nD\nE"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
func TestParseLegacyPptConnectorShapeContainerPreservesPresetAndAdjustments(t *testing.T) {
	appendFoptProp := func(dst []byte, opid uint16, value uint32) []byte {
		prop := make([]byte, 6)
		binary.LittleEndian.PutUint16(prop[0:2], opid)
		binary.LittleEndian.PutUint32(prop[2:6], value)
		return append(dst, prop...)
	}
	var fopt []byte
	fopt = appendFoptProp(fopt, legacyOfficeArtPropAdjustValue, 10800)
	fopt = appendFoptProp(fopt, legacyOfficeArtPropAdjustValue+1, 5400)
	container := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			buildOfficeArtRecord(0x22, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xA10, 0)),
			buildOfficeArtRecord(2, legacyOfficeArtFOPT, 0x0003, fopt)...,
		),
		buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, []byte{0, 0, 0, 0, 100, 0, 100, 0})...,
	))
	shape, ok := parseLegacyPptConnectorShapeContainer(container[8:], legacyPptGroupFrame{}, nil)
	if !ok {
		t.Fatal("parseLegacyPptConnectorShapeContainer returned false")
	}
	if got := legacyPptConnectorType(shape.shapeType); got != "bentConnector3" {
		t.Fatalf("connector type = %q, want bentConnector3", got)
	}
	if len(shape.adjustments) != 1 || shape.adjustments[0] != 50000 {
		t.Fatalf("adjustments = %v, want canonical bentConnector3 guide [50000]", shape.adjustments)
	}
}

func TestParseLegacyPptConnectorRulesResolveConnectionSites(t *testing.T) {
	ruleBody := make([]byte, 24)
	binary.LittleEndian.PutUint32(ruleBody[0:4], 1)
	binary.LittleEndian.PutUint32(ruleBody[4:8], 0xB01)
	binary.LittleEndian.PutUint32(ruleBody[8:12], 0xB02)
	binary.LittleEndian.PutUint32(ruleBody[12:16], 0xB03)
	binary.LittleEndian.PutUint32(ruleBody[16:20], 0) // right
	binary.LittleEndian.PutUint32(ruleBody[20:24], 2) // left

	connector := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		buildOfficeArtRecord(0x21, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xB03, 0)),
		buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, []byte{0, 0, 0, 0, 100, 0, 100, 0})...,
	))
	drawingPayload := append(buildOfficeArtRecord(0, legacyOfficeArtFConnectorRule, 0x0001, ruleBody), connector...)
	payload := buildPptContainerRecord(0, pptRTPPDrawing, 0x000F, drawingPayload)

	shapeIndex := map[uint32]*presentationir.Shape{
		0xB01: {OriginalShapeID: 0xB01, Bounds: presentationir.Rect{X: 100, Y: 200, W: 400, H: 200}},
		0xB02: {OriginalShapeID: 0xB02, Bounds: presentationir.Rect{X: 1000, Y: 300, W: 300, H: 400}},
	}
	scene := collectLegacyPptVisualScene(payload)
	var shapes []*presentationir.Shape
	collectLegacyPptConnectorShapesFromEvents(scene.dispatch.containers.officeArtEvents(), legacyPptGroupFrame{}, nil, scene.dispatch.rules, shapeIndex, &shapes)
	if len(shapes) != 1 {
		t.Fatalf("len(shapes) = %d, want 1", len(shapes))
	}
	shape := shapes[0]
	if shape.ConnectorType != "bentConnector2" {
		t.Fatalf("ConnectorType = %q, want bentConnector2", shape.ConnectorType)
	}
	if shape.ConnectorStart == nil || *shape.ConnectorStart != (presentationir.Point{X: 500, Y: 300}) {
		t.Fatalf("ConnectorStart = %#v, want right-middle of start shape", shape.ConnectorStart)
	}
	if shape.ConnectorEnd == nil || *shape.ConnectorEnd != (presentationir.Point{X: 1000, Y: 500}) {
		t.Fatalf("ConnectorEnd = %#v, want left-middle of end shape", shape.ConnectorEnd)
	}
}

func applyLegacyPptGroupTransformForTest(shape *presentationir.Shape, groupBounds presentationir.Rect, transform legacyPptShapeTransform) {
	if shape == nil {
		return
	}
	presentationir.ApplyVisualTransform(shape, legacyPptCanonicalGroupTransform(legacyPptGroupFrame{
		domain: groupBounds, localTarget: groupBounds, target: groupBounds, valid: true,
	}, transform))
}

func TestApplyLegacyPptGroupTransformComposesRotationFlipAndConnectorPoints(t *testing.T) {
	start := presentationir.Point{X: 100, Y: 300}
	end := presentationir.Point{X: 300, Y: 300}
	shape := &presentationir.Shape{
		Kind:           presentationir.ShapeConnector,
		Bounds:         presentationir.Rect{X: 100, Y: 200, W: 200, H: 200},
		RotationDeg:    15,
		FlipH:          true,
		ConnectorFlipH: true,
		ConnectorStart: &start,
		ConnectorEnd:   &end,
		ConnectorPath:  []presentationir.Point{start, {X: 200, Y: 250}, end},
	}
	transform := legacyPptShapeTransform{HasRotation: true, Rotation: 90, FlipH: true, FlipV: true}
	applyLegacyPptGroupTransformForTest(shape, presentationir.Rect{X: 0, Y: 0, W: 1000, H: 1000}, transform)

	if shape.Bounds != (presentationir.Rect{X: 200, Y: 700, W: 200, H: 200}) {
		t.Fatalf("Bounds = %+v, want x=200 y=700 w=200 h=200", shape.Bounds)
	}
	if shape.RotationDeg != 105 {
		t.Fatalf("RotationDeg = %v, want 105", shape.RotationDeg)
	}
	if shape.FlipH || !shape.FlipV {
		t.Fatalf("shape flips = (%t,%t), want (false,true)", shape.FlipH, shape.FlipV)
	}
	if shape.ConnectorFlipH || !shape.ConnectorFlipV {
		t.Fatalf("connector flips = (%t,%t), want (false,true)", shape.ConnectorFlipH, shape.ConnectorFlipV)
	}
	if shape.ConnectorStart == nil || *shape.ConnectorStart != (presentationir.Point{X: 300, Y: 900}) {
		t.Fatalf("ConnectorStart = %#v, want transformed point (300,900)", shape.ConnectorStart)
	}
	if shape.ConnectorEnd == nil || *shape.ConnectorEnd != (presentationir.Point{X: 300, Y: 700}) {
		t.Fatalf("ConnectorEnd = %#v, want transformed point (300,700)", shape.ConnectorEnd)
	}
	if len(shape.ConnectorPath) != 3 || shape.ConnectorPath[1] != (presentationir.Point{X: 250, Y: 800}) {
		t.Fatalf("ConnectorPath = %+v, want transformed middle point", shape.ConnectorPath)
	}
}

func TestApplyLegacyPptGroupTransformReversesChildRotationUnderSingleAxisReflection(t *testing.T) {
	shape := &presentationir.Shape{
		Kind:        presentationir.ShapeImage,
		Bounds:      presentationir.Rect{X: 100, Y: 200, W: 200, H: 100},
		RotationDeg: 20,
	}
	applyLegacyPptGroupTransformForTest(shape, presentationir.Rect{X: 0, Y: 0, W: 1000, H: 1000}, legacyPptShapeTransform{
		HasRotation: true,
		Rotation:    90,
		FlipH:       true,
	})

	if shape.RotationDeg != 70 {
		t.Fatalf("RotationDeg = %v, want 70 (90 parent - 20 reflected child)", shape.RotationDeg)
	}
	if !shape.FlipH || shape.FlipV {
		t.Fatalf("shape flips = (%t,%t), want (true,false)", shape.FlipH, shape.FlipV)
	}
}

func TestLegacyPptNestedGroupTransformsAccumulateAcrossDescendants(t *testing.T) {
	child := &presentationir.Shape{Kind: presentationir.ShapeText, Bounds: presentationir.Rect{X: 100, Y: 100, W: 100, H: 100}, RotationDeg: 5}
	inner := &presentationir.Shape{Kind: presentationir.ShapeGroup, Bounds: presentationir.Rect{X: 50, Y: 50, W: 400, H: 400}, Children: []*presentationir.Shape{child}}
	applyLegacyPptGroupTransformForTest(child, inner.Bounds, legacyPptShapeTransform{HasRotation: true, Rotation: 30, FlipV: true})
	outerTransform := legacyPptShapeTransform{HasRotation: true, Rotation: 60, FlipH: true}
	applyLegacyPptGroupTransformForTest(inner, presentationir.Rect{X: 0, Y: 0, W: 1000, H: 1000}, outerTransform)

	if child.RotationDeg != 35 {
		t.Fatalf("child RotationDeg = %v, want 35 after reflected nested composition", child.RotationDeg)
	}
	if !child.FlipH || !child.FlipV {
		t.Fatalf("child flips = (%t,%t), want both true", child.FlipH, child.FlipV)
	}
	if inner.RotationDeg != 60 || !inner.FlipH {
		t.Fatalf("inner transform = rotation %v flipH %t, want 60/true", inner.RotationDeg, inner.FlipH)
	}
}

func TestParseLegacyPptGroupShapePreservesRootTransformForLayout(t *testing.T) {
	buildRect32 := func(left, top, right, bottom int32) []byte {
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint32(buf[0:4], uint32(left))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(top))
		binary.LittleEndian.PutUint32(buf[8:12], uint32(right))
		binary.LittleEndian.PutUint32(buf[12:16], uint32(bottom))
		return buf
	}
	buildClientAnchor := func(x1, y1, x2, y2 uint16) []byte {
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint16(buf[0:2], x1)
		binary.LittleEndian.PutUint16(buf[2:4], y1)
		binary.LittleEndian.PutUint16(buf[4:6], x2)
		binary.LittleEndian.PutUint16(buf[6:8], y2)
		return buf
	}
	rotationProp := make([]byte, 6)
	binary.LittleEndian.PutUint16(rotationProp[0:2], legacyOfficeArtPropRotation)
	binary.LittleEndian.PutUint32(rotationProp[2:6], uint32(int32(90*65536)))
	rootBody := append(
		append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xC00, legacyOfficeArtSpFlagFlipH)),
				buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, buildRect32(0, 0, 1000, 1000))...,
			),
			buildOfficeArtRecord(1, legacyOfficeArtFOPT, 0x0003, rotationProp)...,
		),
		buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, buildClientAnchor(0, 0, 1000, 1000))...,
	)
	root := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, rootBody)
	childAnchor := buildRect32(100, 200, 300, 400)
	childRotationProp := make([]byte, 6)
	binary.LittleEndian.PutUint16(childRotationProp[0:2], legacyOfficeArtPropRotation)
	binary.LittleEndian.PutUint32(childRotationProp[2:6], uint32(int32(15*65536)))
	childBody := append(
		buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xC01, legacyOfficeArtSpFlagChild|legacyOfficeArtSpFlagFlipV)),
		buildOfficeArtRecord(1, legacyOfficeArtFOPT, 0x0003, childRotationProp)...,
	)
	childBody = append(childBody, buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, childAnchor)...)
	child := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, childBody)
	groupData := append(root, child...)

	frame, ok := parseLegacyPptGroupFrameFromSpContainer(rootBody, legacyPptGroupFrame{})
	if !ok {
		t.Fatal("group frame not parsed")
	}
	untransformed, ok := mapLegacyPptRectThroughGroupFrame(presentationir.Rect{X: 100, Y: 200, W: 200, H: 200}, frame)
	if !ok {
		t.Fatal("child anchor not mapped")
	}
	resolved := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 0xC01, Bounds: untransformed}
	drawingPayload := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, groupData)
	scene := collectLegacyPptVisualScene(buildPptContainerRecord(0, pptRTPPDrawing, 0x000F, drawingPayload))
	groups := buildLegacyPptGroupShapesFromSceneWithOwnership(scene.dispatch.containers, legacyPptDiscoveredShapeSource{0xC01: resolved}, nil, nil, nil, nil)
	if len(groups) != 1 {
		t.Fatalf("semantic scene groups = %d, want 1", len(groups))
	}
	group := groups[0]
	if len(group.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(group.Children))
	}
	if group.GroupTransform == nil {
		t.Fatal("group transform = nil, want canonical transform preserved for layout")
	}
	if group.GroupTransform.Rotation != 90 || !group.GroupTransform.FlipH {
		t.Fatalf("group transform rotation=%v flipH=%v, want 90/true", group.GroupTransform.Rotation, group.GroupTransform.FlipH)
	}
	got := group.Children[0]
	// A single-axis reflection conjugates the local legacy rotation. The group
	// transform itself remains unmaterialized for layout, but the child leaf is
	// already expressed in canonical DrawingML-style transform semantics.
	if got.RotationDeg != -15 || got.FlipH || !got.FlipV {
		t.Fatalf("child-local canonical transform rotation=%v flipH=%v flipV=%v; want -15/false/true", got.RotationDeg, got.FlipH, got.FlipV)
	}
	wantLocal := presentationir.Rect{X: 100, Y: 200, W: 200, H: 200}
	if got.Bounds != wantLocal {
		t.Fatalf("parser child bounds = %+v, want authored local bounds %+v", got.Bounds, wantLocal)
	}
	if group.GroupTransform.ChildOffset != (presentationir.Point{X: 0, Y: 0}) ||
		group.GroupTransform.ChildExtent != (presentationir.VisualSize{W: 1000, H: 1000}) {
		t.Fatalf("group child coordinate system = off %+v ext %+v, want authored 0,0 / 1000x1000",
			group.GroupTransform.ChildOffset, group.GroupTransform.ChildExtent)
	}
}

func TestLegacyPptNestedGroupFramePreservesImmediateParentAnchor(t *testing.T) {
	parent := legacyPptGroupFrame{
		domain:      presentationir.Rect{X: 0, Y: 0, W: 1000, H: 1000},
		localTarget: presentationir.Rect{X: 0, Y: 0, W: 2000, H: 1000},
		target:      presentationir.Rect{X: 0, Y: 0, W: 2000, H: 1000},
		valid:       true,
	}
	local := presentationir.Rect{X: 100, Y: 200, W: 300, H: 400}
	mapped, ok := mapLegacyPptRectThroughGroupFrame(local, parent)
	if !ok {
		t.Fatal("expected parent frame mapping")
	}
	frame := legacyPptGroupFrame{
		domain:      presentationir.Rect{X: 10, Y: 20, W: 30, H: 40},
		localTarget: local,
		target:      mapped,
		valid:       true,
	}
	transform := legacyPptCanonicalGroupTransform(frame, legacyPptShapeTransform{})
	if transform.Offset != (presentationir.Point{X: local.X, Y: local.Y}) {
		t.Fatalf("offset = %+v, want immediate-parent local anchor %+v", transform.Offset, local)
	}
	if transform.ChildOffset != (presentationir.Point{X: 10, Y: 20}) ||
		transform.ChildExtent != (presentationir.VisualSize{W: 30, H: 40}) {
		t.Fatalf("child coordinate system = off %+v ext %+v", transform.ChildOffset, transform.ChildExtent)
	}
	if frame.target == frame.localTarget {
		t.Fatal("test requires slide-space discovery target to differ from canonical local target")
	}
}

func TestParseLegacyPptGenericLinkedVideoContainer(t *testing.T) {
	mediaAtom := buildPptAtomRecord(0, pptRTExMediaAtom, marshalUint32LE(42))
	name := buildPptAtomRecord(0, pptRTCString, encodeLegacyUTF16("Demo clip"))
	path := buildPptAtomRecord(1, pptRTCString, encodeLegacyUTF16("C:\\media\\demo.avi"))
	container := append(mediaAtom, name...)
	container = append(container, path...)

	id, ext, ok := parseLegacyPptGenericMediaContainer(container)
	if !ok {
		t.Fatal("expected generic media container")
	}
	if id != 42 || ext.Kind != presentationir.ShapeMedia || ext.MediaKind != "video" {
		t.Fatalf("id=%d ext=%+v, want linked video 42", id, ext)
	}
	if ext.MIME != "video/x-msvideo" || ext.LinkURL != "C:\\media\\demo.avi" {
		t.Fatalf("mime=%q link=%q", ext.MIME, ext.LinkURL)
	}
}

func TestParseLegacyPptGenericLinkedMidiContainer(t *testing.T) {
	container := append(
		buildPptAtomRecord(0, pptRTExMediaAtom, marshalUint32LE(7)),
		buildPptAtomRecord(0, pptRTCString, encodeLegacyUTF16("https://example.test/theme.mid"))...,
	)
	id, ext, ok := parseLegacyPptGenericMediaContainer(container)
	if !ok || id != 7 {
		t.Fatalf("id=%d ok=%v", id, ok)
	}
	if ext.MediaKind != "audio" || ext.MIME != "audio/midi" {
		t.Fatalf("media=%q mime=%q", ext.MediaKind, ext.MIME)
	}
}

func TestWalkPptRecordsRecoversAfterMalformedOptionalRecord(t *testing.T) {
	first := buildPptAtomRecord(0, pptRTCString, encodeLegacyUTF16("before"))
	bad := []byte{0x00, 0x00, 0x34, 0x12, 0xff, 0xff, 0xff, 0x7f, 0xde, 0xad, 0xbe, 0xef}
	last := buildPptAtomRecord(0, pptRTCString, encodeLegacyUTF16("after"))
	data := append(append(first, bad...), last...)

	var got []string
	walkPptRecords(data, func(record pptRecord, payload []byte) {
		if record.Type == pptRTCString {
			got = append(got, normalizePptText(decodePptUTF16(payload)))
		}
	})
	if len(got) != 2 || got[0] != "before" || got[1] != "after" {
		t.Fatalf("recovered strings = %v, want [before after]", got)
	}
}

func TestParseLegacyPptShapeAnimationInfoMergesAtomsAndDelay(t *testing.T) {
	firstPayload := make([]byte, 28)
	binary.LittleEndian.PutUint16(firstPayload[4:6], 1<<2) // automatic
	binary.LittleEndian.PutUint32(firstPayload[12:16], 250)
	binary.LittleEndian.PutUint16(firstPayload[16:18], 4)
	firstPayload[20] = 1
	firstPayload[21] = 1
	secondPayload := make([]byte, 28)
	binary.LittleEndian.PutUint16(secondPayload[16:18], 2)
	secondPayload[20] = 1
	secondPayload[21] = 2

	data := append(
		buildPptAtomRecord(0, pptRTAnimationInfoAtom, firstPayload),
		buildPptAtomRecord(0, pptRTAnimationInfoAtom, secondPayload)...,
	)
	info, ok := parseLegacyPptShapeAnimationInfo(data)
	if !ok {
		t.Fatal("expected merged animation info")
	}
	if info.Step != 2 || info.DelayMs != 250 || info.Effect == "" {
		t.Fatalf("info=%+v", info)
	}
	shape := &presentationir.Shape{}
	applyLegacyPptAnimationMetadata(shape, info)
	if shape.AnimationDelayMs != 250 {
		t.Fatalf("AnimationDelayMs=%d, want 250", shape.AnimationDelayMs)
	}
}

func TestLegacyPptMediaKindAndMIMERecognizesVideoAndAudioFamilies(t *testing.T) {
	cases := []struct{ name, kind, mime string }{
		{"movie.avi", "video", "video/x-msvideo"},
		{"movie.mpg", "video", "video/mpeg"},
		{"movie.wmv", "video", "video/x-ms-wmv"},
		{"movie.mov", "video", "video/quicktime"},
		{"music.mid", "audio", "audio/midi"},
		{"music.wma", "audio", "audio/x-ms-wma"},
	}
	for _, tc := range cases {
		kind, mime := legacyPptMediaKindAndMIME(nil, tc.name)
		if kind != tc.kind || mime != tc.mime {
			t.Fatalf("%s => %s/%s, want %s/%s", tc.name, kind, mime, tc.kind, tc.mime)
		}
	}
}

func TestLegacyPptFeatureCorpusMatrix(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "testdata", "ppt")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("legacy PPT corpus unavailable: %v", err)
	}
	cases := []struct {
		name  string
		file  string
		check func(*presentationir.Presentation) bool
	}{
		{"ppt95", "PPT95.ppt", func(p *presentationir.Presentation) bool { return p != nil && len(p.Slides) > 0 }},
		{"ppt97", "basic_test_ppt_file.ppt", func(p *presentationir.Presentation) bool { return p != nil && len(p.Slides) > 0 }},
		{"ppt2000", "SampleShow.ppt", func(p *presentationir.Presentation) bool { return p != nil && len(p.Slides) > 0 }},
		{"ppt2003", "bug58144-headers-footers-2003.ppt", func(p *presentationir.Presentation) bool { return p != nil && len(p.Slides) > 0 }},
		{"hyperlinks", "WithLinks.ppt", func(p *presentationir.Presentation) bool {
			return legacyCorpusHasShape(p, func(s *presentationir.Shape) bool {
				if s.HyperlinkURL != "" || s.ActionLink != "" {
					return true
				}
				for _, para := range s.Paragraphs {
					for _, run := range para.Runs {
						if run != nil && run.HyperlinkURL != "" {
							return true
						}
					}
				}
				return false
			})
		}},
		{"tables", "table_test.ppt", func(p *presentationir.Presentation) bool {
			return legacyCorpusHasShape(p, func(s *presentationir.Shape) bool { return s.Kind == presentationir.ShapeTable && s.Table != nil })
		}},
		{"audio", "sound.ppt", func(p *presentationir.Presentation) bool {
			return legacyCorpusHasShape(p, func(s *presentationir.Shape) bool {
				return s.Kind == presentationir.ShapeMedia && s.MediaKind == "audio"
			})
		}},
		{"ole", "embedded_excel_ole_chart.ppt", func(p *presentationir.Presentation) bool {
			return legacyCorpusHasShape(p, func(s *presentationir.Shape) bool { return s.Kind == presentationir.ShapeOleObject })
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, tc.file)
			if _, err := os.Stat(path); err != nil {
				t.Skipf("fixture unavailable: %v", err)
			}
			p, err := ParsePpt(path)
			if err != nil {
				t.Fatalf("ParsePpt(%s): %v", tc.file, err)
			}
			if !tc.check(p) {
				t.Fatalf("%s did not satisfy feature assertion", tc.file)
			}
		})
	}
}

func legacyCorpusHasShape(p *presentationir.Presentation, pred func(*presentationir.Shape) bool) bool {
	if p == nil {
		return false
	}
	var walk func([]*presentationir.Shape) bool
	walk = func(shapes []*presentationir.Shape) bool {
		for _, shape := range shapes {
			if shape == nil {
				continue
			}
			if pred(shape) || walk(shape.Children) {
				return true
			}
		}
		return false
	}
	for _, slide := range p.Slides {
		if slide != nil && walk(slide.Shapes) {
			return true
		}
	}
	return false
}

func TestParseLegacyPptExternalObjectsAcceptsOleContractUnderUnknownContainer(t *testing.T) {
	atomPayload := make([]byte, 20)
	binary.LittleEndian.PutUint32(atomPayload[8:12], 88)
	containerPayload := append(
		buildPptAtomRecord(0, pptRTExOleObjAtom, atomPayload),
		buildPptAtomRecord(2, pptRTCString, encodeLegacyUTF16("Package"))...,
	)
	unknownContainer := buildPptContainerRecord(0, 0x10FE, 0x0F, containerPayload)
	exObjList := buildPptContainerRecord(0, pptRTExObjList, 0x0F, unknownContainer)
	document := buildPptContainerRecord(0, pptRTDocument, 0x0F, exObjList)

	objects := parseLegacyPptExternalObjectsFromPayload(mustLegacyPptContainerPayload(t, document, pptRTDocument), document, nil)
	obj, ok := objects[88]
	if !ok || obj.Kind != presentationir.ShapeOleObject || obj.ProgID != "Package" {
		t.Fatalf("objects[88]=%+v ok=%v", obj, ok)
	}
}

func TestBuildLegacySlideAnimationTimelineCarriesDelay(t *testing.T) {
	shape := &presentationir.Shape{ID: 9, AnimStep: 2, AnimationTriggers: []string{"afterPrevious"}, AnimationDelayMs: 325}
	events := buildLegacySlideAnimationTimeline(nil, []*presentationir.Shape{shape}, nil, nil, nil)
	if len(events) != 1 {
		t.Fatalf("events=%d, want 1", len(events))
	}
	if events[0].ShapeID != 9 || events[0].Step != 2 || events[0].DelayMs != 325 || events[0].Trigger != "afterPrevious" {
		t.Fatalf("event=%+v", events[0])
	}
}

func TestLegacyPptNoLineDoesNotDiscardOfficeArtTextAndCropSemantics(t *testing.T) {
	shape := &presentationir.Shape{}
	tr := legacyPptShapeTransform{
		HasLineEnable:   true,
		LineEnabled:     false,
		HasTextAnchor:   true,
		TextAnchor:      "ctr",
		HasTextInsets:   true,
		TextInsetLeft:   100,
		TextInsetTop:    200,
		TextInsetRight:  300,
		TextInsetBottom: 400,
		HasAutoGrow:     true,
		AutoGrow:        true,
		HasImageCrop:    true,
		CropLeft:        .1,
		CropTop:         .2,
		CropRight:       .3,
		CropBottom:      .4,
	}
	applyLegacyPptShapeTransform(shape, tr)

	if shape.LineStyle != nil || shape.LineColor != "" || shape.LineWidthPt != 0 {
		t.Fatalf("disabled line leaked into shape: %+v", shape)
	}
	if shape.TextAnchor != "ctr" || !shape.HasTextInsets ||
		shape.TextInsetLeftEMU != 100 || shape.TextInsetTopEMU != 200 ||
		shape.TextInsetRightEMU != 300 || shape.TextInsetBottomEMU != 400 ||
		!shape.TextAutoGrow {
		t.Fatalf("text semantics lost when line disabled: %+v", shape)
	}
	if shape.ImageCropLeft != .1 || shape.ImageCropTop != .2 ||
		shape.ImageCropRight != .3 || shape.ImageCropBottom != .4 {
		t.Fatalf("crop semantics lost when line disabled: %+v", shape)
	}
}

func TestLegacyPptFOPTProjectsSharedGeometryText(t *testing.T) {
	props := []officeartescher.Property{
		{
			ID:        officeartescher.PropGTextUnicode,
			IsComplex: true,
			Complex:   []byte{'W', 0, 'o', 0, 'r', 0, 'd', 0, 'A', 0, 'r', 0, 't', 0, 0, 0},
		},
		{
			ID:        officeartescher.PropGTextFont,
			IsComplex: true,
			Complex:   []byte{'A', 0, 'r', 0, 'i', 0, 'a', 0, 'l', 0, 0, 0},
		},
		{ID: officeartescher.PropGTextSize, Value: 28 << 16},
		{ID: officeartescher.PropGTextAlign, Value: 1},
	}
	var tr legacyPptShapeTransform
	parseLegacyPptFOPTPropertiesTransform(props, nil, &tr)
	if !tr.HasGeometryText || tr.GeometryText.Text != "WordArt" {
		t.Fatalf("transform GeoText = %+v", tr)
	}

	shape := &presentationir.Shape{}
	applyLegacyPptShapeTransform(shape, tr)
	if len(shape.Paragraphs) != 1 || len(shape.Paragraphs[0].Runs) != 1 {
		t.Fatalf("paragraphs = %#v", shape.Paragraphs)
	}
	p := shape.Paragraphs[0]
	run := p.Runs[0]
	if p.Align != "ctr" || run.Text != "WordArt" || run.FontFamily != "Arial" || run.FontSize != 2800 {
		t.Fatalf("projected WordArt = paragraph:%+v run:%+v", p, run)
	}
}

func TestLegacyPptGeometryTextDoesNotOverwriteTextAtomParagraphs(t *testing.T) {
	shape := &presentationir.Shape{
		Paragraphs: []*presentationir.Paragraph{{
			Runs: []*presentationir.Run{{Text: "PowerPoint text atom"}},
		}},
	}
	applyLegacyPptShapeTransform(shape, legacyPptShapeTransform{
		HasGeometryText: true,
		GeometryText: officeartlegacy.GeometryText{
			Present: true,
			Text:    "WordArt fallback",
		},
	})
	if got := shape.Paragraphs[0].Runs[0].Text; got != "PowerPoint text atom" {
		t.Fatalf("existing text overwritten by GeoText: %q", got)
	}
}

func TestLegacyPptTableDiscoveryDoesNotConsumeOLEPreviewDrawing(t *testing.T) {
	exObj := make([]byte, 4)
	binary.LittleEndian.PutUint32(exObj, 17)
	clientData := buildOfficeArtRecord(0, legacyOfficeArtClientData, 0x0000,
		buildPptAtomRecord(0, pptRTExObjRefAtom, exObj))
	shape := buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x701, 0))
	sp := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(shape, clientData...))
	group := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, sp)

	if !legacyPptOfficeArtContainsExternalObject(group) {
		t.Fatal("OLE preview group was not recognized as externally-owned drawing content")
	}
	if table, ok := recoverLegacyPptTableShapeFromGroupContainer(group, legacyPptGroupFrame{}, nil, nil, nil, 1); ok || table != nil {
		t.Fatalf("OLE preview group promoted to slide table: %+v", table)
	}
}

func TestComposeLegacyPptMasterVisualsPaintsMasterBehindLocalShapes(t *testing.T) {
	master := &presentationir.Shape{Kind: presentationir.ShapeImage, Name: "master artwork"}
	local := &presentationir.Shape{Kind: presentationir.ShapeText, Name: "local title"}
	slides := []legacySlideText{{PersistID: 7, MasterID: 42, MasterPersistID: 142}}
	got := composeLegacyPptMasterVisuals(slides, map[int][]*presentationir.Shape{7: {local}}, map[int][]*presentationir.Shape{142: {master}})
	if len(got[7]) != 2 {
		t.Fatalf("composed shape count = %d, want 2", len(got[7]))
	}
	if got[7][0] == master {
		t.Fatal("master artwork must be materialized as a per-slide instance")
	}
	if got[7][0].Kind != master.Kind || got[7][0].Name != master.Name {
		t.Fatalf("first shape = %+v, want cloned master artwork", got[7][0])
	}
	if got[7][1] != local {
		t.Fatal("master artwork must paint before slide-local content")
	}
}

func TestComposeLegacyPptMasterVisualsDoesNotBorrowUnreferencedMaster(t *testing.T) {
	master := &presentationir.Shape{Kind: presentationir.ShapeImage, Name: "other master artwork"}
	local := &presentationir.Shape{Kind: presentationir.ShapeText, Name: "local title"}
	slides := []legacySlideText{{PersistID: 7, MasterID: 99}}
	got := composeLegacyPptMasterVisuals(slides, map[int][]*presentationir.Shape{7: {local}}, map[int][]*presentationir.Shape{142: {master}})
	if len(got[7]) != 1 || got[7][0] != local {
		t.Fatal("slide must inherit artwork only from its referenced master")
	}
}

func TestLegacyMasterTextRejectsSemanticallyImpossibleInheritance(t *testing.T) {
	truth := true
	bad := legacyPptMasterTextLevel{
		paragraph: stpaPFEntry{HasBullet: true, BulletChar: 0x18},
		run: stpaCFRun{
			Bold: &truth, Italic: &truth, Underline: &truth, Strike: &truth,
			FontSize: 72000,
		},
	}
	if legacyPptMasterTextLevelSemanticallyValid(bad) {
		t.Fatal("misdecoded master text level must not enter the inheritance cascade")
	}
	good := legacyPptMasterTextLevel{run: stpaCFRun{FontSize: 1800}}
	if !legacyPptMasterTextLevelSemanticallyValid(good) {
		t.Fatal("ordinary 18pt master text level was rejected")
	}
}

func TestLegacyHeuristicTableGeometryMustRemainInsideOwningGroup(t *testing.T) {
	frame := presentationir.Rect{X: 100, Y: 100, W: 1000, H: 800}
	inside := presentationir.Rect{X: 110, Y: 120, W: 950, H: 740}
	if !legacyPptRectContainedByFrame(inside, frame) {
		t.Fatal("valid table geometry inside group frame was rejected")
	}
	escaped := presentationir.Rect{X: 600, Y: 500, W: 900, H: 700}
	if legacyPptRectContainedByFrame(escaped, frame) {
		t.Fatal("escaped embedded drawing geometry must not become a heuristic slide table")
	}
}

func PutInt16(dst []byte, value int) {
	binary.LittleEndian.PutUint16(dst, uint16(int16(value)))
}

func TestParseLegacyPptClientAnchorUsesPowerPointTopLeftRightBottomOrder(t *testing.T) {
	body := make([]byte, 8)
	PutInt16(body[0:2], 100) // top
	PutInt16(body[2:4], 200) // left
	PutInt16(body[4:6], 600) // right
	PutInt16(body[6:8], 500) // bottom

	got, ok := parseLegacyPptClientAnchor(body)
	if !ok {
		t.Fatal("parseLegacyPptClientAnchor failed")
	}
	want := presentationir.Rect{
		X: 200 * defaultSlideWidthEMU / legacyPptMasterWidth,
		Y: 100 * defaultSlideHeightEMU / legacyPptMasterHeight,
		W: 400 * defaultSlideWidthEMU / legacyPptMasterWidth,
		H: 400 * defaultSlideHeightEMU / legacyPptMasterHeight,
	}
	if got != want {
		t.Fatalf("bounds = %+v, want %+v", got, want)
	}
}

func TestParseLegacyPptClientAnchorPreservesSignedCoordinates(t *testing.T) {
	body := make([]byte, 8)
	top := -100
	PutInt16(body[0:2], top) // top
	PutInt16(body[2:4], 200)
	PutInt16(body[4:6], 600)
	PutInt16(body[6:8], 500)
	got, ok := parseLegacyPptClientAnchor(body)
	if !ok {
		t.Fatal("parseLegacyPptClientAnchor failed")
	}
	if got.Y >= 0 {
		t.Fatalf("Y = %d, want negative signed coordinate", got.Y)
	}
}

func TestLegacyPptBuiltInTitleFallbackDoesNotInventCharacterFormatting(t *testing.T) {
	got, ok := legacyPptBuiltInMasterTextLevel(pptTextTypeTitle)
	if !ok {
		t.Fatal("title fallback missing")
	}
	if got.paragraph.Align != "ctr" {
		t.Fatalf("align = %q, want ctr", got.paragraph.Align)
	}
	if got.run.FontSize != 0 || got.run.FontFamily != "" || got.run.Bold != nil || got.run.Color != "" {
		t.Fatalf("built-in title fallback invented character formatting: %+v", got.run)
	}
}

func TestLegacyPptGroupClassificationRequiresAuthoredTableProperty(t *testing.T) {
	groupRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F,
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0x900, 0x01)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, make([]byte, 16))...,
		),
	)
	group := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, groupRoot)
	payload := group[8:]

	if legacyPptGroupContainerIsTable(payload) {
		t.Fatal("unmarked OfficeArt group classified as an authored table")
	}
	if recoverLegacyPptGroupIsTableLike(payload, legacyPptGroupFrame{}, nil, nil, nil) {
		t.Fatal("geometric/group structure must not promote an unmarked OfficeArt group to table semantics")
	}
	if table, ok := recoverLegacyPptTableShapeFromGroupContainer(payload, legacyPptGroupFrame{}, nil, nil, nil, 1); ok || table != nil {
		t.Fatalf("unmarked OfficeArt group materialized as table: %+v", table)
	}
}

func TestLegacyPptGroupChildSourceCannotOwnCanonicalGeometry(t *testing.T) {
	resolved := &presentationir.Shape{
		Kind:            presentationir.ShapeText,
		OriginalShapeID: 0xC01,
		Bounds:          presentationir.Rect{X: 9000, Y: 9000, W: 9000, H: 9000},
		Paragraphs:      []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "semantic payload"}}}},
	}
	source := legacyPptDiscoveredShapeSource{0xC01: resolved}
	child := source.CloneSemanticPayload(0xC01)
	if child == nil {
		t.Fatal("CloneSemanticPayload returned nil")
	}
	if child.Bounds != (presentationir.Rect{}) || child.RotationDeg != 0 || child.FlipH || child.FlipV {
		t.Fatalf("semantic payload leaked discovery geometry/transform: bounds=%+v rotation=%v flipH=%v flipV=%v", child.Bounds, child.RotationDeg, child.FlipH, child.FlipV)
	}
	child.Bounds = presentationir.Rect{X: 10, Y: 20, W: 30, H: 40}
	if resolved.Bounds == child.Bounds {
		t.Fatal("semantic child source leaked canonical geometry mutation back into discovery shape")
	}
	if len(child.Paragraphs) == 0 {
		t.Fatal("semantic payload was not preserved")
	}
}

func TestLegacyPptUnmarkedDiagramWithTextAndLinesIsNotPromotedToTable(t *testing.T) {
	buildRect32 := func(left, top, right, bottom int32) []byte {
		buf := make([]byte, 16)
		binary.LittleEndian.PutUint32(buf[0:4], uint32(left))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(top))
		binary.LittleEndian.PutUint32(buf[8:12], uint32(right))
		binary.LittleEndian.PutUint32(buf[12:16], uint32(bottom))
		return buf
	}
	buildClientAnchor := func(x1, y1, x2, y2 uint16) []byte {
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint16(buf[0:2], x1)
		binary.LittleEndian.PutUint16(buf[2:4], y1)
		binary.LittleEndian.PutUint16(buf[4:6], x2)
		binary.LittleEndian.PutUint16(buf[6:8], y2)
		return buf
	}
	buildText := func(shapeID uint32, left, top, right, bottom int32, text string) []byte {
		textbox := append(
			buildPptAtomRecord(0, pptRTTextHeaderAtom, marshalUint32LE(pptTextTypeOther)),
			buildPptAtomRecord(0, pptRTTextBytesAtom, []byte(text))...,
		)
		return buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
			append(
				buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(shapeID, legacyOfficeArtSpFlagChild)),
				buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, buildRect32(left, top, right, bottom))...,
			),
			buildOfficeArtRecord(0, legacyOfficeArtClientTextbox, 0x0000, textbox)...,
		))
	}
	buildLine := func(shapeID uint32, left, top, right, bottom int32) []byte {
		return buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
			buildOfficeArtRecord(20, legacyOfficeArtSp, 0x0002, officeArtSpPayload(shapeID, legacyOfficeArtSpFlagChild)),
			buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, buildRect32(left, top, right, bottom))...,
		))
	}

	groupRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xA01, 0x01)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, buildRect32(0, 0, 1000, 800))...,
		),
		buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0x0000, buildClientAnchor(0, 0, 2000, 1600))...,
	))
	payload := append([]byte{}, groupRoot...)
	payload = append(payload, buildText(0xA02, 0, 0, 500, 300, "Space Segment")...)
	payload = append(payload, buildText(0xA03, 500, 300, 1000, 600, "Control Segment")...)
	payload = append(payload, buildText(0xA04, 250, 600, 750, 800, "User Segment")...)
	payload = append(payload, buildLine(0xA05, 0, 300, 1000, 300)...)
	payload = append(payload, buildLine(0xA06, 500, 0, 500, 800)...)

	if legacyPptGroupContainerIsTable(payload) {
		t.Fatal("unmarked diagram unexpectedly carries authored table semantics")
	}
	table, ok := recoverLegacyPptTableShapeFromGroupContainer(payload, legacyPptGroupFrame{}, nil, nil, nil, 1)
	if !ok || table == nil {
		t.Fatal("expected legacy compatibility inference to produce a table candidate")
	}
	flat := map[int][]*presentationir.Shape{1: {
		{Kind: presentationir.ShapeText, Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "Space Segment"}}}}},
		{Kind: presentationir.ShapeText, Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "Control Segment"}}}}},
		{Kind: presentationir.ShapeText, Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "User Segment"}}}}},
	}}
	tables := map[int][]*presentationir.Shape{1: {table}}
	if got := filterLegacyDuplicateProjectedTables(flat, tables); len(got[1]) != 0 {
		t.Fatalf("duplicate diagram table candidate survived sibling-provenance filter: %+v", got[1])
	}
}

func TestLegacyDuplicateProjectedTableMatchesParagraphAtomsAcrossGroupProjection(t *testing.T) {
	table := &presentationir.Shape{Kind: presentationir.ShapeTable, Table: &presentationir.Table{Rows: []*presentationir.TableRow{{Cells: []*presentationir.TableCell{{Paragraphs: []*presentationir.Paragraph{
		{Runs: []*presentationir.Run{{Text: "Broadcast of:"}}},
		{Runs: []*presentationir.Run{{Text: "Navigation   Data"}}},
		{Runs: []*presentationir.Run{{Text: "Spread-spectrum ranging signals"}}},
	}}}}}}}
	groups := map[int][]*presentationir.Shape{1: {{Kind: presentationir.ShapeGroup, Children: []*presentationir.Shape{
		{Kind: presentationir.ShapeText, Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "Broadcast of:"}}}}},
		{Kind: presentationir.ShapeText, Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "Navigation Data"}}}}},
		{Kind: presentationir.ShapeText, Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "Spread-spectrum ranging signals"}}}}},
	}}}}
	tables := map[int][]*presentationir.Shape{1: {table}}
	if got := filterLegacyDuplicateProjectedTables(groups, tables); len(got[1]) != 0 {
		t.Fatalf("group-owned paragraph projection did not suppress inferred duplicate table: %+v", got[1])
	}
}

func TestLegacyPptTextFillRequiresExplicitFilledBoolean(t *testing.T) {
	props := []officeartescher.Property{{ID: legacyOfficeArtPropFillColor, Value: 0x00000000}}
	if color, ok := parseLegacyPptFOPTPropertiesTextFillColor(props, nil); ok || color != "" {
		t.Fatalf("implicit textbox fill = %q, %v; want transparent", color, ok)
	}
	props = append(props, officeartescher.Property{ID: legacyOfficeArtPropFillBools, Value: (1 << 20) | (1 << 4)})
	if color, ok := parseLegacyPptFOPTPropertiesTextFillColor(props, nil); !ok || color != "000000" {
		t.Fatalf("explicit textbox fill = %q, %v; want 000000,true", color, ok)
	}
}

func TestLegacyPptTextFillIgnoresUnrelatedFillBooleanBits(t *testing.T) {
	props := []officeartescher.Property{
		{ID: legacyOfficeArtPropFillColor, Value: 0x00000000},
		// An unrelated value/use pair is authored, but fFilled (bit 4 / use 20) is not.
		{ID: legacyOfficeArtPropFillBools, Value: (1 << 16) | 1},
	}
	if color, ok := parseLegacyPptFOPTPropertiesTextFillColor(props, nil); ok || color != "" {
		t.Fatalf("implicit textbox fill = %q, %v; want transparent", color, ok)
	}
}

func TestResolveLegacyPptOfficeArtColorRefUsesSchemeFlag(t *testing.T) {
	scheme := map[uint8]string{4: "ABCDEF"}
	if got := resolveLegacyPptOfficeArtColorRef(0x08000004, scheme); got != "ABCDEF" {
		t.Fatalf("scheme OfficeArt color = %q", got)
	}
	if got := resolveLegacyPptOfficeArtColorRef(0x00332211, scheme); got != "112233" {
		t.Fatalf("direct OfficeArt color = %q", got)
	}
}

func TestLegacyPptConnectionSitesRespectTriangleSiteCount(t *testing.T) {
	shape := &presentationir.Shape{Geometry: "triangle", Bounds: presentationir.Rect{X: 100, Y: 200, W: 300, H: 400}}
	want := []presentationir.Point{{X: 250, Y: 200}, {X: 100, Y: 600}, {X: 400, Y: 600}, {X: 250, Y: 200}}
	for site, expected := range want {
		got := legacyPptConnectionSitePointForShape(shape, uint32(site))
		if got == nil || *got != expected {
			t.Fatalf("triangle site %d = %#v, want %#v", site, got, expected)
		}
	}
}

func TestLegacyPptConnectionSitesKeepFourSiteRectangleContract(t *testing.T) {
	shape := &presentationir.Shape{Geometry: "rect", Bounds: presentationir.Rect{X: 100, Y: 200, W: 300, H: 400}}
	want := []presentationir.Point{{X: 250, Y: 200}, {X: 100, Y: 400}, {X: 250, Y: 600}, {X: 400, Y: 400}}
	for site, expected := range want {
		got := legacyPptConnectionSitePointForShape(shape, uint32(site))
		if got == nil || *got != expected {
			t.Fatalf("rect site %d = %#v, want %#v", site, got, expected)
		}
	}
}

func TestLegacyPptConnectionSitesUsePresetSiteCounts(t *testing.T) {
	cases := []struct {
		preset string
		count  uint32
	}{
		{"diamond", 4},
		{"pentagon", 5},
		{"hexagon", 6},
		{"octagon", 8},
	}
	for _, tc := range cases {
		shape := &presentationir.Shape{Geometry: tc.preset, Bounds: presentationir.Rect{X: 100, Y: 200, W: 300, H: 400}}
		first := legacyPptConnectionSitePointForShape(shape, 0)
		wrapped := legacyPptConnectionSitePointForShape(shape, tc.count)
		if first == nil || wrapped == nil || *first != *wrapped {
			t.Fatalf("%s site count did not wrap at %d: first=%#v wrapped=%#v", tc.preset, tc.count, first, wrapped)
		}
		if tc.count > 4 {
			fourth := legacyPptConnectionSitePointForShape(shape, 4)
			if fourth == nil || *fourth == *first {
				t.Fatalf("%s site 4 collapsed to four-site fallback: first=%#v site4=%#v", tc.preset, first, fourth)
			}
		}
	}
}

func TestLegacyPptConnectionSitesFollowShapeRotationAndFlip(t *testing.T) {
	shape := &presentationir.Shape{
		Geometry:    "rect",
		Bounds:      presentationir.Rect{X: 100, Y: 200, W: 300, H: 400},
		RotationDeg: 90,
	}
	// The authored top site rotates clockwise to the right side.
	got := legacyPptConnectionSitePointForShape(shape, 0)
	want := presentationir.Point{X: 450, Y: 400}
	if got == nil || *got != want {
		t.Fatalf("rotated rect top site = %#v, want %#v", got, want)
	}

	shape.RotationDeg = 0
	shape.FlipV = true
	got = legacyPptConnectionSitePointForShape(shape, 0)
	want = presentationir.Point{X: 250, Y: 600}
	if got == nil || *got != want {
		t.Fatalf("vertically flipped rect top site = %#v, want %#v", got, want)
	}
}

func TestLegacyPptFOPTProjectsWrapFlowAndAnchorCenterIntoCanonicalTextBody(t *testing.T) {
	props := []officeartescher.Property{
		{ID: officeartescher.PropWrapText, Value: 2},
		{ID: officeartescher.PropAnchorText, Value: 4},
		{ID: officeartescher.PropTextFlow, Value: 2},
	}
	var tr legacyPptShapeTransform
	parseLegacyPptFOPTPropertiesTransform(props, nil, &tr)
	if !tr.HasTextWrap || tr.TextWrap != "none" {
		t.Fatalf("wrap = (%v,%q), want explicit none", tr.HasTextWrap, tr.TextWrap)
	}
	if !tr.HasTextVertical || tr.TextVertical != "vert270" {
		t.Fatalf("vertical = (%v,%q), want explicit vert270", tr.HasTextVertical, tr.TextVertical)
	}
	if !tr.HasAnchorCenter || !tr.AnchorCenter {
		t.Fatalf("anchor center = (%v,%v), want explicit true", tr.HasAnchorCenter, tr.AnchorCenter)
	}
	shape := &presentationir.Shape{}
	applyLegacyPptShapeTransform(shape, tr)
	if shape.TextBodyProps.Wrap != "none" || shape.TextBodyProps.Vert != "vert270" || !shape.TextBodyProps.AnchorCtr {
		t.Fatalf("canonical bodyPr = %+v", shape.TextBodyProps)
	}
}

func TestLegacyPptPlaceholderTextFlowPresenceOverridesMaster(t *testing.T) {
	inherited := legacyPptShapeTransform{
		HasTextWrap: true, TextWrap: "none",
		HasTextVertical: true, TextVertical: "vert",
		HasAnchorCenter: true, AnchorCenter: true,
	}
	local := legacyPptShapeTransform{
		HasTextWrap: true, TextWrap: "square",
		HasTextVertical: true, TextVertical: "",
		HasAnchorCenter: true, AnchorCenter: false,
	}
	got := mergeLegacyPptPlaceholderTransform(local, inherited)
	if got.TextWrap != "square" || got.TextVertical != "" || got.AnchorCenter {
		t.Fatalf("explicit local horizontal/wrap/anchorCtr lost: %+v", got)
	}

	got = mergeLegacyPptPlaceholderTransform(legacyPptShapeTransform{}, inherited)
	if got.TextWrap != "none" || got.TextVertical != "vert" || !got.AnchorCenter {
		t.Fatalf("master text flow not inherited: %+v", got)
	}
}

func TestLegacyPptPartialTextInsetsPreserveUnauthoredDefaults(t *testing.T) {
	props := []officeartescher.Property{{ID: officeartescher.PropTextLeft, Value: 0}}
	var tr legacyPptShapeTransform
	parseLegacyPptFOPTPropertiesTransform(props, nil, &tr)
	if !tr.HasTextInsets || !tr.HasTextInsetLeft || tr.HasTextInsetTop || tr.HasTextInsetRight || tr.HasTextInsetBottom {
		t.Fatalf("inset presence = %+v", tr)
	}
	shape := &presentationir.Shape{}
	applyLegacyPptShapeTransform(shape, tr)
	if !shape.HasTextInsets || shape.TextInsetLeftEMU != 0 || shape.TextInsetRightEMU != 91440 || shape.TextInsetTopEMU != 45720 || shape.TextInsetBottomEMU != 45720 {
		t.Fatalf("materialized insets = L%d T%d R%d B%d", shape.TextInsetLeftEMU, shape.TextInsetTopEMU, shape.TextInsetRightEMU, shape.TextInsetBottomEMU)
	}
}

func TestLegacyPptPlaceholderTextInsetsMergePerEdge(t *testing.T) {
	master := legacyPptShapeTransform{HasTextInsets: true, HasTextInsetLeft: true, HasTextInsetTop: true, HasTextInsetRight: true, HasTextInsetBottom: true, TextInsetLeft: 10, TextInsetTop: 20, TextInsetRight: 30, TextInsetBottom: 40}
	local := legacyPptShapeTransform{HasTextInsets: true, HasTextInsetLeft: true, TextInsetLeft: 0}
	got := mergeLegacyPptPlaceholderTransform(local, master)
	if got.TextInsetLeft != 0 || got.TextInsetTop != 20 || got.TextInsetRight != 30 || got.TextInsetBottom != 40 {
		t.Fatalf("merged insets = L%d T%d R%d B%d", got.TextInsetLeft, got.TextInsetTop, got.TextInsetRight, got.TextInsetBottom)
	}
	if !got.HasTextInsetLeft || !got.HasTextInsetTop || !got.HasTextInsetRight || !got.HasTextInsetBottom {
		t.Fatalf("merged inset presence = %+v", got)
	}
}

func TestLegacyPptFOPTKeepsWrapAndAutofitAsIndependentTextContracts(t *testing.T) {
	parse := func(props []officeartescher.Property) legacyPptShapeTransform {
		var tr legacyPptShapeTransform
		parseLegacyPptFOPTPropertiesTransform(props, nil, &tr)
		return tr
	}
	wrapOnly := parse([]officeartescher.Property{{ID: officeartescher.PropWrapText, Value: 2}})
	if !wrapOnly.HasTextWrap || wrapOnly.TextWrap != "none" || wrapOnly.HasAllowOverflow || wrapOnly.HasAutoGrow {
		t.Fatalf("wrap-only contract invented autofit/overflow ownership: %+v", wrapOnly)
	}
	noAutofit := parse([]officeartescher.Property{{ID: officeartescher.PropTextBooleans, Value: 1 << 16}})
	if !noAutofit.HasAutoGrow || noAutofit.AutoGrow || !noAutofit.HasAllowOverflow || !noAutofit.AllowOverflow || noAutofit.HasTextWrap {
		t.Fatalf("explicit fitShapeToText=off contract = %+v", noAutofit)
	}
	spAutoFit := parse([]officeartescher.Property{{ID: officeartescher.PropTextBooleans, Value: (1 << 16) | 1}})
	if !spAutoFit.HasAutoGrow || !spAutoFit.AutoGrow || !spAutoFit.HasAllowOverflow || spAutoFit.AllowOverflow {
		t.Fatalf("explicit fitShapeToText=on contract = %+v", spAutoFit)
	}
}

func TestLegacyPptPlaceholderExplicitNoAutofitOverridesInheritedAutoGrow(t *testing.T) {
	local := legacyPptShapeTransform{HasAutoGrow: true, AutoGrow: false, HasAllowOverflow: true, AllowOverflow: true}
	master := legacyPptShapeTransform{HasAutoGrow: true, AutoGrow: true, HasAllowOverflow: true, AllowOverflow: false}
	got := mergeLegacyPptPlaceholderTransform(local, master)
	if got.AutoGrow || !got.AllowOverflow {
		t.Fatalf("local noAutofit lost to inherited spAutoFit: %+v", got)
	}
}

func TestBuildTextParagraphsPreservesSoftBreakInsideParagraph(t *testing.T) {
	paragraphs := buildTextParagraphs("Alpha\vBeta\rGamma")
	if len(paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(paragraphs))
	}
	if got := paragraphText(paragraphs[0]); got != "Alpha\nBeta" {
		t.Fatalf("first paragraph = %q, want authored soft break", got)
	}
	if got := paragraphText(paragraphs[1]); got != "Gamma" {
		t.Fatalf("second paragraph = %q", got)
	}
}

func TestApplyLegacyPptThemeFontFallbackFillsOnlyMissingRunFamily(t *testing.T) {
	pres := &presentationir.Presentation{Theme: &presentationir.Theme{MajorFont: "Times New Roman", MinorFont: "Times New Roman"}, Slides: []*presentationir.Slide{{Shapes: []*presentationir.Shape{{Kind: presentationir.ShapeText, Placeholder: presentationir.PlaceholderBody, Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "inherited"}, {Text: "explicit", FontFamily: "Arial"}}}}}}}}}
	applyLegacyPptThemeFontFallback(pres)
	runs := pres.Slides[0].Shapes[0].Paragraphs[0].Runs
	if runs[0].FontFamily != "Times New Roman" {
		t.Fatalf("inherited family=%q", runs[0].FontFamily)
	}
	if runs[1].FontFamily != "Arial" {
		t.Fatalf("explicit family overwritten: %q", runs[1].FontFamily)
	}
}

func TestLegacyPptOtherTextRemainsOrdinaryShapeTextAndUses18PtFallback(t *testing.T) {
	if got := legacyPptPlaceholderFromTextType(pptTextTypeOther, false); got != presentationir.PlaceholderNone {
		t.Fatalf("other text placeholder = %v, want PlaceholderNone", got)
	}
	fallback, ok := legacyPptRoleRunFallback(pptTextTypeOther)
	if !ok || fallback.FontSize != 1800 {
		t.Fatalf("other text fallback = %+v, ok=%v; want 1800 hundredths-point", fallback, ok)
	}
	if fallback.FontFamily != "" {
		t.Fatalf("other text fallback family = %q, want document font inheritance", fallback.FontFamily)
	}
	shape := &presentationir.Shape{Placeholder: presentationir.PlaceholderOther}
	if got := legacyPptMasterTextTypeForShape(shape); got != pptTextTypeOther {
		t.Fatalf("PlaceholderOther master text type = %d, want %d", got, pptTextTypeOther)
	}
}

func TestBindLegacyPptMasterOwnershipResolvesPersistRefWithoutChangingSlideIdentifier(t *testing.T) {
	slides := []legacySlideText{
		{PersistID: 10, MasterID: -2147483648},
		{PersistID: 11, MasterID: -2147483636},
		{PersistID: 12, MasterID: 12345},
	}
	entries := []legacyPptMasterEntry{
		{RefID: 3, SlideIdentifier: -2147483648},
		{RefID: 4, SlideIdentifier: -2147483636},
	}
	got := bindLegacyPptMasterOwnership(slides, entries)
	if got[0].MasterID != -2147483648 || got[0].MasterPersistID != 3 {
		t.Fatalf("slide 0 ownership = identifier %d persist %d, want identifier -2147483648 persist 3", got[0].MasterID, got[0].MasterPersistID)
	}
	if got[1].MasterID != -2147483636 || got[1].MasterPersistID != 4 {
		t.Fatalf("slide 1 ownership = identifier %d persist %d, want identifier -2147483636 persist 4", got[1].MasterID, got[1].MasterPersistID)
	}
	if got[2].MasterPersistID != 0 {
		t.Fatalf("unresolved slide master persist = %d, want 0", got[2].MasterPersistID)
	}
}

func TestBindLegacyPptMasterOwnershipDoesNotRetargetDuplicateMasterIdentifier(t *testing.T) {
	slides := []legacySlideText{{PersistID: 10, MasterID: -2147483648}}
	entries := []legacyPptMasterEntry{
		{RefID: 3, SlideIdentifier: -2147483648},
		{RefID: 99, SlideIdentifier: -2147483648},
	}
	got := bindLegacyPptMasterOwnership(slides, entries)
	if got[0].MasterPersistID != 3 {
		t.Fatalf("master persist = %d, want first authored binding 3", got[0].MasterPersistID)
	}
}

func TestLegacyPptSceneGroupOwnsSparseTableFromOutlineRefs(t *testing.T) {
	cells := []legacyPptTableCellShape{
		{outlineRef: 10},
		{outlineRef: 11},
	}
	if !legacyPptSceneGroupOwnsTable(false, cells, nil) {
		t.Fatal("sparse cells with independent OutlineTextRef identities must retain table ownership")
	}
}

func TestNormalizeLegacyPptHeaderFooterOnlySlidesIgnoresPromptOnlyPersist(t *testing.T) {
	slides := []legacySlideText{
		{PersistID: 10, Fragments: []legacySlideFragment{{Text: "Authored title"}}},
		{PersistID: 20, Fragments: []legacySlideFragment{{Text: "Click to edit Master title style"}}},
	}
	infos := map[int]legacyPptHeaderFooterInfo{
		20: {hasAtom: true, flags: legacyPptHFFlagFooter, footerText: "per-slide footer"},
	}
	gotSlides, gotInfos := normalizeLegacyPptHeaderFooterOnlySlides(slides, nil, infos)
	if len(gotSlides) != 1 || gotSlides[0].PersistID != 10 {
		t.Fatalf("slides = %#v, want only authored persist 10", gotSlides)
	}
	if got := gotInfos[10].footerText; got != "per-slide footer" {
		t.Fatalf("promoted footer = %q, want per-slide footer", got)
	}
}

func TestLegacyPptNestedTableMarkerDoesNotPromoteAncestorGroup(t *testing.T) {
	prop := make([]byte, 6)
	binary.LittleEndian.PutUint16(prop[0:2], legacyOfficeArtPropTableProperties)
	binary.LittleEndian.PutUint32(prop[2:6], 1)

	outerRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xD01, 0x01)),
		buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, make([]byte, 16))...,
	))
	innerRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xD02, 0x01)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, make([]byte, 16))...,
		),
		buildOfficeArtRecord(1, officeartescher.RecordTertiaryFOPT, 0x0003, prop)...,
	))
	innerGroup := buildOfficeArtRecord(0, legacyOfficeArtSpgrContainer, 0x000F, innerRoot)
	outerPayload := append(append([]byte{}, outerRoot...), innerGroup...)

	if legacyPptGroupContainerIsTable(outerPayload) {
		t.Fatal("nested table marker leaked ownership to ancestor group")
	}
	if !legacyPptGroupContainerIsTable(innerRoot) {
		t.Fatal("nested group's own tertiary table marker was not recognized")
	}
}

func TestParseTextRulerAtomOwnsDefaultTabsAndCustomStops(t *testing.T) {
	// TextRuler mask: defaultTabSize + tabStops.
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint16(payload[0:2], 0x0001|0x0004)
	payload = binary.LittleEndian.AppendUint16(payload, 360) // default 360 master units
	payload = binary.LittleEndian.AppendUint16(payload, 2)
	payload = binary.LittleEndian.AppendUint16(payload, 720)
	payload = binary.LittleEndian.AppendUint16(payload, 1) // center
	payload = binary.LittleEndian.AppendUint16(payload, 1080)
	payload = binary.LittleEndian.AppendUint16(payload, 3) // decimal

	ruler, ok := parseTextRulerAtom(payload)
	if !ok || !ruler.hasDefaultTab || ruler.defaultTabSize != 360 {
		t.Fatalf("ruler default tab = %#v, ok=%v", ruler, ok)
	}
	if !ruler.hasTabs || len(ruler.tabs) != 2 {
		t.Fatalf("ruler tabs = %#v", ruler.tabs)
	}
	if ruler.tabs[0].Pos != int64(legacyPptMasterUnitsToEMU(720)) || ruler.tabs[0].Algn != "ctr" ||
		ruler.tabs[1].Pos != int64(legacyPptMasterUnitsToEMU(1080)) || ruler.tabs[1].Algn != "dec" {
		t.Fatalf("ruler tabs = %#v", ruler.tabs)
	}
}

func TestApplyRulerLevelsToParasSuppliesMissingTabSemantics(t *testing.T) {
	p := &presentationir.Paragraph{Level: 0}
	ruler := legacyPptTextRuler{
		defaultTabSize: 360, hasDefaultTab: true, hasTabs: true,
		tabs: []presentationir.TabStop{{Pos: int64(legacyPptMasterUnitsToEMU(720)), Algn: "r"}},
	}
	applyRulerLevelsToParas([]*presentationir.Paragraph{p}, ruler)
	if p.DefaultTabSizeEMU != legacyPptMasterUnitsToEMU(360) || len(p.TabStops) != 1 ||
		p.TabStops[0].Pos != int64(legacyPptMasterUnitsToEMU(720)) || p.TabStops[0].Algn != "r" {
		t.Fatalf("paragraph ruler projection = %#v", p)
	}
}

func TestApplyRulerLevelsToParasDoesNotOverrideParagraphTabSemantics(t *testing.T) {
	p := &presentationir.Paragraph{
		Level: 0, DefaultTabSizeEMU: legacyPptMasterUnitsToEMU(240),
		TabStops: []presentationir.TabStop{{Pos: int64(legacyPptMasterUnitsToEMU(480)), Algn: "l"}},
	}
	ruler := legacyPptTextRuler{
		defaultTabSize: 360, hasDefaultTab: true, hasTabs: true,
		tabs: []presentationir.TabStop{{Pos: int64(legacyPptMasterUnitsToEMU(720)), Algn: "r"}},
	}
	applyRulerLevelsToParas([]*presentationir.Paragraph{p}, ruler)
	if p.DefaultTabSizeEMU != legacyPptMasterUnitsToEMU(240) || len(p.TabStops) != 1 ||
		p.TabStops[0].Pos != int64(legacyPptMasterUnitsToEMU(480)) || p.TabStops[0].Algn != "l" {
		t.Fatalf("paragraph-owned tabs were overridden: %#v", p)
	}
}

func TestMergeLegacyPptMasterParagraphConvertsAbsoluteBulletOffsetToRelativeIndent(t *testing.T) {
	margin := legacyPptMasterUnitsToEMU(39)
	p := &presentationir.Paragraph{
		MarginLeftEMU: margin, HasMarginLeft: true,
		TextIndentEMU: 0, HasTextIndent: true,
	}
	got := mergeLegacyPptMasterParagraph(p, legacyPptMasterTextLevel{}, false)
	if got.TextIndentEMU != -margin {
		t.Fatalf("relative indent = %d, want %d", got.TextIndentEMU, -margin)
	}
}

func TestParseTextRulerAtomPreservesExplicitZeroMarginAndIndent(t *testing.T) {
	payload := make([]byte, 4)
	// level 0 left margin (bit 3) and indent (bit 8).
	binary.LittleEndian.PutUint16(payload[0:2], (1<<3)|(1<<8))
	payload = binary.LittleEndian.AppendUint16(payload, 0)
	payload = binary.LittleEndian.AppendUint16(payload, 0)
	ruler, ok := parseTextRulerAtom(payload)
	if !ok || !ruler.hasLeft[0] || !ruler.hasIndent[0] ||
		ruler.leftMargins[0] != 0 || ruler.indents[0] != 0 {
		t.Fatalf("explicit zero ruler provenance lost: %#v ok=%v", ruler, ok)
	}
}

func TestMergeLegacyPptMasterParagraphIgnoresCachedBulletCharWhenBulletDisabled(t *testing.T) {
	paragraph := &presentationir.Paragraph{Runs: []*presentationir.Run{{Text: "E-Auth Level 1"}}}
	master := legacyPptMasterTextLevel{paragraph: stpaPFEntry{HasBullet: false, HasBulletSpecified: false, BulletChar: '•'}}
	merged := mergeLegacyPptMasterParagraph(paragraph, master, false)
	if merged.BulletChar != "" || merged.BulletNone {
		t.Fatalf("bullet = char:%q none:%v, want no inherited bullet", merged.BulletChar, merged.BulletNone)
	}
}

func TestMergeLegacyPptMasterParagraphPreservesExplicitNoBullet(t *testing.T) {
	paragraph := &presentationir.Paragraph{Runs: []*presentationir.Run{{Text: "E-Auth Level 1"}}}
	master := legacyPptMasterTextLevel{paragraph: stpaPFEntry{HasBullet: false, HasBulletSpecified: true, BulletChar: '•'}}
	merged := mergeLegacyPptMasterParagraph(paragraph, master, false)
	if merged.BulletChar != "" || !merged.BulletNone {
		t.Fatalf("bullet = char:%q none:%v, want explicit no-bullet", merged.BulletChar, merged.BulletNone)
	}
}

func TestMergeLegacyPptMasterParagraphResolvesExplicitEnabledInheritedBullet(t *testing.T) {
	paragraph := &presentationir.Paragraph{
		BulletSpecified: true,
		Runs:            []*presentationir.Run{{Text: "Item"}},
	}
	// A master can retain its marker glyph while disabling bullets at the level.
	// Local fHasBullet=true re-enables that marker and must inherit the glyph.
	master := legacyPptMasterTextLevel{paragraph: stpaPFEntry{HasBullet: false, HasBulletSpecified: true, BulletChar: '▪'}}
	merged := mergeLegacyPptMasterParagraph(paragraph, master, false)
	if merged.BulletNone || merged.BulletChar != "▪" {
		t.Fatalf("bullet = char:%q none:%v, want inherited ▪", merged.BulletChar, merged.BulletNone)
	}
}

func TestMergeLegacyPptMasterParagraphUsesStandardBulletWhenEnabledGlyphIsUnavailable(t *testing.T) {
	paragraph := &presentationir.Paragraph{
		BulletSpecified: true,
		Runs:            []*presentationir.Run{{Text: "Item"}},
	}
	master := legacyPptMasterTextLevel{paragraph: stpaPFEntry{HasBullet: false, HasBulletSpecified: true}}
	merged := mergeLegacyPptMasterParagraph(paragraph, master, false)
	if merged.BulletNone || merged.BulletChar != "•" {
		t.Fatalf("bullet = char:%q none:%v, want standard U+2022", merged.BulletChar, merged.BulletNone)
	}
}

func TestLegacyPptOrdinaryTextBoxUsesOtherMasterTextRole(t *testing.T) {
	shape := &presentationir.Shape{
		Kind:        presentationir.ShapeText,
		Placeholder: presentationir.PlaceholderNone,
		Paragraphs:  []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "axis label"}}}},
	}
	styles := legacyPptMasterTextStyles{
		pptTextTypeBody: {
			0: {paragraph: stpaPFEntry{HasBullet: true, HasBulletSpecified: true, BulletChar: '•'}},
		},
		pptTextTypeOther: {
			0: {paragraph: stpaPFEntry{HasBullet: false, HasBulletSpecified: true}},
		},
	}

	textType := legacyPptMasterTextTypeForShape(shape)
	if textType != pptTextTypeOther {
		t.Fatalf("ordinary textbox text type = %d, want PT_Other %d", textType, pptTextTypeOther)
	}
	applyLegacyPptMasterTextStylesToShape(shape, styles, textType)
	got := shape.Paragraphs[0]
	if got.BulletChar != "" || got.AutoNum {
		t.Fatalf("ordinary textbox bullet = char:%q auto:%v none:%v, want no bullet", got.BulletChar, got.AutoNum, got.BulletNone)
	}
}

func TestLegacyPptBodyPlaceholderStillUsesBodyListSemantics(t *testing.T) {
	shape := &presentationir.Shape{
		Kind:        presentationir.ShapeText,
		Placeholder: presentationir.PlaceholderBody,
		Paragraphs:  []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "body item"}}}},
	}
	styles := legacyPptMasterTextStyles{
		pptTextTypeBody: {
			0: {paragraph: stpaPFEntry{HasBullet: true, HasBulletSpecified: true, BulletChar: '•'}},
		},
		pptTextTypeOther: {
			0: {paragraph: stpaPFEntry{HasBullet: false, HasBulletSpecified: true}},
		},
	}

	textType := legacyPptMasterTextTypeForShape(shape)
	if textType != pptTextTypeBody {
		t.Fatalf("body placeholder text type = %d, want PT_Body %d", textType, pptTextTypeBody)
	}
	applyLegacyPptMasterTextStylesToShape(shape, styles, textType)
	got := shape.Paragraphs[0]
	if got.BulletChar != "•" || got.BulletNone {
		t.Fatalf("body placeholder bullet = char:%q none:%v, want inherited bullet", got.BulletChar, got.BulletNone)
	}
}

func TestLegacyPptBodyMasterSparseLevelDoesNotSynthesizeBullet(t *testing.T) {
	styles := legacyPptMasterTextStyles{
		pptTextTypeBody: {
			0: {paragraph: stpaPFEntry{HasBullet: true, HasBulletSpecified: true, BulletChar: '•'}},
			1: {paragraph: stpaPFEntry{HasMarginLeft: true, MarginLeftEMU: 741363}},
		},
	}
	shape := &presentationir.Shape{
		Kind:        presentationir.ShapeText,
		Placeholder: presentationir.PlaceholderBody,
		Paragraphs: []*presentationir.Paragraph{{
			Level: 1, Runs: []*presentationir.Run{{Text: "plain body paragraph"}},
		}},
	}

	applyLegacyPptMasterTextStylesToShape(shape, styles, pptTextTypeBody)
	got := shape.Paragraphs[0]
	if got.BulletChar != "" || got.AutoNum || got.BulletNone {
		t.Fatalf("sparse BODY level manufactured marker: char=%q auto=%v none=%v", got.BulletChar, got.AutoNum, got.BulletNone)
	}
	if !got.HasMarginLeft || got.MarginLeftEMU != 741363 {
		t.Fatalf("BODY structure did not inherit independently of bullet enablement: %+v", got)
	}
}

func TestLegacyPptMasterExplicitNoBulletLevelIsSemantic(t *testing.T) {
	level := legacyPptMasterTextLevel{paragraph: stpaPFEntry{HasBulletSpecified: true, HasBullet: false}}
	if legacyPptMasterTextLevelEmpty(level) {
		t.Fatal("explicit fHasBullet=false must survive sparse-master filtering")
	}

	fallback := legacyPptMasterTextLevel{paragraph: stpaPFEntry{AutoNum: true, AutoNumType: "arabicPeriod", AutoNumStart: 1}}
	merged := mergeLegacyPptMasterTextLevel(level, fallback)
	if merged.paragraph.HasBullet || merged.paragraph.AutoNum || !merged.paragraph.HasBulletSpecified {
		t.Fatalf("explicit fHasBullet=false inherited a fallback marker: %+v", merged.paragraph)
	}
}

func TestLegacyPptMasterTextTypeWithRolesUsesParserLocalTextHeaderRole(t *testing.T) {
	shape := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 42, Placeholder: presentationir.PlaceholderNone}
	roles := map[uint32]int{42: pptTextTypeTitle}
	if got := legacyPptMasterTextTypeForShapeWithRoles(shape, roles); got != pptTextTypeTitle {
		t.Fatalf("resolved text role = %d, want authored PT_Title %d", got, pptTextTypeTitle)
	}
	if got := legacyPptMasterTextTypeForShapeWithRoles(shape, nil); got != pptTextTypeOther {
		t.Fatalf("role-less ordinary shape = %d, want PT_Other %d", got, pptTextTypeOther)
	}
}

func TestLegacyPptGroupDescendantsPreserveParserLocalTextHeaderRoles(t *testing.T) {
	child := &presentationir.Shape{
		Kind:            presentationir.ShapeText,
		OriginalShapeID: 42,
		Paragraphs: []*presentationir.Paragraph{
			{Level: 0, Runs: []*presentationir.Run{{Text: "Heading"}}},
			{Level: 1, Runs: []*presentationir.Run{{Text: "Detail"}}},
		},
	}
	group := &presentationir.Shape{Kind: presentationir.ShapeGroup, Children: []*presentationir.Shape{child}}
	styles := legacyPptMasterTextStyles{
		pptTextTypeBody: {
			0: {paragraph: stpaPFEntry{HasBullet: true, HasBulletSpecified: true, BulletChar: '•'}},
			1: {paragraph: stpaPFEntry{HasBullet: true, HasBulletSpecified: true, BulletChar: '–'}},
		},
		pptTextTypeOther: {
			0: {paragraph: stpaPFEntry{HasBullet: false, HasBulletSpecified: true}},
			1: {paragraph: stpaPFEntry{HasBullet: false, HasBulletSpecified: true}},
		},
	}
	roles := map[uint32]int{42: pptTextTypeBody}

	applyLegacyPptMasterTextStylesToShapeWithRoles(group, styles, pptTextTypeOther, roles)

	if child.Paragraphs[0].BulletChar != "•" || child.Paragraphs[1].BulletChar != "–" {
		t.Fatalf("nested body role was lost: bullets = %q, %q", child.Paragraphs[0].BulletChar, child.Paragraphs[1].BulletChar)
	}
}

func TestLegacyPptGroupedBodyTextDoesNotInferBulletFromRole(t *testing.T) {
	child := &presentationir.Shape{
		Kind:            presentationir.ShapeText,
		OriginalShapeID: 43,
		Paragraphs: []*presentationir.Paragraph{{
			Level: 1, Runs: []*presentationir.Run{{Text: "Nested plain body text"}},
		}},
	}
	group := &presentationir.Shape{Kind: presentationir.ShapeGroup, Children: []*presentationir.Shape{child}}
	styles := legacyPptMasterTextStyles{
		pptTextTypeBody: {
			1: {paragraph: stpaPFEntry{HasMarginLeft: true, MarginLeftEMU: 500000}},
		},
	}
	roles := map[uint32]int{43: pptTextTypeBody}

	applyLegacyPptMasterTextStylesToShapeWithRoles(group, styles, pptTextTypeOther, roles)

	got := child.Paragraphs[0]
	if got.BulletChar != "" || got.AutoNum || got.BulletNone {
		t.Fatalf("nested BODY role manufactured marker: %+v", got)
	}
	if !got.HasMarginLeft || got.MarginLeftEMU != 500000 {
		t.Fatalf("nested BODY paragraph lost inherited structure: %+v", got)
	}
}

func TestLegacyBodyMasterLevelsInheritBaseCharacterFaceAndColor(t *testing.T) {
	levels := map[int]legacyPptMasterTextLevel{
		0: {run: stpaCFRun{FontSize: 2800, FontFamily: "Times New Roman", Color: "FFFFFF"}},
		1: {run: stpaCFRun{FontSize: 2400}},
		2: {run: stpaCFRun{FontSize: 2000, Color: "FFFF00"}},
	}
	got := legacyPptMasterTextLevelsWithBodyCharacterFallback(levels)
	if got[1].run.FontSize != 2400 || got[1].run.FontFamily != "Times New Roman" || got[1].run.Color != "FFFFFF" {
		t.Fatalf("level 1 = %#v, want own size plus inherited face/color", got[1].run)
	}
	if got[2].run.FontSize != 2000 || got[2].run.FontFamily != "Times New Roman" || got[2].run.Color != "FFFF00" {
		t.Fatalf("level 2 = %#v, want authored color preserved and face inherited", got[2].run)
	}
	if got[0].run.FontSize != 2800 || got[0].run.Color != "FFFFFF" {
		t.Fatalf("level 0 changed = %#v", got[0].run)
	}
}

func TestNormalizeLegacyPptEffectiveParagraphDefaultsResolvesLeftAlignmentRecursively(t *testing.T) {
	pres := &presentationir.Presentation{Slides: []*presentationir.Slide{{Shapes: []*presentationir.Shape{{
		Kind:     presentationir.ShapeGroup,
		Children: []*presentationir.Shape{{Kind: presentationir.ShapeText, Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "x"}}}}}},
	}}}}}
	normalizeLegacyPptEffectiveParagraphDefaults(pres)
	if got := pres.Slides[0].Shapes[0].Children[0].Paragraphs[0].Align; got != "l" {
		t.Fatalf("effective alignment = %q, want l", got)
	}
}

func TestMergeLegacyPptMasterParagraphCarriesRemainingParagraphSemantics(t *testing.T) {
	p := &presentationir.Paragraph{Runs: []*presentationir.Run{{Text: "x"}}}
	master := legacyPptMasterTextLevel{paragraph: stpaPFEntry{
		MarginRightEMU: 914400, HasMarginRight: true,
		DefaultTabSizeEMU: 457200, HasDefaultTabSize: true,
		TabStops:      []presentationir.TabStop{{Pos: 914400, Algn: "r"}},
		FontAlign:     "base",
		LineSpacingPt: 1800, HasLineSpacing: true,
	}}
	got := mergeLegacyPptMasterParagraph(p, master, false)
	if !got.HasMarginRight || got.MarginRightEMU != 914400 || got.DefaultTabSizeEMU != 457200 || len(got.TabStops) != 1 || got.TabStops[0].Algn != "r" || got.FontAlign != "base" || !got.HasLineSpacing || got.LineSpacingPt != 1800 {
		t.Fatalf("merged paragraph semantics = %#v", got)
	}
}

func TestNormalizeLegacyPptAnimatedEntrancePaintRestoresChildStroke(t *testing.T) {
	child := &presentationir.Shape{
		ID:        267,
		LineColor: "FFFFFF",
		LineStyle: &drawing.LineStyle{Color: "FFFFFF", Width: 3, Alpha: 0},
		OfficeArt: &officeartir.Shape{Line: officeartir.Line{Visible: true, Color: "FFFFFF", WidthPt: 3, Alpha: 0}},
	}
	group := &presentationir.Shape{
		ID:              266,
		AnimStep:        1,
		EmphasisEffects: []string{"cut"},
		Children:        []*presentationir.Shape{child},
	}

	normalizeLegacyPptAnimatedEntrancePaint([]*presentationir.Shape{group})

	if child.LineStyle == nil || child.LineStyle.Alpha != 1 {
		t.Fatalf("animated child line alpha = %#v, want opaque final paint", child.LineStyle)
	}
	if child.OfficeArt == nil || child.OfficeArt.Line.Alpha != 1 {
		t.Fatalf("animated child OfficeArt alpha = %#v, want opaque final paint", child.OfficeArt)
	}
}

func TestNormalizeLegacyPptAnimatedEntrancePaintRestoresStandaloneLineEndCache(t *testing.T) {
	shape := &presentationir.Shape{
		ID:        219,
		LineColor: "FFFFFF",
		ArrowTail: "triangle",
		LineStyle: &drawing.LineStyle{Color: "FFFFFF", Width: 0.75, Alpha: 0},
		OfficeArt: &officeartir.Shape{Line: officeartir.Line{Visible: true, Color: "FFFFFF", WidthPt: 0.75, Alpha: 0}},
	}

	normalizeLegacyPptAnimatedEntrancePaint([]*presentationir.Shape{shape})

	if shape.LineStyle == nil || shape.LineStyle.Alpha != 1 {
		t.Fatalf("line-ended cache alpha = %#v, want opaque final paint", shape.LineStyle)
	}
	if shape.OfficeArt == nil || shape.OfficeArt.Line.Alpha != 1 {
		t.Fatalf("line-ended OfficeArt cache alpha = %#v, want opaque final paint", shape.OfficeArt)
	}
}

func TestNormalizeLegacyPptAnimatedEntrancePaintPreservesOrdinaryTransparentStroke(t *testing.T) {
	shape := &presentationir.Shape{
		ID:        99,
		LineColor: "FFFFFF",
		LineStyle: &drawing.LineStyle{Color: "FFFFFF", Width: 3, Alpha: 0},
		OfficeArt: &officeartir.Shape{Line: officeartir.Line{Visible: true, Color: "FFFFFF", WidthPt: 3, Alpha: 0}},
	}

	normalizeLegacyPptAnimatedEntrancePaint([]*presentationir.Shape{shape})

	if shape.LineStyle == nil || shape.LineStyle.Alpha != 0 {
		t.Fatalf("ordinary transparent line alpha = %#v, want authored zero preserved", shape.LineStyle)
	}
	if shape.OfficeArt == nil || shape.OfficeArt.Line.Alpha != 0 {
		t.Fatalf("ordinary transparent OfficeArt alpha = %#v, want authored zero preserved", shape.OfficeArt)
	}
}

func TestParseLegacyPptPictureRecolorPreservesMathTypeOLEForeground(t *testing.T) {
	props := []officeartescher.Property{{ID: legacyOfficeArtPropPictureRecolor, Value: 0x00FFFF00}}
	if got := parseLegacyPptPictureRecolor(props, nil); got != "00FFFF" {
		t.Fatalf("picture recolor = %q, want 00FFFF", got)
	}
}

func TestParseLegacyPptPictureRecolorResolvesSlideSchemeColor(t *testing.T) {
	props := []officeartescher.Property{{ID: legacyOfficeArtPropPictureRecolor, Value: 0x08000004}}
	scheme := map[uint8]string{4: "FFFFFF"}
	if got := parseLegacyPptPictureRecolor(props, scheme); got != "FFFFFF" {
		t.Fatalf("scheme picture recolor = %q, want FFFFFF", got)
	}
}

func TestParseLegacyPptMetafileForegroundRecolorResolvesBlackInkToSchemeColor(t *testing.T) {
	payload := make([]byte, 12+44)
	binary.LittleEndian.PutUint16(payload[0:2], 0x0001) // fShouldRecolor
	binary.LittleEndian.PutUint16(payload[2:4], 1)      // cColors
	entry := payload[12:]
	binary.LittleEndian.PutUint16(entry[0:2], 0x0001) // fDoRecolor
	entry[8] = 0                                      // background scheme slot
	binary.LittleEndian.PutUint16(entry[10:12], 0)    // RecolorEntryColor
	// source WideColorStruct remains black.

	if got := parseLegacyPptRecolorInfoForeground(payload, map[uint8]string{0: "FFFFFF"}); got != "FFFFFF" {
		t.Fatalf("metafile foreground recolor = %q, want FFFFFF", got)
	}
}

func TestParseLegacyPptMetafileForegroundRecolorIgnoresDisabledMapping(t *testing.T) {
	payload := make([]byte, 12+44)
	binary.LittleEndian.PutUint16(payload[0:2], 0x0001)
	binary.LittleEndian.PutUint16(payload[2:4], 1)
	payload[12+8] = 0
	if got := parseLegacyPptRecolorInfoForeground(payload, map[uint8]string{0: "FFFFFF"}); got != "" {
		t.Fatalf("disabled metafile recolor = %q, want empty", got)
	}
}

func TestParseLegacyPptMetafileForegroundRecolorUsesExplicitDestinationColor(t *testing.T) {
	payload := make([]byte, 12+44)
	binary.LittleEndian.PutUint16(payload[0:2], 0x0001)
	binary.LittleEndian.PutUint16(payload[2:4], 1)
	entry := payload[12:]
	binary.LittleEndian.PutUint16(entry[0:2], 0x0001)
	binary.LittleEndian.PutUint16(entry[2:4], 0x1200)
	binary.LittleEndian.PutUint16(entry[4:6], 0x3400)
	binary.LittleEndian.PutUint16(entry[6:8], 0x5600)
	entry[8] = 8
	binary.LittleEndian.PutUint16(entry[10:12], 0)
	if got := parseLegacyPptRecolorInfoForeground(payload, nil); got != "123456" {
		t.Fatalf("explicit metafile recolor = %q, want 123456", got)
	}
}

func TestParseLegacyPptRecolorInfoPreservesAllColorMappings(t *testing.T) {
	payload := make([]byte, 12+2*44)
	binary.LittleEndian.PutUint16(payload[0:2], 0x0001)
	binary.LittleEndian.PutUint16(payload[2:4], 2)

	first := payload[12:]
	binary.LittleEndian.PutUint16(first[0:2], 0x0001)
	first[8] = 0
	binary.LittleEndian.PutUint16(first[10:12], 0)

	second := payload[12+44:]
	binary.LittleEndian.PutUint16(second[0:2], 0x0001)
	binary.LittleEndian.PutUint16(second[2:4], 0xAA00)
	binary.LittleEndian.PutUint16(second[4:6], 0xBB00)
	binary.LittleEndian.PutUint16(second[6:8], 0xCC00)
	second[8] = 8
	binary.LittleEndian.PutUint16(second[10:12], 0)
	binary.LittleEndian.PutUint16(second[12:14], 0x1100)
	binary.LittleEndian.PutUint16(second[14:16], 0x2200)
	binary.LittleEndian.PutUint16(second[16:18], 0x3300)

	got := parseLegacyPptRecolorInfo(payload, map[uint8]string{0: "FFFFFF"})
	if len(got) != 2 {
		t.Fatalf("recolors = %#v, want 2 entries", got)
	}
	if got[0].Source != "000000" || got[0].Destination != "FFFFFF" {
		t.Fatalf("first recolor = %#v", got[0])
	}
	if got[1].Source != "112233" || got[1].Destination != "AABBCC" {
		t.Fatalf("second recolor = %#v", got[1])
	}
}

func TestNormalizeLegacyPptSymbolRunStorageCodes(t *testing.T) {
	pres := &presentationir.Presentation{Slides: []*presentationir.Slide{{Shapes: []*presentationir.Shape{{Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "\uf044 \uf03e \uf030", FontFamily: "Symbol"}, {Text: "\uf044 = \uf030", FontFamily: "Symbol"}}}}}}}}}
	normalizeLegacyPptSymbolRunStorageCodes(pres)
	runs := pres.Slides[0].Shapes[0].Paragraphs[0].Runs
	if got := runs[0].Text; got != "Δ > 0" {
		t.Fatalf("first text = %q, want %q", got, "Δ > 0")
	}
	if got := runs[1].Text; got != "Δ = 0" {
		t.Fatalf("second text = %q, want %q", got, "Δ = 0")
	}
}
