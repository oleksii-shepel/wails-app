import type { EventEmitter } from '@angular/core';
import { FileType, type FileInfo } from '../shared/protocol';
import { parentPath } from './address-bar.utils';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import type { TreeNode, DropTarget } from './pane-tree.component';
import type { ViewLayout } from './file-view-container.component';

export interface FileViewInteractionHost {
  get paneId(): 'left' | 'right';
  get currentPath(): string;
  get currentView(): ViewLayout;
  set currentView(value: ViewLayout);
  dragStart: EventEmitter<{ event: DragEvent; entries: FileInfo[] }>;
  dragDrop: EventEmitter<{ event: DragEvent; source: FileInfo[]; targetPath: string }>;
  viewChange: EventEmitter<ViewLayout>;
  persistViewMode(view: ViewLayout): void;
  syncTreeToCurrentPath(): void;
  enqueueTreeLoad(node: TreeNode): void;
}

export class FileViewInteractionController {
  onTreeDragStart(
    host: FileViewInteractionHost,
    event: { event: DragEvent; nodes: TreeNode[] },
    treeNodeToFileInfo: (node: TreeNode) => FileInfo,
  ): void {
    this.setPaneId(event.event, host.paneId);
    host.dragStart.emit({
      event: event.event,
      entries: event.nodes.map((node) => treeNodeToFileInfo(node)),
    });
  }

  onTreeDragDrop(
    host: FileViewInteractionHost,
    event: { event: DragEvent; source: TreeNode[]; target: DropTarget },
    treeNodeToFileInfo: (node: TreeNode) => FileInfo,
  ): void {
    const targetPath = event.target.position === 'inside'
      ? event.target.node.path
      : parentPath(event.target.node.path);

    host.dragDrop.emit({
      event: event.event,
      source: event.source.map((node) => treeNodeToFileInfo(node)),
      targetPath,
    });
  }

  onViewDragStart(
    host: FileViewInteractionHost,
    event: { event: DragEvent; entries: FileInfo[] },
  ): void {
    this.setPaneId(event.event, host.paneId);
    host.dragStart.emit(event);
  }

  onViewDragDrop(
    host: FileViewInteractionHost,
    event: { event: DragEvent; source: FileInfo[]; target: FileInfo },
  ): void {
    let targetPath = event.target.path;
    if (event.target.path === PARENT_ENTRY_PATH) {
      targetPath = parentPath(host.currentPath);
    } else if (event.target.type !== FileType.DIRECTORY) {
      targetPath = parentPath(event.target.path);
    }

    if (!targetPath) {
      targetPath = host.currentPath;
    }

    host.dragDrop.emit({
      event: event.event,
      source: event.source,
      targetPath,
    });
  }

  onViewChange(host: FileViewInteractionHost, view: ViewLayout): void {
    host.currentView = view;
    host.viewChange.emit(view);
    host.persistViewMode(view);
    if (view === 'tree') {
      host.syncTreeToCurrentPath();
    }
  }

  onLoadChildren(host: FileViewInteractionHost, node: TreeNode): void {
    host.enqueueTreeLoad(node);
  }

  private setPaneId(event: DragEvent, paneId: 'left' | 'right'): void {
    try {
      event.dataTransfer?.setData('application/x-wails-pane-id', paneId);
    } catch {
      // ignore
    }
  }
}
