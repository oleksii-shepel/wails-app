import {
  Component, Input, Output, EventEmitter, OnChanges, SimpleChanges,
  ViewChild, ElementRef, ChangeDetectionStrategy, ChangeDetectorRef,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FilterPreset, FilterGroup, GROUP_ORDER } from './filter-bar.types';
import { FilterMenuItemComponent } from './filter-menu-item.component';

@Component({
  selector: 'app-filter-menu',
  standalone: true,
  imports: [CommonModule, FilterMenuItemComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="fm">

      <!-- Header -->
      <div class="fm-header">
        <span class="fm-title">Filter</span>
        <div class="fm-stats" *ngIf="totalCount > 0">
          <span [class.fm-stats-active]="filteredCount < totalCount">{{ filteredCount | number }}</span>
          <span class="fm-stats-sep">/</span>
          <span>{{ totalCount | number }}</span>
        </div>
        <div class="fm-mode-group">
          <button class="fm-mode-btn" [class.fm-mode-btn--active]="globalMode === 'AND'"
            (click)="globalModeChange.emit('AND')">ALL</button>
          <button class="fm-mode-btn" [class.fm-mode-btn--active]="globalMode === 'OR'"
            (click)="globalModeChange.emit('OR')">ANY</button>
        </div>
      </div>

      <!-- Dynamic hint -->
      <div class="fm-dynamic-hint" *ngIf="hasDynamic">
        <span class="fm-dynamic-dot"></span>Content-aware filters
      </div>

      <!-- Body -->
      <div class="fm-body" #bodyEl>
        <ng-container *ngFor="let group of groups; let gi = index">
          <ng-container *ngIf="group.presets.length > 0">
            <div class="fm-group-head">
              <span class="fm-group-name">{{ group.name }}</span>
              <button class="fm-group-mode" [class.fm-group-mode--or]="getGroupMode(group.name) === 'OR'"
                (click)="toggleGroupMode(group.name)" [title]="getGroupModeTooltip(group.name)">
                {{ getGroupMode(group.name) }}
              </button>
            </div>
            <app-filter-menu-item
              *ngFor="let preset of group.presets; let pi = index"
              [preset]="preset"
              [checked]="isSelected(preset)"
              [focused]="focusedIdx === getItemIdx(gi, pi)"
              [selectionOrder]="getSelectionOrder(preset)"
              [showOrder]="activePresets.length > 1"
              [defaultIcon]="getDefaultIcon(preset)"
              (toggle)="itemToggle.emit($event)"
              (hovered)="focusedIdx = getItemIdx(gi, pi)"
            />
          </ng-container>
        </ng-container>
        <div class="fm-empty" *ngIf="totalItems === 0">No filters available</div>
      </div>

      <!-- Footer -->
      <div class="fm-footer">
        <button class="fm-footer-btn fm-footer-btn--clear" *ngIf="activePresets.length > 0"
          (click)="clearAll.emit()">
          <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor">
            <path d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"/>
          </svg>Clear all
        </button>
        <button class="fm-footer-btn fm-footer-btn--refresh" *ngIf="hasDynamic"
          (click)="refresh.emit()" title="Refresh">
          <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor">
            <path d="M8 3a5 5 0 1 0 4.546 2.914.5.5 0 0 1 .908-.417A6 6 0 1 1 8 2v1z"/>
            <path d="M8 4.466V.534a.25.25 0 0 1 .41-.192l2.36 1.966c.12.1.12.284 0 .384L8.41 4.658A.25.25 0 0 1 8 4.466z"/>
          </svg>Refresh
        </button>
        <span class="fm-hints">
          <kbd>↑↓</kbd> navigate &nbsp;<kbd>Space</kbd> toggle &nbsp;<kbd>Esc</kbd> close
        </span>
      </div>
    </div>
  `,
  styles: [`
    :host { display: block; position: fixed; z-index: var(--vsc-z-dropdown); }

    .fm {
      display: flex; flex-direction: column;
      width: 260px; max-height: 460px;
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      font-family: var(--vsc-font-family);
      font-size: var(--vsc-font-size);
      color: var(--vsc-foreground);
      overflow: hidden;
    }

    .fm-header {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      border-bottom: 1px solid var(--vsc-border-subtle);
      flex-shrink: 0; height: 32px;
    }
    .fm-title {
      font-size: 11px; font-weight: 600; letter-spacing: 0.06em;
      text-transform: uppercase; color: var(--vsc-foreground-dim); flex-shrink: 0;
    }
    .fm-stats {
      display: flex; align-items: center; gap: 2px;
      font-size: 11px; color: var(--vsc-foreground-dim); font-variant-numeric: tabular-nums;
    }
    .fm-stats-active { color: var(--vsc-accent-blue); }
    .fm-stats-sep { opacity: 0.4; }
    .fm-mode-group {
      margin-left: auto; display: flex; align-items: center;
      border: 1px solid var(--vsc-border); border-radius: var(--vsc-border-radius-sm); overflow: hidden;
    }
    .fm-mode-btn {
      font-size: 10px; font-weight: 600; padding: 2px 7px;
      border: none; background: transparent; color: var(--vsc-foreground-dim);
      cursor: pointer; font-family: var(--vsc-font-family);
      transition: background var(--vsc-transition-fast), color var(--vsc-transition-fast);
    }
    .fm-mode-btn:hover { background: var(--vsc-list-hover-background); color: var(--vsc-foreground); }
    .fm-mode-btn--active { background: var(--vsc-accent-blue); color: #fff; }
    .fm-mode-btn + .fm-mode-btn { border-left: 1px solid var(--vsc-border); }

    .fm-dynamic-hint {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      padding: 3px var(--vsc-padding-md); font-size: 11px;
      color: var(--vsc-foreground-dim); border-bottom: 1px solid var(--vsc-border-subtle);
      background: color-mix(in srgb, var(--vsc-accent-blue) 6%, transparent); flex-shrink: 0;
    }
    .fm-dynamic-dot {
      width: 5px; height: 5px; border-radius: 50%;
      background: var(--vsc-accent-blue); flex-shrink: 0;
      animation: pulse 2s ease-in-out infinite;
    }
    @keyframes pulse { 0%,100% { opacity:1; } 50% { opacity:0.25; } }

    .fm-body {
      flex: 1; overflow-y: auto; padding: var(--vsc-padding-xs) 0;
      scrollbar-width: thin;
      scrollbar-color: var(--vsc-scrollbar-thumb) var(--vsc-scrollbar-track);
    }
    .fm-body::-webkit-scrollbar { width: 6px; }
    .fm-body::-webkit-scrollbar-thumb { background: var(--vsc-scrollbar-thumb); border-radius: 3px; }
    .fm-body::-webkit-scrollbar-thumb:hover { background: var(--vsc-scrollbar-thumb-hover); }

    .fm-group-head {
      display: flex; align-items: center; justify-content: space-between;
      padding: 6px var(--vsc-padding-md) 2px; margin-top: var(--vsc-padding-xs);
    }
    .fm-group-name {
      font-size: 10px; font-weight: 600; letter-spacing: 0.08em;
      text-transform: uppercase; color: var(--vsc-foreground-dim); opacity: 0.7;
    }
    .fm-group-mode {
      font-size: 9px; font-weight: 700; padding: 1px 5px;
      border-radius: var(--vsc-border-radius-sm); border: 1px solid var(--vsc-border-subtle);
      background: transparent; color: var(--vsc-foreground-dim);
      cursor: pointer; font-family: var(--vsc-font-family);
      transition: color var(--vsc-transition-fast), border-color var(--vsc-transition-fast);
    }
    .fm-group-mode--or { color: var(--vsc-accent-blue); border-color: var(--vsc-accent-blue); }
    .fm-group-mode:hover { color: var(--vsc-accent-blue); border-color: var(--vsc-accent-blue); }

    .fm-empty {
      padding: var(--vsc-padding-xl); text-align: center;
      color: var(--vsc-foreground-dim); font-size: var(--vsc-font-size-small);
    }

    .fm-footer {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-xs) var(--vsc-padding-sm);
      border-top: 1px solid var(--vsc-border-subtle); flex-shrink: 0; min-height: 28px;
    }
    .fm-footer-btn {
      display: inline-flex; align-items: center; gap: var(--vsc-padding-xs);
      font-size: 11px; padding: 2px var(--vsc-padding-sm);
      border-radius: var(--vsc-border-radius-sm); border: 1px solid transparent;
      background: transparent; cursor: pointer; font-family: var(--vsc-font-family);
      transition: background var(--vsc-transition-fast); white-space: nowrap;
    }
    .fm-footer-btn--clear {
      color: var(--vsc-accent-red);
      border-color: color-mix(in srgb, var(--vsc-accent-red) 30%, transparent);
    }
    .fm-footer-btn--clear:hover { background: color-mix(in srgb, var(--vsc-accent-red) 12%, transparent); }
    .fm-footer-btn--refresh { color: var(--vsc-foreground-dim); border-color: var(--vsc-border-subtle); }
    .fm-footer-btn--refresh:hover { background: var(--vsc-list-hover-background); }
    .fm-hints { margin-left: auto; font-size: 10px; color: var(--vsc-foreground-dim); opacity: 0.6; }
    .fm-hints kbd {
      display: inline-block; padding: 0 3px;
      border: 1px solid var(--vsc-border-subtle); border-radius: var(--vsc-border-radius-sm);
      font-size: 9px; font-family: var(--vsc-font-family-mono);
      color: var(--vsc-foreground-dim); background: var(--vsc-input-background);
    }
  `],
})
export class FilterMenuComponent implements OnChanges {

  @Input() allPresets:    FilterPreset[] = [];
  @Input() activePresets: FilterPreset[] = [];
  @Input() globalMode:   'AND' | 'OR'   = 'AND';
  @Input() hasDynamic  = false;
  @Input() totalCount  = 0;
  @Input() filteredCount = 0;

  @Output() itemToggle       = new EventEmitter<FilterPreset>();
  @Output() globalModeChange = new EventEmitter<'AND' | 'OR'>();
  @Output() groupModeChange  = new EventEmitter<{ group: string; mode: 'AND' | 'OR' }>();
  @Output() clearAll         = new EventEmitter<void>();
  @Output() refresh          = new EventEmitter<void>();

  @ViewChild('bodyEl') bodyEl?: ElementRef<HTMLElement>;

  groups: FilterGroup[] = [];
  focusedIdx = 0;
  private groupModes = new Map<string, 'AND' | 'OR'>();

  constructor(private cdr: ChangeDetectorRef) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['allPresets']) this.buildGroups();
  }

  get totalItems(): number { return this.groups.reduce((s, g) => s + g.presets.length, 0); }

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
    this.groups = sorted.map(([name, presets]) => ({ name, presets }));
  }

  getGroupMode(name: string): 'AND' | 'OR' { return this.groupModes.get(name) ?? 'OR'; }
  getGroupModeTooltip(name: string): string {
    const m = this.getGroupMode(name);
    return `${name}: combined with ${m} — click to switch`;
  }
  toggleGroupMode(name: string): void {
    const next: 'AND' | 'OR' = this.getGroupMode(name) === 'AND' ? 'OR' : 'AND';
    this.groupModes.set(name, next);
    this.groupModeChange.emit({ group: name, mode: next });
  }

  isSelected(p: FilterPreset): boolean { return this.activePresets.some(a => a.id === p.id); }
  getSelectionOrder(p: FilterPreset): number { return this.activePresets.findIndex(a => a.id === p.id) + 1; }
  getItemIdx(gi: number, pi: number): number {
    let idx = 0;
    for (let i = 0; i < gi; i++) idx += this.groups[i].presets.length;
    return idx + pi;
  }

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

  private scrollToFocused(): void {
    if (!this.bodyEl) return;
    const items = this.bodyEl.nativeElement.querySelectorAll('app-filter-menu-item');
    items[this.focusedIdx]?.scrollIntoView({ block: 'nearest' });
  }

  getDefaultIcon(p: FilterPreset): string {
    if (!p.filter)                      return '📄';
    if (p.filter.includes('type:dir'))  return '📁';
    if (p.filter.includes('type:file')) return '📄';
    if (p.filter.includes('size>'))     return '📦';
    if (p.filter.includes('modified:')) return '⚡';
    if (p.filter.includes('hidden:'))   return '👻';
    return '📄';
  }
}
