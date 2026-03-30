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
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
      background: #0c0c0f;
      color: #cccccc;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    }

    /* ── HEADER ────────────────────────────────────────── */
    .viewer-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      padding: 9px 14px;
      border-bottom: 1px solid rgba(255, 255, 255, 0.07);
      background: #0c0c0f;
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
      background: rgba(99, 102, 241, 0.18);
      border: 1px solid rgba(99, 102, 241, 0.32);
      display: flex;
      align-items: center;
      justify-content: center;
      flex-shrink: 0;
      color: rgba(165, 163, 255, 0.9);
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
      color: rgba(255, 255, 255, 0.88);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .path {
      font-family: ui-monospace, 'Cascadia Code', 'Fira Code', monospace;
      font-size: 11px;
      color: rgba(255, 255, 255, 0.35);
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
      background: rgba(99, 102, 241, 0.2);
      color: #a5a3ff;
      border: 1px solid rgba(99, 102, 241, 0.35);
    }

    .size-badge {
      background: rgba(255, 255, 255, 0.06);
      color: rgba(255, 255, 255, 0.4);
      border: 1px solid rgba(255, 255, 255, 0.1);
    }

    .mime-badge {
      background: rgba(255, 255, 255, 0.06);
      color: rgba(255, 255, 255, 0.4);
      border: 1px solid rgba(255, 255, 255, 0.1);
    }

    .dur-badge {
      background: rgba(16, 185, 129, 0.14);
      color: #6ee7b7;
      border: 1px solid rgba(16, 185, 129, 0.28);
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
      background: rgba(255, 255, 255, 0.06);
      border: 1px solid rgba(255, 255, 255, 0.1);
      display: flex;
      align-items: center;
      justify-content: center;
      color: rgba(255, 255, 255, 0.35);
    }

    .state-icon-wrap svg {
      width: 22px;
      height: 22px;
    }

    .state-icon-folder {
      background: rgba(250, 204, 21, 0.1);
      border-color: rgba(250, 204, 21, 0.25);
      color: rgba(250, 204, 21, 0.7);
    }

    .state-icon-warn {
      background: rgba(99, 102, 241, 0.1);
      border-color: rgba(99, 102, 241, 0.25);
      color: rgba(165, 163, 255, 0.8);
    }

    .state-icon-error {
      background: rgba(239, 68, 68, 0.1);
      border-color: rgba(239, 68, 68, 0.25);
      color: rgba(252, 165, 165, 0.9);
    }

    .state-label {
      font-size: 13px;
      color: rgba(255, 255, 255, 0.45);
    }

    .state-sub {
      font-size: 11px;
      color: rgba(255, 255, 255, 0.3);
      font-family: ui-monospace, monospace;
    }

    .state-error .state-label {
      color: rgba(252, 165, 165, 0.9);
    }

    .error-detail {
      font-size: 11px;
      font-family: ui-monospace, monospace;
      color: rgba(252, 165, 165, 0.7);
      background: rgba(239, 68, 68, 0.07);
      border: 1px solid rgba(239, 68, 68, 0.2);
      border-radius: 6px;
      padding: 8px 14px;
      max-width: 480px;
      white-space: pre-wrap;
      word-break: break-word;
    }

    .spinner {
      width: 26px;
      height: 26px;
      border: 2px solid rgba(99, 102, 241, 0.25);
      border-top-color: #6366f1;
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
      background: #0c0c0f;
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
      background: linear-gradient(160deg, #111116 0%, #0f0f14 100%);
      border-right: 1px solid rgba(255, 255, 255, 0.06);
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
      background: linear-gradient(135deg, #6366f1, #4f46e5);
      border: 2px solid rgba(255, 255, 255, 0.15);
      box-shadow:
        0 0 18px rgba(99, 102, 241, 0.5),
        0 0 0 3px rgba(99, 102, 241, 0.12);
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
      color: rgba(99, 102, 241, 0.85);
    }

    .dc-title {
      font-size: 14px;
      font-weight: 700;
      color: rgba(255, 255, 255, 0.9);
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
      color: rgba(255, 255, 255, 0.35);
      margin-top: 2px;
    }

    /* ── SIGNAL PANEL ──────────────────────────────────── */
    .signal-panel {
      display: flex;
      flex-direction: column;
      background: #0d0d12;
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
      color: rgba(255, 255, 255, 0.45);
    }

    .sig-sub {
      font-size: 10px;
      color: rgba(255, 255, 255, 0.22);
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
      background: linear-gradient(180deg, #6366f1 0%, rgba(99, 102, 241, 0.3) 100%);
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
      color: rgba(255, 255, 255, 0.18);
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
      color: rgba(255, 255, 255, 0.25);
      letter-spacing: 0.1em;
      min-width: 10px;
      font-family: ui-monospace, monospace;
    }

    .vu-track {
      flex: 1;
      height: 6px;
      background: rgba(255, 255, 255, 0.07);
      border-radius: 3px;
      overflow: visible;
      position: relative;
    }

    .vu-fill {
      height: 100%;
      border-radius: 3px;
      width: 0%;
      transition: width 0.08s linear;
      background: linear-gradient(90deg, #6366f1 0%, #a78bfa 75%, #f472b6 100%);
    }

    .vu-fill.vu-hot {
      background: linear-gradient(90deg, #6366f1 0%, #f472b6 60%, #f87171 100%);
    }

    .vu-peak {
      position: absolute;
      top: 0;
      right: 0;
      width: 2px;
      height: 100%;
      background: rgba(255, 255, 255, 0.55);
      border-radius: 1px;
      transition: right 0.05s linear;
    }

    /* ── CONTROLS ──────────────────────────────────────── */
    .ctrl-bar {
      padding: 11px 14px 13px;
      background: #0a0a0e;
      border-top: 1px solid rgba(255, 255, 255, 0.07);
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
      color: rgba(255, 255, 255, 0.38);
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
      background: rgba(255, 255, 255, 0.12);
      border-radius: 2px;
      cursor: pointer;
    }

    .prog-fill {
      position: absolute;
      left: 0;
      top: 0;
      height: 100%;
      background: linear-gradient(90deg, #6366f1, #a78bfa);
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
      background: #fff;
      border: 2px solid #6366f1;
      transform: translate(-50%, -50%);
      box-shadow: 0 0 8px rgba(99, 102, 241, 0.6);
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
      border: 1px solid rgba(255, 255, 255, 0.12);
      background: rgba(255, 255, 255, 0.06);
      color: rgba(255, 255, 255, 0.8);
      font-size: 12px;
      font-weight: 600;
      cursor: pointer;
      transition: background 0.15s, transform 0.1s;
      white-space: nowrap;
      letter-spacing: 0.02em;
      font-family: inherit;
    }

    .cbtn:hover:not(:disabled) {
      background: rgba(255, 255, 255, 0.13);
      border-color: rgba(255, 255, 255, 0.22);
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
      background: linear-gradient(135deg, #6366f1, #4f46e5);
      border: 1px solid rgba(99, 102, 241, 0.4);
      border-radius: 10px;
      font-size: 16px;
      box-shadow: 0 4px 16px rgba(99, 102, 241, 0.3);
    }

    .cbtn-play:hover:not(:disabled) {
      background: linear-gradient(135deg, #7173f3, #5855e8);
      box-shadow: 0 6px 22px rgba(99, 102, 241, 0.45);
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
      color: rgba(255, 255, 255, 0.55);
    }

    .cbtn-vol-icon svg {
      width: 16px;
      height: 16px;
    }

    .cbtn-vol-icon:hover:not(:disabled) {
      background: rgba(255, 255, 255, 0.08);
      color: rgba(255, 255, 255, 0.8);
    }

    .time-display {
      font-size: 11px;
      font-family: ui-monospace, monospace;
      color: rgba(255, 255, 255, 0.45);
      padding: 4px 8px;
      border-radius: 5px;
      background: rgba(255, 255, 255, 0.05);
      min-width: 88px;
      text-align: center;
      white-space: nowrap;
    }

    .time-cur {
      color: rgba(255, 255, 255, 0.88);
      font-weight: 600;
    }

    .time-sep {
      opacity: 0.45;
      margin: 0 2px;
    }

    .time-dur {
      color: rgba(255, 255, 255, 0.4);
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
      color: rgba(255, 255, 255, 0.38);
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
      background: rgba(255, 255, 255, 0.15);
      outline: none;
      cursor: pointer;
    }

    .vol-slider::-webkit-slider-thumb {
      appearance: none;
      -webkit-appearance: none;
      width: 10px;
      height: 10px;
      border-radius: 50%;
      background: #fff;
      box-shadow: 0 1px 4px rgba(0, 0, 0, 0.5);
    }

    .vol-slider::-moz-range-thumb {
      width: 10px;
      height: 10px;
      border-radius: 50%;
      border: none;
      background: #fff;
      box-shadow: 0 1px 4px rgba(0, 0, 0, 0.5);
    }

    .vol-slider:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    /* ── PLAYLIST ───────────────────────────────────────── */
    .playlist-panel {
      border-top: 1px solid rgba(255, 255, 255, 0.06);
      background: #0a0a0e;
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
      color: rgba(99, 102, 241, 0.8);
    }

    .playlist-count {
      font-size: 10px;
      color: rgba(255, 255, 255, 0.28);
    }

    .playlist-list {
      display: flex;
      flex-direction: column;
      gap: 4px;
      max-height: 220px;
      overflow: auto;
    }

    .playlist-item {
      display: flex;
      align-items: center;
      gap: 10px;
      width: 100%;
      padding: 8px 10px;
      border-radius: 8px;
      border: 1px solid rgba(255, 255, 255, 0.06);
      background: rgba(255, 255, 255, 0.03);
      color: rgba(255, 255, 255, 0.6);
      cursor: pointer;
      text-align: left;
      font-size: 12px;
      font-family: inherit;
      transition: background 0.14s, border-color 0.14s;
    }

    .playlist-item:hover {
      background: rgba(255, 255, 255, 0.07);
      border-color: rgba(255, 255, 255, 0.12);
    }

    .playlist-item.active {
      background: rgba(99, 102, 241, 0.14);
      border-color: rgba(99, 102, 241, 0.4);
      color: rgba(255, 255, 255, 0.9);
    }

    .pl-dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: #6366f1;
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
      color: rgba(255, 255, 255, 0.25);
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
        border-bottom: 1px solid rgba(255, 255, 255, 0.06);
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

  // VU meter state (driven by a rAF loop when playing)
  vuLeft = 0;
  vuRight = 0;
  vuPeakLeft = 0;
  vuPeakRight = 0;

  playlistItems: AudioPlaylistItem[] = [];
  currentPlaylistIndex = 0;

  readonly spectrumBars = [28, 55, 42, 72, 38, 88, 50, 80, 44, 66, 36, 82, 52, 70, 40, 76, 46, 60, 32, 90];

  private audioElement: HTMLAudioElement | null = null;
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
  }

  async togglePlayPause(): Promise<void> {
    const audio = this.audioElement;
    if (!audio || !this.audioReady) { return; }
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
      const lv = (0.35 + Math.random() * 0.6) * 100;
      const rv = (0.35 + Math.random() * 0.55) * 100;
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
    audio.volume = this.volume / 100;
  }

  private async tryAutoplay(): Promise<void> {
    const audio = this.audioElement;
    if (!audio) { return; }
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
    audio.removeAttribute('src');
    audio.load();
  }
}