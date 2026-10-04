package parts

import (
	"encoding/xml"
	"math"
	"regexp"
	"strconv"
)

var strictHalfPointAttribute = regexp.MustCompile(`(?i)(<(?:[[:alnum:]_]+:)?(?:sz|szCs|position|kern)\b[^>]*\b(?:[[:alnum:]_]+:)?val\s*=\s*["'])([0-9]+(?:\.[0-9]+)?)pt(["'])`)
var strictTwipAttribute = regexp.MustCompile(`(?i)(\b(?:[[:alnum:]_]+:)?[[:alnum:]_]+\s*=\s*["'])([0-9]+(?:\.[0-9]+)?)pt(["'])`)
var strictZoomPercentAttribute = regexp.MustCompile(`(?i)(\bpercent\s*=\s*["'])([0-9]+)%(["'])`)
var strictThemePercentageAttribute = regexp.MustCompile(`(?i)(\b(?:[[:alnum:]_]+:)?[[:alnum:]_]+\s*=\s*["'])([0-9]+(?:\.[0-9]+)?)%(["'])`)

// unmarshalStrictWordPart accepts ECMA-376 Strict's lexical point values while
// retaining the integer twip and half-point representation used by this model.
func unmarshalStrictWordPart(data []byte, value any) error {
	data = strictZoomPercentAttribute.ReplaceAll(data, []byte("${1}${2}${3}"))
	data = normalizeStrictWordPointValues(data, strictHalfPointAttribute, 2)
	data = normalizeStrictWordPointValues(data, strictTwipAttribute, 20)
	return xml.Unmarshal(data, value)
}

func unmarshalStrictThemePart(data []byte, value any) error {
	data = normalizeStrictWordPointValues(data, strictThemePercentageAttribute, 1000)
	return unmarshalStrictWordPart(data, value)
}

func normalizeStrictWordPointValues(data []byte, expression *regexp.Regexp, multiplier float64) []byte {
	return expression.ReplaceAllFunc(data, func(match []byte) []byte {
		parts := expression.FindSubmatch(match)
		points, err := strconv.ParseFloat(string(parts[2]), 64)
		if err != nil {
			return match
		}
		return []byte(string(parts[1]) + strconv.Itoa(int(math.Round(points*multiplier))) + string(parts[3]))
	})
}
