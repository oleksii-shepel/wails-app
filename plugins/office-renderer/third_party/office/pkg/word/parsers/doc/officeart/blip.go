package officeart

import (
	"bytes"
	"encoding/binary"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	officeartlegacy "office/pkg/officeart/legacy"
	officeartescher "office/pkg/officeart/legacy/escher"
	wordir "office/pkg/word/ir"
)

const defaultImageDPI = 96.0

// BLIP is one OfficeArt picture record discovered in DOC source order.
type BLIP struct {
	RecordType uint16
	Payload    []byte
	MIMEType   string
	Data       []byte
}

// ExtractBLIPs discovers OfficeArt image records without application-specific
// parsing. The shared OfficeArt decoder owns UID/metafile/raster extraction.
func ExtractBLIPs(streams ...[]byte) []BLIP {
	var out []BLIP
	for _, stream := range streams {
		if len(stream) < officeartescher.HeaderSize {
			continue
		}
		records := officeartescher.Scan(stream, func(h officeartescher.Header) bool {
			return officeartlegacy.IsBlipRecordType(h.Type) && !h.IsContainer()
		})
		for _, record := range records {
			media := officeartlegacy.ExtractBLIP(record.Header.Type, record.Data)
			if media.MIME == "" || len(media.Data) == 0 {
				continue
			}
			out = append(out, BLIP{
				RecordType: record.Header.Type,
				Payload:    append([]byte(nil), record.Data...),
				MIMEType:   media.MIME,
				Data:       append([]byte(nil), media.Data...),
			})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Image projects one decoded BLIP into renderable Word IR. Raster formats are
// preserved directly; EMF is converted to SVG by the canonical DOC OfficeArt
// layer so callers do not need a legacy vector adapter.
func (b BLIP) Image() *wordir.Image {
	if image := b.RasterImage(); image != nil {
		return image
	}
	return b.MetafileImage()
}

// RasterImage projects a decoded raster BLIP into Word IR.
func (b BLIP) RasterImage() *wordir.Image {
	switch b.MIMEType {
	case "image/png", "image/jpeg", "image/gif", "image/bmp":
	default:
		return nil
	}
	if len(b.Data) == 0 {
		return nil
	}
	imageIR := &wordir.Image{MIMEType: b.MIMEType, Data: append([]byte(nil), b.Data...)}
	width, height := imageDimensions(b.Data, b.MIMEType)
	if width > 0 && height > 0 {
		imageIR.Width = pixelsToEMU(width)
		imageIR.Height = pixelsToEMU(height)
	}
	return imageIR
}

// ProjectImages decodes and projects all renderable DOC OfficeArt images in
// source order. Unsupported vector formats are skipped rather than exposed as
// unusable browser media.
func ProjectImages(streams ...[]byte) []*wordir.Image {
	blips := ExtractBLIPs(streams...)
	if len(blips) == 0 {
		return nil
	}
	out := make([]*wordir.Image, 0, len(blips))
	for _, blip := range blips {
		if image := blip.Image(); image != nil {
			out = append(out, image)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// AttachOLEPreviews applies canonical EMF previews discovered in the Data
// stream to embedded-object runs in source order.
func AttachOLEPreviews(objects []wordir.Run, dataStream []byte) {
	if len(objects) == 0 || len(dataStream) == 0 {
		return
	}
	previewIndex := 0
	for _, blip := range ExtractBLIPs(dataStream) {
		if previewIndex >= len(objects) {
			break
		}
		preview := blip.MetafileImage()
		if preview == nil {
			continue
		}
		copyPreview := *preview
		copyPreview.Data = append([]byte(nil), preview.Data...)
		objects[previewIndex].OLEPreview = &copyPreview
		previewIndex++
	}
}

func imageDimensions(data []byte, mime string) (int, int) {
	if mime == "image/bmp" {
		if len(data) >= 26 && bytes.Equal(data[:2], []byte{'B', 'M'}) {
			width := int(int32(binary.LittleEndian.Uint32(data[18:22])))
			height := int(int32(binary.LittleEndian.Uint32(data[22:26])))
			if width < 0 {
				width = -width
			}
			if height < 0 {
				height = -height
			}
			return width, height
		}
		return 0, 0
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func pixelsToEMU(px int) int64 {
	return int64(float64(px) / defaultImageDPI * 914400.0)
}
