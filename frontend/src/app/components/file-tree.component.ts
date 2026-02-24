// file-tree.component.ts
import { Component, Input, Output, EventEmitter, OnChanges, SimpleChanges, TrackByFunction } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';
import { DEFAULT_FILE_CATEGORIES } from './filter-bar.types';
import { PARENT_ENTRY_PATH } from './file-entry.component';

export interface TreeNode extends FileInfo {
  level: number;
  expanded?: boolean;
  expandable?: boolean;
  children?: TreeNode[];
  parent?: TreeNode;
  loaded?: boolean;
  loading?: boolean;
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
      
      <ng-container *ngTemplateOutlet="treeTemplate; context: { nodes: nodes, level: 0 }"></ng-container>

      <div *ngIf="nodes.length === 0" class="tree-empty">
        No items
      </div>
    </div>

    <ng-template #treeTemplate let-nodes="nodes" let-level="level">
      <div *ngFor="let node of nodes; trackBy: trackByFn" class="tree-node-wrapper">
        <div 
          class="tree-node"
          [class.selected]="isSelected(node)"
          [class.focused]="isFocusedNode(node)"
          [class.expanded]="isExpanded(node)"
          [class.collapsed]="!isExpanded(node) && node.expandable"
          [class.loading]="node.loading"
          [style.padding-left.px]="getIndentation(level)"
          [attr.role]="'treeitem'"
          [attr.aria-level]="level + 1"
          [attr.aria-expanded]="node.expandable ? isExpanded(node) : undefined"
          [attr.aria-selected]="isSelected(node)"
          [attr.data-path]="node.path"
          (click)="onNodeClick(node, $event)"
          (dblclick)="onNodeDoubleClick(node)"
          (contextmenu)="onContextMenu($event, node)">

          <!-- Indentation guides -->
          <div class="indent-guides" *ngIf="level > 0">
            <div 
              *ngFor="let guide of getIndentGuides(level); let i = index"
              class="indent-guide"
              [class.active]="isGuideActive(node, i, level)">
            </div>
          </div>

          <!-- Twistie -->
          <span 
            class="tree-twistie"
            [class.expandable]="node.expandable"
            [class.expanded]="isExpanded(node)"
            [class.visible]="node.expandable"
            (click)="onToggleExpand($event, node)">
            <svg *ngIf="node.expandable" width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path d="M6 4L10 8L6 12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <span *ngIf="!node.expandable" class="twistie-placeholder"></span>
          </span>

          <!-- Icon -->
          <span class="tree-icon">
            <ng-container *ngIf="node.type === FileType.DIRECTORY">
              <span class="folder-icon">{{ isExpanded(node) ? '📂' : '📁' }}</span>
            </ng-container>
            <ng-container *ngIf="node.type === FileType.SYMLINK">
              <span class="symlink-icon">🔗</span>
            </ng-container>
            <ng-container *ngIf="node.type !== FileType.DIRECTORY && node.type !== FileType.SYMLINK">
              <span class="file-icon">{{ getCategoryIcon(node.name) }}</span>
            </ng-container>
          </span>

          <!-- Label -->
          <span class="tree-label" [title]="node.path">{{ node.name }}</span>

          <!-- Loading spinner -->
          <span class="tree-spinner" *ngIf="node.loading">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <circle cx="8" cy="8" r="6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-dasharray="20 10">
                <animateTransform attributeName="transform" type="rotate" from="0 8 8" to="360 8 8" dur="1s" repeatCount="indefinite"/>
              </circle>
            </svg>
          </span>
        </div>

        <!-- Children - only render if expanded -->
        <div class="tree-children" *ngIf="isExpanded(node) && node.children?.length">
          <ng-container *ngTemplateOutlet="treeTemplate; context: { nodes: node.children, level: level + 1 }"></ng-container>
        </div>
      </div>
    </ng-template>
  `,
  styles: [`
    :host { display: block; flex: 1; min-height: 0; overflow: hidden; }
    .file-tree { overflow-y: auto; overflow-x: hidden; background: var(--vsc-editor-background); font-family: var(--vsc-font-family); font-size: var(--vsc-font-size); line-height: 22px; outline: none; height: 100%; user-select: none; }
    .tree-node-wrapper { position: relative; }
    .tree-node { display: flex; align-items: center; height: 22px; cursor: pointer; position: relative; color: var(--vsc-foreground); transition: background-color 0.1s ease; white-space: nowrap; }
    .tree-node.selected { background-color: var(--vsc-list-active-background); color: var(--vsc-foreground-bright); }
    .file-tree.focused .tree-node.selected { outline: 1px solid var(--vsc-focus-border); outline-offset: -1px; }
    .tree-node:hover:not(.selected) { background-color: var(--vsc-list-hover-background); }
    .tree-node.focused { outline: 1px dotted var(--vsc-focus-border); outline-offset: -1px; }
    .indent-guides { display: flex; position: absolute; left: 0; top: 0; bottom: 0; pointer-events: none; z-index: 0; }
    .indent-guide { width: 20px; height: 100%; position: relative; }
    .indent-guide::before { content: ''; position: absolute; left: 9px; top: 0; bottom: 0; width: 1px; background-color: var(--vsc-tree-indent-guide); opacity: 0.3; }
    .indent-guide.active::before { background-color: var(--vsc-tree-indent-guide-active); opacity: 0.8; }
    .tree-twistie { width: 20px; height: 22px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; color: var(--vsc-foreground-dim); z-index: 1; }
    .tree-twistie.expandable { cursor: pointer; opacity: 0.7; }
    .tree-twistie.expandable:hover { opacity: 1; background-color: var(--vsc-list-hover-background); border-radius: 3px; }
    .tree-twistie.expandable.expanded { opacity: 1; }
    .tree-twistie svg { transition: transform 0.15s ease; width: 16px; height: 16px; }
    .tree-twistie.expanded svg { transform: rotate(90deg); }
    .twistie-placeholder { width: 16px; height: 16px; }
    .tree-icon { width: 20px; height: 22px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; font-size: 16px; z-index: 1; }
    .tree-label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding-right: 12px; font-size: var(--vsc-font-size); margin-left: 2px; z-index: 1; }
    .tree-spinner { margin-left: auto; margin-right: 8px; width: 16px; height: 16px; color: var(--vsc-foreground-dim); z-index: 1; }
    .tree-children { position: relative; }
    .tree-empty { padding: 12px; color: var(--vsc-foreground-dim); font-style: italic; text-align: center; }
  `]
})
export class FileTreeComponent implements OnChanges {
  @Input() nodes: TreeNode[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  @Input() indentSize = 20;
  
  @Output() selectionChange = new EventEmitter<TreeNode>();
  @Output() nodeDoubleClick = new EventEmitter<TreeNode>();
  @Output() toggleExpand = new EventEmitter<TreeNode>();
  @Output() navigate = new EventEmitter<TreeNode>();

  // Internal expansion state - persists across re-renders
  private expansionState = new Map<string, boolean>();
  
  private flattenedNodes: TreeNode[] = [];
  focusedNodeIndex = -1;
  isFocused = false;
  FileType = FileType;

  ngOnChanges(changes: SimpleChanges) {
    if (changes['nodes']) {
      this.updateNodeHierarchy(this.nodes, undefined, 0);
      this.refreshFlattenedNodes();
    }
    
    if (changes['selectedFiles']) {
      this.refreshFlattenedNodes();
    }
  }

  /**
   * Check if a node is expanded using internal state
   */
  isExpanded(node: TreeNode): boolean {
    // Use internal state if available, otherwise use node.expanded or default to false
    if (this.expansionState.has(node.path)) {
      return this.expansionState.get(node.path)!;
    }
    return node.expanded ?? false;
  }

  /**
   * Set expansion state for a node
   */
  private setExpanded(node: TreeNode, expanded: boolean): void {
    this.expansionState.set(node.path, expanded);
    node.expanded = expanded;
  }

  private updateNodeHierarchy(nodes: TreeNode[], parent?: TreeNode, level: number = 0): void {
    for (const node of nodes) {
      node.parent = parent;
      node.level = level;
      // Restore expansion state from internal map if exists
      if (this.expansionState.has(node.path)) {
        node.expanded = this.expansionState.get(node.path)!;
      }
      if (node.children) {
        this.updateNodeHierarchy(node.children, node, level + 1);
      }
    }
  }

  refreshFlattenedNodes(): void {
    const result: TreeNode[] = [];
    const flatten = (nodes: TreeNode[]) => {
      for (const node of nodes) {
        result.push(node);
        // Only include children in flattened list if expanded
        if (this.isExpanded(node) && node.children) {
          flatten(node.children);
        }
      }
    };
    flatten(this.nodes);
    this.flattenedNodes = result;
  }

  getIndentation(level: number): number { 
    return 4 + level * this.indentSize; 
  }
  
  getIndentGuides(level: number): number[] { 
    return Array.from({ length: level }, (_, i) => i); 
  }
  
  getCategoryIcon(filename: string): string {
    const ext = filename.split('.').pop()?.toLowerCase() || '';
    const cat = DEFAULT_FILE_CATEGORIES.find(c => c.extensions.includes(ext));
    return cat ? cat.icon : '📄';
  }

  isGuideActive(node: TreeNode, guideIndex: number, currentLevel: number): boolean {
    const targetPaths = Array.from(this.selectedFiles);
    if (targetPaths.length === 0) return false;
    
    for (const targetPath of targetPaths) {
      let current: TreeNode | undefined = this.findNodeByPath(this.nodes, targetPath);
      while (current && current.level > currentLevel) {
        current = current.parent;
      }
      if (current === node && guideIndex === currentLevel - 1) {
        return true;
      }
    }
    return false;
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
          this.toggleSelection(node, event.ctrlKey || event.metaKey, event.shiftKey);
        }
        break;
    }
  }

  private navigateDown(): void {
    if (this.focusedNodeIndex < this.flattenedNodes.length - 1) {
      this.focusedNodeIndex++;
      this.ensureFocusVisible();
    }
  }

  private navigateUp(): void {
    if (this.focusedNodeIndex > 0) {
      this.focusedNodeIndex--;
      this.ensureFocusVisible();
    }
  }

  private handleArrowRight(): void {
    if (this.focusedNodeIndex < 0) return;
    const node = this.flattenedNodes[this.focusedNodeIndex];
    
    if (node.expandable) {
      if (!this.isExpanded(node)) {
        this.expandNode(node);
      } else if (node.children?.length) {
        const firstChildIndex = this.flattenedNodes.findIndex(n => n.parent === node);
        if (firstChildIndex > this.focusedNodeIndex) {
          this.focusedNodeIndex = firstChildIndex;
          this.ensureFocusVisible();
        }
      }
    }
  }

  private handleArrowLeft(): void {
    if (this.focusedNodeIndex < 0) return;
    const node = this.flattenedNodes[this.focusedNodeIndex];
    
    if (node.expandable && this.isExpanded(node)) {
      this.collapseNode(node);
    } else if (node.parent) {
      const parentIndex = this.flattenedNodes.findIndex(n => n === node.parent);
      if (parentIndex >= 0) {
        this.focusedNodeIndex = parentIndex;
        this.ensureFocusVisible();
      }
    }
  }

  private handleEnter(node: TreeNode): void {
    if (node.type === FileType.DIRECTORY) {
      if (node.expandable) {
        if (this.isExpanded(node)) {
          this.collapseNode(node);
        } else {
          this.expandNode(node);
        }
      }
      this.navigate.emit(node);
    } else {
      this.nodeDoubleClick.emit(node);
    }
  }

  private expandNode(node: TreeNode): void {
    this.setExpanded(node, true);
    this.toggleExpand.emit(node);
    this.refreshFlattenedNodes();
  }

  private collapseNode(node: TreeNode): void {
    this.setExpanded(node, false);
    this.toggleExpand.emit(node);
    this.refreshFlattenedNodes();
  }

  private ensureFocusVisible(): void {
    setTimeout(() => {
      const focusedElement = document.querySelector('.tree-node.focused');
      if (focusedElement) {
        focusedElement.scrollIntoView({ block: 'nearest' });
      }
    });
  }

  onContextMenu(event: MouseEvent, node: TreeNode): void {
    event.preventDefault();
    event.stopPropagation();
    const index = this.flattenedNodes.findIndex(n => n.path === node.path);
    if (index >= 0) {
      this.focusedNodeIndex = index;
    }
    if (!this.selectedFiles.has(node.path)) {
      this.toggleSelection(node, false, false);
    }
  }

  onToggleExpand(event: MouseEvent, node: TreeNode): void { 
    event.stopPropagation();
    if (!node.expandable) return;
    
    if (this.isExpanded(node)) {
      this.collapseNode(node);
    } else {
      this.expandNode(node);
    }
  }

  onNodeClick(node: TreeNode, event: MouseEvent): void {
    event.stopPropagation();
    
    const index = this.flattenedNodes.findIndex(n => n.path === node.path);
    if (index >= 0) {
      this.focusedNodeIndex = index;
    }
    
    this.toggleSelection(node, event.ctrlKey || event.metaKey, event.shiftKey);
  }

  private toggleSelection(node: TreeNode, multiSelect: boolean, rangeSelect: boolean): void {
    if (rangeSelect && this.focusedNodeIndex >= 0) {
      const startIndex = this.focusedNodeIndex;
      const endIndex = this.flattenedNodes.findIndex(n => n.path === node.path);
      const [min, max] = [Math.min(startIndex, endIndex), Math.max(startIndex, endIndex)];
      
      for (let i = min; i <= max; i++) {
        const n = this.flattenedNodes[i];
        if (n.path !== PARENT_ENTRY_PATH) {
          this.selectionChange.emit(n);
        }
      }
    } else if (multiSelect) {
      this.selectionChange.emit(node);
    } else {
      // Single selection - toggle this node only
      this.selectionChange.emit(node);
    }
  }

  onNodeDoubleClick(node: TreeNode): void {
    if (node.type === FileType.DIRECTORY) {
      // Toggle expansion
      if (node.expandable) {
        if (this.isExpanded(node)) {
          this.collapseNode(node);
        } else {
          this.expandNode(node);
        }
      }
      this.navigate.emit(node);
    } else {
      this.nodeDoubleClick.emit(node);
    }
  }

  isSelected(node: TreeNode): boolean {
    return this.selectedFiles.has(node.path);
  }

  isFocusedNode(node: TreeNode): boolean {
    return this.flattenedNodes[this.focusedNodeIndex]?.path === node.path;
  }

  trackByFn: TrackByFunction<TreeNode> = (_, node) => node.path;
}