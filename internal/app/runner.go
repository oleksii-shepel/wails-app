package app

import (
	"context"
	"embed"
	"net/http"

	"wails-app/vfs/archive/cache"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func Run(assets embed.FS, binding interface{}, archCache *cache.Cache, onStartup func(context.Context), onShutdown func(context.Context)) error {
	appBinding, _ := binding.(*App)

	// Wrap the default static-file handler with our streaming middleware.
	// The middleware intercepts /wails-stream requests and pipes file/archive
	// content directly, without buffering in JS or using Wails events.
	streamMiddleware := func(next http.Handler) http.Handler {
		return NewStreamMiddleware(appBinding, archCache, next)
	}

	return wails.Run(&options.App{
		Title:            "File Manager",
		Width:            1280,
		Height:           800,
		AssetServer:      &assetserver.Options{Assets: assets, Middleware: streamMiddleware},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        onStartup,
		OnShutdown:       onShutdown,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: false,
		},
		Bind: []interface{}{binding},
	})
}
