package main

import "time"

// StartElevatedSession launches the elevated helper process (one UAC/sudo
// prompt) and establishes a persistent JWT-authenticated TCP channel to it.
// All subsequent permission-denied operations are routed through this channel
// with no further OS prompts.
func (a *App) StartElevatedSession() (*ElevatedSessionStatus, error) {
	return a.elevation.StartSession()
}

// StopElevatedSession closes the elevated IPC channel and terminates the
// helper process. Future permission-denied operations will fall back to the
// interactive per-op UAC dialog.
func (a *App) StopElevatedSession() {
	a.elevation.StopSession()
}

// GetElevatedSessionStatus returns the live status of the elevated service
// channel without starting or stopping it.
func (a *App) GetElevatedSessionStatus() ElevatedSessionStatus {
	return a.elevation.Status()
}

// IsProcessElevated returns true when the current process is running with
// administrator / root privileges.
func (a *App) IsProcessElevated() bool {
	return isProcessElevated()
}

func (a *App) ConfirmElevationRequest(requestID string, approve bool) {
	a.elevation.Confirm(requestID, approve)
}

func (a *App) waitForElevationApproval(requestID string, timeout time.Duration) (bool, error) {
	return a.elevation.WaitForApproval(requestID, timeout)
}