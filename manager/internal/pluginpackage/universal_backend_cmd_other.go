//go:build !windows

package pluginpackage

import "os/exec"

func configureBackendCommand(cmd *exec.Cmd) {}
