import { Component, Input, Output, EventEmitter, ChangeDetectionStrategy, ChangeDetectorRef, ViewChild, ElementRef, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SiblingEntry } from './address-bar.types';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';

@Component({
  selector: 'app-address-bar-sibling-dropdown',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="ab-dropdown ab-sib-drop"
    >
      <div class="ab-drop-head">
        <span>{{ headLabel }}</span>
        <span class="ab-badge" *ngIf="!loading">{{ totalCount > 50 ? '50+' : totalCount }}</span>
      </div>
      <div class="ab-drop-loading" *ngIf="loading && totalCount === 0">
        <span class="ab-spinner"></span>Loading…
      </div>
      <div class="ab-drop-empty" *ngIf="!loading && totalCount === 0">
        <span class="ab-drop-empty-icon">📭</span>
        <span>No sub-folders</span>
      </div>
      <div
        #scrollEl
        class="ab-drop-scroll"
        (scroll)="onScroll()"
      >
        <!-- spacer sets total scrollbar height -->
        <div class="ab-vs-spacer" [style.height.px]="totalHeight"></div>
        <!-- only the visible slice is rendered, translated into place -->
        <div class="ab-vs-window" [style.transform]="'translateY(' + vsTranslate + 'px)'">
          <div
            *ngFor="let s of visibleEntries; let i = index; trackBy: trackBy"
            class="ab-drop-item ab-sib-item"
            [class.ab-drop-item--active]="s.isCurrent"
            [class.ab-drop-item--selected]="vsStart + i === selectedIndex"
            (click)="entryClick.emit(s)"
            (mouseenter)="setSelectedIndex(vsStart + i)"
          >
            <span class="ab-sib-icon">{{ s.isArchive ? '🗜️' : '📁' }}</span>
            <span class="ab-sib-name">{{ s.name }}</span>
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host { display: contents; }

    .ab-sib-drop {
      position: static;
      min-width: 210px;
      max-width: 320px;
    }

    .ab-sib-icon { font-size: 14px; flex-shrink: 0; }

    .ab-sib-name {
      flex: 1;
      font-size: var(--vsc-font-size);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    /* ── dropdown base (duplicated here so component is self-contained) ── */
    .ab-dropdown {
      background-color: var(--vsc-panel-background, #252526) !important;
      background-image: none !important;
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      z-index: 9999;
      overflow: hidden;
    }

    .ab-drop-head {
      display: flex; align-items: center; justify-content: space-between;
      height: 32px; padding: 0 var(--vsc-padding-lg);
      background: var(--vsc-panel-background, #252526) !important;
      border-bottom: 1px solid var(--vsc-border-subtle);
      font-size: var(--vsc-font-size-small); color: var(--vsc-foreground-dim);
      text-transform: uppercase; letter-spacing: 0.5px; font-weight: 600;
    }

    .ab-badge {
      display: inline-flex; align-items: center; justify-content: center;
      min-width: 20px; height: 20px; padding: 0 var(--vsc-padding-xs);
      background: var(--vsc-badge-background); color: var(--vsc-badge-foreground);
      border-radius: 10px; font-size: 11px; font-weight: 600;
    }

    .ab-drop-scroll {
      max-height: 300px; overflow-y: auto; padding: var(--vsc-padding-xs) 0;
      position: relative;
      background-color: var(--vsc-panel-background, #252526) !important;
      scrollbar-width: thin;
      scrollbar-color: var(--vsc-scrollbar-thumb, #424242) transparent;
      &::-webkit-scrollbar { width: 6px; }
      &::-webkit-scrollbar-track { background: transparent; }
      &::-webkit-scrollbar-thumb { background: var(--vsc-scrollbar-thumb, #424242); border-radius: 3px; }
    }

    .ab-vs-spacer {
      position: absolute;
      top: 0; left: 0; width: 1px;
      pointer-events: none;
    }

    .ab-vs-window {
      position: relative;
      will-change: transform;
    }

    .ab-drop-item {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      height: 28px; padding: 0 var(--vsc-padding-lg); cursor: pointer;
      transition: background-color var(--vsc-transition-fast);
      border-left: 2px solid transparent;
      color: var(--vsc-foreground); font-size: var(--vsc-font-size);
      background: transparent;
      &:hover { background: var(--vsc-list-hover-background); }
      &.ab-drop-item--active {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
        border-left-color: var(--vsc-accent-blue);
      }

      &.ab-drop-item--selected {
        background: color-mix(in srgb, var(--vsc-list-hover-background) 80%, transparent);
      }
    }

    .ab-drop-loading {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-lg); font-size: var(--vsc-font-size);
      color: var(--vsc-foreground-dim);
      background-color: var(--vsc-panel-background, #252526) !important;
    }

    .ab-spinner {
      display: inline-block; width: 14px; height: 14px;
      border: 2px solid var(--vsc-border); border-top-color: var(--vsc-accent-blue);
      border-radius: 50%; animation: ab-spin 0.7s linear infinite;
    }

    @keyframes ab-spin { to { transform: rotate(360deg); } }

    .ab-drop-empty {
      display: flex; flex-direction: column; align-items: center;
      gap: var(--vsc-padding-sm); padding: var(--vsc-padding-xl);
      font-size: var(--vsc-font-size-small); color: var(--vsc-foreground-dim);
      text-align: center; line-height: 1.5;
      background-color: var(--vsc-panel-background, #252526) !important;
    }

    .ab-drop-empty-icon { font-size: 24px; opacity: 0.5; margin-bottom: var(--vsc-padding-xs); }
  `],
})
export class AddressBarSiblingDropdownComponent implements OnInit, OnDestroy {
  @ViewChild('scrollEl') scrollElRef?: ElementRef<HTMLElement>;

  private _entries: SiblingEntry[] = [];
  totalCount = 0;

  private readonly ROW_H = 28;
  private readonly MAX_H = 300;
  private readonly OVERSCAN = 4;
  vsStart = 0;
  private vsEnd = 20;
  private scrollRAF: number | null = null;
  private scrollListener?: () => void;
  selectedIndex = -1;
  private readonly commandOwner = `overlay.addressBar.siblings:${Math.random().toString(36).slice(2, 8)}`;
  private releaseOverlayContext: (() => void) | null = null;

  get totalHeight(): number { return this._entries.length * this.ROW_H; }
  get vsTranslate(): number { return this.vsStart * this.ROW_H; }
  get visibleEntries(): SiblingEntry[] {
    return this._entries.slice(this.vsStart, this.vsEnd);
  }
  trackBy(_i: number, s: SiblingEntry): string { return s.path; }

  @Input()
  set entries(val: SiblingEntry[]) {
    this._entries = val || [];
    this.totalCount = this._entries.length;
    this.resetVS();
    this.selectedIndex = this._entries.findIndex(entry => entry.isCurrent);
    if (this.selectedIndex < 0 && this._entries.length > 0) {
      this.selectedIndex = 0;
    }
    // Re-attach scroll listener after Angular renders the scroll div
    setTimeout(() => {
      this.attachScrollListener();
      this.scrollSelectedIntoView();
      this.cdr.markForCheck();
    }, 0);
  }
  get entries(): SiblingEntry[] { return this._entries; }

  @Input() loading = false;
  @Input() headLabel = '';
  @Output() entryClick = new EventEmitter<SiblingEntry>();
  @Output() closeRequested = new EventEmitter<void>();

  constructor(
    private cdr: ChangeDetectorRef,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
  ) {}

  ngOnInit(): void {
    this.registerInputCommands();
    this.releaseOverlayContext ??= this.inputContexts.acquire('overlay.addressBar.siblings');
  }

  ngOnDestroy(): void {
    this.detachScrollListener();
    if (this.scrollRAF) cancelAnimationFrame(this.scrollRAF);
    this.releaseOverlayContext?.();
    this.releaseOverlayContext = null;
    this.unregisterInputCommands();
  }

  onScroll(): void {
    if (this.scrollRAF) return;
    this.scrollRAF = requestAnimationFrame(() => {
      this.scrollRAF = null;
      this.updateVS();
      this.cdr.detectChanges();
    });
  }

  private resetVS(): void {
    const visCount = Math.ceil(this.MAX_H / this.ROW_H);
    this.vsStart = 0;
    this.vsEnd = Math.min(this._entries.length, visCount + this.OVERSCAN * 2);
    const el = this.scrollElRef?.nativeElement;
    if (el) el.scrollTop = 0;
  }

  private updateVS(): void {
    const el = this.scrollElRef?.nativeElement;
    const scrollTop = el?.scrollTop ?? 0;
    const visCount = Math.ceil(this.MAX_H / this.ROW_H);
    const start = Math.max(0, Math.floor(scrollTop / this.ROW_H) - this.OVERSCAN);
    const end   = Math.min(this._entries.length, start + visCount + this.OVERSCAN * 2);
    this.vsStart = start;
    this.vsEnd   = end;
  }

  private attachScrollListener(): void {
    const el = this.scrollElRef?.nativeElement;
    if (!el || this.scrollListener) return;
    this.scrollListener = () => this.onScroll();
    el.addEventListener('scroll', this.scrollListener, { passive: true });
  }

  private detachScrollListener(): void {
    const el = this.scrollElRef?.nativeElement;
    if (el && this.scrollListener) el.removeEventListener('scroll', this.scrollListener);
    this.scrollListener = undefined;
  }

  setSelectedIndex(index: number): void {
    if (index < 0 || index >= this._entries.length) return;
    this.selectedIndex = index;
  }

  private moveSelection(delta: number): void {
    if (this._entries.length === 0) return;

    const baseIndex = this.selectedIndex >= 0 ? this.selectedIndex : 0;
    this.selectedIndex = Math.max(0, Math.min(this._entries.length - 1, baseIndex + delta));
    this.scrollSelectedIntoView();
    this.cdr.markForCheck();
  }

  private openSelected(): void {
    const selected = this._entries[this.selectedIndex];
    if (!selected) return;
    this.entryClick.emit(selected);
  }

  private scrollSelectedIntoView(): void {
    const el = this.scrollElRef?.nativeElement;
    if (!el || this.selectedIndex < 0) return;

    const itemTop = this.selectedIndex * this.ROW_H;
    const itemBottom = itemTop + this.ROW_H;
    if (itemTop < el.scrollTop) {
      el.scrollTop = itemTop;
    } else if (itemBottom > el.scrollTop + this.MAX_H) {
      el.scrollTop = itemBottom - this.MAX_H;
    }

    this.updateVS();
  }

  private registerInputCommands(): void {
    this.inputEngine.registerCommand('overlay.addressBar.siblings.close', () => {
      this.closeRequested.emit();
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.addressBar.siblings.prevItem', () => {
      this.moveSelection(-1);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.addressBar.siblings.nextItem', () => {
      this.moveSelection(1);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.addressBar.siblings.openSelected', () => {
      this.openSelected();
    }, this.commandOwner);
  }

  private unregisterInputCommands(): void {
    this.inputEngine.unregisterCommand('overlay.addressBar.siblings.close', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.addressBar.siblings.prevItem', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.addressBar.siblings.nextItem', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.addressBar.siblings.openSelected', this.commandOwner);
  }
}
