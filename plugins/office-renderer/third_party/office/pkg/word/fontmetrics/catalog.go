package fontmetrics

import (
	"strings"

	shared "office/pkg/fontmetrics"
	"office/pkg/ooxml/fontdata"
	wordir "office/pkg/word/ir"
)

// BuildCatalog lifts usable embedded Word font faces into the shared Office
// metrics catalog. EOT/embedded-source normalization stays owned by fontdata;
// callers receive only parsed SFNT faces suitable for layout measurement.
//
// The bundled metric tables for the common Word families come first, so a face
// the document itself embeds always wins the family/style match while documents
// that do not embed their fonts still measure with the authoring family's real
// advance widths.
func BuildCatalog(doc *wordir.Document) *shared.Catalog {
	cat := shared.BundledCatalog()
	if cat == nil {
		cat = &shared.Catalog{}
	}
	if doc == nil {
		if cat.Empty() {
			return nil
		}
		return cat
	}
	embedded := &shared.Catalog{}
	for _, font := range doc.EmbeddedFonts {
		if font == nil {
			continue
		}
		for _, face := range font.Faces {
			if face == nil || len(face.Data) == 0 {
				continue
			}
			for _, source := range fontdata.EmbeddedSources(face.Data, face.ContentType, face.Family) {
				if source.Format != "truetype" && source.Format != "opentype" {
					continue
				}
				parsed, ok := shared.ParseSFNT(source.Data)
				if !ok {
					continue
				}
				embedded.Faces = append(embedded.Faces, shared.NamedFace{
					Family: face.Family,
					Weight: face.Weight,
					Italic: strings.EqualFold(strings.TrimSpace(face.Style), "italic"),
					Face:   parsed,
				})
				break
			}
		}
	}
	// The match prefers the first face with the best score, so embedded faces
	// must precede the bundled ones.
	cat.Faces = append(embedded.Faces, cat.Faces...)
	if cat.Empty() {
		return nil
	}
	return cat
}
