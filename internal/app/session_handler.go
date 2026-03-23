package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"wails-app/internal/demuxer"
	"wails-app/internal/pathref"
)

// Session request/response types
type sessionOpenRequest struct {
	Path string `json:"path"`
	// Archive-backed media
	FSPath     string   `json:"fs_path,omitempty"`
	NestedPath []string `json:"nested,omitempty"`
	EntryPath  string   `json:"entry,omitempty"`
}

type sessionPacketsRequest struct {
	SessionID string `json:"session_id"`
	TrackID   int    `json:"track_id"`
	Cursor    string `json:"cursor"`
	Limit     int    `json:"limit"`
}

// serveSessionRequest routes media session protocol requests
func serveSessionRequest(w http.ResponseWriter, r *http.Request, app *App) {
	setCORSHeaders(w)

	// Extract the operation from the path: /wails-stream/session/{operation}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "invalid session path", http.StatusBadRequest)
		return
	}

	operation := parts[3]

	switch operation {
	case "open":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleSessionOpen(w, r, app)

	case "packets":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleSessionPackets(w, r, app)

	case "seek":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleSessionSeek(w, r, app)

	case "close":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleSessionClose(w, r, app)

	default:
		http.Error(w, "unknown operation", http.StatusNotFound)
	}
}

// handleSessionOpen creates a new media session
func handleSessionOpen(w http.ResponseWriter, r *http.Request, app *App) {
	var req sessionOpenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// For now, only support disk paths
	if req.Path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	// Normalize the path
	path := pathref.Normalize(req.Path)

	// Open the media session
	info, err := app.MediaSessionManager().OpenPath(path)
	if err != nil {
		http.Error(w, "cannot open media: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return session info as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info) //nolint:errcheck
}

// handleSessionPackets fetches a batch of codec packets
func handleSessionPackets(w http.ResponseWriter, r *http.Request, app *App) {
	// Parse query parameters
	sessionID := r.URL.Query().Get("session_id")
	trackIDStr := r.URL.Query().Get("track_id")
	cursor := r.URL.Query().Get("cursor")
	limitStr := r.URL.Query().Get("limit")

	if sessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	trackID := 0
	if trackIDStr != "" {
		var err error
		trackID, err = strconv.Atoi(trackIDStr)
		if err != nil {
			http.Error(w, "invalid track_id", http.StatusBadRequest)
			return
		}
	}

	limit := 64 // default
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
	}

	// Fetch packet batch
	batch, err := app.MediaSessionManager().ReadPackets(sessionID, trackID, cursor, limit)
	if err != nil {
		http.Error(w, "cannot read packets: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Send binary framed response: [4-byte header length][JSON header][binary payload]
	w.Header().Set("Content-Type", "application/octet-stream")
	writeFramedPacketBatch(w, batch)
}

// handleSessionSeek seeks to a time position
func handleSessionSeek(w http.ResponseWriter, r *http.Request, app *App) {
	var req struct {
		SessionID string `json:"session_id"`
		TrackID   int    `json:"track_id"`
		TimeMs    int64  `json:"time_ms"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	// Perform seek
	result, err := app.MediaSessionManager().Seek(req.SessionID, req.TrackID, req.TimeMs)
	if err != nil {
		http.Error(w, "cannot seek: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return seek result as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result) //nolint:errcheck
}

// handleSessionClose closes a media session
func handleSessionClose(w http.ResponseWriter, r *http.Request, app *App) {
	var req struct {
		SessionID string `json:"session_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		http.Error(w, "session_id required", http.StatusBadRequest)
		return
	}

	// Close the session
	err := app.MediaSessionManager().Close(req.SessionID)
	if err != nil {
		http.Error(w, "cannot close session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "closed"}) //nolint:errcheck
}

// writeFramedPacketBatch writes a binary-framed packet batch:
// [4-byte LE header length][JSON header][binary payload]
func writeFramedPacketBatch(w http.ResponseWriter, batch *demuxer.PacketBatch) {
	// Marshal the header to JSON
	headerJSON, err := json.Marshal(map[string]interface{}{
		"session_id":  batch.SessionID,
		"track_id":    batch.TrackID,
		"next_cursor": batch.NextCursor,
		"packets":     batch.Packets,
	})
	if err != nil {
		http.Error(w, "cannot marshal header", http.StatusInternalServerError)
		return
	}

	// Write header: 4-byte length in little-endian
	headerLen := len(headerJSON)
	w.Write([]byte{
		byte(headerLen),
		byte(headerLen >> 8),
		byte(headerLen >> 16),
		byte(headerLen >> 24),
	})

	// Write JSON header
	w.Write(headerJSON)

	// Write binary payload
	w.Write(batch.Payload)
}
