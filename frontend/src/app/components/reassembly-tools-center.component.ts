import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  EventEmitter,
  OnDestroy,
  OnInit,
  Output,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

import { ApiService } from '../services/api.service';
import {
  DuplicateFinderService,
  type DuplicateGroup,
  type DuplicateScanProgress,
  type DuplicateScanResult,
} from '../services/duplicate-finder.service';
import {
  OperationService,
  type OperationHistoryEntry,
  type OperationState,
} from '../services/operation.service';
import { SelectionBasketService } from '../services/selection-basket.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import type {
  DiskUsageEntry,
  DiskUsageProgress,
  DiskUsageResult,
} from '../shared/protocol-enhanced';

type ToolPage = 'operations' | 'disk' | 'duplicates';

@Component({
  selector: 'app-reassembly-tools-center',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop" *ngIf="visible" (mousedown)="close()">
      <section class="tools" role="dialog" aria-modal="true" aria-label="File manager tools"
        (mousedown)="$event.stopPropagation()">
        <header>
          <div>
            <strong>Tools</strong>
            <span class="root" [title]="rootPath">{{ rootPath }}</span>
          </div>
          <button type="button" class="close" (click)="close()">×</button>
        </header>

        <nav>
          <button type="button" [class.active]="page === 'operations'" (click)="page='operations'">
            Operations <span>{{ activeOperations.length }}</span>
          </button>
          <button type="button" [class.active]="page === 'disk'" (click)="page='disk'">Disk Usage</button>
          <button type="button" [class.active]="page === 'duplicates'" (click)="page='duplicates'">Duplicates</button>
        </nav>

        <div class="body" *ngIf="page === 'operations'">
          <div class="summary">
            <span>{{ activeOperations.length }} active</span>
            <span>{{ history.length }} recent</span>
          </div>

          <article class="operation" *ngFor="let operation of activeOperations; trackBy:trackOperation">
            <div class="row">
              <strong>{{ operation.config.title }}</strong>
              <span>{{ operation.status }}</span>
            </div>
            <div class="progress">
              <i [style.width.%]="operation.metrics.totalBytes > 0 ? operation.percentComplete : 0"></i>
            </div>
            <small *ngIf="operation.metrics.currentFile">{{ operation.metrics.currentFile }}</small>
          </article>

          <div class="history-title" *ngIf="history.length">Recent</div>
          <article class="operation history" *ngFor="let operation of history; trackBy:trackOperation">
            <div class="row">
              <strong>{{ operation.config.title }}</strong>
              <span>{{ operation.status }}</span>
            </div>
            <small>{{ operation.completedAt | date:'short' }}</small>
          </article>

          <div class="empty" *ngIf="activeOperations.length === 0 && history.length === 0">
            No file operations yet.
          </div>
        </div>

        <div class="body" *ngIf="page === 'disk'">
          <div class="tool-actions">
            <button type="button" (click)="scanDisk()" [disabled]="diskScanning || !rootPath">
              {{ diskScanning ? 'Scanning…' : 'Scan current directory' }}
            </button>
            <button type="button" *ngIf="diskScanning" (click)="cancelDisk()">Cancel</button>
          </div>

          <div class="summary" *ngIf="diskProgress">
            <span>{{ formatBytes(diskProgress.totalSize || 0) }}</span>
            <span>{{ diskProgress.totalFiles || 0 }} files</span>
            <span>{{ diskProgress.totalDirs || 0 }} dirs</span>
          </div>

          <div class="disk-list" *ngIf="diskEntries.length">
            <article class="disk-row" *ngFor="let entry of diskEntries; trackBy:trackDisk">
              <span class="name" [title]="entry.path">{{ entry.name }}</span>
              <span>{{ formatBytes(entry.size) }}</span>
              <span>{{ entry.files }} files</span>
              <div class="bar"><i [style.width.%]="diskPercent(entry)"></i></div>
            </article>
          </div>

          <div class="empty" *ngIf="!diskScanning && diskEntries.length === 0">
            Scan the active directory to see where space is used.
          </div>
          <div class="error" *ngIf="diskError">{{ diskError }}</div>
        </div>

        <div class="body" *ngIf="page === 'duplicates'">
          <div class="tool-actions">
            <label>
              Min size
              <input type="number" min="0" [(ngModel)]="duplicateMinSize" [disabled]="duplicateScanning">
            </label>
            <label>
              <input type="checkbox" [(ngModel)]="duplicateIncludeHidden" [disabled]="duplicateScanning">
              Hidden
            </label>
            <button type="button" (click)="scanDuplicates()" [disabled]="duplicateScanning || !rootPath">
              {{ duplicateScanning ? 'Scanning…' : 'Scan' }}
            </button>
            <button type="button" [disabled]="duplicateSelected.size === 0" (click)="collectDuplicateSelection()">
              Add to Basket
            </button>
            <button type="button" class="danger" [disabled]="duplicateSelected.size === 0" (click)="deleteDuplicateSelection()">
              Delete Selected
            </button>
          </div>

          <div class="summary" *ngIf="duplicateProgress">
            <span>{{ duplicateProgress.phase }}</span>
            <span>{{ duplicateProgress.discoveredFiles }} discovered</span>
            <span>{{ duplicateProgress.hashedFiles }} hashed</span>
          </div>

          <div class="summary" *ngIf="duplicateResult">
            <span>{{ duplicateResult.groups.length }} groups</span>
            <span>{{ duplicateResult.scannedFiles }} files</span>
            <span>{{ formatBytes(duplicateResult.reclaimableBytes) }} reclaimable</span>
          </div>

          <section class="duplicate-group" *ngFor="let group of duplicateGroups; trackBy:trackGroup">
            <header>
              <strong>{{ group.files.length }} copies</strong>
              <span>{{ formatBytes(group.size) }} each</span>
              <span>{{ formatBytes(group.reclaimableBytes) }} reclaimable</span>
            </header>

            <label class="duplicate-file" *ngFor="let file of group.files">
              <input
                type="checkbox"
                [checked]="duplicateSelected.has(file.path)"
                [disabled]="file.path === group.suggestedKeeperPath"
                (change)="toggleDuplicate(file.path)">
              <span class="path" [title]="file.path">{{ file.path }}</span>
              <small *ngIf="file.path === group.suggestedKeeperPath">keeper</small>
            </label>
          </section>

          <div class="empty" *ngIf="!duplicateScanning && duplicateGroups.length === 0">
            Scan recursively to find content-identical files.
          </div>

          <div class="warnings" *ngIf="duplicateResult && duplicateResult.warnings.length > 0">
            {{ duplicateResult.warnings.length }} paths could not be scanned.
          </div>
          <div class="error" *ngIf="duplicateError">{{ duplicateError }}</div>
        </div>
      </section>
    </div>
  `,
  styles: [`
    :host{position:fixed;inset:0;z-index:9490;pointer-events:none}
    .backdrop{position:fixed;inset:0;background:rgba(0,0,0,.28);display:grid;place-items:center;pointer-events:auto}
    .tools{width:min(900px,calc(100vw - 30px));height:min(760px,calc(100vh - 30px));display:flex;flex-direction:column;overflow:hidden;border:1px solid var(--vsc-border,#454545);border-radius:8px;background:var(--vsc-panel-background,#1e1e1e);color:var(--vsc-foreground,#ccc);box-shadow:0 18px 70px rgba(0,0,0,.5);font:12px/1.4 var(--vsc-font-family,system-ui,sans-serif)}
    header{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:10px 12px;border-bottom:1px solid var(--vsc-border,#333)}
    header>div{min-width:0;display:flex;gap:10px;align-items:baseline}.root{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--vsc-description-foreground,#969696);font-family:var(--vsc-font-family-mono,ui-monospace,monospace)}
    button,input{font:inherit}.close{border:0;background:transparent;color:inherit;font-size:20px;cursor:pointer}
    nav{display:flex;gap:2px;padding:5px;border-bottom:1px solid var(--vsc-border,#333)}
    nav button,.tool-actions button{border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;padding:5px 9px;cursor:pointer}
    nav button.active,.tool-actions button:hover:not(:disabled){background:var(--vsc-list-hover-background,#2a2d2e);border-color:var(--vsc-border,#454545)}
    nav span{opacity:.6;margin-left:4px}.body{flex:1;min-height:0;overflow:auto}.summary,.tool-actions{display:flex;align-items:center;gap:12px;flex-wrap:wrap;padding:8px 12px;border-bottom:1px solid var(--vsc-border-subtle,#2b2b2b)}
    .tool-actions label{display:flex;align-items:center;gap:5px}.tool-actions input[type=number]{width:90px;background:var(--vsc-input-background,#181818);color:inherit;border:1px solid var(--vsc-border,#454545);padding:3px 5px}
    .danger{color:var(--vsc-error-foreground,#f48771)!important}.operation{padding:9px 12px;border-bottom:1px solid var(--vsc-border-subtle,#2b2b2b)}.row{display:flex;justify-content:space-between;gap:10px}.operation small{display:block;margin-top:4px;color:var(--vsc-description-foreground,#969696);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .progress,.bar{height:4px;margin-top:6px;background:rgba(127,127,127,.18);overflow:hidden;border-radius:99px}.progress i,.bar i{display:block;height:100%;background:var(--vsc-accent-blue,#007acc)}
    .history-title{padding:8px 12px;font-weight:600;color:var(--vsc-description-foreground,#969696)}.operation.history{opacity:.8}
    .disk-row{display:grid;grid-template-columns:minmax(140px,1fr) 100px 80px minmax(100px,220px);gap:10px;align-items:center;padding:7px 12px;border-bottom:1px solid var(--vsc-border-subtle,#2b2b2b)}.disk-row .name,.path{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .duplicate-group{margin:8px 12px;border:1px solid var(--vsc-border,#3a3a3a);border-radius:5px;overflow:hidden}.duplicate-group>header{padding:7px 9px;background:rgba(127,127,127,.07);font-size:11px}.duplicate-file{display:grid;grid-template-columns:22px minmax(0,1fr) auto;gap:7px;align-items:center;padding:6px 9px;border-top:1px solid rgba(127,127,127,.1)}.duplicate-file small{color:var(--vsc-accent-green,#4ec9b0)}
    .empty{padding:48px 20px;text-align:center;color:var(--vsc-description-foreground,#969696)}.error,.warnings{margin:8px 12px;padding:8px;border-radius:4px}.error{color:var(--vsc-error-foreground,#f48771);background:rgba(244,135,113,.08)}.warnings{color:#d7ba7d;background:rgba(215,186,125,.07)}
    button:disabled{opacity:.4;cursor:default}
  `],
})
export class ReassemblyToolsCenterComponent implements OnInit, OnDestroy {
  @Output() changed = new EventEmitter<void>();

  visible = false;
  page: ToolPage = 'operations';
  rootPath = '';

  activeOperations: OperationState[] = [];
  history: OperationHistoryEntry[] = [];

  diskScanning = false;
  diskProgress: DiskUsageProgress | DiskUsageResult | null = null;
  diskEntries: DiskUsageEntry[] = [];
  diskError = '';
  private diskAbort: AbortController | null = null;

  duplicateMinSize = 1;
  duplicateIncludeHidden = false;
  duplicateScanning = false;
  duplicateProgress: DuplicateScanProgress | null = null;
  duplicateResult: DuplicateScanResult | null = null;
  duplicateSelected = new Set<string>();
  duplicateError = '';

  private readonly destroy$ = createSubject<void>();

  constructor(
    private readonly api: ApiService,
    private readonly duplicates: DuplicateFinderService,
    private readonly operations: OperationService,
    private readonly basket: SelectionBasketService,
    private readonly confirm: ConfirmDialogService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get duplicateGroups(): DuplicateGroup[] {
    return [...(this.duplicateResult?.groups ?? [])].sort(
      (a,b) => b.reclaimableBytes - a.reclaimableBytes
    );
  }

  ngOnInit(): void {
    this.operations.visibleOperations$
      .pipe(takeUntil(this.destroy$))
      .subscribe(items => {
        this.activeOperations = items;
        this.cdr.markForCheck();
      });

    this.operations.history$
      .pipe(takeUntil(this.destroy$))
      .subscribe(items => {
        this.history = [...items].slice(-50).reverse();
        this.cdr.markForCheck();
      });
  }

  ngOnDestroy(): void {
    this.cancelDisk();
    this.destroy$.next();
    this.destroy$.complete();
  }

  open(page: ToolPage, rootPath: string): void {
    this.page = page;
    this.rootPath = rootPath;
    this.visible = true;
    this.cdr.markForCheck();

    if (page === 'disk' && this.diskEntries.length === 0) {
      void this.scanDisk();
    }
  }

  close(): void {
    this.visible = false;
    this.cdr.markForCheck();
  }

  async scanDisk(): Promise<void> {
    if (!this.rootPath || this.diskScanning) return;

    this.cancelDisk();
    this.diskAbort = new AbortController();
    this.diskScanning = true;
    this.diskError = '';
    this.diskEntries = [];
    this.diskProgress = null;
    this.cdr.markForCheck();

    try {
      for await (const item of this.api.streamDiskUsage(
        this.rootPath,
        { signal: this.diskAbort.signal },
      )) {
        if (this.isDiskResult(item)) {
          this.diskProgress = item;
          this.diskEntries = [...(item.entries ?? [])]
            .sort((a,b) => (b.size ?? 0) - (a.size ?? 0));
        } else if (this.isDiskEntry(item)) {
          const index = this.diskEntries.findIndex(entry => entry.path === item.path);
          if (index >= 0) {
            const next = [...this.diskEntries];
            next[index] = item;
            this.diskEntries = next;
          } else {
            this.diskEntries = [...this.diskEntries, item];
          }
          this.diskEntries.sort((a,b) => (b.size ?? 0) - (a.size ?? 0));
        } else {
          this.diskProgress = item;
        }
        this.cdr.markForCheck();
      }
    } catch (error) {
      if (!this.diskAbort?.signal.aborted) {
        this.diskError = error instanceof Error ? error.message : String(error);
      }
    } finally {
      this.diskScanning = false;
      this.diskAbort = null;
      this.cdr.markForCheck();
    }
  }

  cancelDisk(): void {
    this.diskAbort?.abort();
    this.diskAbort = null;
    this.diskScanning = false;
  }

  diskPercent(entry: DiskUsageEntry): number {
    const total = Number((this.diskProgress as DiskUsageResult | null)?.totalSize ?? 0);
    return total > 0 ? Math.min(100, (Number(entry.size || 0) / total) * 100) : 0;
  }

  async scanDuplicates(): Promise<void> {
    if (!this.rootPath || this.duplicateScanning) return;

    this.duplicateScanning = true;
    this.duplicateProgress = null;
    this.duplicateResult = null;
    this.duplicateSelected = new Set();
    this.duplicateError = '';
    this.cdr.markForCheck();

    try {
      this.duplicateResult = await this.duplicates.scan(
        this.rootPath,
        {
          includeHidden: this.duplicateIncludeHidden,
          minSize: Math.max(0, Number(this.duplicateMinSize) || 0),
        },
        progress => {
          this.duplicateProgress = progress;
          this.cdr.markForCheck();
        },
      );

      // Preselect every duplicate except the service's suggested keeper.
      const selected = new Set<string>();
      for (const group of this.duplicateResult.groups) {
        for (const file of group.files) {
          if (file.path !== group.suggestedKeeperPath) selected.add(file.path);
        }
      }
      this.duplicateSelected = selected;
    } catch (error) {
      this.duplicateError = error instanceof Error ? error.message : String(error);
    } finally {
      this.duplicateScanning = false;
      this.cdr.markForCheck();
    }
  }

  toggleDuplicate(path: string): void {
    const next = new Set(this.duplicateSelected);
    next.has(path) ? next.delete(path) : next.add(path);
    this.duplicateSelected = next;
    this.cdr.markForCheck();
  }

  collectDuplicateSelection(): void {
    if (this.duplicateSelected.size === 0) return;
    this.basket.add(Array.from(this.duplicateSelected), 'duplicates');
  }

  async deleteDuplicateSelection(): Promise<void> {
    const paths = Array.from(this.duplicateSelected);
    if (paths.length === 0) return;

    const confirmed = await this.confirm.confirm({
      title: 'Delete Duplicate Files',
      message:
        `Delete ${paths.length} selected duplicate file${
          paths.length === 1 ? '' : 's'
        }? The suggested keeper in each group is protected from selection.`,
      confirmText: 'Delete',
      danger: true,
    });
    if (!confirmed) return;

    try {
      const result = await this.api.batchDelete(paths, true, true);

      if (result.failed === 0) {
        this.duplicateSelected = new Set();
        await this.scanDuplicates();
      } else {
        await this.confirm.alert({
          title: 'Duplicate Delete Completed with Errors',
          message:
            `${result.failed} file${result.failed === 1 ? '' : 's'} could not be deleted. ` +
            'The result list has been left intact for review.',
        });
      }

      this.changed.emit();
    } catch (error) {
      await this.confirm.alert({
        title: 'Duplicate Delete Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }

    this.cdr.markForCheck();
  }

  trackOperation(_index:number, item:OperationState):string { return item.id; }
  trackDisk(_index:number, item:DiskUsageEntry):string { return item.path; }
  trackGroup(_index:number, item:DuplicateGroup):string { return item.hash; }

  formatBytes(bytes:number):string {
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
    const units=['B','KB','MB','GB','TB'];
    const i=Math.min(units.length-1,Math.floor(Math.log(bytes)/Math.log(1024)));
    const value=bytes/Math.pow(1024,i);
    return `${value>=10||i===0?value.toFixed(0):value.toFixed(1)} ${units[i]}`;
  }

  private isDiskResult(item: DiskUsageEntry | DiskUsageProgress | DiskUsageResult): item is DiskUsageResult {
    return 'entries' in item && 'totalSize' in item;
  }

  private isDiskEntry(item: DiskUsageEntry | DiskUsageProgress | DiskUsageResult): item is DiskUsageEntry {
    return 'path' in item && 'size' in item && 'isDir' in item && !('entries' in item);
  }
}
