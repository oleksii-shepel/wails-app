package parts

import (
	"encoding/xml"

	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/media"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
)

const (
	relTypeDiagramData       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData"
	relTypeDiagramLayout     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramLayout"
	relTypeDiagramStyle      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramStyle"
	relTypeDiagramColors     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramColors"
	relTypeDiagramQuickStyle = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramQuickStyle"
	relTypeDiagramDrawing    = "http://schemas.microsoft.com/office/2007/relationships/diagramDrawing"
	relTypeChart             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart"

	contentTypeDiagramData       = "application/vnd.openxmlformats-officedocument.drawingml.diagramData+xml"
	contentTypeDiagramLayout     = "application/vnd.openxmlformats-officedocument.drawingml.diagramLayout+xml"
	contentTypeDiagramStyle      = "application/vnd.openxmlformats-officedocument.drawingml.diagramStyle+xml"
	contentTypeDiagramColors     = "application/vnd.openxmlformats-officedocument.drawingml.diagramColors+xml"
	contentTypeDiagramQuickStyle = "application/vnd.openxmlformats-officedocument.drawingml.diagramQuickStyle+xml"
	contentTypeDiagramDrawing    = "application/vnd.ms-office.drawingml.diagramDrawing+xml"
)

// SmartArtPart contains the raw XML data for a SmartArt diagram and its related parts.
type SmartArtPart struct {
	// RelID is the relationship ID from the slide.
	RelID string
	// PartName is the resolved OPC part name for the data model.
	PartName string
	// DataModelXML is the raw dgm:dataModel XML content.
	DataModelXML []byte
	// LayoutXML is the raw dgm:layoutDef XML content (optional).
	LayoutXML []byte
	// StyleXML is the raw dgm:styleDef XML content (optional).
	StyleXML []byte
	// ColorsXML is the raw dgm:colorsDef XML content (optional).
	ColorsXML []byte
	// QuickStyleXML is the raw dgm:quickStyle XML content (optional).
	QuickStyleXML []byte
	// DrawingXML is the pre-rendered dsp:drawing XML content (optional).
	// PowerPoint emits exact shape positions and geometry here; when present it
	// can be used for high-fidelity SmartArt rendering.
	DrawingXML []byte
	// MediaParts maps relationship IDs to embedded image/audio/video parts that
	// are referenced from the SmartArt data model or pre-rendered drawing part.
	MediaParts map[string]media.Part
}

// PresentationTable represents an a:tbl inside a p:graphicFrame.
type PresentationTable struct {
	Props *PresentationTableProps `xml:"tblPr"`
	Grid  PresentationTableGrid   `xml:"tblGrid"`
	Rows  []PresentationTableRow  `xml:"tr"`
}

// PresentationTableProps stores whole-table properties and style toggles.
type PresentationTableProps struct {
	FirstRow     bool               `xml:"firstRow,attr,omitempty"`
	FirstCol     bool               `xml:"firstCol,attr,omitempty"`
	LastRow      bool               `xml:"lastRow,attr,omitempty"`
	LastCol      bool               `xml:"lastCol,attr,omitempty"`
	BandRow      bool               `xml:"bandRow,attr,omitempty"`
	BandCol      bool               `xml:"bandCol,attr,omitempty"`
	RTL          bool               `xml:"rtl,attr,omitempty"`
	TableStyleID string             `xml:"tableStyleId"`
	TableStyle   *DrawingTableStyle `xml:"tableStyle"`
}

// PresentationTableGrid defines table column widths.
type PresentationTableGrid struct {
	Columns []PresentationTableColumn `xml:"gridCol"`
}

// PresentationTableColumn defines a single table grid column.
type PresentationTableColumn struct {
	W int64 `xml:"w,attr,omitempty"`
}

// PresentationTableRow is a table row.
type PresentationTableRow struct {
	H     int64                   `xml:"h,attr,omitempty"`
	Cells []PresentationTableCell `xml:"tc"`
}

// PresentationTableCell is a table cell.
type PresentationTableCell struct {
	GridSpan int                         `xml:"gridSpan,attr,omitempty"`
	RowSpan  int                         `xml:"rowSpan,attr,omitempty"`
	HMerge   bool                        `xml:"hMerge,attr,omitempty"`
	VMerge   bool                        `xml:"vMerge,attr,omitempty"`
	TextBody *TextBody                   `xml:"txBody"`
	Props    *PresentationTableCellProps `xml:"tcPr"`
}

// PresentationTableCellProps stores cell geometry and appearance.
type PresentationTableCellProps struct {
	MarL       int64                 `xml:"marL,attr,omitempty"`
	MarR       int64                 `xml:"marR,attr,omitempty"`
	MarT       int64                 `xml:"marT,attr,omitempty"`
	MarB       int64                 `xml:"marB,attr,omitempty"`
	Anchor     string                `xml:"anchor,attr,omitempty"`
	SolidFill  *ColorRef             `xml:"solidFill>srgbClr"`
	SchemeFill *drawingxml.SchemeClr `xml:"solidFill>schemeClr"`
	GradFill   *GradFill             `xml:"gradFill"`
	PattFill   *PattFill             `xml:"pattFill"`
	BlipFill   *BlipFill             `xml:"blipFill"`
	NoFill     *struct{}             `xml:"noFill"`
	LnL        *ConnectorLine        `xml:"lnL"`
	LnR        *ConnectorLine        `xml:"lnR"`
	LnT        *ConnectorLine        `xml:"lnT"`
	LnB        *ConnectorLine        `xml:"lnB"`
}

// TableStyleList is the root of the DrawingML table styles part.
type TableStyleList struct {
	XMLName xml.Name            `xml:"tblStyleLst"`
	Default string              `xml:"def,attr,omitempty"`
	Styles  []DrawingTableStyle `xml:"tblStyle"`
}

// DrawingTableStyle is a reusable DrawingML table style.
type DrawingTableStyle struct {
	StyleID   string            `xml:"styleId,attr,omitempty"`
	StyleName string            `xml:"styleName,attr,omitempty"`
	WholeTbl  *TableStyleRegion `xml:"wholeTbl"`
	Band1H    *TableStyleRegion `xml:"band1H"`
	Band2H    *TableStyleRegion `xml:"band2H"`
	Band1V    *TableStyleRegion `xml:"band1V"`
	Band2V    *TableStyleRegion `xml:"band2V"`
	FirstCol  *TableStyleRegion `xml:"firstCol"`
	FirstRow  *TableStyleRegion `xml:"firstRow"`
	LastCol   *TableStyleRegion `xml:"lastCol"`
	LastRow   *TableStyleRegion `xml:"lastRow"`
	NECell    *TableStyleRegion `xml:"neCell"`
	NWCell    *TableStyleRegion `xml:"nwCell"`
	SECell    *TableStyleRegion `xml:"seCell"`
	SWCell    *TableStyleRegion `xml:"swCell"`
}

// TableStyleRegion stores conditional cell and text styling.
type TableStyleRegion struct {
	CellStyle *DrawingTableCellStyle     `xml:"tcStyle"`
	TextStyle *DrawingTableCellTextStyle `xml:"tcTxStyle"`
}

// DrawingTableCellStyle stores fill and border defaults for a conditional region.
type DrawingTableCellStyle struct {
	FillColor  *ColorRef                `xml:"fill>solidFill>srgbClr"`
	FillScheme *drawingxml.SchemeClr    `xml:"fill>solidFill>schemeClr"`
	GradFill   *GradFill                `xml:"fill>gradFill"`
	PattFill   *PattFill                `xml:"fill>pattFill"`
	BlipFill   *BlipFill                `xml:"fill>blipFill"`
	NoFill     *struct{}                `xml:"fill>noFill"`
	FillRef    *DrawingTableFillRef     `xml:"fillRef"`
	EffectRef  *DrawingTableEffectRef   `xml:"effectRef"`
	Borders    *DrawingTableCellBorders `xml:"tcBdr"`
}

// DrawingTableFillRef references a theme style-matrix fill entry.
type DrawingTableFillRef struct {
	Idx       int                   `xml:"idx,attr,omitempty"`
	Color     *ColorRef             `xml:"srgbClr"`
	SchemeClr *drawingxml.SchemeClr `xml:"schemeClr"`
}

// DrawingTableEffectRef references a theme style-matrix effect entry.
type DrawingTableEffectRef struct {
	Idx       int                   `xml:"idx,attr,omitempty"`
	Color     *ColorRef             `xml:"srgbClr"`
	SchemeClr *drawingxml.SchemeClr `xml:"schemeClr"`
}

// DrawingTableCellBorders stores the supported border definitions for a region.
type DrawingTableCellBorders struct {
	Left    *DrawingTableBorder `xml:"left"`
	Right   *DrawingTableBorder `xml:"right"`
	Top     *DrawingTableBorder `xml:"top"`
	Bottom  *DrawingTableBorder `xml:"bottom"`
	InsideH *DrawingTableBorder `xml:"insideH"`
	InsideV *DrawingTableBorder `xml:"insideV"`
}

// DrawingTableBorder wraps the line style used for a table border edge.
type DrawingTableBorder struct {
	Line    *ConnectorLine       `xml:"ln"`
	LineRef *DrawingTableLineRef `xml:"lnRef"`
}

// DrawingTableLineRef references a theme style-matrix line entry.
type DrawingTableLineRef struct {
	Idx       int                   `xml:"idx,attr,omitempty"`
	Color     *ColorRef             `xml:"srgbClr"`
	SchemeClr *drawingxml.SchemeClr `xml:"schemeClr"`
}

// DrawingTableCellTextStyle stores conditional text styling for a region.
type DrawingTableCellTextStyle struct {
	Bold      string                `xml:"b,attr,omitempty"`
	Italic    string                `xml:"i,attr,omitempty"`
	FontRef   *DrawingTableFontRef  `xml:"fontRef"`
	Color     *ColorRef             `xml:"srgbClr"`
	SchemeClr *drawingxml.SchemeClr `xml:"schemeClr"`
}

// DrawingTableFontRef identifies the major/minor theme font bucket.
type DrawingTableFontRef struct {
	Idx       string                `xml:"idx,attr,omitempty"`
	Color     *ColorRef             `xml:"srgbClr"`
	SchemeClr *drawingxml.SchemeClr `xml:"schemeClr"`
}

// collectSmartArtReferences scans slide relationships for diagram parts.
func collectSmartArtReferences(pkg *opc.Package, slidePartName string) ([]*SmartArtPart, error) {
	slideRels, ok, err := pkg.Relationships(slidePartName)
	if err != nil || !ok {
		return nil, nil
	}

	var smartArtParts []*SmartArtPart

	byID := make(map[string]rels.Relationship)
	for _, r := range slideRels.Relationships {
		byID[normalizeRelID(r.ID)] = r
	}

	for _, r := range slideRels.Relationships {
		// Check if this is a diagram data relationship
		if !relationshipTypeIs(r.Type, relTypeDiagramData) || r.IsExternal() {
			continue
		}

		part := &SmartArtPart{
			RelID:    normalizeRelID(r.ID),
			PartName: rels.ResolveTarget(slidePartName, r.Target),
		}

		// Load the data model XML
		p, ok := pkg.Part(part.PartName)
		if !ok {
			continue
		}
		part.DataModelXML = p.Data

		// Load related parts (layout, style, colors)
		loadSmartArtRelatedParts(pkg, part, slidePartName, byID)

		// Load media parts referenced from the data model part.
		loadSmartArtMediaParts(pkg, part)

		smartArtParts = append(smartArtParts, part)
	}

	// Load optional pre-rendered drawing parts. Association by index is
	// sufficient because PowerPoint emits one drawing part per diagram.
	var drawingRels []rels.Relationship
	for _, r := range slideRels.Relationships {
		if relationshipTypeIs(r.Type, relTypeDiagramDrawing) && !r.IsExternal() {
			drawingRels = append(drawingRels, r)
		}
	}
	for i, part := range smartArtParts {
		if i >= len(drawingRels) {
			break
		}
		drawingPartName := rels.ResolveTarget(slidePartName, drawingRels[i].Target)
		if p, ok := pkg.Part(drawingPartName); ok {
			part.DrawingXML = p.Data
			loadSmartArtDrawingMediaParts(pkg, part, drawingPartName)
		}
	}

	return smartArtParts, nil
}

// loadSmartArtRelatedParts loads layout, style, colors, and quickStyle parts for a SmartArt.
func loadSmartArtRelatedParts(pkg *opc.Package, part *SmartArtPart, basePartName string, byID map[string]rels.Relationship) {
	// Get relationships for the data model part
	dataRels, ok, _ := pkg.Relationships(part.PartName)
	if !ok {
		return
	}

	for _, r := range dataRels.Relationships {
		if r.IsExternal() {
			continue
		}

		targetPartName := rels.ResolveTarget(part.PartName, r.Target)
		p, ok := pkg.Part(targetPartName)
		if !ok {
			continue
		}

		switch canonicalRelationshipType(r.Type) {
		case relTypeDiagramLayout:
			part.LayoutXML = p.Data
		case relTypeDiagramStyle:
			part.StyleXML = p.Data
		case relTypeDiagramColors:
			part.ColorsXML = p.Data
		case relTypeDiagramQuickStyle:
			part.QuickStyleXML = p.Data
		}
	}
}

// loadSmartArtMediaParts loads embedded image/audio/video parts referenced from
// the SmartArt data model part's relationships.
func loadSmartArtMediaParts(pkg *opc.Package, part *SmartArtPart) {
	if part.MediaParts == nil {
		part.MediaParts = make(map[string]media.Part)
	}
	dataRels, ok, _ := pkg.Relationships(part.PartName)
	if !ok {
		return
	}
	for _, r := range dataRels.Relationships {
		loadRelationshipMediaPart(pkg, part.PartName, r, part.MediaParts)
	}
}

// loadSmartArtDrawingMediaParts loads embedded image/audio/video parts referenced
// from the pre-rendered diagram drawing part's relationships.
func loadSmartArtDrawingMediaParts(pkg *opc.Package, part *SmartArtPart, drawingPartName string) {
	if part.MediaParts == nil {
		part.MediaParts = make(map[string]media.Part)
	}
	drawingRels, ok, _ := pkg.Relationships(drawingPartName)
	if !ok {
		return
	}
	for _, r := range drawingRels.Relationships {
		loadRelationshipMediaPart(pkg, drawingPartName, r, part.MediaParts)
	}
}

// loadSmartArtRelatedPartsFromSlideRels uses the slide-level relationships
// referenced by the hosting p:graphicFrame's dgm:relIds attributes to load the
// diagram layout, colors, and quick-style parts. PowerPoint commonly stores
// these relationships on the slide rather than on the data model part.
func loadSmartArtRelatedPartsFromSlideRels(pkg *opc.Package, slidePartName string, entry *SlideEntry) {
	if entry == nil || entry.Slide == nil || len(entry.SmartArtParts) == 0 {
		return
	}

	slideRels, ok, _ := pkg.Relationships(slidePartName)
	if !ok {
		return
	}
	byID := make(map[string]rels.Relationship, len(slideRels.Relationships))
	for _, r := range slideRels.Relationships {
		byID[normalizeRelID(r.ID)] = r
	}

	partByDataRel := make(map[string]*SmartArtPart, len(entry.SmartArtParts))
	for _, sap := range entry.SmartArtParts {
		partByDataRel[normalizeRelID(sap.RelID)] = sap
	}

	var visitFrames func(frames []SmartArtFrame)
	visitFrames = func(frames []SmartArtFrame) {
		for _, frame := range frames {
			part, ok := partByDataRel[frame.RelIDs["dm"]]
			if !ok {
				continue
			}
			for key, relID := range frame.RelIDs {
				if key == "dm" {
					continue
				}
				r, ok := byID[relID]
				if !ok {
					continue
				}
				targetPartName := rels.ResolveTarget(slidePartName, r.Target)
				p, ok := pkg.Part(targetPartName)
				if !ok {
					continue
				}
				switch canonicalRelationshipType(r.Type) {
				case relTypeDiagramLayout:
					part.LayoutXML = p.Data
				case relTypeDiagramStyle:
					part.StyleXML = p.Data
				case relTypeDiagramColors:
					part.ColorsXML = p.Data
				case relTypeDiagramQuickStyle:
					part.QuickStyleXML = p.Data
				}
			}
		}
	}

	var visitGroup func(g *GroupShape)
	visitGroup = func(g *GroupShape) {
		if g == nil {
			return
		}
		visitFrames(g.SmartArtFrames)
		for i := range g.GroupShapes {
			visitGroup(&g.GroupShapes[i])
		}
	}

	visitFrames(entry.Slide.CSld.SpTree.SmartArtFrames)
	for i := range entry.Slide.CSld.SpTree.GroupShapes {
		visitGroup(&entry.Slide.CSld.SpTree.GroupShapes[i])
	}
}

// buildChartFrameMap indexes the hosting p:graphicFrame transform for each
// chart relationship on the slide. Charts nested inside groups are not yet
// captured.
func buildChartFrameMap(slide *Slide) map[string]*Transform2D {
	m := make(map[string]*Transform2D)
	if slide == nil {
		return m
	}
	for i := range slide.CSld.SpTree.GraphicFrames {
		frame := &slide.CSld.SpTree.GraphicFrames[i]
		if frame.Graphic.GraphicData.Chart == nil {
			continue
		}
		relID := normalizeRelID(frame.Graphic.GraphicData.Chart.RelID)
		if relID != "" {
			m[relID] = frame.Xfrm
		}
	}
	return m
}

// loadSlideChartParts loads chart parts referenced by a slide.
func loadSlideChartParts(pkg *opc.Package, slidePartName string) map[string][]byte {
	chartParts := make(map[string][]byte)
	slideRels, ok, err := pkg.Relationships(slidePartName)
	if err != nil || !ok {
		return chartParts
	}

	for _, r := range slideRels.Relationships {
		if !relationshipTypeIs(r.Type, relTypeChart) || r.IsExternal() {
			continue
		}

		partName := rels.ResolveTarget(slidePartName, r.Target)
		p, ok := pkg.Part(partName)
		if !ok {
			continue
		}

		chartParts[normalizeRelID(r.ID)] = p.Data
	}

	return chartParts
}
