// pane-grid.component.ts - FULLY OPTIMIZED FOR MASSIVE FOLDERS
import {
  Component,
  Input,
  Output,
  EventEmitter,
  OnInit,
  OnDestroy,
  OnChanges,
  AfterViewInit,
  SimpleChanges,
  ElementRef,
  ViewChild,
  ViewChildren,
  QueryList,
  NgZone,
  ChangeDetectionStrategy,
  ChangeDetectorRef,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileEntryComponent } from './file-entry.component';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { parentPath } from './address-bar.utils';
import { FileCategoryResolverService } from '../services/file-category-resolver.service';
import { VirtualCollection } from '../services/virtual-window';
import {
  ThumbnailPipelineService,
  type ThumbnailPriority,
} from '../services/thumbnail-pipeline.service';

@Component({
  selector: 'app-pane-grid',
  standalone: true,
  imports: [CommonModule, FileEntryComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      #gridContainer
      class="file-grid"
      [class.virtual]="virtualEnabled"
      [style.--grid-row-pitch]="rowHeight + 'px'"
      [class.drop-target-bg]="isBackgroundDropTarget"
      [class.dragging]="draggingPaths.size > 0"
      (mousedown)="onGridMouseDown($event)"
      (click)="onBackgroundClick($event)"
      (contextmenu)="onBackgroundContextMenu($event)"
      (dragover)="onBackgroundDragOver($event)"
      (dragleave)="onBackgroundDragLeave($event)"
      (drop)="onBackgroundDrop($event)">
      
      <div class="virtual-spacer" *ngIf="virtualEnabled" [style.height.px]="totalContentHeight"></div>
      
      <div class="grid-window" [class.virtual-window]="virtualEnabled" [style.transform]="virtualWindowTransform">
        <ng-container *ngFor="let entry of displayEntries; trackBy: trackByDisplayPath">
        <div
          *ngIf="entry === null"
          class="grid-item ghost-slot"
          aria-hidden="true">
          <div class="grid-info" *ngIf="dragPreviewEntry">
            <app-file-entry
              [entry]="dragPreviewEntry"
              [selected]="false"
              [colorizeByMime]="colorizeByMime"
              [thumbnailSrc]="thumbnailFor(dragPreviewEntry.path)"
              [thumbnailLoading]="thumbnailLoading(dragPreviewEntry.path)"
              variant="grid">
            </app-file-entry>
            <div class="grid-meta" *ngIf="!isParentEntry(dragPreviewEntry)">
              <span
                *ngIf="isDirectorySizePending(dragPreviewEntry.path); else dragPreviewSize"
                class="grid-meta-spinner"
                aria-label="Calculating folder size"
                title="Calculating folder size">
              </span>
              <ng-template #dragPreviewSize>{{ formatSize(dragPreviewEntry.size) }}</ng-template>
            </div>
          </div>
        </div>

        <div
          *ngIf="entry !== null"
          class="grid-item"
          [attr.data-path]="entry.path"
          [class.dragging-source]="draggingPaths.has(entry.path)"
          [class.selected]="paneActive && isSelected(entry)"
          [class.cursor]="paneActive && isCursor(entry)"
          [class.directory]="entry.type === FileType.DIRECTORY"
          [class.parent-dir]="isParentEntry(entry)"
          [class.drag-over]="isDragOverInside(entry)"
          [class.drag-over-folder]="isDragOverInside(entry) && entry.type === FileType.DIRECTORY"
          [attr.role]="'gridcell'"
          [attr.aria-selected]="isSelected(entry)"
          [class.drop-target]="isDropTarget(entry)"
          [draggable]="!isParentEntry(entry)"
          (click)="onItemClick(entry, $event)"
          (auxclick)="onItemAuxClick($event, entry)"
          (dblclick)="onItemDoubleClick(entry)"
          (contextmenu)="onContextMenu($event, entry)"
          (dragstart)="onDragStart($event, entry)"
          (dragend)="onDragEnd()"
          (dragover)="onDragOver($event, entry)"
          (dragleave)="onDragLeave($event, entry)"
          (drop)="onDrop($event, entry)">
          
          <div class="grid-info" *ngIf="!draggingPaths.has(entry.path)">
            <app-file-entry
              [entry]="entry"
              [selected]="paneActive && isSelected(entry)"
              [cursor]="paneActive && isCursor(entry)"
              [displayNameOverride]="getDisplayNameOverride(entry.path)"
              [colorizeByMime]="colorizeByMime"
              [thumbnailSrc]="thumbnailFor(entry.path)"
              [thumbnailLoading]="thumbnailLoading(entry.path)"
              variant="grid"
              (rename)="onRename($event)">
            </app-file-entry>
            <div class="grid-meta" *ngIf="!isParentEntry(entry)">
              <span
                *ngIf="isDirectorySizePending(entry.path); else gridMetaSize"
                class="grid-meta-spinner"
                aria-label="Calculating folder size"
                title="Calculating folder size">
              </span>
              <ng-template #gridMetaSize>{{ formatSize(entry.size) }}</ng-template>
            </div>
          </div>
        </div>
        </ng-container>
      </div>

      <div
        *ngIf="selectionRect.active"
        class="selection-rect"
        [style.left.px]="selectionRect.left"
        [style.top.px]="selectionRect.top"
        [style.width.px]="selectionRect.width"
        [style.height.px]="selectionRect.height">
      </div>
    </div>
  `,
  styles: [`
    .grid-item,
    .grid-info,
    .grid-info app-file-entry {
      min-width: 0;
      max-width: 100%;
    }

    /* app-file-entry is a custom element and therefore inline by default.
       Grid must own its cell geometry instead of relying on an inline host
       containing a block child, otherwise the track can collapse to a line box
       while the icon/name keep painting into the following row. */
    .grid-info {
      display: flex;
      flex-direction: column;
      align-items: stretch;
      width: 100%;
      flex: 1 1 auto;
      overflow: hidden;
    }

    .grid-info app-file-entry {
      display: block;
      flex: 0 0 auto;
      width: 100%;
      overflow: hidden;
    }


    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
      contain: layout style paint;
    }

    .file-grid {
      position: relative;
      flex: 1 1 0%;
      min-height: 0;
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(100px, max-content));
      --grid-row-gap: var(--vsc-padding-md);
      grid-auto-rows: calc(var(--grid-row-pitch, 124px) - var(--grid-row-gap));
      align-content: start;
      gap: var(--grid-row-gap);
      padding: var(--vsc-padding-md);
      overflow-y: auto;
      overflow-x: hidden;
      touch-action: pan-y;
      background: var(--vsc-editor-background);
    }

    /* Performance hint while dragging (FLIP uses transforms). */
    .file-grid.dragging .grid-item {
      will-change: transform;
    }
    
    .file-grid.virtual {
      display: block;
    }
    
    .virtual-spacer {
      width: 1px;
      pointer-events: none;
    }
    
    .grid-window {
      display: contents;
    }
    
    .grid-window.virtual-window {
      position: absolute;
      left: 0;
      right: 0;
      top: var(--vsc-padding-md);
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(100px, max-content));
      /* Virtual and ordinary Grid use the exact same row geometry. */
      grid-auto-rows: calc(var(--grid-row-pitch, 124px) - var(--grid-row-gap));
      align-content: start;
      gap: var(--grid-row-gap);
      padding: 0 var(--vsc-padding-md);
      will-change: transform;
    }

    .grid-window.virtual-window .grid-item {
      min-height: 0;
      height: 100%;
    }


    .selection-rect {
      position: absolute;
      pointer-events: none;
      z-index: 20;
      border: 1px solid var(--vsc-focus-border);
      background: color-mix(in srgb, var(--vsc-list-active-background) 45%, transparent);
    }

    .grid-item {
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: 6px;
      border-radius: var(--vsc-border-radius);
      cursor: pointer;
      box-sizing: border-box;
      width: 100%;
      min-height: calc(var(--grid-row-pitch, 124px) - var(--grid-row-gap));
      height: auto;
      overflow: hidden;
      transition:
        background-color var(--vsc-transition-fast),
        border-color var(--vsc-transition-fast),
        outline-color var(--vsc-transition-fast),
        box-shadow var(--vsc-transition-fast),
        transform var(--vsc-transition-fast);
      border: none;
      position: relative;
      contain: layout style paint;
      
      &.selected {
        background: var(--vsc-list-active-background);
        border-color: var(--vsc-accent-blue);
      }

      &.cursor:not(.selected) {
        background: color-mix(in srgb, var(--vsc-list-hover-background) 65%, transparent);
        box-shadow: inset 0 0 0 1px var(--vsc-focus-border);

        .grid-name {
          color: var(--vsc-foreground-bright);
        }
      }
      
      &.directory {
        .file-emoji {
          opacity: 1;
        }
      }
      
      &.parent-dir {
        /* Keep cursor/selection chrome identical to ordinary entries.
           Muting belongs to the entry content, not the grid cell. */
        .grid-name {
          font-style: italic;
        }
      }

      &.drop-target {
        outline: 1px dashed var(--vsc-focus-border);
        outline-offset: -2px;
        background: var(--vsc-list-hover-background);
      }

      &.drag-over {
        background-color: var(--vsc-list-drop-background, color-mix(in srgb, var(--vsc-focus-border, var(--vsc-accent-blue)) 20%, transparent));
        outline: 1px dashed var(--vsc-focus-border);
        outline-offset: -2px;
        box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--vsc-focus-border, var(--vsc-accent-blue)) 28%, transparent);
      }

      &.drag-over-folder {
        border-color: var(--vsc-focus-border, var(--vsc-accent-blue));
        border-width: 1px;
        border-style: solid;
        background: var(--vsc-list-drop-background, color-mix(in srgb, var(--vsc-list-active-background) 25%, transparent));
        box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--vsc-focus-border, var(--vsc-accent-blue)) 28%, transparent);
        transform: translateY(-1px);
      }

    }

    .grid-item.dragging-source {
      position: absolute;
      left: 0;
      top: 0;
      width: 0 !important;
      height: 0 !important;
      padding: 0 !important;
      margin: 0 !important;
      border: 0 !important;
      opacity: 0;
      overflow: hidden;
      pointer-events: none;
    }

    .grid-item.ghost-slot {
      pointer-events: none;
      cursor: default;
      contain: layout style;
      background: color-mix(in srgb, var(--vsc-focus-border, var(--vsc-accent-blue)) 12%, transparent);
      border: 2px dashed color-mix(in srgb, var(--vsc-focus-border, var(--vsc-accent-blue)) 85%, transparent);
      outline: none;
      opacity: 0.96;
      box-shadow:
        inset 0 0 0 1px color-mix(in srgb, var(--vsc-focus-border, var(--vsc-accent-blue)) 26%, transparent),
        0 0 12px color-mix(in srgb, var(--vsc-focus-border, var(--vsc-accent-blue)) 18%, transparent);
      animation: ghost-slot-pulse 900ms ease-in-out infinite alternate;
    }

    .grid-item.ghost-slot .grid-info {
      opacity: 0.82;
      filter: saturate(0.9);
    }

    @keyframes ghost-slot-pulse {
      from { transform: scale(0.985); }
      to { transform: scale(1); }
    }

    .grid-icon {
      position: relative;
      width: 48px;
      height: 48px;
      display: flex;
      align-items: center;
      justify-content: center;
      margin-bottom: 3px;
    }

    .file-emoji {
      font-size: 32px;
      line-height: 1;
      filter: drop-shadow(0 2px 4px var(--vsc-shadow-soft));
    }

    .dir-symlink-icon {
      position: relative;
      &::after {
        content: '↗';
        position: absolute;
        bottom: -2px;
        right: -6px;
        font-size: 11px;
        line-height: 1;
        opacity: 0.8;
        filter: none;
      }
    }

    .grid-info {
      text-align: center;
      width: 100%;
    }

    .grid-name {
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      max-width: 100%;
      line-height: 1.2;
    }

    .grid-meta {
      font-size: 11px;
      color: var(--vsc-foreground-dim);
      margin-top: 1px;
      min-height: 11px;
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .grid-meta-spinner {
      width: 12px;
      height: 12px;
      border-radius: 999px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue) 22%, transparent);
      border-top-color: var(--vsc-accent-blue);
      box-sizing: border-box;
      animation: grid-meta-spin 0.9s linear infinite;
    }

    :host-context(.compact-mode) {
      .file-grid {
        --grid-row-gap: var(--vsc-padding-sm);
        grid-template-columns: repeat(auto-fill, minmax(80px, max-content));
        gap: var(--grid-row-gap);
      }

      .grid-window.virtual-window {
        grid-template-columns: repeat(auto-fill, minmax(80px, max-content));
        gap: var(--grid-row-gap);
      }
      
      .grid-icon {
        width: 36px;
        height: 36px;
      }
      
      .file-emoji {
        font-size: 24px;
      }
      
      .grid-item {
        padding: 4px;
      }
    }

    .file-grid.drop-target-bg {
      outline: 2px dashed var(--vsc-focus-border);
      outline-offset: -2px;
      background: color-mix(in srgb, var(--vsc-list-hover-background) 30%, transparent);
    }

    @keyframes grid-meta-spin {
      from { transform: rotate(0deg); }
      to { transform: rotate(360deg); }
    }
  `]
})
export class PaneGridComponent implements OnInit, OnDestroy, OnChanges, AfterViewInit {
  constructor(
    private zone: NgZone,
    private cdr: ChangeDetectorRef,
    private categoryResolver: FileCategoryResolverService,
    private readonly thumbnails: ThumbnailPipelineService,
  ) {}

  @ViewChild('gridContainer') gridContainerRef?: ElementRef<HTMLElement>;
  @ViewChildren(FileEntryComponent) fileEntryComponents?: QueryList<FileEntryComponent>;

  @Input() entries: FileInfo[] = [];
  @Input() pendingDirectorySizePaths: ReadonlySet<string> = new Set();
  @Input() selectedFiles: Set<string> = new Set();
  @Input() cursorPath: string | null = null;
  @Input() paneActive = true;
  @Input() singleClickToOpen = false;
  @Input() colorizeByMime = false;
  @Input() thumbnailsEnabled = true;
  
  @Output() selectionChange = new EventEmitter<FileInfo[]>();
  @Output() cursorPathChange = new EventEmitter<string | null>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() openInNewTab = new EventEmitter<FileInfo>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() dragStart = new EventEmitter<{ event: DragEvent; entries: FileInfo[] }>();
  @Output() dragDrop = new EventEmitter<{ event: DragEvent; source: FileInfo[]; target: FileInfo }>();
  @Output() rename = new EventEmitter<{ entry: FileInfo; newName: string }>();
  @Output() viewportChange = new EventEmitter<void>();

  FileType = FileType;
  
  // OPTIMIZED: Lower threshold for earlier virtualization activation
  private readonly VIRTUAL_THRESHOLD = 200;
  readonly rowHeight = 124;
  private readonly overscanRows = 16;
  private readonly windowGuardRows = 6;
  private scrollRAF: number | null = null;
  private cursorFollowRAF: number | null = null;
  private boundScrollListener?: () => void;
  private resizeObserver?: ResizeObserver;

  private virtualStart = 0;
  private virtualEnd = 0;
  private cachedVirtualColumns = 1; // Cache column count to avoid repeated getComputedStyle
  private lastScrollTop = 0;
  private lastScrollTime = 0;
  private viewportHeight = 0;
  private readonly virtualCollection = new VirtualCollection<FileInfo, string>({
    identity: entry => entry.path,
    itemExtent: this.rowHeight,
    threshold: this.VIRTUAL_THRESHOLD,
    overscanRows: this.overscanRows,
    guardRows: this.windowGuardRows,
    minimumPrefetchItems: 768,
    prefetchMultiplier: 4,
    prefetchGuardRatio: 0.20,
  });
  private visibleEntriesCache: FileInfo[] = [];
  private readonly thumbnailScope = `grid-${Math.random().toString(36).slice(2)}`;
  private thumbnailGeneration = 0;
  private thumbnailUrls = new Map<string, string>();
  private thumbnailLoadingPaths = new Set<string>();
  private lastThumbnailBandKey = '';
  private idleThumbnailHandle: number | null = null;
  private columnsMeasureRAF: number | null = null; // Coalesce column measurements

  private anchorPath: string | null = null;

  private setCursor(path: string | null): void {
    this.anchorPath = path;
    this.cursorPathChange.emit(path);
  }
  dragPreviewEntry: FileInfo | null = null;
  dragOverPath: string | null = null;
  dragOverPosition: 'before' | 'after' | 'inside' | null = null;
  isBackgroundDropTarget = false;
  draggingPaths = new Set<string>();
  private dragImageEl: HTMLElement | null = null;
  private reflowRaf: number | null = null;
  private pendingReflowBefore: Map<string, { el: HTMLElement; rect: DOMRect }> | null = null;
  private marqueeContainer: HTMLElement | null = null;
  private marqueeStartX = 0;
  private marqueeStartY = 0;
  private marqueeAppend = false;
  private suppressBackgroundContextMenu = false;
  private suppressBackgroundClick = false;
  private suppressItemContextMenu = false;
  private selectionMode: 'none' | 'marquee' | 'paint' = 'none';
  private paintedPaths = new Set<string>();
  private marqueeButton: 0 | 2 | null = null;
  private hasMarqueeMoved = false;
  private pendingClearOnMove = false;
  private windowMouseMove?: (event: MouseEvent) => void;
  private windowMouseUp?: (event: MouseEvent) => void;
  private windowContextMenu?: (event: MouseEvent) => void;
  private pendingRenameDisplayNames = new Map<string, string>();

  selectionRect = {
    active: false,
    left: 0,
    top: 0,
    width: 0,
    height: 0,
  };

  get virtualEnabled(): boolean {
    return this.entries.length >= this.VIRTUAL_THRESHOLD;
  }

  get totalContentHeight(): number {
    return this.virtualCollection.window.totalExtentPx ||
      Math.ceil(this.entries.length / this.cachedVirtualColumns) * this.rowHeight;
  }

  get virtualWindowTransform(): string {
    if (!this.virtualEnabled) return '';
    const startRow = Math.floor(this.virtualStart / this.cachedVirtualColumns);
    return `translateY(${startRow * this.rowHeight}px)`;
  }

  get visibleEntries(): FileInfo[] {
    return this.visibleEntriesCache;
  }

  thumbnailFor(path: string): string | null {
    return this.thumbnailUrls.get(path) ?? null;
  }

  thumbnailLoading(path: string): boolean {
    return this.thumbnailLoadingPaths.has(path);
  }

  onRename(event: { entry: FileInfo; newName: string }): void {
    this.rename.emit(event);
  }

  /** Number of columns currently visible in the grid. */
  getColumns(): number {
    return Math.max(1, this.cachedVirtualColumns);
  }

  /** Approximate number of grid items visible in the viewport. */
  getApproxPageSize(): number {
    const host = this.gridContainerRef?.nativeElement;
    if (!host) return 5;
    const rows = Math.max(1, Math.floor(host.clientHeight / this.rowHeight) - 1);
    return rows * Math.max(1, this.cachedVirtualColumns);
  }

  private scheduleCursorFollow(): void {
    const path = this.cursorPath;
    if (!path) return;
    if (this.cursorFollowRAF) {
      cancelAnimationFrame(this.cursorFollowRAF);
    }
    this.cursorFollowRAF = requestAnimationFrame(() => {
      this.cursorFollowRAF = null;
      if (this.cursorPath === path) {
        this.scrollToPath(path);
      }
    });
  }

  /** Scroll the grid to bring the given path into view. */
  scrollToPath(path: string): void {
    if (!path) return;
    const host = this.gridContainerRef?.nativeElement;
    if (!host) return;
    const idx = this.findEntryIndex(path);
    if (idx < 0) return;
    if (this.virtualEnabled) {
      const row = Math.floor(idx / Math.max(1, this.cachedVirtualColumns));
      const itemTop = row * this.rowHeight;
      const itemBottom = itemTop + this.rowHeight;
      if (itemTop < host.scrollTop || itemBottom > host.scrollTop + host.clientHeight) {
        const centerOffset = host.clientHeight / 2 - this.rowHeight / 2;
        host.scrollTop = Math.max(0, itemTop - centerOffset);
      }
      if (this.updateVirtualWindow()) {
        this.refreshThumbnailPriorities();
        this.cdr.markForCheck();
      }
    } else {
      const el = host.querySelector<HTMLElement>(`.grid-item[data-path="${CSS.escape(path)}"]`);
      el?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    }
  }

  getScrollTop(): number {
    return this.gridContainerRef?.nativeElement?.scrollTop ?? 0;
  }


  getScrollMetrics(): { scrollTop: number; scrollHeight: number; clientHeight: number } {
    const element = this.gridContainerRef?.nativeElement;
    return {
      scrollTop: element?.scrollTop ?? 0,
      scrollHeight: element?.scrollHeight ?? 0,
      clientHeight: element?.clientHeight ?? 0,
    };
  }

  setScrollTop(top: number): void {
    const element = this.gridContainerRef?.nativeElement;
    if (!element) return;
    element.scrollTop = Math.max(0, top);
  }

  /** Called by parent (eg. on F2) to begin in-place rename for the current selection. */
  startRenameSelected(): void {
    const selectedPath = this.getFirstSelectedPathInOrder();
    if (!selectedPath) return;
    this.startRenameByPath(selectedPath);
  }

  startRenamePath(path: string): void {
    this.startRenameByPath(path);
  }

  setPendingRenameDisplayName(path: string, displayName: string | null): void {
    if (!path || path === PARENT_ENTRY_PATH) return;
    if (displayName && displayName.trim()) {
      this.pendingRenameDisplayNames.set(path, displayName.trim());
    } else {
      this.pendingRenameDisplayNames.delete(path);
    }
    this.cdr.markForCheck();
  }

  getDisplayNameOverride(path: string): string | null {
    return this.pendingRenameDisplayNames.get(path) ?? null;
  }

  isDirectorySizePending(path: string): boolean {
    return this.pendingDirectorySizePaths.has(path);
  }

  private getFirstSelectedPathInOrder(): string | null {
    if (!this.selectedFiles?.size) return null;
    for (const entry of this.entries) {
      if (!entry || this.isParentEntry(entry)) continue;
      if (this.selectedFiles.has(entry.path)) return entry.path;
    }
    return null;
  }

  private startRenameByPath(path: string): void {
    if (!path || path === PARENT_ENTRY_PATH) return;

    const host = this.gridContainerRef?.nativeElement;
    if (host) {
      if (this.virtualEnabled) {
        const idx = this.findEntryIndex(path);
        if (idx >= 0) {
          const row = Math.floor(idx / Math.max(1, this.cachedVirtualColumns));
          const targetTop = row * this.rowHeight;
          const centerOffset = host.clientHeight / 2 - this.rowHeight / 2;
          host.scrollTop = Math.max(0, targetTop - centerOffset);
          if (this.updateVirtualWindow()) {
            this.cdr.markForCheck();
          }
        }
      } else {
        const el = host.querySelector<HTMLElement>(`.grid-item[data-path="${CSS.escape(path)}"]`);
        el?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
      }
    }

    requestAnimationFrame(() => {
      const comp = this.fileEntryComponents?.find(c => c.entry?.path === path);
      comp?.beginRename();
      this.cdr.markForCheck();
    });
  }

  get displayEntries(): (FileInfo | null)[] {
    const visible = this.visibleEntries;
    if (!this.dragOverPath || this.dragOverPosition === 'inside') return visible;

    const targetIndex = visible.findIndex(entry => entry.path === this.dragOverPath);
    if (targetIndex === -1) return visible;

    const insertAt = this.dragOverPosition === 'after' ? targetIndex + 1 : targetIndex;
    const withGhost: (FileInfo | null)[] = [...visible];
    withGhost.splice(insertAt, 0, null);
    return withGhost;
  }

  trackByDisplayPath(index: number, entry: FileInfo | null): string {
    if (entry) return entry.path;
    return '__grid_ghost__';
  }

  ngOnInit(): void {
    // Seed anchor from parent-provided cursor on first load (e.g. tab restore).
    if (this.cursorPath && !this.anchorPath) {
      this.anchorPath = this.cursorPath;
    }
  }

  ngAfterViewInit(): void {
    const host = this.gridContainerRef?.nativeElement;
    if (host) {
      // Initialize cached columns from actual render
      this.cachedVirtualColumns = this.measureGridColumns(host);
    }
    this.setupResizeObserver();
    this.setupScrollListener();
    this.rebuildDerivedEntries();
    this.beginThumbnailGeneration();
    this.updateVirtualWindow(true);
    this.refreshThumbnailPriorities(true);
    this.scheduleCursorFollow();
  }

  ngOnChanges(changes: SimpleChanges): void {
    // Parent can restore cursor position (e.g. after tab switch / navigation).
    // Only sync if parent explicitly changed it; internal clicks update anchorPath directly.
    if (changes['cursorPath'] && this.cursorPath !== undefined) {
      this.anchorPath = this.cursorPath;
    }
    if (changes['entries'] || changes['colorizeByMime'] || changes['thumbnailsEnabled']) {
      if (changes['entries']) {
        this.beginThumbnailGeneration();
      }
      if (changes['entries']) {
        const prevLen = changes['entries'].previousValue?.length || 0;
        const currLen = changes['entries'].currentValue?.length || 0;
        if (Math.abs(prevLen - currLen) > 1000) {
          this.resetScroll();
        }
      }
      this.rebuildDerivedEntries();
      this.updateVirtualWindow(true);
      this.refreshThumbnailPriorities(true);
      if (changes['cursorPath']) {
        this.scheduleCursorFollow();
      }
      this.cdr.markForCheck();
    } else if (changes['pendingDirectorySizePaths'] || changes['selectedFiles'] || changes['cursorPath']) {
      if (changes['cursorPath']) {
        this.scheduleCursorFollow();
      }
      this.cdr.markForCheck();
    }
  }

  private prewarmVisibleCollisionResolutions(): void {
    if (!this.colorizeByMime) return;
    for (const entry of this.visibleEntriesCache) {
      this.categoryResolver.ensureResolved(entry);
    }
  }

  ngOnDestroy(): void {
    this.detachMarqueeListeners();
    this.cleanupDragImage();
    this.cleanupResizeObserver();
    this.cleanupScrollListener();
    if (this.scrollRAF) {
      cancelAnimationFrame(this.scrollRAF);
    }
    if (this.cursorFollowRAF) {
      cancelAnimationFrame(this.cursorFollowRAF);
    }
    if (this.reflowRaf) {
      cancelAnimationFrame(this.reflowRaf);
      this.reflowRaf = null;
    }
    this.pendingReflowBefore = null;
    this.cancelIdleThumbnailWork();
    this.thumbnails.cancelScope(this.thumbnailScope);
    this.thumbnailUrls.clear();
    this.thumbnailLoadingPaths.clear();
  }

  private cleanupDragImage(): void {
    if (!this.dragImageEl) return;
    try {
      this.dragImageEl.remove();
    } catch {
      // ignore
    }
    this.dragImageEl = null;
  }

  private prefersReducedMotion(): boolean {
    try {
      return window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches ?? false;
    } catch {
      return false;
    }
  }

  private captureGridItemRects(): Map<string, { el: HTMLElement; rect: DOMRect }> {
    const container = this.gridContainerRef?.nativeElement;
    if (!container) return new Map();

    const map = new Map<string, { el: HTMLElement; rect: DOMRect }>();
    const items = container.querySelectorAll<HTMLElement>('.grid-item[data-path]');
    for (const el of Array.from(items)) {
      if (el.classList.contains('dragging-source')) continue;
      const path = el.dataset['path'];
      if (!path) continue;
      map.set(path, { el, rect: el.getBoundingClientRect() });
    }
    return map;
  }

  /** Animate grid reflow using FLIP (First-Last-Invert-Play). */
  private animateGridReflowFrom(before: Map<string, { el: HTMLElement; rect: DOMRect }>): void {
    if (this.prefersReducedMotion()) return;
    if (before.size === 0) return;

    // Coalesce: if dragover fires rapidly, keep only the latest reflow.
    this.pendingReflowBefore = before;
    if (this.reflowRaf) {
      cancelAnimationFrame(this.reflowRaf);
      this.reflowRaf = null;
    }

    this.zone.runOutsideAngular(() => {
      this.reflowRaf = requestAnimationFrame(() => {
        this.reflowRaf = null;

        const beforeSnapshot = this.pendingReflowBefore;
        this.pendingReflowBefore = null;
        if (!beforeSnapshot || beforeSnapshot.size === 0) return;

        const after = this.captureGridItemRects();
        if (after.size === 0) return;

        for (const [path, next] of after) {
          const prev = beforeSnapshot.get(path);
          if (!prev) continue;

          const dx = prev.rect.left - next.rect.left;
          const dy = prev.rect.top - next.rect.top;
          if (Math.abs(dx) < 0.5 && Math.abs(dy) < 0.5) continue;

          // Avoid fighting the folder hover transform while dragging over folders.
          if (next.el.classList.contains('drag-over-folder')) continue;

          try {
            next.el.animate(
              [
                { transform: `translate(${dx}px, ${dy}px)` },
                { transform: 'translate(0px, 0px)' },
              ],
              {
                duration: 110,
                easing: 'cubic-bezier(0.2, 0, 0.2, 1)',
              }
            );
          } catch {
            // ignore (WAAPI not available)
          }
        }
      });
    });
  }

  private setupResizeObserver(): void {
    const element = this.gridContainerRef?.nativeElement;
    if (!element) return;

    this.zone.runOutsideAngular(() => {
      this.resizeObserver = new ResizeObserver((entries) => {
        let shouldMark = false;
        for (const entry of entries) {
          if (this.viewportHeight !== entry.contentRect.height) {
            this.viewportHeight = entry.contentRect.height;
            shouldMark = this.updateVirtualWindow() || shouldMark;
          }
          // Also remeasure columns on width changes (more efficient)
          if (entry.contentRect.width > 0) {
            const cols = this.measureGridColumns(element);
            if (cols !== this.cachedVirtualColumns) {
              this.cachedVirtualColumns = cols;
              shouldMark = this.updateVirtualWindow(true) || shouldMark;
            }
          }
        }
        if (shouldMark) {
          this.refreshThumbnailPriorities();
          this.cdr.markForCheck();
        }
      });

      this.resizeObserver.observe(element);
    });
  }

  private cleanupResizeObserver(): void {
    this.resizeObserver?.disconnect();
  }

  private setupScrollListener(): void {
    const element = this.gridContainerRef?.nativeElement;
    if (!element) return;
    this.boundScrollListener = () => this.onScroll();
    this.zone.runOutsideAngular(() => {
      element.addEventListener('scroll', this.boundScrollListener!, { passive: true });
    });
  }

  private cleanupScrollListener(): void {
    const element = this.gridContainerRef?.nativeElement;
    if (!element || !this.boundScrollListener) return;
    element.removeEventListener('scroll', this.boundScrollListener);
    this.boundScrollListener = undefined;
  }

  private onScroll(): void {
    if (this.scrollRAF) return;
    this.scrollRAF = requestAnimationFrame(() => {
      this.scrollRAF = null;
      const now = performance.now();
      const scrollDelta = Math.abs((this.gridContainerRef?.nativeElement?.scrollTop ?? 0) - this.lastScrollTop);
      this.lastScrollTime = now;
      const windowChanged = this.updateVirtualWindow();
      // Visible-row priority can change while the retained render window stays
      // fixed, so thumbnail scheduling follows scroll position, not DOM rebuilds.
      this.refreshThumbnailPriorities();
      if (windowChanged) {
        this.cdr.markForCheck();
      }
      this.viewportChange.emit();
    });
  }

  private resetScroll(): void {
    const element = this.gridContainerRef?.nativeElement;
    if (element) {
      element.scrollTop = 0;
      this.lastScrollTop = 0;
    }
    this.virtualStart = 0;
    this.virtualEnd = Math.min(this.overscanRows * this.cachedVirtualColumns * 2, this.entries.length);
    this.rebuildVisibleEntries();
    this.refreshThumbnailPriorities(true);
  }

  private updateVirtualWindow(forceRebuild = false): boolean {
    const host = this.gridContainerRef?.nativeElement;
    const viewportHeight = Math.max(
      this.rowHeight,
      this.viewportHeight || host?.clientHeight || this.rowHeight,
    );
    const scrollTop = host?.scrollTop ?? 0;

    let columns = this.cachedVirtualColumns;
    if (forceRebuild) {
      columns = this.measureGridColumns(host);
      this.cachedVirtualColumns = columns;
    }

    this.virtualCollection.configure({
      itemExtent: this.rowHeight,
      threshold: this.VIRTUAL_THRESHOLD,
      overscanRows: this.overscanRows,
      guardRows: this.windowGuardRows,
      columns,
    });
    this.virtualCollection.setLogicalCount(this.entries.length);

    const changed = this.virtualCollection.updateViewport({
      scrollTop,
      viewportHeight,
      columns,
    }, forceRebuild);

    const window = this.virtualCollection.window;
    this.virtualStart = window.start;
    this.virtualEnd = window.end;
    if (changed || forceRebuild) {
      const residentOffset = window.enabled ? window.start : 0;
      const residentEntries = window.enabled
        ? this.entries.slice(window.start, window.end)
        : this.entries;
      this.virtualCollection.populate(
        residentOffset,
        residentEntries,
        this.entries.length,
      );
      this.rebuildVisibleEntries();
      this.prewarmVisibleCollisionResolutions();
    }
    return changed || forceRebuild;
  }

  private beginThumbnailGeneration(): void {
    this.thumbnailGeneration = this.thumbnails.beginGeneration(this.thumbnailScope);
    this.lastThumbnailBandKey = '';
    this.thumbnailLoadingPaths.clear();
    this.cancelIdleThumbnailWork();
  }

  private refreshThumbnailPriorities(force = false): void {
    if (!this.thumbnailsEnabled || !this.entries.length) {
      this.thumbnailLoadingPaths.clear();
      return;
    }

    const host = this.gridContainerRef?.nativeElement;
    if (!host) return;

    const columns = Math.max(1, this.cachedVirtualColumns);
    const scrollTop = host.scrollTop;
    const viewportHeight = Math.max(this.rowHeight, host.clientHeight);

    // Visible band is row-aligned. We never schedule individual-cell geometry.
    const firstVisibleRow = Math.max(0, Math.floor(scrollTop / this.rowHeight));
    const visibleRows = Math.max(1, Math.ceil(viewportHeight / this.rowHeight));
    const visibleStart = firstVisibleRow * columns;
    const visibleEnd = Math.min(
      this.entries.length,
      (firstVisibleRow + visibleRows) * columns,
    );

    // The shared VirtualCollection render window is the overscan band.
    const renderWindow = this.virtualCollection.window;
    const bandKey = [
      visibleStart,
      visibleEnd,
      renderWindow.start,
      renderWindow.end,
      columns,
      this.entries.length,
    ].join(':');

    if (!force && bandKey === this.lastThumbnailBandKey) return;
    this.lastThumbnailBandKey = bandKey;

    // Rapid scrolling invalidates queued/in-flight thumbnail work from the old
    // window. Cached decoded thumbnails remain reusable.
    this.thumbnailGeneration = this.thumbnails.beginGeneration(this.thumbnailScope);
    const generation = this.thumbnailGeneration;

    const columnsForRetention = Math.max(1, this.cachedVirtualColumns);
    const retentionStart = Math.max(0, renderWindow.start - 12 * columnsForRetention);
    const retentionEnd = Math.min(this.entries.length, renderWindow.end + 12 * columnsForRetention);
    const retainedPaths = new Set(
      this.entries
        .slice(retentionStart, retentionEnd)
        .map(entry => entry.path),
    );
    for (const path of [...this.thumbnailUrls.keys()]) {
      if (!retainedPaths.has(path)) this.thumbnailUrls.delete(path);
    }
    for (const path of [...this.thumbnailLoadingPaths]) {
      if (!retainedPaths.has(path)) this.thumbnailLoadingPaths.delete(path);
    }

    this.scheduleThumbnailRange(
      visibleStart,
      visibleEnd,
      'visible',
      generation,
    );

    if (renderWindow.start < visibleStart) {
      this.scheduleThumbnailRange(
        renderWindow.start,
        visibleStart,
        'overscan',
        generation,
      );
    }
    if (visibleEnd < renderWindow.end) {
      this.scheduleThumbnailRange(
        visibleEnd,
        renderWindow.end,
        'overscan',
        generation,
      );
    }

    this.scheduleIdleThumbnailBand(renderWindow, generation);
  }

  private scheduleThumbnailRange(
    start: number,
    end: number,
    priority: ThumbnailPriority,
    generation: number,
  ): void {
    for (let index = Math.max(0, start); index < Math.min(end, this.entries.length); index += 1) {
      const entry = this.entries[index];
      if (!entry || this.isParentEntry(entry) || !this.thumbnails.canThumbnail(entry)) {
        continue;
      }

      const cached = this.thumbnails.peek(entry, 160, 120);
      if (cached) {
        this.thumbnailUrls.set(entry.path, cached);
        continue;
      }

      // A previously cached object URL may already have been LRU-evicted.
      this.thumbnailUrls.delete(entry.path);
      this.thumbnailLoadingPaths.add(entry.path);
      void this.thumbnails.request(entry, {
        scope: this.thumbnailScope,
        generation,
        priority,
        width: 160,
        height: 120,
      }).then(url => {
        if (
          !this.thumbnails.isCurrent(this.thumbnailScope, generation) ||
          generation !== this.thumbnailGeneration
        ) {
          return;
        }

        this.thumbnailLoadingPaths.delete(entry.path);
        if (url) this.thumbnailUrls.set(entry.path, url);
        this.cdr.markForCheck();
      });
    }
  }

  private scheduleIdleThumbnailBand(
    renderWindow: { start: number; end: number },
    generation: number,
  ): void {
    this.cancelIdleThumbnailWork();

    const schedule = (callback: () => void): number => {
      const ric = (window as Window & {
        requestIdleCallback?: (
          callback: IdleRequestCallback,
          options?: IdleRequestOptions,
        ) => number;
      }).requestIdleCallback;

      if (ric) {
        return ric(() => callback(), { timeout: 500 });
      }
      return window.setTimeout(callback, 80);
    };

    this.idleThumbnailHandle = schedule(() => {
      this.idleThumbnailHandle = null;
      if (
        generation !== this.thumbnailGeneration ||
        !this.thumbnails.isCurrent(this.thumbnailScope, generation)
      ) {
        return;
      }

      const columns = Math.max(1, this.cachedVirtualColumns);
      // Low-priority work is intentionally bounded to nearby rows. Scheduling
      // all 100k entries would just churn the bounded decoded-image cache.
      const idleRows = 12;
      const beforeStart = Math.max(0, renderWindow.start - idleRows * columns);
      const afterEnd = Math.min(
        this.entries.length,
        renderWindow.end + idleRows * columns,
      );

      this.scheduleThumbnailRange(
        beforeStart,
        renderWindow.start,
        'idle',
        generation,
      );
      this.scheduleThumbnailRange(
        renderWindow.end,
        afterEnd,
        'idle',
        generation,
      );
    });
  }

  private cancelIdleThumbnailWork(): void {
    if (this.idleThumbnailHandle === null) return;

    const cancel = (window as Window & {
      cancelIdleCallback?: (handle: number) => void;
    }).cancelIdleCallback;
    if (cancel) cancel(this.idleThumbnailHandle);
    else clearTimeout(this.idleThumbnailHandle);
    this.idleThumbnailHandle = null;
  }

  private findEntryIndex(path: string): number {
    const residentIndex = this.virtualCollection.indexOfIdentity(path);
    if (residentIndex >= 0) return residentIndex;
    return this.entries.findIndex(entry => entry.path === path);
  }

  private rebuildDerivedEntries(): void {
    this.rebuildVisibleEntries();
  }

  private rebuildVisibleEntries(): void {
    // Render directly from the logical source window. VirtualCollection keeps
    // only the same resident slice for identity lookups, but must not become a
    // second source of truth for Grid ordering/layout.
    this.visibleEntriesCache = this.virtualEnabled
      ? this.entries.slice(this.virtualStart, this.virtualEnd)
      : this.entries;
  }

  trackByPath(_i: number, entry: FileInfo): string {
    return entry.path;
  }

  isParentEntry(entry: FileInfo): boolean {
    return entry.path === PARENT_ENTRY_PATH;
  }

  isSelected(entry: FileInfo): boolean {
    return this.selectedFiles.has(entry.path);
  }

  isCursor(entry: FileInfo): boolean {
    return entry.path === this.cursorPath;
  }

  isDropTarget(entry: FileInfo): boolean {
    return this.dragOverPath === entry.path && this.dragOverPosition === 'inside';
  }

  isDragOverBefore(entry: FileInfo): boolean {
    return this.dragOverPath === entry.path && this.dragOverPosition === 'before';
  }

  isDragOverAfter(entry: FileInfo): boolean {
    return this.dragOverPath === entry.path && this.dragOverPosition === 'after';
  }

  isDragOverInside(entry: FileInfo): boolean {
    return this.dragOverPath === entry.path && this.dragOverPosition === 'inside';
  }

  private measureGridColumns(host?: HTMLElement): number {
    if (!host) {
      return Math.max(1, this.cachedVirtualColumns);
    }

    const gridElement = this.virtualEnabled
      ? host.querySelector<HTMLElement>('.grid-window.virtual-window')
      : host;

    if (!gridElement) {
      return Math.max(1, this.cachedVirtualColumns);
    }

    // Cache style lookup - only get it once per measurement
    const template = getComputedStyle(gridElement).gridTemplateColumns;
    const resolvedColumns = template
      .split(' ')
      .map(value => value.trim())
      .filter(value => value.length > 0 && value !== 'none');

    if (resolvedColumns.length > 0) {
      return resolvedColumns.length;
    }

    // Fallback calculation for when grid-template-columns can't be parsed
    const compactMode = host.closest('.compact-mode') !== null;
    const minCellWidth = compactMode ? 80 : 100;
    const gap = compactMode ? 8 : 12;
    const horizontalPadding = 24;
    const contentWidth = Math.max(1, host.clientWidth - horizontalPadding * 2);
    return Math.max(1, Math.floor((contentWidth + gap) / (minCellWidth + gap)));
  }

  onItemClick(entry: FileInfo, event: MouseEvent): void {
    if (event.ctrlKey || event.metaKey) {
      this.selectionChange.emit(this.buildToggledSelection(entry));
      this.setCursor(entry.path);
    } else if (event.shiftKey && this.anchorPath) {
      this.selectionChange.emit(this.rangeSelect(entry));
    } else {
      if (this.isParentEntry(entry)) {
        this.anchorPath = entry.path;
        this.selectionChange.emit([entry]);
        return;
      }
      this.setCursor(entry.path);
      this.selectionChange.emit([entry]);
      if (this.singleClickToOpen) {
        this.navigate.emit(entry);
      }
    }
  }

  onItemDoubleClick(entry: FileInfo): void {
    if (this.isParentEntry(entry)) {
      this.navigate.emit(entry);
      return;
    }

    if (this.singleClickToOpen) {
      return;
    }
    this.navigate.emit(entry);
  }

  onContextMenu(event: MouseEvent, entry: FileInfo): void {
    if (this.suppressItemContextMenu) {
      event.preventDefault();
      this.suppressItemContextMenu = false;
      return;
    }

    event.preventDefault();
    if (this.isParentEntry(entry)) return;

    if (!this.isSelected(entry)) {
      // TC: right-click on unselected → replace selection with only this file
      this.selectionChange.emit([entry]);
      this.setCursor(entry.path);
    }
    // TC: right-click on selected → keep full selection unchanged
    this.contextMenu.emit({ event, entry });
  }

  onItemAuxClick(event: MouseEvent, entry: FileInfo): void {
    if (event.button !== 1 || this.isParentEntry(entry)) return;
    event.preventDefault();
    event.stopPropagation();
    this.openInNewTab.emit(entry);
  }

  onBackgroundClick(event: MouseEvent): void {
    if (this.suppressBackgroundClick) {
      this.suppressBackgroundClick = false;
      return;
    }

    if (event.target === event.currentTarget) {
      this.selectionChange.emit([]);
      // TC: background click clears selection but keeps cursor
    }
  }

  onBackgroundContextMenu(event: MouseEvent): void {
    if (this.suppressBackgroundContextMenu) {
      event.preventDefault();
      this.suppressBackgroundContextMenu = false;
      return;
    }

    if (event.target === event.currentTarget) {
      event.preventDefault();
      this.selectionChange.emit([]);
      this.backgroundContextMenu.emit(event);
      // TC: background contextmenu clears selection but keeps cursor
    }
  }

  onGridMouseDown(event: MouseEvent): void {
    if (event.button !== 0 && event.button !== 2) return;

    // Left button on a draggable file item → let HTML5 drag-and-drop handle it
    if (event.button === 0) {
      const target = event.target as HTMLElement;
      if (target.closest('[draggable="true"]')) return;
    }

    const container = event.currentTarget as HTMLElement;
    this.marqueeContainer = container;
    this.marqueeAppend = event.ctrlKey || event.metaKey;
    this.selectionMode = 'marquee';
    this.marqueeButton = event.button as 0 | 2;
    this.hasMarqueeMoved = false;
    this.pendingClearOnMove = this.marqueeButton === 2 && !this.marqueeAppend;
    this.paintedPaths.clear();

    const { x, y } = this.getRelativePoint(event, container);
    this.marqueeStartX = x;
    this.marqueeStartY = y;

    this.selectionRect = {
      active: true,
      left: x,
      top: y,
      width: 0,
      height: 0,
    };

    if (!this.marqueeAppend && !this.pendingClearOnMove) {
      this.selectionChange.emit([]);
    }

    this.suppressBackgroundClick = true;
    this.suppressItemContextMenu = false;

    this.windowMouseMove = (moveEvent: MouseEvent) => this.zone.run(() => this.onGridMouseMove(moveEvent));
    this.windowMouseUp = () => this.zone.run(() => this.finishMarqueeSelection());
    this.windowContextMenu = (contextEvent: MouseEvent) => {
      if (this.selectionMode !== 'none' && this.hasMarqueeMoved) {
        contextEvent.preventDefault();
        contextEvent.stopPropagation();
      }
    };
    
    window.addEventListener('mousemove', this.windowMouseMove);
    window.addEventListener('mouseup', this.windowMouseUp);
    window.addEventListener('contextmenu', this.windowContextMenu, true);
    this.suppressBackgroundContextMenu = event.button === 2;
    
    if (event.button === 0) {
      event.preventDefault();
    }
  }

  private onGridMouseMove(event: MouseEvent): void {
    if (!this.marqueeContainer) return;

    if (this.selectionMode === 'paint') {
      this.updatePaintSelection(event);
      return;
    }

    if (this.selectionMode !== 'marquee' || !this.selectionRect.active) return;

    const { x, y } = this.getRelativePoint(event, this.marqueeContainer);
    const left = Math.min(this.marqueeStartX, x);
    const top = Math.min(this.marqueeStartY, y);
    const width = Math.abs(x - this.marqueeStartX);
    const height = Math.abs(y - this.marqueeStartY);
    
    if (width > 2 || height > 2) {
      this.hasMarqueeMoved = true;
      if (this.pendingClearOnMove) {
        this.selectionChange.emit([]);
        this.pendingClearOnMove = false;
      }
      if (this.marqueeButton === 2) {
        this.suppressItemContextMenu = true;
      }
    }

    this.selectionRect = { active: true, left, top, width, height };
    this.emitMarqueeSelection();
    this.cdr.markForCheck();
  }

  private updatePaintSelection(event: MouseEvent): void {
    const element = document.elementFromPoint(event.clientX, event.clientY) as HTMLElement | null;
    const item = element?.closest('.grid-item') as HTMLElement | null;
    const path = item?.dataset['path'];
    if (!path || path === PARENT_ENTRY_PATH || this.paintedPaths.has(path)) return;

    this.paintedPaths.add(path);

    const selectionPaths = new Set<string>(this.paintedPaths);
    if (this.marqueeAppend) {
      for (const existingPath of this.selectedFiles) {
        if (existingPath !== PARENT_ENTRY_PATH) {
          selectionPaths.add(existingPath);
        }
      }
    }

    const selectedEntries = this.entries.filter(entry => selectionPaths.has(entry.path));
    this.selectionChange.emit(selectedEntries);
  }

  private emitMarqueeSelection(): void {
    if (!this.marqueeContainer) return;

    const selectionPaths = new Set<string>();
    const metrics = this.getGridMarqueeMetrics();

    if (metrics) {
      const selectionRight = this.selectionRect.left + this.selectionRect.width;
      const selectionBottom = this.selectionRect.top + this.selectionRect.height;
      const startColumn = Math.max(0, Math.floor((this.selectionRect.left - metrics.paddingLeft) / metrics.cellStrideX));
      const endColumn = Math.min(metrics.columns - 1, Math.floor(Math.max(0, selectionRight - metrics.paddingLeft - 1) / metrics.cellStrideX));
      const startRow = Math.max(0, Math.floor((this.selectionRect.top - metrics.paddingTop) / metrics.cellStrideY));
      const endRow = Math.max(0, Math.floor(Math.max(0, selectionBottom - metrics.paddingTop - 1) / metrics.cellStrideY));

      for (let row = startRow; row <= endRow; row++) {
        for (let column = startColumn; column <= endColumn; column++) {
          const index = row * metrics.columns + column;
          const entry = this.entries[index];
          if (entry && entry.path !== PARENT_ENTRY_PATH) {
            selectionPaths.add(entry.path);
          }
        }
      }
    } else {
      const containerRect = this.marqueeContainer.getBoundingClientRect();
      const items = this.marqueeContainer.querySelectorAll<HTMLElement>('.grid-item[data-path]');

      for (const item of Array.from(items)) {
        const path = item.dataset['path'];
        if (!path || path === PARENT_ENTRY_PATH) continue;

        const itemRect = item.getBoundingClientRect();
        const itemLeft = itemRect.left - containerRect.left + this.marqueeContainer.scrollLeft;
        const itemTop = itemRect.top - containerRect.top + this.marqueeContainer.scrollTop;
        
        const intersects =
          itemLeft < this.selectionRect.left + this.selectionRect.width &&
          itemLeft + itemRect.width > this.selectionRect.left &&
          itemTop < this.selectionRect.top + this.selectionRect.height &&
          itemTop + itemRect.height > this.selectionRect.top;

        if (intersects) {
          selectionPaths.add(path);
        }
      }
    }

    if (this.marqueeAppend) {
      for (const path of this.selectedFiles) {
        if (path !== PARENT_ENTRY_PATH) {
          selectionPaths.add(path);
        }
      }
    }

    const selectedEntries = this.entries.filter(entry => selectionPaths.has(entry.path));
    this.selectionChange.emit(selectedEntries);
  }

  private getGridMarqueeMetrics(): { columns: number; paddingLeft: number; paddingTop: number; cellStrideX: number; cellStrideY: number } | null {
    const host = this.gridContainerRef?.nativeElement;
    if (!host) return null;

    const columns = Math.max(1, this.cachedVirtualColumns);
    const style = getComputedStyle(host);
    const paddingLeft = parseFloat(style.paddingLeft || '0') || 0;
    const paddingRight = parseFloat(style.paddingRight || '0') || 0;
    const paddingTop = parseFloat(style.paddingTop || '0') || 0;
    const columnGap = parseFloat(style.columnGap || style.gap || '0') || 0;
    const rowGap = parseFloat(style.rowGap || style.gap || '0') || 0;
    const contentWidth = Math.max(1, host.clientWidth - paddingLeft - paddingRight);
    const cellWidth = Math.max(1, (contentWidth - columnGap * (columns - 1)) / columns);

    return {
      columns,
      paddingLeft,
      paddingTop,
      cellStrideX: cellWidth + columnGap,
      cellStrideY: this.rowHeight + rowGap,
    };
  }

  private finishMarqueeSelection(): void {
    this.detachMarqueeListeners();
    this.marqueeContainer = null;
    this.selectionMode = 'none';
    this.marqueeButton = null;
    this.hasMarqueeMoved = false;
    this.pendingClearOnMove = false;
    this.paintedPaths.clear();
    this.selectionRect = {
      active: false,
      left: 0,
      top: 0,
      width: 0,
      height: 0,
    };
    this.cdr.markForCheck();
  }

  private detachMarqueeListeners(): void {
    if (this.windowMouseMove) {
      window.removeEventListener('mousemove', this.windowMouseMove);
      this.windowMouseMove = undefined;
    }
    if (this.windowMouseUp) {
      window.removeEventListener('mouseup', this.windowMouseUp);
      this.windowMouseUp = undefined;
    }
    if (this.windowContextMenu) {
      window.removeEventListener('contextmenu', this.windowContextMenu, true);
      this.windowContextMenu = undefined;
    }
  }

  private getRelativePoint(event: MouseEvent, container: HTMLElement): { x: number; y: number } {
    const rect = container.getBoundingClientRect();
    return {
      x: event.clientX - rect.left + container.scrollLeft,
      y: event.clientY - rect.top + container.scrollTop,
    };
  }

  private toggleSelection(entry: FileInfo): void {
    this.selectionChange.emit(this.buildToggledSelection(entry));
  }

  private rangeSelect(entry: FileInfo): FileInfo[] {
    const selectable = this.entries.filter(e => !this.isParentEntry(e));
    const startIdx = selectable.findIndex(e => e.path === this.anchorPath);
    const endIdx = selectable.findIndex(e => e.path === entry.path);
    const [min, max] = [Math.min(startIdx, endIdx), Math.max(startIdx, endIdx)];

    if (startIdx === -1 || endIdx === -1) {
      return [entry];
    }

    const range: FileInfo[] = [];
    for (let i = min; i <= max; i++) {
      if (!this.isParentEntry(selectable[i])) {
        range.push(selectable[i]);
      }
    }
    return range;
  }

  onDragStart(event: DragEvent, entry: FileInfo): void {
    if (event.button !== 0) {
      event.preventDefault();
      return;
    }

    if (this.selectionMode !== 'none') {
      event.preventDefault();
      return;
    }

    if (this.isParentEntry(entry)) {
      event.preventDefault();
      return;
    }

    let entries: FileInfo[];
    if (this.isSelected(entry)) {
      entries = this.entries.filter(e => this.selectedFiles.has(e.path) && !this.isParentEntry(e));
    } else {
      // TC: dragging unselected file → replace selection with only this file
      entries = [entry];
      this.selectionChange.emit([entry]);
      this.setCursor(entry.path);
    }

    const draggingPaths = entries.map(e => e.path);

    // Snapshot positions before we hide/remove source tiles, so the remaining tiles can FLIP-animate into place.
    const reflowBefore = this.captureGridItemRects();

    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'copyMove';
      event.dataTransfer.setData('application/json', JSON.stringify({
        source: 'grid',
        entries: entries.map(e => ({ path: e.path, type: e.type, name: e.name })),
      }));

      // Ensure we don't leak an old drag image node.
      this.cleanupDragImage();

      const tileEl = event.currentTarget as HTMLElement;
      const tileRect = tileEl.getBoundingClientRect();
      const ghost = tileEl.cloneNode(true) as HTMLElement;
      ghost.style.position = 'fixed';
      ghost.style.top = '-1000px';
      ghost.style.left = '0';
      ghost.style.width = `${Math.round(tileRect.width)}px`;
      ghost.style.height = `${Math.round(tileRect.height)}px`;
      ghost.style.margin = '0';
      ghost.style.pointerEvents = 'none';
      ghost.style.transform = 'none';
      ghost.style.opacity = '0.98';
      ghost.style.background = 'var(--vsc-editor-background)';
      ghost.style.border = '1px solid color-mix(in srgb, var(--vsc-focus-border, var(--vsc-accent-blue)) 35%, transparent)';
      ghost.style.boxShadow = '0 6px 20px var(--vsc-scrim-soft)';
      ghost.style.borderRadius = 'var(--vsc-border-radius)';
      ghost.classList.remove('drag-over', 'drag-over-folder', 'drop-target', 'dragging-source');

      if (entries.length > 1) {
        const badge = document.createElement('div');
        badge.style.cssText = `
          position: absolute;
          top: 6px;
          right: 6px;
          min-width: 18px;
          height: 18px;
          padding: 0 6px;
          border-radius: 9px;
          background: var(--vsc-focus-border, var(--vsc-accent-blue));
          color: var(--vsc-editor-background, var(--vsc-editor-background));
          font-size: 11px;
          line-height: 18px;
          text-align: center;
          box-sizing: border-box;
          border: 1px solid color-mix(in srgb, var(--vsc-focus-border, var(--vsc-accent-blue)) 65%, transparent);
          z-index: 20;
        `;
        badge.textContent = `${entries.length}`;
        ghost.appendChild(badge);
      }

      document.body.appendChild(ghost);

      // Force layout/paint before using it as a drag image.
      void ghost.offsetWidth;

      event.dataTransfer.setDragImage(ghost, Math.floor(tileRect.width / 2), Math.floor(tileRect.height / 2));
      // Keep it mounted for the duration of the drag;
      // some environments (incl. embedded WebView) can lose the drag image if removed immediately.
      this.dragImageEl = ghost;
    }

    this.dragPreviewEntry = entries[0] ?? null;
    // Delay hiding the source tiles until after the drag-start snapshot.
    // This also keeps a fallback preview visible if setDragImage is flaky.
    setTimeout(() => {
      this.draggingPaths = new Set(draggingPaths);
      this.cdr.markForCheck();
      this.animateGridReflowFrom(reflowBefore);
    }, 0);

    this.dragStart.emit({ event, entries });
  }

  onDragEnd(): void {
    const reflowBefore = this.captureGridItemRects();
    this.dragPreviewEntry = null;
    this.dragOverPath = null;
    this.dragOverPosition = null;
    this.draggingPaths = new Set<string>();
    this.cleanupDragImage();
    this.cdr.markForCheck();
    this.animateGridReflowFrom(reflowBefore);
  }

  onDragOver(event: DragEvent, entry: FileInfo): void {
    if (this.isParentEntry(entry)) return;

    const types = event.dataTransfer?.types;
    const hasInternalData = types?.includes('application/json') || false;
    const hasExternalFiles = !hasInternalData && ((types?.includes('Files') || false) || ((event.dataTransfer?.files?.length ?? 0) > 0));

    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    const xFraction = rect.width > 0 ? (event.clientX - rect.left) / rect.width : 0.5;

    event.preventDefault();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = event.ctrlKey || event.metaKey ? 'copy' : 'move';
    }

    if (hasExternalFiles) {
      const nextPosition: 'before' | 'after' | 'inside' = entry.type === FileType.DIRECTORY
        ? 'inside'
        : (xFraction < 0.5 ? 'before' : 'after');
      if (this.dragOverPath === entry.path && this.dragOverPosition === nextPosition) return;

      const reflowBefore = this.captureGridItemRects();
      this.dragOverPath = entry.path;
      this.dragOverPosition = nextPosition;
      this.cdr.markForCheck();
      this.animateGridReflowFrom(reflowBefore);
      return;
    }

    let position: 'before' | 'after' | 'inside';
    if (xFraction < 0.3) {
      position = 'before';
    } else if (xFraction > 0.7) {
      position = 'after';
    } else {
      position = entry.type === FileType.DIRECTORY ? 'inside' : (xFraction < 0.5 ? 'before' : 'after');
    }

    if (this.dragOverPath === entry.path && this.dragOverPosition === position) return;

    const reflowBefore = this.captureGridItemRects();
    this.dragOverPath = entry.path;
    this.dragOverPosition = position;
    this.cdr.markForCheck();
    this.animateGridReflowFrom(reflowBefore);
  }

  onDragLeave(event: DragEvent, entry: FileInfo): void {
    // Intentionally keep current drag-over state here.
    // Clearing on per-tile dragleave causes flicker while moving across child nodes/adjacent tiles.
    // State is cleared on dragend/drop or when leaving the container.
    void event;
    void entry;
  }

  onDrop(event: DragEvent, entry: FileInfo): void {
    if (this.isParentEntry(entry)) return;
    event.preventDefault();

    const types = event.dataTransfer?.types;
    const hasExternalFiles = !(types?.includes('application/json') || false) && ((types?.includes('Files') || false) || ((event.dataTransfer?.files?.length ?? 0) > 0));
    // For external OS drops, don't stopPropagation - let Wails' window handler fire wails:file-drop
    if (hasExternalFiles) {
      this.dragPreviewEntry = null;
      this.dragOverPath = null;
      this.dragOverPosition = null;
      this.draggingPaths = new Set<string>();
      this.cleanupDragImage();
      this.cdr.markForCheck();
      return;
    }

    event.stopPropagation();
    const capturedPosition = this.dragOverPosition;

    const reflowBefore = this.captureGridItemRects();
    this.dragPreviewEntry = null;
    this.dragOverPath = null;
    this.dragOverPosition = null;
    this.draggingPaths = new Set<string>();
    this.cleanupDragImage();
    this.cdr.markForCheck();
    this.animateGridReflowFrom(reflowBefore);

    const draggedEntries = this.readDraggedEntries(event);
    if (!draggedEntries.length) return;

    const source = this.buildSourceFromEntries(draggedEntries);
    if (!source.length) return;

    const goInto = capturedPosition === 'inside' && entry.type === FileType.DIRECTORY;
    const target = goInto
      ? entry
      : ({ ...entry, path: parentPath(entry.path), type: FileType.DIRECTORY } as FileInfo);

    this.dragDrop.emit({ event, source, target });
  }

  onBackgroundDragOver(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = event.ctrlKey || event.metaKey ? 'copy' : 'move';
    }
    
    this.isBackgroundDropTarget = true;
    this.cdr.markForCheck();
  }

  onBackgroundDragLeave(event: DragEvent): void {
    const target = event.relatedTarget as HTMLElement;
    const container = this.gridContainerRef?.nativeElement;
    
    if (container && !container.contains(target)) {
      const reflowBefore = this.captureGridItemRects();
      this.dragPreviewEntry = null;
      this.dragOverPath = null;
      this.dragOverPosition = null;
      this.isBackgroundDropTarget = false;
      this.cdr.markForCheck();
      this.animateGridReflowFrom(reflowBefore);
    }
  }

  onBackgroundDrop(event: DragEvent): void {
    event.preventDefault();

    const types = event.dataTransfer?.types;
    const hasExternalFiles = !(types?.includes('application/json') || false) && ((types?.includes('Files') || false) || ((event.dataTransfer?.files?.length ?? 0) > 0));
    // For external OS drops, don't stopPropagation - let Wails' window handler fire wails:file-drop
    if (hasExternalFiles) {
      this.dragPreviewEntry = null;
      this.dragOverPath = null;
      this.dragOverPosition = null;
      this.isBackgroundDropTarget = false;
      this.draggingPaths = new Set<string>();
      this.cleanupDragImage();
      this.cdr.markForCheck();
      return;
    }

    event.stopPropagation();

    const reflowBefore = this.captureGridItemRects();
    this.dragPreviewEntry = null;
    this.dragOverPath = null;
    this.dragOverPosition = null;
    this.isBackgroundDropTarget = false;
    this.draggingPaths = new Set<string>();
    this.cleanupDragImage();
    this.cdr.markForCheck();
    this.animateGridReflowFrom(reflowBefore);

    // Internal drag and drop to current directory
    const draggedEntries = this.readDraggedEntries(event);
    if (!draggedEntries.length) return;

    const source = this.buildSourceFromEntries(draggedEntries);
    if (!source.length) return;

    const currentDir = this.entries.length > 0 && !this.isParentEntry(this.entries[0])
      ? parentPath(this.entries[0].path)
      : '';
    
    const target: FileInfo = {
      name: '',
      path: currentDir,
      type: FileType.DIRECTORY,
      size: 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
    };

    this.dragDrop.emit({ event, source, target });
  }

  private buildToggledSelection(entry: FileInfo): FileInfo[] {
    const selected = new Set(this.selectedFiles);
    if (selected.has(entry.path)) {
      selected.delete(entry.path);
    } else {
      selected.add(entry.path);
    }
    return this.entries.filter(e => selected.has(e.path) && !this.isParentEntry(e));
  }

  /** Build source entries; uses pane entries for same-pane drag or constructs minimal FileInfo for cross-pane drags. */
  private buildSourceFromEntries(dragged: { path: string; type: FileType; name: string }[]): FileInfo[] {
    const pathSet = new Set(dragged.map(d => d.path));
    const fromEntries = this.entries.filter(e => pathSet.has(e.path) && !this.isParentEntry(e));
    if (fromEntries.length > 0) return fromEntries;
    return dragged.filter(d => d.path && d.path !== PARENT_ENTRY_PATH).map(d => ({
      name: d.name,
      path: d.path,
      type: d.type,
      size: 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
    } as FileInfo));
  }

  private readDraggedEntries(event: DragEvent): { path: string; type: FileType; name: string }[] {
    try {
      const payload = event.dataTransfer?.getData('application/json');
      if (!payload) return [];
      const parsed = JSON.parse(payload);
      if (Array.isArray(parsed?.entries)) return parsed.entries;
      if (Array.isArray(parsed?.paths)) return parsed.paths.map((p: string) => ({ path: p, type: FileType.FILE, name: p.split(/[\/\\]/).pop() || p }));
      return [];
    } catch {
      return [];
    }
  }


  private extractExternalPaths(event: DragEvent): string[] {
    const dataTransfer = event.dataTransfer;
    if (!dataTransfer) return [];

    const paths = new Set<string>();

    const files = dataTransfer.files;
    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      const filePath = (file as any).path as string | undefined;
      if (filePath) {
        paths.add(filePath);
      }
    }

    const uriPayload = dataTransfer.getData('text/uri-list') || dataTransfer.getData('text/plain');
    if (uriPayload) {
      for (const rawLine of uriPayload.split(/\r?\n/)) {
        const line = rawLine.trim();
        if (!line || line.startsWith('#')) continue;
        const parsedPath = this.fileUriToPath(line);
        if (parsedPath) paths.add(parsedPath);
      }
    }

    return Array.from(paths);
  }

  private fileUriToPath(value: string): string | null {
    if (!value) return null;
    const trimmed = value.trim();
    if (!/^file:\/\//i.test(trimmed)) {
      if (/^[A-Za-z]:[\\/]/.test(trimmed)) return trimmed;
      if (/^\\\\/.test(trimmed)) return trimmed;
      if (trimmed.startsWith('/')) return trimmed;
      return null;
    }

    try {
      let decoded = decodeURIComponent(trimmed).replace(/^file:\/\//i, '');

      if (/^\/[A-Za-z]:\//.test(decoded)) {
        decoded = decoded.slice(1);
      }

      if (/^[A-Za-z]:\//.test(decoded)) {
        return decoded.replace(/\//g, '\\');
      }

      if (!decoded.startsWith('/')) {
        return ('//' + decoded).replace(/\//g, '\\');
      }

      return decoded;
    } catch {
      return null;
    }
  }

  formatSize(bytes: number): string {
    if (bytes === 0) return '';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
  }
}
