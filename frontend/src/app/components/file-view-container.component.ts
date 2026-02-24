// file-view-container.component.ts
import {
  Component, Input, Output, EventEmitter,
  ViewChild, OnChanges, OnDestroy, ChangeDetectorRef
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { Subject, merge } from 'rxjs';
import { groupBy, mergeMap, exhaustMap, takeUntil } from 'rxjs/operators';
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
          #fileTree
          *ngIf="currentView === 'tree'"
          [nodes]="treeNodes"
          [selectedFiles]="selectedFiles"
          (selectionChange)="onSelectionChange($event)"
          (nodeDoubleClick)="onTreeNodeDoubleClick($event)"
          (toggleExpand)="onTreeToggleExpand($event)"
          (loadChildren)="onLoadChildren($event)"
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
export class FileViewContainerComponent implements OnChanges, OnDestroy {
  @ViewChild('fileTree') treeComponent?: FileTreeComponent;

  @Input() showHidden    = false;
  @Input() entries: FileInfo[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  @Input() loading       = false;
  @Input() error: string | null = null;
  @Input() currentView: ViewLayout = 'list';
  @Input() sortField     = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';

  @Output() viewChange      = new EventEmitter<ViewLayout>();
  @Output() selectionChange = new EventEmitter<FileInfo | FileInfo[]>();
  @Output() navigate        = new EventEmitter<FileInfo>();
  @Output() sortChange      = new EventEmitter<{ field: string; direction: 'asc' | 'desc' }>();
  @Output() contextMenu     = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() nodeDoubleClick = new EventEmitter<FileInfo>();

  sortedEntries: FileInfo[] = [];
  treeNodes: TreeNode[] = [];

  /**
   * Hot subject that receives every loadChildren request from the tree.
   *
   * We group by node path so each directory gets its own exhaustMap lane:
   * - If a load for /foo is in flight and another /foo request arrives, it's dropped.
   * - Requests for /bar proceed independently.
   */
  private loadRequest$ = new Subject<TreeNode>();
  private destroy$     = new Subject<void>();

  constructor(private apiService: ApiService, private cdr: ChangeDetectorRef) {
    this.loadRequest$
      .pipe(
        // One lane per directory path
        groupBy(node => node.path),
        mergeMap(group$ =>
          group$.pipe(
            // Within each lane: ignore new requests while one is in flight
            exhaustMap(node => this.loadNodeChildren(node))
          )
        ),
        takeUntil(this.destroy$)
      )
      .subscribe();
  }

  ngOnChanges(): void {
    this.sortEntries();
    if (this.currentView === 'tree') {
      this.buildTreeNodes();
    }
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  // ── Load children via RxJS pipeline ───────────────────────

  /** Called from the tree's (loadChildren) output — just push into the stream. */
  onLoadChildren(node: TreeNode): void {
    this.loadRequest$.next(node);
  }

  /**
   * Performs the actual API call for one node.
   * Sets node.loading synchronously so the spinner appears immediately,
   * then resolves the promise into an Observable so exhaustMap can manage it.
   */
  private loadNodeChildren(node: TreeNode) {
    // Set spinner immediately — this is synchronous, before any async work
    node.loading = true;
    node.children = node.children?.length ? node.children : [];
    this.cdr.markForCheck();

    // We return a Subject that completes when the promise settles.
    // exhaustMap subscribes to it and won't accept another request for this
    // node path until the Subject completes (i.e. until the load finishes).
    const result$ = new Subject<void>();

    this.apiService
      .listDirectory(node.path, this.showHidden)
      .then(listing => {
        const childLevel = (node.level ?? 0) + 1;
        node.children = (listing.entries || []).map(e => ({
          ...e,
          level: childLevel,
          expandable: e.type === FileType.DIRECTORY,
          expanded: false,
          children: e.type === FileType.DIRECTORY ? [] : undefined,
          loaded: false,
          loading: false,
          parent: node,
        } as TreeNode));
        node.loaded  = true;
        node.loading = false;
        this.cdr.markForCheck();
      })
      .catch(() => {
        node.children = [];
        node.loading  = false;
        node.loaded   = true;
        this.cdr.markForCheck();
      })
      .finally(() => {
        result$.next();
        result$.complete();
      });

    return result$;
  }

  // ── View / sort ────────────────────────────────────────────

  onViewChange(view: ViewLayout): void {
    this.currentView = view;
    this.viewChange.emit(view);
    localStorage.setItem('fileManager_viewMode', view);
    if (view === 'tree') this.buildTreeNodes();
  }

  onSortChange(sort: { field: string; direction: 'asc' | 'desc' }): void {
    this.sortField     = sort.field;
    this.sortDirection = sort.direction;
    this.sortChange.emit(sort);
    this.sortEntries();
  }

  // ── Event passthrough ──────────────────────────────────────

  onSelectionChange(entryOrNodes: FileInfo | FileInfo[] | TreeNode[]): void {
    if (Array.isArray(entryOrNodes)) {
      const files: FileInfo[] = entryOrNodes.map((n: any) => ({
        name: n.name, path: n.path, type: n.type,
        size: n.size, modified: n.modified, created: n.created,
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
        name: e.name, path: e.path, type: e.type,
        size: e.size, modified: e.modified, created: e.created,
      } as FileInfo);
    }
  }

  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void {
    this.contextMenu.emit(event);
  }

  onTreeNodeDoubleClick(node: TreeNode): void {
    if (node.type !== FileType.DIRECTORY) this.nodeDoubleClick.emit(node);
  }

  onTreeToggleExpand(_node: TreeNode): void {
    // Tree manages expansion state internally; nothing extra needed.
  }

  // ── Sorting ────────────────────────────────────────────────

  private sortEntries(): void {
    const sorted     = [...this.entries];
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
    const root: TreeNode[] = [];
    const map = new Map<string, TreeNode>();

    const sorted = [...this.entries].sort((a, b) => {
      const da = a.path.replace(/\\/g, '/').split('/').length;
      const db = b.path.replace(/\\/g, '/').split('/').length;
      return da - db;
    });

    for (const entry of sorted) {
      if (entry.path === '__archive_parent__') continue;

      const node: TreeNode = {
        ...entry,
        level:      0,
        expandable: entry.type === FileType.DIRECTORY,
        expanded:   false,
        children:   entry.type === FileType.DIRECTORY ? [] : undefined,
        loaded:     false,
        loading:    false,
      };

      map.set(entry.path, node);

      const parentPath = this.getParentPath(entry.path);
      if (parentPath && map.has(parentPath)) {
        const parent = map.get(parentPath)!;
        if (!parent.children) parent.children = [];
        parent.children.push(node);
        node.parent = parent;
        node.level  = parent.level + 1;
      } else {
        root.push(node);
      }
    }

    this.treeNodes = root;
  }

  private getParentPath(path: string): string | null {
    if (path === '/' || path === '') return null;
    const parts = path.replace(/\\/g, '/').split('/').filter(Boolean);
    if (parts.length === 0) return null;
    parts.pop();
    return '/' + parts.join('/');
  }
}