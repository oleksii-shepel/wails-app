import { CommonModule } from '@angular/common';
import {
  AfterViewInit,
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  Input,
  OnChanges,
  OnDestroy,
  ViewChild,
} from '@angular/core';
import { ApiService } from '../../services/api.service';
import {
  CanvasVideoPlayer,
  CanvasVideoPlayerService,
  VideoMetadata,
} from '../../services/canvas-video-player.service';
import { ViewerSurfaceComponent } from '../../surfaces/viewer/viewer-surface.component';
import { ViewerArchiveContext } from '../viewer-archive-context';
import { SegmentVideoLoader } from './segment-video-loader';

@Component({
  selector: 'app-video-viewer',
  standalone: true,
  imports: [CommonModule, ViewerSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-viewer-surface
      *ngIf="useLegacySurface"
      [targetPath]="targetPath"
      [archiveContext]="archiveContext"
      [viewerId]="viewerId">
    </app-viewer-surface>

    <div *ngIf="!useLegacySurface" class="viewer-surface">
      <div class="viewer-header">
        <div class="header-main">
          <span class="file-icon">🎬</span>
          <div class="header-text">
            <div class="title" [title]="displayName">{{ displayName || 'Preview' }}</div>
            <div class="path" [title]="targetPath">{{ targetPath || 'Select a file to preview' }}</div>
          </div>
        </div>
        <div class="header-meta">
          <span class="badge type-badge">{{ typeBadge }}</span>
          <span class="badge size-badge" *ngIf="fileSize > 0">{{ formatSize(fileSize) }}</span>
          <span class="badge seek-badge">{{ playbackMode }}</span>
          <span class="badge codec-badge" *ngIf="videoCodecInfo">{{ videoCodecInfo }}</span>
          <span class="badge resolution-badge" *ngIf="videoResolution">{{ videoResolution }}</span>
        </div>
      </div>

      <div class="viewer-body">
        <div *ngIf="loading" class="state-container">
          <div class="spinner"></div>
          <div class="state-label">Loading segmented preview...</div>
        </div>

        <div *ngIf="error" class="state-container state-error">
          <div class="state-icon">⚠</div>
          <div class="state-label">Preview failed</div>
          <div class="error-detail">{{ error }}</div>
        </div>

        <div *ngIf="!loading && !error" class="video-player-wrapper">
          <div
            #videoContainer
            class="canvas-video-container"
            [class.playing]="videoIsPlaying"
            (mousemove)="onVideoMouseMove()"
            (mouseleave)="onVideoMouseLeave()">
            <div class="video-loading" *ngIf="videoLoading">
              <div class="spinner"></div>
              <div class="loading-text">Loading video...</div>
            </div>
            <div class="play-overlay" *ngIf="!videoIsPlaying && !videoLoading && videoReady" (click)="videoPlayPause()">
              <div class="play-icon">▶</div>
            </div>
          </div>

          <div class="video-control-bar" [class.show]="videoShowControls || !videoIsPlaying">
            <div class="progress-section">
              <div class="progress-container">
                <input
                  type="range"
                  min="0"
                  [max]="videoDuration"
                  step="any"
                  [value]="displayedCurrentTime"
                  [disabled]="!videoReady"
                  class="seek-slider"
                  (mousedown)="onVideoSeekStart()"
                  (touchstart)="onVideoSeekStart()"
                  (input)="onVideoSeekInput($event)"
                  (change)="onVideoSeekCommit($event)">
                <div
                  class="progress-fill"
                  [style.width.%]="(displayedCurrentTime / videoDuration) * 100 || 0"></div>
              </div>
            </div>

            <div class="controls-row">
              <button class="control-btn play-pause-btn" [disabled]="!videoReady" (click)="videoPlayPause()">
                <span class="icon">{{ videoIsPlaying ? '⏸' : '▶' }}</span>
              </button>

              <div class="time-display">
                <span class="current-time">{{ formatTime(displayedCurrentTime) }}</span>
                <span class="separator">/</span>
                <span class="duration">{{ formatTime(videoDuration) }}</span>
              </div>

              <div class="spacer"></div>

              <div class="volume-control">
                <span class="volume-icon">🔊</span>
                <input
                  type="range"
                  min="0"
                  max="100"
                  [value]="videoVolume"
                  [disabled]="!videoReady || !canvasPlayer"
                  class="volume-slider"
                  (input)="onVideoVolumeChange($event)">
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    .viewer-surface {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-editor-background, #1e1e1e);
      color: var(--vsc-foreground, #cccccc);
    }

    .viewer-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      padding: 8px 12px;
      border-bottom: 1px solid var(--vsc-border, #454545);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 92%, black);
      flex-shrink: 0;
      min-width: 0;
    }

    .header-main {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
      flex: 1;
    }

    .file-icon {
      font-size: 18px;
      flex-shrink: 0;
      line-height: 1;
    }

    .header-text {
      min-width: 0;
      flex: 1;
    }

    .title {
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 0.3px;
      text-transform: uppercase;
      opacity: 0.9;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .path {
      font-family: var(--vsc-font-family-mono, ui-monospace, 'Courier New', monospace);
      font-size: 11px;
      opacity: 0.6;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      margin-top: 1px;
    }

    .header-meta {
      display: flex;
      align-items: center;
      gap: 4px;
      flex-shrink: 0;
    }

    .badge {
      font-size: 10px;
      font-weight: 600;
      padding: 2px 6px;
      border-radius: 3px;
      white-space: nowrap;
      letter-spacing: 0.3px;
    }

    .type-badge {
      background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 20%, transparent);
      color: var(--vsc-accent-blue, #4da8ff);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 40%, transparent);
    }

    .size-badge {
      background: color-mix(in srgb, var(--vsc-foreground, #ccc) 8%, transparent);
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 60%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 50%, transparent);
    }

    .seek-badge,
    .codec-badge,
    .resolution-badge {
      display: inline-block;
      padding: 2px 6px;
      border-radius: 3px;
      font-weight: 500;
      white-space: nowrap;
    }

    .seek-badge {
      background: rgba(250, 204, 21, 0.18);
      border: 1px solid rgba(250, 204, 21, 0.35);
      color: #fde68a;
    }

    .codec-badge {
      background: rgba(0, 120, 212, 0.2);
      border: 1px solid rgba(0, 120, 212, 0.4);
      color: #4da8ff;
    }

    .resolution-badge {
      background: rgba(0, 150, 100, 0.2);
      border: 1px solid rgba(0, 150, 100, 0.4);
      color: #4fcc9f;
    }

    .viewer-body {
      flex: 1 1 0%;
      min-height: 0;
      overflow: auto;
      position: relative;
    }

    .state-container {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 100%;
      min-height: 160px;
      gap: 8px;
      padding: 24px;
      text-align: center;
    }

    .state-icon {
      font-size: 36px;
      line-height: 1;
      opacity: 0.55;
    }

    .state-label {
      font-size: 13px;
      opacity: 0.65;
    }

    .state-error .state-icon {
      opacity: 1;
    }

    .state-error .state-label {
      color: #ff7070;
      opacity: 1;
    }

    .error-detail {
      font-size: 11px;
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      color: color-mix(in srgb, #ff7070 70%, var(--vsc-foreground, #ccc));
      background: rgba(255, 0, 0, 0.07);
      border: 1px solid rgba(255, 112, 112, 0.2);
      border-radius: 4px;
      padding: 8px 12px;
      max-width: 480px;
      white-space: pre-wrap;
      word-break: break-word;
    }

    .spinner {
      width: 26px;
      height: 26px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 25%, transparent);
      border-top-color: var(--vsc-accent-blue, #0078d4);
      border-radius: 50%;
      animation: spin 0.65s linear infinite;
    }

    @keyframes spin {
      to {
        transform: rotate(360deg);
      }
    }

    .video-player-wrapper {
      display: flex;
      flex-direction: column;
      width: 100%;
      height: 100%;
      background: #000;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
    }

    .canvas-video-container {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 300px;
      background: linear-gradient(135deg, #0a0a0a 0%, #1a1a1a 100%);
      position: relative;
      overflow: hidden;
      cursor: pointer;
    }

    .canvas-video-container:hover {
      cursor: url('data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="none" stroke="white" stroke-width="1.5"/><polygon points="10,7 16,12 10,17" fill="white"/></svg>') 12 12, pointer;
    }

    .canvas-video-container.playing:hover {
      cursor: url('data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="none" stroke="white" stroke-width="1.5"/><rect x="9" y="8" width="2" height="8" fill="white"/><rect x="13" y="8" width="2" height="8" fill="white"/></svg>') 12 12, pointer;
    }

    .canvas-video-container ::ng-deep canvas {
      width: 100%;
      height: 100%;
      object-fit: contain;
      display: block;
    }

    .video-loading {
      position: absolute;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 16px;
      z-index: 100;
    }

    .video-loading .spinner {
      width: 40px;
      height: 40px;
      border: 3px solid rgba(255, 255, 255, 0.2);
      border-top-color: var(--vsc-accent-blue, #0078d4);
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
    }

    .loading-text {
      color: #aaa;
      font-size: 13px;
      letter-spacing: 0.3px;
    }

    .play-overlay {
      position: absolute;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 100%;
      height: 100%;
      background: rgba(0, 0, 0, 0.3);
      backdrop-filter: blur(2px);
      cursor: pointer;
      z-index: 50;
      transition: background 0.2s ease;
    }

    .play-overlay:hover {
      background: rgba(0, 0, 0, 0.5);
    }

    .play-icon {
      width: 80px;
      height: 80px;
      display: flex;
      align-items: center;
      justify-content: center;
      background: var(--vsc-accent-blue, #0078d4);
      border-radius: 50%;
      font-size: 32px;
      color: white;
      box-shadow: 0 8px 32px rgba(0, 120, 212, 0.4);
      transition: all 0.2s ease;
      font-weight: bold;
    }

    .play-overlay:hover .play-icon {
      background: #1084d7;
      transform: scale(1.1);
      box-shadow: 0 12px 48px rgba(0, 120, 212, 0.6);
    }

    .video-control-bar {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 12px;
      background: linear-gradient(to bottom, rgba(0,0,0,0.01), rgba(0,0,0,0.9));
      border-top: 1px solid rgba(255, 255, 255, 0.1);
      backdrop-filter: blur(10px);
      opacity: 0;
      transition: opacity 0.3s ease;
      pointer-events: none;
    }

    .video-control-bar.show {
      opacity: 1;
      pointer-events: auto;
    }

    .canvas-video-container:hover ~ .video-control-bar,
    .video-control-bar:hover {
      opacity: 1;
      pointer-events: auto;
    }

    .progress-section {
      display: flex;
      align-items: center;
      height: 24px;
      gap: 8px;
    }

    .progress-container {
      flex: 1;
      position: relative;
      height: 4px;
      background: rgba(255, 255, 255, 0.2);
      border-radius: 2px;
      cursor: pointer;
      overflow: hidden;
    }

    .progress-fill {
      position: absolute;
      height: 100%;
      background: linear-gradient(90deg, var(--vsc-accent-blue, #0078d4), #00a4ef);
      border-radius: 2px;
      pointer-events: none;
      transition: width 0.05s linear;
    }

    .seek-slider {
      position: absolute;
      width: 100%;
      height: 24px;
      cursor: pointer;
      appearance: none;
      -webkit-appearance: none;
      background: transparent;
      outline: none;
      margin: -10px 0;
      z-index: 10;
    }

    .seek-slider::-webkit-slider-thumb {
      appearance: none;
      -webkit-appearance: none;
      width: 12px;
      height: 12px;
      background: white;
      border: none;
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.5);
      transition: width 0.05s ease, height 0.05s ease, box-shadow 0.05s ease;
    }

    .seek-slider::-webkit-slider-thumb:hover {
      width: 16px;
      height: 16px;
      box-shadow: 0 4px 16px rgba(0, 120, 212, 0.8);
    }

    .seek-slider::-moz-range-thumb {
      width: 12px;
      height: 12px;
      background: white;
      border: none;
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.5);
      transition: width 0.05s ease, height 0.05s ease, box-shadow 0.05s ease;
    }

    .seek-slider::-moz-range-thumb:hover {
      width: 16px;
      height: 16px;
      box-shadow: 0 4px 16px rgba(0, 120, 212, 0.8);
    }

    .seek-slider:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .controls-row {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 0 4px;
    }

    .control-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 36px;
      height: 36px;
      padding: 0 8px;
      background: rgba(255, 255, 255, 0.1);
      border: 1px solid rgba(255, 255, 255, 0.15);
      color: white;
      border-radius: 4px;
      cursor: pointer;
      font-size: 16px;
      transition: all 0.2s ease;
      font-weight: 600;
      backdrop-filter: blur(4px);
    }

    .control-btn:hover:not(:disabled) {
      background: rgba(255, 255, 255, 0.2);
      border-color: rgba(255, 255, 255, 0.3);
      transform: scale(1.05);
    }

    .control-btn:active:not(:disabled) {
      transform: scale(0.95);
    }

    .control-btn:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    .control-btn .icon {
      display: inline-block;
    }

    .play-pause-btn {
      min-width: 44px;
      background: var(--vsc-accent-blue, #0078d4);
      border: 1px solid rgba(0, 120, 212, 0.3);
    }

    .play-pause-btn:hover:not(:disabled) {
      background: #1084d7;
      box-shadow: 0 0 12px rgba(0, 120, 212, 0.5);
    }

    .time-display {
      display: flex;
      align-items: center;
      gap: 4px;
      font-size: 12px;
      color: #ccc;
      font-variant-numeric: tabular-nums;
      font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
      padding: 4px 8px;
      background: rgba(0, 0, 0, 0.2);
      border-radius: 3px;
      min-width: 90px;
      justify-content: center;
    }

    .current-time {
      color: white;
      font-weight: 500;
    }

    .separator {
      opacity: 0.5;
    }

    .spacer {
      flex: 1;
    }

    .volume-control {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 0 4px;
    }

    .volume-icon {
      display: inline-block;
      font-size: 16px;
    }

    .volume-slider {
      width: 80px;
      height: 4px;
      cursor: pointer;
      appearance: none;
      -webkit-appearance: none;
      background: rgba(255, 255, 255, 0.2);
      border-radius: 2px;
      outline: none;
    }

    .volume-slider::-webkit-slider-thumb {
      appearance: none;
      -webkit-appearance: none;
      width: 10px;
      height: 10px;
      background: white;
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 2px 6px rgba(0, 0, 0, 0.5);
    }

    .volume-slider::-moz-range-thumb {
      width: 10px;
      height: 10px;
      background: white;
      border: none;
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 2px 6px rgba(0, 0, 0, 0.5);
    }

    .volume-slider:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }
  `],
})
export class VideoViewerComponent implements OnChanges, AfterViewInit, OnDestroy {
  private static readonly WINDOWED_VIDEO_STREAM_SECONDS = 90;

  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;

  @ViewChild('videoContainer')
  set videoContainerRef(value: ElementRef<HTMLDivElement> | undefined) {
    this._videoContainerRef = value;
    this.requestSegmentedPlaybackInitialization();
  }

  get videoContainerRef(): ElementRef<HTMLDivElement> | undefined {
    return this._videoContainerRef;
  }

  useLegacySurface = false;
  loading = false;
  error: string | null = null;
  canvasPlayer: CanvasVideoPlayer | null = null;
  videoReady = false;
  videoIsPlaying = false;
  videoLoading = false;
  videoShowControls = true;
  videoCurrentTime = 0;
  videoDuration = 0;
  videoCodecInfo = '';
  videoResolution = '';
  videoVolume = 100;
  videoIsSeeking = false;
  videoSeekTarget = 0;
  fileSize = 0;

  private pendingManifestUrl: string | null = null;
  private pendingStreamUrl: string | null = null;
  private _videoContainerRef?: ElementRef<HTMLDivElement>;
  private initializingManifestUrl: string | null = null;
  private seekGeneration = 0;
  private initializingStreamUrl: string | null = null;
  private currentStreamUrl: string | null = null;
  private loadingStreamUrl: string | null = null;
  private videoTimelineOffsetSeconds = 0;
  private videoControlsTimeout: ReturnType<typeof setTimeout> | null = null;
  private seekDragActive = false;
  private seekPending = false;
  private seekOriginSeconds = 0;
  private seekRequestGeneration = 0;
  playbackMode = '';

  constructor(
    private readonly api: ApiService,
    private readonly cdr: ChangeDetectorRef,
    private readonly videoPlayerService: CanvasVideoPlayerService,
  ) { }

  get displayName(): string {
    const normalized = (this.targetPath ?? '').replace(/\\/g, '/');
    return normalized.split('/').pop() ?? normalized;
  }

  get typeBadge(): string {
    const ext = (this.targetPath.split('.').pop() ?? 'video').toUpperCase();
    return ext || 'VIDEO';
  }

  get displayedCurrentTime(): number {
    return this.seekDragActive ? this.videoSeekTarget : this.videoCurrentTime;
  }

  ngAfterViewInit(): void {
    this.requestSegmentedPlaybackInitialization();
    this.requestUrlPlaybackInitialization();
  }

  async ngOnChanges(): Promise<void> {
    this.resetState();
    if (this.archiveContext) {
      this.useLegacySurface = true;
      this.cdr.markForCheck();
      return;
    }

    const path = (this.targetPath ?? '').trim();
    if (!path) {
      this.useLegacySurface = true;
      this.cdr.markForCheck();
      return;
    }

    this.loading = true;
    this.cdr.markForCheck();
    try {
      const info = await this.api.getFileInfo(path);
      this.fileSize = info.size ?? 0;
      const rawPlan = await this.api.getVideoPlaybackPlan(path);
      const mode = String(rawPlan?.mode ?? rawPlan?.Mode ?? '').toLowerCase();
      const url = String(rawPlan?.url ?? rawPlan?.URL ?? '');
      this.playbackMode = mode || 'video';
      if (!url) {
        this.useLegacySurface = true;
      } else if (mode === 'segmented') {
        this.pendingManifestUrl = url;
        this.requestSegmentedPlaybackInitialization();
      } else {
        // remux / transcode / direct — use canvas player with URL-based loading;
        // the canvas player handles range-based seeking when the server supports it,
        // and falls back to stream-reload seeking for piped sources.
        this.pendingStreamUrl = url;
        this.requestUrlPlaybackInitialization();
      }
    } catch (err: unknown) {
      this.error = err instanceof Error ? err.message : String(err);
    } finally {
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  ngOnDestroy(): void {
    if (this.canvasPlayer) {
      this.canvasPlayer.destroy();
      this.canvasPlayer = null;
    }
    if (this.videoControlsTimeout) {
      clearTimeout(this.videoControlsTimeout);
      this.videoControlsTimeout = null;
    }
  }

  async videoPlayPause(): Promise<void> {
    if (!this.canvasPlayer) {
      return;
    }

    try {
      if (this.videoIsPlaying) {
        this.canvasPlayer.pause();
      } else {
        this.videoLoading = true;
        this.cdr.markForCheck();
        await this.canvasPlayer.play();
        this.videoLoading = false;
      }
      this.cdr.markForCheck();
    } catch (err: unknown) {
      this.error = err instanceof Error ? err.message : 'Playback error';
      this.videoLoading = false;
      this.cdr.markForCheck();
    }
  }

  onVideoSeekStart(): void {
    if (!this.videoReady) {
      return;
    }
    this.seekDragActive = true;
    if (!this.videoLoading) {
      this.seekOriginSeconds = this.videoCurrentTime;
    }
    this.updateVideoSeekingState();
    this.videoSeekTarget = this.videoCurrentTime;
    this.cdr.markForCheck();
  }

  onVideoSeekInput(event: Event): void {
    if (!this.videoReady) {
      return;
    }
    const seekSeconds = parseFloat((event.target as HTMLInputElement).value);
    this.videoSeekTarget = seekSeconds;
    this.videoCurrentTime = seekSeconds;
    // Keep the thumb and time display responsive while dragging, but defer the
    // actual decoder seek until release so a long scrub does not queue many
    // expensive re-seeks.
    this.updateVideoSeekingState();
    this.cdr.markForCheck();
  }

  onVideoSeekCommit(event: Event): void {
    if (!this.videoReady) {
      return;
    }

    const seekSeconds = parseFloat((event.target as HTMLInputElement).value);
    this.seekDragActive = false;
    this.videoSeekTarget = seekSeconds;
    this.videoCurrentTime = seekSeconds;

    // For backward seeks, we want immediate visual feedback
    const isBackwardSeek = seekSeconds < this.videoCurrentTime;
    if (isBackwardSeek && this.canvasPlayer) {
      // Try to show buffered frame immediately for backward seek
      const currentFrame = this.canvasPlayer['findBufferedFrameForTime']?.(seekSeconds * 1000);
      if (currentFrame) {
        // Visual feedback already provided by displayedCurrentTime
      }
    }

    void this.performSeek(seekSeconds);
    this.cdr.markForCheck();
  }

  private async performSeek(seekSeconds: number): Promise<void> {
    if (!this.canvasPlayer || !this.videoReady) {
      return;
    }

    const generation = ++this.seekRequestGeneration;
    const shouldResumePlayback = this.canvasPlayer.isPlaying() || this.videoIsPlaying;
    const currentTimeMs = this.videoCurrentTime * 1000;
    const targetTimeMs = seekSeconds * 1000;
    const isSeekingBackward = targetTimeMs < currentTimeMs;

    // For segmented video, we don't need to restart stream for backward seeks
    const needsRestart = this.shouldRestartStreamOnSeek(seekSeconds) && !isSeekingBackward;

    this.seekPending = true;
    this.videoLoading = needsRestart;
    this.updateVideoSeekingState();
    this.cdr.markForCheck();

    try {
      if (needsRestart) {
        await this.restartVideoAt(seekSeconds, { autoplay: shouldResumePlayback, generation });
      } else {
        const playerSeekSeconds = this.shouldUseWindowedVideoStreaming()
          ? Math.max(0, seekSeconds - this.videoTimelineOffsetSeconds)
          : seekSeconds;
        await this.canvasPlayer.seek(playerSeekSeconds * 1000);
      }
    } catch (err: unknown) {
      if (generation === this.seekRequestGeneration) {
        this.error = err instanceof Error ? err.message : 'Seek failed';
      }
    } finally {
      if (generation === this.seekRequestGeneration) {
        this.seekPending = false;
        if (!needsRestart) {
          this.videoLoading = false;
        }
        this.updateVideoSeekingState();
        this.cdr.markForCheck();
      }
    }
  }

  private updateVideoSeekingState(): void {
    this.videoIsSeeking = this.seekDragActive || this.seekPending || this.videoLoading;
  }

  private shouldUseWindowedVideoStreaming(): boolean {
    return this.playbackMode === 'transcode';
  }

  private shouldRestartStreamOnSeek(seekSeconds: number): boolean {
    if (!this.shouldUseWindowedVideoStreaming()) {
      return false;
    }

    const windowStart = this.videoTimelineOffsetSeconds;
    const windowEnd = windowStart + VideoViewerComponent.WINDOWED_VIDEO_STREAM_SECONDS;
    const isSeekingBackward = seekSeconds < this.videoCurrentTime;

    // For backward seeks, we can often reuse the existing stream
    // Only restart if seeking far backward beyond a threshold
    if (isSeekingBackward) {
      const seekDistance = this.videoCurrentTime - seekSeconds;
      // If seeking backward within the last 30 seconds, don't restart
      if (seekDistance <= 30) {
        return false;
      }
    }

    return seekSeconds < windowStart || seekSeconds >= windowEnd;
  }

  private buildSeekUrl(startSeconds: number): string | null {
    const sourceUrl = this.pendingStreamUrl;
    if (!sourceUrl) {
      return null;
    }

    try {
      const resolved = new URL(sourceUrl, window.location.href);
      if (startSeconds > 0) {
        resolved.searchParams.set('start', startSeconds.toFixed(3));
      } else {
        resolved.searchParams.delete('start');
      }
      resolved.searchParams.set('window', VideoViewerComponent.WINDOWED_VIDEO_STREAM_SECONDS.toFixed(3));
      return resolved.toString();
    } catch {
      return sourceUrl;
    }
  }

  private parseVideoStartOffsetSeconds(url: string): number {
    try {
      const resolved = new URL(url, window.location.href);
      const rawStart = Number(resolved.searchParams.get('start') ?? '0');
      return Number.isFinite(rawStart) && rawStart > 0 ? rawStart : 0;
    } catch {
      return 0;
    }
  }

  private async restartVideoAt(seekSeconds: number, options?: { autoplay?: boolean; generation?: number }): Promise<void> {
    if (!this.canvasPlayer) {
      return;
    }

    const nextUrl = this.buildSeekUrl(seekSeconds);
    if (!nextUrl) {
      return;
    }
    if (this.loadingStreamUrl === nextUrl) {
      return;
    }

    const shouldResumePlayback = options?.autoplay ?? this.canvasPlayer.isPlaying();
    this.error = null;
    this.videoLoading = true;
    this.loadingStreamUrl = nextUrl;
    this.videoTimelineOffsetSeconds = seekSeconds;
    this.videoCurrentTime = seekSeconds;
    this.videoSeekTarget = seekSeconds;
    this.cdr.markForCheck();

    try {
      await this.canvasPlayer.loadFromUrl(nextUrl);
      if (options?.generation === undefined || options.generation === this.seekRequestGeneration) {
        this.currentStreamUrl = nextUrl;
        this.videoReady = true;
      }
      if (shouldResumePlayback && (options?.generation === undefined || options.generation === this.seekRequestGeneration)) {
        await this.canvasPlayer.play();
      }
    } catch (err: unknown) {
      if (options?.generation === undefined || options.generation === this.seekRequestGeneration) {
        this.error = err instanceof Error ? err.message : 'Seek failed';
      }
    } finally {
      if (this.loadingStreamUrl === nextUrl) {
        this.loadingStreamUrl = null;
      }
      if (options?.generation === undefined || options.generation === this.seekRequestGeneration) {
        this.videoLoading = false;
        this.cdr.markForCheck();
      }
    }
  }

  onVideoVolumeChange(event: Event): void {
    const nextVolume = Math.max(0, Math.min(100, Number((event.target as HTMLInputElement).value)));
    this.videoVolume = nextVolume;
    this.canvasPlayer?.setVolume(nextVolume / 100);
    this.cdr.markForCheck();
  }

  onVideoMouseMove(): void {
    this.videoShowControls = true;
    if (this.videoControlsTimeout) {
      clearTimeout(this.videoControlsTimeout);
    }
    if (this.videoIsPlaying) {
      this.videoControlsTimeout = setTimeout(() => {
        this.videoShowControls = false;
        this.cdr.markForCheck();
      }, 3000);
    }
  }

  onVideoMouseLeave(): void {
    if (!this.videoIsPlaying) {
      return;
    }
    this.videoShowControls = false;
    if (this.videoControlsTimeout) {
      clearTimeout(this.videoControlsTimeout);
      this.videoControlsTimeout = null;
    }
    this.cdr.markForCheck();
  }

  formatTime(seconds: number): string {
    if (!seconds || seconds < 0) {
      return '0:00';
    }
    const totalSecs = Math.floor(seconds);
    const mins = Math.floor(totalSecs / 60);
    const secs = totalSecs % 60;
    const hours = Math.floor(mins / 60);
    const displayMins = mins % 60;
    if (hours > 0) {
      return `${hours}:${displayMins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    }
    return `${displayMins}:${secs.toString().padStart(2, '0')}`;
  }

  formatSize(bytes: number): string {
    if (!bytes || bytes <= 0) {
      return '';
    }
    const units = ['B', 'KB', 'MB', 'GB'];
    const unitIndex = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const value = bytes / Math.pow(1024, unitIndex);
    return `${unitIndex === 0 ? value : value.toFixed(1)} ${units[unitIndex]}`;
  }

  private async createAndAttachCanvasPlayer(): Promise<CanvasVideoPlayer> {
    if (!this.videoContainerRef?.nativeElement) {
      throw new Error('Video container is not available.');
    }
    if (this.canvasPlayer) {
      this.canvasPlayer.destroy();
      this.canvasPlayer = null;
    }
    const player = await this.videoPlayerService.createPlayer(this.videoContainerRef.nativeElement);
    this.canvasPlayer = player;
    player.setVolume(this.videoVolume / 100);
    player.stateChanged$.subscribe((state) => {
      this.videoIsPlaying = state === 'playing';
      if (state !== 'playing') {
        this.videoShowControls = true;
      }
      this.cdr.markForCheck();
    });
    player.metadataReady$.subscribe((metadata: VideoMetadata) => {
      this.videoDuration = this.shouldUseWindowedVideoStreaming()
        ? Math.max(this.videoDuration, this.videoTimelineOffsetSeconds + metadata.duration / 1000)
        : metadata.duration / 1000;
      this.videoCodecInfo = metadata.videoCodec || '';
      this.videoResolution = `${metadata.width}\u00d7${metadata.height}`;
      this.cdr.markForCheck();
    });
    player.currentTime$.subscribe((seconds: number) => {
      if (this.seekDragActive) {
        return;
      }
      this.videoCurrentTime = this.shouldUseWindowedVideoStreaming()
        ? this.videoTimelineOffsetSeconds + seconds
        : seconds;
      this.cdr.markForCheck();
    });
    player.error$.subscribe((message: string) => {
      this.error = message;
      this.videoLoading = false;
      this.cdr.markForCheck();
    });
    return player;
  }

  private async initializeSegmentedPlayback(manifestUrl: string): Promise<void> {
    if (!this.videoContainerRef?.nativeElement) {
      return;
    }
    this.videoLoading = true;
    this.error = null;
    this.cdr.markForCheck();
    const loader = new SegmentVideoLoader();
    const source = await loader.load(manifestUrl);
    const player = await this.createAndAttachCanvasPlayer();
    await player.loadFromDemuxedSource(source);
    this.videoReady = true;
    this.videoLoading = false;
    this.cdr.markForCheck();
  }

  private async initializeUrlPlayback(streamUrl: string): Promise<void> {
    if (!this.videoContainerRef?.nativeElement) {
      return;
    }
    this.videoLoading = true;
    this.loadingStreamUrl = streamUrl;
    this.videoTimelineOffsetSeconds = this.parseVideoStartOffsetSeconds(streamUrl);
    this.error = null;
    this.cdr.markForCheck();
    const player = await this.createAndAttachCanvasPlayer();
    try {
      await player.loadFromUrl(streamUrl);
      this.currentStreamUrl = streamUrl;
      this.videoReady = true;
    } finally {
      if (this.loadingStreamUrl === streamUrl) {
        this.loadingStreamUrl = null;
      }
      this.videoLoading = false;
      this.cdr.markForCheck();
    }
  }

  private requestSegmentedPlaybackInitialization(): void {
    const manifestUrl = this.pendingManifestUrl;
    if (!manifestUrl || this.useLegacySurface || !this.videoContainerRef?.nativeElement) {
      return;
    }
    if (this.initializingManifestUrl === manifestUrl || this.canvasPlayer) {
      return;
    }

    this.initializingManifestUrl = manifestUrl;
    void this.initializeSegmentedPlayback(manifestUrl)
      .catch((err: unknown) => {
        this.error = err instanceof Error ? err.message : String(err);
        this.videoLoading = false;
        this.cdr.markForCheck();
      })
      .finally(() => {
        if (this.initializingManifestUrl === manifestUrl) {
          this.initializingManifestUrl = null;
        }
      });
  }

  private requestUrlPlaybackInitialization(): void {
    const streamUrl = this.pendingStreamUrl;
    if (!streamUrl || this.useLegacySurface || !this.videoContainerRef?.nativeElement) {
      return;
    }
    if (this.initializingStreamUrl === streamUrl || this.canvasPlayer) {
      return;
    }

    this.initializingStreamUrl = streamUrl;
    void this.initializeUrlPlayback(streamUrl)
      .catch((err: unknown) => {
        this.error = err instanceof Error ? err.message : String(err);
        this.videoLoading = false;
        this.cdr.markForCheck();
      })
      .finally(() => {
        if (this.initializingStreamUrl === streamUrl) {
          this.initializingStreamUrl = null;
        }
      });
  }

  private resetState(): void {
    this.useLegacySurface = false;
    this.loading = false;
    this.error = null;
    this.pendingManifestUrl = null;
    this.pendingStreamUrl = null;
    this.initializingManifestUrl = null;
    this.initializingStreamUrl = null;
    this.currentStreamUrl = null;
    this.loadingStreamUrl = null;
    this.playbackMode = '';
    this.fileSize = 0;
    this.videoReady = false;
    this.videoIsPlaying = false;
    this.videoLoading = false;
    this.videoShowControls = true;
    this.videoCurrentTime = 0;
    this.videoDuration = 0;
    this.videoTimelineOffsetSeconds = 0;
    this.videoCodecInfo = '';
    this.videoResolution = '';
    this.videoIsSeeking = false;
    this.videoSeekTarget = 0;
    this.seekPending = false;
    this.seekOriginSeconds = 0;
    this.seekRequestGeneration = 0;
    this.seekGeneration++;
    if (this.canvasPlayer) {
      this.canvasPlayer.destroy();
      this.canvasPlayer = null;
    }
  }
}
