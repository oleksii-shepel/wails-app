import { CommonModule } from '@angular/common';
import { Component, EventEmitter, OnDestroy, OnInit, Output } from '@angular/core';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import { FileType } from '../shared/protocol';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';
import { SelectionBasketEntry, SelectionBasketService } from '../services/selection-basket.service';

@Component({
  selector: 'app-selection-basket',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div *ngIf="visible" class="basket-backdrop" (mousedown)="hide()">
      <aside class="basket-panel" role="dialog" aria-modal="true" aria-label="Selection Basket" (mousedown)="$event.stopPropagation()">
        <header class="basket-header">
          <div>
            <div class="eyebrow">Working set</div>
            <h2>Selection Basket</h2>
          </div>
          <button class="icon-button" type="button" title="Close" (click)="hide()">×</button>
        </header>

        <div class="basket-summary">
          <div><strong>{{ entries.length }}</strong><span>Items</span></div>
          <div><strong>{{ knownSizeLabel }}</strong><span>Known size</span></div>
          <div><strong>{{ sourceCount }}</strong><span>Sources</span></div>
        </div>

        <div class="basket-actions">
          <button type="button" [disabled]="!entries.length" (click)="copyTo.emit('left')">Copy → Left</button>
          <button type="button" [disabled]="!entries.length" (click)="copyTo.emit('right')">Copy → Right</button>
          <button type="button" [disabled]="!entries.length" (click)="moveTo.emit('left')">Move → Left</button>
          <button type="button" [disabled]="!entries.length" (click)="moveTo.emit('right')">Move → Right</button>
          <button class="danger" type="button" [disabled]="!entries.length" (click)="deleteRequested.emit()">Delete</button>
        </div>

        <div class="basket-list">
          <div *ngIf="entries.length === 0" class="empty-state">
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
          <button type="button" [disabled]="!entries.length" (click)="clear()">Clear Basket</button>
        </footer>
      </aside>
    </div>
  `,
  styles: [`
    :host { position:relative; z-index:calc(var(--vsc-z-modal) + 3); }
    .basket-backdrop { position:fixed; inset:0; z-index:calc(var(--vsc-z-modal) + 3); background: var(--vsc-scrim-soft); display:flex; justify-content:flex-end; }
    .basket-panel { width:min(600px,94vw); height:100%; display:flex; flex-direction:column; background:var(--vsc-panel-background); color:var(--vsc-foreground); border-left:1px solid var(--vsc-border-bright); box-shadow:var(--vsc-widget-shadow); }
    .basket-header { display:flex; align-items:center; justify-content:space-between; padding:18px 20px 14px; border-bottom:1px solid var(--vsc-border); }
    .basket-header h2 { margin:2px 0 0; font-size:20px; font-weight:600; }
    .eyebrow { color:var(--vsc-foreground-dim); font-size:11px; text-transform:uppercase; letter-spacing:.08em; }
    .icon-button,.remove-button { border:0; background:transparent; color:inherit; cursor:pointer; border-radius:4px; }
    .icon-button { width:30px; height:30px; font-size:24px; }
    .remove-button { width:26px; height:26px; font-size:20px; opacity:.7; }
    .icon-button:hover,.remove-button:hover { background:var(--vsc-list-hover-background); opacity:1; }
    .basket-summary { display:grid; grid-template-columns:repeat(3,1fr); gap:1px; background:var(--vsc-border); border-bottom:1px solid var(--vsc-border); }
    .basket-summary div { display:flex; flex-direction:column; padding:12px 16px; background:var(--vsc-editor-background); }
    .basket-summary strong { font-size:18px; } .basket-summary span { color:var(--vsc-foreground-dim); font-size:11px; }
    .basket-actions { display:flex; flex-wrap:wrap; gap:6px; padding:10px 14px; border-bottom:1px solid var(--vsc-border); }
    .basket-actions button,.basket-footer button { border:1px solid transparent; border-radius:3px; background:var(--vsc-button-secondary); color:var(--vsc-button-secondary-foreground); padding:5px 9px; font:inherit; font-size:12px; cursor:pointer; }
    .basket-actions button:hover:not(:disabled),.basket-footer button:hover:not(:disabled) { background:var(--vsc-button-secondary-hover); }
    .basket-actions button.danger:hover:not(:disabled) { background:color-mix(in srgb,var(--vsc-error) 30%,var(--vsc-button-secondary)); border-color:var(--vsc-error); }
    button:disabled { opacity:.42; cursor:default; }
    .basket-list { flex:1; min-height:0; overflow:auto; }
    .basket-row { display:grid; grid-template-columns:28px minmax(0,1fr) 28px; gap:10px; align-items:start; padding:12px 14px; border-bottom:1px solid var(--vsc-border); }
    .basket-row:hover { background:var(--vsc-list-hover-background); }
    .entry-icon { font-size:16px; padding-top:1px; opacity:.9; }
    .entry-main { min-width:0; }
    .entry-main strong { display:block; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
    .entry-path { margin-top:3px; color:var(--vsc-foreground-dim); font-size:11px; white-space:nowrap; overflow:hidden; text-overflow:ellipsis; }
    .entry-meta { display:flex; gap:5px 12px; flex-wrap:wrap; margin-top:5px; color:var(--vsc-foreground-dim); font-size:10px; }
    .empty-state { height:100%; min-height:250px; display:flex; flex-direction:column; align-items:center; justify-content:center; gap:7px; color:var(--vsc-foreground-dim); text-align:center; padding:24px; }
    .empty-state strong { color:var(--vsc-foreground); } .empty-state code { margin-top:5px; color:var(--vsc-accent-blue); }
    .basket-footer { display:flex; justify-content:space-between; align-items:center; gap:12px; padding:10px 14px; border-top:1px solid var(--vsc-border); color:var(--vsc-foreground-dim); font-size:10px; }
  `],
})
export class SelectionBasketComponent implements OnInit, OnDestroy {
  visible = false;
  entries: SelectionBasketEntry[] = [];
  private readonly destroy$ = createSubject<void>();
  private readonly freezeKey = 'selection-basket';

  @Output() copyTo = new EventEmitter<'left' | 'right'>();
  @Output() moveTo = new EventEmitter<'left' | 'right'>();
  @Output() deleteRequested = new EventEmitter<void>();

  constructor(
    public readonly basket: SelectionBasketService,
    private readonly overlayFreeze: NativeOverlayFreezeService,
  ) {}

  ngOnInit(): void {
    this.basket.entries$.pipe(takeUntil(this.destroy$)).subscribe(entries => this.entries = entries);
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
    this.overlayFreeze.release(this.freezeKey);
  }

  show(): void { this.visible = true; this.overlayFreeze.acquire(this.freezeKey); }
  hide(): void { this.visible = false; this.overlayFreeze.release(this.freezeKey); }
  toggle(): void { this.visible ? this.hide() : this.show(); }
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
}