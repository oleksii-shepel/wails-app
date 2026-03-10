// file-pane.component.ts
import { Component, Input, Output, EventEmitter, OnInit, OnChanges, OnDestroy, ViewChild, SimpleChanges } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TabBarComponent, type TabDropEvent } from './tab-bar.component';
import { FindWidgetState } from './filter-widget.component';
import { FileInfo, FileType } from '../shared/protocol';
import { TabInfo, WorkspaceConfig, FilterQuery } from '../shared/protocol-enhanced';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import { ApiService } from '../services/api.service';
import { WorkspaceService } from '../services/workspace.service';
import { FilterService } from '../services/filter.service';
import { PathHistoryService } from '../services/path-history.service';
import {
  ArchiveService, ArchiveFrame, isArchive, buildArchiveDisplayPath, buildArchiveTabTitle,
  parseArchivePath, buildArchivePath
} from '../services/archive.service';
import { parentPath, normalizePath } from './address-bar.utils';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { FileViewContainerComponent, ViewLayout } from './file-view-container.component';
import { ViewerArchiveContext } from './viewer-surface.component';
import { DirSurfaceVm } from '../surfaces/dir/dir-surface.vm';
import { DirSurfaceSession } from '../surfaces/dir/dir-surface-session';
import { DirSurfaceRuntime } from '../surfaces/dir/dir-surface-runtime';
import { FilePaneDirSurfaceRuntime } from '../surfaces/dir/file-pane-dir-surface-runtime';
import { SurfaceContext } from '../surfaces/surface-context';
import { SurfaceOutletComponent } from '../surfaces/surface-outlet.component';
import {
  DirListStreamController,
  DirStreamMetaEvent,
  DirStreamBatchEvent,
  DirStreamDoneEvent,
  DirStreamErrorEvent,
} from '../surfaces/dir/dir-list-stream.controller';
import { EventsOn, EventsOff } from 'wailsjs/runtime';
import { ConfirmDialogService } from '../services/confirm-dialog.service';

declare const window: any;

@Component({
  selector: 'app-file-pane',
  standalone: true,
  imports: [
    CommonModule,
    TabBarComponent,
    SurfaceOutletComponent
],
  template: `
    <div class="file-pane" [class.file-pane--archive]="isInArchive()">
      <app-tab-bar
        [paneId]="paneId"
        [tabs]="tabs"
        [activeTabId]="currentTabId"
        [inArchive]="isInArchive()"
        (tabChange)="onTabChange($event)"
        (tabClose)="onTabClose($event)"
        (tabPin)="onTabPin($event)"
        (tabDragStart)="onTabDragStart($event)"
        (tabDrop)="onTabDrop($event)"
        (newTab)="onNewTab()">
      </app-tab-bar>

      <ng-container *ngIf="surfaceCtx as ctx">
        <app-surface-outlet
          #surfaceOutlet
          [ctx]="ctx">
        </app-surface-outlet>
      </ng-container>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    .file-pane {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      background: var(--vsc-editor-background);
      overflow: hidden;
    }

    .file-pane--archive {
      background: color-mix(in srgb, var(--vsc-accent-orange) 4%, var(--vsc-editor-background));
    }

    app-tab-bar {
      display: block;
      flex-shrink: 0;
    }
  `]
})
export class FilePaneComponent implements OnInit, OnChanges, OnDestroy {
  private readonly dirStream: DirListStreamController;
  private readonly dirSessions = new Map<string, DirSurfaceSession>();
  private activeDirTabId: string | null = null;
  readonly dirVm: DirSurfaceVm = new (class implements DirSurfaceVm {
    constructor(private pane: FilePaneComponent) {}

    get paneId() { return this.pane.paneId; }
    get pathReady() { return this.pane.pathReady; }
    get currentPath() { return this.pane.currentPath; }
    get showHidden() { return this.pane.showHidden; }
    get paneActive() { return this.pane.paneActive; }

    get entries() { return this.pane.filteredEntries; }
    get selectedFiles() { return this.pane.selectedFiles; }
    get cursorPath() { return this.pane.selectionCursorPath; }
    get error() { return this.pane.error; }

    get currentView() { return this.pane.currentView; }
    get sortField() { return this.pane.sortField; }
    get sortDirection() { return this.pane.sortDirection; }
    get directoriesFirst() { return this.pane.directoriesFirst; }

    get archiveMoveWarning() { return this.pane.archiveMoveWarning; }
    get archiveMoveEntryCount() { return this.pane.archivePendingMoveEntries.length; }

    onPathChange = (path: string) => this.pane.onPathChange(path);
    onNavigateUp = () => this.pane.onNavigateUp();
    onRefresh = () => this.pane.onRefresh();

    onViewChange = (view: ViewLayout) => this.pane.onViewChange(view);
    onSortChange = (sort: { field: string; direction: 'asc' | 'desc' }) => this.pane.onSortChange(sort);
    onDirectoriesFirstChange = (v: boolean) => this.pane.onDirectoriesFirstChange(v);

    onCreateFolder = () => this.pane.onCreateFolder();
    onDeleteSelected = () => this.pane.onDeleteSelected();
    onCopyToOther = () => this.pane.onCopyToOther();
    onMoveToOther = () => this.pane.onMoveToOther();

    onFindStateChange = (state: FindWidgetState | null) => this.pane.onFindStateChange(state);
    onFindNext = () => this.pane.onFindNext();
    onFindPrev = () => this.pane.onFindPrev();

    onSelectionChange = (entries: FileInfo[]) => this.pane.onSelectionChange(entries);
    onNavigate = (entry: FileInfo) => this.pane.onNavigate(entry);
    onOpenInNewTab = (entry: FileInfo) => this.pane.onOpenInNewTab(entry);
    onContextMenu = (ev: { event: MouseEvent; entry: FileInfo }) => this.pane.onContextMenu(ev);
    onNodeDoubleClick = (ev: any) => this.pane.onNodeDoubleClick(ev);
    onDragDrop = (ev: any) => this.pane.onDragDrop(ev);
    onRename = (ev: { entry: FileInfo; newName: string }) => this.pane.onRename(ev);

    onArchiveMoveConfirmed = () => this.pane.executePendingMove();
    onArchiveMoveCancelled = () => { this.pane.archiveMoveWarning = false; };
  })(this);
    /**
     * Extract selected entries from the current archive frame.
     * If all top-level entries are selected, extracts the whole archive (root forrest).
     * Otherwise, extracts only the selected files/folders.
     */
    async onExtractSelected(destDir: string, flattenPaths = false): Promise<void> {
      if (!this.isInArchive() || this.selectedFiles.size === 0) return;
      const frame = this.archiveStack[this.archiveStack.length - 1];
      const entries = Array.from(this.selectedFiles);
      try {
        const result = await this.archiveService.extractEntries(frame, entries, destDir, flattenPaths);
        if (result.failed > 0) {
          this.confirmDialog.alert('Extract Failed\n' + (result.errors ?? []).map((e: any) => e.error).join('\n'));
        } else {
          this.confirmDialog.alert(`Extraction Complete\n${result.succeeded} file(s) extracted to destination.`);
        }
      } catch (err: any) {
        this.confirmDialog.alert(`Extract Failed\n${err.message}`);
      }
    }
  @ViewChild('surfaceOutlet') surfaceOutlet?: SurfaceOutletComponent;

  private get addressBar() { return this.surfaceOutlet?.addressBar; }
  private get fileViewContainer() { return this.surfaceOutlet?.fileViewContainer; }
  private get findWidget() { return this.surfaceOutlet?.findWidget; }

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() workspace: WorkspaceConfig | null = null;
  @Input() showHidden = false;
  @Input() paneActive = true;

  @Output() pathChange    = new EventEmitter<string>();
  @Output() copyToOther   = new EventEmitter<string[]>();
  @Output() moveToOther   = new EventEmitter<string[]>();
  @Output() previewTargetChange = new EventEmitter<string>();
  @Output() activeTabChange = new EventEmitter<TabInfo>();

  currentTabId  = '';
  pathReady     = false;
  tabs: TabInfo[] = [];
  private optimisticHideListener?: (event: Event) => void;
  private optimisticRestoreListener?: (event: Event) => void;

  archiveStack: ArchiveFrame[] = [];

  // Archive move state
  archiveMoveWarning = false;
  archivePendingMoveEntries: string[] = [];
  archivePendingMoveDestDir = '';
  private fsChangeDebounceTimer: ReturnType<typeof setTimeout> | null = null;
  private activeDirStreamPreserveUI = false;
  private pendingFocusPath: string | null = null;

  private destroy$ = createSubject<void>();
  private lastSeenActiveTabId: string | null = null;
  private lastSeenActiveTabPath: string | null = null;

  FileType = FileType;

  // Minimal UI runtime adapter for dir sessions (tree vs list/grid/details).
  private readonly dirRuntime: DirSurfaceRuntime = new FilePaneDirSurfaceRuntime(
    () => this.currentView,
    () => this.fileViewContainer,
    () => this.filteredEntries,
    PARENT_ENTRY_PATH,
  );

  private get dir(): DirSurfaceSession {
    const id = this.activeDirTabId ?? this.currentTabId;
    let session = id ? this.dirSessions.get(id) : undefined;
    if (!session) {
      session = new DirSurfaceSession(id || 'default');
      // Default view prefs
      session.currentView = this.currentViewPrefFromStorage();
      const sf = localStorage.getItem('fileManager_sortField');
      const sd = localStorage.getItem('fileManager_sortDirection') as ('asc' | 'desc' | null);
      if (sf) session.sortField = sf;
      if (sd === 'asc' || sd === 'desc') session.sortDirection = sd;
      const df = localStorage.getItem('fileManager_directoriesFirst');
      if (df === 'true' || df === 'false') session.directoriesFirst = df === 'true';

      this.dirSessions.set(session.tabId, session);
    }
    // If current tab is a dir tab, keep activeDirTabId aligned.
    const t = this.tabs.find(t => t.id === this.currentTabId);
    if ((t?.kind ?? 'dir') === 'dir') this.activeDirTabId = session.tabId;
    return session;
  }

  private currentViewPrefFromStorage(): ViewLayout {
    const saved = localStorage.getItem('fileManager_viewMode') as ViewLayout | null;
    return (saved === 'list' || saved === 'grid' || saved === 'details' || saved === 'tree') ? saved : 'list';
  }

  // Dir state proxies (backed by the active dir session)
  get currentPath(): string { return this.dir.currentPath; }
  set currentPath(v: string) { this.dir.currentPath = v; }

  get fsParentDir(): string { return this.dir.fsParentDir; }
  set fsParentDir(v: string) { this.dir.fsParentDir = v; }

  get entries(): FileInfo[] { return this.dir.entries; }
  set entries(v: FileInfo[]) { this.dir.entries = v; }

  get filteredEntries(): FileInfo[] { return this.dir.filteredEntries; }
  set filteredEntries(v: FileInfo[]) { this.dir.filteredEntries = v; }

  get selectedFiles(): Set<string> { return this.dir.selectedFiles; }
  set selectedFiles(v: Set<string>) { this.dir.selectedFiles = v; }

  get loading(): boolean { return this.dir.loading; }
  set loading(v: boolean) { this.dir.loading = v; }

  get error(): string | null { return this.dir.error; }
  set error(v: string | null) { this.dir.error = v; }

  get filterQuery(): FilterQuery { return this.dir.filterQuery; }
  set filterQuery(v: FilterQuery) { this.dir.filterQuery = v; }

  get currentView(): ViewLayout { return this.dir.currentView; }
  set currentView(v: ViewLayout) { this.dir.currentView = v; }

  get sortField(): string { return this.dir.sortField; }
  set sortField(v: string) { this.dir.sortField = v; }

  get sortDirection(): 'asc' | 'desc' { return this.dir.sortDirection; }
  set sortDirection(v: 'asc' | 'desc') { this.dir.sortDirection = v; }

  get directoriesFirst(): boolean { return this.dir.directoriesFirst; }
  set directoriesFirst(v: boolean) { this.dir.directoriesFirst = v; }

  get selectionCursorPath(): string | null { return this.dir.selectionCursorPath; }
  set selectionCursorPath(v: string | null) { this.dir.selectionCursorPath = v; }

  get selectionAnchorPath(): string | null { return this.dir.selectionAnchorPath; }
  set selectionAnchorPath(v: string | null) { this.dir.selectionAnchorPath = v; }

  getOptimisticHiddenPaths(): string[] { return Array.from(this.dir.optimisticHiddenPaths); }
  setOptimisticHiddenPaths(paths: string[]): void {
    this.dir.optimisticHiddenPaths = new Set((paths ?? []).filter(Boolean));
  }

  get activeTabKind(): 'dir' | 'viewer' {
    const t = this.tabs.find(t => t.id === this.currentTabId);
    return (t?.kind ?? 'dir');
  }

  get activeViewerTargetPath(): string {
    const t = this.tabs.find(t => t.id === this.currentTabId);
    if (!t) return '';
    if ((t.kind ?? 'dir') !== 'viewer') return '';
    return (t.viewer?.targetPath ?? '');
  }

  get activeViewerId(): string | null {
    const t = this.tabs.find(t => t.id === this.currentTabId);
    if (!t || (t.kind ?? 'dir') !== 'viewer') return null;
    return (t.viewer?.viewerId ?? null);
  }

  get activeViewerArchiveContext(): ViewerArchiveContext | null {
    const t = this.tabs.find(t => t.id === this.currentTabId);
    if (!t || (t.kind ?? 'dir') !== 'viewer') return null;
    return (t.viewer?.archiveContext as ViewerArchiveContext) ?? null;
  }

  constructor(
    private apiService: ApiService,
    private workspaceService: WorkspaceService,
    private filterService: FilterService,
    private pathHistoryService: PathHistoryService,
      private confirmDialog: ConfirmDialogService,
    private archiveService: ArchiveService
  ) {
    this.dirStream = new DirListStreamController(this.apiService);
  }

  get surfaceCtx(): SurfaceContext | null {
    const tab = this.tabs.find(t => t.id === this.currentTabId);
    if (!tab) return null;
    return {
      paneId: this.paneId,
      tab,
      dirVm: this.dirVm,
      viewer: {
        targetPath: this.activeViewerTargetPath,
        archiveContext: this.activeViewerArchiveContext,
        viewerId: this.activeViewerId,
      },
    };
  }

  ngOnInit(): void  { 
    this.loadViewPreference();
    this.loadPaneState();
    this.bindFsChangeEvents();
    this.setupOptimisticDnDListeners();

    // Keep panes in sync when tab state changes elsewhere (e.g. cross-pane tab drag/drop).
    this.workspaceService.activeWorkspace$
      .pipe(takeUntil(this.destroy$))
      .subscribe(ws => {
        if (!ws || !this.workspace) return;
        if (ws.id !== this.workspace.id) return;
        this.onWorkspaceUpdate(ws);
      });
  }
  
  ngOnChanges(changes: SimpleChanges): void {
    if (changes['workspace']) {
      // The input is bound to FileBrowserComponent.activeWorkspace which starts as null.
      // That means the *first* change Angular sees is often null, and the transition
      // to a real workspace would be a later change. Always load once we have a
      // non-null workspace so tabs/pane render without requiring a user click.
      if (this.workspace) {
        this.loadViewPreference();
        this.lastSeenActiveTabId = null;
        this.lastSeenActiveTabPath = null;
        this.loadPaneState();
      }
      return;
    }

    if (changes['showHidden'] && !changes['showHidden'].firstChange) {
      this.onRefresh();
    }

    if (changes['paneActive'] && this.paneActive && !changes['paneActive'].firstChange) {
      // Pane just became active вЂ” ensure there is a visible cursor.
      if (this.selectedFiles.size === 0 && !this.selectionCursorPath) {
        const selectable = this.dirRuntime.getSelectableEntries();
        if (selectable.length) {
          this.selectionCursorPath = selectable[0].path;
          this.selectionAnchorPath = selectable[0].path;
          // TC: just position cursor, don't mark the item
          this.fileViewContainer?.scrollToCursorPath(selectable[0].path);
        }
      }
    }
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();

    this.unbindFsChangeEvents();

    if (this.optimisticHideListener) {
      window.removeEventListener('pane:dnd-optimistic-hide', this.optimisticHideListener);
      this.optimisticHideListener = undefined;
    }
    if (this.optimisticRestoreListener) {
      window.removeEventListener('pane:dnd-optimistic-restore', this.optimisticRestoreListener);
      this.optimisticRestoreListener = undefined;
    }
  }

  private onWorkspaceUpdate(ws: WorkspaceConfig): void {
    const pane = this.paneId === 'left' ? ws.leftPane : ws.rightPane;
    const nextActiveId = pane.activeTabId;
    const nextActivePath = pane.tabs.find(t => t.id === nextActiveId)?.path ?? '';

    // First event: establish baseline.
    if (this.lastSeenActiveTabId === null) {
      this.lastSeenActiveTabId = nextActiveId;
      this.lastSeenActiveTabPath = nextActivePath;
      this.tabs = this.workspaceService.getTabs(this.paneId);
      this.currentTabId = nextActiveId;
      return;
    }

    const activeChanged = nextActiveId !== this.lastSeenActiveTabId || nextActivePath !== this.lastSeenActiveTabPath;
    this.lastSeenActiveTabId = nextActiveId;
    this.lastSeenActiveTabPath = nextActivePath;

    if (!activeChanged) {
      // Reorder/pin changes etc вЂ” just update the tab strip.
      this.tabs = this.workspaceService.getTabs(this.paneId);
      return;
    }

    // If we already navigated to the new active tab locally, avoid a redundant reload.
    if (this.currentTabId === nextActiveId && this.currentPath === nextActivePath) {
      this.tabs = this.workspaceService.getTabs(this.paneId);
      return;
    }

    // Active tab changed (including cross-pane drag) вЂ” load the new active path.
    this.loadPaneState();
  }

  onTabDrop(event: TabDropEvent): void {
    if (!event) return;
    this.workspaceService.moveTab(event.fromPaneId, event.toPaneId, event.tabId, event.toIndex);

    // If the drop landed in this pane, ensure we navigate to the newly-active tab.
    if (event.toPaneId === this.paneId) {
      this.tabs = this.workspaceService.getTabs(this.paneId);

      const active = this.workspaceService.getActiveTab(this.paneId);
      if (active?.id === event.tabId) {
        const t = this.tabs.find(tab => tab.id === event.tabId);
        if (t) void this.onTabChange(t);
      }
    }
  }

  onTabDragStart(tab: TabInfo): void {
    // Snapshot current tab state before it potentially leaves this pane.
    if (!tab || tab.id !== this.currentTabId) return;
    if (!this.currentTabId || !this.currentPath) return;

    if (this.isInArchive()) {
      this.workspaceService.saveArchiveState(this.paneId, this.currentTabId, {
        path: this.currentPath,
        archiveStack: this.archiveStack,
        fsDirPath: this.archiveStack[0]?.fsPath ?? '',
        fsParentDir: this.fsParentDir,
      });
    } else {
      this.workspaceService.updateTabPath(this.paneId, this.currentTabId, this.currentPath);
    }
  }

  private setupOptimisticDnDListeners(): void {
    this.optimisticHideListener = (ev: Event) => {
      const detail = (ev as CustomEvent).detail as { paneId?: 'left' | 'right'; paths?: string[] } | undefined;
      if (!detail?.paneId || detail.paneId !== this.paneId) return;
      const paths = (detail.paths ?? []).filter(p => p && p !== PARENT_ENTRY_PATH);
      if (!paths.length) return;

      for (const p of paths) this.dir.optimisticHiddenPaths.add(p);
      for (const p of paths) this.selectedFiles.delete(p);
      this.selectedFiles = new Set(this.selectedFiles);
      this.applyFilter();
    };

    this.optimisticRestoreListener = (ev: Event) => {
      const detail = (ev as CustomEvent).detail as { paneId?: 'left' | 'right'; paths?: string[] } | undefined;
      if (!detail?.paneId || detail.paneId !== this.paneId) return;
      const paths = (detail.paths ?? []).filter(p => p && p !== PARENT_ENTRY_PATH);
      if (!paths.length) return;

      for (const p of paths) this.dir.optimisticHiddenPaths.delete(p);
      this.applyFilter();
    };

    window.addEventListener('pane:dnd-optimistic-hide', this.optimisticHideListener);
    window.addEventListener('pane:dnd-optimistic-restore', this.optimisticRestoreListener);
  }

  get entriesForAnalysis(): FileInfo[] {
    return this.entries.filter(e => e.path !== PARENT_ENTRY_PATH);
  }

  // в”Ђв”Ђ View System Methods в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  private loadViewPreference(): void {
    const savedView = localStorage.getItem('fileManager_viewMode') as ViewLayout;
    if (savedView && ['list', 'grid', 'tree', 'details'].includes(savedView)) {
      this.currentView = savedView;
    }
    
    const savedSortField = localStorage.getItem('fileManager_sortField');
    const savedSortDir = localStorage.getItem('fileManager_sortDirection') as 'asc' | 'desc';
    if (savedSortField) this.sortField = savedSortField;
    if (savedSortDir) this.sortDirection = savedSortDir;

    const savedDirsFirst = localStorage.getItem('fileManager_directoriesFirst');
    if (savedDirsFirst === 'true' || savedDirsFirst === 'false') {
      this.directoriesFirst = savedDirsFirst === 'true';
    }
  }

  private saveViewPreference(): void {
    localStorage.setItem('fileManager_viewMode', this.currentView);
    localStorage.setItem('fileManager_sortField', this.sortField);
    localStorage.setItem('fileManager_sortDirection', this.sortDirection);
    localStorage.setItem('fileManager_directoriesFirst', String(this.directoriesFirst));
  }

  async onViewChange(view: ViewLayout): Promise<void> {
    const previousView = this.currentView;
    this.currentView = view;
    this.saveViewPreference();

    // Tree navigation keeps currentPath in sync without replacing entries.
    // When leaving tree mode, load the current folder so list/grid/details
    // always render the active directory contents.
    if (previousView === 'tree' && view !== 'tree') {
      await this.refreshCurrentPathTransparently();
      return;
    }

    // Refresh entries when switching to ensure proper sorting/filtering
    this.applyFilter();
    this.persistDirTabState();
  }

  onSortChange(sort: { field: string; direction: 'asc' | 'desc' }): void {
    this.sortField = sort.field;
    this.sortDirection = sort.direction;
    this.saveViewPreference();
    this.applyFilter();
    this.persistDirTabState();
  }

  onDirectoriesFirstChange(value: boolean): void {
    this.directoriesFirst = value;
    this.saveViewPreference();
    this.applyFilter();
    this.persistDirTabState();
  }

  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void {
    // Handle context menu - can integrate with existing context menu system
    console.log('Context menu for:', event.entry.name);
  }

  onNodeDoubleClick(entry: FileInfo): void {
    if (entry.path === PARENT_ENTRY_PATH) { this.onNavigateUp(); return; }
    // In tree view, non-directory double-clicks must bypass onNavigate (which
    // now returns early to update the address bar). Handle open actions here.
    if (this.currentView === 'tree' && entry.type !== FileType.DIRECTORY) {
      if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
        this.openRootArchive(entry.path);
        return;
      }
      if (entry.name.toLowerCase().endsWith('.lnk') || entry.type === FileType.SYMLINK) {
        this.resolveAndOpen(entry.path);
        return;
      }
      this.apiService.openFile(entry.path).catch(err =>
        console.error('Failed to open file:', err)
      );
      return;
    }
    this.onNavigate(entry);
  }

  /** Resolves a .lnk or symlink and either navigates into (dir) or opens (file). */
  private resolveAndOpen(path: string): void {
    this.apiService.resolveLink(path).then(result => {
      if (result.isDir) {
        this.loadDirectory(result.target);
      } else {
        this.apiService.openFile(result.target).catch(err =>
          console.error('Failed to open link target:', err)
        );
      }
    }).catch(err => {
      // Fallback: let the OS handle it
      console.warn('Could not resolve link, falling back to OS open:', err);
      this.apiService.openFile(path).catch(e =>
        console.error('Failed to open file:', e)
      );
    });
  }

  // в”Ђв”Ђ Virtual ".." entry helpers в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  /** Returns true when the current path is a filesystem root (e.g. "/" or "C:/"). */
  private isAtFsRoot(path: string): boolean {
    const p = path.replace(/\\/g, '/');
    return p === '/' || /^[A-Za-z]:\/?\s*$/.test(p);
  }

  /** Creates the virtual parent-directory entry. */
  private makeParentEntry(): FileInfo {
    return {
      name: '..',
      path: PARENT_ENTRY_PATH,
      type: FileType.DIRECTORY,
      size: 0,
      modified: 0,
    } as FileInfo;
  }

  /**
   * Prepend the virtual ".." entry to a raw entry list when appropriate.
   * вЂ“ Always prepended inside archives (so the user can navigate out).
   * вЂ“ Prepended for normal directories unless we are at the filesystem root.
   */
  private withParentEntry(raw: FileInfo[], inArchive: boolean, fsPath: string): FileInfo[] {
    if (raw.some(e => e.path === PARENT_ENTRY_PATH)) {
      return raw;
    }
    if (inArchive || !this.isAtFsRoot(fsPath)) {
      return [this.makeParentEntry(), ...raw];
    }
    return raw;
  }

  private pathIsInsideArchive(path: string): boolean {
    const { fsPath } = parseArchivePath(path);
    const name = fsPath.replace(/\\/g, '/').split('/').filter(Boolean).pop() ?? '';
    return isArchive(name);
  }

  private async loadPathTransparently(path: string): Promise<void> {
    if (this.pathIsInsideArchive(path)) {
      await this.restoreArchiveFromPath(path);
      return;
    }

    await this.loadDirectory(path);
  }

  private async refreshCurrentPathTransparently(): Promise<void> {
    if (!this.currentPath) return;

    if (this.isInArchive()) {
      const currentFrame = this.archiveStack[this.archiveStack.length - 1];
      if (currentFrame) {
        this.renderArchiveFrame(currentFrame);
        return;
      }
    }

    if (this.pathIsInsideArchive(this.currentPath)) {
      await this.restoreArchiveFromPath(this.currentPath);
      return;
    }

    await this.loadDirectory(this.currentPath, { preserveUI: true });
  }

  // в”Ђв”Ђ Pane state в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  private async loadPaneState(): Promise<void> {
    if (!this.workspace) return;
    this.tabs = this.workspaceService.getTabs(this.paneId);
    const activeTab  = this.workspaceService.getActiveTab(this.paneId);
    const savedPath  = activeTab?.path ?? '';
    const activeKind = (activeTab?.kind ?? 'dir');

    const allPaths   = this.workspaceService.getAllTabPaths();
    const windowsRoot = allPaths.find(p => /^[A-Za-z]:[/\\]/.test(p));
    let platformRoot = windowsRoot
      ? windowsRoot.replace(/\\/g, '/').replace(/^([A-Za-z]:).*$/, '$1/')
      : '/';

    // No cached Windows path yet вЂ” ask the backend for the system drive.
    // This ensures a fresh Windows install starts on C:/ (or the actual
    // system drive) instead of the Unix-style '/'.
    if (platformRoot === '/') {
      try {
        const osInfo = await this.apiService.getOSInfo();
        if (osInfo?.systemDrive) {
          const drive = (osInfo.systemDrive as string).replace(/\\/g, '/').toUpperCase();
          platformRoot = drive.endsWith('/') ? drive : drive + '/';
        }
      } catch { /* ignore вЂ” stays '/' on non-Windows */ }
    }

    const isWindows   = platformRoot !== '/';
    const pathIsStale = !savedPath || (isWindows && savedPath === '/');
    const pathToLoad  = pathIsStale ? platformRoot : savedPath;

    this.pathReady   = true;
    if (activeTab) this.currentTabId = activeTab.id;

    // Viewer tabs don't drive directory loading; keep directory state as-is.
    if (activeKind === 'viewer') {
      // Keep a stable "active dir session" for operations/preview origin restore.
      if (!this.activeDirTabId) {
        const firstDir = this.tabs.find(t => (t.kind ?? 'dir') === 'dir');
        if (firstDir) this.activeDirTabId = firstDir.id;
      }
      // Ensure currentPath isn't empty (used by various helpers even if UI is hidden).
      if (!this.currentPath) this.currentPath = platformRoot;
      return;
    }

    // Activate per-tab session.
    if (activeTab) this.activeDirTabId = activeTab.id;
    this.currentPath = pathToLoad;
    this.clearArchiveState();

    const session = this.dir;
    const cachedOk =
      !this.pathIsInsideArchive(pathToLoad) &&
      session.currentPath === pathToLoad &&
      session.showHiddenLoaded === this.showHidden &&
      (session.entries?.length ?? 0) > 0;

    if (cachedOk) {
      this.applyFilter();
      this.emitPreviewTargetFromState();
      return;
    }

    await this.loadPathTransparently(pathToLoad);
  }

  private clearArchiveState(): void {
    this.archiveStack = [];
    this.fsParentDir  = '';
  }

  private async loadDirectory(path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void> {
    await this.dirStream.cancelActive();
    this.exitArchive();
    this.clearArchiveState();
    this.activeDirStreamPreserveUI = !!opts?.preserveUI;
    this.pendingFocusPath = opts?.focusPath ?? null;
    this.loading = true;
    this.error   = null;

    if (!this.activeDirStreamPreserveUI) {
      this.selectedFiles.clear();
      this.selectionCursorPath = null;
      this.selectionAnchorPath = null;
      this.entries = this.withParentEntry([], false, path);
      this.applyFilter();
    }
    try {
      const streamId = await this.dirStream.start(path, this.showHidden);
      await this.dirStream.waitFor(streamId);
    } catch (error: any) {
      if (error?.message === 'Directory stream cancelled') {
        return;
      }
      this.error = error.message || 'Failed to load directory';
    } finally {
      this.activeDirStreamPreserveUI = false;
      this.pendingFocusPath = null;
      this.loading = false;
    }
  }

  // в”Ђв”Ђ Tab management в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  async onTabChange(tab: TabInfo): Promise<void> {
    const current = this.tabs.find(t => t.id === this.currentTabId);
    const currentKind = (current?.kind ?? 'dir');

    if (currentKind === 'dir' && this.currentTabId && this.currentPath) {
      if (this.isInArchive()) {
        this.workspaceService.saveArchiveState(this.paneId, this.currentTabId, {
          path: this.currentPath, archiveStack: this.archiveStack,
          fsDirPath: this.archiveStack[0]?.fsPath ?? '', fsParentDir: this.fsParentDir,
        });
      } else {
        this.workspaceService.updateTabPath(this.paneId, this.currentTabId, this.currentPath);
      }
    }

    this.workspaceService.switchTab(this.paneId, tab.id);
    this.currentTabId = tab.id;
    this.clearArchiveState();

    // Viewer tab: render surface only; no directory load.
    if ((tab.kind ?? 'dir') === 'viewer') {
      this.tabs = this.workspaceService.getTabs(this.paneId);
      this.activeTabChange.emit(tab);
      return;
    }

    // Switch active session for this dir tab.
    this.activeDirTabId = tab.id;

    const archiveState = this.workspaceService.getArchiveState(this.paneId, tab.id);
    if (archiveState) {
      this.currentPath  = archiveState.path;
      this.archiveStack = archiveState.archiveStack;
      this.fsParentDir  = archiveState.fsParentDir;
      if (this.archiveStack.length > 0) {
        this.renderArchiveFrame(this.archiveStack[this.archiveStack.length - 1]);
      }
      return;
    }

    const restoredPath = tab.path;
    if (restoredPath) this.currentPath = restoredPath;

    const session = this.dir;
    const cachedOk =
      !this.pathIsInsideArchive(restoredPath) &&
      session.currentPath === restoredPath &&
      session.showHiddenLoaded === this.showHidden &&
      (session.entries?.length ?? 0) > 0;

    if (cachedOk) {
      this.applyFilter();
      this.emitPreviewTargetFromState();
      this.activeTabChange.emit(tab);
      return;
    }

    await this.loadPathTransparently(restoredPath);
    this.activeTabChange.emit(tab);
  }

  /** Best-effort: focus/move cursor to a path if it exists in the current listing. */
  focusCursorPath(path: string): void {
    if (this.activeTabKind !== 'dir') return;
    if (!path) return;

    const normalized = normalizePath(path);
    const selectable = this.dirRuntime.getSelectableEntries();
    const idx = selectable.findIndex(e => normalizePath(e.path) === normalized);
    if (idx < 0) return;

    this.selectionCursorPath = selectable[idx].path;
    this.selectionAnchorPath = this.selectionCursorPath;
    // Keep marks intact; only move cursor.
    this.fileViewContainer?.scrollToCursorPath(this.selectionCursorPath);
    this.emitPreviewTargetFromState();
  }

  /**
   * Restore a specific directory tab (by id) and place the cursor on `focusPath`.
   * Used by FileBrowser to restore selection when switching between preview tabs.
   */
  async activateDirTabAndFocusPath(tabId: string, focusPath: string): Promise<void> {
    if (!tabId) return;
    const tabs = this.workspaceService.getTabs(this.paneId) ?? [];
    const tab = tabs.find(t => t.id === tabId);
    if (!tab) return;
    if ((tab.kind ?? 'dir') !== 'dir') return;

    await this.onTabChange(tab);
    this.focusCursorPath(focusPath);
  }

  private async restoreArchiveFromPath(path: string): Promise<void> {
    this.loading = true;
    this.error   = null;
    try {
      // parseArchivePath correctly splits into:
      //   fsPath      вЂ“ the on-disk archive file (e.g. "C:/foo/archive.zip")
      //   nestedChain вЂ“ nested archive segments inside (e.g. ["folder/inner.zip"])
      //   innerPath   вЂ“ regular subdirectory to navigate into (e.g. "subdir")
      const { fsPath, nestedChain, innerPath } = parseArchivePath(path);

      // 1. Open root archive
      let frame = await this.archiveService.openRootArchive(fsPath);
      const stack: ArchiveFrame[] = [frame];

      // 2. Open each nested archive level using cumulative chains
      for (let i = 0; i < nestedChain.length; i++) {
        frame = await this.archiveService.openNestedArchive(frame, nestedChain.slice(0, i + 1));
        stack.push(frame);
      }

      // 3. Navigate into the inner subdirectory (if any) within the deepest archive
      if (innerPath) {
        frame = await this.archiveService.navigateInto(frame, innerPath);
        stack[stack.length - 1] = frame;
      }

      this.archiveStack = stack;
      this.renderArchiveFrame(stack[stack.length - 1]);

      const virtualPath = buildArchiveDisplayPath(stack[0].fsPath, this.archiveStack);
      const title = buildArchiveTabTitle(stack[0].fsPath, this.archiveStack);
      this.workspaceService.updateTabPathAndTitle(this.paneId, this.currentTabId, virtualPath, title);
      this.tabs = this.workspaceService.getTabs(this.paneId);
      // Focus tree to the virtual path (best-effort)
      setTimeout(() => { try { this.fileViewContainer?.treeComponent?.focusNode(this.currentPath); } catch {} }, 0);
    } catch (err: any) {
      this.error = err.message || 'Failed to restore archive tab';
    } finally {
      this.loading = false;
    }
  }

  onTabClose(tab: TabInfo): void {
    this.dirSessions.delete(tab.id);
    if (this.activeDirTabId === tab.id) this.activeDirTabId = null;
    this.workspaceService.clearArchiveState(this.paneId, tab.id);
    this.workspaceService.closeTab(this.paneId, tab.id);
    this.loadPaneState();
  }

  onTabPin(tab: TabInfo): void {
    this.workspaceService.toggleTabPin(this.paneId, tab.id);
    this.loadPaneState();
  }

  onNewTab(): void {
    const newTab = this.workspaceService.addTab(this.paneId, this.currentPath, true);
    if (!newTab) return;
    this.currentTabId = newTab.id;
    this.tabs = this.workspaceService.getTabs(this.paneId);
    this.workspaceService.switchTab(this.paneId, newTab.id);
    this.tabs = this.workspaceService.getTabs(this.paneId);
  }

  private async openPathInNewTab(path: string): Promise<void> {
    const newTab = this.workspaceService.addTab(this.paneId, path, true);
    if (!newTab) return;
    this.tabs = this.workspaceService.getTabs(this.paneId);
    await this.onTabChange(newTab);
  }

  // в”Ђв”Ђ Address bar / path в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onPathChange(path: string): void {
    if (path && path !== this.currentPath) {
      this.currentPath = path;
      this.exitArchive();
      this.clearArchiveState();
      void this.loadPathTransparently(path);
    }
  }

  async onNavigateUp(): Promise<void> {
    if (this.isInArchive()) {
      await this.navigateArchiveUp();
    } else {
      const fromPath = this.currentPath;
      try {
        const listing = await window.go.main.App.NavGoUp(this.paneId);
        await this.loadDirectory(listing.path, { focusPath: fromPath });
      } catch {
        const target = parentPath(normalizePath(fromPath));
        if (target !== fromPath) await this.loadDirectory(target, { focusPath: fromPath });
      }
    }
  }

  onRefresh(): void {
    void this.refreshCurrentPathTransparently();
    // Keep the tree in sync: reload children of the current directory node
    // without collapsing any expanded subtrees.
    this.fileViewContainer?.refreshTreeNode(this.currentPath);
  }

  private bindFsChangeEvents(): void {
    EventsOn('fs:directoryChanged', (event: { paneId?: string; path?: string } | null) => {
      this.handleFsDirectoryChanged(event);
    });

    this.dirStream.bind(this.paneId, {
      onMeta: (event, isFirst) => this.handleDirStreamMeta(event, isFirst),
      onBatch: (event, baseEntries) => this.handleDirStreamBatch(event, baseEntries),
      onDone: (event, baseEntries) => this.handleDirStreamDone(event, baseEntries),
      onError: (event) => this.handleDirStreamError(event),
    });
  }

  private unbindFsChangeEvents(): void {
    if (this.fsChangeDebounceTimer) {
      clearTimeout(this.fsChangeDebounceTimer);
      this.fsChangeDebounceTimer = null;
    }
    void this.dirStream.unbind();
    EventsOff('fs:directoryChanged');
  }

  private handleFsDirectoryChanged(event: { paneId?: string; path?: string } | null): void {
    if (!event || event.paneId !== this.paneId) return;
    if (this.isInArchive()) return;

    const current = (this.currentPath || '').replace(/\\/g, '/');
    const changed = (event.path || '').replace(/\\/g, '/');
    if (!current || !changed || current !== changed) return;

    if (this.fsChangeDebounceTimer) {
      clearTimeout(this.fsChangeDebounceTimer);
    }
    this.fsChangeDebounceTimer = setTimeout(() => {
      this.fsChangeDebounceTimer = null;
      this.onRefresh();
    }, 150);
  }

  private handleDirStreamMeta(event: DirStreamMetaEvent, isFirstMeta: boolean): void {
    this.currentPath = event.path;
    this.fsParentDir = event.path;
    this.dir.showHiddenLoaded = !!event.showHidden;
    this.workspaceService.updateTabPath(this.paneId, this.currentTabId, event.path);
    this.pathHistoryService.addPath(event.path, this.paneId);
    if (isFirstMeta) {
      this.pathChange.emit(event.path);
    }
    if (!this.activeDirStreamPreserveUI) {
      this.entries = this.withParentEntry([], false, event.path);
      this.applyFilter();
    }

    // best-effort pane watch sync
    void window.go?.main?.App?.SetPaneDirectory?.(this.paneId, event.path);
  }

  private handleDirStreamBatch(event: DirStreamBatchEvent, baseEntries: FileInfo[]): void {
    this.entries = this.withParentEntry(baseEntries, false, this.currentPath || event.path);
    this.applyFilter();
  }

  private handleDirStreamDone(event: DirStreamDoneEvent, baseEntries: FileInfo[]): void {
    this.entries = this.withParentEntry(baseEntries, false, event.path || this.currentPath);
    // Prune selection to keep it stable across refresh.
    if (this.selectedFiles.size > 0) {
      const present = new Set(this.entries.map(e => e.path));
      for (const p of Array.from(this.selectedFiles)) {
        if (!present.has(p)) this.selectedFiles.delete(p);
      }
      this.selectedFiles = new Set(this.selectedFiles);
    }
    this.applyFilter();

    // TC: after navigating up, select the folder we came from; otherwise place cursor on first entry.
    if (this.pendingFocusPath) {
      const focusNorm = normalizePath(this.pendingFocusPath);
      const target = this.filteredEntries.find(e => normalizePath(e.path) === focusNorm);
      if (target) {
        this.selectionCursorPath = target.path;
        this.selectionAnchorPath = target.path;
        this.selectedFiles = new Set([target.path]);
        setTimeout(() => this.fileViewContainer?.scrollToCursorPath(target.path), 0);
      }
      this.pendingFocusPath = null;
    } else if (!this.selectionCursorPath) {
      const firstReal = this.filteredEntries.find(e => e.path !== PARENT_ENTRY_PATH);
      if (firstReal) {
        this.selectionCursorPath = firstReal.path;
        this.selectionAnchorPath = firstReal.path;
      }
    }

    this.persistDirTabState();

    setTimeout(() => {
      try { this.fileViewContainer?.treeComponent?.focusNode(event.path); } catch { /* best-effort */ }
    }, 0);
  }

  private handleDirStreamError(event: DirStreamErrorEvent): void {
    this.error = event.error || 'Failed to load directory';
  }

  /**
   * Returns the effective destination directory for a copy/move operation.
   * If exactly one directory is selected in this pane, operations are directed
   * into that folder rather than the generic currentPath.
   */
  getDestinationDir(): string {
    if (this.selectedFiles.size !== 1) return this.currentPath;
    const [selectedPath] = this.selectedFiles;

    // List / grid / details mode: check entries directly.
    const entry = this.entries.find(e => e.path === selectedPath);
    if (entry?.type === FileType.DIRECTORY) return selectedPath;

    // Tree mode: check visible tree nodes.
    if (this.currentView === 'tree') {
      const nodes = this.fileViewContainer?.treeComponent?.getVisibleNodes() ?? [];
      const node = nodes.find((n: any) => n.path === selectedPath);
      if (node?.type === FileType.DIRECTORY) return selectedPath;
    }

    return this.currentPath;
  }

  // в”Ђв”Ђ Archive segment click в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  async onArchiveSegmentClick(virtualPath: string): Promise<void> {
    // Not currently in an archive вЂ” open it fresh (e.g. typed in address bar, or pin click)
    if (!this.isInArchive()) {
      this.currentPath = virtualPath;
      this.clearArchiveState();
      await this.restoreArchiveFromPath(virtualPath);
      return;
    }

    const fsNorm   = (this.archiveStack[0]?.fsPath ?? '').replace(/\\/g, '/');
    const vNorm    = virtualPath.replace(/\\/g, '/');
    const afterFs  = vNorm.startsWith(fsNorm) ? vNorm.slice(fsNorm.length).replace(/^\//, '') : '';

    if (!afterFs) {
      const rootFrame = this.archiveStack[0];
      this.loading = true;
      try {
        const frame = await this.archiveService.navigateInto(rootFrame, '');
        this.archiveStack = [{ ...frame, innerPath: '' }];
        this.renderArchiveFrame({ ...frame, innerPath: '' });
      } catch (err: any) { this.error = err.message; }
      finally { this.loading = false; }
      return;
    }

    const parts = afterFs.split('/').filter(Boolean);
    this.loading = true;
    try {
      let frame = await this.archiveService.navigateInto(this.archiveStack[0], '');
      let stack: ArchiveFrame[] = [{ ...frame, innerPath: '' }];

      for (const part of parts) {
        const isNested = frame.listing.entries.some(
          e => e.name === part && (e.type === FileType.ARCHIVE || isArchive(e.name))
        );
        if (isNested) {
          const nestedChain = [...frame.nestedChain, part];
          frame = await this.archiveService.openNestedArchive(frame, nestedChain);
          stack.push(frame);
        } else {
          const innerPath = frame.innerPath ? `${frame.innerPath}/${part}` : part;
          frame = await this.archiveService.navigateInto(stack[stack.length - 1], innerPath);
          stack = [...stack.slice(0, -1), frame];
        }
      }

      this.archiveStack = stack;
      this.renderArchiveFrame(stack[stack.length - 1]);
    } catch (err: any) {
      this.error = err.message || 'Failed to navigate to archive segment';
    } finally {
      this.loading = false;
    }
  }

  // в”Ђв”Ђ Navigation в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onNavigate(entry: FileInfo): void {
    if (entry.path === PARENT_ENTRY_PATH) { this.onNavigateUp(); return; }

    // Tree navigation: real FS directory must exit archive mode first so the
    // tree root is never overwritten with a virtual archive path.
    if (this.currentView === 'tree' && entry.type === FileType.DIRECTORY) {
      if (this.isInArchive()) {
        this.exitArchive();
        this.clearArchiveState();
      }
      this.currentPath = entry.path;
      this.pathChange.emit(entry.path);
      return;
    }
    // For file/file-symlink/archive nodes in tree view: update the address bar
    // to the parent directory without opening the item (opening is handled via
    // the nodeDoubleClick path in onNodeDoubleClick).
    // Dir-symlinks have type=DIRECTORY and are handled by the block above.
    if (this.currentView === 'tree' && (entry.type === FileType.FILE || entry.type === FileType.SYMLINK || entry.type === FileType.ARCHIVE)) {
      const parent = parentPath(entry.path);
      this.currentPath = parent;
      this.pathChange.emit(parent);
      return;
    }

    if (this.isInArchive()) {
      if (entry.type === FileType.DIRECTORY) { this.archiveNavigateInto(entry.path); }
      else if (entry.type === FileType.ARCHIVE) {
        const chain: string[] = (entry as any).nestedChain;
        if (chain?.length) this.openNestedArchive(chain);
      }
      return;
    }

    if (entry.type === FileType.DIRECTORY) {
      this.loadDirectory(entry.path);
      return;
    }
    if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
      this.openRootArchive(entry.path);
      return;
    }
    // .lnk Shell Links (Windows) and unresolved Unix file-symlinks
    if (entry.name.toLowerCase().endsWith('.lnk') || entry.type === FileType.SYMLINK) {
      this.resolveAndOpen(entry.path);
      return;
    }
    // Regular file вЂ” open with default OS application
    this.apiService.openFile(entry.path).catch(err =>
      console.error('Failed to open file:', err)
    );
  }

  onOpenSelectedItem(): void {
    const effective = this.getEffectiveSelection();
    const selected = Array.from(effective);
    if (selected.length === 1) {
      const entry = this.entries.find(e => e.path === selected[0]);
      if (entry) this.onNavigate(entry);
    }
  }

  async openQuickView(): Promise<void> {
    if (this.activeTabKind !== 'dir') return;

    const effective = this.getEffectiveSelection();
    const selected = Array.from(effective).filter(p => p && p !== PARENT_ENTRY_PATH);
    if (selected.length !== 1) return;

    const path = selected[0];
    try {
      if (this.isInArchive()) {
        // Build archive context вЂ” viewer extracts entry bytes into memory via ArchiveReadFile.
        const frame = this.archiveStack[this.archiveStack.length - 1];
        const entry = this.entries.find(e => e.path === path);
        if (!entry || entry.type === FileType.DIRECTORY) return;

        const archiveContext: ViewerArchiveContext = {
          fsPath:      this.archiveStack[0].fsPath,
          nestedChain: frame.nestedChain ?? [],
          entryPath:   entry.path,
        };
        const tab = this.workspaceService.addViewerTab(this.paneId, path, {
          activate: true, closable: true, archiveContext,
        });
        if (!tab) return;
        this.tabs = this.workspaceService.getTabs(this.paneId);
        await this.onTabChange(tab);
        return;
      }

      const isDir = await this.apiService.isDirectory(path);
      if (isDir) return;

      const tab = this.workspaceService.addViewerTab(this.paneId, path, {
        activate: true,
        closable: true,
      });
      if (!tab) return;

      this.tabs = this.workspaceService.getTabs(this.paneId);
      await this.onTabChange(tab);
    } catch (err: any) {
      this.confirmDialog.alert(`Preview Failed\n${err?.message || String(err)}`);
    }
  }

  /** Open a new, closable viewer tab for the given filesystem path. */
  async openFrozenPreviewTab(
    targetPath: string,
    opts?: { sourcePane: 'left' | 'right'; sourceTabId: string; sourceDir: string; autoFollow?: boolean },
  ): Promise<void> {
    const path = (targetPath || '').trim();
    if (!path) return;

    try {
      if (this.isInArchive()) {
        // Same as openQuickView вЂ” extract via ArchiveReadFile, no disk writes.
        const frame = this.archiveStack[this.archiveStack.length - 1];
        const entry = this.entries.find(e => e.path === path);
        if (!entry || entry.type === FileType.DIRECTORY) return;

        const archiveContext: ViewerArchiveContext = {
          fsPath:      this.archiveStack[0].fsPath,
          nestedChain: frame.nestedChain ?? [],
          entryPath:   entry.path,
        };
        const tab = this.workspaceService.addViewerTab(this.paneId, path, {
          activate: true, closable: true, archiveContext,
        });
        if (!tab) return;
        this.tabs = this.workspaceService.getTabs(this.paneId);
        await this.onTabChange(tab);
        return;
      }

      const isDir = await this.apiService.isDirectory(path);
      if (isDir) return;

      const tab = this.workspaceService.addViewerTab(this.paneId, path, {
        activate: true,
        closable: true,
        autoFollow: !!opts?.autoFollow,
        sourcePane: opts?.sourcePane,
        sourceTabId: opts?.sourceTabId,
        sourceDir: opts?.sourceDir,
      });
      if (!tab) return;

      this.tabs = this.workspaceService.getTabs(this.paneId);
      await this.onTabChange(tab);
    } catch (err: any) {
      this.confirmDialog.alert(`Preview Failed\n${err?.message || String(err)}`);
    }
  }

  async onOpenInNewTab(entry: FileInfo): Promise<void> {
    if (entry.path === PARENT_ENTRY_PATH) {
      const up = this.isInArchive() ? this.currentPath : parentPath(this.currentPath);
      await this.openPathInNewTab(up);
      return;
    }

    if (entry.type === FileType.DIRECTORY) {
      await this.openPathInNewTab(entry.path);
      return;
    }

    if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
      await this.openPathInNewTab(entry.path);
    }
  }

  async onRenameSelected(): Promise<void> {
    const selected = Array.from(this.selectedFiles);
    if (selected.length !== 1) return;
    const entry = this.entries.find(e => e.path === selected[0]);
    if (!entry || entry.path === PARENT_ENTRY_PATH) return;
    if (this.isInArchive()) {
      this.confirmDialog.alert('Archive Limitation\nRename is not supported inside archives.');
      return;
    }
    this.fileViewContainer?.startRenameSelected();
  }

  async onRename(event: { entry: FileInfo; newName: string }): Promise<void> {
    await this.renameEntry(event.entry, event.newName);
  }

  async onDragDrop(event: { event: DragEvent; source: FileInfo[]; targetPath: string }): Promise<void> {
    if (!event.source?.length || !event.targetPath) return;

    if (this.isInArchive()) {
      this.confirmDialog.alert('Archive Drag & Drop Not Supported\nUse copy/move to other pane instead.');
      return;
    }

    const sourcePaths = event.source
      .map(e => e.path)
      .filter(p => p && p !== PARENT_ENTRY_PATH);

    if (!sourcePaths.length) return;

    try {
      const isCopy = event.event.ctrlKey || event.event.metaKey;
      const sourcePaneIdRaw = event.event.dataTransfer?.getData('application/x-wails-pane-id') || '';
      const sourcePaneId = (sourcePaneIdRaw === 'left' || sourcePaneIdRaw === 'right') ? sourcePaneIdRaw : null;
      const isCrossPane = !!sourcePaneId && sourcePaneId !== this.paneId;

      const items = await Promise.all(sourcePaths.map(async sourcePath => ({
        source: sourcePath,
        destination: await this.buildDestinationPath(sourcePath, event.targetPath),
      })));

      if (!isCopy) {
        const moveItems = items.filter(i => normalizePath(i.source) !== normalizePath(i.destination));
        if (moveItems.length === 0) return;

        // Cross-pane move UX: hide in source pane BEFORE starting the move.
        if (isCrossPane) {
          try {
            window.dispatchEvent(new CustomEvent('pane:dnd-optimistic-hide', {
              detail: { paneId: sourcePaneId, paths: moveItems.map(i => i.source) }
            }));
          } catch {
            // ignore
          }
        }

        await this.apiService.batchMove(moveItems, true);
        return;
      }

      await this.apiService.batchCopy(items, true);
    } catch (error: any) {
      // If we optimistically hid items (cross-pane move), restore them on immediate failure.
      try {
        const isCopy = event.event.ctrlKey || event.event.metaKey;
        const sourcePaneIdRaw = event.event.dataTransfer?.getData('application/x-wails-pane-id') || '';
        const sourcePaneId = (sourcePaneIdRaw === 'left' || sourcePaneIdRaw === 'right') ? sourcePaneIdRaw : null;
        const isCrossPane = !!sourcePaneId && sourcePaneId !== this.paneId;
        if (!isCopy && isCrossPane) {
          window.dispatchEvent(new CustomEvent('pane:dnd-optimistic-restore', {
            detail: { paneId: sourcePaneId, paths: sourcePaths }
          }));
        }
      } catch {
        // ignore
      }
      this.confirmDialog.alert(`Drag & Drop Failed\n${error.message}`);
    }
  }

  // в”Ђв”Ђ Archive navigation в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  isInArchive(): boolean { return this.archiveStack?.length > 0; }

  async openRootArchive(fsPath: string): Promise<void> {
    this.loading = true;
    this.error   = null;
    const exitPath = this.currentPath;  // в†ђ capture parent dir before entering archive
    try {
      const frame       = await this.archiveService.openRootArchive(fsPath);
      this.archiveStack = [frame];
      this.fsParentDir  = exitPath;     // в†ђ use captured value
      this.renderArchiveFrame(frame);
    } catch (err: any) {
      this.error = err.message || 'Failed to open archive';
    } finally {
      this.loading = false;
    }
  }

  private async archiveNavigateInto(folderInnerPath: string): Promise<void> {
    const currentFrame = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error   = null;
    try {
      const newFrame    = await this.archiveService.navigateInto(currentFrame, folderInnerPath);
      this.archiveStack = [...this.archiveStack.slice(0, -1), newFrame];
      this.renderArchiveFrame(newFrame);
    } catch (err: any) {
      this.error = err.message || 'Failed to navigate into folder';
    } finally {
      this.loading = false;
    }
  }

  private async openNestedArchive(nestedChain: string[]): Promise<void> {
    const currentFrame = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error   = null;
    try {
      const newFrame    = await this.archiveService.openNestedArchive(currentFrame, nestedChain);
      this.archiveStack = [...this.archiveStack, newFrame];
      this.renderArchiveFrame(newFrame);
    } catch (err: any) {
      this.error = err.message || 'Failed to open nested archive';
    } finally {
      this.loading = false;
    }
  }

  async navigateArchiveUp(): Promise<void> {
    const currentFrame = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error = null;
    try {
      const parentFrame = await this.archiveService.navigateUp(currentFrame);
      if (parentFrame !== null) {
        // Still inside the archive вЂ” navigate to parent directory within it.
        this.archiveStack = [...this.archiveStack.slice(0, -1), parentFrame];
        this.renderArchiveFrame(parentFrame);
        return;
      }

      const poppedStack = this.archiveStack.slice(0, -1);
      if (poppedStack.length === 0) {
        // Exiting to the real filesystem.
        // Capture both paths before clearArchiveState() zeroes them.
        const exitPath    = this.fsParentDir || this.currentPath;
        const archivePath = this.archiveStack[0]?.fsPath ?? '';
        this.clearArchiveState();
        // Pass the archive file's path as focusPath so it is selected after load.
        await this.loadDirectory(exitPath, { focusPath: archivePath });
        return;
      }

      // Exiting a nested archive вЂ” pop back to the parent archive frame.
      this.archiveStack = poppedStack;
      this.renderArchiveFrame(poppedStack[poppedStack.length - 1]);
    } catch (err: any) {
      this.error = err.message || 'Failed to navigate up';
    } finally {
      this.loading = false;
    }
  }

  exitArchive(): void {
    if (this.isInArchive() && this.currentTabId) {
      this.workspaceService.saveArchiveState(this.paneId, this.currentTabId, {
        path: this.currentPath, archiveStack: this.archiveStack,
        fsDirPath: this.archiveStack[0]?.fsPath ?? '', fsParentDir: this.fsParentDir,
      });
    }
    this.archiveStack = [];
  }

  private async refreshArchive(): Promise<void> {
    const frame  = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error   = null;
    try {
      const refreshed   = await this.archiveService.refreshFrame(frame);
      this.archiveStack = [...this.archiveStack.slice(0, -1), refreshed];
      this.renderArchiveFrame(refreshed);
    } catch (err: any) {
      this.error = err.message || 'Failed to refresh archive';
    } finally {
      this.loading = false;
    }
  }

  private renderArchiveFrame(frame: ArchiveFrame): void {
    const rawEntries  = this.archiveService.listingToFileInfos(frame.listing);
    this.entries      = this.withParentEntry(rawEntries, true, '');
    this.selectedFiles.clear();
    this.applyFilter();

    const fsPath      = this.archiveStack[0]?.fsPath ?? '';
    const virtualPath = buildArchiveDisplayPath(fsPath, this.archiveStack);
    this.currentPath  = virtualPath;

    const title = buildArchiveTabTitle(fsPath, this.archiveStack);
    this.workspaceService.updateTabPathAndTitle(this.paneId, this.currentTabId, virtualPath, title);
    this.tabs = this.workspaceService.getTabs(this.paneId);
  }

  // в”Ђв”Ђ Find widget в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onFindStateChange(state: FindWidgetState | null): void {
    if (!state) {
      this.filterQuery = { text: '', criteria: [] };
    } else {
      // Build a FilterQuery from the find widget state.
      // matchCase / wholeWord / useRegex are passed as criteria for FilterService.
      this.filterQuery = this.filterService.parseFilterText(state.text);
      // Patch in the extra options so FilterService can honour them if supported.
      (this.filterQuery as any).matchCase  = state.matchCase;
      (this.filterQuery as any).wholeWord  = state.wholeWord;
      (this.filterQuery as any).useRegex   = state.useRegex;
    }
    this.applyFilter();

    // Update match count badge on the widget
    if (this.findWidget) {
      this.findWidget.setMatchCount(this.filteredEntries.length);
    }

    this.persistDirTabState();
  }

  onFindNext(): void { /* file-list scroll-to-next can be wired here */ }
  onFindPrev(): void { /* file-list scroll-to-prev can be wired here */ }

  // в”Ђв”Ђ Filtering в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  applyFilter(): void {
    // Prune any optimistic hides that are no longer present in the current listing.
    if (this.dir.optimisticHiddenPaths.size > 0) {
      const present = new Set(this.entries.map(e => e.path));
      for (const p of Array.from(this.dir.optimisticHiddenPaths)) {
        if (!present.has(p)) this.dir.optimisticHiddenPaths.delete(p);
      }
    }

    const baseEntries = this.dir.optimisticHiddenPaths.size > 0
      ? this.entries.filter(e => e.path === PARENT_ENTRY_PATH || !this.dir.optimisticHiddenPaths.has(e.path))
      : this.entries;

    const q = this.filterQuery;
    if (!q?.text?.trim() && !q?.criteria?.length) {
      this.filteredEntries = baseEntries;
      return;
    }
    // Always keep the ".." entry visible regardless of the active filter.
    const parentEntry = baseEntries.filter(e => e.path === PARENT_ENTRY_PATH);
    const filtered    = this.filterService.filterEntries(
      baseEntries.filter(e => e.path !== PARENT_ENTRY_PATH),
      q
    );
    this.filteredEntries = [...parentEntry, ...filtered];
  }

  private persistDirTabState(): void {
    // DirSurfaceSession is the source of truth now; kept for compatibility with existing call sites.
  }

  // в”Ђв”Ђ Selection в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onSelectionChange(entries: FileInfo[]): void {
    const paths = entries.map(e => e.path).filter(p => p !== PARENT_ENTRY_PATH);
    this.selectedFiles = new Set(paths);
    if (paths.length > 0) {
      this.selectionCursorPath = paths[paths.length - 1];
      if (paths.length === 1) {
        this.selectionAnchorPath = this.selectionCursorPath;
      } else if (!this.selectionAnchorPath) {
        this.selectionAnchorPath = this.selectionCursorPath;
      }
    }
    // paths.length === 0: marks cleared but cursor stays where it was (TC style)
    this.emitPreviewTargetFromState();
    this.persistDirTabState();
  }

  /** Returns a filesystem path suitable for preview, or empty string when none. */
  getPreviewTargetPath(): string {
    if (this.isInArchive()) return '';

    const effective = this.getEffectiveSelection();
    const selected = Array.from(effective).filter(p => p && p !== PARENT_ENTRY_PATH);
    if (selected.length !== 1) return '';

    const entry = this.entries.find(e => e.path === selected[0]);
    if (!entry) return '';
    if (entry.type !== FileType.FILE && entry.type !== FileType.SYMLINK) return '';
    return entry.path;
  }

  emitPreviewTargetFromState(): void {
    const next = this.getPreviewTargetPath();
    if (this.dir.lastPreviewTargetEmitted === next) return;
    this.dir.lastPreviewTargetEmitted = next;
    this.previewTargetChange.emit(next);
  }

  // DirTabRuntime adapter methods for DirTabStateController
  getEntriesPaths(): string[] {
    return (this.entries ?? []).map(e => e.path);
  }

  getSelectableEntriesPaths(): string[] {
    return this.dirRuntime.getSelectableEntries().map(e => e.path);
  }

  scrollToCursorPath(path: string): void {
    if (!path) return;
    this.fileViewContainer?.scrollToCursorPath(path);
  }

  onSelectAll(): void {
    const selectable = this.dirRuntime.getSelectableEntries();
    this.selectedFiles = new Set(selectable.map(e => e.path));
    this.selectionCursorPath = selectable.length ? selectable[selectable.length - 1].path : null;
    this.selectionAnchorPath = this.selectionCursorPath;
  }

  onDeselectAll(): void {
    this.selectedFiles = new Set();
    // TC: preserve cursor position when clearing marks
  }

  /**
   * TC-style effective selection: use marked files when any exist,
   * otherwise treat the cursor item as the implicit selection.
   */
  private getEffectiveSelection(): Set<string> {
    if (this.selectedFiles.size > 0) return this.selectedFiles;
    if (this.selectionCursorPath) return new Set([this.selectionCursorPath]);
    return new Set();
  }

  onToggleCurrentAndMoveDown(): void {
    const selectable = this.dirRuntime.getSelectableEntries();
    const index = this.getCursorIndex(selectable);
    if (index < 0) return;

    const path = selectable[index].path;
    const next = new Set(this.selectedFiles);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    this.selectedFiles = next;

    const nextIndex = Math.min(index + 1, selectable.length - 1);
    this.selectionCursorPath = selectable[nextIndex]?.path ?? path;
    this.selectionAnchorPath = this.selectionAnchorPath ?? path;
    this.emitPreviewTargetFromState();
  }

  onSelectCurrentAndMoveDown(): void {
    const selectable = this.dirRuntime.getSelectableEntries();
    const index = this.getCursorIndex(selectable);
    if (index < 0) return;

    const path = selectable[index].path;
    const next = new Set(this.selectedFiles);
    next.add(path);
    this.selectedFiles = next;

    const nextIndex = Math.min(index + 1, selectable.length - 1);
    this.selectionCursorPath = selectable[nextIndex]?.path ?? path;
    this.selectionAnchorPath = path;
    this.emitPreviewTargetFromState();
  }

  /** TC Space: mark/unmark item under cursor and advance cursor down. */
  onMarkCurrentItem(): void {
    if (!this.selectionCursorPath) return;
    const path = this.selectionCursorPath;
    const next = new Set(this.selectedFiles);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    this.selectedFiles = next;

    // Advance cursor down (TC Space key advances after marking)
    const selectable = this.dirRuntime.getSelectableEntries();
    const idx = this.getCursorIndex(selectable);
    if (idx >= 0 && idx < selectable.length - 1) {
      this.selectionCursorPath = selectable[idx + 1].path;
      this.fileViewContainer?.scrollToCursorPath(this.selectionCursorPath);
    }

    this.emitPreviewTargetFromState();
  }

  onExtendSelection(delta: -1 | 1): void {
    const selectable = this.dirRuntime.getSelectableEntries();
    const index = this.getCursorIndex(selectable);
    if (index < 0) return;

    const targetIndex = Math.max(0, Math.min(selectable.length - 1, index + delta));
    const anchorPath = this.selectionAnchorPath ?? selectable[index].path;
    const anchorIndex = selectable.findIndex(e => e.path === anchorPath);
    const start = Math.min(anchorIndex === -1 ? index : anchorIndex, targetIndex);
    const end = Math.max(anchorIndex === -1 ? index : anchorIndex, targetIndex);

    this.selectedFiles = new Set(selectable.slice(start, end + 1).map(e => e.path));
    this.selectionCursorPath = selectable[targetIndex].path;
    this.selectionAnchorPath = anchorPath;
    this.emitPreviewTargetFromState();
  }

  // в”Ђв”Ђ Cursor movement в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  private moveCursorTo(idx: number): void {
    const selectable = this.dirRuntime.getSelectableEntries();
    if (!selectable.length) return;
    const clamped = Math.max(0, Math.min(selectable.length - 1, idx));
    this.selectionCursorPath = selectable[clamped].path;
    this.selectionAnchorPath = this.selectionCursorPath;
    // TC: cursor moves independently of marks вЂ” do NOT touch selectedFiles
    this.fileViewContainer?.scrollToCursorPath(this.selectionCursorPath);
    this.emitPreviewTargetFromState();
    this.persistDirTabState();
  }

  onMoveCursorUp(): void {
    const cols = this.fileViewContainer?.getGridColumns() ?? 1;
    const selectable = this.dirRuntime.getSelectableEntries();
    this.moveCursorTo(this.getCursorIndex(selectable) - cols);
  }

  onMoveCursorDown(): void {
    const cols = this.fileViewContainer?.getGridColumns() ?? 1;
    const selectable = this.dirRuntime.getSelectableEntries();
    this.moveCursorTo(this.getCursorIndex(selectable) + cols);
  }

  onMoveCursorLeft(): void {
    if (this.currentView !== 'grid') return;
    const selectable = this.dirRuntime.getSelectableEntries();
    this.moveCursorTo(this.getCursorIndex(selectable) - 1);
  }

  onMoveCursorRight(): void {
    if (this.currentView !== 'grid') return;
    const selectable = this.dirRuntime.getSelectableEntries();
    this.moveCursorTo(this.getCursorIndex(selectable) + 1);
  }

  onMoveCursorHome(): void {
    this.moveCursorTo(0);
  }

  onMoveCursorEnd(): void {
    const selectable = this.dirRuntime.getSelectableEntries();
    this.moveCursorTo(selectable.length - 1);
  }

  onMoveCursorPageUp(): void {
    const pageSize = this.fileViewContainer?.getApproxPageSize() ?? 10;
    const selectable = this.dirRuntime.getSelectableEntries();
    this.moveCursorTo(this.getCursorIndex(selectable) - pageSize);
  }

  onMoveCursorPageDown(): void {
    const pageSize = this.fileViewContainer?.getApproxPageSize() ?? 10;
    const selectable = this.dirRuntime.getSelectableEntries();
    this.moveCursorTo(this.getCursorIndex(selectable) + pageSize);
  }

  onSelectByMask(): void {
    const mask = prompt('Select by mask (example: *.txt):', '*.*')?.trim();
    if (!mask) return;

    const matcher = this.createMaskMatcher(mask);
    const next = new Set(this.selectedFiles);
    for (const entry of this.dirRuntime.getSelectableEntries()) {
      if (matcher(entry.name)) next.add(entry.path);
    }
    this.selectedFiles = next;
    this.syncCursorFromSelection();
  }

  onUnselectByMask(): void {
    const mask = prompt('Unselect by mask (example: *.txt):', '*.*')?.trim();
    if (!mask) return;

    const matcher = this.createMaskMatcher(mask);
    const next = new Set(this.selectedFiles);
    for (const entry of this.dirRuntime.getSelectableEntries()) {
      if (matcher(entry.name)) next.delete(entry.path);
    }
    this.selectedFiles = next;
    this.syncCursorFromSelection();
  }

  onInvertSelection(): void {
    const selectable = this.dirRuntime.getSelectableEntries();
    const next = new Set<string>();
    for (const entry of selectable) {
      if (!this.selectedFiles.has(entry.path)) {
        next.add(entry.path);
      }
    }
    this.selectedFiles = next;
    this.syncCursorFromSelection();
  }

  onSelectSameExtension(): void {
    const current = this.getCurrentCursorEntry();
    if (!current) return;

    const extension = this.getExtension(current.name);
    if (!extension) return;

    const next = new Set(this.selectedFiles);
    for (const entry of this.dirRuntime.getSelectableEntries()) {
      if (this.getExtension(entry.name) === extension) {
        next.add(entry.path);
      }
    }
    this.selectedFiles = next;
    this.syncCursorFromSelection();
  }

  private syncCursorFromSelection(): void {
    const selectable = this.dirRuntime.getSelectableEntries();
    if (!selectable.length || this.selectedFiles.size === 0) {
      this.selectionCursorPath = selectable.length ? selectable[0].path : null;
      this.selectionAnchorPath = this.selectionCursorPath;
      this.emitPreviewTargetFromState();
      return;
    }

    const firstSelected = selectable.find(e => this.selectedFiles.has(e.path));
    if (firstSelected) {
      this.selectionCursorPath = firstSelected.path;
      this.selectionAnchorPath = this.selectionAnchorPath ?? firstSelected.path;
    }
    this.emitPreviewTargetFromState();
  }

  private getCursorIndex(entries: FileInfo[]): number {
    if (!entries.length) return -1;

    if (this.selectionCursorPath) {
      const idx = entries.findIndex(e => e.path === this.selectionCursorPath);
      if (idx !== -1) return idx;
    }

    const firstSelected = entries.findIndex(e => this.selectedFiles.has(e.path));
    if (firstSelected !== -1) {
      this.selectionCursorPath = entries[firstSelected].path;
      return firstSelected;
    }

    this.selectionCursorPath = entries[0].path;
    return 0;
  }

  private getCurrentCursorEntry(): FileInfo | null {
    const entries = this.dirRuntime.getSelectableEntries();
    const idx = this.getCursorIndex(entries);
    return idx >= 0 ? entries[idx] : null;
  }

  private createMaskMatcher(mask: string): (value: string) => boolean {
    const escaped = mask
      .replace(/[.+^${}()|[\]\\]/g, '\\$&')
      .replace(/\*/g, '.*')
      .replace(/\?/g, '.');
    const regex = new RegExp(`^${escaped}$`, 'i');
    return (value: string) => regex.test(value);
  }

  private getExtension(name: string): string {
    const idx = name.lastIndexOf('.');
    if (idx <= 0 || idx === name.length - 1) return '';
    return name.slice(idx + 1).toLowerCase();
  }

  // в”Ђв”Ђ File operations в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  async onCreateFolder(): Promise<void> {
    if (this.isInArchive()) { this.confirmDialog.alert('Archive Limitation\nCannot create folders inside an archive.'); return; }
    const folderName = prompt('Enter folder name:');
    if (!folderName) return;
    try {
      const newPath: string = await window.go.main.App.JoinPaths([this.currentPath, folderName]);
      await this.apiService.createDirectory(newPath, false);
      await this.loadDirectory(this.currentPath);
    } catch (error: any) { this.confirmDialog.alert(`Create Folder Failed\n${error.message}`); }
  }

  async onDeleteSelected(): Promise<void> {
    const effective = this.getEffectiveSelection();
    if (effective.size === 0) return;

    if (this.isInArchive()) {
      const frame   = this.archiveStack[this.archiveStack.length - 1];
      const entries = Array.from(effective);

      if (frame.nestedChain.length > 0) {
        this.confirmDialog.alert(
          'Nested Archive Limitation\n' +
          'Exit back to the top-level archive first.',
        );
        return;
      }

      const archiveDeleteConfirmed = await this.confirmDialog.confirm({
        title: 'Delete from Archive',
        message:
          `Delete ${entries.length} item(s) from the archive?\n` +
          `This rewrites the archive file on disk and cannot be undone.`,
        confirmText: 'Delete',
        danger: true,
      });
      if (!archiveDeleteConfirmed) return;

      try {
        const result = await this.archiveService.delete(frame, entries);
        if (result.failed > 0) {
          this.confirmDialog.alert('Delete Failed\n' + (result.errors ?? []).map((e: any) => e.error).join('\n'));
        }
      } catch (err: any) {
        this.confirmDialog.alert(`Delete Error\n${err.message}`);
        return;
      }
      // Refresh the archive listing so deleted entries disappear
      await this.refreshArchive();
      return;
    }

    // в”Ђв”Ђ Filesystem delete в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
    const deleteConfirmed = await this.confirmDialog.confirm({
      title: 'Delete Items',
      message: `Delete ${effective.size} item(s)?`,
      confirmText: 'Delete',
      danger: true,
    });
    if (!deleteConfirmed) return;
    try {
      const result = await this.apiService.batchDelete(Array.from(effective), true);
      if (result.failed > 0) {
        const msg = (result.errors ?? []).map((e: any) => `${e.source}: ${e.error}`).join('\n');
        this.confirmDialog.alert(`Delete Completed With Errors\n${result.failed} item(s) failed:\n${msg}`);
      }
      await this.loadDirectory(this.currentPath);
    } catch (error: any) {
      this.confirmDialog.alert(`Delete Failed\n${error.message}`);
    }
  }

  /**
   * Returns archive metadata for the current tree-view selection when the
   * selected nodes are inside an expanded archive node (not in archive-pane
   * mode). All selected nodes must share the same archive FS path and nested
   * chain; otherwise returns null.
   */
  getTreeArchiveSelectionContext(): { archivePath: string; innerPaths: string[] } | null {
    if (this.currentView !== 'tree' || this.selectedFiles.size === 0) return null;

    const tree = this.fileViewContainer?.treeComponent;
    if (!tree) return null;

    const visible = tree.getVisibleNodes();
    const selected = visible.filter((n: any) => this.selectedFiles.has(n.path));
    if (!selected.length) return null;

    const firstArchivePath = (selected[0] as any).archivePath as string | undefined;
    if (!firstArchivePath) return null;

    // Derive the archive context path (fsPath + nestedChain, no innerPath).
    const { fsPath, nestedChain } = parseArchivePath(firstArchivePath);
    const contextPath = buildArchivePath(fsPath, nestedChain, '');

    // All selected must come from the same archive context.
    if (!selected.every((n: any) => {
      const nPath = (n as any).archivePath as string | undefined;
      if (!nPath) return false;
      const { fsPath: nFs, nestedChain: nChain } = parseArchivePath(nPath);
      return buildArchivePath(nFs, nChain, '') === contextPath;
    })) return null;

    // Inner paths: each selected file's path within its (nested) archive.
    const innerPaths = selected.map((n: any) => {
      const nPath = (n as any).archivePath as string | undefined;
      return nPath ? (parseArchivePath(nPath).innerPath || n.name) : n.name;
    }).filter(Boolean);

    return innerPaths.length ? { archivePath: contextPath, innerPaths } : null;
  }

  onCopyToOther(): void { 
    if (this.selectedFiles.size > 0) {
      this.copyToOther.emit(Array.from(this.selectedFiles));
    }
  }

  private async renameEntry(entry: FileInfo, forcedName?: string): Promise<void> {
    if (this.isInArchive()) {
      this.confirmDialog.alert('Archive Limitation\nRename is not supported inside archives.');
      return;
    }

    if (entry.path === PARENT_ENTRY_PATH) return;

    const nextName = (forcedName ?? '').trim();
    if (!nextName || nextName === entry.name) return;

    try {
      const destination = await this.apiService.joinPaths([parentPath(entry.path), nextName]);
      await this.apiService.batchMove([{ source: entry.path, destination }], true);
    } catch (error: any) {
      this.confirmDialog.alert(`Rename Failed\n${error.message}`);
    }
  }

  private async buildDestinationPath(sourcePath: string, targetDir: string): Promise<string> {
    const split = await this.apiService.splitPath(sourcePath);
    const base = split.base || sourcePath.replace(/\\/g, '/').split('/').filter(Boolean).pop() || '';
    return await this.apiService.joinPaths([targetDir, base]);
  }
  
  onMoveToOther(): void { 
    if (this.selectedFiles.size > 0) {
      this.moveToOther.emit(Array.from(this.selectedFiles));
    }
  }

  // в”Ђв”Ђ Archive move operations в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  /**
   * Called by FileBrowserComponent after it has resolved destDir.
   * Shows the confirmation dialog; executePendingMove runs when user confirms.
   */
  initiatePendingMove(entries: string[], destDir: string): void {
    this.archivePendingMoveEntries = entries;
    this.archivePendingMoveDestDir = destDir;
    this.archiveMoveWarning        = true;
  }

  /**
   * Bound to (confirmed) on the ArchiveMoveWarningDialogComponent.
   */
  async executePendingMove(): Promise<void> {
    this.archiveMoveWarning = false;
    const frame   = this.archiveStack[this.archiveStack.length - 1];
    const entries = this.archivePendingMoveEntries;
    const destDir = this.archivePendingMoveDestDir;
    this.archivePendingMoveEntries = [];
    this.archivePendingMoveDestDir = '';

    try {
      const result = await this.archiveService.move(frame, entries, destDir);

      // sourceRemoved === false means extraction succeeded but delete was skipped
      if (!result.sourceRemoved) {
        const softMsg = (result.errors ?? []).find((e: any) =>
          e.error.includes('NOT removed'),
        );
        this.confirmDialog.alert(
          `Move Partially Complete\n` +
          `${result.succeeded} file(s) extracted, but source NOT removed:\n` +
          (softMsg?.error ?? 'Nested archive deletion is not supported.'),
        );
        return;
      }

      if (result.failed > 0) {
        this.confirmDialog.alert('Move Errors\n' + (result.errors ?? []).map((e: any) => e.error).join('\n'));
      }

      // Refresh archive listing вЂ” moved entries have been deleted from it
      await this.refreshArchive();
    } catch (err: any) {
      this.confirmDialog.alert(`Move Failed\n${err.message}`);
    }
  }

  // в”Ђв”Ђ Tab shortcuts в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  onCloseCurrentTab():   void { 
    const t = this.tabs.find(t => t.isActive); 
    if (t) this.onTabClose(t); 
  }
  
  onNextTab():           void { 
    const i = this.tabs.findIndex(t => t.isActive); 
    if (i >= 0) this.onTabChange(this.tabs[(i + 1) % this.tabs.length]); 
  }
  
  onPreviousTab():       void { 
    const i = this.tabs.findIndex(t => t.isActive); 
    if (i >= 0) this.onTabChange(this.tabs[i === 0 ? this.tabs.length - 1 : i - 1]); 
  }
  
  onJumpToTab(n: number): void { 
    if (n > 0 && n <= this.tabs.length) this.onTabChange(this.tabs[n - 1]); 
  }
  
  onTogglePinCurrentTab(): void { 
    const t = this.tabs.find(t => t.isActive); 
    if (t) this.onTabPin(t); 
  }

  focusAddressBar(): void { 
    this.addressBar?.enterEditMode(); 
  }
  
  focusFind():       void { 
    this.findWidget?.open(); 
  }

  focusFilter(): void { 
    this.findWidget?.toggle(); 
  }
  
  onClearFilter(): void { 
    this.findWidget?.close(); 
  }
}
