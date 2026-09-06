package cfb

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// OpenFileBacked parses a CFB archive without reading the whole file into memory.
// The returned File owns the underlying os.File and must be closed by the caller.
func OpenFileBacked(path string) (*File, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := fh.Stat()
	if err != nil {
		_ = fh.Close()
		return nil, err
	}
	f, err := OpenReaderAt(fh, info.Size())
	if err != nil {
		_ = fh.Close()
		return nil, err
	}
	f.closer = fh
	return f, nil
}

// OpenReaderAt parses a CFB archive from random-access storage. Only the header,
// FAT/DIFAT, directory, mini-FAT, and root mini-stream are materialized; regular
// streams are fetched from the ReaderAt on demand.
func OpenReaderAt(r io.ReaderAt, size int64) (*File, error) {
	if r == nil {
		return nil, fmt.Errorf("cfb: nil ReaderAt")
	}
	if size < headerSize {
		return nil, fmt.Errorf("cfb: file too small (%d bytes)", size)
	}
	header := make([]byte, headerSize)
	if _, err := r.ReadAt(header, 0); err != nil {
		return nil, fmt.Errorf("cfb: read header: %w", err)
	}
	for i, b := range cfbMagic {
		if header[i] != b {
			return nil, fmt.Errorf("cfb: invalid magic bytes at offset %d", i)
		}
	}
	if bo := u16(header, 28); bo != 0xFFFE && bo != 0xFF20 {
		return nil, fmt.Errorf("cfb: unexpected byte-order mark 0x%04X", bo)
	}

	f := &File{reader: r, readerSize: size}
	f.sectorSize = 1 << u16(header, 30)
	f.miniSectorSize = 1 << u16(header, 32)
	if f.sectorSize < 512 {
		return nil, fmt.Errorf("cfb: sector size %d is too small", f.sectorSize)
	}

	fatCount := int(u32(header, 44))
	rootDirSector := u32(header, 48)
	f.miniStreamCutoff = u32(header, 56)
	if f.miniStreamCutoff == 0 {
		f.miniStreamCutoff = 0x1000
	}
	miniFATFirst := u32(header, 60)
	miniFATCount := int(u32(header, 64))
	difatFirst := u32(header, 68)
	difatCount := int(u32(header, 72))

	difat := make([]uint32, 0, maxDIFATInHeader)
	for i := 0; i < maxDIFATInHeader; i++ {
		v := u32(header, 76+i*4)
		if v == secFree || v == secEnd {
			break
		}
		difat = append(difat, v)
	}
	if difatCount > 0 && difatFirst != secEnd && difatFirst != secFree {
		sect := difatFirst
		for count := 0; sect != secEnd && sect != secFree && count < difatCount; count++ {
			sd := sectorReadAt(r, size, f.sectorSize, sect)
			if sd == nil {
				break
			}
			n := f.sectorSize/4 - 1
			for i := 0; i < n; i++ {
				v := binary.LittleEndian.Uint32(sd[i*4:])
				if v == secFree || v == secEnd {
					break
				}
				difat = append(difat, v)
			}
			sect = binary.LittleEndian.Uint32(sd[f.sectorSize-4:])
		}
	}

	epsz := f.sectorSize / 4
	f.fat = make([]uint32, fatCount*epsz)
	for idx, fatSect := range difat {
		if idx >= fatCount {
			break
		}
		sd := sectorReadAt(r, size, f.sectorSize, fatSect)
		if sd == nil {
			continue
		}
		base := idx * epsz
		for i := 0; i < epsz && base+i < len(f.fat); i++ {
			f.fat[base+i] = binary.LittleEndian.Uint32(sd[i*4:])
		}
	}

	dirData := chainReadReader(r, size, f.sectorSize, f.fat, rootDirSector)
	count := len(dirData) / dirEntrySize
	f.entries = make([]*entry, count)
	for i := 0; i < count; i++ {
		f.entries[i] = parseDirEntry(dirData[i*dirEntrySize : (i+1)*dirEntrySize])
	}
	if count == 0 || f.entries[0] == nil {
		return nil, fmt.Errorf("cfb: no directory entries found")
	}

	if miniFATCount > 0 && miniFATFirst != secEnd && miniFATFirst != secFree {
		mfd := chainReadReader(r, size, f.sectorSize, f.fat, miniFATFirst)
		f.miniFAT = make([]uint32, len(mfd)/4)
		for i := range f.miniFAT {
			f.miniFAT[i] = binary.LittleEndian.Uint32(mfd[i*4:])
		}
	}

	root := f.entries[0]
	if root.startSect != secEnd && root.startSect != secFree && root.size > 0 {
		f.rootData = chainReadReader(r, size, f.sectorSize, f.fat, root.startSect)
		if uint64(len(f.rootData)) > root.size {
			f.rootData = f.rootData[:root.size]
		}
	}
	return f, nil
}

func sectorReadAt(r io.ReaderAt, size int64, sectorSize int, n uint32) []byte {
	off := int64(sectorSize) + int64(n)*int64(sectorSize)
	if off < 0 || off+int64(sectorSize) > size {
		return nil
	}
	buf := make([]byte, sectorSize)
	if _, err := r.ReadAt(buf, off); err != nil {
		return nil
	}
	return buf
}

func chainReadReader(r io.ReaderAt, size int64, sectorSize int, fat []uint32, startSect uint32) []byte {
	data, _ := chainReadReaderWithLastSector(r, size, sectorSize, fat, startSect)
	return data
}

func chainReadReaderWithLastSector(r io.ReaderAt, size int64, sectorSize int, fat []uint32, startSect uint32) ([]byte, uint32) {
	var buf []byte
	seen := make(map[uint32]bool)
	last := secFree
	for sect := startSect; sect != secEnd && sect != secFree; {
		if int(sect) >= len(fat) || seen[sect] {
			break
		}
		seen[sect] = true
		s := sectorReadAt(r, size, sectorSize, sect)
		if s == nil {
			break
		}
		buf = append(buf, s...)
		last = sect
		sect = fat[sect]
	}
	return buf, last
}

func contiguousSectorReadReader(r io.ReaderAt, size int64, sectorSize int, startSect uint32, want int) []byte {
	if want <= 0 {
		return nil
	}
	var buf []byte
	for sect := startSect; len(buf) < want; sect++ {
		s := sectorReadAt(r, size, sectorSize, sect)
		if s == nil {
			break
		}
		buf = append(buf, s...)
	}
	if len(buf) > want {
		buf = buf[:want]
	}
	return buf
}
