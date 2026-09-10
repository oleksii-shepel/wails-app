package layout

import (
	"testing"

	resolve "office/pkg/word/ir"
)

func TestCommitPageFloatsAssignsPhysicalPageOwnership(t *testing.T) {
	page := Page{Number: 7}
	commitPageFloats(&page, []FloatBox{{X: 10, Y: 20}, {X: 30, Y: 40}})
	if len(page.Floats) != 2 {
		t.Fatalf("committed floats = %d, want 2", len(page.Floats))
	}
	for i, fl := range page.Floats {
		if fl.Page != 7 {
			t.Fatalf("float %d page = %d, want 7", i, fl.Page)
		}
	}
}

func TestFreshPhysicalPageStartsWithoutFloatOwnership(t *testing.T) {
	oldPage := Page{Number: 1}
	commitPageFloats(&oldPage, []FloatBox{{X: 1, Y: 2}})
	newPage := Page{Number: 2}
	if got := wrapFloatsForFlowRegion(newPage); len(got) != 0 {
		t.Fatalf("new page inherited %d wrap floats", len(got))
	}
}

func TestTableSplitBoundaryCompatibilityDelegatesToCanonicalBoundary(t *testing.T) {
	table := &resolve.Table{Rows: []resolve.TableRow{{}, {}}}
	for _, end := range []int{0, 1, 2} {
		if got, want := tableSplitBoundarySafe(table, end), tableFragmentBoundarySafe(table, end); got != want {
			t.Fatalf("boundary %d compatibility = %v, canonical = %v", end, got, want)
		}
	}
}
