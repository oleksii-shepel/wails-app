package main

import internalelevation "wails-app/internal/elevation"

func startElevatedClient(exePath string) (internalelevation.Client, error) {
	return StartElevatedServiceClient(exePath)
}

func emitAppEvent(app *App) func(string, interface{}) {
	return func(name string, data interface{}) {
		if app.State != nil {
			app.State.EmitEvent(name, data)
		}
	}
}
