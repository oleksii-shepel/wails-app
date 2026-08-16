import { ChangeDetectionStrategy, Component, EventEmitter, Input, OnChanges, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import type { PaneComparisonItem, PaneComparisonResult, PaneComparisonStatus } from '../services/pane-comparison.service';
import type { ReassemblySyncHistoryEntry } from '../services/reassembly-operation-history.service';

export type PaneSyncDirection = 'skip' | 'left-to-right' | 'right-to-left';
export type PaneSyncPreset = 'safe' | 'left-to-right' | 'right-to-left' | 'bidirectional';

export interface PaneSyncPlanItem {
  key: string;
  name: string;
  status: PaneComparisonStatus;
  direction: PaneSyncDirection;
  leftPath: string | null;
  rightPath: string | null;
}

export interface PaneSyncPlan {
  items: PaneSyncPlanItem[];
  leftToRight: string[];
  rightToLeft: string[];
  preset: PaneSyncPreset | 'custom';
  conflicts: number;
}

type Filter = 'different' | PaneComparisonStatus;

@Component({
  selector: 'app-pane-sync-review',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="review" *ngIf="visible">
      <div class="header">
        <div class="title">
          <strong>Synchronization Review</strong>
          <span>{{ approvedCount }} approved · {{ skippedCount }} skipped · {{ conflictCount }} conflicts</span>
        </div>
        <div class="header-actions">
          <button type="button" (click)="applyPreset('safe')">Safe</button>
          <button type="button" (click)="applyPreset('left-to-right')">Left → Right</button>
          <button type="button" (click)="applyPreset('right-to-left')">Right → Left</button>
          <button type="button" (click)="applyPreset('bidirectional')">Bidirectional</button>
          <button type="button" class="primary" (click)="executePlan()" [disabled]="approvedCount === 0">Execute {{ approvedCount }}</button>
          <button type="button" class="close" (click)="close.emit()" title="Close review">×</button>
        </div>
      </div>

      <div class="filters">
        <button type="button" [class.active]="filter === 'different'" (click)="filter = 'different'">All differences</button>
        <button type="button" *ngFor="let option of filterOptions" [class.active]="filter === option.status" (click)="filter = option.status">
          {{ option.label }} <span>{{ option.count }}</span>
        </button>
      </div>

      <div class="columns">
        <span>Name</span>
        <span>Status</span>
        <span>Decision</span>
      </div>

      <div class="history" *ngIf="history.length">
        <strong>Recent synchronization</strong>
        <span *ngFor="let run of history.slice(0, 3)">{{ syncHistoryLabel(run) }}</span>
      </div>

      <div class="rows">
        <div class="row" *ngFor="let item of visibleItems; trackBy: trackByKey">
          <div class="name" [title]="item.name">{{ item.name }}</div>
          <div class="status" [attr.data-status]="item.status">{{ statusLabel(item.status) }}</div>
          <div class="decision">
            <button type="button" [class.active]="item.direction === 'left-to-right'" [disabled]="!item.leftPath" (click)="setDirection(item, 'left-to-right')">Left → Right</button>
            <button type="button" [class.active]="item.direction === 'right-to-left'" [disabled]="!item.rightPath" (click)="setDirection(item, 'right-to-left')">Right → Left</button>
            <button type="button" [class.active]="item.direction === 'skip'" (click)="setDirection(item, 'skip')">Skip</button>
          </div>
        </div>
        <div class="empty" *ngIf="visibleItems.length === 0">No items in this filter.</div>
      </div>
    </div>
  `,
  styles: [`
    :host { display: block; flex: 0 0 auto; }
    .review {
      max-height: 320px;
      display: flex;
      flex-direction: column;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-neutral-overlay-medium));
      background: var(--vsc-background, var(--vsc-editor-background));
      font-size: 12px;
    }
    .header, .filters, .columns, .row { display: grid; align-items: center; }
    .header { grid-template-columns: minmax(0,1fr) auto; gap: 12px; padding: 8px 10px; border-bottom: 1px solid var(--vsc-border, var(--vsc-neutral-overlay-soft)); }
    .title { display: flex; align-items: baseline; gap: 8px; min-width: 0; }
    .title span { opacity: .65; }
    .header-actions, .filters, .decision { display: flex; gap: 6px; align-items: center; }
    button {
      border: 1px solid var(--vsc-border, var(--vsc-neutral-overlay-strong));
      background: var(--vsc-button-background, transparent);
      color: inherit;
      border-radius: 3px;
      padding: 3px 8px;
      font: inherit;
      cursor: pointer;
    }
    button:hover:not(:disabled), button.active { background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent); }
    button.primary { background: var(--vsc-accent-blue, var(--vsc-accent-blue)); color: var(--vsc-on-accent); border-color: transparent; }
    button:disabled { opacity: .38; cursor: default; }
    .close { width: 25px; padding-inline: 0; font-size: 16px; }
    .filters { padding: 5px 10px; overflow-x: auto; border-bottom: 1px solid var(--vsc-border, var(--vsc-neutral-overlay-soft)); }
    .filters button { white-space: nowrap; }
    .filters span { opacity: .65; margin-left: 3px; }
    .columns, .row { grid-template-columns: minmax(180px, 1fr) 150px minmax(270px, auto); gap: 10px; padding: 5px 10px; }
    .columns { opacity: .55; text-transform: uppercase; font-size: 10px; letter-spacing: .04em; border-bottom: 1px solid var(--vsc-border, var(--vsc-neutral-overlay-soft)); }
    .history { display:flex; gap:8px; align-items:center; padding:5px 10px; overflow-x:auto; border-bottom:1px solid var(--vsc-border, var(--vsc-neutral-overlay-soft)); font-size:10px; }
    .history strong { white-space:nowrap; }
    .history span { opacity:.7; white-space:nowrap; }
    .rows { overflow: auto; min-height: 0; }
    .row { min-height: 34px; border-bottom: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-neutral-overlay-medium)) 55%, transparent); }
    .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .status { opacity: .85; }
    .status[data-status="only-left"], .status[data-status="only-right"] { font-weight: 600; }
    .decision { justify-content: flex-end; }
    .empty { padding: 18px 10px; text-align: center; opacity: .6; }
  `],
})
export class PaneSyncReviewComponent implements OnChanges {
  @Input() visible = false;
  @Input({ required: true }) result!: PaneComparisonResult;
  @Input() history: readonly ReassemblySyncHistoryEntry[] = [];
  @Output() execute = new EventEmitter<PaneSyncPlan>();
  @Output() close = new EventEmitter<void>();

  filter: Filter = 'different';
  items: PaneSyncPlanItem[] = [];
  preset: PaneSyncPreset | 'custom' = 'safe';

  readonly statuses: PaneComparisonStatus[] = [
    'only-left', 'only-right', 'left-newer', 'right-newer', 'size-different', 'hash-different', 'content-different', 'type-different',
  ];

  ngOnChanges(): void {
    this.rebuildPlan();
  }

  get visibleItems(): PaneSyncPlanItem[] {
    return this.filter === 'different' ? this.items : this.items.filter(item => item.status === this.filter);
  }

  get approvedCount(): number {
    return this.items.filter(item => item.direction !== 'skip').length;
  }

  get skippedCount(): number {
    return this.items.length - this.approvedCount;
  }

  get conflictCount(): number {
    return this.items.filter(item => this.isConflict(item.status)).length;
  }

  get filterOptions(): Array<{ status: PaneComparisonStatus; label: string; count: number }> {
    return this.statuses
      .map(status => ({ status, label: this.statusLabel(status), count: this.items.filter(item => item.status === status).length }))
      .filter(item => item.count > 0);
  }

  setDirection(item: PaneSyncPlanItem, direction: PaneSyncDirection): void {
    if (direction === 'left-to-right' && !item.leftPath) return;
    if (direction === 'right-to-left' && !item.rightPath) return;
    item.direction = direction;
    this.preset = 'custom';
  }

  applySafeSuggestions(): void { this.applyPreset('safe'); }

  applyPreset(preset: PaneSyncPreset): void {
    this.preset = preset;
    for (const item of this.items) {
      item.direction = this.directionForPreset(item, preset);
    }
  }

  executePlan(): void {
    const approved = this.items.filter(item => item.direction !== 'skip');
    this.execute.emit({
      items: approved.map(item => ({ ...item })),
      leftToRight: approved.filter(item => item.direction === 'left-to-right' && item.leftPath).map(item => item.leftPath!),
      rightToLeft: approved.filter(item => item.direction === 'right-to-left' && item.rightPath).map(item => item.rightPath!),
      preset: this.preset,
      conflicts: this.conflictCount,
    });
  }

  trackByKey(_index: number, item: PaneSyncPlanItem): string { return item.key; }

  syncHistoryLabel(run: ReassemblySyncHistoryEntry): string {
    const mode = run.mode === 'bidirectional' ? '↔' : run.mode === 'left-to-right' ? '→' : run.mode === 'right-to-left' ? '←' : 'custom';
    return `${mode} ${run.succeeded}/${run.planned}${run.failed ? ` · ${run.failed} failed` : ''}`;
  }

  statusLabel(status: PaneComparisonStatus): string {
    return ({
      'only-left': 'Left only',
      'only-right': 'Right only',
      'left-newer': 'Left newer',
      'right-newer': 'Right newer',
      'size-different': 'Different size',
      'hash-different': 'Different hash',
      'content-different': 'Different content',
      'type-different': 'Different type',
      'same': 'Same',
    } as Record<PaneComparisonStatus, string>)[status];
  }

  private rebuildPlan(): void {
    const previous = new Map(this.items.map(item => [item.key, item.direction] as const));
    this.items = (this.result?.items ?? [])
      .filter(item => item.status !== 'same')
      .map(item => this.toPlanItem(item, previous.get(item.key)));
  }

  private toPlanItem(item: PaneComparisonItem, previous?: PaneSyncDirection): PaneSyncPlanItem {
    return {
      key: item.key,
      name: item.name,
      status: item.status,
      direction: previous ?? this.defaultDirection(item.status),
      leftPath: item.left?.path ?? null,
      rightPath: item.right?.path ?? null,
    };
  }

  private defaultDirection(status: PaneComparisonStatus): PaneSyncDirection {
    if (status === 'only-left') return 'left-to-right';
    if (status === 'only-right') return 'right-to-left';
    return 'skip';
  }

  private directionForPreset(item: PaneSyncPlanItem, preset: PaneSyncPreset): PaneSyncDirection {
    if (preset === 'safe') return this.defaultDirection(item.status);
    if (preset === 'left-to-right') return item.leftPath ? 'left-to-right' : 'skip';
    if (preset === 'right-to-left') return item.rightPath ? 'right-to-left' : 'skip';

    if (item.status === 'only-left' || item.status === 'left-newer') return item.leftPath ? 'left-to-right' : 'skip';
    if (item.status === 'only-right' || item.status === 'right-newer') return item.rightPath ? 'right-to-left' : 'skip';
    return 'skip';
  }

  private isConflict(status: PaneComparisonStatus): boolean {
    return status === 'size-different' || status === 'hash-different' || status === 'content-different' || status === 'type-different';
  }
}