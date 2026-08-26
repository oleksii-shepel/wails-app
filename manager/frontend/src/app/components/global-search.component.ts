import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  OnDestroy,
  OnInit,
  Output,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { DiscoverySearchService } from '../services/discovery-search.service';
import {
  DiscoveryHistoryService,
  type DiscoverySearchSpec,
  type DiscoverySearchType,
  type SavedDiscoverySearch,
} from '../services/discovery-history.service';
import { RecentFilesService } from '../services/recent-files.service';
import {
  ResultsPanelService,
  type ResultsPanelDefinition,
} from '../services/results-panel.service';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { FileInfo, FileType } from '../shared/protocol';
import {
  SearchResultIndex,
  type SearchResultSort,
} from '../services/search-result-index';
import { VirtualCollection } from '../services/virtual-window';

type SearchSort = SearchResultSort;
type DiscoveryMode = 'search' | 'recent' | 'collections';

export interface SearchResultsPanelRequest {
  readonly title: string;
  readonly rootPath: string;
  readonly query: string;
  readonly recursive: boolean;
  readonly entries: readonly FileInfo[];
}

const CONTEXT_ID = 'overlay.GlobalSearch';
const OWNER_ID = 'overlay.GlobalSearch';

const COMMAND = {
  close: 'reassembly.globalSearch.close',
  previous: 'reassembly.globalSearch.previous',
  next: 'reassembly.globalSearch.next',
  open: 'reassembly.globalSearch.open',
  openFolder: 'reassembly.globalSearch.openFolder',
  toggleRecursive: 'reassembly.globalSearch.toggleRecursive',
  cycleSort: 'reassembly.globalSearch.cycleSort',
  previousQuery: 'reassembly.globalSearch.previousQuery',
  nextQuery: 'reassembly.globalSearch.nextQuery',
  openAsPanel: 'reassembly.globalSearch.openAsPanel',
} as const;

const BINDINGS: readonly InputBindingDefinition[] = [
  { id:'reassembly.globalSearch.escape', commandId:COMMAND.close, contextId:CONTEXT_ID, source:'default', description:'Close global search', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'escape'}]}}},
  { id:'reassembly.globalSearch.up', commandId:COMMAND.previous, contextId:CONTEXT_ID, source:'default', description:'Previous search result', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'arrowup'}]}}},
  { id:'reassembly.globalSearch.down', commandId:COMMAND.next, contextId:CONTEXT_ID, source:'default', description:'Next search result', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'arrowdown'}]}}},
  { id:'reassembly.globalSearch.enter', commandId:COMMAND.open, contextId:CONTEXT_ID, source:'default', description:'Open search result', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'enter'}]}}},
  { id:'reassembly.globalSearch.ctrlEnter', commandId:COMMAND.openFolder, contextId:CONTEXT_ID, source:'default', description:'Open containing folder', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'enter',modifiers:['ctrl']}]}}},
  { id:'reassembly.globalSearch.ctrlShiftEnter', commandId:COMMAND.openAsPanel, contextId:CONTEXT_ID, source:'default', description:'Open search results as a panel', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'enter',modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.globalSearch.toggleRecursive.altR', commandId:COMMAND.toggleRecursive, contextId:CONTEXT_ID, source:'default', description:'Toggle recursive search', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'r',modifiers:['alt']}]}}},
  { id:'reassembly.globalSearch.cycleSort.altS', commandId:COMMAND.cycleSort, contextId:CONTEXT_ID, source:'default', description:'Cycle search sort', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'s',modifiers:['alt']}]}}},
  { id:'reassembly.globalSearch.previousQuery.altUp', commandId:COMMAND.previousQuery, contextId:CONTEXT_ID, source:'default', description:'Previous search query', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'arrowup',modifiers:['alt']}]}}},
  { id:'reassembly.globalSearch.nextQuery.altDown', commandId:COMMAND.nextQuery, contextId:CONTEXT_ID, source:'default', description:'Next search query', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'arrowdown',modifiers:['alt']}]}}},
];

const SEARCH_RESULT_ROW_HEIGHT = 52;
const SEARCH_RESULT_WINDOW_ITEMS = 160;

@Component({
  selector: 'app-global-search',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop palette-dialog-backdrop dialog-group" *ngIf="visible || isClosing" [class.dialog-group--closing]="isClosing" (mousedown)="close()" (animationend)="onAnimationEnd($event)">
      <section class="search palette-dialog-surface dialog-surface" role="dialog" aria-modal="true" aria-label="Global search"
        (mousedown)="$event.stopPropagation()"
        tabindex="-1">
        <header class="palette-dialog-header">
          <div class="palette-dialog-title">
            <strong>Global Search</strong>
            <span class="root palette-dialog-subtitle palette-dialog-mono" [title]="rootPath">{{ mode === 'search' ? rootPath : modeLabel }}</span>
          </div>
          <div class="header-meta">
          <span class="count palette-dialog-meta">
            {{ searching ? 'searching…' : resultCount + ' results' }}
            <ng-container *ngIf="!searching && resultCount">
              · {{ folderCount }} folders · {{ fileCount }} files
            </ng-container>
          </span>
          <button type="button" class="dialog-close-btn" aria-label="Close" title="Close" (click)="close()">×</button>
          </div>
        </header>

        <nav class="modes palette-dialog-tabbar" aria-label="Discovery source">
          <button type="button" [class.active]="mode === 'search'" (click)="setMode('search')">Search</button>
          <button type="button" [class.active]="mode === 'recent'" (click)="setMode('recent')">Recent</button>
          <button type="button" [class.active]="mode === 'collections'" (click)="setMode('collections')">Collections</button>
        </nav>

        <div class="query">
          <input
            #queryInput
            [(ngModel)]="queryText"
            (ngModelChange)="scheduleSearch()"
            [placeholder]="mode === 'search' ? 'Search this provider or folder' : mode === 'recent' ? 'Filter recent files' : 'Filter collections'"
            autocomplete="off"
            spellcheck="false"
            aria-label="Discovery query"
            aria-controls="global-search-results" />

          <ng-container *ngIf="mode === 'search'">
            <label title="Alt+R">
              <input type="checkbox" [(ngModel)]="recursive" (ngModelChange)="scheduleSearch()">
              Recursive
            </label>
            <label>Type
              <select [(ngModel)]="typeFilter" (ngModelChange)="scheduleSearch()" aria-label="Search type">
                <option value="all">All</option>
                <option value="files">Files</option>
                <option value="folders">Folders</option>
                <option value="archives">Archives</option>
              </select>
            </label>
            <label>Sort
              <select [(ngModel)]="sortMode" (ngModelChange)="applySort()" aria-label="Search result sort">
                <option value="relevance">Relevance</option>
                <option value="name">Name</option>
                <option value="path">Path</option>
                <option value="type">Type</option>
              </select>
            </label>
            <button type="button" class="save-search" [disabled]="!queryText.trim()" (click)="saveSearch()">Save…</button>
          </ng-container>
        </div>

        <div class="discovery-shortcuts" *ngIf="mode === 'search' && (savedSearches.length || historyEntries.length)">
          <div *ngIf="savedSearches.length">
            <span>Saved:</span>
            <button type="button" *ngFor="let saved of savedSearches.slice(0, 6)"
              [title]="saved.rootPath + ' · ' + saved.query"
              (click)="useSavedSearch(saved)">
              ★ {{ saved.name }}
            </button>
          </div>
          <div *ngIf="historyEntries.length">
            <span>History:</span>
            <button type="button" *ngFor="let recent of historyEntries.slice(0, 6)"
              [title]="recent.rootPath"
              (click)="useHistory(recent)">
              {{ recent.query }}
            </button>
          </div>
        </div>

        <div id="global-search-results" class="results palette-dialog-body palette-dialog-list" role="listbox" aria-label="Search results" [attr.aria-busy]="searching">
          <ng-container *ngIf="mode !== 'collections'; else collectionsList">
            <button
              *ngFor="let entry of renderedSearchResults; let i=index; trackBy:trackResult"
              type="button"
              class="result palette-dialog-item"
              role="option"
              [class.selected]="searchWindowStart + i === selectedIndex"
              [class.is-selected]="searchWindowStart + i === selectedIndex"
              [attr.aria-selected]="searchWindowStart + i === selectedIndex"
              [attr.aria-posinset]="searchWindowStart + i + 1"
              [attr.aria-setsize]="resultCount"
              (mouseenter)="selectedIndex=searchWindowStart + i"
              (click)="openEntry(entry)">
              <span class="icon">{{ icon(entry) }}</span>
              <span class="details">
                <strong>{{ entry.name }}</strong>
                <small>{{ entry.path }}</small>
              </span>
              <span class="kind">{{ kind(entry) }}</span>
            </button>

            <div class="empty palette-dialog-empty" *ngIf="!searching && resultCount === 0">
              {{ mode === 'recent'
                ? 'No recent files match.'
                : queryText.trim()
                  ? 'No matching resources.'
                  : 'Type a name or wildcard pattern.' }}
            </div>
          </ng-container>

          <ng-template #collectionsList>
            <button
              *ngFor="let panel of visibleCollections; let i=index; trackBy:trackPanel"
              type="button"
              class="result collection palette-dialog-item"
              role="option"
              [class.selected]="i === selectedIndex"
              [class.is-selected]="i === selectedIndex"
              [attr.aria-selected]="i === selectedIndex"
              (mouseenter)="selectedIndex=i"
              (click)="openCollection(panel)">
              <span class="icon">{{ panel.pinned ? '★' : '▤' }}</span>
              <span class="details">
                <strong>{{ panel.title }}</strong>
                <small>{{ collectionSummary(panel) }}</small>
              </span>
              <span class="kind">{{ panel.metadata.resultCount ?? panel.entries.length }} items</span>
            </button>
            <div class="empty palette-dialog-empty" *ngIf="visibleCollections.length === 0">No matching collections.</div>
          </ng-template>
        </div>

        <footer class="palette-dialog-footer">
          <span *ngIf="error" class="error" role="alert">{{ error }}</span>
          <span *ngIf="!error && searchProgress" class="progress">{{ searchProgress }}</span>
          <span *ngIf="searchWarnings.length" class="warning" [title]="searchWarnings.join('\n')">
            {{ searchWarnings.length }} warning{{ searchWarnings.length === 1 ? '' : 's' }}
          </span>
          <button
            *ngIf="mode !== 'collections'"
            type="button"
            class="open-panel palette-dialog-button"
            [disabled]="searching || resultCount === 0"
            (click)="openAsResultsPanel()">
            Open as Panel
          </button>
          <button
            *ngIf="mode === 'collections' && selectedCollection"
            type="button"
            class="open-panel palette-dialog-button"
            (click)="toggleSelectedCollectionPinned()">
            {{ selectedCollection.pinned ? 'Unpin' : 'Pin' }}
          </button>
          <button
            *ngIf="mode === 'search'"
            type="button"
            class="open-panel palette-dialog-button"
            [disabled]="searching || resultCount === 0"
            (click)="openAsResultsWorkspace()">
            Open Workspace
          </button>
          <span class="keys palette-dialog-footnote">↑ ↓ · Enter open · Ctrl+Enter folder · Ctrl+Shift+Enter panel · Alt+R recursive · Alt+S sort</span>
        </footer>
      </section>
    </div>
  `,
  styles: [`
    :host{position:fixed;inset:0;z-index:9480;pointer-events:none}
    .search{box-sizing:border-box;display:flex;flex-direction:column;width:min(820px,calc(100vw - 24px));height:min(660px,calc(100vh - 36px))}
    .header-meta{display:flex;align-items:center;gap:10px}
    .root,.count,small,.kind,.keys{color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}.root{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    
    .save-search{min-height:24px;padding:2px 8px;border:1px solid transparent;border-radius:3px;background:transparent;color:inherit;font:inherit;cursor:pointer}
    .modes button.active{border-color:var(--vsc-focus-border,var(--vsc-accent-blue));background:color-mix(in srgb,var(--vsc-accent-blue) 10%,transparent)}
    .save-search:hover{background:var(--vsc-list-hover-background)}
    .discovery-shortcuts{display:grid;gap:3px;padding:5px 10px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border-subtle));color:var(--vsc-description-foreground)}
    .discovery-shortcuts>div{display:flex;align-items:center;gap:5px;overflow:hidden}.discovery-shortcuts button{max-width:160px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;padding:2px 6px;border:1px solid var(--vsc-border);border-radius:10px;background:transparent;color:inherit;font:10px var(--vsc-font-family);cursor:pointer}
    .collection .details small{opacity:.8}
.query input[type=text],.query>input{flex:1;height:30px;padding:0 9px;border:1px solid var(--vsc-border,var(--vsc-border));border-radius:4px;outline:none;background:var(--vsc-input-background,var(--vsc-popover-background));color:inherit;font:inherit}.query input:focus{border-color:var(--vsc-focus-border,var(--vsc-accent-blue))}.query label{display:flex;gap:5px;align-items:center;white-space:nowrap}.query select{height:28px;border:1px solid var(--vsc-border,var(--vsc-border));border-radius:4px;background:var(--vsc-input-background,var(--vsc-popover-background));color:inherit;font:inherit}
    .recent-queries{display:flex;align-items:center;gap:5px;padding:5px 10px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border-subtle));overflow:hidden;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}.recent-queries button{max-width:150px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;padding:2px 6px;border:1px solid var(--vsc-border,var(--vsc-border));border-radius:10px;background:transparent;color:inherit;cursor:pointer;font:10px var(--vsc-font-family,system-ui,sans-serif)}.recent-queries button:hover{background:var(--vsc-list-hover-background,var(--vsc-list-hover-background))}
    .results{flex:1;min-height:0;overflow:auto}.result{display:grid;grid-template-columns:30px minmax(0,1fr) auto;gap:8px;align-items:center;width:100%;min-height:45px;padding:5px 8px}.result:hover,.result.selected{background:var(--vsc-list-hover-background,var(--vsc-list-hover-background))}.result.selected{border-color:color-mix(in srgb,var(--vsc-accent-blue,var(--vsc-accent-blue)) 55%,transparent)}.icon{text-align:center;font-size:16px}.details{min-width:0;display:grid}.details strong,.details small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.details small{font-family:var(--vsc-font-family-mono,ui-monospace,monospace)}.kind{font-size:10px}.empty{padding:40px;text-align:center;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    .keys{margin-left:auto}.progress{color:var(--vsc-description-foreground)}.warning{color:var(--vsc-warning)}.error{color:var(--vsc-error-foreground,var(--vsc-error));overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.open-panel:disabled{opacity:.45;cursor:default}
  `],
})
export class GlobalSearchComponent implements OnInit, OnDestroy {
  private readonly resultVirtualCollection = new VirtualCollection<FileInfo, string>({
    identity: entry => entry.path,
    itemExtent: SEARCH_RESULT_ROW_HEIGHT,
    threshold: SEARCH_RESULT_WINDOW_ITEMS,
    overscanRows: 80,
    guardRows: 24,
    minimumPrefetchItems: 512,
    prefetchMultiplier: 4,
    prefetchGuardRatio: 0.20,
  });

  @ViewChild('queryInput') queryInput?: ElementRef<HTMLInputElement>;
  @Output() openResult = new EventEmitter<FileInfo>();
  @Output() openContainingFolder = new EventEmitter<FileInfo>();
  @Output() openAsPanel = new EventEmitter<SearchResultsPanelRequest>();
  @Output() openAsWorkspace = new EventEmitter<SearchResultsPanelRequest>();
  @Output() openPanelPath = new EventEmitter<string>();

  visible = false;
  isClosing = false;
  rootPath = '';
  queryText = '';
  recursive = true;
  sortMode: SearchSort = 'relevance';
  typeFilter: DiscoverySearchType = 'all';
  mode: DiscoveryMode = 'search';
  historyEntries: ReturnType<DiscoveryHistoryService['history']> = [];
  savedSearches: ReturnType<DiscoveryHistoryService['saved']> = [];
  collections: readonly ResultsPanelDefinition[] = [];
  private queryHistoryIndex = -1;
  private readonly searchIndex = new SearchResultIndex();
  private resultWindowOffset = 0;
  private resultWindowEntries: readonly FileInfo[] = [];
  searchProgress = '';
  searchWarnings: readonly string[] = [];
  selectedIndex = 0;
  searching = false;
  error = '';

  private searchGeneration = 0;
  private searchAbort: AbortController | null = null;
  private searchTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(
    private readonly searchService: DiscoverySearchService,
    readonly history: DiscoveryHistoryService,
    private readonly recentFiles: RecentFilesService,
    private readonly resultPanels: ResultsPanelService,
    private readonly dialogs: ConfirmDialogService,
    private readonly input: InputEngineService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get resultCount(): number {
    return this.mode === 'collections'
      ? this.visibleCollections.length
      : this.searchIndex.resultCount;
  }

  get searchWindowStart(): number {
    if (this.mode === 'collections') return 0;
    this.refreshSearchVirtualWindow();
    return this.resultWindowOffset;
  }

  get renderedSearchResults(): readonly FileInfo[] {
    if (this.mode === 'collections') return [];
    this.refreshSearchVirtualWindow();
    return this.resultWindowEntries;
  }

  get modeLabel(): string {
    if (this.mode === 'recent') return 'Recent Files';
    if (this.mode === 'collections') return 'Collections';
    return 'Search';
  }

  get visibleCollections(): readonly ResultsPanelDefinition[] {
    const needle = this.queryText.trim().toLocaleLowerCase();
    return this.collections.filter(panel =>
      !needle ||
      panel.title.toLocaleLowerCase().includes(needle) ||
      this.collectionSummary(panel).toLocaleLowerCase().includes(needle)
    );
  }

  get selectedCollection(): ResultsPanelDefinition | null {
    return this.visibleCollections[this.selectedIndex] ?? null;
  }

  get selectedResult(): FileInfo | null {
    return this.mode === 'collections'
      ? null
      : this.searchIndex.at(this.selectedIndex);
  }

  get folderCount(): number {
    return this.searchIndex.folderCount;
  }

  get fileCount(): number {
    return this.searchIndex.fileCount;
  }

  get duplicateCandidateGroupCount(): number {
    return this.searchIndex.snapshot().duplicateCandidateGroups;
  }

  private refreshSearchVirtualWindow(): void {
    const count = this.searchIndex.resultCount;
    this.resultVirtualCollection.setLogicalCount(count);

    if (count === 0) {
      this.resultWindowOffset = 0;
      this.resultWindowEntries = [];
      this.resultVirtualCollection.populate(0, [], 0);
      return;
    }

    const index = Math.max(0, Math.min(count - 1, this.selectedIndex));
    this.resultVirtualCollection.revealIndex(
      index,
      Math.min(SEARCH_RESULT_WINDOW_ITEMS, count),
    );

    const prefetch = this.resultVirtualCollection.prefetch();
    const window = this.searchIndex.window(prefetch.offset, Math.max(1, prefetch.limit));
    this.resultVirtualCollection.populate(
      window.offset,
      window.entries,
      window.totalCount,
    );

    const renderWindow = this.resultVirtualCollection.window;
    this.resultWindowOffset = renderWindow.start;
    this.resultWindowEntries = this.resultVirtualCollection.renderedItems(renderWindow);
  }

  ngOnInit(): void {
    this.input.registerContext({
      id: CONTEXT_ID,
      kind: 'overlay',
      priority: 390, exclusive: true,
      parentId: 'global',
      description: 'Reassembly global search',
    });

    this.input.registerCommandDescriptor({id:COMMAND.close,title:'Close Global Search',category:'Search & Filter',execute:()=>this.close()},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.previous,title:'Previous Search Result',category:'Search & Filter',execute:()=>this.move(-1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.next,title:'Next Search Result',category:'Search & Filter',execute:()=>this.move(1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.open,title:'Open Search Result',category:'Search & Filter',execute:()=>{if(this.mode==='collections'){const panel=this.selectedCollection;if(panel)this.openCollection(panel);return;}const entry=this.selectedResult;if(entry)this.openEntry(entry)}},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openFolder,title:'Open Containing Folder',category:'Search & Filter',execute:()=>{const entry=this.selectedResult;if(entry)this.openFolder(entry)}},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openAsPanel,title:'Open Search Results as Panel',category:'Search & Filter',execute:()=>this.openAsResultsPanel(),isEnabled:()=>this.resultCount>0&&!this.searching},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.toggleRecursive,title:'Toggle Recursive Search',category:'Search & Filter',execute:()=>this.toggleRecursive()},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.cycleSort,title:'Cycle Search Sort',category:'Search & Filter',execute:()=>this.cycleSort()},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.previousQuery,title:'Previous Search Query',category:'Search & Filter',execute:()=>this.moveQueryHistory(-1),isEnabled:()=>this.historyEntries.length>0},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.nextQuery,title:'Next Search Query',category:'Search & Filter',execute:()=>this.moveQueryHistory(1),isEnabled:()=>this.historyEntries.length>0},OWNER_ID);

    for (const binding of BINDINGS) this.input.registerBinding(binding);
  }

  ngOnDestroy(): void {
    if (this.searchTimer) clearTimeout(this.searchTimer);
    this.input.deactivateContext(CONTEXT_ID);
    for (const binding of BINDINGS) this.input.unregisterBinding(binding.id);
    for (const id of Object.values(COMMAND)) {
      this.input.unregisterCommandDescriptor(id, OWNER_ID);
    }
  }

  open(rootPath: string): void {
    
    this.rootPath = rootPath;
    this.historyEntries = this.history.history();
    this.savedSearches = this.history.saved();
    this.collections = this.resultPanels.recent();
    this.queryHistoryIndex = -1;
    this.mode = 'search';
    this.queryText = '';
    this.searchIndex.reset(this.sortMode);
    this.selectedIndex = 0;
    this.error = '';
    this.isClosing = false;
    this.visible = true;
    this.input.activateContext(CONTEXT_ID);
    
    this.cdr.markForCheck();
  }

  close(): void {
    if (!this.visible || this.isClosing) return;
    this.isClosing = true;
    this.input.deactivateContext(CONTEXT_ID);
    this.searchGeneration += 1;
    this.searchAbort?.abort();
    this.searchAbort = null;
    if (this.searchTimer) {
      clearTimeout(this.searchTimer);
      this.searchTimer = null;
    }
    
    
    this.cdr.markForCheck();
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) return;
    this.visible = false;
    this.isClosing = false;
    this.cdr.markForCheck();
  }

  scheduleSearch(): void {
    if (this.searchTimer) clearTimeout(this.searchTimer);
    this.searchTimer = setTimeout(() => {
      this.searchTimer = null;
      void this.search();
    }, 180);
  }

  async search(): Promise<void> {
    const query = this.queryText.trim();
    const generation = ++this.searchGeneration;

    this.searchAbort?.abort();
    this.searchAbort = null;

    if (this.mode === 'collections') {
      this.searchIndex.reset(this.sortMode);
      this.collections = this.resultPanels.recent();
      this.selectedIndex = Math.min(
        this.selectedIndex,
        Math.max(0, this.visibleCollections.length - 1),
      );
      this.searching = false;
      this.error = '';
      this.cdr.markForCheck();
      return;
    }

    if (this.mode === 'recent') {
      this.searchIndex.reset(this.sortMode);
      const recent = this.recentFiles.search(query, 200);
      this.searchIndex.append(recent, recent.length);
      this.selectedIndex = Math.min(
        this.selectedIndex,
        Math.max(0, this.resultCount - 1),
      );
      this.searching = false;
      this.error = '';
      this.cdr.markForCheck();
      return;
    }

    if (!query || !this.rootPath) {
      this.searchIndex.reset(this.sortMode);
      this.searching = false;
      this.error = '';
      this.searchProgress = '';
      this.searchWarnings = [];
      this.selectedIndex = 0;
      this.cdr.markForCheck();
      return;
    }

    const controller = new AbortController();
    this.searchAbort = controller;
    this.searching = true;
    this.error = '';
    this.searchProgress = '';
    this.searchWarnings = [];
    this.searchIndex.reset(this.sortMode);
    this.resultWindowOffset = 0;
    this.resultWindowEntries = [];
    this.cdr.markForCheck();

    try {
      const result = await this.searchService.search(
        this.rootPath,
        query,
        {
          recursive: this.recursive,
          type: this.typeFilter,
          maxResults: 100_000,
          batchSize: 512,
          signal: controller.signal,
          onProgress: progress => {
            if (generation !== this.searchGeneration) return;
            this.searchProgress =
              `${progress.sourceId} · ${progress.visitedDirectories} folders · ${progress.matchedEntries} matches`;
            this.cdr.markForCheck();
          },
          onBatch: (batch, progress) => {
            if (
              controller.signal.aborted ||
              generation !== this.searchGeneration
            ) return;
            this.searchIndex.append(batch, progress.matchedEntries);
            this.selectedIndex = Math.min(
              this.selectedIndex,
              Math.max(0, this.resultCount - 1),
            );
            this.cdr.markForCheck();
          },
        },
      );

      if (generation !== this.searchGeneration) return;

      // Non-streaming providers may still return a terminal population.
      if (result.entries.length) {
        this.searchIndex.append(result.entries, result.entries.length);
      }
      this.searchWarnings = result.warnings;
      this.searchProgress = result.truncated
        ? `${result.sourceId} · result limit reached`
        : `${result.sourceId} · ${result.visitedDirectories} folders`;

      this.history.record(this.currentSearchSpec(), this.resultCount);
      this.historyEntries = this.history.history();

      this.selectedIndex = Math.min(
        this.selectedIndex,
        Math.max(0, this.resultCount - 1),
      );
    } catch (error) {
      if (generation !== this.searchGeneration) return;
      if (error instanceof DOMException && error.name === 'AbortError') return;

      this.searchIndex.reset(this.sortMode);
      this.selectedIndex = 0;
      this.error = error instanceof Error ? error.message : String(error);
    } finally {
      if (generation === this.searchGeneration) {
        this.searching = false;
        this.searchAbort = null;
        this.cdr.markForCheck();
      }
    }
  }

  openAsResultsPanel(): void {
    if (this.resultCount === 0) return;

    const query = this.queryText.trim();
    const panel = this.resultPanels.createFromIndex(
      this.mode === 'recent'
        ? (query ? `Recent: ${query}` : 'Recent Files')
        : `Search: ${query}`,
      this.searchIndex,
      this.mode === 'recent'
        ? {
            origin: 'recent-files',
            query,
            kind: 'recent-files',
            sort: this.sortMode,
          }
        : {
            origin: 'global-search',
            rootPath: this.rootPath,
            query,
            recursive: this.recursive,
            kind: this.recursive ? 'recursive-search' : 'search',
            sort: this.sortMode,
          },
    );

    this.openPanelPath.emit(panel.path);
    this.close();
  }

  openAsResultsWorkspace(): void {
    const query = this.queryText.trim();
    if (!query || this.resultCount === 0) return;

    // Workspace export is an explicit user action, so materialising the current
    // result set here is acceptable. Normal searching/rendering stays windowed.
    const entries = this.searchIndex.materialize();
    this.openAsWorkspace.emit({
      title: `Search: ${query}`,
      rootPath: this.rootPath,
      query,
      recursive: this.recursive,
      entries,
    });

    this.close();
  }

  toggleRecursive(): void {
    this.recursive = !this.recursive;
    this.scheduleSearch();
    this.cdr.markForCheck();
  }

  applySort(): void {
    this.searchIndex.setSort(this.sortMode);
    this.resultWindowEntries = [];
    this.selectedIndex = Math.min(
      this.selectedIndex,
      Math.max(0, this.resultCount - 1),
    );
    this.cdr.markForCheck();
  }

  cycleSort(): void {
    const modes: SearchSort[] = ['relevance', 'name', 'path', 'type'];
    const index = modes.indexOf(this.sortMode);
    this.sortMode = modes[(index + 1) % modes.length];
    this.searchIndex.setSort(this.sortMode);
    this.selectedIndex = Math.min(
      this.selectedIndex,
      Math.max(0, this.resultCount - 1),
    );
    this.cdr.markForCheck();
  }

  setMode(mode: DiscoveryMode): void {
    this.mode = mode;
    this.selectedIndex = 0;
    this.searchProgress = '';
    this.searchWarnings = [];
    this.error = '';

    if (mode === 'collections') {
      this.collections = this.resultPanels.recent();
    }

    void this.search();
    queueMicrotask(() => this.queryInput?.nativeElement.focus());
    this.cdr.markForCheck();
  }

  useSavedSearch(saved: SavedDiscoverySearch): void {
    this.mode = 'search';
    this.rootPath = saved.rootPath;
    this.queryText = saved.query;
    this.recursive = saved.recursive;
    this.typeFilter = saved.type;
    this.sortMode = saved.sort;
    this.selectedIndex = 0;
    this.scheduleSearch();
    queueMicrotask(() => this.queryInput?.nativeElement.focus());
  }

  useHistory(entry: DiscoverySearchSpec): void {
    this.mode = 'search';
    this.rootPath = entry.rootPath;
    this.queryText = entry.query;
    this.recursive = entry.recursive;
    this.typeFilter = entry.type;
    this.sortMode = entry.sort;
    this.selectedIndex = 0;
    this.scheduleSearch();
    queueMicrotask(() => this.queryInput?.nativeElement.focus());
  }

  async saveSearch(): Promise<void> {
    const spec = this.currentSearchSpec();
    if (!spec.query.trim()) return;

    const name = (await this.dialogs.prompt({
      title: 'Save Search',
      message: 'Name this search.',
      confirmText: 'Save',
      defaultValue: spec.query,
      historyKey: 'reassembly.discovery.savedSearch.name',
    }))?.trim();

    if (!name) return;

    this.history.save(name, spec);
    this.savedSearches = this.history.saved();
    this.cdr.markForCheck();
  }

  openCollection(panel: ResultsPanelDefinition): void {
    this.openPanelPath.emit(panel.path);
    this.close();
  }

  toggleSelectedCollectionPinned(): void {
    const panel = this.selectedCollection;
    if (!panel) return;

    this.resultPanels.setPinned(panel.path, !panel.pinned);
    this.collections = this.resultPanels.recent();
    this.cdr.markForCheck();
  }

  collectionSummary(panel: ResultsPanelDefinition): string {
    const parts = [
      panel.metadata.kind,
      panel.metadata.rootPath,
      panel.metadata.query,
      panel.metadata.pluginId ? `plugin:${panel.metadata.pluginId}` : '',
    ].filter(Boolean);
    return parts.join(' · ') || panel.path;
  }

  trackPanel(_index: number, panel: ResultsPanelDefinition): string {
    return panel.path;
  }

  private moveQueryHistory(delta: -1 | 1): void {
    if (!this.historyEntries.length) return;

    if (this.queryHistoryIndex < 0) {
      this.queryHistoryIndex =
        delta < 0 ? 0 : this.historyEntries.length - 1;
    } else {
      this.queryHistoryIndex =
        (this.queryHistoryIndex + delta + this.historyEntries.length) %
        this.historyEntries.length;
    }

    const entry = this.historyEntries[this.queryHistoryIndex];
    if (entry) this.useHistory(entry);
  }

  private currentSearchSpec(): DiscoverySearchSpec {
    return {
      rootPath: this.rootPath,
      query: this.queryText.trim(),
      recursive: this.recursive,
      type: this.typeFilter,
      sort: this.sortMode,
    };
  }

  openEntry(entry: FileInfo): void {
    this.openResult.emit(entry);
    this.close();
  }

  openFolder(entry: FileInfo): void {
    this.openContainingFolder.emit(entry);
    this.close();
  }

  trackResult(_index:number, entry:FileInfo):string { return entry.path; }

  icon(entry:FileInfo):string {
    if (entry.type === FileType.DIRECTORY) return '📁';
    if (entry.type === FileType.ARCHIVE) return '🗜️';
    return '📄';
  }

  kind(entry:FileInfo):string {
    if (entry.type === FileType.DIRECTORY) return 'Folder';
    if (entry.type === FileType.ARCHIVE) return 'Archive';
    return 'File';
  }

  private move(delta:-1|1):void {
    const count=this.mode === 'collections'
      ? this.visibleCollections.length
      : this.resultCount;
    if(!count)return;
    this.selectedIndex=(this.selectedIndex+delta+count)%count;
    this.cdr.markForCheck();
  }
}