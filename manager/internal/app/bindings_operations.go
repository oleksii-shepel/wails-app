package app

import contracts "wails-app/internal/contracts"

func (a *App) GetActiveOperations() []contracts.OperationProgress {
	return a.Operations.ListActiveOperations()
}

func (a *App) CancelOperation(id string) bool {
	return a.Operations.CancelOperation(id)
}

func (a *App) DismissOperation(id string) {
	a.Operations.RemoveOperation(id)
}