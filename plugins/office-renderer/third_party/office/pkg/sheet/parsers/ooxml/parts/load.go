package parts

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"office/pkg/diag"
	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/media"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
)

// LoadPackage parses the typed SpreadsheetML parts from a generic OPC package.
func LoadPackage(pkg *opc.Package) (*Package, error) {
	return loadPackage(pkg, false)
}

// LoadMetadataPackage loads package-level SpreadsheetML metadata plus a sparse
// worksheet identity skeleton. Worksheet content and relationship-heavy object
// graphs are owned by the streaming and relationship-object paths respectively.
func LoadMetadataPackage(pkg *opc.Package) (*Package, error) {
	return loadPackage(pkg, true)
}

func loadPackage(pkg *opc.Package, metadataOnly bool) (*Package, error) {
	if pkg == nil {
		return nil, fmt.Errorf("sheet parts: package cannot be nil")
	}
	if pkg.PackageType() != opc.PackageTypeSpreadsheet {
		return nil, fmt.Errorf("sheet parts: expected %q package, got %q", opc.PackageTypeSpreadsheet, pkg.PackageType())
	}

	mainPartName, err := findMainPart(pkg)
	if err != nil {
		return nil, err
	}

	mainPart, ok := pkg.Part(mainPartName)
	if !ok {
		return nil, fmt.Errorf("sheet parts: main workbook part %q not found", mainPartName)
	}

	var wb Workbook
	if err := xml.Unmarshal(mainPart.Data, &wb); err != nil {
		return nil, fmt.Errorf("sheet parts: parse %s: %w", mainPartName, err)
	}

	out := &Package{
		MainPartName: mainPartName,
		Workbook:     &wb,
		MediaParts:   make(map[string]media.Part),
		ChartParts:   make(map[string][]byte),
	}

	wbRels, ok, err := pkg.Relationships(mainPartName)
	if err != nil {
		return nil, fmt.Errorf("sheet parts: relationships for %s: %w", mainPartName, err)
	}
	if !ok {
		return out, nil
	}

	relsByID := indexRelsByID(wbRels)
	relsByType := indexRelsByType(wbRels)

	if err := loadSharedStrings(pkg, mainPartName, relsByType, out); err != nil {
		return nil, err
	}
	if err := loadStyles(pkg, mainPartName, relsByType, out); err != nil {
		return nil, err
	}
	loadTheme(pkg, mainPartName, relsByType, out)
	loadFontTable(pkg, mainPartName, relsByType, out)
	if err := loadConnections(pkg, mainPartName, relsByType, out); err != nil {
		return nil, err
	}
	if err := loadExternalLinks(pkg, mainPartName, relsByType, out); err != nil {
		return nil, err
	}
	if metadataOnly {
		if err := loadWorksheetSkeletons(pkg, mainPartName, relsByID, out); err != nil {
			return nil, err
		}
	} else if err := loadWorksheets(pkg, mainPartName, relsByID, out); err != nil {
		return nil, err
	}
	if !metadataOnly {
		// Full-loader oracle retains its historical package-level media/chart
		// projection. Production object resolution is worksheet-relationship-owned.
		loadMediaParts(pkg, mainPartName, relsByType, out)
		loadChartParts(pkg, mainPartName, relsByType, out)
	}

	out.Diagnostics = collectPackageDiagnostics(out)
	return out, nil
}

func collectPackageDiagnostics(pkg *Package) diag.List {
	var d diag.List
	for _, ws := range pkg.Worksheets {
		if ws.PartName == "" {
			d.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     "sheet/missing-worksheet-part",
				Message:  fmt.Sprintf("worksheet %q: relationship not resolved to a part", ws.Name),
			})
		} else if ws.Sheet == nil {
			d.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     "sheet/empty-worksheet",
				Message:  fmt.Sprintf("worksheet %q: part loaded but parsed as empty", ws.Name),
			})
			continue
		}
		appendWorksheetFeatureDiagnostics(&d, ws)
		appendDrawingFeatureDiagnostics(&d, ws)
	}
	return d
}

func appendWorksheetFeatureDiagnostics(diagnostics *diag.List, entry *WorksheetEntry) {
	if diagnostics == nil || entry == nil || entry.Sheet == nil {
		return
	}
	ws := entry.Sheet
	if ws.LegacyDrawing != nil && ws.LegacyDrawing.RelID != "" && len(entry.LegacyNotes) == 0 && len(entry.FormControls) == 0 && len(entry.LegacyDrawingVML) == 0 {
		diagnostics.Add(diag.Diagnostic{
			Severity: diag.SeverityWarning,
			Code:     "sheet/unsupported-vml",
			Message:  "Legacy VML drawing content is not supported and was skipped",
			Location: diag.Location{Part: "worksheet/legacyDrawing"},
		})
	}
	if ws.LegacyDrawingHF != nil && ws.LegacyDrawingHF.RelID != "" && len(entry.HFImages) == 0 && len(entry.LegacyDrawingHFVML) == 0 {
		diagnostics.Add(diag.Diagnostic{
			Severity: diag.SeverityWarning,
			Code:     "sheet/unsupported-vml",
			Message:  "Legacy VML header/footer drawing content is not supported and was skipped",
			Location: diag.Location{Part: "worksheet/legacyDrawingHF"},
		})
	}
	if ws.OLEObjects != nil && len(entry.OLEParts) == 0 {
		diagnostics.Add(diag.Diagnostic{
			Severity: diag.SeverityWarning,
			Code:     "sheet/unsupported-ole",
			Message:  "OLE object content is not supported and was skipped",
			Location: diag.Location{Part: "worksheet/oleObjects"},
		})
	}
}

func appendDrawingFeatureDiagnostics(diagnostics *diag.List, entry *WorksheetEntry) {
	if diagnostics == nil || entry == nil || entry.Drawing == nil {
		return
	}
	for i := range entry.Drawing.OneCellAnchors {
		appendGraphicFrameDiagnostic(diagnostics, entry.Drawing.OneCellAnchors[i].GraphicFrame, "drawing/oneCellAnchor/graphicFrame")
	}
	for i := range entry.Drawing.TwoCellAnchors {
		appendGraphicFrameDiagnostic(diagnostics, entry.Drawing.TwoCellAnchors[i].GraphicFrame, "drawing/twoCellAnchor/graphicFrame")
	}
	for i := range entry.Drawing.AbsoluteAnchors {
		appendGraphicFrameDiagnostic(diagnostics, entry.Drawing.AbsoluteAnchors[i].GraphicFrame, "drawing/absoluteAnchor/graphicFrame")
	}
}

func appendGraphicFrameDiagnostic(diagnostics *diag.List, frame *XDRGraphicFrame, path string) {
	if diagnostics == nil || frame == nil {
		return
	}
	uri := normalizeSheetGraphicDataURI(frame.Graphic.GraphicData.URI)
	switch uri {
	case drawingGraphicDataURIChartTransitional, drawingGraphicDataURIChartStrict:
		// Charts are now supported - no diagnostic needed
		return
	case drawingGraphicDataURIDiagramTransitional, drawingGraphicDataURIDiagramStrict:
		if frame.Graphic.GraphicData.RelIDs != nil {
			return
		}
		diagnostics.Add(diag.Diagnostic{
			Severity: diag.SeverityWarning,
			Code:     "sheet/unsupported-smartart",
			Message:  "SmartArt content is not supported and was skipped",
			Location: diag.Location{Part: path},
		})
	case drawingGraphicDataURIOLE:
		diagnostics.Add(diag.Diagnostic{
			Severity: diag.SeverityWarning,
			Code:     "sheet/unsupported-ole",
			Message:  "OLE object content is not supported and was skipped",
			Location: diag.Location{Part: path},
		})
	}
	if frame.Graphic.GraphicData.Chart != nil && uri == "" {
		// Charts are now supported - no diagnostic needed
		return
	}
}

func graphicFrameName(frame *XDRGraphicFrame) string {
	if frame == nil {
		return ""
	}
	name := frame.NvGraphicFramePr.CNvPr.Name
	if title := frame.NvGraphicFramePr.CNvPr.Title; title != "" {
		name = title
	}
	if descr := frame.NvGraphicFramePr.CNvPr.Descr; descr != "" {
		name = descr
	}
	return name
}

func normalizeSheetGraphicDataURI(uri string) string {
	uri = strings.TrimSpace(strings.ToLower(uri))
	switch {
	case strings.Contains(uri, "/chart"):
		return drawingGraphicDataURIChartTransitional
	case strings.Contains(uri, "/diagram"):
		return drawingGraphicDataURIDiagramTransitional
	case strings.Contains(uri, "ole"):
		return drawingGraphicDataURIOLE
	default:
		return uri
	}
}

func findMainPart(pkg *opc.Package) (string, error) {
	rootRels, ok, err := pkg.Relationships("")
	if err != nil {
		return "", fmt.Errorf("sheet parts: root relationships: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("sheet parts: root relationships not found")
	}
	for _, r := range rootRels.Relationships {
		if r.Type == relTypeOfficeDocument && !r.IsExternal() {
			return rels.ResolveTarget("", r.Target), nil
		}
	}
	return "", fmt.Errorf("sheet parts: officeDocument relationship not found")
}

func loadSharedStrings(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) error { //nolint:unparam
	for _, r := range byType[relTypeSharedStrings] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		var ss SharedStrings
		if err := xml.Unmarshal(p.Data, &ss); err != nil {
			return fmt.Errorf("sheet parts: parse %s: %w", partName, err)
		}
		out.SharedStrings = &ss
		return nil
	}
	return nil
}

func loadStyles(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) error {
	for _, r := range byType[relTypeStyles] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		var ss StyleSheet
		if err := xml.Unmarshal(p.Data, &ss); err != nil {
			return fmt.Errorf("sheet parts: parse %s: %w", partName, err)
		}
		out.Styles = &ss
		return nil
	}
	return nil
}

func loadTheme(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) {
	for _, r := range byType[relTypeTheme] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		var theme Theme
		if err := xml.Unmarshal(p.Data, &theme); err != nil {
			continue
		}
		out.Theme = &theme
		return
	}
}

func loadFontTable(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) {
	for _, r := range byType[relTypeFont] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		var fontTable FontTable
		if err := xml.Unmarshal(p.Data, &fontTable); err != nil {
			continue
		}
		out.FontTable = &fontTable
		loadEmbeddedFonts(pkg, partName, &fontTable, out)
		return
	}
}

func loadConnections(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) error {
	for _, r := range byType[relTypeConnections] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		var connections Connections
		if err := xml.Unmarshal(p.Data, &connections); err != nil {
			return fmt.Errorf("sheet parts: parse %s: %w", partName, err)
		}
		out.Connections = &connections
		return nil
	}
	return nil
}

func loadExternalLinks(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) error {
	for _, r := range byType[relTypeExternalLink] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		var link ExternalLink
		if err := xml.Unmarshal(p.Data, &link); err != nil {
			return fmt.Errorf("sheet parts: parse %s: %w", partName, err)
		}
		entry := &ExternalLinkEntry{RelID: normalizeRelID(r.ID), PartName: partName, Link: &link}
		if target, ok := resolveExternalLinkTarget(pkg, partName, &link); ok {
			entry.Target = target
		}
		out.ExternalLinks = append(out.ExternalLinks, entry)
	}
	return nil
}

func resolveExternalLinkTarget(pkg *opc.Package, partName string, link *ExternalLink) (string, bool) {
	if pkg == nil || partName == "" || link == nil || link.ExternalBook == nil || link.ExternalBook.RelID == "" {
		return "", false
	}
	partRels, ok, err := pkg.Relationships(partName)
	if err != nil || !ok || partRels == nil {
		return "", false
	}
	for _, r := range partRels.Relationships {
		if normalizeRelID(r.ID) != normalizeRelID(link.ExternalBook.RelID) {
			continue
		}
		if r.Type != relTypeExternalLinkPath {
			continue
		}
		if r.IsExternal() {
			return r.Target, true
		}
		return rels.ResolveTarget(partName, r.Target), true
	}
	return "", false
}

// relationshipWorksheetMetadata contains only worksheet elements that point to
// relationship-heavy object graphs (drawing/VML/OLE/table/pivot) plus the
// header/footer text that owns legacy header/footer image placeholders.
// It is not part of the worksheet metadata skeleton: production loads this
// object graph explicitly through LoadRelationshipObjects.
type relationshipWorksheetMetadata struct {
	DrawingRef      *DrawingRef     `xml:"drawing"`
	LegacyDrawing   *DrawingRef     `xml:"legacyDrawing"`
	LegacyDrawingHF *DrawingRef     `xml:"legacyDrawingHF"`
	OLEObjects      *OLEObjects     `xml:"oleObjects"`
	HeaderFooter    *HeaderFooter   `xml:"headerFooter"`
	TableParts      *TableParts     `xml:"tableParts"`
	PivotTables     *PivotTableRefs `xml:"pivotTables"`
}

func loadWorksheetSkeletons(pkg *opc.Package, mainPartName string, byID map[string]rels.Relationship, out *Package) error {
	for _, ref := range out.Workbook.Sheets {
		entry := &WorksheetEntry{
			SheetID: ref.SheetID,
			RelID:   normalizeRelID(ref.RelID),
			Name:    ref.Name,
			Sheet:   &Worksheet{},
		}
		r, ok := byID[entry.RelID]
		if ok && r.Type == relTypeWorksheet && !r.IsExternal() {
			entry.PartName = rels.ResolveTarget(mainPartName, r.Target)
		}
		out.Worksheets = append(out.Worksheets, entry)
	}
	return nil
}

// LoadRelationshipObjects loads the complete worksheet object relationship
// layer after the package metadata skeleton has been
// established. This deliberately keeps drawings, charts, tables, pivots, VML,
// form controls, OLE payloads and header/footer images out of the metadata
// worksheet parser and gives them one explicit production ownership boundary.
func LoadRelationshipObjects(pkg *opc.Package, out *Package) error {
	if pkg == nil || out == nil {
		return nil
	}
	for _, entry := range out.Worksheets {
		if entry == nil || entry.PartName == "" {
			continue
		}
		p, ok := pkg.Part(entry.PartName)
		if !ok {
			continue
		}
		var meta relationshipWorksheetMetadata
		if err := xml.Unmarshal(p.Data, &meta); err != nil {
			return fmt.Errorf("sheet parts: parse relationship metadata %s: %w", entry.PartName, err)
		}
		if entry.Sheet == nil {
			entry.Sheet = &Worksheet{}
		}
		entry.Sheet.DrawingRef = meta.DrawingRef
		entry.Sheet.LegacyDrawing = meta.LegacyDrawing
		entry.Sheet.LegacyDrawingHF = meta.LegacyDrawingHF
		entry.Sheet.OLEObjects = meta.OLEObjects
		entry.Sheet.HeaderFooter = meta.HeaderFooter
		entry.Sheet.TableParts = meta.TableParts
		entry.Sheet.PivotTables = meta.PivotTables

		if meta.DrawingRef != nil && meta.DrawingRef.RelID != "" {
			loadWorksheetDrawing(pkg, entry.PartName, normalizeRelID(meta.DrawingRef.RelID), entry)
		}
		loadWorksheetLegacyDrawingRaw(pkg, entry.PartName, entry)
		loadWorksheetLegacyNotes(pkg, entry.PartName, entry)
		loadWorksheetFormControls(pkg, entry.PartName, entry)
		loadWorksheetLegacyHFImages(pkg, entry.PartName, entry)
		loadWorksheetPivotTables(pkg, entry.PartName, entry)
		loadWorksheetTables(pkg, entry.PartName, entry)
		loadWorksheetOLEObjects(pkg, entry.PartName, entry)
	}
	// Object-related diagnostics are meaningful only after this ownership layer
	// has populated VML, controls, drawings, tables, pivots and OLE graphs.
	out.Diagnostics = collectPackageDiagnostics(out)
	return nil
}

func loadWorksheets(pkg *opc.Package, mainPartName string, byID map[string]rels.Relationship, out *Package) error {
	for _, ref := range out.Workbook.Sheets {
		entry := &WorksheetEntry{
			SheetID: ref.SheetID,
			RelID:   normalizeRelID(ref.RelID),
			Name:    ref.Name,
		}

		r, ok := byID[entry.RelID]
		if !ok {
			out.Worksheets = append(out.Worksheets, entry)
			continue
		}
		if r.Type != relTypeWorksheet || r.IsExternal() {
			out.Worksheets = append(out.Worksheets, entry)
			continue
		}

		partName := rels.ResolveTarget(mainPartName, r.Target)
		entry.PartName = partName

		p, ok := pkg.Part(partName)
		if !ok {
			out.Worksheets = append(out.Worksheets, entry)
			continue
		}

		var ws Worksheet
		if err := xml.Unmarshal(p.Data, &ws); err != nil {
			return fmt.Errorf("sheet parts: parse %s: %w", partName, err)
		}
		entry.Sheet = &ws

		if ws.DrawingRef != nil && ws.DrawingRef.RelID != "" {
			loadWorksheetDrawing(pkg, partName, normalizeRelID(ws.DrawingRef.RelID), entry)
		}
		if len(ws.Hyperlinks) > 0 {
			loadWorksheetHyperlinkURLs(pkg, partName, entry)
		}
		loadWorksheetComments(pkg, partName, entry)
		loadWorksheetLegacyDrawingRaw(pkg, partName, entry)
		loadWorksheetLegacyNotes(pkg, partName, entry)
		loadWorksheetFormControls(pkg, partName, entry)
		loadWorksheetLegacyHFImages(pkg, partName, entry)
		loadWorksheetPivotTables(pkg, partName, entry)
		loadWorksheetTables(pkg, partName, entry)
		loadWorksheetOLEObjects(pkg, partName, entry)

		out.Worksheets = append(out.Worksheets, entry)
	}
	return nil
}

func loadWorksheetDrawing(pkg *opc.Package, wsPartName, drawingRelID string, entry *WorksheetEntry) {
	wsRels, ok, err := pkg.Relationships(wsPartName)
	if err != nil || !ok {
		return
	}
	wsRelsByID := indexRelsByID(wsRels)
	dr, ok := wsRelsByID[drawingRelID]
	if !ok || dr.Type != relTypeDrawing || dr.IsExternal() {
		return
	}

	drawingPartName := rels.ResolveTarget(wsPartName, dr.Target)
	p, ok := pkg.Part(drawingPartName)
	if !ok {
		return
	}

	var sd SpreadsheetDrawing
	if err := xml.Unmarshal(p.Data, &sd); err != nil {
		return
	}
	entry.Drawing = &sd

	// Load drawing-level relationships to resolve image media and charts.
	drawingRels, ok, err := pkg.Relationships(drawingPartName)
	if err != nil || !ok {
		return
	}
	entry.DrawingMedia = make(map[string]media.Part)
	entry.DrawingChartParts = make(map[string][]byte)
	for _, r := range drawingRels.Relationships {
		switch {
		case r.Type == relTypeImage && !r.IsExternal():
			mediaPartName := rels.ResolveTarget(drawingPartName, r.Target)
			mp, ok := pkg.Part(mediaPartName)
			if !ok {
				continue
			}
			entry.DrawingMedia[normalizeRelID(r.ID)] = media.Part{
				Name:        mediaPartName,
				ContentType: mp.ContentType,
				Data:        mp.Data,
			}
		case r.Type == relTypeChart && !r.IsExternal():
			chartPartName := rels.ResolveTarget(drawingPartName, r.Target)
			cp, ok := pkg.Part(chartPartName)
			if !ok {
				continue
			}
			entry.DrawingChartParts[normalizeRelID(r.ID)] = cp.Data
		case isDiagramRelationshipType(r.Type):
			if entry.DrawingDiagramParts == nil {
				entry.DrawingDiagramParts = make(map[string]PreservedPart)
			}
			relID := normalizeRelID(r.ID)
			pp := PreservedPart{RelID: relID, RelType: r.Type, External: r.IsExternal(), Target: r.Target}
			if !r.IsExternal() {
				partName := rels.ResolveTarget(drawingPartName, r.Target)
				pp = loadPreservedPartGraph(pkg, partName, pp, map[string]bool{})
			}
			entry.DrawingDiagramParts[relID] = pp
		}
	}
}

func isDiagramRelationshipType(relType string) bool {
	switch relType {
	case relTypeDiagramData, relTypeDiagramLayout, relTypeDiagramQuickStyle, relTypeDiagramColors, relTypeDiagramDrawing:
		return true
	default:
		return strings.Contains(strings.ToLower(relType), "diagram")
	}
}

func loadWorksheetOLEObjects(pkg *opc.Package, wsPartName string, entry *WorksheetEntry) {
	if pkg == nil || entry == nil || entry.Sheet == nil || entry.Sheet.OLEObjects == nil {
		return
	}
	wsRels, ok, err := pkg.Relationships(wsPartName)
	if err != nil || !ok {
		return
	}
	byID := indexRelsByID(wsRels)
	for _, obj := range entry.Sheet.OLEObjects.Objects {
		relID := normalizeRelID(obj.RelID)
		r, ok := byID[relID]
		if !ok {
			continue
		}
		if entry.OLEParts == nil {
			entry.OLEParts = make(map[string]PreservedPart)
		}
		pp := PreservedPart{RelID: relID, RelType: r.Type, External: r.IsExternal(), Target: r.Target}
		if !r.IsExternal() {
			partName := rels.ResolveTarget(wsPartName, r.Target)
			pp = loadPreservedPartGraph(pkg, partName, pp, map[string]bool{})
		}
		entry.OLEParts[relID] = pp
	}
}

func loadPreservedPartGraph(pkg *opc.Package, partName string, base PreservedPart, seen map[string]bool) PreservedPart {
	if pkg == nil || partName == "" || seen[partName] {
		return base
	}
	seen[partName] = true
	part, ok := pkg.Part(partName)
	if !ok {
		return base
	}
	base.PartName = partName
	base.ContentType = part.ContentType
	base.Data = append([]byte(nil), part.Data...)
	rdoc, ok, err := pkg.Relationships(partName)
	if err != nil || !ok {
		return base
	}
	base.Children = make(map[string]PreservedPart)
	for _, r := range rdoc.Relationships {
		pr := PreservedRelationship{ID: r.ID, Type: r.Type, Target: r.Target, TargetMode: r.TargetMode}
		base.Relationships = append(base.Relationships, pr)
		if r.IsExternal() {
			continue
		}
		childName := rels.ResolveTarget(partName, r.Target)
		child := PreservedPart{RelID: r.ID, RelType: r.Type, Target: r.Target}
		child = loadPreservedPartGraph(pkg, childName, child, seen)
		if child.PartName != "" {
			base.Children[child.PartName] = child
		}
	}
	if len(base.Children) == 0 {
		base.Children = nil
	}
	return base
}

func loadWorksheetHyperlinkURLs(pkg *opc.Package, wsPartName string, entry *WorksheetEntry) {
	if entry.Sheet == nil || len(entry.Sheet.Hyperlinks) == 0 {
		return
	}
	wsRels, ok, err := pkg.Relationships(wsPartName)
	if err != nil || !ok {
		return
	}
	relsByID := indexRelsByID(wsRels)
	for _, hl := range entry.Sheet.Hyperlinks {
		if hl.RelID == "" {
			continue
		}
		relID := normalizeRelID(hl.RelID)
		r, ok := relsByID[relID]
		if !ok || !r.IsExternal() {
			continue
		}
		if entry.HyperlinkURLs == nil {
			entry.HyperlinkURLs = make(map[string]string)
		}
		entry.HyperlinkURLs[relID] = r.Target
	}
}

func loadWorksheetComments(pkg *opc.Package, wsPartName string, entry *WorksheetEntry) {
	wsRels, ok, err := pkg.Relationships(wsPartName)
	if err != nil || !ok {
		return
	}
	relsByType := indexRelsByType(wsRels)
	for _, r := range relsByType[relTypeComments] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(wsPartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		var c Comments
		if err := xml.Unmarshal(p.Data, &c); err != nil {
			continue
		}
		entry.Comments = &c
		break // only one comments part per worksheet
	}
}

func loadWorksheetLegacyDrawingRaw(pkg *opc.Package, wsPartName string, entry *WorksheetEntry) {
	if pkg == nil || entry == nil || entry.Sheet == nil {
		return
	}
	wsRels, ok, err := pkg.Relationships(wsPartName)
	if err != nil || !ok {
		return
	}
	if entry.Sheet.LegacyDrawing != nil {
		relID := normalizeRelID(entry.Sheet.LegacyDrawing.RelID)
		for _, r := range wsRels.Relationships {
			if normalizeRelID(r.ID) == relID && r.Type == relTypeVMLDrawing && !r.IsExternal() {
				partName := rels.ResolveTarget(wsPartName, r.Target)
				if p, ok := pkg.Part(partName); ok {
					entry.LegacyDrawingVML = p.Data
					entry.VMLObjects = ParseLegacyDrawingVMLObjects(p.Data)
				}
				break
			}
		}
	}
	if entry.Sheet.LegacyDrawingHF != nil {
		relID := normalizeRelID(entry.Sheet.LegacyDrawingHF.RelID)
		for _, r := range wsRels.Relationships {
			if normalizeRelID(r.ID) == relID && r.Type == relTypeVMLDrawing && !r.IsExternal() {
				partName := rels.ResolveTarget(wsPartName, r.Target)
				if p, ok := pkg.Part(partName); ok {
					entry.LegacyDrawingHFVML = p.Data
				}
				break
			}
		}
	}
}

func loadWorksheetLegacyNotes(pkg *opc.Package, wsPartName string, entry *WorksheetEntry) {
	if pkg == nil || entry == nil || entry.Sheet == nil || entry.Sheet.LegacyDrawing == nil {
		return
	}
	wsRels, ok, err := pkg.Relationships(wsPartName)
	if err != nil || !ok {
		return
	}
	relsByType := indexRelsByType(wsRels)
	for _, r := range relsByType[relTypeVMLDrawing] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(wsPartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		notes := parseLegacyNotesVML(p.Data)
		if len(notes) == 0 {
			notes = legacyNotesFromComments(entry.Comments)
		}
		if len(notes) == 0 {
			continue
		}
		entry.LegacyNotes = notes
		break
	}
}

func parseLegacyNotesVML(data []byte) map[string]LegacyNote {
	var drawing vmlDrawing
	if err := xml.Unmarshal(data, &drawing); err != nil {
		return nil
	}
	out := make(map[string]LegacyNote)
	for _, shape := range drawing.Shapes {
		if shape.ClientData == nil || shape.ClientData.ObjectType != "Note" {
			continue
		}
		row := shape.ClientData.Row + 1
		col := shape.ClientData.Column + 1
		if row <= 0 || col <= 0 {
			continue
		}
		ref := colLettersFromIndex(col) + strconv.Itoa(row)
		out[ref] = LegacyNote{
			Ref:    ref,
			Row:    row,
			Col:    col,
			Anchor: strings.TrimSpace(shape.ClientData.Anchor),
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func legacyNotesFromComments(comments *Comments) map[string]LegacyNote {
	if comments == nil || len(comments.CommentList) == 0 {
		return nil
	}
	out := make(map[string]LegacyNote)
	for _, comment := range comments.CommentList {
		ref := strings.TrimSpace(comment.Ref)
		if ref == "" {
			continue
		}
		out[ref] = LegacyNote{Ref: ref}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// loadWorksheetFormControls parses VML-based form controls from the legacy drawing part.
func loadWorksheetFormControls(pkg *opc.Package, wsPartName string, entry *WorksheetEntry) {
	if pkg == nil || entry == nil || entry.Sheet == nil || entry.Sheet.LegacyDrawing == nil {
		return
	}
	wsRels, ok, err := pkg.Relationships(wsPartName)
	if err != nil || !ok {
		return
	}
	relsByType := indexRelsByType(wsRels)
	for _, r := range relsByType[relTypeVMLDrawing] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(wsPartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		controls := parseFormControlsVML(p.Data)
		if len(controls) == 0 {
			continue
		}
		entry.FormControls = controls
		break
	}
}

func parseFormControlsVML(data []byte) []FormControl {
	var drawing vmlDrawing
	if err := xml.Unmarshal(data, &drawing); err != nil {
		return nil
	}
	var out []FormControl
	for _, shape := range drawing.Shapes {
		if shape.ClientData == nil {
			continue
		}
		cd := shape.ClientData
		switch cd.ObjectType {
		case "Button", "Checkbox", "Radio", "List", "Drop", "Spin", "Scroll", "GBox", "Label", "Edit":
		default:
			continue
		}
		out = append(out, FormControl{
			Type:          cd.ObjectType,
			AltText:       shape.AltText,
			Anchor:        strings.TrimSpace(cd.Anchor),
			Checked:       parseFormControlCheckedState(cd.Checked) == "checked",
			Mixed:         parseFormControlCheckedState(cd.Checked) == "mixed",
			FirstButton:   parseFormControlBool(cd.FirstButton),
			FmlaLink:      strings.TrimSpace(cd.FmlaLink),
			FmlaGroup:     strings.TrimSpace(cd.FmlaGroup),
			FmlaRange:     strings.TrimSpace(cd.FmlaRange),
			SelectionType: normalizeFormControlSelectionType(cd.SelType),
			Selected:      normalizeFormControlSelectedEntries(cd.Sel),
			Min:           cd.Min,
			Max:           cd.Max,
			Val:           cd.Val,
			Increment:     cd.Inc,
			PageStep:      cd.Page,
			Horizontal:    parseFormControlBool(cd.Horiz),
			DropLines:     cd.DropLines,
		})
	}
	return out
}

func parseFormControlCheckedState(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "1", "true", "checked":
		return "checked"
	case "mixed", "2":
		return "mixed"
	default:
		return "unchecked"
	}
}

func parseFormControlBool(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "1", "true", "t", "yes":
		return true
	default:
		return false
	}
}

func normalizeFormControlSelectionType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "single":
		return "single"
	case "multi":
		return "multi"
	case "extend", "extended":
		return "extended"
	default:
		return ""
	}
}

func normalizeFormControlSelectedEntries(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	out := make([]int, 0, len(values))
	seen := map[int]bool{}
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// loadWorksheetLegacyHFImages loads header/footer VML picture shapes referenced by
// legacyDrawingHF, making them available for rendering &G image placeholders.
func loadWorksheetLegacyHFImages(pkg *opc.Package, wsPartName string, entry *WorksheetEntry) {
	if pkg == nil || entry == nil || entry.Sheet == nil || entry.Sheet.LegacyDrawingHF == nil {
		return
	}
	relID := normalizeRelID(entry.Sheet.LegacyDrawingHF.RelID)
	if relID == "" {
		return
	}
	wsRels, ok, err := pkg.Relationships(wsPartName)
	if err != nil || !ok {
		return
	}
	wsRelsByID := indexRelsByID(wsRels)
	hfRel, ok := wsRelsByID[relID]
	if !ok || hfRel.Type != relTypeVMLDrawing || hfRel.IsExternal() {
		return
	}
	vmlPartName := rels.ResolveTarget(wsPartName, hfRel.Target)
	p, ok := pkg.Part(vmlPartName)
	if !ok {
		return
	}
	var drawing vmlDrawing
	if err := xml.Unmarshal(p.Data, &drawing); err != nil {
		return
	}
	// Load the VML part's relationships to resolve image rel IDs.
	vmlRels, hasRels, err := pkg.Relationships(vmlPartName)
	if err != nil || !hasRels {
		return
	}
	vmlRelsByID := indexRelsByID(vmlRels)
	for _, shape := range drawing.Shapes {
		if shape.ClientData == nil || shape.ClientData.ObjectType != "Pict" {
			continue
		}
		if shape.ImageData == nil || shape.ImageData.RelID == "" {
			continue
		}
		imgRel, ok := vmlRelsByID[normalizeRelID(shape.ImageData.RelID)]
		if !ok || imgRel.IsExternal() {
			continue
		}
		imgPartName := rels.ResolveTarget(vmlPartName, imgRel.Target)
		imgPart, ok := pkg.Part(imgPartName)
		if !ok {
			continue
		}
		entry.HFImages = append(entry.HFImages, HFImage{
			ContentType: imgPart.ContentType,
			Data:        imgPart.Data,
		})
	}
}

// loadWorksheetPivotTables parses all pivot table parts referenced by the worksheet.
// Each pivot table's cache definition is also loaded when reachable.
func loadWorksheetPivotTables(pkg *opc.Package, wsPartName string, entry *WorksheetEntry) {
	wsRels, ok, err := pkg.Relationships(wsPartName)
	if err != nil || !ok {
		return
	}
	relsByType := indexRelsByType(wsRels)
	for _, r := range relsByType[relTypePivotTable] {
		if r.IsExternal() {
			continue
		}
		ptPartName := rels.ResolveTarget(wsPartName, r.Target)
		p, ok := pkg.Part(ptPartName)
		if !ok {
			continue
		}
		var pt PivotTableDefinition
		if err := xml.Unmarshal(p.Data, &pt); err != nil {
			continue
		}
		pt.Raw = p.Data
		ptc := &PivotTableWithCache{Table: &pt}
		// Load pivot cache definition via the pivot table part's relationships.
		ptRels, ok, err := pkg.Relationships(ptPartName)
		if err == nil && ok {
			for _, cr := range ptRels.Relationships {
				if cr.Type == relTypePivotCache && !cr.IsExternal() {
					cachePartName := rels.ResolveTarget(ptPartName, cr.Target)
					cp, ok := pkg.Part(cachePartName)
					if !ok {
						continue
					}
					var cache PivotCacheDefinition
					if err := xml.Unmarshal(cp.Data, &cache); err == nil {
						cache.Raw = cp.Data
						ptc.Cache = &cache

						// Load pivot cache records via the cache definition part's relationships.
						cacheRels, ok, err := pkg.Relationships(cachePartName)
						if err == nil && ok {
							for _, rr := range cacheRels.Relationships {
								if rr.Type == relTypePivotCacheRecords && !rr.IsExternal() {
									recordsPartName := rels.ResolveTarget(cachePartName, rr.Target)
									rp, ok := pkg.Part(recordsPartName)
									if ok {
										var records PivotCacheRecords
										if err := xml.Unmarshal(rp.Data, &records); err == nil {
											records.Raw = rp.Data
											ptc.Records = &records
										} else {
											ptc.Records = &PivotCacheRecords{Raw: rp.Data}
										}
									}
									break
								}
							}
						}
					}
					break
				}
			}
		}
		entry.PivotTables = append(entry.PivotTables, ptc)
	}
}

// loadWorksheetTables parses all table definition parts referenced by the worksheet.
// Each table part (xl/tables/tableN.xml) is unmarshalled into a Table struct.
func loadWorksheetTables(pkg *opc.Package, wsPartName string, entry *WorksheetEntry) {
	wsRels, ok, err := pkg.Relationships(wsPartName)
	if err != nil || !ok {
		return
	}
	relsByType := indexRelsByType(wsRels)
	for _, r := range relsByType[relTypeTable] {
		if r.IsExternal() {
			continue
		}
		tablePartName := rels.ResolveTarget(wsPartName, r.Target)
		p, ok := pkg.Part(tablePartName)
		if !ok {
			continue
		}
		var tbl Table
		if err := xml.Unmarshal(p.Data, &tbl); err != nil {
			continue
		}
		entry.Tables = append(entry.Tables, &tbl)
	}
}

func colLettersFromIndex(col int) string {
	if col <= 0 {
		return ""
	}
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
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
		}, byID)
		if embedded != nil {
			out.EmbeddedFonts = append(out.EmbeddedFonts, embedded)
		}
	}
}

func refID(ref *FontEmbedRef) string {
	if ref == nil {
		return ""
	}
	return ref.RelID
}

func loadMediaParts(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) {
	for _, r := range byType[relTypeImage] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		out.MediaParts[normalizeRelID(r.ID)] = media.Part{
			Name:        partName,
			ContentType: p.ContentType,
			Data:        p.Data,
		}
	}
}

func loadChartParts(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) {
	for _, r := range byType[relTypeChart] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}
		out.ChartParts[normalizeRelID(r.ID)] = p.Data
	}
}

func indexRelsByID(r *rels.Document) map[string]rels.Relationship {
	m := make(map[string]rels.Relationship, len(r.Relationships))
	for _, rel := range r.Relationships {
		m[normalizeRelID(rel.ID)] = rel
	}
	return m
}

func indexRelsByType(r *rels.Document) map[string][]rels.Relationship {
	m := make(map[string][]rels.Relationship)
	for _, rel := range r.Relationships {
		m[rel.Type] = append(m[rel.Type], rel)
	}
	return m
}

// normalizeRelID strips the "rId" prefix and returns the canonical form.
// The XML attribute is typically "rId1", "rId2", etc., but some files omit the prefix.
func normalizeRelID(id string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "rId") {
		if n, err := strconv.Atoi(id[3:]); err == nil {
			return "rId" + strconv.Itoa(n)
		}
	}
	return id
}
