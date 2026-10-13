package imageutil

import (
	"encoding/binary"
	"testing"
)

func jfifHeader(units byte, xDensity, yDensity uint16) []byte {
	out := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	out = append(out, "JFIF\x00"...)
	out = append(out, 1, 1, units)
	out = binary.BigEndian.AppendUint16(out, xDensity)
	out = binary.BigEndian.AppendUint16(out, yDensity)
	out = append(out, 0, 0) // thumbnail dimensions
	return append(out, 0xFF, 0xD9)
}

func pngChunk(chunkType string, data []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	out = append(out, chunkType...)
	out = append(out, data...)
	return binary.BigEndian.AppendUint32(out, 0) // CRC is not read
}

func pngHeader(ppuX uint32, unit byte) []byte {
	out := []byte("\x89PNG\r\n\x1a\n")
	ihdr := binary.BigEndian.AppendUint32(nil, 128)
	ihdr = binary.BigEndian.AppendUint32(ihdr, 128)
	ihdr = append(ihdr, 8, 6, 0, 0, 0)
	out = append(out, pngChunk("IHDR", ihdr)...)
	phys := binary.BigEndian.AppendUint32(nil, ppuX)
	phys = binary.BigEndian.AppendUint32(phys, ppuX)
	phys = append(phys, unit)
	return append(out, pngChunk("pHYs", phys)...)
}

func TestNaturalDPI(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int
	}{
		{name: "jpeg jfif density", data: jfifHeader(1, 75, 75), want: 75},
		{name: "jpeg density per centimeter", data: jfifHeader(2, 30, 30), want: 76},
		{name: "jpeg aspect ratio only", data: jfifHeader(0, 1, 1), want: 0},
		{name: "png pixels per meter", data: pngHeader(5906, 1), want: 150},
		{name: "png unknown unit", data: pngHeader(5906, 0), want: 0},
		{name: "not an image", data: []byte("not an image at all"), want: 0},
		{name: "empty", data: nil, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NaturalDPI(tt.data); got != tt.want {
				t.Fatalf("NaturalDPI = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNaturalDPIOfDeckTexture(t *testing.T) {
	// The 41246-2 WordArt texture is a JFIF JPEG tagged 75 dpi; its tiles only
	// match the reference PDF at that resolution.
	data := jfifHeader(1, 75, 75)
	if got := NaturalDPI(data); got != 75 {
		t.Fatalf("texture dpi = %d, want 75", got)
	}
}
