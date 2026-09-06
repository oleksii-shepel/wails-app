package officeart

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestBLIPMetafileImageProjectsEMFToSVG(t *testing.T) {
	data := make([]byte, 108)
	binary.LittleEndian.PutUint32(data[0:4], emrHeader)
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)))
	binary.LittleEndian.PutUint32(data[8:12], 0)
	binary.LittleEndian.PutUint32(data[12:16], 0)
	binary.LittleEndian.PutUint32(data[16:20], 320)
	binary.LittleEndian.PutUint32(data[20:24], 180)
	copy(data[40:44], []byte(" EMF"))

	image := (BLIP{MIMEType: "image/x-emf", Data: data}).MetafileImage()
	if image == nil {
		t.Fatal("MetafileImage returned nil")
	}
	if image.MIMEType != "image/svg+xml" {
		t.Fatalf("MIMEType = %q, want image/svg+xml", image.MIMEType)
	}
	if image.Width <= 0 || image.Height <= 0 {
		t.Fatalf("size = %dx%d, want positive EMU dimensions", image.Width, image.Height)
	}
	if !strings.Contains(string(image.Data), `viewBox="0 0 320.000 180.000"`) {
		t.Fatalf("SVG = %q, want projected EMF bounds", string(image.Data))
	}
}

func TestBLIPImagePrefersCanonicalMetafileProjection(t *testing.T) {
	data := make([]byte, 108)
	binary.LittleEndian.PutUint32(data[0:4], emrHeader)
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)))
	binary.LittleEndian.PutUint32(data[16:20], 96)
	binary.LittleEndian.PutUint32(data[20:24], 96)
	copy(data[40:44], []byte(" EMF"))

	image := (BLIP{MIMEType: "image/x-emf", Data: data}).Image()
	if image == nil || image.MIMEType != "image/svg+xml" {
		t.Fatalf("Image = %#v, want SVG projection", image)
	}
}
