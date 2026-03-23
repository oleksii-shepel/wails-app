package app

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"wails-app/vfs/archive/cache"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func Run(assets embed.FS, binding interface{}, archCache *cache.Cache, onStartup func(context.Context), onShutdown func(context.Context)) error {
	appBinding, _ := binding.(*App)
	if appBinding == nil {
		return fmt.Errorf("app binding is required")
	}

	// Detect if we're in dev mode (Angular dev server is expected to be running on :4200)
	isDev := os.Getenv("WAILS_DEV") == "true"

	if isDev {
		// Development: Use reverse proxy combining stream server + Angular dev server
		streamServer, streamBaseURL, err := startUnifiedServer(appBinding, archCache)
		if err != nil {
			return err
		}

		angularDevURL := "http://127.0.0.1:4200"
		proxyServer, proxyURL, err := StartDevProxy(streamBaseURL, angularDevURL, archCache)
		if err != nil {
			streamServer.Shutdown(context.Background())
			return err
		}

		appBinding.SetStreamBaseURL(proxyURL)

		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := proxyServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				println("dev proxy shutdown error:", err.Error())
			}

			if err := streamServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				println("stream server shutdown error:", err.Error())
			}
		}()
	} else {
		// Production: Use unified stream server + embedded Wails assets
		streamServer, streamBaseURL, err := startUnifiedServer(appBinding, archCache)
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
	}

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

// startUnifiedServer starts a single server that handles both:
// - Video stream requests (/wails-stream)
// - Archive entry streaming
func startUnifiedServer(appBinding *App, archCache *cache.Cache) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("start unified server: %w", err)
	}

	baseURL := "http://" + listener.Addr().String()

	server := &http.Server{
		Handler:           NewUnifiedHandler(appBinding, archCache),
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			println("unified server error:", err.Error())
		}
	}()

	return server, baseURL, nil
}
