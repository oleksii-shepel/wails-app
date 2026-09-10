package legacy

import "testing"

func TestFormatLegacyDttm(t *testing.T) {
	dttm := uint32((124 << 20) | (3 << 16) | (15 << 11) | (14 << 6) | 30)
	got := formatLegacyDttm(dttm)
	want := "2024-03-15T14:30:00"
	if got != want {
		t.Fatalf("formatLegacyDttm = %q, want %q", got, want)
	}
	if got := formatLegacyDttm(0); got != "" {
		t.Fatalf("formatLegacyDttm(0) = %q, want empty", got)
	}
	invalid := uint32((100 << 20) | (15 << 16) | (1 << 11))
	if got := formatLegacyDttm(invalid); got != "" {
		t.Fatalf("formatLegacyDttm(invalid) = %q, want empty", got)
	}
}
