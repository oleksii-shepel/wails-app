// file-view-container.component.ts
import { Component, Input, Output, EventEmitter, ViewChild, OnChanges } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileListComponent } from './file-list.component';
import { FileGridComponent } from './file-grid.component';
import { FileTreeComponent, TreeNode } from './file-tree.component';
import { FileDetailsComponent } from './file-details.component';
import { ViewSwitcherComponent, ViewMode } from './view-switcher.component';
import { FileInfo, FileType } from '../shared/protocol';

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
        <!-- List View (Original) -->
        <app-file-list
          *ngIf="currentView === 'list'"
          [entries]="sortedEntries"
          [selectedFiles]="selectedFiles"
          [loading]="loading"
          [error]="error"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)">
        </app-file-list>
        
        <!-- Grid View -->
        <app-file-grid
          *ngIf="currentView === 'grid'"
          [entries]="sortedEntries"
          [selectedFiles]="selectedFiles"
          (selectionChange)="onSelectionChange($event)"
          (navigate)="onNavigate($event)"
          (contextMenu)="onContextMenu($event)">
        </app-file-grid>
        
        <!-- Tree View -->
        <app-file-tree
          *ngIf="currentView === 'tree'"
          [nodes]="treeNodes"
          [selectedNode]="selectedNode"
          (nodeClick)="onSelectionChange($event)"
          (toggleExpand)="onExpandNode($event)">
        </app-file-tree>
        
        <!-- Details View -->
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

    /* Ensure child components fill the container */
    .view-content > * {
      display: flex;
      flex-direction: column;
      height: 100%;
    }
  `],
})
export class FileViewContainerComponent implements OnChanges {
  @ViewChild(FileTreeComponent) treeComponent?: FileTreeComponent;
  
  @Input() entries: FileInfo[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  @Input() loading = false;
  @Input() error: string | null = null;
  @Input() currentView: ViewLayout = 'list';
  @Input() sortField: string = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';
  @Input() loadChildrenFn?: (node: TreeNode) => Promise<FileInfo[]>;

  @Output() viewChange = new EventEmitter<ViewLayout>();
  @Output() selectionChange = new EventEmitter<FileInfo>();
  @Output() navigate = new EventEmitter<FileInfo>();
  @Output() sortChange = new EventEmitter<{ field: string; direction: 'asc' | 'desc' }>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; entry: FileInfo }>();
  @Output() expandNode = new EventEmitter<TreeNode>();
  @Output() collapseNode = new EventEmitter<TreeNode>();

  sortedEntries: FileInfo[] = [];
  treeNodes: TreeNode[] = [];
  selectedNode?: TreeNode;

  ngOnChanges(): void {
    this.sortEntries();
    
    if (this.currentView === 'tree') {
      this.buildTreeNodes();

      // Update selectedNode based on selectedFiles
      const selectedPath = this.selectedFiles.values().next().value;
      this.selectedNode = selectedPath
        ? this.findNodeByPath(this.treeNodes, selectedPath)
        : undefined;
    }
  }

  onViewChange(view: ViewLayout): void {
    this.currentView = view;
    this.viewChange.emit(view);
    
    // Persist preference
    localStorage.setItem('fileManager_viewMode', view);
    
    // Rebuild tree if switching to tree view
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

  onSelectionChange(entry: FileInfo): void {
    this.selectionChange.emit(entry);
  }

  onNavigate(entry: FileInfo): void {
    this.navigate.emit(entry);
  }

  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void {
    this.contextMenu.emit(event);
  }

  onExpandNode(node: TreeNode): void {
    this.expandNode.emit(node);
  }

  onCollapseNode(node: TreeNode): void {
    this.collapseNode.emit(node);
  }

  private sortEntries(): void {
    const sorted = [...this.entries];
    const multiplier = this.sortDirection === 'asc' ? 1 : -1;
    
    sorted.sort((a, b) => {
      // Always keep parent entry at top
      if (a.path === '__archive_parent__') return -1;
      if (b.path === '__archive_parent__') return 1;
      
      // Directories first in list/details views
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
        case 'type':
          const typeA = a.type === FileType.DIRECTORY ? 'folder' : (a.name.split('.').pop() || '');
          const typeB = b.type === FileType.DIRECTORY ? 'folder' : (b.name.split('.').pop() || '');
          comparison = typeA.localeCompare(typeB);
          break;
        default:
          comparison = a.name.localeCompare(b.name);
      }
      
      return comparison * multiplier;
    });
    
    this.sortedEntries = sorted;
  }

  private buildTreeNodes(): void {
    const root: TreeNode[] = [];
    const map = new Map<string, TreeNode>();

    // Sort entries by path depth so parents come before children
    const sorted = [...this.entries].sort((a, b) => a.path.split('/').length - b.path.split('/').length);

    for (const entry of sorted) {
      if (entry.path === '__archive_parent__') continue;

      const node: TreeNode = {
        ...entry,
        level: 0,
        expandable: entry.type === FileType.DIRECTORY,
        expanded: false,
        children: entry.type === FileType.DIRECTORY ? [] : undefined,
        loaded: false
      };

      map.set(entry.path, node);

      // Determine parent path
      const parentPath = this.getParentPath(entry.path);

      if (parentPath && map.has(parentPath)) {
        const parent = map.get(parentPath)!;
        node.parent = parent;
        node.level = parent.level + 1;
        parent.children = parent.children || [];
        parent.children.push(node);
        parent.expandable = true; // ensure parent is marked expandable
      } else {
        root.push(node); // top-level node
      }
    }

    this.treeNodes = root;

    // Optional: auto-expand all parents so tree shows nested structure
    const expandParents = (nodes: TreeNode[]) => {
      nodes.forEach(n => {
        if (n.children && n.children.length > 0) {
          n.expanded = true;
          expandParents(n.children);
        }
      });
    };
    expandParents(root);
  }

  private getParentPath(path: string): string | null {
    if (path === '/' || path === '') return null;
    const parts = path.split('/').filter(p => p);
    if (parts.length === 0) return null;
    parts.pop();
    return '/' + parts.join('/');
  }

  private findNodeByPath(nodes: TreeNode[], path: string): TreeNode | undefined {
    for (const node of nodes) {
      if (node.path === path) return node;
      if (node.children) {
        const child = this.findNodeByPath(node.children, path);
        if (child) return child;
      }
    }
    return undefined;
  }
}