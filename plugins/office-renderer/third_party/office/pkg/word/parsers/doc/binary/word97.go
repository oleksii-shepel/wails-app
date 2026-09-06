package docbinary

import "encoding/binary"

const (
	fibSignature = 0xA5EC
	fibWord97    = 0x00C1
)

// Word97Layout is the variable-length FIB layout information needed by the
// sequential DOC reader before format-specific semantic decoding begins.
type Word97Layout struct {
	Version         uint16
	TableStreamName string
	StoryCountsBase int
	StoryCountSlots int
	FibBase         int
	CbRgFcLcb       int
}

// ParseWord97Layout walks the variable-length FIB prefix and locates FibRgLw97
// and FibRgFcLcb97 without reading the document body or table stream.
func ParseWord97Layout(fib []byte) (Word97Layout, bool) {
	if len(fib) < 34 || binary.LittleEndian.Uint16(fib[0:2]) != fibSignature {
		return Word97Layout{}, false
	}
	version := binary.LittleEndian.Uint16(fib[2:4])
	if version < fibWord97 {
		return Word97Layout{}, false
	}
	flags := binary.LittleEndian.Uint16(fib[10:12])
	tableName := "0Table"
	if flags&(1<<9) != 0 {
		tableName = "1Table"
	}

	pos := 32
	if pos+2 > len(fib) {
		return Word97Layout{}, false
	}
	csw := int(binary.LittleEndian.Uint16(fib[pos : pos+2]))
	pos += 2 + csw*2
	if pos+2 > len(fib) {
		return Word97Layout{}, false
	}
	cslw := int(binary.LittleEndian.Uint16(fib[pos : pos+2]))
	storyBase := pos + 2
	if storyBase+cslw*4 > len(fib) {
		return Word97Layout{}, false
	}
	pos = storyBase + cslw*4
	if pos+2 > len(fib) {
		return Word97Layout{}, false
	}
	cbRgFcLcb := int(binary.LittleEndian.Uint16(fib[pos : pos+2]))
	fibBase := pos + 2
	if cbRgFcLcb < 0 || fibBase+cbRgFcLcb*8 > len(fib) {
		return Word97Layout{}, false
	}
	return Word97Layout{
		Version:         version,
		TableStreamName: tableName,
		StoryCountsBase: storyBase,
		StoryCountSlots: cslw,
		FibBase:         fibBase,
		CbRgFcLcb:       cbRgFcLcb,
	}, true
}

// FcLcb returns one FibRgFcLcb97 pair.
func (l Word97Layout) FcLcb(fib []byte, index int) (fc, lcb int, ok bool) {
	if index < 0 || index >= l.CbRgFcLcb {
		return 0, 0, false
	}
	off := l.FibBase + index*8
	if off < 0 || off+8 > len(fib) {
		return 0, 0, false
	}
	return int(binary.LittleEndian.Uint32(fib[off : off+4])), int(binary.LittleEndian.Uint32(fib[off+4 : off+8])), true
}
