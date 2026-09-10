package word97

import (
	wordir "office/pkg/word/ir"
	docofficeart "office/pkg/word/parsers/doc/officeart"
)

// RecoverDetachedDrawingImages completes the Word97 lossless OfficeArt rescue
// path without routing BLIP decoding or metafile conversion through legacy.
//
// Normal anchored drawings have already been projected by DrawingProjection.
// ProjectImages handles raster BLIPs and canonical EMF->SVG conversion; the
// detached projector then suppresses images already present in the document and
// preserves only genuinely uncorrelated floating media.
func RecoverDetachedDrawingImages(doc *wordir.Document, streams ...[]byte) {
	AppendDetachedDrawingImages(doc, docofficeart.ProjectImages(streams...))
}
