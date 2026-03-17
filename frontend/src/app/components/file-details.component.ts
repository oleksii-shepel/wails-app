// file-details.component.ts
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
import { FileEntryComponent } from './file-entry.component';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { parentPath } from './address-bar.utils';
import { VscodeScrollbarDirective } from '../directives/vscode-scrollbar.directive';

export interface ColumnDef {
  field: keyof FileInfo | string;
  header: string;
  width?: string;
  flex?: number;
  sortable?: boolean;
  formatter?: (value: any, entry: FileInfo) => string;
}

@Component({
  selector: 'app-file-details',
  standalone: true,
  imports: [CommonModule, FileEntryComponent, VscodeScrollbarDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="file-details" role="table">
      <div class="details-header" role="rowgroup">
        <div class="details-row header-row" role="row">
          <div 
            *ngFor="let col of columns; let i = index"
            class="details-cell header-cell"
            [attr.data-col]="getColumnKey(col)"
            [class.sortable]="col.sortable"
            [style.width]="getColumnWidth(col)"
            [style.flex]="getColumnFlex(col)"
            [attr.role]="'columnheader'"
            [attr.aria-sort]="getSortState(col.field)"
            (click)="col.sortable && onSort(col.field)">
            
            <span class="header-label">{{ col.header }}</span>
            <span class="sort-indicator" *ngIf="sortField === col.field"
                  [class.sort-asc]="sortDirection === 'asc'"
                  [class.sort-desc]="sortDirection === 'desc'">
              <svg viewBox="0 0 16 16">
                <path d="M6 4l4 4-4 4" fill="none" stroke="currentColor" stroke-width="2"/>
              </svg>
            </span>
            <div
              *ngIf="i < columns.length - 1"
              class="view-divider"
              title="Resize column"
              (mousedown)="onColumnResizeStart($event, col)">
            </div>
          </div>
        </div>
      </div>
      
      <div
        #detailsBody
        class="details-body"
        appVscodeScrollbar
        [class.drop-target-bg]="isBackgroundDropTarget"
        role="rowgroup"
        (mousedown)="onBodyMouseDown($event)"
        (click)="onBackgroundClick($event)"
        (contextmenu)="onBackgroundContextMenu($event)"
        (dragover)="onBackgroundDragOver($event)"
        (dragleave)="onBackgroundDragLeave($event)"
        (drop)="onBackgroundDrop($event)">
        
        <div class="virtual-spacer" *ngIf="virtualEnabled" [style.height.px]="totalContentHeight"></div>
        
        <div class="details-window" [class.virtual-window]="virtualEnabled" [style.transform]="virtualWindowTransform">
          <div 
            *ngFor="let entry of visibleEntries; trackBy: trackByPath"
            class="details-row"
            [attr.data-path]="entry.path"
            [style.height.px]="virtualEnabled ? rowHeight : null"
            [class.selected]="isSelected(entry)"
            [class.cursor]="entry.path === cursorPath"
            [class.parent-dir]="isParentEntry(entry)"
            [class.drag-over]="isDragOverInside(entry)"
            [class.drag-over-before]="isDragOverBefore(entry)"
            [class.drag-over-after]="isDragOverAfter(entry)"
            [attr.role]="'row'"
            [attr.aria-selected]="isSelected(entry)"
            [draggable]="!isParentEntry(entry)"
            (click)="onRowClick(entry, $event)"
            (auxclick)="onRowAuxClick($event, entry)"
            (dblclick)="onRowDoubleClick(entry)"
            (contextmenu)="onContextMenu($event, entry)"
            (dragstart)="onDragStart($event, entry)"
            (dragover)="onDragOver($event, entry)"
            (dragleave)="onDragLeave(entry)"
            (drop)="onDrop($event, entry)">
            <div *ngIf="isDragOverBefore(entry)" class="drop-indicator drop-indicator-before"></div>
            <div *ngIf="isDragOverAfter(entry)" class="drop-indicator drop-indicator-after"></div>
            
            <div 
              *ngFor="let col of columns; let i = index"
              class="details-cell"
              [style.width]="getColumnWidth(col)"
              [style.flex]="getColumnFlex(col)"
              [attr.role]="'cell'">
              
              <ng-container *ngIf="col.field === 'name'">
                <app-file-entry
                  [entry]="entry"
                  [selected]="isSelected(entry)"
                  [cursor]="entry.path === cursorPath"
                  variant="inline"
                  (rename)="onRename($event)">
                </app-file-entry>
              </ng-container>
              
              <ng-container *ngIf="col.field !== 'name'">
                {{ getCellValue(entry, col) }}
              </ng-container>
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

    .file-details {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-editor-background);
    }

    .details-header {
      flex-shrink: 0;
      background: var(--vsc-tabbar-background);
      border-bottom: 1px solid var(--vsc-border);
    }

    .details-row {
      display: flex;
      align-items: center;
      height: 24px;
      padding: 0 var(--vsc-padding-md);
      border-bottom: 1px solid transparent;
      flex-shrink: 0;
      contain: layout style;
      position: relative;
      
      &.header-row {
        height: 28px;
        font-weight: 600;
        color: var(--vsc-foreground-dim);
      }
      
      &:not(.header-row) {
        cursor: pointer;
        user-select: none;
        
        &:hover {
          background: var(--vsc-list-hover-background);
        }
        
        &.selected {
          background: var(--vsc-list-active-background);
          color: var(--vsc-foreground-bright);
          
          .file-name-text {
            font-weight: 500;
          }
        }
        
        /* TC cursor indicator */
        &.cursor:not(.selected) {
          background: color-mix(in srgb, var(--vsc-list-active-background) 35%, transparent);
          /* Use inset shadow so cursor highlight never shifts layout. */
          box-shadow: inset 2px 0 0 0 var(--vsc-accent-blue);
        }
        
        &.cursor.selected {
          outline: none;
        }
        
        &.parent-dir {
          opacity: 0.7;
          
          .file-name-text {
            font-style: italic;
          }
        }

        &.drop-target {
          outline: 1px dashed var(--vsc-focus-border);
          outline-offset: -1px;
          background: var(--vsc-list-hover-background);
        }

        &.drag-over {
          background-color: var(--vsc-list-drop-background, rgba(0, 120, 212, 0.2));
          outline: 1px dashed var(--vsc-focus-border);
          outline-offset: -1px;
        }
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

    .details-cell {
      display: flex;
      position: relative;
      align-items: center;
      padding: 0 var(--vsc-padding-sm);
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
      font-size: var(--vsc-font-size);
      
      &.header-cell {
        position: relative;
        &.sortable {
          cursor: pointer;
          
          &:hover {
            background: var(--vsc-list-hover-background);
          }
        }
      }

      .view-divider {
        position: absolute;
        top: 0;
        bottom: 0;
        right: 0;
        width: 1px;
        background: var(--vsc-border);
        pointer-events: none;
      }

      &.header-cell .view-divider {
        right: -3px;
        width: 6px;
        cursor: url("data:image/svg+xml,%3Csvg%20xmlns=%27http://www.w3.org/2000/svg%27%20width=%2730%27%20height=%2730%27%20viewBox=%270%200%2024%2024%27%3E%3Cpath%20fill=%27white%27%20d=%27M11%202h2v20h-2zM6.29%207.29l-4%204a.996.996%200%200%200%200%201.41l4%204%201.41-1.41L4.41%2012%207.7%208.71%206.29%207.3ZM17.71%207.29%2016.3%208.7l3.29%203.29-3.29%203.29%201.41%201.41%204-4a.996.996%200%200%200%200-1.41l-4-4Z%27/%3E%3C/svg%3E") 15 15, col-resize;
        pointer-events: auto;
        background: transparent;
        z-index: 3;
      }

      &.header-cell .view-divider::before {
        content: '';
        position: absolute;
        top: 2px;
        bottom: 2px;
        left: 50%;
        width: 1px;
        transform: translateX(-50%);
        background: var(--vsc-border);
      }

      &.header-cell .view-divider:hover::before {
        background: var(--vsc-focus-border);
      }
      
      .header-label {
        overflow: hidden;
        text-overflow: ellipsis;
        margin-left: var(--vsc-padding-lg);
        margin-right: var(--vsc-padding-lg);
      }
      
      .sort-indicator {
        margin-left: var(--vsc-padding-md);
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: 12px;
        height: 12px;
        flex: 0 0 12px;
        font-size: 12px;
        line-height: 1;
        transform-origin: 50% 50%;
        transition: transform 0.15s ease;

        &.sort-asc  { transform: rotate(-90deg); }
        &.sort-desc { transform: rotate(90deg); }
      }
    }

    .details-body {
      flex: 1;
      position: relative;
      overflow-y: auto;
      overflow-x: hidden;
    }
    
    .virtual-spacer {
      width: 1px;
      pointer-events: none;
    }
    
    .details-window.virtual-window {
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

    .file-icon {
      margin-right: var(--vsc-padding-sm);
      font-size: 14px;
      flex-shrink: 0;
    }

    .dir-symlink-icon {
      position: relative;
      &::after {
        content: '↗';
        position: absolute;
        bottom: -2px;
        right: -4px;
        font-size: 8px;
        line-height: 1;
        opacity: 0.8;
      }
    }

    .file-name-text {
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .col-name { flex: 2; }
    .col-size { width: 100px; text-align: right; justify-content: flex-end; }
    .col-date { width: 150px; }
    .col-type { width: 100px; }

    .details-body.drop-target-bg {
      outline: 2px dashed var(--vsc-focus-border);
      outline-offset: -2px;
      background: color-mix(in srgb, var(--vsc-list-hover-background) 30%, transparent);
    }
  `]
})
export class FileDetailsComponent implements OnDestroy, OnChanges, AfterViewInit {
  FileType = FileType;
  
  constructor(
    private zone: NgZone,
    private cdr: ChangeDetectorRef
  ) {}

  @ViewChild('detailsBody') detailsBodyRef?: ElementRef<HTMLElement>;
  @ViewChildren(FileEntryComponent) fileEntryComponents?: QueryList<FileEntryComponent>;

  @Input() entries: FileInfo[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  @Input() cursorPath: string | null = null;
  @Input() columns: ColumnDef[] = [
    { field: 'name', header: 'Name', flex: 2, sortable: true },
    { field: 'size', header: 'Size', width: '100px', sortable: true, formatter: (v) => this.formatSize(v) },
    { field: 'modified', header: 'Modified', width: '150px', sortable: true, formatter: (v) => this.formatDate(v) },
    { field: 'type', header: 'Type', width: '100px', formatter: (_, entry) => this.getFileTypeLabel(entry) }
  ];
  @Input() sortField: string = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';
  @Input() paneActive = true;
  
  @Output() selectionChange = new EventEmitter<FileInfo[]>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() openInNewTab = new EventEmitter<FileInfo>();
  @Output() sort = new EventEmitter<{ field: string; direction: 'asc' | 'desc' }>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() dragStart = new EventEmitter<{ event: DragEvent; entries: FileInfo[] }>();
  @Output() dragDrop = new EventEmitter<{ event: DragEvent; source: FileInfo[]; target: FileInfo }>();
  @Output() rename = new EventEmitter<{ entry: FileInfo; newName: string }>();

  private readonly VIRTUAL_THRESHOLD = 100;
  private readonly DEFAULT_ROW_HEIGHT = 24;
  rowHeight = 24;
  private readonly overscan = 5;
  private scrollRAF: number | null = null;
  private boundScrollListener?: () => void;
  private resizeObserver?: ResizeObserver;

  private virtualStart = 0;
  private virtualEnd = 0;
  private viewportHeight = 0;

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
  private marqueeButton: 0 | 2 | null = null;
  private hasMarqueeMoved = false;
  private pendingClearOnMove = false;
  private windowMouseMove?: (event: MouseEvent) => void;
  private windowMouseUp?: (event: MouseEvent) => void;
  private windowContextMenu?: (event: MouseEvent) => void;
  private readonly MIN_COLUMN_WIDTH = 80;

  onRename(event: { entry: FileInfo; newName: string }): void {
    this.rename.emit(event);
  }

  /** Called by parent (eg. on F2) to begin in-place rename for the current selection. */
  startRenameSelected(): void {
    const selectedPath = this.getFirstSelectedPathInOrder();
    if (!selectedPath) return;
    this.startRenameByPath(selectedPath);
  }

  /** Scroll so the given path is visible. */
  scrollToPath(path: string): void {
    if (!path || path === PARENT_ENTRY_PATH) return;
    const host = this.detailsBodyRef?.nativeElement;
    if (!host) return;
    if (this.virtualEnabled) {
      const idx = this.entries.findIndex(e => e.path === path);
      if (idx >= 0) {
        const itemTop = idx * this.rowHeight;
        const itemBottom = itemTop + this.rowHeight;
        if (itemTop < host.scrollTop || itemBottom > host.scrollTop + host.clientHeight) {
          const centerOffset = host.clientHeight / 2 - this.rowHeight / 2;
          host.scrollTop = Math.max(0, itemTop - centerOffset);
        }
        this.updateVirtualWindow();
        this.cdr.markForCheck();
      }
    } else {
      const el = host.querySelector<HTMLElement>(`.details-row[data-path="${CSS.escape(path)}"]`);
      el?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    }
  }

  /** Approximate number of rows visible in the viewport. */
  getApproxPageSize(): number {
    const host = this.detailsBodyRef?.nativeElement;
    if (!host) return 10;
    return Math.max(1, Math.floor(host.clientHeight / this.rowHeight) - 1);
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
    const host = this.detailsBodyRef?.nativeElement;
    if (host) {
      if (this.virtualEnabled) {
        const idx = this.entries.findIndex(e => e.path === path);
        if (idx >= 0) {
          const targetTop = idx * this.rowHeight;
          const centerOffset = host.clientHeight / 2 - this.rowHeight / 2;
          host.scrollTop = Math.max(0, targetTop - centerOffset);
          this.updateVirtualWindow();
          this.cdr.markForCheck();
        }
      } else {
        const el = host.querySelector<HTMLElement>(`.details-row[data-path="${CSS.escape(path)}"]`);
        el?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
      }
    }

    requestAnimationFrame(() => {
      const comp = this.fileEntryComponents?.find(c => c.entry?.path === path);
      comp?.beginRename();
      this.cdr.markForCheck();
    });
  }
  private columnWidths = new Map<string, number>();
  private resizingColumnKey: string | null = null;
  private resizeStartX = 0;
  private resizeStartWidth = 0;
  private boundResizeMouseMove?: (event: MouseEvent) => void;
  private boundResizeMouseUp?: () => void;

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
    return this.entries.length >= this.VIRTUAL_THRESHOLD;
  }

  get visibleEntries(): FileInfo[] {
    if (!this.virtualEnabled) return this.entries;
    return this.entries.slice(this.virtualStart, this.virtualEnd);
  }

  get totalContentHeight(): number {
    return this.entries.length * this.rowHeight;
  }

  get virtualWindowTransform(): string {
    if (!this.virtualEnabled) return '';
    return `translateY(${this.virtualStart * this.rowHeight}px)`;
  }

  getColumnKey(col: ColumnDef): string {
    return String(col.field);
  }

  getColumnWidth(col: ColumnDef): string | null {
    const key = this.getColumnKey(col);
    const resized = this.columnWidths.get(key);
    if (typeof resized === 'number') {
      return `${resized}px`;
    }
    return col.width ?? null;
  }

  getColumnFlex(col: ColumnDef): string {
    const key = this.getColumnKey(col);
    if (this.columnWidths.has(key) || !!col.width) {
      return 'none';
    }
    return col.flex ? String(col.flex) : '1';
  }

  ngAfterViewInit(): void {
    const element = this.detailsBodyRef?.nativeElement;
    this.viewportHeight = element?.clientHeight || 0;
    this.setupResizeObserver();
    this.setupScrollListener();
    this.updateVirtualWindow();
    requestAnimationFrame(() => this.measureRowHeight());
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['entries'] || changes['columns']) {
      const entriesChange = changes['entries'];
      if (entriesChange) {
        const prevLen = entriesChange.previousValue?.length || 0;
        const currLen = entriesChange.currentValue?.length || 0;
        if (Math.abs(prevLen - currLen) > 1000) {
          this.resetScroll();
        }
      }
      this.updateVirtualWindow();
      queueMicrotask(() => requestAnimationFrame(() => this.measureRowHeight()));
      this.cdr.markForCheck();
    }
  }

  ngOnDestroy(): void {
    this.detachMarqueeListeners();
    this.cleanupResizeObserver();
    this.cleanupScrollListener();
    this.cleanupColumnResizeListeners();
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    if (this.scrollRAF) {
      cancelAnimationFrame(this.scrollRAF);
    }
  }

  onColumnResizeStart(event: MouseEvent, col: ColumnDef): void {
    event.preventDefault();
    event.stopPropagation();

    const key = this.getColumnKey(col);
    const headerCell = (event.target as HTMLElement).closest('.header-cell') as HTMLElement | null;
    if (!headerCell) return;

    this.resizingColumnKey = key;
    this.resizeStartX = event.clientX;
    this.resizeStartWidth = Math.max(this.MIN_COLUMN_WIDTH, Math.round(headerCell.getBoundingClientRect().width));
    this.columnWidths.set(key, this.resizeStartWidth);

    this.boundResizeMouseMove = (moveEvent: MouseEvent) => this.onColumnResizeMove(moveEvent);
    this.boundResizeMouseUp = () => this.onColumnResizeEnd();
    window.addEventListener('mousemove', this.boundResizeMouseMove);
    window.addEventListener('mouseup', this.boundResizeMouseUp);
    document.body.style.userSelect = 'none';
    this.cdr.markForCheck();
  }

  private onColumnResizeMove(event: MouseEvent): void {
    if (!this.resizingColumnKey) return;
    const delta = event.clientX - this.resizeStartX;
    const next = Math.max(this.MIN_COLUMN_WIDTH, this.resizeStartWidth + delta);
    this.columnWidths.set(this.resizingColumnKey, next);
    this.cdr.markForCheck();
  }

  private onColumnResizeEnd(): void {
    this.resizingColumnKey = null;
    this.cleanupColumnResizeListeners();
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    this.cdr.markForCheck();
  }

  private cleanupColumnResizeListeners(): void {
    if (this.boundResizeMouseMove) {
      window.removeEventListener('mousemove', this.boundResizeMouseMove);
      this.boundResizeMouseMove = undefined;
    }
    if (this.boundResizeMouseUp) {
      window.removeEventListener('mouseup', this.boundResizeMouseUp);
      this.boundResizeMouseUp = undefined;
    }
  }

  private setupResizeObserver(): void {
    if (!this.detailsBodyRef?.nativeElement) return;
    
    this.resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        this.viewportHeight = entry.contentRect.height;
        this.updateVirtualWindow();
        this.cdr.markForCheck();
      }
    });
    
    this.resizeObserver.observe(this.detailsBodyRef.nativeElement);
  }

  private cleanupResizeObserver(): void {
    this.resizeObserver?.disconnect();
  }

  private setupScrollListener(): void {
    const element = this.detailsBodyRef?.nativeElement;
    if (!element) return;
    this.boundScrollListener = () => this.onScroll();
    element.addEventListener('scroll', this.boundScrollListener, { passive: true });
  }

  private cleanupScrollListener(): void {
    const element = this.detailsBodyRef?.nativeElement;
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
    const element = this.detailsBodyRef?.nativeElement;
    if (element) {
      element.scrollTop = 0;
    }
    this.virtualStart = 0;
    this.virtualEnd = Math.min(this.overscan * 2, this.entries.length);
  }

  private updateVirtualWindow(): void {
    if (!this.virtualEnabled) {
      this.virtualStart = 0;
      this.virtualEnd = this.entries.length;
      return;
    }

    const element = this.detailsBodyRef?.nativeElement;
    const scrollTop = element?.scrollTop ?? 0;

    const viewportHeight = this.viewportHeight || element?.clientHeight || 0;
    if (viewportHeight <= 0) {
      this.virtualStart = 0;
      this.virtualEnd = Math.min(this.entries.length, this.overscan * 4);
      return;
    }
    const visibleCount = Math.max(1, Math.ceil(viewportHeight / this.rowHeight));
    
    const start = Math.max(0, Math.floor(scrollTop / this.rowHeight) - this.overscan);
    const end = Math.min(this.entries.length, start + visibleCount + this.overscan * 2);

    if (start !== this.virtualStart || end !== this.virtualEnd) {
      this.virtualStart = start;
      this.virtualEnd = end;
    }
  }

  private measureRowHeight(): void {
    const host = this.detailsBodyRef?.nativeElement;
    if (!host) return;
    const row = host.querySelector<HTMLElement>('.details-row[data-path]');
    if (!row) return;
    const measured = Math.ceil(row.getBoundingClientRect().height);
    if (!Number.isFinite(measured) || measured <= 0) return;
    const next = Math.max(this.DEFAULT_ROW_HEIGHT, measured);
    if (Math.abs(next - this.rowHeight) < 1) return;
    this.rowHeight = next;
    this.updateVirtualWindow();
    this.cdr.markForCheck();
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

  onRowClick(entry: FileInfo, event: MouseEvent): void {
    if (this.isParentEntry(entry)) return;

    if (event.ctrlKey || event.metaKey) {
      this.selectionChange.emit(this.buildToggledSelection(entry));
      this.anchorPath = entry.path;
    } else if (event.shiftKey && this.anchorPath) {
      this.rangeSelect(entry);
    } else {
      this.anchorPath = entry.path;
      this.selectionChange.emit([entry]);
    }
  }

  onRowDoubleClick(entry: FileInfo): void {
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

  onRowAuxClick(event: MouseEvent, entry: FileInfo): void {
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
    // Background click just focuses the pane (handled by parent wrapper).
    // Never clear selection here — the pane-inactive CSS dims the cursor automatically.
  }

  onBackgroundContextMenu(event: MouseEvent): void {
    if (this.suppressBackgroundContextMenu) {
      event.preventDefault();
      this.suppressBackgroundContextMenu = false;
      return;
    }

    if (event.target === event.currentTarget) {
      event.preventDefault();
    }
  }

  onBodyMouseDown(event: MouseEvent): void {
    if (event.button !== 0 && event.button !== 2) return;
    if (event.button === 0 && event.target !== event.currentTarget) return;

    const container = event.currentTarget as HTMLElement;
    this.marqueeContainer = container;
    this.marqueeAppend = event.ctrlKey || event.metaKey;
    this.marqueeButton = event.button as 0 | 2;
    this.hasMarqueeMoved = false;
    // Defer selection clear until the marquee actually moves — a plain background
    // click keeps selection so the cursor stays visible in both active and inactive panes.
    this.pendingClearOnMove = !this.marqueeAppend;

    const { x, y } = this.getRelativePoint(event, container);
    this.marqueeStartX = x;
    this.marqueeStartY = y;
    this.selectionRect = { active: true, left: x, top: y, width: 0, height: 0 };

    this.suppressBackgroundClick = true;
    this.suppressBackgroundContextMenu = event.button === 2;

    this.windowMouseMove = (moveEvent: MouseEvent) => this.zone.run(() => this.onBodyMouseMove(moveEvent));
    this.windowMouseUp = () => this.zone.run(() => this.finishMarqueeSelection());
    this.windowContextMenu = (contextEvent: MouseEvent) => {
      if (this.selectionRect.active && this.hasMarqueeMoved) {
        contextEvent.preventDefault();
        contextEvent.stopPropagation();
      }
    };

    window.addEventListener('mousemove', this.windowMouseMove);
    window.addEventListener('mouseup', this.windowMouseUp);
    window.addEventListener('contextmenu', this.windowContextMenu, true);
    
    if (event.button === 0) {
      event.preventDefault();
    }
  }

  private onBodyMouseMove(event: MouseEvent): void {
    if (!this.marqueeContainer || !this.selectionRect.active) return;

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

  private emitMarqueeSelection(): void {
    if (!this.marqueeContainer) return;

    const containerRect = this.marqueeContainer.getBoundingClientRect();
    const selectionPaths = new Set<string>();
    const rows = this.marqueeContainer.querySelectorAll<HTMLElement>('.details-row[data-path]');

    for (const row of Array.from(rows)) {
      const path = row.dataset['path'];
      if (!path || path === PARENT_ENTRY_PATH) continue;

      const rowRect = row.getBoundingClientRect();
      const rowLeft = rowRect.left - containerRect.left + this.marqueeContainer.scrollLeft;
      const rowTop = rowRect.top - containerRect.top + this.marqueeContainer.scrollTop;
      
      const intersects =
        rowLeft < this.selectionRect.left + this.selectionRect.width &&
        rowLeft + rowRect.width > this.selectionRect.left &&
        rowTop < this.selectionRect.top + this.selectionRect.height &&
        rowTop + rowRect.height > this.selectionRect.top;

      if (intersects) selectionPaths.add(path);
    }

    if (this.marqueeAppend) {
      for (const path of this.selectedFiles) {
        if (path !== PARENT_ENTRY_PATH) selectionPaths.add(path);
      }
    }

    const selectedEntries = this.entries.filter(entry => selectionPaths.has(entry.path));
    this.selectionChange.emit(selectedEntries);
  }

  private finishMarqueeSelection(): void {
    this.detachMarqueeListeners();
    this.marqueeContainer = null;
    this.marqueeButton = null;
    this.hasMarqueeMoved = false;
    this.pendingClearOnMove = false;
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

  onSort(field: string): void {
    const newDirection = this.sortField === field && this.sortDirection === 'asc' ? 'desc' : 'asc';
    this.sort.emit({ field, direction: newDirection });
  }

  getSortState(field: string): string | null {
    if (this.sortField !== field) return null;
    return this.sortDirection === 'asc' ? 'ascending' : 'descending';
  }

  getCellValue(entry: FileInfo, col: ColumnDef): string {
    const value = (entry as any)[col.field];
    if (col.formatter) {
      return col.formatter(value, entry);
    }
    return value?.toString() || '';
  }

  getIcon(entry: FileInfo): string {
    if (this.isParentEntry(entry)) return '↩';
    if (entry.type === FileType.DIRECTORY) return '📁';
    if (entry.type === FileType.SYMLINK) return '🔗';
    return '📄';
  }

  getFileTypeLabel(entry: FileInfo): string {
    if (entry.type === FileType.DIRECTORY) return entry.isSymlink ? 'Symlink' : 'Folder';
    if (entry.type === FileType.SYMLINK) return 'Shortcut';
    const ext = entry.name.split('.').pop()?.toUpperCase();
    return ext ? `${ext} File` : 'File';
  }

  formatSize(bytes: number): string {
    if (bytes === 0) return '';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
  }

  formatDate(timestamp: number): string {
    return new Date(timestamp * 1000).toLocaleString();
  }

  private rangeSelect(entry: FileInfo): void {
    const selectable = this.entries.filter(e => !this.isParentEntry(e));
    const startIdx = selectable.findIndex(e => e.path === this.anchorPath);
    const endIdx = selectable.findIndex(e => e.path === entry.path);

    if (startIdx === -1 || endIdx === -1) return;

    const [min, max] = [Math.min(startIdx, endIdx), Math.max(startIdx, endIdx)];
    const range: FileInfo[] = [];
    
    for (let i = min; i <= max; i++) {
      if (!this.isParentEntry(selectable[i])) {
        range.push(selectable[i]);
      }
    }
    this.selectionChange.emit(range);
  }

  onDragStart(event: DragEvent, entry: FileInfo): void {
    if (event.button !== 0) {
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
        source: 'details',
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
    const container = this.detailsBodyRef?.nativeElement;
    
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
    const paths = dragged.map(d => d.path);
    const fromEntries = this.entries.filter(e => paths.includes(e.path) && !this.isParentEntry(e));
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
}
