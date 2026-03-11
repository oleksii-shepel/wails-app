import type { DirTabRestoreTarget, DirTabSessionState, DirTabStateController, DirTabUiRuntime } from '../surfaces/dir/dir-tab-state-controller';
import { DirSurfaceSession } from '../surfaces/dir/dir-surface-session';
import type { FileInfo } from '../shared/protocol';
import type { TabInfo } from '../shared/protocol-enhanced';
import type { ViewLayout } from './file-view-container.component';

export interface FilePaneDirSessionHost {
  get currentTabId(): string;
  get activeDirTabId(): string | null;
  set activeDirTabId(value: string | null);
  get tabs(): TabInfo[];
  dirTabStateController: DirTabStateController;
  getDirSession(tabId: string): DirSurfaceSession | undefined;
  setDirSession(session: DirSurfaceSession): void;
  isInArchive(): boolean;
  getSelectableEntriesPaths(): string[];
  applyFilter(): void;
  scrollToCursorPath(path: string): void;
  emitPreviewTargetFromState(): void;
}

export class FilePaneDirSessionController {
  getActiveSession(host: FilePaneDirSessionHost): DirSurfaceSession {
    const id = host.activeDirTabId ?? host.currentTabId;
    let session = id ? host.getDirSession(id) : undefined;
    if (!session) {
      session = this.createSession(host, id || 'default');
      host.setDirSession(session);
    }

    const activeTab = host.tabs.find(tab => tab.id === host.currentTabId);
    if ((activeTab?.kind ?? 'dir') === 'dir') {
      host.activeDirTabId = session.tabId;
    }

    return session;
  }

  getActiveRestoreTarget(host: FilePaneDirSessionHost): DirTabRestoreTarget {
    return this.createRestoreTarget(host, this.getActiveSession(host));
  }

  restoreActiveState(host: FilePaneDirSessionHost): void {
    const tabId = host.activeDirTabId ?? host.currentTabId;
    const activeTab = host.tabs.find(tab => tab.id === host.currentTabId);
    if (!tabId || (activeTab?.kind ?? 'dir') !== 'dir' || host.isInArchive()) {
      return;
    }

    const savedState = host.dirTabStateController.load(tabId);
    if (!savedState) {
      return;
    }

    host.dirTabStateController.finalizeAfterLoad(savedState, this.getActiveRestoreTarget(host));
  }

  private createSession(host: FilePaneDirSessionHost, tabId: string): DirSurfaceSession {
    const session = new DirSurfaceSession(tabId);
    session.currentView = this.currentViewPrefFromStorage();

    const savedSortField = localStorage.getItem('fileManager_sortField');
    const savedSortDirection = localStorage.getItem('fileManager_sortDirection') as ('asc' | 'desc' | null);
    if (savedSortField) session.sortField = savedSortField;
    if (savedSortDirection === 'asc' || savedSortDirection === 'desc') session.sortDirection = savedSortDirection;

    const savedDirsFirst = localStorage.getItem('fileManager_directoriesFirst');
    if (savedDirsFirst === 'true' || savedDirsFirst === 'false') {
      session.directoriesFirst = savedDirsFirst === 'true';
    }

    const savedState = host.dirTabStateController.load(session.tabId);
    if (savedState) {
      host.dirTabStateController.applyBeforeLoad(savedState, this.createSessionState(session));
    }

    return session;
  }

  private createRestoreTarget(host: FilePaneDirSessionHost, session: DirSurfaceSession): DirTabRestoreTarget {
    return {
      session: this.createSessionState(session),
      ui: this.createUiRuntime(host),
    };
  }

  private createSessionState(session: DirSurfaceSession): DirTabSessionState {
    return {
      getEntriesPaths: () => session.getEntriesPaths(),
      getOptimisticHiddenPaths: () => session.getOptimisticHiddenPaths(),
      setOptimisticHiddenPaths: (paths: string[]) => session.setOptimisticHiddenPaths(paths),
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
      set filterQuery(value) { session.filterQuery = value; },
      get selectedFiles() { return session.selectedFiles; },
      set selectedFiles(value: Set<string>) { session.selectedFiles = value; },
      get selectionCursorPath() { return session.selectionCursorPath; },
      set selectionCursorPath(value: string | null) { session.selectionCursorPath = value; },
      get selectionAnchorPath() { return session.selectionAnchorPath; },
      set selectionAnchorPath(value: string | null) { session.selectionAnchorPath = value; },
    };
  }

  private createUiRuntime(host: FilePaneDirSessionHost): DirTabUiRuntime {
    return {
      isInArchive: () => host.isInArchive(),
      getSelectableEntriesPaths: () => host.getSelectableEntriesPaths(),
      applyFilter: () => host.applyFilter(),
      scrollToCursorPath: (path: string) => host.scrollToCursorPath(path),
      emitPreviewTargetFromState: () => host.emitPreviewTargetFromState(),
    };
  }

  private currentViewPrefFromStorage(): ViewLayout {
    const saved = localStorage.getItem('fileManager_viewMode') as ViewLayout | null;
    return saved === 'list' || saved === 'grid' || saved === 'details' || saved === 'tree' ? saved : 'list';
  }
}