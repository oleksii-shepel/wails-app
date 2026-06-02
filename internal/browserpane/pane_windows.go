//go:build windows

package browserpane

import (
	"encoding/json"
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

	"wails-app/internal/webresourcecache"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

const (
	wsChild         = 0x40000000
	wsClipChildren  = 0x02000000
	wsClipSiblings  = 0x04000000
	offscreenX      = -32000
	offscreenY      = -32000
	swHide          = 0
	swShow          = 5
	swShowNA        = 8
	swpNoActivate   = 0x0010
	swpNoZOrder     = 0x0004
	pmRemove        = 0x0001
	gwOwner         = 4
	htClient        = 1
	tmeLeave        = 0x00000002
	tmeCancel       = 0x80000000
	wmEraseBkgnd    = 0x0014
	wmSetCursor     = 0x0020
	wmClose         = 0x0010
	wmDestroy       = 0x0002
	wmMouseMove     = 0x0200
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonDown   = 0x0204
	wmRButtonUp     = 0x0205
	wmRButtonDblClk = 0x0206
	wmMButtonDown   = 0x0207
	wmMButtonUp     = 0x0208
	wmMButtonDblClk = 0x0209
	wmMouseWheel    = 0x020A
	wmXButtonDown   = 0x020B
	wmXButtonUp     = 0x020C
	wmXButtonDblClk = 0x020D
	wmMouseHWheel   = 0x020E
	wmMouseLeave    = 0x02A3
	wmNCDestroy     = 0x0082
)

var (
	user32                        = windows.NewLazySystemDLL("user32.dll")
	kernel32                      = windows.NewLazySystemDLL("kernel32.dll")
	gdi32                         = windows.NewLazySystemDLL("gdi32.dll")
	dcomp                         = windows.NewLazySystemDLL("dcomp.dll")
	procRegisterClassExW          = user32.NewProc("RegisterClassExW")
	procCreateWindowExW           = user32.NewProc("CreateWindowExW")
	procDefWindowProcW            = user32.NewProc("DefWindowProcW")
	procDestroyWindow             = user32.NewProc("DestroyWindow")
	procShowWindow                = user32.NewProc("ShowWindow")
	procUpdateWindow              = user32.NewProc("UpdateWindow")
	procInvalidateRect            = user32.NewProc("InvalidateRect")
	procSetWindowPos              = user32.NewProc("SetWindowPos")
	procPeekMessageW              = user32.NewProc("PeekMessageW")
	procTranslateMessage          = user32.NewProc("TranslateMessage")
	procDispatchMessageW          = user32.NewProc("DispatchMessageW")
	procEnumThreadWindows         = user32.NewProc("EnumThreadWindows")
	procEnumWindows               = user32.NewProc("EnumWindows")
	procGetWindowThreadPID        = user32.NewProc("GetWindowThreadProcessId")
	procGetWindow                 = user32.NewProc("GetWindow")
	procIsWindowVisible           = user32.NewProc("IsWindowVisible")
	procIsWindow                  = user32.NewProc("IsWindow")
	procGetCapture                = user32.NewProc("GetCapture")
	procGetClientRect             = user32.NewProc("GetClientRect")
	procClientToScreen            = user32.NewProc("ClientToScreen")
	procScreenToClient            = user32.NewProc("ScreenToClient")
	procPostMessageW              = user32.NewProc("PostMessageW")
	procSetCapture                = user32.NewProc("SetCapture")
	procReleaseCapture            = user32.NewProc("ReleaseCapture")
	procSetCursor                 = user32.NewProc("SetCursor")
	procSetFocus                  = user32.NewProc("SetFocus")
	procTrackMouseEvent           = user32.NewProc("TrackMouseEvent")
	procGetModuleHandleW          = kernel32.NewProc("GetModuleHandleW")
	procGetCurrentThreadID        = kernel32.NewProc("GetCurrentThreadId")
	procCreateSolidBrush          = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject              = gdi32.NewProc("DeleteObject")
	procFillRect                  = user32.NewProc("FillRect")
	procDCompositionCreateDevice2 = dcomp.NewProc("DCompositionCreateDevice2")
	registerPaneClassOnce         sync.Once
	registerPaneClassErr          error
	mainWindowHandleOnce          sync.Once
	mainWindowHandle              uintptr
	mainWindowHandleErr           error
	browserPaneHostColors         sync.Map
	browserPaneStates             sync.Map
	browserPaneWndProc            = windows.NewCallback(browserPaneWindowProc)
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

type trackMouseEvent struct {
	Size      uint32
	Flags     uint32
	HWndTrack uintptr
	HoverTime uint32
}

type windowsPane struct {
	surfaceID string
	commands  chan windowsPaneCommand
	closed    chan struct{}
}

type windowsPaneCommand struct {
	update                  *Update
	dispose                 bool
	capture                 bool
	documentBackground      bool
	postMessage             *string
	result                  chan error
	captureReply            chan string
	documentBackgroundReply chan string
}

type windowsPaneState struct {
	ownerHWND               uintptr
	hostHWND                uintptr
	chromium                *edge.Chromium
	compositionHost         *browserPaneCompositionHost
	compositionTracking     bool
	compositionCapturing    bool
	url                     string
	theme                   string
	backgroundColor         string
	visible                 bool
	captureOnly             bool
	targetVisible           bool
	initialNavigationDone   bool
	initialNavigationQueued bool
	pendingMessages         []string
	newWindowHandler        *newWindowRequestedHandler
	onResourcesChanged      func(string)
	onMessage               func(string)
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

type rawDCompositionDesktopDeviceVtbl struct {
	rawIUnknownVTBL
	Commit                     edge.ComProc
	WaitForCommitCompletion    edge.ComProc
	GetFrameStatistics         edge.ComProc
	CreateVisual               edge.ComProc
	CreateSurfaceFactory       edge.ComProc
	CreateSurface              edge.ComProc
	CreateVirtualSurface       edge.ComProc
	CreateTranslateTransform   edge.ComProc
	CreateScaleTransform       edge.ComProc
	CreateRotateTransform      edge.ComProc
	CreateSkewTransform        edge.ComProc
	CreateMatrixTransform      edge.ComProc
	CreateTransformGroup       edge.ComProc
	CreateTranslateTransform3D edge.ComProc
	CreateScaleTransform3D     edge.ComProc
	CreateRotateTransform3D    edge.ComProc
	CreateMatrixTransform3D    edge.ComProc
	CreateTransform3DGroup     edge.ComProc
	CreateEffectGroup          edge.ComProc
	CreateRectangleClip        edge.ComProc
	CreateAnimation            edge.ComProc
	CreateTargetForHwnd        edge.ComProc
	CreateSurfaceFromHandle    edge.ComProc
	CreateSurfaceFromHwnd      edge.ComProc
}

type rawDCompositionDesktopDevice struct {
	Vtbl *rawDCompositionDesktopDeviceVtbl
}

type rawDCompositionTargetVtbl struct {
	rawIUnknownVTBL
	SetRoot edge.ComProc
}

type rawDCompositionTarget struct {
	Vtbl *rawDCompositionTargetVtbl
}

type rawDCompositionVisualVtbl struct {
	rawIUnknownVTBL
	SetOffsetXFloat            edge.ComProc
	SetOffsetXAnimation        edge.ComProc
	SetOffsetYFloat            edge.ComProc
	SetOffsetYAnimation        edge.ComProc
	SetTransformMatrix         edge.ComProc
	SetTransformObject         edge.ComProc
	SetTransformParent         edge.ComProc
	SetEffect                  edge.ComProc
	SetBitmapInterpolationMode edge.ComProc
	SetBorderMode              edge.ComProc
	SetClipRect                edge.ComProc
	SetClipObject              edge.ComProc
	SetContent                 edge.ComProc
	AddVisual                  edge.ComProc
	RemoveVisual               edge.ComProc
	RemoveAllVisuals           edge.ComProc
	SetCompositeMode           edge.ComProc
}

type rawDCompositionVisual struct {
	Vtbl *rawDCompositionVisualVtbl
}

type browserPaneCompositionHost struct {
	device        *rawDCompositionDesktopDevice
	target        *rawDCompositionTarget
	rootVisual    *rawDCompositionVisual
	webViewVisual *rawDCompositionVisual
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

// rawICoreWebView2NewWindowVtbl extends rawICoreWebView2Vtbl to include
// AddNewWindowRequested (vtable index 44, counting from IUnknown at 0).
type rawICoreWebView2NewWindowVtbl struct {
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
	GetBrowserProcessID                    edge.ComProc
	GetCanGoBack                           edge.ComProc
	GetCanGoForward                        edge.ComProc
	GoBack                                 edge.ComProc
	GoForward                              edge.ComProc
	GetDevToolsProtocolEventReceiver       edge.ComProc
	Stop                                   edge.ComProc
	AddNewWindowRequested                  edge.ComProc
}

type rawICoreWebView2NewWindow struct {
	Vtbl *rawICoreWebView2NewWindowVtbl
}

// rawNewWindowArgsVtbl covers ICoreWebView2NewWindowRequestedEventArgs up to PutHandled.
type rawNewWindowArgsVtbl struct {
	QueryInterface edge.ComProc
	AddRef         edge.ComProc
	Release        edge.ComProc
	GetUri         edge.ComProc
	PutNewWindow   edge.ComProc
	GetNewWindow   edge.ComProc
	PutHandled     edge.ComProc
}

type rawNewWindowArgs struct {
	Vtbl *rawNewWindowArgsVtbl
}

// newWindowRequestedHandlerVtbl is the COM vtable for our NewWindowRequested event handler.
type newWindowRequestedHandlerVtbl struct {
	QueryInterface edge.ComProc
	AddRef         edge.ComProc
	Release        edge.ComProc
	Invoke         edge.ComProc
}

// newWindowRequestedHandler is a COM-compatible event handler that intercepts
// new-window requests and redirects them to the same WebView2 pane.
type newWindowRequestedHandler struct {
	vtbl  *newWindowRequestedHandlerVtbl
	state *windowsPaneState
}

var newWindowRequestedVtblSingleton *newWindowRequestedHandlerVtbl

func init() {
	newWindowRequestedVtblSingleton = &newWindowRequestedHandlerVtbl{
		QueryInterface: edge.NewComProc(nwrQueryInterface),
		AddRef:         edge.NewComProc(nwrAddRef),
		Release:        edge.NewComProc(nwrRelease),
		Invoke:         edge.NewComProc(nwrInvoke),
	}
}

func nwrQueryInterface(this, _, _ uintptr) uintptr { return 0x80004002 } // E_NOINTERFACE
func nwrAddRef(this uintptr) uintptr               { return 1 }
func nwrRelease(this uintptr) uintptr              { return 1 }

func nwrInvoke(this, _ /*sender*/, args uintptr) uintptr {
	if args == 0 {
		return 0
	}
	// this was originally created from a *newWindowRequestedHandler that is
	// kept alive by windowsPaneState.newWindowHandler, so the conversion is safe.
	handler := (*newWindowRequestedHandler)(unsafe.Pointer(this))

	// Read the requested URI.
	argsRaw := (*rawNewWindowArgs)(unsafe.Pointer(args))
	var uriPtr *uint16
	argsRaw.Vtbl.GetUri.Call(args, uintptr(unsafe.Pointer(&uriPtr)))
	url := ""
	if uriPtr != nil {
		url = windows.UTF16PtrToString(uriPtr)
		windows.CoTaskMemFree(unsafe.Pointer(uriPtr))
	}

	// Navigate the existing webview to the requested URL instead of opening a new window.
	if url != "" && handler.state != nil && handler.state.chromium != nil {
		_, webview, _ := chromiumHandles(handler.state.chromium)
		if webview != nil {
			raw := (*rawICoreWebView2)(unsafe.Pointer(webview))
			if urlUTF16, err := windows.UTF16PtrFromString(url); err == nil {
				raw.Vtbl.Navigate.Call(uintptr(unsafe.Pointer(raw)), uintptr(unsafe.Pointer(urlUTF16)))
			}
		}
	}

	// Mark the event as handled — prevents WebView2 from opening a native new window.
	argsRaw.Vtbl.PutHandled.Call(args, 1)
	return 0 // S_OK
}

func (s *windowsPaneState) registerNewWindowHandler() {
	_, webview, _ := chromiumHandles(s.chromium)
	if webview == nil {
		return
	}
	s.newWindowHandler = &newWindowRequestedHandler{
		vtbl:  newWindowRequestedVtblSingleton,
		state: s,
	}
	raw := (*rawICoreWebView2NewWindow)(unsafe.Pointer(webview))
	var token struct{ Value int64 }
	raw.Vtbl.AddNewWindowRequested.Call(
		uintptr(unsafe.Pointer(raw)),
		uintptr(unsafe.Pointer(s.newWindowHandler)),
		uintptr(unsafe.Pointer(&token)),
	)
	runtime.KeepAlive(s.newWindowHandler)
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
	// this was originally created from a *cdpCallbackHandler that is kept alive
	// by the caller until the callback fires, so the conversion is safe.
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

func newPlatformPane(initial Update, onResourcesChanged func(string), onMessage func(string)) (pane, error) {
	result := &windowsPane{
		surfaceID: initial.SurfaceID,
		commands:  make(chan windowsPaneCommand),
		closed:    make(chan struct{}),
	}
	go result.run(initial, onResourcesChanged, onMessage)
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

func (p *windowsPane) GetDocumentBackground() (string, error) {
	reply := make(chan error, 1)
	backgroundReply := make(chan string, 1)
	select {
	case <-p.closed:
		return "", fmt.Errorf("browser pane closed")
	case p.commands <- windowsPaneCommand{
		documentBackground:      true,
		result:                  reply,
		documentBackgroundReply: backgroundReply,
	}:
	}
	err := <-reply
	if err != nil {
		return "", err
	}
	return <-backgroundReply, nil
}

func (p *windowsPane) PostMessage(message string) error {
	reply := make(chan error, 1)
	select {
	case <-p.closed:
		return fmt.Errorf("browser pane closed")
	case p.commands <- windowsPaneCommand{postMessage: &message, result: reply}:
	}
	return <-reply
}

func (p *windowsPane) run(initial Update, onResourcesChanged func(string), onMessage func(string)) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(p.closed)

	if err := initializeBrowserPaneCOM(); err != nil {
		p.drainWithError(err)
		return
	}
	defer windows.CoUninitialize()

	state, err := newWindowsPaneState(initial.Theme, initial.BackgroundColor, onResourcesChanged, onMessage)
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
			if command.documentBackground {
				color, err := state.getDocumentBackground()
				command.result <- err
				if err == nil {
					command.documentBackgroundReply <- color
				}
				continue
			}
			if command.postMessage != nil {
				command.result <- state.postMessage(*command.postMessage)
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

func newWindowsPaneState(initialTheme string, initialBackgroundColor string, onResourcesChanged func(string), onMessage func(string)) (*windowsPaneState, error) {
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
		ownerHWND:          ownerHWND,
		hostHWND:           hostHWND,
		chromium:           chromium,
		theme:              initialTheme,
		backgroundColor:    initialBackgroundColor,
		onResourcesChanged: onResourcesChanged,
		onMessage:          onMessage,
	}
	compositionHost, compositionErr := createBrowserPaneCompositionHost(hostHWND)
	if compositionErr != nil {
		log.Printf("[BrowserPane] Composition host unavailable, using windowed WebView2 hosting: %v", compositionErr)
	} else {
		state.compositionHost = compositionHost
		chromium.UseComposition = true
		chromium.CompositionRootTarget = unsafe.Pointer(compositionHost.webViewVisual)
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
		state.chromium.Eval(`window.__wailsBrowserPaneFlushResources&&window.__wailsBrowserPaneFlushResources()`)
		if err := state.flushPendingMessages(); err != nil {
			log.Printf("[BrowserPane] Failed to flush pending messages: %v", err)
		}
		if state.targetVisible && !state.visible {
			state.show()
		}
	}
	chromium.MessageCallback = func(message string, _ *edge.ICoreWebView2, _ *edge.ICoreWebView2WebMessageReceivedEventArgs) {
		state.handleBrowserMessage(message)
	}
	chromium.WebResourceResponseReceivedCallback = func(request *edge.ICoreWebView2WebResourceRequest, response *edge.ICoreWebView2WebResourceResponseView, _ *edge.ICoreWebView2WebResourceResponseReceivedEventArgs) {
		state.handleBrowserResourceResponse(request, response)
	}
	chromium.SetGlobalPermission(edge.CoreWebView2PermissionStateAllow)
	if !chromium.Embed(hostHWND) {
		if chromiumErr == nil {
			chromiumErr = fmt.Errorf("embed browser pane: WebView2 controller initialization failed")
		}
	}
	if chromiumErr == nil && state.compositionHost != nil {
		if chromium.GetCompositionController() == nil {
			state.compositionHost.release()
			state.compositionHost = nil
		} else if err := state.compositionHost.commit(); err != nil {
			chromiumErr = fmt.Errorf("commit browser pane composition host: %w", err)
		}
	}
	if chromiumErr != nil {
		if state.compositionHost != nil {
			state.compositionHost.release()
			state.compositionHost = nil
		}
		clearBrowserPaneHostBackground(hostHWND)
		_, _, _ = procDestroyWindow.Call(hostHWND)
		return nil, chromiumErr
	}
	browserPaneStates.Store(hostHWND, state)

	// Inject scripts before first page render via AddScriptToExecuteOnDocumentCreated.
	// These run on every navigation, before the page content is displayed.
	chromium.Init(browserPaneScrollbarScript(initialTheme))
	chromium.Init(browserPaneBackgroundScript(r, g, b))
	chromium.Init(browserPaneNewWindowScript())
	chromium.Init(browserPaneInjectedBootstrapScript())
	state.registerNewWindowHandler()

	state.chromium.SetBackgroundColour(r, g, b, 255)
	_, _, _ = procShowWindow.Call(hostHWND, swHide)

	return state, nil
}

func (s *windowsPaneState) handleBrowserMessage(message string) {
	handleBrowserPaneBridgeMessage(message, s.onResourcesChanged, s.onMessage)
}

func (s *windowsPaneState) handleBrowserResourceResponse(request *edge.ICoreWebView2WebResourceRequest, response *edge.ICoreWebView2WebResourceResponseView) {
	if s == nil || request == nil || response == nil || strings.TrimSpace(s.url) == "" {
		return
	}
	requestURL, err := request.GetUri()
	if err != nil {
		return
	}
	headers, err := response.GetHeaders()
	if err != nil || headers == nil {
		return
	}
	defer headers.Release()

	contentLength, _ := headers.GetHeader("Content-Length")
	contentRange, _ := headers.GetHeader("Content-Range")
	contentType, _ := headers.GetHeader("Content-Type")
	size := parseBrowserPaneResponseSize(contentLength, contentRange)
	if !webresourcecache.Merge(s.url, webresourcecache.Resource{
		URL:      requestURL,
		Size:     size,
		MimeType: normalizeBrowserPaneMimeType(contentType),
	}) {
		return
	}
	if s.onResourcesChanged != nil {
		s.onResourcesChanged(s.url)
	}
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

	if update.URL != "" && s.url != update.URL {
		webresourcecache.Clear(update.URL)
		s.initialNavigationDone = false
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

func parseBrowserPaneResponseSize(contentLength string, contentRange string) int64 {
	if trimmed := strings.TrimSpace(contentRange); trimmed != "" {
		if slash := strings.LastIndex(trimmed, "/"); slash >= 0 && slash < len(trimmed)-1 {
			if size, err := strconv.ParseInt(strings.TrimSpace(trimmed[slash+1:]), 10, 64); err == nil {
				return size
			}
		}
	}
	if trimmed := strings.TrimSpace(contentLength); trimmed != "" {
		if size, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			return size
		}
	}
	return 0
}

func normalizeBrowserPaneMimeType(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return ""
	}
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}
	return strings.ToLower(strings.TrimSpace(contentType))
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
	browserPaneStates.Delete(s.hostHWND)
	if capture, _, _ := procGetCapture.Call(); capture == s.hostHWND {
		_, _, _ = procReleaseCapture.Call()
	}

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
		s.chromium = nil
	}
	if s.compositionHost != nil {
		s.compositionHost.release()
		s.compositionHost = nil
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
	case wmSetCursor:
		if state := browserPaneStateForHWND(hwnd); state != nil && state.handleCompositionCursor(lParam) {
			return 1
		}
	case wmMouseMove, wmLButtonDown, wmLButtonUp, wmLButtonDblClk, wmRButtonDown, wmRButtonUp, wmRButtonDblClk,
		wmMButtonDown, wmMButtonUp, wmMButtonDblClk, wmXButtonDown, wmXButtonUp, wmXButtonDblClk, wmMouseWheel,
		wmMouseHWheel, wmMouseLeave:
		if state := browserPaneStateForHWND(hwnd); state != nil && state.handleCompositionMouse(msg, wParam, lParam) {
			return 0
		}
	case wmEraseBkgnd:
		if paintBrowserPaneBackground(hwnd, wParam) {
			return 1
		}
	case wmNCDestroy:
		browserPaneStates.Delete(hwnd)
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
	_ = theme
	return 255, 255, 255
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
    var parseColor = function(value) {
      if (!value || value === 'transparent') return null;
      var match = value.match(/^rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})(?:\s*,\s*([\d.]+)\s*)?\)$/i);
      if (!match) return null;
      return {
        r: Math.max(0, Math.min(255, parseInt(match[1], 10) || 0)),
        g: Math.max(0, Math.min(255, parseInt(match[2], 10) || 0)),
        b: Math.max(0, Math.min(255, parseInt(match[3], 10) || 0)),
        a: Math.max(0, Math.min(1, match[4] == null ? 1 : parseFloat(match[4]) || 0))
      };
    };
    var isTransparent = function(value) {
      var parsed = parseColor(value);
      return !parsed || parsed.a <= 0;
    };
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
        if (isTransparent(computed)) {
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

func browserPaneNewWindowScript() string {
	return `(function(){
  if(window.__wailsBrowserPaneNewWindowInit){return;}
  window.__wailsBrowserPaneNewWindowInit=true;
  // Redirect window.open() to navigate in the same pane.
  var _origOpen=window.open;
  window.open=function(url,target,features){
    if(url&&url!=='about:blank'){
      try{window.location.href=String(url);}catch(_){}
      return null;
    }
    return _origOpen.call(this,url,target,features);
  };
  // Intercept clicks on target=_blank links before the browser handles them.
  document.addEventListener('click',function(e){
    if(e.defaultPrevented){return;}
    var el=e.target;
    while(el&&el.tagName!=='A'){el=el.parentElement;}
    if(!el){return;}
    var t=el.getAttribute('target');
    if(t&&t!=='_self'&&t!=='_parent'&&t!=='_top'&&el.href&&el.href!=='#'){
      e.preventDefault();
      try{window.location.href=el.href;}catch(_){}
    }
  },true);
})();`
}

func (s *windowsPaneState) postMessage(message string) error {
	if strings.TrimSpace(s.url) == "" {
		return fmt.Errorf("browser pane has no active page")
	}
	if !s.initialNavigationDone {
		s.pendingMessages = append(s.pendingMessages, message)
		return nil
	}
	return s.evaluateScript(browserPaneHostMessageEvalScript(message))
}

func (s *windowsPaneState) flushPendingMessages() error {
	if len(s.pendingMessages) == 0 {
		return nil
	}
	pending := append([]string(nil), s.pendingMessages...)
	s.pendingMessages = s.pendingMessages[:0]
	for idx, message := range pending {
		if err := s.evaluateScript(browserPaneHostMessageEvalScript(message)); err != nil {
			s.pendingMessages = append(pending[idx:], s.pendingMessages...)
			return err
		}
	}
	return nil
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
	return chromium.GetController(), chromium.GetWebView(), chromium.Environment()
}

func clearChromiumHandles(chromium *edge.Chromium) {
	_ = chromium
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

func createBrowserPaneCompositionHost(hostHWND uintptr) (*browserPaneCompositionHost, error) {
	if hostHWND == 0 {
		return nil, fmt.Errorf("invalid browser pane host window")
	}
	var device *rawDCompositionDesktopDevice
	iid := edge.NewGUID("{5F4633FE-1E08-4CB8-8C75-CE24333F5602}")
	hr, _, _ := procDCompositionCreateDevice2.Call(
		0,
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&device)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return nil, windows.Errno(hr)
	}
	host := &browserPaneCompositionHost{device: device}
	defer func() {
		if host != nil {
			host.release()
		}
	}()

	hr, _, _ = device.Vtbl.CreateTargetForHwnd.Call(
		uintptr(unsafe.Pointer(device)),
		hostHWND,
		1,
		uintptr(unsafe.Pointer(&host.target)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return nil, windows.Errno(hr)
	}
	hr, _, _ = device.Vtbl.CreateVisual.Call(
		uintptr(unsafe.Pointer(device)),
		uintptr(unsafe.Pointer(&host.rootVisual)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return nil, windows.Errno(hr)
	}
	hr, _, _ = device.Vtbl.CreateVisual.Call(
		uintptr(unsafe.Pointer(device)),
		uintptr(unsafe.Pointer(&host.webViewVisual)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return nil, windows.Errno(hr)
	}
	hr, _, _ = host.target.Vtbl.SetRoot.Call(
		uintptr(unsafe.Pointer(host.target)),
		uintptr(unsafe.Pointer(host.rootVisual)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return nil, windows.Errno(hr)
	}
	hr, _, _ = host.rootVisual.Vtbl.AddVisual.Call(
		uintptr(unsafe.Pointer(host.rootVisual)),
		uintptr(unsafe.Pointer(host.webViewVisual)),
		1,
		0,
	)
	if windows.Handle(hr) != windows.S_OK {
		return nil, windows.Errno(hr)
	}
	if err := host.commit(); err != nil {
		return nil, err
	}

	result := host
	host = nil
	return result, nil
}

func (h *browserPaneCompositionHost) commit() error {
	if h == nil || h.device == nil {
		return nil
	}
	hr, _, _ := h.device.Vtbl.Commit.Call(uintptr(unsafe.Pointer(h.device)))
	if windows.Handle(hr) != windows.S_OK {
		return windows.Errno(hr)
	}
	return nil
}

func (h *browserPaneCompositionHost) release() {
	if h == nil {
		return
	}
	if h.webViewVisual != nil {
		releaseCOMObject(unsafe.Pointer(h.webViewVisual))
		h.webViewVisual = nil
	}
	if h.rootVisual != nil {
		releaseCOMObject(unsafe.Pointer(h.rootVisual))
		h.rootVisual = nil
	}
	if h.target != nil {
		releaseCOMObject(unsafe.Pointer(h.target))
		h.target = nil
	}
	if h.device != nil {
		releaseCOMObject(unsafe.Pointer(h.device))
		h.device = nil
	}
}

func browserPaneStateForHWND(hwnd uintptr) *windowsPaneState {
	if hwnd == 0 {
		return nil
	}
	value, ok := browserPaneStates.Load(hwnd)
	if !ok {
		return nil
	}
	state, _ := value.(*windowsPaneState)
	return state
}

func (s *windowsPaneState) compositionController() *edge.ICoreWebView2CompositionController {
	if s == nil || s.chromium == nil {
		return nil
	}
	return s.chromium.GetCompositionController()
}

func (s *windowsPaneState) handleCompositionCursor(lParam uintptr) bool {
	controller := s.compositionController()
	if controller == nil || loword(uint32(lParam)) != htClient {
		return false
	}
	cursor, err := controller.GetCursor()
	if err != nil || cursor == 0 {
		return false
	}
	_, _, _ = procSetCursor.Call(uintptr(cursor))
	return true
}

func (s *windowsPaneState) handleCompositionMouse(message uint32, wParam uintptr, lParam uintptr) bool {
	controller := s.compositionController()
	if controller == nil {
		return false
	}
	point, hasPoint := compositionPointForMessage(s.hostHWND, message, lParam)
	inside := hasPoint && pointInsideClient(s.hostHWND, point)

	if message == wmMouseMove {
		if inside {
			if !s.compositionTracking {
				startTrackMouseLeave(s.hostHWND)
				s.compositionTracking = true
			}
		} else if s.compositionTracking && !s.compositionCapturing {
			s.compositionTracking = false
			cancelTrackMouseLeave(s.hostHWND)
			_ = controller.SendMouseInput(edge.COREWEBVIEW2_MOUSE_EVENT_KIND_LEAVE, 0, 0, edge.POINT{})
			return false
		}
	}
	if message == wmMouseLeave {
		s.compositionTracking = false
	}
	if !inside && !s.compositionCapturing && message != wmMouseLeave {
		return false
	}
	if isCompositionMouseDown(message) {
		_, _, _ = procSetFocus.Call(s.hostHWND)
		s.chromium.Focus()
		if capture, _, _ := procGetCapture.Call(); capture != s.hostHWND {
			_, _, _ = procSetCapture.Call(s.hostHWND)
		}
		s.compositionCapturing = true
	}
	if isCompositionMouseUp(message) {
		if capture, _, _ := procGetCapture.Call(); capture == s.hostHWND {
			_, _, _ = procReleaseCapture.Call()
		}
		s.compositionCapturing = false
	}

	virtualKeys := edge.COREWEBVIEW2_MOUSE_EVENT_VIRTUAL_KEYS(uint32(wParam) & 0xffff)
	mouseData := compositionMouseData(message, wParam)
	compositionPoint := edge.POINT{}
	if message != wmMouseLeave && hasPoint {
		compositionPoint = edge.POINT{X: point.X, Y: point.Y}
	}
	if err := controller.SendMouseInput(edge.COREWEBVIEW2_MOUSE_EVENT_KIND(message), virtualKeys, mouseData, compositionPoint); err != nil {
		log.Printf("[BrowserPane] Failed to forward mouse input to composition WebView2 host: %v", err)
		return false
	}
	return true
}

func compositionPointForMessage(hwnd uintptr, message uint32, lParam uintptr) (point, bool) {
	if message == wmMouseLeave {
		return point{}, false
	}
	pt := point{X: int32(int16(lParam & 0xffff)), Y: int32(int16((lParam >> 16) & 0xffff))}
	if message == wmMouseWheel || message == wmMouseHWheel {
		_, _, _ = procScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
	}
	return pt, true
}

func pointInsideClient(hwnd uintptr, pt point) bool {
	client := rect{}
	result, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	if result == 0 {
		return false
	}
	return pt.X >= client.Left && pt.Y >= client.Top && pt.X < client.Right && pt.Y < client.Bottom
}

func startTrackMouseLeave(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	track := trackMouseEvent{Size: uint32(unsafe.Sizeof(trackMouseEvent{})), Flags: tmeLeave, HWndTrack: hwnd}
	_, _, _ = procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&track)))
}

func cancelTrackMouseLeave(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	track := trackMouseEvent{Size: uint32(unsafe.Sizeof(trackMouseEvent{})), Flags: tmeLeave | tmeCancel, HWndTrack: hwnd}
	_, _, _ = procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&track)))
}

func compositionMouseData(message uint32, wParam uintptr) uint32 {
	switch message {
	case wmMouseWheel, wmMouseHWheel:
		return uint32(int32(int16((wParam >> 16) & 0xffff)))
	case wmXButtonDown, wmXButtonUp, wmXButtonDblClk:
		return uint32((wParam >> 16) & 0xffff)
	default:
		return 0
	}
}

func isCompositionMouseDown(message uint32) bool {
	switch message {
	case wmLButtonDown, wmLButtonDblClk, wmRButtonDown, wmRButtonDblClk, wmMButtonDown, wmMButtonDblClk, wmXButtonDown, wmXButtonDblClk:
		return true
	default:
		return false
	}
}

func isCompositionMouseUp(message uint32) bool {
	switch message {
	case wmLButtonUp, wmRButtonUp, wmMButtonUp, wmXButtonUp:
		return true
	default:
		return false
	}
}

func loword(value uint32) uint16 {
	return uint16(value & 0xffff)
}

func (s *windowsPaneState) captureScreenshot() (string, error) {
	response, err := s.callDevToolsMethod("Page.captureScreenshot", `{"format":"png","optimizeForSpeed":true}`)
	if err != nil {
		return "", err
	}
	return extractCDPBase64Data(response)
}

func (s *windowsPaneState) getDocumentBackground() (string, error) {
	response, err := s.callDevToolsMethod("Runtime.evaluate", browserPaneDocumentBackgroundParams())
	if err != nil {
		return "", err
	}
	return extractCDPStringValue(response)
}

func (s *windowsPaneState) evaluateScript(script string) error {
	_, err := s.callDevToolsMethod("Runtime.evaluate", browserPaneRuntimeEvaluateParams(script))
	return err
}

func (s *windowsPaneState) callDevToolsMethod(method string, params string) (string, error) {
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

	methodName, err := windows.UTF16PtrFromString(method)
	if err != nil {
		return "", err
	}
	paramsUTF16, err := windows.UTF16PtrFromString(params)
	if err != nil {
		return "", err
	}

	hr, _, _ := raw.Vtbl.CallDevToolsProtocolMethod.Call(
		uintptr(unsafe.Pointer(raw)),
		uintptr(unsafe.Pointer(methodName)),
		uintptr(unsafe.Pointer(paramsUTF16)),
		uintptr(unsafe.Pointer(handler)),
	)
	runtime.KeepAlive(handler)
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
			return result.json, nil
		default:
			if time.Now().After(deadline) {
				return "", fmt.Errorf("DevTools method %q timed out", method)
			}
			pumpWindowsMessages()
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func browserPaneDocumentBackgroundParams() string {
	return browserPaneRuntimeEvaluateParams(`(function(){
  try {
    var parseColor = function(value) {
      if (!value || value === 'transparent') return null;
      var match = value.match(/^rgba?\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})(?:\s*,\s*([\d.]+)\s*)?\)$/i);
      if (!match) return null;
      return {
        r: Math.max(0, Math.min(255, parseInt(match[1], 10) || 0)),
        g: Math.max(0, Math.min(255, parseInt(match[2], 10) || 0)),
        b: Math.max(0, Math.min(255, parseInt(match[3], 10) || 0)),
        a: Math.max(0, Math.min(1, match[4] == null ? 1 : parseFloat(match[4]) || 0))
      };
    };
    var read = function(target) {
      if (!target || !window.getComputedStyle) return '';
      return window.getComputedStyle(target).backgroundColor || '';
    };
    var format = function(color) {
      if (!color || color.a <= 0) return '';
      if (color.a >= 1) return 'rgb(' + color.r + ', ' + color.g + ', ' + color.b + ')';
      return 'rgba(' + color.r + ', ' + color.g + ', ' + color.b + ', ' + color.a + ')';
    };
    var composite = function(fg, bg) {
      if (!fg) return bg;
      if (!bg || fg.a >= 1) return fg;
      if (fg.a <= 0) return bg;
      var alpha = Math.max(0, Math.min(1, fg.a));
      return {
        r: Math.round(fg.r * alpha + bg.r * (1 - alpha)),
        g: Math.round(fg.g * alpha + bg.g * (1 - alpha)),
        b: Math.round(fg.b * alpha + bg.b * (1 - alpha)),
        a: 1
      };
    };
    var body = parseColor(read(document.body));
    var root = parseColor(read(document.documentElement));
    if (body && body.a > 0) {
      return format(root && body.a < 1 ? composite(body, root) : body);
    }
    if (root && root.a > 0) {
      return format(root);
    }
    return '';
  } catch (_) {
    return '';
  }
})()`)
}

func browserPaneRuntimeEvaluateParams(expression string) string {
	payload, err := json.Marshal(map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return `{"expression":"''","returnByValue":true,"awaitPromise":true}`
	}
	return string(payload)
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

func extractCDPStringValue(raw string) (string, error) {
	type runtimeEvaluateResult struct {
		Result struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"result"`
		ExceptionDetails any `json:"exceptionDetails"`
	}

	var parsed runtimeEvaluateResult
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", fmt.Errorf("decode Runtime.evaluate response: %w", err)
	}
	if parsed.ExceptionDetails != nil {
		return "", fmt.Errorf("Runtime.evaluate returned an exception")
	}
	return strings.TrimSpace(parsed.Result.Value), nil
}
