//go:build windows

package browserpane

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

const (
	wsPopup        = 0x80000000
	wsClipChildren = 0x02000000
	wsClipSiblings = 0x04000000
	wsExToolWindow = 0x00000080
	swHide         = 0
	swShow         = 5
	swShowNA       = 8
	swpNoZOrder    = 0x0004
	swpNoActivate  = 0x0010
	pmRemove       = 0x0001
	gwOwner        = 4
	wmClose        = 0x0010
	wmDestroy      = 0x0002
	wmNCDestroy    = 0x0082
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procRegisterClassExW   = user32.NewProc("RegisterClassExW")
	procCreateWindowExW    = user32.NewProc("CreateWindowExW")
	procDefWindowProcW     = user32.NewProc("DefWindowProcW")
	procDestroyWindow      = user32.NewProc("DestroyWindow")
	procShowWindow         = user32.NewProc("ShowWindow")
	procSetWindowPos       = user32.NewProc("SetWindowPos")
	procPeekMessageW       = user32.NewProc("PeekMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procEnumWindows        = user32.NewProc("EnumWindows")
	procGetWindowThreadPID = user32.NewProc("GetWindowThreadProcessId")
	procGetWindow          = user32.NewProc("GetWindow")
	procIsWindowVisible    = user32.NewProc("IsWindowVisible")
	procClientToScreen     = user32.NewProc("ClientToScreen")
	procPostMessageW       = user32.NewProc("PostMessageW")
	procGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	registerPaneClassOnce  sync.Once
	registerPaneClassErr   error
	mainWindowHandleOnce   sync.Once
	mainWindowHandle       uintptr
	mainWindowHandleErr    error
	browserPaneWndProc     = windows.NewCallback(browserPaneWindowProc)
)

type point struct {
	X int32
	Y int32
}

type msg struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type windowsPane struct {
	surfaceID string
	commands  chan windowsPaneCommand
	closed    chan struct{}
}

type windowsPaneCommand struct {
	update  *Update
	dispose bool
	result  chan error
}

type windowsPaneState struct {
	ownerHWND uintptr
	hostHWND  uintptr
	chromium  *edge.Chromium
	url       string
	visible   bool
}

func newPlatformPane(surfaceID string) (pane, error) {
	result := &windowsPane{
		surfaceID: surfaceID,
		commands:  make(chan windowsPaneCommand),
		closed:    make(chan struct{}),
	}
	go result.run()
	return result, nil
}

func (p *windowsPane) Sync(update Update) error {
	reply := make(chan error, 1)
	select {
	case <-p.closed:
		return nil
	case p.commands <- windowsPaneCommand{update: &update, result: reply}:
	}
	return <-reply
}

func (p *windowsPane) Dispose() error {
	reply := make(chan error, 1)
	select {
	case <-p.closed:
		return nil
	case p.commands <- windowsPaneCommand{dispose: true, result: reply}:
	}
	err := <-reply
	<-p.closed
	return err
}

func (p *windowsPane) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(p.closed)

	state, err := newWindowsPaneState()
	if err != nil {
		p.drainWithError(err)
		return
	}

	for {
		select {
		case command := <-p.commands:
			if command.dispose {
				command.result <- state.dispose()
				return
			}
			command.result <- state.sync(*command.update)
		default:
			pumpWindowsMessages()
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (p *windowsPane) drainWithError(err error) {
	for {
		select {
		case command := <-p.commands:
			command.result <- err
			if command.dispose {
				return
			}
		default:
			return
		}
	}
}

func newWindowsPaneState() (*windowsPaneState, error) {
	ownerHWND, err := discoverMainWindowHandle()
	if err != nil {
		return nil, err
	}
	if err := registerBrowserPaneClass(); err != nil {
		return nil, err
	}

	className, _ := windows.UTF16PtrFromString("WailsNativeBrowserPane")
	instance, _, _ := procGetModuleHandleW.Call(0)
	hostHWND, _, createErr := procCreateWindowExW.Call(
		uintptr(wsExToolWindow),
		uintptr(unsafe.Pointer(className)),
		0,
		uintptr(wsPopup|wsClipChildren|wsClipSiblings),
		0,
		0,
		1,
		1,
		ownerHWND,
		0,
		instance,
		0,
	)
	if hostHWND == 0 {
		return nil, fmt.Errorf("create browser pane window: %w", createErr)
	}

	chromium := edge.NewChromium()
	var chromiumErr error
	chromium.SetErrorCallback(func(err error) {
		if err != nil && chromiumErr == nil {
			chromiumErr = err
		}
	})
	chromium.SetGlobalPermission(edge.CoreWebView2PermissionStateAllow)
	chromium.Embed(hostHWND)
	if chromiumErr != nil {
		_, _, _ = procDestroyWindow.Call(hostHWND)
		return nil, chromiumErr
	}
	_, _, _ = procShowWindow.Call(hostHWND, swHide)

	return &windowsPaneState{
		ownerHWND: ownerHWND,
		hostHWND:  hostHWND,
		chromium:  chromium,
	}, nil
}

func (s *windowsPaneState) sync(update Update) error {
	if !update.Visible {
		if s.visible {
			_ = s.chromium.Hide()
			_, _, _ = procShowWindow.Call(s.hostHWND, swHide)
			s.visible = false
		}
		return nil
	}

	screenPoint, err := clientToScreen(s.ownerHWND, update.X, update.Y)
	if err != nil {
		return err
	}

	_, _, setPosErr := procSetWindowPos.Call(
		s.hostHWND,
		0,
		uintptr(screenPoint.X),
		uintptr(screenPoint.Y),
		uintptr(update.Width),
		uintptr(update.Height),
		uintptr(swpNoZOrder),
	)
	if setPosErr != windows.ERROR_SUCCESS && setPosErr != syscall.Errno(0) {
		return fmt.Errorf("position browser pane: %w", setPosErr)
	}

	if s.url != update.URL {
		s.chromium.Navigate(update.URL)
		s.url = update.URL
	}
	if !s.visible {
		_, _, _ = procShowWindow.Call(s.hostHWND, swShowNA)
		_ = s.chromium.Show()
		s.visible = true
	}
	return nil
}

func (s *windowsPaneState) dispose() error {
	if s.chromium != nil {
		_ = s.chromium.Hide()
	}
	if s.hostHWND != 0 {
		_, _, _ = procPostMessageW.Call(s.hostHWND, wmClose, 0, 0)
		_, _, _ = procDestroyWindow.Call(s.hostHWND)
	}
	return nil
}

func registerBrowserPaneClass() error {
	registerPaneClassOnce.Do(func() {
		className, _ := windows.UTF16PtrFromString("WailsNativeBrowserPane")
		instance, _, _ := procGetModuleHandleW.Call(0)
		class := wndClassEx{
			Size:      uint32(unsafe.Sizeof(wndClassEx{})),
			WndProc:   browserPaneWndProc,
			Instance:  instance,
			ClassName: className,
		}
		atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
		if atom == 0 && err != windows.ERROR_CLASS_ALREADY_EXISTS && err != syscall.Errno(1410) {
			registerPaneClassErr = fmt.Errorf("register browser pane class: %w", err)
		}
	})
	return registerPaneClassErr
}

func browserPaneWindowProc(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case wmClose, wmDestroy, wmNCDestroy:
		return 0
	default:
		result, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return result
	}
}

func pumpWindowsMessages() {
	var message msg
	for {
		ret, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, pmRemove)
		if ret == 0 {
			return
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func discoverMainWindowHandle() (uintptr, error) {
	mainWindowHandleOnce.Do(func() {
		currentPID := uint32(windows.GetCurrentProcessId())
		callback := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
			var pid uint32
			procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
			if pid != currentPID {
				return 1
			}
			visible, _, _ := procIsWindowVisible.Call(hwnd)
			if visible == 0 {
				return 1
			}
			owner, _, _ := procGetWindow.Call(hwnd, gwOwner)
			if owner != 0 {
				return 1
			}
			mainWindowHandle = hwnd
			return 0
		})
		ret, _, err := procEnumWindows.Call(callback, 0)
		if ret == 0 && mainWindowHandle == 0 && err != syscall.Errno(0) {
			mainWindowHandleErr = fmt.Errorf("enumerate windows: %w", err)
			return
		}
		if mainWindowHandle == 0 && mainWindowHandleErr == nil {
			mainWindowHandleErr = fmt.Errorf("main window handle not found")
		}
	})
	return mainWindowHandle, mainWindowHandleErr
}

func clientToScreen(hwnd uintptr, x int, y int) (point, error) {
	pt := point{X: int32(x), Y: int32(y)}
	ret, _, err := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
	if ret == 0 {
		return point{}, fmt.Errorf("client to screen: %w", err)
	}
	return pt, nil
}
