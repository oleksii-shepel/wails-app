import { Component, Input, Output, EventEmitter, OnDestroy, NgZone } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileEntryComponent, FileEntrySelectEvent } from './file-entry.component';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { parentPath } from './address-bar.utils';

/**
 * File List Component
 * Displays a list of files and directories
 */
@Component({
  selector: 'app-file-list',
  standalone: true,
  imports: [CommonModule, FileEntryComponent],
  template: `
    <div class="pane-content" (mousedown)="onListMouseDown($event)" (click)="onBackgroundClick($event)" (contextmenu)="onBackgroundContextMenu($event)">
      <div *ngIf="loading" class="loading">
        <div class="spinner"></div>
        <p>Loading...</p>
      </div>
      
      <div *ngIf="error" class="error">
        {{ error }}
      </div>
      
      <div *ngIf="!loading && !error" class="file-list">
        <div
          *ngFor="let entry of entries"
          class="file-list-row"
          [attr.data-path]="entry.path"
          [class.drop-target]="isDropTarget(entry)"
          [draggable]="!isParentEntry(entry)"
          (auxclick)="onItemAuxClick($event, entry)"
          (dragstart)="onDragStart($event, entry)"
          (dragover)="onDragOver($event, entry)"
          (dragleave)="onDragLeave(entry)"
          (drop)="onDrop($event, entry)"
          (contextmenu)="onContextMenu($event, entry)">
          <app-file-entry
            [entry]="entry"
            [selected]="isSelected(entry)"
            (select)="onSelect($event)"
            (navigate)="onNavigate($event)">
          </app-file-entry>
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
    }

    .pane-content {
      position: relative;
      flex: 1;
      overflow-y: auto;
      padding: var(--vsc-padding-sm);
      background: var(--vsc-editor-background);
      min-height: 0;

      &::-webkit-scrollbar {
        width: 10px;
      }

      &::-webkit-scrollbar-track {
        background: var(--vsc-scrollbar-track);
      }

      &::-webkit-scrollbar-thumb {
        background: var(--vsc-scrollbar-thumb);
        border-radius: 5px;

        &:hover {
          background: var(--vsc-scrollbar-thumb-hover);
        }
      }
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

    .file-list {
      display: flex;
      flex-direction: column;
      gap: 1px;
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

      &.drop-target {
        outline: 1px dashed var(--vsc-focus-border);
        outline-offset: -1px;
        background: var(--vsc-list-hover-background);
      }
    }
  `]
})
export class FileListComponent implements OnDestroy {
  constructor(private zone: NgZone) {}

  @Input() entries: FileInfo[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  @Input() loading = false;
  @Input() error: string | null = null;
  
  @Output() selectionChange = new EventEmitter<FileInfo[]>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() openInNewTab = new EventEmitter<FileInfo>();
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

  ngOnDestroy(): void {
    this.detachMarqueeListeners();
  }

  isParentEntry(entry: FileInfo): boolean {
    return entry.path === PARENT_ENTRY_PATH;
  }

  isDropTarget(entry: FileInfo): boolean {
    return this.dropTargetPath === entry.path;
  }

  isSelected(entry: FileInfo): boolean {
    return this.selectedFiles.has(entry.path);
  }

  onSelect(event: FileEntrySelectEvent): void {
    const entry = event.entry;
    if (this.isParentEntry(entry)) {
      return; // navigate only on double-click
    }

    const mouseEvent = event.mouseEvent;
    const isToggle = mouseEvent.ctrlKey || mouseEvent.metaKey;
    const isRange = mouseEvent.shiftKey && this.anchorPath !== null;

    if (isRange) {
      this.selectionChange.emit(this.buildRangeSelection(entry));
      return;
    }

    if (isToggle) {
      this.selectionChange.emit(this.buildToggledSelection(entry));
      this.anchorPath = entry.path;
      return;
    }

    this.anchorPath = entry.path;
    this.selectionChange.emit([entry]);
  }

  onNavigate(entry: FileInfo): void {
    this.navigate.emit(entry);
  }

  onItemDoubleClick(entry: FileInfo): void {
    this.navigate.emit(entry);
  }

  onBackgroundClick(event: MouseEvent): void {
    if (this.suppressBackgroundClick) {
      this.suppressBackgroundClick = false;
      return;
    }

    const target = event.target as HTMLElement;
    if (event.target === event.currentTarget || target.classList.contains('file-list')) {
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

    const target = event.target as HTMLElement;
    if (event.target === event.currentTarget || target.classList.contains('file-list')) {
      event.preventDefault();
      this.selectionChange.emit([]);
      this.anchorPath = null;
    }
  }

  onContextMenu(event: MouseEvent, entry: FileInfo): void {
    if (this.suppressItemContextMenu) {
      event.preventDefault();
      this.suppressItemContextMenu = false;
      return;
    }

    event.preventDefault();
    if (this.isParentEntry(entry)) {
      return;
    }

    if (this.isSelected(entry)) {
      const nextSelection = this.entries.filter(e => this.selectedFiles.has(e.path) && e.path !== entry.path && !this.isParentEntry(e));
      this.selectionChange.emit(nextSelection);
    } else {
      const selected = new Set(this.selectedFiles);
      selected.add(entry.path);
      this.selectionChange.emit(this.entries.filter(e => selected.has(e.path) && !this.isParentEntry(e)));
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

    const container = event.currentTarget as HTMLElement;
    if (!container) return;

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
    this.selectionRect = { active: true, left: x, top: y, width: 0, height: 0 };

    if (!this.marqueeAppend && !this.pendingClearOnMove) {
      this.selectionChange.emit([]);
    }

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

    const parsed = this.readDraggedPaths(event);
    if (!parsed.length) return;

    const source = this.entries.filter(e => parsed.includes(e.path) && !this.isParentEntry(e));
    if (!source.length) return;

    const target = entry.type === FileType.DIRECTORY
      ? entry
      : ({ ...entry, path: parentPath(entry.path), type: FileType.DIRECTORY } as FileInfo);

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
