import type { ArchiveFrame } from '../services/archive.service';
import type { FileViewDirectorySource } from '../services/directory-source.service';
import type { WorkspaceService } from '../services/workspace.service';
import type { DirSurfaceSession } from '../surfaces/dir/dir-surface-session';
import type { DirTabStateController } from '../surfaces/dir/dir-tab-state-controller';
import type { FileInfo } from '../shared/protocol';
import type { TabInfo } from '../shared/protocol-enhanced';
import type { ViewLayout } from './file-view-container.component';
import type { FilePaneDirSessionHost } from './file-pane-dir-session.controller';
import type { FilePaneFocusHost } from './file-pane-focus.controller';
import type { FilePanePathHost } from './file-pane-path.controller';
import type { FilePaneTabsHost } from './file-pane-tabs.controller';
import type { FilePaneUiHost } from './file-pane-ui.controller';

export interface FilePaneSurfaceHostsSource {
  get paneId(): 'left' | 'right';
  get currentTabId(): string;
  set currentTabId(value: string);
  get currentPath(): string;
  get tabs(): TabInfo[];
  set tabs(value: TabInfo[]);
  get activeDirTabId(): string | null;
  set activeDirTabId(value: string | null);
  get archiveStack(): ArchiveFrame[];
  get fsParentDir(): string;
  get lastSeenActiveTabId(): string | null;
  set lastSeenActiveTabId(value: string | null);
  get lastSeenActiveTabPath(): string | null;
  set lastSeenActiveTabPath(value: string | null);
  get currentView(): ViewLayout;
  set currentView(value: ViewLayout);
  get sortField(): string;
  set sortField(value: string);
  get sortDirection(): 'asc' | 'desc';
  set sortDirection(value: 'asc' | 'desc');
  get directoriesFirst(): boolean;
  set directoriesFirst(value: boolean);
  get colorizeByMime(): boolean;
  set colorizeByMime(value: boolean);
  get selectedFiles(): Set<string>;
  set selectedFiles(value: Set<string>);
  get selectionCursorPath(): string | null;
  set selectionCursorPath(value: string | null);
  get selectionAnchorPath(): string | null;
  set selectionAnchorPath(value: string | null);
  directorySource: FileViewDirectorySource;
  workspaceService: WorkspaceService;
  dirTabStateController: DirTabStateController;
  getDirSession(tabId: string): DirSurfaceSession | undefined;
  setDirSession(session: DirSurfaceSession): void;
  isInArchive(): boolean;
  deleteDirSession(tabId: string): void;
  deleteDirTabState(tabId: string): void;
  loadPaneState(): Promise<void>;
  onTabChange(tab: TabInfo): Promise<void>;
  onTabClose(tab: TabInfo): void;
  onTabPin(tab: TabInfo): void;
  getSelectableEntries(): Array<{ path: string }>;
  getSelectableEntriesPaths(): string[];
  getOptimisticHiddenPaths(): string[];
  setOptimisticHiddenPaths(paths: string[]): void;
  applyFilter(): void;
  scrollToCursorPath(path: string): void;
  emitPreviewTargetFromState(): void;
  persistDirTabState(): void;
  refreshCurrentPathTransparently(): Promise<void>;
  loadDirectory(path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void>;
  restoreArchiveFromPath(path: string): Promise<void>;
  renderArchiveFrame(frame: ArchiveFrame): void;
  getWorkspaceTabs(): TabInfo[];
  focusAddressBar(): void;
  openFindWidget(): void;
  toggleFindWidget(): void;
  closeFindWidget(): void;
}

export class FilePaneSurfaceHostsFactory {
  createTabsHost(source: FilePaneSurfaceHostsSource): FilePaneTabsHost {
    return {
      get paneId() { return source.paneId; },
      get currentTabId() { return source.currentTabId; },
      set currentTabId(value: string) { source.currentTabId = value; },
      get currentPath() { return source.currentPath; },
      get tabs() { return source.tabs; },
      set tabs(value: TabInfo[]) { source.tabs = value; },
      get activeDirTabId() { return source.activeDirTabId; },
      set activeDirTabId(value: string | null) { source.activeDirTabId = value; },
      get archiveStack() { return source.archiveStack; },
      get fsParentDir() { return source.fsParentDir; },
      get lastSeenActiveTabId() { return source.lastSeenActiveTabId; },
      set lastSeenActiveTabId(value: string | null) { source.lastSeenActiveTabId = value; },
      get lastSeenActiveTabPath() { return source.lastSeenActiveTabPath; },
      set lastSeenActiveTabPath(value: string | null) { source.lastSeenActiveTabPath = value; },
      workspaceService: source.workspaceService,
      isInArchive: () => source.isInArchive(),
      deleteDirSession: (tabId: string) => source.deleteDirSession(tabId),
      deleteDirTabState: (tabId: string) => source.deleteDirTabState(tabId),
      loadPaneState: () => source.loadPaneState(),
      onTabChange: (tab: TabInfo) => source.onTabChange(tab),
      onTabClose: (tab: TabInfo) => source.onTabClose(tab),
      onTabPin: (tab: TabInfo) => source.onTabPin(tab),
    };
  }

  createDirSessionHost(source: FilePaneSurfaceHostsSource): FilePaneDirSessionHost {
    return {
      get currentTabId() { return source.currentTabId; },
      get activeDirTabId() { return source.activeDirTabId; },
      set activeDirTabId(value: string | null) { source.activeDirTabId = value; },
      get tabs() { return source.tabs; },
      dirTabStateController: source.dirTabStateController,
      getDirSession: (tabId: string) => source.getDirSession(tabId),
      setDirSession: (session: DirSurfaceSession) => source.setDirSession(session),
      isInArchive: () => source.isInArchive(),
      getSelectableEntriesPaths: () => source.getSelectableEntriesPaths(),
      applyFilter: () => source.applyFilter(),
      scrollToCursorPath: (path: string) => source.scrollToCursorPath(path),
      emitPreviewTargetFromState: () => source.emitPreviewTargetFromState(),
    };
  }

  createUiHost(source: FilePaneSurfaceHostsSource): FilePaneUiHost {
    return {
      get paneId() { return source.paneId; },
      get currentView() { return source.currentView; },
      set currentView(value: ViewLayout) { source.currentView = value; },
      get sortField() { return source.sortField; },
      set sortField(value: string) { source.sortField = value; },
      get sortDirection() { return source.sortDirection; },
      set sortDirection(value: 'asc' | 'desc') { source.sortDirection = value; },
      get directoriesFirst() { return source.directoriesFirst; },
      set directoriesFirst(value: boolean) { source.directoriesFirst = value; },
      get colorizeByMime() { return source.colorizeByMime; },
      set colorizeByMime(value: boolean) { source.colorizeByMime = value; },
      get selectedFiles() { return source.selectedFiles; },
      set selectedFiles(value: Set<string>) { source.selectedFiles = value; },
      getOptimisticHiddenPaths: () => source.getOptimisticHiddenPaths(),
      setOptimisticHiddenPaths: (paths: string[]) => source.setOptimisticHiddenPaths(paths),
      applyFilter: () => source.applyFilter(),
      persistDirTabState: () => source.persistDirTabState(),
      refreshCurrentPathTransparently: () => source.refreshCurrentPathTransparently(),
    };
  }

  createPathHost(source: FilePaneSurfaceHostsSource): FilePanePathHost {
    return {
      get currentPath() { return source.currentPath; },
      get archiveStack() { return source.archiveStack; },
      directorySource: source.directorySource,
      isInArchive: () => source.isInArchive(),
      loadDirectory: (path: string, opts?: { preserveUI?: boolean; focusPath?: string }) => source.loadDirectory(path, opts),
      restoreArchiveFromPath: (path: string) => source.restoreArchiveFromPath(path),
      renderArchiveFrame: (frame: ArchiveFrame) => source.renderArchiveFrame(frame),
    };
  }

  createFocusHost(source: FilePaneSurfaceHostsSource): FilePaneFocusHost {
    return {
      get currentTabId() { return source.currentTabId; },
      get paneId() { return source.paneId; },
      get selectionCursorPath() { return source.selectionCursorPath; },
      set selectionCursorPath(value: string | null) { source.selectionCursorPath = value; },
      get selectionAnchorPath() { return source.selectionAnchorPath; },
      set selectionAnchorPath(value: string | null) { source.selectionAnchorPath = value; },
      get selectedFiles() { return source.selectedFiles; },
      getSelectableEntries: () => source.getSelectableEntries(),
      scrollToCursorPath: (path: string) => source.scrollToCursorPath(path),
      emitPreviewTargetFromState: () => source.emitPreviewTargetFromState(),
      getWorkspaceTabs: () => source.getWorkspaceTabs(),
      onTabChange: (tab: TabInfo) => source.onTabChange(tab),
      focusAddressBar: () => source.focusAddressBar(),
      openFindWidget: () => source.openFindWidget(),
      toggleFindWidget: () => source.toggleFindWidget(),
      closeFindWidget: () => source.closeFindWidget(),
    };
  }
}