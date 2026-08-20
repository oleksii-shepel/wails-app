package app

import "testing"

func TestAppResourcesStreamBaseURL(t *testing.T) {
	resources := newHostResources()
	resources.setStreamBaseURL("http://127.0.0.1:1234")
	if got := resources.streamURL(); got != "http://127.0.0.1:1234" {
		t.Fatalf("unexpected stream base URL: %q", got)
	}
}

func TestAppResourcesShutdownIsIdempotentWithoutBrowserPane(t *testing.T) {
	resources := newHostResources()
	resources.shutdown()
	resources.shutdown()
}

func TestZeroValueAppCanUseStreamResource(t *testing.T) {
	var app App
	app.SetStreamBaseURL("http://localhost:9999")
	if got := app.StreamBaseURL(); got != "http://localhost:9999" {
		t.Fatalf("unexpected stream base URL: %q", got)
	}
}
