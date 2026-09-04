package fontdata

import (
	"strings"

	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
)

const relationshipTypeFont = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/font"

// StyleRefs identifies the relationship IDs for the four embedded font styles.
type StyleRefs struct {
	Regular    string
	Bold       string
	Italic     string
	BoldItalic string
}

// EmbeddedFontData stores the concrete font parts loaded for a declared font family.
type EmbeddedFontData struct {
	Typeface   string
	Regular    *FontPart
	Bold       *FontPart
	Italic     *FontPart
	BoldItalic *FontPart
}

// FontPart is a raw embedded font file loaded from the OPC package.
type FontPart struct {
	Name        string
	ContentType string
	Data        []byte
}

// LoadEmbeddedFont resolves the embedded font parts for a single font family.
func LoadEmbeddedFont(pkg *opc.Package, ownerPartName, family string, refs StyleRefs, byID map[string]rels.Relationship) *EmbeddedFontData {
	family = strings.TrimSpace(family)
	if family == "" {
		return nil
	}

	embedded := &EmbeddedFontData{Typeface: family}
	embedded.Regular = loadFontPart(pkg, ownerPartName, refs.Regular, byID)
	embedded.Bold = loadFontPart(pkg, ownerPartName, refs.Bold, byID)
	embedded.Italic = loadFontPart(pkg, ownerPartName, refs.Italic, byID)
	embedded.BoldItalic = loadFontPart(pkg, ownerPartName, refs.BoldItalic, byID)
	if embedded.Regular == nil && embedded.Bold == nil && embedded.Italic == nil && embedded.BoldItalic == nil {
		return nil
	}
	return embedded
}

func loadFontPart(pkg *opc.Package, ownerPartName, relID string, byID map[string]rels.Relationship) *FontPart {
	if pkg == nil {
		return nil
	}
	relID = strings.TrimSpace(relID)
	if relID == "" {
		return nil
	}
	relationship, ok := byID[relID]
	if !ok || relationship.Type != relationshipTypeFont || relationship.IsExternal() {
		return nil
	}
	partName := rels.ResolveTarget(ownerPartName, relationship.Target)
	part, ok := pkg.Part(partName)
	if !ok {
		return nil
	}
	return &FontPart{
		Name:        partName,
		ContentType: part.ContentType,
		Data:        append([]byte(nil), part.Data...),
	}
}
