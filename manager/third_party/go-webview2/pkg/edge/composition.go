//go:build windows

package edge

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type COREWEBVIEW2_MOUSE_EVENT_KIND uint32
type COREWEBVIEW2_MOUSE_EVENT_VIRTUAL_KEYS uint32

const (
	COREWEBVIEW2_MOUSE_EVENT_KIND_LEAVE COREWEBVIEW2_MOUSE_EVENT_KIND = 0x02A3
)

type iCoreWebView2Environment3Vtbl struct {
	_IUnknownVtbl
	CreateCoreWebView2CompositionController ComProc
	CreateCoreWebView2PointerInfo           ComProc
}

type ICoreWebView2Environment3 struct {
	vtbl *iCoreWebView2Environment3Vtbl
}

func (i *ICoreWebView2Environment3) Release() uintptr {
	ret, _, _ := i.vtbl.Release.Call(uintptr(unsafe.Pointer(i)))
	return ret
}

func (i *ICoreWebView2Environment3) CreateCoreWebView2CompositionController(parentWindow HWND, handler *iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandler) error {
	hr, _, _ := i.vtbl.CreateCoreWebView2CompositionController.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(parentWindow),
		uintptr(unsafe.Pointer(handler)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return syscall.Errno(hr)
	}
	return nil
}

func (i *ICoreWebView2Environment) GetICoreWebView2Environment3() *ICoreWebView2Environment3 {
	var result *ICoreWebView2Environment3
	iid := NewGUID("{80A22AE3-BE7C-4CE2-AFE1-5A50056CDEEB}")
	_, _, _ = i.vtbl.QueryInterface.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&result)),
	)
	return result
}

type iCoreWebView2CompositionControllerVtbl struct {
	_IUnknownVtbl
	GetRootVisualTarget ComProc
	PutRootVisualTarget ComProc
	SendMouseInput      ComProc
	SendPointerInput    ComProc
	GetCursor           ComProc
	GetSystemCursorId   ComProc
	AddCursorChanged    ComProc
	RemoveCursorChanged ComProc
}

type ICoreWebView2CompositionController struct {
	vtbl *iCoreWebView2CompositionControllerVtbl
}

func (i *ICoreWebView2CompositionController) GetICoreWebView2Controller() *ICoreWebView2Controller {
	var result *ICoreWebView2Controller
	iid := NewGUID("{4D00C0D1-9434-4EB6-8078-8697A560334F}")
	hr, _, _ := i.vtbl.QueryInterface.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&result)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return nil
	}
	return result
}

func (i *ICoreWebView2CompositionController) PutRootVisualTarget(target *IUnknown) error {
	hr, _, _ := i.vtbl.PutRootVisualTarget.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(target)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return windows.Errno(hr)
	}
	return nil
}

func (i *ICoreWebView2CompositionController) SendMouseInput(eventKind COREWEBVIEW2_MOUSE_EVENT_KIND, virtualKeys COREWEBVIEW2_MOUSE_EVENT_VIRTUAL_KEYS, mouseData uint32, point POINT) error {
	hr, _, _ := i.vtbl.SendMouseInput.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(eventKind),
		uintptr(virtualKeys),
		uintptr(unsafe.Pointer(&mouseData)),
		uintptr(unsafe.Pointer(&point)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return windows.Errno(hr)
	}
	return nil
}

func (i *ICoreWebView2CompositionController) GetCursor() (HCURSOR, error) {
	var cursor HCURSOR
	hr, _, _ := i.vtbl.GetCursor.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&cursor)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return 0, windows.Errno(hr)
	}
	return cursor, nil
}

type iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerImpl interface {
	_IUnknownImpl
	CreateCoreWebView2CompositionControllerCompleted(res uintptr, controller *ICoreWebView2CompositionController) uintptr
}

type iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerVtbl struct {
	_IUnknownVtbl
	Invoke ComProc
}

type iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandler struct {
	vtbl *iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerVtbl
	impl iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerImpl
}

func _ICoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerIUnknownQueryInterface(this *iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandler, refiid, object uintptr) uintptr {
	return this.impl.QueryInterface(refiid, object)
}

func _ICoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerIUnknownAddRef(this *iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandler) uintptr {
	return this.impl.AddRef()
}

func _ICoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerIUnknownRelease(this *iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandler) uintptr {
	return this.impl.Release()
}

func _ICoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerInvoke(this *iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandler, res uintptr, controller *ICoreWebView2CompositionController) uintptr {
	return this.impl.CreateCoreWebView2CompositionControllerCompleted(res, controller)
}

var iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerFn = iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerVtbl{
	_IUnknownVtbl{
		NewComProc(_ICoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerIUnknownQueryInterface),
		NewComProc(_ICoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerIUnknownAddRef),
		NewComProc(_ICoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerIUnknownRelease),
	},
	NewComProc(_ICoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerInvoke),
}

func newICoreWebView2CreateCoreWebView2CompositionControllerCompletedHandler(impl iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerImpl) *iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandler {
	return &iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandler{
		vtbl: &iCoreWebView2CreateCoreWebView2CompositionControllerCompletedHandlerFn,
		impl: impl,
	}
}