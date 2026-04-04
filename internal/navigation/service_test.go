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
