//go:build windows

package edge

type iCoreWebView2WebResourceResponseReceivedEventHandlerVtbl struct {
	_IUnknownVtbl
	Invoke ComProc
}

type iCoreWebView2WebResourceResponseReceivedEventHandler struct {
	vtbl *iCoreWebView2WebResourceResponseReceivedEventHandlerVtbl
	impl iCoreWebView2WebResourceResponseReceivedEventHandlerImpl
}

func iCoreWebView2WebResourceResponseReceivedEventHandlerIUnknownQueryInterface(this *iCoreWebView2WebResourceResponseReceivedEventHandler, refiid, object uintptr) uintptr {
	return this.impl.QueryInterface(refiid, object)
}

func iCoreWebView2WebResourceResponseReceivedEventHandlerIUnknownAddRef(this *iCoreWebView2WebResourceResponseReceivedEventHandler) uintptr {
	return this.impl.AddRef()
}

func iCoreWebView2WebResourceResponseReceivedEventHandlerIUnknownRelease(this *iCoreWebView2WebResourceResponseReceivedEventHandler) uintptr {
	return this.impl.Release()
}

func iCoreWebView2WebResourceResponseReceivedEventHandlerInvoke(this *iCoreWebView2WebResourceResponseReceivedEventHandler, sender *ICoreWebView2, args *ICoreWebView2WebResourceResponseReceivedEventArgs) uintptr {
	return this.impl.WebResourceResponseReceived(sender, args)
}

type iCoreWebView2WebResourceResponseReceivedEventHandlerImpl interface {
	_IUnknownImpl
	WebResourceResponseReceived(sender *ICoreWebView2, args *ICoreWebView2WebResourceResponseReceivedEventArgs) uintptr
}

var iCoreWebView2WebResourceResponseReceivedEventHandlerFn = iCoreWebView2WebResourceResponseReceivedEventHandlerVtbl{
	_IUnknownVtbl{
		NewComProc(iCoreWebView2WebResourceResponseReceivedEventHandlerIUnknownQueryInterface),
		NewComProc(iCoreWebView2WebResourceResponseReceivedEventHandlerIUnknownAddRef),
		NewComProc(iCoreWebView2WebResourceResponseReceivedEventHandlerIUnknownRelease),
	},
	NewComProc(iCoreWebView2WebResourceResponseReceivedEventHandlerInvoke),
}

func newICoreWebView2WebResourceResponseReceivedEventHandler(impl iCoreWebView2WebResourceResponseReceivedEventHandlerImpl) *iCoreWebView2WebResourceResponseReceivedEventHandler {
	return &iCoreWebView2WebResourceResponseReceivedEventHandler{
		vtbl: &iCoreWebView2WebResourceResponseReceivedEventHandlerFn,
		impl: impl,
	}
}