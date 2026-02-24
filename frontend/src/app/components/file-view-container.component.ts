// file-view-container.component.ts
import { Component, Input, Output, EventEmitter, ViewChild, OnChanges, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileListComponent } from './file-list.component';
import { FileGridComponent } from './file-grid.component';
import { FileTreeComponent, TreeNode } from './file-tree.component';
import { FileDetailsComponent } from './file-details.component';
import { ViewSwitcherComponent, ViewMode } from './view-switcher.component';
import { FileInfo, FileType } from '../shared/protocol';
import { ApiService } from '../services/api.service';

export type ViewLayout = ViewMode;

@Component({
  selector: 'app-file-view-container',
  standalone: true,
  imports: [
    CommonModule,
    FileListComponent,
    FileGridComponent,
    FileTreeComponent,
    FileDetailsComponent,
    ViewSwitcherComponent
  ],
  template: `
    <div class="view-container">
      <app-view-switcher
        [currentView]="currentView"
        [currentSort]="sortField"
        [sortDirection]="sortDirection"
        (viewChange)="onViewChange($event)"
        (sortChange)="onSortChange($event)">
      </app-view-switcher>

      <div class="view-content">
        <app-file-list
          *ngIf="currentView === 'list'"
          [entries]="sortedEntries"
          [selectedFiles]="selectedFiles"
          [loading]="loading"
          [error]="error"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)">
        </app-file-list>

        <app-file-grid
          *ngIf="currentView === 'grid'"
          [entries]="sortedEntries"
          [selectedFiles]="selectedFiles"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)"
          (contextMenu)="onContextMenu($event)">
        </app-file-grid>

        <app-file-tree
          *ngIf="currentView === 'tree'"
          [nodes]="treeNodes"
          [selectedFiles]="selectedFiles"
          (selectionChange)="onSelectionChange($event)"
          (nodeDoubleClick)="onTreeNodeDoubleClick($event)"
          (toggleExpand)="onTreeToggleExpand($event)"
          (loadChildren)="onTreeLoadChildren($event)"
          (navigate)="onNavigate($event)">
        </app-file-tree>

        <app-file-details
          *ngIf="currentView === 'details'"
          [entries]="sortedEntries"
          [selectedFiles]="selectedFiles"
          [sortField]="sortField"
          [sortDirection]="sortDirection"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)"
          (sort)="onSortChange($event)"
          (contextMenu)="onContextMenu($event)">
        </app-file-details>
      </div>
    </div>
  `,
  styles: [`
    .view-container {
      display: flex;
      flex-direction: column;
      flex: 1;
      min-height: 0;
      overflow: hidden;
    }

    .view-content {
      flex: 1;
      min-height: 0;
      overflow: hidden;
      position: relative;
    }

    .view-content > * {
      display: flex;
      flex-direction: column;
      height: 100%;
    }
  `]
})
export class FileViewContainerComponent implements OnChanges {
  @ViewChild(FileTreeComponent) treeComponent?: FileTreeComponent;

  @Input() showHidden = false;
  @Input() entries: FileInfo[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  @Input() loading = false;
  @Input() error: string | null = null;
  @Input() currentView: ViewLayout = 'list';
  @Input() sortField: string = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';

  @Output() viewChange = new EventEmitter<ViewLayout>();
  @Output() selectionChange = new EventEmitter<FileInfo | FileInfo[]>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() sortChange = new EventEmitter<{ field: string; direction: 'asc' | 'desc' }>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() nodeDoubleClick = new EventEmitter<FileInfo>();

  sortedEntries: FileInfo[] = [];
  treeNodes: TreeNode[] = [];

  constructor(private apiService: ApiService, private cdr: ChangeDetectorRef) {}

  ngOnChanges(): void {
    this.sortEntries();
    if (this.currentView === 'tree') {
      this.buildTreeNodes();
    }
  }

  onViewChange(view: ViewLayout): void {
    this.currentView = view;
    this.viewChange.emit(view);
    localStorage.setItem('fileManager_viewMode', view);
    if (view === 'tree') {
      this.buildTreeNodes();
    }
  }

  onSortChange(sort: { field: string; direction: 'asc' | 'desc' }): void {
    this.sortField = sort.field;
    this.sortDirection = sort.direction;
    this.sortChange.emit(sort);
    this.sortEntries();
  }

  onSelectionChange(entryOrNodes: FileInfo | FileInfo[] | TreeNode[]): void {
    if (Array.isArray(entryOrNodes)) {
      const files: FileInfo[] = entryOrNodes.map((n: any) => ({
        name: n.name,
        path: n.path,
        type: n.type,
        size: n.size,
        modified: n.modified,
        created: n.created,
      } as FileInfo));
      this.selectionChange.emit(files);
    } else {
      this.selectionChange.emit(entryOrNodes as FileInfo);
    }
  }

  onNavigate(entry: FileInfo | TreeNode | Event): void {
    if (!entry || (entry as Event) instanceof Event) return;
    const e = entry as any;
    if (e.path) {
      this.navigate.emit({
        name: e.name,
        path: e.path,
        type: e.type,
        size: e.size,
        modified: e.modified,
        created: e.created,
      } as FileInfo);
    }
  }

  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void {
    this.contextMenu.emit(event);
  }

  onTreeNodeDoubleClick(node: TreeNode): void {
    if (node.type !== FileType.DIRECTORY) {
      this.nodeDoubleClick.emit(node);
    }
  }

  onTreeToggleExpand(_node: TreeNode): void {
    // Tree manages its own expansion state; nothing extra needed here.
  }

  /**
   * Lazy-load children for a tree node.
   *
   * Called by FileTreeComponent when it wants to populate an expanded directory.
   * The tree has already added the node to expansionState=true and emitted this event.
   * We own the loading flag here — we must NOT double-set it to avoid the infinite-loader bug.
   */
  onTreeLoadChildren(node: TreeNode): void {
    // Guard: skip if already loading or already loaded
    // If already loaded, skip. Allow loading=true (set by tree) so we proceed to fetch.
    if (node.loaded) return;

    // No loader: request children and update when ready.

    this.apiService.listDirectory(node.path, this.showHidden)
      .then(listing => {
        const childLevel = (node.level ?? 0) + 1;
        const children: TreeNode[] = (listing.entries || []).map(e => ({
          ...e,
          level: childLevel,
          expandable: e.type === FileType.DIRECTORY,
          expanded: false,
          children: e.type === FileType.DIRECTORY ? [] : undefined,
          loaded: false,
          loading: false,
          parent: node,
        } as TreeNode));

        node.children = children;
        node.loaded = true;
        node.loading = false;
        this.cdr.markForCheck();
      })
      .catch(() => {
        node.children = [];
        node.loading = false;
        node.loaded = true;
        this.cdr.markForCheck();
      });
  }

  // ── Sorting ──────────────────────────────────────────────

  private sortEntries(): void {
    const sorted = [...this.entries];
    const multiplier = this.sortDirection === 'asc' ? 1 : -1;

    sorted.sort((a, b) => {
      if (a.path === '__archive_parent__') return -1;
      if (b.path === '__archive_parent__') return 1;

      if (this.currentView !== 'grid') {
        if (a.type === FileType.DIRECTORY && b.type !== FileType.DIRECTORY) return -1;
        if (b.type === FileType.DIRECTORY && a.type !== FileType.DIRECTORY) return 1;
      }

      let comparison = 0;
      switch (this.sortField) {
        case 'name':
          comparison = a.name.localeCompare(b.name);
          break;
        case 'size':
          comparison = (a.size || 0) - (b.size || 0);
          break;
        case 'modified':
          comparison = (a.modified || 0) - (b.modified || 0);
          break;
        case 'type': {
          const typeA = a.type === FileType.DIRECTORY ? 'folder' : (a.name.split('.').pop() || '');
          const typeB = b.type === FileType.DIRECTORY ? 'folder' : (b.name.split('.').pop() || '');
          comparison = typeA.localeCompare(typeB);
          break;
        }
        default:
          comparison = a.name.localeCompare(b.name);
      }

      return comparison * multiplier;
    });

    this.sortedEntries = sorted;
  }

  // ── Tree building (root level only) ──────────────────────

  private buildTreeNodes(): void {
    const root: TreeNode[] = [];
    const map = new Map<string, TreeNode>();

    const sorted = [...this.entries].sort((a, b) => {
      const depthA = a.path.replace(/\\/g, '/').split('/').length;
      const depthB = b.path.replace(/\\/g, '/').split('/').length;
      return depthA - depthB;
    });

    for (const entry of sorted) {
      if (entry.path === '__archive_parent__') continue;

      const node: TreeNode = {
        ...entry,
        level: 0,
        expandable: entry.type === FileType.DIRECTORY,
        expanded: false,
        children: entry.type === FileType.DIRECTORY ? [] : undefined,
        loaded: false,
        loading: false,
      };

      map.set(entry.path, node);

      const parentPath = this.getParentPath(entry.path);
      if (parentPath && map.has(parentPath)) {
        const parent = map.get(parentPath)!;
        if (!parent.children) parent.children = [];
        parent.children.push(node);
        node.parent = parent;
        node.level = parent.level + 1;
      } else {
        root.push(node);
      }
    }

    this.treeNodes = root;
  }

  private getParentPath(path: string): string | null {
    if (path === '/' || path === '') return null;
    const fixed = path.replace(/\\/g, '/');
    const parts = fixed.split('/').filter(p => p);
    if (parts.length === 0) return null;
    parts.pop();
    return '/' + parts.join('/');
  }
}