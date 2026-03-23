# Video Player Implementation Options

## Comparison Matrix

### Option 1: **Stick with HTML5 Video (Baseline)**
Pros:
- ✅ Minimal code, already works
- ✅ Excellent performance out-of-box
- ✅ Works in older browsers
- ✅ Custom controls easy to add

Cons:
- ❌ Limited to browser's native codec support
- ❌ MKV, MXF, AVI often fail
- ❌ No container flexibility

**Effort**: Zero  
**Format Support**: ~60% (mp4, webm, mov)  
**Perf**: Best  
**User Experience**: Common formats work, exotic formats fail

---

### Option 2: **Canvas + WebCodecs (Recommended)**
Pros:
- ✅ Supports 95%+ of video formats
- ✅ Hardware acceleration (native WebCodecs)
- ✅ Modern, performant, web standard
- ✅ Can fall back to HTML5 video
- ✅ Progressive enhancement

Cons:
- ❌ Moderate implementation effort
- ❌ Need to build playback UI
- ❌ Requires `navigator.mediaCapabilities` API
- ❌ Older browsers fall back to HTML5

**Effort**: 2-4 weeks  
**Format Support**: ~95% (everything with modern browser)  
**Perf**: Good (hardware accelerated most of the time)  
**User Experience**: "Just Works" for almost everything

---

### Option 3: **Canvas + FFmpeg.wasm (Maximum Compatibility)**
Pros:
- ✅ Supports literally every format
- ✅ Pure software codec (no browser limitations)
- ✅ Works even in older browsers (IE11 fallback possible)

Cons:
- ❌ Heavy library (8-20 MB download)
- ❌ Slow software decoding (1080p = 10-20ms per frame)
- ❌ High CPU usage (fans spin up)
- ❌ Poor mobile experience
- ❌ Privacy concerns (file reading in JS)

**Effort**: 1-2 weeks (simpler demuxing)  
**Format Support**: 100%  
**Perf**: Poor (software decoding)  
**User Experience**: Works everywhere but might be laggy

---

### Option 4: **Hybrid (WebCodecs + FFmpeg.wasm Fallback)**
Pros:
- ✅ Best of both worlds
- ✅ 95%+ formats with hardware accel (WebCodecs)
- ✅ 100% format support with fallback (FFmpeg)
- ✅ Graceful degradation

Cons:
- ❌ Most complex implementation
- ❌ Large bundle size (if both included)
- ❌ Complex error handling & fallback logic

**Effort**: 3-5 weeks  
**Format Support**: 100%  
**Perf**: Good (hybrid strategy)  
**User Experience**: Excellent, but heavy

---

## Recommendation: **Option 2 (Canvas + WebCodecs)**

### Why?
1. **Sweet Spot**: 95% format support without bloat
2. **Performance**: Hardware acceleration on modern browsers (95%+ market share)
3. **Maintainability**: Standard web APIs, no vendor lock-in
4. **User Experience**: Progressive enhancement—works great for 95%, HTML5 fallback for legacy
5. **Effort/Reward**: Best ROI for your file manager use case

### Decision Tree

```
User opens video file
    ├─ MP4/WebM/MOV?
    │   └─ Use HTML5 video (existing code)
    │
    ├─ MKV/AVI/TS/FLV?
    │   ├─ Modern browser? → Canvas + WebCodecs
    │   └─ Old browser? → Try HTML5, show "unsupported"
    │
    └─ Exotic? (MXF, etc)
        └─ Canvas + WebCodecs only
```

---

## Implementation Checklist for Option 2

### Stage 1: Infrastructure (1 week)
- [ ] Research @vscode/webdemuxer or tsdemux library
- [ ] Create `CanvasVideoPlayer` service
- [ ] Implement MediaCapabilities codec detection
- [ ] Build frame buffer management

### Stage 2: Core Playback (1 week)
- [ ] Demux container → extract video/audio tracks
- [ ] Initialize VideoDecoder for primary codec
- [ ] Implement frame decoding loop
- [ ] Sync audio track (AudioContext)
- [ ] Basic canvas rendering

### Stage 3: Playback Controls (1 week)
- [ ] Custom playback UI (play, pause, seek)
- [ ] Time display & progress bar
- [ ] Volume control
- [ ] Fullscreen support

### Stage 4: Polish & Fallback (1 week)
- [ ] Error handling (unsupported codec → show error)
- [ ] Fallback to HTML5 video when WebCodecs unavailable
- [ ] Format detection & auto-selection
- [ ] Performance monitoring

---

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

---

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
- WebCodecs API is built-in to modern browsers
- Canvas 2D is built-in
- AudioContext is built-in

---

## Risk Analysis

### High Risk Items
| Risk | Mitigation |
|------|-----------|
| Audio sync drift | Timestamp-based sync, periodic resync |
| Seeking precision | Build keyframe index on file load |
| Codec not available | Check MediaCapabilities before init |
| Container parsing fails | Validate file format, show error |

### Testing Strategy
1. Test each format category (MP4, WebM, MKV, AVI, etc.)
2. Seek forward/backward randomly
3. Pause/Resume cycles
4. Long files (>1 hour)
5. High bitrate (4K)
6. Edge cases (corrupted files, truncated)

---

## Timeline Estimate

| Phase | Duration | Milestone |
|-------|----------|-----------|
| Research & Proto | 3-5 days | Proof of concept with one format |
| Core Impl | 5-7 days | Play any WebCodecs format |
| UI & Controls | 5-7 days | Polished playback experience |
| Testing & Polish | 3-5 days | Error handling, edge cases |
| **Total** | **2-3 weeks** | Production ready |

---

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
  // Existing: /wails-stream URL for range requests
  // Canvas player can use same URL and fetch chunks!
  return `${API_BASE}/wails-stream?path=${path}`;
}
```

---

## Go Backend Changes (Minimal)

Your `/wails-stream` endpoint already supports range requests—perfect for Canvas player!
```go
// No changes needed—Canvas player can fetch file in chunks
// using Range: bytes=0-1024 headers
```

### Optional Enhancement
```go
// Add metadata endpoint for quick format detection
GET /api/video-metadata/{path}
└─ Returns: { format: "mkv", width: 1920, height: 1080, duration: 120000, codec: "h264" }
```

---

## Next Steps

### Immediate
1. Review @vscode/webdemuxer or tsdemux APIs
2. Create proof-of-concept with MKV file
3. Benchmark WebCodecs decode time
4. Estimate final effort

### This Week
- [ ] Decision: Go with Option 2?
- [ ] Create detailed task breakdown
- [ ] Plan sprint schedule

### Implementation
- [ ] Start with Proto or Stage 1
- [ ] Get playback working for MKV
- [ ] Iterate with feedback
