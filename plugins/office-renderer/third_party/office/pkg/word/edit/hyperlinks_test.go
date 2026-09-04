package edit

import "testing"

func TestDocumentHyperlinks(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	r1 := p1.AddRun()
	r1.SetText("click here")
	r1.SetHyperlink("https://example.com")

	p2 := d.AddParagraph()
	r2 := p2.AddRun()
	r2.SetText("anchor link")
	r2.ir().Anchor = "bookmark1"
	r2.ir().Kind = "hyperlink"

	p3 := d.AddParagraph()
	p3.AddRun().SetText("no link")

	h := d.AddHeader("default")
	hp := h.AddParagraph()
	hr := hp.AddRun()
	hr.SetText("header link")
	hr.SetHyperlink("https://header.example.com")

	links := d.Hyperlinks()
	if len(links) != 3 {
		t.Fatalf("links = %d, want 3", len(links))
	}

	if links[0].Target != "https://example.com" {
		t.Fatalf("link0 target = %q", links[0].Target)
	}
	if links[0].Text != "click here" {
		t.Fatalf("link0 text = %q", links[0].Text)
	}
	if links[0].Location != "body" {
		t.Fatalf("link0 location = %q", links[0].Location)
	}

	if links[1].Anchor != "bookmark1" {
		t.Fatalf("link1 anchor = %q", links[1].Anchor)
	}

	if links[2].Location != "header" {
		t.Fatalf("link2 location = %q", links[2].Location)
	}
}

func TestDocumentHyperlinkCount(t *testing.T) {
	d := NewDocument()
	if d.HyperlinkCount() != 0 {
		t.Fatalf("count = %d, want 0", d.HyperlinkCount())
	}
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetHyperlink("https://example.com")
	if d.HyperlinkCount() != 1 {
		t.Fatalf("count = %d, want 1", d.HyperlinkCount())
	}
}