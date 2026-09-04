package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// FormatCategory classifies an Excel number format code.
type FormatCategory int

const (
	CategoryGeneral FormatCategory = iota
	CategoryNumber
	CategoryCurrency
	CategoryAccounting
	CategoryDate
	CategoryTime
	CategoryDateTime
	CategoryPercentage
	CategoryFraction
	CategoryScientific
	CategoryText
	CategorySpecial
)

// FormatValue formats a numeric cell value according to an Excel number format code.
// It is category-aware: it detects whether the code represents a date, time,
// percentage, fraction, scientific, currency, or plain number and formats
// accordingly.
func FormatValue(v float64, code string) string {
	code = strings.TrimSpace(code)
	if code == "" || code == "General" {
		return formatGeneral(v)
	}

	sections := splitSections(code)
	sec, wrapParens := chooseSection(v, sections)

	// Detect category from the chosen section.
	cat := detectCategory(sec)

	switch cat {
	case CategoryDate, CategoryTime, CategoryDateTime:
		return formatDateTime(v, sec, cat)
	case CategoryPercentage:
		return formatPercentage(v, sec)
	case CategoryScientific:
		return formatScientific(v, sec)
	case CategoryFraction:
		return formatFraction(v, sec)
	case CategoryText:
		return formatGeneral(v)
	default:
		return formatNumberSection(v, sec, wrapParens)
	}
}

// ---------------------------------------------------------------------------
// Section splitting and selection
// ---------------------------------------------------------------------------

func splitSections(code string) []string {
	var sections []string
	var sb strings.Builder
	inQuote := false
	for i := 0; i < len(code); i++ {
		ch := code[i]
		if ch == '"' {
			inQuote = !inQuote
			sb.WriteByte(ch)
			continue
		}
		if inQuote {
			sb.WriteByte(ch)
			continue
		}
		if ch == ';' {
			sections = append(sections, sb.String())
			sb.Reset()
			continue
		}
		sb.WriteByte(ch)
	}
	sections = append(sections, sb.String())
	return sections
}

func chooseSection(v float64, sections []string) (string, bool) {
	wrapParens := false
	if len(sections) == 1 {
		return sections[0], false
	}

	// Up to 4 sections: positive;negative;zero;text
	var idx int
	switch {
	case v < 0:
		idx = 1
	case v == 0:
		idx = 2
	default:
		idx = 0
	}

	if idx >= len(sections) {
		// Fall back to positive format for missing sections.
		idx = 0
	}
	sec := strings.TrimSpace(sections[idx])

	// Detect parenthesized negative accounting format.
	if v < 0 && strings.HasPrefix(sec, "(") && strings.HasSuffix(sec, ")") {
		sec = strings.TrimPrefix(strings.TrimSuffix(sec, ")"), "(")
		wrapParens = true
	}

	return sec, wrapParens
}

// ---------------------------------------------------------------------------
// Category detection
// ---------------------------------------------------------------------------

func detectCategory(sec string) FormatCategory {
	lower := strings.ToLower(sec)

	// Text format
	if lower == "@" {
		return CategoryText
	}

	// Check for elapsed-time brackets on the original string.
	if strings.Contains(lower, "[h]") || strings.Contains(lower, "[m]") || strings.Contains(lower, "[s]") {
		return CategoryTime
	}

	// Strip color/condition/locale brackets for detection.
	clean := stripBrackets(sec)
	cleanLower := strings.ToLower(clean)

	// Percentage
	if strings.Contains(cleanLower, "%") {
		return CategoryPercentage
	}

	// Scientific
	if strings.Contains(cleanLower, "e+") || strings.Contains(cleanLower, "e-") {
		return CategoryScientific
	}

	// Date / Time
	hasDate := strings.ContainsAny(cleanLower, "dy") || strings.Contains(cleanLower, "mmm") || strings.Contains(cleanLower, "mmmmm")
	hasTime := strings.ContainsAny(cleanLower, "hs") || strings.Contains(cleanLower, "am/pm") || strings.Contains(cleanLower, "a/p")

	if hasDate || hasTime {
		if hasDate && hasTime {
			return CategoryDateTime
		}
		if hasTime {
			return CategoryTime
		}
		return CategoryDate
	}

	// Fraction
	if strings.Contains(cleanLower, "/") && !strings.Contains(cleanLower, ":") {
		return CategoryFraction
	}

	// Currency / Accounting
	if strings.ContainsAny(clean, "$€£¥") {
		if strings.Contains(clean, "_") || strings.Contains(clean, "*") {
			return CategoryAccounting
		}
		return CategoryCurrency
	}

	return CategoryNumber
}

// stripBrackets removes color, condition, and locale brackets from a format string.
func stripBrackets(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '[' {
			if j := strings.Index(s[i:], "]"); j >= 0 {
				i += j
				continue
			}
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

// ---------------------------------------------------------------------------
// General
// ---------------------------------------------------------------------------

func formatGeneral(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// ---------------------------------------------------------------------------
// Number, Currency, Accounting
// ---------------------------------------------------------------------------

func formatNumberSection(v float64, sec string, wrapParens bool) string {
	// Preserve currency prefix/suffix.
	clean := stripBrackets(sec)

	prefix, suffix, code := extractPrefixSuffix(clean)
	decimals := countDecimalPlaces(code)

	// If there's no numeric format code left, return just the text decorations.
	if code == "" {
		return prefix + suffix
	}

	// Comma grouping
	useComma := strings.Contains(code, ",") || strings.Contains(code, "#")

	neg := v < 0
	if neg {
		v = -v
	}

	var numStr string
	if useComma && decimals > 0 {
		numStr = formatComma(v, decimals)
	} else if useComma {
		numStr = formatComma(v, 0)
	} else if decimals > 0 {
		numStr = fmt.Sprintf("%.*f", decimals, v)
	} else if strings.Contains(code, "0") || strings.Contains(code, "#") {
		numStr = strconv.FormatInt(int64(math.Round(v)), 10)
	} else {
		numStr = formatGeneral(v)
	}

	result := prefix + numStr + suffix
	if neg && !wrapParens {
		result = "-" + result
	}
	if wrapParens {
		result = "(" + result + ")"
	}
	return result
}

func extractPrefixSuffix(code string) (prefix, suffix, remaining string) {
	rs := []rune(code)
	// Extract prefix: currency symbols, quoted strings, escaped chars.
	i := 0
	for i < len(rs) {
		ch := rs[i]
		switch ch {
		case '$', '€', '£', '¥':
			prefix += string(ch)
			i++
		case '"':
			rest := string(rs[i+1:])
			if j := strings.Index(rest, "\""); j >= 0 {
				prefix += rest[:j]
				i += j + 2
			} else {
				i++
			}
		case '\\':
			if i+1 < len(rs) {
				prefix += string(rs[i+1])
				i += 2
			} else {
				i++
			}
		case '_':
			if i+1 < len(rs) {
				prefix += " " // underscore is a space placeholder
				i += 2
			} else {
				i++
			}
		case '*':
			if i+1 < len(rs) {
				i += 2 // skip repeat char
			} else {
				i++
			}
		case ' ', '(', ')', '+', '-':
			prefix += string(ch)
			i++
		default:
			goto donePrefix
		}
	}
donePrefix:
	remaining = string(rs[i:])

	// Extract suffix from the end.
	rs2 := []rune(remaining)
	j := len(rs2)
	for j > 0 {
		ch := rs2[j-1]
		switch ch {
		case '$', '€', '£', '¥':
			suffix = string(ch) + suffix
			j--
		case '"':
			rest := string(rs2[:j-1])
			if k := strings.LastIndex(rest, "\""); k >= 0 {
				suffix = rest[k+1:] + suffix
				j = k
			} else {
				j--
			}
		case '\\':
			if j > 1 {
				suffix = string(rs2[j-2]) + suffix
				j -= 2
			} else {
				j--
			}
		case '_':
			suffix = " " + suffix
			j--
		case '*':
			if j > 1 {
				j -= 2
			} else {
				j--
			}
		case ' ', '(', ')', '+', '-':
			suffix = string(ch) + suffix
			j--
		default:
			goto doneSuffix
		}
	}
doneSuffix:
	remaining = string(rs2[:j])
	return prefix, suffix, remaining
}

func countDecimalPlaces(format string) int {
	dot := strings.Index(format, ".")
	if dot < 0 {
		return 0
	}
	dec := 0
	for _, ch := range format[dot+1:] {
		if ch == '0' || ch == '#' {
			dec++
		} else if ch != ',' {
			break
		}
	}
	return dec
}

func formatComma(v float64, decimals int) string {
	neg := v < 0
	if neg {
		v = -v
	}
	var intPart, fracPart string
	if decimals > 0 {
		s := fmt.Sprintf("%.*f", decimals, v)
		parts := strings.SplitN(s, ".", 2)
		intPart = parts[0]
		fracPart = "." + parts[1]
	} else {
		intPart = strconv.FormatInt(int64(math.Round(v)), 10)
	}
	var out []byte
	for i, ch := range intPart {
		pos := len(intPart) - i
		if i > 0 && pos%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(ch))
	}
	result := string(out) + fracPart
	if neg {
		return "-" + result
	}
	return result
}

// ---------------------------------------------------------------------------
// Percentage
// ---------------------------------------------------------------------------

func formatPercentage(v float64, sec string) string {
	clean := stripBrackets(sec)
	prefix, suffix, code := extractPrefixSuffix(clean)
	decimals := countDecimalPlaces(code)
	v = v * 100
	var numStr string
	if decimals > 0 {
		numStr = fmt.Sprintf("%.*f", decimals, v)
	} else if strings.Contains(code, "0") || strings.Contains(code, "#") {
		numStr = strconv.FormatInt(int64(math.Round(v)), 10)
	} else {
		numStr = formatGeneral(v)
	}
	return prefix + numStr + suffix + "%"
}

// ---------------------------------------------------------------------------
// Scientific
// ---------------------------------------------------------------------------

func formatScientific(v float64, sec string) string {
	clean := stripBrackets(sec)
	_, _, code := extractPrefixSuffix(clean)
	upper := strings.ToUpper(code)
	parts := strings.Split(upper, "E")
	if len(parts) != 2 {
		return strconv.FormatFloat(v, 'e', -1, 64)
	}
	precision := countDecimalPlaces(parts[0])
	return strconv.FormatFloat(v, 'e', precision, 64)
}

// ---------------------------------------------------------------------------
// Fraction
// ---------------------------------------------------------------------------

func formatFraction(v float64, sec string) string {
	clean := stripBrackets(sec)
	_, _, code := extractPrefixSuffix(clean)

	// Determine denominator precision from format like "# ?/?" or "# ??/??"
	maxDenom := 100
	if strings.Contains(code, "??/??") {
		maxDenom = 100
	} else if strings.Contains(code, "?/?") {
		maxDenom = 10
	} else if strings.Contains(code, "#/##") {
		maxDenom = 100
	} else if strings.Contains(code, "#/#") {
		maxDenom = 10
	}

	neg := v < 0
	if neg {
		v = -v
	}

	intPart := int64(math.Floor(v))
	frac := v - float64(intPart)

	num, denom := approximateFraction(frac, maxDenom)

	var result string
	if intPart > 0 {
		result = fmt.Sprintf("%d %d/%d", intPart, num, denom)
	} else if num == 0 {
		result = "0"
	} else {
		result = fmt.Sprintf("%d/%d", num, denom)
	}
	if neg {
		result = "-" + result
	}
	return result
}

func approximateFraction(x float64, maxDenom int) (num, denom int) {
	if x <= 0 {
		return 0, 1
	}
	bestNum, bestDenom := 1, 1
	bestErr := math.Abs(x - 1.0)
	for d := 1; d <= maxDenom; d++ {
		n := int(math.Round(x * float64(d)))
		if n < 0 {
			n = 0
		}
		err := math.Abs(x - float64(n)/float64(d))
		if err < bestErr {
			bestErr = err
			bestNum = n
			bestDenom = d
		}
	}
	return bestNum, bestDenom
}

// ---------------------------------------------------------------------------
// Date / Time
// ---------------------------------------------------------------------------

func formatDateTime(v float64, sec string, cat FormatCategory) string {
	t := excelSerialToTime(v)
	if t.IsZero() {
		return ""
	}

	return applyDateTimeFormat(v, t, sec)
}

// excelSerialToTime converts an Excel serial number (1900 date system) to time.Time.
func excelSerialToTime(serial float64) time.Time {
	if serial < 1 {
		return time.Time{}
	}
	n := int(serial)
	frac := serial - float64(n)
	if n == 60 {
		n = 59 // Lotus 1-2-3 phantom day
	} else if n > 60 {
		n--
	}
	epoch := time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	d := epoch.AddDate(0, 0, n)
	if frac > 0 {
		totalSecs := int(math.Round(frac * 86400))
		d = d.Add(time.Duration(totalSecs) * time.Second)
	}
	return d
}

func applyDateTimeFormat(serial float64, t time.Time, code string) string {
	// Determine which 'm' tokens are minutes vs months.
	// In Excel, 'm'/'mm' means minutes when it appears after h/hh in the format.
	isMinute := make([]bool, len(code))
	// Scan left-to-right: once we see h/hh, subsequent m/mm are minutes.
	sawHour := false
	for i := 0; i < len(code); i++ {
		if code[i] == 'h' || code[i] == 'H' {
			// Make sure it's not part of a longer token (only h and hh are hours)
			if i+1 < len(code) && (code[i+1] == 'h' || code[i+1] == 'H') {
				i++ // skip second h
			}
			sawHour = true
			continue
		}
		if code[i] != 'm' && code[i] != 'M' {
			continue
		}
		// mmmmm/mmmm/mmm are always month names.
		if i+5 <= len(code) && strings.EqualFold(code[i:i+5], "mmmmm") {
			sawHour = false
			continue
		}
		if i+4 <= len(code) && strings.EqualFold(code[i:i+4], "mmmm") {
			sawHour = false
			continue
		}
		if i+3 <= len(code) && strings.EqualFold(code[i:i+3], "mmm") {
			sawHour = false
			continue
		}
		if sawHour {
			isMinute[i] = true
			if i+1 < len(code) && (code[i+1] == 'm' || code[i+1] == 'M') {
				isMinute[i+1] = true
			}
		}
	}

	var out strings.Builder
	i := 0
	for i < len(code) {
		ch := code[i]

		// Quoted literal
		if ch == '"' {
			if j := strings.Index(code[i+1:], "\""); j >= 0 {
				out.WriteString(code[i+1 : i+1+j])
				i += j + 2
				continue
			}
		}

		// Escaped character
		if ch == '\\' && i+1 < len(code) {
			out.WriteByte(code[i+1])
			i += 2
			continue
		}

		// Underscore placeholder (space)
		if ch == '_' && i+1 < len(code) {
			out.WriteByte(' ')
			i += 2
			continue
		}

		// Bracketed codes — handle elapsed time [h], [m], [s].
		if ch == '[' {
			if j := strings.Index(code[i:], "]"); j >= 0 {
				inner := strings.ToLower(code[i+1 : i+j])
				switch inner {
				case "h":
					out.WriteString(fmt.Sprintf("%d", int(serial*24)))
				case "m":
					out.WriteString(fmt.Sprintf("%d", int(serial*1440)))
				case "s":
					out.WriteString(fmt.Sprintf("%d", int(serial*86400)))
				default:
					// Color, condition, locale — strip.
				}
				i += j + 1
				continue
			}
		}

		// Asterisk repeat — skip.
		if ch == '*' && i+1 < len(code) {
			i += 2
			continue
		}

		lower := strings.ToLower(code[i:])

		// AM/PM tokens
		if strings.HasPrefix(lower, "am/pm") {
			if t.Hour() >= 12 {
				out.WriteString("PM")
			} else {
				out.WriteString("AM")
			}
			i += 5
			continue
		}
		if strings.HasPrefix(lower, "a/p") {
			if t.Hour() >= 12 {
				out.WriteString("P")
			} else {
				out.WriteString("A")
			}
			i += 3
			continue
		}

		// Fractional seconds .0, .00, .000
		if ch == '.' && i+1 < len(code) {
			zeroCount := 0
			for k := i + 1; k < len(code) && code[k] == '0'; k++ {
				zeroCount++
			}
			if zeroCount > 0 {
				// Format fractional seconds
				ms := t.Nanosecond() / 1e6
				factor := 1
				for k := 0; k < zeroCount; k++ {
					factor *= 10
				}
				frac := (ms * factor) / 1000
				out.WriteString(fmt.Sprintf(".%0*d", zeroCount, frac))
				i += 1 + zeroCount
				continue
			}
		}

		// Date/time tokens.
		switch {
		case strings.HasPrefix(lower, "yyyy"):
			out.WriteString(fmt.Sprintf("%04d", t.Year()))
			i += 4
		case strings.HasPrefix(lower, "yy"):
			out.WriteString(fmt.Sprintf("%02d", t.Year()%100))
			i += 2
		case strings.HasPrefix(lower, "mmmmm"):
			out.WriteString(t.Month().String()[:1])
			i += 5
		case strings.HasPrefix(lower, "mmmm"):
			out.WriteString(t.Month().String())
			i += 4
		case strings.HasPrefix(lower, "mmm"):
			out.WriteString(t.Month().String()[:3])
			i += 3
		case strings.HasPrefix(lower, "mm"):
			if i < len(isMinute) && isMinute[i] {
				out.WriteString(fmt.Sprintf("%02d", t.Minute()))
			} else {
				out.WriteString(fmt.Sprintf("%02d", int(t.Month())))
			}
			i += 2
		case strings.HasPrefix(lower, "m"):
			if i < len(isMinute) && isMinute[i] {
				out.WriteString(fmt.Sprintf("%d", t.Minute()))
			} else {
				out.WriteString(fmt.Sprintf("%d", int(t.Month())))
			}
			i++
		case strings.HasPrefix(lower, "dddd"):
			out.WriteString(t.Weekday().String())
			i += 4
		case strings.HasPrefix(lower, "ddd"):
			out.WriteString(t.Weekday().String()[:3])
			i += 3
		case strings.HasPrefix(lower, "dd"):
			out.WriteString(fmt.Sprintf("%02d", t.Day()))
			i += 2
		case strings.HasPrefix(lower, "d"):
			out.WriteString(fmt.Sprintf("%d", t.Day()))
			i++
		case strings.HasPrefix(lower, "hh"):
			out.WriteString(fmt.Sprintf("%02d", t.Hour()))
			i += 2
		case strings.HasPrefix(lower, "h"):
			out.WriteString(fmt.Sprintf("%d", t.Hour()))
			i++
		case strings.HasPrefix(lower, "ss"):
			out.WriteString(fmt.Sprintf("%02d", t.Second()))
			i += 2
		case strings.HasPrefix(lower, "s"):
			out.WriteString(fmt.Sprintf("%d", t.Second()))
			i++
		default:
			out.WriteByte(ch)
			i++
		}
	}
	return out.String()
}
