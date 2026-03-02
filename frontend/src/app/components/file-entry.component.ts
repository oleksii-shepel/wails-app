import { Component, Input, Output, EventEmitter, HostBinding } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';

export interface FileEntrySelectEvent {
  entry: FileInfo;
  mouseEvent: MouseEvent;
}

/** Sentinel path for the virtual ".." parent-directory entry. */
export const PARENT_ENTRY_PATH = '__parent__';

@Component({
  selector: 'app-file-entry',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div
      class="file-entry"
      [class.selected]="selected"
      [class.directory]="entry.type === FileType.DIRECTORY"
      [class.parent-dir]="isParentEntry()"
      (click)="onClick($event)"
      (dblclick)="onDoubleClick()">

      <!-- Parent ".." — plain SVG arrow, no badge -->
      <span *ngIf="isParentEntry()" class="file-icon">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
          <path transform="rotate(45,8,8)"
            d="M7.78 3.22a.75.75 0 0 1 0 1.06L4.56 7.5H13.25a.75.75 0 0 1 0 1.5H4.56l3.22 3.22a.75.75 0 1 1-1.06 1.06l-4.5-4.5a.75.75 0 0 1 0-1.06l4.5-4.5a.75.75 0 0 1 1.06 0z"/>
        </svg>
      </span>

      <!-- Normal entry: emoji + optional symlink badge -->
      <span *ngIf="!isParentEntry()" class="file-icon">
        <span class="icon-emoji">{{ getIconEmoji() }}</span>
        <svg *ngIf="isSymlink()" class="symlink-badge" viewBox="0 0 10 10" fill="none" aria-label="symlink">
          <path d="M2 8 C2 5 5 3 8 2"
            stroke="var(--vsc-icon-symlink,#5dade2)" stroke-width="1.8" stroke-linecap="round"/>
          <path d="M6 1.2 L8.8 1.2 L8.8 4"
            stroke="var(--vsc-icon-symlink,#5dade2)" stroke-width="1.8"
            stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </span>

      <div class="file-info">
        <div class="file-name">{{ entry.name }}</div>
        <div class="file-details" *ngIf="!isParentEntry()">
          <span class="file-size">{{ formatSize(entry.size) }}</span>
          <span class="file-date">{{ formatDate(entry.modified) }}</span>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .file-entry {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-md);
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      border-radius: var(--vsc-border-radius-sm);
      cursor: pointer;
      transition: all var(--vsc-transition-fast);
      user-select: none;
      min-height: 28px;
      border-left: 3px solid transparent;

      &:hover {
        background: var(--vsc-list-hover-background);
        border-left-color: var(--vsc-border-bright);
      }

      &.selected {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
        border-left-color: var(--vsc-accent-blue);

        &:hover { background: var(--vsc-list-focus-background); }
        .file-name { font-weight: 500; }
      }

      &.directory .file-icon { opacity: 1; }

      &.parent-dir {
        opacity: 0.65;
        border-left-color: transparent !important;

        &:hover {
          opacity: 0.9;
          background: var(--vsc-list-hover-background);
          border-left-color: var(--vsc-border-bright) !important;
        }
        &.selected { background: transparent; color: inherit; }
        .file-name { font-style: italic; letter-spacing: 0.02em; }
      }

      .file-info { flex: 1; min-width: 0; }

      .file-name {
        font-size: var(--vsc-font-size);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        line-height: 1.4;
      }

      .file-details {
        display: flex;
        gap: var(--vsc-padding-lg);
        font-size: var(--vsc-font-size-small);
        color: var(--vsc-foreground-dim);
        margin-top: 2px;

        .file-size, .file-date { white-space: nowrap; }
      }
    }

    /* ── Icon container ──────────────────────────────────────
       Fixed size gives the badge a stable anchor to sit on.
       position:relative lets the badge use absolute positioning.
    ───────────────────────────────────────────────────────── */
    .file-icon {
      flex-shrink: 0;
      position: relative;
      width: 20px;
      height: 20px;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      --symlink-badge-size: 9px;
      --symlink-badge-right: 0px;
      --symlink-badge-bottom: 0px;
      --symlink-badge-transform: translate(0%, 0%);
      --vsc-icon-symlink: color-mix(in srgb, var(--vsc-accent-blue) 82%, white 18%);
    }

    .icon-emoji {
      font-size: 15px;
      line-height: 1;
      display: block;
    }

    /* ── Symlink badge ───────────────────────────────────────
       Cyan curved-arrow SVG, pinned to the bottom-right of
       the icon cell.  Dark halo keeps it legible on any bg.
    ───────────────────────────────────────────────────────── */
    .symlink-badge {
      position: absolute;
      right: var(--symlink-badge-right);
      bottom: var(--symlink-badge-bottom);
      width: var(--symlink-badge-size);
      height: var(--symlink-badge-size);
      pointer-events: none;
      filter: drop-shadow(0 0 1.5px var(--vsc-editor-background, #1e1e1e));
      transform: var(--symlink-badge-transform);
    }

    :host-context(.compact-mode) .file-entry {
      min-height: 24px;
      padding: var(--vsc-padding-xs) var(--vsc-padding-sm);
      gap: var(--vsc-padding-sm);

      .file-icon      { width: 16px; height: 16px; }
      .icon-emoji     { font-size: 13px; }
      .file-icon      {
        --symlink-badge-size: 7px;
        --symlink-badge-right: 0px;
        --symlink-badge-bottom: 0px;
        --symlink-badge-transform: translate(18%, 18%);
      }
      .file-name      { font-size: var(--vsc-font-size-small); }
    }

    :host(.variant-inline) .file-entry {
      min-height: auto;
      padding: 0;
      gap: 6px;
      border-left: none;
      border-radius: 0;
      background: transparent;
      pointer-events: none;
    }

    :host(.variant-inline) .file-entry:hover,
    :host(.variant-inline) .file-entry.selected {
      background: transparent;
      border-left: none;
    }

    :host(.variant-inline) .file-details {
      display: none;
    }

    :host(.variant-inline) .file-icon {
      width: 16px;
      height: 16px;
      --symlink-badge-size: 7px;
      --symlink-badge-right: 0px;
      --symlink-badge-bottom: 0px;
      --symlink-badge-transform: translate(18%, 18%);
    }

    :host(.variant-inline) .icon-emoji {
      font-size: 13px;
    }

    :host(.variant-grid) .file-entry {
      min-height: auto;
      padding: 0;
      gap: 4px;
      border-left: none;
      border-radius: 0;
      background: transparent;
      pointer-events: none;
      flex-direction: column;
      justify-content: flex-start;
    }

    :host(.variant-grid) .file-entry:hover,
    :host(.variant-grid) .file-entry.selected {
      background: transparent;
      border-left: none;
    }

    :host(.variant-grid) .file-details {
      display: none;
    }

    :host(.variant-grid) .file-info {
      text-align: center;
      width: 100%;
    }

    :host(.variant-grid) .file-name {
      font-size: var(--vsc-font-size-small);
      line-height: 1.3;
    }

    :host(.variant-grid) .file-icon {
      width: 48px;
      height: 48px;
      --symlink-badge-size: 10px;
      --symlink-badge-right: 8px;
      --symlink-badge-bottom: 8px;
      --symlink-badge-transform: translate(35%, 35%);
    }

    :host(.variant-grid) .icon-emoji {
      font-size: 32px;
    }
  `]
})
export class FileEntryComponent {
  @Input() entry!: FileInfo;
  @Input() selected = false;
  @Input() variant: 'row' | 'inline' | 'grid' = 'row';

  @Output() select   = new EventEmitter<FileEntrySelectEvent>();
  @Output() navigate = new EventEmitter<FileInfo>();

  FileType = FileType;

  @HostBinding('class.variant-inline') get isInlineVariant(): boolean {
    return this.variant === 'inline';
  }

  @HostBinding('class.variant-grid') get isGridVariant(): boolean {
    return this.variant === 'grid';
  }

  isParentEntry(): boolean {
    return this.entry?.path === PARENT_ENTRY_PATH;
  }

  /** True for file-symlinks (type === SYMLINK) and dir-symlinks (type === DIRECTORY, isSymlink === true). */
  isSymlink(): boolean {
    return this.entry?.type === FileType.SYMLINK || !!this.entry?.isSymlink;
  }

  /**
   * Base emoji for the icon.
   * Symlinks show the icon of their target kind — the cyan badge
   * is the only indicator that distinguishes them from real entries.
   */
  getIconEmoji(): string {
    if (!this.entry) return '📄';

    switch (this.entry.type) {
      case FileType.DIRECTORY: return '📁';
      // File symlink: render as a plain file so the badge does the talking
      case FileType.SYMLINK:   return '📄';
      default:                 return this.extIcon(this.entry.name);
    }
  }

  onClick(mouseEvent: MouseEvent): void { this.select.emit({ entry: this.entry, mouseEvent }); }
  onDoubleClick(): void { this.navigate.emit(this.entry); }

  private extIcon(name: string): string {
    const ext = name.split('.').pop()?.toLowerCase();
    switch (ext) {
      case 'txt': case 'md':                return '📄';
      case 'jpg': case 'jpeg': case 'png':
      case 'gif': case 'webp':              return '🖼️';
      case 'pdf':                           return '📕';
      case 'zip': case 'tar': case 'gz':
      case 'tgz': case 'rar': case '7z':   return '📦';
      case 'mp3': case 'wav': case 'flac': return '🎵';
      case 'mp4': case 'avi': case 'mkv':  return '🎬';
      case 'js':  case 'ts':  case 'jsx':
      case 'tsx':                           return '📝';
      case 'json': case 'yaml': case 'yml':return '📋';
      case 'html': case 'htm':             return '🌐';
      case 'css':  case 'scss':            return '🎨';
      default:                             return '📄';
    }
  }

  formatSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
  }

  formatDate(timestamp: number): string {
    return new Date(timestamp * 1000).toLocaleString();
  }
}
