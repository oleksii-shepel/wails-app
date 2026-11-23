package app

import "wails-app/internal/cli"

func (a *App) ExecuteCliCommand(command, cwd string, color bool) (cli.Result, error) {
	services := cli.Services{CreateZip: a.CreateZipArchive}
	if a.State != nil && a.Workspace != nil {
		services.Workspaces = a.GetWorkspaces
	}
	return services.Execute(command, cwd, color), nil
}
