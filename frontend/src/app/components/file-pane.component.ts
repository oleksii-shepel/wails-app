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
  ArchiveService, ArchiveFrame
} from '../services/archive.service';
import { normalizePath } from './address-bar.utils';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { FileViewContainerComponent, ViewLayout } from './file-view-container.component';
import { ViewerArchiveContext } from './viewer-surface.component';
import { DirSurfaceVm } from '../surfaces/dir/dir-surface.vm';
import { DirSurfaceSession } from '../surfaces/dir/dir-surface-session';
import { DirSurfaceRuntime } from '../surfaces/dir/dir-surface-runtime';
import { FilePaneDirSurfaceRuntime } from '../surfaces/dir/file-pane-dir-surface-runtime';
import { DirTabStateController } from '../surfaces/dir/dir-tab-state-controller';
import { SurfaceContext } from '../surfaces/surface-context';
import { SurfaceOutletComponent } from '../surfaces/surface-outlet.component';
import { FilePaneArchiveController, type FilePaneArchiveHost } from './file-pane-archive.controller';
import { FilePaneNavigationController, type FilePaneNavigationHost } from './file-pane-navigation.controller';
import { FilePaneOpenController, type FilePaneOpenHost } from './file-pane-open.controller';
import { FilePaneOperationsController, type FilePaneOperationsHost } from './file-pane-operations.controller';
import { FilePanePreviewController, type FilePanePreviewHost } from './file-pane-preview.controller';
import { FilePaneDirectoryController, type FilePaneDirectoryHost } from './file-pane-directory.controller';
import { FilePaneDirSessionController, type FilePaneDirSessionHost } from './file-pane-dir-session.controller';
import { FilePaneFocusController, type FilePaneFocusHost } from './file-pane-focus.controller';
import { FilePanePathController, type FilePanePathHost } from './file-pane-path.controller';
import { FilePaneUiController, type FilePaneUiHost } from './file-pane-ui.controller';
import { FilePaneTabsController, type FilePaneTabsHost } from './file-pane-tabs.controller';
import { FilePaneStateController, type FilePaneStateHost } from './file-pane-state.controller';
import { DirListStreamController } from '../surfaces/dir/dir-list-stream.controller';
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

  archiveStack: ArchiveFrame[] = [];

  // Archive move state
  archiveMoveWarning = false;
  archivePendingMoveEntries: string[] = [];
  archivePendingMoveDestDir = '';

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
  private readonly directoryController = new FilePaneDirectoryController();
  private readonly dirSessionController = new FilePaneDirSessionController();
  private readonly focusController = new FilePaneFocusController();
  private readonly pathController = new FilePanePathController();
  private readonly uiController = new FilePaneUiController();
  private readonly openController = new FilePaneOpenController();
  private readonly operationsController = new FilePaneOperationsController();
  private readonly previewController = new FilePanePreviewController();
  private readonly stateController = new FilePaneStateController(
    PARENT_ENTRY_PATH,
    FileType.FILE,
    FileType.SYMLINK,
  );
  private readonly tabsController = new FilePaneTabsController();

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
      exitArchive: () => this.archiveController.exitArchive(this.archiveHost),
      clearArchiveState: () => this.clearArchiveState(),
      pathIsInsideArchive: (path: string) => this.pathIsInsideArchive(path),
      loadPathTransparently: (path: string) => this.loadPathTransparently(path),
      loadDirectory: (path: string, opts?: { preserveUI?: boolean; focusPath?: string }) => this.loadDirectory(path, opts),
      navigateArchiveUp: () => this.archiveController.navigateArchiveUp(this.archiveHost),
      refreshCurrentPathTransparently: () => this.refreshCurrentPathTransparently(),
      refreshTreeNode: (path: string) => this.fileViewContainer?.refreshTreeNode(path),
      renderArchiveFrame: (frame: ArchiveFrame) => this.archiveController.renderArchiveFrame(this.archiveHost, frame),
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
      parseFilterText: (text: string) => thisRef.filterService.parseFilterText(text),
      filterEntries: (entries: FileInfo[], query: FilterQuery) => thisRef.filterService.filterEntries(entries, query),
      findEntry: (path: string) => thisRef.entries.find(entry => entry.path === path),
      emitPreviewTarget: (path: string) => thisRef.previewTargetChange.emit(path),
      setFindMatchCount: (count: number) => thisRef.findWidget?.setMatchCount(count),
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

  private get directoryHost(): FilePaneDirectoryHost {
    const thisRef = this;
    return {
      get paneId() { return thisRef.paneId; },
      get currentPath() { return thisRef.currentPath; },
      set currentPath(value: string) { thisRef.currentPath = value; },
      get fsParentDir() { return thisRef.fsParentDir; },
      set fsParentDir(value: string) { thisRef.fsParentDir = value; },
      get currentTabId() { return thisRef.currentTabId; },
      get showHidden() { return thisRef.showHidden; },
      get entries() { return thisRef.entries; },
      set entries(value: FileInfo[]) { thisRef.entries = value; },
      get filteredEntries() { return thisRef.filteredEntries; },
      get selectedFiles() { return thisRef.selectedFiles; },
      set selectedFiles(value: Set<string>) { thisRef.selectedFiles = value; },
      get selectionCursorPath() { return thisRef.selectionCursorPath; },
      set selectionCursorPath(value: string | null) { thisRef.selectionCursorPath = value; },
      get selectionAnchorPath() { return thisRef.selectionAnchorPath; },
      set selectionAnchorPath(value: string | null) { thisRef.selectionAnchorPath = value; },
      get loading() { return thisRef.loading; },
      set loading(value: boolean) { thisRef.loading = value; },
      get error() { return thisRef.error; },
      set error(value: string | null) { thisRef.error = value; },
      workspaceService: this.workspaceService,
      pathHistoryService: this.pathHistoryService,
      dirStream: this.dirStream,
      isInArchive: () => thisRef.isInArchive(),
      exitArchive: () => thisRef.archiveController.exitArchive(thisRef.archiveHost),
      clearArchiveState: () => thisRef.clearArchiveState(),
      withParentEntry: (raw: FileInfo[], inArchive: boolean, fsPath: string) => thisRef.withParentEntry(raw, inArchive, fsPath),
      applyFilter: () => thisRef.applyFilter(),
      setShowHiddenLoaded: (showHidden: boolean) => {
        thisRef.dir.showHiddenLoaded = showHidden;
      },
      emitPathChange: (path: string) => thisRef.pathChange.emit(path),
      runInAngular: (update: () => void) => thisRef.runInAngular(update),
      onRefresh: () => thisRef.onRefresh(),
      hasSavedDirTabState: () => !!thisRef.dirTabStateController.load(thisRef.activeDirTabId ?? thisRef.currentTabId),
      restoreActiveDirTabState: () => thisRef.restoreActiveDirTabState(),
      persistDirTabState: () => thisRef.persistDirTabState(),
      scrollToCursorPath: (path: string) => thisRef.scrollToCursorPath(path),
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
      exitArchive: () => thisRef.archiveController.exitArchive(thisRef.archiveHost),
      clearArchiveState: () => thisRef.clearArchiveState(),
      archiveNavigateInto: (path: string) => thisRef.archiveController.archiveNavigateInto(thisRef.archiveHost, path),
      openNestedArchive: (chain: string[]) => thisRef.archiveController.openNestedArchive(thisRef.archiveHost, chain),
      loadDirectory: (path: string) => thisRef.loadDirectory(path),
      openRootArchive: (path: string) => thisRef.archiveController.openRootArchive(thisRef.archiveHost, path),
      emitPathChange: (path: string) => thisRef.pathChange.emit(path),
      getEffectiveSelection: () => thisRef.getEffectiveSelection(),
      openPathInNewTab: (path: string) => thisRef.openPathInNewTab(path),
    };
  }

  private get tabsHost(): FilePaneTabsHost {
    const thisRef = this;
    return {
      get paneId() { return thisRef.paneId; },
      get currentTabId() { return thisRef.currentTabId; },
      set currentTabId(value: string) { thisRef.currentTabId = value; },
      get currentPath() { return thisRef.currentPath; },
      get tabs() { return thisRef.tabs; },
      set tabs(value: TabInfo[]) { thisRef.tabs = value; },
      get activeDirTabId() { return thisRef.activeDirTabId; },
      set activeDirTabId(value: string | null) { thisRef.activeDirTabId = value; },
      get archiveStack() { return thisRef.archiveStack; },
      get fsParentDir() { return thisRef.fsParentDir; },
      get lastSeenActiveTabId() { return thisRef.lastSeenActiveTabId; },
      set lastSeenActiveTabId(value: string | null) { thisRef.lastSeenActiveTabId = value; },
      get lastSeenActiveTabPath() { return thisRef.lastSeenActiveTabPath; },
      set lastSeenActiveTabPath(value: string | null) { thisRef.lastSeenActiveTabPath = value; },
      workspaceService: this.workspaceService,
      isInArchive: () => thisRef.isInArchive(),
      deleteDirSession: (tabId: string) => {
        thisRef.dirSessions.delete(tabId);
      },
      deleteDirTabState: (tabId: string) => {
        thisRef.dirTabStateController.delete(tabId);
      },
      loadPaneState: () => thisRef.loadPaneState(),
      onTabChange: (tab: TabInfo) => thisRef.onTabChange(tab),
      onTabClose: (tab: TabInfo) => thisRef.onTabClose(tab),
      onTabPin: (tab: TabInfo) => thisRef.onTabPin(tab),
    };
  }

  private get dirSessionHost(): FilePaneDirSessionHost {
    const thisRef = this;
    return {
      get currentTabId() { return thisRef.currentTabId; },
      get activeDirTabId() { return thisRef.activeDirTabId; },
      set activeDirTabId(value: string | null) { thisRef.activeDirTabId = value; },
      get tabs() { return thisRef.tabs; },
      dirTabStateController: this.dirTabStateController,
      getDirSession: (tabId: string) => thisRef.dirSessions.get(tabId),
      setDirSession: (session: DirSurfaceSession) => {
        thisRef.dirSessions.set(session.tabId, session);
      },
      isInArchive: () => thisRef.isInArchive(),
      getSelectableEntriesPaths: () => thisRef.dirRuntime.getSelectableEntries().map(entry => entry.path),
      applyFilter: () => thisRef.applyFilter(),
      scrollToCursorPath: (path: string) => thisRef.scrollToCursorPath(path),
      emitPreviewTargetFromState: () => thisRef.emitPreviewTargetFromState(),
    };
  }

  private get uiHost(): FilePaneUiHost {
    const thisRef = this;
    return {
      get paneId() { return thisRef.paneId; },
      get currentView() { return thisRef.currentView; },
      set currentView(value: ViewLayout) { thisRef.currentView = value; },
      get sortField() { return thisRef.sortField; },
      set sortField(value: string) { thisRef.sortField = value; },
      get sortDirection() { return thisRef.sortDirection; },
      set sortDirection(value: 'asc' | 'desc') { thisRef.sortDirection = value; },
      get directoriesFirst() { return thisRef.directoriesFirst; },
      set directoriesFirst(value: boolean) { thisRef.directoriesFirst = value; },
      get selectedFiles() { return thisRef.selectedFiles; },
      set selectedFiles(value: Set<string>) { thisRef.selectedFiles = value; },
      getOptimisticHiddenPaths: () => thisRef.getOptimisticHiddenPaths(),
      setOptimisticHiddenPaths: (paths: string[]) => thisRef.setOptimisticHiddenPaths(paths),
      applyFilter: () => thisRef.applyFilter(),
      persistDirTabState: () => thisRef.persistDirTabState(),
      refreshCurrentPathTransparently: () => thisRef.refreshCurrentPathTransparently(),
    };
  }

  private get pathHost(): FilePanePathHost {
    const thisRef = this;
    return {
      get currentPath() { return thisRef.currentPath; },
      get archiveStack() { return thisRef.archiveStack; },
      isInArchive: () => thisRef.isInArchive(),
      loadDirectory: (path: string, opts?: { preserveUI?: boolean; focusPath?: string }) => thisRef.loadDirectory(path, opts),
      restoreArchiveFromPath: (path: string) => thisRef.archiveController.restoreArchiveFromPath(thisRef.archiveHost, path),
      renderArchiveFrame: (frame: ArchiveFrame) => thisRef.archiveController.renderArchiveFrame(thisRef.archiveHost, frame),
    };
  }

  private get focusHost(): FilePaneFocusHost {
    const thisRef = this;
    return {
      get currentTabId() { return thisRef.currentTabId; },
      get paneId() { return thisRef.paneId; },
      get selectionCursorPath() { return thisRef.selectionCursorPath; },
      set selectionCursorPath(value: string | null) { thisRef.selectionCursorPath = value; },
      get selectionAnchorPath() { return thisRef.selectionAnchorPath; },
      set selectionAnchorPath(value: string | null) { thisRef.selectionAnchorPath = value; },
      get selectedFiles() { return thisRef.selectedFiles; },
      getSelectableEntries: () => thisRef.dirRuntime.getSelectableEntries(),
      scrollToCursorPath: (path: string) => thisRef.scrollToCursorPath(path),
      emitPreviewTargetFromState: () => thisRef.emitPreviewTargetFromState(),
      getWorkspaceTabs: () => thisRef.workspaceService.getTabs(thisRef.paneId) ?? [],
      onTabChange: (tab: TabInfo) => thisRef.onTabChange(tab),
      focusAddressBar: () => thisRef.addressBar?.enterEditMode(),
      openFindWidget: () => thisRef.findWidget?.open(),
      toggleFindWidget: () => thisRef.findWidget?.toggle(),
      closeFindWidget: () => thisRef.findWidget?.close(),
    };
  }

  private get dir(): DirSurfaceSession {
    return this.dirSessionController.getActiveSession(this.dirSessionHost);
  }

  private restoreActiveDirTabState(): void {
    this.dirSessionController.restoreActiveState(this.dirSessionHost);
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
    this.uiController.loadViewPreference(this.uiHost);
    this.loadPaneState();
    this.directoryController.bind(this.directoryHost);
    this.uiController.bindOptimisticDnD(this.uiHost);

    // Keep panes in sync when tab state changes elsewhere (e.g. cross-pane tab drag/drop).
    this.workspaceService.activeWorkspace$
      .pipe(takeUntil(this.destroy$))
      .subscribe(ws => {
        if (!ws || !this.workspace) return;
        if (ws.id !== this.workspace.id) return;
        this.tabsController.onWorkspaceUpdate(this.tabsHost, ws);
      });
  }
  
  ngOnChanges(changes: SimpleChanges): void {
    if (changes['workspace']) {
      // The input is bound to FileBrowserComponent.activeWorkspace which starts as null.
      // That means the *first* change Angular sees is often null, and the transition
      // to a real workspace would be a later change. Always load once we have a
      // non-null workspace so tabs/pane render without requiring a user click.
      if (this.workspace) {
        this.uiController.loadViewPreference(this.uiHost);
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
      this.focusController.ensureCursorVisibleOnActivation(this.focusHost);
    }
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();

    void this.directoryController.unbind(this.directoryHost);
    this.uiController.unbindOptimisticDnD();
  }

  onTabDrop(event: TabDropEvent): void {
    this.tabsController.onTabDrop(this.tabsHost, event);
  }

  onTabDragStart(tab: TabInfo): void {
    this.tabsController.onTabDragStart(this.tabsHost, tab);
  }

  get entriesForAnalysis(): FileInfo[] {
    return this.entries.filter(e => e.path !== PARENT_ENTRY_PATH);
  }

  // в”Ђв”Ђ View System Methods в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  async onViewChange(view: ViewLayout): Promise<void> {
    await this.uiController.onViewChange(this.uiHost, view);
  }

  onSortChange(sort: { field: string; direction: 'asc' | 'desc' }): void {
    this.uiController.onSortChange(this.uiHost, sort);
  }

  onDirectoriesFirstChange(value: boolean): void {
    this.uiController.onDirectoriesFirstChange(this.uiHost, value);
  }

  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void {
    void event;
  }

  onNodeDoubleClick(entry: FileInfo): void {
    this.openController.onNodeDoubleClick(this.openHost, entry, () => this.onNavigateUp());
  }

  private withParentEntry(raw: FileInfo[], inArchive: boolean, fsPath: string): FileInfo[] {
    return this.pathController.withParentEntry(raw, inArchive, fsPath);
  }

  private pathIsInsideArchive(path: string): boolean {
    return this.pathController.pathIsInsideArchive(path);
  }

  private async loadPathTransparently(path: string): Promise<void> {
    await this.pathController.loadPathTransparently(this.pathHost, path);
  }

  private async refreshCurrentPathTransparently(): Promise<void> {
    await this.pathController.refreshCurrentPathTransparently(this.pathHost);
  }

  // в”Ђв”Ђ Pane state в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  private async loadPaneState(): Promise<void> {
    const host = this.navigationHost;
    await this.navigationController.loadPaneState(host);
    this.applyNavigationHostState(host);
  }

  private clearArchiveState(): void {
    this.archiveStack = [];
    this.fsParentDir  = '';
  }

  private async loadDirectory(path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void> {
    await this.directoryController.loadDirectory(this.directoryHost, path, opts);
  }

  // в”Ђв”Ђ Tab management в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  async onTabChange(tab: TabInfo): Promise<void> {
    const host = this.navigationHost;
    await this.navigationController.onTabChange(host, tab);
    this.applyNavigationHostState(host);
  }

  private applyNavigationHostState(host: FilePaneNavigationHost): void {
    this.currentTabId = host.currentTabId;
    this.currentPath = host.currentPath;
    this.pathReady = host.pathReady;
    this.tabs = host.tabs;
    this.activeDirTabId = host.activeDirTabId;
    this.archiveStack = host.archiveStack;
    this.restoreActiveDirTabState();
  }

  /** Best-effort: focus/move cursor to a path if it exists in the current listing. */
  focusCursorPath(path: string): void {
    if (this.activeTabKind !== 'dir') return;
    this.focusController.focusCursorPath(this.focusHost, path);
  }

  /**
   * Restore a specific directory tab (by id) and place the cursor on `focusPath`.
   * Used by FileBrowser to restore selection when switching between preview tabs.
   */
  async activateDirTabAndFocusPath(tabId: string, focusPath: string): Promise<void> {
    await this.focusController.activateDirTabAndFocusPath(this.focusHost, tabId, focusPath);
  }

  onTabClose(tab: TabInfo): void {
    this.tabsController.onTabClose(this.tabsHost, tab);
  }

  onTabPin(tab: TabInfo): void {
    this.tabsController.onTabPin(this.tabsHost, tab);
  }

  onNewTab(): void {
    this.tabsController.onNewTab(this.tabsHost);
  }

  private async openPathInNewTab(path: string): Promise<void> {
    await this.tabsController.openPathInNewTab(this.tabsHost, path);
  }

  // в”Ђв”Ђ Address bar / path в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onPathChange(path: string): void {
    this.navigationController.onPathChange(this.navigationHost, path);
  }

  async onNavigateUp(): Promise<void> {
    await this.navigationController.onNavigateUp(this.navigationHost);
  }

  onRefresh(): void {
    this.navigationController.onRefresh(this.navigationHost);
  }

  private runInAngular(update: () => void): void {
    this.zone.run(() => {
      update();
      this.cdr.markForCheck();
    });
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

  // в”Ђв”Ђ Find widget в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onFindStateChange(state: FindWidgetState | null): void {
    this.stateController.onFindStateChange(this.stateHost, state);
  }

  onFindNext(): void {
    this.stateController.onFindNext(this.stateHost);
  }

  onFindPrev(): void {
    this.stateController.onFindPrev(this.stateHost);
  }

  // в”Ђв”Ђ Filtering в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  applyFilter(): void {
    this.stateController.applyFilter(this.stateHost);
  }

  private persistDirTabState(): void {
    const tabId = this.activeDirTabId ?? this.currentTabId;
    if (!tabId || this.activeTabKind !== 'dir' || this.isInArchive()) {
      return;
    }

    this.dirTabStateController.save(tabId, this.dirSessionController.getActiveRuntime(this.dirSessionHost));
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
    this.tabsController.closeCurrentTab(this.tabsHost); 
  }
  
  onNextTab():           void { 
    this.tabsController.nextTab(this.tabsHost); 
  }
  
  onPreviousTab():       void { 
    this.tabsController.previousTab(this.tabsHost); 
  }
  
  onJumpToTab(n: number): void { 
    this.tabsController.jumpToTab(this.tabsHost, n); 
  }
  
  onTogglePinCurrentTab(): void { 
    this.tabsController.togglePinCurrentTab(this.tabsHost); 
  }

  focusAddressBar(): void { 
    this.focusController.focusAddressBar(this.focusHost); 
  }
  
  focusFind():       void { 
    this.focusController.focusFind(this.focusHost); 
  }

  focusFilter(): void { 
    this.focusController.focusFilter(this.focusHost); 
  }
  
  onClearFilter(): void { 
    this.focusController.clearFilter(this.focusHost); 
  }
}
