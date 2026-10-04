package units_test

import (
	"math"
	"testing"

	"office/pkg/ooxml/units"
)

func TestTwipAndEMUConversions(t *testing.T) {
	if got := units.InchesToTwips(1.5); got != 2160 {
		t.Fatalf("InchesToTwips(1.5) = %d, want 2160", got)
	}
	if got := units.PointsToTwips(12); got != 240 {
		t.Fatalf("PointsToTwips(12) = %d, want 240", got)
	}
	if got := units.InchesToEMU(2); got != 1828800 {
		t.Fatalf("InchesToEMU(2) = %d, want 1828800", got)
	}
	if got := units.TwipsToEMU(1440); got != units.EMUsPerInch {
		t.Fatalf("TwipsToEMU(1440) = %d, want %d", got, units.EMUsPerInch)
	}
	if got := units.EMUToTwips(units.EMUsPerInch); got != units.TwipsPerInch {
		t.Fatalf("EMUToTwips(EMUsPerInch) = %d, want %d", got, units.TwipsPerInch)
	}
}

func TestParseMeasureTwips(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		want   int64
		wantOK bool
	}{
		{name: "bare twips", value: "12240", want: 12240, wantOK: true},
		{name: "negative twips", value: "-240", want: -240, wantOK: true},
		{name: "points", value: "612pt", want: 12240, wantOK: true},
		{name: "fractional points", value: "7.5pt", want: 150, wantOK: true},
		{name: "inches", value: "1in", want: 1440, wantOK: true},
		{name: "centimeters", value: "2.54cm", want: 1440, wantOK: true},
		{name: "millimeters", value: "10mm", want: 567, wantOK: true},
		{name: "picas", value: "1pc", want: 240, wantOK: true},
		{name: "emu", value: "914400emu", want: 1440, wantOK: true},
		{name: "percentage is not a length", value: "50%", wantOK: false},
		{name: "garbage", value: "wide", wantOK: false},
		{name: "empty", value: "", wantOK: false},
		{name: "non finite", value: "NaNpt", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := units.ParseMeasureTwips(tt.value)
			if ok != tt.wantOK {
				t.Fatalf("ParseMeasureTwips(%q) ok = %v, want %v", tt.value, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("ParseMeasureTwips(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestFloatingPointRoundTrips(t *testing.T) {
	if diff := math.Abs(units.TwipsToInches(720) - 0.5); diff > 1e-9 {
		t.Fatalf("TwipsToInches(720) diff = %f", diff)
	}
	if diff := math.Abs(units.TwipsToPoints(30) - 1.5); diff > 1e-9 {
		t.Fatalf("TwipsToPoints(30) diff = %f", diff)
	}
	if diff := math.Abs(units.TwipsToCentimeters(units.CentimetersToTwips(2.54)) - 2.54); diff > 0.01 {
		t.Fatalf("centimeter round-trip diff = %f", diff)
	}
	if diff := math.Abs(units.EMUToInches(units.InchesToEMU(0.75)) - 0.75); diff > 1e-9 {
		t.Fatalf("EMUToInches round-trip diff = %f", diff)
	}
}
