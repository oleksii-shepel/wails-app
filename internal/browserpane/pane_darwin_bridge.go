//go:build darwin && cgo

package browserpane

/*
#include <stdint.h>
*/
import "C"

//export browserPaneDarwinHandleScriptMessage
func browserPaneDarwinHandleScriptMessage(callbackID C.ulonglong, message *C.char) {
	if message == nil {
		return
	}
	dispatchBrowserPaneMessageCallback(uint64(callbackID), C.GoString(message))
}
