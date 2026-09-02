import { createSubject, takeUntil } from '@epikodelabs/streamix';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  EventEmitter,
  HostListener,
  Input,
  OnDestroy,
  OnInit,
  Output,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { FileType } from '../shared/protocol';
import { SelectionBasketService, type SelectionBasketEntry } from '../services/selection-basket.service';
import type {
  SelectionBasketDestination,
  SelectionBasketOperation,
  SelectionBasketOperationRequest,
} from './selection-basket-operation';

@Component({
  selector: 'app-selection-basket',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      *ngIf="visible || isClosing"
      class="basket-backdrop palette-dialog-backdrop dialog-group"
      [class.dialog-group--closing]="isClosing"
      (mousedown)="hide()"
      (animationend)="onAnimationEnd($event)">
      <aside
        class="basket-panel palette-dialog-surface dialog-surface"
        role="dialog"
        aria-modal="true"
        aria-label="Selection Basket"
        (mousedown)="$event.stopPropagation()">
        <header class="basket-header palette-dialog-header">
          <div class="basket-title palette-dialog-title">
            <strong>Selection Basket</strong>
            <span class="palette-dialog-subtitle">{{ entries.length }} items</span>
          </div>
          <span
            class="dialog-close"
            title="Close"
            aria-label="Close Selection Basket"
            (click)="hide()"
            role="button"
            tabindex="0"
            (keydown.enter)="hide()"
            (keydown.space)="hide(); $event.preventDefault()"> × </span>
        </header>

        <div class="basket-summary">
          <div><strong>{{ entries.length }}</strong><span>Items</span></div>
          <div><strong>{{ knownSizeLabel }}</strong><span>Known size</span></div>
          <div><strong>{{ sourceCount }}</strong><span>Sources</span></div>
        </div>

        <div class="basket-workspace">
          <section class="basket-list palette-dialog-body" aria-label="Basket contents">
            <div *ngIf="entries.length === 0" class="empty-state palette-dialog-empty">
              <strong>The basket is empty</strong>
              <span>Add selections from panes, searches, duplicate results, or commands.</span>
              <code>basket.add</code>
            </div>

            <article *ngFor="let entry of entries; trackBy: trackEntry" class="basket-row">
              <div class="entry-icon" aria-hidden="true">{{ iconFor(entry) }}</div>
              <div class="entry-main">
                <strong [title]="entry.label">{{ entry.label }}</strong>
                <div class="entry-path" [title]="entry.path">{{ entry.path }}</div>
                <div class="entry-meta">
                  <span *ngIf="entry.size !== undefined">{{ formatBytes(entry.size) }}</span>
                  <span *ngIf="entry.source">from {{ entry.source }}</span>
                  <span>{{ ageLabel(entry.addedAt) }}</span>
                </div>
              </div>
              <button class="remove-button" type="button" title="Remove from basket" (click)="remove(entry.id)">×</button>
            </article>
          </section>

          <section class="batch-panel" aria-label="Batch operation">
            <div class="batch-section">
              <div class="section-label">Operation</div>
              <div class="operation-tabs" role="group" aria-label="Batch operation">
                <button type="button" [class.active]="operation === 'copy'" (click)="selectOperation('copy')">Copy</button>
                <button type="button" [class.active]="operation === 'move'" (click)="selectOperation('move')">Move</button>
                <button type="button" class="danger-tab" [class.active]="operation === 'delete'" (click)="selectOperation('delete')">Delete</button>
              </div>
            </div>

            <div class="batch-section" *ngIf="operation !== 'delete'">
              <label class="section-label" for="basket-destination">Destination</label>
              <select id="basket-destination" [value]="destinationSide" (change)="setDestination($any($event.target).value)">
                <option value="left" [disabled]="!leftDestinationWritable">Left pane — {{ leftDestination || 'Unavailable' }}</option>
                <option value="right" [disabled]="!rightDestinationWritable">Right pane — {{ rightDestination || 'Unavailable' }}</option>
              </select>
              <div class="section-note" *ngIf="!destinationWritable">Choose a writable directory pane.</div>
            </div>

            <div class="batch-section">
              <div class="section-label">Options</div>
              <label class="check-row">
                <input type="checkbox" [checked]="recursive" (change)="recursive = $any($event.target).checked" />
                <span>Include subdirectories</span>
              </label>
              <label class="check-row" *ngIf="operation === 'delete'">
                <input type="checkbox" [checked]="useTrash" (change)="useTrash = $any($event.target).checked" />
                <span>Move to Recycle Bin when available</span>
              </label>
              <div class="section-note" *ngIf="operation !== 'delete'">Conflicts are resolved by the existing operation conflict prompt.</div>
            </div>

            <div class="batch-section batch-preview">
              <div class="section-label">Ready</div>
              <strong>{{ operationLabel }}</strong>
              <span>{{ operationSummary }}</span>
            </div>
          </section>
        </div>

        <footer class="basket-footer">
          <button type="button" class="palette-dialog-button" [disabled]="!entries.length" (click)="clear()">Clear Basket</button>
          <span class="footer-spacer"></span>
          <span class="footer-status">{{ entries.length }} item{{ entries.length === 1 ? '' : 's' }}</span>
          <button
            type="button"
            class="palette-dialog-button execute-button"
            [class.danger]="operation === 'delete'"
            [disabled]="!canExecute"
            (click)="execute()">
            {{ executeLabel }}
          </button>
        </footer>
      </aside>
    </div>
  `,
  styles: [`
    :host { position: fixed; inset: 0; z-index: 9460; pointer-events: none; }

    .basket-panel {
      box-sizing: border-box;
      display: flex;
      flex-direction: column;
      width: min(940px, calc(100vw - 24px));
      height: min(700px, calc(100vh - 40px));
      overflow: hidden;
      border: 1px solid var(--vsc-border, var(--vsc-border));
      border-radius: 8px;
      background: var(--vsc-menu-background, var(--vsc-panel-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
      box-shadow: 0 18px 60px var(--vsc-scrim);
      font: 12px/1.4 var(--vsc-font-family, system-ui, sans-serif);
    }

    .basket-header,.basket-footer { display:flex; align-items:center; gap:9px; padding:9px 10px; }
    .basket-header { justify-content:space-between; border-bottom:1px solid var(--vsc-border-subtle, var(--vsc-border-subtle)); }
    .basket-title { display:flex; gap:8px; align-items:baseline; min-width:0; }
    .basket-title strong { font-weight:600; }
    .basket-title span,.basket-footer>span,.entry-path,.entry-meta,.basket-summary span,.section-note,.batch-preview span,.footer-status { color:var(--vsc-description-foreground, var(--vsc-foreground-dim)); }

    button,select { font:inherit; }
    button { color:inherit; }
    button:disabled,select:disabled { opacity:.35; cursor:default; }

    .basket-summary { display:grid; grid-template-columns:repeat(3,1fr); border-bottom:1px solid var(--vsc-border-subtle, var(--vsc-border-subtle)); }
    .basket-summary>div { display:flex; align-items:baseline; gap:6px; min-width:0; padding:6px 10px; }
    .basket-summary>div+div { border-left:1px solid var(--vsc-border-subtle, var(--vsc-border-subtle)); }
    .basket-summary strong { font-size:12px; font-weight:600; }
    .basket-summary span { font-size:10px; }

    .basket-workspace { display:grid; grid-template-columns:minmax(0,1fr) 300px; flex:1; min-height:0; }
    .basket-list { min-height:0; overflow:auto; padding:5px; border-right:1px solid var(--vsc-border-subtle, var(--vsc-border-subtle)); }
    .basket-row { display:grid; grid-template-columns:30px minmax(0,1fr) 27px; gap:8px; align-items:center; min-height:52px; padding:5px 8px; border:1px solid transparent; border-radius:4px; }
    .basket-row:hover { background:var(--vsc-list-hover-background, var(--vsc-list-hover-background)); }
    .entry-icon { display:flex; align-items:center; justify-content:center; font-size:16px; }
    .entry-main { min-width:0; display:grid; }
    .entry-main strong,.entry-path { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
    .entry-path { font-family:var(--vsc-font-family-mono, ui-monospace, monospace); font-size:10px; }
    .entry-meta { display:flex; gap:4px 10px; margin-top:2px; font-size:10px; }

    .remove-button,.basket-footer button,.operation-tabs button {
      min-height:27px; padding:3px 8px; border:1px solid transparent; border-radius:4px; background:transparent; cursor:pointer;
    }
    .remove-button { width:27px; padding:0; font-size:16px; opacity:.7; }
    .remove-button:hover,.basket-footer button:hover:not(:disabled),.operation-tabs button:hover:not(:disabled) { border-color:var(--vsc-border, var(--vsc-border)); background:var(--vsc-list-hover-background, var(--vsc-list-hover-background)); opacity:1; }

    .batch-panel { min-width:0; overflow:auto; background:var(--vsc-editor-background, var(--vsc-panel-background)); }
    .batch-section { display:grid; gap:7px; padding:12px; border-bottom:1px solid var(--vsc-border-subtle, var(--vsc-border-subtle)); }
    .section-label { color:var(--vsc-description-foreground, var(--vsc-foreground-dim)); font-size:10px; font-weight:600; text-transform:uppercase; letter-spacing:.04em; }
    .section-note { font-size:10px; }
    .operation-tabs { display:grid; grid-template-columns:repeat(3,1fr); gap:4px; }
    .operation-tabs button { border-color:var(--vsc-border-subtle, var(--vsc-border)); }
    .operation-tabs button.active { border-color:var(--vsc-focus-border, var(--vsc-accent-blue)); background:var(--vsc-list-active-background, var(--vsc-list-hover-background)); }
    .operation-tabs .danger-tab.active,.execute-button.danger { color:var(--vsc-error-foreground, var(--vsc-error)); }

    select { width:100%; min-width:0; height:28px; padding:2px 6px; border:1px solid var(--vsc-border, var(--vsc-border)); border-radius:4px; background:var(--vsc-input-background, var(--vsc-editor-background)); color:var(--vsc-input-foreground, var(--vsc-foreground)); }
    .check-row { display:flex; align-items:center; gap:7px; min-height:24px; }
    .check-row input { margin:0; }
    .batch-preview strong { font-weight:600; }

    .empty-state { padding:38px; text-align:center; color:var(--vsc-description-foreground, var(--vsc-foreground-dim)); }
    .empty-state strong,.empty-state span,.empty-state code { display:block; }
    .empty-state strong { color:var(--vsc-foreground); }
    .empty-state span { margin-top:4px; }
    .empty-state code { margin-top:8px; color:var(--vsc-accent-blue); }

    .basket-footer { border-top:1px solid var(--vsc-border-subtle, var(--vsc-border-subtle)); font-size:10px; }
    .footer-spacer { flex:1; }
    .execute-button { min-width:82px; border-color:var(--vsc-border, var(--vsc-border))!important; }

    @media (max-width:760px),(max-height:560px) {
      .basket-panel { width:calc(100vw - 16px); height:calc(100vh - 16px); }
      .basket-workspace { grid-template-columns:1fr; }
      .basket-list { border-right:0; border-bottom:1px solid var(--vsc-border-subtle, var(--vsc-border-subtle)); }
      .batch-panel { max-height:250px; }
      .basket-summary { grid-template-columns:1fr; }
      .basket-summary>div+div { border-left:0; border-top:1px solid var(--vsc-border-subtle, var(--vsc-border-subtle)); }
    }
  `],
})
export class SelectionBasketComponent implements OnInit, OnDestroy {
  @Input() leftDestination = '';
  @Input() rightDestination = '';
  @Input() leftDestinationWritable = false;
  @Input() rightDestinationWritable = false;

  /** Legacy outputs retained for extension compatibility. */
  @Output() copyTo = new EventEmitter<SelectionBasketDestination>();
  @Output() moveTo = new EventEmitter<SelectionBasketDestination>();
  @Output() deleteRequested = new EventEmitter<void>();
  @Output() executeRequested = new EventEmitter<SelectionBasketOperationRequest>();

  visible = false;
  isClosing = false;
  entries: SelectionBasketEntry[] = [];
  operation: SelectionBasketOperation = 'copy';
  destinationSide: SelectionBasketDestination = 'right';
  recursive = true;
  useTrash = true;

  private readonly destroy$ = createSubject<void>();

  constructor(
    public readonly basket: SelectionBasketService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    this.basket.entries$.pipe(takeUntil(this.destroy$)).subscribe(entries => {
      this.entries = entries;
      this.cdr.markForCheck();
    });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  show(): void {
    if (this.visible) return;
    this.isClosing = false;
    this.visible = true;
    this.normalizeDestination();
    this.cdr.markForCheck();
  }

  hide(): void {
    if (!this.visible || this.isClosing) return;
    this.isClosing = true;
    this.cdr.markForCheck();
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) return;
    this.visible = false;
    this.isClosing = false;
    this.cdr.markForCheck();
  }

  toggle(): void { this.visible ? this.hide() : this.show(); }
  remove(id: string): void { this.basket.remove(id); }
  clear(): void { this.basket.clear(); }
  trackEntry(_index: number, entry: SelectionBasketEntry): string { return entry.id; }

  selectOperation(operation: SelectionBasketOperation): void {
    this.operation = operation;
    if (operation !== 'delete') this.normalizeDestination();
  }

  setDestination(value: string): void {
    if (value === 'left' || value === 'right') this.destinationSide = value;
  }

  execute(): void {
    if (!this.canExecute) return;
    this.executeRequested.emit({
      operation: this.operation,
      destinationSide: this.destinationSide,
      recursive: this.recursive,
      useTrash: this.useTrash,
    });
  }

  get destinationWritable(): boolean {
    return this.destinationSide === 'left' ? this.leftDestinationWritable : this.rightDestinationWritable;
  }

  get canExecute(): boolean {
    return this.entries.length > 0 && (this.operation === 'delete' || this.destinationWritable);
  }

  get operationLabel(): string {
    switch (this.operation) {
      case 'copy': return 'Copy basket contents';
      case 'move': return 'Move basket contents';
      case 'delete': return this.useTrash ? 'Move basket contents to Recycle Bin' : 'Delete basket contents permanently';
    }
  }

  get operationSummary(): string {
    const count = `${this.entries.length} item${this.entries.length === 1 ? '' : 's'}`;
    if (this.operation === 'delete') return count;
    const path = this.destinationSide === 'left' ? this.leftDestination : this.rightDestination;
    return `${count} → ${path || `${this.destinationSide} pane`}`;
  }

  get executeLabel(): string {
    switch (this.operation) {
      case 'copy': return 'Copy';
      case 'move': return 'Move';
      case 'delete': return this.useTrash ? 'Recycle' : 'Delete';
    }
  }

  get sourceCount(): number {
    return new Set(this.entries.map(entry => entry.source).filter(Boolean)).size;
  }

  get knownSizeLabel(): string {
    const total = this.entries.reduce((sum, entry) => sum + (entry.size ?? 0), 0);
    return total > 0 ? this.formatBytes(total) : '—';
  }

  iconFor(entry: SelectionBasketEntry): string {
    if (entry.type === FileType.DIRECTORY) return '▰';
    if (entry.type === FileType.ARCHIVE) return '▣';
    if (entry.type === FileType.SYMLINK) return '↗';
    return '·';
  }

  formatBytes(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
    const units = ['B','KB','MB','GB','TB'];
    const index = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
    const value = bytes / Math.pow(1024, index);
    return `${value >= 10 || index === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[index]}`;
  }

  ageLabel(addedAt: number): string {
    const delta = Math.max(0, Date.now() - addedAt);
    if (delta < 60_000) return 'just added';
    if (delta < 3_600_000) return `${Math.floor(delta / 60_000)}m ago`;
    return new Date(addedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  @HostListener('document:keydown', ['$event'])
  onDocumentKeydown(event: KeyboardEvent): void {
    if (!this.visible || event.key !== 'Escape') return;
    event.preventDefault();
    event.stopPropagation();
    this.hide();
  }

  private normalizeDestination(): void {
    if (this.destinationWritable) return;
    if (this.rightDestinationWritable) this.destinationSide = 'right';
    else if (this.leftDestinationWritable) this.destinationSide = 'left';
  }
}
