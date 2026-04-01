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
    <div class="av-root">

      <!-- HEADER -->
      <div class="av-header">
        <svg class="av-hdr-icon" viewBox="0 0 16 16" fill="currentColor">
          <circle cx="8" cy="8" r="3"/><path d="M11 5V2l3-1v3l-3 1z"/>
        </svg>
        <div class="av-hdr-main">
          <div class="av-hdr-text">
            <div class="av-hdr-title" [title]="displayName">{{ displayName || 'Audio' }}</div>
            <div class="av-hdr-path" [title]="displayPath" *ngIf="displayPath">{{ displayPath }}</div>
          </div>
        </div>
        <div class="av-hdr-meta" *ngIf="kind === 'ready'">
          <span class="av-badge av-badge-type">{{ typeBadge }}</span>
          <span class="av-badge av-badge-size" *ngIf="info?.size">{{ formatBytes(info!.size) }}</span>
          <span class="av-badge av-badge-duration" *ngIf="duration > 0">{{ formatTime(duration) }}</span>
        </div>
      </div>

      <!-- BODY -->
      <div class="av-body" appVscodeScrollbar>

        <!-- loading -->
        <div *ngIf="kind === 'loading'" class="av-state">
          <div class="av-spinner"></div>
          <span class="av-state-label">Loading audio...</span>
        </div>

        <!-- empty -->
        <div *ngIf="kind === 'empty'" class="av-state">
          <div class="av-state-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="4"/><path d="M16 8V4l5-2v4l-5 2z"/>
            </svg>
          </div>
          <span class="av-state-label">Select a file to preview</span>
        </div>

        <!-- directory -->
        <div *ngIf="kind === 'directory'" class="av-state">
          <div class="av-state-icon av-state-icon--warn">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
            </svg>
          </div>
          <span class="av-state-label">Directory вЂ” no preview</span>
        </div>

        <!-- unsupported -->
        <div *ngIf="kind === 'unsupported'" class="av-state">
          <div class="av-state-icon av-state-icon--warn">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
            </svg>
          </div>
          <span class="av-state-label">No audio preview for this file type</span>
          <code class="av-state-ext" *ngIf="fileExt">.{{ fileExt }}</code>
        </div>

        <!-- error -->
        <div *ngIf="kind === 'error'" class="av-state">
          <div class="av-state-icon av-state-icon--error">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
              <line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>
            </svg>
          </div>
          <span class="av-state-label av-state-label--error">Preview failed</span>
          <code class="av-error-detail">{{ error }}</code>
        </div>

        <!-- READY -->
        <div *ngIf="kind === 'ready'" class="av-player">

          <audio
            #audioElement
            class="av-audio-el"
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

          <!-- Mini Spectrum -->
          <div class="av-spectrum-min" [class.av-spectrum-min--live]="isPlaying">
            <span
              *ngFor="let bar of spectrumBars; let i = index; trackBy: trackByIndex"
              class="av-bar-min"
              [style.height.%]="bar"
              [style.animation-delay.ms]="i * -80"></span>
          </div>

          <!-- Progress -->
          <div class="av-progress-min">
            <div class="av-rail">
              <div class="av-rail-fill" [style.width.%]="progressPercent"></div>
              <input
                type="range"
                class="av-seek"
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
            <div class="av-time-min">
              <span class="av-time-cur">{{ formatTime(displayedCurrentTime) }}</span>
              <span class="av-time-sep">/</span>
              <span>{{ formatTime(duration) }}</span>
            </div>
          </div>

          <!-- Controls -->
          <div class="av-controls-min">
            <button
              class="av-btn av-btn-prev"
              type="button"
              [disabled]="!hasPrevious"
              [title]="'Previous track'"
              (click)="showPrevious()">
              <svg viewBox="0 0 24 24" fill="currentColor">
                <path d="M6 6h2v12H6zm3.5 6l8.5 6V6z"/>
              </svg>
            </button>

            <button
              class="av-btn-play-min"
              type="button"
              [disabled]="!audioReady"
              [title]="isPlaying ? 'Pause' : 'Play'"
              (click)="togglePlayPause()">
              <svg *ngIf="!isPlaying" viewBox="0 0 24 24" fill="currentColor">
                <path d="M8 6.5v11l9-5.5-9-5.5z"/>
              </svg>
              <svg *ngIf="isPlaying" viewBox="0 0 24 24" fill="currentColor">
                <path d="M8 6h3.5v12H8zM12.5 6H16v12h-3.5z"/>
              </svg>
            </button>

            <button
              class="av-btn av-btn-next"
              type="button"
              [disabled]="!hasNext"
              [title]="'Next track'"
              (click)="showNext()">
              <svg viewBox="0 0 24 24" fill="currentColor">
                <path d="M16 6h2v12h-2zM6 6v12l8.5-6z"/>
              </svg>
            </button>

            <div class="av-vol-min">
              <span class="av-vol-icon" [title]="'Volume: ' + volume + '%'">{{ volume === 0 ? '🔇' : '🔊' }}</span>
              <input
                type="range"
                class="av-vol-slider-min"
                min="0"
                max="100"
                [value]="volume"
                [disabled]="!audioReady"
                [title]="'Volume: ' + volume + '%'"
                (input)="onVolumeChange($event)">
            </div>
          </div>

          <!-- Compact Playlist -->
          <div class="av-pl-min" *ngIf="hasPlaylist">
            <div class="av-pl-item-min" *ngFor="let item of playlistItems; let idx = index; trackBy: trackPlaylistItem"
              [class.active]="idx === currentPlaylistIndex"
              (click)="selectPlaylistIndex(idx)">
              {{ idx + 1 }}. {{ item.name }}
            </div>
          </div>

        </div><!-- /av-player -->
      </div><!-- /av-body -->
    </div><!-- /av-root -->
  `,
  styles: [VIEWER_BASE_STYLES, `
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    .av-root {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-editor-background, #1e1e1e);
      color: var(--vsc-foreground, #cccccc);
    }

    .av-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 10px 12px;
      border-bottom: 1px solid var(--vsc-border, #454545);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 92%, black);
      flex-shrink: 0;
      min-width: 0;
    }

    .av-hdr-icon {
      width: 16px;
      height: 16px;
      flex-shrink: 0;
      color: var(--vsc-entry-audio, #00ff00);
    }

    .av-hdr-main {
      flex: 1;
      min-width: 0;
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .av-hdr-text {
      flex: 1;
      min-width: 0;
    }

    .av-hdr-title {
      font-size: 12px;
      font-weight: 600;
      color: var(--vsc-foreground, #cccccc);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      letter-spacing: 0.3px;
    }

    .av-hdr-path {
      font-size: 10px;
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 55%, transparent);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      margin-top: 2px;
    }

    .av-hdr-meta {
      display: flex;
      gap: 6px;
      flex-shrink: 0;
      align-items: center;
    }

    .av-badge {
      padding: 3px 8px;
      font-size: 9px;
      font-weight: 500;
      border-radius: 4px;
      border: 1px solid transparent;
      white-space: nowrap;
      letter-spacing: 0.3px;
      text-transform: uppercase;
    }

    .av-badge-type {
      background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 18%, transparent);
      border-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 32%, transparent);
      color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 90%, var(--vsc-foreground, #ccc));
    }

    .av-badge-size {
      background: color-mix(in srgb, var(--vsc-foreground, #ccc) 10%, transparent);
      border-color: color-mix(in srgb, var(--vsc-foreground, #ccc) 24%, transparent);
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 65%, transparent);
    }

    .av-badge-duration {
      background: color-mix(in srgb, var(--vsc-foreground, #ccc) 10%, transparent);
      border-color: color-mix(in srgb, var(--vsc-foreground, #ccc) 24%, transparent);
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 65%, transparent);
    }

    .av-body {
      flex: 1 1 0%;
      min-height: 0;
      overflow: auto;
      display: flex;
      flex-direction: column;
      position: relative;
    }

    .av-state {
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

    .av-state-icon {
      width: 48px;
      height: 48px;
      border-radius: 50%;
      border: 1px solid var(--vsc-border, #454545);
      background: color-mix(in srgb, var(--vsc-foreground, #ccc) 8%, transparent);
      display: flex;
      align-items: center;
      justify-content: center;
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 60%, transparent);
    }

    .av-state-icon svg { width: 22px; height: 22px; }

    .av-state-icon--warn {
      background: color-mix(in srgb, var(--vsc-editorWarning-foreground, #b89500) 14%, transparent);
      border-color: color-mix(in srgb, var(--vsc-editorWarning-foreground, #b89500) 34%, transparent);
      color: var(--vsc-editorWarning-foreground, #8a6d00);
    }

    .av-state-icon--error {
      background: color-mix(in srgb, var(--vsc-editorError-foreground, #f48771) 14%, transparent);
      border-color: color-mix(in srgb, var(--vsc-editorError-foreground, #f48771) 34%, transparent);
      color: var(--vsc-editorError-foreground, #f48771);
    }

    .av-state-label {
      font-size: 13px;
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 65%, transparent);
    }

    .av-state-label--error {
      color: var(--vsc-editorError-foreground, #f48771);
    }

    .av-state-ext {
      font-family: var(--vsc-font-family-mono, 'Courier New', monospace);
      font-size: 11px;
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 55%, transparent);
    }

    .av-error-detail {
      font-family: var(--vsc-font-family-mono, 'Courier New', monospace);
      font-size: 11px;
      color: color-mix(in srgb, var(--vsc-editorError-foreground, #f48771) 80%, var(--vsc-foreground, #ccc));
      background: color-mix(in srgb, var(--vsc-editorError-foreground, #f48771) 8%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-editorError-foreground, #f48771) 20%, transparent);
      border-radius: 4px;
      padding: 8px 12px;
      max-width: 480px;
      white-space: pre-wrap;
      word-break: break-word;
    }

    .av-spinner {
      width: 24px;
      height: 24px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 25%, transparent);
      border-top-color: var(--vsc-accent-blue, #0078d4);
      border-radius: 50%;
      animation: av-spin 0.7s linear infinite;
    }

    @keyframes av-spin {
      to { transform: rotate(360deg); }
    }

    .av-audio-el { display: none; }

    .av-player {
      display: flex;
      flex-direction: column;
      gap: 16px;
      padding: 16px;
      flex: 1 1 0%;
      overflow-y: auto;
      max-width: 860px;
      margin: 0 auto;
      width: 100%;
    }

    /* Spectrum Visualization */
    .av-spectrum-min {
      display: flex;
      align-items: flex-end;
      gap: 2px;
      height: 48px;
      padding: 0 4px;
      background: color-mix(in srgb, black 94%, var(--vsc-editor-background, #1e1e1e));
      border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 55%, transparent);
      border-radius: 6px;
      overflow: hidden;
    }

    .av-bar-min {
      flex: 1;
      min-width: 3px;
      border-radius: 1px 1px 0 0;
      background: linear-gradient(to top, var(--vsc-entry-audio, #00ff00), color-mix(in srgb, var(--vsc-entry-audio, #00ff00) 56%, transparent));
      opacity: 0.5;
      transform-origin: bottom;
      animation: av-bar-idle-min 1.8s ease-in-out infinite;
      animation-play-state: paused;
    }

    .av-spectrum-min--live .av-bar-min {
      animation-name: av-bar-live-min;
      animation-play-state: running;
      opacity: 0.9;
    }

    @keyframes av-bar-idle-min {
      0%, 100% { transform: scaleY(0.2); }
      50% { transform: scaleY(0.35); }
    }

    @keyframes av-bar-live-min {
      0%, 100% { transform: scaleY(0.3); }
      40% { transform: scaleY(1); }
      70% { transform: scaleY(0.55); }
    }

    /* Progress & Time */
    .av-progress-min {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .av-rail {
      position: relative;
      height: 5px;
      border-radius: 999px;
      background: color-mix(in srgb, var(--vsc-border, #454545) 90%, transparent);
      overflow: hidden;
    }

    .av-rail-fill {
      position: absolute;
      inset: 0 auto 0 0;
      height: 100%;
      background: linear-gradient(90deg, var(--vsc-entry-audio, #00ff00), color-mix(in srgb, var(--vsc-entry-audio, #00ff00) 70%, transparent));
      border-radius: inherit;
      transition: width 70ms linear;
      pointer-events: none;
    }

    .av-seek {
      position: absolute;
      inset: -8px 0;
      width: 100%;
      margin: 0;
      appearance: none;
      -webkit-appearance: none;
      background: transparent;
      outline: none;
      cursor: pointer;
      z-index: 2;
    }

    .av-seek::-webkit-slider-thumb {
      appearance: none;
      -webkit-appearance: none;
      width: 12px;
      height: 12px;
      border-radius: 50%;
      background: #fff;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.5);
      cursor: pointer;
      transition: all 0.2s ease;
    }

    .av-seek::-webkit-slider-thumb:hover {
      width: 16px;
      height: 16px;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.6);
    }

    .av-seek::-moz-range-thumb {
      width: 12px;
      height: 12px;
      border: none;
      border-radius: 50%;
      background: #fff;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.5);
      cursor: pointer;
      transition: all 0.2s ease;
    }

    .av-seek::-moz-range-thumb:hover {
      width: 16px;
      height: 16px;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.6);
    }

    .av-seek:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .av-time-min {
      display: flex;
      align-items: center;
      gap: 4px;
      font-size: 12px;
      color: #ccc;
      font-variant-numeric: tabular-nums;
      font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
      padding: 4px 8px;
      background: rgba(0, 0, 0, 0.2);
      border-radius: 4px;
      min-width: 90px;
      justify-content: center;
    }

    .av-time-cur {
      color: white;
      font-weight: 500;
    }

    .av-time-sep { opacity: 0.5; }

    /* Controls */
    .av-controls-min {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 12px;
      background: rgba(0, 0, 0, 0.3);
      backdrop-filter: blur(8px);
      border: 1px solid rgba(255, 255, 255, 0.1);
      border-radius: 8px;
    }

    .av-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 36px;
      height: 36px;
      padding: 0 8px;
      background: rgba(255, 255, 255, 0.1);
      border: 1px solid rgba(255, 255, 255, 0.15);
      color: white;
      border-radius: 6px;
      cursor: pointer;
      flex-shrink: 0;
      transition: all 0.2s ease;
    }

    .av-btn:hover:not(:disabled) {
      background: rgba(255, 255, 255, 0.2);
      border-color: rgba(255, 255, 255, 0.3);
      transform: scale(1.05);
    }

    .av-btn:active:not(:disabled) {
      transform: scale(0.95);
    }

    .av-btn:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    .av-btn svg { width: 16px; height: 16px; }

    .av-btn-play-min {
      min-width: 44px;
      height: 36px;
      background: var(--vsc-accent-blue, #0078d4);
      border: 1px solid rgba(0, 120, 212, 0.3);
      color: white;
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: pointer;
      flex-shrink: 0;
      transition: all 0.2s ease;
      font-weight: 600;
      border-radius: 6px;
      padding: 0;
    }

    .av-btn-play-min:hover:not(:disabled) {
      background: #1084d7;
      box-shadow: 0 0 12px rgba(0, 120, 212, 0.5);
      transform: scale(1.05);
    }

    .av-btn-play-min:active:not(:disabled) {
      transform: scale(0.95);
    }

    .av-btn-play-min:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    .av-btn-play-min svg { width: 18px; height: 18px; }

    .av-vol-min {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-left: auto;
      padding: 0 4px;
    }

    .av-vol-icon {
      display: inline-block;
      font-size: 16px;
      line-height: 1;
      min-width: 16px;
      text-align: center;
    }

    .av-vol-slider-min {
      width: 80px;
      height: 4px;
      cursor: pointer;
      appearance: none;
      -webkit-appearance: none;
      background: rgba(255, 255, 255, 0.2);
      border-radius: 2px;
      outline: none;
    }

    .av-vol-slider-min::-webkit-slider-thumb {
      appearance: none;
      -webkit-appearance: none;
      width: 10px;
      height: 10px;
      background: white;
      border-radius: 50%;
      cursor: pointer;
      transition: all 0.2s ease;
    }

    .av-vol-slider-min::-webkit-slider-thumb:hover {
      width: 12px;
      height: 12px;
    }

    .av-vol-slider-min::-moz-range-thumb {
      width: 10px;
      height: 10px;
      border: none;
      border-radius: 50%;
      background: white;
      cursor: pointer;
      transition: all 0.2s ease;
    }

    .av-vol-slider-min::-moz-range-thumb:hover {
      width: 12px;
      height: 12px;
    }

    .av-vol-slider-min:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    /* Playlist */
    .av-pl-min {
      display: flex;
      flex-direction: column;
      gap: 1px;
      max-height: 240px;
      overflow-y: auto;
      border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 55%, transparent);
      border-radius: 6px;
      background: color-mix(in srgb, var(--vsc-editorWidget-background, #252526) 94%, black);
      padding: 4px;
    }

    .av-pl-item-min {
      padding: 6px 10px;
      font-size: 12px;
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 65%, transparent);
      border-radius: 4px;
      cursor: pointer;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      transition: all 0.15s ease;
      border: 1px solid transparent;
    }

    .av-pl-item-min:hover {
      background: var(--vsc-list-hover-background, rgba(255, 255, 255, 0.08));
      color: var(--vsc-foreground, #cccccc);
    }

    .av-pl-item-min.active {
      background: var(--vsc-list-active-background, rgba(0, 120, 212, 0.5));
      color: var(--vsc-list-active-foreground, #ffffff);
      border-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 40%, transparent);
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

  spectrumBars = new Array<number>(40).fill(0);

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

  // в”Ђв”Ђ VU METER LOOP в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

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

  // в”Ђв”Ђ PRIVATE HELPERS в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

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
