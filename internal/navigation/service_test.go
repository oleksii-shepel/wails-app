package navigation

import (
	"testing"

	contracts "wails-app/internal/contracts"
)

func TestSyncPathKeepsGoUpAlignedWithPanePath(t *testing.T) {
	manager := NewManager()
	service := NewService(manager, func(path string, showHidden bool) (*contracts.DirectoryListing, error) {
		return &contracts.DirectoryListing{Path: path}, nil
	})

	service.SyncPath("left", "ftp://samples.ffmpeg.org/pub")

	listing, err := service.GoUp("left")
	if err != nil {
		t.Fatalf("GoUp returned error: %v", err)
	}
	if listing.Path != "ftp://samples.ffmpeg.org" {
		t.Fatalf("unexpected parent path\nwant: %q\n got: %q", "ftp://samples.ffmpeg.org", listing.Path)
	}
}

func TestSyncPathKeepsGoUpAlignedWithExplicitFTPPort(t *testing.T) {
	manager := NewManager()
	service := NewService(manager, func(path string, showHidden bool) (*contracts.DirectoryListing, error) {
		return &contracts.DirectoryListing{Path: path}, nil
	})

	service.SyncPath("left", "ftp://samples.ffmpeg.org:21/pub")

	listing, err := service.GoUp("left")
	if err != nil {
		t.Fatalf("GoUp returned error: %v", err)
	}
	if listing.Path != "ftp://samples.ffmpeg.org:21" {
		t.Fatalf("unexpected parent path\nwant: %q\n got: %q", "ftp://samples.ffmpeg.org:21", listing.Path)
	}
}

func TestSyncPathKeepsGoUpAlignedWithHTTPAlias(t *testing.T) {
	manager := NewManager()
	service := NewService(manager, func(path string, showHidden bool) (*contracts.DirectoryListing, error) {
		return &contracts.DirectoryListing{Path: path}, nil
	})

	service.SyncPath("left", "http://example.com/docs/reference")

	listing, err := service.GoUp("left")
	if err != nil {
		t.Fatalf("GoUp returned error: %v", err)
	}
	if listing.Path != "http://example.com/docs" {
		t.Fatalf("unexpected parent path\nwant: %q\n got: %q", "http://example.com/docs", listing.Path)
	}
}

func TestSyncPathKeepsGoUpAlignedWithHTTPSAlias(t *testing.T) {
	manager := NewManager()
	service := NewService(manager, func(path string, showHidden bool) (*contracts.DirectoryListing, error) {
		return &contracts.DirectoryListing{Path: path}, nil
	})

	service.SyncPath("left", "https://example.com/docs/reference")

	listing, err := service.GoUp("left")
	if err != nil {
		t.Fatalf("GoUp returned error: %v", err)
	}
	if listing.Path != "https://example.com/docs" {
		t.Fatalf("unexpected parent path\nwant: %q\n got: %q", "https://example.com/docs", listing.Path)
	}
}
