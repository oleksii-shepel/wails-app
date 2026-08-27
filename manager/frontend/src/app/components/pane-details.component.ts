// pane-details.component.ts
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
import {
} from '@reassembly/core/ui/panes';
import { FileEntryInlineComponent } from './file-entry-inline.component';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { parentPath } from './address-bar.utils';
import { isRegistryPath } from '../services/registry.service';
import { VirtualCollection } from '../services/virtual-window';
import { PaneDetailsResizeHandleDirective, type PaneDetailsColumnWidthChange } from './pane-details-resize-handle.directive';

export interface PaneDetailsColumn {
  field: keyof FileInfo | string;
  header: string;
  width?: string;
  flex?: number;
  sortable?: boolean;
  formatter?: (value: any, entry: FileInfo) => string;
  minWidth?: number;
  maxWidth?: number;
}

export interface PaneDetailsColumnState {
  readonly key: string;
  readonly visible?: boolean;
  readonly order?: number;
  readonly width?: number;
}

export interface PaneDetailsColumnConfiguration {
  readonly columns: readonly PaneDetailsColumnState[];
}

interface DetailsColumnVm {
  column: PaneDetailsColumn;
  key: string;
  widthVar: string | null;
  flex: string;
  minWidth: number;
  maxWidth: number;
}

interface DetailsCellVm {
  column: PaneDetailsColumn;
  key: string;
  widthVar: string | null;
  flex: string;
  value: string;
  isName: boolean;
  showDirectorySizePending: boolean;
}

interface DetailsRowVm {
  entry: FileInfo;
  selected: boolean;
  cursor: boolean;
  parentDir: boolean;
  renaming: boolean;
  editingName: string;
  directorySizePending: boolean;
  dragOverInside: boolean;
  dragOverBefore: boolean;
  dragOverAfter: boolean;
  draggable: boolean;
  displayNameOverride: string | null;
  cells: DetailsCellVm[];
}

@Component({
  selector: 'app-pane-details',
  standalone: true,
  imports: [
    CommonModule,
    FileEntryInlineComponent,
    PaneDetailsResizeHandleDirective,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div #detailsRoot class="file-details" role="table" data-pane-details-root>
      <div class="details-header" role="rowgroup">
        <div class="details-row header-row" role="row">
          <div 
            *ngFor="let col of activeColumnVms; let i = index; trackBy: trackByColumnKey"
            class="details-cell header-cell"
            [attr.data-col]="col.key"
            [class.sortable]="col.column.sortable"
            [style.width]="col.widthVar"
            [style.flex]="col.flex"
            [attr.role]="'columnheader'"
            [attr.aria-sort]="getSortState(col.column.field)"
            (click)="col.column.sortable && onSort(col.column.field)">
            
            <span class="header-label">{{ col.column.header }}</span>
            <span class="sort-indicator" *ngIf="sortField === col.column.field"
                  [class.sort-asc]="sortDirection === 'asc'"
                  [class.sort-desc]="sortDirection === 'desc'">
              <svg viewBox="0 0 16 16">
                <path d="M6 4l4 4-4 4" fill="none" stroke="currentColor" stroke-width="2"/>
              </svg>
            </span>
            <div
              *ngIf="i < activeColumnVms.length - 1"
              class="view-divider"
              title="Resize column"
              [paneDetailsResizeHandle]="col.key"
              [paneDetailsMinWidth]="col.minWidth"
              [paneDetailsMaxWidth]="col.maxWidth"
              (paneDetailsWidthCommit)="onColumnWidthCommit($event)">
            </div>
          </div>
        </div>
      </div>
      
      <div
        #detailsBody
        class="details-body"
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
            *ngFor="let row of visibleRowVms; trackBy: trackByRowPath"
            class="details-row"
            [attr.data-path]="row.entry.path"
            [style.height.px]="virtualEnabled ? rowHeight : null"
            [class.selected]="row.selected"
            [class.cursor]="row.cursor"
            [class.parent-dir]="row.parentDir"
            [class.drag-over]="row.dragOverInside"
            [class.drag-over-before]="row.dragOverBefore"
            [class.drag-over-after]="row.dragOverAfter"
            [attr.role]="'row'"
            [attr.aria-selected]="row.selected"
            [draggable]="row.draggable"
            (click)="onRowClick(row.entry, $event)"
            (auxclick)="onRowAuxClick($event, row.entry)"
            (dblclick)="onRowDoubleClick(row.entry)"
            (contextmenu)="onContextMenu($event, row.entry)"
            (dragstart)="onDragStart($event, row.entry)"
            (dragover)="onDragOver($event, row.entry)"
            (dragleave)="onDragLeave(row.entry)"
            (drop)="onDrop($event, row.entry)">
            <div *ngIf="row.dragOverBefore" class="drop-indicator drop-indicator-before"></div>
            <div *ngIf="row.dragOverAfter" class="drop-indicator drop-indicator-after"></div>
            
            <div 
              *ngFor="let cell of row.cells; trackBy: trackByCellKey"
              class="details-cell"
              [style.width]="cell.widthVar"
              [style.flex]="cell.flex"
              [attr.role]="'cell'">
              
              <ng-container *ngIf="cell.isName">
                <input
                  *ngIf="row.renaming; else detailsNameDisplay"
                  class="details-rename-input"
                  [value]="row.editingName"
                  (click)="$event.stopPropagation()"
                  (dblclick)="$event.stopPropagation()"
                  (mousedown)="$event.stopPropagation()"
                  (input)="onRenameInput(row.entry, $event)"
                  (blur)="onRenameBlur(row.entry)"
                  (keydown)="onRenameKeyDown(row.entry, $event)" />

                <ng-template #detailsNameDisplay>
                  <app-file-entry-inline
                    [entry]="row.entry"
                    [displayNameOverride]="row.displayNameOverride"
                    [colorizeByMime]="colorizeByMime">
                  </app-file-entry-inline>
                </ng-template>
              </ng-container>
              
              <ng-container *ngIf="cell.showDirectorySizePending; else defaultCellContent">
                <span
                  class="details-size-spinner"
                  aria-label="Calculating folder size"
                  title="Calculating folder size">
                </span>
              </ng-container>

              <ng-template #defaultCellContent>
              <ng-container *ngIf="!cell.isName">
                {{ cell.value }}
              </ng-container>
              </ng-template>
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
    .details-cell,
    .details-cell .file-name,
    .details-cell .file-name-text {
      min-width: 0;
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .details-cell > app-file-entry-inline {
      display: block;
      min-width: 0;
      width: 100%;
      max-width: 100%;
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

    .file-details {
      position: relative;
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      width: 100%;
      max-width: 100%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      contain: layout paint;
      isolation: isolate;
      background: var(--vsc-editor-background);
    }

    .details-header {
      flex-shrink: 0;
      padding: 0 var(--vsc-padding-sm);
      background: var(--vsc-tabbar-background);
      border-bottom: 1px solid var(--vsc-border);
    }

    :host-context(.pane:not(.active-pane)) .details-row.selected {
      background: var(
        --vsc-list-inactive-selection,
        color-mix(in srgb, var(--vsc-list-active-background) 48%, transparent)
      );
      color: var(--vsc-foreground);
    }

    :host-context(.pane:not(.active-pane)) .details-row.cursor {
      border-left-color: color-mix(
        in srgb,
        var(--vsc-foreground-dim, var(--vsc-border-bright)) 58%,
        transparent
      );
    }

    :host-context(.pane.active-pane) .details-row.cursor {
      border-left-color: var(--vsc-accent-blue, var(--vsc-focus-border));
    }

    .details-row {
      display: flex;
      align-items: center;
      min-height: 28px;
      height: 28px;
      padding: 0 var(--vsc-padding-md);
      border: 0;
      border-left: 3px solid transparent;
      border-radius: var(--vsc-border-radius-sm);
      flex-shrink: 0;
      contain: layout style;
      position: relative;
      transition:
        background-color var(--vsc-transition-fast),
        border-color var(--vsc-transition-fast),
        color var(--vsc-transition-fast);

      &.header-row {
        min-height: 24px;
        height: 24px;
        border-left-color: transparent;
        border-radius: 0;
        font-weight: 600;
        color: var(--vsc-foreground-dim);
      }

      &:not(.header-row) {
        cursor: pointer;
        user-select: none;

        &:hover {
          background: var(--vsc-list-hover-background);
          color: var(--vsc-foreground);
          border-left-color: var(--vsc-border-bright);
        }

        &.selected {
          background: var(--vsc-list-active-background);
          color: var(--vsc-list-active-foreground);
          border-left-color: var(--vsc-accent-blue);
        }

        &.cursor:not(.selected) {
          background: color-mix(in srgb, var(--vsc-list-active-background) 35%, transparent);
          border-left-color: var(--vsc-accent-blue);
          box-shadow: none;
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
          background-color: var(--vsc-list-drop-background, var(--vsc-accent-selection));
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
      background-color: var(--vsc-focus-border, var(--vsc-accent-blue));
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
      line-height: 1.4;
      
      &.header-cell {
        position: relative;
        font-size: 10px;
        line-height: 1.35;
        font-weight: 600;
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
        display: inline-flex;
        align-items: center;
        min-height: 14px;
        line-height: 1.35;
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
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      position: relative;
      overflow-y: auto;
      overflow-x: hidden;
      padding: var(--vsc-padding-sm);
      background: var(--vsc-editor-background);
      contain: layout paint;
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

    .details-rename-input {
      width: 100%;
      min-width: 0;
      box-sizing: border-box;
      font: inherit;
      line-height: inherit;
      color: var(--vsc-input-foreground);
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-input-border);
      border-radius: var(--vsc-border-radius-sm);
      padding: 0 6px;
      outline: none;
    }

    .details-rename-input:focus {
      border-color: var(--vsc-input-focus-border);
      box-shadow: 0 0 0 1px var(--vsc-input-focus-border);
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

    .col-name { flex: 2; }
    .col-size { width: 100px; text-align: right; justify-content: flex-end; }
    .col-date { width: 150px; }
    .col-type { width: 100px; }

    .details-size-spinner {
      width: 12px;
      height: 12px;
      border-radius: 999px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue) 22%, transparent);
      border-top-color: var(--vsc-accent-blue);
      box-sizing: border-box;
      animation: details-size-spin 0.9s linear infinite;
      flex-shrink: 0;
    }

    .details-body.drop-target-bg {
      outline: 2px dashed var(--vsc-focus-border);
      outline-offset: -2px;
      background: color-mix(in srgb, var(--vsc-list-hover-background) 30%, transparent);
    }

    @keyframes details-size-spin {
      from { transform: rotate(0deg); }
      to { transform: rotate(360deg); }
    }
  `]
})
export class PaneDetailsComponent implements OnDestroy, OnChanges, AfterViewInit {
  FileType = FileType;
  private readonly defaultColumns: PaneDetailsColumn[] = [
    { field: 'name', header: 'Name', flex: 2, sortable: true, minWidth: 140 },
    { field: 'size', header: 'Size', width: '100px', sortable: true, minWidth: 72, maxWidth: 220, formatter: (value) => this.formatSize(value) },
    { field: 'modified', header: 'Modified', width: '150px', sortable: true, minWidth: 110, maxWidth: 320, formatter: (value) => this.formatDate(value) },
    { field: 'permissions', header: 'Permissions', width: '110px', sortable: true, minWidth: 90, maxWidth: 180, formatter: (value) => value?.toString() || '' },
    { field: 'type', header: 'Type', width: '100px', sortable: true, minWidth: 80, maxWidth: 260, formatter: (_, entry) => this.getFileTypeLabel(entry) }
  ];
  private readonly registryColumns: PaneDetailsColumn[] = [
    { field: 'name', header: 'Name', flex: 1.7, sortable: true },
    { field: 'registryKind', header: 'Kind', width: '110px', sortable: true, formatter: (_, entry) => this.getRegistryKind(entry) },
    { field: 'registryValueType', header: 'Data Type', width: '150px', sortable: true, formatter: (value) => value?.toString() || '' },
    { field: 'registryPreview', header: 'Data', flex: 2.3, sortable: true, formatter: (value) => value?.toString() || '' }
  ];
  private readonly searchColumns: PaneDetailsColumn[] = [
    { field: 'name', header: 'Name', flex: 1.5, sortable: true },
    { field: 'searchParentPath', header: 'Folder', flex: 1.8, sortable: true, formatter: (_, entry) => this.getSearchParentPath(entry) },
    { field: 'modified', header: 'Modified', width: '150px', sortable: true, minWidth: 110, maxWidth: 320, formatter: (value) => this.formatDate(value) },
    { field: 'size', header: 'Size', width: '100px', sortable: true, minWidth: 72, maxWidth: 220, formatter: (value) => this.formatSize(value) },
    { field: 'type', header: 'Type', width: '100px', sortable: true, minWidth: 80, maxWidth: 260, formatter: (_, entry) => this.getFileTypeLabel(entry) }
  ];
  
  constructor(
    private zone: NgZone,
    private cdr: ChangeDetectorRef
  ) {}

  @ViewChild('detailsRoot') detailsRootRef?: ElementRef<HTMLElement>;
  @ViewChild('detailsBody') detailsBodyRef?: ElementRef<HTMLElement>;
  @Input() entries: FileInfo[] = [];
  @Input() pendingDirectorySizePaths: ReadonlySet<string> = new Set();
  @Input() currentPath = '';
  @Input() selectedFiles: Set<string> = new Set();
  @Input() cursorPath: string | null = null;
  @Input() columns: PaneDetailsColumn[] | null = null;
  @Input() columnConfiguration: PaneDetailsColumnConfiguration | null = null;
  @Input() sortField: string = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';
  @Input() paneActive = true;
  @Input() singleClickToOpen = false;
  @Input() colorizeByMime = false;
  
  @Output() selectionChange = new EventEmitter<FileInfo[]>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() openInNewTab = new EventEmitter<FileInfo>();
  @Output() sort = new EventEmitter<{ field: string; direction: 'asc' | 'desc' }>();
  @Output() columnConfigurationChange = new EventEmitter<PaneDetailsColumnConfiguration>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() dragStart = new EventEmitter<{ event: DragEvent; entries: FileInfo[] }>();
  @Output() dragDrop = new EventEmitter<{ event: DragEvent; source: FileInfo[]; target: FileInfo }>();
  @Output() rename = new EventEmitter<{ entry: FileInfo; newName: string }>();
  @Output() viewportChange = new EventEmitter<void>();

  private readonly VIRTUAL_THRESHOLD = 100;
  private readonly DEFAULT_ROW_HEIGHT = 24;
  rowHeight = 24;
  private readonly overscan = 64;
  private readonly windowGuardRows = 24;
  private scrollRAF: number | null = null;
  private boundScrollListener?: () => void;
  private resizeObserver?: ResizeObserver;

  private virtualStart = 0;
  private virtualEnd = 0;
  private viewportHeight = 0;
  private readonly virtualCollection = new VirtualCollection<FileInfo, string>({
    identity: entry => entry.path,
    itemExtent: this.DEFAULT_ROW_HEIGHT,
    threshold: this.VIRTUAL_THRESHOLD,
    overscanRows: this.overscan,
    guardRows: this.windowGuardRows,
    minimumPrefetchItems: 1024,
    prefetchMultiplier: 4,
    prefetchGuardRatio: 0.20,
  });
  private visibleEntriesCache: FileInfo[] = [];
  private pathToEntryIndex = new Map<string, number>();
  activeColumnVms: DetailsColumnVm[] = [];
  visibleRowVms: DetailsRowVm[] = [];
  private resolvedActiveColumns: PaneDetailsColumn[] = [];
  private rowVmCache = new Map<string, DetailsRowVm>();
  private readonly formattedValueCache = new WeakMap<FileInfo, Map<string, { signature: string; value: string }>>();
  private readonly columnWidths = new Map<string, number>();

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
  private pendingRenameDisplayNames = new Map<string, string>();
  private renamingPath: string | null = null;
  private editingName = '';

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
    this.rebuildVisibleRowVms();
    this.cdr.markForCheck();
  }

  getDisplayNameOverride(path: string): string | null {
    return this.pendingRenameDisplayNames.get(path) ?? null;
  }

  /** Scroll so the given path is visible. */
  scrollToPath(path: string): void {
    if (!path || path === PARENT_ENTRY_PATH) return;
    const host = this.detailsBodyRef?.nativeElement;
    if (!host) return;
    if (this.virtualEnabled) {
      const idx = this.pathToEntryIndex.get(path) ?? -1;
      if (idx >= 0) {
        const itemTop = idx * this.rowHeight;
        const itemBottom = itemTop + this.rowHeight;
        if (itemTop < host.scrollTop || itemBottom > host.scrollTop + host.clientHeight) {
          const centerOffset = host.clientHeight / 2 - this.rowHeight / 2;
          host.scrollTop = Math.max(0, itemTop - centerOffset);
        }
        if (this.updateVirtualWindow()) {
          this.cdr.markForCheck();
        }
      }
    } else {
      const el = host.querySelector<HTMLElement>(`.details-row[data-path="${CSS.escape(path)}"]`);
      el?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    }
  }

  getScrollTop(): number {
    return this.detailsBodyRef?.nativeElement?.scrollTop ?? 0;
  }


  getScrollMetrics(): { scrollTop: number; scrollHeight: number; clientHeight: number } {
    const element = this.detailsBodyRef?.nativeElement;
    return {
      scrollTop: element?.scrollTop ?? 0,
      scrollHeight: element?.scrollHeight ?? 0,
      clientHeight: element?.clientHeight ?? 0,
    };
  }

  setScrollTop(top: number): void {
    const element = this.detailsBodyRef?.nativeElement;
    if (!element) return;
    element.scrollTop = Math.max(0, top);
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
        const idx = this.pathToEntryIndex.get(path) ?? -1;
        if (idx >= 0) {
          const targetTop = idx * this.rowHeight;
          const centerOffset = host.clientHeight / 2 - this.rowHeight / 2;
          host.scrollTop = Math.max(0, targetTop - centerOffset);
          if (this.updateVirtualWindow()) {
            this.cdr.markForCheck();
          }
        }
      } else {
        const el = host.querySelector<HTMLElement>(`.details-row[data-path="${CSS.escape(path)}"]`);
        el?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
      }
    }

    requestAnimationFrame(() => {
      const target = this.entries.find((entry) => entry.path === path);
      if (!target || this.isParentEntry(target)) return;
      this.renamingPath = path;
      this.editingName = target.name;
      this.rebuildVisibleRowVms();
      this.cdr.markForCheck();
    });
  }

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
    return this.visibleEntriesCache;
  }

  get totalContentHeight(): number {
    return this.virtualCollection.window.totalExtentPx ||
      this.entries.length * this.rowHeight;
  }

  get virtualWindowTransform(): string {
    if (!this.virtualEnabled) return '';
    return `translateY(${this.virtualStart * this.rowHeight}px)`;
  }

  get activeColumns(): PaneDetailsColumn[] {
    return this.resolvedActiveColumns;
  }

  getColumnKey(col: PaneDetailsColumn): string {
    return String(col.field);
  }

  getColumnWidthVar(col: PaneDetailsColumn): string | null {
    const key = this.getColumnKey(col);
    if (this.columnWidths.has(key) || !!col.width) {
      return `var(--details-column-${this.cssKey(key)}-width)`;
    }
    return null;
  }

  getColumnFlex(col: PaneDetailsColumn): string {
    const key = this.getColumnKey(col);
    if (this.columnWidths.has(key) || !!col.width) {
      return 'none';
    }
    return col.flex ? `${col.flex} 1 0%` : '1 1 0%';
  }

  ngAfterViewInit(): void {
    const element = this.detailsBodyRef?.nativeElement;
    this.viewportHeight = element?.clientHeight || 0;
    this.refreshColumnState();
    this.rebuildColumnVms();
    this.setupResizeObserver();
    this.setupScrollListener();
    this.rebuildDerivedEntries();
    this.updateVirtualWindow(true);
    requestAnimationFrame(() => this.measureRowHeight());
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (
      changes['entries'] ||
      changes['columns'] ||
      changes['columnConfiguration'] ||
      changes['currentPath']
    ) {
      const columnsChanged =
        !!changes['columns'] ||
        !!changes['columnConfiguration'] ||
        !!changes['currentPath'];
      if (columnsChanged) this.refreshColumnState();
      const entriesChange = changes['entries'];
      if (entriesChange) {
        const prevLen = entriesChange.previousValue?.length || 0;
        const currLen = entriesChange.currentValue?.length || 0;
        if (Math.abs(prevLen - currLen) > 1000) {
          this.resetScroll();
        }
      }
      this.rebuildDerivedEntries();
      this.updateVirtualWindow(true);
      queueMicrotask(() => requestAnimationFrame(() => this.measureRowHeight()));
      this.cdr.markForCheck();
    } else if (changes['pendingDirectorySizePaths'] || changes['selectedFiles'] || changes['cursorPath']) {
      this.rebuildVisibleRowVms();
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

  onColumnWidthCommit(change: PaneDetailsColumnWidthChange): void {
    this.columnWidths.set(change.key, Math.max(1, Math.round(change.width)));
    this.columnConfigurationChange.emit({
      columns: this.serializeColumnConfiguration(),
    });
  }

  private setupResizeObserver(): void {
    const element = this.detailsBodyRef?.nativeElement;
    if (!element) return;

    this.zone.runOutsideAngular(() => {
      this.resizeObserver = new ResizeObserver((entries) => {
        let shouldMark = false;
        for (const entry of entries) {
          if (this.viewportHeight !== entry.contentRect.height) {
            this.viewportHeight = entry.contentRect.height;
            shouldMark = this.updateVirtualWindow() || shouldMark;
          }
        }
        if (shouldMark) {
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
    const element = this.detailsBodyRef?.nativeElement;
    if (!element) return;
    this.boundScrollListener = () => this.onScroll();
    this.zone.runOutsideAngular(() => {
      element.addEventListener('scroll', this.boundScrollListener!, { passive: true });
    });
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
      if (this.updateVirtualWindow()) {
        this.cdr.markForCheck();
      }
      this.viewportChange.emit();
    });
  }

  private resetScroll(): void {
    const element = this.detailsBodyRef?.nativeElement;
    if (element) {
      element.scrollTop = 0;
    }
    this.virtualStart = 0;
    this.virtualEnd = Math.min(this.overscan * 2, this.entries.length);
    this.rebuildVisibleEntries();
  }

  private updateVirtualWindow(forceRebuild = false): boolean {
    const element = this.detailsBodyRef?.nativeElement;
    const viewportHeight = Math.max(
      this.rowHeight,
      this.viewportHeight || element?.clientHeight || this.rowHeight,
    );
    const scrollTop = element?.scrollTop ?? 0;

    this.virtualCollection.configure({
      itemExtent: this.rowHeight,
      threshold: this.VIRTUAL_THRESHOLD,
      overscanRows: this.overscan,
      guardRows: this.windowGuardRows,
      columns: 1,
    });
    this.virtualCollection.setLogicalCount(this.entries.length);
    this.virtualCollection.populate(0, this.entries, this.entries.length);

    const changed = this.virtualCollection.updateViewport({
      scrollTop,
      viewportHeight,
      columns: 1,
    }, forceRebuild);

    const window = this.virtualCollection.window;
    this.virtualStart = window.start;
    this.virtualEnd = window.end;

    if (changed || forceRebuild) {
      this.rebuildVisibleEntries();
    }
    return changed || forceRebuild;
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
    this.updateVirtualWindow(true);
    this.cdr.markForCheck();
  }

  private rebuildDerivedEntries(): void {
    this.pathToEntryIndex.clear();
    const pathSet = new Set<string>();
    this.entries.forEach((entry, index) => {
      this.pathToEntryIndex.set(entry.path, index);
      pathSet.add(entry.path);
    });
    for (const key of this.rowVmCache.keys()) {
      if (!pathSet.has(key)) {
        this.rowVmCache.delete(key);
      }
    }
    this.rebuildVisibleEntries();
  }

  private rebuildVisibleEntries(): void {
    this.visibleEntriesCache = this.virtualEnabled
      ? this.entries.slice(this.virtualStart, this.virtualEnd)
      : this.entries;
    this.rebuildVisibleRowVms();
  }

  private refreshColumnState(): void {
    this.resolvedActiveColumns = this.resolveActiveColumns();
    this.rebuildColumnVms();
  }

  private resolveActiveColumns(): PaneDetailsColumn[] {
    const source = this.columns?.length
      ? this.columns
      : this.computeIsRegistryListing()
        ? this.registryColumns
        : this.computeIsSearchListing()
          ? this.searchColumns
          : this.defaultColumns;

    const stateByKey = new Map(
      (this.columnConfiguration?.columns ?? []).map(state => [state.key, state]),
    );

    for (const state of stateByKey.values()) {
      if (Number.isFinite(state.width) && (state.width ?? 0) > 0) {
        this.columnWidths.set(state.key, Math.round(state.width!));
      }
    }

    return source
      .filter(column => stateByKey.get(this.getColumnKey(column))?.visible !== false)
      .map((column, index) => ({
        column,
        index,
        order: stateByKey.get(this.getColumnKey(column))?.order ?? index,
      }))
      .sort((a, b) => a.order - b.order || a.index - b.index)
      .map(value => value.column);
  }

  private rebuildColumnVms(): void {
    this.activeColumnVms = this.resolvedActiveColumns.map((column) => {
      const key = this.getColumnKey(column);
      const configured = this.columnWidths.get(key);
      const fallbackWidth = this.pixelWidth(column.width);

      if (configured !== undefined) {
        this.setColumnWidthCss(key, configured);
      } else if (fallbackWidth !== null) {
        this.setColumnWidthCss(key, fallbackWidth);
      }

      return {
        column,
        key,
        widthVar: this.getColumnWidthVar(column),
        flex: this.getColumnFlex(column),
        minWidth: column.minWidth ?? this.MIN_COLUMN_WIDTH,
        maxWidth: column.maxWidth ?? 1200,
      };
    });
  }

  private rebuildVisibleRowVms(): void {
    this.visibleRowVms = this.visibleEntriesCache.map((entry) => this.buildRowVm(entry));
  }

  private buildRowVm(entry: FileInfo): DetailsRowVm {
    const selected = this.selectedFiles.has(entry.path);
    const cursor = entry.path === this.cursorPath;
    const directorySizePending = this.pendingDirectorySizePaths.has(entry.path);
    const dragOverInside = this.dragOverPath === entry.path && this.dragOverPosition === 'inside';
    const dragOverBefore = this.dragOverPath === entry.path && this.dragOverPosition === 'before';
    const dragOverAfter = this.dragOverPath === entry.path && this.dragOverPosition === 'after';
    const displayNameOverride = this.pendingRenameDisplayNames.get(entry.path) ?? null;
    const existing = this.rowVmCache.get(entry.path);

    if (
      existing &&
      existing.entry === entry &&
      existing.selected === selected &&
      existing.cursor === cursor &&
      existing.directorySizePending === directorySizePending &&
      existing.dragOverInside === dragOverInside &&
      existing.dragOverBefore === dragOverBefore &&
      existing.dragOverAfter === dragOverAfter &&
      existing.displayNameOverride === displayNameOverride
    ) {
      return existing;
    }

    const vm: DetailsRowVm = {
      entry,
      selected,
      cursor,
      parentDir: this.isParentEntry(entry),
      renaming: this.renamingPath === entry.path,
      editingName: this.renamingPath === entry.path ? this.editingName : '',
      directorySizePending,
      dragOverInside,
      dragOverBefore,
      dragOverAfter,
      draggable: !this.isParentEntry(entry),
      displayNameOverride,
      cells: this.activeColumnVms.map((columnVm) => this.buildCellVm(entry, columnVm, directorySizePending)),
    };
    this.rowVmCache.set(entry.path, vm);
    return vm;
  }

  private buildCellVm(entry: FileInfo, columnVm: DetailsColumnVm, directorySizePending: boolean): DetailsCellVm {
    const isName = columnVm.column.field === 'name';
    return {
      column: columnVm.column,
      key: `${entry.path}:${columnVm.key}`,
      widthVar: columnVm.widthVar,
      flex: columnVm.flex,
      value: isName ? '' : this.getCellValue(entry, columnVm.column),
      isName,
      showDirectorySizePending: columnVm.column.field === 'size' && directorySizePending,
    };
  }

  trackByRowPath(_i: number, row: DetailsRowVm): string {
    return row.entry.path;
  }

  trackByColumnKey(_i: number, column: DetailsColumnVm): string {
    return column.key;
  }

  trackByCellKey(_i: number, cell: DetailsCellVm): string {
    return cell.key;
  }

  private computeIsRegistryListing(): boolean {
    if (isRegistryPath(this.currentPath)) {
      return true;
    }
    for (const entry of this.entries) {
      if (!entry || this.isParentEntry(entry)) {
        continue;
      }
      if (isRegistryPath(entry.path) || !!entry.registryKind || !!entry.registryValueType || !!entry.registryPreview) {
        return true;
      }
    }
    return false;
  }

  private computeIsSearchListing(): boolean {
    if ((this.currentPath || '').toLowerCase().startsWith('search://')) {
      return true;
    }
    for (const entry of this.entries) {
      if (!entry || this.isParentEntry(entry)) {
        continue;
      }
      if (!!entry.searchParentPath || !!entry.searchPattern || !!entry.searchBasePath) {
        return true;
      }
    }
    return false;
  }

  trackByPath(_i: number, entry: FileInfo): string {
    return entry.path;
  }

  onRenameInput(entry: FileInfo, event: Event): void {
    if (this.renamingPath !== entry.path) return;
    const target = event.target as HTMLInputElement | null;
    this.editingName = target?.value ?? '';
    this.rebuildVisibleRowVms();
  }

  onRenameBlur(entry: FileInfo): void {
    this.commitRename(entry);
  }

  onRenameKeyDown(entry: FileInfo, event: KeyboardEvent): void {
    if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      this.commitRename(entry);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      this.cancelRename();
    } else {
      event.stopPropagation();
    }
  }

  private commitRename(entry: FileInfo): void {
    if (this.renamingPath !== entry.path) return;
    const newName = (this.editingName ?? '').trim();
    this.renamingPath = null;
    this.editingName = '';
    this.rebuildVisibleRowVms();
    if (!newName || newName === entry.name) {
      this.cdr.markForCheck();
      return;
    }
    this.rename.emit({ entry, newName });
    this.cdr.markForCheck();
  }

  private cancelRename(): void {
    this.renamingPath = null;
    this.editingName = '';
    this.rebuildVisibleRowVms();
    this.cdr.markForCheck();
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
    if (this.isParentEntry(entry)) {
      this.anchorPath = entry.path;
      this.selectionChange.emit([entry]);
      return;
    }

    if (event.ctrlKey || event.metaKey) {
      this.selectionChange.emit(this.buildToggledSelection(entry));
      this.anchorPath = entry.path;
    } else if (event.shiftKey && this.anchorPath) {
      this.rangeSelect(entry);
    } else {
      this.anchorPath = entry.path;
      this.selectionChange.emit([entry]);
      if (this.singleClickToOpen) {
        this.navigate.emit(entry);
      }
    }
  }

  onRowDoubleClick(entry: FileInfo): void {
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
      this.backgroundContextMenu.emit(event);
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
    const selectionPaths = new Set<string>();
    const selectionBottom = this.selectionRect.top + this.selectionRect.height;
    const startIndex = Math.max(0, Math.floor(this.selectionRect.top / this.rowHeight));
    const endIndex = Math.min(this.entries.length - 1, Math.floor(Math.max(0, selectionBottom - 1) / this.rowHeight));

    for (let i = startIndex; i <= endIndex; i++) {
      const entry = this.entries[i];
      if (entry && entry.path !== PARENT_ENTRY_PATH) {
        selectionPaths.add(entry.path);
      }
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

  getCellValue(entry: FileInfo, col: PaneDetailsColumn): string {
    const key = this.getColumnKey(col);
    const value = (entry as any)[col.field];
    const signature = this.cellSignature(entry, col, value);

    let entryCache = this.formattedValueCache.get(entry);
    if (!entryCache) {
      entryCache = new Map();
      this.formattedValueCache.set(entry, entryCache);
    }

    const cached = entryCache.get(key);
    if (cached?.signature === signature) {
      return cached.value;
    }

    const formatted = col.formatter
      ? col.formatter(value, entry)
      : value?.toString() || '';

    entryCache.set(key, { signature, value: formatted });
    return formatted;
  }

  private cellSignature(entry: FileInfo, col: PaneDetailsColumn, value: unknown): string {
    switch (col.field) {
      case 'size':
        return `size:${entry.size}`;
      case 'modified':
      case 'created':
      case 'accessed':
        return `${String(col.field)}:${String(value ?? '')}`;
      case 'type':
        return `type:${entry.type}:${entry.name}:${entry.isSymlink ? 1 : 0}`;
      case 'searchParentPath':
        return `searchParent:${entry.searchParentPath ?? ''}:${entry.path}`;
      case 'registryKind':
        return `registryKind:${entry.registryKind ?? ''}:${entry.type}`;
      default:
        return `${String(col.field)}:${String(value ?? '')}`;
    }
  }

  isDirectorySizePending(path: string): boolean {
    return this.pendingDirectorySizePaths.has(path);
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

  getRegistryKind(entry: FileInfo): string {
	if (entry.registryKind) return entry.registryKind;
	return entry.type === FileType.DIRECTORY ? 'Key' : 'Value';
  }

  getSearchParentPath(entry: FileInfo): string {
	if (entry.searchParentPath) return entry.searchParentPath;
	return parentPath(entry.path);
  }

  formatSize(bytes: number): string {
    if (bytes === 0) return '';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
  }

  formatDate(timestamp: number): string {
    if (!Number.isFinite(timestamp) || timestamp <= 0) return '';
    const date = new Date(timestamp * 1000);
    return Number.isNaN(date.getTime()) ? '' : date.toLocaleString();
  }

  private setColumnWidthCss(key: string, width: number): void {
    const rounded = Math.max(1, Math.round(width));
    this.columnWidths.set(key, rounded);
    this.detailsRootRef?.nativeElement.style.setProperty(
      `--details-column-${this.cssKey(key)}-width`,
      `${rounded}px`,
    );
  }

  private serializeColumnConfiguration(): PaneDetailsColumnState[] {
    return this.resolvedActiveColumns.map((column, order) => {
      const key = this.getColumnKey(column);
      return {
        key,
        visible: true,
        order,
        width: this.columnWidths.get(key),
      };
    });
  }

  private pixelWidth(width: string | undefined): number | null {
    if (!width) return null;
    const match = /^(\d+(?:\.\d+)?)px$/i.exec(width.trim());
    return match ? Number(match[1]) : null;
  }

  private cssKey(value: string): string {
    return value.replace(/[^a-zA-Z0-9_-]/g, '-');
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
    if (this.dragOverPath === entry.path && this.dragOverPosition === position) {
      return;
    }
    this.dragOverPath = entry.path;
    this.dragOverPosition = position;
    this.rebuildVisibleRowVms();
    this.cdr.markForCheck();
  }

  onDragLeave(entry: FileInfo): void {
    if (this.dragOverPath === entry.path) {
      this.dragOverPath = null;
      this.dragOverPosition = null;
      this.rebuildVisibleRowVms();
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
      this.rebuildVisibleRowVms();
      this.cdr.markForCheck();
      return;
    }

    event.stopPropagation();
    const capturedPosition = this.dragOverPosition;
    this.dragOverPath = null;
    this.dragOverPosition = null;
    this.rebuildVisibleRowVms();
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
}