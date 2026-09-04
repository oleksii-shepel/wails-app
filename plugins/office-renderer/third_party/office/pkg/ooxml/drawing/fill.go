package drawing

import (
	"encoding/base64"
	"fmt"
	"math"
	"strings"
)

// FillType represents the type of fill.
type FillType string

const (
	FillTypeNone     FillType = "none"
	FillTypeSolid    FillType = "solid"
	FillTypeGradient FillType = "gradient"
	FillTypePattern  FillType = "pattern"
	FillTypePicture  FillType = "picture"
)

// Fill represents a DrawingML fill.
type Fill interface {
	Type() FillType
}

// NoFill represents no fill (transparent).
type NoFill struct{}

func (f NoFill) Type() FillType { return FillTypeNone }

// SolidFill represents a solid color fill.
type SolidFill struct {
	Color       string  // Hex RGB color (e.g., "FF0000")
	Alpha       float64 // Transparency (0.0 = transparent, 1.0 = opaque)
	AlphaMod    float64 // Alpha modulation (0.0-1.0, 1.0 = no change)
	AlphaOff    float64 // Alpha offset (-1.0 to 1.0)
	ThemeColor  string  // Theme color reference (e.g., "accent1")
	ThemeShade  string  // Theme shade modifier
	ThemeTint   string  // Theme tint modifier
	Shade       float64 // DrawingML shade transform (0.0-1.0)
	Tint        float64 // DrawingML tint transform (0.0-1.0)
	HasShade    bool    // Whether a DrawingML shade transform is explicitly present
	HasTint     bool    // Whether a DrawingML tint transform is explicitly present
	Red         float64 // Red channel replacement (0.0-1.0)
	RedMod      float64 // Red channel modulation (0.0-1.0)
	RedOff      float64 // Red channel offset (0.0-1.0)
	Green       float64 // Green channel replacement (0.0-1.0)
	GreenMod    float64 // Green channel modulation (0.0-1.0)
	GreenOff    float64 // Green channel offset (0.0-1.0)
	Blue        float64 // Blue channel replacement (0.0-1.0)
	BlueMod     float64 // Blue channel modulation (0.0-1.0)
	BlueOff     float64 // Blue channel offset (0.0-1.0)
	HasRed      bool    // Whether a DrawingML red transform is explicitly present
	HasRedMod   bool    // Whether a DrawingML redMod transform is explicitly present
	HasRedOff   bool    // Whether a DrawingML redOff transform is explicitly present
	HasGreen    bool    // Whether a DrawingML green transform is explicitly present
	HasGreenMod bool    // Whether a DrawingML greenMod transform is explicitly present
	HasGreenOff bool    // Whether a DrawingML greenOff transform is explicitly present
	HasBlue     bool    // Whether a DrawingML blue transform is explicitly present
	HasBlueMod  bool    // Whether a DrawingML blueMod transform is explicitly present
	HasBlueOff  bool    // Whether a DrawingML blueOff transform is explicitly present
	Hue         float64 // Hue replacement in degrees
	Sat         float64 // Saturation replacement (0.0-1.0)
	Lum         float64 // Lightness replacement (0.0-1.0)
	HasHue      bool    // Whether a DrawingML hue transform is explicitly present
	HasSat      bool    // Whether a DrawingML sat transform is explicitly present
	HasLum      bool    // Whether a DrawingML lum transform is explicitly present
	LumMod      float64 // Luminance modulation (0.0-1.0, 1.0 = no change)
	LumOff      float64 // Luminance offset (0.0-1.0, 0.0 = no change)
	SatMod      float64 // Saturation modulation (0.0-1.0, 1.0 = no change)
	SatOff      float64 // Saturation offset (0.0-1.0, 0.0 = no change)
	HueMod      float64 // Hue modulation (0.0-1.0, 1.0 = no change)
	HueOff      float64 // Hue offset in degrees
	Gray        bool    // Convert the color to grayscale
	Comp        bool    // Complement the color
	Inv         bool    // Invert the color
	Gamma       bool    // Apply gamma correction
	InvGamma    bool    // Apply inverse gamma correction
}

func (f SolidFill) Type() FillType { return FillTypeSolid }

// ApplyColorTransforms applies luminance modulation/offset, saturation modulation/offset,
// and hue modulation/offset to a hex color string. Returns the modified hex color.
func (f SolidFill) ApplyColorTransforms(hex string) string {
	if hex == "" {
		return hex
	}
	r, g, b, ok := parseHexRGB(hex)
	if !ok {
		return hex
	}

	// Apply luminance modulation (lumMod) and offset (lumOff)
	if f.HasLum {
		h, s, _ := rgbToHSL(r, g, b)
		r, g, b = hslToRGB(h, s, clampUnit(f.Lum))
	}
	if (f.LumMod > 0 && f.LumMod != 1.0) || f.LumOff != 0 {
		applyLum := func(channel uint8) uint8 {
			value := float64(channel)
			if f.LumMod > 0 {
				value *= f.LumMod
			}
			value += 255 * f.LumOff
			if value < 0 {
				value = 0
			}
			if value > 255 {
				value = 255
			}
			return uint8(math.Round(value))
		}
		r = applyLum(r)
		g = applyLum(g)
		b = applyLum(b)
	}

	// Apply saturation modulation (satMod) and offset (satOff)
	if f.HasSat {
		h, _, l := rgbToHSL(r, g, b)
		r, g, b = hslToRGB(h, clampUnit(f.Sat), l)
	}
	if f.SatMod > 0 && f.SatMod != 1.0 {
		gray := uint8(float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114)
		r = uint8(float64(gray) + (float64(r)-float64(gray))*f.SatMod)
		g = uint8(float64(gray) + (float64(g)-float64(gray))*f.SatMod)
		b = uint8(float64(gray) + (float64(b)-float64(gray))*f.SatMod)
	}
	if f.SatOff > 0 {
		gray := uint8(float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114)
		off := uint8(f.SatOff * 255)
		r = uint8(float64(r) + (float64(gray)-float64(r))*float64(off)/255.0)
		g = uint8(float64(g) + (float64(gray)-float64(g))*float64(off)/255.0)
		b = uint8(float64(b) + (float64(gray)-float64(b))*float64(off)/255.0)
	}

	// Apply hue modulation (hueMod) and offset (hueOff)
	if f.HasHue || (f.HueMod > 0 && f.HueMod != 1.0) || f.HueOff != 0 {
		h, s, l := rgbToHSL(r, g, b)
		if f.HasHue {
			h = math.Mod(f.Hue, 360)
			if h < 0 {
				h += 360
			}
		}
		if f.HueMod > 0 && f.HueMod != 1.0 {
			h = h * f.HueMod
		}
		if f.HueOff != 0 {
			h = h + f.HueOff
		}
		// Normalize hue to 0-360
		h = math.Mod(h, 360)
		if h < 0 {
			h += 360
		}
		r, g, b = hslToRGB(h, s, l)
	}
	if f.Comp {
		h, s, l := rgbToHSL(r, g, b)
		h = math.Mod(h+180, 360)
		r, g, b = hslToRGB(h, s, l)
	}
	if f.HasShade {
		r = uint8(math.Round(float64(r) * clampUnit(f.Shade)))
		g = uint8(math.Round(float64(g) * clampUnit(f.Shade)))
		b = uint8(math.Round(float64(b) * clampUnit(f.Shade)))
	}
	if f.HasTint {
		tint := clampUnit(f.Tint)
		r = uint8(math.Round(float64(r) + (255-float64(r))*tint))
		g = uint8(math.Round(float64(g) + (255-float64(g))*tint))
		b = uint8(math.Round(float64(b) + (255-float64(b))*tint))
	}
	r = applyChannelTransform(r, f.Red, f.HasRed, f.RedMod, f.HasRedMod, f.RedOff, f.HasRedOff)
	g = applyChannelTransform(g, f.Green, f.HasGreen, f.GreenMod, f.HasGreenMod, f.GreenOff, f.HasGreenOff)
	b = applyChannelTransform(b, f.Blue, f.HasBlue, f.BlueMod, f.HasBlueMod, f.BlueOff, f.HasBlueOff)
	if f.Gray {
		gray := uint8(float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114)
		r, g, b = gray, gray, gray
	}
	if f.Inv {
		r, g, b = 255-r, 255-g, 255-b
	}
	if f.Gamma {
		r = gammaTransform(r, 2.2)
		g = gammaTransform(g, 2.2)
		b = gammaTransform(b, 2.2)
	}
	if f.InvGamma {
		r = gammaTransform(r, 1.0/2.2)
		g = gammaTransform(g, 1.0/2.2)
		b = gammaTransform(b, 1.0/2.2)
	}

	return formatHexRGB(r, g, b)
}

func (f SolidFill) ResolvedAlpha() float64 {
	alpha := f.Alpha
	if alpha <= 0 {
		alpha = 1.0
	}
	if f.AlphaMod > 0 {
		alpha *= f.AlphaMod
	}
	if f.AlphaOff != 0 {
		alpha += f.AlphaOff
	}
	return clampUnit(alpha)
}

func gammaTransform(channel uint8, gamma float64) uint8 {
	if gamma <= 0 {
		return channel
	}
	value := math.Pow(float64(channel)/255.0, gamma) * 255.0
	return uint8(math.Round(value))
}

func applyChannelTransform(channel uint8, absolute float64, hasAbsolute bool, mod float64, hasMod bool, off float64, hasOff bool) uint8 {
	value := float64(channel)
	if hasAbsolute {
		value = 255 * clampUnit(absolute)
	}
	if hasMod {
		value *= clampUnit(mod)
	}
	if hasOff {
		value += 255 * clampUnit(off)
	}
	if value > 255 {
		value = 255
	}
	return uint8(math.Round(value))
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// rgbToHSL converts RGB (0-255) to HSL (hue 0-360, saturation 0-1, lightness 0-1).
func rgbToHSL(r, g, b uint8) (h, s, l float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	l = (max + min) / 2.0

	if max == min {
		return 0, 0, l
	}

	d := max - min
	if l > 0.5 {
		s = d / (2.0 - max - min)
	} else {
		s = d / (max + min)
	}

	switch max {
	case rf:
		h = (gf - bf) / d
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/d + 2
	case bf:
		h = (rf-gf)/d + 4
	}
	h *= 60.0

	return h, s, l
}

// hslToRGB converts HSL (hue 0-360, saturation 0-1, lightness 0-1) to RGB (0-255).
func hslToRGB(h, s, l float64) (r, g, b uint8) {
	if s == 0 {
		gray := uint8(l * 255)
		return gray, gray, gray
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q

	h = h / 360.0
	r = uint8(hueToRGB(p, q, h+1.0/3.0) * 255)
	g = uint8(hueToRGB(p, q, h) * 255)
	b = uint8(hueToRGB(p, q, h-1.0/3.0) * 255)
	return r, g, b
}

// hueToRGB is a helper for HSL to RGB conversion.
func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	if t < 1.0/6.0 {
		return p + (q-p)*6*t
	}
	if t < 1.0/2.0 {
		return q
	}
	if t < 2.0/3.0 {
		return p + (q-p)*(2.0/3.0-t)*6
	}
	return p
}

// GradientType represents the type of gradient.
type GradientType string

const (
	GradientTypeLinear GradientType = "linear"
	GradientTypeRadial GradientType = "radial"
	GradientTypePath   GradientType = "path"
)

// GradientStop represents a color stop in a gradient.
type GradientStop struct {
	Position float64 // 0.0 to 1.0
	Color    string  // Hex RGB color
	Alpha    float64 // Transparency
}

// GradientFill represents a gradient fill.
type GradientFill struct {
	GradientType    GradientType
	Angle           float64 // Degrees for linear gradients
	Stops           []GradientStop
	TileRect        *Rect  // For tile rectangle
	Flip            string // "x", "y", "xy", or ""
	RotateWithShape bool
}

func (f GradientFill) Type() FillType { return FillTypeGradient }

// PatternType represents the type of pattern fill.
type PatternType string

const (
	PatternTypeSolidDiamond     PatternType = "solidDiamond"
	PatternTypeDarkHorizontal   PatternType = "darkHorizontal"
	PatternTypeDarkVertical     PatternType = "darkVertical"
	PatternTypeDarkUpDiagonal   PatternType = "darkUpDiagonal"
	PatternTypeDarkDownDiagonal PatternType = "darkDownDiagonal"
	PatternTypeSmallGrid        PatternType = "smallGrid"
	PatternTypeLargeGrid        PatternType = "largeGrid"
	PatternTypeDashedHorizontal PatternType = "dashedHorizontal"
	PatternTypeDashedVertical   PatternType = "dashedVertical"
	// Light variants
	PatternTypeLightHorizontal        PatternType = "lightHorizontal"
	PatternTypeLightVertical          PatternType = "lightVertical"
	PatternTypeLightUpDiagonal        PatternType = "lightUpDiagonal"
	PatternTypeLightDownDiagonal      PatternType = "lightDownDiagonal"
	PatternTypeLargeConfetti          PatternType = "largeConfetti"
	PatternTypeSmallConfetti          PatternType = "smallConfetti"
	PatternTypeLargeCheckerBoard      PatternType = "largeCheckerBoard"
	PatternTypeSmallCheckerBoard      PatternType = "smallCheckerBoard"
	PatternTypeOpenDiamond            PatternType = "openDiamond"
	PatternTypeDiagonalBrick          PatternType = "diagonalBrick"
	PatternTypeHorizontalBrick        PatternType = "horizontalBrick"
	PatternTypeWeaveBraidBig          PatternType = "wideUpwardDiagonal"
	PatternTypeWeaveBraidNarrow       PatternType = "narrowUpwardDiagonal"
	PatternTypeDottedGrid             PatternType = "dottedGrid"
	PatternTypeDottedDiamond          PatternType = "dottedDiamond"
	PatternTypeDivot                  PatternType = "divot"
	PatternTypeShingle                PatternType = "shingle"
	PatternTypeWave                   PatternType = "wave"
	PatternTypeTrellis                PatternType = "trellis"
	PatternTypeSphere                 PatternType = "sphere"
	PatternTypeNarrowDownwardDiagonal PatternType = "narrowDownwardDiagonal"
	PatternTypeWideDownwardDiagonal   PatternType = "wideDownwardDiagonal"
	PatternTypeDashedUpDiagonal       PatternType = "dashedUpDiagonal"
	PatternTypeDashedDownDiagonal     PatternType = "dashedDownDiagonal"
	PatternTypeCross                  PatternType = "cross"
	PatternTypeDiagonalCross          PatternType = "diagonalCross"
	PatternTypeSmallCheckerCenter     PatternType = "plaid"
	PatternTypeZigZag                 PatternType = "zigZag"
	PatternTypeSolidFill              PatternType = "solid"
	PatternTypePercent5               PatternType = "percent5"
	PatternTypePercent10              PatternType = "percent10"
	PatternTypePercent20              PatternType = "percent20"
	PatternTypePercent25              PatternType = "percent25"
	PatternTypePercent30              PatternType = "percent30"
	PatternTypePercent40              PatternType = "percent40"
	PatternTypePercent50              PatternType = "percent50"
	PatternTypePercent60              PatternType = "percent60"
	PatternTypePercent70              PatternType = "percent70"
	PatternTypePercent75              PatternType = "percent75"
	PatternTypePercent80              PatternType = "percent80"
	PatternTypePercent90              PatternType = "percent90"
)

// PatternFill represents a pattern fill.
type PatternFill struct {
	PatternType PatternType
	Foreground  string // Hex RGB color
	Background  string // Hex RGB color
	ForeAlpha   float64
	BackAlpha   float64
}

func (f PatternFill) Type() FillType { return FillTypePattern }

// PictureFill represents a picture/image fill.
type PictureFill struct {
	ImageURL        string // URL or data URL of the image
	EmbedID         string // Relationship ID when the image is package-local
	ImageData       []byte // Raw image bytes when materialized during resolve
	ContentType     string // MIME type for ImageData
	Tile            bool   // Whether to tile the image
	Stretch         bool   // Whether to stretch the image
	TileOffsetX     int64  // EMU; horizontal offset of the first tile
	TileOffsetY     int64  // EMU; vertical offset of the first tile
	TileScaleX      int    // thousandths of a percent (100000 = 100%)
	TileScaleY      int    // thousandths of a percent (100000 = 100%)
	TileFlip        string // "x", "y", "xy", or empty
	TileAlign       string // e.g. "tl", "ctr", etc.
	Crop            *Rect  // Source crop rectangle (0-1 normalized)
	FillRect        *Rect  // Destination rectangle for stretch fills (0-1 normalized)
	DPI             int    // DPI used to calculate the picture size; 0 means use embedded/default DPI
	RotWithShape    bool   // Whether the picture fill rotates with the containing shape
	RotWithShapeSet bool   // Whether RotWithShape was explicitly authored
	Alpha           float64
	BlipEffects     []Effect // Blip image filter effects to apply
}

func (f PictureFill) Type() FillType { return FillTypePicture }

// SourceURL resolves the most concrete image source available for the fill.
func (f PictureFill) SourceURL() string {
	if strings.TrimSpace(f.ImageURL) != "" {
		return strings.TrimSpace(f.ImageURL)
	}
	if len(f.ImageData) == 0 {
		return ""
	}
	contentType := strings.TrimSpace(f.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(f.ImageData))
}

// ParseFill creates a Fill from DrawingML XML data.
// This is a simplified parser - a real implementation would parse XML.
func ParseFill(data map[string]interface{}) (Fill, error) {
	// Check for noFill
	if _, ok := data["noFill"]; ok {
		return NoFill{}, nil
	}

	// Check for solidFill
	if solid, ok := data["solidFill"].(map[string]interface{}); ok {
		fill := SolidFill{Alpha: 1.0}

		// Check for srgbClr
		if srgb, ok := solid["srgbClr"].(map[string]interface{}); ok {
			if val, ok := srgb["val"].(string); ok {
				fill.Color = val
			}
			if alpha, ok := srgb["alpha"].(float64); ok {
				fill.Alpha = alpha / 100000.0 // Convert from percentage
			}
		}

		// Check for schemeClr
		if scheme, ok := solid["schemeClr"].(map[string]interface{}); ok {
			if val, ok := scheme["val"].(string); ok {
				fill.ThemeColor = val
			}
			if shade, ok := scheme["shade"].(string); ok {
				fill.ThemeShade = shade
			}
			if tint, ok := scheme["tint"].(string); ok {
				fill.ThemeTint = tint
			}
		}

		return fill, nil
	}

	// Check for gradFill
	if grad, ok := data["gradFill"].(map[string]interface{}); ok {
		fill := GradientFill{GradientType: GradientTypeLinear}

		// Parse gradient type
		if lin, ok := grad["lin"].(map[string]interface{}); ok {
			if ang, ok := lin["ang"].(float64); ok {
				// DrawingML uses 60000 per degree
				fill.Angle = ang / 60000.0
			}
		}

		// Parse gradient stops
		if gsLst, ok := grad["gsLst"].(map[string]interface{}); ok {
			if gsList, ok := gsLst["gs"].([]interface{}); ok {
				for _, gs := range gsList {
					if stopMap, ok := gs.(map[string]interface{}); ok {
						stop := GradientStop{Alpha: 1.0}

						if pos, ok := stopMap["pos"].(float64); ok {
							stop.Position = pos / 100000.0 // Convert from percentage
						}

						if srgb, ok := stopMap["srgbClr"].(map[string]interface{}); ok {
							if val, ok := srgb["val"].(string); ok {
								stop.Color = val
							}
							if alpha, ok := srgb["alpha"].(float64); ok {
								stop.Alpha = alpha / 100000.0
							}
						}

						fill.Stops = append(fill.Stops, stop)
					}
				}
			}
		}

		return fill, nil
	}

	// Default to no fill
	return NoFill{}, nil
}
