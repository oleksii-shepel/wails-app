package docbinary

import (
	"encoding/binary"
	"fmt"
)

const FKPPageSize = 512

// BTEPageNumbers decodes a PlcBteChpx/PlcBtePapx and returns the referenced
// WordDocument FKP page numbers. Both PLCs share the same layout: (n+1) FCs
// followed by n page numbers.
func BTEPageNumbers(plc []byte) ([]int, error) {
	if len(plc) < 12 { return nil, fmt.Errorf("doc binary: truncated PlcBte") }
	n := (len(plc)-4)/8
	if n <= 0 || (n+1)*4+n*4 > len(plc) { return nil, fmt.Errorf("doc binary: invalid PlcBte") }
	out := make([]int,n); base := (n+1)*4
	for i:=0;i<n;i++ { out[i]=int(binary.LittleEndian.Uint32(plc[base+i*4:base+i*4+4])) }
	return out,nil
}

type RawCHPX struct { FC uint32; Grpprl []byte }
func ParseCHPXFKP(page []byte) ([]RawCHPX,error) {
	if len(page)<FKPPageSize{return nil,fmt.Errorf("doc binary: truncated ChpxFkp")}; crun:=int(page[FKPPageSize-1]); if crun==0{return nil,nil}; rgbxOff:=(crun+1)*4; if rgbxOff+crun>FKPPageSize-1{return nil,fmt.Errorf("doc binary: invalid ChpxFkp run table")}; out:=make([]RawCHPX,0,crun)
	for i:=0;i<crun;i++ { r:=RawCHPX{FC:binary.LittleEndian.Uint32(page[i*4:i*4+4])}; bx:=int(page[rgbxOff+i]); if bx!=0 { off:=bx*2; if off+1<FKPPageSize { cb:=int(page[off]); if cb>0&&off+1+cb<=FKPPageSize { r.Grpprl=append([]byte(nil),page[off+1:off+1+cb]...) } } }; out=append(out,r) }; return out,nil
}

type RawPAPX struct { FC uint32; StyleIndex int; HasStyle bool; Grpprl []byte }
func ParsePAPXFKP(page []byte) ([]RawPAPX,error) {
	if len(page)<FKPPageSize{return nil,fmt.Errorf("doc binary: truncated PapxFkp")}; cpara:=int(page[FKPPageSize-1]); if cpara==0{return nil,nil}; rgbxStart:=(cpara+1)*4; if rgbxStart+cpara*13>FKPPageSize-1{return nil,fmt.Errorf("doc binary: invalid PapxFkp paragraph table")}; out:=make([]RawPAPX,0,cpara)
	for i:=0;i<cpara;i++ { r:=RawPAPX{FC:binary.LittleEndian.Uint32(page[i*4:i*4+4])}; b:=int(page[rgbxStart+i*13]); if b!=0 { off:=b*2; if off<FKPPageSize-1 { cb:=int(page[off]); if cb==0 { if off+4<=FKPPageSize { cb2:=int(page[off+1]); r.StyleIndex=int(binary.LittleEndian.Uint16(page[off+2:off+4])); r.HasStyle=true; gl:=(cb2-1)*2; goff:=off+4; if gl>0&&goff+gl<=FKPPageSize { r.Grpprl=append([]byte(nil),page[goff:goff+gl]...) } } } else if off+3<=FKPPageSize { r.StyleIndex=int(binary.LittleEndian.Uint16(page[off+1:off+3])); r.HasStyle=true; gl:=(cb-1)*2; goff:=off+3; if gl>0&&goff+gl<=FKPPageSize { r.Grpprl=append([]byte(nil),page[goff:goff+gl]...) } } } }; out=append(out,r) }; return out,nil
}
