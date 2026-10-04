package ir

import "math"

// PictureLumCorrection converts an authored DrawingML luminance (a:lum)
// brightness/contrast percentage pair into the picture corrections Word
// applies when it paints a picture.
//
// Word shifts levels for brightness and scales them for contrast, combining the
// two into the affine map
//
//	out = scale*in + (1-scale)/2 + offset*(1+scale)/2
//
// where offset is the authored brightness fraction. brightness is returned in
// the IR's 1+offset form so that a neutral adjustment stays 1.
//
// The positive contrast range is asymptotic rather than linear: Office renders
// +50% as a 2x scale, +80% as 5x and +95% as 20x (verified against Word's own
// PDF output), approaching a hard threshold as the authored value approaches
// +100%. Negative contrast stays linear (1 + value). The scale is capped at
// OfficeArt's maximum so a +100% endpoint stays finite.
func PictureLumCorrection(bright, contrast int) (brightness, contrastScale float64) {
	offset := float64(bright) / 100000.0
	brightness = math.Max(0, 1+offset)
	scale := float64(contrast) / 100000.0
	switch {
	case scale < 0:
		contrastScale = math.Max(0, 1+scale)
	case scale >= 1:
		contrastScale = maxOfficeArtPictureContrast
	default:
		contrastScale = 1 / (1 - scale)
	}
	return brightness, contrastScale
}

// maxOfficeArtPictureContrast mirrors MS-ODRAW's maximum pictureContrast
// (0x7FFFFFFF in 16.16 fixed point); larger scales are indistinguishable from a
// hard threshold but stay finite for downstream filters.
const maxOfficeArtPictureContrast = 32768.0

// PictureLumPercentages is the inverse of [PictureLumCorrection]: it converts an
// IR picture brightness (1+offset) and contrast scale back into the authored
// DrawingML a:lum percentages, in thousandths of a percent. Writing the
// percentages back keeps a saved document's corrections identical to the ones
// that were read.
func PictureLumPercentages(brightness, contrastScale float64) (bright, contrast int) {
	if brightness > 0 {
		bright = int(math.Round((brightness - 1) * 100000))
	}
	switch {
	case contrastScale <= 0:
	case contrastScale == 1:
	case contrastScale > 1:
		// scale = 1/(1-c) for c >= 0.
		if contrastScale >= maxOfficeArtPictureContrast {
			contrast = 100000
		} else {
			contrast = int(math.Round((1 - 1/contrastScale) * 100000))
		}
	default:
		// scale = 1+c for c < 0.
		contrast = int(math.Round((contrastScale - 1) * 100000))
	}
	return bright, contrast
}
