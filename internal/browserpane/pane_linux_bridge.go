//go:build linux && cgo

package browserpane

/*
#include <stdint.h>
*/
import "C"

//export browserPaneLinuxHandleScriptMessage
func browserPaneLinuxHandleScriptMessage(callbackID C.ulonglong, message *C.char) {
	if message == nil {
		return
	}
	dispatchBrowserPaneMessageCallback(uint64(callbackID), C.GoString(message))
}
