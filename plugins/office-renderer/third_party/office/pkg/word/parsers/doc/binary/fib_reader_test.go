package docbinary

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestReadWord97FIBReaderAtReadsDeclaredAreaOnly(t *testing.T){
	data:=make([]byte,2048)
	// csw=0 at offset 32, cslw=0 at offset 34, cbRgFcLcb=40 at offset 36.
	binary.LittleEndian.PutUint16(data[32:34],0)
	binary.LittleEndian.PutUint16(data[34:36],0)
	binary.LittleEndian.PutUint16(data[36:38],40)
	want:=38+40*8
	fib,ok:=ReadWord97FIBReaderAt(bytes.NewReader(data),int64(len(data)))
	if !ok{t.Fatal("ReadWord97FIBReaderAt failed")}
	if len(fib)!=want{t.Fatalf("len=%d want=%d",len(fib),want)}
}
