package pre97

import (
	"encoding/binary"
	"testing"
)

func TestPieceCursorReadsSimpleOldDocument(t *testing.T) {
	word := make([]byte, 1024)
	binary.LittleEndian.PutUint16(word[0:2], 0xA5DC)
	binary.LittleEndian.PutUint16(word[2:4], 0x0065)
	binary.LittleEndian.PutUint32(word[24:28], 512)
	binary.LittleEndian.PutUint32(word[28:32], 516)
	binary.LittleEndian.PutUint32(word[52:56], 4)
	copy(word[512:516], []byte("abc\r"))
	core, ok := OpenCore(word)
	if !ok { t.Fatal("OpenCore rejected old document") }
	cursor, err := NewPieceCursor(word, core)
	if err != nil { t.Fatalf("NewPieceCursor: %v", err) }
	var text string
	for {
		r, ok, err := cursor.Next()
		if err != nil { t.Fatalf("Next: %v", err) }
		if !ok { break }
		text += r.Text
	}
	if text != "abc\r" { t.Fatalf("text = %q, want %q", text, "abc\r") }
}
