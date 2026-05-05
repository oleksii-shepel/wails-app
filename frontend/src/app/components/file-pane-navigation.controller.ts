import type { ApiService } from '../services/api.service';
import type { ArchiveFrame } from '../services/archive.service';
import type { FileViewDirectorySource } from '../services/directory-source.service';
import type { WorkspaceService } from '../services/workspace.service';
import type { DirSurfaceSession } from '../surfaces/dir/dir-surface-session';
import type { TabInfo, WorkspaceConfig } from '../shared/protocol-enhanced';
import { normalizePath, samePath } from './address-bar.utils';

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
  promptForRemoteMount(protocol: string, alias: string): Promise<string | null>;
}

export class FilePaneNavigationController {

  async loadPaneState(host: FilePaneNavigationHost): Promise<void> {
    if (!host.workspace) return;

    host.tabs = host.workspaceService.getTabs(host.paneId);
    const activeTab = host.workspaceService.getActiveTab(host.paneId);
    const savedPath = activeTab?.path ?? '';
    const activeKind = activeTab?.kind ?? 'dir';

    const initialPath = await this.resolveInitialPath(host, savedPath);
    const pathToLoad = await this.resolveRemotePathForLoad(host, initialPath)
      ?? await this.resolveInitialPath(host, '');
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

    const archiveState = activeTab ? host.workspaceService.getArchiveState(host.paneId, activeTab.id) : null;
    if (archiveState) {
      host.currentPath = archiveState.path;
      host.archiveStack = archiveState.archiveStack;
      host.getDirSession().fsParentDir = archiveState.fsParentDir;
      if (host.archiveStack.length > 0) {
        host.renderArchiveFrame(host.archiveStack[host.archiveStack.length - 1]);
        return;
      }
    }

    const session = host.getDirSession();
    const cachedOk =
      !host.directorySource.isPathInsideArchive(pathToLoad) &&
      session.currentPath === pathToLoad &&
      session.showHiddenLoaded === host.showHidden &&
      !session.loading &&
      !session.error;

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

    host.currentTabId = tab.id;
    host.workspaceService.switchTab(host.paneId, tab.id);
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

    const restoredPath = await this.resolveRemotePathForLoad(host, tab.path);
    if (!restoredPath) {
      host.emitActiveTabChange(tab);
      return;
    }
    host.currentPath = restoredPath;

    const session = host.getDirSession();
    const cachedOk =
      !host.directorySource.isPathInsideArchive(restoredPath) &&
      samePath(session.currentPath, restoredPath) &&
      session.showHiddenLoaded === host.showHidden &&
      !session.loading &&
      !session.error;

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
    if (!path || samePath(path, host.currentPath)) return;

    const previousPath = host.currentPath;
    host.currentPath = path;
    host.exitArchive();
    host.clearArchiveState();
    void this.navigateToEnteredPath(host, path, previousPath);
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

  private async navigateToEnteredPath(host: FilePaneNavigationHost, path: string, previousPath?: string): Promise<void> {
    const resolvedRemotePath = await this.resolveRemotePathForLoad(host, path, { interactive: true });
    if (!resolvedRemotePath) {
      if (previousPath) {
        host.currentPath = previousPath;
      }
      return;
    }
    if (!samePath(resolvedRemotePath, path)) {
      host.currentPath = resolvedRemotePath;
      await host.loadPathTransparently(resolvedRemotePath);
      return;
    }

    if (host.directorySource.isPathInsideArchive(path)) {
      await host.loadPathTransparently(path);
      return;
    }

    try {
      const info = await host.apiService.getFileInfo(path);
      if (info.type !== 'DIRECTORY') {
        const parent = host.directorySource.getParentPath(path);
        if (parent && !samePath(parent, path)) {
          await host.loadDirectory(parent, { focusPath: path });
          return;
        }
      }
    } catch {
      // Fall back to directory-style navigation for synthetic roots and unresolved paths.
    }

    await host.loadPathTransparently(path);
  }

  private async resolveInitialPath(host: FilePaneNavigationHost, savedPath: string): Promise<string> {
    const allPaths = host.workspaceService.getAllTabPaths();
    const windowsRoot = allPaths
      .map(path => normalizePath(path))
      .find(path => /^filesystem:\/\/[a-z](?:\/.*)?$/i.test(path));
    let platformRoot = windowsRoot
      ? windowsRoot.replace(/^(filesystem:\/\/[a-z]).*$/i, '$1')
      : '/';

    if (platformRoot === '/') {
      try {
        const osInfo = await host.apiService.getOSInfo();
        if (osInfo?.systemDrive) {
          platformRoot = normalizePath(osInfo.systemDrive as string);
        }
      } catch {
        // Keep the generic root when OS info isn't available.
      }
    }

    const isWindows = platformRoot !== '/';
    const pathIsStale = !savedPath || (isWindows && savedPath === '/');
    return pathIsStale ? platformRoot : savedPath;
  }

  private async resolveRemotePathForLoad(
    host: FilePaneNavigationHost,
    path: string,
    options?: { interactive?: boolean },
  ): Promise<string | null> {
    const normalized = normalizePath(path);
    const remoteMatch = /^(s3|gdrive|dropbox|onedrive):\/\/([^/]+)(?:\/(.*))?$/i.exec(normalized);
    if (!remoteMatch) {
      return path;
    }

    const protocol = remoteMatch[1].toLowerCase();
    const alias = remoteMatch[2];
    const aliasRoot = `${protocol}://${alias}`;

    try {
      const connections = await host.apiService.remoteListConnections();
      const mounted = connections.some((mount) => mount.protocol === protocol && samePath(mount.publicPath, aliasRoot));
      if (mounted) {
        return normalized;
      }
    } catch {
      return normalized;
    }

    if (!options?.interactive || samePath(normalized, `${protocol}://`)) {
      return normalized;
    }

    const mountedRoot = await host.promptForRemoteMount(protocol, alias);
    if (!mountedRoot) {
      return protocol === 's3' ? null : normalized;
    }

    const suffix = normalized.slice(aliasRoot.length).replace(/^\/+/, '');
    return suffix ? `${mountedRoot}/${suffix}` : mountedRoot;
  }
}
