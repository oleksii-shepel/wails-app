package app

import (
	internalarchive "wails-app/internal/archive"
	internalconverter "wails-app/internal/converter"
	internalnav "wails-app/internal/navigation"
	internalops "wails-app/internal/ops"
	internalworkspace "wails-app/internal/workspace"
	"wails-app/vfs/archive/cache"
)

type Core struct {
	WorkspaceManager  *internalworkspace.Manager
	NavigationManager *internalnav.Manager
	Operations        *internalops.Manager
	ArchiveCache      *cache.Cache
	Converter         *internalconverter.Service
}

func NewCore() (*Core, error) {
	workspaceManager, err := internalworkspace.NewManager()
	if err != nil {
		workspaceManager = internalworkspace.NewTransientManager()
	}

	return &Core{
		WorkspaceManager:  workspaceManager,
		NavigationManager: internalnav.NewManager(),
		Operations:        internalops.NewManager(),
		Converter:         internalconverter.NewService(),
		ArchiveCache:      cache.New(internalarchive.ResolveChain),
	}, err
}
