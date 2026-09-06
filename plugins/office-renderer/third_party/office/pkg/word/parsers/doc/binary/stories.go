package docbinary

import (
	"encoding/binary"
	"strings"
	"unicode"
	"unicode/utf16"

	"golang.org/x/text/encoding/charmap"
)

// StoryCounts contains the Word97 story character counts from FibRgLw97.
type StoryCounts struct {
	Text, Ftn, Hdr, Mcr, Atn, Edn, Txbx, HdrTxbx int
}

// TextboxStoryBase returns the absolute CP where the requested textbox story
// begins. Main-document textbox text follows the ordinary text/note stories;
// header textbox text follows the main textbox story.
func TextboxStoryBase(counts StoryCounts, header bool) int {
	base := counts.Text + counts.Ftn + counts.Hdr + counts.Mcr + counts.Atn + counts.Edn
	if header {
		base += counts.Txbx
	}
	return base
}

// ParseStoryCounts reads the story-count slots from a Word97 FIB prefix.
func ParseStoryCounts(fib []byte, base, slots int) StoryCounts {
	read := func(slot int) int {
		off := base + slot*4
		if slot < 0 || slot >= slots || off < 0 || off+4 > len(fib) { return 0 }
		return int(binary.LittleEndian.Uint32(fib[off : off+4]))
	}
	// FibRgLw97: ccpText starts at slot 3, followed by the story lengths.
	return StoryCounts{Text:read(3),Ftn:read(4),Hdr:read(5),Mcr:read(6),Atn:read(7),Edn:read(8),Txbx:read(9),HdrTxbx:read(10)}
}

// ParseCPArray parses a raw CP-only PLC payload. It is used by structures such
// as PlcfHdd where every 4-byte value is a character position.
func ParseCPArray(plc []byte) []int {
	if len(plc) < 4 || len(plc)%4 != 0 { return nil }
	out := make([]int, len(plc)/4)
	for i := range out { out[i] = int(binary.LittleEndian.Uint32(plc[i*4:(i+1)*4])) }
	return out
}

// ParseNoteRefCPs parses PLCFFndRef/PLCFE EdnRef style CP arrays. Word's
// canonical note PLC includes a two-byte descriptor per reference; some
// producers emit a CP-only form, which is retained as a compatibility case.
func ParseNoteRefCPs(plc []byte) []int {
	switch {
	case len(plc) >= 10 && (len(plc)-4)%6 == 0:
		count := (len(plc)-4)/6
		return parseCPPrefix(plc,count)
	case len(plc) >= 8 && len(plc)%4 == 0:
		count := len(plc)/4-1
		return parseCPPrefix(plc,count)
	default:
		return nil
	}
}

// ParseAnnotationRefCPs parses PLCFANDREF, whose records contain 30-byte
// ATRDPre10 descriptors after the CP array.
func ParseAnnotationRefCPs(plc []byte) []int {
	const recordSize = 34
	if len(plc) >= 4+recordSize && (len(plc)-4)%recordSize == 0 {
		count := (len(plc)-4)/recordSize
		if count > 0 { return parseCPPrefix(plc,count) }
	}
	return ParseNoteRefCPs(plc)
}

// ParseNoteTextCPs parses the CP-only text-boundary PLC used by note stories.
func ParseNoteTextCPs(plc []byte) []int {
	if len(plc) < 8 || len(plc)%4 != 0 { return nil }
	return ParseCPArray(plc)
}

// ParseBookmarkStartCPs parses PlcfBkf's CP array. Each bookmark contributes a
// four-byte FBKF record in addition to its CP.
func ParseBookmarkStartCPs(plc []byte, maxCP int) []int {
	if len(plc)<12 || (len(plc)-4)%8!=0 { return nil }
	count:=(len(plc)-4)/8
	out:=make([]int,count)
	for i:=0;i<count;i++ {
		cp:=int(binary.LittleEndian.Uint32(plc[i*4:(i+1)*4]))
		if cp<0 || cp>maxCP || (i>0 && cp<out[i-1]) { return nil }
		out[i]=cp
	}
	return out
}

// ParseBookmarkEndCPs parses the CP-only PlcfBkl array.
func ParseBookmarkEndCPs(plc []byte, count, maxCP int) []int {
	if count<=0 || len(plc)<(count+1)*4 { return nil }
	out:=make([]int,count)
	for i:=0;i<count;i++ { cp:=int(binary.LittleEndian.Uint32(plc[i*4:(i+1)*4])); if cp<0||cp>maxCP{return nil}; out[i]=cp }
	return out
}

// ParseBookmarkNames parses SttbfBkmk in either Unicode or legacy single-byte form.
func ParseBookmarkNames(data []byte) []string {
	if len(data)<6{return nil}
	extended:=binary.LittleEndian.Uint16(data[0:2])==0xFFFF
	count:=int(binary.LittleEndian.Uint16(data[2:4]))
	extra:=int(binary.LittleEndian.Uint16(data[4:6]))
	if count<=0||count>4096||extra<0||extra>64{return nil}
	pos:=6; out:=make([]string,0,count)
	for i:=0;i<count;i++ {
		if pos+2>len(data){return nil}; chars:=int(binary.LittleEndian.Uint16(data[pos:pos+2]));pos+=2
		if chars<0||chars>1024{return nil}; n:=chars;if extended{n*=2};if pos+n+extra>len(data){return nil}
		var name string
		if extended { u:=make([]uint16,chars);for j:=range u{u[j]=binary.LittleEndian.Uint16(data[pos+j*2:pos+j*2+2])};name=string(utf16.Decode(u)) } else { decoded,err:=charmap.Windows1252.NewDecoder().Bytes(data[pos:pos+n]);if err!=nil{return nil};name=string(decoded) }
		if !validBookmarkName(name){return nil};out=append(out,name);pos+=n+extra
	}
	return out
}

func parseCPPrefix(plc []byte,count int) []int { if count<=0||len(plc)<count*4{return nil};out:=make([]int,count);for i:=range out{out[i]=int(binary.LittleEndian.Uint32(plc[i*4:(i+1)*4]))};return out }
func validBookmarkName(name string) bool { name=strings.TrimSpace(name);if name==""||len(name)>256{return false};for i,r:=range name{if r=='_'||unicode.IsLetter(r)||(unicode.IsDigit(r)&&i>0){continue};return false};return true }
