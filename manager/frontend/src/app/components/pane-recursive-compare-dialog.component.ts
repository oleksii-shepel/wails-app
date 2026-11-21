import { ChangeDetectionStrategy, Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import {
  PaneRecursiveCompareDialogService,
  type PaneRecursiveCompareDialogRequest,
} from '../services/pane-recursive-compare-dialog.service';
import type {
  PaneComparisonItem,
  PaneComparisonPolicy,
  PaneComparisonStatus,
} from '../services/pane-comparison.service';
import type { FileInfo } from '../shared/protocol';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';

type RecursiveCompareFilter = 'different' | 'only-left' | 'only-right' | 'modified' | 'all';

@Component({
  selector: 'app-pane-recursive-compare-dialog',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container *ngIf="activeRequest as request">
      <div class="dialog-backdrop dialog-group" [class.dialog-group--closing]="isClosing" (click)="dialogs.close()" (animationend)="onAnimationEnd($event)">
        <section class="dialog-panel palette-dialog-surface dialog-surface" role="dialog" aria-modal="true" aria-label="Recursive compare" (click)="$event.stopPropagation()" (keydown.escape)="dialogs.close(); $event.preventDefault(); $event.stopPropagation()">
          <header class="dialog-header">
            <div class="header-copy">
              <h2>Recursive Compare</h2>
              <p [title]="request.viewModel.getLeftPath() + ' ↔ ' + request.viewModel.getRightPath()">
                {{ request.viewModel.getLeftPath() }} ↔ {{ request.viewModel.getRightPath() }}
              </p>
            </div>
            <button type="button" class="dialog-close" aria-label="Close dialog" title="Close" (click)="dialogs.close()"> × </button>
          </header>

          <div class="dialog-body">
            @let scanning = request.viewModel.getScanning();
            @let scanError = request.viewModel.getError();
            @let result = request.viewModel.getResult();
            @let summary = result.summary;

            <div class="summary-strip" aria-label="Recursive comparison summary">
              <div class="summary-stats">
                @if (scanning) {
                  <span class="stat-item">Scanning nested directories…</span>
                } @else {
                  <span class="stat-item"><strong>{{ summary.different }}</strong> different</span>
                  <span class="sep" aria-hidden="true">·</span>
                  <span class="stat-item"><strong>{{ summary.onlyLeft }}</strong> left only</span>
                  <span class="sep" aria-hidden="true">·</span>
                  <span class="stat-item"><strong>{{ summary.onlyRight }}</strong> right only</span>
                  <span class="sep" aria-hidden="true">·</span>
                  <span class="stat-item"><strong>{{ summary.leftNewer + summary.rightNewer }}</strong> newer</span>
                  <span class="sep" aria-hidden="true">·</span>
                  <span class="stat-item"><strong>{{ summary.same }}</strong> identical</span>
                }
              </div>

              <label class="policy">
                <span>Policy</span>
                <select [value]="request.viewModel.getPolicy()" (change)="onPolicyChange(request.viewModel, $event)" [disabled]="scanning">
                  <option value="size-date">Size + date</option>
                  <option value="size">Size</option>
                  <option value="date">Date</option>
                  <option value="hash">SHA-256</option>
                  <option value="content">Content</option>
                </select>
              </label>
            </div>

            @if (scanError) {
              <div class="scan-error" role="alert">{{ scanError }}</div>
            }

            <div class="filter-bar">
              <div class="filter-tabs" role="group" aria-label="Filter recursive differences">
                <button type="button" [attr.aria-pressed]="filter === 'different'" [class.active]="filter === 'different'" (click)="filter = 'different'">Differences ({{ summary.different }})</button>
                <button type="button" [attr.aria-pressed]="filter === 'only-left'" [class.active]="filter === 'only-left'" (click)="filter = 'only-left'">Left Only ({{ summary.onlyLeft }})</button>
                <button type="button" [attr.aria-pressed]="filter === 'only-right'" [class.active]="filter === 'only-right'" (click)="filter = 'only-right'">Right Only ({{ summary.onlyRight }})</button>
                <button type="button" [attr.aria-pressed]="filter === 'modified'" [class.active]="filter === 'modified'" (click)="filter = 'modified'">Modified ({{ summary.different - summary.onlyLeft - summary.onlyRight }})</button>
                <button type="button" [attr.aria-pressed]="filter === 'all'" [class.active]="filter === 'all'" (click)="filter = 'all'">All ({{ summary.total }})</button>
              </div>
              <input
                type="search"
                class="filter-input"
                [value]="searchQuery"
                (input)="searchQuery = $any($event.target).value"
                placeholder="Filter relative path…"
                aria-label="Filter recursive comparison paths" />
            </div>

            <div class="diff-table" role="region" aria-label="Recursive compared paths">
              <div class="diff-head">
                <span>Relative Path</span>
                <span>Status</span>
                <span class="num">Left</span>
                <span class="num">Right</span>
              </div>
              <div class="diff-rows">
                @for (item of filteredItems(result.items); track item.key) {
                  <div class="diff-row" [class.diff-row--same]="item.status === 'same'">
                    <span class="entry-name" [title]="item.key || item.name">{{ item.key || item.name }}</span>
                    <span class="entry-status" [attr.data-status]="item.status">{{ statusLabel(item.status) }}</span>
                    <span class="entry-meta num" [title]="sideTooltip(item.left)">{{ sideLabel(item.left) }}</span>
                    <span class="entry-meta num" [title]="sideTooltip(item.right)">{{ sideLabel(item.right) }}</span>
                  </div>
                } @empty {
                  <div class="diff-empty" role="status">
                    {{ scanning ? 'Walking directory trees…' : summary.different === 0 && filter === 'different' ? 'No recursive differences found under this policy.' : 'No entries match the current filter.' }}
                  </div>
                }
              </div>
            </div>

            <div class="actions">
              <div class="action-group">
                <button type="button" (click)="request.viewModel.onRefresh()" [disabled]="scanning">Refresh</button>
                <button type="button" (click)="request.viewModel.onSelectLeft()" [disabled]="summary.different === 0">Select Left</button>
                <button type="button" (click)="request.viewModel.onSelectRight()" [disabled]="summary.different === 0">Select Right</button>
                <button type="button" (click)="request.viewModel.onCopyMissingLeftToRight()" [disabled]="summary.onlyLeft === 0">Copy Missing →</button>
                <button type="button" (click)="request.viewModel.onCopyMissingRightToLeft()" [disabled]="summary.onlyRight === 0">← Copy Missing</button>
              </div>
              <div class="action-group">
                <button type="button" class="primary" (click)="request.viewModel.onOpenSyncReview()" [disabled]="summary.different === 0">Review Sync Plan…</button>
              </div>
            </div>
          </div>
        </section>
      </div>
    </ng-container>
  `,
  styleUrls: ['./tools-dialog-remote-theme.scss'],
  styles: [`
    :host { position: fixed; inset: 0; z-index: 30011; }
    .dialog-panel { width: min(980px, calc(100vw - 48px)); max-height: min(760px, calc(100vh - 48px)); display: flex; flex-direction: column; overflow: hidden; background: var(--vsc-editor-background); color: var(--vsc-foreground); border: 1px solid var(--vsc-border-bright); border-radius: var(--vsc-border-radius-lg, 8px); box-shadow: var(--vsc-widget-shadow), 0 32px 64px var(--vsc-scrim-soft), 0 0 0 1px var(--vsc-neutral-overlay-subtle) inset; }
    .dialog-header { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; padding:12px 16px; border-bottom:1px solid var(--vsc-border-subtle); background: var(--vsc-titlebar-background); flex: 0 0 auto; }
    .header-copy { min-width: 0; }
    .dialog-header h2, .dialog-header p { margin:0; }
    .dialog-header h2 { color: var(--vsc-foreground-bright); font-size: 14px; line-height: 1.15; font-weight: 600; }
    .dialog-header p { margin-top:4px; color: var(--vsc-foreground-dim); font-size:12px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .dialog-body { display:flex; flex-direction:column; gap:12px; padding:14px 16px; background: var(--vsc-editor-background); flex: 1 1 auto; min-height: 0; }
    .summary-strip { display:flex; justify-content:space-between; align-items:center; gap:12px; flex-wrap:wrap; padding: 8px 12px; border: 1px solid var(--vsc-border-subtle); border-radius: 6px; background: color-mix(in srgb, var(--vsc-foreground) 3%, var(--vsc-editor-background)); }
    .summary-stats { display:flex; flex-wrap:wrap; align-items:center; gap:6px; font-size:11px; color: var(--vsc-foreground-dim); font-variant-numeric: tabular-nums; }
    .summary-stats strong { color: var(--vsc-foreground-bright); font-weight: 600; }
    .sep { opacity: 0.45; }
    .scan-error { padding: 8px 10px; border: 1px solid color-mix(in srgb, var(--vsc-error) 35%, transparent); border-radius: 5px; background: color-mix(in srgb, var(--vsc-error) 10%, transparent); color: var(--vsc-error-foreground, var(--vsc-error)); font-size: 11.5px; }
    .policy { display:flex; gap:6px; align-items:center; color: var(--vsc-foreground-dim); font-size: 11px; }
    .filter-bar { display:flex; align-items:center; justify-content:space-between; gap:10px; flex-wrap:wrap; }
    .filter-tabs { display:flex; gap:4px; flex-wrap:wrap; }
    .filter-tabs button.active { background: var(--vsc-list-active-background); border-color: var(--vsc-accent-blue); color: var(--vsc-foreground-bright); }
    .filter-input { flex: 0 1 240px; min-width: 170px; height: 28px; padding: 0 8px; border: 1px solid var(--vsc-input-border, var(--vsc-border)); border-radius: 4px; background: var(--vsc-input-background); color: var(--vsc-input-foreground, var(--vsc-foreground)); font: inherit; }
    .diff-table { flex: 1 1 auto; min-height: 180px; max-height: 360px; display:flex; flex-direction:column; border: 1px solid var(--vsc-border-subtle); border-radius: 6px; overflow:hidden; }
    .diff-head, .diff-row { display:grid; grid-template-columns: minmax(180px, 1.5fr) 125px minmax(110px, .85fr) minmax(110px, .85fr); gap:10px; align-items:center; padding: 6px 10px; }
    .diff-head { font-size:10px; text-transform:uppercase; letter-spacing:.06em; color: var(--vsc-foreground-dim); border-bottom: 1px solid var(--vsc-border-subtle); background: color-mix(in srgb, var(--vsc-sidebar-background) 35%, var(--vsc-editor-background)); }
    .diff-rows { flex: 1 1 auto; overflow:auto; }
    .diff-row { min-height: 34px; font-size: 11.5px; border-bottom: 1px solid color-mix(in srgb, var(--vsc-border-subtle) 55%, transparent); }
    .diff-row:last-child { border-bottom: none; }
    .diff-row:hover { background: var(--vsc-list-hover-background); }
    .diff-row--same { opacity: 0.65; }
    .entry-name, .entry-meta { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .entry-name { color: var(--vsc-foreground-bright); font-family: var(--vsc-font-family-mono, ui-monospace, monospace); font-size: 11px; }
    .entry-status { font-size: 11px; color: var(--vsc-accent-blue); white-space: nowrap; }
    .entry-status[data-status="same"] { color: var(--vsc-foreground-dim); }
    .entry-status[data-status="size-different"], .entry-status[data-status="hash-different"], .entry-status[data-status="content-different"], .entry-status[data-status="type-different"] { color: var(--vsc-warning); }
    .entry-meta { font-size: 11px; color: var(--vsc-foreground-dim); }
    .num { text-align: right; font-variant-numeric: tabular-nums; }
    .diff-empty { padding: 36px 16px; text-align: center; color: var(--vsc-foreground-dim); font-size: 12px; }
    .actions { display:flex; justify-content:space-between; flex-wrap:wrap; gap:8px; align-items:center; }
    .action-group { display:flex; flex-wrap:wrap; gap:6px; align-items:center; }
    button:not(.dialog-close), select { border:1px solid var(--vsc-border-subtle, transparent); background: var(--vsc-button-secondary, var(--vsc-control-background)); color:inherit; border-radius: var(--vsc-border-radius-sm, 4px); min-height:28px; padding:0 10px; font:inherit; white-space:nowrap; cursor:pointer; transition: background-color var(--vsc-transition-fast), border-color var(--vsc-transition-fast), color var(--vsc-transition-fast); }
    button:not(.dialog-close).primary { background: var(--vsc-button-background, var(--vsc-accent-blue)); color: var(--vsc-button-foreground, var(--vsc-on-accent)); border-color: transparent; }
    button:not(.dialog-close):hover:not(:disabled), select:hover:not(:disabled) { background: var(--vsc-button-secondary-hover, var(--vsc-list-hover-background)); }
    button:not(.dialog-close):focus-visible, select:focus-visible, .filter-input:focus-visible { outline: 1px solid var(--vsc-input-focus-border); outline-offset: 1px; }
    @media (max-width: 720px) {
      .diff-head, .diff-row { grid-template-columns: minmax(120px, 1fr) 100px 90px; }
      .diff-head span:last-child, .diff-row span:last-child { display: none; }
    }
  `],
})
export class PaneRecursiveCompareDialogComponent implements OnInit, OnDestroy {
  filter: RecursiveCompareFilter = 'different';
  searchQuery = '';
  private readonly destroy$ = createSubject<void>();
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;

  constructor(readonly dialogs: PaneRecursiveCompareDialogService) {}

  ngOnInit(): void {
    this.dialogs.activeRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe((request) => {
        if (request) {
          if (this.modalStack && this.hostElement && !this.unregisterModal) {
            this.unregisterModal = this.modalStack.register({
              id: 'pane-recursive-compare-dialog',
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

  get activeRequest(): PaneRecursiveCompareDialogRequest | null {
    return this.dialogs.mountedRequest$.value;
  }

  get isClosing(): boolean {
    return !!this.dialogs.closingRequest$.value;
  }

  filteredItems(items: readonly PaneComparisonItem[]): PaneComparisonItem[] {
    const query = this.searchQuery.trim().toLocaleLowerCase();
    return items.filter((item) => {
      if (this.filter === 'different' && item.status === 'same') return false;
      if (this.filter === 'only-left' && item.status !== 'only-left') return false;
      if (this.filter === 'only-right' && item.status !== 'only-right') return false;
      if (this.filter === 'modified' && (item.status === 'same' || item.status === 'only-left' || item.status === 'only-right')) return false;
      if (query) {
        const haystack = `${item.key} ${item.name}`.toLocaleLowerCase();
        if (!haystack.includes(query)) return false;
      }
      return true;
    });
  }

  statusLabel(status: PaneComparisonStatus): string {
    const labels: Record<PaneComparisonStatus, string> = {
      'only-left': 'Left only',
      'only-right': 'Right only',
      'left-newer': 'Left newer',
      'right-newer': 'Right newer',
      'size-different': 'Size mismatch',
      'hash-different': 'Hash mismatch',
      'content-different': 'Content differs',
      'type-different': 'Type differs',
      same: 'Identical',
    };
    return labels[status] ?? status;
  }

  sideLabel(entry: FileInfo | null): string {
    if (!entry) return '—';
    return this.formatBytes(entry.size);
  }

  sideTooltip(entry: FileInfo | null): string {
    if (!entry) return 'Not present on this side';
    const modified = entry.modified ? new Date(entry.modified).toLocaleString() : 'Unknown time';
    return `${this.formatBytes(entry.size)} · Modified ${modified}`;
  }

  formatBytes(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
    const val = bytes / Math.pow(1024, i);
    return `${val >= 10 || i === 0 ? val.toFixed(0) : val.toFixed(1)} ${units[i]}`;
  }

  onPolicyChange(viewModel: { onPolicyChange(policy: PaneComparisonPolicy): void | Promise<void> }, event: Event): void {
    void viewModel.onPolicyChange((event.target as HTMLSelectElement).value as PaneComparisonPolicy);
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) {
      return;
    }
    this.dialogs.finishClose();
  }
}