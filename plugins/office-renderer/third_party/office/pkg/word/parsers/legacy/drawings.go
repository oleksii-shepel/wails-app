package legacy

import (
	"crypto/sha1"

	officeartescher "office/pkg/officeart/legacy/escher"
	wordir "office/pkg/word/ir"
	docofficeart "office/pkg/word/parsers/doc/officeart"
)

// Writer/fixture constants remain here until the legacy DOC writer is split
// from the parser package. Semantic OfficeArt decoding is owned by the shared
// OfficeArt frontend and pkg/word/parsers/doc/officeart.
const (
	legacyOfficeArtSpgrContainer       = 0xF003
	legacyOfficeArtSpContainer         = 0xF004
	legacyOfficeArtFSPGRRecord         = 0xF009
	legacyOfficeArtFSPRecord           = 0xF00A
	legacyOfficeArtFOPTRecord          = 0xF00B
	legacyOfficeArtSecondaryFOPT       = 0xF121
	legacyOfficeArtTertiaryFOPT        = 0xF122
	legacyOfficeArtTextLeftProperty    = 0x0081
	legacyOfficeArtTextTopProperty     = 0x0082
	legacyOfficeArtTextRightProperty   = 0x0083
	legacyOfficeArtTextBottomProperty  = 0x0084
	legacyOfficeArtWrapTextProperty    = 0x0085
	legacyOfficeArtAnchorTextProperty  = 0x0087
	legacyOfficeArtTextFlowProperty    = 0x0088
	legacyOfficeArtChildAnchorRecord   = 0xF00F
	legacyOfficeArtClientTextboxRecord = 0xF00D

	legacyOfficeArtRotationProperty          = 0x0004
	legacyOfficeArtFillColorProperty         = 0x0181
	legacyOfficeArtFillBoolsProperty         = 0x01BF
	legacyOfficeArtLineColorProperty         = 0x01C0
	legacyOfficeArtLineWidthProperty         = 0x01CB
	legacyOfficeArtLineBoolsProperty         = 0x01FF
	legacyOfficeArtCropFromTopProperty       = officeartescher.PropCropTop
	legacyOfficeArtCropFromBottomProperty    = officeartescher.PropCropBottom
	legacyOfficeArtCropFromLeftProperty      = officeartescher.PropCropLeft
	legacyOfficeArtCropFromRightProperty     = officeartescher.PropCropRight
	legacyOfficeArtPictureContrastProperty   = 0x0108
	legacyOfficeArtPictureBrightnessProperty = 0x0109
	legacyOfficeArtPictureGrayProperty       = 0x013D
	legacyOfficeArtPictureBiLevelProperty    = 0x013E
	legacyOfficeArtPictureBoolsProperty      = 0x013F
	legacyOfficeArtGeoLeftProperty           = 0x0140
	legacyOfficeArtGeoTopProperty            = 0x0141
	legacyOfficeArtGeoRightProperty          = 0x0142
	legacyOfficeArtGeoBottomProperty         = 0x0143
	legacyOfficeArtShapePathProperty         = 0x0144
	legacyOfficeArtVerticesProperty          = 0x0145
	legacyOfficeArtSegmentInfoProperty       = 0x0146
	legacyOfficeArtAdjust1Property           = 0x0147
	legacyOfficeArtAdjust2Property           = 0x0148
	legacyOfficeArtAdjust3Property           = 0x0149
	legacyOfficeArtAdjust4Property           = 0x014A
	legacyOfficeArtAdjust5Property           = 0x014B
	legacyOfficeArtAdjust6Property           = 0x014C
	legacyOfficeArtAdjust7Property           = 0x014D
	legacyOfficeArtAdjust8Property           = 0x014E

	legacyOfficeArtBlipEMF  = 0xF01A
	legacyOfficeArtBlipWMF  = 0xF01B
	legacyOfficeArtBlipPICT = 0xF01C
	legacyOfficeArtBlipJPEG = 0xF01D
	legacyOfficeArtBlipPNG  = 0xF01E
	legacyOfficeArtBlipDIB  = 0xF01F
)

// extractLegacyDrawingImages remains only as the explicit lossless/rescue-path
// adapter. BLIP discovery and raster projection are canonical; only EMF/WMF
// conversion still delegates to the narrow legacy metafile adapter.
func extractPreWord97DrawingImages(streams ...[]byte) []*wordir.Image {
	blips := docofficeart.ExtractBLIPs(streams...)
	if len(blips) == 0 {
		return nil
	}
	images := make([]*wordir.Image, 0, len(blips))
	for _, blip := range blips {
		if image := blip.RasterImage(); image != nil {
			images = append(images, image)
			continue
		}
		if blip.RecordType == legacyOfficeArtBlipEMF || blip.RecordType == legacyOfficeArtBlipWMF {
			if image := legacyOfficeArtMetafileImage(blip.RecordType, blip.Payload); image != nil {
				images = append(images, image)
			}
		}
	}
	if len(images) == 0 {
		return nil
	}
	return images
}

func extractLegacyDrawingMetafileImages(streams ...[]byte) []*wordir.Image {
	blips := docofficeart.ExtractBLIPs(streams...)
	if len(blips) == 0 {
		return nil
	}
	images := make([]*wordir.Image, 0, len(blips))
	for _, blip := range blips {
		if blip.RecordType != legacyOfficeArtBlipEMF && blip.RecordType != legacyOfficeArtBlipWMF {
			continue
		}
		if image := legacyOfficeArtMetafileImage(blip.RecordType, blip.Payload); image != nil {
			images = append(images, image)
		}
	}
	if len(images) == 0 {
		return nil
	}
	return images
}

func appendPreWord97DetachedDrawingImages(doc *wordir.Document, images []*wordir.Image) {
	if doc == nil || len(images) == 0 {
		return
	}
	seen := existingPreWord97DocImageDigests(doc)
	for _, img := range images {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		digest := sha1.Sum(img.Data)
		if _, ok := seen[digest]; ok {
			continue
		}
		seen[digest] = struct{}{}
		// This rescue path is fed by OfficeArt BLIPs recovered from the
		// WordDocument stream. Inline DOC pictures come through PICF/Data and are
		// handled by the canonical inline-image cursor, so a detached OfficeArt
		// image here necessarily represents a floating drawing whose anchor could
		// not be correlated losslessly.
		image := *img
		image.Floating = true
		paragraph := wordir.Paragraph{
			Index: len(doc.Paragraphs),
			Runs:  []wordir.Run{{Kind: "image", Image: &image}},
		}
		doc.Paragraphs = append(doc.Paragraphs, paragraph)
		doc.Blocks = append(doc.Blocks, wordir.Block{
			Kind:      "paragraph",
			Paragraph: &doc.Paragraphs[len(doc.Paragraphs)-1],
		})
	}
}

func existingPreWord97DocImageDigests(doc *wordir.Document) map[[20]byte]struct{} {
	seen := map[[20]byte]struct{}{}
	if doc == nil {
		return seen
	}
	for _, paragraph := range doc.Paragraphs {
		for _, run := range paragraph.Runs {
			if run.Image == nil || len(run.Image.Data) == 0 {
				continue
			}
			seen[sha1.Sum(run.Image.Data)] = struct{}{}
		}
	}
	return seen
}
