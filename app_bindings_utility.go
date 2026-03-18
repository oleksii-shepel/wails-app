package main

import (
	"time"

	internalutility "wails-app/internal/utility"
)

type ResolvedLink = internalutility.ResolvedLink

func (a *App) Ping() int64 {
	return time.Now().UnixMilli()
}

func (a *App) OpenFile(path string) error {
	return a.utility.OpenFile(path)
}

func (a *App) ResolveLink(path string) (*ResolvedLink, error) {
	return a.utility.ResolveLink(path)
}
