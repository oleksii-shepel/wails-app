package units

import "math"

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
