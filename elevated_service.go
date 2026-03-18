package main

import (
	"os"

	internalelevation "wails-app/internal/elevation"
)

const elevatedServiceFlag = internalelevation.ServiceFlag

type ElevatedSessionStatus = internalelevation.SessionStatus

type ElevatedServiceClient = internalelevation.IPCClient

func StartElevatedServiceClient(exePath string) (*ElevatedServiceClient, error) {
	return internalelevation.StartClient(exePath, launchElevatedService)
}

func maybeRunElevatedService() (bool, error) {
	return internalelevation.MaybeRunService(os.Args, elevatedRawHandler)
}