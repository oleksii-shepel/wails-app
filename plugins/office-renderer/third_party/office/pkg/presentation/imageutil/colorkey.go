package imageutil

import (
	"bytes"
	"image"
	stdimagedraw "image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"strconv"
	"strings"

	"office/pkg/ooxml/drawing"
)

// MaterializeTransparentColorKeys converts exact transparent-color clrChange
// effects into raster alpha. Effects that are not exact transparent color keys,
// or images that cannot be decoded, are preserved for downstream rendering.
func MaterializeTransparentColorKeys(data []byte, mime string, effects []drawing.Effect) ([]byte, string, []drawing.Effect) {
	if len(data) == 0 || len(effects) == 0 {
		return data, mime, effects
	}
	keys := make(map[uint32]struct{})
	remaining := make([]drawing.Effect, 0, len(effects))
	for _, effect := range effects {
		cc, ok := effect.(drawing.ClrChange)
		if !ok {
			if p, yes := effect.(*drawing.ClrChange); yes && p != nil {
				cc, ok = *p, true
			}
		}
		if !ok || !cc.UseAlpha || cc.ToAlpha != 0 || !strings.EqualFold(cc.From, cc.To) {
			remaining = append(remaining, effect)
			continue
		}
		rgb, ok := parseRGBHex(cc.From)
		if !ok {
			remaining = append(remaining, effect)
			continue
		}
		keys[rgb] = struct{}{}
	}
	if len(keys) == 0 {
		return data, mime, effects
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, mime, effects
	}
	bounds := src.Bounds()
	dst := image.NewNRGBA(bounds)
	stdimagedraw.Draw(dst, bounds, src, bounds.Min, stdimagedraw.Src)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			off := dst.PixOffset(x, y)
			p := dst.Pix[off : off+4]
			rgb := uint32(p[0])<<16 | uint32(p[1])<<8 | uint32(p[2])
			if _, ok := keys[rgb]; ok {
				p[3] = 0
			}
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return data, mime, effects
	}
	return out.Bytes(), "image/png", remaining
}

func parseRGBHex(value string) (uint32, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return 0, false
	}
	v, err := strconv.ParseUint(value, 16, 24)
	return uint32(v), err == nil
}
