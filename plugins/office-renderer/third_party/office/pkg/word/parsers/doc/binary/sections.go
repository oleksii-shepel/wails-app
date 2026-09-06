package docbinary

import "encoding/binary"

// SectionDescriptor is the structural portion of one PlcfSed entry.
type SectionDescriptor struct {
	StartCP    int
	EndCP      int
	SepxOffset uint32
	HasSepx    bool
}

// ParseSectionDescriptors parses PlcfSed without interpreting SEPX SPRMs.
func ParseSectionDescriptors(plc []byte) []SectionDescriptor {
	if len(plc)<20 || (len(plc)-4)%16!=0{return nil}
	count:=(len(plc)-4)/16
	if count<=0{return nil}
	out:=make([]SectionDescriptor,0,count)
	for i:=0;i<count;i++ {
		sedOff:=(count+1)*4+i*12
		if sedOff+12>len(plc){return nil}
		start:=int(binary.LittleEndian.Uint32(plc[i*4:(i+1)*4]))
		end:=int(binary.LittleEndian.Uint32(plc[(i+1)*4:(i+2)*4]))
		if start<0||end<start{return nil}
		fc:=binary.LittleEndian.Uint32(plc[sedOff+2:sedOff+6])
		out=append(out,SectionDescriptor{StartCP:start,EndCP:end,SepxOffset:fc,HasSepx:fc!=0xFFFFFFFF})
	}
	return out
}
