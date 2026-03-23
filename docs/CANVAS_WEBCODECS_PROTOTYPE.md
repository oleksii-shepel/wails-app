# Canvas + WebCodecs Prototype

## Minimal Working Example

This shows the core pattern for Canvas-based video playback using WebCodecs.

### 1. Core API (Pure TypeScript)

```typescript
// src/app/services/canvas-video-player.ts

import { BehaviorSubject, Subject } from 'rxjs';

export interface VideoMetadata {
  width: number;
  height: number;
  duration: number;
  format: string;
  videoCodec?: string;
  audioCodec?: string;
}

export interface VideoFrame {
  timestamp: number;
  data: VideoFrame;
}

export class CanvasVideoPlayer {
  private videoDecoder?: VideoDecoder;
  private audioDecoder?: AudioDecoder;
  private canvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;
  
  private frameBuffer: Map<number, CanvasImageSource> = new Map();
  private isPlaying = false;
  private currentTime = 0;
  private duration = 0;
  
  readonly stateChanged$ = new BehaviorSubject<'idle' | 'playing' | 'paused' | 'ended'>('idle');
  readonly frameReady$ = new Subject<number>();
  readonly error$ = new Subject<string>();
  
  constructor(private container: HTMLElement) {
    this.canvas = document.createElement('canvas');
    this.ctx = this.canvas.getContext('2d')!;
    this.container.appendChild(this.canvas);
  }

  // ──────────────────────────────────────────────────────────────
  // Initialization
  // ──────────────────────────────────────────────────────────────
  
  async loadFromUrl(url: string): Promise<VideoMetadata> {
    try {
      const response = await fetch(url, { headers: { 'Range': 'bytes=0-10485760' } });
      const buffer = await response.arrayBuffer();
      return await this.loadFromBuffer(buffer);
    } catch (err) {
      const msg = `Failed to load video: ${err}`;
      this.error$.next(msg);
      throw err;
    }
  }

  async loadFromBuffer(buffer: ArrayBuffer): Promise<VideoMetadata> {
    try {
      // Step 1: Demux container (you'd use @vscode/webdemuxer here)
      const metadata = this.demuxContainer(buffer);
      
      // Step 2: Check if codec is supported
      const supported = await this.checkCodecSupport(metadata.videoCodec!);
      if (!supported) {
        throw new Error(`Codec not supported: ${metadata.videoCodec}`);
      }
      
      // Step 3: Initialize decoders
      await this.initializeDecoders(metadata);
      
      this.duration = metadata.duration;
      this.canvas.width = metadata.width;
      this.canvas.height = metadata.height;
      
      return metadata;
    } catch (err: any) {
      const msg = `Demuxing failed: ${err.message}`;
      this.error$.next(msg);
      throw err;
    }
  }

  // ──────────────────────────────────────────────────────────────
  // Playback Control
  // ──────────────────────────────────────────────────────────────

  async play(): Promise<void> {
    this.isPlaying = true;
    this.stateChanged$.next('playing');
    this.renderLoop();
  }

  pause(): void {
    this.isPlaying = false;
    this.stateChanged$.next('paused');
  }

  async seek(timeMs: number): Promise<void> {
    this.currentTime = Math.max(0, Math.min(timeMs, this.duration));
    
    // Reset decoders and seek to keyframe
    await this.flushDecoders();
    await this.seekToKeyframe(this.currentTime);
    
    // Decode up to current time
    await this.decodeToTime(this.currentTime);
  }

  // ──────────────────────────────────────────────────────────────
  // Internal: Rendering
  // ──────────────────────────────────────────────────────────────

  private renderLoop(): void {
    if (!this.isPlaying) return;
    
    const frame = this.frameBuffer.get(Math.floor(this.currentTime));
    if (frame) {
      this.ctx.drawImage(frame as CanvasImageSource, 0, 0);
    }

    this.currentTime += 33; // ~30fps (1000/30)
    
    if (this.currentTime >= this.duration) {
      this.isPlaying = false;
      this.currentTime = this.duration;
      this.stateChanged$.next('ended');
      return;
    }

    requestAnimationFrame(() => this.renderLoop());
  }

  // ──────────────────────────────────────────────────────────────
  // Internal: Codec & Decoding
  // ──────────────────────────────────────────────────────────────

  private async checkCodecSupport(codecString: string): Promise<boolean> {
    if (!navigator.mediaCapabilities?.decodingInfo) {
      console.warn('MediaCapabilities API not available');
      return false;
    }

    try {
      const info = await navigator.mediaCapabilities.decodingInfo({
        type: 'file',
        video: {
          contentType: codecString,
          width: 1920,
          height: 1080,
          bitrate: 5e6,
          framerate: 30,
        },
      });

      return info.supported;
    } catch {
      return false;
    }
  }

  private async initializeDecoders(metadata: VideoMetadata): Promise<void> {
    if (!VideoDecoder.isConfigSupported) {
      throw new Error('VideoDecoder API not available');
    }

    // Video decoder config
    const videoConfig: VideoDecoderConfig = {
      codec: metadata.videoCodec || 'avc1.42001e',
      codedWidth: metadata.width,
      codedHeight: metadata.height,
      description: undefined, // Would be populated from demuxer
    };

    const supported = await VideoDecoder.isConfigSupported(videoConfig);
    if (!supported.supported) {
      throw new Error(`Video codec config not supported: ${metadata.videoCodec}`);
    }

    this.videoDecoder = new VideoDecoder({
      output: (frame: VideoFrame) => {
        // Buffer decoded frames
        const canvas = new OffscreenCanvas(frame.codedWidth, frame.codedHeight);
        const ctx = canvas.getContext('2d')!;
        ctx.drawImage(frame as any, 0, 0);
        
        this.frameBuffer.set(frame.timestamp, canvas);
        this.frameReady$.next(frame.timestamp);
        
        frame.close(); // Release GPU memory
      },
      error: (err: DOMException) => {
        console.error('VideoDecoder error:', err);
        this.error$.next(`Decode error: ${err.message}`);
      },
    });

    await this.videoDecoder.configure(videoConfig);
  }

  private async getOrCreateChunk(offset: number, size: number): Promise<EncodedVideoChunk> {
    // In real implementation: fetch chunk from server using Range header
    // For now: stub that would be populated from loaded buffer
    throw new Error('Not implemented in prototype');
  }

  private async decodeToTime(timeMs: number): Promise<void> {
    // This would iterate through chunks and decode until currentTime
    // Placeholder for actual chunk fetching + decoding loop
    if (!this.videoDecoder) return;
    
    // In reality: fetch frames from disk/memory up to timeMs
    // and decode them into frameBuffer
  }

  private async flushDecoders(): Promise<void> {
    this.frameBuffer.clear();
    if (this.videoDecoder) {
      await this.videoDecoder.flush();
    }
  }

  private async seekToKeyframe(timeMs: number): Promise<void> {
    // Query keyframe index to find nearest keyframe before timeMs
    // This requires building an index during initialization
    console.log(`Seeking to keyframe near ${timeMs}ms`);
  }

  // ──────────────────────────────────────────────────────────────
  // Internal: Container Parsing (Stub)
  // ──────────────────────────────────────────────────────────────

  private demuxContainer(buffer: ArrayBuffer): VideoMetadata {
    // TODO: Use @vscode/webdemuxer or tsdemux library
    // For now, stub showing what would be returned:
    
    return {
      width: 1920,
      height: 1080,
      duration: 120000,
      format: 'matroska',
      videoCodec: 'video/avc1.42001e',
      audioCodec: 'audio/opus',
    };
  }
}
```

---

### 2. Angular Component Integration

```typescript
// src/app/components/canvas-video-viewer.component.ts

import { Component, Input, OnInit, OnDestroy, ViewChild, ElementRef } from '@angular/core';
import { CanvasVideoPlayer } from '../services/canvas-video-player';

@Component({
  selector: 'app-canvas-video-viewer',
  standalone: true,
  template: `
    <div class="video-player-container">
      <div #playerContainer class="player-canvas"></div>
      
      <div class="player-controls">
        <button (click)="togglePlay()" [disabled]="!canPlay">
          {{ isPlaying ? 'Pause' : 'Play' }}
        </button>
        
        <div class="progress-bar">
          <input 
            type="range" 
            min="0" 
            [max]="duration"
            [value]="currentTime"
            (change)="onSeek($event)"
            class="seek-slider">
          <span class="time-display">
            {{ formatTime(currentTime) }} / {{ formatTime(duration) }}
          </span>
        </div>

        <button (click)="toggleFullscreen()">Fullscreen</button>
      </div>

      <div class="error-message" *ngIf="error">
        {{ error }}
      </div>
    </div>
  `,
  styles: [`
    .video-player-container {
      display: flex;
      flex-direction: column;
      max-width: 100%;
      height: 100%;
      background: #000;
    }

    .player-canvas {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 300px;
      background: #1a1a1a;

      ::ng-deep canvas {
        max-width: 100%;
        max-height: 100%;
        object-fit: contain;
      }
    }

    .player-controls {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 12px;
      background: #333;
    }

    .progress-bar {
      flex: 1;
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .seek-slider {
      flex: 1;
      cursor: pointer;
    }

    .time-display {
      font-size: 12px;
      color: #aaa;
      font-family: monospace;
      min-width: 90px;
    }

    button {
      padding: 6px 12px;
      background: #0078d4;
      color: white;
      border: none;
      border-radius: 3px;
      cursor: pointer;
    }

    button:hover:not(:disabled) {
      background: #1084d7;
    }

    button:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .error-message {
      padding: 12px;
      color: #ff7070;
      background: rgba(255, 0, 0, 0.1);
      border-top: 1px solid rgba(255, 112, 112, 0.2);
      font-size: 13px;
    }
  `],
})
export class CanvasVideoViewerComponent implements OnInit, OnDestroy {
  @Input() videoUrl!: string;
  @ViewChild('playerContainer') playerContainer!: ElementRef<HTMLDivElement>;

  private player?: CanvasVideoPlayer;
  
  isPlaying = false;
  currentTime = 0;
  duration = 0;
  canPlay = false;
  error: string | null = null;

  async ngOnInit(): Promise<void> {
    if (!this.playerContainer) return;

    this.player = new CanvasVideoPlayer(this.playerContainer.nativeElement);

    try {
      const metadata = await this.player.loadFromUrl(this.videoUrl);
      this.duration = metadata.duration;
      this.canPlay = true;

      // Subscribe to state changes
      this.player.stateChanged$.subscribe((state) => {
        this.isPlaying = state === 'playing';
      });

      this.player.error$.subscribe((err) => {
        this.error = err;
        this.canPlay = false;
      });
    } catch (err: any) {
      this.error = err.message;
      this.canPlay = false;
    }
  }

  ngOnDestroy(): void {
    this.player?.pause();
  }

  async togglePlay(): Promise<void> {
    if (!this.player) return;
    
    if (this.isPlaying) {
      this.player.pause();
    } else {
      await this.player.play();
    }
  }

  async onSeek(event: Event): Promise<void> {
    const target = event.target as HTMLInputElement;
    const timeMs = parseFloat(target.value);
    
    if (this.player) {
      await this.player.seek(timeMs);
      this.currentTime = timeMs;
    }
  }

  toggleFullscreen(): void {
    if (this.playerContainer?.nativeElement) {
      this.playerContainer.nativeElement.requestFullscreen?.();
    }
  }

  formatTime(ms: number): string {
    const seconds = Math.floor(ms / 1000);
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  }
}
```

---

### 3. Integration into `viewer-surface.component.ts`

```typescript
// Add to imports
import { CanvasVideoViewerComponent } from './canvas-video-viewer.component';

// In template, add alongside existing video tag:
<ng-container *ngIf="kind === 'video' && useCanvasPlayer">
  <app-canvas-video-viewer [videoUrl]="mediaSrc!"></app-canvas-video-viewer>
</ng-container>

<ng-container *ngIf="kind === 'video' && !useCanvasPlayer">
  <!-- Existing HTML5 video tag -->
  <video class="preview-video" controls preload="auto" playsinline autoplay
         [src]="mediaSrc"
         (loadedmetadata)="onMediaMetadata('video', $event)"
         (canplay)="onMediaCanPlay('video', $event)"
         (loadeddata)="onVideoLoadedData($event)"
         (error)="onMediaError('video', $event)">
    Your browser does not support video.
  </video>
</ng-container>

// In component class:
async renderContent(...): Promise<void> {
  // After determining videoMime...
  
  const videoMime = this.inferVideoMime(ext);
  if (videoMime) {
    // Decide: Canvas or HTML5?
    this.useCanvasPlayer = await this.shouldUseCanvasPlayer(ext, info);
    
    try {
      this.mediaSrc = await readers.streamBinary(videoMime);
      this.mediaMime = videoMime;
      this.kind = 'video';
      return;
    } catch { /** ... */ }
  }
}

private async shouldUseCanvasPlayer(ext: string, info: FileInfo): Promise<boolean> {
  // Canvas for formats that HTML5 video typically doesn't handle well
  const canvasFormats = new Set(['mkv', 'mxf', 'avi', 'flv', 'ts']);
  
  if (!canvasFormats.has(ext)) {
    return false; // Use HTML5 for mp4, webm, mov
  }

  // If browser doesn't support WebCodecs, fall back to HTML5
  return 'VideoDecoder' in window && 'mediaCapabilities' in navigator;
}
```

---

### 4. Package.json Dependencies

```json
{
  "dependencies": {
    "@vscode/webdemuxer": "^0.1.0",
    "rxjs": "^7.8.0"
  },
  "devDependencies": {
    "@types/web-codecs": "^0.1.0"
  }
}
```

---

## Key Differences from HTML5 Video

| Aspect            | HTML5 Video | Canvas Player |
|-------------------|------------|--------------|
| Initialization    | Instant    | ~100ms to demux & init decoder |
| Seeking           | Instant *  | Decode to keyframe (~50-200ms) |
| Pause/Resume      | Instant    | Flush/reinit decoders (~10ms) |
| Format Support    | 3-4 formats | 95%+ of modern formats |
| Browser Support   | All        | 94+ (Chrome, Edge, Firefox, Safari) |
| Codec Flexibility | None       | Hardware accelerated when available |

*HTML5 video seeking is instant from browser cache; Canvas player requires decoding to keyframe

---

## Testing Checklist for Prototype

- [ ] Load MKV file and demux successfully
- [ ] Detect H.264 codec in MKV
- [ ] Check MediaCapabilities for codec support
- [ ] Initialize VideoDecoder
- [ ] Fetch first chunk from server
- [ ] Decode chunk to frame
- [ ] Draw frame to canvas
- [ ] Implement play button (start renderLoop)
- [ ] Implement pause button (stop renderLoop)
- [ ] Implement basic seek (flush + decode to time)
- [ ] Verify audio sync (if audio present)
- [ ] Test with 3+ different MKV files
- [ ] Fallback when WebCodecs unavailable

---

## Performance Profiling

For benchmarking, add perf marks:

```typescript
performance.mark('decode-start');
await this.videoDecoder.decode(chunk);
performance.mark('decode-end');
performance.measure('decode-time', 'decode-start', 'decode-end');

console.log(performance.getEntriesByName('decode-time'));
```

Target: <20ms per frame (50fps capable)

---

## Next Steps

1. Copy `CanvasVideoPlayer` class and start prototyping
2. Install @vscode/webdemuxer and integrate demuxer
3. Test with 1 MKV file end-to-end
4. Measure performance
5. Integrate with viewer component
6. Expand to more formats
