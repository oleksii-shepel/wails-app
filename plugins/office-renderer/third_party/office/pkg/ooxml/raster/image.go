package raster

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// DecodeImage decodes a PNG or JPEG from raw bytes.
func DecodeImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return img, nil
}

// RasterizeSVG renders an SVG string to an RGBA image at the given width and height.
func RasterizeSVG(svg string, width, height int) (*image.RGBA, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader([]byte(svg)), oksvg.WarnErrorMode)
	if err != nil {
		return nil, fmt.Errorf("parse svg: %w", err)
	}
	icon.SetTarget(0, 0, float64(width), float64(height))
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	scanner := rasterx.NewScannerGV(width, height, img, img.Bounds())
	raster := rasterx.NewDasher(width, height, scanner)
	icon.Draw(raster, 1)
	return img, nil
}
