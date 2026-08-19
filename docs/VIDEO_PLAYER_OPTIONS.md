# Video Player Implementation Options

## Current Direction

The codebase has already committed to a canvas-only playback surface for video preview. The main architectural question is no longer whether to use HTML5 video, but how to transport media in a way that preserves reliable seeking.

The current conclusion is:


## Comparison Matrix

### Option 1: **Stick with HTML5 Video (Baseline)**
Pros:

Cons:

**Effort**: Zero  
**Format Support**: ~60% (mp4, webm, mov)  
**Perf**: Best  
**User Experience**: Common formats work, exotic formats fail


### Option 2: **Canvas + WebCodecs (Current Baseline)**
Pros:

Cons:

**Effort**: 2-4 weeks  
**Format Support**: ~95% (everything with modern browser)  
**Perf**: Good (hardware accelerated most of the time)  
**User Experience**: Broad format support, but seek quality depends on transport mode


### Option 2A: **Canvas + WebCodecs + Seekable File Transport**

Pros:

Cons:

**Effort**: Low incremental effort  
**Format Support**: High, bounded by WebCodecs and demux support  
**Perf**: Best within the current architecture  
**User Experience**: Good scrubbing and stable decode recovery


### Option 2B: **Canvas + WebCodecs + Restart-On-Seek Transcoding**

Pros:

Cons:

**Effort**: Moderate  
**Format Support**: High for transcoded content  
**Perf**: Good for playback, acceptable for discrete seeks  
**User Experience**: Predictable seek behavior instead of partial/random success


### Option 3: **Canvas + FFmpeg.wasm (Maximum Compatibility)**
Pros:

Cons:

**Effort**: 1-2 weeks (simpler demuxing)  
**Format Support**: 100%  
**Perf**: Poor (software decoding)  
**User Experience**: Works everywhere but might be laggy


### Option 4: **Hybrid (WebCodecs + FFmpeg.wasm Fallback)**
Pros:

Cons:

**Effort**: 3-5 weeks  
**Format Support**: 100%  
**Perf**: Good (hybrid strategy)  
**User Experience**: Excellent, but heavy


## Recommendation: **Hybrid by Source Seekability**

### Why?
1. **Root cause fit**: The current seek issues are transport-related, not just decoder-related.
2. **Direct files already have the right shape**: HTTP range requests map naturally to demuxer seeking.
3. **Sequential transcodes do not**: A single ffmpeg pipe cannot reliably support arbitrary back-and-forth seeking.
4. **Smaller change surface**: Restart-on-seek is much simpler than inventing a chunk protocol or moving immediately to HLS/DASH.
5. **Clear UX contract**: Sources can advertise whether they support full scrubbing or restart-based seeking.

### Decision Tree

```
User opens video file
  ├─ Source is byte-seekable?
  │   └─ Use canvas player with range-backed transport
  │
  ├─ Source requires live transcode?
  │   ├─ Initial playback → sequential transcode stream
  │   └─ Seek → restart transcode near requested timestamp
  │
  └─ Source is neither seekable nor restartable?
    └─ Offer limited preview semantics or disable scrubbing
```


## Implementation Checklist for Option 2

### Stage 1: Infrastructure (1 week)

### Stage 2: Core Playback (1 week)

### Stage 3: Playback Controls (1 week)

### Stage 4: Transport Semantics (1 week)


## Code Structure for Option 2

```
frontend/src/app/services/
├── video-decoder/
│   ├── codec-detector.service.ts
│   ├── canvas-video-player.ts
│   ├── frame-buffer.ts
│   ├── audio-sync.ts
│   └── demuxer-adapter.ts
└── video-player.service.ts (orchestrator)

frontend/src/app/components/
├── canvas-video-player/
│   ├── canvas-video-player.component.ts
│   ├── playback-controls.component.ts
│   └── canvas-video-player.component.scss
```


## Library Recommendations for Option 2

### Primary Dependencies
```json
{
  "dependencies": {
    "@vscode/webdemuxer": "^0.1.0",
    "tsdemux": "^2.0.0"  // Alternative
  }
}
```

### No Heavy Downloads Needed


## Risk Analysis

### High Risk Items
| Risk | Mitigation |
|------|-----------|
| Audio sync drift | Timestamp-based sync, periodic resync |
| Seeking precision | Use range-backed demuxing for seekable sources; restart transcode for sequential sources |
| Codec not available | Check MediaCapabilities before init |
| Container parsing fails | Validate file format, show error |
| Sequential stream scrub instability | Do not promise full random seek inside one ffmpeg pipe |

### Testing Strategy
1. Test each format category (MP4, WebM, MKV, AVI, etc.)
2. Seek forward/backward randomly
3. Pause/Resume cycles
4. Long files (>1 hour)
5. High bitrate (4K)
6. Edge cases (corrupted files, truncated)


## Timeline Estimate

| Phase | Duration | Milestone |
|-------|----------|-----------|
| Research & Proto | 3-5 days | Proof of concept with one format |
| Core Impl | 5-7 days | Play any WebCodecs format |
| UI & Controls | 5-7 days | Polished playback experience |
| Testing & Polish | 3-5 days | Error handling, edge cases |
| **Total** | **2-3 weeks** | Production ready |


## File Manager Integration Points

### 1. Format Detection (in `viewer-surface.component.ts`)
```typescript
// Current
if (ext === 'mkv') this.kind = 'unsupported';

// New
if (ext === 'mkv' && this.hasWebCodecsSupport()) {
  this.kind = 'video';
  this.useCanvasPlayer = true;  // Signal to use Canvas player
} else if (ext === 'mkv') {
  this.kind = 'unsupported';  // Fallback
}
```

### 2. Mime Type Expansion
```typescript
private inferVideoMime(ext: string): string | null {
  // Existing logic handles mp4, webm, mov
  // Add new codecs for WebCodecs usage
  return { /* current */ ...{
    mkv: 'video/x-matroska',
    avi: 'video/x-msvideo',
    ts: 'video/mp2t',
    mxf: 'application/mxf',
    // Add dynamically based on detected codecs inside
  }}[ext] ?? null;
}
```

### 3. Streaming URL Logic
```typescript
async getRawStreamUrl(path: string): Promise<string> {
  // Existing: /wails-stream URL for direct seekable playback
  return `${API_BASE}/wails-stream?path=${path}`;
}
```

For transcoded playback, the URL shape needs to express playback mode explicitly:

```typescript
// Initial playback
`${API_BASE}/wails-stream?path=${path}&transcode=mp4&mode=transcode`

// Restart-based seek
`${API_BASE}/wails-stream?path=${path}&transcode=mp4&mode=transcode&start=${seconds}`
```


## Go Backend Changes

The backend already supports both of the transport classes the frontend needs, but they should be treated differently:


That means the main change is not low-level chunk formatting. The main change is making seek behavior mode-aware.

```go
// Direct file path: true byte-range seek support
// Transcode path: sequential output, optional restart with ?start=<seconds>
```

### Optional Enhancement
```go
// Add metadata endpoint for quick format detection
GET /api/video-metadata/{path}
└─ Returns: { format: "mkv", width: 1920, height: 1080, duration: 120000, codec: "h264" }
```


## Next Steps

### Immediate
1. Review @vscode/webdemuxer or tsdemux APIs
2. Create proof-of-concept with MKV file
3. Benchmark WebCodecs decode time
4. Estimate final effort

### This Week

### Implementation