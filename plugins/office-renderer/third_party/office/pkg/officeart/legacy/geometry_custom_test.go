package legacy

import (
	"encoding/binary"
	"testing"

	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

func TestDecodeCustomGeometryProjectsOfficeArtPath(t *testing.T) {
	vertices := make([]byte, 6+4*8)
	binary.LittleEndian.PutUint16(vertices[0:2], 4)
	binary.LittleEndian.PutUint16(vertices[2:4], 4)
	binary.LittleEndian.PutUint16(vertices[4:6], 8)
	points := [][2]int32{{0, 0}, {21600, 0}, {21600, 21600}, {0, 21600}}
	for i, point := range points {
		off := 6 + i*8
		binary.LittleEndian.PutUint32(vertices[off:off+4], uint32(point[0]))
		binary.LittleEndian.PutUint32(vertices[off+4:off+8], uint32(point[1]))
	}
	segments := make([]byte, 6+4*2)
	binary.LittleEndian.PutUint16(segments[0:2], 4)
	binary.LittleEndian.PutUint16(segments[2:4], 4)
	binary.LittleEndian.PutUint16(segments[4:6], 2)
	values := []uint16{officeArtPathMoveTo, uint16(3<<3) | officeArtPathLineTo, officeArtPathClose, officeArtPathEnd}
	for i, value := range values {
		binary.LittleEndian.PutUint16(segments[6+i*2:8+i*2], value)
	}

	got := DecodeCustomGeometry([]escher.Property{
		{ID: escher.PropGeoRight, Value: 21600},
		{ID: escher.PropGeoBottom, Value: 21600},
		{ID: escher.PropVertices, IsComplex: true, Complex: vertices},
		{ID: escher.PropSegmentInfo, IsComplex: true, Complex: segments},
		{ID: escher.PropAdjust1, Value: 10800},
	})
	if got.Kind != ir.GeometryCustom || len(got.Paths) != 1 {
		t.Fatalf("geometry=%+v", got)
	}
	if len(got.Paths[0].Commands) != 5 {
		t.Fatalf("commands=%+v", got.Paths[0].Commands)
	}
	if len(got.Adjustments) != 1 || got.Adjustments[0] != 10800 {
		t.Fatalf("adjustments=%v", got.Adjustments)
	}
}
