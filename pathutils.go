package main

import internalplatform "wails-app/internal/platform"

func toSlash(path string) string {
	return internalplatform.ToSlash(path)
}

func cleanPath(path string) string {
	return internalplatform.CleanPath(path)
}

func parentPath(path string) string {
	return internalplatform.ParentPath(path)
}
