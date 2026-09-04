// Package legacytoooxml converts legacy PowerPoint presentations (.ppt) to OOXML
// PresentationML packages (.pptx).
package legacytoooxml

import (
	"fmt"

	"office/pkg/ooxml/media"
	presentationir "office/pkg/presentation/ir"
	presparts "office/pkg/presentation/parsers/ooxml/parts"
)

// Convert transforms a legacy presentation IR into an OOXML parts.Package
// that can be serialized to a .pptx file via presparts.WritePackage.
func Convert(pres *presentationir.Presentation) (*presparts.Package, error) {
	if pres == nil {
		return nil, fmt.Errorf("nil presentation")
	}

	c := &converter{
		pkg: &presparts.Package{
			Presentation: &presparts.Presentation{
				SlideSize: &presparts.SlideSize{
					CX: pres.SlideWidthEMU,
					CY: pres.SlideHeightEMU,
				},
				SlideIDs: []presparts.IDRef{},
			},
			Themes:       make(map[string]*presparts.Theme),
			MediaParts:   make(map[string]media.Part),
			SlideMasters: []*presparts.SlideMasterEntry{},
		},
		shapeID: 1,
	}

	// Create a default theme if one exists in IR.
	if pres.Theme != nil {
		c.mapTheme(pres.Theme)
	}

	// Ensure at least one slide master and layout exist.
	c.ensureDefaultMasters(pres)
	c.convertSlideMasters(pres.SlideMasters)

	// Convert each slide.
	for i, slide := range pres.Slides {
		if err := c.convertSlide(slide, i+1); err != nil {
			return nil, fmt.Errorf("slide %d: %w", i, err)
		}
	}

	c.convertEmbeddedFonts(pres.EmbeddedFonts)

	return c.pkg, nil
}

type converter struct {
	pkg          *presparts.Package
	nextRelID    int
	shapeID      int
	mediaNum     int
	embedNum     int
	smartArtNum  int
	currentEntry *presparts.SlideEntry
	shapeIDMap   map[*presentationir.Shape]int
}

func (c *converter) nextRID() string {
	c.nextRelID++
	return fmt.Sprintf("rId%d", c.nextRelID)
}

func (c *converter) nextShapeID() int {
	c.shapeID++
	return c.shapeID
}

func ptrInt64(v int64) *int64 {
	return &v
}

func (c *converter) nextMediaNum() int {
	c.mediaNum++
	return c.mediaNum
}

func (c *converter) nextEmbedNum() int {
	c.embedNum++
	return c.embedNum
}

func (c *converter) recordShapeID(shape *presentationir.Shape, id int) {
	if c.shapeIDMap != nil && shape != nil {
		c.shapeIDMap[shape] = id
	}
}

func ptrInt(v int) *int {
	return &v
}

func boolPtrIf(v bool) *bool {
	if !v {
		return nil
	}
	return &v
}