// file-details.component.ts
import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from './file-entry.component';

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
      <div class="details-body" role="rowgroup">
        <div 
          *ngFor="let entry of entries"
          class="details-row"
          [class.selected]="isSelected(entry)"
          [class.parent-dir]="isParentEntry(entry)"
          [attr.role]="'row'"
          [attr.aria-selected]="isSelected(entry)"
          (click)="onRowClick(entry, $event)"
          (dblclick)="onRowDoubleClick(entry)"
          (contextmenu)="onContextMenu($event, entry)">
          
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
      </div>
    </div>
  `,
  styles: [`
    .file-details {
      display: flex;
      flex-direction: column;
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
      overflow-y: auto;
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
export class FileDetailsComponent {
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
  
  @Output() selectionChange = new EventEmitter<FileInfo>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() sort = new EventEmitter<{ field: string; direction: 'asc' | 'desc' }>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();

  private lastClickedEntry: FileInfo | null = null;

  isParentEntry(entry: FileInfo): boolean {
    return entry.path === PARENT_ENTRY_PATH;
  }

  isSelected(entry: FileInfo): boolean {
    return this.selectedFiles.has(entry.path);
  }

  onRowClick(entry: FileInfo, event: MouseEvent): void {
    if (this.isParentEntry(entry)) {
      this.navigate.emit(entry);
      return;
    }

    if (event.ctrlKey || event.metaKey) {
      // Toggle selection
      this.selectionChange.emit(entry);
    } else if (event.shiftKey && this.lastClickedEntry) {
      // Range selection
      this.rangeSelect(entry);
    } else {
      this.selectionChange.emit(entry);
    }
    this.lastClickedEntry = entry;
  }

  onRowDoubleClick(entry: FileInfo): void {
    if (!this.isParentEntry(entry)) {
      this.navigate.emit(entry);
    }
  }

  onContextMenu(event: MouseEvent, entry: FileInfo): void {
    event.preventDefault();
    if (!this.isSelected(entry)) {
      this.selectionChange.emit(entry);
    }
    this.contextMenu.emit({ event, entry });
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
    const startIdx = this.entries.indexOf(this.lastClickedEntry!);
    const endIdx = this.entries.indexOf(entry);
    const [min, max] = [Math.min(startIdx, endIdx), Math.max(startIdx, endIdx)];
    
    for (let i = min; i <= max; i++) {
      if (!this.isParentEntry(this.entries[i])) {
        this.selectionChange.emit(this.entries[i]);
      }
    }
  }
}