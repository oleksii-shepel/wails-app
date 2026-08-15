// file-pane.component.ts
import { Component, Input, Output, EventEmitter, OnInit, OnChanges, OnDestroy, ViewChild, ViewChildren, QueryList, SimpleChanges, NgZone, ChangeDetectorRef } from '@angular/core';
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
import { ViewerArchiveContext } from '../viewers/viewer-archive-context';
import { DirSurfaceVm } from '../surfaces/dir/dir-surface.vm';
import { DirSurfaceSession } from '../surfaces/dir/dir-surface-session';
import { DirSurfaceRuntime } from '../surfaces/dir/dir-surface-runtime';
import { FilePaneDirSurfaceRuntime } from '../surfaces/dir/file-pane-dir-surface-runtime';
import { DirTabStateController } from '../surfaces/dir/dir-tab-state-controller';
import { SurfaceContext } from '../surfaces/surface-context';
import { SurfaceRegistryService } from '../surfaces/surface-registry.service';
import type { SurfaceCapability } from '../surfaces/surface-contribution';
import type { SizeManagerSurfaceVm } from '../surfaces/size-manager/size-manager-surface.vm';
import { SurfaceOutletComponent } from '../surfaces/surface-outlet.component';
import { FilePaneArchiveController, type FilePaneArchiveHost } from './file-pane-archive.controller';
import { FilePaneActionHostsFactory, type FilePaneActionHostsSource } from './file-pane-action-hosts.factory';
import { FilePaneDirVmFactory, type FilePaneDirVmSource } from './file-pane-dir-vm.factory';
import { FilePaneNavigationController, type FilePaneNavigationHost } from './file-pane-navigation.controller';
import { FilePaneSurfaceContextFactory, type FilePaneSurfaceContextSource } from './file-pane-surface-context.factory';
import { FilePaneRuntimeStateController, type FilePaneRuntimeStateHost } from './file-pane-runtime-state.controller';
import { FilePaneBrowserCommandsFactory, type FilePaneBrowserCommands, type FilePaneBrowserCommandsSource } from './file-pane-browser-commands.factory';
import { FilePaneShortcutCommandsFactory, type FilePaneShortcutCommands, type FilePaneShortcutCommandsSource } from './file-pane-shortcut-commands.factory';
import { FilePaneStateRuntimeFactory, type FilePaneStateRuntimeAdapter, type FilePaneStateRuntimeSource } from './file-pane-state-runtime.factory';
import { FilePaneOpenController, type FilePaneOpenHost } from './file-pane-open.controller';
import { FilePaneOperationsController, type FilePaneOperationsHost } from './file-pane-operations.controller';
import { FilePanePreviewController, type FilePanePreviewHost } from './file-pane-preview.controller';
import { FilePaneDirectoryController, type FilePaneDirectoryHost } from './file-pane-directory.controller';
import { FilePaneDirSessionController, type FilePaneDirSessionHost } from './file-pane-dir-session.controller';
import { FilePaneFocusController, type FilePaneFocusHost } from './file-pane-focus.controller';
import { FilePanePathController, type FilePanePathHost } from './file-pane-path.controller';
import { FilePaneRuntimeHostsFactory, type FilePaneRuntimeHostsSource } from './file-pane-runtime-hosts.factory';
import { FilePaneSurfaceHostsFactory, type FilePaneSurfaceHostsSource } from './file-pane-surface-hosts.factory';
import { FilePaneUiController, type FilePaneUiHost } from './file-pane-ui.controller';
import { FilePaneTabsController, type FilePaneTabsHost } from './file-pane-tabs.controller';
import { FilePaneStateController, type FilePaneStateHost } from './file-pane-state.controller';
import { DirListStreamController } from '../surfaces/dir/dir-list-stream.controller';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { DirectorySourceService } from '../services/directory-source.service';
import { OperationService } from '../services/operation.service';
import { BatchRenameDialogService } from '../services/batch-rename-dialog.service';
import { RemoteConnectDialogService } from '../services/remote-connect-dialog.service';
import { SettingsService } from '../services/settings.service';
import { getSourceCapabilities } from '../services/source-capabilities';
import { NO_ACTIONS, type ActionAvailability } from '../services/action-availability';
import { PluginManagerService } from '../services/plugin-manager.service';
import { adaptPluginToolbarActions, buildBuiltinPaneToolbarActions } from './pane-toolbar-action';
import { bindCommandPresentation, type PresentedCommand } from './presented-command';
import { presentPluginCommand } from './plugin-command-presentation';
import { InputEngineService } from '../services/input-engine.service';
import type { MenuContext, ToolbarContext } from '../plugin/plugin-registry-api';
import type { PinnedLocation } from './address-bar.types';
import { BC } from './file-browser-command-registry';
import { buildBackgroundFileContextMenu, buildBuiltinFileContextMenu, type BackgroundFileContextAction, type BuiltinFileContextAction } from './file-context-menu.model';
import { FileContextMenuComponent, type FileContextMenuState } from './file-context-menu.component';
import type { SearchResultItem } from '../services/global-search.service';
import { tabSurfaceId } from '../surfaces/tab-surface';

declare const window: any;

@Component({
  selector: 'app-file-pane',
  standalone: true,
  imports: [
    CommonModule,
    TabBarComponent,
    SurfaceOutletComponent,
    FileContextMenuComponent,
  ],
  template: `
    <div class="file-pane" [class.file-pane--archive]="isInArchive()">
      <app-tab-bar
        *ngIf="showTabBar"
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

      <app-surface-outlet
        *ngFor="let item of allTabSurfaceCtxs; trackBy: trackTabSurface"
        #surfaceOutlet
        [ctx]="item.ctx"
        [active]="item.active"
        [class.tab-surface--inactive]="!item.active"
        [attr.aria-hidden]="!item.active ? true : null"
        (firstEntryArrived)="item.active && onSurfaceFirstEntry()">
      </app-surface-outlet>

      <app-file-context-menu
        [state]="fileContextMenu"
        (closed)="closeFileContextMenu()">
      </app-file-context-menu>
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

    .tab-surface--inactive {
      position: absolute;
      width: 0;
      height: 0;
      overflow: hidden;
      pointer-events: none;
    }
  `]
})
export class FilePaneComponent implements OnInit, OnChanges, OnDestroy {
  private readonly dirStream: DirListStreamController;
  private readonly dirSessions = new Map<string, DirSurfaceSession>();
  private readonly dirTabStateController = new DirTabStateController();
  private readonly actionHostsFactory = new FilePaneActionHostsFactory();
  private readonly dirVmFactory = new FilePaneDirVmFactory();
  private readonly runtimeHostsFactory = new FilePaneRuntimeHostsFactory();
  private readonly runtimeStateController = new FilePaneRuntimeStateController();
  private readonly browserCommandsFactory = new FilePaneBrowserCommandsFactory();
  private readonly shortcutCommandsFactory = new FilePaneShortcutCommandsFactory();
  private readonly stateRuntimeFactory = new FilePaneStateRuntimeFactory();
  private readonly surfaceContextFactory = new FilePaneSurfaceContextFactory();
  private readonly surfaceHostsFactory = new FilePaneSurfaceHostsFactory();
  private activeDirTabId: string | null = null;

  get shortcuts(): FilePaneShortcutCommands {
    return this.shortcutCommandsFactory.create(this.shortcutCommandsSource);
  }

  get browserCommands(): FilePaneBrowserCommands {
    return this.browserCommandsFactory.create(this.browserCommandsSource);
  }

  get sourceCapabilities() {
    return getSourceCapabilities(this.currentPath, { inArchive: this.isInArchive() });
  }

  get toolbarActions(): PresentedCommand[] {
    const context: ToolbarContext = {
      selectedPaths: Array.from(this.selectedFiles).filter(path => path && path !== PARENT_ENTRY_PATH),
      currentDirectory: this.currentPath,
    };

    const builtins = buildBuiltinPaneToolbarActions(this.actionAvailability, {
      createFolder: () => this.browserCommands.selection.createFolder(),
      deleteSelected: () => this.browserCommands.selection.deleteSelected(),
      copyToOther: () => this.browserCommands.selection.copyToOther(),
      moveToOther: () => this.browserCommands.selection.moveToOther(),
      refresh: () => this.browserCommands.navigation.refresh(),
    });
    const plugins = adaptPluginToolbarActions(
      this.pluginManager.resolveToolbarActions(context),
      context,
      this.inputEngine,
    );
    return [...builtins, ...plugins];
  }

  private get stateRuntime(): FilePaneStateRuntimeAdapter {
    return this.stateRuntimeFactory.create(this.stateRuntimeSource);
  }

  private get browserCommandsSource(): FilePaneBrowserCommandsSource {
    return {
      surfaceCapabilities: this.activeSurfaceCapabilities,
      sourceCapabilities: getSourceCapabilities(this.currentPath, { inArchive: this.isInArchive() }),
      navigationController: this.navigationController,
      navigationHost: this.navigationHost,
      openController: this.openController,
      openHost: this.openHost,
      operationsController: this.operationsController,
      operationsHost: this.operationsHost,
      previewController: this.previewController,
      previewHost: this.previewHost,
      focusController: this.focusController,
      focusHost: this.focusHost,
      stateRuntime: this.stateRuntime,
      zoomInActiveViewer: () => this.zoomActiveViewerIn(),
      zoomOutActiveViewer: () => this.zoomActiveViewerOut(),
      resetActiveViewerZoom: () => this.resetActiveViewerZoom(),
    };
  }

  private get dirVmSource(): FilePaneDirVmSource {
    const thisRef = this;
    return {
      get paneId() { return thisRef.paneId; },
      get pathReady() { return thisRef.pathReady; },
      get currentPath() { return thisRef.currentPath; },
      get loading() { return thisRef.loading; },
      get showPathBar() { return thisRef.showPathBar; },
      get showBreadcrumbs() { return thisRef.showBreadcrumbs; },
      get singleClickToOpen() { return thisRef.settingsService.getSettings().files.singleClickToOpen; },
      get showHidden() { return thisRef.showHidden; },
      get paneActive() { return thisRef.paneActive; },
      get actionAvailability() { return thisRef.actionAvailability; },
      get toolbarActions() { return thisRef.toolbarActions; },
      get filteredEntries() { return thisRef.filteredEntries; },
      get selectedFiles() { return thisRef.selectedFiles; },
      get selectionCursorPath() { return thisRef.selectionCursorPath; },
      get error() { return thisRef.error; },
      get currentView() { return thisRef.currentView; },
      get sortField() { return thisRef.sortField; },
      get sortDirection() { return thisRef.sortDirection; },
      get directoriesFirst() { return thisRef.directoriesFirst; },
      get colorizeByMime() { return thisRef.colorizeByMime; },
      get browserPaneSplit() { return thisRef.getPersistedBrowserPaneSplit(); },
      get archiveMoveWarning() { return thisRef.archiveMoveWarning; },
      set archiveMoveWarning(value: boolean) { thisRef.archiveMoveWarning = value; },
      get archivePendingMoveEntries() { return thisRef.archivePendingMoveEntries; },
      onPathChange: (path: string) => thisRef.onPathChange(path),
      onPinnedLocationOpen: (pin: PinnedLocation) => void thisRef.openPinnedLocation(pin),
      onArchiveSegmentClick: (path: string) => thisRef.handleArchiveSegmentClick(path),
      onNavigateUp: () => thisRef.browserCommands.navigation.navigateUp(),
      onRefresh: () => thisRef.browserCommands.navigation.refresh(),
      onViewChange: (view: ViewLayout) => thisRef.onViewChange(view),
      onSortChange: (sort: { field: string; direction: 'asc' | 'desc' }) => thisRef.onSortChange(sort),
      onDirectoriesFirstChange: (value: boolean) => thisRef.onDirectoriesFirstChange(value),
      onColorizeByMimeChange: (value: boolean) => thisRef.onColorizeByMimeChange(value),
      onBrowserPaneSplitChange: (value: number) => thisRef.setPersistedBrowserPaneSplit(value),
      onCreateFolder: () => thisRef.browserCommands.selection.createFolder(),
      onDeleteSelected: () => thisRef.browserCommands.selection.deleteSelected(),
      onCopyToOther: () => thisRef.browserCommands.selection.copyToOther(),
      onMoveToOther: () => thisRef.browserCommands.selection.moveToOther(),
      onFindStateChange: (state: FindWidgetState | null) => thisRef.onFindStateChange(state),
      onFindNext: () => thisRef.onFindNext(),
      onFindPrev: () => thisRef.onFindPrev(),
      onSelectionChange: (entries: FileInfo[]) => thisRef.onSelectionChange(entries),
      onNavigate: (entry: FileInfo) => thisRef.onNavigate(entry),
      onOpenInNewTab: (entry: FileInfo) => thisRef.openEntryInNewTab(entry),
      onContextMenu: (event: { event: MouseEvent; entry: FileInfo }) => thisRef.onContextMenu(event),
      onBackgroundContextMenu: (event: MouseEvent) => thisRef.onBackgroundContextMenu(event),
      onNodeDoubleClick: (entry: FileInfo) => thisRef.onNodeDoubleClick(entry),
      onDragDrop: (event: any) => thisRef.handleDragDrop(event),
      onRename: (event: { entry: FileInfo; newName: string }) => thisRef.renameEntry(event),
      executePendingMove: () => thisRef.executePendingMove(),
    };
  }

  async onExtractSelected(destDir: string, flattenPaths = false): Promise<void> {
    await this.archiveController.extractSelected(this.archiveHost, flattenPaths, destDir);
  }

  @ViewChildren('surfaceOutlet') private surfaceOutletList?: QueryList<SurfaceOutletComponent>;
  private get surfaceOutlet(): SurfaceOutletComponent | undefined {
    return this.surfaceOutletList?.find((o) => o.ctx?.tab?.id === this.currentTabId);
  }

  private get addressBar() { return this.surfaceOutlet?.addressBar; }
  private get fileViewContainer() { return this.surfaceOutlet?.fileViewContainer; }
  private get findWidget() { return this.surfaceOutlet?.findWidget; }
  private get viewer() { return this.surfaceOutlet?.viewer; }

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() workspace: WorkspaceConfig | null = null;
  @Input() showHidden = false;
  @Input() paneActive = true;
  @Input() actionAvailability: ActionAvailability = NO_ACTIONS;
  @Input() showTabBar = true;
  @Input() showPathBar = true;
  @Input() showBreadcrumbs = true;

  @Output() pathChange = new EventEmitter<string>();
  @Output() previewTargetChange = new EventEmitter<string>();
  @Output() activeTabChange = new EventEmitter<TabInfo>();
  @Output() copyToOther = new EventEmitter<string[]>();
  @Output() moveToOther = new EventEmitter<string[]>();

  tabs: TabInfo[] = [];
  currentTabId = '';
  pathReady = false;
  archiveStack: ArchiveFrame[] = [];
  archiveMoveWarning = false;
  archivePendingMoveEntries: string[] = [];
  archivePendingMoveDestDir = '';
  lastSeenActiveTabId: string | null = null;
  lastSeenActiveTabPath: string | null = null;
  fileContextMenu: FileContextMenuState | null = null;

  private readonly navigationController: FilePaneNavigationController;
  private readonly archiveController = new FilePaneArchiveController();
  private readonly openController = new FilePaneOpenController();
  private readonly operationsController: FilePaneOperationsController;
  private readonly previewController = new FilePanePreviewController();
  private readonly directoryController = new FilePaneDirectoryController();
  private readonly dirSessionController = new FilePaneDirSessionController();
  private readonly focusController = new FilePaneFocusController();
  private readonly pathController = new FilePanePathController();
  private readonly uiController = new FilePaneUiController();
  private readonly tabsController = new FilePaneTabsController();
  private readonly stateController = new FilePaneStateController(PARENT_ENTRY_PATH, FileType.FILE, FileType.SYMLINK);
  private readonly destroy$ = createSubject<void>();
  private readonly dirRuntime: DirSurfaceRuntime = new FilePaneDirSurfaceRuntime(
    () => this.fileViewContainer,
    PARENT_ENTRY_PATH,
  );

  private get navigationHost(): FilePaneNavigationHost {
    return this.runtimeHostsFactory.createNavigationHost(this.runtimeHostSource);
  }

  private get stateHost(): FilePaneStateHost {
    return this.runtimeHostsFactory.createStateHost(this.runtimeHostSource);
  }

  private get runtimeHostSource(): FilePaneRuntimeHostsSource {
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
      apiService: this.apiService,
      workspaceService: this.workspaceService,
      filterService: this.filterService,
      directorySource: this.directorySource,
      getDirSession: () => thisRef.dir,
      isInArchive: () => thisRef.isInArchive(),
      exitArchive: () => thisRef.archiveController.exitArchive(thisRef.archiveHost),
      clearArchiveState: () => thisRef.clearArchiveState(),
      loadPathTransparently: (path: string) => thisRef.loadPathTransparently(path),
      loadDirectory: (path: string, opts?: { preserveUI?: boolean; focusPath?: string }) => thisRef.loadDirectory(path, opts),
      navigateArchiveUp: () => thisRef.archiveController.navigateArchiveUp(thisRef.archiveHost),
      refreshCurrentPathTransparently: () => thisRef.refreshCurrentPathTransparently(),
      refreshTreeNode: (path: string) => thisRef.fileViewContainer?.refreshTreeNode(path),
      renderArchiveFrame: (frame: ArchiveFrame) => thisRef.archiveController.renderArchiveFrame(thisRef.archiveHost, frame),
      applyFilter: () => thisRef.applyFilter(),
      emitPreviewTargetFromState: () => thisRef.stateRuntime.emitPreviewTargetFromState(),
      emitActiveTabChange: (tab: TabInfo) => thisRef.activeTabChange.emit(tab),
      promptForRemoteMount: async (protocol: string, alias: string) => {
        const mount = await thisRef.remoteConnectDialog.openDialog({ name: alias, protocol });
        return mount?.publicPath ?? null;
      },
      getSelectableEntries: () => thisRef.dirRuntime.getSelectableEntries(),
      getCursorEntries: () => thisRef.dirRuntime.getCursorEntries(),
      getOptimisticHiddenPaths: () => thisRef.getOptimisticHiddenPaths(),
      setOptimisticHiddenPaths: (paths: string[]) => thisRef.setOptimisticHiddenPaths(paths),
      findEntry: (path: string) => thisRef.entries.find((entry) => entry.path === path),
      emitPreviewTarget: (path: string) => thisRef.previewTargetChange.emit(path),
      setFindMatchCount: (count: number) => thisRef.findWidget?.setMatchCount(count),
      promptForMask: (mode: 'select' | 'unselect') => thisRef.confirmDialog.prompt({
        title: mode === 'select' ? 'Select By Mask' : 'Unselect By Mask',
        message: mode === 'select'
          ? 'Enter a mask, for example *.txt.'
          : 'Enter a mask to remove from the selection, for example *.txt.',
        confirmText: mode === 'select' ? 'Select' : 'Unselect',
        cancelText: 'Cancel',
        placeholder: '*.txt',
        defaultValue: '*.*',
        historyKey: 'files.mask',
      }),
      scrollToCursorPath: (path: string) => thisRef.stateRuntime.scrollToCursorPath(path),
      getGridColumns: () => thisRef.fileViewContainer?.getGridColumns() ?? 1,
      getApproxPageSize: () => thisRef.fileViewContainer?.getApproxPageSize() ?? 10,
      persistDirTabState: () => thisRef.persistDirTabState(),
    };
  }

  private get runtimeStateHost(): FilePaneRuntimeStateHost {
    const thisRef = this;
    return {
      get archiveStack() { return thisRef.archiveStack; },
      set archiveStack(value: ArchiveFrame[]) { thisRef.archiveStack = value; },
      get fsParentDir() { return thisRef.fsParentDir; },
      set fsParentDir(value: string) { thisRef.fsParentDir = value; },
      get activeDirTabId() { return thisRef.activeDirTabId; },
      get currentTabId() { return thisRef.currentTabId; },
      get activeSurfaceId() { return thisRef.activeSurfaceId; },
      isInArchive: () => thisRef.isInArchive(),
      restoreActiveDirTabState: () => thisRef.restoreActiveDirTabState(),
      saveDirTabState: (tabId: string) => {
        thisRef.dirTabStateController.save(tabId, thisRef.dirSessionController.getActiveRestoreTarget(thisRef.dirSessionHost));
      },
    };
  }

  private get stateRuntimeSource(): FilePaneStateRuntimeSource {
    return {
      stateController: this.stateController,
      stateHost: this.stateHost,
      getSelectableEntries: () => this.dirRuntime.getSelectableEntries(),
      scrollFileViewToPath: (path: string) => this.fileViewContainer?.scrollToCursorPath(path),
    };
  }

  private get surfaceContextSource(): FilePaneSurfaceContextSource {
    const thisRef = this;
    return {
      get paneId() { return thisRef.paneId; },
      get currentTabId() { return thisRef.currentTabId; },
      get tabs() { return thisRef.tabs; },
      dirVm: this.dirVmFactory.create(this.dirVmSource),
      sizeManagerVm: this.buildSizeManagerVm(),
    };
  }

  private get shortcutCommandsSource(): FilePaneShortcutCommandsSource {
    return {
      stateController: this.stateController,
      stateHost: this.stateHost,
      tabsController: this.tabsController,
      tabsHost: this.tabsHost,
      focusController: this.focusController,
      focusHost: this.focusHost,
    };
  }

  private get tabsHost(): FilePaneTabsHost {
    return this.surfaceHostsFactory.createTabsHost(this.surfaceHostSource);
  }

  private get dirSessionHost(): FilePaneDirSessionHost {
    return this.surfaceHostsFactory.createDirSessionHost(this.surfaceHostSource);
  }

  private get uiHost(): FilePaneUiHost {
    return this.surfaceHostsFactory.createUiHost(this.surfaceHostSource);
  }

  private get pathHost(): FilePanePathHost {
    return this.surfaceHostsFactory.createPathHost(this.surfaceHostSource);
  }

  private get focusHost(): FilePaneFocusHost {
    return this.surfaceHostsFactory.createFocusHost(this.surfaceHostSource);
  }

  private get surfaceHostSource(): FilePaneSurfaceHostsSource {
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
      get currentView() { return thisRef.currentView; },
      set currentView(value: ViewLayout) { thisRef.currentView = value; },
      get sortField() { return thisRef.sortField; },
      set sortField(value: string) { thisRef.sortField = value; },
      get sortDirection() { return thisRef.sortDirection; },
      set sortDirection(value: 'asc' | 'desc') { thisRef.sortDirection = value; },
      get directoriesFirst() { return thisRef.directoriesFirst; },
      set directoriesFirst(value: boolean) { thisRef.directoriesFirst = value; },
      get colorizeByMime() { return thisRef.colorizeByMime; },
      set colorizeByMime(value: boolean) { thisRef.colorizeByMime = value; },
      get selectedFiles() { return thisRef.selectedFiles; },
      set selectedFiles(value: Set<string>) { thisRef.selectedFiles = value; },
      get selectionCursorPath() { return thisRef.selectionCursorPath; },
      set selectionCursorPath(value: string | null) { thisRef.selectionCursorPath = value; },
      get selectionAnchorPath() { return thisRef.selectionAnchorPath; },
      set selectionAnchorPath(value: string | null) { thisRef.selectionAnchorPath = value; },
      directorySource: this.directorySource,
      workspaceService: this.workspaceService,
      dirTabStateController: this.dirTabStateController,
      getDirSession: (tabId: string) => thisRef.dirSessions.get(tabId),
      setDirSession: (session: DirSurfaceSession) => { thisRef.dirSessions.set(session.tabId, session); },
      isInArchive: () => thisRef.isInArchive(),
      deleteDirSession: (tabId: string) => { thisRef.dirSessions.delete(tabId); },
      deleteDirTabState: (tabId: string) => { thisRef.dirTabStateController.delete(tabId); },
      loadPaneState: () => thisRef.loadPaneState(),
      onTabChange: (tab: TabInfo) => thisRef.onTabChange(tab),
      onTabClose: (tab: TabInfo) => thisRef.onTabClose(tab),
      onTabPin: (tab: TabInfo) => thisRef.onTabPin(tab),
      getSelectableEntries: () => thisRef.dirRuntime.getSelectableEntries(),
      getSelectableEntriesPaths: () => thisRef.stateRuntime.getSelectableEntriesPaths(),
      getOptimisticHiddenPaths: () => thisRef.getOptimisticHiddenPaths(),
      setOptimisticHiddenPaths: (paths: string[]) => thisRef.setOptimisticHiddenPaths(paths),
      applyFilter: () => thisRef.applyFilter(),
      scrollToCursorPath: (path: string) => thisRef.stateRuntime.scrollToCursorPath(path),
      getExpandedPaths: () => thisRef.fileViewContainer?.getExpandedPaths() ?? [],
      setExpandedPaths: (paths: string[]) => thisRef.fileViewContainer?.setExpandedPaths(paths),
      getScrollTop: () => thisRef.fileViewContainer?.getScrollTop() ?? 0,
      setScrollTop: (top: number) => thisRef.fileViewContainer?.setScrollTop(top),
      emitPreviewTargetFromState: () => thisRef.stateRuntime.emitPreviewTargetFromState(),
      persistDirTabState: () => thisRef.persistDirTabState(),
      refreshCurrentPathTransparently: () => thisRef.refreshCurrentPathTransparently(),
      loadDirectory: (path: string, opts?: { preserveUI?: boolean; focusPath?: string }) => thisRef.loadDirectory(path, opts),
      restoreArchiveFromPath: (path: string) => thisRef.archiveController.restoreArchiveFromPath(thisRef.archiveHost, path),
      renderArchiveFrame: (frame: ArchiveFrame) => thisRef.archiveController.renderArchiveFrame(thisRef.archiveHost, frame),
      getWorkspaceTabs: () => thisRef.workspaceService.getTabs(thisRef.paneId) ?? [],
      focusAddressBar: () => thisRef.addressBar?.enterEditMode(),
      openFindWidget: () => thisRef.findWidget?.open(),
      toggleFindWidget: () => thisRef.findWidget?.toggle(),
      closeFindWidget: () => thisRef.findWidget?.close(),
    };
  }

  private get archiveHost(): FilePaneArchiveHost {
    return this.actionHostsFactory.createArchiveHost(this.actionHostSource);
  }

  private get directoryHost(): FilePaneDirectoryHost {
    return this.actionHostsFactory.createDirectoryHost(this.actionHostSource);
  }

  private get operationsHost(): FilePaneOperationsHost {
    return this.actionHostsFactory.createOperationsHost(this.actionHostSource);
  }

  private get previewHost(): FilePanePreviewHost {
    return this.actionHostsFactory.createPreviewHost(this.actionHostSource);
  }

  private get openHost(): FilePaneOpenHost {
    return this.actionHostsFactory.createOpenHost(this.actionHostSource);
  }

  private get actionHostSource(): FilePaneActionHostsSource {
    const thisRef = this;
    return {
      get paneId() { return thisRef.paneId; },
      get currentTabId() { return thisRef.currentTabId; },
      get currentPath() { return thisRef.currentPath; },
      set currentPath(value: string) { thisRef.currentPath = value; },
      get fsParentDir() { return thisRef.fsParentDir; },
      set fsParentDir(value: string) { thisRef.fsParentDir = value; },
      get activeSurfaceCapabilities() { return thisRef.activeSurfaceCapabilities; },
      get currentView() { return thisRef.currentView; },
      get defaultOpenAction() { return thisRef.settingsService.getSettings().files.defaultOpenAction; },
      get confirmBeforeDelete() { return thisRef.settingsService.getSettings().files.confirmBeforeDelete; },
      get confirmDragAndDrop() { return thisRef.settingsService.getSettings().files.confirmDragAndDrop; },
      get showHidden() { return thisRef.showHidden; },
      get entries() { return thisRef.entries; },
      getSortedEntries: () => thisRef.fileViewContainer?.getSortedEntries() ?? thisRef.entries,
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
      get tabs() { return thisRef.tabs; },
      set tabs(value: TabInfo[]) { thisRef.tabs = value; },
      get archiveStack() { return thisRef.archiveStack; },
      set archiveStack(value: ArchiveFrame[]) { thisRef.archiveStack = value; },
      get archiveMoveWarning() { return thisRef.archiveMoveWarning; },
      set archiveMoveWarning(value: boolean) { thisRef.archiveMoveWarning = value; },
      get archivePendingMoveEntries() { return thisRef.archivePendingMoveEntries; },
      set archivePendingMoveEntries(value: string[]) { thisRef.archivePendingMoveEntries = value; },
      get archivePendingMoveDestDir() { return thisRef.archivePendingMoveDestDir; },
      set archivePendingMoveDestDir(value: string) { thisRef.archivePendingMoveDestDir = value; },
      apiService: this.apiService,
      archiveService: this.archiveService,
      workspaceService: this.workspaceService,
      pathHistoryService: this.pathHistoryService,
      directorySource: this.directorySource,
      confirmDialog: this.confirmDialog,
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
      onRefresh: () => thisRef.browserCommands.navigation.refresh(),
      hasSavedDirTabState: () => !!thisRef.dirTabStateController.load(thisRef.activeDirTabId ?? thisRef.currentTabId),
      restoreActiveDirTabState: () => thisRef.restoreActiveDirTabState(),
      reapplyPendingDirTabRestore: () => thisRef.reapplyPendingDirTabRestore(),
      persistDirTabState: () => thisRef.persistDirTabState(),
      scrollToCursorPath: (path: string) => thisRef.stateRuntime.scrollToCursorPath(path),
      shouldSuppressFsRefresh: () => thisRef.fileViewContainer?.hasPendingTreeRename() ?? false,
      loadDirectory: (path: string, opts?: { preserveUI?: boolean; focusPath?: string }) => thisRef.loadDirectory(path, opts),
      refreshTreeNode: (path: string) => thisRef.fileViewContainer?.refreshTreeNode(path),
      deleteSelectedFromArchive: (effective: Set<string>) => thisRef.archiveController.deleteSelectedFromArchive(thisRef.archiveHost, effective),
      getEffectiveSelection: () => thisRef.stateRuntime.getEffectiveSelection(),
      emitCopyToOther: (paths: string[]) => thisRef.copyToOther.emit(paths),
      emitMoveToOther: (paths: string[]) => thisRef.moveToOther.emit(paths),
      startRenameSelected: () => thisRef.fileViewContainer?.startRenameSelected(),
      startRenamePath: (path: string) => thisRef.fileViewContainer?.startRenamePath(path),
      setPendingRenameDisplayName: (path: string, displayName: string | null) => thisRef.fileViewContainer?.setPendingRenameDisplayName(path, displayName),
      getVisibleTreeNodes: () => thisRef.fileViewContainer?.getVisibleTreeNodes() ?? [],
      isTreeNodeExpanded: (path: string) => thisRef.fileViewContainer?.isTreeNodeExpanded(path) ?? false,
      suppressNextTreeAutoExpand: () => thisRef.fileViewContainer?.suppressNextTreeAutoExpand(),
      focusTreeNode: (path: string) => thisRef.fileViewContainer?.focusTreeNode(path),
      onTabChange: (tab: TabInfo) => thisRef.onTabChange(tab),
      archiveNavigateInto: (path: string) => thisRef.archiveController.archiveNavigateInto(thisRef.archiveHost, path),
      openNestedArchive: (chain: string[]) => thisRef.archiveController.openNestedArchive(thisRef.archiveHost, chain),
      openRootArchive: (path: string) => thisRef.archiveController.openRootArchive(thisRef.archiveHost, path),
      openPathInNewTab: (path: string) => thisRef.openPathInNewTab(path),
      openPreviewPath: (path: string) => thisRef.previewController.openPreviewPath(thisRef.previewHost, path),
      openEditorPath: (path: string) => thisRef.previewController.openEditorPath(thisRef.previewHost, path),
    };
  }

  private get dir(): DirSurfaceSession {
    return this.dirSessionController.getActiveSession(this.dirSessionHost);
  }

  private restoreActiveDirTabState(): void {
    this.dirSessionController.restoreActiveState(this.dirSessionHost);
  }

  private reapplyPendingDirTabRestore(): void {
    this.dirSessionController.reapplyPendingRestoreToActive(this.dirSessionHost);
  }

  private getPersistedBrowserPaneSplit(): number | null {
    if (!this.currentTabId) return null;
    return this.workspaceService.getTabBrowserPaneSplit(this.paneId, this.currentTabId);
  }

  private setPersistedBrowserPaneSplit(split: number): void {
    if (!this.currentTabId) return;
    this.workspaceService.setTabBrowserPaneSplit(this.paneId, this.currentTabId, split);
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

  get colorizeByMime(): boolean { return this.dir.colorizeByMime; }
  set colorizeByMime(v: boolean) { this.dir.colorizeByMime = v; }

  get selectionCursorPath(): string | null { return this.dir.selectionCursorPath; }
  set selectionCursorPath(v: string | null) { this.dir.selectionCursorPath = v; }

  get selectionAnchorPath(): string | null { return this.dir.selectionAnchorPath; }
  set selectionAnchorPath(v: string | null) { this.dir.selectionAnchorPath = v; }

  getOptimisticHiddenPaths(): string[] { return Array.from(this.dir.optimisticHiddenPaths); }
  setOptimisticHiddenPaths(paths: string[]): void {
    this.dir.optimisticHiddenPaths = new Set((paths ?? []).filter(Boolean));
  }

  get activeSurfaceCapabilities(): ReadonlySet<SurfaceCapability> {
    const tab = this.workspaceService.getActiveTab(this.paneId)
      ?? this.tabs.find((candidate) => candidate.id === this.currentTabId);
    return tab ? this.surfaceRegistry.getCapabilities({ tab }) : new Set<SurfaceCapability>();
  }

  get activeSurfaceId(): string {
    const tab = this.workspaceService.getActiveTab(this.paneId)
      ?? this.tabs.find((candidate) => candidate.id === this.currentTabId);
    return tab ? tabSurfaceId(tab) : 'dir';
  }

  get activeViewerTargetPath(): string {
    return this.surfaceContextFactory.getActiveViewerTargetPath(this.surfaceContextSource);
  }

  get activeViewerId(): string | null {
    return this.surfaceContextFactory.getActiveViewerId(this.surfaceContextSource);
  }

  get activeViewerArchiveContext(): ViewerArchiveContext | null {
    return this.surfaceContextFactory.getActiveViewerArchiveContext(this.surfaceContextSource);
  }

  constructor(
    private apiService: ApiService,
    private workspaceService: WorkspaceService,
    private filterService: FilterService,
    private pathHistoryService: PathHistoryService,
    private directorySource: DirectorySourceService,
    private confirmDialog: ConfirmDialogService,
    private remoteConnectDialog: RemoteConnectDialogService,
    private operationService: OperationService,
    private batchRenameDialog: BatchRenameDialogService,
    private settingsService: SettingsService,
    private zone: NgZone,
    private cdr: ChangeDetectorRef,
    private archiveService: ArchiveService,
    private surfaceRegistry: SurfaceRegistryService,
    private pluginManager: PluginManagerService,
    private inputEngine: InputEngineService,
  ) {
    this.dirStream = new DirListStreamController(this.apiService);
    this.operationsController = new FilePaneOperationsController(this.operationService, this.batchRenameDialog);
    this.navigationController = new FilePaneNavigationController();
  }

  get allTabSurfaceCtxs(): Array<{ ctx: SurfaceContext; active: boolean }> {
    const orderedTabs = this.tabs
      .slice()
      .sort((left, right) => {
        const leftActive = left.id === this.currentTabId ? 1 : 0;
        const rightActive = right.id === this.currentTabId ? 1 : 0;
        return leftActive - rightActive;
      });

    return orderedTabs.map((tab) => ({
      ctx: this.surfaceContextFactory.getSurfaceContextForTab(this.surfaceContextSource, tab),
      active: tab.id === this.currentTabId,
    }));
  }

  trackTabSurface(_index: number, item: { ctx: SurfaceContext; active: boolean }): string {
    return item.ctx.tab.id;
  }

  ngOnInit(): void  {
    this.dirTabStateController.setPersistCallback((tabId, json) => {
      this.workspaceService.setTabViewState(this.paneId, tabId, json);
    });
    this.hydrateDirTabStatesFromWorkspace();

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
        this.hydrateDirTabStatesFromWorkspace();
        this.tabsController.onWorkspaceUpdate(this.tabsHost, ws);
      });
  }

  private hydrateDirTabStatesFromWorkspace(): void {
    const tabs = this.workspaceService.getTabs(this.paneId);
    for (const tab of tabs) {
      if ((tab.kind ?? 'dir') !== 'dir') continue;
      if (tab.viewStateJson) {
        this.dirTabStateController.hydrateFromJson(tab.id, tab.viewStateJson);
      }
    }
  }
  
  ngOnChanges(changes: SimpleChanges): void {
    if (changes['workspace']) {
      // The input is bound to FileBrowserComponent.activeWorkspace which starts as null.
      // That means the *first* change Angular sees is often null, and the transition
      // to a real workspace would be a later change. Always load once we have a
      // non-null workspace so tabs/pane render without requiring a user click.
      if (this.workspace) {
        this.hydrateDirTabStatesFromWorkspace();
        this.uiController.loadViewPreference(this.uiHost);
        this.lastSeenActiveTabId = null;
        this.lastSeenActiveTabPath = null;
        this.loadPaneState();
      }
      return;
    }

    if (changes['showHidden'] && !changes['showHidden'].firstChange) {
      this.browserCommands.navigation.refresh();
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

  // Fired when a dynamic surface notifies that first data arrived.
  onSurfaceFirstEntry(): void {
    // Trigger a change detection pass so parent UI (loaders/overlays) can update.
    this.cdr.markForCheck();
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

  onColorizeByMimeChange(value: boolean): void {
    this.uiController.onColorizeByMimeChange(this.uiHost, value);
  }

  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void {
    const mouse = event.event;
    const entry = event.entry;
    mouse.preventDefault();
    mouse.stopPropagation();

    if (!entry?.path || entry.path === PARENT_ENTRY_PATH) {
      this.closeFileContextMenu();
      return;
    }

    // Context-menu commands operate on the clicked item unless it is already
    // part of a multi-selection. This keeps right-click behavior consistent
    // across list/grid/details/tree views.
    if (!this.selectedFiles.has(entry.path)) {
      this.onSelectionChange([entry]);
    }

    const x = Math.max(4, Math.min(mouse.clientX, Math.max(4, window.innerWidth - 244)));
    const y = Math.max(4, Math.min(mouse.clientY, Math.max(4, window.innerHeight - 360)));

    // Let the parent browser recompute ActionAvailability after the selection
    // mutation before freezing the menu snapshot.
    setTimeout(() => {
      const selectedPaths = Array.from(this.selectedFiles).filter(path => path && path !== PARENT_ENTRY_PATH);
      const context: MenuContext = {
        targetPath: entry.path,
        selectedPaths,
        currentDirectory: this.currentPath,
      };
      const builtins = buildBuiltinFileContextMenu({
        availability: this.actionAvailability,
        selectedCount: selectedPaths.length,
        isDirectory: entry.type === FileType.DIRECTORY,
      }).map(item => bindCommandPresentation(
        item,
        () => this.executeBuiltinContextAction(item.id, entry),
      ));

      const plugins = this.pluginManager.resolveContextMenuItems(context).map((item, index) =>
        presentPluginCommand(item, context, this.inputEngine, { separatorBefore: index === 0 }),
      );

      this.fileContextMenu = { x, y, items: [...builtins, ...plugins] };
      this.cdr.markForCheck();
    }, 0);
  }

  onBackgroundContextMenu(mouse: MouseEvent): void {
    mouse.preventDefault();
    mouse.stopPropagation();
    this.onSelectionChange([]);

    const x = Math.max(4, Math.min(mouse.clientX, Math.max(4, window.innerWidth - 244)));
    const y = Math.max(4, Math.min(mouse.clientY, Math.max(4, window.innerHeight - 180)));

    setTimeout(() => {
      const context: MenuContext = {
        targetPath: this.currentPath,
        selectedPaths: [],
        currentDirectory: this.currentPath,
      };
      const builtins = buildBackgroundFileContextMenu(this.actionAvailability).map(item => bindCommandPresentation(
        item,
        () => this.executeBackgroundContextAction(item.id),
      ));
      const plugins = this.pluginManager.resolveContextMenuItems(context).map((item, index) =>
        presentPluginCommand(item, context, this.inputEngine, { separatorBefore: index === 0 }),
      );
      this.fileContextMenu = { x, y, items: [...builtins, ...plugins] };
      this.cdr.markForCheck();
    }, 0);
  }

  private executeBackgroundContextAction(action: BackgroundFileContextAction): void {
    const commandId = {
      newFolder: BC.newFolder,
      paste: BC.paste,
      refresh: BC.refresh,
    } satisfies Record<BackgroundFileContextAction, string>;
    this.inputEngine.dispatchCommandById(commandId[action]);
  }

  closeFileContextMenu(): void {
    if (!this.fileContextMenu) return;
    this.fileContextMenu = null;
    this.cdr.markForCheck();
  }

  private executeBuiltinContextAction(action: BuiltinFileContextAction, entry: FileInfo): void | Promise<void> {
    if (action === 'openInNewTab') {
      return this.openEntryInNewTab(entry);
    }

    const commandId = {
      open: BC.openItem,
      quickView: BC.quickView,
      copyPath: BC.copyCursorPath,
      openWithExternal: BC.openWithExternal,
      copy: BC.copy,
      cut: BC.cut,
      delete: BC.deleteSelected,
      rename: BC.rename,
      copyToOther: BC.copyToOtherPane,
      moveToOther: BC.moveToOtherPane,
    } satisfies Record<Exclude<BuiltinFileContextAction, 'openInNewTab'>, string>;

    this.inputEngine.dispatchCommandById(commandId[action]);
  }

  onNodeDoubleClick(entry: FileInfo): void {
    this.openController.onNodeDoubleClick(this.openHost, entry, () => this.browserCommands.navigation.navigateUp());
  }

  zoomActiveViewerIn(): void {
    this.viewer?.zoomIn?.();
  }

  zoomActiveViewerOut(): void {
    this.viewer?.zoomOut?.();
  }

  resetActiveViewerZoom(): void {
    this.viewer?.resetZoom?.();
  }

  deleteInActiveViewer(): void {
    this.viewer?.deleteSelected?.();
  }

  async openSearchResult(result: SearchResultItem): Promise<void> {
    const targetPath = normalizePath(result.file.path);
    const focusPath = result.file.type === FileType.DIRECTORY ? targetPath : result.file.path;
    const directoryPath = result.file.type === FileType.DIRECTORY
      ? targetPath
      : normalizePath(targetPath.replace(/[/\\][^/\\]+$/, '') || targetPath);

    await this.loadDirectory(directoryPath, { focusPath });
    this.focusController.focusCursorPath(this.focusHost, focusPath);
    this.runtimeStateController.afterNavigationMutation(this.runtimeStateHost);
  }

  private withParentEntry(raw: FileInfo[], inArchive: boolean, fsPath: string): FileInfo[] {
    return this.pathController.withParentEntry(raw, inArchive, fsPath, this.directorySource);
  }

  private async loadPathTransparently(path: string): Promise<void> {
    await this.pathController.loadPathTransparently(this.pathHost, path);
  }

  private async refreshCurrentPathTransparently(): Promise<void> {
    await this.pathController.refreshCurrentPathTransparently(this.pathHost);
  }

  // в”Ђв”Ђ Pane state в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  private async loadPaneState(): Promise<void> {
    await this.navigationController.loadPaneState(this.navigationHost);
    this.runInAngular(() => {
      this.runtimeStateController.afterNavigationMutation(this.runtimeStateHost);
    });
  }

  private clearArchiveState(): void {
    this.runtimeStateController.clearArchiveState(this.runtimeStateHost);
  }

  private async loadDirectory(path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void> {
    await this.directoryController.loadDirectory(this.directoryHost, path, opts);
  }

  // в”Ђв”Ђ Tab management в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  async onTabChange(tab: TabInfo): Promise<void> {
    await this.navigationController.onTabChange(this.navigationHost, tab);
    this.runInAngular(() => {
      this.runtimeStateController.afterNavigationMutation(this.runtimeStateHost);
    });
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

  async openPinnedLocation(pin: PinnedLocation): Promise<void> {
    const surfaceId = pin.surfaceId?.trim() || 'dir';
    if (surfaceId === 'dir') {
      this.onPathChange(pin.location);
      return;
    }

    const tab = this.workspaceService.addSurfaceTab(this.paneId, pin.location, surfaceId, {
      title: pin.label,
      activate: true,
    });
    if (!tab) return;

    this.tabs = this.workspaceService.getTabs(this.paneId);
    await this.onTabChange(tab);
  }

  async openPathInNewTab(path: string): Promise<void> {
    try {
      const probe = await this.apiService.probeFile(path);
      if (probe?.isArchive) {
        await this.archiveController.openRootArchive(this.archiveHost, path);
        return;
      }
    } catch {
      // fall back to the standard tab path
    }

    await this.tabsController.openPathInNewTab(this.tabsHost, path);
  }

  // в”Ђв”Ђ Address bar / path в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onPathChange(path: string): void {
    this.navigationController.onPathChange(this.navigationHost, path);
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

  private async handleArchiveSegmentClick(virtualPath: string): Promise<void> {
    await this.archiveController.onArchiveSegmentClick(this.archiveHost, virtualPath);
  }

  // в”Ђв”Ђ Navigation в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onNavigate(entry: FileInfo): void {
    this.openController.onNavigate(this.openHost, entry, () => this.browserCommands.navigation.navigateUp());
  }

  private async openEntryInNewTab(entry: FileInfo): Promise<void> {
    await this.openController.onOpenInNewTab(this.openHost, entry);
  }

  private async renameEntry(event: { entry: FileInfo; newName: string }): Promise<void> {
    await this.operationsController.onRename(this.operationsHost, event);
  }

  private async handleDragDrop(event: { event: DragEvent; source: FileInfo[]; targetPath: string }): Promise<void> {
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
    this.runtimeStateController.persistDirTabState(this.runtimeStateHost);
  }

  // в”Ђв”Ђ Selection в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onSelectionChange(entries: FileInfo[]): void {
    this.stateController.onSelectionChange(this.stateHost, entries);
  }

  measureCurrentDirectorySize(): void {
    if (this.activeSurfaceId !== 'dir' || this.isInArchive()) return;

    const path = this.selectionCursorPath ?? (this.selectedFiles.size === 1 ? Array.from(this.selectedFiles)[0] : null);
    if (!path || path === PARENT_ENTRY_PATH) return;

    const entry = this.entries.find(candidate => candidate.path === path);
    if (!entry || entry.type !== FileType.DIRECTORY) return;

    void this.fileViewContainer?.calculateDirectorySize(path);
  }

  measureCurrentDirectorySizeAndMoveDown(): void {
    this.measureCurrentDirectorySize();
    this.shortcuts.selection.toggleCurrentAndMoveDown();
  }

  // в”Ђв”Ђ File operations в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  /**
   * Returns archive metadata for the current tree-view selection when the
   * selected nodes are inside an expanded archive node (not in archive-pane
   * mode). All selected nodes must share the same archive FS path and nested
   * chain; otherwise returns null.
   */
  getTreeArchiveSelectionContext(): { archivePath: string; innerPaths: string[] } | null {
    return this.operationsController.getTreeArchiveSelectionContext(this.operationsHost);
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


  // ── Disk Usage Analyzer ───────────────────────────────────────────────────

  openDiskUsageAnalyzer(path?: string): void {
    const analysisPath = this.resolveDiskUsageAnalysisPath(path);
    const tab = this.workspaceService.addSizeManagerTab(this.paneId, analysisPath, { activate: true });
    if (tab) {
      this.tabs = this.workspaceService.getTabs(this.paneId);
      this.currentTabId = tab.id;
      this.cdr.markForCheck();
    }
  }

  /** Called by the other pane to sync the active size-manager tab to a new path. */
  followSizeManagerNavigation(path: string): void {
    const tab = this.tabs.find(t => t.id === this.currentTabId);
    if (!tab || (tab.kind ?? 'dir') !== 'size-manager') return;
    if (tab.path === path) return;
    tab.path = path;
    this.workspaceService.updateTabPathAndTitle(this.paneId, tab.id, path, tab.title || 'Storage Footprint');
    this.tabs = this.workspaceService.getTabs(this.paneId);
    this.cdr.markForCheck();
  }

  private resolveDiskUsageAnalysisPath(path?: string): string {
    if (path) return path;
    return this.currentPath || '/';
  }

  private buildSizeManagerVm(): SizeManagerSurfaceVm | null {
    const tab = this.tabs.find(t => t.id === this.currentTabId);
    if (!tab || (tab.kind ?? 'dir') !== 'size-manager') return null;
    const thisRef = this;
    return {
      paneId: this.paneId,
      analysisPath: tab.path || this.currentPath || '/',
      onAnalyse(p: string) {
        tab.path = p;
        thisRef.cdr.markForCheck();
      },
      onClose() {
        thisRef.workspaceService.closeTab(thisRef.paneId, tab.id);
        thisRef.tabs = thisRef.workspaceService.getTabs(thisRef.paneId);
        thisRef.currentTabId = thisRef.workspaceService.getActiveTab(thisRef.paneId)?.id ?? thisRef.tabs[0]?.id ?? '';
        thisRef.cdr.markForCheck();
      },
    };
  }

}
