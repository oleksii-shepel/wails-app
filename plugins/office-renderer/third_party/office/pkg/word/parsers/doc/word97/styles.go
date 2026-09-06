package word97

import (
	"encoding/binary"
	"strings"

	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"
)

// RawStyle is one Word97 STD entry decoded from STSH before inheritance is
// resolved. Keeping this representation in the sequential parser lets both the
// bounded stream and full materializer consume the same binary style decoder.
type RawStyle struct {
	Index         int
	STI           int
	Kind          int
	Name          string
	BasedOnIndex  int
	LinkedIndex   int
	RawParaFormat wordir.ParagraphFormat
	RawRunFormat  wordir.RunFormat
	ParaSprms     []byte
	RunSprms      []byte
}

// ParseStyleSheet decodes a Word97+ STSH blob into raw paragraph/character
// styles. PAPX and CHPX semantics are delegated to the canonical format parser.
func ParseStyleSheet(stsh []byte) []RawStyle {
	if len(stsh) < 6 { return nil }
	cbStshi := int(binary.LittleEndian.Uint16(stsh[0:2]))
	if cbStshi < 4 || 2+cbStshi > len(stsh) { return nil }
	stshi := stsh[2:2+cbStshi]
	cstd := int(binary.LittleEndian.Uint16(stshi[0:2]))
	cbSTDBaseInFile := int(binary.LittleEndian.Uint16(stshi[2:4]))
	if cstd <= 0 || cbSTDBaseInFile < 10 { return nil }

	pos := 2 + cbStshi
	out := make([]RawStyle, 0, cstd)
	for istd := 0; istd < cstd && pos+2 <= len(stsh); istd++ {
		cbStd := int(binary.LittleEndian.Uint16(stsh[pos:pos+2])); pos += 2
		if cbStd == 0 { continue }
		if cbStd < 0 || pos+cbStd > len(stsh) { break }
		std := stsh[pos:pos+cbStd]; pos += cbStd
		if pos&1 != 0 { pos++ }
		if raw, ok := parseSTD(std, istd, cbSTDBaseInFile); ok { out = append(out, raw) }
	}
	if len(out)==0 { return nil }
	return out
}

func parseSTD(std []byte, istd, cbSTDBaseInFile int) (RawStyle, bool) {
	if len(std)<10 || cbSTDBaseInFile<10 || cbSTDBaseInFile>len(std) { return RawStyle{},false }
	w0:=binary.LittleEndian.Uint16(std[0:2]); w1:=binary.LittleEndian.Uint16(std[2:4]); w2:=binary.LittleEndian.Uint16(std[4:6])
	raw:=RawStyle{Index:istd,STI:int(w0&0x0FFF),Kind:int(w1&0x000F),BasedOnIndex:int(w1>>4),LinkedIndex:-1}
	cupx:=int(w2&0x000F)
	if raw.BasedOnIndex==0x0FFF { raw.BasedOnIndex=-1 }
	if cbSTDBaseInFile>=12 && len(std)>=12 { link:=int(binary.LittleEndian.Uint16(std[10:12])&0x0FFF); if link!=0 { raw.LinkedIndex=link } }
	pos:=cbSTDBaseInFile
	name,consumed,ok:=parseXstz(std[pos:]); if !ok{return RawStyle{},false};raw.Name=name;pos+=consumed
	switch raw.Kind {
	case 1:
		if cupx>=1 && pos<len(std) { var n int; raw.RawParaFormat,raw.ParaSprms,n=parsePapxUPX(std[pos:],istd);pos+=n }
		if cupx>=2 && pos<len(std) { raw.RawRunFormat,raw.RunSprms,_=parseChpxUPX(std[pos:]) }
		return raw,true
	case 2:
		if cupx>=1 && pos<len(std) { raw.RawRunFormat,raw.RunSprms,_=parseChpxUPX(std[pos:]) }
		return raw,true
	case 3:
		return raw,true
	default:
		return RawStyle{},false
	}
}

func parseXstz(data []byte)(string,int,bool){
	if len(data)<4{return "",0,false};cch:=int(binary.LittleEndian.Uint16(data[0:2]));need:=2+cch*2+2
	if cch<0||need>len(data){return "",0,false};runes:=make([]rune,0,cch);for i:=0;i<cch;i++{runes=append(runes,rune(binary.LittleEndian.Uint16(data[2+i*2:])))}
	name:=string(runes);if i:=strings.IndexRune(name,',');i>=0{name=name[:i]};name=strings.TrimSpace(name);if name==""{return "",0,false};return name,need,true
}

func parsePapxUPX(data []byte,istd int)(wordir.ParagraphFormat,[]byte,int){
	if len(data)<2{return wordir.ParagraphFormat{},nil,len(data)};cb:=int(binary.LittleEndian.Uint16(data[0:2]));if cb<0||2+cb>len(data){return wordir.ParagraphFormat{},nil,len(data)}
	consumed:=2+cb;if consumed&1!=0{consumed++};payload:=data[2:2+cb];sprms:=payload;if len(payload)>=2&&int(binary.LittleEndian.Uint16(payload[:2]))==istd{sprms=payload[2:]}
	raw:=append([]byte(nil),sprms...);format,_:=docformat.DecodePAPX(raw);return format,raw,consumed
}

func parseChpxUPX(data []byte)(wordir.RunFormat,[]byte,int){
	if len(data)<2{return wordir.RunFormat{},nil,len(data)};cb:=int(binary.LittleEndian.Uint16(data[0:2]));if cb<0||2+cb>len(data){return wordir.RunFormat{},nil,len(data)}
	consumed:=2+cb;if consumed&1!=0{consumed++};sprms:=append([]byte(nil),data[2:2+cb]...);entry:=docformat.DecodeCHPX(sprms,nil);return entry.Format,sprms,consumed
}
