package layout

import "testing"

func TestPageWindowRetainsOnlyRequestedRange(t *testing.T) {
	window := NewPageWindow(3, 3)
	for n := 1; n <= 4; n++ {
		keepGoing := window.Push(Page{Number: n})
		if n < 3 && !keepGoing {
			t.Fatalf("Push page %d stopped before target", n)
		}
		if n == 3 && keepGoing {
			t.Fatal("Push target page should allow pagination to stop")
		}
		if n == 3 {
			break
		}
	}
	pages := window.Pages()
	if len(pages) != 1 || pages[0].Number != 3 {
		t.Fatalf("Pages = %#v, want only page 3", pages)
	}
}

func TestPageWindowNormalizesRange(t *testing.T) {
	window := NewPageWindow(0, -1)
	if window.Start() != 1 || window.End() != 0 {
		t.Fatalf("range = %d..%d, want 1..unbounded", window.Start(), window.End())
	}
}
