package drawing

// EffectType represents the type of visual effect.
type EffectType string

const (
	EffectTypeShadow       EffectType = "shadow"
	EffectTypeInnerShadow  EffectType = "innerShadow"
	EffectTypeGlow         EffectType = "glow"
	EffectTypeSoftEdge     EffectType = "softEdge"
	EffectTypeReflection   EffectType = "reflection"
	EffectTypeBlur         EffectType = "blur"
	EffectTypeThreeD       EffectType = "threeD"
	EffectTypeGrayscale    EffectType = "grayscl"
	EffectTypeBiLevel      EffectType = "biLevel"
	EffectTypeDuotone      EffectType = "duotone"
	EffectTypeAlphaBiLevel EffectType = "alphaBiLevel"
	EffectTypeAlphaMod     EffectType = "alphaMod"
	EffectTypeAlphaModFix  EffectType = "alphaModFix"
	EffectTypeAlphaOff     EffectType = "alphaOff"
	EffectTypeAlphaOutset  EffectType = "alphaOutset"
	EffectTypeAlphaRepl    EffectType = "alphaRepl"
	EffectTypeAlphaInv     EffectType = "alphaInv"
	EffectTypeAlphaCeiling EffectType = "alphaCeiling"
	EffectTypeAlphaFloor   EffectType = "alphaFloor"
	EffectTypeDeSat        EffectType = "deSat"
	EffectTypeDeSatLow     EffectType = "deSatLow"
	EffectTypeDeSatMod     EffectType = "deSatMod"
	EffectTypeHSLEffect    EffectType = "hsleff"
	EffectTypeLum          EffectType = "lum"
	EffectTypeClrChange    EffectType = "clrChange"
	EffectTypeClrRepl      EffectType = "clrRepl"
	EffectTypeFillOverlay  EffectType = "fillOverlay"
	EffectTypeBlendMode    EffectType = "blend"
	EffectTypeRelOff       EffectType = "relOff"
	EffectTypeTint         EffectType = "tint"
	EffectTypeTransform    EffectType = "xfrm"
)

// Effect represents a DrawingML visual effect.
type Effect interface {
	Type() EffectType
}

// Shadow represents a shadow effect.
type Shadow struct {
	Color           string  // Hex RGB color
	Alpha           float64 // Transparency
	Blur            float64 // Blur radius in points
	Distance        float64 // Offset distance in points
	Angle           float64 // Angle in degrees (0 = right, 90 = up)
	ScaleX          float64 // Horizontal scale (1.0 = 100%)
	ScaleY          float64 // Vertical scale (1.0 = 100%)
	SkewX           float64 // Horizontal skew in degrees
	SkewY           float64 // Vertical skew in degrees
	Rotate          float64 // Rotation in degrees
	Align           bool    // Whether an explicit alignment is present (compatibility flag)
	Alignment       string  // DrawingML alignment token (tl, t, tr, l, ctr, r, bl, b, br)
	RotateWithShape bool    // Whether the shadow rotates with its source shape
	ContainerType   string  // DrawingML effect container type ("sib" or "tree") when known
}

func (s Shadow) Type() EffectType { return EffectTypeShadow }

// InnerShadow represents an inner shadow effect (a:innerShdw).
// It maps to CSS box-shadow with the inset keyword.
type InnerShadow struct {
	Color         string  // Hex RGB color
	Alpha         float64 // Transparency (0.0–1.0)
	Blur          float64 // Blur radius in points
	Distance      float64 // Offset distance in points
	Angle         float64 // Angle in degrees (clockwise from right; same convention as Shadow)
	ContainerType string  // DrawingML effect container type ("sib" or "tree") when known
}

func (s InnerShadow) Type() EffectType { return EffectTypeInnerShadow }

// Glow represents a glow effect.
type Glow struct {
	Color  string  // Hex RGB color
	Alpha  float64 // Transparency
	Radius float64 // Glow radius in points
	// Grow indicates whether the glow expands beyond the shape bounds
	Grow          bool
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (g Glow) Type() EffectType { return EffectTypeGlow }

// SoftEdge represents a soft edge (feather) effect.
type SoftEdge struct {
	Radius        float64 // Soft edge radius in points
	ContainerType string  // DrawingML effect container type ("sib" or "tree") when known
}

func (se SoftEdge) Type() EffectType { return EffectTypeSoftEdge }

// Reflection represents a reflection effect.
type Reflection struct {
	Alpha           float64 // Transparency of reflection (0.0 to 1.0)
	Blur            float64 // Blur radius in points
	Distance        float64 // Distance from shape in points
	Size            float64 // Size of reflection (0.0 to 1.0)
	Direction       float64 // Direction (0 = below, 180 = above, 90 = right, 270 = left)
	Fade            bool    // Whether to fade the reflection
	StartAlpha      int     // DrawingML stA (0-100000)
	StartPosition   int     // DrawingML stPos (0-100000)
	EndAlpha        int     // DrawingML endA (0-100000)
	EndPosition     int     // DrawingML endPos (0-100000)
	FadeDirection   float64 // DrawingML fadeDir in degrees
	ScaleX          float64 // Horizontal scale (1.0 = 100%)
	ScaleY          float64 // Vertical scale (1.0 = 100%)
	SkewX           float64 // Horizontal skew in degrees
	SkewY           float64 // Vertical skew in degrees
	Alignment       string  // DrawingML alignment token
	RotateWithShape bool    // Whether the reflection rotates with its source shape
	ContainerType   string  // DrawingML effect container type ("sib" or "tree") when known
}

func (r Reflection) Type() EffectType { return EffectTypeReflection }

// Blur represents a blur effect.
type Blur struct {
	Radius        float64 // Blur radius in points
	Grow          bool    // Whether blur expands beyond shape bounds
	ContainerType string  // DrawingML effect container type ("sib" or "tree") when known
}

func (b Blur) Type() EffectType { return EffectTypeBlur }

// ThreeD approximates DrawingML scene3d/sp3d shape styling for HTML renderers.
type ThreeD struct {
	CameraPreset      string
	CameraFOV         float64 // Perspective field of view in degrees when authored
	CameraZoom        float64 // Camera zoom multiplier (1.0 = 100%)
	LightRig          string
	LightDirection    string
	LightRotationX    float64
	LightRotationY    float64
	LightRotationZ    float64
	Material          string
	RotationX         float64
	RotationY         float64
	RotationZ         float64
	ExtrusionDepth    float64
	ContourWidth      float64
	BevelTopWidth     float64
	BevelTopHeight    float64
	BevelTopPreset    string
	BevelBottomWidth  float64
	BevelBottomHeight float64
	BevelBottomPreset string
	ContourColor      string
	ExtrusionColor    string
}

func (t ThreeD) Type() EffectType { return EffectTypeThreeD }

// Grayscale represents a grayscale image effect (a:grayscl).
type Grayscale struct{}

func (g Grayscale) Type() EffectType { return EffectTypeGrayscale }

// BiLevel represents a bi-level (black/white) image effect (a:biLevel).
type BiLevel struct {
	Threshold int // 0-100000 threshold value
}

func (b BiLevel) Type() EffectType { return EffectTypeBiLevel }

// AlphaBiLevel represents an alpha bi-level effect (a:alphaBiLevel).
type AlphaBiLevel struct {
	Threshold     int    // 0-100000 threshold value
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (a AlphaBiLevel) Type() EffectType { return EffectTypeAlphaBiLevel }

// Duotone represents a duotone image effect (a:duotone).
type Duotone struct {
	Color1 string // First color (hex)
	Color2 string // Second color (hex)
}

func (d Duotone) Type() EffectType { return EffectTypeDuotone }

// AlphaMod represents an alpha modifier effect (a:alphaMod).
type AlphaMod struct {
	Amount        int    // Percentage in thousandths of a percent (0-100000)
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (a AlphaMod) Type() EffectType { return EffectTypeAlphaMod }

// AlphaOff represents an alpha offset effect (a:alphaOff).
type AlphaOff struct {
	Amount        int    // Signed percentage offset in thousandths of a percent
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (a AlphaOff) Type() EffectType { return EffectTypeAlphaOff }

// AlphaModFix represents a fixed alpha modifier effect (a:alphaModFix).
type AlphaModFix struct {
	Amount        int    // Percentage in thousandths of a percent (0-100000)
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (a AlphaModFix) Type() EffectType { return EffectTypeAlphaModFix }

// AlphaRepl represents an alpha replace effect (a:alphaRepl).
type AlphaRepl struct {
	Alpha         int    // Alpha value in thousandths of a percent (0-100000)
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (a AlphaRepl) Type() EffectType { return EffectTypeAlphaRepl }

// AlphaOutset represents an alpha inset/outset effect (a:alphaOutset).
type AlphaOutset struct {
	Radius        float64 // Outset radius in points
	ContainerType string  // DrawingML effect container type ("sib" or "tree") when known
}

func (a AlphaOutset) Type() EffectType { return EffectTypeAlphaOutset }

// AlphaInv represents an alpha inverse effect (a:alphaInv).
type AlphaInv struct {
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (a AlphaInv) Type() EffectType { return EffectTypeAlphaInv }

// AlphaCeiling represents an alpha ceiling effect (a:alphaCeiling).
type AlphaCeiling struct {
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (a AlphaCeiling) Type() EffectType { return EffectTypeAlphaCeiling }

// AlphaFloor represents an alpha floor effect (a:alphaFloor).
type AlphaFloor struct {
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (a AlphaFloor) Type() EffectType { return EffectTypeAlphaFloor }

// DeSat represents a desaturation effect (a:deSat).
type DeSat struct{}

func (d DeSat) Type() EffectType { return EffectTypeDeSat }

// DeSatLow represents a low desaturation effect (a:deSatLow).
type DeSatLow struct{}

func (d DeSatLow) Type() EffectType { return EffectTypeDeSatLow }

// DeSatMod represents a desaturation modifier effect (a:deSatMod).
type DeSatMod struct {
	Amount int // Percentage in thousandths of a percent (0-100000)
}

func (d DeSatMod) Type() EffectType { return EffectTypeDeSatMod }

// HSLEffect represents an HSL modifier effect (a:hsleff).
type HSLEffect struct {
	Hue int // Hue shift in degrees (1/60000 degree units)
	Sat int // Saturation modifier (0-100000)
	Lum int // Luminance modifier (0-100000)
}

func (h HSLEffect) Type() EffectType { return EffectTypeHSLEffect }

// Lum represents a luminance effect (a:lum).
type Lum struct {
	Brightness int // Brightness modifier (0-100000)
	Contrast   int // Contrast modifier (0-100000)
}

func (l Lum) Type() EffectType { return EffectTypeLum }

// ClrChange represents a color change effect (a:clrChange).
type ClrChange struct {
	From      string  // Original color (hex)
	To        string  // Replacement color (hex)
	FromAlpha float64 // Resolved source alpha (0-1)
	ToAlpha   float64 // Resolved target alpha (0-1)
	UseAlpha  bool    // Whether alpha participates in color matching/replacement
}

func (c ClrChange) Type() EffectType { return EffectTypeClrChange }

// ClrRepl represents a color replacement effect (a:clrRepl).
type ClrRepl struct {
	Color string // Replacement color (hex)
}

func (c ClrRepl) Type() EffectType { return EffectTypeClrRepl }

// FillOverlay represents a fill overlay effect (a:fillOverlay).
type FillOverlay struct {
	Fill          Fill   // The fill to overlay
	BlendMode     string // DrawingML blend mode (for example: over, mult, screen)
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (f FillOverlay) Type() EffectType { return EffectTypeFillOverlay }

// BlendMode approximates a blend wrapper effect (a:blend) using CSS mix-blend-mode.
type BlendMode struct {
	Mode          string
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (b BlendMode) Type() EffectType { return EffectTypeBlendMode }

// RelativeOffset represents a relative translation effect (a:relOff).
type RelativeOffset struct {
	X             float64
	Y             float64
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (r RelativeOffset) Type() EffectType { return EffectTypeRelOff }

// Tint represents a tint effect (a:tint). This is only a coarse CSS approximation.
type Tint struct {
	Hue           int
	Amount        int
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (t Tint) Type() EffectType { return EffectTypeTint }

// TransformEffect represents a transform effect (a:xfrm).
type TransformEffect struct {
	ScaleX        float64
	ScaleY        float64
	SkewX         float64
	SkewY         float64
	TranslateX    float64
	TranslateY    float64
	ContainerType string // DrawingML effect container type ("sib" or "tree") when known
}

func (t TransformEffect) Type() EffectType { return EffectTypeTransform }

// EffectList represents a collection of effects to be applied in order.
type EffectList struct {
	Effects []Effect
}

// ParseEffect creates an Effect from DrawingML XML data.
func ParseEffect(data map[string]interface{}) (Effect, error) {
	// Check for outerShdw (outer shadow)
	if outerShdw, ok := data["outerShdw"].(map[string]interface{}); ok {
		shadow := Shadow{
			Color:    "000000",
			Alpha:    0.75,
			Blur:     3.0,
			Distance: 3.0,
			Angle:    45.0,
			ScaleX:   1.0,
			ScaleY:   1.0,
		}

		// Parse color
		if srgb, ok := outerShdw["srgbClr"].(map[string]interface{}); ok {
			if val, ok := srgb["val"].(string); ok {
				shadow.Color = val
			}
			if alpha, ok := srgb["alpha"].(float64); ok {
				shadow.Alpha = alpha / 100000.0
			}
		}

		// Parse blur radius
		if blurRad, ok := outerShdw["blurRad"].(float64); ok {
			shadow.Blur = EMUToPoints(int64(blurRad))
		}

		// Parse distance
		if dist, ok := outerShdw["dist"].(float64); ok {
			shadow.Distance = EMUToPoints(int64(dist))
		}

		// Parse direction
		if dir, ok := outerShdw["dir"].(float64); ok {
			// DrawingML uses 60000 per degree
			shadow.Angle = dir / 60000.0
		}

		return shadow, nil
	}

	// Check for glow
	if glow, ok := data["glow"].(map[string]interface{}); ok {
		glowEffect := Glow{
			Color:  "000000",
			Alpha:  0.5,
			Radius: 5.0,
		}

		// Parse color
		if srgb, ok := glow["srgbClr"].(map[string]interface{}); ok {
			if val, ok := srgb["val"].(string); ok {
				glowEffect.Color = val
			}
			if alpha, ok := srgb["alpha"].(float64); ok {
				glowEffect.Alpha = alpha / 100000.0
			}
		}

		// Parse radius
		if rad, ok := glow["rad"].(float64); ok {
			glowEffect.Radius = EMUToPoints(int64(rad))
		}

		return glowEffect, nil
	}

	// Check for softEdge
	if softEdge, ok := data["softEdge"].(map[string]interface{}); ok {
		se := SoftEdge{Radius: 2.0}

		// Parse radius
		if rad, ok := softEdge["rad"].(float64); ok {
			se.Radius = EMUToPoints(int64(rad))
		}

		return se, nil
	}

	// Default: no effect
	return nil, nil
}
