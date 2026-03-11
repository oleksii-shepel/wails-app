// file-pane.component.ts
import { Component, Input, Output, EventEmitter, OnInit, OnChanges, OnDestroy, ViewChild, SimpleChanges, NgZone, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TabBarComponent, type TabDropEvent } from './tab-bar.component';
import { FindWidgetState } from './filter-widget.component';
import { FileInfo, FileType } from '../shared/protocol';
import { TabInfo, WorkspaceConfig, FilterQuery } from '../shared/protocol-enhanced';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import { ApiService } from '../services/api.service';
import { WorkspaceService } from '../services/workspace.service';
import { FilterService } from '../services/filter.service';
import { PathHistoryService } from '../services/path-history.service';
import {
  ArchiveService, ArchiveFrame, isArchive,
  parseArchivePath, buildArchivePath
} from '../services/archive.service';
import { parentPath, normalizePath } from './address-bar.utils';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { FileViewContainerComponent, ViewLayout } from './file-view-container.component';
import { ViewerArchiveContext } from './viewer-surface.component';
import { DirSurfaceVm } from '../surfaces/dir/dir-surface.vm';
import { DirSurfaceSession } from '../surfaces/dir/dir-surface-session';
import { DirSurfaceRuntime } from '../surfaces/dir/dir-surface-runtime';
import { FilePaneDirSurfaceRuntime } from '../surfaces/dir/file-pane-dir-surface-runtime';
import { DirTabStateController, type DirTabRuntime } from '../surfaces/dir/dir-tab-state-controller';
import { SurfaceContext } from '../surfaces/surface-context';
import { SurfaceOutletComponent } from '../surfaces/surface-outlet.component';
import { FilePaneArchiveController, type FilePaneArchiveHost } from './file-pane-archive.controller';
import { FilePaneNavigationController, type FilePaneNavigationHost } from './file-pane-navigation.controller';
import { FilePaneOpenController, type FilePaneOpenHost } from './file-pane-open.controller';
import { FilePaneOperationsController, type FilePaneOperationsHost } from './file-pane-operations.controller';
import { FilePanePreviewController, type FilePanePreviewHost } from './file-pane-preview.controller';
import { FilePaneStateController, type FilePaneStateHost } from './file-pane-state.controller';
import {
  DirListStreamController,
  DirStreamMetaEvent,
  DirStreamBatchEvent,
  DirStreamDoneEvent,
  DirStreamErrorEvent,
} from '../surfaces/dir/dir-list-stream.controller';
import { EventsOn, EventsOff } from 'wailsjs/runtime';
import { ConfirmDialogService } from '../services/confirm-dialog.service';

declare const window: any;

@Component({
  selector: 'app-file-pane',
  standalone: true,
  imports: [
    CommonModule,
    TabBarComponent,
    SurfaceOutletComponent
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

      <ng-container *ngIf="surfaceCtx as ctx">
        <app-surface-outlet
          #surfaceOutlet
          [ctx]="ctx">
        </app-surface-outlet>
      </ng-container>
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

    app-tab-bar {
      display: block;
      flex-shrink: 0;
    }

    app-surface-outlet {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }
  `]
})
export class FilePaneComponent implements OnInit, OnChanges, OnDestroy {
  private readonly dirStream: DirListStreamController;
  private readonly dirSessions = new Map<string, DirSurfaceSession>();
  private readonly dirTabStateController = new DirTabStateController();
  private activeDirTabId: string | null = null;
  readonly dirVm: DirSurfaceVm = new (class implements DirSurfaceVm {
    constructor(private pane: FilePaneComponent) {}

    get paneId() { return this.pane.paneId; }
    get pathReady() { return this.pane.pathReady; }
    get currentPath() { return this.pane.currentPath; }
    get showHidden() { return this.pane.showHidden; }
    get paneActive() { return this.pane.paneActive; }

    get entries() { return this.pane.filteredEntries; }
    get selectedFiles() { return this.pane.selectedFiles; }
    get cursorPath() { return this.pane.selectionCursorPath; }
    get error() { return this.pane.error; }

    get currentView() { return this.pane.currentView; }
    get sortField() { return this.pane.sortField; }
    get sortDirection() { return this.pane.sortDirection; }
    get directoriesFirst() { return this.pane.directoriesFirst; }

    get archiveMoveWarning() { return this.pane.archiveMoveWarning; }
    get archiveMoveEntryCount() { return this.pane.archivePendingMoveEntries.length; }

    onPathChange = (path: string) => this.pane.onPathChange(path);
    onNavigateUp = () => this.pane.onNavigateUp();
    onRefresh = () => this.pane.onRefresh();

    onViewChange = (view: ViewLayout) => this.pane.onViewChange(view);
    onSortChange = (sort: { field: string; direction: 'asc' | 'desc' }) => this.pane.onSortChange(sort);
    onDirectoriesFirstChange = (v: boolean) => this.pane.onDirectoriesFirstChange(v);

    onCreateFolder = () => this.pane.onCreateFolder();
    onDeleteSelected = () => this.pane.onDeleteSelected();
    onCopyToOther = () => this.pane.onCopyToOther();
    onMoveToOther = () => this.pane.onMoveToOther();

    onFindStateChange = (state: FindWidgetState | null) => this.pane.onFindStateChange(state);
    onFindNext = () => this.pane.onFindNext();
    onFindPrev = () => this.pane.onFindPrev();

    onSelectionChange = (entries: FileInfo[]) => this.pane.onSelectionChange(entries);
    onNavigate = (entry: FileInfo) => this.pane.onNavigate(entry);
    onOpenInNewTab = (entry: FileInfo) => this.pane.onOpenInNewTab(entry);
    onContextMenu = (ev: { event: MouseEvent; entry: FileInfo }) => this.pane.onContextMenu(ev);
    onNodeDoubleClick = (ev: any) => this.pane.onNodeDoubleClick(ev);
    onDragDrop = (ev: any) => this.pane.onDragDrop(ev);
    onRename = (ev: { entry: FileInfo; newName: string }) => this.pane.onRename(ev);

    onArchiveMoveConfirmed = () => this.pane.executePendingMove();
    onArchiveMoveCancelled = () => { this.pane.archiveMoveWarning = false; };
  })(this);
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
  @ViewChild('surfaceOutlet') surfaceOutlet?: SurfaceOutletComponent;

  private get addressBar() { return this.surfaceOutlet?.addressBar; }
  private get fileViewContainer() { return this.surfaceOutlet?.fileViewContainer; }
  private get findWidget() { return this.surfaceOutlet?.findWidget; }

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() workspace: WorkspaceConfig | null = null;
  @Input() showHidden = false;
  @Input() paneActive = true;

  @Output() pathChange    = new EventEmitter<string>();
  @Output() copyToOther   = new EventEmitter<string[]>();
  @Output() moveToOther   = new EventEmitter<string[]>();
  @Output() previewTargetChange = new EventEmitter<string>();
  @Output() activeTabChange = new EventEmitter<TabInfo>();

  currentTabId  = '';
  pathReady     = false;
  tabs: TabInfo[] = [];
  private optimisticHideListener?: (event: Event) => void;
  private optimisticRestoreListener?: (event: Event) => void;

  archiveStack: ArchiveFrame[] = [];

  // Archive move state
  archiveMoveWarning = false;
  archivePendingMoveEntries: string[] = [];
  archivePendingMoveDestDir = '';
  private fsChangeDebounceTimer: ReturnType<typeof setTimeout> | null = null;
  private activeDirStreamPreserveUI = false;
  private pendingFocusPath: string | null = null;

  private destroy$ = createSubject<void>();
  private lastSeenActiveTabId: string | null = null;
  private lastSeenActiveTabPath: string | null = null;

  FileType = FileType;

  // Minimal UI runtime adapter for dir sessions (tree vs list/grid/details).
  private readonly dirRuntime: DirSurfaceRuntime = new FilePaneDirSurfaceRuntime(
    () => this.currentView,
    () => this.fileViewContainer,
    () => this.filteredEntries,
    PARENT_ENTRY_PATH,
  );
  private readonly navigationController = new FilePaneNavigationController();
  private readonly archiveController = new FilePaneArchiveController();
  private readonly openController = new FilePaneOpenController();
  private readonly operationsController = new FilePaneOperationsController();
  private readonly previewController = new FilePanePreviewController();
  private readonly stateController = new FilePaneStateController(
    PARENT_ENTRY_PATH,
    FileType.FILE,
    FileType.SYMLINK,
  );

  private get navigationHost(): FilePaneNavigationHost {
    const thisRef = this;
    return {
      get paneId() { return thisRef.paneId; },
      get workspace() { return thisRef.workspace; },
      get showHidden() { return thisRef.showHidden; },
      get currentTabId() { return thisRef.currentTabId; },
      set currentTabId(value: string) { thisRef.currentTabId = value; },
      get currentPath() { return thisRef.currentPath; },
      set currentPath(value: string) { thisRef.currentPath = value; },
      get pathReady() { return thisRef.pathReady; },
      set pathReady(value: boolean) { thisRef.pathReady = value; },
      get tabs() { return thisRef.tabs; },
      set tabs(value: TabInfo[]) { thisRef.tabs = value; },
      get activeDirTabId() { return thisRef.activeDirTabId; },
      set activeDirTabId(value: string | null) { thisRef.activeDirTabId = value; },
      get archiveStack() { return thisRef.archiveStack; },
      set archiveStack(value: ArchiveFrame[]) { thisRef.archiveStack = value; },
      apiService: this.apiService,
      workspaceService: this.workspaceService,
      getDirSession: () => this.dir,
      isInArchive: () => this.isInArchive(),
      clearArchiveState: () => this.clearArchiveState(),
      pathIsInsideArchive: (path: string) => this.pathIsInsideArchive(path),
      loadPathTransparently: (path: string) => this.loadPathTransparently(path),
      renderArchiveFrame: (frame: ArchiveFrame) => this.renderArchiveFrame(frame),
      applyFilter: () => this.applyFilter(),
      emitPreviewTargetFromState: () => this.emitPreviewTargetFromState(),
      emitActiveTabChange: (tab: TabInfo) => this.activeTabChange.emit(tab),
    };
  }

  private get stateHost(): FilePaneStateHost {
    const thisRef = this;
    return {
      get entries() { return thisRef.entries; },
      set entries(value: FileInfo[]) { thisRef.entries = value; },
      get filteredEntries() { return thisRef.filteredEntries; },
      set filteredEntries(value: FileInfo[]) { thisRef.filteredEntries = value; },
      get selectedFiles() { return thisRef.selectedFiles; },
      set selectedFiles(value: Set<string>) { thisRef.selectedFiles = value; },
      get selectionCursorPath() { return thisRef.selectionCursorPath; },
      set selectionCursorPath(value: string | null) { thisRef.selectionCursorPath = value; },
      get selectionAnchorPath() { return thisRef.selectionAnchorPath; },
      set selectionAnchorPath(value: string | null) { thisRef.selectionAnchorPath = value; },
      get filterQuery() { return thisRef.filterQuery; },
      set filterQuery(value: FilterQuery) { thisRef.filterQuery = value; },
      get currentView() { return thisRef.currentView; },
      set currentView(value: ViewLayout) { thisRef.currentView = value; },
      get lastPreviewTargetEmitted() { return thisRef.dir.lastPreviewTargetEmitted; },
      set lastPreviewTargetEmitted(value: string | null) { thisRef.dir.lastPreviewTargetEmitted = value; },
      isInArchive: () => thisRef.isInArchive(),
      getSelectableEntries: () => thisRef.dirRuntime.getSelectableEntries(),
      getOptimisticHiddenPaths: () => thisRef.getOptimisticHiddenPaths(),
      setOptimisticHiddenPaths: (paths: string[]) => thisRef.setOptimisticHiddenPaths(paths),
      filterEntries: (entries: FileInfo[], query: FilterQuery) => thisRef.filterService.filterEntries(entries, query),
      findEntry: (path: string) => thisRef.entries.find(entry => entry.path === path),
      emitPreviewTarget: (path: string) => thisRef.previewTargetChange.emit(path),
      scrollToCursorPath: (path: string) => thisRef.scrollToCursorPath(path),
      getGridColumns: () => thisRef.fileViewContainer?.getGridColumns() ?? 1,
      getApproxPageSize: () => thisRef.fileViewContainer?.getApproxPageSize() ?? 10,
      persistDirTabState: () => thisRef.persistDirTabState(),
    };
  }

  private get archiveHost(): FilePaneArchiveHost {
    const thisRef = this;
    return {
      get paneId() { return thisRef.paneId; },
      get currentTabId() { return thisRef.currentTabId; },
      get currentPath() { return thisRef.currentPath; },
      set currentPath(value: string) { thisRef.currentPath = value; },
      get fsParentDir() { return thisRef.fsParentDir; },
      set fsParentDir(value: string) { thisRef.fsParentDir = value; },
      get archiveStack() { return thisRef.archiveStack; },
      set archiveStack(value: ArchiveFrame[]) { thisRef.archiveStack = value; },
      get entries() { return thisRef.entries; },
      set entries(value: FileInfo[]) { thisRef.entries = value; },
      get selectedFiles() { return thisRef.selectedFiles; },
      set selectedFiles(value: Set<string>) { thisRef.selectedFiles = value; },
      get loading() { return thisRef.loading; },
      set loading(value: boolean) { thisRef.loading = value; },
      get error() { return thisRef.error; },
      set error(value: string | null) { thisRef.error = value; },
      get tabs() { return thisRef.tabs; },
      set tabs(value: TabInfo[]) { thisRef.tabs = value; },
      get archiveMoveWarning() { return thisRef.archiveMoveWarning; },
      set archiveMoveWarning(value: boolean) { thisRef.archiveMoveWarning = value; },
      get archivePendingMoveEntries() { return thisRef.archivePendingMoveEntries; },
      set archivePendingMoveEntries(value: string[]) { thisRef.archivePendingMoveEntries = value; },
      get archivePendingMoveDestDir() { return thisRef.archivePendingMoveDestDir; },
      set archivePendingMoveDestDir(value: string) { thisRef.archivePendingMoveDestDir = value; },
      archiveService: this.archiveService,
      workspaceService: this.workspaceService,
      confirmDialog: this.confirmDialog,
      withParentEntry: (raw: FileInfo[], inArchive: boolean, fsPath: string) => thisRef.withParentEntry(raw, inArchive, fsPath),
      applyFilter: () => thisRef.applyFilter(),
      loadDirectory: (path: string, opts?: { preserveUI?: boolean; focusPath?: string }) => thisRef.loadDirectory(path, opts),
      clearArchiveState: () => thisRef.clearArchiveState(),
      focusTreeNode: (path: string) => {
        try {
          thisRef.fileViewContainer?.treeComponent?.focusNode(path);
        } catch {
          // best-effort only
        }
      },
    };
  }

  private get operationsHost(): FilePaneOperationsHost {
    const thisRef = this;
    return {
      get paneId() { return thisRef.paneId; },
      get currentPath() { return thisRef.currentPath; },
      get currentView() { return thisRef.currentView; },
      get entries() { return thisRef.entries; },
      get selectedFiles() { return thisRef.selectedFiles; },
      apiService: this.apiService,
      confirmDialog: this.confirmDialog,
      isInArchive: () => thisRef.isInArchive(),
      deleteSelectedFromArchive: (effective: Set<string>) => thisRef.archiveController.deleteSelectedFromArchive(thisRef.archiveHost, effective),
      getEffectiveSelection: () => thisRef.getEffectiveSelection(),
      loadDirectory: (path: string) => thisRef.loadDirectory(path),
      emitCopyToOther: (paths: string[]) => thisRef.copyToOther.emit(paths),
      emitMoveToOther: (paths: string[]) => thisRef.moveToOther.emit(paths),
      startRenameSelected: () => thisRef.fileViewContainer?.startRenameSelected(),
      getVisibleTreeNodes: () => thisRef.fileViewContainer?.treeComponent?.getVisibleNodes() ?? [],
    };
  }

  private get previewHost(): FilePanePreviewHost {
    const thisRef = this;
    return {
      get paneId() { return thisRef.paneId; },
      get activeTabKind() { return thisRef.activeTabKind; },
      get entries() { return thisRef.entries; },
      get archiveStack() { return thisRef.archiveStack; },
      get tabs() { return thisRef.tabs; },
      set tabs(value: any[]) { thisRef.tabs = value as TabInfo[]; },
      apiService: this.apiService,
      workspaceService: this.workspaceService,
      confirmDialog: this.confirmDialog,
      isInArchive: () => thisRef.isInArchive(),
      getEffectiveSelection: () => thisRef.getEffectiveSelection(),
      onTabChange: (tab: any) => thisRef.onTabChange(tab as TabInfo),
    };
  }

  private get openHost(): FilePaneOpenHost {
    const thisRef = this;
    return {
      get currentPath() { return thisRef.currentPath; },
      set currentPath(value: string) { thisRef.currentPath = value; },
      get currentView() { return thisRef.currentView; },
      get entries() { return thisRef.entries; },
      apiService: this.apiService,
      isInArchive: () => thisRef.isInArchive(),
      exitArchive: () => thisRef.exitArchive(),
      clearArchiveState: () => thisRef.clearArchiveState(),
      archiveNavigateInto: (path: string) => thisRef.archiveNavigateInto(path),
      openNestedArchive: (chain: string[]) => thisRef.openNestedArchive(chain),
      loadDirectory: (path: string) => thisRef.loadDirectory(path),
      openRootArchive: (path: string) => thisRef.openRootArchive(path),
      emitPathChange: (path: string) => thisRef.pathChange.emit(path),
      getEffectiveSelection: () => thisRef.getEffectiveSelection(),
      openPathInNewTab: (path: string) => thisRef.openPathInNewTab(path),
    };
  }

  private get dir(): DirSurfaceSession {
    const id = this.activeDirTabId ?? this.currentTabId;
    let session = id ? this.dirSessions.get(id) : undefined;
    if (!session) {
      session = new DirSurfaceSession(id || 'default');
      // Default view prefs
      session.currentView = this.currentViewPrefFromStorage();
      const sf = localStorage.getItem('fileManager_sortField');
      const sd = localStorage.getItem('fileManager_sortDirection') as ('asc' | 'desc' | null);
      if (sf) session.sortField = sf;
      if (sd === 'asc' || sd === 'desc') session.sortDirection = sd;
      const df = localStorage.getItem('fileManager_directoriesFirst');
      if (df === 'true' || df === 'false') session.directoriesFirst = df === 'true';

      const savedState = this.dirTabStateController.load(session.tabId);
      if (savedState) {
        this.dirTabStateController.applyBeforeLoad(savedState, this.createDirTabRuntime(session));
      }

      this.dirSessions.set(session.tabId, session);
    }
    // If current tab is a dir tab, keep activeDirTabId aligned.
    const t = this.tabs.find(t => t.id === this.currentTabId);
    if ((t?.kind ?? 'dir') === 'dir') this.activeDirTabId = session.tabId;
    return session;
  }

  private createDirTabRuntime(session: DirSurfaceSession): DirTabRuntime {
    return {
      isInArchive: () => this.isInArchive(),
      getEntriesPaths: () => (session.entries ?? []).map(entry => entry.path),
      getSelectableEntriesPaths: () => this.dirRuntime.getSelectableEntries().map(entry => entry.path),
      applyFilter: () => this.applyFilter(),
      scrollToCursorPath: (path: string) => this.scrollToCursorPath(path),
      emitPreviewTargetFromState: () => this.emitPreviewTargetFromState(),
      getOptimisticHiddenPaths: () => Array.from(session.optimisticHiddenPaths ?? []),
      setOptimisticHiddenPaths: (paths: string[]) => {
        session.optimisticHiddenPaths = new Set((paths ?? []).filter(Boolean));
      },
      get currentPath() { return session.currentPath; },
      set currentPath(value: string) { session.currentPath = value; },
      get showHidden() { return !!session.showHiddenLoaded; },
      set showHidden(_value: boolean) {},
      get entries() { return session.entries; },
      set entries(value: FileInfo[]) { session.entries = value; },
      get currentView() { return session.currentView; },
      set currentView(value: ViewLayout) { session.currentView = value; },
      get sortField() { return session.sortField; },
      set sortField(value: string) { session.sortField = value; },
      get sortDirection() { return session.sortDirection; },
      set sortDirection(value: 'asc' | 'desc') { session.sortDirection = value; },
      get directoriesFirst() { return session.directoriesFirst; },
      set directoriesFirst(value: boolean) { session.directoriesFirst = value; },
      get filterQuery() { return session.filterQuery; },
      set filterQuery(value: FilterQuery) { session.filterQuery = value; },
      get selectedFiles() { return session.selectedFiles; },
      set selectedFiles(value: Set<string>) { session.selectedFiles = value; },
      get selectionCursorPath() { return session.selectionCursorPath; },
      set selectionCursorPath(value: string | null) { session.selectionCursorPath = value; },
      get selectionAnchorPath() { return session.selectionAnchorPath; },
      set selectionAnchorPath(value: string | null) { session.selectionAnchorPath = value; },
    };
  }

  private get activeDirTabStateRuntime(): DirTabRuntime {
    return this.createDirTabRuntime(this.dir);
  }

  private restoreActiveDirTabState(): void {
    const tabId = this.activeDirTabId ?? this.currentTabId;
    if (!tabId || this.activeTabKind !== 'dir' || this.isInArchive()) {
      return;
    }

    const savedState = this.dirTabStateController.load(tabId);
    if (!savedState) {
      return;
    }

    this.dirTabStateController.finalizeAfterLoad(savedState, this.activeDirTabStateRuntime);
  }

  private currentViewPrefFromStorage(): ViewLayout {
    const saved = localStorage.getItem('fileManager_viewMode') as ViewLayout | null;
    return (saved === 'list' || saved === 'grid' || saved === 'details' || saved === 'tree') ? saved : 'list';
  }

  // Dir state proxies (backed by the active dir session)
  get currentPath(): string { return this.dir.currentPath; }
  set currentPath(v: string) { this.dir.currentPath = v; }

  get fsParentDir(): string { return this.dir.fsParentDir; }
  set fsParentDir(v: string) { this.dir.fsParentDir = v; }

  get entries(): FileInfo[] { return this.dir.entries; }
  set entries(v: FileInfo[]) { this.dir.entries = v; }

  get filteredEntries(): FileInfo[] { return this.dir.filteredEntries; }
  set filteredEntries(v: FileInfo[]) { this.dir.filteredEntries = v; }

  get selectedFiles(): Set<string> { return this.dir.selectedFiles; }
  set selectedFiles(v: Set<string>) { this.dir.selectedFiles = v; }

  get loading(): boolean { return this.dir.loading; }
  set loading(v: boolean) { this.dir.loading = v; }

  get error(): string | null { return this.dir.error; }
  set error(v: string | null) { this.dir.error = v; }

  get filterQuery(): FilterQuery { return this.dir.filterQuery; }
  set filterQuery(v: FilterQuery) { this.dir.filterQuery = v; }

  get currentView(): ViewLayout { return this.dir.currentView; }
  set currentView(v: ViewLayout) { this.dir.currentView = v; }

  get sortField(): string { return this.dir.sortField; }
  set sortField(v: string) { this.dir.sortField = v; }

  get sortDirection(): 'asc' | 'desc' { return this.dir.sortDirection; }
  set sortDirection(v: 'asc' | 'desc') { this.dir.sortDirection = v; }

  get directoriesFirst(): boolean { return this.dir.directoriesFirst; }
  set directoriesFirst(v: boolean) { this.dir.directoriesFirst = v; }

  get selectionCursorPath(): string | null { return this.dir.selectionCursorPath; }
  set selectionCursorPath(v: string | null) { this.dir.selectionCursorPath = v; }

  get selectionAnchorPath(): string | null { return this.dir.selectionAnchorPath; }
  set selectionAnchorPath(v: string | null) { this.dir.selectionAnchorPath = v; }

  getOptimisticHiddenPaths(): string[] { return Array.from(this.dir.optimisticHiddenPaths); }
  setOptimisticHiddenPaths(paths: string[]): void {
    this.dir.optimisticHiddenPaths = new Set((paths ?? []).filter(Boolean));
  }

  get activeTabKind(): 'dir' | 'viewer' {
    const t = this.tabs.find(t => t.id === this.currentTabId);
    return (t?.kind ?? 'dir');
  }

  get activeViewerTargetPath(): string {
    const t = this.tabs.find(t => t.id === this.currentTabId);
    if (!t) return '';
    if ((t.kind ?? 'dir') !== 'viewer') return '';
    return (t.viewer?.targetPath ?? '');
  }

  get activeViewerId(): string | null {
    const t = this.tabs.find(t => t.id === this.currentTabId);
    if (!t || (t.kind ?? 'dir') !== 'viewer') return null;
    return (t.viewer?.viewerId ?? null);
  }

  get activeViewerArchiveContext(): ViewerArchiveContext | null {
    const t = this.tabs.find(t => t.id === this.currentTabId);
    if (!t || (t.kind ?? 'dir') !== 'viewer') return null;
    return (t.viewer?.archiveContext as ViewerArchiveContext) ?? null;
  }

  constructor(
    private apiService: ApiService,
    private workspaceService: WorkspaceService,
    private filterService: FilterService,
    private pathHistoryService: PathHistoryService,
    private confirmDialog: ConfirmDialogService,
    private zone: NgZone,
    private cdr: ChangeDetectorRef,
    private archiveService: ArchiveService
  ) {
    this.dirStream = new DirListStreamController(this.apiService);
  }

  get surfaceCtx(): SurfaceContext | null {
    const tab = this.tabs.find(t => t.id === this.currentTabId);
    if (!tab) return null;
    return {
      paneId: this.paneId,
      tab,
      dirVm: this.dirVm,
      viewer: {
        targetPath: this.activeViewerTargetPath,
        archiveContext: this.activeViewerArchiveContext,
        viewerId: this.activeViewerId,
      },
    };
  }

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

    if (changes['paneActive'] && this.paneActive && !changes['paneActive'].firstChange) {
      // Pane just became active вЂ” ensure there is a visible cursor.
      if (this.selectedFiles.size === 0 && !this.selectionCursorPath) {
        const selectable = this.dirRuntime.getSelectableEntries();
        if (selectable.length) {
          this.selectionCursorPath = selectable[0].path;
          this.selectionAnchorPath = selectable[0].path;
          // TC: just position cursor, don't mark the item
          this.fileViewContainer?.scrollToCursorPath(selectable[0].path);
        }
      }
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
      // Reorder/pin changes etc вЂ” just update the tab strip.
      this.tabs = this.workspaceService.getTabs(this.paneId);
      return;
    }

    // If we already navigated to the new active tab locally, avoid a redundant reload.
    if (this.currentTabId === nextActiveId && this.currentPath === nextActivePath) {
      this.tabs = this.workspaceService.getTabs(this.paneId);
      return;
    }

    // Active tab changed (including cross-pane drag) вЂ” load the new active path.
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
      this.workspaceService.updateTabPath(this.paneId, this.currentTabId, this.currentPath);
    }
  }

  private setupOptimisticDnDListeners(): void {
    this.optimisticHideListener = (ev: Event) => {
      const detail = (ev as CustomEvent).detail as { paneId?: 'left' | 'right'; paths?: string[] } | undefined;
      if (!detail?.paneId || detail.paneId !== this.paneId) return;
      const paths = (detail.paths ?? []).filter(p => p && p !== PARENT_ENTRY_PATH);
      if (!paths.length) return;

      for (const p of paths) this.dir.optimisticHiddenPaths.add(p);
      for (const p of paths) this.selectedFiles.delete(p);
      this.selectedFiles = new Set(this.selectedFiles);
      this.applyFilter();
    };

    this.optimisticRestoreListener = (ev: Event) => {
      const detail = (ev as CustomEvent).detail as { paneId?: 'left' | 'right'; paths?: string[] } | undefined;
      if (!detail?.paneId || detail.paneId !== this.paneId) return;
      const paths = (detail.paths ?? []).filter(p => p && p !== PARENT_ENTRY_PATH);
      if (!paths.length) return;

      for (const p of paths) this.dir.optimisticHiddenPaths.delete(p);
      this.applyFilter();
    };

    window.addEventListener('pane:dnd-optimistic-hide', this.optimisticHideListener);
    window.addEventListener('pane:dnd-optimistic-restore', this.optimisticRestoreListener);
  }

  get entriesForAnalysis(): FileInfo[] {
    return this.entries.filter(e => e.path !== PARENT_ENTRY_PATH);
  }

  // в”Ђв”Ђ View System Methods в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

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
    this.persistDirTabState();
  }

  onSortChange(sort: { field: string; direction: 'asc' | 'desc' }): void {
    this.sortField = sort.field;
    this.sortDirection = sort.direction;
    this.saveViewPreference();
    this.applyFilter();
    this.persistDirTabState();
  }

  onDirectoriesFirstChange(value: boolean): void {
    this.directoriesFirst = value;
    this.saveViewPreference();
    this.applyFilter();
    this.persistDirTabState();
  }

  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void {
    // Handle context menu - can integrate with existing context menu system
    console.log('Context menu for:', event.entry.name);
  }

  onNodeDoubleClick(entry: FileInfo): void {
    this.openController.onNodeDoubleClick(this.openHost, entry, () => this.onNavigateUp());
  }

  // в”Ђв”Ђ Virtual ".." entry helpers в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

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
   * вЂ“ Always prepended inside archives (so the user can navigate out).
   * вЂ“ Prepended for normal directories unless we are at the filesystem root.
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

  // в”Ђв”Ђ Pane state в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  private async loadPaneState(): Promise<void> {
    const host = this.navigationHost;
    await this.navigationController.loadPaneState(host);
    this.currentTabId = host.currentTabId;
    this.currentPath = host.currentPath;
    this.pathReady = host.pathReady;
    this.tabs = host.tabs;
    this.activeDirTabId = host.activeDirTabId;
    this.archiveStack = host.archiveStack;
    this.restoreActiveDirTabState();
  }

  private clearArchiveState(): void {
    this.archiveStack = [];
    this.fsParentDir  = '';
  }

  private async loadDirectory(path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void> {
    await this.dirStream.cancelActive();
    this.exitArchive();
    this.clearArchiveState();
    this.activeDirStreamPreserveUI = !!opts?.preserveUI;
    this.pendingFocusPath = opts?.focusPath ?? null;
    this.loading = true;
    this.error   = null;

    if (!this.activeDirStreamPreserveUI) {
      this.selectedFiles.clear();
      this.selectionCursorPath = null;
      this.selectionAnchorPath = null;
      this.entries = this.withParentEntry([], false, path);
      this.applyFilter();
    }
    try {
      const streamId = await this.dirStream.start(path, this.showHidden);
      await this.dirStream.waitFor(streamId);
    } catch (error: any) {
      if (error?.message === 'Directory stream cancelled') {
        return;
      }
      this.error = error.message || 'Failed to load directory';
    } finally {
      this.activeDirStreamPreserveUI = false;
      this.pendingFocusPath = null;
      this.loading = false;
    }
  }

  // в”Ђв”Ђ Tab management в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  async onTabChange(tab: TabInfo): Promise<void> {
    const host = this.navigationHost;
    await this.navigationController.onTabChange(host, tab);
    this.currentTabId = host.currentTabId;
    this.currentPath = host.currentPath;
    this.tabs = host.tabs;
    this.activeDirTabId = host.activeDirTabId;
    this.archiveStack = host.archiveStack;
    this.restoreActiveDirTabState();
  }

  /** Best-effort: focus/move cursor to a path if it exists in the current listing. */
  focusCursorPath(path: string): void {
    if (this.activeTabKind !== 'dir') return;
    if (!path) return;

    const normalized = normalizePath(path);
    const selectable = this.dirRuntime.getSelectableEntries();
    const idx = selectable.findIndex(e => normalizePath(e.path) === normalized);
    if (idx < 0) return;

    this.selectionCursorPath = selectable[idx].path;
    this.selectionAnchorPath = this.selectionCursorPath;
    // Keep marks intact; only move cursor.
    this.fileViewContainer?.scrollToCursorPath(this.selectionCursorPath);
    this.emitPreviewTargetFromState();
  }

  /**
   * Restore a specific directory tab (by id) and place the cursor on `focusPath`.
   * Used by FileBrowser to restore selection when switching between preview tabs.
   */
  async activateDirTabAndFocusPath(tabId: string, focusPath: string): Promise<void> {
    if (!tabId) return;
    const tabs = this.workspaceService.getTabs(this.paneId) ?? [];
    const tab = tabs.find(t => t.id === tabId);
    if (!tab) return;
    if ((tab.kind ?? 'dir') !== 'dir') return;

    await this.onTabChange(tab);
    this.focusCursorPath(focusPath);
  }

  private async restoreArchiveFromPath(path: string): Promise<void> {
    await this.archiveController.restoreArchiveFromPath(this.archiveHost, path);
  }

  onTabClose(tab: TabInfo): void {
    this.dirSessions.delete(tab.id);
    this.dirTabStateController.delete(tab.id);
    if (this.activeDirTabId === tab.id) this.activeDirTabId = null;
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

  // в”Ђв”Ђ Address bar / path в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

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
      const fromPath = this.currentPath;
      try {
        const listing = await window.go.main.App.NavGoUp(this.paneId);
        await this.loadDirectory(listing.path, { focusPath: fromPath });
      } catch {
        const target = parentPath(normalizePath(fromPath));
        if (target !== fromPath) await this.loadDirectory(target, { focusPath: fromPath });
      }
    }
  }

  onRefresh(): void {
    void this.refreshCurrentPathTransparently();
    // Keep the tree in sync: reload children of the current directory node
    // without collapsing any expanded subtrees.
    this.fileViewContainer?.refreshTreeNode(this.currentPath);
  }

  private runInAngular(update: () => void): void {
    this.zone.run(() => {
      update();
      this.cdr.markForCheck();
    });
  }

  private bindFsChangeEvents(): void {
    EventsOn('fs:directoryChanged', (event: { paneId?: string; path?: string } | null) => {
      this.runInAngular(() => this.handleFsDirectoryChanged(event));
    });

    this.dirStream.bind(this.paneId, {
      onMeta: (event, isFirst) => this.runInAngular(() => this.handleDirStreamMeta(event, isFirst)),
      onBatch: (event, baseEntries) => this.runInAngular(() => this.handleDirStreamBatch(event, baseEntries)),
      onDone: (event, baseEntries) => this.runInAngular(() => this.handleDirStreamDone(event, baseEntries)),
      onError: (event) => this.runInAngular(() => this.handleDirStreamError(event)),
    });
  }

  private unbindFsChangeEvents(): void {
    if (this.fsChangeDebounceTimer) {
      clearTimeout(this.fsChangeDebounceTimer);
      this.fsChangeDebounceTimer = null;
    }
    void this.dirStream.unbind();
    EventsOff('fs:directoryChanged');
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

  private handleDirStreamMeta(event: DirStreamMetaEvent, isFirstMeta: boolean): void {
    this.currentPath = event.path;
    this.fsParentDir = event.path;
    this.dir.showHiddenLoaded = !!event.showHidden;
    this.workspaceService.updateTabPath(this.paneId, this.currentTabId, event.path);
    this.pathHistoryService.addPath(event.path, this.paneId);
    if (isFirstMeta) {
      this.pathChange.emit(event.path);
    }
    if (!this.activeDirStreamPreserveUI) {
      this.entries = this.withParentEntry([], false, event.path);
      this.applyFilter();
    }

    // best-effort pane watch sync
    void window.go?.main?.App?.SetPaneDirectory?.(this.paneId, event.path);
  }

  private handleDirStreamBatch(event: DirStreamBatchEvent, baseEntries: FileInfo[]): void {
    this.entries = this.withParentEntry(baseEntries, false, this.currentPath || event.path);
    this.applyFilter();
  }

  private handleDirStreamDone(event: DirStreamDoneEvent, baseEntries: FileInfo[]): void {
    this.entries = this.withParentEntry(baseEntries, false, event.path || this.currentPath);
    // Prune selection to keep it stable across refresh.
    if (this.selectedFiles.size > 0) {
      const present = new Set(this.entries.map(e => e.path));
      for (const p of Array.from(this.selectedFiles)) {
        if (!present.has(p)) this.selectedFiles.delete(p);
      }
      this.selectedFiles = new Set(this.selectedFiles);
    }
    this.applyFilter();

    // TC: after navigating up, select the folder we came from; otherwise place cursor on first entry.
    if (this.pendingFocusPath) {
      const focusNorm = normalizePath(this.pendingFocusPath);
      const target = this.filteredEntries.find(e => normalizePath(e.path) === focusNorm);
      if (target) {
        this.selectionCursorPath = target.path;
        this.selectionAnchorPath = target.path;
        this.selectedFiles = new Set([target.path]);
        setTimeout(() => this.fileViewContainer?.scrollToCursorPath(target.path), 0);
      }
      this.pendingFocusPath = null;
    } else if (this.dirTabStateController.load(this.activeDirTabId ?? this.currentTabId)) {
      this.restoreActiveDirTabState();
    } else if (!this.selectionCursorPath) {
      const firstReal = this.filteredEntries.find(e => e.path !== PARENT_ENTRY_PATH);
      if (firstReal) {
        this.selectionCursorPath = firstReal.path;
        this.selectionAnchorPath = firstReal.path;
      }
    }

    this.persistDirTabState();

    setTimeout(() => {
      try { this.fileViewContainer?.treeComponent?.focusNode(event.path); } catch { /* best-effort */ }
    }, 0);
  }

  private handleDirStreamError(event: DirStreamErrorEvent): void {
    this.error = event.error || 'Failed to load directory';
  }

  /**
   * Returns the effective destination directory for a copy/move operation.
   * If exactly one directory is selected in this pane, operations are directed
   * into that folder rather than the generic currentPath.
   */
  getDestinationDir(): string {
    return this.operationsController.getDestinationDir(this.operationsHost);
  }

  // в”Ђв”Ђ Archive segment click в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  async onArchiveSegmentClick(virtualPath: string): Promise<void> {
    await this.archiveController.onArchiveSegmentClick(this.archiveHost, virtualPath);
  }

  // в”Ђв”Ђ Navigation в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onNavigate(entry: FileInfo): void {
    this.openController.onNavigate(this.openHost, entry, () => this.onNavigateUp());
  }

  onOpenSelectedItem(): void {
    this.openController.onOpenSelectedItem(this.openHost, () => this.onNavigateUp());
  }

  async openQuickView(): Promise<void> {
    await this.previewController.openQuickView(this.previewHost);
  }

  /** Open a new, closable viewer tab for the given filesystem path. */
  async openFrozenPreviewTab(
    targetPath: string,
    opts?: { sourcePane: 'left' | 'right'; sourceTabId: string; sourceDir: string; autoFollow?: boolean },
  ): Promise<void> {
    await this.previewController.openFrozenPreviewTab(this.previewHost, targetPath, opts);
  }

  async onOpenInNewTab(entry: FileInfo): Promise<void> {
    await this.openController.onOpenInNewTab(this.openHost, entry);
  }

  async onRenameSelected(): Promise<void> {
    await this.operationsController.onRenameSelected(this.operationsHost);
  }

  async onRename(event: { entry: FileInfo; newName: string }): Promise<void> {
    await this.operationsController.onRename(this.operationsHost, event);
  }

  async onDragDrop(event: { event: DragEvent; source: FileInfo[]; targetPath: string }): Promise<void> {
    await this.operationsController.onDragDrop(this.operationsHost, event);
  }

  // в”Ђв”Ђ Archive navigation в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  isInArchive(): boolean { return this.archiveController.isInArchive(this.archiveHost); }

  async openRootArchive(fsPath: string): Promise<void> {
    await this.archiveController.openRootArchive(this.archiveHost, fsPath);
  }

  private async archiveNavigateInto(folderInnerPath: string): Promise<void> {
    await this.archiveController.archiveNavigateInto(this.archiveHost, folderInnerPath);
  }

  private async openNestedArchive(nestedChain: string[]): Promise<void> {
    await this.archiveController.openNestedArchive(this.archiveHost, nestedChain);
  }

  async navigateArchiveUp(): Promise<void> {
    await this.archiveController.navigateArchiveUp(this.archiveHost);
  }

  exitArchive(): void {
    this.archiveController.exitArchive(this.archiveHost);
  }

  private async refreshArchive(): Promise<void> {
    await this.archiveController.refreshArchive(this.archiveHost);
  }

  private renderArchiveFrame(frame: ArchiveFrame): void {
    this.archiveController.renderArchiveFrame(this.archiveHost, frame);
  }

  // в”Ђв”Ђ Find widget в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

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

    this.persistDirTabState();
  }

  onFindNext(): void { /* file-list scroll-to-next can be wired here */ }
  onFindPrev(): void { /* file-list scroll-to-prev can be wired here */ }

  // в”Ђв”Ђ Filtering в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  applyFilter(): void {
    this.stateController.applyFilter(this.stateHost);
  }

  private persistDirTabState(): void {
    const tabId = this.activeDirTabId ?? this.currentTabId;
    if (!tabId || this.activeTabKind !== 'dir' || this.isInArchive()) {
      return;
    }

    this.dirTabStateController.save(tabId, this.activeDirTabStateRuntime);
  }

  // в”Ђв”Ђ Selection в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onSelectionChange(entries: FileInfo[]): void {
    this.stateController.onSelectionChange(this.stateHost, entries);
  }

  /** Returns a filesystem path suitable for preview, or empty string when none. */
  getPreviewTargetPath(): string {
    return this.stateController.getPreviewTargetPath(this.stateHost);
  }

  emitPreviewTargetFromState(): void {
    this.stateController.emitPreviewTargetFromState(this.stateHost);
  }

  // State/runtime adapter methods shared by pane-level state controllers.
  getEntriesPaths(): string[] {
    return (this.entries ?? []).map(e => e.path);
  }

  getSelectableEntriesPaths(): string[] {
    return this.dirRuntime.getSelectableEntries().map(e => e.path);
  }

  scrollToCursorPath(path: string): void {
    if (!path) return;
    this.fileViewContainer?.scrollToCursorPath(path);
  }

  onSelectAll(): void {
    this.stateController.onSelectAll(this.stateHost);
  }

  onDeselectAll(): void {
    this.stateController.onDeselectAll(this.stateHost);
  }

  /**
   * TC-style effective selection: use marked files when any exist,
   * otherwise treat the cursor item as the implicit selection.
   */
  private getEffectiveSelection(): Set<string> {
    return this.stateController.getEffectiveSelection(this.stateHost);
  }

  onToggleCurrentAndMoveDown(): void {
    this.stateController.onToggleCurrentAndMoveDown(this.stateHost);
  }

  onSelectCurrentAndMoveDown(): void {
    this.stateController.onSelectCurrentAndMoveDown(this.stateHost);
  }

  /** TC Space: mark/unmark item under cursor and advance cursor down. */
  onMarkCurrentItem(): void {
    this.stateController.onMarkCurrentItem(this.stateHost);
  }

  onExtendSelection(delta: -1 | 1): void {
    this.stateController.onExtendSelection(this.stateHost, delta);
  }

  // в”Ђв”Ђ Cursor movement в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onMoveCursorUp(): void {
    this.stateController.onMoveCursorUp(this.stateHost);
  }

  onMoveCursorDown(): void {
    this.stateController.onMoveCursorDown(this.stateHost);
  }

  onMoveCursorLeft(): void {
    this.stateController.onMoveCursorLeft(this.stateHost);
  }

  onMoveCursorRight(): void {
    this.stateController.onMoveCursorRight(this.stateHost);
  }

  onMoveCursorHome(): void {
    this.stateController.onMoveCursorHome(this.stateHost);
  }

  onMoveCursorEnd(): void {
    this.stateController.onMoveCursorEnd(this.stateHost);
  }

  onMoveCursorPageUp(): void {
    this.stateController.onMoveCursorPageUp(this.stateHost);
  }

  onMoveCursorPageDown(): void {
    this.stateController.onMoveCursorPageDown(this.stateHost);
  }

  onSelectByMask(): void {
    const mask = prompt('Select by mask (example: *.txt):', '*.*')?.trim();
    if (!mask) return;
    this.stateController.onSelectByMask(this.stateHost, mask);
  }

  onUnselectByMask(): void {
    const mask = prompt('Unselect by mask (example: *.txt):', '*.*')?.trim();
    if (!mask) return;
    this.stateController.onUnselectByMask(this.stateHost, mask);
  }

  onInvertSelection(): void {
    this.stateController.onInvertSelection(this.stateHost);
  }

  onSelectSameExtension(): void {
    this.stateController.onSelectSameExtension(this.stateHost);
  }

  // в”Ђв”Ђ File operations в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  async onCreateFolder(): Promise<void> {
    await this.operationsController.onCreateFolder(this.operationsHost);
  }

  async onDeleteSelected(): Promise<void> {
    await this.operationsController.onDeleteSelected(this.operationsHost);
  }

  /**
   * Returns archive metadata for the current tree-view selection when the
   * selected nodes are inside an expanded archive node (not in archive-pane
   * mode). All selected nodes must share the same archive FS path and nested
   * chain; otherwise returns null.
   */
  getTreeArchiveSelectionContext(): { archivePath: string; innerPaths: string[] } | null {
    return this.operationsController.getTreeArchiveSelectionContext(this.operationsHost);
  }

  onCopyToOther(): void { 
    this.operationsController.onCopyToOther(this.operationsHost);
  }
  
  onMoveToOther(): void { 
    this.operationsController.onMoveToOther(this.operationsHost);
  }

  // в”Ђв”Ђ Archive move operations в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  /**
   * Called by FileBrowserComponent after it has resolved destDir.
   * Shows the confirmation dialog; executePendingMove runs when user confirms.
   */
  initiatePendingMove(entries: string[], destDir: string): void {
    this.archiveController.initiatePendingMove(this.archiveHost, entries, destDir);
  }

  /**
   * Bound to (confirmed) on the ArchiveMoveWarningDialogComponent.
   */
  async executePendingMove(): Promise<void> {
    await this.archiveController.executePendingMove(this.archiveHost);
  }

  // в”Ђв”Ђ Tab shortcuts в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

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
