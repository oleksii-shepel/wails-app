import { CommonModule } from '@angular/common';
import {
  Component, Input, OnChanges, OnDestroy, ViewChild, ElementRef, Inject,
  ChangeDetectionStrategy, ChangeDetectorRef,
} from '@angular/core';
import { DomSanitizer, SafeResourceUrl } from '@angular/platform-browser';
import { ApiService } from '../../services/api.service';
import { ThemeService } from '../../services/theme.service';
import { FileInfo, FileProbe, FileType } from '../../shared/protocol';
import type { ViewerFileListItem } from '../../shared/protocol-enhanced';
import { CanvasVideoPlayer, CanvasVideoPlayerService, VideoMetadata } from '../../services/canvas-video-player.service';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import { ViewerArchiveContext } from '../../viewers/viewer-archive-context';
import { SegmentVideoLoader } from '../../viewers/builtins/segment-video-loader';
import { VIEWER_BASE_STYLES, VIEWER_PRODUCTION_CHROME_STYLES } from '../../viewers/builtins/builtin-viewer.shared';
import { withViewerFrameScrollbars } from '../../viewers/viewer-frame-scrollbars';

type ViewerKind =
  | 'image' | 'video' | 'audio' | 'pdf' | 'svg'
  | 'json' | 'markdown' | 'html' | 'text'
  | 'directory' | 'toobig' | 'unsupported' | 'error' | 'empty' | 'loading';

interface ImageMeta { width: number; height: number }

interface VideoPlaybackPlan {
  mode: 'direct' | 'segmented' | 'remux' | 'transcode';
  format: string;
  videoCodec?: string;
  audioCodec?: string;
  durationSeconds: number;
  zeroDelay: boolean;
  seekable: boolean;
  useNativePlayer: boolean;
  url: string;
}

function normalizeMime(mime: string | null | undefined): string {
  return (mime ?? '').split(';', 1)[0].trim().toLowerCase();
}

function probeLooksText(probe: FileProbe | null | undefined): boolean {
  const mime = normalizeMime(probe?.mimeType);
  if (mime.startsWith('text/')) return true;
  if (mime.endsWith('+json') || mime.endsWith('+xml')) return true;
  return !!probe?.encoding;
}

function probeLooksTransportStream(probe: FileProbe | null | undefined): boolean {
  const mime = normalizeMime(probe?.mimeType);
  return mime === 'video/mp2t' || mime === 'application/mp2t';
}

@Component({
  selector: 'app-viewer-surface',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <!-- Toolbar -->
      <div class="viewer-toolbar" *ngIf="hasToolbar">
        <ng-container *ngIf="kind === 'image' || kind === 'svg'">
          <button type="button" class="tool-btn" (click)="zoomOut()" aria-label="Zoom out" title="Zoom out">&#x2212;</button>
          <span class="zoom-label">{{ zoomLabel }}</span>
          <button type="button" class="tool-btn" (click)="zoomIn()" aria-label="Zoom in" title="Zoom in">+</button>
          <button type="button" class="tool-btn" (click)="resetZoom()" aria-label="Fit to window" title="Fit to window">Fit</button>
          <button type="button" class="tool-btn" (click)="zoom100()" aria-label="Actual size (1:1)" title="Actual size">1:1</button>
          <span class="toolbar-divider"></span>
          <button type="button" class="tool-btn" (click)="toggleCheckerboard()" [class.active]="showCheckerboard" [attr.aria-pressed]="showCheckerboard"
                  aria-label="Transparency grid" title="Toggle transparency grid">Grid</button>
        </ng-container>
          <ng-container *ngIf="isMarkupKind">
            <button type="button" class="tool-btn" (click)="setMarkupViewMode('rendered')" [class.active]="markupViewMode === 'rendered'" [attr.aria-pressed]="markupViewMode === 'rendered'"
              aria-label="Rendered preview" title="Rendered preview">Preview</button>
            <button type="button" class="tool-btn" (click)="setMarkupViewMode('source')" [class.active]="markupViewMode === 'source'" [attr.aria-pressed]="markupViewMode === 'source'"
              aria-label="Source view" title="Source view">Source</button>
            <span class="toolbar-divider"></span>
          </ng-container>
          <ng-container *ngIf="kind === 'text' || kind === 'json' || kind === 'markdown' || kind === 'html'">
          <button type="button" class="tool-btn" (click)="toggleWrap()" [class.active]="wordWrap" [attr.aria-pressed]="wordWrap"
                  aria-label="Word wrap" title="Toggle word wrap">Wrap</button>
        </ng-container>
      </div>

      <!-- Body -->
      <div class="viewer-body" [class.viewer-body--json]="kind === 'json'">

        <div *ngIf="kind === 'loading'" class="state-container">
          <div class="spinner"></div>
          <div class="state-label">Loading preview&#x2026;</div>
        </div>

        <div *ngIf="kind === 'empty'" class="state-container">
          <div class="state-icon">&#x1F50D;</div>
          <div class="state-label">Select a file to preview</div>
        </div>

        <div *ngIf="kind === 'error'" class="state-container state-error" role="alert">
          <div class="state-icon">&#x26A0;</div>
          <div class="state-label">Preview failed</div>
          <div class="error-detail">{{ error }}</div>
        </div>

        <div *ngIf="kind === 'directory'" class="state-container">
          <div class="state-icon">&#x1F4C1;</div>
          <div class="state-label">Directory — no preview available</div>
        </div>

        <div *ngIf="kind === 'toobig'" class="state-container">
          <div class="state-icon">&#x1F4E6;</div>
          <div class="state-label">File too large to preview</div>
          <div class="state-sub">{{ formatSize(info?.size ?? 0) }}</div>
        </div>

        <div *ngIf="kind === 'unsupported'" class="state-container">
          <div class="state-icon">&#x1F6AB;</div>
          <div class="state-label">No preview for this file type</div>
          <div class="state-sub" *ngIf="fileExt">.{{ fileExt }}</div>
          <div class="error-detail" *ngIf="error">{{ error }}</div>
        </div>

        <!-- Image -->
        <div *ngIf="kind === 'image'" class="image-host" [class.checkerboard]="showCheckerboard">
          <img class="preview-image"
               [src]="imageSrc!"
               [alt]="info?.name || 'Image preview'"
               [style.transform]="imageTransform"
               (load)="onImageLoad($event)"
               (error)="onImageError()">
        </div>

        <!-- SVG inline -->
        <div *ngIf="kind === 'svg'" class="image-host" [class.checkerboard]="showCheckerboard">
          <div class="preview-image svg-container"
               [style.transform]="imageTransform"
               [innerHTML]="svgContent"></div>
        </div>

        <!-- Canvas Video Player -->
        <div *ngIf="kind === 'video'" class="video-player-wrapper">
          <div class="canvas-video-container" 
               #videoContainer
               (mousemove)="onVideoMouseMove()"
               (mouseleave)="onVideoMouseLeave()"
               [class.playing]="videoIsPlaying"
               [class.show-controls]="videoShowControls">

            <!-- Loading indicator -->
            <div class="video-loading" role="status" *ngIf="videoLoading">
              <div class="spinner"></div>
              <div class="loading-text">Loading video...</div>
            </div>
            
            <!-- Play overlay (shown when paused) -->
            <div class="play-overlay" 
                 *ngIf="!videoIsPlaying && !videoLoading && videoReady"
                 (click)="videoPlayPause()"
                 title="Click to play">
              <div class="play-icon">▶</div>
            </div>
          </div>
          
          <!-- Video Controls (fade in/out on hover) -->
          <div class="video-control-bar" [class.show]="videoShowControls || !videoIsPlaying">
            <!-- Progress bar with hover preview -->
            <div class="progress-section">
              <div class="progress-container">
                <input 
                  type="range" 
                  min="0" 
                  [max]="videoDuration"
                  step="any"
                  [value]="videoIsSeeking ? videoSeekTarget : videoCurrentTime"
                  (mousedown)="onVideoSeekStart()"
                  (touchstart)="onVideoSeekStart()"
                  (input)="onVideoSeekInput($event)"
                  (change)="onVideoSeekCommit($event)"
                  class="seek-slider"
                  [disabled]="!videoSeekable || !videoReady || videoLoading"
                  title="Seek video">
                <div class="progress-fill" [style.width.%]="((videoIsSeeking ? videoSeekTarget : videoCurrentTime) / videoDuration) * 100 || 0"></div>
              </div>
            </div>

            <!-- Control buttons row -->
            <div class="controls-row">
              <!-- Play/Pause -->
              <button type="button"
                class="control-btn play-pause-btn"
                (click)="videoPlayPause()"
                [disabled]="!videoReady"
                [title]="videoIsPlaying ? 'Pause (Space)' : 'Play (Space)'"
                [attr.aria-label]="videoIsPlaying ? 'Pause' : 'Play'">
                <span class="icon">{{ videoIsPlaying ? '⏸' : '▶' }}</span>
              </button>

              <!-- Time display -->
              <div class="time-display">
                <span class="current-time">{{ formatTime(videoIsSeeking ? videoSeekTarget : videoCurrentTime) }}</span>
                <span class="separator">/</span>
                <span class="duration">{{ formatTime(videoDuration) }}</span>
              </div>

              <!-- Video info chip -->
              <div class="video-info-chip">
                <span class="mode-badge" [title]="videoPlaybackModeTitle" *ngIf="videoPlaybackModeLabel">
                  {{ videoPlaybackModeLabel }}
                </span>
                <span class="path-badge" [title]="videoPlaybackPathTitle" *ngIf="kind === 'video'">
                  {{ videoPlaybackPathLabel }}
                </span>
                <span class="latency-badge" title="Low-startup streaming playback path" *ngIf="videoPlaybackZeroDelay">
                  zero-delay
                </span>
                <span class="seek-badge" [title]="videoSeekable ? 'Seeking is available' : 'Seeking is not available for this playback path'" *ngIf="kind === 'video'">
                  {{ videoSeekable ? 'seek' : 'stream-only' }}
                </span>
                <span class="codec-badge" [title]="'Video codec: ' + videoCodecInfo">
                  {{ videoCodecInfo || '?' }}
                </span>
                <span class="resolution-badge" [title]="videoResolution" *ngIf="videoResolution">
                  {{ videoResolution }}
                </span>
                <span class="framerate-badge" [title]="'Frame rate: ' + videoFrameRate + ' fps'" *ngIf="videoFrameRate">
                  {{ videoFrameRate }}fps
                </span>
              </div>

              <!-- Spacer -->
              <div class="spacer"></div>

              <!-- Volume control -->
              <div class="volume-control" title="Volume">
                <span class="volume-icon">🔊</span>
                <input 
                  type="range" 
                  min="0" 
                  max="100"
                  [value]="videoVolume"
                  class="volume-slider"
                  [disabled]="!videoReady || !canvasPlayer"
                  (input)="onVideoVolumeChange($event)"
                  title="Volume">
              </div>

              <!-- Fullscreen button -->
              <button type="button" 
                class="control-btn fullscreen-btn" 
                (click)="toggleFullscreen()"
                [disabled]="!videoReady"
                aria-label="Toggle fullscreen" title="Fullscreen (F)">
                <span class="icon">⛶</span>
              </button>
            </div>
          </div>
          
          <!-- Error display -->
          <div class="video-error-banner" role="alert" *ngIf="videoError">
            <span class="error-icon">⚠️</span>
            <span class="error-message">{{ videoError }}</span>
            <button type="button" class="close-btn" (click)="dismissVideoError()" aria-label="Dismiss video error" title="Dismiss">✕</button>
          </div>
        </div>

        <!-- Audio -->
        <div *ngIf="kind === 'audio'" class="audio-host">
          <div class="audio-icon">&#x1F3B5;</div>
          <div class="state-label">{{ info?.name }}</div>
          <audio class="preview-audio" controls preload="metadata"
                 [src]="mediaSrc!"
                 (loadedmetadata)="onMediaMetadata('audio', $event)"
                 (canplay)="onMediaCanPlay('audio', $event)"
                 (error)="onMediaError('audio', $event)">
            Your browser does not support audio.
          </audio>
        </div>

        <!-- PDF — the embedded renderer becomes ready only after the full document load -->
        <div *ngIf="kind === 'pdf'" class="pdf-host">
          <object class="preview-pdf" [data]="pdfSrc!" type="application/pdf" (load)="onPdfLoad()">
            <div class="pdf-fallback">
              <div class="state-label">Embedded PDF preview is unavailable</div>
              <div class="state-sub">The stream loaded, but the embedded renderer could not display this PDF.</div>
              <button type="button" *ngIf="!archiveContext && targetPath" class="tool-btn" (click)="openCurrentFileExternally()">Open externally</button>
            </div>
          </object>
          <div *ngIf="pdfLoading" class="pdf-loading" role="status">
            <div class="spinner"></div>
            <div class="state-label">Loading PDF preview…</div>
            <div class="state-sub">Embedded PDF rendering starts after the full document is loaded.</div>
          </div>
        </div>

        <!-- JSON -->
           <pre *ngIf="kind === 'json'"
             class="preview-text json-text"
             [class.word-wrap]="wordWrap"><code [innerHTML]="highlightedJson"></code></pre>

        <!-- Markdown -->
        <div *ngIf="kind === 'markdown' && markupViewMode === 'rendered'"
             class="markdown-body"
             [class.word-wrap]="wordWrap"
             [innerHTML]="renderedMarkdown"></div>

        <!-- HTML -->
        <iframe *ngIf="kind === 'html' && markupViewMode === 'rendered'"
          class="html-preview-frame"
          sandbox
          referrerpolicy="no-referrer"
          [attr.srcdoc]="renderedHtmlDocument"></iframe>

        <!-- Plain text / source -->
        <pre *ngIf="kind === 'text' || ((kind === 'markdown' || kind === 'html') && markupViewMode === 'source')"
             class="preview-text"
             [class.word-wrap]="wordWrap">{{ textContent }}</pre>

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

    /* ── Header ─────────────────────────────────────────────────────────── */

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

    .file-icon { font-size: 18px; flex-shrink: 0; line-height: 1; }

    .header-text { min-width: 0; flex: 1; }

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
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 40%, transparent);
    }

    .archive-badge {
      background: color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-warning)) 20%, transparent);
      color: var(--vsc-accent-orange, var(--vsc-warning));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-warning)) 40%, transparent);
    }

    .size-badge, .dim-badge, .line-badge {
      background: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 8%, transparent);
      color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 60%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 50%, transparent);
    }

    /* ── Toolbar ─────────────────────────────────────────────────────────── */

    .viewer-toolbar {
      display: flex;
      align-items: center;
      gap: 2px;
      padding: 4px 8px;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-border));
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 96%, black);
      flex-shrink: 0;
    }

    .tool-btn {
      background: transparent;
      border: 1px solid transparent;
      color: var(--vsc-foreground, var(--vsc-foreground));
      font-size: 12px;
      padding: 2px 8px;
      border-radius: 3px;
      cursor: pointer;
      line-height: 1.4;
      min-width: 24px;
      transition: background 0.1s, border-color 0.1s;

      &:hover {
        background: var(--vsc-list-hover-background, var(--vsc-neutral-overlay-subtle));
        border-color: var(--vsc-border, var(--vsc-border));
      }

      &.active {
        background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent);
        border-color: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 50%, transparent);
        color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      }
    }

    .zoom-label {
      font-size: 11px;
      min-width: 44px;
      text-align: center;
      opacity: 0.8;
      font-variant-numeric: tabular-nums;
    }

    .toolbar-divider {
      width: 1px;
      height: 16px;
      background: var(--vsc-border, var(--vsc-border));
      margin: 0 4px;
    }

    /* ── Body ────────────────────────────────────────────────────────────── */

    .viewer-body {
      flex: 1 1 0%;
      display: flex;
      flex-direction: column;
      min-height: 0;
      overflow: auto;
      position: relative;
    }

    .viewer-body--json {
      overflow: hidden;
    }

    /* ── State containers ────────────────────────────────────────────────── */

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

    .state-icon { font-size: 36px; line-height: 1; opacity: 0.55; }
    .state-label { font-size: 13px; opacity: 0.65; }
    .state-sub   { font-size: 11px; opacity: 0.4; }

    .state-error .state-icon  { opacity: 1; }
    .state-error .state-label { color: var(--vsc-error); opacity: 1; }

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

    /* ── Loading spinner ─────────────────────────────────────────────────── */

    .spinner {
      width: 26px;
      height: 26px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 25%, transparent);
      border-top-color: var(--vsc-accent-blue, var(--vsc-accent-blue));
      border-radius: 50%;
      animation: spin 0.65s linear infinite;
    }

    @keyframes spin { to { transform: rotate(360deg); } }

    /* ── Image viewer ────────────────────────────────────────────────────── */

    .image-host {
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100%;
      padding: 16px;
      box-sizing: border-box;
      overflow: hidden;
    }

    .image-host.checkerboard {
      background-image:
        linear-gradient(45deg, var(--vsc-neutral-overlay-soft) 25%, transparent 25%),
        linear-gradient(-45deg, var(--vsc-neutral-overlay-soft) 25%, transparent 25%),
        linear-gradient(45deg, transparent 75%, var(--vsc-neutral-overlay-soft) 75%),
        linear-gradient(-45deg, transparent 75%, var(--vsc-neutral-overlay-soft) 75%);
      background-size: 16px 16px;
      background-position: 0 0, 0 8px, 8px -8px, -8px 0px;
    }

    .preview-image {
      display: block;
      max-width: 100%;
      max-height: 100%;
      object-fit: contain;
      transition: transform 0.15s ease;
    }

    .svg-container {
      max-width: 100%;
      max-height: 100%;
      display: flex;
      align-items: center;
      justify-content: center;
      transition: transform 0.15s ease;

      svg { max-width: 100%; max-height: 100%; }
    }

    /* ── Video ───────────────────────────────────────────────────────────── */

    .media-host {
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100%;
      padding: 16px;
      box-sizing: border-box;
    }

    /* ── Video (Canvas Player) ────────────────────────────────────────────── */

    .video-player-wrapper {
      display: flex;
      flex-direction: column;
      width: 100%;
      height: 100%;
      background: var(--vsc-media-canvas-background);
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
    }

    .canvas-video-container {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 300px;
      background: linear-gradient(135deg, var(--vsc-media-canvas-background) 0%, var(--vsc-media-background) 100%);
      position: relative;
      overflow: hidden;
      cursor: pointer;
    }

    .canvas-video-container:hover {
      cursor: pointer;
    }

    .canvas-video-container.playing:hover {
      cursor: pointer;
    }

    .canvas-video-container ::ng-deep canvas {
      width: 100%;
      height: 100%;
      object-fit: contain;
      display: block;
    }

    .native-video-element {
      width: 100%;
      height: 100%;
      object-fit: contain;
      display: block;
      background: var(--vsc-media-canvas-background);
    }

    /* Loading indicator */
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

    /* Play overlay */
    .play-overlay {
      position: absolute;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 100%;
      height: 100%;
      background: var(--vsc-scrim);
      backdrop-filter: blur(2px);
      cursor: pointer;
      z-index: 50;
      transition: background 0.2s ease;
    }

    .play-overlay:hover {
      background: var(--vsc-scrim);
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
      transition: all 0.2s ease;
      font-weight: bold;
    }

    .play-overlay:hover .play-icon {
      background: var(--vsc-info);
      transform: scale(1.1);
      box-shadow: 0 12px 48px var(--vsc-accent-border);
    }

    /* Control bar */
    .video-control-bar {
      display: flex;
      flex-direction: column;
      gap: 8px;
      padding: 12px;
      background: linear-gradient(to bottom, transparent, var(--vsc-media-overlay-strong));
      border-top: 1px solid var(--vsc-neutral-overlay-soft);
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

    /* Progress section */
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
      transition: background 0.05s linear;
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
      transition: width 0.05s ease, height 0.05s ease, box-shadow 0.05s ease;
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
      transition: width 0.05s ease, height 0.05s ease, box-shadow 0.05s ease;
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

    /* Controls row */
    .controls-row {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 0 4px;
    }

    /* Control buttons */
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
      border-radius: 4px;
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

    /* Time display */
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
      border-radius: 3px;
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

    /* Video info chip */
    .video-info-chip {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 4px 8px;
      background: var(--vsc-scrim);
      border-radius: 12px;
      font-size: 11px;
      border: 1px solid var(--vsc-neutral-overlay-soft);
    }

    .mode-badge, .path-badge, .latency-badge, .seek-badge, .codec-badge, .resolution-badge, .framerate-badge {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      padding: 2px 6px;
      background: color-mix(in srgb, var(--vsc-button-hover) 10%, transparent);
      border: 0.5px solid color-mix(in srgb, var(--vsc-button-hover) 24%, transparent);
      border-radius: 3px;
      color: var(--vsc-button-hover);
      font-weight: 500;
      line-height: 1;
      cursor: help;
      white-space: nowrap;
    }

    .mode-badge {
      background: color-mix(in srgb, var(--vsc-foreground) 12%, transparent);
      border-color: var(--vsc-neutral-overlay-medium);
      color: var(--vsc-media-foreground-strong);
      text-transform: uppercase;
      letter-spacing: 0.3px;
    }

    .path-badge {
      background: var(--vsc-chart-fill-4);
      border-color: var(--vsc-accent-border);
      color: var(--vsc-chart-4);
    }

    .latency-badge {
      background: var(--vsc-success-surface);
      border-color: var(--vsc-success-border);
      color: var(--vsc-chart-5);
    }

    .seek-badge {
      background: color-mix(in srgb, var(--vsc-editorWarning-foreground, var(--vsc-warning)) 14%, transparent);
      border-color: color-mix(in srgb, var(--vsc-editorWarning-foreground, var(--vsc-warning)) 34%, transparent);
      color: var(--vsc-editorWarning-foreground, var(--vsc-warning));
    }

    .resolution-badge {
      background: color-mix(in srgb, var(--vsc-success) 10%, transparent);
      border-color: color-mix(in srgb, var(--vsc-success) 24%, transparent);
      color: var(--vsc-success);
    }

    .framerate-badge {
      background: var(--vsc-warning-surface);
      border-color: var(--vsc-warning-border);
      color: var(--vsc-chart-6);
    }

    /* Spacer */
    .spacer {
      flex: 1;
    }

    /* Volume control */
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

    .fullscreen-btn {
      min-width: 40px;
    }

    /* Error banner */
    .video-error-banner {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 12px;
      background: linear-gradient(135deg, var(--vsc-error-surface) 0%, var(--vsc-error-surface) 100%);
      border-top: 1px solid var(--vsc-error-border);
      backdrop-filter: blur(8px);
      color: var(--vsc-error-bright);
      font-size: 12px;
      animation: slideDown 0.3s ease;
    }

    .error-icon {
      display: inline-block;
      font-size: 16px;
      flex-shrink: 0;
    }

    .error-message {
      flex: 1;
      line-height: 1.4;
    }

    .close-btn {
      background: none;
      border: none;
      color: var(--vsc-error-bright);
      cursor: pointer;
      font-size: 16px;
      padding: 0 4px;
      flex-shrink: 0;
      transition: color 0.2s ease;
    }

    .close-btn:hover {
      color: var(--vsc-error-bright);
    }

    @keyframes slideDown {
      from {
        transform: translateY(-100%);
        opacity: 0;
      }
      to {
        transform: translateY(0);
        opacity: 1;
      }
    }

    /* ── Audio ───────────────────────────────────────────────────────────── */

    .audio-host {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      min-height: 100%;
      gap: 16px;
      padding: 32px;
    }

    .audio-icon { font-size: 56px; opacity: 0.4; }
    .preview-audio { width: 100%; max-width: 380px; }

    /* ── PDF ─────────────────────────────────────────────────────────────── */

    .pdf-host { display: flex; flex-direction: column; flex: 1 1 0%; min-height: 0; min-width: 0; background: var(--vsc-document-canvas-background); position: relative; }
    .preview-pdf { flex: 1; width: 100%; height: 100%; border: none; background: var(--vsc-document-canvas-background); }
    .pdf-loading {
      position: absolute;
      inset: 0;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 10px;
      padding: 24px;
      text-align: center;
      color: var(--vsc-media-panel-background);
      background: color-mix(in srgb, var(--vsc-foreground) 30%, transparent);
    }
    .pdf-fallback {
      height: 100%;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 12px;
      padding: 24px;
      text-align: center;
      color: var(--vsc-media-panel-background);
      background: var(--vsc-document-canvas-background);
    }

    /* ── Text / JSON / Markdown ──────────────────────────────────────────── */

    .preview-text {
      margin: 0;
      padding: 12px 16px;
      white-space: pre;
      font-size: 12px;
      line-height: 1.5;
      font-family: var(--vsc-font-family-mono, ui-monospace, 'Courier New', monospace);
      tab-size: 4;
      color: var(--vsc-foreground, var(--vsc-foreground));

      &.word-wrap { white-space: pre-wrap; word-break: break-word; }
    }

    .json-text {
      display: block;
      flex: 1 1 auto;
      min-height: 0;
      min-width: 0;
      width: 100%;
      box-sizing: border-box;
      overflow: auto;

      .json-key  { color: var(--vsc-chart-7); }
      .json-str  { color: var(--vsc-chart-6); }
      .json-num  { color: var(--vsc-chart-8); }
      .json-bool { color: var(--vsc-chart-1); }
      .json-null { color: var(--vsc-chart-1); }
    }

    .markdown-body {
      display: block;
      width: 100%;
      min-height: 100%;
      box-sizing: border-box;
      padding: 16px 20px;
      line-height: 1.65;
      font-size: 13px;

      &.word-wrap { word-break: break-word; }

      img {
        max-width: 100%;
        height: auto;
      }
    }

    .html-preview-frame {
      display: block;
      width: 100%;
      height: 100%;
      border: 0;
      background: var(--vsc-document-canvas-background);
    }

    .viewer-body > .html-preview-frame {
      flex: 1 1 auto;
      min-height: 100%;
    }

    .viewer-body > .markdown-body {
      flex: 1 1 auto;
    }

    .markdown-body {
      h1, h2, h3, h4, h5, h6 {
        color: var(--vsc-foreground-bright, var(--vsc-on-accent));
        margin: 1.2em 0 0.4em;
        font-weight: 600;
        line-height: 1.3;
      }
      h1 { font-size: 1.6em; border-bottom: 1px solid var(--vsc-border, var(--vsc-border)); padding-bottom: 6px; }
      h2 { font-size: 1.3em; border-bottom: 1px solid var(--vsc-border, var(--vsc-border)); padding-bottom: 4px; }
      h3 { font-size: 1.1em; }
      p  { margin: 0.6em 0; }

      a { color: var(--vsc-accent-blue, var(--vsc-focus-ring)); text-decoration: none; }
      a:hover { text-decoration: underline; }

      code {
        font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
        font-size: 0.9em;
        background: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 10%, transparent);
        border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 80%, transparent);
        padding: 1px 5px;
        border-radius: 3px;
      }

      pre {
        background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 70%, black);
        border: 1px solid var(--vsc-border, var(--vsc-border));
        border-radius: 5px;
        padding: 12px 14px;
        overflow-x: auto;
        margin: 0.8em 0;

        code { background: none; border: none; padding: 0; font-size: 11px; }
      }

      blockquote {
        margin: 0.8em 0;
        padding: 4px 12px;
        border-left: 3px solid var(--vsc-accent-blue, var(--vsc-accent-blue));
        opacity: 0.8;
      }

      ul, ol { margin: 0.6em 0; padding-left: 24px; }
      li { margin: 2px 0; }

      table { border-collapse: collapse; margin: 0.8em 0; font-size: 12px; }
      th, td { padding: 5px 12px; border: 1px solid var(--vsc-border, var(--vsc-border)); text-align: left; }
      th { background: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 8%, transparent); font-weight: 600; }

      hr { border: none; border-top: 1px solid var(--vsc-border, var(--vsc-border)); margin: 1.2em 0; }
      img { max-width: 100%; border-radius: 3px; }
    }
  `, VIEWER_PRODUCTION_CHROME_STYLES]
})
export class ViewerSurfaceComponent implements OnChanges, OnDestroy {
  private static readonly WINDOWED_VIDEO_STREAM_SECONDS = 90;

  /**
   * Filesystem path for regular (non-archive) files.
   * When this is a VFS archive path and archiveContext is also set, the
   * context takes precedence for reading content.
   */
  @Input() targetPath = '';
  @Input() fileList: ViewerFileListItem[] = [];

  @Input() hideHeader = false;

  /**
   * Optional — populated by pane host when the viewer tab was opened
    * for a file inside an archive. When present, content is read through the
    * archive service rather than a direct filesystem ReadFile call.
   */
  @Input() archiveContext: ViewerArchiveContext | null = null;

  /**
   * Optional viewer registry id passed by the host surface. The built-in
   * preview currently doesn't need it directly, but it is part of the viewer
   * component contract and must be accepted by Angular.
   */
  @Input() viewerId: string | null = null;
  @Input() probe: FileProbe | null = null;
  @ViewChild('videoContainer') videoContainerRef?: ElementRef<HTMLDivElement>;

  // ── State ────────────────────────────────────────────────────────────────

  private destroy$ = createSubject<void>();
  canvasPlayer: CanvasVideoPlayer | null = null;
  
  // Video player state
  videoReady = false;
  videoIsPlaying = false;
  videoLoading = false;
  videoShowControls = true;
  videoCurrentTime = 0;
  videoDuration = 0;
  videoCodecInfo = '';
  videoFrameRate = 0;
  videoResolution = '';
  videoVolume = 100;
  videoError: string | null = null;
  videoSeekable = true;
  videoIsSeeking = false;
  videoSeekTarget = 0;
  private videoControlsTimeout: ReturnType<typeof setTimeout> | null = null;
  private videoPlaybackPlan: VideoPlaybackPlan | null = null;
  private videoTimelineOffsetSeconds = 0;
  private restartLoadGeneration = 0;
  private seekGeneration = 0;
  private seekDragActive = false;
  private seekInFlight = false;
  private queuedSeekSeconds: number | null = null;

  info: FileInfo | null = null;
  kind: ViewerKind = 'empty';
  error: string | null = null;

  imageSrc:         string | null        = null;
  svgContent:       string | null        = null;
  mediaSrc:         string | null        = null;
  mediaMime:        string | null        = null;
  /** Stream URL for embedded PDF rendering. */
  pdfSrc:           SafeResourceUrl | null = null;
  pdfLoading        = false;
  textContent:      string | null        = null;
  highlightedJson:  string | null        = null;
  renderedMarkdown: string | null        = null;
  renderedHtmlDocument: string | null    = null;
  markupViewMode: 'rendered' | 'source'  = 'rendered';

  imageMeta:  ImageMeta | null = null;
  lineCount:  number | null    = null;
  zoomLevel   = 0;
  showCheckerboard = false;
  wordWrap = false;
  private mediaProbeTimer: ReturnType<typeof setTimeout> | null = null;
  private lastAutoplayKey: string | null = null;
  private readonly themeSubscription: { unsubscribe(): void };



  FileType = FileType;

  // ── Derived ──────────────────────────────────────────────────────────────

  get displayName(): string {
    if (this.info?.name) return this.info.name;
    if (this.archiveContext?.entryPath) {
      return this.archiveContext.entryPath.replace(/\\/g, '/').split('/').pop() ?? '';
    }
    return this.targetPath ? this.baseName(this.targetPath) : '';
  }

  get hasToolbar(): boolean {
    return this.kind === 'image' || this.kind === 'svg' ||
           this.kind === 'text'  || this.kind === 'json' || this.kind === 'markdown' || this.kind === 'html';
  }

  get isMarkupKind(): boolean {
    return this.kind === 'markdown' || this.kind === 'html';
  }

  get videoPlaybackModeLabel(): string {
    switch (this.videoPlaybackPlan?.mode) {
      case 'direct':
        return 'direct';
      case 'segmented':
        return 'segmented';
      case 'remux':
        return 'remux';
      case 'transcode':
        return 'transcode';
      default:
        return '';
    }
  }

  get videoPlaybackModeTitle(): string {
    switch (this.videoPlaybackPlan?.mode) {
      case 'direct':
        return 'Direct playback: preview uses the original stream as-is.';
      case 'segmented':
        return 'Segmented playback: preview uses a fragmented MP4 manifest and keyframe-aware segment loading.';
      case 'remux':
        return 'Remux playback: the preview stream is repackaged without re-encoding when possible.';
      case 'transcode':
        return 'Transcode playback: the preview stream is re-encoded for compatibility.';
      default:
        return 'Playback mode unavailable.';
    }
  }

  get videoPlaybackPathLabel(): string {
    return 'canvas';
  }

  get videoPlaybackPathTitle(): string {
    return 'Preview is using the canvas/WebCodecs playback path.';
  }

  get videoPlaybackZeroDelay(): boolean {
    return Boolean(this.videoPlaybackPlan?.zeroDelay);
  }

  get fileExt(): string {
    return this.extractExt(this.archiveContext?.entryPath ?? this.targetPath);
  }

  get imageTransform(): string {
    return this.zoomLevel === 0 ? 'none' : `scale(${this.zoomLevel})`;
  }

  get zoomLabel(): string {
    return this.zoomLevel === 0 ? 'Fit' : `${Math.round(this.zoomLevel * 100)}%`;
  }

  get fileIcon(): string {
    const name = this.info?.name ?? this.archiveContext?.entryPath ?? this.targetPath;
    const ext  = this.extractExt(name);
    if (this.info?.type === FileType.DIRECTORY) return '📁';
    const map: Record<string, string> = {
      png: '🖼', jpg: '🖼', jpeg: '🖼', gif: '🖼', webp: '🖼',
      bmp: '🖼', svg: '🖼', ico: '🖼', avif: '🖼',
      mp4: '🎬', webm: '🎬', mov: '🎬', ogv: '🎬',
      mp3: '🎵', wav: '🎵', ogg: '🎵', flac: '🎵', aac: '🎵', m4a: '🎵',
      pdf: '📕', md: '📝', markdown: '📝',
      zip: '📦', tar: '📦', gz: '📦', '7z': '📦', rar: '📦',
    };
    return map[ext] ?? '📄';
  }

  get typeBadge(): string {
    const name = this.info?.name ?? this.archiveContext?.entryPath ?? this.targetPath;
    const ext  = this.extractExt(name);
    if (ext) return ext.toUpperCase();
    if (this.info?.type === FileType.DIRECTORY) return 'DIR';
    return 'FILE';
  }

  // ── Size limits ──────────────────────────────────────────────────────────
  // With streaming, there are no practical limits - files are streamed progressively
  private readonly MAX_TEXT_BYTES  = Infinity;
  private readonly MAX_IMAGE_BYTES = Infinity;
  private readonly MAX_MEDIA_BYTES = Infinity;

  constructor(
    private api: ApiService,
    private cdr: ChangeDetectorRef,
    private sanitizer: DomSanitizer,
    @Inject(CanvasVideoPlayerService) private videoPlayerService: CanvasVideoPlayerService,
    private themeService: ThemeService,
  ) {
    this.themeSubscription = this.themeService.getTheme$().subscribe(() => {
      if (this.kind === 'html' && this.textContent !== null) {
        this.renderedHtmlDocument = this.renderHTMLDocument(this.textContent);
        this.cdr.markForCheck();
      }
    });
  }

  async ngOnChanges(): Promise<void> {
    this.resetState();
    const path = (this.targetPath ?? '').trim();

    if (!path && !this.archiveContext) {
      this.kind = 'empty';
      this.cdr.markForCheck();
      return;
    }

    this.kind = 'loading';
    this.cdr.markForCheck();

    try {
      if (this.archiveContext) {
        await this.loadFromArchive(this.archiveContext);
      } else {
        await this.loadFromDisk(path);
      }
    } catch (err: any) {
      this.error = err?.message || String(err);
      this.kind  = 'error';
    }

    this.cdr.markForCheck();
  }

  ngOnDestroy(): void {
    this.themeSubscription.unsubscribe();
    this.clearMediaProbe();
    if (this.canvasPlayer) {
      this.canvasPlayer.destroy();
      this.canvasPlayer = null;
    }
    this.destroy$.next();
    this.destroy$.complete();
  }

  // ── Toolbar controls ──────────────────────────────────────────────────────

  zoomIn():    void { this.zoomLevel = this.zoomLevel === 0 ? 1.25 : Math.min(this.zoomLevel * 1.25, 8); this.cdr.markForCheck(); }
  zoomOut():   void { this.zoomLevel = this.zoomLevel === 0 ? 0.8  : Math.max(this.zoomLevel * 0.8, 0.1); this.cdr.markForCheck(); }
  resetZoom(): void { this.zoomLevel = 0; this.cdr.markForCheck(); }
  zoom100():   void { this.zoomLevel = 1; this.cdr.markForCheck(); }
  toggleCheckerboard(): void { this.showCheckerboard = !this.showCheckerboard; this.cdr.markForCheck(); }
  toggleWrap():         void { this.wordWrap = !this.wordWrap; this.cdr.markForCheck(); }
  setMarkupViewMode(mode: 'rendered' | 'source'): void {
    if (this.markupViewMode === mode) return;
    this.markupViewMode = mode;
    this.cdr.markForCheck();
  }

  onImageLoad(event: Event): void {
    const img = event.target as HTMLImageElement;
    this.imageMeta = { width: img.naturalWidth, height: img.naturalHeight };
    this.cdr.markForCheck();
  }

  async onImageError(): Promise<void> {
    // If a normal image stream fails, check if we can convert it as a fallback
    if (!this.archiveContext && this.kind === 'image') {
      const convertible = await this.api.canConvertImage(this.targetPath || '');
      if (convertible) {
        try {
          this.imageSrc = await this.api.getConvertedImageURL(this.targetPath || '', 'jpeg');
          this.cdr.markForCheck();
          return;
        } catch { /* proceed to error */ }
      }
    }

    this.error = 'Image failed to render.';
    this.kind  = 'error';
    this.cdr.markForCheck();
  }

  onMediaError(kind: 'video' | 'audio', event: Event): void {
    this.clearMediaProbe();
    const media = event.target as HTMLMediaElement | null;
    const code = media?.error?.code;
    const currentSrc = media?.currentSrc || this.mediaSrc || '';
    const reason = code === MediaError.MEDIA_ERR_ABORTED ? 'load aborted'
      : code === MediaError.MEDIA_ERR_NETWORK ? 'network error'
      : code === MediaError.MEDIA_ERR_DECODE ? 'decode error or unsupported codec'
      : code === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED ? 'source not supported'
      : 'playback failed';
    this.error = `${kind} failed to play: ${reason}${currentSrc ? ` (${currentSrc})` : ''}`;
    this.kind = 'error';
    this.cdr.markForCheck();
  }

  onMediaMetadata(kind: 'video' | 'audio', event: Event): void {
    this.armMediaProbe(kind, event.target as HTMLMediaElement | null);
  }

  onMediaCanPlay(kind: 'video' | 'audio', event: Event): void {
    this.attemptMediaAutoplay(kind, event.target as HTMLMediaElement | null);
  }

  onVideoLoadedData(event: Event): void {
    this.clearMediaProbe();
    const video = event.target as HTMLVideoElement | null;
    this.attemptMediaAutoplay('video', video);
    if (video && video.videoWidth === 0 && video.videoHeight === 0) {
      this.error = `video loaded but no decodable video track (${this.mediaMime ?? 'unknown mime'})`;
      this.kind = 'error';
    }
    this.cdr.markForCheck();
  }

  onPdfLoad(): void {
    this.pdfLoading = false;
    this.cdr.markForCheck();
  }

  private attemptCanvasAutoplay(): void {
    if (!this.canvasPlayer || this.kind !== 'video') {
      return;
    }

    this.canvasPlayer.play()
      .then(() => {
        this.videoIsPlaying = true;
        this.videoLoading = false;
        this.cdr.markForCheck();
      })
      .catch((error: unknown) => {
        this.videoLoading = false;
        this.videoError = error instanceof Error ? error.message : 'Playback failed';
        this.cdr.markForCheck();
      });
  }

  // ── Canvas Video Player Controls ──────────────────────────────────────────

  async videoPlayPause(): Promise<void> {
    if (!this.canvasPlayer) return;

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
    } catch (err: any) {
      console.error('Playback control error:', err);
      this.videoError = err.message || 'Playback error';
      this.videoLoading = false;
      this.cdr.markForCheck();
    }
  }

  onVideoSeekStart(): void {
    if (!this.videoSeekable || !this.videoReady || this.videoLoading) return;
    this.seekDragActive = true;
    this.updateVideoSeekingState();
    this.videoSeekTarget = this.videoCurrentTime;
    this.cdr.markForCheck();
  }

  onVideoSeekInput(event: Event): void {
    if (!this.videoSeekable || !this.videoReady || this.videoLoading) return;
    const target = event.target as HTMLInputElement;
    const seekSeconds = parseFloat(target.value);
    this.videoSeekTarget = seekSeconds;
    this.videoCurrentTime = seekSeconds;
    this.queueVideoSeek(seekSeconds);
    this.cdr.markForCheck();
  }

  onVideoSeekCommit(event: Event): void {
    const target = event.target as HTMLInputElement;
    const seekSeconds = parseFloat(target.value);
    if (!this.videoSeekable || !this.videoReady || this.videoLoading) return;
    this.seekDragActive = false;
    this.videoSeekTarget = seekSeconds;
    this.videoCurrentTime = seekSeconds;
    this.queueVideoSeek(seekSeconds);
    this.updateVideoSeekingState();
    this.cdr.markForCheck();
  }

  private queueVideoSeek(seekSeconds: number): void {
    if (!this.canvasPlayer || !this.videoSeekable || !this.videoReady || this.videoLoading) return;
    this.queuedSeekSeconds = seekSeconds;
    this.updateVideoSeekingState();
    if (this.seekInFlight) {
      return;
    }

    void this.flushQueuedVideoSeek();
  }

  private async flushQueuedVideoSeek(): Promise<void> {
    if (!this.canvasPlayer || !this.videoSeekable || !this.videoReady || this.videoLoading || this.seekInFlight) return;

    this.seekInFlight = true;
    try {
      while (this.queuedSeekSeconds !== null) {
        const seekSeconds = this.queuedSeekSeconds;
        this.queuedSeekSeconds = null;
        const generation = ++this.seekGeneration;
        const shouldResumePlayback = this.canvasPlayer.isPlaying() || this.isVideoAtEnd();

        this.videoSeekTarget = seekSeconds;
        this.videoCurrentTime = seekSeconds;
        this.updateVideoSeekingState();
        this.cdr.markForCheck();

        try {
          if (this.shouldRestartVideoStreamOnSeek(seekSeconds)) {
            await this.restartCanvasVideoAt(seekSeconds, { autoplay: shouldResumePlayback, accurate: true });
          } else {
            const playerSeekSeconds = this.shouldUseWindowedVideoStreaming()
              ? Math.max(0, seekSeconds - this.videoTimelineOffsetSeconds)
              : seekSeconds;

            await this.canvasPlayer.seek(playerSeekSeconds * 1000);

            this.videoReady = true;
            this.videoError = null;
            this.cdr.markForCheck();
          }
        } catch (err: any) {
          if (generation === this.seekGeneration) {
            console.error('Seek error:', err);
            this.videoError = err.message || 'Seek failed';
            this.videoLoading = false;
            this.cdr.markForCheck();
          }
        }
      }
    } finally {
      this.seekInFlight = false;
      this.updateVideoSeekingState();
      this.cdr.markForCheck();
    }
  }

  private updateVideoSeekingState(): void {
    this.videoIsSeeking = this.seekDragActive || this.seekInFlight || this.queuedSeekSeconds !== null;
  }

  onVideoVolumeChange(event: Event): void {
    const target = event.target as HTMLInputElement;
    const nextVolume = Math.max(0, Math.min(100, Number(target.value)));

    this.videoVolume = nextVolume;
    this.canvasPlayer?.setVolume(nextVolume / 100);
    this.cdr.markForCheck();
  }

  onVideoMouseMove(): void {
    this.videoShowControls = true;
    
    // Auto-hide controls after 3 seconds of inactivity when playing
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
    if (this.videoIsPlaying) {
      this.videoShowControls = false;
      if (this.videoControlsTimeout) {
        clearTimeout(this.videoControlsTimeout);
        this.videoControlsTimeout = null;
      }
    }
  }

  toggleFullscreen(): void {
    const container = this.videoContainerRef?.nativeElement;
    if (!container) return;

    const isFullscreen = document.fullscreenElement === container;

    if (isFullscreen) {
      document.exitFullscreen?.().catch(() => {});
    } else {
      container.requestFullscreen?.().catch(() => {
        this.videoError = 'Fullscreen not supported';
        this.cdr.markForCheck();
      });
    }
  }

  dismissVideoError(): void {
    this.videoError = null;
    this.cdr.markForCheck();
  }

  private initializeCanvasVideoPlayer(): void {
    if (!this.videoContainerRef?.nativeElement || !this.mediaSrc || this.kind !== 'video') return;

    if (this.canvasPlayer) {
      this.canvasPlayer.destroy();
      this.canvasPlayer = null;
    }

    this.videoReady = false;
    this.videoError = null;
    this.videoTimelineOffsetSeconds = this.parseVideoStartOffsetSeconds(this.mediaSrc);
    this.videoCurrentTime = this.videoTimelineOffsetSeconds;
    this.videoDuration = 0;
    this.videoCodecInfo = '';
    this.videoFrameRate = 0;
    this.videoResolution = '';
    this.videoVolume = 100;

    this.videoPlayerService.createPlayer(this.videoContainerRef.nativeElement)
      .then((player: CanvasVideoPlayer) => {
        this.canvasPlayer = player;
        this.canvasPlayer.setVolume(this.videoVolume / 100);

        // Subscribe to state changes
        this.canvasPlayer.stateChanged$
          .pipe(takeUntil(this.destroy$))
          .subscribe((state: 'idle' | 'paused' | 'playing' | 'ended') => {
            this.videoIsPlaying = state === 'playing';
            if (state === 'ended') {
              if (this.shouldAutoAdvanceWindowedPlayback()) {
                void this.restartCanvasVideoAt(this.videoCurrentTime, { autoplay: true });
                return;
              }
              this.videoCurrentTime = this.videoDuration;
            }
            if (state !== 'playing') {
              this.videoShowControls = true;
              if (this.videoControlsTimeout) {
                clearTimeout(this.videoControlsTimeout);
                this.videoControlsTimeout = null;
              }
            }
            this.cdr.markForCheck();
          });

        // Subscribe to metadata ready
        this.canvasPlayer.metadataReady$
          .pipe(takeUntil(this.destroy$))
          .subscribe((metadata: VideoMetadata) => {
            this.videoDuration = this.getDisplayedVideoDuration(metadata);
            this.videoCodecInfo = metadata.videoCodec || 'Unknown codec';
            this.videoFrameRate = metadata.frameRate || 0;
            this.videoResolution = `${metadata.width}×${metadata.height}`;
            this.videoCurrentTime = this.videoTimelineOffsetSeconds;
            this.cdr.markForCheck();
          });

        this.canvasPlayer.currentTime$
          .pipe(takeUntil(this.destroy$))
          .subscribe((seconds: number) => {
            if (this.videoIsSeeking || this.videoLoading) {
              return;
            }
            this.videoCurrentTime = this.getDisplayedVideoCurrentTime(seconds);
            this.cdr.markForCheck();
          });

        // Subscribe to errors
        this.canvasPlayer.error$
          .pipe(takeUntil(this.destroy$))
          .subscribe((error: string) => {
            this.videoError = error;
            this.videoLoading = false;
            this.cdr.markForCheck();
          });

        // Load the video
        if (this.mediaSrc) {
          this.videoLoading = true;
          this.cdr.markForCheck();

          const loadPromise = this.videoPlaybackPlan?.mode === 'segmented'
            ? (async () => {
                const loader = new SegmentVideoLoader();
                const source = await loader.load(this.mediaSrc!);
                return this.canvasPlayer!.loadFromDemuxedSource(source);
              })()
            : this.canvasPlayer.loadFromUrl(this.mediaSrc);

          loadPromise
            .then(() => {
              this.videoReady = true;
              this.attemptCanvasAutoplay();
              this.cdr.markForCheck();
            })
            .catch((err: unknown) => {
              console.error('Video load error:', err);
              this.videoError = err instanceof Error ? err.message : 'Failed to load video';
              this.videoReady = false;
              this.videoLoading = false;
              this.cdr.markForCheck();
            });
        }
      })
      .catch((err: unknown) => {
        console.error('Failed to create video player:', err);
        this.videoError = err instanceof Error ? err.message : 'Failed to initialize video player';
        this.cdr.markForCheck();
      });
  }

  async openCurrentFileExternally(): Promise<void> {
    if (this.archiveContext || !this.targetPath) {
      return;
    }
    try {
      await this.api.openFile(this.targetPath);
    } catch (err) {
      this.error = err instanceof Error ? err.message : 'Failed to open file externally';
      this.kind = 'error';
      this.cdr.markForCheck();
    }
  }

  baseName(path: string): string {
    return path.replace(/\\/g, '/').split('/').pop() || path;
  }

  formatSize(bytes: number): string {
    if (!bytes || bytes <= 0) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const v = bytes / Math.pow(1024, i);
    return `${i === 0 ? v : v.toFixed(1)} ${units[i]}`;
  }

  formatTime(seconds: number): string {
    if (!seconds || seconds < 0) return '0:00';
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

  // ── Private: loading paths ────────────────────────────────────────────────

  /**
   * Load a regular on-disk file.  The path must be a real filesystem path —
   * not a virtual archive path.
   */
  private async loadFromDisk(path: string): Promise<void> {
    const info = await this.api.getFileInfo(path);
    this.info = info;

    if (info.type === FileType.DIRECTORY) {
      this.kind = 'directory';
      return;
    }

    const ext = this.extractExt(path);
    const probe = this.probe ?? (ext === 'ts' ? await this.api.probeFile(path).catch(() => null) : null);
    await this.renderContent(ext, info.size, {
      readText: async () => {
        const streamURL = await this.api.getStreamURL(path);
        const result = await this.api.fetchTextFromURL(streamURL, this.normalizeEncoding(probe?.encoding ?? ''));
        return result.content;
      },
      streamBinary: (_mimeType: string) => {
    // Return a dedicated localhost stream URL so large media avoids
    // Wails' buffered asset server path on Windows.
    return this.api.getStreamURL(path);
      },
    }, probe);
  }

  /**
   * Load a file from inside an archive using streaming.
   */
  private async loadFromArchive(ctx: ViewerArchiveContext): Promise<void> {
    const entryName = ctx.entryPath.replace(/\\/g, '/').split('/').pop() ?? ctx.entryPath;
    const ext = this.extractExt(ctx.entryPath);
    const probe = this.probe ?? await this.api.probeArchiveFile(ctx.fsPath, ctx.nestedChain, ctx.entryPath).catch(() => null);

    await this.renderContent(ext, null, {
      readText: async () => {
        const encoding = this.normalizeEncoding(probe?.encoding ?? '');
        const streamURL = await this.api.getArchiveStreamURL(ctx.fsPath, ctx.nestedChain, ctx.entryPath);
        const result = await this.api.fetchTextFromURL(streamURL, encoding);
        this.info = this.makeSyntheticInfo(entryName, ctx.entryPath, result.size);
        return result.content;
      },
      streamBinary: (_mimeType: string) => {
    // Archive streams use the same dedicated localhost server to avoid
    // WebView buffering for large binary responses.
    return this.api.getArchiveStreamURL(ctx.fsPath, ctx.nestedChain, ctx.entryPath);
      },
    }, probe);

    if (!this.info) {
      this.info = this.makeSyntheticInfo(entryName, ctx.entryPath, 0);
    }
  }

  /**
   * Core rendering logic — shared between disk and archive paths.
    * Uses stream URLs for all viewable content.
   */
  private async renderContent(
    ext: string,
    sizeHint: number | null,
    readers: {
      readText: () => Promise<string | null>;
      streamBinary: (mimeType: string) => Promise<string>;
    },
    probe: FileProbe | null = null,
  ): Promise<void> {
    try {
      const tsLooksText = ext === 'ts' && probeLooksText(probe);
      const tsLooksTransportStream = ext === 'ts' && probeLooksTransportStream(probe);

      // ── SVG ────────────────────────────────────────────────────────────────
      if (ext === 'svg') {
        if (sizeHint !== null && sizeHint > this.MAX_IMAGE_BYTES) { this.kind = 'toobig'; return; }
        const text = await readers.readText();
        if (text === null) { this.kind = 'unsupported'; return; }
        this.svgContent = text
          .replace(/<\?xml[^>]*\?>/gi, '')
          .replace(/<!DOCTYPE[^>]*>/gi, '')
          .trim();
        this.kind = 'svg';
        return;
      }

      // ── Raster images ───────────────────────────────────────────────────────
      const imageMime = this.inferImageMime(ext);
      if (imageMime) {
        if (sizeHint !== null && sizeHint > this.MAX_IMAGE_BYTES) { this.kind = 'toobig'; return; }
        try {
          this.imageSrc = await readers.streamBinary(imageMime);
          this.kind = 'image';
          return;
        } catch { this.kind = 'error'; this.error = 'Failed to stream image'; return; }
      }

      // ── Image Conversion Fallback ────────────────────────────────────────────
      // Not an archive stream and is convertible?
      if (!this.archiveContext) {
        const convertible = await this.api.canConvertImage(this.targetPath || '');
        if (convertible) {
          try {
            // Ask Go to convert and give us a temporary preview URL
            this.imageSrc = await this.api.getConvertedImageURL(this.targetPath || '', 'jpeg');
            this.kind = 'image';
            return;
          } catch (err) {
            console.warn('Conversion failed', err);
            // Fall through to other types
          }
        }
      }

      // ── Video ────────────────────────────────────────────────────────────────
      const videoMime = this.inferVideoMime(ext);
      if (videoMime && (!tsLooksText || tsLooksTransportStream || ext !== 'ts')) {
        if (sizeHint !== null && sizeHint > this.MAX_MEDIA_BYTES) { this.kind = 'toobig'; return; }
        if (!this.videoPlayerService.supportsCanvasVideoFormat(ext)) {
          this.kind = 'unsupported';
          this.error = this.videoPlayerService.getUnsupportedVideoMessage(ext);
          return;
        }
        try {
          const playbackPlan = await this.resolveVideoPlaybackPlan(videoMime, readers);
          this.videoPlaybackPlan = playbackPlan;
          this.videoSeekable = playbackPlan.seekable;
          this.mediaSrc = playbackPlan.url;
          this.mediaMime = videoMime;
          this.videoDuration = playbackPlan.durationSeconds > 0 ? playbackPlan.durationSeconds : 0;
          this.videoCodecInfo = playbackPlan.videoCodec || '';
          this.kind = 'video';
          this.videoLoading = true;
          
          // Initialize Canvas video player after view renders
          setTimeout(() => this.initializeCanvasVideoPlayer(), 50);
          return;
        } catch { this.kind = 'error'; this.error = 'Failed to stream video'; return; }
      }

      // ── Audio ────────────────────────────────────────────────────────────────
      const audioMime = this.inferAudioMime(ext);
      if (audioMime) {
        if (sizeHint !== null && sizeHint > this.MAX_MEDIA_BYTES) { this.kind = 'toobig'; return; }
        try {
          this.mediaSrc = await readers.streamBinary(audioMime);
          this.mediaMime = audioMime;
          this.kind = 'audio';
          return;
        } catch { this.kind = 'error'; this.error = 'Failed to stream audio'; return; }
      }

      // ── PDF ──────────────────────────────────────────────────────────────────
      if (ext === 'pdf') {
        if (sizeHint !== null && sizeHint > this.MAX_MEDIA_BYTES) { this.kind = 'toobig'; return; }
        try {
          const pdfUrl = await readers.streamBinary('application/pdf');
          this.pdfSrc = this.sanitizer.bypassSecurityTrustResourceUrl(pdfUrl);
          this.pdfLoading = true;
          this.kind = 'pdf';
          return;
        } catch { this.kind = 'error'; this.error = 'Failed to stream PDF'; return; }
      }

      // ── Text-based types ─────────────────────────────────────────────────────
      if (sizeHint !== null && sizeHint > this.MAX_TEXT_BYTES) { this.kind = 'toobig'; return; }

      const text = await readers.readText();
      if (text === null) { this.kind = 'unsupported'; return; }

      // JSON
      if (ext === 'json' || ext === 'jsonc') {
        try {
          const pretty = JSON.stringify(JSON.parse(text), null, 2);
          this.highlightedJson = this.syntaxHighlightJson(pretty);
          this.lineCount = pretty.split('\n').length;
          this.kind = 'json';
          return;
        } catch { /* fall through to plain text */ }
      }

      // Markdown
      if (ext === 'md' || ext === 'markdown' || ext === 'mdx') {
        this.textContent = text;
        this.renderedMarkdown = this.renderMarkdown(text);
        this.lineCount = text.split('\n').length;
        this.kind = 'markdown';
        return;
      }

      // HTML
      if (ext === 'html' || ext === 'htm') {
        this.textContent = text;
        this.renderedHtmlDocument = this.renderHTMLDocument(text);
        this.lineCount = text.split('\n').length;
        this.kind = 'html';
        return;
      }

      // Plain text
      if (this.isTextExtension(ext) || this.looksLikeText(text)) {
        this.textContent = text;
        this.lineCount   = text.split('\n').length;
        this.kind        = 'text';
        return;
      }

      this.kind = 'unsupported';
    } catch (err) {
      this.kind = 'error';
      this.error = err instanceof Error ? err.message : String(err);
    }
  }

  // ── Private helpers ──────────────────────────────────────────────────────

  private resetState(): void {
    this.clearMediaProbe();
    if (this.canvasPlayer) {
      this.canvasPlayer.destroy();
      this.canvasPlayer = null;
    }
    if (this.videoControlsTimeout) {
      clearTimeout(this.videoControlsTimeout);
      this.videoControlsTimeout = null;
    }
    this.info            = null;
    this.error           = null;
    this.imageSrc        = null;
    this.svgContent      = null;
    this.mediaSrc        = null;
    this.mediaMime       = null;
    this.pdfSrc          = null;
    this.pdfLoading      = false;
    this.textContent     = null;
    this.highlightedJson = null;
    this.renderedMarkdown = null;
    this.renderedHtmlDocument = null;
    this.imageMeta       = null;
    this.lineCount       = null;
    this.zoomLevel       = 0;
    this.showCheckerboard = false;
    this.lastAutoplayKey = null;
    this.videoReady      = false;
    this.videoIsPlaying  = false;
    this.videoLoading    = false;
    this.videoShowControls = true;
    this.videoCurrentTime = 0;
    this.videoDuration   = 0;
    this.videoCodecInfo  = '';
    this.videoFrameRate  = 0;
    this.videoResolution = '';
    this.videoError      = null;
    this.videoSeekable   = true;
    this.videoIsSeeking  = false;
    this.videoSeekTarget = 0;
    this.videoPlaybackPlan = null;
    this.videoTimelineOffsetSeconds = 0;
    this.restartLoadGeneration = 0;
  }

  private attemptMediaAutoplay(kind: 'video' | 'audio', media: HTMLMediaElement | null): void {
    if (!media || !this.mediaSrc || this.kind !== kind) {
      return;
    }

    const sourceKey = `${kind}:${media.currentSrc || this.mediaSrc}`;
    if (this.lastAutoplayKey === sourceKey) {
      return;
    }
    this.lastAutoplayKey = sourceKey;

    const playPromise = media.play();
    if (playPromise && typeof playPromise.catch === 'function') {
      playPromise.catch(() => {
        // Best-effort autoplay only. Embedded runtimes may still enforce policies.
      });
    }
  }

  private armMediaProbe(kind: 'video' | 'audio', media: HTMLMediaElement | null): void {
    this.clearMediaProbe();
    if (!media) {
      return;
    }

    this.mediaProbeTimer = setTimeout(() => {
      if (kind === 'video') {
        const video = media as HTMLVideoElement;
        if (this.kind === 'video' && (video.readyState < HTMLMediaElement.HAVE_CURRENT_DATA || video.videoWidth === 0)) {
          this.error = `video stream did not produce a frame (readyState=${video.readyState}, networkState=${video.networkState}, mime=${this.mediaMime ?? 'unknown'})`;
          this.kind = 'error';
          this.cdr.markForCheck();
        }
        return;
      }

      if (this.kind === 'audio' && media.readyState === HTMLMediaElement.HAVE_NOTHING) {
        this.error = `audio stream did not load media data (networkState=${media.networkState}, mime=${this.mediaMime ?? 'unknown'})`;
        this.kind = 'error';
        this.cdr.markForCheck();
      }
    }, 2000);
  }

  private clearMediaProbe(): void {
    if (this.mediaProbeTimer !== null) {
      clearTimeout(this.mediaProbeTimer);
      this.mediaProbeTimer = null;
    }
  }

  private makeSyntheticInfo(name: string, path: string, size: number): FileInfo {
    return {
      name, path, size,
      type: FileType.FILE,
      modified: 0, created: 0, accessed: 0,
      permissions: '', isHidden: false,
    } as FileInfo;
  }

  private extractExt(path: string): string {
    const idx = (path ?? '').lastIndexOf('.');
    return (idx >= 0 ? path.slice(idx + 1) : '').toLowerCase();
  }

  private normalizeEncoding(raw: string): string {
    const encoding = (raw ?? '').trim().toLowerCase();
    if (!encoding || encoding === 'utf-8') return '';
    if (encoding === 'utf16' || encoding === 'utf-16') return 'utf16le';
    return encoding;
  }

  private inferImageMime(ext: string): string | null {
    return ({
      png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg',
      gif: 'image/gif', webp: 'image/webp', bmp: 'image/bmp',
      ico: 'image/x-icon', tif: 'image/tiff', tiff: 'image/tiff', avif: 'image/avif',
      // High-end formats that usually need conversion
      psd: 'image/vnd.adobe.photoshop', ai: 'image/application/postscript',
      heic: 'image/heic', heif: 'image/heif',
    } as Record<string, string>)[ext] ?? null;
  }

  private inferVideoMime(ext: string): string | null {
    return ({
      mp4: 'video/mp4', m4v: 'video/mp4', webm: 'video/webm', ogv: 'video/ogg', mov: 'video/quicktime', qt: 'video/quicktime',
      mkv: 'video/x-matroska', avi: 'video/x-msvideo', divx: 'video/x-msvideo', xvid: 'video/x-msvideo', flv: 'video/x-flv',
      ts: 'video/mp2t', m2ts: 'video/mp2t', mts: 'video/mp2t',
      mpg: 'video/mpeg', mpeg: 'video/mpeg', vob: 'video/mpeg', m2p: 'video/mpeg', m2v: 'video/mpeg',
      mjpeg: 'video/x-motion-jpeg', mjpg: 'video/x-motion-jpeg',
      m3u8: 'application/x-mpegURL', mxf: 'application/mxf', wmv: 'video/x-ms-wmv',
    } as Record<string, string>)[ext] ?? null;
  }

  private async resolveVideoPlaybackPlan(
    videoMime: string,
    readers: { streamBinary: (mimeType: string) => Promise<string> },
  ): Promise<VideoPlaybackPlan> {
    const fallbackUrl = await readers.streamBinary(videoMime);

    if (this.archiveContext) {
      const rawPlan = await this.api.getArchiveVideoPlaybackPlan(this.archiveContext.fsPath, this.archiveContext.nestedChain, this.archiveContext.entryPath);
      return this.normalizeVideoPlaybackPlan(rawPlan, fallbackUrl);
    }

    const rawPlan = await this.api.getVideoPlaybackPlan(this.targetPath || '');
    return this.normalizeVideoPlaybackPlan(rawPlan, fallbackUrl);
  }

  private normalizeVideoPlaybackPlan(rawPlan: any, fallbackUrl: string): VideoPlaybackPlan {
    const readPlanValue = <T>(camelKey: string, pascalKey: string): T | undefined => {
      if (!rawPlan || typeof rawPlan !== 'object') {
        return undefined;
      }
      if (rawPlan[camelKey] !== undefined) {
        return rawPlan[camelKey] as T;
      }
      if (rawPlan[pascalKey] !== undefined) {
        return rawPlan[pascalKey] as T;
      }
      return undefined;
    };

    const rawMode = readPlanValue<string>('mode', 'Mode');
    const normalized: VideoPlaybackPlan = {
      mode: rawMode === 'segmented' || rawMode === 'remux' || rawMode === 'transcode' ? rawMode : 'direct',
      format: String(readPlanValue<string>('format', 'Format') ?? ''),
      videoCodec: readPlanValue<string>('videoCodec', 'VideoCodec') ?? undefined,
      audioCodec: readPlanValue<string>('audioCodec', 'AudioCodec') ?? undefined,
      durationSeconds: Number(readPlanValue<number>('durationSeconds', 'DurationSeconds') ?? 0),
      zeroDelay: Boolean(readPlanValue<boolean>('zeroDelay', 'ZeroDelay')),
      seekable: readPlanValue<boolean>('seekable', 'Seekable') !== false,
      useNativePlayer: Boolean(readPlanValue<boolean>('useNativePlayer', 'UseNativePlayer')),
      url: (() => {
        const planUrl = readPlanValue<string>('url', 'URL');
        return typeof planUrl === 'string' && planUrl ? planUrl : fallbackUrl;
      })(),
    };

    if (!normalized.url) {
      normalized.url = fallbackUrl;
    }
    if (normalized.mode === 'direct') {
      normalized.useNativePlayer = false;
    }
    return normalized;
  }

  private buildNativeSeekURL(startSeconds: number, options?: { accurate?: boolean }): string {
    const currentUrl = this.videoPlaybackPlan?.url ?? this.mediaSrc ?? '';
    if (!currentUrl) return '';

    try {
      const resolved = new URL(currentUrl, window.location.href);
      if (startSeconds > 0) {
        resolved.searchParams.set('start', startSeconds.toFixed(3));
      } else {
        resolved.searchParams.delete('start');
      }
      if (this.shouldUseWindowedVideoStreaming()) {
        resolved.searchParams.set('window', ViewerSurfaceComponent.WINDOWED_VIDEO_STREAM_SECONDS.toFixed(3));
      }
      if (options?.accurate && startSeconds > 0 && this.shouldRestartVideoStreamOnSeek(startSeconds)) {
        resolved.searchParams.set('mode', 'transcode');
      }
      return resolved.toString();
    } catch {
      return currentUrl;
    }
  }

  private shouldRestartVideoStreamOnSeek(seekSeconds: number): boolean {
    if (this.videoPlaybackPlan?.mode !== 'transcode') {
      return false;
    }

    const windowStart = this.videoTimelineOffsetSeconds;
    const windowEnd = windowStart + ViewerSurfaceComponent.WINDOWED_VIDEO_STREAM_SECONDS;
    return seekSeconds < windowStart || seekSeconds >= windowEnd;
  }

  private shouldUseWindowedVideoStreaming(): boolean {
    return this.videoPlaybackPlan?.mode === 'transcode';
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

  private getDisplayedVideoDuration(metadata: VideoMetadata): number {
    const plannedDuration = this.videoPlaybackPlan?.durationSeconds ?? 0;
    if (this.shouldUseWindowedVideoStreaming() && plannedDuration > 0) {
      return plannedDuration;
    }

    return metadata.duration / 1000;
  }

  private getDisplayedVideoCurrentTime(playerSeconds: number): number {
    const absoluteSeconds = this.videoTimelineOffsetSeconds + playerSeconds;
    if (this.videoDuration > 0) {
      return Math.min(absoluteSeconds, this.videoDuration);
    }

    return absoluteSeconds;
  }

  private shouldAutoAdvanceWindowedPlayback(): boolean {
    return this.shouldUseWindowedVideoStreaming() && this.videoCurrentTime + 0.5 < this.videoDuration;
  }

  private isVideoAtEnd(): boolean {
    return this.videoDuration > 0 && this.videoCurrentTime >= this.videoDuration - 0.25;
  }

  private async restartCanvasVideoAt(seekSeconds: number, options?: { autoplay?: boolean; accurate?: boolean }): Promise<void> {
    if (!this.canvasPlayer) {
      return;
    }

    const generation = ++this.restartLoadGeneration;
    const nextUrl = this.buildNativeSeekURL(seekSeconds, { accurate: options?.accurate });
    if (!nextUrl) {
      throw new Error('Unable to build a seekable playback URL for this stream.');
    }

    const previousUrl = this.mediaSrc ?? this.videoPlaybackPlan?.url ?? '';
    const previousTimelineOffsetSeconds = this.videoTimelineOffsetSeconds;
    const previousCurrentTime = this.videoCurrentTime;
    const shouldResumePlayback = options?.autoplay ?? this.videoIsPlaying;
    this.videoLoading = true;
    this.videoReady = false;
    this.videoError = null;
    this.videoTimelineOffsetSeconds = seekSeconds;
    this.videoCurrentTime = seekSeconds;
    this.mediaSrc = nextUrl;
    this.cdr.markForCheck();

    try {
      await this.canvasPlayer.loadFromUrl(nextUrl);

      if (generation !== this.restartLoadGeneration) return;

      this.videoReady = true;

      if (shouldResumePlayback) {
        await this.canvasPlayer.play();
      }
    } catch (err: any) {
      if (generation !== this.restartLoadGeneration) return;
      const loadError = err instanceof Error ? err.message : 'Failed to load video';

      if (previousUrl && previousUrl !== nextUrl) {
        try {
          await this.canvasPlayer.loadFromUrl(previousUrl);
          if (generation !== this.restartLoadGeneration) return;

          const previousSeekSeconds = Math.max(0, previousCurrentTime - previousTimelineOffsetSeconds);
          if (previousSeekSeconds > 0) {
            await this.canvasPlayer.seek(previousSeekSeconds * 1000);
          }

          this.mediaSrc = previousUrl;
          this.videoTimelineOffsetSeconds = previousTimelineOffsetSeconds;
          this.videoCurrentTime = previousCurrentTime;
          this.videoReady = true;
          this.videoError = null;

          if (shouldResumePlayback) {
            await this.canvasPlayer.play();
          }
        } catch {
          this.videoError = loadError;
          this.videoReady = false;
        }
      } else {
        this.videoError = loadError;
        this.videoReady = false;
      }
    } finally {
      if (generation === this.restartLoadGeneration) {
        this.videoLoading = false;
      }
    }

    this.cdr.markForCheck();
  }

  private inferAudioMime(ext: string): string | null {
    return ({
      mp3: 'audio/mpeg', wav: 'audio/wav', ogg: 'audio/ogg',
      flac: 'audio/flac', aac: 'audio/aac', m4a: 'audio/mp4',
    } as Record<string, string>)[ext] ?? null;
  }

  private isTextExtension(ext: string): boolean {
    return new Set([
      'txt','log','csv','tsv','ini','cfg','conf','yaml','yml','toml',
      'xml','html','htm','css','scss','sass','less',
      'js','ts','jsx','tsx','vue','svelte',
      'py','rb','go','rs','java','kt','swift','c','cpp','h','hpp',
      'cs','php','sh','bash','zsh','fish','ps1','bat','cmd',
      'sql','graphql','gql','proto','dockerfile','gitignore','env',
      'lock','diff','patch','r','lua','m','pl','ex','exs',
    ]).has(ext);
  }

  private looksLikeText(content: string): boolean {
    const sample = content.slice(0, 1024);
    let ctrl = 0;
    for (let i = 0; i < sample.length; i++) {
      const code = sample.charCodeAt(i);
      if (code < 9 || (code > 13 && code < 32)) ctrl++;
    }
    return sample.length === 0 || ctrl / sample.length < 0.05;
  }

  private syntaxHighlightJson(json: string): string {
    return json
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(
        /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
        match => {
          if (/^"/.test(match))
            return match.endsWith(':')
              ? `<span class="json-key">${match}</span>`
              : `<span class="json-str">${match}</span>`;
          if (/true|false/.test(match)) return `<span class="json-bool">${match}</span>`;
          if (/null/.test(match))        return `<span class="json-null">${match}</span>`;
          return `<span class="json-num">${match}</span>`;
        }
      );
  }

  private renderMarkdown(md: string): string {
    let html = md
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/```[\w]*\n([\s\S]*?)```/g, (_m, code) => `<pre><code>${code}</code></pre>`)
      .replace(/^###### (.+)$/gm, '<h6>$1</h6>')
      .replace(/^##### (.+)$/gm,  '<h5>$1</h5>')
      .replace(/^#### (.+)$/gm,   '<h4>$1</h4>')
      .replace(/^### (.+)$/gm,    '<h3>$1</h3>')
      .replace(/^## (.+)$/gm,     '<h2>$1</h2>')
      .replace(/^# (.+)$/gm,      '<h1>$1</h1>')
      .replace(/^---+$/gm, '<hr>')
      .replace(/^&gt; (.+)$/gm, '<blockquote>$1</blockquote>')
      .replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      .replace(/\*(.+?)\*/g,     '<em>$1</em>')
      .replace(/__(.+?)__/g,      '<strong>$1</strong>')
      .replace(/_(.+?)_/g,        '<em>$1</em>')
      .replace(/`([^`]+)`/g,      '<code>$1</code>')
      .replace(/~~(.+?)~~/g,      '<del>$1</del>')
      .replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img src="$2" alt="$1">')
      .replace(/\[([^\]]+)\]\(([^)]+)\)/g,  '<a href="$2" target="_blank" rel="noopener">$1</a>')
      .replace(/^\s*[-*+] (.+)$/gm, '<li>$1</li>')
      .replace(/^\s*\d+\. (.+)$/gm, '<li>$1</li>')
      .replace(/(<li>.*<\/li>\n?)+/gs, m => `<ul>${m}</ul>`)
      .replace(/\n{2,}/g, '</p><p>');
    if (!/^<(h[1-6]|ul|ol|pre|blockquote|hr)/.test(html.trim()))
      html = `<p>${html}</p>`;
    return html;
  }

  private renderHTMLDocument(source: string): string {
    const document = /<!doctype/i.test(source) || /<html[\s>]/i.test(source)
      ? source
      : `<!doctype html>
<html>
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
      body {
        margin: 16px;
        font-family: Segoe UI, system-ui, sans-serif;
        line-height: 1.5;
      }
    </style>
  </head>
  <body>${source}</body>
</html>`;

    return withViewerFrameScrollbars(document);
  }
}
