package main

func (a *App) GetActiveOperations() []OperationProgress {
	return a.operations.ListActiveOperations()
}

func (a *App) CancelOperation(id string) bool {
	return a.operations.CancelOperation(id)
}

func (a *App) DismissOperation(id string) {
	a.operations.RemoveOperation(id)
}