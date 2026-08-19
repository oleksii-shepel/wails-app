//go:build windows

package runtime

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func SetupFileDropHandler(hwnd uintptr) error {
	user32 := windows.NewLazyDLL("user32.dll")
	dragAcceptFiles := user32.NewProc("DragAcceptFiles")

	if dragAcceptFiles.Find() != nil {
		return fmt.Errorf("DragAcceptFiles not found")
	}

	dragAcceptFiles.Call(hwnd, 1)
	fmt.Println("[SetupFileDropHandler] File drop handler registered for window")
	return nil
}

func RegisterDropListener() {
}