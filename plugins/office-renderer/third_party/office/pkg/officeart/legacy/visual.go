package legacy

import (
	"math"

	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

// ColorResolver resolves an OfficeArt COLORREF in the application color
// context. DOC/XLS normally use direct RGB; PPT can additionally resolve
// scheme-indexed colors.
type ColorResolver func(raw uint32) string

// VisualProperties is the normalized semantic result of one or more OfficeArt
// FOPT tables. Presence flags are explicit so zero-valued authored properties
// survive primary/secondary/tertiary overlay.
type VisualProperties struct {
	HasRotation bool
	RotationDeg float64

	HasFillColor bool
	FillColor    string
	HasFill      bool
	FillEnabled  bool
	HasFillType  bool
	FillType     uint32

	HasLineColor bool
	LineColor    string
	HasLineWidth bool
	LineWidthEMU int64
	HasLineDash  bool
	LineDash     string
	HasLine      bool
	LineEnabled  bool

	HasStartArrow    bool
	StartArrow       ArrowKind
	StartArrowWidth  ArrowSize
	StartArrowLength ArrowSize
	HasEndArrow      bool
	EndArrow         ArrowKind
	EndArrowWidth    ArrowSize
	EndArrowLength   ArrowSize

	Text TextProperties

	HasCrop       bool
	HasCropTop    bool
	HasCropBottom bool
	HasCropLeft   bool
	HasCropRight  bool
	Crop          ir.Crop

	HasPictureBrightness bool
	PictureBrightness    float64
	HasPictureContrast   bool
	PictureContrast      float64
	HasPictureGrayscale  bool
	PictureGrayscale     bool
	HasPictureBiLevel    bool
	PictureBiLevel       bool

	HasShadowColor    bool
	ShadowColor       string
	HasShadowOpacity  bool
	ShadowOpacity     float64
	HasShadowOffsetX  bool
	ShadowOffsetXEMU  int64
	HasShadowOffsetY  bool
	ShadowOffsetYEMU  int64
	HasShadowSoftness bool
	ShadowSoftnessEMU int64
}

// DecodeVisualProperties interprets OfficeArt properties that are genuinely
// common to DOC/XLS/PPT. Application-specific properties remain in adapters.
func DecodeVisualProperties(props []escher.Property, resolve ColorResolver) VisualProperties {
	if resolve == nil {
		resolve = ColorRefToHex
	}
	var out VisualProperties
	out.Text = DecodeTextProperties(props)

	for _, prop := range props {
		if prop.IsComplex || prop.IsBlip {
			continue
		}
		v := prop.Value
		switch prop.ID {
		case escher.PropRotation:
			out.HasRotation = true
			out.RotationDeg = RotationDegrees(v)

		case escher.PropFillType:
			out.HasFillType = true
			out.FillType = v
		case escher.PropFillColor:
			out.HasFillColor = true
			out.FillColor = resolve(v)
		case escher.PropFillBooleans:
			out.HasFill = true
			valueBits := v & 0xFFFF
			useBits := (v >> 16) & 0xFFFF
			// fFilled is bit 4 in the OfficeArt fill boolean set.
			if useBits&(1<<4) != 0 {
				out.FillEnabled = valueBits&(1<<4) != 0
			} else {
				// Older producer paths have historically surfaced bit 0.
				out.FillEnabled = valueBits&1 != 0
			}

		case escher.PropLineColor:
			out.HasLineColor = true
			out.LineColor = resolve(v)
		case escher.PropLineWidth:
			out.HasLineWidth = true
			out.LineWidthEMU = int64(int32(v))
		case escher.PropLineDashing:
			out.HasLineDash = true
			out.LineDash = DashName(v)
		case escher.PropLineBooleans:
			out.HasLine = true
			out.LineEnabled = ((v >> 3) & 1) == 1
			// Respect the use bit when present; absent use means "leave prior".
			if ((v >> 19) & 1) == 0 {
				out.HasLine = false
			}
		case escher.PropLineStartArrow:
			out.HasStartArrow = true
			out.StartArrow = ArrowKindFromOfficeArt(v)
		case escher.PropLineEndArrow:
			out.HasEndArrow = true
			out.EndArrow = ArrowKindFromOfficeArt(v)
		case escher.PropLineStartArrowWidth:
			out.StartArrowWidth = ArrowSizeFromOfficeArt(v)
		case escher.PropLineStartArrowLength:
			out.StartArrowLength = ArrowSizeFromOfficeArt(v)
		case escher.PropLineEndArrowWidth:
			out.EndArrowWidth = ArrowSizeFromOfficeArt(v)
		case escher.PropLineEndArrowLength:
			out.EndArrowLength = ArrowSizeFromOfficeArt(v)

		case escher.PropCropTop:
			out.HasCrop = true
			out.HasCropTop = true
			out.Crop.Top = Fixed16ToFraction(v)
		case escher.PropCropBottom:
			out.HasCrop = true
			out.HasCropBottom = true
			out.Crop.Bottom = Fixed16ToFraction(v)
		case escher.PropCropLeft:
			out.HasCrop = true
			out.HasCropLeft = true
			out.Crop.Left = Fixed16ToFraction(v)
		case escher.PropCropRight:
			out.HasCrop = true
			out.HasCropRight = true
			out.Crop.Right = Fixed16ToFraction(v)

		case escher.PropPictureBrightness:
			out.HasPictureBrightness = true
			out.PictureBrightness = math.Max(0, math.Min(2, 1+float64(int32(v))/32768.0))
		case escher.PropPictureContrast:
			out.HasPictureContrast = true
			out.PictureContrast = math.Max(0, float64(int32(v))/65536.0)
		case escher.PropPictureGray:
			out.HasPictureGrayscale = true
			out.PictureGrayscale = v != 0
		case escher.PropPictureBiLevel:
			out.HasPictureBiLevel = true
			out.PictureBiLevel = v != 0
		case escher.PropPictureBooleans:
			valueBits := v & 0xFFFF
			useBits := (v >> 16) & 0xFFFF
			if useBits&0x0004 != 0 {
				out.HasPictureGrayscale = true
				out.PictureGrayscale = valueBits&0x0004 != 0
			}
			if useBits&0x0002 != 0 {
				out.HasPictureBiLevel = true
				out.PictureBiLevel = valueBits&0x0002 != 0
			}

		case escher.PropShadowColor:
			out.HasShadowColor = true
			out.ShadowColor = resolve(v)
		case escher.PropShadowOpacity:
			out.HasShadowOpacity = true
			out.ShadowOpacity = math.Max(0, math.Min(1, float64(int32(v))/65536.0))
		case escher.PropShadowOffsetX:
			out.HasShadowOffsetX = true
			out.ShadowOffsetXEMU = int64(int32(v))
		case escher.PropShadowOffsetY:
			out.HasShadowOffsetY = true
			out.ShadowOffsetYEMU = int64(int32(v))
		case escher.PropShadowSoftness:
			out.HasShadowSoftness = true
			out.ShadowSoftnessEMU = int64(int32(v))
		}
	}
	return out
}

// MergeVisualProperties overlays incoming authored properties on base. This is
// the common primary → secondary → tertiary FOPT merge rule.
func MergeVisualProperties(base, incoming VisualProperties) VisualProperties {
	if incoming.HasRotation {
		base.HasRotation = true
		base.RotationDeg = incoming.RotationDeg
	}
	if incoming.HasFillColor {
		base.HasFillColor = true
		base.FillColor = incoming.FillColor
	}
	if incoming.HasFill {
		base.HasFill = true
		base.FillEnabled = incoming.FillEnabled
	}
	if incoming.HasFillType {
		base.HasFillType = true
		base.FillType = incoming.FillType
	}
	if incoming.HasLineColor {
		base.HasLineColor = true
		base.LineColor = incoming.LineColor
	}
	if incoming.HasLineWidth {
		base.HasLineWidth = true
		base.LineWidthEMU = incoming.LineWidthEMU
	}
	if incoming.HasLineDash {
		base.HasLineDash = true
		base.LineDash = incoming.LineDash
	}
	if incoming.HasLine {
		base.HasLine = true
		base.LineEnabled = incoming.LineEnabled
	}
	if incoming.HasStartArrow {
		base.HasStartArrow = true
		base.StartArrow = incoming.StartArrow
	}
	if incoming.StartArrowWidth != "" {
		base.StartArrowWidth = incoming.StartArrowWidth
	}
	if incoming.StartArrowLength != "" {
		base.StartArrowLength = incoming.StartArrowLength
	}
	if incoming.HasEndArrow {
		base.HasEndArrow = true
		base.EndArrow = incoming.EndArrow
	}
	if incoming.EndArrowWidth != "" {
		base.EndArrowWidth = incoming.EndArrowWidth
	}
	if incoming.EndArrowLength != "" {
		base.EndArrowLength = incoming.EndArrowLength
	}
	base.Text = MergeTextProperties(base.Text, incoming.Text)
	if incoming.HasCrop {
		base.HasCrop = true
		if incoming.HasCropTop {
			base.HasCropTop = true
			base.Crop.Top = incoming.Crop.Top
		}
		if incoming.HasCropBottom {
			base.HasCropBottom = true
			base.Crop.Bottom = incoming.Crop.Bottom
		}
		if incoming.HasCropLeft {
			base.HasCropLeft = true
			base.Crop.Left = incoming.Crop.Left
		}
		if incoming.HasCropRight {
			base.HasCropRight = true
			base.Crop.Right = incoming.Crop.Right
		}
	}
	if incoming.HasPictureBrightness {
		base.HasPictureBrightness = true
		base.PictureBrightness = incoming.PictureBrightness
	}
	if incoming.HasPictureContrast {
		base.HasPictureContrast = true
		base.PictureContrast = incoming.PictureContrast
	}
	if incoming.HasPictureGrayscale {
		base.HasPictureGrayscale = true
		base.PictureGrayscale = incoming.PictureGrayscale
	}
	if incoming.HasPictureBiLevel {
		base.HasPictureBiLevel = true
		base.PictureBiLevel = incoming.PictureBiLevel
	}
	if incoming.HasShadowColor {
		base.HasShadowColor = true
		base.ShadowColor = incoming.ShadowColor
	}
	if incoming.HasShadowOpacity {
		base.HasShadowOpacity = true
		base.ShadowOpacity = incoming.ShadowOpacity
	}
	if incoming.HasShadowOffsetX {
		base.HasShadowOffsetX = true
		base.ShadowOffsetXEMU = incoming.ShadowOffsetXEMU
	}
	if incoming.HasShadowOffsetY {
		base.HasShadowOffsetY = true
		base.ShadowOffsetYEMU = incoming.ShadowOffsetYEMU
	}
	if incoming.HasShadowSoftness {
		base.HasShadowSoftness = true
		base.ShadowSoftnessEMU = incoming.ShadowSoftnessEMU
	}
	return base
}
