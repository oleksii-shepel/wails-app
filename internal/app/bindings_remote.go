package app

import contracts "wails-app/internal/contracts"

// RemoteConnect establishes a new remote filesystem connection and mounts it
// in the VFS. Returns the mount metadata including the public path to navigate to.
func (a *App) RemoteConnect(cfg contracts.RemoteConfig) (*contracts.RemoteMount, error) {
	return a.State.RemoteConnect(cfg)
}

// RemoteDisconnect unmounts the remote filesystem at the given VFS mount point
// (as returned by RemoteConnect.MountPoint) and closes the connection.
func (a *App) RemoteDisconnect(mountPoint string) error {
	return a.State.RemoteDisconnect(mountPoint)
}

// RemoteListConnections returns all currently active remote mounts.
func (a *App) RemoteListConnections() []contracts.RemoteMount {
	return a.State.RemoteListConnections()
}
