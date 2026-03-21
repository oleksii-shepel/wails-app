import { CommonModule } from '@angular/common';
import {
  Component, Input, OnChanges, OnDestroy,
  ChangeDetectionStrategy, ChangeDetectorRef,
} from '@angular/core';
import { DomSanitizer, SafeResourceUrl } from '@angular/platform-browser';
import { ApiService } from '../services/api.service';
import { FileInfo, FileType } from '../shared/protocol';

type ViewerKind =
  | 'image' | 'video' | 'audio' | 'pdf' | 'svg'
  | 'json' | 'markdown' | 'text'
  | 'directory' | 'toobig' | 'unsupported' | 'error' | 'empty' | 'loading';

interface ImageMeta { width: number; height: number }

//
// When the viewer tab is opened for a file inside an archive, the pane passes
// the current ArchiveFrame stack so the viewer can call ArchiveRead through the
// same VFS-backed archive handler path used by archive browsing.
export interface ViewerArchiveContext {
  /** Outermost archive on disk, e.g. "drive://c/foo/archive.zip" */
  fsPath: string;
  /** Nested archive chain, e.g. ["inner.tar.gz"] — empty for flat archives */
  nestedChain: string[];
  /** Entry path inside the innermost archive, e.g. "src/main.go" */
  entryPath: string;
}

@Component({
  selector: 'app-viewer-surface',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <!-- Header -->
      <div class="viewer-header">
        <div class="header-main">
          <span class="file-icon">{{ fileIcon }}</span>
          <div class="header-text">
            <div class="title" [title]="displayName">{{ displayName || 'Preview' }}</div>
            <div class="path" [title]="targetPath">{{ targetPath || 'Select a file to preview' }}</div>
          </div>
        </div>
        <div class="header-meta" *ngIf="info && kind !== 'empty' && kind !== 'loading'">
          <span class="badge archive-badge" *ngIf="archiveContext" title="Extracted from archive into memory">
            &#x1F4E6; archive
          </span>
          <span class="badge type-badge">{{ typeBadge }}</span>
          <span class="badge size-badge" *ngIf="info.size > 0">{{ formatSize(info.size) }}</span>
          <span class="badge dim-badge" *ngIf="imageMeta">{{ imageMeta.width }}&#xD7;{{ imageMeta.height }}</span>
          <span class="badge line-badge" *ngIf="lineCount !== null">{{ lineCount }} lines</span>
        </div>
      </div>

      <!-- Toolbar -->
      <div class="viewer-toolbar" *ngIf="hasToolbar">
        <ng-container *ngIf="kind === 'image' || kind === 'svg'">
          <button class="tool-btn" (click)="zoomOut()" title="Zoom out">&#x2212;</button>
          <span class="zoom-label">{{ zoomLabel }}</span>
          <button class="tool-btn" (click)="zoomIn()" title="Zoom in">+</button>
          <button class="tool-btn" (click)="resetZoom()" title="Fit to window">Fit</button>
          <button class="tool-btn" (click)="zoom100()" title="Actual size">1:1</button>
          <span class="toolbar-divider"></span>
          <button class="tool-btn" (click)="toggleCheckerboard()" [class.active]="showCheckerboard"
                  title="Toggle transparency grid">Grid</button>
        </ng-container>
        <ng-container *ngIf="kind === 'text' || kind === 'json' || kind === 'markdown'">
          <button class="tool-btn" (click)="toggleWrap()" [class.active]="wordWrap"
                  title="Toggle word wrap">Wrap</button>
        </ng-container>
      </div>

      <!-- Body -->
      <div class="viewer-body">

        <div *ngIf="kind === 'loading'" class="state-container">
          <div class="spinner"></div>
          <div class="state-label">Loading preview&#x2026;</div>
        </div>

        <div *ngIf="kind === 'empty'" class="state-container">
          <div class="state-icon">&#x1F50D;</div>
          <div class="state-label">Select a file to preview</div>
        </div>

        <div *ngIf="kind === 'error'" class="state-container state-error">
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

        <!-- Video -->
        <div *ngIf="kind === 'video'" class="media-host">
          <video class="preview-video" controls preload="auto" playsinline autoplay
                 [src]="mediaSrc!"
                 (loadedmetadata)="onMediaMetadata('video', $event)"
                 (canplay)="onMediaCanPlay('video', $event)"
                 (loadeddata)="onVideoLoadedData($event)"
                 (error)="onMediaError('video', $event)">
            Your browser does not support video.
          </video>
        </div>

        <!-- Audio -->
        <div *ngIf="kind === 'audio'" class="audio-host">
          <div class="audio-icon">&#x1F3B5;</div>
          <div class="state-label">{{ info?.name }}</div>
          <audio class="preview-audio" controls preload="auto" autoplay
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
              <button *ngIf="!archiveContext && targetPath" class="tool-btn" (click)="openCurrentFileExternally()">Open externally</button>
            </div>
          </object>
          <div *ngIf="pdfLoading" class="pdf-loading">
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
        <div *ngIf="kind === 'markdown'"
             class="markdown-body"
             [class.word-wrap]="wordWrap"
             [innerHTML]="renderedMarkdown"></div>

        <!-- Plain text -->
        <pre *ngIf="kind === 'text'"
             class="preview-text"
             [class.word-wrap]="wordWrap">{{ textContent }}</pre>

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

    /* ── Header ─────────────────────────────────────────────────────────── */

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
      background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 20%, transparent);
      color: var(--vsc-accent-blue, #4da8ff);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 40%, transparent);
    }

    .archive-badge {
      background: color-mix(in srgb, var(--vsc-accent-orange, #d4a000) 20%, transparent);
      color: var(--vsc-accent-orange, #e8c040);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-orange, #d4a000) 40%, transparent);
    }

    .size-badge, .dim-badge, .line-badge {
      background: color-mix(in srgb, var(--vsc-foreground, #ccc) 8%, transparent);
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 60%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 50%, transparent);
    }

    /* ── Toolbar ─────────────────────────────────────────────────────────── */

    .viewer-toolbar {
      display: flex;
      align-items: center;
      gap: 2px;
      padding: 4px 8px;
      border-bottom: 1px solid var(--vsc-border, #454545);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 96%, black);
      flex-shrink: 0;
    }

    .tool-btn {
      background: transparent;
      border: 1px solid transparent;
      color: var(--vsc-foreground, #ccc);
      font-size: 12px;
      padding: 2px 8px;
      border-radius: 3px;
      cursor: pointer;
      line-height: 1.4;
      min-width: 24px;
      transition: background 0.1s, border-color 0.1s;

      &:hover {
        background: var(--vsc-list-hover-background, rgba(255,255,255,0.05));
        border-color: var(--vsc-border, #454545);
      }

      &.active {
        background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 20%, transparent);
        border-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 50%, transparent);
        color: var(--vsc-accent-blue, #4da8ff);
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
      background: var(--vsc-border, #454545);
      margin: 0 4px;
    }

    /* ── Body ────────────────────────────────────────────────────────────── */

    .viewer-body {
      flex: 1 1 0%;
      min-height: 0;
      overflow: auto;
      position: relative;
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
    .state-error .state-label { color: #ff7070; opacity: 1; }

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

    /* ── Loading spinner ─────────────────────────────────────────────────── */

    .spinner {
      width: 26px;
      height: 26px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 25%, transparent);
      border-top-color: var(--vsc-accent-blue, #0078d4);
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
        linear-gradient(45deg, rgba(128,128,128,0.15) 25%, transparent 25%),
        linear-gradient(-45deg, rgba(128,128,128,0.15) 25%, transparent 25%),
        linear-gradient(45deg, transparent 75%, rgba(128,128,128,0.15) 75%),
        linear-gradient(-45deg, transparent 75%, rgba(128,128,128,0.15) 75%);
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

    .preview-video { max-width: 100%; max-height: 100%; border-radius: 4px; outline: none; }

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

    .pdf-host { display: flex; flex-direction: column; height: 100%; min-height: 0; background: white; position: relative; }
    .preview-pdf { flex: 1; width: 100%; height: 100%; border: none; background: white; }
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
      color: #222;
      background: rgba(255, 255, 255, 0.92);
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
      color: #222;
      background: white;
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
      color: var(--vsc-foreground, #ccc);

      &.word-wrap { white-space: pre-wrap; word-break: break-word; }
    }

    .json-text {
      .json-key  { color: #9cdcfe; }
      .json-str  { color: #ce9178; }
      .json-num  { color: #b5cea8; }
      .json-bool { color: #569cd6; }
      .json-null { color: #569cd6; }
    }

    .markdown-body {
      padding: 16px 20px;
      line-height: 1.65;
      max-width: 780px;
      font-size: 13px;

      &.word-wrap { word-break: break-word; }

      h1, h2, h3, h4, h5, h6 {
        color: var(--vsc-foreground-bright, #fff);
        margin: 1.2em 0 0.4em;
        font-weight: 600;
        line-height: 1.3;
      }
      h1 { font-size: 1.6em; border-bottom: 1px solid var(--vsc-border, #454545); padding-bottom: 6px; }
      h2 { font-size: 1.3em; border-bottom: 1px solid var(--vsc-border, #454545); padding-bottom: 4px; }
      h3 { font-size: 1.1em; }
      p  { margin: 0.6em 0; }

      a { color: var(--vsc-accent-blue, #4da8ff); text-decoration: none; }
      a:hover { text-decoration: underline; }

      code {
        font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
        font-size: 0.9em;
        background: color-mix(in srgb, var(--vsc-foreground, #ccc) 10%, transparent);
        border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 80%, transparent);
        padding: 1px 5px;
        border-radius: 3px;
      }

      pre {
        background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 70%, black);
        border: 1px solid var(--vsc-border, #454545);
        border-radius: 5px;
        padding: 12px 14px;
        overflow-x: auto;
        margin: 0.8em 0;

        code { background: none; border: none; padding: 0; font-size: 11px; }
      }

      blockquote {
        margin: 0.8em 0;
        padding: 4px 12px;
        border-left: 3px solid var(--vsc-accent-blue, #0078d4);
        opacity: 0.8;
      }

      ul, ol { margin: 0.6em 0; padding-left: 24px; }
      li { margin: 2px 0; }

      table { border-collapse: collapse; margin: 0.8em 0; font-size: 12px; }
      th, td { padding: 5px 12px; border: 1px solid var(--vsc-border, #454545); text-align: left; }
      th { background: color-mix(in srgb, var(--vsc-foreground, #ccc) 8%, transparent); font-weight: 600; }

      hr { border: none; border-top: 1px solid var(--vsc-border, #454545); margin: 1.2em 0; }
      img { max-width: 100%; border-radius: 3px; }
    }
  `]
})
export class ViewerSurfaceComponent implements OnChanges, OnDestroy {
  /**
   * Filesystem path for regular (non-archive) files.
   * When this is a VFS archive path and archiveContext is also set, the
   * context takes precedence for reading content.
   */
  @Input() targetPath = '';

  /**
   * Optional — populated by FilePaneComponent when the viewer tab was opened
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

  // ── State ────────────────────────────────────────────────────────────────

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

  imageMeta:  ImageMeta | null = null;
  lineCount:  number | null    = null;
  zoomLevel   = 0;
  showCheckerboard = false;
  wordWrap = false;
  private mediaProbeTimer: ReturnType<typeof setTimeout> | null = null;
  private lastAutoplayKey: string | null = null;



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
           this.kind === 'text'  || this.kind === 'json' || this.kind === 'markdown';
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
  ) {}

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
    this.clearMediaProbe();
  }

  // ── Toolbar controls ──────────────────────────────────────────────────────

  zoomIn():    void { this.zoomLevel = this.zoomLevel === 0 ? 1.25 : Math.min(this.zoomLevel * 1.25, 8); this.cdr.markForCheck(); }
  zoomOut():   void { this.zoomLevel = this.zoomLevel === 0 ? 0.8  : Math.max(this.zoomLevel * 0.8, 0.1); this.cdr.markForCheck(); }
  resetZoom(): void { this.zoomLevel = 0; this.cdr.markForCheck(); }
  zoom100():   void { this.zoomLevel = 1; this.cdr.markForCheck(); }
  toggleCheckerboard(): void { this.showCheckerboard = !this.showCheckerboard; this.cdr.markForCheck(); }
  toggleWrap():         void { this.wordWrap = !this.wordWrap; this.cdr.markForCheck(); }

  onImageLoad(event: Event): void {
    const img = event.target as HTMLImageElement;
    this.imageMeta = { width: img.naturalWidth, height: img.naturalHeight };
    this.cdr.markForCheck();
  }

  onImageError(): void {
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
    await this.renderContent(ext, info.size, {
      readText: async () => {
        const r = await this.api.readFile(path, 'utf8');
        return r?.content ?? null;
      },
      streamBinary: (_mimeType: string) => {
        // Return a /wails-stream URL — the browser fetches it directly.
        // For disk files this uses http.ServeContent which supports range
        // requests (video seek, partial load) without any JS buffering.
        return Promise.resolve(streamFileURL(path));
      },
    });
  }

  /**
   * Load a file from inside an archive using streaming.
   */
  private async loadFromArchive(ctx: ViewerArchiveContext): Promise<void> {
    const entryName = ctx.entryPath.replace(/\\/g, '/').split('/').pop() ?? ctx.entryPath;
    const ext = this.extractExt(ctx.entryPath);

    await this.renderContent(ext, null, {
      readText: async () => {
        const r = await this.api.archiveRead(ctx.fsPath, ctx.nestedChain, ctx.entryPath, 'utf8');
        if (!r) return null;
        this.info = this.makeSyntheticInfo(entryName, ctx.entryPath, r.size ?? 0);
        return r.content;
      },
      streamBinary: (_mimeType: string) => {
        // Return a /wails-stream URL — the browser fetches the archive entry
        // directly through the Go middleware without any JS buffering.
        return Promise.resolve(streamArchiveURL(ctx.fsPath, ctx.nestedChain, ctx.entryPath));
      },
    });

    if (!this.info) {
      this.info = this.makeSyntheticInfo(entryName, ctx.entryPath, 0);
    }
  }

  /**
   * Core rendering logic — shared between disk and archive paths.
   * Uses streaming for binary content and direct read for text.
   */
  private async renderContent(
    ext: string,
    sizeHint: number | null,
    readers: {
      readText: () => Promise<string | null>;
      streamBinary: (mimeType: string) => Promise<string>;
    },
  ): Promise<void> {
    try {
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

      // ── Video ────────────────────────────────────────────────────────────────
      const videoMime = this.inferVideoMime(ext);
      if (videoMime) {
        if (sizeHint !== null && sizeHint > this.MAX_MEDIA_BYTES) { this.kind = 'toobig'; return; }
        try {
          this.mediaSrc = await readers.streamBinary(videoMime);
          this.mediaMime = videoMime;
          this.kind = 'video';
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
        this.renderedMarkdown = this.renderMarkdown(text);
        this.lineCount = text.split('\n').length;
        this.kind = 'markdown';
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
    this.imageMeta       = null;
    this.lineCount       = null;
    this.zoomLevel       = 0;
    this.showCheckerboard = false;
    this.lastAutoplayKey = null;
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

  private inferImageMime(ext: string): string | null {
    return ({
      png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg',
      gif: 'image/gif', webp: 'image/webp', bmp: 'image/bmp',
      ico: 'image/x-icon', tif: 'image/tiff', tiff: 'image/tiff', avif: 'image/avif',
    } as Record<string, string>)[ext] ?? null;
  }

  private inferVideoMime(ext: string): string | null {
    return ({
      mp4: 'video/mp4', webm: 'video/webm', ogv: 'video/ogg', mov: 'video/quicktime',
      mkv: 'video/x-matroska', avi: 'video/x-msvideo', flv: 'video/x-flv',
      ts: 'video/mp2t', m3u8: 'application/x-mpegURL', mts: 'video/mp2t',
      mxf: 'application/mxf', wmv: 'video/x-ms-wmv',
    } as Record<string, string>)[ext] ?? null;
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
}

// ── Module-level helpers: build /wails-stream URLs ────────────────────────────

/** Returns a /wails-stream URL for a disk file (range-request / seeking supported). */
function streamFileURL(publicPath: string): string {
  return '/wails-stream?path=' + encodeURIComponent(publicPath);
}

/** Returns a /wails-stream URL for a file inside an archive. */
function streamArchiveURL(fsPath: string, nestedChain: string[], entryPath: string): string {
  let url = '/wails-stream?fsPath=' + encodeURIComponent(fsPath)
          + '&entry=' + encodeURIComponent(entryPath);
  if (nestedChain && nestedChain.length > 0) {
    url += '&nested=' + encodeURIComponent(nestedChain.join(','));
  }
  return url;
}