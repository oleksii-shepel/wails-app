package demuxer

// SourceDescriptor describes where media is located
type SourceDescriptor struct {
	// Disk file path (drive:// or native)
	Path string

	// Archive-backed media (future: requires spool/cache)
	FSPath     string   // Archive file path
	NestedPath []string // Inner archives (tar in zip, etc.)
	EntryPath  string   // File within archive
}

// SessionInfo contains metadata after opening a media session
type SessionInfo struct {
	SessionID    string
	Title        string
	DurationMs   int64
	Seekable     bool
	Tracks       []TrackInfo
	SeekPointsMs []int64 // Coarse keyframe timeline
}

// TrackInfo describes a single audio/video/subtitle track
type TrackInfo struct {
	ID    int
	Kind  string // "video", "audio", "subtitle"
	Codec string // WebCodecs-compatible codec string (e.g., "avc1.640028")

	// Video
	Width  int
	Height int
	FPS    float64

	// Audio
	SampleRate int
	Channels   int

	// Codec-specific initialization data (base64-encoded)
	// May contain SPS/PPS for AVC, AudioSpecificConfig for AAC, etc.
	DescriptionB64 string

	// Bitstream format tells the frontend how packets are framed
	BitstreamFormat string // "annexb", "avcc", "raw", etc.
	HasBFrames      bool
	KeyframeCount   int
}

// PacketBatch is a response containing multiple packets
type PacketBatch struct {
	SessionID  string
	TrackID    int
	NextCursor string       // Cursor for next batch request
	Packets    []PacketMeta // Metadata for each packet
	Payload    []byte       // Concatenated packet binary data
}

// PacketMeta describes a single packet within a batch
type PacketMeta struct {
	Cursor      string // Opaque backend cursor
	Offset      int    // Byte offset in Payload
	Size        int    // Byte size of packet
	TimestampUs int64  // Microseconds (matches WebCodecs)
	DurationUs  int64  // Microseconds
	IsKeyframe  bool
}

// SeekResult is the response to a seek request
type SeekResult struct {
	SessionID      string
	ResolvedTimeMs int64  // Actual seek position (usually on a keyframe)
	Cursor         string // Packet cursor for resumed playback
	IsKeyframe     bool
}

// SessionManager handles opening, reading, seeking media sessions
type SessionManager interface {
	// OpenPath creates a new media session from a disk file or VFS source
	OpenPath(path string) (*SessionInfo, error)

	// ReadPackets fetches the next batch of packets from a session
	// trackID: which audio/video track
	// cursor: opaque position marker (from SessionInfo or previous ReadPackets)
	// limit: max packets to return
	ReadPackets(sessionID string, trackID int, cursor string, limit int) (*PacketBatch, error)

	// Seek resolves a time position to a keyframe or random access point
	Seek(sessionID string, trackID int, timeMs int64) (*SeekResult, error)

	// Close terminates a session and frees resources
	Close(sessionID string) error
}
