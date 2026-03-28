import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  Input,
  OnChanges,
  OnDestroy,
} from '@angular/core';
import { VscodeScrollbarDirective } from '../../directives/vscode-scrollbar.directive';
import { ApiService } from '../../services/api.service';
import { FileInfo, FileProbe, FileType } from '../../shared/protocol';
import { ViewerArchiveContext } from '../viewer-archive-context';

type ImageViewerKind = 'empty' | 'loading' | 'image' | 'svg' | 'directory' | 'toobig' | 'unsupported' | 'error';
interface ImageMeta { width: number; height: number }

@Component({
  selector: 'app-image-viewer',
  standalone: true,
  imports: [CommonModule, VscodeScrollbarDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <div class="viewer-header">
        <div class="header-main">
          <span class="file-icon">&#x1F5BC;</span>
          <div class="header-text">
            <div class="title" [title]="displayName">{{ displayName || 'Image Preview' }}</div>
            <div class="path" [title]="displayPath">{{ displayPath || 'Select a file to preview' }}</div>
          </div>
        </div>
        <div class="header-meta" *ngIf="info && kind !== 'empty' && kind !== 'loading'">
          <span class="badge archive-badge" *ngIf="archiveContext" title="Extracted from archive into memory">
            &#x1F4E6; archive
          </span>
          <span class="badge type-badge">{{ typeBadge }}</span>
          <span class="badge size-badge" *ngIf="info.size > 0">{{ formatSize(info.size) }}</span>
          <span class="badge dim-badge" *ngIf="imageMeta">{{ imageMeta.width }}&#xD7;{{ imageMeta.height }}</span>
        </div>
      </div>

      <div class="viewer-toolbar" *ngIf="kind === 'image' || kind === 'svg'">
        <button class="tool-btn" type="button" (click)="zoomOut()" title="Zoom out">&#x2212;</button>
        <span class="zoom-label">{{ zoomLabel }}</span>
        <button class="tool-btn" type="button" (click)="zoomIn()" title="Zoom in">+</button>
        <button class="tool-btn" type="button" (click)="resetZoom()" title="Fit to window">Fit</button>
        <button class="tool-btn" type="button" (click)="zoom100()" title="Actual size">1:1</button>
        <span class="toolbar-divider"></span>
        <button class="tool-btn" type="button" (click)="toggleCheckerboard()" [class.active]="showCheckerboard"
                title="Toggle transparency grid">Grid</button>
      </div>

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
          <div class="state-label">Directory - no preview available</div>
        </div>

        <div *ngIf="kind === 'toobig'" class="state-container">
          <div class="state-icon">&#x1F4E6;</div>
          <div class="state-label">Image too large to preview</div>
          <div class="state-sub">{{ formatSize(info?.size ?? 0) }}</div>
        </div>

        <div *ngIf="kind === 'unsupported'" class="state-container">
          <div class="state-icon">&#x1F6AB;</div>
          <div class="state-label">No image preview for this file type</div>
          <div class="state-sub" *ngIf="fileExt">.{{ fileExt }}</div>
        </div>

        <div *ngIf="kind === 'svg'" class="svg-host" [class.checkerboard]="showCheckerboard" appVscodeScrollbar>
          <img class="preview-image"
               [src]="imageSrc!"
               [alt]="displayName || 'SVG preview'"
               [style.transform]="imageTransform"
               (load)="onImageLoad($event)"
               (error)="onImageError()" />
        </div>

        <div *ngIf="kind === 'image'" class="image-host" [class.checkerboard]="showCheckerboard" appVscodeScrollbar>
          <img class="preview-image"
               [src]="imageSrc!"
               [alt]="displayName || 'Image preview'"
               [style.transform]="imageTransform"
           (load)="onImageLoad($event)"
           (error)="onImageError()" />
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }
    .viewer-surface {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-code-background, var(--vsc-editor-background, #1e1e1e));
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
    .archive-badge {
      background: color-mix(in srgb, var(--vsc-accent-orange, #f90) 20%, transparent);
      color: var(--vsc-accent-orange, #e8c040);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-orange, #f90) 40%, transparent);
    }
    .type-badge {
      background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 20%, transparent);
      color: var(--vsc-accent-blue, #4da8ff);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 40%, transparent);
    }
    .size-badge,
    .dim-badge {
      background: color-mix(in srgb, var(--vsc-foreground, #ccc) 8%, transparent);
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 60%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 50%, transparent);
    }
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
      font: inherit;
      font-size: 12px;
      background: transparent;
      color: var(--vsc-foreground, #ccc);
      border: 1px solid transparent;
      padding: 2px 8px;
      border-radius: 3px;
      cursor: pointer;
      line-height: 1.4;
      min-width: 24px;
      transition: background 0.1s, border-color 0.1s;
    }
    .tool-btn:hover {
      background: var(--vsc-list-hover-background, rgba(255,255,255,0.05));
      border-color: var(--vsc-border, #454545);
    }
    .tool-btn.active {
      background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 20%, transparent);
      border-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 50%, transparent);
      color: var(--vsc-accent-blue, #4da8ff);
    }
    .zoom-label {
      font-size: 11px;
      opacity: 0.85;
      min-width: 44px;
      text-align: center;
      font-variant-numeric: tabular-nums;
    }
    .toolbar-divider {
      width: 1px;
      height: 16px;
      background: var(--vsc-border, #454545);
      margin: 0 4px;
    }
    .viewer-body { flex: 1 1 auto; min-height: 0; position: relative; overflow: hidden; }
    .state-container {
      height: 100%;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 10px;
      color: var(--vsc-foreground, #ccc);
      user-select: none;
      padding: 20px;
      text-align: center;
      opacity: 0.85;
    }
    .state-icon { font-size: 22px; }
    .state-label { font-size: 13px; font-weight: 600; }
    .state-sub { font-size: 12px; opacity: 0.8; }
    .state-error { color: color-mix(in srgb, var(--vsc-accent-red, #f14c4c) 70%, var(--vsc-foreground, #ccc)); }
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
      border-radius: 50%;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 25%, transparent);
      border-top-color: var(--vsc-accent-blue, #0078d4);
      animation: spin 0.65s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
    .checkerboard {
      background-image:
        linear-gradient(45deg, rgba(255,255,255,0.06) 25%, transparent 25%),
        linear-gradient(-45deg, rgba(255,255,255,0.06) 25%, transparent 25%),
        linear-gradient(45deg, transparent 75%, rgba(255,255,255,0.06) 75%),
        linear-gradient(-45deg, transparent 75%, rgba(255,255,255,0.06) 75%);
      background-size: 18px 18px;
      background-position: 0 0, 0 9px, 9px -9px, -9px 0px;
    }
    .image-host, .svg-host {
      height: 100%;
      overflow: auto;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 18px;
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 96%, black);
    }
    .preview-image {
      max-width: 100%;
      max-height: 100%;
      transform-origin: center;
      box-shadow: 0 18px 48px rgba(0, 0, 0, 0.28);
      border-radius: 6px;
    }
    .svg-wrap {
      transform-origin: center;
      box-shadow: 0 18px 48px rgba(0, 0, 0, 0.22);
      border-radius: 6px;
    }
  `],
})
export class ImageViewerComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;
  @Input() probe: FileProbe | null = null;

  kind: ImageViewerKind = 'empty';
  error: string | null = null;
  info: FileInfo | null = null;

  imageSrc: string | null = null;
  imageMeta: ImageMeta | null = null;

  zoomLevel = 0;
  showCheckerboard = false;

  private destroyed = false;

  // With streaming support, no size limits needed
  private readonly MAX_IMAGE_BYTES = Infinity;

  constructor(
    private api: ApiService,
    private cdr: ChangeDetectorRef,
  ) {}

  get displayName(): string {
    if (this.info?.name) return this.info.name;
    if (this.archiveContext?.entryPath) {
      return this.archiveContext.entryPath.replace(/\\/g, '/').split('/').pop() ?? '';
    }
    return this.targetPath ? this.baseName(this.targetPath) : '';
  }

  get displayPath(): string {
    if (this.archiveContext) {
      return `${this.archiveContext.fsPath} :: ${this.archiveContext.entryPath}`;
    }
    return this.targetPath;
  }

  get fileExt(): string {
    return this.extractExt(this.archiveContext?.entryPath ?? this.targetPath);
  }

  get typeBadge(): string {
    const ext = this.fileExt;
    return ext ? ext.toUpperCase() : 'FILE';
  }

  get imageTransform(): string {
    return this.zoomLevel === 0 ? 'none' : `scale(${this.zoomLevel})`;
  }

  get zoomLabel(): string {
    return this.zoomLevel === 0 ? 'Fit' : `${Math.round(this.zoomLevel * 100)}%`;
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
      this.kind = 'error';
    }

    this.cdr.markForCheck();
  }

  ngOnDestroy(): void {
    this.destroyed = true;
  }

  zoomIn(): void { this.zoomLevel = this.zoomLevel === 0 ? 1.25 : Math.min(this.zoomLevel * 1.25, 8); this.cdr.markForCheck(); }
  zoomOut(): void { this.zoomLevel = this.zoomLevel === 0 ? 0.8 : Math.max(this.zoomLevel * 0.8, 0.1); this.cdr.markForCheck(); }
  resetZoom(): void { this.zoomLevel = 0; this.cdr.markForCheck(); }
  zoom100(): void { this.zoomLevel = 1; this.cdr.markForCheck(); }
  toggleCheckerboard(): void { this.showCheckerboard = !this.showCheckerboard; this.cdr.markForCheck(); }

  onImageLoad(event: Event): void {
    const img = event.target as HTMLImageElement;
    this.imageMeta = { width: img.naturalWidth, height: img.naturalHeight };
    this.cdr.markForCheck();
  }

  onImageError(): void {
    this.error = 'Image failed to render.';
    this.kind = 'error';
    this.cdr.markForCheck();
  }

  private async loadFromDisk(path: string): Promise<void> {
    const info = await this.api.getFileInfo(path);
    this.info = info;
    if (info.type === FileType.DIRECTORY) { this.kind = 'directory'; return; }

    const ext = this.extractExt(path);
    if (ext === 'svg') {
      this.imageSrc = await this.api.getStreamURL(path);
      this.kind = 'svg';
      return;
    }

    const mime = this.inferImageMime(ext);
    if (mime) {
      try {
        this.imageSrc = await this.api.getStreamURL(path);
        this.kind = 'image';
      } catch (err) {
        this.error = err instanceof Error ? err.message : 'Failed to load image';
        this.kind = 'error';
      }
      return;
    }

    // Try backend conversion for unsupported formats
    try {
      await this.convertImageAndDisplay(path, ext);
    } catch (err) {
      this.error = err instanceof Error ? err.message : 'Unsupported image format';
      this.kind = 'unsupported';
    }
  }

  private async loadFromArchive(ctx: ViewerArchiveContext): Promise<void> {
    const entryName = ctx.entryPath.replace(/\\/g, '/').split('/').pop() ?? ctx.entryPath;
    const ext = this.extractExt(ctx.entryPath);
    const streamURL = await this.api.getArchiveStreamURL(ctx.fsPath, ctx.nestedChain, ctx.entryPath);
    this.info = this.makeSyntheticInfo(entryName, ctx.entryPath, 0);

    if (ext === 'svg') {
      this.imageSrc = streamURL;
      this.kind = 'svg';
      return;
    }

    const mime = this.inferImageMime(ext);
    if (mime) {
      try {
        this.imageSrc = streamURL;
        this.kind = 'image';
      } catch (err) {
        this.error = err instanceof Error ? err.message : 'Failed to load image from archive';
        this.kind = 'error';
      }
      return;
    }

    // Archive conversion not supported yet (would require temp extraction)
    this.error = `Format ${ext.toUpperCase()} in archives is not yet supported`;
    this.kind = 'unsupported';
  }

  private async convertImageAndDisplay(path: string, ext: string): Promise<void> {
    const canConvert = await (window as any).go.app.App.CanConvertImage(ext);
    if (!canConvert) {
      throw new Error(`Format .${ext} cannot be converted`);
    }

    this.imageSrc = await this.api.getConvertedImageURL(path, 'png');
    this.kind = 'image';
  }

  private resetState(): void {
    this.error = null;
    this.info = null;
    this.imageSrc = null;
    this.imageMeta = null;
    // Keep zoom/checkerboard as user-set knobs.
  }

  private inferImageMime(ext: string): string | null {
    const map: Record<string, string> = {
      png: 'image/png',
      jpg: 'image/jpeg',
      jpeg: 'image/jpeg',
      gif: 'image/gif',
      webp: 'image/webp',
      bmp: 'image/bmp',
      ico: 'image/x-icon',
      avif: 'image/avif',
    };
    return map[ext] ?? null;
  }
  private extractExt(path: string): string {
    const p = (path ?? '').replace(/\\/g, '/');
    const last = p.split('/').pop() ?? p;
    const i = last.lastIndexOf('.');
    return i >= 0 ? last.slice(i + 1).toLowerCase() : '';
  }

  private baseName(path: string): string {
    const p = (path ?? '').replace(/\\/g, '/');
    return p.split('/').pop() ?? p;
  }

  private makeSyntheticInfo(name: string, path: string, size: number): FileInfo {
    return {
      name,
      path,
      type: FileType.FILE,
      size,
      modified: 0,
      created: 0,
      accessed: 0,
      permissions: '-rw-r--r--',
      isHidden: false,
    };
  }

  formatSize(bytes: number): string {
    if (!bytes || bytes <= 0) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const v = bytes / Math.pow(1024, i);
    return `${i === 0 ? v : v.toFixed(1)} ${units[i]}`;
  }
}

