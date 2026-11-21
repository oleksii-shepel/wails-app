import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, Input, OnChanges, OnDestroy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../../services/api.service';
import { ConfirmDialogService } from '../../services/confirm-dialog.service';
import { DuplicateFinderService, DuplicateGroup, DuplicateKeeperPolicy, DuplicateScanProgress, DuplicateScanResult } from '../../services/duplicate-finder.service';
import { WorkspaceService } from '../../services/workspace.service';
import { SelectionBasketService } from '../../services/selection-basket.service';
import { ResultsPanelService } from '../../services/results-panel.service';
import { SurfaceContext } from '../surface-context';
import { LocalDateTimePipe } from '../../utils/local-date-time.pipe';

@Component({
  selector: 'app-duplicate-finder-surface-tab',
  standalone: true,
  imports: [LocalDateTimePipe, CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="duplicates">
      <header>
        <div>
          <strong>Duplicate Finder</strong>
          <div class="root" [title]="rootPath">{{ rootPath }}</div>
        </div>
        <div class="actions">
          <label>Min size <input type="number" min="0" [(ngModel)]="minSize" [disabled]="scanning"></label>
          <label><input type="checkbox" [(ngModel)]="includeHidden" [disabled]="scanning"> Hidden</label>
          <button (click)="scan()" [disabled]="scanning || !rootPath">{{ scanning ? 'Scanning…' : 'Scan' }}</button>
          <button type="button" *ngIf="scanning" (click)="cancelScan()">Cancel scan</button>
          <label>Keep
            <select [(ngModel)]="keeperPolicy" [disabled]="scanning" (ngModelChange)="onKeeperPolicyChange()">
              <option value="oldest">Oldest</option>
              <option value="newest">Newest</option>
              <option value="shortest-path">Shortest path</option>
            </select>
          </label>
          <label>Sort
            <select [(ngModel)]="sortMode" [disabled]="scanning">
              <option value="wasted">Wasted space</option>
              <option value="copies">Copies</option>
              <option value="size">File size</option>
              <option value="path">Path</option>
            </select>
          </label>
          <input class="filter" type="search" placeholder="Filter groups…" aria-label="Filter duplicate groups" [(ngModel)]="filterText" [disabled]="scanning">
          <button (click)="selectSuggested()" [disabled]="!result?.groups?.length || scanning">Select Matching Copies</button>
          <button (click)="openResultsPanel()" [disabled]="!result?.groups?.length || scanning">Open Results Panel</button>
          <button (click)="collectSelected()" [disabled]="selected.size === 0 || scanning">Collect {{ selected.size }}</button>
          <button (click)="deleteSelected()" [disabled]="selected.size === 0 || scanning || deleting">Trash {{ selected.size }}</button>
        </div>
      </header>

      <div class="summary" *ngIf="progress || result">
        <span *ngIf="progress">{{ progress.phase }} · {{ progress.discoveredFiles }} files · {{ progress.hashedFiles }}/{{ progress.candidateFiles }} hashed</span>
        <span *ngIf="result">{{ result.groups.length }} groups · {{ result.scannedFiles }} files · {{ formatSize(result.reclaimableBytes) }} reclaimable</span>
        <span class="warning" *ngIf="result?.truncated">scan limit reached</span>
      </div>

      <div class="empty" *ngIf="!scanning && result && result.groups.length === 0" role="status">No verified duplicates found.</div>
      <div class="empty" *ngIf="!scanning && result && result.groups.length > 0 && visibleGroups.length === 0" role="status">No duplicate groups match this filter.</div>

      <section class="group" *ngFor="let group of visibleGroups; trackBy: trackGroup">
        <div class="group-head">
          <strong><span class="type">{{ groupIcon(group) }}</span>{{ group.files.length }} copies</strong>
          <span>{{ formatSize(group.size) }} each · {{ formatSize(group.reclaimableBytes) }} reclaimable</span>
          <code>{{ group.hash.slice(0, 12) }}…</code>
        </div>
        <div class="file" *ngFor="let file of group.files">
          <input type="checkbox" [attr.aria-label]="'Select duplicate: ' + file.path" [checked]="selected.has(file.path)" [disabled]="file.path === keeperPath(group)" (change)="toggle(file.path)">
          <span class="keeper" *ngIf="file.path === keeperPath(group)">KEEP</span>
          <span class="path" [title]="file.path">{{ file.path }}</span>
          <span class="mtime">{{ file.modified | localDateTime }}</span>
          <button (click)="preview(file.path, file.name)">Preview</button>
        </div>
      </section>

      <div class="warnings" *ngIf="result?.warnings?.length">
        <strong>Warnings</strong>
        <div *ngFor="let warning of result.warnings">{{ warning }}</div>
      </div>
    </div>
  `,
  styles: [`
    :host,.duplicates{display:flex;flex-direction:column;min-height:0;height:100%;background:var(--vsc-bg,var(--vsc-editor-background));color:var(--vsc-fg,var(--vsc-foreground))}
    header{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:10px 12px;border-bottom:1px solid var(--vsc-border,var(--vsc-activitybar-background))}
    .root{font-size:12px;opacity:.7;max-width:700px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.actions label{font-size:12px;display:flex;gap:5px;align-items:center}.actions input[type=number]{width:90px}.actions select{max-width:130px}.filter{width:150px}.type{margin-right:5px}
    button{background:var(--vsc-button-bg,var(--vsc-toolbar-background));color:inherit;border:1px solid var(--vsc-border,var(--vsc-border));padding:4px 8px;border-radius:3px}button:disabled{opacity:.45}.summary{display:flex;gap:14px;padding:7px 12px;border-bottom:1px solid var(--vsc-border,var(--vsc-activitybar-background));font-size:12px}.warning{color: var(--vsc-accent-yellow)}.group{margin:8px 12px;border:1px solid var(--vsc-border,var(--vsc-list-active-background));border-radius:4px;overflow:hidden}.group-head{display:flex;gap:12px;align-items:center;padding:7px 9px;background:var(--vsc-neutral-overlay-subtle);font-size:12px}.group-head code{margin-left:auto;opacity:.65}.file{display:grid;grid-template-columns:22px 45px minmax(120px,1fr) 150px auto;gap:8px;align-items:center;padding:5px 9px;border-top:1px solid var(--vsc-neutral-overlay-soft);font-size:12px}.path{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.keeper{font-size:10px;color:var(--vsc-success)}.mtime{opacity:.65}.empty,.warnings{padding:14px}.warnings{color: var(--vsc-accent-yellow);font-size:12px;overflow:auto}
  `],
})
export class DuplicateFinderSurfaceTabComponent implements OnChanges, OnDestroy {
  @Input({ required: true }) ctx!: SurfaceContext;
  @Input() active = false;
  rootPath = '';
  minSize = 1;
  includeHidden = false;
  scanning = false;
  deleting = false;
  progress: DuplicateScanProgress | null = null;
  result: DuplicateScanResult | null = null;
  selected = new Set<string>();
  keeperPolicy: DuplicateKeeperPolicy = 'oldest';
  sortMode: 'wasted' | 'copies' | 'size' | 'path' = 'wasted';
  filterText = '';
  private loadKey = '';
  private scanController: AbortController | null = null;

  constructor(
    private readonly finder: DuplicateFinderService,
    private readonly api: ApiService,
    private readonly confirm: ConfirmDialogService,
    private readonly workspace: WorkspaceService,
    private readonly basket: SelectionBasketService,
    private readonly resultsPanels: ResultsPanelService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(): void {
    const key = this.ctx?.tab?.path ?? '';
    if (key === this.loadKey) return;
    this.cancelScan();
    this.loadKey = key;
    this.rootPath = this.parseRoot(key);
    this.result = null;
    this.selected.clear();
    if (this.rootPath) void this.scan();
  }

  ngOnDestroy(): void { this.cancelScan(); }

  cancelScan(): void {
    this.scanController?.abort();
    this.scanController = null;
    this.scanning = false;
    this.progress = null;
  }

  async scan(): Promise<void> {
    if (!this.rootPath || this.scanning) return;
    const controller = new AbortController();
    this.scanController = controller;
    this.scanning = true;
    this.progress = null;
    this.result = null;
    this.selected.clear();
    this.cdr.markForCheck();
    try {
      const result = await this.finder.scan(this.rootPath, {
        includeHidden: this.includeHidden,
        minSize: this.minSize,
        signal: controller.signal,
      }, progress => {
        if (controller.signal.aborted) return;
        this.progress = progress;
        this.cdr.markForCheck();
      });
      if (!controller.signal.aborted) this.result = result;
    } catch (error) {
      if (!controller.signal.aborted) {
        await this.confirm.alert(`Duplicate Scan Failed\n${error instanceof Error ? error.message : String(error)}`);
      }
    } finally {
      if (this.scanController === controller) {
        this.scanController = null;
        this.scanning = false;
        this.progress = null;
        this.cdr.markForCheck();
      }
    }
  }

  selectSuggested(): void {
    this.selected = new Set(this.finder.redundantPaths(this.visibleGroups, this.keeperPolicy));
    this.cdr.markForCheck();
  }

  onKeeperPolicyChange(): void {
    if (this.selected.size > 0) this.selectSuggested();
    this.cdr.markForCheck();
  }

  get visibleGroups(): DuplicateGroup[] {
    const needle = this.filterText.trim().toLowerCase();
    const groups = [...(this.result?.groups ?? [])].filter(group => !needle || group.files.some(file => file.path.toLowerCase().includes(needle)));
    groups.sort((a, b) => {
      switch (this.sortMode) {
        case 'copies': return b.files.length - a.files.length || b.reclaimableBytes - a.reclaimableBytes;
        case 'size': return b.size - a.size || b.reclaimableBytes - a.reclaimableBytes;
        case 'path': return (a.files[0]?.path ?? '').localeCompare(b.files[0]?.path ?? '');
        default: return b.reclaimableBytes - a.reclaimableBytes || (a.files[0]?.path ?? '').localeCompare(b.files[0]?.path ?? '');
      }
    });
    return groups;
  }

  keeperPath(group: DuplicateGroup): string { return this.finder.keeperPath(group, this.keeperPolicy); }

  groupIcon(group: DuplicateGroup): string {
    const ext = (group.files[0]?.name.split('.').pop() ?? '').toLowerCase();
    if (['jpg','jpeg','png','gif','webp','bmp','tif','tiff','heic','heif','avif'].includes(ext)) return '🖼️';
    if (['mp4','mkv','mov','avi','webm','m4v'].includes(ext)) return '🎬';
    if (['mp3','flac','wav','m4a','aac','ogg'].includes(ext)) return '🎵';
    if (['zip','7z','rar','tar','gz','bz2','xz'].includes(ext)) return '📦';
    if (['pdf','doc','docx','txt','md','rtf'].includes(ext)) return '📄';
    return '📎';
  }

  toggle(path: string): void {
    const next = new Set(this.selected); next.has(path) ? next.delete(path) : next.add(path); this.selected = next;
  }

  openResultsPanel(): void {
    const entries = this.uniqueResultFiles();
    if (entries.length === 0) return;

    const panel = this.resultsPanels.createDuplicateResults(
      this.rootPath,
      entries,
      `Duplicates: ${this.rootPath}`,
    );
    const tab = this.workspace.addTab(this.ctx.paneId, panel.path, true);
    if (tab) {
      this.workspace.updateTabPathAndTitle(this.ctx.paneId, tab.id, panel.path, panel.title);
    }
  }

  private uniqueResultFiles(): import('../../shared/protocol').FileInfo[] {
    const files = new Map<string, import('../../shared/protocol').FileInfo>();
    for (const group of this.visibleGroups) {
      for (const file of group.files) files.set(file.path, { ...file });
    }
    return [...files.values()];
  }

  collectSelected(): void {
    this.basket.add(Array.from(this.selected), 'duplicates');
  }

  async deleteSelected(): Promise<void> {
    const scan = this.result;
    if (this.deleting || this.scanning || !scan) return;
    this.deleting = true;
    this.cdr.markForCheck();
    try {
      await this.confirmAndDelete(scan);
    } catch (error) {
      await this.confirm.alert(`Duplicate Cleanup Failed\n${error instanceof Error ? error.message : String(error)}`);
    } finally {
      this.deleting = false;
      this.cdr.markForCheck();
    }
  }

  private async confirmAndDelete(scan: DuplicateScanResult): Promise<void> {
    const policy = this.keeperPolicy;
    const paths = this.finder.redundantPaths(scan.groups, policy).filter(path => this.selected.has(path));
    if (!paths.length) return;
    const bytes = this.selectedBytes();
    const visiblePaths = new Set(this.visibleGroups.flatMap(group => group.files.map(file => file.path)));
    const hidden = paths.filter(path => !visiblePaths.has(path)).length;
    const hiddenMessage = hidden ? `\n${hidden} selected item(s) are outside the current filter.` : '';
    const ok = await this.confirm.confirm({ title: 'Remove Duplicate Files', message: `Move ${paths.length} selected duplicate file(s) to Trash?\nPotentially reclaim ${this.formatSize(bytes)}.${hiddenMessage}`, confirmText: 'Move to Trash', danger: true });
    if (!ok || this.result !== scan || this.keeperPolicy !== policy) return;
    const result = await this.api.batchDelete(paths, false, true);
    if (result.failed > 0) await this.confirm.alert(`Duplicate Cleanup Completed With Errors\n${result.failed} item(s) could not be removed.`);
    if (this.result === scan) await this.scan();
  }

  preview(path: string, name: string): void {
    this.workspace.addViewerTab(this.ctx.paneId, path, { activate: true, title: name, autoFollow: false });
  }

  trackGroup(_index: number, group: DuplicateGroup): string { return `${group.size}:${group.hash}`; }
  formatSize(bytes: number): string {
    if (!bytes) return '0 B'; const units = ['B','KB','MB','GB','TB']; const i = Math.min(Math.floor(Math.log(bytes)/Math.log(1024)), units.length-1); const value = bytes/Math.pow(1024,i); return `${i === 0 ? value : value.toFixed(1)} ${units[i]}`;
  }

  private selectedBytes(): number {
    let total = 0; for (const group of this.result?.groups ?? []) for (const file of group.files) if (this.selected.has(file.path)) total += file.size; return total;
  }
  private parseRoot(location: string): string {
    try { const url = new URL(location); return url.searchParams.get('path') ?? ''; } catch { return ''; }
  }
}