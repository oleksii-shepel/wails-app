// file-pane.component.ts
import { Component, Input, Output, EventEmitter, OnInit, OnChanges, OnDestroy, ViewChild, SimpleChanges } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TabBarComponent, type TabDropEvent } from './tab-bar.component';
import { AddressBarComponent } from './address-bar.component';
import { FindWidgetComponent, FindWidgetState } from './filter-widget.component';
import { ActionBarComponent } from './action-bar.component';
import { FileInfo, FileType } from '../shared/protocol';
import { TabInfo, WorkspaceConfig, FilterQuery } from '../shared/protocol-enhanced';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
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
import { EventsOn, EventsOff } from 'wailsjs/runtime';
import { ConfirmDialogService } from '../services/confirm-dialog.service';

declare const window: any;

interface DirStreamMetaEvent {
  streamId: string;
  paneId: string;
  path: string;
  showHidden: boolean;
}

interface DirStreamBatchEvent {
  streamId: string;
  paneId: string;
  path: string;
  entries: FileInfo[];
}

interface DirStreamDoneEvent {
  streamId: string;
  paneId: string;
  path: string;
  entryCount: number;
  totalSize: number;
  durationMs: number;
}

interface DirStreamErrorEvent {
  streamId: string;
  paneId: string;
  path: string;
  error: string;
}

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
        [paneId]="paneId"
        [tabs]="tabs"
        [activeTabId]="currentTabId"
        [inArchive]="isInArchive()"
        (tabChange)="onTabChange($event)"
        (tabClose)="onTabClose($event)"
        (tabPin)="onTabPin($event)"
        (tabDragStart)="onTabDragStart($event)"
        (tabDrop)="onTabDrop($event)"
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
        [directoriesFirst]="directoriesFirst"
        (directoriesFirstChange)="onDirectoriesFirstChange($event)"
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
          [paneId]="paneId"
          [entries]="filteredEntries"
          [currentPath]="currentPath"
          [showHidden]="showHidden"
          [selectedFiles]="selectedFiles"
          [error]="error"
          [currentView]="currentView"
          [sortField]="sortField"
          [sortDirection]="sortDirection"
          [directoriesFirst]="directoriesFirst"
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
export class FilePaneComponent implements OnInit, OnChanges, OnDestroy {
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
          this.confirmDialog.alert('Extract Failed\n' + (result.errors ?? []).map((e: any) => e.error).join('\n'));
        } else {
          this.confirmDialog.alert(`Extraction Complete\n${result.succeeded} file(s) extracted to destination.`);
        }
      } catch (err: any) {
        this.confirmDialog.alert(`Extract Failed\n${err.message}`);
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

  private optimisticHiddenPaths = new Set<string>();
  private optimisticHideListener?: (event: Event) => void;
  private optimisticRestoreListener?: (event: Event) => void;

  // View system state
  currentView: ViewLayout = 'list';
  sortField: string = 'name';
  sortDirection: 'asc' | 'desc' = 'asc';
  directoriesFirst = true;

  archiveStack: ArchiveFrame[] = [];

  // Archive move state
  archiveMoveWarning = false;
  archivePendingMoveEntries: string[] = [];
  archivePendingMoveDestDir = '';

  private tabPaths = new Map<string, string>();
  private selectionCursorPath: string | null = null;
  private selectionAnchorPath: string | null = null;
  private fsChangeDebounceTimer: ReturnType<typeof setTimeout> | null = null;
  private activeDirStreamId: string | null = null;
  private streamBaseEntries: FileInfo[] = [];
  private streamWaiters = new Map<string, { resolve: () => void; reject: (err: Error) => void }>();
  private streamPathChangeEmitted = false;
  private activeDirStreamPreserveUI = false;

  private destroy$ = createSubject<void>();
  private lastSeenActiveTabId: string | null = null;
  private lastSeenActiveTabPath: string | null = null;

  FileType = FileType;

  constructor(
    private apiService: ApiService,
    private workspaceService: WorkspaceService,
    private filterService: FilterService,
    private pathHistoryService: PathHistoryService,
      private confirmDialog: ConfirmDialogService,
    private archiveService: ArchiveService
  ) {}

  ngOnInit(): void  { 
    this.loadViewPreference();
    this.loadPaneState();
    this.bindFsChangeEvents();
    this.setupOptimisticDnDListeners();

    // Keep panes in sync when tab state changes elsewhere (e.g. cross-pane tab drag/drop).
    this.workspaceService.activeWorkspace$
      .pipe(takeUntil(this.destroy$))
      .subscribe(ws => {
        if (!ws || !this.workspace) return;
        if (ws.id !== this.workspace.id) return;
        this.onWorkspaceUpdate(ws);
      });
  }
  
  ngOnChanges(changes: SimpleChanges): void {
    if (changes['workspace']) {
      // The input is bound to FileBrowserComponent.activeWorkspace which starts as null.
      // That means the *first* change Angular sees is often null, and the transition
      // to a real workspace would be a later change. Always load once we have a
      // non-null workspace so tabs/pane render without requiring a user click.
      if (this.workspace) {
        this.loadViewPreference();
        this.lastSeenActiveTabId = null;
        this.lastSeenActiveTabPath = null;
        this.loadPaneState();
      }
      return;
    }

    if (changes['showHidden'] && !changes['showHidden'].firstChange) {
      this.onRefresh();
    }
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();

    this.unbindFsChangeEvents();

    if (this.optimisticHideListener) {
      window.removeEventListener('pane:dnd-optimistic-hide', this.optimisticHideListener);
      this.optimisticHideListener = undefined;
    }
    if (this.optimisticRestoreListener) {
      window.removeEventListener('pane:dnd-optimistic-restore', this.optimisticRestoreListener);
      this.optimisticRestoreListener = undefined;
    }
  }

  private onWorkspaceUpdate(ws: WorkspaceConfig): void {
    const pane = this.paneId === 'left' ? ws.leftPane : ws.rightPane;
    const nextActiveId = pane.activeTabId;
    const nextActivePath = pane.tabs.find(t => t.id === nextActiveId)?.path ?? '';

    // First event: establish baseline.
    if (this.lastSeenActiveTabId === null) {
      this.lastSeenActiveTabId = nextActiveId;
      this.lastSeenActiveTabPath = nextActivePath;
      this.tabs = this.workspaceService.getTabs(this.paneId);
      this.currentTabId = nextActiveId;
      return;
    }

    const activeChanged = nextActiveId !== this.lastSeenActiveTabId || nextActivePath !== this.lastSeenActiveTabPath;
    this.lastSeenActiveTabId = nextActiveId;
    this.lastSeenActiveTabPath = nextActivePath;

    if (!activeChanged) {
      // Reorder/pin changes etc — just update the tab strip.
      this.tabs = this.workspaceService.getTabs(this.paneId);
      return;
    }

    // If we already navigated to the new active tab locally, avoid a redundant reload.
    if (this.currentTabId === nextActiveId && this.currentPath === nextActivePath) {
      this.tabs = this.workspaceService.getTabs(this.paneId);
      return;
    }

    // Active tab changed (including cross-pane drag) — load the new active path.
    this.loadPaneState();
  }

  onTabDrop(event: TabDropEvent): void {
    if (!event) return;
    this.workspaceService.moveTab(event.fromPaneId, event.toPaneId, event.tabId, event.toIndex);

    // If the drop landed in this pane, ensure we navigate to the newly-active tab.
    if (event.toPaneId === this.paneId) {
      this.tabs = this.workspaceService.getTabs(this.paneId);

      const active = this.workspaceService.getActiveTab(this.paneId);
      if (active?.id === event.tabId) {
        const t = this.tabs.find(tab => tab.id === event.tabId);
        if (t) void this.onTabChange(t);
      }
    }
  }

  onTabDragStart(tab: TabInfo): void {
    // Snapshot current tab state before it potentially leaves this pane.
    if (!tab || tab.id !== this.currentTabId) return;
    if (!this.currentTabId || !this.currentPath) return;

    if (this.isInArchive()) {
      this.workspaceService.saveArchiveState(this.paneId, this.currentTabId, {
        path: this.currentPath,
        archiveStack: this.archiveStack,
        fsDirPath: this.archiveStack[0]?.fsPath ?? '',
        fsParentDir: this.fsParentDir,
      });
    } else {
      this.tabPaths.set(this.currentTabId, this.currentPath);
      this.workspaceService.updateTabPath(this.paneId, this.currentTabId, this.currentPath);
    }
  }

  private setupOptimisticDnDListeners(): void {
    this.optimisticHideListener = (ev: Event) => {
      const detail = (ev as CustomEvent).detail as { paneId?: 'left' | 'right'; paths?: string[] } | undefined;
      if (!detail?.paneId || detail.paneId !== this.paneId) return;
      const paths = (detail.paths ?? []).filter(p => p && p !== PARENT_ENTRY_PATH);
      if (!paths.length) return;

      for (const p of paths) this.optimisticHiddenPaths.add(p);
      for (const p of paths) this.selectedFiles.delete(p);
      this.selectedFiles = new Set(this.selectedFiles);
      this.applyFilter();
    };

    this.optimisticRestoreListener = (ev: Event) => {
      const detail = (ev as CustomEvent).detail as { paneId?: 'left' | 'right'; paths?: string[] } | undefined;
      if (!detail?.paneId || detail.paneId !== this.paneId) return;
      const paths = (detail.paths ?? []).filter(p => p && p !== PARENT_ENTRY_PATH);
      if (!paths.length) return;

      for (const p of paths) this.optimisticHiddenPaths.delete(p);
      this.applyFilter();
    };

    window.addEventListener('pane:dnd-optimistic-hide', this.optimisticHideListener);
    window.addEventListener('pane:dnd-optimistic-restore', this.optimisticRestoreListener);
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

    const savedDirsFirst = localStorage.getItem('fileManager_directoriesFirst');
    if (savedDirsFirst === 'true' || savedDirsFirst === 'false') {
      this.directoriesFirst = savedDirsFirst === 'true';
    }
  }

  private saveViewPreference(): void {
    localStorage.setItem('fileManager_viewMode', this.currentView);
    localStorage.setItem('fileManager_sortField', this.sortField);
    localStorage.setItem('fileManager_sortDirection', this.sortDirection);
    localStorage.setItem('fileManager_directoriesFirst', String(this.directoriesFirst));
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

  onDirectoriesFirstChange(value: boolean): void {
    this.directoriesFirst = value;
    this.saveViewPreference();
    this.applyFilter();
  }

  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void {
    // Handle context menu - can integrate with existing context menu system
    console.log('Context menu for:', event.entry.name);
  }

  onNodeDoubleClick(entry: FileInfo): void {
    if (entry.path === PARENT_ENTRY_PATH) { this.onNavigateUp(); return; }
    // In tree view, non-directory double-clicks must bypass onNavigate (which
    // now returns early to update the address bar). Handle open actions here.
    if (this.currentView === 'tree' && entry.type !== FileType.DIRECTORY) {
      if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
        this.openRootArchive(entry.path);
        return;
      }
      if (entry.name.toLowerCase().endsWith('.lnk') || entry.type === FileType.SYMLINK) {
        this.resolveAndOpen(entry.path);
        return;
      }
      this.apiService.openFile(entry.path).catch(err =>
        console.error('Failed to open file:', err)
      );
      return;
    }
    this.onNavigate(entry);
  }

  /** Resolves a .lnk or symlink and either navigates into (dir) or opens (file). */
  private resolveAndOpen(path: string): void {
    this.apiService.resolveLink(path).then(result => {
      if (result.isDir) {
        this.loadDirectory(result.target);
      } else {
        this.apiService.openFile(result.target).catch(err =>
          console.error('Failed to open link target:', err)
        );
      }
    }).catch(err => {
      // Fallback: let the OS handle it
      console.warn('Could not resolve link, falling back to OS open:', err);
      this.apiService.openFile(path).catch(e =>
        console.error('Failed to open file:', e)
      );
    });
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

  private pathIsInsideArchive(path: string): boolean {
    const { fsPath } = parseArchivePath(path);
    const name = fsPath.replace(/\\/g, '/').split('/').filter(Boolean).pop() ?? '';
    return isArchive(name);
  }

  private async loadPathTransparently(path: string): Promise<void> {
    if (this.pathIsInsideArchive(path)) {
      await this.restoreArchiveFromPath(path);
      return;
    }

    await this.loadDirectory(path);
  }

  private async refreshCurrentPathTransparently(): Promise<void> {
    if (!this.currentPath) return;

    if (this.isInArchive()) {
      const currentFrame = this.archiveStack[this.archiveStack.length - 1];
      if (currentFrame) {
        this.renderArchiveFrame(currentFrame);
        return;
      }
    }

    if (this.pathIsInsideArchive(this.currentPath)) {
      await this.restoreArchiveFromPath(this.currentPath);
      return;
    }

    await this.loadDirectory(this.currentPath, { preserveUI: true });
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

  private async loadDirectory(path: string, opts?: { preserveUI?: boolean }): Promise<void> {
    await this.cancelActiveDirectoryStream();
    this.exitArchive();
    this.clearArchiveState();
    this.activeDirStreamPreserveUI = !!opts?.preserveUI;
    this.loading = true;
    this.error   = null;
    this.streamBaseEntries = [];

    if (!this.activeDirStreamPreserveUI) {
      this.selectedFiles.clear();
      this.entries = this.withParentEntry([], false, path);
      this.applyFilter();
    }
    try {
      const streamId = await this.apiService.startListDirectoryStream(this.paneId, path, this.showHidden);
      this.activeDirStreamId = streamId;
      this.streamPathChangeEmitted = false;
      await this.waitForDirectoryStream(streamId);
    } catch (error: any) {
      if (error?.message === 'Directory stream cancelled') {
        return;
      }
      this.error = error.message || 'Failed to load directory';
    } finally {
      this.activeDirStreamId = null;
      this.streamBaseEntries = [];
      this.activeDirStreamPreserveUI = false;
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
      // parseArchivePath correctly splits into:
      //   fsPath      – the on-disk archive file (e.g. "C:/foo/archive.zip")
      //   nestedChain – nested archive segments inside (e.g. ["folder/inner.zip"])
      //   innerPath   – regular subdirectory to navigate into (e.g. "subdir")
      const { fsPath, nestedChain, innerPath } = parseArchivePath(path);

      // 1. Open root archive
      let frame = await this.archiveService.openRootArchive(fsPath);
      const stack: ArchiveFrame[] = [frame];

      // 2. Open each nested archive level using cumulative chains
      for (let i = 0; i < nestedChain.length; i++) {
        frame = await this.archiveService.openNestedArchive(frame, nestedChain.slice(0, i + 1));
        stack.push(frame);
      }

      // 3. Navigate into the inner subdirectory (if any) within the deepest archive
      if (innerPath) {
        frame = await this.archiveService.navigateInto(frame, innerPath);
        stack[stack.length - 1] = frame;
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
        await this.loadDirectory(listing.path);
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

  private bindFsChangeEvents(): void {
    EventsOn('fs:directoryChanged', (event: { paneId?: string; path?: string } | null) => {
      this.handleFsDirectoryChanged(event);
    });
    EventsOn('fs:listStream:meta', (event: DirStreamMetaEvent | null) => {
      this.handleDirStreamMeta(event);
    });
    EventsOn('fs:listStream:batch', (event: DirStreamBatchEvent | null) => {
      this.handleDirStreamBatch(event);
    });
    EventsOn('fs:listStream:done', (event: DirStreamDoneEvent | null) => {
      this.handleDirStreamDone(event);
    });
    EventsOn('fs:listStream:error', (event: DirStreamErrorEvent | null) => {
      this.handleDirStreamError(event);
    });
  }

  private unbindFsChangeEvents(): void {
    if (this.fsChangeDebounceTimer) {
      clearTimeout(this.fsChangeDebounceTimer);
      this.fsChangeDebounceTimer = null;
    }
    void this.cancelActiveDirectoryStream();
    this.rejectAllDirectoryStreamWaiters(new Error('Pane disposed'));
    EventsOff(
      'fs:directoryChanged',
      'fs:listStream:meta',
      'fs:listStream:batch',
      'fs:listStream:done',
      'fs:listStream:error',
    );
  }

  private handleFsDirectoryChanged(event: { paneId?: string; path?: string } | null): void {
    if (!event || event.paneId !== this.paneId) return;
    if (this.isInArchive()) return;

    const current = (this.currentPath || '').replace(/\\/g, '/');
    const changed = (event.path || '').replace(/\\/g, '/');
    if (!current || !changed || current !== changed) return;

    if (this.fsChangeDebounceTimer) {
      clearTimeout(this.fsChangeDebounceTimer);
    }
    this.fsChangeDebounceTimer = setTimeout(() => {
      this.fsChangeDebounceTimer = null;
      this.onRefresh();
    }, 150);
  }

  private handleDirStreamMeta(event: DirStreamMetaEvent | null): void {
    if (!event || event.paneId !== this.paneId) return;
    if (!this.activeDirStreamId || event.streamId !== this.activeDirStreamId) return;

    this.currentPath = event.path;
    this.fsParentDir = event.path;
    this.tabPaths.set(this.currentTabId, event.path);
    this.workspaceService.updateTabPath(this.paneId, this.currentTabId, event.path);
    this.pathHistoryService.addPath(event.path, this.paneId);
    if (!this.streamPathChangeEmitted) {
      this.pathChange.emit(event.path);
      this.streamPathChangeEmitted = true;
    }
    if (!this.activeDirStreamPreserveUI) {
      this.entries = this.withParentEntry([], false, event.path);
      this.applyFilter();
    }

    // best-effort pane watch sync
    void window.go?.main?.App?.SetPaneDirectory?.(this.paneId, event.path);
  }

  private handleDirStreamBatch(event: DirStreamBatchEvent | null): void {
    if (!event || event.paneId !== this.paneId) return;
    if (!this.activeDirStreamId || event.streamId !== this.activeDirStreamId) return;

    this.streamBaseEntries = this.streamBaseEntries.concat(event.entries || []);
    this.entries = this.withParentEntry(this.streamBaseEntries, false, this.currentPath || event.path);
    this.applyFilter();
  }

  private handleDirStreamDone(event: DirStreamDoneEvent | null): void {
    if (!event || event.paneId !== this.paneId) return;
    if (!this.activeDirStreamId || event.streamId !== this.activeDirStreamId) return;

    this.entries = this.withParentEntry(this.streamBaseEntries, false, event.path || this.currentPath);
    // Prune selection to keep it stable across refresh.
    if (this.selectedFiles.size > 0) {
      const present = new Set(this.entries.map(e => e.path));
      for (const p of Array.from(this.selectedFiles)) {
        if (!present.has(p)) this.selectedFiles.delete(p);
      }
      this.selectedFiles = new Set(this.selectedFiles);
    }
    this.applyFilter();
    this.resolveDirectoryStreamWaiter(event.streamId);
    setTimeout(() => {
      try { this.fileViewContainer?.treeComponent?.focusNode(event.path); } catch { /* best-effort */ }
    }, 0);
  }

  private handleDirStreamError(event: DirStreamErrorEvent | null): void {
    if (!event || event.paneId !== this.paneId) return;
    if (!this.activeDirStreamId || event.streamId !== this.activeDirStreamId) return;

    this.error = event.error || 'Failed to load directory';
    this.rejectDirectoryStreamWaiter(event.streamId, new Error(this.error));
  }

  private waitForDirectoryStream(streamId: string): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      this.streamWaiters.set(streamId, { resolve, reject });
    });
  }

  private resolveDirectoryStreamWaiter(streamId: string): void {
    const waiter = this.streamWaiters.get(streamId);
    if (!waiter) return;
    this.streamWaiters.delete(streamId);
    waiter.resolve();
  }

  private rejectDirectoryStreamWaiter(streamId: string, err: Error): void {
    const waiter = this.streamWaiters.get(streamId);
    if (!waiter) return;
    this.streamWaiters.delete(streamId);
    waiter.reject(err);
  }

  private rejectAllDirectoryStreamWaiters(err: Error): void {
    for (const [id, waiter] of this.streamWaiters.entries()) {
      this.streamWaiters.delete(id);
      waiter.reject(err);
    }
  }

  private async cancelActiveDirectoryStream(): Promise<void> {
    if (!this.activeDirStreamId) return;
    const streamId = this.activeDirStreamId;
    this.activeDirStreamId = null;
    this.streamBaseEntries = [];
    this.rejectDirectoryStreamWaiter(streamId, new Error('Directory stream cancelled'));
    try {
      await this.apiService.cancelListDirectoryStream(streamId);
    } catch {
      // best effort
    }
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
    // For file/file-symlink/archive nodes in tree view: update the address bar
    // to the parent directory without opening the item (opening is handled via
    // the nodeDoubleClick path in onNodeDoubleClick).
    // Dir-symlinks have type=DIRECTORY and are handled by the block above.
    if (this.currentView === 'tree' && (entry.type === FileType.FILE || entry.type === FileType.SYMLINK || entry.type === FileType.ARCHIVE)) {
      const parent = parentPath(entry.path);
      this.currentPath = parent;
      this.pathChange.emit(parent);
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
      return;
    }
    // .lnk Shell Links (Windows) and unresolved Unix file-symlinks
    if (entry.name.toLowerCase().endsWith('.lnk') || entry.type === FileType.SYMLINK) {
      this.resolveAndOpen(entry.path);
      return;
    }
    // Regular file — open with default OS application
    this.apiService.openFile(entry.path).catch(err =>
      console.error('Failed to open file:', err)
    );
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
      this.confirmDialog.alert('Archive Drag & Drop Not Supported\nUse copy/move to other pane instead.');
      return;
    }

    const sourcePaths = event.source
      .map(e => e.path)
      .filter(p => p && p !== PARENT_ENTRY_PATH);

    if (!sourcePaths.length) return;

    try {
      const isCopy = event.event.ctrlKey || event.event.metaKey;
      const sourcePaneIdRaw = event.event.dataTransfer?.getData('application/x-wails-pane-id') || '';
      const sourcePaneId = (sourcePaneIdRaw === 'left' || sourcePaneIdRaw === 'right') ? sourcePaneIdRaw : null;
      const isCrossPane = !!sourcePaneId && sourcePaneId !== this.paneId;

      const items = await Promise.all(sourcePaths.map(async sourcePath => ({
        source: sourcePath,
        destination: await this.buildDestinationPath(sourcePath, event.targetPath),
      })));

      if (!isCopy) {
        const moveItems = items.filter(i => normalizePath(i.source) !== normalizePath(i.destination));
        if (moveItems.length === 0) return;

        // Cross-pane move UX: hide in source pane BEFORE starting the move.
        if (isCrossPane) {
          try {
            window.dispatchEvent(new CustomEvent('pane:dnd-optimistic-hide', {
              detail: { paneId: sourcePaneId, paths: moveItems.map(i => i.source) }
            }));
          } catch {
            // ignore
          }
        }

        await this.apiService.batchMove(moveItems, true);
        return;
      }

      await this.apiService.batchCopy(items, true);
    } catch (error: any) {
      // If we optimistically hid items (cross-pane move), restore them on immediate failure.
      try {
        const isCopy = event.event.ctrlKey || event.event.metaKey;
        const sourcePaneIdRaw = event.event.dataTransfer?.getData('application/x-wails-pane-id') || '';
        const sourcePaneId = (sourcePaneIdRaw === 'left' || sourcePaneIdRaw === 'right') ? sourcePaneIdRaw : null;
        const isCrossPane = !!sourcePaneId && sourcePaneId !== this.paneId;
        if (!isCopy && isCrossPane) {
          window.dispatchEvent(new CustomEvent('pane:dnd-optimistic-restore', {
            detail: { paneId: sourcePaneId, paths: sourcePaths }
          }));
        }
      } catch {
        // ignore
      }
      this.confirmDialog.alert(`Drag & Drop Failed\n${error.message}`);
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
    // Prune any optimistic hides that are no longer present in the current listing.
    if (this.optimisticHiddenPaths.size > 0) {
      const present = new Set(this.entries.map(e => e.path));
      for (const p of Array.from(this.optimisticHiddenPaths)) {
        if (!present.has(p)) this.optimisticHiddenPaths.delete(p);
      }
    }

    const baseEntries = this.optimisticHiddenPaths.size > 0
      ? this.entries.filter(e => e.path === PARENT_ENTRY_PATH || !this.optimisticHiddenPaths.has(e.path))
      : this.entries;

    const q = this.filterQuery;
    if (!q?.text?.trim() && !q?.criteria?.length) {
      this.filteredEntries = baseEntries;
      return;
    }
    // Always keep the ".." entry visible regardless of the active filter.
    const parentEntry = baseEntries.filter(e => e.path === PARENT_ENTRY_PATH);
    const filtered    = this.filterService.filterEntries(
      baseEntries.filter(e => e.path !== PARENT_ENTRY_PATH),
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
    if (this.isInArchive()) { this.confirmDialog.alert('Archive Limitation\nCannot create folders inside an archive.'); return; }
    const folderName = prompt('Enter folder name:');
    if (!folderName) return;
    try {
      const newPath: string = await window.go.main.App.JoinPaths([this.currentPath, folderName]);
      await this.apiService.createDirectory(newPath, false);
      await this.loadDirectory(this.currentPath);
    } catch (error: any) { this.confirmDialog.alert(`Create Folder Failed\n${error.message}`); }
  }

  async onDeleteSelected(): Promise<void> {
    if (this.selectedFiles.size === 0) return;

    if (this.isInArchive()) {
      const frame   = this.archiveStack[this.archiveStack.length - 1];
      const entries = Array.from(this.selectedFiles);

      if (frame.nestedChain.length > 0) {
        this.confirmDialog.alert(
          'Nested Archive Limitation\n' +
          'Exit back to the top-level archive first.',
        );
        return;
      }

      const archiveDeleteConfirmed = await this.confirmDialog.confirm({
        title: 'Delete from Archive',
        message:
          `Delete ${entries.length} item(s) from the archive?\n` +
          `This rewrites the archive file on disk and cannot be undone.`,
        confirmText: 'Delete',
        danger: true,
      });
      if (!archiveDeleteConfirmed) return;

      try {
        const result = await this.archiveService.delete(frame, entries);
        if (result.failed > 0) {
          this.confirmDialog.alert('Delete Failed\n' + (result.errors ?? []).map((e: any) => e.error).join('\n'));
        }
      } catch (err: any) {
        this.confirmDialog.alert(`Delete Error\n${err.message}`);
        return;
      }
      // Refresh the archive listing so deleted entries disappear
      await this.refreshArchive();
      return;
    }

    // ── Filesystem delete — unchanged from original ───────────────────────────
    const deleteConfirmed = await this.confirmDialog.confirm({
      title: 'Delete Items',
      message: `Delete ${this.selectedFiles.size} item(s)?`,
      confirmText: 'Delete',
      danger: true,
    });
    if (!deleteConfirmed) return;
    try {
      const result = await this.apiService.batchDelete(Array.from(this.selectedFiles), true);
      if (result.failed > 0) {
        const msg = (result.errors ?? []).map((e: any) => `${e.source}: ${e.error}`).join('\n');
        this.confirmDialog.alert(`Delete Completed With Errors\n${result.failed} item(s) failed:\n${msg}`);
      }
      await this.loadDirectory(this.currentPath);
    } catch (error: any) {
      this.confirmDialog.alert(`Delete Failed\n${error.message}`);
    }
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
      this.confirmDialog.alert('Archive Limitation\nRename is not supported inside archives.');
      return;
    }

    if (entry.path === PARENT_ENTRY_PATH) return;

    const nextName = (forcedName ?? prompt('Rename to:', entry.name) ?? '').trim();
    if (!nextName || nextName === entry.name) return;

    try {
      const destination = await this.apiService.joinPaths([parentPath(entry.path), nextName]);
      await this.apiService.batchMove([{ source: entry.path, destination }], true);
    } catch (error: any) {
      this.confirmDialog.alert(`Rename Failed\n${error.message}`);
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
        this.confirmDialog.alert(
          `Move Partially Complete\n` +
          `${result.succeeded} file(s) extracted, but source NOT removed:\n` +
          (softMsg?.error ?? 'Nested archive deletion is not supported.'),
        );
        return;
      }

      if (result.failed > 0) {
        this.confirmDialog.alert('Move Errors\n' + (result.errors ?? []).map((e: any) => e.error).join('\n'));
      }

      // Refresh archive listing — moved entries have been deleted from it
      await this.refreshArchive();
    } catch (err: any) {
      this.confirmDialog.alert(`Move Failed\n${err.message}`);
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
