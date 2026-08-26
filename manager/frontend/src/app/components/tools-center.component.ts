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
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import type {
  DiskUsageEntry,
  DiskUsageProgress,
  DiskUsageResult,
} from '../shared/protocol-enhanced';
type ToolPage = 'operations' | 'disk' | 'duplicates';
type OperationFilter = 'all' | 'active' | 'completed' | 'failed';
type DiskSort = 'size' | 'files' | 'name';
type DuplicateSort = 'reclaimable' | 'size' | 'copies';

const CONTEXT_ID = 'overlay.Tools';
const OWNER_ID = 'overlay.Tools';

const COMMAND = {
  close: 'reassembly.toolsCenter.close',
  operations: 'reassembly.toolsCenter.operations',
  disk: 'reassembly.toolsCenter.disk',
  duplicates: 'reassembly.toolsCenter.duplicates',
  rescan: 'reassembly.toolsCenter.rescan',
  selectAllDuplicates: 'reassembly.toolsCenter.selectAllDuplicates',
  clearDuplicateSelection: 'reassembly.toolsCenter.clearDuplicateSelection',
  addDuplicatesToBasket: 'reassembly.toolsCenter.addDuplicatesToBasket',
  deleteDuplicates: 'reassembly.toolsCenter.deleteDuplicates',
} as const;

const BINDINGS: readonly InputBindingDefinition[] = [
  { id:'reassembly.toolsCenter.escape', commandId:COMMAND.close, contextId:CONTEXT_ID, source:'default', description:'Close Tools Center', category:'Tools', trigger:{device:'keyboard', chord:{steps:[{key:'escape'}]}}},
  { id:'reassembly.toolsCenter.page1', commandId:COMMAND.operations, contextId:CONTEXT_ID, source:'default', description:'Operations page', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'1'}]}}},
  { id:'reassembly.toolsCenter.page2', commandId:COMMAND.disk, contextId:CONTEXT_ID, source:'default', description:'Disk Usage page', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'2'}]}}},
  { id:'reassembly.toolsCenter.page3', commandId:COMMAND.duplicates, contextId:CONTEXT_ID, source:'default', description:'Duplicates page', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'3'}]}}},
  { id:'reassembly.toolsCenter.rescan', commandId:COMMAND.rescan, contextId:CONTEXT_ID, source:'default', description:'Rescan current tool', category:'Tools', trigger:{device:'keyboard', chord:{steps:[{key:'r'}]}}},
  { id:'reassembly.toolsCenter.selectAllDuplicates', commandId:COMMAND.selectAllDuplicates, contextId:CONTEXT_ID, source:'default', description:'Select reclaimable duplicates', category:'Tools', trigger:{device:'keyboard', chord:{steps:[{key:'a'}]}}},
  { id:'reassembly.toolsCenter.clearDuplicateSelection', commandId:COMMAND.clearDuplicateSelection, contextId:CONTEXT_ID, source:'default', description:'Clear duplicate selection', category:'Tools', trigger:{device:'keyboard', chord:{steps:[{key:'u'}]}}},
  { id:'reassembly.toolsCenter.addDuplicatesToBasket', commandId:COMMAND.addDuplicatesToBasket, contextId:CONTEXT_ID, source:'default', description:'Add duplicate selection to basket', category:'Tools', trigger:{device:'keyboard', chord:{steps:[{key:'b'}]}}},
  { id:'reassembly.toolsCenter.deleteDuplicates', commandId:COMMAND.deleteDuplicates, contextId:CONTEXT_ID, source:'default', description:'Delete selected duplicates', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'delete'}]}}},
];

@Component({
  selector: 'app-tools-center',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop palette-dialog-backdrop dialog-group" *ngIf="visible" (mousedown)="close()">
      <section class="tools palette-dialog-surface dialog-surface" role="dialog" aria-modal="true" aria-label="File manager tools"
        (mousedown)="$event.stopPropagation()"
        tabindex="-1">
        <header class="palette-dialog-header">
          <div class="palette-dialog-title">
            <strong>Tools</strong>
            <span class="root palette-dialog-subtitle palette-dialog-mono" [title]="rootPath">{{ rootPath }}</span>
          </div>
          <button type="button" class="dialog-close-btn" aria-label="Close" title="Close" (click)="close()">×</button>
        </header>

        <nav class="palette-dialog-tabbar" role="tablist" aria-label="Tool pages">
          <button type="button" role="tab" [attr.aria-selected]="page === 'operations'" [attr.tabindex]="page === 'operations' ? 0 : -1" [class.active]="page === 'operations'" (click)="page='operations'">
            Operations <span>{{ activeOperations.length }}</span>
          </button>
          <button type="button" role="tab" [attr.aria-selected]="page === 'disk'" [attr.tabindex]="page === 'disk' ? 0 : -1" [class.active]="page === 'disk'" (click)="page='disk'">Disk Usage</button>
          <button type="button" role="tab" [attr.aria-selected]="page === 'duplicates'" [attr.tabindex]="page === 'duplicates' ? 0 : -1" [class.active]="page === 'duplicates'" (click)="page='duplicates'">Duplicates</button>
        </nav>

        <div class="body" role="tabpanel" aria-label="Operations" *ngIf="page === 'operations'">
          <div class="tool-actions operation-filters palette-dialog-toolbar">
            <label>
              Show
              <select [(ngModel)]="operationFilter">
                <option value="all">All</option>
                <option value="active">Active</option>
                <option value="completed">Completed</option>
                <option value="failed">Failed</option>
              </select>
            </label>
            <button
              type="button"
              [disabled]="history.length === 0"
              (click)="clearOperationHistory()">
              Clear history
            </button>
          </div>

          <div class="summary">
            <span>{{ activeOperations.length }} active</span>
            <span>{{ completedOperationCount }} completed</span>
            <span>{{ failedOperationCount }} failed</span>
            <span>{{ history.length }} recent</span>
          </div>

          <article class="operation" *ngFor="let operation of filteredActiveOperations; trackBy:trackOperation">
            <div class="row">
              <strong>{{ operation.config.title }}</strong>
              <span>{{ operation.status }}</span>
            </div>
            <div class="progress">
              <i [style.width.%]="operation.metrics.totalBytes > 0 ? operation.percentComplete : 0"></i>
            </div>
            <small *ngIf="operation.metrics.currentFile">{{ operation.metrics.currentFile }}</small>
            <small class="operation-error" *ngIf="operation.error">{{ operation.error }}</small>
            <div class="operation-actions">
              <button
                type="button"
                *ngIf="operation.config.cancellable && (operation.status === 'running' || operation.status === 'pending' || operation.status === 'paused')"
                (click)="cancelOperation(operation)">
                Cancel
              </button>
              <button
                type="button"
                *ngIf="operation.status === 'error' || operation.status === 'cancelled' || operation.status === 'completed'"
                (click)="dismissOperation(operation)">
                Dismiss
              </button>
            </div>
          </article>

          <div class="history-title" *ngIf="filteredHistory.length">Recent</div>
          <article class="operation history" *ngFor="let operation of filteredHistory; trackBy:trackOperation">
            <div class="row">
              <strong>{{ operation.config.title }}</strong>
              <span>{{ operation.status }}</span>
            </div>
            <small>{{ operation.completedAt | date:'short' }}</small>
            <small *ngIf="operation.destination" [title]="operation.destination">
              → {{ operation.destination }}
            </small>
            <small class="operation-error" *ngIf="operation.error">{{ operation.error }}</small>
          </article>

          <div class="empty" *ngIf="filteredActiveOperations.length === 0 && filteredHistory.length === 0">
            No operations match this view.
          </div>
        </div>

        <div class="body" role="tabpanel" aria-label="Disk Usage" *ngIf="page === 'disk'">
          <div class="tool-actions palette-dialog-toolbar">
            <button type="button" (click)="scanDisk()" [disabled]="diskScanning || !rootPath">
              {{ diskScanning ? 'Scanning…' : 'Scan current directory' }}
            </button>
            <button type="button" *ngIf="diskScanning" (click)="cancelDisk()">Cancel</button>
            <label>
              Sort
              <select [(ngModel)]="diskSort">
                <option value="size">Size</option>
                <option value="files">Files</option>
                <option value="name">Name</option>
              </select>
            </label>
          </div>

          <div class="summary" *ngIf="diskProgress">
            <span>{{ formatBytes(diskProgress.totalSize || 0) }}</span>
            <span>{{ diskProgress.totalFiles || 0 }} files</span>
            <span>{{ diskProgress.totalDirs || 0 }} dirs</span>
          </div>

          <div class="disk-list" *ngIf="diskEntries.length">
            <button
              type="button"
              class="disk-row disk-row-button"
              *ngFor="let entry of sortedDiskEntries; trackBy:trackDisk"
              [class.navigable]="entry.isDir"
              [disabled]="!entry.isDir"
              (click)="openDiskEntry(entry)">
              <span class="name" [title]="entry.path">{{ entry.name }}</span>
              <span>{{ formatBytes(entry.size) }}</span>
              <span>{{ entry.files }} files</span>
              <div class="bar"><i [style.width.%]="diskPercent(entry)"></i></div>
            </button>
          </div>

          <div class="empty" *ngIf="!diskScanning && diskEntries.length === 0">
            Scan the active directory to see where space is used.
          </div>
          <div class="error" *ngIf="diskError">{{ diskError }}</div>
        </div>

        <div class="body" role="tabpanel" aria-label="Duplicates" *ngIf="page === 'duplicates'">
          <div class="tool-actions palette-dialog-toolbar">
            <label>
              Min size
              <input type="number" min="0" [(ngModel)]="duplicateMinSize" [disabled]="duplicateScanning">
            </label>
            <label>
              <input type="checkbox" [(ngModel)]="duplicateIncludeHidden" [disabled]="duplicateScanning">
              Hidden
            </label>
            <label>
              Sort
              <select [(ngModel)]="duplicateSort" [disabled]="duplicateScanning">
                <option value="reclaimable">Reclaimable</option>
                <option value="size">File size</option>
                <option value="copies">Copies</option>
              </select>
            </label>
            <button type="button" (click)="scanDuplicates()" [disabled]="duplicateScanning || !rootPath">
              {{ duplicateScanning ? 'Scanning…' : 'Scan' }}
            </button>
            <button type="button" [disabled]="duplicateGroups.length === 0" (click)="selectAllDuplicates()">
              Select reclaimable
            </button>
            <button type="button" [disabled]="duplicateSelected.size === 0" (click)="clearDuplicateSelection()">
              Unselect
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
              <button
                type="button"
                class="path path-button"
                [title]="'Open containing folder: ' + file.path"
                (click)="openContainingFolder(file.path); $event.preventDefault()">
                {{ file.path }}
              </button>
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

        <footer class="tool-keys palette-dialog-footer">
          <span class="palette-dialog-footnote">1 Operations · 2 Disk · 3 Duplicates · R rescan</span>
          <span *ngIf="page === 'duplicates'" class="palette-dialog-footnote">A select · U unselect · B basket · Delete remove</span>
        </footer>
      </section>
    </div>
  `,
  styleUrls: ['./tools-dialog-remote-theme.scss'],
  styles: [`
    :host{position:fixed;inset:0;z-index:9490;pointer-events:none}
    .tools{width:min(900px,calc(100vw - 30px));height:min(760px,calc(100vh - 30px));display:flex;flex-direction:column}
    .root{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    button,input,select{font:inherit}
    .tool-actions button{border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;padding:5px 9px;cursor:pointer}
    .tool-actions button:hover:not(:disabled){background:var(--vsc-list-hover-background,var(--vsc-list-hover-background));border-color:var(--vsc-border,var(--vsc-border))}
    nav span{opacity:.6;margin-left:4px}.body{flex:1;min-height:0;overflow:auto}.summary,.tool-actions{display:flex;align-items:center;gap:12px;flex-wrap:wrap;padding:8px 12px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border-subtle))}
    .tool-actions label{display:flex;align-items:center;gap:5px}.tool-actions input[type=number]{width:90px;background:var(--vsc-input-background,var(--vsc-popover-background));color:inherit;border:1px solid var(--vsc-border,var(--vsc-border));padding:3px 5px}
    .danger{color:var(--vsc-error-foreground,var(--vsc-error))!important}.operation{padding:9px 12px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border-subtle))}.row{display:flex;justify-content:space-between;gap:10px}.operation small{display:block;margin-top:4px;color:var(--vsc-description-foreground,var(--vsc-foreground-dim));overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .progress,.bar{height:4px;margin-top:6px;background:var(--vsc-neutral-overlay-soft);overflow:hidden;border-radius:99px}.progress i,.bar i{display:block;height:100%;background:var(--vsc-accent-blue,var(--vsc-accent-blue))}
    .history-title{padding:8px 12px;font-weight:600;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}.operation.history{opacity:.8}
    .disk-row{display:grid;grid-template-columns:minmax(140px,1fr) 100px 80px minmax(100px,220px);gap:10px;align-items:center;padding:7px 12px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border-subtle))}.disk-row .name,.path{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .duplicate-group{margin:8px 12px;border:1px solid var(--vsc-border,var(--vsc-list-active-background));border-radius:5px;overflow:hidden}.duplicate-group>header{padding:7px 9px;background:var(--vsc-neutral-overlay-subtle);font-size:11px}.duplicate-file{display:grid;grid-template-columns:22px minmax(0,1fr) auto;gap:7px;align-items:center;padding:6px 9px;border-top:1px solid var(--vsc-neutral-overlay-soft)}.duplicate-file small{color:var(--vsc-accent-green,var(--vsc-success))}
    .empty{padding:48px 20px;text-align:center;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}.error,.warnings{margin:8px 12px;padding:8px;border-radius:4px}.error{color:var(--vsc-error-foreground,var(--vsc-error));background:var(--vsc-error-surface)}.warnings{color: var(--vsc-accent-yellow);background:var(--vsc-warning-surface)}
    button:disabled{opacity:.4;cursor:default}
    .disk-row-button{border-left:0;border-right:0;background:transparent;color:inherit;text-align:left;width:100%}
    .disk-row-button.navigable{cursor:pointer}.disk-row-button.navigable:hover{background:var(--vsc-list-hover-background,var(--vsc-list-hover-background))}.disk-row-button:disabled{opacity:1}
    .path-button{border:0;background:transparent;color:inherit;text-align:left;padding:2px 4px;border-radius:3px;cursor:pointer}.path-button:hover{background:var(--vsc-list-hover-background,var(--vsc-list-hover-background))}
    .tool-keys{display:flex;justify-content:space-between;gap:12px;color:var(--vsc-description-foreground,var(--vsc-foreground-dim));font-size:10px}
    .operation-filters{justify-content:flex-end}
  `],
})
export class ToolsCenterComponent implements OnInit, OnDestroy {
  @Output() changed = new EventEmitter<void>();
  @Output() navigateTo = new EventEmitter<string>();
  @Output() openBasketRequested = new EventEmitter<void>();

  visible = false;
  page: ToolPage = 'operations';
  rootPath = '';

  activeOperations: OperationState[] = [];
  history: OperationHistoryEntry[] = [];
  operationFilter: OperationFilter = 'all';

  diskScanning = false;
  diskSort: DiskSort = 'size';
  diskProgress: DiskUsageProgress | DiskUsageResult | null = null;
  diskEntries: DiskUsageEntry[] = [];
  diskError = '';
  private diskAbort: AbortController | null = null;

  duplicateMinSize = 1;
  duplicateSort: DuplicateSort = 'reclaimable';
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
    private readonly input: InputEngineService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get filteredActiveOperations(): OperationState[] {
    if (this.operationFilter === 'completed' || this.operationFilter === 'failed') {
      return [];
    }
    return this.activeOperations;
  }

  get filteredHistory(): OperationHistoryEntry[] {
    if (this.operationFilter === 'active') return [];
    if (this.operationFilter === 'failed') {
      return this.history.filter(
        item => item.status === 'error' || item.status === 'cancelled',
      );
    }
    if (this.operationFilter === 'completed') {
      return this.history.filter(item => item.status === 'completed');
    }
    return this.history;
  }

  get completedOperationCount(): number {
    return this.history.filter(item => item.status === 'completed').length;
  }

  get failedOperationCount(): number {
    return this.history.filter(
      item => item.status === 'error' || item.status === 'cancelled',
    ).length;
  }

  get sortedDiskEntries(): DiskUsageEntry[] {
    const entries = [...this.diskEntries];
    if (this.diskSort === 'files') {
      return entries.sort((a,b) => (b.files ?? 0) - (a.files ?? 0));
    }
    if (this.diskSort === 'name') {
      return entries.sort((a,b) => (a.name ?? '').localeCompare(b.name ?? ''));
    }
    return entries.sort((a,b) => (b.size ?? 0) - (a.size ?? 0));
  }

  get duplicateGroups(): DuplicateGroup[] {
    const groups = [...(this.duplicateResult?.groups ?? [])];
    if (this.duplicateSort === 'size') {
      return groups.sort((a,b) => b.size - a.size);
    }
    if (this.duplicateSort === 'copies') {
      return groups.sort((a,b) => b.files.length - a.files.length);
    }
    return groups.sort((a,b) => b.reclaimableBytes - a.reclaimableBytes);
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

    this.input.registerContext({
      id: CONTEXT_ID,
      kind: 'overlay',
      priority: 395, exclusive: true,
      parentId: 'global',
      description: 'Reassembly tools center',
    });

    this.input.registerCommandDescriptor({id:COMMAND.close,title:'Close Tools Center',category:'Tools',execute:()=>this.close()},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.operations,title:'Tools: Operations',category:'Tools',execute:()=>this.setPage('operations')},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.disk,title:'Tools: Disk Usage',category:'Tools',execute:()=>this.setPage('disk')},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.duplicates,title:'Tools: Duplicates',category:'Tools',execute:()=>this.setPage('duplicates')},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.rescan,title:'Rescan Current Tool',category:'Tools',execute:()=>this.rescan(),isEnabled:()=>this.page!=='operations'&&!!this.rootPath},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.selectAllDuplicates,title:'Select Reclaimable Duplicates',category:'Tools',execute:()=>this.selectAllDuplicates(),isEnabled:()=>this.page==='duplicates'&&this.duplicateGroups.length>0},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.clearDuplicateSelection,title:'Clear Duplicate Selection',category:'Tools',execute:()=>this.clearDuplicateSelection(),isEnabled:()=>this.page==='duplicates'&&this.duplicateSelected.size>0},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.addDuplicatesToBasket,title:'Add Duplicates to Basket',category:'Tools',execute:()=>this.collectDuplicateSelection(),isEnabled:()=>this.page==='duplicates'&&this.duplicateSelected.size>0},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.deleteDuplicates,title:'Delete Selected Duplicates',category:'Tools',execute:()=>this.deleteDuplicateSelection(),isEnabled:()=>this.page==='duplicates'&&this.duplicateSelected.size>0},OWNER_ID);

    for (const binding of BINDINGS) this.input.registerBinding(binding);
  }

  ngOnDestroy(): void {
    this.cancelDisk();
    this.input.deactivateContext(CONTEXT_ID);
    for (const binding of BINDINGS) this.input.unregisterBinding(binding.id);
    for (const id of Object.values(COMMAND)) {
      this.input.unregisterCommandDescriptor(id, OWNER_ID);
    }
    this.destroy$.next();
    this.destroy$.complete();
  }

  open(page: ToolPage, rootPath: string): void {
    
    const rootChanged = this.rootPath !== rootPath;
    this.page = page;
    this.rootPath = rootPath;
    this.visible = true;

    if (rootChanged) {
      this.diskEntries = [];
      this.diskProgress = null;
      this.diskError = '';
      this.duplicateResult = null;
      this.duplicateProgress = null;
      this.duplicateSelected = new Set();
      this.duplicateError = '';
    }

    this.input.activateContext(CONTEXT_ID);
    this.cdr.markForCheck();

    if (page === 'disk' && this.diskEntries.length === 0) {
      void this.scanDisk();
    }
  }

  close(): void {
    if (!this.visible) return;
    this.visible = false;
    this.input.deactivateContext(CONTEXT_ID);
    
    
    this.cdr.markForCheck();
  }

  setPage(page: ToolPage): void {
    this.page = page;
    this.cdr.markForCheck();

    if (page === 'disk' && this.diskEntries.length === 0 && this.rootPath) {
      void this.scanDisk();
    }
  }

  rescan(): void {
    if (this.page === 'disk') {
      void this.scanDisk();
    } else if (this.page === 'duplicates') {
      void this.scanDuplicates();
    }
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

  selectAllDuplicates(): void {
    const selected = new Set<string>();
    for (const group of this.duplicateGroups) {
      for (const file of group.files) {
        if (file.path !== group.suggestedKeeperPath) {
          selected.add(file.path);
        }
      }
    }
    this.duplicateSelected = selected;
    this.cdr.markForCheck();
  }

  clearDuplicateSelection(): void {
    this.duplicateSelected = new Set();
    this.cdr.markForCheck();
  }

  openDiskEntry(entry: DiskUsageEntry): void {
    if (!entry.isDir) return;
    this.navigateTo.emit(entry.path);
    this.close();
  }

  openContainingFolder(path: string): void {
    const normalized = (path ?? '').replace(/\\/g, '/').replace(/\/+$/, '');
    const schemeIndex = normalized.indexOf('://');
    const minimum = schemeIndex >= 0 ? schemeIndex + 3 : 0;
    const slash = normalized.lastIndexOf('/');
    const parent = slash < minimum ? normalized : normalized.slice(0, slash);
    if (!parent) return;
    this.navigateTo.emit(parent);
    this.close();
  }

  collectDuplicateSelection(): void {
    if (this.duplicateSelected.size === 0) return;
    this.basket.add(Array.from(this.duplicateSelected), 'duplicates');
    this.openBasketRequested.emit();
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

  clearOperationHistory(): void {
    this.operations.clearHistory();
    this.cdr.markForCheck();
  }

  async cancelOperation(operation: OperationState): Promise<void> {
    if (!operation.config.cancellable) return;
    await this.operations.cancel(operation.id);
  }

  dismissOperation(operation: OperationState): void {
    this.operations.dismiss(operation.id);
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