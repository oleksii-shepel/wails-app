package cfb_test

import (
	"bytes"
	"io"
	"testing"

	"office/pkg/ooxml/cfb"
)

func TestStreamReaderAtReadsAcrossCFBSectors(t *testing.T) {
	payload := make([]byte, 5000)
	for i := range payload {
		payload[i] = byte((i * 17) % 251)
	}
	data := buildMinimalCFB("WordDocument", payload)
	f, err := cfb.OpenReaderAt(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReaderAt: %v", err)
	}
	r, size, ok, err := f.StreamReaderAt("WordDocument")
	if err != nil || !ok {
		t.Fatalf("StreamReaderAt: ok=%v err=%v", ok, err)
	}
	if size != int64(len(payload)) {
		t.Fatalf("size=%d want=%d", size, len(payload))
	}
	buf := make([]byte, 900)
	n, err := r.ReadAt(buf, 400)
	if err != nil || n != len(buf) {
		t.Fatalf("ReadAt: n=%d err=%v", n, err)
	}
	if !bytes.Equal(buf, payload[400:1300]) {
		t.Fatal("cross-sector read mismatch")
	}
}

func TestStreamReaderAtHonorsLogicalEOF(t *testing.T) {
	payload := make([]byte, 4096)
	copy(payload, []byte("logical-eof"))
	data := buildMinimalCFB("WordDocument", payload)
	f, err := cfb.OpenReaderAt(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReaderAt: %v", err)
	}
	r, size, ok, err := f.StreamReaderAt("WordDocument")
	if err != nil || !ok {
		t.Fatalf("StreamReaderAt: ok=%v err=%v", ok, err)
	}
	buf := make([]byte, 16)
	n, err := r.ReadAt(buf, size-4)
	if n != 4 || err != io.EOF {
		t.Fatalf("ReadAt EOF: n=%d err=%v", n, err)
	}
}
