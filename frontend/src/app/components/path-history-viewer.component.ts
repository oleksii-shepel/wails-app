import { Component, OnInit, OnDestroy, ViewChild, ElementRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import { PathHistoryService, PathHistoryEntry } from '../services/path-history.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';
import { locationIcon, locationLabel } from '../services/location-reference';
import { DialogFocusTrapDirective } from '../directives/dialog-focus-trap.directive';
import { VscodeScrollbarDirective } from '../directives/vscode-scrollbar.directive';

@Component({
  selector: 'app-path-history-viewer',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogFocusTrapDirective, VscodeScrollbarDirective],
  template: `
    <div class="path-history-overlay" *ngIf="isVisible" (click)="hide()">
      <div class="path-history-modal" [appTrapFocus]="true" role="dialog" aria-modal="true" tabindex="-1" (click)="$event.stopPropagation()">
        <!-- Header -->
        <div class="modal-header">
          <div>
            <h2>Location History</h2>
            <p>Recently visited locations. Click to navigate.</p>
          </div>
          <button type="button" class="dialog-close-btn" aria-label="Close" (click)="hide()" title="Close (Esc)">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 7.293l3.536-3.536.707.707L8.707 8l3.536 3.536-.707.707L8 8.707l-3.536 3.536-.707-.707L7.293 8 3.757 4.464l.707-.707L8 7.293z"/></svg>
          </button>
        </div>

        <!-- Stats -->
        <div class="stats-bar">
          <div class="stat-item">
            <span class="stat-label">Total Locations:</span>
            <span class="stat-value">{{ historyEntries.length }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">Most Visited:</span>
            <span class="stat-value">{{ getMostVisited() ? normalizeEntryPath(getMostVisited()?.location) : 'None' }}</span>
          </div>
          <button (click)="clearHistory()" class="btn-clear-history">
            Clear All
          </button>
        </div>

        <!-- Search Filter -->
        <div class="search-bar">
          <input
            #filterInput
            data-dialog-initial-focus
            type="text"
            [(ngModel)]="filterText"
            (ngModelChange)="applyFilter()"
            placeholder="Filter locations..."
            class="filter-input"
          />
        </div>

        <!-- History List -->
        <div class="history-list" appVscodeScrollbar>
          <div *ngIf="filteredEntries.length === 0" class="empty-state">
            <span class="empty-icon">📭</span>
            <p>No history entries found</p>
            <p class="empty-hint">Navigate to some locations to build history</p>
          </div>

          <div
            *ngFor="let entry of filteredEntries; let i = index"
            class="history-entry"
            [class.selected]="i === selectedIndex"
            [class.recent]="isRecent(entry)"
            tabindex="0"
            role="button"
            (focus)="selectedIndex = i"
            (mouseenter)="selectedIndex = i"
            (click)="selectPath(entry)"
          >
            <div class="entry-icon">{{ entryIcon(entry) }}</div>
            <div class="entry-info">
              <div class="entry-path">{{ entryLabel(entry) }}</div>
              <div class="entry-location" [title]="entry.location">{{ entry.location }}</div>
              <div class="entry-meta">
                <span class="entry-pane">{{ entry.paneId }} pane</span>
                <span class="entry-count">{{ entry.accessCount }} visit{{ entry.accessCount > 1 ? 's' : '' }}</span>
                <span class="entry-time">{{ formatTime(entry.timestamp) }}</span>
              </div>
            </div>
            <button
              (click)="removePath(entry, $event)"
              class="btn-remove"
              title="Remove from history"
            >
              ✕
            </button>
          </div>
        </div>

        <!-- Footer -->
        <div class="modal-footer">
          <div class="footer-info">
            Showing {{ filteredEntries.length }} of {{ historyEntries.length }} locations
          </div>
          <div class="footer-actions">
            <button (click)="exportHistory()" class="btn-secondary">
              Export
            </button>
            <button (click)="hide()" class="btn-primary">
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .path-history-overlay {
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background: var(--vsc-scrim);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: var(--vsc-z-modal);
      backdrop-filter: blur(2px);
      animation: fadeIn 0.2s ease;
      padding: var(--vsc-padding-xl);
    }

    @keyframes fadeIn {
      from { opacity: 0; }
      to { opacity: 1; }
    }

    .path-history-modal {
      background: var(--vsc-editor-background);
      border: 1px solid var(--vsc-border-bright);
      border-radius: var(--vsc-border-radius);
      width: 90%;
      max-width: 800px;
      max-height: 80vh;
      display: flex;
      flex-direction: column;
      box-shadow: var(--vsc-widget-shadow), 0 24px 56px rgba(0, 0, 0, 0.45);
      animation: slideDown 0.28s cubic-bezier(0.16, 1, 0.3, 1);
      color: var(--vsc-foreground);
      overflow: hidden;
    }

    @keyframes slideDown {
      from { transform: translateY(-18px); opacity: 0; }
      to { transform: translateY(0); opacity: 1; }
    }

    .modal-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      padding: 12px 16px;
      border-bottom: 1px solid var(--vsc-border-subtle);
      background: var(--vsc-titlebar-background);
      flex-shrink: 0;
    }

    .modal-header h2 {
      margin: 0;
      font-size: 16px;
      font-weight: 600;
      color: var(--vsc-foreground-bright);
      line-height: 1.2;
    }

    .modal-header p {
      margin: 4px 0 0;
      color: var(--vsc-foreground-dim);
      font-size: 12px;
      line-height: 1.45;
    }

    .dialog-close-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 28px;
      height: 28px;
      flex-shrink: 0;
      margin-top: -2px;
      padding: 0;
      border: none;
      background: transparent;
      color: var(--vsc-foreground-dim);
      border-radius: var(--vsc-border-radius-sm);
      cursor: pointer;
      transition: background-color var(--vsc-transition-fast), color var(--vsc-transition-fast);
    }

    .dialog-close-btn:hover {
      background: color-mix(in srgb, var(--vsc-foreground) 10%, transparent);
      color: var(--vsc-foreground-bright);
    }

    .dialog-close-btn:focus-visible {
      outline: 1px solid var(--vsc-input-focus-border);
      outline-offset: 2px;
    }

    .stats-bar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: var(--vsc-padding-md) var(--vsc-padding-lg);
      background: color-mix(in srgb, var(--vsc-panel-background) 85%, var(--vsc-sidebar-background) 15%);
      border-bottom: 1px solid var(--vsc-border-subtle);
      gap: 1rem;
      flex-wrap: wrap;
    }

    .stat-item {
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }

    .stat-label {
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-muted);
    }

    .stat-value {
      font-size: var(--vsc-font-size);
      color: var(--vsc-foreground);
      font-weight: 600;
      font-family: var(--vsc-font-family-mono);
    }

    .btn-clear-history {
      padding: 0.4rem 0.75rem;
      background: var(--vsc-error);
      border: 1px solid transparent;
      border-radius: var(--vsc-border-radius-sm);
      color: white;
      cursor: pointer;
      font-size: var(--vsc-font-size-small);
      transition: all var(--vsc-transition-fast);
    }

    .btn-clear-history:hover {
      filter: brightness(1.08);
    }

    .btn-clear-history:focus-visible {
      outline: 1px solid var(--vsc-input-focus-border);
      outline-offset: 1px;
    }

    .search-bar {
      padding: var(--vsc-padding-md) var(--vsc-padding-lg);
      border-bottom: 1px solid var(--vsc-border-subtle);
    }

    .filter-input {
      width: 100%;
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-input-border);
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-input-foreground);
      font-size: var(--vsc-font-size);
    }

    .filter-input:focus {
      outline: none;
      border-color: var(--vsc-input-focus-border);
    }

    .filter-input::placeholder {
      color: var(--vsc-input-placeholder);
    }

    .history-list {
      flex: 1;
      overflow-y: auto;
      padding: var(--vsc-padding-xs) 0;
    }

    .empty-state {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      padding: 3rem 1.5rem;
      text-align: center;
      color: var(--vsc-foreground-muted);
    }

    .empty-icon {
      font-size: 3rem;
      margin-bottom: 1rem;
      opacity: 0.5;
    }

    .empty-state p {
      margin: 0.5rem 0;
      font-size: 1rem;
      color: var(--vsc-foreground);
    }

    .empty-hint {
      font-size: 0.9rem;
      color: var(--vsc-foreground-muted) !important;
    }

    .history-entry {
      display: flex;
      align-items: center;
      gap: 1rem;
      padding: 0.75rem 1.5rem;
      cursor: pointer;
      transition: all 0.15s;
      position: relative;
    }

    .history-entry:hover {
      background: var(--vsc-list-hover-background);
    }

    .history-entry.selected {
      background: var(--vsc-list-active-background, var(--vsc-list-hover-background));
    }

    .history-entry.recent {
      border-left: 3px solid var(--vsc-accent-blue);
    }

    .entry-icon {
      font-size: 1.5rem;
      flex-shrink: 0;
    }

    .entry-info {
      flex: 1;
      min-width: 0;
    }

    .entry-path {
      font-size: var(--vsc-font-size);
      color: var(--vsc-foreground);
      font-family: var(--vsc-font-family-mono);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      margin-bottom: 0.25rem;
    }

    .entry-meta {
      display: flex;
      gap: 1rem;
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-muted);
    }

    .entry-pane {
      text-transform: capitalize;
    }

    .entry-count {
      color: var(--vsc-accent-blue);
      font-weight: 600;
    }

    .btn-remove {
      padding: 0.25rem 0.5rem;
      background: transparent;
      border: 1px solid transparent;
      color: var(--vsc-foreground-muted);
      cursor: pointer;
      font-size: 1rem;
      opacity: 0;
      transition: all var(--vsc-transition-fast);
      border-radius: var(--vsc-border-radius-sm);
    }

    .history-entry:hover .btn-remove {
      opacity: 1;
    }

    .btn-remove:hover {
      background: var(--vsc-error);
      color: white;
    }

    .btn-remove:focus-visible {
      opacity: 1;
      outline: 1px solid var(--vsc-input-focus-border);
      outline-offset: 1px;
    }

    .modal-footer {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: var(--vsc-padding-md) var(--vsc-padding-lg);
      border-top: 1px solid var(--vsc-border-subtle);
      background: color-mix(in srgb, var(--vsc-panel-background) 85%, var(--vsc-sidebar-background) 15%);
    }

    .footer-info {
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-muted);
    }

    .footer-actions {
      display: flex;
      gap: 0.75rem;
    }

    .btn-primary,
    .btn-secondary {
      min-width: 86px;
      height: 28px;
      padding: 0 var(--vsc-padding-md);
      border: 1px solid transparent;
      border-radius: var(--vsc-border-radius-sm);
      cursor: pointer;
      font-size: var(--vsc-font-size);
      transition: all var(--vsc-transition-fast);
      outline: none;
    }

    .btn-primary {
      background: var(--vsc-button-background);
      color: var(--vsc-button-foreground);
    }

    .btn-primary:hover {
      background: var(--vsc-button-hover);
    }

    .btn-secondary {
      background: var(--vsc-button-secondary);
      color: var(--vsc-button-secondary-foreground);
    }

    .btn-secondary:hover {
      background: var(--vsc-button-secondary-hover);
    }

    .btn-primary:focus-visible,
    .btn-secondary:focus-visible {
      outline: 1px solid var(--vsc-input-focus-border);
      outline-offset: 1px;
    }

    @media (max-width: 900px) {
      .path-history-modal {
        width: 100%;
        max-height: calc(100vh - 2 * var(--vsc-padding-xl));
      }

      .modal-footer {
        flex-direction: column;
        align-items: flex-start;
        gap: var(--vsc-padding-sm);
      }

      .footer-actions {
        width: 100%;
        justify-content: flex-end;
      }
    }
  `]
})
export class PathHistoryViewerComponent implements OnInit, OnDestroy {
  @ViewChild('filterInput') filterInput?: ElementRef<HTMLInputElement>;

  isVisible = false;
  historyEntries: PathHistoryEntry[] = [];
  filteredEntries: PathHistoryEntry[] = [];
  filterText = '';
  selectedIndex = 0;

  private destroy$ = createSubject<void>();
  private releaseDialogContext: (() => void) | null = null;

  constructor(
    private pathHistoryService: PathHistoryService,
    private confirmDialog: ConfirmDialogService,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
    private overlayFreeze: NativeOverlayFreezeService,
  ) {}

  ngOnInit(): void {
    this.registerInputCommands();
    // Subscribe to history changes
    this.pathHistoryService.history$
      .pipe(takeUntil(this.destroy$))
      .subscribe(entries => {
        this.historyEntries = entries;
        this.applyFilter();
      });

    // Load initial history
    this.historyEntries = this.pathHistoryService.getAllHistory();
    this.applyFilter();
  }

  ngOnDestroy(): void {
    this.overlayFreeze.release('dialog.pathHistory');
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.unregisterInputCommands();
    this.destroy$.next();
    this.destroy$.complete();
  }

  show(): void {
    this.isVisible = true;
    this.releaseDialogContext ??= this.inputContexts.acquire('dialog.pathHistory');
    this.overlayFreeze.acquire('dialog.pathHistory');
    this.filterText = '';
    this.historyEntries = this.pathHistoryService.getAllHistory();
    this.applyFilter();
    this.selectedIndex = 0;
  }

  hide(): void {
    this.isVisible = false;
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.overlayFreeze.release('dialog.pathHistory');
  }

  normalizeEntryPath(location: string | null | undefined): string {
    return location ?? '';
  }

  entryLabel(entry: PathHistoryEntry): string {
    return locationLabel(entry.location);
  }

  entryIcon(entry: PathHistoryEntry): string {
    return locationIcon(entry.scheme);
  }

  applyFilter(): void {
    if (!this.filterText || this.filterText.trim() === '') {
      this.filteredEntries = [...this.historyEntries];
    } else {
      const filter = this.normalizeEntryPath(this.filterText).toLowerCase();
      this.filteredEntries = this.historyEntries.filter(entry =>
        this.normalizeEntryPath(entry.location).toLowerCase().includes(filter)
      );
    }

    if (this.filteredEntries.length === 0) {
      this.selectedIndex = 0;
      return;
    }

    this.selectedIndex = Math.max(0, Math.min(this.selectedIndex, this.filteredEntries.length - 1));
  }

  selectPath(entry: PathHistoryEntry): void {
    console.log('Selected path:', this.normalizeEntryPath(entry.location));
    // Emit event to navigate to this path
    // This would need to be wired up to the main component
  }

  async removePath(entry: PathHistoryEntry, event: Event): Promise<void> {
    event.stopPropagation();
    const normalizedPath = this.normalizeEntryPath(entry.location);
    const confirmed = await this.confirmDialog.confirm({
      title: 'Remove History Entry',
      message: `Remove "${normalizedPath}" from history?`,
      confirmText: 'Remove',
      danger: true,
    });
    if (confirmed) {
      this.pathHistoryService.removeLocation(normalizedPath);
    }
  }

  async clearHistory(): Promise<void> {
    const confirmed = await this.confirmDialog.confirm({
      title: 'Clear Location History',
      message: 'Clear all path history?',
      confirmText: 'Clear',
      danger: true,
    });
    if (confirmed) {
      this.pathHistoryService.clearHistory();
    }
  }

  getMostVisited(): PathHistoryEntry | null {
    if (this.historyEntries.length === 0) return null;
    return this.historyEntries.reduce((max, entry) =>
      entry.accessCount > max.accessCount ? entry : max
    );
  }

  isRecent(entry: PathHistoryEntry): boolean {
    const oneHourAgo = Date.now() - (60 * 60 * 1000);
    return entry.timestamp > oneHourAgo;
  }

  formatTime(timestamp: number): string {
    const now = Date.now();
    const diff = now - timestamp;
    const minutes = Math.floor(diff / 60000);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (minutes < 1) return 'just now';
    if (minutes < 60) return `${minutes}m ago`;
    if (hours < 24) return `${hours}h ago`;
    if (days < 7) return `${days}d ago`;
    return new Date(timestamp).toLocaleDateString();
  }

  exportHistory(): void {
    const normalizedEntries = this.historyEntries.map(entry => ({
      ...entry,
      path: this.normalizeEntryPath(entry.location),
    }));
    const data = JSON.stringify(normalizedEntries, null, 2);
    const blob = new Blob([data], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `path-history-${Date.now()}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }

  selectPathAt(index: number): void {
    if (index < 0 || index >= this.filteredEntries.length) return;
    this.selectedIndex = index;
    this.selectPath(this.filteredEntries[index]);
  }

  async removeSelectedPath(): Promise<void> {
    const entry = this.filteredEntries[this.selectedIndex];
    if (!entry) return;

    const normalizedPath = this.normalizeEntryPath(entry.location);
    const confirmed = await this.confirmDialog.confirm({
      title: 'Remove History Entry',
      message: `Remove "${normalizedPath}" from history?`,
      confirmText: 'Remove',
      danger: true,
    });
    if (confirmed) {
      this.pathHistoryService.removeLocation(normalizedPath);
    }
  }

  private moveSelection(delta: number): void {
    if (!this.isVisible || this.filteredEntries.length === 0) return;
    this.selectedIndex = Math.max(0, Math.min(this.selectedIndex + delta, this.filteredEntries.length - 1));
  }

  private registerInputCommands(): void {
    this.inputEngine.registerCommand('dialog.pathHistory.close', () => {
      if (!this.isVisible) return;
      this.hide();
    });

    this.inputEngine.registerCommand('dialog.pathHistory.focusFilter', () => {
      if (!this.isVisible) return;
      this.filterInput?.nativeElement.focus();
      this.filterInput?.nativeElement.select();
    });

    this.inputEngine.registerCommand('dialog.pathHistory.prevItem', () => {
      if (!this.isVisible) return;
      this.moveSelection(-1);
    });

    this.inputEngine.registerCommand('dialog.pathHistory.nextItem', () => {
      if (!this.isVisible) return;
      this.moveSelection(1);
    });

    this.inputEngine.registerCommand('dialog.pathHistory.openSelected', () => {
      if (!this.isVisible) return;
      this.selectPathAt(this.selectedIndex);
    });

    this.inputEngine.registerCommand('dialog.pathHistory.removeSelected', () => {
      if (!this.isVisible) return;
      return this.removeSelectedPath();
    });
  }

  private unregisterInputCommands(): void {
    this.inputEngine.unregisterCommand('dialog.pathHistory.close');
    this.inputEngine.unregisterCommand('dialog.pathHistory.focusFilter');
    this.inputEngine.unregisterCommand('dialog.pathHistory.prevItem');
    this.inputEngine.unregisterCommand('dialog.pathHistory.nextItem');
    this.inputEngine.unregisterCommand('dialog.pathHistory.openSelected');
    this.inputEngine.unregisterCommand('dialog.pathHistory.removeSelected');
  }
}