// file-view-container.component.ts
// Add these new imports and update the component

import {
  Component, Input, Output, EventEmitter,
  ViewChild, OnChanges, OnDestroy, ChangeDetectorRef, SimpleChanges
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { DomSanitizer, SafeResourceUrl } from '@angular/platform-browser';
import { createSubject, fromPromise } from '@epikodelabs/streamix';
import { mergeMap, takeUntil } from '@epikodelabs/streamix';
import { FileListComponent } from './file-list.component';
import { FileGridComponent } from './file-grid.component';
import { FileTreeComponent, TreeNode, DropTarget } from './file-tree.component';
import { FileDetailsComponent } from './file-details.component';
import { ViewMode } from './action-bar.component';
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

export type ViewLayout = ViewMode;

@Component({
  selector: 'app-file-view-container',
  standalone: true,
  imports: [
    CommonModule,
    FileListComponent,
    FileGridComponent,
    FileTreeComponent,
    FileDetailsComponent
  ],
  template: `
    <div class="view-container">
      <div class="view-browser" *ngIf="browserFrameUrl">
        <iframe
          class="view-browser-frame"
          [src]="browserFrameUrl"
          [attr.title]="currentPath"
          referrerpolicy="no-referrer">
        </iframe>
      </div>

      <div class="view-content">
        <app-file-list
          *ngIf="currentView === 'list'"
          [entries]="sortedEntries"
          [pendingDirectorySizePaths]="pendingDirectorySizePaths"
          [selectedFiles]="selectedFiles"
          [cursorPath]="cursorPath"
          [error]="error"
          [colorizeByMime]="colorizeByMime"
          [paneActive]="paneActive"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)"
          (openInNewTab)="onOpenInNewTab($event)"
          (contextMenu)="onContextMenu($event)"
          (dragStart)="onViewDragStart($event)"
          (dragDrop)="onViewDragDrop($event)"
          (rename)="rename.emit($event)">
        </app-file-list>

        <app-file-grid
          *ngIf="currentView === 'grid'"
          [entries]="sortedEntries"
          [pendingDirectorySizePaths]="pendingDirectorySizePaths"
          [selectedFiles]="selectedFiles"
          [cursorPath]="cursorPath"
          [colorizeByMime]="colorizeByMime"
          [paneActive]="paneActive"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)"
          (openInNewTab)="onOpenInNewTab($event)"
          (contextMenu)="onContextMenu($event)"
          (dragStart)="onViewDragStart($event)"
          (dragDrop)="onViewDragDrop($event)"
          (rename)="rename.emit($event)">
        </app-file-grid>

        <app-file-tree
          #fileTree
          *ngIf="currentView === 'tree'"
          [nodes]="treeNodes"
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
          (dragStart)="onTreeDragStart($event)"
          (dragDrop)="onTreeDragDrop($event)"
          (rename)="onTreeRename($event)">
        </app-file-tree>

        <app-file-details
          *ngIf="currentView === 'details'"
          [entries]="sortedEntries"
          [pendingDirectorySizePaths]="pendingDirectorySizePaths"
          [currentPath]="currentPath"
          [selectedFiles]="selectedFiles"
          [cursorPath]="cursorPath"
          [sortField]="sortField"
          [sortDirection]="sortDirection"
          [colorizeByMime]="colorizeByMime"
          [paneActive]="paneActive"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)"
          (openInNewTab)="onOpenInNewTab($event)"
          (sort)="onSortChange($event)"
          (contextMenu)="onContextMenu($event)"
          (dragStart)="onViewDragStart($event)"
          (dragDrop)="onViewDragDrop($event)"
          (rename)="rename.emit($event)">
        </app-file-details>
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
      flex: 0 0 42%;
      min-height: 180px;
      min-width: 0;
      overflow: hidden;
      border-bottom: 1px solid var(--vsc-border);
      background: var(--vsc-editor-background);
    }

    .view-browser-frame {
      width: 100%;
      height: 100%;
      border: 0;
      background: #fff;
      flex: 1 1 auto;
    }

    .view-content {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    app-file-list,
    app-file-grid,
    app-file-tree,
    app-file-details {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }
  `]
})
export class FileViewContainerComponent implements OnChanges, OnDestroy {
  @ViewChild('fileTree') treeComponent?: FileTreeComponent;
  @ViewChild(FileListComponent) listComponent?: FileListComponent;
  @ViewChild(FileGridComponent) gridComponent?: FileGridComponent;
  @ViewChild(FileDetailsComponent) detailsComponent?: FileDetailsComponent;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() showHidden    = false;
  @Input() entries: FileInfo[] = [];
  @Input() currentPath = '';
  @Input() selectedFiles: Set<string> = new Set();
  @Input() cursorPath: string | null = null;
  @Input() error: string | null = null;
  @Input() currentView: ViewLayout = 'list';
  @Input() sortField     = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';
  @Input() directoriesFirst = true;
  @Input() colorizeByMime = false;
  @Input() paneActive = true;

  @Output() viewChange      = new EventEmitter<ViewLayout>();
  @Output() selectionChange = new EventEmitter<FileInfo[]>();
  @Output() navigate        = new EventEmitter<FileInfo>();
  @Output() openInNewTab    = new EventEmitter<FileInfo>();
  @Output() sortChange      = new EventEmitter<{ field: string; direction: 'asc' | 'desc' }>();
  @Output() contextMenu     = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() nodeDoubleClick = new EventEmitter<FileInfo>();
  @Output() dragStart       = new EventEmitter<{ event: DragEvent; entries: FileInfo[] }>();
  @Output() dragDrop        = new EventEmitter<{ event: DragEvent; source: FileInfo[]; targetPath: string }>();
  @Output() rename          = new EventEmitter<{ entry: FileInfo; newName: string }>();

  sortedEntries: FileInfo[] = [];
  treeNodes: TreeNode[] = [];
  browserFrameUrl: SafeResourceUrl | null = null;

  private loadRequest$ = createSubject<TreeNode>();
  private destroy$     = createSubject<void>();
  pendingDirectorySizePaths: ReadonlySet<string> = new Set<string>();
  private lastTreeRefreshPath: string | null = null;
  private browserFrameRequestId = 0;
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
    private sanitizer: DomSanitizer,
    private cdr: ChangeDetectorRef,
  ) {
    this.loadRequest$
      .pipe(
        mergeMap(node => fromPromise(this.treeController.ensureNodeChildrenLoaded(this.treeHost, node))),
        takeUntil(this.destroy$)
      )
      .subscribe();
  }

  private get treeHost(): FileViewTreeHost {
    const thisRef = this;
    return {
      get currentPath() { return thisRef.currentPath; },
      get currentView() { return thisRef.currentView; },
      get showHidden() { return thisRef.showHidden; },
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
      persistViewMode: (view: ViewLayout) => {
        localStorage.setItem('fileManager_viewMode', view);
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
    if (changes['entries']) {
      this.lastTreeRefreshPath = null;
    }
    if (changes['currentPath']) {
      void this.refreshBrowserFrameUrl();
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

  getSelectableEntries(parentEntryPath: string, includeParent = false): FileInfo[] {
    return this.runtimeController.getSelectableEntries(this.runtimeHost, parentEntryPath, includeParent);
  }

  getVisibleTreeNodes(): TreeNode[] {
    return this.treeComponent?.getVisibleNodes() ?? [];
  }

  isTreeNodeExpanded(path: string): boolean {
    return this.treeComponent?.isNodeExpanded(path) ?? false;
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
    this.sortField     = sort.field;
    this.sortDirection = sort.direction;
    this.sortChange.emit(sort);
    this.sortController.sortEntries(this.sortHost, PARENT_ENTRY_PATH);
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

  private async refreshBrowserFrameUrl(): Promise<void> {
    const requestId = ++this.browserFrameRequestId;
    if (!this.isWebBrowserPath(this.currentPath)) {
      this.browserFrameUrl = null;
      this.cdr.markForCheck();
      return;
    }

    this.browserFrameUrl = null;
    this.cdr.markForCheck();

    try {
      const streamURL = await this.apiService.getStreamURL(this.currentPath);
      if (requestId !== this.browserFrameRequestId) return;
      this.browserFrameUrl = this.sanitizer.bypassSecurityTrustResourceUrl(streamURL);
    } catch (error) {
      if (requestId !== this.browserFrameRequestId) return;
      console.warn('Failed to resolve browser preview URL:', this.currentPath, error);
      this.browserFrameUrl = null;
    }

    this.cdr.markForCheck();
  }
}
