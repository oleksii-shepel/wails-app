package stream

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"mime"
	"path"
	"regexp"
	"strconv"
	"strings"

	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
	"office/pkg/word/parsers/ooxml/parts"
	wordresolve "office/pkg/word/parsers/ooxml/resolve"
)

// Strict OOXML permits a percentage lexical form for w:zoom/@percent, while
// the generated settings binding stores it as an integer. Normalize it before
// typed decoding so an optional settings detail cannot discard all render
// metadata (notably styles and document defaults).
var strictZoomPercentAttribute = regexp.MustCompile(`(?i)(\bpercent\s*=\s*["'])([0-9]+)%(["'])`)

// Strict XML uses the same lexical points form for both twip measurements
// (paragraph spacing, indents, tabs and table widths) and half-point
// measurements (font sizes, character position, and kerning). Normalize
// those two families separately before unmarshalling the integer raw model.
var strictHalfPointAttribute = regexp.MustCompile(`(?i)(<(?:[[:alnum:]_]+:)?(?:sz|szCs|position|kern)\b[^>]*\b(?:[[:alnum:]_]+:)?val\s*=\s*["'])([0-9]+(?:\.[0-9]+)?)pt(["'])`)
var strictTwipAttribute = regexp.MustCompile(`(?i)(\b(?:[[:alnum:]_]+:)?[[:alnum:]_]+\s*=\s*["'])([0-9]+(?:\.[0-9]+)?)pt(["'])`)

// DrawingML theme percentages map to the integer 0..100000 representation
// used by the raw DrawingML bindings. This intentionally excludes Word's
// w:zoom/@percent, which is a plain integer and is handled above.
var strictThemePercentageAttribute = regexp.MustCompile(`(?i)(\b(?:[[:alnum:]_]+:)?[[:alnum:]_]+\s*=\s*["'])([0-9]+(?:\.[0-9]+)?)%(["'])`)

func loadDOCXRenderMetadata(r io.ReaderAt, size int64) (*wordresolve.RenderMetadata, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	files := make(map[string]*zip.File, len(zr.File))
	for _, file := range zr.File {
		files[normalizeDOCXPartName(file.Name)] = file
	}

	rootData, err := readDOCXZipPart(files, "_rels/.rels")
	if err != nil {
		return nil, err
	}
	rootRels, err := rels.Parse(rootData)
	if err != nil {
		return nil, fmt.Errorf("word stream: parse package relationships: %w", err)
	}
	mainPart := ""
	for _, relationship := range rootRels.Relationships {
		if relationship.IsExternal() || !strings.HasSuffix(strings.ToLower(relationship.Type), "/officedocument") {
			continue
		}
		mainPart = rels.ResolveTarget("", relationship.Target)
		break
	}
	if mainPart == "" {
		return nil, fmt.Errorf("word stream: officeDocument relationship not found")
	}

	docRelsName := rels.RelationshipsPartName(mainPart)
	docRelsData, err := readDOCXZipPart(files, docRelsName)
	if err != nil {
		// A document without related metadata is legal; return empty defaults.
		return wordresolve.BuildRenderMetadata(nil, nil, nil, nil, nil), nil
	}
	docRels, err := rels.Parse(docRelsData)
	if err != nil {
		return nil, fmt.Errorf("word stream: parse document relationships: %w", err)
	}

	var styles *parts.Styles
	var numbering *parts.Numbering
	var numberingPartName string
	var theme *parts.Theme
	var settings *parts.Settings
	var fontTable *parts.FontTable
	// fontTablePartName is the OPC part that owns the embedded-font
	// relationships; it is needed again when the ODTTF parts are loaded.
	var fontTablePartName string

	for _, relationship := range docRels.Relationships {
		if relationship.IsExternal() {
			continue
		}
		target := rels.ResolveTarget(mainPart, relationship.Target)
		typeName := strings.ToLower(relationship.Type)
		switch {
		case strings.HasSuffix(typeName, "/styles"):
			value := &parts.Styles{}
			if err := unmarshalDOCXMetadataPart(files, target, value); err != nil {
				return nil, err
			}
			styles = value
		case strings.HasSuffix(typeName, "/numbering"):
			value := &parts.Numbering{}
			if err := unmarshalDOCXMetadataPart(files, target, value); err != nil {
				return nil, err
			}
			numbering = value
			numberingPartName = target
		case strings.HasSuffix(typeName, "/theme"):
			value := &parts.Theme{}
			if err := unmarshalDOCXMetadataPart(files, target, value); err != nil {
				return nil, err
			}
			theme = value
		case strings.HasSuffix(typeName, "/settings"):
			value := &parts.Settings{}
			if err := unmarshalDOCXMetadataPart(files, target, value); err != nil {
				return nil, err
			}
			settings = value
		case strings.HasSuffix(typeName, "/fonttable"):
			value := &parts.FontTable{}
			if err := unmarshalDOCXMetadataPart(files, target, value); err != nil {
				return nil, err
			}
			fontTable = value
			fontTablePartName = target
		}
	}

	attachDOCXPictureBullets(files, numberingPartName, numbering)
	meta := wordresolve.BuildRenderMetadata(styles, numbering, theme, settings, fontTable)
	attachDOCXEmbeddedFonts(r, size, fontTablePartName, fontTable, meta)
	return meta, nil
}

func attachDOCXPictureBullets(files map[string]*zip.File, numberingPartName string, numbering *parts.Numbering) {
	if numbering == nil || numberingPartName == "" || len(numbering.NumPicBullets) == 0 {
		return
	}
	relData, err := readDOCXZipPart(files, rels.RelationshipsPartName(numberingPartName))
	if err != nil {
		return
	}
	numberingRels, err := rels.Parse(relData)
	if err != nil {
		return
	}
	relByID := make(map[string]rels.Relationship, len(numberingRels.Relationships))
	for _, relationship := range numberingRels.Relationships {
		relByID[relationship.ID] = relationship
	}
	for _, bullet := range numbering.NumPicBullets {
		if bullet == nil || bullet.Pict == nil || bullet.Pict.Shape == nil || bullet.Pict.Shape.ImageData == nil {
			continue
		}
		relationship, ok := relByID[bullet.Pict.Shape.ImageData.RelID]
		if !ok || relationship.IsExternal() {
			continue
		}
		target := rels.ResolveTarget(numberingPartName, relationship.Target)
		data, err := readDOCXZipPart(files, target)
		if err != nil {
			continue
		}
		contentType := mime.TypeByExtension(path.Ext(target))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		bullet.VMLRelID = relationship.ID
		bullet.Media = &parts.MediaPart{ContentType: contentType, Data: data}
		bullet.WidthPt, bullet.HeightPt = docxVMLPictureBulletExtent(bullet.Pict.Shape.Style)
		if bullet.WidthPt <= 0 {
			bullet.WidthPt = 9
		}
		if bullet.HeightPt <= 0 {
			bullet.HeightPt = 9
		}
	}
}

func docxVMLPictureBulletExtent(style string) (widthPt, heightPt float64) {
	for _, declaration := range strings.Split(style, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(declaration), ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(strings.ToLower(value))
		if !strings.HasSuffix(value, "pt") {
			continue
		}
		points, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, "pt")), 64)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(strings.ToLower(name)) {
		case "width":
			widthPt = points
		case "height":
			heightPt = points
		}
	}
	return widthPt, heightPt
}

// attachDOCXEmbeddedFonts lifts ODTTF font parts referenced by the font table
// into the document. Layout measures advance widths from these faces, so
// omitting them makes every line break drift from the authoring application
// whenever a document embeds its fonts. Failure is non-fatal: embedded fonts
// are a fidelity refinement, not a loading requirement.
func attachDOCXEmbeddedFonts(r io.ReaderAt, size int64, fontTablePartName string, fontTable *parts.FontTable, meta *wordresolve.RenderMetadata) {
	if fontTable == nil || meta == nil || len(fontTable.Fonts) == 0 || fontTablePartName == "" {
		return
	}
	pkg, err := opc.OpenReaderAt(r, size)
	if err != nil {
		return
	}
	relsForPart, ok, err := pkg.Relationships(fontTablePartName)
	if err != nil || !ok {
		return
	}
	byID := make(map[string]rels.Relationship, len(relsForPart.Relationships))
	for _, relationship := range relsForPart.Relationships {
		byID[relationship.ID] = relationship
	}
	fontParts := make([]*fontdata.EmbeddedFontData, 0, len(fontTable.Fonts))
	for _, font := range fontTable.Fonts {
		if font == nil || strings.TrimSpace(font.Name) == "" {
			continue
		}
		embedded := fontdata.LoadEmbeddedFont(pkg, fontTablePartName, font.Name, fontdata.StyleRefs{
			Regular:    embeddedFontRefID(font.EmbedRegular),
			Bold:       embeddedFontRefID(font.EmbedBold),
			Italic:     embeddedFontRefID(font.EmbedItalic),
			BoldItalic: embeddedFontRefID(font.EmbedBoldItalic),
		}, fontdata.StyleKeys{
			Regular:    embeddedFontKey(font.EmbedRegular),
			Bold:       embeddedFontKey(font.EmbedBold),
			Italic:     embeddedFontKey(font.EmbedItalic),
			BoldItalic: embeddedFontKey(font.EmbedBoldItalic),
		}, byID)
		if embedded != nil {
			fontParts = append(fontParts, embedded)
		}
	}
	if len(fontParts) == 0 {
		return
	}
	meta.Document.EmbeddedFonts = append(meta.Document.EmbeddedFonts, wordresolve.ResolveEmbeddedFonts(fontParts)...)
}

func embeddedFontRefID(ref *parts.FontEmbedRef) string {
	if ref == nil {
		return ""
	}
	return ref.RelID
}

func embeddedFontKey(ref *parts.FontEmbedRef) string {
	if ref == nil {
		return ""
	}
	return ref.FontKey
}

func unmarshalDOCXMetadataPart(files map[string]*zip.File, name string, out any) error {
	data, err := readDOCXZipPart(files, name)
	if err != nil {
		return err
	}
	data = normalizeStrictSettingsLexicalValues(data)
	if strings.HasPrefix(strings.ToLower(normalizeDOCXPartName(name)), "word/theme/") {
		data = normalizeStrictPointValues(data, strictThemePercentageAttribute, 1000)
	}
	if err := xml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("word stream: parse metadata part %q: %w", name, err)
	}
	return nil
}

func normalizeStrictSettingsLexicalValues(data []byte) []byte {
	data = strictZoomPercentAttribute.ReplaceAll(data, []byte("${1}${2}${3}"))
	data = normalizeStrictPointValues(data, strictHalfPointAttribute, 2)
	return normalizeStrictPointValues(data, strictTwipAttribute, 20)
}

func normalizeStrictPointValues(data []byte, expression *regexp.Regexp, multiplier float64) []byte {
	return expression.ReplaceAllFunc(data, func(match []byte) []byte {
		parts := expression.FindSubmatch(match)
		points, err := strconv.ParseFloat(string(parts[2]), 64)
		if err != nil {
			return match
		}
		return []byte(string(parts[1]) + strconv.Itoa(int(math.Round(points*multiplier))) + string(parts[3]))
	})
}

func readDOCXZipPart(files map[string]*zip.File, name string) ([]byte, error) {
	file := files[normalizeDOCXPartName(name)]
	if file == nil {
		return nil, fmt.Errorf("word stream: DOCX part %q not found", name)
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
