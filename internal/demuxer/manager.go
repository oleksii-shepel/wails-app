package demuxer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FFmpegSessionManager is a pragmatic MVP using ffmpeg-go (CLI wrapper)
// This allows testing on Windows without CGO complexity.
// TODO: Migrate to direct libavformat bindings once architecture is proven.
type FFmpegSessionManager struct {
	sessions map[string]*mediaSession
	mu       sync.RWMutex
	// Session idle timeout
	idleTimeout time.Duration
}

// mediaSession holds state for an open media session
type mediaSession struct {
	sessionID  string
	path       string
	info       *SessionInfo
	openedAt   time.Time
	lastAccess time.Time
}

// NewFFmpegSessionManager creates a new session manager
// idleTimeout specifies how long to keep idle sessions open
func NewFFmpegSessionManager(idleTimeout time.Duration) *FFmpegSessionManager {
	m := &FFmpegSessionManager{
		sessions:    make(map[string]*mediaSession),
		idleTimeout: idleTimeout,
	}

	// Periodically clean up idle sessions
	go m.reapIdleSessions()

	return m
}

// OpenPath opens a media file and creates a session
func (m *FFmpegSessionManager) OpenPath(path string) (*SessionInfo, error) {
	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("cannot access file: %w", err)
	}

	// Call ffprobe directly to get metadata
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w; is ffprobe in PATH?", err)
	}

	// Parse the ffprobe JSON output
	var probeData struct {
		Format struct {
			Duration   string            `json:"duration"`
			Filename   string            `json:"filename"`
			FormatName string            `json:"format_name"`
			Tags       map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			Index        int               `json:"index"`
			CodecType    string            `json:"codec_type"`
			CodecName    string            `json:"codec_name"`
			Width        int               `json:"width"`
			Height       int               `json:"height"`
			AvgFrameRate string            `json:"avg_frame_rate"`
			SampleRate   int               `json:"sample_rate"`
			Channels     int               `json:"channels"`
			TimeBase     string            `json:"time_base"`
			Tags         map[string]string `json:"tags"`
		} `json:"streams"`
	}

	err = json.Unmarshal(output, &probeData)
	if err != nil {
		return nil, fmt.Errorf("cannot parse ffprobe JSON: %w", err)
	}

	// Build session info
	durationSec, _ := strconv.ParseFloat(probeData.Format.Duration, 64)
	durationMs := int64(durationSec * 1000)

	info := &SessionInfo{
		SessionID:    generateSessionID(),
		Title:        extractFFprobeTitle(probeData.Format.Tags),
		DurationMs:   durationMs,
		Seekable:     true,
		Tracks:       []TrackInfo{},
		SeekPointsMs: []int64{},
	}

	// Extract track info from streams
	for _, stream := range probeData.Streams {
		track := TrackInfo{
			ID:   stream.Index,
			Kind: stream.CodecType,
		}

		if stream.CodecType == "video" {
			track.Width = stream.Width
			track.Height = stream.Height
			track.Codec = codecNameToWebCodecs(stream.CodecName)
			track.BitstreamFormat = "annexb"

			// Parse FPS from avg_frame_rate (e.g., "30/1" or "24000/1001")
			if stream.AvgFrameRate != "" {
				parts := strings.Split(stream.AvgFrameRate, "/")
				if len(parts) == 2 {
					if num, err := strconv.ParseFloat(parts[0], 64); err == nil {
						if den, err := strconv.ParseFloat(parts[1], 64); err == nil && den > 0 {
							track.FPS = num / den
						}
					}
				}
			}
		} else if stream.CodecType == "audio" {
			track.SampleRate = stream.SampleRate
			track.Channels = stream.Channels
			track.Codec = codecNameToWebCodecs(stream.CodecName)
			track.BitstreamFormat = "adts"
		}

		info.Tracks = append(info.Tracks, track)
	}

	// Create session
	sess := &mediaSession{
		sessionID:  info.SessionID,
		path:       path,
		info:       info,
		openedAt:   time.Now(),
		lastAccess: time.Now(),
	}

	m.mu.Lock()
	m.sessions[info.SessionID] = sess
	m.mu.Unlock()

	return info, nil
}

// ReadPackets fetches a batch of codec packets from a session
func (m *FFmpegSessionManager) ReadPackets(sessionID string, trackID int, cursor string, limit int) (*PacketBatch, error) {
	m.mu.RLock()
	sess, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	sess.lastAccess = time.Now()

	// Find the track in session info
	var track *TrackInfo
	for i := range sess.info.Tracks {
		if sess.info.Tracks[i].ID == trackID {
			track = &sess.info.Tracks[i]
			break
		}
	}
	if track == nil {
		return nil, fmt.Errorf("track %d not found in session", trackID)
	}

	// Start cursor position (0 if not specified)
	startPos := 0
	if cursor != "" {
		fmt.Sscanf(cursor, "%d", &startPos)
	}

	// Extract packets based on track type
	var packets []PacketMeta
	var payload []byte
	var nextCursor string
	var err error

	if track.Kind == "video" {
		packets, payload, nextCursor, err = m.extractVideoPackets(sess.path, trackID, startPos, limit)
	} else if track.Kind == "audio" {
		packets, payload, nextCursor, err = m.extractAudioPackets(sess.path, trackID, startPos, limit)
	} else {
		return nil, fmt.Errorf("unsupported track kind: %s", track.Kind)
	}

	if err != nil {
		return nil, fmt.Errorf("packet extraction failed: %w", err)
	}

	return &PacketBatch{
		SessionID:  sessionID,
		TrackID:    trackID,
		NextCursor: nextCursor,
		Packets:    packets,
		Payload:    payload,
	}, nil
}

// Seek seeks to a time position and returns the nearest keyframe
// NOTE: This is a stub for the MVP.
// Seek seeks to a time position and returns the nearest keyframe
func (m *FFmpegSessionManager) Seek(sessionID string, trackID int, timeMs int64) (*SeekResult, error) {
	m.mu.RLock()
	sess, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	sess.lastAccess = time.Now()

	// Find the track in session info
	var track *TrackInfo
	for i := range sess.info.Tracks {
		if sess.info.Tracks[i].ID == trackID {
			track = &sess.info.Tracks[i]
			break
		}
	}
	if track == nil {
		return nil, fmt.Errorf("track %d not found in session", trackID)
	}

	// Clamp seek time to valid range
	if timeMs < 0 {
		timeMs = 0
	}
	if timeMs > sess.info.DurationMs {
		timeMs = sess.info.DurationMs
	}

	// Seek using ffmpeg
	var packets []PacketMeta
	var resolvedTimeMs int64
	var err error

	if track.Kind == "video" {
		var payload []byte
		packets, payload, resolvedTimeMs, err = m.seekVideoPackets(sess.path, trackID, timeMs)
		_ = payload // Payload handled by caller (HTTP handler writes to response)
		if err != nil {
			return nil, fmt.Errorf("video seek failed: %w", err)
		}
	} else if track.Kind == "audio" {
		var payload []byte
		packets, payload, resolvedTimeMs, err = m.seekAudioPackets(sess.path, trackID, timeMs)
		_ = payload // Payload handled by caller (HTTP handler writes to response)
		if err != nil {
			return nil, fmt.Errorf("audio seek failed: %w", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported track kind for seek: %s", track.Kind)
	}

	// Find first keyframe (for video) or use first packet (for audio)
	cursor := "0"
	isKeyframe := false
	if len(packets) > 0 {
		cursor = packets[0].Cursor
		isKeyframe = packets[0].IsKeyframe
	}

	return &SeekResult{
		SessionID:      sessionID,
		ResolvedTimeMs: resolvedTimeMs,
		Cursor:         cursor,
		IsKeyframe:     isKeyframe,
	}, nil
}

// Close terminates a session and frees resources
func (m *FFmpegSessionManager) Close(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	delete(m.sessions, sessionID)
	return nil
}

// reapIdleSessions periodically removes sessions that have been idle
func (m *FFmpegSessionManager) reapIdleSessions() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		var toDelete []string

		for sessionID, sess := range m.sessions {
			if now.Sub(sess.lastAccess) > m.idleTimeout {
				toDelete = append(toDelete, sessionID)
			}
		}

		for _, sessionID := range toDelete {
			delete(m.sessions, sessionID)
		}
		m.mu.Unlock()
	}
}

// Helper: generate a unique session ID
func generateSessionID() string {
	// Return something like "m_8f3b6b8d"
	return fmt.Sprintf("m_%d", time.Now().UnixNano()%1e10)
}

// Helper: encode bytes as base64 for JSON transmission
func encodeB64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Helper: decode base64 from JSON
func decodeB64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// extractVideoPackets extracts H.264/HEVC packets from a video file
func (m *FFmpegSessionManager) extractVideoPackets(filePath string, trackID int, startPos int, limit int) ([]PacketMeta, []byte, string, error) {
	// Use ffmpeg to output raw H.264/HEVC bitstream
	// Convert MP4 to Annex B format (NAL units with start codes)
	cmd := exec.Command("ffmpeg",
		"-i", filePath,
		"-map", fmt.Sprintf("0:%d", trackID),
		"-c:v", "copy",
		"-bsf:v", "h264_mp4toannexb", // Converts to Annex B; works for H.264
		"-f", "rawvideo",
		"-",
	)

	output, err := cmd.Output()
	if err != nil {
		// Fallback for non-H.264 codecs (HEVC, etc.)
		// Return empty batch for now; would need codec-specific handling
		return nil, nil, "", fmt.Errorf("video packet extraction: %w", err)
	}

	// Parse NAL units from bitstream
	packets, payload := parseNALUnits(output, startPos, limit)
	nextCursor := ""
	if len(packets) > 0 && startPos+len(packets) < countNALUnits(output) {
		nextCursor = fmt.Sprintf("%d", startPos+len(packets))
	}

	return packets, payload, nextCursor, nil
}

// extractAudioPackets extracts ADTS audio packets from an audio track
func (m *FFmpegSessionManager) extractAudioPackets(filePath string, trackID int, startPos int, limit int) ([]PacketMeta, []byte, string, error) {
	// Use ffmpeg to output raw ADTS audio
	cmd := exec.Command("ffmpeg",
		"-i", filePath,
		"-map", fmt.Sprintf("0:%d", trackID),
		"-c:a", "copy",
		"-f", "adts",
		"-",
	)

	output, err := cmd.Output()
	if err != nil {
		// Fallback for non-AAC codecs
		return nil, nil, "", fmt.Errorf("audio packet extraction: %w", err)
	}

	// Parse ADTS frames
	packets, payload := parseADTSFrames(output, startPos, limit)
	nextCursor := ""
	if len(packets) > 0 && startPos+len(packets) < countADTSFrames(output) {
		nextCursor = fmt.Sprintf("%d", startPos+len(packets))
	}

	return packets, payload, nextCursor, nil
}

// parseNALUnits parses H.264 NAL units from Annex B bitstream
func parseNALUnits(data []byte, startPos int, limit int) ([]PacketMeta, []byte) {
	var packets []PacketMeta
	var payload []byte
	packetCount := 0
	offset := 0

	// Find start codes and group NAL units
	i := 0
	nalStart := -1
	count := 0

	for i < len(data) {
		// Look for 0x00 0x00 0x00 0x01 or 0x00 0x00 0x01 start code
		isStartCode := false
		codeLen := 0

		if i+4 <= len(data) && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01 {
			isStartCode = true
			codeLen = 4
		} else if i+3 <= len(data) && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			isStartCode = true
			codeLen = 3
		}

		if isStartCode {
			// Found a start code
			if nalStart >= 0 && count >= startPos {
				// Save previous NAL unit
				nalEnd := i
				nalData := data[nalStart+codeLen : nalEnd]

				// Determine if keyframe (I-slice NAL type)
				var nalType byte
				if nalStart+codeLen < len(data) {
					nalType = data[nalStart+codeLen] & 0x1F
				}
				isKeyframe := nalType == 5 || nalType == 7 || nalType == 8 // IDR (5), SPS (7), PPS (8)

				packet := PacketMeta{
					Cursor:      fmt.Sprintf("%d", count),
					Offset:      offset,
					Size:        len(nalData),
					IsKeyframe:  isKeyframe,
					TimestampUs: 0, // Placeholder; would need temporal info
					DurationUs:  0,
				}
				packets = append(packets, packet)
				payload = append(payload, nalData...)
				offset += len(nalData)
				packetCount++

				if packetCount >= limit {
					break
				}
			}

			nalStart = i
			count++
			i += codeLen
		} else {
			i++
		}
	}

	// Handle last NAL unit
	if nalStart >= 0 && count >= startPos && packetCount < limit {
		codeLen := 3 // Assume 3-byte or 4-byte start code
		nalData := data[nalStart+codeLen:]

		var nalType byte
		if nalStart+codeLen < len(data) {
			nalType = data[nalStart+codeLen] & 0x1F
		}
		isKeyframe := nalType == 5 || nalType == 7 || nalType == 8

		packet := PacketMeta{
			Cursor:      fmt.Sprintf("%d", count),
			Offset:      offset,
			Size:        len(nalData),
			IsKeyframe:  isKeyframe,
			TimestampUs: 0,
			DurationUs:  0,
		}
		packets = append(packets, packet)
		payload = append(payload, nalData...)
	}

	return packets, payload
}

// parseADTSFrames parses ADTS audio frames
func parseADTSFrames(data []byte, startPos int, limit int) ([]PacketMeta, []byte) {
	var packets []PacketMeta
	var payload []byte
	packetCount := 0
	frameCount := 0
	offset := 0
	i := 0

	for i < len(data)-7 {
		// Look for ADTS sync word (0xFF 0xF9 or 0xFF 0xFA)
		if data[i] == 0xFF && (data[i+1]&0xF0) == 0xF0 {
			if frameCount >= startPos {
				// Parse ADTS header for frame size (13 bits at position 30)
				frameSize := int((data[i+3]&0x03)<<11 | (data[i+4] << 3) | (data[i+5] >> 5))
				if frameSize < 7 || i+frameSize > len(data) {
					frameSize = 1 // Fallback for malformed frame
				}

				frameData := data[i : i+frameSize]

				packet := PacketMeta{
					Cursor:      fmt.Sprintf("%d", frameCount),
					Offset:      offset,
					Size:        len(frameData),
					IsKeyframe:  false, // Audio frames don't have keyframes
					TimestampUs: 0,
					DurationUs:  0,
				}
				packets = append(packets, packet)
				payload = append(payload, frameData...)
				offset += len(frameData)
				packetCount++

				if packetCount >= limit {
					break
				}
			}
			frameCount++
			i += 7 // Skip at least past the ADTS header
		} else {
			i++
		}
	}

	return packets, payload
}

// countNALUnits counts total NAL units in Annex B bitstream (for cursor validation)
func countNALUnits(data []byte) int {
	count := 0
	for i := 0; i < len(data); {
		if i+4 <= len(data) && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01 {
			count++
			i += 4
		} else if i+3 <= len(data) && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			count++
			i += 3
		} else {
			i++
		}
	}
	return count
}

// countADTSFrames counts total ADTS frames (for cursor validation)
func countADTSFrames(data []byte) int {
	count := 0
	for i := 0; i < len(data)-1; {
		if data[i] == 0xFF && (data[i+1]&0xF0) == 0xF0 {
			count++
			i += 2
		} else {
			i++
		}
	}
	return count
}

// seekVideoPackets seeks to a time position and extracts video packets
func (m *FFmpegSessionManager) seekVideoPackets(filePath string, trackID int, timeMs int64) ([]PacketMeta, []byte, int64, error) {
	// Convert milliseconds to seconds for ffmpeg -ss parameter
	seekSeconds := float64(timeMs) / 1000.0

	// Use ffmpeg to seek and output bitstream
	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", seekSeconds),
		"-i", filePath,
		"-map", fmt.Sprintf("0:%d", trackID),
		"-c:v", "copy",
		"-bsf:v", "h264_mp4toannexb",
		"-f", "rawvideo",
		"-",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, nil, timeMs, fmt.Errorf("ffmpeg seek failed: %w", err)
	}

	// Parse NAL units starting from position 0
	packets, payload := parseNALUnits(output, 0, 100) // Extract up to 100 packets after seek
	resolvedTimeMs := timeMs                          // Assumed time (would need frame timestamp from ffmpeg)

	return packets, payload, resolvedTimeMs, nil
}

// seekAudioPackets seeks to a time position and extracts audio packets
func (m *FFmpegSessionManager) seekAudioPackets(filePath string, trackID int, timeMs int64) ([]PacketMeta, []byte, int64, error) {
	// Convert milliseconds to seconds for ffmpeg -ss parameter
	seekSeconds := float64(timeMs) / 1000.0

	// Use ffmpeg to seek and output ADTS audio
	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", seekSeconds),
		"-i", filePath,
		"-map", fmt.Sprintf("0:%d", trackID),
		"-c:a", "copy",
		"-f", "adts",
		"-",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, nil, timeMs, fmt.Errorf("ffmpeg audio seek failed: %w", err)
	}

	// Parse ADTS frames starting from position 0
	packets, payload := parseADTSFrames(output, 0, 100) // Extract up to 100 frames after seek
	resolvedTimeMs := timeMs                            // Assumed time (would need frame timestamp from ffmpeg)

	return packets, payload, resolvedTimeMs, nil
}

// Helper: convert codec name from ffprobe to WebCodecs codec string
func codecNameToWebCodecs(codecName string) string {
	switch codecName {
	case "h264":
		return "avc1.640028"
	case "hevc":
		return "hev1.1.6.L123.B0"
	case "vp8":
		return "vp8"
	case "vp9":
		return "vp9"
	case "av1":
		return "av01.0.08M.08"
	case "aac":
		return "mp4a.40.2"
	case "opus":
		return "opus"
	case "vorbis":
		return "vorbis"
	default:
		return codecName
	}
}

// Helper: extract title from ffprobe tags
func extractFFprobeTitle(tags map[string]string) string {
	if title, ok := tags["title"]; ok {
		return title
	}
	return ""
}
