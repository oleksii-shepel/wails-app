import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';
import {
  WorkspaceConfig,
  WorkspaceList,
  PaneState,
  TabInfo,
  STORAGE_KEYS,
} from '../shared/protocol-enhanced';
import { ApiService } from './api.service';

/**
 * WorkspaceService – manages workspaces and per-pane tab state.
 *
 * Persistence strategy:
 *   1. Primary:   Wails backend (SaveWorkspace / GetWorkspaces / DeleteWorkspace / SetActiveWorkspace)
 *   2. Fallback:  localStorage (used when the Wails bridge is not yet available,
 *                 e.g. during Angular unit tests or early bootstrap)
 *
 * All public surface-area is identical to the previous implementation so that
 * components that inject this service require zero changes.
 */
@Injectable({
  providedIn: 'root',
})
export class WorkspaceService {
  private workspacesSubject = new BehaviorSubject<WorkspaceList>(this.loadLocalWorkspaces());
  public workspaces$ = this.workspacesSubject.asObservable();

  private activeWorkspaceSubject = new BehaviorSubject<WorkspaceConfig | null>(null);
  public activeWorkspace$ = this.activeWorkspaceSubject.asObservable();

  /** True once the initial load from the Wails backend has completed. */
  private backendLoaded = false;

  constructor(private api: ApiService) {
    this.initializeWorkspaces();
  }

  // ---------------------------------------------------------------------------
  // Initialisation
  // ---------------------------------------------------------------------------

  /**
   * Load workspaces from the Wails backend first; fall back to localStorage if
   * the backend is not yet reachable (first-run, test environment, etc.).
   */
  private async initializeWorkspaces(): Promise<void> {
    let workspaceList: WorkspaceList;

    try {
      workspaceList = await this.loadBackendWorkspaces();
      this.backendLoaded = true;
    } catch {
      // wait for backend instead of falling back
      await new Promise(r => setTimeout(r, 300));
      return this.initializeWorkspaces();
    }

    // Bootstrap: create a default workspace if none exist yet
    if (!workspaceList.workspaces || workspaceList.workspaces.length === 0) {
      const defaultWorkspace = await this.buildDefaultWorkspace();
      workspaceList = {
        workspaces: [defaultWorkspace],
        activeWorkspaceId: defaultWorkspace.id,
      };

      await this.persistWorkspace(defaultWorkspace);
      await this.setActiveBackend(defaultWorkspace.id);
    }

    // Ensure an activeWorkspaceId is set
    if (!workspaceList.activeWorkspaceId && workspaceList.workspaces.length > 0) {
      workspaceList.activeWorkspaceId = workspaceList.workspaces[0].id;
    }

    this.workspacesSubject.next(workspaceList);
    this.saveLocalWorkspaces(workspaceList);

    const active = workspaceList.workspaces.find(
      (w) => w.id === workspaceList.activeWorkspaceId,
    );
    this.activeWorkspaceSubject.next(active ?? null);
  }

  // ---------------------------------------------------------------------------
  // Backend persistence
  // ---------------------------------------------------------------------------

  /**
   * Load all workspaces from the Wails Go backend.
   * Maps the backend WorkspaceList model to our local type.
   */
  private async loadBackendWorkspaces(): Promise<WorkspaceList> {
    const bridge = (window as any)?.go?.main?.App;
    if (!bridge?.GetWorkspaces) throw new Error('Wails bridge not ready');
    const raw = await bridge.GetWorkspaces() as WorkspaceList;
    return raw;
  }

  /** Persist a single workspace to the Wails backend. */
  private async persistWorkspace(workspace: WorkspaceConfig): Promise<void> {
    try {
      const bridge = (window as any)?.go?.main?.App;
      if (bridge?.SaveWorkspace) {
        await bridge.SaveWorkspace(workspace);
      }
    } catch (err) {
      console.warn('WorkspaceService: failed to persist workspace to backend', err);
    }
  }

  /** Delete a workspace from the Wails backend. */
  private async deleteBackendWorkspace(id: string): Promise<void> {
    try {
      const bridge = (window as any)?.go?.main?.App;
      if (bridge?.DeleteWorkspace) {
        await bridge.DeleteWorkspace(id);
      }
    } catch (err) {
      console.warn('WorkspaceService: failed to delete workspace from backend', err);
    }
  }

  /** Mark a workspace as active in the Wails backend. */
  private async setActiveBackend(id: string): Promise<void> {
    try {
      const bridge = (window as any)?.go?.main?.App;
      if (bridge?.SetActiveWorkspace) {
        await bridge.SetActiveWorkspace(id);
      }
    } catch (err) {
      console.warn('WorkspaceService: failed to set active workspace on backend', err);
    }
  }

  // ---------------------------------------------------------------------------
  // localStorage fallback helpers
  // ---------------------------------------------------------------------------

  private loadLocalWorkspaces(): WorkspaceList {
    try {
      const stored = localStorage.getItem(STORAGE_KEYS.WORKSPACES);
      if (stored) return JSON.parse(stored);
    } catch {}
    return { workspaces: [], activeWorkspaceId: '' };
  }

  private saveLocalWorkspaces(list: WorkspaceList): void {
    try {
      localStorage.setItem(STORAGE_KEYS.WORKSPACES, JSON.stringify(list));
    } catch {}
  }

  // ---------------------------------------------------------------------------
  // Internal mutations (sync emit + async persist)
  // ---------------------------------------------------------------------------

  private updateAndPersist(
    list: WorkspaceList,
    changedWorkspace?: WorkspaceConfig,
    silent = false,
  ): void {
    this.saveLocalWorkspaces(list);

    if (!silent) {
      this.workspacesSubject.next(list);
    }

    if (changedWorkspace) {
      // Fire-and-forget backend persistence
      this.persistWorkspace(changedWorkspace);
    }
  }

  // ---------------------------------------------------------------------------
  // Public API
  // ---------------------------------------------------------------------------

  getWorkspaces(): WorkspaceList {
    return this.workspacesSubject.value;
  }

  getActiveWorkspace(): WorkspaceConfig | null {
    return this.activeWorkspaceSubject.value;
  }

  // ---------------------------------------------------------------------------
  // Workspace CRUD
  // ---------------------------------------------------------------------------

  async createWorkspace(name: string, icon?: string, color?: string): Promise<WorkspaceConfig> {
    const homePath = await this.resolveHomePath();
    const now = Date.now();

    const workspace: WorkspaceConfig = {
      id: this.generateId(),
      name,
      icon: icon ?? '📁',
      color: color ?? '#007acc',
      leftPane: this.createDefaultPane('left', homePath),
      rightPane: this.createDefaultPane('right', homePath),
      showHidden: false,
      createdAt: now,
      lastAccessedAt: now,
    };

    const list = this.workspacesSubject.value;
    list.workspaces.push(workspace);
    this.updateAndPersist(list, workspace);

    return workspace;
  }

  switchWorkspace(workspaceId: string): boolean {
    const list = this.workspacesSubject.value;
    const workspace = list.workspaces.find((w) => w.id === workspaceId);
    if (!workspace) return false;

    workspace.lastAccessedAt = Date.now();
    list.activeWorkspaceId = workspaceId;
    this.updateAndPersist(list, workspace);
    this.activeWorkspaceSubject.next(workspace);
    this.setActiveBackend(workspaceId);

    return true;
  }

  updateWorkspace(
    workspaceId: string,
    updates: Partial<WorkspaceConfig>,
    silent = false,
  ): boolean {
    const list = this.workspacesSubject.value;
    const workspace = list.workspaces.find((w) => w.id === workspaceId);
    if (!workspace) return false;

    Object.assign(workspace, updates);
    this.updateAndPersist(list, workspace, silent);

    if (!silent && workspace.id === this.activeWorkspaceSubject.value?.id) {
      this.activeWorkspaceSubject.next(workspace);
    }

    return true;
  }

  deleteWorkspace(workspaceId: string): boolean {
    const list = this.workspacesSubject.value;
    const index = list.workspaces.findIndex((w) => w.id === workspaceId);

    if (index === -1 || list.workspaces.length <= 1) return false;

    list.workspaces.splice(index, 1);

    if (list.activeWorkspaceId === workspaceId) {
      const newActive = list.workspaces[0];
      list.activeWorkspaceId = newActive.id;
      this.activeWorkspaceSubject.next(newActive);
      this.setActiveBackend(newActive.id);
    }

    this.updateAndPersist(list);
    this.deleteBackendWorkspace(workspaceId);

    return true;
  }

  // ---------------------------------------------------------------------------
  // Tab management
  // ---------------------------------------------------------------------------

  addTab(paneId: string, path: string, activate = true): TabInfo | null {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return null;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;

    const newTab: TabInfo = {
      id: this.generateId(),
      title: this.getPathTitle(path),
      path,
      isActive: activate,
      isPinned: false,
      createdAt: Date.now(),
    };

    if (activate) {
      pane.tabs.forEach((t) => (t.isActive = false));
      pane.activeTabId = newTab.id;
    }

    pane.tabs.push(newTab);
    this.updateWorkspace(workspace.id, {
      [paneId === 'left' ? 'leftPane' : 'rightPane']: pane,
    });

    return newTab;
  }

  closeTab(paneId: string, tabId: string): boolean {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return false;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const tabIndex = pane.tabs.findIndex((t) => t.id === tabId);
    if (tabIndex === -1) return false;

    // Keep at least one tab – replace it with the drive root
    if (pane.tabs.length === 1) {
      const closedTab = pane.tabs[tabIndex];
      const rootPath = this.getRootForPath(closedTab.path || '/');
      const newTab: TabInfo = {
        id: this.generateId(),
        title: this.getPathTitle(rootPath),
        path: rootPath,
        isActive: true,
        isPinned: false,
        createdAt: Date.now(),
      };
      pane.tabs[tabIndex] = newTab;
      pane.activeTabId = newTab.id;
      this.updateWorkspace(workspace.id, {
        [paneId === 'left' ? 'leftPane' : 'rightPane']: pane,
      });
      return true;
    }

    const wasActive = pane.tabs[tabIndex].isActive;
    pane.tabs.splice(tabIndex, 1);

    if (wasActive && pane.tabs.length > 0) {
      const newActiveIdx = Math.min(tabIndex, pane.tabs.length - 1);
      pane.tabs[newActiveIdx].isActive = true;
      pane.activeTabId = pane.tabs[newActiveIdx].id;
    }

    this.updateWorkspace(workspace.id, {
      [paneId === 'left' ? 'leftPane' : 'rightPane']: pane,
    });
    return true;
  }

  switchTab(paneId: string, tabId: string): boolean {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return false;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const tab = pane.tabs.find((t) => t.id === tabId);
    if (!tab) return false;

    pane.tabs.forEach((t) => (t.isActive = false));
    tab.isActive = true;
    pane.activeTabId = tabId;

    this.updateWorkspace(workspace.id, {
      [paneId === 'left' ? 'leftPane' : 'rightPane']: pane,
    });
    return true;
  }

  /**
   * Update the filesystem path of a tab and derive its title from the basename.
   * Should NOT be called while inside an archive — use updateTabTitle instead.
   */
  updateTabPath(paneId: string, tabId: string, path: string): boolean {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return false;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const tab = pane.tabs.find((t) => t.id === tabId);
    if (!tab) return false;

    tab.path = path;
    tab.title = this.getPathTitle(path);

    this.updateWorkspace(
      workspace.id,
      { [paneId === 'left' ? 'leftPane' : 'rightPane']: pane },
      true, // silent — avoids reload loop
    );
    return true;
  }

  /**
   * Update only the display title of a tab without touching its stored path.
   *
   * Used by FilePaneComponent when navigating inside an archive so the tab
   * shows e.g. "project.zip > src" while tab.path still holds the real FS
   * path (needed to restore navigation on tab switch / workspace reload).
   */
  updateTabTitle(paneId: string, tabId: string, title: string): boolean {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return false;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const tab = pane.tabs.find((t) => t.id === tabId);
    if (!tab) return false;

    tab.title = title;

    this.updateWorkspace(
      workspace.id,
      { [paneId === 'left' ? 'leftPane' : 'rightPane']: pane },
      true, // silent — title change doesn't need to re-trigger workspace reload
    );
    return true;
  }

  toggleTabPin(paneId: string, tabId: string): boolean {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return false;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const tab = pane.tabs.find((t) => t.id === tabId);
    if (!tab) return false;

    tab.isPinned = !tab.isPinned;
    this.updateWorkspace(workspace.id, {
      [paneId === 'left' ? 'leftPane' : 'rightPane']: pane,
    });
    return true;
  }

  getActiveTab(paneId: string): TabInfo | null {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return null;
    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    return pane.tabs.find((t) => t.id === pane.activeTabId) ?? null;
  }

  getTabs(paneId: string): TabInfo[] {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return [];
    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    return pane.tabs;
  }

  // ---------------------------------------------------------------------------
  // Helpers
  // ---------------------------------------------------------------------------

  private async buildDefaultWorkspace(): Promise<WorkspaceConfig> {
    const now = Date.now();
    const homePath = await this.resolveHomePath();
    return {
      id: this.generateId(),
      name: 'Default Workspace',
      icon: '🏠',
      color: '#007acc',
      leftPane: this.createDefaultPane('left', homePath),
      rightPane: this.createDefaultPane('right', homePath),
      showHidden: false,
      createdAt: now,
      lastAccessedAt: now,
    };
  }

  private createDefaultPane(paneId: string, path: string): PaneState {
    const tabId = this.generateId();
    return {
      id: paneId,
      tabs: [
        {
          id: tabId,
          title: this.getPathTitle(path),
          path,
          isActive: true,
          isPinned: false,
          createdAt: Date.now(),
        },
      ],
      activeTabId: tabId,
    };
  }

  /**
   * Resolve the home path using the Wails OS info, falling back to '/'.
   */
  private async resolveHomePath(): Promise<string> {
    try {
      const info = await this.api.getOSInfo();
      if (info?.systemDrive) return info.systemDrive;
    } catch {}
    return '/';
  }

  private generateId(): string {
    return `${Date.now()}-${Math.random().toString(36).substring(2, 11)}`;
  }

  /**
   * Derive a short display title from a filesystem path.
   *
   * Rules:
   *   /                     → /
   *   C:/                   → C:
   *   /home/user/documents  → documents
   *   C:/Users/Alice/docs   → docs
   */
  private getPathTitle(path: string): string {
    const normalized = this.normalizePath(path);

    // Unix / Windows drive roots
    if (normalized === '/') return '/';
    if (/^[a-zA-Z]:\/?$/.test(normalized)) return normalized.slice(0, 2);

    // Return only the last path segment (basename)
    const parts = normalized.replace(/\\/g, '/').split('/').filter(Boolean);
    return parts[parts.length - 1] ?? normalized;
  }

  private normalizePath(path: string): string {
    if (!path) return '/';
    let p = path.replace(/\\/g, '/').replace(/\/+/g, '/');
    if (p.length > 1 && p.endsWith('/')) p = p.slice(0, -1);
    return p || '/';
  }

  private getRootForPath(path: string): string {
    const normalized = this.normalizePath(path);
    const drive = normalized.match(/^([a-zA-Z]):/);
    return drive ? `${drive[1]}:/` : '/';
  }

  getAllTabPaths(): string[] {
    const state = this.workspacesSubject.value;
    const paths: string[] = [];
    for (const ws of state.workspaces) {
      for (const pane of [ws.leftPane, ws.rightPane]) {
        if (pane && Array.isArray(pane.tabs)) {
          for (const tab of pane.tabs) {
            if (tab.path) paths.push(tab.path);
          }
        }
      }
    }
    return paths;
  }
}