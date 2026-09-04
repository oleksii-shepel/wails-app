package render

import (
	"testing"
	"time"
)

func TestGregorianToHebrew(t *testing.T) {
	tests := []struct {
		greg   time.Time
		want   hebrewDate
		desc   string
	}{
		// hebcal: HDate.hebrew2abs(5769, CHESHVAN, 15) = 733359 → 13 Nov 2008
		{time.Date(2008, 11, 13, 0, 0, 0, 0, time.UTC), hebrewDate{Year: 5769, Month: 8, Day: 15}, "Cheshvan 15 5769"},
		// 2016-01-01 → 5776 Tevet 20 (from R calcal package)
		{time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC), hebrewDate{Year: 5776, Month: 10, Day: 20}, "Tevet 20 5776"},
		// 2025-01-01 → 5785 Tevet 1
		{time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), hebrewDate{Year: 5785, Month: 10, Day: 1}, "Tevet 1 5785"},
		// 2025-01-31 → 5785 Shevat 2
		{time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), hebrewDate{Year: 5785, Month: 11, Day: 2}, "Shevat 2 5785"},
		// Rosh Hashanah 5785 = 2024-10-03 (Tishrei 1)
		{time.Date(2024, 10, 3, 0, 0, 0, 0, time.UTC), hebrewDate{Year: 5785, Month: 7, Day: 1}, "Tishrei 1 5785"},
		// Passover 5785 = 2025-04-13 (Nisan 15)
		{time.Date(2025, 4, 13, 0, 0, 0, 0, time.UTC), hebrewDate{Year: 5785, Month: 1, Day: 15}, "Nisan 15 5785"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := gregorianToHebrew(tt.greg)
			if got.Year != tt.want.Year || got.Month != tt.want.Month || got.Day != tt.want.Day {
				t.Errorf("gregorianToHebrew(%v) = %+v, want %+v", tt.greg.Format("2006-01-02"), got, tt.want)
			}
		})
	}
}

func TestHebrewCivilMonth(t *testing.T) {
	// Common year 5769: Tishrei=7 (religious) → civil 1
	hd := hebrewDate{Year: 5769, Month: 7, Day: 1}
	if got := hebrewCivilMonth(hd); got != 1 {
		t.Errorf("Tishrei civil month = %d, want 1", got)
	}
	// Cheshvan=8 (religious) → civil 2
	hd.Month = 8
	if got := hebrewCivilMonth(hd); got != 2 {
		t.Errorf("Cheshvan civil month = %d, want 2", got)
	}
	// Nisan=1 (religious) → civil 7
	hd.Month = 1
	if got := hebrewCivilMonth(hd); got != 7 {
		t.Errorf("Nisan civil month = %d, want 7", got)
	}
	// Elul=6 (religious) → civil 12
	hd.Month = 6
	if got := hebrewCivilMonth(hd); got != 12 {
		t.Errorf("Elul civil month = %d, want 12", got)
	}

	// Leap year 5784: Adar I=12 (religious) → civil 6
	hd = hebrewDate{Year: 5784, Month: 12, Day: 1}
	if got := hebrewCivilMonth(hd); got != 6 {
		t.Errorf("Adar I civil month = %d, want 6", got)
	}
	// Adar II=13 (religious) → civil 7
	hd.Month = 13
	if got := hebrewCivilMonth(hd); got != 7 {
		t.Errorf("Adar II civil month = %d, want 7", got)
	}
	// Nisan=1 (religious) → civil 8
	hd.Month = 1
	if got := hebrewCivilMonth(hd); got != 8 {
		t.Errorf("Nisan in leap year civil month = %d, want 8", got)
	}
	// Elul=6 (religious) → civil 13
	hd.Month = 6
	if got := hebrewCivilMonth(hd); got != 13 {
		t.Errorf("Elul in leap year civil month = %d, want 13", got)
	}
}

func TestFormatHebrewDate(t *testing.T) {
	// 2008-11-13 = 5769 Cheshvan 15
	ref := time.Date(2008, 11, 13, 14, 30, 45, 0, time.UTC)

	tests := []struct {
		format string
		want   string
	}{
		{"yyyy", "5769"},
		{"yy", "69"},
		{"MMMM", "Cheshvan"},
		{"MMM", "Che"},
		{"MM", "02"},
		{"M", "2"},
		{"dd", "15"},
		{"d", "15"},
		{"dddd", "Thursday"},
		{"ddd", "Thu"},
		{"HH", "14"},
		{"H", "14"},
		{"hh", "02"},
		{"h", "2"},
		{"mm", "30"},
		{"m", "30"},
		{"ss", "45"},
		{"s", "45"},
		{"tt", "PM"},
		{"t", "P"},
		{"AM/PM", "PM"},
		{"am/pm", "pm"},
		{"A/P", "P"},
		{"a/p", "p"},
		{"d MMMM yyyy", "15 Cheshvan 5769"},
		{"yyyy-MM-dd", "5769-02-15"},
		{"h:mm tt", "2:30 PM"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got := formatHebrewDate(ref, tt.format)
			if got != tt.want {
				t.Errorf("formatHebrewDate(%q) = %q, want %q", tt.format, got, tt.want)
			}
		})
	}
}

func TestFormatHebrewDateLeapYear(t *testing.T) {
	// Leap year 5784: verify Adar I and Adar II names.
	// 2024-03-12 should be in Adar II 5784 (leap year).
	// Let's find a known date: Purim in leap year is Adar II 14.
	// 5784 Purim = 2024-03-24 (Gregorian)
	ref := time.Date(2024, 3, 24, 0, 0, 0, 0, time.UTC)
	got := formatHebrewDate(ref, "d MMMM yyyy")
	want := "14 Adar II 5784"
	if got != want {
		t.Errorf("leap year format = %q, want %q", got, want)
	}
}

func TestFormatFieldDateTimeHebrewSwitch(t *testing.T) {
	ref := time.Date(2008, 11, 13, 14, 30, 0, 0, time.UTC)

	// With \h and explicit format
	instr := `DATE \h \@ "d MMMM yyyy"`
	got := formatFieldDateTime(instr, "DATE", ref)
	want := "15 Cheshvan 5769"
	if got != want {
		t.Errorf("DATE with \\h = %q, want %q", got, want)
	}

	// With \h and no format switch
	instr = `DATE \h`
	got = formatFieldDateTime(instr, "DATE", ref)
	want = "15 Cheshvan 5769"
	if got != want {
		t.Errorf("DATE with \\h default = %q, want %q", got, want)
	}
}