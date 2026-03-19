import { Injectable, NgZone } from '@angular/core';
import { createBehaviorSubject, type Stream } from '@epikodelabs/streamix';
import {
  WorkspaceConfig,
  WorkspaceList,
  PaneState,
  TabInfo,
  STORAGE_KEYS,
} from '../shared/protocol-enhanced';
import { ApiService } from './api.service';
import { ArchiveFrame } from './archive.service';
import {
  GetWorkspaces,
  SaveWorkspace,
  DeleteWorkspace,
} from 'wailsjs/go/app/App';

/**
 * Archive tab state - stores the complete archive navigation state
 */
export interface ArchiveTabState {
  path: string;
  archiveStack: ArchiveFrame[];
  fsDirPath: string;
  fsParentDir: string;
}

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
  private workspacesSubject = createBehaviorSubject<WorkspaceList>(this.loadLocalWorkspaces());
  public workspaces$ = this.workspacesSubject;

  private activeWorkspaceSubject = createBehaviorSubject<WorkspaceConfig | null>(null);
  public activeWorkspace$ = this.activeWorkspaceSubject;

  /** True once the initial load from the Wails backend has completed. */
  private backendLoaded = false;

  /** Store archive states per pane per tab */
  private archiveStates = new Map<string, Map<string, ArchiveTabState>>();

  constructor(private api: ApiService, private ngZone: NgZone) {
    this.initializeWorkspaces();
  }

  private emitInZone(fn: () => void): void {
    this.ngZone.run(fn);
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

    // Repair/normalize loaded workspaces so panes are never empty.
    const homePath = await this.resolveHomePath();
    const normalizedWorkspaces: WorkspaceConfig[] = [];
    for (const ws of (workspaceList.workspaces ?? [])) {
      const { workspace, changed } = this.normalizeWorkspace(ws, homePath);
      normalizedWorkspaces.push(workspace);
      if (changed) {
        // Best-effort persist so future launches don't start with empty panes.
        void this.persistWorkspace(workspace);
      }
    }
    workspaceList.workspaces = normalizedWorkspaces;

    // Ensure an activeWorkspaceId is set
    if (!workspaceList.activeWorkspaceId && workspaceList.workspaces.length > 0) {
      workspaceList.activeWorkspaceId = workspaceList.workspaces[0].id;
    }

    // Ensure activeWorkspaceId points to an existing workspace.
    if (
      workspaceList.activeWorkspaceId &&
      !workspaceList.workspaces.some(w => w.id === workspaceList.activeWorkspaceId)
    ) {
      workspaceList.activeWorkspaceId = workspaceList.workspaces[0]?.id ?? '';
    }

    this.emitInZone(() => {
      this.workspacesSubject.next(workspaceList);
      this.saveLocalWorkspaces(workspaceList);

      const active = workspaceList.workspaces.find(
        (w) => w.id === workspaceList.activeWorkspaceId,
      );
      this.activeWorkspaceSubject.next(active ?? null);
    });
  }

  private normalizeWorkspace(
    workspace: WorkspaceConfig,
    homePath: string,
  ): { workspace: WorkspaceConfig; changed: boolean } {
    let changed = false;

    const left = this.normalizePane('left', workspace.leftPane, homePath);
    const right = this.normalizePane('right', workspace.rightPane, homePath);
    if (left.changed || right.changed) changed = true;

    const normalized: WorkspaceConfig = {
      ...workspace,
      leftPane: left.pane,
      rightPane: right.pane,
    };
    return { workspace: normalized, changed };
  }

  private normalizePane(
    paneId: 'left' | 'right',
    pane: PaneState | undefined,
    homePath: string,
  ): { pane: PaneState; changed: boolean } {
    if (!pane || !Array.isArray(pane.tabs) || pane.tabs.length === 0) {
      return { pane: this.createDefaultPane(paneId, homePath), changed: true };
    }

    let changed = false;
    let activeTabId = pane.activeTabId;
    if (!activeTabId || !pane.tabs.some(t => t.id === activeTabId)) {
      activeTabId = pane.tabs[0].id;
      changed = true;
    }

    let tabs: TabInfo[] = pane.tabs.map(t => {
      const shouldBeActive = t.id === activeTabId;
      if (t.isActive !== shouldBeActive) changed = true;
      const kind = (t.kind ?? 'dir');
      if (!t.kind) changed = true;
      // Migration: earlier iterations made the auto-follow Preview tab non-closable.
      // Preview should behave like other tabs, so force it back to closable.
      const isClosable = (t.isClosable === false && kind === 'viewer') ? true : t.isClosable;
      if (t.isClosable === false && kind === 'viewer') changed = true;
      return { ...t, kind, isClosable, isActive: shouldBeActive } as TabInfo;
    });

    // Do not force-create preview tabs here. Preview tabs are user-facing and closable;
    // if the user closes them, they should stay closed across launches.

    const normalized: PaneState = {
      ...pane,
      id: paneId,
      tabs,
      activeTabId,
    };
    if (pane.id !== paneId) changed = true;
    return { pane: normalized, changed };
  }

  // ---------------------------------------------------------------------------
  // Backend persistence
  // ---------------------------------------------------------------------------

  /**
   * Load all workspaces from the Wails Go backend.
   * Maps the backend WorkspaceList model to our local type.
   */
  private async loadBackendWorkspaces(): Promise<WorkspaceList> {
    const raw = await GetWorkspaces();
    return raw as WorkspaceList;
  }

  /** Persist a single workspace to the Wails backend. */
  private async persistWorkspace(workspace: WorkspaceConfig): Promise<void> {
    try {
      await SaveWorkspace(workspace as any);
    } catch (err) {
      console.warn('WorkspaceService: failed to persist workspace to backend', err);
    }
  }

  /** Delete a workspace from the Wails backend. */
  private async deleteBackendWorkspace(id: string): Promise<void> {
    try {
      await DeleteWorkspace(id);
    } catch (err) {
      console.warn('WorkspaceService: failed to delete workspace from backend', err);
    }
  }

  /** Mark a workspace as active in the Wails backend. */
  private async setActiveBackend(_id: string): Promise<void> {
    // SetActiveWorkspace is not exposed by the backend; state is derived from
    // the saved workspace list on next load.
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
      this.emitInZone(() => this.workspacesSubject.next(list));
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
    this.emitInZone(() => this.activeWorkspaceSubject.next(workspace));
    this.setActiveBackend(workspaceId);

    // Clear archive states when switching workspaces
    this.archiveStates.clear();

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
      this.emitInZone(() => this.activeWorkspaceSubject.next(workspace));
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
      this.emitInZone(() => this.activeWorkspaceSubject.next(newActive));
      this.setActiveBackend(newActive.id);
    }

    this.updateAndPersist(list);
    this.deleteBackendWorkspace(workspaceId);

    // Clear archive states for deleted workspace
    this.clearAllArchiveStatesForWorkspace(workspaceId);

    return true;
  }

  // ---------------------------------------------------------------------------
  // Archive State Management
  // ---------------------------------------------------------------------------

  /**
   * Save archive state for a specific tab
   */
  saveArchiveState(paneId: string, tabId: string, state: ArchiveTabState): void {
    const paneKey = `${this.activeWorkspaceSubject.value?.id}:${paneId}`;
    
    if (!this.archiveStates.has(paneKey)) {
      this.archiveStates.set(paneKey, new Map());
    }
    
    this.archiveStates.get(paneKey)!.set(tabId, {
      ...state,
      // Clone the archive stack to prevent reference issues
      archiveStack: state.archiveStack.map(frame => ({
        ...frame,
        listing: { ...frame.listing },
        nestedChain: [...frame.nestedChain]
      }))
    });
  }

  /**
   * Get archive state for a specific tab
   */
  getArchiveState(paneId: string, tabId: string): ArchiveTabState | null {
    const paneKey = `${this.activeWorkspaceSubject.value?.id}:${paneId}`;
    const state = this.archiveStates.get(paneKey)?.get(tabId);
    
    if (state) {
      // Return a deep clone to prevent mutation issues
      return {
        ...state,
        archiveStack: state.archiveStack.map(frame => ({
          ...frame,
          listing: { ...frame.listing },
          nestedChain: [...frame.nestedChain]
        }))
      };
    }
    
    return null;
  }

  /**
   * Clear archive state for a specific tab
   */
  clearArchiveState(paneId: string, tabId: string): void {
    const paneKey = `${this.activeWorkspaceSubject.value?.id}:${paneId}`;
    this.archiveStates.get(paneKey)?.delete(tabId);
  }

  /**
   * Clear all archive states for a specific pane
   */
  clearAllArchiveStatesForPane(paneId: string): void {
    const paneKey = `${this.activeWorkspaceSubject.value?.id}:${paneId}`;
    this.archiveStates.delete(paneKey);
  }

  /**
   * Clear all archive states for a specific workspace
   */
  clearAllArchiveStatesForWorkspace(workspaceId: string): void {
    for (const key of this.archiveStates.keys()) {
      if (key.startsWith(`${workspaceId}:`)) {
        this.archiveStates.delete(key);
      }
    }
  }

  /**
   * Check if a tab has archive state
   */
  hasArchiveState(paneId: string, tabId: string): boolean {
    const paneKey = `${this.activeWorkspaceSubject.value?.id}:${paneId}`;
    return this.archiveStates.get(paneKey)?.has(tabId) || false;
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
      kind: 'dir',
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

    // Clear archive state when closing tab
    this.clearArchiveState(paneId, tabId);

    // Keep at least one tab – replace it with the drive root
    if (pane.tabs.length === 1) {
      const closedTab = pane.tabs[tabIndex];
      const rootPath = this.getRootForPath(closedTab.path || '/');
      const newTab: TabInfo = {
        id: this.generateId(),
        title: this.getPathTitle(rootPath),
        path: rootPath,
        kind: 'dir',
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
   * Only called during real FS navigation — never while inside an archive.
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
      true,
    );
    return true;
  }

  /**
   * Update both the path and title of a tab explicitly, without any derivation.
   *
   * Used for archive navigation where path and title are virtual
   * (e.g. "C:/Desktop.zip/src") and must not be processed by getPathTitle.
   * Also used on archive exit to restore the real FS path + basename title.
   */
  updateTabPathAndTitle(paneId: string, tabId: string, path: string, title: string): boolean {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return false;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const tab = pane.tabs.find((t) => t.id === tabId);
    if (!tab) return false;

    tab.path = path;
    tab.title = title;

    this.updateWorkspace(
      workspace.id,
      { [paneId === 'left' ? 'leftPane' : 'rightPane']: pane },
      true,
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

  /**
   * Move/reorder a tab within a pane or across panes.
   *
   * - When moving across panes, the tab becomes active in the destination pane.
   * - The source pane will always keep at least one tab (replacing with a root tab if needed).
   * - Archive browsing state for the tab (if any) is migrated to the destination pane.
   */
  moveTab(
    fromPaneId: 'left' | 'right',
    toPaneId: 'left' | 'right',
    tabId: string,
    toIndex: number,
  ): boolean {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return false;

    const fromPane = fromPaneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const toPane = toPaneId === 'left' ? workspace.leftPane : workspace.rightPane;

    const fromIndex = fromPane.tabs.findIndex((t) => t.id === tabId);
    if (fromIndex === -1) return false;

    // No-op fast path
    if (fromPaneId === toPaneId) {
      const clamped = Math.max(0, Math.min(toIndex, fromPane.tabs.length));
      // Dropping "before itself" (or immediately after) should do nothing.
      if (clamped === fromIndex || clamped === fromIndex + 1) return true;
    }

    const [tab] = fromPane.tabs.splice(fromIndex, 1);
    const wasActive = tab.isActive;

    // If the tab had archive state, migrate it to the destination pane.
    if (fromPaneId !== toPaneId) {
      const archiveState = this.getArchiveState(fromPaneId, tabId);
      if (archiveState) {
        this.saveArchiveState(toPaneId, tabId, archiveState);
        this.clearArchiveState(fromPaneId, tabId);
      }
    }

    if (fromPaneId === toPaneId) {
      // Reorder within the same pane.
      const target = Math.max(0, Math.min(toIndex, fromPane.tabs.length));
      const adjusted = target > fromIndex ? target - 1 : target;
      fromPane.tabs.splice(adjusted, 0, tab);
      // Preserve active tab bookkeeping
      const active = fromPane.tabs.find((t) => t.isActive) ?? fromPane.tabs[0];
      if (active) fromPane.activeTabId = active.id;

      this.updateWorkspace(workspace.id, {
        [fromPaneId === 'left' ? 'leftPane' : 'rightPane']: fromPane,
      });
      return true;
    }

    // Destination insert
    const insertAt = Math.max(0, Math.min(toIndex, toPane.tabs.length));
    toPane.tabs.splice(insertAt, 0, tab);

    // Make moved tab active in the destination.
    toPane.tabs.forEach((t) => (t.isActive = false));
    tab.isActive = true;
    toPane.activeTabId = tab.id;

    // Source pane: ensure it still has an active tab and at least one tab.
    if (fromPane.tabs.length === 0) {
      const rootPath = this.getRootForPath(tab.path || '/');
      const replacement: TabInfo = {
        id: this.generateId(),
        title: this.getPathTitle(rootPath),
        path: rootPath,
        kind: 'dir',
        isActive: true,
        isPinned: false,
        createdAt: Date.now(),
      };
      fromPane.tabs.push(replacement);
      fromPane.activeTabId = replacement.id;
    } else if (wasActive) {
      // Activate a neighbor when the active tab moved away.
      fromPane.tabs.forEach((t) => (t.isActive = false));
      const newActiveIdx = Math.min(fromIndex, fromPane.tabs.length - 1);
      fromPane.tabs[newActiveIdx].isActive = true;
      fromPane.activeTabId = fromPane.tabs[newActiveIdx].id;
    } else {
      // Keep current activeTabId consistent.
      const active = fromPane.tabs.find((t) => t.isActive);
      if (active) fromPane.activeTabId = active.id;
      else {
        fromPane.tabs[0].isActive = true;
        fromPane.activeTabId = fromPane.tabs[0].id;
      }
    }

    this.updateWorkspace(workspace.id, {
      [fromPaneId === 'left' ? 'leftPane' : 'rightPane']: fromPane,
      [toPaneId === 'left' ? 'leftPane' : 'rightPane']: toPane,
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
    const previewId = this.generateId();
    return {
      id: paneId,
      tabs: [
        {
          id: tabId,
          title: this.getPathTitle(path),
          path,
          kind: 'dir',
          isActive: true,
          isPinned: false,
          createdAt: Date.now(),
        },
        {
          id: previewId,
          title: 'Preview',
          path: '',
          kind: 'viewer',
          viewer: { targetPath: '', autoFollow: true },
          isActive: false,
          isPinned: false,
          createdAt: Date.now(),
        },
      ],
      activeTabId: tabId,
    };
  }

  updateViewerTabTargetPath(
    paneId: string,
    tabId: string,
    targetPath: string,
    source?: { sourcePane: 'left' | 'right'; sourceTabId: string; sourceDir: string },
  ): boolean {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return false;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const tab = pane.tabs.find((t) => t.id === tabId);
    if (!tab) return false;
    if ((tab.kind ?? 'dir') !== 'viewer') return false;

    const next = targetPath || '';
    tab.path = next;
    tab.title = next || 'Preview';
    tab.viewer = {
      targetPath: next,
      viewerId: tab.viewer?.viewerId,
      autoFollow: tab.viewer?.autoFollow,
      sourcePane: source?.sourcePane ?? tab.viewer?.sourcePane,
      sourceTabId: source?.sourceTabId ?? tab.viewer?.sourceTabId,
      sourceDir: source?.sourceDir ?? tab.viewer?.sourceDir,
      // archiveContext intentionally omitted — auto-follow always points to FS files
    };
    this.updateWorkspace(
      workspace.id,
      { [paneId === 'left' ? 'leftPane' : 'rightPane']: pane },
      false,
    );
    return true;
  }

  addViewerTab(
    paneId: string,
    targetPath: string,
    opts?: {
      activate?: boolean;
      title?: string;
      viewerId?: string;
      autoFollow?: boolean;
      closable?: boolean;
      sourcePane?: 'left' | 'right';
      sourceTabId?: string;
      sourceDir?: string;
      archiveContext?: { fsPath: string; nestedChain: string[]; entryPath: string };
    },
  ): TabInfo | null {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return null;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const activate = opts?.activate !== false;

    const newTab: TabInfo = {
      id: this.generateId(),
      title: (opts?.title ?? (targetPath || 'Preview')),
      path: targetPath || '',
      kind: 'viewer',
      viewer: {
        targetPath: targetPath || '',
        viewerId: opts?.viewerId,
        autoFollow: !!opts?.autoFollow,
        sourcePane: opts?.sourcePane,
        sourceTabId: opts?.sourceTabId,
        sourceDir: opts?.sourceDir,
        archiveContext: opts?.archiveContext,
      },
      isClosable: opts?.closable === false ? false : true,
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

  addSizeManagerTab(
    paneId: string,
    analysisPath: string,
    opts?: { activate?: boolean; closable?: boolean },
  ): TabInfo | null {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return null;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const activate = opts?.activate !== false;

    const newTab: TabInfo = {
      id: this.generateId(),
      title: 'Storage Footprint',
      path: analysisPath,
      kind: 'size-manager',
      isClosable: opts?.closable === false ? false : true,
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
    if (normalized === 'registry://') return 'Registry';
    if (normalized.startsWith('registry://')) {
      const rel = normalized.slice('registry://'.length);
      return rel.split('/').filter(Boolean).pop() || 'Registry';
    }
    const aliasMatch = /^([a-z][a-z0-9+.-]*):\/\/(.*)?$/i.exec(normalized);
    if (aliasMatch) {
      const scheme = aliasMatch[1].toLowerCase();
      const rel = (aliasMatch[2] || '').replace(/^\/+/, '');
      return rel.split('/').filter(Boolean).pop() || scheme;
    }
    // Drive roots: show "C:" not "C:/"
    if (/^[a-zA-Z]:\/?$/.test(normalized)) return normalized.slice(0, 2);
    return normalized;
  }

  private normalizePath(path: string): string {
    if (!path) return '/';
    if (/^registry:\/\//i.test(path)) {
      const rest = path.replace(/\\/g, '/').slice('registry://'.length).replace(/^\/+/, '').replace(/\/+/g, '/');
      return rest ? `registry://${rest}` : 'registry://';
    }
    const aliasMatch = /^([a-z][a-z0-9+.-]*):\/\/(.*)?$/i.exec(path.replace(/\\/g, '/'));
    if (aliasMatch) {
      const scheme = aliasMatch[1].toLowerCase();
      const rest = (aliasMatch[2] || '').replace(/^\/+/, '').replace(/\/+/g, '/');
      return rest ? `${scheme}://${rest}` : `${scheme}://`;
    }
    let p = path.replace(/\\/g, '/').replace(/\/+/g, '/');
    if (p.length > 1 && p.endsWith('/')) p = p.slice(0, -1);
    return p || '/';
  }

  private getRootForPath(path: string): string {
    const normalized = this.normalizePath(path);
    if (normalized.startsWith('registry://')) return 'registry://';
    const aliasMatch = /^([a-z][a-z0-9+.-]*):\/\//i.exec(normalized);
    if (aliasMatch) return `${aliasMatch[1].toLowerCase()}://`;
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
