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

        <!-- Twistie -->
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
          <ng-container *ngIf="node.type === FileType.DIRECTORY">📁</ng-container>
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
    .file-tree { overflow-y:auto; background: var(--vsc-editor-background); font-family: var(--vsc-font-family); font-size: var(--vsc-font-size); line-height: var(--vsc-line-height); }
    .tree-node { display:flex; align-items:center; height:22px; cursor:pointer; position:relative; color: var(--vsc-foreground); transition: background-color 0.1s ease; }
    .tree-node.selected { background-color: var(--vsc-list-active-background); color: var(--vsc-foreground-bright); }
    .file-tree.focused .tree-node.selected { outline:1px solid var(--vsc-accent-blue); outline-offset:-1px; }
    .tree-node:hover:not(.selected) { background-color: var(--vsc-list-hover-background); }
    .tree-node.focused { outline:1px dotted var(--vsc-border-bright); outline-offset:-1px; }
    .indent-guides { display:flex; position:absolute; left:0; top:0; bottom:0; pointer-events:none; }
    .indent-guide { width:20px; height:100%; position:relative; }
    .indent-guide::before { content:''; position:absolute; left:9.5px; top:0; bottom:0; width:1px; background-color: var(--vsc-border-subtle); opacity:0.3; }
    .indent-guide.active::before { background-color: var(--vsc-border-bright); opacity:0.8; }
    .tree-twistie { width:16px; height:16px; display:flex; align-items:center; justify-content:center; flex-shrink:0; margin-right:2px; opacity:0; transition:opacity 0.15s ease; }
    .tree-node:hover .tree-twistie.expandable, .tree-twistie.expandable { opacity:1; cursor:pointer; border-radius:3px; }
    .tree-twistie:hover { background-color: var(--vsc-list-hover-background); }
    .tree-twistie svg { color: var(--vsc-foreground-dim); transition:transform 0.15s ease; }
    .tree-twistie.expanded svg { transform: rotate(90deg); }
    .tree-icon { width:16px; height:16px; display:flex; align-items:center; justify-content:center; flex-shrink:0; margin-right:4px; }
    .tree-label { flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; padding-right:12px; }
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
        if (node.expandable && node.expanded && node.children) flatten(node.children, node);
      }
    };
    flatten(this.nodes);
    this.flattenedNodes = result;
  }

  getIndentation(level: number): number { return 8 + level * this.indentSize; }
  getIndentGuides(level: number): number[] { return Array.from({ length: level }, (_, i) => i); }
  getCategoryIcon(filename: string): string {
    const ext = filename.split('.').pop()?.toLowerCase() || '';
    const cat = DEFAULT_FILE_CATEGORIES.find(c => c.extensions.includes(ext));
    return cat ? cat.icon : '📄';
  }

  isGuideActive(node: TreeNode, guideIndex: number): boolean {
    // Highlight all ancestors for focused/selected node
    const targetNode = this.selectedNode || this.flattenedNodes[this.focusedNodeIndex];
    if (!targetNode) return false;
    let current: TreeNode | undefined = targetNode;
    while (current?.parent) {
      if (current.parent === node.parent && guideIndex === node.level - 1) return true;
      current = current.parent;
    }
    return node.selected || node.focused || false;
  }

  onFocus() { this.isFocused = true; }
  onBlur() { this.isFocused = false; }
  onKeyDown(event: KeyboardEvent) { /* same as previous, unchanged */ }
  onContextMenu(event: MouseEvent, node: TreeNode): void {
    event.preventDefault();
    event.stopPropagation();
    this.focusedNodeIndex = this.flattenedNodes.findIndex(n => n.path === node.path);
    this.nodeClick.emit(node); // optional: select on right-click
  }

  onToggleExpand(event: MouseEvent, node: TreeNode) { event.stopPropagation(); node.expanded = !node.expanded; this.toggleExpand.emit(node); this.refreshFlattenedNodes(); }
  onNodeClick(node: TreeNode, event: MouseEvent): void {
    event.stopPropagation();

    // Focus the node
    this.focusedNodeIndex = this.flattenedNodes.findIndex(n => n.path === node.path);

    // Select node
    this.nodeClick.emit(node);

    // Expand/collapse if folder
    if (node.expandable) {
      node.expanded = !node.expanded;
      this.toggleExpand.emit(node);
      this.refreshFlattenedNodes();
    }
  }

  onNodeDoubleClick(node: TreeNode) { node.expanded = !node.expanded; this.toggleExpand.emit(node); this.refreshFlattenedNodes(); this.nodeDoubleClick.emit(node); }
  isSelected(node: TreeNode) { return this.selectedNode?.path === node.path; }
  isFocusedNode(node: TreeNode) { return this.flattenedNodes[this.focusedNodeIndex]?.path === node.path; }
  trackByFn: TrackByFunction<TreeNode> = (_, n) => n.path;
}