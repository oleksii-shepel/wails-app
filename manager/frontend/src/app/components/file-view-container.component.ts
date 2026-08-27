ypescript
// file-view-container.component.ts

import {
  Component, Input, Output, EventEmitter,
  ViewChild, OnChanges, OnDestroy, ChangeDetectorRef, SimpleChanges, AfterViewInit,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, fromPromise } from '@epikodelabs/streamix';
import { mergeMap, takeUntil } from '@epikodelabs/streamix';
import { PaneListComponent } from './pane-list.component';
import { PaneGridComponent } from './pane-grid.component';
import { PaneTreeComponent, TreeNode, DropTarget, type TreeContentMode } from './pane-tree.component';
import {
  PaneDetailsComponent,
  type PaneDetailsColumnConfiguration,
  type PaneDetailsColumn,
} from './pane-details.component';
import type { DirectoryView as ViewMode } from './pane-component.contract';
import { FileInfo, FileType } from '../shared/protocol';
import { ApiService } from '../services/api.service';
import { DirectorySourceService } from '../services/directory-source.service';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { FileViewEventsController, type FileViewEventsHost } from './file-view-events.controller';
import { FileViewInteractionController, type FileViewInteractionHost } from './file-view-interaction.controller';
import { FileViewLifecycleController, type FileViewLifecycleHost } from './file-view-lifecycle.controller';
import { FileViewTreeController, type FileViewTreeHost } from './file-view-tree.controller';
import { FileViewRuntimeController, type FileViewRuntimeHost } from './file-view-runtime.controller';
import { FileViewSortController, type FileViewSortHost } from './file-view-sort.controller';
import { NativeOverlayComponent } from './native-overlay.component';
import { SplitDividerComponent } from './split-divider.component';

export type ViewLayout = ViewMode;

@Component({
  selector: 'app-file-view-container',
  standalone: true,
  imports: [
    CommonModule,
    PaneListComponent,
    PaneGridComponent,
    PaneTreeComponent,
    PaneDetailsComponent,
    NativeOverlayComponent,
    SplitDividerComponent,
  ],
  template: `
    <div class="view-container">
      <div
        class="view-browser"
        [class.view-browser--hidden]="!cachedShowBrowserPane"
        [style.flexBasis.%]="browserPaneSplit">
        <app-native-overlay
          [surfaceId]="surfaceId"
          [url]="cachedBrowserPath"
          [active]="surfaceActive">
        </app-native-overlay>
      </div>

      <app-split-divider
        *ngIf="cachedShowBrowserPane"
        orientation="horizontal"
        [position]="browserPaneSplit"
        [resetPosition]="defaultBrowserPaneSplit"
        [min]="minBrowserPaneSplit"
        [max]="maxBrowserPaneSplit"
        [snapPoints]="browserPaneSnapPoints"
        (positionChange)="onBrowserPaneSplitChange($event)">
      </app-split-divider>

      <div class="view-content">
        <app-pane-list
          *ngIf="currentView === 'list'"
          [entries]="sortedEntries"
          [pendingDirectorySizePaths]="pendingDirectorySizePaths"
          [selectedFiles]="selectedFiles"
          [cursorPath]="cursorPath"
          [error]="error"
          [singleClickToOpen]="singleClickToOpen"
          [colorizeByMime]="colorizeByMime"
          [paneActive]="paneActive"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)"
          (openInNewTab)="onOpenInNewTab($event)"
          (contextMenu)="onContextMenu($event)"
          (backgroundContextMenu)="onBackgroundContextMenu($event)"
          (dragStart)="onViewDragStart($event)"
          (dragDrop)="onViewDragDrop($event)"
          (rename)="rename.emit($event)"
          (viewportChange)="viewportChange.emit()">
        </app-pane-list>

        <app-pane-grid
          *ngIf="currentView === 'grid'"
          [entries]="sortedEntries"
          [pendingDirectorySizePaths]="pendingDirectorySizePaths"
          [selectedFiles]="selectedFiles"
          [cursorPath]="cursorPath"
          [singleClickToOpen]="singleClickToOpen"
          [colorizeByMime]="colorizeByMime"
          [paneActive]="paneActive"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)"
          (openInNewTab)="onOpenInNewTab($event)"
          (contextMenu)="onContextMenu($event)"
          (backgroundContextMenu)="onBackgroundContextMenu($event)"
          (dragStart)="onViewDragStart($event)"
          (dragDrop)="onViewDragDrop($event)"
          (rename)="rename.emit($event)"
          (viewportChange)="viewportChange.emit()">
        </app-pane-grid>

        <app-pane-tree
          #fileTree
          *ngIf="currentView === 'tree'"
          [nodes]="treeNodes"
          [contentMode]="treeContentMode"
          [pendingDirectorySizePaths]="pendingDirectorySizePaths"
          [currentPath]="currentPath"
          [colorizeByMime]="colorizeByMime"
          [paneActive]="paneActive"
          [selectedFiles]="selectedFiles"
          (selectionChange)="onTreeSelectionChange($event)"
          (inlineSize)="onTreeInlineSize($event)"
          (nodeDoubleClick)="onTreeNodeDoubleClick($event)"
          (toggleExpand)="onTreeToggleExpand($event)"
          (loadChildren)="onLoadChildren($event)"
          (navigate)="onNavigate($event)"
          (openInNewTab)="onTreeOpenInNewTab($event)"
          (contextMenu)="onTreeContextMenu($event)"
          (backgroundContextMenu)="onBackgroundContextMenu($event)"
          (dragStart)="onTreeDragStart($event)"
          (dragDrop)="onTreeDragDrop($event)"
          (rename)="onTreeRename($event)"
          (viewportChange)="viewportChange.emit()">
        </app-pane-tree>

        <app-pane-details
          *ngIf="currentView === 'details'"
          [entries]="sortedEntries"
          [pendingDirectorySizePaths]="pendingDirectorySizePaths"
          [currentPath]="currentPath"
          [selectedFiles]="selectedFiles"
          [cursorPath]="cursorPath"
          [singleClickToOpen]="singleClickToOpen"
          [sortField]="sortField"
          [sortDirection]="sortDirection"
          [columns]="detailsColumns"
          [columnConfiguration]="detailsColumnConfiguration"
          [colorizeByMime]="colorizeByMime"
          [paneActive]="paneActive"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)"
          (openInNewTab)="onOpenInNewTab($event)"
          (sort)="onSortChange($event)"
          (columnConfigurationChange)="detailsColumnConfigurationChange.emit($event)"
          (contextMenu)="onContextMenu($event)"
          (backgroundContextMenu)="onBackgroundContextMenu($event)"
          (dragStart)="onViewDragStart($event)"
          (dragDrop)="onViewDragDrop($event)"
          (rename)="rename.emit($event)"
          (viewportChange)="viewportChange.emit()">
        </app-pane-details>
      </div>
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

    .view-container {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    .view-browser {
      display: flex;
      flex: 0 0 auto;
      flex-basis: 42%;
      min-height: 180px;
      min-width: 0;
      overflow: hidden;
      background: var(--vsc-editor-background);
      position: relative;
      z-index: 0;
    }

    .view-browser.view-browser--hidden {
      display: none;
    }

    app-split-divider {
      display: block;
      flex: 0 0 auto;
    }

    .view-content {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    app-pane-list,
    app-pane-grid,
    app-pane-tree,
    app-pane-details {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }
  `]
})
export class FileViewContainerComponent implements OnChanges, OnDestroy, AfterViewInit {
  readonly minBrowserPaneSplit = 20;
  readonly maxBrowserPaneSplit = 72;
  readonly defaultBrowserPaneSplit = 42;
  readonly browserPaneSnapPoints = [25, 42, 60];

  @ViewChild('fileTree') treeComponent?: PaneTreeComponent;
  @ViewChild(PaneListComponent) listComponent?: PaneListComponent;
  @ViewChild(PaneGridComponent) gridComponent?: PaneGridComponent;
  @ViewChild(PaneDetailsComponent) detailsComponent?: PaneDetailsComponent;

  cachedBrowserPath = '';
  cachedShowBrowserPane = false;
  browserPaneSplit = this.defaultBrowserPaneSplit;

  @Input() surfaceId = '';
  @Input() surfaceActive = true;
  @Input() browserPath = '';
  @Input() persistedBrowserPaneSplit: number | null = null;
  @Input() paneId: 'left' | 'right' = 'left';
  @Input() showHidden    = false;
  @Input() entries: FileInfo[] = [];
  @Input() currentPath = '';
  @Input() selectedFiles: Set<string> = new Set();
  @Input() cursorPath: string | null = null;
  @Input() error: string | null = null;
  @Input() currentView: ViewLayout = 'list';
  @Input() treeContentMode: TreeContentMode = 'all';
  @Input() singleClickToOpen = false;
  @Input() sortField     = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';
  @Input() directoriesFirst = true;
  @Input() detailsColumns: PaneDetailsColumn[] | null = null;
  @Input() detailsColumnConfiguration: PaneDetailsColumnConfiguration | null = null;
  @Input() colorizeByMime = false;
  @Input() paneActive = true;

  @Output() viewChange      = new EventEmitter<ViewLayout>();
  @Output() selectionChange = new EventEmitter<FileInfo[]>();
  @Output() navigate        = new EventEmitter<FileInfo>();
  @Output() openInNewTab    = new EventEmitter<FileInfo>();
  @Output() sortChange      = new EventEmitter<{ field: string; direction: 'asc' | 'desc' }>();
  @Output() detailsColumnConfigurationChange = new EventEmitter<PaneDetailsColumnConfiguration>();
  @Output() contextMenu     = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() nodeDoubleClick = new EventEmitter<FileInfo>();
  @Output() dragStart       = new EventEmitter<{ event: DragEvent; entries: FileInfo[] }>();
  @Output() dragDrop        = new EventEmitter<{ event: DragEvent; source: FileInfo[]; targetPath: string }>();
  @Output() rename          = new EventEmitter<{ entry: FileInfo; newName: string }>();
  @Output() browserPaneSplitChange = new EventEmitter<number>();
  @Output() viewportChange = new EventEmitter<void>();

  sortedEntries: FileInfo[] = [];
  treeNodes: TreeNode[] = [];
  private loadRequest$ = createSubject<TreeNode>();
  private destroy$     = createSubject<void>();
  pendingDirectorySizePaths: ReadonlySet<string> = new Set<string>();
  private lastTreeRefreshPath: string | null = null;
  private loadedBrowserPaneStorageKey: string | null = null;
  private readonly eventsController = new FileViewEventsController();
  private readonly interactionController = new FileViewInteractionController();
  private readonly lifecycleController = new FileViewLifecycleController();
  private readonly treeController = new FileViewTreeController();
  private readonly runtimeController = new FileViewRuntimeController();
  private readonly sortController = new FileViewSortController();

  getSortedEntries(): FileInfo[] {
    return this.sortedEntries;
  }

  constructor(
    private apiService: ApiService,
    private directorySource: DirectorySourceService,
    private cdr: ChangeDetectorRef,
  ) {
    this.loadRequest$
      .pipe(
        mergeMap(node => fromPromise(this.treeController.ensureNodeChildrenLoaded(this.treeHost, node))),
        takeUntil(this.destroy$)
      )
      .subscribe();
  }

  ngAfterViewInit(): void {
    this.cdr.markForCheck();
  }

  private get treeHost(): FileViewTreeHost {
    const thisRef = this;
    return {
      get currentPath() { return thisRef.currentPath; },
      get currentView() { return thisRef.currentView; },
      get showHidden() { return thisRef.showHidden; },
      get treeContentMode() { return thisRef.treeContentMode; },
      get entries() { return thisRef.entries; },
      get treeNodes() { return thisRef.treeNodes; },
      set treeNodes(value: TreeNode[]) { thisRef.treeNodes = value; },
      directorySource: this.directorySource,
      markForCheck: () => thisRef.cdr.markForCheck(),
      sortTreeNodes: (nodes: TreeNode[]) => thisRef.sortTreeNodes(nodes),
      expandTreeNode: (path: string) => thisRef.treeComponent?.expandNode(path),
      centerTreeNode: (path: string) => thisRef.treeComponent?.centerNode(path),
    };
  }

  private get eventsHost(): FileViewEventsHost {
    const thisRef = this;
    return {
      selectionChange: this.selectionChange,
      navigate: this.navigate,
      openInNewTab: this.openInNewTab,
      contextMenu: this.contextMenu,
      nodeDoubleClick: this.nodeDoubleClick,
      rename: this.rename,
      setSelectedPaths: (paths: string[]) => {
        thisRef.selectedFiles = new Set(paths);
      },
      setCursorPath: (path: string | null) => {
        thisRef.cursorPath = path;
      },
    };
  }

  private get interactionHost(): FileViewInteractionHost {
    const thisRef = this;
    return {
      get paneId() { return thisRef.paneId; },
      get currentPath() { return thisRef.currentPath; },
      get currentView() { return thisRef.currentView; },
      set currentView(value: ViewLayout) { thisRef.currentView = value; },
      dragStart: this.dragStart,
      dragDrop: this.dragDrop,
      viewChange: this.viewChange,
      persistViewMode: (_view: ViewLayout) => {
        // Per-tab view persistence is handled by file-pane-ui.controller via
        // the per-tab DirTabState; the global localStorage default is no
        // longer rewritten on every per-tab change.
      },
      syncTreeToCurrentPath: () => {
        thisRef.treeController.buildTreeNodes(thisRef.treeHost);
        void thisRef.treeController.syncToCurrentPath(thisRef.treeHost);
      },
      enqueueTreeLoad: (node: TreeNode) => {
        thisRef.loadRequest$.next(node);
      },
    };
  }

  private get lifecycleHost(): FileViewLifecycleHost {
    const thisRef = this;
    return {
      get currentView() { return thisRef.currentView; },
      sortEntries: () => {
        thisRef.sortController.sortEntries(thisRef.sortHost, PARENT_ENTRY_PATH);
      },
      syncTreeToCurrentPath: () => {
        thisRef.lastTreeRefreshPath = thisRef.currentPath;
        thisRef.treeController.buildTreeNodes(thisRef.treeHost);
        void thisRef.treeController.syncToCurrentPath(thisRef.treeHost);
      },
      refreshCurrentTreePath: () => {
        const targetPath = thisRef.currentPath;
        if (!targetPath || thisRef.lastTreeRefreshPath === targetPath) {
          return;
        }
        thisRef.lastTreeRefreshPath = targetPath;
        thisRef.treeController.refreshTreeNode(thisRef.treeHost, targetPath);
      },
      resortTreeNodes: () => {
        thisRef.sortController.resortAllLoadedTreeNodes(thisRef.sortHost, thisRef.treeNodes);
      },
      markTreeNodesChanged: () => {
        thisRef.treeNodes = [...thisRef.treeNodes];
      },
      markForCheck: () => thisRef.cdr.markForCheck(),
    };
  }

  private get runtimeHost(): FileViewRuntimeHost {
    const thisRef = this;
    return {
      get currentView() { return thisRef.currentView; },
      get sortedEntries() { return thisRef.sortedEntries; },
      getVisibleTreeNodes: () => thisRef.getVisibleTreeNodes(),
      startRenameInList: () => thisRef.listComponent?.startRenameSelected(),
      startRenameInGrid: () => thisRef.gridComponent?.startRenameSelected(),
      startRenameInDetails: () => thisRef.detailsComponent?.startRenameSelected(),
      startRenameInTree: () => {
        try {
          (thisRef.treeComponent as any)?.startRenameFocused?.();
        } catch {
          // ignore
        }
      },
      startRenamePathInList: (path: string) => thisRef.listComponent?.startRenamePath(path),
      startRenamePathInGrid: (path: string) => thisRef.gridComponent?.startRenamePath(path),
      startRenamePathInDetails: (path: string) => thisRef.detailsComponent?.startRenamePath(path),
      startRenamePathInTree: (path: string) => thisRef.treeComponent?.startRenamePath(path),
      scrollListToPath: (path: string) => thisRef.listComponent?.scrollToPath(path),
      scrollDetailsToPath: (path: string) => thisRef.detailsComponent?.scrollToPath(path),
      scrollGridToPath: (path: string) => thisRef.gridComponent?.scrollToPath(path),
      focusTreeNode: (path: string) => thisRef.focusTreeNode(path),
      getListApproxPageSize: () => thisRef.listComponent?.getApproxPageSize() ?? 10,
      getDetailsApproxPageSize: () => thisRef.detailsComponent?.getApproxPageSize() ?? 10,
      getGridApproxPageSize: () => thisRef.gridComponent?.getApproxPageSize() ?? 5,
      getGridColumns: () => thisRef.gridComponent?.getColumns() ?? 1,
    };
  }

  private get sortHost(): FileViewSortHost {
    const thisRef = this;
    return {
      get entries() { return thisRef.entries; },
      get sortField() { return thisRef.sortField; },
      get sortDirection() { return thisRef.sortDirection; },
      get directoriesFirst() { return thisRef.directoriesFirst; },
      setSortedEntries: (entries: FileInfo[]) => {
        thisRef.sortedEntries = entries;
      },
    };
  }

  ngOnChanges(changes: SimpleChanges): void {
    this.ensureBrowserPaneSplitLoaded();
    if (changes['persistedBrowserPaneSplit']) {
      const persisted = changes['persistedBrowserPaneSplit'].currentValue;
      if (typeof persisted === 'number' && Number.isFinite(persisted)) {
        this.browserPaneSplit = this.clampBrowserPaneSplit(persisted);
      } else {
        this.browserPaneSplit = this.readBrowserPaneSplit(this.getBrowserPaneStorageKey());
      }
    }

    if (changes['currentPath'] || changes['browserPath'] || changes['surfaceActive']) {
      this.cachedBrowserPath = this.resolveSurfaceBrowserPath();
      this.cachedShowBrowserPane = this.isWebBrowserPath(this.cachedBrowserPath);
    }
    if (changes['entries']) {
      this.lastTreeRefreshPath = null;
    }
    if (changes['treeContentMode']) {
      this.treeController.reset();
      this.treeNodes = [];
    }
    this.lifecycleController.onChanges(this.lifecycleHost, changes);
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
    this.treeController.reset();
  }

  // ── Tree specific handlers ─────────────────────────────────

  onTreeSelectionChange(nodes: TreeNode[]): void {
    this.eventsController.onTreeSelectionChange(this.eventsHost, nodes);
  }

  onTreeInlineSize(node: TreeNode): void {
    void this.calculateDirectorySize(node.path);
  }

  onTreeContextMenu(event: { event: MouseEvent; node: TreeNode }): void {
    this.eventsController.onTreeContextMenu(this.eventsHost, event);
  }

  onBackgroundContextMenu(event: MouseEvent): void {
    this.backgroundContextMenu.emit(event);
  }

  onTreeDragStart(event: { event: DragEvent; nodes: TreeNode[] }): void {
    this.interactionController.onTreeDragStart(
      this.interactionHost,
      event,
      (node) => this.runtimeController.treeNodeToFileInfo(node),
    );
  }

  onTreeDragDrop(event: { event: DragEvent; source: TreeNode[]; target: DropTarget }): void {
    this.interactionController.onTreeDragDrop(
      this.interactionHost,
      event,
      (node) => this.runtimeController.treeNodeToFileInfo(node),
    );
  }

  onViewDragStart(event: { event: DragEvent; entries: FileInfo[] }): void {
    this.interactionController.onViewDragStart(this.interactionHost, event);
  }

  onViewDragDrop(event: { event: DragEvent; source: FileInfo[]; target: FileInfo }): void {
    this.interactionController.onViewDragDrop(this.interactionHost, event);
  }

  onTreeRename(event: { node: TreeNode; newName: string }): void {
    this.eventsController.onTreeRename(this.eventsHost, event);
  }

  /** Trigger in-place rename for the currently selected item in the active view (eg. on F2). */
  startRenameSelected(): void {
    this.runtimeController.startRenameSelected(this.runtimeHost);
  }

  startRenamePath(path: string): void {
    this.runtimeController.startRenamePath(this.runtimeHost, path);
  }

  setPendingRenameDisplayName(path: string, displayName: string | null): void {
    switch (this.currentView) {
      case 'list':
        this.listComponent?.setPendingRenameDisplayName(path, displayName);
        return;
      case 'grid':
        this.gridComponent?.setPendingRenameDisplayName(path, displayName);
        return;
      case 'details':
        this.detailsComponent?.setPendingRenameDisplayName(path, displayName);
        return;
      case 'tree':
        this.treeComponent?.setPendingRenameDisplayName(path, displayName);
        return;
      default:
        return;
    }
  }

  hasPendingTreeRename(): boolean {
    return this.treeComponent?.hasPendingRenameState() ?? false;
  }

  refreshTreeNode(path: string): void {
    if (!path) return;
    this.treeController.refreshTreeNode(this.treeHost, path);
  }

  /** Scroll the active view to bring the given path into view. */
  scrollToCursorPath(path: string): void {
    this.runtimeController.scrollToCursorPath(this.runtimeHost, path);
  }

  getScrollTop(): number {
    switch (this.currentView) {
      case 'list':
        return this.listComponent?.getScrollTop() ?? 0;
      case 'grid':
        return this.gridComponent?.getScrollTop() ?? 0;
      case 'details':
        return this.detailsComponent?.getScrollTop() ?? 0;
      case 'tree':
        return this.treeComponent?.getScrollTop() ?? 0;
      default:
        return 0;
    }
  }


  getScrollMetrics(): { scrollTop: number; scrollHeight: number; clientHeight: number } {
    switch (this.currentView) {
      case 'list':
        return this.listComponent?.getScrollMetrics() ?? { scrollTop: 0, scrollHeight: 0, clientHeight: 0 };
      case 'grid':
        return this.gridComponent?.getScrollMetrics() ?? { scrollTop: 0, scrollHeight: 0, clientHeight: 0 };
      case 'details':
        return this.detailsComponent?.getScrollMetrics() ?? { scrollTop: 0, scrollHeight: 0, clientHeight: 0 };
      case 'tree':
        return this.treeComponent?.getScrollMetrics() ?? { scrollTop: 0, scrollHeight: 0, clientHeight: 0 };
      default:
        return { scrollTop: 0, scrollHeight: 0, clientHeight: 0 };
    }
  }

  setScrollTop(top: number): void {
    switch (this.currentView) {
      case 'list':
        this.listComponent?.setScrollTop(top);
        return;
      case 'grid':
        this.gridComponent?.setScrollTop(top);
        return;
      case 'details':
        this.detailsComponent?.setScrollTop(top);
        return;
      case 'tree':
        this.treeComponent?.setScrollTop(top);
        return;
      default:
        return;
    }
  }

  getSelectableEntries(parentEntryPath: string, includeParent = false): FileInfo[] {
    return this.runtimeController.getSelectableEntries(this.runtimeHost, parentEntryPath, includeParent);
  }

  getVisibleTreeNodes(): TreeNode[] {
    return this.treeComponent?.getVisibleNodes() ?? [];
  }

  isTreeNodeExpanded(path: string): boolean {
    return this.treeComponent?.isNodeExpanded(path) ?? false;
  }

  getExpandedPaths(): string[] {
    return this.treeComponent?.getExpandedPaths() ?? [];
  }

  setExpandedPaths(paths: string[]): void {
    this.treeComponent?.setExpandedPaths(paths);
  }

  suppressNextTreeAutoExpand(): void {
    this.treeComponent?.suppressNextAutoExpandToCurrentPath();
  }

  async calculateDirectorySize(path: string): Promise<void> {
    if (!path || path === PARENT_ENTRY_PATH) return;
    if (this.pendingDirectorySizePaths.has(path)) return;

    const entry = this.entries.find(candidate => candidate.path === path);
    if (!entry || entry.type !== FileType.DIRECTORY) return;

    this.setDirectorySizePending(path, true);
    try {
      const size = await this.apiService.dirSizeRecursive(path);
      this.applyDirectorySize(path, size);
    } catch (error) {
      console.warn('Failed to calculate directory size inline:', path, error);
    } finally {
      this.setDirectorySizePending(path, false);
    }
  }

  focusTreeNode(path: string): void {
    try {
      this.treeComponent?.focusNode(path);
    } catch {
      // best-effort only
    }
  }

  /** Approximate number of items per page in the active view. */
  getApproxPageSize(): number {
    return this.runtimeController.getApproxPageSize(this.runtimeHost);
  }

  /** Number of columns in grid view (1 for all other views). */
  getGridColumns(): number {
    return this.runtimeController.getGridColumns(this.runtimeHost);
  }

  onTreeOpenInNewTab(node: TreeNode): void {
    this.eventsController.onTreeOpenInNewTab(this.eventsHost, node);
  }

  // ── Load children ──────────────────────────────────────────

  onLoadChildren(node: TreeNode): void {
    this.interactionController.onLoadChildren(this.interactionHost, node);
  }

  // ── View / sort ────────────────────────────────────────────

  onViewChange(view: ViewLayout): void {
    this.interactionController.onViewChange(this.interactionHost, view);
  }

  onSortChange(sort: { field: string; direction: 'asc' | 'desc' }): void {
    this.sortField = sort.field;
    this.sortDirection = sort.direction;

    // Sorting is source-owned. DirectoryPane/DirListStreamController will
    // rebuild incremental sorted runs or request a new backend window. Do not
    // perform a second O(n log n) client sort from the Details header.
    this.sortChange.emit(sort);
  }

  // ── Event passthrough ──────────────────────────────────────

  onSelectionChange(entries: FileInfo[]): void {
    this.eventsController.onSelectionChange(this.eventsHost, entries);
  }

  onNavigate(entry: FileInfo | TreeNode | Event): void {
    this.eventsController.onNavigate(this.eventsHost, entry);
  }

  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void {
    this.eventsController.onContextMenu(this.eventsHost, event);
  }

  onOpenInNewTab(entry: FileInfo): void {
    this.eventsController.onOpenInNewTab(this.eventsHost, entry);
  }

  onTreeNodeDoubleClick(node: TreeNode): void {
    this.eventsController.onTreeNodeDoubleClick(this.eventsHost, node);
  }

  onTreeToggleExpand(_node: TreeNode): void {
    // Tree manages expansion state internally
  }

  // ── Sorting ────────────────────────────────────────────────

  private sortTreeNodes(nodes: TreeNode[]): TreeNode[] {
    return this.sortController.sortTreeNodes(this.sortHost, nodes);
  }

  private setDirectorySizePending(path: string, pending: boolean): void {
    const next = new Set(this.pendingDirectorySizePaths);
    if (pending) {
      next.add(path);
    } else {
      next.delete(path);
    }
    this.pendingDirectorySizePaths = next;
    this.cdr.markForCheck();
  }

  private applyDirectorySize(path: string, size: number): void {
    const entry = this.entries.find(candidate => candidate.path === path);
    if (entry) {
      entry.size = size;
    }

    const sortedEntry = this.sortedEntries.find(candidate => candidate.path === path);
    if (sortedEntry && sortedEntry !== entry) {
      sortedEntry.size = size;
    }

    const treeNode = this.findTreeNodeByPath(this.treeNodes, path);
    if (treeNode) {
      treeNode.size = size;
    }

    if (this.sortField === 'size') {
      this.sortController.sortEntries(this.sortHost, PARENT_ENTRY_PATH);
      this.sortController.resortAllLoadedTreeNodes(this.sortHost, this.treeNodes);
    } else {
      this.sortedEntries = [...this.sortedEntries];
    }

    this.treeNodes = [...this.treeNodes];
    this.cdr.markForCheck();
  }

  private findTreeNodeByPath(nodes: TreeNode[], path: string): TreeNode | undefined {
    for (const node of nodes) {
      if (node.path === path) return node;
      const childNodes = node.children;
      if (!childNodes?.length) continue;
      const match = this.findTreeNodeByPath(childNodes, path);
      if (match) return match;
    }

    return undefined;
  }

  private isWebBrowserPath(path: string): boolean {
    const trimmed = (path ?? '').trim();
    if (!trimmed) return false;

    try {
      const parsed = new URL(trimmed);
      return (parsed.protocol === 'http:' || parsed.protocol === 'https:') && parsed.host.length > 0;
    } catch {
      return false;
    }
  }

  private resolveSurfaceBrowserPath(): string {
    if (this.surfaceActive) {
      return this.currentPath || this.browserPath;
    }

    return this.browserPath || this.currentPath;
  }

  getBrowserPath(): string {
    return this.cachedBrowserPath;
  }

  showBrowserPane(): boolean {
    return this.cachedShowBrowserPane;
  }

  onBrowserPaneSplitChange(position: number): void {
    const next = this.clampBrowserPaneSplit(position);
    if (next === this.browserPaneSplit) {
      return;
    }

    this.browserPaneSplit = next;
    this.persistBrowserPaneSplit();
    this.browserPaneSplitChange.emit(next);
    this.cdr.markForCheck();
  }

  private ensureBrowserPaneSplitLoaded(): void {
    const storageKey = this.getBrowserPaneStorageKey();
    if (this.loadedBrowserPaneStorageKey === storageKey) {
      return;
    }

    this.loadedBrowserPaneStorageKey = storageKey;
    this.browserPaneSplit = this.readBrowserPaneSplit(storageKey);
  }

  private getBrowserPaneStorageKey(): string {
    return `fileManager_browserPaneSplit:${this.paneId}`;
  }

  private readBrowserPaneSplit(storageKey: string): number {
    try {
      const raw = localStorage.getItem(storageKey);
      if (!raw) {
        return this.defaultBrowserPaneSplit;
      }

      const parsed = Number.parseFloat(raw);
      if (!Number.isFinite(parsed)) {
        return this.defaultBrowserPaneSplit;
      }

      return this.clampBrowserPaneSplit(parsed);
    } catch {
      return this.defaultBrowserPaneSplit;
    }
  }

  private persistBrowserPaneSplit(): void {
    try {
      localStorage.setItem(this.getBrowserPaneStorageKey(), this.browserPaneSplit.toString());
    } catch {
      // Ignore storage failures; the split still works for the current session.
    }
  }

  private clampBrowserPaneSplit(value: number): number {
    return Math.max(this.minBrowserPaneSplit, Math.min(this.maxBrowserPaneSplit, value));
  }
}