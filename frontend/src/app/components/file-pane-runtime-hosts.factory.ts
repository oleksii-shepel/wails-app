import type { ApiService } from '../services/api.service';
import type { ArchiveFrame } from '../services/archive.service';
import type { FilterService } from '../services/filter.service';
import type { WorkspaceService } from '../services/workspace.service';
import type { DirSurfaceSession } from '../surfaces/dir/dir-surface-session';
import type { FileInfo, FileType } from '../shared/protocol';
import type { FilterQuery, TabInfo, WorkspaceConfig } from '../shared/protocol-enhanced';
import type { FilePaneNavigationHost } from './file-pane-navigation.controller';
import type { FilePaneStateHost } from './file-pane-state.controller';
import type { ViewLayout } from './file-view-container.component';

export interface FilePaneRuntimeHostsSource {
  get paneId(): 'left' | 'right';
  get workspace(): WorkspaceConfig | null;
  get showHidden(): boolean;
  get currentTabId(): string;
  set currentTabId(value: string);
  get currentPath(): string;
  set currentPath(value: string);
  get pathReady(): boolean;
  set pathReady(value: boolean);
  get tabs(): TabInfo[];
  set tabs(value: TabInfo[]);
  get activeDirTabId(): string | null;
  set activeDirTabId(value: string | null);
  get archiveStack(): ArchiveFrame[];
  set archiveStack(value: ArchiveFrame[]);
  get entries(): FileInfo[];
  set entries(value: FileInfo[]);
  get filteredEntries(): FileInfo[];
  set filteredEntries(value: FileInfo[]);
  get selectedFiles(): Set<string>;
  set selectedFiles(value: Set<string>);
  get selectionCursorPath(): string | null;
  set selectionCursorPath(value: string | null);
  get selectionAnchorPath(): string | null;
  set selectionAnchorPath(value: string | null);
  get filterQuery(): FilterQuery;
  set filterQuery(value: FilterQuery);
  get currentView(): ViewLayout;
  set currentView(value: ViewLayout);
  get lastPreviewTargetEmitted(): string | null;
  set lastPreviewTargetEmitted(value: string | null);
  apiService: ApiService;
  workspaceService: WorkspaceService;
  filterService: FilterService;
  getDirSession(): DirSurfaceSession;
  isInArchive(): boolean;
  exitArchive(): void;
  clearArchiveState(): void;
  pathIsInsideArchive(path: string): boolean;
  loadPathTransparently(path: string): Promise<void>;
  loadDirectory(path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void>;
  navigateArchiveUp(): Promise<void>;
  refreshCurrentPathTransparently(): Promise<void>;
  refreshTreeNode(path: string): void;
  renderArchiveFrame(frame: ArchiveFrame): void;
  applyFilter(): void;
  emitPreviewTargetFromState(): void;
  emitActiveTabChange(tab: TabInfo): void;
  getSelectableEntries(): FileInfo[];
  getOptimisticHiddenPaths(): string[];
  setOptimisticHiddenPaths(paths: string[]): void;
  findEntry(path: string): FileInfo | undefined;
  emitPreviewTarget(path: string): void;
  setFindMatchCount(count: number): void;
  promptForMask(mode: 'select' | 'unselect'): string | null;
  scrollToCursorPath(path: string): void;
  getGridColumns(): number;
  getApproxPageSize(): number;
  persistDirTabState(): void;
}

export class FilePaneRuntimeHostsFactory {
  createNavigationHost(source: FilePaneRuntimeHostsSource): FilePaneNavigationHost {
    return {
      get paneId() { return source.paneId; },
      get workspace() { return source.workspace; },
      get showHidden() { return source.showHidden; },
      get currentTabId() { return source.currentTabId; },
      set currentTabId(value: string) { source.currentTabId = value; },
      get currentPath() { return source.currentPath; },
      set currentPath(value: string) { source.currentPath = value; },
      get pathReady() { return source.pathReady; },
      set pathReady(value: boolean) { source.pathReady = value; },
      get tabs() { return source.tabs; },
      set tabs(value: TabInfo[]) { source.tabs = value; },
      get activeDirTabId() { return source.activeDirTabId; },
      set activeDirTabId(value: string | null) { source.activeDirTabId = value; },
      get archiveStack() { return source.archiveStack; },
      set archiveStack(value: ArchiveFrame[]) { source.archiveStack = value; },
      apiService: source.apiService,
      workspaceService: source.workspaceService,
      getDirSession: () => source.getDirSession(),
      isInArchive: () => source.isInArchive(),
      exitArchive: () => source.exitArchive(),
      clearArchiveState: () => source.clearArchiveState(),
      pathIsInsideArchive: (path: string) => source.pathIsInsideArchive(path),
      loadPathTransparently: (path: string) => source.loadPathTransparently(path),
      loadDirectory: (path: string, opts?: { preserveUI?: boolean; focusPath?: string }) => source.loadDirectory(path, opts),
      navigateArchiveUp: () => source.navigateArchiveUp(),
      refreshCurrentPathTransparently: () => source.refreshCurrentPathTransparently(),
      refreshTreeNode: (path: string) => source.refreshTreeNode(path),
      renderArchiveFrame: (frame: ArchiveFrame) => source.renderArchiveFrame(frame),
      applyFilter: () => source.applyFilter(),
      emitPreviewTargetFromState: () => source.emitPreviewTargetFromState(),
      emitActiveTabChange: (tab: TabInfo) => source.emitActiveTabChange(tab),
    };
  }

  createStateHost(source: FilePaneRuntimeHostsSource): FilePaneStateHost {
    return {
      get entries() { return source.entries; },
      set entries(value: FileInfo[]) { source.entries = value; },
      get filteredEntries() { return source.filteredEntries; },
      set filteredEntries(value: FileInfo[]) { source.filteredEntries = value; },
      get selectedFiles() { return source.selectedFiles; },
      set selectedFiles(value: Set<string>) { source.selectedFiles = value; },
      get selectionCursorPath() { return source.selectionCursorPath; },
      set selectionCursorPath(value: string | null) { source.selectionCursorPath = value; },
      get selectionAnchorPath() { return source.selectionAnchorPath; },
      set selectionAnchorPath(value: string | null) { source.selectionAnchorPath = value; },
      get filterQuery() { return source.filterQuery; },
      set filterQuery(value: FilterQuery) { source.filterQuery = value; },
      get currentView() { return source.currentView; },
      set currentView(value: ViewLayout) { source.currentView = value; },
      get lastPreviewTargetEmitted() { return source.lastPreviewTargetEmitted; },
      set lastPreviewTargetEmitted(value: string | null) { source.lastPreviewTargetEmitted = value; },
      isInArchive: () => source.isInArchive(),
      getSelectableEntries: () => source.getSelectableEntries(),
      getOptimisticHiddenPaths: () => source.getOptimisticHiddenPaths(),
      setOptimisticHiddenPaths: (paths: string[]) => source.setOptimisticHiddenPaths(paths),
      parseFilterText: (text: string) => source.filterService.parseFilterText(text),
      filterEntries: (entries: FileInfo[], query: FilterQuery) => source.filterService.filterEntries(entries, query),
      findEntry: (path: string) => source.findEntry(path),
      emitPreviewTarget: (path: string) => source.emitPreviewTarget(path),
      setFindMatchCount: (count: number) => source.setFindMatchCount(count),
      promptForMask: (mode: 'select' | 'unselect') => source.promptForMask(mode),
      scrollToCursorPath: (path: string) => source.scrollToCursorPath(path),
      getGridColumns: () => source.getGridColumns(),
      getApproxPageSize: () => source.getApproxPageSize(),
      persistDirTabState: () => source.persistDirTabState(),
    };
  }
}