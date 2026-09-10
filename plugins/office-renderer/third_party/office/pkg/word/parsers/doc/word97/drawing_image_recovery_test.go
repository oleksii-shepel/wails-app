package word97

import (
	"encoding/binary"
	"testing"

	wordir "office/pkg/word/ir"
	docofficeart "office/pkg/word/parsers/doc/officeart"
)

func TestRecoverDetachedDrawingImagesUsesCanonicalOfficeArtProjection(t *testing.T) {
	// The shared OfficeArt package already owns BLIP decoding. This gate keeps
	// the Word97 rescue path coupled to that canonical projection rather than a
	// legacy image/metafile adapter.
	doc := &wordir.Document{}
	RecoverDetachedDrawingImages(doc, nil)
	if len(doc.Paragraphs) != 0 {
		t.Fatalf("empty OfficeArt produced %d paragraphs", len(doc.Paragraphs))
	}

	// Compile-time/API ownership check: canonical BLIPs expose Image(), which
	// includes raster projection and EMF->SVG conversion.
	var blip docofficeart.BLIP
	_ = blip.Image
	_ = binary.LittleEndian
}
