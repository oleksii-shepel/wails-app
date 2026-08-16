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

import { ApiService } from '../services/api.service';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { FileInfo, FileType } from '../shared/protocol';
import { ReassemblyModalSurfaceDirective } from './reassembly-modal-surface.directive';

type SearchSort = 'relevance' | 'name' | 'path' | 'type';

export interface SearchResultsPanelRequest {
  readonly title: string;
  readonly rootPath: string;
  readonly query: string;
  readonly recursive: boolean;
  readonly entries: readonly FileInfo[];
}

const SEARCH_HISTORY_KEY = 'reassembly.globalSearch.queries.v1';
const SEARCH_HISTORY_LIMIT = 20;

const CONTEXT_ID = 'overlay.reassemblyGlobalSearch';
const OWNER_ID = 'overlay.reassemblyGlobalSearch';

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

@Component({
  selector: 'app-reassembly-global-search',
  standalone: true,
  imports: [CommonModule, FormsModule, ReassemblyModalSurfaceDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop" *ngIf="visible" (mousedown)="close()">
      <section class="search" role="dialog" aria-modal="true" aria-label="Global search"
        (mousedown)="$event.stopPropagation()"
        reassemblyModalSurface
        [reassemblyModalInitial]="\'.query > input\'"
        tabindex="-1">
        <header>
          <div>
            <strong>Search</strong>
            <span class="root" [title]="rootPath">{{ rootPath }}</span>
          </div>
          <span class="count">
            {{ searching ? 'searching…' : displayedResults.length + ' results' }}
            <ng-container *ngIf="!searching && results.length">
              · {{ folderCount }} folders · {{ fileCount }} files
            </ng-container>
          </span>
        </header>

        <div class="query">
          <input
            #queryInput
            [(ngModel)]="queryText"
            (ngModelChange)="scheduleSearch()"
            placeholder="Search recursively in active pane"
            autocomplete="off"
            spellcheck="false"
            aria-label="Search query" aria-controls="reassembly-global-search-results" />
          <label title="Alt+R">
            <input type="checkbox" [(ngModel)]="recursive" (ngModelChange)="scheduleSearch()">
            Recursive
          </label>
          <label>
            Sort
            <select [(ngModel)]="sortMode" aria-label="Search result sort">
              <option value="relevance">Relevance</option>
              <option value="name">Name</option>
              <option value="path">Path</option>
              <option value="type">Type</option>
            </select>
          </label>
        </div>

        <div class="recent-queries" *ngIf="recentQueries.length">
          <span>Recent:</span>
          <button
            type="button"
            *ngFor="let recent of recentQueries.slice(0, 6)"
            [title]="recent"
            (click)="useQuery(recent)">
            {{ recent }}
          </button>
        </div>

        <div id="reassembly-global-search-results" class="results" role="listbox" aria-label="Search results" [attr.aria-busy]="searching">
          <button
            *ngFor="let entry of displayedResults; let i=index; trackBy:trackResult"
            type="button"
            class="result"
            role="option"
            [class.selected]="i === selectedIndex"
            [attr.aria-selected]="i === selectedIndex"
            [attr.aria-posinset]="i + 1"
            [attr.aria-setsize]="displayedResults.length"
            (mouseenter)="selectedIndex=i"
            (click)="openEntry(entry)">
            <span class="icon">{{ icon(entry) }}</span>
            <span class="details">
              <strong>{{ entry.name }}</strong>
              <small>{{ entry.path }}</small>
            </span>
            <span class="kind">{{ kind(entry) }}</span>
          </button>

          <div class="empty" *ngIf="!searching && queryText.trim() && displayedResults.length === 0">
            No matching files.
          </div>
          <div class="empty" *ngIf="!queryText.trim()">
            Type a name or wildcard pattern.
          </div>
        </div>

        <footer>
          <span *ngIf="error" class="error" role="alert">{{ error }}</span>
          <button
            type="button"
            class="open-panel"
            [disabled]="searching || displayedResults.length === 0"
            (click)="openAsResultsPanel()">
            Open as Panel
          </button>
          <button
            type="button"
            class="open-panel"
            [disabled]="searching || displayedResults.length === 0"
            (click)="openAsResultsWorkspace()">
            Open Workspace
          </button>
          <span class="keys">↑ ↓ · Enter open · Ctrl+Enter folder · Ctrl+Shift+Enter panel · Alt+R recursive · Alt+S sort · Esc</span>
        </footer>
      </section>
    </div>
  `,
  styles: [`
    :host{position:fixed;inset:0;z-index:9480;pointer-events:none}
    .backdrop{position:fixed;inset:0;display:grid;place-items:start center;padding-top:min(8vh,72px);background: var(--vsc-scrim-soft);pointer-events:auto}
    .search{box-sizing:border-box;display:flex;flex-direction:column;width:min(820px,calc(100vw - 24px));height:min(660px,calc(100vh - 36px));overflow:hidden;border:1px solid var(--vsc-border,var(--vsc-border));border-radius:8px;background:var(--vsc-menu-background,var(--vsc-panel-background));color:var(--vsc-foreground,var(--vsc-foreground));box-shadow:0 18px 60px var(--vsc-scrim-strong);font:12px/1.4 var(--vsc-font-family,system-ui,sans-serif)}
    header,footer,.query{display:flex;align-items:center;gap:10px;padding:9px 10px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border-subtle))}header{justify-content:space-between}header>div{display:flex;align-items:baseline;gap:8px;min-width:0}.root,.count,small,.kind,.keys{color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}.root{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-family:var(--vsc-font-family-mono,ui-monospace,monospace)}
    .query input[type=text],.query>input{flex:1;height:30px;padding:0 9px;border:1px solid var(--vsc-border,var(--vsc-border));border-radius:4px;outline:none;background:var(--vsc-input-background,var(--vsc-popover-background));color:inherit;font:inherit}.query input:focus{border-color:var(--vsc-focus-border,var(--vsc-accent-blue))}.query label{display:flex;gap:5px;align-items:center;white-space:nowrap}.query select{height:28px;border:1px solid var(--vsc-border,var(--vsc-border));border-radius:4px;background:var(--vsc-input-background,var(--vsc-popover-background));color:inherit;font:inherit}
    .recent-queries{display:flex;align-items:center;gap:5px;padding:5px 10px;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border-subtle));overflow:hidden;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}.recent-queries button{max-width:150px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;padding:2px 6px;border:1px solid var(--vsc-border,var(--vsc-border));border-radius:10px;background:transparent;color:inherit;cursor:pointer;font:10px var(--vsc-font-family,system-ui,sans-serif)}.recent-queries button:hover{background:var(--vsc-list-hover-background,var(--vsc-list-hover-background))}
    .results{flex:1;min-height:0;overflow:auto;padding:5px}.result{display:grid;grid-template-columns:30px minmax(0,1fr) auto;gap:8px;align-items:center;width:100%;min-height:45px;padding:5px 8px;border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;text-align:left;cursor:pointer;font:inherit}.result:hover,.result.selected{background:var(--vsc-list-hover-background,var(--vsc-list-hover-background))}.result.selected{border-color:color-mix(in srgb,var(--vsc-accent-blue,var(--vsc-accent-blue)) 55%,transparent)}.icon{text-align:center;font-size:16px}.details{min-width:0;display:grid}.details strong,.details small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.details small{font-family:var(--vsc-font-family-mono,ui-monospace,monospace)}.kind{font-size:10px}.empty{padding:40px;text-align:center;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    footer{border-top:1px solid var(--vsc-border-subtle,var(--vsc-border-subtle));border-bottom:0}.keys{margin-left:auto}.error{color:var(--vsc-error-foreground,var(--vsc-error));overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.open-panel{height:26px;padding:0 9px;border:1px solid var(--vsc-border,var(--vsc-border));border-radius:4px;background:var(--vsc-button-secondary-background,var(--vsc-control-background));color:inherit;cursor:pointer;font:inherit}.open-panel:disabled{opacity:.45;cursor:default}
  `],
})
export class ReassemblyGlobalSearchComponent implements OnInit, OnDestroy {
  @ViewChild('queryInput') queryInput?: ElementRef<HTMLInputElement>;
  @Output() openResult = new EventEmitter<FileInfo>();
  @Output() openContainingFolder = new EventEmitter<FileInfo>();
  @Output() openAsPanel = new EventEmitter<SearchResultsPanelRequest>();
  @Output() openAsWorkspace = new EventEmitter<SearchResultsPanelRequest>();

  visible = false;
  rootPath = '';
  queryText = '';
  recursive = true;
  sortMode: SearchSort = 'relevance';
  recentQueries: string[] = [];
  private queryHistoryIndex = -1;
  results: FileInfo[] = [];
  selectedIndex = 0;
  searching = false;
  error = '';

  private searchGeneration = 0;
  private searchTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(
    private readonly api: ApiService,
    private readonly input: InputEngineService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get displayedResults(): FileInfo[] {
    const results = [...this.results];

    if (this.sortMode === 'name') {
      return results.sort((a,b) =>
        (a.name ?? '').localeCompare(b.name ?? '', undefined, {
          numeric: true,
          sensitivity: 'base',
        }),
      );
    }

    if (this.sortMode === 'path') {
      return results.sort((a,b) =>
        (a.path ?? '').localeCompare(b.path ?? '', undefined, {
          numeric: true,
          sensitivity: 'base',
        }),
      );
    }

    if (this.sortMode === 'type') {
      return results.sort((a,b) =>
        this.kind(a).localeCompare(this.kind(b)) ||
        (a.name ?? '').localeCompare(b.name ?? ''),
      );
    }

    return results;
  }

  get selectedResult(): FileInfo | null {
    return this.displayedResults[this.selectedIndex] ?? null;
  }

  get folderCount(): number {
    return this.results.filter(entry => entry.type === FileType.DIRECTORY).length;
  }

  get fileCount(): number {
    return this.results.length - this.folderCount;
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
    this.input.registerCommandDescriptor({id:COMMAND.open,title:'Open Search Result',category:'Search & Filter',execute:()=>{const entry=this.selectedResult;if(entry)this.openEntry(entry)}},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openFolder,title:'Open Containing Folder',category:'Search & Filter',execute:()=>{const entry=this.selectedResult;if(entry)this.openFolder(entry)}},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openAsPanel,title:'Open Search Results as Panel',category:'Search & Filter',execute:()=>this.openAsResultsPanel(),isEnabled:()=>this.displayedResults.length>0&&!this.searching},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.toggleRecursive,title:'Toggle Recursive Search',category:'Search & Filter',execute:()=>this.toggleRecursive()},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.cycleSort,title:'Cycle Search Sort',category:'Search & Filter',execute:()=>this.cycleSort()},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.previousQuery,title:'Previous Search Query',category:'Search & Filter',execute:()=>this.moveQueryHistory(-1),isEnabled:()=>this.recentQueries.length>0},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.nextQuery,title:'Next Search Query',category:'Search & Filter',execute:()=>this.moveQueryHistory(1),isEnabled:()=>this.recentQueries.length>0},OWNER_ID);

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
    this.recentQueries = this.loadRecentQueries();
    this.queryHistoryIndex = -1;
    this.queryText = '';
    this.results = [];
    this.selectedIndex = 0;
    this.error = '';
    this.visible = true;
    this.input.activateContext(CONTEXT_ID);
    
    this.cdr.markForCheck();
  }

  close(): void {
    if (!this.visible) return;
    this.visible = false;
    this.input.deactivateContext(CONTEXT_ID);
    this.searchGeneration += 1;
    if (this.searchTimer) {
      clearTimeout(this.searchTimer);
      this.searchTimer = null;
    }
    
    
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

    if (!query || !this.rootPath) {
      this.results = [];
      this.searching = false;
      this.error = '';
      this.selectedIndex = 0;
      this.cdr.markForCheck();
      return;
    }

    this.searching = true;
    this.error = '';
    this.cdr.markForCheck();

    try {
      const result = await this.api.search(
        this.rootPath,
        query,
        this.recursive,
      );
      if (generation !== this.searchGeneration) return;

      this.results = [...(result.matches ?? [])];
      this.rememberQuery(query);
      this.selectedIndex = Math.min(
        this.selectedIndex,
        Math.max(0, this.displayedResults.length - 1),
      );
    } catch (error) {
      if (generation !== this.searchGeneration) return;
      this.results = [];
      this.selectedIndex = 0;
      this.error = error instanceof Error ? error.message : String(error);
    } finally {
      if (generation === this.searchGeneration) {
        this.searching = false;
        this.cdr.markForCheck();
      }
    }
  }

  openAsResultsPanel(): void {
    const query = this.queryText.trim();
    const entries = this.displayedResults;
    if (!query || entries.length === 0) return;

    this.openAsPanel.emit({
      title: `Search: ${query}`,
      rootPath: this.rootPath,
      query,
      recursive: this.recursive,
      entries: entries.map(entry => ({ ...entry })),
    });

    this.close();
  }

  openAsResultsWorkspace(): void {
    const query = this.queryText.trim();
    const entries = this.displayedResults;
    if (!query || entries.length === 0) return;

    this.openAsWorkspace.emit({
      title: `Search: ${query}`,
      rootPath: this.rootPath,
      query,
      recursive: this.recursive,
      entries: entries.map(entry => ({ ...entry })),
    });

    this.close();
  }


  toggleRecursive(): void {
    this.recursive = !this.recursive;
    this.scheduleSearch();
    this.cdr.markForCheck();
  }

  cycleSort(): void {
    const modes: SearchSort[] = ['relevance', 'name', 'path', 'type'];
    const index = modes.indexOf(this.sortMode);
    this.sortMode = modes[(index + 1) % modes.length];
    this.selectedIndex = Math.min(
      this.selectedIndex,
      Math.max(0, this.displayedResults.length - 1),
    );
    this.cdr.markForCheck();
  }

  useQuery(query: string): void {
    this.queryText = query;
    this.queryHistoryIndex = this.recentQueries.indexOf(query);
    this.scheduleSearch();
    queueMicrotask(() => this.queryInput?.nativeElement.focus());
    this.cdr.markForCheck();
  }

  private moveQueryHistory(delta: -1 | 1): void {
    if (this.recentQueries.length === 0) return;

    if (this.queryHistoryIndex < 0) {
      this.queryHistoryIndex = delta < 0 ? 0 : this.recentQueries.length - 1;
    } else {
      this.queryHistoryIndex =
        (this.queryHistoryIndex + delta + this.recentQueries.length) %
        this.recentQueries.length;
    }

    this.queryText = this.recentQueries[this.queryHistoryIndex] ?? '';
    this.scheduleSearch();
    this.cdr.markForCheck();
  }

  private rememberQuery(query: string): void {
    const normalized = query.trim();
    if (!normalized) return;

    this.recentQueries = [
      normalized,
      ...this.recentQueries.filter(item =>
        item.toLocaleLowerCase() !== normalized.toLocaleLowerCase(),
      ),
    ].slice(0, SEARCH_HISTORY_LIMIT);

    try {
      localStorage.setItem(
        SEARCH_HISTORY_KEY,
        JSON.stringify(this.recentQueries),
      );
    } catch {
      // Search stays functional when storage is unavailable.
    }
  }

  private loadRecentQueries(): string[] {
    try {
      const parsed = JSON.parse(
        localStorage.getItem(SEARCH_HISTORY_KEY) ?? '[]',
      );

      return Array.isArray(parsed)
        ? parsed
            .filter((value): value is string =>
              typeof value === 'string' && !!value.trim(),
            )
            .slice(0, SEARCH_HISTORY_LIMIT)
        : [];
    } catch {
      return [];
    }
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
    const count=this.results.length;
    if(!count)return;
    this.selectedIndex=(this.selectedIndex+delta+count)%count;
    this.cdr.markForCheck();
  }
}