package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"wails-app/internal/demuxer"
)

func main() {
	videoPath := flag.String("file", "", "Path to video file to probe")
	flag.Parse()

	if *videoPath == "" {
		fmt.Println(`Demo: Test FFmpeg Session Manager with a real video file

Usage:
  go run ./cmd/demuxer-demo -file /path/to/video.mp4

Example:
  go run ./cmd/demuxer-demo -file "C:\\Users\\Downloads\\sample.mp4"

What it does:
  1. Opens the video file via FFprobe metadata extraction
  2. Prints session info (codec, duration, tracks, FPS, etc.)
  3. Tests session manager lifecycle (open → close)

Output will show:
  - Session ID
  - Video codec (WebCodecs format)
  - Audio codec (WebCodecs format)  
  - Duration, FPS, resolution, sample rates
  - Codec details for frontend WebCodecs decoder

Requirements:
  - ffprobe must be in system PATH (FFmpeg installation)
  - Video file path must exist and be readable
`)
		return
	}

	fmt.Printf("🎬 Demuxer Test: Opening %s\n\n", *videoPath)

	// Create session manager
	mgr := demuxer.NewFFmpegSessionManager(5 * time.Minute)
	fmt.Println("✓ Session manager created")

	// Open the video file
	fmt.Printf("📋 Probing metadata...\n")
	info, err := mgr.OpenPath(*videoPath)
	if err != nil {
		log.Fatalf("❌ Error opening file: %v", err)
	}

	fmt.Println("\n✅ Session opened successfully!\n")

	// Pretty-print the session info as JSON
	jsonData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	fmt.Println("📊 Session Info:")
	fmt.Println(string(jsonData))

	fmt.Printf("\n📌 Summary:\n")
	fmt.Printf("  Session ID: %s\n", info.SessionID)
	fmt.Printf("  Title: %s\n", info.Title)
	fmt.Printf("  Duration: %dms (%.2fs)\n", info.DurationMs, float64(info.DurationMs)/1000)
	fmt.Printf("  Seekable: %v\n", info.Seekable)
	fmt.Printf("  Track count: %d\n\n", len(info.Tracks))

	for i, track := range info.Tracks {
		fmt.Printf("  Track %d (%s):\n", i, track.Kind)
		fmt.Printf("    Codec: %s\n", track.Codec)
		fmt.Printf("    Bitstream Format: %s\n", track.BitstreamFormat)
		if track.Kind == "video" {
			fmt.Printf("    Resolution: %dx%d\n", track.Width, track.Height)
			fmt.Printf("    FPS: %.2f\n", track.FPS)
		} else if track.Kind == "audio" {
			fmt.Printf("    Sample Rate: %d Hz\n", track.SampleRate)
			fmt.Printf("    Channels: %d\n", track.Channels)
		}
		fmt.Println()
	}

	// Test close
	fmt.Println("🔒 Closing session...")
	err = mgr.Close(info.SessionID)
	if err != nil {
		log.Fatalf("Error closing session: %v", err)
	}

	fmt.Println("✅ Session closed successfully!\n")
	fmt.Println("🎉 Demo complete. Backend demuxer is working!")
}
