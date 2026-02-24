// file-tree.component.ts
import {
  Component,
  Input,
  Output,
  EventEmitter,
  OnChanges,
  OnDestroy,
  SimpleChanges,
  TrackByFunction,
  ChangeDetectorRef
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { Subject } from 'rxjs';
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
          (click)="onNodeClick(node, $event)"
          (dblclick)="onNodeDoubleClick(node, $event)"
          (contextmenu)="onContextMenu($event, node)">

          <!-- VSCode-style indent guides, one per level -->
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
            (click)="onToggleExpand($event, node)">

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
        <!-- Only render children container when expanded AND there are children or a load is in progress.
             This keeps the twistie rotating for empty folders but prevents showing an empty expansion panel. -->
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

    /* ── VSCode indent guides ─────────────────────────────── */

    .indent-guide {
      display: inline-flex;
      flex-shrink: 0;
      width: 20px;
      height: 22px;
      position: relative;
    }

    /* Default: full vertical line through entire row height */
    .indent-guide::before {
      content: '';
      position: absolute;
      left: 9px;
      top: 0;
      bottom: 0;
      width: 1px;
      background: var(--vsc-tree-indent-guide, rgba(255,255,255,0.12));
      pointer-events: none;
    }

    /* Brighter line when a selected node passes through this level */
    .indent-guide--active::before {
      background: var(--vsc-tree-indent-guide-active, rgba(255,255,255,0.3));
    }

    /*
      Last-child elbow:
      - Vertical line runs only from top down to the row midpoint
      - Horizontal stub then runs right from midpoint toward the twistie
    */
    .indent-guide--last::before {
      /* stop exactly at midpoint to align with horizontal stub */
      bottom: 50%;
    }

    .indent-guide--last::after {
      content: '';
      position: absolute;
      left: 9px;
      /* place the horizontal stub centered on the midpoint and use 1px height for a crisp line */
      top: calc(50% - 0.5px);
      width: 12px;
      height: 1px;
      background: var(--vsc-tree-indent-guide, rgba(255,255,255,0.12));
      pointer-events: none;
      /* avoid subpixel blending where possible */
      transform: translateZ(0);
    }

    /* ── Twistie ──────────────────────────────────────────── */

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

    /* ── Spinner ──────────────────────────────────────────── */

    .spinner-small {
      width: 12px;
      height: 12px;
      border: 1.5px solid var(--vsc-foreground-dim);
      border-radius: 50%;
      flex-shrink: 0;
    }

    /* ── Icon & Label ─────────────────────────────────────── */

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

    /* ── Children ─────────────────────────────────────────── */

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

    /* ── Empty / Loading states ───────────────────────────── */

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
    }
  `]
})
export class FileTreeComponent implements OnChanges, OnDestroy {
  @Input() nodes: TreeNode[] = [];
  @Input() selectedFiles: Set<string> = new Set();
  @Input() loading = false;
  @Input() emptyMessage = 'No items';

  @Output() selectionChange = new EventEmitter<TreeNode[]>();
  @Output() nodeDoubleClick = new EventEmitter<TreeNode>();
  @Output() navigate        = new EventEmitter<TreeNode>();
  @Output() toggleExpand    = new EventEmitter<TreeNode>();
  @Output() loadChildren    = new EventEmitter<TreeNode>();
  @Output() contextMenu     = new EventEmitter<{ event: MouseEvent; node: TreeNode }>();

  /**
   * The container subscribes to this stream and handles each load with exhaustMap,
   * so concurrent requests for the same node are automatically dropped.
   */
  readonly loadRequest$ = new Subject<TreeNode>();

  private expansionState = new Map<string, boolean>();
  private focusedNodePath: string | null = null;
  private lastSelectedPath: string | null = null;
  private destroy$ = new Subject<void>();

  isFocused = false;
  FileType = FileType;

  constructor(private cdr: ChangeDetectorRef) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['nodes']) {
      this.updateNodeHierarchy(this.nodes);
    }
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  // ── Expansion ─────────────────────────────────────────────

  isExpanded(node: TreeNode): boolean {
    return this.expansionState.get(node.path) ?? false;
  }

  /**
   * Update expansion state.
   * - Does NOT write node.expanded (expansionState map is sole source of truth)
   * - Does NOT set node.loading (container owns that)
   * - Emits loadChildren when expanding a not-yet-loaded node
   */
  private setExpanded(node: TreeNode, expanded: boolean): void {
    this.expansionState.set(node.path, expanded);

    if (expanded && !node.loaded && node.expandable && !node.loading) {
      // Set loading=true synchronously HERE before emitting, so any re-click
      // that arrives before the container's async pipeline runs will hit the
      // `if (node.loading) return` guard in toggleNode and be blocked.
      node.loading = true;
      this.loadChildren.emit(node);
    }
  }

  /**
   * Toggle open/closed.
   * Only guard: skip if a load is actively in flight for this node.
   * No debounce, no suppress windows — those caused the need-to-click-3-times bug.
   */
  private toggleNode(node: TreeNode): void {
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

  // ── Visible node list (keyboard nav) ──────────────────────

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

  // ── Indent guide helpers ───────────────────────────────────

  /**
   * Called from the template when descending into a children block.
   * Tracks whether each ancestor level still has more siblings below.
   *
   *   true  = this level has more siblings below → draw full vertical line
   *   false = this was the last sibling           → draw elbow
   */
  buildParentLines(parentLines: boolean[], isLastChild: boolean): boolean[] {
    return [...parentLines, !isLastChild];
  }

  /**
   * Returns one descriptor per indent level for the given node.
   * The innermost level shows the L-shaped elbow when the node is a last child.
   */
  getGuides(node: TreeNode, parentLines: boolean[]): { active: boolean; last: boolean }[] {
    const guides: { active: boolean; last: boolean }[] = [];
    for (let i = 0; i < node.level; i++) {
      const isInnermost     = i === node.level - 1;
      // Determine if this node is the last child of its parent (only meaningful for the innermost level)
      const isLastChild = isInnermost && !!node.parent && Array.isArray(node.parent.children)
        ? node.parent.children[node.parent.children.length - 1].path === node.path
        : false;

      guides.push({
        active: this.isSelected(node),
        // Only mark `last` for the innermost level when this node is actually the last sibling
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

  onNodeClick(node: TreeNode, event: MouseEvent): void {
    event.stopPropagation();
    this.focusedNodePath = node.path;

    const isMulti = event.ctrlKey || event.metaKey;
    const isRange = event.shiftKey;

    if (isRange && this.lastSelectedPath) {
      const visible  = this.getVisibleNodes();
      const startIdx = visible.findIndex(n => n.path === this.lastSelectedPath);
      const endIdx   = visible.findIndex(n => n.path === node.path);
      const [min, max] = [Math.min(startIdx, endIdx), Math.max(startIdx, endIdx)];
      const newSelection: TreeNode[] = [];
      for (let i = min; i <= max; i++) {
        if (i >= 0 && i < visible.length) newSelection.push(visible[i]);
      }
      this.selectionChange.emit(newSelection);
    } else if (isMulti) {
      const current  = Array.from(this.selectedFiles);
      const exists   = current.includes(node.path);
      const newPaths = exists
        ? current.filter(p => p !== node.path)
        : [...current, node.path];
      const selectedNodes = this.getVisibleNodes().filter(n => newPaths.includes(n.path));
      this.selectionChange.emit(selectedNodes);
      this.lastSelectedPath = node.path;
    } else {
      this.selectionChange.emit([node]);
      this.lastSelectedPath = node.path;
    }

    this.cdr.markForCheck();
  }

  onNodeDoubleClick(node: TreeNode, event: MouseEvent): void {
    event.stopPropagation();
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

  onContextMenu(event: MouseEvent, node: TreeNode): void {
    event.preventDefault();
    event.stopPropagation();
    this.focusedNodePath = node.path;
    // removed: selectionChange.emit — this was causing the pane to rebuild the tree
    this.contextMenu.emit({ event, node });
    this.cdr.markForCheck();
  }

  // ── Keyboard navigation ────────────────────────────────────

  onKeyDown(event: KeyboardEvent): void {
    const visible = this.getVisibleNodes();
    if (visible.length === 0) return;

    const currentIdx = visible.findIndex(n => n.path === this.focusedNodePath);
    const current    = currentIdx >= 0 ? visible[currentIdx] : null;

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
          this.onNodeClick(current, new MouseEvent('click', {
            ctrlKey:  event.ctrlKey,
            shiftKey: event.shiftKey,
          }));
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
        event.preventDefault();
        (current?.parent?.children ?? this.nodes).forEach(node => {
          if (node.expandable && !this.isExpanded(node)) this.toggleNode(node);
        });
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
    if (node?.expandable) { this.setExpanded(node, true); this.cdr.markForCheck(); }
  }

  collapseNode(path: string): void {
    const node = this.findNodeByPath(this.nodes, path);
    if (node) { this.setExpanded(node, false); this.cdr.markForCheck(); }
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
}