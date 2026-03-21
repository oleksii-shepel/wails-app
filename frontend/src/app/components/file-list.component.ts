// file-list.component.ts
import {
  Component,
  Input,
  Output,
  EventEmitter,
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
import { FileEntryComponent, FileEntrySelectEvent } from './file-entry.component';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { parentPath } from './address-bar.utils';
import { VscodeScrollbarDirective } from '../directives/vscode-scrollbar.directive';

@Component({
  selector: 'app-file-list',
  standalone: true,
  imports: [CommonModule, FileEntryComponent, VscodeScrollbarDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      #paneContent
      class="pane-content"
      appVscodeScrollbar
      [class.virtual]="virtualEnabled"
      [class.drop-target-bg]="isBackgroundDropTarget"
      (mousedown)="onListMouseDown($event)"
      (click)="onBackgroundClick($event)"
      (contextmenu)="onBackgroundContextMenu($event)"
      (dragover)="onBackgroundDragOver($event)"
      (dragleave)="onBackgroundDragLeave($event)"
      (drop)="onBackgroundDrop($event)">
      
      <div *ngIf="loading" class="loading">
        <div class="spinner"></div>
        <p>Loading...</p>
      </div>
      
      <div *ngIf="error" class="error">
        {{ error }}
      </div>
      
      <div *ngIf="!loading && !error" class="file-list-container">
        <div
          *ngIf="virtualEnabled && parentEntry"
          #parentRow
          class="file-list-row"
          [attr.data-path]="parentEntry.path"
          [draggable]="false"
          (auxclick)="onItemAuxClick($event, parentEntry)"
          (dragstart)="onDragStart($event, parentEntry)"
          (dragover)="onDragOver($event, parentEntry)"
          (dragleave)="onDragLeave(parentEntry)"
          (drop)="onDrop($event, parentEntry)"
          (contextmenu)="onContextMenu($event, parentEntry)">
          <app-file-entry
            [entry]="parentEntry"
            [selected]="isSelected(parentEntry)"
            [cursor]="parentEntry.path === cursorPath"
            (select)="onSelect($event)"
            (navigate)="onNavigate($event)"
            (rename)="onRename($event)">
          </app-file-entry>
        </div>

        <div class="virtual-spacer" *ngIf="virtualEnabled" [style.height.px]="totalContentHeight"></div>
        
        <div
          class="file-list-window"
          [class.virtual-window]="virtualEnabled"
          [style.top.px]="virtualTopOffset"
          [style.transform]="virtualWindowTransform">
          <div
            *ngFor="let entry of visibleEntries; trackBy: trackByPath"
            class="file-list-row"
            [attr.data-path]="entry.path"
            [style.height.px]="virtualEnabled ? rowHeight : null"
            [class.drag-over]="isDragOverInside(entry)"
            [class.drag-over-before]="isDragOverBefore(entry)"
            [class.drag-over-after]="isDragOverAfter(entry)"
            [draggable]="!isParentEntry(entry)"
            (auxclick)="onItemAuxClick($event, entry)"
            (dragstart)="onDragStart($event, entry)"
            (dragover)="onDragOver($event, entry)"
            (dragleave)="onDragLeave(entry)"
            (drop)="onDrop($event, entry)"
            (contextmenu)="onContextMenu($event, entry)">
            <div *ngIf="isDragOverBefore(entry)" class="drop-indicator drop-indicator-before"></div>
            <div *ngIf="isDragOverAfter(entry)" class="drop-indicator drop-indicator-after"></div>
            <app-file-entry
              [entry]="entry"
              [selected]="isSelected(entry)"
              [cursor]="entry.path === cursorPath"
              (select)="onSelect($event)"
              (navigate)="onNavigate($event)"
              (rename)="onRename($event)">
            </app-file-entry>
          </div>
        </div>
      </div>

      <div
        *ngIf="showSelectionRect"
        class="selection-rect"
        [style.left.px]="selectionRect.left"
        [style.top.px]="selectionRect.top"
        [style.width.px]="selectionRect.width"
        [style.height.px]="selectionRect.height">
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
      contain: layout style paint;
    }

    .pane-content {
      position: relative;
      flex: 1;
      overflow-y: auto;
      overflow-x: hidden;
      padding: var(--vsc-padding-sm);
      background: var(--vsc-editor-background);
      min-height: 0;
      contain: layout style paint;
    }

    .loading,
    .error {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      height: 100%;
      gap: var(--vsc-padding-md);
      font-size: var(--vsc-font-size);
      color: var(--vsc-foreground-dim);

      .spinner {
        width: 40px;
        height: 40px;
        border: 3px solid var(--vsc-border);
        border-top-color: var(--vsc-accent-blue);
        border-radius: 50%;
        animation: spin 1s linear infinite;
      }

      p {
        margin: 0;
      }
    }

    .error {
      color: var(--vsc-error);
      font-weight: 500;
    }

    @keyframes spin {
      to {
        transform: rotate(360deg);
      }
    }

    .file-list-container {
      position: relative;
      display: flex;
      flex-direction: column;
    }

    .virtual-spacer {
      width: 1px;
      pointer-events: none;
      flex-shrink: 0;
    }

    .file-list-window {
      display: flex;
      flex-direction: column;
      gap: 0;
    }

    .file-list-window.virtual-window {
      position: absolute;
      left: 0;
      right: 0;
      top: 0;
      will-change: transform;
    }

    .selection-rect {
      position: absolute;
      pointer-events: none;
      z-index: 20;
      border: 1px solid var(--vsc-focus-border);
      background: color-mix(in srgb, var(--vsc-list-active-background) 45%, transparent);
    }

    .file-list-row {
      border-radius: var(--vsc-border-radius-sm);
      flex-shrink: 0;
      contain: layout style;
      position: relative;

      &.drag-over {
        background-color: var(--vsc-list-drop-background, rgba(0, 120, 212, 0.2));
        outline: 1px dashed var(--vsc-focus-border);
        outline-offset: -1px;
      }
    }

    .drop-indicator {
      position: absolute;
      left: 0;
      right: 0;
      height: 3px;
      background-color: var(--vsc-focus-border, #0078d4);
      pointer-events: none;
      z-index: 10;
    }

    .drop-indicator-before {
      top: -1px;
    }

    .drop-indicator-after {
      bottom: -1px;
    }

    .pane-content.drop-target-bg {
      outline: 2px dashed var(--vsc-focus-border);
      outline-offset: -2px;
      background: color-mix(in srgb, var(--vsc-list-hover-background) 30%, transparent);
    }
  `]
})
export class FileListComponent implements OnDestroy, OnChanges, AfterViewInit {
  constructor(
    private zone: NgZone,
    private cdr: ChangeDetectorRef
  ) {}

  @ViewChild('paneContent') paneContentRef?: ElementRef<HTMLElement>;
  @ViewChild('parentRow') parentRowRef?: ElementRef<HTMLElement>;
  @ViewChildren(FileEntryComponent) fileEntryComponents?: QueryList<FileEntryComponent>;

  @Input() entries: FileInfo[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  @Input() cursorPath: string | null = null;
  @Input() loading = false;
  @Input() error: string | null = null;
  @Input() paneActive = true;
  
  @Output() selectionChange = new EventEmitter<FileInfo[]>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() openInNewTab = new EventEmitter<FileInfo>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() dragStart = new EventEmitter<{ event: DragEvent; entries: FileInfo[] }>();
  @Output() dragDrop = new EventEmitter<{ event: DragEvent; source: FileInfo[]; target: FileInfo }>();
  @Output() rename = new EventEmitter<{ entry: FileInfo; newName: string }>();

  private readonly VIRTUAL_THRESHOLD = 100;
  private readonly DEFAULT_ROW_HEIGHT = 44;
  rowHeight = 44;
  private readonly overscan = 5;
  private scrollRAF: number | null = null;
  private boundScrollListener?: () => void;
  private resizeObserver?: ResizeObserver;

  private virtualStart = 0;
  private virtualEnd = 0;
  private viewportHeight = 0;
  private parentRowHeight = 0;

  private anchorPath: string | null = null;
  dragOverPath: string | null = null;
  dragOverPosition: 'before' | 'after' | 'inside' | null = null;
  isBackgroundDropTarget = false;
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

  selectionRect = {
    active: false,
    left: 0,
    top: 0,
    width: 0,
    height: 0,
  };

  get showSelectionRect(): boolean {
    return this.selectionRect.active && this.marqueeButton !== 2;
  }

  get virtualEnabled(): boolean {
    return !this.loading && !this.error && this.virtualEntries.length >= this.VIRTUAL_THRESHOLD;
  }

  get parentEntry(): FileInfo | null {
    const first = this.entries[0];
    return first && this.isParentEntry(first) ? first : null;
  }

  get virtualEntries(): FileInfo[] {
    if (!this.parentEntry) return this.entries;
    return this.entries.slice(1);
  }

  get visibleEntries(): FileInfo[] {
    if (!this.virtualEnabled) return this.entries;
    return this.virtualEntries.slice(this.virtualStart, this.virtualEnd);
  }

  get totalContentHeight(): number {
    return this.virtualEntries.length * this.rowHeight;
  }

  get virtualWindowTransform(): string {
    if (!this.virtualEnabled) return '';
    return `translateY(${this.virtualStart * this.rowHeight}px)`;
  }

  get virtualTopOffset(): number | null {
    if (!this.virtualEnabled) return null;
    return this.parentEntry ? this.parentRowHeight : 0;
  }

  ngAfterViewInit(): void {
    this.setupResizeObserver();
    this.setupScrollListener();
    this.updateVirtualWindow();
    requestAnimationFrame(() => this.measureHeights());
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['entries'] || changes['loading'] || changes['error']) {
      if (changes['entries'] && !changes['entries'].firstChange) {
        const prevLen = changes['entries'].previousValue?.length || 0;
        const currLen = changes['entries'].currentValue?.length || 0;
        if (Math.abs(prevLen - currLen) > 1000) {
          this.resetScroll();
        }
      }
      this.updateVirtualWindow();
      queueMicrotask(() => requestAnimationFrame(() => this.measureHeights()));
      this.cdr.markForCheck();
    }
  }

  ngOnDestroy(): void {
    this.detachMarqueeListeners();
    this.cleanupResizeObserver();
    this.cleanupScrollListener();
    if (this.scrollRAF) {
      cancelAnimationFrame(this.scrollRAF);
    }
  }

  private setupResizeObserver(): void {
    if (!this.paneContentRef?.nativeElement) return;
    
    this.resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        this.viewportHeight = entry.contentRect.height;
        this.updateVirtualWindow();
        this.cdr.markForCheck();
      }
    });
    
    this.resizeObserver.observe(this.paneContentRef.nativeElement);
  }

  private cleanupResizeObserver(): void {
    this.resizeObserver?.disconnect();
  }

  private setupScrollListener(): void {
    const element = this.paneContentRef?.nativeElement;
    if (!element) return;
    this.boundScrollListener = () => this.onScroll();
    element.addEventListener('scroll', this.boundScrollListener, { passive: true });
  }

  private cleanupScrollListener(): void {
    const element = this.paneContentRef?.nativeElement;
    if (!element || !this.boundScrollListener) return;
    element.removeEventListener('scroll', this.boundScrollListener);
    this.boundScrollListener = undefined;
  }

  private onScroll(): void {
    if (this.scrollRAF) return;
    this.scrollRAF = requestAnimationFrame(() => {
      this.scrollRAF = null;
      this.updateVirtualWindow();
      this.cdr.markForCheck();
    });
  }

  private resetScroll(): void {
    const element = this.paneContentRef?.nativeElement;
    if (element) {
      element.scrollTop = 0;
    }
    this.virtualStart = 0;
    this.virtualEnd = Math.min(this.overscan * 2, this.virtualEntries.length);
  }

  private updateVirtualWindow(): void {
    if (!this.virtualEnabled) {
      this.virtualStart = 0;
      this.virtualEnd = this.entries.length;
      return;
    }

    const element = this.paneContentRef?.nativeElement;
    const scrollTop = element?.scrollTop ?? 0;
    const parentOffset = this.parentEntry ? this.parentRowHeight : 0;
    const contentScrollTop = Math.max(0, scrollTop - parentOffset);
    const visibleParentPart = this.parentEntry ? Math.max(0, parentOffset - scrollTop) : 0;

    const viewportHeight = Math.max(
      1,
      (this.viewportHeight || element?.clientHeight || 0) - visibleParentPart
    );
    const visibleCount = Math.max(1, Math.ceil(viewportHeight / this.rowHeight));
    
    const start = Math.max(0, Math.floor(contentScrollTop / this.rowHeight) - this.overscan);
    const end = Math.min(this.virtualEntries.length, start + visibleCount + this.overscan * 2);

    if (start !== this.virtualStart || end !== this.virtualEnd) {
      this.virtualStart = start;
      this.virtualEnd = end;
    }
  }

  private measureHeights(): void {
    const host = this.paneContentRef?.nativeElement;
    if (!host) return;
    const row = host.querySelector<HTMLElement>(
      `.file-list-row[data-path]:not([data-path="${PARENT_ENTRY_PATH}"]) .file-entry`
    ) ?? host.querySelector<HTMLElement>('.file-list-row .file-entry');
    if (!row) return;
    const measured = Math.ceil(row.getBoundingClientRect().height);
    if (!Number.isFinite(measured) || measured <= 0) return;
    const nextRowHeight = Math.max(this.DEFAULT_ROW_HEIGHT, measured);
    let changed = false;
    if (Math.abs(nextRowHeight - this.rowHeight) >= 1) {
      this.rowHeight = nextRowHeight;
      changed = true;
    }

    const parentEl = this.parentRowRef?.nativeElement?.querySelector<HTMLElement>('.file-entry');
    if (parentEl) {
      const nextParentHeight = Math.ceil(parentEl.getBoundingClientRect().height);
      if (Number.isFinite(nextParentHeight) && nextParentHeight > 0 && Math.abs(nextParentHeight - this.parentRowHeight) >= 1) {
        this.parentRowHeight = nextParentHeight;
        changed = true;
      }
    } else if (this.parentRowHeight !== 0) {
      this.parentRowHeight = 0;
      changed = true;
    }

    if (changed) {
      this.updateVirtualWindow();
      this.cdr.markForCheck();
    }
  }

  trackByPath(_index: number, entry: FileInfo): string {
    return entry.path;
  }

  isParentEntry(entry: FileInfo): boolean {
    return entry.path === PARENT_ENTRY_PATH;
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

  isSelected(entry: FileInfo): boolean {
    return this.selectedFiles.has(entry.path);
  }

  onSelect(event: FileEntrySelectEvent): void {
    const entry = event.entry;
    if (this.isParentEntry(entry)) {
      this.navigate.emit(entry);
      return;
    }

    const mouseEvent = event.mouseEvent;
    const isToggle = mouseEvent.ctrlKey || mouseEvent.metaKey;
    const isRange = mouseEvent.shiftKey && this.anchorPath !== null;

    if (isRange) {
      this.selectionChange.emit(this.buildRangeSelection(entry));
    } else if (isToggle) {
      this.selectionChange.emit(this.buildToggledSelection(entry));
      this.anchorPath = entry.path;
    } else {
      this.anchorPath = entry.path;
      this.selectionChange.emit([entry]);
    }
  }

  onNavigate(entry: FileInfo): void {
    this.navigate.emit(entry);
  }

  onRename(event: { entry: FileInfo; newName: string }): void {
    this.rename.emit(event);
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

    const host = this.paneContentRef?.nativeElement;
    if (host) {
      if (this.virtualEnabled) {
        const idx = this.virtualEntries.findIndex(e => e.path === path);
        if (idx >= 0) {
          const parentOffset = this.parentEntry ? this.parentRowHeight : 0;
          const targetTop = idx * this.rowHeight;
          const centerOffset = host.clientHeight / 2 - this.rowHeight / 2;
          host.scrollTop = Math.max(0, parentOffset + targetTop - centerOffset);
          this.updateVirtualWindow();
          this.cdr.markForCheck();
        }
      } else {
        const el = host.querySelector<HTMLElement>(`.file-list-row[data-path="${CSS.escape(path)}"]`);
        el?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
      }
    }

    requestAnimationFrame(() => {
      const comp = this.fileEntryComponents?.find(c => c.entry?.path === path);
      comp?.beginRename();
      this.cdr.markForCheck();
    });
  }

  /** Scroll the list to bring the given path into view. */
  scrollToPath(path: string): void {
    if (!path || path === PARENT_ENTRY_PATH) return;
    const host = this.paneContentRef?.nativeElement;
    if (!host) return;
    if (this.virtualEnabled) {
      const idx = this.virtualEntries.findIndex(e => e.path === path);
      if (idx >= 0) {
        const parentOffset = this.parentEntry ? this.parentRowHeight : 0;
        const itemTop = parentOffset + idx * this.rowHeight;
        const itemBottom = itemTop + this.rowHeight;
        if (itemTop < host.scrollTop || itemBottom > host.scrollTop + host.clientHeight) {
          const centerOffset = host.clientHeight / 2 - this.rowHeight / 2;
          host.scrollTop = Math.max(0, parentOffset + idx * this.rowHeight - centerOffset);
        }
        this.updateVirtualWindow();
        this.cdr.markForCheck();
      }
    } else {
      const el = host.querySelector<HTMLElement>(`.file-list-row[data-path="${CSS.escape(path)}"]`);
      el?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    }
  }

  /** Approximate number of rows visible in the viewport. */
  getApproxPageSize(): number {
    const host = this.paneContentRef?.nativeElement;
    if (!host) return 10;
    return Math.max(1, Math.floor(host.clientHeight / this.rowHeight) - 1);
  }

  onBackgroundClick(event: MouseEvent): void {
    if (this.suppressBackgroundClick) {
      this.suppressBackgroundClick = false;
      return;
    }
    // Background click just focuses the pane (handled by parent wrapper).
    // Never clear selection here — the pane-inactive CSS dims the cursor automatically.
  }

  onBackgroundContextMenu(event: MouseEvent): void {
    if (this.suppressBackgroundContextMenu) {
      event.preventDefault();
      this.suppressBackgroundContextMenu = false;
      return;
    }

    const target = event.target as HTMLElement;
    if (event.target === event.currentTarget || target.classList.contains('file-list-container')) {
      event.preventDefault();
    }
  }

  onContextMenu(event: MouseEvent, entry: FileInfo): void {
    if (this.suppressItemContextMenu) {
      event.preventDefault();
      this.suppressItemContextMenu = false;
      return;
    }

    event.preventDefault();
    if (this.isParentEntry(entry)) return;

    if (this.isSelected(entry)) {
      const nextSelection = this.entries.filter(e => 
        this.selectedFiles.has(e.path) && e.path !== entry.path && !this.isParentEntry(e)
      );
      this.selectionChange.emit(nextSelection);
    } else {
      const selected = new Set(this.selectedFiles);
      selected.add(entry.path);
      this.selectionChange.emit(this.entries.filter(e => 
        selected.has(e.path) && !this.isParentEntry(e)
      ));
      this.anchorPath = entry.path;
    }
    this.contextMenu.emit({ event, entry });
  }

  onItemAuxClick(event: MouseEvent, entry: FileInfo): void {
    if (event.button !== 1 || this.isParentEntry(entry)) return;
    event.preventDefault();
    event.stopPropagation();
    this.openInNewTab.emit(entry);
  }

  onListMouseDown(event: MouseEvent): void {
    if (event.button !== 0 && event.button !== 2) return;

    // Left button on a draggable file row → let HTML5 drag-and-drop handle it
    if (event.button === 0) {
      const target = event.target as HTMLElement;
      if (target.closest('[draggable="true"]')) return;
    }

    const container = event.currentTarget as HTMLElement;
    if (!container) return;

    this.marqueeContainer = container;
    this.marqueeAppend = event.ctrlKey || event.metaKey;
    this.selectionMode = 'marquee';
    this.marqueeButton = event.button as 0 | 2;
    this.hasMarqueeMoved = false;
    // Defer selection clear until the marquee actually moves — a plain background
    // click keeps selection so the cursor stays visible in both active and inactive panes.
    this.pendingClearOnMove = !this.marqueeAppend;
    this.paintedPaths.clear();
    
    const { x, y } = this.getRelativePoint(event, container);
    this.marqueeStartX = x;
    this.marqueeStartY = y;
    this.selectionRect = { active: true, left: x, top: y, width: 0, height: 0 };

    this.suppressBackgroundClick = true;
    this.suppressItemContextMenu = false;

    this.windowMouseMove = (moveEvent: MouseEvent) => this.zone.run(() => this.onListMouseMove(moveEvent));
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

  private onListMouseMove(event: MouseEvent): void {
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
    
    if (width > 10 || height > 10) {
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
    if (this.hasMarqueeMoved) {
      this.emitMarqueeSelection();
    }
    this.cdr.markForCheck();
  }

  private updatePaintSelection(event: MouseEvent): void {
    const element = document.elementFromPoint(event.clientX, event.clientY) as HTMLElement | null;
    const row = element?.closest('.file-list-row') as HTMLElement | null;
    const path = row?.dataset['path'];
    
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
    
    const containerRect = this.marqueeContainer.getBoundingClientRect();
    const selectionPaths = new Set<string>();
    const items = this.marqueeContainer.querySelectorAll<HTMLElement>('.file-list-row[data-path]');

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

  private finishMarqueeSelection(): void {
    this.detachMarqueeListeners();
    this.marqueeContainer = null;
    this.selectionMode = 'none';
    this.marqueeButton = null;
    this.hasMarqueeMoved = false;
    this.pendingClearOnMove = false;
    this.paintedPaths.clear();
    this.selectionRect = { active: false, left: 0, top: 0, width: 0, height: 0 };
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

    const entries = this.isSelected(entry)
      ? this.entries.filter(e => this.selectedFiles.has(e.path) && !this.isParentEntry(e))
      : [entry];

    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'copyMove';
      event.dataTransfer.setData('application/json', JSON.stringify({
        source: 'list',
        entries: entries.map(e => ({ path: e.path, type: e.type, name: e.name })),
      }));
    }

    this.dragStart.emit({ event, entries });
  }

  onDragOver(event: DragEvent, entry: FileInfo): void {
    if (this.isParentEntry(entry)) return;
    event.preventDefault();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = event.ctrlKey || event.metaKey ? 'copy' : 'move';
    }
    const el = event.currentTarget as HTMLElement;
    const rect = el.getBoundingClientRect();
    const fraction = rect.height > 0 ? (event.clientY - rect.top) / rect.height : 0.5;
    let position: 'before' | 'after' | 'inside';
    if (fraction < 0.3) {
      position = 'before';
    } else if (fraction > 0.7) {
      position = 'after';
    } else {
      position = entry.type === FileType.DIRECTORY ? 'inside' : (fraction < 0.5 ? 'before' : 'after');
    }
    this.dragOverPath = entry.path;
    this.dragOverPosition = position;
    this.cdr.markForCheck();
  }

  onDragLeave(entry: FileInfo): void {
    if (this.dragOverPath === entry.path) {
      this.dragOverPath = null;
      this.dragOverPosition = null;
      this.cdr.markForCheck();
    }
  }

  onDrop(event: DragEvent, entry: FileInfo): void {
    if (this.isParentEntry(entry)) return;
    event.preventDefault();

    // For external OS drops, don't stopPropagation - let Wails' window handler fire wails:file-drop
    if (this.hasExternalFiles(event)) {
      this.dragOverPath = null;
      this.dragOverPosition = null;
      this.cdr.markForCheck();
      return;
    }

    event.stopPropagation();
    const capturedPosition = this.dragOverPosition;
    this.dragOverPath = null;
    this.dragOverPosition = null;
    this.cdr.markForCheck();

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
    // Prevent default to stop browser from opening files
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
    const container = this.paneContentRef?.nativeElement;
    
    if (container && !container.contains(target)) {
      this.isBackgroundDropTarget = false;
      this.cdr.markForCheck();
    }
  }

  onBackgroundDrop(event: DragEvent): void {
    event.preventDefault();

    // For external OS drops, don't stopPropagation - let Wails' window handler fire wails:file-drop
    if (this.hasExternalFiles(event)) {
      this.isBackgroundDropTarget = false;
      this.cdr.markForCheck();
      return;
    }

    event.stopPropagation();
    this.isBackgroundDropTarget = false;
    this.cdr.markForCheck();

    // Internal drag to background
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

  private hasExternalFiles(event: DragEvent): boolean {
    const types = event.dataTransfer?.types;
    if (!types) return false;
    if (types.includes('application/json')) return false;
    return types.includes('Files') || (event.dataTransfer?.files?.length ?? 0) > 0;
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

  private handleExternalDrop(event: DragEvent, targetEntry: FileInfo): void {
    const externalPaths = this.extractExternalPaths(event);
    if (externalPaths.length === 0) return;

    const source: FileInfo[] = externalPaths.map(path => ({
      name: path.split(/[/\\]/).pop() || '',
      path: path,
      type: FileType.FILE,
      size: 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
    }));

    const target = targetEntry.type === FileType.DIRECTORY
      ? targetEntry
      : ({ ...targetEntry, path: parentPath(targetEntry.path), type: FileType.DIRECTORY } as FileInfo);

    this.dragDrop.emit({ event, source, target });
  }

  private handleExternalBackgroundDrop(event: DragEvent): void {
    const externalPaths = this.extractExternalPaths(event);
    if (externalPaths.length === 0) return;

    const source: FileInfo[] = externalPaths.map(path => ({
      name: path.split(/[/\\]/).pop() || '',
      path: path,
      type: FileType.FILE,
      size: 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
    }));

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

  private buildRangeSelection(entry: FileInfo): FileInfo[] {
    const selectable = this.entries.filter(e => !this.isParentEntry(e));
    const anchorIndex = selectable.findIndex(e => e.path === this.anchorPath);
    const currentIndex = selectable.findIndex(e => e.path === entry.path);

    if (anchorIndex === -1 || currentIndex === -1) {
      return [entry];
    }

    const [start, end] = [Math.min(anchorIndex, currentIndex), Math.max(anchorIndex, currentIndex)];
    return selectable.slice(start, end + 1);
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

  /** Build source entries from dragged paths; uses pane entries when available (same-pane drag) or constructs minimal FileInfo for cross-pane drags. */
  private buildSourceFromEntries(dragged: { path: string; type: FileType; name: string }[]): FileInfo[] {
    const paths = dragged.map(d => d.path);
    const fromEntries = this.entries.filter(e => paths.includes(e.path) && !this.isParentEntry(e));
    if (fromEntries.length > 0) return fromEntries;
    // Cross-pane drag: reconstruct minimal FileInfo using actual types from payload
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
      // backward compat: old payload used 'paths' without type
      if (Array.isArray(parsed?.paths)) return parsed.paths.map((p: string) => ({ path: p, type: FileType.FILE, name: p.split(/[\/\\]/).pop() || p }));
      return [];
    } catch {
      return [];
    }
  }
}
