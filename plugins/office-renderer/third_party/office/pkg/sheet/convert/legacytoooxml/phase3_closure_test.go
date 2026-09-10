package legacytoooxml

import (
	"testing"

	sheetir "office/pkg/sheet/ir"
)

func TestPhase3ClosureSheetFormatPreservesCanonicalViewAndOutlineMetadata(t *testing.T) {
	sheet := &sheetir.Sheet{
		DefaultRowHeightPt:         18.5,
		DefaultColWidthChars:       11.25,
		DefaultRowHeightUnsynced:   true,
		DefaultRowHeightZeroHeight: true,
		RowOutlineLevelMax:         2,
		ColOutlineLevelMax:         1,
		OutlineRows:                map[int]int{3: 4},
		OutlineCols:                map[int]int{2: 3},
	}

	got := buildSheetFormatPr(sheet)
	if got.DefaultRowHeight != 18.5 || got.DefaultColWidth != 11.25 {
		t.Fatalf("default dimensions = (%v, %v), want (18.5, 11.25)", got.DefaultRowHeight, got.DefaultColWidth)
	}
	if !got.CustomHeight || !got.ZeroHeight {
		t.Fatalf("default row flags = custom:%v zero:%v, want true/true", got.CustomHeight, got.ZeroHeight)
	}
	if got.OutlineLevelRow != 4 || got.OutlineLevelCol != 3 {
		t.Fatalf("outline maxima = row:%d col:%d, want 4/3", got.OutlineLevelRow, got.OutlineLevelCol)
	}
}

func TestPhase3ClosureSheetFormatKeepsAuthoredOutlineMaxima(t *testing.T) {
	got := buildSheetFormatPr(&sheetir.Sheet{
		RowOutlineLevelMax: 5,
		ColOutlineLevelMax: 6,
		OutlineRows:        map[int]int{1: 2},
		OutlineCols:        map[int]int{1: 3},
	})
	if got.OutlineLevelRow != 5 || got.OutlineLevelCol != 6 {
		t.Fatalf("authored outline maxima = row:%d col:%d, want 5/6", got.OutlineLevelRow, got.OutlineLevelCol)
	}
}
