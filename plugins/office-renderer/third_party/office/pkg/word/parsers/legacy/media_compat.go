package legacy

import "bytes"

// EMF preview dimensions are interpreted at Office's historical 96 DPI when
// converted to EMUs. Raster BLIP sizing is owned by doc/officeart.
const legacyMetafileDefaultDPI = 96.0

// legacyPixelsToEMU remains a narrow compatibility utility for EMF preview
// conversion. Raster BLIP sizing is owned by pkg/word/parsers/doc/officeart.
func legacyPixelsToEMU(px int) int64 {
	return int64(float64(px) / legacyMetafileDefaultDPI * 914400.0)
}

// detectLegacyImageMIME is writer-side signature detection only. It does not
// scan or decode document image streams; production DOC image parsing is owned
// by the canonical Word97/DOC OfficeArt paths.
func detectLegacyImageMIME(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}):
		return "image/png"
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8:
		return "image/jpeg"
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		return "image/gif"
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		return "image/bmp"
	default:
		return ""
	}
}
