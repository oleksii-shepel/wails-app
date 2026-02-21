// filter-bar.component.ts
import {
  Component, Input, Output, EventEmitter,
  OnInit, OnDestroy, ViewChild, ElementRef,
  ChangeDetectorRef, HostListener
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Subject, debounceTime, distinctUntilChanged, takeUntil } from 'rxjs';

import { DropdownDirective, DropdownTriggerDirective, DropdownMenuDirective } from '../directives/dropdown.directive';
import { FilterBarCategoryDropdownComponent } from './filter-bar-category-dropdown.component';
import { FilterBarCustomCategoryDialogComponent } from './filter-bar-custom-category-dialog.component';
import {
  FilterCategory,
  FilterPreset,
  BUILT_IN_CATEGORIES,
  FILTER_PRESETS_KEY,
  CustomCategoryData
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
    FilterBarCategoryDropdownComponent,
    FilterBarCustomCategoryDialogComponent
  ],
  template: `
<div class="fb-host" (keydown)="onHostKeyDown($event)">

  <!-- Main filter bar -->
  <div class="fb-bar" [class.fb-bar--compact]="compact">

    <!-- Left cluster: filter icon + category dropdown -->
    <div class="fb-cluster fb-cluster--left">
      <div class="fb-icon">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
          <path d="M1.5 2A1.5 1.5 0 0 0 0 3.5v9A1.5 1.5 0 0 0 1.5 14h13a1.5 1.5 0 0 0 1.5-1.5v-9A1.5 1.5 0 0 0 14.5 2h-13zM1 3.5a.5.5 0 0 1 .5-.5h13a.5.5 0 0 1 .5.5v9a.5.5 0 0 1-.5.5h-13a.5.5 0 0 1-.5-.5v-9z"/>
          <path d="M3 6h10v1H3zm0 3h10v1H3z"/>
        </svg>
      </div>

      <div class="fb-drop-anchor" appDropdown [(isOpen)]="showCategoryDropdown">
        <button
          appDropdownTrigger
          class="fb-btn fb-btn--category"
          [class.fb-btn--open]="showCategoryDropdown"
          [class.fb-btn--active]="hasActiveFilters"
          (click)="toggleCategoryDropdown()"
          [title]="'Filter by category'"
        >
          <span class="fb-btn-label">Category</span>
          <span class="fb-btn-badge" *ngIf="activeCategoryCount">{{ activeCategoryCount }}</span>
          <span class="fb-btn-chevron">›</span>
        </button>

        <app-filter-bar-category-dropdown
          appDropdownMenu
          *ngIf="showCategoryDropdown"
          [categories]="categories"
          [selectedCategoryIds]="selectedCategoryIds"
          [left]="categoryDropLeft"
          [top]="categoryDropTop"
          (categorySelect)="onCategorySelect($event)"
          (addCustom)="onAddCustom($event)"
          (editCustom)="onEditCustom($event)"
          (deleteCustom)="onDeleteCustom($event)"
        />
      </div>
    </div>

    <!-- Search input -->
    <div class="fb-search">
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
      <button
        *ngIf="searchText"
        class="fb-search-clear"
        (click)="clearSearch()"
        title="Clear search"
      >×</button>
    </div>

    <!-- Right cluster: preset dropdown + clear button -->
    <div class="fb-cluster fb-cluster--right">
      <div class="fb-drop-anchor" appDropdown [(isOpen)]="showPresetDropdown">
        <button
          appDropdownTrigger
          class="fb-btn fb-btn--preset"
          [class.fb-btn--open]="showPresetDropdown"
          (click)="togglePresetDropdown()"
          title="Filter presets"
        >
          <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
            <path d="M9.5 1.5a1 1 0 0 0-1.415 0L4.94 4.645a3.5 3.5 0 0 0-.871 3.547l-2.646 2.647a.5.5 0 0 0 .708.707l2.646-2.646a3.5 3.5 0 0 0 3.547-.872l3.144-3.144A1 1 0 0 0 11.5 3l-2-1.5z"/>
          </svg>
        </button>

        <div
          appDropdownMenu
          *ngIf="showPresetDropdown"
          class="fb-preset-dropdown"
        >
          <div class="fb-preset-head">
            <span>Filter Presets</span>
            <button
              class="fb-preset-save"
              (click)="onSavePreset()"
              [disabled]="!canSavePreset"
              title="Save current filters as preset"
            >+ Save</button>
          </div>

          <div class="fb-preset-list">
            <div
              *ngFor="let preset of presets"
              class="fb-preset-item"
              [class.fb-preset-item--builtin]="preset.isBuiltIn"
            >
              <span class="fb-preset-name" (click)="applyPreset(preset)">
                {{ preset.name }}
              </span>
              <button
                *ngIf="!preset.isBuiltIn"
                class="fb-preset-delete"
                (click)="deletePreset(preset)"
                title="Delete preset"
              >×</button>
            </div>

            <div *ngIf="presets.length === 0" class="fb-preset-empty">
              No saved presets
            </div>
          </div>
        </div>
      </div>

      <button
        class="fb-btn fb-btn--clear"
        [disabled]="!hasActiveFilters && !searchText"
        (click)="clearAllFilters()"
        title="Clear all filters"
      >
        <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
          <path d="M2.5 1A1.5 1.5 0 0 0 1 2.5v11A1.5 1.5 0 0 0 2.5 15h11a1.5 1.5 0 0 0 1.5-1.5v-11A1.5 1.5 0 0 0 13.5 1h-11zM2 2.5a.5.5 0 0 1 .5-.5h11a.5.5 0 0 1 .5.5v11a.5.5 0 0 1-.5.5h-11a.5.5 0 0 1-.5-.5v-11z"/>
          <path d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"/>
        </svg>
      </button>
    </div>
  </div>

  <!-- Active filter chips -->
  <div class="fb-chips" *ngIf="activeCategories.length > 0 || searchText">
    <div class="fb-chips-scroll">
      <div
        *ngFor="let cat of activeCategories"
        class="fb-chip"
        [class.fb-chip--custom]="cat.isCustom"
      >
        <span class="fb-chip-icon">{{ cat.icon || '📌' }}</span>
        <span class="fb-chip-label">{{ cat.label }}</span>
        <button
          class="fb-chip-remove"
          (click)="removeCategory(cat.id)"
          title="Remove filter"
        >×</button>
      </div>

      <div *ngIf="searchText" class="fb-chip fb-chip--search">
        <span class="fb-chip-label">"{{ searchText }}"</span>
        <button
          class="fb-chip-remove"
          (click)="clearSearch()"
          title="Clear search"
        >×</button>
      </div>
    </div>
  </div>

  <!-- Custom category dialog -->
  <app-filter-bar-custom-category-dialog
    *ngIf="showCustomDialog"
    [category]="editingCategory"
    [left]="dialogLeft"
    [top]="dialogTop"
    (save)="onCustomCategorySave($event)"
    (cancel)="closeCustomDialog()"
  />

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

    .fb-host {
      position: relative;
      background: var(--vsc-sidebar-background);
    }

    .fb-bar {
      display: flex;
      align-items: center;
      height: 36px;
      padding: 0 var(--vsc-padding-sm);
      gap: var(--vsc-padding-sm);
      background: var(--vsc-sidebar-background);
      user-select: none;
    }

    .fb-bar--compact {
      height: 30px;
      
      .fb-btn { height: 24px; }
      .fb-search-input { height: 24px; }
    }

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
      width: 26px;
      height: 26px;
      color: var(--vsc-foreground-dim);
      
      svg { fill: currentColor; }
    }

    .fb-drop-anchor {
      position: relative;
    }

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
      cursor: pointer;
      transition: all var(--vsc-transition-fast);
      white-space: nowrap;

      &:hover {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground-bright);
        border-color: var(--vsc-border);
      }

      &:focus-visible {
        outline: 1px solid var(--vsc-focusBorder, var(--vsc-accent-blue));
        outline-offset: -1px;
      }

      &:disabled {
        opacity: 0.5;
        cursor: not-allowed;
        &:hover {
          background: transparent;
          border-color: transparent;
        }
      }

      &.fb-btn--open {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
        border-color: var(--vsc-border);
      }

      &.fb-btn--active {
        color: var(--vsc-accent-blue);
        .fb-btn-badge {
          background: var(--vsc-accent-blue);
          color: white;
        }
      }

      svg { fill: currentColor; }
    }

    .fb-btn--category {
      min-width: 90px;
    }

    .fb-btn--preset {
      padding: 0 var(--vsc-padding-sm);
    }

    .fb-btn--clear {
      padding: 0 var(--vsc-padding-sm);
    }

    .fb-btn-label {
      flex: 1;
    }

    .fb-btn-badge {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 18px;
      height: 18px;
      padding: 0 4px;
      background: var(--vsc-badge-background);
      color: var(--vsc-badge-foreground);
      border-radius: 10px;
      font-size: 10px;
      font-weight: 600;
    }

    .fb-btn-chevron {
      font-size: 14px;
      transform: rotate(90deg);
      opacity: 0.7;
    }

    .fb-search {
      position: relative;
      flex: 1;
      min-width: 200px;
    }

    .fb-search-input {
      width: 100%;
      height: 28px;
      padding: 0 var(--vsc-padding-md);
      padding-right: 28px;
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size);
      
      &:focus {
        outline: none;
        border-color: var(--vsc-focusBorder, var(--vsc-accent-blue));
      }
      
      &::placeholder {
        color: var(--vsc-foreground-dim);
        opacity: 0.7;
      }
    }

    .fb-search-clear {
      position: absolute;
      right: 4px;
      top: 50%;
      transform: translateY(-50%);
      display: flex;
      align-items: center;
      justify-content: center;
      width: 20px;
      height: 20px;
      padding: 0;
      background: transparent;
      border: none;
      border-radius: 4px;
      color: var(--vsc-foreground-dim);
      font-size: 16px;
      cursor: pointer;
      
      &:hover {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground-bright);
      }
    }

    /* Preset dropdown */
    .fb-preset-dropdown {
      position: fixed;
      min-width: 220px;
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      z-index: 10000;
      overflow: hidden;
    }

    .fb-preset-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: var(--vsc-padding-sm) var(--vsc-padding-lg);
      background: var(--vsc-panel-background);
      border-bottom: 1px solid var(--vsc-border-subtle);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-dim);
      text-transform: uppercase;
    }

    .fb-preset-save {
      padding: 2px var(--vsc-padding-sm);
      background: transparent;
      border: 1px solid var(--vsc-border);
      border-radius: 4px;
      color: var(--vsc-foreground);
      font-size: 11px;
      cursor: pointer;
      
      &:hover:not(:disabled) {
        background: var(--vsc-list-hover-background);
        border-color: var(--vsc-focusBorder);
      }
      
      &:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }
    }

    .fb-preset-list {
      max-height: 200px;
      overflow-y: auto;
      padding: var(--vsc-padding-xs) 0;
    }

    .fb-preset-item {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: var(--vsc-padding-sm) var(--vsc-padding-lg);
      
      &:hover {
        background: var(--vsc-list-hover-background);
      }
      
      &.fb-preset-item--builtin {
        color: var(--vsc-foreground-dim);
        font-style: italic;
      }
    }

    .fb-preset-name {
      flex: 1;
      cursor: pointer;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .fb-preset-delete {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 18px;
      height: 18px;
      padding: 0;
      background: transparent;
      border: none;
      border-radius: 4px;
      color: var(--vsc-foreground-dim);
      font-size: 14px;
      cursor: pointer;
      
      &:hover {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
      }
    }

    .fb-preset-empty {
      padding: var(--vsc-padding-lg);
      text-align: center;
      color: var(--vsc-foreground-dim);
      font-size: var(--vsc-font-size-small);
      font-style: italic;
    }

    /* Filter chips */
    .fb-chips {
      padding: 0 var(--vsc-padding-sm) var(--vsc-padding-xs);
      border-top: 1px solid var(--vsc-border-subtle);
      background: var(--vsc-sidebar-background);
    }

    .fb-chips-scroll {
      display: flex;
      gap: var(--vsc-padding-xs);
      overflow-x: auto;
      overflow-y: hidden;
      padding: var(--vsc-padding-xs) 0;
      scrollbar-width: thin;
      
      &::-webkit-scrollbar {
        height: 4px;
      }
    }

    .fb-chip {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-xs);
      height: 24px;
      padding: 0 var(--vsc-padding-sm);
      background: var(--vsc-badge-background);
      border-radius: 12px;
      color: var(--vsc-badge-foreground);
      font-size: var(--vsc-font-size-small);
      white-space: nowrap;
      
      &.fb-chip--custom {
        background: color-mix(in srgb, var(--vsc-accent-purple) 20%, transparent);
        border: 1px solid color-mix(in srgb, var(--vsc-accent-purple) 40%, transparent);
      }
      
      &.fb-chip--search {
        background: var(--vsc-input-background);
        border: 1px solid var(--vsc-border);
      }
    }

    .fb-chip-icon {
      font-size: 12px;
      opacity: 0.8;
    }

    .fb-chip-label {
      max-width: 120px;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .fb-chip-remove {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 16px;
      height: 16px;
      padding: 0;
      background: transparent;
      border: none;
      border-radius: 4px;
      color: currentColor;
      font-size: 14px;
      cursor: pointer;
      opacity: 0.7;
      
      &:hover {
        opacity: 1;
        background: rgba(255,255,255,0.1);
      }
    }
  `]
})
export class FilterBarComponent implements OnInit, OnDestroy {
  @ViewChild('searchInput') searchInput?: ElementRef<HTMLInputElement>;

  @Input() placeholder = 'Filter files and folders...';
  @Input() compact = false;
  @Input() initialSearch = '';
  @Input() initialCategories: string[] = [];

  @Output() filterChange = new EventEmitter<{
    search: string;
    categories: FilterCategory[];
  }>();

  // Categories
  categories: FilterCategory[] = [...BUILT_IN_CATEGORIES];
  selectedCategoryIds: string[] = [];

  // Category dropdown
  showCategoryDropdown = false;
  categoryDropLeft = 0;
  categoryDropTop = 0;

  // Search
  searchText = '';

  // Presets
  presets: FilterPreset[] = [];
  showPresetDropdown = false;

  // Custom category dialog
  showCustomDialog = false;
  editingCategory: FilterCategory | null = null;
  dialogLeft = 0;
  dialogTop = 0;

  private destroy$ = new Subject<void>();
  private searchChange$ = new Subject<string>();

  constructor(
    private elRef: ElementRef,
    private cdr: ChangeDetectorRef
  ) { }

  ngOnInit(): void {
    this.searchText = this.initialSearch;
    this.selectedCategoryIds = this.initialCategories;
    this.loadPresets();

    this.searchChange$
      .pipe(debounceTime(300), distinctUntilChanged(), takeUntil(this.destroy$))
      .subscribe(search => this.emitFilterChange());

    // Load custom categories from storage
    this.loadCustomCategories();
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  // Getters
  get activeCategories(): FilterCategory[] {
    return this.categories.filter(c => this.selectedCategoryIds.includes(c.id));
  }

  get activeCategoryCount(): number {
    return this.activeCategories.length;
  }

  get hasActiveFilters(): boolean {
    return this.activeCategoryCount > 0;
  }

  get canSavePreset(): boolean {
    return this.hasActiveFilters && this.presets.length < 20; // Limit to 20 presets
  }

  // Category dropdown
  toggleCategoryDropdown(): void {
    if (this.showCategoryDropdown) {
      this.showCategoryDropdown = false;
    } else {
      const btn = this.elRef.nativeElement.querySelector('.fb-btn--category');
      if (btn) {
        const rect = btn.getBoundingClientRect();
        this.categoryDropLeft = rect.left;
        this.categoryDropTop = rect.bottom + 4;
        this.showCategoryDropdown = true;
      }
    }
  }

  onCategorySelect(category: FilterCategory): void {
    if (category.id === 'all') {
      // 'All' clears other selections
      this.selectedCategoryIds = ['all'];
    } else {
      // Toggle category
      const index = this.selectedCategoryIds.indexOf(category.id);
      if (index >= 0) {
        this.selectedCategoryIds.splice(index, 1);
      } else {
        // Remove 'all' if present
        this.selectedCategoryIds = this.selectedCategoryIds.filter(id => id !== 'all');
        this.selectedCategoryIds.push(category.id);
      }
    }

    this.emitFilterChange();
  }

  removeCategory(categoryId: string): void {
    this.selectedCategoryIds = this.selectedCategoryIds.filter(id => id !== categoryId);
    this.emitFilterChange();
  }

  // Search
  onSearchChange(value: string): void {
    this.searchText = value;
    this.searchChange$.next(value);
  }

  onSearchKeyDown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      this.clearSearch();
    }
  }

  clearSearch(): void {
    this.searchText = '';
    this.emitFilterChange();
  }

  // Custom categories
  // In filter-bar.component.ts - update these methods:

  loadCustomCategories(): void {
    try {
      const saved = localStorage.getItem('filter-bar-custom-categories');
      if (saved) {
        const customCats = JSON.parse(saved);
        this.categories = [
          ...BUILT_IN_CATEGORIES,
          ...customCats.map((c: any) => ({
            id: c.id,
            label: c.label,
            icon: c.icon,
            pattern: c.pattern,
            isCustom: true,
            predicate: this.createPredicateFromPattern(c.pattern)
          }))
        ];
      }
    } catch (e) {
      console.error('Failed to load custom categories', e);
    }
  }

  saveCustomCategories(): void {
    const customCats = this.categories
      .filter(c => c.isCustom)
      .map(({ id, label, icon, pattern }) => ({ id, label, icon, pattern }));

    try {
      localStorage.setItem('filter-bar-custom-categories', JSON.stringify(customCats));
    } catch (e) {
      console.error('Failed to save custom categories', e);
    }
  }

  createPredicateFromPattern(pattern: string): (entry: any) => boolean {
    const patterns = pattern.split(',').map(p => p.trim().toLowerCase());
    return (entry) => {
      const name = entry.name?.toLowerCase() || '';
      return patterns.some(p => {
        if (p.includes('*')) {
          const regex = new RegExp('^' + p.replace(/\*/g, '.*') + '$');
          return regex.test(name);
        }
        return name.includes(p);
      });
    };
  }

  onAddCustom(event: MouseEvent): void {
    const btn = event.currentTarget as HTMLElement;
    const rect = btn.getBoundingClientRect();
    this.dialogLeft = rect.left - 160; // Center roughly
    this.dialogTop = rect.bottom + 4;
    this.editingCategory = null;
    this.showCustomDialog = true;
  }

  onEditCustom(event: { category: FilterCategory; event: MouseEvent }): void {
    const rect = (event.event.currentTarget as HTMLElement).getBoundingClientRect();
    this.dialogLeft = rect.left - 160;
    this.dialogTop = rect.bottom + 4;
    this.editingCategory = event.category;
    this.showCustomDialog = true;
  }

  onDeleteCustom(event: { category: FilterCategory; event: MouseEvent }): void {
    event.event.stopPropagation();
    this.categories = this.categories.filter(c => c.id !== event.category.id);
    this.selectedCategoryIds = this.selectedCategoryIds.filter(id => id !== event.category.id);
    this.saveCustomCategories();
    this.emitFilterChange();
  }

  onCustomCategorySave(data: CustomCategoryData): void {
    if (this.editingCategory) {
      // Update existing
      const index = this.categories.findIndex(c => c.id === this.editingCategory!.id);
      if (index >= 0) {
        this.categories[index] = {
          ...this.categories[index],
          label: data.label,
          icon: data.icon,
          pattern: data.pattern,
          predicate: this.createPredicateFromPattern(data.pattern)
        };
      }
    } else {
      // Add new
      const newCategory: FilterCategory = {
        id: `custom-${Date.now()}`,
        label: data.label,
        icon: data.icon,
        pattern: data.pattern,
        isCustom: true,
        predicate: this.createPredicateFromPattern(data.pattern)
      };
      this.categories.push(newCategory);
    }

    this.saveCustomCategories();
    this.closeCustomDialog();
    this.cdr.detectChanges();
  }

  closeCustomDialog(): void {
    this.showCustomDialog = false;
    this.editingCategory = null;
  }

  // Presets
  loadPresets(): void {
    try {
      this.presets = JSON.parse(localStorage.getItem(FILTER_PRESETS_KEY) || '[]');
    } catch {
      this.presets = [];
    }
  }

  savePresets(): void {
    try {
      localStorage.setItem(FILTER_PRESETS_KEY, JSON.stringify(this.presets));
    } catch (e) {
      console.error('Failed to save presets', e);
    }
  }

  togglePresetDropdown(): void {
    this.showPresetDropdown = !this.showPresetDropdown;
  }

  onSavePreset(): void {
    const name = prompt('Enter preset name:');
    if (!name) return;

    const preset: FilterPreset = {
      id: `preset-${Date.now()}`,
      name,
      categories: [...this.selectedCategoryIds],
      isBuiltIn: false
    };

    this.presets.push(preset);
    this.savePresets();
    this.showPresetDropdown = false;
  }

  applyPreset(preset: FilterPreset): void {
    this.selectedCategoryIds = [...preset.categories];
    this.showPresetDropdown = false;
    this.emitFilterChange();
  }

  deletePreset(preset: FilterPreset): void {
    this.presets = this.presets.filter(p => p.id !== preset.id);
    this.savePresets();
  }

  // Clear all
  clearAllFilters(): void {
    this.selectedCategoryIds = [];
    this.searchText = '';
    this.emitFilterChange();
  }

  // Emit filter change
  private emitFilterChange(): void {
    this.filterChange.emit({
      search: this.searchText,
      categories: this.activeCategories
    });
    this.cdr.detectChanges();
  }

  // Keyboard handling
  onHostKeyDown(event: KeyboardEvent): void {
    if ((event.ctrlKey || event.metaKey) && event.key === 'f') {
      event.preventDefault();
      this.searchInput?.nativeElement.focus();
    }
  }

  @HostListener('document:keydown', ['$event'])
  onDocKey(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      if (this.showCategoryDropdown) {
        this.showCategoryDropdown = false;
        event.preventDefault();
      }
      if (this.showPresetDropdown) {
        this.showPresetDropdown = false;
        event.preventDefault();
      }
      if (this.showCustomDialog) {
        this.closeCustomDialog();
        event.preventDefault();
      }
    }
  }
}