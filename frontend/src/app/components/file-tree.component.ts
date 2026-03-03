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
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { FileEntryComponent } from './file-entry.component';
import { FileInfo, FileType } from '../shared/protocol';
import { DEFAULT_FILE_CATEGORIES } from './filter-bar.types';

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
  imports: [CommonModule, FormsModule, FileEntryComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      #treeContainer
      class="file-tree-container"
      role="tree"
      tabindex="0"
      [class.focused]="isFocused"
      [class.dragging]="dragState.isDragging"
      (focus)="onFocus()"
      (blur)="onBlur()"
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
              [class.indent-guide--last]="guide.last">
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

            <app-file-entry
              *ngIf="!node.renaming"
              class="tree-entry"
              [entry]="node"
              [selected]="isSelected(node)"
              variant="inline">
            </app-file-entry>
            
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
      outline: 1px solid var(--vsc-focus-border);
      outline-offset: -1px;
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
      background-color: var(--vsc-list-active-background);
      color: var(--vsc-foreground-bright);
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

    .tree-node-wrapper.drag-over > .tree-node {
      background-color: var(--vsc-list-drop-background, rgba(0, 120, 212, 0.2));
    }

    .drop-indicator {
      position: absolute;
      left: 0;
      right: 0;
      height: 2px;
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

    .tree-node-wrapper.drag-over-inside > .tree-node {
      background-color: var(--vsc-list-drop-background, rgba(0, 120, 212, 0.3));
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

    @keyframes spin {
      to { transform: rotate(360deg); }
    }
  `]
})
export class FileTreeComponent implements OnChanges, OnDestroy, AfterViewInit {
  @Input() nodes: TreeNode[] = [];
  @Input() currentPath = '';
  @Input() selectedFiles: Set<string> = new Set();
  @Input() loading = false;
  @Input() emptyMessage = 'No items';

  @Output() selectionChange = new EventEmitter<TreeNode[]>();
  @Output() nodeDoubleClick = new EventEmitter<TreeNode>();
  @Output() navigate = new EventEmitter<TreeNode>();
  @Output() openInNewTab = new EventEmitter<TreeNode>();
  @Output() toggleExpand = new EventEmitter<TreeNode>();
  @Output() loadChildren = new EventEmitter<TreeNode>();
  @Output() contextMenu = new EventEmitter<{ event: MouseEvent; node: TreeNode }>();
  @Output() dragStart = new EventEmitter<{ event: DragEvent; nodes: TreeNode[] }>();
  @Output() dragDrop = new EventEmitter<{ event: DragEvent; source: TreeNode[]; target: DropTarget }>();
  @Output() rename = new EventEmitter<{ node: TreeNode; newName: string }>();

  @ViewChild('renameInput') renameInput?: ElementRef<HTMLInputElement>;
  @ViewChild('treeContainer') treeContainer?: ElementRef<HTMLElement>;

  private expansionState = new Map<string, boolean>();
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

  // OPTIMIZED: Virtual scroll for massive trees
  private readonly VIRTUAL_THRESHOLD = 500;
  private readonly VIRTUAL_ROW_HEIGHT = 22;
  private readonly VIRTUAL_OVERSCAN = 20;
  private scrollRAF: number | null = null;
  private resizeObserver?: ResizeObserver;
  
  private flatVisibleNodes: TreeNode[] = [];
  private virtualStart = 0;
  private virtualEnd = 0;
  private lastScrollTop = 0;
  private viewportHeight = 0;

  constructor(private cdr: ChangeDetectorRef) {}

  get visibleNodeCount(): number {
    return this.flatVisibleNodes.length;
  }

  get virtualEnabled(): boolean {
    return this.visibleNodeCount >= this.VIRTUAL_THRESHOLD;
  }

  get visibleWindowNodes(): TreeNode[] {
    if (!this.virtualEnabled) return this.flatVisibleNodes;
    return this.flatVisibleNodes.slice(this.virtualStart, this.virtualEnd);
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
    this.updateVirtualWindow();
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['nodes']) {
      this.updateNodeHierarchy(this.nodes);
    }
    if (changes['nodes'] || changes['currentPath']) {
      this.expandToCurrentPath();
    }
    this.refreshVisibleNodes();
    this.updateVirtualWindow();
    this.cdr.markForCheck();
  }

  ngOnDestroy(): void {
    this.cleanupResizeObserver();
    this.cleanupScrollListener();
    if (this.scrollRAF) {
      cancelAnimationFrame(this.scrollRAF);
    }
  }

  private setupResizeObserver(): void {
    if (!this.treeContainer?.nativeElement) return;
    
    this.resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        this.viewportHeight = entry.contentRect.height;
        this.updateVirtualWindow();
        this.cdr.markForCheck();
      }
    });
    
    this.resizeObserver.observe(this.treeContainer.nativeElement);
  }

  private cleanupResizeObserver(): void {
    this.resizeObserver?.disconnect();
  }

  private setupScrollListener(): void {
    const element = this.treeContainer?.nativeElement;
    if (!element) return;
    element.addEventListener('scroll', this.onScroll.bind(this), { passive: true });
  }

  private cleanupScrollListener(): void {
    // Auto-removed
  }

  private onScroll(): void {
    if (this.scrollRAF) return;
    this.scrollRAF = requestAnimationFrame(() => {
      this.scrollRAF = null;
      this.updateVirtualWindow();
      this.cdr.markForCheck();
    });
  }

  private updateVirtualWindow(): void {
    if (!this.virtualEnabled) {
      this.virtualStart = 0;
      this.virtualEnd = this.flatVisibleNodes.length;
      return;
    }

    const host = this.treeContainer?.nativeElement;
    const viewportHeight = Math.max(1, this.viewportHeight || host?.clientHeight || 1);
    const scrollTop = host?.scrollTop ?? 0;

    if (Math.abs(scrollTop - this.lastScrollTop) < this.VIRTUAL_ROW_HEIGHT && this.virtualEnd > 0) {
      return;
    }
    
    this.lastScrollTop = scrollTop;

    const visibleRows = Math.max(1, Math.ceil(viewportHeight / this.VIRTUAL_ROW_HEIGHT));
    const start = Math.max(0, Math.floor(scrollTop / this.VIRTUAL_ROW_HEIGHT) - this.VIRTUAL_OVERSCAN);
    const end = Math.min(this.flatVisibleNodes.length, start + visibleRows + this.VIRTUAL_OVERSCAN * 2);

    if (start !== this.virtualStart || end !== this.virtualEnd) {
      this.virtualStart = start;
      this.virtualEnd = end;
    }
  }

  private refreshVisibleNodes(): void {
    this.flatVisibleNodes = this.getVisibleNodes();
    if (!this.virtualEnabled) {
      this.virtualStart = 0;
      this.virtualEnd = this.flatVisibleNodes.length;
    }
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
    this.updateVirtualWindow();
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

  getGuides(node: TreeNode): { active: boolean; last: boolean }[] {
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

    return chain.map((n) => {
      const siblings = n.parent?.children ?? [];
      const isLast = siblings.length > 0 && siblings[siblings.length - 1].path === n.path;
      return { active: this.isSelected(node), last: isLast };
    });
  }

  getCategoryIcon(filename: string): string {
    const ext = filename.split('.').pop()?.toLowerCase() || '';
    const cat = DEFAULT_FILE_CATEGORIES.find(c => c.extensions.includes(ext));
    return cat ? cat.icon : '📄';
  }

  isSelected(node: TreeNode): boolean {
    return this.selectedFiles.has(node.path);
  }

  isFocusedNode(node: TreeNode): boolean {
    return this.focusedNodePath === node.path;
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
    if (newSelection.length === 1) {
      this.navigate.emit(node);
    }
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

    this.isMouseDownSelecting = true;
    
    const isCtrl = event.ctrlKey || event.metaKey;
    const isShift = event.shiftKey;

    if (isShift && this.anchorNodePath) {
      this.selectNode(node, false, true);
    } else if (isCtrl) {
      this.selectNode(node, true, false);
    } else if (!this.isSelected(node)) {
      this.selectNode(node, false, false);
    }
  }

  onNodeClick(node: TreeNode, event: MouseEvent): void {
    event.stopPropagation();

    if (this.clickTimer) {
      clearTimeout(this.clickTimer);
      this.clickTimer = null;
    }

    if (!this.isMouseDownSelecting) {
      const isCtrl = event.ctrlKey || event.metaKey;
      const isShift = event.shiftKey;
      this.selectNode(node, isCtrl, isShift);
    }
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

    if ((node.type === FileType.DIRECTORY || node.type === FileType.ARCHIVE) && node.expandable) {
      this.toggleNode(node);
    } else {
      this.nodeDoubleClick.emit(node);
      this.navigate.emit(node);
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
    if (!this.isSelected(node)) {
      this.selectNode(node, false, false);
    }

    const selectedNodes = this.getSelectedNodes();
    this.dragState.isDragging = true;
    this.dragState.draggedNodes = selectedNodes;

    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move';
      event.dataTransfer.setData('application/json', JSON.stringify({
        paths: selectedNodes.map(n => n.path),
        source: 'tree'
      }));
      
      if (selectedNodes.length > 1) {
        const dragImage = document.createElement('div');
        dragImage.textContent = `${selectedNodes.length} items`;
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
    if (!this.dragState.isDragging) return;

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

    const isValidDrop = !this.dragState.draggedNodes.some(dragged => 
      dragged.path === node.path || this.isDescendant(node, dragged)
    );

    if (isValidDrop) {
      this.dragState.dropTarget = { node, position };
      if (event.dataTransfer) {
        event.dataTransfer.dropEffect = 'move';
      }
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
    event.stopPropagation();

    if (!this.dragState.dropTarget) return;

    this.dragDrop.emit({
      event,
      source: this.dragState.draggedNodes,
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
  }

  onContainerClick(event: MouseEvent): void {
    const target = event.target as HTMLElement;
    if (!target.closest('.tree-node')) {
      this.selectionChange.emit([]);
      this.anchorNodePath = null;
      this.focusedNodePath = null;
      this.cdr.markForCheck();
    }
  }

  onContainerContextMenu(event: MouseEvent): void {
    const target = event.target as HTMLElement;
    if (!target.closest('.tree-node')) {
      event.preventDefault();
      this.selectionChange.emit([]);
      this.anchorNodePath = null;
      this.focusedNodePath = null;
      this.cdr.markForCheck();
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

  onRenameBlur(node: TreeNode): void {
    this.commitRename(node);
  }

  onRenameKeyDown(event: KeyboardEvent, node: TreeNode): void {
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
          this.selectNode(current, event.ctrlKey || event.metaKey, event.shiftKey);
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
        const idx = this.flatVisibleNodes.findIndex(n => n.path === node.path);
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
        const idx = this.flatVisibleNodes.findIndex(n => n.path === node.path);
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
    const normalized = path.replace(/\\/g, '/').replace(/\/+$/, '');
    return normalized.length ? normalized : '/';
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
