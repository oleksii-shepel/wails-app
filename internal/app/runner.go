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

	"wails-app/internal/workspace"
	"wails-app/vfs/archive/cache"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func Run(assets embed.FS, binding interface{}, archCache *cache.Cache, onStartup func(context.Context), onBeforeClose func(context.Context) bool, onShutdown func(context.Context)) error {
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

	width, height := 1280, 800
	x, y := 0, 0
	hasPosition := false
	startState := options.Normal

	if saved := workspace.LoadStoredWindowState("wails-file-manager"); saved != nil {
		if saved.Width >= 480 && saved.Height >= 320 {
			width = saved.Width
			height = saved.Height
		}
		// Legacy records (pre-HasPosition) had no flag; infer one from non-zero
		// coords. New records always carry HasPosition explicitly so legitimate
		// (0,0) restores correctly.
		if saved.HasPosition || saved.X != 0 || saved.Y != 0 {
			x = saved.X
			y = saved.Y
			hasPosition = true
		}
		if saved.Maximised {
			startState = options.Maximised
		}
	}

	wrappedStartup := onStartup
	if hasPosition {
		savedX, savedY, savedW, savedH := x, y, width, height
		wrappedStartup = func(ctx context.Context) {
			if onStartup != nil {
				onStartup(ctx)
			}
			wailsruntime.WindowSetPosition(ctx, savedX, savedY)
			if !rectIntersectsAnyScreen(ctx, savedX, savedY, savedW, savedH) {
				wailsruntime.WindowCenter(ctx)
			}
		}
	}

	return wails.Run(&options.App{
		Title:            "File Manager",
		Width:            width,
		Height:           height,
		Frameless:        true,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		WindowStartState: startState,
		OnStartup:        wrappedStartup,
		OnBeforeClose:    onBeforeClose,
		OnShutdown:       onShutdown,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: false,
		},
		Bind: []interface{}{binding},
	})
}

// rectIntersectsAnyScreen reports whether the window — already positioned by
// the caller — sits on a connected display. Wails marks the screen the window
// currently occupies with IsCurrent; if no screen reports IsCurrent the window
// has been placed off-screen (e.g. monitor disconnected since last save) and
// the caller should fall back to centring.
//
// The width/height arguments are unused today (Wails' Screen has no origin so
// we cannot compute geometric intersection here) but kept on the signature so
// a richer check can be wired later without rippling through callers.
func rectIntersectsAnyScreen(ctx context.Context, _, _, _, _ int) bool {
	screens, err := wailsruntime.ScreenGetAll(ctx)
	if err != nil || len(screens) == 0 {
		// Fail open: assume the position is fine if we cannot enumerate.
		return true
	}
	for _, screen := range screens {
		if screen.IsCurrent {
			return true
		}
	}
	return false
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
