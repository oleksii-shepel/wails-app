package main

import (
	"os"
	"strings"

	internalfilesystem "wails-app/internal/filesystem"
	internalelevation "wails-app/internal/elevation"
)

const elevatedHelperFlag = internalelevation.HelperFlag

type elevatedRequest = internalelevation.Request

type elevatedResponse = internalelevation.Response

var elevatedRawHandler = internalfilesystem.NewElevatedHandler(internalfilesystem.RawOperations{
	ListDirectory:   listDirectoryRaw,
	ReadFile:        readFileRaw,
	WriteFile:       writeFileRaw,
	DeleteFile:      deleteFileRaw,
	CreateDirectory: createDirectoryRaw,
	MoveFile:        moveFileRaw,
	CopyFile:        copyFileRaw,
})

func maybeRunElevatedHelper() (bool, error) {
	return internalelevation.MaybeRunHelper(os.Args, elevatedRawHandler)
}

func executeElevatedRequest(req elevatedRequest) elevatedResponse {
	return elevatedRawHandler(req)
}

func runElevatedRequest(req elevatedRequest) (elevatedResponse, error) {
	return internalelevation.RunRequest(req, os.Executable, launchElevatedProcess)
}

func isPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	if os.IsPermission(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "eacces") ||
		strings.Contains(msg, "eperm")
}

func shouldRetryDeleteWithElevation(err error) bool {
	if isPermissionDenied(err) {
		return true
	}
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "shfileoperationw returned error code 0x78")
}