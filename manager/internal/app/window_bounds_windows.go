//go:build windows

package app

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	spiGetWorkArea          = 0x0030
	monitorDefaultToPrimary = 0x00000001
	monitorDefaultToNearest = 0x00000002
)

var (
	user32DLL                = windows.NewLazySystemDLL("user32.dll")
	procGetMonitorInfoW      = user32DLL.NewProc("GetMonitorInfoW")
	procMonitorFromRect      = user32DLL.NewProc("MonitorFromRect")
	procSystemParametersInfo = user32DLL.NewProc("SystemParametersInfoW")
)

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type monitorInfo struct {
	CbSize    uint32
	RcMonitor winRect
	RcWork    winRect
	DwFlags   uint32
}

func fitStartupWindowBounds(x, y, width, height int, hasPosition bool) (int, int, int, int) {
	area, ok := startupWorkArea(x, y, width, height, hasPosition)
	if !ok {
		return x, y, width, height
	}
	return fitBoundsToDisplay(area, x, y, width, height, hasPosition)
}

func startupWorkArea(x, y, width, height int, hasPosition bool) (displayRect, bool) {
	if hasPosition {
		if area, ok := monitorWorkAreaForRect(winRect{
			Left:   int32(x),
			Top:    int32(y),
			Right:  int32(x + width),
			Bottom: int32(y + height),
		}); ok {
			return area, true
		}
	}
	return primaryWorkArea()
}

func primaryWorkArea() (displayRect, bool) {
	var rect winRect
	ret, _, _ := procSystemParametersInfo.Call(
		uintptr(spiGetWorkArea),
		0,
		uintptr(unsafe.Pointer(&rect)),
		0,
	)
	if ret == 0 {
		return displayRect{}, false
	}
	return displayRect{
		left:   int(rect.Left),
		top:    int(rect.Top),
		right:  int(rect.Right),
		bottom: int(rect.Bottom),
	}, true
}

func monitorWorkAreaForRect(rect winRect) (displayRect, bool) {
	monitor, _, _ := procMonitorFromRect.Call(
		uintptr(unsafe.Pointer(&rect)),
		uintptr(monitorDefaultToNearest),
	)
	if monitor == 0 {
		monitor, _, _ = procMonitorFromRect.Call(
			uintptr(unsafe.Pointer(&rect)),
			uintptr(monitorDefaultToPrimary),
		)
		if monitor == 0 {
			return displayRect{}, false
		}
	}

	info := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	ret, _, _ := procGetMonitorInfoW.Call(
		monitor,
		uintptr(unsafe.Pointer(&info)),
	)
	if ret == 0 {
		return displayRect{}, false
	}

	return displayRect{
		left:   int(info.RcWork.Left),
		top:    int(info.RcWork.Top),
		right:  int(info.RcWork.Right),
		bottom: int(info.RcWork.Bottom),
	}, true
}
