package app

import (
	"os"
	"testing"

	contracts "wails-app/internal/contracts"
)

func TestServiceConfigUsesNarrowCompositionGroups(t *testing.T) {
	var config ServiceConfig

	config.Events.EmitEvent = func(string, interface{}) {}
	config.Directory.ElevatedList = func(string, bool) ([]contracts.FileInfo, error) { return nil, nil }
	config.Directory.BuildFileInfo = func(string, string, os.FileInfo, bool) contracts.FileInfo {
		return contracts.FileInfo{}
	}
	config.Directory.IsHidden = func(string, string) bool { return false }
	config.Directory.ListDirectory = func(string, bool) (*contracts.DirectoryListing, error) {
		return &contracts.DirectoryListing{}, nil
	}

	if config.Events.EmitEvent == nil {
		t.Fatal("expected event composition group to be assignable")
	}
	if config.Directory.ListDirectory == nil {
		t.Fatal("expected directory composition group to be assignable")
	}
}
