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
func BuildCatalog(doc *wordir.Document) *shared.Catalog {
	if doc == nil {
		return nil
	}
	cat := &shared.Catalog{}
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
				cat.Faces = append(cat.Faces, shared.NamedFace{
					Family: face.Family,
					Weight: face.Weight,
					Italic: strings.EqualFold(strings.TrimSpace(face.Style), "italic"),
					Face:   parsed,
				})
				break
			}
		}
	}
	if cat.Empty() {
		return nil
	}
	return cat
}
