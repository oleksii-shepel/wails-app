import type { ApiService } from '../services/api.service';
import type { ArchiveFrame, ArchiveService } from '../services/archive.service';
import type { ConfirmDialogService } from '../services/confirm-dialog.service';
import type { FileViewDirectorySource } from '../services/directory-source.service';
import type { PathHistoryService } from '../services/path-history.service';
import type { WorkspaceService } from '../services/workspace.service';
import type { DirListStreamController } from '../surfaces/dir/dir-list-stream.controller';
import type { FileInfo } from '../shared/protocol';
import type { TabInfo } from '../shared/protocol-enhanced';
import type { FilePaneArchiveHost } from './file-pane-archive.controller';
import type { FilePaneDirectoryHost } from './file-pane-directory.controller';
import type { FilePaneOpenHost } from './file-pane-open.controller';
import type { FilePaneOperationsHost } from './file-pane-operations.controller';
import type { FilePanePreviewHost } from './file-pane-preview.controller';
import type { ViewLayout } from './file-view-container.component';

export interface FilePaneActionHostsSource {
  get paneId(): 'left' | 'right';
  get currentTabId(): string;
  get currentPath(): string;
  set currentPath(value: string);
  get fsParentDir(): string;
  set fsParentDir(value: string);
  get activeTabKind(): 'dir' | 'viewer';
  get currentView(): ViewLayout;
  get showHidden(): boolean;
  get entries(): FileInfo[];
  set entries(value: FileInfo[]);
  get filteredEntries(): FileInfo[];
  get selectedFiles(): Set<string>;
  set selectedFiles(value: Set<string>);
  get selectionCursorPath(): string | null;
  set selectionCursorPath(value: string | null);
  get selectionAnchorPath(): string | null;
  set selectionAnchorPath(value: string | null);
  get loading(): boolean;
  set loading(value: boolean);
  get error(): string | null;
  set error(value: string | null);
  get tabs(): TabInfo[];
  set tabs(value: TabInfo[]);
  get archiveStack(): ArchiveFrame[];
  set archiveStack(value: ArchiveFrame[]);
  get archiveMoveWarning(): boolean;
  set archiveMoveWarning(value: boolean);
  get archivePendingMoveEntries(): string[];
  set archivePendingMoveEntries(value: string[]);
  get archivePendingMoveDestDir(): string;
  set archivePendingMoveDestDir(value: string);
  apiService: ApiService;
  archiveService: ArchiveService;
  workspaceService: WorkspaceService;
  pathHistoryService: PathHistoryService;
  directorySource: FileViewDirectorySource;
  confirmDialog: ConfirmDialogService;
  dirStream: DirListStreamController;
  isInArchive(): boolean;
  exitArchive(): void;
  clearArchiveState(): void;
  withParentEntry(raw: FileInfo[], inArchive: boolean, fsPath: string): FileInfo[];
  applyFilter(): void;
  setShowHiddenLoaded(showHidden: boolean): void;
  emitPathChange(path: string): void;
  runInAngular(update: () => void): void;
  onRefresh(): void;
  hasSavedDirTabState(): boolean;
  restoreActiveDirTabState(): void;
  persistDirTabState(): void;
  scrollToCursorPath(path: string): void;
  focusTreeNode(path: string): void;
  loadDirectory(path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void>;
  deleteSelectedFromArchive(effective: Set<string>): Promise<boolean>;
  getEffectiveSelection(): Set<string>;
  emitCopyToOther(paths: string[]): void;
  emitMoveToOther(paths: string[]): void;
  startRenameSelected(): void;
  getVisibleTreeNodes(): any[];
  onTabChange(tab: TabInfo): Promise<void>;
  archiveNavigateInto(path: string): Promise<void>;
  openNestedArchive(chain: string[]): Promise<void>;
  openRootArchive(path: string): Promise<void>;
  openPathInNewTab(path: string): Promise<void>;
}

export class FilePaneActionHostsFactory {
  createArchiveHost(source: FilePaneActionHostsSource): FilePaneArchiveHost {
    return {
      get paneId() { return source.paneId; },
      get currentTabId() { return source.currentTabId; },
      get currentPath() { return source.currentPath; },
      set currentPath(value: string) { source.currentPath = value; },
      get fsParentDir() { return source.fsParentDir; },
      set fsParentDir(value: string) { source.fsParentDir = value; },
      get archiveStack() { return source.archiveStack; },
      set archiveStack(value: ArchiveFrame[]) { source.archiveStack = value; },
      get entries() { return source.entries; },
      set entries(value: FileInfo[]) { source.entries = value; },
      get selectedFiles() { return source.selectedFiles; },
      set selectedFiles(value: Set<string>) { source.selectedFiles = value; },
      get loading() { return source.loading; },
      set loading(value: boolean) { source.loading = value; },
      get error() { return source.error; },
      set error(value: string | null) { source.error = value; },
      get tabs() { return source.tabs; },
      set tabs(value: TabInfo[]) { source.tabs = value; },
      get archiveMoveWarning() { return source.archiveMoveWarning; },
      set archiveMoveWarning(value: boolean) { source.archiveMoveWarning = value; },
      get archivePendingMoveEntries() { return source.archivePendingMoveEntries; },
      set archivePendingMoveEntries(value: string[]) { source.archivePendingMoveEntries = value; },
      get archivePendingMoveDestDir() { return source.archivePendingMoveDestDir; },
      set archivePendingMoveDestDir(value: string) { source.archivePendingMoveDestDir = value; },
      archiveService: source.archiveService,
      workspaceService: source.workspaceService,
      confirmDialog: source.confirmDialog,
      withParentEntry: (raw: FileInfo[], inArchive: boolean, fsPath: string) => source.withParentEntry(raw, inArchive, fsPath),
      applyFilter: () => source.applyFilter(),
      loadDirectory: (path: string, opts?: { preserveUI?: boolean; focusPath?: string }) => source.loadDirectory(path, opts),
      clearArchiveState: () => source.clearArchiveState(),
      focusTreeNode: (path: string) => source.focusTreeNode(path),
    };
  }

  createDirectoryHost(source: FilePaneActionHostsSource): FilePaneDirectoryHost {
    return {
      get paneId() { return source.paneId; },
      get currentPath() { return source.currentPath; },
      set currentPath(value: string) { source.currentPath = value; },
      get fsParentDir() { return source.fsParentDir; },
      set fsParentDir(value: string) { source.fsParentDir = value; },
      get currentTabId() { return source.currentTabId; },
      get showHidden() { return source.showHidden; },
      get entries() { return source.entries; },
      set entries(value: FileInfo[]) { source.entries = value; },
      get filteredEntries() { return source.filteredEntries; },
      get selectedFiles() { return source.selectedFiles; },
      set selectedFiles(value: Set<string>) { source.selectedFiles = value; },
      get selectionCursorPath() { return source.selectionCursorPath; },
      set selectionCursorPath(value: string | null) { source.selectionCursorPath = value; },
      get selectionAnchorPath() { return source.selectionAnchorPath; },
      set selectionAnchorPath(value: string | null) { source.selectionAnchorPath = value; },
      get loading() { return source.loading; },
      set loading(value: boolean) { source.loading = value; },
      get error() { return source.error; },
      set error(value: string | null) { source.error = value; },
      workspaceService: source.workspaceService,
      pathHistoryService: source.pathHistoryService,
      directorySource: source.directorySource,
      dirStream: source.dirStream,
      isInArchive: () => source.isInArchive(),
      exitArchive: () => source.exitArchive(),
      clearArchiveState: () => source.clearArchiveState(),
      withParentEntry: (raw: FileInfo[], inArchive: boolean, fsPath: string) => source.withParentEntry(raw, inArchive, fsPath),
      applyFilter: () => source.applyFilter(),
      setShowHiddenLoaded: (showHidden: boolean) => source.setShowHiddenLoaded(showHidden),
      emitPathChange: (path: string) => source.emitPathChange(path),
      runInAngular: (update: () => void) => source.runInAngular(update),
      onRefresh: () => source.onRefresh(),
      hasSavedDirTabState: () => source.hasSavedDirTabState(),
      restoreActiveDirTabState: () => source.restoreActiveDirTabState(),
      persistDirTabState: () => source.persistDirTabState(),
      scrollToCursorPath: (path: string) => source.scrollToCursorPath(path),
      focusTreeNode: (path: string) => source.focusTreeNode(path),
    };
  }

  createOperationsHost(source: FilePaneActionHostsSource): FilePaneOperationsHost {
    return {
      get paneId() { return source.paneId; },
      get currentPath() { return source.currentPath; },
      get currentView() { return source.currentView; },
      get entries() { return source.entries; },
      get selectedFiles() { return source.selectedFiles; },
      apiService: source.apiService,
      confirmDialog: source.confirmDialog,
      isInArchive: () => source.isInArchive(),
      deleteSelectedFromArchive: (effective: Set<string>) => source.deleteSelectedFromArchive(effective),
      getEffectiveSelection: () => source.getEffectiveSelection(),
      loadDirectory: (path: string) => source.loadDirectory(path),
      emitCopyToOther: (paths: string[]) => source.emitCopyToOther(paths),
      emitMoveToOther: (paths: string[]) => source.emitMoveToOther(paths),
      startRenameSelected: () => source.startRenameSelected(),
      getVisibleTreeNodes: () => source.getVisibleTreeNodes(),
    };
  }

  createPreviewHost(source: FilePaneActionHostsSource): FilePanePreviewHost {
    return {
      get paneId() { return source.paneId; },
      get activeTabKind() { return source.activeTabKind; },
      get entries() { return source.entries; },
      get archiveStack() { return source.archiveStack; },
      get tabs() { return source.tabs; },
      set tabs(value: any[]) { source.tabs = value as TabInfo[]; },
      apiService: source.apiService,
      workspaceService: source.workspaceService,
      confirmDialog: source.confirmDialog,
      isInArchive: () => source.isInArchive(),
      getEffectiveSelection: () => source.getEffectiveSelection(),
      onTabChange: (tab: any) => source.onTabChange(tab as TabInfo),
    };
  }

  createOpenHost(source: FilePaneActionHostsSource): FilePaneOpenHost {
    return {
      get currentPath() { return source.currentPath; },
      set currentPath(value: string) { source.currentPath = value; },
      get currentView() { return source.currentView; },
      get entries() { return source.entries; },
      apiService: source.apiService,
      isInArchive: () => source.isInArchive(),
      exitArchive: () => source.exitArchive(),
      clearArchiveState: () => source.clearArchiveState(),
      archiveNavigateInto: (path: string) => source.archiveNavigateInto(path),
      openNestedArchive: (chain: string[]) => source.openNestedArchive(chain),
      loadDirectory: (path: string) => source.loadDirectory(path),
      openRootArchive: (path: string) => source.openRootArchive(path),
      emitPathChange: (path: string) => source.emitPathChange(path),
      getEffectiveSelection: () => source.getEffectiveSelection(),
      openPathInNewTab: (path: string) => source.openPathInNewTab(path),
    };
  }
}