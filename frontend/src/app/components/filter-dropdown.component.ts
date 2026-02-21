// ─── filter-dropdown.component.ts ───────────────────────────────────────────

import {
  Component, Input, Output, EventEmitter, OnChanges, SimpleChanges,
  ViewChild, ElementRef, ChangeDetectionStrategy, ChangeDetectorRef,
  AfterViewChecked
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FilterPreset, FilterGroup, GROUP_ORDER } from './filter-bar.types';
import { FilterMenuItemComponent } from './filter-menu-item.component';

@Component({
  selector: 'app-filter-dropdown',
  standalone: true,
  imports: [CommonModule, FilterMenuItemComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="fd-panel"
      [style.width.px]="width"
      (mousedown)="$event.preventDefault()"
      (click)="$event.stopPropagation()"
      role="menu"
    >
      <!-- ── Header ────────────────────────────────────────────── -->
      <div class="fd-header">
        <span class="fd-header-label">Filters</span>
        <div class="fd-global-mode">
          <span class="fd-mode-hint">Combine with</span>
          <button
            class="fd-mode-toggle"
            [class.fd-mode-toggle--or]="globalMode === 'OR'"
            (click)="globalModeChange.emit(globalMode === 'AND' ? 'OR' : 'AND')"
            [title]="'Click to switch to ' + (globalMode === 'AND' ? 'OR' : 'AND')"
          >{{ globalMode }}</button>
        </div>
      </div>

      <!-- Dynamic hint -->
      <div class="fd-dynamic-hint" *ngIf="hasDynamic">
        <span class="fd-dynamic-dot"></span>
        <span>Auto-generated from content</span>
      </div>

      <!-- ── Scrollable group list ──────────────────────────────── -->
      <div class="fd-body" #bodyEl>
        <ng-container *ngFor="let group of groups; let gi = index">
          <ng-container *ngIf="group.presets.length > 0">

            <!-- Group header -->
            <div class="fd-group-head">
              <span class="fd-group-name">{{ group.name }}</span>
              <button
                class="fd-group-mode"
                [class.fd-group-mode--or]="getGroupMode(group.name) === 'OR'"
                (click)="toggleGroupMode(group.name, $event)"
                [title]="getGroupModeTooltip(group.name)"
              >{{ getGroupMode(group.name) }}</button>
            </div>

            <!-- Items -->
            <app-filter-menu-item
              *ngFor="let preset of group.presets; let pi = index"
              [preset]="preset"
              [checked]="isSelected(preset)"
              [focused]="focusedIdx === getItemIdx(gi, pi)"
              [selectionOrder]="getSelectionOrder(preset)"
              [showOrder]="activePresets.length > 1"
              [defaultIcon]="getDefaultIcon(preset)"
              (toggle)="itemToggle.emit($event)"
              (focus)="focusedIdx = getItemIdx(gi, pi)"
            />

          </ng-container>
        </ng-container>

        <div class="fd-empty" *ngIf="groups.length === 0 || totalItems === 0">
          No filters available
        </div>
      </div>

      <!-- ── Footer ─────────────────────────────────────────────── -->
      <div class="fd-footer">
        <button class="fd-footer-btn fd-footer-btn--clear"
          *ngIf="activePresets.length > 0"
          (click)="clearAll.emit()">
          <svg width="9" height="9" viewBox="0 0 16 16" fill="currentColor">
            <path d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"/>
          </svg>
          Clear all
        </button>

        <button class="fd-footer-btn fd-footer-btn--refresh"
          *ngIf="hasDynamic"
          (click)="refresh.emit()"
          title="Refresh content analysis">
          <svg width="9" height="9" viewBox="0 0 16 16" fill="currentColor">
            <path d="M8 3a5 5 0 1 0 4.546 2.914.5.5 0 0 1 .908-.417A6 6 0 1 1 8 2v1z"/>
            <path d="M8 4.466V.534a.25.25 0 0 1 .41-.192l2.36 1.966c.12.1.12.284 0 .384L8.41 4.658A.25.25 0 0 1 8 4.466z"/>
          </svg>
          Refresh
        </button>

        <span class="fd-footer-hint">
          <kbd>↑↓</kbd> navigate &nbsp; <kbd>Space</kbd> toggle &nbsp; <kbd>Esc</kbd> close
        </span>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      position: fixed;
      z-index: 9999;
    }

    .fd-panel {
      display: flex;
      flex-direction: column;
      background: var(--menu-bg, #1e212c);
      border: 1px solid var(--menu-border, #2e3242);
      border-radius: 7px;
      box-shadow:
        0 4px 6px rgba(0,0,0,0.25),
        0 12px 32px rgba(0,0,0,0.35),
        0 0 0 0.5px rgba(255,255,255,0.04) inset;
      min-width: 220px;
      max-height: 480px;
      overflow: hidden;
      font-size: 12px;
      font-family: inherit;
    }

    /* ── Header ─────────────────────────────── */
    .fd-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 8px 12px 6px;
      border-bottom: 1px solid var(--menu-divider, #252836);
      flex-shrink: 0;
    }
    .fd-header-label {
      font-size: 10px;
      font-weight: 600;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      color: var(--meta-fg, #5a6070);
    }
    .fd-global-mode {
      display: flex;
      align-items: center;
      gap: 6px;
    }
    .fd-mode-hint {
      font-size: 10px;
      color: var(--meta-fg, #5a6070);
    }
    .fd-mode-toggle {
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.05em;
      padding: 2px 7px;
      border-radius: 4px;
      border: 1px solid var(--accent, #4a7cc7);
      background: transparent;
      color: var(--accent, #4a7cc7);
      cursor: pointer;
      transition: background 80ms, color 80ms;
      font-family: inherit;
    }
    .fd-mode-toggle--or {
      background: var(--accent, #4a7cc7);
      color: #fff;
    }
    .fd-mode-toggle:hover {
      background: var(--accent, #4a7cc7);
      color: #fff;
    }

    /* ── Dynamic hint ───────────────────────── */
    .fd-dynamic-hint {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 5px 12px;
      font-size: 10px;
      color: var(--meta-fg, #5a6070);
      border-bottom: 1px solid var(--menu-divider, #252836);
      background: var(--dynamic-hint-bg, rgba(74,124,199,0.06));
      flex-shrink: 0;
    }
    .fd-dynamic-dot {
      width: 5px;
      height: 5px;
      border-radius: 50%;
      background: var(--accent, #4a7cc7);
      flex-shrink: 0;
      animation: pulse 2s ease-in-out infinite;
    }
    @keyframes pulse {
      0%, 100% { opacity: 1; }
      50%       { opacity: 0.3; }
    }

    /* ── Body ────────────────────────────────── */
    .fd-body {
      flex: 1;
      overflow-y: auto;
      padding: 4px 0;
      scrollbar-width: thin;
      scrollbar-color: var(--scroll-thumb, #2e3242) transparent;
    }
    .fd-body::-webkit-scrollbar { width: 4px; }
    .fd-body::-webkit-scrollbar-thumb {
      border-radius: 2px;
      background: var(--scroll-thumb, #2e3242);
    }

    /* ── Group header ───────────────────────── */
    .fd-group-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 8px 10px 3px 10px;
      margin-top: 2px;
    }
    .fd-group-head:first-child { margin-top: 0; }

    .fd-group-name {
      font-size: 9.5px;
      font-weight: 600;
      letter-spacing: 0.1em;
      text-transform: uppercase;
      color: var(--group-label-fg, #4a5060);
    }

    .fd-group-mode {
      font-size: 9px;
      font-weight: 700;
      letter-spacing: 0.05em;
      padding: 1px 5px;
      border-radius: 3px;
      border: 1px solid var(--group-mode-border, #2e3242);
      background: transparent;
      color: var(--meta-fg, #5a6070);
      cursor: pointer;
      font-family: inherit;
      transition: color 80ms, border-color 80ms, background 80ms;
    }
    .fd-group-mode--or {
      color: var(--accent, #4a7cc7);
      border-color: var(--accent, #4a7cc7);
    }
    .fd-group-mode:hover {
      color: var(--accent, #4a7cc7);
      border-color: var(--accent, #4a7cc7);
    }

    /* ── Empty ───────────────────────────────── */
    .fd-empty {
      padding: 16px;
      text-align: center;
      color: var(--meta-fg, #5a6070);
      font-size: 11px;
    }

    /* ── Footer ──────────────────────────────── */
    .fd-footer {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 5px 10px;
      border-top: 1px solid var(--menu-divider, #252836);
      flex-shrink: 0;
      min-height: 32px;
    }

    .fd-footer-btn {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      font-size: 11px;
      padding: 3px 8px;
      border-radius: 4px;
      border: 1px solid transparent;
      background: transparent;
      cursor: pointer;
      font-family: inherit;
      transition: background 80ms;
      white-space: nowrap;
    }
    .fd-footer-btn--clear {
      color: var(--danger, #c96a6a);
      border-color: var(--danger-border, #3a2222);
    }
    .fd-footer-btn--clear:hover { background: var(--danger-hover-bg, #2a1818); }
    .fd-footer-btn--refresh {
      color: var(--meta-fg, #5a6070);
      border-color: var(--menu-divider, #252836);
    }
    .fd-footer-btn--refresh:hover { background: var(--menu-item-hover, #2a2f3d); }

    .fd-footer-hint {
      margin-left: auto;
      font-size: 10px;
      color: var(--meta-fg, #4a5060);
    }
    .fd-footer-hint kbd {
      display: inline-block;
      padding: 1px 4px;
      border: 1px solid var(--kbd-border, #2e3242);
      border-radius: 3px;
      font-size: 9px;
      font-family: inherit;
      color: var(--meta-fg, #5a6070);
      background: var(--kbd-bg, #171920);
    }
  `]
})
export class FilterDropdownComponent implements OnChanges, AfterViewChecked {

  @Input() allPresets: FilterPreset[]  = [];
  @Input() activePresets: FilterPreset[] = [];
  @Input() globalMode: 'AND' | 'OR'   = 'AND';
  @Input() hasDynamic = false;
  @Input() width = 240;

  @Output() itemToggle      = new EventEmitter<FilterPreset>();
  @Output() globalModeChange = new EventEmitter<'AND' | 'OR'>();
  @Output() groupModeChange  = new EventEmitter<{ group: string; mode: 'AND' | 'OR' }>();
  @Output() clearAll         = new EventEmitter<void>();
  @Output() refresh          = new EventEmitter<void>();

  @ViewChild('bodyEl') bodyEl?: ElementRef<HTMLElement>;

  groups: FilterGroup[] = [];
  focusedIdx = 0;

  // Group modes: default is OR within a group (e.g. ext:jpg OR ext:png)
  private groupModes = new Map<string, 'AND' | 'OR'>();

  constructor(private cdr: ChangeDetectorRef) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['allPresets']) {
      this.buildGroups();
    }
  }

  ngAfterViewChecked(): void {}

  get totalItems(): number {
    return this.groups.reduce((s, g) => s + g.presets.length, 0);
  }

  // ── Group building ────────────────────────────────────────────────────────

  private buildGroups(): void {
    const map = new Map<string, FilterPreset[]>();
    for (const p of this.allPresets) {
      const g = p.group || 'Other';
      if (!map.has(g)) map.set(g, []);
      map.get(g)!.push(p);
    }
    const sorted = Array.from(map.entries()).sort((a, b) => {
      const ia = GROUP_ORDER.indexOf(a[0]), ib = GROUP_ORDER.indexOf(b[0]);
      return (ia === -1 ? 999 : ia) - (ib === -1 ? 999 : ib);
    });
    this.groups = sorted.map(([name, presets]) => ({
      name,
      presets,
      mode: this.getGroupMode(name),
    }));
  }

  // ── Group mode ────────────────────────────────────────────────────────────

  getGroupMode(name: string): 'AND' | 'OR' {
    return this.groupModes.get(name) ?? 'OR';
  }

  getGroupModeTooltip(name: string): string {
    const m = this.getGroupMode(name);
    return `${name} filters combined with ${m} — click to switch to ${m === 'AND' ? 'OR' : 'AND'}`;
  }

  toggleGroupMode(name: string, e: Event): void {
    e.stopPropagation();
    const next: 'AND' | 'OR' = this.getGroupMode(name) === 'AND' ? 'OR' : 'AND';
    this.groupModes.set(name, next);
    this.groupModeChange.emit({ group: name, mode: next });
    this.buildGroups();
  }

  // ── Selection helpers ─────────────────────────────────────────────────────

  isSelected(preset: FilterPreset): boolean {
    return this.activePresets.some(p => p.id === preset.id);
  }

  getSelectionOrder(preset: FilterPreset): number {
    return this.activePresets.findIndex(p => p.id === preset.id) + 1;
  }

  // ── Focus navigation ──────────────────────────────────────────────────────

  moveFocus(delta: number): void {
    this.focusedIdx = Math.max(0, Math.min(this.focusedIdx + delta, this.totalItems - 1));
    this.scrollToFocused();
    this.cdr.markForCheck();
  }

  activateFocused(): void {
    let idx = 0;
    for (const g of this.groups) {
      for (const p of g.presets) {
        if (idx++ === this.focusedIdx) { this.itemToggle.emit(p); return; }
      }
    }
  }

  getItemIdx(gi: number, pi: number): number {
    let idx = 0;
    for (let i = 0; i < gi; i++) idx += this.groups[i].presets.length;
    return idx + pi;
  }

  private scrollToFocused(): void {
    if (!this.bodyEl) return;
    const items = this.bodyEl.nativeElement.querySelectorAll('app-filter-menu-item');
    items[this.focusedIdx]?.scrollIntoView({ block: 'nearest' });
  }

  // ── Icon fallback ─────────────────────────────────────────────────────────

  getDefaultIcon(p: FilterPreset): string {
    if (!p.filter)                   return '📄';
    if (p.filter.includes('type:dir'))  return '📁';
    if (p.filter.includes('type:file')) return '📄';
    if (p.filter.includes('size>'))     return '📦';
    if (p.filter.includes('modified:')) return '⚡';
    if (p.filter.includes('hidden:'))   return '👻';
    return '📄';
  }
}
