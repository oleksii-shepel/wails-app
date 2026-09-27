package legacy

import (
	"encoding/binary"
	"math"
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
	values := []uint16{
		officeArtPathInfo(officeArtPathMoveTo, 0),
		officeArtPathInfo(officeArtPathLineTo, 3),
		officeArtPathInfo(officeArtPathClose, 1),
		officeArtPathInfo(officeArtPathEnd, 0),
	}
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

func TestDecodeCustomGeometryAcceptsCompactFourByteVertexArray(t *testing.T) {
	vertices := make([]byte, 6+3*4)
	binary.LittleEndian.PutUint16(vertices[0:2], 3)
	binary.LittleEndian.PutUint16(vertices[2:4], 3)
	binary.LittleEndian.PutUint16(vertices[4:6], 0xFFF0)
	points := [][2]int16{{0, 0}, {100, 50}, {200, 100}}
	for i, point := range points {
		off := 6 + i*4
		binary.LittleEndian.PutUint16(vertices[off:off+2], uint16(point[0]))
		binary.LittleEndian.PutUint16(vertices[off+2:off+4], uint16(point[1]))
	}
	segments := make([]byte, 6+3*2)
	binary.LittleEndian.PutUint16(segments[0:2], 3)
	binary.LittleEndian.PutUint16(segments[2:4], 3)
	binary.LittleEndian.PutUint16(segments[4:6], 2)
	values := []uint16{
		officeArtPathInfo(officeArtPathMoveTo, 0),
		officeArtPathInfo(officeArtPathLineTo, 2),
		officeArtPathInfo(officeArtPathEnd, 0),
	}
	for i, value := range values {
		binary.LittleEndian.PutUint16(segments[6+i*2:8+i*2], value)
	}

	got := DecodeCustomGeometry([]escher.Property{
		{ID: escher.PropGeoRight, Value: 200},
		{ID: escher.PropGeoBottom, Value: 100},
		{ID: escher.PropVertices, IsComplex: true, Complex: vertices},
		{ID: escher.PropSegmentInfo, IsComplex: true, Complex: segments},
	})
	if got.Kind != ir.GeometryCustom || len(got.Paths) != 1 || len(got.Paths[0].Commands) != 3 {
		t.Fatalf("geometry=%+v", got)
	}
	if p := got.Paths[0].Commands[2].Points[0]; p.X != 1 || p.Y != 1 {
		t.Fatalf("last normalized point = %+v, want {1 1}", p)
	}
}

func TestDecodeCustomGeometryUsesMSOPATHINFOHighBitsAndIgnoresEditingEscapes(t *testing.T) {
	vertices := make([]byte, 6+2*4)
	binary.LittleEndian.PutUint16(vertices[0:2], 2)
	binary.LittleEndian.PutUint16(vertices[2:4], 2)
	binary.LittleEndian.PutUint16(vertices[4:6], 0xFFF0)
	points := [][2]int16{{10, 20}, {90, 80}}
	for i, point := range points {
		off := 6 + i*4
		binary.LittleEndian.PutUint16(vertices[off:off+2], uint16(point[0]))
		binary.LittleEndian.PutUint16(vertices[off+2:off+4], uint16(point[1]))
	}

	// Real OfficeArt encoding: 0x4000=MoveTo, 0xAC00=AutoLine escape,
	// 0x0001=one LineTo, 0x6001=Close, 0x8000=End.
	values := []uint16{0x4000, 0xAC00, 0x0001, 0x6001, 0x8000}
	segments := make([]byte, 6+len(values)*2)
	binary.LittleEndian.PutUint16(segments[0:2], uint16(len(values)))
	binary.LittleEndian.PutUint16(segments[2:4], uint16(len(values)))
	binary.LittleEndian.PutUint16(segments[4:6], 2)
	for i, value := range values {
		binary.LittleEndian.PutUint16(segments[6+i*2:8+i*2], value)
	}

	got := DecodeCustomGeometry([]escher.Property{
		{ID: escher.PropGeoRight, Value: 100},
		{ID: escher.PropGeoBottom, Value: 100},
		{ID: escher.PropVertices, IsComplex: true, Complex: vertices},
		{ID: escher.PropSegmentInfo, IsComplex: true, Complex: segments},
	})
	if got.Kind != ir.GeometryCustom || len(got.Paths) != 1 {
		t.Fatalf("geometry=%+v", got)
	}
	commands := got.Paths[0].Commands
	if len(commands) != 3 || commands[0].Op != "M" || commands[1].Op != "L" || commands[2].Op != "Z" {
		t.Fatalf("commands=%+v", commands)
	}
	if p := commands[1].Points[0]; p.X != 0.9 || p.Y != 0.8 {
		t.Fatalf("line endpoint=%+v, want {0.9 0.8}", p)
	}
}

func TestDecodeCustomGeometryHonorsNoFillAndNoLineEscapes(t *testing.T) {
	vertices := make([]byte, 6+2*8)
	binary.LittleEndian.PutUint16(vertices[0:2], 2)
	binary.LittleEndian.PutUint16(vertices[2:4], 2)
	binary.LittleEndian.PutUint16(vertices[4:6], 8)
	points := [][2]int32{{0, 0}, {21600, 21600}}
	for i, point := range points {
		off := 6 + i*8
		binary.LittleEndian.PutUint32(vertices[off:off+4], uint32(point[0]))
		binary.LittleEndian.PutUint32(vertices[off+4:off+8], uint32(point[1]))
	}
	values := []uint16{
		officeArtPathInfo(officeArtPathMoveTo, 0),
		officeArtEscapeInfo(officeArtEscapeNoFill, 0),
		officeArtEscapeInfo(officeArtEscapeNoLine, 0),
		officeArtPathInfo(officeArtPathLineTo, 1),
		officeArtPathInfo(officeArtPathEnd, 0),
	}
	segments := make([]byte, 6+len(values)*2)
	binary.LittleEndian.PutUint16(segments[0:2], uint16(len(values)))
	binary.LittleEndian.PutUint16(segments[2:4], uint16(len(values)))
	binary.LittleEndian.PutUint16(segments[4:6], 2)
	for i, value := range values {
		binary.LittleEndian.PutUint16(segments[6+i*2:8+i*2], value)
	}

	got := DecodeCustomGeometry([]escher.Property{
		{ID: escher.PropGeoRight, Value: 21600},
		{ID: escher.PropGeoBottom, Value: 21600},
		{ID: escher.PropVertices, IsComplex: true, Complex: vertices},
		{ID: escher.PropSegmentInfo, IsComplex: true, Complex: segments},
	})
	if got.Kind != ir.GeometryCustom || len(got.Paths) != 1 {
		t.Fatalf("geometry=%+v", got)
	}
	if got.Paths[0].Fill || got.Paths[0].Stroke {
		t.Fatalf("path paint flags = fill:%v stroke:%v, want both false", got.Paths[0].Fill, got.Paths[0].Stroke)
	}
}

func TestDecodeGeometryAdjustmentsPreservesPresetSlots(t *testing.T) {
	got := DecodeGeometryAdjustments([]escher.Property{
		{ID: escher.PropAdjust2, Value: uint32(int32(200))},
		{ID: escher.PropAdjust4, Value: uint32(0xfffffe70)},
	})
	if len(got) != 4 || got[0] != 0 || got[1] != 200 || got[2] != 0 || got[3] != -400 {
		t.Fatalf("adjustments=%v, want [0 200 0 -400]", got)
	}
}

func TestDecodeArcGeometryMaterializesDefaultClockwiseQuarterArc(t *testing.T) {
	got := DecodeArcGeometry(nil)
	if got.Kind != ir.GeometryCustom || len(got.Paths) != 2 {
		t.Fatalf("arc geometry=%+v", got)
	}
	stroke := got.Paths[0].Commands
	if len(stroke) != 2 || stroke[0].Op != "M" || stroke[1].Op != "C" {
		t.Fatalf("stroke commands=%+v", stroke)
	}
	assertPointNear := func(name string, got ir.PointF, x, y float64) {
		t.Helper()
		const eps = 1e-6
		if math.Abs(got.X-x) > eps || math.Abs(got.Y-y) > eps {
			t.Fatalf("%s=%+v want (%.9f, %.9f)", name, got, x, y)
		}
	}
	assertPointNear("start", stroke[0].Points[0], 0, 0)
	assertPointNear("c1", stroke[1].Points[0], 0.5522847498307933, 0)
	assertPointNear("c2", stroke[1].Points[1], 1, 0.44771525016920666)
	assertPointNear("end", stroke[1].Points[2], 1, 1)
	fill := got.Paths[1].Commands
	if len(fill) != 4 || fill[2].Op != "L" || fill[3].Op != "Z" {
		t.Fatalf("fill commands=%+v", fill)
	}
	assertPointNear("center", fill[2].Points[0], 0, 1)
}

func TestDecodeArcGeometryUsesSectorViewBoxAndAuthoredAngles(t *testing.T) {
	vertices := make([]byte, 6+9*8)
	binary.LittleEndian.PutUint16(vertices[0:2], 9)
	binary.LittleEndian.PutUint16(vertices[2:4], 9)
	binary.LittleEndian.PutUint16(vertices[4:6], 8)
	points := [][2]int32{
		{-21327, 0}, {21873, 43200}, {0, 2}, {21873, 21600},
		{-21327, 0}, {21873, 43200}, {0, 2}, {21873, 21600}, {273, 21600},
	}
	for i, point := range points {
		off := 6 + i*8
		binary.LittleEndian.PutUint32(vertices[off:off+4], uint32(point[0]))
		binary.LittleEndian.PutUint32(vertices[off+4:off+8], uint32(point[1]))
	}
	got := DecodeArcGeometry([]escher.Property{
		{ID: escher.PropGeoRight, Value: 21873},
		{ID: escher.PropVertices, IsComplex: true, Complex: vertices},
		{ID: escher.PropAdjust1, Value: 17647192},
	})
	if got.Kind != ir.GeometryCustom || len(got.Paths) != 2 {
		t.Fatalf("arc geometry=%+v", got)
	}
	if commands := got.Paths[0].Commands; len(commands) != 3 || commands[0].Op != "M" || commands[1].Op != "C" || commands[2].Op != "C" {
		t.Fatalf("adjusted arc commands=%+v", commands)
	}
	center := got.Paths[1].Commands[len(got.Paths[1].Commands)-2].Points[0]
	if math.Abs(center.X-(273.0/21873.0)) > 1e-9 || math.Abs(center.Y-1) > 1e-9 {
		t.Fatalf("center=%+v", center)
	}
}

func TestDecodeGeometryAdjustmentSlotsDistinguishesMissingAndAuthoredZero(t *testing.T) {
	values, present := DecodeGeometryAdjustmentSlots([]escher.Property{
		{ID: escher.PropAdjust2, Value: 0},
	})
	if len(values) != 2 || len(present) != 2 {
		t.Fatalf("slots = values:%v present:%v", values, present)
	}
	if present[0] || !present[1] || values[1] != 0 {
		t.Fatalf("slots = values:%v present:%v, want missing slot 1 and authored-zero slot 2", values, present)
	}
}
