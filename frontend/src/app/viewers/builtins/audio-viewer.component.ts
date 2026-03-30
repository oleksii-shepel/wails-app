import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  Input,
  OnChanges,
  OnDestroy,
  ViewChild,
} from '@angular/core';
import { VscodeScrollbarDirective } from '../../directives/vscode-scrollbar.directive';
import { ApiService } from '../../services/api.service';
import { FileInfo, FileProbe, FileType } from '../../shared/protocol';
import type { ViewerFileListItem } from '../../shared/protocol-enhanced';
import {
  VIEWER_BASE_STYLES,
  buildDisplayName,
  extractExt,
  formatSize,
  inferAudioMime,
} from './builtin-viewer.shared';

type AudioViewerKind = 'empty' | 'loading' | 'ready' | 'directory' | 'unsupported' | 'error';

interface AudioPlaylistItem {
  path: string;
  name: string;
}

@Component({
  selector: 'app-audio-viewer',
  standalone: true,
  imports: [CommonModule, VscodeScrollbarDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <div class="viewer-header">
        <div class="header-main">
          <span class="file-icon">♫</span>
          <div class="header-text">
            <div class="title" [title]="displayName">{{ displayName || 'Audio Preview' }}</div>
            <div class="path" [title]="displayPath">{{ displayPath || 'Select a file to preview' }}</div>
          </div>
        </div>

        <div class="header-meta" *ngIf="kind !== 'empty' && kind !== 'loading' && info">
          <span class="badge type-badge">{{ typeBadge }}</span>
          <span class="badge size-badge" *ngIf="info.size > 0">{{ formatBytes(info.size) }}</span>
          <span class="badge meta-badge" *ngIf="mimeLabel">{{ mimeLabel }}</span>
          <span class="badge meta-badge" *ngIf="duration > 0">{{ formatTime(duration) }}</span>
        </div>
      </div>

      <div class="viewer-toolbar" *ngIf="hasPlaylist">
        <button class="tool-btn" type="button" (click)="showPrevious()" [disabled]="!hasPrevious" title="Previous audio file">&#x2039;</button>
        <span class="playlist-counter">{{ currentPlaylistIndex + 1 }} / {{ playlistItems.length }}</span>
        <button class="tool-btn" type="button" (click)="showNext()" [disabled]="!hasNext" title="Next audio file">&#x203A;</button>
      </div>

      <div class="viewer-body" appVscodeScrollbar>
        <div *ngIf="kind === 'loading'" class="state-container">
          <div class="spinner"></div>
          <div class="state-label">Loading audio preview...</div>
        </div>

        <div *ngIf="kind === 'empty'" class="state-container">
          <div class="state-icon">♫</div>
          <div class="state-label">Select a file to preview</div>
        </div>

        <div *ngIf="kind === 'directory'" class="state-container">
          <div class="state-icon">📁</div>
          <div class="state-label">Directory - no preview available</div>
        </div>

        <div *ngIf="kind === 'unsupported'" class="state-container">
          <div class="state-icon">⛔</div>
          <div class="state-label">No audio preview for this file type</div>
          <div class="state-sub" *ngIf="fileExt">.{{ fileExt }}</div>
        </div>

        <div *ngIf="kind === 'error'" class="state-container state-error">
          <div class="state-icon">⚠</div>
          <div class="state-label">Preview failed</div>
          <div class="error-detail">{{ error }}</div>
        </div>

        <div *ngIf="kind === 'ready'" class="audio-player-shell">
          <audio
            #audioElement
            class="native-audio"
            [src]="streamUrl || undefined"
            preload="metadata"
            (loadedmetadata)="onLoadedMetadata($event)"
            (canplay)="onCanPlay()"
            (timeupdate)="onTimeUpdate($event)"
            (play)="onPlay()"
            (pause)="onPause()"
            (ended)="onEnded()"
            (waiting)="onWaiting()"
            (error)="onAudioError()">
          </audio>

          <section class="audio-stage" [class.playing]="isPlaying">
            <div class="art-panel">
              <div class="record-shell" [class.playing]="isPlaying">
                <div class="record-core"></div>
                <div class="record-ring ring-1"></div>
                <div class="record-ring ring-2"></div>
              </div>

              <div class="stage-copy">
                <div class="eyebrow">Audio preview</div>
                <div class="track-title" [title]="displayName">{{ displayName || 'Unknown track' }}</div>
                <div class="track-status">{{ playbackLabel }}</div>
              </div>
            </div>

            <div class="wave-panel">
              <div class="wave-copy">
                <span class="wave-heading">Signal</span>
                <span class="wave-sub">Direct stream</span>
              </div>

              <div class="waveform" [class.playing]="isPlaying">
                <span
                  *ngFor="let bar of waveformBars; let index = index; trackBy: trackWaveBar"
                  class="wave-bar"
                  [style.height.%]="bar"
                  [style.animation-delay.ms]="index * -90">
                </span>
              </div>
            </div>
          </section>

          <div class="video-control-bar show">
            <div class="progress-section">
              <div class="progress-container">
                <input
                  type="range"
                  min="0"
                  [max]="duration || 0"
                  step="any"
                  [value]="displayedCurrentTime"
                  [disabled]="!audioReady"
                  class="seek-slider"
                  (mousedown)="onSeekStart()"
                  (touchstart)="onSeekStart()"
                  (input)="onSeekInput($event)"
                  (change)="onSeekCommit($event)">
                <div class="progress-fill" [style.width.%]="progressPercent"></div>
              </div>
            </div>

            <div class="controls-row">
              <button class="control-btn play-pause-btn" type="button" [disabled]="!audioReady" (click)="togglePlayPause()">
                <span class="icon">{{ isPlaying ? '⏸' : '▶' }}</span>
              </button>

              <button class="control-btn skip-btn" type="button" [disabled]="!audioReady" (click)="seekRelative(-10)">
                -10
              </button>

              <button class="control-btn skip-btn" type="button" [disabled]="!audioReady" (click)="seekRelative(10)">
                +10
              </button>

              <div class="time-display">
                <span class="current-time">{{ formatTime(displayedCurrentTime) }}</span>
                <span class="separator">/</span>
                <span class="duration">{{ formatTime(duration) }}</span>
              </div>

              <div class="spacer"></div>

              <div class="volume-control">
                <span class="volume-icon" role="button" tabindex="0" (click)="toggleMute()" (keydown.enter)="toggleMute()" (keydown.space)="toggleMute()">{{ volumeIcon }}</span>
                <input
                  type="range"
                  min="0"
                  max="100"
                  [value]="volume"
                  [disabled]="!audioReady"
                  class="volume-slider"
                  (input)="onVolumeChange($event)">
              </div>
            </div>
          </div>

          <section class="playlist-panel" *ngIf="hasPlaylist">
            <div class="playlist-header">
              <span class="playlist-title">Playlist</span>
              <span class="playlist-summary">{{ playlistItems.length }} tracks</span>
            </div>

            <div class="playlist-list">
              <button
                *ngFor="let item of playlistItems; let index = index; trackBy: trackPlaylistItem"
                class="playlist-item"
                type="button"
                [class.active]="index === currentPlaylistIndex"
                (click)="selectPlaylistIndex(index)">
                <span class="playlist-index">{{ index + 1 }}</span>
                <span class="playlist-name" [title]="item.name">{{ item.name }}</span>
              </button>
            </div>
          </section>
        </div>
      </div>
    </div>
  `,
  styles: [VIEWER_BASE_STYLES, `
    .audio-player-shell {
      display: flex;
      flex-direction: column;
      min-height: 100%;
      background: var(--vsc-editor-background, #1e1e1e);
    }

    .viewer-toolbar {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 6px 12px;
      border-bottom: 1px solid var(--vsc-border, #454545);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 96%, black);
      flex-shrink: 0;
    }

    .tool-btn {
      font: inherit;
      font-size: 12px;
      line-height: 1.4;
      min-width: 28px;
      padding: 3px 8px;
      border-radius: 3px;
      border: 1px solid transparent;
      background: transparent;
      color: var(--vsc-foreground, #ccc);
      cursor: pointer;
    }

    .tool-btn:hover:not(:disabled) {
      background: var(--vsc-list-hover-background, rgba(255,255,255,0.05));
      border-color: var(--vsc-border, #454545);
    }

    .tool-btn:disabled {
      opacity: 0.4;
      cursor: default;
    }

    .playlist-counter {
      font-size: 11px;
      font-weight: 600;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 70%, transparent);
    }

    .native-audio {
      display: none;
    }

    .audio-stage {
      display: grid;
      grid-template-columns: minmax(220px, 320px) minmax(0, 1fr);
      gap: 16px;
      padding: 16px;
      flex: 1 1 auto;
      min-height: 0;
      align-items: stretch;
    }

    .art-panel,
    .wave-panel {
      border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 75%, transparent);
      border-radius: 8px;
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 94%, black);
      box-shadow: 0 10px 28px rgba(0, 0, 0, 0.22);
    }

    .art-panel {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 16px;
      padding: 20px;
      min-height: 240px;
    }

    .record-shell {
      position: relative;
      width: 156px;
      aspect-ratio: 1;
      border-radius: 50%;
      background:
        radial-gradient(circle at center, rgba(0, 122, 204, 0.16) 0 14%, transparent 14% 16%),
        radial-gradient(circle at center, rgba(255, 255, 255, 0.05) 0 27%, transparent 27% 29%),
        linear-gradient(180deg, rgba(255, 255, 255, 0.03), rgba(0, 0, 0, 0.12)),
        color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 88%, black);
      border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 78%, transparent);
      box-shadow:
        inset 0 0 0 1px rgba(255, 255, 255, 0.02),
        0 12px 26px rgba(0, 0, 0, 0.22);
      overflow: hidden;
    }

    .record-shell.playing {
      animation: spin-record 10s linear infinite;
    }

    .record-core,
    .record-ring {
      position: absolute;
      inset: 50%;
      transform: translate(-50%, -50%);
      border-radius: 50%;
    }

    .record-core {
      width: 18px;
      height: 18px;
      background: var(--vsc-accent-blue, #0078d4);
      box-shadow: 0 0 14px color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 35%, transparent);
      z-index: 2;
    }

    .record-ring {
      border: 1px solid rgba(255, 255, 255, 0.06);
    }

    .ring-1 {
      width: 72px;
      height: 72px;
    }

    .ring-2 {
      width: 112px;
      height: 112px;
    }

    .stage-copy {
      display: flex;
      flex-direction: column;
      gap: 4px;
      text-align: center;
      max-width: 260px;
    }

    .eyebrow {
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.18em;
      text-transform: uppercase;
      color: var(--vsc-accent-blue, #4da8ff);
      opacity: 0.9;
    }

    .track-title {
      font-size: 18px;
      font-weight: 700;
      line-height: 1.3;
      color: var(--vsc-foreground, #cccccc);
      word-break: break-word;
    }

    .track-status {
      font-size: 12px;
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 68%, transparent);
    }

    .wave-panel {
      display: flex;
      flex-direction: column;
      justify-content: center;
      gap: 16px;
      padding: 20px;
      min-height: 240px;
    }

    .wave-copy {
      display: flex;
      justify-content: space-between;
      gap: 12px;
      align-items: baseline;
    }

    .wave-heading {
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.14em;
      text-transform: uppercase;
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 85%, transparent);
    }

    .wave-sub {
      font-size: 11px;
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 52%, transparent);
    }

    .waveform {
      display: flex;
      align-items: end;
      gap: 6px;
      height: 132px;
      padding: 8px 0;
    }

    .wave-bar {
      flex: 1 1 0%;
      min-width: 6px;
      border-radius: 999px 999px 3px 3px;
      background: linear-gradient(
        180deg,
        color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 85%, white),
        color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 32%, transparent)
      );
      transform-origin: bottom;
      animation: wave-idle 1.4s ease-in-out infinite;
      animation-play-state: paused;
      box-shadow: 0 0 12px color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 14%, transparent);
    }

    .waveform.playing .wave-bar {
      animation-name: wave-live;
      animation-play-state: running;
    }

    .video-control-bar {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 12px;
      border-top: 1px solid rgba(255, 255, 255, 0.08);
      background: linear-gradient(to bottom, rgba(0,0,0,0.01), rgba(0,0,0,0.72));
      opacity: 1;
      pointer-events: auto;
    }

    .progress-section {
      display: flex;
      align-items: center;
      height: 24px;
    }

    .progress-container {
      position: relative;
      flex: 1;
      height: 4px;
      border-radius: 2px;
      background: rgba(255, 255, 255, 0.2);
      overflow: hidden;
    }

    .progress-fill {
      position: absolute;
      inset: 0 auto 0 0;
      background: linear-gradient(90deg, var(--vsc-accent-blue, #0078d4), #00a4ef);
      border-radius: inherit;
      pointer-events: none;
      transition: width 0.05s linear;
    }

    .seek-slider {
      position: absolute;
      inset: -10px 0;
      width: 100%;
      margin: 0;
      appearance: none;
      -webkit-appearance: none;
      background: transparent;
      outline: none;
      cursor: pointer;
      z-index: 1;
    }

    .seek-slider::-webkit-slider-thumb {
      appearance: none;
      -webkit-appearance: none;
      width: 12px;
      height: 12px;
      border-radius: 50%;
      border: none;
      background: white;
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
      border-radius: 50%;
      border: none;
      background: white;
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
      min-width: 0;
      padding: 0 4px;
    }

    .control-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 36px;
      height: 36px;
      padding: 0 8px;
      border-radius: 4px;
      border: 1px solid rgba(255, 255, 255, 0.15);
      background: rgba(255, 255, 255, 0.1);
      color: white;
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      transition: all 0.2s ease;
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
      opacity: 0.45;
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

    .skip-btn {
      min-width: 42px;
      font-size: 12px;
      font-variant-numeric: tabular-nums;
    }

    .play-pause-btn:hover:not(:disabled) {
      background: #1084d7;
      box-shadow: 0 0 12px rgba(0, 120, 212, 0.5);
    }

    .time-display {
      display: flex;
      align-items: center;
      gap: 4px;
      min-width: 90px;
      justify-content: center;
      padding: 4px 8px;
      border-radius: 3px;
      background: rgba(0, 0, 0, 0.2);
      color: #ccc;
      font-size: 12px;
      font-variant-numeric: tabular-nums;
      font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
    }

    .current-time {
      color: white;
      font-weight: 500;
    }

    .separator {
      opacity: 0.5;
    }

    .spacer {
      flex: 1 1 auto;
      min-width: 12px;
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
      line-height: 1;
      cursor: pointer;
      user-select: none;
    }

    .volume-slider {
      width: 120px;
      max-width: 100%;
      height: 4px;
      appearance: none;
      -webkit-appearance: none;
      background: rgba(255, 255, 255, 0.2);
      border-radius: 2px;
      outline: none;
      cursor: pointer;
    }

    .volume-slider::-webkit-slider-thumb {
      appearance: none;
      -webkit-appearance: none;
      width: 10px;
      height: 10px;
      border-radius: 50%;
      border: none;
      background: white;
      box-shadow: 0 2px 6px rgba(0, 0, 0, 0.5);
    }

    .volume-slider::-moz-range-thumb {
      width: 10px;
      height: 10px;
      border-radius: 50%;
      border: none;
      background: white;
      box-shadow: 0 2px 6px rgba(0, 0, 0, 0.5);
    }

    .volume-slider:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .playlist-panel {
      border-top: 1px solid var(--vsc-border, #454545);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 94%, black);
      padding: 12px;
      display: flex;
      flex-direction: column;
      gap: 10px;
    }

    .playlist-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
    }

    .playlist-title {
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.14em;
      text-transform: uppercase;
      color: var(--vsc-accent-blue, #4da8ff);
    }

    .playlist-summary {
      font-size: 11px;
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 60%, transparent);
    }

    .playlist-list {
      display: flex;
      flex-direction: column;
      gap: 6px;
      max-height: 240px;
      overflow: auto;
    }

    .playlist-item {
      display: flex;
      align-items: center;
      gap: 10px;
      width: 100%;
      padding: 10px 12px;
      border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 75%, transparent);
      border-radius: 8px;
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 92%, black);
      color: var(--vsc-foreground, #ccc);
      cursor: pointer;
      text-align: left;
    }

    .playlist-item:hover {
      background: var(--vsc-list-hover-background, rgba(255,255,255,0.05));
      border-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 35%, transparent);
    }

    .playlist-item.active {
      background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 18%, transparent);
      border-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 48%, transparent);
    }

    .playlist-index {
      min-width: 24px;
      font-size: 11px;
      font-weight: 700;
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 62%, transparent);
    }

    .playlist-name {
      min-width: 0;
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    @keyframes spin-record {
      to {
        transform: rotate(360deg);
      }
    }

    @keyframes wave-idle {
      0%, 100% {
        transform: scaleY(0.42);
      }
      50% {
        transform: scaleY(0.6);
      }
    }

    @keyframes wave-live {
      0%, 100% {
        transform: scaleY(0.35);
      }
      35% {
        transform: scaleY(1);
      }
      70% {
        transform: scaleY(0.52);
      }
    }

    @media (max-width: 900px) {
      .audio-stage {
        grid-template-columns: 1fr;
      }

      .art-panel,
      .wave-panel {
        min-height: 200px;
      }
    }

    @media (max-width: 640px) {
      .audio-stage {
        padding: 12px;
        gap: 12px;
      }

      .video-control-bar {
        padding: 12px;
      }

      .controls-row {
        flex-wrap: wrap;
      }

      .spacer {
        display: none;
      }

      .volume-control {
        width: 100%;
      }

      .volume-slider {
        width: 100%;
        max-width: 100%;
      }
    }
  `],
})
export class AudioViewerComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  @Input() fileList: ViewerFileListItem[] = [];
  @Input() probe: FileProbe | null = null;
  @Input() viewerId: string | null = null;

  @ViewChild('audioElement')
  set audioElementRef(value: ElementRef<HTMLAudioElement> | undefined) {
    this.audioElement = value?.nativeElement ?? null;
    this.syncAudioElement();
  }

  kind: AudioViewerKind = 'empty';
  info: FileInfo | null = null;
  error = '';
  streamUrl: string | null = null;
  audioReady = false;
  isPlaying = false;
  isBuffering = false;
  isSeeking = false;
  currentTime = 0;
  seekTarget = 0;
  duration = 0;
  volume = 90;
  playlistItems: AudioPlaylistItem[] = [];
  currentPlaylistIndex = 0;

  readonly waveformBars = [38, 62, 48, 76, 52, 90, 44, 82, 58, 68, 40, 86, 54, 72, 46, 80];

  private audioElement: HTMLAudioElement | null = null;
  private loadVersion = 0;
  private autoplayPending = false;
  private lastNonZeroVolume = 90;
  private activeTargetPath = '';

  constructor(
    private readonly api: ApiService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get displayName(): string {
    return buildDisplayName(this.info, this.activeTargetPath, null);
  }

  get displayPath(): string {
    return this.activeTargetPath;
  }

  get fileExt(): string {
    return extractExt(this.activeTargetPath);
  }

  get typeBadge(): string {
    const ext = this.fileExt;
    return ext ? ext.toUpperCase() : 'AUDIO';
  }

  get mimeLabel(): string {
    return this.resolveAudioMime(this.activeTargetPath, this.activeTargetPath === this.targetPath) ?? '';
  }

  get displayedCurrentTime(): number {
    return this.isSeeking ? this.seekTarget : this.currentTime;
  }

  get progressPercent(): number {
    if (!this.duration || this.duration <= 0) {
      return 0;
    }
    return Math.max(0, Math.min(100, (this.displayedCurrentTime / this.duration) * 100));
  }

  get playbackLabel(): string {
    if (this.isBuffering) {
      return 'Buffering stream';
    }
    if (!this.audioReady) {
      return 'Preparing playback';
    }
    if (this.isPlaying) {
      return 'Playing now';
    }
    if (this.currentTime > 0 && this.currentTime >= this.duration && this.duration > 0) {
      return 'Playback finished';
    }
    if (this.currentTime > 0) {
      return 'Paused';
    }
    return 'Ready to play';
  }

  get volumeIcon(): string {
    if (this.volume <= 0) {
      return '🔇';
    }
    if (this.volume < 50) {
      return '🔉';
    }
    return '🔊';
  }

  get hasPlaylist(): boolean {
    return this.playlistItems.length > 1;
  }

  get hasPrevious(): boolean {
    return this.currentPlaylistIndex > 0;
  }

  get hasNext(): boolean {
    return this.currentPlaylistIndex >= 0 && this.currentPlaylistIndex < this.playlistItems.length - 1;
  }

  async ngOnChanges(): Promise<void> {
    const currentLoad = ++this.loadVersion;
    this.resetState();

    const path = (this.targetPath ?? '').trim();
    if (!path) {
      this.kind = 'empty';
      this.cdr.markForCheck();
      return;
    }

    this.kind = 'loading';
    this.cdr.markForCheck();

    try {
      const prepared = this.preparePlaylist(path);
      this.playlistItems = prepared.items;
      this.currentPlaylistIndex = prepared.index;
      this.activeTargetPath = prepared.items[prepared.index]?.path ?? path;
      await this.loadFromDisk(this.activeTargetPath, currentLoad);
    } catch (err: unknown) {
      if (currentLoad !== this.loadVersion) {
        return;
      }
      this.error = err instanceof Error ? err.message : String(err);
      this.kind = 'error';
    }

    if (currentLoad === this.loadVersion) {
      this.cdr.markForCheck();
    }
  }

  ngOnDestroy(): void {
    this.pauseAndDetachAudio();
  }

  async togglePlayPause(): Promise<void> {
    const audio = this.audioElement;
    if (!audio || !this.audioReady) {
      return;
    }
    try {
      if (audio.paused) {
        this.isBuffering = true;
        this.cdr.markForCheck();
        await audio.play();
      } else {
        audio.pause();
      }
    } catch {
      this.isBuffering = false;
      this.cdr.markForCheck();
    }
  }

  seekRelative(deltaSeconds: number): void {
    const audio = this.audioElement;
    if (!audio || !this.audioReady) {
      return;
    }
    const nextTime = Math.max(0, Math.min(this.duration || audio.duration || 0, audio.currentTime + deltaSeconds));
    audio.currentTime = nextTime;
    this.currentTime = nextTime;
    this.seekTarget = nextTime;
    this.cdr.markForCheck();
  }

  onSeekStart(): void {
    if (!this.audioReady) {
      return;
    }
    this.isSeeking = true;
    this.seekTarget = this.currentTime;
    this.cdr.markForCheck();
  }

  onSeekInput(event: Event): void {
    if (!this.audioReady) {
      return;
    }
    this.seekTarget = Number((event.target as HTMLInputElement).value);
    this.cdr.markForCheck();
  }

  onSeekCommit(event: Event): void {
    const audio = this.audioElement;
    if (!audio || !this.audioReady) {
      return;
    }
    const nextTime = Number((event.target as HTMLInputElement).value);
    audio.currentTime = nextTime;
    this.currentTime = nextTime;
    this.seekTarget = nextTime;
    this.isSeeking = false;
    this.cdr.markForCheck();
  }

  onVolumeChange(event: Event): void {
    const nextVolume = Math.max(0, Math.min(100, Number((event.target as HTMLInputElement).value)));
    this.volume = nextVolume;
    if (nextVolume > 0) {
      this.lastNonZeroVolume = nextVolume;
    }
    this.syncAudioElement();
    this.cdr.markForCheck();
  }

  toggleMute(): void {
    if (this.volume <= 0) {
      this.volume = this.lastNonZeroVolume > 0 ? this.lastNonZeroVolume : 90;
    } else {
      this.lastNonZeroVolume = this.volume;
      this.volume = 0;
    }
    this.syncAudioElement();
    this.cdr.markForCheck();
  }

  onLoadedMetadata(event: Event): void {
    const audio = event.target as HTMLAudioElement;
    this.duration = Number.isFinite(audio.duration) ? audio.duration : 0;
    this.currentTime = Number.isFinite(audio.currentTime) ? audio.currentTime : 0;
    this.seekTarget = this.currentTime;
    this.audioReady = this.duration > 0 || audio.readyState >= HTMLMediaElement.HAVE_METADATA;
    this.cdr.markForCheck();
  }

  onCanPlay(): void {
    this.audioReady = true;
    this.isBuffering = false;
    this.cdr.markForCheck();
    if (this.autoplayPending) {
      this.autoplayPending = false;
      void this.tryAutoplay();
    }
  }

  onTimeUpdate(event: Event): void {
    if (this.isSeeking) {
      return;
    }
    const audio = event.target as HTMLAudioElement;
    this.currentTime = Number.isFinite(audio.currentTime) ? audio.currentTime : 0;
    this.cdr.markForCheck();
  }

  onPlay(): void {
    this.isPlaying = true;
    this.isBuffering = false;
    this.cdr.markForCheck();
  }

  onPause(): void {
    this.isPlaying = false;
    this.isBuffering = false;
    this.cdr.markForCheck();
  }

  onEnded(): void {
    this.isPlaying = false;
    this.isBuffering = false;
    this.currentTime = this.duration;
    this.seekTarget = this.duration;
    this.cdr.markForCheck();
  }

  onWaiting(): void {
    if (this.audioReady) {
      this.isBuffering = true;
      this.cdr.markForCheck();
    }
  }

  onAudioError(): void {
    this.error = 'Audio stream failed to load.';
    this.kind = 'error';
    this.audioReady = false;
    this.isPlaying = false;
    this.isBuffering = false;
    this.cdr.markForCheck();
  }

  formatTime(seconds: number): string {
    if (!seconds || seconds < 0) {
      return '0:00';
    }
    const totalSeconds = Math.floor(seconds);
    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const remainder = totalSeconds % 60;
    if (hours > 0) {
      return `${hours}:${minutes.toString().padStart(2, '0')}:${remainder.toString().padStart(2, '0')}`;
    }
    return `${minutes}:${remainder.toString().padStart(2, '0')}`;
  }

  formatBytes(bytes: number): string {
    return formatSize(bytes);
  }

  trackWaveBar(index: number): number {
    return index;
  }

  showPrevious(): void {
    if (!this.hasPrevious) {
      return;
    }
    void this.displayPlaylistIndex(this.currentPlaylistIndex - 1);
  }

  showNext(): void {
    if (!this.hasNext) {
      return;
    }
    void this.displayPlaylistIndex(this.currentPlaylistIndex + 1);
  }

  selectPlaylistIndex(index: number): void {
    if (index === this.currentPlaylistIndex) {
      return;
    }
    void this.displayPlaylistIndex(index);
  }

  trackPlaylistItem(_index: number, item: AudioPlaylistItem): string {
    return item.path;
  }

  private async loadFromDisk(path: string, loadVersion: number): Promise<void> {
    const info = await this.api.getFileInfo(path);
    if (loadVersion !== this.loadVersion) {
      return;
    }
    this.activeTargetPath = path;
    this.info = info;

    if (info.type === FileType.DIRECTORY) {
      this.kind = 'directory';
      return;
    }

    if (!this.resolveAudioMime(path, path === this.targetPath)) {
      this.kind = 'unsupported';
      return;
    }

    this.streamUrl = await this.api.getStreamURL(path);
    if (loadVersion !== this.loadVersion) {
      return;
    }

    this.kind = 'ready';
    this.autoplayPending = true;
  }

  private resolveAudioMime(path: string, allowProbe: boolean): string | null {
    const normalizedProbeMime = allowProbe ? (this.probe?.mimeType ?? '').split(';', 1)[0].trim().toLowerCase() : '';
    if (normalizedProbeMime.startsWith('audio/')) {
      return normalizedProbeMime;
    }
    return inferAudioMime(extractExt(path));
  }

  private preparePlaylist(path: string): { items: AudioPlaylistItem[]; index: number } {
    const normalizedTarget = (path ?? '').trim();
    const items = this.fileList
      .filter(item => item.path && this.isAudioCandidate(item.path))
      .map(item => ({ path: item.path, name: item.name || this.baseName(item.path) }));

    if (!items.length) {
      return {
        items: [{ path: normalizedTarget, name: this.baseName(normalizedTarget) }],
        index: 0,
      };
    }

    const currentIndex = items.findIndex(item => item.path === normalizedTarget);
    if (currentIndex >= 0) {
      return { items, index: currentIndex };
    }

    return {
      items: [{ path: normalizedTarget, name: this.baseName(normalizedTarget) }, ...items],
      index: 0,
    };
  }

  private isAudioCandidate(path: string): boolean {
    return !!inferAudioMime(extractExt(path));
  }

  private async displayPlaylistIndex(index: number): Promise<void> {
    const item = this.playlistItems[index];
    if (!item) {
      return;
    }

    const currentLoad = ++this.loadVersion;
    this.pauseAndDetachAudio();
    this.currentPlaylistIndex = index;
    this.kind = 'loading';
    this.info = null;
    this.error = '';
    this.streamUrl = null;
    this.audioReady = false;
    this.isPlaying = false;
    this.isBuffering = false;
    this.isSeeking = false;
    this.currentTime = 0;
    this.seekTarget = 0;
    this.duration = 0;
    this.autoplayPending = false;
    this.activeTargetPath = item.path;
    this.cdr.markForCheck();

    try {
      await this.loadFromDisk(item.path, currentLoad);
    } catch (err: unknown) {
      if (currentLoad !== this.loadVersion) {
        return;
      }
      this.error = err instanceof Error ? err.message : String(err);
      this.kind = 'error';
    }

    if (currentLoad === this.loadVersion) {
      this.cdr.markForCheck();
    }
  }

  private baseName(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/');
    return normalized.split('/').pop() ?? normalized;
  }

  private syncAudioElement(): void {
    const audio = this.audioElement;
    if (!audio) {
      return;
    }
    audio.volume = this.volume / 100;
  }

  private async tryAutoplay(): Promise<void> {
    const audio = this.audioElement;
    if (!audio) {
      return;
    }
    try {
      this.isBuffering = true;
      this.cdr.markForCheck();
      await audio.play();
    } catch {
      this.isBuffering = false;
      this.cdr.markForCheck();
    }
  }

  private resetState(): void {
    this.pauseAndDetachAudio();
    this.kind = 'empty';
    this.info = null;
    this.error = '';
    this.streamUrl = null;
    this.audioReady = false;
    this.isPlaying = false;
    this.isBuffering = false;
    this.isSeeking = false;
    this.currentTime = 0;
    this.seekTarget = 0;
    this.duration = 0;
    this.autoplayPending = false;
    this.playlistItems = [];
    this.currentPlaylistIndex = 0;
    this.activeTargetPath = '';
  }

  private pauseAndDetachAudio(): void {
    const audio = this.audioElement;
    if (!audio) {
      return;
    }
    audio.pause();
    audio.removeAttribute('src');
    audio.load();
  }
}
