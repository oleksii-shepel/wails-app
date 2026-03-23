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

	// Pass the opaque cursor directly to the extract functions.
	// Cursor format: "seekMs:nalOffset" or legacy "nalOffset".
	if cursor == "" {
		cursor = "0"
	}

	// Extract packets based on track type
	var packets []PacketMeta
	var payload []byte
	var nextCursor string
	var err error

	if track.Kind == "video" {
		packets, payload, nextCursor, err = m.extractVideoPackets(sess.path, trackID, cursor, limit)
	} else if track.Kind == "audio" {
		packets, payload, nextCursor, err = m.extractAudioPackets(sess.path, trackID, cursor, limit)
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

// extractVideoPackets extracts H.264/HEVC packets from a video file.
// cursor format: "seekMs:nalOffset" or legacy "nalOffset" (seekMs=0 means start of file).
func (m *FFmpegSessionManager) extractVideoPackets(filePath string, trackID int, cursor string, limit int) ([]PacketMeta, []byte, string, error) {
	// Parse cursor
	var seekMs int64
	var startPos int
	if n, _ := fmt.Sscanf(cursor, "%d:%d", &seekMs, &startPos); n < 2 {
		seekMs = 0
		fmt.Sscanf(cursor, "%d", &startPos)
	}

	// Build ffmpeg args; place -ss before -i for fast keyframe-aligned seek
	var args []string
	if seekMs > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", float64(seekMs)/1000.0))
	}
	args = append(args,
		"-i", filePath,
		"-map", fmt.Sprintf("0:%d", trackID),
		"-c:v", "copy",
		"-bsf:v", "h264_mp4toannexb",
		"-f", "rawvideo",
		"-",
	)
	cmd := exec.Command("ffmpeg", args...)

	output, err := cmd.Output()
	if err != nil {
		return nil, nil, "", fmt.Errorf("video packet extraction: %w", err)
	}

	// Parse NAL units from bitstream
	packets, payload := parseNALUnits(output, startPos, limit)

	// Remap cursors to include the seek time so subsequent ReadPackets calls
	// continue from the correct ffmpeg seek context.
	for i := range packets {
		var nalIdx int
		fmt.Sscanf(packets[i].Cursor, "%d", &nalIdx)
		packets[i].Cursor = fmt.Sprintf("%d:%d", seekMs, nalIdx)
	}
	nextCursor := ""
	if len(packets) > 0 && startPos+len(packets) < countNALUnits(output) {
		nextCursor = fmt.Sprintf("%d:%d", seekMs, startPos+len(packets))
	}

	return packets, payload, nextCursor, nil
}

// extractAudioPackets extracts ADTS audio packets from an audio track.
// cursor format: "seekMs:frameOffset" or legacy "frameOffset".
func (m *FFmpegSessionManager) extractAudioPackets(filePath string, trackID int, cursor string, limit int) ([]PacketMeta, []byte, string, error) {
	// Parse cursor
	var seekMs int64
	var startPos int
	if n, _ := fmt.Sscanf(cursor, "%d:%d", &seekMs, &startPos); n < 2 {
		seekMs = 0
		fmt.Sscanf(cursor, "%d", &startPos)
	}

	// Build ffmpeg args
	var args []string
	if seekMs > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", float64(seekMs)/1000.0))
	}
	args = append(args,
		"-i", filePath,
		"-map", fmt.Sprintf("0:%d", trackID),
		"-c:a", "copy",
		"-f", "adts",
		"-",
	)
	cmd := exec.Command("ffmpeg", args...)

	output, err := cmd.Output()
	if err != nil {
		return nil, nil, "", fmt.Errorf("audio packet extraction: %w", err)
	}

	packets, payload := parseADTSFrames(output, startPos, limit)

	// Remap cursors to include the seek time
	for i := range packets {
		var frameIdx int
		fmt.Sscanf(packets[i].Cursor, "%d", &frameIdx)
		packets[i].Cursor = fmt.Sprintf("%d:%d", seekMs, frameIdx)
	}
	nextCursor := ""
	if len(packets) > 0 && startPos+len(packets) < countADTSFrames(output) {
		nextCursor = fmt.Sprintf("%d:%d", seekMs, startPos+len(packets))
	}

	return packets, payload, nextCursor, nil
}

type h264NALUnit struct {
	start  int
	end    int
	typeID byte
}

func findH264NALUnits(data []byte) []h264NALUnit {
	var units []h264NALUnit
	for i := 0; i < len(data); {
		codeLen := 0
		switch {
		case i+4 <= len(data) && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01:
			codeLen = 4
		case i+3 <= len(data) && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01:
			codeLen = 3
		default:
			i++
			continue
		}

		nalStart := i
		headerIndex := i + codeLen
		if headerIndex >= len(data) {
			break
		}

		i = headerIndex
		nalEnd := len(data)
		for j := i; j < len(data); j++ {
			if j+4 <= len(data) && data[j] == 0x00 && data[j+1] == 0x00 && data[j+2] == 0x00 && data[j+3] == 0x01 {
				nalEnd = j
				break
			}
			if j+3 <= len(data) && data[j] == 0x00 && data[j+1] == 0x00 && data[j+2] == 0x01 {
				nalEnd = j
				break
			}
		}

		units = append(units, h264NALUnit{
			start:  nalStart,
			end:    nalEnd,
			typeID: data[headerIndex] & 0x1F,
		})
		i = nalEnd
	}

	return units
}

func isH264VCLNAL(typeID byte) bool {
	return typeID == 1 || typeID == 5
}

func countH264AccessUnits(data []byte) int {
	units := findH264NALUnits(data)
	if len(units) == 0 {
		return 0
	}

	count := 0
	hasCurrent := false
	currentHasVCL := false

	flushCurrent := func() {
		if hasCurrent {
			count++
			hasCurrent = false
			currentHasVCL = false
		}
	}

	for _, unit := range units {
		if hasCurrent {
			if unit.typeID == 9 || (currentHasVCL && isH264VCLNAL(unit.typeID)) {
				flushCurrent()
			}
		}

		hasCurrent = true
		if isH264VCLNAL(unit.typeID) {
			currentHasVCL = true
		}
	}

	flushCurrent()
	return count
}

// parseNALUnits parses H.264 Annex B into access units suitable for WebCodecs.
func parseNALUnits(data []byte, startPos int, limit int) ([]PacketMeta, []byte) {
	units := findH264NALUnits(data)
	if len(units) == 0 || limit <= 0 {
		return nil, nil
	}

	var packets []PacketMeta
	var payload []byte
	offset := 0
	accessUnitCount := 0
	packetCount := 0

	currentStart := -1
	currentEnd := -1
	currentHasVCL := false
	currentIsKeyframe := false

	flushCurrent := func() bool {
		if currentStart < 0 || currentEnd <= currentStart {
			return false
		}
		if accessUnitCount >= startPos {
			unitData := data[currentStart:currentEnd]
			packets = append(packets, PacketMeta{
				Cursor:      fmt.Sprintf("%d", accessUnitCount),
				Offset:      offset,
				Size:        len(unitData),
				IsKeyframe:  currentIsKeyframe,
				TimestampUs: 0,
				DurationUs:  0,
			})
			payload = append(payload, unitData...)
			offset += len(unitData)
			packetCount++
		}
		accessUnitCount++
		currentStart = -1
		currentEnd = -1
		currentHasVCL = false
		currentIsKeyframe = false
		return packetCount >= limit
	}

	for _, unit := range units {
		startNewUnit := false
		if currentStart >= 0 {
			if unit.typeID == 9 {
				startNewUnit = true
			} else if currentHasVCL && isH264VCLNAL(unit.typeID) {
				startNewUnit = true
			}
		}

		if startNewUnit && flushCurrent() {
			break
		}

		if currentStart < 0 {
			currentStart = unit.start
		}
		currentEnd = unit.end
		if isH264VCLNAL(unit.typeID) {
			currentHasVCL = true
			if unit.typeID == 5 {
				currentIsKeyframe = true
			}
		}
	}

	if packetCount < limit {
		flushCurrent()
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

// countNALUnits counts total H.264 access units in Annex B bitstream (for cursor validation).
func countNALUnits(data []byte) int {
	return countH264AccessUnits(data)
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

// seekVideoPackets seeks to a time position by delegating to extractVideoPackets
// with a seek-encoded cursor, and returns the first actual IDR packet cursor.
func (m *FFmpegSessionManager) seekVideoPackets(filePath string, trackID int, timeMs int64) ([]PacketMeta, []byte, int64, error) {
	cursor := fmt.Sprintf("%d:0", timeMs)
	packets, payload, _, err := m.extractVideoPackets(filePath, trackID, cursor, 128)
	if err != nil {
		return nil, nil, timeMs, err
	}

	for _, packet := range packets {
		if packet.IsKeyframe {
			return []PacketMeta{packet}, payload, timeMs, nil
		}
	}

	if len(packets) > 0 {
		return []PacketMeta{packets[0]}, payload, timeMs, nil
	}

	return nil, payload, timeMs, nil
}

// seekAudioPackets seeks to a time position by delegating to extractAudioPackets.
func (m *FFmpegSessionManager) seekAudioPackets(filePath string, trackID int, timeMs int64) ([]PacketMeta, []byte, int64, error) {
	cursor := fmt.Sprintf("%d:0", timeMs)
	packets, payload, _, err := m.extractAudioPackets(filePath, trackID, cursor, 1)
	return packets, payload, timeMs, err
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
