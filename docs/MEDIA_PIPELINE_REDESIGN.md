# Media Pipeline Redesign: Backend Demux + Frontend Decode

## Executive Summary

**Goal**: Go owns source access, indexing, demuxing, and packet delivery. The frontend owns decoding, A/V scheduling, and rendering via WebCodecs.

**What this improves**:
- Lower frontend complexity: container parsing and seek logic move out of TypeScript.
- Better playback behavior: backend can build indexes, jump to keyframes, and avoid repeated browser-side scans.
- Better source coverage: the same backend path can serve disk files, mounted VFS sources, and archives when the source can be made seekable.
- Better runtime characteristics: less JS CPU and memory pressure on the UI thread.

**What this does not automatically improve**:
- It does **not** guarantee dramatically lower byte volume. Packetized elementary streams are often in the same order of magnitude as the original media payload. The win is smarter access and less frontend work, not a guaranteed 75% traffic reduction.
- It does **not** remove the need for a custom media endpoint. Wails asset serving is still the wrong abstraction for a stateful media protocol.

**Effort**: moderate. The hard parts are seekability, packet/index caching, codec-specific bitstream packaging, and audio/video sync.

---

## Decision: AssetHandler vs App-Owned Media Endpoint

If by `AssetHandler` you mean Wails' embedded asset serving path, **do not build the media protocol on top of it as if it were a static file server**.

Use the existing app-managed HTTP surface for media requests instead:
- Keep a dedicated app-owned route such as `/wails-stream`.
- Keep it tied to the app lifecycle, not to a separately deployed service.
- Treat it as a custom protocol endpoint with its own handlers, session state, and caches.

That gives you the benefit you want:
- no separate external media server to deploy,
- but still full control over routing, binary framing, sessions, and backend caches.

So the answer is:
- **Yes**, you can avoid a separate standalone service.
- **No**, `AssetHandler` by itself is not the right primitive for demux/index/packet protocol logic.

---

## Recommended Architecture

```
Frontend viewer
    -> open media session
    -> receive track metadata + codec config + seek map summary
    -> request packet batches by cursor
    -> decode with WebCodecs
    -> render video / schedule audio

Go media pipeline
    -> resolve path via disk / VFS / archive layer
    -> ensure source is seekable
    -> open demux session
    -> build or load packet/keyframe index
    -> serve packet batches and seek results
    -> close/evict idle sessions
```

### Why session-based instead of plain stateless requests

Re-opening a demuxer on every request is the wrong cost profile.

You want a session because it lets the backend keep:
- the opened demux context,
- stream metadata,
- keyframe index,
- packet cursors,
- temporary spool/cache state for non-native sources.

Without a session, seek and packet requests either become expensive or force you to rebuild too much state per call.

---

## Protocol Shape

Keep `/wails-stream`, but evolve it into a session protocol.

### 1. Open Session

**Request**

`POST /wails-stream/session/open`

Body:

```json
{
  "path": "drive://C:/video.mkv"
}
```

Archive/VFS variants can use a different source descriptor:

```json
{
  "fs_path": "drive://C:/archive.zip",
  "nested": ["inner.tar"],
  "entry": "movie.mkv"
}
```

**Response**

```json
{
  "session_id": "m_8f3b6b8d",
  "title": "video.mkv",
  "duration_ms": 120000,
  "seekable": true,
  "tracks": [
    {
      "id": 0,
      "kind": "video",
      "codec": "avc1.640028",
      "bitstream_format": "annexb",
      "width": 1920,
      "height": 1080,
      "fps": 30.0,
      "description_b64": "...",
      "has_b_frames": true,
      "keyframe_count": 312
    },
    {
      "id": 1,
      "kind": "audio",
      "codec": "mp4a.40.2",
      "sample_rate": 48000,
      "channels": 2,
      "description_b64": "..."
    }
  ],
  "seek_points_ms": [0, 2033, 4010, 6021]
}
```

Notes:
- Return **WebCodecs-usable codec strings**, not generic labels like `h264` or `aac`.
- Return whether the delivered bitstream is `annexb`, `avcc`, or another codec-specific format the frontend must honor.
- `description_b64` is codec-specific initialization data, not always required and not always sufficient on its own.

### 2. Read Packets

**Request**

`GET /wails-stream/session/packets?session_id=m_8f3b6b8d&track_id=0&cursor=120&limit=64`

**Response**

Use binary framing, not text splitting.

Recommended wire format:
- 4-byte little-endian header length
- JSON header bytes
- concatenated packet payload bytes

Header example:

```json
{
  "session_id": "m_8f3b6b8d",
  "track_id": 0,
  "next_cursor": 184,
  "packets": [
    {
      "cursor": 120,
      "offset": 0,
      "size": 2048,
      "timestamp_us": 3333000,
      "duration_us": 33333,
      "is_keyframe": false
    },
    {
      "cursor": 121,
      "offset": 2048,
      "size": 834,
      "timestamp_us": 3366333,
      "duration_us": 33333,
      "is_keyframe": false
    }
  ]
}
```

Notes:
- Use microseconds in the protocol because that matches WebCodecs timestamps directly.
- `cursor` should be a backend-defined opaque packet position or stable index within the session.
- For audio/video playback, track packets can be fetched independently, but the shared session timeline must stay authoritative.

### 3. Seek

**Request**

`POST /wails-stream/session/seek`

```json
{
  "session_id": "m_8f3b6b8d",
  "track_id": 0,
  "time_ms": 45000
}
```

**Response**

```json
{
  "session_id": "m_8f3b6b8d",
  "resolved_time_ms": 44988,
  "cursor": 449,
  "is_keyframe": true
}
```

Notes:
- Seek should resolve to the nearest decodable point, usually the prior random access point.
- The frontend should flush and rebuild decoders as needed, then resume from the returned cursor.

### 4. Close Session

**Request**

`POST /wails-stream/session/close`

```json
{
  "session_id": "m_8f3b6b8d"
}
```

Also expire sessions automatically after an idle timeout.

---

## Source Layer Requirements

This is the main architectural constraint.

Most demuxers expect a source that behaves like:
- `Read`,
- `Seek`,
- often effectively `ReadAt` or equivalent random access.

That has consequences for your existing source types.

### Disk files

Easy case. Native file access is already seekable.

### Mounted VFS sources

Depends on the adapter.

If the VFS adapter supports efficient random access, it can back demuxing directly.

If it is only sequential or expensive to seek remotely, you will likely need:
- a bounded local spool file, or
- a random-access block cache.

### Archive entries

This is the biggest correction to the draft.

Your current archive stream path explicitly serves archive entries as non-seekable streams. That is fine for raw file download, but **not** fine for a general demux/seek pipeline.

For archive-backed media, you need one of these:
- archive implementation with efficient random access into the entry,
- temporary extraction to a local spool file,
- indexed block cache that can emulate seekable reads.

Without that, backend demuxing for archive entries will be fragile or impossible for many formats.

---

## Backend Implementation

### 1. Demux backend choice

The original draft listed `ffmpeg-go` and `go-echarts/video`. That is not a good basis for this design.

#### `ffmpeg-go`

Do **not** treat it as the core demux API for packet streaming.

Why:
- it is oriented around building ffmpeg command graphs,
- it is process-based,
- it is poor for maintaining an in-process packet/session model,
- it is awkward for repeated packet-range reads and stable cursors.

It may still be useful for quick probing or experiments, but not as the real transport engine.

#### Better production options

Use one of these approaches:

1. **Direct libavformat/libavcodec bindings in-process**
   - Example family: `go-astiav`.
   - Best fit for session-based demux, indexing, packet reads, and seek control.
   - Cost: CGO, FFmpeg binaries/libs, Windows toolchain complexity.

2. **Long-lived helper process you own**
   - A separate local helper process, started by the app, not a separately deployed server.
   - Communicates over pipes or localhost IPC.
   - Better operational isolation than CGO in the main process.
   - Good fallback if Windows FFmpeg linking becomes painful.

Recommendation:
- If you want the cleanest long-term architecture, use direct libav bindings.
- If you want lower risk on Windows, consider a long-lived helper process before committing to deep CGO in the main app.

### 2. Core interfaces

The interface should be session-oriented, not `Init(filePath)` on every call.

```go
type SessionManager interface {
    OpenPath(ctx context.Context, source SourceDescriptor) (*SessionInfo, error)
    ReadPackets(ctx context.Context, sessionID string, trackID int, cursor string, limit int) (*PacketBatch, error)
    Seek(ctx context.Context, sessionID string, trackID int, timeMs int64) (*SeekResult, error)
    Close(ctx context.Context, sessionID string) error
}

type SessionInfo struct {
    SessionID   string
    DurationMs  int64
    Seekable    bool
    Tracks      []TrackInfo
    SeekPoints  []int64
}

type TrackInfo struct {
    ID              int
    Kind            string
    Codec           string
    BitstreamFormat string
    Width           int
    Height          int
    SampleRate      int
    Channels        int
    Description     []byte
}

type PacketBatch struct {
    NextCursor string
    Packets    []PacketMeta
    Payload    []byte
}

type PacketMeta struct {
    Cursor       string
    Offset       int
    Size         int
    TimestampUs  int64
    DurationUs   int64
    IsKeyframe   bool
}
```

### 3. Caching

Do **not** reuse the archive cache as the primary demuxer cache.

Use separate caches for separate lifetimes:
- archive/source cache,
- demux session cache,
- packet index cache,
- optional spool/block cache for non-native sources.

That separation matters because they have different memory pressure and eviction behavior.

### 4. Stream handler changes

Keep the current file-streaming behavior as a fallback path.

Add a new protocol branch under the same route family, for example:
- `/wails-stream/session/open`
- `/wails-stream/session/packets`
- `/wails-stream/session/seek`
- `/wails-stream/session/close`

That lets legacy viewers continue to use raw file URLs while the new canvas player opts into the session protocol.

---

## Frontend Implementation Notes

### 1. Binary parsing

The original draft used `resp.text()` and `Buffer.from(...)`. That is wrong for browser-side binary protocol handling.

Use `arrayBuffer()` and parse the framed payload explicitly.

```typescript
async function readPacketBatch(url: string): Promise<PacketBatch> {
  const response = await fetch(url);
  const buffer = await response.arrayBuffer();
  const view = new DataView(buffer);
  const headerLength = view.getUint32(0, true);

  const headerBytes = new Uint8Array(buffer, 4, headerLength);
  const payload = new Uint8Array(buffer, 4 + headerLength);
  const headerText = new TextDecoder().decode(headerBytes);
  const header = JSON.parse(headerText);

  return {
    packets: header.packets,
    nextCursor: header.next_cursor,
    payload,
  };
}
```

### 2. WebCodecs config details

Do not assume:
- `codec: "h264"` is enough,
- `codec: "aac"` is enough,
- `description` is always required,
- one config shape works across AVC/HEVC/AV1/VP9.

Instead:
- return canonical codec strings such as `avc1.640028` or `mp4a.40.2`,
- return codec initialization data only when required,
- return whether the payload is `annexb` or length-prefixed,
- keep codec-specific mapping logic in one frontend adapter layer.

### 3. Decoder API behavior

`VideoDecoder.decode()` and `AudioDecoder.decode()` are synchronous calls that enqueue work. Do not `await` them.

The async boundaries are around:
- `flush()`,
- stream fetches,
- your own scheduling and seek orchestration.

### 4. Audio/video sync

Moving demuxing to Go does **not** remove the need for frontend scheduling.

The frontend still needs to:
- manage a playback clock,
- feed decoders ahead of time,
- render decoded frames at the correct moment,
- schedule decoded audio against `AudioContext`.

The backend simplifies packet acquisition and seek correctness. It does not replace playback control.

---

## Migration Path

### Current state

- Browser-side container parsing and packet extraction live in the canvas player.
- `/wails-stream` already exists for raw file/archive access.
- Archive entry streaming is currently non-seekable.

### Target state

- Existing raw streaming remains for simple viewers and fallback.
- New canvas player uses media sessions under `/wails-stream/session/*`.
- Media demux/index logic moves to Go.
- Frontend keeps decode/render responsibilities only.

### Suggested rollout

1. Implement session open + metadata only.
2. Implement video packet fetch for seekable disk files only.
3. Switch one video path in the canvas player behind a feature flag.
4. Add keyframe seek.
5. Add audio.
6. Add VFS sources that are truly seekable.
7. Add archive-backed playback only after you have a spool/cache strategy.

This sequence avoids overcommitting before the hardest I/O case is solved.

---

## Performance Expectations

The realistic wins are:
- lower JS parse/demux cost,
- smaller frontend code surface,
- more reliable seeks,
- better reuse of cached indexes,
- less duplicate work between seeks/reloads.

The wins that should be treated as conditional, not promised:
- time-to-first-frame depends on how quickly session open can probe metadata and acquire init data,
- seek latency depends on whether the index is already cached,
- byte volume depends on whether you skip unused tracks and how much framing overhead you add.

So avoid hard promises like:
- fixed `500-800ms` startup,
- fixed `200-300ms` seek,
- fixed `15-25%` network usage.

Those numbers may be achievable in some cases, but they are not architectural guarantees.

---

## Main Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Non-seekable archive and remote sources | High | Add spool file or random-access block cache before supporting them |
| Windows FFmpeg integration complexity | High | Prefer one validated binding/helper approach early and test packaging immediately |
| Codec-specific bitstream packaging | High | Normalize per codec: AVC, HEVC, AV1, VP9, AAC |
| Session/file handle leaks | High | Idle TTL, explicit close path, bounded cache sizes |
| Audio/video drift | Medium | Keep shared timestamps authoritative and schedule audio from decoded output clock |
| Overly chatty HTTP packet fetches | Medium | Batch packets and prefetch ahead |

---

## Recommended Next Step

Build the narrowest useful slice first:

1. Disk-file-only media session open.
2. Video-track packet fetch for one codec path you already know works in WebCodecs.
3. Frontend adapter that consumes framed binary packet batches.
4. Keyframe seek for that same path.

That will validate the architecture without forcing archive/VFS/audio complexity on day one.

---

## Conclusion

The architecture direction is sound:
- Go as media pipeline engine,
- frontend as decoder + renderer,
- custom protocol over the app-owned `/wails-stream` surface.

The corrected version of the plan is:
- **do not** rely on Wails asset serving as the media engine,
- **do not** use per-request stateless demuxing,
- **do not** assume archive entries are ready for seek-heavy demuxing,
- **do** build a session-based protocol with explicit source, seek, packet, and close semantics,
- **do** keep the frontend focused on WebCodecs decode and playback scheduling.
