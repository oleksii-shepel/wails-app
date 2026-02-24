// file-tree.component.ts
import { Component, Input, Output, EventEmitter, OnChanges, SimpleChanges, TrackByFunction } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';
import { DEFAULT_FILE_CATEGORIES } from './filter-bar.types';

export interface FileCategory {
  id: string;
  name: string;
  icon: string;
  extensions: string[];
}

export interface TreeNode extends FileInfo {
  level: number;
  expanded?: boolean;
  expandable?: boolean;
  children?: TreeNode[];
  parent?: TreeNode;
  loaded?: boolean;
  loading?: boolean;
  selected?: boolean;
  focused?: boolean;
}

@Component({
  selector: 'app-file-tree',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div 
      class="file-tree" 
      role="tree" 
      tabindex="0"
      [class.focused]="isFocused"
      (focus)="onFocus()"
      (blur)="onBlur()"
      (keydown)="onKeyDown($event)">
      
      <div 
        *ngFor="let node of flattenedNodes; trackBy: trackByFn"
        class="tree-node"
        [class.selected]="isSelected(node)"
        [class.focused]="isFocusedNode(node)"
        [class.expanded]="node.expanded"
        [class.collapsed]="!node.expanded && node.expandable"
        [class.loading]="node.loading"
        [style.padding-left.px]="getIndentation(node.level)"
        [attr.role]="'treeitem'"
        [attr.aria-expanded]="node.expandable ? node.expanded : undefined"
        [attr.aria-selected]="isSelected(node)"
        [attr.data-level]="node.level"
        (click)="onNodeClick(node, $event)"
        (dblclick)="onNodeDoubleClick(node)"
        (contextmenu)="onContextMenu($event, node)">

        <!-- Indentation guides -->
        <div class="indent-guides">
          <div 
            *ngFor="let guide of getIndentGuides(node.level); let i = index"
            class="indent-guide"
            [class.active]="isGuideActive(node, i)">
          </div>
        </div>

        <!-- Twistie (expand/collapse arrow) -->
        <span 
          class="tree-twistie"
          [class.expandable]="node.expandable"
          [class.expanded]="node.expanded"
          [class.collapsed]="!node.expanded && node.expandable"
          (click)="onToggleExpand($event, node)">
          <svg *ngIf="node.expandable" width="16" height="16" viewBox="0 0 16 16" fill="none">
            <path d="M6 4L10 8L6 12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </span>

        <!-- Icon -->
        <span class="tree-icon">
          <ng-container *ngIf="node.type === FileType.DIRECTORY">
            {{ node.expanded ? '📂' : '📁' }}
          </ng-container>
          <ng-container *ngIf="node.type === FileType.SYMLINK">🔗</ng-container>
          <ng-container *ngIf="node.type !== FileType.DIRECTORY && node.type !== FileType.SYMLINK">
            {{ getCategoryIcon(node.name) }}
          </ng-container>
        </span>

        <!-- Label -->
        <span class="tree-label" [title]="node.name">{{ node.name }}</span>

        <!-- Loading Spinner -->
        <span class="tree-spinner" *ngIf="node.loading">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <circle cx="8" cy="8" r="6" stroke="var(--vsc-foreground-dim)" stroke-width="1.5" stroke-linecap="round" stroke-dasharray="20 10">
              <animateTransform attributeName="transform" type="rotate" from="0 8 8" to="360 8 8" dur="1s" repeatCount="indefinite"/>
            </circle>
          </svg>
        </span>
      </div>
    </div>
  `,
  styles: [`
    :host { display:block; flex:1; min-height:0; overflow:hidden; }
    .file-tree { overflow-y:auto; background: var(--vsc-editor-background); font-family: var(--vsc-font-family); font-size: var(--vsc-font-size); line-height: var(--vsc-line-height); outline: none; }
    .tree-node { display:flex; align-items:center; height:22px; cursor:pointer; position:relative; color: var(--vsc-foreground); transition: background-color 0.1s ease; }
    .tree-node.selected { background-color: var(--vsc-list-active-background); color: var(--vsc-foreground-bright); }
    .file-tree.focused .tree-node.selected { outline:1px solid var(--vsc-accent-blue); outline-offset:-1px; }
    .tree-node:hover:not(.selected) { background-color: var(--vsc-list-hover-background); }
    .tree-node.focused { outline:1px dotted var(--vsc-border-bright); outline-offset:-1px; }
    
    /* Indentation guides */
    .indent-guides { display:flex; position:absolute; left:0; top:0; bottom:0; pointer-events:none; }
    .indent-guide { width:20px; height:100%; position:relative; }
    .indent-guide::before { content:''; position:absolute; left:9.5px; top:0; bottom:0; width:1px; background-color: var(--vsc-border-subtle); opacity:0.3; }
    .indent-guide.active::before { background-color: var(--vsc-border-bright); opacity:0.8; }
    
    /* Twistie */
    .tree-twistie { width:16px; height:16px; display:flex; align-items:center; justify-content:center; flex-shrink:0; margin-right:2px; opacity:0; transition:opacity 0.15s ease; }
    .tree-node:hover .tree-twistie.expandable, .tree-twistie.expandable { opacity:1; cursor:pointer; border-radius:3px; }
    .tree-twistie:hover { background-color: var(--vsc-list-hover-background); }
    .tree-twistie svg { color: var(--vsc-foreground-dim); transition:transform 0.15s ease; width: 16px; height: 16px; }
    .tree-twistie.expanded svg { transform: rotate(90deg); }
    .tree-twistie.expandable:hover svg { color: var(--vsc-foreground); }
    
    /* Icon */
    .tree-icon { width:16px; height:16px; display:flex; align-items:center; justify-content:center; flex-shrink:0; margin-right:4px; font-size: 14px; }
    
    /* Label */
    .tree-label { flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; padding-right:12px; font-size: var(--vsc-font-size); }
    
    /* Loading */
    .tree-spinner { margin-left:auto; margin-right:8px; width:16px; height:16px; }
  `]
})
export class FileTreeComponent implements OnChanges {
  @Input() nodes: TreeNode[] = [];
  @Input() selectedNode?: TreeNode;
  @Input() indentSize = 20;
  
  @Output() nodeClick = new EventEmitter<TreeNode>();
  @Output() nodeDoubleClick = new EventEmitter<TreeNode>();
  @Output() toggleExpand = new EventEmitter<TreeNode>();
  @Output() navigate = new EventEmitter<TreeNode>(); // Navigate into folder
  
  flattenedNodes: TreeNode[] = [];
  focusedNodeIndex = -1;
  isFocused = false;
  FileType = FileType;

  ngOnChanges(changes: SimpleChanges) {
    if (changes['nodes'] || changes['selectedNode']) {
      this.refreshFlattenedNodes();
      if (this.selectedNode) {
        this.focusedNodeIndex = this.flattenedNodes.findIndex(n => n.path === this.selectedNode?.path);
      }
    }
  }

  refreshFlattenedNodes(): void {
    const result: TreeNode[] = [];
    const flatten = (nodes: TreeNode[], parent?: TreeNode) => {
      for (const node of nodes) {
        node.parent = parent;
        result.push(node);
        // Only show children if expanded
        if (node.expandable && node.expanded && node.children) {
          flatten(node.children, node);
        }
      }
    };
    flatten(this.nodes);
    this.flattenedNodes = result;
  }

  getIndentation(level: number): number { 
    return 8 + level * this.indentSize; 
  }
  
  getIndentGuides(level: number): number[] { 
    return Array.from({ length: level }, (_, i) => i); 
  }
  
  getCategoryIcon(filename: string): string {
    const ext = filename.split('.').pop()?.toLowerCase() || '';
    const cat = DEFAULT_FILE_CATEGORIES.find(c => c.extensions.includes(ext));
    return cat ? cat.icon : '📄';
  }

  isGuideActive(node: TreeNode, guideIndex: number): boolean {
    // Highlight guide if this node is an ancestor of the selected/focused node
    const targetNode = this.selectedNode || this.flattenedNodes[this.focusedNodeIndex];
    if (!targetNode) return false;
    
    // Check if this guide represents an ancestor level of the target node
    let current: TreeNode | undefined = targetNode;
    while (current) {
      if (current.parent === node.parent && guideIndex === current.level - 1) {
        return true;
      }
      current = current.parent;
    }
    return false;
  }

  onFocus() { this.isFocused = true; }
  onBlur() { this.isFocused = false; }

  onKeyDown(event: KeyboardEvent) {
    if (this.flattenedNodes.length === 0) return;

    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        this.navigateDown();
        break;
      case 'ArrowUp':
        event.preventDefault();
        this.navigateUp();
        break;
      case 'ArrowRight':
        event.preventDefault();
        this.handleArrowRight();
        break;
      case 'ArrowLeft':
        event.preventDefault();
        this.handleArrowLeft();
        break;
      case 'Enter':
        event.preventDefault();
        if (this.focusedNodeIndex >= 0) {
          const node = this.flattenedNodes[this.focusedNodeIndex];
          this.handleEnter(node);
        }
        break;
      case ' ':
        event.preventDefault();
        if (this.focusedNodeIndex >= 0) {
          const node = this.flattenedNodes[this.focusedNodeIndex];
          this.onNodeClick(node, new MouseEvent('click'));
        }
        break;
    }
  }

  private navigateDown(): void {
    this.focusedNodeIndex = Math.min(this.focusedNodeIndex + 1, this.flattenedNodes.length - 1);
    this.emitSelection();
  }

  private navigateUp(): void {
    this.focusedNodeIndex = Math.max(this.focusedNodeIndex - 1, 0);
    this.emitSelection();
  }

  private handleArrowRight(): void {
    if (this.focusedNodeIndex < 0) return;
    const node = this.flattenedNodes[this.focusedNodeIndex];
    
    if (node.expandable && !node.expanded) {
      // Expand folder
      this.expandNode(node);
    } else if (node.expandable && node.expanded && node.children && node.children.length > 0) {
      // Navigate to first child
      this.navigateDown();
    }
  }

  private handleArrowLeft(): void {
    if (this.focusedNodeIndex < 0) return;
    const node = this.flattenedNodes[this.focusedNodeIndex];
    
    if (node.expandable && node.expanded) {
      // Collapse folder
      this.collapseNode(node);
    } else if (node.level > 0 && node.parent) {
      // Navigate to parent
      const parentIndex = this.flattenedNodes.findIndex(n => n === node.parent);
      if (parentIndex >= 0) {
        this.focusedNodeIndex = parentIndex;
        this.emitSelection();
      }
    }
  }

  private handleEnter(node: TreeNode): void {
    if (node.type === FileType.DIRECTORY) {
      // Toggle expansion on Enter for folders
      if (node.expandable) {
        if (node.expanded) {
          this.collapseNode(node);
        } else {
          this.expandNode(node);
        }
      }
    } else {
      // Open file on Enter
      this.nodeDoubleClick.emit(node);
    }
  }

  private expandNode(node: TreeNode): void {
    node.expanded = true;
    this.toggleExpand.emit(node);
    this.refreshFlattenedNodes();
  }

  private collapseNode(node: TreeNode): void {
    node.expanded = false;
    this.toggleExpand.emit(node);
    this.refreshFlattenedNodes();
  }

  private emitSelection(): void {
    if (this.focusedNodeIndex >= 0) {
      const node = this.flattenedNodes[this.focusedNodeIndex];
      this.nodeClick.emit(node);
    }
  }

  onContextMenu(event: MouseEvent, node: TreeNode): void {
    event.preventDefault();
    event.stopPropagation();
    this.focusedNodeIndex = this.flattenedNodes.findIndex(n => n.path === node.path);
    this.nodeClick.emit(node);
  }

  onToggleExpand(event: MouseEvent, node: TreeNode): void { 
    event.stopPropagation();
    if (!node.expandable) return;
    
    if (node.expanded) {
      this.collapseNode(node);
    } else {
      this.expandNode(node);
    }
  }

  onNodeClick(node: TreeNode, event: MouseEvent): void {
    event.stopPropagation();

    // Focus the node
    this.focusedNodeIndex = this.flattenedNodes.findIndex(n => n.path === node.path);
    
    // Select the node (emit click)
    this.nodeClick.emit(node);
  }

  onNodeDoubleClick(node: TreeNode): void {
    if (node.type === FileType.DIRECTORY) {
      // For folders, double-click toggles expansion (like VS Code)
      if (node.expandable) {
        if (node.expanded) {
          this.collapseNode(node);
        } else {
          this.expandNode(node);
        }
      }
      // Also emit navigate for folders if needed
      this.navigate.emit(node);
    } else {
      // For files, double-click opens them
      this.nodeDoubleClick.emit(node);
    }
  }

  isSelected(node: TreeNode): boolean {
    return this.selectedNode?.path === node.path;
  }

  isFocusedNode(node: TreeNode): boolean {
    return this.flattenedNodes[this.focusedNodeIndex]?.path === node.path;
  }

  trackByFn: TrackByFunction<TreeNode> = (_, n) => n.path;

  // Public API methods
  expandAll(): void {
    const expand = (nodes: TreeNode[]) => {
      nodes.forEach(node => {
        if (node.expandable) {
          node.expanded = true;
          if (node.children) expand(node.children);
        }
      });
    };
    expand(this.nodes);
    this.refreshFlattenedNodes();
  }

  collapseAll(): void {
    const collapse = (nodes: TreeNode[]) => {
      nodes.forEach(node => {
        if (node.expandable) {
          node.expanded = false;
          if (node.children) collapse(node.children);
        }
      });
    };
    collapse(this.nodes);
    this.refreshFlattenedNodes();
  }

  expandToLevel(level: number): void {
    const expand = (nodes: TreeNode[], currentLevel: number) => {
      nodes.forEach(node => {
        if (node.expandable && currentLevel < level) {
          node.expanded = true;
          if (node.children) expand(node.children, currentLevel + 1);
        }
      });
    };
    expand(this.nodes, 0);
    this.refreshFlattenedNodes();
  }

  revealNode(path: string): void {
    // Expand parents to make node visible
    const findAndExpand = (nodes: TreeNode[]): boolean => {
      for (const node of nodes) {
        if (node.path === path) return true;
        if (node.children) {
          const found = findAndExpand(node.children);
          if (found) {
            node.expanded = true;
            return true;
          }
        }
      }
      return false;
    };
    findAndExpand(this.nodes);
    this.refreshFlattenedNodes();
    
    // Focus the revealed node
    const index = this.flattenedNodes.findIndex(n => n.path === path);
    if (index >= 0) {
      this.focusedNodeIndex = index;
    }
  }
}