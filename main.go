package main

import (
	"embed"
	"os"

	internalapp "wails-app/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Elevated service mode: helper process binds TCP, serves JWT-auth'd requests.
	if handled, err := maybeRunElevatedService(); handled {
		if err != nil {
			println("Elevated service error:", err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Legacy single-shot elevated helper (per-op UAC spawn).
	if handled, err := maybeRunElevatedHelper(); handled {
		if err != nil {
			println("Elevated helper error:", err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}

	app := NewApp()

	err := internalapp.Run(assets, app, app.startup, app.shutdown)

	if err != nil {
		println("Error:", err.Error())
	}
}
