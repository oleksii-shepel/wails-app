import type { EventEmitter } from '@angular/core';
import type { FileInfo } from '../shared/protocol';
import { FileType } from '../shared/protocol';
import type { TreeNode } from './file-tree.component';
import { PARENT_ENTRY_PATH } from './file-entry.component';

export interface FileViewEventsHost {
  selectionChange: EventEmitter<FileInfo[]>;
  navigate: EventEmitter<FileInfo>;
  openInNewTab: EventEmitter<FileInfo>;
  contextMenu: EventEmitter<{ event: MouseEvent; entry: FileInfo }>;
  nodeDoubleClick: EventEmitter<FileInfo>;
  rename: EventEmitter<{ entry: FileInfo; newName: string }>;
  setSelectedPaths(paths: string[]): void;
  setCursorPath(path: string | null): void;
}

export class FileViewEventsController {
  onSelectionChange(host: FileViewEventsHost, entries: FileInfo[]): void {
    host.setSelectedPaths(
      entries
        .map((entry) => entry.path)
        .filter((path) => path !== PARENT_ENTRY_PATH),
    );
    host.setCursorPath(entries.length > 0 ? entries[entries.length - 1].path : null);
    host.selectionChange.emit(entries);
  }

  onNavigate(host: FileViewEventsHost, entry: FileInfo | TreeNode | Event): void {
    const payload = this.toFileInfo(entry);
    if (!payload) return;
    host.navigate.emit(payload);
  }

  onContextMenu(host: FileViewEventsHost, event: { event: MouseEvent; entry: FileInfo }): void {
    host.contextMenu.emit(event);
  }

  onOpenInNewTab(host: FileViewEventsHost, entry: FileInfo): void {
    host.openInNewTab.emit(entry);
  }

  onTreeSelectionChange(host: FileViewEventsHost, nodes: TreeNode[]): void {
    const files = nodes.map((node) => this.treeNodeToFileInfo(node));
    host.setSelectedPaths(nodes.map((node) => node.path));
    host.setCursorPath(nodes.length > 0 ? nodes[nodes.length - 1].path : null);
    host.selectionChange.emit(files);
  }

  onTreeContextMenu(host: FileViewEventsHost, event: { event: MouseEvent; node: TreeNode }): void {
    host.contextMenu.emit({ event: event.event, entry: this.treeNodeToFileInfo(event.node) });
  }

  onTreeRename(host: FileViewEventsHost, event: { node: TreeNode; newName: string }): void {
    host.rename.emit({
      entry: this.treeNodeToFileInfo(event.node),
      newName: event.newName,
    });
  }

  onTreeOpenInNewTab(host: FileViewEventsHost, node: TreeNode): void {
    host.openInNewTab.emit(this.treeNodeToFileInfo(node));
  }

  onTreeNodeDoubleClick(host: FileViewEventsHost, node: TreeNode): void {
    const file = this.treeNodeToFileInfo(node);
    if (!file) return;

    if (node.type === FileType.DIRECTORY) {
      host.navigate.emit(file);
      return;
    }

    host.nodeDoubleClick.emit(file);
  }

  treeNodeToFileInfo(node: TreeNode): FileInfo {
    return {
      name: node.name,
      path: node.path,
      type: node.type,
      size: node.size,
      modified: node.modified,
      created: node.created,
    } as FileInfo;
  }

  private toFileInfo(entry: FileInfo | TreeNode | Event): FileInfo | null {
    if (!entry || entry instanceof Event) return null;
    const candidate = entry as any;
    if (!candidate.path) return null;

    const payload: any = {
      name: candidate.name,
      path: candidate.path,
      type: candidate.type,
      size: candidate.size,
      modified: candidate.modified,
      created: candidate.created,
    };
    if (Array.isArray(candidate.nestedChain)) {
      payload.nestedChain = [...candidate.nestedChain];
    }
    return payload as FileInfo;
  }
}