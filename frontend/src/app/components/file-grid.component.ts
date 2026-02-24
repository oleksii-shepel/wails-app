// file-grid.component.ts
import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';

@Component({
  selector: 'app-file-grid',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="file-grid" role="grid">
      <div 
        *ngFor="let entry of entries"
        class="grid-item"
        [class.selected]="isSelected(entry)"
        [class.directory]="entry.type === FileType.DIRECTORY"
        [class.parent-dir]="isParentEntry(entry)"
        [attr.role]="'gridcell'"
        [attr.aria-selected]="isSelected(entry)"
        (click)="onItemClick(entry, $event)"
        (dblclick)="onItemDoubleClick(entry)"
        (contextmenu)="onContextMenu($event, entry)">
        
        <div class="grid-icon">
          <span class="file-emoji">{{ getIcon(entry) }}</span>
          <div class="selection-indicator" *ngIf="isSelected(entry)">
            <span class="check">✓</span>
          </div>
        </div>
        
        <div class="grid-info">
          <div class="grid-name" [title]="entry.name">{{ entry.name }}</div>
          <div class="grid-meta" *ngIf="!isParentEntry(entry)">
            {{ formatSize(entry.size) }}
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .file-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
      gap: var(--vsc-padding-md);
      padding: var(--vsc-padding-md);
      overflow-y: auto;
      background: var(--vsc-editor-background);
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

    .selection-indicator {
      position: absolute;
      bottom: -4px;
      right: -4px;
      width: 18px;
      height: 18px;
      background: var(--vsc-accent-blue);
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      border: 2px solid var(--vsc-editor-background);
      
      .check {
        color: white;
        font-size: 10px;
        font-weight: bold;
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
export class FileGridComponent {
  @Input() entries: FileInfo[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  
  @Output() selectionChange = new EventEmitter<FileInfo>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();

  FileType = FileType;
  private lastClickedEntry: FileInfo | null = null;

  isParentEntry(entry: FileInfo): boolean {
    return entry.path === '__archive_parent__';
  }

  isSelected(entry: FileInfo): boolean {
    return this.selectedFiles.has(entry.path);
  }

  onItemClick(entry: FileInfo, event: MouseEvent): void {
    // Handle multi-select with Ctrl/Cmd
    if (event.ctrlKey || event.metaKey) {
      this.toggleSelection(entry);
    } else if (event.shiftKey && this.lastClickedEntry) {
      this.rangeSelect(entry);
    } else {
      // Single select
      if (this.isParentEntry(entry)) {
        this.navigate.emit(entry);
      } else {
        this.selectionChange.emit(entry);
      }
    }
    this.lastClickedEntry = entry;
  }

  onItemDoubleClick(entry: FileInfo): void {
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

  private toggleSelection(entry: FileInfo): void {
    // Implementation depends on parent component handling
    this.selectionChange.emit(entry);
  }

  private rangeSelect(entry: FileInfo): void {
    // Range selection logic for grid view
    const startIdx = this.entries.indexOf(this.lastClickedEntry!);
    const endIdx = this.entries.indexOf(entry);
    const [min, max] = [Math.min(startIdx, endIdx), Math.max(startIdx, endIdx)];
    
    // Emit range selection event - parent should handle multi-select
    for (let i = min; i <= max; i++) {
      if (!this.isParentEntry(this.entries[i])) {
        this.selectionChange.emit(this.entries[i]);
      }
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