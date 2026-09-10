package parts

import (
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
)

func writePreservedPartGraph(opcPkg *opc.Package, pp PreservedPart, partName, contentType string) {
	if opcPkg == nil || partName == "" {
		return
	}
	opcPkg.AddPart(partName, contentType, pp.Data)
	if len(pp.Relationships) > 0 {
		rdoc := rels.Document{}
		for _, r := range pp.Relationships {
			rdoc.Relationships = append(rdoc.Relationships, rels.Relationship{ID: r.ID, Type: r.Type, Target: r.Target, TargetMode: r.TargetMode})
		}
		if data, err := marshalRels(rdoc); err == nil {
			opcPkg.AddPart(rels.RelationshipsPartName(partName), "application/vnd.openxmlformats-package.relationships+xml", data)
		}
	}
	for childName, child := range pp.Children {
		ct := child.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		writePreservedPartGraph(opcPkg, child, childName, ct)
	}
}

// WritePackage serializes a parts.Package into an OPC ZIP archive.
// It generates relationship files, content types, and marshals all XML parts.
func WritePackage(pkg *Package, w io.Writer) error {
	if pkg == nil {
		return fmt.Errorf("nil package")
	}

	opcPkg := opc.NewPackage()
	relMgr := newRelationshipManager()

	// Ensure common defaults.
	opcPkg.AddPart("[content_types].xml", "application/vnd.openxmlformats-package.core-properties+xml", []byte{})

	if pkg.Workbook != nil {
		// Root rel: workbook.
		workbookRel := relMgr.addRootRel(relTypeOfficeDocument, "xl/workbook.xml")
		_ = workbookRel

		// Ensure workbook sheet refs have relationship IDs.
		for i := range pkg.Workbook.Sheets {
			if pkg.Workbook.Sheets[i].RelID == "" {
				pkg.Workbook.Sheets[i].RelID = relMgr.nextRelID()
			}
		}

		// Counters for pivot table/cache, chart, and VML drawing part numbering.
		ptNum, cacheNum, chartNum := 0, 0, 0
		tableNum := 0
		vmlNum := 0
		pkg.Workbook.PivotCaches = nil

		// Workbook relationships.
		for i, entry := range pkg.Worksheets {
			rel := relMgr.addWorkbookRel(relTypeWorksheet, relPartName(entry.PartName), entry.RelID)
			if entry.RelID == "" {
				entry.RelID = rel.ID
			}

			// Handle drawing for this worksheet.
			if entry.Drawing != nil {
				drawingNum := i + 1
				drawingPartName := fmt.Sprintf("xl/drawings/drawing%d.xml", drawingNum)
				drawingRelID := fmt.Sprintf("rIdDraw%d", drawingNum)
				if entry.Sheet != nil {
					entry.Sheet.DrawingRef = &DrawingRef{RelID: drawingRelID}
				}

				// Write drawing.
				drawingData, err := xml.Marshal(entry.Drawing)
				if err != nil {
					return fmt.Errorf("marshal drawing %s: %w", drawingPartName, err)
				}
				opcPkg.AddPart(drawingPartName, "application/vnd.openxmlformats-officedocument.drawing+xml", prependXMLHeader(drawingData))

				// Write drawing-level image and chart relationships.
				var drawingRels []rels.Relationship
				for relID, mp := range entry.DrawingMedia {
					target := makeRelTarget(drawingPartName, mp.Name)
					drawingRels = append(drawingRels, rels.Relationship{ID: relID, Type: relTypeImage, Target: target})
					opcPkg.AddPart(mp.Name, mp.ContentType, mp.Data)
				}
				chartRelIDs := make([]string, 0, len(entry.DrawingChartParts))
				for relID := range entry.DrawingChartParts {
					chartRelIDs = append(chartRelIDs, relID)
				}
				sort.Strings(chartRelIDs)
				for _, relID := range chartRelIDs {
					chartNum++
					chartPartName := fmt.Sprintf("xl/charts/chart%d.xml", chartNum)
					target := makeRelTarget(drawingPartName, chartPartName)
					drawingRels = append(drawingRels, rels.Relationship{ID: relID, Type: relTypeChart, Target: target})
					opcPkg.AddPart(chartPartName, "application/vnd.openxmlformats-officedocument.drawingml.chart+xml", prependXMLHeader(entry.DrawingChartParts[relID]))
				}
				diagramRelIDs := make([]string, 0, len(entry.DrawingDiagramParts))
				for relID := range entry.DrawingDiagramParts {
					diagramRelIDs = append(diagramRelIDs, relID)
				}
				sort.Strings(diagramRelIDs)
				for _, relID := range diagramRelIDs {
					pp := entry.DrawingDiagramParts[relID]
					if pp.External {
						drawingRels = append(drawingRels, rels.Relationship{ID: relID, Type: pp.RelType, Target: pp.Target, TargetMode: "External"})
						continue
					}
					partName := pp.PartName
					if partName == "" {
						partName = fmt.Sprintf("xl/diagrams/data%d.xml", len(diagramRelIDs))
					}
					ct := pp.ContentType
					if ct == "" {
						ct = "application/xml"
					}
					writePreservedPartGraph(opcPkg, pp, partName, ct)
					drawingRels = append(drawingRels, rels.Relationship{ID: relID, Type: pp.RelType, Target: makeRelTarget(drawingPartName, partName)})
				}
				if len(drawingRels) > 0 {
					relsPath := rels.RelationshipsPartName(drawingPartName)
					relsDoc := rels.Document{Relationships: drawingRels}
					relsData, err := marshalRels(relsDoc)
					if err != nil {
						return fmt.Errorf("marshal drawing rels for %s: %w", drawingPartName, err)
					}
					opcPkg.AddPart(relsPath, "application/vnd.openxmlformats-package.relationships+xml", relsData)
				}
			}

			// Write worksheet-level relationships (drawing + hyperlinks + tables).
			var wsRels []rels.Relationship
			if entry.Drawing != nil {
				drawingNum := i + 1
				drawingPartName := fmt.Sprintf("xl/drawings/drawing%d.xml", drawingNum)
				drawingRelID := fmt.Sprintf("rIdDraw%d", drawingNum)
				drawingTarget := makeRelTarget(entry.PartName, drawingPartName)
				wsRels = append(wsRels, rels.Relationship{ID: drawingRelID, Type: relTypeDrawing, Target: drawingTarget})
			}
			for relID, url := range entry.HyperlinkURLs {
				wsRels = append(wsRels, rels.Relationship{ID: relID, Type: relTypeHyperlink, Target: url, TargetMode: "External"})
			}
			// Write table definitions and add relationships.
			if len(entry.Tables) > 0 && entry.Sheet != nil {
				tp := &TableParts{Count: len(entry.Tables), Tables: make([]TablePart, len(entry.Tables))}
				for ti, tbl := range entry.Tables {
					tableNum++
					tablePartName := fmt.Sprintf("xl/tables/table%d.xml", tableNum)
					tableRelID := fmt.Sprintf("rIdTable%d", tableNum)
					tp.Tables[ti] = TablePart{RelID: tableRelID}
					tableData, err := xml.Marshal(tbl)
					if err != nil {
						return fmt.Errorf("marshal table %s: %w", tablePartName, err)
					}
					opcPkg.AddPart(tablePartName, "application/vnd.openxmlformats-officedocument.spreadsheetml.table+xml", prependXMLHeader(tableData))
					tableTarget := makeRelTarget(entry.PartName, tablePartName)
					wsRels = append(wsRels, rels.Relationship{ID: tableRelID, Type: relTypeTable, Target: tableTarget})
				}
				entry.Sheet.TableParts = tp
			}
			// Write comments part and add worksheet relationship.
			if entry.Comments != nil {
				commentsNum := i + 1
				commentsPartName := fmt.Sprintf("xl/comments%d.xml", commentsNum)
				commentsRelID := fmt.Sprintf("rIdComments%d", commentsNum)
				commentsData, err := xml.Marshal(entry.Comments)
				if err != nil {
					return fmt.Errorf("marshal comments %s: %w", commentsPartName, err)
				}
				opcPkg.AddPart(commentsPartName, "application/vnd.openxmlformats-officedocument.spreadsheetml.comments+xml", prependXMLHeader(commentsData))
				commentsTarget := makeRelTarget(entry.PartName, commentsPartName)
				wsRels = append(wsRels, rels.Relationship{ID: commentsRelID, Type: relTypeComments, Target: commentsTarget})
			}
			// Preserve embedded/linked OLE object payload relationships.
			if entry.Sheet != nil && entry.Sheet.OLEObjects != nil {
				for _, obj := range entry.Sheet.OLEObjects.Objects {
					relID := normalizeRelID(obj.RelID)
					pp, ok := entry.OLEParts[relID]
					if !ok {
						continue
					}
					if pp.External {
						wsRels = append(wsRels, rels.Relationship{ID: relID, Type: pp.RelType, Target: pp.Target, TargetMode: "External"})
						continue
					}
					partName := pp.PartName
					if partName == "" {
						partName = fmt.Sprintf("xl/embeddings/oleObject%d.bin", i+1)
					}
					ct := pp.ContentType
					if ct == "" {
						ct = "application/vnd.openxmlformats-officedocument.oleObject"
					}
					opcPkg.AddPart(partName, ct, pp.Data)
					relType := pp.RelType
					if relType == "" {
						relType = relTypeOLEObject
					}
					wsRels = append(wsRels, rels.Relationship{ID: relID, Type: relType, Target: makeRelTarget(entry.PartName, partName)})
				}
			}

			var err error
			wsRels, err = writePivotTables(entry, relMgr, opcPkg, &ptNum, &cacheNum, pkg.Workbook, wsRels)
			if err != nil {
				return err
			}

			// Write legacy VML drawing parts (form controls / comment indicators / header-footer images).
			entry.LegacyDrawingVML = MergeLegacyDrawingVMLForWorksheet(entry.LegacyDrawingVML, entry.FormControls, entry.Comments)
			if entry.Sheet != nil {
				entry.Sheet.LegacyDrawing = nil
			}
			if len(entry.LegacyDrawingVML) > 0 {
				vmlNum++
				vmlPartName := fmt.Sprintf("xl/drawings/vmlDrawing%d.vml", vmlNum)
				vmlRelID := fmt.Sprintf("rIdVML%d", vmlNum)
				opcPkg.AddPart(vmlPartName, "application/vnd.openxmlformats-officedocument.vmlDrawing", entry.LegacyDrawingVML)
				wsRels = append(wsRels, rels.Relationship{ID: vmlRelID, Type: relTypeVMLDrawing, Target: makeRelTarget(entry.PartName, vmlPartName)})
				entry.Sheet.LegacyDrawing = &DrawingRef{RelID: vmlRelID}
			}
			if len(entry.LegacyDrawingHFVML) > 0 {
				vmlNum++
				vmlPartName := fmt.Sprintf("xl/drawings/vmlDrawing%d.vml", vmlNum)
				vmlRelID := fmt.Sprintf("rIdVMLHF%d", vmlNum)
				opcPkg.AddPart(vmlPartName, "application/vnd.openxmlformats-officedocument.vmlDrawing", entry.LegacyDrawingHFVML)
				wsRels = append(wsRels, rels.Relationship{ID: vmlRelID, Type: relTypeVMLDrawing, Target: makeRelTarget(entry.PartName, vmlPartName)})
				entry.Sheet.LegacyDrawingHF = &DrawingRef{RelID: vmlRelID}
			}

			if len(wsRels) > 0 {
				relsPath := rels.RelationshipsPartName(entry.PartName)
				relsDoc := rels.Document{Relationships: wsRels}
				relsData, err := marshalRels(relsDoc)
				if err != nil {
					return fmt.Errorf("marshal worksheet rels for %s: %w", entry.PartName, err)
				}
				opcPkg.AddPart(relsPath, "application/vnd.openxmlformats-package.relationships+xml", relsData)
			}

			if entry.Sheet != nil {
				wsData, err := xml.Marshal(entry.Sheet)
				if err != nil {
					return fmt.Errorf("marshal worksheet %s: %w", entry.PartName, err)
				}
				opcPkg.AddPart(entry.PartName, "application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml", prependXMLHeader(wsData))
			}
		}

		// Shared strings.
		if pkg.SharedStrings != nil {
			rel := relMgr.addWorkbookRel(relTypeSharedStrings, "sharedstrings.xml", "")
			ssData, err := xml.Marshal(pkg.SharedStrings)
			if err != nil {
				return fmt.Errorf("marshal shared strings: %w", err)
			}
			opcPkg.AddPart("xl/sharedstrings.xml", "application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml", prependXMLHeader(ssData))
			_ = rel.ID
		}

		// Styles.
		if pkg.Styles != nil {
			rel := relMgr.addWorkbookRel(relTypeStyles, "styles.xml", "")
			stylesData, err := xml.Marshal(pkg.Styles)
			if err != nil {
				return fmt.Errorf("marshal styles: %w", err)
			}
			opcPkg.AddPart("xl/styles.xml", "application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml", prependXMLHeader(stylesData))
			_ = rel.ID
		}

		// Theme.
		if pkg.Theme != nil {
			rel := relMgr.addWorkbookRel(relTypeTheme, "theme/theme1.xml", "")
			themeData, err := xml.Marshal(pkg.Theme)
			if err != nil {
				return fmt.Errorf("marshal theme: %w", err)
			}
			opcPkg.AddPart("xl/theme/theme1.xml", "application/vnd.openxmlformats-officedocument.theme+xml", prependXMLHeader(themeData))
			_ = rel.ID
		}

		// Connections.
		if pkg.Connections != nil {
			rel := relMgr.addWorkbookRel(relTypeConnections, "connections.xml", "")
			connData, err := xml.Marshal(pkg.Connections)
			if err != nil {
				return fmt.Errorf("marshal connections: %w", err)
			}
			opcPkg.AddPart("xl/connections.xml", "application/vnd.openxmlformats-officedocument.spreadsheetml.connections+xml", prependXMLHeader(connData))
			_ = rel.ID
		}

		// External links.
		if len(pkg.ExternalLinks) > 0 {
			pkg.Workbook.ExternalReferences = nil
			for i, entry := range pkg.ExternalLinks {
				linkNum := i + 1
				linkPartName := fmt.Sprintf("xl/externalLinks/externalLink%d.xml", linkNum)
				linkRelID := fmt.Sprintf("rIdExtLink%d", linkNum)
				rel := relMgr.addWorkbookRel(relTypeExternalLink, fmt.Sprintf("externalLinks/externalLink%d.xml", linkNum), linkRelID)
				pkg.Workbook.ExternalReferences = append(pkg.Workbook.ExternalReferences, ExternalReference{RelID: rel.ID})

				linkData, err := xml.Marshal(entry.Link)
				if err != nil {
					return fmt.Errorf("marshal external link %s: %w", linkPartName, err)
				}
				opcPkg.AddPart(linkPartName, "application/vnd.openxmlformats-officedocument.spreadsheetml.externalLink+xml", prependXMLHeader(linkData))

				pathTarget := entry.Target
				if pathTarget == "" {
					pathTarget = "external.xlsx"
				}
				linkRels := []rels.Relationship{{
					ID:         entry.Link.ExternalBook.RelID,
					Type:       relTypeExternalLinkPath,
					Target:     pathTarget,
					TargetMode: "External",
				}}
				relsPath := rels.RelationshipsPartName(linkPartName)
				relsData, err := marshalRels(rels.Document{Relationships: linkRels})
				if err != nil {
					return fmt.Errorf("marshal external link rels for %s: %w", linkPartName, err)
				}
				opcPkg.AddPart(relsPath, "application/vnd.openxmlformats-package.relationships+xml", relsData)
			}
		}

		// Marshal and write workbook.xml now that all workbook-level references are set.
		workbookData, err := xml.Marshal(pkg.Workbook)
		if err != nil {
			return fmt.Errorf("marshal workbook: %w", err)
		}
		opcPkg.AddPart("xl/workbook.xml", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml", prependXMLHeader(workbookData))

		// Workbook rels.
		wbRels := relMgr.workbookRels()
		wbRelsData, err := marshalRels(wbRels)
		if err != nil {
			return fmt.Errorf("marshal workbook rels: %w", err)
		}
		opcPkg.AddPart("xl/_rels/workbook.xml.rels", "application/vnd.openxmlformats-package.relationships+xml", wbRelsData)

		// Root rels.
		rootRels := relMgr.rootRels()
		rootRelsData, err := marshalRels(rootRels)
		if err != nil {
			return fmt.Errorf("marshal root rels: %w", err)
		}
		opcPkg.AddPart("_rels/.rels", "application/vnd.openxmlformats-package.relationships+xml", rootRelsData)
	}

	return opcPkg.Write(w)
}

// prependXMLHeader adds the XML declaration if missing.
func prependXMLHeader(data []byte) []byte {
	if len(data) > 0 && data[0] == '<' {
		return append([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`), data...)
	}
	return data
}

func marshalRels(doc rels.Document) ([]byte, error) {
	var out []byte
	out = append(out, []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)...)
	for _, r := range doc.Relationships {
		out = append(out, []byte(`<Relationship Id="`+r.ID+`" Type="`+r.Type+`" Target="`+r.Target+`"`)...)
		if r.TargetMode != "" {
			out = append(out, []byte(` TargetMode="`+r.TargetMode+`"`)...)
		}
		out = append(out, []byte(`/>`)...)
	}
	out = append(out, []byte(`</Relationships>`)...)
	return out, nil
}

// relPartName returns the part name relative to the xl/ directory for
// relationship targets.
func relPartName(name string) string {
	if len(name) > 3 && name[:3] == "xl/" {
		return name[3:]
	}
	return name
}

type relationshipManager struct {
	nextID       int
	root         []rels.Relationship
	workbook     []rels.Relationship
	usedRoot     map[string]bool
	usedWorkbook map[string]bool
}

func newRelationshipManager() *relationshipManager {
	return &relationshipManager{
		nextID:       1,
		usedRoot:     make(map[string]bool),
		usedWorkbook: make(map[string]bool),
	}
}

func (rm *relationshipManager) nextRelID() string {
	for {
		id := "rId" + strconv.Itoa(rm.nextID)
		rm.nextID++
		if !rm.usedRoot[id] && !rm.usedWorkbook[id] {
			return id
		}
	}
}

func (rm *relationshipManager) nextWorkbookRelID() string {
	for {
		id := "rId" + strconv.Itoa(rm.nextID)
		rm.nextID++
		if !rm.usedWorkbook[id] {
			return id
		}
	}
}

func (rm *relationshipManager) addRootRel(relType, target string) rels.Relationship {
	id := "rId" + strconv.Itoa(rm.nextID)
	rm.nextID++
	for rm.usedRoot[id] {
		id = "rId" + strconv.Itoa(rm.nextID)
		rm.nextID++
	}
	r := rels.Relationship{ID: id, Type: relType, Target: target}
	rm.root = append(rm.root, r)
	rm.usedRoot[id] = true
	return r
}

func (rm *relationshipManager) addWorkbookRel(relType, target, preferredID string) rels.Relationship {
	id := preferredID
	if id == "" || rm.usedWorkbook[id] {
		id = rm.nextWorkbookRelID()
	}
	r := rels.Relationship{ID: id, Type: relType, Target: target}
	rm.workbook = append(rm.workbook, r)
	rm.usedWorkbook[id] = true
	return r
}

func (rm *relationshipManager) rootRels() rels.Document {
	return rels.Document{Relationships: rm.root}
}

func (rm *relationshipManager) workbookRels() rels.Document {
	return rels.Document{Relationships: rm.workbook}
}

func makeRelTarget(sourcePart, targetPart string) string {
	sourceDir := path.Dir(sourcePart)
	targetDir := path.Dir(targetPart)
	targetFile := path.Base(targetPart)
	if sourceDir == targetDir {
		return targetFile
	}
	var ups []string
	for {
		if strings.HasPrefix(targetDir, sourceDir+"/") {
			rel := strings.TrimPrefix(targetDir, sourceDir+"/")
			if len(ups) > 0 {
				return strings.Join(ups, "/") + "/" + rel + "/" + targetFile
			}
			return rel + "/" + targetFile
		}
		if sourceDir == "" || sourceDir == "." {
			break
		}
		ups = append(ups, "..")
		sourceDir = path.Dir(sourceDir)
	}
	if len(ups) > 0 {
		return strings.Join(ups, "/") + "/" + targetDir + "/" + targetFile
	}
	return targetDir + "/" + targetFile
}

// writePivotTables serializes any pivot table/cache/records parts for a worksheet
// and wires up the worksheet, pivot table, cache, and workbook relationships.
// Raw bytes from the original file are preferred when present.
func writePivotTables(entry *WorksheetEntry, relMgr *relationshipManager, opcPkg *opc.Package, ptNum, cacheNum *int, wb *Workbook, wsRels []rels.Relationship) ([]rels.Relationship, error) {
	if len(entry.PivotTables) == 0 || entry.Sheet == nil {
		return wsRels, nil
	}
	refs := &PivotTableRefs{Count: len(entry.PivotTables)}
	for _, ptc := range entry.PivotTables {
		if ptc.Table == nil {
			continue
		}
		*ptNum++
		*cacheNum++
		ptPartName := fmt.Sprintf("xl/pivotTables/pivotTable%d.xml", *ptNum)
		cachePartName := fmt.Sprintf("xl/pivotCache/pivotCacheDefinition%d.xml", *cacheNum)
		recordsPartName := fmt.Sprintf("xl/pivotCache/pivotCacheRecords%d.xml", *cacheNum)

		ptRelID := fmt.Sprintf("rIdPivotTable%d", *ptNum)
		cacheRelID := fmt.Sprintf("rIdPivotCache%d", *cacheNum)

		// Worksheet -> pivot table.
		wsRels = append(wsRels, rels.Relationship{ID: ptRelID, Type: relTypePivotTable, Target: makeRelTarget(entry.PartName, ptPartName)})
		refs.Tables = append(refs.Tables, PivotTablePart{RelID: ptRelID, Name: ptc.Table.Name})

		cacheID := ptc.Table.CacheID
		if cacheID == 0 {
			cacheID = *cacheNum
		}

		// Pivot table part.
		ptData := ptc.Table.Raw
		if len(ptData) == 0 {
			var err error
			ptData, err = xml.Marshal(ptc.Table)
			if err != nil {
				return wsRels, fmt.Errorf("marshal pivot table %s: %w", ptPartName, err)
			}
		}
		opcPkg.AddPart(ptPartName, "application/vnd.openxmlformats-officedocument.spreadsheetml.pivotTable+xml", prependXMLHeader(ptData))

		// Pivot table -> cache definition relationship.
		ptRels := []rels.Relationship{{
			ID:     "rId1",
			Type:   relTypePivotCache,
			Target: makeRelTarget(ptPartName, cachePartName),
		}}
		ptRelsPath := rels.RelationshipsPartName(ptPartName)
		ptRelsData, err := marshalRels(rels.Document{Relationships: ptRels})
		if err != nil {
			return wsRels, fmt.Errorf("marshal pivot table rels for %s: %w", ptPartName, err)
		}
		opcPkg.AddPart(ptRelsPath, "application/vnd.openxmlformats-package.relationships+xml", ptRelsData)

		if ptc.Cache == nil {
			continue
		}

		// Pivot cache definition part.
		cacheData := ptc.Cache.Raw
		if len(cacheData) == 0 {
			cacheData, err = xml.Marshal(ptc.Cache)
			if err != nil {
				return wsRels, fmt.Errorf("marshal pivot cache %s: %w", cachePartName, err)
			}
		}
		opcPkg.AddPart(cachePartName, "application/vnd.openxmlformats-officedocument.spreadsheetml.pivotCacheDefinition+xml", prependXMLHeader(cacheData))

		// Cache definition -> records relationship and records part.
		if ptc.Records != nil {
			recordsData := ptc.Records.Raw
			if len(recordsData) == 0 {
				recordsData, err = xml.Marshal(ptc.Records)
				if err != nil {
					return wsRels, fmt.Errorf("marshal pivot cache records %s: %w", recordsPartName, err)
				}
			}
			cacheRels := []rels.Relationship{{
				ID:     "rId1",
				Type:   relTypePivotCacheRecords,
				Target: makeRelTarget(cachePartName, recordsPartName),
			}}
			cacheRelsPath := rels.RelationshipsPartName(cachePartName)
			cacheRelsData, err := marshalRels(rels.Document{Relationships: cacheRels})
			if err != nil {
				return wsRels, fmt.Errorf("marshal pivot cache rels for %s: %w", cachePartName, err)
			}
			opcPkg.AddPart(cacheRelsPath, "application/vnd.openxmlformats-package.relationships+xml", cacheRelsData)
			opcPkg.AddPart(recordsPartName, "application/vnd.openxmlformats-officedocument.spreadsheetml.pivotCacheRecords+xml", prependXMLHeader(recordsData))
		}

		// Workbook -> cache definition relationship.
		cacheTarget := makeRelTarget("xl/workbook.xml", cachePartName)
		wbRel := relMgr.addWorkbookRel(relTypePivotCache, cacheTarget, cacheRelID)
		wb.PivotCaches = append(wb.PivotCaches, PivotCache{CacheID: cacheID, RelID: wbRel.ID})
	}
	entry.Sheet.PivotTables = refs
	return wsRels, nil
}
