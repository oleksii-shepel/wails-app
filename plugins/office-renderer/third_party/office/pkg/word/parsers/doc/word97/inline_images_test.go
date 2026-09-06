package word97

import (
	"encoding/binary"
	"testing"
)

func TestCanonicalInlineImageBlobDiscovery(t *testing.T) {
	png:=make([]byte,33)
	copy(png[:8],[]byte{0x89,'P','N','G','\r','\n',0x1A,'\n'})
	binary.BigEndian.PutUint32(png[8:12],13)
	copy(png[12:16],[]byte("IHDR"))
	binary.BigEndian.PutUint32(png[16:20],2)
	binary.BigEndian.PutUint32(png[20:24],3)
	// Remaining IHDR data and CRC bytes are zeroed in this structural fixture.
	png=append(png,0,0,0,0)
	png=append(png,[]byte("IEND")...)
	png=append(png,0,0,0,0)

	blobs:=scanImageBlobs(append([]byte{1,2,3},png...))
	if len(blobs)!=1{t.Fatalf("blobs=%d",len(blobs))}
	if mime:=detectImageMIME(blobs[0]);mime!="image/png"{t.Fatalf("mime=%q",mime)}
	w,h:=detectImageDimensions(blobs[0],"image/png")
	if w!=2||h!=3{t.Fatalf("dims=%dx%d",w,h)}
}
