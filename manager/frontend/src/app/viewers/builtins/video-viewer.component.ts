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
import { WorkspaceService } from '../../services/workspace.service';
import { ViewerSurfaceComponent } from '../../surfaces/viewer/viewer-surface.component';
import type { ViewerFileListItem } from '../../shared/protocol-enhanced';
import { ViewerArchiveContext } from '../viewer-archive-context';
import { VIEWER_BASE_STYLES, VIEWER_PRODUCTION_CHROME_STYLES } from './builtin-viewer.shared';
import { SegmentVideoLoader } from './segment-video-loader';
import type { FileProbe } from '../../shared/protocol';

type Unsubscribable = { unsubscribe(): void };

interface VideoNavItem {
  path: string;
  name: string;
  thumbSrc: string | null;
  thumbLoading: boolean;
  thumbFailed: boolean;
}

@Component({
  selector: 'app-video-viewer',
  standalone: true,
  imports: [CommonModule, ViewerSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-viewer-surface
      *ngIf="useLegacySurface"
      [targetPath]="targetPath"
      [fileList]="fileList"
      [archiveContext]="archiveContext"
      [viewerId]="viewerId">
    </app-viewer-surface>

    <div *ngIf="!useLegacySurface" class="viewer-surface">
      <div class="viewer-body">
        <div *ngIf="loading" class="state-container" role="status">
          <div class="spinner"></div>
          <div class="state-label">Loading segmented preview...</div>
        </div>

        <div *ngIf="error" class="state-container state-error" role="alert">
          <div class="state-icon" aria-hidden="true">⚠</div>
          <div class="state-label">Preview failed</div>
          <div class="error-detail">{{ error }}</div>
        </div>

        <div *ngIf="!loading && !error" class="video-player-wrapper">
          <div
            class="video-stage"
            (mousemove)="onVideoMouseMove()"
            (mouseleave)="onVideoMouseLeave()">
            <div
              #videoContainer
              class="canvas-video-container"
              [class.playing]="videoIsPlaying">
            </div>
            
            <!-- Previous Video Navigation -->
            <button
              *ngIf="previousVideo"
              type="button"
              class="video-nav-hotspot video-nav-hotspot--prev"
              [attr.aria-label]="'Open previous video: ' + previousVideo.name"
              (click)="navigateToSibling(previousVideo, $event)">
              <div class="video-nav-card">
                <div class="video-nav-thumb-wrapper">
                  <div class="video-nav-thumb" [class.video-nav-thumb--empty]="!previousVideo.thumbSrc || previousVideo.thumbFailed">
                    <img
                      *ngIf="previousVideo.thumbSrc && !previousVideo.thumbFailed"
                      [src]="previousVideo.thumbSrc"
                      [alt]="previousVideo.name"
                      loading="lazy">
                    <div *ngIf="!previousVideo.thumbSrc || previousVideo.thumbFailed" class="video-nav-fallback">
                      {{ thumbnailFallback(previousVideo.name) }}
                    </div>
                  </div>
                </div>
                <div class="video-nav-info">
                  <span class="video-nav-label">Previous</span>
                  <span class="video-nav-name" [title]="previousVideo.name">{{ previousVideo.name }}</span>
                </div>
              </div>
            </button>

            <!-- Next Video Navigation -->
            <button
              *ngIf="nextVideo"
              type="button"
              class="video-nav-hotspot video-nav-hotspot--next"
              [attr.aria-label]="'Open next video: ' + nextVideo.name"
              (click)="navigateToSibling(nextVideo, $event)">
              <div class="video-nav-card">
                <div class="video-nav-thumb-wrapper">
                  <div class="video-nav-thumb" [class.video-nav-thumb--empty]="!nextVideo.thumbSrc || nextVideo.thumbFailed">
                    <img
                      *ngIf="nextVideo.thumbSrc && !nextVideo.thumbFailed"
                      [src]="nextVideo.thumbSrc"
                      [alt]="nextVideo.name"
                      loading="lazy">
                    <div *ngIf="!nextVideo.thumbSrc || nextVideo.thumbFailed" class="video-nav-fallback">
                      {{ thumbnailFallback(nextVideo.name) }}
                    </div>
                  </div>
                </div>
                <div class="video-nav-info">
                  <span class="video-nav-label">Next</span>
                  <span class="video-nav-name" [title]="nextVideo.name">{{ nextVideo.name }}</span>
                </div>
              </div>
            </button>

            <div class="video-loading" *ngIf="videoLoading" role="status">
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
              <button type="button" class="control-btn play-pause-btn" [disabled]="!videoReady" (click)="videoPlayPause()"
                      [attr.aria-label]="videoIsPlaying ? 'Pause' : 'Play'">
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
  styles: [VIEWER_BASE_STYLES, `
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
      background: var(--vsc-editor-background, var(--vsc-editor-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
    }

    .viewer-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      padding: 8px 12px;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-border));
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 92%, black);
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
      display: inline-flex;
      align-items: center;
      justify-content: center;
      font-size: 10px;
      font-weight: 600;
      line-height: 1;
      padding: 2px 6px;
      border-radius: 3px;
      white-space: nowrap;
      letter-spacing: 0.3px;
    }

    .type-badge {
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 40%, transparent);
    }

    .size-badge {
      background: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 8%, transparent);
      color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 60%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 50%, transparent);
    }

    .seek-badge,
    .codec-badge,
    .resolution-badge {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      padding: 2px 6px;
      border-radius: 3px;
      font-weight: 500;
      line-height: 1;
      white-space: nowrap;
    }

    .seek-badge {
      background: color-mix(in srgb, var(--vsc-editorWarning-foreground, var(--vsc-warning)) 14%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-editorWarning-foreground, var(--vsc-warning)) 34%, transparent);
      color: var(--vsc-editorWarning-foreground, var(--vsc-warning));
    }

    .codec-badge {
      background: color-mix(in srgb, var(--vsc-button-hover) 10%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-button-hover) 24%, transparent);
      color: var(--vsc-button-hover);
    }

    .resolution-badge {
      background: color-mix(in srgb, var(--vsc-success) 10%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-success) 24%, transparent);
      color: var(--vsc-success);
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
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 25%, transparent);
      border-top-color: var(--vsc-accent-blue, var(--vsc-accent-blue));
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
      background: var(--vsc-media-canvas-background);
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
    }

    .video-stage {
      position: relative;
      flex: 1;
      min-height: 300px;
      overflow: hidden;
      background: var(--vsc-media-canvas-background);
    }

    .canvas-video-container {
      position: absolute;
      inset: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      background: linear-gradient(135deg, var(--vsc-media-canvas-background) 0%, var(--vsc-media-background) 100%);
      overflow: hidden;
      cursor: pointer;
      transition: filter 0.2s ease;
    }

    .video-stage:hover .canvas-video-container {
      filter: brightness(1.12);
    }

    .canvas-video-container ::ng-deep canvas {
      width: 100%;
      height: 100%;
      object-fit: contain;
      display: block;
    }

    /* Professional Video Navigation Styles */
    .video-nav-hotspot {
      position: absolute;
      top: 0;
      bottom: 0;
      width: 320px;
      padding: 0;
      border: 0;
      background: transparent;
      color: inherit;
      cursor: pointer;
      z-index: 70;
      opacity: 0;
      transition: opacity 0.3s cubic-bezier(0.2, 0.9, 0.4, 1.1);
      appearance: none;
      -webkit-appearance: none;
      border-radius: 0;
      outline: none;
      -webkit-tap-highlight-color: transparent;
    }

    .video-nav-hotspot--prev {
      z-index: 70;
    }

    .video-nav-hotspot--next {
      z-index: 71;
    }

    .video-nav-hotspot--prev {
      left: 0;
      background: linear-gradient(
        90deg,
        var(--vsc-media-overlay-strong) 0%,
        var(--vsc-media-overlay) 52%,
        transparent 100%
      );
    }

    .video-nav-hotspot--next {
      right: 0;
      background: linear-gradient(
        270deg,
        var(--vsc-media-overlay-strong) 0%,
        var(--vsc-media-overlay) 52%,
        transparent 100%
      );
    }

    .video-stage:hover .video-nav-hotspot {
      opacity: 1;
    }

    .video-nav-hotspot--prev:hover,
    .video-nav-hotspot--prev:focus-visible {
      background: linear-gradient(
        90deg,
        var(--vsc-media-overlay-strong) 0%,
        var(--vsc-media-overlay-soft) 50%,
        transparent 100%
      );
    }

    .video-nav-hotspot--next:hover,
    .video-nav-hotspot--next:focus-visible {
      background: linear-gradient(
        270deg,
        var(--vsc-media-overlay-strong) 0%,
        var(--vsc-media-overlay-soft) 50%,
        transparent 100%
      );
      
    }
    .video-nav-hotspot:hover,
    .video-nav-hotspot:focus,
    .video-nav-hotspot:focus-visible {
      z-index: 74;
    }

    .video-nav-card {
      position: absolute;
      top: 50%;
      transform: translateY(-50%);
      width: 260px;
      pointer-events: none;
      transition: all 0.3s cubic-bezier(0.2, 0.9, 0.4, 1.1);
    }

    .video-nav-hotspot--prev .video-nav-card {
      left: 24px;
      transform: translateY(-50%) translateX(-20px);
    }

    .video-nav-hotspot--next .video-nav-card {
      right: 24px;
      transform: translateY(-50%) translateX(20px);
    }

    .video-stage:hover .video-nav-hotspot--prev .video-nav-card {
      transform: translateY(-50%) translateX(0);
    }

    .video-stage:hover .video-nav-hotspot--next .video-nav-card {
      transform: translateY(-50%) translateX(0);
    }

    .video-nav-thumb-wrapper {
      position: relative;
      border-radius: 12px;
      overflow: hidden;
      box-shadow: var(--vsc-widget-shadow, 0 5px 8px 0 var(--vsc-shadow));
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 55%, transparent);
      transition: all 0.3s cubic-bezier(0.2, 0.9, 0.4, 1.1);
    }

    .video-nav-thumb {
      position: relative;
      width: 260px;
      aspect-ratio: 16 / 9;
      background: linear-gradient(
        135deg,
        color-mix(in srgb, var(--vsc-editorWidget-background, var(--vsc-panel-background)) 94%, black) 0%,
        color-mix(in srgb, var(--vsc-editorWidget-background, var(--vsc-panel-background)) 74%, black) 100%
      );
      overflow: hidden;
    }

    .video-nav-thumb img {
      width: 100%;
      height: 100%;
      display: block;
      object-fit: cover;
    }

    .video-nav-thumb--empty {
      display: flex;
      align-items: center;
      justify-content: center;
      background: linear-gradient(
        135deg,
        color-mix(in srgb, var(--vsc-editorWidget-background, var(--vsc-panel-background)) 94%, black) 0%,
        color-mix(in srgb, var(--vsc-editorWidget-background, var(--vsc-panel-background)) 74%, black) 100%
      );
    }

    .video-nav-fallback {
      font-size: 48px;
      font-weight: 600;
      color: var(--vsc-foreground, var(--vsc-foreground));
      opacity: 0.5;
      text-transform: uppercase;
    }

    .video-nav-info {
      margin-top: 12px;
      padding: 0 8px;
      text-align: left;
    }

    .video-nav-hotspot--next .video-nav-info {
      text-align: right;
    }

    .video-nav-label {
      display: inline-block;
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: var(--vsc-media-control-muted);
      background: color-mix(in srgb, var(--vsc-foreground) 10%, transparent);
      padding: 2px 8px;
      border-radius: 12px;
      margin-bottom: 6px;
    }

    .video-nav-name {
      display: block;
      font-size: 14px;
      font-weight: 500;
      color: var(--vsc-media-control-foreground);
      line-height: 1.3;
      word-break: break-word;
      text-shadow: 0 1px 4px var(--vsc-media-overlay-soft);
    }

    /* Loading State */
    .video-loading {
      position: absolute;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
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
      border: 3px solid var(--vsc-neutral-overlay-medium);
      border-top-color: var(--vsc-accent-blue, var(--vsc-accent-blue));
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
    }

    .loading-text {
      color: var(--vsc-foreground-dim);
      font-size: 13px;
      letter-spacing: 0.3px;
    }

    /* Play Overlay */
    .play-overlay {
      position: absolute;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 100%;
      height: 100%;
      background: var(--vsc-scrim);
      backdrop-filter: blur(4px);
      cursor: pointer;
      z-index: 50;
      transition: all 0.3s ease;
    }

    .play-overlay:hover {
      background: var(--vsc-scrim);
      backdrop-filter: blur(8px);
    }

    .play-icon {
      width: 80px;
      height: 80px;
      display: flex;
      align-items: center;
      justify-content: center;
      background: var(--vsc-accent-blue, var(--vsc-accent-blue));
      border-radius: 50%;
      font-size: 32px;
      color: var(--vsc-on-accent);
      box-shadow: 0 8px 32px var(--vsc-accent-border);
      transition: all 0.3s cubic-bezier(0.2, 0.9, 0.4, 1.1);
      font-weight: bold;
    }

    .play-overlay:hover .play-icon {
      background: var(--vsc-info);
      transform: scale(1.1);
      box-shadow: 0 12px 48px var(--vsc-accent-border);
    }

    /* Video Controls */
    .video-control-bar {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 12px 16px;
      background: linear-gradient(to top, var(--vsc-media-overlay-strong) 0%, var(--vsc-media-overlay) 50%, var(--vsc-scrim) 80%, transparent 100%);
      border-top: 1px solid var(--vsc-neutral-overlay-soft);
      opacity: 0;
      transition: opacity 0.3s ease;
      pointer-events: none;
    }

    .video-control-bar.show {
      opacity: 1;
      pointer-events: auto;
    }

    .video-stage:hover ~ .video-control-bar,
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
      background: color-mix(in srgb, var(--vsc-foreground) 20%, transparent);
      border-radius: 2px;
      cursor: pointer;
      overflow: hidden;
    }

    .progress-fill {
      position: absolute;
      height: 100%;
      background: linear-gradient(90deg, var(--vsc-accent-blue, var(--vsc-accent-blue)), var(--vsc-info));
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
      background: var(--vsc-document-canvas-background);
      border: none;
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 2px 8px var(--vsc-scrim-strong);
      transition: all 0.2s ease;
    }

    .seek-slider::-webkit-slider-thumb:hover {
      width: 16px;
      height: 16px;
      box-shadow: 0 4px 16px var(--vsc-accent-blue);
    }

    .seek-slider::-moz-range-thumb {
      width: 12px;
      height: 12px;
      background: var(--vsc-document-canvas-background);
      border: none;
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 2px 8px var(--vsc-scrim-strong);
      transition: all 0.2s ease;
    }

    .seek-slider::-moz-range-thumb:hover {
      width: 16px;
      height: 16px;
      box-shadow: 0 4px 16px var(--vsc-accent-blue);
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
      background: color-mix(in srgb, var(--vsc-foreground) 10%, transparent);
      border: 1px solid var(--vsc-neutral-overlay-medium);
      color: var(--vsc-on-accent);
      border-radius: 6px;
      cursor: pointer;
      font-size: 16px;
      transition: all 0.2s ease;
      font-weight: 600;
      backdrop-filter: blur(4px);
    }

    .control-btn:hover:not(:disabled) {
      background: color-mix(in srgb, var(--vsc-foreground) 20%, transparent);
      border-color: var(--vsc-neutral-overlay-strong);
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
      background: var(--vsc-accent-blue, var(--vsc-accent-blue));
      border: 1px solid var(--vsc-accent-selection);
    }

    .play-pause-btn:hover:not(:disabled) {
      background: var(--vsc-info);
      box-shadow: 0 0 12px var(--vsc-accent-border);
    }

    .time-display {
      display: flex;
      align-items: center;
      gap: 4px;
      font-size: 12px;
      color: var(--vsc-foreground);
      font-variant-numeric: tabular-nums;
      font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
      padding: 4px 8px;
      background: var(--vsc-scrim-soft);
      border-radius: 4px;
      min-width: 90px;
      justify-content: center;
    }

    .current-time {
      color: var(--vsc-on-accent);
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
      background: color-mix(in srgb, var(--vsc-foreground) 20%, transparent);
      border-radius: 2px;
      outline: none;
    }

    .volume-slider::-webkit-slider-thumb {
      appearance: none;
      -webkit-appearance: none;
      width: 10px;
      height: 10px;
      background: var(--vsc-document-canvas-background);
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 2px 6px var(--vsc-scrim-strong);
    }

    .volume-slider::-moz-range-thumb {
      width: 10px;
      height: 10px;
      background: var(--vsc-document-canvas-background);
      border: none;
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 2px 6px var(--vsc-scrim-strong);
    }

    .volume-slider:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    /* Responsive adjustments */
    @media (max-width: 768px) {
      .video-nav-hotspot {
        width: 200px;
      }
      
      .video-nav-card {
        width: 160px;
      }
      
      .video-nav-thumb {
        width: 160px;
      }
      
      .video-nav-name {
        font-size: 12px;
      }
    }
  `, VIEWER_PRODUCTION_CHROME_STYLES],
})
export class VideoViewerComponent implements OnChanges, AfterViewInit, OnDestroy {
  private static readonly WINDOWED_VIDEO_STREAM_SECONDS = 90;
  private static readonly AMBIGUOUS_VIDEO_EXTENSIONS = new Set(['ts']);
  private static readonly VIDEO_EXTENSIONS = new Set([
    'mp4', 'm4v', 'mov', 'qt', 'avi', 'divx', 'xvid', 'mkv', 'wmv', 'asf', 'flv', 'f4v',
    'webm', 'ogv', 'ts', 'mts', 'm2ts', 'mpg', 'mpeg', 'mpe', 'mpv', 'mp2', 'm2p', 'm2v',
    'vob', 'evo', 'mjpeg', 'mjpg', '3gp', '3g2', 'rm', 'rmvb',
  ]);

  @Input() targetPath = '';
  @Input() fileList: ViewerFileListItem[] = [];
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;
  @Input() probe: FileProbe | null = null;

  @ViewChild('videoContainer')
  set videoContainerRef(value: ElementRef<HTMLDivElement> | undefined) {
    this._videoContainerRef = value;
    this.requestSegmentedPlaybackInitialization();
    this.requestUrlPlaybackInitialization();
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
  previousVideo: VideoNavItem | null = null;
  nextVideo: VideoNavItem | null = null;

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
  private navThumbnailGeneration = 0;
  private initGeneration = 0;
  private playerSubscriptions: Unsubscribable[] = [];
  playbackMode = '';

  constructor(
    private readonly api: ApiService,
    private readonly cdr: ChangeDetectorRef,
    private readonly videoPlayerService: CanvasVideoPlayerService,
    private readonly workspaceService: WorkspaceService,
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

    const path = (this.targetPath ?? '').trim();
    if (!path && !this.archiveContext) {
      this.previousVideo = null;
      this.nextVideo = null;
      this.useLegacySurface = true;
      this.cdr.markForCheck();
      return;
    }

    await this.refreshSiblingNavigation();

    this.loading = true;
    this.cdr.markForCheck();
    try {
      let rawPlan: any;
      if (this.archiveContext) {
        rawPlan = await this.api.getArchiveVideoPlaybackPlan(
          this.archiveContext.fsPath,
          this.archiveContext.nestedChain,
          this.archiveContext.entryPath,
        );
      } else {
        const info = await this.api.getFileInfo(path);
        this.fileSize = info.size ?? 0;
        rawPlan = await this.api.getVideoPlaybackPlan(path);
      }
      const mode = String(rawPlan?.mode ?? rawPlan?.Mode ?? '').toLowerCase();
      const url = String(rawPlan?.url ?? rawPlan?.URL ?? '');
      this.playbackMode = mode || 'video';
      if (!url) {
        this.useLegacySurface = true;
      } else if (mode === 'segmented') {
        this.pendingManifestUrl = url;
        this.requestSegmentedPlaybackInitialization();
      } else {
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
    this.clearPlayerSubscriptions();
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

    if (isSeekingBackward) {
      const seekDistance = this.videoCurrentTime - seekSeconds;
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

  thumbnailFallback(name: string): string {
    const source = (name ?? '').trim();
    const first = source.charAt(0).toUpperCase();
    return first || 'V';
  }

  navigateToSibling(item: VideoNavItem, event?: Event): void {
    event?.preventDefault();
    event?.stopPropagation();
    if (!item.path || this.archiveContext) {
      return;
    }

    const workspace = this.workspaceService.getActiveWorkspace();
    if (!workspace) {
      return;
    }

    const current = this.findViewerTab(workspace, this.targetPath);
    if (!current) {
      return;
    }

    this.workspaceService.updateViewerTabTargetPath(current.paneId, current.tab.id, item.path, {
      sourcePane: current.tab.viewer?.sourcePane ?? current.paneId,
      sourceTabId: current.tab.viewer?.sourceTabId ?? '',
      sourceDir: current.tab.viewer?.sourceDir ?? '',
      fileList: this.fileList,
    });
  }

  private async createAndAttachCanvasPlayer(generation: number): Promise<CanvasVideoPlayer> {
    if (!this.videoContainerRef?.nativeElement) {
      throw new Error('Video container is not available.');
    }
    this.clearPlayerSubscriptions();
    if (this.canvasPlayer) {
      this.canvasPlayer.destroy();
      this.canvasPlayer = null;
    }
    const player = await this.videoPlayerService.createPlayer(this.videoContainerRef.nativeElement);
    this.canvasPlayer = player;
    player.setVolume(this.videoVolume / 100);
    this.playerSubscriptions.push(player.stateChanged$.subscribe((state) => {
      if (generation !== this.initGeneration || this.canvasPlayer !== player) return;
      this.videoIsPlaying = state === 'playing';
      if (state !== 'playing') {
        this.videoShowControls = true;
      }
      this.cdr.markForCheck();
    }));
    this.playerSubscriptions.push(player.metadataReady$.subscribe((metadata: VideoMetadata) => {
      if (generation !== this.initGeneration || this.canvasPlayer !== player) return;
      this.videoDuration = this.shouldUseWindowedVideoStreaming()
        ? Math.max(this.videoDuration, this.videoTimelineOffsetSeconds + metadata.duration / 1000)
        : metadata.duration / 1000;
      this.videoCodecInfo = metadata.videoCodec || '';
      this.videoResolution = `${metadata.width}\u00d7${metadata.height}`;
      this.cdr.markForCheck();
    }));
    this.playerSubscriptions.push(player.currentTime$.subscribe((seconds: number) => {
      if (generation !== this.initGeneration || this.canvasPlayer !== player) return;
      if (this.seekDragActive) {
        return;
      }
      this.videoCurrentTime = this.shouldUseWindowedVideoStreaming()
        ? this.videoTimelineOffsetSeconds + seconds
        : seconds;
      this.cdr.markForCheck();
    }));
    this.playerSubscriptions.push(player.error$.subscribe((message: string) => {
      if (generation !== this.initGeneration || this.canvasPlayer !== player) return;
      this.error = message;
      this.videoLoading = false;
      this.cdr.markForCheck();
    }));
    return player;
  }

  private async initializeSegmentedPlayback(manifestUrl: string, generation: number): Promise<void> {
    if (generation !== this.initGeneration) return;
    if (!this.videoContainerRef?.nativeElement) {
      return;
    }
    this.videoLoading = true;
    this.error = null;
    this.cdr.markForCheck();
    const loader = new SegmentVideoLoader();
    const source = await loader.load(manifestUrl);
    if (generation !== this.initGeneration) return;
    const player = await this.createAndAttachCanvasPlayer(generation);
    if (generation !== this.initGeneration || this.canvasPlayer !== player) return;
    await player.loadFromDemuxedSource(source);
    if (generation !== this.initGeneration || this.canvasPlayer !== player) return;
    this.videoReady = true;
    try {
      await player.play();
    } catch {
      // Ignore autoplay errors; user can manually start playback
    }
    this.videoLoading = false;
    this.cdr.markForCheck();
  }

  private async initializeUrlPlayback(streamUrl: string, generation: number): Promise<void> {
    if (generation !== this.initGeneration) return;
    if (!this.videoContainerRef?.nativeElement) {
      return;
    }
    this.videoLoading = true;
    this.loadingStreamUrl = streamUrl;
    this.videoTimelineOffsetSeconds = this.parseVideoStartOffsetSeconds(streamUrl);
    this.error = null;
    this.cdr.markForCheck();
    const player = await this.createAndAttachCanvasPlayer(generation);
    try {
      if (generation !== this.initGeneration || this.canvasPlayer !== player) return;
      await player.loadFromUrl(streamUrl);
      if (generation !== this.initGeneration || this.canvasPlayer !== player) return;
      this.currentStreamUrl = streamUrl;
      this.videoReady = true;
      try {
        await player.play();
      } catch {
        // Ignore autoplay errors
      }
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
    const generation = this.initGeneration;
    void this.initializeSegmentedPlayback(manifestUrl, generation)
      .catch((err: unknown) => {
        if (generation !== this.initGeneration) return;
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
    const generation = this.initGeneration;
    void this.initializeUrlPlayback(streamUrl, generation)
      .catch((err: unknown) => {
        if (generation !== this.initGeneration) return;
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
    this.initGeneration += 1;
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
    this.clearPlayerSubscriptions();
    if (this.canvasPlayer) {
      this.canvasPlayer.destroy();
      this.canvasPlayer = null;
    }
  }

  private clearPlayerSubscriptions(): void {
    for (const sub of this.playerSubscriptions) {
      try {
        sub.unsubscribe();
      } catch {
        // no-op
      }
    }
    this.playerSubscriptions = [];
  }

  private async refreshSiblingNavigation(): Promise<void> {
    const generation = ++this.navThumbnailGeneration;
    this.previousVideo = null;
    this.nextVideo = null;
    this.cdr.markForCheck();

    if (this.archiveContext) {
      return;
    }

    let siblings = await this.buildVideoNavItems(this.fileList, generation);

    if (!siblings.length && this.targetPath) {
      siblings = await this.loadSiblingVideosFromDirectory(generation);
    }

    if (!siblings.length) {
      return;
    }

    const currentIndex = siblings.findIndex(item => this.samePath(item.path, this.targetPath));
    if (currentIndex < 0) {
      return;
    }

    this.previousVideo = currentIndex > 0 ? siblings[currentIndex - 1] : null;
    this.nextVideo = currentIndex < siblings.length - 1 ? siblings[currentIndex + 1] : null;
    this.cdr.markForCheck();

    void this.hydrateNavThumbnail(this.previousVideo, generation);
    void this.hydrateNavThumbnail(this.nextVideo, generation);
  }

  private async loadSiblingVideosFromDirectory(generation: number): Promise<VideoNavItem[]> {
    try {
      const parentPath = await this.api.getParentPath(this.targetPath);
      if (!parentPath || generation !== this.navThumbnailGeneration) {
        return [];
      }

      const listing = await this.api.listDirectory(parentPath, false);
      if (generation !== this.navThumbnailGeneration) {
        return [];
      }

      return await this.buildVideoNavItems((listing.entries ?? []).map(entry => ({
          path: entry.path,
          name: entry.name || this.baseName(entry.path),
        })), generation);
    } catch {
      return [];
    }
  }

  private async buildVideoNavItems(items: Array<{ path: string; name: string }>, generation: number): Promise<VideoNavItem[]> {
    const resolved = await Promise.all(items.map(async item => {
      if (!item.path || generation !== this.navThumbnailGeneration) {
        return null;
      }
      return await this.isVideoNavCandidate(item.path)
        ? this.makeNavItem(item)
        : null;
    }));

    if (generation !== this.navThumbnailGeneration) {
      return [];
    }

    return resolved.filter((item): item is VideoNavItem => !!item);
  }

  private makeNavItem(item: ViewerFileListItem): VideoNavItem {
    return {
      path: item.path,
      name: item.name || this.baseName(item.path),
      thumbSrc: null,
      thumbLoading: false,
      thumbFailed: false,
    };
  }

  private async hydrateNavThumbnail(item: VideoNavItem | null, generation: number): Promise<void> {
    if (!item || item.thumbLoading || item.thumbSrc || item.thumbFailed) {
      return;
    }

    item.thumbLoading = true;
    try {
      if (generation !== this.navThumbnailGeneration) {
        return;
      }
      item.thumbSrc = await this.api.getVideoPreviewURL(item.path);
      item.thumbFailed = !item.thumbSrc;
    } catch {
      item.thumbFailed = true;
      item.thumbSrc = null;
    } finally {
      item.thumbLoading = false;
      if (generation === this.navThumbnailGeneration) {
        this.cdr.markForCheck();
      }
    }
  }

  private isVideoPath(path: string): boolean {
    return VideoViewerComponent.VIDEO_EXTENSIONS.has(this.extractExt(path));
  }

  private async isVideoNavCandidate(path: string): Promise<boolean> {
    const ext = this.extractExt(path);
    if (!VideoViewerComponent.VIDEO_EXTENSIONS.has(ext)) {
      return false;
    }

    if (!VideoViewerComponent.AMBIGUOUS_VIDEO_EXTENSIONS.has(ext)) {
      return true;
    }

    try {
      const probe = this.samePath(path, this.targetPath) && this.probe
        ? this.probe
        : await this.api.probeFile(path);
      const mime = (probe?.mimeType ?? '').toLowerCase();
      return mime.startsWith('video/') || mime === 'application/mp2t';
    } catch {
      return false;
    }
  }

  private extractExt(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/');
    const idx = normalized.lastIndexOf('.');
    return idx >= 0 ? normalized.slice(idx + 1).toLowerCase() : '';
  }

  private baseName(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/');
    return normalized.split('/').pop() ?? normalized;
  }

  private samePath(left: string, right: string): boolean {
    return left.trim().replace(/\\/g, '/').toLowerCase() === right.trim().replace(/\\/g, '/').toLowerCase();
  }

  private findViewerTab(workspace: ReturnType<WorkspaceService['getActiveWorkspace']>, targetPath: string): { paneId: 'left' | 'right'; tab: NonNullable<typeof workspace>['leftPane']['tabs'][number] } | null {
    if (!workspace) {
      return null;
    }

    const leftTab = workspace.leftPane.tabs.find(tab =>
      (tab.kind ?? 'dir') === 'viewer' && this.samePath(tab.viewer?.targetPath ?? '', targetPath)
    );
    if (leftTab) {
      return { paneId: 'left', tab: leftTab };
    }

    const rightTab = workspace.rightPane.tabs.find(tab =>
      (tab.kind ?? 'dir') === 'viewer' && this.samePath(tab.viewer?.targetPath ?? '', targetPath)
    );
    if (rightTab) {
      return { paneId: 'right', tab: rightTab };
    }

    return null;
  }
}