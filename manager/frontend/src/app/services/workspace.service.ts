import { Injectable, NgZone, signal } from '@angular/core';
import { createBehaviorSubject, type Stream } from '@epikodelabs/streamix';
import {
  WorkspaceConfig,
  WorkspaceList,
  PaneState,
  TabInfo,
  ViewerFileListItem,
  STORAGE_KEYS,
} from '../shared/protocol-enhanced';
import { ApiService } from './api.service';
import { WorkspaceSurfaceService } from './workspace-surface.service';
import type { WorkspacePresentation } from './workspace-surface.model';
import { ArchiveFrame } from './archive.service';
import { normalizePath as normalizeReferencePath } from '../components/address-bar.utils';
import { describeLocation, locationLabel } from './location-reference';
import { legacyKindSurfaceId, tabLocation } from '../surfaces/tab-surface';
import {
  GetPreferences,
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
  exitViewState?: {
    path: string;
    scrollTop: number;
    selectedPaths: string[];
    cursorPath: string | null;
    anchorPath: string | null;
  };
}

interface PersistedTabViewStateEnvelope {
  version?: number;
  dir?: unknown;
  archive?: ArchiveTabState;
  browserPaneSplit?: number;
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
export interface RecentlyClosedTab {
  readonly id: string;
  readonly workspaceId: string;
  readonly paneId: 'left' | 'right';
  readonly closedAt: number;
  readonly tabIndex?: number;
  readonly tab: TabInfo;
}

interface RecentlyClosedTabEnvelope {
  readonly version: 1;
  readonly entries: RecentlyClosedTab[];
}

const RECENTLY_CLOSED_TABS_KEY = 'reassembly.workspace.closedTabs.v1';
const RECENTLY_CLOSED_TABS_LIMIT = 40;

@Injectable({
  providedIn: 'root',
})
export class WorkspaceService {
  private static readonly SETTINGS_STORAGE_KEY = 'project-settings-v1';
  private workspacesSubject = createBehaviorSubject<WorkspaceList>(this.loadLocalWorkspaces());
  public workspaces$ = this.workspacesSubject;

  private activeWorkspaceSubject = createBehaviorSubject<WorkspaceConfig | null>(null);
  public activeWorkspace$ = this.activeWorkspaceSubject;

  /** True once the initial load from the Wails backend has completed. */
  private backendLoaded = false;

  /** Store archive states per pane per tab */
  private archiveStates = new Map<string, Map<string, ArchiveTabState>>();
  private recentlyClosedTabs = this.loadRecentlyClosedTabs();
  private readonly closedTabsRevision = signal(0);

  constructor(
    private api: ApiService,
    private ngZone: NgZone,
    private workspaceSurfaces: WorkspaceSurfaceService,
  ) {
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
    const reopenLastWorkspace = await this.loadReopenLastWorkspacePreference();

    try {
      workspaceList = await this.loadBackendWorkspaces();
      this.backendLoaded = true;
    } catch {
      // Crash/restart resilience: the local durable snapshot is authoritative
      // enough to restore tabs/navigation while the backend bridge recovers.
      workspaceList = this.loadLocalWorkspaces();
      this.backendLoaded = false;
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

    if (!reopenLastWorkspace && workspaceList.workspaces.length > 0) {
      workspaceList.activeWorkspaceId = workspaceList.workspaces[0].id;
    }

    this.emitInZone(() => {
      this.workspacesSubject.next(workspaceList);
      this.saveLocalWorkspaces(workspaceList);

      const active = workspaceList.workspaces.find(
        (w) => w.id === workspaceList.activeWorkspaceId,
      );
      this.activeWorkspaceSubject.next(active ?? null);
      if (active) {
        this.workspaceSurfaces.activateFilesWorkspace(active);
      }
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
    const normalizedTabs = this.normalizePaneTabs(pane.tabs, pane.activeTabId, homePath);
    if (normalizedTabs.changed) changed = true;

    // Do not force-create preview tabs here. Preview tabs are user-facing and closable;
    // if the user closes them, they should stay closed across launches.

    const normalized: PaneState = {
      ...pane,
      id: paneId,
      tabs: normalizedTabs.tabs,
      activeTabId: normalizedTabs.activeTabId,
    };
    if (pane.id !== paneId) changed = true;
    return { pane: normalized, changed };
  }

  private normalizePaneTabs(
    tabs: TabInfo[],
    activeTabId: string | undefined,
    homePath: string,
  ): { tabs: TabInfo[]; activeTabId: string; changed: boolean } {
    let changed = false;
    const seenIds = new Set<string>();
    const deduped: TabInfo[] = [];

    for (const tab of tabs) {
      if (seenIds.has(tab.id)) {
        changed = true;
        continue;
      }

      seenIds.add(tab.id);

      const kind = tab.kind ?? 'dir';
      const surfaceId = tab.surfaceId?.trim() || legacyKindSurfaceId(kind);
      if (!tab.surfaceId) changed = true;
      if (!tab.kind) changed = true;

      const persistedPath = this.persistedNavigationTarget(tab, surfaceId);
      const canonicalPath = this.normalizePersistedTabPath(persistedPath, surfaceId, homePath);
      if (canonicalPath !== (tab.path ?? '')) changed = true;

      const location = surfaceId === 'viewer'
        ? tab.location
        : (describeLocation(canonicalPath) ?? undefined);
      if (surfaceId !== 'viewer') {
        if (!tab.location && location) changed = true;
        if (tab.location?.location !== location?.location) changed = true;
      }

      // Migration: earlier iterations made the auto-follow Preview tab non-closable.
      // Preview should behave like other tabs, so force it back to closable.
      const isClosable = (tab.isClosable === false && surfaceId === 'viewer') ? true : tab.isClosable;
      if (tab.isClosable === false && surfaceId === 'viewer') changed = true;

      deduped.push({
        ...tab,
        path: canonicalPath,
        kind,
        surfaceId,
        location,
        isClosable,
      } as TabInfo);
    }

    const nextActiveTabId = activeTabId && deduped.some(tab => tab.id === activeTabId)
      ? activeTabId
      : (deduped[0]?.id ?? '');
    if (nextActiveTabId !== (activeTabId ?? '')) changed = true;

    const normalizedTabs = deduped.map((tab) => {
      const shouldBeActive = tab.id === nextActiveTabId;
      if (tab.isActive !== shouldBeActive) changed = true;
      return { ...tab, isActive: shouldBeActive } as TabInfo;
    });

    return {
      tabs: normalizedTabs,
      activeTabId: nextActiveTabId,
      changed,
    };
  }

  private persistedNavigationTarget(tab: TabInfo, surfaceId: string): string | undefined {
    if (surfaceId === 'viewer') {
      return tab.path;
    }

    const location = tab.location?.location?.trim();
    if (location) {
      return location;
    }

    const path = tab.path?.trim();
    return path || undefined;
  }

  private sanitizePaneUpdate(
    paneId: 'left' | 'right',
    pane: PaneState,
  ): PaneState {
    if (!pane || !Array.isArray(pane.tabs) || pane.tabs.length === 0) {
      return { ...pane, id: paneId };
    }

    const fallbackHomePath = pane.tabs.find(tab => tab.path?.startsWith('filesystem://'))?.path || '/';
    const normalized = this.normalizePaneTabs(pane.tabs, pane.activeTabId, fallbackHomePath);
    return {
      ...pane,
      id: paneId,
      tabs: normalized.tabs,
      activeTabId: normalized.activeTabId,
    };
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

  private async loadReopenLastWorkspacePreference(): Promise<boolean> {
    try {
      const stored = localStorage.getItem(WorkspaceService.SETTINGS_STORAGE_KEY);
      if (stored) {
        const parsed = JSON.parse(stored) as { general?: { reopenLastWorkspace?: boolean } };
        if (typeof parsed?.general?.reopenLastWorkspace === 'boolean') {
          return parsed.general.reopenLastWorkspace;
        }
      }
    } catch {
      // Ignore storage errors and fall back to backend preferences.
    }

    try {
      const preferences = await GetPreferences() as { reopenLastWorkspace?: boolean };
      if (typeof preferences?.reopenLastWorkspace === 'boolean') {
        return preferences.reopenLastWorkspace;
      }
    } catch {
      // Ignore backend errors during early startup.
    }

    return true;
  }

  private saveLocalWorkspaces(list: WorkspaceList): void {
    try {
      localStorage.setItem(
        STORAGE_KEYS.WORKSPACES,
        JSON.stringify(this.durableWorkspaceSnapshot(list)),
      );
    } catch {}
  }

  private durableWorkspaceSnapshot(list: WorkspaceList): WorkspaceList {
    const clone = JSON.parse(JSON.stringify(list)) as WorkspaceList;

    for (const workspace of clone.workspaces ?? []) {
      for (const pane of [workspace.leftPane, workspace.rightPane]) {
        for (const tab of pane?.tabs ?? []) {
          // Viewer next/previous populations can contain tens of thousands of
          // entries. Persist the target and source location, not that runtime
          // population; it is regenerated from the source pane on restore.
          if (tab.viewer?.fileList) {
            delete tab.viewer.fileList;
          }

          // Defensive cleanup for old/experimental runtime state.
          const runtimeTab = tab as TabInfo & {
            entries?: unknown;
            visibleEntries?: unknown;
            population?: unknown;
          };
          delete runtimeTab.entries;
          delete runtimeTab.visibleEntries;
          delete runtimeTab.population;
        }
      }
    }
    return clone;
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
      void this.persistWorkspace(changedWorkspace).catch(() => undefined);
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

  getActiveWorkspacePresentation(): WorkspacePresentation | null {
    return this.workspaceSurfaces.active;
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
      color: color ?? 'var(--vsc-accent-blue)',
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
    this.workspaceSurfaces.activateFilesWorkspace(workspace);
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

    const normalizedUpdates: Partial<WorkspaceConfig> = { ...updates };
    if (updates.leftPane) {
      normalizedUpdates.leftPane = this.sanitizePaneUpdate('left', updates.leftPane);
    }
    if (updates.rightPane) {
      normalizedUpdates.rightPane = this.sanitizePaneUpdate('right', updates.rightPane);
    }

    Object.assign(workspace, normalizedUpdates);
    this.updateAndPersist(list, workspace, silent);

    if (!silent && workspace.id === this.activeWorkspaceSubject.value?.id) {
      this.emitInZone(() => this.activeWorkspaceSubject.next(workspace));
      this.workspaceSurfaces.activateFilesWorkspace(workspace);
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
      this.workspaceSurfaces.activateFilesWorkspace(newActive);
      this.setActiveBackend(newActive.id);
    }

    this.updateAndPersist(list);
    this.deleteBackendWorkspace(workspaceId);

    // Clear archive states for deleted workspace
    this.clearAllArchiveStatesForWorkspace(workspaceId);
    this.clearRecentlyClosedTabs(workspaceId);

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

    this.archiveStates.get(paneKey)!.set(tabId, this.cloneArchiveState(state));

    const workspace = this.activeWorkspaceSubject.value;
    const tab = this.findTab(paneId as 'left' | 'right', tabId);
    if (workspace && tab) {
      const envelope = this.parseTabViewStateEnvelope(tab.viewStateJson);
      envelope.archive = this.cloneArchiveState(state);
      const next = this.serializeTabViewStateEnvelope(envelope);
      if ((tab.viewStateJson ?? undefined) !== next) {
        tab.viewStateJson = next;
        this.scheduleViewStatePersist(workspace);
      }
    }
  }

  /**
   * Get archive state for a specific tab
   */
  getArchiveState(paneId: string, tabId: string): ArchiveTabState | null {
    const paneKey = `${this.activeWorkspaceSubject.value?.id}:${paneId}`;
    const state = this.archiveStates.get(paneKey)?.get(tabId);
    
    if (state) {
      return this.cloneArchiveState(state);
    }

    const tab = this.findTab(paneId as 'left' | 'right', tabId);
    if (tab?.viewStateJson) {
      const envelope = this.parseTabViewStateEnvelope(tab.viewStateJson);
      if (envelope.archive) {
        return this.cloneArchiveState(envelope.archive);
      }
    }

    return null;
  }

  /**
   * Clear archive state for a specific tab
   */
  clearArchiveState(paneId: string, tabId: string): void {
    const paneKey = `${this.activeWorkspaceSubject.value?.id}:${paneId}`;
    this.archiveStates.get(paneKey)?.delete(tabId);

    const workspace = this.activeWorkspaceSubject.value;
    const tab = this.findTab(paneId as 'left' | 'right', tabId);
    if (workspace && tab?.viewStateJson) {
      const envelope = this.parseTabViewStateEnvelope(tab.viewStateJson);
      if (envelope.archive) {
        delete envelope.archive;
        tab.viewStateJson = this.serializeTabViewStateEnvelope(envelope);
        this.scheduleViewStatePersist(workspace);
      }
    }
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
    if (this.archiveStates.get(paneKey)?.has(tabId)) {
      return true;
    }

    const tab = this.findTab(paneId as 'left' | 'right', tabId);
    if (!tab?.viewStateJson) {
      return false;
    }

    return !!this.parseTabViewStateEnvelope(tab.viewStateJson).archive;
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
      title: locationLabel(path) || this.getPathTitle(path),
      path,
      location: describeLocation(path) ?? undefined,
      kind: 'dir',
      surfaceId: 'dir',
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

    const closingTab = pane.tabs[tabIndex];
    this.recordClosedTab(
      workspace.id,
      paneId === 'right' ? 'right' : 'left',
      closingTab,
      tabIndex,
    );

    // Clear archive state when closing tab
    this.clearArchiveState(paneId, tabId);

    // Keep at least one tab – replace it with the drive root
    if (pane.tabs.length === 1) {
      const closedTab = pane.tabs[tabIndex];
      const rootPath = this.getRootForPath(tabLocation(closedTab) || '/');
      const newTab: TabInfo = {
        id: this.generateId(),
        title: locationLabel(rootPath) || this.getPathTitle(rootPath),
        path: rootPath,
        location: describeLocation(rootPath) ?? undefined,
        kind: 'dir',
      surfaceId: 'dir',
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

  getRecentlyClosedTabs(
    workspaceId = this.activeWorkspaceSubject.value?.id ?? '',
    limit = RECENTLY_CLOSED_TABS_LIMIT,
  ): RecentlyClosedTab[] {
    this.closedTabsRevision();
    return this.recentlyClosedTabs
      .filter(entry => entry.workspaceId === workspaceId)
      .slice(0, Math.max(0, limit))
      .map(entry => ({
        ...entry,
        tab: this.cloneTab(entry.tab),
      }));
  }

  canReopenClosedTab(
    workspaceId = this.activeWorkspaceSubject.value?.id ?? '',
  ): boolean {
    return this.recentlyClosedTabs.some(entry =>
      entry.workspaceId === workspaceId
    );
  }

  reopenClosedTabById(
    closedId: string,
  ): { paneId: 'left' | 'right'; tab: TabInfo } | null {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace || !closedId) return null;

    const index = this.recentlyClosedTabs.findIndex(entry =>
      entry.id === closedId &&
      entry.workspaceId === workspace.id
    );
    if (index < 0) return null;

    return this.restoreClosedTabAtIndex(workspace, index);
  }

  reopenLastClosedTab(
    preferredPaneId?: 'left' | 'right',
  ): { paneId: 'left' | 'right'; tab: TabInfo } | null {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return null;

    const preferredIndex = this.recentlyClosedTabs.findIndex(entry =>
      entry.workspaceId === workspace.id &&
      (!preferredPaneId || entry.paneId === preferredPaneId)
    );

    const index = preferredIndex >= 0
      ? preferredIndex
      : this.recentlyClosedTabs.findIndex(entry =>
          entry.workspaceId === workspace.id
        );

    if (index < 0) return null;
    return this.restoreClosedTabAtIndex(workspace, index);
  }

  clearRecentlyClosedTabs(
    workspaceId = this.activeWorkspaceSubject.value?.id ?? '',
  ): void {
    if (!workspaceId) return;
    this.recentlyClosedTabs = this.recentlyClosedTabs.filter(entry =>
      entry.workspaceId !== workspaceId,
    );
    this.persistRecentlyClosedTabs();
  }

  switchTab(paneId: string, tabId: string): boolean {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return false;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const tab = pane.tabs.find((t) => t.id === tabId);
    if (!tab) return false;

    const updatedPane: PaneState = {
      ...pane,
      tabs: pane.tabs.map((currentTab) => ({
        ...currentTab,
        isActive: currentTab.id === tabId,
      })),
      activeTabId: tabId,
    };

    this.updateWorkspace(workspace.id, {
      [paneId === 'left' ? 'leftPane' : 'rightPane']: updatedPane,
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
    tab.location = describeLocation(path) ?? undefined;
    tab.title = locationLabel(path) || this.getPathTitle(path);

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
    * (e.g. "filesystem://c/Desktop.zip/src") and must not be processed by getPathTitle.
   * Also used on archive exit to restore the real FS path + basename title.
   */
  updateTabPathAndTitle(paneId: string, tabId: string, path: string, title: string): boolean {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return false;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const tab = pane.tabs.find((t) => t.id === tabId);
    if (!tab) return false;

    tab.path = path;
    tab.location = describeLocation(path) ?? undefined;
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
      const rootPath = this.getRootForPath(tabLocation(tab) || '/');
      const replacement: TabInfo = {
        id: this.generateId(),
        title: locationLabel(rootPath) || this.getPathTitle(rootPath),
        path: rootPath,
        location: describeLocation(rootPath) ?? undefined,
        kind: 'dir',
      surfaceId: 'dir',
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

  /**
   * Persist the per-tab view state slice (JSON-encoded). Debounced per workspace
   * so rapid selection/sort changes don't flood the backend.
   */
  setTabViewState(paneId: 'left' | 'right', tabId: string, viewStateJson: string | null): void {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return;

    const tab = this.findTab(paneId, tabId);
    if (!tab) return;

    const envelope = this.parseTabViewStateEnvelope(tab.viewStateJson);
    if (!viewStateJson) {
      delete envelope.dir;
    } else {
      try {
        envelope.dir = JSON.parse(viewStateJson);
      } catch {
        return;
      }
    }

    const next = this.serializeTabViewStateEnvelope(envelope);
    if (tab.viewStateJson === next) return;
    tab.viewStateJson = next;

    this.scheduleViewStatePersist(workspace);
  }

  /** Persist which pane currently holds focus on the active workspace. */
  setActivePaneId(paneId: 'left' | 'right'): void {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return;
    if (workspace.activePaneId === paneId) return;
    workspace.activePaneId = paneId;
    this.scheduleViewStatePersist(workspace);
  }

  /** Persist the left-pane width (0-100) on the active workspace. */
  setSplitRatio(leftPercent: number): void {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return;
    const clamped = Math.max(0, Math.min(100, leftPercent));
    if (workspace.splitRatio === clamped) return;
    workspace.splitRatio = clamped;
    this.scheduleViewStatePersist(workspace);
  }

  private viewStatePersistTimer: ReturnType<typeof setTimeout> | null = null;
  private viewStatePersistTarget: WorkspaceConfig | null = null;

  private scheduleViewStatePersist(workspace: WorkspaceConfig): void {
    this.viewStatePersistTarget = workspace;
    this.saveLocalWorkspaces(this.workspacesSubject.value);
    if (this.viewStatePersistTimer) return;
    this.viewStatePersistTimer = setTimeout(() => {
      const target = this.viewStatePersistTarget;
      this.viewStatePersistTimer = null;
      this.viewStatePersistTarget = null;
      if (target) this.persistWorkspace(target);
    }, 250);
  }

  getTabs(paneId: string): TabInfo[] {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return [];
    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    return pane.tabs;
  }

  getTabBrowserPaneSplit(paneId: 'left' | 'right', tabId: string): number | null {
    const tab = this.findTab(paneId, tabId);
    if (!tab?.viewStateJson) return null;

    const envelope = this.parseTabViewStateEnvelope(tab.viewStateJson);
    return typeof envelope.browserPaneSplit === 'number' && Number.isFinite(envelope.browserPaneSplit)
      ? envelope.browserPaneSplit
      : null;
  }

  setTabBrowserPaneSplit(paneId: 'left' | 'right', tabId: string, split: number | null): void {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace || !tabId) return;

    const tab = this.findTab(paneId, tabId);
    if (!tab) return;

    const envelope = this.parseTabViewStateEnvelope(tab.viewStateJson);
    if (split === null || !Number.isFinite(split)) {
      delete envelope.browserPaneSplit;
    } else {
      envelope.browserPaneSplit = split;
    }

    const next = this.serializeTabViewStateEnvelope(envelope);
    if ((tab.viewStateJson ?? undefined) === next) return;
    tab.viewStateJson = next;
    this.scheduleViewStatePersist(workspace);
  }

  private restoreClosedTabAtIndex(
    workspace: WorkspaceConfig,
    index: number,
  ): { paneId: 'left' | 'right'; tab: TabInfo } | null {
    if (index < 0 || index >= this.recentlyClosedTabs.length) return null;

    const entry = this.recentlyClosedTabs[index];

    const pane =
      entry.paneId === 'right'
        ? workspace.rightPane
        : workspace.leftPane;

    const tabs = pane.tabs.map(tab => ({ ...tab, isActive: false }));

    const restored: TabInfo = {
      ...this.cloneTab(entry.tab),
      id: this.generateId(),
      isActive: true,
      createdAt: Date.now(),
    };

    const previousPosition = entry.tabIndex;
    const position = previousPosition !== undefined && Number.isSafeInteger(previousPosition) && previousPosition >= 0
      ? Math.min(previousPosition, tabs.length)
      : tabs.length;
    tabs.splice(position, 0, restored);

    const updated = this.updateWorkspace(workspace.id, {
      [entry.paneId === 'left' ? 'leftPane' : 'rightPane']: { ...pane, tabs, activeTabId: restored.id },
    });
    if (!updated) return null;
    this.recentlyClosedTabs = this.recentlyClosedTabs.filter(closed => closed.id !== entry.id);
    this.persistRecentlyClosedTabs();

    return {
      paneId: entry.paneId,
      tab: this.cloneTab(restored),
    };
  }

  private recordClosedTab(
    workspaceId: string,
    paneId: 'left' | 'right',
    tab: TabInfo,
    tabIndex: number,
  ): void {
    if (!workspaceId || !tab?.id) return;

    const record: RecentlyClosedTab = {
      id:
        globalThis.crypto?.randomUUID?.() ??
        `${Date.now()}-${Math.random().toString(16).slice(2)}`,
      workspaceId,
      paneId,
      closedAt: Date.now(),
      tabIndex,
      tab: this.cloneTab(tab),
    };

    this.recentlyClosedTabs = [
      record,
      ...this.recentlyClosedTabs.filter(entry =>
        !(
          entry.workspaceId === workspaceId &&
          entry.paneId === paneId &&
          entry.tab.id === tab.id
        )
      ),
    ].slice(0, RECENTLY_CLOSED_TABS_LIMIT);

    this.persistRecentlyClosedTabs();
  }

  private cloneTab(tab: TabInfo): TabInfo {
    if (typeof structuredClone === 'function') {
      return structuredClone(tab);
    }

    return JSON.parse(JSON.stringify(tab)) as TabInfo;
  }

  private loadRecentlyClosedTabs(): RecentlyClosedTab[] {
    try {
      const raw = localStorage.getItem(RECENTLY_CLOSED_TABS_KEY);
      if (!raw) return [];

      const envelope = JSON.parse(raw) as RecentlyClosedTabEnvelope;
      if (envelope?.version !== 1 || !Array.isArray(envelope.entries)) {
        return [];
      }

      return envelope.entries
        .filter(entry =>
          !!entry?.id &&
          !!entry.workspaceId &&
          (entry.paneId === 'left' || entry.paneId === 'right') &&
          !!entry.tab?.id
        )
        .slice(0, RECENTLY_CLOSED_TABS_LIMIT)
        .map(entry => ({
          ...entry,
          tab: this.cloneTab(entry.tab),
        }));
    } catch {
      return [];
    }
  }

  private persistRecentlyClosedTabs(): void {
    this.closedTabsRevision.update(value => value + 1);
    try {
      const envelope: RecentlyClosedTabEnvelope = {
        version: 1,
        entries: this.recentlyClosedTabs.slice(
          0,
          RECENTLY_CLOSED_TABS_LIMIT,
        ),
      };
      localStorage.setItem(
        RECENTLY_CLOSED_TABS_KEY,
        JSON.stringify(envelope),
      );
    } catch {
      // Session recovery is best-effort.
    }
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
      color: 'var(--vsc-accent-blue)',
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
          title: locationLabel(path) || this.getPathTitle(path),
          path,
          location: describeLocation(path) ?? undefined,
          kind: 'dir',
      surfaceId: 'dir',
          isActive: true,
          isPinned: false,
          createdAt: Date.now(),
        },
        {
          id: previewId,
          title: 'Preview',
          path: '',
          kind: 'viewer',
      surfaceId: 'viewer',
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
    source?: { sourcePane: 'left' | 'right'; sourceTabId: string; sourceDir: string; fileList?: ViewerFileListItem[] },
  ): boolean {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace) return false;

    const next = targetPath || '';
    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const tabIndex = pane.tabs.findIndex((t) => t.id === tabId);
    if (tabIndex === -1) return false;

    const currentTab = pane.tabs[tabIndex];
    if ((currentTab.kind ?? 'dir') !== 'viewer') return false;

    const updatedTab: TabInfo = {
      ...currentTab,
      path: next,
      title: next || 'Preview',
      viewer: {
        targetPath: next,
        fileList: source?.fileList ?? currentTab.viewer?.fileList ?? [],
        viewerId: currentTab.viewer?.viewerId,
        autoFollow: currentTab.viewer?.autoFollow,
        sourcePane: source?.sourcePane ?? currentTab.viewer?.sourcePane,
        sourceTabId: source?.sourceTabId ?? currentTab.viewer?.sourceTabId,
        sourceDir: source?.sourceDir ?? currentTab.viewer?.sourceDir,
        // archiveContext intentionally omitted — auto-follow always points to FS files
      },
    };

    const updatedTabs = [...pane.tabs];
    updatedTabs[tabIndex] = updatedTab;

    const updatedPane: PaneState = {
      ...pane,
      tabs: updatedTabs,
    };

    this.updateWorkspace(
      workspace.id,
      { [paneId === 'left' ? 'leftPane' : 'rightPane']: updatedPane },
      false,
    );
    return true;
  }

  addSurfaceTab(
    paneId: string,
    location: string,
    surfaceId: string,
    opts?: { activate?: boolean; closable?: boolean; title?: string },
  ): TabInfo | null {
    const workspace = this.activeWorkspaceSubject.value;
    const normalizedSurfaceId = surfaceId.trim();
    if (!workspace || !normalizedSurfaceId) return null;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    const activate = opts?.activate !== false;
    const descriptor = describeLocation(location) ?? undefined;
    const newTab: TabInfo = {
      id: this.generateId(),
      title: opts?.title?.trim() || locationLabel(location) || normalizedSurfaceId,
      path: descriptor?.location ?? location,
      location: descriptor,
      surfaceId: normalizedSurfaceId,
      isClosable: opts?.closable === false ? false : true,
      isActive: activate,
      isPinned: false,
      createdAt: Date.now(),
    };

    if (activate) {
      pane.tabs.forEach(tab => (tab.isActive = false));
      pane.activeTabId = newTab.id;
    }

    pane.tabs.push(newTab);
    this.updateWorkspace(workspace.id, {
      [paneId === 'left' ? 'leftPane' : 'rightPane']: pane,
    });
    return newTab;
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
      fileList?: ViewerFileListItem[];
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
      surfaceId: 'viewer',
      viewer: {
        targetPath: targetPath || '',
        fileList: opts?.fileList ?? [],
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
      surfaceId: 'size-manager',
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

  private normalizePersistedTabPath(path: string | undefined, surfaceId: string, homePath: string): string {
    const raw = (path ?? '').trim();
    if (surfaceId === 'viewer') {
      return raw ? this.normalizePath(raw) : '';
    }

    if (!raw) {
      return this.defaultFilesystemRoot(homePath);
    }

    const normalized = this.normalizePath(raw);
    if (surfaceId !== 'viewer' && normalized === '/' && this.isWindowsHomePath(homePath)) {
      return this.defaultFilesystemRoot(homePath);
    }

    return normalized;
  }

  private defaultFilesystemRoot(homePath: string): string {
    return this.isWindowsHomePath(homePath) ? homePath : '/';
  }

  private isWindowsHomePath(path: string): boolean {
    return /^filesystem:\/\/[a-z](?:\/.*)?$/i.test((path || '').trim());
  }

  private generateId(): string {
    return `${Date.now()}-${Math.random().toString(36).substring(2, 11)}`;
  }

  /**
   * Derive a short display title from a filesystem path.
   *
   * Rules:
   *   /                     → /
    *   filesystem://c             → C:
   *   /home/user/documents  → documents
    *   filesystem://c/Users/Alice/docs → docs
   */
  private getPathTitle(path: string): string {
    const normalized = this.normalizePath(path);
    if (normalized === 'registry://') return 'Registry';
    if (normalized.startsWith('registry://')) {
      const rel = normalized.slice('registry://'.length);
      return rel.split('/').filter(Boolean).pop() || 'Registry';
    }
    const driveMatch = /^(?:filesystem|drive):\/\/([a-z])(\/.*)?$/i.exec(normalized);
    if (driveMatch) {
      const rel = (driveMatch[2] || '').replace(/^\/+/, '');
      return rel.split('/').filter(Boolean).pop() || `${driveMatch[1].toUpperCase()}:`;
    }
    const aliasMatch = /^([a-z][a-z0-9+.-]*):\/\/(.*)?$/i.exec(normalized);
    if (aliasMatch) {
      const scheme = aliasMatch[1].toLowerCase();
      const rel = (aliasMatch[2] || '').replace(/^\/+/, '');
      return rel.split('/').filter(Boolean).pop() || scheme;
    }
    return normalized;
  }

  private normalizePath(path: string): string {
    return normalizeReferencePath(path);
  }

  private getRootForPath(path: string): string {
    const normalized = this.normalizePath(path);
    if (normalized.startsWith('registry://')) return 'registry://';
    const driveMatch = /^(?:filesystem|drive):\/\/([a-z])(?:\/.*)?$/i.exec(normalized);
    if (driveMatch) return `filesystem://${driveMatch[1].toLowerCase()}`;
    const aliasMatch = /^([a-z][a-z0-9+.-]*):\/\//i.exec(normalized);
    if (aliasMatch) return `${aliasMatch[1].toLowerCase()}://`;
    return '/';
  }

  getAllTabPaths(): string[] {
    const state = this.workspacesSubject.value;
    const paths: string[] = [];
    for (const ws of state.workspaces) {
      for (const pane of [ws.leftPane, ws.rightPane]) {
        if (pane && Array.isArray(pane.tabs)) {
          for (const tab of pane.tabs) {
            const location = tabLocation(tab);
            if (location) paths.push(location);
          }
        }
      }
    }
    return paths;
  }

  private findTab(paneId: 'left' | 'right', tabId: string): TabInfo | null {
    const workspace = this.activeWorkspaceSubject.value;
    if (!workspace || !tabId) return null;

    const pane = paneId === 'left' ? workspace.leftPane : workspace.rightPane;
    return pane.tabs.find((tab) => tab.id === tabId) ?? null;
  }

  private cloneArchiveState(state: ArchiveTabState): ArchiveTabState {
    return {
      ...state,
      exitViewState: state.exitViewState
        ? { ...state.exitViewState, selectedPaths: [...state.exitViewState.selectedPaths] }
        : undefined,
      archiveStack: state.archiveStack.map(frame => ({
        ...frame,
        listing: {
          ...frame.listing,
          entries: Array.isArray(frame.listing?.entries)
            ? frame.listing.entries.map(entry => ({ ...entry }))
            : [],
        },
        nestedChain: [...frame.nestedChain],
      })),
    };
  }

  private parseTabViewStateEnvelope(viewStateJson: string | null | undefined): PersistedTabViewStateEnvelope {
    if (!viewStateJson) {
      return { version: 1 };
    }

    try {
      const parsed = JSON.parse(viewStateJson) as Record<string, unknown>;
      if (!parsed || typeof parsed !== 'object') {
        return { version: 1 };
      }

      if ('dir' in parsed || 'archive' in parsed || 'browserPaneSplit' in parsed || 'version' in parsed) {
        return parsed as PersistedTabViewStateEnvelope;
      }

      return { version: 1, dir: parsed };
    } catch {
      return { version: 1 };
    }
  }

  private serializeTabViewStateEnvelope(envelope: PersistedTabViewStateEnvelope): string | undefined {
    const next: PersistedTabViewStateEnvelope = { version: 1 };

    if (envelope.dir && typeof envelope.dir === 'object') {
      next.dir = envelope.dir;
    }
    if (envelope.archive) {
      next.archive = this.cloneArchiveState(envelope.archive);
    }
    if (typeof envelope.browserPaneSplit === 'number' && Number.isFinite(envelope.browserPaneSplit)) {
      next.browserPaneSplit = envelope.browserPaneSplit;
    }

    if (!next.dir && !next.archive && typeof next.browserPaneSplit !== 'number') {
      return undefined;
    }

    return JSON.stringify(next);
  }
}