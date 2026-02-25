// file-pane.component.ts
import { Component, Input, Output, EventEmitter, OnInit, OnChanges, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TabBarComponent } from './tab-bar.component';
import { AddressBarComponent } from './address-bar.component';
import { FindWidgetComponent, FindWidgetState } from './filter-widget.component';
import { ActionBarComponent } from './action-bar.component';
import { FileInfo, FileType } from '../shared/protocol';
import { TabInfo, WorkspaceConfig, FilterQuery } from '../shared/protocol-enhanced';
import { ApiService } from '../services/api.service';
import { WorkspaceService } from '../services/workspace.service';
import { FilterService } from '../services/filter.service';
import { PathHistoryService } from '../services/path-history.service';
import {
  ArchiveService, ArchiveFrame, isArchive, buildArchiveDisplayPath, buildArchiveTabTitle
} from '../services/archive.service';
import { parentPath, normalizePath } from './address-bar.utils';
import { TreeNode } from './file-tree.component';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { FileViewContainerComponent, ViewLayout } from './file-view-container.component';

declare const window: any;

@Component({
  selector: 'app-file-pane',
  standalone: true,
  imports: [
    CommonModule,
    TabBarComponent,
    AddressBarComponent,
    FindWidgetComponent,
    ActionBarComponent,
    FileViewContainerComponent
  ],
  template: `
    <div class="file-pane" [class.file-pane--archive]="isInArchive()">
      <app-tab-bar
        [tabs]="tabs"
        [activeTabId]="currentTabId"
        [inArchive]="isInArchive()"
        (tabChange)="onTabChange($event)"
        (tabClose)="onTabClose($event)"
        (tabPin)="onTabPin($event)"
        (newTab)="onNewTab()">
      </app-tab-bar>

      <app-address-bar
        *ngIf="pathReady"
        [initialPath]="currentPath"
        [paneId]="paneId"
        (pathChange)="onPathChange($event)"
        (navigateUpClicked)="onNavigateUp()"
        (refreshClicked)="onRefresh()"
        (archiveSegmentClick)="onArchiveSegmentClick($event)">
      </app-address-bar>

      <app-action-bar
        [selectedCount]="selectedFiles.size"
        [panePosition]="paneId"
        [currentView]="currentView"
        [currentSort]="sortField"
        [sortDirection]="sortDirection"
        (viewChange)="onViewChange($event)"
        (sortChange)="onSortChange($event)"
        (createFolder)="onCreateFolder()"
        (deleteSelected)="onDeleteSelected()"
        (copyToOther)="onCopyToOther()"
        (moveToOther)="onMoveToOther()">
      </app-action-bar>

      <!-- File list + find widget overlay share the same relative container -->
      <div class="file-pane__list-host">
        <app-find-widget
          #findWidget
          (stateChange)="onFindStateChange($event)"
          (nextMatch)="onFindNext()"
          (prevMatch)="onFindPrev()">
        </app-find-widget>

        <app-file-view-container
          [entries]="filteredEntries"
          [currentPath]="currentPath"
          [showHidden]="showHidden"
          [selectedFiles]="selectedFiles"
          [loading]="loading"
          [error]="error"
          [currentView]="currentView"
          [sortField]="sortField"
          [sortDirection]="sortDirection"
          (viewChange)="onViewChange($event)"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)"
          (sortChange)="onSortChange($event)"
          (contextMenu)="onContextMenu($event)"
          (nodeDoubleClick)="onNodeDoubleClick($event)">
        </app-file-view-container>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    .file-pane {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      background: var(--vsc-editor-background);
      overflow: hidden;
    }

    .file-pane--archive {
      background: color-mix(in srgb, var(--vsc-accent-orange) 4%, var(--vsc-editor-background));
    }

    app-tab-bar,
    app-address-bar,
    app-action-bar {
      display: block;
      flex-shrink: 0;
    }

    /* The list host is the anchor for the find widget overlay */
    .file-pane__list-host {
      position: relative;
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
    }

    app-find-widget {
      /* find-widget positions itself absolute inside .file-pane__list-host */
    }

    app-file-view-container {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
    }
  `]
})
export class FilePaneComponent implements OnInit, OnChanges {
  @ViewChild(AddressBarComponent) addressBar?: AddressBarComponent;
  @ViewChild(FileViewContainerComponent) fileViewContainer?: FileViewContainerComponent;
  @ViewChild('findWidget')        findWidget?: FindWidgetComponent;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() workspace: WorkspaceConfig | null = null;
  @Input() showHidden = false;

  @Output() pathChange   = new EventEmitter<string>();
  @Output() copyToOther  = new EventEmitter<string[]>();
  @Output() moveToOther  = new EventEmitter<string[]>();

  currentPath   = '';
  fsParentDir = '';
  currentTabId  = '';
  pathReady     = false;
  tabs: TabInfo[] = [];
  entries: FileInfo[] = [];
  filteredEntries: FileInfo[] = [];
  selectedFiles: Set<string> = new Set();
  loading = false;
  error: string | null = null;
  filterQuery: FilterQuery = { text: '', criteria: [] };

  // View system state
  currentView: ViewLayout = 'list';
  sortField: string = 'name';
  sortDirection: 'asc' | 'desc' = 'asc';

  archiveStack: ArchiveFrame[] = [];

  private tabPaths = new Map<string, string>();

  FileType = FileType;

  constructor(
    private apiService: ApiService,
    private workspaceService: WorkspaceService,
    private filterService: FilterService,
    private pathHistoryService: PathHistoryService,
    private archiveService: ArchiveService,
  ) {}

  ngOnInit(): void  { 
    this.loadViewPreference();
    this.loadPaneState(); 
  }
  
  ngOnChanges(): void { 
    if (this.workspace) {
      this.loadViewPreference();
      this.loadPaneState(); 
    }
  }

  get entriesForAnalysis(): FileInfo[] {
    return this.entries.filter(e => e.path !== PARENT_ENTRY_PATH);
  }

  // ── View System Methods ───────────────────────────────────────────────────

  private loadViewPreference(): void {
    const savedView = localStorage.getItem('fileManager_viewMode') as ViewLayout;
    if (savedView && ['list', 'grid', 'tree', 'details'].includes(savedView)) {
      this.currentView = savedView;
    }
    
    const savedSortField = localStorage.getItem('fileManager_sortField');
    const savedSortDir = localStorage.getItem('fileManager_sortDirection') as 'asc' | 'desc';
    if (savedSortField) this.sortField = savedSortField;
    if (savedSortDir) this.sortDirection = savedSortDir;
  }

  private saveViewPreference(): void {
    localStorage.setItem('fileManager_viewMode', this.currentView);
    localStorage.setItem('fileManager_sortField', this.sortField);
    localStorage.setItem('fileManager_sortDirection', this.sortDirection);
  }

  async onViewChange(view: ViewLayout): Promise<void> {
    const previousView = this.currentView;
    this.currentView = view;
    this.saveViewPreference();

    // Tree navigation keeps currentPath in sync without replacing entries.
    // When leaving tree mode, load the current folder so list/grid/details
    // always render the active directory contents.
    if (previousView === 'tree' && view !== 'tree') {
      if (this.archiveService.isArchiveVirtualPath(this.currentPath)) {
        if (this.isInArchive()) {
          const currentFrame = this.archiveStack[this.archiveStack.length - 1];
          if (currentFrame) {
            this.renderArchiveFrame(currentFrame);
          } else {
            await this.restoreArchiveFromPath(this.currentPath);
          }
        } else {
          await this.restoreArchiveFromPath(this.currentPath);
        }
      } else {
        await this.loadDirectory(this.currentPath);
      }
      return;
    }

    // Refresh entries when switching to ensure proper sorting/filtering
    this.applyFilter();
  }

  onSortChange(sort: { field: string; direction: 'asc' | 'desc' }): void {
    this.sortField = sort.field;
    this.sortDirection = sort.direction;
    this.saveViewPreference();
    this.applyFilter();
  }

  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void {
    // Handle context menu - can integrate with existing context menu system
    console.log('Context menu for:', event.entry.name);
  }

  onNodeDoubleClick(entry: FileInfo): void {
    // Handle file open - for non-directory files
    if (entry.type !== FileType.DIRECTORY) {
      console.log('Open file:', entry.name);
      // Implement file opening logic here
    }
  }

  // ── Virtual ".." entry helpers ────────────────────────────────────────────

  /** Returns true when the current path is a filesystem root (e.g. "/" or "C:/"). */
  private isAtFsRoot(path: string): boolean {
    const p = path.replace(/\\/g, '/');
    return p === '/' || /^[A-Za-z]:\/?\s*$/.test(p);
  }

  /** Creates the virtual parent-directory entry. */
  private makeParentEntry(): FileInfo {
    return {
      name: '..',
      path: PARENT_ENTRY_PATH,
      type: FileType.DIRECTORY,
      size: 0,
      modified: 0,
    } as FileInfo;
  }

  /**
   * Prepend the virtual ".." entry to a raw entry list when appropriate.
   * – Always prepended inside archives (so the user can navigate out).
   * – Prepended for normal directories unless we are at the filesystem root.
   */
  private withParentEntry(raw: FileInfo[], inArchive: boolean, fsPath: string): FileInfo[] {
    if (raw.some(e => e.path === PARENT_ENTRY_PATH)) {
      return raw;
    }
    if (inArchive || !this.isAtFsRoot(fsPath)) {
      return [this.makeParentEntry(), ...raw];
    }
    return raw;
  }

  // ── Pane state ────────────────────────────────────────────────────────────

  private async loadPaneState(): Promise<void> {
    if (!this.workspace) return;
    this.tabs = this.workspaceService.getTabs(this.paneId);
    const activeTab  = this.workspaceService.getActiveTab(this.paneId);
    const savedPath  = activeTab?.path ?? '';

    const allPaths   = this.workspaceService.getAllTabPaths();
    const windowsRoot = allPaths.find(p => /^[A-Za-z]:[/\\]/.test(p));
    const platformRoot = windowsRoot
      ? windowsRoot.replace(/\\/g, '/').replace(/^([A-Za-z]:).*$/, '$1/')
      : '/';

    const isWindows   = platformRoot !== '/';
    const pathIsStale = !savedPath || (isWindows && savedPath === '/');
    const pathToLoad  = pathIsStale ? platformRoot : savedPath;

    this.currentPath = pathToLoad;
    this.pathReady   = true;
    if (activeTab) this.currentTabId = activeTab.id;

    this.clearArchiveState();

    if (this.archiveService.isArchiveVirtualPath(pathToLoad)) {
      await this.restoreArchiveFromPath(pathToLoad);
    } else {
      await this.loadDirectory(pathToLoad);
    }
  }

  private clearArchiveState(): void {
    this.archiveStack = [];
    this.fsParentDir  = '';
  }

  private async loadDirectory(path: string): Promise<void> {
    this.exitArchive();
    this.clearArchiveState();
    this.loading = true;
    this.error   = null;
    try {
      const listing       = await this.apiService.listDirectory(path, this.showHidden);
      this.currentPath    = listing.path;
      this.fsParentDir    = listing.path;
      this.entries        = this.withParentEntry(listing.entries, false, listing.path);
      this.selectedFiles.clear();
      this.applyFilter();
      this.tabPaths.set(this.currentTabId, listing.path);
      this.workspaceService.updateTabPath(this.paneId, this.currentTabId, listing.path);
      this.pathHistoryService.addPath(listing.path, this.paneId);
      this.pathChange.emit(listing.path);

      try {
        if (window.go?.main?.App?.SetPaneDirectory) {
          await window.go.main.App.SetPaneDirectory(this.paneId, listing.path);
        }
      } catch { /* optional */ }
      // Focus the tree (if visible) to the newly-loaded path so address-bar and tree stay in sync
      setTimeout(() => {
        try { this.fileViewContainer?.treeComponent?.focusNode(listing.path); } catch { /* best-effort */ }
      }, 0);
    } catch (error: any) {
      this.error = error.message || 'Failed to load directory';
    } finally {
      this.loading = false;
    }
  }

  // ── Tab management ────────────────────────────────────────────────────────

  async onTabChange(tab: TabInfo): Promise<void> {
    if (this.currentTabId && this.currentPath) {
      if (this.isInArchive()) {
        this.workspaceService.saveArchiveState(this.paneId, this.currentTabId, {
          path: this.currentPath, archiveStack: this.archiveStack,
          fsDirPath: this.archiveStack[0]?.fsPath ?? '', fsParentDir: this.fsParentDir,
        });
      } else {
        this.tabPaths.set(this.currentTabId, this.currentPath);
        this.workspaceService.updateTabPath(this.paneId, this.currentTabId, this.currentPath);
      }
    }

    this.workspaceService.switchTab(this.paneId, tab.id);
    this.currentTabId = tab.id;
    this.clearArchiveState();

    const archiveState = this.workspaceService.getArchiveState(this.paneId, tab.id);
    if (archiveState) {
      this.currentPath  = archiveState.path;
      this.archiveStack = archiveState.archiveStack;
      this.fsParentDir  = archiveState.fsParentDir;
      if (this.archiveStack.length > 0) {
        this.renderArchiveFrame(this.archiveStack[this.archiveStack.length - 1]);
      }
      return;
    }

    const restoredPath = this.tabPaths.get(tab.id) ?? tab.path;
    this.currentPath = restoredPath;

    if (this.archiveService.isArchiveVirtualPath(restoredPath)) {
      await this.restoreArchiveFromPath(restoredPath);
      return;
    }

    await this.loadDirectory(restoredPath);
  }

  private async restoreArchiveFromPath(path: string): Promise<void> {
    this.loading = true;
    this.error   = null;
    try {
      const { fsPath, chain } = this.archiveService.parseVirtualPath(path);
      let frame = await this.archiveService.openRootArchive(fsPath);
      const stack: ArchiveFrame[] = [frame];
      for (const part of chain) {
        frame = await this.archiveService.openNestedArchive(frame, [part]);
        stack.push(frame);
      }
      this.archiveStack = stack;
      this.renderArchiveFrame(stack[stack.length - 1]);

      const virtualPath = buildArchiveDisplayPath(stack[0].fsPath, this.archiveStack);
      this.tabPaths.set(this.currentTabId, virtualPath);
      const title = buildArchiveTabTitle(stack[0].fsPath, this.archiveStack);
      this.workspaceService.updateTabPathAndTitle(this.paneId, this.currentTabId, virtualPath, title);
      this.tabs = this.workspaceService.getTabs(this.paneId);
      // Focus tree to the virtual path (best-effort)
      setTimeout(() => { try { this.fileViewContainer?.treeComponent?.focusNode(this.currentPath); } catch {} }, 0);
    } catch (err: any) {
      this.error = err.message || 'Failed to restore archive tab';
    } finally {
      this.loading = false;
    }
  }

  onTabClose(tab: TabInfo): void {
    this.workspaceService.clearArchiveState(this.paneId, tab.id);
    this.workspaceService.closeTab(this.paneId, tab.id);
    this.loadPaneState();
  }

  onTabPin(tab: TabInfo): void {
    this.workspaceService.toggleTabPin(this.paneId, tab.id);
    this.loadPaneState();
  }

  onNewTab(): void {
    const newTab = this.workspaceService.addTab(this.paneId, this.currentPath, true);
    if (!newTab) return;
    this.currentTabId = newTab.id;
    this.tabs = this.workspaceService.getTabs(this.paneId);
    this.workspaceService.switchTab(this.paneId, newTab.id);
    this.tabs = this.workspaceService.getTabs(this.paneId);
  }

  // ── Address bar / path ────────────────────────────────────────────────────

  onPathChange(path: string): void {
    if (path && path !== this.currentPath) {
      if (this.archiveService.isArchiveVirtualPath(path)) {
        this.currentPath = path;
        this.clearArchiveState();
        this.restoreArchiveFromPath(path);
      } else {
        this.exitArchive();
        this.clearArchiveState();
        this.loadDirectory(path);
      }
    }
  }

  async onNavigateUp(): Promise<void> {
    if (this.isInArchive()) {
      await this.navigateArchiveUp();
    } else {
      try {
        const listing = await window.go.main.App.NavGoUp(this.paneId);
        this.currentPath = listing.path;
        this.tabPaths.set(this.currentTabId, listing.path);
        this.entries = this.withParentEntry(listing.entries, false, listing.path);
        this.selectedFiles.clear();
        this.applyFilter();
        this.workspaceService.updateTabPath(this.paneId, this.currentTabId, listing.path);
        this.pathHistoryService.addPath(listing.path, this.paneId);
        this.pathChange.emit(listing.path);
        // Ensure tree focus updates after navigating up
        setTimeout(() => { try { this.fileViewContainer?.treeComponent?.focusNode(listing.path); } catch {} }, 0);
      } catch {
        const target = parentPath(normalizePath(this.currentPath));
        if (target !== this.currentPath) await this.loadDirectory(target);
      }
    }
  }

  onRefresh(): void {
    if (this.isInArchive()) this.refreshArchive();
    else this.loadDirectory(this.currentPath);
  }

  // ── Archive segment click ────────────────────────────────────────────────

  async onArchiveSegmentClick(virtualPath: string): Promise<void> {
    // Not currently in an archive — open it fresh (e.g. typed in address bar, or pin click)
    if (!this.isInArchive()) {
      this.currentPath = virtualPath;
      this.clearArchiveState();
      await this.restoreArchiveFromPath(virtualPath);
      return;
    }

    const fsNorm   = (this.archiveStack[0]?.fsPath ?? '').replace(/\\/g, '/');
    const vNorm    = virtualPath.replace(/\\/g, '/');
    const afterFs  = vNorm.startsWith(fsNorm) ? vNorm.slice(fsNorm.length).replace(/^\//, '') : '';

    if (!afterFs) {
      const rootFrame = this.archiveStack[0];
      this.loading = true;
      try {
        const frame = await this.archiveService.navigateInto(rootFrame, '');
        this.archiveStack = [{ ...frame, innerPath: '' }];
        this.renderArchiveFrame({ ...frame, innerPath: '' });
      } catch (err: any) { this.error = err.message; }
      finally { this.loading = false; }
      return;
    }

    const parts = afterFs.split('/').filter(Boolean);
    this.loading = true;
    try {
      let frame = await this.archiveService.navigateInto(this.archiveStack[0], '');
      let stack: ArchiveFrame[] = [{ ...frame, innerPath: '' }];

      for (const part of parts) {
        const isNested = frame.listing.entries.some(
          e => e.name === part && (e.type === FileType.ARCHIVE || isArchive(e.name))
        );
        if (isNested) {
          const nestedChain = [...frame.nestedChain, part];
          frame = await this.archiveService.openNestedArchive(frame, nestedChain);
          stack.push(frame);
        } else {
          const innerPath = frame.innerPath ? `${frame.innerPath}/${part}` : part;
          frame = await this.archiveService.navigateInto(stack[stack.length - 1], innerPath);
          stack = [...stack.slice(0, -1), frame];
        }
      }

      this.archiveStack = stack;
      this.renderArchiveFrame(stack[stack.length - 1]);
    } catch (err: any) {
      this.error = err.message || 'Failed to navigate to archive segment';
    } finally {
      this.loading = false;
    }
  }

  // ── Navigation ────────────────────────────────────────────────────────────

  onNavigate(entry: FileInfo): void {
    if (entry.path === PARENT_ENTRY_PATH) { this.onNavigateUp(); return; }

    if (this.isInArchive()) {
      if (entry.type === FileType.DIRECTORY) { this.archiveNavigateInto(entry.path); }
      else if (entry.type === FileType.ARCHIVE) {
        const chain: string[] = (entry as any).nestedChain;
        if (chain?.length) this.openNestedArchive(chain);
      }
      return;
    }

    if (this.currentView === 'tree' && entry.type === FileType.DIRECTORY) {
      this.currentPath = entry.path;
      this.pathChange.emit(entry.path);
      return;
    }

    if (entry.type === FileType.DIRECTORY) {
      this.loadDirectory(entry.path);
      return;
    }
    if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
      this.openRootArchive(entry.path);
    }
  }

  onOpenSelectedItem(): void {
    const selected = Array.from(this.selectedFiles);
    if (selected.length === 1) {
      const entry = this.entries.find(e => e.path === selected[0]);
      if (entry) this.onNavigate(entry);
    }
  }

  // ── Archive navigation ────────────────────────────────────────────────────

  isInArchive(): boolean { return this.archiveStack?.length > 0; }

  async openRootArchive(fsPath: string): Promise<void> {
    this.loading = true;
    this.error   = null;
    const exitPath = this.currentPath;  // ← capture parent dir before entering archive
    try {
      const frame       = await this.archiveService.openRootArchive(fsPath);
      this.archiveStack = [frame];
      this.fsParentDir  = exitPath;     // ← use captured value
      this.renderArchiveFrame(frame);
    } catch (err: any) {
      this.error = err.message || 'Failed to open archive';
    } finally {
      this.loading = false;
    }
  }

  private async archiveNavigateInto(folderInnerPath: string): Promise<void> {
    const currentFrame = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error   = null;
    try {
      const newFrame    = await this.archiveService.navigateInto(currentFrame, folderInnerPath);
      this.archiveStack = [...this.archiveStack.slice(0, -1), newFrame];
      this.renderArchiveFrame(newFrame);
    } catch (err: any) {
      this.error = err.message || 'Failed to navigate into folder';
    } finally {
      this.loading = false;
    }
  }

  private async openNestedArchive(nestedChain: string[]): Promise<void> {
    const currentFrame = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error   = null;
    try {
      const newFrame    = await this.archiveService.openNestedArchive(currentFrame, nestedChain);
      this.archiveStack = [...this.archiveStack, newFrame];
      this.renderArchiveFrame(newFrame);
    } catch (err: any) {
      this.error = err.message || 'Failed to open nested archive';
    } finally {
      this.loading = false;
    }
  }

  async navigateArchiveUp(): Promise<void> {
    const currentFrame = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error = null;
    try {
      const parentFrame = await this.archiveService.navigateUp(currentFrame);
      if (parentFrame !== null) {
        this.archiveStack = [...this.archiveStack.slice(0, -1), parentFrame];
        this.renderArchiveFrame(parentFrame);
        return;
      }

      const poppedStack = this.archiveStack.slice(0, -1);
      if (poppedStack.length === 0) {
        const exitPath = this.fsParentDir || this.currentPath;  // capture before clear
        this.clearArchiveState();                                 // now zero both fields
        await this.loadDirectory(exitPath);
        return;
      }
      
      this.archiveStack = poppedStack;
      this.renderArchiveFrame(poppedStack[poppedStack.length - 1]);
    } catch (err: any) {
      this.error = err.message || 'Failed to navigate up';
    } finally {
      this.loading = false;
    }
  }

  exitArchive(): void {
    if (this.isInArchive() && this.currentTabId) {
      this.workspaceService.saveArchiveState(this.paneId, this.currentTabId, {
        path: this.currentPath, archiveStack: this.archiveStack,
        fsDirPath: this.archiveStack[0]?.fsPath ?? '', fsParentDir: this.fsParentDir,
      });
    }
    this.archiveStack = [];
  }

  private async refreshArchive(): Promise<void> {
    const frame  = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error   = null;
    try {
      const refreshed   = await this.archiveService.refreshFrame(frame);
      this.archiveStack = [...this.archiveStack.slice(0, -1), refreshed];
      this.renderArchiveFrame(refreshed);
    } catch (err: any) {
      this.error = err.message || 'Failed to refresh archive';
    } finally {
      this.loading = false;
    }
  }

  private renderArchiveFrame(frame: ArchiveFrame): void {
    const rawEntries  = this.archiveService.listingToFileInfos(frame.listing);
    this.entries      = this.withParentEntry(rawEntries, true, '');
    this.selectedFiles.clear();
    this.applyFilter();

    const fsPath      = this.archiveStack[0]?.fsPath ?? '';
    const virtualPath = buildArchiveDisplayPath(fsPath, this.archiveStack);
    this.currentPath  = virtualPath;
    this.tabPaths.set(this.currentTabId, virtualPath);

    const title = buildArchiveTabTitle(fsPath, this.archiveStack);
    this.workspaceService.updateTabPathAndTitle(this.paneId, this.currentTabId, virtualPath, title);
    this.tabs = this.workspaceService.getTabs(this.paneId);
  }

  // ── Find widget ───────────────────────────────────────────────────────────

  onFindStateChange(state: FindWidgetState | null): void {
    if (!state) {
      this.filterQuery = { text: '', criteria: [] };
    } else {
      // Build a FilterQuery from the find widget state.
      // matchCase / wholeWord / useRegex are passed as criteria for FilterService.
      this.filterQuery = this.filterService.parseFilterText(state.text);
      // Patch in the extra options so FilterService can honour them if supported.
      (this.filterQuery as any).matchCase  = state.matchCase;
      (this.filterQuery as any).wholeWord  = state.wholeWord;
      (this.filterQuery as any).useRegex   = state.useRegex;
    }
    this.applyFilter();

    // Update match count badge on the widget
    if (this.findWidget) {
      this.findWidget.setMatchCount(this.filteredEntries.length);
    }
  }

  onFindNext(): void { /* file-list scroll-to-next can be wired here */ }
  onFindPrev(): void { /* file-list scroll-to-prev can be wired here */ }

  // ── Filtering ─────────────────────────────────────────────────────────────

  private applyFilter(): void {
    const q = this.filterQuery;
    if (!q?.text?.trim() && !q?.criteria?.length) {
      this.filteredEntries = this.entries;
      return;
    }
    // Always keep the ".." entry visible regardless of the active filter.
    const parentEntry = this.entries.filter(e => e.path === PARENT_ENTRY_PATH);
    const filtered    = this.filterService.filterEntries(
      this.entries.filter(e => e.path !== PARENT_ENTRY_PATH),
      q
    );
    this.filteredEntries = [...parentEntry, ...filtered];
  }

  // ── Selection ─────────────────────────────────────────────────────────────

  onSelectionChange(entryOrEntries: FileInfo | FileInfo[]): void {
    // Support single FileInfo or array of FileInfo from tree selection
    if (Array.isArray(entryOrEntries)) {
      const paths = entryOrEntries.map(e => e.path).filter(p => p !== PARENT_ENTRY_PATH);
      this.selectedFiles = new Set(paths);
      return;
    }

    const entry = entryOrEntries as FileInfo;
    // The ".." entry is never selectable
    if (entry.path === PARENT_ENTRY_PATH) return;

    // Toggle selection
    if (this.selectedFiles.has(entry.path)) {
      this.selectedFiles.delete(entry.path);
    } else {
      // For single selection mode, clear others first
      // For multi-selection, keep existing and add new
      this.selectedFiles.add(entry.path);
    }
    this.selectedFiles = new Set(this.selectedFiles);
  }

  onSelectAll(): void {
    this.filteredEntries
      .filter(e => e.path !== PARENT_ENTRY_PATH)
      .forEach(e => this.selectedFiles.add(e.path));
    this.selectedFiles = new Set(this.selectedFiles);
  }

  onDeselectAll(): void {
    this.selectedFiles.clear();
    this.selectedFiles = new Set(this.selectedFiles);
  }

  // ── File operations ───────────────────────────────────────────────────────

  async onCreateFolder(): Promise<void> {
    if (this.isInArchive()) { alert('Cannot create folders inside an archive.'); return; }
    const folderName = prompt('Enter folder name:');
    if (!folderName) return;
    try {
      const newPath: string = await window.go.main.App.JoinPaths([this.currentPath, folderName]);
      await this.apiService.createDirectory(newPath, false);
      await this.loadDirectory(this.currentPath);
    } catch (error: any) { alert(`Failed to create folder: ${error.message}`); }
  }

  async onDeleteSelected(): Promise<void> {
    if (this.isInArchive()) { alert('Cannot delete files inside an archive.'); return; }
    if (this.selectedFiles.size === 0) return;
    if (!confirm(`Delete ${this.selectedFiles.size} item(s)?`)) return;
    for (const path of Array.from(this.selectedFiles)) {
      try { await this.apiService.deleteFile(path, true); }
      catch (error: any) { alert(`Failed to delete ${path}: ${error.message}`); }
    }
    await this.loadDirectory(this.currentPath);
  }

  onCopyToOther(): void { 
    if (this.selectedFiles.size > 0) {
      this.copyToOther.emit(Array.from(this.selectedFiles));
    }
  }
  
  onMoveToOther(): void { 
    if (this.selectedFiles.size > 0) {
      this.moveToOther.emit(Array.from(this.selectedFiles));
    }
  }

  // ── Tab shortcuts ─────────────────────────────────────────────────────────

  onCloseCurrentTab():   void { 
    const t = this.tabs.find(t => t.isActive); 
    if (t) this.onTabClose(t); 
  }
  
  onNextTab():           void { 
    const i = this.tabs.findIndex(t => t.isActive); 
    if (i >= 0) this.onTabChange(this.tabs[(i + 1) % this.tabs.length]); 
  }
  
  onPreviousTab():       void { 
    const i = this.tabs.findIndex(t => t.isActive); 
    if (i >= 0) this.onTabChange(this.tabs[i === 0 ? this.tabs.length - 1 : i - 1]); 
  }
  
  onJumpToTab(n: number): void { 
    if (n > 0 && n <= this.tabs.length) this.onTabChange(this.tabs[n - 1]); 
  }
  
  onTogglePinCurrentTab(): void { 
    const t = this.tabs.find(t => t.isActive); 
    if (t) this.onTabPin(t); 
  }

  focusAddressBar(): void { 
    this.addressBar?.enterEditMode(); 
  }
  
  focusFind():       void { 
    this.findWidget?.open(); 
  }

  focusFilter(): void { 
    this.findWidget?.toggle(); 
  }
  
  onClearFilter(): void { 
    this.findWidget?.close(); 
  }
}
