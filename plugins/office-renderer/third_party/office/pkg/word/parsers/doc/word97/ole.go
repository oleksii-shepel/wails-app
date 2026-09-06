package word97

import (
	"strings"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
)

// LoadOLEObjects discovers embedded OLE/package streams from the compound file.
// Object payload interpretation remains intentionally conservative; the stream
// cursor later binds these objects to Obj-marked CPs in document order.
func LoadOLEObjects(f *cfb.File) []wordir.Run {
	if f==nil{return nil}
	payloads:=make([][]byte,0)
	for _,name:=range f.StreamNames(){
		lower:=strings.ToLower(name)
		if !strings.Contains(lower,"ole10native")&&!strings.Contains(lower,"objectpool")&&!strings.Contains(lower,"package"){continue}
		data,ok:=f.Stream(name);if !ok||len(data)==0{continue}
		payloads=append(payloads,append([]byte(nil),data...))
	}
	out:=make([]wordir.Run,len(payloads))
	for i,payload:=range payloads{out[i]=wordir.Run{Kind:"olePlaceholder",Text:"Embedded Object",OLEData:payload,OLEMIME:"application/octet-stream",OLEType:"Embed"}}
	return out
}
