//go:build !windows

package elevation

import "fmt"

func RelaunchCurrentProcessElevated() error {
	return fmt.Errorf("Manager elevation is not implemented on this platform")
}
