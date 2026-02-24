// file-tree.component.ts
import { 
  Component, 
  Input, 
  Output, 
  EventEmitter, 
  OnChanges, 
  SimpleChanges, 
  TrackByFunction,
  ChangeDetectionStrategy,
  ChangeDetectorRef
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FileInfo, FileType } from '../shared/protocol';
import { DEFAULT_FILE_CATEGORIES } from './filter-bar.types';

export interface TreeNode extends FileInfo {
  level: number;
  expanded?: boolean;
  expandable?: boolean;
  children?: TreeNode[];
  parent?: TreeNode;
  loaded?: boolean;      // Whether children have been loaded
  loading?: boolean;     // Currently loading children
}

@Component({
  selector: 'app-file-tree',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div 
      class="file-tree-container" 
      role="tree" 
      tabindex="0"
      [class.focused]="isFocused"
      (focus)="onFocus()"
      (blur)="onBlur()"
      (keydown)="onKeyDown($event)">
      
      <ng-container *ngTemplateOutlet="treeTemplate; context: { $implicit: nodes }"></ng-container>

      <div *ngIf="nodes.length === 0 && !loading" class="tree-empty">
        {{ emptyMessage }}
      </div>
      
      <div *ngIf="loading" class="tree-loading">
        <span class="spinner"></span>
        Loading...
      </div>
    </div>

    <ng-template #treeTemplate let-nodes>
      <div 
        *ngFor="let node of nodes; trackBy: trackByFn" 
        class="tree-node-wrapper"
        [attr.data-path]="node.path">
        
        <div 
          class="tree-node"
          [class.selected]="isSelected(node)"
          [class.focused]="isFocusedNode(node)"
          [class.expanded]="isExpanded(node)"
          [class.collapsed]="!isExpanded(node) && node.expandable"
          [style.--indent-level]="node.level"
          [attr.role]="'treeitem'"
          [attr.aria-level]="node.level + 1"
          [attr.aria-expanded]="node.expandable ? isExpanded(node) : undefined"
          [attr.aria-selected]="isSelected(node)"
          (dblclick)="onNodeDoubleClick(node)"
          (contextmenu)="onContextMenu($event, node)">

          <!-- Indentation guides -->
          <div class="indent-guides" *ngIf="node.level > 0">
            <div 
              *ngFor="let guide of getIndentGuides(node.level); let i = index"
              class="indent-guide"
              [class.active]="isGuideActive(node, i)">
            </div>
          </div>

          <!-- Expand/Collapse toggle -->
          <span 
            class="tree-twistie"
            [class.expandable]="node.expandable"
            [class.expanded]="isExpanded(node)"
            [class.loading]="node.loading"
            (click)="onToggleExpand($event, node)">
            
            <ng-container *ngIf="node.expandable">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                <path d="M6 4L10 8L6 12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </ng-container>
            <span *ngIf="!node.expandable" class="twistie-placeholder"></span>
          </span>

          <!-- Icon -->
          <span class="tree-icon">
            <ng-container [ngSwitch]="node.type">
              <span *ngSwitchCase="FileType.DIRECTORY" class="folder-icon">
                {{ isExpanded(node) ? '📂' : '📁' }}
              </span>
              <span *ngSwitchCase="FileType.SYMLINK" class="symlink-icon">🔗</span>
              <span *ngSwitchDefault class="file-icon">{{ getCategoryIcon(node.name) }}</span>
            </ng-container>
          </span>

          <!-- Label -->
          <span class="tree-label" [title]="node.path">{{ node.name }}</span>
        </div>

        <!-- Children container - rendered when expanded -->
        <div 
          class="tree-children" 
          *ngIf="isExpanded(node)">
          
          <!-- Recursively render children -->
          <ng-container *ngIf="node.children?.length; else noChildren">
            <ng-container *ngTemplateOutlet="treeTemplate; context: { $implicit: node.children }"></ng-container>
          </ng-container>
          
          <ng-template #noChildren>
            <div *ngIf="node.loaded && !node.loading" class="tree-node empty-children">
              <span [style.--indent-level]="node.level + 1" class="empty-message">Empty folder</span>
            </div>
          </ng-template>
        </div>
      </div>
    </ng-template>
  `,
  styles: [`
    :host { 
      display: flex; 
      flex-direction: column; 
      flex: 1; 
      min-height: 0; 
      overflow: hidden; 
    }
    
    .file-tree-container { 
      overflow: auto; 
      background: var(--vsc-editor-background); 
      font-family: var(--vsc-font-family); 
      font-size: var(--vsc-font-size); 
      line-height: 22px; 
      outline: none; 
      height: 100%; 
      user-select: none;
    }
    
    .tree-node-wrapper { position: relative; }
    
    .tree-node { 
      display: flex; 
      align-items: center; 
      height: 22px; 
      cursor: pointer; 
      position: relative; 
      color: var(--vsc-foreground); 
      transition: background-color 0.1s ease; 
      white-space: nowrap;
      padding-left: calc(4px + var(--indent-level, 0) * 20px);
    }
    
    .tree-node.selected { 
      background-color: var(--vsc-list-active-background); 
      color: var(--vsc-foreground-bright); 
    }
    
    .file-tree-container.focused .tree-node.selected { 
      outline: 1px solid var(--vsc-focus-border); 
      outline-offset: -1px; 
    }
    
    .tree-node:hover:not(.selected) { 
      background-color: var(--vsc-list-hover-background); 
    }
    
    .tree-node.focused:not(.selected) { 
      outline: 1px dotted var(--vsc-focus-border); 
      outline-offset: -1px; 
    }

    /* Indentation guides */
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
      left: 9px; 
      top: 0; 
      bottom: 0; 
      width: 1px; 
      background-color: var(--vsc-tree-indent-guide); 
      opacity: 0.3; 
    }
    
    .indent-guide.active::before { 
      background-color: var(--vsc-tree-indent-guide-active); 
      opacity: 0.8; 
    }

    /* Twistie/Expand button */
    .tree-twistie { 
      width: 20px; 
      height: 22px; 
      display: flex; 
      align-items: center; 
      justify-content: center; 
      flex-shrink: 0; 
      color: var(--vsc-foreground-dim); 
    }
    
    .tree-twistie.expandable { 
      cursor: pointer; 
      opacity: 0.7; 
    }
    
    .tree-twistie.expandable:hover { 
      opacity: 1; 
      background-color: var(--vsc-list-hover-background); 
      border-radius: 3px; 
    }
    
    .tree-twistie.expanded { opacity: 1; }
    .tree-twistie.loading { opacity: 0.7; }
    
    .tree-twistie svg { 
      transition: transform 0.15s ease; 
      width: 16px; 
      height: 16px; 
    }
    
    .tree-twistie.expanded svg { 
      transform: rotate(90deg); 
    }
    
    .twistie-placeholder { 
      width: 16px; 
      height: 16px; 
      display: inline-block;
    }

    /* Spinner */
    .spinner-small {
      width: 12px;
      height: 12px;
      border: 2px solid var(--vsc-foreground-dim);
      border-top-color: transparent;
      border-radius: 50%;
      animation: spin 1s linear infinite;
    }
    
    @keyframes spin {
      to { transform: rotate(360deg); }
    }

    /* Icons */
    .tree-icon { 
      width: 20px; 
      height: 22px; 
      display: flex; 
      align-items: center; 
      justify-content: center; 
      flex-shrink: 0; 
      font-size: 16px; 
    }
    
    .tree-label { 
      flex: 1; 
      overflow: hidden; 
      text-overflow: ellipsis; 
      white-space: nowrap; 
      padding-right: 12px; 
      font-size: var(--vsc-font-size); 
      margin-left: 2px; 
    }

    /* Children container */
    .tree-children { 
      position: relative;
      overflow: hidden;
    }

    .tree-node.empty-children {
      color: var(--vsc-foreground-dim);
      font-style: italic;
      font-size: 0.9em;
    }

    /* Empty states */
    .tree-empty, .tree-loading { 
      padding: 20px; 
      color: var(--vsc-foreground-dim); 
      text-align: center; 
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
    }
    
    .spinner {
      width: 16px;
      height: 16px;
      border: 2px solid var(--vsc-foreground-dim);
      border-top-color: transparent;
      border-radius: 50%;
      animation: spin 1s linear infinite;
    }
  `]
})
export class FileTreeComponent implements OnChanges {
  @Input() nodes: TreeNode[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  @Input() loading = false;
  @Input() emptyMessage = 'No items';
  
  @Output() selectionChange = new EventEmitter<TreeNode[]>();
  @Output() nodeDoubleClick = new EventEmitter<TreeNode>();
  @Output() navigate = new EventEmitter<TreeNode>();
  @Output() toggleExpand = new EventEmitter<TreeNode>();  // Request to expand/collapse (lazy load)
  @Output() loadChildren = new EventEmitter<TreeNode>();   // Request to load children (lazy loading)
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; node: TreeNode }>();

  // Internal state
  private expansionState = new Map<string, boolean>();
  private focusedNodePath: string | null = null;
  private lastSelectedPath: string | null = null;
  
  isFocused = false;
  FileType = FileType;

  constructor(private cdr: ChangeDetectorRef) {}

  ngOnChanges(changes: SimpleChanges) {
    if (changes['nodes']) {
      this.updateNodeHierarchy(this.nodes);
    }
  }

  /**
   * Check if node is expanded
   */
  isExpanded(node: TreeNode): boolean {
    return this.expansionState.get(node.path) ?? node.expanded ?? false;
  }

  /**
   * Set expansion state
   */
  private setExpanded(node: TreeNode, expanded: boolean): void {
    this.expansionState.set(node.path, expanded);
    node.expanded = expanded;
    
    // If expanding and not loaded, trigger lazy load
    if (expanded && !node.loaded && node.expandable && !node.loading) {
      node.loading = true;
      this.loadChildren.emit(node);
    }
  }

  /**
   * Update hierarchy references and restore expansion state
   */
  private updateNodeHierarchy(nodes: TreeNode[], parent?: TreeNode): void {
    for (const node of nodes) {
      node.parent = parent;
      // Restore expansion state from map
      if (this.expansionState.has(node.path)) {
        node.expanded = this.expansionState.get(node.path)!;
      }
      if (node.children) {
        this.updateNodeHierarchy(node.children, node);
      }
    }
  }

  /**
   * Get flattened list of visible nodes for keyboard navigation
   */
  getVisibleNodes(): TreeNode[] {
    const result: TreeNode[] = [];
    const traverse = (nodes: TreeNode[]) => {
      for (const node of nodes) {
        result.push(node);
        if (this.isExpanded(node) && node.children) {
          traverse(node.children);
        }
      }
    };
    traverse(this.nodes);
    return result;
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
    // Active if any selected descendant passes through this guide
    const selectedPaths = Array.from(this.selectedFiles);
    if (selectedPaths.length === 0) return false;

    return selectedPaths.some(path => {
      // Check if path is descendant of node at this level
      return path.startsWith(node.path + '/') && 
             path.split('/').length > guideIndex + 2;
    });
  }

  // Selection handling
  isSelected(node: TreeNode): boolean {
    return this.selectedFiles.has(node.path);
  }

  isFocusedNode(node: TreeNode): boolean {
    return this.focusedNodePath === node.path;
  }

  // Event handlers
  onFocus(): void {
    this.isFocused = true;
    // Focus first node if none focused
    if (!this.focusedNodePath && this.nodes.length > 0) {
      this.focusedNodePath = this.nodes[0].path;
      this.cdr.markForCheck();
    }
  }

  onBlur(): void {
    this.isFocused = false;
  }

  onNodeClick(node: TreeNode, event: MouseEvent): void {
    event.stopPropagation();
    this.focusedNodePath = node.path;

    const isMulti = event.ctrlKey || event.metaKey;
    const isRange = event.shiftKey;
    
    if (isRange && this.lastSelectedPath) {
      const visible = this.getVisibleNodes();
      const startIdx = visible.findIndex(n => n.path === this.lastSelectedPath);
      const endIdx = visible.findIndex(n => n.path === node.path);
      const [min, max] = [Math.min(startIdx, endIdx), Math.max(startIdx, endIdx)];
      const newSelection: TreeNode[] = [];
      for (let i = min; i <= max; i++) {
        if (i >= 0 && i < visible.length) newSelection.push(visible[i]);
      }
      this.selectionChange.emit(newSelection);
    } else if (isMulti) {
      const current = Array.from(this.selectedFiles);
      const exists = current.includes(node.path);
      const newPaths = exists ? current.filter(p => p !== node.path) : [...current, node.path];
      const selectedNodes = this.getVisibleNodes().filter(n => newPaths.includes(n.path));
      this.selectionChange.emit(selectedNodes);
      this.lastSelectedPath = node.path;
    } else {
      this.selectionChange.emit([node]);
      this.lastSelectedPath = node.path;
    }

    this.cdr.markForCheck();
  }

  onNodeDoubleClick(node: TreeNode): void {
    // Restore original: directories toggle expand AND navigate, files just navigate
    if (node.type === FileType.DIRECTORY && node.expandable) {
      this.toggleNode(node);
    } else {
      this.nodeDoubleClick.emit(node);
      this.navigate.emit(node);
    }
  }

  onToggleExpand(event: MouseEvent, node: TreeNode): void {
    event.stopPropagation();
    if (!node.expandable) return;
    this.toggleNode(node);
  }

  private toggleNode(node: TreeNode): void {
    const expanded = !this.isExpanded(node);
    this.setExpanded(node, expanded);
    this.toggleExpand.emit(node);
    this.cdr.markForCheck();
  }

  onContextMenu(event: MouseEvent, node: TreeNode): void {
    event.preventDefault();
    event.stopPropagation();
    this.focusedNodePath = node.path;
    
    if (!this.selectedFiles.has(node.path)) {
      this.selectionChange.emit([node]);
    }
    
    this.contextMenu.emit({ event, node });
  }

  // Keyboard navigation
  onKeyDown(event: KeyboardEvent): void {
    const visible = this.getVisibleNodes();
    if (visible.length === 0) return;

    const currentIdx = visible.findIndex(n => n.path === this.focusedNodePath);
    const current = currentIdx >= 0 ? visible[currentIdx] : null;

    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        if (currentIdx < visible.length - 1) {
          this.focusedNodePath = visible[currentIdx + 1].path;
          this.ensureVisible(visible[currentIdx + 1]);
        }
        break;

      case 'ArrowUp':
        event.preventDefault();
        if (currentIdx > 0) {
          this.focusedNodePath = visible[currentIdx - 1].path;
          this.ensureVisible(visible[currentIdx - 1]);
        }
        break;

      case 'ArrowRight':
        event.preventDefault();
        if (current?.expandable) {
          if (!this.isExpanded(current)) {
            this.setExpanded(current, true);
            this.toggleExpand.emit(current);
          } else if (current.children?.length) {
            // Move to first child
            const childIdx = visible.findIndex(n => n.parent === current);
            if (childIdx > currentIdx) {
              this.focusedNodePath = visible[childIdx].path;
            }
          }
        }
        break;

      case 'ArrowLeft':
        event.preventDefault();
        if (current) {
          if (current.expandable && this.isExpanded(current)) {
            this.setExpanded(current, false);
            this.toggleExpand.emit(current);
          } else if (current.parent) {
            this.focusedNodePath = current.parent.path;
            this.ensureVisible(current.parent);
          }
        }
        break;

      case 'Enter':
        event.preventDefault();
        if (current) {
          if (current.type === FileType.DIRECTORY && current.expandable) {
            this.toggleNode(current);
          } else {
            this.nodeDoubleClick.emit(current);
          }
        }
        break;

      case ' ':
        event.preventDefault();
        if (current) {
          this.onNodeClick(current, new MouseEvent('click', { ctrlKey: event.ctrlKey, shiftKey: event.shiftKey }));
        }
        break;

      case 'Home':
        event.preventDefault();
        this.focusedNodePath = visible[0].path;
        this.ensureVisible(visible[0]);
        break;

      case 'End':
        event.preventDefault();
        this.focusedNodePath = visible[visible.length - 1].path;
        this.ensureVisible(visible[visible.length - 1]);
        break;

      case '*':
        // Expand all siblings (Windows Explorer style)
        event.preventDefault();
        if (current?.parent) {
          current.parent.children?.forEach(child => {
            if (child.expandable && !this.isExpanded(child)) {
              this.setExpanded(child, true);
              this.toggleExpand.emit(child);
            }
          });
        } else {
          // Expand all root level
          this.nodes.forEach(node => {
            if (node.expandable && !this.isExpanded(node)) {
              this.setExpanded(node, true);
              this.toggleExpand.emit(node);
            }
          });
        }
        break;
    }
    
    this.cdr.markForCheck();
  }

  private ensureVisible(node: TreeNode): void {
    // Use requestAnimationFrame to ensure DOM is updated
    requestAnimationFrame(() => {
      const element = document.querySelector(`[data-path="${CSS.escape(node.path)}"]`);
      element?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    });
  }

  trackByFn: TrackByFunction<TreeNode> = (_, node) => node.path;

  // Public API for parent component
  expandNode(path: string): void {
    const node = this.findNodeByPath(this.nodes, path);
    if (node && node.expandable) {
      this.setExpanded(node, true);
      this.cdr.markForCheck();
    }
  }

  collapseNode(path: string): void {
    const node = this.findNodeByPath(this.nodes, path);
    if (node) {
      this.setExpanded(node, false);
      this.cdr.markForCheck();
    }
  }

  focusNode(path: string): void {
    this.focusedNodePath = path;
    const node = this.findNodeByPath(this.nodes, path);
    if (node) {
      this.ensureVisible(node);
    }
    this.cdr.markForCheck();
  }

  private findNodeByPath(nodes: TreeNode[], path: string): TreeNode | undefined {
    for (const node of nodes) {
      if (node.path === path) return node;
      if (node.children) {
        const found = this.findNodeByPath(node.children, path);
        if (found) return found;
      }
    }
    return undefined;
  }
}