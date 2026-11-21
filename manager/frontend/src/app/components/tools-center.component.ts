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
import { createSubject, takeUntil } from '@epikodelabs/streamix';

import { ApiService } from '../services/api.service';
import type { AppOverlayToolsPage } from '../services/application-overlay-host.service';
import { FolderInventoryComponent } from './folder-inventory.component';
import { parentPath } from './address-bar.utils';
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
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';
import { FileType, type FileInfo } from '../shared/protocol';
import type {
  DiskUsageEntry,
  DiskUsageProgress,
  DiskUsageResult,
} from '../shared/protocol-enhanced';

type OperationFilter = 'all' | 'active' | 'completed' | 'failed';
type DiskSort = 'size' | 'files' | 'name';
type DuplicateSort = 'reclaimable' | 'size' | 'copies';

interface ChecksumAlgorithmRow {
  id: 'sha256' | 'sha512' | 'sha1' | 'md5';
  label: string;
  value: string;
}

const CONTEXT_ID = 'overlay.Tools';
const OWNER_ID = 'overlay.Tools';

const COMMAND = {
  close: 'reassembly.toolsCenter.close',
  operations: 'reassembly.toolsCenter.operations',
  disk: 'reassembly.toolsCenter.disk',
  duplicates: 'reassembly.toolsCenter.duplicates',
  checksums: 'reassembly.toolsCenter.checksums',
  inventory: 'reassembly.toolsCenter.inventory',
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
  { id:'reassembly.toolsCenter.page4', commandId:COMMAND.checksums, contextId:CONTEXT_ID, source:'default', description:'Checksums page', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'4'}]}}},
  { id:'reassembly.toolsCenter.page5', commandId:COMMAND.inventory, contextId:CONTEXT_ID, source:'default', description:'Folder Inventory page', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'5'}]}}},
  { id:'reassembly.toolsCenter.rescan', commandId:COMMAND.rescan, contextId:CONTEXT_ID, source:'default', description:'Rescan current tool', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'r'}]}}},
  { id:'reassembly.toolsCenter.selectAllDuplicates', commandId:COMMAND.selectAllDuplicates, contextId:CONTEXT_ID, source:'default', description:'Select reclaimable duplicates', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'a'}]}}},
  { id:'reassembly.toolsCenter.clearDuplicateSelection', commandId:COMMAND.clearDuplicateSelection, contextId:CONTEXT_ID, source:'default', description:'Clear duplicate selection', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'u'}]}}},
  { id:'reassembly.toolsCenter.addDuplicatesToBasket', commandId:COMMAND.addDuplicatesToBasket, contextId:CONTEXT_ID, source:'default', description:'Add duplicate selection to basket', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'b'}]}}},
  { id:'reassembly.toolsCenter.deleteDuplicates', commandId:COMMAND.deleteDuplicates, contextId:CONTEXT_ID, source:'default', description:'Delete selected duplicates', category:'Tools', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'delete'}]}}},
];

@Component({
  selector: 'app-tools-center',
  imports: [CommonModule, FolderInventoryComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop palette-dialog-backdrop dialog-group" *ngIf="renderVisible" [class.dialog-group--closing]="isClosing" (mousedown)="close()" (animationend)="onAnimationEnd($event)">
      <section class="tools palette-dialog-surface dialog-surface" role="dialog" aria-modal="true" aria-label="File manager tools"
        (mousedown)="$event.stopPropagation()"
        tabindex="-1" (keydown.escape)="close(); $event.preventDefault(); $event.stopPropagation()">
        <header class="palette-dialog-header">
          <div class="palette-dialog-title">
            <strong>Tools</strong>
            <span class="root palette-dialog-subtitle palette-dialog-mono" [title]="rootPath">{{ rootPath }}</span>
          </div>
          <button type="button" class="dialog-close" aria-label="Close dialog" title="Close" (click)="close()">×</button>
        </header>

        <nav class="palette-dialog-tabbar" role="tablist" aria-label="Tool pages" (keydown)="onTabKey($event)">
          <button type="button" role="tab" id="tool-tab-operations" aria-controls="tool-panel-operations" [attr.aria-selected]="page === 'operations'" [attr.tabindex]="page === 'operations' ? 0 : -1" [class.active]="page === 'operations'" (click)="setPage('operations')">
            Operations <span>{{ activeOperations.length }}</span>
          </button>
          <button type="button" role="tab" id="tool-tab-disk" aria-controls="tool-panel-disk" [attr.aria-selected]="page === 'disk'" [attr.tabindex]="page === 'disk' ? 0 : -1" [class.active]="page === 'disk'" (click)="setPage('disk')">Disk Usage</button>
          <button type="button" role="tab" id="tool-tab-duplicates" aria-controls="tool-panel-duplicates" [attr.aria-selected]="page === 'duplicates'" [attr.tabindex]="page === 'duplicates' ? 0 : -1" [class.active]="page === 'duplicates'" (click)="setPage('duplicates')">Duplicates</button>
          <button type="button" role="tab" id="tool-tab-checksums" aria-controls="tool-panel-checksums" [attr.aria-selected]="page === 'checksums'" [attr.tabindex]="page === 'checksums' ? 0 : -1" [class.active]="page === 'checksums'" (click)="setPage('checksums')">Checksums</button>
          <button type="button" role="tab" id="tool-tab-inventory" aria-controls="tool-panel-inventory" [attr.aria-selected]="page === 'inventory'" [attr.tabindex]="page === 'inventory' ? 0 : -1" [class.active]="page === 'inventory'" (click)="setPage('inventory')">Folder Inventory</button>
        </nav>

        <div class="body" role="tabpanel" id="tool-panel-inventory" aria-labelledby="tool-tab-inventory" tabindex="0" *ngIf="page === 'inventory'">
          <app-folder-inventory [rootPath]="rootPath" (navigateTo)="navigateTo.emit($event); close()" />
        </div>

        <div class="body" role="tabpanel" id="tool-panel-operations" aria-labelledby="tool-tab-operations" tabindex="0" *ngIf="page === 'operations'">
          <div class="tool-actions operation-filters palette-dialog-toolbar">
            <button type="button" (click)="close(); openJournalRequested.emit()">Operation History…</button>
            <input type="search" class="tool-filter-input palette-dialog-field" placeholder="Filter operations…"
              aria-label="Filter operations" (input)="onOperationQueryInput($event)" [value]="operationQuery" />
            <label>
              Show
              <select class="palette-dialog-field" [value]="operationFilter" (change)="onOperationFilterChange($event)">
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
            <progress class="operation-progress" [attr.aria-label]="operation.config.title" max="100"
              [attr.value]="operation.metrics.totalBytes > 0 ? operation.percentComplete : null"></progress>
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

        <div class="body" role="tabpanel" id="tool-panel-disk" aria-labelledby="tool-tab-disk" tabindex="0" *ngIf="page === 'disk'">
          <div class="tool-actions palette-dialog-toolbar">
            <button type="button" (click)="scanDisk()" [disabled]="diskScanning || !rootPath">
              {{ diskScanning ? 'Scanning…' : 'Scan current directory' }}
            </button>
            <button type="button" *ngIf="diskScanning" (click)="cancelDisk()">Cancel</button>
            <input
              type="search"
              class="tool-filter-input palette-dialog-field"
              [value]="diskFilter"
              (input)="onDiskFilterInput($event)"
              placeholder="Filter entries…"
              aria-label="Filter disk usage entries" />
            <label>
              Sort
              <select [value]="diskSort" (change)="onDiskSortChange($event)">
                <option value="size">Size</option>
                <option value="files">Files</option>
                <option value="name">Name</option>
              </select>
            </label>
          </div>

          <div class="summary" *ngIf="diskProgress || diskCancelled || diskScanning">
            <span *ngIf="diskScanning" role="status">Scanning folder…</span>
            <span *ngIf="diskCancelled" role="status">Scan cancelled. Results may be incomplete.</span>
            <ng-container *ngIf="diskProgress">
            <span>{{ formatBytes(diskProgress.totalSize || 0) }}</span>
            <span>{{ diskProgress.totalFiles || 0 }} files</span>
            <span>{{ diskProgress.totalDirs || 0 }} dirs</span>
            </ng-container>
          </div>

          <div class="disk-list" *ngIf="sortedDiskEntries.length">
            <button
              type="button"
              class="disk-row disk-row-button"
              *ngFor="let entry of sortedDiskEntries; trackBy:trackDisk"
              [class.navigable]="entry.isDir"
              [disabled]="!entry.isDir"
              (click)="openDiskEntry(entry)">
              <span class="name" [title]="entry.path">{{ entry.name }}</span>
              <span class="num">{{ formatBytes(entry.size) }}</span>
              <span class="num">{{ entry.files }} files</span>
              <div class="bar"><i [style.width.%]="diskPercent(entry)"></i></div>
            </button>
          </div>

          <div class="empty" *ngIf="!diskScanning && diskEntries.length === 0">
            Scan the active directory to see where space is used.
          </div>
          <div class="empty" *ngIf="!diskScanning && diskEntries.length > 0 && sortedDiskEntries.length === 0">
            No entries match "{{ diskFilter }}".
          </div>
          <div class="error" role="alert" *ngIf="diskError">{{ diskError }}</div>
        </div>

        <div class="body" role="tabpanel" id="tool-panel-duplicates" aria-labelledby="tool-tab-duplicates" tabindex="0" *ngIf="page === 'duplicates'">
          <div class="tool-actions palette-dialog-toolbar">
            <label>
              Min size
              <input type="number" min="0" [value]="duplicateMinSize" (input)="onDuplicateMinSizeInput($event)" [disabled]="duplicateScanning">
            </label>
            <label>
              <input type="checkbox" [checked]="duplicateIncludeHidden" (change)="onDuplicateHiddenChange($event)" [disabled]="duplicateScanning">
              Hidden
            </label>
            <label>
              Sort
              <select [value]="duplicateSort" (change)="onDuplicateSortChange($event)" [disabled]="duplicateScanning">
                <option value="reclaimable">Reclaimable</option>
                <option value="size">File size</option>
                <option value="copies">Copies</option>
              </select>
            </label>
            <input
              type="search"
              class="tool-filter-input palette-dialog-field"
              [value]="duplicateFilter"
              (input)="onDuplicateFilterInput($event)"
              placeholder="Filter path…"
              aria-label="Filter duplicate paths" />
            <button type="button" (click)="scanDuplicates()" [disabled]="duplicateScanning || !rootPath">
              {{ duplicateScanning ? 'Scanning…' : 'Scan' }}
            </button>
            <button type="button" *ngIf="duplicateScanning" (click)="cancelDuplicates()">Cancel</button>
            <button type="button" [disabled]="duplicateGroups.length === 0" (click)="selectAllDuplicates()">
              Select reclaimable
            </button>
            <button type="button" [disabled]="duplicateSelected.size === 0" (click)="clearDuplicateSelection()">
              Unselect
            </button>
            <button type="button" [disabled]="duplicateSelected.size === 0" (click)="collectDuplicateSelection()">
              Add to Basket
            </button>
            <button type="button" class="danger" [disabled]="duplicateDeleting || duplicateSelected.size === 0" (click)="deleteDuplicateSelection()">
              Move Selected to Trash
            </button>
          </div>

          <div class="summary" *ngIf="duplicateProgress">
            <span>{{ duplicateProgress.phase }}</span>
            <span>{{ duplicateProgress.discoveredFiles }} discovered</span>
            <span *ngIf="duplicateProgress.phase === 'hashing'">{{ duplicateProgress.hashedFiles }} of {{ duplicateProgress.candidateFiles }} hashed</span>
          </div>

          <div class="summary" *ngIf="duplicateResult">
            <span>{{ duplicateResult.groups.length }} groups</span>
            <span>{{ duplicateResult.scannedFiles }} files</span>
            <span>{{ formatBytes(duplicateResult.reclaimableBytes) }} reclaimable</span>
            <span>{{ duplicateSelected.size }} selected</span>
          </div>

          <section class="duplicate-group" *ngFor="let group of duplicateGroups; trackBy:trackGroup">
            <header>
              <strong>{{ group.files.length }} copies</strong>
              <span> · {{ formatBytes(group.size) }} each</span>
              <span> · {{ formatBytes(group.reclaimableBytes) }} reclaimable</span>
            </header>

            <div class="duplicate-file" *ngFor="let file of group.files">
              <input
                type="checkbox"
                [attr.aria-label]="'Select duplicate: ' + file.path"
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
              <div class="duplicate-file-actions">
                <small *ngIf="file.path === group.suggestedKeeperPath">keeper</small>
                <button
                  type="button"
                  class="copy-mini-btn"
                  (click)="copyPath(file.path)">
                  {{ copiedValue === file.path ? 'Copied' : 'Copy Path' }}
                </button>
              </div>
            </div>
          </section>

          <div class="empty" *ngIf="!duplicateScanning && !duplicateError && duplicateGroups.length === 0">
            {{ !duplicateResult ? 'Scan recursively to find content-identical files.' :
              duplicateResult.groups.length ? 'No duplicate groups match this filter.' : 'No duplicate groups were found.' }}
          </div>

          <div class="warnings" role="status" *ngIf="duplicateResult?.truncated">
            Scan reached a file or depth limit. Results cover only the scanned portion of this folder.
          </div>
          <details class="warnings" *ngIf="duplicateResult && duplicateResult.warnings.length > 0">
            <summary>{{ duplicateResult.warnings.length }} paths could not be scanned</summary>
            <ul><li *ngFor="let warning of duplicateResult.warnings">{{ warning }}</li></ul>
          </details>
          <div class="error" role="alert" *ngIf="duplicateError">{{ duplicateError }}</div>
        </div>

        <div class="body" role="tabpanel" id="tool-panel-checksums" aria-labelledby="tool-tab-checksums" tabindex="0" *ngIf="page === 'checksums'">
          <div class="tool-actions palette-dialog-toolbar">
            <select
              *ngIf="checksumCandidates.length > 0"
              class="checksum-select palette-dialog-field"
              [value]="checksumTargetPath"
              (change)="onChecksumCandidateSelect($event)"
              aria-label="Files in active directory">
              <option value="">Select file from active directory…</option>
              <option *ngFor="let file of checksumCandidates" [value]="file.path">
                {{ file.name }} ({{ formatBytes(file.size) }})
              </option>
            </select>
            <input
              type="text"
              class="checksum-path-input palette-dialog-field"
              [value]="checksumTargetPath"
              (input)="onChecksumPathInput($event)"
              (keydown.enter)="computeChecksums()"
              placeholder="Enter or choose file path…"
              aria-label="Target file path" />
            <button
              type="button"
              [disabled]="checksumComputing || !checksumTargetPath.trim()"
              (click)="computeChecksums()">
              {{ checksumComputing ? 'Hashing…' : 'Compute' }}
            </button>
          </div>

          <div class="summary" *ngIf="checksumResults.length > 0">
            <span>{{ checksumTargetPath }}</span>
            <span *ngIf="checksumFileSize !== null">{{ formatBytes(checksumFileSize) }}</span>
            <button type="button" class="palette-dialog-button" (click)="copyChecksumReport()">{{ copiedValue === checksumReport ? 'Copied' : 'Copy all checksums' }}</button>
          </div>

          <div class="checksum-grid" *ngIf="checksumResults.length > 0">
            <div class="checksum-row" *ngFor="let item of checksumResults">
              <span class="checksum-algo">{{ item.label }}</span>
              <code class="checksum-digest" [title]="item.value">{{ item.value }}</code>
              <button type="button" class="copy-mini-btn" (click)="copyDigest(item.value)">
                {{ copiedValue === item.value ? 'Copied' : 'Copy' }}
              </button>
            </div>
          </div>

          <div class="checksum-verify" *ngIf="checksumResults.length > 0">
            <label for="checksum-expected">Verify expected digest ({{ availableChecksumAlgorithms }})</label>
            <div class="checksum-verify-row">
              <input
                id="checksum-expected"
                type="text"
                class="checksum-path-input palette-dialog-field"
                [value]="expectedDigest"
                (input)="onExpectedDigestInput($event)"
                placeholder="Paste expected hex digest to verify…" />
              <span
                *ngIf="expectedDigest.trim()"
                class="verify-status"
                role="status"
                [class.match]="!!matchedAlgorithm"
                [class.mismatch]="!matchedAlgorithm">
                {{ matchedAlgorithm ? 'Match (' + matchedAlgorithm + ')' : 'Does not match any digest' }}
              </span>
            </div>
          </div>

          <div class="empty" *ngIf="!checksumComputing && checksumResults.length === 0 && !checksumError">
            Choose a file or enter a path to compute its checksum.
          </div>
          <div class="error" role="alert" *ngIf="checksumError">{{ checksumError }}</div>
        </div>

        <p class="error" role="alert" *ngIf="clipboardError">{{ clipboardError }}</p>
        <footer class="tool-keys palette-dialog-footer">
          <span class="palette-dialog-footnote">1 Operations · 2 Disk · 3 Duplicates · 4 Checksums · 5 Inventory</span>
          <span *ngIf="page === 'duplicates'" class="palette-dialog-footnote">A select · U unselect · B basket · Delete move to Trash</span>
        </footer>
      </section>
    </div>
  `,
  styleUrls: ['./tools-dialog-remote-theme.scss'],
  styles: [`
    :host{position:fixed;inset:0;z-index:9490;pointer-events:none}
    .tools{width:min(920px,calc(100vw - 30px));height:min(760px,calc(100vh - 30px));display:flex;flex-direction:column}
    .root{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    button,input,select{font:inherit}
    .tool-actions button{border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;padding:5px 9px;cursor:pointer}
    .tool-actions button:hover:not(:disabled){background:var(--vsc-list-hover-background);border-color:var(--vsc-border)}
    nav span{opacity:.6;margin-left:4px}.body{flex:1;min-height:0;overflow:auto}.summary,.tool-actions{display:flex;align-items:center;gap:10px;flex-wrap:wrap;padding:8px 12px;border-bottom:1px solid var(--vsc-border-subtle);font-variant-numeric:tabular-nums}
    .tool-actions label{display:flex;align-items:center;gap:5px}.tool-actions input[type=number]{width:82px;background:var(--vsc-input-background);color:inherit;border:1px solid var(--vsc-border);padding:3px 5px}
    .tool-filter-input{flex:1;min-width:min(150px,100%)}
    .danger{color:var(--vsc-error-foreground,var(--vsc-error))!important}.operation{padding:9px 12px;border-bottom:1px solid var(--vsc-border-subtle)}.row{display:flex;justify-content:space-between;gap:10px}.operation small{display:block;margin-top:4px;color:var(--vsc-description-foreground,var(--vsc-foreground-dim));overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .progress,.bar{height:4px;margin-top:6px;background:var(--vsc-neutral-overlay-soft);overflow:hidden;border-radius:99px}.progress i,.bar i{display:block;height:100%;background:var(--vsc-accent-blue)}
    .operation-progress{display:block;width:100%;height:6px;margin-top:6px;accent-color:var(--vsc-accent-blue)}
    .history-title{padding:8px 12px;font-weight:600;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}.operation.history{opacity:.8}
    .disk-row{display:grid;grid-template-columns:minmax(140px,1fr) 100px 80px minmax(100px,220px);gap:10px;align-items:center;padding:7px 12px;border-bottom:1px solid var(--vsc-border-subtle)}.disk-row .name,.path{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.num{font-variant-numeric:tabular-nums}
    .duplicate-group{margin:8px 12px;border:1px solid var(--vsc-border,var(--vsc-list-active-background));border-radius:5px;overflow:hidden}.duplicate-group>header{padding:7px 9px;background:var(--vsc-neutral-overlay-subtle);font-size:11px;font-variant-numeric:tabular-nums}.duplicate-file{display:grid;grid-template-columns:22px minmax(0,1fr) auto;gap:7px;align-items:center;padding:6px 9px;border-top:1px solid var(--vsc-neutral-overlay-soft)}.duplicate-file small{color:var(--vsc-accent-green,var(--vsc-success))}
    .duplicate-file-actions{display:flex;align-items:center;gap:8px}
    .copy-mini-btn{padding:2px 7px;border:1px solid var(--vsc-border-subtle,var(--vsc-border));border-radius:3px;font-size:10px;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    .copy-mini-btn:hover{color:var(--vsc-foreground);background:var(--vsc-list-hover-background)}
    .checksum-select{max-width:min(260px,100%)}
    .checksum-path-input{flex:1;min-width:min(200px,100%);font-family:var(--vsc-font-family-mono,ui-monospace,monospace);font-size:11px}
    .palette-dialog-tabbar{overflow-x:auto;flex-shrink:0}
    .palette-dialog-tabbar button{white-space:nowrap}
    @media (max-width: 560px) {
      .disk-row{grid-template-columns:minmax(0,1fr) auto auto}
      .disk-row .bar{display:none}
      .tool-keys{flex-wrap:wrap}
    }
    .checksum-grid{display:grid;gap:1px;border-bottom:1px solid var(--vsc-border-subtle)}
    .checksum-row{display:grid;grid-template-columns:72px minmax(0,1fr) auto;align-items:center;gap:10px;padding:9px 12px;border-bottom:1px solid var(--vsc-border-subtle)}
    .checksum-row:last-child{border-bottom:0}
    .checksum-algo{font-size:11px;font-weight:600;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    .checksum-digest{font-family:var(--vsc-font-family-mono,ui-monospace,monospace);font-size:11px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;user-select:all}
    .checksum-verify{display:grid;gap:6px;padding:12px}
    .checksum-verify label{font-size:11px;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    .checksum-verify-row{display:flex;align-items:center;flex-wrap:wrap;gap:8px}
    .verify-status{font-size:11px;font-weight:600}
    .verify-status.match{color:var(--vsc-accent-green,var(--vsc-success))}
    .verify-status.mismatch{color:var(--vsc-error-foreground,var(--vsc-error))}
    .empty{padding:48px 20px;text-align:center;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}.error,.warnings{margin:8px 12px;padding:8px;border-radius:4px}.error{color:var(--vsc-error-foreground,var(--vsc-error));background:var(--vsc-error-surface)}.warnings{color:var(--vsc-accent-yellow);background:var(--vsc-warning-surface)}
    button:disabled{opacity:.4;cursor:default}
    .disk-row-button{border-left:0;border-right:0;background:transparent;color:inherit;text-align:left;width:100%}
    .disk-row-button.navigable{cursor:pointer}.disk-row-button.navigable:hover{background:var(--vsc-list-hover-background)}.disk-row-button:disabled{opacity:1}
    .path-button{border:0;background:transparent;color:inherit;text-align:left;padding:2px 4px;border-radius:3px;cursor:pointer}.path-button:hover{background:var(--vsc-list-hover-background)}
    .tool-keys{display:flex;justify-content:space-between;gap:12px;color:var(--vsc-description-foreground,var(--vsc-foreground-dim));font-size:10px}
    .operation-filters{justify-content:flex-end}
  `],
})
export class ToolsCenterComponent implements OnInit, OnDestroy {
  @Output() changed = new EventEmitter<void>();
  @Output() navigateTo = new EventEmitter<string>();
  @Output() openBasketRequested = new EventEmitter<void>();
  @Output() openJournalRequested = new EventEmitter<void>();

  visible = false;
  renderVisible = false;
  isClosing = false;
  page: AppOverlayToolsPage = 'operations';
  rootPath = '';

  activeOperations: OperationState[] = [];
  history: OperationHistoryEntry[] = [];
  operationFilter: OperationFilter = 'all';
  operationQuery = '';

  diskScanning = false;
  diskCancelled = false;
  diskSort: DiskSort = 'size';
  diskFilter = '';
  diskProgress: DiskUsageProgress | DiskUsageResult | null = null;
  diskEntries: DiskUsageEntry[] = [];
  diskError = '';
  private diskAbort: AbortController | null = null;

  duplicateMinSize = 1;
  duplicateSort: DuplicateSort = 'reclaimable';
  duplicateFilter = '';
  duplicateIncludeHidden = false;
  duplicateScanning = false;
  duplicateDeleting = false;
  private duplicateAbort: AbortController | null = null;
  duplicateProgress: DuplicateScanProgress | null = null;
  duplicateResult: DuplicateScanResult | null = null;
  duplicateSelected = new Set<string>();
  duplicateError = '';

  checksumCandidates: FileInfo[] = [];
  checksumTargetPath = '';
  checksumComputing = false;
  private checksumRequest = 0;
  checksumFileSize: number | null = null;
  checksumResults: ChecksumAlgorithmRow[] = [];
  get availableChecksumAlgorithms(): string {
    return this.checksumResults.map(result => result.label).join(' / ');
  }
  checksumError = '';
  expectedDigest = '';
  copiedValue = '';
  clipboardError = '';

  private readonly destroy$ = createSubject<void>();
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;
  private copiedTimer: number | null = null;

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
    return this.activeOperations.filter(item => this.operationMatches(item.config.title, item.metrics.currentFile));
  }

  get filteredHistory(): OperationHistoryEntry[] {
    if (this.operationFilter === 'active') return [];
    const matching = this.history.filter(item => this.operationMatches(item.config.title, item.destination, item.error));
    if (this.operationFilter === 'failed') {
      return matching.filter(
        item => item.status === 'error' || item.status === 'cancelled',
      );
    }
    if (this.operationFilter === 'completed') {
      return matching.filter(item => item.status === 'completed');
    }
    return matching;
  }

  private operationMatches(...parts: (string | null | undefined)[]): boolean {
    const query = this.operationQuery.trim().toLowerCase();
    return !query || parts.some(part => part?.toLowerCase().includes(query));
  }

  onOperationQueryInput(event: Event): void {
    this.operationQuery = (event.target as HTMLInputElement).value;
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
    const query = this.diskFilter.trim().toLowerCase();
    const entries = query
      ? this.diskEntries.filter(
          entry =>
            (entry.name ?? '').toLowerCase().includes(query) ||
            (entry.path ?? '').toLowerCase().includes(query),
        )
      : [...this.diskEntries];
    if (this.diskSort === 'files') {
      return entries.sort((a,b) => (b.files ?? 0) - (a.files ?? 0));
    }
    if (this.diskSort === 'name') {
      return entries.sort((a,b) => (a.name ?? '').localeCompare(b.name ?? ''));
    }
    return entries.sort((a,b) => (b.size ?? 0) - (a.size ?? 0));
  }

  get duplicateGroups(): DuplicateGroup[] {
    const query = this.duplicateFilter.trim().toLowerCase();
    const groups = [...(this.duplicateResult?.groups ?? [])].filter(group => {
      if (!query) return true;
      return group.files.some(file => file.path.toLowerCase().includes(query));
    });
    if (this.duplicateSort === 'size') {
      return groups.sort((a,b) => b.size - a.size);
    }
    if (this.duplicateSort === 'copies') {
      return groups.sort((a,b) => b.files.length - a.files.length);
    }
    return groups.sort((a,b) => b.reclaimableBytes - a.reclaimableBytes);
  }

  get matchedAlgorithm(): string | null {
    const input = this.expectedDigest.trim().toLowerCase();
    const normalized = /^(?:sha-?(?:256|512|1)|md5)\s*:\s*([a-f0-9]+)$/.exec(input)?.[1]
      ?? /^([a-f0-9]+)\s+.+$/.exec(input)?.[1]
      ?? input;
    if (!normalized || this.checksumResults.length === 0) return null;
    const match = this.checksumResults.find(row => row.value.toLowerCase() === normalized);
    return match ? match.label : null;
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
    this.input.registerCommandDescriptor({id:COMMAND.checksums,title:'Tools: Checksums',category:'Tools',execute:()=>this.setPage('checksums')},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.inventory,title:'Tools: Folder Inventory',category:'Tools',execute:()=>this.setPage('inventory')},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.rescan,title:'Rescan Current Tool',category:'Tools',execute:()=>this.rescan(),isEnabled:()=>this.page==='checksums'||((this.page==='disk'||this.page==='duplicates')&&!!this.rootPath)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.selectAllDuplicates,title:'Select Reclaimable Duplicates',category:'Tools',execute:()=>this.selectAllDuplicates(),isEnabled:()=>this.page==='duplicates'&&this.duplicateGroups.length>0},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.clearDuplicateSelection,title:'Clear Duplicate Selection',category:'Tools',execute:()=>this.clearDuplicateSelection(),isEnabled:()=>this.page==='duplicates'&&this.duplicateSelected.size>0},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.addDuplicatesToBasket,title:'Add Duplicates to Basket',category:'Tools',execute:()=>this.collectDuplicateSelection(),isEnabled:()=>this.page==='duplicates'&&this.duplicateSelected.size>0},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.deleteDuplicates,title:'Move Selected Duplicates to Trash',category:'Tools',execute:()=>this.deleteDuplicateSelection(),isEnabled:()=>this.page==='duplicates'&&!this.duplicateDeleting&&this.duplicateSelected.size>0},OWNER_ID);

    for (const binding of BINDINGS) this.input.registerBinding(binding);
  }

  ngOnDestroy(): void {
    this.checksumRequest++;
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.cancelDisk();
    this.cancelDuplicates();
    if (this.copiedTimer !== null) {
      window.clearTimeout(this.copiedTimer);
      this.copiedTimer = null;
    }
    this.input.deactivateContext(CONTEXT_ID);
    for (const binding of BINDINGS) this.input.unregisterBinding(binding.id);
    for (const id of Object.values(COMMAND)) {
      this.input.unregisterCommandDescriptor(id, OWNER_ID);
    }
    this.destroy$.next();
    this.destroy$.complete();
  }

  open(page: AppOverlayToolsPage, rootPath: string): void {
    const rootChanged = this.rootPath !== rootPath;
    this.page = page;
    this.clipboardError = '';
    this.rootPath = rootPath;
    this.isClosing = false;
    this.renderVisible = true;
    this.visible = true;

    if (rootChanged) {
      this.cancelDisk();
      this.diskCancelled = false;
      this.cancelDuplicates();
      this.diskEntries = [];
      this.diskProgress = null;
      this.diskError = '';
      this.diskFilter = '';
      this.duplicateResult = null;
      this.duplicateProgress = null;
      this.duplicateSelected = new Set();
      this.duplicateError = '';
      this.duplicateFilter = '';
      this.checksumCandidates = [];
      this.updateChecksumTarget('');
      this.checksumResults = [];
      this.checksumFileSize = null;
      this.checksumError = '';
    }

    this.input.activateContext(CONTEXT_ID);
    if (this.modalStack && this.hostElement && !this.unregisterModal) {
      this.unregisterModal = this.modalStack.register({
        id: 'tools-center',
        contextId: CONTEXT_ID,
        element: this.hostElement.nativeElement,
        dismiss: () => this.close(),
        dismissible: () => this.visible && !this.isClosing,
      });
    }
    this.cdr.markForCheck();

    if (page === 'disk' && this.diskEntries.length === 0) {
      void this.scanDisk();
    } else if (page === 'checksums' && this.checksumCandidates.length === 0 && rootPath) {
      void this.loadChecksumCandidates();
    }
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) return;
    this.renderVisible = false;
    this.isClosing = false;
    this.cdr.markForCheck();
  }

  close(): void {
    this.unregisterModal?.();
    this.unregisterModal = null;
    if (!this.renderVisible || this.isClosing) return;
    this.visible = false;
    this.isClosing = true;
    this.input.deactivateContext(CONTEXT_ID);
    this.cdr.markForCheck();
  }

  onTabKey(event: KeyboardEvent): void {
    const tabs = Array.from((event.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('[role="tab"]'));
    const index = tabs.indexOf(event.target as HTMLButtonElement);
    if (index < 0) return;
    const next = {
      ArrowLeft: (index + tabs.length - 1) % tabs.length,
      ArrowRight: (index + 1) % tabs.length,
      Home: 0,
      End: tabs.length - 1,
    }[event.key];
    if (next === undefined) return;
    event.preventDefault();
    tabs[next].focus();
    tabs[next].click();
  }

  setPage(page: AppOverlayToolsPage): void {
    this.page = page;
    this.clipboardError = '';
    this.cdr.markForCheck();

    if (page === 'disk' && this.diskEntries.length === 0 && this.rootPath) {
      void this.scanDisk();
    } else if (page === 'checksums' && this.checksumCandidates.length === 0 && this.rootPath) {
      void this.loadChecksumCandidates();
    }
  }

  rescan(): void {
    if (this.page === 'disk') {
      void this.scanDisk();
    } else if (this.page === 'duplicates') {
      void this.scanDuplicates();
    } else if (this.page === 'checksums') {
      void this.computeChecksums();
    }
  }

  onOperationFilterChange(event: Event): void {
    const val = (event.target as HTMLSelectElement).value as OperationFilter;
    this.operationFilter = val;
    this.cdr.markForCheck();
  }

  onDiskFilterInput(event: Event): void {
    this.diskFilter = (event.target as HTMLInputElement).value;
    this.cdr.markForCheck();
  }

  onDiskSortChange(event: Event): void {
    this.diskSort = (event.target as HTMLSelectElement).value as DiskSort;
    this.cdr.markForCheck();
  }

  onDuplicateMinSizeInput(event: Event): void {
    this.duplicateMinSize = Number((event.target as HTMLInputElement).value);
    this.cdr.markForCheck();
  }

  onDuplicateHiddenChange(event: Event): void {
    this.duplicateIncludeHidden = (event.target as HTMLInputElement).checked;
    this.cdr.markForCheck();
  }

  onDuplicateSortChange(event: Event): void {
    this.duplicateSort = (event.target as HTMLSelectElement).value as DuplicateSort;
    this.cdr.markForCheck();
  }

  onDuplicateFilterInput(event: Event): void {
    this.duplicateFilter = (event.target as HTMLInputElement).value;
    this.cdr.markForCheck();
  }

  onChecksumCandidateSelect(event: Event): void {
    const val = (event.target as HTMLSelectElement).value;
    this.updateChecksumTarget(val);
    if (val) {
      void this.computeChecksums();
    }
    this.cdr.markForCheck();
  }

  onChecksumPathInput(event: Event): void {
    this.updateChecksumTarget((event.target as HTMLInputElement).value);
    this.cdr.markForCheck();
  }

  onExpectedDigestInput(event: Event): void {
    this.expectedDigest = (event.target as HTMLInputElement).value;
    this.cdr.markForCheck();
  }

  async loadChecksumCandidates(): Promise<void> {
    if (!this.rootPath) return;
    try {
      const root = this.rootPath;
      const listing = await this.api.listDirectory(root, false);
      if (root !== this.rootPath) return;
      this.checksumCandidates = (listing.entries ?? [])
        .filter(entry => entry.type !== FileType.DIRECTORY)
        .slice(0, 200);
      if (!this.checksumTargetPath && this.checksumCandidates.length > 0) {
        this.checksumTargetPath = this.checksumCandidates[0].path;
      }
      this.cdr.markForCheck();
    } catch {
      // Directory listing may fail for virtual schemes; user can still paste a path.
    }
  }

  private updateChecksumTarget(path: string): void {
    if (path === this.checksumTargetPath) return;
    this.checksumRequest++;
    this.checksumTargetPath = path;
    this.checksumComputing = false;
    this.checksumResults = [];
    this.checksumFileSize = null;
    this.checksumError = '';
    this.expectedDigest = '';
  }

  async computeChecksums(): Promise<void> {
    const target = this.checksumTargetPath.trim();
    if (!target || this.checksumComputing) return;
    const request = ++this.checksumRequest;

    this.checksumComputing = true;
    this.checksumError = '';
    this.checksumResults = [];
    this.checksumFileSize = null;
    this.cdr.markForCheck();

    try {
      const data = await this.api.getFileChecksums(target);
      if (request !== this.checksumRequest) return;
      const algorithms = [
        ['sha256', 'SHA-256', data.algorithms?.sha256 || data.hash],
        ['sha512', 'SHA-512', data.algorithms?.sha512],
        ['sha1', 'SHA-1', data.algorithms?.sha1],
        ['md5', 'MD5', data.algorithms?.md5],
      ] as const;
      this.checksumResults = algorithms.flatMap(([id, label, value]) => value ? [{ id, label, value }] : []);
      if (!this.checksumResults.length) throw new Error('No checksum was returned.');
      this.checksumFileSize = data.size ?? null;
    } catch (error) {
      if (request === this.checksumRequest) this.checksumError = error instanceof Error ? error.message : String(error);
    } finally {
      if (request === this.checksumRequest) {
        this.checksumComputing = false;
        this.cdr.markForCheck();
      }
    }
  }

  async copyPath(path: string): Promise<void> {
    await this.copyText(path);
  }

  get checksumReport(): string {
    return this.checksumResults.map(({label, value}) => label + ': ' + value).join('\n');
  }

  copyChecksumReport(): Promise<void> {
    return this.copyText(this.checksumReport);
  }

  async copyDigest(value: string): Promise<void> {
    await this.copyText(value);
  }

  private async copyText(value: string): Promise<void> {
    this.clipboardError = '';
    try {
      await navigator.clipboard.writeText(value);
      this.copiedValue = value;
      if (this.copiedTimer !== null) window.clearTimeout(this.copiedTimer);
      this.copiedTimer = window.setTimeout(() => {
        this.copiedValue = '';
        this.copiedTimer = null;
        this.cdr.markForCheck();
      }, 1800);
      this.cdr.markForCheck();
    } catch {
      this.clipboardError = 'Clipboard unavailable.';
      this.cdr.markForCheck();
    }
  }

  async scanDisk(): Promise<void> {
    if (!this.rootPath || this.diskScanning) return;

    this.cancelDisk();
    const controller = new AbortController();
    this.diskAbort = controller;
    this.diskScanning = true;
    this.diskCancelled = false;
    this.diskError = '';
    this.diskEntries = [];
    this.diskProgress = null;
    this.cdr.markForCheck();

    try {
      for await (const item of this.api.streamDiskUsage(
        this.rootPath,
        { signal: controller.signal },
      )) {
        if (controller.signal.aborted) return;
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
      if (!controller.signal.aborted) {
        this.diskError = error instanceof Error ? error.message : String(error);
      }
    } finally {
      if (this.diskAbort === controller) {
        this.diskScanning = false;
        this.diskAbort = null;
        this.cdr.markForCheck();
      }
    }
  }

  cancelDisk(): void {
    if (this.diskAbort) this.diskCancelled = true;
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

    const controller = new AbortController();
    this.duplicateAbort = controller;
    this.duplicateScanning = true;
    this.duplicateProgress = null;
    this.duplicateResult = null;
    this.duplicateSelected = new Set();
    this.duplicateError = '';
    this.cdr.markForCheck();

    try {
      const result = await this.duplicates.scan(
        this.rootPath,
        {
          includeHidden: this.duplicateIncludeHidden,
          signal: controller.signal,
          minSize: Math.max(0, Number(this.duplicateMinSize) || 0),
        },
        progress => {
          if (controller.signal.aborted) return;
          this.duplicateProgress = progress;
          this.cdr.markForCheck();
        },
      );
      if (controller.signal.aborted) return;
      this.duplicateResult = result;
    } catch (error) {
      if (!controller.signal.aborted) this.duplicateError = error instanceof Error ? error.message : String(error);
    } finally {
      if (this.duplicateAbort === controller) {
        this.duplicateAbort = null;
        this.duplicateScanning = false;
        this.cdr.markForCheck();
      }
    }
  }

  cancelDuplicates(): void {
    this.duplicateAbort?.abort();
    this.duplicateAbort = null;
    this.duplicateScanning = false;
    this.duplicateProgress = null;
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
    const parent = parentPath(path);
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
    if (this.duplicateDeleting) return;
    this.duplicateDeleting = true;
    this.cdr.markForCheck();
    try {
      await this.confirmAndDeleteDuplicates();
    } finally {
      this.duplicateDeleting = false;
      this.cdr.markForCheck();
    }
  }

  private async confirmAndDeleteDuplicates(): Promise<void> {
    const scan = this.duplicateResult;
    if (!scan) return;
    const paths = this.duplicates.suggestedRedundantPaths(scan.groups).filter(path => this.duplicateSelected.has(path));
    if (!paths.length) return;

    const visiblePaths = new Set(this.duplicateGroups.flatMap(group => group.files.map(file => file.path)));
    const hidden = paths.filter(path => !visiblePaths.has(path)).length;
    const hiddenMessage = hidden ? ` ${hidden} selected ${hidden === 1 ? 'file is' : 'files are'} outside the current filter.` : '';
    const confirmed = await this.confirm.confirm({
      title: 'Move Duplicate Files to Trash',
      message:
        `Move ${paths.length} selected duplicate file${
          paths.length === 1 ? '' : 's'
        } to Trash? The suggested keeper in each group is protected from selection.${hiddenMessage}`,
      confirmText: 'Move to Trash',
      danger: true,
    });
    if (!confirmed || !this.visible || this.duplicateResult !== scan) return;

    const root = this.rootPath;
    try {
      const result = await this.api.batchDelete(paths, false, true);

      if (this.rootPath === root) {
        this.duplicateSelected = new Set();
        await this.scanDuplicates();
      }
      if (result.failed > 0) {
        await this.confirm.alert({
          title: 'Duplicate Delete Completed with Errors',
          message:
            `${result.failed} file${result.failed === 1 ? '' : 's'} could not be deleted.`,
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
