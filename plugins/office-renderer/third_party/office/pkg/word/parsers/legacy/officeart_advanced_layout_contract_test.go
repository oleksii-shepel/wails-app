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

	// Legacy SHAPE fields use anchor-line flow semantics. Preserve FSPA CP order
	// and deterministic packing until layout has enough paragraph-anchor metadata
	// to resolve each local FSPA rectangle against its actual anchor line.
	if !(got[0].x < got[1].x && got[1].x < got[2].x) {
		t.Fatalf("first row collapsed/ordered incorrectly: %+v", got[:3])
	}
	if math.Abs(got[3].x-got[0].x) > 0.5 {
		t.Fatalf("second row did not restart at first-row origin: first=%.1f second=%.1f", got[0].x, got[3].x)
	}
	if !(got[3].x < got[4].x) {
		t.Fatalf("second-row shapes collapsed: %+v", got[3:])
	}
	if got[3].y <= got[1].y+got[1].h-0.01 {
		t.Fatalf("second row overlaps tallest first-row shape: row2=%.1f firstBottom=%.1f", got[3].y, got[1].y+got[1].h)
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
