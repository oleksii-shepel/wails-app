// file-tree.component.ts
import { Component, Input, Output, EventEmitter, OnChanges, SimpleChanges, HostListener, TrackByFunction } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';

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
    <div class="file-tree" role="tree" tabindex="0">
      <div 
        *ngFor="let node of flattenedNodes; trackBy: trackByFn"
        class="tree-node"
        [class.selected]="isSelected(node)"
        [class.expanded]="node.expanded"
        [class.collapsed]="!node.expanded && node.expandable"
        [class.loading]="node.loading"
        [style.padding-left.px]="node.level * 20 + 8"
        [style.background-size]="node.level * 20 + 'px 100%'"
        [attr.role]="'treeitem'"
        [attr.aria-expanded]="node.expandable ? node.expanded : undefined"
        [attr.aria-selected]="isSelected(node)"
        (click)="onNodeClick(node, $event)"
        (dblclick)="onNodeDoubleClick(node)"
      >
        <span 
          class="tree-twistie"
          [class.expandable]="node.expandable"
          [class.open]="node.expanded"
          (click)="onToggleExpand($event, node)">
        </span>

        <span class="tree-icon">{{ getIcon(node) }}</span>

        <span class="tree-label" [title]="node.name">{{ node.name }}</span>

        <span class="tree-spinner" *ngIf="node.loading">⟳</span>
      </div>
    </div>
  `,
  styles: [`
    .file-tree {
      overflow-y: auto;
      overflow-x: hidden;
      background: var(--vsc-editor-background);
      padding: var(--vsc-padding-xs) 0;
      outline: none;
      font-family: var(--vsc-font-family);
    }

    .tree-node {
      display: flex;
      align-items: center;
      height: 22px;
      cursor: pointer;
      user-select: none;
      transition: background 0.12s ease, color 0.12s ease;
      position: relative;

      /* Indent guides like VSCode */
      background-image: none;
      background-position-x: 8px;
      background-repeat: no-repeat;
      opacity: 0.85;
    }

    .file-tree:hover .tree-node,
    .file-tree:focus-within .tree-node {
      opacity: 1;
    }

    .tree-node:hover {
      background: var(--vsc-list-hover-background);
    }

    .tree-node.selected {
      background: var(--vsc-list-active-background);
      color: var(--vsc-foreground-bright);
    }

    .tree-node.loading {
      opacity: 0.7;
    }

    .tree-node::before {
      content: '';
      position: absolute;
      top: 0;
      bottom: 0;
      left: 8px; /* base padding */
      width: 1px;
      background-color: var(--vsc-border-subtle);
      pointer-events: none;
      opacity: 0; /* hidden by default */
    }

    .file-tree.show-guides .tree-node::before {
      opacity: 1;
    }
    
    .tree-twistie {
      width: 16px;
      height: 16px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 11px;
      margin-right: 4px;
      color: var(--vsc-foreground-dim);
      flex-shrink: 0;
      cursor: default;
      transition: transform 0.12s ease, color 0.12s ease;
    }

    .tree-twistie.expandable {
      cursor: pointer;
    }

    .tree-twistie.expandable:hover {
      color: var(--vsc-foreground);
    }

    .tree-twistie::before {
      content: ">";
      font-weight: bold;
      display: inline-block;
      transform-origin: center;
    }

    .tree-twistie.open::before {
      transform: rotate(90deg);
    }

    .tree-icon {
      font-size: 16px;
      margin-right: 4px;
      flex-shrink: 0;
      opacity: 0.9;
    }

    .tree-label {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: var(--vsc-font-size);
      line-height: var(--vsc-line-height);
    }

    .tree-spinner {
      margin-left: auto;
      font-size: 12px;
      animation: spin 1s linear infinite;
    }

    @keyframes spin {
      from { transform: rotate(0deg); }
      to { transform: rotate(360deg); }
    }

    /* Focus styling */
    .file-tree:focus-within .tree-node.selected {
      outline: 1px solid var(--vsc-focusBorder);
      outline-offset: -1px;
    }
  `]
})
export class FileTreeComponent implements OnChanges {
  @Input() nodes: TreeNode[] = [];
  @Input() selectedNode?: TreeNode;

  @Output() nodeClick = new EventEmitter<TreeNode>();
  @Output() nodeDoubleClick = new EventEmitter<TreeNode>();
  @Output() toggleExpand = new EventEmitter<TreeNode>();

  flattenedNodes: TreeNode[] = [];

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['nodes']) {
      this.refreshFlattenedNodes();
    }
  }

  /** Recursively flattens the visible tree nodes */
  private refreshFlattenedNodes(): void {
    const result: TreeNode[] = [];
    const flatten = (nodes: TreeNode[]) => {
      for (const node of nodes) {
        result.push(node);
        if (node.expandable && node.expanded && node.children) {
          flatten(node.children);
        }
      }
    };
    flatten(this.nodes);
    this.flattenedNodes = result;
  }

  onToggleExpand(event: MouseEvent, node: TreeNode): void {
    event.stopPropagation();
    if (node.expandable) {
      node.expanded = !node.expanded;
      this.toggleExpand.emit(node);
      this.refreshFlattenedNodes();
    }
  }

  onNodeClick(node: TreeNode, event: MouseEvent): void {
    event.stopPropagation();
    this.nodeClick.emit(node);
  }

  onNodeDoubleClick(node: TreeNode): void {
    if (node.expandable) {
      node.expanded = !node.expanded;
      this.refreshFlattenedNodes();
    }
    this.nodeDoubleClick.emit(node);
  }

  isSelected(node: TreeNode): boolean {
    return this.selectedNode?.path === node.path;
  }

  getIcon(node: TreeNode): string {
    if (node.type === FileType.DIRECTORY) {
      return node.expanded ? '📂' : '📁';
    }
    const ext = node.name.split('.').pop()?.toLowerCase() || '';
    const map: Record<string, string> = {
      'ts': '📜', 'js': '📜', 'json': '⚙️', 'md': '📝', 'txt': '📄'
    };
    return map[ext] || '📄';
  }

  trackByFn: TrackByFunction<TreeNode> = (_, node) => node.path;
}