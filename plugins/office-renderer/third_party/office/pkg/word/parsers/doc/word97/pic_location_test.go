package word97

import (
	"testing"

	docformat "office/pkg/word/parsers/doc/format"
	wordir "office/pkg/word/ir"
)

type picLocationTestCursor struct {
	runs []Run
	pos  int
}

func (c *picLocationTestCursor) Next() (Run, bool, error) {
	if c == nil || c.pos >= len(c.runs) {
		return Run{}, false, nil
	}
	r := c.runs[c.pos]
	c.pos++
	return r, true, nil
}

func TestPicLocationFromCHPX(t *testing.T) {
	grpprl := []byte{0x03, 0x6A, 0x34, 0x12, 0x00, 0x00, 0x55, 0x08, 0x01}
	got, ok := picLocationFromCHPX(grpprl)
	if !ok || got != 0x1234 {
		t.Fatalf("picLocationFromCHPX = %d, %v; want 4660, true", got, ok)
	}

	var run Run
	applyCHPX(&run, docformat.CHPXEntry{Sprms: grpprl})
	if !run.HasPicLocation || run.PicLocation != 0x1234 {
		t.Fatalf("Run picture location = %d, %v; want 4660, true", run.PicLocation, run.HasPicLocation)
	}
	if run.Format.PicLocation != 0x1234 {
		t.Fatalf("Run.Format.PicLocation = %d, want 4660", run.Format.PicLocation)
	}
}

func TestDrawingCursorResolvesPicLocationWithoutFData(t *testing.T) {
	image := &wordir.Image{MIMEType: "image/png", Data: []byte{1, 2, 3}}
	inner := &picLocationTestCursor{runs: []Run{{
		Text: "\x01",
		Spec: true,
		HasPicLocation: true,
		PicLocation: 128,
		Format: wordir.RunFormat{PicLocation: 128},
	}}}
	cursor := NewDrawingCursorWithPictureLocations(inner, nil, nil, map[int]*wordir.Image{128: image}, nil, nil)
	run, ok, err := cursor.Next()
	if err != nil {
		t.Fatal(err)
	}
	if !ok || run.Kind != "image" || run.Image != image {
		t.Fatalf("drawing run = %#v, want image run", run)
	}
}

func TestDrawingCursorReusesRepeatedPicLocation(t *testing.T) {
	image := &wordir.Image{MIMEType: "image/png", Data: []byte{1}}
	inner := &picLocationTestCursor{runs: []Run{
		{Text: "\x01", Spec: true, HasPicLocation: true, PicLocation: 128, Format: wordir.RunFormat{PicLocation: 128}},
		{Text: "\x01", Spec: true, HasPicLocation: true, PicLocation: 128, Format: wordir.RunFormat{PicLocation: 128}},
	}}
	cursor := NewDrawingCursorWithPictureLocations(inner, nil, nil, map[int]*wordir.Image{128: image}, nil, nil)
	for i := 0; i < 2; i++ {
		run, ok, err := cursor.Next()
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if !ok || run.Kind != "image" || run.Image != image {
			t.Fatalf("run %d = %#v, want reused image", i, run)
		}
	}
}
