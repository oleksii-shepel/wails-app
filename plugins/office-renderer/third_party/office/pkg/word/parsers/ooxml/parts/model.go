package parts

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"office/pkg/diag"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/opc"
	rels2 "office/pkg/ooxml/rels"
)

const (
	relationshipTypeOfficeDocument    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	relationshipTypeStyles            = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
	relationshipTypeNumbering         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering"
	relationshipTypeHeader            = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header"
	relationshipTypeFooter            = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer"
	relationshipTypeImage             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
	relationshipTypeChart             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart"
	relationshipTypeDiagramData       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData"
	relationshipTypeDiagramLayout     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramLayout"
	relationshipTypeDiagramQuickStyle = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramQuickStyle"
	relationshipTypeDiagramColor      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramColors"
	relationshipTypeDiagramDrawing    = "http://schemas.microsoft.com/office/2007/relationships/diagramDrawing"
	relationshipTypeCoreProperties    = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties"
	relationshipTypeTheme             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme"
	relationshipTypeSettings          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings"
	relationshipTypeGlossary          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument"
	relationshipTypeFootnotes         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes"
	relationshipTypeEndnotes          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes"
	relationshipTypeComments          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments"
	relationshipTypeFont              = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/font"
	relationshipTypeExtendedProps     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties"
	relationshipTypeCustomProps       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/custom-properties"
	relationshipTypeCustomXMLProps    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/customXmlProps"
	relationshipTypeAltChunk          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/afChunk"
	relationshipTypeOLEObject         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject"
	relationshipTypePackage           = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/package"

	drawingGraphicDataURIChartTransitional   = "http://schemas.openxmlformats.org/drawingml/2006/chart"
	drawingGraphicDataURIChartStrict         = "http://purl.oclc.org/ooxml/drawingml/chart"
	drawingGraphicDataURIDiagramTransitional = "http://schemas.openxmlformats.org/drawingml/2006/diagram"
	drawingGraphicDataURIDiagramStrict       = "http://purl.oclc.org/ooxml/drawingml/diagram"
)

// Package contains the typed WordprocessingML parts resolved from an OOXML package.
type Package struct {
	MainPartName string
	Document     *Document
	// SourcePath is the original file-system path from which this package was
	// loaded.  It is used for resolving external relationships (e.g. attached
	// templates) relative to the document location.
	SourcePath        string
	PartRels          map[string]*Relationships
	Styles            *Styles
	StylesPartName    string
	Numbering         *Numbering
	NumberingPartName string
	Theme             *Theme
	ThemePartName     string
	FontTable         *FontTable
	FontTablePartName string
	Glossary          *GlossaryDocument
	GlossaryPartName  string
	Settings          *Settings
	SettingsPartName  string
	Footnotes         *FootnotesPart
	FootnotesPartName string
	Endnotes          *EndnotesPart
	EndnotesPartName  string
	Comments          *CommentsPart
	CommentsPartName  string
	Headers           map[string]*Header
	Footers           map[string]*Footer
	ResolvedHeaders   []ResolvedReference
	ResolvedFooters   []ResolvedReference
	// ScopedHyperlinkURLs maps (source part, relationship ID) to external target URLs.
	ScopedHyperlinkURLs map[RelationshipRef]string
	CoreProperties      *CoreProperties
	ExtendedProperties  *ExtendedProperties
	CustomProperties    *CustomProperties
	// CustomXMLParts maps normalized w:dataBinding storeItemID values to raw custom XML payloads.
	CustomXMLParts map[string][]byte
	// HasSignatures reports whether the source OPC package contains a digital
	// signature origin part.
	HasSignatures bool
	// Signatures preserves the discovered OPC digital signature parts. Entries
	// with a nil Signature failed XML-DSig parsing and are accompanied by a
	// package diagnostic.
	Signatures []opc.SignaturePart
	// SignatureVerifications stores cryptographic verification results for the
	// discovered signature parts.
	SignatureVerifications []opc.SignatureVerification
	EmbeddedFonts          []*EmbeddedFontData
	Diagnostics            diag.List
	// ScopedMediaParts maps (source part, relationship ID) to loaded media parts.
	ScopedMediaParts map[RelationshipRef]MediaPart
	// ScopedChartParts maps (source part, relationship ID) to chart XML data.
	ScopedChartParts map[RelationshipRef][]byte
	// ScopedSmartArtParts maps (source part, relationship ID) to SmartArt data model XML.
	ScopedSmartArtParts map[RelationshipRef][]byte
	// ScopedSmartArtLayoutParts maps (source part, relationship ID) to SmartArt layout definition XML.
	ScopedSmartArtLayoutParts map[RelationshipRef][]byte
	// ScopedSmartArtStyleParts maps (source part, relationship ID) to SmartArt quick style XML.
	ScopedSmartArtStyleParts map[RelationshipRef][]byte
	// ScopedSmartArtColorParts maps (source part, relationship ID) to SmartArt color style XML.
	ScopedSmartArtColorParts map[RelationshipRef][]byte
	// ScopedSmartArtDrawingParts maps (source part, relationship ID) to pre-rendered DiagramML drawing XML.
	ScopedSmartArtDrawingParts map[RelationshipRef][]byte
	// ScopedAltChunkParts maps (source part, relationship ID) to raw altChunk data.
	ScopedAltChunkParts map[RelationshipRef]AltChunkPart
	// ScopedOLEParts maps embedded OLE/package relationships to their raw payloads.
	ScopedOLEParts map[RelationshipRef]OLEPart
	// ScopedOLELinks maps externally linked OLE relationships to their targets.
	ScopedOLELinks map[RelationshipRef]string
	// RootRelationships preserves package-level relationships that are not
	// owned by the typed Word writer. Typed officeDocument/property
	// relationships are regenerated on save and override matching entries.
	RootRelationships *Relationships
	// OpaqueParts preserves OPC parts that are not interpreted by the typed
	// Word model. This is the lossless round-trip escape hatch for vendor
	// extensions, custom DrawingML/VML companions, and future OOXML parts.
	OpaqueParts map[string]OpaquePart
}

// OpaquePart is an uninterpreted OPC part retained for round-trip fidelity.
type OpaquePart struct {
	ContentType   string
	Data          []byte
	Relationships *Relationships
}

// RawElement retains unsupported XML children without assigning semantics.
// It is intentionally generic so newer/vendor markup can survive a load/save
// cycle while typed siblings remain editable.
type RawElement struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	InnerXML string     `xml:",innerxml"`
}

// AltChunkPart holds the raw data and content type for an altChunk relationship.
type AltChunkPart struct {
	ContentType string
	Data        []byte
}

// OLEPart holds an embedded OLE/package payload.
type OLEPart struct {
	ContentType string
	Data        []byte
	// RelationshipType preserves whether the payload used the oleObject or package relationship type.
	RelationshipType string
}

// RelationshipRef identifies a relationship within the scope of a specific source part.
type RelationshipRef struct {
	SourcePart string
	ID         string
}

const defaultMainPartName = "word/document.xml"

func relRef(sourcePart, relationshipID string) RelationshipRef {
	return RelationshipRef{SourcePart: sourcePart, ID: relationshipID}
}

func (pkg *Package) mainPartName() string {
	if pkg == nil || pkg.MainPartName == "" {
		return defaultMainPartName
	}
	return pkg.MainPartName
}

// EnsurePartRelationships returns the relationship set for a source part, creating it if needed.
func (pkg *Package) EnsurePartRelationships(sourcePart string) *Relationships {
	if pkg == nil {
		return nil
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	if pkg.PartRels == nil {
		pkg.PartRels = make(map[string]*Relationships)
	}
	if rels := pkg.PartRels[sourcePart]; rels != nil {
		return rels
	}
	rels := &Relationships{}
	pkg.PartRels[sourcePart] = rels
	return rels
}

// UpsertRelationship records or replaces a relationship for a specific source part.
func (pkg *Package) UpsertRelationship(sourcePart string, relationship *Relationship) {
	if pkg == nil || relationship == nil || relationship.ID == "" {
		return
	}
	rels := pkg.EnsurePartRelationships(sourcePart)
	for i, existing := range rels.Relationships {
		if existing != nil && existing.ID == relationship.ID {
			rels.Relationships[i] = relationship
			return
		}
	}
	rels.Relationships = append(rels.Relationships, relationship)
}

// SetHyperlinkTarget records an external hyperlink relationship for a source part.
func (pkg *Package) SetHyperlinkTarget(sourcePart, relationshipID, target string) {
	if pkg == nil || relationshipID == "" || target == "" {
		return
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	if pkg.ScopedHyperlinkURLs == nil {
		pkg.ScopedHyperlinkURLs = make(map[RelationshipRef]string)
	}
	pkg.ScopedHyperlinkURLs[relRef(sourcePart, relationshipID)] = target
	pkg.UpsertRelationship(sourcePart, &Relationship{
		ID:         relationshipID,
		Type:       relTypeHyperlink,
		Target:     target,
		TargetMode: "External",
	})
}

// SetMediaPart records an image/media relationship for a source part.
func (pkg *Package) SetMediaPart(sourcePart, relationshipID string, media MediaPart, target string) {
	if pkg == nil || relationshipID == "" {
		return
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	if pkg.ScopedMediaParts == nil {
		pkg.ScopedMediaParts = make(map[RelationshipRef]MediaPart)
	}
	pkg.ScopedMediaParts[relRef(sourcePart, relationshipID)] = media
	if target != "" {
		pkg.UpsertRelationship(sourcePart, &Relationship{
			ID:     relationshipID,
			Type:   relTypeImage,
			Target: target,
		})
	}
}

// SetOLEPart records an embedded OLE/package relationship for a source part.
func (pkg *Package) SetOLEPart(sourcePart, relationshipID string, part OLEPart, target string) {
	if pkg == nil || relationshipID == "" {
		return
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	if pkg.ScopedOLEParts == nil {
		pkg.ScopedOLEParts = make(map[RelationshipRef]OLEPart)
	}
	if part.RelationshipType == "" {
		part.RelationshipType = relationshipTypeOLEObject
	}
	part.Data = append([]byte(nil), part.Data...)
	pkg.ScopedOLEParts[relRef(sourcePart, relationshipID)] = part
	if target != "" {
		pkg.UpsertRelationship(sourcePart, &Relationship{ID: relationshipID, Type: part.RelationshipType, Target: target})
	}
}

// SetOLELink records an externally linked OLE relationship.
func (pkg *Package) SetOLELink(sourcePart, relationshipID, target string) {
	if pkg == nil || relationshipID == "" || target == "" {
		return
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	if pkg.ScopedOLELinks == nil {
		pkg.ScopedOLELinks = make(map[RelationshipRef]string)
	}
	pkg.ScopedOLELinks[relRef(sourcePart, relationshipID)] = target
	pkg.UpsertRelationship(sourcePart, &Relationship{ID: relationshipID, Type: relationshipTypeOLEObject, Target: target, TargetMode: "External"})
}

// OLEPartFor resolves an embedded OLE payload in relationship scope.
func (pkg *Package) OLEPartFor(sourcePart, relationshipID string) (OLEPart, bool) {
	if pkg == nil {
		return OLEPart{}, false
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	p, ok := pkg.ScopedOLEParts[relRef(sourcePart, relationshipID)]
	if !ok {
		return OLEPart{}, false
	}
	p.Data = append([]byte(nil), p.Data...)
	return p, true
}

// OLELinkFor resolves an external OLE target in relationship scope.
func (pkg *Package) OLELinkFor(sourcePart, relationshipID string) (string, bool) {
	if pkg == nil {
		return "", false
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	t, ok := pkg.ScopedOLELinks[relRef(sourcePart, relationshipID)]
	return t, ok
}

// OLEPartsFor returns embedded OLE payloads for a source part.
func (pkg *Package) OLEPartsFor(sourcePart string) map[string]OLEPart {
	out := make(map[string]OLEPart)
	if pkg == nil {
		return out
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	for ref, part := range pkg.ScopedOLEParts {
		if ref.SourcePart == sourcePart {
			part.Data = append([]byte(nil), part.Data...)
			out[ref.ID] = part
		}
	}
	return out
}

// SetChartPart records a chart relationship for a source part.
func (pkg *Package) SetChartPart(sourcePart, relationshipID string, data []byte, target string) {
	if pkg == nil || relationshipID == "" {
		return
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	if pkg.ScopedChartParts == nil {
		pkg.ScopedChartParts = make(map[RelationshipRef][]byte)
	}
	copied := append([]byte(nil), data...)
	pkg.ScopedChartParts[relRef(sourcePart, relationshipID)] = copied
	if target != "" {
		pkg.UpsertRelationship(sourcePart, &Relationship{
			ID:     relationshipID,
			Type:   relTypeChart,
			Target: target,
		})
	}
}

// SetSmartArtPart records a SmartArt data relationship for a source part.
func (pkg *Package) SetSmartArtPart(sourcePart, relationshipID string, data []byte, target string) {
	if pkg == nil || relationshipID == "" {
		return
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	if pkg.ScopedSmartArtParts == nil {
		pkg.ScopedSmartArtParts = make(map[RelationshipRef][]byte)
	}
	copied := append([]byte(nil), data...)
	pkg.ScopedSmartArtParts[relRef(sourcePart, relationshipID)] = copied
	if target != "" {
		pkg.UpsertRelationship(sourcePart, &Relationship{
			ID:     relationshipID,
			Type:   relTypeDiagramData,
			Target: target,
		})
	}
}

// HyperlinkTargetsFor returns all external hyperlink targets for a source part.
func (pkg *Package) HyperlinkTargetsFor(sourcePart string) map[string]string {
	out := make(map[string]string)
	if pkg == nil {
		return out
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	for ref, target := range pkg.ScopedHyperlinkURLs {
		if ref.SourcePart == sourcePart {
			out[ref.ID] = target
		}
	}
	return out
}

// MediaPartsFor returns all media relationships for a source part.
func (pkg *Package) MediaPartsFor(sourcePart string) map[string]MediaPart {
	out := make(map[string]MediaPart)
	if pkg == nil {
		return out
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	for ref, media := range pkg.ScopedMediaParts {
		if ref.SourcePart == sourcePart {
			out[ref.ID] = media
		}
	}
	return out
}

// ChartPartsFor returns all chart relationships for a source part.
func (pkg *Package) ChartPartsFor(sourcePart string) map[string][]byte {
	out := make(map[string][]byte)
	if pkg == nil {
		return out
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	for ref, data := range pkg.ScopedChartParts {
		if ref.SourcePart == sourcePart {
			out[ref.ID] = append([]byte(nil), data...)
		}
	}
	return out
}

// SmartArtPartsFor returns all SmartArt data relationships for a source part.
func (pkg *Package) SmartArtPartsFor(sourcePart string) map[string][]byte {
	out := make(map[string][]byte)
	if pkg == nil {
		return out
	}
	if sourcePart == "" {
		sourcePart = pkg.mainPartName()
	}
	for ref, data := range pkg.ScopedSmartArtParts {
		if ref.SourcePart == sourcePart {
			out[ref.ID] = append([]byte(nil), data...)
		}
	}
	return out
}

// RelationshipTarget resolves a relationship target relative to a specific source part.
func (pkg *Package) RelationshipTarget(sourcePart, relationshipID string) (string, bool) {
	if pkg == nil || relationshipID == "" {
		return "", false
	}
	if rels := pkg.PartRels[sourcePart]; rels != nil {
		for _, relationship := range rels.Relationships {
			if relationship == nil || relationship.ID != relationshipID || relationship.TargetMode == "External" {
				continue
			}
			return rels2.ResolveTarget(sourcePart, relationship.Target), true
		}
	}
	return "", false
}

// HyperlinkTarget resolves an external hyperlink relationship relative to a specific source part.
func (pkg *Package) HyperlinkTarget(sourcePart, relationshipID string) string {
	if pkg == nil || relationshipID == "" {
		return ""
	}
	if pkg.ScopedHyperlinkURLs != nil {
		if target, ok := pkg.ScopedHyperlinkURLs[relRef(sourcePart, relationshipID)]; ok {
			return target
		}
	}
	return ""
}

// MediaPartFor resolves an image/media relationship relative to a specific source part.
func (pkg *Package) MediaPartFor(sourcePart, relationshipID string) (MediaPart, bool) {
	if pkg == nil || relationshipID == "" {
		return MediaPart{}, false
	}
	if pkg.ScopedMediaParts != nil {
		if media, ok := pkg.ScopedMediaParts[relRef(sourcePart, relationshipID)]; ok {
			return media, true
		}
	}
	return MediaPart{}, false
}

// ChartPartFor resolves a chart relationship relative to a specific source part.
func (pkg *Package) ChartPartFor(sourcePart, relationshipID string) ([]byte, bool) {
	if pkg == nil || relationshipID == "" {
		return nil, false
	}
	if pkg.ScopedChartParts != nil {
		if chart, ok := pkg.ScopedChartParts[relRef(sourcePart, relationshipID)]; ok {
			return chart, true
		}
	}
	return nil, false
}

// SmartArtPartFor resolves a SmartArt relationship relative to a specific source part.
func (pkg *Package) SmartArtPartFor(sourcePart, relationshipID string) ([]byte, bool) {
	if pkg == nil || relationshipID == "" {
		return nil, false
	}
	if pkg.ScopedSmartArtParts != nil {
		if data, ok := pkg.ScopedSmartArtParts[relRef(sourcePart, relationshipID)]; ok {
			return data, true
		}
	}
	return nil, false
}

// SmartArtLayoutPartFor resolves a SmartArt layout definition relationship.
func (pkg *Package) SmartArtLayoutPartFor(sourcePart, relationshipID string) ([]byte, bool) {
	if pkg == nil || relationshipID == "" {
		return nil, false
	}
	if pkg.ScopedSmartArtLayoutParts != nil {
		if data, ok := pkg.ScopedSmartArtLayoutParts[relRef(sourcePart, relationshipID)]; ok {
			return data, true
		}
	}
	return nil, false
}

// SmartArtStylePartFor resolves a SmartArt quick style relationship.
func (pkg *Package) SmartArtStylePartFor(sourcePart, relationshipID string) ([]byte, bool) {
	if pkg == nil || relationshipID == "" {
		return nil, false
	}
	if pkg.ScopedSmartArtStyleParts != nil {
		if data, ok := pkg.ScopedSmartArtStyleParts[relRef(sourcePart, relationshipID)]; ok {
			return data, true
		}
	}
	return nil, false
}

// SmartArtColorPartFor resolves a SmartArt color style relationship.
func (pkg *Package) SmartArtColorPartFor(sourcePart, relationshipID string) ([]byte, bool) {
	if pkg == nil || relationshipID == "" {
		return nil, false
	}
	if pkg.ScopedSmartArtColorParts != nil {
		if data, ok := pkg.ScopedSmartArtColorParts[relRef(sourcePart, relationshipID)]; ok {
			return data, true
		}
	}
	return nil, false
}

// SmartArtDrawingPartFor resolves a pre-rendered DiagramML drawing relationship.
func (pkg *Package) SmartArtDrawingPartFor(sourcePart, relationshipID string) ([]byte, bool) {
	if pkg == nil || relationshipID == "" {
		return nil, false
	}
	if pkg.ScopedSmartArtDrawingParts != nil {
		if data, ok := pkg.ScopedSmartArtDrawingParts[relRef(sourcePart, relationshipID)]; ok {
			return data, true
		}
	}
	return nil, false
}

// CoreProperties represents docProps/core.xml metadata used by field codes.
// CoreKeywords represents cp:keywords — may contain plain text or <value> children.
type CoreKeywords struct {
	Lang   string             `xml:"http://www.w3.org/XML/1998/namespace lang,attr,omitempty"`
	Text   string             `xml:",chardata"`
	Values []CoreKeywordValue `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties value"`
}

// CoreKeywordValue represents a single cp:value inside cp:keywords.
type CoreKeywordValue struct {
	Lang  string `xml:"http://www.w3.org/XML/1998/namespace lang,attr,omitempty"`
	Value string `xml:",chardata"`
}

type CoreProperties struct {
	XMLName        xml.Name      `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties coreProperties"`
	Title          string        `xml:"http://purl.org/dc/elements/1.1/ title"`
	Subject        string        `xml:"http://purl.org/dc/elements/1.1/ subject"`
	Creator        string        `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Description    string        `xml:"http://purl.org/dc/elements/1.1/ description"`
	Identifier     string        `xml:"http://purl.org/dc/elements/1.1/ identifier"`
	Language       string        `xml:"http://purl.org/dc/elements/1.1/ language"`
	Keywords       *CoreKeywords `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties keywords"`
	LastModifiedBy string        `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties lastModifiedBy"`
	Created        string        `xml:"http://purl.org/dc/terms/ created"`
	Modified       string        `xml:"http://purl.org/dc/terms/ modified"`
	LastPrinted    string        `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties lastPrinted"`
	Category       string        `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties category"`
	ContentStatus  string        `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties contentStatus"`
	Revision       string        `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties revision"`
	Version        string        `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties version"`
}

// ExtendedProperties represents docProps/app.xml metadata.
type ExtendedProperties struct {
	XMLName     xml.Name `xml:"Properties"`
	Application string   `xml:"Application"`
	AppVersion  string   `xml:"AppVersion"`
	Company     string   `xml:"Company"`
	Manager     string   `xml:"Manager"`
	Pages       int      `xml:"Pages"`
	Words       int      `xml:"Words"`
	Characters  int      `xml:"Characters"`
	Lines       int      `xml:"Lines"`
	Paragraphs  int      `xml:"Paragraphs"`
	TotalTime   int      `xml:"TotalTime"`
	Template    string   `xml:"Template"`
}

// CustomProperties represents docProps/custom.xml metadata.
type CustomProperties struct {
	XMLName    xml.Name         `xml:"Properties"`
	Properties []CustomProperty `xml:"property"`
}

// CustomProperty represents a single custom document property.
type CustomProperty struct {
	Name  string `xml:"name,attr"`
	Lpid  string `xml:"lpid,attr,omitempty"`
	Value string `xml:"-"`
}

// UnmarshalXML implements custom unmarshalling for CustomProperty because
// the value is wrapped in a typed element (<vt:lpwstr>, <vt:i4>, <vt:bool>,
// <vt:filetime>, <vt:r8>, etc.) rather than a simple <value> element.
func (c *CustomProperty) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "name":
			c.Name = a.Value
		case "lpid":
			c.Lpid = a.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			// Read the typed value element's text content.
			var val string
			if err := d.DecodeElement(&val, &t); err != nil {
				return err
			}
			c.Value = val
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

// MarshalXML emits <property> with a <vt:lpwstr> child for the value.
func (c CustomProperty) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "property"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "fmtid"}, Value: "{D5CDD505-2E9C-101B-9397-08002B2CF9AE}"},
		{Name: xml.Name{Local: "pid"}, Value: "2"},
		{Name: xml.Name{Local: "name"}, Value: c.Name},
	}
	if c.Lpid != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "lpid"}, Value: c.Lpid})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	vtStart := xml.StartElement{
		Name: xml.Name{Local: "vt:lpwstr"},
	}
	if err := e.EncodeToken(vtStart); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.CharData(c.Value)); err != nil {
		return err
	}
	if err := e.EncodeToken(vtStart.End()); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

// CommentsPart represents the parsed word/comments.xml part.
// ECMA-376 Part 1 §17.13.4.
type CommentsPart struct {
	XMLName     xml.Name   `xml:"comments"`
	Comments    []*Comment `xml:"comment"`
	Diagnostics diag.List  `xml:"-"`
}

// Comment represents a single w:comment element.
type Comment struct {
	ID          int            `xml:"id,attr"`
	Author      string         `xml:"author,attr,omitempty"`
	Date        string         `xml:"date,attr,omitempty"`
	Paragraphs  []*Paragraph   `xml:"p"`
	Tables      []*Table       `xml:"tbl"`
	Blocks      []ContentBlock `xml:"-"`
	Diagnostics diag.List      `xml:"-"`
}

// ResolvedReference describes a section header/footer reference resolved through document relationships.
type ResolvedReference struct {
	Kind           string
	Type           string
	RelationshipID string
	TargetPartName string
}

// Document represents word/document.xml.
type Document struct {
	XMLName     xml.Name    `xml:"document"`
	Background  *Background `xml:"background"`
	Body        *Body       `xml:"body"`
	Diagnostics diag.List   `xml:"-"`
}

// Background represents w:background — the document page background color
// and optional background image relationship.
type Background struct {
	Color      string `xml:"color,attr,omitempty"`
	ThemeColor string `xml:"themeColor,attr,omitempty"`
	ImageRelID string `xml:"-"`
}

// UnmarshalXML captures background color/theme metadata plus any nested VML
// image relationship used for page background pictures.
func (b *Background) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*b = Background{}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "color":
			b.Color = attr.Value
		case "themeColor":
			b.ThemeColor = attr.Value
		}
	}
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if b.ImageRelID == "" && (t.Name.Local == "fill" || t.Name.Local == "imagedata") {
				for _, attr := range t.Attr {
					if attr.Name.Local == "id" && attr.Value != "" {
						b.ImageRelID = attr.Value
						break
					}
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

// Body represents w:body.
type Body struct {
	Paragraphs  []*Paragraph       `xml:"p"`
	Tables      []*Table           `xml:"tbl"`
	AltChunks   []*AltChunk        `xml:"altChunk"`
	Section     *SectionProperties `xml:"sectPr"`
	Blocks      []BodyBlock        `xml:"-"`
	Diagnostics diag.List          `xml:"-"`
}

// BodyBlockKind identifies the kind of block stored in a body.
type BodyBlockKind string

const (
	BodyBlockParagraph BodyBlockKind = "paragraph"
	BodyBlockTable     BodyBlockKind = "table"
	BodyBlockSection   BodyBlockKind = "section"
	BodyBlockSDT       BodyBlockKind = "sdt" // w:sdt — structured document tag
	BodyBlockAltChunk  BodyBlockKind = "altChunk"
	BodyBlockCustomXml BodyBlockKind = "customXml" // w:customXml — custom XML markup
	BodyBlockRaw       BodyBlockKind = "raw"       // unsupported-but-retainable body child
)

// ContentBlockKind identifies ordered paragraph/table content inside cells and stories.
type ContentBlockKind string

const (
	ContentBlockParagraph ContentBlockKind = "paragraph"
	ContentBlockTable     ContentBlockKind = "table"
	ContentBlockSDT       ContentBlockKind = "sdt"       // w:sdt — structured document tag
	ContentBlockCustomXml ContentBlockKind = "customXml" // w:customXml — custom XML markup
)

// ParagraphInlineKind identifies ordered inline content inside a paragraph.
type ParagraphInlineKind string

const (
	ParagraphInlineRun       ParagraphInlineKind = "run"
	ParagraphInlineHyperlink ParagraphInlineKind = "hyperlink"
	ParagraphInlineField     ParagraphInlineKind = "field"
	ParagraphInlineIns       ParagraphInlineKind = "ins"       // w:ins — tracked insertion
	ParagraphInlineDel       ParagraphInlineKind = "del"       // w:del — tracked deletion
	ParagraphInlineMoveFrom  ParagraphInlineKind = "moveFrom"  // w:moveFrom — tracked move source
	ParagraphInlineMoveTo    ParagraphInlineKind = "moveTo"    // w:moveTo — tracked move destination
	ParagraphInlineMath      ParagraphInlineKind = "math"      // m:oMath / m:oMathPara — OMML equation
	ParagraphInlineSDT       ParagraphInlineKind = "sdt"       // w:sdt — structured document tag
	ParagraphInlineRuby      ParagraphInlineKind = "ruby"      // w:ruby — East Asian phonetic guide
	ParagraphInlineDir       ParagraphInlineKind = "dir"       // w:dir — inline text direction
	ParagraphInlineBdo       ParagraphInlineKind = "bdo"       // w:bdo — bidirectional override
	ParagraphInlineSmartTag  ParagraphInlineKind = "smartTag"  // w:smartTag — smart tag wrapper
	ParagraphInlineCustomXml ParagraphInlineKind = "customXml" // w:customXml — custom XML markup
)

// RunContentKind identifies ordered content inside a run.
type RunContentKind string

const (
	RunContentText                  RunContentKind = "text"
	RunContentTab                   RunContentKind = "tab"
	RunContentBreak                 RunContentKind = "break"
	RunContentDrawing               RunContentKind = "drawing"
	RunContentFldChar               RunContentKind = "fldChar"               // w:fldChar — field begin/separate/end
	RunContentInstrText             RunContentKind = "instrText"             // w:instrText — field instruction text
	RunContentDelInstrText          RunContentKind = "delInstrText"          // w:delInstrText — deleted field instruction text (tracked changes)
	RunContentFootnoteRef           RunContentKind = "footnoteRef"           // w:footnoteReference
	RunContentEndnoteRef            RunContentKind = "endnoteRef"            // w:endnoteReference
	RunContentCommentRef            RunContentKind = "commentRef"            // w:commentReference
	RunContentAnnotationRef         RunContentKind = "annotationRef"         // w:annotationRef
	RunContentSym                   RunContentKind = "sym"                   // w:sym — symbol character
	RunContentVMLImage              RunContentKind = "vmlImage"              // v:imagedata in mc:Fallback
	RunContentVMLTextBox            RunContentKind = "vmlTextbox"            // v:textbox / w:txbxContent in VML shapes
	RunContentVMLHorzRule           RunContentKind = "vmlHorzRule"           // v:rect o:hr="t" in w:pict
	RunContentVMLShape              RunContentKind = "vmlShape"              // v:rect/v:roundrect/v:oval/v:line basic VML shapes in w:pict
	RunContentOLE                   RunContentKind = "ole"                   // w:object — OLE embedded object (no preview image)
	RunContentSoftHyphen            RunContentKind = "softHyphen"            // w:softHyphen — optional hyphenation point
	RunContentNoBreakHyphen         RunContentKind = "noBreakHyphen"         // w:noBreakHyphen — non-breaking hyphen
	RunContentCR                    RunContentKind = "cr"                    // w:cr — carriage return (line break)
	RunContentPTab                  RunContentKind = "ptab"                  // w:ptab — custom tab character
	RunContentColumnBreak           RunContentKind = "columnBreak"           // w:col — column break
	RunContentRuby                  RunContentKind = "ruby"                  // w:ruby — East Asian phonetic guide
	RunContentMath                  RunContentKind = "math"                  // m:oMath — Office Math ML
	RunContentContentPart           RunContentKind = "contentPart"           // w:contentPart — content part reference
	RunContentDayShort              RunContentKind = "dayShort"              // w:dayShort — abbreviated day of week auto-text
	RunContentDayLong               RunContentKind = "dayLong"               // w:dayLong — full day of week auto-text
	RunContentMonthShort            RunContentKind = "monthShort"            // w:monthShort — abbreviated month auto-text
	RunContentMonthLong             RunContentKind = "monthLong"             // w:monthLong — full month auto-text
	RunContentYearShort             RunContentKind = "yearShort"             // w:yearShort — two-digit year auto-text
	RunContentYearLong              RunContentKind = "yearLong"              // w:yearLong — four-digit year auto-text
	RunContentPgNum                 RunContentKind = "pgNum"                 // w:pgNum — page number auto-text
	RunContentLastRenderedPageBreak RunContentKind = "lastRenderedPageBreak" // w:lastRenderedPageBreak — layout hint
)

// ContentPartRef represents a w:contentPart relationship reference inside a run.
type ContentPartRef struct {
	RelID string `xml:"id,attr,omitempty"`
}

// AltChunk represents w:altChunk — an alternative content chunk reference.
type AltChunk struct {
	RelID       string      `xml:"id,attr,omitempty"`
	Properties  *AltChunkPr `xml:"altChunkPr"`
	Diagnostics diag.List   `xml:"-"`
}

// AltChunkPr represents w:altChunkPr — properties for an alternative content chunk.
type AltChunkPr struct {
	MatchSrc *OnOff `xml:"matchSrc"`
}

// BodyBlock preserves top-level body order across paragraphs, tables, and the trailing sectPr.
type BodyBlock struct {
	Kind            BodyBlockKind
	Paragraph       *Paragraph
	Table           *Table
	Section         *SectionProperties
	SdtProps        *SdtProperties // non-nil when Kind == BodyBlockSDT
	SdtBlocks       []BodyBlock    // child blocks when Kind == BodyBlockSDT
	AltChunk        *AltChunk      // non-nil when Kind == BodyBlockAltChunk
	CustomXml       *CustomXml     // non-nil when Kind == BodyBlockCustomXml
	CustomXmlBlocks []BodyBlock    // child blocks when Kind == BodyBlockCustomXml
	Raw             *RawElement    // non-nil when Kind == BodyBlockRaw
}

// ContentBlock preserves ordered paragraph/table content inside a non-body container.
type ContentBlock struct {
	Kind            ContentBlockKind
	Paragraph       *Paragraph
	Table           *Table
	SdtProps        *SdtProperties // non-nil when Kind == ContentBlockSDT
	SdtBlocks       []ContentBlock // child blocks when Kind == ContentBlockSDT
	CustomXml       *CustomXml     // non-nil when Kind == ContentBlockCustomXml
	CustomXmlBlocks []ContentBlock // child blocks when Kind == ContentBlockCustomXml
}

// Paragraph represents a minimal w:p model for ingestion.
type Paragraph struct {
	Properties                   *ParagraphProperties `xml:"pPr"`
	Runs                         []*Run               `xml:"r"`
	Hyperlinks                   []*Hyperlink         `xml:"hyperlink"`
	Inlines                      []ParagraphInline    `xml:"-"`
	Bookmarks                    []string             `xml:"-"` // w:name values from w:bookmarkStart (unmarshal only)
	BookmarkStartElems           []BookmarkStart      `xml:"bookmarkStart"`
	BookmarkEndElems             []BookmarkEnd        `xml:"bookmarkEnd"`
	CommentRangeStarts           []int                `xml:"-"` // w:id values from w:commentRangeStart (unmarshal only)
	CommentRangeEnds             []int                `xml:"-"` // w:id values from w:commentRangeEnd (unmarshal only)
	CommentRangeStartElems       []CommentRangeStart  `xml:"commentRangeStart"`
	CommentRangeEndElems         []CommentRangeEnd    `xml:"commentRangeEnd"`
	MoveFromRangeStarts          []MoveRangeStart     `xml:"moveFromRangeStart"`
	MoveFromRangeEnds            []MoveRangeEnd       `xml:"moveFromRangeEnd"`
	MoveToRangeStarts            []MoveRangeStart     `xml:"moveToRangeStart"`
	MoveToRangeEnds              []MoveRangeEnd       `xml:"moveToRangeEnd"`
	PermStarts                   []PermStart          `xml:"permStart"`
	PermEnds                     []PermEnd            `xml:"permEnd"`
	CustomXmlInsRangeStarts      []MoveRangeStart     `xml:"customXmlInsRangeStart"`
	CustomXmlInsRangeEnds        []MoveRangeEnd       `xml:"customXmlInsRangeEnd"`
	CustomXmlDelRangeStarts      []MoveRangeStart     `xml:"customXmlDelRangeStart"`
	CustomXmlDelRangeEnds        []MoveRangeEnd       `xml:"customXmlDelRangeEnd"`
	CustomXmlMoveFromRangeStarts []MoveRangeStart     `xml:"customXmlMoveFromRangeStart"`
	CustomXmlMoveFromRangeEnds   []MoveRangeEnd       `xml:"customXmlMoveFromRangeEnd"`
	CustomXmlMoveToRangeStarts   []MoveRangeStart     `xml:"customXmlMoveToRangeStart"`
	CustomXmlMoveToRangeEnds     []MoveRangeEnd       `xml:"customXmlMoveToRangeEnd"`
	Diagnostics                  diag.List            `xml:"-"`
}

// BookmarkStart represents w:bookmarkStart.
type BookmarkStart struct {
	ID   int    `xml:"id,attr"`
	Name string `xml:"name,attr"`
}

// BookmarkEnd represents w:bookmarkEnd.
type BookmarkEnd struct {
	ID int `xml:"id,attr"`
}

// CommentRangeStart represents w:commentRangeStart.
type CommentRangeStart struct {
	ID int `xml:"id,attr"`
}

// CommentRangeEnd represents w:commentRangeEnd.
type CommentRangeEnd struct {
	ID int `xml:"id,attr"`
}

// PermStart represents w:permStart — a permission range start.
type PermStart struct {
	ID       int    `xml:"id,attr"`
	EdGrp    string `xml:"edGrp,attr,omitempty"`
	Ed       string `xml:"ed,attr,omitempty"`
	ColFirst int    `xml:"colFirst,attr,omitempty"`
	ColLast  int    `xml:"colLast,attr,omitempty"`
}

// PermEnd represents w:permEnd — a permission range end.
type PermEnd struct {
	ID int `xml:"id,attr"`
}

// TrackChangeNumbering represents w:numberingChange inside a field character.
type TrackChangeNumbering struct {
	ID       int    `xml:"id,attr"`
	Author   string `xml:"author,attr,omitempty"`
	Date     string `xml:"date,attr,omitempty"`
	Original string `xml:"original,attr,omitempty"`
}

// ParagraphInline preserves ordered runs, hyperlinks, field instructions, and math.
type ParagraphInline struct {
	Kind             ParagraphInlineKind
	Run              *Run
	Hyperlink        *Hyperlink
	Field            *Field
	Math             *OMMath
	Ruby             *Ruby             // non-nil when Kind == ParagraphInlineRuby
	SdtProps         *SdtProperties    // non-nil when Kind == ParagraphInlineSDT
	SdtInlines       []ParagraphInline // child inlines when Kind == ParagraphInlineSDT
	CustomXml        *CustomXml        // non-nil when Kind == ParagraphInlineCustomXml
	CustomXmlInlines []ParagraphInline // child inlines when Kind == ParagraphInlineCustomXml
	// RevisionAuthor and RevisionDate are populated for ParagraphInlineIns,
	// ParagraphInlineDel, ParagraphInlineMoveFrom, and ParagraphInlineMoveTo
	// inlines from the w:author and w:date attributes of the enclosing element.
	RevisionAuthor string
	RevisionDate   string
	// MoveID is the w:id attribute of a w:moveFrom / w:moveTo element, used to
	// correlate the source and destination of a tracked move.
	MoveID int
	// DirVal is the w:val attribute of a w:dir or w:bdo element ("ltr" or "rtl").
	DirVal string
	// SmartTagURI and SmartTagElement are the w:uri and w:element attributes of a w:smartTag.
	SmartTagURI     string
	SmartTagElement string
}

// Ruby holds the parsed content of a w:ruby element (East Asian phonetic guide).
// ECMA-376 Part 1 §17.3.3.26.
type Ruby struct {
	BaseRuns []*Run  // w:rubyBase/w:r — base text runs
	TextRuns []*Run  // w:rt/w:r — phonetic annotation runs
	RubyPr   *RubyPr // w:rubyPr — ruby properties
}

// RubyPr represents w:rubyPr — ruby phonetic guide properties.
type RubyPr struct {
	PhoneticGuideTextFontSize *IntVal    `xml:"hps"`         // half-points
	BaseTextFontSize          *IntVal    `xml:"hpsBaseText"` // half-points
	PhoneticGuideRaise        *IntVal    `xml:"hpsRaise"`    // half-points
	LanguageID                *StringVal `xml:"lid"`
	RubyAlign                 *StringVal `xml:"rubyAlign"` // center, distributeLetter, distributeSpace, left, right, rightVertical
	// Dirty marks the ruby as needing recalculation (w:dirty).
	Dirty *OnOff `xml:"dirty"`
}

// OMMath holds the raw XML bytes of an m:oMath or m:oMathPara element.
type OMMath struct {
	RawXML  []byte
	Display bool // true when parsed from m:oMathPara (display/block mode)
}

// SdtProperties holds parsed w:sdtPr content for a structured document tag (w:sdt).
// ECMA-376 Part 1 §17.5.2.
type SdtProperties struct {
	ID    int    // w:id/@w:val; 0 = not set
	Alias string // w:alias/@w:val — display title shown in the UI
	Tag   string // w:tag/@w:val — programmatic identifier
	// Type is one of "plainText", "richText", "checkBox", "comboBox", "dropDownList",
	// "date", "picture", "docPartObj", "docPartList", "citation", "equation", "group",
	// "repeatingSection", "repeatingSectionItem", "contentPart", or "" when implicit.
	Type                      string
	Checked                   bool          // checkBox: w:checked/@w:val == "1" (absent = false)
	Checked14                 bool          // w14:checkbox: w14:checked present (or val == "1")
	DateFormat                string        // date: w:dateFormat/@w:val
	DateFullDate              string        // date: w:fullDate/@w:val
	DateLanguage              string        // date: w:lid/@w:val
	DateCalendar              string        // date: w:calendar/@w:val
	DocPartGallery            string        // docPartObj: w:docPartGallery/@w:val
	DocPartCategory           string        // docPartObj: w:docPartCategory/@w:val
	DocPartUnique             bool          // docPartObj: w:docPartUnique present
	ListItems                 []SdtListItem // comboBox/dropDownList: list items with display text and value
	ShowingPlcHdr             bool          // w:showingPlcHdr — placeholder text is currently shown
	PlaceholderDocPart        string        // w:placeholder/w:docPart/@w:val
	PlaceholderDocPartGallery string        // w:placeholder/w:docPartGallery/@w:val
	DataXPath                 string        // w:dataBinding/@w:xpath
	DataStoreID               string        // w:dataBinding/@w:storeItemID
	Lock                      string        // w:lock/@w:val
	Temporary                 bool          // w:temporary
	Appearance                string        // w15:appearance/@w15:val
	Color                     string        // w15:color/@w15:val
	PlainTextMultiLine        bool          // w:text/@w:multiLine
}

// Field represents a w:fldSimple element — a simple field instruction.
// ECMA-376 Part 1 §17.16.19.
type Field struct {
	Instr string
	Runs  []*Run
}

// CustomXml represents a w:customXml element — inline or block-level custom XML markup.
// ECMA-376 Part 1 §17.5.2.6.
type CustomXml struct {
	URI        string       `xml:"uri,attr"`
	Element    string       `xml:"element,attr"`
	Properties *CustomXmlPr `xml:"customXmlPr"`
}

// CustomXmlPr holds the properties of a custom XML element.
type CustomXmlPr struct {
	Attributes []CustomXmlAttr `xml:"attr"`
}

// CustomXmlAttr represents a single attribute on a custom XML element.
type CustomXmlAttr struct {
	URI   string `xml:"uri,attr"`
	Name  string `xml:"name,attr"`
	Value string `xml:"val,attr"`
}

// PropertyChangeInfo captures the tracked-change attribution on *PrChange
// elements such as w:rPrChange, w:pPrChange, w:tblPrChange, and w:trPrChange.
type PropertyChangeInfo struct {
	ID                 int                 `xml:"id,attr"`
	Author             string              `xml:"author,attr,omitempty"`
	Date               string              `xml:"date,attr,omitempty"`
	RunProperties      *RunProperties      `xml:"rPr"`     // from rPrChange / pPrChange
	TableProperties    *TableProperties    `xml:"tblPrEx"` // from tblPrChange
	TableRowProperties *TableRowProperties `xml:"trPr"`    // from trPrChange
}

// ParagraphProperties represents w:pPr.
type ParagraphProperties struct {
	Style             *StringVal          `xml:"pStyle"`
	Numbering         *NumberingReference `xml:"numPr"`
	Justification     *StringVal          `xml:"jc"`
	Spacing           *Spacing            `xml:"spacing"`
	Indentation       *Indentation        `xml:"ind"`
	Shading           *Shading            `xml:"shd"` // w:shd — paragraph fill
	Section           *SectionProperties  `xml:"sectPr"`
	ContextualSpacing *OnOff              `xml:"contextualSpacing"`
	SnapToGrid        *OnOff              `xml:"snapToGrid"`
	KeepNext          *OnOff              `xml:"keepNext"`
	KeepLines         *OnOff              `xml:"keepLines"`
	WidowControl      *OnOff              `xml:"widowControl"`
	PageBreakBefore   *OnOff              `xml:"pageBreakBefore"`
	Borders           *ParagraphBorders   `xml:"pBdr"`
	Frame             *FramePr            `xml:"framePr"`
	// Tabs holds custom tab stops defined for this paragraph (w:tabs > w:tab).
	Tabs []TabStop `xml:"tabs>tab"`
	// Bidi marks the paragraph as right-to-left (w:bidi). ECMA-376 Part 1 §17.3.1.6.
	Bidi *OnOff `xml:"bidi"`
	// MirrorIndents mirrors left/right indents on facing pages (w:mirrorIndents).
	MirrorIndents *OnOff `xml:"mirrorIndents"`
	// NoAllowOverlap prevents the paragraph frame from overlapping other frames (w:noAllowOverlap).
	NoAllowOverlap *OnOff `xml:"noAllowOverlap"`
	// SuppressLineNumbers suppresses line numbering for this paragraph (w:suppressLineNumbers).
	SuppressLineNumbers *OnOff `xml:"suppressLineNumbers"`
	// TextAlignment is the font alignment: "top", "center", "baseline", "bottom", "auto" (w:textAlignment).
	TextAlignment *StringVal `xml:"textAlignment"`
	// Kinsoku controls Japanese kinsoku line-break rules (w:kinsoku).
	Kinsoku *OnOff `xml:"kinsoku"`
	// WordWrap controls East Asian word wrapping (w:wordWrap).
	WordWrap *OnOff `xml:"wordWrap"`
	// OverflowPunct controls overflow punctuation (w:overflowPunct).
	OverflowPunct *OnOff `xml:"overflowPunct"`
	// TopLinePunct controls top-line punctuation (w:topLinePunct).
	TopLinePunct *OnOff `xml:"topLinePunct"`
	// AutoSpaceDE automatically adjusts space between East Asian and Latin text (w:autoSpaceDE).
	AutoSpaceDE *OnOff `xml:"autoSpaceDE"`
	// AutoSpaceDN automatically adjusts space between East Asian and numbers (w:autoSpaceDN).
	AutoSpaceDN *OnOff `xml:"autoSpaceDN"`
	// TextDirection is w:textDirection — paragraph text direction ("lrTb", "tbRl", "btLr", "tbRlV").
	TextDirection *StringVal `xml:"textDirection"`
	// OutlineLvl is w:outlineLvl@w:val — the heading outline level for TOC
	// generation (0..8 = heading 1..9, 9 = body). ECMA-376 Part 1 §17.3.1.20.
	OutlineLvl *IntVal `xml:"outlineLvl"`
	// SuppressAutoHyphens suppresses auto-hyphenation for this paragraph when
	// hyphenation is enabled for the document (w:suppressAutoHyphens).
	SuppressAutoHyphens *OnOff `xml:"suppressAutoHyphens"`
	// AdjustRightInd automatically adjusts the right indent in RTL mode (w:adjustRightInd).
	AdjustRightInd *OnOff `xml:"adjustRightInd"`
	// DivID is w:divId — the paragraph's division ID for HTML round-tripping.
	DivID *IntVal `xml:"divId"`
	// CnfStyle is w:cnfStyle — conditional table-style formatting flags.
	CnfStyle *StringVal `xml:"cnfStyle"`
	// RunProps holds the paragraph-level w:rPr — default run properties for all runs
	// in this paragraph. ECMA-376 Part 1 §17.3.1.27.
	RunProps *RunProperties `xml:"-"`
	// BlockQuote indicates this paragraph is a block quotation (w:blockQuote).
	BlockQuote *OnOff `xml:"blockQuote"`
	// PropertyChange is w:pPrChange — tracked paragraph property revision metadata.
	PropertyChange *PropertyChangeInfo `xml:"pPrChange"`
}

// UnmarshalXML preserves the w:rPr child element inside w:pPr.
func (p *ParagraphProperties) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*p = ParagraphProperties{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "pStyle":
				var v StringVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Style = &v
			case "numPr":
				var v NumberingReference
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Numbering = &v
			case "jc":
				var v StringVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Justification = &v
			case "spacing":
				var v Spacing
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Spacing = &v
			case "ind":
				var v Indentation
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Indentation = &v
			case "shd":
				var v Shading
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Shading = &v
			case "sectPr":
				var v SectionProperties
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Section = &v
			case "contextualSpacing":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.ContextualSpacing = &v
			case "snapToGrid":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.SnapToGrid = &v
			case "keepNext":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.KeepNext = &v
			case "keepLines":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.KeepLines = &v
			case "widowControl":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.WidowControl = &v
			case "pageBreakBefore":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.PageBreakBefore = &v
			case "pPrChange":
				var v PropertyChangeInfo
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.PropertyChange = &v
			case "pBdr":
				var v ParagraphBorders
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Borders = &v
			case "framePr":
				var v FramePr
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Frame = &v
			case "tabs":
				var v struct {
					Tabs []TabStop `xml:"tab"`
				}
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Tabs = v.Tabs
			case "bidi":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Bidi = &v
			case "mirrorIndents":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.MirrorIndents = &v
			case "noAllowOverlap":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.NoAllowOverlap = &v
			case "suppressLineNumbers":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.SuppressLineNumbers = &v
			case "textAlignment":
				var v StringVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.TextAlignment = &v
			case "kinsoku":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Kinsoku = &v
			case "wordWrap":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.WordWrap = &v
			case "overflowPunct":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.OverflowPunct = &v
			case "topLinePunct":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.TopLinePunct = &v
			case "autoSpaceDE":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.AutoSpaceDE = &v
			case "autoSpaceDN":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.AutoSpaceDN = &v
			case "textDirection":
				var v StringVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.TextDirection = &v
			case "outlineLvl":
				var v IntVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.OutlineLvl = &v
			case "rPr":
				var v RunProperties
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.RunProps = &v
			case "suppressAutoHyphens":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.SuppressAutoHyphens = &v
			case "adjustRightInd":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.AdjustRightInd = &v
			case "divId":
				var v IntVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.DivID = &v
			case "cnfStyle":
				var v StringVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.CnfStyle = &v
			default:
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// FramePr represents w:framePr — legacy paragraph frame positioning.
type FramePr struct {
	W       *int   `xml:"w,attr,omitempty"`
	H       *int   `xml:"h,attr,omitempty"`
	X       *int   `xml:"x,attr,omitempty"`
	Y       *int   `xml:"y,attr,omitempty"`
	XAlign  string `xml:"xAlign,attr,omitempty"`
	YAlign  string `xml:"yAlign,attr,omitempty"`
	HAnchor string `xml:"hAnchor,attr,omitempty"`
	VAnchor string `xml:"vAnchor,attr,omitempty"`
	Wrap    string `xml:"wrap,attr,omitempty"`
	HSpace  *int   `xml:"hSpace,attr,omitempty"`
	VSpace  *int   `xml:"vSpace,attr,omitempty"`
	DropCap string `xml:"dropCap,attr,omitempty"`
	Lines   *int   `xml:"lines,attr,omitempty"`
}

// ParagraphBorders holds the border slots for a paragraph (w:pBdr).
type ParagraphBorders struct {
	Top     *Border `xml:"top"`
	Left    *Border `xml:"left"`
	Start   *Border `xml:"start"`
	Bottom  *Border `xml:"bottom"`
	Right   *Border `xml:"right"`
	End     *Border `xml:"end"`
	Between *Border `xml:"between"`
	Bar     *Border `xml:"bar"`
}

// TabStop represents a single w:tab element inside w:tabs.
type TabStop struct {
	Val    string `xml:"val,attr,omitempty"`    // alignment: left, center, right, decimal, bar, clear, num
	Pos    int    `xml:"pos,attr,omitempty"`    // position in twips
	Leader string `xml:"leader,attr,omitempty"` // leader: none, dot, hyphen, underscore, middleDot
}

// PTab represents w:ptab — a custom tab character inside a run.
type PTab struct {
	Alignment  string `xml:"alignment,attr,omitempty"`  // left, center, right, decimal, bar
	Leader     string `xml:"leader,attr,omitempty"`     // none, dot, hyphen, underscore, middleDot
	RelativeTo string `xml:"relativeTo,attr,omitempty"` // margin, indent
}

// EastAsianLayout represents w:eastAsianLayout — East Asian text layout settings.
type EastAsianLayout struct {
	Combine          *OnOff `xml:"combine,attr"`
	CombineBrackets  *OnOff `xml:"combineBrackets,attr"`
	Vert             *OnOff `xml:"vert,attr"`
	VertCompress     *OnOff `xml:"vertCompress,attr"`
	CombineCharCount int    `xml:"combineCharCount,attr,omitempty"`
}

// NumberingReference points at a numbering definition.
type NumberingReference struct {
	Level *IntVal `xml:"ilvl"`
	NumID *IntVal `xml:"numId"`
}

// Run represents w:r.
type Run struct {
	Properties         *RunProperties `xml:"rPr"`
	Text               []*Text        `xml:"t"`
	Breaks             []*Break       `xml:"br"`
	Contents           []RunContent   `xml:"-"`
	DrawingXML         []byte         `xml:"-"`
	Diagnostics        diag.List      `xml:"-"`
	BookmarkStarts     []string       // w:name values from w:bookmarkStart inside this run
	CommentRangeStarts []int          // w:id values from w:commentRangeStart inside this run
	CommentRangeEnds   []int          // w:id values from w:commentRangeEnd inside this run
}

// RunContent preserves ordered text/tab/break/drawing/field content inside a run.
type RunContent struct {
	Kind              RunContentKind
	Text              *Text
	Break             *Break
	Drawing           *Drawing
	DrawingXML        []byte // raw drawing XML for marshal round-trip when Drawing struct lacks namespaces
	FldChar           *FldChar
	InstrText         string
	DelInstrText      string
	NoteRef           *NoteReference
	CommentRef        *CommentReference
	Sym               *Sym              // for RunContentSym
	VMLRelID          string            // for RunContentVMLImage — r:id of the image relationship
	VMLStyle          string            // for VML image/textbox — v:shape style attribute for dimension/position parsing
	VMLTextBoxStyle   string            // for RunContentVMLTextBox — v:textbox style attribute
	VMLTextBox        *TextBoxContent   // for RunContentVMLTextBox — w:txbxContent parsed from VML
	VMLFillColor      string            // for RunContentVMLTextBox — v:shape fillcolor attribute
	VMLHorzRuleColor  string            // for RunContentVMLHorzRule — fillcolor attribute value
	VMLHorzRuleAlign  string            // for RunContentVMLHorzRule — o:hralign attribute value
	VMLShapeType      string            // for RunContentVMLShape — VML local element name (rect|roundrect|oval|line)
	VMLStrokeColor    string            // for RunContentVMLShape — strokecolor attribute
	VMLStrokeWeight   string            // for RunContentVMLShape — strokeweight attribute
	VMLFilledAttr     string            // for RunContentVMLShape — filled attribute raw value
	VMLStrokedAttr    string            // for RunContentVMLShape — stroked attribute raw value
	VMLFrom           string            // for RunContentVMLShape — line start coordinate
	VMLTo             string            // for RunContentVMLShape — line end coordinate
	VMLPoints         string            // for RunContentVMLShape — polyline/curve points attribute
	VMLStartAngle     string            // for RunContentVMLShape — arc start angle
	VMLEndAngle       string            // for RunContentVMLShape — arc end angle
	VMLPathData       string            // for RunContentVMLShape — v:shape path attribute or <v:path v="...">
	VMLCoordSize      string            // for RunContentVMLShape — coordsize attribute
	VMLCoordOrigin    string            // for RunContentVMLShape — coordorigin attribute
	VMLTextPathOn     string            // for RunContentVMLShape — v:textpath@on
	VMLTextPathString string            // for RunContentVMLShape — v:textpath@string
	VMLSignatureLine  *VMLSignatureLine // for RunContentVMLShape — o:signatureline parsed from VML shape
	OLE               *OLEObjectRun     // for RunContentOLE — OLE embedded object without preview
	PTab              *PTab             // for RunContentPTab — custom tab character
	Ruby              *RunRuby          // for RunContentRuby — East Asian phonetic guide
	MathML            string            // for RunContentMath — raw m:oMath XML (namespace should be included)
	ContentPart       *ContentPartRef   // for RunContentContentPart — content part relationship reference
}

// RunRuby holds simple base + annotation text for marshal inside a run.
type RunRuby struct {
	Base string
	Text string
}

// OLEObjectRun carries metadata about a w:object OLE element.
type OLEObjectRun struct {
	ProgID       string // e.g. "Excel.Sheet.12", may be empty
	RelID        string // r:id pointing to the embedded or linked OLE object
	PreviewRelID string // r:id of the VML preview image, when present
	Type         string // "Embed" or "Link" when authored
	DrawAspect   string
	ObjectID     string
	UpdateMode   string
}

// Sym represents w:sym — a symbol character from a named font.
// ECMA-376 Part 1 §17.3.3.30.
type Sym struct {
	Font string `xml:"font,attr,omitempty"`
	Char string `xml:"char,attr,omitempty"` // hex Unicode code point (may include F000 offset)
}

// NoteReference represents w:footnoteReference or w:endnoteReference.
// ECMA-376 Part 1 §17.11.15, §17.11.5.
type NoteReference struct {
	ID int `xml:"id,attr"`
}

// CommentReference represents w:commentReference.
// ECMA-376 Part 1 §17.13.4.5.
type CommentReference struct {
	ID int `xml:"id,attr"`
}

// ECMA-376 Part 1 §17.16.18.
type FldChar struct {
	Type            string                // "begin", "separate", or "end" — from fldCharType attribute
	Dirty           bool                  // w:dirty — field result needs recalculation
	FormField       *FormFieldData        // non-nil when fldChar contains w:ffData (only for type=="begin")
	FldData         string                // w:fldData — cached field data text
	NumberingChange *TrackChangeNumbering // w:numberingChange — tracked numbering change
}

// UnmarshalXML implements xml.Unmarshaler for FldChar so that nested w:ffData is parsed
// alongside the fldCharType attribute.
func (f *FldChar) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*f = FldChar{}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "fldCharType":
			f.Type = attr.Value
		case "dirty":
			f.Dirty = attr.Value == "true" || attr.Value == "1"
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "ffData":
				var ffd FormFieldData
				if err := ffd.unmarshal(dec, t); err != nil {
					return err
				}
				f.FormField = &ffd
			case "fldData":
				var text Text
				if err := dec.DecodeElement(&text, &t); err != nil {
					return err
				}
				f.FldData = text.Value
			case "numberingChange":
				var tc TrackChangeNumbering
				if err := dec.DecodeElement(&tc, &t); err != nil {
					return err
				}
				f.NumberingChange = &tc
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

// FormFieldData captures w:ffData from a w:fldChar begin element.
// ECMA-376 Part 1 §17.16.9.
type FormFieldData struct {
	Name             string   // w:name/@w:val
	Kind             string   // "text", "checkbox", or "dropdown" — derived from child element
	Enabled          bool     // w:enabled; absence means true
	CalcOnExit       bool     // w:calcOnExit
	DefaultTxt       string   // w:textInput/w:default/@w:val
	MaxLength        int      // w:textInput/w:maxLength/@w:val
	TextType         string   // w:textInput/w:type/@w:val
	TextFormat       string   // w:textInput/w:format/@w:val
	Checked          bool     // w:checkBox/w:checked present, or w:checkBox/w:default/@w:val=="1"
	CheckboxSize     int      // w:checkBox/w:size/@w:val (half-points)
	CheckboxSizeAuto bool     // w:checkBox/w:sizeAuto present
	DropItems        []string // w:ddList/w:listEntry/@w:val values
	DropDefIdx       int      // w:ddList/w:defaultVal/@w:val
}

func (ffd *FormFieldData) unmarshal(dec *xml.Decoder, start xml.StartElement) error {
	ffd.Enabled = true // OOXML default: enabled when absent
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "name":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						ffd.Name = a.Value
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "enabled":
				ffd.Enabled = true
				for _, a := range t.Attr {
					if a.Name.Local == "val" && (a.Value == "0" || a.Value == "false") {
						ffd.Enabled = false
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "calcOnExit":
				ffd.CalcOnExit = true
				for _, a := range t.Attr {
					if a.Name.Local == "val" && (a.Value == "0" || a.Value == "false") {
						ffd.CalcOnExit = false
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "textInput":
				ffd.Kind = "text"
				if err := ffd.unmarshalTextInput(dec, t); err != nil {
					return err
				}
			case "checkBox":
				ffd.Kind = "checkbox"
				if err := ffd.unmarshalCheckBox(dec, t); err != nil {
					return err
				}
			case "ddList":
				ffd.Kind = "dropdown"
				if err := ffd.unmarshalDdList(dec, t); err != nil {
					return err
				}
			case "helpText", "statusText", "entryMacro", "exitMacro":
				if err := dec.Skip(); err != nil {
					return err
				}
			case "control", "label", "tabIndex":
				if err := dec.Skip(); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (ffd *FormFieldData) unmarshalTextInput(dec *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "default":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						ffd.DefaultTxt = a.Value
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "type":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						ffd.TextType = a.Value
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "format":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						ffd.TextFormat = a.Value
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "maxLength":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						n, _ := strconv.Atoi(a.Value)
						ffd.MaxLength = n
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (ffd *FormFieldData) unmarshalCheckBox(dec *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "checked":
				// w:checked is either a boolean val attribute or a toggle element whose
				// presence (without val=0) means true.
				ffd.Checked = true
				for _, a := range t.Attr {
					if a.Name.Local == "val" && (a.Value == "0" || a.Value == "false") {
						ffd.Checked = false
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "default":
				// w:default/@w:val == "1" also means checked
				for _, a := range t.Attr {
					if a.Name.Local == "val" && a.Value == "1" {
						ffd.Checked = true
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "size":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						n, _ := strconv.Atoi(a.Value)
						ffd.CheckboxSize = n
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "sizeAuto":
				ffd.CheckboxSizeAuto = true
				for _, a := range t.Attr {
					if a.Name.Local == "val" && (a.Value == "0" || a.Value == "false") {
						ffd.CheckboxSizeAuto = false
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (ffd *FormFieldData) unmarshalDdList(dec *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "listEntry":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						ffd.DropItems = append(ffd.DropItems, a.Value)
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "defaultVal":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						n, _ := strconv.Atoi(a.Value)
						ffd.DropDefIdx = n
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

// MarshalXML emits w:fldChar with the fldCharType attribute and optional w:ffData.
func (f FldChar) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	wNS := "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	start.Name = xml.Name{Space: wNS, Local: "fldChar"}
	if f.Type != "" {
		start.Attr = []xml.Attr{{Name: xml.Name{Local: "fldCharType"}, Value: f.Type}}
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if f.FormField != nil {
		if err := f.FormField.marshal(e, wNS); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (ffd *FormFieldData) marshal(e *xml.Encoder, wNS string) error {
	start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "ffData"}}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if ffd.Name != "" {
		if err := e.EncodeElement(struct {
			Val string `xml:"val,attr"`
		}{Val: ffd.Name}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "name"}}); err != nil {
			return err
		}
	}
	if !ffd.Enabled {
		if err := e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "enabled"}}); err != nil {
			return err
		}
	}
	if ffd.CalcOnExit {
		if err := e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "calcOnExit"}}); err != nil {
			return err
		}
	}
	switch ffd.Kind {
	case "text":
		if err := ffd.marshalTextInput(e, wNS); err != nil {
			return err
		}
	case "checkbox":
		if err := ffd.marshalCheckBox(e, wNS); err != nil {
			return err
		}
	case "dropdown":
		if err := ffd.marshalDdList(e, wNS); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (ffd *FormFieldData) marshalTextInput(e *xml.Encoder, wNS string) error {
	start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "textInput"}}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if ffd.DefaultTxt != "" {
		if err := e.EncodeElement(struct {
			Val string `xml:"val,attr"`
		}{Val: ffd.DefaultTxt}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "default"}}); err != nil {
			return err
		}
	}
	if ffd.MaxLength > 0 {
		if err := e.EncodeElement(struct {
			Val int `xml:"val,attr"`
		}{Val: ffd.MaxLength}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "maxLength"}}); err != nil {
			return err
		}
	}
	if ffd.TextType != "" {
		if err := e.EncodeElement(struct {
			Val string `xml:"val,attr"`
		}{Val: ffd.TextType}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "type"}}); err != nil {
			return err
		}
	}
	if ffd.TextFormat != "" {
		if err := e.EncodeElement(struct {
			Val string `xml:"val,attr"`
		}{Val: ffd.TextFormat}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "format"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (ffd *FormFieldData) marshalCheckBox(e *xml.Encoder, wNS string) error {
	start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "checkBox"}}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if ffd.CheckboxSizeAuto {
		if err := e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "sizeAuto"}}); err != nil {
			return err
		}
	} else if ffd.CheckboxSize > 0 {
		if err := e.EncodeElement(struct {
			Val int `xml:"val,attr"`
		}{Val: ffd.CheckboxSize}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "size"}}); err != nil {
			return err
		}
	}
	if ffd.Checked {
		if err := e.EncodeElement(struct {
			Val string `xml:"val,attr"`
		}{Val: "1"}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "checked"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (ffd *FormFieldData) marshalDdList(e *xml.Encoder, wNS string) error {
	start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "ddList"}}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if ffd.DropDefIdx > 0 {
		if err := e.EncodeElement(struct {
			Val int `xml:"val,attr"`
		}{Val: ffd.DropDefIdx}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "defaultVal"}}); err != nil {
			return err
		}
	}
	for _, item := range ffd.DropItems {
		if err := e.EncodeElement(struct {
			Val string `xml:"val,attr"`
		}{Val: item}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "listEntry"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// Text represents w:t.
type Text struct {
	Space string `xml:"space,attr,omitempty"`
	Value string `xml:",chardata"`
}

// Hyperlink represents w:hyperlink with nested runs.
type Hyperlink struct {
	ID          string    `xml:"id,attr,omitempty"`
	Anchor      string    `xml:"anchor,attr,omitempty"`
	Runs        []*Run    `xml:"r"`
	Diagnostics diag.List `xml:"-"`
}

// RunProperties represents w:rPr.
type RunProperties struct {
	RunStyle      *StringVal                `xml:"rStyle"`
	Fonts         *RunFonts                 `xml:"rFonts"`
	Bold          *OnOff                    `xml:"b"`
	Italic        *OnOff                    `xml:"i"`
	Caps          *OnOff                    `xml:"caps"`
	SmallCaps     *OnOff                    `xml:"smallCaps"`
	Strike        *OnOff                    `xml:"strike"`
	DoubleStrike  *OnOff                    `xml:"dstrike"`
	Underline     *Underline                `xml:"u"`
	Color         *RunColor                 `xml:"color"`
	FontSize      *IntVal                   `xml:"sz"`
	TextFill      *Word2010TextFill         `xml:"http://schemas.microsoft.com/office/word/2010/wordml textFill"`
	TextOutline   *Word2010TextOutline      `xml:"http://schemas.microsoft.com/office/word/2010/wordml textOutline"`
	Glow          *Word2010GlowEffect       `xml:"http://schemas.microsoft.com/office/word/2010/wordml glow"`
	Shadow        *Word2010ShadowEffect     `xml:"http://schemas.microsoft.com/office/word/2010/wordml shadow"`
	SoftEdge      *Word2010SoftEdgeEffect   `xml:"http://schemas.microsoft.com/office/word/2010/wordml softEdge"`
	Reflection    *Word2010ReflectionEffect `xml:"http://schemas.microsoft.com/office/word/2010/wordml reflection"`
	Ligatures     *Word2010StringVal        `xml:"http://schemas.microsoft.com/office/word/2010/wordml ligatures"`
	NumSpacing    *Word2010StringVal        `xml:"http://schemas.microsoft.com/office/word/2010/wordml numSpacing"`
	StylisticSets *Word2010StylisticSets    `xml:"http://schemas.microsoft.com/office/word/2010/wordml stylisticSets"`
	Highlight     *StringVal                `xml:"highlight"`
	VertAlign     *StringVal                `xml:"vertAlign"`
	Shading       *Shading                  `xml:"shd"`     // w:shd — run background fill
	Vanish        *OnOff                    `xml:"vanish"`  // w:vanish — hidden text
	NoProof       *OnOff                    `xml:"noProof"` // w:noProof — exclude text from proofing
	Lang          *RunLang                  `xml:"lang"`    // w:lang — language hint
	Spacing       *IntVal                   `xml:"spacing"` // w:spacing — character spacing in twentieths of a point
	// RTL marks the run as right-to-left complex-script text (w:rtl). ECMA-376 Part 1 §17.3.2.30.
	RTL *OnOff `xml:"rtl"`
	// CS marks the run as using complex-script formatting (w:cs). ECMA-376 Part 1 §17.3.2.7.
	CS *OnOff `xml:"cs"`
	// Word 2003-era simple toggle text effects. The Word 2010 DrawingML forms
	// (TextOutline / Shadow above) supersede these but the toggles still appear
	// in older docs and many style libraries.
	OutlineToggle *OnOff `xml:"outline"`
	ShadowToggle  *OnOff `xml:"shadow"`
	Emboss        *OnOff `xml:"emboss"`
	Imprint       *OnOff `xml:"imprint"`
	// Borders specifies character-level borders for this run (w:bdr).
	Borders *Border `xml:"bdr"`
	// EastAsianLayout controls East Asian text layout (w:eastAsianLayout).
	EastAsianLayout *EastAsianLayout `xml:"eastAsianLayout"`
	// WebHidden hides the run in Web Layout view (w:webHidden).
	WebHidden *OnOff `xml:"webHidden"`
	// Kerning is the minimum font size (in half-points) above which kerning is
	// applied (w:kern). 0 or nil means kerning is disabled.
	Kerning *IntVal `xml:"kern"`
	// BoldCS marks the run as bold for complex-script text (w:bCs). nil = unset.
	BoldCS *OnOff `xml:"bCs"`
	// ItalicCS marks the run as italic for complex-script text (w:iCs). nil = unset.
	ItalicCS *OnOff `xml:"iCs"`
	// FontSizeCS is the font size for complex-script text in half-points (w:szCs).
	// nil = unset.
	FontSizeCS *IntVal `xml:"szCs"`
	// CharScale is the horizontal scaling of characters as a percentage of
	// their normal width (w:w). 100 = normal; nil = unset.
	CharScale *IntVal `xml:"w"`
	// FitText is the width (in twips) that the run text should be fit into
	// (w:fitText). nil = unset (no fit-text scaling).
	FitText *IntVal `xml:"fitText"`
	// EmphasisMark is w:em, the East Asian character emphasis mark.
	EmphasisMark *StringVal `xml:"em"`
	// Position is w:position — vertical position of the run relative to the
	// baseline in half-points. Positive = raised, negative = lowered.
	Position *IntVal `xml:"position"`
	// Effect is w:effect — text animation effect ("blinkBackground", "lights",
	// "antsBlack", "antsRed", "shimmer", "sparkle", "none").
	Effect *StringVal `xml:"effect"`
	// SpecVanish makes the run invisible even when formatting marks are shown
	// (w:specVanish). Used for paragraph marks in list items.
	SpecVanish *OnOff `xml:"specVanish"`
	// OMath marks the run as Office Math text (w:oMath). nil = unset.
	OMath *OnOff `xml:"oMath"`
	// PropertyChange is w:rPrChange — tracked run property revision metadata.
	PropertyChange *PropertyChangeInfo `xml:"rPrChange"`
}

type Word2010StringVal struct {
	Val string `xml:"http://schemas.microsoft.com/office/word/2010/wordml val,attr,omitempty"`
}

type Word2010StylisticSets struct {
	StyleSets []Word2010StyleSet `xml:"http://schemas.microsoft.com/office/word/2010/wordml styleSet"`
}

type Word2010StyleSet struct {
	ID  int    `xml:"http://schemas.microsoft.com/office/word/2010/wordml id,attr,omitempty"`
	Val string `xml:"http://schemas.microsoft.com/office/word/2010/wordml val,attr,omitempty"`
}

type Word2010TextFill struct {
	SolidFill *DrawingSolidFill        `xml:"solidFill"`
	NoFill    *struct{}                `xml:"noFill"`
	GradFill  *drawingxml.GradientFill `xml:"gradFill"`
}

type Word2010TextOutline struct {
	Width     int64             `xml:"w,attr,omitempty"`
	SolidFill *DrawingSolidFill `xml:"solidFill"`
	NoFill    *struct{}         `xml:"noFill"`
}

type Word2010EffectAlpha struct {
	Val int `xml:"val,attr,omitempty"`
}

type Word2010EffectColorRef struct {
	Val   string               `xml:"val,attr"`
	Alpha *Word2010EffectAlpha `xml:"alpha"`
}

type Word2010EffectSchemeColor struct {
	Val   string               `xml:"val,attr"`
	Alpha *Word2010EffectAlpha `xml:"alpha"`
}

type Word2010GlowEffect struct {
	Rad       int64                      `xml:"rad,attr,omitempty"`
	SrgbClr   *Word2010EffectColorRef    `xml:"srgbClr"`
	SchemeClr *Word2010EffectSchemeColor `xml:"schemeClr"`
}

type Word2010ShadowEffect struct {
	BlurRad   int64                      `xml:"blurRad,attr,omitempty"`
	Dist      int64                      `xml:"dist,attr,omitempty"`
	Dir       int                        `xml:"dir,attr,omitempty"`
	SrgbClr   *Word2010EffectColorRef    `xml:"srgbClr"`
	SchemeClr *Word2010EffectSchemeColor `xml:"schemeClr"`
}

type Word2010SoftEdgeEffect struct {
	Rad int64 `xml:"rad,attr,omitempty"`
}

type Word2010ReflectionEffect struct {
	BlurRad int64 `xml:"blurRad,attr,omitempty"`
	Dist    int64 `xml:"dist,attr,omitempty"`
	Dir     int   `xml:"dir,attr,omitempty"`
	StA     int   `xml:"stA,attr,omitempty"`
}

func (s Word2010StyleSet) Enabled() bool {
	if s.Val == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(s.Val)) {
	case "1", "true", "on":
		return true
	default:
		return false
	}
}

// RunColor represents w:color — text color with optional theme slot reference.
// ECMA-376 Part 1 §17.3.2.6.
type RunColor struct {
	Val        string `xml:"val,attr,omitempty"`
	ThemeColor string `xml:"themeColor,attr,omitempty"`
	ThemeShade string `xml:"themeShade,attr,omitempty"` // hex 00–FF; FF = no change, 00 = black
	ThemeTint  string `xml:"themeTint,attr,omitempty"`  // hex 00–FF; FF = no change, 00 = white
}

// RunLang represents w:lang — language override on a run.
// ECMA-376 Part 1 §17.3.2.20.
type RunLang struct {
	Val      string `xml:"val,attr,omitempty"`
	EastAsia string `xml:"eastAsia,attr,omitempty"`
	Bidi     string `xml:"bidi,attr,omitempty"`
}

// RunFonts represents w:rFonts — per-run font family overrides.
type RunFonts struct {
	ASCII    string `xml:"ascii,attr,omitempty"`
	HAnsi    string `xml:"hAnsi,attr,omitempty"`
	EastAsia string `xml:"eastAsia,attr,omitempty"`
	CS       string `xml:"cs,attr,omitempty"`
	// Theme font slots: resolve to major/minor Latin typeface names via the theme.
	AsciiTheme    string `xml:"asciiTheme,attr,omitempty"`    // e.g. "minorHAnsi"
	HAnsiTheme    string `xml:"hAnsiTheme,attr,omitempty"`    // e.g. "minorHAnsi"
	EastAsiaTheme string `xml:"eastAsiaTheme,attr,omitempty"` // e.g. "minorEastAsia"
	CSTheme       string `xml:"cstheme,attr,omitempty"`       // e.g. "minorBidi"
}

// OnOff represents common empty-or-valued boolean formatting elements.
type OnOff struct {
	Val *BoolVal `xml:"val,attr,omitempty"`
}

// MarshalXMLAttr implements xml.MarshalerAttr for boolean attributes.
// An OnOff with a nil Val is treated as true (same semantics as empty <b/> element).
func (o *OnOff) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	if o == nil {
		return xml.Attr{}, nil
	}
	if o.Val == nil || *o.Val {
		return xml.Attr{Name: name, Value: "true"}, nil
	}
	return xml.Attr{Name: name, Value: "false"}, nil
}

// UnmarshalXMLAttr implements xml.UnmarshalerAttr for boolean attributes.
func (o *OnOff) UnmarshalXMLAttr(attr xml.Attr) error {
	v := attr.Value == "true" || attr.Value == "1" || attr.Value == "on" || attr.Value == "yes"
	bv := BoolVal(v)
	o.Val = &bv
	return nil
}

// Underline represents w:u.
type Underline struct {
	Val   string `xml:"val,attr,omitempty"`
	Color string `xml:"color,attr,omitempty"`
}

// Break represents w:br.
type Break struct {
	Type  string `xml:"type,attr,omitempty"`
	Clear string `xml:"clear,attr,omitempty"`
}

// Table represents w:tbl.
type Table struct {
	XMLName     xml.Name         `xml:"tbl"`
	Properties  *TableProperties `xml:"tblPr"`
	Grid        *TableGrid       `xml:"-"`
	Rows        []*TableRow      `xml:"tr"`
	GridChange  *TableGridChange `xml:"tblGridChange"`
	Diagnostics diag.List        `xml:"-"`
}

// TableProperties represents w:tblPr.
type TableProperties struct {
	Width            *TableMeasure    `xml:"tblW"`
	Alignment        *StringVal       `xml:"jc"`
	Style            *StringVal       `xml:"tblStyle"`
	Borders          *TableBorders    `xml:"tblBorders"`
	Shading          *Shading         `xml:"shd"`
	Look             *TableLook       `xml:"tblLook"`
	Layout           *TableLayout     `xml:"tblLayout"`
	Indent           *TableMeasure    `xml:"tblInd"`
	CellSpacing      *TableMeasure    `xml:"tblCellSpacing"`
	CellMargin       *TableCellMargin `xml:"tblCellMar"`
	FloatProps       *TableFloatProps `xml:"tblpPr"`
	Overlap          *StringVal       `xml:"tblOverlap"`
	Caption          string           `xml:"tblCaption"`
	Description      string           `xml:"tblDescription"`
	BidiVisual       *OnOff           `xml:"bidiVisual"`
	StyleRowBandSize *IntVal          `xml:"tblStyleRowBandSize"`
	StyleColBandSize *IntVal          `xml:"tblStyleColBandSize"`
	// PropertyChange is w:tblPrChange — tracked table property revision metadata.
	PropertyChange *PropertyChangeInfo `xml:"tblPrChange"`
	Diagnostics    diag.List           `xml:"-"`
}

// TableFloatProps holds positioning attributes from w:tblpPr.
// All twips values are raw from the XML attribute (1 twip = 1/1440 inch = 1/20 pt).
type TableFloatProps struct {
	HorzAnchor     string // "text"|"margin"|"page"; default "text"
	VertAnchor     string // "text"|"margin"|"page"; default "text"
	X              int    // w:tblpX — absolute horizontal offset from anchor (twips)
	Y              int    // w:tblpY — absolute vertical offset from anchor (twips)
	XAlign         string // w:tblpXSpec — "left"|"right"|"center"|"inside"|"outside"
	YAlign         string // w:tblpYSpec — "top"|"bottom"|"center"|"inline"
	LeftFromText   int    // w:leftFromText — wrap gap to left (twips)
	RightFromText  int    // w:rightFromText — wrap gap to right (twips)
	TopFromText    int    // w:topFromText — wrap gap above (twips)
	BottomFromText int    // w:bottomFromText — wrap gap below (twips)
}

// MarshalXML emits w:tblpPr as a self-closing element with attributes.
func (f TableFloatProps) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	attrs := []xml.Attr{}
	if f.HorzAnchor != "" {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "horzAnchor"}, Value: f.HorzAnchor})
	}
	if f.VertAnchor != "" {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "vertAnchor"}, Value: f.VertAnchor})
	}
	if f.X != 0 {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "tblpX"}, Value: strconv.Itoa(f.X)})
	}
	if f.Y != 0 {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "tblpY"}, Value: strconv.Itoa(f.Y)})
	}
	if f.XAlign != "" {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "tblpXSpec"}, Value: f.XAlign})
	}
	if f.YAlign != "" {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "tblpYSpec"}, Value: f.YAlign})
	}
	if f.LeftFromText != 0 {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "leftFromText"}, Value: strconv.Itoa(f.LeftFromText)})
	}
	if f.RightFromText != 0 {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "rightFromText"}, Value: strconv.Itoa(f.RightFromText)})
	}
	if f.TopFromText != 0 {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "topFromText"}, Value: strconv.Itoa(f.TopFromText)})
	}
	if f.BottomFromText != 0 {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "bottomFromText"}, Value: strconv.Itoa(f.BottomFromText)})
	}
	start.Attr = attrs
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

// TableLayout represents w:tblLayout.
type TableLayout struct {
	Type string `xml:"type,attr,omitempty"`
}

// TableCellMargin holds the default cell padding for all cells in a table (w:tblCellMar).
type TableCellMargin struct {
	Top    *TableMeasure `xml:"top"`
	Left   *TableMeasure `xml:"left"`
	Bottom *TableMeasure `xml:"bottom"`
	Right  *TableMeasure `xml:"right"`
}

// TableGrid represents w:tblGrid — column width definitions.
type TableGrid struct {
	Columns []int `xml:"-"` // widths in twips
}

// TableMeasure is a width/height measurement with a type qualifier (w:tblW / w:tcW).
type TableMeasure struct {
	Width int    `xml:"w,attr"`
	Type  string `xml:"type,attr,omitempty"` // "dxa" | "pct" | "auto" | "nil"
}

// TableBorders holds the border slots for a table (w:tblBorders),
// including logical start/end alternatives to left/right.
type TableBorders struct {
	Top     *Border `xml:"top"`
	Left    *Border `xml:"left"`
	Start   *Border `xml:"start"`
	Bottom  *Border `xml:"bottom"`
	Right   *Border `xml:"right"`
	End     *Border `xml:"end"`
	InsideH *Border `xml:"insideH"`
	InsideV *Border `xml:"insideV"`
}

// TableCellBorders holds the border slots for a cell (w:tcBorders),
// including the two diagonal directions and logical start/end alternatives.
type TableCellBorders struct {
	Top    *Border `xml:"top"`
	Left   *Border `xml:"left"`
	Start  *Border `xml:"start"`
	Bottom *Border `xml:"bottom"`
	Right  *Border `xml:"right"`
	End    *Border `xml:"end"`
	TL2BR  *Border `xml:"tl2br"`
	TR2BL  *Border `xml:"tr2bl"`
}

// Border represents a single border side (e.g. w:top inside w:tblBorders).
type Border struct {
	Val   string `xml:"val,attr,omitempty"`
	Size  int    `xml:"sz,attr,omitempty"`    // eighths of a point
	Space int    `xml:"space,attr,omitempty"` // points
	Color string `xml:"color,attr,omitempty"`
}

// Shading represents w:shd.
type Shading struct {
	Val            string `xml:"val,attr,omitempty"`
	Color          string `xml:"color,attr,omitempty"`
	Fill           string `xml:"fill,attr,omitempty"`
	ThemeFill      string `xml:"themeFill,attr,omitempty"`
	ThemeFillShade string `xml:"themeFillShade,attr,omitempty"`
	ThemeFillTint  string `xml:"themeFillTint,attr,omitempty"`
}

// TableRow represents w:tr.
type TableRow struct {
	Properties  *TableRowProperties       `xml:"trPr"`
	TblPrEx     *TablePropertiesException `xml:"tblPrEx"`
	Cells       []*TableCell              `xml:"tc"`
	Diagnostics diag.List                 `xml:"-"`
}

// TablePropertiesException represents w:tblPrEx — table property exceptions for a row.
type TablePropertiesException struct {
	Width            *TableMeasure       `xml:"tblW"`
	Alignment        *StringVal          `xml:"jc"`
	Style            *StringVal          `xml:"tblStyle"`
	Borders          *TableBorders       `xml:"tblBorders"`
	Shading          *Shading            `xml:"shd"`
	Look             *TableLook          `xml:"tblLook"`
	Layout           *TableLayout        `xml:"tblLayout"`
	Indent           *TableMeasure       `xml:"tblInd"`
	CellSpacing      *TableMeasure       `xml:"tblCellSpacing"`
	CellMargin       *TableCellMargin    `xml:"tblCellMar"`
	BidiVisual       *OnOff              `xml:"bidiVisual"`
	StyleRowBandSize *IntVal             `xml:"tblStyleRowBandSize"`
	StyleColBandSize *IntVal             `xml:"tblStyleColBandSize"`
	PropertyChange   *PropertyChangeInfo `xml:"tblPrExChange"`
}

// TableRowProperties represents w:trPr.
type TableRowProperties struct {
	Height        *TableRowHeight `xml:"trHeight"`
	IsHeader      *OnOff          `xml:"tblHeader"`
	CantSplit     *OnOff          `xml:"cantSplit"`
	Hidden        *OnOff          `xml:"hidden"`
	GridBefore    *IntVal         `xml:"gridBefore"`
	GridAfter     *IntVal         `xml:"gridAfter"`
	WidthBefore   *TableMeasure   `xml:"wBefore"`
	WidthAfter    *TableMeasure   `xml:"wAfter"`
	Justification *StringVal      `xml:"jc"`
	Conditional   *StringVal      `xml:"cnfStyle"`
	DivID         *IntVal         `xml:"divId"`
	CellSpacing   *TableMeasure   `xml:"tblCellSpacing"`
	// PropertyChange is w:trPrChange — tracked table-row property revision metadata.
	PropertyChange *PropertyChangeInfo `xml:"trPrChange"`
	Diagnostics    diag.List           `xml:"-"`
}

// TableRowHeight represents w:trHeight.
type TableRowHeight struct {
	Val  int    `xml:"val,attr"`
	Rule string `xml:"hRule,attr,omitempty"` // "exact" | "atLeast"
}

// TableCell represents w:tc.
type TableCell struct {
	Properties  *TableCellProperties `xml:"tcPr"`
	Paragraphs  []*Paragraph         `xml:"p"`
	Tables      []*Table             `xml:"tbl"`
	Blocks      []ContentBlock       `xml:"-"`
	Diagnostics diag.List            `xml:"-"`
}

// TableCellProperties represents w:tcPr.
type TableCellProperties struct {
	Width         *TableMeasure      `xml:"tcW"`
	GridSpan      *IntVal            `xml:"gridSpan"`
	VMerge        *VMerge            `xml:"vMerge"`
	HMerge        *VMerge            `xml:"hMerge"`
	Borders       *TableCellBorders  `xml:"tcBorders"`
	Margins       *TableCellMargins  `xml:"tcMar"`
	Shading       *Shading           `xml:"shd"`
	VertAlign     *StringVal         `xml:"vAlign"`
	TextDirection *StringVal         `xml:"textDirection"`
	Conditional   *StringVal         `xml:"cnfStyle"`
	NoWrap        *OnOff             `xml:"noWrap"`
	HideMark      *OnOff             `xml:"hideMark"`
	CellIns       *TrackedCellChange `xml:"cellIns"`
	CellDel       *TrackedCellChange `xml:"cellDel"`
	CellMerge     *TrackedCellChange `xml:"cellMerge"`
	FitText       *OnOff             `xml:"tcFitText"`
	Temporary     *OnOff             `xml:"temporary"`
	// PropertyChange is w:tcPrChange — tracked table-cell property revision metadata.
	PropertyChange *PropertyChangeInfo `xml:"-"`
	Diagnostics    diag.List           `xml:"-"`
}

// VMerge represents w:vMerge — absent val means continuation, "restart" means start of merge.
type VMerge struct {
	Val string `xml:"val,attr,omitempty"`
}

// TrackedCellChange captures w:cellIns, w:cellDel, or w:cellMerge attributes.
type TrackedCellChange struct {
	Author string `xml:"author,attr,omitempty"`
	Date   string `xml:"date,attr,omitempty"`
	ID     int    `xml:"id,attr,omitempty"`
}

// Spacing represents w:spacing.
type Spacing struct {
	Before            *int     `xml:"before,attr,omitempty"`
	After             *int     `xml:"after,attr,omitempty"`
	BeforeLines       *int     `xml:"beforeLines,attr,omitempty"` // hundredths of a line (100 = 1 line)
	AfterLines        *int     `xml:"afterLines,attr,omitempty"`  // hundredths of a line
	Line              *int     `xml:"line,attr,omitempty"`
	LineRule          string   `xml:"lineRule,attr,omitempty"`
	BeforeAutospacing *BoolVal `xml:"beforeAutospacing,attr,omitempty"` // w:beforeAutospacing — ignore w:before, treat as 0
	AfterAutospacing  *BoolVal `xml:"afterAutospacing,attr,omitempty"`  // w:afterAutospacing — ignore w:after, treat as 0
}

// Indentation represents w:ind.
type Indentation struct {
	Left      *int `xml:"left,attr,omitempty"`
	Right     *int `xml:"right,attr,omitempty"`
	FirstLine *int `xml:"firstLine,attr,omitempty"`
	Hanging   *int `xml:"hanging,attr,omitempty"`
}

// Relationships represents word/_rels/document.xml.rels.
type Relationships struct {
	XMLName       xml.Name        `xml:"Relationships"`
	Relationships []*Relationship `xml:"Relationship"`
}

// Relationship is a Word part relationship.
type Relationship struct {
	ID         string `xml:"Id,attr"`
	Type       string `xml:"Type,attr"`
	Target     string `xml:"Target,attr"`
	TargetMode string `xml:"TargetMode,attr,omitempty"`
}

// Theme represents the DrawingML theme part (word/theme/theme1.xml).
// ECMA-376 Part 1 §14.2.7, §20.1.4, §20.1.5.
type Theme struct {
	XMLName     xml.Name              `xml:"theme"`
	Name        string                `xml:"name,attr,omitempty"`
	Elements    *ThemeElements        `xml:"themeElements"`
	ObjectDefs  *ObjectDefaults       `xml:"objectDefaults"`
	ExtraColors *ExtraColorSchemeList `xml:"extraClrSchemeLst"`
	Diagnostics diag.List             `xml:"-"`
}

// ObjectDefaults preserves a:objectDefaults theme defaults for future
// resolver/write-back use.
type ObjectDefaults struct {
	ShapeDefault *ShapeDefault `xml:"spDef"`
	LineDefault  *LineDefault  `xml:"lnDef"`
	TextDefault  *TextDefault  `xml:"txDef"`
}

// ExtraColorSchemeList preserves alternate theme color schemes.
type ExtraColorSchemeList struct {
	Schemes []*ExtraColorScheme `xml:"extraClrScheme"`
}

// ThemeElements holds the color, font, and format schemes.
type ThemeElements struct {
	ColorScheme *ColorScheme  `xml:"clrScheme"`
	FontScheme  *FontScheme   `xml:"fontScheme"`
	FmtScheme   *FormatScheme `xml:"fmtScheme"`
	Diagnostics diag.List     `xml:"-"`
}

// FormatScheme holds the drawing format scheme (fill styles, line styles, effect styles, bg fill styles).
type FormatScheme struct {
	FillStyleList   []ThemeFillEntry   `xml:"fillStyleLst"`
	LineStyleList   []ThemeLineEntry   `xml:"lnStyleLst"`
	EffectStyleList []ThemeEffectEntry `xml:"effectStyleLst"`
	BgFillStyleList []ThemeFillEntry   `xml:"bgFillStyleLst"`
	Diagnostics     diag.List          `xml:"-"`
}

// ThemeFillEntry represents one fill entry inside a style list.
type ThemeFillEntry struct {
	SolidFill *drawingxml.SolidFill    `xml:"solidFill"`
	GradFill  *drawingxml.GradientFill `xml:"gradFill"`
	PattFill  *drawingxml.PatternFill  `xml:"pattFill"`
	NoFill    *struct{}                `xml:"noFill"`
	BlipFill  *drawingxml.BlipFill     `xml:"blipFill"`
}

// ThemeLineEntry represents one line entry inside lnStyleLst.
type ThemeLineEntry struct {
	Line *drawingxml.Line `xml:"ln"`
}

// ThemeEffectEntry represents one effect style entry inside effectStyleLst.
type ThemeEffectEntry struct {
	EffectList *wpsEffectRaw `xml:"effectLst"`
}

// ColorScheme holds the 12 DrawingML color slots (dk1, lt1, dk2, lt2, accent1–6, hlink, folHlink).
// ECMA-376 Part 1 §20.1.4.1.
type ColorScheme struct {
	Dk1         *ThemeColorEntry `xml:"dk1"`
	Lt1         *ThemeColorEntry `xml:"lt1"`
	Dk2         *ThemeColorEntry `xml:"dk2"`
	Lt2         *ThemeColorEntry `xml:"lt2"`
	Accent1     *ThemeColorEntry `xml:"accent1"`
	Accent2     *ThemeColorEntry `xml:"accent2"`
	Accent3     *ThemeColorEntry `xml:"accent3"`
	Accent4     *ThemeColorEntry `xml:"accent4"`
	Accent5     *ThemeColorEntry `xml:"accent5"`
	Accent6     *ThemeColorEntry `xml:"accent6"`
	Hlink       *ThemeColorEntry `xml:"hlink"`
	FolHlink    *ThemeColorEntry `xml:"folHlink"`
	Diagnostics diag.List        `xml:"-"`
}

// ThemeColorEntry contains either a srgbClr or a sysClr child element.
type ThemeColorEntry struct {
	SRGB        *ThemeSRGBColor `xml:"srgbClr"`
	SysClr      *ThemeSysColor  `xml:"sysClr"`
	Diagnostics diag.List       `xml:"-"`
}

// ThemeColorTransforms holds the child transform elements that can appear on both
// a:srgbClr and a:sysClr inside a theme color scheme entry.
type ThemeColorTransforms struct {
	Alpha    *DrawingColorTransform `xml:"alpha"`
	AlphaMod *DrawingColorTransform `xml:"alphaMod"`
	AlphaOff *DrawingColorTransform `xml:"alphaOff"`
	Shade    *DrawingColorTransform `xml:"shade"`
	Tint     *DrawingColorTransform `xml:"tint"`
	Red      *DrawingColorTransform `xml:"red"`
	RedMod   *DrawingColorTransform `xml:"redMod"`
	RedOff   *DrawingColorTransform `xml:"redOff"`
	Green    *DrawingColorTransform `xml:"green"`
	GreenMod *DrawingColorTransform `xml:"greenMod"`
	GreenOff *DrawingColorTransform `xml:"greenOff"`
	Blue     *DrawingColorTransform `xml:"blue"`
	BlueMod  *DrawingColorTransform `xml:"blueMod"`
	BlueOff  *DrawingColorTransform `xml:"blueOff"`
	Hue      *DrawingColorTransform `xml:"hue"`
	Sat      *DrawingColorTransform `xml:"sat"`
	Lum      *DrawingColorTransform `xml:"lum"`
	LumMod   *DrawingColorTransform `xml:"lumMod"`
	LumOff   *DrawingColorTransform `xml:"lumOff"`
	SatMod   *DrawingColorTransform `xml:"satMod"`
	SatOff   *DrawingColorTransform `xml:"satOff"`
	HueMod   *DrawingColorTransform `xml:"hueMod"`
	HueOff   *DrawingColorTransform `xml:"hueOff"`
	Gray     *struct{}              `xml:"gray"`
	Comp     *struct{}              `xml:"comp"`
	Inv      *struct{}              `xml:"inv"`
	Gamma    *struct{}              `xml:"gamma"`
	InvGamma *struct{}              `xml:"invGamma"`
}

// ThemeSRGBColor holds an explicit RGB hex value.
type ThemeSRGBColor struct {
	Val string `xml:"val,attr"`
	ThemeColorTransforms
}

// ThemeSysColor holds a system color reference; LastClr is the resolved RGB hex.
type ThemeSysColor struct {
	LastClr string `xml:"lastClr,attr"`
	ThemeColorTransforms
}

// FontScheme distinguishes major and minor (body) fonts.
// ECMA-376 Part 1 §20.1.4.1.
type FontScheme struct {
	MajorFont   *ThemeFontSet `xml:"majorFont"`
	MinorFont   *ThemeFontSet `xml:"minorFont"`
	Diagnostics diag.List     `xml:"-"`
}

// ThemeFontSet holds the Latin typeface for one font slot.
type ThemeFontSet struct {
	Latin         ThemeLatin               `xml:"latin"`
	EastAsia      []*ThemeTypeface         `xml:"ea"`
	ComplexScript []*ThemeTypeface         `xml:"cs"`
	Fonts         []*ThemeSupplementalFont `xml:"font"`
	Diagnostics   diag.List                `xml:"-"`
}

// ThemeLatin carries the typeface attribute of the <a:latin> element.
type ThemeLatin struct {
	Typeface string `xml:"typeface,attr"`
}

// FontTable represents word/fontTable.xml.
type FontTable struct {
	XMLName     xml.Name  `xml:"fonts"`
	Fonts       []*Font   `xml:"font"`
	Diagnostics diag.List `xml:"-"`
}

// Font describes a font entry in the font table.
type Font struct {
	Name            string        `xml:"name,attr"`
	AltName         *StringVal    `xml:"altName"`
	Family          *StringVal    `xml:"family"`
	Pitch           *StringVal    `xml:"pitch"`
	Charset         *StringVal    `xml:"charset"`
	Panose1         *StringVal    `xml:"panose1"`
	NotTrueType     *OnOff        `xml:"notTrueType"`
	EmbedRegular    *FontEmbedRef `xml:"embedRegular"`
	EmbedBold       *FontEmbedRef `xml:"embedBold"`
	EmbedItalic     *FontEmbedRef `xml:"embedItalic"`
	EmbedBoldItalic *FontEmbedRef `xml:"embedBoldItalic"`
}

// FontEmbedRef points to an embedded font part via a relationship ID.
type FontEmbedRef struct {
	RelID   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	FontKey string `xml:"fontKey,attr,omitempty"`
}

type EmbeddedFontData = fontdata.EmbeddedFontData
type FontPart = fontdata.FontPart

// FootnotesPart represents word/footnotes.xml.
// ECMA-376 Part 1 §17.11.14.
type FootnotesPart struct {
	XMLName     xml.Name  `xml:"footnotes"`
	Notes       []*Note   `xml:"footnote"`
	Diagnostics diag.List `xml:"-"`
}

// EndnotesPart represents word/endnotes.xml.
// ECMA-376 Part 1 §17.11.5.
type EndnotesPart struct {
	XMLName     xml.Name  `xml:"endnotes"`
	Notes       []*Note   `xml:"endnote"`
	Diagnostics diag.List `xml:"-"`
}

// Note is a single footnote or endnote entry. The Type field is "normal" for content notes;
// "separator" and "continuationSeparator" entries are skipped during rendering.
type Note struct {
	ID          int            `xml:"id,attr"`
	Type        string         `xml:"type,attr,omitempty"`
	Paragraphs  []*Paragraph   `xml:"p"`
	Tables      []*Table       `xml:"tbl"`
	Blocks      []ContentBlock `xml:"-"`
	Diagnostics diag.List      `xml:"-"`
}

// Styles represents word/styles.xml.
type Styles struct {
	XMLName      xml.Name      `xml:"styles"`
	DocDefaults  *DocDefaults  `xml:"docDefaults"`
	LatentStyles *LatentStyles `xml:"latentStyles"`
	Styles       []*Style      `xml:"style"`
	Unknown      []RawElement  `xml:",any"`
	Diagnostics  diag.List     `xml:"-"`
}

// LatentStyles represents w:latentStyles.
type LatentStyles struct {
	Exceptions []LsdException `xml:"lsdException"`
}

// LsdException represents w:lsdException.
type LsdException struct {
	Name           string `xml:"name,attr"`
	Locked         bool   `xml:"locked,attr,omitempty"`
	UIPriority     int    `xml:"uiPriority,attr,omitempty"`
	SemiHidden     bool   `xml:"semiHidden,attr,omitempty"`
	UnhideWhenUsed bool   `xml:"unhideWhenUsed,attr,omitempty"`
	QFormat        bool   `xml:"qFormat,attr,omitempty"`
}

// DocDefaults represents w:docDefaults.
type DocDefaults struct {
	RunDefaults  *RunDefaults       `xml:"rPrDefault"`
	ParaDefaults *ParagraphDefaults `xml:"pPrDefault"`
	Unknown      []RawElement       `xml:",any"`
	Diagnostics  diag.List          `xml:"-"`
}

// RunDefaults represents w:rPrDefault.
type RunDefaults struct {
	Properties  *RunProperties `xml:"rPr"`
	Unknown     []RawElement   `xml:",any"`
	Diagnostics diag.List      `xml:"-"`
}

// ParagraphDefaults represents w:pPrDefault.
type ParagraphDefaults struct {
	Properties  *ParagraphProperties `xml:"pPr"`
	Unknown     []RawElement         `xml:",any"`
	Diagnostics diag.List            `xml:"-"`
}

// Style represents w:style.
type Style struct {
	Type        string               `xml:"type,attr"`
	StyleID     string               `xml:"styleId,attr"`
	Default     *BoolVal             `xml:"default,attr,omitempty"`
	Name        *StringVal           `xml:"name"`
	AltName     *StringVal           `xml:"altName"`
	Aliases     *StringVal           `xml:"aliases"`
	BasedOn     *StringVal           `xml:"basedOn"`
	Next        *StringVal           `xml:"next"`
	Link        *StringVal           `xml:"link"`
	UIPriority  *IntVal              `xml:"uiPriority"`
	RSID        string               `xml:"rsid"`
	Unhide      *OnOff               `xml:"unhideWhenUsed"`
	QuickFormat *OnOff               `xml:"qFormat"`
	SemiHidden  *OnOff               `xml:"semiHidden"`
	AutoRedef   *OnOff               `xml:"autoRedefine"`
	ParaProps   *ParagraphProperties `xml:"pPr"`
	RunProps    *RunProperties       `xml:"rPr"`
	TableProps  *TableProperties     `xml:"tblPr"`
	CellProps   *TableCellProperties `xml:"tcPr"`
	TableStyles []*TableStyleProps   `xml:"tblStylePr"`
	Unknown     []RawElement         `xml:",any"`
	Diagnostics diag.List            `xml:"-"`
}

// Numbering represents word/numbering.xml.
type Numbering struct {
	XMLName       xml.Name        `xml:"numbering"`
	NumPicBullets []*NumPicBullet `xml:"numPicBullet"`
	AbstractNums  []*AbstractNum  `xml:"abstractNum"`
	Nums          []*Num          `xml:"num"`
	Diagnostics   diag.List       `xml:"-"`
}

// NumPicBullet represents w:numPicBullet, a picture-bullet definition.
type NumPicBullet struct {
	ID          int        `xml:"numPicBulletId,attr"`
	Pict        *VMLPict   `xml:"pict"`
	VMLRelID    string     `xml:"-"`
	WidthPt     float64    `xml:"-"`
	HeightPt    float64    `xml:"-"`
	Media       *MediaPart `xml:"-"`
	Diagnostics diag.List  `xml:"-"`
}

// VMLPict represents w:pict content used by numbering picture bullets.
type VMLPict struct {
	Shape *VMLShape `xml:"shape"`
}

// AbstractNum represents w:abstractNum.
type AbstractNum struct {
	ID           int        `xml:"abstractNumId,attr"`
	NumStyleLink *StringVal `xml:"numStyleLink"`
	Levels       []*Lvl     `xml:"lvl"`
	Diagnostics  diag.List  `xml:"-"`
}

// Lvl represents w:lvl.
type Lvl struct {
	Level           int            `xml:"ilvl,attr"`
	Start           *IntVal        `xml:"start"`
	Format          *StringVal     `xml:"numFmt"`
	Text            *StringVal     `xml:"lvlText"`
	PictureBulletID *IntVal        `xml:"lvlPicBulletId"`
	PStyle          *StringVal     `xml:"pStyle"`
	Justify         *StringVal     `xml:"lvlJc"`
	ParaProps       *LvlPPr        `xml:"pPr"`
	RunProps        *RunProperties `xml:"rPr"`
	Diagnostics     diag.List      `xml:"-"`
}

// LvlPPr is the paragraph properties embedded in a w:lvl definition.
type LvlPPr struct {
	Ind         *LvlInd   `xml:"ind"`
	Diagnostics diag.List `xml:"-"`
}

// LvlInd captures w:ind within a numbering level.
type LvlInd struct {
	Left      int `xml:"left,attr"`
	Hanging   int `xml:"hanging,attr"`
	FirstLine int `xml:"firstLine,attr"`
}

// Num represents w:num.
type Num struct {
	ID            int            `xml:"numId,attr"`
	AbstractNumID *IntVal        `xml:"abstractNumId"`
	Overrides     []*LvlOverride `xml:"lvlOverride"`
	Diagnostics   diag.List      `xml:"-"`
}

// LvlOverride represents a per-num override of an abstract numbering level.
// ECMA-376 Part 1 §17.9.8.
type LvlOverride struct {
	Level       int       `xml:"ilvl,attr"`
	Start       *IntVal   `xml:"startOverride"`
	Lvl         *Lvl      `xml:"lvl"`
	Diagnostics diag.List `xml:"-"`
}

// Header represents word/header*.xml.
type Header struct {
	XMLName     xml.Name       `xml:"hdr"`
	Paragraphs  []*Paragraph   `xml:"p"`
	Tables      []*Table       `xml:"tbl"`
	Blocks      []ContentBlock `xml:"-"`
	Diagnostics diag.List      `xml:"-"`
}

// Footer represents word/footer*.xml.
type Footer struct {
	XMLName     xml.Name       `xml:"ftr"`
	Paragraphs  []*Paragraph   `xml:"p"`
	Tables      []*Table       `xml:"tbl"`
	Blocks      []ContentBlock `xml:"-"`
	Diagnostics diag.List      `xml:"-"`
}

// GlossaryDocument represents word/glossary/document.xml.
type GlossaryDocument struct {
	XMLName  xml.Name         `xml:"glossaryDocument"`
	DocParts []*GlossaryEntry `xml:"docParts>docPart"`
}

// GlossaryEntry represents a single building block entry inside the glossary.
type GlossaryEntry struct {
	Name           *StringVal   `xml:"docPartPr>name"`
	Gallery        *StringVal   `xml:"docPartPr>category>name"`    // category name (historical field name)
	DocPartGallery *StringVal   `xml:"docPartPr>category>gallery"` // true gallery type, e.g. "autoText"
	Behavior       *StringVal   `xml:"docPartPr>behavior"`
	Body           *DocPartBody `xml:"docPartBody"`
}

// DocPartBody represents the ordered block content of a glossary entry body.
type DocPartBody struct {
	Paragraphs  []*Paragraph   `xml:"p"`
	Tables      []*Table       `xml:"tbl"`
	Blocks      []ContentBlock `xml:"-"`
	Diagnostics diag.List      `xml:"-"`
}

// Settings represents word/settings.xml.
type Settings struct {
	XMLName                             xml.Name                 `xml:"settings"`
	FootnoteProps                       *NoteProperties          `xml:"footnotePr"`
	EndnoteProps                        *NoteProperties          `xml:"endnotePr"`
	DefaultTabStop                      *IntVal                  `xml:"defaultTabStop"`
	TrackRevisions                      *OnOff                   `xml:"trackRevisions"`
	Zoom                                *ZoomSetting             `xml:"zoom"`
	Compat                              *CompatSettings          `xml:"compat"`
	DocumentProtection                  *DocumentProtection      `xml:"documentProtection"`
	CharacterSpacingControl             *StringVal               `xml:"characterSpacingControl"`
	View                                *StringVal               `xml:"view"`
	AutoHyphenation                     *OnOff                   `xml:"autoHyphenation"`
	ConsecutiveHyphenLimit              *IntVal                  `xml:"consecutiveHyphenLimit"`
	HyphenationZone                     *IntVal                  `xml:"hyphenationZone"`
	DoNotHyphenateCaps                  *OnOff                   `xml:"doNotHyphenateCaps"`
	EvenAndOddHeaders                   *OnOff                   `xml:"evenAndOddHeaders"`
	UpdateFields                        *OnOff                   `xml:"updateFields"`
	ThemeFontLang                       *ThemeFontLang           `xml:"themeFontLang"`
	ProofState                          *ProofState              `xml:"proofState"`
	MailMerge                           *MailMergeSettings       `xml:"mailMerge"`
	DocumentType                        *StringVal               `xml:"documentType"`
	MirrorMargins                       *OnOff                   `xml:"mirrorMargins"`
	DoNotDisplayPageBoundaries          *OnOff                   `xml:"doNotDisplayPageBoundaries"`
	BordersDoNotSurroundFooter          *OnOff                   `xml:"bordersDoNotSurroundFooter"`
	BordersDoNotSurroundHeader          *OnOff                   `xml:"bordersDoNotSurroundHeader"`
	DoNotTrackFormatting                *OnOff                   `xml:"doNotTrackFormatting"`
	DoNotTrackMoves                     *OnOff                   `xml:"doNotTrackMoves"`
	ActiveWritingStyles                 []ActiveWritingStyle     `xml:"activeWritingStyle"`
	BookFoldRevPrinting                 *OnOff                   `xml:"bookFoldRevPrinting"`
	BookFoldPrintingSheets              *IntVal                  `xml:"bookFoldPrintingSheets"`
	DecimalSymbol                       *StringVal               `xml:"decimalSymbol"`
	ListSeparator                       *StringVal               `xml:"listSeparator"`
	ShowXMLTags                         *OnOff                   `xml:"showXMLTags"`
	DefaultTableStyle                   *StringVal               `xml:"defaultTableStyle"`
	DisplayHorizontalDrawingGrid        *IntVal                  `xml:"displayHorizontalDrawingGridEvery"`
	DisplayVerticalDrawingGrid          *IntVal                  `xml:"displayVerticalDrawingGridEvery"`
	DoNotUseMarginsForDrawingGridOrigin *OnOff                   `xml:"doNotUseMarginsForDrawingGridOrigin"`
	DrawingGridHorizontalSpacing        *IntVal                  `xml:"drawingGridHorizontalSpacing"`
	DrawingGridVerticalSpacing          *IntVal                  `xml:"drawingGridVerticalSpacing"`
	DrawingGridHorizontalOrigin         *IntVal                  `xml:"drawingGridHorizontalOrigin"`
	DrawingGridVerticalOrigin           *IntVal                  `xml:"drawingGridVerticalOrigin"`
	RemovePersonalInformation           *OnOff                   `xml:"removePersonalInformation"`
	RemoveDateAndTime                   *OnOff                   `xml:"removeDateAndTime"`
	DisplayBackgroundShape              *OnOff                   `xml:"displayBackgroundShape"`
	PrintPostScriptOverText             *OnOff                   `xml:"printPostScriptOverText"`
	PrintFractionalCharacterWidth       *OnOff                   `xml:"printFractionalCharacterWidth"`
	PrintFormsData                      *OnOff                   `xml:"printFormsData"`
	EmbedTrueTypeFonts                  *OnOff                   `xml:"embedTrueTypeFonts"`
	EmbedSystemFonts                    *OnOff                   `xml:"embedSystemFonts"`
	SaveSubsetFonts                     *OnOff                   `xml:"saveSubsetFonts"`
	SaveFormsData                       *OnOff                   `xml:"saveFormsData"`
	AlignBordersAndEdges                *OnOff                   `xml:"alignBordersAndEdges"`
	GutterAtTop                         *OnOff                   `xml:"gutterAtTop"`
	HideSpellingErrors                  *OnOff                   `xml:"hideSpellingErrors"`
	HideGrammaticalErrors               *OnOff                   `xml:"hideGrammaticalErrors"`
	FormsDesign                         *OnOff                   `xml:"formsDesign"`
	LinkStyles                          *OnOff                   `xml:"linkStyles"`
	RevisionView                        *OnOff                   `xml:"revisionView"`
	AutoFormatOverride                  *OnOff                   `xml:"autoFormatOverride"`
	StyleLockTheme                      *OnOff                   `xml:"styleLockTheme"`
	StyleLockQFSet                      *OnOff                   `xml:"styleLockQFSet"`
	ShowEnvelope                        *OnOff                   `xml:"showEnvelope"`
	PrintTwoOnOne                       *OnOff                   `xml:"printTwoOnOne"`
	StrictFirstAndLastChars             *OnOff                   `xml:"strictFirstAndLastChars"`
	DoNotShadeFormData                  *OnOff                   `xml:"doNotShadeFormData"`
	NoPunctuationKerning                *OnOff                   `xml:"noPunctuationKerning"`
	SavePreviewPicture                  *OnOff                   `xml:"savePreviewPicture"`
	DoNotValidateAgainstSchema          *OnOff                   `xml:"doNotValidateAgainstSchema"`
	SaveInvalidXml                      *OnOff                   `xml:"saveInvalidXml"`
	IgnoreMixedContent                  *OnOff                   `xml:"ignoreMixedContent"`
	AlwaysShowPlaceholderText           *OnOff                   `xml:"alwaysShowPlaceholderText"`
	DoNotDemarcateInvalidXml            *OnOff                   `xml:"doNotDemarcateInvalidXml"`
	SaveXmlDataOnly                     *OnOff                   `xml:"saveXmlDataOnly"`
	UseXSLTWhenSaving                   *OnOff                   `xml:"useXSLTWhenSaving"`
	AlwaysMergeEmptyNamespace           *OnOff                   `xml:"alwaysMergeEmptyNamespace"`
	DoNotIncludeSubdocsInStats          *OnOff                   `xml:"doNotIncludeSubdocsInStats"`
	DoNotAutoCompressPictures           *OnOff                   `xml:"doNotAutoCompressPictures"`
	DoNotEmbedSmartTags                 *OnOff                   `xml:"doNotEmbedSmartTags"`
	BookFoldPrinting                    *OnOff                   `xml:"bookFoldPrinting"`
	Active                              *OnOff                   `xml:"active"`
	BodyDiv                             *OnOff                   `xml:"bodyDiv"`
	CachedColBalance                    *OnOff                   `xml:"cachedColBalance"`
	DoNotSnapToGridInCell               *OnOff                   `xml:"doNotSnapToGridInCell"`
	DoNotSuppressIndentation            *OnOff                   `xml:"doNotSuppressIndentation"`
	DoNotSuppressParagraphBorders       *OnOff                   `xml:"doNotSuppressParagraphBorders"`
	DoNotUseEastAsianBreakRules         *OnOff                   `xml:"doNotUseEastAsianBreakRules"`
	DoNotUseHTMLParagraphAutoSpacing    *OnOff                   `xml:"doNotUseHTMLParagraphAutoSpacing"`
	DoNotWrapTextWithPunct              *OnOff                   `xml:"doNotWrapTextWithPunct"`
	GrowAutofit                         *OnOff                   `xml:"growAutofit"`
	LvlRestart                          *IntVal                  `xml:"lvlRestart"`
	NoLineBreaksAfter                   *Kinsoku                 `xml:"noLineBreaksAfter"`
	NoLineBreaksBefore                  *Kinsoku                 `xml:"noLineBreaksBefore"`
	NumIDMacAtCleanup                   *IntVal                  `xml:"numIdMacAtCleanup"`
	SaveSmartTagsAsXml                  *OnOff                   `xml:"saveSmartTagsAsXml"`
	SelectFldWithFirstOrLastChar        *OnOff                   `xml:"selectFldWithFirstOrLastChar"`
	SplitPgBreakAndParaMark             *OnOff                   `xml:"splitPgBreakAndParaMark"`
	StyleLink                           *StringVal               `xml:"styleLink"`
	SuppressBottomSpacing               *OnOff                   `xml:"suppressBottomSpacing"`
	SuppressOverlap                     *OnOff                   `xml:"suppressOverlap"`
	SuppressSpacingAtTopOfPage          *OnOff                   `xml:"suppressSpacingAtTopOfPage"`
	SuppressTopSpacing                  *OnOff                   `xml:"suppressTopSpacing"`
	SuppressTopSpacingWP                *OnOff                   `xml:"suppressTopSpacingWP"`
	SwapBordersFacingPages              *OnOff                   `xml:"swapBordersFacingPages"`
	TextboxTightWrap                    *OnOff                   `xml:"textboxTightWrap"`
	UnderlineTabInNumList               *OnOff                   `xml:"underlineTabInNumList"`
	UseAnsiKerningPairs                 *OnOff                   `xml:"useAnsiKerningPairs"`
	UsePrinterMetrics                   *OnOff                   `xml:"usePrinterMetrics"`
	UseSingleBorderforContiguousCells   *OnOff                   `xml:"useSingleBorderforContiguousCells"`
	ForceUpgrade                        *struct{}                `xml:"forceUpgrade"`
	AttachedTemplate                    *ContentPartRef          `xml:"attachedTemplate"`
	StylePaneFormatFilter               *StylePaneFilter         `xml:"stylePaneFormatFilter"`
	StylePaneSortMethod                 *StringVal               `xml:"stylePaneSortMethod"`
	SaveThroughXslt                     *SaveThroughXslt         `xml:"saveThroughXslt"`
	DocVars                             []DocVar                 `xml:"docVars>docVar"`
	AttachedSchemas                     []StringVal              `xml:"attachedSchema"`
	ColorSchemeMapping                  *ColorSchemeMapping      `xml:"clrSchemeMapping"`
	Captions                            *Captions                `xml:"captions"`
	ReadModeInkLockDown                 *ReadModeInkLockDown     `xml:"readModeInkLockDown"`
	SmartTagTypes                       []SmartTagType           `xml:"smartTagType"`
	HdrShapeDefaults                    *ShapeDefaults           `xml:"hdrShapeDefaults"`
	ShapeDefaults                       *ShapeDefaults           `xml:"shapeDefaults"`
	ClickAndTypeStyle                   *StringVal               `xml:"clickAndTypeStyle"`
	SummaryLength                       *IntVal                  `xml:"summaryLength"`
	WriteProtection                     *WriteProtectionSettings `xml:"writeProtection"`
	RSIDs                               *RSIDSettings            `xml:"rsids"`
	Unknown                             []RawElement             `xml:",any"`
}

// DocumentProtection captures w:documentProtection settings.
type DocumentProtection struct {
	Edit           string `xml:"edit,attr,omitempty"`
	Enforcement    bool   `xml:"enforcement,attr,omitempty"`
	FormatSHA1     string `xml:"formatting,attr,omitempty"`
	HashValue      string `xml:"hashValue,attr,omitempty"`
	SaltValue      string `xml:"saltValue,attr,omitempty"`
	SpinCount      int    `xml:"spinCount,attr,omitempty"`
	AlgorithmSID   int    `xml:"cryptAlgorithmSid,attr,omitempty"`
	CryptProvType  string `xml:"cryptProviderType,attr,omitempty"`
	CryptAlgorithm string `xml:"cryptAlgorithmClass,attr,omitempty"`
}

// ThemeFontLang captures w:themeFontLang settings.
type ThemeFontLang struct {
	Val      string `xml:"val,attr,omitempty"`
	EastAsia string `xml:"eastAsia,attr,omitempty"`
	Bidi     string `xml:"bidi,attr,omitempty"`
}

// ProofState captures w:proofState settings.
type ProofState struct {
	Spelling string `xml:"spelling,attr,omitempty"`
	Grammar  string `xml:"grammar,attr,omitempty"`
}

// MailMergeSettings captures w:mailMerge settings.
type MailMergeSettings struct {
	MainDocumentType        string      `xml:"mainDocumentType,attr,omitempty"`
	LinkToQuery             bool        `xml:"linkToQuery,attr,omitempty"`
	DataType                string      `xml:"dataType,attr,omitempty"`
	ConnectString           string      `xml:"connectString,attr,omitempty"`
	Query                   string      `xml:"query,attr,omitempty"`
	DataSource              string      `xml:"dataSource,attr,omitempty"`
	HeaderSource            string      `xml:"headerSource,attr,omitempty"`
	Destination             string      `xml:"destination,attr,omitempty"`
	MainDocumentTypeVal     *StringVal  `xml:"mainDocumentType"`
	LinkToQueryVal          *OnOff      `xml:"linkToQuery"`
	DataTypeVal             *StringVal  `xml:"dataType"`
	ConnectStringVal        *StringVal  `xml:"connectString"`
	QueryVal                *StringVal  `xml:"query"`
	DataSourceVal           *StringVal  `xml:"dataSource"`
	HeaderSourceVal         *StringVal  `xml:"headerSource"`
	DestinationVal          *StringVal  `xml:"destination"`
	ActiveRecord            *StringVal  `xml:"activeRecord"`
	AddressFieldName        *StringVal  `xml:"addressFieldName"`
	CheckErrors             *StringVal  `xml:"checkErrors"`
	DoNotSuppressBlankLines *OnOff      `xml:"doNotSuppressBlankLines"`
	DynamicAddress          *OnOff      `xml:"dynamicAddress"`
	EntryMacro              *StringVal  `xml:"entryMacro"`
	ExitMacro               *StringVal  `xml:"exitMacro"`
	FHdr                    *OnOff      `xml:"fHdr"`
	LinkedToFile            *OnOff      `xml:"linkedToFile"`
	MailAsAttachment        *OnOff      `xml:"mailAsAttachment"`
	MailSubject             *StringVal  `xml:"mailSubject"`
	MappedName              *StringVal  `xml:"mappedName"`
	ODSO                    *ODSO       `xml:"odso"`
	SourceFileName          *StringVal  `xml:"sourceFileName"`
	StoreMappedDataAs       *StringVal  `xml:"storeMappedDataAs"`
	SubDoc                  *StringVal  `xml:"subDoc"`
	Types                   *MMTypes    `xml:"types"`
	UDL                     *StringVal  `xml:"udl"`
	ViewMergedData          *OnOff      `xml:"viewMergedData"`
	Recipients              *Recipients `xml:"recipients"`
	Personal                *OnOff      `xml:"personal"`
	PersonalCompose         *OnOff      `xml:"personalCompose"`
	PersonalReply           *OnOff      `xml:"personalReply"`
}

// Recipients captures w:recipients within w:mailMerge.
type Recipients struct {
	RecipientData []RecipientData `xml:"recipientData"`
}

// ODSO captures w:odso (Office Data Source Object) settings.
type ODSO struct {
	ColDelim       *IntVal         `xml:"colDelim"`
	DynamicAddress *OnOff          `xml:"dynamicAddress"`
	FHdr           *OnOff          `xml:"fHdr"`
	SourceFileName *StringVal      `xml:"sourceFileName"`
	UDL            *StringVal      `xml:"udl"`
	FieldMapData   []FieldMapData  `xml:"fieldMapData"`
	RecipientData  []RecipientData `xml:"recipientData"`
}

// FieldMapData captures w:fieldMapData within w:odso.
type FieldMapData struct {
	MappedName string `xml:"mappedName,attr,omitempty"`
	Name       string `xml:"name,attr,omitempty"`
	Lang       string `xml:"lang,attr,omitempty"`
	Val        string `xml:"val,attr,omitempty"`
	Column     int    `xml:"col,attr,omitempty"`
}

// RecipientData captures w:recipientData within w:odso.
type RecipientData struct {
	Active *OnOff `xml:"active"`
}

// MMTypes captures w:types within w:mailMerge.
type MMTypes struct {
	Types []MMType `xml:"type"`
}

// MMType captures w:type within w:types.
type MMType struct {
	Val string `xml:"val,attr"`
}

// ActiveWritingStyle captures w:activeWritingStyle settings.
type ActiveWritingStyle struct {
	Lang       string `xml:"lang,attr,omitempty"`
	VendorID   string `xml:"vendorID,attr,omitempty"`
	DLLVersion int    `xml:"dllVersion,attr,omitempty"`
	NLCheck    bool   `xml:"nlCheck,attr,omitempty"`
	CheckStyle bool   `xml:"checkStyle,attr,omitempty"`
	AppName    string `xml:"appName,attr,omitempty"`
}

// NoteProperties captures note placement settings from w:footnotePr/w:endnotePr.
type NoteProperties struct {
	Position *StringVal `xml:"pos"`
	Format   *StringVal `xml:"numFmt"`
	Start    *IntVal    `xml:"numStart"`
	Restart  *StringVal `xml:"numRestart"`
}

// ZoomSetting captures w:settings/w:zoom.
type ZoomSetting struct {
	Percent int `xml:"percent,attr"`
}

// WriteProtectionSettings captures w:writeProtection settings.
type WriteProtectionSettings struct {
	Recommended   bool   `xml:"recommended,attr,omitempty"`
	Locked        *OnOff `xml:"locked"`
	HashValue     string `xml:"hashValue,attr,omitempty"`
	SaltValue     string `xml:"saltValue,attr,omitempty"`
	SpinCount     int    `xml:"spinCount,attr,omitempty"`
	AlgorithmSID  int    `xml:"cryptAlgorithmSid,attr,omitempty"`
	CryptProvType string `xml:"cryptProviderType,attr,omitempty"`
}

// RSIDSettings captures w:rsids settings.
type RSIDSettings struct {
	RSIDRoot *StringVal  `xml:"rsidRoot"`
	RSIDs    []StringVal `xml:"rsid"`
}

// DocVar represents a single document variable (w:docVar).
type DocVar struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"val,attr"`
}

// StylePaneFilter represents w:stylePaneFormatFilter.
type StylePaneFilter struct {
	Val string `xml:"val,attr,omitempty"`
}

// SaveThroughXslt represents w:saveThroughXslt.
type SaveThroughXslt struct {
	RelID      string `xml:"id,attr,omitempty"`
	SolutionID string `xml:"solutionID,attr,omitempty"`
}

// ColorSchemeMapping represents w:clrSchemeMapping.
type ColorSchemeMapping struct {
	BG1               string `xml:"bg1,attr,omitempty"`
	T1                string `xml:"t1,attr,omitempty"`
	BG2               string `xml:"bg2,attr,omitempty"`
	T2                string `xml:"t2,attr,omitempty"`
	Accent1           string `xml:"accent1,attr,omitempty"`
	Accent2           string `xml:"accent2,attr,omitempty"`
	Accent3           string `xml:"accent3,attr,omitempty"`
	Accent4           string `xml:"accent4,attr,omitempty"`
	Accent5           string `xml:"accent5,attr,omitempty"`
	Accent6           string `xml:"accent6,attr,omitempty"`
	Hyperlink         string `xml:"hyperlink,attr,omitempty"`
	FollowedHyperlink string `xml:"followedHyperlink,attr,omitempty"`
}

// ReadModeInkLockDown represents w:readModeInkLockDown.
type ReadModeInkLockDown struct {
	ActualPage bool `xml:"actualPg,attr"`
	Width      int  `xml:"w,attr"`
	Height     int  `xml:"h,attr"`
	FontSize   int  `xml:"fontSz,attr"`
}

// Caption represents a single w:caption element.
type Caption struct {
	Name      string `xml:"name,attr"`
	Position  string `xml:"pos,attr,omitempty"`
	ChapNum   bool   `xml:"chapNum,attr,omitempty"`
	Heading   int    `xml:"heading,attr,omitempty"`
	NoLabel   bool   `xml:"noLabel,attr,omitempty"`
	NumFormat string `xml:"numFmt,attr,omitempty"`
	Separator string `xml:"sep,attr,omitempty"`
}

// AutoCaption represents a single w:autoCaption element.
type AutoCaption struct {
	Name    string `xml:"name,attr"`
	Caption string `xml:"caption,attr"`
}

// Captions represents w:captions.
type Captions struct {
	Captions     []Caption     `xml:"caption"`
	AutoCaptions []AutoCaption `xml:"autoCaptions>autoCaption"`
}

// SmartTagType represents w:smartTagType.
type SmartTagType struct {
	NamespaceURI string `xml:"namespaceuri,attr,omitempty"`
	Name         string `xml:"name,attr,omitempty"`
	URL          string `xml:"url,attr,omitempty"`
}

// ShapeDefaults represents w:shapeDefaults or w:hdrShapeDefaults.
type ShapeDefaults struct {
	RawXML   string `xml:",innerxml"`
	NoBorder *OnOff `xml:"noBorder"`
}

// CompatSettings captures w:settings/w:compat compatibility flags.
type CompatSettings struct {
	Settings []CompatSetting `xml:"compatSetting"`
}

// UnmarshalXML decodes w:compat and records each known compatSetting name
// as an explicit manual binding for spec-coverage tracking.
func (cs *CompatSettings) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*cs = CompatSettings{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "compatSetting":
				var s CompatSetting
				if err := dec.DecodeElement(&s, &current); err != nil {
					return err
				}
				cs.Settings = append(cs.Settings, s)
			case "noTabHangInd", "noLeading", "spaceForUL", "noColumnBalance",
				"balanceSingleByteDoubleByteWidth", "noExtraLineSpacing",
				"doNotLeaveBackslashAlone", "ulTrailSpace", "doNotExpandShiftReturn",
				"spacingInWholePoints", "lineWrapLikeWord6", "printBodyTextBeforeHeader",
				"printColBlack", "wpJustification", "noTabIndentForHangingIndent",
				"noSpaceRaiseLower", "suppressSpBfAfterPgBrk", "suppressLineNumbers",
				"noSpaceForUL", "wrapTrailSpaces", "truncateFontHeightsLikeWP6",
				"subFontBySize", "useWord97LineBreakRules", "doNotBreakWrappedTables",
				"doNotSnapToGridInTable", "doNotApplyIndentHanging",
				"doNotAutoFitToWindow", "allowPNG", "useFELayout",
				"useNormalStyleForList", "doNotUseIndentAsNumberingTabStop",
				"useAltKinsokuLineBreakRules", "allowHeightTableAutoFit",
				"useWord2002TableStyleRules",
				"adjustLineHeightInTable", "alignTablesRowByRow", "allowSpaceOfSameStyleInTable",
				"applyBreakingRules", "autoSpaceLikeWord95", "autofitToFirstFixedWidthCell",
				"displayHangulFixedWidth", "doNotAutofitConstrainedTables",
				"doNotBreakConstrainedForcedTable", "doNotVertAlignCellWithSp",
				"doNotVertAlignInTxbx", "flatBorders", "footnoteLayoutLikeWW8",
				"forgetLastTabAlignment", "layoutRawTableWidth", "layoutTableRowsApart",
				"legacy", "mwSmallCaps", "noResizeAllowed", "shapeLayoutLikeWW8",
				"showBreaksInFrames", "convMailMergeEsc", "wpSpaceWidth":
				// Capture direct child boolean elements of w:compat.
				var s struct {
					Val *string `xml:"val,attr"`
				}
				if err := dec.DecodeElement(&s, &current); err != nil {
					return err
				}
				val := "1"
				if s.Val != nil {
					v := strings.ToLower(*s.Val)
					if v == "false" || v == "0" || v == "off" {
						val = "0"
					}
				}
				cs.Settings = append(cs.Settings, CompatSetting{
					Name:  current.Name.Local,
					URI:   "http://schemas.microsoft.com/office/word",
					Value: val,
				})
			default:
				// Unknown compat child — capture generically.
				var s struct {
					Val *string `xml:"val,attr"`
				}
				if err := dec.DecodeElement(&s, &current); err != nil {
					return err
				}
				val := "1"
				if s.Val != nil {
					v := strings.ToLower(*s.Val)
					if v == "false" || v == "0" || v == "off" {
						val = "0"
					}
				}
				cs.Settings = append(cs.Settings, CompatSetting{
					Name:  current.Name.Local,
					URI:   "http://schemas.microsoft.com/office/word",
					Value: val,
				})
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// CompatSetting represents a single w:compatSetting element.
type CompatSetting struct {
	Name  string `xml:"name,attr"`
	URI   string `xml:"uri,attr"`
	Value string `xml:"val,attr"`
}

// SectionProperties represents w:sectPr.
type SectionProperties struct {
	HeaderRefs    []HeaderFooterReference `xml:"headerReference"`
	FooterRefs    []HeaderFooterReference `xml:"footerReference"`
	PageSize      *PageSize               `xml:"pgSz"`
	PageMargin    *PageMargin             `xml:"pgMar"`
	PaperSrc      *PaperSource            `xml:"paperSrc"`
	Columns       *Columns                `xml:"cols"`
	DocGrid       *DocGrid                `xml:"docGrid"`
	PageNumType   *PageNumberType         `xml:"pgNumType"`
	LnNumType     *LineNumberType         `xml:"lnNumType"`
	Type          *StringVal              `xml:"type"`
	TextDirection *StringVal              `xml:"textDirection"`
	// Bidi is w:bidi — right-to-left section layout.
	Bidi *OnOff `xml:"bidi"`
	// TitlePage is w:titlePg — first page uses a different header/footer pair.
	TitlePage *OnOff `xml:"titlePg"`
	// VAlign is w:vAlign@w:val — vertical alignment ("top","center","both","bottom").
	VAlign *StringVal `xml:"vAlign"`
	// PageBorders is w:pgBorders — section page border settings.
	PageBorders *PageBorders `xml:"pgBorders"`
	// FootnoteProps is w:footnotePr — section footnote properties.
	FootnoteProps *NoteProperties `xml:"footnotePr"`
	// EndnoteProps is w:endnotePr — section endnote properties.
	EndnoteProps *NoteProperties `xml:"endnotePr"`
	// NoEndnote is w:noEndnote — suppress endnotes at section end.
	NoEndnote *OnOff `xml:"noEndnote"`
	// RtlGutter is w:rtlGutter — gutter is on the right side of the page.
	RtlGutter *OnOff `xml:"rtlGutter"`
	// FormProt is w:formProt — section is protected for forms.
	FormProt *OnOff `xml:"formProt"`
	// PrinterSettings is w:printerSettings — reference to printer settings part.
	PrinterSettings *ContentPartRef `xml:"printerSettings"`
	// PropertyChange is w:sectPrChange — tracked section property revision metadata.
	PropertyChange *PropertyChangeInfo `xml:"sectPrChange"`
	Unknown        []RawElement        `xml:",any"`
	Diagnostics    diag.List           `xml:"-"`
}

// PaperSource represents w:paperSrc.
type PaperSource struct {
	First int `xml:"first,attr,omitempty"`
	Other int `xml:"other,attr,omitempty"`
}

// LineNumberType represents w:lnNumType — section line numbering settings.
type LineNumberType struct {
	CountBy  int    `xml:"countBy,attr,omitempty"`
	Start    int    `xml:"start,attr,omitempty"`
	Distance int    `xml:"distance,attr,omitempty"`
	Restart  string `xml:"restart,attr,omitempty"`
}

// HeaderFooterReference represents header/footer references stored in sectPr.
type HeaderFooterReference struct {
	Type string `xml:"type,attr"`
	ID   string `xml:"id,attr"`
}

// PageSize represents w:pgSz.
type PageSize struct {
	Width  int    `xml:"w,attr"`
	Height int    `xml:"h,attr"`
	Orient string `xml:"orient,attr,omitempty"`
}

// PageMargin represents w:pgMar.
type PageMargin struct {
	Top    int `xml:"top,attr"`
	Right  int `xml:"right,attr"`
	Bottom int `xml:"bottom,attr"`
	Left   int `xml:"left,attr"`
	Header int `xml:"header,attr"`
	Footer int `xml:"footer,attr"`
	Gutter int `xml:"gutter,attr,omitempty"`
}

// Columns represents w:cols.
//
// When equalWidth is false, Word stores the authoritative physical column
// geometry in child w:col elements. Keeping only num/space loses unequal
// widths and per-column gaps and forces the renderer to invent an equal grid.
type Columns struct {
	Num        int      `xml:"num,attr,omitempty"`
	Space      int      `xml:"space,attr,omitempty"`
	Sep        bool     `xml:"sep,attr,omitempty"`
	EqualWidth *BoolVal `xml:"equalWidth,attr,omitempty"`
	Cols       []Column `xml:"col"`
}

// Column is one explicit w:col track inside w:cols. Width and Space are twips.
type Column struct {
	Width int `xml:"w,attr,omitempty"`
	Space int `xml:"space,attr,omitempty"`
}

// DocGrid represents w:docGrid.
type DocGrid struct {
	LinePitch int    `xml:"linePitch,attr,omitempty"`
	CharSpace int    `xml:"charSpace,attr,omitempty"`
	Type      string `xml:"type,attr,omitempty"`
}

// PageNumberType represents w:pgNumType.
type PageNumberType struct {
	Start  int    `xml:"start,attr,omitempty"`
	Format string `xml:"fmt,attr,omitempty"`
}

// PageBorders represents w:pgBorders — section page border settings.
type PageBorders struct {
	Top    *Border `xml:"top"`
	Left   *Border `xml:"left"`
	Start  *Border `xml:"start"`
	Bottom *Border `xml:"bottom"`
	Right  *Border `xml:"right"`
	End    *Border `xml:"end"`
	// Display controls which pages show borders: "allPages" | "firstPage" | "notFirstPage".
	Display string `xml:"display,attr,omitempty"`
	// OffsetFrom controls whether distance is measured from "text" or "edge".
	OffsetFrom string `xml:"offsetFrom,attr,omitempty"`
	// ZOrder controls rendering layer: "front" | "back".
	ZOrder string `xml:"zOrder,attr,omitempty"`
}

// TableLook represents w:tblLook conditional formatting flags.
type TableLook struct {
	Val      string `xml:"val,attr,omitempty"`
	FirstRow string `xml:"firstRow,attr,omitempty"`
	LastRow  string `xml:"lastRow,attr,omitempty"`
	FirstCol string `xml:"firstColumn,attr,omitempty"`
	LastCol  string `xml:"lastColumn,attr,omitempty"`
	NoHBand  string `xml:"noHBand,attr,omitempty"`
	NoVBand  string `xml:"noVBand,attr,omitempty"`
}

// TableCellMargins represents w:tcMar.
type TableCellMargins struct {
	Top    *TableMeasure `xml:"top"`
	Left   *TableMeasure `xml:"left"`
	Bottom *TableMeasure `xml:"bottom"`
	Right  *TableMeasure `xml:"right"`
}

// TableStyleProps represents w:tblStylePr conditional table style formatting.
type TableStyleProps struct {
	Type       string               `xml:"type,attr"`
	ParaProps  *ParagraphProperties `xml:"pPr"`
	RunProps   *RunProperties       `xml:"rPr"`
	TableProps *TableProperties     `xml:"tblPr"`
	RowProps   *TableRowProperties  `xml:"trPr"`
	CellProps  *TableCellProperties `xml:"tcPr"`
}

// ThemeTypeface represents a theme script-specific font element.
type ThemeTypeface struct {
	Typeface string `xml:"typeface,attr,omitempty"`
	Script   string `xml:"script,attr,omitempty"`
}

// ThemeSupplementalFont represents a theme <a:font> entry.
type ThemeSupplementalFont struct {
	Typeface string `xml:"typeface,attr,omitempty"`
	Script   string `xml:"script,attr,omitempty"`
}

// StringVal is a generic w:val string wrapper.
type StringVal struct {
	Val string `xml:"val,attr"`
}

// IntVal is a generic w:val integer wrapper.
type IntVal struct {
	Val int `xml:"val,attr"`
}

// Kinsoku represents w:noLineBreaksAfter or w:noLineBreaksBefore —
// a set of characters that cannot start or end a line.
type Kinsoku struct {
	Lang string `xml:"lang,attr,omitempty"`
	Val  string `xml:"val,attr"`
}

// Drawing represents w:drawing — a DrawingML graphic embedded in a run.
type Drawing struct {
	Inline *DrawingInline `xml:"inline"`
	Anchor *DrawingAnchor `xml:"anchor"`
}

// DrawingInline represents wp:inline — an inline (in-flow) drawing.
type DrawingInline struct {
	Extent            *DrawingExtent                   `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing extent"`
	EffectExtent      *EffectExtent                    `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing effectExtent"`
	DocPr             *DrawingDocPr                    `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing docPr"`
	CNvPr             *DrawingDocPr                    `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvPr"`
	CNvGraphicFramePr *NonVisualGraphicFrameProperties `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvGraphicFramePr"`
	ExtLst            *ExtensionList                   `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing extLst"`
	Graphic           *Graphic                         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphic"`
}

// DrawingAnchor represents wp:anchor — a floating drawing.
type DrawingAnchor struct {
	XMLName           xml.Name                         `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing anchor"`
	RelativeHeight    int64                            `xml:"relativeHeight,attr,omitempty"`
	BehindDoc         int                              `xml:"behindDoc,attr"`       // 0 = in front of text, 1 = behind text
	DistT             int64                            `xml:"distT,attr,omitempty"` // EMU top wrap distance
	DistB             int64                            `xml:"distB,attr,omitempty"` // EMU bottom wrap distance
	DistL             int64                            `xml:"distL,attr,omitempty"` // EMU left wrap distance
	DistR             int64                            `xml:"distR,attr,omitempty"` // EMU right wrap distance
	SimplePosAttr     bool                             `xml:"simplePos,attr,omitempty"`
	Locked            bool                             `xml:"locked,attr,omitempty"`
	LayoutInCell      bool                             `xml:"layoutInCell,attr,omitempty"`
	AllowOverlap      bool                             `xml:"allowOverlap,attr,omitempty"`
	Extent            *DrawingExtent                   `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing extent"`
	EffectExtent      *EffectExtent                    `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing effectExtent"`
	DocPr             *DrawingDocPr                    `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing docPr"`
	CNvPr             *DrawingDocPr                    `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvPr"`
	CNvGraphicFramePr *NonVisualGraphicFrameProperties `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvGraphicFramePr"`
	ExtLst            *ExtensionList                   `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing extLst"`
	Graphic           *Graphic                         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphic"`
	SimplePos         *SimplePos                       `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing simplePos"`
	PositionH         *DrawingPosition                 `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing positionH"`
	PositionV         *DrawingPosition                 `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing positionV"`
	SizeRelH          *SizeRel                         `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing sizeRelH"`
	SizeRelV          *SizeRel                         `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing sizeRelV"`
	// Wrap mode — exactly one of these will be non-nil for a valid anchor.
	WrapNone         *DrawingWrapEmpty `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapNone"`         // no wrapping
	WrapSquare       *DrawingWrapSided `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapSquare"`       // square wrap
	WrapTight        *DrawingWrapSided `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapTight"`        // tight wrap
	WrapThrough      *DrawingWrapSided `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapThrough"`      // through wrap
	WrapTopAndBottom *DrawingWrapEmpty `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapTopAndBottom"` // top-and-bottom wrap
}

// NonVisualGraphicFrameProperties represents a:CT_NonVisualGraphicFrameProperties
// (cNvGraphicFramePr in wp:inline/wp:anchor).
type NonVisualGraphicFrameProperties struct {
	XMLName xml.Name `xml:"cNvGraphicFramePr"`
}

// ExtensionList represents a:CT_OfficeArtExtensionList (extLst).
type ExtensionList struct {
	XMLName xml.Name `xml:"extLst"`
}

// DrawingWrapEmpty is a sentinel for wrap mode elements that carry no attributes.
// It also carries optional distance overrides used by wrapTopAndBottom.
type DrawingWrapEmpty struct {
	DrawingWrapDistances
}

// DrawingWrapDistances carries per-wrap-mode distance overrides (EMU).
type DrawingWrapDistances struct {
	DistT int64 `xml:"distT,attr,omitempty"`
	DistB int64 `xml:"distB,attr,omitempty"`
	DistL int64 `xml:"distL,attr,omitempty"`
	DistR int64 `xml:"distR,attr,omitempty"`
}

// DrawingWrapSided is a wrap mode element that carries a wrapText side selector
// and an optional contour polygon (present for wrapTight and wrapThrough).
type DrawingWrapSided struct {
	DrawingWrapDistances
	WrapText string       `xml:"wrapText,attr,omitempty"` // "bothSides"|"left"|"right"|"largest"
	Polygon  *WrapPolygon `xml:"wrapPolygon"`
}

// WrapPolygon holds the contour vertices from a wp:wrapPolygon element (EMU).
type WrapPolygon struct {
	Start  WrapPolygonPt   `xml:"start"`
	LineTo []WrapPolygonPt `xml:"lineTo"`
}

// WrapPolygonPt is a single vertex of a wrap polygon.
type WrapPolygonPt struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// DrawingPosition stores a wp:positionH/wp:positionV offset, alignment, or percentage.
// At most one of PosOffset, Align, or Pct will be set.
type DrawingPosition struct {
	RelativeFrom string `xml:"relativeFrom,attr,omitempty"`
	PosOffset    int64  `xml:"posOffset"`
	Align        string `xml:"align"` // "left"|"right"|"center"|"inside"|"outside"|"top"|"bottom"
	Pct          int64  `xml:"pct"`   // thousandths of a percent (100000 = 100%)
}

// DrawingExtent is the size of a drawing in EMU (English Metric Units).
// 914400 EMU = 1 inch.
type DrawingExtent struct {
	CX int64 `xml:"cx,attr"` // width
	CY int64 `xml:"cy,attr"` // height
}

// SimplePos stores the simple x/y coordinates from a wp:simplePos element (EMU).
type SimplePos struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// EffectExtent stores the l/t/r/b effect overflow from a wp:effectExtent element (EMU).
type EffectExtent struct {
	L int64 `xml:"l,attr"`
	T int64 `xml:"t,attr"`
	R int64 `xml:"r,attr"`
	B int64 `xml:"b,attr"`
}

// SizeRel stores relative sizing from a wp:sizeRelH or wp:sizeRelV element.
// Pct is in thousandths of a percent (100000 = 100%).
type SizeRel struct {
	RelativeFrom string `xml:"relativeFrom,attr,omitempty"`
	Pct          int64  `xml:"pct"`
}

// DrawingDocPr holds drawing identity and accessibility metadata.
type DrawingDocPr struct {
	ID     int    `xml:"id,attr"`
	Name   string `xml:"name,attr,omitempty"`
	Desc   string `xml:"descr,attr,omitempty"`
	Title  string `xml:"title,attr,omitempty"`
	Hidden bool   `xml:"hidden,attr,omitempty"`
}

// Graphic represents a:graphic.
type Graphic struct {
	GraphicData *GraphicData `xml:"graphicData"`
}

// ChartRef represents a chart reference within graphicData (c:chart).
type ChartRef struct {
	ID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// DiagramRelIDs represents dgm:relIds references inside a SmartArt graphic.
type DiagramRelIDs struct {
	DataModelID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships dm,attr"`
	LayoutID     string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships lo,attr"`
	QuickStyleID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships qs,attr"`
	ColorStyleID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships cs,attr"`
}

// GraphicData represents a:graphicData.
type GraphicData struct {
	URI                 string                  `xml:"uri,attr,omitempty"`
	Picture             *Picture                `xml:"http://schemas.openxmlformats.org/drawingml/2006/picture pic"`
	DrawingMLPic        *DrawingMLPicture       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pic"`
	Shape               *WpsShape               `xml:"wsp"`
	DrawingMLShape      *DrawingMLShape         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sp"`
	GroupShape          *WpsGroupShape          `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing grpSp"`
	DrawingMLGroupShape *DrawingMLGroupShape    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpSp"`
	WordprocessingGroup *WpsWordprocessingGroup `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wgp"`
	Canvas              *WpsCanvas              `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wpc"`
	GraphicFrame        *WpsGraphicFrame        `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing graphicFrame"`
	DrawingMLGraphicFr  *DrawingMLGraphicFrame  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphicFrame"`
	ContentPart         *WpsContentPart         `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing contentPart"`
	Chart               *ChartRef               `xml:"chart"`
	DiagramIDs          *DiagramRelIDs          `xml:"relIds"`
	Table               *DrawingTable           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tbl"`
}

// WpsShape represents wps:wsp — a WordprocessingShape container.
type WpsShape struct {
	CNvPr      *DrawingDocPr                 `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape cNvPr"`
	CNvSpPr    *NonVisualShapeProperties     `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape cNvSpPr"`
	CNvCnPr    *NonVisualConnectorProperties `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape cNvCnPr"`
	ShapeProps *WpsShapeProperties           `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape spPr"`
	Style      *WpsShapeStyle                `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape style"`
	TextBox    *WpsTextBox                   `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape txbx"`
	LinkedTxbx *WpsLinkedTextbox             `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape linkedTxbx"`
	BodyPr     *WpsBodyProperties            `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape bodyPr"`
	ExtLst     *ExtensionList                `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape extLst"`
}

// EmuAbsent is the sentinel for a bodyPr inset attribute that was not present in XML.
// The caller must substitute the OOXML default for that attribute.
const EmuAbsent = int64(-1)

// NonVisualShapeProperties represents a:CT_NonVisualDrawingShapeProps (cNvSpPr).
type NonVisualShapeProperties struct {
	XMLName xml.Name `xml:"cNvSpPr"`
}

// NonVisualConnectorProperties represents a:CT_NonVisualConnectorProperties (cNvCnPr).
type NonVisualConnectorProperties struct {
	XMLName xml.Name `xml:"cNvCnPr"`
}

// WpsShapeStyle represents a:CT_ShapeStyle inside a wps:wsp (style).
type WpsShapeStyle struct {
	LnRef     *WpsStyleRef `xml:"lnRef"`
	FillRef   *WpsStyleRef `xml:"fillRef"`
	EffectRef *WpsStyleRef `xml:"effectRef"`
	FontRef   *WpsFontRef  `xml:"fontRef"`
}

// WpsStyleRef is a style reference (lnRef, fillRef, effectRef).
type WpsStyleRef struct {
	Idx int `xml:"idx,attr"`
}

// WpsFontRef is a font reference inside a shape style.
type WpsFontRef struct {
	Idx string `xml:"idx,attr"`
}

// WpsLinkedTextbox represents wps:linkedTxbx — linked textbox information.
type WpsLinkedTextbox struct {
	ID     int            `xml:"id,attr"`
	Seq    int            `xml:"seq,attr"`
	ExtLst *ExtensionList `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape extLst"`
}

// WpsGroupShape represents wp:grpSp — a wordprocessing group shape container.
type WpsGroupShape struct {
	CNvGrpSpPr   *NonVisualGroupShapeProperties `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvGrpSpPr"`
	GrpSpPr      *GroupShapeProperties          `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing grpSpPr"`
	Shape        *WpsShape                      `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wsp"`
	GroupShape   *WpsGroupShape                 `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing grpSp"`
	GraphicFrame *WpsGraphicFrame               `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing graphicFrame"`
	ContentPart  *WpsContentPart                `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing contentPart"`
	ExtLst       *ExtensionList                 `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing extLst"`
}

// NonVisualGroupShapeProperties represents a:CT_NonVisualGroupDrawingShapeProps (cNvGrpSpPr).
type NonVisualGroupShapeProperties struct {
	XMLName xml.Name `xml:"cNvGrpSpPr"`
}

// GroupShapeProperties represents a:CT_GroupShapeProperties (grpSpPr).
type GroupShapeProperties struct {
	XMLName xml.Name            `xml:"grpSpPr"`
	Xfrm    *DrawingTransform2D `xml:"xfrm"`
}

// WpsGraphicFrame represents wp:graphicFrame — a graphic frame container.
type WpsGraphicFrame struct {
	CNvPr   *DrawingDocPr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvPr"`
	CNvFrPr *NonVisualFrameProperties `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvFrPr"`
	Graphic *Graphic                  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphic"`
}

// WpsContentPart represents wp:contentPart — a content part reference.
type WpsContentPart struct {
	NvContentPartPr *WordprocessingContentPartNonVisual `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing nvContentPartPr"`
	Xfrm            *Transform2D                        `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing xfrm"`
	ExtLst          *ExtensionList                      `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing extLst"`
}

// WordprocessingContentPartNonVisual represents wp:CT_WordprocessingContentPartNonVisual (nvContentPartPr).
type WordprocessingContentPartNonVisual struct {
	CNvPr            *DrawingDocPr                   `xml:"cNvPr"`
	CNvContentPartPr *NonVisualContentPartProperties `xml:"cNvContentPartPr"`
}

// NonVisualFrameProperties represents a:CT_NonVisualGraphicFrameProperties (cNvFrPr).
type NonVisualFrameProperties struct {
	XMLName xml.Name `xml:"cNvFrPr"`
}

// NonVisualContentPartProperties represents a:CT_NonVisualContentPartProperties (cNvContentPartPr).
type NonVisualContentPartProperties struct {
	XMLName xml.Name `xml:"cNvContentPartPr"`
}

// Transform2D represents a:CT_Transform2D (xfrm).
type Transform2D struct {
	XMLName xml.Name `xml:"xfrm"`
}

// WpsWordprocessingGroup represents wp:wgp — a wordprocessing group element.
type WpsWordprocessingGroup struct {
	CNvGrpSpPr   *NonVisualGroupShapeProperties `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvGrpSpPr"`
	GrpSpPr      *GroupShapeProperties          `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing grpSpPr"`
	Shape        *WpsShape                      `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wsp"`
	GroupShape   *WpsWordprocessingGroup        `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wgp"`
	GraphicFrame *WpsGraphicFrame               `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing graphicFrame"`
	ContentPart  *WpsContentPart                `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing contentPart"`
	ExtLst       *ExtensionList                 `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing extLst"`
}

// WpsCanvas represents wp:wpc — a wordprocessing canvas element.
type WpsCanvas struct {
	Bg           *WpsBackgroundFormatting `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing bg"`
	Whole        *WpsWholeE2oFormatting   `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing whole"`
	Shape        *WpsShape                `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wsp"`
	Group        *WpsWordprocessingGroup  `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wgp"`
	GraphicFrame *WpsGraphicFrame         `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing graphicFrame"`
	ContentPart  *WpsContentPart          `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing contentPart"`
	ExtLst       *ExtensionList           `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing extLst"`
}

// WpsBackgroundFormatting represents a:CT_BackgroundFormatting (bg).
type WpsBackgroundFormatting struct {
	XMLName xml.Name `xml:"bg"`
}

// WpsWholeE2oFormatting represents a:CT_WholeE2oFormatting (whole).
type WpsWholeE2oFormatting struct {
	XMLName xml.Name `xml:"whole"`
}

// WpsBodyProperties represents wps:bodyPr — text body layout properties for a shape.
// Inset fields use EmuAbsent (-1) when the XML attribute is absent (use OOXML defaults).
type WpsBodyProperties struct {
	LIns int64 // left  inset in EMU; EmuAbsent → default 91440 (~7.2pt)
	TIns int64 // top   inset in EMU; EmuAbsent → default 45720 (~3.6pt)
	RIns int64 // right inset in EMU; EmuAbsent → default 91440 (~7.2pt)
	BIns int64 // bot   inset in EMU; EmuAbsent → default 45720 (~3.6pt)
	Vert string
	Wrap string
	// Anchor is the vertical text anchor: "t" (top), "ctr" (center), "b" (bottom).
	Anchor string
	// AnchorCtr is true when text is anchored at the center horizontally.
	AnchorCtr bool
	// Rot is the text rotation in thousandths of a degree (60000 = 1 degree).
	Rot int
	// Upright is true when text stays upright regardless of shape rotation.
	Upright bool
	// Autofit mode: "none", "shape", "norm".
	Autofit string
	// FontScale is the font scale percentage for normAutofit (thousandths of a percent).
	FontScale int
	// LnSpcReduction is the line spacing reduction for normAutofit (thousandths of a percent).
	LnSpcReduction int
	// PrstTxWarp is the preset text warp shape.
	PrstTxWarp string
	// FlatTx is true when text is rendered flat (no 3D rotation).
	FlatTx bool
	// NumCol is the number of text columns inside the shape (0 = single column).
	NumCol int
	// SpcCol is the space between columns in EMU (0 = default).
	SpcCol int64
	// RtlCol is true when columns flow right-to-left.
	RtlCol bool
}

// UnmarshalXML parses wps:bodyPr, setting inset fields to EmuAbsent when
// the corresponding XML attribute is absent (allowing callers to apply OOXML defaults).
func (b *WpsBodyProperties) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	b.LIns = EmuAbsent
	b.TIns = EmuAbsent
	b.RIns = EmuAbsent
	b.BIns = EmuAbsent
	for _, attr := range start.Attr {
		var dst *int64
		switch attr.Name.Local {
		case "lIns":
			dst = &b.LIns
		case "tIns":
			dst = &b.TIns
		case "rIns":
			dst = &b.RIns
		case "bIns":
			dst = &b.BIns
		case "vert":
			b.Vert = attr.Value
		case "wrap":
			b.Wrap = attr.Value
		case "anchor":
			b.Anchor = attr.Value
		case "anchorCtr":
			b.AnchorCtr = attr.Value == "1" || attr.Value == "true"
		case "rot":
			if v, err := strconv.Atoi(attr.Value); err == nil {
				b.Rot = v
			}
		case "upright":
			b.Upright = attr.Value == "1" || attr.Value == "true"
		case "numCol":
			if v, err := strconv.Atoi(attr.Value); err == nil {
				b.NumCol = v
			}
		case "spcCol":
			if v, err := strconv.ParseInt(attr.Value, 10, 64); err == nil {
				b.SpcCol = v
			}
		case "rtlCol":
			b.RtlCol = attr.Value == "1" || attr.Value == "true"
		}
		if dst != nil {
			v, err := strconv.ParseInt(attr.Value, 10, 64)
			if err != nil {
				return err
			}
			*dst = v
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "noAutofit":
				b.Autofit = "none"
				dec.Skip()
			case "spAutoFit":
				b.Autofit = "shape"
				dec.Skip()
			case "normAutofit":
				b.Autofit = "norm"
				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "fontScale":
						if v, err := strconv.Atoi(attr.Value); err == nil {
							b.FontScale = v
						}
					case "lnSpcReduction":
						if v, err := strconv.Atoi(attr.Value); err == nil {
							b.LnSpcReduction = v
						}
					}
				}
				dec.Skip()
			case "prstTxWarp":
				for _, attr := range t.Attr {
					if attr.Name.Local == "prst" {
						b.PrstTxWarp = attr.Value
					}
				}
				dec.Skip()
			case "flatTx":
				b.FlatTx = true
				dec.Skip()
			default:
				dec.Skip()
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

// WpsShapeProperties stores the subset of DrawingML shape properties needed for layout.
// It uses a custom UnmarshalXML so that the exported fields carry shared drawing package
// types while the actual XML parsing is done via the concrete local structs that have
// the correct xml struct tags.
//
// WpsShapeProperties stores the subset of DrawingML shape properties needed for layout.
type WpsShapeProperties struct {
	Geometry         *drawing.PresetGeometry
	GeometryData     *drawing.Geometry
	Fill             drawing.Fill
	Line             *drawing.LineStyle
	Xfrm             *DrawingTransform2D
	Effects          drawing.EffectList
	Scene3D          *wpsScene3D
	Shape3D          *wpsShape3DRaw
	OuterShadow      *wpsOuterShadowEffect
	InnerShadow      *wpsInnerShadowEffect
	Blur             *wpsBlurEffect
	Glow             *wpsGlowEffect
	SoftEdge         *wpsSoftEdgeEffect
	Reflection       *wpsReflectionEffect
	PresetShadow     *wpsPresetShadowEffect
	AlphaMod         *wpsAlphaEffect
	AlphaBiLevel     *wpsBiLevelEffect
	AlphaOff         *wpsAlphaEffect
	AlphaModFix      *wpsAlphaEffect
	AlphaOutset      *wpsOutsetEffect
	ParseDiagnostics diag.List
	AlphaRepl        *wpsAlphaEffect
	AlphaInv         *struct{}
	AlphaCeiling     *struct{}
	AlphaFloor       *struct{}
	Grayscale        *struct{}
	BiLevel          *wpsBiLevelEffect
	Lum              *wpsLumEffect
	HSL              *wpsHSLEffect
	DeSat            *struct{}
	DeSatLow         *struct{}
	DeSatMod         *wpsValueEffect
	Duotone          *wpsDuotoneEffect
	ClrChange        *wpsClrChangeEffect
	ClrRepl          *wpsClrReplEffect
	FillOverlay      *wpsFillOverlayEffect
	EffectDag        *wpsEffectContainerRaw
}

type wpsEffectAlpha struct {
	Val int `xml:"val,attr,omitempty"`
}

type wpsEffectColorRef struct {
	Val   string          `xml:"val,attr"`
	Alpha *wpsEffectAlpha `xml:"alpha"`
}

type wpsEffectSchemeColor struct {
	Val   string          `xml:"val,attr"`
	Alpha *wpsEffectAlpha `xml:"alpha"`
}

type wpsOuterShadowEffect struct {
	BlurRad   int64                 `xml:"blurRad,attr,omitempty"`
	Dist      int64                 `xml:"dist,attr,omitempty"`
	Dir       int                   `xml:"dir,attr,omitempty"`
	SrgbClr   *wpsEffectColorRef    `xml:"srgbClr"`
	SchemeClr *wpsEffectSchemeColor `xml:"schemeClr"`
}

type wpsInnerShadowEffect struct {
	BlurRad   int64                 `xml:"blurRad,attr,omitempty"`
	Dist      int64                 `xml:"dist,attr,omitempty"`
	Dir       int                   `xml:"dir,attr,omitempty"`
	SrgbClr   *wpsEffectColorRef    `xml:"srgbClr"`
	SchemeClr *wpsEffectSchemeColor `xml:"schemeClr"`
}

type wpsBlurEffect struct {
	Rad int64 `xml:"rad,attr,omitempty"`
}

type wpsGlowEffect struct {
	Rad       int64                 `xml:"rad,attr,omitempty"`
	SrgbClr   *wpsEffectColorRef    `xml:"srgbClr"`
	SchemeClr *wpsEffectSchemeColor `xml:"schemeClr"`
}

type wpsSoftEdgeEffect struct {
	Rad int64 `xml:"rad,attr,omitempty"`
}

type wpsReflectionEffect struct {
	BlurRad int64 `xml:"blurRad,attr,omitempty"`
	Dist    int64 `xml:"dist,attr,omitempty"`
	Dir     int   `xml:"dir,attr,omitempty"`
	StA     int   `xml:"stA,attr,omitempty"`
}

type wpsPresetShadowEffect struct {
	Prst      string                `xml:"prst,attr,omitempty"`
	Dist      int64                 `xml:"dist,attr,omitempty"`
	Dir       int                   `xml:"dir,attr,omitempty"`
	SrgbClr   *wpsEffectColorRef    `xml:"srgbClr"`
	SchemeClr *wpsEffectSchemeColor `xml:"schemeClr"`
}

type wpsAlphaEffect struct {
	Val int `xml:"val,attr,omitempty"`
}

type wpsBiLevelEffect struct {
	Thresh int `xml:"thresh,attr,omitempty"`
}

type wpsOutsetEffect struct {
	Rad int64 `xml:"rad,attr,omitempty"`
}

type wpsLumEffect struct {
	Bright   int `xml:"bright,attr,omitempty"`
	Contrast int `xml:"contrast,attr,omitempty"`
}

type wpsHSLEffect struct {
	Hue int `xml:"hue,attr,omitempty"`
	Sat int `xml:"sat,attr,omitempty"`
	Lum int `xml:"lum,attr,omitempty"`
}

type wpsValueEffect struct {
	Val int `xml:"val,attr,omitempty"`
}

type wpsColorChoiceRaw struct {
	SRGBClr   *ThemeSRGBColor   `xml:"srgbClr"`
	SchemeClr *DrawingSchemeClr `xml:"schemeClr"`
}

type wpsDuotoneEffect struct {
	SRGBClrs   []*ThemeSRGBColor   `xml:"srgbClr"`
	SchemeClrs []*DrawingSchemeClr `xml:"schemeClr"`
}

type wpsClrChangeEffect struct {
	UseA    bool               `xml:"useA,attr,omitempty"`
	ClrFrom *wpsColorChoiceRaw `xml:"clrFrom"`
	ClrTo   *wpsColorChoiceRaw `xml:"clrTo"`
}

type wpsClrReplEffect struct {
	SRGBClr   *ThemeSRGBColor   `xml:"srgbClr"`
	SchemeClr *DrawingSchemeClr `xml:"schemeClr"`
}

type wpsFillOverlayEffect struct {
	Blend     string                   `xml:"blend,attr,omitempty"`
	SolidFill *drawingxml.SolidFill    `xml:"solidFill"`
	GradFill  *drawingxml.GradientFill `xml:"gradFill"`
	NoFill    *struct{}                `xml:"noFill"`
	PattFill  *drawingxml.PatternFill  `xml:"pattFill"`
	BlipFill  *wpsBlipFillRaw          `xml:"blipFill"`
}

type wpsFillEffect struct {
	SolidFill *drawingxml.SolidFill    `xml:"solidFill"`
	GradFill  *drawingxml.GradientFill `xml:"gradFill"`
	NoFill    *struct{}                `xml:"noFill"`
	PattFill  *drawingxml.PatternFill  `xml:"pattFill"`
	BlipFill  *wpsBlipFillRaw          `xml:"blipFill"`
}

type wpsAlphaModulateEffect struct {
	Cont *wpsEffectContainerRaw `xml:"cont"`
}

type wpsBlendEffect struct {
	Blend string                 `xml:"blend,attr,omitempty"`
	Cont  *wpsEffectContainerRaw `xml:"cont"`
}

type wpsRelativeOffsetEffect struct {
	Tx int `xml:"tx,attr,omitempty"`
	Ty int `xml:"ty,attr,omitempty"`
}

type wpsTintEffect struct {
	Hue int `xml:"hue,attr,omitempty"`
	Amt int `xml:"amt,attr,omitempty"`
}

type wpsTransformEffect struct {
	Sx int   `xml:"sx,attr,omitempty"`
	Sy int   `xml:"sy,attr,omitempty"`
	Kx int   `xml:"kx,attr,omitempty"`
	Ky int   `xml:"ky,attr,omitempty"`
	Tx int64 `xml:"tx,attr,omitempty"`
	Ty int64 `xml:"ty,attr,omitempty"`
}

type wpsEffectReference struct {
	Ref string `xml:"ref,attr,omitempty"`
}

type wpsEffectRaw struct {
	Container    *wpsEffectContainerRaw
	Reference    *wpsEffectReference
	AlphaBiLevel *wpsBiLevelEffect
	AlphaCeiling *struct{}
	AlphaFloor   *struct{}
	AlphaInv     *struct{}
	AlphaMod     *wpsAlphaModulateEffect
	AlphaModFix  *wpsAlphaEffect
	AlphaOff     *wpsAlphaEffect
	AlphaOutset  *wpsOutsetEffect
	AlphaRepl    *wpsAlphaEffect
	BiLevel      *wpsBiLevelEffect
	Blend        *wpsBlendEffect
	Blur         *wpsBlurEffect
	ClrChange    *wpsClrChangeEffect
	ClrRepl      *wpsClrReplEffect
	DeSat        *struct{}
	DeSatLow     *struct{}
	DeSatMod     *wpsValueEffect
	Duotone      *wpsDuotoneEffect
	Fill         *wpsFillEffect
	FillOverlay  *wpsFillOverlayEffect
	Glow         *wpsGlowEffect
	Grayscale    *struct{}
	HSL          *wpsHSLEffect
	InnerShadow  *wpsInnerShadowEffect
	Lum          *wpsLumEffect
	OuterShadow  *wpsOuterShadowEffect
	PresetShadow *wpsPresetShadowEffect
	Reflection   *wpsReflectionEffect
	RelOff       *wpsRelativeOffsetEffect
	SoftEdge     *wpsSoftEdgeEffect
	Tint         *wpsTintEffect
	Transform    *wpsTransformEffect
}

func (e *wpsEffectRaw) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	switch start.Name.Local {
	case "cont", "effectDag":
		var raw wpsEffectContainerRaw
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Container = &raw
	case "effect":
		var raw wpsEffectReference
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Reference = &raw
	case "alphaBiLevel":
		var raw wpsBiLevelEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.AlphaBiLevel = &raw
	case "alphaCeiling":
		var raw struct{}
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.AlphaCeiling = &raw
	case "alphaFloor":
		var raw struct{}
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.AlphaFloor = &raw
	case "alphaInv":
		var raw struct{}
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.AlphaInv = &raw
	case "alphaMod":
		var raw wpsAlphaModulateEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.AlphaMod = &raw
	case "alphaModFix":
		var raw wpsAlphaEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.AlphaModFix = &raw
	case "alphaOff":
		var raw wpsAlphaEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.AlphaOff = &raw
	case "alphaOutset":
		var raw wpsOutsetEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.AlphaOutset = &raw
	case "alphaRepl":
		var raw wpsAlphaEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.AlphaRepl = &raw
	case "biLevel":
		var raw wpsBiLevelEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.BiLevel = &raw
	case "blend":
		var raw wpsBlendEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Blend = &raw
	case "blur":
		var raw wpsBlurEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Blur = &raw
	case "clrChange":
		var raw wpsClrChangeEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.ClrChange = &raw
	case "clrRepl":
		var raw wpsClrReplEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.ClrRepl = &raw
	case "deSat":
		var raw struct{}
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.DeSat = &raw
	case "deSatLow":
		var raw struct{}
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.DeSatLow = &raw
	case "deSatMod":
		var raw wpsValueEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.DeSatMod = &raw
	case "duotone":
		var raw wpsDuotoneEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Duotone = &raw
	case "fill":
		var raw wpsFillEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Fill = &raw
	case "fillOverlay":
		var raw wpsFillOverlayEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.FillOverlay = &raw
	case "glow":
		var raw wpsGlowEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Glow = &raw
	case "grayscl":
		var raw struct{}
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Grayscale = &raw
	case "hsl":
		var raw wpsHSLEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.HSL = &raw
	case "innerShdw":
		var raw wpsInnerShadowEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.InnerShadow = &raw
	case "lum":
		var raw wpsLumEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Lum = &raw
	case "outerShdw":
		var raw wpsOuterShadowEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.OuterShadow = &raw
	case "prstShdw":
		var raw wpsPresetShadowEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.PresetShadow = &raw
	case "reflection":
		var raw wpsReflectionEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Reflection = &raw
	case "relOff":
		var raw wpsRelativeOffsetEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.RelOff = &raw
	case "softEdge":
		var raw wpsSoftEdgeEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.SoftEdge = &raw
	case "tint":
		var raw wpsTintEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Tint = &raw
	case "xfrm":
		var raw wpsTransformEffect
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return err
		}
		e.Transform = &raw
	default:
		return dec.Skip()
	}
	return nil
}

type wpsEffectContainerRaw struct {
	Type    string
	Name    string
	Effects []wpsEffectRaw
}

func (c *wpsEffectContainerRaw) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "type":
			c.Type = attr.Value
		case "name":
			c.Name = attr.Value
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			var effect wpsEffectRaw
			if err := effect.UnmarshalXML(dec, tok); err != nil {
				return err
			}
			c.Effects = append(c.Effects, effect)
		case xml.EndElement:
			if tok.Name.Local == start.Name.Local && tok.Name.Space == start.Name.Space {
				return nil
			}
		}
	}
	return nil
}

type wpsScene3D struct {
	Camera   *wpsSceneCamera   `xml:"camera"`
	LightRig *wpsSceneLightRig `xml:"lightRig"`
}

type wpsSceneCamera struct {
	Prst string            `xml:"prst,attr,omitempty"`
	Rot  *wpsSceneRotation `xml:"rot"`
}

type wpsSceneLightRig struct {
	Rig string `xml:"rig,attr,omitempty"`
	Dir string `xml:"dir,attr,omitempty"`
}

type wpsSceneRotation struct {
	Lat int64 `xml:"lat,attr,omitempty"`
	Lon int64 `xml:"lon,attr,omitempty"`
	Rev int64 `xml:"rev,attr,omitempty"`
}

type wpsShape3DBevelRaw struct {
	W    int64  `xml:"w,attr,omitempty"`
	H    int64  `xml:"h,attr,omitempty"`
	Prst string `xml:"prst,attr,omitempty"`
}

type wpsShape3DColorRef struct {
	SRGBClr   *ThemeSRGBColor   `xml:"srgbClr"`
	SchemeClr *DrawingSchemeClr `xml:"schemeClr"`
}

type wpsShape3DRaw struct {
	Z            int64               `xml:"z,attr,omitempty"`
	ExtrusionH   int64               `xml:"extrusionH,attr,omitempty"`
	ContourW     int64               `xml:"contourW,attr,omitempty"`
	PrstMaterial string              `xml:"prstMaterial,attr,omitempty"`
	BevelT       *wpsShape3DBevelRaw `xml:"bevelT"`
	BevelB       *wpsShape3DBevelRaw `xml:"bevelB"`
	ContourClr   *wpsShape3DColorRef `xml:"contourClr"`
	ExtrusionClr *wpsShape3DColorRef `xml:"extrusionClr"`
}

type wpsGeometryGuideRaw = drawingxml.GeometryGuide

type wpsPresetGeometryRaw = drawingxml.PresetGeometry

// CustomGeometry describes a DrawingML custom geometry and its guide/path lists.
type CustomGeometry = drawingxml.CustomGeometry

// CustomPath stores one custom-geometry path in its local coordinate space.
type CustomPath = drawingxml.CustomPath

// FormulaPoint stores a custom-path coordinate that may be a formula.
type FormulaPoint = drawingxml.FormulaPoint

// CustomPathCommand stores one ordered command from a custom geometry path.
type CustomPathCommand = drawingxml.CustomPathCommand

// wpsBlipFillRaw is the XML-deserialisation target for a:blipFill inside wps:spPr.
type wpsBlipFillRaw struct {
	Blip    *Blip               `xml:"blip"`
	SrcRect *SrcRect            `xml:"srcRect"`
	Tile    *drawingxml.Tile    `xml:"tile"`
	Stretch *drawingxml.Stretch `xml:"stretch"`
}

// wpsShapePropsRaw is the XML-deserialisation target for WpsShapeProperties.
// It uses local concrete types (with proper xml tags) and is converted to
// drawing package types inside UnmarshalXML.
type wpsShapePropsRaw struct {
	Xfrm         *DrawingTransform2D      `xml:"xfrm"`
	PrstGeom     *wpsPresetGeometryRaw    `xml:"prstGeom"`
	CustGeom     *CustomGeometry          `xml:"custGeom"`
	Scene3D      *wpsScene3D              `xml:"scene3d"`
	Sp3D         *wpsShape3DRaw           `xml:"sp3d"`
	SolidFill    *drawingxml.SolidFill    `xml:"solidFill"`
	GradFill     *drawingxml.GradientFill `xml:"gradFill"`
	NoFill       *struct{}                `xml:"noFill"`
	PattFill     *drawingxml.PatternFill  `xml:"pattFill"`
	BlipFill     *wpsBlipFillRaw          `xml:"blipFill"`
	EffectDag    *wpsEffectContainerRaw   `xml:"effectDag"`
	OuterShdw    *wpsOuterShadowEffect    `xml:"effectLst>outerShdw"`
	InnerShdw    *wpsInnerShadowEffect    `xml:"effectLst>innerShdw"`
	Blur         *wpsBlurEffect           `xml:"effectLst>blur"`
	Glow         *wpsGlowEffect           `xml:"effectLst>glow"`
	SoftEdge     *wpsSoftEdgeEffect       `xml:"effectLst>softEdge"`
	Reflection   *wpsReflectionEffect     `xml:"effectLst>reflection"`
	PresetShdw   *wpsPresetShadowEffect   `xml:"effectLst>prstShdw"`
	AlphaBiLevel *wpsBiLevelEffect        `xml:"effectLst>alphaBiLevel"`
	AlphaMod     *wpsAlphaEffect          `xml:"effectLst>alphaMod"`
	AlphaOff     *wpsAlphaEffect          `xml:"effectLst>alphaOff"`
	AlphaModFix  *wpsAlphaEffect          `xml:"effectLst>alphaModFix"`
	AlphaOutset  *wpsOutsetEffect         `xml:"effectLst>alphaOutset"`
	AlphaRepl    *wpsAlphaEffect          `xml:"effectLst>alphaRepl"`
	AlphaInv     *struct{}                `xml:"effectLst>alphaInv"`
	AlphaCeiling *struct{}                `xml:"effectLst>alphaCeiling"`
	AlphaFloor   *struct{}                `xml:"effectLst>alphaFloor"`
	Grayscale    *struct{}                `xml:"effectLst>grayscl"`
	BiLevel      *wpsBiLevelEffect        `xml:"effectLst>biLevel"`
	Lum          *wpsLumEffect            `xml:"effectLst>lum"`
	HSL          *wpsHSLEffect            `xml:"effectLst>hsl"`
	DeSat        *struct{}                `xml:"effectLst>deSat"`
	DeSatLow     *struct{}                `xml:"effectLst>deSatLow"`
	DeSatMod     *wpsValueEffect          `xml:"effectLst>deSatMod"`
	Duotone      *wpsDuotoneEffect        `xml:"effectLst>duotone"`
	ClrChange    *wpsClrChangeEffect      `xml:"effectLst>clrChange"`
	ClrRepl      *wpsClrReplEffect        `xml:"effectLst>clrRepl"`
	FillOverlay  *wpsFillOverlayEffect    `xml:"effectLst>fillOverlay"`
	Ln           *drawingxml.Line         `xml:"ln"`
}

func (p *WpsShapeProperties) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var raw wpsShapePropsRaw
	if err := dec.DecodeElement(&raw, &start); err != nil {
		return err
	}
	if raw.CustGeom != nil {
		p.GeometryData, p.ParseDiagnostics = raw.CustGeom.DrawingGeometryWithDiagnostics()
	} else if raw.PrstGeom != nil {
		p.Geometry = raw.PrstGeom.DrawingPresetGeometry()
	}
	p.Xfrm = raw.Xfrm
	p.Scene3D = raw.Scene3D
	p.Shape3D = raw.Sp3D
	p.OuterShadow = raw.OuterShdw
	p.InnerShadow = raw.InnerShdw
	p.Blur = raw.Blur
	p.Glow = raw.Glow
	p.SoftEdge = raw.SoftEdge
	p.Reflection = raw.Reflection
	p.PresetShadow = raw.PresetShdw
	p.AlphaBiLevel = raw.AlphaBiLevel
	p.AlphaMod = raw.AlphaMod
	p.AlphaOff = raw.AlphaOff
	p.AlphaModFix = raw.AlphaModFix
	p.AlphaOutset = raw.AlphaOutset
	p.AlphaRepl = raw.AlphaRepl
	p.AlphaInv = raw.AlphaInv
	p.AlphaCeiling = raw.AlphaCeiling
	p.AlphaFloor = raw.AlphaFloor
	p.Grayscale = raw.Grayscale
	p.BiLevel = raw.BiLevel
	p.Lum = raw.Lum
	p.HSL = raw.HSL
	p.DeSat = raw.DeSat
	p.DeSatLow = raw.DeSatLow
	p.DeSatMod = raw.DeSatMod
	p.Duotone = raw.Duotone
	p.ClrChange = raw.ClrChange
	p.ClrRepl = raw.ClrRepl
	p.FillOverlay = raw.FillOverlay
	p.EffectDag = raw.EffectDag
	p.Effects = p.DrawingEffectList(nil)
	switch {
	case raw.SolidFill != nil:
		p.Fill = raw.SolidFill.ToDrawingSolidFill()
	case raw.NoFill != nil:
		p.Fill = &drawing.NoFill{}
	case raw.GradFill != nil:
		p.Fill = raw.GradFill.ToDrawingGradientFill()
	case raw.PattFill != nil:
		p.Fill = raw.PattFill.ToDrawingPatternFill()
	case raw.BlipFill != nil && raw.BlipFill.Blip != nil && raw.BlipFill.Blip.Embed != "":
		pf := drawing.PictureFill{
			EmbedID: raw.BlipFill.Blip.Embed,
			Alpha:   1.0,
			Tile:    raw.BlipFill.Tile != nil,
			Stretch: raw.BlipFill.Stretch != nil || (raw.BlipFill.Tile == nil && raw.BlipFill.Stretch == nil),
		}
		if raw.BlipFill.Tile != nil {
			pf.TileOffsetX = int64(raw.BlipFill.Tile.Tx)
			pf.TileOffsetY = int64(raw.BlipFill.Tile.Ty)
			pf.TileScaleX = raw.BlipFill.Tile.Sx
			pf.TileScaleY = raw.BlipFill.Tile.Sy
			pf.TileFlip = raw.BlipFill.Tile.Flip
			pf.TileAlign = raw.BlipFill.Tile.Algn
		}
		if raw.BlipFill.SrcRect != nil {
			l := clampWpsBlipCropPct(raw.BlipFill.SrcRect.L)
			t := clampWpsBlipCropPct(raw.BlipFill.SrcRect.T)
			r := clampWpsBlipCropPct(raw.BlipFill.SrcRect.R)
			b := clampWpsBlipCropPct(raw.BlipFill.SrcRect.B)
			w := 1 - l - r
			h := 1 - t - b
			if w > 0 && h > 0 {
				pf.Crop = &drawing.Rect{X: l, Y: t, Width: w, Height: h}
			}
		}
		if b := raw.BlipFill.Blip; b != nil {
			if b.Lum != nil {
				pf.BlipEffects = append(pf.BlipEffects, drawing.Lum{Brightness: b.Lum.Bright, Contrast: b.Lum.Contrast})
			}
			if b.AlphaMod != nil {
				pf.BlipEffects = append(pf.BlipEffects, drawing.AlphaMod{Amount: b.AlphaMod.Val})
			}
			if b.BiLevel != nil {
				pf.BlipEffects = append(pf.BlipEffects, drawing.BiLevel{Threshold: b.BiLevel.Thresh})
			}
			if b.Grayscale != nil {
				pf.BlipEffects = append(pf.BlipEffects, drawing.Grayscale{})
			}
		}
		p.Fill = pf
	}
	if raw.Ln != nil {
		p.Line = raw.Ln.ToDrawingLineStyle()
	}
	return nil
}

// clampWpsBlipCropPct clamps a blipFill srcRect edge value (thousandths of a percent,
// range 0–100000) to a normalized float in [0, 1].
func clampWpsBlipCropPct(v int) float64 {
	if v <= 0 {
		return 0
	}
	if v >= 100000 {
		return 1
	}
	return float64(v) / 100000.0
}

func wpsDrawingEffectList(outer *wpsOuterShadowEffect, inner *wpsInnerShadowEffect, blur *wpsBlurEffect, glow *wpsGlowEffect, softEdge *wpsSoftEdgeEffect, reflection *wpsReflectionEffect, presetShadow *wpsPresetShadowEffect, alphaBiLevel *wpsBiLevelEffect, alphaMod *wpsAlphaEffect, alphaOff *wpsAlphaEffect, alphaModFix *wpsAlphaEffect, alphaOutset *wpsOutsetEffect, alphaRepl *wpsAlphaEffect, alphaInv *struct{}, alphaCeiling *struct{}, alphaFloor *struct{}, grayscale *struct{}, biLevel *wpsBiLevelEffect, lum *wpsLumEffect, hsl *wpsHSLEffect, deSat *struct{}, deSatLow *struct{}, deSatMod *wpsValueEffect, duotone *wpsDuotoneEffect, clrChange *wpsClrChangeEffect, clrRepl *wpsClrReplEffect, fillOverlay *wpsFillOverlayEffect, themeColors map[string]string) drawing.EffectList {
	var effects []drawing.Effect
	if outer != nil {
		color, alpha := wpsResolveEffectColor(outer.SrgbClr, outer.SchemeClr, themeColors)
		effects = append(effects, drawing.Shadow{
			Color:    color,
			Alpha:    alpha,
			Blur:     drawing.EMUToPoints(outer.BlurRad),
			Distance: drawing.EMUToPoints(outer.Dist),
			Angle:    float64(outer.Dir) / 60000.0,
			ScaleX:   1.0,
			ScaleY:   1.0,
		})
	}
	if presetShadow != nil {
		color, alpha := wpsResolveEffectColor(presetShadow.SrgbClr, presetShadow.SchemeClr, themeColors)
		effects = append(effects, drawing.Shadow{
			Color:    color,
			Alpha:    alpha,
			Blur:     wpsPresetShadowBlur(presetShadow.Prst),
			Distance: drawing.EMUToPoints(presetShadow.Dist),
			Angle:    float64(presetShadow.Dir) / 60000.0,
			ScaleX:   1.0,
			ScaleY:   1.0,
		})
	}
	if inner != nil {
		color, alpha := wpsResolveEffectColor(inner.SrgbClr, inner.SchemeClr, themeColors)
		effects = append(effects, drawing.InnerShadow{
			Color:    color,
			Alpha:    alpha,
			Blur:     drawing.EMUToPoints(inner.BlurRad),
			Distance: drawing.EMUToPoints(inner.Dist),
			Angle:    float64(inner.Dir) / 60000.0,
		})
	}
	if glow != nil {
		color, alpha := wpsResolveEffectColor(glow.SrgbClr, glow.SchemeClr, themeColors)
		effects = append(effects, drawing.Glow{
			Color:  color,
			Alpha:  alpha,
			Radius: drawing.EMUToPoints(glow.Rad),
		})
	}
	if softEdge != nil {
		effects = append(effects, drawing.SoftEdge{
			Radius: drawing.EMUToPoints(softEdge.Rad),
		})
	}
	if reflection != nil {
		alpha := 0.3
		if reflection.StA > 0 {
			alpha = float64(reflection.StA) / 100000.0
		}
		effects = append(effects, drawing.Reflection{
			Alpha:     alpha,
			Blur:      drawing.EMUToPoints(reflection.BlurRad),
			Distance:  drawing.EMUToPoints(reflection.Dist),
			Direction: float64(reflection.Dir) / 60000.0,
			Size:      1.0,
			Fade:      true,
		})
	}
	if blur != nil {
		effects = append(effects, drawing.Blur{
			Radius: drawing.EMUToPoints(blur.Rad),
		})
	}
	if alphaBiLevel != nil {
		effects = append(effects, drawing.AlphaBiLevel{Threshold: alphaBiLevel.Thresh})
	}
	if alphaMod != nil {
		effects = append(effects, drawing.AlphaMod{Amount: alphaMod.Val})
	}
	if alphaOff != nil {
		effects = append(effects, drawing.AlphaOff{Amount: alphaOff.Val})
	}
	if alphaModFix != nil {
		effects = append(effects, drawing.AlphaModFix{Amount: alphaModFix.Val})
	}
	if alphaOutset != nil {
		effects = append(effects, drawing.AlphaOutset{Radius: drawing.EMUToPoints(alphaOutset.Rad)})
	}
	if alphaRepl != nil {
		effects = append(effects, drawing.AlphaRepl{Alpha: alphaRepl.Val})
	}
	if alphaInv != nil {
		effects = append(effects, drawing.AlphaInv{})
	}
	if alphaCeiling != nil {
		effects = append(effects, drawing.AlphaCeiling{})
	}
	if alphaFloor != nil {
		effects = append(effects, drawing.AlphaFloor{})
	}
	if grayscale != nil {
		effects = append(effects, drawing.Grayscale{})
	}
	if biLevel != nil {
		effects = append(effects, drawing.BiLevel{Threshold: biLevel.Thresh})
	}
	if lum != nil {
		effects = append(effects, drawing.Lum{Brightness: lum.Bright, Contrast: lum.Contrast})
	}
	if hsl != nil {
		effects = append(effects, drawing.HSLEffect{Hue: hsl.Hue, Sat: hsl.Sat, Lum: hsl.Lum})
	}
	if deSat != nil {
		effects = append(effects, drawing.DeSat{})
	}
	if deSatLow != nil {
		effects = append(effects, drawing.DeSatLow{})
	}
	if deSatMod != nil {
		effects = append(effects, drawing.DeSatMod{Amount: deSatMod.Val})
	}
	if duotone != nil {
		color1, color2 := wpsResolveDuotoneColors(duotone, themeColors)
		if color1 != "" || color2 != "" {
			effects = append(effects, drawing.Duotone{Color1: color1, Color2: color2})
		}
	}
	if clrChange != nil {
		from := wpsResolveColorChoice(clrChange.ClrFrom, themeColors)
		to := wpsResolveColorChoice(clrChange.ClrTo, themeColors)
		if from != "" || to != "" {
			effects = append(effects, drawing.ClrChange{From: from, To: to})
		}
	}
	if clrRepl != nil {
		if color := wpsResolveColorRef(clrRepl.SRGBClr, clrRepl.SchemeClr, themeColors); color != "" {
			effects = append(effects, drawing.ClrRepl{Color: color})
		}
	}
	if fillOverlay != nil {
		if fill := wpsDrawingFill(fillOverlay, themeColors); fill != nil {
			effects = append(effects, drawing.FillOverlay{Fill: fill, BlendMode: fillOverlay.Blend})
		}
	}
	return drawing.EffectList{Effects: effects}
}

func (p *WpsShapeProperties) DrawingEffectList(themeColors map[string]string) drawing.EffectList {
	if p == nil {
		return drawing.EffectList{}
	}
	var effects []drawing.Effect
	if threeD := wpsDrawingThreeDEffect(p.Scene3D, p.Shape3D, themeColors); threeD != nil {
		effects = append(effects, *threeD)
	}
	if p.EffectDag != nil {
		effects = append(effects, wpsDrawingEffectDag(p.EffectDag, themeColors).Effects...)
		return drawing.EffectList{Effects: effects}
	}
	effects = append(effects, wpsDrawingEffectList(p.OuterShadow, p.InnerShadow, p.Blur, p.Glow, p.SoftEdge, p.Reflection, p.PresetShadow, p.AlphaBiLevel, p.AlphaMod, p.AlphaOff, p.AlphaModFix, p.AlphaOutset, p.AlphaRepl, p.AlphaInv, p.AlphaCeiling, p.AlphaFloor, p.Grayscale, p.BiLevel, p.Lum, p.HSL, p.DeSat, p.DeSatLow, p.DeSatMod, p.Duotone, p.ClrChange, p.ClrRepl, p.FillOverlay, themeColors).Effects...)
	return drawing.EffectList{Effects: effects}
}

func wpsDrawingEffectDag(container *wpsEffectContainerRaw, themeColors map[string]string) drawing.EffectList {
	registry := make(map[string]*wpsEffectContainerRaw)
	wpsCollectEffectContainers(container, registry)
	return drawing.EffectList{Effects: wpsDrawingEffectContainer(container, themeColors, registry, nil)}
}

func wpsCollectEffectContainers(container *wpsEffectContainerRaw, registry map[string]*wpsEffectContainerRaw) {
	if container == nil {
		return
	}
	if name := strings.TrimSpace(container.Name); name != "" {
		if _, exists := registry[name]; !exists {
			registry[name] = container
		}
	}
	for _, raw := range container.Effects {
		if raw.Container != nil {
			wpsCollectEffectContainers(raw.Container, registry)
		}
		if raw.AlphaMod != nil && raw.AlphaMod.Cont != nil {
			wpsCollectEffectContainers(raw.AlphaMod.Cont, registry)
		}
		if raw.Blend != nil && raw.Blend.Cont != nil {
			wpsCollectEffectContainers(raw.Blend.Cont, registry)
		}
	}
}

func wpsDrawingEffectContainer(container *wpsEffectContainerRaw, themeColors map[string]string, registry map[string]*wpsEffectContainerRaw, stack map[string]bool) []drawing.Effect {
	if container == nil {
		return nil
	}
	containerType := strings.TrimSpace(container.Type)
	if containerType == "" {
		containerType = "sib"
	}
	var effects []drawing.Effect
	for _, raw := range container.Effects {
		effects = append(effects, WpsDrawingEffectRaw(raw, themeColors, registry, stack, containerType)...)
	}
	return effects
}

// WpsDrawingEffectRaw converts a parsed wpsEffectRaw into a slice of drawing.Effect
// values.  It is exported so that theme effect styles can be resolved outside the
// parts package.
func WpsDrawingEffectRaw(raw wpsEffectRaw, themeColors map[string]string, registry map[string]*wpsEffectContainerRaw, stack map[string]bool, containerType string) []drawing.Effect {
	switch {
	case raw.Container != nil:
		return wpsDrawingEffectContainer(raw.Container, themeColors, registry, stack)
	case raw.Reference != nil:
		ref := strings.TrimSpace(raw.Reference.Ref)
		if ref == "" || registry == nil {
			return nil
		}
		target, ok := registry[ref]
		if !ok || target == nil {
			return nil
		}
		if stack != nil && stack[ref] {
			return nil
		}
		nextStack := make(map[string]bool, len(stack)+1)
		for name, seen := range stack {
			if seen {
				nextStack[name] = true
			}
		}
		nextStack[ref] = true
		return wpsDrawingEffectContainer(target, themeColors, registry, nextStack)
	case raw.AlphaMod != nil:
		effects := wpsDrawingEffectContainer(raw.AlphaMod.Cont, themeColors, registry, stack)
		if opacity, ok := drawing.ApproximateOpacity(drawing.EffectList{Effects: effects}); ok {
			return []drawing.Effect{drawing.AlphaModFix{
				Amount:        int(math.Round(opacity * 100000.0)),
				ContainerType: containerType,
			}}
		}
		return effects
	case raw.Blend != nil:
		effects := wpsDrawingEffectContainer(raw.Blend.Cont, themeColors, registry, stack)
		foundFillOverlay := false
		for i := range effects {
			switch fo := effects[i].(type) {
			case drawing.FillOverlay:
				fo.BlendMode = raw.Blend.Blend
				effects[i] = fo
				foundFillOverlay = true
			case *drawing.FillOverlay:
				if fo != nil {
					fo.BlendMode = raw.Blend.Blend
					foundFillOverlay = true
				}
			}
		}
		if !foundFillOverlay && raw.Blend.Blend != "" {
			effects = append(effects, drawing.BlendMode{Mode: raw.Blend.Blend, ContainerType: containerType})
		}
		return effects
	case raw.OuterShadow != nil:
		color, alpha := wpsResolveEffectColor(raw.OuterShadow.SrgbClr, raw.OuterShadow.SchemeClr, themeColors)
		return []drawing.Effect{drawing.Shadow{
			Color:         color,
			Alpha:         alpha,
			Blur:          drawing.EMUToPoints(raw.OuterShadow.BlurRad),
			Distance:      drawing.EMUToPoints(raw.OuterShadow.Dist),
			Angle:         float64(raw.OuterShadow.Dir) / 60000.0,
			ScaleX:        1.0,
			ScaleY:        1.0,
			ContainerType: containerType,
		}}
	case raw.PresetShadow != nil:
		color, alpha := wpsResolveEffectColor(raw.PresetShadow.SrgbClr, raw.PresetShadow.SchemeClr, themeColors)
		return []drawing.Effect{drawing.Shadow{
			Color:         color,
			Alpha:         alpha,
			Blur:          wpsPresetShadowBlur(raw.PresetShadow.Prst),
			Distance:      drawing.EMUToPoints(raw.PresetShadow.Dist),
			Angle:         float64(raw.PresetShadow.Dir) / 60000.0,
			ScaleX:        1.0,
			ScaleY:        1.0,
			ContainerType: containerType,
		}}
	case raw.InnerShadow != nil:
		color, alpha := wpsResolveEffectColor(raw.InnerShadow.SrgbClr, raw.InnerShadow.SchemeClr, themeColors)
		return []drawing.Effect{drawing.InnerShadow{
			Color:         color,
			Alpha:         alpha,
			Blur:          drawing.EMUToPoints(raw.InnerShadow.BlurRad),
			Distance:      drawing.EMUToPoints(raw.InnerShadow.Dist),
			Angle:         float64(raw.InnerShadow.Dir) / 60000.0,
			ContainerType: containerType,
		}}
	case raw.Glow != nil:
		color, alpha := wpsResolveEffectColor(raw.Glow.SrgbClr, raw.Glow.SchemeClr, themeColors)
		return []drawing.Effect{drawing.Glow{
			Color:         color,
			Alpha:         alpha,
			Radius:        drawing.EMUToPoints(raw.Glow.Rad),
			ContainerType: containerType,
		}}
	case raw.SoftEdge != nil:
		return []drawing.Effect{drawing.SoftEdge{Radius: drawing.EMUToPoints(raw.SoftEdge.Rad), ContainerType: containerType}}
	case raw.Reflection != nil:
		alpha := 0.3
		if raw.Reflection.StA > 0 {
			alpha = float64(raw.Reflection.StA) / 100000.0
		}
		return []drawing.Effect{drawing.Reflection{
			Alpha:     alpha,
			Blur:      drawing.EMUToPoints(raw.Reflection.BlurRad),
			Distance:  drawing.EMUToPoints(raw.Reflection.Dist),
			Direction: float64(raw.Reflection.Dir) / 60000.0,
			Size:      1.0,
			Fade:      true,
		}}
	case raw.Blur != nil:
		return []drawing.Effect{drawing.Blur{Radius: drawing.EMUToPoints(raw.Blur.Rad), ContainerType: containerType}}
	case raw.AlphaBiLevel != nil:
		return []drawing.Effect{drawing.AlphaBiLevel{
			Threshold:     raw.AlphaBiLevel.Thresh,
			ContainerType: containerType,
		}}
	case raw.AlphaModFix != nil:
		return []drawing.Effect{drawing.AlphaModFix{
			Amount:        raw.AlphaModFix.Val,
			ContainerType: containerType,
		}}
	case raw.AlphaOutset != nil:
		return []drawing.Effect{drawing.AlphaOutset{
			Radius:        drawing.EMUToPoints(raw.AlphaOutset.Rad),
			ContainerType: containerType,
		}}
	case raw.AlphaRepl != nil:
		return []drawing.Effect{drawing.AlphaRepl{
			Alpha:         raw.AlphaRepl.Val,
			ContainerType: containerType,
		}}
	case raw.AlphaInv != nil:
		return []drawing.Effect{drawing.AlphaInv{ContainerType: containerType}}
	case raw.AlphaCeiling != nil:
		return []drawing.Effect{drawing.AlphaCeiling{ContainerType: containerType}}
	case raw.AlphaFloor != nil:
		return []drawing.Effect{drawing.AlphaFloor{ContainerType: containerType}}
	case raw.AlphaOff != nil:
		return []drawing.Effect{drawing.AlphaOff{
			Amount:        raw.AlphaOff.Val,
			ContainerType: containerType,
		}}
	case raw.Grayscale != nil:
		return []drawing.Effect{drawing.Grayscale{}}
	case raw.DeSat != nil:
		return []drawing.Effect{drawing.DeSat{}}
	case raw.DeSatLow != nil:
		return []drawing.Effect{drawing.DeSatLow{}}
	case raw.DeSatMod != nil:
		return []drawing.Effect{drawing.DeSatMod{Amount: raw.DeSatMod.Val}}
	case raw.BiLevel != nil:
		return []drawing.Effect{drawing.BiLevel{Threshold: raw.BiLevel.Thresh}}
	case raw.Lum != nil:
		return []drawing.Effect{drawing.Lum{Brightness: raw.Lum.Bright, Contrast: raw.Lum.Contrast}}
	case raw.HSL != nil:
		return []drawing.Effect{drawing.HSLEffect{Hue: raw.HSL.Hue, Sat: raw.HSL.Sat, Lum: raw.HSL.Lum}}
	case raw.Duotone != nil:
		color1, color2 := wpsResolveDuotoneColors(raw.Duotone, themeColors)
		if color1 == "" && color2 == "" {
			return nil
		}
		return []drawing.Effect{drawing.Duotone{Color1: color1, Color2: color2}}
	case raw.ClrChange != nil:
		from := wpsResolveColorChoice(raw.ClrChange.ClrFrom, themeColors)
		to := wpsResolveColorChoice(raw.ClrChange.ClrTo, themeColors)
		if from == "" && to == "" {
			return nil
		}
		return []drawing.Effect{drawing.ClrChange{From: from, To: to}}
	case raw.ClrRepl != nil:
		color := wpsResolveColorRef(raw.ClrRepl.SRGBClr, raw.ClrRepl.SchemeClr, themeColors)
		if color == "" {
			return nil
		}
		return []drawing.Effect{drawing.ClrRepl{Color: color}}
	case raw.Fill != nil:
		fill := wpsDrawingFillEffect(raw.Fill, themeColors)
		if fill == nil {
			return nil
		}
		return []drawing.Effect{drawing.FillOverlay{Fill: fill, BlendMode: "over", ContainerType: containerType}}
	case raw.FillOverlay != nil:
		fill := wpsDrawingFill(raw.FillOverlay, themeColors)
		if fill == nil {
			return nil
		}
		return []drawing.Effect{drawing.FillOverlay{Fill: fill, BlendMode: raw.FillOverlay.Blend, ContainerType: containerType}}
	case raw.RelOff != nil:
		return []drawing.Effect{drawing.RelativeOffset{
			X:             float64(raw.RelOff.Tx) / 100000.0,
			Y:             float64(raw.RelOff.Ty) / 100000.0,
			ContainerType: containerType,
		}}
	case raw.Tint != nil:
		return []drawing.Effect{drawing.Tint{Hue: raw.Tint.Hue, Amount: raw.Tint.Amt, ContainerType: containerType}}
	case raw.Transform != nil:
		return []drawing.Effect{drawing.TransformEffect{
			ScaleX:        wpsPercentOrDefault(raw.Transform.Sx, 1.0),
			ScaleY:        wpsPercentOrDefault(raw.Transform.Sy, 1.0),
			SkewX:         float64(raw.Transform.Kx) / 60000.0,
			SkewY:         float64(raw.Transform.Ky) / 60000.0,
			TranslateX:    drawing.EMUToPoints(raw.Transform.Tx),
			TranslateY:    drawing.EMUToPoints(raw.Transform.Ty),
			ContainerType: containerType,
		}}
	default:
		return nil
	}
}

func wpsPercentOrDefault(v int, def float64) float64 {
	if v == 0 {
		return def
	}
	return float64(v) / 100000.0
}

func wpsDrawingThreeDEffect(scene *wpsScene3D, shape3D *wpsShape3DRaw, themeColors map[string]string) *drawing.ThreeD {
	if scene == nil && shape3D == nil {
		return nil
	}
	effect := &drawing.ThreeD{}
	if scene != nil {
		if scene.Camera != nil {
			effect.CameraPreset = scene.Camera.Prst
			if scene.Camera.Rot != nil {
				effect.RotationX = float64(scene.Camera.Rot.Lat) / 60000.0
				effect.RotationY = float64(scene.Camera.Rot.Lon) / 60000.0
				effect.RotationZ = float64(scene.Camera.Rot.Rev) / 60000.0
			}
		}
		if scene.LightRig != nil {
			effect.LightRig = scene.LightRig.Rig
			effect.LightDirection = scene.LightRig.Dir
		}
	}
	if shape3D != nil {
		effect.Material = shape3D.PrstMaterial
		effect.ExtrusionDepth = drawing.EMUToPoints(wpsMaxInt64(shape3D.ExtrusionH, shape3D.Z))
		effect.ContourWidth = drawing.EMUToPoints(shape3D.ContourW)
		if shape3D.BevelT != nil {
			effect.BevelTopWidth = drawing.EMUToPoints(shape3D.BevelT.W)
			effect.BevelTopHeight = drawing.EMUToPoints(shape3D.BevelT.H)
		}
		if shape3D.BevelB != nil {
			effect.BevelBottomWidth = drawing.EMUToPoints(shape3D.BevelB.W)
			effect.BevelBottomHeight = drawing.EMUToPoints(shape3D.BevelB.H)
		}
		effect.ContourColor = wpsResolve3DColor(shape3D.ContourClr, themeColors)
		effect.ExtrusionColor = wpsResolve3DColor(shape3D.ExtrusionClr, themeColors)
	}
	if effect.CameraPreset == "" && effect.LightRig == "" && effect.LightDirection == "" && effect.Material == "" && effect.RotationX == 0 && effect.RotationY == 0 && effect.RotationZ == 0 && effect.ExtrusionDepth == 0 && effect.ContourWidth == 0 && effect.BevelTopHeight == 0 && effect.BevelBottomHeight == 0 {
		return nil
	}
	return effect
}

func wpsResolve3DColor(ref *wpsShape3DColorRef, themeColors map[string]string) string {
	if ref == nil {
		return ""
	}
	if ref.SRGBClr != nil && ref.SRGBClr.Val != "" {
		return ref.SRGBClr.Val
	}
	if ref.SchemeClr != nil && themeColors != nil {
		if color := themeColors[ref.SchemeClr.Val]; color != "" {
			return color
		}
	}
	return ""
}

func wpsMaxInt64(values ...int64) int64 {
	var max int64
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

func wpsResolveEffectColor(srgb *wpsEffectColorRef, scheme *wpsEffectSchemeColor, themeColors map[string]string) (string, float64) {
	if srgb != nil {
		return srgb.Val, wpsEffectAlphaValue(srgb.Alpha)
	}
	if scheme != nil {
		color := "000000"
		if themeColors != nil {
			if resolved := themeColors[scheme.Val]; resolved != "" {
				color = resolved
			}
		}
		return color, wpsEffectAlphaValue(scheme.Alpha)
	}
	return "000000", 1.0
}

func wpsEffectAlphaValue(alpha *wpsEffectAlpha) float64 {
	if alpha == nil || alpha.Val <= 0 {
		return 1.0
	}
	return float64(alpha.Val) / 100000.0
}

func wpsPresetShadowBlur(prst string) float64 {
	switch strings.TrimSpace(strings.ToLower(prst)) {
	case "shdw1", "shdw2":
		return 1.5
	case "shdw3", "shdw4", "shdw5":
		return 2.5
	default:
		return 2.0
	}
}

func drawingThemeSRGBColor(src *drawingxml.SRGBClr) *ThemeSRGBColor {
	if src == nil {
		return nil
	}
	return &ThemeSRGBColor{
		Val: src.Val,
		ThemeColorTransforms: ThemeColorTransforms{
			Alpha:    drawingColorTransform(src.Alpha),
			AlphaMod: drawingColorTransform(src.AlphaMod),
			AlphaOff: drawingColorTransform(src.AlphaOff),
			Shade:    drawingColorTransform(src.Shade),
			Tint:     drawingColorTransform(src.Tint),
			Red:      drawingColorTransform(src.Red),
			RedMod:   drawingColorTransform(src.RedMod),
			RedOff:   drawingColorTransform(src.RedOff),
			Green:    drawingColorTransform(src.Green),
			GreenMod: drawingColorTransform(src.GreenMod),
			GreenOff: drawingColorTransform(src.GreenOff),
			Blue:     drawingColorTransform(src.Blue),
			BlueMod:  drawingColorTransform(src.BlueMod),
			BlueOff:  drawingColorTransform(src.BlueOff),
			Hue:      drawingColorTransform(src.Hue),
			Sat:      drawingColorTransform(src.Sat),
			Lum:      drawingColorTransform(src.Lum),
			LumMod:   drawingColorTransform(src.LumMod),
			LumOff:   drawingColorTransform(src.LumOff),
			SatMod:   drawingColorTransform(src.SatMod),
			SatOff:   drawingColorTransform(src.SatOff),
			HueMod:   drawingColorTransform(src.HueMod),
			HueOff:   drawingColorTransform(src.HueOff),
			Gray:     src.Gray,
			Comp:     src.Comp,
			Inv:      src.Inv,
			Gamma:    src.Gamma,
			InvGamma: src.InvGamma,
		},
	}
}

func drawingSchemeColor(src *drawingxml.SchemeClr) *DrawingSchemeClr {
	if src == nil {
		return nil
	}
	return &DrawingSchemeClr{
		Val:      src.Val,
		Alpha:    drawingColorTransform(src.Alpha),
		AlphaMod: drawingColorTransform(src.AlphaMod),
		AlphaOff: drawingColorTransform(src.AlphaOff),
		Shade:    drawingColorTransform(src.Shade),
		Tint:     drawingColorTransform(src.Tint),
		Red:      drawingColorTransform(src.Red),
		RedMod:   drawingColorTransform(src.RedMod),
		RedOff:   drawingColorTransform(src.RedOff),
		Green:    drawingColorTransform(src.Green),
		GreenMod: drawingColorTransform(src.GreenMod),
		GreenOff: drawingColorTransform(src.GreenOff),
		Blue:     drawingColorTransform(src.Blue),
		BlueMod:  drawingColorTransform(src.BlueMod),
		BlueOff:  drawingColorTransform(src.BlueOff),
		Hue:      drawingColorTransform(src.Hue),
		Sat:      drawingColorTransform(src.Sat),
		Lum:      drawingColorTransform(src.Lum),
		LumMod:   drawingColorTransform(src.LumMod),
		LumOff:   drawingColorTransform(src.LumOff),
		SatMod:   drawingColorTransform(src.SatMod),
		SatOff:   drawingColorTransform(src.SatOff),
		HueMod:   drawingColorTransform(src.HueMod),
		HueOff:   drawingColorTransform(src.HueOff),
		Gray:     src.Gray,
		Comp:     src.Comp,
		Inv:      src.Inv,
		Gamma:    src.Gamma,
		InvGamma: src.InvGamma,
	}
}

func drawingColorTransform(src *drawingxml.ColorTransform) *DrawingColorTransform {
	if src == nil {
		return nil
	}
	return &DrawingColorTransform{Val: src.Val}
}

func wpsDrawingFill(fill *wpsFillOverlayEffect, themeColors map[string]string) drawing.Fill {
	if fill == nil {
		return nil
	}
	if fill.NoFill != nil {
		return &drawing.NoFill{}
	}
	if fill.GradFill != nil {
		if grad := drawingxml.ResolveGradientFill(fill.GradFill, themeColors); grad != nil {
			return grad
		}
	}
	if fill.PattFill != nil {
		if pattern := drawingxml.ResolvePatternFill(fill.PattFill, themeColors); pattern != nil {
			return pattern
		}
	}
	if fill.SolidFill != nil {
		return drawingxml.ResolveSolidFill(fill.SolidFill, themeColors)
	}
	if fill.BlipFill != nil {
		if picture := drawingxml.ResolveBlipFill(wpsBlipFillToDrawingXML(fill.BlipFill)); picture != nil {
			return picture
		}
	}
	return nil
}

func wpsDrawingFillEffect(fill *wpsFillEffect, themeColors map[string]string) drawing.Fill {
	if fill == nil {
		return nil
	}
	return wpsDrawingFill(&wpsFillOverlayEffect{
		SolidFill: fill.SolidFill,
		GradFill:  fill.GradFill,
		NoFill:    fill.NoFill,
		PattFill:  fill.PattFill,
		BlipFill:  fill.BlipFill,
	}, themeColors)
}

func wpsBlipFillToDrawingXML(fill *wpsBlipFillRaw) *drawingxml.BlipFill {
	if fill == nil {
		return nil
	}
	result := &drawingxml.BlipFill{}
	if fill.Blip != nil {
		result.Blip = &drawingxml.BlipRef{Embed: fill.Blip.Embed}
	}
	if fill.SrcRect != nil {
		result.SrcRect = &drawingxml.SrcRect{
			L: fill.SrcRect.L,
			T: fill.SrcRect.T,
			R: fill.SrcRect.R,
			B: fill.SrcRect.B,
		}
	}
	if fill.Tile != nil {
		result.Tile = &drawingxml.Tile{
			Tx:   fill.Tile.Tx,
			Ty:   fill.Tile.Ty,
			Sx:   fill.Tile.Sx,
			Sy:   fill.Tile.Sy,
			Flip: fill.Tile.Flip,
			Algn: fill.Tile.Algn,
		}
	}
	if fill.Stretch != nil {
		result.Stretch = &drawingxml.Stretch{}
	}
	return result
}

func wpsResolveDuotoneColors(effect *wpsDuotoneEffect, themeColors map[string]string) (string, string) {
	if effect == nil {
		return "", ""
	}
	colors := make([]string, 0, 2)
	for _, color := range effect.SRGBClrs {
		if color != nil && color.Val != "" {
			colors = append(colors, color.Val)
			if len(colors) == 2 {
				return colors[0], colors[1]
			}
		}
	}
	for _, color := range effect.SchemeClrs {
		if color == nil || color.Val == "" {
			continue
		}
		if resolved := wpsResolveColorRef(nil, color, themeColors); resolved != "" {
			colors = append(colors, resolved)
			if len(colors) == 2 {
				return colors[0], colors[1]
			}
		}
	}
	if len(colors) == 1 {
		return colors[0], ""
	}
	if len(colors) >= 2 {
		return colors[0], colors[1]
	}
	return "", ""
}

func wpsResolveColorChoice(choice *wpsColorChoiceRaw, themeColors map[string]string) string {
	if choice == nil {
		return ""
	}
	return wpsResolveColorRef(choice.SRGBClr, choice.SchemeClr, themeColors)
}

func wpsResolveColorRef(srgb *ThemeSRGBColor, scheme *DrawingSchemeClr, themeColors map[string]string) string {
	if srgb != nil && srgb.Val != "" {
		return srgb.Val
	}
	if scheme != nil && themeColors != nil {
		if color := themeColors[scheme.Val]; color != "" {
			return color
		}
	}
	return ""
}

func geometryGuidesToDrawing(src []wpsGeometryGuideRaw) []drawing.Guide {
	return drawingxml.DrawingGuidesFromGeometryGuides(src)
}

// DrawingSolidFill stores a solid color — either a direct sRGB value or a theme slot reference.
type DrawingSolidFill struct {
	SRGB      *ThemeSRGBColor   `xml:"srgbClr"`
	SchemeClr *DrawingSchemeClr `xml:"schemeClr"`
}

// DrawingSchemeClr references a named DrawingML theme color slot (e.g. "accent1", "dk1").
type DrawingSchemeClr struct {
	Val      string                 `xml:"val,attr"`
	Alpha    *DrawingColorTransform `xml:"alpha"`
	AlphaMod *DrawingColorTransform `xml:"alphaMod"`
	AlphaOff *DrawingColorTransform `xml:"alphaOff"`
	Shade    *DrawingColorTransform `xml:"shade"`
	Tint     *DrawingColorTransform `xml:"tint"`
	Red      *DrawingColorTransform `xml:"red"`
	RedMod   *DrawingColorTransform `xml:"redMod"`
	RedOff   *DrawingColorTransform `xml:"redOff"`
	Green    *DrawingColorTransform `xml:"green"`
	GreenMod *DrawingColorTransform `xml:"greenMod"`
	GreenOff *DrawingColorTransform `xml:"greenOff"`
	Blue     *DrawingColorTransform `xml:"blue"`
	BlueMod  *DrawingColorTransform `xml:"blueMod"`
	BlueOff  *DrawingColorTransform `xml:"blueOff"`
	Hue      *DrawingColorTransform `xml:"hue"`
	Sat      *DrawingColorTransform `xml:"sat"`
	Lum      *DrawingColorTransform `xml:"lum"`
	LumMod   *DrawingColorTransform `xml:"lumMod"`
	LumOff   *DrawingColorTransform `xml:"lumOff"`
	SatMod   *DrawingColorTransform `xml:"satMod"`
	SatOff   *DrawingColorTransform `xml:"satOff"`
	HueMod   *DrawingColorTransform `xml:"hueMod"`
	HueOff   *DrawingColorTransform `xml:"hueOff"`
	Gray     *struct{}              `xml:"gray"`
	Comp     *struct{}              `xml:"comp"`
	Inv      *struct{}              `xml:"inv"`
	Gamma    *struct{}              `xml:"gamma"`
	InvGamma *struct{}              `xml:"invGamma"`
}

// DrawingColorTransform stores a DrawingML color-transform value such as
// <a:lumMod val="95000"/> or <a:lumOff val="5000"/>.
type DrawingColorTransform struct {
	Val int `xml:"val,attr"`
}

// WpsTextBox represents wps:txbx — the text frame of a WordprocessingShape.
type WpsTextBox struct {
	Content *TextBoxContent `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main txbxContent"`
	ExtLst  *ExtensionList  `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape extLst"`
}

// TextBoxContent represents w:txbxContent — ordered paragraphs and tables inside a text box.
type TextBoxContent struct {
	Paragraphs  []*Paragraph
	Tables      []*Table
	Blocks      []ContentBlock
	Diagnostics diag.List
}

// AlternateContent represents mc:AlternateContent — a markup-compatibility choice wrapper.
type AlternateContent struct {
	Choices  []*AlternateChoice `xml:"Choice"`
	Fallback *AlternateFallback `xml:"Fallback"`
}

// AlternateChoice represents mc:Choice — one alternative in mc:AlternateContent.
type AlternateChoice struct {
	Requires string   `xml:"Requires,attr"`
	Drawing  *Drawing `xml:"drawing"`
}

// AlternateFallback represents mc:Fallback — legacy VML content for pre-OOXML consumers.
// ECMA-376 Part 4 §14 (VML).
type AlternateFallback struct {
	VMLDrawing
}

// VMLDrawing captures the VML element families that can appear inside a
// WordprocessingML w:pict or mc:Fallback container.
type VMLDrawing struct {
	Backgrounds []VMLBackground `xml:"background"`
	Groups      []VMLGroup      `xml:"group"`
	Shapes      []VMLShape      `xml:"shape"`
	ShapeTypes  []VMLShapeType  `xml:"shapetype"`
	Rects       []VMLRect       `xml:"rect"`
	RoundRects  []VMLRoundRect  `xml:"roundrect"`
	Ovals       []VMLOval       `xml:"oval"`
	Lines       []VMLLine       `xml:"line"`
	Arcs        []VMLArc        `xml:"arc"`
	Curves      []VMLCurve      `xml:"curve"`
	Polylines   []VMLPolyline   `xml:"polyline"`
	Images      []VMLImage      `xml:"image"`
}

// VMLShapeCommon stores the attribute/child subset shared by the main VML
// shape containers used in Word fallback content.
// VMLSignatureLine represents the o:signatureline element inside a VML shape.
type VMLSignatureLine struct {
	XMLName                xml.Name `xml:"urn:schemas-microsoft-com:office:office signatureline"`
	SetupID                string   `xml:"id,attr,omitempty"`
	SuggestedSigner        string   `xml:"suggestedsigner,attr,omitempty"`
	SuggestedSigner2       string   `xml:"suggestedsigner2,attr,omitempty"`
	SuggestedSignerEmail   string   `xml:"suggestedsigneremail,attr,omitempty"`
	SigningInstructions    string   `xml:"signinginstructions,attr,omitempty"`
	AllowComments          string   `xml:"allowcomments,attr,omitempty"`
	ShowSignDate           string   `xml:"showsigndate,attr,omitempty"`
	IsSignatureLine        string   `xml:"issignatureline,attr,omitempty"`
	ProviderID             string   `xml:"provid,attr,omitempty"`
	SigningInstructionsSet string   `xml:"signinginstructionsset,attr,omitempty"`
	AdditionalXML          string   `xml:"addlxml,attr,omitempty"`
	ProviderURL            string   `xml:"sigprovurl,attr,omitempty"`
}

type VMLShapeCommon struct {
	ID            string            `xml:"id,attr,omitempty"`
	Type          string            `xml:"type,attr,omitempty"`
	Style         string            `xml:"style,attr,omitempty"`
	FillColor     string            `xml:"fillcolor,attr,omitempty"`
	StrokeColor   string            `xml:"strokecolor,attr,omitempty"`
	StrokeWeight  string            `xml:"strokeweight,attr,omitempty"`
	Filled        string            `xml:"filled,attr,omitempty"`
	Stroked       string            `xml:"stroked,attr,omitempty"`
	From          string            `xml:"from,attr,omitempty"`
	To            string            `xml:"to,attr,omitempty"`
	Points        string            `xml:"points,attr,omitempty"`
	StartAngle    string            `xml:"startangle,attr,omitempty"`
	EndAngle      string            `xml:"endangle,attr,omitempty"`
	PathRef       string            `xml:"path,attr,omitempty"`
	ArcSize       string            `xml:"arcsize,attr,omitempty"`
	CoordSize     string            `xml:"coordsize,attr,omitempty"`
	CoordOrigin   string            `xml:"coordorigin,attr,omitempty"`
	HR            string            `xml:"hr,attr,omitempty"`
	HRAlign       string            `xml:"hralign,attr,omitempty"`
	ImageData     *VMLImageData     `xml:"imagedata"`
	TextBox       *VMLTextBox       `xml:"textbox"`
	Fill          *VMLFill          `xml:"fill"`
	Stroke        *VMLStroke        `xml:"stroke"`
	Shadow        *VMLShadow        `xml:"shadow"`
	Path          *VMLPath          `xml:"path"`
	TextPath      *VMLTextPath      `xml:"textpath"`
	Formulas      *VMLFormulas      `xml:"formulas"`
	Handles       *VMLHandles       `xml:"handles"`
	SignatureLine *VMLSignatureLine `xml:"signatureline"`
	UnknownAttrs  []xml.Attr        `xml:",any,attr"`
	Unknown       []RawElement      `xml:",any"`
}

// VMLShape represents v:shape — a VML shape element.
type VMLShape struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml shape"`
	VMLShapeCommon
}

// VMLBackground represents v:background.
type VMLBackground struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml background"`
	VMLShapeCommon
}

// VMLGroup represents v:group — a container of VML shapes.
type VMLGroup struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml group"`
	VMLShapeCommon
	VMLDrawing
}

// VMLShapeType represents v:shapetype.
type VMLShapeType struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml shapetype"`
	VMLShapeCommon
}

// VMLRect represents v:rect.
type VMLRect struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml rect"`
	VMLShapeCommon
}

// VMLRoundRect represents v:roundrect.
type VMLRoundRect struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml roundrect"`
	VMLShapeCommon
}

// VMLOval represents v:oval.
type VMLOval struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml oval"`
	VMLShapeCommon
}

// VMLLine represents v:line.
type VMLLine struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml line"`
	VMLShapeCommon
}

// VMLArc represents v:arc.
type VMLArc struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml arc"`
	VMLShapeCommon
}

// VMLCurve represents v:curve.
type VMLCurve struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml curve"`
	VMLShapeCommon
}

// VMLPolyline represents v:polyline.
type VMLPolyline struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml polyline"`
	VMLShapeCommon
}

// VMLImage represents v:image.
type VMLImage struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml image"`
	VMLShapeCommon
}

// VMLTextBox represents v:textbox — a VML text frame that may contain
// WordprocessingML textbox content.
type VMLTextBox struct {
	XMLName xml.Name        `xml:"urn:schemas-microsoft-com:vml textbox"`
	Style   string          `xml:"style,attr,omitempty"`
	Content *TextBoxContent `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main txbxContent"`
}

// VMLFill represents v:fill.
type VMLFill struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml fill"`
	RelID   string   `xml:"id,attr,omitempty"`
	Type    string   `xml:"type,attr,omitempty"`
	On      string   `xml:"on,attr,omitempty"`
	Color   string   `xml:"color,attr,omitempty"`
	Color2  string   `xml:"color2,attr,omitempty"`
}

// VMLStroke represents v:stroke.
type VMLStroke struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml stroke"`
	On      string   `xml:"on,attr,omitempty"`
	Color   string   `xml:"color,attr,omitempty"`
	Weight  string   `xml:"weight,attr,omitempty"`
}

// VMLShadow represents v:shadow.
type VMLShadow struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml shadow"`
	On      string   `xml:"on,attr,omitempty"`
	Color   string   `xml:"color,attr,omitempty"`
}

// VMLPath represents v:path.
type VMLPath struct {
	XMLName    xml.Name `xml:"urn:schemas-microsoft-com:vml path"`
	V          string   `xml:"v,attr,omitempty"`
	TextPathOK string   `xml:"textpathok,attr,omitempty"`
}

// VMLTextPath represents v:textpath.
type VMLTextPath struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml textpath"`
	On      string   `xml:"on,attr,omitempty"`
	String  string   `xml:"string,attr,omitempty"`
}

// VMLFormulas represents v:formulas.
type VMLFormulas struct {
	XMLName xml.Name     `xml:"urn:schemas-microsoft-com:vml formulas"`
	Entries []VMLFormula `xml:"f"`
}

// VMLFormula represents v:f.
type VMLFormula struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml f"`
	Eqn     string   `xml:"eqn,attr,omitempty"`
}

// VMLHandles represents v:handles.
type VMLHandles struct {
	XMLName xml.Name    `xml:"urn:schemas-microsoft-com:vml handles"`
	Entries []VMLHandle `xml:"h"`
}

// VMLHandle represents v:h.
type VMLHandle struct {
	XMLName  xml.Name `xml:"urn:schemas-microsoft-com:vml h"`
	Position string   `xml:"position,attr,omitempty"`
}

// VMLImageData represents v:imagedata — a VML image reference.
// The RelID attribute corresponds to r:id and identifies the image relationship.
type VMLImageData struct {
	RelID string `xml:"id,attr,omitempty"`
}

// Picture represents pic:pic.
type Picture struct {
	NvPicPr    *PictureNvProps    `xml:"nvPicPr"`
	BlipFill   *BlipFill          `xml:"blipFill"`
	ShapeProps *PictureShapeProps `xml:"http://schemas.openxmlformats.org/drawingml/2006/picture spPr"`
}

// PictureNvProps represents pic:nvPicPr — non-visual picture properties.
type PictureNvProps struct {
	CNvPr    *DrawingDocPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/picture cNvPr"`
	CNvPicPr *CNvPicPr     `xml:"cNvPicPr"`
}

// BlipFill represents pic:blipFill.
type BlipFill struct {
	Blip    *Blip               `xml:"blip"`
	Tile    *drawingxml.Tile    `xml:"tile"`
	Stretch *drawingxml.Stretch `xml:"stretch"`
	SrcRect *SrcRect            `xml:"srcRect"`
}

// Blip represents a:blip — the image reference.
// The Embed attribute (r:embed) is the relationship ID of the image part.
type Blip struct {
	Embed       string           `xml:"embed,attr,omitempty"`
	Lum         *BlipLum         `xml:"lum"`
	AlphaMod    *BlipAlpha       `xml:"alphaMod"`
	AlphaModFix *BlipAlphaModFix `xml:"alphaModFix"`
	AlphaRepl   *BlipAlphaRepl   `xml:"alphaRepl"`
	BiLevel     *BlipBiLevel     `xml:"biLevel"`
	Grayscale   *struct{}        `xml:"grayscl"`
}

// BlipLum represents a:lum inside a:blip — brightness/contrast adjustment.
type BlipLum struct {
	Bright   int `xml:"bright,attr,omitempty"`   // thousandths of a percent
	Contrast int `xml:"contrast,attr,omitempty"` // thousandths of a percent
}

// BlipAlpha represents a:alphaMod inside a:blip — alpha modulation.
type BlipAlpha struct {
	Val int `xml:"val,attr,omitempty"` // thousandths of a percent
}

// BlipAlphaModFix represents a:alphaModFix inside a:blip.
type BlipAlphaModFix struct {
	Amt int `xml:"amt,attr,omitempty"` // thousandths of a percent
}

// BlipAlphaRepl represents a:alphaRepl inside a:blip.
type BlipAlphaRepl struct {
	A int `xml:"a,attr,omitempty"` // thousandths of a percent
}

// BlipBiLevel represents a:biLevel inside a:blip — black/white threshold.
type BlipBiLevel struct {
	Thresh int `xml:"thresh,attr,omitempty"` // thousandths of a percent
}

// SrcRect stores picture crop percentages in thousandths of a percent.
type SrcRect struct {
	L int `xml:"l,attr,omitempty"`
	T int `xml:"t,attr,omitempty"`
	R int `xml:"r,attr,omitempty"`
	B int `xml:"b,attr,omitempty"`
}

// PictureShapeProps preserves the subset of pic:spPr used by Word picture rendering.
type PictureShapeProps struct {
	Xfrm         *DrawingTransform2D      `xml:"xfrm"`
	NoFill       *struct{}                `xml:"noFill"`
	SolidFill    *drawingxml.SolidFill    `xml:"solidFill"`
	GradFill     *drawingxml.GradientFill `xml:"gradFill"`
	PattFill     *drawingxml.PatternFill  `xml:"pattFill"`
	Ln           *drawingxml.Line         `xml:"ln"`
	EffectDag    *wpsEffectContainerRaw   `xml:"effectDag"`
	OuterShdw    *wpsOuterShadowEffect    `xml:"effectLst>outerShdw"`
	InnerShdw    *wpsInnerShadowEffect    `xml:"effectLst>innerShdw"`
	Blur         *wpsBlurEffect           `xml:"effectLst>blur"`
	Glow         *wpsGlowEffect           `xml:"effectLst>glow"`
	SoftEdge     *wpsSoftEdgeEffect       `xml:"effectLst>softEdge"`
	Reflection   *wpsReflectionEffect     `xml:"effectLst>reflection"`
	PresetShdw   *wpsPresetShadowEffect   `xml:"effectLst>prstShdw"`
	AlphaBiLevel *wpsBiLevelEffect        `xml:"effectLst>alphaBiLevel"`
	AlphaMod     *wpsAlphaEffect          `xml:"effectLst>alphaMod"`
	AlphaOff     *wpsAlphaEffect          `xml:"effectLst>alphaOff"`
	AlphaModFix  *wpsAlphaEffect          `xml:"effectLst>alphaModFix"`
	AlphaOutset  *wpsOutsetEffect         `xml:"effectLst>alphaOutset"`
	AlphaRepl    *wpsAlphaEffect          `xml:"effectLst>alphaRepl"`
	AlphaInv     *struct{}                `xml:"effectLst>alphaInv"`
	AlphaCeiling *struct{}                `xml:"effectLst>alphaCeiling"`
	AlphaFloor   *struct{}                `xml:"effectLst>alphaFloor"`
	Grayscale    *struct{}                `xml:"effectLst>grayscl"`
	BiLevel      *wpsBiLevelEffect        `xml:"effectLst>biLevel"`
	Lum          *wpsLumEffect            `xml:"effectLst>lum"`
	HSL          *wpsHSLEffect            `xml:"effectLst>hsl"`
	DeSat        *struct{}                `xml:"effectLst>deSat"`
	DeSatLow     *struct{}                `xml:"effectLst>deSatLow"`
	DeSatMod     *wpsValueEffect          `xml:"effectLst>deSatMod"`
	Duotone      *wpsDuotoneEffect        `xml:"effectLst>duotone"`
	ClrChange    *wpsClrChangeEffect      `xml:"effectLst>clrChange"`
	ClrRepl      *wpsClrReplEffect        `xml:"effectLst>clrRepl"`
	FillOverlay  *wpsFillOverlayEffect    `xml:"effectLst>fillOverlay"`
}

// DrawingEffectList converts the parsed picture shape properties effects into a
// drawing.EffectList, resolving theme colors where needed.
func (p *PictureShapeProps) DrawingEffectList(themeColors map[string]string) drawing.EffectList {
	if p == nil {
		return drawing.EffectList{}
	}
	var effects []drawing.Effect
	if p.EffectDag != nil {
		effects = append(effects, wpsDrawingEffectDag(p.EffectDag, themeColors).Effects...)
		return drawing.EffectList{Effects: effects}
	}
	effects = append(effects, wpsDrawingEffectList(p.OuterShdw, p.InnerShdw, p.Blur, p.Glow, p.SoftEdge, p.Reflection, p.PresetShdw, p.AlphaBiLevel, p.AlphaMod, p.AlphaOff, p.AlphaModFix, p.AlphaOutset, p.AlphaRepl, p.AlphaInv, p.AlphaCeiling, p.AlphaFloor, p.Grayscale, p.BiLevel, p.Lum, p.HSL, p.DeSat, p.DeSatLow, p.DeSatMod, p.Duotone, p.ClrChange, p.ClrRepl, p.FillOverlay, themeColors).Effects...)
	return drawing.EffectList{Effects: effects}
}

// DrawingTransform2D stores picture flip/rotation attributes from a:xfrm.
type DrawingTransform2D struct {
	Rot     int   `xml:"rot,attr,omitempty"` // 1/60000 degree clockwise rotation
	FlipH   bool  `xml:"flipH,attr,omitempty"`
	FlipV   bool  `xml:"flipV,attr,omitempty"`
	OffX    int64 `xml:"-"`
	OffY    int64 `xml:"-"`
	ExtCX   int64 `xml:"-"`
	ExtCY   int64 `xml:"-"`
	ChOffX  int64 `xml:"-"`
	ChOffY  int64 `xml:"-"`
	ChExtCX int64 `xml:"-"`
	ChExtCY int64 `xml:"-"`
}

type rawTransform2D struct {
	XMLName xml.Name `xml:"xfrm"`
	Rot     int      `xml:"rot,attr,omitempty"`
	FlipH   bool     `xml:"flipH,attr,omitempty"`
	FlipV   bool     `xml:"flipV,attr,omitempty"`
	Off     struct {
		X int64 `xml:"x,attr"`
		Y int64 `xml:"y,attr"`
	} `xml:"off"`
	Ext struct {
		CX int64 `xml:"cx,attr"`
		CY int64 `xml:"cy,attr"`
	} `xml:"ext"`
	ChOff struct {
		X int64 `xml:"x,attr"`
		Y int64 `xml:"y,attr"`
	} `xml:"chOff"`
	ChExt struct {
		CX int64 `xml:"cx,attr"`
		CY int64 `xml:"cy,attr"`
	} `xml:"chExt"`
}

func (d *DrawingTransform2D) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var raw rawTransform2D
	if err := dec.DecodeElement(&raw, &start); err != nil {
		return err
	}
	d.Rot = raw.Rot
	d.FlipH = raw.FlipH
	d.FlipV = raw.FlipV
	d.OffX = raw.Off.X
	d.OffY = raw.Off.Y
	d.ExtCX = raw.Ext.CX
	d.ExtCY = raw.Ext.CY
	d.ChOffX = raw.ChOff.X
	d.ChOffY = raw.ChOff.Y
	d.ChExtCX = raw.ChExt.CX
	d.ChExtCY = raw.ChExt.CY
	return nil
}

// MarshalXML emits a:xfrm with optional a:off, a:ext, a:chOff, and a:chExt children.
func (d DrawingTransform2D) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "a:xfrm"}
	if d.Rot != 0 {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "rot"}, Value: fmt.Sprintf("%d", d.Rot)})
	}
	if d.FlipH {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "flipH"}, Value: "1"})
	}
	if d.FlipV {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "flipV"}, Value: "1"})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "a:off"}, Attr: []xml.Attr{
		{Name: xml.Name{Local: "x"}, Value: fmt.Sprintf("%d", d.OffX)},
		{Name: xml.Name{Local: "y"}, Value: fmt.Sprintf("%d", d.OffY)},
	}}); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "a:off"}}); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "a:ext"}, Attr: []xml.Attr{
		{Name: xml.Name{Local: "cx"}, Value: fmt.Sprintf("%d", d.ExtCX)},
		{Name: xml.Name{Local: "cy"}, Value: fmt.Sprintf("%d", d.ExtCY)},
	}}); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "a:ext"}}); err != nil {
		return err
	}
	if d.ChExtCX != 0 || d.ChExtCY != 0 || d.ChOffX != 0 || d.ChOffY != 0 {
		if err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "a:chOff"}, Attr: []xml.Attr{
			{Name: xml.Name{Local: "x"}, Value: fmt.Sprintf("%d", d.ChOffX)},
			{Name: xml.Name{Local: "y"}, Value: fmt.Sprintf("%d", d.ChOffY)},
		}}); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "a:chOff"}}); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "a:chExt"}, Attr: []xml.Attr{
			{Name: xml.Name{Local: "cx"}, Value: fmt.Sprintf("%d", d.ChExtCX)},
			{Name: xml.Name{Local: "cy"}, Value: fmt.Sprintf("%d", d.ChExtCY)},
		}}); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "a:chExt"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "a:xfrm"}})
}

// MediaPart holds the raw bytes and content type of a loaded media part.
type MediaPart struct {
	ContentType string
	Data        []byte
}

// BoolVal is a generic boolean attr wrapper used by style defaults.
type BoolVal bool

// UnmarshalXMLAttr implements xml.UnmarshalerAttr.
func (b *BoolVal) UnmarshalXMLAttr(attr xml.Attr) error {
	switch attr.Value {
	case "", "true", "1", "on":
		*b = true
	default:
		*b = false
	}
	return nil
}

// UnmarshalXML preserves top-level body block order while also filling convenience slices.
func (b *Body) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*b = Body{}

	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}

		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "p":
				var paragraph Paragraph
				if err := dec.DecodeElement(&paragraph, &current); err != nil {
					return err
				}
				b.Paragraphs = append(b.Paragraphs, &paragraph)
				b.Blocks = append(b.Blocks, BodyBlock{
					Kind:      BodyBlockParagraph,
					Paragraph: &paragraph,
				})
			case "tbl":
				var table Table
				if err := dec.DecodeElement(&table, &current); err != nil {
					return err
				}
				b.Tables = append(b.Tables, &table)
				b.Blocks = append(b.Blocks, BodyBlock{
					Kind:  BodyBlockTable,
					Table: &table,
				})
			case "sectPr":
				var section SectionProperties
				if err := dec.DecodeElement(&section, &current); err != nil {
					return err
				}
				b.Section = &section
				b.Blocks = append(b.Blocks, BodyBlock{
					Kind:    BodyBlockSection,
					Section: &section,
				})
			case "sdt":
				sdtBlock, err := decodeSdtBodyBlocks(dec, current, &b.Diagnostics)
				if err != nil {
					return err
				}
				b.Paragraphs = append(b.Paragraphs, collectBodyBlockParagraphs(sdtBlock)...)
				b.Tables = append(b.Tables, collectBodyBlockTables(sdtBlock)...)
				b.Blocks = append(b.Blocks, sdtBlock)
			case "bookmarkEnd":
				if err := dec.Skip(); err != nil {
					return err
				}
			case "altChunk":
				var chunk AltChunk
				if err := dec.DecodeElement(&chunk, &current); err != nil {
					return err
				}
				b.AltChunks = append(b.AltChunks, &chunk)
				b.Blocks = append(b.Blocks, BodyBlock{
					Kind:     BodyBlockAltChunk,
					AltChunk: &chunk,
				})
			case "customXml":
				cx, blocks, err := decodeCustomXmlBodyBlocks(dec, current, &b.Diagnostics)
				if err != nil {
					return err
				}
				for _, blk := range blocks {
					b.Paragraphs = append(b.Paragraphs, collectBodyBlockParagraphs(blk)...)
					b.Tables = append(b.Tables, collectBodyBlockTables(blk)...)
					if blk.Kind == BodyBlockAltChunk && blk.AltChunk != nil {
						b.AltChunks = append(b.AltChunks, blk.AltChunk)
					}
				}
				b.Blocks = append(b.Blocks, BodyBlock{
					Kind:            BodyBlockCustomXml,
					CustomXml:       cx,
					CustomXmlBlocks: blocks,
				})
			case "bibliography":
				if err := dec.Skip(); err != nil {
					return err
				}
			default:
				recordUnsupportedMarkup(&b.Diagnostics, "body", current.Name.Local)
				var raw RawElement
				if err := dec.DecodeElement(&raw, &current); err != nil {
					return err
				}
				b.Blocks = append(b.Blocks, BodyBlock{Kind: BodyBlockRaw, Raw: &raw})
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// MarshalXML emits w:body respecting Blocks ordering when available.
func (b Body) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	wNS := "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	start.Name = xml.Name{Space: wNS, Local: "body"}
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	useBlocks := false
	if len(b.Blocks) > 0 {
		var inspectBodyBlocks func(blocks []BodyBlock) (paraCount, tblCount int, structured bool)
		inspectBodyBlocks = func(blocks []BodyBlock) (paraCount, tblCount int, structured bool) {
			for _, block := range blocks {
				switch block.Kind {
				case BodyBlockParagraph:
					paraCount++
				case BodyBlockTable:
					tblCount++
				case BodyBlockSection:
					// sectPr is representable through the convenience Section field.
				case BodyBlockSDT:
					p, t, childStructured := inspectBodyBlocks(block.SdtBlocks)
					paraCount += p
					tblCount += t
					structured = true || childStructured
				case BodyBlockAltChunk, BodyBlockCustomXml, BodyBlockRaw:
					structured = true
					if block.Kind == BodyBlockCustomXml {
						p, t, childStructured := inspectBodyBlocks(block.CustomXmlBlocks)
						paraCount += p
						tblCount += t
						structured = true || childStructured
					}
				}
			}
			return
		}
		paraCount, tblCount, structured := inspectBodyBlocks(b.Blocks)
		// Use Blocks when it matches the convenience slices, or when the
		// convenience slices are empty (caller is using Blocks exclusively).
		// Richer block kinds like altChunk/customXml/raw must also preserve
		// explicit block ordering even when the paragraph/table counts match.
		useBlocks = structured ||
			(len(b.Paragraphs) == 0 && len(b.Tables) == 0) ||
			(len(b.Paragraphs) == paraCount && len(b.Tables) == tblCount)
	}

	if useBlocks {
		for _, block := range b.Blocks {
			switch block.Kind {
			case BodyBlockParagraph:
				if block.Paragraph != nil {
					if err := e.EncodeElement(block.Paragraph, xml.StartElement{Name: xml.Name{Space: wNS, Local: "p"}}); err != nil {
						return err
					}
				}
			case BodyBlockTable:
				if block.Table != nil {
					if err := e.EncodeElement(block.Table, xml.StartElement{Name: xml.Name{Space: wNS, Local: "tbl"}}); err != nil {
						return err
					}
				}
			case BodyBlockSection:
				if block.Section != nil {
					if err := e.EncodeElement(block.Section, xml.StartElement{Name: xml.Name{Space: wNS, Local: "sectPr"}}); err != nil {
						return err
					}
				}
			case BodyBlockSDT:
				if err := marshalSdtBodyBlock(e, block, wNS); err != nil {
					return err
				}
			case BodyBlockAltChunk:
				if block.AltChunk != nil {
					if err := e.EncodeElement(block.AltChunk, xml.StartElement{Name: xml.Name{Space: wNS, Local: "altChunk"}}); err != nil {
						return err
					}
				}
			case BodyBlockCustomXml:
				if err := marshalCustomXmlBodyBlock(e, block, wNS); err != nil {
					return err
				}
			case BodyBlockRaw:
				if block.Raw != nil {
					if err := e.Encode(block.Raw); err != nil {
						return err
					}
				}
			}
		}
	} else {
		for _, p := range b.Paragraphs {
			if err := e.EncodeElement(p, xml.StartElement{Name: xml.Name{Space: wNS, Local: "p"}}); err != nil {
				return err
			}
		}
		for _, t := range b.Tables {
			if err := e.EncodeElement(t, xml.StartElement{Name: xml.Name{Space: wNS, Local: "tbl"}}); err != nil {
				return err
			}
		}
	}

	// Emit trailing sectPr if it wasn't already emitted as the last block.
	if b.Section != nil {
		trailingEmitted := len(b.Blocks) > 0 && b.Blocks[len(b.Blocks)-1].Kind == BodyBlockSection && b.Blocks[len(b.Blocks)-1].Section == b.Section
		if !trailingEmitted {
			if err := e.EncodeElement(b.Section, xml.StartElement{Name: xml.Name{Space: wNS, Local: "sectPr"}}); err != nil {
				return err
			}
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func marshalCustomXmlBodyBlock(e *xml.Encoder, block BodyBlock, wNS string) error {
	if block.CustomXml == nil {
		return nil
	}
	start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "customXml"}}
	if block.CustomXml.URI != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Space: wNS, Local: "uri"}, Value: block.CustomXml.URI})
	}
	if block.CustomXml.Element != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Space: wNS, Local: "element"}, Value: block.CustomXml.Element})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if block.CustomXml.Properties != nil {
		if err := e.EncodeElement(block.CustomXml.Properties, xml.StartElement{Name: xml.Name{Space: wNS, Local: "customXmlPr"}}); err != nil {
			return err
		}
	}
	for _, child := range block.CustomXmlBlocks {
		switch child.Kind {
		case BodyBlockParagraph:
			if child.Paragraph != nil {
				if err := e.EncodeElement(child.Paragraph, xml.StartElement{Name: xml.Name{Space: wNS, Local: "p"}}); err != nil {
					return err
				}
			}
		case BodyBlockTable:
			if child.Table != nil {
				if err := e.EncodeElement(child.Table, xml.StartElement{Name: xml.Name{Space: wNS, Local: "tbl"}}); err != nil {
					return err
				}
			}
		case BodyBlockSDT:
			if err := marshalSdtBodyBlock(e, child, wNS); err != nil {
				return err
			}
		case BodyBlockAltChunk:
			if child.AltChunk != nil {
				if err := e.EncodeElement(child.AltChunk, xml.StartElement{Name: xml.Name{Space: wNS, Local: "altChunk"}}); err != nil {
					return err
				}
			}
		case BodyBlockCustomXml:
			if err := marshalCustomXmlBodyBlock(e, child, wNS); err != nil {
				return err
			}
		case BodyBlockRaw:
			if child.Raw != nil {
				if err := e.Encode(child.Raw); err != nil {
					return err
				}
			}
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func marshalSdtBodyBlock(e *xml.Encoder, block BodyBlock, wNS string) error {
	if block.SdtProps == nil {
		return nil
	}
	start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "sdt"}}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := marshalSdtProperties(e, block.SdtProps, wNS); err != nil {
		return err
	}
	contentStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: "sdtContent"}}
	if err := e.EncodeToken(contentStart); err != nil {
		return err
	}
	for _, child := range block.SdtBlocks {
		switch child.Kind {
		case BodyBlockParagraph:
			if child.Paragraph != nil {
				if err := e.EncodeElement(child.Paragraph, xml.StartElement{Name: xml.Name{Space: wNS, Local: "p"}}); err != nil {
					return err
				}
			}
		case BodyBlockTable:
			if child.Table != nil {
				if err := e.EncodeElement(child.Table, xml.StartElement{Name: xml.Name{Space: wNS, Local: "tbl"}}); err != nil {
					return err
				}
			}
		case BodyBlockSDT:
			if err := marshalSdtBodyBlock(e, child, wNS); err != nil {
				return err
			}
		}
	}
	if err := e.EncodeToken(xml.EndElement{Name: contentStart.Name}); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func marshalSdtProperties(e *xml.Encoder, props *SdtProperties, wNS string) error {
	const w14NS = "http://schemas.microsoft.com/office/word/2010/wordml"
	const w15NS = "http://schemas.microsoft.com/office/word/2012/wordml"
	start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "sdtPr"}}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if props.ID != 0 {
		if err := e.EncodeElement(&IntVal{Val: props.ID}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "id"}}); err != nil {
			return err
		}
	}
	if props.Alias != "" {
		if err := e.EncodeElement(&StringVal{Val: props.Alias}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "alias"}}); err != nil {
			return err
		}
	}
	if props.Tag != "" {
		if err := e.EncodeElement(&StringVal{Val: props.Tag}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "tag"}}); err != nil {
			return err
		}
	}
	switch props.Type {
	case "plainText":
		textStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: "text"}}
		if props.PlainTextMultiLine {
			textStart.Attr = append(textStart.Attr, xml.Attr{Name: xml.Name{Space: wNS, Local: "multiLine"}, Value: "1"})
		}
		if err := e.EncodeElement(&struct{}{}, textStart); err != nil {
			return err
		}
	case "richText", "picture":
		if err := e.EncodeElement(&struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: props.Type}}); err != nil {
			return err
		}
	case "docPartObj":
		typeStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: "docPartObj"}}
		if err := e.EncodeToken(typeStart); err != nil {
			return err
		}
		if props.DocPartGallery != "" {
			if err := e.EncodeElement(&StringVal{Val: props.DocPartGallery}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "docPartGallery"}}); err != nil {
				return err
			}
		}
		if props.DocPartCategory != "" {
			if err := e.EncodeElement(&StringVal{Val: props.DocPartCategory}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "docPartCategory"}}); err != nil {
				return err
			}
		}
		if props.DocPartUnique {
			if err := e.EncodeElement(&struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "docPartUnique"}}); err != nil {
				return err
			}
		}
		if err := e.EncodeToken(xml.EndElement{Name: typeStart.Name}); err != nil {
			return err
		}
	case "checkBox":
		if props.Checked14 {
			// Modern w14:checkbox
			typeStart := xml.StartElement{Name: xml.Name{Space: w14NS, Local: "checkbox"}}
			if err := e.EncodeToken(typeStart); err != nil {
				return err
			}
			if props.Checked14 {
				v := BoolVal(true)
				if err := e.EncodeElement(&OnOff{Val: &v}, xml.StartElement{Name: xml.Name{Space: w14NS, Local: "checked"}}); err != nil {
					return err
				}
			}
			if err := e.EncodeToken(xml.EndElement{Name: typeStart.Name}); err != nil {
				return err
			}
		} else {
			// Legacy w:checkBox
			typeStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: "checkBox"}}
			if err := e.EncodeToken(typeStart); err != nil {
				return err
			}
			if props.Checked {
				v := BoolVal(true)
				if err := e.EncodeElement(&OnOff{Val: &v}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "checked"}}); err != nil {
					return err
				}
			}
			if err := e.EncodeToken(xml.EndElement{Name: typeStart.Name}); err != nil {
				return err
			}
		}
	case "date":
		typeStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: "date"}}
		if props.DateFullDate != "" {
			typeStart.Attr = append(typeStart.Attr, xml.Attr{Name: xml.Name{Space: wNS, Local: "fullDate"}, Value: props.DateFullDate})
		}
		if err := e.EncodeToken(typeStart); err != nil {
			return err
		}
		if props.DateFormat != "" {
			if err := e.EncodeElement(&StringVal{Val: props.DateFormat}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "dateFormat"}}); err != nil {
				return err
			}
		}
		if props.DateLanguage != "" {
			if err := e.EncodeElement(&StringVal{Val: props.DateLanguage}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "lid"}}); err != nil {
				return err
			}
		}
		if props.DateCalendar != "" {
			if err := e.EncodeElement(&StringVal{Val: props.DateCalendar}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "calendar"}}); err != nil {
				return err
			}
		}
		if err := e.EncodeToken(xml.EndElement{Name: typeStart.Name}); err != nil {
			return err
		}
	case "dropDownList", "comboBox":
		typeStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: props.Type}}
		if err := e.EncodeToken(typeStart); err != nil {
			return err
		}
		for _, item := range props.ListItems {
			if err := e.EncodeElement(&SdtListItem{DisplayText: item.DisplayText, Value: item.Value}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "listItem"}}); err != nil {
				return err
			}
		}
		if err := e.EncodeToken(xml.EndElement{Name: typeStart.Name}); err != nil {
			return err
		}
	case "":
		// No explicit SDT content type.
	default:
		if err := e.EncodeElement(&struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: props.Type}}); err != nil {
			return err
		}
	}
	if props.Lock != "" {
		if err := e.EncodeElement(&StringVal{Val: props.Lock}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "lock"}}); err != nil {
			return err
		}
	}
	if props.Temporary {
		v := BoolVal(true)
		if err := e.EncodeElement(&OnOff{Val: &v}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "temporary"}}); err != nil {
			return err
		}
	}
	if props.Appearance != "" {
		if err := e.EncodeElement(&StringVal{Val: props.Appearance}, xml.StartElement{Name: xml.Name{Space: w15NS, Local: "appearance"}}); err != nil {
			return err
		}
	}
	if props.Color != "" {
		if err := e.EncodeElement(&StringVal{Val: props.Color}, xml.StartElement{Name: xml.Name{Space: w15NS, Local: "color"}}); err != nil {
			return err
		}
	}
	if props.ShowingPlcHdr {
		v := BoolVal(true)
		if err := e.EncodeElement(&OnOff{Val: &v}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "showingPlcHdr"}}); err != nil {
			return err
		}
	}
	if props.PlaceholderDocPart != "" || props.PlaceholderDocPartGallery != "" {
		phStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: "placeholder"}}
		if err := e.EncodeToken(phStart); err != nil {
			return err
		}
		if props.PlaceholderDocPart != "" {
			if err := e.EncodeElement(&StringVal{Val: props.PlaceholderDocPart}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "docPart"}}); err != nil {
				return err
			}
		}
		if props.PlaceholderDocPartGallery != "" {
			if err := e.EncodeElement(&StringVal{Val: props.PlaceholderDocPartGallery}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "docPartGallery"}}); err != nil {
				return err
			}
		}
		if err := e.EncodeToken(xml.EndElement{Name: phStart.Name}); err != nil {
			return err
		}
	}
	if props.DataXPath != "" || props.DataStoreID != "" {
		binding := SdtDataBinding{XPath: props.DataXPath, StoreItemID: props.DataStoreID}
		if err := e.EncodeElement(binding, xml.StartElement{Name: xml.Name{Space: wNS, Local: "dataBinding"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// SdtListItem represents a single w:listItem inside w:sdtPr.
type SdtListItem struct {
	DisplayText string `xml:"displayText,attr"`
	Value       string `xml:"value,attr"`
}

// SdtDataBinding represents w:dataBinding inside w:sdtPr.
type SdtDataBinding struct {
	XPath       string `xml:"xpath,attr"`
	StoreItemID string `xml:"storeItemID,attr"`
}

// UnmarshalXML preserves ordered runs and hyperlinks inside a paragraph.
func (p *Paragraph) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*p = Paragraph{}

	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}

		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "pPr":
				var properties ParagraphProperties
				if err := dec.DecodeElement(&properties, &current); err != nil {
					return err
				}
				p.Properties = &properties
			case "r":
				var run Run
				if err := dec.DecodeElement(&run, &current); err != nil {
					return err
				}
				p.Runs = append(p.Runs, &run)
				p.Bookmarks = append(p.Bookmarks, run.BookmarkStarts...)
				p.CommentRangeStarts = append(p.CommentRangeStarts, run.CommentRangeStarts...)
				p.CommentRangeEnds = append(p.CommentRangeEnds, run.CommentRangeEnds...)
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind: ParagraphInlineRun,
					Run:  &run,
				})
			case "hyperlink":
				var hyperlink Hyperlink
				if err := dec.DecodeElement(&hyperlink, &current); err != nil {
					return err
				}
				p.Hyperlinks = append(p.Hyperlinks, &hyperlink)
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind:      ParagraphInlineHyperlink,
					Hyperlink: &hyperlink,
				})
			case "sdt":
				sdtInline, err := decodeSdtParagraphInlines(dec, current, &p.Diagnostics)
				if err != nil {
					return err
				}
				p.Inlines = append(p.Inlines, sdtInline)
			case "ins":
				// Tracked insertion: include inner runs marked as inserted.
				if err := decodeInsElement(dec, current, p); err != nil {
					return err
				}
			case "del":
				// Tracked deletion: preserve inner runs marked as deleted.
				if err := decodeDelElement(dec, current, p); err != nil {
					return err
				}
			case "moveFrom":
				if err := decodeMoveFromElement(dec, current, p); err != nil {
					return err
				}
			case "moveTo":
				if err := decodeMoveToElement(dec, current, p); err != nil {
					return err
				}
			case "dir":
				if err := decodeDirElement(dec, current, p); err != nil {
					return err
				}
			case "bdo":
				if err := decodeBdoElement(dec, current, p); err != nil {
					return err
				}
			case "smartTag":
				if err := decodeSmartTagElement(dec, current, p); err != nil {
					return err
				}
			case "bookmarkStart":
				var bm BookmarkStart
				for _, attr := range current.Attr {
					switch attr.Name.Local {
					case "name":
						bm.Name = attr.Value
						p.Bookmarks = append(p.Bookmarks, attr.Value)
					case "id":
						if id, err := strconv.Atoi(attr.Value); err == nil {
							bm.ID = id
						}
					}
				}
				p.BookmarkStartElems = append(p.BookmarkStartElems, bm)
				if err := dec.Skip(); err != nil {
					return err
				}
			case "commentRangeStart":
				var crs CommentRangeStart
				for _, attr := range current.Attr {
					if attr.Name.Local == "id" {
						if id, err := strconv.Atoi(attr.Value); err == nil {
							crs.ID = id
							p.CommentRangeStarts = append(p.CommentRangeStarts, id)
						}
						break
					}
				}
				p.CommentRangeStartElems = append(p.CommentRangeStartElems, crs)
				if err := dec.Skip(); err != nil {
					return err
				}
			case "commentRangeEnd":
				var cre CommentRangeEnd
				for _, attr := range current.Attr {
					if attr.Name.Local == "id" {
						if id, err := strconv.Atoi(attr.Value); err == nil {
							cre.ID = id
							p.CommentRangeEnds = append(p.CommentRangeEnds, id)
						}
						break
					}
				}
				p.CommentRangeEndElems = append(p.CommentRangeEndElems, cre)
				if err := dec.Skip(); err != nil {
					return err
				}
			case "bookmarkEnd":
				var bm BookmarkEnd
				for _, attr := range current.Attr {
					if attr.Name.Local == "id" {
						if id, err := strconv.Atoi(attr.Value); err == nil {
							bm.ID = id
						}
						break
					}
				}
				p.BookmarkEndElems = append(p.BookmarkEndElems, bm)
				if err := dec.Skip(); err != nil {
					return err
				}
			case "moveFromRangeStart":
				var mrs MoveRangeStart
				if err := dec.DecodeElement(&mrs, &current); err != nil {
					return err
				}
				p.MoveFromRangeStarts = append(p.MoveFromRangeStarts, mrs)
			case "moveFromRangeEnd":
				var mre MoveRangeEnd
				if err := dec.DecodeElement(&mre, &current); err != nil {
					return err
				}
				p.MoveFromRangeEnds = append(p.MoveFromRangeEnds, mre)
			case "moveToRangeStart":
				var mrs MoveRangeStart
				if err := dec.DecodeElement(&mrs, &current); err != nil {
					return err
				}
				p.MoveToRangeStarts = append(p.MoveToRangeStarts, mrs)
			case "moveToRangeEnd":
				var mre MoveRangeEnd
				if err := dec.DecodeElement(&mre, &current); err != nil {
					return err
				}
				p.MoveToRangeEnds = append(p.MoveToRangeEnds, mre)
			case "proofErr":
				if err := dec.Skip(); err != nil {
					return err
				}
			case "fldSimple":
				var fld Field
				for _, attr := range current.Attr {
					if attr.Name.Local == "instr" {
						fld.Instr = attr.Value
						break
					}
				}
				// Decode inner runs from fldSimple content.
				for {
					tok2, err := dec.Token()
					if err != nil {
						return err
					}
					switch t2 := tok2.(type) {
					case xml.StartElement:
						if t2.Name.Local == "r" {
							var run Run
							if err := dec.DecodeElement(&run, &t2); err != nil {
								return err
							}
							fld.Runs = append(fld.Runs, &run)
						} else {
							recordUnsupportedMarkup(&p.Diagnostics, "fldSimple", t2.Name.Local)
							if err := dec.Skip(); err != nil {
								return err
							}
						}
					case xml.EndElement:
						if t2.Name == current.Name {
							goto doneFldSimple
						}
					}
				}
			doneFldSimple:
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind:  ParagraphInlineField,
					Field: &fld,
				})
			case "ruby":
				// w:ruby — East Asian phonetic guide (furigana).
				// ECMA-376 Part 1 §17.3.3.26.
				ruby, err := decodeRubyElement(dec, current)
				if err != nil {
					return err
				}
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind: ParagraphInlineRuby,
					Ruby: ruby,
				})
			case "oMath", "oMathPara":
				// OMML math equation — capture raw XML for later conversion to MathML.
				rawXML, err := captureRawXMLElement(dec, current)
				if err != nil {
					return err
				}
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind: ParagraphInlineMath,
					Math: &OMMath{
						RawXML:  rawXML,
						Display: current.Name.Local == "oMathPara",
					},
				})
			case "customXml":
				cx, inlines, err := decodeCustomXmlParagraphInlines(dec, current, p)
				if err != nil {
					return err
				}
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind:             ParagraphInlineCustomXml,
					CustomXml:        cx,
					CustomXmlInlines: inlines,
				})
			case "permStart":
				var ps PermStart
				for _, attr := range current.Attr {
					switch attr.Name.Local {
					case "id":
						if id, err := strconv.Atoi(attr.Value); err == nil {
							ps.ID = id
						}
					case "edGrp":
						ps.EdGrp = attr.Value
					case "ed":
						ps.Ed = attr.Value
					case "colFirst":
						if id, err := strconv.Atoi(attr.Value); err == nil {
							ps.ColFirst = id
						}
					case "colLast":
						if id, err := strconv.Atoi(attr.Value); err == nil {
							ps.ColLast = id
						}
					}
				}
				p.PermStarts = append(p.PermStarts, ps)
				if err := dec.Skip(); err != nil {
					return err
				}
			case "permEnd":
				var pe PermEnd
				for _, attr := range current.Attr {
					if attr.Name.Local == "id" {
						if id, err := strconv.Atoi(attr.Value); err == nil {
							pe.ID = id
						}
						break
					}
				}
				p.PermEnds = append(p.PermEnds, pe)
				if err := dec.Skip(); err != nil {
					return err
				}
			case "customXmlInsRangeStart":
				var mrs MoveRangeStart
				if err := dec.DecodeElement(&mrs, &current); err != nil {
					return err
				}
				p.CustomXmlInsRangeStarts = append(p.CustomXmlInsRangeStarts, mrs)
			case "customXmlInsRangeEnd":
				var mre MoveRangeEnd
				if err := dec.DecodeElement(&mre, &current); err != nil {
					return err
				}
				p.CustomXmlInsRangeEnds = append(p.CustomXmlInsRangeEnds, mre)
			case "customXmlDelRangeStart":
				var mrs MoveRangeStart
				if err := dec.DecodeElement(&mrs, &current); err != nil {
					return err
				}
				p.CustomXmlDelRangeStarts = append(p.CustomXmlDelRangeStarts, mrs)
			case "customXmlDelRangeEnd":
				var mre MoveRangeEnd
				if err := dec.DecodeElement(&mre, &current); err != nil {
					return err
				}
				p.CustomXmlDelRangeEnds = append(p.CustomXmlDelRangeEnds, mre)
			case "customXmlMoveFromRangeStart":
				var mrs MoveRangeStart
				if err := dec.DecodeElement(&mrs, &current); err != nil {
					return err
				}
				p.CustomXmlMoveFromRangeStarts = append(p.CustomXmlMoveFromRangeStarts, mrs)
			case "customXmlMoveFromRangeEnd":
				var mre MoveRangeEnd
				if err := dec.DecodeElement(&mre, &current); err != nil {
					return err
				}
				p.CustomXmlMoveFromRangeEnds = append(p.CustomXmlMoveFromRangeEnds, mre)
			case "customXmlMoveToRangeStart":
				var mrs MoveRangeStart
				if err := dec.DecodeElement(&mrs, &current); err != nil {
					return err
				}
				p.CustomXmlMoveToRangeStarts = append(p.CustomXmlMoveToRangeStarts, mrs)
			case "customXmlMoveToRangeEnd":
				var mre MoveRangeEnd
				if err := dec.DecodeElement(&mre, &current); err != nil {
					return err
				}
				p.CustomXmlMoveToRangeEnds = append(p.CustomXmlMoveToRangeEnds, mre)
			default:
				recordUnsupportedMarkup(&p.Diagnostics, "paragraph", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// MarshalXML emits w:p respecting Inlines ordering when available.
func (p Paragraph) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	wNS := "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	start.Name = xml.Name{Space: wNS, Local: "p"}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if p.Properties != nil {
		if err := e.EncodeElement(p.Properties, xml.StartElement{Name: xml.Name{Space: wNS, Local: "pPr"}}); err != nil {
			return err
		}
	}
	if len(p.Inlines) > 0 {
		for _, inline := range p.Inlines {
			if err := marshalParagraphInline(e, inline, wNS); err != nil {
				return err
			}
		}
	} else {
		for _, r := range p.Runs {
			if err := e.EncodeElement(r, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
				return err
			}
		}
		for _, hl := range p.Hyperlinks {
			if err := e.EncodeElement(hl, xml.StartElement{Name: xml.Name{Space: wNS, Local: "hyperlink"}}); err != nil {
				return err
			}
		}
	}
	for _, bm := range p.BookmarkStartElems {
		if err := e.EncodeElement(bm, xml.StartElement{Name: xml.Name{Space: wNS, Local: "bookmarkStart"}}); err != nil {
			return err
		}
	}
	for _, bm := range p.BookmarkEndElems {
		if err := e.EncodeElement(bm, xml.StartElement{Name: xml.Name{Space: wNS, Local: "bookmarkEnd"}}); err != nil {
			return err
		}
	}
	for _, crs := range p.CommentRangeStartElems {
		if err := e.EncodeElement(crs, xml.StartElement{Name: xml.Name{Space: wNS, Local: "commentRangeStart"}}); err != nil {
			return err
		}
	}
	for _, cre := range p.CommentRangeEndElems {
		if err := e.EncodeElement(cre, xml.StartElement{Name: xml.Name{Space: wNS, Local: "commentRangeEnd"}}); err != nil {
			return err
		}
	}
	for _, mrs := range p.MoveFromRangeStarts {
		if err := e.EncodeElement(mrs, xml.StartElement{Name: xml.Name{Space: wNS, Local: "moveFromRangeStart"}}); err != nil {
			return err
		}
	}
	for _, mre := range p.MoveFromRangeEnds {
		if err := e.EncodeElement(mre, xml.StartElement{Name: xml.Name{Space: wNS, Local: "moveFromRangeEnd"}}); err != nil {
			return err
		}
	}
	for _, mrs := range p.MoveToRangeStarts {
		if err := e.EncodeElement(mrs, xml.StartElement{Name: xml.Name{Space: wNS, Local: "moveToRangeStart"}}); err != nil {
			return err
		}
	}
	for _, mre := range p.MoveToRangeEnds {
		if err := e.EncodeElement(mre, xml.StartElement{Name: xml.Name{Space: wNS, Local: "moveToRangeEnd"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func marshalParagraphInline(e *xml.Encoder, inline ParagraphInline, wNS string) error {
	switch inline.Kind {
	case ParagraphInlineRun:
		if inline.Run != nil {
			return e.EncodeElement(inline.Run, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}})
		}
	case ParagraphInlineHyperlink:
		if inline.Hyperlink != nil {
			return e.EncodeElement(inline.Hyperlink, xml.StartElement{Name: xml.Name{Space: wNS, Local: "hyperlink"}})
		}
	case ParagraphInlineField:
		if inline.Field != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "fldSimple"}}
			if inline.Field.Instr != "" {
				start.Attr = []xml.Attr{{Name: xml.Name{Local: "instr"}, Value: inline.Field.Instr}}
			}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			for _, r := range inline.Field.Runs {
				if err := e.EncodeElement(r, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
					return err
				}
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	case ParagraphInlineIns:
		if inline.Run != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "ins"}}
			if inline.RevisionAuthor != "" {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "author"}, Value: inline.RevisionAuthor})
			}
			if inline.RevisionDate != "" {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "date"}, Value: inline.RevisionDate})
			}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			if err := e.EncodeElement(inline.Run, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
				return err
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	case ParagraphInlineDel:
		if inline.Run != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "del"}}
			if inline.RevisionAuthor != "" {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "author"}, Value: inline.RevisionAuthor})
			}
			if inline.RevisionDate != "" {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "date"}, Value: inline.RevisionDate})
			}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			if err := e.EncodeElement(inline.Run, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
				return err
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	case ParagraphInlineMoveFrom:
		if inline.Run != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "moveFrom"}}
			if inline.RevisionAuthor != "" {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "author"}, Value: inline.RevisionAuthor})
			}
			if inline.RevisionDate != "" {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "date"}, Value: inline.RevisionDate})
			}
			if inline.MoveID != 0 {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "id"}, Value: strconv.Itoa(inline.MoveID)})
			}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			if err := e.EncodeElement(inline.Run, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
				return err
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	case ParagraphInlineMoveTo:
		if inline.Run != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "moveTo"}}
			if inline.RevisionAuthor != "" {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "author"}, Value: inline.RevisionAuthor})
			}
			if inline.RevisionDate != "" {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "date"}, Value: inline.RevisionDate})
			}
			if inline.MoveID != 0 {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "id"}, Value: strconv.Itoa(inline.MoveID)})
			}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			if err := e.EncodeElement(inline.Run, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
				return err
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	case ParagraphInlineMath:
		if inline.Math != nil && len(inline.Math.RawXML) > 0 {
			mNS := "http://schemas.openxmlformats.org/officeDocument/2006/math"
			local := "oMath"
			if inline.Math.Display {
				local = "oMathPara"
			}
			var raw struct {
				Content string `xml:",innerxml"`
			}
			raw.Content = string(inline.Math.RawXML)
			return e.EncodeElement(raw, xml.StartElement{Name: xml.Name{Space: mNS, Local: local}})
		}
	case ParagraphInlineSDT:
		if inline.SdtProps != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "sdt"}}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			if err := marshalSdtProperties(e, inline.SdtProps, wNS); err != nil {
				return err
			}
			contentStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: "sdtContent"}}
			if err := e.EncodeToken(contentStart); err != nil {
				return err
			}
			for _, child := range inline.SdtInlines {
				if err := marshalParagraphInline(e, child, wNS); err != nil {
					return err
				}
			}
			if err := e.EncodeToken(xml.EndElement{Name: contentStart.Name}); err != nil {
				return err
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	case ParagraphInlineDir:
		if inline.Run != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "dir"}}
			if inline.DirVal != "" {
				start.Attr = []xml.Attr{{Name: xml.Name{Local: "val"}, Value: inline.DirVal}}
			}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			if err := e.EncodeElement(inline.Run, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
				return err
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	case ParagraphInlineBdo:
		if inline.Run != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "bdo"}}
			if inline.DirVal != "" {
				start.Attr = []xml.Attr{{Name: xml.Name{Local: "val"}, Value: inline.DirVal}}
			}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			if err := e.EncodeElement(inline.Run, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
				return err
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	case ParagraphInlineSmartTag:
		if inline.Run != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "smartTag"}}
			if inline.SmartTagURI != "" {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "uri"}, Value: inline.SmartTagURI})
			}
			if inline.SmartTagElement != "" {
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "element"}, Value: inline.SmartTagElement})
			}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			if err := e.EncodeElement(inline.Run, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
				return err
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	case ParagraphInlineRuby:
		if inline.Ruby != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "ruby"}}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			rbStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: "rubyBase"}}
			if err := e.EncodeToken(rbStart); err != nil {
				return err
			}
			for _, r := range inline.Ruby.BaseRuns {
				if err := e.EncodeElement(r, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
					return err
				}
			}
			if err := e.EncodeToken(xml.EndElement{Name: rbStart.Name}); err != nil {
				return err
			}
			rtStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: "rt"}}
			if err := e.EncodeToken(rtStart); err != nil {
				return err
			}
			for _, r := range inline.Ruby.TextRuns {
				if err := e.EncodeElement(r, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
					return err
				}
			}
			if err := e.EncodeToken(xml.EndElement{Name: rtStart.Name}); err != nil {
				return err
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	}
	return nil
}

// MarshalXML emits w:r with the correct namespace, handling Properties, Text, and DrawingXML.
func (r Run) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	wNS := "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	start.Name = xml.Name{Space: wNS, Local: "r"}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if r.Properties != nil {
		if err := e.EncodeElement(r.Properties, xml.StartElement{Name: xml.Name{Space: wNS, Local: "rPr"}}); err != nil {
			return err
		}
	}
	if len(r.Contents) > 0 {
		for _, rc := range r.Contents {
			if err := marshalRunContent(e, rc, wNS); err != nil {
				return err
			}
		}
	} else {
		for _, t := range r.Text {
			if err := e.EncodeElement(t, xml.StartElement{Name: xml.Name{Space: wNS, Local: "t"}}); err != nil {
				return err
			}
		}
		for _, br := range r.Breaks {
			if err := e.EncodeElement(br, xml.StartElement{Name: xml.Name{Space: wNS, Local: "br"}}); err != nil {
				return err
			}
		}
		if len(r.DrawingXML) > 0 {
			var raw struct {
				Content string `xml:",innerxml"`
			}
			raw.Content = string(r.DrawingXML)
			if err := e.EncodeElement(raw, xml.StartElement{Name: xml.Name{Space: wNS, Local: "drawing"}}); err != nil {
				return err
			}
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func marshalRunContent(e *xml.Encoder, rc RunContent, wNS string) error {
	switch rc.Kind {
	case RunContentText:
		if rc.Text != nil {
			return e.EncodeElement(rc.Text, xml.StartElement{Name: xml.Name{Space: wNS, Local: "t"}})
		}
	case RunContentTab:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "tab"}})
	case RunContentBreak:
		if rc.Break != nil {
			return e.EncodeElement(rc.Break, xml.StartElement{Name: xml.Name{Space: wNS, Local: "br"}})
		}
	case RunContentDrawing:
		if len(rc.DrawingXML) > 0 {
			var raw struct {
				Content string `xml:",innerxml"`
			}
			raw.Content = string(rc.DrawingXML)
			return e.EncodeElement(raw, xml.StartElement{Name: xml.Name{Space: wNS, Local: "drawing"}})
		}
		if rc.Drawing != nil {
			return e.EncodeElement(rc.Drawing, xml.StartElement{Name: xml.Name{Space: wNS, Local: "drawing"}})
		}
	case RunContentFldChar:
		if rc.FldChar != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "fldChar"}}
			if rc.FldChar.Type != "" {
				start.Attr = []xml.Attr{{Name: xml.Name{Local: "fldCharType"}, Value: rc.FldChar.Type}}
			}
			return e.EncodeElement(rc.FldChar, start)
		}
	case RunContentInstrText:
		if rc.InstrText != "" {
			return e.EncodeElement(&Text{Value: rc.InstrText, Space: "preserve"}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "instrText"}})
		}
	case RunContentFootnoteRef:
		if rc.NoteRef != nil {
			return e.EncodeElement(rc.NoteRef, xml.StartElement{Name: xml.Name{Space: wNS, Local: "footnoteReference"}})
		}
	case RunContentEndnoteRef:
		if rc.NoteRef != nil {
			return e.EncodeElement(rc.NoteRef, xml.StartElement{Name: xml.Name{Space: wNS, Local: "endnoteReference"}})
		}
	case RunContentCommentRef:
		if rc.CommentRef != nil {
			return e.EncodeElement(rc.CommentRef, xml.StartElement{Name: xml.Name{Space: wNS, Local: "commentReference"}})
		}
	case RunContentSym:
		if rc.Sym != nil {
			return e.EncodeElement(rc.Sym, xml.StartElement{Name: xml.Name{Space: wNS, Local: "sym"}})
		}
	case RunContentSoftHyphen:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "softHyphen"}})
	case RunContentNoBreakHyphen:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "noBreakHyphen"}})
	case RunContentCR:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "cr"}})
	case RunContentDayShort:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "dayShort"}})
	case RunContentDayLong:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "dayLong"}})
	case RunContentMonthShort:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "monthShort"}})
	case RunContentMonthLong:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "monthLong"}})
	case RunContentYearShort:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "yearShort"}})
	case RunContentYearLong:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "yearLong"}})
	case RunContentPgNum:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "pgNum"}})
	case RunContentLastRenderedPageBreak:
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "lastRenderedPageBreak"}})
	case RunContentContentPart:
		if rc.ContentPart != nil && rc.ContentPart.RelID != "" {
			return e.EncodeElement(rc.ContentPart, xml.StartElement{Name: xml.Name{Space: wNS, Local: "contentPart"}})
		}
	case RunContentRuby:
		if rc.Ruby != nil {
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "ruby"}}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			// rubyBase
			rbStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: "rubyBase"}}
			if err := e.EncodeToken(rbStart); err != nil {
				return err
			}
			if err := e.EncodeElement(&Run{Text: []*Text{{Value: rc.Ruby.Base, Space: "preserve"}}}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
				return err
			}
			if err := e.EncodeToken(xml.EndElement{Name: rbStart.Name}); err != nil {
				return err
			}
			// rt
			rtStart := xml.StartElement{Name: xml.Name{Space: wNS, Local: "rt"}}
			if err := e.EncodeToken(rtStart); err != nil {
				return err
			}
			if err := e.EncodeElement(&Run{Text: []*Text{{Value: rc.Ruby.Text, Space: "preserve"}}}, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
				return err
			}
			if err := e.EncodeToken(xml.EndElement{Name: rtStart.Name}); err != nil {
				return err
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	case RunContentOLE:
		if rc.OLE != nil {
			vNS := "urn:schemas-microsoft-com:vml"
			oNS := "urn:schemas-microsoft-com:office:office"
			rNS := "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
			start := xml.StartElement{Name: xml.Name{Space: wNS, Local: "object"}}
			if err := e.EncodeToken(start); err != nil {
				return err
			}
			if rc.OLE.PreviewRelID != "" {
				shapeStart := xml.StartElement{Name: xml.Name{Space: vNS, Local: "shape"}}
				if err := e.EncodeToken(shapeStart); err != nil {
					return err
				}
				imgStart := xml.StartElement{Name: xml.Name{Space: vNS, Local: "imagedata"}, Attr: []xml.Attr{{Name: xml.Name{Space: rNS, Local: "id"}, Value: rc.OLE.PreviewRelID}}}
				if err := e.EncodeToken(imgStart); err != nil {
					return err
				}
				if err := e.EncodeToken(xml.EndElement{Name: imgStart.Name}); err != nil {
					return err
				}
				if err := e.EncodeToken(xml.EndElement{Name: shapeStart.Name}); err != nil {
					return err
				}
			}
			oleStart := xml.StartElement{Name: xml.Name{Space: oNS, Local: "OLEObject"}}
			for _, a := range []struct{ name, value string }{{"ProgID", rc.OLE.ProgID}, {"Type", rc.OLE.Type}, {"DrawAspect", rc.OLE.DrawAspect}, {"ObjectID", rc.OLE.ObjectID}, {"UpdateMode", rc.OLE.UpdateMode}} {
				if a.value != "" {
					oleStart.Attr = append(oleStart.Attr, xml.Attr{Name: xml.Name{Local: a.name}, Value: a.value})
				}
			}
			if rc.OLE.RelID != "" {
				oleStart.Attr = append(oleStart.Attr, xml.Attr{Name: xml.Name{Space: rNS, Local: "id"}, Value: rc.OLE.RelID})
			}
			if err := e.EncodeToken(oleStart); err != nil {
				return err
			}
			if err := e.EncodeToken(xml.EndElement{Name: oleStart.Name}); err != nil {
				return err
			}
			return e.EncodeToken(xml.EndElement{Name: start.Name})
		}
	case RunContentMath:
		if rc.MathML != "" {
			mNS := "http://schemas.openxmlformats.org/officeDocument/2006/math"
			var raw struct {
				Content string `xml:",innerxml"`
			}
			raw.Content = rc.MathML
			return e.EncodeElement(raw, xml.StartElement{Name: xml.Name{Space: mNS, Local: "oMath"}})
		}
	}
	return nil
}

// UnmarshalXML preserves ordered text/tab/break content inside a run.
func (r *Run) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*r = Run{}

	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}

		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "rPr":
				var properties RunProperties
				if err := dec.DecodeElement(&properties, &current); err != nil {
					return err
				}
				r.Properties = &properties
			case "t", "delText":
				// w:t — regular text; w:delText — text inside a w:del tracked-deletion run.
				var text Text
				if err := dec.DecodeElement(&text, &current); err != nil {
					return err
				}
				r.Text = append(r.Text, &text)
				r.Contents = append(r.Contents, RunContent{
					Kind: RunContentText,
					Text: &text,
				})
			case "tab":
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{
					Kind: RunContentTab,
				})
			case "br":
				var br Break
				if err := dec.DecodeElement(&br, &current); err != nil {
					return err
				}
				r.Breaks = append(r.Breaks, &br)
				r.Contents = append(r.Contents, RunContent{
					Kind:  RunContentBreak,
					Break: &br,
				})
			case "drawing":
				var raw struct {
					Content string `xml:",innerxml"`
				}
				if err := dec.DecodeElement(&raw, &current); err != nil {
					return err
				}
				r.DrawingXML = []byte(raw.Content)
				// Also decode for Contents/diagnostics.
				wrapped := "<drawing" +
					" xmlns:w=\"http://schemas.openxmlformats.org/wordprocessingml/2006/main\"" +
					" xmlns:wp=\"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing\"" +
					" xmlns:a=\"http://schemas.openxmlformats.org/drawingml/2006/main\"" +
					" xmlns:pic=\"http://schemas.openxmlformats.org/drawingml/2006/picture\"" +
					" xmlns:r=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships\"" +
					" xmlns:dgm=\"http://schemas.openxmlformats.org/drawingml/2006/diagram\"" +
					" xmlns:c=\"http://schemas.openxmlformats.org/drawingml/2006/chart\"" +
					" xmlns:mc=\"http://schemas.openxmlformats.org/markup-compatibility/2006\"" +
					" xmlns:wps=\"http://schemas.microsoft.com/office/word/2010/wordprocessingShape\"" +
					" xmlns:wpc=\"http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas\"" +
					" xmlns:wpg=\"http://schemas.microsoft.com/office/word/2010/wordprocessingGroup\"" +
					" xmlns:wp14=\"http://schemas.microsoft.com/office/word/2010/wordprocessingDrawing\"" +
					" xmlns:a14=\"http://schemas.microsoft.com/office/drawing/2010/main\"" +
					">" + raw.Content + "</drawing>"
				innerDec := xml.NewDecoder(strings.NewReader(wrapped))
				var drawing Drawing
				if err := innerDec.Decode(&drawing); err == nil {
					if code, message := unsupportedDrawingFeature(&drawing); code != "" {
						recordUnsupportedFeature(&r.Diagnostics, code, message, "run/drawing")
					}
					r.Contents = append(r.Contents, RunContent{
						Kind:       RunContentDrawing,
						Drawing:    &drawing,
						DrawingXML: []byte(raw.Content),
					})
				}
			case "AlternateContent":
				var alt AlternateContent
				if err := dec.DecodeElement(&alt, &current); err != nil {
					return err
				}
				// Prefer mc:Choice (modern drawing); fall back to VML mc:Fallback.
				found := false
				for _, choice := range alt.Choices {
					if choice.Drawing != nil {
						if code, message := unsupportedDrawingFeature(choice.Drawing); code != "" {
							recordUnsupportedFeature(&r.Diagnostics, code, message, "run/AlternateContent/Choice")
						}
						r.Contents = append(r.Contents, RunContent{
							Kind:    RunContentDrawing,
							Drawing: choice.Drawing,
						})
						found = true
						break
					}
				}
				if !found && alt.Fallback != nil {
					if appendVMLDrawingRunContents(&r.Contents, &r.Diagnostics, alt.Fallback.VMLDrawing, "run/AlternateContent/Fallback") {
						recordUnsupportedFeature(&r.Diagnostics, "word/unsupported-vml", "VML fallback content may render with reduced fidelity", "run/AlternateContent/Fallback")
					} else {
						recordUnsupportedFeature(&r.Diagnostics, "word/unsupported-vml", "VML fallback content may render with reduced fidelity", "run/AlternateContent/Fallback")
					}
				}
			case "object":
				// Scan for v:imagedata (preview image) and o:OLEObject (ProgID, r:id).
				var imageRelID, oleRelID, progID, oleType, drawAspect, objectID, updateMode string
				depthObj := 1
				for depthObj > 0 {
					tok, err := dec.Token()
					if err != nil {
						return err
					}
					switch t := tok.(type) {
					case xml.StartElement:
						depthObj++
						switch t.Name.Local {
						case "imagedata":
							for _, attr := range t.Attr {
								if attr.Name.Local == "id" {
									imageRelID = attr.Value
								}
							}
						case "OLEObject":
							for _, attr := range t.Attr {
								switch attr.Name.Local {
								case "ProgID":
									progID = attr.Value
								case "id":
									oleRelID = attr.Value
								case "Type":
									oleType = attr.Value
								case "DrawAspect":
									drawAspect = attr.Value
								case "ObjectID":
									objectID = attr.Value
								case "UpdateMode":
									updateMode = attr.Value
								}
							}
						}
					case xml.EndElement:
						depthObj--
					}
				}
				if imageRelID == "" {
					recordUnsupportedFeature(&r.Diagnostics, "word/unsupported-ole",
						"OLE object has no preview image; rendering a labeled placeholder", "run/object")
				}
				r.Contents = append(r.Contents, RunContent{
					Kind: RunContentOLE,
					OLE:  &OLEObjectRun{ProgID: progID, RelID: oleRelID, PreviewRelID: imageRelID, Type: oleType, DrawAspect: drawAspect, ObjectID: objectID, UpdateMode: updateMode},
				})
			case "fldChar":
				var fc FldChar
				if err := dec.DecodeElement(&fc, &current); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{
					Kind:    RunContentFldChar,
					FldChar: &fc,
				})
			case "instrText":
				var text Text
				if err := dec.DecodeElement(&text, &current); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{
					Kind:      RunContentInstrText,
					InstrText: text.Value,
				})
			case "delInstrText":
				var text Text
				if err := dec.DecodeElement(&text, &current); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{
					Kind:         RunContentDelInstrText,
					DelInstrText: text.Value,
				})
			case "footnoteReference":
				var ref NoteReference
				if err := dec.DecodeElement(&ref, &current); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{
					Kind:    RunContentFootnoteRef,
					NoteRef: &ref,
				})
			case "endnoteReference":
				var ref NoteReference
				if err := dec.DecodeElement(&ref, &current); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{
					Kind:    RunContentEndnoteRef,
					NoteRef: &ref,
				})
			case "sym":
				// w:sym — symbol character from a named font (ECMA-376 §17.3.3.30).
				var sym Sym
				if err := dec.DecodeElement(&sym, &current); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{
					Kind: RunContentSym,
					Sym:  &sym,
				})
			case "commentReference":
				var ref CommentReference
				if err := dec.DecodeElement(&ref, &current); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{
					Kind:       RunContentCommentRef,
					CommentRef: &ref,
				})
			case "annotationRef":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentAnnotationRef})
			case "lastRenderedPageBreak":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentLastRenderedPageBreak})
			case "dayShort":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentDayShort})
			case "dayLong":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentDayLong})
			case "monthShort":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentMonthShort})
			case "monthLong":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentMonthLong})
			case "yearShort":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentYearShort})
			case "yearLong":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentYearLong})
			case "pgNum":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentPgNum})
			case "contentPart":
				var cp ContentPartRef
				for _, attr := range current.Attr {
					if attr.Name.Local == "id" {
						cp.RelID = attr.Value
						break
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{
					Kind:        RunContentContentPart,
					ContentPart: &cp,
				})
			case "bookmarkStart":
				for _, attr := range current.Attr {
					if attr.Name.Local == "name" && attr.Value != "" {
						r.BookmarkStarts = append(r.BookmarkStarts, attr.Value)
						break
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "bookmarkEnd":
				if err := dec.Skip(); err != nil {
					return err
				}
			case "commentRangeStart":
				for _, attr := range current.Attr {
					if attr.Name.Local == "id" {
						if id, err := strconv.Atoi(attr.Value); err == nil {
							r.CommentRangeStarts = append(r.CommentRangeStarts, id)
						}
						break
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "commentRangeEnd":
				for _, attr := range current.Attr {
					if attr.Name.Local == "id" {
						if id, err := strconv.Atoi(attr.Value); err == nil {
							r.CommentRangeEnds = append(r.CommentRangeEnds, id)
						}
						break
					}
				}
				if err := dec.Skip(); err != nil {
					return err
				}
			case "softHyphen":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentSoftHyphen})
			case "noBreakHyphen":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentNoBreakHyphen})
			case "cr":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentCR})
			case "ptab":
				var ptab PTab
				if err := dec.DecodeElement(&ptab, &current); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{
					Kind: RunContentPTab,
					PTab: &ptab,
				})
			case "col":
				if err := dec.Skip(); err != nil {
					return err
				}
				r.Contents = append(r.Contents, RunContent{Kind: RunContentColumnBreak})
			case "separator", "continuationSeparator":
				if err := dec.Skip(); err != nil {
					return err
				}
			case "footnoteRef", "endnoteRef":
				if err := dec.Skip(); err != nil {
					return err
				}
			case "pict":
				// Legacy VML picture (ECMA-376 Part 4 §14). Decode to capture horizontal
				// rule elements (v:rect o:hr="t"). Other VML content is skipped.
				if err := decodePictElement(dec, current, r); err != nil {
					return err
				}
			case "movie", "objectEmbed", "objectLink":
				if err := dec.Skip(); err != nil {
					return err
				}
			default:
				recordUnsupportedMarkup(&r.Diagnostics, "run", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// UnmarshalXML preserves ordered content inside table cells, including cell properties.
func (c *TableCell) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*c = TableCell{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "tcPr":
				var props TableCellProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				c.Properties = &props
			case "p":
				var paragraph Paragraph
				if err := dec.DecodeElement(&paragraph, &current); err != nil {
					return err
				}
				c.Paragraphs = append(c.Paragraphs, &paragraph)
				c.Blocks = append(c.Blocks, ContentBlock{Kind: ContentBlockParagraph, Paragraph: &paragraph})
			case "tbl":
				var table Table
				if err := dec.DecodeElement(&table, &current); err != nil {
					return err
				}
				c.Tables = append(c.Tables, &table)
				c.Blocks = append(c.Blocks, ContentBlock{Kind: ContentBlockTable, Table: &table})
			case "sdt":
				sdtBlock, err := decodeSdtCellBlocks(dec, current, &c.Diagnostics)
				if err != nil {
					return err
				}
				c.Paragraphs = append(c.Paragraphs, collectContentBlockParagraphs(sdtBlock)...)
				c.Tables = append(c.Tables, collectContentBlockTables(sdtBlock)...)
				c.Blocks = append(c.Blocks, sdtBlock)
			case "customXml":
				cx, blocks, err := decodeCustomXmlBodyBlocks(dec, current, &c.Diagnostics)
				if err != nil {
					return err
				}
				var cellBlocks []ContentBlock
				for _, blk := range blocks {
					switch blk.Kind {
					case BodyBlockParagraph:
						c.Paragraphs = append(c.Paragraphs, blk.Paragraph)
						cellBlocks = append(cellBlocks, ContentBlock{Kind: ContentBlockParagraph, Paragraph: blk.Paragraph})
					case BodyBlockTable:
						c.Tables = append(c.Tables, blk.Table)
						cellBlocks = append(cellBlocks, ContentBlock{Kind: ContentBlockTable, Table: blk.Table})
					}
				}
				c.Blocks = append(c.Blocks, ContentBlock{
					Kind:            ContentBlockCustomXml,
					CustomXml:       cx,
					CustomXmlBlocks: cellBlocks,
				})
			default:
				recordUnsupportedMarkup(&c.Diagnostics, "table cell", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// UnmarshalXML preserves ordered content inside headers.
func (h *Header) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	paragraphs, tables, blocks, err := decodeContentBlocks(dec, start, &h.Diagnostics)
	if err != nil {
		return err
	}
	h.Paragraphs = paragraphs
	h.Tables = tables
	h.Blocks = blocks
	return nil
}

// UnmarshalXML preserves ordered content inside footers.
func (f *Footer) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	paragraphs, tables, blocks, err := decodeContentBlocks(dec, start, &f.Diagnostics)
	if err != nil {
		return err
	}
	f.Paragraphs = paragraphs
	f.Tables = tables
	f.Blocks = blocks
	return nil
}

func (b *DocPartBody) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	paragraphs, tables, blocks, err := decodeContentBlocks(dec, start, &b.Diagnostics)
	if err != nil {
		return err
	}
	b.Paragraphs = paragraphs
	b.Tables = tables
	b.Blocks = blocks
	return nil
}

// UnmarshalXML preserves comments and records unsupported top-level comment markup.
func (c *CommentsPart) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*c = CommentsPart{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "comment":
				var comment Comment
				if err := dec.DecodeElement(&comment, &current); err != nil {
					return err
				}
				c.Comments = append(c.Comments, &comment)
			default:
				recordUnsupportedMarkup(&c.Diagnostics, "comments", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// UnmarshalXML preserves ordered block content inside a comment.
func (c *Comment) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*c = Comment{
		ID:     attrIntValue(start, "id"),
		Author: attrValue(start, "author"),
		Date:   attrValue(start, "date"),
	}
	paragraphs, tables, blocks, err := decodeContentBlocks(dec, start, &c.Diagnostics)
	if err != nil {
		return err
	}
	c.Paragraphs = paragraphs
	c.Tables = tables
	c.Blocks = blocks
	return nil
}

// MarshalXML emits w:hyperlink with the correct namespace and r:id attribute.
func (h Hyperlink) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	wNS := "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	start.Name = xml.Name{Space: wNS, Local: "hyperlink"}
	if h.ID != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Space: "http://schemas.openxmlformats.org/officeDocument/2006/relationships", Local: "id"},
			Value: h.ID,
		})
	}
	if h.Anchor != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "anchor"}, Value: h.Anchor})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, r := range h.Runs {
		if err := e.EncodeElement(r, xml.StartElement{Name: xml.Name{Space: wNS, Local: "r"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML preserves runs inside a hyperlink and records unsupported children.
func (h *Hyperlink) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*h = Hyperlink{
		ID:     attrValue(start, "id"),
		Anchor: attrValue(start, "anchor"),
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "r":
				var run Run
				if err := dec.DecodeElement(&run, &current); err != nil {
					return err
				}
				h.Runs = append(h.Runs, &run)
			default:
				recordUnsupportedMarkup(&h.Diagnostics, "hyperlink", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// UnmarshalXML preserves table rows and records unsupported table children.
func (t *Table) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*t = Table{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "tblPr":
				var props TableProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				t.Properties = &props
			case "tblGrid":
				var grid TableGrid
				if err := grid.unmarshalXML(dec, current); err != nil {
					return err
				}
				t.Grid = &grid
			case "tr":
				var row TableRow
				if err := dec.DecodeElement(&row, &current); err != nil {
					return err
				}
				t.Rows = append(t.Rows, &row)
			default:
				recordUnsupportedMarkup(&t.Diagnostics, "table", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// MarshalXML emits w:tbl respecting the Grid columns when available.
func (t Table) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if t.Properties != nil {
		if err := e.EncodeElement(t.Properties, xml.StartElement{Name: xml.Name{Local: "tblPr"}}); err != nil {
			return err
		}
	}
	if t.Grid != nil && len(t.Grid.Columns) > 0 {
		gridStart := xml.StartElement{Name: xml.Name{Local: "tblGrid"}}
		if err := e.EncodeToken(gridStart); err != nil {
			return err
		}
		for _, w := range t.Grid.Columns {
			col := struct {
				XMLName xml.Name `xml:"gridCol"`
				Width   int      `xml:"w,attr"`
			}{Width: w}
			if err := e.Encode(col); err != nil {
				return err
			}
		}
		if err := e.EncodeToken(gridStart.End()); err != nil {
			return err
		}
	}
	for _, row := range t.Rows {
		if err := e.EncodeElement(row, xml.StartElement{Name: xml.Name{Local: "tr"}}); err != nil {
			return err
		}
	}
	if t.GridChange != nil {
		if err := e.EncodeElement(t.GridChange, xml.StartElement{Name: xml.Name{Local: "tblGridChange"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

func (g *TableGrid) unmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			if current.Name.Local == "gridCol" {
				w := attrIntValue(current, "w")
				g.Columns = append(g.Columns, w)
			}
			var discard struct{}
			if err := dec.DecodeElement(&discard, &current); err != nil {
				return err
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// UnmarshalXML preserves supported table properties and records skipped children.
func (p *TableProperties) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*p = TableProperties{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "tblW":
				var width TableMeasure
				if err := dec.DecodeElement(&width, &current); err != nil {
					return err
				}
				p.Width = &width
			case "jc":
				var alignment StringVal
				if err := dec.DecodeElement(&alignment, &current); err != nil {
					return err
				}
				p.Alignment = &alignment
			case "tblBorders":
				var borders TableBorders
				if err := dec.DecodeElement(&borders, &current); err != nil {
					return err
				}
				p.Borders = &borders
			case "tblStyle":
				var style StringVal
				if err := dec.DecodeElement(&style, &current); err != nil {
					return err
				}
				p.Style = &style
			case "shd":
				var shading Shading
				if err := dec.DecodeElement(&shading, &current); err != nil {
					return err
				}
				p.Shading = &shading
			case "tblLook":
				var look TableLook
				if err := dec.DecodeElement(&look, &current); err != nil {
					return err
				}
				p.Look = &look
			case "tblLayout":
				var layout TableLayout
				if err := dec.DecodeElement(&layout, &current); err != nil {
					return err
				}
				p.Layout = &layout
			case "tblInd":
				var ind TableMeasure
				if err := dec.DecodeElement(&ind, &current); err != nil {
					return err
				}
				p.Indent = &ind
			case "tblCellSpacing":
				var spacing TableMeasure
				if err := dec.DecodeElement(&spacing, &current); err != nil {
					return err
				}
				p.CellSpacing = &spacing
			case "tblCellMar":
				var mar TableCellMargin
				if err := dec.DecodeElement(&mar, &current); err != nil {
					return err
				}
				p.CellMargin = &mar
			case "tblpPr":
				fp := TableFloatProps{}
				for _, attr := range current.Attr {
					switch attr.Name.Local {
					case "horzAnchor":
						fp.HorzAnchor = attr.Value
					case "vertAnchor":
						fp.VertAnchor = attr.Value
					case "tblpX":
						fp.X, _ = strconv.Atoi(attr.Value)
					case "tblpY":
						fp.Y, _ = strconv.Atoi(attr.Value)
					case "tblpXSpec":
						fp.XAlign = attr.Value
					case "tblpYSpec":
						fp.YAlign = attr.Value
					case "leftFromText":
						fp.LeftFromText, _ = strconv.Atoi(attr.Value)
					case "rightFromText":
						fp.RightFromText, _ = strconv.Atoi(attr.Value)
					case "topFromText":
						fp.TopFromText, _ = strconv.Atoi(attr.Value)
					case "bottomFromText":
						fp.BottomFromText, _ = strconv.Atoi(attr.Value)
					}
				}
				// tblpPr is always a self-closing element; consume its end token
				if err := dec.Skip(); err != nil {
					return err
				}
				p.FloatProps = &fp
			case "tblOverlap":
				var v StringVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Overlap = &v
			case "tblCaption":
				var v struct {
					Val  string `xml:"val,attr,omitempty"`
					Body string `xml:",chardata"`
				}
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				if v.Val != "" {
					p.Caption = v.Val
				} else {
					p.Caption = v.Body
				}
			case "tblDescription":
				var v struct {
					Val  string `xml:"val,attr,omitempty"`
					Body string `xml:",chardata"`
				}
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				if v.Val != "" {
					p.Description = v.Val
				} else {
					p.Description = v.Body
				}
			case "bidiVisual":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.BidiVisual = &v
			case "tblStyleRowBandSize":
				var v IntVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.StyleRowBandSize = &v
			case "tblStyleColBandSize":
				var v IntVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.StyleColBandSize = &v
			case "tblPrChange":
				var change PropertyChangeInfo
				if err := dec.DecodeElement(&change, &current); err != nil {
					return err
				}
				p.PropertyChange = &change
			default:
				recordUnsupportedMarkup(&p.Diagnostics, "tblPr", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// UnmarshalXML preserves row properties and cells.
func (r *TableRow) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*r = TableRow{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "trPr":
				var props TableRowProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				r.Properties = &props
			case "tc":
				var cell TableCell
				if err := dec.DecodeElement(&cell, &current); err != nil {
					return err
				}
				r.Cells = append(r.Cells, &cell)
			default:
				recordUnsupportedMarkup(&r.Diagnostics, "table row", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// UnmarshalXML preserves supported row properties and records skipped children.
func (p *TableRowProperties) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*p = TableRowProperties{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "trHeight":
				var height TableRowHeight
				if err := dec.DecodeElement(&height, &current); err != nil {
					return err
				}
				p.Height = &height
			case "tblHeader":
				var header OnOff
				if err := dec.DecodeElement(&header, &current); err != nil {
					return err
				}
				p.IsHeader = &header
			case "cantSplit":
				var cantSplit OnOff
				if err := dec.DecodeElement(&cantSplit, &current); err != nil {
					return err
				}
				p.CantSplit = &cantSplit
			case "gridBefore":
				var value IntVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				p.GridBefore = &value
			case "gridAfter":
				var value IntVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				p.GridAfter = &value
			case "wBefore":
				var width TableMeasure
				if err := dec.DecodeElement(&width, &current); err != nil {
					return err
				}
				p.WidthBefore = &width
			case "wAfter":
				var width TableMeasure
				if err := dec.DecodeElement(&width, &current); err != nil {
					return err
				}
				p.WidthAfter = &width
			case "hidden":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Hidden = &v
			case "jc":
				var v StringVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Justification = &v
			case "cnfStyle":
				var v StringVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.Conditional = &v
			case "divId":
				var v IntVal
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.DivID = &v
			case "ins", "del":
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			case "trPrChange":
				var change PropertyChangeInfo
				if err := dec.DecodeElement(&change, &current); err != nil {
					return err
				}
				p.PropertyChange = &change
			default:
				recordUnsupportedMarkup(&p.Diagnostics, "trPr", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// UnmarshalXML preserves supported cell properties and records skipped children.
func (p *TableCellProperties) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*p = TableCellProperties{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "tcW":
				var width TableMeasure
				if err := dec.DecodeElement(&width, &current); err != nil {
					return err
				}
				p.Width = &width
			case "gridSpan":
				var span IntVal
				if err := dec.DecodeElement(&span, &current); err != nil {
					return err
				}
				p.GridSpan = &span
			case "vMerge":
				var merge VMerge
				if err := dec.DecodeElement(&merge, &current); err != nil {
					return err
				}
				p.VMerge = &merge
			case "tcBorders":
				var borders TableCellBorders
				if err := dec.DecodeElement(&borders, &current); err != nil {
					return err
				}
				p.Borders = &borders
			case "tcMar":
				var margins TableCellMargins
				if err := dec.DecodeElement(&margins, &current); err != nil {
					return err
				}
				p.Margins = &margins
			case "shd":
				var shading Shading
				if err := dec.DecodeElement(&shading, &current); err != nil {
					return err
				}
				p.Shading = &shading
			case "vAlign":
				var valign StringVal
				if err := dec.DecodeElement(&valign, &current); err != nil {
					return err
				}
				p.VertAlign = &valign
			case "textDirection":
				var direction StringVal
				if err := dec.DecodeElement(&direction, &current); err != nil {
					return err
				}
				p.TextDirection = &direction
			case "cnfStyle":
				var conditional StringVal
				if err := dec.DecodeElement(&conditional, &current); err != nil {
					return err
				}
				p.Conditional = &conditional
			case "noWrap":
				var noWrap OnOff
				if err := dec.DecodeElement(&noWrap, &current); err != nil {
					return err
				}
				p.NoWrap = &noWrap
			case "hideMark":
				var v OnOff
				if err := dec.DecodeElement(&v, &current); err != nil {
					return err
				}
				p.HideMark = &v
			case "hMerge":
				var merge VMerge
				if err := dec.DecodeElement(&merge, &current); err != nil {
					return err
				}
				p.HMerge = &merge
			case "cellIns":
				var tc TrackedCellChange
				for _, attr := range current.Attr {
					switch attr.Name.Local {
					case "author":
						tc.Author = attr.Value
					case "date":
						tc.Date = attr.Value
					case "id":
						tc.ID, _ = strconv.Atoi(attr.Value)
					}
				}
				p.CellIns = &tc
				if err := dec.Skip(); err != nil {
					return err
				}
			case "cellDel":
				var tc TrackedCellChange
				for _, attr := range current.Attr {
					switch attr.Name.Local {
					case "author":
						tc.Author = attr.Value
					case "date":
						tc.Date = attr.Value
					case "id":
						tc.ID, _ = strconv.Atoi(attr.Value)
					}
				}
				p.CellDel = &tc
				if err := dec.Skip(); err != nil {
					return err
				}
			case "cellMerge":
				var tc TrackedCellChange
				for _, attr := range current.Attr {
					switch attr.Name.Local {
					case "author":
						tc.Author = attr.Value
					case "date":
						tc.Date = attr.Value
					case "id":
						tc.ID, _ = strconv.Atoi(attr.Value)
					}
				}
				p.CellMerge = &tc
				if err := dec.Skip(); err != nil {
					return err
				}
			case "tcPrChange":
				var change PropertyChangeInfo
				if err := dec.DecodeElement(&change, &current); err != nil {
					return err
				}
				p.PropertyChange = &change
			default:
				recordUnsupportedMarkup(&p.Diagnostics, "tcPr", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

// UnmarshalXML preserves theme elements and records skipped theme children.
func (t *Theme) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*t = Theme{XMLName: start.Name}
	for _, attr := range start.Attr {
		if attr.Name.Local == "name" {
			t.Name = attr.Value
			break
		}
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "themeElements":
				var elements ThemeElements
				if err := dec.DecodeElement(&elements, &current); err != nil {
					return err
				}
				t.Elements = &elements
			case "objectDefaults":
				var defaults ObjectDefaults
				if err := dec.DecodeElement(&defaults, &current); err != nil {
					return err
				}
				t.ObjectDefs = &defaults
			case "extraClrSchemeLst":
				var extras ExtraColorSchemeList
				if err := dec.DecodeElement(&extras, &current); err != nil {
					return err
				}
				t.ExtraColors = &extras
			case "extLst":
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			default:
				recordUnsupportedMarkup(&t.Diagnostics, "theme", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (t *ThemeElements) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*t = ThemeElements{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "clrScheme":
				var scheme ColorScheme
				if err := dec.DecodeElement(&scheme, &current); err != nil {
					return err
				}
				t.ColorScheme = &scheme
			case "fontScheme":
				var scheme FontScheme
				if err := dec.DecodeElement(&scheme, &current); err != nil {
					return err
				}
				t.FontScheme = &scheme
			case "fmtScheme":
				var fs FormatScheme
				if err := dec.DecodeElement(&fs, &current); err != nil {
					return err
				}
				t.FmtScheme = &fs
			case "extLst":
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			default:
				recordUnsupportedMarkup(&t.Diagnostics, "themeElements", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (c *ColorScheme) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*c = ColorScheme{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			var entry ThemeColorEntry
			switch current.Name.Local {
			case "dk1":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.Dk1 = &entry
			case "lt1":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.Lt1 = &entry
			case "dk2":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.Dk2 = &entry
			case "lt2":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.Lt2 = &entry
			case "accent1":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.Accent1 = &entry
			case "accent2":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.Accent2 = &entry
			case "accent3":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.Accent3 = &entry
			case "accent4":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.Accent4 = &entry
			case "accent5":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.Accent5 = &entry
			case "accent6":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.Accent6 = &entry
			case "hlink":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.Hlink = &entry
			case "folHlink":
				if err := dec.DecodeElement(&entry, &current); err != nil {
					return err
				}
				c.FolHlink = &entry
			default:
				recordUnsupportedMarkup(&c.Diagnostics, "clrScheme", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (e *ThemeColorEntry) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*e = ThemeColorEntry{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "srgbClr":
				var color ThemeSRGBColor
				if err := dec.DecodeElement(&color, &current); err != nil {
					return err
				}
				e.SRGB = &color
			case "sysClr":
				var color ThemeSysColor
				if err := dec.DecodeElement(&color, &current); err != nil {
					return err
				}
				e.SysClr = &color
			default:
				recordUnsupportedMarkup(&e.Diagnostics, "theme color", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (f *FontScheme) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*f = FontScheme{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "majorFont":
				var fontSet ThemeFontSet
				if err := dec.DecodeElement(&fontSet, &current); err != nil {
					return err
				}
				f.MajorFont = &fontSet
			case "minorFont":
				var fontSet ThemeFontSet
				if err := dec.DecodeElement(&fontSet, &current); err != nil {
					return err
				}
				f.MinorFont = &fontSet
			default:
				recordUnsupportedMarkup(&f.Diagnostics, "fontScheme", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (f *ThemeFontSet) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*f = ThemeFontSet{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "latin":
				if err := dec.DecodeElement(&f.Latin, &current); err != nil {
					return err
				}
			case "ea":
				var face ThemeTypeface
				if err := dec.DecodeElement(&face, &current); err != nil {
					return err
				}
				f.EastAsia = append(f.EastAsia, &face)
			case "cs":
				var face ThemeTypeface
				if err := dec.DecodeElement(&face, &current); err != nil {
					return err
				}
				f.ComplexScript = append(f.ComplexScript, &face)
			case "font":
				var face ThemeSupplementalFont
				if err := dec.DecodeElement(&face, &current); err != nil {
					return err
				}
				f.Fonts = append(f.Fonts, &face)
			default:
				recordUnsupportedMarkup(&f.Diagnostics, "theme font set", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (f *FootnotesPart) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*f = FootnotesPart{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "footnote":
				var note Note
				if err := dec.DecodeElement(&note, &current); err != nil {
					return err
				}
				f.Notes = append(f.Notes, &note)
			default:
				recordUnsupportedMarkup(&f.Diagnostics, "footnotes", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (e *EndnotesPart) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*e = EndnotesPart{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "endnote":
				var note Note
				if err := dec.DecodeElement(&note, &current); err != nil {
					return err
				}
				e.Notes = append(e.Notes, &note)
			default:
				recordUnsupportedMarkup(&e.Diagnostics, "endnotes", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (n *Note) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*n = Note{
		ID:   attrIntValue(start, "id"),
		Type: attrValue(start, "type"),
	}
	paragraphs, tables, blocks, err := decodeContentBlocks(dec, start, &n.Diagnostics)
	if err != nil {
		return err
	}
	n.Paragraphs = paragraphs
	n.Tables = tables
	n.Blocks = blocks
	return nil
}

func (s *Styles) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*s = Styles{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "docDefaults":
				var defaults DocDefaults
				if err := dec.DecodeElement(&defaults, &current); err != nil {
					return err
				}
				s.DocDefaults = &defaults
			case "style":
				var style Style
				if err := dec.DecodeElement(&style, &current); err != nil {
					return err
				}
				s.Styles = append(s.Styles, &style)
			case "latentStyles":
				var ls LatentStyles
				if err := dec.DecodeElement(&ls, &current); err != nil {
					return err
				}
				s.LatentStyles = &ls
			default:
				recordUnsupportedMarkup(&s.Diagnostics, "styles", current.Name.Local)
				var raw RawElement
				if err := dec.DecodeElement(&raw, &current); err != nil {
					return err
				}
				s.Unknown = append(s.Unknown, raw)
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (d *DocDefaults) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*d = DocDefaults{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "rPrDefault":
				var defaults RunDefaults
				if err := dec.DecodeElement(&defaults, &current); err != nil {
					return err
				}
				d.RunDefaults = &defaults
			case "pPrDefault":
				var defaults ParagraphDefaults
				if err := dec.DecodeElement(&defaults, &current); err != nil {
					return err
				}
				d.ParaDefaults = &defaults
			default:
				recordUnsupportedMarkup(&d.Diagnostics, "docDefaults", current.Name.Local)
				var raw RawElement
				if err := dec.DecodeElement(&raw, &current); err != nil {
					return err
				}
				d.Unknown = append(d.Unknown, raw)
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (d *RunDefaults) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*d = RunDefaults{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "rPr":
				var props RunProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				d.Properties = &props
			default:
				recordUnsupportedMarkup(&d.Diagnostics, "rPrDefault", current.Name.Local)
				var raw RawElement
				if err := dec.DecodeElement(&raw, &current); err != nil {
					return err
				}
				d.Unknown = append(d.Unknown, raw)
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (d *ParagraphDefaults) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*d = ParagraphDefaults{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "pPr":
				var props ParagraphProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				d.Properties = &props
			default:
				recordUnsupportedMarkup(&d.Diagnostics, "pPrDefault", current.Name.Local)
				var raw RawElement
				if err := dec.DecodeElement(&raw, &current); err != nil {
					return err
				}
				d.Unknown = append(d.Unknown, raw)
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (s *Style) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*s = Style{
		Type:    attrValue(start, "type"),
		StyleID: attrValue(start, "styleId"),
	}
	if defaultValue, ok := attrValueOK(start, "default"); ok {
		b := parseBoolVal(defaultValue)
		s.Default = &b
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "name":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.Name = &value
			case "altName":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.AltName = &value
			case "aliases":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.Aliases = &value
			case "basedOn":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.BasedOn = &value
			case "next":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.Next = &value
			case "link":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.Link = &value
			case "uiPriority":
				var value IntVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.UIPriority = &value
			case "rsid":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.RSID = value.Val
			case "unhideWhenUsed":
				var value OnOff
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.Unhide = &value
			case "qFormat":
				var value OnOff
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.QuickFormat = &value
			case "semiHidden":
				var value OnOff
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.SemiHidden = &value
			case "autoRedefine":
				var value OnOff
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.AutoRedef = &value
			case "pPr":
				var props ParagraphProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				s.ParaProps = &props
			case "rPr":
				var props RunProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				s.RunProps = &props
			case "tblPr":
				var props TableProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				s.TableProps = &props
			case "tcPr":
				var props TableCellProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				s.CellProps = &props
			case "tblStylePr":
				var props TableStyleProps
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				s.TableStyles = append(s.TableStyles, &props)
			default:
				recordUnsupportedMarkup(&s.Diagnostics, "style", current.Name.Local)
				var raw RawElement
				if err := dec.DecodeElement(&raw, &current); err != nil {
					return err
				}
				s.Unknown = append(s.Unknown, raw)
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (n *Numbering) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*n = Numbering{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "abstractNum":
				var abstractNum AbstractNum
				if err := dec.DecodeElement(&abstractNum, &current); err != nil {
					return err
				}
				n.AbstractNums = append(n.AbstractNums, &abstractNum)
			case "num":
				var num Num
				if err := dec.DecodeElement(&num, &current); err != nil {
					return err
				}
				n.Nums = append(n.Nums, &num)
			case "numPicBullet":
				// picture bullet definition — not needed for rendering
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			default:
				recordUnsupportedMarkup(&n.Diagnostics, "numbering", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (a *AbstractNum) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*a = AbstractNum{ID: attrIntValue(start, "abstractNumId")}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "lvl":
				var level Lvl
				if err := dec.DecodeElement(&level, &current); err != nil {
					return err
				}
				a.Levels = append(a.Levels, &level)
			case "nsid", "multiLevelType", "tmpl", "lvlOverride":
				// metadata or override hints — not needed for rendering
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			default:
				recordUnsupportedMarkup(&a.Diagnostics, "abstractNum", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (l *Lvl) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*l = Lvl{Level: attrIntValue(start, "ilvl")}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "start":
				var value IntVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				l.Start = &value
			case "numFmt":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				l.Format = &value
			case "lvlText":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				l.Text = &value
			case "pStyle":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				l.PStyle = &value
			case "lvlJc":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				l.Justify = &value
			case "pPr":
				var props LvlPPr
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				l.ParaProps = &props
			case "rPr":
				var props RunProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				l.RunProps = &props
			case "lvlPicBulletId", "isLgl", "suff":
				// picture bullet ref, legal numbering, suffix — not needed for rendering
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			default:
				recordUnsupportedMarkup(&l.Diagnostics, "lvl", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (p *LvlPPr) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*p = LvlPPr{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "ind":
				var ind LvlInd
				if err := dec.DecodeElement(&ind, &current); err != nil {
					return err
				}
				p.Ind = &ind
			case "tabs", "jc":
				// tab stop positions and alignment in list levels — not needed for basic rendering
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			default:
				recordUnsupportedMarkup(&p.Diagnostics, "lvlPPr", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (n *Num) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*n = Num{ID: attrIntValue(start, "numId")}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "abstractNumId":
				var value IntVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				n.AbstractNumID = &value
			case "lvlOverride":
				var override LvlOverride
				if err := dec.DecodeElement(&override, &current); err != nil {
					return err
				}
				n.Overrides = append(n.Overrides, &override)
			default:
				recordUnsupportedMarkup(&n.Diagnostics, "num", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (o *LvlOverride) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*o = LvlOverride{Level: attrIntValue(start, "ilvl")}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "startOverride":
				var value IntVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				o.Start = &value
			case "lvl":
				var level Lvl
				if err := dec.DecodeElement(&level, &current); err != nil {
					return err
				}
				o.Lvl = &level
			default:
				recordUnsupportedMarkup(&o.Diagnostics, "lvlOverride", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func (s *SectionProperties) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	*s = SectionProperties{}
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "headerReference":
				var ref HeaderFooterReference
				if err := dec.DecodeElement(&ref, &current); err != nil {
					return err
				}
				s.HeaderRefs = append(s.HeaderRefs, ref)
			case "footerReference":
				var ref HeaderFooterReference
				if err := dec.DecodeElement(&ref, &current); err != nil {
					return err
				}
				s.FooterRefs = append(s.FooterRefs, ref)
			case "pgSz":
				var pageSize PageSize
				if err := dec.DecodeElement(&pageSize, &current); err != nil {
					return err
				}
				s.PageSize = &pageSize
			case "pgMar":
				var margins PageMargin
				if err := dec.DecodeElement(&margins, &current); err != nil {
					return err
				}
				s.PageMargin = &margins
			case "cols":
				var cols Columns
				if err := dec.DecodeElement(&cols, &current); err != nil {
					return err
				}
				s.Columns = &cols
			case "docGrid":
				var grid DocGrid
				if err := dec.DecodeElement(&grid, &current); err != nil {
					return err
				}
				s.DocGrid = &grid
			case "pgNumType":
				var pg PageNumberType
				if err := dec.DecodeElement(&pg, &current); err != nil {
					return err
				}
				s.PageNumType = &pg
			case "lnNumType":
				var ln LineNumberType
				if err := dec.DecodeElement(&ln, &current); err != nil {
					return err
				}
				s.LnNumType = &ln
			case "type":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.Type = &value
			case "textDirection":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.TextDirection = &value
			case "bidi":
				var value OnOff
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.Bidi = &value
			case "titlePg":
				var value OnOff
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.TitlePage = &value
			case "vAlign":
				var value StringVal
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.VAlign = &value
			case "pgBorders":
				var borders PageBorders
				if err := dec.DecodeElement(&borders, &current); err != nil {
					return err
				}
				s.PageBorders = &borders
			case "footnotePr":
				var props NoteProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				s.FootnoteProps = &props
			case "endnotePr":
				var props NoteProperties
				if err := dec.DecodeElement(&props, &current); err != nil {
					return err
				}
				s.EndnoteProps = &props
			case "noEndnote":
				var value OnOff
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.NoEndnote = &value
			case "rtlGutter":
				var value OnOff
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.RtlGutter = &value
			case "formProt":
				var value OnOff
				if err := dec.DecodeElement(&value, &current); err != nil {
					return err
				}
				s.FormProt = &value
			case "paperSrc":
				var src PaperSource
				if err := dec.DecodeElement(&src, &current); err != nil {
					return err
				}
				s.PaperSrc = &src
			case "sectPrChange":
				var change PropertyChangeInfo
				if err := dec.DecodeElement(&change, &current); err != nil {
					return err
				}
				s.PropertyChange = &change
			default:
				recordUnsupportedMarkup(&s.Diagnostics, "sectPr", current.Name.Local)
				var raw RawElement
				if err := dec.DecodeElement(&raw, &current); err != nil {
					return err
				}
				s.Unknown = append(s.Unknown, raw)
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return nil
			}
		}
	}
}

func decodeContentBlocks(dec *xml.Decoder, start xml.StartElement, diagnostics *diag.List) ([]*Paragraph, []*Table, []ContentBlock, error) {
	var paragraphs []*Paragraph
	var tables []*Table
	var blocks []ContentBlock

	for {
		token, err := dec.Token()
		if err != nil {
			return nil, nil, nil, err
		}

		switch current := token.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "p":
				var paragraph Paragraph
				if err := dec.DecodeElement(&paragraph, &current); err != nil {
					return nil, nil, nil, err
				}
				paragraphs = append(paragraphs, &paragraph)
				blocks = append(blocks, ContentBlock{
					Kind:      ContentBlockParagraph,
					Paragraph: &paragraph,
				})
			case "tbl":
				var table Table
				if err := dec.DecodeElement(&table, &current); err != nil {
					return nil, nil, nil, err
				}
				tables = append(tables, &table)
				blocks = append(blocks, ContentBlock{
					Kind:  ContentBlockTable,
					Table: &table,
				})
			case "sdt":
				sdtBlock, err := decodeSdtCellBlocks(dec, current, diagnostics)
				if err != nil {
					return nil, nil, nil, err
				}
				paragraphs = append(paragraphs, collectContentBlockParagraphs(sdtBlock)...)
				tables = append(tables, collectContentBlockTables(sdtBlock)...)
				blocks = append(blocks, sdtBlock)
			default:
				recordUnsupportedMarkup(diagnostics, start.Name.Local, current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return nil, nil, nil, err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return paragraphs, tables, blocks, nil
			}
		}
	}
}

// UnmarshalXML preserves ordered paragraphs and tables inside a text box.
func (t *TextBoxContent) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	paragraphs, tables, blocks, err := decodeContentBlocks(dec, start, &t.Diagnostics)
	if err != nil {
		return err
	}
	t.Paragraphs = paragraphs
	t.Tables = tables
	t.Blocks = blocks
	return nil
}

// decodeSdtBodyBlocks parses a w:sdt element at body level and returns a single BodyBlock of
// kind BodyBlockSDT. SdtProps carries parsed w:sdtPr metadata; SdtBlocks holds the ordered
// paragraph/table/nested-sdt children from w:sdtContent.
// ECMA-376 Part 1 §17.5.2.
func decodeSdtBodyBlocks(dec *xml.Decoder, sdtStart xml.StartElement, diagnostics *diag.List) (BodyBlock, error) {
	var props SdtProperties
	var children []BodyBlock
	for {
		tok, err := dec.Token()
		if err != nil {
			return BodyBlock{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "sdtPr":
				p, err := decodeSdtPr(dec, t)
				if err != nil {
					return BodyBlock{}, err
				}
				props = p
			case "sdtContent":
				ch, err := decodeSdtBodyContent(dec, t, diagnostics)
				if err != nil {
					return BodyBlock{}, err
				}
				children = ch
			case "sdtEndPr", "result":
				if err := dec.Skip(); err != nil {
					return BodyBlock{}, err
				}
			default:
				if err := dec.Skip(); err != nil {
					return BodyBlock{}, err
				}
			}
		case xml.EndElement:
			if t.Name == sdtStart.Name {
				return BodyBlock{Kind: BodyBlockSDT, SdtProps: &props, SdtBlocks: children}, nil
			}
		}
	}
}

func decodeSdtBodyContent(dec *xml.Decoder, start xml.StartElement, diagnostics *diag.List) ([]BodyBlock, error) {
	var blocks []BodyBlock
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				var para Paragraph
				if err := dec.DecodeElement(&para, &t); err != nil {
					return nil, err
				}
				blocks = append(blocks, BodyBlock{Kind: BodyBlockParagraph, Paragraph: &para})
			case "tbl":
				var table Table
				if err := dec.DecodeElement(&table, &t); err != nil {
					return nil, err
				}
				blocks = append(blocks, BodyBlock{Kind: BodyBlockTable, Table: &table})
			case "sdt":
				sdtBlock, err := decodeSdtBodyBlocks(dec, t, diagnostics)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, sdtBlock)
			default:
				recordUnsupportedMarkup(diagnostics, "sdtContent", t.Name.Local)
				if err := dec.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return blocks, nil
			}
		}
	}
}

// decodeCustomXmlBodyBlocks parses a w:customXml element at body level and returns
// the parsed metadata plus its child blocks.
func decodeCustomXmlBodyBlocks(dec *xml.Decoder, start xml.StartElement, diagnostics *diag.List) (*CustomXml, []BodyBlock, error) {
	cx := &CustomXml{}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "uri":
			cx.URI = attr.Value
		case "element":
			cx.Element = attr.Value
		}
	}
	var blocks []BodyBlock
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "customXmlPr":
				var pr CustomXmlPr
				if err := dec.DecodeElement(&pr, &t); err != nil {
					return nil, nil, err
				}
				cx.Properties = &pr
			case "p":
				var para Paragraph
				if err := dec.DecodeElement(&para, &t); err != nil {
					return nil, nil, err
				}
				blocks = append(blocks, BodyBlock{Kind: BodyBlockParagraph, Paragraph: &para})
			case "tbl":
				var table Table
				if err := dec.DecodeElement(&table, &t); err != nil {
					return nil, nil, err
				}
				blocks = append(blocks, BodyBlock{Kind: BodyBlockTable, Table: &table})
			case "sdt":
				sdtBlock, err := decodeSdtBodyBlocks(dec, t, diagnostics)
				if err != nil {
					return nil, nil, err
				}
				blocks = append(blocks, sdtBlock)
			case "altChunk":
				var chunk AltChunk
				if err := dec.DecodeElement(&chunk, &t); err != nil {
					return nil, nil, err
				}
				blocks = append(blocks, BodyBlock{Kind: BodyBlockAltChunk, AltChunk: &chunk})
			case "customXml":
				nestedCx, nested, err := decodeCustomXmlBodyBlocks(dec, t, diagnostics)
				if err != nil {
					return nil, nil, err
				}
				blocks = append(blocks, BodyBlock{
					Kind:            BodyBlockCustomXml,
					CustomXml:       nestedCx,
					CustomXmlBlocks: nested,
				})
			default:
				recordUnsupportedMarkup(diagnostics, "customXml", t.Name.Local)
				if err := dec.Skip(); err != nil {
					return nil, nil, err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return cx, blocks, nil
			}
		}
	}
}

// decodeCustomXmlParagraphInlines parses a w:customXml element inside a paragraph and
// returns the parsed metadata plus its child inlines.
func decodeCustomXmlParagraphInlines(dec *xml.Decoder, start xml.StartElement, p *Paragraph) (*CustomXml, []ParagraphInline, error) {
	cx := &CustomXml{}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "uri":
			cx.URI = attr.Value
		case "element":
			cx.Element = attr.Value
		}
	}
	var inlines []ParagraphInline
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		switch current := tok.(type) {
		case xml.StartElement:
			switch current.Name.Local {
			case "customXmlPr":
				var pr CustomXmlPr
				if err := dec.DecodeElement(&pr, &current); err != nil {
					return nil, nil, err
				}
				cx.Properties = &pr
			case "r":
				var run Run
				if err := dec.DecodeElement(&run, &current); err != nil {
					return nil, nil, err
				}
				p.Runs = append(p.Runs, &run)
				p.Bookmarks = append(p.Bookmarks, run.BookmarkStarts...)
				p.CommentRangeStarts = append(p.CommentRangeStarts, run.CommentRangeStarts...)
				p.CommentRangeEnds = append(p.CommentRangeEnds, run.CommentRangeEnds...)
				inlines = append(inlines, ParagraphInline{Kind: ParagraphInlineRun, Run: &run})
			case "hyperlink":
				var hyperlink Hyperlink
				if err := dec.DecodeElement(&hyperlink, &current); err != nil {
					return nil, nil, err
				}
				p.Hyperlinks = append(p.Hyperlinks, &hyperlink)
				inlines = append(inlines, ParagraphInline{Kind: ParagraphInlineHyperlink, Hyperlink: &hyperlink})
			case "sdt":
				sdtInline, err := decodeSdtParagraphInlines(dec, current, &p.Diagnostics)
				if err != nil {
					return nil, nil, err
				}
				inlines = append(inlines, sdtInline)
			case "ins":
				if err := decodeInsElement(dec, current, p); err != nil {
					return nil, nil, err
				}
				for i := len(p.Inlines) - 1; i >= 0 && p.Inlines[i].Kind == ParagraphInlineIns; i-- {
					inlines = append(inlines, p.Inlines[i])
				}
			case "del":
				if err := decodeDelElement(dec, current, p); err != nil {
					return nil, nil, err
				}
				for i := len(p.Inlines) - 1; i >= 0 && p.Inlines[i].Kind == ParagraphInlineDel; i-- {
					inlines = append(inlines, p.Inlines[i])
				}
			case "moveFrom":
				if err := decodeMoveFromElement(dec, current, p); err != nil {
					return nil, nil, err
				}
				for i := len(p.Inlines) - 1; i >= 0 && p.Inlines[i].Kind == ParagraphInlineMoveFrom; i-- {
					inlines = append(inlines, p.Inlines[i])
				}
			case "moveTo":
				if err := decodeMoveToElement(dec, current, p); err != nil {
					return nil, nil, err
				}
				for i := len(p.Inlines) - 1; i >= 0 && p.Inlines[i].Kind == ParagraphInlineMoveTo; i-- {
					inlines = append(inlines, p.Inlines[i])
				}
			case "bookmarkStart":
				var bm BookmarkStart
				for _, attr := range current.Attr {
					switch attr.Name.Local {
					case "name":
						bm.Name = attr.Value
						p.Bookmarks = append(p.Bookmarks, attr.Value)
					case "id":
						if id, err := strconv.Atoi(attr.Value); err == nil {
							bm.ID = id
						}
					}
				}
				p.BookmarkStartElems = append(p.BookmarkStartElems, bm)
				if err := dec.Skip(); err != nil {
					return nil, nil, err
				}
			case "commentRangeStart":
				var crs CommentRangeStart
				for _, attr := range current.Attr {
					if attr.Name.Local == "id" {
						if id, err := strconv.Atoi(attr.Value); err == nil {
							crs.ID = id
							p.CommentRangeStarts = append(p.CommentRangeStarts, id)
						}
						break
					}
				}
				p.CommentRangeStartElems = append(p.CommentRangeStartElems, crs)
				if err := dec.Skip(); err != nil {
					return nil, nil, err
				}
			case "commentRangeEnd":
				var cre CommentRangeEnd
				for _, attr := range current.Attr {
					if attr.Name.Local == "id" {
						if id, err := strconv.Atoi(attr.Value); err == nil {
							cre.ID = id
							p.CommentRangeEnds = append(p.CommentRangeEnds, id)
						}
						break
					}
				}
				p.CommentRangeEndElems = append(p.CommentRangeEndElems, cre)
				if err := dec.Skip(); err != nil {
					return nil, nil, err
				}
			case "bookmarkEnd":
				var bm BookmarkEnd
				for _, attr := range current.Attr {
					if attr.Name.Local == "id" {
						if id, err := strconv.Atoi(attr.Value); err == nil {
							bm.ID = id
						}
						break
					}
				}
				p.BookmarkEndElems = append(p.BookmarkEndElems, bm)
				if err := dec.Skip(); err != nil {
					return nil, nil, err
				}
			case "moveFromRangeStart", "moveFromRangeEnd", "moveToRangeStart", "moveToRangeEnd":
				if err := dec.Skip(); err != nil {
					return nil, nil, err
				}
			case "proofErr":
				if err := dec.Skip(); err != nil {
					return nil, nil, err
				}
			case "fldSimple":
				var fld Field
				for _, attr := range current.Attr {
					if attr.Name.Local == "instr" {
						fld.Instr = attr.Value
						break
					}
				}
				for {
					tok2, err := dec.Token()
					if err != nil {
						return nil, nil, err
					}
					switch t2 := tok2.(type) {
					case xml.StartElement:
						if t2.Name.Local == "r" {
							var run Run
							if err := dec.DecodeElement(&run, &t2); err != nil {
								return nil, nil, err
							}
							fld.Runs = append(fld.Runs, &run)
						} else {
							recordUnsupportedMarkup(&p.Diagnostics, "fldSimple", t2.Name.Local)
							if err := dec.Skip(); err != nil {
								return nil, nil, err
							}
						}
					case xml.EndElement:
						if t2.Name == current.Name {
							goto doneFldSimple
						}
					}
				}
			doneFldSimple:
				inlines = append(inlines, ParagraphInline{Kind: ParagraphInlineField, Field: &fld})
			case "ruby":
				ruby, err := decodeRubyElement(dec, current)
				if err != nil {
					return nil, nil, err
				}
				inlines = append(inlines, ParagraphInline{Kind: ParagraphInlineRuby, Ruby: ruby})
			case "oMath", "oMathPara":
				rawXML, err := captureRawXMLElement(dec, current)
				if err != nil {
					return nil, nil, err
				}
				inlines = append(inlines, ParagraphInline{
					Kind: ParagraphInlineMath,
					Math: &OMMath{RawXML: rawXML, Display: current.Name.Local == "oMathPara"},
				})
			case "customXml":
				nestedCx, nested, err := decodeCustomXmlParagraphInlines(dec, current, p)
				if err != nil {
					return nil, nil, err
				}
				inlines = append(inlines, ParagraphInline{
					Kind:             ParagraphInlineCustomXml,
					CustomXml:        nestedCx,
					CustomXmlInlines: nested,
				})
			default:
				recordUnsupportedMarkup(&p.Diagnostics, "customXml", current.Name.Local)
				var discard struct{}
				if err := dec.DecodeElement(&discard, &current); err != nil {
					return nil, nil, err
				}
			}
		case xml.EndElement:
			if current.Name == start.Name {
				return cx, inlines, nil
			}
		}
	}
}

// decodeSdtCellBlocks parses a w:sdt at cell/story level and returns a single ContentBlock of
// kind ContentBlockSDT. SdtProps carries parsed w:sdtPr metadata; SdtBlocks holds ordered children.
func decodeSdtCellBlocks(dec *xml.Decoder, sdtStart xml.StartElement, diagnostics *diag.List) (ContentBlock, error) {
	var props SdtProperties
	var children []ContentBlock
	for {
		tok, err := dec.Token()
		if err != nil {
			return ContentBlock{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "sdtPr":
				p, err := decodeSdtPr(dec, t)
				if err != nil {
					return ContentBlock{}, err
				}
				props = p
			case "sdtContent":
				ch, err := decodeSdtCellContent(dec, t, diagnostics)
				if err != nil {
					return ContentBlock{}, err
				}
				children = ch
			case "sdtEndPr", "result":
				if err := dec.Skip(); err != nil {
					return ContentBlock{}, err
				}
			default:
				if err := dec.Skip(); err != nil {
					return ContentBlock{}, err
				}
			}
		case xml.EndElement:
			if t.Name == sdtStart.Name {
				return ContentBlock{Kind: ContentBlockSDT, SdtProps: &props, SdtBlocks: children}, nil
			}
		}
	}
}

func decodeSdtCellContent(dec *xml.Decoder, start xml.StartElement, diagnostics *diag.List) ([]ContentBlock, error) {
	var blocks []ContentBlock
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				var para Paragraph
				if err := dec.DecodeElement(&para, &t); err != nil {
					return nil, err
				}
				blocks = append(blocks, ContentBlock{Kind: ContentBlockParagraph, Paragraph: &para})
			case "tbl":
				var table Table
				if err := dec.DecodeElement(&table, &t); err != nil {
					return nil, err
				}
				blocks = append(blocks, ContentBlock{Kind: ContentBlockTable, Table: &table})
			case "sdt":
				sdtBlock, err := decodeSdtCellBlocks(dec, t, diagnostics)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, sdtBlock)
			default:
				recordUnsupportedMarkup(diagnostics, "sdtContent", t.Name.Local)
				if err := dec.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return blocks, nil
			}
		}
	}
}

// decodeSdtParagraphInlines unwraps a w:sdt at inline level, returning its sdtContent as paragraph inlines.
// decodeInsElement decodes a w:ins tracked-change element, appending any contained
// w:r runs to p as normal ParagraphInlineRun inlines (accept tracked insertions).
// decodePictElement decodes a w:pict element, capturing basic VML fallback
// content such as images, text boxes, horizontal rules, and simple shapes.
func decodePictElement(dec *xml.Decoder, start xml.StartElement, r *Run) error {
	var pict VMLDrawing
	if err := dec.DecodeElement(&pict, &start); err != nil {
		return err
	}
	appendVMLDrawingRunContents(&r.Contents, &r.Diagnostics, pict, "run/pict")
	return nil
}

func appendVMLDrawingRunContents(contents *[]RunContent, diagnostics *diag.List, drawing VMLDrawing, path string) bool {
	appended := false
	for i := range drawing.Backgrounds {
		if appendVMLShapeRunContent(contents, diagnostics, "background", &drawing.Backgrounds[i].VMLShapeCommon, path+"/background") {
			appended = true
		}
	}
	for i := range drawing.Groups {
		if appendVMLShapeRunContent(contents, diagnostics, "group", &drawing.Groups[i].VMLShapeCommon, path+"/group") {
			appended = true
		}
		if appendVMLDrawingRunContents(contents, diagnostics, drawing.Groups[i].VMLDrawing, path+"/group") {
			appended = true
		}
	}
	for i := range drawing.Shapes {
		if appendVMLShapeRunContent(contents, diagnostics, "shape", &drawing.Shapes[i].VMLShapeCommon, path+"/shape") {
			appended = true
		}
	}
	for i := range drawing.Rects {
		if appendVMLShapeRunContent(contents, diagnostics, "rect", &drawing.Rects[i].VMLShapeCommon, path+"/rect") {
			appended = true
		}
	}
	for i := range drawing.RoundRects {
		if appendVMLShapeRunContent(contents, diagnostics, "roundrect", &drawing.RoundRects[i].VMLShapeCommon, path+"/roundrect") {
			appended = true
		}
	}
	for i := range drawing.Ovals {
		if appendVMLShapeRunContent(contents, diagnostics, "oval", &drawing.Ovals[i].VMLShapeCommon, path+"/oval") {
			appended = true
		}
	}
	for i := range drawing.Lines {
		if appendVMLShapeRunContent(contents, diagnostics, "line", &drawing.Lines[i].VMLShapeCommon, path+"/line") {
			appended = true
		}
	}
	for i := range drawing.Arcs {
		if appendVMLShapeRunContent(contents, diagnostics, "arc", &drawing.Arcs[i].VMLShapeCommon, path+"/arc") {
			appended = true
		}
	}
	for i := range drawing.Curves {
		if appendVMLShapeRunContent(contents, diagnostics, "curve", &drawing.Curves[i].VMLShapeCommon, path+"/curve") {
			appended = true
		}
	}
	for i := range drawing.Polylines {
		if appendVMLShapeRunContent(contents, diagnostics, "polyline", &drawing.Polylines[i].VMLShapeCommon, path+"/polyline") {
			appended = true
		}
	}
	for i := range drawing.Images {
		if appendVMLShapeRunContent(contents, diagnostics, "image", &drawing.Images[i].VMLShapeCommon, path+"/image") {
			appended = true
		}
	}
	return appended
}

func appendVMLShapeRunContent(contents *[]RunContent, diagnostics *diag.List, shapeType string, shape *VMLShapeCommon, path string) bool {
	if shape == nil {
		return false
	}
	fillColor := shape.FillColor
	if fillColor == "" && shape.Fill != nil {
		fillColor = shape.Fill.Color
	}
	strokeColor := shape.StrokeColor
	if strokeColor == "" && shape.Stroke != nil {
		strokeColor = shape.Stroke.Color
	}
	strokeWeight := shape.StrokeWeight
	if strokeWeight == "" && shape.Stroke != nil {
		strokeWeight = shape.Stroke.Weight
	}

	if tb := shape.TextBox; tb != nil && tb.Content != nil {
		*contents = append(*contents, RunContent{
			Kind:            RunContentVMLTextBox,
			VMLStyle:        shape.Style,
			VMLTextBoxStyle: tb.Style,
			VMLTextBox:      tb.Content,
			VMLFillColor:    fillColor,
		})
		return true
	}
	if img := shape.ImageData; img != nil && img.RelID != "" {
		*contents = append(*contents, RunContent{
			Kind:     RunContentVMLImage,
			VMLRelID: img.RelID,
			VMLStyle: shape.Style,
		})
		return true
	}
	if shapeType == "rect" && (shape.HR == "t" || shape.HR == "true" || shape.HR == "1") {
		*contents = append(*contents, RunContent{
			Kind:             RunContentVMLHorzRule,
			VMLHorzRuleColor: fillColor,
			VMLHorzRuleAlign: shape.HRAlign,
		})
		return true
	}
	if shapeType == "group" || shapeType == "background" {
		return false
	}
	// Record unsupported-VML diagnostic for shapes with features beyond our
	// current rendering path.  v:shape with a simple path is now handled;
	// shapetype references, textpaths, formulas, and handles are still reduced.
	hasComplexFeatures := shape.Type != "" || shape.TextPath != nil || shape.Formulas != nil || shape.Handles != nil
	hasUnsupportedPath := shapeType == "shape" && shape.PathRef == "" && (shape.Path == nil || shape.Path.V == "") && shape.Type == ""
	if diagnostics != nil && (shapeType == "curve" || shapeType == "polyline" || hasComplexFeatures || hasUnsupportedPath) {
		recordUnsupportedFeature(diagnostics, "word/unsupported-vml", "VML fallback content may render with reduced fidelity", path)
	}
	*contents = append(*contents, RunContent{
		Kind:              RunContentVMLShape,
		VMLShapeType:      shapeType,
		VMLStyle:          shape.Style,
		VMLFillColor:      fillColor,
		VMLStrokeColor:    strokeColor,
		VMLStrokeWeight:   strokeWeight,
		VMLFilledAttr:     shape.Filled,
		VMLStrokedAttr:    shape.Stroked,
		VMLFrom:           shape.From,
		VMLTo:             shape.To,
		VMLPoints:         shape.Points,
		VMLStartAngle:     shape.StartAngle,
		VMLEndAngle:       shape.EndAngle,
		VMLPathData:       vmlPathData(shape.PathRef, shape.Path),
		VMLCoordSize:      shape.CoordSize,
		VMLCoordOrigin:    shape.CoordOrigin,
		VMLTextPathOn:     vmlTextPathOn(shape.TextPath),
		VMLTextPathString: vmlTextPathString(shape.TextPath),
		VMLSignatureLine:  shape.SignatureLine,
	})
	return true
}

// vmlPathData returns the effective path string from a VML shape, preferring the
// child <v:path> element over the path attribute.
func vmlPathData(pathRef string, path *VMLPath) string {
	if path != nil && path.V != "" {
		return path.V
	}
	return pathRef
}

func vmlTextPathOn(tp *VMLTextPath) string {
	if tp == nil {
		return ""
	}
	return tp.On
}

func vmlTextPathString(tp *VMLTextPath) string {
	if tp == nil {
		return ""
	}
	return tp.String
}

func decodeInsElement(dec *xml.Decoder, start xml.StartElement, p *Paragraph) error {
	var author, date string
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "author":
			author = attr.Value
		case "date":
			date = attr.Value
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "r" {
				var run Run
				if err := dec.DecodeElement(&run, &t); err != nil {
					return err
				}
				p.Runs = append(p.Runs, &run)
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind:           ParagraphInlineIns,
					Run:            &run,
					RevisionAuthor: author,
					RevisionDate:   date,
				})
			} else {
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func decodeDirElement(dec *xml.Decoder, start xml.StartElement, p *Paragraph) error {
	var val string
	for _, attr := range start.Attr {
		if attr.Name.Local == "val" {
			val = attr.Value
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "r" {
				var run Run
				if err := dec.DecodeElement(&run, &t); err != nil {
					return err
				}
				p.Runs = append(p.Runs, &run)
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind:   ParagraphInlineDir,
					Run:    &run,
					DirVal: val,
				})
			} else {
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func decodeBdoElement(dec *xml.Decoder, start xml.StartElement, p *Paragraph) error {
	var val string
	for _, attr := range start.Attr {
		if attr.Name.Local == "val" {
			val = attr.Value
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "r" {
				var run Run
				if err := dec.DecodeElement(&run, &t); err != nil {
					return err
				}
				p.Runs = append(p.Runs, &run)
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind:   ParagraphInlineBdo,
					Run:    &run,
					DirVal: val,
				})
			} else {
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func decodeSmartTagElement(dec *xml.Decoder, start xml.StartElement, p *Paragraph) error {
	var uri, element string
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "uri":
			uri = attr.Value
		case "element":
			element = attr.Value
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "r":
				var run Run
				if err := dec.DecodeElement(&run, &t); err != nil {
					return err
				}
				p.Runs = append(p.Runs, &run)
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind:            ParagraphInlineSmartTag,
					Run:             &run,
					SmartTagURI:     uri,
					SmartTagElement: element,
				})
			case "smartTagPr", "uniqueTag":
				if err := dec.Skip(); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

// decodeRubyElement parses a w:ruby element and returns a Ruby containing the base
// text runs (w:rubyBase) and phonetic annotation runs (w:rt).
// ECMA-376 Part 1 §17.3.3.26.
func decodeRubyElement(dec *xml.Decoder, start xml.StartElement) (*Ruby, error) {
	var ruby Ruby
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "rt":
				// w:rt — ruby annotation text (phonetic guide above base text).
				runs, err := decodeRunGroup(dec, t)
				if err != nil {
					return nil, err
				}
				ruby.TextRuns = append(ruby.TextRuns, runs...)
			case "rubyBase":
				// w:rubyBase — base text that the annotation applies to.
				runs, err := decodeRunGroup(dec, t)
				if err != nil {
					return nil, err
				}
				ruby.BaseRuns = append(ruby.BaseRuns, runs...)
			case "rubyPr":
				var rp RubyPr
				if err := dec.DecodeElement(&rp, &t); err != nil {
					return nil, err
				}
				ruby.RubyPr = &rp
			default:
				if err := dec.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return &ruby, nil
			}
		}
	}
}

// decodeRunGroup consumes runs (w:r) from a container element and returns them.
func decodeRunGroup(dec *xml.Decoder, start xml.StartElement) ([]*Run, error) {
	var runs []*Run
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "r" {
				var run Run
				if err := dec.DecodeElement(&run, &t); err != nil {
					return nil, err
				}
				runs = append(runs, &run)
			} else {
				if err := dec.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return runs, nil
			}
		}
	}
}

func decodeDelElement(dec *xml.Decoder, start xml.StartElement, p *Paragraph) error {
	var author, date string
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "author":
			author = attr.Value
		case "date":
			date = attr.Value
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "r" {
				var run Run
				if err := dec.DecodeElement(&run, &t); err != nil {
					return err
				}
				// Deleted runs are captured in Inlines only; they do not appear in p.Runs.
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind:           ParagraphInlineDel,
					Run:            &run,
					RevisionAuthor: author,
					RevisionDate:   date,
				})
			} else {
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func decodeMoveFromElement(dec *xml.Decoder, start xml.StartElement, p *Paragraph) error {
	var author, date string
	var moveID int
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "author":
			author = attr.Value
		case "date":
			date = attr.Value
		case "id":
			moveID, _ = strconv.Atoi(attr.Value)
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "r" {
				var run Run
				if err := dec.DecodeElement(&run, &t); err != nil {
					return err
				}
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind:           ParagraphInlineMoveFrom,
					Run:            &run,
					RevisionAuthor: author,
					RevisionDate:   date,
					MoveID:         moveID,
				})
			} else {
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func decodeMoveToElement(dec *xml.Decoder, start xml.StartElement, p *Paragraph) error {
	var author, date string
	var moveID int
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "author":
			author = attr.Value
		case "date":
			date = attr.Value
		case "id":
			moveID, _ = strconv.Atoi(attr.Value)
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "r" {
				var run Run
				if err := dec.DecodeElement(&run, &t); err != nil {
					return err
				}
				p.Runs = append(p.Runs, &run)
				p.Inlines = append(p.Inlines, ParagraphInline{
					Kind:           ParagraphInlineMoveTo,
					Run:            &run,
					RevisionAuthor: author,
					RevisionDate:   date,
					MoveID:         moveID,
				})
			} else {
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func decodeSdtParagraphInlines(dec *xml.Decoder, sdtStart xml.StartElement, diagnostics *diag.List) (ParagraphInline, error) {
	var props SdtProperties
	var inlines []ParagraphInline
	for {
		tok, err := dec.Token()
		if err != nil {
			return ParagraphInline{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "sdtPr":
				p, err := decodeSdtPr(dec, t)
				if err != nil {
					return ParagraphInline{}, err
				}
				props = p
			case "sdtContent":
				ch, err := decodeSdtInlineContent(dec, t, diagnostics)
				if err != nil {
					return ParagraphInline{}, err
				}
				inlines = ch
			case "sdtEndPr", "result":
				if err := dec.Skip(); err != nil {
					return ParagraphInline{}, err
				}
			default:
				if err := dec.Skip(); err != nil {
					return ParagraphInline{}, err
				}
			}
		case xml.EndElement:
			if t.Name == sdtStart.Name {
				return ParagraphInline{Kind: ParagraphInlineSDT, SdtProps: &props, SdtInlines: inlines}, nil
			}
		}
	}
}

func decodeSdtInlineContent(dec *xml.Decoder, start xml.StartElement, diagnostics *diag.List) ([]ParagraphInline, error) {
	var inlines []ParagraphInline
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "r":
				var run Run
				if err := dec.DecodeElement(&run, &t); err != nil {
					return nil, err
				}
				inlines = append(inlines, ParagraphInline{Kind: ParagraphInlineRun, Run: &run})
			case "hyperlink":
				var hyperlink Hyperlink
				if err := dec.DecodeElement(&hyperlink, &t); err != nil {
					return nil, err
				}
				inlines = append(inlines, ParagraphInline{Kind: ParagraphInlineHyperlink, Hyperlink: &hyperlink})
			case "sdt":
				sdtInline, err := decodeSdtParagraphInlines(dec, t, diagnostics)
				if err != nil {
					return nil, err
				}
				inlines = append(inlines, sdtInline)
			default:
				recordUnsupportedMarkup(diagnostics, "sdtContent", t.Name.Local)
				if err := dec.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return inlines, nil
			}
		}
	}
}

// decodeSdtPr parses a w:sdtPr element, extracting structured document tag properties.
func decodeSdtPr(dec *xml.Decoder, start xml.StartElement) (SdtProperties, error) {
	var props SdtProperties
	for {
		tok, err := dec.Token()
		if err != nil {
			return props, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "alias":
				props.Alias = attrValue(t, "val")
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "tag":
				props.Tag = attrValue(t, "val")
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "id":
				props.ID = attrIntValue(t, "val")
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "checkBox":
				props.Type = "checkBox"
				checked, err := decodeSdtCheckBox(dec, t)
				if err != nil {
					return props, err
				}
				props.Checked = checked
			case "checkbox":
				// w14:checkbox — modern Word checkbox (Word 2013+)
				props.Type = "checkBox"
				checked, err := decodeSdtCheckBox14(dec, t)
				if err != nil {
					return props, err
				}
				props.Checked14 = checked
			case "dropDownList":
				props.Type = "dropDownList"
				items, err := decodeSdtListItems(dec, t)
				if err != nil {
					return props, err
				}
				props.ListItems = items
			case "comboBox":
				props.Type = "comboBox"
				items, err := decodeSdtListItems(dec, t)
				if err != nil {
					return props, err
				}
				props.ListItems = items
			case "date":
				props.Type = "date"
				dateProps, err := decodeSdtDateProperties(dec, t)
				if err != nil {
					return props, err
				}
				props.DateFormat = dateProps.Format
				props.DateFullDate = dateProps.FullDate
				props.DateLanguage = dateProps.Language
				props.DateCalendar = dateProps.Calendar
			case "richText":
				props.Type = "richText"
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "text":
				props.Type = "plainText"
				multiLine := attrValue(t, "multiLine")
				props.PlainTextMultiLine = multiLine == "1" || strings.EqualFold(multiLine, "true")
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "picture":
				props.Type = "picture"
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "docPartObj", "docPart":
				props.Type = "docPartObj"
				docPartProps, err := decodeSdtDocPartProperties(dec, t)
				if err != nil {
					return props, err
				}
				props.DocPartGallery = docPartProps.Gallery
				props.DocPartCategory = docPartProps.Category
				props.DocPartUnique = docPartProps.Unique
			case "placeholder":
				ph, err := decodeSdtPlaceholder(dec, t)
				if err != nil {
					return props, err
				}
				props.PlaceholderDocPart = ph.DocPart
				props.PlaceholderDocPartGallery = ph.DocPartGallery
			case "showingPlcHdr":
				props.ShowingPlcHdr = true
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "dataBinding":
				props.DataXPath = attrValue(t, "xpath")
				props.DataStoreID = attrValue(t, "storeItemID")
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "docPartList":
				props.Type = "docPartList"
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "citation":
				props.Type = "citation"
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "equation":
				props.Type = "equation"
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "group":
				props.Type = "group"
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "repeatingSection":
				props.Type = "repeatingSection"
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "repeatingSectionItem":
				props.Type = "repeatingSectionItem"
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "contentPart":
				props.Type = "contentPart"
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "gallery":
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "lock":
				props.Lock = attrValue(t, "val")
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "temporary":
				val := attrValue(t, "val")
				props.Temporary = val == "" || val == "1" || strings.EqualFold(val, "true")
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "appearance":
				props.Appearance = attrValue(t, "val")
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "color":
				props.Color = attrValue(t, "val")
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "sig":
				props.Type = "sig"
				if err := dec.Skip(); err != nil {
					return props, err
				}
			case "guid":
				if err := dec.Skip(); err != nil {
					return props, err
				}
			default:
				if err := dec.Skip(); err != nil {
					return props, err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return props, nil
			}
		}
	}
}

// decodeSdtCheckBox parses a w:checkBox element and returns whether the checkbox is checked.
// w:checked without a w:val attribute (or w:val="1") means checked; w:val="0" means unchecked.
func decodeSdtCheckBox(dec *xml.Decoder, start xml.StartElement) (bool, error) {
	var checked bool
	for {
		tok, err := dec.Token()
		if err != nil {
			return false, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "checked" {
				val := attrValue(t, "val")
				// Absent val (val=="") means true; "1" or "true" also means true.
				checked = val == "" || val == "1" || val == "true"
			}
			if err := dec.Skip(); err != nil {
				return false, err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return checked, nil
			}
		}
	}
}

// decodeSdtCheckBox14 parses a w14:checkbox element and returns whether the checkbox is checked.
// The w14:checked child without a w14:val attribute (or val="1") means checked; val="0" means unchecked.
func decodeSdtCheckBox14(dec *xml.Decoder, start xml.StartElement) (bool, error) {
	var checked bool
	for {
		tok, err := dec.Token()
		if err != nil {
			return false, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "checked" {
				val := attrValue(t, "val")
				checked = val == "" || val == "1" || val == "true"
			}
			if err := dec.Skip(); err != nil {
				return false, err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return checked, nil
			}
		}
	}
}

// sdtPlaceholder holds parsed w:placeholder content.
type sdtPlaceholder struct {
	DocPart        string
	DocPartGallery string
}

// decodeSdtPlaceholder parses a w:placeholder element, extracting w:docPart or w:docPartGallery.
func decodeSdtPlaceholder(dec *xml.Decoder, start xml.StartElement) (sdtPlaceholder, error) {
	var ph sdtPlaceholder
	for {
		tok, err := dec.Token()
		if err != nil {
			return sdtPlaceholder{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "docPart":
				ph.DocPart = attrValue(t, "val")
			case "docPartGallery":
				ph.DocPartGallery = attrValue(t, "val")
			}
			if err := dec.Skip(); err != nil {
				return sdtPlaceholder{}, err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return ph, nil
			}
		}
	}
}

// decodeSdtListItems parses w:listItem children of a w:dropDownList or w:comboBox element,
// returning both w:displayText and w:value attribute values from each item.
func decodeSdtListItems(dec *xml.Decoder, start xml.StartElement) ([]SdtListItem, error) {
	var items []SdtListItem
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "listItem" {
				items = append(items, SdtListItem{
					DisplayText: attrValue(t, "displayText"),
					Value:       attrValue(t, "value"),
				})
			}
			if err := dec.Skip(); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return items, nil
			}
		}
	}
}

type sdtDateProperties struct {
	Format   string
	FullDate string
	Language string
	Calendar string
}

type sdtDocPartProperties struct {
	Gallery  string
	Category string
	Unique   bool
}

// decodeSdtDateProperties parses a w:date element and returns its date metadata.
func decodeSdtDateProperties(dec *xml.Decoder, start xml.StartElement) (sdtDateProperties, error) {
	var props sdtDateProperties
	props.FullDate = attrValue(start, "fullDate")
	for {
		tok, err := dec.Token()
		if err != nil {
			return sdtDateProperties{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "dateFormat":
				props.Format = attrValue(t, "val")
			case "lid":
				props.Language = attrValue(t, "val")
			case "calendar":
				props.Calendar = attrValue(t, "val")
			}
			if err := dec.Skip(); err != nil {
				return sdtDateProperties{}, err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return props, nil
			}
		}
	}
}

func decodeSdtDocPartProperties(dec *xml.Decoder, start xml.StartElement) (sdtDocPartProperties, error) {
	var props sdtDocPartProperties
	for {
		tok, err := dec.Token()
		if err != nil {
			return sdtDocPartProperties{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "docPartGallery":
				props.Gallery = attrValue(t, "val")
			case "docPartCategory":
				props.Category = attrValue(t, "val")
			case "docPartUnique":
				props.Unique = true
			}
			if err := dec.Skip(); err != nil {
				return sdtDocPartProperties{}, err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return props, nil
			}
		}
	}
}

func collectBodyBlockParagraphs(block BodyBlock) []*Paragraph {
	var out []*Paragraph
	switch block.Kind {
	case BodyBlockParagraph:
		if block.Paragraph != nil {
			out = append(out, block.Paragraph)
		}
	case BodyBlockSDT:
		for _, child := range block.SdtBlocks {
			out = append(out, collectBodyBlockParagraphs(child)...)
		}
	case BodyBlockCustomXml:
		for _, child := range block.CustomXmlBlocks {
			out = append(out, collectBodyBlockParagraphs(child)...)
		}
	}
	return out
}

func collectBodyBlockTables(block BodyBlock) []*Table {
	var out []*Table
	switch block.Kind {
	case BodyBlockTable:
		if block.Table != nil {
			out = append(out, block.Table)
		}
	case BodyBlockSDT:
		for _, child := range block.SdtBlocks {
			out = append(out, collectBodyBlockTables(child)...)
		}
	case BodyBlockCustomXml:
		for _, child := range block.CustomXmlBlocks {
			out = append(out, collectBodyBlockTables(child)...)
		}
	}
	return out
}

func collectContentBlockParagraphs(block ContentBlock) []*Paragraph {
	var out []*Paragraph
	switch block.Kind {
	case ContentBlockParagraph:
		if block.Paragraph != nil {
			out = append(out, block.Paragraph)
		}
	case ContentBlockSDT:
		for _, child := range block.SdtBlocks {
			out = append(out, collectContentBlockParagraphs(child)...)
		}
	}
	return out
}

func collectContentBlockTables(block ContentBlock) []*Table {
	var out []*Table
	switch block.Kind {
	case ContentBlockTable:
		if block.Table != nil {
			out = append(out, block.Table)
		}
	case ContentBlockSDT:
		for _, child := range block.SdtBlocks {
			out = append(out, collectContentBlockTables(child)...)
		}
	}
	return out
}

// captureRawXMLElement re-encodes the full element tree (from start through its
// matching end tag) into a byte slice. The bytes can be fed back to an
// xml.Decoder for later structured parsing (e.g. OMML → MathML conversion).
func captureRawXMLElement(dec *xml.Decoder, start xml.StartElement) ([]byte, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := enc.EncodeToken(start); err != nil {
		return nil, err
	}
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if err := enc.EncodeToken(tok); err != nil {
			return nil, err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func recordUnsupportedMarkup(diagnostics *diag.List, container, local string) {
	if diagnostics == nil || local == "" {
		return
	}
	diagnostics.Add(diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Code:     "word.unsupported_markup",
		Message:  "unsupported " + container + " child <" + local + "> was skipped",
		Location: diag.Location{Part: container + "/" + local},
	})
}

func recordUnsupportedFeature(diagnostics *diag.List, code, message, path string) {
	if diagnostics == nil || code == "" || message == "" {
		return
	}
	diagnostics.Add(diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Code:     code,
		Message:  message,
		Location: diag.Location{Part: path},
	})
}

func unsupportedDrawingFeature(drawing *Drawing) (string, string) {
	if drawing == nil {
		return "", ""
	}
	for _, graphicData := range []*GraphicData{
		drawingInlineGraphicData(drawing.Inline),
		drawingAnchorGraphicData(drawing.Anchor),
	} {
		if graphicData == nil {
			continue
		}
		if code, message := unsupportedGraphicDataFeature(graphicData.URI); code != "" {
			return code, message
		}
	}
	return "", ""
}

func drawingInlineGraphicData(inline *DrawingInline) *GraphicData {
	if inline == nil || inline.Graphic == nil {
		return nil
	}
	return inline.Graphic.GraphicData
}

func drawingAnchorGraphicData(anchor *DrawingAnchor) *GraphicData {
	if anchor == nil || anchor.Graphic == nil {
		return nil
	}
	return anchor.Graphic.GraphicData
}

func unsupportedGraphicDataFeature(uri string) (string, string) {
	switch normalizeGraphicDataURI(uri) {
	case drawingGraphicDataURIChartTransitional, drawingGraphicDataURIChartStrict:
		// Charts are now supported - no warning
		return "", ""
	case drawingGraphicDataURIDiagramTransitional, drawingGraphicDataURIDiagramStrict:
		// SmartArt diagrams are resolved through the shared smartart package.
		return "", ""
	}
	return "", ""
}

func normalizeGraphicDataURI(uri string) string {
	uri = strings.TrimSpace(strings.ToLower(uri))
	switch {
	case strings.Contains(uri, "/chart"):
		return drawingGraphicDataURIChartTransitional
	case strings.Contains(uri, "/diagram"):
		return drawingGraphicDataURIDiagramTransitional
	default:
		return uri
	}
}

func collectPackageDiagnostics(pkg *Package) diag.List {
	if pkg == nil {
		return nil
	}
	var diagnostics diag.List
	if pkg.Document != nil {
		appendDiagnostics(&diagnostics, pkg.Document.Diagnostics)
		appendDiagnostics(&diagnostics, collectBodyDiagnostics(pkg.Document.Body))
	}
	appendDiagnostics(&diagnostics, collectStylesDiagnostics(pkg.Styles))
	appendDiagnostics(&diagnostics, collectNumberingDiagnostics(pkg.Numbering))
	appendDiagnostics(&diagnostics, collectThemeDiagnostics(pkg.Theme))
	for _, header := range pkg.Headers {
		if header == nil {
			continue
		}
		appendDiagnostics(&diagnostics, header.Diagnostics)
		appendDiagnostics(&diagnostics, collectContentBlocksDiagnostics(header.Blocks))
	}
	for _, footer := range pkg.Footers {
		if footer == nil {
			continue
		}
		appendDiagnostics(&diagnostics, footer.Diagnostics)
		appendDiagnostics(&diagnostics, collectContentBlocksDiagnostics(footer.Blocks))
	}
	if pkg.Footnotes != nil {
		appendDiagnostics(&diagnostics, pkg.Footnotes.Diagnostics)
		for _, note := range pkg.Footnotes.Notes {
			appendDiagnostics(&diagnostics, collectNoteDiagnostics(note))
		}
	}
	if pkg.Endnotes != nil {
		appendDiagnostics(&diagnostics, pkg.Endnotes.Diagnostics)
		for _, note := range pkg.Endnotes.Notes {
			appendDiagnostics(&diagnostics, collectNoteDiagnostics(note))
		}
	}
	if pkg.Comments != nil {
		appendDiagnostics(&diagnostics, pkg.Comments.Diagnostics)
		for _, comment := range pkg.Comments.Comments {
			appendDiagnostics(&diagnostics, collectCommentDiagnostics(comment))
		}
	}
	return diagnostics
}

func appendDiagnostics(dst *diag.List, src diag.List) {
	if dst == nil || len(src) == 0 {
		return
	}
	*dst = append(*dst, src...)
}

func collectBodyDiagnostics(body *Body) diag.List {
	if body == nil {
		return nil
	}
	var diagnostics diag.List
	appendDiagnostics(&diagnostics, body.Diagnostics)
	appendDiagnostics(&diagnostics, collectSectionDiagnostics(body.Section))
	for _, paragraph := range body.Paragraphs {
		appendDiagnostics(&diagnostics, collectParagraphDiagnostics(paragraph))
	}
	for _, table := range body.Tables {
		appendDiagnostics(&diagnostics, collectTableDiagnostics(table))
	}
	return diagnostics
}

func collectParagraphsDiagnostics(paragraphs ...*Paragraph) diag.List {
	var diagnostics diag.List
	for _, paragraph := range paragraphs {
		appendDiagnostics(&diagnostics, collectParagraphDiagnostics(paragraph))
	}
	return diagnostics
}

func collectParagraphDiagnostics(paragraph *Paragraph) diag.List {
	if paragraph == nil {
		return nil
	}
	var diagnostics diag.List
	appendDiagnostics(&diagnostics, paragraph.Diagnostics)
	for _, run := range paragraph.Runs {
		appendDiagnostics(&diagnostics, collectRunDiagnostics(run))
	}
	for _, hyperlink := range paragraph.Hyperlinks {
		if hyperlink == nil {
			continue
		}
		appendDiagnostics(&diagnostics, hyperlink.Diagnostics)
		for _, run := range hyperlink.Runs {
			appendDiagnostics(&diagnostics, collectRunDiagnostics(run))
		}
	}
	return diagnostics
}

func collectRunDiagnostics(run *Run) diag.List {
	if run == nil {
		return nil
	}
	return append(diag.List(nil), run.Diagnostics...)
}

func collectTableDiagnostics(table *Table) diag.List {
	if table == nil {
		return nil
	}
	var diagnostics diag.List
	appendDiagnostics(&diagnostics, table.Diagnostics)
	if table.Properties != nil {
		appendDiagnostics(&diagnostics, table.Properties.Diagnostics)
	}
	for _, row := range table.Rows {
		if row == nil {
			continue
		}
		appendDiagnostics(&diagnostics, row.Diagnostics)
		if row.Properties != nil {
			appendDiagnostics(&diagnostics, row.Properties.Diagnostics)
		}
		for _, cell := range row.Cells {
			appendDiagnostics(&diagnostics, collectCellDiagnostics(cell))
		}
	}
	return diagnostics
}

func collectCellDiagnostics(cell *TableCell) diag.List {
	if cell == nil {
		return nil
	}
	var diagnostics diag.List
	appendDiagnostics(&diagnostics, cell.Diagnostics)
	if cell.Properties != nil {
		appendDiagnostics(&diagnostics, cell.Properties.Diagnostics)
	}
	appendDiagnostics(&diagnostics, collectContentBlocksDiagnostics(cell.Blocks))
	return diagnostics
}

func collectContentBlocksDiagnostics(blocks []ContentBlock) diag.List {
	var diagnostics diag.List
	for _, block := range blocks {
		switch block.Kind {
		case ContentBlockParagraph:
			appendDiagnostics(&diagnostics, collectParagraphDiagnostics(block.Paragraph))
		case ContentBlockTable:
			appendDiagnostics(&diagnostics, collectTableDiagnostics(block.Table))
		}
	}
	return diagnostics
}

func noteParagraphs(note *Note) []*Paragraph {
	if note == nil {
		return nil
	}
	return note.Paragraphs
}

func commentParagraphs(comment *Comment) []*Paragraph {
	if comment == nil {
		return nil
	}
	return comment.Paragraphs
}

func collectNoteDiagnostics(note *Note) diag.List {
	if note == nil {
		return nil
	}
	var diagnostics diag.List
	appendDiagnostics(&diagnostics, note.Diagnostics)
	appendDiagnostics(&diagnostics, collectContentBlocksDiagnostics(note.Blocks))
	return diagnostics
}

func collectCommentDiagnostics(comment *Comment) diag.List {
	if comment == nil {
		return nil
	}
	var diagnostics diag.List
	appendDiagnostics(&diagnostics, comment.Diagnostics)
	appendDiagnostics(&diagnostics, collectContentBlocksDiagnostics(comment.Blocks))
	return diagnostics
}

func collectStylesDiagnostics(styles *Styles) diag.List {
	if styles == nil {
		return nil
	}
	var diagnostics diag.List
	appendDiagnostics(&diagnostics, styles.Diagnostics)
	if styles.DocDefaults != nil {
		appendDiagnostics(&diagnostics, styles.DocDefaults.Diagnostics)
		if styles.DocDefaults.RunDefaults != nil {
			appendDiagnostics(&diagnostics, styles.DocDefaults.RunDefaults.Diagnostics)
		}
		if styles.DocDefaults.ParaDefaults != nil {
			appendDiagnostics(&diagnostics, styles.DocDefaults.ParaDefaults.Diagnostics)
			appendDiagnostics(&diagnostics, collectSectionDiagnostics(sectionFromParagraphProperties(styles.DocDefaults.ParaDefaults.Properties)))
		}
	}
	for _, style := range styles.Styles {
		if style == nil {
			continue
		}
		appendDiagnostics(&diagnostics, style.Diagnostics)
		appendDiagnostics(&diagnostics, collectSectionDiagnostics(sectionFromParagraphProperties(style.ParaProps)))
	}
	return diagnostics
}

func collectNumberingDiagnostics(numbering *Numbering) diag.List {
	if numbering == nil {
		return nil
	}
	var diagnostics diag.List
	appendDiagnostics(&diagnostics, numbering.Diagnostics)
	for _, abstractNum := range numbering.AbstractNums {
		if abstractNum == nil {
			continue
		}
		appendDiagnostics(&diagnostics, abstractNum.Diagnostics)
		for _, level := range abstractNum.Levels {
			if level == nil {
				continue
			}
			appendDiagnostics(&diagnostics, level.Diagnostics)
			if level.ParaProps != nil {
				appendDiagnostics(&diagnostics, level.ParaProps.Diagnostics)
			}
		}
	}
	for _, num := range numbering.Nums {
		if num == nil {
			continue
		}
		appendDiagnostics(&diagnostics, num.Diagnostics)
		for _, override := range num.Overrides {
			if override == nil {
				continue
			}
			appendDiagnostics(&diagnostics, override.Diagnostics)
			if override.Lvl != nil {
				appendDiagnostics(&diagnostics, override.Lvl.Diagnostics)
				if override.Lvl.ParaProps != nil {
					appendDiagnostics(&diagnostics, override.Lvl.ParaProps.Diagnostics)
				}
			}
		}
	}
	return diagnostics
}

func collectThemeDiagnostics(theme *Theme) diag.List {
	if theme == nil {
		return nil
	}
	var diagnostics diag.List
	appendDiagnostics(&diagnostics, theme.Diagnostics)
	if theme.Elements != nil {
		appendDiagnostics(&diagnostics, theme.Elements.Diagnostics)
		if theme.Elements.ColorScheme != nil {
			appendDiagnostics(&diagnostics, theme.Elements.ColorScheme.Diagnostics)
			for _, entry := range []*ThemeColorEntry{
				theme.Elements.ColorScheme.Dk1,
				theme.Elements.ColorScheme.Lt1,
				theme.Elements.ColorScheme.Dk2,
				theme.Elements.ColorScheme.Lt2,
				theme.Elements.ColorScheme.Accent1,
				theme.Elements.ColorScheme.Accent2,
				theme.Elements.ColorScheme.Accent3,
				theme.Elements.ColorScheme.Accent4,
				theme.Elements.ColorScheme.Accent5,
				theme.Elements.ColorScheme.Accent6,
				theme.Elements.ColorScheme.Hlink,
				theme.Elements.ColorScheme.FolHlink,
			} {
				if entry != nil {
					appendDiagnostics(&diagnostics, entry.Diagnostics)
				}
			}
		}
		if theme.Elements.FontScheme != nil {
			appendDiagnostics(&diagnostics, theme.Elements.FontScheme.Diagnostics)
			if theme.Elements.FontScheme.MajorFont != nil {
				appendDiagnostics(&diagnostics, theme.Elements.FontScheme.MajorFont.Diagnostics)
			}
			if theme.Elements.FontScheme.MinorFont != nil {
				appendDiagnostics(&diagnostics, theme.Elements.FontScheme.MinorFont.Diagnostics)
			}
		}
	}
	return diagnostics
}

func collectSectionDiagnostics(section *SectionProperties) diag.List {
	if section == nil {
		return nil
	}
	return append(diag.List(nil), section.Diagnostics...)
}

func sectionFromParagraphProperties(properties *ParagraphProperties) *SectionProperties {
	if properties == nil {
		return nil
	}
	return properties.Section
}

func attrValue(start xml.StartElement, local string) string {
	value, _ := attrValueOK(start, local)
	return value
}

func attrValueOK(start xml.StartElement, local string) (string, bool) {
	for _, attr := range start.Attr {
		if attr.Name.Local == local {
			return attr.Value, true
		}
	}
	return "", false
}

func attrIntValue(start xml.StartElement, local string) int {
	value, ok := attrValueOK(start, local)
	if !ok || value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func parseBoolVal(value string) BoolVal {
	switch value {
	case "", "true", "1", "on":
		return BoolVal(true)
	default:
		return BoolVal(false)
	}
}

// WebSettings represents w:webSettings.
type WebSettings struct {
	XMLName               xml.Name   `xml:"webSettings"`
	OptimizeForBrowser    *OnOff     `xml:"optimizeForBrowser"`
	PixelsPerInch         *IntVal    `xml:"pixelsPerInch"`
	RelyOnVML             *OnOff     `xml:"relyOnVML"`
	TargetScreenSz        *StringVal `xml:"targetScreenSz"`
	DoNotOrganizeInFolder *OnOff     `xml:"doNotOrganizeInFolder"`
	DoNotRelyOnCSS        *OnOff     `xml:"doNotRelyOnCSS"`
	DoNotSaveAsSingleFile *OnOff     `xml:"doNotSaveAsSingleFile"`
	DoNotUseLongFileNames *OnOff     `xml:"doNotUseLongFileNames"`
	Behaviors             *Behaviors `xml:"behaviors"`
	Divs                  *Divs      `xml:"divs"`
	Frameset              *Frameset  `xml:"frameset"`
	Title                 *StringVal `xml:"title"`
	Scrollbar             *OnOff     `xml:"scrollbar"`
	LongDesc              *StringVal `xml:"longDesc"`
	Description           *StringVal `xml:"description"`
	Encoding              *StringVal `xml:"encoding"`
	Header                *StringVal `xml:"header"`
	Headers               *OnOff     `xml:"headers"`
	Table                 *OnOff     `xml:"table"`
}

// Behaviors represents w:behaviors.
type Behaviors struct {
	Behavior []Behavior `xml:"behavior"`
}

// Behavior represents w:behavior.
type Behavior struct {
	Val string `xml:"val,attr"`
}

// Divs represents w:divs.
type Divs struct {
	Divs []Div `xml:"div"`
}

// Div represents w:div within w:divs.
type Div struct {
	ID        *IntVal     `xml:"id,attr"`
	DivBdr    *DivBorders `xml:"divBdr"`
	DivsChild *Divs       `xml:"divsChild"`
	Columns   []DivColumn `xml:"column"`
}

// DivColumn represents w:column within w:div.
type DivColumn struct {
	Space int `xml:"space,attr,omitempty"`
}

// DivBorders represents w:divBdr.
type DivBorders struct {
	Top    *Border `xml:"top"`
	Left   *Border `xml:"left"`
	Bottom *Border `xml:"bottom"`
	Right  *Border `xml:"right"`
}

// Frameset represents w:frameset.
type Frameset struct {
	FrameLayout      *StringVal        `xml:"frameLayout"`
	FramesetSplitbar *FramesetSplitbar `xml:"framesetSplitbar"`
	Frames           []Frame           `xml:"frame"`
	ChildFramesets   []Frameset        `xml:"frameset"`
}

// FramesetSplitbar represents w:framesetSplitbar.
type FramesetSplitbar struct {
	Val string `xml:"val,attr,omitempty"`
}

// Frame represents w:frame within w:frameset.
type Frame struct {
	Name       string `xml:"name,attr,omitempty"`
	SourceFile string `xml:"sourceFile,attr,omitempty"`
	Src        string `xml:"src,attr,omitempty"`
	Header     string `xml:"header,attr,omitempty"`
	MarBottom  int    `xml:"marBottom,attr,omitempty"`
	MarH       int    `xml:"marH,attr,omitempty"`
	MarLeft    int    `xml:"marLeft,attr,omitempty"`
	MarRight   int    `xml:"marRight,attr,omitempty"`
	MarTop     int    `xml:"marTop,attr,omitempty"`
	MarW       int    `xml:"marW,attr,omitempty"`
}

// unmarshalFrameAttrs is present for spec-coverage tracking of frame-level
// attribute names that are stored as struct-tag attributes rather than child
// element bindings.
func (f *Frame) unmarshalFrameAttrs(dec *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "marBottom", "marH", "marLeft", "marRight", "marTop", "marW", "src":
			// parsed via struct tags
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "marBottom", "marH", "marLeft", "marRight", "marTop", "marW", "src":
				if err := dec.Skip(); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

// TableGridChange represents w:tblGridChange.
type TableGridChange struct {
	ID int `xml:"id,attr"`
}

// TablePropertyChange represents w:tblPrExChange.
type TablePropertyChange struct {
	ID int `xml:"id,attr"`
}

// MoveRangeStart represents w:moveFromRangeStart or w:moveToRangeStart.
type MoveRangeStart struct {
	ID     int    `xml:"id,attr"`
	Author string `xml:"author,attr,omitempty"`
	Date   string `xml:"date,attr,omitempty"`
	Name   string `xml:"name,attr,omitempty"`
}

// MoveRangeEnd represents w:moveFromRangeEnd or w:moveToRangeEnd.
type MoveRangeEnd struct {
	ID int `xml:"id,attr"`
}

// DrawingTextBody represents a:txBody — the text body inside a DrawingML shape.
type DrawingTextBody struct {
	BodyPr     *DrawingTextBodyProperties `xml:"bodyPr"`
	LstStyle   *DrawingTextListStyle      `xml:"lstStyle"`
	Paragraphs []DrawingDMLParagraph      `xml:"p"`
}

// DrawingTextBodyProperties represents a:bodyPr inside a:txBody.
type DrawingTextBodyProperties struct {
	Anchor      string                 `xml:"anchor,attr,omitempty"`
	AnchorCtr   bool                   `xml:"anchorCtr,attr,omitempty"`
	Rot         int                    `xml:"rot,attr,omitempty"`
	Vert        string                 `xml:"vert,attr,omitempty"`
	Wrap        string                 `xml:"wrap,attr,omitempty"`
	LIns        int64                  `xml:"lIns,attr,omitempty"`
	TIns        int64                  `xml:"tIns,attr,omitempty"`
	RIns        int64                  `xml:"rIns,attr,omitempty"`
	BIns        int64                  `xml:"bIns,attr,omitempty"`
	NoAutofit   *DrawingNoAutofit      `xml:"noAutofit"`
	SpAutofit   *DrawingSpAutofit      `xml:"spAutoFit"`
	NormAutofit *DrawingNormAutofit    `xml:"normAutofit"`
	PrstTxWarp  *DrawingPresetTextWarp `xml:"prstTxWarp"`
	FlatTx      *struct{}              `xml:"flatTx"`
}

// DrawingNoAutofit represents a:noAutofit.
type DrawingNoAutofit struct{}

// DrawingSpAutofit represents a:spAutoFit.
type DrawingSpAutofit struct{}

// DrawingNormAutofit represents a:normAutofit.
type DrawingNormAutofit struct {
	FontScale      int `xml:"fontScale,attr,omitempty"`
	LnSpcReduction int `xml:"lnSpcReduction,attr,omitempty"`
}

// DrawingPresetTextWarp represents a:prstTxWarp.
type DrawingPresetTextWarp struct {
	Prst string `xml:"prst,attr,omitempty"`
}

// DrawingTextListStyle represents a:lstStyle inside a:txBody.
type DrawingTextListStyle struct {
	Levels []DrawingTextStyleLevel `xml:"lvl*"`
}

// DrawingTextStyleLevel represents a:defPPr or a:lvl*pPr inside a:lstStyle.
type DrawingTextStyleLevel struct {
	Lvl    int               `xml:"lvl,attr,omitempty"`
	DefPPr *DrawingParaProps `xml:"defPPr"`
}

// DrawingDMLParagraph represents a:p inside a:txBody.
type DrawingDMLParagraph struct {
	PPr  *DrawingParaProps `xml:"pPr"`
	Runs []DrawingDMLRun   `xml:"r"`
	EndP *struct{}         `xml:"endParaRPr"`
}

// DrawingParaProps represents a:pPr inside a:txBody paragraph.
type DrawingParaProps struct {
	Lvl    int              `xml:"lvl,attr,omitempty"`
	DefRPr *DrawingRunProps `xml:"defRPr"`
}

// DrawingDMLRun represents a:r inside a:txBody paragraph.
type DrawingDMLRun struct {
	RPr *DrawingRunProps `xml:"rPr"`
	T   string           `xml:"t"`
	Tab *struct{}        `xml:"tab"`
	Rtl *struct{}        `xml:"rtl"`
}

// DrawingTextFont represents a:latin, a:ea, or a:cs inside a DrawingML run.
type DrawingTextFont struct {
	Typeface string `xml:"typeface,attr,omitempty"`
}

// DrawingRunProps represents a:rPr inside a:txBody run.
type DrawingRunProps struct {
	Lang       string                `xml:"lang,attr,omitempty"`
	Latin      *DrawingTextFont      `xml:"latin"`
	EastAsian  *DrawingTextFont      `xml:"ea"`
	ComplexScr *DrawingTextFont      `xml:"cs"`
	Size       int                   `xml:"sz,attr,omitempty"`
	Bold       bool                  `xml:"b"`
	Italic     bool                  `xml:"i"`
	Underline  string                `xml:"u,attr,omitempty"`
	Strike     string                `xml:"strike,attr,omitempty"`
	SolidFill  *drawingxml.SolidFill `xml:"solidFill"`
	Baseline   int                   `xml:"baseline,attr,omitempty"`
}

// DrawingTable represents a:tbl inside a DrawingML shape.
type DrawingTable struct {
	XMLName xml.Name                `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tbl"`
	TblPr   *DrawingTableProperties `xml:"tblPr"`
	TblGrid *DrawingTableGrid       `xml:"tblGrid"`
	Rows    []DrawingTableRow       `xml:"tr"`
}

// DrawingTableProperties represents a:tblPr.
type DrawingTableProperties struct {
	TableStyleID string `xml:"tableStyleId,attr,omitempty"`
}

// DrawingTableGrid represents a:tblGrid.
type DrawingTableGrid struct {
	GridCols []DrawingGridColumn `xml:"gridCol"`
}

// DrawingGridColumn represents a:gridCol.
type DrawingGridColumn struct {
	W int64 `xml:"w,attr,omitempty"`
}

// DrawingTableRow represents a:tr.
type DrawingTableRow struct {
	H     int64              `xml:"h,attr,omitempty"`
	Cells []DrawingTableCell `xml:"tc"`
}

// DrawingTableCell represents a:tc.
type DrawingTableCell struct {
	TcPr   *DrawingTableCellProperties `xml:"tcPr"`
	TxBody *DrawingTextBody            `xml:"txBody"`
}

// DrawingTableCellProperties represents a:tcPr.
type DrawingTableCellProperties struct {
	TcBdr  *DrawingTableCellBorders `xml:"tcBdr"`
	Cell3D *Cell3D                  `xml:"cell3D"`
}

// DrawingTableCellBorders represents a:tcBdr.
type DrawingTableCellBorders struct {
	XMLName       xml.Name     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tcBdr"`
	Left          *LineWrapper `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnL"`
	Right         *LineWrapper `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnR"`
	Top           *LineWrapper `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnT"`
	Bottom        *LineWrapper `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnB"`
	DiagonalTL2BR *LineWrapper `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnTlToBr"`
	DiagonalTR2BL *LineWrapper `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnBlToTr"`
}

// LineWrapper wraps a drawingxml.Line to avoid XML namespace conflicts.
type LineWrapper struct {
	Ln *drawingxml.Line `xml:"ln"`
}

// CNvPicPr represents a:cNvPicPr (non-visual picture properties).
type CNvPicPr struct {
	PreferRelativeResize bool `xml:"preferRelativeResize,attr,omitempty"`
}

// CNvCxnSpPr represents a:cNvCxnSpPr (non-visual connector shape properties).
type CNvCxnSpPr struct{}
