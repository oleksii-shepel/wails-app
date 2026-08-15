import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, Input, OnChanges } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../../services/api.service';
import { ConfirmDialogService } from '../../services/confirm-dialog.service';
import { DuplicateFinderService, DuplicateGroup, DuplicateKeeperPolicy, DuplicateScanProgress, DuplicateScanResult } from '../../services/duplicate-finder.service';
import { WorkspaceService } from '../../services/workspace.service';
import { SurfaceContext } from '../surface-context';

@Component({
  selector: 'app-duplicate-finder-surface-tab',
  standalone: true,
  imports: [CommonModule, FormsModule],
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
          <input class="filter" type="search" placeholder="Filter groups…" [(ngModel)]="filterText" [disabled]="scanning">
          <button (click)="selectSuggested()" [disabled]="!result?.groups?.length || scanning">Select Redundant</button>
          <button (click)="deleteSelected()" [disabled]="selected.size === 0 || scanning">Delete {{ selected.size }}</button>
        </div>
      </header>

      <div class="summary" *ngIf="progress || result">
        <span *ngIf="progress">{{ progress.phase }} · {{ progress.discoveredFiles }} files · {{ progress.hashedFiles }}/{{ progress.candidateFiles }} hashed</span>
        <span *ngIf="result">{{ result.groups.length }} groups · {{ result.scannedFiles }} files · {{ formatSize(result.reclaimableBytes) }} reclaimable</span>
        <span class="warning" *ngIf="result?.truncated">scan limit reached</span>
      </div>

      <div class="empty" *ngIf="!scanning && result && result.groups.length === 0">No verified duplicates found.</div>

      <section class="group" *ngFor="let group of visibleGroups; trackBy: trackGroup">
        <div class="group-head">
          <strong><span class="type">{{ groupIcon(group) }}</span>{{ group.files.length }} copies</strong>
          <span>{{ formatSize(group.size) }} each · {{ formatSize(group.reclaimableBytes) }} reclaimable</span>
          <code>{{ group.hash.slice(0, 12) }}…</code>
        </div>
        <div class="file" *ngFor="let file of group.files">
          <input type="checkbox" [checked]="selected.has(file.path)" [disabled]="file.path === keeperPath(group)" (change)="toggle(file.path)">
          <span class="keeper" *ngIf="file.path === keeperPath(group)">KEEP</span>
          <span class="path" [title]="file.path">{{ file.path }}</span>
          <span class="mtime">{{ file.modified * 1000 | date:'short' }}</span>
          <button (click)="preview(file.path, file.name)">Preview</button>
        </div>
      </section>

      <div class="warnings" *ngIf="result?.warnings?.length">
        <strong>Warnings</strong>
        <div *ngFor="let warning of result?.warnings">{{ warning }}</div>
      </div>
    </div>
  `,
  styles: [`
    :host,.duplicates{display:flex;flex-direction:column;min-height:0;height:100%;background:var(--vsc-bg,#1e1e1e);color:var(--vsc-fg,#ddd)}
    header{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:10px 12px;border-bottom:1px solid var(--vsc-border,#333)}
    .root{font-size:12px;opacity:.7;max-width:700px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.actions label{font-size:12px;display:flex;gap:5px;align-items:center}.actions input[type=number]{width:90px}.actions select{max-width:130px}.filter{width:150px}.type{margin-right:5px}
    button{background:var(--vsc-button-bg,#2d2d30);color:inherit;border:1px solid var(--vsc-border,#444);padding:4px 8px;border-radius:3px}button:disabled{opacity:.45}.summary{display:flex;gap:14px;padding:7px 12px;border-bottom:1px solid var(--vsc-border,#333);font-size:12px}.warning{color:#d7ba7d}.group{margin:8px 12px;border:1px solid var(--vsc-border,#3a3a3a);border-radius:4px;overflow:hidden}.group-head{display:flex;gap:12px;align-items:center;padding:7px 9px;background:rgba(127,127,127,.08);font-size:12px}.group-head code{margin-left:auto;opacity:.65}.file{display:grid;grid-template-columns:22px 45px minmax(120px,1fr) 150px auto;gap:8px;align-items:center;padding:5px 9px;border-top:1px solid rgba(127,127,127,.12);font-size:12px}.path{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.keeper{font-size:10px;color:#4ec9b0}.mtime{opacity:.65}.empty,.warnings{padding:14px}.warnings{color:#d7ba7d;font-size:12px;overflow:auto}
  `],
})
export class DuplicateFinderSurfaceTabComponent implements OnChanges {
  @Input({ required: true }) ctx!: SurfaceContext;
  @Input() active = false;
  rootPath = '';
  minSize = 1;
  includeHidden = false;
  scanning = false;
  progress: DuplicateScanProgress | null = null;
  result: DuplicateScanResult | null = null;
  selected = new Set<string>();
  keeperPolicy: DuplicateKeeperPolicy = 'oldest';
  sortMode: 'wasted' | 'copies' | 'size' | 'path' = 'wasted';
  filterText = '';
  private loadKey = '';

  constructor(
    private readonly finder: DuplicateFinderService,
    private readonly api: ApiService,
    private readonly confirm: ConfirmDialogService,
    private readonly workspace: WorkspaceService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(): void {
    const key = this.ctx?.tab?.path ?? '';
    if (key === this.loadKey) return;
    this.loadKey = key;
    this.rootPath = this.parseRoot(key);
    this.result = null;
    this.selected.clear();
    if (this.rootPath) void this.scan();
  }

  async scan(): Promise<void> {
    if (!this.rootPath || this.scanning) return;
    this.scanning = true; this.progress = null; this.selected.clear(); this.cdr.markForCheck();
    try {
      this.result = await this.finder.scan(this.rootPath, { includeHidden: this.includeHidden, minSize: this.minSize }, p => {
        this.progress = p; this.cdr.markForCheck();
      });
    } catch (error) {
      await this.confirm.alert(`Duplicate Scan Failed\n${error instanceof Error ? error.message : String(error)}`);
    } finally {
      this.scanning = false; this.progress = null; this.cdr.markForCheck();
    }
  }

  selectSuggested(): void {
    this.selected = new Set(this.finder.redundantPaths(this.result?.groups ?? [], this.keeperPolicy));
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

  async deleteSelected(): Promise<void> {
    const paths = Array.from(this.selected);
    if (!paths.length) return;
    const bytes = this.selectedBytes();
    const ok = await this.confirm.confirm({ title: 'Remove Duplicate Files', message: `Move ${paths.length} selected duplicate file(s) to Trash?\nPotentially reclaim ${this.formatSize(bytes)}.`, confirmText: 'Move to Trash', danger: true });
    if (!ok) return;
    const result = await this.api.batchDelete(paths, false, true);
    if (result.failed > 0) await this.confirm.alert(`Duplicate Cleanup Completed With Errors\n${result.failed} item(s) could not be removed.`);
    await this.scan();
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
