import { ChangeDetectionStrategy, Component, DoCheck, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import type { PaneComparisonItem, PaneComparisonResult, PaneComparisonStatus } from '../services/pane-comparison.service';
import type { SyncHistoryEntry } from '../services/operation-history.service';
import {
  PaneSyncReviewDialogService,
  type PaneSyncDirection,
  type PaneSyncPlanItem,
  type PaneSyncPreset,
  type PaneSyncReviewDialogRequest,
} from '../services/pane-sync-review-dialog.service';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';

type Filter = 'different' | PaneComparisonStatus;

@Component({
  selector: 'app-pane-sync-review',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container *ngIf="activeRequest as request">
    <div class="review-backdrop dialog-group" [class.dialog-group--closing]="isClosing" (click)="dialogs.close()" (animationend)="onAnimationEnd($event)">
      <div class="review dialog-surface" role="dialog" aria-modal="true" aria-label="Synchronization review" (click)="$event.stopPropagation()" (keydown.escape)="dialogs.close(); $event.preventDefault(); $event.stopPropagation()">
      <div class="header">
        <div class="title">
          <strong>Synchronization Review</strong>
          <span class="tabular">
            {{ approvedCount }} approved · {{ skippedCount }} skipped · {{ conflictCount }} conflicts
            @if (transferBytesLabel) {
              · {{ transferBytesLabel }}
            }
          </span>
        </div>
        <div class="header-actions">
          <button type="button" [class.active]="preset === 'safe'" [attr.aria-pressed]="preset === 'safe'" (click)="applyPreset('safe')">Safe</button>
          <button type="button" [class.active]="preset === 'left-to-right'" [attr.aria-pressed]="preset === 'left-to-right'" (click)="applyPreset('left-to-right')">Left → Right</button>
          <button type="button" [class.active]="preset === 'right-to-left'" [attr.aria-pressed]="preset === 'right-to-left'" (click)="applyPreset('right-to-left')">Right → Left</button>
          <button type="button" [class.active]="preset === 'bidirectional'" [attr.aria-pressed]="preset === 'bidirectional'" (click)="applyPreset('bidirectional')">Bidirectional</button>
          <button type="button" (click)="skipConflicts()" [disabled]="conflictCount === 0" aria-label="Set all conflicting items to Skip" title="Set all conflicting items to Skip">Skip Conflicts</button>
          <button type="button" class="primary" (click)="executePlan()" [disabled]="approvedCount === 0">Execute {{ approvedCount }}</button>
          <button type="button" class="dialog-close" aria-label="Close dialog" title="Close" (click)="dialogs.close()"> × </button>
        </div>
      </div>

      <div class="filters">
        <div class="filter-buttons">
          <button type="button" [class.active]="filter === 'different'" [attr.aria-pressed]="filter === 'different'" (click)="filter = 'different'">All differences</button>
          <button type="button" *ngFor="let option of filterOptions" [class.active]="filter === option.status" [attr.aria-pressed]="filter === option.status" (click)="filter = option.status">
            {{ option.label }} <span>{{ option.count }}</span>
          </button>
        </div>
        <input
          type="search"
          class="filter-search"
          [value]="searchQuery"
          (input)="searchQuery = $any($event.target).value"
          placeholder="Filter files…"
          aria-label="Filter sync plan items" />
      </div>

      <div class="columns">
        <span>Name &amp; Sizes</span>
        <span>Status</span>
        <span>Decision</span>
      </div>

      <div class="history" *ngIf="history.length">
        <strong>Recent synchronization</strong>
        <span *ngFor="let run of history.slice(0, 3)">{{ syncHistoryLabel(run) }}</span>
      </div>

      <div class="rows">
        <div class="row" *ngFor="let item of visibleItems; trackBy: trackByKey">
          <div class="name-cell">
            <div class="name" [title]="item.key || item.name">{{ item.key || item.name }}</div>
            <div class="subline tabular">{{ metadataSummary(item.key) }}</div>
          </div>
          <div class="status" [attr.data-status]="item.status">{{ statusLabel(item.status) }}</div>
          <div class="decision">
            <button type="button" [class.active]="item.direction === 'left-to-right'" [attr.aria-pressed]="item.direction === 'left-to-right'" [disabled]="!item.leftPath" (click)="setDirection(item, 'left-to-right')">Left → Right</button>
            <button type="button" [class.active]="item.direction === 'right-to-left'" [attr.aria-pressed]="item.direction === 'right-to-left'" [disabled]="!item.rightPath" (click)="setDirection(item, 'right-to-left')">Right → Left</button>
            <button type="button" [class.active]="item.direction === 'skip'" [attr.aria-pressed]="item.direction === 'skip'" (click)="setDirection(item, 'skip')">Skip</button>
          </div>
        </div>
        <div class="empty" *ngIf="visibleItems.length === 0" role="status">No items match this filter.</div>
      </div>
    </div>
    </div>
    </ng-container>
  `,
  styleUrls: ['./tools-dialog-remote-theme.scss'],
  styles: [`
    :host { position: fixed; inset: 0; z-index: 30012; }
    .review {
      width: min(1100px, calc(100vw - 48px));
      max-height: min(720px, calc(100vh - 48px));
      display: flex;
      flex-direction: column;
      overflow: hidden;
      border: 1px solid var(--vsc-border-bright, var(--vsc-neutral-overlay-medium));
      border-radius: var(--vsc-border-radius-lg, 8px);
      background: var(--vsc-editor-background);
      box-shadow: var(--vsc-widget-shadow), 0 32px 64px var(--vsc-scrim-soft), 0 0 0 1px var(--vsc-neutral-overlay-subtle) inset;
      font-size: 12px;
    }
    .header, .filters, .columns, .row { display: grid; align-items: center; }
    .header { grid-template-columns: minmax(0,1fr) auto; gap: 12px; padding: 12px 16px; border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-neutral-overlay-soft)); background: var(--vsc-titlebar-background); }
    .title { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; min-width: 0; }
    .title strong { color: var(--vsc-foreground-bright); font-size: 14px; font-weight: 600; }
    .title span { color: var(--vsc-foreground-dim); font-size: 11px; }
    .tabular { font-variant-numeric: tabular-nums; }
    .header-actions, .decision { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
    button:not(.dialog-close) {
      border: 1px solid transparent;
      background: var(--vsc-button-secondary, transparent);
      color: inherit;
      border-radius: var(--vsc-border-radius-sm, 4px);
      min-height: 28px;
      padding: 0 10px;
      font: inherit;
      white-space: nowrap;
      cursor: pointer;
      transition: background-color var(--vsc-transition-fast), border-color var(--vsc-transition-fast), color var(--vsc-transition-fast);
    }
    button:not(.dialog-close):hover:not(:disabled), button:not(.dialog-close).active { background: var(--vsc-button-secondary-hover, color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent)); }
    button:not(.dialog-close).primary { background: var(--vsc-button-background, var(--vsc-accent-blue)); color: var(--vsc-button-foreground, var(--vsc-on-accent)); border-color: transparent; }
    button:not(.dialog-close):disabled { opacity: .38; cursor: default; }
    button:not(.dialog-close):focus-visible, .filter-search:focus-visible { outline: 1px solid var(--vsc-input-focus-border); outline-offset: 1px; }
    .filters { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 8px 16px; border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-neutral-overlay-soft)); background: color-mix(in srgb, var(--vsc-foreground) 2%, var(--vsc-editor-background)); flex-wrap: wrap; }
    .filter-buttons { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
    .filters button:not(.dialog-close) { white-space: nowrap; }
    .filters span { opacity: .65; margin-left: 3px; }
    .filter-search { flex: 0 1 220px; min-width: 160px; height: 28px; padding: 0 8px; border: 1px solid var(--vsc-input-border, var(--vsc-border)); border-radius: 4px; background: var(--vsc-input-background); color: var(--vsc-input-foreground, var(--vsc-foreground)); font: inherit; }
    .columns, .row { grid-template-columns: minmax(180px, 1fr) 140px minmax(270px, auto); gap: 10px; padding: 7px 16px; }
    .columns { color: var(--vsc-foreground-dim); text-transform: uppercase; font-size: 10px; letter-spacing: .08em; border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-neutral-overlay-soft)); background: color-mix(in srgb, var(--vsc-sidebar-background) 30%, var(--vsc-editor-background)); }
    .history { display:flex; gap:8px; align-items:center; padding:8px 16px; overflow-x:auto; border-bottom:1px solid var(--vsc-border-subtle, var(--vsc-neutral-overlay-soft)); font-size:10px; background: color-mix(in srgb, var(--vsc-foreground) 2%, var(--vsc-editor-background)); }
    .history strong { white-space:nowrap; }
    .history span { color: var(--vsc-foreground-dim); white-space:nowrap; }
    .rows { overflow: auto; min-height: 0; }
    .row { min-height: 44px; border-bottom: 1px solid color-mix(in srgb, var(--vsc-border-subtle, var(--vsc-neutral-overlay-medium)) 60%, transparent); }
    .name-cell { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
    .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 500; color: var(--vsc-foreground-bright); }
    .subline { font-size: 10.5px; color: var(--vsc-foreground-dim); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .status { color: var(--vsc-foreground-dim); }
    .status[data-status="only-left"], .status[data-status="only-right"] { font-weight: 600; color: var(--vsc-accent-blue); }
    .status[data-status="size-different"], .status[data-status="hash-different"], .status[data-status="content-different"], .status[data-status="type-different"] { color: var(--vsc-warning); }
    .decision { justify-content: flex-end; }
    .empty { padding: 28px 10px; text-align: center; opacity: .65; }
  `],
})
export class PaneSyncReviewComponent implements OnInit, OnDestroy, DoCheck {
  filter: Filter = 'different';
  searchQuery = '';
  items: PaneSyncPlanItem[] = [];
  preset: PaneSyncPreset | 'custom' = 'safe';
  private readonly destroy$ = createSubject<void>();
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;

  readonly statuses: PaneComparisonStatus[] = [
    'only-left', 'only-right', 'left-newer', 'right-newer', 'size-different', 'hash-different', 'content-different', 'type-different',
  ];

  constructor(readonly dialogs: PaneSyncReviewDialogService) {}

  ngOnInit(): void {
    this.dialogs.activeRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe((request) => {
        if (request) {
          if (this.modalStack && this.hostElement && !this.unregisterModal) {
            this.unregisterModal = this.modalStack.register({
              id: 'pane-sync-review',
              element: this.hostElement.nativeElement,
              dismiss: () => this.dialogs.close(),
              dismissible: () => !this.isClosing,
            });
          }
        } else {
          this.unregisterModal?.();
          this.unregisterModal = null;
        }
      });
  }

  ngOnDestroy(): void {
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.destroy$.next();
    this.destroy$.complete();
  }

  get activeRequest(): PaneSyncReviewDialogRequest | null {
    return this.dialogs.mountedRequest$.value;
  }

  get isClosing(): boolean {
    return !!this.dialogs.closingRequest$.value;
  }

  private get result(): PaneComparisonResult {
    return this.dialogs.mountedRequest$.value?.viewModel.getResult() ?? {
      items: [],
      summary: {
        total: 0, same: 0, onlyLeft: 0, onlyRight: 0,
        leftNewer: 0, rightNewer: 0, sizeDifferent: 0,
        hashDifferent: 0, contentDifferent: 0, typeDifferent: 0,
        different: 0,
      },
      policy: 'size-date',
    };
  }

  get history(): readonly SyncHistoryEntry[] {
    return this.dialogs.mountedRequest$.value?.viewModel.getHistory() ?? [];
  }

  ngDoCheck(): void {
    this.rebuildPlan();
  }

  get visibleItems(): PaneSyncPlanItem[] {
    const query = this.searchQuery.trim().toLocaleLowerCase();
    return this.items.filter((item) => {
      if (this.filter !== 'different' && item.status !== this.filter) return false;
      if (query) {
        const haystack = `${item.key} ${item.name}`.toLocaleLowerCase();
        if (!haystack.includes(query)) return false;
      }
      return true;
    });
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

  get transferBytesLabel(): string {
    const byKey = new Map(this.result.items.map((item) => [item.key, item] as const));
    let ltr = 0;
    let rtl = 0;
    for (const plan of this.items) {
      const comp = byKey.get(plan.key);
      if (plan.direction === 'left-to-right' && comp?.left) {
        ltr += comp.left.size || 0;
      } else if (plan.direction === 'right-to-left' && comp?.right) {
        rtl += comp.right.size || 0;
      }
    }
    const parts: string[] = [];
    if (ltr > 0) parts.push(`L→R ${this.formatBytes(ltr)}`);
    if (rtl > 0) parts.push(`R→L ${this.formatBytes(rtl)}`);
    return parts.join(' · ');
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

  skipConflicts(): void {
    let changed = false;
    for (const item of this.items) {
      if (this.isConflict(item.status) && item.direction !== 'skip') {
        item.direction = 'skip';
        changed = true;
      }
    }
    if (changed) {
      this.preset = 'custom';
    }
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
    this.dialogs.mountedRequest$.value?.viewModel.onExecute({
      items: approved.map(item => ({ ...item })),
      leftToRight: approved.filter(item => item.direction === 'left-to-right' && item.leftPath).map(item => item.leftPath!),
      rightToLeft: approved.filter(item => item.direction === 'right-to-left' && item.rightPath).map(item => item.rightPath!),
      preset: this.preset,
      conflicts: this.conflictCount,
    });
  }

  metadataSummary(key: string): string {
    const comp = this.result.items.find((entry) => entry.key === key);
    if (!comp) return '';
    const leftText = comp.left ? `L: ${this.formatBytes(comp.left.size)}` : 'L: missing';
    const rightText = comp.right ? `R: ${this.formatBytes(comp.right.size)}` : 'R: missing';
    return `${leftText}  ·  ${rightText}`;
  }

  trackByKey(_index: number, item: PaneSyncPlanItem): string { return item.key; }

  syncHistoryLabel(run: SyncHistoryEntry): string {
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

  private formatBytes(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
    const val = bytes / Math.pow(1024, i);
    return `${val >= 10 || i === 0 ? val.toFixed(0) : val.toFixed(1)} ${units[i]}`;
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

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) {
      return;
    }
    this.dialogs.finishClose();
  }
}