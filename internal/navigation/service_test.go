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

	service.SyncPath("left", "smb://fileserver/share/docs")

	listing, err := service.GoUp("left")
	if err != nil {
		t.Fatalf("GoUp returned error: %v", err)
	}
	if listing.Path != "smb://fileserver/share" {
		t.Fatalf("unexpected parent path\nwant: %q\n got: %q", "smb://fileserver/share", listing.Path)
	}
}

func TestSyncPathKeepsGoUpAlignedWithExplicitSMBPort(t *testing.T) {
	manager := NewManager()
	service := NewService(manager, func(path string, showHidden bool) (*contracts.DirectoryListing, error) {
		return &contracts.DirectoryListing{Path: path}, nil
	})

	service.SyncPath("left", "smb://fileserver:445/share/docs")

	listing, err := service.GoUp("left")
	if err != nil {
		t.Fatalf("GoUp returned error: %v", err)
	}
	if listing.Path != "smb://fileserver:445/share" {
		t.Fatalf("unexpected parent path\nwant: %q\n got: %q", "smb://fileserver:445/share", listing.Path)
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
