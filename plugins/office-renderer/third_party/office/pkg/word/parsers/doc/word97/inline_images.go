package word97

import (
	"bytes"
	"encoding/binary"

	"office/pkg/ooxml/cfb"
	"office/pkg/ooxml/units"
	wordir "office/pkg/word/ir"
)

const inlineImageDefaultDPI = 96.0

// ExtractInlineImages discovers bitmap payloads from the Word97 Data/DataStream
// and creates canonical image nodes with intrinsic dimensions. PICF/OfficeArt
// display transforms are intentionally layered later by drawing projection.
func ExtractInlineImages(f *cfb.File) []*wordir.Image {
	if f == nil { return nil }
	data, ok := f.Stream("Data")
	if !ok || len(data) == 0 { data, ok = f.Stream("DataStream") }
	if !ok || len(data) == 0 { return nil }
	blobs := scanImageBlobs(data)
	if len(blobs) == 0 { return nil }
	out := make([]*wordir.Image, 0, len(blobs))
	for _, blob := range blobs {
		mime := detectImageMIME(blob)
		if mime == "" { continue }
		copied := append([]byte(nil), blob...)
		img := &wordir.Image{MIMEType:mime, Data:copied}
		w,h := detectImageDimensions(blob,mime)
		if w>0 && h>0 {
			img.Width = units.InchesToEMU(float64(w)/inlineImageDefaultDPI)
			img.Height = units.InchesToEMU(float64(h)/inlineImageDefaultDPI)
		}
		out=append(out,img)
	}
	if len(out)==0{return nil}
	return out
}

// LoadInlineImages returns fully projected Word97 inline pictures: intrinsic
// bitmap media plus Word-specific PICF sizing and shared OfficeArt visuals.
func LoadInlineImages(f *cfb.File) []*wordir.Image {
	images := ExtractInlineImages(f)
	if len(images) == 0 || f == nil {
		return images
	}
	data, ok := f.Stream("Data")
	if !ok || len(data) == 0 {
		data, ok = f.Stream("DataStream")
	}
	if !ok || len(data) == 0 {
		return images
	}
	descriptors := ScanInlinePictureDescriptors(data)
	for i := 0; i < len(images) && i < len(descriptors); i++ {
		ApplyInlinePictureDescriptor(images[i], descriptors[i])
	}
	return images
}

func scanImageBlobs(data []byte) [][]byte {
	var out [][]byte
	for i:=0;i<len(data);i++ {
		switch {
		case i+8<=len(data)&&bytes.Equal(data[i:i+8],[]byte{0x89,'P','N','G','\r','\n',0x1A,'\n'}):
			if end,ok:=scanPNGEndCanonical(data,i);ok{out=append(out,data[i:end]);i=end-1}
		case i+2<=len(data)&&data[i]==0xFF&&data[i+1]==0xD8:
			if end,ok:=scanJPEGEndCanonical(data,i);ok{out=append(out,data[i:end]);i=end-1}
		case i+6<=len(data)&&(bytes.Equal(data[i:i+6],[]byte("GIF87a"))||bytes.Equal(data[i:i+6],[]byte("GIF89a"))):
			if end,ok:=scanGIFEndCanonical(data,i);ok{out=append(out,data[i:end]);i=end-1}
		case i+6<=len(data)&&data[i]=='B'&&data[i+1]=='M':
			sz:=int(binary.LittleEndian.Uint32(data[i+2:i+6]));if sz>14&&i+sz<=len(data){out=append(out,data[i:i+sz]);i+=sz-1}
		}
	}
	return out
}

func scanPNGEndCanonical(data []byte,start int)(int,bool){off:=start+8;for off+12<=len(data){n:=int(binary.BigEndian.Uint32(data[off:off+4]));if n<0||off+12+n>len(data){return 0,false};typ:=data[off+4:off+8];off+=8+n+4;if bytes.Equal(typ,[]byte("IEND")){return off,true}};return 0,false}
func scanJPEGEndCanonical(data []byte,start int)(int,bool){for i:=start+2;i+1<len(data);i++{if data[i]==0xFF&&data[i+1]==0xD9{return i+2,true}};return 0,false}
func scanGIFEndCanonical(data []byte,start int)(int,bool){for i:=start+6;i<len(data);i++{if data[i]==0x3B{return i+1,true}};return 0,false}

func detectImageMIME(data []byte)string{
	switch{
	case len(data)>=8&&bytes.Equal(data[:8],[]byte{0x89,'P','N','G','\r','\n',0x1A,'\n'}):return "image/png"
	case len(data)>=2&&data[0]==0xFF&&data[1]==0xD8:return "image/jpeg"
	case len(data)>=6&&(bytes.Equal(data[:6],[]byte("GIF87a"))||bytes.Equal(data[:6],[]byte("GIF89a"))):return "image/gif"
	case len(data)>=2&&data[0]=='B'&&data[1]=='M':return "image/bmp"
	default:return ""
	}
}

func detectImageDimensions(data []byte,mime string)(int,int){switch mime{case "image/png":return pngDims(data);case "image/jpeg":return jpegDims(data);case "image/gif":return gifDims(data);case "image/bmp":return bmpDims(data);default:return 0,0}}
func pngDims(data []byte)(int,int){if len(data)<24||!bytes.Equal(data[:8],[]byte{0x89,'P','N','G','\r','\n',0x1A,'\n'})||!bytes.Equal(data[12:16],[]byte("IHDR")){return 0,0};w:=int(binary.BigEndian.Uint32(data[16:20]));h:=int(binary.BigEndian.Uint32(data[20:24]));if w<=0||h<=0{return 0,0};return w,h}
func gifDims(data []byte)(int,int){if len(data)<10||!(bytes.Equal(data[:6],[]byte("GIF87a"))||bytes.Equal(data[:6],[]byte("GIF89a"))){return 0,0};w:=int(binary.LittleEndian.Uint16(data[6:8]));h:=int(binary.LittleEndian.Uint16(data[8:10]));if w<=0||h<=0{return 0,0};return w,h}
func bmpDims(data []byte)(int,int){if len(data)<26||data[0]!='B'||data[1]!='M'{return 0,0};w:=int(int32(binary.LittleEndian.Uint32(data[18:22])));h:=int(int32(binary.LittleEndian.Uint32(data[22:26])));if h<0{h=-h};if w<=0||h<=0{return 0,0};return w,h}
func jpegDims(data []byte)(int,int){if len(data)<4||data[0]!=0xFF||data[1]!=0xD8{return 0,0};i:=2;for i+3<len(data){if data[i]!=0xFF{i++;continue};for i<len(data)&&data[i]==0xFF{i++};if i>=len(data){break};marker:=data[i];i++;if marker==0xD8||marker==0xD9{continue};if i+1>=len(data){break};seg:=int(binary.BigEndian.Uint16(data[i:i+2]));if seg<2||i+seg>len(data){break};if jpegSOF(marker){if seg<7{return 0,0};h:=int(binary.BigEndian.Uint16(data[i+3:i+5]));w:=int(binary.BigEndian.Uint16(data[i+5:i+7]));if w<=0||h<=0{return 0,0};return w,h};i+=seg};return 0,0}
func jpegSOF(marker byte)bool{switch marker{case 0xC0,0xC1,0xC2,0xC3,0xC5,0xC6,0xC7,0xC9,0xCA,0xCB,0xCD,0xCE,0xCF:return true;default:return false}}
