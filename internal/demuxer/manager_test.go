package demuxer

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestFFprobeAvailable verifies ffprobe is in PATH
func TestFFprobeAvailable(t *testing.T) {
	cmd := exec.Command("ffprobe", "-version")
	err := cmd.Run()
	if err != nil {
		t.Fatalf("ffprobe not available in PATH: %v\nPlease install FFmpeg (includes ffprobe)", err)
	}
}

// TestSessionManagerCreation verifies NewFFmpegSessionManager initializes correctly
func TestSessionManagerCreation(t *testing.T) {
	mgr := NewFFmpegSessionManager(0)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	if mgr.sessions == nil {
		t.Fatal("expected non-nil sessions map")
	}
}

// TestOpenPathWithMissingFile verifies error handling for missing files
func TestOpenPathWithMissingFile(t *testing.T) {
	mgr := NewFFmpegSessionManager(0)
	_, err := mgr.OpenPath("/nonexistent/file.mp4")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	t.Logf("Got expected error: %v", err)
}

// TestCodecNameToWebCodecs verifies codec name mapping
func TestCodecNameToWebCodecs(t *testing.T) {
	tests := map[string]string{
		"h264": "avc1.640028",
		"hevc": "hev1.1.6.L123.B0",
		"aac":  "mp4a.40.2",
		"opus": "opus",
	}
	for codec, expected := range tests {
		result := codecNameToWebCodecs(codec)
		if result != expected {
			t.Errorf("codecNameToWebCodecs(%q) = %q, want %q", codec, result, expected)
		}
	}
}

// Helper: Runs main test without -short flag
func TestMain(m *testing.M) {
	// Check if ffprobe is available
	cmd := exec.Command("ffprobe", "-version")
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  WARNING: ffprobe not found in PATH\n")
		fmt.Printf("To test with real video files, install FFmpeg:\n")
		fmt.Printf("  Windows (Chocolatey): choco install ffmpeg\n")
		fmt.Printf("  Windows (scoop): scoop install ffmpeg\n")
		fmt.Printf("  Linux (apt): sudo apt-get install ffmpeg\n")
		fmt.Printf("  macOS (brew): brew install ffmpeg\n\n")
	}
	os.Exit(m.Run())
}
