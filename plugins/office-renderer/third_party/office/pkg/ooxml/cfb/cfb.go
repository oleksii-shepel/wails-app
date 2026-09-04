// Package cfb parses OLE2 Compound File Binary (CFB) archives as specified
// in [MS-CFB].  The CFB container is the outer packaging used by the legacy
// binary Office formats: .doc (MS-DOC), .xls (BIFF8), and .ppt (MS-PPT).
//
// Usage:
//
//	f, err := cfb.Open("document.doc")
//	data, ok := f.Stream("WordDocument")
package cfb

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
)

// cfbMagic is the 8-byte signature at offset 0 of every CFB file.
var cfbMagic = [8]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// Sector index sentinel values defined in [MS-CFB] §2.2.
const (
	secFree    uint32 = 0xFFFFFFFF // free / unallocated sector
	secEnd     uint32 = 0xFFFFFFFE // end-of-chain marker
	secFAT     uint32 = 0xFFFFFFFD // FAT sector (not data)
	secDIFAT   uint32 = 0xFFFFFFFC // DIFAT sector (not data)
	secNoEntry uint32 = 0xFFFFFFFF // absent directory link
)

// Directory entry object types [MS-CFB] §2.6.1.
const (
	objEmpty   = 0
	objStorage = 1
	objStream  = 2
	objRoot    = 5
)

// headerSize is the fixed size of the CFB file header in bytes.
const headerSize = 512

// dirEntrySize is the size of one directory entry in bytes.
const dirEntrySize = 128

// maxDIFATInHeader is the number of FAT sector locations stored directly in
// the header (the initial DIFAT array).
const maxDIFATInHeader = 109

// entry is a single directory entry parsed from the CFB directory stream.
type entry struct {
	name      string
	objType   byte
	child     uint32 // dirID of first child (for storages)
	left      uint32 // dirID of left sibling in the RB-tree
	right     uint32 // dirID of right sibling in the RB-tree
	startSect uint32 // first sector (or mini-sector) of the stream
	size      uint64 // byte length of the stream
}

// File is a parsed Compound File Binary archive.
// Use Stream to access named streams embedded in the archive.
type File struct {
	raw              []byte   // entire file content, kept for sector reads
	sectorSize       int      // 512 (v3) or 4096 (v4)
	miniSectorSize   int      // always 64
	miniStreamCutoff uint32   // streams smaller than this use the mini stream
	fat              []uint32 // main FAT: fat[sectorN] = next sector
	miniFAT          []uint32 // mini FAT: miniFAT[miniSectorN] = next mini-sector
	rootData         []byte   // content of the root entry stream (mini-stream container)
	entries          []*entry // flat slice of all directory entries; index 0 = root
}

// Open reads a CFB archive from disk.
func Open(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return OpenBytes(data)
}

// OpenBytes parses a CFB archive from an in-memory byte slice.
// The slice is retained by the returned File; do not modify it.
func OpenBytes(data []byte) (*File, error) {
	return parse(data)
}

// SectorSize returns the sector size of the CFB archive (512 for v3 or 4096
// for v4).
func (f *File) SectorSize() int {
	return f.sectorSize
}

// Stream returns the raw content of the named top-level stream.
// The lookup is case-insensitive.  Returns (nil, false) when the stream does
// not exist in the archive.
func (f *File) Stream(name string) ([]byte, bool) {
	data, ok, _ := f.StreamWithError(name)
	return data, ok
}

// StreamWithError returns the raw content of the named top-level stream.
// The lookup is case-insensitive. Returns (nil, false, nil) when the stream
// does not exist. If the stream exists but cannot be read, ok is true and err
// contains the read failure.
func (f *File) StreamWithError(name string) ([]byte, bool, error) {
	if f == nil || len(f.entries) == 0 {
		return nil, false, nil
	}
	e := f.streamEntryByName(name)
	if e == nil || e.objType != objStream {
		return nil, false, nil
	}
	data, err := f.readStream(e)
	if err != nil {
		return nil, true, err
	}
	return data, true, nil
}

// StreamBestEffort returns the raw content of the named top-level stream,
// tolerating size mismatches from broken directory metadata.
//
// When the stream exists but its declared size exceeds chain-readable bytes,
// this method returns the readable bytes with a non-nil warning error.
func (f *File) StreamBestEffort(name string) ([]byte, bool, error) {
	if f == nil || len(f.entries) == 0 {
		return nil, false, nil
	}
	e := f.streamEntryByName(name)
	if e == nil || e.objType != objStream {
		return nil, false, nil
	}
	data, warn := f.readStreamBestEffort(e)
	return data, true, warn
}

func (f *File) streamEntryByName(name string) *entry {
	lower := strings.ToLower(name)
	// Entry 0 is root; its child is the root storage's first child.
	e := f.findEntry(f.entries[0].child, lower)
	if e == nil || e.objType != objStream {
		e = f.findEntryLinear(lower)
	}
	return e
}

// StreamAt returns the content of the stream at the given slash-separated
// path. Each component is matched case-insensitively against its parent
// storage's children. Returns (nil, false) if any component is missing or the
// final component is not a stream.
func (f *File) StreamAt(path string) ([]byte, bool) {
	if f == nil || len(f.entries) == 0 {
		return nil, false
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return nil, false
	}
	// Walk from the root storage's first child down through nested storages.
	dirID := f.entries[0].child
	var leaf *entry
	for i, name := range parts {
		e := f.findEntry(dirID, strings.ToLower(name))
		if e == nil {
			return nil, false
		}
		if i == len(parts)-1 {
			leaf = e
			break
		}
		if e.objType != objStorage {
			return nil, false
		}
		dirID = e.child
	}
	if leaf == nil || leaf.objType != objStream {
		return nil, false
	}
	data, err := f.readStream(leaf)
	if err != nil {
		return nil, false
	}
	return data, true
}

func (f *File) findEntryLinear(lower string) *entry {
	for _, e := range f.entries {
		if e == nil || e.objType != objStream {
			continue
		}
		if strings.EqualFold(e.name, lower) {
			return e
		}
	}
	return nil
}

// StreamNames returns the names of all direct-child streams in the root
// storage, in tree-traversal order.
func (f *File) StreamNames() []string {
	if f == nil || len(f.entries) == 0 {
		return nil
	}
	var names []string
	f.walkNames(f.entries[0].child, false, &names)
	return names
}

// StorageNames returns the names of all direct-child storages in the root
// storage, in tree-traversal order.
func (f *File) StorageNames() []string {
	if f == nil || len(f.entries) == 0 {
		return nil
	}
	var names []string
	f.walkNames(f.entries[0].child, true, &names)
	return names
}

// walkNames does an in-order walk of the RB-tree rooted at dirID, appending
// stream (wantStorage=false) or storage (wantStorage=true) names.
func (f *File) walkNames(dirID uint32, wantStorage bool, names *[]string) {
	if dirID == secNoEntry || int(dirID) >= len(f.entries) {
		return
	}
	e := f.entries[dirID]
	if e == nil || e.objType == objEmpty {
		return
	}
	f.walkNames(e.left, wantStorage, names)
	if wantStorage && e.objType == objStorage {
		*names = append(*names, e.name)
	} else if !wantStorage && e.objType == objStream {
		*names = append(*names, e.name)
	}
	f.walkNames(e.right, wantStorage, names)
}

// findEntry searches the RB-tree rooted at dirID for an entry with the given
// lowercased name.  CFB ordering: shorter names sort before longer ones, then
// case-insensitive lexicographic order.
func (f *File) findEntry(dirID uint32, lower string) *entry {
	if dirID == secNoEntry || int(dirID) >= len(f.entries) {
		return nil
	}
	e := f.entries[dirID]
	if e == nil || e.objType == objEmpty {
		return nil
	}
	eLow := strings.ToLower(e.name)
	switch cfbCmp(lower, eLow) {
	case -1:
		return f.findEntry(e.left, lower)
	case 1:
		return f.findEntry(e.right, lower)
	default:
		return e
	}
}

// cfbCmp implements the CFB directory ordering: length first, then
// case-insensitive lexicographic.
func cfbCmp(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// readStream reads the full byte content of a directory entry using either
// the regular FAT chain or the mini-stream FAT chain.
func (f *File) readStream(e *entry) ([]byte, error) {
	if e.size == 0 || e.startSect == secEnd || e.startSect == secFree {
		return nil, nil
	}
	if e.objType != objRoot && e.size < uint64(f.miniStreamCutoff) {
		return f.readMiniStream(e)
	}
	return f.readRegularStream(e)
}

// readStreamBestEffort mirrors readStream but allows truncated reads when
// stream size metadata is inconsistent with the FAT chain.
func (f *File) readStreamBestEffort(e *entry) ([]byte, error) {
	if e.size == 0 || e.startSect == secEnd || e.startSect == secFree {
		return nil, nil
	}
	if e.objType != objRoot && e.size < uint64(f.miniStreamCutoff) {
		if f.miniFAT == nil || f.rootData == nil {
			return nil, fmt.Errorf("cfb: mini stream not available for %q", e.name)
		}
		data := miniChainRead(f.rootData, f.miniSectorSize, f.miniFAT, e.startSect)
		if uint64(len(data)) >= e.size {
			return data[:e.size], nil
		}
		return data, fmt.Errorf("cfb: mini stream %q: expected %d bytes, got %d", e.name, e.size, len(data))
	}

	data, lastSect := chainReadWithLastSector(f.raw, f.sectorSize, f.fat, e.startSect)
	if uint64(len(data)) >= e.size {
		return data[:e.size], nil
	}
	if lastSect != secFree {
		data = append(data, contiguousSectorRead(f.raw, f.sectorSize, lastSect+1, int(e.size)-len(data))...)
		if uint64(len(data)) >= e.size {
			return data[:e.size], fmt.Errorf("cfb: stream %q: FAT chain truncated, recovered %d contiguous bytes", e.name, e.size)
		}
	}
	return data, fmt.Errorf("cfb: stream %q: expected %d bytes, got %d", e.name, e.size, len(data))
}

// readRegularStream reads a stream from the main FAT chain.
func (f *File) readRegularStream(e *entry) ([]byte, error) {
	data := chainRead(f.raw, f.sectorSize, f.fat, e.startSect)
	if uint64(len(data)) < e.size {
		return nil, fmt.Errorf("cfb: stream %q: expected %d bytes, got %d", e.name, e.size, len(data))
	}
	return data[:e.size], nil
}

// readMiniStream reads a stream from the mini FAT.
func (f *File) readMiniStream(e *entry) ([]byte, error) {
	if f.miniFAT == nil || f.rootData == nil {
		return nil, fmt.Errorf("cfb: mini stream not available for %q", e.name)
	}
	data := miniChainRead(f.rootData, f.miniSectorSize, f.miniFAT, e.startSect)
	if uint64(len(data)) < e.size {
		return nil, fmt.Errorf("cfb: mini stream %q: expected %d bytes, got %d", e.name, e.size, len(data))
	}
	return data[:e.size], nil
}

// ─── core parser ────────────────────────────────────────────────────────────

func parse(raw []byte) (*File, error) {
	if len(raw) < headerSize {
		return nil, fmt.Errorf("cfb: file too small (%d bytes)", len(raw))
	}
	// Magic check.
	for i, b := range cfbMagic {
		if raw[i] != b {
			return nil, fmt.Errorf("cfb: invalid magic bytes at offset %d", i)
		}
	}
	// Most CFB files use the required 0xFFFE value, but some LibreOffice bug
	// fixtures stamp 0xFF20 while remaining otherwise readable as little-endian.
	if bo := u16(raw, 28); bo != 0xFFFE && bo != 0xFF20 {
		return nil, fmt.Errorf("cfb: unexpected byte-order mark 0x%04X", bo)
	}

	f := &File{raw: raw}
	f.sectorSize = 1 << u16(raw, 30)     // 0x0009 → 512, 0x000C → 4096
	f.miniSectorSize = 1 << u16(raw, 32) // always 0x0006 → 64

	if f.sectorSize < 512 {
		return nil, fmt.Errorf("cfb: sector size %d is too small", f.sectorSize)
	}

	fatCount := int(u32(raw, 44))     // total FAT sectors
	rootDirSector := u32(raw, 48)     // first directory sector
	f.miniStreamCutoff = u32(raw, 56) // mini stream cutoff (default 0x1000)
	if f.miniStreamCutoff == 0 {
		f.miniStreamCutoff = 0x1000
	}
	miniFATFirst := u32(raw, 60)      // first mini FAT sector
	miniFATCount := int(u32(raw, 64)) // total mini FAT sectors
	difatFirst := u32(raw, 68)        // first DIFAT continuation sector
	difatCount := int(u32(raw, 72))   // total DIFAT continuation sectors

	// ── collect DIFAT (list of FAT sector numbers) ──────────────────────────
	difat := make([]uint32, 0, maxDIFATInHeader)
	for i := 0; i < maxDIFATInHeader; i++ {
		v := u32(raw, 76+i*4)
		if v == secFree || v == secEnd {
			break
		}
		difat = append(difat, v)
	}
	// Follow DIFAT continuation sectors.
	if difatCount > 0 && difatFirst != secEnd && difatFirst != secFree {
		sect := difatFirst
		for sect != secEnd && sect != secFree {
			sd := sectorSlice(raw, f.sectorSize, sect)
			if sd == nil {
				break
			}
			count := f.sectorSize/4 - 1 // last slot is the next DIFAT sector link
			for i := 0; i < count; i++ {
				v := binary.LittleEndian.Uint32(sd[i*4:])
				if v == secFree || v == secEnd {
					break
				}
				difat = append(difat, v)
			}
			sect = binary.LittleEndian.Uint32(sd[f.sectorSize-4:])
		}
	}

	// ── build FAT from DIFAT sectors ─────────────────────────────────────────
	epsz := f.sectorSize / 4 // FAT entries per sector
	f.fat = make([]uint32, fatCount*epsz)
	for idx, fatSect := range difat {
		sd := sectorSlice(raw, f.sectorSize, fatSect)
		if sd == nil {
			continue
		}
		base := idx * epsz
		for i := 0; i < epsz && base+i < len(f.fat); i++ {
			f.fat[base+i] = binary.LittleEndian.Uint32(sd[i*4:])
		}
	}

	// ── read directory entries ─────────────────────────────────────────────
	dirData := chainRead(raw, f.sectorSize, f.fat, rootDirSector)
	count := len(dirData) / dirEntrySize
	f.entries = make([]*entry, count)
	for i := 0; i < count; i++ {
		f.entries[i] = parseDirEntry(dirData[i*dirEntrySize : (i+1)*dirEntrySize])
	}
	if count == 0 || f.entries[0] == nil {
		return nil, fmt.Errorf("cfb: no directory entries found")
	}

	// ── build mini FAT ────────────────────────────────────────────────────
	if miniFATCount > 0 && miniFATFirst != secEnd && miniFATFirst != secFree {
		mfd := chainRead(raw, f.sectorSize, f.fat, miniFATFirst)
		f.miniFAT = make([]uint32, len(mfd)/4)
		for i := range f.miniFAT {
			f.miniFAT[i] = binary.LittleEndian.Uint32(mfd[i*4:])
		}
	}

	// ── read root stream (mini-stream container) ──────────────────────────
	root := f.entries[0]
	if root.startSect != secEnd && root.startSect != secFree && root.size > 0 {
		f.rootData = chainRead(raw, f.sectorSize, f.fat, root.startSect)
		if uint64(len(f.rootData)) > root.size {
			f.rootData = f.rootData[:root.size]
		}
	}

	return f, nil
}

// parseDirEntry decodes one 128-byte directory entry.
func parseDirEntry(d []byte) *entry {
	// Name: 64 bytes UTF-16LE, length (including null) in bytes at offset 64.
	nameBytes := int(u16(d, 64))
	if nameBytes > 64 {
		nameBytes = 64
	}
	var name string
	if nameBytes >= 2 {
		nchars := nameBytes/2 - 1 // exclude null terminator
		if nchars > 0 {
			u := make([]uint16, nchars)
			for i := range u {
				u[i] = binary.LittleEndian.Uint16(d[i*2:])
			}
			name = string(utf16.Decode(u))
		}
	}

	// High 32 bits of size (v4 only); for v3 they must be 0.
	sizeLo := uint64(u32(d, 120))
	sizeHi := uint64(u32(d, 124))
	size := (sizeHi << 32) | sizeLo

	return &entry{
		name:      name,
		objType:   d[66],
		left:      u32(d, 68),
		right:     u32(d, 72),
		child:     u32(d, 76),
		startSect: u32(d, 116),
		size:      size,
	}
}

// ─── sector / chain helpers ──────────────────────────────────────────────────

// sectorSlice returns a slice of raw covering sector n.
// Returns nil if the sector is out of range.
//
// Per [MS-CFB] §2.2 the header occupies the first sector of the file, so data
// sector 0 starts at offset sectorSize (= 512 for v3, 4096 for v4). The
// 512-byte header structure is followed by 3584 reserved bytes in v4.
func sectorSlice(raw []byte, sectorSize int, n uint32) []byte {
	off := sectorSize + int(n)*sectorSize
	if off < 0 || off+sectorSize > len(raw) {
		return nil
	}
	return raw[off : off+sectorSize]
}

// chainRead follows the FAT chain starting at startSect and returns the
// concatenated sector data.
func chainRead(raw []byte, sectorSize int, fat []uint32, startSect uint32) []byte {
	data, _ := chainReadWithLastSector(raw, sectorSize, fat, startSect)
	return data
}

func chainReadWithLastSector(raw []byte, sectorSize int, fat []uint32, startSect uint32) ([]byte, uint32) {
	var buf []byte
	seen := make(map[uint32]bool) // cycle guard
	last := secFree
	for sect := startSect; sect != secEnd && sect != secFree; {
		if int(sect) >= len(fat) || seen[sect] {
			break
		}
		seen[sect] = true
		s := sectorSlice(raw, sectorSize, sect)
		if s == nil {
			break
		}
		buf = append(buf, s...)
		last = sect
		sect = fat[sect]
	}
	return buf, last
}

// miniChainRead follows the mini FAT chain, reading miniSectorSize chunks from
// rootData (the root-entry stream that holds the mini-stream).
func miniChainRead(rootData []byte, miniSectorSize int, miniFAT []uint32, startSect uint32) []byte {
	var buf []byte
	seen := make(map[uint32]bool)
	for sect := startSect; sect != secEnd && sect != secFree; {
		if int(sect) >= len(miniFAT) || seen[sect] {
			break
		}
		seen[sect] = true
		off := int(sect) * miniSectorSize
		if off+miniSectorSize > len(rootData) {
			break
		}
		buf = append(buf, rootData[off:off+miniSectorSize]...)
		sect = miniFAT[sect]
	}
	return buf
}

func contiguousSectorRead(raw []byte, sectorSize int, startSect uint32, want int) []byte {
	if want <= 0 {
		return nil
	}
	var buf []byte
	for sect := startSect; len(buf) < want; sect++ {
		s := sectorSlice(raw, sectorSize, sect)
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

// ─── little-endian read helpers ──────────────────────────────────────────────

func u16(b []byte, off int) uint16 {
	return binary.LittleEndian.Uint16(b[off:])
}

func u32(b []byte, off int) uint32 {
	return binary.LittleEndian.Uint32(b[off:])
}
