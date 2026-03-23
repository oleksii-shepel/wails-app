package app

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"wails-app/vfs/archive/cache"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func Run(assets embed.FS, binding interface{}, archCache *cache.Cache, onStartup func(context.Context), onShutdown func(context.Context)) error {
	appBinding, _ := binding.(*App)
	if appBinding == nil {
		return fmt.Errorf("app binding is required for stream server")
	}

	streamServer, streamBaseURL, err := startStreamServer(appBinding, archCache)
	if err != nil {
		return err
	}
	appBinding.SetStreamBaseURL(streamBaseURL)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := streamServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			println("stream server shutdown error:", err.Error())
		}
	}()

	return wails.Run(&options.App{
		Title:            "File Manager",
		Width:            1280,
		Height:           800,
		AssetServer:      &assetserver.Options{Assets: assets},
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

func startStreamServer(appBinding *App, archCache *cache.Cache) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("start stream server: %w", err)
	}

	server := &http.Server{
		Handler:           NewStreamHandler(appBinding, archCache),
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			println("stream server error:", err.Error())
		}
	}()

	return server, "http://" + listener.Addr().String(), nil
}
