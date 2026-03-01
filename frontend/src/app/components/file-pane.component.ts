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
  ArchiveService, ArchiveFrame, isArchive, buildArchiveDisplayPath, buildArchiveTabTitle,
  parseArchivePath, buildArchivePath
} from '../services/archive.service';
import { parentPath, normalizePath } from './address-bar.utils';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { FileViewContainerComponent, ViewLayout } from './file-view-container.component';
import { ArchiveMoveWarningComponent } from "./archive-move-warning.component";

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
    FileViewContainerComponent,
    ArchiveMoveWarningComponent
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
        (refreshClicked)="onRefresh()">
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
          [error]="error"
          [currentView]="currentView"
          [sortField]="sortField"
          [sortDirection]="sortDirection"
          (viewChange)="onViewChange($event)"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)"
          (openInNewTab)="onOpenInNewTab($event)"
          (sortChange)="onSortChange($event)"
          (contextMenu)="onContextMenu($event)"
          (nodeDoubleClick)="onNodeDoubleClick($event)"
          (dragDrop)="onDragDrop($event)"
          (rename)="onRename($event)">
        </app-file-view-container>
      </div>

      <!-- Archive move warning dialog -->
      <app-archive-move-warning
        [visible]="archiveMoveWarning"
        [entryCount]="archivePendingMoveEntries.length"
        (confirmed)="executePendingMove()"
        (cancelled)="archiveMoveWarning = false">
      </app-archive-move-warning>
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
    /**
     * Extract selected entries from the current archive frame.
     * If all top-level entries are selected, extracts the whole archive (root forrest).
     * Otherwise, extracts only the selected files/folders.
     */
    async onExtractSelected(destDir: string, flattenPaths = false): Promise<void> {
      if (!this.isInArchive() || this.selectedFiles.size === 0) return;
      const frame = this.archiveStack[this.archiveStack.length - 1];
      const entries = Array.from(this.selectedFiles);
      try {
        const result = await this.archiveService.extractEntries(frame, entries, destDir, flattenPaths);
        if (result.failed > 0) {
          alert('Extract failed:\n' + (result.errors ?? []).map((e: any) => e.error).join('\n'));
        } else {
          alert(`${result.succeeded} file(s) extracted to destination.`);
        }
      } catch (err: any) {
        alert(`Extract failed: ${err.message}`);
      }
    }
  @ViewChild(AddressBarComponent) addressBar?: AddressBarComponent;
  @ViewChild(FileViewContainerComponent) fileViewContainer?: FileViewContainerComponent;
  @ViewChild('findWidget')        findWidget?: FindWidgetComponent;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() workspace: WorkspaceConfig | null = null;
  @Input() showHidden = false;

  @Output() pathChange    = new EventEmitter<string>();
  @Output() copyToOther   = new EventEmitter<string[]>();
  @Output() moveToOther   = new EventEmitter<string[]>();
  @Output() filesDeleted  = new EventEmitter<void>();

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

  // Archive move state
  archiveMoveWarning = false;
  archivePendingMoveEntries: string[] = [];
  archivePendingMoveDestDir = '';

  private tabPaths = new Map<string, string>();
  private selectionCursorPath: string | null = null;
  private selectionAnchorPath: string | null = null;

  FileType = FileType;

  constructor(
    private apiService: ApiService,
    private workspaceService: WorkspaceService,
    private filterService: FilterService,
    private pathHistoryService: PathHistoryService,
    private archiveService: ArchiveService
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
      await this.refreshCurrentPathTransparently();
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

  private isTransparentArchivePath(path: string): boolean {
    return this.archiveService.isArchiveVirtualPath(path);
  }

  private async loadPathTransparently(path: string): Promise<void> {
    if (this.isTransparentArchivePath(path)) {
      await this.restoreArchiveFromPath(path);
      return;
    }

    await this.loadDirectory(path);
  }

  private async refreshCurrentPathTransparently(): Promise<void> {
    if (!this.currentPath) return;

    if (!this.isTransparentArchivePath(this.currentPath)) {
      await this.loadDirectory(this.currentPath);
      return;
    }

    if (this.isInArchive()) {
      const currentFrame = this.archiveStack[this.archiveStack.length - 1];
      if (currentFrame) {
        this.renderArchiveFrame(currentFrame);
        return;
      }
    }

    await this.restoreArchiveFromPath(this.currentPath);
  }

  // ── Pane state ────────────────────────────────────────────────────────────

  private async loadPaneState(): Promise<void> {
    if (!this.workspace) return;
    this.tabs = this.workspaceService.getTabs(this.paneId);
    const activeTab  = this.workspaceService.getActiveTab(this.paneId);
    const savedPath  = activeTab?.path ?? '';

    const allPaths   = this.workspaceService.getAllTabPaths();
    const windowsRoot = allPaths.find(p => /^[A-Za-z]:[/\\]/.test(p));
    let platformRoot = windowsRoot
      ? windowsRoot.replace(/\\/g, '/').replace(/^([A-Za-z]:).*$/, '$1/')
      : '/';

    // No cached Windows path yet — ask the backend for the system drive.
    // This ensures a fresh Windows install starts on C:/ (or the actual
    // system drive) instead of the Unix-style '/'.
    if (platformRoot === '/') {
      try {
        const osInfo = await this.apiService.getOSInfo();
        if (osInfo?.systemDrive) {
          const drive = (osInfo.systemDrive as string).replace(/\\/g, '/').toUpperCase();
          platformRoot = drive.endsWith('/') ? drive : drive + '/';
        }
      } catch { /* ignore — stays '/' on non-Windows */ }
    }

    const isWindows   = platformRoot !== '/';
    const pathIsStale = !savedPath || (isWindows && savedPath === '/');
    const pathToLoad  = pathIsStale ? platformRoot : savedPath;

    this.currentPath = pathToLoad;
    this.pathReady   = true;
    if (activeTab) this.currentTabId = activeTab.id;

    this.clearArchiveState();
    await this.loadPathTransparently(pathToLoad);
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
    await this.loadPathTransparently(restoredPath);
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

  private async openPathInNewTab(path: string): Promise<void> {
    const newTab = this.workspaceService.addTab(this.paneId, path, true);
    if (!newTab) return;
    this.tabs = this.workspaceService.getTabs(this.paneId);
    await this.onTabChange(newTab);
  }

  // ── Address bar / path ────────────────────────────────────────────────────

  onPathChange(path: string): void {
    if (path && path !== this.currentPath) {
      this.currentPath = path;
      this.exitArchive();
      this.clearArchiveState();
      void this.loadPathTransparently(path);
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
    void this.refreshCurrentPathTransparently();
    // Keep the tree in sync: reload children of the current directory node
    // without collapsing any expanded subtrees.
    this.fileViewContainer?.refreshTreeNode(this.currentPath);
  }

  /**
   * Returns the effective destination directory for a copy/move operation.
   * If exactly one directory is selected in this pane, operations are directed
   * into that folder rather than the generic currentPath.
   */
  getDestinationDir(): string {
    if (this.selectedFiles.size !== 1) return this.currentPath;
    const [selectedPath] = this.selectedFiles;

    // List / grid / details mode: check entries directly.
    const entry = this.entries.find(e => e.path === selectedPath);
    if (entry?.type === FileType.DIRECTORY) return selectedPath;

    // Tree mode: check visible tree nodes.
    if (this.currentView === 'tree') {
      const nodes = this.fileViewContainer?.treeComponent?.getVisibleNodes() ?? [];
      const node = nodes.find(n => n.path === selectedPath);
      if (node?.type === FileType.DIRECTORY) return selectedPath;
    }

    return this.currentPath;
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

    // Tree navigation: real FS directory must exit archive mode first so the
    // tree root is never overwritten with a virtual archive path.
    if (this.currentView === 'tree' && entry.type === FileType.DIRECTORY) {
      if (this.isInArchive()) {
        this.exitArchive();
        this.clearArchiveState();
      }
      this.currentPath = entry.path;
      this.pathChange.emit(entry.path);
      return;
    }

    if (this.isInArchive()) {
      if (entry.type === FileType.DIRECTORY) { this.archiveNavigateInto(entry.path); }
      else if (entry.type === FileType.ARCHIVE) {
        const chain: string[] = (entry as any).nestedChain;
        if (chain?.length) this.openNestedArchive(chain);
      }
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

  async onOpenInNewTab(entry: FileInfo): Promise<void> {
    if (entry.path === PARENT_ENTRY_PATH) {
      const up = this.isInArchive() ? this.currentPath : parentPath(this.currentPath);
      await this.openPathInNewTab(up);
      return;
    }

    if (entry.type === FileType.DIRECTORY) {
      await this.openPathInNewTab(entry.path);
      return;
    }

    if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
      await this.openPathInNewTab(entry.path);
    }
  }

  async onRenameSelected(): Promise<void> {
    const selected = Array.from(this.selectedFiles);
    if (selected.length !== 1) return;
    const entry = this.entries.find(e => e.path === selected[0]);
    if (!entry || entry.path === PARENT_ENTRY_PATH) return;
    await this.renameEntry(entry);
  }

  async onRename(event: { entry: FileInfo; newName: string }): Promise<void> {
    await this.renameEntry(event.entry, event.newName);
  }

  async onDragDrop(event: { event: DragEvent; source: FileInfo[]; targetPath: string }): Promise<void> {
    if (!event.source?.length || !event.targetPath) return;

    if (this.isInArchive()) {
      alert('Drag & drop within archive view is not supported yet. Use copy/move to other pane.');
      return;
    }

    const sourcePaths = event.source
      .map(e => e.path)
      .filter(p => p && p !== PARENT_ENTRY_PATH);

    if (!sourcePaths.length) return;

    try {
      const items = await Promise.all(sourcePaths.map(async sourcePath => ({
        source: sourcePath,
        destination: await this.buildDestinationPath(sourcePath, event.targetPath),
      })));

      const isCopy = event.event.ctrlKey || event.event.metaKey;
      if (isCopy) {
        await this.apiService.batchCopy(items, true);
      } else {
        await this.apiService.batchMove(items, true);
      }

      await this.onRefresh();
    } catch (error: any) {
      alert(`Drag & drop failed: ${error.message}`);
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

  onSelectionChange(entries: FileInfo[]): void {
    const paths = entries.map(e => e.path).filter(p => p !== PARENT_ENTRY_PATH);
    this.selectedFiles = new Set(paths);
    this.selectionCursorPath = paths.length ? paths[paths.length - 1] : null;
    if (paths.length === 0) {
      this.selectionAnchorPath = null;
    } else if (paths.length === 1) {
      this.selectionAnchorPath = this.selectionCursorPath;
    } else if (!this.selectionAnchorPath) {
      this.selectionAnchorPath = this.selectionCursorPath;
    }
  }

  onSelectAll(): void {
    const selectable = this.getSelectableEntries();
    this.selectedFiles = new Set(selectable.map(e => e.path));
    this.selectionCursorPath = selectable.length ? selectable[selectable.length - 1].path : null;
    this.selectionAnchorPath = this.selectionCursorPath;
  }

  onDeselectAll(): void {
    this.selectedFiles.clear();
    this.selectedFiles = new Set(this.selectedFiles);
    this.selectionCursorPath = null;
    this.selectionAnchorPath = null;
  }

  onToggleCurrentAndMoveDown(): void {
    const selectable = this.getSelectableEntries();
    const index = this.getCursorIndex(selectable);
    if (index < 0) return;

    const path = selectable[index].path;
    const next = new Set(this.selectedFiles);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    this.selectedFiles = next;

    const nextIndex = Math.min(index + 1, selectable.length - 1);
    this.selectionCursorPath = selectable[nextIndex]?.path ?? path;
    this.selectionAnchorPath = this.selectionAnchorPath ?? path;
  }

  onSelectCurrentAndMoveDown(): void {
    const selectable = this.getSelectableEntries();
    const index = this.getCursorIndex(selectable);
    if (index < 0) return;

    const path = selectable[index].path;
    const next = new Set(this.selectedFiles);
    next.add(path);
    this.selectedFiles = next;

    const nextIndex = Math.min(index + 1, selectable.length - 1);
    this.selectionCursorPath = selectable[nextIndex]?.path ?? path;
    this.selectionAnchorPath = path;
  }

  onExtendSelection(delta: -1 | 1): void {
    const selectable = this.getSelectableEntries();
    const index = this.getCursorIndex(selectable);
    if (index < 0) return;

    const targetIndex = Math.max(0, Math.min(selectable.length - 1, index + delta));
    const anchorPath = this.selectionAnchorPath ?? selectable[index].path;
    const anchorIndex = selectable.findIndex(e => e.path === anchorPath);
    const start = Math.min(anchorIndex === -1 ? index : anchorIndex, targetIndex);
    const end = Math.max(anchorIndex === -1 ? index : anchorIndex, targetIndex);

    this.selectedFiles = new Set(selectable.slice(start, end + 1).map(e => e.path));
    this.selectionCursorPath = selectable[targetIndex].path;
    this.selectionAnchorPath = anchorPath;
  }

  onSelectByMask(): void {
    const mask = prompt('Select by mask (example: *.txt):', '*.*')?.trim();
    if (!mask) return;

    const matcher = this.createMaskMatcher(mask);
    const next = new Set(this.selectedFiles);
    for (const entry of this.getSelectableEntries()) {
      if (matcher(entry.name)) next.add(entry.path);
    }
    this.selectedFiles = next;
    this.syncCursorFromSelection();
  }

  onUnselectByMask(): void {
    const mask = prompt('Unselect by mask (example: *.txt):', '*.*')?.trim();
    if (!mask) return;

    const matcher = this.createMaskMatcher(mask);
    const next = new Set(this.selectedFiles);
    for (const entry of this.getSelectableEntries()) {
      if (matcher(entry.name)) next.delete(entry.path);
    }
    this.selectedFiles = next;
    this.syncCursorFromSelection();
  }

  onInvertSelection(): void {
    const selectable = this.getSelectableEntries();
    const next = new Set<string>();
    for (const entry of selectable) {
      if (!this.selectedFiles.has(entry.path)) {
        next.add(entry.path);
      }
    }
    this.selectedFiles = next;
    this.syncCursorFromSelection();
  }

  onSelectSameExtension(): void {
    const current = this.getCurrentCursorEntry();
    if (!current) return;

    const extension = this.getExtension(current.name);
    if (!extension) return;

    const next = new Set(this.selectedFiles);
    for (const entry of this.getSelectableEntries()) {
      if (this.getExtension(entry.name) === extension) {
        next.add(entry.path);
      }
    }
    this.selectedFiles = next;
    this.syncCursorFromSelection();
  }

  private syncCursorFromSelection(): void {
    const selectable = this.getSelectableEntries();
    if (!selectable.length || this.selectedFiles.size === 0) {
      this.selectionCursorPath = selectable.length ? selectable[0].path : null;
      this.selectionAnchorPath = this.selectionCursorPath;
      return;
    }

    const firstSelected = selectable.find(e => this.selectedFiles.has(e.path));
    if (firstSelected) {
      this.selectionCursorPath = firstSelected.path;
      this.selectionAnchorPath = this.selectionAnchorPath ?? firstSelected.path;
    }
  }

  private getSelectableEntries(): FileInfo[] {
    if (this.currentView === 'tree') {
      const tree = this.fileViewContainer?.treeComponent;
      if (tree) {
        return tree
          .getVisibleNodes()
          .filter(node => node.path !== PARENT_ENTRY_PATH && (node.level ?? 0) >= 0)
          .map(node => ({
            name: node.name,
            path: node.path,
            type: node.type,
            size: node.size,
            modified: node.modified,
            created: node.created,
          } as FileInfo));
      }
    }

    return this.filteredEntries.filter(e => e.path !== PARENT_ENTRY_PATH);
  }

  private getCursorIndex(entries: FileInfo[]): number {
    if (!entries.length) return -1;

    if (this.selectionCursorPath) {
      const idx = entries.findIndex(e => e.path === this.selectionCursorPath);
      if (idx !== -1) return idx;
    }

    const firstSelected = entries.findIndex(e => this.selectedFiles.has(e.path));
    if (firstSelected !== -1) {
      this.selectionCursorPath = entries[firstSelected].path;
      return firstSelected;
    }

    this.selectionCursorPath = entries[0].path;
    return 0;
  }

  private getCurrentCursorEntry(): FileInfo | null {
    const entries = this.getSelectableEntries();
    const idx = this.getCursorIndex(entries);
    return idx >= 0 ? entries[idx] : null;
  }

  private createMaskMatcher(mask: string): (value: string) => boolean {
    const escaped = mask
      .replace(/[.+^${}()|[\]\\]/g, '\\$&')
      .replace(/\*/g, '.*')
      .replace(/\?/g, '.');
    const regex = new RegExp(`^${escaped}$`, 'i');
    return (value: string) => regex.test(value);
  }

  private getExtension(name: string): string {
    const idx = name.lastIndexOf('.');
    if (idx <= 0 || idx === name.length - 1) return '';
    return name.slice(idx + 1).toLowerCase();
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
    if (this.selectedFiles.size === 0) return;

    if (this.isInArchive()) {
      const frame   = this.archiveStack[this.archiveStack.length - 1];
      const entries = Array.from(this.selectedFiles);

      if (frame.nestedChain.length > 0) {
        alert(
          'Cannot delete from a nested archive.\n' +
          'Exit back to the top-level archive first.',
        );
        return;
      }

      if (!confirm(
        `Delete ${entries.length} item(s) from the archive?\n` +
        `This rewrites the archive file on disk and cannot be undone.`,
      )) return;

      try {
        const result = await this.archiveService.delete(frame, entries);
        if (result.failed > 0) {
          alert('Delete failed:\n' + (result.errors ?? []).map((e: any) => e.error).join('\n'));
        }
      } catch (err: any) {
        alert(`Delete error: ${err.message}`);
        return;
      }
      // Refresh the archive listing so deleted entries disappear
      await this.refreshArchive();
      return;
    }

    // ── Filesystem delete — unchanged from original ───────────────────────────
    if (!confirm(`Delete ${this.selectedFiles.size} item(s)?`)) return;
    try {
      await this.apiService.batchDelete(Array.from(this.selectedFiles), true);
    } catch (error: any) {
      alert(`Failed to delete items: ${error.message}`);
    }
    await this.loadDirectory(this.currentPath);
    this.fileViewContainer?.refreshTreeNode(this.currentPath);
    this.filesDeleted.emit();
  }

  /**
   * Returns archive metadata for the current tree-view selection when the
   * selected nodes are inside an expanded archive node (not in archive-pane
   * mode). All selected nodes must share the same archive FS path and nested
   * chain; otherwise returns null.
   */
  getTreeArchiveSelectionContext(): { archivePath: string; innerPaths: string[] } | null {
    if (this.currentView !== 'tree' || this.selectedFiles.size === 0) return null;

    const tree = this.fileViewContainer?.treeComponent;
    if (!tree) return null;

    const visible = tree.getVisibleNodes();
    const selected = visible.filter(n => this.selectedFiles.has(n.path));
    if (!selected.length) return null;

    const firstArchivePath = (selected[0] as any).archivePath as string | undefined;
    if (!firstArchivePath) return null;

    // Derive the archive context path (fsPath + nestedChain, no innerPath).
    const { fsPath, nestedChain } = parseArchivePath(firstArchivePath);
    const contextPath = buildArchivePath(fsPath, nestedChain, '');

    // All selected must come from the same archive context.
    if (!selected.every(n => {
      const nPath = (n as any).archivePath as string | undefined;
      if (!nPath) return false;
      const { fsPath: nFs, nestedChain: nChain } = parseArchivePath(nPath);
      return buildArchivePath(nFs, nChain, '') === contextPath;
    })) return null;

    // Inner paths: each selected file's path within its (nested) archive.
    const innerPaths = selected.map(n => {
      const nPath = (n as any).archivePath as string | undefined;
      return nPath ? (parseArchivePath(nPath).innerPath || n.name) : n.name;
    }).filter(Boolean);

    return innerPaths.length ? { archivePath: contextPath, innerPaths } : null;
  }

  onCopyToOther(): void { 
    if (this.selectedFiles.size > 0) {
      this.copyToOther.emit(Array.from(this.selectedFiles));
    }
  }

  private async renameEntry(entry: FileInfo, forcedName?: string): Promise<void> {
    if (this.isInArchive()) {
      alert('Rename inside archive view is not supported yet.');
      return;
    }

    if (entry.path === PARENT_ENTRY_PATH) return;

    const nextName = (forcedName ?? prompt('Rename to:', entry.name) ?? '').trim();
    if (!nextName || nextName === entry.name) return;

    try {
      const destination = await this.apiService.joinPaths([parentPath(entry.path), nextName]);
      await this.apiService.batchMove([{ source: entry.path, destination }], true);
      await this.onRefresh();
    } catch (error: any) {
      alert(`Rename failed: ${error.message}`);
    }
  }

  private async buildDestinationPath(sourcePath: string, targetDir: string): Promise<string> {
    const split = await this.apiService.splitPath(sourcePath);
    const base = split.base || sourcePath.replace(/\\/g, '/').split('/').filter(Boolean).pop() || '';
    return await this.apiService.joinPaths([targetDir, base]);
  }
  
  onMoveToOther(): void { 
    if (this.selectedFiles.size > 0) {
      this.moveToOther.emit(Array.from(this.selectedFiles));
    }
  }

  // ── Archive move operations ───────────────────────────────────────────────

  /**
   * Called by FileBrowserComponent after it has resolved destDir.
   * Shows the confirmation dialog; executePendingMove runs when user confirms.
   */
  initiatePendingMove(entries: string[], destDir: string): void {
    this.archivePendingMoveEntries = entries;
    this.archivePendingMoveDestDir = destDir;
    this.archiveMoveWarning        = true;
  }

  /**
   * Bound to (confirmed) on the ArchiveMoveWarningDialogComponent.
   */
  async executePendingMove(): Promise<void> {
    this.archiveMoveWarning = false;
    const frame   = this.archiveStack[this.archiveStack.length - 1];
    const entries = this.archivePendingMoveEntries;
    const destDir = this.archivePendingMoveDestDir;
    this.archivePendingMoveEntries = [];
    this.archivePendingMoveDestDir = '';

    try {
      const result = await this.archiveService.move(frame, entries, destDir);

      // sourceRemoved === false means extraction succeeded but delete was skipped
      if (!result.sourceRemoved) {
        const softMsg = (result.errors ?? []).find((e: any) =>
          e.error.includes('NOT removed'),
        );
        alert(
          `${result.succeeded} file(s) extracted to destination.\n\n` +
          `⚠ Source entries were NOT removed from the archive: ` +
          (softMsg?.error ?? 'nested archive deletion is not supported.'),
        );
        return;
      }

      if (result.failed > 0) {
        alert('Move errors:\n' + (result.errors ?? []).map((e: any) => e.error).join('\n'));
      }

      // Refresh archive listing — moved entries have been deleted from it
      await this.refreshArchive();
    } catch (err: any) {
      alert(`Move failed: ${err.message}`);
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