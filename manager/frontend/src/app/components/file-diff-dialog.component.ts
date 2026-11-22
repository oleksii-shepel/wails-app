import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  OnDestroy,
  OnInit,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

import {
  FileDiffService,
  type DiffResult,
  type DiffRow,
  type LoadedDiffFile,
} from '../services/file-diff.service';
import {
  FileDiffDialogService,
  type FileDiffDialogRequest,
} from '../services/file-diff-dialog.service';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';
import { FileType, type FileInfo } from '../shared/protocol';

type DiffViewMode = 'split' | 'unified';

@Component({
  selector: 'app-file-diff-dialog',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="dialog-backdrop palette-dialog-backdrop dialog-group"
      *ngIf="request"
      [class.dialog-group--closing]="closing"
      (mousedown)="close()"
      (animationend)="onAnimationEnd($event)">
      <section
        class="dialog-panel diff-panel palette-dialog-surface dialog-surface"
        role="dialog"
        aria-modal="true"
        aria-label="Compare Files (Diff)"
        (mousedown)="$event.stopPropagation()"
        tabindex="-1"
        (keydown.escape)="onEscape($event)">

        <header class="dialog-header palette-dialog-header">
          <div class="palette-dialog-title">
            <strong>File & Text Diff</strong>
            <span class="palette-dialog-subtitle" *ngIf="diffResult">
              +{{ diffResult.stats.added }} added · -{{ diffResult.stats.removed }} removed · ~{{ diffResult.stats.modified }} modified · {{ diffResult.hunks.length }} hunks
            </span>
          </div>
          <span
            class="dialog-close"
            role="button"
            tabindex="0"
            aria-label="Close"
            title="Close"
            (click)="close()"
            (keydown.enter)="close()"
            (keydown.space)="close(); $event.preventDefault()">×</span>
        </header>

        <div class="diff-controls palette-dialog-toolbar">
          <div class="diff-path-group">
            <label class="diff-path-label" for="diff-left-path">Left</label>
            <select
              *ngIf="fileCandidates.length > 0"
              class="diff-select"
              [value]="leftPath"
              (change)="onLeftSelect($event)"
              aria-label="Select left file">
              <option value="">Choose file…</option>
              <option *ngFor="let item of fileCandidates" [value]="item.path">
                {{ item.name }}
              </option>
            </select>
            <input
              id="diff-left-path"
              type="text"
              class="diff-path-input palette-dialog-mono"
              [value]="leftPath"
              (input)="onLeftInput($event)"
              (keydown.enter)="compare()"
              placeholder="Left file path…"
              aria-label="Left file path" />
          </div>

          <button
            type="button"
            class="diff-swap-btn"
            title="Swap left and right files"
            (click)="swapSides()">
            ⇄ Swap
          </button>

          <div class="diff-path-group">
            <label class="diff-path-label" for="diff-right-path">Right</label>
            <select
              *ngIf="fileCandidates.length > 0"
              class="diff-select"
              [value]="rightPath"
              (change)="onRightSelect($event)"
              aria-label="Select right file">
              <option value="">Choose file…</option>
              <option *ngFor="let item of fileCandidates" [value]="item.path">
                {{ item.name }}
              </option>
            </select>
            <input
              id="diff-right-path"
              type="text"
              class="diff-path-input palette-dialog-mono"
              [value]="rightPath"
              (input)="onRightInput($event)"
              (keydown.enter)="compare()"
              placeholder="Right file path…"
              aria-label="Right file path" />
          </div>

          <button
            type="button"
            class="primary"
            [disabled]="loading || !leftPath.trim() || !rightPath.trim()"
            (click)="compare()">
            {{ loading ? 'Comparing…' : 'Compare' }}
          </button>
        </div>

        <div class="diff-options-bar">
          <div class="diff-mode-group" role="tablist" aria-label="Diff view mode">
            <button
              type="button"
              role="tab"
              [attr.aria-selected]="viewMode === 'split'"
              [class.active]="viewMode === 'split'"
              (click)="setViewMode('split')">
              Side-by-Side
            </button>
            <button
              type="button"
              role="tab"
              [attr.aria-selected]="viewMode === 'unified'"
              [class.active]="viewMode === 'unified'"
              (click)="setViewMode('unified')">
              Unified
            </button>
          </div>

          <label class="diff-check">
            <input type="checkbox" [checked]="ignoreWhitespace" (change)="onIgnoreWhitespaceChange($event)" />
            Ignore whitespace
          </label>
          <label class="diff-check">
            <input type="checkbox" [checked]="ignoreCase" (change)="onIgnoreCaseChange($event)" />
            Ignore case
          </label>
          <label class="diff-check">
            <input type="checkbox" [checked]="changesOnly" (change)="onChangesOnlyChange($event)" />
            Changes only
          </label>

          <div class="diff-spacer"></div>

          <div class="diff-hunk-nav" *ngIf="diffResult && diffResult.hunks.length > 0">
            <button type="button" (click)="prevHunk()" [disabled]="diffResult.hunks.length === 0">Prev Hunk</button>
            <span class="diff-hunk-count">Hunk {{ activeHunkIndex + 1 }} / {{ diffResult.hunks.length }}</span>
            <button type="button" (click)="nextHunk()" [disabled]="diffResult.hunks.length === 0">Next Hunk</button>
          </div>

          <button
            type="button"
            *ngIf="diffResult && !diffResult.isIdentical"
            (click)="copyPatch()">
            {{ patchCopied ? 'Patch Copied' : 'Copy Patch' }}
          </button>
        </div>

        <div class="diff-body">
          <div class="diff-error" *ngIf="errorMessage">{{ errorMessage }}</div>

          <div class="diff-empty" *ngIf="!loading && !errorMessage && !diffResult">
            Choose two text files and click Compare to inspect line-by-line differences.
          </div>

          <div class="diff-identical" *ngIf="!loading && !errorMessage && diffResult?.isIdentical">
            Files are identical ({{ diffResult!.stats.totalLeftLines }} lines matching under current options).
          </div>

          <div class="diff-split-table" *ngIf="!loading && !errorMessage && diffResult && visibleRows.length > 0 && viewMode === 'split'">
            <div class="diff-split-header">
              <span>#</span>
              <span class="palette-dialog-mono" [title]="leftPath">{{ fileLabel(leftPath) || 'Left' }} ({{ diffResult.stats.totalLeftLines }} lines)</span>
              <span>#</span>
              <span class="palette-dialog-mono" [title]="rightPath">{{ fileLabel(rightPath) || 'Right' }} ({{ diffResult.stats.totalRightLines }} lines)</span>
            </div>
            <div
              *ngFor="let row of visibleRows; trackBy: trackRow"
              class="diff-split-row"
              [class.diff-row-add]="row.kind === 'add'"
              [class.diff-row-remove]="row.kind === 'remove'"
              [class.diff-row-modify]="row.kind === 'modify'"
              [class.diff-row-focused]="focusedRowIndex === row.index"
              [attr.data-diff-row]="row.index">
              <span class="diff-gutter">{{ row.leftLine ?? '' }}</span>
              <pre class="diff-cell diff-cell-left" [class.diff-cell-empty]="row.kind === 'add'">{{ row.leftText }}</pre>
              <span class="diff-gutter">{{ row.rightLine ?? '' }}</span>
              <pre class="diff-cell diff-cell-right" [class.diff-cell-empty]="row.kind === 'remove'">{{ row.rightText }}</pre>
            </div>
          </div>

          <div class="diff-unified-table" *ngIf="!loading && !errorMessage && diffResult && visibleRows.length > 0 && viewMode === 'unified'">
            <ng-container *ngFor="let row of visibleRows; trackBy: trackRow">
              <div
                *ngIf="row.kind === 'equal'"
                class="diff-uni-row"
                [attr.data-diff-row]="row.index">
                <span class="diff-gutter">{{ row.leftLine ?? '' }}</span>
                <span class="diff-gutter">{{ row.rightLine ?? '' }}</span>
                <span class="diff-sign"> </span>
                <pre class="diff-cell">{{ row.leftText }}</pre>
              </div>
              <div
                *ngIf="row.kind === 'remove' || row.kind === 'modify'"
                class="diff-uni-row diff-row-remove"
                [class.diff-row-focused]="focusedRowIndex === row.index"
                [attr.data-diff-row]="row.index">
                <span class="diff-gutter">{{ row.leftLine ?? '' }}</span>
                <span class="diff-gutter"></span>
                <span class="diff-sign">-</span>
                <pre class="diff-cell">{{ row.leftText }}</pre>
              </div>
              <div
                *ngIf="row.kind === 'add' || row.kind === 'modify'"
                class="diff-uni-row diff-row-add"
                [class.diff-row-focused]="focusedRowIndex === row.index">
                <span class="diff-gutter"></span>
                <span class="diff-gutter">{{ row.rightLine ?? '' }}</span>
                <span class="diff-sign">+</span>
                <pre class="diff-cell">{{ row.rightText }}</pre>
              </div>
            </ng-container>
          </div>
        </div>

        <footer class="diff-footer palette-dialog-footer">
          <span class="palette-dialog-footnote" *ngIf="leftLoaded && rightLoaded">
            Left: {{ formatBytes(leftLoaded.size) }} · Right: {{ formatBytes(rightLoaded.size) }}
          </span>
          <span class="palette-dialog-footnote" *ngIf="!leftLoaded || !rightLoaded">
            Compare text files up to 2 MB
          </span>
          <button type="button" (click)="close()">Close</button>
        </footer>
      </section>
    </div>
  `,
  styleUrls: ['./tools-dialog-remote-theme.scss'],
  styles: [`
    :host {
      position: fixed;
      inset: 0;
      z-index: 9520;
      pointer-events: none;
    }

    .diff-panel.diff-panel {
      width: min(1160px, calc(100vw - 24px));
      height: min(780px, calc(100vh - min(12vh, 110px) - 32px));
      display: flex;
      flex-direction: column;
    }

    .diff-controls {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr) auto;
      gap: 8px;
      align-items: center;
      padding: 8px 10px;
      border-bottom: 1px solid var(--vsc-border-subtle);
    }

    .diff-path-group {
      display: flex;
      align-items: center;
      gap: 6px;
      min-width: 0;
    }

    .diff-path-label {
      font-size: 11px;
      font-weight: 600;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      white-space: nowrap;
    }

    .diff-select {
      max-width: 150px;
      height: 26px;
      padding: 0 6px;
    }

    .diff-path-input {
      flex: 1;
      min-width: 120px;
      height: 26px;
      padding: 0 8px;
      font-size: 11px;
    }

    .diff-swap-btn {
      height: 26px;
      padding: 0 8px;
      border: 1px solid var(--vsc-border);
      white-space: nowrap;
    }

    .diff-options-bar {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 10px;
      padding: 6px 10px;
      border-bottom: 1px solid var(--vsc-border-subtle);
      background: var(--vsc-neutral-overlay-subtle);
      font-size: 11px;
    }

    .diff-mode-group {
      display: inline-flex;
      border: 1px solid var(--vsc-border);
      border-radius: 4px;
      overflow: hidden;
    }

    .diff-mode-group button {
      border: 0;
      border-radius: 0;
      padding: 3px 10px;
      font-size: 11px;
    }

    .diff-mode-group button.active {
      background: var(--vsc-list-active-background);
      color: var(--vsc-list-active-foreground);
    }

    .diff-check {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      cursor: pointer;
    }

    .diff-spacer {
      flex: 1;
    }

    .diff-hunk-nav {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      font-variant-numeric: tabular-nums;
    }

    .diff-hunk-nav button {
      padding: 2px 8px;
      border: 1px solid var(--vsc-border);
    }

    .diff-hunk-count {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .diff-body {
      flex: 1;
      min-height: 0;
      overflow: auto;
      background: var(--vsc-editor-background);
    }

    .diff-empty,
    .diff-identical {
      padding: 40px 20px;
      text-align: center;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .diff-identical {
      color: var(--vsc-accent-green, var(--vsc-success));
    }

    .diff-error {
      margin: 12px;
      padding: 10px 12px;
      border-radius: 4px;
      border: 1px solid var(--vsc-error-border);
      background: var(--vsc-error-surface);
      color: var(--vsc-error-foreground, var(--vsc-error));
    }

    .diff-split-header {
      position: sticky;
      top: 0;
      z-index: 2;
      display: grid;
      grid-template-columns: 46px minmax(0, 1fr) 46px minmax(0, 1fr);
      gap: 0;
      background: var(--vsc-panel-background);
      border-bottom: 1px solid var(--vsc-border);
      font-size: 11px;
      font-weight: 600;
    }

    .diff-split-header span {
      padding: 5px 8px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      border-right: 1px solid var(--vsc-border-subtle);
    }

    .diff-split-row {
      display: grid;
      grid-template-columns: 46px minmax(0, 1fr) 46px minmax(0, 1fr);
      border-bottom: 1px solid var(--vsc-neutral-overlay-subtle);
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 11.5px;
      line-height: 1.45;
    }

    .diff-uni-row {
      display: grid;
      grid-template-columns: 44px 44px 18px minmax(0, 1fr);
      border-bottom: 1px solid var(--vsc-neutral-overlay-subtle);
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 11.5px;
      line-height: 1.45;
    }

    .diff-gutter {
      padding: 1px 6px;
      text-align: right;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      background: var(--vsc-neutral-overlay-subtle);
      border-right: 1px solid var(--vsc-border-subtle);
      user-select: none;
      font-variant-numeric: tabular-nums;
    }

    .diff-sign {
      text-align: center;
      user-select: none;
      font-weight: 600;
      border-right: 1px solid var(--vsc-border-subtle);
    }

    .diff-cell {
      margin: 0;
      padding: 1px 8px;
      white-space: pre-wrap;
      word-break: break-word;
      font-family: inherit;
      border-right: 1px solid var(--vsc-border-subtle);
    }

    .diff-cell-empty {
      background: var(--vsc-neutral-overlay-subtle);
    }

    .diff-row-add {
      background: var(--vsc-success-surface);
    }

    .diff-row-add .diff-gutter,
    .diff-row-add .diff-sign {
      color: var(--vsc-accent-green, var(--vsc-success));
    }

    .diff-row-remove {
      background: var(--vsc-error-surface);
    }

    .diff-row-remove .diff-gutter,
    .diff-row-remove .diff-sign {
      color: var(--vsc-accent-red, var(--vsc-error));
    }

    .diff-row-modify .diff-cell-left {
      background: var(--vsc-error-surface);
    }

    .diff-row-modify .diff-cell-right {
      background: var(--vsc-success-surface);
    }

    .diff-row-focused {
      outline: 1px solid var(--vsc-accent-blue);
      outline-offset: -1px;
    }

    .diff-footer {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      padding: 7px 10px;
      border-top: 1px solid var(--vsc-border-subtle);
    }

    .diff-panel button.primary {
      border-color: color-mix(in srgb, var(--vsc-accent-blue) 55%, var(--vsc-border));
      background: color-mix(in srgb, var(--vsc-accent-blue) 18%, transparent);
    }

    @media (max-width: 820px) {
      .diff-controls {
        grid-template-columns: minmax(0, 1fr);
      }
    }
  `],
})
export class FileDiffDialogComponent implements OnInit, OnDestroy {
  request: FileDiffDialogRequest | null = null;
  closing = false;

  leftPath = '';
  rightPath = '';
  fileCandidates: FileInfo[] = [];

  leftLoaded: LoadedDiffFile | null = null;
  rightLoaded: LoadedDiffFile | null = null;
  diffResult: DiffResult | null = null;

  loading = false;
  errorMessage = '';
  viewMode: DiffViewMode = 'split';
  ignoreWhitespace = false;
  ignoreCase = false;
  changesOnly = false;
  activeHunkIndex = 0;
  focusedRowIndex: number | null = null;
  patchCopied = false;

  private readonly destroy$ = createSubject<void>();
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;
  private copiedTimer: number | null = null;

  constructor(
    private readonly dialog: FileDiffDialogService,
    private readonly diffService: FileDiffService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get visibleRows(): readonly DiffRow[] {
    if (!this.diffResult) return [];
    if (!this.changesOnly) return this.diffResult.rows;
    return this.diffResult.rows.filter(r => r.kind !== 'equal');
  }

  ngOnInit(): void {
    this.dialog.mountedRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe((req: FileDiffDialogRequest | null) => {
        this.request = req;
        this.closing = false;
        if (req) {
          this.fileCandidates = (req.candidates ?? []).filter(
            (item: FileInfo) => item.type !== FileType.DIRECTORY,
          );
          this.leftPath = req.leftPath ?? '';
          this.rightPath = req.rightPath ?? '';
          this.errorMessage = '';
          this.diffResult = null;
          this.leftLoaded = null;
          this.rightLoaded = null;
          this.activeHunkIndex = 0;
          this.focusedRowIndex = null;
          if (this.modalStack && this.hostElement && !this.unregisterModal) {
            this.unregisterModal = this.modalStack.register({
              id: 'file-diff-dialog',
              element: this.hostElement.nativeElement,
              dismiss: () => this.close(),
              dismissible: () => !!this.request && !this.closing,
            });
          }
          if (this.leftPath && this.rightPath) {
            void this.compare();
          }
        } else {
          this.unregisterModal?.();
          this.unregisterModal = null;
        }
        this.cdr.markForCheck();
      });

    this.dialog.closingRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe((req: FileDiffDialogRequest | null) => {
        this.closing = !!req;
        this.cdr.markForCheck();
      });
  }

  ngOnDestroy(): void {
    this.unregisterModal?.();
    this.unregisterModal = null;
    if (this.copiedTimer !== null) {
      window.clearTimeout(this.copiedTimer);
      this.copiedTimer = null;
    }
    this.destroy$.next();
    this.destroy$.complete();
  }

  onLeftSelect(event: Event): void {
    this.leftPath = (event.target as HTMLSelectElement).value;
    this.cdr.markForCheck();
  }

  onRightSelect(event: Event): void {
    this.rightPath = (event.target as HTMLSelectElement).value;
    this.cdr.markForCheck();
  }

  onLeftInput(event: Event): void {
    this.leftPath = (event.target as HTMLInputElement).value;
  }

  onRightInput(event: Event): void {
    this.rightPath = (event.target as HTMLInputElement).value;
  }

  swapSides(): void {
    const tmp = this.leftPath;
    this.leftPath = this.rightPath;
    this.rightPath = tmp;
    const tmpLoaded = this.leftLoaded;
    this.leftLoaded = this.rightLoaded;
    this.rightLoaded = tmpLoaded;
    if (this.leftLoaded && this.rightLoaded && !this.leftLoaded.error && !this.rightLoaded.error) {
      this.recomputeDiff();
    } else if (this.leftPath && this.rightPath) {
      void this.compare();
    } else {
      this.cdr.markForCheck();
    }
  }

  setViewMode(mode: DiffViewMode): void {
    this.viewMode = mode;
    this.cdr.markForCheck();
  }

  onIgnoreWhitespaceChange(event: Event): void {
    this.ignoreWhitespace = (event.target as HTMLInputElement).checked;
    this.recomputeDiff();
  }

  onIgnoreCaseChange(event: Event): void {
    this.ignoreCase = (event.target as HTMLInputElement).checked;
    this.recomputeDiff();
  }

  onChangesOnlyChange(event: Event): void {
    this.changesOnly = (event.target as HTMLInputElement).checked;
    this.cdr.markForCheck();
  }

  async compare(): Promise<void> {
    const left = this.leftPath.trim();
    const right = this.rightPath.trim();
    if (!left || !right || this.loading) return;

    this.loading = true;
    this.errorMessage = '';
    this.diffResult = null;
    this.cdr.markForCheck();

    try {
      const [leftFile, rightFile] = await Promise.all([
        this.diffService.loadFileForDiff(left),
        this.diffService.loadFileForDiff(right),
      ]);
      this.leftLoaded = leftFile;
      this.rightLoaded = rightFile;

      if (leftFile.error) {
        this.errorMessage = `Left file: ${leftFile.error}`;
      } else if (rightFile.error) {
        this.errorMessage = `Right file: ${rightFile.error}`;
      } else {
        this.recomputeDiff();
      }
    } finally {
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  private recomputeDiff(): void {
    if (!this.leftLoaded || !this.rightLoaded) return;
    if (this.leftLoaded.error || this.rightLoaded.error) return;
    this.diffResult = this.diffService.computeLineDiff(
      this.leftLoaded.content,
      this.rightLoaded.content,
      {
        ignoreWhitespace: this.ignoreWhitespace,
        ignoreCase: this.ignoreCase,
      },
    );
    this.activeHunkIndex = 0;
    this.focusedRowIndex = this.diffResult.hunks[0]?.firstRowIndex ?? null;
    this.cdr.markForCheck();
  }

  prevHunk(): void {
    if (!this.diffResult?.hunks.length) return;
    const count = this.diffResult.hunks.length;
    this.activeHunkIndex = (this.activeHunkIndex - 1 + count) % count;
    this.scrollToHunk(this.activeHunkIndex);
  }

  nextHunk(): void {
    if (!this.diffResult?.hunks.length) return;
    const count = this.diffResult.hunks.length;
    this.activeHunkIndex = (this.activeHunkIndex + 1) % count;
    this.scrollToHunk(this.activeHunkIndex);
  }

  private scrollToHunk(index: number): void {
    const hunk = this.diffResult?.hunks[index];
    if (!hunk) return;
    this.focusedRowIndex = hunk.firstRowIndex;
    this.cdr.markForCheck();
    queueMicrotask(() => {
      const el = this.hostElement?.nativeElement.querySelector<HTMLElement>(
        `[data-diff-row="${hunk.firstRowIndex}"]`,
      );
      el?.scrollIntoView({ block: 'center', behavior: 'smooth' });
    });
  }

  async copyPatch(): Promise<void> {
    if (!this.diffResult) return;
    const patch = this.diffService.formatUnifiedPatch(
      this.leftPath,
      this.rightPath,
      this.diffResult,
    );
    try {
      await navigator.clipboard.writeText(patch);
      this.patchCopied = true;
      if (this.copiedTimer !== null) window.clearTimeout(this.copiedTimer);
      this.copiedTimer = window.setTimeout(() => {
        this.patchCopied = false;
        this.copiedTimer = null;
        this.cdr.markForCheck();
      }, 1800);
      this.cdr.markForCheck();
    } catch {
      // Clipboard may be unavailable in headless environments.
    }
  }

  fileLabel(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/');
    const idx = normalized.lastIndexOf('/');
    return idx >= 0 ? normalized.slice(idx + 1) : normalized;
  }

  formatBytes(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
    const val = bytes / Math.pow(1024, i);
    return `${val >= 10 || i === 0 ? val.toFixed(0) : val.toFixed(1)} ${units[i]}`;
  }

  trackRow(_index: number, row: DiffRow): number {
    return row.index;
  }

  onEscape(event: Event): void {
    event.preventDefault();
    event.stopPropagation();
    this.close();
  }

  close(): void {
    this.dialog.close();
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (!this.closing || event.target !== event.currentTarget) return;
    this.dialog.finishClose();
  }
}
