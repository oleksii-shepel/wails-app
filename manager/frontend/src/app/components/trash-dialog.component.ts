import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  OnDestroy,
  OnInit,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, takeUntil, type Stream } from '@epikodelabs/streamix';

import {
  ApiService,
  type TrashEntry,
} from '../services/api.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { TrashDialogService, type TrashDialogRequest } from '../services/trash-dialog.service';
import { moveFocusWithin } from '../utils/focus-navigation.utils';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';
@Component({
  selector: 'app-trash-dialog',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      *ngIf="request$ | async"
      class="backdrop palette-dialog-backdrop dialog-group"
      [class.backdrop--closing]="isClosing"
      [class.dialog-group--closing]="isClosing"
      (mousedown)="onBackdrop($event)"
      (animationend)="onAnimationEnd($event)">
      <section
        #dialogPanel
        class="dialog palette-dialog-surface dialog-surface"
        role="dialog"
        aria-modal="true"
        aria-label="Trash / Recycle Bin"
        tabindex="-1"
        (keydown.tab)="onTabKey($event)"
         (keydown.escape)="close(); $event.preventDefault(); $event.stopPropagation()">
        <header class="palette-dialog-header">
          <div class="palette-dialog-title">
            <h2>Trash / Recycle Bin</h2>
            <p class="palette-dialog-subtitle">{{ filteredEntries.length }} of {{ entries.length }} shown · {{ selectedIds.size }} selected<span *ngIf="hiddenSelectionCount as count"> ({{ count }} outside filter)</span></p>
          </div>
          <button type="button" class="dialog-close" aria-label="Close dialog" title="Close" (click)="close()"> × </button>
        </header>

        <div class="toolbar palette-dialog-toolbar">
          <button
            type="button"
            class="palette-dialog-button"
            [disabled]="loading || confirmationPending || !canRestoreSelection"
            (click)="restoreSelected()">
            Restore
          </button>
          <span *ngIf="selectedIds.size && !canRestoreSelection" class="palette-dialog-meta" role="status">Some selected items cannot be restored.</span>
          <button
            type="button"
            class="palette-dialog-button palette-dialog-button--danger"
            [disabled]="loading || confirmationPending || selectedIds.size === 0"
            (click)="deleteSelected()">
            Delete permanently
          </button>
          <button
            type="button"
            class="palette-dialog-button"
            [disabled]="loading || confirmationPending || filteredEntries.length === 0"
            (click)="toggleSelectAll()">
            {{ allFilteredSelected ? 'Deselect matches' : 'Select matches' }}
          </button>
          <button type="button" class="palette-dialog-button" [disabled]="loading || confirmationPending || !hasRestorableMatches" (click)="selectRestorableMatches()">Select restorable</button>
          <input
            type="search"
            class="trash-search palette-dialog-field"
            [value]="searchText"
            (input)="onSearchInput($event)"
            placeholder="Filter by name or original path…"
            aria-label="Filter trash items" />
          <button type="button" class="palette-dialog-button" [disabled]="!searchText" (click)="clearSearch()">Clear filter</button>
          <select class="trash-sort palette-dialog-field" [value]="sortBy" (change)="onSortChange($event)" aria-label="Sort trash items">
            <option value="deleted">Newest deleted</option>
            <option value="oldest">Oldest deleted</option>
            <option value="name">Name</option>
            <option value="path">Original location</option>
          </select>
          <button type="button" class="palette-dialog-button" [disabled]="loading || confirmationPending" (click)="reload()">
            Refresh
          </button>
          <button
            type="button"
            class="palette-dialog-button palette-dialog-button--danger"
            [disabled]="loading || confirmationPending || entries.length === 0"
            (click)="emptyTrash()">
            Empty Trash
          </button>
        </div>

        <div
          class="trash-list palette-dialog-body palette-dialog-list"
          tabindex="0"
          role="listbox"
          [attr.aria-busy]="loading"
          aria-multiselectable="true"
          aria-label="Trash contents">
          <button
            *ngFor="let entry of filteredEntries; trackBy: trackById"
            type="button"
            class="trash-row palette-dialog-item"
            role="option"
            [class.selected]="selectedIds.has(entry.id)"
            [class.is-selected]="selectedIds.has(entry.id)"
            [attr.aria-selected]="selectedIds.has(entry.id)"
            [disabled]="loading || confirmationPending"
            (click)="toggle(entry, $event)"
            (dblclick)="openSelection()">
            <span class="icon" aria-hidden="true">{{ entry.isDir ? '📁' : '📄' }}</span>
            <span class="main">
              <strong>{{ entry.name }}</strong>
              <span [title]="entry.originalPath">{{ entry.originalPath || 'Original location unavailable' }}</span>
            </span>
            <span class="row-right">
              <span *ngIf="!entry.canRestore" class="restore-note">Restore unavailable</span>
              <span class="date">{{ formatDeleted(entry.deletedAt) }}</span>
            </span>
          </button>

          <div *ngIf="!loading && !error && entries.length === 0" class="empty palette-dialog-empty" role="status">
            Trash is empty.
          </div>

          <div *ngIf="!loading && entries.length > 0 && filteredEntries.length === 0" class="empty palette-dialog-empty" role="status">
            No matching items. Clear the search to see everything in the Trash.
          </div>

          <div *ngIf="loading" class="empty palette-dialog-empty" role="status">
            Loading…
          </div>
        </div>

        <footer *ngIf="error" class="palette-dialog-footer" role="alert">
          {{ error }}
        </footer>
      </section>
    </div>
  `,
  styleUrls: ['./tools-dialog-remote-theme.scss'],
  styles: [`
    .backdrop { z-index: 1200; place-items: center; }
    .backdrop--closing { pointer-events: none; }

    .dialog {
      width: min(900px, calc(100vw - 48px));
      height: min(680px, calc(100vh - 96px));
      min-width: 0;
      min-height: 0;
      display: flex;
      flex-direction: column;
      background: var(--vsc-menu-background, var(--vsc-panel-background));
    }

    header,
    .toolbar,
    footer {
      flex: 0 0 auto;
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 6px;
      padding: 9px 12px;
      border-bottom: 1px solid var(--vsc-panel-border, var(--vsc-control-background));
    }

    header {
      justify-content: space-between;
    }

    .dialog-close {
      flex-shrink: 0;
    }

    header h2 {
      margin: 0;
      font-size: 14px;
      font-weight: 600;
    }

    header p {
      margin: 2px 0 0;
      opacity: .72;
      font-size: 11px;
      font-variant-numeric: tabular-nums;
    }

    .trash-search { flex: 1; min-width: min(170px, 100%); }
    .trash-sort { max-width: 100%; }

    .trash-list {
      flex: 1 1 0%;
      min-height: 0;
      overflow: auto;
      padding: 4px;
      outline: none;
    }

    .trash-row {
      width: 100%;
      display: grid;
      grid-template-columns: 28px minmax(0, 1fr) auto;
      align-items: center;
      gap: 8px;
      padding: 7px 8px;
      text-align: left;
    }

    .trash-row:hover {
      background: var(--vsc-list-hoverBackground, var(--vsc-list-hover-background));
    }

    .trash-row.selected {
      background: var(--vsc-list-activeSelectionBackground, var(--vsc-list-selected-emphasis));
    }

    .main {
      min-width: 0;
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    .main strong,
    .main span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .row-right {
      display: flex;
      flex-direction: column;
      align-items: flex-end;
      gap: 2px;
    }

    .restore-note {
      font-size: 10px;
      color: var(--vsc-warning, var(--vsc-accent-yellow));
    }

    .main span,
    .date {
      opacity: .72;
      font-size: 11px;
      font-variant-numeric: tabular-nums;
    }

    .empty {
      display: grid;
      place-items: center;
      height: 100%;
      opacity: .7;
    }

    footer {
      border-top: 1px solid var(--vsc-panel-border, var(--vsc-control-background));
      border-bottom: 0;
      color: var(--vsc-errorForeground, var(--vsc-error));
    }
  `],
})
export class TrashDialogComponent implements OnInit, OnDestroy {
  @ViewChild('dialogPanel') private dialogPanel?: ElementRef<HTMLElement>;

  readonly request$: Stream<TrashDialogRequest | null> = this.dialog.mountedRequest$;
  loading = false;
  private listingRevision = 0;
  confirmationPending = false;
  error = '';
  entries: TrashEntry[] = [];
  selectedIds = new Set<string>();
  searchText = '';
  sortBy: 'deleted' | 'oldest' | 'name' | 'path' = 'deleted';

  private readonly destroy$ = createSubject<void>();
  private releaseDialogContext: (() => void) | null = null;
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;

  constructor(
    private readonly api: ApiService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly dialog: TrashDialogService,
    private readonly cdr: ChangeDetectorRef,
    private readonly inputContexts: InputContextService,
    private readonly inputEngine: InputEngineService,
  ) {}

  ngOnInit(): void {
    this.inputEngine.registerCommand('dialog.trash.close', () => this.close());
    this.inputEngine.registerCommand('dialog.trash.focusNext', () => this.moveDialogFocus(1));
    this.inputEngine.registerCommand('dialog.trash.focusPrev', () => this.moveDialogFocus(-1));
    this.inputEngine.registerCommand('dialog.trash.openSelection', () => this.openSelection());

    this.dialog.activeRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe(request => {
        if (request) {
          if (this.modalStack && this.hostElement && !this.unregisterModal) {
            this.unregisterModal = this.modalStack.register({
              id: 'trash-dialog',
              contextId: 'dialog.trash',
              element: this.hostElement.nativeElement,
              dismiss: () => this.close(),
              dismissible: () => !this.isClosing,
            });
          }
        } else {
          this.unregisterModal?.();
          this.unregisterModal = null;
        }
      });

    this.dialog.mountedRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe(request => {
        if (request) {
          this.selectedIds = new Set();
          this.searchText = '';
          this.error = '';
          this.releaseDialogContext ??= this.inputContexts.acquire('dialog.trash');
          this.focusInitialControl();
          void this.reload();
          return;
        }

        this.listingRevision++;
        this.selectedIds = new Set();
        this.loading = false;
        this.error = '';
        this.entries = [];
        this.releaseDialogContext?.();
        this.releaseDialogContext = null;
        this.cdr.markForCheck();
      });
  }

  ngOnDestroy(): void {
    this.listingRevision++;
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.inputEngine.unregisterCommand('dialog.trash.close');
    this.inputEngine.unregisterCommand('dialog.trash.focusNext');
    this.inputEngine.unregisterCommand('dialog.trash.focusPrev');
    this.inputEngine.unregisterCommand('dialog.trash.openSelection');
    this.destroy$.next();
    this.destroy$.complete();
  }

  get filteredEntries(): TrashEntry[] {
    const query = this.searchText.trim().toLowerCase();
    const filtered = query
      ? this.entries.filter(
          entry =>
            entry.name.toLowerCase().includes(query) ||
            (entry.originalPath ?? '').toLowerCase().includes(query),
        )
      : [...this.entries];

    if (this.sortBy === 'name') {
      return filtered.sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }));
    }
    if (this.sortBy === 'path') {
      return filtered.sort((a, b) => (a.originalPath ?? '').localeCompare(b.originalPath ?? ''));
    }
    return filtered.sort((a, b) => {
      const ta = Date.parse(a.deletedAt || '') || 0;
      const tb = Date.parse(b.deletedAt || '') || 0;
      return this.sortBy === 'oldest' ? ta - tb : tb - ta;
    });
  }

  get hasRestorableMatches(): boolean { return this.filteredEntries.some(entry => entry.canRestore); }

  selectRestorableMatches(): void {
    if (this.loading || this.confirmationPending) return;
    this.selectedIds = new Set(this.filteredEntries.filter(entry => entry.canRestore).map(entry => entry.id));
    this.cdr.markForCheck();
  }

  get allFilteredSelected(): boolean {
    const visible = this.filteredEntries;
    return visible.length > 0 && visible.every(entry => this.selectedIds.has(entry.id));
  }

  get selectedEntries(): TrashEntry[] {
    return this.entries.filter(entry => this.selectedIds.has(entry.id));
  }

  get hiddenSelectionCount(): number {
    const matchingSelected = this.filteredEntries.filter(entry => this.selectedIds.has(entry.id)).length;
    return this.selectedIds.size - matchingSelected;
  }

  get canRestoreSelection(): boolean {
    const selected = this.selectedEntries;
    return selected.length > 0 && selected.every(entry => entry.canRestore);
  }

  clearSearch(): void {
    this.searchText = '';
    this.cdr.markForCheck();
  }

  onSearchInput(event: Event): void {
    this.searchText = (event.target as HTMLInputElement).value;
    this.cdr.markForCheck();
  }

  onSortChange(event: Event): void {
    const val = (event.target as HTMLSelectElement).value;
    if (val === 'deleted' || val === 'oldest' || val === 'name' || val === 'path') {
      this.sortBy = val;
      this.cdr.markForCheck();
    }
  }

  toggleSelectAll(): void {
    if (this.loading || this.confirmationPending) return;
    const visible = this.filteredEntries;
    if (!visible.length) return;
    if (this.allFilteredSelected) {
      const next = new Set(this.selectedIds);
      for (const entry of visible) next.delete(entry.id);
      this.selectedIds = next;
    } else {
      const next = new Set(this.selectedIds);
      for (const entry of visible) next.add(entry.id);
      this.selectedIds = next;
    }
    this.cdr.markForCheck();
  }

  trackById(_: number, entry: TrashEntry): string {
    return entry.id;
  }

  toggle(entry: TrashEntry, event: MouseEvent): void {
    if (this.loading || this.confirmationPending) return;
    const next = new Set(this.selectedIds);
    if (event.ctrlKey || event.metaKey) {
      next.has(entry.id) ? next.delete(entry.id) : next.add(entry.id);
    } else {
      next.clear();
      next.add(entry.id);
    }
    this.selectedIds = next;
    this.cdr.markForCheck();
  }

  formatDeleted(value: string): string {
    if (!value) return '';
    const date = new Date(value);
    return Number.isFinite(date.getTime()) ? date.toLocaleString() : value;
  }

  onTabKey(event: Event): void {
    if (!(event instanceof KeyboardEvent)) {
      return;
    }
    if (!this.dialogPanel?.nativeElement) {
      return;
    }
    moveFocusWithin(this.dialogPanel.nativeElement, event.shiftKey ? -1 : 1);
    event.preventDefault();
    event.stopPropagation();
  }

  async reload(): Promise<void> {
    const revision = ++this.listingRevision;
    this.loading = true;
    this.error = '';
    this.cdr.markForCheck();
    try {
      const entries = await this.api.listTrashEntries();
      if (revision !== this.listingRevision) return;
      this.entries = entries;
      const available = new Set(this.entries.map(entry => entry.id));
      this.selectedIds = new Set(
        [...this.selectedIds].filter(id => available.has(id)),
      );
    } catch (error) {
      if (revision === this.listingRevision) this.error = error instanceof Error ? error.message : String(error);
    } finally {
      if (revision === this.listingRevision) {
        this.loading = false;
        this.cdr.markForCheck();
      }
    }
  }

  async restoreSelected(): Promise<void> {
    if (this.loading || this.confirmationPending) return;
    const selected = this.selectedEntries;
    if (selected.length === 0 || !this.canRestoreSelection) return;

    await this.mutateSelected(selected, 'restore', entry => this.api.restoreTrashEntry(entry.id));
  }

  async deleteSelected(): Promise<void> {
    if (this.loading || this.confirmationPending) return;
    const selected = this.selectedEntries;
    if (selected.length === 0) return;

    const request = this.dialog.activeRequest$.value;
    if (!request) return;
    const hidden = this.hiddenSelectionCount;
    const hiddenMessage = hidden ? ` ${hidden} selected ${hidden === 1 ? 'item is' : 'items are'} outside the filter.` : '';
    const confirmed = await this.confirmTrashChange({
      title: 'Delete Permanently',
      message:
        selected.length === 1
          ? `Delete "${selected[0].name}" permanently?${hiddenMessage}`
          : `Delete ${selected.length} Trash items permanently?${hiddenMessage}`,
      confirmText: 'Delete',
      danger: true,
    });
    if (!confirmed || this.loading || this.dialog.activeRequest$.value !== request) return;

    await this.mutateSelected(selected, 'delete', entry => this.api.deleteTrashEntry(entry.id));
  }

  async emptyTrash(): Promise<void> {
    if (this.loading || this.confirmationPending || this.entries.length === 0) return;

    const request = this.dialog.activeRequest$.value;
    if (!request) return;
    const confirmed = await this.confirmTrashChange({
      title: 'Empty Trash',
      message: 'Permanently delete every item in Trash / Recycle Bin?',
      confirmText: 'Empty Trash',
      danger: true,
    });
    if (!confirmed || this.loading || this.dialog.activeRequest$.value !== request) return;

    this.loading = true;
    this.error = '';
    this.cdr.markForCheck();
    try {
      await this.api.emptyTrash();
      this.dialog.notifyMutated();
      if (this.dialog.activeRequest$.value !== request) return;
      this.selectedIds = new Set();
      await this.reload();
    } catch (error) {
      if (this.dialog.activeRequest$.value !== request) return;
      this.error = error instanceof Error ? error.message : String(error);
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  private async mutateSelected(
    selected: readonly TrashEntry[], action: 'restore' | 'delete', mutate: (entry: TrashEntry) => Promise<unknown>,
  ): Promise<void> {
    const request = this.dialog.activeRequest$.value;
    if (!request) return;
    this.loading = true;
    this.error = '';
    this.cdr.markForCheck();
    const failed: Array<{ entry: TrashEntry; message: string }> = [];
    for (const entry of selected) {
      try {
        await mutate(entry);
      } catch (error) {
        failed.push({ entry, message: error instanceof Error ? error.message : String(error) });
      }
    }
    if (failed.length < selected.length) this.dialog.notifyMutated();
    if (this.dialog.activeRequest$.value !== request) return;
    this.selectedIds = new Set(failed.map(item => item.entry.id));
    await this.reload();
    if (this.dialog.activeRequest$.value !== request || !failed.length) return;
    const first = failed[0];
    this.error = [`Could not ${action} ${failed.length} of ${selected.length} Trash items.`,
      `${first.entry.name}: ${first.message}`, this.error].filter(Boolean).join(' ');
    this.cdr.markForCheck();
  }

  private async confirmTrashChange(options: Parameters<ConfirmDialogService['confirm']>[0]): Promise<boolean> {
    this.confirmationPending = true;
    this.cdr.markForCheck();
    try {
      return await this.confirmDialog.confirm(options);
    } finally {
      this.confirmationPending = false;
      this.cdr.markForCheck();
    }
  }

  close(): void {
    this.dialog.close();
  }

  onBackdrop(event: MouseEvent): void {
    if (event.target === event.currentTarget) {
      this.close();
    }
  }

  private focusInitialControl(): void {
    requestAnimationFrame(() => {
      const panel = this.dialogPanel?.nativeElement;
      if (!panel || !this.dialog.activeRequest$.value) {
        return;
      }

      const firstFocusable = panel.querySelector<HTMLElement>('button:not([disabled]), [tabindex]:not([tabindex="-1"])');
      if (firstFocusable) {
        firstFocusable.focus({ preventScroll: true });
        return;
      }

      panel.focus({ preventScroll: true });
    });
  }

  private moveDialogFocus(delta: number): void {
    if (!this.dialogPanel?.nativeElement) {
      return;
    }
    moveFocusWithin(this.dialogPanel.nativeElement, delta);
  }

  openSelection(): void {
    if (this.canRestoreSelection) {
      void this.restoreSelected();
    }
  }

  get isClosing(): boolean {
    return !!this.dialog.closingRequest$.value;
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) {
      return;
    }
    this.dialog.finishClose();
  }
}