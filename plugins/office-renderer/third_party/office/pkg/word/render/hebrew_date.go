package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// hebrewDate represents a date in the Hebrew calendar.
// Internally months use the Nisan=1 religious numbering (matching the
// Reingold/Dershowitz algorithm).  For display we convert to the civil
// (Tishrei=1) numbering used by Windows/.NET HebrewCalendar so that Word
// month numbers match user expectations.
type hebrewDate struct {
	Year  int
	Month int // Nisan=1 … Tishrei=7 (religious)
	Day   int
}

// ----------------------------------------------------------------------
// Gregorian ↔ Rata Die
// ----------------------------------------------------------------------

func gregorianToRD(year, month, day int) int {
	py := year - 1
	adj := 0
	if month > 2 {
		if isGregLeapYear(year) {
			adj = -1
		} else {
			adj = -2
		}
	}
	return 365*py + py/4 - py/100 + py/400 + (367*month-362)/12 + adj + day
}

func isGregLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

func timeToRD(t time.Time) int {
	y, m, d := t.Date()
	return gregorianToRD(y, int(m), d)
}

// ----------------------------------------------------------------------
// Hebrew calendar core (based on Reingold & Dershowitz, implemented in
// hebcal/hdate-js).
// ----------------------------------------------------------------------

const hebrewEpoch = -1373428

func hebrewElapsedDays(year int) int {
	prevYear := year - 1
	mElapsed := 235*(prevYear/19) + 12*(prevYear%19) + ((prevYear%19)*7+1)/19
	pElapsed := 204 + 793*(mElapsed%1080)
	hElapsed := 5 + 12*mElapsed + 793*(mElapsed/1080) + pElapsed/1080
	parts := (pElapsed % 1080) + 1080*(hElapsed%24)
	day := 1 + 29*mElapsed + hElapsed/24
	altDay := day
	if parts >= 19440 ||
		(day%7 == 2 && parts >= 9924 && !hebrewIsLeapYear(year)) ||
		(day%7 == 1 && parts >= 16789 && hebrewIsLeapYear(prevYear)) {
		altDay++
	}
	if altDay%7 == 0 || altDay%7 == 3 || altDay%7 == 5 {
		return altDay + 1
	}
	return altDay
}

func hebrewNewYear(year int) int {
	return hebrewEpoch + hebrewElapsedDays(year)
}

func hebrewIsLeapYear(year int) bool {
	return (1+year*7)%19 < 7
}

func hebrewMonthsInYear(year int) int {
	if hebrewIsLeapYear(year) {
		return 13
	}
	return 12
}

func hebrewDaysInYear(year int) int {
	return hebrewElapsedDays(year+1) - hebrewElapsedDays(year)
}

func hebrewLongCheshvan(year int) bool {
	return hebrewDaysInYear(year)%10 == 5
}

func hebrewShortKislev(year int) bool {
	return hebrewDaysInYear(year)%10 == 3
}

// staticHebrewDaysInMonth is indexed by Nisan=1 religious month number.
var staticHebrewDaysInMonth = []int{0, 30, 29, 30, 29, 30, 29, 30, 0, 0, 29, 30, 0, 29}

func hebrewDaysInMonth(month, year int) int {
	d := staticHebrewDaysInMonth[month]
	if d != 0 {
		return d
	}
	switch month {
	case 12: // Adar I (religious)
		if hebrewIsLeapYear(year) {
			return 30
		}
		return 29
	case 8: // Cheshvan
		if hebrewLongCheshvan(year) {
			return 30
		}
		return 29
	case 9: // Kislev
		if hebrewShortKislev(year) {
			return 29
		}
		return 30
	}
	return 29
}

func hebrew2abs(year, month, day int) int {
	tempabs := day
	if month < 7 { // Nisan through Elul
		endMonth := hebrewMonthsInYear(year)
		for m := 7; m <= endMonth; m++ {
			tempabs += hebrewDaysInMonth(m, year)
		}
		for m := 1; m < month; m++ {
			tempabs += hebrewDaysInMonth(m, year)
		}
	} else { // Tishrei through Adar II
		for m := 7; m < month; m++ {
			tempabs += hebrewDaysInMonth(m, year)
		}
	}
	return hebrewEpoch + hebrewElapsedDays(year) + tempabs - 1
}

func absToHebrew(abs int) hebrewDate {
	const avgHebrewYearDays = 365.24682220597794
	abs = int(math.Trunc(float64(abs)))
	year := int(math.Floor(float64(abs-hebrewEpoch) / avgHebrewYearDays))
	for hebrewNewYear(year) <= abs {
		year++
	}
	year--

	month := 7
	if abs >= hebrew2abs(year, 1, 1) {
		month = 1
	}
	for abs > hebrew2abs(year, month, hebrewDaysInMonth(month, year)) {
		month++
	}

	day := 1 + abs - hebrew2abs(year, month, 1)
	return hebrewDate{Year: year, Month: month, Day: day}
}

func gregorianToHebrew(t time.Time) hebrewDate {
	return absToHebrew(timeToRD(t))
}

// ----------------------------------------------------------------------
// Civil (Tishrei=1) month numbering
// ----------------------------------------------------------------------

// hebrewCivilMonth converts the religious month number (Nisan=1) to the
// civil month number (Tishrei=1) used by Windows/.NET HebrewCalendar.
func hebrewCivilMonth(hd hebrewDate) int {
	n := hebrewMonthsInYear(hd.Year)
	cm := (hd.Month + n - 6) % n
	if cm == 0 {
		cm = n
	}
	return cm
}

// hebrewCivilMonthName returns the transliterated month name using civil
// numbering.  In leap years Adar I and Adar II are distinguished.
func hebrewCivilMonthName(hd hebrewDate) string {
	civil := hebrewCivilMonth(hd)
	if hebrewIsLeapYear(hd.Year) {
		names := []string{
			"", "Tishrei", "Cheshvan", "Kislev", "Tevet", "Shevat",
			"Adar I", "Adar II", "Nisan", "Iyyar", "Sivan",
			"Tammuz", "Av", "Elul",
		}
		return names[civil]
	}
	names := []string{
		"", "Tishrei", "Cheshvan", "Kislev", "Tevet", "Shevat",
		"Adar", "Nisan", "Iyyar", "Sivan", "Tammuz", "Av", "Elul",
	}
	return names[civil]
}

// ----------------------------------------------------------------------
// Formatting
// ----------------------------------------------------------------------

// formatHebrewDate formats a Gregorian time using a Word date-time picture
// string but with Hebrew calendar values for year/month/day.
func formatHebrewDate(t time.Time, wordFmt string) string {
	hd := gregorianToHebrew(t)

	// Pre-compute all values.
	yearFull := strconv.Itoa(hd.Year)
	yearShort := fmt.Sprintf("%02d", hd.Year%100)

	civilMonth := hebrewCivilMonth(hd)
	monthName := hebrewCivilMonthName(hd)
	monthNum := strconv.Itoa(civilMonth)
	monthNum2 := fmt.Sprintf("%02d", civilMonth)
	monthAbbr := monthName
	if len(monthAbbr) > 3 {
		monthAbbr = monthAbbr[:3]
	}

	dayNum := strconv.Itoa(hd.Day)
	dayNum2 := fmt.Sprintf("%02d", hd.Day)

	wd := t.Weekday().String()
	wdShort := wd[:3]

	h24 := strconv.Itoa(t.Hour())
	h24_2 := fmt.Sprintf("%02d", t.Hour())
	h12v := t.Hour() % 12
	if h12v == 0 {
		h12v = 12
	}
	h12 := strconv.Itoa(h12v)
	h12_2 := fmt.Sprintf("%02d", h12v)
	min := strconv.Itoa(t.Minute())
	min2 := fmt.Sprintf("%02d", t.Minute())
	sec := strconv.Itoa(t.Second())
	sec2 := fmt.Sprintf("%02d", t.Second())
	ampm := "AM"
	if t.Hour() >= 12 {
		ampm = "PM"
	}
	ampmL := strings.ToLower(ampm)

	var b strings.Builder
	for i := 0; i < len(wordFmt); {
		// Try longest tokens first so that e.g. "MMMM" is consumed
		// before "MMM" or "MM" or "M".
		if i+5 <= len(wordFmt) {
			switch wordFmt[i : i+5] {
			case "AM/PM":
				b.WriteString(ampm)
				i += 5
				continue
			case "am/pm":
				b.WriteString(ampmL)
				i += 5
				continue
			}
		}
		if i+4 <= len(wordFmt) {
			switch wordFmt[i : i+4] {
			case "yyyy", "YYYY":
				b.WriteString(yearFull)
				i += 4
				continue
			case "MMMM":
				b.WriteString(monthName)
				i += 4
				continue
			case "dddd":
				b.WriteString(wd)
				i += 4
				continue
			}
		}
		if i+3 <= len(wordFmt) {
			switch wordFmt[i : i+3] {
			case "MMM":
				b.WriteString(monthAbbr)
				i += 3
				continue
			case "ddd":
				b.WriteString(wdShort)
				i += 3
				continue
			case "A/P":
				b.WriteString(string(ampm[0]))
				i += 3
				continue
			case "a/p":
				b.WriteString(string(ampmL[0]))
				i += 3
				continue
			}
		}
		if i+2 <= len(wordFmt) {
			switch wordFmt[i : i+2] {
			case "MM":
				b.WriteString(monthNum2)
				i += 2
				continue
			case "dd":
				b.WriteString(dayNum2)
				i += 2
				continue
			case "HH":
				b.WriteString(h24_2)
				i += 2
				continue
			case "hh":
				b.WriteString(h12_2)
				i += 2
				continue
			case "mm":
				b.WriteString(min2)
				i += 2
				continue
			case "ss":
				b.WriteString(sec2)
				i += 2
				continue
			case "tt":
				b.WriteString(ampm)
				i += 2
				continue
			case "yy", "YY":
				b.WriteString(yearShort)
				i += 2
				continue
			}
		}
		if i+1 <= len(wordFmt) {
			switch wordFmt[i] {
			case 'M':
				b.WriteString(monthNum)
				i++
				continue
			case 'd':
				b.WriteString(dayNum)
				i++
				continue
			case 'H':
				b.WriteString(h24)
				i++
				continue
			case 'h':
				b.WriteString(h12)
				i++
				continue
			case 'm':
				b.WriteString(min)
				i++
				continue
			case 's':
				b.WriteString(sec)
				i++
				continue
			case 't':
				b.WriteString(string(ampm[0]))
				i++
				continue
			case 'y', 'Y':
				b.WriteString(yearShort)
				i++
				continue
			}
		}
		// Literal character – copy as-is.
		b.WriteByte(wordFmt[i])
		i++
	}
	return b.String()
}