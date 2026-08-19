package app

import (
	"time"

	internalelevation "wails-app/internal/elevation"
)

func (a *App) StartElevatedSession() (*internalelevation.SessionStatus, error) {
	if a.State == nil || a.Elevation == nil {
		return nil, nil
	}
	return a.Elevation.StartSession()
}

func (a *App) StopElevatedSession() {
	if a.State != nil && a.Elevation != nil {
		a.Elevation.StopSession()
	}
}

func (a *App) GetElevatedSessionStatus() internalelevation.SessionStatus {
	if a.State == nil || a.Elevation == nil {
		return internalelevation.SessionStatus{}
	}
	return a.Elevation.Status()
}

func (a *App) ConfirmElevationRequest(requestID string, approve bool) {
	if a.State != nil && a.Elevation != nil {
		a.Elevation.Confirm(requestID, approve)
	}
}

func (a *App) WaitForElevationApproval(requestID string, timeout time.Duration) (bool, error) {
	if a.State == nil || a.Elevation == nil {
		return false, nil
	}
	return a.Elevation.WaitForApproval(requestID, timeout)
}