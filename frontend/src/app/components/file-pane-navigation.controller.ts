import type { ApiService } from '../services/api.service';
import type { ArchiveFrame } from '../services/archive.service';
import type { FileViewDirectorySource } from '../services/directory-source.service';
import type { WorkspaceService } from '../services/workspace.service';
import type { DirSurfaceSession } from '../surfaces/dir/dir-surface-session';
import type { TabInfo, WorkspaceConfig } from '../shared/protocol-enhanced';

declare const window: any;

export interface FilePaneNavigationHost {
  paneId: 'left' | 'right';
  workspace: WorkspaceConfig | null;
  showHidden: boolean;
  currentTabId: string;
  currentPath: string;
  pathReady: boolean;
  tabs: TabInfo[];
  activeDirTabId: string | null;
  archiveStack: ArchiveFrame[];
  apiService: ApiService;
  directorySource: FileViewDirectorySource;
  workspaceService: WorkspaceService;
  getDirSession(): DirSurfaceSession;
  isInArchive(): boolean;
  exitArchive(): void;
  clearArchiveState(): void;
  loadPathTransparently(path: string): Promise<void>;
  loadDirectory(path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void>;
  navigateArchiveUp(): Promise<void>;
  refreshCurrentPathTransparently(): Promise<void>;
  refreshTreeNode(path: string): void;
  renderArchiveFrame(frame: ArchiveFrame): void;
  applyFilter(): void;
  emitPreviewTargetFromState(): void;
  emitActiveTabChange(tab: TabInfo): void;
}

export class FilePaneNavigationController {
  async loadPaneState(host: FilePaneNavigationHost): Promise<void> {
    if (!host.workspace) return;

    host.tabs = host.workspaceService.getTabs(host.paneId);
    const activeTab = host.workspaceService.getActiveTab(host.paneId);
    const savedPath = activeTab?.path ?? '';
    const activeKind = activeTab?.kind ?? 'dir';

    const pathToLoad = await this.resolveInitialPath(host, savedPath);
    host.pathReady = true;
    if (activeTab) host.currentTabId = activeTab.id;

    if (activeKind === 'viewer') {
      if (!host.activeDirTabId) {
        const firstDir = host.tabs.find(tab => (tab.kind ?? 'dir') === 'dir');
        if (firstDir) host.activeDirTabId = firstDir.id;
      }
      if (!host.currentPath) host.currentPath = pathToLoad;
      return;
    }

    if (activeTab) host.activeDirTabId = activeTab.id;
    host.currentPath = pathToLoad;
    host.clearArchiveState();

    const session = host.getDirSession();
    const cachedOk =
      !host.directorySource.isPathInsideArchive(pathToLoad) &&
      session.currentPath === pathToLoad &&
      session.showHiddenLoaded === host.showHidden &&
      (session.entries?.length ?? 0) > 0;

    if (cachedOk) {
      host.applyFilter();
      host.emitPreviewTargetFromState();
      return;
    }

    await host.loadPathTransparently(pathToLoad);
  }

  async onTabChange(host: FilePaneNavigationHost, tab: TabInfo): Promise<void> {
    const current = host.tabs.find(existing => existing.id === host.currentTabId);
    const currentKind = current?.kind ?? 'dir';

    if (currentKind === 'dir' && host.currentTabId && host.currentPath) {
      if (host.isInArchive()) {
        host.workspaceService.saveArchiveState(host.paneId, host.currentTabId, {
          path: host.currentPath,
          archiveStack: host.archiveStack,
          fsDirPath: host.archiveStack[0]?.fsPath ?? '',
          fsParentDir: host.getDirSession().fsParentDir,
        });
      } else {
        host.workspaceService.updateTabPath(host.paneId, host.currentTabId, host.currentPath);
      }
    }

    host.workspaceService.switchTab(host.paneId, tab.id);
    host.currentTabId = tab.id;
    host.clearArchiveState();

    if ((tab.kind ?? 'dir') === 'viewer') {
      host.tabs = host.workspaceService.getTabs(host.paneId);
      host.emitActiveTabChange(tab);
      return;
    }

    host.activeDirTabId = tab.id;

    const archiveState = host.workspaceService.getArchiveState(host.paneId, tab.id);
    if (archiveState) {
      host.currentPath = archiveState.path;
      host.archiveStack = archiveState.archiveStack;
      host.getDirSession().fsParentDir = archiveState.fsParentDir;
      if (host.archiveStack.length > 0) {
        host.renderArchiveFrame(host.archiveStack[host.archiveStack.length - 1]);
      }
      return;
    }

    const restoredPath = tab.path;
    if (restoredPath) host.currentPath = restoredPath;

    const session = host.getDirSession();
    const cachedOk =
      !host.directorySource.isPathInsideArchive(restoredPath) &&
      session.currentPath === restoredPath &&
      session.showHiddenLoaded === host.showHidden &&
      (session.entries?.length ?? 0) > 0;

    if (cachedOk) {
      host.applyFilter();
      host.emitPreviewTargetFromState();
      host.emitActiveTabChange(tab);
      return;
    }

    await host.loadPathTransparently(restoredPath);
    host.emitActiveTabChange(tab);
  }

  onPathChange(host: FilePaneNavigationHost, path: string): void {
    if (!path || path === host.currentPath) return;

    host.currentPath = path;
    host.exitArchive();
    host.clearArchiveState();
    void host.loadPathTransparently(path);
  }

  async onNavigateUp(host: FilePaneNavigationHost): Promise<void> {
    if (host.isInArchive()) {
      await host.navigateArchiveUp();
      return;
    }

    const fromPath = host.currentPath;
    const target = host.directorySource.getParentPath(fromPath);
    if (host.directorySource.getPathKind(fromPath) === 'registry') {
      if (target && target !== fromPath) {
        await host.loadDirectory(target, { focusPath: fromPath });
      }
      return;
    }

    try {
      const listing = await window.go.app.App.NavGoUp(host.paneId);
      await host.loadDirectory(listing.path, { focusPath: fromPath });
    } catch {
      if (target !== fromPath) {
        await host.loadDirectory(target, { focusPath: fromPath });
      }
    }
  }

  onRefresh(host: FilePaneNavigationHost): void {
    void host.refreshCurrentPathTransparently();
    host.refreshTreeNode(host.currentPath);
  }

  private async resolveInitialPath(host: FilePaneNavigationHost, savedPath: string): Promise<string> {
    const allPaths = host.workspaceService.getAllTabPaths();
    const windowsRoot = allPaths.find(path => /^[A-Za-z]:[/\\]/.test(path));
    let platformRoot = windowsRoot
      ? windowsRoot.replace(/\\/g, '/').replace(/^([A-Za-z]:).*$/, '$1/')
      : '/';

    if (platformRoot === '/') {
      try {
        const osInfo = await host.apiService.getOSInfo();
        if (osInfo?.systemDrive) {
          const drive = (osInfo.systemDrive as string).replace(/\\/g, '/').toUpperCase();
          platformRoot = drive.endsWith('/') ? drive : drive + '/';
        }
      } catch {
        // Keep the generic root when OS info isn't available.
      }
    }

    const isWindows = platformRoot !== '/';
    const pathIsStale = !savedPath || (isWindows && savedPath === '/');
    return pathIsStale ? platformRoot : savedPath;
  }
}