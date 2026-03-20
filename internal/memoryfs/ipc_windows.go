//go:build windows

package memoryfs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

const ControlPipe = `\\.\pipe\memfs-control`

type ConnectionInfo struct {
	Platform   string `json:"platform"`
	PID        int    `json:"pid"`
	HiddenPath string `json:"hiddenPath"`
	Pipe       string `json:"pipe"`
}

type connectionInfoRequest struct {
	Action string `json:"action"`
}

func GetConnectionInfo() (*ConnectionInfo, error) {
	pipePath, err := windows.UTF16PtrFromString(ControlPipe)
	if err != nil {
		return nil, err
	}

	handle, err := windows.CreateFile(
		pipePath,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("connect to memfs control pipe: %w", err)
	}

	file := os.NewFile(uintptr(handle), ControlPipe)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("wrap memfs control pipe handle")
	}
	defer file.Close()

	reqBytes, err := json.Marshal(connectionInfoRequest{Action: "get-connection-info"})
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(append(reqBytes, '\n')); err != nil {
		return nil, fmt.Errorf("write memfs control request: %w", err)
	}

	line, err := bufio.NewReader(file).ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read memfs control response: %w", err)
	}

	var info ConnectionInfo
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &info); err != nil {
		return nil, fmt.Errorf("decode memfs control response: %w", err)
	}
	if strings.TrimSpace(info.HiddenPath) == "" {
		return nil, fmt.Errorf("memfs control response missing hiddenPath")
	}
	return &info, nil
}
