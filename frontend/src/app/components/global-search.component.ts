import { Component, OnInit, OnDestroy, ViewChild, ElementRef, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { createSubject } from '@epikodelabs/streamix';
import { takeUntil, debounce } from '@epikodelabs/streamix';
import { GlobalSearchService, SearchResultItem, IndexStats, SearchConfig } from '../services/global-search.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';
import { FileType } from '../shared/protocol';
import { getFileEntryPresentation } from './file-entry-presentation.util';
import { DialogFocusTrapDirective } from '../directives/dialog-focus-trap.directive';
import { moveFocusWithin } from '../directives/focus-trap.utils';
import { VscodeScrollbarDirective } from '../directives/vscode-scrollbar.directive';

@Component({
  selector: 'app-global-search',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogFocusTrapDirective, VscodeScrollbarDirective],
  templateUrl: './global-search.component.html',
  styleUrls: ['./global-search.component.scss']
})
export class GlobalSearchComponent implements OnInit, OnDestroy {
  @ViewChild('searchInput') searchInput?: ElementRef<HTMLInputElement>;
  @ViewChild('settingsPanel') settingsPanel?: ElementRef<HTMLElement>;
  @Output() resultSelected = new EventEmitter<SearchResultItem>();
  @Output() searchFolderRequested = new EventEmitter<string>();

  isVisible = false;
  searchQuery = '';
  results: SearchResultItem[] = [];
  isSearching = false;
  selectedIndex = 0;
  indexStats: IndexStats | null = null;
  
  activeFilters = new Set<string>(['all']);
  showSettingsModal = false;
  
  searchConfig: SearchConfig = {
    maxResults: 50,
    fuzzyThreshold: 0.7,
    enableTypoTolerance: true,
    searchHidden: false,
    fileTypes: [FileType.FILE, FileType.DIRECTORY],
  };

  private destroy$ = createSubject<void>();
  private searchSubject = createSubject<string>();
  private releaseSearchContext: (() => void) | null = null;
  private releaseSearchSettingsContext: (() => void) | null = null;

  constructor(
    private searchService: GlobalSearchService,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
    private overlayFreeze: NativeOverlayFreezeService,
  ) {}

  ngOnInit(): void {
    this.registerInputCommands();

    // Subscribe to index stats
    this.searchService.stats$
      .pipe(takeUntil(this.destroy$))
      .subscribe(stats => {
        this.indexStats = stats;
      });

    // Setup debounced search
    this.searchSubject
      .pipe(
        debounce(300),
        takeUntil(this.destroy$)
      )
      .subscribe(query => {
        this.performSearch(query);
      });

    // Load config
    this.searchConfig = this.searchService.getConfig();
  }

  ngOnDestroy(): void {
    this.overlayFreeze.release('dialog.globalSearch');
    this.releaseSearchSettingsContext?.();
    this.releaseSearchSettingsContext = null;
    this.releaseSearchContext?.();
    this.releaseSearchContext = null;
    this.unregisterInputCommands();
    this.destroy$.next();
    this.destroy$.complete();
  }

  /**
   * Show the global search modal
   */
  show(): void {
    this.isVisible = true;
    this.releaseSearchContext ??= this.inputContexts.acquire('dialog.globalSearch');
    this.overlayFreeze.acquire('dialog.globalSearch');
    this.searchQuery = '';
    this.results = [];
    this.selectedIndex = 0;
    this.activeFilters = new Set(['all']);
    setTimeout(() => this.searchInput?.nativeElement.focus(), 0);
  }

  /**
   * Hide the global search modal
   */
  hide(): void {
    this.isVisible = false;
    this.searchQuery = '';
    this.results = [];
    this.hideSettings();
    this.releaseSearchContext?.();
    this.releaseSearchContext = null;
    this.overlayFreeze.release('dialog.globalSearch');
  }

  /**
   * Handle search query change
   */
  onSearchChange(): void {
    if (!this.searchQuery || this.searchQuery.trim() === '') {
      this.results = [];
      this.isSearching = false;
      return;
    }

    this.isSearching = true;
    this.searchSubject.next(this.searchQuery);
  }

  /**
   * Perform the search
   */
  private performSearch(query: string): void {
    try {
      const config = this.getActiveSearchConfig();
      const rawResults = this.searchService.search(query, config);

      // Apply active filters
      this.results = this.applyFilters(rawResults);
      this.selectedIndex = 0;
    } catch (error) {
      console.error('Search error:', error);
      this.results = [];
    } finally {
      this.isSearching = false;
    }
  }

  /**
   * Get search config based on active filters
   */
  private getActiveSearchConfig(): Partial<SearchConfig> {
    const config: Partial<SearchConfig> = { ...this.searchConfig };

    // File type filters
    if (this.activeFilters.has('files') && !this.activeFilters.has('folders')) {
      config.fileTypes = [FileType.FILE];
    } else if (this.activeFilters.has('folders') && !this.activeFilters.has('files')) {
      config.fileTypes = [FileType.DIRECTORY];
    } else {
      config.fileTypes = [FileType.FILE, FileType.DIRECTORY];
    }

    return config;
  }

  /**
   * Apply additional filters to results
   */
  private applyFilters(results: SearchResultItem[]): SearchResultItem[] {
    let filtered = results;

    // Recent filter (last 7 days)
    if (this.activeFilters.has('recent')) {
      const weekAgo = Date.now() / 1000 - (7 * 24 * 60 * 60);
      filtered = filtered.filter(r => r.file.modified >= weekAgo);
    }

    return filtered;
  }

  /**
   * Toggle a filter
   */
  toggleFilter(filter: string): void {
    if (filter === 'all') {
      this.activeFilters.clear();
      this.activeFilters.add('all');
    } else {
      this.activeFilters.delete('all');
      
      if (this.activeFilters.has(filter)) {
        this.activeFilters.delete(filter);
      } else {
        this.activeFilters.add(filter);
      }

      // If no filters, default to 'all'
      if (this.activeFilters.size === 0) {
        this.activeFilters.add('all');
      }
    }

    // Re-search with new filters
    if (this.searchQuery) {
      this.performSearch(this.searchQuery);
    }
  }

  /**
   * Clear search
   */
  clearSearch(): void {
    this.searchQuery = '';
    this.results = [];
    this.selectedIndex = 0;
    this.searchInput?.nativeElement.focus();
  }

  /**
   * Select a result
   */
  selectResult(result: SearchResultItem): void {
    this.resultSelected.emit(result);
    this.hide();
  }

  openSearchFolder(): void {
    const trimmedQuery = this.searchQuery.trim();
    if (!trimmedQuery) {
      return;
    }

    this.searchFolderRequested.emit(trimmedQuery);
    this.hide();
  }

  /**
   * Get file icon
   */
  getFileIcon(file: SearchResultItem['file']): string {
    return getFileEntryPresentation(file as any, false).iconEmoji;
  }

  isSymlink(file: SearchResultItem['file']): boolean {
    return getFileEntryPresentation(file as any, false).isSymlink;
  }

  /**
   * Refresh the search index
   */
  async refreshIndex(): Promise<void> {
    this.isSearching = true;
    try {
      await this.searchService.refreshIndex();
    } finally {
      this.isSearching = false;
    }
  }

  /**
   * Show settings modal
   */
  showSettings(): void {
    this.showSettingsModal = true;
    this.releaseSearchSettingsContext ??= this.inputContexts.acquire('dialog.globalSearch.settings');
  }

  /**
   * Hide settings modal
   */
  hideSettings(): void {
    this.showSettingsModal = false;
    this.releaseSearchSettingsContext?.();
    this.releaseSearchSettingsContext = null;
  }

  /**
   * Update search configuration
   */
  updateSearchConfig(): void {
    this.searchService.updateConfig(this.searchConfig);
    
    // Re-search if there's an active query
    if (this.searchQuery) {
      this.performSearch(this.searchQuery);
    }
  }

  /**
   * Reset settings to defaults
   */
  resetSettings(): void {
    this.searchConfig = {
      maxResults: 50,
      fuzzyThreshold: 0.7,
      enableTypoTolerance: true,
      searchHidden: false,
      fileTypes: [FileType.FILE, FileType.DIRECTORY],
    };
    this.updateSearchConfig();
  }

  /**
   * Handle keyboard navigation
   */
  /**
   * Scroll to selected result
   */
  private scrollToSelected(): void {
    setTimeout(() => {
      const selected = document.querySelector('.result-item.selected');
      if (selected) {
        selected.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
      }
    }, 0);
  }

  private registerInputCommands(): void {
    this.inputEngine.registerCommand('dialog.globalSearch.close', () => {
      if (!this.isVisible) return;
      this.hide();
    });

    this.inputEngine.registerCommand('dialog.globalSearch.prevResult', () => {
      if (!this.isVisible || this.showSettingsModal || this.results.length === 0) return;
      this.selectedIndex = Math.max(this.selectedIndex - 1, 0);
      this.scrollToSelected();
    });

    this.inputEngine.registerCommand('dialog.globalSearch.nextResult', () => {
      if (!this.isVisible || this.showSettingsModal || this.results.length === 0) return;
      this.selectedIndex = Math.min(this.selectedIndex + 1, this.results.length - 1);
      this.scrollToSelected();
    });

    this.inputEngine.registerCommand('dialog.globalSearch.openSelected', () => {
      if (!this.isVisible || this.showSettingsModal) return;
      if (this.results.length === 0 || this.selectedIndex >= this.results.length) return;
      this.selectResult(this.results[this.selectedIndex]);
    });

    this.inputEngine.registerCommand('dialog.globalSearch.openFolder', () => {
      if (!this.isVisible || this.showSettingsModal) return;
      if (!this.searchQuery.trim()) return;
      this.openSearchFolder();
    });

    this.inputEngine.registerCommand('dialog.globalSearch.openSettings', () => {
      if (!this.isVisible) return;
      this.showSettings();
    });

    this.inputEngine.registerCommand('dialog.globalSearch.settings.close', () => {
      if (!this.showSettingsModal) return;
      this.hideSettings();
    });

    this.inputEngine.registerCommand('dialog.globalSearch.settings.apply', () => {
      if (!this.showSettingsModal) return;
      this.updateSearchConfig();
      this.hideSettings();
    });

    this.inputEngine.registerCommand('dialog.globalSearch.settings.focusNextArrow', () => {
      if (!this.showSettingsModal || !this.shouldUseSettingsArrowFocusNavigation()) return;
      this.moveSettingsFocus(1);
    });

    this.inputEngine.registerCommand('dialog.globalSearch.settings.focusPrevArrow', () => {
      if (!this.showSettingsModal || !this.shouldUseSettingsArrowFocusNavigation()) return;
      this.moveSettingsFocus(-1);
    });
  }

  private unregisterInputCommands(): void {
    this.inputEngine.unregisterCommand('dialog.globalSearch.close');
    this.inputEngine.unregisterCommand('dialog.globalSearch.prevResult');
    this.inputEngine.unregisterCommand('dialog.globalSearch.nextResult');
    this.inputEngine.unregisterCommand('dialog.globalSearch.openSelected');
    this.inputEngine.unregisterCommand('dialog.globalSearch.openFolder');
    this.inputEngine.unregisterCommand('dialog.globalSearch.openSettings');
    this.inputEngine.unregisterCommand('dialog.globalSearch.settings.close');
    this.inputEngine.unregisterCommand('dialog.globalSearch.settings.apply');
    this.inputEngine.unregisterCommand('dialog.globalSearch.settings.focusNextArrow');
    this.inputEngine.unregisterCommand('dialog.globalSearch.settings.focusPrevArrow');
  }

  private moveSettingsFocus(delta: number): void {
    const panel = this.settingsPanel?.nativeElement;
    if (!panel) {
      return;
    }

    moveFocusWithin(panel, delta, '.settings-footer button');
  }

  private shouldUseSettingsArrowFocusNavigation(): boolean {
    const panel = this.settingsPanel?.nativeElement;
    const activeElement = document.activeElement;
    if (!panel || !(activeElement instanceof HTMLElement) || !panel.contains(activeElement)) {
      return false;
    }

    return !!activeElement.closest('.settings-footer button');
  }
}





