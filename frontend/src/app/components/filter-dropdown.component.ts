import {
  Component, Input, Output, EventEmitter,
  ChangeDetectionStrategy, ChangeDetectorRef, OnChanges, SimpleChanges, OnDestroy, OnInit
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FilterPreset, FilterGroup, GROUP_ORDER, GROUP_MODES } from './filter-bar.types';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';

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

        <span
          *ngIf="selected.length > 0"
          class="fd-header-count"
          title="Selected filters"
        >
          {{ selected.length }}
        </span>

        <button
          class="fd-header-clear"
          *ngIf="selected.length > 0"
          (click)="onClearAllClick($event)"
          title="Clear all filters"
        >Clear</button>
      </div>

      <!-- ── Dynamic content hint ──────────────────────────────── -->
      <div class="fd-dynamic-hint" *ngIf="hasDynamic">
        <span class="fd-dynamic-icon">📊</span>
        <span>List reflects current folder content</span>
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
                class="fb-group-badge"
                [class.fb-group-badge--or]="getGroupMode(group.name) === 'OR'"
                [class.fb-group-badge--and]="getGroupMode(group.name) === 'AND'"
                [title]="'Items in this group are combined with ' + getGroupMode(group.name)"
              >{{ getGroupMode(group.name) }}</span>
            </div>

            <!-- Items -->
            <div
              class="fd-item"
              *ngFor="let preset of group.presets; let i = index"
              role="menuitemcheckbox"
              [attr.aria-checked]="isSelected(preset)"
              [class.fd-item--selected]="isSelected(preset)"
              [class.fd-item--focused]="isFocused(group.name, preset.id)"
              [class.fd-item--dynamic]="preset.isDynamic"
              (click)="onItemClick(preset, $event)"
              (mouseenter)="setFocusedItem(group.name, preset.id)"
            >
              <span class="fd-checkbox" [class.fd-checkbox--on]="isSelected(preset)" aria-hidden="true">
                <svg *ngIf="isSelected(preset)" width="8" height="8" viewBox="0 0 16 16" fill="currentColor">
                  <path d="M13.485 1.929L5.5 9.914 2.515 6.929a1 1 0 0 0-1.414 1.414l3.692 3.693a1 1 0 0 0 1.414 0l8.692-8.693a1 1 0 0 0-1.414-1.414z"/>
                </svg>
              </span>
              <span class="fd-item-icon">{{ preset.icon || '📄' }}</span>
              <span class="fd-item-label">{{ preset.name }}</span>
            </div>

          </ng-container>
        </ng-container>
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
      color: var(--vsc-badge-foreground);
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
      flex: 1 1 auto;
      min-height: 0;
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
    .fb-group-badge {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      height: 16px;
      padding: 0 5px;
      border-radius: 3px;
      font-size: 9px;
      font-weight: 700;
      font-family: var(--vsc-font-family-mono, Consolas, monospace);
      min-width: 28px;
    }

    .fb-group-badge--or {
      background: color-mix(in srgb, #2d7d46 20%, transparent);
      border: 1px solid rgba(45,125,70,0.4);
      color: #7ee787;
    }

    .fb-group-badge--and {
      background: color-mix(in srgb, #007fd4 20%, transparent);
      border: 1px solid rgba(0,127,212,0.4);
      color: #7ec8f8;
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
    .fd-item:hover {
      background: var(--vsc-list-hover-background);
    }
    .fd-item--selected {
      background: var(--vsc-list-active-background);
      color: var(--vsc-foreground-bright);
      border-left-color: var(--vsc-accent-blue);
    }
    .fd-item--selected:hover {
      background: var(--vsc-list-active-background);
    }
    .fd-item--focused:not(.fd-item--selected) {
      background: var(--vsc-list-hover-background);
    }

    /* ── Checkbox ────────────────────────────────────────── */
    .fd-checkbox {
      flex-shrink: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 14px;
      height: 14px;
      border: 1px solid var(--vsc-checkbox-border);
      border-radius: var(--vsc-border-radius-sm);
      background: var(--vsc-checkbox-background);
      transition: background var(--vsc-transition-fast), border-color var(--vsc-transition-fast);
    }
    .fd-checkbox--on {
      background: var(--vsc-accent-blue);
      border-color: var(--vsc-accent-blue);
    }

    .fd-item-icon  { 
      font-size: 15px; 
      flex-shrink: 0; 
      width: 18px; 
      text-align: center; 
      line-height: 1; 
      opacity: 0.9;
    }

    .fd-item-label { 
      flex: 1; 
      white-space: nowrap; 
      overflow: hidden; 
      text-overflow: ellipsis; 
    }

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
  `],
})
export class FilterDropdownComponent implements OnChanges, OnDestroy, OnInit {

  @Input() presets:    FilterPreset[] = [];
  @Input() selected:   FilterPreset[] = [];
  @Input() hasDynamic = false;
  @Input() contextId = 'overlay.filter.category';

  @Output() itemToggle      = new EventEmitter<FilterPreset>();
  @Output() clearAll        = new EventEmitter<void>();
  @Output() refresh         = new EventEmitter<void>();
  @Output() groupModeChange = new EventEmitter<{ group: string; mode: 'AND' | 'OR' }>();
  @Output() closeRequested  = new EventEmitter<void>();

  groups: FilterGroup[] = [];
  private groupModes = new Map<string, 'AND' | 'OR'>();
  private focusedGroupName: string | null = null;
  private focusedPresetId: string | null = null;
  private readonly commandOwner = `filterDropdown:${Math.random().toString(36).slice(2, 8)}`;
  private releaseOverlayContext: (() => void) | null = null;

  // Define which groups are OR vs AND
  private readonly orGroups = new Set(['Basic', 'Extensions', 'Other']);
  private readonly andGroups = new Set(['Date', 'Size', 'Special']);

  constructor(
    private cdr: ChangeDetectorRef,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
  ) {}

  ngOnInit(): void {
    this.initializeGroupModes();
    this.registerInputCommands();
    this.releaseOverlayContext ??= this.inputContexts.acquire(this.contextId);
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['presets']) {
      this.buildGroups();
    }
  }

  ngOnDestroy(): void {
    this.releaseOverlayContext?.();
    this.releaseOverlayContext = null;
    this.unregisterInputCommands();
  }

  private initializeGroupModes(): void {
    // OR groups
    this.orGroups.forEach(group => this.groupModes.set(group, 'OR'));
    // AND groups
    this.andGroups.forEach(group => this.groupModes.set(group, 'AND'));
  }

  isSelected(preset: FilterPreset): boolean {
    return this.selected.some(p => p.id === preset.id);
  }

  getGroupMode(group: string): 'AND' | 'OR' {
    return this.groupModes.get(group) ?? 'OR';
  }

  onItemClick(preset: FilterPreset, event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.itemToggle.emit(preset);
  }

  onClearAllClick(event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.clearAll.emit();
  }

  onRefreshClick(event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.refresh.emit();
  }

  isFocused(groupName: string, presetId: string): boolean {
    return this.focusedGroupName === groupName && this.focusedPresetId === presetId;
  }

  setFocusedItem(groupName: string, presetId: string): void {
    this.focusedGroupName = groupName;
    this.focusedPresetId = presetId;
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
        const groupA = a[0];
        const groupB = b[0];
        
        const aIsOr = this.orGroups.has(groupA);
        const bIsOr = this.orGroups.has(groupB);
        
        if (aIsOr && !bIsOr) return -1;
        if (!aIsOr && bIsOr) return 1;
        
        const ia = GROUP_ORDER.indexOf(groupA);
        const ib = GROUP_ORDER.indexOf(groupB);
        return (ia === -1 ? 999 : ia) - (ib === -1 ? 999 : ib);
      })
      .map(([name, presets]) => ({ 
        name, 
        presets: this.sortPresetsByCount(presets)
      }));

    const firstPreset = this.groups[0]?.presets[0] ?? null;
    if (firstPreset) {
      this.focusedGroupName ??= this.groups[0].name;
      this.focusedPresetId ??= firstPreset.id;
    } else {
      this.focusedGroupName = null;
      this.focusedPresetId = null;
    }
    
    this.cdr.markForCheck();
  }

  private sortPresetsByCount(presets: FilterPreset[]): FilterPreset[] {
    return [...presets].sort((a, b) => {
      const countA = a.count ?? 0;
      const countB = b.count ?? 0;
      if (countA !== countB) return countB - countA;
      return a.name.localeCompare(b.name);
    });
  }

  private moveFocus(delta: number): void {
    const items = this.groups.flatMap(group => group.presets.map(preset => ({ groupName: group.name, preset })));
    if (items.length === 0) return;

    const currentIndex = items.findIndex(item => this.isFocused(item.groupName, item.preset.id));
    const baseIndex = currentIndex >= 0 ? currentIndex : 0;
    const nextIndex = Math.max(0, Math.min(items.length - 1, baseIndex + delta));
    this.focusedGroupName = items[nextIndex].groupName;
    this.focusedPresetId = items[nextIndex].preset.id;
    this.cdr.markForCheck();
  }

  private toggleFocused(): void {
    const preset = this.groups
      .flatMap(group => group.presets)
      .find(item => item.id === this.focusedPresetId);
    if (!preset) return;
    this.itemToggle.emit(preset);
  }

  private registerInputCommands(): void {
    const prefix = this.contextId === 'overlay.filter.presets' ? 'overlay.filter.presets' : 'overlay.filter.category';

    this.inputEngine.registerCommand(`${prefix}.close`, () => {
      this.closeRequested.emit();
    }, this.commandOwner);

    this.inputEngine.registerCommand(`${prefix}.prevItem`, () => {
      this.moveFocus(-1);
    }, this.commandOwner);

    this.inputEngine.registerCommand(`${prefix}.nextItem`, () => {
      this.moveFocus(1);
    }, this.commandOwner);

    this.inputEngine.registerCommand(`${prefix}.select`, () => {
      this.toggleFocused();
    }, this.commandOwner);
  }

  private unregisterInputCommands(): void {
    const prefix = this.contextId === 'overlay.filter.presets' ? 'overlay.filter.presets' : 'overlay.filter.category';
    this.inputEngine.unregisterCommand(`${prefix}.close`, this.commandOwner);
    this.inputEngine.unregisterCommand(`${prefix}.prevItem`, this.commandOwner);
    this.inputEngine.unregisterCommand(`${prefix}.nextItem`, this.commandOwner);
    this.inputEngine.unregisterCommand(`${prefix}.select`, this.commandOwner);
  }
}