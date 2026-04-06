//go:build windows

package browserpane

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

const (
	wsChild        = 0x40000000
	wsClipChildren = 0x02000000
	wsClipSiblings = 0x04000000
	swHide         = 0
	swShow         = 5
	swShowNA       = 8
	swpNoActivate  = 0x0010
	pmRemove       = 0x0001
	gwOwner        = 4
	wmEraseBkgnd   = 0x0014
	wmClose        = 0x0010
	wmDestroy      = 0x0002
	wmNCDestroy    = 0x0082
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	gdi32                  = windows.NewLazySystemDLL("gdi32.dll")
	procRegisterClassExW   = user32.NewProc("RegisterClassExW")
	procCreateWindowExW    = user32.NewProc("CreateWindowExW")
	procDefWindowProcW     = user32.NewProc("DefWindowProcW")
	procDestroyWindow      = user32.NewProc("DestroyWindow")
	procShowWindow         = user32.NewProc("ShowWindow")
	procUpdateWindow       = user32.NewProc("UpdateWindow")
	procInvalidateRect     = user32.NewProc("InvalidateRect")
	procSetWindowPos       = user32.NewProc("SetWindowPos")
	procPeekMessageW       = user32.NewProc("PeekMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procEnumThreadWindows  = user32.NewProc("EnumThreadWindows")
	procEnumWindows        = user32.NewProc("EnumWindows")
	procGetWindowThreadPID = user32.NewProc("GetWindowThreadProcessId")
	procGetWindow          = user32.NewProc("GetWindow")
	procIsWindowVisible    = user32.NewProc("IsWindowVisible")
	procIsWindow           = user32.NewProc("IsWindow")
	procGetClientRect      = user32.NewProc("GetClientRect")
	procClientToScreen     = user32.NewProc("ClientToScreen")
	procPostMessageW       = user32.NewProc("PostMessageW")
	procGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	procGetCurrentThreadID = kernel32.NewProc("GetCurrentThreadId")
	procCreateSolidBrush   = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procFillRect           = user32.NewProc("FillRect")
	registerPaneClassOnce  sync.Once
	registerPaneClassErr   error
	mainWindowHandleOnce   sync.Once
	mainWindowHandle       uintptr
	mainWindowHandleErr    error
	browserPaneHostColors  sync.Map
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

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type browserPaneHostColor struct {
	R uint8
	G uint8
	B uint8
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
	theme     string
	visible   bool
}

type rawIUnknownVTBL struct {
	QueryInterface edge.ComProc
	AddRef         edge.ComProc
	Release        edge.ComProc
}

type rawWebView2ControllerVTBL struct {
	rawIUnknownVTBL
	GetIsVisible                      edge.ComProc
	PutIsVisible                      edge.ComProc
	GetBounds                         edge.ComProc
	PutBounds                         edge.ComProc
	GetZoomFactor                     edge.ComProc
	PutZoomFactor                     edge.ComProc
	AddZoomFactorChanged              edge.ComProc
	RemoveZoomFactorChanged           edge.ComProc
	SetBoundsAndZoomFactor            edge.ComProc
	MoveFocus                         edge.ComProc
	AddMoveFocusRequested             edge.ComProc
	RemoveMoveFocusRequested          edge.ComProc
	AddGotFocus                       edge.ComProc
	RemoveGotFocus                    edge.ComProc
	AddLostFocus                      edge.ComProc
	RemoveLostFocus                   edge.ComProc
	AddAcceleratorKeyPressed          edge.ComProc
	RemoveAcceleratorKeyPressed       edge.ComProc
	GetParentWindow                   edge.ComProc
	PutParentWindow                   edge.ComProc
	NotifyParentWindowPositionChanged edge.ComProc
	Close                             edge.ComProc
	GetCoreWebView2                   edge.ComProc
}

type rawWebView2Controller struct {
	Vtbl *rawWebView2ControllerVTBL
}

type rawWebView2EnvironmentVTBL struct {
	rawIUnknownVTBL
	CreateCoreWebView2Controller     edge.ComProc
	CreateWebResourceResponse        edge.ComProc
	GetBrowserVersionString          edge.ComProc
	AddNewBrowserVersionAvailable    edge.ComProc
	RemoveNewBrowserVersionAvailable edge.ComProc
}

type rawWebView2Environment struct {
	Vtbl *rawWebView2EnvironmentVTBL
}

type rawChromium struct {
	HWND                             uintptr
	Padding                          struct{ Left, Top, Right, Bottom int32 }
	Controller                       *edge.ICoreWebView2Controller
	WebView                          *edge.ICoreWebView2
	Inited                           uintptr
	EnvCompleted                     unsafe.Pointer
	ControllerCompleted              unsafe.Pointer
	WebMessageReceived               unsafe.Pointer
	ContainsFullScreenElementChanged unsafe.Pointer
	PermissionRequested              unsafe.Pointer
	WebResourceRequested             unsafe.Pointer
	AcceleratorKeyPressed            unsafe.Pointer
	NavigationCompleted              unsafe.Pointer
	ProcessFailed                    unsafe.Pointer
	Environment                      *edge.ICoreWebView2Environment
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

	if err := initializeBrowserPaneCOM(); err != nil {
		p.drainWithError(err)
		return
	}
	defer windows.CoUninitialize()

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

func initializeBrowserPaneCOM() error {
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE)
	if err == nil {
		return nil
	}
	if err == windows.Errno(windows.S_FALSE) {
		return nil
	}
	return fmt.Errorf("initialize COM for browser pane: %w", err)
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
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		uintptr(wsChild|wsClipChildren|wsClipSiblings),
		0,
		0,
		64,
		64,
		ownerHWND,
		0,
		instance,
		0,
	)
	if hostHWND == 0 {
		return nil, fmt.Errorf("create browser pane window: %w", createErr)
	}
	_, _, _ = procShowWindow.Call(hostHWND, swShowNA)
	_, _, _ = procUpdateWindow.Call(hostHWND)

	chromium := edge.NewChromium()
	chromium.DataPath = browserPaneDataPath()
	var chromiumErr error
	state := &windowsPaneState{
		ownerHWND: ownerHWND,
		hostHWND:  hostHWND,
		chromium:  chromium,
	}
	chromium.SetErrorCallback(func(err error) {
		if err == nil {
			return
		}
		log.Printf("[BrowserPane] WebView2 error: %v", err)
		if chromiumErr == nil {
			chromiumErr = err
		}
	})
	chromium.NavigationCompletedCallback = func(_ *edge.ICoreWebView2, _ *edge.ICoreWebView2NavigationCompletedEventArgs) {
		if err := state.applyScrollbarTheme(); err != nil {
			log.Printf("[BrowserPane] Failed to apply scrollbar theme after navigation: %v", err)
		}
	}
	chromium.SetGlobalPermission(edge.CoreWebView2PermissionStateAllow)
	if !chromium.Embed(hostHWND) {
		if chromiumErr == nil {
			chromiumErr = fmt.Errorf("embed browser pane: WebView2 controller initialization failed")
		}
	}
	if chromiumErr != nil {
		_, _, _ = procDestroyWindow.Call(hostHWND)
		return nil, chromiumErr
	}
	_, _, _ = procShowWindow.Call(hostHWND, swHide)

	return state, nil
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

	_, _, setPosErr := procSetWindowPos.Call(
		s.hostHWND,
		0,
		uintptr(update.X),
		uintptr(update.Y),
		uintptr(update.Width),
		uintptr(update.Height),
		uintptr(swpNoActivate),
	)
	if setPosErr != windows.ERROR_SUCCESS && setPosErr != syscall.Errno(0) {
		return fmt.Errorf("position browser pane: %w", setPosErr)
	}
	s.chromium.Resize()

	if s.theme != update.Theme {
		s.theme = update.Theme
	}
	if err := s.applyTheme(); err != nil {
		log.Printf("[BrowserPane] Failed to apply theme: %v", err)
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
	var firstErr error

	if s.chromium != nil {
		s.chromium.ShuttingDown()
		controller, webview, environment := chromiumHandles(s.chromium)
		if controller != nil {
			_ = controller.PutIsVisible(false)
			if err := closeWebView2Controller(controller); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("close browser controller: %w", err)
			}
		}
		if webview != nil {
			webview.Release()
		}
		if controller != nil {
			controller.Release()
		}
		if environment != nil {
			releaseCOMObject(unsafe.Pointer(environment))
		}
		clearChromiumHandles(s.chromium)
		s.chromium = nil
	}
	if s.hostHWND != 0 {
		clearBrowserPaneHostBackground(s.hostHWND)
		hostHWND := s.hostHWND
		threadID := currentThreadID()
		if isWindow(hostHWND) {
			result, _, destroyErr := procDestroyWindow.Call(hostHWND)
			if result == 0 && destroyErr != windows.ERROR_SUCCESS && destroyErr != syscall.Errno(0) && firstErr == nil {
				firstErr = fmt.Errorf("destroy browser pane window: %w", destroyErr)
			}
		}
		pumpWindowsMessagesUntil(time.Now().Add(2*time.Second), func() bool {
			return !isWindow(hostHWND) && !threadHasTopLevelWindows(threadID)
		})
		s.hostHWND = 0
	}
	return firstErr
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
	case wmEraseBkgnd:
		if paintBrowserPaneBackground(hwnd, wParam) {
			return 1
		}
	case wmNCDestroy:
		clearBrowserPaneHostBackground(hwnd)
	}
	result, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return result
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

func pumpWindowsMessagesUntil(deadline time.Time, done func() bool) {
	for time.Now().Before(deadline) {
		if done != nil && done() {
			return
		}
		pumpWindowsMessages()
		time.Sleep(10 * time.Millisecond)
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

func browserPaneDataPath() string {
	baseDir := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if baseDir == "" {
		if userConfigDir, err := os.UserConfigDir(); err == nil {
			baseDir = userConfigDir
		}
	}
	if baseDir == "" {
		baseDir = os.TempDir()
	}

	exeName := "wails-app"
	if exePath, err := os.Executable(); err == nil {
		exeName = filepath.Base(exePath)
	}
	dataPath := filepath.Join(baseDir, exeName, "browser-pane-webview2")
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		log.Printf("[BrowserPane] Failed to create WebView2 data path %q: %v", dataPath, err)
	}
	return dataPath
}

func (s *windowsPaneState) applyScrollbarTheme() error {
	if s == nil || s.chromium == nil {
		return nil
	}
	s.chromium.Eval(browserPaneScrollbarScript(s.theme))
	return nil
}

func (s *windowsPaneState) applyTheme() error {
	if s == nil || s.chromium == nil {
		return nil
	}
	r, g, b := browserPaneThemeBackground(s.theme)
	setBrowserPaneHostBackground(s.hostHWND, r, g, b)
	_, _, _ = procInvalidateRect.Call(s.hostHWND, 0, 1)
	s.chromium.SetBackgroundColour(r, g, b, 255)
	return s.applyScrollbarTheme()
}

func browserPaneThemeBackground(theme string) (uint8, uint8, uint8) {
	if strings.EqualFold(strings.TrimSpace(theme), "light") {
		return 255, 255, 255
	}
	return 30, 30, 30
}

func browserPaneScrollbarScript(theme string) string {
	thumb := "rgba(121, 121, 121, 0.42)"
	thumbHover := "rgba(121, 121, 121, 0.58)"
	thumbActive := "rgba(121, 121, 121, 0.72)"
	if strings.EqualFold(strings.TrimSpace(theme), "light") {
		thumb = "rgba(100, 100, 100, 0.36)"
		thumbHover = "rgba(100, 100, 100, 0.50)"
		thumbActive = "rgba(100, 100, 100, 0.64)"
	}
	return fmt.Sprintf(`(function(){
  try {
    var root = document.documentElement;
    if (!root) return;
    root.setAttribute('data-wails-scrollbar-theme', %q);
    root.style.setProperty('--wails-scrollbar-size', '12px');
    root.style.setProperty('--wails-scrollbar-thumb', %q);
    root.style.setProperty('--wails-scrollbar-thumb-hover', %q);
    root.style.setProperty('--wails-scrollbar-thumb-active', %q);
    root.style.setProperty('scrollbar-color', 'var(--wails-scrollbar-thumb) transparent');
    var css = [
      'html, body, * { scrollbar-width: thin !important; scrollbar-color: var(--wails-scrollbar-thumb) transparent !important; }',
      '*::-webkit-scrollbar { width: var(--wails-scrollbar-size) !important; height: var(--wails-scrollbar-size) !important; background: transparent !important; }',
      '*::-webkit-scrollbar-button { width: 0 !important; height: 0 !important; display: none !important; }',
      '*::-webkit-scrollbar-track { background: transparent !important; }',
      '*::-webkit-scrollbar-track-piece { background: transparent !important; }',
      '*::-webkit-scrollbar-corner { background: transparent !important; }',
      '*::-webkit-resizer { background: transparent !important; }',
      '*::-webkit-scrollbar-thumb {',
      '  min-height: 20px !important;',
      '  min-width: 20px !important;',
      '  border-radius: 0 !important;',
      '  border: none !important;',
      '  box-shadow: none !important;',
      '  background: var(--wails-scrollbar-thumb) !important;',
      '}',
      '*::-webkit-scrollbar-thumb:hover { background: var(--wails-scrollbar-thumb-hover) !important; }',
      '*::-webkit-scrollbar-thumb:active { background: var(--wails-scrollbar-thumb-active) !important; }'
    ].join('\n');

    var sheet = window.__wailsBrowserPaneScrollbarSheet;
    if (!sheet && typeof CSSStyleSheet !== 'undefined') {
      try {
        sheet = new CSSStyleSheet();
        sheet.replaceSync(css);
        window.__wailsBrowserPaneScrollbarSheet = sheet;
      } catch (_) {
        sheet = null;
      }
    }

    if (sheet && 'adoptedStyleSheets' in document) {
      var adopted = document.adoptedStyleSheets || [];
      if (adopted.indexOf(sheet) === -1) {
        document.adoptedStyleSheets = adopted.concat(sheet);
      }
    } else {
      var styleId = 'wails-browserpane-scrollbars';
      var style = document.getElementById(styleId);
      if (!style) {
        style = document.createElement('style');
        style.id = styleId;
        style.textContent = css;
        (document.head || document.documentElement).appendChild(style);
      } else if (style.textContent !== css) {
        style.textContent = css;
      }
    }
  } catch (_) {}
})();`, theme, thumb, thumbHover, thumbActive)
}

func setBrowserPaneHostBackground(hwnd uintptr, r uint8, g uint8, b uint8) {
	if hwnd == 0 {
		return
	}
	browserPaneHostColors.Store(hwnd, browserPaneHostColor{R: r, G: g, B: b})
}

func clearBrowserPaneHostBackground(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	browserPaneHostColors.Delete(hwnd)
}

func paintBrowserPaneBackground(hwnd uintptr, hdc uintptr) bool {
	if hwnd == 0 || hdc == 0 {
		return false
	}
	value, ok := browserPaneHostColors.Load(hwnd)
	if !ok {
		return false
	}
	color := value.(browserPaneHostColor)
	client := rect{}
	result, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	if result == 0 {
		return false
	}
	colorRef := uintptr(uint32(color.R) | (uint32(color.G) << 8) | (uint32(color.B) << 16))
	brush, _, _ := procCreateSolidBrush.Call(colorRef)
	if brush == 0 {
		return false
	}
	defer procDeleteObject.Call(brush)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&client)), brush)
	return true
}

func currentThreadID() uint32 {
	threadID, _, _ := procGetCurrentThreadID.Call()
	return uint32(threadID)
}

func isWindow(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	result, _, _ := procIsWindow.Call(hwnd)
	return result != 0
}

func threadHasTopLevelWindows(threadID uint32) bool {
	if threadID == 0 {
		return false
	}
	hasWindow := false
	callback := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		hasWindow = true
		return 0
	})
	result, _, err := procEnumThreadWindows.Call(uintptr(threadID), callback, 0)
	if result == 0 && !hasWindow && err != syscall.Errno(0) {
		return false
	}
	return hasWindow
}

func chromiumHandles(chromium *edge.Chromium) (*edge.ICoreWebView2Controller, *edge.ICoreWebView2, *edge.ICoreWebView2Environment) {
	if chromium == nil {
		return nil, nil, nil
	}
	raw := (*rawChromium)(unsafe.Pointer(chromium))
	return raw.Controller, raw.WebView, raw.Environment
}

func clearChromiumHandles(chromium *edge.Chromium) {
	if chromium == nil {
		return
	}
	raw := (*rawChromium)(unsafe.Pointer(chromium))
	raw.Controller = nil
	raw.WebView = nil
	raw.Environment = nil
}

func closeWebView2Controller(controller *edge.ICoreWebView2Controller) error {
	if controller == nil {
		return nil
	}
	raw := (*rawWebView2Controller)(unsafe.Pointer(controller))
	hr, _, _ := raw.Vtbl.Close.Call(uintptr(unsafe.Pointer(raw)))
	if windows.Handle(hr) != windows.S_OK {
		return windows.Errno(hr)
	}
	return nil
}

func releaseCOMObject(object unsafe.Pointer) {
	if object == nil {
		return
	}
	raw := (*rawWebView2Environment)(object)
	raw.Vtbl.Release.Call(uintptr(object))
}
