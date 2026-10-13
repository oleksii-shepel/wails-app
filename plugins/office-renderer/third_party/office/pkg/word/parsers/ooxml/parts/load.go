package parts

import (
	"encoding/xml"
	"fmt"
	"strings"

	"office/pkg/diag"
	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
	"office/pkg/ooxml/xmltree"
)

func loadCustomXMLData(pkg *opc.Package, out *Package) {
	if pkg == nil || out == nil {
		return
	}
	for _, part := range pkg.Parts() {
		name := strings.TrimPrefix(part.Name, "/")
		lowerName := strings.ToLower(name)
		if !strings.HasPrefix(lowerName, "customxml/") || !strings.HasSuffix(lowerName, ".xml") {
			continue
		}
		if strings.Contains(lowerName, "itemprops") {
			continue
		}
		relsForPart, ok, err := pkg.Relationships(name)
		if err != nil || !ok {
			continue
		}
		for _, relationship := range relsForPart.Relationships {
			if relationship.Type != relationshipTypeCustomXMLProps || relationship.IsExternal() {
				continue
			}
			propsName := rels.ResolveTarget(name, relationship.Target)
			propsPart, ok := pkg.Part(propsName)
			if !ok {
				continue
			}
			propsRoot, err := xmltree.Parse(propsPart.Data)
			if err != nil {
				continue
			}
			storeID, ok := propsRoot.AttrValue("itemID")
			if !ok {
				continue
			}
			storeID = normalizeCustomXMLStoreID(storeID)
			if storeID == "" {
				continue
			}
			out.CustomXMLParts[storeID] = append([]byte(nil), part.Data...)
		}
	}
	if len(out.CustomXMLParts) == 0 {
		out.CustomXMLParts = nil
	}
}

func captureOpaqueParts(pkg *opc.Package, out *Package) {
	if pkg == nil || out == nil {
		return
	}
	known := map[string]bool{
		strings.TrimPrefix(out.MainPartName, "/"): true,
		"docProps/core.xml":                       true,
		"docProps/app.xml":                        true,
		"docProps/custom.xml":                     true,
	}
	for _, name := range []string{out.StylesPartName, out.NumberingPartName, out.ThemePartName, out.FontTablePartName, out.GlossaryPartName, out.SettingsPartName, out.FootnotesPartName, out.EndnotesPartName, out.CommentsPartName} {
		if name != "" {
			known[strings.TrimPrefix(name, "/")] = true
		}
	}
	for name := range out.Headers {
		known[strings.TrimPrefix(name, "/")] = true
	}
	for name := range out.Footers {
		known[strings.TrimPrefix(name, "/")] = true
	}
	for source, rs := range out.PartRels {
		_ = source
		if rs == nil {
			continue
		}
		for _, r := range rs.Relationships {
			if r == nil || r.TargetMode == "External" || !typedRelationshipType(r.Type) {
				continue
			}
			known[strings.TrimPrefix(rels.ResolveTarget(source, r.Target), "/")] = true
		}
	}
	for _, part := range pkg.Parts() {
		name := strings.TrimPrefix(part.Name, "/")
		if name == "" || strings.HasSuffix(strings.ToLower(name), ".rels") || known[name] {
			continue
		}
		if strings.HasPrefix(strings.ToLower(name), "customxml/") {
			continue
		}
		opaque := OpaquePart{ContentType: part.ContentType, Data: append([]byte(nil), part.Data...)}
		if sourceRels, ok, err := pkg.Relationships(name); err == nil && ok {
			opaque.Relationships = convertRelationships(sourceRels)
		}
		out.OpaqueParts[name] = opaque
	}
	if len(out.OpaqueParts) == 0 {
		out.OpaqueParts = nil
	}
}

func typedRelationshipType(t string) bool {
	switch t {
	case relationshipTypeStyles, relationshipTypeNumbering, relationshipTypeHeader, relationshipTypeFooter,
		relationshipTypeImage, relationshipTypeAltChunk, relationshipTypeOLEObject, relationshipTypePackage,
		relationshipTypeChart, relationshipTypeDiagramData, relationshipTypeDiagramLayout, relationshipTypeDiagramQuickStyle,
		relationshipTypeDiagramColor, relationshipTypeDiagramDrawing, relationshipTypeSettings, relationshipTypeTheme,
		relTypeFontTable, relationshipTypeGlossary, relationshipTypeFootnotes, relationshipTypeEndnotes, relationshipTypeComments:
		return true
	default:
		return false
	}
}

func normalizeCustomXMLStoreID(id string) string {
	return strings.ToUpper(strings.TrimSpace(id))
}

func findMainDocumentPart(pkg *opc.Package) (string, error) {
	rootRels, ok, err := pkg.Relationships("")
	if err != nil {
		return "", fmt.Errorf("word parts: parse root relationships: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("word parts: root relationships not found")
	}

	for _, relationship := range rootRels.Relationships {
		if relationship.Type != relationshipTypeOfficeDocument || relationship.IsExternal() {
			continue
		}
		return rels.ResolveTarget("", relationship.Target), nil
	}

	return "", fmt.Errorf("word parts: officeDocument relationship not found")
}

func loadRelatedParts(pkg *opc.Package, out *Package) error {
	docRels := out.PartRels[out.MainPartName]
	if docRels == nil {
		return nil
	}

	for _, relationship := range docRels.Relationships {
		if relationship == nil {
			continue
		}
		if relationship.TargetMode == "External" {
			if relationship.Type == relationshipTypeOLEObject || relationship.Type == relationshipTypePackage {
				out.ScopedOLELinks[relRef(out.MainPartName, relationship.ID)] = relationship.Target
			}
			continue
		}

		partName := rels.ResolveTarget(out.MainPartName, relationship.Target)
		part, ok := pkg.Part(partName)
		if !ok {
			continue
		}

		switch relationship.Type {
		case relationshipTypeStyles:
			var styles Styles
			if err := unmarshalStrictWordPart(part.Data, &styles); err != nil {
				return fmt.Errorf("word parts: parse styles %s: %w", partName, err)
			}
			out.Styles = &styles
			out.StylesPartName = partName
		case relationshipTypeNumbering:
			var numbering Numbering
			if err := unmarshalStrictWordPart(part.Data, &numbering); err != nil {
				return fmt.Errorf("word parts: parse numbering %s: %w", partName, err)
			}
			out.Numbering = &numbering
			out.NumberingPartName = partName
		case relationshipTypeHeader:
			var header Header
			if err := unmarshalStrictWordPart(part.Data, &header); err != nil {
				return fmt.Errorf("word parts: parse header %s: %w", partName, err)
			}
			out.Headers[partName] = &header
		case relationshipTypeFooter:
			var footer Footer
			if err := unmarshalStrictWordPart(part.Data, &footer); err != nil {
				return fmt.Errorf("word parts: parse footer %s: %w", partName, err)
			}
			out.Footers[partName] = &footer
		case relationshipTypeImage:
			out.ScopedMediaParts[relRef(out.MainPartName, relationship.ID)] = MediaPart{
				ContentType: part.ContentType,
				Data:        append([]byte(nil), part.Data...),
			}
		case relationshipTypeAltChunk:
			out.ScopedAltChunkParts[relRef(out.MainPartName, relationship.ID)] = AltChunkPart{
				ContentType: part.ContentType,
				Data:        append([]byte(nil), part.Data...),
			}
		case relationshipTypeOLEObject, relationshipTypePackage:
			out.ScopedOLEParts[relRef(out.MainPartName, relationship.ID)] = OLEPart{ContentType: part.ContentType, Data: append([]byte(nil), part.Data...), RelationshipType: relationship.Type}
		case relationshipTypeTheme:
			var theme Theme
			if err := unmarshalStrictThemePart(part.Data, &theme); err != nil {
				return fmt.Errorf("word parts: parse theme %s: %w", partName, err)
			}
			out.Theme = &theme
			out.ThemePartName = partName
		case relationshipTypeGlossary:
			var glossary GlossaryDocument
			if err := unmarshalStrictWordPart(part.Data, &glossary); err != nil {
				return fmt.Errorf("word parts: parse glossary %s: %w", partName, err)
			}
			out.Glossary = &glossary
			out.GlossaryPartName = partName
		case relationshipTypeFont:
			var fontTable FontTable
			if err := unmarshalStrictWordPart(part.Data, &fontTable); err != nil {
				return fmt.Errorf("word parts: parse font table %s: %w", partName, err)
			}
			out.FontTable = &fontTable
			out.FontTablePartName = partName
			loadEmbeddedFonts(pkg, partName, &fontTable, out)
		case relationshipTypeSettings:
			var settings Settings
			if err := unmarshalStrictWordPart(part.Data, &settings); err != nil {
				return fmt.Errorf("word parts: parse settings %s: %w", partName, err)
			}
			out.Settings = &settings
			out.SettingsPartName = partName
		case relationshipTypeFootnotes:
			var fn FootnotesPart
			if err := unmarshalStrictWordPart(part.Data, &fn); err != nil {
				return fmt.Errorf("word parts: parse footnotes %s: %w", partName, err)
			}
			out.Footnotes = &fn
			out.FootnotesPartName = partName
		case relationshipTypeEndnotes:
			var en EndnotesPart
			if err := unmarshalStrictWordPart(part.Data, &en); err != nil {
				return fmt.Errorf("word parts: parse endnotes %s: %w", partName, err)
			}
			out.Endnotes = &en
			out.EndnotesPartName = partName
		case relationshipTypeComments:
			var cm CommentsPart
			if err := unmarshalStrictWordPart(part.Data, &cm); err != nil {
				return fmt.Errorf("word parts: parse comments %s: %w", partName, err)
			}
			out.Comments = &cm
			out.CommentsPartName = partName
		case relationshipTypeChart:
			out.ScopedChartParts[relRef(out.MainPartName, relationship.ID)] = append([]byte(nil), part.Data...)
		case relationshipTypeDiagramData:
			out.ScopedSmartArtParts[relRef(out.MainPartName, relationship.ID)] = append([]byte(nil), part.Data...)
		case relationshipTypeDiagramLayout:
			out.ScopedSmartArtLayoutParts[relRef(out.MainPartName, relationship.ID)] = append([]byte(nil), part.Data...)
		case relationshipTypeDiagramQuickStyle:
			out.ScopedSmartArtStyleParts[relRef(out.MainPartName, relationship.ID)] = append([]byte(nil), part.Data...)
		case relationshipTypeDiagramColor:
			out.ScopedSmartArtColorParts[relRef(out.MainPartName, relationship.ID)] = append([]byte(nil), part.Data...)
		case relationshipTypeDiagramDrawing:
			out.ScopedSmartArtDrawingParts[relRef(out.MainPartName, relationship.ID)] = append([]byte(nil), part.Data...)
		}
	}

	loadPartRelationships(pkg, out, out.MainPartName)
	loadPartRelationships(pkg, out, out.GlossaryPartName)
	loadPartRelationships(pkg, out, out.SettingsPartName)
	for partName := range out.Headers {
		loadPartRelationships(pkg, out, partName)
	}
	for partName := range out.Footers {
		loadPartRelationships(pkg, out, partName)
	}
	loadPartRelationships(pkg, out, out.FootnotesPartName)
	loadPartRelationships(pkg, out, out.EndnotesPartName)
	loadPartRelationships(pkg, out, out.CommentsPartName)

	// Load relationships from every SmartArt diagram data part so that layout,
	// quick style, color, and pre-rendered drawing definitions are available
	// regardless of whether the diagram lives in the body or another story.
	dataParts := make(map[string]struct{})
	for ref := range out.ScopedSmartArtParts {
		if partName, ok := out.RelationshipTarget(ref.SourcePart, ref.ID); ok && partName != "" {
			dataParts[partName] = struct{}{}
		}
	}
	for partName := range dataParts {
		loadPartRelationships(pkg, out, partName)
	}

	// A diagram data part can point at a pre-rendered diagramDrawing. Load that
	// drawing's own relationships as well so picture fills can resolve media.
	for dataPartName := range dataParts {
		relsForData := out.PartRels[dataPartName]
		if relsForData == nil {
			continue
		}
		for _, rel := range relsForData.Relationships {
			if rel == nil || rel.Type != relationshipTypeDiagramDrawing || rel.TargetMode == "External" {
				continue
			}
			drawingPartName := rels.ResolveTarget(dataPartName, rel.Target)
			loadPartRelationships(pkg, out, drawingPartName)
		}
	}

	section := lastSection(out.Document)
	if section == nil {
		return nil
	}

	for _, ref := range section.HeaderRefs {
		if resolved, ok := resolveReference(out, ref.ID); ok {
			out.ResolvedHeaders = append(out.ResolvedHeaders, ResolvedReference{
				Kind:           "header",
				Type:           ref.Type,
				RelationshipID: ref.ID,
				TargetPartName: resolved,
			})
		}
	}

	for _, ref := range section.FooterRefs {
		if resolved, ok := resolveReference(out, ref.ID); ok {
			out.ResolvedFooters = append(out.ResolvedFooters, ResolvedReference{
				Kind:           "footer",
				Type:           ref.Type,
				RelationshipID: ref.ID,
				TargetPartName: resolved,
			})
		}
	}

	return nil
}

func loadCoreProperties(pkg *opc.Package, out *Package) error {
	rootRels, ok, err := pkg.Relationships("")
	if err != nil {
		return fmt.Errorf("word parts: parse root relationships: %w", err)
	}
	if !ok {
		return nil
	}
	for _, relationship := range rootRels.Relationships {
		if relationship.Type != relationshipTypeCoreProperties || relationship.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget("", relationship.Target)
		part, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		var props CoreProperties
		if err := xml.Unmarshal(part.Data, &props); err != nil {
			return fmt.Errorf("word parts: parse core properties %s: %w", partName, err)
		}
		out.CoreProperties = &props
		return nil
	}
	return nil
}

func loadExtendedProperties(pkg *opc.Package, out *Package) {
	rootRels, ok, err := pkg.Relationships("")
	if err != nil || !ok {
		return
	}
	for _, relationship := range rootRels.Relationships {
		if relationship.Type != relationshipTypeExtendedProps || relationship.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget("", relationship.Target)
		part, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		var props ExtendedProperties
		if err := xml.Unmarshal(part.Data, &props); err != nil {
			out.Diagnostics.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     "word.extended_properties_parse_error",
				Message:  fmt.Sprintf("parse extended properties %s: %v", partName, err),
			})
			continue
		}
		out.ExtendedProperties = &props
		return
	}
}

func loadCustomProperties(pkg *opc.Package, out *Package) {
	rootRels, ok, err := pkg.Relationships("")
	if err != nil || !ok {
		return
	}
	for _, relationship := range rootRels.Relationships {
		if relationship.Type != relationshipTypeCustomProps || relationship.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget("", relationship.Target)
		part, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		var props CustomProperties
		if err := xml.Unmarshal(part.Data, &props); err != nil {
			out.Diagnostics.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     "word.custom_properties_parse_error",
				Message:  fmt.Sprintf("parse custom properties %s: %v", partName, err),
			})
			continue
		}
		out.CustomProperties = &props
		return
	}
}

func loadEmbeddedFonts(pkg *opc.Package, fontTablePartName string, fontTable *FontTable, out *Package) {
	if pkg == nil || fontTable == nil {
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
	for _, font := range fontTable.Fonts {
		embedded := fontdata.LoadEmbeddedFont(pkg, fontTablePartName, font.Name, fontdata.StyleRefs{
			Regular:    refID(font.EmbedRegular),
			Bold:       refID(font.EmbedBold),
			Italic:     refID(font.EmbedItalic),
			BoldItalic: refID(font.EmbedBoldItalic),
		}, fontdata.StyleKeys{
			Regular:    fontKey(font.EmbedRegular),
			Bold:       fontKey(font.EmbedBold),
			Italic:     fontKey(font.EmbedItalic),
			BoldItalic: fontKey(font.EmbedBoldItalic),
		}, byID)
		if embedded != nil {
			out.EmbeddedFonts = append(out.EmbeddedFonts, embedded)
		}
	}
}

func fontKey(ref *FontEmbedRef) string {
	if ref == nil {
		return ""
	}
	return ref.FontKey
}

func refID(ref *FontEmbedRef) string {
	if ref == nil {
		return ""
	}
	return ref.RelID
}

func loadSignatures(pkg *opc.Package, out *Package) {
	if pkg == nil || out == nil {
		return
	}

	out.HasSignatures = pkg.HasSignatures()
	for _, signature := range pkg.Signatures() {
		out.Signatures = append(out.Signatures, signature)
		if signature.Signature != nil {
			continue
		}
		if signature.ContentType != "application/vnd.openxmlformats-package.digital-signature-xmlsignature+xml" {
			continue
		}
		out.Diagnostics.Add(diag.Diagnostic{
			Severity: diag.SeverityWarning,
			Code:     "word.digital_signature_parse_error",
			Message:  fmt.Sprintf("parse digital signature %s", signature.PartName),
		})
	}
	out.SignatureVerifications = append(out.SignatureVerifications, pkg.VerifySignatures()...)
}

func loadPartRelationships(pkg *opc.Package, out *Package, sourcePartName string) {
	if pkg == nil || out == nil || sourcePartName == "" {
		return
	}
	relsForPart, ok, err := pkg.Relationships(sourcePartName)
	if err != nil || !ok || relsForPart == nil {
		return
	}
	converted := convertRelationships(relsForPart)
	out.PartRels[sourcePartName] = converted
	for _, relationship := range relsForPart.Relationships {
		ref := relRef(sourcePartName, relationship.ID)
		if relationship.Type == relTypeHyperlink && relationship.IsExternal() {
			out.ScopedHyperlinkURLs[ref] = relationship.Target
			continue
		}
		if relationship.IsExternal() {
			if relationship.Type == relationshipTypeOLEObject || relationship.Type == relationshipTypePackage {
				out.ScopedOLELinks[ref] = relationship.Target
			}
			continue
		}
		partName := rels.ResolveTarget(sourcePartName, relationship.Target)
		part, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		switch relationship.Type {
		case relationshipTypeImage:
			out.ScopedMediaParts[ref] = MediaPart{
				ContentType: part.ContentType,
				Data:        append([]byte(nil), part.Data...),
			}
		case relationshipTypeOLEObject, relationshipTypePackage:
			out.ScopedOLEParts[ref] = OLEPart{ContentType: part.ContentType, Data: append([]byte(nil), part.Data...), RelationshipType: relationship.Type}
		case relationshipTypeChart:
			out.ScopedChartParts[ref] = append([]byte(nil), part.Data...)
		case relationshipTypeDiagramData:
			out.ScopedSmartArtParts[ref] = append([]byte(nil), part.Data...)
		case relationshipTypeDiagramLayout:
			out.ScopedSmartArtLayoutParts[ref] = append([]byte(nil), part.Data...)
		case relationshipTypeDiagramQuickStyle:
			out.ScopedSmartArtStyleParts[ref] = append([]byte(nil), part.Data...)
		case relationshipTypeDiagramColor:
			out.ScopedSmartArtColorParts[ref] = append([]byte(nil), part.Data...)
		case relationshipTypeDiagramDrawing:
			out.ScopedSmartArtDrawingParts[ref] = append([]byte(nil), part.Data...)
		}
	}
}

func resolveReference(pkg *Package, relationshipID string) (string, bool) {
	if pkg == nil {
		return "", false
	}
	return pkg.RelationshipTarget(pkg.MainPartName, relationshipID)
}

func lastSection(document *Document) *SectionProperties {
	if document == nil || document.Body == nil {
		return nil
	}
	if document.Body.Section != nil {
		return document.Body.Section
	}
	for i := len(document.Body.Paragraphs) - 1; i >= 0; i-- {
		paragraph := document.Body.Paragraphs[i]
		if paragraph != nil && paragraph.Properties != nil && paragraph.Properties.Section != nil {
			return paragraph.Properties.Section
		}
	}
	return nil
}

func convertRelationships(doc *rels.Document) *Relationships {
	if doc == nil {
		return nil
	}
	out := &Relationships{
		Relationships: make([]*Relationship, 0, len(doc.Relationships)),
	}
	for _, relationship := range doc.Relationships {
		copy := relationship
		out.Relationships = append(out.Relationships, &Relationship{
			ID:         copy.ID,
			Type:       copy.Type,
			Target:     copy.Target,
			TargetMode: copy.TargetMode,
		})
	}
	return out
}
