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
