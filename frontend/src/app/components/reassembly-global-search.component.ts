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

const CONTEXT_ID = 'overlay.reassemblyGlobalSearch';
const OWNER_ID = 'overlay.reassemblyGlobalSearch';

const COMMAND = {
  close: 'reassembly.globalSearch.close',
  previous: 'reassembly.globalSearch.previous',
  next: 'reassembly.globalSearch.next',
  open: 'reassembly.globalSearch.open',
  openFolder: 'reassembly.globalSearch.openFolder',
} as const;

const BINDINGS: readonly InputBindingDefinition[] = [
  { id:'reassembly.globalSearch.escape', commandId:COMMAND.close, contextId:CONTEXT_ID, source:'default', description:'Close global search', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'escape'}]}}},
  { id:'reassembly.globalSearch.up', commandId:COMMAND.previous, contextId:CONTEXT_ID, source:'default', description:'Previous search result', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'arrowup'}]}}},
  { id:'reassembly.globalSearch.down', commandId:COMMAND.next, contextId:CONTEXT_ID, source:'default', description:'Next search result', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'arrowdown'}]}}},
  { id:'reassembly.globalSearch.enter', commandId:COMMAND.open, contextId:CONTEXT_ID, source:'default', description:'Open search result', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'enter'}]}}},
  { id:'reassembly.globalSearch.ctrlEnter', commandId:COMMAND.openFolder, contextId:CONTEXT_ID, source:'default', description:'Open containing folder', category:'Search & Filter', trigger:{device:'keyboard', chord:{steps:[{key:'enter',modifiers:['ctrl']}]}}},
];

@Component({
  selector: 'app-reassembly-global-search',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop" *ngIf="visible" (mousedown)="close()">
      <section class="search" role="dialog" aria-modal="true" aria-label="Global search"
        (mousedown)="$event.stopPropagation()">
        <header>
          <div>
            <strong>Search</strong>
            <span class="root" [title]="rootPath">{{ rootPath }}</span>
          </div>
          <span class="count">{{ searching ? 'searching…' : results.length + ' results' }}</span>
        </header>

        <div class="query">
          <input
            #queryInput
            [(ngModel)]="queryText"
            (ngModelChange)="scheduleSearch()"
            placeholder="Search recursively in active pane"
            autocomplete="off"
            spellcheck="false"
            aria-label="Search query" />
          <label><input type="checkbox" [(ngModel)]="recursive" (ngModelChange)="scheduleSearch()"> Recursive</label>
        </div>

        <div class="results" role="listbox">
          <button
            *ngFor="let entry of results; let i=index; trackBy:trackResult"
            type="button"
            class="result"
            role="option"
            [class.selected]="i === selectedIndex"
            [attr.aria-selected]="i === selectedIndex"
            (mouseenter)="selectedIndex=i"
            (click)="openEntry(entry)">
            <span class="icon">{{ icon(entry) }}</span>
            <span class="details">
              <strong>{{ entry.name }}</strong>
              <small>{{ entry.path }}</small>
            </span>
            <span class="kind">{{ kind(entry) }}</span>
          </button>

          <div class="empty" *ngIf="!searching && queryText.trim() && results.length === 0">
            No matching files.
          </div>
          <div class="empty" *ngIf="!queryText.trim()">
            Type a name or wildcard pattern.
          </div>
        </div>

        <footer>
          <span *ngIf="error" class="error">{{ error }}</span>
          <span class="keys">↑ ↓ · Enter open · Ctrl+Enter folder · Esc</span>
        </footer>
      </section>
    </div>
  `,
  styles: [`
    :host{position:fixed;inset:0;z-index:9480;pointer-events:none}
    .backdrop{position:fixed;inset:0;display:grid;place-items:start center;padding-top:min(8vh,72px);background:rgba(0,0,0,.25);pointer-events:auto}
    .search{box-sizing:border-box;display:flex;flex-direction:column;width:min(820px,calc(100vw - 24px));height:min(660px,calc(100vh - 36px));overflow:hidden;border:1px solid var(--vsc-border,#454545);border-radius:8px;background:var(--vsc-menu-background,#252526);color:var(--vsc-foreground,#ccc);box-shadow:0 18px 60px rgba(0,0,0,.5);font:12px/1.4 var(--vsc-font-family,system-ui,sans-serif)}
    header,footer,.query{display:flex;align-items:center;gap:10px;padding:9px 10px;border-bottom:1px solid var(--vsc-border-subtle,#2b2b2b)}header{justify-content:space-between}header>div{display:flex;align-items:baseline;gap:8px;min-width:0}.root,.count,small,.kind,.keys{color:var(--vsc-description-foreground,#969696)}.root{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-family:var(--vsc-font-family-mono,ui-monospace,monospace)}
    .query input[type=text],.query>input{flex:1;height:30px;padding:0 9px;border:1px solid var(--vsc-border,#454545);border-radius:4px;outline:none;background:var(--vsc-input-background,#181818);color:inherit;font:inherit}.query input:focus{border-color:var(--vsc-focus-border,#007fd4)}.query label{display:flex;gap:5px;align-items:center;white-space:nowrap}
    .results{flex:1;min-height:0;overflow:auto;padding:5px}.result{display:grid;grid-template-columns:30px minmax(0,1fr) auto;gap:8px;align-items:center;width:100%;min-height:45px;padding:5px 8px;border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;text-align:left;cursor:pointer;font:inherit}.result:hover,.result.selected{background:var(--vsc-list-hover-background,#2a2d2e)}.result.selected{border-color:color-mix(in srgb,var(--vsc-accent-blue,#007acc) 55%,transparent)}.icon{text-align:center;font-size:16px}.details{min-width:0;display:grid}.details strong,.details small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.details small{font-family:var(--vsc-font-family-mono,ui-monospace,monospace)}.kind{font-size:10px}.empty{padding:40px;text-align:center;color:var(--vsc-description-foreground,#969696)}
    footer{border-top:1px solid var(--vsc-border-subtle,#2b2b2b);border-bottom:0}.keys{margin-left:auto}.error{color:var(--vsc-error-foreground,#f48771);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  `],
})
export class ReassemblyGlobalSearchComponent implements OnInit, OnDestroy {
  @ViewChild('queryInput') queryInput?: ElementRef<HTMLInputElement>;
  @Output() openResult = new EventEmitter<FileInfo>();
  @Output() openContainingFolder = new EventEmitter<FileInfo>();

  visible = false;
  rootPath = '';
  queryText = '';
  recursive = true;
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

  get selectedResult(): FileInfo | null {
    return this.results[this.selectedIndex] ?? null;
  }

  ngOnInit(): void {
    this.input.registerContext({
      id: CONTEXT_ID,
      kind: 'overlay',
      priority: 390,
      parentId: 'global',
      description: 'Reassembly global search',
    });

    this.input.registerCommandDescriptor({id:COMMAND.close,title:'Close Global Search',category:'Search & Filter',execute:()=>this.close()},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.previous,title:'Previous Search Result',category:'Search & Filter',execute:()=>this.move(-1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.next,title:'Next Search Result',category:'Search & Filter',execute:()=>this.move(1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.open,title:'Open Search Result',category:'Search & Filter',execute:()=>{const entry=this.selectedResult;if(entry)this.openEntry(entry)}},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openFolder,title:'Open Containing Folder',category:'Search & Filter',execute:()=>{const entry=this.selectedResult;if(entry)this.openFolder(entry)}},OWNER_ID);

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
    this.queryText = '';
    this.results = [];
    this.selectedIndex = 0;
    this.error = '';
    this.visible = true;
    this.input.activateContext(CONTEXT_ID);
    queueMicrotask(() => this.queryInput?.nativeElement.focus());
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
      this.selectedIndex = Math.min(
        this.selectedIndex,
        Math.max(0, this.results.length - 1),
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
