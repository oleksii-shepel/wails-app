import {
  Component, Input, Output, EventEmitter,
  OnInit, OnDestroy, OnChanges, SimpleChanges,
  ViewChild, ElementRef,
  ChangeDetectorRef, HostListener,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Subject, debounceTime, distinctUntilChanged, takeUntil } from 'rxjs';

import { FileInfo } from '../shared/protocol';
import { FilterQuery } from '../shared/protocol-enhanced';
import { FilterService, CriteriaGroup } from '../services/filter.service';

import { DropdownDirective, DropdownTriggerDirective, DropdownMenuDirective } from '../directives/dropdown.directive';
import { FilterDropdownComponent } from './filter-bar-dropdown.component';
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
    DropdownDirective,
    DropdownTriggerDirective,
    DropdownMenuDirective,
    FilterDropdownComponent,
  ],
  template: `
<div class="fb-host" (keydown)="onHostKeyDown($event)">

  <div class="fb-bar" [class.fb-bar--compact]="compact">

    <!-- Left: filter icon + category dropdown -->
    <div class="fb-cluster fb-cluster--left">
      <div class="fb-icon">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
          <path d="M6 10.5a.5.5 0 0 1 .5-.5h3a.5.5 0 0 1 0 1h-3a.5.5 0 0 1-.5-.5zm-2-3a.5.5 0 0 1 .5-.5h7a.5.5 0 0 1 0 1h-7a.5.5 0 0 1-.5-.5zm-2-3a.5.5 0 0 1 .5-.5h11a.5.5 0 0 1 0 1h-11a.5.5 0 0 1-.5-.5z"/>
        </svg>
      </div>

      <div class="fb-drop-anchor" appDropdown [(isOpen)]="showCategoryDropdown">
        <button
          appDropdownTrigger
          class="fb-btn fb-btn--category"
          [class.fb-btn--open]="showCategoryDropdown"
          [class.fb-btn--active]="selectedPresets.length > 0"
          (click)="toggleCategoryDropdown()"
          title="Filter by type, size, date…"
        >
          <span class="fb-btn-label">Filter</span>
          <span class="fb-btn-chevron" [class.fb-btn-chevron--open]="showCategoryDropdown">›</span>
        </button>

        <app-filter-dropdown
          appDropdownMenu
          *ngIf="showCategoryDropdown"
          [presets]="allPresets"
          [selected]="selectedPresets"
          [hasDynamic]="hasDynamic"
          (itemToggle)="onPresetToggle($event)"
          (clearAll)="clearPresets()"
          (refresh)="refreshPresets()"
          (groupModeChange)="onGroupModeChange($event)"
        />
      </div>
    </div>

    <!-- Search input -->
    <div class="fb-search">
      <svg class="fb-search-icon" width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
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
      <button *ngIf="searchText" class="fb-search-clear" (click)="clearSearch()" title="Clear search">×</button>
    </div>

    <!-- Right: preset dropdown + clear button -->
    <div class="fb-cluster fb-cluster--right">
      <div class="fb-drop-anchor" appDropdown [(isOpen)]="showPresetDropdown">
        <button
          appDropdownTrigger
          class="fb-btn fb-btn--icon"
          [class.fb-btn--open]="showPresetDropdown"
          (click)="showPresetDropdown = !showPresetDropdown"
          title="Saved presets"
        >
          <svg width="13" height="13" viewBox="0 0 16 16" fill="currentColor">
            <path d="M2 2a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v13.5a.5.5 0 0 1-.777.416L8 13.101l-5.223 2.815A.5.5 0 0 1 2 15.5V2zm2-1a1 1 0 0 0-1 1v12.566l4.723-2.482a.5.5 0 0 1 .554 0L13 14.566V2a1 1 0 0 0-1-1H4z"/>
          </svg>
        </button>

        <div appDropdownMenu *ngIf="showPresetDropdown" class="fb-preset-dropdown">
          <div class="fb-preset-head">
            <span>Presets</span>
            <button
              class="fb-preset-save"
              (click)="onSavePreset()"
              [disabled]="!canSavePreset"
              title="Save current selection as preset"
            >+ Save</button>
          </div>
          <div class="fb-preset-list">
            <div *ngFor="let p of savedPresets" class="fb-preset-item">
              <span class="fb-preset-name" (click)="applyPreset(p)">{{ p.name }}</span>
              <button class="fb-preset-delete" (click)="deletePreset(p)" title="Delete">×</button>
            </div>
            <div *ngIf="savedPresets.length === 0" class="fb-preset-empty">No saved presets</div>
          </div>
        </div>
      </div>

      <button
        class="fb-btn fb-btn--icon fb-btn--clear"
        [disabled]="!hasAnyFilter"
        (click)="clearAllFilters()"
        title="Clear all filters"
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

      <div *ngFor="let p of selectedPresets" class="fb-chip">
        <span class="fb-chip-icon">{{ p.icon || '🔍' }}</span>
        <span class="fb-chip-label">{{ p.name }}</span>
        <button class="fb-chip-close" (click)="onPresetToggle(p)" title="Remove">×</button>
      </div>
    </div>
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
      display: flex;
      align-items: center;
      justify-content: center;
      width: 24px; height: 24px;
      color: var(--vsc-foreground-dim);
      svg { fill: currentColor; }
    }

    .fb-drop-anchor { position: relative; }

    .fb-btn {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-xs);
      height: 28px;
      padding: 0 var(--vsc-padding-md);
      background: transparent;
      border: 1px solid transparent;
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground-dim);
      font-size: var(--vsc-font-size);
      font-family: var(--vsc-font-family);
      cursor: pointer;
      transition: background var(--vsc-transition-fast), color var(--vsc-transition-fast), border-color var(--vsc-transition-fast);
      white-space: nowrap;
      svg { fill: currentColor; }

      &:hover {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground-bright);
        border-color: var(--vsc-border);
      }
      &:focus-visible { outline: 1px solid var(--vsc-accent-blue); outline-offset: -1px; }
      &:disabled { opacity: 0.4; cursor: not-allowed; &:hover { background: transparent; border-color: transparent; } }
      &.fb-btn--open { background: var(--vsc-list-active-background); color: var(--vsc-foreground-bright); border-color: var(--vsc-border); }
      &.fb-btn--active { color: var(--vsc-accent-blue); border-color: color-mix(in srgb, var(--vsc-accent-blue) 40%, transparent); }
      &.fb-btn--active:hover { background: color-mix(in srgb, var(--vsc-accent-blue) 10%, transparent); }
    }

    .fb-btn--category { min-width: 80px; }
    .fb-btn--icon { padding: 0 var(--vsc-padding-sm); min-width: unset; }

    .fb-btn-label { flex: 1; }
    .fb-btn-badge {
      display: inline-flex; align-items: center; justify-content: center;
      min-width: 18px; height: 18px; padding: 0 4px;
      background: var(--vsc-accent-blue); color: #fff;
      border-radius: 10px; font-size: 10px; font-weight: 700;
    }
    .fb-btn-chevron {
      font-size: 14px; opacity: 0.6;
      transition: transform var(--vsc-transition-fast);
      display: inline-block;
      transform: rotate(90deg);
    }
    .fb-btn-chevron--open { transform: rotate(270deg); }

    /* ── Search ───────────────────────────────────────────────────── */
    .fb-search {
      position: relative;
      flex: 1;
      min-width: 160px;
      display: flex;
      align-items: center;
    }
    .fb-search-icon {
      position: absolute;
      left: 8px;
      color: var(--vsc-foreground-dim);
      pointer-events: none;
      flex-shrink: 0;
    }
    .fb-search-input {
      width: 100%;
      height: 28px;
      padding: 0 28px 0 28px;
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-input-border);
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size);
      font-family: var(--vsc-font-family);
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

    /* ── Preset dropdown ──────────────────────────────────────────── */
    .fb-preset-dropdown {
      position: fixed;
      min-width: 220px;
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      z-index: var(--vsc-z-dropdown);
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

    /* ── Chips ────────────────────────────────────────────────────── */
    .fb-chips {
      padding: 0 var(--vsc-padding-sm) var(--vsc-padding-xs);
      border-top: 1px solid var(--vsc-border-subtle);
    }
    .fb-chips-scroll {
      display: flex; gap: var(--vsc-padding-xs);
      overflow-x: auto; overflow-y: hidden;
      padding: var(--vsc-padding-xs) 0;
      scrollbar-width: thin;
      &::-webkit-scrollbar { height: 3px; }
    }
    .fb-chip {
      display: inline-flex; align-items: center; gap: 3px;
      height: 18px; padding: 0 6px;
      background: transparent; border: 1px solid transparent;
      border-radius: 2px; cursor: pointer; flex-shrink: 0;
      color: var(--vsc-foreground-dim, #858585);
      font-size: 11.5px; font-family: inherit;
      transition: background 0.08s, border-color 0.08s, color 0.08s;

      &:hover {
        background: var(--vsc-list-hover-background, #2a2d2e);
        border-color: var(--vsc-border, #3c3c3c);
        color: var(--vsc-foreground, #ccc);
      }

      &.fb-chip--on {
        background: #094771;
        border-color: rgba(0,127,212,0.4);
        color: #ccddff;
      }

      /* Extra chips (from dropdown, not a default category) */
      &.fb-chip--extra {
        border-style: dashed;
        border-color: var(--vsc-border, #3c3c3c);
        &.fb-chip--on { border-style: solid; }
      }
    }

    .fb-chip-icon  { font-size: 11px; line-height: 1; flex-shrink: 0; }
    .fb-chip-label { white-space: nowrap; line-height: 1; }
    .fb-chip-count {
      font-size: 9px; font-weight: 600; opacity: 0.55; margin-left: 1px;
      font-family: var(--vsc-font-family-mono, Consolas, monospace);
    }
    .fb-chip-close {
      display: flex; align-items: center; justify-content: center;
      background: transparent; border: none; color: var(--vsc-foreground-dim);
      width: 12px; height: 12px; margin-left: 1px;
      opacity: 0.6; border-radius: 2px;
      transition: opacity 0.1s, background 0.1s;
      &:hover { opacity: 1; background: rgba(255,255,255,0.15); }
    }
  `],
})
export class FilterBarComponent implements OnInit, OnDestroy, OnChanges {

  @ViewChild('searchInput') searchInput?: ElementRef<HTMLInputElement>;

  // ── Inputs ──────────────────────────────────────────────────────────────

  /** The full unfiltered dataset — drives dynamic preset generation */
  @Input() dataset: FileInfo[] = [];

  @Input() placeholder        = 'Search files…';
  @Input() compact            = false;
  @Input() initialSearch      = '';
  @Input() initialCategories: string[] = [];

  // ── Outputs ─────────────────────────────────────────────────────────────

  /**
   * Emits a FilterQuery (compatible with FilterService.filterEntries) every
   * time the search text or selected presets change.
   */
  @Output() filterChange = new EventEmitter<FilterQuery & { groups: CriteriaGroup[] }>();

  // ── State ────────────────────────────────────────────────────────────────

  allPresets:      FilterPreset[] = [];
  selectedPresets: FilterPreset[] = [];
  searchText       = '';

  showCategoryDropdown = false;
  showPresetDropdown   = false;

  savedPresets: { id: string; name: string; presetIds: string[] }[] = [];

  showCustomDialog = false;
  editingCategory: FilterCategory | null = null;
  dialogLeft = 0;
  dialogTop  = 0;

  private groupModes = new Map<string, 'AND' | 'OR'>();
  private destroy$   = new Subject<void>();
  private search$    = new Subject<string>();

  // ── Computed ─────────────────────────────────────────────────────────────

  get hasDynamic(): boolean     { return this.allPresets.some(p => p.isDynamic); }
  get hasAnyFilter(): boolean   { return this.selectedPresets.length > 0 || !!this.searchText; }
  get canSavePreset(): boolean  { return this.selectedPresets.length > 0 && this.savedPresets.length < 20; }

  constructor(
    private filterService: FilterService,
    private elRef: ElementRef,
    private cdr: ChangeDetectorRef,
  ) {}

  // ── Lifecycle ─────────────────────────────────────────────────────────────

  ngOnInit(): void {
    this.searchText = this.initialSearch;
    this.loadSavedPresets();
    this.rebuildPresets();

    // Debounce search input
    this.search$
      .pipe(debounceTime(200), distinctUntilChanged(), takeUntil(this.destroy$))
      .subscribe(() => this.emitFilterChange());
  }

  ngOnChanges(changes: SimpleChanges): void {
    // Re-analyse whenever the dataset changes (new folder navigated to, etc.)
    if (changes['dataset'] && !changes['dataset'].firstChange) {
      this.rebuildPresets();
    }
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  // ── Preset generation ─────────────────────────────────────────────────────

  /**
   * Re-analyse dataset → rebuild allPresets with live counts.
   * Also drops any selectedPreset that no longer exists in the new list.
   */
  rebuildPresets(): void {
    this.allPresets = buildPresetsFromDataset(this.dataset);

    // Drop selections that vanished (e.g. navigated to a folder with no images)
    const ids = new Set(this.allPresets.map(p => p.id));
    this.selectedPresets = this.selectedPresets.filter(p => ids.has(p.id));

    // Refresh counts on already-selected presets
    this.selectedPresets = this.selectedPresets.map(sel =>
      this.allPresets.find(p => p.id === sel.id) ?? sel,
    );

    this.cdr.detectChanges();

    // Emit updated query so the file list re-filters with new counts
    if (this.hasAnyFilter) this.emitFilterChange();
  }

  refreshPresets(): void { this.rebuildPresets(); }

  // ── Dropdown toggle ───────────────────────────────────────────────────────

  toggleCategoryDropdown(): void {
    this.showCategoryDropdown = !this.showCategoryDropdown;
  }

  // ── Preset selection ──────────────────────────────────────────────────────

  onPresetToggle(preset: FilterPreset): void {
    const idx = this.selectedPresets.findIndex(p => p.id === preset.id);
    if (idx >= 0) {
      this.selectedPresets = this.selectedPresets.filter(p => p.id !== preset.id);
    } else {
      this.selectedPresets = [...this.selectedPresets, preset];
    }
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

  onSearchChange(value: string): void {
    this.searchText = value;
    this.search$.next(value);
  }

  onSearchKeyDown(e: KeyboardEvent): void {
    if (e.key === 'Escape') this.clearSearch();
  }

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

  onSavePreset(): void {
    const name = prompt('Preset name:');
    if (!name?.trim()) return;
    this.savedPresets = [
      ...this.savedPresets,
      { id: `preset-${Date.now()}`, name: name.trim(), presetIds: this.selectedPresets.map(p => p.id) },
    ];
    this.persistSavedPresets();
    this.showPresetDropdown = false;
  }

  applyPreset(saved: { id: string; name: string; presetIds: string[] }): void {
    this.selectedPresets = saved.presetIds
      .map(id => this.allPresets.find(p => p.id === id))
      .filter(Boolean) as FilterPreset[];
    this.showPresetDropdown = false;
    this.emitFilterChange();
    this.cdr.detectChanges();
  }

  deletePreset(saved: { id: string; name: string; presetIds: string[] }): void {
    this.savedPresets = this.savedPresets.filter(p => p.id !== saved.id);
    this.persistSavedPresets();
  }

  // ── Custom categories (dialog plumbing kept from original) ────────────────

  onCustomCategorySave(data: CustomCategoryData): void { this.closeCustomDialog(); }
  closeCustomDialog(): void { this.showCustomDialog = false; this.editingCategory = null; }

  // ── Keyboard ──────────────────────────────────────────────────────────────

  onHostKeyDown(e: KeyboardEvent): void {
    if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
      e.preventDefault();
      this.searchInput?.nativeElement.focus();
    }
  }

  @HostListener('document:keydown', ['$event'])
  onDocKey(e: KeyboardEvent): void {
    if (e.key === 'Escape') {
      if (this.showCategoryDropdown)  { this.showCategoryDropdown = false; e.preventDefault(); }
      if (this.showPresetDropdown)    { this.showPresetDropdown   = false; e.preventDefault(); }
      if (this.showCustomDialog)      { this.closeCustomDialog();          e.preventDefault(); }
    }
  }

  // ── Emit ──────────────────────────────────────────────────────────────────

  /**
   * Build a FilterQuery from selected presets + search text and emit.
   *
   * Logic:
   *  • Each preset's filter string is parsed via FilterService.parseFilterText().
   *  • Presets in the same group are combined according to that group's mode (AND/OR).
   *  • Groups are combined with OR (entry must pass any one group).
   *  • The search text is AND-ed globally (top-level query.text).
   */
  private emitFilterChange(): void {
    if (!this.hasAnyFilter) {
      this.filterChange.emit(this.filterService.parseFilterText(''));
      return;
    }

    if (this.selectedPresets.length === 0) {
      // Only text search
      this.filterChange.emit(this.filterService.parseFilterText(this.searchText));
      return;
    }

    // Group presets by group name
    const groupMap = new Map<string, FilterPreset[]>();
    for (const p of this.selectedPresets) {
      const g = p.group || 'Other';
      if (!groupMap.has(g)) groupMap.set(g, []);
      groupMap.get(g)!.push(p);
    }

    // Build one OR-segment per group
    const segments: string[] = [];
    for (const [groupName, presets] of groupMap) {
      const mode = this.groupModes.get(groupName) ?? 'OR';
      const filterStrings = presets.map(p => p.filter).filter(Boolean);

      if (filterStrings.length === 1) {
        segments.push(filterStrings[0]);
      } else if (mode === 'OR') {
        // Join multiple presets within group with " OR "
        segments.push(filterStrings.join(' OR '));
      } else {
        // AND: concatenate tokens (FilterService ANDs different types within a segment)
        segments.push(filterStrings.join(' '));
      }
    }

    // All groups are OR-ed together
    const combinedFilter = segments.join(' OR ');
    const query = this.filterService.parseFilterText(combinedFilter);

    // Global text search on top
    query.text = this.searchText;

    this.filterChange.emit(query);
  }
}
