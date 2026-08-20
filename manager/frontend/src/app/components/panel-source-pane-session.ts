import type { FileInfo } from '../shared/protocol';
import { FileType } from '../shared/protocol';
import type { SourceCapabilities } from '../services/source-capabilities';
import type { PanelSourceRegistryService } from '../services/panel-source-registry.service';
import type { PluginLifecycleCoordinatorService } from '../services/plugin-lifecycle-coordinator.service';
import type { WorkspaceService } from '../services/workspace.service';
import type {
  PaneComponent,
  PanePresentation,
  PaneViewportSnapshot,
} from './pane-component.contract';
import { schemeOf } from './pane-renderer-registry.service';

const PARENT_ENTRY_PATH = '__parent__';

export interface PanelSourcePaneSessionEvents {
  readonly changed?: () => void;
  readonly locationChanged?: (path: string) => void;
  readonly cursorChanged?: (entry: FileInfo | null) => void;
  readonly presentationChanged?: (presentation: PanePresentation) => void;
  readonly viewportChanged?: (snapshot: PaneViewportSnapshot) => void;
}

/**
 * Shared shell state for non-directory sibling panes.
 *
 * PanelSource owns data/navigation semantics.
 * A sibling component owns domain presentation.
 * This session owns the narrow pane contracts consumed by DualPane.
 */
export class PanelSourcePaneSession implements PaneComponent {
  paneId: 'left' | 'right' = 'left';
  showHidden = false;

  path = '';
  entries: FileInfo[] = [];
  selectedPaths: ReadonlySet<string> = new Set<string>();
  cursorPath: string | null = null;
  loading = false;
  error = '';
  viewMode: 'list' | 'details' = 'details';
  lastAction = '';

  readonly activeViewerTab = null;
  readonly mountedViewer = null;
  readonly viewerArchiveContext = null;
  readonly currentArchiveFrame = null;

  private backTrail: string[] = [];
  private forwardTrail: string[] = [];
  private loadToken = 0;
  private syncedTabId = '';
  private syncedPath = '';
  private viewport: PaneViewportSnapshot = { topName: null, ratio: 0 };

  constructor(
    private readonly workspace: WorkspaceService,
    private readonly panelSources: PanelSourceRegistryService,
    private readonly pluginLifecycle: PluginLifecycleCoordinatorService,
    private readonly events: PanelSourcePaneSessionEvents = {},
  ) {}

  get activeSurfaceId(): string { return schemeOf(this.path) || 'panel-source'; }
  get pinnableSurfaceId(): string { return this.activeSurfaceId; }

  get tabs(): readonly any[] { return this.workspace.getTabs(this.paneId); }
  get activeTab(): any { return this.workspace.getActiveTab(this.paneId); }

  get visibleEntries(): readonly FileInfo[] { return this.entries; }

  get cursorEntry(): FileInfo | null {
    return this.cursorPath
      ? this.entries.find(entry => entry.path === this.cursorPath) ?? null
      : null;
  }

  get selectedEntries(): FileInfo[] {
    return this.entries.filter(entry => this.selectedPaths.has(entry.path));
  }

  get activePanelSource(): any {
    return this.panelSources.resolve({
      path: this.path,
      entries: this.entries,
      inArchive: false,
    });
  }

  get sourceCapabilities(): SourceCapabilities {
    return this.activePanelSource.capabilities;
  }

  get pinnableLocation(): string { return this.path; }
  get pinnableTitle(): string {
    return this.activePanelSource?.source?.title || this.path;
  }

  get hasSelection(): boolean { return this.selectedPaths.size > 0; }

  // Mutations on specialized panes are domain commands, not generic file
  // mutations. This prevents F7/Delete/etc. from taking the filesystem path.
  get canCreate(): boolean { return false; }
  get canRename(): boolean { return false; }
  get canDelete(): boolean { return false; }
  get canCut(): boolean { return false; }
  get canPaste(): boolean { return false; }

  get isInArchive(): boolean { return false; }
  get isDirectoryTab(): boolean { return true; }

  async sync(
    paneId: 'left' | 'right',
    path: string,
    showHidden: boolean,
  ): Promise<void> {
    this.paneId = paneId;
    this.showHidden = showHidden;
    const tab = this.workspace.getActiveTab(paneId);
    const tabId = tab?.id ?? '';

    if (!path) return;
    if (tabId === this.syncedTabId && path === this.syncedPath && path === this.path) return;

    this.syncedTabId = tabId;
    this.syncedPath = path;
    await this.load(path, { history: 'none', preserveSelection: true });
  }

  async refresh(): Promise<void> {
    if (this.path) await this.load(this.path, { history: 'none', preserveSelection: true });
  }

  focusContent(): void {}
  focusLocation(): void {}
  focusFilter(): void {}

  canGoBack(): boolean { return this.backTrail.length > 0; }
  canGoForward(): boolean { return this.forwardTrail.length > 0; }

  goBack(): void {
    const target = this.backTrail.pop();
    if (!target) return;
    if (this.path) this.forwardTrail.push(this.path);
    void this.load(target, { history: 'none', preserveSelection: false });
  }

  goForward(): void {
    const target = this.forwardTrail.pop();
    if (!target) return;
    if (this.path) this.backTrail.push(this.path);
    void this.load(target, { history: 'none', preserveSelection: false });
  }

  navigateUp(): void {
    if (!this.path) return;
    const resolved = this.activePanelSource;
    const parent = resolved.source.getParentPath({
      path: this.path,
      entries: this.entries,
      inArchive: false,
    });
    if (parent && parent !== this.path) {
      void this.load(parent, { history: 'record', preserveSelection: false });
    }
  }

  navigateTo(path: string): Promise<void> {
    return this.load(path, { history: 'record', preserveSelection: false });
  }

  navigateLinkedLocation(path: string): Promise<boolean> {
    return this.navigateTo(path).then(() => true);
  }

  canNavigateToLocation(path: string): boolean {
    return !!schemeOf(path) && schemeOf(path) === schemeOf(this.path || path);
  }

  currentLocationAvailable(): Promise<boolean> { return Promise.resolve(!!this.path); }

  jumpNavigationHistory(path: string): Promise<boolean> {
    return this.navigateTo(path).then(() => true);
  }

  panePresentation(): PanePresentation {
    return {
      viewMode: this.viewMode,
      sortField: 'name',
      sortDirection: 'asc',
      directoriesFirst: true,
      filterText: '',
      filterVisible: false,
    };
  }

  applyMirroredPresentation(presentation: PanePresentation): void {
    if (presentation.viewMode === 'list' || presentation.viewMode === 'details') {
      this.setViewMode(presentation.viewMode);
    }
  }

  setViewMode(viewMode: 'list' | 'details'): void {
    if (this.viewMode === viewMode) return;
    this.viewMode = viewMode;
    this.events.presentationChanged?.(this.panePresentation());
    this.events.changed?.();
  }

  viewportSnapshot(): PaneViewportSnapshot { return this.viewport; }

  syncViewportToPeer(snapshot: PaneViewportSnapshot): void {
    this.viewport = snapshot;
    this.events.changed?.();
  }

  onScroll(event: Event): void {
    const element = event.target as HTMLElement | null;
    if (!element) return;
    const max = Math.max(0, element.scrollHeight - element.clientHeight);
    this.viewport = {
      topName: this.entries[0]?.name ?? null,
      ratio: max > 0 ? element.scrollTop / max : 0,
    };
    this.events.viewportChanged?.(this.viewport);
  }

  openEntry(entry: FileInfo): Promise<void> {
    if (entry.path === PARENT_ENTRY_PATH) {
      this.navigateUp();
      return Promise.resolve();
    }

    const resolved = this.activePanelSource;
    const activation = resolved.source.activateEntry?.(
      { path: this.path, entries: this.entries, inArchive: false },
      entry,
    );

    if (activation !== undefined) {
      if (typeof activation === 'boolean') {
        if (activation) this.selectPaths([entry.path]);
        return Promise.resolve();
      }
      return activation.then((handled: boolean) => {
        if (handled) this.selectPaths([entry.path]);
      });
    }

    if (entry.type === FileType.DIRECTORY) {
      return this.navigateTo(entry.path);
    }

    this.selectPaths([entry.path]);
    return Promise.resolve();
  }

  openViewer(_entry: FileInfo, _viewerId?: string | null, _autoFollow?: boolean): any { return null; }
  openSelectedWithViewer(_viewerId: string): Promise<void> { return Promise.resolve(); }
  openQuickView(_entry: FileInfo): void {}
  openActiveViewerExternally(): Promise<void> { return Promise.resolve(); }
  returnToViewerSource(): Promise<void> { return Promise.resolve(); }
  openResultsPanel(_title: string, _entries: readonly FileInfo[], _metadata?: unknown): Promise<void> { return Promise.resolve(); }
  openRecentFilesPanel(): Promise<void> { return Promise.resolve(); }
  openSurfaceTab(_surfaceId: string, _title?: string, _location?: string): Promise<void> { return Promise.resolve(); }

  closeActiveTab(): void {
    const tab = this.activeTab;
    if (tab) this.workspace.closeTab(this.paneId, tab.id);
  }

  newTab(): void { this.workspace.addTab(this.paneId, this.path || '', true); }
  nextTab(): void { this.switchRelativeTab(1); }
  previousTab(): void { this.switchRelativeTab(-1); }

  jumpToTab(index: number): void {
    const tab = this.tabs[index];
    if (tab) this.workspace.switchTab(this.paneId, tab.id);
  }

  closeOtherTabs(): void {
    const active = this.activeTab;
    if (!active) return;
    for (const tab of [...this.tabs]) {
      if (tab.id !== active.id && tab.isClosable && !tab.isPinned) {
        this.workspace.closeTab(this.paneId, tab.id);
      }
    }
  }

  closeTabsToRight(): void {
    const active = this.activeTab;
    if (!active) return;
    const index = this.tabs.findIndex(tab => tab.id === active.id);
    for (const tab of this.tabs.slice(index + 1)) {
      if (tab.isClosable && !tab.isPinned) this.workspace.closeTab(this.paneId, tab.id);
    }
  }

  toggleActiveTabPin(): void {
    const active = this.activeTab;
    if (active) this.workspace.toggleTabPin(this.paneId, active.id);
  }

  exportActiveTabId(): string | null { return this.activeTab?.id ?? null; }
  exportTabCount(): number { return this.tabs.length; }

  async restoreWorkspaceState(): Promise<void> {
    const active = this.activeTab;
    if (active?.path) await this.sync(this.paneId, active.path, this.showHidden);
  }

  reloadWorkspaceTabs(_preferredTabId?: string): Promise<void> { return Promise.resolve(); }

  moveCursor(delta: number, extend = false): void {
    const entries = this.selectableEntries();
    if (!entries.length) return;
    const current = Math.max(0, entries.findIndex(entry => entry.path === this.cursorPath));
    const index = Math.max(0, Math.min(entries.length - 1, current + delta));
    const path = entries[index]?.path;
    if (!path) return;
    this.cursorPath = path;
    if (!extend) this.selectedPaths = new Set([path]);
    this.emitCursor();
  }

  moveCursorHome(extend = false): void { this.moveCursor(-Number.MAX_SAFE_INTEGER, extend); }
  moveCursorEnd(extend = false): void { this.moveCursor(Number.MAX_SAFE_INTEGER, extend); }
  moveCursorPage(direction: -1 | 1, extend = false): void { this.moveCursor(direction * 10, extend); }

  selectAll(): void {
    this.selectedPaths = new Set(this.selectableEntries().map(entry => entry.path));
    this.cursorPath ??= this.selectableEntries()[0]?.path ?? null;
    this.emitCursor();
  }

  clearSelection(): void {
    this.selectedPaths = new Set<string>();
    this.events.changed?.();
  }

  toggleCurrentSelection(): void {
    if (!this.cursorPath) return;
    const next = new Set(this.selectedPaths);
    if (next.has(this.cursorPath)) next.delete(this.cursorPath);
    else next.add(this.cursorPath);
    this.selectedPaths = next;
    this.emitCursor();
  }

  extendSelection(delta: number): void { this.moveCursor(delta, true); }

  invertSelection(): void {
    const old = this.selectedPaths;
    this.selectedPaths = new Set(
      this.selectableEntries().filter(entry => !old.has(entry.path)).map(entry => entry.path),
    );
    this.emitCursor();
  }

  selectSameExtension(): void {}
  selectSameType(): void {}

  selectFiles(): void {
    this.selectedPaths = new Set(
      this.selectableEntries().filter(entry => entry.type !== FileType.DIRECTORY).map(entry => entry.path),
    );
    this.emitCursor();
  }

  selectFolders(): void {
    this.selectedPaths = new Set(
      this.selectableEntries().filter(entry => entry.type === FileType.DIRECTORY).map(entry => entry.path),
    );
    this.emitCursor();
  }

  unselectFiles(): void {
    this.selectedPaths = new Set(
      [...this.selectedPaths].filter(path =>
        this.entries.find(entry => entry.path === path)?.type === FileType.DIRECTORY,
      ),
    );
    this.emitCursor();
  }

  unselectFolders(): void {
    this.selectedPaths = new Set(
      [...this.selectedPaths].filter(path =>
        this.entries.find(entry => entry.path === path)?.type !== FileType.DIRECTORY,
      ),
    );
    this.emitCursor();
  }

  selectByMaskPrompt(): Promise<void> { return Promise.resolve(); }
  unselectByMaskPrompt(): Promise<void> { return Promise.resolve(); }

  selectPaths(paths: readonly string[]): void {
    const available = new Set(this.entries.map(entry => entry.path));
    this.selectedPaths = new Set(paths.filter(path => available.has(path)));
    this.cursorPath = [...this.selectedPaths][0] ?? this.cursorPath;
    this.emitCursor();
  }

  selectComparisonItem(path: string): void { this.selectPaths([path]); }

  recoverVisibleCursor(): void {
    if (!this.entries.length) {
      this.cursorPath = null;
      this.selectedPaths = new Set<string>();
    } else if (!this.cursorPath || !this.entries.some(entry => entry.path === this.cursorPath)) {
      this.cursorPath = this.selectableEntries()[0]?.path ?? this.entries[0]?.path ?? null;
    }
    this.emitCursor();
  }

  calculateSelectedSize(): Promise<void> { return Promise.resolve(); }

  selectPeerByName(name: string): boolean {
    const entry = this.entries.find(candidate => candidate.name === name);
    if (!entry) return false;
    this.selectPaths([entry.path]);
    return true;
  }

  selectFromPointer(entry: FileInfo, event: MouseEvent): void {
    const next = new Set(this.selectedPaths);
    if (event.ctrlKey || event.metaKey) {
      if (next.has(entry.path)) next.delete(entry.path);
      else next.add(entry.path);
    } else {
      next.clear();
      next.add(entry.path);
    }
    this.selectedPaths = next;
    this.cursorPath = entry.path;
    this.emitCursor();
  }

  copySelected(): void {}
  cutSelected(): void {}
  paste(): Promise<void> { return Promise.resolve(); }
  deleteSelected(_permanent?: boolean): Promise<void> { return Promise.resolve(); }
  renameSelected(): Promise<void> { return Promise.resolve(); }
  newFolder(): Promise<void> { return Promise.resolve(); }
  newFile(): Promise<void> { return Promise.resolve(); }
  newDocumentTemplate(_kind: string): Promise<void> { return Promise.resolve(); }
  moveSelectionToNewFolder(): Promise<void> { return Promise.resolve(); }
  duplicateSelected(): Promise<void> { return Promise.resolve(); }
  revealSelectedInFileManager(): Promise<void> { return Promise.resolve(); }
  touchSelectedModifiedTime(): Promise<void> { return Promise.resolve(); }
  editSelectedAttributes(): Promise<void> { return Promise.resolve(); }
  showSelectedProperties(): Promise<void> { return Promise.resolve(); }
  showSelectedChecksum(): Promise<void> { return Promise.resolve(); }
  copySelectedPaths(): Promise<void> { return Promise.resolve(); }
  copySelectedRelativePaths(): Promise<void> { return Promise.resolve(); }
  copySelectedFileUris(): Promise<void> { return Promise.resolve(); }
  copySelectedNames(): Promise<void> { return Promise.resolve(); }
  createLinkForSelected(_kind: 'symbolic' | 'hard'): Promise<void> { return Promise.resolve(); }
  createShortcutForSelected(): Promise<void> { return Promise.resolve(); }
  createZipFromSelection(): Promise<void> { return Promise.resolve(); }
  extractSelectedTo(_destinationDir: string, _flattenPaths = false): Promise<boolean> { return Promise.resolve(false); }
  transferArchiveSelectionTo(_destinationDir: string, _operation: 'copy' | 'move'): Promise<null> { return Promise.resolve(null); }
  treeLeft(): void {}
  treeRight(): void {}

  private async load(
    requestedPath: string,
    options: { history: 'record' | 'none'; preserveSelection: boolean },
  ): Promise<void> {
    const path = this.canonicalize(requestedPath);
    if (!path) return;

    const token = ++this.loadToken;
    const previousPath = this.path;
    const previousSelection = new Set(this.selectedPaths);

    this.loading = true;
    this.error = '';
    this.lastAction = `load: ${path}`;
    this.events.changed?.();

    try {
      await this.activatePluginsForLocation(path);

      const context = { path, entries: this.entries, inArchive: false };
      const resolved = this.panelSources.resolve(context);
      if (resolved.source.kind === 'filesystem') {
        throw new Error(`No panel source is registered for ${schemeOf(path)}://.`);
      }

      const listing = await resolved.source.list(context, this.showHidden);
      if (token !== this.loadToken) return;

      const nextPath = listing.location.path || path;
      this.path = nextPath;
      this.entries = this.withParent(nextPath, [...listing.entries] as FileInfo[], resolved.source);
      this.selectedPaths = options.preserveSelection
        ? new Set([...previousSelection].filter(candidate => this.entries.some(entry => entry.path === candidate)))
        : new Set<string>();

      if (!this.cursorPath || !this.entries.some(entry => entry.path === this.cursorPath)) {
        this.cursorPath = this.selectableEntries()[0]?.path ?? this.entries[0]?.path ?? null;
      }

      if (options.history === 'record' && previousPath && previousPath !== nextPath) {
        this.backTrail.push(previousPath);
        this.forwardTrail = [];
      }

      this.syncWorkspacePath(nextPath);
      this.events.locationChanged?.(nextPath);
      this.emitCursor();
      this.events.presentationChanged?.(this.panePresentation());
    } catch (error) {
      if (token !== this.loadToken) return;
      this.error = error instanceof Error ? error.message : String(error);
      this.entries = [];
      this.selectedPaths = new Set<string>();
      this.cursorPath = null;
      this.events.cursorChanged?.(null);
    } finally {
      if (token === this.loadToken) {
        this.loading = false;
        this.events.changed?.();
      }
    }
  }

  private withParent(path: string, entries: FileInfo[], source: any): FileInfo[] {
    const context = { path, entries, inArchive: false };
    const root = source.getRootPath(context);
    if (!path || !root || path === root || entries.some(entry => entry.path === PARENT_ENTRY_PATH)) {
      return entries;
    }

    return [{
      name: '..',
      path: PARENT_ENTRY_PATH,
      type: FileType.DIRECTORY,
      size: 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
    }, ...entries];
  }

  private selectableEntries(): FileInfo[] {
    return this.entries.filter(entry => entry.path !== PARENT_ENTRY_PATH);
  }

  private emitCursor(): void {
    this.events.cursorChanged?.(this.cursorEntry);
    this.events.changed?.();
  }

  private switchRelativeTab(delta: -1 | 1): void {
    const tabs = this.tabs;
    if (tabs.length < 2) return;
    const current = tabs.findIndex(tab => tab.id === this.activeTab?.id);
    const next = ((current < 0 ? 0 : current) + delta + tabs.length) % tabs.length;
    this.workspace.switchTab(this.paneId, tabs[next].id);
  }

  private syncWorkspacePath(path: string): void {
    const active = this.workspace.getActiveTab(this.paneId);
    if (!active || active.path === path) return;
    this.workspace.updateTabPath(this.paneId, active.id, path);
    this.syncedPath = path;
  }

  private async activatePluginsForLocation(path: string): Promise<void> {
    const scheme = schemeOf(path);
    if (!scheme || scheme === 'filesystem') return;

    const results = await this.pluginLifecycle.activateByEvent(`onScheme:${scheme}`);
    const failed = results.find(result => !result.activated);
    if (failed) {
      throw new Error(failed.error || `Plugin ${failed.pluginId} failed to activate for ${scheme}://.`);
    }
  }

  private canonicalize(path: string): string {
    let value = String(path ?? '').trim();
    if (/^\/[a-z][a-z0-9+.-]*:/i.test(value)) value = value.slice(1);
    const scheme = schemeOf(value);
    if (scheme && value.toLowerCase() === `${scheme}:`) return `${scheme}://`;
    return value;
  }
}
