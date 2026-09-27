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

	HasFillColor     bool
	FillColor        string
	HasFillOpacity   bool
	FillOpacity      float64
	HasFill          bool
	FillEnabled      bool
	HasFillType      bool
	FillType         uint32
	HasFillBackColor bool
	FillBackColor    string
	HasFillAngle     bool
	FillAngleDeg     float64
	HasFillFocus     bool
	FillFocus        int32
	HasFillToLeft    bool
	FillToLeft       float64
	HasFillToTop     bool
	FillToTop        float64
	HasFillToRight   bool
	FillToRight      float64
	HasFillToBottom  bool
	FillToBottom     float64

	HasLineColor   bool
	LineColor      string
	HasLineWidth   bool
	LineWidthEMU   int64
	HasLineDash    bool
	LineDash       string
	HasLineOpacity bool
	LineOpacity    float64
	HasLineJoin    bool
	LineJoin       string
	HasLineCap     bool
	LineCap        string
	HasLine        bool
	LineEnabled    bool

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

	HasShadow         bool
	ShadowEnabled     bool
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
		case escher.PropFillOpacity:
			out.HasFillOpacity = true
			out.FillOpacity = Fixed16ToFraction(v)
		case escher.PropFillBackColor:
			out.HasFillBackColor = true
			out.FillBackColor = resolve(v)
		case escher.PropFillAngle:
			out.HasFillAngle = true
			out.FillAngleDeg = float64(int32(v)) / 65536.0
		case escher.PropFillFocus:
			out.HasFillFocus = true
			out.FillFocus = int32(v)
		case escher.PropFillToLeft:
			out.HasFillToLeft = true
			out.FillToLeft = Fixed16ToFraction(v)
		case escher.PropFillToTop:
			out.HasFillToTop = true
			out.FillToTop = Fixed16ToFraction(v)
		case escher.PropFillToRight:
			out.HasFillToRight = true
			out.FillToRight = Fixed16ToFraction(v)
		case escher.PropFillToBottom:
			out.HasFillToBottom = true
			out.FillToBottom = Fixed16ToFraction(v)
		case escher.PropFillBooleans:
			valueBits := v & 0xFFFF
			useBits := (v >> 16) & 0xFFFF
			// fFilled is bit 4 in the OfficeArt fill boolean set. A boolean
			// property can author unrelated fill flags without authoring fFilled;
			// in that case it must not cancel an earlier explicit fFilled value from
			// another FOPT tier/record.
			if useBits&(1<<4) != 0 {
				out.HasFill = true
				out.FillEnabled = valueBits&(1<<4) != 0
			} else if useBits == 0 && valueBits&1 != 0 {
				// Preserve the narrow compatibility form used by older producers:
				// an isolated low value bit can mean enabled. Never interpret an
				// unrelated non-zero use mask as an authored fFilled=false.
				out.HasFill = true
				out.FillEnabled = true
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
		case escher.PropLineOpacity:
			out.HasLineOpacity = true
			out.LineOpacity = Fixed16ToFraction(v)
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
		case escher.PropLineJoinStyle:
			out.HasLineJoin = true
			out.LineJoin = LineJoinFromOfficeArt(v)
		case escher.PropLineEndCapStyle:
			out.HasLineCap = true
			out.LineCap = LineCapFromOfficeArt(v)

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

		case escher.PropShadowBooleans:
			valueBits := v & 0xFFFF
			useBits := (v >> 16) & 0xFFFF
			// fShadow is conceptual PID 0x023E: value bit 14, use bit 30.
			if useBits&(1<<14) != 0 {
				out.HasShadow = true
				out.ShadowEnabled = valueBits&(1<<14) != 0
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
	if incoming.HasFillOpacity {
		base.HasFillOpacity = true
		base.FillOpacity = incoming.FillOpacity
	}
	if incoming.HasFillType {
		base.HasFillType = true
		base.FillType = incoming.FillType
	}
	if incoming.HasFillBackColor {
		base.HasFillBackColor = true
		base.FillBackColor = incoming.FillBackColor
	}
	if incoming.HasFillAngle {
		base.HasFillAngle = true
		base.FillAngleDeg = incoming.FillAngleDeg
	}
	if incoming.HasFillFocus {
		base.HasFillFocus = true
		base.FillFocus = incoming.FillFocus
	}
	if incoming.HasFillToLeft {
		base.HasFillToLeft = true
		base.FillToLeft = incoming.FillToLeft
	}
	if incoming.HasFillToTop {
		base.HasFillToTop = true
		base.FillToTop = incoming.FillToTop
	}
	if incoming.HasFillToRight {
		base.HasFillToRight = true
		base.FillToRight = incoming.FillToRight
	}
	if incoming.HasFillToBottom {
		base.HasFillToBottom = true
		base.FillToBottom = incoming.FillToBottom
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
	if incoming.HasLineOpacity {
		base.HasLineOpacity = true
		base.LineOpacity = incoming.LineOpacity
	}
	if incoming.HasLineJoin {
		base.HasLineJoin = true
		base.LineJoin = incoming.LineJoin
	}
	if incoming.HasLineCap {
		base.HasLineCap = true
		base.LineCap = incoming.LineCap
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
	if incoming.HasShadow {
		base.HasShadow = true
		base.ShadowEnabled = incoming.ShadowEnabled
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
