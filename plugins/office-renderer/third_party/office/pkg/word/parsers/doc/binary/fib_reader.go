package docbinary

import (
	"encoding/binary"
	"io"
)

// ReadWord97FIBReaderAt reads only the declared Word97 FIB area from a
// WordDocument ReaderAt. It never materializes the document stream.
func ReadWord97FIBReaderAt(r io.ReaderAt, size int64) ([]byte, bool) {
	if r == nil || size < 34 { return nil, false }
	probeSize:=int64(512);if size<probeSize{probeSize=size}
	probe:=make([]byte,int(probeSize))
	if _,err:=r.ReadAt(probe,0);err!=nil&&err!=io.EOF{return nil,false}
	if len(probe)<34{return nil,false}
	pos:=32
	csw:=int(binary.LittleEndian.Uint16(probe[pos:pos+2]));pos+=2+csw*2
	if pos+2>len(probe){return nil,false}
	cslw:=int(binary.LittleEndian.Uint16(probe[pos:pos+2]));pos+=2+cslw*4
	if pos+2>len(probe){return nil,false}
	cb:=int(binary.LittleEndian.Uint16(probe[pos:pos+2]));fibEnd:=pos+2+cb*8
	if fibEnd<=len(probe){return append([]byte(nil),probe[:fibEnd]...),true}
	if int64(fibEnd)>size{return nil,false}
	fib:=make([]byte,fibEnd);if _,err:=r.ReadAt(fib,0);err!=nil&&err!=io.EOF{return nil,false}
	return fib,true
}
