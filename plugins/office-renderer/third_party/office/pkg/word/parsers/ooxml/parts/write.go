package parts

import (
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/opc"
	rels2 "office/pkg/ooxml/rels"
)

// WritePackage serializes a Word parts.Package into an OPC ZIP archive.
func WritePackage(pkg *Package, w io.Writer) error {
	if pkg == nil {
		return fmt.Errorf("nil package")
	}

	opcPkg := opc.NewPackage()
	for _, sourceName := range sortedOpaquePartNames(pkg.OpaqueParts) {
		part := pkg.OpaqueParts[sourceName]
		name, ok := normalizeOPCPartName(sourceName)
		if !ok || len(part.Data) == 0 {
			continue
		}
		opcPkg.AddPart(name, part.ContentType, append([]byte(nil), part.Data...))
		if part.Relationships != nil && len(part.Relationships.Relationships) > 0 {
			relsData, err := xml.Marshal(canonicalRelationships(part.Relationships))
			if err != nil {
				return fmt.Errorf("marshal opaque relationships for %s: %w", name, err)
			}
			opcPkg.AddPart(relationshipsPartName(name), "application/vnd.openxmlformats-package.relationships+xml", prependXMLHeader(relsData))
		}
	}
	relMgr := newRelationshipManager()
	mainPartName := pkg.mainPartName()
	pkg.MainPartName = mainPartName

	if pkg.Document != nil {
		// Root rel: document.
		relMgr.addRootRel(relTypeOfficeDocument, "word/document.xml")

		docData, err := xml.Marshal(pkg.Document)
		if err != nil {
			return fmt.Errorf("marshal document: %w", err)
		}
		// Prepend WordprocessingML namespace.
		docXML := prependXMLHeader(addWordNamespace(string(docData)))
		opcPkg.AddPart("word/document.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml", docXML)

		docRels := pkg.EnsurePartRelationships(mainPartName)

		// Document relationships.
		for relID, url := range pkg.HyperlinkTargetsFor(mainPartName) {
			pkg.UpsertRelationship(mainPartName, &Relationship{
				ID:         relID,
				Type:       relTypeHyperlink,
				Target:     url,
				TargetMode: "External",
			})
		}
		// Image relationships.
		mediaParts := pkg.MediaPartsFor(mainPartName)
		if len(mediaParts) > 0 {
			mediaRelIDs := make([]string, 0, len(mediaParts))
			for relID := range mediaParts {
				mediaRelIDs = append(mediaRelIDs, relID)
			}
			sort.Strings(mediaRelIDs)
			mediaNum := 0
			for _, relID := range mediaRelIDs {
				if relationshipTargetValue(docRels, relID) != "" {
					mediaNum++
				}
			}
			for _, relID := range mediaRelIDs {
				mp := mediaParts[relID]
				target := relationshipTargetValue(docRels, relID)
				if target == "" {
					mediaNum++
					target = fmt.Sprintf("media/image%d%s", mediaNum, extFromContentType(mp.ContentType))
					pkg.SetMediaPart(mainPartName, relID, mp, target)
				}
				opcPkg.AddPart("word/"+strings.TrimPrefix(target, "word/"), mp.ContentType, mp.Data)
			}
		}
		// Embedded OLE/package relationships.
		oleParts := pkg.OLEPartsFor(mainPartName)
		if len(oleParts) > 0 {
			oleRelIDs := make([]string, 0, len(oleParts))
			for relID := range oleParts {
				oleRelIDs = append(oleRelIDs, relID)
			}
			sort.Strings(oleRelIDs)
			oleNum := 0
			for _, relID := range oleRelIDs {
				if relationshipTargetValue(docRels, relID) != "" {
					oleNum++
				}
			}
			for _, relID := range oleRelIDs {
				op := oleParts[relID]
				target := relationshipTargetValue(docRels, relID)
				if target == "" {
					oleNum++
					target = fmt.Sprintf("embeddings/oleObject%d.bin", oleNum)
					pkg.SetOLEPart(mainPartName, relID, op, target)
				}
				partName := target
				if !strings.HasPrefix(partName, "word/") {
					partName = "word/" + strings.TrimPrefix(partName, "/")
				}
				contentType := op.ContentType
				if contentType == "" {
					contentType = "application/vnd.openxmlformats-officedocument.oleObject"
				}
				opcPkg.AddPart(partName, contentType, op.Data)
			}
		}

		// Chart relationships.
		chartParts := pkg.ChartPartsFor(mainPartName)
		if len(chartParts) > 0 {
			chartRelIDs := make([]string, 0, len(chartParts))
			for relID := range chartParts {
				chartRelIDs = append(chartRelIDs, relID)
			}
			sort.Strings(chartRelIDs)
			chartNum := 0
			for _, relID := range chartRelIDs {
				if relationshipTargetValue(docRels, relID) != "" {
					chartNum++
				}
			}
			for _, relID := range chartRelIDs {
				target := relationshipTargetValue(docRels, relID)
				if target == "" {
					chartNum++
					target = fmt.Sprintf("charts/chart%d.xml", chartNum)
					pkg.SetChartPart(mainPartName, relID, chartParts[relID], target)
				}
				opcPkg.AddPart("word/"+strings.TrimPrefix(target, "word/"), "application/vnd.openxmlformats-officedocument.drawingml.chart+xml", prependXMLHeader(chartParts[relID]))
			}
		}

		// Helper for generating sequential relationship IDs.
		nextRelID := makeNextRelID(docRels)
		ensureDocumentRel := func(target, relType string) string {
			for _, r := range docRels.Relationships {
				if r.Target == target && r.Type == relType {
					return r.ID
				}
			}
			id := nextRelID()
			pkg.UpsertRelationship(mainPartName, &Relationship{
				ID:     id,
				Type:   relType,
				Target: target,
			})
			return id
		}

		// Headers.
		for _, partName := range sortedHeaderNames(pkg.Headers) {
			header := pkg.Headers[partName]
			headerData, err := xml.Marshal(header)
			if err != nil {
				return fmt.Errorf("marshal header %s: %w", partName, err)
			}
			opcPkg.AddPart(partName, "application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml", prependXMLHeader(addWordNamespace(string(headerData))))
			target := strings.TrimPrefix(partName, "word/")
			ensureDocumentRel(target, relTypeHeader)
			if err := writeScopedPartRelationships(opcPkg, pkg, partName); err != nil {
				return fmt.Errorf("write header rels %s: %w", partName, err)
			}
		}

		// Footers.
		for _, partName := range sortedFooterNames(pkg.Footers) {
			footer := pkg.Footers[partName]
			footerData, err := xml.Marshal(footer)
			if err != nil {
				return fmt.Errorf("marshal footer %s: %w", partName, err)
			}
			opcPkg.AddPart(partName, "application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml", prependXMLHeader(addWordNamespace(string(footerData))))
			target := strings.TrimPrefix(partName, "word/")
			ensureDocumentRel(target, relTypeFooter)
			if err := writeScopedPartRelationships(opcPkg, pkg, partName); err != nil {
				return fmt.Errorf("write footer rels %s: %w", partName, err)
			}
		}

		// Footnotes.
		if pkg.Footnotes != nil {
			fnData, err := xml.Marshal(pkg.Footnotes)
			if err != nil {
				return fmt.Errorf("marshal footnotes: %w", err)
			}
			if pkg.FootnotesPartName == "" {
				pkg.FootnotesPartName = "word/footnotes.xml"
			}
			target := "footnotes.xml"
			ensureDocumentRel(target, relTypeFootnotes)
			opcPkg.AddPart(pkg.FootnotesPartName, "application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml", prependXMLHeader(addWordNamespace(string(fnData))))
			if err := writeScopedPartRelationships(opcPkg, pkg, pkg.FootnotesPartName); err != nil {
				return fmt.Errorf("write footnotes rels: %w", err)
			}
		}

		// Endnotes.
		if pkg.Endnotes != nil {
			enData, err := xml.Marshal(pkg.Endnotes)
			if err != nil {
				return fmt.Errorf("marshal endnotes: %w", err)
			}
			if pkg.EndnotesPartName == "" {
				pkg.EndnotesPartName = "word/endnotes.xml"
			}
			target := "endnotes.xml"
			ensureDocumentRel(target, relTypeEndnotes)
			opcPkg.AddPart(pkg.EndnotesPartName, "application/vnd.openxmlformats-officedocument.wordprocessingml.endnotes+xml", prependXMLHeader(addWordNamespace(string(enData))))
			if err := writeScopedPartRelationships(opcPkg, pkg, pkg.EndnotesPartName); err != nil {
				return fmt.Errorf("write endnotes rels: %w", err)
			}
		}

		// Comments.
		if pkg.Comments != nil {
			cmData, err := xml.Marshal(pkg.Comments)
			if err != nil {
				return fmt.Errorf("marshal comments: %w", err)
			}
			if pkg.CommentsPartName == "" {
				pkg.CommentsPartName = "word/comments.xml"
			}
			target := "comments.xml"
			ensureDocumentRel(target, relTypeComments)
			opcPkg.AddPart(pkg.CommentsPartName, "application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml", prependXMLHeader(addWordNamespace(string(cmData))))
			if err := writeScopedPartRelationships(opcPkg, pkg, pkg.CommentsPartName); err != nil {
				return fmt.Errorf("write comments rels: %w", err)
			}
		}

		// SmartArt / diagram data parts.
		smartArtParts := pkg.SmartArtPartsFor(mainPartName)
		if len(smartArtParts) > 0 {
			saRelIDs := make([]string, 0, len(smartArtParts))
			for relID := range smartArtParts {
				saRelIDs = append(saRelIDs, relID)
			}
			sort.Strings(saRelIDs)
			saNum := 0
			for _, relID := range saRelIDs {
				if relationshipTargetValue(docRels, relID) != "" {
					saNum++
				}
			}
			for _, relID := range saRelIDs {
				target := relationshipTargetValue(docRels, relID)
				if target == "" {
					saNum++
					target = fmt.Sprintf("diagrams/data%d.xml", saNum)
					pkg.SetSmartArtPart(mainPartName, relID, smartArtParts[relID], target)
				}
				dataPartName := rels2.ResolveTarget(mainPartName, target)
				opcPkg.AddPart(dataPartName, "application/vnd.openxmlformats-officedocument.drawingml.diagramData+xml", prependXMLHeader(smartArtParts[relID]))
				if err := writeSmartArtCompanionParts(opcPkg, pkg, dataPartName); err != nil {
					return fmt.Errorf("write SmartArt companion parts: %w", err)
				}
			}
		}
		if err := writeSmartArtCompanionParts(opcPkg, pkg, mainPartName); err != nil {
			return fmt.Errorf("write direct SmartArt companion parts: %w", err)
		}

		nextDocRelID := makeNextRelID(docRels)

		// Styles.
		if pkg.Styles != nil {
			ensureDocumentRelationship(pkg, relTypeStyles, "styles.xml", nextDocRelID)
			stylesData, err := xml.Marshal(pkg.Styles)
			if err != nil {
				return fmt.Errorf("marshal styles: %w", err)
			}
			opcPkg.AddPart("word/styles.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml", prependXMLHeader(addWordNamespace(string(stylesData))))
		}

		// Numbering.
		if pkg.Numbering != nil {
			ensureDocumentRelationship(pkg, relTypeNumbering, "numbering.xml", nextDocRelID)
			numData, err := xml.Marshal(pkg.Numbering)
			if err != nil {
				return fmt.Errorf("marshal numbering: %w", err)
			}
			opcPkg.AddPart("word/numbering.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml", prependXMLHeader(addWordNamespace(string(numData))))
		}

		// Settings.
		if pkg.Settings != nil {
			ensureDocumentRelationship(pkg, relTypeSettings, "settings.xml", nextDocRelID)
			settingsData, err := xml.Marshal(pkg.Settings)
			if err != nil {
				return fmt.Errorf("marshal settings: %w", err)
			}
			opcPkg.AddPart("word/settings.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml", prependXMLHeader(addWordNamespace(string(settingsData))))
		}

		// Theme.
		if pkg.Theme != nil {
			ensureDocumentRelationship(pkg, relTypeTheme, "theme/theme1.xml", nextDocRelID)
			themeData, err := xml.Marshal(pkg.Theme)
			if err != nil {
				return fmt.Errorf("marshal theme: %w", err)
			}
			opcPkg.AddPart("word/theme/theme1.xml", "application/vnd.openxmlformats-officedocument.theme+xml", prependXMLHeader(themeData))
		}

		// Font table.
		if pkg.FontTable != nil {
			ensureDocumentRelationship(pkg, relTypeFontTable, "fontTable.xml", nextDocRelID)
			ftData, err := xml.Marshal(pkg.FontTable)
			if err != nil {
				return fmt.Errorf("marshal fontTable: %w", err)
			}
			opcPkg.AddPart("word/fontTable.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml", prependXMLHeader(addWordNamespace(string(ftData))))

			// Embedded fonts: write font data parts and fontTable.xml.rels.
			if len(pkg.EmbeddedFonts) > 0 {
				fontRels := buildFontTableRels(pkg)
				if fontRels != nil && len(fontRels.Relationships) > 0 {
					fontRelsData, err := xml.Marshal(canonicalRelationships(fontRels))
					if err != nil {
						return fmt.Errorf("marshal fontTable rels: %w", err)
					}
					opcPkg.AddPart("word/_rels/fontTable.xml.rels", "application/vnd.openxmlformats-package.relationships+xml", prependXMLHeader(fontRelsData))
				}
				for _, ef := range pkg.EmbeddedFonts {
					writeFontPart(opcPkg, ef.Regular)
					writeFontPart(opcPkg, ef.Bold)
					writeFontPart(opcPkg, ef.Italic)
					writeFontPart(opcPkg, ef.BoldItalic)
				}
			}
		}

		if docRels != nil && len(docRels.Relationships) > 0 {
			docRelsData, err := xml.Marshal(canonicalRelationships(docRels))
			if err != nil {
				return fmt.Errorf("marshal document rels: %w", err)
			}
			opcPkg.AddPart("word/_rels/document.xml.rels", "application/vnd.openxmlformats-package.relationships+xml", prependXMLHeader(docRelsData))
		}
	}

	// Core properties.
	if pkg.CoreProperties != nil {
		rel := relMgr.addRootRel(relationshipTypeCoreProperties, "docProps/core.xml")
		coreData, err := xml.Marshal(pkg.CoreProperties)
		if err != nil {
			return fmt.Errorf("marshal core properties: %w", err)
		}
		opcPkg.AddPart("docProps/core.xml", "application/vnd.openxmlformats-package.core-properties+xml", prependXMLHeader(coreData))
		_ = rel
	}

	// Extended properties.
	if pkg.ExtendedProperties != nil {
		rel := relMgr.addRootRel(relationshipTypeExtendedProps, "docProps/app.xml")
		appData, err := xml.Marshal(pkg.ExtendedProperties)
		if err != nil {
			return fmt.Errorf("marshal extended properties: %w", err)
		}
		opcPkg.AddPart("docProps/app.xml", "application/vnd.openxmlformats-officedocument.extended-properties+xml", prependXMLHeader(appData))
		_ = rel
	}

	// Custom properties.
	if pkg.CustomProperties != nil && len(pkg.CustomProperties.Properties) > 0 {
		rel := relMgr.addRootRel(relationshipTypeCustomProps, "docProps/custom.xml")
		customData, err := xml.Marshal(pkg.CustomProperties)
		if err != nil {
			return fmt.Errorf("marshal custom properties: %w", err)
		}
		opcPkg.AddPart("docProps/custom.xml", "application/vnd.openxmlformats-officedocument.custom-properties+xml", prependXMLHeader(customData))
		_ = rel
	}
	if err := writeCustomXMLParts(opcPkg, pkg); err != nil {
		return err
	}

	// Root rels. Preserve untyped package-level relationships while typed
	// officeDocument/property relationships are regenerated above.
	rootRels := mergeRootRelationships(relMgr.rootRels(), pkg.RootRelationships)
	rootRelsData, err := xml.Marshal(canonicalRelationships(rootRels))
	if err != nil {
		return fmt.Errorf("marshal root rels: %w", err)
	}
	opcPkg.AddPart("_rels/.rels", "application/vnd.openxmlformats-package.relationships+xml", prependXMLHeader(rootRelsData))

	return opcPkg.Write(w)
}

func mergeRootRelationships(generated, source *Relationships) *Relationships {
	out := &Relationships{}
	seen := make(map[string]bool)
	if generated != nil {
		for _, r := range generated.Relationships {
			if r == nil {
				continue
			}
			copy := *r
			out.Relationships = append(out.Relationships, &copy)
			seen[r.ID] = true
		}
	}
	if source != nil {
		nextOpaqueID := 1
		for _, r := range source.Relationships {
			if r == nil || generatedRootRelationshipType(r.Type) {
				continue
			}
			copy := *r
			if copy.ID == "" || seen[copy.ID] {
				for {
					candidate := fmt.Sprintf("rIdOpaque%d", nextOpaqueID)
					nextOpaqueID++
					if !seen[candidate] {
						copy.ID = candidate
						break
					}
				}
			}
			out.Relationships = append(out.Relationships, &copy)
			seen[copy.ID] = true
		}
	}
	return out
}

func generatedRootRelationshipType(t string) bool {
	switch t {
	case relationshipTypeOfficeDocument, relationshipTypeCoreProperties, relationshipTypeExtendedProps, relationshipTypeCustomProps:
		return true
	default:
		return false
	}
}

func writeCustomXMLParts(opcPkg *opc.Package, pkg *Package) error {
	if opcPkg == nil || pkg == nil || len(pkg.CustomXMLParts) == 0 {
		return nil
	}

	keys := make([]string, 0, len(pkg.CustomXMLParts))
	for key := range pkg.CustomXMLParts {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)

	type customXMLProps struct {
		XMLName xml.Name `xml:"http://schemas.openxmlformats.org/officeDocument/2006/customXml datastoreItem"`
		ItemID  string   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/customXml itemID,attr"`
	}

	for i, key := range keys {
		index := i + 1
		itemName := fmt.Sprintf("customXml/item%d.xml", index)
		propsName := fmt.Sprintf("customXml/itemProps%d.xml", index)
		relsName := fmt.Sprintf("customXml/_rels/item%d.xml.rels", index)

		opcPkg.AddPart(itemName, "application/xml", pkg.CustomXMLParts[key])

		propsData, err := xml.Marshal(customXMLProps{ItemID: key})
		if err != nil {
			return fmt.Errorf("marshal custom XML props %s: %w", key, err)
		}
		opcPkg.AddPart(propsName, "application/vnd.openxmlformats-officedocument.customXmlProperties+xml", prependXMLHeader(propsData))

		relsData, err := xml.Marshal(&Relationships{
			Relationships: []*Relationship{
				{
					ID:     "rId1",
					Type:   relationshipTypeCustomXMLProps,
					Target: fmt.Sprintf("itemProps%d.xml", index),
				},
			},
		})
		if err != nil {
			return fmt.Errorf("marshal custom XML rels %s: %w", key, err)
		}
		opcPkg.AddPart(relsName, "application/vnd.openxmlformats-package.relationships+xml", prependXMLHeader(relsData))
	}
	return nil
}

func sortedOpaquePartNames(parts map[string]OpaquePart) []string {
	keys := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for name := range parts {
		normalized, ok := normalizeOPCPartName(name)
		if !ok || seen[normalized] {
			continue
		}
		seen[normalized] = true
		keys = append(keys, name)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, _ := normalizeOPCPartName(keys[i])
		b, _ := normalizeOPCPartName(keys[j])
		return a < b
	})
	return keys
}

func sortedHeaderNames(parts map[string]*Header) []string {
	keys := make([]string, 0, len(parts))
	for name := range parts {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

func sortedFooterNames(parts map[string]*Footer) []string {
	keys := make([]string, 0, len(parts))
	for name := range parts {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

func normalizeOPCPartName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsRune(name, '\x00') || strings.Contains(name, "\\") {
		return "", false
	}
	name = strings.TrimPrefix(name, "/")
	clean := path.Clean(name)
	if clean == "." || clean == "" || clean != name || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", false
	}
	return clean, true
}

func canonicalRelationships(rels *Relationships) *Relationships {
	if rels == nil || len(rels.Relationships) == 0 {
		return rels
	}
	out := &Relationships{Relationships: make([]*Relationship, 0, len(rels.Relationships))}
	for _, rel := range rels.Relationships {
		if rel == nil {
			continue
		}
		copy := *rel
		out.Relationships = append(out.Relationships, &copy)
	}
	sort.SliceStable(out.Relationships, func(i, j int) bool {
		a, b := out.Relationships[i], out.Relationships[j]
		ai, aok := numericRelationshipID(a.ID)
		bi, bok := numericRelationshipID(b.ID)
		if aok != bok {
			return aok
		}
		if aok && ai != bi {
			return ai < bi
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		return a.TargetMode < b.TargetMode
	})
	return out
}

func numericRelationshipID(id string) (int, bool) {
	if !strings.HasPrefix(id, "rId") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, "rId"))
	return n, err == nil
}

// buildFontTableRels constructs the fontTable.xml.rels Relationships by correlating
// pkg.FontTable embed references with the loaded pkg.EmbeddedFonts font parts.
func buildFontTableRels(pkg *Package) *Relationships {
	if pkg.FontTable == nil {
		return nil
	}
	// Build lookup: font family -> style -> FontPart.
	fontParts := make(map[string]map[string]*fontdata.FontPart)
	for _, ef := range pkg.EmbeddedFonts {
		m := map[string]*fontdata.FontPart{
			"regular":    ef.Regular,
			"bold":       ef.Bold,
			"italic":     ef.Italic,
			"boldItalic": ef.BoldItalic,
		}
		fontParts[ef.Typeface] = m
	}

	fontRels := &Relationships{}
	seen := make(map[string]bool)
	for _, font := range pkg.FontTable.Fonts {
		refs := []struct {
			ref   *FontEmbedRef
			style string
		}{
			{font.EmbedRegular, "regular"},
			{font.EmbedBold, "bold"},
			{font.EmbedItalic, "italic"},
			{font.EmbedBoldItalic, "boldItalic"},
		}
		for _, r := range refs {
			if r.ref == nil || r.ref.RelID == "" {
				continue
			}
			parts := fontParts[strings.TrimSpace(font.Name)]
			if parts == nil {
				continue
			}
			fp := parts[r.style]
			if fp == nil || fp.Name == "" {
				continue
			}
			target := strings.TrimPrefix(fp.Name, "word/")
			key := r.ref.RelID + "|" + target
			if seen[key] {
				continue
			}
			seen[key] = true
			fontRels.Relationships = append(fontRels.Relationships, &Relationship{
				ID:     r.ref.RelID,
				Type:   relTypeFont,
				Target: target,
			})
		}
	}
	if len(fontRels.Relationships) == 0 {
		return nil
	}
	return fontRels
}

func writeFontPart(opcPkg *opc.Package, fp *fontdata.FontPart) {
	if fp == nil || fp.Name == "" || len(fp.Data) == 0 {
		return
	}
	opcPkg.AddPart(fp.Name, fp.ContentType, fp.Data)
}

func addWordNamespace(s string) []byte {
	const wNamespace = ` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
	const docOpen = "<document>"
	for i := 0; i < len(s)-len(docOpen)+1; i++ {
		if s[i:i+len(docOpen)] == docOpen {
			return []byte(s[:i+len(docOpen)-1] + wNamespace + ">" + s[i+len(docOpen):])
		}
	}
	return []byte(s)
}

func prependXMLHeader(data []byte) []byte {
	if len(data) > 0 && data[0] == '<' {
		return append([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`), data...)
	}
	return data
}

func makeNextRelID(rels *Relationships) func() string {
	max := 0
	if rels != nil {
		for _, r := range rels.Relationships {
			if n, err := strconv.Atoi(strings.TrimPrefix(r.ID, "rId")); err == nil && n > max {
				max = n
			}
		}
	}
	return func() string {
		max++
		return "rId" + strconv.Itoa(max)
	}
}

func ensureDocumentRelationship(pkg *Package, relType, target string, nextID func() string) {
	if pkg == nil {
		return
	}
	rels := pkg.EnsurePartRelationships(pkg.mainPartName())
	for _, rel := range rels.Relationships {
		if rel != nil && rel.Type == relType && rel.Target == target {
			return
		}
	}
	rels.Relationships = append(rels.Relationships, &Relationship{
		ID:     nextID(),
		Type:   relType,
		Target: target,
	})
}

func relationshipTargetValue(rels *Relationships, relationshipID string) string {
	if rels == nil || relationshipID == "" {
		return ""
	}
	for _, rel := range rels.Relationships {
		if rel != nil && rel.ID == relationshipID {
			return rel.Target
		}
	}
	return ""
}

func writeScopedPartRelationships(opcPkg *opc.Package, pkg *Package, sourcePart string) error {
	if opcPkg == nil || pkg == nil || sourcePart == "" || sourcePart == pkg.mainPartName() {
		return nil
	}

	rels := pkg.EnsurePartRelationships(sourcePart)
	for relID, target := range pkg.HyperlinkTargetsFor(sourcePart) {
		pkg.UpsertRelationship(sourcePart, &Relationship{
			ID:         relID,
			Type:       relTypeHyperlink,
			Target:     target,
			TargetMode: "External",
		})
	}

	mediaParts := pkg.MediaPartsFor(sourcePart)
	if len(mediaParts) > 0 {
		mediaRelIDs := make([]string, 0, len(mediaParts))
		for relID := range mediaParts {
			mediaRelIDs = append(mediaRelIDs, relID)
		}
		sort.Strings(mediaRelIDs)
		for _, relID := range mediaRelIDs {
			mp := mediaParts[relID]
			target := relationshipTargetValue(rels, relID)
			if target == "" {
				target = scopedAssetTarget(sourcePart, "media", relID, extFromContentType(mp.ContentType))
				pkg.SetMediaPart(sourcePart, relID, mp, target)
			}
			opcPkg.AddPart(rels2.ResolveTarget(sourcePart, target), mp.ContentType, mp.Data)
		}
	}

	chartParts := pkg.ChartPartsFor(sourcePart)
	if len(chartParts) > 0 {
		chartRelIDs := make([]string, 0, len(chartParts))
		for relID := range chartParts {
			chartRelIDs = append(chartRelIDs, relID)
		}
		sort.Strings(chartRelIDs)
		for _, relID := range chartRelIDs {
			target := relationshipTargetValue(rels, relID)
			if target == "" {
				target = scopedAssetTarget(sourcePart, "charts", relID, ".xml")
				pkg.SetChartPart(sourcePart, relID, chartParts[relID], target)
			}
			opcPkg.AddPart(rels2.ResolveTarget(sourcePart, target), "application/vnd.openxmlformats-officedocument.drawingml.chart+xml", prependXMLHeader(chartParts[relID]))
		}
	}

	smartArtParts := pkg.SmartArtPartsFor(sourcePart)
	if len(smartArtParts) > 0 {
		saRelIDs := make([]string, 0, len(smartArtParts))
		for relID := range smartArtParts {
			saRelIDs = append(saRelIDs, relID)
		}
		sort.Strings(saRelIDs)
		for _, relID := range saRelIDs {
			target := relationshipTargetValue(rels, relID)
			if target == "" {
				target = scopedAssetTarget(sourcePart, "diagrams", relID, ".xml")
				pkg.SetSmartArtPart(sourcePart, relID, smartArtParts[relID], target)
			}
			dataPartName := rels2.ResolveTarget(sourcePart, target)
			opcPkg.AddPart(dataPartName, "application/vnd.openxmlformats-officedocument.drawingml.diagramData+xml", prependXMLHeader(smartArtParts[relID]))
			if err := writeSmartArtCompanionParts(opcPkg, pkg, dataPartName); err != nil {
				return err
			}
		}
	}
	if err := writeSmartArtCompanionParts(opcPkg, pkg, sourcePart); err != nil {
		return err
	}

	if rels != nil && len(rels.Relationships) > 0 {
		data, err := xml.Marshal(canonicalRelationships(rels))
		if err != nil {
			return err
		}
		opcPkg.AddPart(relationshipsPartName(sourcePart), "application/vnd.openxmlformats-package.relationships+xml", prependXMLHeader(data))
	}
	return nil
}

func writeSmartArtCompanionParts(opcPkg *opc.Package, pkg *Package, sourcePart string) error {
	if opcPkg == nil || pkg == nil || sourcePart == "" {
		return nil
	}
	rels := pkg.PartRels[sourcePart]
	if rels == nil {
		return nil
	}
	for _, rel := range rels.Relationships {
		if rel == nil || rel.TargetMode == "External" {
			continue
		}
		var data []byte
		var contentType string
		switch rel.Type {
		case relationshipTypeDiagramLayout:
			data, _ = pkg.SmartArtLayoutPartFor(sourcePart, rel.ID)
			contentType = "application/vnd.openxmlformats-officedocument.drawingml.diagramLayout+xml"
		case relationshipTypeDiagramQuickStyle:
			data, _ = pkg.SmartArtStylePartFor(sourcePart, rel.ID)
			contentType = "application/vnd.openxmlformats-officedocument.drawingml.diagramStyle+xml"
		case relationshipTypeDiagramColor:
			data, _ = pkg.SmartArtColorPartFor(sourcePart, rel.ID)
			contentType = "application/vnd.openxmlformats-officedocument.drawingml.diagramColors+xml"
		case relationshipTypeDiagramDrawing:
			data, _ = pkg.SmartArtDrawingPartFor(sourcePart, rel.ID)
			contentType = "application/vnd.ms-office.drawingml.diagramDrawing+xml"
		default:
			continue
		}
		if len(data) == 0 {
			continue
		}
		targetPart := rels2.ResolveTarget(sourcePart, rel.Target)
		opcPkg.AddPart(targetPart, contentType, prependXMLHeader(data))
		if rel.Type == relationshipTypeDiagramDrawing {
			if err := writeScopedPartRelationships(opcPkg, pkg, targetPart); err != nil {
				return err
			}
		}
	}
	// Data-part relationships are not otherwise emitted by the Word writer.
	if sourcePart != pkg.mainPartName() {
		data, err := xml.Marshal(canonicalRelationships(rels))
		if err != nil {
			return err
		}
		opcPkg.AddPart(relationshipsPartName(sourcePart), "application/vnd.openxmlformats-package.relationships+xml", prependXMLHeader(data))
	}
	return nil
}

func relationshipsPartName(sourcePart string) string {
	dir := path.Dir(sourcePart)
	base := path.Base(sourcePart)
	if dir == "." || dir == "" {
		return "_rels/" + base + ".rels"
	}
	return path.Join(dir, "_rels", base+".rels")
}

func scopedAssetTarget(sourcePart, folder, relID, suffix string) string {
	base := strings.TrimSuffix(path.Base(sourcePart), path.Ext(sourcePart))
	replacer := strings.NewReplacer(" ", "-", ":", "-", "/", "-", "\\", "-")
	base = replacer.Replace(base)
	if base == "" {
		base = "document"
	}
	if suffix == "" {
		suffix = ".bin"
	}
	return path.Join(folder, base+"-"+relID+suffix)
}

type relationshipManager struct {
	nextID   int
	root     []Relationship
	document []Relationship
}

func newRelationshipManager() *relationshipManager {
	return &relationshipManager{nextID: 1}
}

func (rm *relationshipManager) nextRelID() string {
	id := "rId" + strconv.Itoa(rm.nextID)
	rm.nextID++
	return id
}

func (rm *relationshipManager) addRootRel(relType, target string) Relationship {
	r := Relationship{ID: rm.nextRelID(), Type: relType, Target: target}
	rm.root = append(rm.root, r)
	return r
}

func (rm *relationshipManager) addDocumentRel(relType, target string) Relationship {
	r := Relationship{ID: rm.nextRelID(), Type: relType, Target: target}
	rm.document = append(rm.document, r)
	return r
}

func (rm *relationshipManager) rootRels() *Relationships {
	out := &Relationships{Relationships: make([]*Relationship, len(rm.root))}
	for i, r := range rm.root {
		copy := r
		out.Relationships[i] = &copy
	}
	return out
}

const (
	relTypeOfficeDocument    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	relTypeStyles            = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
	relTypeNumbering         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering"
	relTypeSettings          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings"
	relTypeTheme             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme"
	relTypeFontTable         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/fontTable"
	relTypeHyperlink         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
	relTypeImage             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
	relTypeChart             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart"
	relTypeHeader            = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header"
	relTypeFooter            = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer"
	relTypeFootnotes         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes"
	relTypeEndnotes          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes"
	relTypeComments          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments"
	relTypeDiagramData       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData"
	relTypeDiagramLayout     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramLayout"
	relTypeDiagramQuickStyle = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramQuickStyle"
	relTypeDiagramColor      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramColors"
	relTypeFont              = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/font"
)

func extFromContentType(ct string) string {
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "bmp"):
		return ".bmp"
	default:
		return ""
	}
}
