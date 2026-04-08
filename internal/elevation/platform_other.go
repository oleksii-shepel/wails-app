//go:build !windows && !linux && !darwin

package elevation

import (
	"fmt"
	"runtime"
)

func IsProcessElevated() bool {
	return false
}

func LaunchService(exePath string, port int, secretB64 string) (int, error) {
	return 0, fmt.Errorf("elevation service not supported on %s", runtime.GOOS)
}

func LaunchProcess(exePath, reqB64, resultPath string) error {
	return fmt.Errorf("elevation not supported on %s", runtime.GOOS)
}
