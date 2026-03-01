// file-details.component.ts
import { Component, Input, Output, EventEmitter, OnDestroy, NgZone } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { parentPath } from './address-bar.utils';

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
  imports: [CommonModule],
  template: `
    <div class="file-details" role="table">
      <!-- Header -->
      <div class="details-header" role="rowgroup">
        <div class="details-row header-row" role="row">
          <div 
            *ngFor="let col of columns"
            class="details-cell header-cell"
            [class.sortable]="col.sortable"
            [style.width]="col.width"
            [style.flex]="col.width ? 'none' : '1'"
            [attr.role]="'columnheader'"
            [attr.aria-sort]="getSortState(col.field)"
            (click)="col.sortable && onSort(col.field)">
            
            <span class="header-label">{{ col.header }}</span>
            <span class="sort-indicator" *ngIf="sortField === col.field">
              {{ sortDirection === 'asc' ? '▲' : '▼' }}
            </span>
          </div>
        </div>
      </div>
      
      <!-- Body -->
      <div class="details-body" role="rowgroup" (mousedown)="onBodyMouseDown($event)" (click)="onBackgroundClick($event)" (contextmenu)="onBackgroundContextMenu($event)">
        <div 
          *ngFor="let entry of entries"
          class="details-row"
          [attr.data-path]="entry.path"
          [class.selected]="isSelected(entry)"
          [class.parent-dir]="isParentEntry(entry)"
          [class.drop-target]="isDropTarget(entry)"
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
          
          <div 
            *ngFor="let col of columns"
            class="details-cell"
            [style.width]="col.width"
            [style.flex]="col.width ? 'none' : '1'"
            [attr.role]="'cell'">
            
            <ng-container *ngIf="col.field === 'name'">
              <span class="file-icon">{{ getIcon(entry) }}</span>
              <span class="file-name-text">{{ entry.name }}</span>
            </ng-container>
            
            <ng-container *ngIf="col.field !== 'name'">
              {{ getCellValue(entry, col) }}
            </ng-container>
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
      }
    }

    .details-cell {
      display: flex;
      align-items: center;
      padding: 0 var(--vsc-padding-sm);
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
      font-size: var(--vsc-font-size);
      
      &.header-cell {
        &.sortable {
          cursor: pointer;
          
          &:hover {
            background: var(--vsc-list-hover-background);
          }
        }
      }
      
      .header-label {
        overflow: hidden;
        text-overflow: ellipsis;
      }
      
      .sort-indicator {
        margin-left: var(--vsc-padding-xs);
        font-size: 10px;
      }
    }

    .details-body {
      flex: 1;
      position: relative;
      overflow-y: auto;
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

    .file-name-text {
      overflow: hidden;
      text-overflow: ellipsis;
    }

    /* Column specific widths */
    .col-name { flex: 2; }
    .col-size { width: 100px; text-align: right; justify-content: flex-end; }
    .col-date { width: 150px; }
    .col-type { width: 100px; }
  `]
})
export class FileDetailsComponent implements OnDestroy {
  constructor(private zone: NgZone) {}

  @Input() entries: FileInfo[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  @Input() columns: ColumnDef[] = [
    { field: 'name', header: 'Name', flex: 2, sortable: true },
    { field: 'size', header: 'Size', width: '100px', sortable: true, formatter: (v) => this.formatSize(v) },
    { field: 'modified', header: 'Modified', width: '150px', sortable: true, formatter: (v) => this.formatDate(v) },
    { field: 'type', header: 'Type', width: '100px', formatter: (_, entry) => this.getFileTypeLabel(entry) }
  ];
  @Input() sortField: string = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';
  
  @Output() selectionChange = new EventEmitter<FileInfo[]>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() openInNewTab = new EventEmitter<FileInfo>();
  @Output() sort = new EventEmitter<{ field: string; direction: 'asc' | 'desc' }>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() dragStart = new EventEmitter<{ event: DragEvent; entries: FileInfo[] }>();
  @Output() dragDrop = new EventEmitter<{ event: DragEvent; source: FileInfo[]; target: FileInfo }>();

  private anchorPath: string | null = null;
  private dropTargetPath: string | null = null;
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

  ngOnDestroy(): void {
    this.detachMarqueeListeners();
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

  onRowClick(entry: FileInfo, event: MouseEvent): void {
    if (this.isParentEntry(entry)) {
      this.navigate.emit(entry);
      return;
    }

    if (event.ctrlKey || event.metaKey) {
      this.selectionChange.emit(this.buildToggledSelection(entry));
      this.anchorPath = entry.path;
    } else if (event.shiftKey && this.anchorPath) {
      // Range selection
      this.rangeSelect(entry);
    } else {
      this.anchorPath = entry.path;
      this.selectionChange.emit([entry]);
    }
  }

  onRowDoubleClick(entry: FileInfo): void {
    if (!this.isParentEntry(entry)) {
      this.navigate.emit(entry);
    }
  }

  onContextMenu(event: MouseEvent, entry: FileInfo): void {
    if (this.suppressItemContextMenu) {
      event.preventDefault();
      this.suppressItemContextMenu = false;
      return;
    }

    event.preventDefault();
    if (!this.isParentEntry(entry) && !this.isSelected(entry)) {
      const selected = new Set(this.selectedFiles);
      selected.add(entry.path);
      this.selectionChange.emit(this.entries.filter(e => selected.has(e.path) && !this.isParentEntry(e)));
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

  onBodyMouseDown(event: MouseEvent): void {
    if (event.button !== 0 && event.button !== 2) return;
    if (event.button === 0 && event.target !== event.currentTarget) return;

    const container = event.currentTarget as HTMLElement;
    this.marqueeContainer = container;
    this.marqueeAppend = event.ctrlKey || event.metaKey;
    this.marqueeButton = event.button as 0 | 2;
    this.hasMarqueeMoved = false;
    this.pendingClearOnMove = this.marqueeButton === 2 && !this.marqueeAppend;

    const { x, y } = this.getRelativePoint(event, container);
    this.marqueeStartX = x;
    this.marqueeStartY = y;
    this.selectionRect = { active: true, left: x, top: y, width: 0, height: 0 };

    if (!this.marqueeAppend && !this.pendingClearOnMove) {
      this.selectionChange.emit([]);
    }

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
    if (entry.type === FileType.DIRECTORY) return 'Folder';
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

    if (startIdx === -1 || endIdx === -1) {
      return;
    }

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
}