//go:build !windows

package memoryfs

import "fmt"

const ControlPipe = ""

type ConnectionInfo struct {
	Platform   string `json:"platform"`
	PID        int    `json:"pid"`
	HiddenPath string `json:"hiddenPath"`
	Pipe       string `json:"pipe"`
}

func GetConnectionInfo() (*ConnectionInfo, error) {
	return nil, fmt.Errorf("memfs control pipe is only available on windows")
}