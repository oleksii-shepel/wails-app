import {
  Component, Input, Output, EventEmitter,
  OnInit, OnDestroy, OnChanges, SimpleChanges,
  ViewChild, ElementRef,
  ChangeDetectorRef,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { createSubject, debounce, distinctUntilChanged, takeUntil } from '@epikodelabs/streamix';

import { FileInfo } from '../shared/protocol';
import { FilterQuery } from '../shared/protocol-enhanced';
import { FilterService, CriteriaGroup } from '../services/filter.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';

import { DropdownDirective, DropdownTriggerDirective, DropdownMenuDirective } from '../directives/dropdown.directive';
import { DropdownContainerComponent } from './dropdown-container.component';
import { FilterDropdownComponent } from './filter-dropdown.component';
import { FilterChipComponent } from './filter-chip.component';
import {
  FilterPreset,
  FilterCategory,
  CustomCategoryData,
  FILTER_PRESETS_KEY,
  buildPresetsFromDataset,
} from './filter-bar.types';

@Component({
  selector: 'app-filter-bar',
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    DropdownTriggerDirective,
    DropdownMenuDirective,
    DropdownContainerComponent,
    FilterDropdownComponent,
    FilterChipComponent,
  ],
  template: `
<div class="fb-host" (keydown)="onHostKeyDown($event)">

  <div class="fb-bar" [class.fb-bar--compact]="compact">

    <!-- Left: filter icon + category dropdown -->
    <div class="fb-cluster fb-cluster--left">
      <button type="button"
        #categoryBtn
        appDropdownTrigger
        class="fb-btn fb-btn--category"
        [class.fb-btn--open]="showCategoryDropdown"
        [class.fb-btn--active]="selectedPresets.length > 0"
        (click)="toggleCategoryDropdown()"
        aria-label="Open advanced filters" [attr.aria-expanded]="showCategoryDropdown" title="Filter by type, size, date…"
      >
        <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" class="fb-icon">
          <path d="M6 10.5a.5.5 0 0 1 .5-.5h3a.5.5 0 0 1 0 1h-3a.5.5 0 0 1-.5-.5zm-2-3a.5.5 0 0 1 .5-.5h7a.5.5 0 0 1 0 1h-7a.5.5 0 0 1-.5-.5zm-2-3a.5.5 0 0 1 .5-.5h11a.5.5 0 0 1 0 1h-11a.5.5 0 0 1-.5-.5z"/>
        </svg>
        <span class="fb-btn-chevron" aria-hidden="true" [class.fb-btn-chevron--open]="showCategoryDropdown">›</span>
      </button>
      <app-dropdown-container class="fb-drop-anchor" [(isOpen)]="showCategoryDropdown" (isOpenChange)="onCategoryDropdownStateChange($event)" [anchorEl]="categoryBtnRef?.nativeElement ?? null">
        <app-filter-dropdown
          appDropdownMenu
          *ngIf="showCategoryDropdown"
          [contextId]="'overlay.filter.category'"
          [presets]="allPresets"
          [selected]="selectedPresets"
          [hasDynamic]="hasDynamic"
          (itemToggle)="onPresetToggle($event)"
          (clearAll)="clearPresets()"
          (refresh)="refreshPresets()"
          (groupModeChange)="onGroupModeChange($event)"
          (closeRequested)="onCategoryDropdownStateChange(false)"
        />
      </app-dropdown-container>
    </div>

    <!-- Search input -->
    <div class="fb-search">
      <svg class="fb-search-icon" aria-hidden="true" width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
        <path d="M11.742 10.344a6.5 6.5 0 1 0-1.397 1.398h-.001c.03.04.062.078.098.115l3.85 3.85a1 1 0 0 0 1.415-1.414l-3.85-3.85a1.007 1.007 0 0 0-.115-.099zM12 6.5a5.5 5.5 0 1 1-11 0 5.5 5.5 0 0 1 11 0z"/>
      </svg>
      <input
        #searchInput
        type="text"
        class="fb-search-input"
        [placeholder]="placeholder"
        [(ngModel)]="searchText"
        (ngModelChange)="onSearchChange($event)"
        (keydown)="onSearchKeyDown($event)"
        [attr.aria-label]="placeholder"
      />
      <button type="button" *ngIf="searchText" class="fb-search-clear" (click)="clearSearch()" aria-label="Clear filter search" title="Clear search">×</button>
    </div>

    <!-- Right: saved presets dropdown + clear button -->
    <div class="fb-cluster fb-cluster--right">
    <button type="button"
        #presetBtn
        class="fb-btn fb-btn--icon"
        [class.fb-btn--open]="showPresetDropdown"
        (click)="togglePresetDropdown()"
        aria-label="Open saved filter presets" title="Saved presets"
      >
        <svg width="13" height="13" viewBox="0 0 16 16" fill="currentColor">
          <path d="M2 2a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v13.5a.5.5 0 0 1-.777.416L8 13.101l-5.223 2.815A.5.5 0 0 1 2 15.5V2zm2-1a1 1 0 0 0-1 1v12.566l4.723-2.482a.5.5 0 0 1 .554 0L13 14.566V2a1 1 0 0 0-1-1H4z"/>
        </svg>
      </button>
    <app-dropdown-container class="fb-drop-anchor" [(isOpen)]="showPresetDropdown" (isOpenChange)="onPresetDropdownStateChange($event)" [anchorEl]="presetBtnRef?.nativeElement ?? null">
        <div appDropdownMenu *ngIf="showPresetDropdown" class="fb-preset-dropdown">
          <div class="fb-preset-head">
            <span>Presets</span>
            <button type="button"
              class="fb-preset-save"
              (click)="onSavePreset()"
              [disabled]="!canSavePreset"
              aria-label="Save current selection as preset" title="Save current selection as preset"
            >+ Save</button>
          </div>
          <div class="fb-preset-list">
            <div *ngFor="let p of savedPresets; let i = index" class="fb-preset-item" [class.fb-preset-item--selected]="i === selectedPresetIndex" (mouseenter)="setSelectedPresetIndex(i)">
              <span class="fb-preset-name" (click)="applyPreset(p)">{{ p.name }}</span>
              <button type="button" class="fb-preset-delete" (click)="deletePreset(p)" aria-label="Delete saved filter preset" title="Delete">×</button>
            </div>
            <div *ngIf="savedPresets.length === 0" class="fb-preset-empty" role="status">No saved presets</div>
          </div>
        </div>
      </app-dropdown-container>

      <button type="button"
        class="fb-btn fb-btn--icon fb-btn--clear"
        [disabled]="!hasAnyFilter"
        (click)="clearAllFilters()"
        aria-label="Clear all filters" title="Clear all filters"
      >
        <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
          <path d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"/>
        </svg>
      </button>
    </div>
  </div>

  <!-- Active chips -->
  <div class="fb-chips" *ngIf="selectedPresets.length > 0 || searchText">
    <div class="fb-chips-scroll">
      <!-- OR chips -->
      <ng-container *ngIf="orPresets.length > 0">
        <!-- OR badge (only if >1 OR item) -->
        <span
          *ngIf="orPresets.length > 1"
          class="fb-mode-badge fb-mode-badge--or"
          title="Any of these filters may match"
        >OR</span>

        <app-filter-chip
          *ngFor="let p of orPresets"
          [preset]="p"
          (remove)="onPresetToggle($event)"
        />
      </ng-container>

      <!-- AND chips -->
      <ng-container *ngIf="andPresets.length > 0">
        <!-- AND badge (only if OR exists + ≥1 AND) -->
        <span
          *ngIf="orPresets.length > 0"
          class="fb-mode-badge fb-mode-badge--and"
          title="All of these filters must match"
        >AND</span>

        <app-filter-chip
          *ngFor="let p of andPresets"
          [preset]="p"
          (remove)="onPresetToggle($event)"
        />
      </ng-container>

      <!-- Search chip -->
      <div *ngIf="searchText" class="fb-chip fb-chip--search" (click)="clearSearch()">
        <span class="fb-chip-label">"{{ searchText }}"</span>
        <button type="button" class="fb-chip-close" aria-label="Remove search filter" (click)="$event.stopPropagation(); clearSearch()">×</button>
      </div>
    </div>
    
    <!-- Filter count on the right -->
    <span class="fb-chip-count" *ngIf="dataset.length > 0">
      {{ filteredCount }} / {{ dataset.length }}
    </span>
  </div>

</div>
  `,
  styles: [`
    :host {
      display: block;
      font-family: var(--vsc-font-family);
      font-size: var(--vsc-font-size);
      color: var(--vsc-foreground);
      background: var(--vsc-sidebar-background);
      border-bottom: 1px solid var(--vsc-border-subtle);
    }

    .fb-host { position: relative; }

    /* ── Bar row ──────────────────────────────────────────────────── */
    .fb-bar {
      display: flex;
      align-items: center;
      height: 36px;
      padding: 0 var(--vsc-padding-sm);
      gap: var(--vsc-padding-sm);
      user-select: none;
    }
    .fb-bar--compact { height: 30px; }

    .fb-cluster {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-xs);
      flex-shrink: 0;
    }
    .fb-cluster--left {
      padding-right: var(--vsc-padding-sm);
      border-right: 1px solid var(--vsc-border-subtle);
    }
    .fb-cluster--right {
      padding-left: var(--vsc-padding-sm);
      border-left: 1px solid var(--vsc-border-subtle);
    }

    .fb-icon {
      margin: 0 15px;
      display: flex; align-items: center; justify-content: center;
      color: var(--vsc-foreground-dim);
      svg { fill: currentColor; }
    }

    .fb-drop-anchor { position: relative; }

    /* ── Buttons ──────────────────────────────────────────────────── */
    .fb-btn {
      display: flex; align-items: center; gap: var(--vsc-padding-xs);
      height: 28px; padding: 0 var(--vsc-padding-md);
      background: transparent; border: 1px solid transparent;
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground-dim);
      font-size: var(--vsc-font-size); font-family: var(--vsc-font-family);
      cursor: pointer; white-space: nowrap;
      transition: background var(--vsc-transition-fast), color var(--vsc-transition-fast), border-color var(--vsc-transition-fast);
      svg { fill: currentColor; }

      &:hover { background: var(--vsc-list-hover-background); color: var(--vsc-foreground-bright); border-color: var(--vsc-border); }
      &:focus-visible { outline: 1px solid var(--vsc-accent-blue); outline-offset: -1px; }
      &:disabled { opacity: 0.4; cursor: not-allowed; &:hover { background: transparent; border-color: transparent; } }
      &.fb-btn--open { background: var(--vsc-list-active-background); color: var(--vsc-foreground-bright); border-color: var(--vsc-border); }
      &.fb-btn--active { color: var(--vsc-accent-blue); border-color: color-mix(in srgb, var(--vsc-accent-blue) 40%, transparent); }
      &.fb-btn--active:hover { background: color-mix(in srgb, var(--vsc-accent-blue) 10%, transparent); }
    }
    .fb-btn--category { min-width: 80px; }
    .fb-btn--icon { padding: 0 var(--vsc-padding-sm); min-width: unset; }

    .fb-btn-label { flex: 1; }
    .fb-btn-chevron {
      font-size: 14px; opacity: 0.6; display: inline-block;
      transform: rotate(90deg);
      transition: transform var(--vsc-transition-fast);
    }
    .fb-btn-chevron--open { transform: rotate(270deg); }

    /* ── Search ───────────────────────────────────────────────────── */
    .fb-search {
      position: relative; flex: 1; min-width: 160px;
      display: flex; align-items: center;
    }
    .fb-search-icon {
      position: absolute; left: 8px;
      color: var(--vsc-foreground-dim); pointer-events: none; flex-shrink: 0;
    }
    .fb-search-input {
      width: 100%; height: 28px;
      padding: 0 28px 0 28px;
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-input-border);
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size); font-family: var(--vsc-font-family);
      &:focus { outline: none; border-color: var(--vsc-input-focus-border); }
      &::placeholder { color: var(--vsc-input-placeholder); }
    }
    .fb-search-clear {
      position: absolute; right: 4px;
      display: flex; align-items: center; justify-content: center;
      width: 20px; height: 20px; padding: 0;
      background: transparent; border: none; border-radius: 4px;
      color: var(--vsc-foreground-dim); font-size: 16px; cursor: pointer;
      &:hover { background: var(--vsc-list-hover-background); color: var(--vsc-foreground-bright); }
    }

    /* ── Saved preset dropdown ────────────────────────────────────── */
    .fb-preset-dropdown {
      min-width: 220px;
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      overflow: hidden;
    }
    .fb-preset-head {
      display: flex; align-items: center; justify-content: space-between;
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      border-bottom: 1px solid var(--vsc-border-subtle);
      font-size: 11px; font-weight: 700; letter-spacing: 0.08em;
      text-transform: uppercase; color: var(--vsc-foreground-dim);
    }
    .fb-preset-save {
      padding: 2px var(--vsc-padding-sm);
      background: transparent; border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground); font-size: 11px; cursor: pointer;
      font-family: var(--vsc-font-family);
      &:hover:not(:disabled) { background: var(--vsc-list-hover-background); }
      &:disabled { opacity: 0.4; cursor: not-allowed; }
    }
    .fb-preset-list { max-height: 200px; overflow-y: auto; padding: var(--vsc-padding-xs) 0; }
    .fb-preset-item {
      display: flex; align-items: center; justify-content: space-between;
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      &:hover { background: var(--vsc-list-hover-background); }
    }
    .fb-preset-item--selected {
      background: var(--vsc-list-hover-background);
    }
    .fb-preset-name {
      flex: 1; cursor: pointer; white-space: nowrap;
      overflow: hidden; text-overflow: ellipsis; font-size: var(--vsc-font-size);
    }
    .fb-preset-delete {
      display: flex; align-items: center; justify-content: center;
      width: 18px; height: 18px; padding: 0;
      background: transparent; border: none; border-radius: 4px;
      color: var(--vsc-foreground-dim); font-size: 14px; cursor: pointer;
      &:hover { background: var(--vsc-list-active-background); color: var(--vsc-foreground-bright); }
    }
    .fb-preset-empty {
      padding: var(--vsc-padding-md) var(--vsc-padding-lg);
      text-align: center; color: var(--vsc-foreground-dim);
      font-size: var(--vsc-font-size-small); font-style: italic;
    }

    /* ── AND/OR mode badges ───────────────────────────────────────── */
    .fb-mode-badge {
      display: inline-flex; align-items: center; justify-content: center;
      height: 16px; padding: 0 5px; border-radius: 3px; margin: 0 5px;
      font-size: 9px; font-weight: 700;
      font-family: var(--vsc-font-family-mono);
      min-width: 28px; flex-shrink: 0;
    }
    .fb-mode-badge--or {
      background: color-mix(in srgb, var(--vsc-accent-green) 15%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-green) 45%, transparent);
      color: var(--vsc-accent-green);
    }
    .fb-mode-badge--and {
      background: color-mix(in srgb, var(--vsc-accent-blue) 15%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue) 45%, transparent);
      color: var(--vsc-accent-blue);
    }

    /* ── Chips row ────────────────────────────────────────────────── */
    .fb-chips {
      display: flex;
      flex-wrap: nowrap;
      padding: 0 var(--vsc-padding-sm) var(--vsc-padding-xs);
      border-top: 1px solid var(--vsc-border-subtle);
    }
    
    .fb-chips-scroll {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-xs);
      overflow-x: auto;
      overflow-y: hidden;
      padding: var(--vsc-padding-xs) 0;
      flex: 1;
      min-width: 0;
    }

    /* Search chip — inline, no component */
    .fb-chip {
      display: inline-flex; align-items: center; gap: 3px;
      height: 18px; padding: 0 6px;
      background: transparent; border: 1px solid transparent;
      border-radius: 2px; flex-shrink: 0;
      color: var(--vsc-foreground-dim);
      font-size: 11.5px; font-family: var(--vsc-font-family);
      transition: background var(--vsc-transition-fast), border-color var(--vsc-transition-fast), color var(--vsc-transition-fast);
      cursor: pointer;
      &:hover { background: var(--vsc-list-hover-background); border-color: var(--vsc-border); color: var(--vsc-foreground); }
    }
    .fb-chip--search { font-style: italic; }
    .fb-chip-label { white-space: nowrap; line-height: 1; }
    .fb-chip-close {
      display: flex; align-items: center; justify-content: center;
      width: 12px; height: 12px; margin-left: 1px;
      background: transparent; border: none; border-radius: 2px;
      color: currentColor; font-size: 14px; line-height: 1; padding: 0;
      cursor: pointer; opacity: 0.55;
      transition: opacity var(--vsc-transition-fast), background var(--vsc-transition-fast);
      &:hover { opacity: 1; background: color-mix(in srgb, var(--vsc-foreground) 15%, transparent); }
    }

    .fb-chip-count {
      flex-shrink: 0;
      padding: 0 var(--vsc-padding-sm);
      height: 18px;
      color: var(--vsc-badge-foreground);
      font-size: 10px;
      font-weight: 500;
      white-space: nowrap;
      display: inline-flex;
      align-items: center;
    }
  `],
})
export class FilterBarComponent implements OnInit, OnDestroy, OnChanges {

  @ViewChild('searchInput') searchInput?: ElementRef<HTMLInputElement>;
  @ViewChild('categoryBtn') categoryBtnRef?: ElementRef<HTMLElement>;
  @ViewChild('presetBtn')   presetBtnRef?: ElementRef<HTMLElement>;

  // ── Inputs ────────────────────────────────────────────────────────────────

  @Input() dataset:           FileInfo[] = [];
  @Input() placeholder        = 'Search files…';
  @Input() compact            = false;
  @Input() initialSearch      = '';
  @Input() initialCategories: string[] = [];

  // ── Outputs ───────────────────────────────────────────────────────────────

  @Output() filterChange = new EventEmitter<FilterQuery & { groups: CriteriaGroup[] }>();

  // ── State ─────────────────────────────────────────────────────────────────

  allPresets:      FilterPreset[] = [];
  selectedPresets: FilterPreset[] = [];
  searchText       = '';

  showCategoryDropdown = false;
  showPresetDropdown   = false;
  selectedPresetIndex = -1;

  savedPresets: { id: string; name: string; presetIds: string[] }[] = [];

  showCustomDialog  = false;
  editingCategory: FilterCategory | null = null;

  private groupModes = new Map<string, 'AND' | 'OR'>();
  private destroy$   = createSubject<void>();
  private search$    = createSubject<string>();
  private readonly presetCommandOwner = `overlay.filter.presets:${Math.random().toString(36).slice(2, 8)}`;
  private releasePresetOverlayContext: (() => void) | null = null;

  // Groups that combine with OR (any one match is enough)
  private readonly orGroups  = new Set(['Basic', 'Extensions', 'Other']);
  // Groups that combine with AND (all must match)
  private readonly andGroups = new Set(['Date', 'Size', 'Special']);

  // ── Computed ──────────────────────────────────────────────────────────────

  get hasDynamic():    boolean { return this.allPresets.some(p => p.isDynamic); }
  get hasAnyFilter():  boolean { return this.selectedPresets.length > 0 || !!this.searchText; }
  get canSavePreset(): boolean { return this.selectedPresets.length > 0 && this.savedPresets.length < 20; }
  get filteredCount(): number {
    if (!this.dataset.length) return 0;
    
    // Build a filter query from current state
    let query: FilterQuery & { groups: CriteriaGroup[] };
    
    if (!this.hasAnyFilter) {
      query = this.filterService.parseFilterText('');
    } else if (this.selectedPresets.length === 0) {
      query = this.filterService.parseFilterText(this.searchText);
    } else {
      // Build the query from selected presets
      const groupMap = new Map<string, FilterPreset[]>();
      for (const p of this.selectedPresets) {
        const g = p.group || 'Other';
        if (!groupMap.has(g)) groupMap.set(g, []);
        groupMap.get(g)!.push(p);
      }

      const segments: string[] = [];
      for (const [groupName, presets] of groupMap) {
        const mode = this.groupModes.get(groupName) ?? 'OR';
        const filters = presets.map(p => (p as any).filter).filter(Boolean) as string[];
        if (filters.length === 1) {
          segments.push(filters[0]);
        } else if (mode === 'OR') {
          segments.push(filters.join(' OR '));
        } else {
          segments.push(filters.join(' '));
        }
      }
      
      query = this.filterService.parseFilterText(segments.join(' OR '));
      query.text = this.searchText;
    }
    
    // Use filterEntries to get the filtered count
    return this.filterService.filterEntries(this.dataset, query).length;
  }

  private getGroupCounts(): Map<string, number> {
    const counts = new Map<string, number>();
    for (const p of this.selectedPresets) {
      const g = p.group || 'Other';
      counts.set(g, (counts.get(g) || 0) + 1);
    }
    return counts;
  }

  get orPresets(): FilterPreset[] {
    return this.selectedPresets.filter(p =>
      this.orGroups.has(p.group || 'Other')
    );
  }

  get andPresets(): FilterPreset[] {
    return this.selectedPresets.filter(p =>
      this.andGroups.has(p.group || 'Other')
    );
  }

  // ── Constructor ───────────────────────────────────────────────────────────

  constructor(
    private filterService: FilterService,
    private elRef: ElementRef,
    private cdr: ChangeDetectorRef,
    private confirmDialog: ConfirmDialogService,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
  ) {}

  // ── Lifecycle ─────────────────────────────────────────────────────────────

  ngOnInit(): void {
    this.searchText = this.initialSearch;
    this.loadSavedPresets();
    this.rebuildPresets();
    this.registerPresetOverlayCommands();

    this.search$
      .pipe(debounce(200), distinctUntilChanged(), takeUntil(this.destroy$))
      .subscribe(() => this.emitFilterChange());
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['dataset'] && !changes['dataset'].firstChange) {
      this.rebuildPresets();
    }
  }

  ngOnDestroy(): void {
    this.releasePresetOverlayContext?.();
    this.releasePresetOverlayContext = null;
    this.unregisterPresetOverlayCommands();
    this.destroy$.next();
    this.destroy$.complete();
  }

  // ── Preset generation ─────────────────────────────────────────────────────

  rebuildPresets(): void {
    this.allPresets = buildPresetsFromDataset(this.dataset);

    const ids = new Set(this.allPresets.map(p => p.id));
    this.selectedPresets = this.selectedPresets.filter(p => ids.has(p.id));
    this.selectedPresets = this.selectedPresets.map(sel =>
      this.allPresets.find(p => p.id === sel.id) ?? sel,
    );

    this.cdr.detectChanges();
    if (this.hasAnyFilter) this.emitFilterChange();
  }

  refreshPresets(): void { this.rebuildPresets(); }

  // ── Dropdown ──────────────────────────────────────────────────────────────

  toggleCategoryDropdown(): void {
    this.onCategoryDropdownStateChange(!this.showCategoryDropdown);
  }

  onCategoryDropdownStateChange(show: boolean): void {
    this.showCategoryDropdown = show;
  }

  togglePresetDropdown(): void {
    this.onPresetDropdownStateChange(!this.showPresetDropdown);
  }

  onPresetDropdownStateChange(show: boolean): void {
    this.showPresetDropdown = show;
    if (show) {
      this.selectedPresetIndex = this.savedPresets.length > 0 ? 0 : -1;
      this.releasePresetOverlayContext ??= this.inputContexts.acquire('overlay.filter.presets');
      return;
    }

    this.releasePresetOverlayContext?.();
    this.releasePresetOverlayContext = null;
  }

  // ── Preset selection ──────────────────────────────────────────────────────

  onPresetToggle(preset: FilterPreset): void {
    const idx = this.selectedPresets.findIndex(p => p.id === preset.id);
    this.selectedPresets = idx >= 0
      ? this.selectedPresets.filter(p => p.id !== preset.id)
      : [...this.selectedPresets, preset];
    this.emitFilterChange();
    this.cdr.detectChanges();
  }

  clearPresets(): void {
    this.selectedPresets = [];
    this.emitFilterChange();
    this.cdr.detectChanges();
  }

  onGroupModeChange(ev: { group: string; mode: 'AND' | 'OR' }): void {
    this.groupModes.set(ev.group, ev.mode);
    if (this.selectedPresets.length > 0) this.emitFilterChange();
  }

  // ── Search ────────────────────────────────────────────────────────────────

  onSearchChange(value: string): void { this.searchText = value; this.search$.next(value); }
  onSearchKeyDown(e: KeyboardEvent): void { if (e.key === 'Escape') this.clearSearch(); }

  clearSearch(): void {
    this.searchText = '';
    this.emitFilterChange();
    this.cdr.detectChanges();
  }

  clearAllFilters(): void {
    this.selectedPresets = [];
    this.searchText = '';
    this.emitFilterChange();
    this.cdr.detectChanges();
  }

  // ── Saved presets ─────────────────────────────────────────────────────────

  loadSavedPresets(): void {
    try { this.savedPresets = JSON.parse(localStorage.getItem(FILTER_PRESETS_KEY) || '[]'); }
    catch { this.savedPresets = []; }
  }

  persistSavedPresets(): void {
    try { localStorage.setItem(FILTER_PRESETS_KEY, JSON.stringify(this.savedPresets)); } catch {}
  }

  async onSavePreset(): Promise<void> {
    const name = (await this.confirmDialog.prompt({
      title: 'Save Preset',
      message: 'Enter preset name.',
      confirmText: 'Save',
      cancelText: 'Cancel',
      placeholder: 'Preset name',
      defaultValue: '',
      historyKey: 'filter.presetName',
    }))?.trim();
    if (!name) return;
    this.savedPresets = [...this.savedPresets, {
      id: `preset-${Date.now()}`,
      name,
      presetIds: this.selectedPresets.map(p => p.id),
    }];
    this.persistSavedPresets();
    this.onPresetDropdownStateChange(false);
  }

  applyPreset(saved: { id: string; name: string; presetIds: string[] }): void {
    this.selectedPresets = saved.presetIds
      .map(id => this.allPresets.find(p => p.id === id))
      .filter(Boolean) as FilterPreset[];
    this.onPresetDropdownStateChange(false);
    this.emitFilterChange();
    this.cdr.detectChanges();
  }

  deletePreset(saved: { id: string; name: string; presetIds: string[] }): void {
    this.savedPresets = this.savedPresets.filter(p => p.id !== saved.id);
    this.persistSavedPresets();
  }

  // ── Custom categories ─────────────────────────────────────────────────────

  onCustomCategorySave(_data: CustomCategoryData): void { this.closeCustomDialog(); }
  closeCustomDialog(): void { this.showCustomDialog = false; this.editingCategory = null; }

  // ── Keyboard ──────────────────────────────────────────────────────────────

  onHostKeyDown(e: KeyboardEvent): void {
    if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
      e.preventDefault();
      this.searchInput?.nativeElement.focus();
    }
  }

  onDocKey(e: KeyboardEvent): void {
    if (e.key === 'Escape') {
      if (this.showCategoryDropdown) { this.onCategoryDropdownStateChange(false); e.preventDefault(); }
      if (this.showPresetDropdown)   { this.onPresetDropdownStateChange(false); e.preventDefault(); }
      if (this.showCustomDialog)     { this.closeCustomDialog();          e.preventDefault(); }
    }
  }

  setSelectedPresetIndex(index: number): void {
    if (index < 0 || index >= this.savedPresets.length) return;
    this.selectedPresetIndex = index;
  }

  private movePresetSelection(delta: number): void {
    if (this.savedPresets.length === 0) return;
    const baseIndex = this.selectedPresetIndex >= 0 ? this.selectedPresetIndex : 0;
    this.selectedPresetIndex = Math.max(0, Math.min(this.savedPresets.length - 1, baseIndex + delta));
    this.cdr.markForCheck();
  }

  private applySelectedPreset(): void {
    const preset = this.savedPresets[this.selectedPresetIndex];
    if (!preset) return;
    this.applyPreset(preset);
  }

  private registerPresetOverlayCommands(): void {
    this.inputEngine.registerCommand('overlay.filter.presets.close', () => {
      if (!this.showPresetDropdown) return;
      this.onPresetDropdownStateChange(false);
    }, this.presetCommandOwner);

    this.inputEngine.registerCommand('overlay.filter.presets.prevItem', () => {
      if (!this.showPresetDropdown) return;
      this.movePresetSelection(-1);
    }, this.presetCommandOwner);

    this.inputEngine.registerCommand('overlay.filter.presets.nextItem', () => {
      if (!this.showPresetDropdown) return;
      this.movePresetSelection(1);
    }, this.presetCommandOwner);

    this.inputEngine.registerCommand('overlay.filter.presets.select', () => {
      if (!this.showPresetDropdown) return;
      this.applySelectedPreset();
    }, this.presetCommandOwner);
  }

  private unregisterPresetOverlayCommands(): void {
    this.inputEngine.unregisterCommand('overlay.filter.presets.close', this.presetCommandOwner);
    this.inputEngine.unregisterCommand('overlay.filter.presets.prevItem', this.presetCommandOwner);
    this.inputEngine.unregisterCommand('overlay.filter.presets.nextItem', this.presetCommandOwner);
    this.inputEngine.unregisterCommand('overlay.filter.presets.select', this.presetCommandOwner);
  }

  // ── Emit ──────────────────────────────────────────────────────────────────

  private emitFilterChange(): void {
    if (!this.hasAnyFilter) {
      this.filterChange.emit(this.filterService.parseFilterText(''));
      return;
    }
    if (this.selectedPresets.length === 0) {
      this.filterChange.emit(this.filterService.parseFilterText(this.searchText));
      return;
    }

    // Group presets → build combined filter string
    const groupMap = new Map<string, FilterPreset[]>();
    for (const p of this.selectedPresets) {
      const g = p.group || 'Other';
      if (!groupMap.has(g)) groupMap.set(g, []);
      groupMap.get(g)!.push(p);
    }

    const segments: string[] = [];
    for (const [groupName, presets] of groupMap) {
      const mode    = this.groupModes.get(groupName) ?? 'OR';
      const filters = presets.map(p => (p as any).filter).filter(Boolean) as string[];
      if (filters.length === 1) {
        segments.push(filters[0]);
      } else if (mode === 'OR') {
        segments.push(filters.join(' OR '));
      } else {
        segments.push(filters.join(' '));
      }
    }

    const query    = this.filterService.parseFilterText(segments.join(' OR '));
    query.text     = this.searchText;
    this.filterChange.emit(query);
  }
}