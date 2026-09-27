package render

import (
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestPresentationMetafileFontFacesUsesCanonicalEmbeddedFonts(t *testing.T) {
	pres := &ir.Presentation{EmbeddedFonts: []*ir.EmbeddedFont{{
		Family: "Embedded Face",
		Faces: []*ir.EmbeddedFontFace{{
			Family:      "Embedded Face",
			Weight:      700,
			Style:       "italic",
			ContentType: "font/opentype",
			Data:        append([]byte("OTTO"), make([]byte, 24)...),
		}},
	}}}
	faces := presentationMetafileFontFaces(pres)
	if len(faces) != 1 {
		t.Fatalf("metafile faces = %d, want 1", len(faces))
	}
	if faces[0].Family != "Embedded Face" || !faces[0].Bold || !faces[0].Italic {
		t.Fatalf("metafile face = %#v", faces[0])
	}
	if string(faces[0].Data[:4]) != "OTTO" {
		t.Fatalf("metafile face did not preserve sfnt bytes: %q", faces[0].Data[:4])
	}
}
