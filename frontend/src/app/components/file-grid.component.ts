// file-grid.component.ts
import { Component, Input, Output, EventEmitter, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { parentPath } from './address-bar.utils';

@Component({
  selector: 'app-file-grid',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="file-grid" role="grid" (mousedown)="onGridMouseDown($event)" (click)="onBackgroundClick($event)" (contextmenu)="onBackgroundContextMenu($event)">
      <div 
        *ngFor="let entry of entries"
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
        
        <div class="grid-icon">
          <span class="file-emoji">{{ getIcon(entry) }}</span>
        </div>
        
        <div class="grid-info">
          <div class="grid-name" [title]="entry.name">{{ entry.name }}</div>
          <div class="grid-meta" *ngIf="!isParentEntry(entry)">
            {{ formatSize(entry.size) }}
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
    .file-grid {
      position: relative;
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
      gap: var(--vsc-padding-md);
      padding: var(--vsc-padding-md);
      overflow-y: auto;
      background: var(--vsc-editor-background);
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

    /* Compact mode adjustments */
    :host-context(.compact-mode) {
      .file-grid {
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
export class FileGridComponent implements OnDestroy {
  @Input() entries: FileInfo[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  
  @Output() selectionChange = new EventEmitter<FileInfo | FileInfo[]>;
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() openInNewTab = new EventEmitter<FileInfo>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() dragStart = new EventEmitter<{ event: DragEvent; entries: FileInfo[] }>();
  @Output() dragDrop = new EventEmitter<{ event: DragEvent; source: FileInfo[]; target: FileInfo }>();

  FileType = FileType;
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

  onItemClick(entry: FileInfo, event: MouseEvent): void {
    // Handle multi-select with Ctrl/Cmd
    if (event.ctrlKey || event.metaKey) {
      this.selectionChange.emit(this.buildToggledSelection(entry));
      this.anchorPath = entry.path;
    } else if (event.shiftKey && this.anchorPath) {
      this.selectionChange.emit(this.rangeSelect(entry));
    } else {
      // Single select
      if (this.isParentEntry(entry)) {
        this.navigate.emit(entry);
      } else {
        this.anchorPath = entry.path;
        this.selectionChange.emit(entry);
      }
    }
  }

  onItemDoubleClick(entry: FileInfo): void {
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
    if (!this.isSelected(entry)) {
      this.selectionChange.emit(entry);
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

    const target = event.target as HTMLElement;
    if (target.closest('.grid-item')) return;

    const container = event.currentTarget as HTMLElement;
    this.marqueeContainer = container;
    this.marqueeAppend = event.ctrlKey || event.metaKey;
    this.selectionMode = event.button === 0 ? 'marquee' : 'paint';
    this.paintedPaths.clear();

    const { x, y } = this.getRelativePoint(event, container);
    this.marqueeStartX = x;
    this.marqueeStartY = y;

    this.selectionRect = this.selectionMode === 'marquee'
      ? {
          active: true,
          left: x,
          top: y,
          width: 0,
          height: 0,
        }
      : {
          active: false,
          left: 0,
          top: 0,
          width: 0,
          height: 0,
        };

    if (!this.marqueeAppend) {
      this.selectionChange.emit([]);
    }

    if (this.selectionMode === 'paint') {
      this.updatePaintSelection(event);
    }

    this.suppressBackgroundClick = true;
    if (this.selectionMode === 'paint') {
      this.suppressItemContextMenu = true;
    }

    this.windowMouseMove = (moveEvent: MouseEvent) => this.onGridMouseMove(moveEvent);
    this.windowMouseUp = () => this.finishMarqueeSelection();
    this.windowContextMenu = (contextEvent: MouseEvent) => {
      if (this.selectionMode !== 'none') {
        contextEvent.preventDefault();
        contextEvent.stopPropagation();
      }
    };
    window.addEventListener('mousemove', this.windowMouseMove);
    window.addEventListener('mouseup', this.windowMouseUp);
    window.addEventListener('contextmenu', this.windowContextMenu, true);
    this.suppressBackgroundContextMenu = this.selectionMode === 'paint';
    event.preventDefault();
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

    this.selectionRect = { active: true, left, top, width, height };
    this.emitMarqueeSelection();
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
    this.paintedPaths.clear();
    this.selectionRect = {
      active: false,
      left: 0,
      top: 0,
      width: 0,
      height: 0,
    };
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
    // Range selection logic for grid view
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