package app

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"wails-app/vfs/archive/cache"
)

// StartDevProxy starts a reverse proxy on localhost for development.
// It routes /wails-stream/* to the stream server and everything else to Angular dev server.
func StartDevProxy(streamBaseURL, angularDevURL string, archCache *cache.Cache) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("start dev proxy: %w", err)
	}

	proxyPort := listener.Addr().String()

	// Parse URLs for proxying
	streamURL, err := url.Parse(streamBaseURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid stream base URL: %w", err)
	}

	angularURL, err := url.Parse(angularDevURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid angular dev URL: %w", err)
	}

	// Create reverse proxies
	streamProxy := httputil.NewSingleHostReverseProxy(streamURL)
	angularProxy := httputil.NewSingleHostReverseProxy(angularURL)

	// Main proxy handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wails-stream" || len(r.URL.Path) > len("/wails-stream") && r.URL.Path[:len("/wails-stream")] == "/wails-stream" {
			// Stream requests go to stream server
			streamProxy.ServeHTTP(w, r)
			return
		}
		// Everything else goes to Angular dev server
		angularProxy.ServeHTTP(w, r)
	})

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			println("dev proxy error:", err.Error())
		}
	}()

	return server, "http://" + proxyPort, nil
}
