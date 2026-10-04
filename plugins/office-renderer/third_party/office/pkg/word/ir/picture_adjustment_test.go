package ir

import (
	"math"
	"testing"
)

func TestPictureLumCorrectionMatchesOfficePictureCorrections(t *testing.T) {
	tests := []struct {
		name           string
		bright         int
		contrast       int
		wantBrightness float64
		wantContrast   float64
	}{
		{name: "neutral", bright: 0, contrast: 0, wantBrightness: 1, wantContrast: 1},
		{name: "brightness shifts levels", bright: 50000, contrast: 0, wantBrightness: 1.5, wantContrast: 1},
		{name: "negative brightness darkens", bright: -30000, contrast: 0, wantBrightness: 0.7, wantContrast: 1},
		{name: "contrast 20 percent scales by 1.25", bright: 0, contrast: 20000, wantBrightness: 1, wantContrast: 1.25},
		{name: "contrast 50 percent scales by 2", bright: 0, contrast: 50000, wantBrightness: 1, wantContrast: 2},
		{name: "contrast 80 percent scales by 5", bright: 0, contrast: 80000, wantBrightness: 1, wantContrast: 5},
		{name: "negative contrast is linear", bright: 0, contrast: -70000, wantBrightness: 1, wantContrast: 0.3},
		{name: "full contrast saturates", bright: 0, contrast: 100000, wantBrightness: 1, wantContrast: maxOfficeArtPictureContrast},
		{name: "washout combines both", bright: 70000, contrast: -70000, wantBrightness: 1.7, wantContrast: 0.3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brightness, contrast := PictureLumCorrection(tt.bright, tt.contrast)
			if math.Abs(brightness-tt.wantBrightness) > 1e-9 {
				t.Errorf("brightness = %.4f, want %.4f", brightness, tt.wantBrightness)
			}
			if math.Abs(contrast-tt.wantContrast) > 1e-9 {
				t.Errorf("contrast = %.4f, want %.4f", contrast, tt.wantContrast)
			}
		})
	}
}
