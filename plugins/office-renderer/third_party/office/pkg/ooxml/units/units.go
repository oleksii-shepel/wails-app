package units

import (
	"math"
	"strconv"
	"strings"
)

const (
	TwipsPerInch       = 1440
	TwipsPerPoint      = 20
	TwipsPerCentimeter = 1440.0 / 2.54
	PointsPerInch      = 72
	EMUsPerInch        = 914400
	EMUsPerTwip        = EMUsPerInch / TwipsPerInch
)

func InchesToTwips(inches float64) int64 {
	return round(inches * TwipsPerInch)
}

func TwipsToInches(twips int64) float64 {
	return float64(twips) / TwipsPerInch
}

func PointsToTwips(points float64) int64 {
	return round(points * TwipsPerPoint)
}

func TwipsToPoints(twips int64) float64 {
	return float64(twips) / TwipsPerPoint
}

func CentimetersToTwips(cm float64) int64 {
	return round(cm * TwipsPerCentimeter)
}

func TwipsToCentimeters(twips int64) float64 {
	return float64(twips) / TwipsPerCentimeter
}

func InchesToEMU(inches float64) int64 {
	return round(inches * EMUsPerInch)
}

func EMUToInches(emu int64) float64 {
	return float64(emu) / EMUsPerInch
}

func TwipsToEMU(twips int64) int64 {
	return twips * EMUsPerTwip
}

func EMUToTwips(emu int64) int64 {
	return round(float64(emu) / EMUsPerTwip)
}

func round(value float64) int64 {
	return int64(math.Round(value))
}

// ParseMeasureTwips converts an OOXML measurement into twips. Transitional
// documents store a bare count of twentieths of a point, while the strict
// vocabulary writes an ST_UniversalMeasure such as "612pt", "1in" or "2.5cm".
// Values that are not lengths (percentages, bad input, non-finite numbers)
// report ok=false so callers keep their own handling for those.
func ParseMeasureTwips(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		return n, true
	}

	unit := ""
	for _, suffix := range []string{"pt", "in", "cm", "mm", "pc", "pi", "emu"} {
		if strings.HasSuffix(value, suffix) {
			unit = suffix
			value = strings.TrimSpace(strings.TrimSuffix(value, suffix))
			break
		}
	}
	if unit == "" {
		return 0, false
	}
	measure, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(measure) || math.IsInf(measure, 0) {
		return 0, false
	}
	switch unit {
	case "pt":
		return round(measure * TwipsPerPoint), true
	case "in":
		return round(measure * TwipsPerInch), true
	case "cm":
		return round(measure * TwipsPerCentimeter), true
	case "mm":
		return round(measure * TwipsPerCentimeter / 10), true
	case "pc", "pi":
		// A pica is twelve points.
		return round(measure * TwipsPerPoint * 12), true
	case "emu":
		return round(measure / EMUsPerTwip), true
	}
	return 0, false
}
