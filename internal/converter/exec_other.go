//go:build !windows

package converter

import "os/exec"

func hideWindow(_ *exec.Cmd) {}
