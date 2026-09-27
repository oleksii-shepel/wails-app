package legacy

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	layoutpkg "office/pkg/word/layout"
)

func TestLegacyAdvancedTextboxesOfficeArtLayoutContract(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "doc", "legacy-advanced-textboxes.doc")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skip("legacy advanced-textbox OfficeArt fixture is not available")
		}
		t.Fatalf("stat fixture: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	pages := layoutpkg.Paginate(doc)
	if len(pages) == 0 {
		t.Fatal("expected at least one page")
	}

	type placed struct {
		id    uint32
		order int
		x, y  float64
		w, h  float64
	}
	var got []placed
	for _, page := range pages {
		for _, fb := range page.Floats {
			if fb.Shape == nil || !fb.Shape.LegacyFlowAnchor {
				continue
			}
			got = append(got, placed{
				id: fb.Shape.LegacyShapeID, order: fb.Shape.LegacyFlowOrder,
				x: fb.X, y: fb.Y, w: fb.Width, h: fb.Height,
			})
		}
	}
	if len(got) != 5 {
		t.Fatalf("legacy SHAPE-field floats=%d, want 5: %+v", len(got), got)
	}
	sort.Slice(got, func(i, j int) bool { return got[i].order < got[j].order })

	wantOrders := []int{166, 178, 190, 202, 214}
	for i, want := range wantOrders {
		if got[i].order != want {
			t.Fatalf("flow order[%d]=%d, want FSPA CP %d", i, got[i].order, want)
		}
	}

	// These are page-absolute coordinates measured from the Word 2021 PDF.
	// The orange box has zero local FSPA coordinates; this locks the fact that
	// its final position is derived from the preceding anchored OfficeArt row.
	wantPositions := []struct{ x, y float64 }{
		{56.70, 102.70},  // 1028
		{263.75, 102.70}, // 1029
		{66.05, 318.90},  // 1030, unrotated FSPA rectangle
		{238.90, 281.25}, // 1026
		{56.70, 519.35},  // 1027
	}
	for i, want := range wantPositions {
		if math.Abs(got[i].x-want.x) > 1 || math.Abs(got[i].y-want.y) > 1 {
			t.Fatalf("shape %d absolute placement=(%.2f, %.2f), want PDF (%.2f, %.2f)", got[i].id, got[i].x, got[i].y, want.x, want.y)
		}
	}

	// Legacy SHAPE fields use anchor-line flow semantics. Preserve FSPA CP order
	// and deterministic packing until layout has enough paragraph-anchor metadata
	// to resolve each local FSPA rectangle against its actual anchor line.
	if !(got[0].x < got[1].x) {
		t.Fatalf("first row collapsed/ordered incorrectly: %+v", got[:2])
	}
	// The rotated third SHAPE preserves its local FSPA offsets inside its row;
	// a positive local Y is not an implicit row break.
	if math.Abs(got[2].x-(got[0].x+9.35)) > 0.5 || math.Abs((got[2].y-got[3].y)-37.65) > 0.5 {
		t.Fatalf("third shape local offsets lost: third=%+v row=%+v", got[2], got[3])
	}
	if !(got[2].x < got[3].x) || got[3].y >= got[2].y {
		t.Fatalf("second-row local/continuation placement incorrect: %+v", got[2:4])
	}
	if math.Abs(got[4].x-got[0].x) > 0.5 || got[4].y <= got[3].y+got[3].h-0.01 {
		t.Fatalf("third row placement incorrect: row3=%+v row2=%+v", got[4], got[3])
	}

	if len(pages[0].Blocks) == 0 {
		t.Fatal("expected visible paragraph block")
	}
	firstBlock := pages[0].Blocks[0]
	minY := pages[0].Geometry.MarginTop + firstBlock.Y + firstBlock.Height
	if got[0].y < minY-0.5 {
		t.Fatalf("OfficeArt row starts at %.1fpt before paragraph bottom %.1fpt", got[0].y, minY)
	}
}
