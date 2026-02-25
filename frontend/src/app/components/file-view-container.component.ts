// file-view-container.component.ts
// Add these new imports and update the component

import {
  Component, Input, Output, EventEmitter,
  ViewChild, OnChanges, OnDestroy, ChangeDetectorRef, SimpleChanges
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { Subject } from 'rxjs';
import { groupBy, mergeMap, exhaustMap, takeUntil } from 'rxjs/operators';
import { FileListComponent } from './file-list.component';
import { FileGridComponent } from './file-grid.component';
import { FileTreeComponent, TreeNode, DropTarget } from './file-tree.component';
import { FileDetailsComponent } from './file-details.component';
import { ViewMode } from './action-bar.component';
import { FileInfo, FileType } from '../shared/protocol';
import { ApiService } from '../services/api.service';
import { parentPath } from './address-bar.utils';

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
          (selectionChange)="onTreeSelectionChange($event)"
          (nodeDoubleClick)="onTreeNodeDoubleClick($event)"
          (toggleExpand)="onTreeToggleExpand($event)"
          (loadChildren)="onLoadChildren($event)"
          (navigate)="onNavigate($event)"
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
          (sort)="onSortChange($event)"
          (contextMenu)="onContextMenu($event)">
        </app-file-details>
      </div>
    </div>
  `,
  styles: [`/* ... keep existing styles ... */`]
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
  @Output() dragStart       = new EventEmitter<{ event: DragEvent; entries: FileInfo[] }>();
  @Output() dragDrop        = new EventEmitter<{ event: DragEvent; source: FileInfo[]; targetPath: string }>();
  @Output() rename          = new EventEmitter<{ entry: FileInfo; newName: string }>();

  sortedEntries: FileInfo[] = [];
  treeNodes: TreeNode[] = [];

  private loadRequest$ = new Subject<TreeNode>();
  private destroy$     = new Subject<void>();

  constructor(private apiService: ApiService, private cdr: ChangeDetectorRef) {
    this.loadRequest$
      .pipe(
        groupBy(node => node.path),
        mergeMap(group$ =>
          group$.pipe(
            exhaustMap(node => this.loadNodeChildren(node))
          )
        ),
        takeUntil(this.destroy$)
      )
      .subscribe();
  }

  ngOnChanges(changes: SimpleChanges): void {
    this.sortEntries();
    if (this.currentView === 'tree') {
      if (changes['entries']) {
        this.buildTreeNodes();
      }
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

  onTreeRename(event: { node: TreeNode; newName: string }): void {
    this.rename.emit({
      entry: this.treeNodeToFileInfo(event.node),
      newName: event.newName
    });
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

  private loadNodeChildren(node: TreeNode) {
    node.loading = true;
    node.children = node.children?.length ? node.children : [];
    this.cdr.markForCheck();

    const result$ = new Subject<void>();

    this.apiService
      .listDirectory(node.path, this.showHidden)
      .then(listing => {
        const childLevel = (node.level ?? 0) + 1;
        const existingChildren = new Map<string, TreeNode>();
        if (node.children) {
          for (const child of node.children) {
            existingChildren.set(child.path, child);
          }
        }

        node.children = (listing.entries || []).map(e => {
          const existing = existingChildren.get(e.path);
          return {
            ...e,
            level: childLevel,
            expandable: e.type === FileType.DIRECTORY,
            expanded: existing?.expanded ?? false,
            children: existing?.children ?? (e.type === FileType.DIRECTORY ? [] : undefined),
            loaded: existing?.loaded ?? false,
            loading: false,
            parent: node,
          } as TreeNode;
        });
        
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

  onSelectionChange(entryOrEntries: FileInfo | FileInfo[]): void {
    if (Array.isArray(entryOrEntries)) {
      this.selectedFiles = new Set(entryOrEntries.map(e => e.path));
      this.selectionChange.emit(entryOrEntries);
    } else {
      this.selectedFiles = new Set([entryOrEntries.path]);
      this.selectionChange.emit(entryOrEntries);
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
    const existingMap = new Map<string, TreeNode>();
    
    const traverseExisting = (nodes: TreeNode[]) => {
      for (const node of nodes) {
        existingMap.set(node.path, node);
        if (node.children) traverseExisting(node.children);
      }
    };
    traverseExisting(this.treeNodes);

    const sorted = [...this.entries].sort((a, b) => {
      const da = a.path.replace(/\\/g, '/').split('/').length;
      const db = b.path.replace(/\\/g, '/').split('/').length;
      return da - db;
    });

    for (const entry of sorted) {
      if (entry.path === '__archive_parent__') continue;

      const existing = existingMap.get(entry.path);
      
      const node: TreeNode = {
        ...entry,
        level: 0,
        expandable: entry.type === FileType.DIRECTORY,
        expanded: existing?.expanded ?? false,
        children: existing?.children ?? (entry.type === FileType.DIRECTORY ? [] : undefined),
        loaded: existing?.loaded ?? false,
        loading: existing?.loading ?? false,
        parent: undefined,
      };

      map.set(entry.path, node);

      const parent = parentPath(entry.path);
      const parentKey = parent !== entry.path ? parent : null;

      if (parentKey && map.has(parentKey)) {
        const parentNode = map.get(parentKey)!;
        if (!parentNode.children) parentNode.children = [];
        parentNode.children.push(node);
        node.parent = parentNode;
        node.level = parentNode.level + 1;
      } else {
        root.push(node);
      }
    }

    this.treeNodes = root;
  }
}