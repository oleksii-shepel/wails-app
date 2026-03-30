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

      <!-- ── HEADER ─────────────────────────────────────── -->
      <div class="viewer-header">
        <div class="header-main">
          <div class="hdr-icon">
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="8" cy="8" r="3"/>
              <path d="M11 5V2l3-1v3l-3 1z"/>
            </svg>
          </div>
          <div class="header-text">
            <div class="title" [title]="displayName">{{ displayName || 'Audio Preview' }}</div>
            <div class="path" [title]="displayPath">{{ displayPath || 'Select a file to preview' }}</div>
          </div>
        </div>

        <div class="header-meta" *ngIf="kind !== 'empty' && kind !== 'loading' && info">
          <span class="badge type-badge">{{ typeBadge }}</span>
          <span class="badge size-badge" *ngIf="info.size > 0">{{ formatBytes(info.size) }}</span>
          <span class="badge mime-badge" *ngIf="mimeLabel">{{ mimeLabel }}</span>
          <span class="badge dur-badge" *ngIf="duration > 0">{{ formatTime(duration) }}</span>
        </div>
      </div>

      <!-- ── BODY ───────────────────────────────────────── -->
      <div class="viewer-body" appVscodeScrollbar>

        <!-- loading -->
        <div *ngIf="kind === 'loading'" class="state-container">
          <div class="spinner"></div>
          <div class="state-label">Loading audio preview...</div>
        </div>

        <!-- empty -->
        <div *ngIf="kind === 'empty'" class="state-container">
          <div class="state-icon-wrap">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="4"/><path d="M16 8V4l5-2v4l-5 2z"/>
            </svg>
          </div>
          <div class="state-label">Select a file to preview</div>
        </div>

        <!-- directory -->
        <div *ngIf="kind === 'directory'" class="state-container">
          <div class="state-icon-wrap state-icon-folder">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
            </svg>
          </div>
          <div class="state-label">Directory — no preview available</div>
        </div>

        <!-- unsupported -->
        <div *ngIf="kind === 'unsupported'" class="state-container">
          <div class="state-icon-wrap state-icon-warn">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
            </svg>
          </div>
          <div class="state-label">No audio preview for this file type</div>
          <div class="state-sub" *ngIf="fileExt">.{{ fileExt }}</div>
        </div>

        <!-- error -->
        <div *ngIf="kind === 'error'" class="state-container state-error">
          <div class="state-icon-wrap state-icon-error">
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
              <div class="disc-wrap" [class.playing]="isPlaying">
                <div class="disc" [class.spinning]="isPlaying">
                  <div class="disc-label"></div>
                </div>
                <div class="needle"></div>
              </div>

              <div class="disc-copy">
                <div class="dc-eyebrow">Audio preview</div>
                <div class="dc-title" [title]="displayName">{{ displayName || 'Unknown track' }}</div>
                <div class="dc-status">{{ playbackLabel }}</div>
              </div>
            </div>

            <!-- right: signal panel -->
            <div class="signal-panel">
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
            </div>
          </section>

          <!-- ── TRANSPORT CONTROLS ─────────────────────── -->
          <div class="ctrl-bar">

            <!-- progress row -->
            <div class="progress-row">
              <span class="time-edge">{{ formatTime(displayedCurrentTime) }}</span>
              <div class="prog-track">
                <div class="prog-fill" [style.width.%]="progressPercent"></div>
                <div class="prog-thumb" [style.left.%]="progressPercent"></div>
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
                (click)="togglePlayPause()">
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

              <!-- playlist nav (compact, only when playlist exists) -->
              <div class="playlist-nav" *ngIf="hasPlaylist">
                <button class="cbtn cbtn-nav" type="button" [disabled]="!hasPrevious" (click)="showPrevious()" title="Previous">
                  <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="10 12 6 8 10 4"/>
                  </svg>
                </button>
                <span class="pl-counter">{{ currentPlaylistIndex + 1 }}/{{ playlistItems.length }}</span>
                <button class="cbtn cbtn-nav" type="button" [disabled]="!hasNext" (click)="showNext()" title="Next">
                  <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="6 4 10 8 6 12"/>
                  </svg>
                </button>
              </div>

              <!-- volume -->
              <div class="vol-wrap">
                <button
                  class="cbtn cbtn-vol-icon"
                  type="button"
                  (click)="toggleMute()"
                  [attr.aria-label]="volume <= 0 ? 'Unmute' : 'Mute'">
                  <svg *ngIf="volume <= 0" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M9 5v6l-4-3H3V8h2L9 5z"/><line x1="12" y1="5" x2="15" y2="8"/><line x1="15" y1="5" x2="12" y2="8"/>
                  </svg>
                  <svg *ngIf="volume > 0 && volume < 50" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M9 5v6l-4-3H3V8h2L9 5z"/><path d="M11.5 6.5a2.5 2.5 0 0 1 0 3"/>
                  </svg>
                  <svg *ngIf="volume >= 50" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M9 5v6l-4-3H3V8h2L9 5z"/><path d="M11.5 6.5a2.5 2.5 0 0 1 0 3"/><path d="M13.5 4.5a5 5 0 0 1 0 7"/>
                  </svg>
                </button>
                <input
                  type="range"
                  class="vol-slider"
                  min="0"
                  max="100"
                  [value]="volume"
                  [disabled]="!audioReady"
                  (input)="onVolumeChange($event)">
              </div>
            </div>
          </div>

          <!-- ── PLAYLIST ────────────────────────────────── -->
          <section class="playlist-panel" *ngIf="hasPlaylist">
            <div class="playlist-header">
              <span class="playlist-title">Playlist</span>
              <span class="playlist-count">{{ playlistItems.length }} tracks</span>
            </div>
            <div class="playlist-list">
              <button
                *ngFor="let item of playlistItems; let idx = index; trackBy: trackPlaylistItem"
                class="playlist-item"
                type="button"
                [class.active]="idx === currentPlaylistIndex"
                (click)="selectPlaylistIndex(idx)">
                <span class="pl-dot" *ngIf="idx === currentPlaylistIndex"></span>
                <span class="pl-num" *ngIf="idx !== currentPlaylistIndex">{{ (idx + 1).toString().padStart(2, '0') }}</span>
                <span class="pl-name" [title]="item.name">{{ item.name }}</span>
              </button>
            </div>
          </section>

        </div><!-- /audio-player-shell -->
      </div><!-- /viewer-body -->
    </div><!-- /viewer-surface -->
  `,
  styles: [VIEWER_BASE_STYLES, `
    /* ── SURFACE ───────────────────────────────────────── */
    .viewer-surface {
      --av-surface: var(--vsc-panel-background, #252526);
      --av-surface-alt: color-mix(in srgb, var(--vsc-panel-background, #252526) 88%, var(--vsc-editor-background, #1e1e1e));
      --av-surface-muted: color-mix(in srgb, var(--vsc-panel-background, #252526) 72%, black);
      --av-border: color-mix(in srgb, var(--vsc-border, #454545) 70%, transparent);
      --av-text: var(--vsc-foreground, #cccccc);
      --av-text-muted: color-mix(in srgb, var(--vsc-foreground, #cccccc) 58%, transparent);
      --av-text-dim: color-mix(in srgb, var(--vsc-foreground, #cccccc) 36%, transparent);
      --av-accent: var(--vsc-button-background, #0e639c);
      --av-accent-foreground: var(--vsc-button-foreground, #ffffff);
      --av-focus: var(--vsc-focusBorder, #007fd4);
      --av-shadow: var(--vsc-widget-shadow, 0 5px 8px 0 rgba(0, 0, 0, 0.36));
      --av-button: var(--vsc-button-secondary, #3a3d41);
      --av-button-hover: var(--vsc-button-secondary-hover, #45494e);
      --av-list-hover: var(--vsc-list-hover-background, #2a2d2e);
      --av-list-active: var(--vsc-list-active-background, #094771);
      --av-list-active-foreground: var(--vsc-list-active-foreground, #ffffff);
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
      background: var(--av-surface-muted);
      color: var(--av-text);
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    }

    /* ── HEADER ────────────────────────────────────────── */
    .viewer-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      padding: 9px 14px;
      border-bottom: 1px solid var(--av-border);
      background: var(--av-surface-muted);
      flex-shrink: 0;
      min-width: 0;
    }

    .header-main {
      display: flex;
      align-items: center;
      gap: 10px;
      min-width: 0;
      flex: 1;
    }

    .hdr-icon {
      width: 30px;
      height: 30px;
      border-radius: 50%;
      background: color-mix(in srgb, var(--av-accent) 16%, transparent);
      border: 1px solid color-mix(in srgb, var(--av-accent) 32%, transparent);
      display: flex;
      align-items: center;
      justify-content: center;
      flex-shrink: 0;
      color: color-mix(in srgb, var(--av-accent) 70%, white);
    }

    .hdr-icon svg {
      width: 14px;
      height: 14px;
    }

    .header-text {
      min-width: 0;
      flex: 1;
    }

    .title {
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 0.3px;
      color: color-mix(in srgb, var(--av-text) 90%, white);
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
      border-radius: 4px;
      white-space: nowrap;
      letter-spacing: 0.3px;
    }

    .type-badge {
      background: color-mix(in srgb, var(--av-accent) 16%, transparent);
      color: color-mix(in srgb, var(--av-accent) 70%, white);
      border: 1px solid color-mix(in srgb, var(--av-accent) 34%, transparent);
    }

    .size-badge {
      background: color-mix(in srgb, var(--av-text) 7%, transparent);
      color: var(--av-text-muted);
      border: 1px solid var(--av-border);
    }

    .mime-badge {
      background: color-mix(in srgb, var(--av-text) 7%, transparent);
      color: var(--av-text-muted);
      border: 1px solid var(--av-border);
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
      min-height: 200px;
      gap: 10px;
      padding: 32px;
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
      background: color-mix(in srgb, var(--vsc-accent-yellow, #d7ba7d) 14%, transparent);
      border-color: color-mix(in srgb, var(--vsc-accent-yellow, #d7ba7d) 28%, transparent);
      color: var(--vsc-accent-yellow, #d7ba7d);
    }

    .state-icon-warn {
      background: color-mix(in srgb, var(--vsc-warning, #cca700) 14%, transparent);
      border-color: color-mix(in srgb, var(--vsc-warning, #cca700) 28%, transparent);
      color: var(--vsc-warning, #cca700);
    }

    .state-icon-error {
      background: color-mix(in srgb, var(--vsc-error, #f48771) 14%, transparent);
      border-color: color-mix(in srgb, var(--vsc-error, #f48771) 28%, transparent);
      color: var(--vsc-error, #f48771);
    }

    .state-label {
      font-size: 13px;
      color: var(--av-text-muted);
    }

    .state-sub {
      font-size: 11px;
      color: var(--av-text-dim);
      font-family: ui-monospace, monospace;
    }

    .state-error .state-label {
      color: var(--vsc-error, #f48771);
    }

    .error-detail {
      font-size: 11px;
      font-family: ui-monospace, monospace;
      color: color-mix(in srgb, var(--vsc-error, #f48771) 82%, var(--av-text));
      background: color-mix(in srgb, var(--vsc-error, #f48771) 10%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-error, #f48771) 24%, transparent);
      border-radius: 6px;
      padding: 8px 14px;
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
      background: var(--av-surface-muted);
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

    .disc {
      width: 148px;
      height: 148px;
      border-radius: 50%;
      background: conic-gradient(
        #1a1a22 0deg, #222230 30deg, #1a1a22 60deg, #222230 90deg,
        #1a1a22 120deg, #222230 150deg, #1a1a22 180deg, #222230 210deg,
        #1a1a22 240deg, #222230 270deg, #1a1a22 300deg, #222230 330deg, #1a1a22 360deg
      );
      border: 1px solid rgba(255, 255, 255, 0.1);
      position: relative;
      box-shadow:
        inset 0 0 0 1px rgba(255, 255, 255, 0.04),
        0 12px 32px rgba(0, 0, 0, 0.6);
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
      border: 1px solid rgba(255, 255, 255, 0.05);
      box-shadow:
        0 0 0 12px rgba(0, 0, 0, 0.15),
        0 0 0 24px rgba(255, 255, 255, 0.025),
        0 0 0 38px rgba(0, 0, 0, 0.1);
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
      background: rgba(255, 255, 255, 0.9);
    }

    /* needle */
    .needle {
      position: absolute;
      top: -6px;
      right: -18px;
      width: 54px;
      height: 2px;
      background: linear-gradient(90deg, rgba(255, 255, 255, 0.55), rgba(255, 255, 255, 0.2));
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
      background: rgba(255, 255, 255, 0.75);
      box-shadow: 0 0 6px rgba(255, 255, 255, 0.3);
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

    .dc-eyebrow {
      font-size: 9px;
      font-weight: 700;
      letter-spacing: 0.18em;
      text-transform: uppercase;
      color: color-mix(in srgb, var(--av-accent) 72%, white);
    }

    .dc-title {
      font-size: 14px;
      font-weight: 700;
      color: color-mix(in srgb, var(--av-text) 92%, white);
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
      color: color-mix(in srgb, var(--av-text) 22%, transparent);
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
      color: color-mix(in srgb, var(--av-text) 28%, transparent);
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
      background: linear-gradient(90deg, var(--av-accent) 0%, var(--vsc-editorWarning-foreground, #b89500) 100%);
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
      padding: 11px 14px 13px;
      background: var(--av-surface-alt);
      border-top: 1px solid var(--av-border);
      display: flex;
      flex-direction: column;
      gap: 10px;
      flex-shrink: 0;
    }

    /* progress */
    .progress-row {
      display: flex;
      align-items: center;
      gap: 10px;
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
      background: color-mix(in srgb, var(--av-text) 14%, transparent);
      border-radius: 2px;
      cursor: pointer;
    }

    .prog-fill {
      position: absolute;
      left: 0;
      top: 0;
      height: 100%;
      background: linear-gradient(90deg, var(--av-accent), color-mix(in srgb, var(--av-accent) 72%, white));
      border-radius: 2px;
      transition: width 0.06s linear;
      pointer-events: none;
    }

    .prog-thumb {
      position: absolute;
      top: 50%;
      width: 12px;
      height: 12px;
      border-radius: 50%;
      background: var(--av-accent-foreground);
      border: 2px solid var(--av-accent);
      transform: translate(-50%, -50%);
      box-shadow: 0 0 8px color-mix(in srgb, var(--av-accent) 38%, transparent);
      transition: left 0.06s linear;
      pointer-events: none;
    }

    .seek-input {
      position: absolute;
      inset: -10px 0;
      width: 100%;
      opacity: 0;
      cursor: pointer;
      z-index: 2;
      margin: 0;
    }

    /* controls row */
    .controls-row {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
    }

    .spacer {
      flex: 1 1 auto;
    }

    .cbtn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 36px;
      height: 36px;
      padding: 0 10px;
      border-radius: 8px;
      border: 1px solid var(--av-border);
      background: var(--av-button);
      color: var(--av-text);
      font-size: 12px;
      font-weight: 600;
      cursor: pointer;
      transition: background 0.15s, transform 0.1s;
      white-space: nowrap;
      letter-spacing: 0.02em;
      font-family: inherit;
    }

    .cbtn:hover:not(:disabled) {
      background: var(--av-button-hover);
      border-color: color-mix(in srgb, var(--av-text) 24%, var(--av-border));
    }

    .cbtn:active:not(:disabled) {
      transform: scale(0.94);
    }

    .cbtn:disabled {
      opacity: 0.35;
      cursor: not-allowed;
    }

    .cbtn-play {
      min-width: 44px;
      height: 40px;
      background: var(--av-accent);
      border: 1px solid color-mix(in srgb, var(--av-accent) 72%, black);
      border-radius: 10px;
      font-size: 16px;
      box-shadow: 0 0 0 1px color-mix(in srgb, var(--av-accent) 16%, transparent);
      color: var(--av-accent-foreground);
    }

    .cbtn-play:hover:not(:disabled) {
      background: var(--vsc-button-hover, #1177bb);
      border-color: color-mix(in srgb, var(--vsc-button-hover, #1177bb) 72%, black);
      box-shadow: 0 0 0 1px color-mix(in srgb, var(--vsc-button-hover, #1177bb) 16%, transparent);
    }

    .cbtn-icon {
      display: inline-block;
    }

    .cbtn-skip {
      min-width: 40px;
      font-size: 11px;
      font-family: ui-monospace, monospace;
    }

    .cbtn-nav {
      min-width: 28px;
      height: 28px;
      padding: 0 6px;
      border-radius: 6px;
    }

    .cbtn-nav svg {
      width: 14px;
      height: 14px;
    }

    .cbtn-vol-icon {
      min-width: 32px;
      height: 32px;
      padding: 0 6px;
      border: none;
      background: transparent;
      color: var(--av-text-muted);
    }

    .cbtn-vol-icon svg {
      width: 16px;
      height: 16px;
    }

    .cbtn-vol-icon:hover:not(:disabled) {
      background: color-mix(in srgb, var(--av-text) 10%, transparent);
      color: color-mix(in srgb, var(--av-text) 86%, transparent);
    }

    .time-display {
      font-size: 11px;
      font-family: ui-monospace, monospace;
      color: var(--av-text-muted);
      padding: 4px 8px;
      border-radius: 5px;
      background: color-mix(in srgb, var(--av-text) 7%, transparent);
      min-width: 88px;
      text-align: center;
      white-space: nowrap;
    }

    .time-cur {
      color: color-mix(in srgb, var(--av-text) 90%, white);
      font-weight: 600;
    }

    .time-sep {
      opacity: 0.45;
      margin: 0 2px;
    }

    .time-dur {
      color: var(--av-text-muted);
    }

    /* playlist nav */
    .playlist-nav {
      display: flex;
      align-items: center;
      gap: 4px;
    }

    .pl-counter {
      font-size: 10px;
      font-weight: 600;
      color: var(--av-text-muted);
      letter-spacing: 0.06em;
      min-width: 28px;
      text-align: center;
      font-family: ui-monospace, monospace;
    }

    /* volume */
    .vol-wrap {
      display: flex;
      align-items: center;
      gap: 4px;
    }

    .vol-slider {
      width: 72px;
      height: 4px;
      border-radius: 2px;
      appearance: none;
      -webkit-appearance: none;
      background: color-mix(in srgb, var(--av-text) 18%, transparent);
      outline: none;
      cursor: pointer;
    }

    .vol-slider::-webkit-slider-thumb {
      appearance: none;
      -webkit-appearance: none;
      width: 10px;
      height: 10px;
      border-radius: 50%;
      background: var(--av-accent-foreground);
      box-shadow: 0 1px 4px rgba(0, 0, 0, 0.5);
    }

    .vol-slider::-moz-range-thumb {
      width: 10px;
      height: 10px;
      border-radius: 50%;
      border: none;
      background: var(--av-accent-foreground);
      box-shadow: 0 1px 4px rgba(0, 0, 0, 0.5);
    }

    .vol-slider:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    /* ── PLAYLIST ───────────────────────────────────────── */
    .playlist-panel {
      border-top: 1px solid var(--av-border);
      background: var(--av-surface-alt);
      padding: 12px 14px 14px;
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .playlist-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
    }

    .playlist-title {
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.14em;
      text-transform: uppercase;
      color: var(--av-text-muted);
    }

    .playlist-count {
      font-size: 10px;
      color: var(--av-text-dim);
    }

    .playlist-list {
      display: flex;
      flex-direction: column;
      gap: 4px;
      max-height: min(320px, 36vh);
      overflow-y: auto;
      overflow-x: hidden;
      overscroll-behavior: contain;
      scrollbar-gutter: stable;
    }

    .playlist-item {
      display: flex;
      align-items: center;
      gap: 10px;
      width: 100%;
      padding: 8px 10px;
      border-radius: 8px;
      border: 1px solid transparent;
      background: transparent;
      color: var(--av-text-muted);
      cursor: pointer;
      text-align: left;
      font-size: 12px;
      font-family: inherit;
      transition: background 0.14s, border-color 0.14s;
    }

    .playlist-item:hover {
      background: var(--av-list-hover);
      border-color: var(--av-border);
    }

    .playlist-item.active {
      background: var(--av-list-active);
      border-color: color-mix(in srgb, var(--av-accent) 36%, transparent);
      color: var(--av-list-active-foreground);
    }

    .pl-dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: var(--av-accent);
      flex-shrink: 0;
      animation: pl-blink 1s ease-in-out infinite;
    }

    @keyframes pl-blink {
      0%, 100% { opacity: 1; }
      50% { opacity: 0.3; }
    }

    .pl-num {
      font-size: 10px;
      font-weight: 600;
      min-width: 18px;
      color: var(--av-text-dim);
      font-family: ui-monospace, monospace;
    }

    .pl-name {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
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

      .vol-wrap {
        width: 100%;
      }

      .vol-slider {
        flex: 1;
        width: auto;
      }

      .time-display {
        min-width: 80px;
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

  // Live meter state (driven by Web Audio analyser data while playing)
  vuLeft = 0;
  vuRight = 0;
  vuPeakLeft = 0;
  vuPeakRight = 0;

  playlistItems: AudioPlaylistItem[] = [];
  currentPlaylistIndex = 0;

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

  get hasPlaylist(): boolean { return this.playlistItems.length > 1; }
  get hasPrevious(): boolean { return this.currentPlaylistIndex > 0; }
  get hasNext(): boolean { return this.currentPlaylistIndex >= 0 && this.currentPlaylistIndex < this.playlistItems.length - 1; }

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
    this.pauseAndDetachAudio();
    this.destroyAudioAnalysis();
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
    this.startVuLoop();
    this.cdr.markForCheck();
  }

  onPause(): void {
    this.isPlaying = false;
    this.isBuffering = false;
    this.stopVuLoop();
    this.drainVu();
    this.cdr.markForCheck();
  }

  onEnded(): void {
    this.isPlaying = false;
    this.isBuffering = false;
    this.currentTime = this.duration;
    this.seekTarget = this.duration;
    this.stopVuLoop();
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
    this.stopVuLoop();
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

  showPrevious(): void {
    if (!this.hasPrevious) { return; }
    void this.displayPlaylistIndex(this.currentPlaylistIndex - 1);
  }

  showNext(): void {
    if (!this.hasNext) { return; }
    void this.displayPlaylistIndex(this.currentPlaylistIndex + 1);
  }

  selectPlaylistIndex(index: number): void {
    if (index === this.currentPlaylistIndex) { return; }
    void this.displayPlaylistIndex(index);
  }

  trackPlaylistItem(_index: number, item: AudioPlaylistItem): string {
    return item.path;
  }

  // ── VU METER LOOP ──────────────────────────────────────

  private startVuLoop(): void {
    if (this.vuRafId !== null) { return; }
    const tick = () => {
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

    if (info.type === FileType.DIRECTORY) {
      this.kind = 'directory';
      return;
    }

    if (!this.resolveAudioMime(path, path === this.targetPath)) {
      this.kind = 'unsupported';
      return;
    }

    this.streamUrl = await this.api.getStreamURL(path);
    if (loadVersion !== this.loadVersion) { return; }

    this.kind = 'ready';
    this.autoplayPending = true;
  }

  private resolveAudioMime(path: string, allowProbe: boolean): string | null {
    const normalizedProbeMime = allowProbe
      ? (this.probe?.mimeType ?? '').split(';', 1)[0].trim().toLowerCase()
      : '';
    if (normalizedProbeMime.startsWith('audio/')) { return normalizedProbeMime; }
    return inferAudioMime(extractExt(path));
  }

  private preparePlaylist(path: string): { items: AudioPlaylistItem[]; index: number } {
    const normalizedTarget = (path ?? '').trim();
    const items = this.fileList
      .filter(item => item.path && this.isAudioCandidate(item.path))
      .map(item => ({ path: item.path, name: item.name || this.baseName(item.path) }));

    if (!items.length) {
      return { items: [{ path: normalizedTarget, name: this.baseName(normalizedTarget) }], index: 0 };
    }

    const currentIndex = items.findIndex(item => item.path === normalizedTarget);
    if (currentIndex >= 0) { return { items, index: currentIndex }; }

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
    if (!item) { return; }

    const currentLoad = ++this.loadVersion;
    this.stopVuLoop();
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
    this.vuLeft = 0;
    this.vuRight = 0;
    this.vuPeakLeft = 0;
    this.vuPeakRight = 0;
    this.spectrumBars = this.spectrumBars.map(() => 0);
    this.autoplayPending = false;
    this.activeTargetPath = item.path;
    this.cdr.markForCheck();

    try {
      await this.loadFromDisk(item.path, currentLoad);
    } catch (err: unknown) {
      if (currentLoad !== this.loadVersion) { return; }
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
    if (!audio) { return; }
    // Set native audio volume to 1 (full) - we control volume via gain node
    audio.volume = 1;
    if (this.outputGainNode) {
      this.outputGainNode.gain.value = Math.max(0, Math.min(1, this.volume / 100));
    }
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
    this.autoplayPending = false;
    this.playlistItems = [];
    this.currentPlaylistIndex = 0;
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
