import type { FileInfo } from '../shared/protocol';
import type { TreeNode } from './file-tree.component';
import type { ViewLayout } from './file-view-container.component';

export interface FileViewRuntimeHost {
  get currentView(): ViewLayout;
  get sortedEntries(): FileInfo[];
  getVisibleTreeNodes(): TreeNode[];
  startRenameInList(): void;
  startRenameInGrid(): void;
  startRenameInDetails(): void;
  startRenameInTree(): void;
  scrollListToPath(path: string): void;
  scrollDetailsToPath(path: string): void;
  scrollGridToPath(path: string): void;
  focusTreeNode(path: string): void;
  getListApproxPageSize(): number;
  getDetailsApproxPageSize(): number;
  getGridApproxPageSize(): number;
  getGridColumns(): number;
}

export class FileViewRuntimeController {
  startRenameSelected(host: FileViewRuntimeHost): void {
    switch (host.currentView) {
      case 'list':
        host.startRenameInList();
        return;
      case 'grid':
        host.startRenameInGrid();
        return;
      case 'details':
        host.startRenameInDetails();
        return;
      case 'tree':
        host.startRenameInTree();
        return;
      default:
        return;
    }
  }

  scrollToCursorPath(host: FileViewRuntimeHost, path: string): void {
    switch (host.currentView) {
      case 'list':
        host.scrollListToPath(path);
        return;
      case 'details':
        host.scrollDetailsToPath(path);
        return;
      case 'grid':
        host.scrollGridToPath(path);
        return;
      case 'tree':
        host.focusTreeNode(path);
        return;
      default:
        return;
    }
  }

  getSelectableEntries(host: FileViewRuntimeHost, parentEntryPath: string): FileInfo[] {
    if (host.currentView === 'tree') {
      return host.getVisibleTreeNodes()
        .filter((node) => node.path !== parentEntryPath && (node.level ?? 0) >= 0)
        .map((node) => this.treeNodeToFileInfo(node));
    }

    return (host.sortedEntries ?? []).filter((entry) => entry.path !== parentEntryPath);
  }

  getApproxPageSize(host: FileViewRuntimeHost): number {
    switch (host.currentView) {
      case 'list':
        return host.getListApproxPageSize();
      case 'details':
        return host.getDetailsApproxPageSize();
      case 'grid':
        return host.getGridApproxPageSize();
      default:
        return 10;
    }
  }

  getGridColumns(host: FileViewRuntimeHost): number {
    return host.currentView === 'grid' ? host.getGridColumns() : 1;
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
}