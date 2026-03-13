import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  Input,
  OnDestroy,
  OnInit,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { ApiService } from '../../services/api.service';
import { FileType } from '../../shared/protocol';
import { SizeManagerSurfaceContext } from '../surface-context';

interface SizeEntry {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  loading: boolean;
}

function formatSize(bytes: number): string {
  if (bytes >= 1_073_741_824) return (bytes / 1_073_741_824).toFixed(2) + ' GB';
  if (bytes >= 1_048_576)     return (bytes / 1_048_576).toFixed(2)     + ' MB';
  if (bytes >= 1_024)         return (bytes / 1_024).toFixed(1)         + ' KB';
  return bytes + ' B';
}

@Component({
  selector: 'app-size-manager',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="sm-root">
      <div class="sm-header">
        <span class="sm-title">Size Analysis</span>
        <span class="sm-path">{{ ctx.sourcePath }}</span>
      </div>

      <div class="sm-list" *ngIf="entries.length > 0; else empty">
        <div
          class="sm-row"
          *ngFor="let entry of entries"
          [class.sm-row--dir]="entry.isDir"
        >
          <span class="sm-icon">
            <svg *ngIf="entry.isDir" viewBox="0 0 16 16" width="14" height="14" fill="var(--vscode-icon-foreground, #c5c5c5)">
              <path d="M14.5 3H7.71l-.85-.85-.15-.15H1.5l-.5.5v10l.5.5h13l.5-.5v-9l-.5-.5z"/>
            </svg>
            <svg *ngIf="!entry.isDir" viewBox="0 0 16 16" width="14" height="14" fill="var(--vscode-icon-foreground, #c5c5c5)">
              <path d="M13.71 4.29l-3-3L10 1H4L3 2v12l1 1h9l1-1V5l-.29-.71zM13 14H4V2h5v4h4v8zm-3-9V2l3 3h-3z"/>
            </svg>
          </span>
          <span class="sm-name" [title]="entry.name">{{ entry.name }}</span>
          <div class="sm-bar-track">
            <div
              class="sm-bar-fill"
              [class.sm-bar-fill--dir]="entry.isDir"
              [style.width.%]="maxSize > 0 ? (entry.size / maxSize) * 100 : 0"
            ></div>
          </div>
          <span class="sm-size" [class.sm-size--loading]="entry.loading">
            <span class="sm-spinner" *ngIf="entry.loading"></span>
            {{ entry.loading ? '…' : formatSize(entry.size) }}
          </span>
        </div>
      </div>

      <ng-template #empty>
        <div class="sm-empty" *ngIf="!loading; else spinner">
          Directory is empty.
        </div>
        <ng-template #spinner>
          <div class="sm-empty">Loading…</div>
        </ng-template>
      </ng-template>
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
      background: var(--vscode-editor-background, #1e1e1e);
      color: var(--vscode-editor-foreground, #d4d4d4);
      font-family: var(--vscode-font-family, monospace);
      font-size: 12px;
    }

    .sm-root {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
      padding: 8px 0;
    }

    .sm-header {
      display: flex;
      flex-direction: column;
      gap: 2px;
      padding: 0 12px 8px;
      border-bottom: 1px solid var(--vscode-panel-border, #3c3c3c);
      flex-shrink: 0;
    }

    .sm-title {
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--vscode-sideBarTitle-foreground, #bbb);
    }

    .sm-path {
      font-size: 11px;
      color: var(--vscode-descriptionForeground, #858585);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .sm-list {
      flex: 1 1 0%;
      overflow-y: auto;
      overflow-x: hidden;
    }

    .sm-row {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 3px 12px;
      min-height: 22px;
      cursor: default;
      user-select: none;

      &:hover {
        background: var(--vscode-list-hoverBackground, rgba(255,255,255,.06));
      }
    }

    .sm-icon {
      flex-shrink: 0;
      display: flex;
      align-items: center;
      opacity: 0.8;
    }

    .sm-name {
      flex: 0 0 180px;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .sm-row--dir .sm-name {
      color: var(--vscode-symbolIcon-folderForeground, #dcb67a);
    }

    .sm-bar-track {
      flex: 1 1 0%;
      height: 8px;
      background: var(--vscode-scrollbarSlider-background, rgba(121,121,121,.2));
      border-radius: 2px;
      overflow: hidden;
      min-width: 40px;
    }

    .sm-bar-fill {
      height: 100%;
      background: var(--vscode-progressBar-background, #0e70c0);
      border-radius: 2px;
      transition: width 200ms ease;
    }

    .sm-bar-fill--dir {
      background: var(--vscode-symbolIcon-folderForeground, #dcb67a);
    }

    .sm-size {
      flex: 0 0 72px;
      text-align: right;
      font-variant-numeric: tabular-nums;
      color: var(--vscode-descriptionForeground, #858585);
      white-space: nowrap;
    }

    .sm-size--loading {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 4px;
    }

    .sm-spinner {
      display: inline-block;
      width: 8px;
      height: 8px;
      border: 1.5px solid var(--vscode-progressBar-background, #0e70c0);
      border-top-color: transparent;
      border-radius: 50%;
      animation: spin 0.6s linear infinite;
      flex-shrink: 0;
    }

    @keyframes spin {
      to { transform: rotate(360deg); }
    }

    .sm-empty {
      padding: 24px 12px;
      color: var(--vscode-descriptionForeground, #858585);
      text-align: center;
    }
  `],
})
export class SizeManagerComponent implements OnInit, OnDestroy {
  @Input({ required: true }) ctx!: SizeManagerSurfaceContext;

  entries: SizeEntry[] = [];
  maxSize = 0;
  loading = true;

  private destroyed = false;

  readonly formatSize = formatSize;

  constructor(
    private readonly api: ApiService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    void this.load();
  }

  ngOnDestroy(): void {
    this.destroyed = true;
  }

  private async load(): Promise<void> {
    this.loading = true;
    this.entries = [];
    this.maxSize = 0;

    let listing;
    try {
      listing = await this.api.listDirectory(this.ctx.sourcePath, true);
    } catch {
      this.loading = false;
      this.cdr.markForCheck();
      return;
    }

    if (this.destroyed) return;

    // Build initial entries; files have size immediately, dirs start at 0.
    this.entries = (listing.entries ?? [])
      .filter(e => e.type !== FileType.SYMLINK)
      .map((e): SizeEntry => ({
        name: e.name,
        path: e.path,
        isDir: e.type === FileType.DIRECTORY || e.type === FileType.ARCHIVE,
        size: e.type === FileType.FILE ? (e.size ?? 0) : 0,
        loading: e.type === FileType.DIRECTORY,
      }));

    this.recomputeMaxAndSort();
    this.loading = false;
    this.cdr.markForCheck();

    // Fire recursive size requests for directories in parallel.
    const dirEntries = this.entries.filter(e => e.isDir && e.loading);
    await Promise.all(
      dirEntries.map(async (entry) => {
        try {
          const size = await window.go.main.App.DirSizeRecursive(entry.path);
          if (this.destroyed) return;
          entry.size = size ?? 0;
          entry.loading = false;
          this.recomputeMaxAndSort();
          this.cdr.markForCheck();
        } catch {
          if (this.destroyed) return;
          entry.loading = false;
          this.cdr.markForCheck();
        }
      }),
    );
  }

  private recomputeMaxAndSort(): void {
    // Keep dirs before files, then sort each group by size desc.
    this.entries.sort((a, b) => {
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
      return b.size - a.size;
    });
    this.maxSize = this.entries.reduce((m, e) => (e.size > m ? e.size : m), 0);
  }
}
