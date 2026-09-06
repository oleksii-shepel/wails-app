package cfb

import (
	"bytes"
	"fmt"
	"io"
)

// StreamReaderAt exposes a named CFB stream as logical random-access storage
// without materializing regular-stream payload bytes. The returned size is the
// declared logical stream size. Mini streams remain backed by the already
// materialized root mini-stream container.
func (f *File) StreamReaderAt(name string) (io.ReaderAt, int64, bool, error) {
	if f == nil || len(f.entries) == 0 {
		return nil, 0, false, nil
	}
	e := f.streamEntryByName(name)
	if e == nil || e.objType != objStream {
		return nil, 0, false, nil
	}
	if e.size > uint64(^uint64(0)>>1) {
		return nil, 0, true, fmt.Errorf("cfb: stream %q is too large", e.name)
	}
	if e.size == 0 {
		return bytes.NewReader(nil), 0, true, nil
	}
	if e.size < uint64(f.miniStreamCutoff) {
		data, err := f.readMiniStream(e)
		if err != nil {
			return nil, 0, true, err
		}
		return bytes.NewReader(data), int64(len(data)), true, nil
	}
	chain, err := f.regularStreamChain(e)
	if err != nil {
		return nil, 0, true, err
	}
	return &regularStreamReaderAt{file: f, size: int64(e.size), sectors: chain}, int64(e.size), true, nil
}

func (f *File) regularStreamChain(e *entry) ([]uint32, error) {
	if f == nil || e == nil {
		return nil, fmt.Errorf("cfb: invalid regular stream")
	}
	if e.startSect == secEnd || e.startSect == secFree || e.size == 0 {
		return nil, nil
	}
	need := int((e.size + uint64(f.sectorSize) - 1) / uint64(f.sectorSize))
	chain := make([]uint32, 0, need)
	seen := make(map[uint32]bool, need)
	for sect := e.startSect; sect != secEnd && sect != secFree && len(chain) < need; {
		if int(sect) >= len(f.fat) {
			return nil, fmt.Errorf("cfb: stream %q FAT sector %d out of range", e.name, sect)
		}
		if seen[sect] {
			return nil, fmt.Errorf("cfb: stream %q FAT cycle at sector %d", e.name, sect)
		}
		seen[sect] = true
		chain = append(chain, sect)
		sect = f.fat[sect]
	}
	if len(chain) < need {
		return nil, fmt.Errorf("cfb: stream %q: expected %d sectors, got %d", e.name, need, len(chain))
	}
	return chain, nil
}

type regularStreamReaderAt struct {
	file    *File
	size    int64
	sectors []uint32
}

func (r *regularStreamReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r == nil || r.file == nil || off < 0 {
		return 0, fmt.Errorf("cfb: invalid stream read offset %d", off)
	}
	if off >= r.size {
		return 0, io.EOF
	}
	want := len(p)
	if remaining := r.size - off; int64(want) > remaining {
		want = int(remaining)
	}
	written := 0
	for written < want {
		logical := off + int64(written)
		sectorIndex := int(logical / int64(r.file.sectorSize))
		sectorOffset := int(logical % int64(r.file.sectorSize))
		if sectorIndex < 0 || sectorIndex >= len(r.sectors) {
			break
		}
		sector := r.file.readSector(r.sectors[sectorIndex])
		if sector == nil {
			break
		}
		n := want - written
		if max := len(sector) - sectorOffset; n > max {
			n = max
		}
		copy(p[written:written+n], sector[sectorOffset:sectorOffset+n])
		written += n
	}
	if written < len(p) {
		return written, io.EOF
	}
	return written, nil
}

func (f *File) readSector(n uint32) []byte {
	if f == nil {
		return nil
	}
	if f.reader != nil {
		return sectorReadAt(f.reader, f.readerSize, f.sectorSize, n)
	}
	return sectorSlice(f.raw, f.sectorSize, n)
}
