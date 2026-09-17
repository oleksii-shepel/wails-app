package layout

import "testing"

func TestLucidaSansUnicodeUsesDedicatedOfficeAdvanceProfile(t *testing.T) {
	lucida := ResolveFontAdvanceMetrics("Lucida Sans Unicode")
	if lucida.Profile != "lucida" {
		t.Fatalf("profile = %q, want lucida", lucida.Profile)
	}
	text := "Thankfully, the 5th postulate is equivalent to another"
	lw := measurePlainTextWidthPt(text, 28, "Lucida Sans Unicode", false, false)
	aw := measurePlainTextWidthPt(text, 28, "Arial", false, false)
	if !(lw < aw) {
		t.Fatalf("Lucida width = %.2f, Arial = %.2f; expected calibrated Lucida advance to be narrower", lw, aw)
	}
}
