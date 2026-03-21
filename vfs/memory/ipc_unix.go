//go:build !windows

package memory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// ControlPipe is the Unix domain socket path used to reach the running memfs
// sidecar. It is per-user so multiple accounts can coexist without conflict.
var ControlPipe = "/tmp/memfs-control-" + strconv.Itoa(os.Getuid()) + ".sock"

type ConnectionInfo struct {
	Platform   string `json:"platform"`
	PID        int    `json:"pid"`
	HiddenPath string `json:"hiddenPath"`
	Pipe       string `json:"pipe"`
}

type connectionInfoRequest struct {
	Action string `json:"action"`
}

// GetConnectionInfo connects to the running memfs sidecar via its Unix domain
// socket and retrieves the mount path exposed as HiddenPath.
func GetConnectionInfo() (*ConnectionInfo, error) {
	conn, err := net.Dial("unix", ControlPipe)
	if err != nil {
		return nil, fmt.Errorf("connect to memfs control socket: %w", err)
	}
	defer conn.Close()

	reqBytes, err := json.Marshal(connectionInfoRequest{Action: "get-connection-info"})
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(append(reqBytes, '\n')); err != nil {
		return nil, fmt.Errorf("write memfs control request: %w", err)
	}

	line, err := bufio.NewReader(conn).ReadString('\n')
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
