package render

import (
	"testing"
)

func TestFormatValueGeneral(t *testing.T) {
	tests := []struct {
		v    float64
		code string
		want string
	}{
		{42, "General", "42"},
		{42, "", "42"},
		{3.14159, "General", "3.14159"},
		{1e20, "General", "1e+20"},
		{-5, "General", "-5"},
	}
	for _, tt := range tests {
		got := FormatValue(tt.v, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", tt.v, tt.code, got, tt.want)
		}
	}
}

func TestFormatValueNumber(t *testing.T) {
	tests := []struct {
		v    float64
		code string
		want string
	}{
		{1234.5, "0", "1235"},
		{1234.5, "0.00", "1234.50"},
		{1234.5, "#,##0", "1,235"},
		{1234.5, "#,##0.00", "1,234.50"},
		{-1234.5, "#,##0.00", "-1,234.50"},
		{0.5, "0.00", "0.50"},
		{0, "0", "0"},
	}
	for _, tt := range tests {
		got := FormatValue(tt.v, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", tt.v, tt.code, got, tt.want)
		}
	}
}

func TestFormatValueCurrency(t *testing.T) {
	tests := []struct {
		v    float64
		code string
		want string
	}{
		{1234.5, "$#,##0.00", "$1,234.50"},
		{-1234.5, "$#,##0.00", "-$1,234.50"},
		{1234.5, "€#,##0.00", "€1,234.50"},
		{1234.5, "£#,##0.00", "£1,234.50"},
	}
	for _, tt := range tests {
		got := FormatValue(tt.v, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", tt.v, tt.code, got, tt.want)
		}
	}
}

func TestFormatValueAccounting(t *testing.T) {
	tests := []struct {
		v    float64
		code string
		want string
	}{
		{1234.5, "_( $* #,##0.00_);_( $* (#,##0.00);_( $* \"-\"??_);_(@_)", "  $1,234.50 )"},
		{-1234.5, "_( $* #,##0.00_);_( $* (#,##0.00);_( $* \"-\"??_);_(@_)", "-  $(1,234.50)"},
	}
	for _, tt := range tests {
		got := FormatValue(tt.v, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", tt.v, tt.code, got, tt.want)
		}
	}
}

func TestFormatValuePercentage(t *testing.T) {
	tests := []struct {
		v    float64
		code string
		want string
	}{
		{0.05, "0%", "5%"},
		{0.1234, "0.00%", "12.34%"},
		{1.5, "0%", "150%"},
	}
	for _, tt := range tests {
		got := FormatValue(tt.v, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", tt.v, tt.code, got, tt.want)
		}
	}
}

func TestFormatValueScientific(t *testing.T) {
	tests := []struct {
		v    float64
		code string
		want string
	}{
		{12345, "0.00E+00", "1.23e+04"},
		{0.00123, "0.00E+00", "1.23e-03"},
	}
	for _, tt := range tests {
		got := FormatValue(tt.v, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", tt.v, tt.code, got, tt.want)
		}
	}
}

func TestFormatValueFraction(t *testing.T) {
	tests := []struct {
		v    float64
		code string
		want string
	}{
		{0.5, "# ?/?", "1/2"},
		{1.5, "# ?/?", "1 1/2"},
		{0.333, "# ?/?", "1/3"},
	}
	for _, tt := range tests {
		got := FormatValue(tt.v, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", tt.v, tt.code, got, tt.want)
		}
	}
}

func TestFormatValueDate(t *testing.T) {
	// Excel serial for 2024-03-15 = 45366
	serial := 45366.0
	tests := []struct {
		code string
		want string
	}{
		{"yyyy-mm-dd", "2024-03-15"},
		{"yy/mm/dd", "24/03/15"},
		{"dddd", "Friday"},
		{"ddd", "Fri"},
		{"mmmm", "March"},
		{"mmm", "Mar"},
		{"mmmmm", "M"},
		{"d/m/yyyy", "15/3/2024"},
		{"dd-mm-yy", "15-03-24"},
	}
	for _, tt := range tests {
		got := FormatValue(serial, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", serial, tt.code, got, tt.want)
		}
	}
}

func TestFormatValueTime(t *testing.T) {
	// Noon = 0.5 days
	serial := 45366.5
	tests := []struct {
		code string
		want string
	}{
		{"hh:mm", "12:00"},
		{"hh:mm:ss", "12:00:00"},
		{"h:mm AM/PM", "12:00 PM"},
		{"h:mm A/P", "12:00 P"},
	}
	for _, tt := range tests {
		got := FormatValue(serial, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", serial, tt.code, got, tt.want)
		}
	}
}

func TestFormatValueDateTime(t *testing.T) {
	serial := 45366.5
	tests := []struct {
		code string
		want string
	}{
		{"yyyy-mm-dd hh:mm", "2024-03-15 12:00"},
		{"dd/mm/yyyy hh:mm:ss", "15/03/2024 12:00:00"},
	}
	for _, tt := range tests {
		got := FormatValue(serial, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", serial, tt.code, got, tt.want)
		}
	}
}

func TestFormatValueElapsedTime(t *testing.T) {
	// 1.5 days = 36 hours
	serial := 1.5
	tests := []struct {
		code string
		want string
	}{
		{"[h]:mm", "36:00"},
	}
	for _, tt := range tests {
		got := FormatValue(serial, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", serial, tt.code, got, tt.want)
		}
	}
}

func TestFormatValueText(t *testing.T) {
	got := FormatValue(123, "@")
	if got != "123" {
		t.Errorf("FormatValue(123, @) = %q, want 123", got)
	}
}

func TestFormatValueMultiSection(t *testing.T) {
	tests := []struct {
		v    float64
		code string
		want string
	}{
		{10, "0.0;(0.0);\"zero\"", "10.0"},
		{-10, "0.0;(0.0);\"zero\"", "(10.0)"},
		{0, "0.0;(0.0);\"zero\"", "zero"},
	}
	for _, tt := range tests {
		got := FormatValue(tt.v, tt.code)
		if got != tt.want {
			t.Errorf("FormatValue(%v, %q) = %q, want %q", tt.v, tt.code, got, tt.want)
		}
	}
}

func TestFormatValueZeroDecimal(t *testing.T) {
	got := FormatValue(3.7, "0")
	if got != "4" {
		t.Errorf("FormatValue(3.7, 0) = %q, want 4", got)
	}
}

func TestFormatValueSpecial(t *testing.T) {
	// Empty serial should return empty for date formats
	got := FormatValue(0, "yyyy-mm-dd")
	if got != "" {
		t.Errorf("FormatValue(0, yyyy-mm-dd) = %q, want empty", got)
	}
}

func TestApproximateFraction(t *testing.T) {
	tests := []struct {
		x        float64
		maxDenom int
		num      int
		denom    int
	}{
		{0.5, 10, 1, 2},
		{0.333, 10, 1, 3},
		{0.75, 10, 3, 4},
		{0.0, 10, 0, 1},
	}
	for _, tt := range tests {
		num, denom := approximateFraction(tt.x, tt.maxDenom)
		if num != tt.num || denom != tt.denom {
			t.Errorf("approximateFraction(%v, %d) = %d/%d, want %d/%d", tt.x, tt.maxDenom, num, denom, tt.num, tt.denom)
		}
	}
}

func TestExcelSerialToTime(t *testing.T) {
	// 45366 = 2024-03-15
	tm := excelSerialToTime(45366)
	if tm.Year() != 2024 || tm.Month() != 3 || tm.Day() != 15 {
		t.Errorf("excelSerialToTime(45366) = %v, want 2024-03-15", tm.Format("2006-01-02"))
	}
	// 0 should return zero time
	tm = excelSerialToTime(0)
	if !tm.IsZero() {
		t.Errorf("excelSerialToTime(0) should be zero time")
	}
}
