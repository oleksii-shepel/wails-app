package main

import internalops "wails-app/internal/ops"

func calculateSize(path string) (items, bytes int64) {
	return internalops.CalculateSize(path)
}