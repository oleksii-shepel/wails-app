import { Component, Input, Output, EventEmitter, HostBinding, ElementRef, ViewChild } from '@angular/core';
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
      [class.cursor]="cursor"
      [class.directory]="entry.type === FileType.DIRECTORY"
      [class.parent-dir]="isParentEntry()"
      (click)="onClick($event)"
      (dblclick)="onDoubleClick()">

      <span class="file-icon" [class.parent-icon]="isParentEntry()">
        <ng-container *ngIf="isParentEntry(); else normalIcon">
          <svg
            width="20"
            height="20"
            viewBox="0 0 20 20"
            fill="none"
            stroke="currentColor"
            stroke-width="1.6"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true">
            <path d="M13 15 L7 15 L7 6"/>
            <path d="M4.5 8.5 L7 6 L9.5 8.5"/>
          </svg>
        </ng-container>
        <ng-template #normalIcon>
          <span class="icon-emoji">{{ getIconEmoji() }}</span>
          <svg *ngIf="isSymlink()" class="symlink-badge" viewBox="0 0 10 10" fill="none" aria-label="symlink">
            <!-- Curved arrow body -->
            <path d="M2 8 C2 5 5 3 8 2"
              stroke="var(--vsc-icon-symlink,#5dade2)" stroke-width="2" stroke-linecap="round"/>
            <!-- Arrow corner/head -->
            <path d="M6 1.2 L8.8 1.2 L8.8 4"
              stroke="var(--vsc-icon-symlink,#5dade2)" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </ng-template>
      </span>

      <div class="file-info">
        <div class="file-name">
          <input
            *ngIf="isRenaming"
            #renameInput
            class="rename-input"
            [value]="editingName"
            (click)="$event.stopPropagation()"
            (dblclick)="$event.stopPropagation()"
            (mousedown)="$event.stopPropagation()"
            (input)="onRenameInput($event)"
            (blur)="onRenameBlur()"
            (keydown)="onRenameKeyDown($event)" />
          <span *ngIf="!isRenaming">{{ entry.name }}</span>
        </div>
        <div class="file-details" [attr.aria-hidden]="isParentEntry() ? 'true' : null">
          <span class="file-size" *ngIf="!isParentEntry(); else detailPlaceholder">{{ formatSize(entry.size) }}</span>
          <span class="file-date" *ngIf="!isParentEntry(); else detailPlaceholder">{{ formatDate(entry.modified) }}</span>
        </div>
      </div>
    </div>
    <ng-template #detailPlaceholder>&nbsp;</ng-template>
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
      position: relative; /* Add positioning context */
      color: var(--vsc-foreground);

      &:hover {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground);
        border-left-color: var(--vsc-border-bright);
      }

      &.selected {
        background: var(--vsc-list-active-background);
        color: var(--vsc-list-active-foreground);
        /* Fix border positioning */
        border-left-color: var(--vsc-accent-blue);
        border-left-width: 3px;
        border-left-style: solid;
        /* Ensure border aligns perfectly */
        margin-left: 0;
        padding-left: var(--vsc-padding-md); /* Keep consistent padding */

        .file-details { color: var(--vsc-list-active-foreground-dim); }

        &:hover { background: var(--vsc-list-focus-background); }
      }

      /* TC-style cursor: position indicator separate from marks */
      &.cursor:not(.selected) {
        background: color-mix(in srgb, var(--vsc-list-active-background) 35%, transparent);
        border-left-color: var(--vsc-accent-blue);
      }

      &.cursor.selected {
        outline: none;
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
        .file-details { visibility: hidden; }
        .file-icon { opacity: calc(1 / 0.65); }
      }

      .file-info { flex: 1; min-width: 0; }

      .file-name {
        display: flex;
        align-items: center;
        font-size: var(--vsc-font-size);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        line-height: 1.4;
      }

      .rename-input {
        width: 100%;
        min-width: 0;
        box-sizing: border-box;
        font: inherit;
        line-height: inherit;
        color: var(--vsc-input-foreground);
        background: var(--vsc-input-background);
        border: 1px solid var(--vsc-input-border);
        border-radius: var(--vsc-border-radius-sm);
        padding: var(--vsc-padding-xs) var(--vsc-padding-sm);
        outline: none;
      }

      .rename-input:focus {
        border-color: var(--vsc-input-focus-border);
        box-shadow: 0 0 0 1px var(--vsc-input-focus-border);
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

      /* Badge geometry tokens — overridden per variant below */
      --symlink-badge-size: 10px;
      --symlink-badge-right: -1px;
      --symlink-badge-bottom: -1px;
      --symlink-badge-transform: translate(0%, 0%);

      /* Symlink arrow colour: accent-blue lightened slightly */
      --vsc-icon-symlink: color-mix(in srgb, var(--vsc-accent-blue, #4fc3f7) 80%, white 20%);
    }

    .parent-icon svg {
      width: 100%;
      height: 100%;
      display: block;
    }

    .icon-emoji {
      font-size: 15px;
      line-height: 1;
      display: block;
    }

    /* ── Symlink badge ───────────────────────────────────────
       Pinned to the bottom-right corner of the icon cell.
       Two-layer shadow: dark outline for contrast on any bg +
       subtle cyan glow for identity/visibility.
    ───────────────────────────────────────────────────────── */
    .symlink-badge {
      position: absolute;
      right: var(--symlink-badge-right);
      bottom: var(--symlink-badge-bottom);
      width: var(--symlink-badge-size);
      height: var(--symlink-badge-size);
      pointer-events: none;
      transform: var(--symlink-badge-transform);

      /*
        Layer 1 – dark halo: punches the badge out from any bg colour.
        Layer 2 – cyan glow: ties the badge to the symlink accent colour.
      */
      filter:
        drop-shadow(0 0 1.5px color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 90%, transparent))
        drop-shadow(0 0 2.5px color-mix(in srgb, var(--vsc-icon-symlink, #4fc3f7) 55%, transparent));
    }

    /* ── Compact mode ───────────────────────────────────────── */
    :host-context(.compact-mode) .file-entry {
      min-height: 24px;
      padding: var(--vsc-padding-xs) var(--vsc-padding-sm);
      gap: var(--vsc-padding-sm);

      .file-name { font-size: var(--vsc-font-size-small); }
    }

    :host-context(.compact-mode) .file-icon {
      width: 16px;
      height: 16px;
      --symlink-badge-size: 8px;
      --symlink-badge-right: -1px;
      --symlink-badge-bottom: -1px;
      --symlink-badge-transform: translate(0%, 0%);
    }

    :host-context(.compact-mode) .icon-emoji {
      font-size: 13px;
    }

    /* ── Inline variant ─────────────────────────────────────── */
    :host(.variant-inline) .file-entry {
      min-height: auto;
      padding: 0;
      gap: 6px;
      border-left: none;
      border-radius: 0;
      background: transparent;
      pointer-events: none;
    }

    :host(.variant-inline).renaming .file-entry {
      pointer-events: auto;
    }

    :host(.variant-inline) .file-entry:hover,
    :host(.variant-inline) .file-entry.selected,
    :host(.variant-inline) .file-entry.cursor {
      background: transparent;
      border-left: none;
      outline: none;
    }

    :host(.variant-inline) .file-details {
      display: none;
    }

    :host(.variant-inline) .file-icon {
      width: 16px;
      height: 16px;
      --symlink-badge-size: 8px;
      --symlink-badge-right: -1px;
      --symlink-badge-bottom: -1px;
      --symlink-badge-transform: translate(0%, 0%);
    }

    :host(.variant-inline) .icon-emoji {
      font-size: 13px;
    }

    /* ── Grid variant ───────────────────────────────────────── */
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
    :host(.variant-grid) .file-entry.selected,
    :host(.variant-grid) .file-entry.cursor {
      background: transparent;
      border-left: none;
      outline: none;
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
      justify-content: center;
      text-align: center;
    }

    :host(.variant-grid).renaming .file-entry {
      pointer-events: auto;
    }

    :host(.variant-grid) .file-icon {
      width: 48px;
      height: 48px;
      --symlink-badge-size: 12px;
      --symlink-badge-right: 4px;
      --symlink-badge-bottom: 4px;
      --symlink-badge-transform: translate(0%, 0%);
    }

    :host(.variant-grid) .icon-emoji {
      font-size: 32px;
    }
  `]
})
export class FileEntryComponent {
  @Input() entry!: FileInfo;
  @Input() selected = false;
  @Input() cursor = false;
  @Input() variant: 'row' | 'inline' | 'grid' = 'row';

  @Output() select   = new EventEmitter<FileEntrySelectEvent>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() rename = new EventEmitter<{ entry: FileInfo; newName: string }>();

  @ViewChild('renameInput') renameInput?: ElementRef<HTMLInputElement>;

  isRenaming = false;
  editingName = '';

  FileType = FileType;

  @HostBinding('class.variant-inline') get isInlineVariant(): boolean {
    return this.variant === 'inline';
  }

  @HostBinding('class.variant-grid') get isGridVariant(): boolean {
    return this.variant === 'grid';
  }

  @HostBinding('class.renaming') get isRenamingHost(): boolean {
    return this.isRenaming;
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

  onClick(mouseEvent: MouseEvent): void {
    if (this.isRenaming) return;
    this.select.emit({ entry: this.entry, mouseEvent });
  }

  onDoubleClick(): void {
    if (this.isRenaming) return;
    this.navigate.emit(this.entry);
  }

  /** Begin in-place rename. Parent components can call this (eg. for F2). */
  beginRename(): void {
    if (!this.entry || this.isParentEntry()) return;
    this.isRenaming = true;
    this.editingName = this.entry.name;
    setTimeout(() => {
      const input = this.renameInput?.nativeElement;
      if (!input) return;
      input.focus();
      input.select();
    }, 0);
  }

  private commitRename(): void {
    if (!this.isRenaming) return;
    const newName = (this.editingName ?? '').trim();
    this.isRenaming = false;
    if (!newName || newName === this.entry.name) return;
    this.rename.emit({ entry: this.entry, newName });
  }

  private cancelRename(): void {
    this.isRenaming = false;
    this.editingName = '';
  }

  onRenameInput(event: Event): void {
    const target = event.target as HTMLInputElement | null;
    this.editingName = target?.value ?? '';
  }

  onRenameBlur(): void {
    // Commit on blur to match common file manager UX.
    this.commitRename();
  }

  onRenameKeyDown(event: KeyboardEvent): void {
    if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      this.commitRename();
    } else if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      this.cancelRename();
    } else {
      event.stopPropagation();
    }
  }

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
    if (!Number.isFinite(timestamp) || timestamp <= 0) return '';
    const date = new Date(timestamp * 1000);
    return Number.isNaN(date.getTime()) ? '' : date.toLocaleString();
  }
}
