import { ChangeDetectionStrategy, ChangeDetectorRef, Component, EventEmitter, Input, OnChanges, OnDestroy, Output, SimpleChanges } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';
import { getFileEntryPresentation } from './file-entry-presentation.util';
import { FileCategoryResolverService } from '../services/file-category-resolver.service';

@Component({
  selector: 'app-file-entry-inline',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="inline-entry" [class.parent-dir]="isParent" (click)="select.emit($event)" (dblclick)="navigate.emit()">
      <span class="file-icon" [class.parent-icon]="isParent">
        <ng-container *ngIf="isParent; else normalIcon">
          <svg
            width="16"
            height="16"
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
          <span class="icon-emoji">{{ iconEmoji }}</span>
          <svg *ngIf="isSymlink" class="symlink-badge" viewBox="0 0 10 10" fill="none" aria-label="symlink">
            <path d="M2 8 C2 5 5 3 8 2"
              stroke="var(--vsc-icon-symlink,#5dade2)" stroke-width="2" stroke-linecap="round"/>
            <path d="M6 1.2 L8.8 1.2 L8.8 4"
              stroke="var(--vsc-icon-symlink,#5dade2)" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </ng-template>
      </span>

      <div class="file-name">
        <span class="file-name-text" [attr.data-mime-kind]="mimeColorKind">{{ displayName }}</span>
        <span
          *ngIf="showDirectorySizeSpinnerInName && showDirectorySizeSpinner"
          class="file-size-spinner"
          aria-label="Calculating folder size"
          title="Calculating folder size">
        </span>
      </div>
    </div>
  `,
  styles: [`
    .inline-entry {
      display: inline-flex;
      align-items: center;
      min-width: 0;
      width: 100%;
      color: inherit;
      pointer-events: none;
    }

    .inline-entry.parent-dir .file-name-text {
      font-style: italic;
    }

    .file-icon {
      margin-right: var(--vsc-padding-sm);
      font-size: 14px;
      flex-shrink: 0;
      position: relative;
      width: 16px;
      height: 16px;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      --vsc-icon-symlink: color-mix(in srgb, var(--vsc-accent-blue, #4fc3f7) 80%, white 20%);
    }

    .parent-icon svg {
      width: 100%;
      height: 100%;
      display: block;
    }

    .icon-emoji {
      font-size: 13px;
      line-height: 1;
      display: block;
    }

    .symlink-badge {
      position: absolute;
      right: -1px;
      bottom: -1px;
      width: 8px;
      height: 8px;
      pointer-events: none;
      filter:
        drop-shadow(0 0 1.5px color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 90%, transparent))
        drop-shadow(0 0 2.5px color-mix(in srgb, var(--vsc-icon-symlink, #4fc3f7) 55%, transparent));
    }

    .file-name {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      min-width: 0;
      width: 100%;
      overflow: hidden;
    }

    .file-name-text {
      overflow: hidden;
      text-overflow: ellipsis;
      color: var(--entry-name-color, inherit);
      white-space: nowrap;
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

    .file-size-spinner {
      width: 11px;
      height: 11px;
      border-radius: 999px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue) 22%, transparent);
      border-top-color: var(--vsc-accent-blue);
      box-sizing: border-box;
      animation: inline-size-spin 0.9s linear infinite;
      flex-shrink: 0;
    }

    @keyframes inline-size-spin {
      from { transform: rotate(0deg); }
      to { transform: rotate(360deg); }
    }
  `]
})
export class FileEntryInlineComponent implements OnChanges, OnDestroy {
  @Input({ required: true }) entry!: FileInfo;
  @Input() displayNameOverride: string | null = null;
  @Input() colorizeByMime = false;
  @Input() directorySizePending = false;
  @Input() showDirectorySizeSpinnerInName = false;

  @Output() select = new EventEmitter<MouseEvent>();
  @Output() navigate = new EventEmitter<void>();

  private resolvedCategoryId: string | null | undefined;
  private unwatchResolvedCategory: (() => void) | null = null;

  constructor(
    private readonly categoryResolver: FileCategoryResolverService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(_changes: SimpleChanges): void {
    this.bindResolvedCategory();
  }

  ngOnDestroy(): void {
    this.unwatchResolvedCategory?.();
  }

  get isParent(): boolean {
    return getFileEntryPresentation(this.entry, this.colorizeByMime, this.resolvedCategoryId ?? null).isParent;
  }

  get isSymlink(): boolean {
    return getFileEntryPresentation(this.entry, this.colorizeByMime, this.resolvedCategoryId ?? null).isSymlink;
  }

  get displayName(): string {
    return this.displayNameOverride?.trim() || this.entry?.name || '';
  }

  get mimeColorKind(): string {
    return getFileEntryPresentation(this.entry, this.colorizeByMime, this.resolvedCategoryId ?? null).mimeColorKind;
  }

  get showDirectorySizeSpinner(): boolean {
    return this.directorySizePending && !this.isParent && this.entry?.type === FileType.DIRECTORY;
  }

  get iconEmoji(): string {
    return getFileEntryPresentation(this.entry, this.colorizeByMime, this.resolvedCategoryId ?? null).iconEmoji;
  }

  private bindResolvedCategory(): void {
    this.unwatchResolvedCategory?.();
    this.unwatchResolvedCategory = null;
    this.resolvedCategoryId = undefined;

    const path = this.entry?.path;
    if (!path) return;

    this.resolvedCategoryId = this.categoryResolver.getResolvedCategory(path);
    this.unwatchResolvedCategory = this.categoryResolver.watch(path, () => {
      this.resolvedCategoryId = this.categoryResolver.getResolvedCategory(path);
      this.cdr.markForCheck();
    });
    this.categoryResolver.ensureResolved(this.entry);
  }
}
