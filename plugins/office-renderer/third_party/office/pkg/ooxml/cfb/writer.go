package cfb

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"
)

// Writer assembles a Compound File Binary archive in memory.
//
// The default constructor produces v3 archives (512-byte sectors). Use
// NewWriter4K for v4 (4096-byte sectors), which scale beyond 2 GB.
//
// AddStream and AddStorage accept slash-separated paths so nested storage
// hierarchies can be built without explicit storage handles. Names are
// case-insensitively unique within a storage and must fit in 31 UTF-16
// code units.
type Writer struct {
	sectorSize int
	root       *writerNode
}

type writerNode struct {
	name     string
	isStream bool
	data     []byte
	children map[string]*writerNode // by lowercased name
}

func newWriterNode(name string, isStream bool) *writerNode {
	return &writerNode{
		name:     name,
		isStream: isStream,
		children: make(map[string]*writerNode),
	}
}

// NewWriter returns a v3 (512-byte sector) writer.
func NewWriter() *Writer {
	return newWriter(writerSectorSizeV3)
}

// NewWriter4K returns a v4 (4096-byte sector) writer. Use this when the
// archive is expected to exceed roughly 2 GB or when targeting v4-only
// readers; otherwise the v3 default is broadly compatible.
func NewWriter4K() *Writer {
	return newWriter(writerSectorSizeV4)
}

func newWriter(sectorSize int) *Writer {
	return &Writer{
		sectorSize: sectorSize,
		root:       newWriterNode("Root Entry", false),
	}
}

// AddStream creates a stream at the given slash-separated path. Intermediate
// storages are auto-created. The leaf must not already exist (as a stream or
// a storage). A path without slashes places the stream directly under the
// root storage.
func (w *Writer) AddStream(path string, data []byte) error {
	parts, err := splitPath(path)
	if err != nil {
		return err
	}
	parent, err := w.ensureStorage(parts[:len(parts)-1])
	if err != nil {
		return err
	}
	leaf := parts[len(parts)-1]
	if err := validateName(leaf); err != nil {
		return err
	}
	key := strings.ToLower(leaf)
	if _, exists := parent.children[key]; exists {
		return fmt.Errorf("cfb: %q already exists", path)
	}
	n := newWriterNode(leaf, true)
	n.data = append([]byte(nil), data...)
	parent.children[key] = n
	return nil
}

// AddStorage ensures a storage exists at the given slash-separated path,
// creating any intermediate storages along the way. Calling AddStorage on an
// existing storage path is a no-op; calling it on a path that already exists
// as a stream is an error.
func (w *Writer) AddStorage(path string) error {
	parts, err := splitPath(path)
	if err != nil {
		return err
	}
	_, err = w.ensureStorage(parts)
	return err
}

func (w *Writer) ensureStorage(parts []string) (*writerNode, error) {
	cur := w.root
	for _, name := range parts {
		if err := validateName(name); err != nil {
			return nil, err
		}
		key := strings.ToLower(name)
		if existing, ok := cur.children[key]; ok {
			if existing.isStream {
				return nil, fmt.Errorf("cfb: %q is a stream, expected storage", name)
			}
			cur = existing
			continue
		}
		n := newWriterNode(name, false)
		cur.children[key] = n
		cur = n
	}
	return cur, nil
}

func splitPath(path string) ([]string, error) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil, fmt.Errorf("cfb: path must not be empty")
	}
	parts := strings.Split(trimmed, "/")
	for _, c := range parts {
		if c == "" {
			return nil, fmt.Errorf("cfb: path %q has empty component", path)
		}
	}
	return parts, nil
}

func validateName(name string) error {
	u := utf16.Encode([]rune(name))
	if len(u) == 0 {
		return fmt.Errorf("cfb: name must not be empty")
	}
	if len(u) > 31 {
		return fmt.Errorf("cfb: name %q exceeds 31 UTF-16 code units", name)
	}
	for _, c := range u {
		switch c {
		case '/', '\\', ':', '!':
			return fmt.Errorf("cfb: name %q contains illegal character", name)
		}
	}
	return nil
}

// Bytes serialises the archive as a single in-memory CFB file.
func (w *Writer) Bytes() ([]byte, error) {
	return marshalCFB(w)
}

const (
	writerSectorSizeV3 = 512
	writerSectorSizeV4 = 4096
	writerMiniSecSize  = 64
	writerMiniCutoff   = 4096
)

type dirSlot struct {
	name      string
	objType   byte
	left      uint32
	right     uint32
	child     uint32
	startSect uint32
	size      uint64
}

func marshalCFB(w *Writer) ([]byte, error) {
	sectorSize := w.sectorSize
	fatEntriesPerSect := sectorSize / 4
	dirEntriesPerSect := sectorSize / dirEntrySize
	difatSlotsPerSect := fatEntriesPerSect - 1

	// ── Walk the tree, assigning dirIDs in DFS order. Each storage records
	// its sorted children so we can build BST sibling links afterwards. ──
	type walkEntry struct {
		node       *writerNode
		sortedKids []int
	}
	walked := []*walkEntry{}
	var walk func(n *writerNode) int
	walk = func(n *writerNode) int {
		id := len(walked)
		walked = append(walked, &walkEntry{node: n})
		if n.isStream {
			return id
		}
		kids := make([]*writerNode, 0, len(n.children))
		for _, c := range n.children {
			kids = append(kids, c)
		}
		sort.SliceStable(kids, func(i, j int) bool {
			return cfbCmp(strings.ToLower(kids[i].name), strings.ToLower(kids[j].name)) < 0
		})
		ids := make([]int, 0, len(kids))
		for _, k := range kids {
			ids = append(ids, walk(k))
		}
		walked[id].sortedKids = ids
		return id
	}
	walk(w.root)

	// ── Partition stream nodes by mini vs main FAT. ──
	type placement struct {
		dirID     int
		mini      bool
		startMini uint32
		startBig  uint32
	}
	placements := []placement{}
	miniStreamBytes := 0
	largeBigSectors := 0
	for id, e := range walked {
		if !e.node.isStream {
			continue
		}
		size := len(e.node.data)
		switch {
		case size == 0:
			placements = append(placements, placement{dirID: id})
		case size < writerMiniCutoff:
			placements = append(placements, placement{dirID: id, mini: true})
			miniStreamBytes += roundUpInt(size, writerMiniSecSize)
		default:
			placements = append(placements, placement{dirID: id})
			largeBigSectors += (size + sectorSize - 1) / sectorSize
		}
	}

	miniSectorCount := miniStreamBytes / writerMiniSecSize
	miniStreamBigSectors := (miniStreamBytes + sectorSize - 1) / sectorSize
	miniFATSectors := 0
	if miniSectorCount > 0 {
		miniFATSectors = (miniSectorCount + fatEntriesPerSect - 1) / fatEntriesPerSect
	}

	// ── Directory sectors. Round entry count up to a full sector; minimum
	// is one sector even when only the root exists. ──
	dirEntryCount := len(walked)
	if dirEntryCount%dirEntriesPerSect != 0 {
		dirEntryCount = ((dirEntryCount / dirEntriesPerSect) + 1) * dirEntriesPerSect
	}
	if dirEntryCount == 0 {
		dirEntryCount = dirEntriesPerSect
	}
	dirSectors := dirEntryCount / dirEntriesPerSect

	dataSectors := dirSectors + miniFATSectors + miniStreamBigSectors + largeBigSectors

	// Resolve FAT and DIFAT counts at a fixed point. Both are monotonically
	// non-decreasing; the loop terminates within a few iterations.
	nFAT, nDIFAT := 0, 0
	for {
		totalIndexed := dataSectors + nFAT + nDIFAT
		needFAT := (totalIndexed + fatEntriesPerSect - 1) / fatEntriesPerSect
		if needFAT == 0 && totalIndexed > 0 {
			needFAT = 1
		}
		needDIFAT := 0
		if needFAT > maxDIFATInHeader {
			needDIFAT = (needFAT - maxDIFATInHeader + difatSlotsPerSect - 1) / difatSlotsPerSect
		}
		if needFAT == nFAT && needDIFAT == nDIFAT {
			break
		}
		nFAT, nDIFAT = needFAT, needDIFAT
	}

	// ── Sector layout (data sector indices, post-header):
	//   [0 .. nFAT-1]                 FAT sectors
	//   [nFAT .. nFAT+nDIFAT-1]       DIFAT continuation sectors
	//   [base ..]                     Directory, mini-FAT, mini-stream, large streams
	base := nFAT + nDIFAT
	dirStart := base
	miniFATStart := dirStart + dirSectors
	miniStreamStart := miniFATStart + miniFATSectors
	largeStart := miniStreamStart + miniStreamBigSectors

	totalSectors := nFAT + nDIFAT + dataSectors
	// The header occupies the first whole sector regardless of v3/v4.
	file := make([]byte, sectorSize+totalSectors*sectorSize)
	sectorAt := func(s int) int { return sectorSize + s*sectorSize }

	// ── Header. ────────────────────────────────────────────────────────────
	copy(file[0:8], cfbMagic[:])
	binary.LittleEndian.PutUint16(file[24:], 0x003E) // minor version
	if sectorSize == writerSectorSizeV4 {
		binary.LittleEndian.PutUint16(file[26:], 0x0004) // major version (v4)
		binary.LittleEndian.PutUint16(file[30:], 12)     // sector shift → 4096
	} else {
		binary.LittleEndian.PutUint16(file[26:], 0x0003) // major version (v3)
		binary.LittleEndian.PutUint16(file[30:], 9)      // sector shift → 512
	}
	binary.LittleEndian.PutUint16(file[28:], 0xFFFE) // little-endian
	binary.LittleEndian.PutUint16(file[32:], 6)      // mini sector shift → 64
	// reserved [34:40] zero
	if sectorSize == writerSectorSizeV4 {
		binary.LittleEndian.PutUint32(file[40:], uint32(dirSectors))
	} else {
		binary.LittleEndian.PutUint32(file[40:], 0) // v3: must be zero
	}
	binary.LittleEndian.PutUint32(file[44:], uint32(nFAT))
	binary.LittleEndian.PutUint32(file[48:], uint32(dirStart))
	binary.LittleEndian.PutUint32(file[52:], 0) // transaction signature
	binary.LittleEndian.PutUint32(file[56:], writerMiniCutoff)
	if miniFATSectors > 0 {
		binary.LittleEndian.PutUint32(file[60:], uint32(miniFATStart))
		binary.LittleEndian.PutUint32(file[64:], uint32(miniFATSectors))
	} else {
		binary.LittleEndian.PutUint32(file[60:], secEnd)
		binary.LittleEndian.PutUint32(file[64:], 0)
	}
	if nDIFAT > 0 {
		binary.LittleEndian.PutUint32(file[68:], uint32(nFAT))
		binary.LittleEndian.PutUint32(file[72:], uint32(nDIFAT))
	} else {
		binary.LittleEndian.PutUint32(file[68:], secEnd)
		binary.LittleEndian.PutUint32(file[72:], 0)
	}

	// First 109 DIFAT slots in the header.
	headerDIFATCount := nFAT
	if headerDIFATCount > maxDIFATInHeader {
		headerDIFATCount = maxDIFATInHeader
	}
	for i := 0; i < maxDIFATInHeader; i++ {
		v := secFree
		if i < headerDIFATCount {
			v = uint32(i)
		}
		binary.LittleEndian.PutUint32(file[76+i*4:], v)
	}

	// ── In-memory FAT, written into FAT sectors at the end. ──
	fat := make([]uint32, nFAT*fatEntriesPerSect)
	for i := range fat {
		fat[i] = secFree
	}
	setFAT := func(sect int, next uint32) {
		if sect >= 0 && sect < len(fat) {
			fat[sect] = next
		}
	}
	for i := 0; i < nFAT; i++ {
		setFAT(i, secFAT)
	}
	for i := 0; i < nDIFAT; i++ {
		setFAT(nFAT+i, secDIFAT)
	}
	chain := func(start, count int) {
		for i := 0; i < count; i++ {
			sect := start + i
			if i == count-1 {
				setFAT(sect, secEnd)
			} else {
				setFAT(sect, uint32(sect+1))
			}
		}
	}
	chain(dirStart, dirSectors)
	chain(miniFATStart, miniFATSectors)
	chain(miniStreamStart, miniStreamBigSectors)

	// ── Large stream sectors: copy data, mark FAT chains. ──
	largeCursor := largeStart
	for pi, p := range placements {
		if p.mini {
			continue
		}
		data := walked[p.dirID].node.data
		if len(data) == 0 {
			continue
		}
		n := (len(data) + sectorSize - 1) / sectorSize
		placements[pi].startBig = uint32(largeCursor)
		chain(largeCursor, n)
		for j := 0; j < n; j++ {
			off := sectorAt(largeCursor + j)
			lo := j * sectorSize
			hi := lo + sectorSize
			if hi > len(data) {
				hi = len(data)
			}
			copy(file[off:off+(hi-lo)], data[lo:hi])
		}
		largeCursor += n
	}

	// ── Mini stream: copy data into mini-stream container, build mini-FAT. ──
	miniFAT := make([]uint32, miniSectorCount)
	miniCursor := 0
	for pi, p := range placements {
		if !p.mini {
			continue
		}
		data := walked[p.dirID].node.data
		n := (len(data) + writerMiniSecSize - 1) / writerMiniSecSize
		placements[pi].startMini = uint32(miniCursor)
		for j := 0; j < n; j++ {
			ms := miniCursor + j
			if j == n-1 {
				miniFAT[ms] = secEnd
			} else {
				miniFAT[ms] = uint32(ms + 1)
			}
			destOff := sectorAt(miniStreamStart) + ms*writerMiniSecSize
			lo := j * writerMiniSecSize
			hi := lo + writerMiniSecSize
			if hi > len(data) {
				hi = len(data)
			}
			copy(file[destOff:destOff+(hi-lo)], data[lo:hi])
		}
		miniCursor += n
	}

	// ── Mini-FAT entries + free padding. ──
	for s := 0; s < miniFATSectors; s++ {
		secOff := sectorAt(miniFATStart + s)
		for slot := 0; slot < fatEntriesPerSect; slot++ {
			idx := s*fatEntriesPerSect + slot
			v := secFree
			if idx < len(miniFAT) {
				v = miniFAT[idx]
			}
			binary.LittleEndian.PutUint32(file[secOff+slot*4:], v)
		}
	}

	// ── Build directory entries. ──
	slots := make([]dirSlot, dirEntryCount)
	for i := range slots {
		slots[i].objType = objEmpty
		slots[i].left = secNoEntry
		slots[i].right = secNoEntry
		slots[i].child = secNoEntry
	}
	for id, e := range walked {
		objType := byte(objStorage)
		if id == 0 {
			objType = objRoot
		} else if e.node.isStream {
			objType = objStream
		}
		slots[id] = dirSlot{
			name:    e.node.name,
			objType: objType,
			left:    secNoEntry,
			right:   secNoEntry,
			child:   secNoEntry,
		}
	}

	// Stream placements → startSect / size.
	for _, p := range placements {
		data := walked[p.dirID].node.data
		startSect := uint32(secEnd)
		switch {
		case len(data) == 0:
			startSect = secEnd
		case p.mini:
			startSect = p.startMini
		default:
			startSect = p.startBig
		}
		slots[p.dirID].startSect = startSect
		slots[p.dirID].size = uint64(len(data))
	}

	// Storage children → balanced BST sibling links + child pointer.
	var buildBST func(kids []int) uint32
	buildBST = func(kids []int) uint32 {
		if len(kids) == 0 {
			return secNoEntry
		}
		mid := len(kids) / 2
		id := uint32(kids[mid])
		slots[id].left = buildBST(kids[:mid])
		slots[id].right = buildBST(kids[mid+1:])
		return id
	}
	for id, e := range walked {
		if e.node.isStream {
			continue
		}
		slots[id].child = buildBST(e.sortedKids)
	}

	// Root entry's stream is the mini-stream container.
	if miniStreamBytes > 0 {
		slots[0].startSect = uint32(miniStreamStart)
		slots[0].size = uint64(miniStreamBytes)
	} else {
		slots[0].startSect = secEnd
		slots[0].size = 0
	}

	for i, s := range slots {
		sect := dirStart + i/dirEntriesPerSect
		slot := i % dirEntriesPerSect
		off := sectorAt(sect) + slot*dirEntrySize
		writeDirEntry(file[off:off+dirEntrySize], s)
	}

	// ── Write FAT sectors. ──
	for i, v := range fat {
		sect := i / fatEntriesPerSect
		slot := i % fatEntriesPerSect
		binary.LittleEndian.PutUint32(file[sectorAt(sect)+slot*4:], v)
	}

	// ── Write DIFAT continuation sectors. ──
	if nDIFAT > 0 {
		remaining := nFAT - maxDIFATInHeader
		fatIdx := maxDIFATInHeader
		for i := 0; i < nDIFAT; i++ {
			sect := nFAT + i
			off := sectorAt(sect)
			for j := 0; j < fatEntriesPerSect; j++ {
				binary.LittleEndian.PutUint32(file[off+j*4:], secFree)
			}
			n := remaining
			if n > difatSlotsPerSect {
				n = difatSlotsPerSect
			}
			for j := 0; j < n; j++ {
				binary.LittleEndian.PutUint32(file[off+j*4:], uint32(fatIdx+j))
			}
			fatIdx += n
			remaining -= n
			next := secEnd
			if i < nDIFAT-1 {
				next = uint32(nFAT + i + 1)
			}
			binary.LittleEndian.PutUint32(file[off+(fatEntriesPerSect-1)*4:], next)
		}
	}

	return file, nil
}

func writeDirEntry(dst []byte, e dirSlot) {
	for i := range dst {
		dst[i] = 0
	}
	if e.objType == objEmpty {
		binary.LittleEndian.PutUint32(dst[68:], secNoEntry)
		binary.LittleEndian.PutUint32(dst[72:], secNoEntry)
		binary.LittleEndian.PutUint32(dst[76:], secNoEntry)
		return
	}
	u := utf16.Encode([]rune(e.name))
	for i, c := range u {
		binary.LittleEndian.PutUint16(dst[i*2:], c)
	}
	nameLen := (len(u) + 1) * 2 // include null terminator
	binary.LittleEndian.PutUint16(dst[64:], uint16(nameLen))
	dst[66] = e.objType
	dst[67] = 1 // black; readers in this repo do not enforce RB balance
	binary.LittleEndian.PutUint32(dst[68:], e.left)
	binary.LittleEndian.PutUint32(dst[72:], e.right)
	binary.LittleEndian.PutUint32(dst[76:], e.child)
	binary.LittleEndian.PutUint32(dst[116:], e.startSect)
	binary.LittleEndian.PutUint32(dst[120:], uint32(e.size))
	binary.LittleEndian.PutUint32(dst[124:], uint32(e.size>>32))
}

func roundUpInt(x, m int) int {
	if m <= 0 || x%m == 0 {
		return x
	}
	return ((x / m) + 1) * m
}
