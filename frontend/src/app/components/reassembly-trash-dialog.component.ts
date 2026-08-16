import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  OnDestroy,
  OnInit,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { Subscription } from 'rxjs';

import {
  ApiService,
  type TrashEntry,
} from '../services/api.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { ReassemblyTrashDialogService } from '../services/reassembly-trash-dialog.service';
import { ReassemblyModalSurfaceDirective } from './reassembly-modal-surface.directive';

@Component({
  selector: 'app-reassembly-trash-dialog',
  standalone: true,
  imports: [CommonModule, ReassemblyModalSurfaceDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      *ngIf="visible"
      class="backdrop"
      (mousedown)="onBackdrop($event)">
      <section
        class="dialog"
        role="dialog"
        aria-modal="true"
        aria-label="Trash / Recycle Bin"
        reassemblyModalSurface
        reassemblyModalInitial=".trash-list"
        (keydown.escape)="close()">
        <header>
          <div>
            <h2>Trash / Recycle Bin</h2>
            <p>{{ entries.length }} item{{ entries.length === 1 ? '' : 's' }}</p>
          </div>
          <button type="button" (click)="close()" title="Close">×</button>
        </header>

        <div class="toolbar">
          <button
            type="button"
            [disabled]="loading || !canRestoreSelection"
            (click)="restoreSelected()">
            Restore
          </button>
          <button
            type="button"
            [disabled]="loading || selectedIds.size === 0"
            (click)="deleteSelected()">
            Delete permanently
          </button>
          <span class="spacer"></span>
          <button type="button" [disabled]="loading" (click)="reload()">
            Refresh
          </button>
          <button
            type="button"
            class="danger"
            [disabled]="loading || entries.length === 0"
            (click)="emptyTrash()">
            Empty Trash
          </button>
        </div>

        <div
          class="trash-list"
          tabindex="0"
          role="listbox"
          aria-label="Trash contents">
          <button
            *ngFor="let entry of entries; trackBy: trackById"
            type="button"
            class="trash-row"
            [class.selected]="selectedIds.has(entry.id)"
            [attr.aria-selected]="selectedIds.has(entry.id)"
            (click)="toggle(entry, $event)">
            <span class="icon">{{ entry.isDir ? '📁' : '📄' }}</span>
            <span class="main">
              <strong>{{ entry.name }}</strong>
              <span>{{ entry.originalPath || 'Original location unavailable' }}</span>
            </span>
            <span class="date">{{ formatDeleted(entry.deletedAt) }}</span>
          </button>

          <div *ngIf="!loading && entries.length === 0" class="empty">
            Trash is empty.
          </div>

          <div *ngIf="loading" class="empty">
            Loading…
          </div>
        </div>

        <footer *ngIf="error" role="alert">
          {{ error }}
        </footer>
      </section>
    </div>
  `,
  styles: [`
    .backdrop {
      position: fixed;
      inset: 0;
      z-index: 1200;
      display: grid;
      place-items: center;
      background: var(--vsc-scrim);
    }

    .dialog {
      width: min(900px, calc(100vw - 48px));
      height: min(680px, calc(100vh - 48px));
      min-width: 0;
      min-height: 0;
      display: flex;
      flex-direction: column;
      background: var(--vsc-editor-background, #1e1e1e);
      color: var(--vsc-foreground, #cccccc);
      border: 1px solid var(--vsc-panel-border, #3c3c3c);
      box-shadow: 0 18px 60px rgba(0, 0, 0, .45);
    }

    header,
    .toolbar,
    footer {
      flex: 0 0 auto;
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 10px 12px;
      border-bottom: 1px solid var(--vsc-panel-border, #3c3c3c);
    }

    header {
      justify-content: space-between;
    }

    header h2 {
      margin: 0;
      font-size: 15px;
      font-weight: 600;
    }

    header p {
      margin: 2px 0 0;
      opacity: .68;
      font-size: 12px;
    }

    button {
      color: inherit;
      background: var(--vsc-button-secondaryBackground, #313131);
      border: 1px solid var(--vsc-panel-border, #454545);
      min-height: 28px;
    }

    button:disabled {
      opacity: .45;
    }

    button.danger {
      color: var(--vsc-errorForeground, #f48771);
    }

    .spacer {
      flex: 1;
    }

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
      background: transparent;
      border: 0;
    }

    .trash-row:hover {
      background: var(--vsc-list-hoverBackground, #2a2d2e);
    }

    .trash-row.selected {
      background: var(--vsc-list-activeSelectionBackground, #04395e);
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

    .main span,
    .date {
      opacity: .7;
      font-size: 11px;
    }

    .empty {
      display: grid;
      place-items: center;
      height: 100%;
      opacity: .7;
    }

    footer {
      border-top: 1px solid var(--vsc-panel-border, #3c3c3c);
      border-bottom: 0;
      color: var(--vsc-errorForeground, #f48771);
    }
  `],
})
export class ReassemblyTrashDialogComponent implements OnInit, OnDestroy {
  visible = false;
  loading = false;
  error = '';
  entries: TrashEntry[] = [];
  selectedIds = new Set<string>();

  private readonly subscription = new Subscription();

  constructor(
    private readonly api: ApiService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly dialog: ReassemblyTrashDialogService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    this.subscription.add(
      this.dialog.openRequested$.subscribe(() => {
        this.visible = true;
        this.selectedIds = new Set();
        this.error = '';
        void this.reload();
      }),
    );
  }

  ngOnDestroy(): void {
    this.subscription.unsubscribe();
  }

  get selectedEntries(): TrashEntry[] {
    return this.entries.filter(entry => this.selectedIds.has(entry.id));
  }

  get canRestoreSelection(): boolean {
    const selected = this.selectedEntries;
    return selected.length > 0 && selected.every(entry => entry.canRestore);
  }

  trackById(_: number, entry: TrashEntry): string {
    return entry.id;
  }

  toggle(entry: TrashEntry, event: MouseEvent): void {
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

  async reload(): Promise<void> {
    this.loading = true;
    this.error = '';
    this.cdr.markForCheck();
    try {
      this.entries = await this.api.listTrashEntries();
      const available = new Set(this.entries.map(entry => entry.id));
      this.selectedIds = new Set(
        [...this.selectedIds].filter(id => available.has(id)),
      );
    } catch (error) {
      this.error = error instanceof Error ? error.message : String(error);
    } finally {
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  async restoreSelected(): Promise<void> {
    const selected = this.selectedEntries;
    if (selected.length === 0 || !this.canRestoreSelection) return;

    this.loading = true;
    this.error = '';
    this.cdr.markForCheck();
    try {
      for (const entry of selected) {
        await this.api.restoreTrashEntry(entry.id);
      }
      this.selectedIds = new Set();
      await this.reload();
    } catch (error) {
      this.error = error instanceof Error ? error.message : String(error);
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  async deleteSelected(): Promise<void> {
    const selected = this.selectedEntries;
    if (selected.length === 0) return;

    const confirmed = await this.confirmDialog.confirm({
      title: 'Delete Permanently',
      message:
        selected.length === 1
          ? `Delete "${selected[0].name}" permanently?`
          : `Delete ${selected.length} Trash items permanently?`,
      confirmText: 'Delete',
      danger: true,
    });
    if (!confirmed) return;

    this.loading = true;
    this.error = '';
    this.cdr.markForCheck();
    try {
      for (const entry of selected) {
        await this.api.deleteTrashEntry(entry.id);
      }
      this.selectedIds = new Set();
      await this.reload();
    } catch (error) {
      this.error = error instanceof Error ? error.message : String(error);
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  async emptyTrash(): Promise<void> {
    if (this.entries.length === 0) return;

    const confirmed = await this.confirmDialog.confirm({
      title: 'Empty Trash',
      message: 'Permanently delete every item in Trash / Recycle Bin?',
      confirmText: 'Empty Trash',
      danger: true,
    });
    if (!confirmed) return;

    this.loading = true;
    this.error = '';
    this.cdr.markForCheck();
    try {
      await this.api.emptyTrash();
      this.selectedIds = new Set();
      await this.reload();
    } catch (error) {
      this.error = error instanceof Error ? error.message : String(error);
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  close(): void {
    this.visible = false;
    this.selectedIds = new Set();
    this.cdr.markForCheck();
  }

  onBackdrop(event: MouseEvent): void {
    if (event.target === event.currentTarget) {
      this.close();
    }
  }
}