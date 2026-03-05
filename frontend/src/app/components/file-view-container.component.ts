// file-view-container.component.ts
// Add these new imports and update the component

import {
  Component, Input, Output, EventEmitter,
  ViewChild, OnChanges, OnDestroy, ChangeDetectorRef, SimpleChanges
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, fromPromise } from '@epikodelabs/streamix';
import { mergeMap, takeUntil } from '@epikodelabs/streamix';
import { FileListComponent } from './file-list.component';
import { FileGridComponent } from './file-grid.component';
import { FileTreeComponent, TreeNode, DropTarget } from './file-tree.component';
import { FileDetailsComponent } from './file-details.component';
import { ViewMode } from './action-bar.component';
import { FileInfo, FileType } from '../shared/protocol';
import { ApiService } from '../services/api.service';
import { parseArchivePath, buildArchivePath } from '../services/archive.service';
import { parentPath } from './address-bar.utils';
import { PARENT_ENTRY_PATH } from './file-entry.component';

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
      <div class="view-content">
        <app-file-list
          *ngIf="currentView === 'list'"
          [entries]="sortedEntries"
          [selectedFiles]="selectedFiles"
          [error]="error"
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
          [selectedFiles]="selectedFiles"
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
          [currentPath]="currentPath"
          [selectedFiles]="selectedFiles"
          (selectionChange)="onTreeSelectionChange($event)"
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
          [selectedFiles]="selectedFiles"
          [sortField]="sortField"
          [sortDirection]="sortDirection"
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
  @Input() error: string | null = null;
  @Input() currentView: ViewLayout = 'list';
  @Input() sortField     = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';
  @Input() directoriesFirst = true;

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

  private loadRequest$ = createSubject<TreeNode>();
  private destroy$     = createSubject<void>();
  private nodeLoadPromises = new Map<string, Promise<void>>();
  private syncToken = 0;

  constructor(private apiService: ApiService, private cdr: ChangeDetectorRef) {
    this.loadRequest$
      .pipe(
        mergeMap(node => fromPromise(this.ensureNodeChildrenLoaded(node))),
        takeUntil(this.destroy$)
      )
      .subscribe();
  }

  ngOnChanges(changes: SimpleChanges): void {
    this.sortEntries();
    const switchedToTree = !!changes['currentView'] && this.currentView === 'tree';
    const treeRelevantChange = switchedToTree || !!changes['currentPath'] || !!changes['showHidden'];
    if (this.currentView === 'tree' && treeRelevantChange) {
      this.buildTreeNodes();
      void this.syncTreeToCurrentPath();
    }
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  // ── Tree specific handlers ─────────────────────────────────

  onTreeSelectionChange(nodes: TreeNode[]): void {
    const files = nodes.map(n => this.treeNodeToFileInfo(n));
    this.selectedFiles = new Set(nodes.map(n => n.path));
    this.selectionChange.emit(files);
  }

  onTreeContextMenu(event: { event: MouseEvent; node: TreeNode }): void {
    this.contextMenu.emit({ event: event.event, entry: this.treeNodeToFileInfo(event.node) });
  }

  onTreeDragStart(event: { event: DragEvent; nodes: TreeNode[] }): void {
    try {
      event.event.dataTransfer?.setData('application/x-wails-pane-id', this.paneId);
    } catch {
      // ignore
    }
    this.dragStart.emit({
      event: event.event,
      entries: event.nodes.map(n => this.treeNodeToFileInfo(n))
    });
  }

  onTreeDragDrop(event: { event: DragEvent; source: TreeNode[]; target: DropTarget }): void {
    const targetPath = event.target.position === 'inside' 
      ? event.target.node.path 
      : parentPath(event.target.node.path);
    
    this.dragDrop.emit({
      event: event.event,
      source: event.source.map(n => this.treeNodeToFileInfo(n)),
      targetPath
    });
  }

  onViewDragStart(event: { event: DragEvent; entries: FileInfo[] }): void {
    try {
      event.event.dataTransfer?.setData('application/x-wails-pane-id', this.paneId);
    } catch {
      // ignore
    }
    this.dragStart.emit(event);
  }

  onViewDragDrop(event: { event: DragEvent; source: FileInfo[]; target: FileInfo }): void {
    let targetPath = event.target.path;
    if (event.target.path === PARENT_ENTRY_PATH) {
      targetPath = parentPath(this.currentPath);
    } else if (event.target.type !== FileType.DIRECTORY) {
      targetPath = parentPath(event.target.path);
    }

    if (!targetPath) {
      targetPath = this.currentPath;
    }

    this.dragDrop.emit({
      event: event.event,
      source: event.source,
      targetPath,
    });
  }

  onTreeRename(event: { node: TreeNode; newName: string }): void {
    this.rename.emit({
      entry: this.treeNodeToFileInfo(event.node),
      newName: event.newName
    });
  }

  /** Trigger in-place rename for the currently selected item in the active view (eg. on F2). */
  startRenameSelected(): void {
    switch (this.currentView) {
      case 'list':
        this.listComponent?.startRenameSelected();
        return;
      case 'grid':
        this.gridComponent?.startRenameSelected();
        return;
      case 'details':
        this.detailsComponent?.startRenameSelected();
        return;
      case 'tree':
        try {
          (this.treeComponent as any)?.startRenameFocused?.();
        } catch {
          // ignore
        }
        return;
      default:
        return;
    }
  }

  onTreeOpenInNewTab(node: TreeNode): void {
    this.openInNewTab.emit(this.treeNodeToFileInfo(node));
  }

  private treeNodeToFileInfo(node: TreeNode): FileInfo {
    return {
      name: node.name,
      path: node.path,
      type: node.type,
      size: node.size,
      modified: node.modified,
      created: node.created
    } as FileInfo;
  }

  // ── Load children ──────────────────────────────────────────

  onLoadChildren(node: TreeNode): void {
    this.loadRequest$.next(node);
  }

  private loadNodeChildren(node: TreeNode): Promise<void> {
    return this.ensureNodeChildrenLoaded(node);
  }

  // ── View / sort ────────────────────────────────────────────

  onViewChange(view: ViewLayout): void {
    this.currentView = view;
    this.viewChange.emit(view);
    localStorage.setItem('fileManager_viewMode', view);
    if (view === 'tree') {
      this.buildTreeNodes();
      void this.syncTreeToCurrentPath();
    }
  }

  onSortChange(sort: { field: string; direction: 'asc' | 'desc' }): void {
    this.sortField     = sort.field;
    this.sortDirection = sort.direction;
    this.sortChange.emit(sort);
    this.sortEntries();
  }

  // ── Event passthrough ──────────────────────────────────────

  onSelectionChange(entries: FileInfo[]): void {
    this.selectedFiles = new Set(entries.map(e => e.path));
    this.selectionChange.emit(entries);
  }

  onNavigate(entry: FileInfo | TreeNode | Event): void {
    if (!entry || (entry as Event) instanceof Event) return;
    const e = entry as any;
    if (e.path) {
      const payload: any = {
        name: e.name, path: e.path, type: e.type,
        size: e.size, modified: e.modified, created: e.created,
      };
      if (Array.isArray(e.nestedChain)) {
        payload.nestedChain = [...e.nestedChain];
      }
      this.navigate.emit(payload as FileInfo);
    }
  }

  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void {
    this.contextMenu.emit(event);
  }

  onOpenInNewTab(entry: FileInfo): void {
    this.openInNewTab.emit(entry);
  }

  onTreeNodeDoubleClick(node: TreeNode): void {
    if (node && typeof node === 'object' && 'path' in node && 'name' in node && 'type' in node) {
      const file: FileInfo = {
        name: node.name,
        path: node.path,
        type: node.type,
        size: node.size,
        modified: node.modified,
        created: node.created
      } as FileInfo;

      if (node.type === FileType.DIRECTORY) {
        this.onNavigate(file);
      } else {
        this.nodeDoubleClick.emit(file);
      }
    }
  }

  onTreeToggleExpand(_node: TreeNode): void {
    // Tree manages expansion state internally
  }

  // ── Sorting ────────────────────────────────────────────────

  private sortEntries(): void {
    const sorted     = [...this.entries];
    const multiplier = this.sortDirection === 'asc' ? 1 : -1;

    sorted.sort((a, b) => {
      if (a.path === PARENT_ENTRY_PATH) return -1;
      if (b.path === PARENT_ENTRY_PATH) return 1;

      if (this.directoriesFirst) {
        if (a.type === FileType.DIRECTORY && b.type !== FileType.DIRECTORY) return -1;
        if (b.type === FileType.DIRECTORY && a.type !== FileType.DIRECTORY) return 1;
      }

      let comparison = 0;
      switch (this.sortField) {
        case 'name':     comparison = a.name.localeCompare(b.name); break;
        case 'size':     comparison = (a.size || 0) - (b.size || 0); break;
        case 'modified': comparison = (a.modified || 0) - (b.modified || 0); break;
        case 'type': {
          const ta = a.type === FileType.DIRECTORY ? 'folder' : (a.name.split('.').pop() || '');
          const tb = b.type === FileType.DIRECTORY ? 'folder' : (b.name.split('.').pop() || '');
          comparison = ta.localeCompare(tb);
          break;
        }
        default: comparison = a.name.localeCompare(b.name);
      }

      return comparison * multiplier;
    });

    this.sortedEntries = sorted;
  }

  // ── Tree building ──────────────────────────────────────────

  private buildTreeNodes(): void {
    const rootPath = this.getRootPath(this.currentPath);
    if (!rootPath) {
      this.treeNodes = [];
      return;
    }

    const currentRoot = this.treeNodes[0];
    if (currentRoot && this.normalizePath(currentRoot.path) === this.normalizePath(rootPath)) {
      return;
    }

    this.nodeLoadPromises.clear();
    this.treeNodes = [this.createRootNode(rootPath)];
    this.cdr.markForCheck();
  }

  private async syncTreeToCurrentPath(): Promise<void> {
    if (this.currentView !== 'tree') return;
    this.buildTreeNodes();
    const rootNode = this.treeNodes[0];
    if (!rootNode) return;

    const token = ++this.syncToken;
    const chain = this.buildPathChain(rootNode.path, this.currentPath);
    if (chain.length === 0) return;

    let current = rootNode;
    const expandedNodes: TreeNode[] = [rootNode];
    await this.ensureNodeChildrenLoaded(current);
    if (token !== this.syncToken) return;

    for (let i = 1; i < chain.length; i++) {
      const expected = this.normalizePath(chain[i]);
      const next = current.children?.find(c => this.normalizePath(c.path) === expected);
      if (!next || (next.type !== FileType.DIRECTORY && next.type !== FileType.ARCHIVE)) break;
      current = next;
      expandedNodes.push(current);
      await this.ensureNodeChildrenLoaded(current);
      if (token !== this.syncToken) return;
    }

    setTimeout(() => {
      if (token !== this.syncToken || this.currentView !== 'tree') return;
      for (const node of expandedNodes) {
        this.treeComponent?.expandNode(node.path);
      }
      this.treeComponent?.centerNode(current.path);
    }, 0);
  }

  private async ensureNodeChildrenLoaded(node: TreeNode): Promise<void> {
    if (node.loaded) return;

    const existingPromise = this.nodeLoadPromises.get(node.path);
    if (existingPromise) {
      await existingPromise;
      return;
    }

    const loadPromise = this.populateNodeChildren(node);
    this.nodeLoadPromises.set(node.path, loadPromise);
    try {
      await loadPromise;
    } finally {
      this.nodeLoadPromises.delete(node.path);
    }
  }

  private async populateNodeChildren(node: TreeNode): Promise<void> {
    node.loading = true;
    node.children = node.children?.length ? node.children : [];
    this.cdr.markForCheck();

    try {
      const ctx = node.archivePath ? parseArchivePath(node.archivePath) : null;

      const listing = ctx
        ? await this.apiService.archiveBrowse(ctx.fsPath, ctx.nestedChain, ctx.innerPath)
        : await this.apiService.listDirectory(node.path, this.showHidden);
      const childLevel = (node.level ?? 0) + 1;
      const existingChildren = new Map<string, TreeNode>();
      if (node.children) {
        for (const child of node.children) {
          existingChildren.set(this.normalizePath(child.path), child);
        }
      }

      node.children = (listing.entries || [])
        .filter(e => e.path !== PARENT_ENTRY_PATH)
        .map(e => {
          const entryNestedChain = (e as any).nestedChain as string[] | undefined;
          const isArchiveEntry = e.type === FileType.ARCHIVE;

          // Build the single archivePath that encodes the full context for this child.
          let childArchivePath: string | undefined;
          if (ctx) {
            if (isArchiveEntry) {
              // Entering a nested archive: chain grows, innerPath resets.
              const newChain = entryNestedChain?.length
                ? entryNestedChain
                : [...ctx.nestedChain, e.path];
              childArchivePath = buildArchivePath(ctx.fsPath, newChain, '');
            } else {
              // Navigating deeper inside the same archive.
              childArchivePath = buildArchivePath(ctx.fsPath, ctx.nestedChain, e.path);
            }
          } else {
            // Not inside an archive yet — only set archivePath for archive entries.
            childArchivePath = isArchiveEntry ? e.path : undefined;
          }

          const childPath = ctx
            ? `${node.path.replace(/\/+$/, '')}/${e.name}`.replace(/\/+/g, '/')
            : e.path;

          const existing = existingChildren.get(this.normalizePath(childPath));
          return {
            ...e,
            path: childPath,
            level: childLevel,
            expandable: e.type === FileType.DIRECTORY || e.type === FileType.ARCHIVE,
            expanded: existing?.expanded ?? false,
            children: existing?.children ?? ((e.type === FileType.DIRECTORY || e.type === FileType.ARCHIVE) ? [] : undefined),
            loaded: existing?.loaded ?? false,
            loading: false,
            parent: node,
            archivePath: childArchivePath,
          } as TreeNode;
        });

      node.loaded = true;
      node.loading = false;
      this.cdr.markForCheck();
    } catch {
      node.children = [];
      node.loading = false;
      node.loaded = true;
      this.cdr.markForCheck();
    }
  }

  private createRootNode(rootPath: string, archivePath?: string): TreeNode {
    const isWindowsRoot = /^[A-Za-z]:\/$/.test(rootPath);
    const rootName = archivePath
      ? rootPath.split('/').filter(Boolean).pop() || rootPath
      : (rootPath === '/' ? '/' : (isWindowsRoot ? rootPath.slice(0, 2) : rootPath));
    return {
      name: rootName,
      path: rootPath,
      type: FileType.DIRECTORY,
      size: 0,
      modified: 0,
      created: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
      level: -1,
      syntheticRoot: rootPath,
      expandable: true,
      expanded: true,
      children: [],
      loaded: false,
      loading: false,
      parent: undefined,
      archivePath,
    } as TreeNode;
  }

  private getRootPath(path: string): string {
    const normalized = this.normalizePath(path || '/');
    const winDrive = normalized.match(/^([A-Za-z]:)\//);
    if (winDrive) return `${winDrive[1]}/`;
    return '/';
  }

  private buildPathChain(rootPath: string, targetPath: string): string[] {
    const root = this.normalizePath(rootPath);
    const target = this.normalizePath(targetPath || rootPath);

    const chain = [root];
    const rootForPrefix = root.endsWith('/') ? root : `${root}/`;
    const startsAtRoot = target === root || this.normalizePath(target).startsWith(rootForPrefix);
    if (!startsAtRoot) return chain;
    if (target === root) return chain;

    const relative = target.slice(root.length).replace(/^\/+/, '');
    if (!relative) return chain;

    let current = root;
    for (const segment of relative.split('/').filter(Boolean)) {
      current = current === '/' ? `/${segment}` : `${current.replace(/\/$/, '')}/${segment}`;
      chain.push(this.normalizePath(current));
    }
    return chain;
  }

  /**
   * Reload the children of the tree node that corresponds to `path`.
   * Call this after a file operation so the tree stays in sync without
   * collapsing any already-expanded subtrees.
   */
  refreshTreeNode(path: string): void {
    const target = this.normalizePath(path);
    const node = this.findTreeNodeByPath(this.treeNodes, target);
    if (!node || !node.loaded) return;   // not yet loaded — nothing to refresh
    node.loaded = false;
    void this.ensureNodeChildrenLoaded(node);
  }

  private findTreeNodeByPath(nodes: TreeNode[], target: string): TreeNode | undefined {
    for (const n of nodes) {
      if (this.normalizePath(n.path) === target) return n;
      if (n.children) {
        const found = this.findTreeNodeByPath(n.children, target);
        if (found) return found;
      }
    }
    return undefined;
  }

  private normalizePath(path: string): string {
    const normalized = (path || '/').replace(/\\/g, '/').replace(/\/+/g, '/');
    // Always normalise Windows drive roots (with or without trailing slash) to "X:/"
    if (/^[A-Za-z]:\/?$/.test(normalized)) return `${normalized.slice(0, 2)}/`;
    if (normalized.length > 1 && normalized.endsWith('/')) return normalized.slice(0, -1);
    return normalized || '/';
  }
}
