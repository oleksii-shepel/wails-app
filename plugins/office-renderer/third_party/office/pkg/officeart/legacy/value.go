package legacy

import (
	"fmt"
	"math"
)

// Fixed16ToFraction interprets a signed 16.16 OfficeArt fixed-point value and
// clamps the result to [0,1]. It is used for picture crop properties.
func Fixed16ToFraction(raw uint32) float64 {
	v := float64(int32(raw)) / 65536.0
	return math.Max(0, math.Min(1, v))
}

// RotationDegrees converts the signed 16.16 OfficeArt rotation value to a
// clockwise [0,360) degree value.
func RotationDegrees(raw uint32) float64 {
	deg := math.Mod(float64(int32(raw))/65536.0, 360)
	if deg < 0 {
		deg += 360
	}
	return deg
}

// ColorRefToHex decodes the direct RGB form of OfficeArt COLORREF. System,
// scheme, palette and other indexed forms deliberately return "" so individual
// format adapters can resolve them using their own color context.
func ColorRefToHex(raw uint32) string {
	// The high byte contains COLORREF flags. Direct RGB has no flag bits.
	if raw&0xFF000000 != 0 {
		return ""
	}
	r := byte(raw)
	g := byte(raw >> 8)
	b := byte(raw >> 16)
	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}
