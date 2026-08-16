import { CommonModule } from '@angular/common';
import { Component, OnDestroy, OnInit } from '@angular/core';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';
import { OperationHistoryEntry, OperationService, OperationState } from '../services/operation.service';

type OperationCenterFilter = 'all' | 'active' | 'history' | 'failed';

@Component({
  selector: 'app-operation-center',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div *ngIf="visible" class="operation-center-backdrop" (mousedown)="hide()">
      <aside class="operation-center" role="dialog" aria-modal="true" aria-label="Operations" (mousedown)="$event.stopPropagation()">
        <header class="center-header">
          <div>
            <div class="eyebrow">Activity</div>
            <h2>Operations</h2>
          </div>
          <button class="icon-button" type="button" title="Close" (click)="hide()">×</button>
        </header>

        <div class="summary">
          <div><strong>{{ active.length }}</strong><span>Active</span></div>
          <div><strong>{{ failedCount }}</strong><span>Failed</span></div>
          <div><strong>{{ history.length }}</strong><span>History</span></div>
        </div>

        <nav class="filters" aria-label="Operation filters">
          <button *ngFor="let item of filterItems"
                  type="button"
                  [class.active]="filter === item.id"
                  (click)="filter = item.id">
            {{ item.label }}
          </button>
        </nav>

        <div class="operation-list">
          <div *ngIf="entries.length === 0" class="empty-state">
            <strong>No operations here</strong>
            <span>File operations and long-running tasks will appear here.</span>
          </div>

          <article *ngFor="let op of entries; trackBy: trackOperation" class="operation-row" [class.failed]="op.status === 'error'">
            <div class="operation-icon" aria-hidden="true">{{ iconFor(op) }}</div>
            <div class="operation-main">
              <div class="operation-title-line">
                <strong>{{ op.config.title }}</strong>
                <span class="status" [attr.data-status]="op.status">{{ statusLabel(op.status) }}</span>
              </div>
              <div class="operation-path" [title]="operationPath(op)">{{ operationPath(op) }}</div>

              <div *ngIf="isActive(op)" class="progress-track" aria-hidden="true">
                <span [style.width.%]="progressFor(op)"></span>
              </div>

              <div class="operation-meta">
                <span *ngIf="op.metrics.totalItems > 0">{{ op.metrics.processedItems }}/{{ op.metrics.totalItems }} items</span>
                <span *ngIf="op.metrics.processedBytes > 0 || op.metrics.totalBytes > 0">
                  {{ operationService.formatBytes(op.metrics.processedBytes) }}<ng-container *ngIf="op.metrics.totalBytes > 0"> / {{ operationService.formatBytes(op.metrics.totalBytes) }}</ng-container>
                </span>
                <span *ngIf="op.timing.speedBytesPerSecond > 0">{{ operationService.formatSpeed(op.timing.speedBytesPerSecond) }}</span>
                <span>{{ timeLabel(op) }}</span>
              </div>

              <div *ngIf="op.error" class="operation-error">{{ op.error }}</div>
            </div>

            <div class="operation-actions">
              <button *ngIf="isActive(op)" type="button" (click)="cancel(op.id)">Cancel</button>
            </div>
          </article>
        </div>

        <footer class="center-footer">
          <span>Recent operations are kept for this app session.</span>
          <button type="button" [disabled]="history.length === 0" (click)="clearHistory()">Clear History</button>
        </footer>
      </aside>
    </div>
  `,
  styles: [`
    :host { position: relative; z-index: calc(var(--vsc-z-modal) + 2); }
    .operation-center-backdrop {
      position: fixed; inset: 0; z-index: calc(var(--vsc-z-modal) + 2);
      background: var(--vsc-scrim-soft); display: flex; justify-content: flex-end;
    }
    .operation-center {
      width: min(560px, 92vw); height: 100%; display: flex; flex-direction: column;
      background: var(--vsc-panel-background); color: var(--vsc-foreground);
      border-left: 1px solid var(--vsc-border-bright); box-shadow: var(--vsc-widget-shadow);
    }
    .center-header { display:flex; align-items:center; justify-content:space-between; padding:18px 20px 14px; border-bottom:1px solid var(--vsc-border); }
    .center-header h2 { margin:2px 0 0; font-size:20px; font-weight:600; }
    .eyebrow { color:var(--vsc-foreground-dim); font-size:11px; text-transform:uppercase; letter-spacing:.08em; }
    .icon-button { width:30px; height:30px; border:0; background:transparent; color:inherit; font-size:24px; cursor:pointer; border-radius:4px; }
    .icon-button:hover { background:var(--vsc-list-hover-background); }
    .summary { display:grid; grid-template-columns:repeat(3,1fr); gap:1px; background:var(--vsc-border); border-bottom:1px solid var(--vsc-border); }
    .summary div { display:flex; flex-direction:column; padding:12px 16px; background:var(--vsc-editor-background); }
    .summary strong { font-size:18px; } .summary span { color:var(--vsc-foreground-dim); font-size:11px; }
    .filters { display:flex; gap:4px; padding:10px 14px; border-bottom:1px solid var(--vsc-border); }
    .filters button, .operation-actions button, .center-footer button {
      border:1px solid transparent; border-radius:3px; background:var(--vsc-button-secondary); color:var(--vsc-button-secondary-foreground);
      padding:4px 9px; font:inherit; font-size:12px; cursor:pointer;
    }
    .filters button:hover, .operation-actions button:hover, .center-footer button:hover:not(:disabled) { background:var(--vsc-button-secondary-hover); }
    .filters button.active { border-color:var(--vsc-accent-blue); background:var(--vsc-list-active-background); }
    .operation-list { flex:1; min-height:0; overflow:auto; }
    .operation-row { display:grid; grid-template-columns:28px minmax(0,1fr) auto; gap:10px; padding:13px 16px; border-bottom:1px solid var(--vsc-border); }
    .operation-row:hover { background:var(--vsc-list-hover-background); }
    .operation-row.failed { border-left:2px solid var(--vsc-error); padding-left:14px; }
    .operation-icon { font-size:16px; padding-top:1px; opacity:.9; }
    .operation-main { min-width:0; }
    .operation-title-line { display:flex; gap:8px; align-items:center; min-width:0; }
    .operation-title-line strong { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .status { margin-left:auto; font-size:10px; text-transform:uppercase; letter-spacing:.04em; color:var(--vsc-foreground-dim); }
    .status[data-status='error'] { color:var(--vsc-error); }
    .status[data-status='running'], .status[data-status='paused'] { color:var(--vsc-accent-blue); }
    .operation-path { margin-top:3px; color:var(--vsc-foreground-dim); font-size:11px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
    .progress-track { height:3px; margin-top:8px; overflow:hidden; background:var(--vsc-border); border-radius:2px; }
    .progress-track span { display:block; height:100%; min-width:2px; background:var(--vsc-accent-blue); }
    .operation-meta { display:flex; flex-wrap:wrap; gap:5px 12px; margin-top:6px; color:var(--vsc-foreground-dim); font-size:10px; }
    .operation-error { margin-top:6px; color:var(--vsc-error); font-size:11px; white-space:pre-wrap; }
    .operation-actions { display:flex; align-items:flex-start; }
    .empty-state { height:100%; min-height:220px; display:flex; flex-direction:column; align-items:center; justify-content:center; gap:6px; color:var(--vsc-foreground-dim); text-align:center; padding:24px; }
    .empty-state strong { color:var(--vsc-foreground); }
    .center-footer { display:flex; justify-content:space-between; align-items:center; gap:12px; padding:10px 14px; border-top:1px solid var(--vsc-border); color:var(--vsc-foreground-dim); font-size:10px; }
    .center-footer button:disabled { opacity:.45; cursor:default; }
  `],
})
export class OperationCenterComponent implements OnInit, OnDestroy {
  visible = false;
  filter: OperationCenterFilter = 'all';
  active: OperationState[] = [];
  history: OperationHistoryEntry[] = [];
  private readonly destroy$ = createSubject<void>();
  private readonly freezeKey = 'operation-center';

  readonly filterItems: Array<{ id: OperationCenterFilter; label: string }> = [
    { id: 'all', label: 'All' },
    { id: 'active', label: 'Active' },
    { id: 'history', label: 'History' },
    { id: 'failed', label: 'Failed' },
  ];

  constructor(
    public readonly operationService: OperationService,
    private readonly overlayFreeze: NativeOverlayFreezeService,
  ) {}

  ngOnInit(): void {
    this.operationService.operations$.pipe(takeUntil(this.destroy$)).subscribe((map: Map<string, OperationState>) => {
      this.active = Array.from(map.values()).filter(op => this.isActive(op));
    });
    this.operationService.history$.pipe(takeUntil(this.destroy$)).subscribe((history: OperationHistoryEntry[]) => {
      this.history = history;
    });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
    this.overlayFreeze.release(this.freezeKey);
  }

  show(): void { this.visible = true; this.overlayFreeze.acquire(this.freezeKey); }
  hide(): void { this.visible = false; this.overlayFreeze.release(this.freezeKey); }
  toggle(): void { this.visible ? this.hide() : this.show(); }

  get entries(): Array<OperationState | OperationHistoryEntry> {
    const activeIds = new Set(this.active.map(op => op.id));
    const historical = this.history.filter(op => !activeIds.has(op.id));
    const all = [...this.active, ...historical];
    if (this.filter === 'active') return this.active;
    if (this.filter === 'history') return historical;
    if (this.filter === 'failed') return all.filter(op => op.status === 'error');
    return all;
  }

  get failedCount(): number { return this.history.filter(op => op.status === 'error').length; }

  trackOperation(_index: number, op: OperationState): string { return op.id; }
  isActive(op: OperationState): boolean { return op.status === 'running' || op.status === 'pending' || op.status === 'paused'; }
  progressFor(op: OperationState): number { return Math.max(0, Math.min(100, op.percentComplete || 0)); }
  iconFor(op: OperationState): string {
    const icons: Record<string, string> = { copy:'⧉', move:'↪', delete:'⌫', archive:'▣', extract:'⇲', search:'⌕', calculate:'∑' };
    return icons[op.config.kind] ?? '⚙';
  }
  statusLabel(status: string): string { return status === 'error' ? 'Failed' : status.charAt(0).toUpperCase() + status.slice(1); }
  operationPath(op: OperationState): string {
    const source = op.sources?.filter(Boolean).join(', ') || op.metrics.currentFile || '';
    return op.destination ? `${source} → ${op.destination}` : source;
  }
  timeLabel(op: OperationState | OperationHistoryEntry): string {
    const when = 'completedAt' in op ? op.completedAt : op.timing.startedAt;
    if (!when) return '';
    const delta = Math.max(0, Date.now() - when);
    if (delta < 60_000) return 'just now';
    if (delta < 3_600_000) return `${Math.floor(delta / 60_000)}m ago`;
    return new Date(when).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }
  async cancel(id: string): Promise<void> { await this.operationService.cancel(id); }
  clearHistory(): void { this.operationService.clearHistory(); }
}