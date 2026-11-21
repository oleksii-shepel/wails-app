import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  Input,
  OnChanges,
  OnDestroy,
  Output,
  QueryList,
  SimpleChanges,
  ViewChild,
  ViewChildren,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import {
  WorkspaceService,
  type RecentlyClosedTab,
} from '../services/workspace.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';
@Component({
  selector: 'app-closed-tabs',
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop palette-dialog-backdrop dialog-group" *ngIf="visible" (mousedown)="close()">
      <section
        class="panel palette-dialog-surface dialog-surface"
        role="dialog"
        aria-modal="true"
        aria-label="Recently Closed Tabs"
        (mousedown)="$event.stopPropagation()" (keydown.escape)="close(); $event.preventDefault(); $event.stopPropagation()">
        <header class="palette-dialog-header">
          <div class="palette-dialog-title">
            <strong>Recently Closed Tabs</strong>
            <span class="palette-dialog-subtitle">{{ entries.length }} recoverable tabs</span>
          </div>
          <button type="button" class="dialog-close" aria-label="Close dialog" title="Close" (click)="close()"> × </button>
        </header>

        <div class="tabs-toolbar palette-dialog-toolbar">
          <input
            #tabsSearch
            type="search"
            class="tabs-search palette-dialog-field"
            [value]="searchText"
            (input)="onSearchInput($event)"
            (keydown.enter)="reopenLatest(); $event.preventDefault(); $event.stopPropagation()"
            (keydown.arrowdown)="focusResult($event)" (keydown.arrowup)="focusResult($event, true)"
            placeholder="Filter closed tabs by title or path…"
            aria-label="Filter recently closed tabs" />
          <select class="palette-dialog-field" aria-label="Filter by original pane" [value]="paneFilter" (change)="onPaneFilterChange($event)">
            <option value="all">Both panes</option><option value="left">Left pane</option><option value="right">Right pane</option>
          </select>
          <button type="button" class="palette-dialog-button" [disabled]="!searchText && paneFilter === 'all'" (click)="resetFilters()">Reset filters</button>
        </div>

        @let matching = filteredEntries;
        <div class="tabs palette-dialog-body palette-dialog-list">
          <button
            #closedTabResult
            *ngFor="let entry of matching; trackBy: trackEntry"
            type="button"
            class="closed-tab palette-dialog-item"
            (click)="reopen(entry)">
            <span class="icon" aria-hidden="true">{{ icon(entry) }}</span>
            <span class="main">
              <strong>{{ entry.tab.title }}</strong>
              <small [title]="displayPath(entry)">{{ displayPath(entry) }}</small>
            </span>
            <span class="meta">
              {{ entry.paneId === 'left' ? 'Left pane' : 'Right pane' }} · {{ entry.closedAt | date:'short' }}
            </span>
          </button>

          <div class="empty palette-dialog-empty" *ngIf="entries.length === 0" role="status">
            No recently closed tabs in this workspace.
          </div>

          <div class="empty palette-dialog-empty" *ngIf="entries.length > 0 && matching.length === 0" role="status">
            No closed tabs match the current filters.
          </div>
        </div>

        <footer class="palette-dialog-footer">
          <span class="palette-dialog-meta" role="status">{{ matching.length }} of {{ entries.length }} shown</span>
          <div class="palette-dialog-footer-actions">
            <button type="button" class="palette-dialog-button" [disabled]="entries.length === 0" (click)="clear()">
              Clear Recovery List
            </button>
          </div>
          <div class="palette-dialog-footer-actions">
            <button type="button" class="palette-dialog-button" [disabled]="matching.length === 0" (click)="reopenLatest()">
              Reopen Latest Match
            </button>
            <button type="button" class="palette-dialog-button palette-dialog-button--primary" (click)="close()">Close</button>
          </div>
        </footer>
      </section>
    </div>
  `,
  styles: [`
    :host{position:fixed;inset:0;z-index:8750;pointer-events:none}
    .panel{display:grid;grid-template-rows:auto auto minmax(0,1fr) auto;width:min(720px,calc(100vw - 24px));height:min(600px,calc(100vh - 40px));overflow:hidden;border:1px solid var(--vsc-border);border-radius:7px;background:var(--vsc-dialog-background,var(--vsc-panel-background));color:var(--vsc-foreground);box-shadow:0 14px 48px var(--vsc-scrim);font:12px/1.4 var(--vsc-font-family,system-ui,sans-serif)}
    .tabs-toolbar{display:flex;padding:7px 10px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border))}
    .tabs-search{flex:1;min-width:min(180px,100%)}
    .tabs{min-height:0;overflow:auto}
    .palette-dialog-footer{flex-wrap:wrap}
    .closed-tab{display:grid;grid-template-columns:24px minmax(0,1fr) auto;align-items:center;gap:8px;width:100%;min-height:45px;padding:5px 8px;border:1px solid transparent;border-radius:4px;background:transparent;text-align:left;cursor:pointer}
    .closed-tab:hover,.closed-tab:focus-visible{background:var(--vsc-list-hover-background);border-color:var(--vsc-border)}
    .main{display:grid;min-width:0}.main strong,.main small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.main small,.meta{color:var(--vsc-description-foreground)}.meta{font-size:10px;font-variant-numeric:tabular-nums}
  `],
})
export class ClosedTabsComponent implements OnChanges, OnDestroy {
  @Input() visible = false;
  @Output() closed = new EventEmitter<void>();
  @Output() reopenRequested = new EventEmitter<RecentlyClosedTab>();

  searchText = '';
  paneFilter: 'all' | 'left' | 'right' = 'all';

  @ViewChildren('closedTabResult') private tabResults?: QueryList<ElementRef<HTMLButtonElement>>;

  @ViewChild('tabsSearch') set tabsSearch(field: ElementRef<HTMLInputElement> | undefined) {
    const input = field?.nativeElement;
    if (!input) return;
    queueMicrotask(() => {
      if (this.visible && input.isConnected) input.focus();
    });
  }
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;

  constructor(
    readonly workspaces: WorkspaceService,
    private readonly cdr: ChangeDetectorRef,
    private readonly confirm: ConfirmDialogService,
  ) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (!changes['visible']) return;
    if (this.visible) {
      this.resetFilters();
      if (this.modalStack && this.hostElement && !this.unregisterModal) {
        this.unregisterModal = this.modalStack.register({
          id: 'closed-tabs',
          element: this.hostElement.nativeElement,
          dismiss: () => this.close(),
          dismissible: () => this.visible,
        });
      }
    } else {
      this.unregisterModal?.();
      this.unregisterModal = null;
    }
  }

  ngOnDestroy(): void {
    this.unregisterModal?.();
    this.unregisterModal = null;
  }

  get entries(): readonly RecentlyClosedTab[] {
    return this.workspaces.getRecentlyClosedTabs();
  }

  get filteredEntries(): readonly RecentlyClosedTab[] {
    const query = this.searchText.trim().toLowerCase();
    const scoped = this.paneFilter === 'all' ? this.entries : this.entries.filter(entry => entry.paneId === this.paneFilter);
    if (!query) return scoped;
    return scoped.filter(entry =>
      (entry.tab.title ?? '').toLowerCase().includes(query) ||
      (entry.tab.path ?? '').toLowerCase().includes(query) ||
      this.viewerPath(entry).toLowerCase().includes(query) ||
      (entry.paneId ?? '').toLowerCase().includes(query),
    );
  }

  resetFilters(): void {
    this.searchText = '';
    this.paneFilter = 'all';
  }

  onPaneFilterChange(event: Event): void {
    const value = (event.target as HTMLSelectElement).value;
    this.paneFilter = value === 'left' || value === 'right' ? value : 'all';
  }

  focusResult(event: Event, last = false): void {
    const result = last ? this.tabResults?.last : this.tabResults?.first;
    if (!result) return;
    event.preventDefault();
    event.stopPropagation();
    result.nativeElement.focus();
  }

  onSearchInput(event: Event): void {
    this.searchText = (event.target as HTMLInputElement).value;
    this.cdr.markForCheck();
  }

  close(): void {
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.closed.emit();
  }

  reopen(entry: RecentlyClosedTab): void {
    this.reopenRequested.emit(entry);
  }

  reopenLatest(): void {
    const latest = this.filteredEntries[0];
    if (latest) {
      this.reopenRequested.emit(latest);
    }
  }

  async clear(): Promise<void> {
    const workspaceId = this.entries[0]?.workspaceId;
    if (!workspaceId) return;
    const confirmed = await this.confirm.confirm({
      title: 'Clear Recently Closed Tabs',
      message: 'Remove all recoverable tabs for this workspace, including tabs outside the current filter?',
      confirmText: 'Clear',
    });
    if (!confirmed) return;
    this.workspaces.clearRecentlyClosedTabs(workspaceId);
    this.cdr.markForCheck();
  }

  icon(entry: RecentlyClosedTab): string {
    const kind = entry.tab.kind ?? 'dir';
    if (kind === 'viewer') return '◫';
    if (kind === 'size-manager') return '▦';
    return '▣';
  }

  displayPath(entry: RecentlyClosedTab): string {
    return this.viewerPath(entry) || entry.tab.path || entry.tab.surfaceId || 'Workspace surface';
  }

  viewerPath(entry: RecentlyClosedTab): string {
    return entry.tab.viewer?.targetPath ?? '';
  }

  trackEntry(_index: number, entry: RecentlyClosedTab): string {
    return entry.id;
  }
}