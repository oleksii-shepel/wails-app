package main

import (
	internalelevation "wails-app/internal/elevation"
	internalfilesystem "wails-app/internal/filesystem"
)

type filesystemElevation interface {
	Client() internalelevation.Client
}

type FilesystemService = internalfilesystem.Service

func NewFilesystemService(
	elevation filesystemElevation,
	emitEvent func(string, interface{}),
	startElevatedSession func() (*ElevatedSessionStatus, error),
) *FilesystemService {
	return internalfilesystem.NewService(internalfilesystem.ServiceConfig{
		Elevation:                      elevation,
		EmitEvent:                      emitEvent,
		StartElevatedSession:           startElevatedSession,
		Raw:                            rawFilesystemOps,
		Metadata:                       rawFilesystemMetadataCallbacks(),
		IsProcessElevated:              isProcessElevated,
		ExecuteElevatedRequest:         executeElevatedRequest,
		RunElevatedRequest:             runElevatedRequest,
		IsPermissionDenied:             internalelevation.IsPermissionDenied,
		ShouldRetryDeleteWithElevation: internalelevation.ShouldRetryDeleteWithElevation,
	})
}
