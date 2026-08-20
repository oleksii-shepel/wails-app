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
	shcoreDLL                = windows.NewLazySystemDLL("shcore.dll")
	procGetMonitorInfoW      = user32DLL.NewProc("GetMonitorInfoW")
	procMonitorFromRect      = user32DLL.NewProc("MonitorFromRect")
	procSystemParametersInfo = user32DLL.NewProc("SystemParametersInfoW")
	procGetDpiForMonitor     = shcoreDLL.NewProc("GetDpiForMonitor")
)

const mdtEffectiveDPI = 0

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
	metrics, ok := startupWorkMetrics(x, y, width, height, hasPosition)
	if !ok {
		return x, y, width, height
	}

	maxWidth := scaleToDefaultDPI(metrics.right-metrics.left, metrics.dpiX)
	maxHeight := scaleToDefaultDPI(metrics.bottom-metrics.top, metrics.dpiY)
	width = clampWindowDimension(width, maxWidth)
	height = clampWindowDimension(height, maxHeight)

	if !hasPosition {
		return x, y, width, height
	}

	windowWidthPx := scaleWithDPI(width, metrics.dpiX)
	windowHeightPx := scaleWithDPI(height, metrics.dpiY)
	if x < metrics.left {
		x = metrics.left
	}
	if y < metrics.top {
		y = metrics.top
	}
	if x+windowWidthPx > metrics.right {
		x = metrics.right - windowWidthPx
	}
	if y+windowHeightPx > metrics.bottom {
		y = metrics.bottom - windowHeightPx
	}
	if x < metrics.left {
		x = metrics.left
	}
	if y < metrics.top {
		y = metrics.top
	}

	return x, y, width, height
}

func startupWorkArea(x, y, width, height int, hasPosition bool) (displayRect, bool) {
	metrics, ok := startupWorkMetrics(x, y, width, height, hasPosition)
	if !ok {
		return displayRect{}, false
	}
	return displayRect{
		left:   metrics.left,
		top:    metrics.top,
		right:  metrics.left + scaleToDefaultDPI(metrics.right-metrics.left, metrics.dpiX),
		bottom: metrics.top + scaleToDefaultDPI(metrics.bottom-metrics.top, metrics.dpiY),
	}, true
}

type workAreaMetrics struct {
	left   int
	top    int
	right  int
	bottom int
	dpiX   uint
	dpiY   uint
}

func startupWorkMetrics(x, y, width, height int, hasPosition bool) (workAreaMetrics, bool) {
	if hasPosition {
		if metrics, ok := monitorWorkMetricsForRect(winRect{
			Left:   int32(x),
			Top:    int32(y),
			Right:  int32(x + width),
			Bottom: int32(y + height),
		}); ok {
			return metrics, true
		}
	}
	return primaryWorkMetrics()
}

func primaryWorkMetrics() (workAreaMetrics, bool) {
	var rect winRect
	ret, _, _ := procSystemParametersInfo.Call(
		uintptr(spiGetWorkArea),
		0,
		uintptr(unsafe.Pointer(&rect)),
		0,
	)
	if ret == 0 {
		return workAreaMetrics{}, false
	}
	// SPI_GETWORKAREA returns physical-pixel bounds. Resolve the owning monitor
	// so we can convert the work area into Wails' default 96-DPI space using
	// the monitor's effective DPI instead of assuming 96.
	if metrics, ok := monitorWorkMetricsForRect(rect); ok {
		return metrics, true
	}
	return workAreaMetrics{
		left:   int(rect.Left),
		top:    int(rect.Top),
		right:  int(rect.Right),
		bottom: int(rect.Bottom),
		dpiX:   96,
		dpiY:   96,
	}, true
}

func monitorWorkMetricsForRect(rect winRect) (workAreaMetrics, bool) {
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
			return workAreaMetrics{}, false
		}
	}

	info := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	ret, _, _ := procGetMonitorInfoW.Call(
		monitor,
		uintptr(unsafe.Pointer(&info)),
	)
	if ret == 0 {
		return workAreaMetrics{}, false
	}

	dpiX, dpiY := monitorDPI(monitor)
	return workAreaMetrics{
		left:   int(info.RcWork.Left),
		top:    int(info.RcWork.Top),
		right:  int(info.RcWork.Right),
		bottom: int(info.RcWork.Bottom),
		dpiX:   dpiX,
		dpiY:   dpiY,
	}, true
}

func monitorDPI(monitor uintptr) (uint, uint) {
	var dpiX uint32 = 96
	var dpiY uint32 = 96
	if procGetDpiForMonitor.Find() == nil {
		ret, _, _ := procGetDpiForMonitor.Call(
			monitor,
			uintptr(mdtEffectiveDPI),
			uintptr(unsafe.Pointer(&dpiX)),
			uintptr(unsafe.Pointer(&dpiY)),
		)
		if ret == 0 && dpiX > 0 && dpiY > 0 {
			return uint(dpiX), uint(dpiY)
		}
	}
	return 96, 96
}

func scaleWithDPI(value int, dpi uint) int {
	if dpi == 0 {
		return value
	}
	return (value * int(dpi)) / 96
}

func scaleToDefaultDPI(value int, dpi uint) int {
	if dpi == 0 {
		return value
	}
	return (value * 96) / int(dpi)
}
