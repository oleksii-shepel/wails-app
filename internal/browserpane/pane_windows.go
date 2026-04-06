//go:build windows

package browserpane

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
	offscreenX     = -32000
	offscreenY     = -32000
	swHide         = 0
	swShow         = 5
	swShowNA       = 8
	swpNoActivate  = 0x0010
	swpNoZOrder    = 0x0004
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
	update       *Update
	dispose      bool
	capture      bool
	result       chan error
	captureReply chan string
}

type windowsPaneState struct {
	ownerHWND               uintptr
	hostHWND                uintptr
	chromium                *edge.Chromium
	url                     string
	theme                   string
	backgroundColor         string
	visible                 bool
	captureOnly             bool
	targetVisible           bool
	initialNavigationDone   bool
	initialNavigationQueued bool
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

// rawICoreWebView2Vtbl mirrors the ICoreWebView2 vtable layout up to CallDevToolsProtocolMethod.
type rawICoreWebView2Vtbl struct {
	rawIUnknownVTBL
	GetSettings                            edge.ComProc
	GetSource                              edge.ComProc
	Navigate                               edge.ComProc
	NavigateToString                       edge.ComProc
	AddNavigationStarting                  edge.ComProc
	RemoveNavigationStarting               edge.ComProc
	AddContentLoading                      edge.ComProc
	RemoveContentLoading                   edge.ComProc
	AddSourceChanged                       edge.ComProc
	RemoveSourceChanged                    edge.ComProc
	AddHistoryChanged                      edge.ComProc
	RemoveHistoryChanged                   edge.ComProc
	AddNavigationCompleted                 edge.ComProc
	RemoveNavigationCompleted              edge.ComProc
	AddFrameNavigationStarting             edge.ComProc
	RemoveFrameNavigationStarting          edge.ComProc
	AddFrameNavigationCompleted            edge.ComProc
	RemoveFrameNavigationCompleted         edge.ComProc
	AddScriptDialogOpening                 edge.ComProc
	RemoveScriptDialogOpening              edge.ComProc
	AddPermissionRequested                 edge.ComProc
	RemovePermissionRequested              edge.ComProc
	AddProcessFailed                       edge.ComProc
	RemoveProcessFailed                    edge.ComProc
	AddScriptToExecuteOnDocumentCreated    edge.ComProc
	RemoveScriptToExecuteOnDocumentCreated edge.ComProc
	ExecuteScript                          edge.ComProc
	CapturePreview                         edge.ComProc
	Reload                                 edge.ComProc
	PostWebMessageAsJSON                   edge.ComProc
	PostWebMessageAsString                 edge.ComProc
	AddWebMessageReceived                  edge.ComProc
	RemoveWebMessageReceived               edge.ComProc
	CallDevToolsProtocolMethod             edge.ComProc
}

type rawICoreWebView2 struct {
	Vtbl *rawICoreWebView2Vtbl
}

// cdpCallbackHandler is a COM callback for CallDevToolsProtocolMethod completion.
type cdpCallbackHandler struct {
	vtbl *cdpCallbackVtbl
	ch   chan cdpResult
}

type cdpCallbackVtbl struct {
	QueryInterface edge.ComProc
	AddRef         edge.ComProc
	Release        edge.ComProc
	Invoke         edge.ComProc
}

type cdpResult struct {
	json string
	err  error
}

var cdpCallbackVtblInstance *cdpCallbackVtbl

func init() {
	cdpCallbackVtblInstance = &cdpCallbackVtbl{
		QueryInterface: edge.NewComProc(cdpQueryInterface),
		AddRef:         edge.NewComProc(cdpAddRef),
		Release:        edge.NewComProc(cdpRelease),
		Invoke:         edge.NewComProc(cdpInvoke),
	}
}

func cdpQueryInterface(this, _, _ uintptr) uintptr { return 0 }
func cdpAddRef(this uintptr) uintptr               { return 1 }
func cdpRelease(this uintptr) uintptr              { return 1 }

func cdpInvoke(this uintptr, errorCode uintptr, resultUTF16 uintptr) uintptr {
	handler := (*cdpCallbackHandler)(unsafe.Pointer(this))
	var result string
	if resultUTF16 != 0 {
		result = windows.UTF16PtrToString((*uint16)(unsafe.Pointer(resultUTF16)))
	}
	var err error
	if errorCode != 0 {
		err = fmt.Errorf("CDP method failed: HRESULT 0x%x", errorCode)
	}
	select {
	case handler.ch <- cdpResult{json: result, err: err}:
	default:
	}
	return 0
}

func newPlatformPane(initial Update) (pane, error) {
	result := &windowsPane{
		surfaceID: initial.SurfaceID,
		commands:  make(chan windowsPaneCommand),
		closed:    make(chan struct{}),
	}
	go result.run(initial)
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

func (p *windowsPane) CaptureScreenshot() (string, error) {
	reply := make(chan error, 1)
	captureReply := make(chan string, 1)
	select {
	case <-p.closed:
		return "", fmt.Errorf("browser pane closed")
	case p.commands <- windowsPaneCommand{capture: true, result: reply, captureReply: captureReply}:
	}
	err := <-reply
	if err != nil {
		return "", err
	}
	return <-captureReply, nil
}

func (p *windowsPane) run(initial Update) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(p.closed)

	if err := initializeBrowserPaneCOM(); err != nil {
		p.drainWithError(err)
		return
	}
	defer windows.CoUninitialize()

	state, err := newWindowsPaneState(initial.Theme, initial.BackgroundColor)
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
			if command.capture {
				data, err := state.captureScreenshot()
				command.result <- err
				if err == nil {
					command.captureReply <- data
				}
				continue
			}
			command.result <- state.sync(*command.update)
		default:
			pumpWindowsMessages()
			time.Sleep(2 * time.Millisecond)
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

func newWindowsPaneState(initialTheme string, initialBackgroundColor string) (*windowsPaneState, error) {
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
	initialTheme = normalizeBrowserPaneTheme(initialTheme)
	r, g, b := resolveBrowserPaneBackgroundColor(initialTheme, initialBackgroundColor)
	setBrowserPaneHostBackground(hostHWND, r, g, b)
	_, _, _ = procShowWindow.Call(hostHWND, swShowNA)
	_, _, _ = procUpdateWindow.Call(hostHWND)

	chromium := edge.NewChromium()
	chromium.DataPath = browserPaneDataPath()
	var chromiumErr error
	state := &windowsPaneState{
		ownerHWND:       ownerHWND,
		hostHWND:        hostHWND,
		chromium:        chromium,
		theme:           initialTheme,
		backgroundColor: initialBackgroundColor,
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
		state.initialNavigationDone = true
		// Re-inject scrollbar styles via Eval as a reliable fallback;
		// Init (AddScriptToExecuteOnDocumentCreated) may not fire in all scenarios.
		state.chromium.Eval(browserPaneScrollbarScript(state.theme))
		if state.targetVisible && !state.visible {
			state.show()
		}
	}
	chromium.SetGlobalPermission(edge.CoreWebView2PermissionStateAllow)
	if !chromium.Embed(hostHWND) {
		if chromiumErr == nil {
			chromiumErr = fmt.Errorf("embed browser pane: WebView2 controller initialization failed")
		}
	}
	if chromiumErr != nil {
		clearBrowserPaneHostBackground(hostHWND)
		_, _, _ = procDestroyWindow.Call(hostHWND)
		return nil, chromiumErr
	}

	// Inject scripts before first page render via AddScriptToExecuteOnDocumentCreated.
	// These run on every navigation, before the page content is displayed.
	chromium.Init(browserPaneScrollbarScript(initialTheme))
	chromium.Init(browserPaneBackgroundScript(r, g, b))

	state.chromium.SetBackgroundColour(r, g, b, 255)
	_, _, _ = procShowWindow.Call(hostHWND, swHide)

	return state, nil
}

func (s *windowsPaneState) sync(update Update) error {
	s.targetVisible = update.Visible
	captureOnly := !update.Visible && update.Width > 0 && update.Height > 0
	targetX := update.X
	targetY := update.Y
	if captureOnly {
		targetX = offscreenX
		targetY = offscreenY
	}
	if update.Width > 0 && update.Height > 0 {
		_, _, setPosErr := procSetWindowPos.Call(
			s.hostHWND,
			0,
			uintptr(targetX),
			uintptr(targetY),
			uintptr(update.Width),
			uintptr(update.Height),
			uintptr(swpNoActivate),
		)
		if setPosErr != windows.ERROR_SUCCESS && setPosErr != syscall.Errno(0) {
			return fmt.Errorf("position browser pane: %w", setPosErr)
		}
		s.chromium.Resize()
		s.notifyPositionChanged()
	}

	themeChanged := s.theme != update.Theme || s.backgroundColor != update.BackgroundColor
	if s.theme != update.Theme {
		s.theme = update.Theme
	}
	if s.backgroundColor != update.BackgroundColor {
		s.backgroundColor = update.BackgroundColor
	}
	if themeChanged {
		if err := s.applyTheme(); err != nil {
			log.Printf("[BrowserPane] Failed to apply theme: %v", err)
		}
	}

	if s.url != update.URL {
		s.chromium.Navigate(update.URL)
		s.url = update.URL
		if !s.initialNavigationQueued {
			s.initialNavigationQueued = true
		}
	}
	if captureOnly {
		// Keep the live WebView rendering offscreen so screenshots stay fresh
		// while the DOM canvas replacement is visible to the user.
		s.captureOnly = true
		s.setControllerVisible(true)
		_, _, _ = procShowWindow.Call(s.hostHWND, swShowNA)
		_ = s.chromium.Show()
		s.visible = false
		return nil
	}
	if s.captureOnly {
		s.captureOnly = false
		s.setControllerVisible(true)
	}
	if !update.Visible {
		if s.visible {
			_ = s.chromium.Hide()
			_, _, _ = procShowWindow.Call(s.hostHWND, swHide)
			s.visible = false
		}
		return nil
	}
	if !s.initialNavigationDone {
		return nil
	}
	s.show()
	return nil
}

func (s *windowsPaneState) show() {
	if s == nil || s.hostHWND == 0 || s.chromium == nil {
		return
	}
	s.setControllerVisible(true)
	_, _, _ = procShowWindow.Call(s.hostHWND, swShowNA)
	_ = s.chromium.Show()
	s.visible = true
}

func (s *windowsPaneState) notifyPositionChanged() {
	controller, _, _ := chromiumHandles(s.chromium)
	if controller == nil {
		return
	}
	raw := (*rawWebView2Controller)(unsafe.Pointer(controller))
	raw.Vtbl.NotifyParentWindowPositionChanged.Call(uintptr(unsafe.Pointer(raw)))
}

func (s *windowsPaneState) setControllerVisible(visible bool) {
	controller, _, _ := chromiumHandles(s.chromium)
	if controller == nil {
		return
	}
	_ = controller.PutIsVisible(visible)
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

func (s *windowsPaneState) applyTheme() error {
	if s == nil || s.chromium == nil {
		return nil
	}
	r, g, b := resolveBrowserPaneBackgroundColor(s.theme, s.backgroundColor)
	setBrowserPaneHostBackground(s.hostHWND, r, g, b)
	_, _, _ = procInvalidateRect.Call(s.hostHWND, 0, 1)
	s.chromium.SetBackgroundColour(r, g, b, 255)
	// Push updated styles into the current page immediately.
	s.chromium.Eval(browserPaneBackgroundScript(r, g, b))
	s.chromium.Eval(browserPaneScrollbarScript(s.theme))
	return nil
}

func browserPaneThemeBackground(theme string) (uint8, uint8, uint8) {
	if strings.EqualFold(strings.TrimSpace(theme), "light") {
		return 255, 255, 255
	}
	return 30, 30, 30
}

func resolveBrowserPaneBackgroundColor(theme string, raw string) (uint8, uint8, uint8) {
	if r, g, b, ok := parseBrowserPaneColor(raw); ok {
		return r, g, b
	}
	return browserPaneThemeBackground(theme)
}

func parseBrowserPaneColor(raw string) (uint8, uint8, uint8, bool) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return 0, 0, 0, false
	}
	if match := strings.TrimPrefix(value, "#"); len(match) == 6 && len(value) == 7 {
		r, errR := strconv.ParseUint(match[0:2], 16, 8)
		g, errG := strconv.ParseUint(match[2:4], 16, 8)
		b, errB := strconv.ParseUint(match[4:6], 16, 8)
		if errR == nil && errG == nil && errB == nil {
			return uint8(r), uint8(g), uint8(b), true
		}
	}
	if match := strings.TrimPrefix(value, "#"); len(match) == 3 && len(value) == 4 {
		r, errR := strconv.ParseUint(strings.Repeat(string(match[0]), 2), 16, 8)
		g, errG := strconv.ParseUint(strings.Repeat(string(match[1]), 2), 16, 8)
		b, errB := strconv.ParseUint(strings.Repeat(string(match[2]), 2), 16, 8)
		if errR == nil && errG == nil && errB == nil {
			return uint8(r), uint8(g), uint8(b), true
		}
	}
	return 0, 0, 0, false
}

func normalizeBrowserPaneTheme(theme string) string {
	if strings.EqualFold(strings.TrimSpace(theme), "light") {
		return "light"
	}
	return "dark"
}

func browserPaneBackgroundScript(r, g, b uint8) string {
	return fmt.Sprintf(`(function(){
  try {
    var color = 'rgb(%d,%d,%d)';
    var root = document.documentElement;
    if (root && root.style) {
      root.style.backgroundColor = color;
      root.style.background = color;
    }
    var applyBodyBackground = function() {
      try {
        var body = document.body;
        if (!body || !body.style) return;
        var computed = '';
        if (window.getComputedStyle) {
          computed = window.getComputedStyle(body).backgroundColor || '';
        }
        if (!computed || computed === 'transparent' || computed === 'rgba(0, 0, 0, 0)') {
          body.style.backgroundColor = color;
          body.style.background = color;
        }
      } catch(_){}
    };
    applyBodyBackground();
    if (!window.__wailsBrowserPaneBackgroundInit) {
      window.__wailsBrowserPaneBackgroundInit = true;
      document.addEventListener('DOMContentLoaded', applyBodyBackground, { once: true });
    }
  } catch(_){}
})();`, r, g, b)
}

func browserPaneScrollbarScript(theme string) string {
	thumb := "rgba(121,121,121,0.42)"
	thumbHover := "rgba(121,121,121,0.58)"
	thumbActive := "rgba(121,121,121,0.72)"
	if strings.EqualFold(strings.TrimSpace(theme), "light") {
		thumb = "rgba(100,100,100,0.36)"
		thumbHover = "rgba(100,100,100,0.50)"
		thumbActive = "rgba(100,100,100,0.64)"
	}
	return fmt.Sprintf(`(function(){
var T='%s',TH='%s',TA='%s';
function setup(){
  if(window.__wv2sb) return;
  window.__wv2sb=1;

  var s=document.createElement('style');
  s.id='__wv2sb_css';
  s.textContent=[
    'html,body,*{scrollbar-width:none!important}',
    '*::-webkit-scrollbar{display:none!important;width:0!important;height:0!important}'
  ].join('\n');
  (document.head||document.documentElement).appendChild(s);

  var bar=document.createElement('div');
  bar.id='__wv2sb';
  bar.style.cssText='position:fixed;top:0;right:0;width:12px;height:100%%;pointer-events:none;z-index:2147483647;opacity:0;transition:opacity 0.15s ease;';
  var thumb=document.createElement('div');
  thumb.style.cssText='position:absolute;top:0;right:0;width:12px;min-height:20px;background:'+T+';pointer-events:auto;transition:background-color 0.1s ease;cursor:default;';
  bar.appendChild(thumb);
  document.body.appendChild(bar);

  var timer=null,dragging=false,dragY=0,dragST=0;
  function scrollEl(){return document.scrollingElement||document.documentElement}
  function update(){
    var el=scrollEl(),ch=window.innerHeight,sh=el.scrollHeight;
    if(sh<=ch){bar.style.display='none';return}
    bar.style.display='';
    var tH=Math.max(20,Math.round((ch/sh)*ch));
    var ms=sh-ch,mt=ch-tH;
    var off=ms>0?(el.scrollTop/ms)*mt:0;
    thumb.style.height=tH+'px';
    thumb.style.transform='translateY('+Math.round(off)+'px)';
  }
  function show(){
    bar.style.opacity='1';
    if(timer)clearTimeout(timer);
    timer=setTimeout(function(){if(!dragging)bar.style.opacity='0'},700);
  }

  window.addEventListener('scroll',function(){update();show()},{passive:true});
  window.addEventListener('resize',update,{passive:true});
  window.addEventListener('mousemove',function(e){if(e.clientX>=window.innerWidth-20)show()},{passive:true});

  thumb.addEventListener('mouseenter',function(){thumb.style.background=TH});
  thumb.addEventListener('mouseleave',function(){if(!dragging)thumb.style.background=T});

  thumb.addEventListener('pointerdown',function(e){
    e.preventDefault();dragging=true;dragY=e.clientY;dragST=scrollEl().scrollTop;
    thumb.setPointerCapture(e.pointerId);thumb.style.background=TA;
    bar.style.opacity='1';if(timer)clearTimeout(timer);
  });
  window.addEventListener('pointermove',function(e){
    if(!dragging)return;
    var el=scrollEl(),ch=window.innerHeight,sh=el.scrollHeight;
    var tH=Math.max(20,Math.round((ch/sh)*ch)),mt=ch-tH,ms=sh-ch;
    if(mt<=0)return;
    el.scrollTop=dragST+((e.clientY-dragY)/mt)*ms;
  });
  window.addEventListener('pointerup',function(){
    if(!dragging)return;dragging=false;thumb.style.background=T;show();
  });

  bar.addEventListener('pointerdown',function(e){
    if(e.target===thumb)return;
    var el=scrollEl(),ch=window.innerHeight,sh=el.scrollHeight;
    var tH=Math.max(20,Math.round((ch/sh)*ch)),mt=ch-tH,ms=sh-ch;
    var pos=Math.max(0,Math.min(1,(e.clientY-tH/2)/mt));
    el.scrollTop=pos*ms;update();show();
  });

  if(typeof MutationObserver!=='undefined'){
    new MutationObserver(update).observe(document.body,{childList:true,subtree:true,characterData:true});
  }
  if(typeof ResizeObserver!=='undefined'){
    new ResizeObserver(update).observe(document.body);
  }
  update();
}
if(document.body)setup();else document.addEventListener('DOMContentLoaded',setup);
})();`, thumb, thumbHover, thumbActive)
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

func (s *windowsPaneState) captureScreenshot() (string, error) {
	if s.chromium == nil {
		return "", fmt.Errorf("browser pane not initialized")
	}
	_, webview, _ := chromiumHandles(s.chromium)
	if webview == nil {
		return "", fmt.Errorf("webview not available")
	}
	if !s.initialNavigationDone {
		return "", fmt.Errorf("no page loaded yet")
	}

	raw := (*rawICoreWebView2)(unsafe.Pointer(webview))

	handler := &cdpCallbackHandler{
		vtbl: cdpCallbackVtblInstance,
		ch:   make(chan cdpResult, 1),
	}

	methodName, _ := windows.UTF16PtrFromString("Page.captureScreenshot")
	params, _ := windows.UTF16PtrFromString(`{"format":"jpeg","quality":55,"optimizeForSpeed":true}`)

	hr, _, _ := raw.Vtbl.CallDevToolsProtocolMethod.Call(
		uintptr(unsafe.Pointer(raw)),
		uintptr(unsafe.Pointer(methodName)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(handler)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return "", fmt.Errorf("CallDevToolsProtocolMethod failed: HRESULT 0x%x", hr)
	}

	// Pump messages until the callback fires or timeout.
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case result := <-handler.ch:
			if result.err != nil {
				return "", result.err
			}
			return extractCDPBase64Data(result.json)
		default:
			if time.Now().After(deadline) {
				return "", fmt.Errorf("capture screenshot timed out")
			}
			pumpWindowsMessages()
			time.Sleep(1 * time.Millisecond)
		}
	}
}

// extractCDPBase64Data extracts the "data" field from CDP JSON like {"data":"..."}.
func extractCDPBase64Data(json string) (string, error) {
	const prefix = `"data":"`
	idx := strings.Index(json, prefix)
	if idx < 0 {
		return "", fmt.Errorf("no data field in CDP response")
	}
	start := idx + len(prefix)
	end := strings.Index(json[start:], `"`)
	if end < 0 {
		return "", fmt.Errorf("malformed CDP data field")
	}
	return json[start : start+end], nil
}
