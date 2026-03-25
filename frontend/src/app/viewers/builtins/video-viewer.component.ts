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
          <span class="badge seek-badge">segmented</span>
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
                  [value]="videoIsSeeking ? videoSeekTarget : videoCurrentTime"
                  [disabled]="!videoReady || videoLoading"
                  class="seek-slider"
                  (mousedown)="onVideoSeekStart()"
                  (touchstart)="onVideoSeekStart()"
                  (input)="onVideoSeekInput($event)"
                  (change)="onVideoSeek($event)">
                <div
                  class="progress-fill"
                  [style.width.%]="((videoIsSeeking ? videoSeekTarget : videoCurrentTime) / videoDuration) * 100 || 0"></div>
              </div>
            </div>

            <div class="controls-row">
              <button class="control-btn play-pause-btn" [disabled]="!videoReady" (click)="videoPlayPause()">
                <span class="icon">{{ videoIsPlaying ? '⏸' : '▶' }}</span>
              </button>

              <div class="time-display">
                <span class="current-time">{{ formatTime(videoIsSeeking ? videoSeekTarget : videoCurrentTime) }}</span>
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
})
export class VideoViewerComponent implements OnChanges, AfterViewInit, OnDestroy {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;
  @ViewChild('videoContainer') videoContainerRef?: ElementRef<HTMLDivElement>;

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
  private videoControlsTimeout: ReturnType<typeof setTimeout> | null = null;

  constructor(
    private readonly api: ApiService,
    private readonly cdr: ChangeDetectorRef,
    private readonly videoPlayerService: CanvasVideoPlayerService,
  ) {}

  get displayName(): string {
    const normalized = (this.targetPath ?? '').replace(/\\/g, '/');
    return normalized.split('/').pop() ?? normalized;
  }

  get typeBadge(): string {
    const ext = (this.targetPath.split('.').pop() ?? 'video').toUpperCase();
    return ext || 'VIDEO';
  }

  ngAfterViewInit(): void {
    if (!this.useLegacySurface && this.pendingManifestUrl) {
      void this.initializeSegmentedPlayback(this.pendingManifestUrl);
    }
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
      if (mode !== 'segmented' || !url) {
        this.useLegacySurface = true;
      } else {
        this.pendingManifestUrl = url;
        if (this.videoContainerRef?.nativeElement) {
          await this.initializeSegmentedPlayback(url);
        }
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
    if (!this.videoReady || this.videoLoading) {
      return;
    }
    this.videoIsSeeking = true;
    this.videoSeekTarget = this.videoCurrentTime;
    this.cdr.markForCheck();
  }

  onVideoSeekInput(event: Event): void {
    if (!this.videoReady || this.videoLoading) {
      return;
    }
    this.videoSeekTarget = parseFloat((event.target as HTMLInputElement).value);
    this.cdr.markForCheck();
  }

  async onVideoSeek(event: Event): Promise<void> {
    if (!this.canvasPlayer || !this.videoReady || this.videoLoading) {
      return;
    }

    const seekSeconds = parseFloat((event.target as HTMLInputElement).value);
    try {
      this.videoIsSeeking = true;
      this.videoSeekTarget = seekSeconds;
      this.videoCurrentTime = seekSeconds;
      this.cdr.markForCheck();
      await this.canvasPlayer.seek(seekSeconds * 1000);
    } catch (err: unknown) {
      this.error = err instanceof Error ? err.message : 'Seek failed';
    } finally {
      this.videoIsSeeking = false;
      this.cdr.markForCheck();
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

  private async initializeSegmentedPlayback(manifestUrl: string): Promise<void> {
    if (!this.videoContainerRef?.nativeElement) {
      return;
    }

    if (this.canvasPlayer) {
      this.canvasPlayer.destroy();
      this.canvasPlayer = null;
    }

    this.videoLoading = true;
    this.error = null;
    this.cdr.markForCheck();

    const loader = new SegmentVideoLoader();
    const source = await loader.load(manifestUrl);
    const player = await this.videoPlayerService.createPlayer(this.videoContainerRef.nativeElement);
    this.canvasPlayer = player;
    this.canvasPlayer.setVolume(this.videoVolume / 100);
    this.canvasPlayer.stateChanged$.subscribe((state) => {
      this.videoIsPlaying = state === 'playing';
      if (state !== 'playing') {
        this.videoShowControls = true;
      }
      this.cdr.markForCheck();
    });
    this.canvasPlayer.metadataReady$.subscribe((metadata: VideoMetadata) => {
      this.videoDuration = metadata.duration / 1000;
      this.videoCodecInfo = metadata.videoCodec || '';
      this.videoResolution = `${metadata.width}×${metadata.height}`;
      this.cdr.markForCheck();
    });
    this.canvasPlayer.currentTime$.subscribe((seconds: number) => {
      if (this.videoIsSeeking) {
        return;
      }
      this.videoCurrentTime = seconds;
      this.cdr.markForCheck();
    });
    this.canvasPlayer.error$.subscribe((message: string) => {
      this.error = message;
      this.videoLoading = false;
      this.cdr.markForCheck();
    });
    await this.canvasPlayer.loadFromDemuxedSource(source);
    this.videoReady = true;
    this.videoLoading = false;
    this.cdr.markForCheck();
  }

  private resetState(): void {
    this.useLegacySurface = false;
    this.loading = false;
    this.error = null;
    this.pendingManifestUrl = null;
    this.fileSize = 0;
    this.videoReady = false;
    this.videoIsPlaying = false;
    this.videoLoading = false;
    this.videoShowControls = true;
    this.videoCurrentTime = 0;
    this.videoDuration = 0;
    this.videoCodecInfo = '';
    this.videoResolution = '';
    this.videoIsSeeking = false;
    this.videoSeekTarget = 0;
    if (this.canvasPlayer) {
      this.canvasPlayer.destroy();
      this.canvasPlayer = null;
    }
  }
}