import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  Input,
  OnChanges,
  OnDestroy,
  ViewChild,
  ElementRef,
} from '@angular/core';
import { ApiService } from '../../services/api.service';
import { ArchiveEntry, FileInfo, FileProbe, FileType } from '../../shared/protocol';
import type { ViewerFileListItem } from '../../shared/protocol-enhanced';
import { ViewerArchiveContext } from '../viewer-archive-context';
import { VIEWER_BASE_STYLES, VIEWER_PRODUCTION_CHROME_STYLES } from './builtin-viewer.shared';

type ImageViewerKind = 'empty' | 'loading' | 'image' | 'svg' | 'directory' | 'toobig' | 'unsupported' | 'error';

interface ImageMeta {
  width: number;
  height: number;
}

interface CarouselImageItem {
  id: string;
  name: string;
  ext: string;
  source: 'disk' | 'archive';
  diskPath: string | null;
  archiveContext: ViewerArchiveContext | null;
  info: FileInfo | null;
  thumbnailSrc: string | null;
  thumbnailFailed: boolean;
  thumbnailLoading: boolean;
}

interface CarouselVirtualFrame {
  item: CarouselImageItem;
  index: number;
}

@Component({
  selector: 'app-image-viewer',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface"
         #viewerSurface
         tabindex="0"
         (pointerdown)="focusViewerSurface()">
      <div class="viewer-toolbar" role="group" aria-label="Image preview controls" *ngIf="kind === 'image' || kind === 'svg' || hasCarousel">
        <ng-container *ngIf="kind === 'image' || kind === 'svg'">
          <button class="tool-btn" type="button" (click)="zoomOut()" title="Zoom out" aria-label="Zoom out">&#x2212;</button>
          <span class="zoom-label">{{ zoomLabel }}</span>
          <button class="tool-btn" type="button" (click)="zoomIn()" title="Zoom in" aria-label="Zoom in">+</button>
          <button class="tool-btn" type="button" (click)="resetZoom()" title="Fit to window">Fit</button>
          <button class="tool-btn" type="button" (click)="zoom100()" title="Actual size">1:1</button>
          <span class="toolbar-divider"></span>
          <button class="tool-btn" type="button" (click)="toggleCheckerboard()" [class.active]="showCheckerboard" [attr.aria-pressed]="showCheckerboard"
                  title="Toggle transparency grid">Grid</button>
        </ng-container>

        <ng-container *ngIf="hasCarousel">
          <span class="toolbar-divider" *ngIf="kind === 'image' || kind === 'svg'"></span>
          <button class="tool-btn" type="button" (click)="showPrevious()" [disabled]="!hasPrevious" title="Previous image" aria-label="Previous image">&#x2039;</button>
          <span class="carousel-counter">{{ previewCenterIndex + 1 }} / {{ carouselItems.length }}</span>
          <button class="tool-btn" type="button" (click)="showNext()" [disabled]="!hasNext" title="Next image" aria-label="Next image">&#x203A;</button>
        </ng-container>
      </div>

      <div class="viewer-body">
        <div class="viewer-main">
          <div *ngIf="kind === 'loading'" class="state-container" role="status">
            <div class="spinner"></div>
            <div class="state-label">Loading preview&#x2026;</div>
          </div>

          <div *ngIf="kind === 'empty'" class="state-container">
            <div class="state-icon" aria-hidden="true">&#x1F50D;</div>
            <div class="state-label">Select a file to preview</div>
          </div>

          <div *ngIf="kind === 'error'" class="state-container state-error" role="alert">
            <div class="state-icon" aria-hidden="true">&#x26A0;</div>
            <div class="state-label">Preview failed</div>
            <div class="error-detail">{{ error }}</div>
          </div>

          <div *ngIf="kind === 'directory'" class="state-container">
            <div class="state-icon" aria-hidden="true">&#x1F4C1;</div>
            <div class="state-label">Directory - no preview available</div>
          </div>

          <div *ngIf="kind === 'toobig'" class="state-container">
            <div class="state-icon" aria-hidden="true">&#x1F4E6;</div>
            <div class="state-label">Image too large to preview</div>
            <div class="state-sub">{{ formatSize(info?.size ?? 0) }}</div>
          </div>

          <div *ngIf="kind === 'unsupported'" class="state-container">
            <div class="state-icon" aria-hidden="true">&#x1F6AB;</div>
            <div class="state-label">No image preview for this file type</div>
            <div class="state-sub" *ngIf="fileExt">.{{ fileExt }}</div>
          </div>

          <div *ngIf="kind === 'image' || kind === 'svg'"
               class="image-host"
               [class.checkerboard]="showCheckerboard"
               [class.image-host--busy]="imageSwitchPending"
               #imageHost
               (wheel)="onImageHostWheel($event)"
               (pointerdown)="onImageHostPointerDown($event)"
               (pointermove)="onImageHostPointerMove($event)"
               (pointerup)="onImageHostPointerUp($event)"
               (pointercancel)="onImageHostPointerUp($event)">
            <img class="preview-image"
                 [class.preview-image--ready]="imageLoaded"
                 [src]="imageSrc || undefined"
                 [attr.alt]="kind === 'svg' ? 'SVG preview' : 'Image preview'"
                 [style.transform]="imageTransform"
                 (load)="onImageLoad($event)"
                 (error)="onImageError()" />
            <div *ngIf="imageSwitchPending" class="image-status">Loading next image</div>
          </div>
        </div>

        <div class="carousel-strip" *ngIf="hasCarousel">
          <div class="carousel-list-wrapper">
            <div class="carousel-list" #carouselList>
              <div class="carousel-virtual-spacer" [style.width.px]="virtualLeadingWidth" aria-hidden="true"></div>
              <button
                *ngFor="let frame of visibleCarouselItems; trackBy: trackCarouselItem"
                class="carousel-item"
                type="button"
                [class.active]="frame.index === previewCenterIndex" [attr.aria-current]="frame.index === previewCenterIndex ? 'true' : null"
                [title]="frame.item.name"
                [attr.data-index]="frame.index"
                (click)="selectCarouselIndex(frame.index)">
                <span class="carousel-thumb" [class.carousel-thumb--placeholder]="!frame.item.thumbnailSrc || frame.item.thumbnailFailed">
                  <img *ngIf="frame.item.thumbnailSrc && !frame.item.thumbnailFailed"
                       [src]="frame.item.thumbnailSrc"
                       [alt]="frame.item.name"
                       loading="lazy"
                       (error)="onThumbnailError(frame.item)" />
                  <span *ngIf="!frame.item.thumbnailSrc || frame.item.thumbnailFailed" class="carousel-fallback">{{ thumbnailFallback(frame.item) }}</span>
                </span>
                <span class="carousel-name">{{ frame.item.name }}</span>
              </button>
              <div class="carousel-virtual-spacer" [style.width.px]="virtualTrailingWidth" aria-hidden="true"></div>
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
      background: var(--vsc-code-background, var(--vsc-editor-background, var(--vsc-editor-background)));
      color: var(--vsc-foreground, var(--vsc-foreground));
      outline: none;
    }
    .viewer-surface:focus-visible {
      box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 60%, transparent);
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
      font-size: 10px;
      font-weight: 600;
      padding: 2px 6px;
      border-radius: 3px;
      white-space: nowrap;
      letter-spacing: 0.3px;
    }
    .archive-badge {
      background: color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-accent-orange)) 20%, transparent);
      color: var(--vsc-accent-orange, var(--vsc-warning));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-accent-orange)) 40%, transparent);
    }
    .type-badge {
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 40%, transparent);
    }
    .size-badge,
    .dim-badge {
      background: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 8%, transparent);
      color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 60%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 50%, transparent);
    }
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
      font: inherit;
      font-size: 12px;
      background: transparent;
      color: var(--vsc-foreground, var(--vsc-foreground));
      border: 1px solid transparent;
      padding: 2px 8px;
      border-radius: 3px;
      cursor: pointer;
      line-height: 1.4;
      min-width: 24px;
      transition: background 0.1s, border-color 0.1s;
    }
    .tool-btn:hover:not(:disabled) {
      background: var(--vsc-list-hover-background, var(--vsc-neutral-overlay-subtle));
      border-color: var(--vsc-border, var(--vsc-border));
    }
    .tool-btn:focus-visible,
    .carousel-item:focus-visible {
      outline: none;
      border-color: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 70%, transparent);
      box-shadow: 0 0 0 1px color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 28%, transparent);
    }
    .tool-btn:disabled {
      opacity: 0.4;
      cursor: default;
    }
    .tool-btn.active {
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent);
      border-color: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 50%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
    }
    .zoom-label,
    .carousel-counter {
      font-size: 11px;
      opacity: 0.85;
      min-width: 44px;
      text-align: center;
      font-variant-numeric: tabular-nums;
    }
    .toolbar-divider {
      width: 1px;
      height: 16px;
      background: var(--vsc-border, var(--vsc-border));
      margin: 0 4px;
    }
    .viewer-body {
      display: flex;
      flex-direction: column;
      flex: 1 1 auto;
      min-height: 0;
      overflow: hidden;
    }
    .viewer-main {
      flex: 1 1 auto;
      min-height: 0;
      position: relative;
      overflow: hidden;
    }
    .state-container {
      height: 100%;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 10px;
      color: var(--vsc-foreground, var(--vsc-foreground));
      user-select: none;
      padding: 20px;
      text-align: center;
      opacity: 0.85;
    }
    .state-icon { font-size: 22px; }
    .state-label { font-size: 13px; font-weight: 600; }
    .state-sub { font-size: 12px; opacity: 0.8; }
    .state-error { color: color-mix(in srgb, var(--vsc-accent-red, var(--vsc-error)) 70%, var(--vsc-foreground, var(--vsc-foreground))); }
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
      border-radius: 50%;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 25%, transparent);
      border-top-color: var(--vsc-accent-blue, var(--vsc-accent-blue));
      animation: spin 0.65s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
    .checkerboard {
      background-image:
        linear-gradient(45deg, var(--vsc-neutral-overlay-subtle) 25%, transparent 25%),
        linear-gradient(-45deg, var(--vsc-neutral-overlay-subtle) 25%, transparent 25%),
        linear-gradient(45deg, transparent 75%, var(--vsc-neutral-overlay-subtle) 75%),
        linear-gradient(-45deg, transparent 75%, var(--vsc-neutral-overlay-subtle) 75%);
      background-size: 18px 18px;
      background-position: 0 0, 0 9px, 9px -9px, -9px 0px;
    }
    .image-host {
      height: 100%;
      overflow: hidden;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 18px;
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 96%, black);
      touch-action: none;
      user-select: none;
      position: relative;
      transition: background-color 0.16s ease;
    }
    .image-host--busy {
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 92%, var(--vsc-accent-blue, var(--vsc-accent-blue)) 6%);
    }
    .preview-image {
      max-width: 100%;
      max-height: 100%;
      transform-origin: center;
      box-shadow: 0 18px 48px var(--vsc-scrim-soft);
      border-radius: 6px;
      opacity: 0;
      transition: opacity 0.14s ease;
    }
    .preview-image--ready {
      opacity: 1;
    }
    .image-status {
      position: absolute;
      right: 18px;
      bottom: 18px;
      padding: 6px 10px;
      border-radius: 999px;
      font-size: 11px;
      letter-spacing: 0.02em;
      color: var(--vsc-foreground, var(--vsc-foreground));
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 80%, black);
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 70%, transparent);
      box-shadow: 0 8px 20px var(--vsc-shadow-soft);
    }
    .carousel-strip {
      display: flex;
      padding: 12px 12px 18px;
      border-top: 1px solid var(--vsc-border, var(--vsc-border));
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 94%, black);
      flex-shrink: 0;
      min-height: calc(118px * 1.05);
    }
    .carousel-list-wrapper {
      position: relative;
      flex: 1;
      min-width: 0;
      overflow: hidden;
      display: flex;
      align-items: center;
    }
    .carousel-list {
      display: flex;
      gap: 12px;
      overflow-x: auto;
      overflow-y: hidden;
      min-width: 0;
      padding-bottom: 12px;
      scroll-snap-type: x mandatory;
padding-inline: max(32px, calc((100% - 112px) / 2 + 16px));
      scroll-padding-inline: max(16px, calc((100% - 112px) / 2 + 8px));
      position: relative;
      z-index: 0;
      align-items: flex-start;
    }
    .carousel-virtual-spacer {
      flex: 0 0 auto;
      height: 0;
      width: 0;
    }
    .carousel-item {
      box-sizing: border-box;
      width: 112px;
      min-width: 112px;
      display: flex;
      flex-direction: column;
      gap: 6px;
      padding: 6px;
      border-radius: 8px;
      border: 1px solid transparent;
      background: transparent;
      color: var(--vsc-foreground, var(--vsc-foreground));
      cursor: pointer;
      text-align: left;
      transition: border-color 0.2s ease;
      scroll-snap-align: center;
      align-items: flex-start;
    }
    .carousel-item:hover {
      background: var(--vsc-list-hover-background, var(--vsc-neutral-overlay-subtle));
      border-color: color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 70%, transparent);
    }
    .carousel-item.active {
      border-color: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 90%, transparent);
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 12%, transparent);
      box-shadow: 0 0 0 1px color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 35%, transparent);
    }
    .carousel-item.active .carousel-name {
      color: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 70%, var(--vsc-foreground, var(--vsc-foreground)));
    }
    .carousel-thumb {
      width: 100%;
      aspect-ratio: 4 / 3;
      border-radius: 6px;
      overflow: hidden;
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 70%, transparent);
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 88%, white 3%);
      display: flex;
      align-items: center;
      justify-content: center;
    }
    .carousel-thumb img {
      width: 100%;
      height: 100%;
      object-fit: cover;
      display: block;
    }
    .carousel-thumb--placeholder {
      background:
        linear-gradient(135deg,
          color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 22%, transparent),
          color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 92%, black));
    }
    .carousel-fallback {
      font-size: 20px;
      font-weight: 700;
      letter-spacing: 0.03em;
      opacity: 0.8;
      text-transform: uppercase;
    }
    .carousel-name {
      font-size: 11px;
      line-height: 1.3;
      display: -webkit-box;
      -webkit-box-orient: vertical;
      -webkit-line-clamp: 2;
      overflow: hidden;
      text-overflow: ellipsis;
      opacity: 0.9;
      width: 100%;
    }
  `, VIEWER_PRODUCTION_CHROME_STYLES],
})
export class ImageViewerComponent implements OnChanges, OnDestroy {
  private carouselListElement: HTMLDivElement | null = null;
  private viewerSurfaceElement: HTMLDivElement | null = null;
  private scrollListenerCleanup: (() => void) | null = null;
  private pendingInitialCenterFrame: number | null = null;
  previewCenterIndex = 0;
  private previewCommitTimer: number | null = null;
  private readonly previewCommitDelay = 150;
  private suppressPreviewCommit = false;
  private suppressPreviewCommitTimer: number | null = null;
  private explicitSelectionLockIndex: number | null = null;
  private explicitSelectionLockTimer: number | null = null;
  
  // Pointer gesture properties for natural scrolling
  private pointerDragActive = false;
  private pointerLastY: number | null = null;
  private pointerScrollAccumulator = 0;
  private readonly previewStep = 800; // Reduced from 1000 for slightly more responsive feel
  private pendingPreviewIndex: number | null = null;
  private readonly pointerMaxGestureSteps = 3; // Max 3 items per gesture
  private readonly pointerScrollSensitivity = 0.008; // Increased slightly for better responsiveness
  private pointerGestureCompletedSteps = 0; // Track completed steps in current gesture

  // Wheel-based scrolling properties
  private wheelScrollAccumulator = 0;
  private readonly wheelStepThreshold = 150; // Lower threshold for wheel vs touch
  private wheelDebounceTimer: number | null = null;
  private lastWheelTimestamp = 0;
  private readonly wheelCooldownMs = 100;

  visibleCarouselItems: CarouselVirtualFrame[] = [];
  private virtualStartIndex = 0;
  private virtualEndIndex = 0;
  virtualLeadingWidth = 0;
  virtualTrailingWidth = 0;
  private readonly virtualBuffer = 6;

  @ViewChild('viewerSurface', { static: true })
  set viewerSurfaceRef(value: ElementRef<HTMLDivElement> | undefined) {
    this.viewerSurfaceElement = value?.nativeElement ?? null;
  }

  @ViewChild('carouselList', { static: false })
  set carouselListRef(value: ElementRef<HTMLDivElement> | undefined) {
    const element = value?.nativeElement ?? null;
    if (element === this.carouselListElement) {
      return;
    }
    this.detachCarouselScrollListener();
    this.carouselListElement = element;
    if (element) {
      this.scheduleCenterOnCurrentItem();
      this.attachCarouselScrollListener(element);
    }
  }

  @Input() targetPath = '';
  @Input() fileList: ViewerFileListItem[] = [];
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;
  @Input() probe: FileProbe | null = null;

  kind: ImageViewerKind = 'empty';
  error: string | null = null;
  info: FileInfo | null = null;
  imageSrc: string | null = null;
  imageMeta: ImageMeta | null = null;
  imageLoaded = false;
  imageSwitchPending = false;
  zoomLevel = 0;
  showCheckerboard = false;
  carouselItems: CarouselImageItem[] = [];
  currentCarouselIndex = 0;

  private activeTargetPath = '';
  private activeArchiveContext: ViewerArchiveContext | null = null;
  private destroyed = false;
  private contextToken = 0;
  private displayToken = 0;

  private readonly nativeImageExts = new Set(['svg', 'png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico', 'avif']);
  private readonly convertibleImageExts = new Set(['tiff', 'tif', 'exr', 'heif', 'heic', 'jp2', 'j2k', 'jxl', 'fits', 'hdr', 'pfm']);
  private readonly thumbnailWidth = 160;
  private readonly thumbnailHeight = 120;
  private readonly thumbnailHydrationRadius = 4;
  private readonly carouselItemDefaultWidth = 112;
  private readonly carouselItemGap = 12;
  private deletionInProgress = false;

  constructor(
    private api: ApiService,
    private cdr: ChangeDetectorRef,
  ) {}

  get activeContext(): ViewerArchiveContext | null {
    return this.activeArchiveContext;
  }

  get displayName(): string {
    if (this.info?.name) return this.info.name;
    if (this.activeArchiveContext?.entryPath) return this.baseName(this.activeArchiveContext.entryPath);
    return this.activeTargetPath ? this.baseName(this.activeTargetPath) : '';
  }

  get displayPath(): string {
    if (this.activeArchiveContext) {
      return `${this.activeArchiveContext.fsPath} :: ${this.activeArchiveContext.entryPath}`;
    }
    return this.activeTargetPath;
  }

  get fileExt(): string {
    return this.extractExt(this.activeArchiveContext?.entryPath ?? this.activeTargetPath);
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

  get hasCarousel(): boolean {
    return this.carouselItems.length > 1;
  }

  get hasPrevious(): boolean {
    return this.currentCarouselIndex > 0;
  }

  get hasNext(): boolean {
    return this.currentCarouselIndex >= 0 && this.currentCarouselIndex < this.carouselItems.length - 1;
  }

  async ngOnChanges(): Promise<void> {
    const token = ++this.contextToken;
    this.resetState();

    const path = (this.targetPath ?? '').trim();
    if (!path) {
      this.kind = 'empty';
      this.cdr.markForCheck();
      return;
    }

    if (this.archiveContext) {
      this.kind = 'unsupported';
      this.error = 'Preview inside archives is disabled.';
      this.cdr.markForCheck();
      return;
    }

    this.kind = 'loading';
    this.cdr.markForCheck();

    try {
      const prepared = await this.prepareDiskCarousel(path);

      if (token !== this.contextToken || this.destroyed) {
        return;
      }

      this.carouselItems = prepared.items;
      this.currentCarouselIndex = prepared.index;
      this.previewCenterIndex = prepared.index;
      this.lockExplicitSelection(prepared.index, 400);
      this.seedVirtualWindowAround(prepared.index);
      this.cdr.markForCheck();
      this.scheduleCenterOnCurrentItem();

      await this.displayCarouselIndex(prepared.index, token);
      this.onCarouselScroll();
    } catch (err: any) {
      if (token !== this.contextToken || this.destroyed) {
        return;
      }
      this.error = err?.message || String(err);
      this.kind = 'error';
      this.cdr.markForCheck();
    }
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.detachCarouselScrollListener();
    if (this.pendingInitialCenterFrame !== null) {
      cancelAnimationFrame(this.pendingInitialCenterFrame);
      this.pendingInitialCenterFrame = null;
    }
    this.clearPreviewCommitTimer();
    this.clearSuppressPreviewCommitTimer();
    this.clearExplicitSelectionLockTimer();
    if (this.wheelDebounceTimer) {
      window.clearTimeout(this.wheelDebounceTimer);
      this.wheelDebounceTimer = null;
    }
  }

  private attachCarouselScrollListener(el: HTMLDivElement): void {
    const handler = () => this.onCarouselScroll();
    el.addEventListener('scroll', handler, { passive: true });
    this.scrollListenerCleanup = () => el.removeEventListener('scroll', handler);
    this.onCarouselScroll();
  }

  private detachCarouselScrollListener(): void {
    if (this.scrollListenerCleanup) {
      this.scrollListenerCleanup();
      this.scrollListenerCleanup = null;
    }
  }

  private onCarouselScroll(): void {
    const el = this.carouselListElement;
    if (!el) return;
    this.updateVirtualWindow();
    if (!this.carouselItems.length) {
      return;
    }
    const rangeStart = Math.max(0, this.virtualStartIndex - this.thumbnailHydrationRadius);
    const endCandidate = this.virtualEndIndex - 1 + this.thumbnailHydrationRadius;
    const rangeEnd = Math.min(this.carouselItems.length - 1, endCandidate);
    const visible: number[] = [];
    for (let idx = rangeStart; idx <= rangeEnd; idx++) {
      visible.push(idx);
    }
    this.hydrateThumbnailsForIndices(visible);
    if (this.explicitSelectionLockIndex !== null) {
      return;
    }
    if (this.suppressPreviewCommit) {
      return;
    }
    const centerIndex = this.computeCenterIndex(el);
    this.updatePreviewCenterIndex(centerIndex);
    this.schedulePreviewCommit();
  }

  private schedulePreviewCommit(): void {
    if (this.pointerDragActive) {
      return;
    }
    this.clearPreviewCommitTimer();
    this.previewCommitTimer = window.setTimeout(() => {
      this.previewCommitTimer = null;
      this.commitPreviewSelection();
    }, this.previewCommitDelay);
  }

  private clearPreviewCommitTimer(): void {
    if (this.previewCommitTimer !== null) {
      window.clearTimeout(this.previewCommitTimer);
      this.previewCommitTimer = null;
    }
  }

  private armSuppressPreviewCommit(durationMs: number): void {
    this.suppressPreviewCommit = true;
    this.clearSuppressPreviewCommitTimer();
    this.suppressPreviewCommitTimer = window.setTimeout(() => {
      this.suppressPreviewCommit = false;
      this.suppressPreviewCommitTimer = null;
    }, durationMs);
  }

  private clearSuppressPreviewCommitTimer(): void {
    if (this.suppressPreviewCommitTimer !== null) {
      window.clearTimeout(this.suppressPreviewCommitTimer);
      this.suppressPreviewCommitTimer = null;
    }
  }

  private lockExplicitSelection(index: number, durationMs = 320): void {
    this.explicitSelectionLockIndex = index;
    this.clearExplicitSelectionLockTimer();
    this.explicitSelectionLockTimer = window.setTimeout(() => {
      this.explicitSelectionLockIndex = null;
      this.explicitSelectionLockTimer = null;
    }, durationMs);
  }

  private clearExplicitSelectionLockTimer(): void {
    if (this.explicitSelectionLockTimer !== null) {
      window.clearTimeout(this.explicitSelectionLockTimer);
      this.explicitSelectionLockTimer = null;
    }
  }

  private commitPreviewSelection(): void {
    if (!this.hasCarousel) {
      return;
    }
    const candidate = this.previewCenterIndex;
    if (candidate < 0) {
      return;
    }
    this.centerCarouselItem(candidate, 'smooth');
    if (candidate === this.currentCarouselIndex) {
      return;
    }
    void this.displayCarouselIndex(candidate, this.contextToken);
  }

  private updatePreviewCenterIndex(centerIndex: number): void {
    if (centerIndex < 0 || centerIndex >= this.carouselItems.length) {
      return;
    }
    if (centerIndex === this.previewCenterIndex) {
      return;
    }
    this.previewCenterIndex = centerIndex;
    this.cdr.markForCheck();
  }


  onImageHostWheel(event: WheelEvent): void {
    if (!this.hasCarousel) {
      return;
    }
    event.preventDefault();

    // Use vertical wheel movement to switch the active image.
    let delta = event.deltaY;
    if (delta === 0 || Math.abs(delta) < Math.abs(event.deltaX)) {
      return;
    }

    if (event.deltaMode === 1) {
      delta = delta * 40;
    } else if (event.deltaMode === 2) {
      delta = delta * 200;
    }

    const scaledDelta = delta * 0.5;
    this.wheelScrollAccumulator += scaledDelta;

    const now = Date.now();
    if (now - this.lastWheelTimestamp < this.wheelCooldownMs) {
      return;
    }
    this.lastWheelTimestamp = now;
    
    // Clear previous debounce timer
    if (this.wheelDebounceTimer) {
      window.clearTimeout(this.wheelDebounceTimer);
    }
    
    // Process the accumulated scroll
    this.processWheelScroll();
    
    // Set debounce to process any remaining accumulation after wheel stops
    this.wheelDebounceTimer = window.setTimeout(() => {
      this.processWheelScroll(true); // Force final processing
      this.wheelDebounceTimer = null;
    }, this.wheelCooldownMs);
  }

  private processWheelScroll(isFinal: boolean = false): void {
    if (!this.hasCarousel) return;

    const threshold = isFinal ? this.wheelStepThreshold / 2 : this.wheelStepThreshold;
    const steps = Math.floor(Math.abs(this.wheelScrollAccumulator) / threshold);
    let shouldCommit = false;

    if (steps > 0) {
      const direction = Math.sign(this.wheelScrollAccumulator);
      const maxSteps = Math.min(steps, this.pointerMaxGestureSteps);
      let movedSteps = 0;
      for (let i = 0; i < maxSteps; i++) {
        const moved = this.tryAdvancePreview(direction);
        if (moved) {
          movedSteps++;
        } else {
          break; // Can't move further (at edge)
        }
      }

      this.wheelScrollAccumulator -= direction * movedSteps * threshold;
      shouldCommit = movedSteps > 0;
    }

    if (isFinal && Math.abs(this.wheelScrollAccumulator) > threshold * 0.3) {
      const direction = Math.sign(this.wheelScrollAccumulator);
      shouldCommit = this.tryAdvancePreview(direction) || shouldCommit;
      this.wheelScrollAccumulator = 0;
    }

    if (shouldCommit && this.pendingPreviewIndex !== null && this.pendingPreviewIndex !== this.currentCarouselIndex) {
      this.commitPreviewSelection();
      this.pendingPreviewIndex = null;
    }

    if (Math.abs(this.wheelScrollAccumulator) < threshold * 0.1) {
      this.wheelScrollAccumulator = 0;
    }
  }

  private centerCarouselItem(index: number, behavior: ScrollBehavior = 'auto'): void {
    const el = this.carouselListElement;
    if (!el) {
      return;
    }
    this.armSuppressPreviewCommit(behavior === 'smooth' ? 280 : 80);
    const itemEl = el.querySelector<HTMLElement>(`.carousel-item[data-index="${index}"]`);
    if (itemEl) {
      const itemCenter = itemEl.offsetLeft + itemEl.offsetWidth / 2;
      const targetLeft = itemCenter - el.clientWidth / 2;
      const nativeMaxScroll = Math.max(0, el.scrollWidth - el.clientWidth);
      const clampedLeft = Math.max(0, Math.min(targetLeft, nativeMaxScroll));
      el.scrollTo({ left: clampedLeft, behavior });
      return;
    }

    el.scrollTo({ left: this.getAlignedScrollLeftForIndex(el, index), behavior });
  }

  private getCarouselItemStride(): number {
    return this.carouselItemDefaultWidth + this.carouselItemGap;
  }

  private getCarouselPaddingLeft(el: HTMLDivElement): number {
    const style = getComputedStyle(el);
    return parseFloat(style.paddingInlineStart) || parseFloat(style.paddingLeft) || 0;
  }

  private getAlignedScrollLeftForIndex(el: HTMLDivElement, index: number): number {
    const stride = this.getCarouselItemStride();
    const paddingLeft = this.getCarouselPaddingLeft(el);
    const viewportWidth = el.clientWidth;
    const lead = Math.max(0, (viewportWidth - this.carouselItemDefaultWidth) / 2);
    const target = paddingLeft + index * stride - lead;
    const nativeMaxScroll = Math.max(0, el.scrollWidth - viewportWidth);
    return Math.max(0, Math.min(target, nativeMaxScroll));
  }

  private updateVirtualWindow(): void {
    const el = this.carouselListElement;
    const totalItems = this.carouselItems.length;
    if (!el || totalItems === 0) {
      this.visibleCarouselItems = [];
      this.virtualStartIndex = 0;
      this.virtualEndIndex = 0;
      this.virtualLeadingWidth = 0;
      this.virtualTrailingWidth = 0;
      return;
    }
    const stride = this.getCarouselItemStride();
    const paddingLeft = this.getCarouselPaddingLeft(el);
    const effectiveScroll = Math.max(0, el.scrollLeft - paddingLeft);
    const approxIndex = Math.floor(effectiveScroll / stride);
    const start = Math.max(0, approxIndex - this.virtualBuffer);
    const viewportItems = Math.ceil(el.clientWidth / stride) + this.virtualBuffer * 2;
    const end = Math.min(totalItems, start + viewportItems);
    this.virtualStartIndex = start;
    this.virtualEndIndex = end;
    const frames: CarouselVirtualFrame[] = [];
    for (let idx = start; idx < end; idx++) {
      const item = this.carouselItems[idx];
      if (item) {
        frames.push({ item, index: idx });
      }
    }
    this.visibleCarouselItems = frames;
    this.virtualLeadingWidth = start * stride;
    this.virtualTrailingWidth = Math.max(0, (totalItems - end) * stride);
  }

  private computeCenterIndex(el: HTMLDivElement): number {
    if (!this.carouselItems.length) {
      return -1;
    }
    const wrapper = el.parentElement;
    const itemElements = Array.from(el.querySelectorAll<HTMLElement>('.carousel-item[data-index]'));
    if (wrapper && itemElements.length > 0) {
      const wrapperRect = wrapper.getBoundingClientRect();
      const indicatorCenterX = wrapperRect.left + wrapperRect.width / 2;
      let bestIndex = -1;
      let bestDistance = Number.POSITIVE_INFINITY;

      for (const itemEl of itemElements) {
        const rawIndex = Number(itemEl.dataset['index']);
        if (!Number.isFinite(rawIndex)) {
          continue;
        }
        const rect = itemEl.getBoundingClientRect();
        const itemCenterX = rect.left + rect.width / 2;
        const distance = Math.abs(itemCenterX - indicatorCenterX);
        if (distance < bestDistance) {
          bestDistance = distance;
          bestIndex = rawIndex;
        }
      }

      if (bestIndex >= 0) {
        return Math.max(0, Math.min(this.carouselItems.length - 1, bestIndex));
      }
    }

    const stride = this.getCarouselItemStride();
    const paddingLeft = this.getCarouselPaddingLeft(el);
    const effectiveScroll = Math.max(0, el.scrollLeft - paddingLeft);
    const centeredItemOrigin = effectiveScroll + Math.max(0, (el.clientWidth - this.carouselItemDefaultWidth) / 2);
    return Math.max(0, Math.min(this.carouselItems.length - 1, Math.round(centeredItemOrigin / stride)));
  }

  onImageHostPointerDown(event: PointerEvent): void {
    if (event.pointerType === 'mouse') {
      return;
    }
    const host = event.currentTarget as HTMLElement | null;
    if (!host) {
      return;
    }
    this.pointerDragActive = true;
    this.pointerLastY = event.clientY;
    this.pointerScrollAccumulator = 0;
    this.pendingPreviewIndex = this.previewCenterIndex;
    this.pointerGestureCompletedSteps = 0;
    this.focusViewerSurface();
    host.setPointerCapture?.(event.pointerId);
    this.clearPreviewCommitTimer();
    event.preventDefault();
  }

  onImageHostPointerMove(event: PointerEvent): void {
    if (!this.pointerDragActive || this.pointerLastY === null) {
      return;
    }
    const deltaY = this.pointerLastY - event.clientY;
    this.pointerLastY = event.clientY;
    
    // Only accumulate if we haven't reached max steps for this gesture
    if (this.pointerGestureCompletedSteps < this.pointerMaxGestureSteps && Math.abs(deltaY) > 0.2) {
      const scaled = deltaY * this.pointerScrollSensitivity;
      this.pointerScrollAccumulator += scaled;
      this.consumePointerScroll();
    }
    event.preventDefault();
  }

  onImageHostPointerUp(event: PointerEvent): void {
    if (!this.pointerDragActive) {
      return;
    }
    const host = event.currentTarget as HTMLElement | null;
    host?.releasePointerCapture?.(event.pointerId);
    this.pointerDragActive = false;
    this.pointerLastY = null;
    
    // Only commit if we actually changed the preview index
    if (this.pendingPreviewIndex !== null && this.pendingPreviewIndex !== this.currentCarouselIndex) {
      this.commitPreviewSelection();
    }
    
    this.pointerScrollAccumulator = 0;
    this.pendingPreviewIndex = null;
    this.pointerGestureCompletedSteps = 0;
    event.preventDefault();
  }

  private consumePointerScroll(): void {
    const stepThreshold = this.previewStep;
    // Only consume if we haven't reached max steps for this gesture
    while (Math.abs(this.pointerScrollAccumulator) >= stepThreshold && 
           this.pointerGestureCompletedSteps < this.pointerMaxGestureSteps) {
      const direction = Math.sign(this.pointerScrollAccumulator);
      const moved = this.tryAdvancePreview(direction);
      if (moved) {
        this.pointerScrollAccumulator -= direction * stepThreshold;
        this.pointerGestureCompletedSteps++;
      } else {
        // Can't move further (at edge of carousel), break out
        break;
      }
    }
    
    // Reset accumulator if we've reached max steps to prevent further movement
    if (this.pointerGestureCompletedSteps >= this.pointerMaxGestureSteps) {
      this.pointerScrollAccumulator = 0;
    }
  }

  private tryAdvancePreview(direction: number): boolean {
    if (!this.hasCarousel || direction === 0) {
      return false;
    }
    const length = this.carouselItems.length;
    const current = this.pendingPreviewIndex ?? this.previewCenterIndex;
    const nextIndex = Math.max(0, Math.min(length - 1, current + direction));
    if (nextIndex === current) {
      return false; // At edge, couldn't move
    }
    this.pendingPreviewIndex = nextIndex;
    this.previewCenterIndex = nextIndex;
    this.centerCarouselItem(nextIndex);
    this.cdr.markForCheck();
    return true; // Successfully moved
  }


  private canDeleteCurrentTarget(): boolean {
    const selectedItem = this.carouselItems[this.previewCenterIndex] ?? null;
    return !!selectedItem?.diskPath && !selectedItem.archiveContext && !this.deletionInProgress;
  }

  private async deleteCurrentTarget(): Promise<void> {
    if (!this.canDeleteCurrentTarget()) {
      return;
    }
    const selectedItem = this.carouselItems[this.previewCenterIndex] ?? null;
    const targetPath = selectedItem?.diskPath ?? '';
    if (!targetPath) {
      return;
    }
    const currentIndex = this.carouselItems.findIndex(item => item.diskPath === targetPath);
    if (currentIndex < 0) {
      return;
    }
    this.deletionInProgress = true;
    try {
      const result = await this.api.batchDelete([targetPath], true, true);
      if (result.failed > 0) {
        const message = result.errors?.[0]?.error ?? 'Delete failed';
        console.error('Image viewer delete failed', message);
        this.error = message;
        this.cdr.markForCheck();
        return;
      }

      this.fileList = this.fileList.filter(item => item.path !== targetPath);
      this.carouselItems.splice(currentIndex, 1);
      if (!this.carouselItems.length) {
        this.resetState();
        this.kind = 'empty';
        this.cdr.markForCheck();
        return;
      }

      const nextIndex = currentIndex < this.carouselItems.length
        ? currentIndex
        : this.carouselItems.length - 1;
      this.currentCarouselIndex = nextIndex;
      this.previewCenterIndex = nextIndex;
      this.seedVirtualWindowAround(nextIndex);
      this.scheduleCenterOnCurrentItem();
      await this.displayCarouselIndex(nextIndex, this.contextToken, { preserveCurrentImage: true });
      this.onCarouselScroll();
    } catch (err: any) {
      console.error('Image viewer delete failed', err);
      this.error = err?.message ?? String(err);
      this.cdr.markForCheck();
    } finally {
      this.deletionInProgress = false;
    }
  }

  private async hydrateThumbnailsForIndices(indices: number[]): Promise<void> {
    const token = this.contextToken;
    const tasks: Array<Promise<void>> = [];
    for (const idx of indices) {
      const item = this.carouselItems[idx];
      if (item) {
        tasks.push(this.hydrateThumbnailUrl(item, token));
      }
    }
    await Promise.all(tasks);
    this.cdr.markForCheck();
  }

  zoomIn(): void {
    this.zoomLevel = this.zoomLevel === 0 ? 1.25 : Math.min(this.zoomLevel * 1.25, 8);
    this.cdr.markForCheck();
  }

  zoomOut(): void {
    this.zoomLevel = this.zoomLevel === 0 ? 0.8 : Math.max(this.zoomLevel * 0.8, 0.1);
    this.cdr.markForCheck();
  }

  resetZoom(): void {
    this.zoomLevel = 0;
    this.cdr.markForCheck();
  }

  zoom100(): void {
    this.zoomLevel = 1;
    this.cdr.markForCheck();
  }

  toggleCheckerboard(): void {
    this.showCheckerboard = !this.showCheckerboard;
    this.cdr.markForCheck();
  }

  onImageLoad(event: Event): void {
    const img = event.target as HTMLImageElement;
    this.imageMeta = { width: img.naturalWidth, height: img.naturalHeight };
    this.imageLoaded = true;
    this.imageSwitchPending = false;
    this.cdr.markForCheck();
  }

  onImageError(): void {
    this.error = 'Image failed to render.';
    this.kind = 'error';
    this.imageLoaded = false;
    this.imageSwitchPending = false;
    this.cdr.markForCheck();
  }

  onThumbnailError(item: CarouselImageItem): void {
    item.thumbnailFailed = true;
    item.thumbnailSrc = null;
    this.cdr.markForCheck();
  }

  thumbnailFallback(item: CarouselImageItem): string {
    const base = item.name.trim();
    return (base[0] ?? '?').toUpperCase();
  }

  trackCarouselItem(_index: number, frame: CarouselVirtualFrame): string {
    return frame.item.id;
  }

  selectCarouselIndex(index: number): void {
    this.clearPreviewCommitTimer();
    this.lockExplicitSelection(index);
    this.previewCenterIndex = index;
    void this.displayCarouselIndex(index, this.contextToken);
  }

  deleteSelected(): void {
    void this.deleteCurrentTarget();
  }


  showPrevious(): void {
    if (!this.hasPrevious) return;
    void this.displayCarouselIndex(this.currentCarouselIndex - 1, this.contextToken);
  }

  showNext(): void {
    if (!this.hasNext) return;
    void this.displayCarouselIndex(this.currentCarouselIndex + 1, this.contextToken);
  }

  formatSize(bytes: number): string {
    if (!bytes || bytes <= 0) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const value = bytes / Math.pow(1024, index);
    return `${index === 0 ? value : value.toFixed(1)} ${units[index]}`;
  }

  private async displayCarouselIndex(
    index: number,
    contextToken: number,
    options?: { preserveCurrentImage?: boolean },
  ): Promise<void> {
    if (index < 0 || index >= this.carouselItems.length) {
      return;
    }

    this.centerCarouselItem(index);

    const item = this.carouselItems[index];
    const displayToken = ++this.displayToken;
    this.currentCarouselIndex = index;
    this.previewCenterIndex = index;
    void this.scheduleThumbnailHydration(contextToken, index);
    this.error = null;
    this.imageLoaded = false;
    this.imageSwitchPending = !!options?.preserveCurrentImage;
    if (!options?.preserveCurrentImage) {
      this.kind = 'loading';
      this.imageSrc = null;
      this.imageMeta = null;
    } else {
      this.imageMeta = null;
    }
    this.cdr.markForCheck();

    try {
      if (item.diskPath) {
        await this.displayDiskItem(item, contextToken, displayToken);
      } else {
        this.error = 'Preview inside archives is disabled.';
        this.kind = 'unsupported';
      }
    } catch (err: any) {
      if (!this.isDisplayCurrent(contextToken, displayToken)) {
        return;
      }
      this.error = err?.message || String(err);
      this.kind = 'error';
      this.cdr.markForCheck();
    }
  }

  private async displayDiskItem(item: CarouselImageItem, contextToken: number, displayToken: number): Promise<void> {
    const path = item.diskPath ?? '';
    const info = item.info ?? await this.api.getFileInfo(path);
    if (!this.isDisplayCurrent(contextToken, displayToken)) {
      return;
    }

    this.info = info;
    this.activeTargetPath = path;
    this.activeArchiveContext = null;

    if (info.type === FileType.DIRECTORY) {
      this.kind = 'directory';
      this.cdr.markForCheck();
      return;
    }

    const ext = item.ext || this.extractExt(path);
    if (ext === 'svg') {
      this.imageSrc = await this.api.getStreamURL(path);
      if (!this.isDisplayCurrent(contextToken, displayToken)) {
        return;
      }
      this.kind = 'svg';
      this.cdr.markForCheck();
      return;
    }

    if (this.inferImageMime(ext)) {
      this.imageSrc = await this.api.getStreamURL(path);
      if (!this.isDisplayCurrent(contextToken, displayToken)) {
        return;
      }
      this.kind = 'image';
      this.cdr.markForCheck();
      return;
    }

    this.imageSrc = await this.api.getConvertedImageURL(path, 'jpeg');
    if (!this.isDisplayCurrent(contextToken, displayToken)) {
      return;
    }
    this.kind = 'image';
    this.cdr.markForCheck();
  }

  private async displayArchiveItem(item: CarouselImageItem, contextToken: number, displayToken: number): Promise<void> {
    const archiveContext = item.archiveContext;
    if (!archiveContext) {
      return;
    }

    this.info = item.info ?? this.makeSyntheticInfo(item.name, archiveContext.entryPath, 0);
    this.activeTargetPath = archiveContext.fsPath;
    this.activeArchiveContext = archiveContext;

    const ext = item.ext || this.extractExt(archiveContext.entryPath);
    if (ext === 'svg') {
      this.imageSrc = await this.api.getArchiveStreamURL(archiveContext.fsPath, archiveContext.nestedChain, archiveContext.entryPath);
      if (!this.isDisplayCurrent(contextToken, displayToken)) {
        return;
      }
      this.kind = 'svg';
      this.cdr.markForCheck();
      return;
    }

    if (this.inferImageMime(ext)) {
      this.imageSrc = await this.api.getArchiveStreamURL(archiveContext.fsPath, archiveContext.nestedChain, archiveContext.entryPath);
      if (!this.isDisplayCurrent(contextToken, displayToken)) {
        return;
      }
      this.kind = 'image';
      this.cdr.markForCheck();
      return;
    }

    this.error = `Format ${ext.toUpperCase()} in archives is not yet supported`;
    this.kind = 'unsupported';
    this.cdr.markForCheck();
  }

  private async prepareDiskCarousel(path: string): Promise<{ items: CarouselImageItem[]; index: number }> {
    const fileListItems = this.buildCarouselItemsFromFileList(path);
    if (fileListItems) {
      return fileListItems;
    }

    const info = await this.api.getFileInfo(path);
    if (info.type === FileType.DIRECTORY) {
      return { items: [this.makeDiskCarouselItem(info)], index: 0 };
    }

    const currentItem = this.makeDiskCarouselItem(info);
    try {
      const parentPath = await this.api.getParentPath(path);
      const listing = await this.api.listDirectory(parentPath, false);
      const items = listing.entries
        .filter(entry => this.isDiskCarouselCandidate(entry))
        .map(entry => this.makeDiskCarouselItem(entry));

      const currentIndex = items.findIndex(item => item.diskPath === path);
      if (currentIndex >= 0) {
        return { items, index: currentIndex };
      }
      return { items: [currentItem, ...items], index: 0 };
    } catch {
      return { items: [currentItem], index: 0 };
    }
  }

  private seedVirtualWindowAround(index: number): void {
    const totalItems = this.carouselItems.length;
    if (!totalItems) {
      this.visibleCarouselItems = [];
      this.virtualStartIndex = 0;
      this.virtualEndIndex = 0;
      this.virtualLeadingWidth = 0;
      this.virtualTrailingWidth = 0;
      return;
    }

    const visibleCount = Math.min(totalItems, this.virtualBuffer * 2 + 3);
    const start = Math.max(0, Math.min(index - this.virtualBuffer, totalItems - visibleCount));
    const end = Math.min(totalItems, start + visibleCount);
    const stride = this.getCarouselItemStride();
    const frames: CarouselVirtualFrame[] = [];
    for (let idx = start; idx < end; idx++) {
      const item = this.carouselItems[idx];
      if (item) {
        frames.push({ item, index: idx });
      }
    }

    this.virtualStartIndex = start;
    this.virtualEndIndex = end;
    this.visibleCarouselItems = frames;
    this.virtualLeadingWidth = start * stride;
    this.virtualTrailingWidth = Math.max(0, (totalItems - end) * stride);
  }

  private scheduleCenterOnCurrentItem(): void {
    if (this.pendingInitialCenterFrame !== null) {
      cancelAnimationFrame(this.pendingInitialCenterFrame);
      this.pendingInitialCenterFrame = null;
    }
    this.pendingInitialCenterFrame = requestAnimationFrame(() => {
      this.pendingInitialCenterFrame = null;
      if (!this.carouselListElement || !this.carouselItems.length) {
        return;
      }
      this.centerCarouselItem(this.currentCarouselIndex);
    });
  }

  private buildCarouselItemsFromFileList(path: string): { items: CarouselImageItem[]; index: number } | null {
    if (!this.fileList.length) {
      return null;
    }

    const normalizedTarget = path.trim();
    const items = this.fileList
      .filter(item => item.path && this.isPreviewableImageExt(this.extractExt(item.path)))
      .map(item => this.makeDiskCarouselItemFromPath(item.path, item.name));

    if (!items.length) {
      return null;
    }

    const currentIndex = items.findIndex(item => item.diskPath === normalizedTarget);
    if (currentIndex >= 0) {
      return { items, index: currentIndex };
    }

    const targetItem = this.makeDiskCarouselItemFromPath(normalizedTarget);
    return { items: [targetItem, ...items], index: 0 };
  }

  private async prepareArchiveCarousel(ctx: ViewerArchiveContext): Promise<{ items: CarouselImageItem[]; index: number }> {
    const currentItem = this.makeArchiveCarouselItem(ctx, null);
    try {
      const innerPath = this.parentArchivePath(ctx.entryPath);
      const listing = await this.api.archiveBrowse(ctx.fsPath, ctx.nestedChain, innerPath);
      const items = listing.entries
        .filter(entry => this.isArchiveCarouselCandidate(entry))
        .map(entry => this.makeArchiveCarouselItem({
          fsPath: ctx.fsPath,
          nestedChain: ctx.nestedChain,
          entryPath: entry.path,
        }, entry));

      const currentIndex = items.findIndex(item => item.archiveContext?.entryPath === ctx.entryPath);
      if (currentIndex >= 0) {
        return { items, index: currentIndex };
      }
      return { items: [currentItem, ...items], index: 0 };
    } catch {
      return { items: [currentItem], index: 0 };
    }
  }

  private async scheduleThumbnailHydration(contextToken: number, centerIndex: number): Promise<void> {
    const start = Math.max(0, centerIndex - this.thumbnailHydrationRadius);
    const end = Math.min(this.carouselItems.length - 1, centerIndex + this.thumbnailHydrationRadius);
    const indices = [];
    for (let i = start; i <= end; i++) indices.push(i);
    await this.hydrateThumbnailsForIndices(indices);
    this.onCarouselScroll();
  }

  private async hydrateThumbnailUrl(item: CarouselImageItem, contextToken: number): Promise<void> {
    if (item.thumbnailSrc || item.thumbnailFailed || item.thumbnailLoading || this.destroyed || contextToken !== this.contextToken) {
      return;
    }

    item.thumbnailLoading = true;
    try {
      if (item.source === 'archive' && item.archiveContext) {
        item.thumbnailSrc = await this.buildArchiveThumbnailURL(item.archiveContext, item.ext);
      } else if (item.diskPath) {
        item.thumbnailSrc = await this.buildDiskThumbnailURL(item.diskPath, item.ext);
      }
      if (!item.thumbnailSrc) {
        item.thumbnailFailed = true;
      }
    } catch {
      item.thumbnailFailed = true;
      item.thumbnailSrc = null;
    } finally {
      item.thumbnailLoading = false;
    }

    if (contextToken === this.contextToken && !this.destroyed) {
      this.cdr.markForCheck();
    }
  }

  private async buildDiskThumbnailURL(path: string, ext: string): Promise<string | null> {
    if (ext === 'svg') {
      return await this.api.getStreamURL(path);
    }
    try {
      return await this.api.getImageThumbnailURL(path, this.thumbnailWidth, this.thumbnailHeight);
    } catch {
      return null;
    }
  }

  private async buildArchiveThumbnailURL(ctx: ViewerArchiveContext, ext: string): Promise<string | null> {
    if (ext === 'svg') {
      return await this.api.getArchiveStreamURL(ctx.fsPath, ctx.nestedChain, ctx.entryPath);
    }
    try {
      return await this.api.getArchiveImageThumbnailURL(ctx.fsPath, ctx.nestedChain, ctx.entryPath, this.thumbnailWidth, this.thumbnailHeight);
    } catch {
      return null;
    }
  }

  private isDisplayCurrent(contextToken: number, displayToken: number): boolean {
    return !this.destroyed && contextToken === this.contextToken && displayToken === this.displayToken;
  }

  private isDiskCarouselCandidate(entry: FileInfo): boolean {
    if (entry.type === FileType.DIRECTORY || entry.type === FileType.ARCHIVE) {
      return false;
    }
    return this.isPreviewableImageExt(this.extractExt(entry.path || entry.name));
  }

  private isArchiveCarouselCandidate(entry: ArchiveEntry): boolean {
    if (entry.type === FileType.DIRECTORY || entry.type === FileType.ARCHIVE) {
      return false;
    }
    return this.isArchiveDisplayableExt(this.extractExt(entry.path || entry.name));
  }

  private isPreviewableImageExt(ext: string): boolean {
    return this.nativeImageExts.has(ext) || this.convertibleImageExts.has(ext);
  }

  private isArchiveDisplayableExt(ext: string): boolean {
    return ext === 'svg' || !!this.inferImageMime(ext);
  }

  private makeDiskCarouselItem(info: FileInfo): CarouselImageItem {
    const path = info.path;
    return {
      id: `disk:${path}`,
      name: info.name || this.baseName(path),
      ext: this.extractExt(path),
      source: 'disk',
      diskPath: path,
      archiveContext: null,
      info,
      thumbnailSrc: null,
      thumbnailFailed: false,
      thumbnailLoading: false,
    };
  }

  private makeDiskCarouselItemFromPath(path: string, name?: string): CarouselImageItem {
    return {
      id: `disk:${path}`,
      name: name || this.baseName(path),
      ext: this.extractExt(path),
      source: 'disk',
      diskPath: path,
      archiveContext: null,
      info: null,
      thumbnailSrc: null,
      thumbnailFailed: false,
      thumbnailLoading: false,
    };
  }

  private makeArchiveCarouselItem(ctx: ViewerArchiveContext, info: ArchiveEntry | null): CarouselImageItem {
    const name = info?.name || this.baseName(ctx.entryPath);
    return {
      id: `archive:${ctx.fsPath}:${ctx.nestedChain.join('|')}:${ctx.entryPath}`,
      name,
      ext: this.extractExt(ctx.entryPath),
      source: 'archive',
      diskPath: null,
      archiveContext: { fsPath: ctx.fsPath, nestedChain: [...ctx.nestedChain], entryPath: ctx.entryPath },
      info: info ? this.makeSyntheticInfo(name, ctx.entryPath, info.size) : this.makeSyntheticInfo(name, ctx.entryPath, 0),
      thumbnailSrc: null,
      thumbnailFailed: false,
      thumbnailLoading: false,
    };
  }

  private parentArchivePath(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/');
    const index = normalized.lastIndexOf('/');
    return index >= 0 ? normalized.slice(0, index) : '';
  }

  private resetState(): void {
    this.explicitSelectionLockIndex = null;
    this.clearExplicitSelectionLockTimer();
    this.error = null;
    this.info = null;
    this.imageSrc = null;
    this.imageMeta = null;
    this.imageLoaded = false;
    this.imageSwitchPending = false;
    this.kind = 'empty';
    this.carouselItems = [];
    this.currentCarouselIndex = 0;
    this.previewCenterIndex = 0;
    this.activeTargetPath = '';
    this.activeArchiveContext = null;
  }

  focusViewerSurface(): void {
    if (document.activeElement === this.viewerSurfaceElement) {
      return;
    }
    this.viewerSurfaceElement?.focus({ preventScroll: true });
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
    const normalized = (path ?? '').replace(/\\/g, '/');
    const last = normalized.split('/').pop() ?? normalized;
    const index = last.lastIndexOf('.');
    return index >= 0 ? last.slice(index + 1).toLowerCase() : '';
  }

  private baseName(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/');
    return normalized.split('/').pop() ?? normalized;
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
}
