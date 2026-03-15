import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  EventEmitter,
  Input,
  OnChanges,
  OnDestroy,
  Output,
  SimpleChanges,
} from '@angular/core';
import { ApiService } from '../../services/api.service';
import type { DiskUsageEntry, DiskUsageResult } from '../../shared/protocol-enhanced';
import type { SizeManagerSurfaceContext } from '../surface-context';
import { DiskPieChartComponent } from 'src/app/components/disk-usage-diagram.component';

// ── Pie nav frame ─────────────────────────────────────────────────────────────

interface PieFrame {
  label: string;
  entries: DiskUsageEntry[];
  /**
   * Denominator for % calculations at this level.
   * Root: directory totalSize.
   * Sub-levels: sum of the Other bucket that was drilled into.
   */
  totalSize: number;
}

// ─────────────────────────────────────────────────────────────────────────────

@Component({
  selector: 'app-size-manager-surface',
  standalone: true,
  imports: [CommonModule, DiskPieChartComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <!-- ── Header bar ─────────────────────────────────────── -->
    <div class="sm-header">
      <div class="sm-header-left">
        <span class="sm-kicker">Disk Usage</span>
        <span class="sm-sep">·</span>
        <span class="sm-current-label">{{ currentLabel }}</span>
        <span class="sm-path" [title]="currentPath">{{ currentPath }}</span>
      </div>
      <div class="sm-header-actions">
        <span
          class="sm-header-activity"
          *ngIf="isTraversing"
          [attr.aria-label]="headerProgressLabel"
          [title]="headerProgressLabel"
          role="status">
          <span class="sm-header-activity-dot"></span>
        </span>
        <button class="sm-btn" [disabled]="!parentPath" (click)="goUp()" title="Go to parent">↑ Parent</button>
        <button class="sm-btn" (click)="refresh()">↺ Refresh</button>
        <button class="sm-btn sm-btn--close" (click)="close()" title="Close">✕</button>
      </div>
    </div>

    <!-- ── Loading ────────────────────────────────────────── -->
    <div class="sm-loading" *ngIf="isTraversing && !hasVisibleContent">
      <div class="sm-spinner"></div>
      <span>Scanning <strong>{{ currentLabel }}</strong>…</span>
    </div>

    <!-- ── Error ──────────────────────────────────────────── -->
    <div class="sm-error" *ngIf="error && !isTraversing">
      <span class="sm-error-icon">⚠</span>
      <span>{{ error }}</span>
      <button class="sm-btn sm-btn--primary" (click)="refresh()">Retry</button>
    </div>

    <!-- ── Dashboard ──────────────────────────────────────── -->
    <div class="sm-body" *ngIf="hasVisibleContent">

      <div class="sm-inline-status" *ngIf="isTraversing">
        <div class="sm-spinner"></div>
        <span>Scanning <strong>{{ currentLabel }}</strong>… showing live results.</span>
      </div>

      <!-- Metric strip -->
      <div class="sm-metrics">
        <div class="sm-metric">
          <div class="sm-metric-val">{{ totalSizeLabel }}</div>
          <div class="sm-metric-lbl">{{ totalSizeMetricLabel }}</div>
        </div>
        <div class="sm-metric">
          <div class="sm-metric-val">{{ totalItems | number }}</div>
          <div class="sm-metric-lbl">{{ itemMixLabel }}</div>
        </div>
        <div class="sm-metric">
          <div class="sm-metric-val">{{ averageEntrySizeLabel }}</div>
          <div class="sm-metric-lbl">Avg child size</div>
        </div>
        <div class="sm-metric">
          <div class="sm-metric-val">{{ elapsedLabel }}</div>
          <div class="sm-metric-lbl">{{ elapsedMetricLabel }}</div>
        </div>
      </div>

      <!-- ── Pie panel ──────────────────────────────────────── -->
      <div class="sm-panel">

        <!-- Breadcrumb nav — only visible when drilled into Other -->
        <div class="sm-pie-nav" *ngIf="pieStack.length > 1">
          <button
            *ngFor="let frame of pieStack; let i = index; let last = last"
            class="sm-pie-crumb"
            [class.sm-pie-crumb--active]="last"
            [disabled]="last"
            (click)="pieGoTo(i)">{{ frame.label }}</button>
        </div>

        <!-- Pure display chart — receives only the current frame's data -->
        <app-disk-pie-chart
          [entries]="currentPieFrame.entries"
          [totalSize]="currentPieFrame.totalSize"
          [minPct]="10"
          (select)="onEntryClick($event)"
          (otherDrill)="onOtherDrill($event)"
          (drillDown)="onEntryDblClick($event)">
        </app-disk-pie-chart>

      </div>

      <!-- ── Breakdown table ──────────────────────────────── -->
      <div class="sm-panel sm-breakdown">
        <div class="sm-panel-head">
          <span class="sm-panel-title">Breakdown</span>
          <span class="sm-panel-hint">Single-click to inspect · double-click to drill into folders</span>
        </div>

        <div class="sm-table-head" *ngIf="entries.length > 0">
          <span>Name</span>
          <span>Composition</span>
          <span class="sm-align-r">Size</span>
          <span class="sm-align-r">Share</span>
        </div>

        <div class="sm-table-body" *ngIf="entries.length > 0; else emptyBreakdown">
          <button
            class="sm-row"
            *ngFor="let e of entries; trackBy: trackEntry"
            [class.sm-row--dir]="e.isDir"
            [class.sm-row--active]="selectedEntry?.path === e.path"
            (click)="onEntryClick(e)"
            (dblclick)="onEntryDblClick(e)">
            <div class="sm-row-name">
              <span class="sm-row-icon" [class.sm-row-icon--dir]="e.isDir">{{ e.isDir ? '▣' : '•' }}</span>
              <span class="sm-row-label" [title]="e.path">{{ e.name }}</span>
            </div>
            <span class="sm-row-meta">{{ entryComposition(e) }}</span>
            <span class="sm-row-size sm-align-r">{{ formatBytes(e.size) }}</span>
            <div class="sm-row-pct-cell sm-align-r">
              <span class="sm-row-pct">{{ pct(e) }}</span>
              <div class="sm-row-track">
                <div class="sm-row-bar" [style.width.%]="barPercent(e)"></div>
              </div>
            </div>
          </button>
        </div>

        <ng-template #emptyBreakdown>
          <div class="sm-empty">Folder is empty.</div>
        </ng-template>
      </div>
    </div>
  `,
  styles: [`
    :host {
      --_bg:         var(--vsc-editor-background,    #1e1e1e);
      --_surface:    var(--vsc-panel-background,     #252526);
      --_surface2:   var(--vsc-list-hover-background,#2a2d2e);
      --_input-bg:   var(--vsc-input-background,     #3c3c3c);
      --_text:       var(--vsc-foreground,           #cccccc);
      --_text-dim:   var(--vsc-foreground-dim,       #888888);
      --_border:     var(--vsc-border,               #454545);
      --_border-sub: var(--vsc-border-subtle,        #3a3a3a);
      --_focus:      var(--vsc-input-focus-border,   #007acc);
      --_btn-bg:     var(--vsc-button-background,    #0e639c);
      --_btn-hover:  var(--vsc-button-hover,         #1177bb);
      --_hover-bg:   var(--vsc-list-hover-background,#2a2d2e);
      --_active-bg:  var(--vsc-list-active-background,#094771);
      --_accent-a:   #2ec4b6;
      --_accent-b:   #3a86ff;
      --_error:      var(--vsc-error,                #f48771);
      --_blue:       var(--vsc-accent-blue,          #007acc);
      --_icon-file:  #8fd3ff;
      --_icon-dir:   #dcb67a;

      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
      font-size: var(--vsc-font-size, 13px);
      font-family: var(--vsc-font-family, system-ui, sans-serif);
      color: var(--_text);
      background: var(--_bg);
    }

    :host-context(.vscode-light) {
      --_bg: #ffffff; --_surface: #f3f3f3; --_surface2: #e4e6f1;
      --_input-bg: #ffffff; --_text: #3b3b3b; --_text-dim: #737373;
      --_border: #c4c4c4; --_border-sub: #e5e5e5; --_focus: #007acc;
      --_btn-bg: #007acc; --_btn-hover: #005ba1;
      --_hover-bg: #e4e6f1; --_active-bg: #cce0f5;
      --_error: #c7372f; --_blue: #007acc;
      --_accent-a: #0fa89e; --_accent-b: #1a6fd4;
      --_icon-file: #1a6fd4; --_icon-dir: #a0700a;
    }

    .sm-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 14px; flex-shrink: 0; border-bottom: 1px solid var(--_border-sub); background: var(--_surface); }
    .sm-header-left { display: flex; align-items: center; gap: 8px; min-width: 0; overflow: hidden; }
    .sm-kicker { font-size: 10px; text-transform: uppercase; letter-spacing: .14em; color: var(--_text-dim); flex-shrink: 0; }
    .sm-sep { color: var(--_text-dim); flex-shrink: 0; }
    .sm-header-activity { display: inline-flex; align-items: center; justify-content: center; width: 18px; height: 18px; flex-shrink: 0; border-radius: 999px; background: color-mix(in srgb, #2bb24c 14%, transparent); border: 1px solid color-mix(in srgb, #2bb24c 35%, var(--_border)); }
    .sm-header-activity-dot { width: 10px; height: 10px; border-radius: 999px; background: #2bb24c; border: 1px solid rgba(255, 255, 255, 0.35); box-shadow: 0 0 0 0 rgba(43, 178, 76, 0.45); animation: sm-pulse-green 1.3s ease-out infinite; }
    .sm-current-label { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; flex-shrink: 0; max-width: 200px; }
    .sm-path { font-size: 11px; color: var(--_text-dim); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0; }
    .sm-header-actions { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }

    .sm-btn { appearance: none; border: 1px solid var(--_border); border-radius: var(--vsc-border-radius-lg, 4px); padding: 4px 10px; font-size: 12px; font-weight: 500; font-family: inherit; cursor: pointer; color: var(--_text); background: transparent; transition: background .1s, border-color .1s; white-space: nowrap; }
    .sm-btn:hover:not(:disabled) { background: var(--_hover-bg); }
    .sm-btn:disabled { opacity: .35; cursor: default; }
    .sm-btn--primary { background: var(--_btn-bg); color: #fff; border-color: transparent; }
    .sm-btn--primary:hover:not(:disabled) { background: var(--_btn-hover); }
    .sm-btn--close { padding: 4px 8px; }

    .sm-loading { display: flex; align-items: center; gap: 10px; padding: 16px 14px; flex-shrink: 0; color: var(--_text-dim); }
    .sm-spinner { width: 14px; height: 14px; border: 2px solid var(--_border); border-top-color: var(--_blue); border-radius: 50%; animation: sm-spin .6s linear infinite; flex-shrink: 0; }
    @keyframes sm-spin { to { transform: rotate(360deg); } }
    @keyframes sm-pulse-green {
      0% { box-shadow: 0 0 0 0 rgba(43, 178, 76, 0.45); }
      70% { box-shadow: 0 0 0 8px rgba(43, 178, 76, 0); }
      100% { box-shadow: 0 0 0 0 rgba(43, 178, 76, 0); }
    }

    .sm-error { display: flex; align-items: center; gap: 10px; padding: 12px 14px; flex-shrink: 0; color: var(--_error); border-bottom: 1px solid color-mix(in srgb, var(--_error) 25%, transparent); background: color-mix(in srgb, var(--_error) 8%, var(--_surface)); }
    .sm-error-icon { font-size: 15px; flex-shrink: 0; }

    .sm-body { display: flex; flex-direction: column; flex: 1 1 0%; min-height: 0; overflow: auto; padding: 12px 14px; gap: 12px; scrollbar-width: thin; scrollbar-color: var(--vsc-scrollbar-thumb, #424242) transparent; }
    .sm-inline-status { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-radius: var(--vsc-border-radius-lg, 4px); border: 1px solid var(--_border-sub); background: color-mix(in srgb, var(--_surface2) 65%, transparent); color: var(--_text-dim); }

    .sm-metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 8px; flex-shrink: 0; }
    .sm-metric { padding: 10px 12px; border-radius: var(--vsc-border-radius-lg, 4px); border: 1px solid var(--_border-sub); background: var(--_surface); }
    .sm-metric-val { font-size: 18px; font-weight: 700; letter-spacing: -.03em; font-variant-numeric: tabular-nums; line-height: 1.1; }
    .sm-metric-lbl { margin-top: 3px; font-size: 11px; color: var(--_text-dim); }

    .sm-panel { display: flex; flex-direction: column; gap: 10px; padding: 12px 14px; border-radius: var(--vsc-border-radius-lg, 4px); border: 1px solid var(--_border-sub); background: var(--_surface); min-width: 0; flex-shrink: 0; }
    .sm-panel-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
    .sm-panel-title { font-size: 10px; text-transform: uppercase; letter-spacing: .12em; color: var(--_text-dim); }
    .sm-panel-hint  { font-size: 11px; color: var(--_text-dim); }

    /* Pie breadcrumb nav */
    .sm-pie-nav { display: flex; align-items: center; flex-wrap: wrap; }
    .sm-pie-crumb { appearance: none; border: none; background: none; font-family: inherit; font-size: 10px; text-transform: uppercase; letter-spacing: .12em; color: var(--_text-dim); cursor: pointer; padding: 0; transition: color .12s; }
    .sm-pie-crumb:hover:not(:disabled) { color: var(--_text); text-decoration: underline; }
    .sm-pie-crumb--active { color: var(--_text); cursor: default; }
    .sm-pie-crumb:not(:last-child)::after { content: ' › '; color: var(--_text-dim); margin: 0 4px; }

    /* Breakdown */
    .sm-breakdown { gap: 8px; }
    .sm-table-head { display: grid; grid-template-columns: minmax(0,2fr) minmax(100px,1fr) 90px 110px; gap: 10px; padding: 0 8px 6px; font-size: 10px; text-transform: uppercase; letter-spacing: .12em; color: var(--_text-dim); border-bottom: 1px solid var(--_border-sub); }
    .sm-table-body { display: flex; flex-direction: column; gap: 1px; }
    .sm-row { appearance: none; border: 1px solid transparent; border-radius: var(--vsc-border-radius-lg, 4px); display: grid; grid-template-columns: minmax(0,2fr) minmax(100px,1fr) 90px 110px; gap: 10px; align-items: center; padding: 6px 8px; cursor: pointer; color: var(--_text); background: transparent; text-align: left; transition: background .1s; width: 100%; min-width: 0; font-family: inherit; }
    .sm-row:hover { background: var(--_hover-bg); }
    .sm-row--active { background: color-mix(in srgb, var(--_focus) 10%, var(--_surface2) 90%); border-color: color-mix(in srgb, var(--_focus) 35%, transparent); }
    .sm-row-name { display: flex; align-items: center; gap: 8px; min-width: 0; }
    .sm-row-icon { flex-shrink: 0; font-size: 11px; color: var(--_icon-file); }
    .sm-row-icon--dir { color: var(--_icon-dir); }
    .sm-row-label { font-weight: 500; font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .sm-row-meta  { font-size: 11px; color: var(--_text-dim); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .sm-row-size  { font-size: 12px; font-variant-numeric: tabular-nums; white-space: nowrap; }
    .sm-row-pct-cell { display: flex; flex-direction: column; gap: 3px; align-items: flex-end; }
    .sm-row-pct   { font-size: 11px; font-variant-numeric: tabular-nums; color: var(--_text-dim); }
    .sm-row-track { width: 100%; height: 3px; border-radius: 999px; background: var(--_input-bg); border: 1px solid var(--_border-sub); overflow: hidden; }
    .sm-row-bar   { height: 100%; border-radius: inherit; background: linear-gradient(90deg, var(--_accent-a), var(--_accent-b)); transition: width .5s cubic-bezier(.4,0,.2,1); }

    .sm-align-r { text-align: right; }
    .sm-empty { display: flex; align-items: center; justify-content: center; min-height: 72px; font-size: 12px; color: var(--_text-dim); }

    @media (max-width: 640px) {
      .sm-table-head { display: none; }
      .sm-row { grid-template-columns: minmax(0,1fr) auto; }
      .sm-row-meta { display: none; }
    }
  `],
})
export class SizeManagerSurfaceComponent implements OnChanges, OnDestroy {
  @Input({ required: true }) ctx!: SizeManagerSurfaceContext;
  @Output() firstEntryArrived = new EventEmitter<void>();

  loading = false;
  isTraversing = false;
  error: string | null = null;
  result: DiskUsageResult | null = null;
  entries: DiskUsageEntry[] = [];
  selectedEntry: DiskUsageEntry | null = null;
  currentPath = '';
  parentPath: string | null = null;

  // ── Pie nav stack — owned here, not in the chart ──────────────────────────

  pieStack: PieFrame[] = [];

  get currentPieFrame(): PieFrame {
    return this.pieStack[this.pieStack.length - 1] ?? { label: '', entries: [], totalSize: 0 };
  }

  private destroyed = false;

  constructor(
    private api: ApiService,
    private cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['ctx']) {
      const path = this.ctx?.sizeVm?.analysisPath;
      if (path && path !== this.currentPath) this.analyse(path);
    }
  }

  ngOnDestroy(): void { this.destroyed = true; }

  // ── Actions ───────────────────────────────────────────────────────────────

  close():   void { this.ctx?.sizeVm?.onClose(); }
  goUp():    void { if (this.parentPath) this.analyse(this.parentPath); }
  refresh(): void { if (this.currentPath) this.analyse(this.currentPath); }

  onEntryClick(e: DiskUsageEntry): void {
    this.selectedEntry = e;
    this.cdr.markForCheck();
  }

  onEntryDblClick(e: DiskUsageEntry): void {
    if (e.isDir) this.analyse(e.path);
  }

  /** Chart emits bucketed Other entries — we push a new pie frame. */
  onOtherDrill(otherEntries: DiskUsageEntry[]): void {
    const depth = this.pieStack.length;
    const label = depth === 1 ? 'Other' : `Other (${depth})`;
    const totalSize = otherEntries.reduce((s, e) => s + e.size, 0);
    this.pieStack = [...this.pieStack, { label, entries: otherEntries, totalSize }];
    this.cdr.markForCheck();
  }

  /** Jump to any frame in the pie breadcrumb trail. */
  pieGoTo(index: number): void {
    if (index >= this.pieStack.length - 1) return;
    this.pieStack = this.pieStack.slice(0, index + 1);
    this.cdr.markForCheck();
  }

  trackEntry(_: number, e: DiskUsageEntry): string { return e.path; }

  // ── View helpers ──────────────────────────────────────────────────────────

  get currentLabel(): string {
    return this.labelFromPath(this.currentPath) || this.currentPath || 'Disk Usage';
  }

  get hasVisibleContent(): boolean {
    return !!this.result || this.entries.length > 0;
  }

  get totalSizeLabel(): string {
    return this.formatBytes(this.result?.totalSize ?? this.visibleEntriesSize);
  }

  get totalSizeMetricLabel(): string {
    return this.result ? 'Total size' : 'Visible size so far';
  }

  get totalItems(): number {
    return this.result ? this.result.totalFiles + this.result.totalDirs : this.entries.length;
  }

  get itemMixLabel(): string {
    if (this.result) return `${this.result.totalFiles} files · ${this.result.totalDirs} dirs`;
    const files = this.entries.filter(e => !e.isDir).length;
    const dirs = this.entries.filter(e => e.isDir).length;
    return `${files} files · ${dirs} dirs loaded`;
  }

  get headerProgressLabel(): string {
    if (!this.entries.length) return 'Traversing...';
    return `Traversing... ${this.entries.length} children loaded`;
  }

  get averageEntrySizeLabel(): string {
    if (!this.entries.length) return '0 B';
    return this.formatBytes((this.result?.totalSize ?? this.visibleEntriesSize) / this.entries.length);
  }

  get elapsedLabel(): string {
    if (this.isTraversing) return 'Scanning';
    return this.result ? `${this.result.elapsedMs} ms` : 'Idle';
  }

  get elapsedMetricLabel(): string {
    if (this.isTraversing) {
      return this.entries.length ? `${this.entries.length} children loaded` : 'Starting traversal';
    }
    return this.result ? 'Scan completed' : 'Scan time';
  }

  barPercent(e: DiskUsageEntry): number {
    const totalSize = this.result?.totalSize ?? this.visibleEntriesSize;
    if (!totalSize) return 0;
    return Math.max(0.3, (e.size / totalSize) * 100);
  }

  pct(e: DiskUsageEntry): string {
    const totalSize = this.result?.totalSize ?? this.visibleEntriesSize;
    if (!totalSize) return '—';
    const p = (e.size / totalSize) * 100;
    return p < 0.1 ? '<0.1%' : p.toFixed(1) + '%';
  }

  formatBytes(bytes: number): string {
    if (!bytes) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const k = 1024, i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), units.length - 1);
    const val = bytes / Math.pow(k, i);
    return (i === 0 ? val : val.toFixed(1)) + ' ' + units[i];
  }

  entryComposition(entry: DiskUsageEntry): string {
    return entry.isDir ? `${entry.files} files · ${entry.dirs} dirs` : 'File';
  }

  private get visibleEntriesSize(): number {
    return this.entries.reduce((sum, entry) => sum + entry.size, 0);
  }

  private isDiskUsageResult(item: DiskUsageEntry | DiskUsageResult): item is DiskUsageResult {
    return Array.isArray((item as DiskUsageResult).entries);
  }

  private syncRootPieFrame(totalSize: number): void {
    const rootFrame: PieFrame = {
      label: 'Distribution',
      entries: this.entries,
      totalSize,
    };
    if (this.pieStack.length <= 1) {
      this.pieStack = [rootFrame];
    }
  }

  // ── Core analysis ─────────────────────────────────────────────────────────

  async analyse(path: string): Promise<void> {
    this.ctx?.sizeVm?.onAnalyse(path);
    this.currentPath = path;
    this.parentPath  = this.getParent(path);
    this.loading     = true;
    this.isTraversing = true;
    this.error       = null;
    this.result      = null;
    this.entries     = [];
    this.selectedEntry = null;
    this.pieStack    = [];
    this.cdr.markForCheck();

    let firstEntryArrived = false;
    try {
      for await (const item of this.api.streamDiskUsage(path)) {
        if (this.destroyed || this.currentPath !== path) break;
        if (this.isDiskUsageResult(item)) {
          this.result = item;
          this.entries = item.entries?.length ? item.entries : this.entries;
          this.syncRootPieFrame(item.totalSize);
          if (!firstEntryArrived) {
            firstEntryArrived = true;
            this.firstEntryArrived.emit();
          }
          this.cdr.markForCheck();
        } else {
          this.entries = [...this.entries, item];
          this.syncRootPieFrame(this.visibleEntriesSize);
          if (!firstEntryArrived) {
            firstEntryArrived = true;
            this.firstEntryArrived.emit();
          }
          this.cdr.markForCheck();
        }
      }
    } catch (err: any) {
      if (this.destroyed || this.currentPath !== path) return;
      this.error   = err?.message || String(err);
      this.result  = null;
      this.entries = [];
      this.pieStack = [];
    } finally {
      this.loading = false;
      this.isTraversing = false;
      this.cdr.markForCheck();
    }
  }

  private getParent(path: string): string | null {
    const n = path.replace(/\\/g, '/').replace(/\/+$/, '');
    if (/^[a-zA-Z]:?\/?$/.test(n) || !n || n === '/') return null;
    const i = n.lastIndexOf('/');
    if (i <= 0) return n.charAt(0) === '/' ? '/' : null;
    const p = n.substring(0, i) || '/';
    return /^[a-zA-Z]:$/.test(p) ? p + '/' : p;
  }

  private labelFromPath(path: string): string {
    const n = path.replace(/\\/g, '/').replace(/\/+$/, '');
    if (!n) return '';
    if (/^[a-zA-Z]:$/.test(n)) return n + '/';
    const segs = n.split('/');
    return segs[segs.length - 1] || n;
  }
}