package drawing

import "math"

// ApproximateOpacity computes an overall opacity value from an effect list.
// It returns the computed opacity and true if any opacity-modifying effect
// was found.
func ApproximateOpacity(el EffectList) (float64, bool) {
	opacity := 1.0
	found := false
	for _, effect := range el.Effects {
		if updatedOpacity, ok := applyOpacityEffect(effect, opacity); ok {
			opacity = updatedOpacity
			found = true
			continue
		}
		if alpha, ok := approximateEffectOpacity(effect); ok {
			opacity = clampFloat(opacity*alpha, 0, 1)
			found = true
		}
	}
	return clampFloat(opacity, 0, 1), found
}

func applyOpacityEffect(effect Effect, current float64) (float64, bool) {
	switch e := effect.(type) {
	case AlphaMod:
		return clampFloat(current*(float64(e.Amount)/100000.0), 0, 1), true
	case *AlphaMod:
		if e == nil {
			return 0, false
		}
		return clampFloat(current*(float64(e.Amount)/100000.0), 0, 1), true
	case AlphaModFix:
		return clampFloat(current*(float64(e.Amount)/100000.0), 0, 1), true
	case *AlphaModFix:
		if e == nil {
			return 0, false
		}
		return clampFloat(current*(float64(e.Amount)/100000.0), 0, 1), true
	case AlphaRepl:
		return clampFloat(float64(e.Alpha)/100000.0, 0, 1), true
	case *AlphaRepl:
		if e == nil {
			return 0, false
		}
		return clampFloat(float64(e.Alpha)/100000.0, 0, 1), true
	case AlphaOff:
		return clampFloat(current+(float64(e.Amount)/100000.0), 0, 1), true
	case *AlphaOff:
		if e == nil {
			return 0, false
		}
		return clampFloat(current+(float64(e.Amount)/100000.0), 0, 1), true
	case AlphaBiLevel:
		return clampFloat(current*(float64(e.Threshold)/100000.0), 0, 1), true
	case *AlphaBiLevel:
		if e == nil {
			return 0, false
		}
		return clampFloat(current*(float64(e.Threshold)/100000.0), 0, 1), true
	default:
		return 0, false
	}
}

func approximateEffectOpacity(effect Effect) (float64, bool) {
	switch e := effect.(type) {
	case Shadow:
		return approximateAlphaValue(e.Alpha)
	case *Shadow:
		if e == nil {
			return 0, false
		}
		return approximateAlphaValue(e.Alpha)
	case InnerShadow:
		return approximateAlphaValue(e.Alpha)
	case *InnerShadow:
		if e == nil {
			return 0, false
		}
		return approximateAlphaValue(e.Alpha)
	case Glow:
		return approximateAlphaValue(e.Alpha)
	case *Glow:
		if e == nil {
			return 0, false
		}
		return approximateAlphaValue(e.Alpha)
	case Reflection:
		return approximateAlphaValue(e.Alpha)
	case *Reflection:
		if e == nil {
			return 0, false
		}
		return approximateAlphaValue(e.Alpha)
	case FillOverlay:
		return approximateFillOpacity(e.Fill)
	case *FillOverlay:
		if e == nil {
			return 0, false
		}
		return approximateFillOpacity(e.Fill)
	// Non-opacity-modifying effects: recognized so alphaMod wrappers don't discard them.
	case Blur, SoftEdge, RelativeOffset, TransformEffect,
		Grayscale, DeSat, DeSatLow, DeSatMod,
		HSLEffect, Lum, ClrChange, ClrRepl, Duotone, BlendMode,
		AlphaInv, AlphaCeiling, AlphaFloor, AlphaOutset:
		return 1.0, true
	case *Blur:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *SoftEdge:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *RelativeOffset:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *TransformEffect:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *Grayscale:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *DeSat:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *DeSatLow:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *DeSatMod:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *HSLEffect:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *Lum:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *ClrChange:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *ClrRepl:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *Duotone:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *BlendMode:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *AlphaInv:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *AlphaCeiling:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *AlphaFloor:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	case *AlphaOutset:
		if e == nil {
			return 0, false
		}
		return 1.0, true
	default:
		return 0, false
	}
}

func approximateAlphaValue(alpha float64) (float64, bool) {
	if alpha <= 0 {
		return 0, false
	}
	return clampFloat(alpha, 0, 1), true
}

func approximateFillOpacity(fill Fill) (float64, bool) {
	switch f := fill.(type) {
	case SolidFill:
		return f.ResolvedAlpha(), true
	case *SolidFill:
		if f == nil {
			return 0, false
		}
		return f.ResolvedAlpha(), true
	case GradientFill:
		if len(f.Stops) == 0 {
			return 0, false
		}
		total := 0.0
		for _, stop := range f.Stops {
			alpha := stop.Alpha
			if alpha <= 0 {
				alpha = 1
			}
			total += clampFloat(alpha, 0, 1)
		}
		return clampFloat(total/float64(len(f.Stops)), 0, 1), true
	case *GradientFill:
		if f == nil {
			return 0, false
		}
		return approximateFillOpacity(*f)
	case PatternFill:
		return clampFloat(math.Max(f.ForeAlpha, f.BackAlpha), 0, 1), true
	case *PatternFill:
		if f == nil {
			return 0, false
		}
		return approximateFillOpacity(*f)
	case PictureFill:
		return approximateAlphaValue(f.Alpha)
	case *PictureFill:
		if f == nil {
			return 0, false
		}
		return approximateAlphaValue(f.Alpha)
	default:
		if fill == nil {
			return 0, false
		}
		return 0, false
	}
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
