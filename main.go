package main

import (
	"context"
	"embed"
	"os"

	internalapp "wails-app/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Elevated service mode: helper process binds TCP, serves JWT-auth'd requests.
	if handled, err := internalapp.MaybeRunElevatedService(os.Args); handled {
		if err != nil {
			println("Elevated service error:", err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Legacy single-shot elevated helper (per-op UAC spawn).
	if handled, err := internalapp.MaybeRunElevatedHelper(os.Args); handled {
		if err != nil {
			println("Elevated helper error:", err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}

	app := internalapp.BuildApp()

	err := internalapp.Run(
		assets,
		app,
		app.Startup,
		func(context.Context) { app.Shutdown() },
	)

	if err != nil {
		println("Error:", err.Error())
	}
}
