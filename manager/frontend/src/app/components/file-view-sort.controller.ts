import { FileType, type FileInfo } from '../shared/protocol';
import type { TreeNode } from './pane-tree.component';

export interface FileViewSortHost {
  get entries(): FileInfo[];
  get sortField(): string;
  get sortDirection(): 'asc' | 'desc';
  get directoriesFirst(): boolean;
  setSortedEntries(entries: FileInfo[]): void;
}

export class FileViewSortController {
  sortEntries(host: FileViewSortHost, parentEntryPath: string): void {
    const sorted = [...host.entries];
    const multiplier = host.sortDirection === 'asc' ? 1 : -1;

    sorted.sort((left, right) => {
      if (left.path === parentEntryPath) return -1;
      if (right.path === parentEntryPath) return 1;

      if (host.directoriesFirst) {
        if (left.type === FileType.DIRECTORY && right.type !== FileType.DIRECTORY) return -1;
        if (right.type === FileType.DIRECTORY && left.type !== FileType.DIRECTORY) return 1;
      }

      return this.compareEntries(host.sortField, multiplier, left, right);
    });

    host.setSortedEntries(sorted);
  }

  sortTreeNodes(host: FileViewSortHost, nodes: TreeNode[]): TreeNode[] {
    const multiplier = host.sortDirection === 'asc' ? 1 : -1;
    return [...nodes].sort((left, right) => {
      if (host.directoriesFirst) {
        const leftIsDir = left.type === FileType.DIRECTORY || left.type === FileType.ARCHIVE;
        const rightIsDir = right.type === FileType.DIRECTORY || right.type === FileType.ARCHIVE;
        if (leftIsDir && !rightIsDir) return -1;
        if (!leftIsDir && rightIsDir) return 1;
      }

      return this.compareByField(host.sortField, multiplier, left, right);
    });
  }

  resortAllLoadedTreeNodes(host: FileViewSortHost, nodes: TreeNode[]): void {
    for (const node of nodes) {
      if (!node.children?.length) continue;
      node.children = this.sortTreeNodes(host, node.children);
      this.resortAllLoadedTreeNodes(host, node.children);
    }
  }

  private compareEntries(
    sortField: string,
    multiplier: number,
    left: FileInfo,
    right: FileInfo,
  ): number {
    return this.compareByField(sortField, multiplier, left, right);
  }

  private compareByField(
    sortField: string,
    multiplier: number,
    left: Pick<FileInfo, 'name' | 'type' | 'size' | 'modified' | 'registryKind' | 'registryValueType' | 'registryPreview' | 'searchParentPath'>,
    right: Pick<FileInfo, 'name' | 'type' | 'size' | 'modified' | 'registryKind' | 'registryValueType' | 'registryPreview' | 'searchParentPath'>,
  ): number {
    let comparison = 0;
    switch (sortField) {
      case 'name':
        comparison = left.name.localeCompare(right.name);
        break;
      case 'size':
        comparison = (left.size || 0) - (right.size || 0);
        break;
      case 'modified':
        comparison = (left.modified || 0) - (right.modified || 0);
        break;
      case 'type': {
        const leftType = this.sortTypeLabel(left);
        const rightType = this.sortTypeLabel(right);
        comparison = leftType.localeCompare(rightType);
        break;
      }
      case 'registryKind':
        comparison = (left.registryKind || '').localeCompare(right.registryKind || '');
        break;
      case 'registryValueType':
        comparison = (left.registryValueType || '').localeCompare(right.registryValueType || '');
        break;
      case 'registryPreview':
        comparison = (left.registryPreview || '').localeCompare(right.registryPreview || '');
        break;
      case 'searchParentPath':
        comparison = (left.searchParentPath || '').localeCompare(right.searchParentPath || '');
        break;
      default:
        comparison = left.name.localeCompare(right.name);
    }
    return comparison * multiplier;
  }

  private sortTypeLabel(entry: Pick<FileInfo, 'name' | 'type'>): string {
    return entry.type === FileType.DIRECTORY || entry.type === FileType.ARCHIVE
      ? 'folder'
      : (entry.name.split('.').pop() || '');
  }
}
