package ooxml

import (
	"testing"

	sheetir "office/pkg/sheet/ir"
)

func TestParseStreamingMergeRef(t *testing.T) {
	got, ok := parseStreamingMergeRef("$B$2:D5")
	if !ok { t.Fatal("merge ref rejected") }
	want := sheetir.MergedCell{TopRow:2, LeftCol:2, BottomRow:5, RightCol:4}
	if *got != want { t.Fatalf("merge = %#v, want %#v", *got, want) }
}

func TestParseStreamingMergeRefNormalizesReverseBounds(t *testing.T) {
	got, ok := parseStreamingMergeRef("D5:B2")
	if !ok { t.Fatal("merge ref rejected") }
	if got.TopRow != 2 || got.LeftCol != 2 || got.BottomRow != 5 || got.RightCol != 4 {
		t.Fatalf("merge = %#v", got)
	}
}
