import { ChangeDetectionStrategy, ChangeDetectorRef, Component, ElementRef, EventEmitter, HostBinding, Input, OnChanges, OnDestroy, Output, SimpleChanges, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';
import { getFileEntryPresentation } from './file-entry-presentation.util';
import { FileCategoryResolverService } from '../services/file-category-resolver.service';

export interface FileEntrySelectEvent {
  entry: FileInfo;
  mouseEvent: MouseEvent;
}

/** Sentinel path for the virtual ".." parent-directory entry. */
export const PARENT_ENTRY_PATH = '__parent__';

/** Smart truncate file names while preserving extension. */
function truncateFileName(name: string, maxLength: number): string {
  if (!name || name.length <= maxLength) {
    return name;
  }

  // Find the last dot to separate name from extension
  const lastDotIndex = name.lastIndexOf('.');
  const hasExtension = lastDotIndex > 0 && lastDotIndex < name.length - 1;

  if (!hasExtension || lastDotIndex < 3) {
    // No extension or very short base name, just truncate at end
    return name.slice(0, Math.max(1, maxLength - 1)) + '…';
  }

  const baseName = name.slice(0, lastDotIndex);
  const extension = name.slice(lastDotIndex);
  const maxBaseLength = Math.max(1, maxLength - extension.length - 1); // -1 for ellipsis

  if (maxBaseLength < 3) {
    // Not enough space to show meaningful base + extension
    return name.slice(0, maxLength - 1) + '…';
  }

  // Truncate base name and add ellipsis in the middle to hint at full length
  const keep = Math.max(1, Math.floor(maxBaseLength / 2) - 1);
  return baseName.slice(0, keep) + '…' + extension;
}

@Component({
  selector: 'app-file-entry',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="file-entry"
      [class.selected]="selected"
      [class.cursor]="cursor"
        [class.directory]="entry.type === FileType.DIRECTORY"
      [class.parent-dir]="isParent"
      (click)="onClick($event)"
      (dblclick)="onDoubleClick()">
      <ng-container *ngIf="isInlineVariant; else defaultLayout">
        <span class="file-icon" [class.parent-icon]="isParent">
          <ng-container *ngTemplateOutlet="entryIcon"></ng-container>
        </span>

        <div class="file-info">
          <div class="file-name">
            <ng-container *ngTemplateOutlet="entryName"></ng-container>
          </div>
        </div>
      </ng-container>

      <ng-template #defaultLayout>
        <span class="file-icon" [class.parent-icon]="isParent">
          <ng-container *ngTemplateOutlet="entryIcon"></ng-container>
        </span>

        <div class="file-info">
          <div class="file-name">
            <ng-container *ngTemplateOutlet="entryName"></ng-container>
            <span *ngIf="showSearchBadgeInVariant" class="entry-chip search-chip">Search</span>
          </div>
          <div class="file-context-grid" *ngIf="showSearchContextGrid">{{ searchParentPath }}</div>
          <div class="file-details" [attr.aria-hidden]="isParent ? 'true' : null">
            <ng-container *ngIf="!isParent; else detailPlaceholder">
              <span class="file-size file-size--pending" *ngIf="showDirectorySizeSpinner; else fileSizeValue">
                <span
                  class="file-size-spinner"
                  aria-label="Calculating folder size"
                  title="Calculating folder size">
                </span>
              </span>
              <ng-template #fileSizeValue>
                <span class="file-size">{{ formatSize(entry.size) }}</span>
              </ng-template>
              <span class="file-date">{{ formatDate(entry.modified) }}</span>
            </ng-container>
            <span class="file-context" *ngIf="showSearchContextRow">{{ searchParentPath }}</span>
          </div>
        </div>
      </ng-template>
    </div>

    <ng-template #entryIcon>
      <ng-container *ngIf="isParent; else normalIcon">
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
        <img
          *ngIf="variant === 'grid' && thumbnailSrc; else emojiIcon"
          class="file-thumbnail"
          [src]="thumbnailSrc"
          alt=""
          draggable="false"
          decoding="async" />
        <ng-template #emojiIcon>
          <span class="icon-emoji">{{ iconEmoji }}</span>
        </ng-template>
        <span
          *ngIf="variant === 'grid' && thumbnailLoading && !thumbnailSrc"
          class="thumbnail-loading"
          aria-hidden="true">
        </span>
        <svg *ngIf="isSymlinkEntry" class="symlink-badge" viewBox="0 0 10 10" fill="none" aria-label="symlink">
          <path d="M2 8 C2 5 5 3 8 2"
            stroke="var(--vsc-icon-symlink,var(--vsc-info))" stroke-width="2" stroke-linecap="round"/>
          <path d="M6 1.2 L8.8 1.2 L8.8 4"
            stroke="var(--vsc-icon-symlink,var(--vsc-info))" stroke-width="2"
            stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </ng-template>
    </ng-template>

    <ng-template #entryName>
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
      <ng-container *ngIf="!isRenaming">
        <span class="file-name-text" [attr.data-mime-kind]="mimeColorKind" [title]="fullName">{{ displayName }}</span>
        <span
          *ngIf="showDirectorySizeSpinnerInName && showDirectorySizeSpinner"
          class="file-size-spinner file-size-spinner--inline"
          aria-label="Calculating folder size"
          title="Calculating folder size">
        </span>
      </ng-container>
    </ng-template>

    <ng-template #detailPlaceholder>&nbsp;</ng-template>
  `,
  styles: [`
    :host {
      min-width: 0;
      max-width: 100%;
      overflow: hidden;
    }

    .file-info,
    .file-name,
    .file-name-text {
      min-width: 0;
      max-width: 100%;
    }

    .file-name,
    .file-name-text {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }


    .file-entry {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-md);
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      border-radius: var(--vsc-border-radius-sm);
      background: transparent;
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
        gap: 6px;
        font-size: var(--vsc-font-size);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        line-height: 1.4;
      }

      .file-name-text {
        color: var(--entry-name-color, inherit);
        font-weight: 400; /* Prevent inherited weight changes on selection */
      }

      .file-name-text[data-mime-kind='default'] { --entry-name-color: inherit; }
      .file-name-text[data-mime-kind='directory'] { --entry-name-color: var(--vsc-entry-directory); }
      .file-name-text[data-mime-kind='archive'] { --entry-name-color: var(--vsc-entry-archive); }
      .file-name-text[data-mime-kind='symlink'] { --entry-name-color: var(--vsc-entry-symlink); }
      .file-name-text[data-mime-kind='images'] { --entry-name-color: var(--vsc-entry-images); }
      .file-name-text[data-mime-kind='documents'] { --entry-name-color: var(--vsc-entry-documents); }
      .file-name-text[data-mime-kind='spreadsheets'] { --entry-name-color: var(--vsc-entry-spreadsheets); }
      .file-name-text[data-mime-kind='presentations'] { --entry-name-color: var(--vsc-entry-presentations); }
      .file-name-text[data-mime-kind='code'] { --entry-name-color: var(--vsc-entry-code); }
      .file-name-text[data-mime-kind='web'] { --entry-name-color: var(--vsc-entry-web); }
      .file-name-text[data-mime-kind='data'] { --entry-name-color: var(--vsc-entry-data); }
      .file-name-text[data-mime-kind='archives'] { --entry-name-color: var(--vsc-entry-archives); }
      .file-name-text[data-mime-kind='audio'] { --entry-name-color: var(--vsc-entry-audio); }
      .file-name-text[data-mime-kind='video'] { --entry-name-color: var(--vsc-entry-video); }
      .file-name-text[data-mime-kind='fonts'] { --entry-name-color: var(--vsc-entry-fonts); }
      .file-name-text[data-mime-kind='markdown'] { --entry-name-color: var(--vsc-entry-markdown); }
      .file-name-text[data-mime-kind='scripts'] { --entry-name-color: var(--vsc-entry-scripts); }
      .file-name-text[data-mime-kind='3d'] { --entry-name-color: var(--vsc-entry-3d); }
      .file-name-text[data-mime-kind='design'] { --entry-name-color: var(--vsc-entry-design); }
      .file-name-text[data-mime-kind='executable'] { --entry-name-color: var(--vsc-entry-executable); }

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
        min-width: 0;

        .file-size, .file-date { white-space: nowrap; }

        .file-size--pending {
          display: inline-flex;
          align-items: center;
          min-height: 12px;
        }

        .file-context {
          min-width: 0;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
      }

      .file-size-spinner {
        width: 12px;
        height: 12px;
        border-radius: 999px;
        border: 2px solid color-mix(in srgb, var(--vsc-accent-blue) 22%, transparent);
        border-top-color: var(--vsc-accent-blue);
        box-sizing: border-box;
        animation: file-size-spin 0.9s linear infinite;
        flex-shrink: 0;
      }

      .file-size-spinner--inline {
        width: 11px;
        height: 11px;
      }

      .entry-chip {
        display: inline-flex;
        align-items: center;
        height: 16px;
        padding: 0 6px;
        border-radius: 999px;
        font-size: 10px;
        font-weight: 700;
        letter-spacing: 0.04em;
        text-transform: uppercase;
        white-space: nowrap;
      }

      .search-chip {
        background: color-mix(in srgb, var(--vsc-accent-blue) 18%, transparent);
        color: var(--vsc-accent-blue);
        border: 1px solid color-mix(in srgb, var(--vsc-accent-blue) 35%, transparent);
      }

      .file-context-grid {
        display: none;
      }
    }

    /* ── Icon container ──────────────────────────────────────
       Fixed size gives the badge a stable anchor to sit on.
       position:relative lets the badge use absolute positioning.
    ───────────────────────────────────────────────────────── */
    .file-icon {
      flex-shrink: 0;
      position: relative;
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
      --vsc-icon-symlink: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-info)) 80%, white 20%);
    }

    .parent-icon svg {
      width: 100%;
      height: 100%;
      display: block;
    }

    .icon-emoji {
      width: 100%;
      height: 100%;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 15px;
      line-height: 1;
      text-align: center;
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
        drop-shadow(0 0 1.5px color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 90%, transparent))
        drop-shadow(0 0 2.5px color-mix(in srgb, var(--vsc-icon-symlink, var(--vsc-info)) 55%, transparent));
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

    :host(.variant-inline) .entry-chip,
    :host(.variant-inline) .file-context-grid {
      display: none;
    }

    :host(.variant-inline) .file-size-spinner--inline {
      margin-left: 2px;
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
      display: inline-flex;
      flex-wrap: wrap;
      font-size: var(--vsc-font-size-small);
      line-height: 1.3;
      justify-content: center;
      text-align: center;
      white-space: normal;
    }

    :host(.variant-grid) .entry-chip {
      margin-top: 2px;
      justify-content: center;
    }

    :host(.variant-grid) .file-context-grid {
      display: block;
      margin-top: 4px;
      font-size: 11px;
      line-height: 1.25;
      color: var(--vsc-foreground-dim);
      overflow: hidden;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
      word-break: break-word;
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

    :host(.variant-grid) .file-thumbnail {
      display: block;
      width: 48px;
      height: 48px;
      object-fit: cover;
      border-radius: 4px;
      background: var(--vsc-editor-background);
      box-shadow: 0 1px 4px var(--vsc-shadow-soft);
      user-select: none;
      pointer-events: none;
    }

    :host(.variant-grid) .thumbnail-loading {
      position: absolute;
      width: 16px;
      height: 16px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue) 20%, transparent);
      border-top-color: var(--vsc-accent-blue);
      border-radius: 999px;
      animation: file-size-spin .8s linear infinite;
      pointer-events: none;
    }

    @keyframes file-size-spin {
      from { transform: rotate(0deg); }
      to { transform: rotate(360deg); }
    }
  `]
})
export class FileEntryComponent implements OnChanges, OnDestroy {
  @Input() entry!: FileInfo;
  @Input() selected = false;
  @Input() cursor = false;
  @Input() displayNameOverride: string | null = null;
  @Input() variant: 'row' | 'inline' | 'grid' = 'row';
  @Input() colorizeByMime = false;
  @Input() directorySizePending = false;
  @Input() showDirectorySizeSpinnerInName = false;
  @Input() thumbnailSrc: string | null = null;
  @Input() thumbnailLoading = false;

  @Output() select   = new EventEmitter<FileEntrySelectEvent>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() rename = new EventEmitter<{ entry: FileInfo; newName: string }>();

  @ViewChild('renameInput') renameInput?: ElementRef<HTMLInputElement>;

  isRenaming = false;
  editingName = '';
  isParent = false;
  isSymlinkEntry = false;
  displayName = '';
  fullName = ''; // Store original name for tooltip
  mimeColorKind = 'default';
  iconEmoji = '📄';
  searchParentPath = '';
  showSearchBadgeInVariant = false;
  showSearchContextRow = false;
  showSearchContextGrid = false;
  showDirectorySizeSpinner = false;
  private resolvedCategoryId: string | null | undefined;
  private unwatchResolvedCategory: (() => void) | null = null;

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

  constructor(
    private readonly categoryResolver: FileCategoryResolverService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(_changes: SimpleChanges): void {
    this.bindResolvedCategory();
    this.rebuildRenderState();
  }

  ngOnDestroy(): void {
    this.unwatchResolvedCategory?.();
  }

  isParentEntry(): boolean {
    return this.isParent;
  }

  /** True for file-symlinks (type === SYMLINK) and dir-symlinks (type === DIRECTORY, isSymlink === true). */
  isSymlink(): boolean {
    return this.isSymlinkEntry;
  }

  isSearchEntry(): boolean {
    return !!this.entry && !!(this.entry.searchParentPath || this.entry.searchPattern || this.entry.searchBasePath);
  }

  showSearchBadge(): boolean {
    return this.showSearchBadgeInVariant;
  }

  showSearchContextInRow(): boolean {
    return this.showSearchContextRow;
  }

  showSearchContextInGrid(): boolean {
    return this.showSearchContextGrid;
  }

  shouldShowDirectorySizeSpinner(): boolean {
    return this.showDirectorySizeSpinner;
  }

  getSearchParentPath(): string {
    return this.searchParentPath;
  }

  /**
   * Base emoji for the icon.
   * Symlinks show the icon of their target kind — the cyan badge
   * is the only indicator that distinguishes them from real entries.
   */
  getIconEmoji(): string {
    return this.iconEmoji;
  }

  private rebuildRenderState(): void {
    const entry = this.entry;
    const presentation = getFileEntryPresentation(entry, this.colorizeByMime, this.resolvedCategoryId ?? null);
    this.isParent = presentation.isParent;
    this.isSymlinkEntry = presentation.isSymlink;
    
    let displayName = this.displayNameOverride?.trim() || entry?.name || '';
    this.fullName = displayName; // Store original for tooltip
    
    // Apply smart truncation based on variant
    if (displayName && !this.isParent) {
      const maxLengthByVariant = this.variant === 'row' ? 85 : this.variant === 'inline' ? 65 : 55;
      displayName = truncateFileName(displayName, maxLengthByVariant);
    }
    this.displayName = displayName;
    
    this.mimeColorKind = presentation.mimeColorKind;
    this.searchParentPath = entry?.searchParentPath || '';
    const isSearchEntry = !!entry && !!(entry.searchParentPath || entry.searchPattern || entry.searchBasePath);
    this.showSearchBadgeInVariant = !this.isParent && this.variant !== 'inline' && isSearchEntry;
    this.showSearchContextRow = !this.isParent && this.variant === 'row' && isSearchEntry;
    this.showSearchContextGrid = !this.isParent && this.variant === 'grid' && isSearchEntry;
    this.showDirectorySizeSpinner = this.directorySizePending && !this.isParent && entry?.type === FileType.DIRECTORY;
    this.iconEmoji = presentation.iconEmoji;
  }

  private bindResolvedCategory(): void {
    this.unwatchResolvedCategory?.();
    this.unwatchResolvedCategory = null;
    this.resolvedCategoryId = undefined;

    const path = this.entry?.path;
    if (!path || !this.colorizeByMime) return;

    this.resolvedCategoryId = this.categoryResolver.getResolvedCategory(path);
    this.unwatchResolvedCategory = this.categoryResolver.watch(path, () => {
      this.resolvedCategoryId = this.categoryResolver.getResolvedCategory(path);
      this.rebuildRenderState();
      this.cdr.markForCheck();
    });
    this.categoryResolver.ensureResolved(this.entry);
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

  formatSize(bytes: number): string {
    if (bytes === 0) return '';
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
