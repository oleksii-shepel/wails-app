# Canvas + WebCodecs Video Player Architecture

## Overview
A custom video player using Canvas for rendering, WebCodecs for decoding, and WebDemuxer for container parsing. This enables playback of virtually any video format supported by modern browsers.

## Architecture Layers

### 1. Container Parsing (WebDemuxer)
**Library**: `@vscode/webdemuxer` (or equivalent)
- Demuxes container formats and extracts video/audio tracks
- Supported containers:
  - MP4 (H.264, H.265, VP9, AV1)
  - WebM (VP8, VP9, AV1, Opus, Vorbis)
  - Matroska/MKV (any codec inside)
  - AVI, MOV, FLV, TS, M3U8

### 2. Decoding (WebCodecs API)
**Standard Spec**: MediaCapabilities + VideoDecoder API
- Detects device codec support at runtime
- Hardware acceleration (when available)
- Supported codecs:
  - **H.264/AVC, H.265/HEVC** (common in MP4)
  - **VP8, VP9** (WebM, Matroska)
  - **AV1** (modern high-efficiency)
  - **Audio**: AAC, Opus, Vorbis, MP3, FLAC

### 3. Rendering (Canvas 2D)
- `canvas.getContext('2d')` for frame display
- Frame-by-frame control via RequestAnimationFrame
- Optional: HDR via Canvas with ImageData enhancements (future)

### 4. Audio Playback (WebAudio / AudioContext)
- Parallel audio track decoding
- Sync with video rendering via timestamps

## Implementation Strategy

### Phase 1: Core Player (Canvas + WebCodecs)
```typescript
interface VideoPlayer {
  // Initialization
  initialize(containerData: ArrayBuffer): Promise<void>;
  getMetadata(): VideoMetadata;
  
  // Playback control
  play(): Promise<void>;
  pause(): void;
  seek(timeMs: number): Promise<void>;
  
  // Rendering
  render(): void;
  
  // Properties
  currentTime: number;
  duration: number;
  isPlaying: boolean;
}
```

**Key Components:**
1. **Demuxer**: Initialize with file buffer or stream
2. **Decoder Pool**: Reuse VideoDecoder instances for efficiency
3. **Frame Buffer**: Circular queue of decoded frames
4. **Audio Sync**: Track presentation timestamps (PTS)

### Phase 2: Streaming & Performance
- Fetch container in chunks (progressive demuxing)
- Lazy-load codec initialization (only decode requested time ranges)
- Cache decoded frame metadata for fast seeking
- Implement adaptive bitrate if multiple streams exist

### Phase 3: UI Integration
Replace current HTML5 video with:
- Canvas surface for video rendering
- Custom playback controls (pause, play, seek, volume)
- Format detection & codec support warning
- Fallback to HTML5 video if WebCodecs unavailable

## Supported Video Formats (Comprehensive)

| Container | Video Codecs            | Audio Codecs        | Browser Support |
|-----------|------------------------|---------------------|-----------------|
| MP4       | H.264, H.265, VP9, AV1 | AAC, MP3, Opus      | ✅ Excellent    |
| WebM      | VP8, VP9, AV1          | Opus, Vorbis        | ✅ Very Good    |
| MKV       | All above              | All above           | ✅ With demuxer |
| MOV       | H.264, H.265           | AAC                 | ✅ With demuxer |
| AVI       | MPEG-4, Xvid           | MP3, PCM            | ⚠️  Limited     |
| TS        | H.264, H.265           | AAC, MP3            | ⚠️  Limited     |
| FLV       | H.264, VP6             | MP3, AAC            | ⚠️  Limited     |

## Codec Support Matrix

### Guaranteed (All Modern Browsers)
- ✅ H.264 / AVC
- ✅ VP8, VP9
- ✅ Opus, AAC

### Excellent (95%+ Coverage)
- ✅ HEVC / H.265 (Chrome/Edge 108+, Safari 13+)
- ✅ AV1 (Chrome/Edge 74+, Firefox 67+)
- ✅ Vorbis

### Situational
- ⚠️ MPEG-4 (requires safari/webkit)
- ⚠️ Xvid (via software fallback)

## MediaCapabilities Probe

Before playback, query codec support:
```typescript
async function checkCodecSupport(config: VideoDecoderConfig): Promise<boolean> {
  if (!navigator.mediaCapabilities?.decodingInfo) return false;
  
  const info = await navigator.mediaCapabilities.decodingInfo({
    type: 'file',
    video: {
      contentType: config.codec,
      width: config.width,
      height: config.height,
      bitrate: config.bitrate,
      framerate: config.framerate,
    },
  });
  
  return info.supported;
}
```

## Performance Considerations

### Memory
- Decoded frame buffer: ~3-5 frames (width × height × 4 bytes each)
- For 4K @ 60fps: ~50-80 MB (acceptable)
- Older files: proportionally less

### Decoding Speed
- Hardware acceleration: negligible overhead
- Software decoding: depends on codec and CPU
- Rule of thumb: 1 frame decode = 1-3ms (modern CPU)

### Seeking
- Fast: Cache last 10-15 keyframe positions
- Rebuild cache on first seek (one-time ~100ms for most files)

## Browser Requirements

**Minimum:**
- WebCodecs API (Chrome 94+, Edge 94+, Firefox 121+, Safari 17.2+)
- MediaCapabilities API (same as above)

**Fallback:**
- HTML5 video for browsers without WebCodecs
- Show explicit "format not supported" for unsupported codecs

## Error Handling Strategy

```
1. Try demuxing with WebDemuxer
   ├─ Success → detect codec requirements
   ├─ Unsupported containers → show error
   └─ Corrupted → attempt repair or fail gracefully

2. Check codec support via MediaCapabilities
   ├─ Supported → decode
   ├─ Unsupported on this browser → offer fallback or explain
   └─ Encrypted → show HDCP message

3. Decode attempt
   ├─ Success → render
   ├─ Codec mismatch → auto-retry with detected codec
   └─ Decode error → show detailed error message
```

## Integration with Current Viewer

### Changes to `viewer-surface.component.ts`
1. Add canvas element alongside existing video tag
2. Detect file format → decide: Canvas Player vs HTML5 video
3. For Canvas Player:
   - Fetch file chunks via existing streaming API
   - Initialize VideoPlayer with ArrayBuffer
   - Render to canvas on RAF
4. Fallback to HTML5 video for standard formats

### Minimal Code Example
```typescript
// New service
@Injectable({ providedIn: 'root' })
export class CanvasVideoPlayerService {
  async createPlayer(
    sourceUrl: string,
    container: HTMLElement
  ): Promise<VideoPlayer> {
    const response = await fetch(sourceUrl);
    const buffer = await response.arrayBuffer();
    
    const player = new CanvasVideoPlayer(buffer, container);
    await player.initialize();
    return player;
  }
}

// In viewer component
if (shouldUseCanvasPlayer(ext)) {
  const player = await this.videoService.createPlayer(url, canvasEl);
  player.play();
} else {
  // Existing HTML5 video logic
}
```

## Migration Plan

**Phase 1**: Add Canvas player for MKV, MXF, AVI (currently problematic)
**Phase 2**: Expand to all supported formats
**Phase 3**: Make Canvas player default, HTML5 as fallback
**Phase 4**: Add advanced features (filters, HDR, subtitle rendering)

## Limitations & Tradeoffs

| Aspect            | Canvas Player                    | HTML5 Video |
|------------------|----------------------------------|-------------|
| Format Support   | ~95% of modern containers        | Limited     |
| Performance      | Software decode overhead        | Optimized   |
| Seeking          | Cache required                  | Native      |
| Controls         | Must implement custom           | Built-in    |
| Memory           | Higher (frame buffer)           | Lower       |
| Dev Time         | Higher                          | Lower       |
| Browser Compat   | Modern only (94+)               | Older       |

## Recommended Libraries

| Library            | Purpose                | Notes                              |
|--------------------|----------------------|-----------------------------------|
| `@vscode/webdemuxer` | MP4, WebM, MKV parsing | Used by VSCode for preview         |
| `ffmpeg.js` / `ffmpeg.wasm` | Full codec suite (optional) | Heavier but maximum format support |
| Native WebCodecs   | Decoding              | Standard, no library needed        |

## Next Steps

1. ✅ Prototype with a single format (MKV recommended)
2. ✅ Measure performance vs HTML5 video
3. ✅ Test codec detection & fallback flow
4. ✅ Implement error handling & user feedback
5. ✅ Gradual rollout to support all formats
