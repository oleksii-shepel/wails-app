//go:build windows

package fontdata

import (
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	lfFacesize       = 32
	getFontDataError = 0xFFFFFFFF
)

var (
	modGdi32               = syscall.NewLazyDLL("gdi32.dll")
	procCreateCompatibleDC = modGdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = modGdi32.NewProc("DeleteDC")
	procCreateFontIndirect = modGdi32.NewProc("CreateFontIndirectW")
	procSelectObject       = modGdi32.NewProc("SelectObject")
	procDeleteObject       = modGdi32.NewProc("DeleteObject")
	procGetFontData        = modGdi32.NewProc("GetFontData")

	modT2Embed               = syscall.NewLazyDLL("t2embed.dll")
	procTTLoadEmbeddedFont   = modT2Embed.NewProc("TTLoadEmbeddedFont")
	procTTDeleteEmbeddedFont = modT2Embed.NewProc("TTDeleteEmbeddedFont")
	procTTGetNewFontName     = modT2Embed.NewProc("TTGetNewFontName")
)

type embeddedFontStream struct {
	data []byte
	pos  int
	mu   sync.Mutex
}

type ttLoadInfo struct {
	StructSize uint16
	RefStrSize uint16
	RefStr     *uint16
}

type logFont struct {
	Height         int32
	Width          int32
	Escapement     int32
	Orientation    int32
	Weight         int32
	Italic         byte
	Underline      byte
	StrikeOut      byte
	CharSet        byte
	OutPrecision   byte
	ClipPrecision  byte
	Quality        byte
	PitchAndFamily byte
	FaceName       [lfFacesize]uint16
}

func decodeEOT(data []byte, family string) ([]byte, bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if len(data) == 0 {
		return nil, false
	}
	stream := &embeddedFontStream{data: data}
	callback := syscall.NewCallbackCDecl(readEmbeddedFontStream)
	var loadInfo *ttLoadInfo
	var familyBuf []uint16
	if family != "" {
		familyBuf = utf16.Encode([]rune(family + "\x00"))
		if len(familyBuf) > 0 {
			loadInfo = &ttLoadInfo{
				StructSize: uint16(unsafe.Sizeof(ttLoadInfo{})),
				RefStrSize: uint16(len(familyBuf)),
				RefStr:     &familyBuf[0],
			}
		}
	}

	var fontRef uintptr
	var privStatus, status uint32
	ret, _, _ := procTTLoadEmbeddedFont.Call(
		uintptr(unsafe.Pointer(&fontRef)),
		0,
		uintptr(unsafe.Pointer(&privStatus)),
		0,
		uintptr(unsafe.Pointer(&status)),
		callback,
		uintptr(unsafe.Pointer(stream)),
		stringPtr(familyBuf),
		0,
		uintptr(unsafe.Pointer(loadInfo)),
	)
	if ret != 0 || fontRef == 0 {
		return nil, false
	}
	defer procTTDeleteEmbeddedFont.Call(fontRef, 0, 0)

	hdc, _, _ := procCreateCompatibleDC.Call(0)
	if hdc == 0 {
		return nil, false
	}
	defer procDeleteDC.Call(hdc)

	name := family
	if name == "" {
		if loaded, ok := embeddedFontName(&fontRef); ok {
			name = loaded
		}
	}
	hfont, ok := createFontByName(name)
	if !ok && name != "" {
		if loaded, ok2 := embeddedFontName(&fontRef); ok2 {
			hfont, ok = createFontByName(loaded)
		}
	}
	if !ok {
		return nil, false
	}
	defer procDeleteObject.Call(hfont)

	oldObj, _, _ := procSelectObject.Call(hdc, hfont)
	if oldObj == 0 {
		return nil, false
	}
	defer procSelectObject.Call(hdc, oldObj)

	size, ok := getSelectedFontDataSize(hdc)
	if !ok || size == 0 {
		return nil, false
	}
	buf := make([]byte, size)
	ret, _, _ = procGetFontData.Call(hdc, 0, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(size))
	if ret == getFontDataError || ret == 0 {
		return nil, false
	}
	if int(ret) < len(buf) {
		buf = buf[:ret]
	}
	return buf, true
}

//go:nocheckptr

// readEmbeddedFontStream is a cdecl callback used by TTLoadEmbeddedFont.
func readEmbeddedFontStream(lpvReadStream, lpvBuffer, cbBuffer uintptr) uintptr {
	if lpvReadStream == 0 || lpvBuffer == 0 || cbBuffer == 0 {
		return 0
	}
	stream := (*embeddedFontStream)(unsafe.Pointer(lpvReadStream))
	stream.mu.Lock()
	defer stream.mu.Unlock()

	remaining := len(stream.data) - stream.pos
	if remaining <= 0 {
		return 0
	}
	n := int(cbBuffer)
	if n > remaining {
		n = remaining
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(lpvBuffer)), n)
	copy(dst, stream.data[stream.pos:stream.pos+n])
	stream.pos += n
	return uintptr(n)
}

func embeddedFontName(fontRef *uintptr) (string, bool) {
	buf := make([]uint16, lfFacesize)
	ret, _, _ := procTTGetNewFontName.Call(
		uintptr(unsafe.Pointer(fontRef)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0,
		0,
	)
	if ret != 0 {
		return "", false
	}
	n := 0
	for ; n < len(buf); n++ {
		if buf[n] == 0 {
			break
		}
	}
	name := strings.TrimSpace(string(utf16.Decode(buf[:n])))
	if name == "" {
		return "", false
	}
	return name, true
}

func createFontByName(name string) (uintptr, bool) {
	if strings.TrimSpace(name) == "" {
		return 0, false
	}
	runes := utf16.Encode([]rune(name))
	if len(runes) >= lfFacesize {
		runes = runes[:lfFacesize-1]
	}
	var lf logFont
	copy(lf.FaceName[:], runes)
	hfont, _, _ := procCreateFontIndirect.Call(uintptr(unsafe.Pointer(&lf)))
	if hfont == 0 {
		return 0, false
	}
	return hfont, true
}

func stringPtr(buf []uint16) uintptr {
	if len(buf) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&buf[0]))
}

func getSelectedFontDataSize(hdc uintptr) (int, bool) {
	size, _, _ := procGetFontData.Call(hdc, 0, 0, 0, 0)
	if size == getFontDataError || size == 0 {
		return 0, false
	}
	return int(size), true
}
