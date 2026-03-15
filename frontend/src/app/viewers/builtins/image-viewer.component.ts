import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  Input,
  OnChanges,
  OnDestroy,
} from '@angular/core';
import { ApiService } from '../../services/api.service';
import { FileInfo, FileType } from '../../shared/protocol';
import { ViewerArchiveContext } from '../../components/viewer-surface.component';

type ImageViewerKind = 'empty' | 'loading' | 'image' | 'svg' | 'directory' | 'toobig' | 'unsupported' | 'error';
interface ImageMeta { width: number; height: number }

@Component({
  selector: 'app-image-viewer',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <div class="viewer-header">
        <div class="header-main">
          <span class="file-icon">&#x1F5BC;</span>
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
        </div>
      </div>

      <div class="viewer-toolbar" *ngIf="kind === 'image' || kind === 'svg'">
        <button class="tool-btn" (click)="zoomOut()" title="Zoom out">&#x2212;</button>
        <span class="zoom-label">{{ zoomLabel }}</span>
        <button class="tool-btn" (click)="zoomIn()" title="Zoom in">+</button>
        <button class="tool-btn" (click)="resetZoom()" title="Fit to window">Fit</button>
        <button class="tool-btn" (click)="zoom100()" title="Actual size">1:1</button>
        <span class="toolbar-divider"></span>
        <button class="tool-btn" (click)="toggleCheckerboard()" [class.active]="showCheckerboard"
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
          <div class="state-label">Directory вЂ” no preview available</div>
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

        <div *ngIf="kind === 'svg'" class="svg-host" [class.checkerboard]="showCheckerboard">
          <div class="svg-wrap" [style.transform]="imageTransform" [innerHTML]="svgContent"></div>
        </div>

        <div *ngIf="kind === 'image'" class="image-host" [class.checkerboard]="showCheckerboard">
          <img class="preview-image"
               [src]="imageSrc!"
               [alt]="displayName || 'Image preview'"
               [style.transform]="imageTransform"
               (load)="onImageLoad($event)" />
        </div>
      </div>
    </div>
  `,
  styles: [`
    .viewer-surface { display: flex; flex-direction: column; height: 100%; min-height: 0; }
    .viewer-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      border-bottom: 1px solid var(--vsc-border, #333);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 92%, #000);
    }
    .header-main { display: flex; align-items: center; gap: 10px; min-width: 0; }
    .file-icon { font-size: 18px; opacity: 0.9; }
    .header-text { min-width: 0; }
    .title { font-weight: 600; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .path { font-size: 11px; opacity: 0.75; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .header-meta { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; justify-content: flex-end; }
    .badge { font-size: 10px; padding: 2px 6px; border: 1px solid var(--vsc-border, #444); border-radius: 999px; opacity: 0.9; }
    .archive-badge { border-color: color-mix(in srgb, var(--vsc-accent-orange, #f90) 60%, var(--vsc-border, #444)); }
    .viewer-toolbar {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 6px 10px;
      border-bottom: 1px solid var(--vsc-border, #333);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 95%, #000);
    }
    .tool-btn {
      font: inherit;
      font-size: 12px;
      padding: 4px 8px;
      background: transparent;
      color: var(--vsc-foreground, #ccc);
      border: 1px solid var(--vsc-border, #444);
      border-radius: 6px;
      cursor: pointer;
    }
    .tool-btn.active { background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 20%, transparent); }
    .zoom-label { font-size: 11px; opacity: 0.85; min-width: 44px; text-align: center; }
    .toolbar-divider { width: 1px; height: 18px; background: var(--vsc-border, #333); margin: 0 4px; opacity: 0.9; }
    .viewer-body { flex: 1 1 auto; min-height: 0; position: relative; overflow: hidden; }
    .state-container {
      height: 100%;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 10px;
      color: var(--vsc-foreground, #ccc);
      opacity: 0.85;
      user-select: none;
      padding: 20px;
      text-align: center;
    }
    .state-icon { font-size: 22px; }
    .state-label { font-size: 13px; font-weight: 600; }
    .state-sub { font-size: 12px; opacity: 0.8; }
    .state-error { color: color-mix(in srgb, var(--vsc-accent-red, #f14c4c) 70%, var(--vsc-foreground, #ccc)); }
    .error-detail { font-size: 12px; opacity: 0.9; max-width: 680px; }
    .spinner {
      width: 22px; height: 22px;
      border-radius: 50%;
      border: 2px solid color-mix(in srgb, var(--vsc-foreground, #ccc) 20%, transparent);
      border-top-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 80%, transparent);
      animation: spin 0.9s linear infinite;
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
    }
    .preview-image { max-width: 100%; max-height: 100%; transform-origin: center; }
    .svg-wrap { transform-origin: center; }
  `],
})
export class ImageViewerComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;

  kind: ImageViewerKind = 'empty';
  error: string | null = null;
  info: FileInfo | null = null;

  imageSrc: string | null = null;
  svgContent: string | null = null;
  imageMeta: ImageMeta | null = null;

  zoomLevel = 0;
  showCheckerboard = false;

  private destroyed = false;

  private readonly MAX_IMAGE_BYTES = 20 * 1024 * 1024; // 20 MB

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

  private async loadFromDisk(path: string): Promise<void> {
    const info = await this.api.getFileInfo(path);
    this.info = info;
    if (info.type === FileType.DIRECTORY) { this.kind = 'directory'; return; }

    const ext = this.extractExt(path);
    if (ext === 'svg') {
      if (info.size > this.MAX_IMAGE_BYTES) { this.kind = 'toobig'; return; }
      const r = await this.api.readFile(path, 'utf8');
      const text = r?.content ?? null;
      if (text === null) { this.kind = 'unsupported'; return; }
      this.svgContent = this.sanitizeSvg(text);
      this.kind = 'svg';
      return;
    }

    const mime = this.inferImageMime(ext);
    if (!mime) { this.kind = 'unsupported'; return; }
    if (info.size > this.MAX_IMAGE_BYTES) { this.kind = 'toobig'; return; }

    const r = await this.api.readFile(path, 'base64');
    const raw = r?.content ?? '';
    if (!raw) { this.kind = 'unsupported'; return; }
    this.imageSrc = `data:${mime};base64,${raw}`;
    this.kind = 'image';
  }

  private async loadFromArchive(ctx: ViewerArchiveContext): Promise<void> {
    const entryName = ctx.entryPath.replace(/\\/g, '/').split('/').pop() ?? ctx.entryPath;
    const ext = this.extractExt(ctx.entryPath);

    if (ext === 'svg') {
      const r = await this.api.archiveRead(ctx.fsPath, ctx.nestedChain, ctx.entryPath, 'utf8');
      if (!r) { this.kind = 'unsupported'; return; }
      this.info = this.makeSyntheticInfo(entryName, ctx.entryPath, r.size ?? 0);
      if ((r.size ?? 0) > this.MAX_IMAGE_BYTES) { this.kind = 'toobig'; return; }
      this.svgContent = this.sanitizeSvg(r.content ?? '');
      this.kind = 'svg';
      return;
    }

    const mime = this.inferImageMime(ext);
    if (!mime) { this.kind = 'unsupported'; return; }
    const r = await this.api.archiveRead(ctx.fsPath, ctx.nestedChain, ctx.entryPath, 'base64');
    if (!r) { this.kind = 'unsupported'; return; }
    this.info = this.makeSyntheticInfo(entryName, ctx.entryPath, r.size ?? 0);
    if ((r.size ?? 0) > this.MAX_IMAGE_BYTES) { this.kind = 'toobig'; return; }
    const raw = r.content ?? '';
    if (!raw) { this.kind = 'unsupported'; return; }
    this.imageSrc = `data:${mime};base64,${raw}`;
    this.kind = 'image';
  }

  private resetState(): void {
    this.error = null;
    this.info = null;
    this.imageSrc = null;
    this.svgContent = null;
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

  private sanitizeSvg(text: string): string {
    return (text ?? '')
      .replace(/<\?xml[^>]*\?>/gi, '')
      .replace(/<!DOCTYPE[^>]*>/gi, '')
      .trim();
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

