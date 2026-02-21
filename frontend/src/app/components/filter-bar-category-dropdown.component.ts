import {
  Component, Input, Output, EventEmitter,
  ChangeDetectionStrategy, ChangeDetectorRef, OnChanges, SimpleChanges,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FilterPreset, FilterGroup, GROUP_ORDER } from './filter-bar.types';

@Component({
  selector: 'app-filter-dropdown',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="fd-panel"
      role="menu"
      (mousedown)="$event.preventDefault()"
      (click)="$event.stopPropagation()"
    >
      <!-- ── Header ────────────────────────────────────────────── -->
      <div class="fd-header">
        <span class="fd-header-label">Filters</span>
        <span class="fd-header-count" *ngIf="selected.length > 0">
          {{ selected.length }} active
        </span>
        <button
          class="fd-header-clear"
          *ngIf="selected.length > 0"
          (click)="clearAll.emit()"
          title="Clear all filters"
        >Clear</button>
      </div>

      <!-- ── Dynamic content hint ──────────────────────────────── -->
      <div class="fd-dynamic-hint" *ngIf="hasDynamic">
        <span class="fd-dynamic-icon">📊</span>
        <span>Counts reflect current folder</span>
      </div>

      <!-- ── No data state ─────────────────────────────────────── -->
      <div class="fd-empty" *ngIf="groups.length === 0">
        <span>No files to analyse</span>
      </div>

      <!-- ── Scrollable body ───────────────────────────────────── -->
      <div class="fd-body" *ngIf="groups.length > 0">
        <ng-container *ngFor="let group of groups">
          <ng-container *ngIf="group.presets.length > 0">

            <!-- Group row -->
            <div class="fd-group-head">
              <span class="fd-group-name">{{ group.name }}</span>
              <span
                class="fd-group-mode"
                [class.fd-group-mode--or]="getGroupMode(group.name) === 'OR'"
                [class.fd-group-mode--and]="getGroupMode(group.name) === 'AND'"
                [title]="'Items in this group are combined with ' + getGroupMode(group.name)"
              >{{ getGroupMode(group.name) }}</span>
            </div>

            <!-- Items -->
            <button
              class="fd-item"
              *ngFor="let preset of group.presets"
              role="menuitemcheckbox"
              [attr.aria-checked]="isSelected(preset)"
              [class.fd-item--active]="isSelected(preset)"
              [class.fd-item--dynamic]="preset.isDynamic"
              (click)="itemToggle.emit(preset)"
            >
              <span class="fd-checkbox" [class.fd-checkbox--on]="isSelected(preset)" aria-hidden="true">
                <svg *ngIf="isSelected(preset)" width="8" height="8" viewBox="0 0 16 16" fill="currentColor">
                  <path d="M13.485 1.929L5.5 9.914 2.515 6.929a1 1 0 0 0-1.414 1.414l3.692 3.693a1 1 0 0 0 1.414 0l8.692-8.693a1 1 0 0 0-1.414-1.414z"/>
                </svg>
              </span>
              <span class="fd-item-icon">{{ preset.icon || '📄' }}</span>
              <span class="fd-item-label">{{ preset.name }}</span>
              <span class="fd-item-count" *ngIf="preset.count != null">{{ preset.count }}</span>
            </button>

          </ng-container>
        </ng-container>
      </div>

      <!-- ── Footer ────────────────────────────────────────────── -->
      <div class="fd-footer">
        <button class="fd-refresh" (click)="refresh.emit()" title="Re-analyse folder">
          <svg width="11" height="11" viewBox="0 0 16 16" fill="currentColor">
            <path d="M8 3a5 5 0 1 0 4.546 2.914.5.5 0 0 1 .908-.417A6 6 0 1 1 8 2v1z"/>
            <path d="M8 4.466V.534a.25.25 0 0 1 .41-.192l2.36 1.966c.12.1.12.284 0 .384L8.41 4.658A.25.25 0 0 1 8 4.466z"/>
          </svg>
          Refresh
        </button>
        <div class="fd-hints">
          <kbd>Space</kbd><span class="fd-hint-label">toggle</span>
          <kbd>Esc</kbd><span class="fd-hint-label">close</span>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      position: fixed;
      z-index: var(--vsc-z-dropdown);
    }

    .fd-panel {
      display: flex;
      flex-direction: column;
      width: 300px;
      max-height: 480px;
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-lg);
      box-shadow: var(--vsc-widget-shadow);
      font-size: var(--vsc-font-size);
      font-family: var(--vsc-font-family);
      overflow: hidden;
      animation: fd-appear var(--vsc-transition-fast) cubic-bezier(0.4, 0, 0.2, 1);
    }

    @keyframes fd-appear {
      from { opacity: 0; transform: translateY(-3px); }
      to   { opacity: 1; transform: translateY(0); }
    }

    /* ── Header ─────────────────────────────────────────── */
    .fd-header {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      height: 32px;
      padding: 0 var(--vsc-padding-md);
      border-bottom: 1px solid var(--vsc-border-subtle);
      flex-shrink: 0;
    }
    .fd-header-label {
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.1em;
      text-transform: uppercase;
      color: var(--vsc-foreground-dim);
    }
    .fd-header-count {
      font-size: 11px;
      padding: 1px 6px;
      border-radius: 8px;
      background: var(--vsc-accent-blue);
      color: #fff;
      font-weight: 600;
    }
    .fd-header-clear {
      margin-left: auto;
      font-size: 11px;
      padding: 2px 6px;
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-sm);
      background: transparent;
      color: var(--vsc-foreground-dim);
      cursor: pointer;
      font-family: var(--vsc-font-family);
      transition: background var(--vsc-transition-fast), color var(--vsc-transition-fast);
    }
    .fd-header-clear:hover {
      background: color-mix(in srgb, var(--vsc-accent-red) 12%, transparent);
      color: var(--vsc-accent-red);
      border-color: color-mix(in srgb, var(--vsc-accent-red) 40%, transparent);
    }

    /* ── Dynamic hint ────────────────────────────────────── */
    .fd-dynamic-hint {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: var(--vsc-padding-sm);
      height: 26px;
      padding: 0 var(--vsc-padding-md);
      background: color-mix(in srgb, var(--vsc-accent-green) 10%, var(--vsc-panel-background));
      border-bottom: 1px solid color-mix(in srgb, var(--vsc-accent-green) 20%, var(--vsc-border-subtle));
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-accent-green);
      flex-shrink: 0;
    }
    .fd-dynamic-icon { font-size: 13px; line-height: 1; }

    /* ── Body ────────────────────────────────────────────── */
    .fd-body {
      flex: 1;
      overflow-y: auto;
      padding: var(--vsc-padding-xs) 0;
      scrollbar-width: thin;
      scrollbar-color: var(--vsc-scrollbar-thumb) var(--vsc-scrollbar-track);
    }
    .fd-body::-webkit-scrollbar { width: 4px; }
    .fd-body::-webkit-scrollbar-track { background: var(--vsc-scrollbar-track); }
    .fd-body::-webkit-scrollbar-thumb {
      background: var(--vsc-scrollbar-thumb);
      border-radius: var(--vsc-border-radius);
    }

    /* ── Group header ────────────────────────────────────── */
    .fd-group-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: 26px;
      padding: 0 var(--vsc-padding-md);
      margin-top: var(--vsc-padding-xs);
    }
    .fd-group-head:first-child { margin-top: 0; }
    .fd-group-name {
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.1em;
      text-transform: uppercase;
      color: var(--vsc-foreground-dim);
    }

    /* AND / OR pill — display only, not interactive */
    .fd-group-mode {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      height: 20px;
      min-width: 36px;
      padding: 0 var(--vsc-padding-sm);
      border-radius: var(--vsc-border-radius-sm);
      font-size: 11px;
      font-weight: 700;
      font-family: var(--vsc-font-family-mono);
      letter-spacing: 0.05em;
      pointer-events: none;
      user-select: none;
    }
    /* OR = green outlined pill (matches screenshot) */
    .fd-group-mode--or {
      background: transparent;
      border: 1.5px solid var(--vsc-accent-green);
      color: var(--vsc-accent-green);
    }
    /* AND = blue filled pill (matches screenshot) */
    .fd-group-mode--and {
      background: var(--vsc-accent-blue);
      border: 1.5px solid var(--vsc-accent-blue);
      color: #fff;
    }

    /* ── Item ────────────────────────────────────────────── */
    .fd-item {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      width: 100%;
      height: 28px;
      padding: 0 var(--vsc-padding-md);
      border: none;
      border-left: 2px solid transparent;
      background: transparent;
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size);
      font-family: var(--vsc-font-family);
      cursor: pointer;
      text-align: left;
      transition: background var(--vsc-transition-fast), border-color var(--vsc-transition-fast);
    }
    .fd-item:hover { background: var(--vsc-list-hover-background); }
    .fd-item--active {
      background: var(--vsc-list-focus-background);
      border-left-color: var(--vsc-accent-blue);
      color: var(--vsc-foreground-bright);
    }
    .fd-item--active:hover { background: var(--vsc-list-active-background); }

    /* ── Checkbox ────────────────────────────────────────── */
    .fd-checkbox {
      flex-shrink: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 14px;
      height: 14px;
      border: 1px solid var(--vsc-border-bright);
      border-radius: var(--vsc-border-radius-sm);
      background: var(--vsc-input-background);
      transition: background var(--vsc-transition-fast), border-color var(--vsc-transition-fast);
    }
    .fd-checkbox--on {
      background: var(--vsc-accent-blue);
      border-color: var(--vsc-accent-blue);
    }

    .fd-item-icon  { font-size: 15px; flex-shrink: 0; width: 18px; text-align: center; line-height: 1; }
    .fd-item-label { flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

    .fd-item-count {
      flex-shrink: 0;
      font-size: 10px;
      font-family: var(--vsc-font-family-mono);
      padding: 1px var(--vsc-padding-sm);
      border-radius: var(--vsc-border-radius);
      background: var(--vsc-badge-background);
      color: var(--vsc-badge-foreground);
      min-width: 20px;
      text-align: center;
    }

    /* ── Empty state ─────────────────────────────────────── */
    .fd-empty {
      padding: var(--vsc-padding-xl);
      text-align: center;
      color: var(--vsc-foreground-dim);
      font-size: var(--vsc-font-size-small);
    }

    /* ── Footer ──────────────────────────────────────────── */
    .fd-footer {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: 32px;
      padding: 0 var(--vsc-padding-sm);
      border-top: 1px solid var(--vsc-border-subtle);
      flex-shrink: 0;
      gap: var(--vsc-padding-sm);
    }

    .fd-refresh {
      display: inline-flex;
      align-items: center;
      gap: var(--vsc-padding-xs);
      height: 22px;
      padding: 0 var(--vsc-padding-sm);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-sm);
      background: transparent;
      color: var(--vsc-foreground-dim);
      font-size: var(--vsc-font-size-small);
      font-family: var(--vsc-font-family);
      cursor: pointer;
      transition: background var(--vsc-transition-fast), color var(--vsc-transition-fast);
    }
    .fd-refresh:hover {
      background: var(--vsc-list-hover-background);
      color: var(--vsc-foreground);
    }

    .fd-hints {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-xs);
      margin-left: auto;
      font-size: 11px;
      color: var(--vsc-foreground-dim);
    }
    .fd-hints kbd {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      height: 20px;
      padding: 0 var(--vsc-padding-sm);
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-border);
      border-bottom-width: 2px;
      border-radius: var(--vsc-border-radius-sm);
      font-size: 10px;
      font-family: var(--vsc-font-family-mono);
      color: var(--vsc-foreground);
      font-weight: 600;
    }
    .fd-hint-label {
      margin-right: var(--vsc-padding-sm);
      font-size: 11px;
    }
    .fd-hint-label:last-child { margin-right: 0; }
  `],
})
export class FilterDropdownComponent implements OnChanges {

  @Input() presets:    FilterPreset[] = [];
  @Input() selected:   FilterPreset[] = [];
  @Input() hasDynamic = false;

  @Output() itemToggle      = new EventEmitter<FilterPreset>();
  @Output() clearAll        = new EventEmitter<void>();
  @Output() refresh         = new EventEmitter<void>();
  @Output() groupModeChange = new EventEmitter<{ group: string; mode: 'AND' | 'OR' }>();

  groups: FilterGroup[] = [];
  private groupModes = new Map<string, 'AND' | 'OR'>();

  constructor(private cdr: ChangeDetectorRef) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['presets']) this.buildGroups();
  }

  isSelected(preset: FilterPreset): boolean {
    return this.selected.some(p => p.id === preset.id);
  }

  getGroupMode(group: string): 'AND' | 'OR' {
    return this.groupModes.get(group) ?? 'OR';
  }

  private buildGroups(): void {
    const map = new Map<string, FilterPreset[]>();
    for (const p of this.presets) {
      const g = p.group || 'Other';
      if (!map.has(g)) map.set(g, []);
      map.get(g)!.push(p);
    }
    this.groups = Array.from(map.entries())
      .sort((a, b) => {
        const ia = GROUP_ORDER.indexOf(a[0]), ib = GROUP_ORDER.indexOf(b[0]);
        return (ia === -1 ? 999 : ia) - (ib === -1 ? 999 : ib);
      })
      .map(([name, presets]) => ({ name, presets }));
    this.cdr.markForCheck();
  }
}
