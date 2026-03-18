package main

import (
	"os"

	internalelevation "wails-app/internal/elevation"
	internalfilesystem "wails-app/internal/filesystem"
)

const elevatedHelperFlag = internalelevation.HelperFlag

var elevatedRawHandler = internalfilesystem.NewElevatedHandler(internalfilesystem.RawOperations{
	ListDirectory:   rawFilesystemOps.ListDirectory,
	ReadFile:        rawFilesystemOps.ReadFile,
	WriteFile:       rawFilesystemOps.WriteFile,
	DeleteFile:      rawFilesystemOps.DeleteFile,
	CreateDirectory: rawFilesystemOps.CreateDirectory,
	MoveFile:        rawFilesystemOps.MoveFile,
	CopyFile:        rawFilesystemOps.CopyFile,
})

func maybeRunElevatedHelper() (bool, error) {
	return internalelevation.MaybeRunHelper(os.Args, elevatedRawHandler)
}

func executeElevatedRequest(req internalelevation.Request) internalelevation.Response {
	return elevatedRawHandler(req)
}

func runElevatedRequest(req internalelevation.Request) (internalelevation.Response, error) {
	return internalelevation.RunRequest(req, os.Executable, launchElevatedProcess)
}
