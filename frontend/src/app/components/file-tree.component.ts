// file-tree.component.ts
import { Component, Input, Output, EventEmitter, OnChanges, SimpleChanges, TrackByFunction, HostBinding } from '@angular/core';
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
        *ngFor="let node of flattenedNodes; trackBy: trackByFn; let i = index"
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
        
        <!-- Indentation guide lines -->
        <div class="indent-guides">
          <div 
            *ngFor="let guide of getIndentGuides(node.level)"
            class="indent-guide"
            [class.active]="isActiveGuide(node, guide)">
          </div>
        </div>

        <!-- Twistie (expand/collapse icon) -->
        <span 
          class="tree-twistie"
          [class.expandable]="node.expandable"
          [class.expanded]="node.expanded"
          [class.collapsed]="!node.expanded && node.expandable"
          (click)="onToggleExpand($event, node)">
          <svg 
            *ngIf="node.expandable"
            width="16" 
            height="16" 
            viewBox="0 0 16 16" 
            fill="none" 
            xmlns="http://www.w3.org/2000/svg">
            <path 
              d="M6 4L10 8L6 12" 
              stroke="currentColor" 
              stroke-width="1.5" 
              stroke-linecap="round" 
              stroke-linejoin="round"
              class="chevron"/>
          </svg>
        </span>

        <!-- File/Folder Icon -->
        <span class="tree-icon">
          <svg 
            *ngIf="node.type === FileType.DIRECTORY"
            width="16" 
            height="16" 
            viewBox="0 0 16 16" 
            fill="none"
            class="folder-icon"
            [class.open]="node.expanded">
            <path 
              d="M14.5 3H7.5L6 1.5H1.5V12.5C1.5 13.0523 1.94772 13.5 2.5 13.5H13.5C14.0523 13.5 14.5 13.0523 14.5 12.5V3.5C14.5 3.22386 14.2761 3 14 3H14.5Z" 
              fill="var(--vsc-folder-fill, #dcb67a)"/>
            <path 
              d="M1.5 3V12.5C1.5 13.0523 1.94772 13.5 2.5 13.5H13.5C14.0523 13.5 14.5 13.0523 14.5 12.5V5H7.5L6 3.5H1.5V3Z" 
              fill="var(--vsc-folder-fill, #dcb67a)"/>
            <path 
              d="M1.5 3.5V12.5C1.5 13.0523 1.94772 13.5 2.5 13.5H13.5C14.0523 13.5 14.5 13.0523 14.5 12.5V5H7.5L6 3.5H1.5Z" 
              stroke="var(--vsc-folder-stroke, #dcb67a)" 
              stroke-width="1"/>
          </svg>

          <svg 
            *ngIf="node.type === FileType.SYMLINK"
            width="16" 
            height="16" 
            viewBox="0 0 16 16" 
            fill="none"
            class="symlink-icon">
            <path 
              d="M4 8V5.5C4 3.567 5.567 2 7.5 2H11.5C13.433 2 15 3.567 15 5.5C15 7.433 13.433 9 11.5 9H8" 
              stroke="var(--vsc-accent-blue)" 
              stroke-width="1.5" 
              stroke-linecap="round"/>
            <path 
              d="M12 8V10.5C12 12.433 10.433 14 8.5 14H4.5C2.567 14 1 12.433 1 10.5C1 8.567 2.567 7 4.5 7H8" 
              stroke="var(--vsc-accent-blue)" 
              stroke-width="1.5" 
              stroke-linecap="round"/>
            <path 
              d="M11 5L13.5 2.5L11 0" 
              stroke="var(--vsc-accent-blue)" 
              stroke-width="1.5" 
              stroke-linecap="round" 
              stroke-linejoin="round"/>
            <path 
              d="M5 11L2.5 13.5L5 16" 
              stroke="var(--vsc-accent-blue)" 
              stroke-width="1.5" 
              stroke-linecap="round" 
              stroke-linejoin="round"/>
          </svg>

          <svg 
            *ngIf="node.type !== FileType.DIRECTORY && node.type !== FileType.SYMLINK"
            width="16" 
            height="16" 
            viewBox="0 0 16 16" 
            fill="none"
            class="file-icon"
            [class]="getFileIconClass(node)">
            <path 
              d="M9 1.5H4C3.17157 1.5 2.5 2.17157 2.5 3V13C2.5 13.8284 3.17157 14.5 4 14.5H12C12.8284 14.5 13.5 13.8284 13.5 13V5.5L9 1.5Z" 
              fill="var(--vsc-file-fill, #3c3c3c)"
              stroke="var(--vsc-file-stroke, #5a5a5a)"
              stroke-width="1"/>
            <path 
              d="M9 1.5V5C9 5.27614 9.22386 5.5 9.5 5.5H13.5" 
              stroke="var(--vsc-file-stroke, #5a5a5a)"
              stroke-width="1"
              stroke-linecap="round"/>
            <!-- File extension indicator -->
            <text 
              *ngIf="getFileExtension(node)"
              x="8" 
              y="11" 
              text-anchor="middle" 
              font-size="5" 
              font-weight="bold"
              fill="var(--vsc-foreground-dim)"
              class="file-ext">{{ getFileExtension(node) }}</text>
          </svg>
        </span>

        <!-- Node Label -->
        <span class="tree-label" [title]="node.name">{{ node.name }}</span>

        <!-- Loading Spinner -->
        <span class="tree-spinner" *ngIf="node.loading">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <circle 
              cx="8" 
              cy="8" 
              r="6" 
              stroke="var(--vsc-foreground-dim)" 
              stroke-width="1.5" 
              stroke-linecap="round" 
              stroke-dasharray="20 10"
              class="spinner-circle">
              <animateTransform
                attributeName="transform"
                type="rotate"
                from="0 8 8"
                to="360 8 8"
                dur="1s"
                repeatCount="indefinite"/>
            </circle>
          </svg>
        </span>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      flex: 1;
      min-height: 0;
      overflow: hidden;
    }

    .file-tree {
      overflow-y: auto;
      overflow-x: hidden;
      background: var(--vsc-editor-background);
      padding: 0;
      outline: none;
      font-family: var(--vsc-font-family);
      font-size: var(--vsc-font-size);
      line-height: var(--vsc-line-height);
      user-select: none;
    }

    .file-tree.focused {
      outline: none;
    }

    .tree-node {
      display: flex;
      align-items: center;
      height: 22px;
      cursor: pointer;
      position: relative;
      color: var(--vsc-foreground);
      transition: background-color 0.1s ease;
    }

    /* Selection styling - VS Code style */
    .tree-node.selected {
      background-color: var(--vsc-list-active-background);
      color: var(--vsc-foreground-bright);
    }

    .file-tree.focused .tree-node.selected {
      background-color: var(--vsc-list-active-background);
      outline: 1px solid var(--vsc-accent-blue);
      outline-offset: -1px;
    }

    .tree-node:hover:not(.selected) {
      background-color: var(--vsc-list-hover-background);
    }

    .tree-node.focused {
      outline: 1px dotted var(--vsc-border-bright);
      outline-offset: -1px;
    }

    /* Indentation guides - VS Code style */
    .indent-guides {
      display: flex;
      position: absolute;
      left: 0;
      top: 0;
      bottom: 0;
      pointer-events: none;
    }

    .indent-guide {
      width: 20px;
      height: 100%;
      position: relative;
    }

    .indent-guide::before {
      content: '';
      position: absolute;
      left: 9.5px;
      top: 0;
      bottom: 0;
      width: 1px;
      background-color: var(--vsc-border-subtle);
      opacity: 0.5;
    }

    .indent-guide.active::before {
      background-color: var(--vsc-border-bright);
      opacity: 0.8;
    }

    /* Twistie styling */
    .tree-twistie {
      width: 16px;
      height: 16px;
      display: flex;
      align-items: center;
      justify-content: center;
      flex-shrink: 0;
      margin-right: 2px;
      opacity: 0;
      transition: opacity 0.15s ease;
    }

    .tree-node:hover .tree-twistie.expandable,
    .tree-twistie.expandable {
      opacity: 1;
    }

    .tree-twistie.expandable {
      cursor: pointer;
      border-radius: 3px;
    }

    .tree-twistie.expandable:hover {
      background-color: var(--vsc-list-hover-background);
    }

    .tree-twistie svg {
      color: var(--vsc-foreground-dim);
      transition: transform 0.15s ease;
    }

    .tree-twistie.expanded svg {
      transform: rotate(90deg);
    }

    .tree-twistie.collapsed svg {
      transform: rotate(0deg);
    }

    .tree-twistie .chevron {
      stroke: var(--vsc-foreground-dim);
    }

    .tree-twistie:hover .chevron {
      stroke: var(--vsc-foreground);
    }

    /* Icon styling */
    .tree-icon {
      width: 16px;
      height: 16px;
      display: flex;
      align-items: center;
      justify-content: center;
      flex-shrink: 0;
      margin-right: 4px;
    }

    .tree-icon svg {
      width: 100%;
      height: 100%;
    }

    /* Folder icon colors */
    .folder-icon path[fill] {
      fill: var(--vsc-folder-fill, #dcb67a);
    }

    .folder-icon.open path[fill] {
      fill: var(--vsc-folder-open-fill, #dcb67a);
      opacity: 0.9;
    }

    /* File icon styling */
    .file-icon path[fill] {
      fill: var(--vsc-file-fill, #3c3c3c);
    }

    .file-icon path[stroke] {
      stroke: var(--vsc-file-stroke, #5a5a5a);
    }

    .file-icon.text-file .file-ext { fill: var(--vsc-accent-blue); }
    .file-icon.code-file .file-ext { fill: var(--vsc-accent-purple); }
    .file-icon.image-file .file-ext { fill: var(--vsc-accent-green); }
    .file-icon.config-file .file-ext { fill: var(--vsc-accent-yellow); }

    /* Symlink icon */
    .symlink-icon path {
      stroke: var(--vsc-accent-blue);
    }

    /* Label styling */
    .tree-label {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: var(--vsc-font-size);
      line-height: 1.4;
      padding-right: 12px;
    }

    /* Loading spinner */
    .tree-spinner {
      margin-left: auto;
      margin-right: 8px;
      width: 16px;
      height: 16px;
    }

    .spinner-circle {
      transform-origin: center;
    }

    /* Compact mode */
    :host-context(.compact-mode) {
      .tree-node {
        height: 20px;
      }
      
      .tree-label {
        font-size: var(--vsc-font-size-small);
      }
    }

    /* High contrast mode support */
    @media (prefers-contrast: high) {
      .tree-node.selected {
        outline: 2px solid var(--vsc-accent-blue);
        outline-offset: -2px;
      }
      
      .indent-guide::before {
        opacity: 1;
      }
    }
  `]
})
export class FileTreeComponent implements OnChanges {
  @Input() nodes: TreeNode[] = [];
  @Input() selectedNode?: TreeNode;
  @Input() indentSize: number = 20; // VS Code default is 8, but 20 is more readable [^4^]
  @Input() showIndentGuides: boolean = true;

  @Output() nodeClick = new EventEmitter<TreeNode>();
  @Output() nodeDoubleClick = new EventEmitter<TreeNode>();
  @Output() toggleExpand = new EventEmitter<TreeNode>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; node: TreeNode }>();
  @Output() keyboardNavigation = new EventEmitter<{ node: TreeNode; key: string }>();

  flattenedNodes: TreeNode[] = [];
  focusedNodeIndex: number = -1;
  isFocused: boolean = false;

  FileType = FileType;

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['nodes'] || changes['selectedNode']) {
      this.refreshFlattenedNodes();
      if (this.selectedNode && changes['selectedNode']) {
        this.focusedNodeIndex = this.flattenedNodes.findIndex(n => n.path === this.selectedNode!.path);
      }
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

  getIndentation(level: number): number {
    // VS Code style: base padding + level * indentSize
    return 8 + (level * this.indentSize);
  }

  getIndentGuides(level: number): number[] {
    if (!this.showIndentGuides || level === 0) return [];
    return Array.from({ length: level }, (_, i) => i);
  }

  isActiveGuide(node: TreeNode, guideIndex: number): boolean {
    // Highlight guide if this node is selected or has selected descendant
    if (!node.children) return false;
    return this.hasSelectedDescendant(node);
  }

  private hasSelectedDescendant(node: TreeNode): boolean {
    if (!node.children) return false;
    return node.children.some(child => 
      this.isSelected(child) || this.hasSelectedDescendant(child)
    );
  }

  onFocus(): void {
    this.isFocused = true;
  }

  onBlur(): void {
    this.isFocused = false;
  }

  onKeyDown(event: KeyboardEvent): void {
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
        this.expandOrNavigateDown();
        break;
      case 'ArrowLeft':
        event.preventDefault();
        this.collapseOrNavigateUp();
        break;
      case 'Enter':
        event.preventDefault();
        if (this.focusedNodeIndex >= 0) {
          const node = this.flattenedNodes[this.focusedNodeIndex];
          this.nodeDoubleClick.emit(node);
        }
        break;
      case ' ':
        event.preventDefault();
        if (this.focusedNodeIndex >= 0) {
          const node = this.flattenedNodes[this.focusedNodeIndex];
          this.nodeClick.emit(node);
        }
        break;
    }
  }

  private navigateDown(): void {
    this.focusedNodeIndex = Math.min(
      this.focusedNodeIndex + 1, 
      this.flattenedNodes.length - 1
    );
    this.emitKeyboardNavigation('ArrowDown');
  }

  private navigateUp(): void {
    this.focusedNodeIndex = Math.max(this.focusedNodeIndex - 1, 0);
    this.emitKeyboardNavigation('ArrowUp');
  }

  private expandOrNavigateDown(): void {
    if (this.focusedNodeIndex < 0) return;
    const node = this.flattenedNodes[this.focusedNodeIndex];
    if (node.expandable && !node.expanded) {
      this.onToggleExpand(new MouseEvent('click'), node);
    } else if (node.children && node.children.length > 0) {
      this.navigateDown();
    }
  }

  private collapseOrNavigateUp(): void {
    if (this.focusedNodeIndex < 0) return;
    const node = this.flattenedNodes[this.focusedNodeIndex];
    if (node.expandable && node.expanded) {
      this.onToggleExpand(new MouseEvent('click'), node);
    } else if (node.level > 0) {
      // Navigate to parent
      const parentIndex = this.findParentIndex(node);
      if (parentIndex >= 0) {
        this.focusedNodeIndex = parentIndex;
        this.emitKeyboardNavigation('ArrowLeft');
      }
    }
  }

  private findParentIndex(node: TreeNode): number {
    for (let i = this.focusedNodeIndex - 1; i >= 0; i--) {
      if (this.flattenedNodes[i].level < node.level) {
        return i;
      }
    }
    return -1;
  }

  private emitKeyboardNavigation(key: string): void {
    if (this.focusedNodeIndex >= 0) {
      const node = this.flattenedNodes[this.focusedNodeIndex];
      this.keyboardNavigation.emit({ node, key });
      this.nodeClick.emit(node);
    }
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
    this.focusedNodeIndex = this.flattenedNodes.findIndex(n => n.path === node.path);
    this.nodeClick.emit(node);
  }

  onNodeDoubleClick(node: TreeNode): void {
    if (node.expandable) {
      node.expanded = !node.expanded;
      this.toggleExpand.emit(node);
      this.refreshFlattenedNodes();
    }
    this.nodeDoubleClick.emit(node);
  }

  onContextMenu(event: MouseEvent, node: TreeNode): void {
    event.preventDefault();
    event.stopPropagation();
    this.focusedNodeIndex = this.flattenedNodes.findIndex(n => n.path === node.path);
    this.contextMenu.emit({ event, node });
    this.nodeClick.emit(node);
  }

  isSelected(node: TreeNode): boolean {
    return this.selectedNode?.path === node.path;
  }

  isFocusedNode(node: TreeNode): boolean {
    if (this.focusedNodeIndex < 0) return false;
    return this.flattenedNodes[this.focusedNodeIndex]?.path === node.path;
  }

  getFileExtension(node: TreeNode): string {
    if (node.type === FileType.DIRECTORY) return '';
    const match = node.name.match(/\.([^.]+)$/);
    return match ? match[1].substring(0, 3).toUpperCase() : '';
  }

  getFileIconClass(node: TreeNode): string {
    const ext = node.name.split('.').pop()?.toLowerCase() || '';
    const codeExts = ['ts', 'js', 'tsx', 'jsx', 'py', 'java', 'cpp', 'c', 'go', 'rs'];
    const textExts = ['txt', 'md', 'json', 'xml', 'yaml', 'yml'];
    const imageExts = ['png', 'jpg', 'jpeg', 'gif', 'svg', 'ico'];
    const configExts = ['json', 'xml', 'ini', 'conf', 'config'];

    if (codeExts.includes(ext)) return 'code-file';
    if (textExts.includes(ext)) return 'text-file';
    if (imageExts.includes(ext)) return 'image-file';
    if (configExts.includes(ext)) return 'config-file';
    return '';
  }

  trackByFn: TrackByFunction<TreeNode> = (_, node) => node.path;

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