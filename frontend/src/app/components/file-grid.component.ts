// file-grid.component.ts - FULLY OPTIMIZED FOR MASSIVE FOLDERS
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
  NgZone,
  ChangeDetectionStrategy,
  ChangeDetectorRef,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileEntryComponent } from './file-entry.component';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { parentPath } from './address-bar.utils';

@Component({
  selector: 'app-file-grid',
  standalone: true,
  imports: [CommonModule, FileEntryComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      #gridContainer
      class="file-grid"
      [class.virtual]="virtualEnabled"
      (mousedown)="onGridMouseDown($event)"
      (click)="onBackgroundClick($event)"
      (contextmenu)="onBackgroundContextMenu($event)">
      
      <div class="virtual-spacer" *ngIf="virtualEnabled" [style.height.px]="totalContentHeight"></div>
      
      <div class="grid-window" [class.virtual-window]="virtualEnabled" [style.transform]="virtualWindowTransform">
        <div 
          *ngFor="let entry of visibleEntries; trackBy: trackByPath"
          class="grid-item"
          [attr.data-path]="entry.path"
          [class.selected]="isSelected(entry)"
          [class.directory]="entry.type === FileType.DIRECTORY"
          [class.parent-dir]="isParentEntry(entry)"
          [attr.role]="'gridcell'"
          [attr.aria-selected]="isSelected(entry)"
          [class.drop-target]="isDropTarget(entry)"
          [draggable]="!isParentEntry(entry)"
          (click)="onItemClick(entry, $event)"
          (auxclick)="onItemAuxClick($event, entry)"
          (dblclick)="onItemDoubleClick(entry)"
          (contextmenu)="onContextMenu($event, entry)"
          (dragstart)="onDragStart($event, entry)"
          (dragover)="onDragOver($event, entry)"
          (dragleave)="onDragLeave(entry)"
          (drop)="onDrop($event, entry)">
          
          <div class="grid-info">
            <app-file-entry
              [entry]="entry"
              [selected]="isSelected(entry)"
              variant="grid">
            </app-file-entry>
            <div class="grid-meta" *ngIf="!isParentEntry(entry)">
              {{ formatSize(entry.size) }}
            </div>
          </div>
        </div>
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
      grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
      align-content: start;
      gap: var(--vsc-padding-md);
      padding: var(--vsc-padding-md);
      overflow-y: auto;
      overflow-x: hidden;
      background: var(--vsc-editor-background);
      scrollbar-width: thin;
      scrollbar-color: rgba(121, 121, 121, 0.4) transparent;

      &::-webkit-scrollbar {
        width: 10px;
      }

      &::-webkit-scrollbar-track {
        background: transparent;
      }

      &::-webkit-scrollbar-thumb {
        background: rgba(121, 121, 121, 0.4);
        border-radius: 0;

        &:hover {
          background: rgba(100, 100, 100, 0.7);
        }

        &:active {
          background: rgba(85, 85, 85, 0.8);
        }
      }

      &::-webkit-scrollbar-corner {
        background: transparent;
      }
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
      grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
      align-content: start;
      gap: var(--vsc-padding-md);
      padding: 0 var(--vsc-padding-md);
      will-change: transform;
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
      padding: var(--vsc-padding-md);
      border-radius: var(--vsc-border-radius);
      cursor: pointer;
      transition: all var(--vsc-transition-fast);
      border: 2px solid transparent;
      position: relative;
      contain: layout style paint;
      
      &:hover {
        background: var(--vsc-list-hover-background);
      }
      
      &.selected {
        background: var(--vsc-list-active-background);
        border-color: var(--vsc-accent-blue);
        
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
        opacity: 0.7;
        
        .grid-name {
          font-style: italic;
        }
      }

      &.drop-target {
        outline: 1px dashed var(--vsc-focus-border);
        outline-offset: -2px;
        background: var(--vsc-list-hover-background);
      }
    }

    .grid-icon {
      position: relative;
      width: 48px;
      height: 48px;
      display: flex;
      align-items: center;
      justify-content: center;
      margin-bottom: var(--vsc-padding-sm);
    }

    .file-emoji {
      font-size: 32px;
      line-height: 1;
      filter: drop-shadow(0 2px 4px rgba(0,0,0,0.2));
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
      line-height: 1.4;
    }

    .grid-meta {
      font-size: 11px;
      color: var(--vsc-foreground-dim);
      margin-top: 2px;
    }

    :host-context(.compact-mode) {
      .file-grid,
      .grid-window.virtual-window {
        grid-template-columns: repeat(auto-fill, minmax(80px, 1fr));
        gap: var(--vsc-padding-sm);
      }
      
      .grid-icon {
        width: 36px;
        height: 36px;
      }
      
      .file-emoji {
        font-size: 24px;
      }
      
      .grid-item {
        padding: var(--vsc-padding-sm);
      }
    }
  `]
})
export class FileGridComponent implements OnDestroy, OnChanges, AfterViewInit {
  constructor(
    private zone: NgZone,
    private cdr: ChangeDetectorRef
  ) {}

  @ViewChild('gridContainer') gridContainerRef?: ElementRef<HTMLElement>;

  @Input() entries: FileInfo[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  
  @Output() selectionChange = new EventEmitter<FileInfo[]>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() openInNewTab = new EventEmitter<FileInfo>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() dragStart = new EventEmitter<{ event: DragEvent; entries: FileInfo[] }>();
  @Output() dragDrop = new EventEmitter<{ event: DragEvent; source: FileInfo[]; target: FileInfo }>();

  FileType = FileType;
  
  // OPTIMIZED: Lower threshold for earlier virtualization activation
  private readonly VIRTUAL_THRESHOLD = 200;
  private readonly rowHeight = 124;
  private readonly overscanRows = 2;
  private scrollRAF: number | null = null;
  private resizeObserver?: ResizeObserver;

  private virtualStart = 0;
  private virtualEnd = 0;
  private virtualColumns = 1;
  private lastScrollTop = 0;
  private viewportHeight = 0;

  private anchorPath: string | null = null;
  private dropTargetPath: string | null = null;
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

  get virtualEnabled(): boolean {
    return this.entries.length >= this.VIRTUAL_THRESHOLD;
  }

  get totalContentHeight(): number {
    const rows = Math.ceil(this.entries.length / this.virtualColumns);
    return rows * this.rowHeight;
  }

  get virtualWindowTransform(): string {
    if (!this.virtualEnabled) return '';
    const startRow = Math.floor(this.virtualStart / this.virtualColumns);
    return `translateY(${startRow * this.rowHeight}px)`;
  }

  get visibleEntries(): FileInfo[] {
    if (!this.virtualEnabled) return this.entries;
    return this.entries.slice(this.virtualStart, this.virtualEnd);
  }

  ngAfterViewInit(): void {
    this.setupResizeObserver();
    this.setupScrollListener();
    this.updateVirtualWindow();
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['entries']) {
      const prevLen = changes['entries'].previousValue?.length || 0;
      const currLen = changes['entries'].currentValue?.length || 0;
      if (Math.abs(prevLen - currLen) > 1000) {
        this.resetScroll();
      }
      this.updateVirtualWindow();
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
    if (!this.gridContainerRef?.nativeElement) return;
    
    this.resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        this.viewportHeight = entry.contentRect.height;
        this.updateVirtualWindow();
        this.cdr.markForCheck();
      }
    });
    
    this.resizeObserver.observe(this.gridContainerRef.nativeElement);
  }

  private cleanupResizeObserver(): void {
    this.resizeObserver?.disconnect();
  }

  private setupScrollListener(): void {
    const element = this.gridContainerRef?.nativeElement;
    if (!element) return;
    element.addEventListener('scroll', this.onScroll.bind(this), { passive: true });
  }

  private cleanupScrollListener(): void {
    // Auto-removed with element destruction
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
    const element = this.gridContainerRef?.nativeElement;
    if (element) {
      element.scrollTop = 0;
      this.lastScrollTop = 0;
    }
    this.virtualStart = 0;
    this.virtualEnd = Math.min(this.overscanRows * this.virtualColumns * 2, this.entries.length);
  }

  private updateVirtualWindow(): void {
    if (!this.virtualEnabled) {
      this.virtualStart = 0;
      this.virtualEnd = this.entries.length;
      this.virtualColumns = 1;
      return;
    }

    const host = this.gridContainerRef?.nativeElement;
    const width = Math.max(1, host?.clientWidth ?? 1);
    const viewportHeight = this.viewportHeight || host?.clientHeight || 0;
    const scrollTop = host?.scrollTop ?? 0;

    // Calculate columns based on CSS grid logic
    const cellMin = 100;
    const gap = 8;
    const pad = 16;
    const contentWidth = Math.max(1, width - pad * 2);
    const cols = Math.max(1, Math.floor((contentWidth + gap) / (cellMin + gap)));
    
    if (cols !== this.virtualColumns) {
      this.virtualColumns = cols;
    }

    const startRow = Math.max(0, Math.floor(scrollTop / this.rowHeight) - this.overscanRows);
    const visibleRows = Math.max(1, Math.ceil(viewportHeight / this.rowHeight));
    const endRow = startRow + visibleRows + this.overscanRows * 2;

    const start = startRow * cols;
    const end = Math.min(this.entries.length, endRow * cols);

    if (start !== this.virtualStart || end !== this.virtualEnd) {
      this.virtualStart = start;
      this.virtualEnd = end;
    }
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

  isDropTarget(entry: FileInfo): boolean {
    return this.dropTargetPath === entry.path;
  }

  onItemClick(entry: FileInfo, event: MouseEvent): void {
    if (event.ctrlKey || event.metaKey) {
      this.selectionChange.emit(this.buildToggledSelection(entry));
      this.anchorPath = entry.path;
    } else if (event.shiftKey && this.anchorPath) {
      this.selectionChange.emit(this.rangeSelect(entry));
    } else {
      if (this.isParentEntry(entry)) return;
      this.anchorPath = entry.path;
      this.selectionChange.emit([entry]);
    }
  }

  onItemDoubleClick(entry: FileInfo): void {
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

  onBackgroundClick(event: MouseEvent): void {
    if (this.suppressBackgroundClick) {
      this.suppressBackgroundClick = false;
      return;
    }

    if (event.target === event.currentTarget) {
      this.selectionChange.emit([]);
      this.anchorPath = null;
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
      this.anchorPath = null;
    }
  }

  onGridMouseDown(event: MouseEvent): void {
    if (event.button !== 0 && event.button !== 2) return;

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

    const containerRect = this.marqueeContainer.getBoundingClientRect();
    const selectionPaths = new Set<string>();
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
        source: 'grid',
        paths: entries.map(e => e.path),
      }));
    }

    this.dragStart.emit({ event, entries });
  }

  onDragOver(event: DragEvent, entry: FileInfo): void {
    if (this.isParentEntry(entry) || entry.type !== FileType.DIRECTORY) return;
    event.preventDefault();
    
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = event.ctrlKey || event.metaKey ? 'copy' : 'move';
    }
    this.dropTargetPath = entry.path;
  }

  onDragLeave(entry: FileInfo): void {
    if (this.dropTargetPath === entry.path) {
      this.dropTargetPath = null;
    }
  }

  onDrop(event: DragEvent, entry: FileInfo): void {
    if (this.isParentEntry(entry)) return;
    event.preventDefault();
    event.stopPropagation();

    this.dropTargetPath = null;

    const paths = this.readDraggedPaths(event);
    if (!paths.length) return;

    const source = this.entries.filter(e => paths.includes(e.path) && !this.isParentEntry(e));
    if (!source.length) return;

    const target = entry.type === FileType.DIRECTORY
      ? entry
      : ({ ...entry, path: parentPath(entry.path), type: FileType.DIRECTORY } as FileInfo);

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

  private readDraggedPaths(event: DragEvent): string[] {
    try {
      const payload = event.dataTransfer?.getData('application/json');
      if (!payload) return [];
      const parsed = JSON.parse(payload);
      return Array.isArray(parsed?.paths) ? parsed.paths : [];
    } catch {
      return [];
    }
  }

  getIcon(entry: FileInfo): string {
    if (this.isParentEntry(entry)) return '↩';
    if (entry.type === FileType.DIRECTORY) return '📁';
    if (entry.type === FileType.SYMLINK) return '🔗';
    
    const ext = entry.name.split('.').pop()?.toLowerCase();
    const iconMap: Record<string, string> = {
      'txt': '📄', 'md': '📝', 'js': '📜', 'ts': '🔷',
      'json': '📋', 'html': '🌐', 'css': '🎨', 'scss': '🎨',
      'jpg': '🖼️', 'png': '🖼️', 'gif': '🖼️', 'svg': '🎭',
      'pdf': '📕', 'zip': '📦', 'mp3': '🎵', 'mp4': '🎬'
    };
    return iconMap[ext || ''] || '📄';
  }

  formatSize(bytes: number): string {
    if (bytes === 0) return '';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
  }
}