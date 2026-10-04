package rels

import (
	"encoding/xml"
	"path"
	"strings"
)

const (
	namespacePackageRelationships        = "http://schemas.openxmlformats.org/package/2006/relationships"
	targetModeExternal                   = "External"
	rootRelationshipsPart                = "_rels/.rels"
	strictOfficeRelationshipPrefix       = "http://purl.oclc.org/ooxml/officeDocument/relationships/"
	transitionalOfficeRelationshipPrefix = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"
)

// Relationship is a single OPC relationship edge.
type Relationship struct {
	ID         string `xml:"Id,attr"`
	Type       string `xml:"Type,attr"`
	Target     string `xml:"Target,attr"`
	TargetMode string `xml:"TargetMode,attr,omitempty"`
}

// Document is a parsed .rels part.
type Document struct {
	XMLName       xml.Name       `xml:"Relationships"`
	Relationships []Relationship `xml:"Relationship"`
}

// Parse decodes a .rels part.
func Parse(data []byte) (*Document, error) {
	var doc Document
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	for i := range doc.Relationships {
		doc.Relationships[i].Type = canonicalRelationshipType(doc.Relationships[i].Type)
	}
	return &doc, nil
}

// canonicalRelationshipType maps ECMA-376 Strict relationship URIs to their
// Transitional equivalents. Relationship kind is semantic rather than a part
// of XML markup, and the rest of the package readers compare these values to
// canonical OOXML relationship constants.
func canonicalRelationshipType(value string) string {
	if strings.HasPrefix(value, strictOfficeRelationshipPrefix) {
		return transitionalOfficeRelationshipPrefix + strings.TrimPrefix(value, strictOfficeRelationshipPrefix)
	}
	return value
}

// ByID finds a relationship by its ID.
func (d *Document) ByID(id string) (Relationship, bool) {
	if d == nil {
		return Relationship{}, false
	}
	for _, rel := range d.Relationships {
		if rel.ID == id {
			return rel, true
		}
	}
	return Relationship{}, false
}

// IsExternal reports whether the relationship points outside the package.
func (r Relationship) IsExternal() bool {
	return strings.EqualFold(strings.TrimSpace(r.TargetMode), targetModeExternal)
}

// RelationshipsPartName returns the .rels part path for a package part.
func RelationshipsPartName(sourcePart string) string {
	sourcePart = normalizePartName(sourcePart)
	if sourcePart == "" {
		return rootRelationshipsPart
	}

	dir, file := path.Split(sourcePart)
	if dir == "" {
		return "_rels/" + file + ".rels"
	}
	return path.Join(dir, "_rels", file+".rels")
}

// ResolveTarget resolves a relationship target against its owning part.
func ResolveTarget(sourcePart, target string) string {
	target = strings.TrimSpace(strings.ReplaceAll(target, "\\", "/"))
	if target == "" {
		return ""
	}
	if strings.HasPrefix(target, "/") {
		return normalizePartName(target)
	}

	sourcePart = normalizePartName(sourcePart)
	if sourcePart == "" {
		return normalizePartName(target)
	}

	baseDir := path.Dir(sourcePart)
	if baseDir == "." {
		baseDir = ""
	}
	return normalizePartName(path.Join(baseDir, target))
}

func normalizePartName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSpace(name)
	return strings.ToLower(name)
}
