// file-tree.component.ts
import {
  Component,
  Input,
  Output,
  EventEmitter,
  OnChanges,
  SimpleChanges,
  TrackByFunction,
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
    loaded?: boolean;
    loading?: boolean;
  }

  @Component({
    selector: 'app-file-tree',
    standalone: true,
    imports: [CommonModule],
    template: `
      <div
        class="file-tree-container"
        role="tree"
        tabindex="0"
        [class.focused]="isFocused"
        (focus)="onFocus()"
        (blur)="onBlur()"
        (keydown)="onKeyDown($event)">

        <ng-container *ngTemplateOutlet="treeTemplate; context: { $implicit: nodes, parentLines: [] }"></ng-container>

        <div *ngIf="nodes.length === 0 && !loading" class="tree-empty">
          {{ emptyMessage }}
        </div>

        <div *ngIf="loading" class="tree-loading">
          <span class="spinner"></span>
          Loading...
        </div>
      </div>

      <ng-template #treeTemplate let-nodes let-parentLines="parentLines">
        <div
          *ngFor="let node of nodes; let last = last; trackBy: trackByFn"
          class="tree-node-wrapper"
          [attr.data-path]="node.path">

          <div
            class="tree-node"
            [class.selected]="isSelected(node)"
            [class.focused-node]="isFocusedNode(node)"
            [attr.role]="'treeitem'"
            [attr.aria-level]="node.level + 1"
            [attr.aria-expanded]="node.expandable ? isExpanded(node) : undefined"
            [attr.aria-selected]="isSelected(node)"
            (mousedown)="onNodeMouseDown(node, $event)"
            (click)="onNodeClick(node, $event)"
            (dblclick)="onNodeDoubleClick(node, $event)"
            (contextmenu)="onContextMenu($event, node)">

            <!-- Indent guides -->
            <span
              *ngFor="let guide of getGuides(node, parentLines)"
              class="indent-guide"
              [class.indent-guide--active]="guide.active"
              [class.indent-guide--last]="guide.last">
            </span>

            <!-- Twistie -->
            <span
              class="tree-twistie"
              [class.tree-twistie--expandable]="node.expandable"
              [class.tree-twistie--expanded]="node.expandable && isExpanded(node)"
              (mousedown)="onTwistieMouseDown($event, node)"
              (click)="onTwistieClick($event, node)">

              <ng-container *ngIf="node.expandable">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                  <path d="M6 4L10 8L6 12" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </ng-container>
              <span *ngIf="!node.expandable" class="twistie-placeholder"></span>
            </span>

            <!-- Icon -->
            <span class="tree-icon">
              <ng-container [ngSwitch]="node.type">
                <span *ngSwitchCase="FileType.DIRECTORY">{{ isExpanded(node) ? '📂' : '📁' }}</span>
                <span *ngSwitchCase="FileType.SYMLINK">🔗</span>
                <span *ngSwitchDefault>{{ getCategoryIcon(node.name) }}</span>
              </ng-container>
            </span>

            <!-- Label -->
            <span class="tree-label" [title]="node.path">{{ node.name }}</span>
          </div>

          <!-- Children -->
          <div class="tree-children" *ngIf="isExpanded(node) && (node.loading || (node.children && node.children.length > 0))">
            <ng-container *ngIf="node.children?.length; else emptyOrLoading">
              <ng-container *ngTemplateOutlet="treeTemplate; context: {
                $implicit: node.children,
                parentLines: buildParentLines(parentLines, last)
              }"></ng-container>
            </ng-container>
            <ng-template #emptyOrLoading>
              <div *ngIf="node.loaded && !node.loading" class="tree-empty-folder">
                <span class="indent-guide" *ngFor="let g of buildParentLines(parentLines, last)"></span>
                <span class="tree-twistie"></span>
                <span class="empty-label">Empty folder</span>
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
        color: var(--vsc-foreground);
      }

      .tree-node-wrapper { position: relative; }

      .tree-node {
        display: flex;
        align-items: center;
        height: 22px;
        cursor: pointer;
        white-space: nowrap;
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

      .tree-node.focused-node:not(.selected) {
        outline: 1px dotted var(--vsc-focus-border);
        outline-offset: -1px;
      }

      .indent-guide {
        display: inline-flex;
        flex-shrink: 0;
        width: 20px;
        height: 22px;
        position: relative;
      }

      .indent-guide::before {
        content: '';
        position: absolute;
        left: 7px;
        top: 0;
        bottom: 0;
        width: 1px;
        background: var(--vsc-tree-indent-guide, rgba(255,255,255,0.06));
        pointer-events: none;
      }

      .indent-guide--active::before {
        background: var(--vsc-tree-indent-guide-active, rgba(255,255,255,0.12));
      }

      .indent-guide--last::before {
        bottom: 50%;
      }

      .indent-guide--last::after {
        content: '';
        position: absolute;
        left: 9px;
        top: calc(50% - 0.5px);
        width: 12px;
        height: 1px;
        background: var(--vsc-tree-indent-guide, rgba(255,255,255,0.12));
        pointer-events: none;
      }

      .tree-twistie {
        width: 20px;
        height: 22px;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
        color: var(--vsc-foreground-dim);
      }

      .tree-twistie--expandable { cursor: pointer; }
      .tree-twistie--expandable:hover { color: var(--vsc-foreground); }

      .tree-twistie svg {
        width: 16px;
        height: 16px;
        pointer-events: none;
      }

      .tree-twistie--expanded svg { transform: rotate(90deg); }

      .twistie-placeholder {
        width: 16px;
        height: 16px;
        display: inline-block;
      }

      .tree-icon {
        width: 20px;
        height: 22px;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
        font-size: 14px;
      }

      .tree-label {
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        padding-right: 12px;
        font-size: var(--vsc-font-size);
        margin-left: 3px;
      }

      .tree-children { position: relative; }

      .tree-empty-folder {
        display: flex;
        align-items: center;
        height: 22px;
        color: var(--vsc-foreground-dim);
        font-style: italic;
        font-size: 0.9em;
      }

      .empty-label { margin-left: 4px; }

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
        width: 14px;
        height: 14px;
        border: 2px solid var(--vsc-foreground-dim);
        border-radius: 50%;
        animation: spin 1s linear infinite;
      }

      @keyframes spin {
        to { transform: rotate(360deg); }
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
    @Output() toggleExpand = new EventEmitter<TreeNode>();
    @Output() loadChildren = new EventEmitter<TreeNode>();
    @Output() contextMenu = new EventEmitter<{ event: MouseEvent; node: TreeNode }>();

    private expansionState = new Map<string, boolean>();
    private focusedNodePath: string | null = null;
    private clickTimer: number | null = null;
    private isMouseDownSelecting = false;

    isFocused = false;
    FileType = FileType;

    constructor(private cdr: ChangeDetectorRef) {}

    ngOnChanges(changes: SimpleChanges): void {
      if (changes['nodes']) {
        this.updateNodeHierarchy(this.nodes);
      }
    }

    // ── Expansion ─────────────────────────────────────────────

    isExpanded(node: TreeNode): boolean {
      return this.expansionState.get(node.path) ?? false;
    }

    private setExpanded(node: TreeNode, expanded: boolean): void {
      this.expansionState.set(node.path, expanded);
      if (expanded && !node.loaded && node.expandable && !node.loading) {
        node.loading = true;
        this.loadChildren.emit(node);
      }
    }

    toggleNode(node: TreeNode): void {
      if (node.loading) return;
      this.setExpanded(node, !this.isExpanded(node));
      this.toggleExpand.emit(node);
      this.cdr.markForCheck();
    }

    private updateNodeHierarchy(nodes: TreeNode[], parent?: TreeNode): void {
      for (const node of nodes) {
        node.parent = parent;
        if (node.children) {
          this.updateNodeHierarchy(node.children, node);
        }
      }
    }

    // ── Visible nodes ─────────────────────────────────────────

    getVisibleNodes(): TreeNode[] {
      const result: TreeNode[] = [];
      const traverse = (nodes: TreeNode[]) => {
        for (const node of nodes) {
          result.push(node);
          if (this.isExpanded(node) && node.children?.length) {
            traverse(node.children);
          }
        }
      };
      traverse(this.nodes);
      return result;
    }

    // ── Indent guides ─────────────────────────────────────────

    buildParentLines(parentLines: boolean[], isLastChild: boolean): boolean[] {
      return [...parentLines, !isLastChild];
    }

    getGuides(node: TreeNode, parentLines: boolean[]): { active: boolean; last: boolean }[] {
      const guides: { active: boolean; last: boolean }[] = [];
      for (let i = 0; i < node.level; i++) {
        const isInnermost = i === node.level - 1;
        const isLastChild = isInnermost && !!node.parent && Array.isArray(node.parent.children)
          ? node.parent.children[node.parent.children.length - 1].path === node.path
          : false;

        guides.push({
          active: this.isSelected(node),
          last: isLastChild,
        });
      }
      return guides;
    }

    // ── Icons ──────────────────────────────────────────────────

    getCategoryIcon(filename: string): string {
      const ext = filename.split('.').pop()?.toLowerCase() || '';
      const cat = DEFAULT_FILE_CATEGORIES.find(c => c.extensions.includes(ext));
      return cat ? cat.icon : '📄';
    }

    // ── Selection ──────────────────────────────────────────────

    isSelected(node: TreeNode): boolean {
      return this.selectedFiles.has(node.path);
    }

    isFocusedNode(node: TreeNode): boolean {
      return this.focusedNodePath === node.path;
    }

    private selectNode(node: TreeNode): void {
      this.focusedNodePath = node.path;
      this.selectionChange.emit([node]);
      this.cdr.markForCheck();
    }

    // ── Event handlers ─────────────────────────────────────────

    onFocus(): void {
      this.isFocused = true;
      if (!this.focusedNodePath && this.nodes.length > 0) {
        this.focusedNodePath = this.nodes[0].path;
      }
      this.cdr.markForCheck();
    }

    onBlur(): void {
      this.isFocused = false;
      this.cdr.markForCheck();
    }

    /**
     * MouseDown - Select immediately (VSCode behavior)
     * This happens before click, giving instant visual feedback
     */
    onNodeMouseDown(node: TreeNode, event: MouseEvent): void {
      // Only left mouse button (0) and no twistie click
      if (event.button !== 0) return;
      
      // Don't select if clicking on twistie (handled separately)
      const target = event.target as HTMLElement;
      if (target.closest('.tree-twistie')) return;

      this.isMouseDownSelecting = true;
      this.selectNode(node);
    }

    /**
     * Click - Only handles focus, selection already done on mousedown
     * Kept for compatibility but selection happens immediately on mousedown
     */
    onNodeClick(node: TreeNode, event: MouseEvent): void {
      event.stopPropagation();

      // Clear pending double-click timer
      if (this.clickTimer) {
        clearTimeout(this.clickTimer);
        this.clickTimer = null;
      }

      // If we already selected on mousedown, don't re-select
      // But we need to handle the case where mousedown didn't fire (e.g. drag start)
      if (!this.isMouseDownSelecting) {
        this.selectNode(node);
      }
      this.isMouseDownSelecting = false;

      // Use timer to detect double-click
      this.clickTimer = window.setTimeout(() => {
        this.clickTimer = null;
      }, 250);
    }

    onNodeDoubleClick(node: TreeNode, event: MouseEvent): void {
      event.stopPropagation();

      // Cancel single-click timer
      if (this.clickTimer) {
        clearTimeout(this.clickTimer);
        this.clickTimer = null;
      }

      this.isMouseDownSelecting = false;

      if (node.type === FileType.DIRECTORY && node.expandable) {
        this.toggleNode(node);
      } else {
        this.nodeDoubleClick.emit(node);
        this.navigate.emit(node);
      }
    }

    /**
     * Twistie mouseDown - Stop propagation to prevent node selection
     */
    onTwistieMouseDown(event: MouseEvent, node: TreeNode): void {
      event.stopPropagation();
    }

    onTwistieClick(event: MouseEvent, node: TreeNode): void {
      event.stopPropagation();
      if (!node.expandable) return;

      if (this.clickTimer) {
        clearTimeout(this.clickTimer);
        this.clickTimer = null;
      }

      this.toggleNode(node);
    }

    onContextMenu(event: MouseEvent, node: TreeNode): void {
      event.preventDefault();
      event.stopPropagation();
      
      // Select on right-click too (VSCode behavior)
      this.focusedNodePath = node.path;
      this.selectionChange.emit([node]);
      this.contextMenu.emit({ event, node });
      this.cdr.markForCheck();
    }

    // ── Keyboard navigation ────────────────────────────────────

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
              this.toggleNode(current);
            } else {
              const child = visible[currentIdx + 1];
              if (child && child.parent === current) {
                this.focusedNodePath = child.path;
              }
            }
          }
          break;

        case 'ArrowLeft':
          event.preventDefault();
          if (current) {
            if (current.expandable && this.isExpanded(current)) {
              this.toggleNode(current);
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
            this.selectNode(current);
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
      }

      this.cdr.markForCheck();
    }

    private ensureVisible(node: TreeNode): void {
      requestAnimationFrame(() => {
        const el = document.querySelector(`[data-path="${CSS.escape(node.path)}"]`);
        el?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
      });
    }

    trackByFn: TrackByFunction<TreeNode> = (_, node) => node.path;

    // ── Public API ─────────────────────────────────────────────

    expandNode(path: string): void {
      const node = this.findNodeByPath(this.nodes, path);
      if (node?.expandable) {
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
      if (node) this.ensureVisible(node);
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

    notifyChildrenLoaded(node: TreeNode, children: TreeNode[]): void {
      node.children = children;
      node.loading = false;
      node.loaded = true;
      node.expandable = children.length > 0;
      this.updateNodeHierarchy(children, node);
      this.cdr.markForCheck();
    }
  }