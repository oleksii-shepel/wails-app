import { createSubject, takeUntil } from '@epikodelabs/streamix';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, EventEmitter, HostListener, OnDestroy, OnInit, Output } from '@angular/core';
import { FileType } from '../shared/protocol';
import { SelectionBasketEntry, SelectionBasketService } from '../services/selection-basket.service';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-selection-basket',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div *ngIf="visible || isClosing" class="basket-backdrop palette-dialog-backdrop dialog-group" [class.dialog-group--closing]="isClosing" (mousedown)="hide()" (animationend)="onAnimationEnd($event)">
      <aside class="basket-panel palette-dialog-surface dialog-surface" role="dialog" aria-modal="true" aria-label="Selection Basket" (mousedown)="$event.stopPropagation()">
        <header class="basket-header palette-dialog-header">
          <div class="basket-title palette-dialog-title">
            <strong>Selection Basket</strong>
            <span class="palette-dialog-subtitle">{{ entries.length }} items</span>
          </div>
          <button class="dialog-close-btn" type="button" title="Close" aria-label="Close Selection Basket" (click)="hide()">×</button>
        </header>

        <div class="basket-summary">
          <div><strong>{{ entries.length }}</strong><span>Items</span></div>
          <div><strong>{{ knownSizeLabel }}</strong><span>Known size</span></div>
          <div><strong>{{ sourceCount }}</strong><span>Sources</span></div>
        </div>

        <div class="basket-actions">
          <button type="button" class="palette-dialog-button" [disabled]="!entries.length" (click)="copyTo.emit('left')">Copy → Left</button>
          <button type="button" class="palette-dialog-button" [disabled]="!entries.length" (click)="copyTo.emit('right')">Copy → Right</button>
          <button type="button" class="palette-dialog-button" [disabled]="!entries.length" (click)="moveTo.emit('left')">Move → Left</button>
          <button type="button" class="palette-dialog-button" [disabled]="!entries.length" (click)="moveTo.emit('right')">Move → Right</button>
          <button class="palette-dialog-button palette-dialog-button--danger" type="button" [disabled]="!entries.length" (click)="deleteRequested.emit()">Delete</button>
        </div>

        <div class="basket-list palette-dialog-body">
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
        </div>

        <footer class="basket-footer">
          <span>Kept for this app session, across navigation and surfaces.</span>
          <button type="button" class="palette-dialog-button" [disabled]="!entries.length" (click)="clear()">Clear Basket</button>
        </footer>
      </aside>
    </div>
  `,
  styles: [`
    :host {
      position: fixed;
      inset: 0;
      z-index: 9460;
      pointer-events: none;
    }

    .basket-panel {
      box-sizing: border-box;
      display: flex;
      flex-direction: column;
      width: min(720px, calc(100vw - 24px));
      max-height: min(680px, calc(100vh - 40px));
      overflow: hidden;
      border: 1px solid var(--vsc-border, var(--vsc-border));
      border-radius: 8px;
      background: var(--vsc-menu-background, var(--vsc-panel-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
      box-shadow: 0 18px 60px var(--vsc-scrim);
      font: 12px/1.4 var(--vsc-font-family, system-ui, sans-serif);
    }

    .basket-header,
    .basket-footer {
      display: flex;
      align-items: center;
      gap: 9px;
      padding: 9px 10px;
    }

    .basket-header {
      justify-content: space-between;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
    }

    .basket-title {
      display: flex;
      gap: 8px;
      align-items: baseline;
      min-width: 0;
    }

    .basket-title strong {
      font-weight: 600;
    }

    .basket-title span,
    .basket-footer > span,
    .entry-path,
    .entry-meta,
    .basket-summary span {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .remove-button,
    .basket-actions button,
    .basket-footer button {
      min-height: 27px;
      padding: 3px 8px;
      border: 1px solid transparent;
      border-radius: 4px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      font: inherit;
    }

    .remove-button {
      width: 27px;
      padding: 0;
      font-size: 16px;
      opacity: .7;
    }

    .remove-button:hover,
    .basket-actions button:hover:not(:disabled),
    .basket-footer button:hover:not(:disabled) {
      border-color: var(--vsc-border, var(--vsc-border));
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
      opacity: 1;
    }

    button:disabled {
      opacity: .35;
      cursor: default;
    }

    .basket-summary {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
    }

    .basket-summary > div {
      display: flex;
      align-items: baseline;
      gap: 6px;
      min-width: 0;
      padding: 6px 10px;
    }

    .basket-summary > div + div {
      border-left: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
    }

    .basket-summary strong {
      font-size: 12px;
      font-weight: 600;
    }

    .basket-summary span {
      font-size: 10px;
    }

    .basket-actions {
      display: flex;
      flex-wrap: wrap;
      gap: 2px;
      padding: 5px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
    }

    .basket-actions .danger {
      color: var(--vsc-error-foreground, var(--vsc-error));
    }

    .basket-list {
      flex: 1;
      min-height: 0;
      overflow: auto;
      padding: 5px;
    }

    .basket-row {
      display: grid;
      grid-template-columns: 30px minmax(0, 1fr) 27px;
      gap: 8px;
      align-items: center;
      min-height: 52px;
      padding: 5px 8px;
      border: 1px solid transparent;
      border-radius: 4px;
    }

    .basket-row:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    .entry-icon {
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 16px;
    }

    .entry-main {
      min-width: 0;
      display: grid;
    }

    .entry-main strong,
    .entry-path {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .entry-path {
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 10px;
    }

    .entry-meta {
      display: flex;
      gap: 4px 10px;
      margin-top: 2px;
      font-size: 10px;
    }

    .empty-state {
      padding: 38px;
      text-align: center;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .empty-state strong,
    .empty-state span,
    .empty-state code {
      display: block;
    }

    .empty-state strong {
      color: var(--vsc-foreground);
    }

    .empty-state span {
      margin-top: 4px;
    }

    .empty-state code {
      margin-top: 8px;
      color: var(--vsc-accent-blue);
    }

    .basket-footer {
      border-top: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      font-size: 10px;
    }

    .basket-footer button {
      margin-left: auto;
    }

    @media (max-width: 760px), (max-height: 560px) {
      .basket-panel {
        width: calc(100vw - 16px);
        max-height: calc(100vh - 16px);
      }

      .basket-summary {
        grid-template-columns: 1fr;
      }

      .basket-summary > div + div {
        border-left: 0;
        border-top: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      }

      .basket-footer > span {
        display: none;
      }
    }
  `],
})
export class SelectionBasketComponent implements OnInit, OnDestroy {
  visible = false;
  isClosing = false;
  entries: SelectionBasketEntry[] = [];
  private readonly destroy$ = createSubject<void>();
  @Output() copyTo = new EventEmitter<'left' | 'right'>();
  @Output() moveTo = new EventEmitter<'left' | 'right'>();
  @Output() deleteRequested = new EventEmitter<void>();

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

  toggle(): void {
    this.visible ? this.hide() : this.show();
  }
  remove(id: string): void { this.basket.remove(id); }
  clear(): void { this.basket.clear(); }
  trackEntry(_index: number, entry: SelectionBasketEntry): string { return entry.id; }

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
}