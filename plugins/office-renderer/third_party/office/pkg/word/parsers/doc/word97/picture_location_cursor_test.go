package word97

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestDrawingCursorResolvesZeroPicLocation(t *testing.T) {
	image := &wordir.Image{MIMEType: "image/png", Data: []byte{1, 2, 3}}
	inner := &sliceCursor{runs: []Run{{
		Text:           "\x01",
		StartCP:        7,
		Spec:           true,
		PicLocation:    0,
		HasPicLocation: true,
	}}}
	cursor := NewDrawingCursorWithPictureLocations(inner, nil, nil, map[int]*wordir.Image{0: image}, nil, nil)

	run, ok, err := cursor.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if !ok {
		t.Fatal("Next returned no run")
	}
	if run.Kind != "image" || run.Image != image || run.StartCP != 7 {
		t.Fatalf("resolved run = %#v", run)
	}
	if _, ok, err := cursor.Next(); err != nil || ok {
		t.Fatalf("second Next = ok %v err %v, want exhausted", ok, err)
	}
}

func TestRecognizedInlinePictureBlockMatchesWordPictureTypes(t *testing.T) {
	for _, test := range []struct {
		blockType uint16
		mapping   uint16
		want      bool
	}{
		{0x0008, 0, true},
		{0x000A, 0, true},
		{0x0000, 0x0064, true},
		{0x0002, 0x0064, true},
		{0x0000, 0, false},
		{0x000E, 0x0064, false},
	} {
		if got := recognizedInlinePictureBlock(test.blockType, test.mapping); got != test.want {
			t.Fatalf("recognizedInlinePictureBlock(%#x, %#x) = %v, want %v", test.blockType, test.mapping, got, test.want)
		}
	}
}
