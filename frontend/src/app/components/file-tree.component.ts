// file-tree.component.ts - FULLY OPTIMIZED FOR MASSIVE TREES
import {
  Component,
  Input,
  Output,
  EventEmitter,
  OnChanges,
  SimpleChanges,
  TrackByFunction,
  ChangeDetectorRef,
  ElementRef,
  ViewChild,
  ChangeDetectionStrategy,
  OnDestroy,
  AfterViewInit,
  NgZone,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { FileEntryInlineComponent } from './file-entry-inline.component';
import { FileInfo, FileType } from '../shared/protocol';
import { getPrimaryFileCategoryForExtension } from '../shared/file-categories';
import { normalizePath as normalizePathReference } from './address-bar.utils';
import { VscodeScrollbarDirective } from '../directives/vscode-scrollbar.directive';
import { FileCategoryResolverService } from '../services/file-category-resolver.service';

export interface TreeNode extends FileInfo {
  level: number;
  syntheticRoot?: string;
  archivePath?: string;
  expanded?: boolean;
  expandable?: boolean;
  children?: TreeNode[];
  parent?: TreeNode;
  loaded?: boolean;
  loading?: boolean;
  renaming?: boolean;
  editingName?: string;
}

export interface DropTarget {
  node: TreeNode;
  position: 'before' | 'after' | 'inside';
}

@Component({
  selector: 'app-file-tree',
  standalone: true,
  imports: [CommonModule, FormsModule, FileEntryInlineComponent, VscodeScrollbarDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      #treeContainer
      class="file-tree-container"
      appVscodeScrollbar
      role="tree"
      tabindex="0"
      [class.focused]="paneActive || isFocused"
      [class.dragging]="dragState.isDragging"
      (focus)="onFocus()"
      (blur)="onBlur()"
      (mousedown)="onContainerMouseDown($event)"
      (click)="onContainerClick($event)"
      (contextmenu)="onContainerContextMenu($event)"
      (keydown)="onKeyDown($event)"
      (dragover)="onContainerDragOver($event)"
      (drop)="onContainerDrop($event)">

      <div class="tree-spacer" *ngIf="virtualEnabled" [style.height.px]="totalContentHeight"></div>
      
      <div class="tree-window" [class.virtual-window]="virtualEnabled" [style.transform]="virtualWindowTransform">
        <div
          *ngFor="let node of visibleWindowNodes; trackBy: trackByFn"
          class="tree-node-wrapper"
          [attr.data-path]="node.path"
          [class.drag-over]="isDragOver(node)"
          [class.drag-over-before]="isDragOver(node, 'before')"
          [class.drag-over-after]="isDragOver(node, 'after')"
          [class.drag-over-inside]="isDragOver(node, 'inside')">

          <div *ngIf="isDragOver(node, 'before')" class="drop-indicator drop-indicator-before"></div>
          <div *ngIf="isDragOver(node, 'after')" class="drop-indicator drop-indicator-after"></div>

          <div
            class="tree-node"
            [class.selected]="isSelected(node)"
            [class.focused-node]="isFocusedNode(node)"
            [class.dragging-node]="isDraggingNode(node)"
            [attr.role]="'treeitem'"
            [attr.aria-level]="node.level + 1"
            [attr.aria-expanded]="node.expandable ? isExpanded(node) : undefined"
            [attr.aria-selected]="isSelected(node)"
            [draggable]="true"
            (mousedown)="onNodeMouseDown(node, $event)"
            (click)="onNodeClick(node, $event)"
            (auxclick)="onNodeAuxClick(node, $event)"
            (dblclick)="onNodeDoubleClick(node, $event)"
            (contextmenu)="onContextMenu($event, node)"
            (dragstart)="onDragStart($event, node)"
            (dragend)="onDragEnd($event)"
            (dragover)="onDragOver($event, node)"
            (dragleave)="onDragLeave($event, node)"
            (drop)="onDrop($event, node)">

            <span
              *ngFor="let guide of getGuides(node)"
              class="indent-guide"
              [class.indent-guide--active]="guide.active"
              [class.indent-guide--last]="guide.last"
              [class.indent-guide--skip]="guide.skip">
            </span>

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

            <app-file-entry-inline
              *ngIf="!node.renaming"
              class="tree-entry"
              [entry]="node"
              [displayNameOverride]="getDisplayNameOverride(node.path)"
              [directorySizePending]="isDirectorySizePending(node.path)"
              [showDirectorySizeSpinnerInName]="true"
              [colorizeByMime]="colorizeByMime"
              >
            </app-file-entry-inline>
            
            <input
              *ngIf="node.renaming"
              #renameInput
              class="tree-rename-input"
              type="text"
              [(ngModel)]="node.editingName"
              (blur)="onRenameBlur(node)"
              (keydown)="onRenameKeyDown($event, node)"
              (click)="$event.stopPropagation()"
              (mousedown)="$event.stopPropagation()"/>
          </div>
        </div>
      </div>

      <div *ngIf="loading" class="tree-loading">
        <span class="spinner"></span>
        Loading...
      </div>

      <div
        *ngIf="showSelectionRect"
        class="selection-rect"
        [style.left.px]="selectionRect.left"
        [style.top.px]="selectionRect.top"
        [style.width.px]="selectionRect.width"
        [style.height.px]="selectionRect.height">
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1;
      min-height: 0;
      overflow: hidden;
      contain: layout style paint;
    }

    .file-tree-container {
      position: relative;
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
    
    .tree-spacer {
      width: 1px;
      pointer-events: none;
    }
    
    .tree-window {
      display: block;
    }
    
    .tree-window.virtual-window {
      position: absolute;
      left: 0;
      right: 0;
      top: 0;
      will-change: transform;
    }

    .file-tree-container.focused .tree-node.selected {
      outline: none;
      background-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 26%, var(--vsc-list-focus-background, var(--vsc-list-active-background)) 74%);
      box-shadow: inset 2px 0 0 var(--vsc-accent-blue, #0078d4);
    }

    .tree-node-wrapper { 
      position: relative; 
    }

    .tree-node {
      display: flex;
      align-items: center;
      height: 22px;
      cursor: pointer;
      white-space: nowrap;
      position: relative;
      contain: layout style paint;
    }

    .tree-node.selected {
      background-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 18%, var(--vsc-list-active-background) 82%);
      color: var(--vsc-list-active-foreground, var(--vsc-foreground-bright));
      box-shadow: inset 2px 0 0 color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 82%, white 18%);
    }

    .tree-node:hover:not(.selected) {
      background-color: var(--vsc-list-hover-background);
    }

    .tree-node.focused-node:not(.selected) {
      outline: 1px dotted var(--vsc-focus-border);
      outline-offset: -1px;
    }

    .tree-node.dragging-node {
      opacity: 0.5;
    }

    /* During drag we only show the thin indicators; do not highlight the row itself */
    .tree-node-wrapper.drag-over > .tree-node {
      background-color: transparent;
    }

    .drop-indicator {
      position: absolute;
      left: 0;
      right: 0;
      height: 3px;
      background-color: var(--vsc-focus-border, #0078d4);
      pointer-events: none;
      z-index: 10;
    }

    .drop-indicator-before {
      top: -1px;
    }

    .drop-indicator-after {
      bottom: -1px;
    }

    /* When dropping inside a folder, give it a soft highlight plus the outline */
    .tree-node-wrapper.drag-over-inside > .tree-node {
      background-color: var(--vsc-list-drop-background, rgba(0, 120, 212, 0.16));
      outline: 1px dashed var(--vsc-focus-border);
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

    .indent-guide--skip::before,
    .indent-guide--skip::after {
      display: none;
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

    .dir-symlink-icon {
      position: relative;
      &::after {
        content: '↗';
        position: absolute;
        bottom: -1px;
        right: -5px;
        font-size: 8px;
        line-height: 1;
        opacity: 0.8;
      }
    }

    .tree-entry {
      flex: 1;
      min-width: 0;
      margin-left: 3px;
      pointer-events: none;
    }

    .tree-rename-input {
      flex: 1;
      height: 18px;
      margin: 0 4px;
      padding: 0 4px;
      border: 1px solid var(--vsc-focus-border);
      background: var(--vsc-input-background);
      color: var(--vsc-foreground);
      font-family: inherit;
      font-size: inherit;
      outline: none;
    }

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
      border-bottom-color: transparent;
      border-radius: 50%;
      animation: spin 1s linear infinite;
    }

    .selection-rect {
      position: absolute;
      pointer-events: none;
      z-index: 20;
      border: 1px solid var(--vsc-focus-border);
      background: color-mix(in srgb, var(--vsc-list-active-background) 45%, transparent);
    }

    @keyframes spin {
      to { transform: rotate(360deg); }
    }
  `]
})
export class FileTreeComponent implements OnChanges, OnDestroy, AfterViewInit {
  @Input() nodes: TreeNode[] = [];
  @Input() pendingDirectorySizePaths: ReadonlySet<string> = new Set();
  @Input() currentPath = '';
  @Input() paneActive = true;
  @Input() colorizeByMime = false;
  @Input() selectedFiles: Set<string> = new Set();
  @Input() loading = false;
  @Input() emptyMessage = 'No items';

  @Output() selectionChange = new EventEmitter<TreeNode[]>();
  @Output() inlineSize = new EventEmitter<TreeNode>();
  @Output() nodeDoubleClick = new EventEmitter<TreeNode>();
  @Output() navigate = new EventEmitter<TreeNode>();
  @Output() openInNewTab = new EventEmitter<TreeNode>();
  @Output() toggleExpand = new EventEmitter<TreeNode>();
  @Output() loadChildren = new EventEmitter<TreeNode>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; node: TreeNode }>();
  @Output() backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() dragStart = new EventEmitter<{ event: DragEvent; nodes: TreeNode[] }>();
  @Output() dragDrop = new EventEmitter<{ event: DragEvent; source: TreeNode[]; target: DropTarget }>();
  @Output() rename = new EventEmitter<{ node: TreeNode; newName: string }>();

  @ViewChild('renameInput') renameInput?: ElementRef<HTMLInputElement>;
  @ViewChild('treeContainer') treeContainer?: ElementRef<HTMLElement>;

  private expansionState = new Map<string, boolean>();
  private pendingRenameDisplayNames = new Map<string, string>();
  private suppressNextAutoExpand = false;
  private focusedNodePath: string | null = null;
  private anchorNodePath: string | null = null;
  private clickTimer: number | null = null;
  private isMouseDownSelecting = false;

  dragState = {
    isDragging: false,
    draggedNodes: [] as TreeNode[],
    dropTarget: null as DropTarget | null
  };

  isFocused = false;
  FileType = FileType;

  isDirectorySizePending(path: string): boolean {
    return this.pendingDirectorySizePaths.has(path);
  }

  // OPTIMIZED: Virtual scroll for massive trees
  private readonly VIRTUAL_THRESHOLD = 500;
  private readonly VIRTUAL_ROW_HEIGHT = 22;
  private readonly VIRTUAL_OVERSCAN = 20;
  private scrollRAF: number | null = null;
  private boundScrollListener?: () => void;
  private resizeObserver?: ResizeObserver;
  
  private flatVisibleNodes: TreeNode[] = [];
  private virtualStart = 0;
  private virtualEnd = 0;
  private lastScrollTop = 0;
  private viewportHeight = 0;
  private visibleWindowNodesCache: TreeNode[] = [];
  private visibleNodeIndex = new Map<string, number>();

  constructor(
    private cdr: ChangeDetectorRef,
    private zone: NgZone,
    private categoryResolver: FileCategoryResolverService,
  ) {}

  // ── Marquee selection state ───────────────────────────────────────────────
  selectionRect = { active: false, left: 0, top: 0, width: 0, height: 0 };

  get showSelectionRect(): boolean {
    return this.selectionRect.active && this.marqueeButton !== 2;
  }

  private marqueeContainer: HTMLElement | null = null;
  private marqueeStartX = 0;
  private marqueeStartY = 0;
  private marqueeAppend = false;
  private marqueeButton: 0 | 2 | null = null;
  private hasMarqueeMoved = false;
  private pendingClearOnMove = false;
  private suppressContainerClick = false;
  private suppressContainerContextMenu = false;
  private suppressItemContextMenu = false;
  private windowMouseMove?: (event: MouseEvent) => void;
  private windowMouseUp?: (event: MouseEvent) => void;
  private windowContextMenu?: (event: MouseEvent) => void;

  get visibleNodeCount(): number {
    return this.flatVisibleNodes.length;
  }

  get virtualEnabled(): boolean {
    return this.visibleNodeCount >= this.VIRTUAL_THRESHOLD;
  }

  get visibleWindowNodes(): TreeNode[] {
    return this.visibleWindowNodesCache;
  }

  get totalContentHeight(): number {
    return this.visibleNodeCount * this.VIRTUAL_ROW_HEIGHT;
  }

  get virtualWindowTransform(): string {
    if (!this.virtualEnabled) return '';
    return `translateY(${this.virtualStart * this.VIRTUAL_ROW_HEIGHT}px)`;
  }

  get renderedNodes(): TreeNode[] {
    const root = this.nodes[0];
    if (this.nodes.length === 1 && typeof root?.syntheticRoot === 'string') {
      return root.children ?? [];
    }
    return this.nodes;
  }

  ngAfterViewInit(): void {
    this.setupResizeObserver();
    this.setupScrollListener();
    this.refreshVisibleNodes();
    this.updateVirtualWindow(true);
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['nodes'] || changes['colorizeByMime']) {
      this.prewarmCollisionResolutions(this.nodes);
      if (changes['nodes']) {
        this.updateNodeHierarchy(this.nodes);
      }
    }
    const shouldExpandToCurrentPath = !!changes['currentPath'] || (!!changes['nodes'] && !this.suppressNextAutoExpand);
    if (shouldExpandToCurrentPath) {
      this.expandToCurrentPath();
    }
    if (changes['nodes'] && this.suppressNextAutoExpand) {
      this.suppressNextAutoExpand = false;
    }
    this.refreshVisibleNodes();
    this.updateVirtualWindow(true);
    this.cdr.markForCheck();
  }

  private prewarmCollisionResolutions(nodes: TreeNode[]): void {
    if (!this.colorizeByMime) {
      return;
    }
    for (const node of nodes) {
      this.categoryResolver.ensureResolved(node);
      if (node.children?.length) {
        this.prewarmCollisionResolutions(node.children);
      }
    }
  }

  ngOnDestroy(): void {
    this.detachMarqueeListeners();
    this.cleanupResizeObserver();
    this.cleanupScrollListener();
    if (this.scrollRAF) {
      cancelAnimationFrame(this.scrollRAF);
    }
  }

  private setupResizeObserver(): void {
    const element = this.treeContainer?.nativeElement;
    if (!element) return;

    this.zone.runOutsideAngular(() => {
      this.resizeObserver = new ResizeObserver((entries) => {
        let shouldMark = false;
        for (const entry of entries) {
          if (this.viewportHeight !== entry.contentRect.height) {
            this.viewportHeight = entry.contentRect.height;
            shouldMark = this.updateVirtualWindow() || shouldMark;
          }
        }
        if (shouldMark) {
          this.zone.run(() => this.cdr.markForCheck());
        }
      });

      this.resizeObserver.observe(element);
    });
  }

  private cleanupResizeObserver(): void {
    this.resizeObserver?.disconnect();
  }

  private setupScrollListener(): void {
    const element = this.treeContainer?.nativeElement;
    if (!element) return;
    this.boundScrollListener = () => this.onScroll();
    this.zone.runOutsideAngular(() => {
      element.addEventListener('scroll', this.boundScrollListener!, { passive: true });
    });
  }

  private cleanupScrollListener(): void {
    const element = this.treeContainer?.nativeElement;
    if (!element || !this.boundScrollListener) return;
    element.removeEventListener('scroll', this.boundScrollListener);
    this.boundScrollListener = undefined;
  }

  private onScroll(): void {
    if (this.scrollRAF) return;
    this.scrollRAF = requestAnimationFrame(() => {
      this.scrollRAF = null;
      if (this.updateVirtualWindow()) {
        this.zone.run(() => this.cdr.markForCheck());
      }
    });
  }

  private updateVirtualWindow(forceRebuild = false): boolean {
    let changed = forceRebuild;
    if (!this.virtualEnabled) {
      if (this.virtualStart !== 0 || this.virtualEnd !== this.flatVisibleNodes.length) {
        this.virtualStart = 0;
        this.virtualEnd = this.flatVisibleNodes.length;
        changed = true;
      }
      if (changed) {
        this.rebuildVisibleWindowNodes();
      }
      return changed;
    }

    const host = this.treeContainer?.nativeElement;
    const viewportHeight = Math.max(1, this.viewportHeight || host?.clientHeight || 1);
    const scrollTop = host?.scrollTop ?? 0;

    if (Math.abs(scrollTop - this.lastScrollTop) < this.VIRTUAL_ROW_HEIGHT && this.virtualEnd > 0) {
      if (changed) {
        this.rebuildVisibleWindowNodes();
      }
      return changed;
    }
    
    this.lastScrollTop = scrollTop;

    const visibleRows = Math.max(1, Math.ceil(viewportHeight / this.VIRTUAL_ROW_HEIGHT));
    const start = Math.max(0, Math.floor(scrollTop / this.VIRTUAL_ROW_HEIGHT) - this.VIRTUAL_OVERSCAN);
    const end = Math.min(this.flatVisibleNodes.length, start + visibleRows + this.VIRTUAL_OVERSCAN * 2);

    if (start !== this.virtualStart || end !== this.virtualEnd) {
      this.virtualStart = start;
      this.virtualEnd = end;
      changed = true;
    }
    if (changed) {
      this.rebuildVisibleWindowNodes();
    }
    return changed;
  }

  private refreshVisibleNodes(): void {
    this.flatVisibleNodes = this.getVisibleNodes();
    this.visibleNodeIndex.clear();
    this.flatVisibleNodes.forEach((node, index) => this.visibleNodeIndex.set(node.path, index));
    if (!this.virtualEnabled) {
      this.virtualStart = 0;
      this.virtualEnd = this.flatVisibleNodes.length;
    }
    this.rebuildVisibleWindowNodes();
  }

  private rebuildVisibleWindowNodes(): void {
    this.visibleWindowNodesCache = this.virtualEnabled
      ? this.flatVisibleNodes.slice(this.virtualStart, this.virtualEnd)
      : this.flatVisibleNodes;
  }

  isExpanded(node: TreeNode): boolean {
    return this.expansionState.get(node.path) ?? false;
  }

  private setExpanded(node: TreeNode, expanded: boolean): void {
    this.expansionState.set(node.path, expanded);
    if (expanded && !node.loaded && node.expandable && !node.loading) {
      node.loading = true;
      this.loadChildren.emit(node);
    }
    this.refreshVisibleNodes();
    this.updateVirtualWindow(true);
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
    traverse(this.renderedNodes);
    return result;
  }

  isNodeExpanded(path: string): boolean {
    const node = this.findNodeByPath(this.nodes, path);
    return !!node && this.isExpanded(node);
  }

  suppressNextAutoExpandToCurrentPath(): void {
    this.suppressNextAutoExpand = true;
  }

  getGuides(node: TreeNode): { active: boolean; last: boolean; skip: boolean }[] {
    // Do not draw guide lines for top-level entries under a synthetic root.
    if (typeof node.parent?.syntheticRoot === 'string') {
      return [];
    }

    const chain: TreeNode[] = [];
    let cur: TreeNode | undefined = node;
    while (cur?.parent) {
      if (typeof cur.parent.syntheticRoot === 'string') {
        break;
      }
      chain.push(cur);
      cur = cur.parent;
    }
    chain.reverse();

    return chain.map((n, index, arr) => {
      const siblings = n.parent?.children ?? [];
      const isLast = siblings.length > 0 && siblings[siblings.length - 1].path === n.path;
      const isDeepest = index === arr.length - 1;
      
      // Skip rendering guide lines for ancestors that are last children (to avoid vertical continuation).
      // Only the deepest node should show an elbow if it's last.
      const skip = isLast && !isDeepest;
      const shouldMarkAsLast = isLast && isDeepest;
      
      return { active: this.isSelected(node), last: shouldMarkAsLast, skip };
    });
  }

  getCategoryIcon(filename: string): string {
    const ext = filename.split('.').pop()?.toLowerCase() || '';
    const cat = getPrimaryFileCategoryForExtension(ext);
    return cat ? cat.icon : '📄';
  }

  isSelected(node: TreeNode): boolean {
    return this.selectedFiles.has(node.path);
  }

  isFocusedNode(node: TreeNode): boolean {
    return this.focusedNodePath === node.path;
  }

  private selectNodeAndAdvance(node: TreeNode, visible: TreeNode[]): void {
    this.selectNode(node, false, false);

    const currentIdx = visible.findIndex(candidate => candidate.path === node.path);
    if (currentIdx === -1) return;

    const nextNode = visible[Math.min(currentIdx + 1, visible.length - 1)];
    if (!nextNode || nextNode.path === node.path) return;

    this.focusedNodePath = nextNode.path;
    this.ensureVisible(nextNode);
  }

  private selectNode(node: TreeNode, addToSelection = false, range = false): void {
    const visible = this.getVisibleNodes();
    let newSelection: TreeNode[];

    if (range && this.anchorNodePath) {
      const anchorIdx = visible.findIndex(n => n.path === this.anchorNodePath);
      const currentIdx = visible.findIndex(n => n.path === node.path);
      
      if (anchorIdx !== -1 && currentIdx !== -1) {
        const start = Math.min(anchorIdx, currentIdx);
        const end = Math.max(anchorIdx, currentIdx);
        newSelection = visible.slice(start, end + 1);
      } else {
        newSelection = [node];
      }
    } else if (addToSelection) {
      const currentSelection = this.getSelectedNodes();
      if (this.isSelected(node)) {
        newSelection = currentSelection.filter(n => n.path !== node.path);
      } else {
        newSelection = [...currentSelection, node];
      }
      this.anchorNodePath = node.path;
    } else {
      newSelection = [node];
      this.anchorNodePath = node.path;
    }

    this.focusedNodePath = node.path;
    this.selectionChange.emit(newSelection);
    this.cdr.markForCheck();
  }

  private getSelectedNodes(): TreeNode[] {
    const result: TreeNode[] = [];
    const traverse = (nodes: TreeNode[]) => {
      for (const node of nodes) {
        if (this.isSelected(node)) result.push(node);
        if (node.children) traverse(node.children);
      }
    };
    traverse(this.nodes);
    return result;
  }

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

  onNodeMouseDown(node: TreeNode, event: MouseEvent): void {
    if (event.button !== 0) return;
    
    const target = event.target as HTMLElement;
    if (target.closest('.tree-twistie')) return;

    // Do not change selection on mousedown; wait for click or keyboard.
    // This keeps drag initiation from altering selection when drawing the drop indicator.
    this.isMouseDownSelecting = false;
  }

  onNodeClick(node: TreeNode, event: MouseEvent): void {
    event.stopPropagation();

    if (this.clickTimer) {
      clearTimeout(this.clickTimer);
      this.clickTimer = null;
    }

    const isCtrl = event.ctrlKey || event.metaKey;
    const isShift = event.shiftKey;
    this.selectNode(node, isCtrl, isShift);
    this.isMouseDownSelecting = false;

    this.clickTimer = window.setTimeout(() => {
      this.clickTimer = null;
    }, 250);
  }

  onNodeDoubleClick(node: TreeNode, event: MouseEvent): void {
    event.stopPropagation();

    if (this.clickTimer) {
      clearTimeout(this.clickTimer);
      this.clickTimer = null;
    }

    this.isMouseDownSelecting = false;

    if (node.type === FileType.FILE || node.type === FileType.SYMLINK) {
      this.nodeDoubleClick.emit(node);
      return;
    }

    if ((node.type === FileType.DIRECTORY || node.type === FileType.ARCHIVE) && node.expandable) {
      this.toggleNode(node);
    } else {
      this.nodeDoubleClick.emit(node);
    }
  }

  onNodeAuxClick(node: TreeNode, event: MouseEvent): void {
    if (event.button !== 1) return;
    event.preventDefault();
    event.stopPropagation();
    this.openInNewTab.emit(node);
  }

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

    if (!this.isSelected(node)) {
      this.selectNode(node, true, false);
    }
    
    this.focusedNodePath = node.path;
    this.contextMenu.emit({ event, node });
    this.cdr.markForCheck();
  }

  isDraggingNode(node: TreeNode): boolean {
    return this.dragState.isDragging && this.dragState.draggedNodes.some(n => n.path === node.path);
  }

  isDragOver(node: TreeNode, position?: 'before' | 'after' | 'inside'): boolean {
    if (!this.dragState.dropTarget) return false;
    if (this.dragState.dropTarget.node.path !== node.path) return false;
    if (!position) return true;
    return this.dragState.dropTarget.position === position;
  }

  onDragStart(event: DragEvent, node: TreeNode): void {
    if (event.button !== 0) {
      event.preventDefault();
      return;
    }

    // Do not alter selection when beginning a drag. Use existing selection set; if none, drag the node itself.
    const selectedNodes = this.getSelectedNodes();
    const dragNodes = selectedNodes.length ? selectedNodes : [node];
    this.dragState.isDragging = true;
    this.dragState.draggedNodes = dragNodes;

    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move';
      event.dataTransfer.setData('application/json', JSON.stringify({
        entries: dragNodes.map(n => ({ path: n.path, type: n.type, name: n.name })),
        source: 'tree'
      }));
      
      if (dragNodes.length > 1) {
        const dragImage = document.createElement('div');
        dragImage.textContent = `${dragNodes.length} items`;
        dragImage.style.cssText = `
          background: var(--vsc-list-active-background);
          color: var(--vsc-foreground);
          padding: 4px 8px;
          border-radius: 3px;
          font-size: 12px;
          position: fixed;
          top: -1000px;
        `;
        document.body.appendChild(dragImage);
        event.dataTransfer.setDragImage(dragImage, 0, 0);
        setTimeout(() => document.body.removeChild(dragImage), 0);
      }
    }

    this.dragStart.emit({ event, nodes: selectedNodes });
    this.cdr.markForCheck();
  }

  onDragEnd(event: DragEvent): void {
    this.dragState.isDragging = false;
    this.dragState.draggedNodes = [];
    this.dragState.dropTarget = null;
    this.cdr.markForCheck();
  }

  onDragOver(event: DragEvent, node: TreeNode): void {
    event.preventDefault();
    
    // Handle external files (from OS file manager)
    if (this.hasExternalFiles(event)) {
      if (event.dataTransfer) {
        event.dataTransfer.dropEffect = event.ctrlKey || event.metaKey ? 'copy' : 'move';
      }
      // Set simple drop target for external files
      const position = node.type === FileType.DIRECTORY ? 'inside' : 'after';
      if (this.dragState.dropTarget?.node.path === node.path && this.dragState.dropTarget.position === position) {
        return;
      }
      this.dragState.dropTarget = { node, position };
      this.cdr.markForCheck();
      return;
    }
    
    // Internal drag: from tree itself, or cross-component (list/grid/details → tree)
    const isInternalDrag = this.dragState.isDragging || (event.dataTransfer?.types ?? []).includes('application/json');
    if (!isInternalDrag) return;

    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    const y = event.clientY - rect.top;
    const height = rect.height;
    
    let position: 'before' | 'after' | 'inside';
    
    if (node.expandable && y > height * 0.25 && y < height * 0.75) {
      position = 'inside';
    } else if (y < height / 2) {
      position = 'before';
    } else {
      position = 'after';
    }

    // For tree-internal drags, validate against dragged nodes to prevent dropping onto self
    if (this.dragState.isDragging) {
      const isValidDrop = !this.dragState.draggedNodes.some(dragged => 
        dragged.path === node.path || this.isDescendant(node, dragged)
      );
      if (!isValidDrop) return;
    }

    if (this.dragState.dropTarget?.node.path === node.path && this.dragState.dropTarget.position === position) {
      return;
    }
    this.dragState.dropTarget = { node, position };
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = event.ctrlKey || event.metaKey ? 'copy' : 'move';
    }

    this.cdr.markForCheck();
  }

  onDragLeave(event: DragEvent, node: TreeNode): void {
    if (this.dragState.dropTarget?.node.path === node.path) {
      this.dragState.dropTarget = null;
      this.cdr.markForCheck();
    }
  }

  onDrop(event: DragEvent, node: TreeNode): void {
    event.preventDefault();

    // For external OS drops, don't stopPropagation - let Wails' window handler fire wails:file-drop
    if (this.hasExternalFiles(event)) {
      this.dragState.dropTarget = null;
      this.cdr.markForCheck();
      return;
    }

    event.stopPropagation();

    if (!this.dragState.dropTarget) return;

    // Use draggedNodes when drag started within the tree; fall back to dataTransfer for cross-component drags
    let sourceNodes: TreeNode[] = this.dragState.draggedNodes;
    if (!sourceNodes.length) {
      const dragged = this.readDraggedEntries(event);
      sourceNodes = dragged.map(d => ({
        name: d.name,
        path: d.path,
        type: d.type as FileType,
        size: 0,
        created: 0,
        modified: 0,
        accessed: 0,
        permissions: '',
        isHidden: false,
        level: 0,
      } as TreeNode));
    }

    if (!sourceNodes.length) {
      this.dragState.dropTarget = null;
      this.cdr.markForCheck();
      return;
    }

    this.dragDrop.emit({
      event,
      source: sourceNodes,
      target: this.dragState.dropTarget
    });

    this.dragState.dropTarget = null;
    this.cdr.markForCheck();
  }

  onContainerDragOver(event: DragEvent): void {
    event.preventDefault();
  }

  onContainerDrop(event: DragEvent): void {
    event.preventDefault();

    // For external OS drops, don't stopPropagation - let Wails' window handler fire wails:file-drop
    if (this.hasExternalFiles(event)) {
      return;
    }

    event.stopPropagation();
  }

  private readDraggedEntries(event: DragEvent): { path: string; type: FileType; name: string }[] {
    try {
      const payload = event.dataTransfer?.getData('application/json');
      if (!payload) return [];
      const parsed = JSON.parse(payload);
      if (Array.isArray(parsed?.entries)) return parsed.entries;
      if (Array.isArray(parsed?.paths)) return parsed.paths.map((p: string) => ({ path: p, type: FileType.FILE, name: p.split(/[\/\\]/).pop() || p }));
      return [];
    } catch {
      return [];
    }
  }

  private hasExternalFiles(event: DragEvent): boolean {
    const types = event.dataTransfer?.types;
    if (!types) return false;
    if (types.includes('application/json')) return false;
    return types.includes('Files') || (event.dataTransfer?.files?.length ?? 0) > 0;
  }

  private extractExternalPaths(event: DragEvent): string[] {
    const dataTransfer = event.dataTransfer;
    if (!dataTransfer) return [];

    const paths = new Set<string>();

    const files = dataTransfer.files;
    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      const filePath = (file as any).path as string | undefined;
      if (filePath) {
        paths.add(this.normalizePath(filePath));
      }
    }

    const uriPayload = dataTransfer.getData('text/uri-list') || dataTransfer.getData('text/plain');
    if (uriPayload) {
      for (const rawLine of uriPayload.split(/\r?\n/)) {
        const line = rawLine.trim();
        if (!line || line.startsWith('#')) continue;
        const parsedPath = this.fileUriToPath(line);
        if (parsedPath) paths.add(parsedPath);
      }
    }

    return Array.from(paths);
  }

  private fileUriToPath(value: string): string | null {
    if (!value) return null;
    const trimmed = value.trim();
    if (!/^file:\/\//i.test(trimmed)) {
      if (/^[A-Za-z]:[\\/]/.test(trimmed)) return this.normalizePath(trimmed);
      if (/^\\\\/.test(trimmed)) return this.normalizePath(trimmed);
      if (trimmed.startsWith('/')) return this.normalizePath(trimmed);
      return null;
    }

    try {
      let decoded = decodeURIComponent(trimmed).replace(/^file:\/\//i, '');

      if (/^\/[A-Za-z]:\//.test(decoded)) {
        decoded = decoded.slice(1);
      }

      if (/^[A-Za-z]:\//.test(decoded)) {
        return this.normalizePath(decoded.replace(/\//g, '\\'));
      }

      if (!decoded.startsWith('/')) {
        return this.normalizePath(('//' + decoded).replace(/\//g, '\\'));
      }

      return this.normalizePath(decoded);
    } catch {
      return null;
    }
  }

  onContainerMouseDown(event: MouseEvent): void {
    if (event.button !== 0 && event.button !== 2) return;

    // Left-button on a node is handled by click/drag-and-drop — skip.
    // Right-button on a node should still start marquee selection (TC behaviour).
    const target = event.target as HTMLElement;
    if (event.button === 0 && target.closest('.tree-node')) return;

    const container = event.currentTarget as HTMLElement;
    this.marqueeContainer = container;
    this.marqueeAppend = event.ctrlKey || event.metaKey;
    this.marqueeButton = event.button as 0 | 2;
    this.hasMarqueeMoved = false;
    // Defer clear until marquee actually moves — plain click keeps cursor visible
    this.pendingClearOnMove = !this.marqueeAppend;

    const { x, y } = this.getRelativePoint(event, container);
    this.marqueeStartX = x;
    this.marqueeStartY = y;
    this.selectionRect = { active: true, left: x, top: y, width: 0, height: 0 };

    this.suppressContainerClick = true;
    this.suppressContainerContextMenu = event.button === 2;

    this.windowMouseMove = (e: MouseEvent) => this.zone.run(() => this.onMarqueeMouseMove(e));
    this.windowMouseUp = () => this.zone.run(() => this.finishMarqueeSelection());
    this.windowContextMenu = (e: MouseEvent) => {
      if (this.selectionRect.active && this.hasMarqueeMoved) {
        e.preventDefault();
        e.stopPropagation();
      }
    };

    window.addEventListener('mousemove', this.windowMouseMove);
    window.addEventListener('mouseup', this.windowMouseUp);
    window.addEventListener('contextmenu', this.windowContextMenu, true);

    if (event.button === 0) {
      event.preventDefault();
    }
  }

  private onMarqueeMouseMove(event: MouseEvent): void {
    if (!this.marqueeContainer || !this.selectionRect.active) return;

    const { x, y } = this.getRelativePoint(event, this.marqueeContainer);
    const left = Math.min(this.marqueeStartX, x);
    const top = Math.min(this.marqueeStartY, y);
    const width = Math.abs(x - this.marqueeStartX);
    const height = Math.abs(y - this.marqueeStartY);

    if (width > 10 || height > 10) {
      this.hasMarqueeMoved = true;
      if (this.pendingClearOnMove) {
        this.selectionChange.emit([]);
        this.pendingClearOnMove = false;
      }
    }

    this.selectionRect = { active: true, left, top, width, height };
    if (this.hasMarqueeMoved) {
      this.emitMarqueeSelection();
    }
    this.cdr.markForCheck();
  }

  private emitMarqueeSelection(): void {
    if (!this.marqueeContainer) return;
    const selectionPaths = new Set<string>();
    const selectionBottom = this.selectionRect.top + this.selectionRect.height;
    const startIndex = Math.max(0, Math.floor(this.selectionRect.top / this.VIRTUAL_ROW_HEIGHT));
    const endIndex = Math.min(this.flatVisibleNodes.length - 1, Math.floor(Math.max(0, selectionBottom - 1) / this.VIRTUAL_ROW_HEIGHT));

    for (let i = startIndex; i <= endIndex; i++) {
      const node = this.flatVisibleNodes[i];
      if (node) {
        selectionPaths.add(node.path);
      }
    }

    if (this.marqueeAppend) {
      for (const path of this.selectedFiles) selectionPaths.add(path);
    }

    const selectedNodes = this.flatVisibleNodes.filter(n => selectionPaths.has(n.path));
    this.selectionChange.emit(selectedNodes);
  }

  private finishMarqueeSelection(): void {
    this.detachMarqueeListeners();
    this.marqueeContainer = null;
    this.marqueeButton = null;
    this.hasMarqueeMoved = false;
    this.pendingClearOnMove = false;
    this.selectionRect = { active: false, left: 0, top: 0, width: 0, height: 0 };
    this.cdr.markForCheck();
  }

  private detachMarqueeListeners(): void {
    if (this.windowMouseMove) {
      window.removeEventListener('mousemove', this.windowMouseMove);
      this.windowMouseMove = undefined;
    }
    if (this.windowMouseUp) {
      window.removeEventListener('mouseup', this.windowMouseUp);
      this.windowMouseUp = undefined;
    }
    if (this.windowContextMenu) {
      window.removeEventListener('contextmenu', this.windowContextMenu, true);
      this.windowContextMenu = undefined;
    }
  }

  private getRelativePoint(event: MouseEvent, container: HTMLElement): { x: number; y: number } {
    const rect = container.getBoundingClientRect();
    return {
      x: event.clientX - rect.left + container.scrollLeft,
      y: event.clientY - rect.top + container.scrollTop,
    };
  }

  // TC: background left-click — cursor stays, selection unchanged
  onContainerClick(event: MouseEvent): void {
    if (this.suppressContainerClick) {
      this.suppressContainerClick = false;
      return;
    }
  }

  // TC: background right-click — open directory context menu, do NOT change selection
  onContainerContextMenu(event: MouseEvent): void {
    if (this.suppressContainerContextMenu) {
      event.preventDefault();
      this.suppressContainerContextMenu = false;
      return;
    }

    const target = event.target as HTMLElement;
    if (!target.closest('.tree-node')) {
      event.preventDefault();
      this.backgroundContextMenu.emit(event);
    }
  }

  private isDescendant(potentialDescendant: TreeNode, ancestor: TreeNode): boolean {
    let current = potentialDescendant.parent;
    while (current) {
      if (current.path === ancestor.path) return true;
      current = current.parent;
    }
    return false;
  }

  startRename(node: TreeNode): void {
    node.renaming = true;
    node.editingName = node.name;
    this.cdr.markForCheck();
    
    setTimeout(() => {
      this.renameInput?.nativeElement.focus();
      this.renameInput?.nativeElement.select();
    }, 0);
  }

  /** Public helper for parent components to trigger rename on the focused node (eg. global F2). */
  startRenameFocused(): void {
    const visible = this.getVisibleNodes();
    if (visible.length === 0) return;
    const currentIdx = visible.findIndex(n => n.path === this.focusedNodePath);
    const current = currentIdx >= 0 ? visible[currentIdx] : visible[0];
    if (current) this.startRename(current);
  }

  startRenamePath(path: string): void {
    if (!path) return;

    this.tryStartRenamePath(path, Date.now() + 3000);
  }

  setPendingRenameDisplayName(path: string, displayName: string | null): void {
    if (!path) return;
    if (displayName && displayName.trim()) {
      this.pendingRenameDisplayNames.set(path, displayName.trim());
    } else {
      this.pendingRenameDisplayNames.delete(path);
    }
    this.cdr.markForCheck();
  }

  getDisplayNameOverride(path: string): string | null {
    return this.pendingRenameDisplayNames.get(path) ?? null;
  }

  hasPendingRenameState(): boolean {
    return this.pendingRenameDisplayNames.size > 0 || this.hasRenamingNode(this.nodes);
  }

  private tryStartRenamePath(path: string, deadline: number): void {
    this.focusNode(path);

    requestAnimationFrame(() => {
      const target = this.getVisibleNodes().find((node) => node.path === path);
      if (target) {
        this.centerNode(path);
        this.startRename(target);
        return;
      }

      if (Date.now() < deadline) {
        setTimeout(() => this.tryStartRenamePath(path, deadline), 50);
      }
    });
  }

  onRenameBlur(node: TreeNode): void {
    this.commitRename(node);
  }

  onRenameKeyDown(event: KeyboardEvent, node: TreeNode): void {
    event.stopPropagation();

    if (event.key === 'Enter') {
      event.preventDefault();
      this.commitRename(node);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      this.cancelRename(node);
    }
  }

  private commitRename(node: TreeNode): void {
    if (!node.renaming) return;
    
    const newName = node.editingName?.trim();
    if (newName && newName !== node.name) {
      this.rename.emit({ node, newName });
    }
    
    node.renaming = false;
    node.editingName = undefined;
    this.cdr.markForCheck();
  }

  private cancelRename(node: TreeNode): void {
    node.renaming = false;
    node.editingName = undefined;
    this.cdr.markForCheck();
  }

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
          if (event.shiftKey && this.anchorNodePath) {
            this.selectNode(visible[currentIdx + 1], false, true);
          } else {
            this.ensureVisible(visible[currentIdx + 1]);
          }
        }
        break;

      case 'ArrowUp':
        event.preventDefault();
        if (currentIdx > 0) {
          this.focusedNodePath = visible[currentIdx - 1].path;
          if (event.shiftKey && this.anchorNodePath) {
            this.selectNode(visible[currentIdx - 1], false, true);
          } else {
            this.ensureVisible(visible[currentIdx - 1]);
          }
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
          if (!event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) {
            if (current.type === FileType.DIRECTORY) {
              this.inlineSize.emit(current);
            }
            this.selectNodeAndAdvance(current, visible);
          } else {
            this.selectNode(current, event.ctrlKey || event.metaKey, event.shiftKey);
          }
        }
        break;

      case 'Insert':
        event.preventDefault();
        if (current) {
          if (!event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) {
            this.selectNodeAndAdvance(current, visible);
          } else {
            this.selectNode(current, event.ctrlKey || event.metaKey, event.shiftKey);
          }
        }
        break;

      case 'F2':
        event.preventDefault();
        if (current) {
          this.startRename(current);
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

      case 'a':
      case 'A':
        if (event.ctrlKey || event.metaKey) {
          event.preventDefault();
          this.selectionChange.emit(visible);
        }
        break;
    }

    this.cdr.markForCheck();
  }

  private ensureVisible(node: TreeNode): void {
    requestAnimationFrame(() => {
      const host = this.treeContainer?.nativeElement;
      if (!host) return;
      
      if (this.virtualEnabled) {
        const idx = this.visibleNodeIndex.get(node.path) ?? -1;
        if (idx >= 0) {
          const nodeTop = idx * this.VIRTUAL_ROW_HEIGHT;
          const nodeBottom = nodeTop + this.VIRTUAL_ROW_HEIGHT;
          const viewTop = host.scrollTop;
          const viewBottom = viewTop + host.clientHeight;
          
          if (nodeTop < viewTop) {
            host.scrollTop = nodeTop;
          } else if (nodeBottom > viewBottom) {
            host.scrollTop = nodeBottom - host.clientHeight;
          }
          this.updateVirtualWindow();
        }
        return;
      }
      
      const el = host.querySelector(`[data-path="${CSS.escape(node.path)}"]`);
      el?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    });
  }

  trackByFn: TrackByFunction<TreeNode> = (_, node) => node.path;

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
    this.cdr.markForCheck();
  }

  centerNode(path: string): void {
    const node = this.findNodeByPath(this.nodes, path);
    const host = this.treeContainer?.nativeElement;
    if (!host) return;

    if (!node) {
      host.scrollTop = 0;
      return;
    }

    requestAnimationFrame(() => {
      if (this.virtualEnabled) {
        const idx = this.visibleNodeIndex.get(node.path) ?? -1;
        if (idx >= 0) {
          const nodeTop = idx * this.VIRTUAL_ROW_HEIGHT;
          const centerOffset = host.clientHeight / 2 - this.VIRTUAL_ROW_HEIGHT / 2;
          host.scrollTop = Math.max(0, nodeTop - centerOffset);
          this.updateVirtualWindow();
        }
        return;
      }
      
      const el = host.querySelector(`[data-path="${CSS.escape(node.path)}"]`);
      if (!el) return;
      el.scrollIntoView({ block: 'center', inline: 'nearest' });
    });
  }

  private findNodeByPath(nodes: TreeNode[], path: string): TreeNode | undefined {
    const target = this.normalizePath(path);
    for (const node of nodes) {
      if (this.normalizePath(node.path) === target) return node;
      if (node.children) {
        const found = this.findNodeByPath(node.children, target);
        if (found) return found;
      }
    }
    return undefined;
  }

  private expandToCurrentPath(): void {
    if (!this.currentPath || this.nodes.length === 0) return;

    const target = this.findBestPathMatch(this.nodes, this.currentPath);
    if (!target) return;

    let current: TreeNode | undefined = target;
    while (current) {
      if (current.expandable) {
        this.setExpanded(current, true);
      }
      current = current.parent;
    }
    this.cdr.markForCheck();
  }

  private findBestPathMatch(nodes: TreeNode[], path: string): TreeNode | undefined {
    const target = this.normalizePath(path);
    let best: TreeNode | undefined;
    let bestLen = -1;

    const visit = (items: TreeNode[]) => {
      for (const node of items) {
        const nodePath = this.normalizePath(node.path);
        const isExact = nodePath === target;
        const isAncestor = target.startsWith(`${nodePath}/`);

        if ((isExact || isAncestor) && nodePath.length > bestLen) {
          best = node;
          bestLen = nodePath.length;
        }

        if (node.children?.length) {
          visit(node.children);
        }
      }
    };

    visit(nodes);
    return best;
  }

  private normalizePath(path: string): string {
    return normalizePathReference(path || '/');
  }

  private hasRenamingNode(nodes: TreeNode[]): boolean {
    for (const node of nodes) {
      if (node.renaming) {
        return true;
      }
      if (node.children?.length && this.hasRenamingNode(node.children)) {
        return true;
      }
    }
    return false;
  }

  notifyChildrenLoaded(node: TreeNode, children: TreeNode[]): void {
    node.children = children;
    node.loading = false;
    node.loaded = true;
    node.expandable = children.length > 0;
    this.updateNodeHierarchy(children, node);
    this.refreshVisibleNodes();
    this.cdr.markForCheck();
  }
}
