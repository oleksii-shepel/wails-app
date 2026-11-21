import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  HostListener,
  Input,
  OnChanges,
  OnDestroy,
  ViewChild,
} from '@angular/core';
import { parseBuffer, selectCover, type IPicture } from 'music-metadata';
import { ApiService } from '../../services/api.service';
import { FileInfo, FileProbe, FileType } from '../../shared/protocol';
import type { ViewerFileListItem } from '../../shared/protocol-enhanced';
import {
  VIEWER_PRODUCTION_CHROME_STYLES,
  VIEWER_BASE_STYLES,
  buildDisplayName,
  extractExt,
  formatSize,
  inferAudioMime,
} from './builtin-viewer.shared';

type AudioViewerKind = 'empty' | 'loading' | 'ready' | 'directory' | 'unsupported' | 'error';

type AudioTagKey = 'title' | 'artist' | 'album' | 'albumArtist' | 'genre' | 'year' | 'track' | 'composer';

interface AudioViewerTags {
  title?: string;
  artist?: string;
  album?: string;
  albumArtist?: string;
  genre?: string;
  year?: string;
  track?: string;
  composer?: string;
  coverArtDataUrl?: string;
}

interface AudioTagRow {
  key: AudioTagKey;
  label: string;
  value: string;
}

@Component({
  selector: 'app-audio-viewer',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <!-- ── BODY ───────────────────────────────────────── -->
      <div class="viewer-body">

        <!-- loading -->
        <div *ngIf="kind === 'loading'" class="state-container" role="status">
          <div class="spinner"></div>
          <div class="state-label">Loading audio preview...</div>
        </div>

        <!-- empty -->
        <div *ngIf="kind === 'empty'" class="state-container">
          <div class="state-icon-wrap" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="4"/><path d="M16 8V4l5-2v4l-5 2z"/>
            </svg>
          </div>
          <div class="state-label">Select a file to preview</div>
        </div>

        <!-- directory -->
        <div *ngIf="kind === 'directory'" class="state-container">
          <div class="state-icon-wrap state-icon-folder" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
            </svg>
          </div>
          <div class="state-label">Directory — no preview available</div>
        </div>

        <!-- unsupported -->
        <div *ngIf="kind === 'unsupported'" class="state-container">
          <div class="state-icon-wrap state-icon-warn" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
            </svg>
          </div>
          <div class="state-label">No audio preview for this file type</div>
          <div class="state-sub" *ngIf="fileExt">.{{ fileExt }}</div>
        </div>

        <!-- error -->
        <div *ngIf="kind === 'error'" class="state-container state-error" role="alert">
          <div class="state-icon-wrap state-icon-error" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
              <line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>
            </svg>
          </div>
          <div class="state-label">Preview failed</div>
          <div class="error-detail">{{ error }}</div>
        </div>

        <!-- ── READY STATE ──────────────────────────────── -->
        <div *ngIf="kind === 'ready'" class="audio-player-shell">

          <!-- hidden native audio element -->
          <audio
            #audioElement
            class="native-audio"
            [src]="streamUrl || undefined"
            preload="metadata"
            crossorigin="anonymous"
            (loadedmetadata)="onLoadedMetadata($event)"
            (canplay)="onCanPlay()"
            (timeupdate)="onTimeUpdate($event)"
            (play)="onPlay()"
            (pause)="onPause()"
            (ended)="onEnded()"
            (waiting)="onWaiting()"
            (error)="onAudioError()">
          </audio>

          <!-- ── ART STAGE ──────────────────────────────── -->
          <section class="art-stage">

            <!-- left: disc panel -->
            <div class="disc-panel">
              <div class="disc-wrap" [class.playing]="isPlaying" [class.has-cover]="!!coverArtDataUrl">
                <ng-container *ngIf="coverArtDataUrl; else discFallback">
                  <div class="cover-art-frame">
                    <img class="cover-art-image" [src]="coverArtDataUrl" [alt]="displayTitle">
                  </div>
                </ng-container>
                <ng-template #discFallback>
                  <div class="disc" [class.spinning]="isPlaying">
                    <div class="disc-label"></div>
                  </div>
                  <div class="needle"></div>
                </ng-template>
              </div>

              <div class="disc-copy">
                <div class="dc-eyebrow">Audio preview</div>
                <div class="dc-title" [title]="displayTitle">{{ displayTitle }}</div>
                <div class="dc-meta" *ngIf="displaySubtitle" [title]="displaySubtitle">{{ displaySubtitle }}</div>
                <div class="dc-status">{{ playbackLabel }}</div>
              </div>
            </div>

            <!-- right: signal panel -->
            <div class="signal-panel" #signalPanel>
              <div class="sig-header">
                <span class="sig-label">Spectrum</span>
                <span class="sig-sub">Direct stream · {{ typeBadge }}</span>
              </div>

              <div class="spectrum" [class.live]="isPlaying">
                <span
                  *ngFor="let bar of spectrumBars; let i = index; trackBy: trackByIndex"
                  class="s-bar"
                  [style.height.%]="bar"
                  [style.animation-delay.ms]="i * -120">
                </span>
              </div>

              <div class="freq-labels">
                <span>20Hz</span><span>200Hz</span><span>2kHz</span><span>10kHz</span><span>20kHz</span>
              </div>

              <div class="sig-header vu-header">
                <span class="sig-label">Level</span>
              </div>
              <div class="vu-strip">
                <span class="vu-channel">L</span>
                <div class="vu-track">
                  <div class="vu-fill" [class.vu-hot]="vuLeft > 80" [style.width.%]="vuLeft"></div>
                  <div class="vu-peak" [style.right.%]="100 - vuPeakLeft"></div>
                </div>
              </div>
              <div class="vu-strip">
                <span class="vu-channel">R</span>
                <div class="vu-track">
                  <div class="vu-fill" [class.vu-hot]="vuRight > 80" [style.width.%]="vuRight"></div>
                  <div class="vu-peak" [style.right.%]="100 - vuPeakRight"></div>
                </div>
              </div>

              <section class="av-tags av-tags-inline" *ngIf="audioTagRows.length > 0">
                <div class="av-tags-title">Audio tags</div>
                <div class="av-tags-grid">
                  <div class="av-tag-row" *ngFor="let tag of audioTagRows; trackBy: trackTagRow">
                    <span class="av-tag-key">{{ tag.label }}</span>
                    <span class="av-tag-value" [title]="tag.value">{{ tag.value }}</span>
                  </div>
                </div>
              </section>
            </div>
          </section>

          <!-- ── TRANSPORT CONTROLS ─────────────────────── -->
          <div class="ctrl-bar">

            <!-- progress row -->
            <div class="progress-row">
              <span class="time-edge">{{ formatTime(displayedCurrentTime) }}</span>
              <div class="prog-track">
                <div class="prog-fill" [style.width.%]="progressPercent"></div>
                <input
                  type="range"
                  class="seek-input"
                  min="0"
                  [max]="duration || 0"
                  step="any"
                  [value]="displayedCurrentTime"
                  [disabled]="!audioReady"
                  (mousedown)="onSeekStart()"
                  (touchstart)="onSeekStart()"
                  (input)="onSeekInput($event)"
                  (change)="onSeekCommit($event)">
              </div>
              <span class="time-edge time-edge--right">{{ formatTime(duration) }}</span>
            </div>

            <!-- buttons row -->
            <div class="controls-row">
              <button
                class="cbtn cbtn-play"
                type="button"
                [disabled]="!audioReady"
                (click)="togglePlayPause()"
                [attr.aria-label]="isPlaying ? 'Pause' : 'Play'"
                [attr.aria-pressed]="isPlaying">
                <span class="cbtn-icon">{{ isPlaying ? '⏸' : '▶' }}</span>
              </button>

              <button class="cbtn cbtn-skip" type="button" [disabled]="!audioReady" (click)="seekRelative(-10)">-10s</button>
              <button class="cbtn cbtn-skip" type="button" [disabled]="!audioReady" (click)="seekRelative(10)">+10s</button>

              <div class="time-display">
                <b class="time-cur">{{ formatTime(displayedCurrentTime) }}</b>
                <span class="time-sep">/</span>
                <span class="time-dur">{{ formatTime(duration) }}</span>
              </div>

              <div class="spacer"></div>

              <!-- volume -->
              <div class="volume-control">
                <button
                  class="volume-icon-btn"
                  type="button"
                  (click)="toggleMute()"
                  [attr.aria-label]="volume <= 0 ? 'Unmute' : 'Mute'"
                  [title]="volume <= 0 ? 'Unmute' : 'Mute'">
                  <span class="volume-icon">{{ volume <= 0 ? '🔇' : '🔊' }}</span>
                </button>
                <input
                  type="range"
                  class="volume-slider"
                  min="0"
                  max="100"
                  [value]="volume"
                  [disabled]="!audioReady"
                  (input)="onVolumeChange($event)">
              </div>
            </div>
          </div>

        </div><!-- /audio-player-shell -->
      </div><!-- /viewer-body -->
    </div><!-- /viewer-surface -->
  `,
  styles: [VIEWER_BASE_STYLES, `
    /* ── SURFACE ───────────────────────────────────────── */
    .viewer-surface {
      --av-surface: var(--vsc-editor-background, var(--vsc-editor-background));
      --av-surface-alt: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 92%, black);
      --av-surface-muted: var(--vsc-editor-background, var(--vsc-editor-background));
      --av-border: color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 70%, transparent);
      --av-text: var(--vsc-foreground, var(--vsc-foreground));
      --av-text-muted: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 58%, transparent);
      --av-text-dim: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 36%, transparent);
      --av-accent: var(--vsc-button-background, var(--vsc-button-background));
      --av-accent-foreground: var(--vsc-button-foreground, var(--vsc-on-accent));
      --av-focus: var(--vsc-focusBorder, var(--vsc-accent-blue));
      --av-shadow: var(--vsc-widget-shadow, 0 5px 8px 0 var(--vsc-shadow));
      --av-button: var(--vsc-button-secondary, var(--vsc-button-secondary));
      --av-button-hover: var(--vsc-button-secondary-hover, var(--vsc-control-hover-background));
      --av-list-hover: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
      --av-list-active: var(--vsc-list-active-background, var(--vsc-list-selected-emphasis));
      --av-list-active-foreground: var(--vsc-list-active-foreground, var(--vsc-on-accent));
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
      background: var(--av-surface);
      color: var(--av-text);
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    }

    :host-context(.vscode-light) .viewer-surface {
      --av-surface: var(--vsc-editor-background, var(--vsc-on-accent));
      --av-surface-alt: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-on-accent)) 94%, var(--vsc-panel-background, var(--vsc-panel-background)));
      --av-surface-muted: var(--vsc-editor-background, var(--vsc-on-accent));
      --av-border: color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 88%, transparent);
      --av-text: var(--vsc-foreground, var(--vsc-foreground));
      --av-text-muted: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 78%, white);
      --av-text-dim: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 64%, white);
      --av-button: color-mix(in srgb, var(--vsc-input-background, var(--vsc-on-accent)) 86%, var(--vsc-panel-background, var(--vsc-panel-background)));
      --av-button-hover: var(--vsc-list-hover-background, var(--vsc-foreground));
      --av-list-hover: var(--vsc-list-hover-background, var(--vsc-foreground));
      --av-list-active: color-mix(in srgb, var(--vsc-list-active-background, var(--vsc-list-active-background)) 84%, white);
      --av-list-active-foreground: var(--vsc-list-active-foreground, var(--vsc-media-background));
    }

    /* ── HEADER ────────────────────────────────────────── */
    .viewer-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      padding: 9px 14px;
      border-bottom: 1px solid var(--av-border);
      background: color-mix(in srgb, var(--av-surface) 92%, black);
      flex-shrink: 0;
      min-width: 0;
    }

    :host-context(.vscode-light) .viewer-header {
      background: color-mix(in srgb, var(--av-surface) 99%, var(--vsc-panel-background, var(--vsc-panel-background)));
    }

    .header-main {
      display: flex;
      align-items: center;
      gap: 10px;
      min-width: 0;
      flex: 1;
    }

    .hdr-icon {
      flex-shrink: 0;
      color: color-mix(in srgb, var(--av-accent) 78%, var(--av-text));
      font-size: 18px;
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
      color: var(--av-text);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .path {
      font-family: ui-monospace, 'Cascadia Code', 'Fira Code', monospace;
      font-size: 11px;
      color: var(--av-text-dim);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      margin-top: 1px;
    }

    :host-context(.vscode-light) .path {
      color: var(--av-text-dim);
    }

    .header-meta {
      display: flex;
      align-items: center;
      gap: 5px;
      flex-shrink: 0;
    }

    .badge {
      font-size: 10px;
      font-weight: 600;
      padding: 2px 7px;
      border-radius: 3px;
      white-space: nowrap;
      letter-spacing: 0.3px;
    }

    .type-badge {
      background: color-mix(in srgb, var(--av-accent) 16%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      border: 1px solid color-mix(in srgb, var(--av-accent) 40%, transparent);
    }

    .size-badge {
      background: color-mix(in srgb, var(--av-text) 7%, transparent);
      color: color-mix(in srgb, var(--av-text) 60%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 50%, transparent);
    }

    :host-context(.vscode-light) .size-badge,
    :host-context(.vscode-light) .mime-badge {
      background: color-mix(in srgb, var(--av-text) 6%, white);
      color: var(--av-text);
      border-color: color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 78%, transparent);
    }

    :host-context(.vscode-light) .dur-badge {
      color: color-mix(in srgb, var(--av-focus) 72%, var(--vsc-media-panel-background));
    }

    :host-context(.vscode-light) .type-badge {
      color: color-mix(in srgb, var(--av-accent) 78%, var(--vsc-media-panel-background));
    }

    .mime-badge {
      background: color-mix(in srgb, var(--av-text) 7%, transparent);
      color: color-mix(in srgb, var(--av-text) 60%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 50%, transparent);
    }

    .dur-badge {
      background: color-mix(in srgb, var(--av-focus) 12%, transparent);
      color: color-mix(in srgb, var(--av-focus) 64%, white);
      border: 1px solid color-mix(in srgb, var(--av-focus) 26%, transparent);
    }

    /* ── BODY ──────────────────────────────────────────── */
    .viewer-body {
      flex: 1 1 0%;
      min-height: 0;
      overflow: auto;
      position: relative;
    }

    /* ── STATE CONTAINERS ──────────────────────────────── */
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

    .state-icon-wrap {
      width: 48px;
      height: 48px;
      border-radius: 50%;
      background: color-mix(in srgb, var(--av-text) 10%, transparent);
      border: 1px solid var(--av-border);
      display: flex;
      align-items: center;
      justify-content: center;
      color: var(--av-text-dim);
    }

    .state-icon-wrap svg {
      width: 22px;
      height: 22px;
    }

    .state-icon-folder {
      background: color-mix(in srgb, var(--vsc-accent-yellow, var(--vsc-accent-yellow)) 14%, transparent);
      border-color: color-mix(in srgb, var(--vsc-accent-yellow, var(--vsc-accent-yellow)) 28%, transparent);
      color: var(--vsc-accent-yellow, var(--vsc-accent-yellow));
    }

    .state-icon-warn {
      background: color-mix(in srgb, var(--vsc-warning, var(--vsc-warning)) 14%, transparent);
      border-color: color-mix(in srgb, var(--vsc-warning, var(--vsc-warning)) 28%, transparent);
      color: var(--vsc-warning, var(--vsc-warning));
    }

    .state-icon-error {
      background: color-mix(in srgb, var(--vsc-error, var(--vsc-error)) 14%, transparent);
      border-color: color-mix(in srgb, var(--vsc-error, var(--vsc-error)) 28%, transparent);
      color: var(--vsc-error, var(--vsc-error));
    }

    .state-label {
      font-size: 13px;
      opacity: 0.65;
      color: inherit;
    }

    .av-tags {
      display: flex;
      flex-direction: column;
      gap: 10px;
      padding: 12px;
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 55%, transparent);
      border-radius: 8px;
      background: color-mix(in srgb, var(--vsc-editorWidget-background, var(--vsc-panel-background)) 92%, black);
    }

    .av-tags-inline {
      margin-top: 8px;
    }

    .av-tags-title {
      font-size: 11px;
      font-weight: 600;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 68%, transparent);
    }

    .av-tags-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: 8px;
    }

    .av-tag-row {
      display: flex;
      flex-direction: column;
      gap: 4px;
      min-width: 0;
      padding: 8px 10px;
      border-radius: 6px;
      background: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 6%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 38%, transparent);
    }

    .av-tag-key {
      font-size: 10px;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 56%, transparent);
    }

    .av-tag-value {
      font-size: 12px;
      color: var(--vsc-foreground, var(--vsc-foreground));
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    :host-context(.vscode-light) .state-label {
      opacity: 1;
      color: var(--av-text-muted);
    }

    .state-sub {
      font-size: 11px;
      color: var(--av-text-dim);
      font-family: ui-monospace, monospace;
    }

    .state-error .state-label {
      color: var(--vsc-error);
      opacity: 1;
    }

    .error-detail {
      font-size: 11px;
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      color: color-mix(in srgb, var(--vsc-error-bright) 70%, var(--vsc-foreground, var(--vsc-foreground)));
      background: var(--vsc-error-surface);
      border: 1px solid var(--vsc-error-border);
      border-radius: 4px;
      padding: 8px 12px;
      max-width: 480px;
      white-space: pre-wrap;
      word-break: break-word;
    }

    .spinner {
      width: 26px;
      height: 26px;
      border: 2px solid color-mix(in srgb, var(--av-accent) 24%, transparent);
      border-top-color: var(--av-accent);
      border-radius: 50%;
      animation: spin-basic 0.65s linear infinite;
    }

    @keyframes spin-basic {
      to { transform: rotate(360deg); }
    }

    /* ── PLAYER SHELL ──────────────────────────────────── */
    .native-audio {
      display: none;
    }

    .audio-player-shell {
      display: flex;
      flex-direction: column;
      min-height: 100%;
      background: var(--av-surface);
    }

    /* ── ART STAGE ─────────────────────────────────────── */
    .art-stage {
      display: grid;
      grid-template-columns: minmax(210px, 270px) 1fr;
      flex: 1 1 auto;
      min-height: 0;
    }

    /* ── DISC PANEL ────────────────────────────────────── */
    .disc-panel {
      background: linear-gradient(160deg, var(--av-surface-alt) 0%, var(--av-surface-muted) 100%);
      border-right: 1px solid var(--av-border);
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 18px;
      padding: 24px 20px;
      min-height: 230px;
    }

    .disc-wrap {
      position: relative;
      width: 148px;
      height: 148px;
      flex-shrink: 0;
    }

    .cover-art-frame {
      width: 148px;
      height: 148px;
      border-radius: 18px;
      overflow: hidden;
      border: 1px solid var(--vsc-neutral-overlay-medium);
      box-shadow:
        inset 0 0 0 1px var(--vsc-neutral-overlay-subtle),
        0 12px 32px var(--vsc-scrim);
      background: color-mix(in srgb, var(--av-surface-alt) 84%, black);
    }

    .cover-art-image {
      display: block;
      width: 100%;
      height: 100%;
      object-fit: cover;
    }

    .disc {
      width: 148px;
      height: 148px;
      border-radius: 50%;
      background: conic-gradient(
        var(--vsc-media-background) 0deg, var(--vsc-media-panel-background) 30deg, var(--vsc-media-background) 60deg, var(--vsc-media-panel-background) 90deg,
        var(--vsc-media-background) 120deg, var(--vsc-media-panel-background) 150deg, var(--vsc-media-background) 180deg, var(--vsc-media-panel-background) 210deg,
        var(--vsc-media-background) 240deg, var(--vsc-media-panel-background) 270deg, var(--vsc-media-background) 300deg, var(--vsc-media-panel-background) 330deg, var(--vsc-media-background) 360deg
      );
      border: 1px solid var(--vsc-neutral-overlay-soft);
      position: relative;
      box-shadow:
        inset 0 0 0 1px var(--vsc-neutral-overlay-subtle),
        0 12px 32px var(--vsc-media-overlay);
    }

    :host-context(.vscode-light) .disc-panel {
      background: linear-gradient(160deg, color-mix(in srgb, var(--av-surface-alt) 92%, white) 0%, var(--av-surface-muted) 100%);
    }

    .disc.spinning {
      animation: spin-disc 6s linear infinite;
    }

    @keyframes spin-disc {
      to { transform: rotate(360deg); }
    }

    /* grooves */
    .disc::before {
      content: '';
      position: absolute;
      inset: 14px;
      border-radius: 50%;
      border: 1px solid var(--vsc-neutral-overlay-subtle);
      box-shadow:
        0 0 0 12px var(--vsc-shadow-soft),
        0 0 0 24px var(--vsc-neutral-overlay-subtle),
        0 0 0 38px var(--vsc-shadow-soft);
    }

    /* label center */
    .disc-label {
      position: absolute;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
      width: 44px;
      height: 44px;
      border-radius: 50%;
      background: linear-gradient(135deg, color-mix(in srgb, var(--av-accent) 82%, white), var(--av-accent));
      border: 2px solid color-mix(in srgb, var(--av-accent-foreground) 18%, transparent);
      box-shadow:
        0 0 18px color-mix(in srgb, var(--av-accent) 40%, transparent),
        0 0 0 3px color-mix(in srgb, var(--av-accent) 12%, transparent);
      z-index: 2;
    }

    .disc-label::after {
      content: '';
      position: absolute;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: color-mix(in srgb, var(--vsc-foreground) 30%, transparent);
    }

    /* needle */
    .needle {
      position: absolute;
      top: -6px;
      right: -18px;
      width: 54px;
      height: 2px;
      background: linear-gradient(90deg, var(--vsc-media-control-muted), var(--vsc-neutral-overlay-medium));
      border-radius: 1px;
      transform-origin: right center;
      transform: rotate(-34deg);
      transition: transform 0.5s cubic-bezier(0.4, 0, 0.2, 1);
      z-index: 5;
    }

    .needle::before {
      content: '';
      position: absolute;
      right: -1px;
      top: 50%;
      transform: translateY(-50%);
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: color-mix(in srgb, var(--vsc-foreground) 30%, transparent);
      box-shadow: 0 0 6px var(--vsc-neutral-overlay-strong);
    }

    .disc-wrap.playing .needle {
      transform: rotate(-22deg);
    }

    .disc-copy {
      display: flex;
      flex-direction: column;
      gap: 5px;
      text-align: center;
      width: 100%;
    }

    .dc-meta {
      font-size: 11px;
      color: var(--av-text-muted);
      line-height: 1.35;
      word-break: break-word;
    }

    .dc-eyebrow {
      font-size: 9px;
      font-weight: 700;
      letter-spacing: 0.18em;
      text-transform: uppercase;
      color: color-mix(in srgb, var(--av-accent) 82%, var(--av-text));
    }

    .dc-title {
      font-size: 14px;
      font-weight: 700;
      color: var(--av-text);
      line-height: 1.3;
      word-break: break-word;
      overflow: hidden;
      text-overflow: ellipsis;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
    }

    .dc-status {
      font-size: 11px;
      color: var(--av-text-dim);
      margin-top: 2px;
    }

    /* ── SIGNAL PANEL ──────────────────────────────────── */
    .signal-panel {
      display: flex;
      flex-direction: column;
      background: var(--av-surface);
      padding: 16px 18px 14px;
      gap: 10px;
    }

    .sig-header {
      display: flex;
      align-items: baseline;
      justify-content: space-between;
      gap: 8px;
    }

    .vu-header {
      margin-top: 4px;
    }

    .sig-label {
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.15em;
      text-transform: uppercase;
      color: var(--av-text-muted);
    }

    .sig-sub {
      font-size: 10px;
      color: var(--av-text-dim);
      letter-spacing: 0.04em;
    }

    /* spectrum */
    .spectrum {
      display: flex;
      align-items: flex-end;
      gap: 3px;
      height: 80px;
      flex: 1;
      padding-bottom: 4px;
    }

    .s-bar {
      flex: 1;
      min-width: 5px;
      border-radius: 3px 3px 2px 2px;
      background: linear-gradient(180deg, color-mix(in srgb, var(--av-accent) 82%, white) 0%, color-mix(in srgb, var(--av-accent) 30%, transparent) 100%);
      transform-origin: bottom;
      animation: s-idle 1.6s ease-in-out infinite;
      animation-play-state: paused;
    }

    .spectrum.live .s-bar {
      animation-name: s-live;
      animation-play-state: running;
    }

    @keyframes s-idle {
      0%, 100% { transform: scaleY(0.28); }
      50% { transform: scaleY(0.42); }
    }

    @keyframes s-live {
      0%, 100% { transform: scaleY(0.32); }
      35% { transform: scaleY(1); }
      70% { transform: scaleY(0.55); }
    }

    .freq-labels {
      display: flex;
      justify-content: space-between;
      padding: 0 2px;
    }

    .freq-labels span {
      font-size: 9px;
      color: var(--av-text-dim);
      font-family: ui-monospace, monospace;
      letter-spacing: 0.02em;
    }

    /* VU meters */
    .vu-strip {
      display: flex;
      gap: 6px;
      align-items: center;
    }

    .vu-channel {
      font-size: 9px;
      font-weight: 700;
      color: var(--av-text-dim);
      letter-spacing: 0.1em;
      min-width: 10px;
      font-family: ui-monospace, monospace;
    }

    .vu-track {
      flex: 1;
      height: 6px;
      background: color-mix(in srgb, var(--av-text) 10%, transparent);
      border-radius: 3px;
      overflow: visible;
      position: relative;
    }

    .vu-fill {
      height: 100%;
      border-radius: 3px;
      width: 0%;
      transition: width 0.08s linear;
      background: linear-gradient(90deg, var(--av-accent) 0%, color-mix(in srgb, var(--av-accent) 72%, white) 100%);
    }

    .vu-fill.vu-hot {
      background: linear-gradient(90deg, var(--av-accent) 0%, var(--vsc-editorWarning-foreground, var(--vsc-warning)) 100%);
    }

    .vu-peak {
      position: absolute;
      top: 0;
      right: 0;
      width: 2px;
      height: 100%;
      background: color-mix(in srgb, var(--av-text) 58%, transparent);
      border-radius: 1px;
      transition: right 0.05s linear;
    }

    /* ── CONTROLS ──────────────────────────────────────── */
    .ctrl-bar {
      padding: 12px 14px 14px;
      background: color-mix(in srgb, var(--av-surface) 96%, black);
      border-top: 1px solid var(--av-border);
      display: flex;
      flex-direction: column;
      gap: 12px;
      flex-shrink: 0;
    }

    /* progress */
    .progress-row {
      display: flex;
      align-items: center;
      gap: 8px;
      min-height: 24px;
    }

    .time-edge {
      font-size: 10px;
      color: var(--av-text-muted);
      font-family: ui-monospace, monospace;
      white-space: nowrap;
      min-width: 34px;
    }

    .time-edge--right {
      text-align: right;
    }

    .prog-track {
      flex: 1;
      position: relative;
      height: 4px;
      background: color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 80%, transparent);
      border-radius: 2px;
      cursor: pointer;
      overflow: hidden;
    }

    .prog-fill {
      position: absolute;
      left: 0;
      top: 0;
      height: 100%;
      background: linear-gradient(90deg, var(--vsc-accent-blue, var(--vsc-accent-blue)), var(--vsc-info));
      border-radius: 2px;
      transition: width 0.05s linear;
      pointer-events: none;
    }

    .seek-input {
      position: absolute;
      width: 100%;
      height: 24px;
      width: 100%;
      margin: -10px 0;
      appearance: none;
      -webkit-appearance: none;
      background: transparent;
      outline: none;
      cursor: pointer;
      z-index: 2;
    }

    .seek-input::-webkit-slider-thumb {
      appearance: none;
      -webkit-appearance: none;
      width: 12px;
      height: 12px;
      background: var(--vsc-document-canvas-background);
      border: none;
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 2px 8px var(--vsc-scrim-strong);
      transition: all 0.2s ease;
    }

    .seek-input::-webkit-slider-thumb:hover {
      width: 16px;
      height: 16px;
      box-shadow: 0 4px 16px var(--vsc-accent-blue);
    }

    .seek-input::-moz-range-thumb {
      width: 12px;
      height: 12px;
      background: var(--vsc-document-canvas-background);
      border: none;
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 2px 8px var(--vsc-scrim-strong);
      transition: all 0.2s ease;
    }

    .seek-input::-moz-range-thumb:hover {
      width: 16px;
      height: 16px;
      box-shadow: 0 4px 16px var(--vsc-accent-blue);
    }

    /* controls row */
    .controls-row {
      display: flex;
      align-items: center;
      gap: 12px;
      min-width: 0;
      padding: 0 4px;
    }

    .spacer {
      flex: 1 1 auto;
    }

    .cbtn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 36px;
      height: 32px;
      padding: 0 10px;
      border-radius: 4px;
      border: 1px solid var(--av-border);
      background: var(--vsc-input-background, var(--av-button));
      color: var(--av-text);
      font-size: 13px;
      font-weight: 600;
      cursor: pointer;
      transition: background 0.12s, border-color 0.12s;
      white-space: nowrap;
      letter-spacing: 0.02em;
      font-family: inherit;
    }

    .cbtn:hover:not(:disabled) {
      background: var(--av-button-hover);
      border-color: color-mix(in srgb, var(--av-border) 100%, var(--av-text) 20%);
    }

    .cbtn:active:not(:disabled) {
      background: color-mix(in srgb, var(--av-button-hover) 90%, black);
    }

    .cbtn:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    .cbtn-play {
      min-width: 40px;
      height: 32px;
      background: var(--vsc-button-background, var(--vsc-button-background));
      border-color: color-mix(in srgb, var(--vsc-button-background, var(--vsc-button-background)) 60%, transparent);
      color: var(--vsc-button-foreground, var(--vsc-on-accent));
      font-size: 14px;
    }

    .cbtn-play:hover:not(:disabled) {
      background: var(--vsc-button-hover-background, var(--vsc-button-hover));
      border-color: color-mix(in srgb, var(--vsc-button-hover-background, var(--vsc-button-hover)) 70%, transparent);
    }

    .cbtn-icon {
      display: inline-block;
    }

    .cbtn-skip {
      min-width: 44px;
      font-size: 11px;
      font-family: ui-monospace, monospace;
    }

    .time-display {
      display: flex;
      align-items: center;
      gap: 4px;
      font-size: 12px;
      font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
      color: var(--av-text-muted);
      padding: 3px 8px;
      border-radius: 4px;
      border: 1px solid var(--av-border);
      background: var(--vsc-input-background, var(--av-button));
      min-width: 90px;
      justify-content: center;
      white-space: nowrap;
      font-variant-numeric: tabular-nums;
    }

    :host-context(.vscode-light) .time-display {
      background: var(--vsc-input-background, var(--vsc-on-accent));
    }

    .time-cur {
      color: var(--av-text);
      font-weight: 600;
    }

    .time-sep {
      opacity: 0.45;
    }

    .time-dur {
      color: var(--av-text-muted);
    }

    /* volume */
    .volume-control {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 0 4px;
    }

    .volume-icon-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 24px;
      height: 24px;
      padding: 0;
      border: none;
      background: transparent;
      color: inherit;
      cursor: pointer;
    }

    .volume-icon-btn:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    .volume-icon {
      display: inline-block;
      font-size: 16px;
      line-height: 1;
    }

    .volume-slider {
      width: 80px;
      height: 4px;
      border-radius: 2px;
      appearance: none;
      -webkit-appearance: none;
      background: color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 80%, transparent);
      outline: none;
      cursor: pointer;
    }

    .volume-slider::-webkit-slider-thumb {
      appearance: none;
      -webkit-appearance: none;
      width: 10px;
      height: 10px;
      border-radius: 50%;
      background: var(--vsc-document-canvas-background);
      box-shadow: 0 2px 6px var(--vsc-scrim-strong);
    }

    .volume-slider::-moz-range-thumb {
      width: 10px;
      height: 10px;
      border-radius: 50%;
      border: none;
      background: var(--vsc-document-canvas-background);
      box-shadow: 0 2px 6px var(--vsc-scrim-strong);
    }

    .volume-slider:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    :host-context(.vscode-light) .av-tags {
      background: color-mix(in srgb, var(--av-surface-alt) 96%, white);
      border-color: color-mix(in srgb, var(--av-border) 82%, transparent);
    }

    :host-context(.vscode-light) .av-tag-row {
      background: color-mix(in srgb, var(--av-text) 5%, white);
      border-color: color-mix(in srgb, var(--av-border) 74%, transparent);
    }

    :host-context(.vscode-light) .av-tags-title,
    :host-context(.vscode-light) .time-edge,
    :host-context(.vscode-light) .time-dur,
    :host-context(.vscode-light) .sig-label,
    :host-context(.vscode-light) .dc-status {
      color: var(--av-text-muted);
    }

    :host-context(.vscode-light) .sig-sub,
    :host-context(.vscode-light) .freq-labels span,
    :host-context(.vscode-light) .vu-channel,
    :host-context(.vscode-light) .av-tag-key,
    :host-context(.vscode-light) .path {
      color: var(--av-text-dim);
    }

    /* ── RESPONSIVE ─────────────────────────────────────── */
    @media (max-width: 820px) {
      .art-stage {
        grid-template-columns: 1fr;
      }

      .disc-panel {
        border-right: none;
        border-bottom: 1px solid var(--av-border);
        min-height: 200px;
        padding: 20px;
        flex-direction: row;
        gap: 20px;
        align-items: center;
        justify-content: flex-start;
      }

      .disc-wrap {
        width: 110px;
        height: 110px;
        flex-shrink: 0;
      }

      .cover-art-frame {
        width: 110px;
        height: 110px;
      }

      .disc {
        width: 110px;
        height: 110px;
      }

      .disc-copy {
        text-align: left;
      }
    }

    @media (max-width: 560px) {
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
        flex: 1;
        width: auto;
      }

      .time-display {
        min-width: 80px;
      }
    }
  `, VIEWER_PRODUCTION_CHROME_STYLES],
})
export class AudioViewerComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  @Input() fileList: ViewerFileListItem[] = [];
  @Input() probe: FileProbe | null = null;
  @Input() viewerId: string | null = null;

  @ViewChild('audioElement')
  set audioElementRef(value: ElementRef<HTMLAudioElement> | undefined) {
    const nextElement = value?.nativeElement ?? null;
    if (this.audioElement && this.audioElement !== nextElement) {
      this.destroyAudioAnalysis();
    }
    this.audioElement = nextElement;
    if (this.audioElement) {
      // Ensure audio element doesn't have its own volume control
      this.audioElement.volume = 1;
    }
    this.setupAudioAnalysis();
    this.syncAudioElement();
  }

  @ViewChild('signalPanel')
  set signalPanelRef(value: ElementRef<HTMLElement> | undefined) {
    this.signalPanelElement = value?.nativeElement ?? null;
    this.attachSignalPanelObserver();
  }

  kind: AudioViewerKind = 'empty';
  info: FileInfo | null = null;
  activeProbe: FileProbe | null = null;
  activeAudioTags: AudioViewerTags | null = null;
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

  // Live meter state (driven by Web Audio analyser data while playing)
  vuLeft = 0;
  vuRight = 0;
  vuPeakLeft = 0;
  vuPeakRight = 0;

  spectrumBars = new Array<number>(20).fill(0);

  private audioElement: HTMLAudioElement | null = null;
  private audioContext: AudioContext | null = null;
  private mediaSourceNode: MediaElementAudioSourceNode | null = null;
  private outputGainNode: GainNode | null = null;
  private splitterNode: ChannelSplitterNode | null = null;
  private spectrumAnalyser: AnalyserNode | null = null;
  private leftLevelAnalyser: AnalyserNode | null = null;
  private rightLevelAnalyser: AnalyserNode | null = null;
  private frequencyData: Uint8Array<ArrayBuffer> | null = null;
  private leftTimeDomainData: Uint8Array<ArrayBuffer> | null = null;
  private rightTimeDomainData: Uint8Array<ArrayBuffer> | null = null;
  private loadVersion = 0;
  private autoplayPending = false;
  private lastNonZeroVolume = 90;
  private activeTargetPath = '';
  private vuRafId: ReturnType<typeof requestAnimationFrame> | null = null;
  private vuPeakLeftTs = 0;
  private vuPeakRightTs = 0;
  private signalPanelElement: HTMLElement | null = null;
  private signalPanelObserver: IntersectionObserver | null = null;
  private signalPanelVisible = true;

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
    return this.resolveAudioMime(this.activeTargetPath, this.activeProbe) ?? '';
  }

  get coverArtDataUrl(): string | null {
    return this.activeAudioTags?.coverArtDataUrl ?? null;
  }

  get displayTitle(): string {
    return this.activeAudioTags?.title?.trim() || this.displayName || 'Unknown track';
  }

  get displaySubtitle(): string {
    const tags = this.activeAudioTags;
    if (!tags) {
      return '';
    }
    const parts = [tags.artist, tags.album].map(part => part?.trim() || '').filter(part => part.length > 0);
    return parts.join(' • ');
  }

  get audioTagRows(): AudioTagRow[] {
    const tags = this.activeAudioTags;
    if (!tags) {
      return [];
    }
    const rows: AudioTagRow[] = [
      { key: 'title', label: 'Title', value: tags.title ?? '' },
      { key: 'artist', label: 'Artist', value: tags.artist ?? '' },
      { key: 'album', label: 'Album', value: tags.album ?? '' },
      { key: 'albumArtist', label: 'Album artist', value: tags.albumArtist ?? '' },
      { key: 'genre', label: 'Genre', value: tags.genre ?? '' },
      { key: 'year', label: 'Year', value: tags.year ?? '' },
      { key: 'track', label: 'Track', value: tags.track ?? '' },
      { key: 'composer', label: 'Composer', value: tags.composer ?? '' },
    ];
    return rows.filter(tag => tag.value.trim().length > 0);
  }

  get displayedCurrentTime(): number {
    return this.isSeeking ? this.seekTarget : this.currentTime;
  }

  get progressPercent(): number {
    if (!this.duration || this.duration <= 0) { return 0; }
    return Math.max(0, Math.min(100, (this.displayedCurrentTime / this.duration) * 100));
  }

  get playbackLabel(): string {
    if (this.isBuffering) { return 'Buffering stream'; }
    if (!this.audioReady) { return 'Preparing playback'; }
    if (this.isPlaying) { return 'Playing now'; }
    if (this.currentTime > 0 && this.currentTime >= this.duration && this.duration > 0) { return 'Playback finished'; }
    if (this.currentTime > 0) { return 'Paused'; }
    return 'Ready to play';
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
      this.activeTargetPath = path;
      await this.loadFromDisk(path, currentLoad);
    } catch (err: unknown) {
      if (currentLoad !== this.loadVersion) { return; }
      this.error = err instanceof Error ? err.message : String(err);
      this.kind = 'error';
    }

    if (currentLoad === this.loadVersion) {
      this.cdr.markForCheck();
    }
  }

  ngOnDestroy(): void {
    this.stopVuLoop();
    this.detachSignalPanelObserver();
    this.pauseAndDetachAudio();
    this.destroyAudioAnalysis();
  }

  @HostListener('document:visibilitychange')
  onDocumentVisibilityChange(): void {
    this.updateVisualizerLoopState();
  }

  async togglePlayPause(): Promise<void> {
    const audio = this.audioElement;
    if (!audio || !this.audioReady) { return; }
    try {
      if (audio.paused) {
        // Resume audio context if suspended (browser autoplay policy)
        if (this.audioContext && this.audioContext.state === 'suspended') {
          await this.audioContext.resume();
        }
        this.isBuffering = true;
        this.cdr.markForCheck();
        await audio.play();
      } else {
        audio.pause();
      }
    } catch (err) {
      console.warn('Playback error:', err);
      this.isBuffering = false;
      this.cdr.markForCheck();
    }
  }

  seekRelative(deltaSeconds: number): void {
    const audio = this.audioElement;
    if (!audio || !this.audioReady) { return; }
    const nextTime = Math.max(0, Math.min(this.duration || audio.duration || 0, audio.currentTime + deltaSeconds));
    audio.currentTime = nextTime;
    this.currentTime = nextTime;
    this.seekTarget = nextTime;
    this.cdr.markForCheck();
  }

  onSeekStart(): void {
    if (!this.audioReady) { return; }
    this.isSeeking = true;
    this.seekTarget = this.currentTime;
    this.cdr.markForCheck();
  }

  onSeekInput(event: Event): void {
    if (!this.audioReady) { return; }
    this.seekTarget = Number((event.target as HTMLInputElement).value);
    this.cdr.markForCheck();
  }

  onSeekCommit(event: Event): void {
    const audio = this.audioElement;
    if (!audio || !this.audioReady) { return; }
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
    if (nextVolume > 0) { this.lastNonZeroVolume = nextVolume; }
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
    if (this.isSeeking) { return; }
    const audio = event.target as HTMLAudioElement;
    this.currentTime = Number.isFinite(audio.currentTime) ? audio.currentTime : 0;
    this.cdr.markForCheck();
  }

  onPlay(): void {
    this.isPlaying = true;
    this.isBuffering = false;
    this.setupAudioAnalysis();
    this.syncAudioElement();
    // Resume audio context if needed
    if (this.audioContext && this.audioContext.state === 'suspended') {
      void this.audioContext.resume();
    }
    this.updateVisualizerLoopState();
    this.cdr.markForCheck();
  }

  onPause(): void {
    this.isPlaying = false;
    this.isBuffering = false;
    this.updateVisualizerLoopState();
    this.drainVu();
    this.cdr.markForCheck();
  }

  onEnded(): void {
    this.isPlaying = false;
    this.isBuffering = false;
    this.currentTime = this.duration;
    this.seekTarget = this.duration;
    this.updateVisualizerLoopState();
    this.drainVu();
    this.cdr.markForCheck();
  }

  onWaiting(): void {
    if (this.audioReady) {
      this.isBuffering = true;
      this.cdr.markForCheck();
    }
  }

  onAudioError(): void {
    console.error('Audio error occurred');
    this.error = 'Audio stream failed to load.';
    this.kind = 'error';
    this.audioReady = false;
    this.isPlaying = false;
    this.isBuffering = false;
    this.updateVisualizerLoopState();
    this.cdr.markForCheck();
  }

  formatTime(seconds: number): string {
    if (!seconds || seconds < 0) { return '0:00'; }
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

  trackByIndex(index: number): number {
    return index;
  }

  trackTagRow(_index: number, item: AudioTagRow): string {
    return String(item.key);
  }

  // ── VU METER LOOP ──────────────────────────────────────

  private startVuLoop(): void {
    if (this.vuRafId !== null || !this.shouldAnimateVisualizers()) { return; }
    const tick = () => {
      if (!this.shouldAnimateVisualizers()) {
        this.vuRafId = null;
        return;
      }
      const lv = this.readLevelPercent(this.leftLevelAnalyser, this.leftTimeDomainData);
      const rv = this.readLevelPercent(this.rightLevelAnalyser, this.rightTimeDomainData);
      this.updateSpectrumBars();
      this.vuLeft = lv;
      this.vuRight = rv;
      const now = performance.now();
      if (lv > this.vuPeakLeft) { this.vuPeakLeft = lv; this.vuPeakLeftTs = now; }
      if (rv > this.vuPeakRight) { this.vuPeakRight = rv; this.vuPeakRightTs = now; }
      if (now - this.vuPeakLeftTs > 1200) { this.vuPeakLeft = Math.max(0, this.vuPeakLeft - 1.5); }
      if (now - this.vuPeakRightTs > 1200) { this.vuPeakRight = Math.max(0, this.vuPeakRight - 1.5); }
      this.cdr.markForCheck();
      this.vuRafId = requestAnimationFrame(tick);
    };
    this.vuRafId = requestAnimationFrame(tick);
  }

  private stopVuLoop(): void {
    if (this.vuRafId !== null) {
      cancelAnimationFrame(this.vuRafId);
      this.vuRafId = null;
    }
  }

  private drainVu(): void {
    if (!this.signalPanelVisible || document.hidden) {
      this.vuLeft = 0;
      this.vuRight = 0;
      this.spectrumBars = this.spectrumBars.map(() => 0);
      this.vuPeakLeft = 0;
      this.vuPeakRight = 0;
      this.cdr.markForCheck();
      return;
    }
    const step = () => {
      this.vuLeft = 0;
      this.vuRight = 0;
      this.spectrumBars = this.spectrumBars.map(() => 0);
      this.vuPeakLeft = Math.max(0, this.vuPeakLeft - 3);
      this.vuPeakRight = Math.max(0, this.vuPeakRight - 3);
      this.cdr.markForCheck();
      if (this.vuPeakLeft > 0 || this.vuPeakRight > 0) {
        requestAnimationFrame(step);
      }
    };
    requestAnimationFrame(step);
  }

  // ── PRIVATE HELPERS ────────────────────────────────────

  private async loadFromDisk(path: string, loadVersion: number): Promise<void> {
    const info = await this.api.getFileInfo(path);
    if (loadVersion !== this.loadVersion) { return; }
    this.activeTargetPath = path;
    this.info = info;
    this.activeAudioTags = null;

    let probe: FileProbe | null = path === this.targetPath ? this.probe : null;
    if (!probe) {
      try {
        probe = await this.api.probeFile(path);
      } catch {
        probe = null;
      }
    }
    if (loadVersion !== this.loadVersion) { return; }
    this.activeProbe = probe;

    if (info.type === FileType.DIRECTORY) {
      this.kind = 'directory';
      return;
    }

    if (!this.resolveAudioMime(path, probe)) {
      this.kind = 'unsupported';
      return;
    }

    this.streamUrl = await this.api.getStreamURL(path);
    if (loadVersion !== this.loadVersion) { return; }

    this.kind = 'ready';
    this.autoplayPending = true;
    void this.loadAudioTagsFromFile(path, info.size, probe, loadVersion);
  }

  private resolveAudioMime(path: string, probe: FileProbe | null): string | null {
    const normalizedProbeMime = (probe?.mimeType ?? '').split(';', 1)[0].trim().toLowerCase();
    if (normalizedProbeMime.startsWith('audio/')) { return normalizedProbeMime; }
    return inferAudioMime(extractExt(path));
  }

  private async loadAudioTagsFromFile(path: string, fileSize: number, probe: FileProbe | null, loadVersion: number): Promise<void> {
    const mimeType = (probe?.mimeType ?? '').split(';', 1)[0].trim().toLowerCase() || this.resolveAudioMime(path, probe) || 'audio/mpeg';
    const initialChunkSize = Math.min(fileSize || 0, 512 * 1024);
    if (initialChunkSize <= 0) {
      return;
    }

    try {
      const initialChunk = await this.api.readFileRange(path, 0, initialChunkSize);
      const initialBytes = this.base64ToUint8Array(initialChunk.content);
      const metadata = await parseBuffer(initialBytes, { mimeType, size: fileSize });
      if (loadVersion !== this.loadVersion) {
        return;
      }

      let tags = this.mapMetadataToAudioTags(metadata.common);
      if (!tags && fileSize > initialChunkSize && fileSize <= 16 * 1024 * 1024) {
        const fullFile = await this.api.readFile(path, 'base64');
        if (loadVersion !== this.loadVersion) {
          return;
        }
        const fullBytes = this.base64ToUint8Array(fullFile.content);
        const fullMetadata = await parseBuffer(fullBytes, { mimeType, size: fileSize });
        if (loadVersion !== this.loadVersion) {
          return;
        }
        tags = this.mapMetadataToAudioTags(fullMetadata.common);
      }

      this.activeAudioTags = tags;
      this.cdr.markForCheck();
    } catch (error) {
      console.debug('Audio tag parsing failed', error);
    }
  }

  private base64ToUint8Array(content: string): Uint8Array {
    const binary = atob(content);
    const bytes = new Uint8Array(binary.length);
    for (let index = 0; index < binary.length; index += 1) {
      bytes[index] = binary.charCodeAt(index);
    }
    return bytes;
  }

  private mapMetadataToAudioTags(common: {
    title?: string;
    artist?: string;
    album?: string;
    albumartist?: string;
    genre?: string[];
    year?: number;
    date?: string;
    composer?: string[];
    picture?: IPicture[];
    track?: { no?: number | null; of?: number | null };
  }): AudioViewerTags | null {
    const pictures = common.picture ?? [];
    const cover = pictures.length > 0 ? selectCover(pictures) : null;
    const tags: AudioViewerTags = {
      title: this.cleanTagText(common.title),
      artist: this.cleanTagText(common.artist),
      album: this.cleanTagText(common.album),
      albumArtist: this.cleanTagText(common.albumartist),
      genre: this.joinTagValues(common.genre),
      year: common.year ? String(common.year) : this.cleanTagText(common.date),
      track: this.formatTrack(common.track),
      composer: this.joinTagValues(common.composer),
      coverArtDataUrl: this.pictureToDataUrl(cover),
    };

    if (!tags.title && !tags.artist && !tags.album && !tags.albumArtist && !tags.genre && !tags.year && !tags.track && !tags.composer && !tags.coverArtDataUrl) {
      return null;
    }

    return tags;
  }

  private cleanTagText(value: string | null | undefined): string {
    return (value ?? '').trim();
  }

  private joinTagValues(values: string[] | null | undefined): string {
    if (!values?.length) {
      return '';
    }
    return values.map(value => value.trim()).filter(value => value.length > 0).join(', ');
  }

  private formatTrack(track: { no?: number | null; of?: number | null } | null | undefined): string {
    const number = track?.no ?? 0;
    const total = track?.of ?? 0;
    if (!number) {
      return '';
    }
    return total ? `${number}/${total}` : String(number);
  }

  private pictureToDataUrl(picture: IPicture | null | undefined): string {
    if (!picture?.data?.length || !picture.format) {
      return '';
    }

    const chunkSize = 0x8000;
    let binary = '';
    for (let offset = 0; offset < picture.data.length; offset += chunkSize) {
      const chunk = picture.data.subarray(offset, offset + chunkSize);
      binary += String.fromCharCode(...chunk);
    }

    return `data:${picture.format};base64,${btoa(binary)}`;
  }

  private syncAudioElement(): void {
    const audio = this.audioElement;
    if (!audio) { return; }
    // Set native audio volume to 1 (full) - we control volume via gain node
    audio.volume = 1;
    if (this.outputGainNode) {
      this.outputGainNode.gain.value = Math.max(0, Math.min(1, this.volume / 100));
    }
  }

  private shouldAnimateVisualizers(): boolean {
    return this.isPlaying && this.signalPanelVisible && !document.hidden;
  }

  private updateVisualizerLoopState(): void {
    if (this.shouldAnimateVisualizers()) {
      this.startVuLoop();
      return;
    }
    this.stopVuLoop();
  }

  private attachSignalPanelObserver(): void {
    this.detachSignalPanelObserver();
    const target = this.signalPanelElement;
    if (!target) {
      this.signalPanelVisible = true;
      this.updateVisualizerLoopState();
      return;
    }

    if (typeof IntersectionObserver === 'undefined') {
      this.signalPanelVisible = true;
      this.updateVisualizerLoopState();
      return;
    }

    this.signalPanelObserver = new IntersectionObserver((entries) => {
      const nextVisible = entries.some(entry => entry.isIntersecting && entry.intersectionRatio > 0);
      if (this.signalPanelVisible === nextVisible) {
        return;
      }
      this.signalPanelVisible = nextVisible;
      this.updateVisualizerLoopState();
    }, {
      threshold: 0.01,
    });
    this.signalPanelObserver.observe(target);
  }

  private detachSignalPanelObserver(): void {
    if (!this.signalPanelObserver) {
      return;
    }
    this.signalPanelObserver.disconnect();
    this.signalPanelObserver = null;
  }

  private async tryAutoplay(): Promise<void> {
    const audio = this.audioElement;
    if (!audio) { return; }
    try {
      // Resume audio context if needed
      if (this.audioContext && this.audioContext.state === 'suspended') {
        await this.audioContext.resume();
      }
      this.isBuffering = true;
      this.cdr.markForCheck();
      await audio.play();
    } catch (err) {
      console.warn('Autoplay failed:', err);
      this.isBuffering = false;
      this.cdr.markForCheck();
    }
  }

  private resetState(): void {
    this.stopVuLoop();
    this.pauseAndDetachAudio();
    this.kind = 'empty';
    this.info = null;
    this.activeProbe = null;
    this.activeAudioTags = null;
    this.error = '';
    this.streamUrl = null;
    this.audioReady = false;
    this.isPlaying = false;
    this.isBuffering = false;
    this.isSeeking = false;
    this.currentTime = 0;
    this.seekTarget = 0;
    this.duration = 0;
    this.vuLeft = 0;
    this.vuRight = 0;
    this.vuPeakLeft = 0;
    this.vuPeakRight = 0;
    this.spectrumBars = this.spectrumBars.map(() => 0);
    this.autoplayPending = false;
    this.activeTargetPath = '';
  }

  private pauseAndDetachAudio(): void {
    const audio = this.audioElement;
    if (!audio) { return; }
    audio.pause();
    // Don't remove src immediately, just pause
  }

  private setupAudioAnalysis(): void {
    const audio = this.audioElement;
    if (!audio || this.mediaSourceNode) { return; }

    const AudioContextCtor = window.AudioContext || (window as any).webkitAudioContext;
    if (!AudioContextCtor) { return; }

    try {
      const audioContext = new AudioContextCtor();
      const mediaSourceNode = audioContext.createMediaElementSource(audio);
      const outputGainNode = audioContext.createGain();
      outputGainNode.gain.value = this.volume / 100;
      
      // Connect: audio element -> media source -> gain -> destination
      mediaSourceNode.connect(outputGainNode);
      outputGainNode.connect(audioContext.destination);

      this.audioContext = audioContext;
      this.mediaSourceNode = mediaSourceNode;
      this.outputGainNode = outputGainNode;
      
      // Create analyser nodes for visualization
      const spectrumAnalyser = audioContext.createAnalyser();
      spectrumAnalyser.fftSize = 256;
      spectrumAnalyser.smoothingTimeConstant = 0.82;

      const leftLevelAnalyser = audioContext.createAnalyser();
      leftLevelAnalyser.fftSize = 2048;
      leftLevelAnalyser.smoothingTimeConstant = 0.7;

      const rightLevelAnalyser = audioContext.createAnalyser();
      rightLevelAnalyser.fftSize = 2048;
      rightLevelAnalyser.smoothingTimeConstant = 0.7;

      const splitterNode = audioContext.createChannelSplitter(2);

      // Connect gain to analysers
      outputGainNode.connect(spectrumAnalyser);
      outputGainNode.connect(splitterNode);
      splitterNode.connect(leftLevelAnalyser, 0);
      splitterNode.connect(rightLevelAnalyser, 1);

      this.spectrumAnalyser = spectrumAnalyser;
      this.leftLevelAnalyser = leftLevelAnalyser;
      this.rightLevelAnalyser = rightLevelAnalyser;
      this.splitterNode = splitterNode;
      this.frequencyData = new Uint8Array(spectrumAnalyser.frequencyBinCount);
      this.leftTimeDomainData = new Uint8Array(leftLevelAnalyser.fftSize);
      this.rightTimeDomainData = new Uint8Array(rightLevelAnalyser.fftSize);
    } catch (err) {
      console.warn('Failed to setup audio analysis:', err);
      this.destroyAudioAnalysis();
    }
  }

  private destroyAudioAnalysis(): void {
    try { 
      if (this.mediaSourceNode) {
        this.mediaSourceNode.disconnect();
      }
    } catch {}
    try { 
      if (this.outputGainNode) {
        this.outputGainNode.disconnect();
      }
    } catch {}
    try { 
      if (this.splitterNode) {
        this.splitterNode.disconnect();
      }
    } catch {}
    try { 
      if (this.spectrumAnalyser) {
        this.spectrumAnalyser.disconnect();
      }
    } catch {}
    try { 
      if (this.leftLevelAnalyser) {
        this.leftLevelAnalyser.disconnect();
      }
    } catch {}
    try { 
      if (this.rightLevelAnalyser) {
        this.rightLevelAnalyser.disconnect();
      }
    } catch {}
    try { 
      if (this.audioContext) {
        void this.audioContext.close();
      }
    } catch {}
    this.audioContext = null;
    this.mediaSourceNode = null;
    this.outputGainNode = null;
    this.splitterNode = null;
    this.spectrumAnalyser = null;
    this.leftLevelAnalyser = null;
    this.rightLevelAnalyser = null;
    this.frequencyData = null;
    this.leftTimeDomainData = null;
    this.rightTimeDomainData = null;
  }

  private updateSpectrumBars(): void {
    const analyser = this.spectrumAnalyser;
    const data = this.frequencyData;
    if (!analyser || !data) { return; }

    analyser.getByteFrequencyData(data);
    const bars = new Array<number>(this.spectrumBars.length).fill(0);
    const binsPerBar = Math.max(1, Math.floor(data.length / bars.length));

    for (let i = 0; i < bars.length; i += 1) {
      const start = i * binsPerBar;
      const end = i === bars.length - 1 ? data.length : Math.min(data.length, start + binsPerBar);
      let sum = 0;
      for (let j = start; j < end; j += 1) {
        sum += data[j];
      }
      const avg = end > start ? sum / (end - start) : 0;
      bars[i] = Math.max(6, Math.min(100, (avg / 255) * 100));
    }

    this.spectrumBars = bars;
  }

  private readLevelPercent(analyser: AnalyserNode | null, data: Uint8Array<ArrayBuffer> | null): number {
    if (!analyser || !data) { return 0; }
    analyser.getByteTimeDomainData(data);
    let sumSquares = 0;
    for (let i = 0; i < data.length; i += 1) {
      const normalized = (data[i] - 128) / 128;
      sumSquares += normalized * normalized;
    }
    const rms = Math.sqrt(sumSquares / data.length);
    return Math.max(0, Math.min(100, rms * 200));
  }
}